package shape

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode"
)

// Options control how a shape is rendered.
type Options struct {
	// Color highlights types, keys and optional markers with ANSI escapes.
	Color bool
}

// Render writes s to w as an indented tree, one key per line:
//
//	[
//	  {
//	    id: int
//	    email: string | null
//	    tags: [string]
//	    deleted_at?: string
//	  }
//	]
func Render(w io.Writer, s *Shape, opts Options) error {
	r := renderer{opts: opts}
	r.expr(s, 0)
	r.b.WriteByte('\n')
	_, err := io.WriteString(w, r.b.String())
	return err
}

type renderer struct {
	opts Options
	b    strings.Builder
}

// style is the color class of a piece of output.
type style uint8

const (
	plain style = iota
	keyStyle
	optionalStyle
	stringStyle
	numberStyle
	boolStyle
	nullStyle
)

var ansi = [...]string{
	keyStyle:      "34",
	optionalStyle: "33",
	stringStyle:   "32",
	numberStyle:   "36",
	boolStyle:     "35",
	nullStyle:     "90",
}

func (k kind) style() style {
	switch k {
	case kindBool:
		return boolStyle
	case kindInt, kindFloat, kindNumber:
		return numberStyle
	case kindString:
		return stringStyle
	case kindNull:
		return nullStyle
	}
	return plain
}

// paint writes text in the given style.
func (r *renderer) paint(text string, st style) {
	if !r.opts.Color || st == plain {
		r.b.WriteString(text)
		return
	}
	r.b.WriteString("\x1b[")
	r.b.WriteString(ansi[st])
	r.b.WriteByte('m')
	r.b.WriteString(text)
	r.b.WriteString("\x1b[0m")
}

func (r *renderer) indent(n int) {
	for range n {
		r.b.WriteString("  ")
	}
}

// expr writes the type of s starting at the current position. Objects and
// arrays of objects continue over several lines, indented one step past
// indent.
func (r *renderer) expr(s *Shape, indent int) {
	for i, m := range s.members(false) {
		if i > 0 {
			r.b.WriteString(" | ")
		}
		switch m.kind {
		case kindObject:
			r.object(s.Object, indent)
		case kindArray:
			r.array(s.Array, indent)
		default:
			r.paint(m.kind.String(), m.kind.style())
		}
	}
}

func (r *renderer) object(o *Object, indent int) {
	if len(o.Fields) == 0 {
		r.b.WriteString("{}")
		return
	}
	r.b.WriteString("{\n")
	for _, f := range o.Fields {
		r.indent(indent + 1)
		r.paint(displayKey(f.Key), keyStyle)
		if o.Optional(f) {
			r.paint("?", optionalStyle)
		}
		r.b.WriteString(": ")
		r.expr(f.Shape, indent+1)
		r.b.WriteByte('\n')
	}
	r.indent(indent)
	r.b.WriteByte('}')
}

func (r *renderer) array(a *Array, indent int) {
	if a.Items.Count == 0 {
		r.b.WriteString("[]")
		return
	}
	if inline(a.Items) {
		r.b.WriteByte('[')
		r.expr(a.Items, indent)
		r.b.WriteByte(']')
		return
	}
	r.b.WriteString("[\n")
	r.indent(indent + 1)
	r.expr(a.Items, indent+1)
	r.b.WriteByte('\n')
	r.indent(indent)
	r.b.WriteByte(']')
}

// inline reports whether s fits on one line: it holds no non-empty object,
// at any depth.
func inline(s *Shape) bool {
	if s.Object != nil && len(s.Object.Fields) > 0 {
		return false
	}
	if s.Array != nil {
		return inline(s.Array.Items)
	}
	return true
}

// displayKey returns k bare when it is made only of letters, digits and a
// few common punctuation characters, and as a quoted JSON string otherwise.
func displayKey(k string) string {
	if k == "" {
		return `""`
	}
	for _, c := range k {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && !strings.ContainsRune("_-$@", c) {
			return quote(k)
		}
	}
	return k
}

// quote returns s as a JSON string literal, without the HTML escaping that
// json.Marshal applies.
func quote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	return strings.TrimSuffix(b.String(), "\n")
}
