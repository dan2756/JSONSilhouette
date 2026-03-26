// Package shape infers the structure of JSON values.
//
// Values are read token by token from an encoding/json Decoder and merged
// into a Shape as they stream past, so object keys keep the order in which
// they first appeared and numbers are classified from their literal text
// without ever being converted to float64.
package shape

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Shape is the merged type of every value seen at one position. A position
// that held a string in one sample and null in another has both String and
// Null set; it renders as the union "string | null".
type Shape struct {
	Count int // values seen at this position

	Null   int
	Bool   int
	Int    int
	Float  int
	String int

	Object *Object // non-nil once an object has been seen here
	Array  *Array  // non-nil once an array has been seen here
}

// Object is the merged shape of every object seen at one position.
type Object struct {
	Count  int      // objects seen
	Fields []*Field // in order of first appearance
	index  map[string]*Field
}

// Field is one key of an Object.
type Field struct {
	Key   string
	Count int // objects that contained the key
	Shape *Shape
}

// Optional reports whether some of the objects lacked f.
func (o *Object) Optional(f *Field) bool { return f.Count < o.Count }

// Array is the merged shape of every array seen at one position. The
// elements of all of them are merged into Items; Items.Count == 0 means
// every array was empty.
type Array struct {
	Count int // arrays seen
	Items *Shape
}

// New returns an empty shape, ready for Add.
func New() *Shape { return &Shape{} }

// NewDecoder returns a decoder suitable for Add: it reports numbers as
// json.Number so that ints and floats can be told apart.
func NewDecoder(r io.Reader) *json.Decoder {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec
}

// Add reads one JSON value from dec and merges it into s. It returns io.EOF
// if dec holds no further values, and io.ErrUnexpectedEOF if the input ends
// in the middle of one.
func (s *Shape) Add(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	return s.add(dec, tok)
}

func (s *Shape) add(dec *json.Decoder, tok json.Token) error {
	s.Count++
	switch v := tok.(type) {
	case nil:
		s.Null++
	case bool:
		s.Bool++
	case string:
		s.String++
	case json.Number:
		if isFloat(string(v)) {
			s.Float++
		} else {
			s.Int++
		}
	case float64: // decoder without UseNumber
		if isFloat(strconv.FormatFloat(v, 'g', -1, 64)) {
			s.Float++
		} else {
			s.Int++
		}
	case json.Delim:
		switch v {
		case '{':
			return s.addObject(dec)
		case '[':
			return s.addArray(dec)
		}
		return fmt.Errorf("unexpected %q", rune(v))
	default:
		return fmt.Errorf("unexpected token %v", tok)
	}
	return nil
}

func (s *Shape) addObject(dec *json.Decoder) error {
	if s.Object == nil {
		s.Object = &Object{index: make(map[string]*Field)}
	}
	o := s.Object
	o.Count++
	for {
		tok, err := next(dec)
		if err != nil {
			return err
		}
		if tok == json.Delim('}') {
			return nil
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("unexpected object key %v", tok)
		}
		f := o.field(key)
		f.Count++
		if tok, err = next(dec); err != nil {
			return err
		}
		if err := f.Shape.add(dec, tok); err != nil {
			return err
		}
	}
}

func (s *Shape) addArray(dec *json.Decoder) error {
	if s.Array == nil {
		s.Array = &Array{Items: New()}
	}
	a := s.Array
	a.Count++
	for {
		tok, err := next(dec)
		if err != nil {
			return err
		}
		if tok == json.Delim(']') {
			return nil
		}
		if err := a.Items.add(dec, tok); err != nil {
			return err
		}
	}
}

func (o *Object) field(key string) *Field {
	if f, ok := o.index[key]; ok {
		return f
	}
	f := &Field{Key: key, Shape: New()}
	o.index[key] = f
	o.Fields = append(o.Fields, f)
	return f
}

// next reads a token inside a value, where running out of input is an error.
func next(dec *json.Decoder) (json.Token, error) {
	tok, err := dec.Token()
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	return tok, err
}

// isFloat reports whether a JSON number literal is written as a float.
// The decision is lexical: 1.0 and 1e3 are floats, 12345678901234567890
// is an int however large it is.
func isFloat(lit string) bool {
	return strings.ContainsAny(lit, ".eE")
}

type kind uint8

const (
	kindBool kind = iota
	kindInt
	kindFloat
	kindNumber // int and float merged
	kindString
	kindArray
	kindObject
	kindNull
)

var kindNames = [...]string{
	kindBool:   "bool",
	kindInt:    "int",
	kindFloat:  "float",
	kindNumber: "number",
	kindString: "string",
	kindArray:  "array",
	kindObject: "object",
	kindNull:   "null",
}

func (k kind) String() string { return kindNames[k] }

type member struct {
	kind  kind
	count int
}

// members lists the types in s in display order: scalars first, then
// containers, with null last so that unions read like "string | null".
// Ints and floats seen at the same position merge into number unless
// splitNumbers is set.
func (s *Shape) members(splitNumbers bool) []member {
	var ms []member
	add := func(k kind, n int) {
		if n > 0 {
			ms = append(ms, member{k, n})
		}
	}
	add(kindBool, s.Bool)
	if s.Int > 0 && s.Float > 0 && !splitNumbers {
		add(kindNumber, s.Int+s.Float)
	} else {
		add(kindInt, s.Int)
		add(kindFloat, s.Float)
	}
	add(kindString, s.String)
	if s.Array != nil {
		add(kindArray, s.Array.Count)
	}
	if s.Object != nil {
		add(kindObject, s.Object.Count)
	}
	add(kindNull, s.Null)
	return ms
}
