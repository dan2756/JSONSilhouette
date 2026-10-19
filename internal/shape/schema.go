package shape

import (
	"bytes"
	"io"
	"strings"
)

// SchemaDialect is the JSON Schema version written by RenderSchema.
const SchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// RenderSchema writes a JSON Schema that every value merged into s
// satisfies. Each position gets the types seen there; objects list their
// properties in order of first appearance and require the keys that every
// object had. Ints and floats together become "number".
//
// Only Options.Depth applies: containers below it are described by their
// type alone. SplitNumbers has no effect, as every integer is a number.
func RenderSchema(w io.Writer, s *Shape, opts Options) error {
	root := append(ordered{{"$schema", SchemaDialect}}, schemaFor(s, 1, opts)...)
	var b bytes.Buffer
	writeJSON(&b, root, 0)
	b.WriteByte('\n')
	_, err := w.Write(b.Bytes())
	return err
}

var schemaTypes = [...]string{
	kindBool:   "boolean",
	kindInt:    "integer",
	kindFloat:  "number",
	kindNumber: "number",
	kindString: "string",
	kindArray:  "array",
	kindObject: "object",
	kindNull:   "null",
}

// schemaFor describes s, a value at the given nesting level. A single
// schema can hold the keywords of several types, since "items" only
// applies to arrays and "properties" only to objects.
func schemaFor(s *Shape, level int, opts Options) ordered {
	var n ordered
	var types []string
	for _, m := range s.members(false) {
		types = append(types, schemaTypes[m.kind])
	}
	switch len(types) {
	case 0: // nothing seen here; anything goes
	case 1:
		n = append(n, kv{"type", types[0]})
	default:
		n = append(n, kv{"type", types})
	}
	if opts.collapsed(level) {
		return n
	}
	if a := s.Array; a != nil && a.Items.Count > 0 {
		n = append(n, kv{"items", schemaFor(a.Items, level+1, opts)})
	}
	if o := s.Object; o != nil && len(o.Fields) > 0 {
		props := make(ordered, 0, len(o.Fields))
		var required []string
		for _, f := range o.Fields {
			props = append(props, kv{f.Key, schemaFor(f.Shape, level+1, opts)})
			if !o.Optional(f) {
				required = append(required, f.Key)
			}
		}
		n = append(n, kv{"properties", props})
		if len(required) > 0 {
			n = append(n, kv{"required", required})
		}
	}
	return n
}

// ordered is a JSON object that keeps its keys in the order given.
type ordered []kv

type kv struct {
	key   string
	value any // string, []string or ordered
}

// Arrays of strings stay on one line when they fit in this many characters.
const maxInlineArray = 72

// writeJSON writes v, indented by two spaces per level, the way
// json.MarshalIndent would except that short arrays of strings stay on one
// line.
func writeJSON(b *bytes.Buffer, v any, indent int) {
	switch v := v.(type) {
	case string:
		b.WriteString(quote(v))
	case []string:
		quoted := make([]string, len(v))
		width := 0
		for i, s := range v {
			quoted[i] = quote(s)
			width += len(quoted[i]) + 2
		}
		if width <= maxInlineArray {
			b.WriteString("[" + strings.Join(quoted, ", ") + "]")
			return
		}
		b.WriteString("[\n")
		for i, q := range quoted {
			writeIndent(b, indent+1)
			b.WriteString(q)
			if i < len(quoted)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		writeIndent(b, indent)
		b.WriteByte(']')
	case ordered:
		if len(v) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, f := range v {
			writeIndent(b, indent+1)
			b.WriteString(quote(f.key))
			b.WriteString(": ")
			writeJSON(b, f.value, indent+1)
			if i < len(v)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		writeIndent(b, indent)
		b.WriteByte('}')
	default:
		panic("writeJSON: unexpected value type")
	}
}

func writeIndent(b *bytes.Buffer, n int) {
	for range n {
		b.WriteString("  ")
	}
}
