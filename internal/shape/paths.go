package shape

import (
	"io"
	"slices"
	"strings"
)

// The type column of RenderPaths starts after the longest path, but no
// further right than this.
const maxPathColumn = 48

// RenderPaths writes s to w with one line per position: a jq-style path
// and the type found there.
//
//	.                array
//	.[]              object
//	.[].id           int
//	.[].tags         array
//	.[].tags[]       string
//	.[].owner        object | null
//	.[].owner.login  string
//	.[].deleted_at?  string
//
// A "?" keeps the path valid for jq. Keys missing from some objects get
// one, and so does a key of a value that is not always an object: jq will
// not index a string, number, bool, or array, so ["s", {"k": true}] is
// .[].k?. jq yields null for a missing key and for a field of null, but
// it cannot iterate over null or a scalar, so a [] that might see one is
// written []?: .[].tags?[]? when tags is optional, .[].tags[]? when tags
// is sometimes null. [] iterates objects as well as arrays, so where a
// value was both, the type is the union of the array's elements and the
// object's values: [[1], {"a": "s"}] is .[][] int | string. Indexing null
// does not error, so .[].owner.login is left as it is when owner is null.
func RenderPaths(w io.Writer, s *Shape, opts Options) error {
	p := pathRenderer{opts: opts}
	p.walk(nil, s, 1, samples(s), false)

	col := 0
	for _, e := range p.entries {
		col = max(col, e.path.width())
	}
	col = min(col, maxPathColumn) + 2

	lines := make([]line, len(p.entries))
	for i, e := range p.entries {
		l := line{segs: slices.Clone(e.path.segs), note: e.note}
		l.add(strings.Repeat(" ", max(2, col-l.width())), plain)
		l.segs = append(l.segs, e.typ.segs...)
		lines[i] = l
	}
	return writeLines(w, lines, opts.Color)
}

type pathEntry struct {
	path, typ line
	note      string
}

type pathRenderer struct {
	opts    Options
	entries []pathEntry
}

// walk lists the position at path, holding s at the given nesting level,
// and everything below it. The root's path is empty and printed as ".".
// first leads the position's --counts note. missing is set when the path
// can already be null in jq, so a [] below it is written []? instead of
// failing on that null.
func (p *pathRenderer) walk(path []seg, s *Shape, level int, first string, missing bool) {
	e := pathEntry{path: line{segs: path}, typ: typeSummary(s, p.opts.SplitNumbers)}
	if len(path) == 0 {
		e.path.add(".", plain)
	}
	if p.opts.Counts {
		e.note = joinNote(first, breakdown(s))
	}
	p.entries = append(p.entries, e)
	if p.opts.collapsed(level) {
		return
	}
	if o := s.Object; o != nil {
		// .key errors on a string, number, bool, or array. It does not
		// error on null: the result is null, and a [] below that has to
		// be []? or jq will stop.
		unsafeKey := keyAccessMayFail(s)
		for _, f := range o.Fields {
			child := childKey(path, f.Key)
			if o.Optional(f) || unsafeKey {
				child = append(child, seg{"?", optionalStyle})
			}
			childMissing := missing || o.Optional(f) || s.Null > 0
			p.walk(child, f.Shape, level+1, presence(f.Count, o.Count), childMissing)
		}
	}
	// [] is only listed when an array was seen here. A pure object is shown
	// by its keys; once an array is present, the same [] also yields any
	// object values, and those belong in the type.
	if s.Array != nil {
		items := indexedValues(s)
		if items != nil && items.Count > 0 {
			text := "[]"
			if len(path) == 0 {
				text = ".[]"
			}
			child := append(slices.Clip(path), seg{text, plain})
			if missing || !jqCanIterate(s) {
				child = append(child, seg{"?", optionalStyle})
			}
			p.walk(child, items, level+1, countOf(items.Count, "item"), false)
		}
	}
}

// indexedValues is the shape of what jq's [] yields for s: every array
// element, and every object value, since [] iterates both. Nil when []
// would yield nothing, including an array that was always empty and an
// object that never had keys. The original shape is not modified.
func indexedValues(s *Shape) *Shape {
	var items *Shape
	if s.Array != nil && s.Array.Items.Count > 0 {
		items = s.Array.Items
	}
	o := s.Object
	if o == nil || len(o.Fields) == 0 {
		return items
	}
	if items == nil {
		items = New()
	} else {
		items = items.clone()
	}
	for _, f := range o.Fields {
		items.merge(f.Shape)
	}
	if items.Count == 0 {
		return nil
	}
	return items
}

func (s *Shape) clone() *Shape {
	c := *s
	if s.Object != nil {
		c.Object = s.Object.clone()
	}
	if s.Array != nil {
		c.Array = s.Array.clone()
	}
	return &c
}

func (o *Object) clone() *Object {
	c := &Object{Count: o.Count, index: make(map[string]*Field, len(o.Fields))}
	for _, f := range o.Fields {
		nf := &Field{Key: f.Key, Count: f.Count, Shape: f.Shape.clone()}
		c.index[nf.Key] = nf
		c.Fields = append(c.Fields, nf)
	}
	return c
}

func (a *Array) clone() *Array {
	return &Array{Count: a.Count, Items: a.Items.clone()}
}

// merge folds o into s. Counts add, and object keys keep the order they
// first had in s, then in o.
func (s *Shape) merge(o *Shape) {
	s.Count += o.Count
	s.Null += o.Null
	s.Bool += o.Bool
	s.Int += o.Int
	s.Float += o.Float
	s.String += o.String
	if o.Object != nil {
		if s.Object == nil {
			s.Object = o.Object.clone()
		} else {
			s.Object.merge(o.Object)
		}
	}
	if o.Array != nil {
		if s.Array == nil {
			s.Array = o.Array.clone()
		} else {
			s.Array.Count += o.Array.Count
			s.Array.Items.merge(o.Array.Items)
		}
	}
}

func (o *Object) merge(src *Object) {
	o.Count += src.Count
	for _, f := range src.Fields {
		df := o.field(f.Key)
		df.Count += f.Count
		df.Shape.merge(f.Shape)
	}
}

// keyAccessMayFail reports whether jq's .key errors on some value in s.
// Null and objects are fine; strings, numbers, bools, and arrays are not.
func keyAccessMayFail(s *Shape) bool {
	return s.Bool > 0 || s.Int > 0 || s.Float > 0 || s.String > 0 || s.Array != nil
}

// jqCanIterate reports whether jq's [] accepts every value in s. Arrays
// and objects both iterate; null and scalars do not.
func jqCanIterate(s *Shape) bool {
	ms := s.members(true)
	if len(ms) == 0 {
		return false
	}
	for _, m := range ms {
		if m.kind != kindArray && m.kind != kindObject {
			return false
		}
	}
	return true
}

// childKey returns the path of key inside the object at path: .key for
// identifiers and ["key"] for anything else.
func childKey(path []seg, key string) []seg {
	var text string
	switch {
	case isIdent(key):
		text = "." + key
	case len(path) == 0:
		text = ".[" + quote(key) + "]"
	default:
		text = "[" + quote(key) + "]"
	}
	return append(slices.Clip(path), seg{text, plain})
}

// isIdent reports whether key can follow a dot in a jq path.
func isIdent(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		switch {
		case c == '_', 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// typeSummary names the types in s on one line, with containers written
// as object and array, or {} and [] when they were always empty.
func typeSummary(s *Shape, splitNumbers bool) line {
	var l line
	for i, m := range s.members(splitNumbers) {
		if i > 0 {
			l.add(" | ", plain)
		}
		switch {
		case m.kind == kindObject && len(s.Object.Fields) == 0:
			l.add("{}", plain)
		case m.kind == kindArray && s.Array.Items.Count == 0:
			l.add("[]", plain)
		default:
			l.add(m.kind.String(), m.kind.style())
		}
	}
	return l
}
