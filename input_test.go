package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/dan2756/jsonshape/internal/shape"
)

func TestDecodeDocumentErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string // Error() of the result
	}{
		{"trailing comma", "{\"a\": 1,}", `x:1:9: invalid character '}' looking for beginning of object key string`},
		{"second line", "{\n  \"a\": tru}", `x:2:11: invalid character '}' in literal true (expecting 'e')`},
		{"bad escape", "[\"ok\",\n \"\\q\"]", `x:2:4: invalid character 'q' in string escape code`},
		{"missing colon", "{\"a\" 1}", `x:1:6: invalid character '1' after object key`},
		{"leading zero", "[01]", `x:1:3: invalid character '1' after array element`},
		{"truncated", "{\"a\": [1, 2\n\n", `x:1:12: unexpected end of JSON input`},
		{"truncated literal", "[nul", `x:1:5: unexpected end of JSON input`},
		{"trailing value", "{}\n{}", `x:2:1: invalid character '{' after top-level value (use --lines for one JSON value per line)`},
		{"trailing garbage", "[1]]", `x:1:4: invalid character ']' after top-level value`},
		{"truncated trailing value", "[1] \"abc", `x:1:5: invalid character '"' after top-level value (use --lines for one JSON value per line)`},
		{"columns count characters", "{\"größe\": x}", `x:1:11: invalid character 'x' looking for beginning of value`},
		{"crlf", "{\r\n\"a\":1,\r\n}", `x:3:1: invalid character '}' looking for beginning of object key string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeDocument(shape.New(), "x", []byte(tt.in))
			var se *syntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %v (%T), want a syntax error", err, err)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestDecodeLines(t *testing.T) {
	in := "{\"a\": 1}\n\n  \n{\"a\": 2.5, \"b\": [true]}\r\n{\"b\": []}" // no final newline
	root := shape.New()
	if err := decodeLines(root, "x", strings.NewReader(in)); err != nil {
		t.Fatal(err)
	}
	if root.Count != 3 {
		t.Errorf("samples = %d, want 3", root.Count)
	}
	var b strings.Builder
	if err := shape.Render(&b, root, shape.Options{}); err != nil {
		t.Fatal(err)
	}
	if want := "{\n  a?: number\n  b?: [bool]\n}\n"; b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestDecodeLinesErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"bad third line", "{}\n[]\n{\"a\" 1}\n{}\n", `x:3:6: invalid character '1' after object key`},
		{"blank lines still count", "{}\n\n\n{,}\n", `x:4:2: invalid character ',' looking for beginning of object key string`},
		{"two values on a line", "{}\n{} {}\n", `x:2:4: invalid character '{' after top-level value`},
		{"value split over lines", "{\n\"a\": 1}\n", `x:1:2: unexpected end of JSON input`},
		{"truncated last line", "{\"a\": 1}\n{\"a\": ", `x:2:6: unexpected end of JSON input`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := decodeLines(shape.New(), "x", strings.NewReader(tt.in))
			var se *syntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %v (%T), want a syntax error", err, err)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestDecodeDocumentEmpty(t *testing.T) {
	for _, in := range []string{"", " \n\t "} {
		err := decodeDocument(shape.New(), "x", []byte(in))
		if err == nil || err.Error() != "x: no JSON input" {
			t.Errorf("decodeDocument(%q) = %v", in, err)
		}
	}
}

func TestErrorContext(t *testing.T) {
	err := decodeDocument(shape.New(), "x", []byte("{\n  \"id\": 1,\n  \"name\": \"x\",}\n"))
	var se *syntaxError
	if !errors.As(err, &se) {
		t.Fatalf("got %v", err)
	}
	want := "" +
		"  3 |   \"name\": \"x\",}\n" +
		"    |               ^\n"
	if got := se.context(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestErrorContextLongLine(t *testing.T) {
	in := `{"a": [` + strings.Repeat(`1, `, 100) + `x` + strings.Repeat(`, 2`, 100) + `]}`
	err := decodeDocument(shape.New(), "x", []byte(in))
	var se *syntaxError
	if !errors.As(err, &se) {
		t.Fatalf("got %v", err)
	}
	if se.col != 308 {
		t.Errorf("col = %d, want 308", se.col)
	}
	want := "" +
		"  1 | … 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, x, 2, 2, 2, 2, 2, 2, 2, 2, 2, …\n" +
		"    |                                          ^\n"
	if got := se.context(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintable(t *testing.T) {
	if got := printable([]byte("a\tb\xffc")); got != "a b\uFFFDc" {
		t.Errorf("printable = %q", got)
	}
}
