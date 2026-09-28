package shape

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func renderSchema(t *testing.T, s *Shape, opts Options) string {
	t.Helper()
	var b strings.Builder
	if err := RenderSchema(&b, s, opts); err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(b.String())) {
		t.Fatalf("schema is not valid JSON:\n%s", b.String())
	}
	return b.String()
}

func TestRenderSchema(t *testing.T) {
	s := infer(t, `[{"id": 1, "email": "a", "tags": ["x"], "o": {"k": 1.5}}, {"id": 2, "email": null, "tags": [], "o": null, "extra": true}]`)
	want := `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "id": {
        "type": "integer"
      },
      "email": {
        "type": ["string", "null"]
      },
      "tags": {
        "type": "array",
        "items": {
          "type": "string"
        }
      },
      "o": {
        "type": ["object", "null"],
        "properties": {
          "k": {
            "type": "number"
          }
        },
        "required": ["k"]
      },
      "extra": {
        "type": "boolean"
      }
    },
    "required": ["id", "email", "tags", "o"]
  }
}
`
	if got := renderSchema(t, s, Options{}); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderSchemaTypes(t *testing.T) {
	tests := []struct {
		in   string
		want string // the root schema without $schema, compacted
	}{
		{`1`, `{"type":"integer"}`},
		{`1.5`, `{"type":"number"}`},
		{`[1, 1.5]`, `{"type":"array","items":{"type":"number"}}`},
		{`"s"`, `{"type":"string"}`},
		{`true`, `{"type":"boolean"}`},
		{`null`, `{"type":"null"}`},
		{`{}`, `{"type":"object"}`},
		{`[]`, `{"type":"array"}`},
		{`[[]]`, `{"type":"array","items":{"type":"array"}}`},
		{`["a", [1], {"k": 1}]`, `{"type":"array","items":{"type":["string","array","object"],"items":{"type":"integer"},"properties":{"k":{"type":"integer"}},"required":["k"]}}`},
		{`[{"a": 1}, {"b": 2}]`, `{"type":"array","items":{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"}}}}`},
		{`{"<a&b>": 1, "q\"uote": 2}`, `{"type":"object","properties":{"<a&b>":{"type":"integer"},"q\"uote":{"type":"integer"}},"required":["<a&b>","q\"uote"]}`},
	}
	prefix := `{"$schema":"` + SchemaDialect + `",`
	for _, tt := range tests {
		got := compactJSON(t, renderSchema(t, infer(t, tt.in), Options{}))
		if !strings.HasPrefix(got, prefix) {
			t.Errorf("%s: schema does not start with $schema: %s", tt.in, got)
			continue
		}
		if got = "{" + strings.TrimPrefix(got, prefix); got != tt.want {
			t.Errorf("%s:\ngot  %s\nwant %s", tt.in, got, tt.want)
		}
	}
}

func TestRenderSchemaDepth(t *testing.T) {
	got := compactJSON(t, renderSchema(t, infer(t, `{"a": {"b": {"c": 1}}, "l": [[1]]}`), Options{Depth: 2}))
	want := `{"$schema":"` + SchemaDialect + `","type":"object","properties":{` +
		`"a":{"type":"object","properties":{"b":{"type":"object"}},"required":["b"]},` +
		`"l":{"type":"array","items":{"type":"array"}}},"required":["a","l"]}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestRenderSchemaLongRequired(t *testing.T) {
	var keys []string
	for i := range 12 {
		keys = append(keys, fmt.Sprintf(`"key_number_%02d": 1`, i))
	}
	got := renderSchema(t, infer(t, "{"+strings.Join(keys, ", ")+"}"), Options{})
	if !strings.Contains(got, "  \"required\": [\n    \"key_number_00\",\n    \"key_number_01\",\n") {
		t.Errorf("long required list should wrap:\n%s", got)
	}
}

// TestSchemaAcceptsSamples checks that every sample validates against the
// schema inferred from all of them, with a minimal validator for the
// keywords RenderSchema uses.
func TestSchemaAcceptsSamples(t *testing.T) {
	sets := [][]string{
		{`{"id": 1, "tags": ["a"], "o": {"x": null}}`, `{"id": 2.5, "tags": [], "o": null}`, `{"tags": [1, {"y": true}]}`},
		{`[1, "a", null, [2, [3]], {"k": [{"z": 1}, {}]}]`},
		{`"s"`, `1`, `{"a": {}}`, `[]`},
	}
	for _, docs := range sets {
		var schema map[string]any
		if err := json.Unmarshal([]byte(renderSchema(t, infer(t, docs...), Options{})), &schema); err != nil {
			t.Fatal(err)
		}
		for _, doc := range docs {
			dec := json.NewDecoder(strings.NewReader(doc))
			dec.UseNumber()
			var v any
			if err := dec.Decode(&v); err != nil {
				t.Fatal(err)
			}
			if err := validate(schema, v, "$"); err != nil {
				t.Errorf("%s does not match its schema: %v", doc, err)
			}
		}
	}
}

func validate(schema map[string]any, v any, at string) error {
	if t, ok := schema["type"]; ok {
		var types []string
		switch t := t.(type) {
		case string:
			types = []string{t}
		case []any:
			for _, x := range t {
				types = append(types, x.(string))
			}
		}
		ok := false
		for _, typ := range types {
			ok = ok || hasType(typ, v)
		}
		if !ok {
			return fmt.Errorf("%s: %v is not %v", at, v, types)
		}
	}
	switch v := v.(type) {
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for i, x := range v {
				if err := validate(items, x, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		for k, x := range v {
			if p, ok := props[k].(map[string]any); ok {
				if err := validate(p, x, at+"."+k); err != nil {
					return err
				}
			}
		}
		req, _ := schema["required"].([]any)
		for _, k := range req {
			if _, ok := v[k.(string)]; !ok {
				return fmt.Errorf("%s: missing required %q", at, k)
			}
		}
	}
	return nil
}

func hasType(typ string, v any) bool {
	switch v := v.(type) {
	case nil:
		return typ == "null"
	case bool:
		return typ == "boolean"
	case string:
		return typ == "string"
	case json.Number:
		return typ == "number" || typ == "integer" && !isFloat(string(v))
	case []any:
		return typ == "array"
	case map[string]any:
		return typ == "object"
	}
	return false
}

func compactJSON(t *testing.T, s string) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, []byte(s)); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
