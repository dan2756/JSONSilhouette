package shape

import (
	"bufio"
	"io"
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

// line is one line of output, built up from segments.
type line struct {
	segs []seg
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

// writeLines writes lines to w, coloring them with ANSI escapes if color is
// set.
func writeLines(w io.Writer, lines []line, color bool) error {
	bw := bufio.NewWriter(w)
	for _, l := range lines {
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
