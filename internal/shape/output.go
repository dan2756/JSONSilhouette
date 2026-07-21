package shape

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// style is the color class of a piece of output.
type style uint8

const (
	plain style = iota
	keyStyle
	optionalStyle
	stringStyle
	numberStyle
	boolStyle
	nullStyle
	dimStyle
)

var ansi = [...]string{
	keyStyle:      "34",
	optionalStyle: "33",
	stringStyle:   "32",
	numberStyle:   "36",
	boolStyle:     "35",
	nullStyle:     "90",
	dimStyle:      "90",
}

func (k kind) style() style {
	switch k {
	case kindBool:
		return boolStyle
	case kindInt, kindFloat, kindNumber:
		return numberStyle
	case kindString:
		return stringStyle
	case kindNull:
		return nullStyle
	}
	return plain
}

// seg is a run of text in one style.
type seg struct {
	text string
	st   style
}

// line is one line of output, built up from segments, with an optional
// note that is printed as an aligned comment after it.
type line struct {
	segs []seg
	note string
}

func (l *line) add(text string, st style) {
	l.segs = append(l.segs, seg{text, st})
}

// width is the length of l in characters, ignoring color.
func (l *line) width() int {
	n := 0
	for _, s := range l.segs {
		n += utf8.RuneCountInString(s.text)
	}
	return n
}

// Notes start two spaces after the longest line that has one, but no
// further right than this.
const maxNoteColumn = 64

// writeLines writes lines to w, coloring them with ANSI escapes if color is
// set. Notes are lined up in one column as "# note" comments.
func writeLines(w io.Writer, lines []line, color bool) error {
	col := 0
	for _, l := range lines {
		if l.note != "" {
			col = max(col, l.width())
		}
	}
	col = min(col, maxNoteColumn) + 2

	bw := bufio.NewWriter(w)
	for _, l := range lines {
		if l.note != "" {
			l.segs = append(l.segs[:len(l.segs):len(l.segs)],
				seg{strings.Repeat(" ", max(2, col-l.width())), plain},
				seg{"# " + l.note, dimStyle})
		}
		for _, s := range l.segs {
			if !color || s.st == plain {
				bw.WriteString(s.text)
				continue
			}
			bw.WriteString("\x1b[")
			bw.WriteString(ansi[s.st])
			bw.WriteString("m")
			bw.WriteString(s.text)
			bw.WriteString("\x1b[0m")
		}
		bw.WriteByte('\n')
	}
	return bw.Flush()
}

// presence describes how many of total objects held a key: "2/3".
func presence(n, total int) string {
	return fmt.Sprintf("%d/%d", n, total)
}

// countOf returns "1 item", "2 items" and so on.
func countOf(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// breakdown lists how many values of each type s held, when it held more
// than one: "string 2, null 1". Ints and floats are always counted apart.
func breakdown(s *Shape) string {
	ms := s.members(true)
	if len(ms) < 2 {
		return ""
	}
	parts := make([]string, len(ms))
	for i, m := range ms {
		parts[i] = fmt.Sprintf("%s %d", m.kind, m.count)
	}
	return strings.Join(parts, ", ")
}

// joinNote joins the non-empty parts of a note.
func joinNote(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(p)
	}
	return b.String()
}

// samples describes the number of top-level values, when there is more
// than one.
func samples(s *Shape) string {
	if s.Count < 2 {
		return ""
	}
	return countOf(s.Count, "sample")
}
