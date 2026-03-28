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
		{"empty input", "", nil, 1, "jsonshape: <stdin>: no JSON input\n"},
		{"truncated", `{"a": [1, 2`, nil, 1, "<stdin>:1:12: unexpected end of JSON input"},
		{"syntax error", `{"a": 1,}`, nil, 1, "<stdin>:1:9: invalid character '}'"},
		{"trailing data", `{} {}`, nil, 1, "<stdin>:1:4: invalid character '{' after top-level value"},
		{"error in file", ``, []string{"testdata/bad.json"}, 1, "testdata/bad.json:3:15: invalid character '}'"},
		{"missing file", ``, []string{"testdata/nope.json"}, 1, "open testdata/nope.json: no such file or directory"},
		{"unknown flag", `{}`, []string{"--nope"}, 2, "flag provided but not defined"},
		{"two files", `{}`, []string{"a.json", "b.json"}, 2, "too many arguments"},
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

func TestRunFile(t *testing.T) {
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
    score: number
    deleted_at?: string
  }
]
`
	for _, args := range [][]string{
		{"testdata/users.json"},
		{"--", "testdata/users.json"},
	} {
		r := runWith(t, "", args...)
		if r.code != 0 || r.stderr != "" {
			t.Fatalf("%q: exit %d, stderr %q", args, r.code, r.stderr)
		}
		if r.stdout != want {
			t.Errorf("%q: got:\n%s\nwant:\n%s", args, r.stdout, want)
		}
	}
}

func TestRunDashIsStdin(t *testing.T) {
	r := runWith(t, `{"a": [1.5]}`, "-")
	if r.code != 0 || r.stdout != "{\n  a: [float]\n}\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestRunFlagAfterFile(t *testing.T) {
	r := runWith(t, "", "testdata/users.json", "--version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "jsonshape ") {
		t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestRunErrorShowsContext(t *testing.T) {
	r := runWith(t, "", "testdata/bad.json")
	want := "jsonshape: testdata/bad.json:3:15: invalid character '}' looking for beginning of object key string\n" +
		"  3 |   \"name\": \"x\",}\n" +
		"    |               ^\n"
	if r.stderr != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.stderr, want)
	}
}
