package main

import (
	"strings"
	"testing"
)

type result struct {
	code           int
	stdout, stderr string
}

func runWith(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	var out, errb strings.Builder
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return result{code, out.String(), errb.String()}
}

func TestRunStdin(t *testing.T) {
	in := `[
  {"id": 1, "name": "Ada", "email": "ada@example.com", "tags": ["admin"], "owner": {"login": "ada", "site_admin": true}},
  {"id": 2, "name": "Bo", "email": null, "tags": [], "owner": {"login": "bo", "site_admin": false}, "deleted_at": "2026-01-02"}
]`
	want := `[
  {
    id: int
    name: string
    email: string | null
    tags: [string]
    owner: {
      login: string
      site_admin: bool
    }
    deleted_at?: string
  }
]
`
	r := runWith(t, in)
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if r.stdout != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.stdout, want)
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name   string
		stdin  string
		args   []string
		code   int
		stderr string
	}{
		{"empty input", "", nil, 1, "no JSON input"},
		{"truncated", `{"a": [1, 2`, nil, 1, "unexpected end of JSON input"},
		{"syntax error", `{"a": 1,}`, nil, 1, "invalid character '}'"},
		{"trailing data", `{} {}`, nil, 1, "after top-level value"},
		{"unknown flag", `{}`, []string{"--nope"}, 2, "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runWith(t, tt.stdin, tt.args...)
			if r.code != tt.code {
				t.Errorf("exit code %d, want %d", r.code, tt.code)
			}
			if !strings.Contains(r.stderr, tt.stderr) {
				t.Errorf("stderr %q, want it to contain %q", r.stderr, tt.stderr)
			}
			if r.stdout != "" {
				t.Errorf("unexpected stdout %q", r.stdout)
			}
		})
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		r := runWith(t, "", arg)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "Usage: jsonshape") {
			t.Errorf("%s: exit %d, stdout %q", arg, r.code, r.stdout)
		}
	}
	r := runWith(t, "", "--version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "jsonshape ") {
		t.Errorf("--version: exit %d, stdout %q", r.code, r.stdout)
	}
}
