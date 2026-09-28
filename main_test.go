package main

import (
	"encoding/json"
	"fmt"
	"io"
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
		{"negative depth", `{}`, []string{"--depth", "-1"}, 2, "invalid --depth -1"},
		{"bad depth", `{}`, []string{"--depth", "two"}, 2, `invalid value "two" for flag -depth`},
		{"schema and paths", `{}`, []string{"--json-schema", "--paths"}, 2, "cannot be combined"},
		{"schema and counts", `{}`, []string{"--counts", "--json-schema"}, 2, "cannot be combined"},
		{"empty lines input", "\n\n", []string{"--lines"}, 1, "jsonshape: no JSON input\n"},
		{"error in second file", `{}`, []string{"-", "testdata/bad.json"}, 1, "testdata/bad.json:3:15:"},
		{"ndjson without --lines", "{}\n{}\n", nil, 1, "(use --lines for one JSON value per line)"},
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

func TestRunColor(t *testing.T) {
	orig := isTerminal
	isTerminal = func(io.Writer) bool { return true }
	t.Cleanup(func() { isTerminal = orig })
	t.Setenv("TERM", "xterm-256color")

	colored := "{\n  \x1b[34ma\x1b[0m: \x1b[36mint\x1b[0m\n}\n"
	plain := "{\n  a: int\n}\n"
	tests := []struct {
		name    string
		noColor string
		args    []string
		want    string
	}{
		{"terminal", "", nil, colored},
		{"--no-color", "", []string{"--no-color"}, plain},
		{"NO_COLOR", "1", nil, plain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			r := runWith(t, `{"a": 1}`, tt.args...)
			if r.stdout != tt.want {
				t.Errorf("got %q, want %q", r.stdout, tt.want)
			}
		})
	}
}

func TestRunLines(t *testing.T) {
	want := `{
  ts: string
  level: string
  msg: string
  port?: int
  method?: string
  path?: string
  status?: int
  duration_ms?: number
  error?: {
    code: string
    retryable: bool
  }
}
`
	r := runWith(t, "", "--lines", "testdata/events.ndjson")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if r.stdout != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.stdout, want)
	}
}

func TestRunMergesFiles(t *testing.T) {
	r := runWith(t, `{"id": "x", "extra": true}`, "testdata/users.json", "-")
	want := "[\n  {\n    id: int\n" // the array's shape comes first
	if r.code != 0 || !strings.HasPrefix(r.stdout, want) {
		t.Fatalf("exit %d, stderr %q, stdout:\n%s", r.code, r.stderr, r.stdout)
	}
	if !strings.HasSuffix(r.stdout, "] | {\n  id: string\n  extra: bool\n}\n") {
		t.Errorf("stdout:\n%s", r.stdout)
	}
}

func TestRunPathsAndDepth(t *testing.T) {
	want := `.                array
.[]              object
.[].id           int
.[].name         string
.[].email        string | null
.[].tags         array
.[].owner        object
.[].score        number
.[].deleted_at?  string
`
	r := runWith(t, "", "testdata/users.json", "--paths", "--depth=2")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if r.stdout != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.stdout, want)
	}
}

func TestRunCountsLines(t *testing.T) {
	want := `{                       # 4 samples
  ts: string            # 4/4
  level: string         # 4/4
  msg: string           # 4/4
  port?: int            # 1/4
  method?: string       # 3/4
  path?: string         # 3/4
  status?: int          # 3/4
  duration_ms?: number  # 3/4 · int 1, float 2
  error?: {             # 1/4
    code: string        # 1/1
    retryable: bool     # 1/1
  }
}
`
	r := runWith(t, "", "--counts", "--lines", "testdata/events.ndjson")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if r.stdout != want {
		t.Errorf("got:\n%s\nwant:\n%s", r.stdout, want)
	}
}

func TestRunJSONSchema(t *testing.T) {
	orig := isTerminal
	isTerminal = func(io.Writer) bool { return true } // schemas are never colored
	t.Cleanup(func() { isTerminal = orig })

	r := runWith(t, "", "--json-schema", "testdata/users.json")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if strings.Contains(r.stdout, "\x1b[") {
		t.Errorf("schema contains color escapes:\n%s", r.stdout)
	}
	var schema struct {
		Schema string `json:"$schema"`
		Type   string `json:"type"`
		Items  struct {
			Properties map[string]struct {
				Type any `json:"type"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &schema); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if schema.Schema != "https://json-schema.org/draft/2020-12/schema" || schema.Type != "array" {
		t.Errorf("$schema %q, type %q", schema.Schema, schema.Type)
	}
	if got := strings.Join(schema.Items.Required, ","); got != "id,name,email,tags,owner,score" {
		t.Errorf("required = %s", got)
	}
	if got := fmt.Sprint(schema.Items.Properties["email"].Type); got != "[string null]" {
		t.Errorf("email type = %s", got)
	}
}
