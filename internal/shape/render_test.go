package shape

import (
	"fmt"
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

func TestCountsAlignByDisplayWidth(t *testing.T) {
	// 名前 is two runes and four columns. Aligning by runes puts the
	// notes in different terminal columns.
	got := render(t, infer(t, `{"名前": "x", "id": 1}`), Options{Counts: true})
	var cols []int
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		i := strings.Index(line, "#")
		if i < 0 {
			continue
		}
		cols = append(cols, DisplayWidth(line[:i]))
	}
	if len(cols) != 2 || cols[0] != cols[1] {
		t.Fatalf("note columns %v\n%s", cols, got)
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

func TestDisplayWidthEmoji(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"id", 2},
		{"名前", 4},
		{"🎉", 2},
		{"👍", 2},
		{"👍🏽", 2},
		{"❤", 1},
		{"❤️", 2},
		{"🇺🇸", 2},
		{"👨‍👩‍👧‍👦", 2},
		{"👨‍⚕️", 2},
		{"1️⃣", 2},
		{"a\u200db", 2},
		{"a\u200d🎉", 3},
		{"e\u0301", 1},
		{"ｆ", 2},
	}
	for _, tt := range tests {
		if got := DisplayWidth(tt.in); got != tt.want {
			t.Errorf("DisplayWidth(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestRenderCountsEmojiWidth(t *testing.T) {
	// A skin-tone emoji and a heart with a variation selector are each two
	// columns, so the notes land in the same place as for id.
	for _, key := range []string{"\U0001F44D\U0001F3FD", "\u2764\ufe0f"} {
		got := render(t, infer(t, `{"`+key+`": 1, "id": 2}`), Options{Counts: true})
		want := "{\n  \"" + key + "\": int  # 1/1\n  id: int    # 1/1\n}\n"
		if got != want {
			t.Errorf("key %q\ngot:\n%s\nwant:\n%s", key, got, want)
		}
	}
}

func TestEscapeBidi(t *testing.T) {
	for _, r := range []rune{
		0x061C, 0x200E, 0x200F,
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
		0x2066, 0x2067, 0x2068, 0x2069,
	} {
		got := EscapeBidi("a" + string(r) + "b")
		want := fmt.Sprintf(`a\u%04xb`, r)
		if got != want {
			t.Errorf("%U: got %q, want %q", r, got, want)
		}
	}
	if EscapeBidi("abc\u200d\ufe0f") != "abc\u200d\ufe0f" {
		t.Fatal("joiner or variation selector was escaped")
	}
}

func TestRenderBidiKey(t *testing.T) {
	key := "a\u202eb"
	got := render(t, infer(t, `{"`+key+`": 1}`), Options{})
	if strings.ContainsRune(got, '\u202e') {
		t.Fatalf("raw bidi in output:\n%s", got)
	}
	want := "{\n  \"a\\u202eb\": int\n}\n"
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
