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
