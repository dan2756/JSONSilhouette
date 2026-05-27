package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dan2756/jsonshape/internal/shape"
)

// syntaxError is invalid JSON in one of the inputs, located by line and
// column.
type syntaxError struct {
	name      string // file name, or <stdin>
	line, col int    // 1-based; col counts characters, not bytes
	msg       string

	excerpt string // the offending line, cut down around the error
	caret   int    // character index of the error within excerpt
}

func (e *syntaxError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", e.name, e.line, e.col, e.msg)
}

// context returns the excerpt with a caret under the error, prefixed with
// the line number.
func (e *syntaxError) context() string {
	num := fmt.Sprint(e.line)
	pad := strings.Repeat(" ", len(num))
	return fmt.Sprintf("  %s | %s\n  %s | %s^\n", num, e.excerpt, pad, strings.Repeat(" ", e.caret))
}

// decodeDocument merges the single JSON value held in data into root.
func decodeDocument(root *shape.Shape, name string, data []byte) error {
	dec := shape.NewDecoder(bytes.NewReader(data))
	err := root.Add(dec)
	if err == io.EOF {
		return fmt.Errorf("%s: no JSON input", name)
	}
	if err != nil {
		return locate(name, data, 1, err)
	}
	end := dec.InputOffset()
	if _, err := dec.Token(); err == io.EOF {
		return nil
	}
	// Something follows the value. If it looks like another value, the
	// input is probably NDJSON.
	e := locate(name, data, 1, nil)
	rest := bytes.TrimLeft(data[end:], " \t\r\n")
	if se, ok := e.(*syntaxError); ok && len(rest) > 0 && bytes.IndexByte([]byte(`{["-0123456789tfn`), rest[0]) >= 0 {
		se.msg += " (use --lines for one JSON value per line)"
	}
	return e
}

// decodeLines merges each line of r into root as a separate sample, as for
// NDJSON (JSON Lines) input. Blank lines are skipped.
func decodeLines(root *shape.Shape, name string, r io.Reader) error {
	br := bufio.NewReader(r)
	for lineNo := 1; ; lineNo++ {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			if err := decodeLine(root, name, line, lineNo); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
}

// decodeLine merges the one JSON value on line lineNo into root.
func decodeLine(root *shape.Shape, name string, line []byte, lineNo int) error {
	dec := shape.NewDecoder(bytes.NewReader(line))
	if err := root.Add(dec); err != nil {
		return locate(name, line, lineNo, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return locate(name, line, lineNo, nil)
	}
	return nil
}

// locate reports the first error in data, which is known not to hold
// exactly one valid JSON value, as a syntaxError. decodeErr is the error the
// inference decoder stopped with, and firstLine the line number of the
// first byte of data.
//
// The decoder's own offsets are relative to the last value it buffered, so
// the error is found again with a fresh scan of the whole input; this only
// happens on the error path.
func locate(name string, data []byte, firstLine int, decodeErr error) error {
	off, msg := len(data), "invalid JSON"
	var se *json.SyntaxError
	err := json.Unmarshal(data, new(json.RawMessage))
	switch {
	case errors.Is(decodeErr, io.ErrUnexpectedEOF),
		errors.As(err, &se) && se.Error() == "unexpected end of JSON input":
		// Point just past the last character rather than at the end of
		// whatever whitespace follows it.
		off, msg = len(bytes.TrimRight(data, " \t\r\n")), "unexpected end of JSON input"
	case errors.As(err, &se):
		off, msg = int(se.Offset)-1, se.Error() // Offset counts the bad byte
	case err != nil:
		msg = err.Error()
	}
	e := &syntaxError{name: name, msg: msg}
	e.line, e.col, e.excerpt, e.caret = position(data, off)
	e.line += firstLine - 1
	return e
}

// Excerpts keep this many characters before and after the error column.
const excerptBefore, excerptAfter = 40, 30

// position returns the 1-based line and column of byte offset off in data,
// and an excerpt of that line around it with the column's index in the
// excerpt.
func position(data []byte, off int) (line, col int, excerpt string, caret int) {
	off = max(0, min(off, len(data)))
	lineStart := bytes.LastIndexByte(data[:off], '\n') + 1
	lineEnd := len(data)
	if i := bytes.IndexByte(data[off:], '\n'); i >= 0 {
		lineEnd = off + i
	}
	if lineEnd > off && data[lineEnd-1] == '\r' {
		lineEnd--
	}
	line = 1 + bytes.Count(data[:lineStart], []byte{'\n'})
	col = 1 + utf8.RuneCount(data[lineStart:off])

	start := off
	for i := 0; i < excerptBefore && start > lineStart; i++ {
		_, size := utf8.DecodeLastRune(data[lineStart:start])
		start -= size
	}
	end := off
	for i := 0; i < excerptAfter && end < lineEnd; i++ {
		_, size := utf8.DecodeRune(data[end:lineEnd])
		end += size
	}
	before, after := printable(data[start:off]), printable(data[off:end])
	if start > lineStart {
		before = "…" + before
	}
	if end < lineEnd {
		after += "…"
	}
	return line, col, before + after, utf8.RuneCountInString(before)
}

// printable returns b as valid UTF-8 with exactly one character per rune
// of b, so that a caret placed under it lines up: invalid bytes become
// U+FFFD and tabs and other control characters become spaces.
func printable(b []byte) string {
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if unicode.IsControl(r) {
			r = ' '
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
