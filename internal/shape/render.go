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
	// Depth limits how many levels of nested objects and arrays are shown;
	// deeper ones collapse to {…} and […]. Zero means no limit.
	Depth int

	// Color highlights types, keys and optional markers with ANSI escapes.
	Color bool
}

// collapsed reports whether a container at the given nesting level, where
// the top-level value is level 1, is hidden by the Depth limit.
func (o Options) collapsed(level int) bool {
	return o.Depth > 0 && level > o.Depth
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
	r.expr(s, 1, 0)
	r.newline()
	return writeLines(w, r.lines, opts.Color)
}

type renderer struct {
	opts  Options
	lines []line
	cur   line
}

func (r *renderer) add(text string, st style) { r.cur.add(text, st) }

func (r *renderer) newline() {
	r.lines = append(r.lines, r.cur)
	r.cur = line{}
}

func (r *renderer) indent(n int) {
	if n > 0 {
		r.add(strings.Repeat("  ", n), plain)
	}
}

// expr writes the type of s, a value at the given nesting level, starting
// at the current position. Objects and arrays of objects continue over
// several lines, indented one step past indent.
func (r *renderer) expr(s *Shape, level, indent int) {
	for i, m := range s.members(false) {
		if i > 0 {
			r.add(" | ", plain)
		}
		switch m.kind {
		case kindObject:
			r.object(s.Object, level, indent)
		case kindArray:
			r.array(s.Array, level, indent)
		default:
			r.add(m.kind.String(), m.kind.style())
		}
	}
}

func (r *renderer) object(o *Object, level, indent int) {
	switch {
	case len(o.Fields) == 0:
		r.add("{}", plain)
		return
	case r.opts.collapsed(level):
		r.add("{…}", dimStyle)
		return
	}
	r.add("{", plain)
	r.newline()
	for _, f := range o.Fields {
		r.indent(indent + 1)
		r.add(displayKey(f.Key), keyStyle)
		if o.Optional(f) {
			r.add("?", optionalStyle)
		}
		r.add(": ", plain)
		r.expr(f.Shape, level+1, indent+1)
		r.newline()
	}
	r.indent(indent)
	r.add("}", plain)
}

func (r *renderer) array(a *Array, level, indent int) {
	switch {
	case a.Items.Count == 0:
		r.add("[]", plain)
		return
	case r.opts.collapsed(level):
		r.add("[…]", dimStyle)
		return
	case r.inline(a.Items, level+1):
		r.add("[", plain)
		r.expr(a.Items, level+1, indent)
		r.add("]", plain)
		return
	}
	r.add("[", plain)
	r.newline()
	r.indent(indent + 1)
	r.expr(a.Items, level+1, indent+1)
	r.newline()
	r.indent(indent)
	r.add("]", plain)
}

// inline reports whether s, a value at the given level, fits on one line:
// it shows no non-empty object at any depth.
func (r *renderer) inline(s *Shape, level int) bool {
	switch {
	case r.opts.collapsed(level):
		return true
	case s.Object != nil && len(s.Object.Fields) > 0:
		return false
	case s.Array != nil:
		return r.inline(s.Array.Items, level+1)
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
