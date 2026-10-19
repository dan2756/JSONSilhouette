package shape

import (
	"strings"
	"testing"
)

func TestRenderDepth(t *testing.T) {
	in := `{"id": 1, "owner": {"login": "a", "repos": [{"name": "x"}]}, "tags": ["t"], "empty": {}, "none": []}`
	tests := []struct {
		depth int
		want  []string
	}{
		{1, []string{
			"{",
			"  id: int",
			"  owner: {…}",
			"  tags: […]",
			"  empty: {}",
			"  none: []",
			"}",
		}},
		{2, []string{
			"{",
			"  id: int",
			"  owner: {",
			"    login: string",
			"    repos: […]",
			"  }",
			"  tags: [string]",
			"  empty: {}",
			"  none: []",
			"}",
		}},
		{3, []string{
			"{",
			"  id: int",
			"  owner: {",
			"    login: string",
			"    repos: [{…}]",
			"  }",
			"  tags: [string]",
			"  empty: {}",
			"  none: []",
			"}",
		}},
	}
	for _, tt := range tests {
		got := render(t, infer(t, in), Options{Depth: tt.depth})
		if want := strings.Join(tt.want, "\n") + "\n"; got != want {
			t.Errorf("depth %d\ngot:\n%s\nwant:\n%s", tt.depth, got, want)
		}
	}
	// Depth 0 means no limit.
	full := render(t, infer(t, in), Options{})
	if deep := render(t, infer(t, in), Options{Depth: 4}); deep != full {
		t.Errorf("depth 4 differs from no limit:\n%s\nvs\n%s", deep, full)
	}
}

func TestRenderDepthScalarRoot(t *testing.T) {
	if got := render(t, infer(t, `[1]`), Options{Depth: 1}); got != "[int]\n" {
		t.Errorf("got %q", got)
	}
	if got := render(t, infer(t, `[[1]]`), Options{Depth: 1}); got != "[[…]]\n" {
		t.Errorf("got %q", got)
	}
}

func TestRenderCounts(t *testing.T) {
	tests := []struct {
		name string
		docs []string
		want []string
	}{
		{"single scalar has nothing to count", []string{`1`}, []string{"int"}},
		{"samples", []string{`1`, `"x"`, `2`}, []string{
			"int | string  # 3 samples · int 2, string 1",
		}},
		{"keys and types", []string{`[{"id": 1, "email": "a"}, {"id": 2.5, "email": null}, {"id": 3}]`}, []string{
			"[",
			"  {                        # 3 items",
			"    id: number             # 3/3 · int 2, float 1",
			"    email?: string | null  # 2/3 · string 1, null 1",
			"  }",
			"]",
		}},
		{"inline arrays count their items", []string{`{"tags": ["a", "b", 3]}`}, []string{
			"{",
			"  tags: [int | string]  # 1/1 · 3 items (int 1, string 2)",
			"}",
		}},
		{"nested objects", []string{`[{"o": {"a": 1}}, {"o": {"b": true}}, {"o": null}]`}, []string{
			"[",
			"  {             # 3 items",
			"    o: {        # 3/3 · object 2, null 1",
			"      a?: int   # 1/2",
			"      b?: bool  # 1/2",
			"    } | null",
			"  }",
			"]",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, infer(t, tt.docs...), Options{Counts: true})
			if want := strings.Join(tt.want, "\n") + "\n"; got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestRenderCountsColor(t *testing.T) {
	got := render(t, infer(t, `{"a": 1}`), Options{Counts: true, Color: true})
	want := "{\n  \x1b[34ma\x1b[0m: \x1b[36mint\x1b[0m  \x1b[90m# 1/1\x1b[0m\n}\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestNotesStopAtMaxColumn(t *testing.T) {
	long := strings.Repeat("k", 80)
	got := render(t, infer(t, `{"`+long+`": 1, "a": 2}`), Options{Counts: true})
	want := "{\n" +
		"  " + long + ": int  # 1/1\n" +
		"  a: int" + strings.Repeat(" ", maxNoteColumn+2-len("  a: int")) + "# 1/1\n" +
		"}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderSplitNumbers(t *testing.T) {
	tests := []struct {
		in         string
		merged     string
		splitApart string
	}{
		{`[1, 2]`, "[int]", "[int]"},
		{`[1.5]`, "[float]", "[float]"},
		{`[1, 2.5]`, "[number]", "[int | float]"},
		{`[1, 2.5, "x", null]`, "[number | string | null]", "[int | float | string | null]"},
	}
	for _, tt := range tests {
		s := infer(t, tt.in)
		if got := render(t, s, Options{}); got != tt.merged+"\n" {
			t.Errorf("%s: got %q, want %q", tt.in, got, tt.merged)
		}
		if got := render(t, s, Options{SplitNumbers: true}); got != tt.splitApart+"\n" {
			t.Errorf("%s with SplitNumbers: got %q, want %q", tt.in, got, tt.splitApart)
		}
	}
	// --paths follows the same rule; --json-schema always says number.
	s := infer(t, `{"n": [1, 2.5]}`)
	if got := renderPaths(t, s, Options{SplitNumbers: true}); !strings.Contains(got, ".n[]  int | float\n") {
		t.Errorf("paths:\n%s", got)
	}
	if got := compactJSON(t, renderSchema(t, s, Options{SplitNumbers: true})); !strings.Contains(got, `"items":{"type":"number"}`) {
		t.Errorf("schema: %s", got)
	}
}
