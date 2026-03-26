package shape

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode"
)

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
func Render(w io.Writer, s *Shape) error {
	var r renderer
	r.expr(s, 0)
	r.b.WriteByte('\n')
	_, err := io.WriteString(w, r.b.String())
	return err
}

type renderer struct {
	b strings.Builder
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
			r.b.WriteString(m.kind.String())
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
		r.b.WriteString(displayKey(f.Key))
		if o.Optional(f) {
			r.b.WriteByte('?')
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
