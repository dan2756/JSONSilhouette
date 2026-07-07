package shape

import (
	"strings"
	"testing"
)

func renderPaths(t *testing.T, s *Shape, opts Options) string {
	t.Helper()
	var b strings.Builder
	if err := RenderPaths(&b, s, opts); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRenderPaths(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"scalar", `"x"`, []string{".  string"}},
		{"empty containers", `{"a": [], "b": {}}`, []string{
			".   object",
			".a  []",
			".b  {}",
		}},
		{"users", `[{"id": 1, "tags": ["x"], "owner": {"login": "a"}}, {"id": 2, "tags": [], "owner": null, "deleted_at": "t"}]`, []string{
			".                array",
			".[]              object",
			".[].id           int",
			".[].tags         array",
			".[].tags[]       string",
			".[].owner        object | null",
			".[].owner.login  string",
			".[].deleted_at?  string",
		}},
		{"optional parent marks its children", `[{"a": {"b": 1}}, {}]`, []string{
			".         array",
			".[]       object",
			".[].a?    object",
			".[].a?.b  int",
		}},
		{"nested arrays", `[[1, 2.5], [null]]`, []string{
			".      array",
			".[]    array",
			".[][]  number | null",
		}},
		{"keys that need quoting", `{"a b": {"c-d": 1, "": 2, "_x9": 3, "9x": 4}}`, []string{
			".                object",
			`.["a b"]         object`,
			`.["a b"]["c-d"]  int`,
			`.["a b"][""]     int`,
			`.["a b"]._x9     int`,
			`.["a b"]["9x"]   int`,
		}},
		{"mixed array", `["s", {"k": true}]`, []string{
			".      array",
			".[]    string | object",
			".[].k  bool",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderPaths(t, infer(t, tt.in), Options{})
			if want := strings.Join(tt.want, "\n") + "\n"; got != want {
				t.Errorf("RenderPaths(%s)\ngot:\n%s\nwant:\n%s", tt.in, got, want)
			}
		})
	}
}

func TestRenderPathsDepth(t *testing.T) {
	s := infer(t, `{"a": {"b": {"c": 1}}, "x": [[true]]}`)
	want := strings.Join([]string{
		".     object",
		".a    object",
		".a.b  object",
		".x    array",
		".x[]  array",
	}, "\n") + "\n"
	if got := renderPaths(t, s, Options{Depth: 2}); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderPathsLongPathsDoNotPushTheColumn(t *testing.T) {
	long := strings.Repeat("k", 60)
	s := infer(t, `{"`+long+`": 1, "a": 2}`)
	want := "." + strings.Repeat(" ", maxPathColumn+1) + "object\n" +
		"." + long + "  int\n" +
		".a" + strings.Repeat(" ", maxPathColumn) + "int\n"
	if got := renderPaths(t, s, Options{}); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderPathsColor(t *testing.T) {
	s := infer(t, `[{"a": "x"}, {}]`)
	want := ".       array\n" +
		".[]     object\n" +
		".[].a\x1b[33m?\x1b[0m  \x1b[32mstring\x1b[0m\n"
	if got := renderPaths(t, s, Options{Color: true}); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
