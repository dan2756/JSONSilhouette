package shape

import (
	"strings"
	"testing"
)

// infer merges each of docs into a new shape as a separate sample.
func infer(t *testing.T, docs ...string) *Shape {
	t.Helper()
	s := New()
	for _, doc := range docs {
		if err := s.Add(NewDecoder(strings.NewReader(doc))); err != nil {
			t.Fatalf("Add(%q): %v", doc, err)
		}
	}
	return s
}

func render(t *testing.T, s *Shape, opts Options) string {
	t.Helper()
	var b strings.Builder
	if err := Render(&b, s, opts); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"int", `42`, "int"},
		{"negative int", `-7`, "int"},
		{"float", `1.5`, "float"},
		{"exponent is float", `1e3`, "float"},
		{"float with zero fraction", `1.0`, "float"},
		{"huge int keeps its kind", `123456789012345678901234567890`, "int"},
		{"string", `"hi"`, "string"},
		{"bool", `false`, "bool"},
		{"null", `null`, "null"},
		{"empty object", `{}`, "{}"},
		{"empty array", `[]`, "[]"},
		{"array of scalars", `[1, 2, 3]`, "[int]"},
		{"int and float merge to number", `[1, 2.5]`, "[number]"},
		{"scalar union", `[null, "a", 1, true]`, "[bool | int | string | null]"},
		{"nested arrays", `[[1], [2, 3], []]`, "[[int]]"},
		{"only empty nested arrays", `[[], []]`, "[[]]"},
		{"array of nulls", `[null, null]`, "[null]"},
		{"array of empty objects", `[{}, {}]`, "[{}]"},
		{
			"object keeps key order",
			`{"zeta": 1, "alpha": "a", "mid": null}`,
			"{\n  zeta: int\n  alpha: string\n  mid: null\n}",
		},
		{
			"nested object",
			`{"owner": {"login": "x", "site_admin": false}}`,
			"{\n  owner: {\n    login: string\n    site_admin: bool\n  }\n}",
		},
		{
			"array of objects",
			`[{"id": 1}, {"id": 2}]`,
			"[\n  {\n    id: int\n  }\n]",
		},
		{
			"optional key",
			`[{"id": 1, "deleted_at": "x"}, {"id": 2}]`,
			"[\n  {\n    id: int\n    deleted_at?: string\n  }\n]",
		},
		{
			"keys ordered by first appearance across elements",
			`[{"b": 1}, {"a": 1, "b": 2}, {"c": 3, "a": 4}]`,
			"[\n  {\n    b?: int\n    a?: int\n    c?: int\n  }\n]",
		},
		{
			"nullable field",
			`[{"email": "a@b.c"}, {"email": null}]`,
			"[\n  {\n    email: string | null\n  }\n]",
		},
		{
			"nullable object",
			`[{"o": {"a": 1}}, {"o": null}]`,
			"[\n  {\n    o: {\n      a: int\n    } | null\n  }\n]",
		},
		{
			"mixed array of strings and objects",
			`["a", {"x": 1}, "b"]`,
			"[\n  string | {\n    x: int\n  }\n]",
		},
		{
			"array of arrays of objects",
			`[[{"a": 1}], [{"a": 2, "b": true}]]`,
			"[\n  [\n    {\n      a: int\n      b?: bool\n    }\n  ]\n]",
		},
		{
			"empty array merged with a full one",
			`[{"tags": []}, {"tags": ["x"]}]`,
			"[\n  {\n    tags: [string]\n  }\n]",
		},
		{
			"repeated key counts once per object",
			`[{"a": 1, "a": 2}, {}]`,
			"[\n  {\n    a?: int\n  }\n]",
		},
		{
			"odd keys are quoted",
			`{"foo bar": 1, "": 2, "content-type": "x", "@timestamp": "t", "a:b": 3, "<tag>": 4}`,
			"{\n  \"foo bar\": int\n  \"\": int\n  content-type: string\n  @timestamp: string\n  \"a:b\": int\n  \"<tag>\": int\n}",
		},
		{
			"unicode keys stay bare",
			`{"größe": 1, "名前": "x"}`,
			"{\n  größe: int\n  名前: string\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, infer(t, tt.in), Options{})
			if want := tt.want + "\n"; got != want {
				t.Errorf("Render(%s)\ngot:\n%s\nwant:\n%s", tt.in, got, want)
			}
		})
	}
}

func TestSamplesMerge(t *testing.T) {
	s := infer(t, `{"id": 1, "name": "a"}`, `{"id": 2.5}`, `{"id": 3, "extra": [true]}`)
	want := "{\n  id: number\n  name?: string\n  extra?: [bool]\n}\n"
	if got := render(t, s, Options{}); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCounts(t *testing.T) {
	s := infer(t, `[{"a": 1, "b": null}, {"a": "x"}, {"a": 2, "a": 3}, 7]`)
	if s.Count != 1 || s.Array == nil || s.Array.Count != 1 {
		t.Fatalf("root: %+v", s)
	}
	items := s.Array.Items
	if items.Count != 4 || items.Int != 1 || items.Object == nil || items.Object.Count != 3 {
		t.Fatalf("items: %+v", items)
	}
	o := items.Object
	if len(o.Fields) != 2 {
		t.Fatalf("fields: %d, want 2", len(o.Fields))
	}
	a, b := o.Fields[0], o.Fields[1]
	if a.Key != "a" || a.Count != 3 || o.Optional(a) {
		t.Errorf("a: key=%q count=%d optional=%v", a.Key, a.Count, o.Optional(a))
	}
	if a.Shape.Count != 4 || a.Shape.Int != 3 || a.Shape.String != 1 {
		t.Errorf("a shape: %+v", a.Shape)
	}
	if b.Key != "b" || b.Count != 1 || !o.Optional(b) || b.Shape.Null != 1 {
		t.Errorf("b: key=%q count=%d optional=%v", b.Key, b.Count, o.Optional(b))
	}
}

func TestAddErrors(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{``, "EOF"},
		{`   `, "EOF"},
		{`{"a": 1`, "unexpected EOF"},
		{`[1, 2`, "unexpected EOF"},
		{`[1, tr`, "unexpected EOF"},
		{`{"a": 1,}`, "invalid character '}'"},
		{`[1 2]`, "invalid character '2' after array element"},
		{`{1: 2}`, "invalid character '1'"},
	}
	for _, tt := range tests {
		err := New().Add(NewDecoder(strings.NewReader(tt.in)))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Add(%q) = %v, want error containing %q", tt.in, err, tt.want)
		}
	}
}

func TestRenderColor(t *testing.T) {
	s := infer(t, `[{"id": 1, "ok": true, "name": "x", "x": null}, {"id": 2.5, "ok": false}]`)
	const (
		blue   = "\x1b[34m"
		yellow = "\x1b[33m"
		green  = "\x1b[32m"
		cyan   = "\x1b[36m"
		purple = "\x1b[35m"
		gray   = "\x1b[90m"
		reset  = "\x1b[0m"
	)
	want := "[\n  {\n" +
		"    " + blue + "id" + reset + ": " + cyan + "number" + reset + "\n" +
		"    " + blue + "ok" + reset + ": " + purple + "bool" + reset + "\n" +
		"    " + blue + "name" + reset + yellow + "?" + reset + ": " + green + "string" + reset + "\n" +
		"    " + blue + "x" + reset + yellow + "?" + reset + ": " + gray + "null" + reset + "\n" +
		"  }\n]\n"
	if got := render(t, s, Options{Color: true}); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
