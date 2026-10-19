package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	caret   int    // terminal column of the error within excerpt
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

// bom is the UTF-8 byte order mark, which some Windows tools put at the
// start of text files. It is not JSON, so it is skipped.
var bom = []byte("\xef\xbb\xbf")

// decodeDocument merges the single JSON value held in data into root.
func decodeDocument(root *shape.Shape, name string, data []byte) error {
	data = bytes.TrimPrefix(data, bom)
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
		if lineNo == 1 {
			line = bytes.TrimPrefix(line, bom)
		}
		if len(bytes.TrimSpace(line)) > 0 {
			if err := decodeLine(root, name, line, lineNo); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return readError(name, err)
		}
	}
}

// readError adds the input's name to a read error, unless it already
// carries the file's path.
func readError(name string, err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return err
	}
	return fmt.Errorf("%s: %w", name, err)
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
// exactly one valid JSON value (or to nest deeper than shape.MaxDepth), as
// a syntaxError. decodeErr is the error the inference decoder stopped with,
// and firstLine the line number of the first byte of data.
//
// The decoder's own offsets are relative to the last value it buffered, so
// the error is found again with a fresh scan of the whole input; this only
// happens on the error path.
func locate(name string, data []byte, firstLine int, decodeErr error) error {
	off, msg := len(data), "invalid JSON"
	var se *json.SyntaxError
	var de *shape.DepthError
	err := json.Unmarshal(data, new(json.RawMessage))
	switch {
	case errors.As(decodeErr, &de):
		off, msg = int(de.Offset), de.Error()
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
// and an excerpt of that line around it with the terminal column at which
// the error appears in the excerpt.
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
	return line, col, before + after, width(before)
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

// width returns how many columns s takes up on a terminal: two for each
// wide character, none for combining marks and invisible format
// characters, and one for anything else.
func width(s string) int {
	n := 0
	for _, r := range s {
		switch {
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		case unicode.Is(wide, r):
			n += 2
		default:
			n++
		}
	}
	return n
}

// wide holds the characters that terminals show two columns wide: the East
// Asian wide and fullwidth ones and emoji, by block. Symbols elsewhere,
// which are mostly shown one column wide, are left out.
var wide = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x1100, Hi: 0x115f, Stride: 1}, // Hangul Jamo initial consonants
		{Lo: 0x2e80, Hi: 0x303e, Stride: 1}, // CJK radicals, symbols and punctuation
		{Lo: 0x3041, Hi: 0x4dbf, Stride: 1}, // kana, Bopomofo, Hangul Jamo, CJK extension A
		{Lo: 0x4e00, Hi: 0xa4cf, Stride: 1}, // CJK unified ideographs, Yi
		{Lo: 0xac00, Hi: 0xd7a3, Stride: 1}, // Hangul syllables
		{Lo: 0xf900, Hi: 0xfaff, Stride: 1}, // CJK compatibility ideographs
		{Lo: 0xfe10, Hi: 0xfe19, Stride: 1}, // vertical forms
		{Lo: 0xfe30, Hi: 0xfe6f, Stride: 1}, // CJK compatibility forms, small forms
		{Lo: 0xff00, Hi: 0xff60, Stride: 1}, // fullwidth forms
		{Lo: 0xffe0, Hi: 0xffe6, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1f200, Hi: 0x1f64f, Stride: 1}, // enclosed ideographs, pictographs, emoticons
		{Lo: 0x1f680, Hi: 0x1f6ff, Stride: 1}, // transport and map symbols
		{Lo: 0x1f900, Hi: 0x1f9ff, Stride: 1}, // supplemental symbols and pictographs
		{Lo: 0x1fa70, Hi: 0x1faff, Stride: 1}, // symbols and pictographs extended-A
		{Lo: 0x20000, Hi: 0x3fffd, Stride: 1}, // CJK extensions B and later
	},
}
