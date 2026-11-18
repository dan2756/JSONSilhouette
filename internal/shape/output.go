package shape

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
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

// width is the length of l in terminal columns, ignoring color.
func (l *line) width() int {
	n := 0
	for _, s := range l.segs {
		n += DisplayWidth(s.text)
	}
	return n
}

// DisplayWidth reports how many columns s takes up on a terminal: two for
// each wide character, none for combining marks and invisible format
// characters, and one for anything else. A sequence terminals draw as one
// emoji is two columns: a skin-tone modifier adds nothing, a variation
// selector widens a narrow emoji base, and a ZWJ sequence does not grow.
// Error carets and the aligned columns of --counts and --paths both use it.
func DisplayWidth(s string) int {
	n := 0
	wideGrapheme := false
	join := false
	var base rune
	haveBase := false
	for _, r := range s {
		switch {
		case r == 0x200D: // zero-width joiner
			join = haveBase && (wideGrapheme || emojiLike(base))
			continue
		case isEmojiModifier(r), r == 0xFE0E: // skin tone, text presentation
			continue
		case r == 0xFE0F, r == 0x20E3: // emoji presentation, keycap
			if haveBase && !wideGrapheme && (r == 0x20E3 || emojiLike(base)) {
				n++
				wideGrapheme = true
			}
			continue
		case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
			continue
		case join && emojiLike(r):
			join = false
			if !wideGrapheme {
				n++
				wideGrapheme = true
			}
			base = r
			haveBase = true
			continue
		case unicode.Is(wide, r):
			join = false
			n += 2
			wideGrapheme = true
			base = r
			haveBase = true
		default:
			join = false
			n++
			wideGrapheme = false
			base = r
			haveBase = true
		}
	}
	return n
}

// isEmojiModifier reports whether r is a Fitzpatrick skin-tone modifier.
// Those sit inside the pictograph block but take no column of their own.
func isEmojiModifier(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

// EscapeBidi returns s with bidi controls written as \uXXXX. Left raw, they
// reorder the rest of a terminal line, so a key or an error excerpt can
// hide the character a caret points at. Variation selectors and joiners
// are not controls of this kind and are left alone.
func EscapeBidi(s string) string {
	if !strings.ContainsFunc(s, isBidiControl) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 6)
	for _, r := range s {
		if isBidiControl(r) {
			fmt.Fprintf(&b, `\u%04x`, r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isBidiControl(r rune) bool {
	switch r {
	case 0x061C, 0x200E, 0x200F,
		0x202A, 0x202B, 0x202C, 0x202D, 0x202E,
		0x2066, 0x2067, 0x2068, 0x2069:
		return true
	}
	return false
}

// emojiLike reports whether r takes part in an emoji sequence. Most
// pictographs are in wide; the ranges here are text-presentation symbols
// that become emoji with a variation selector or a joiner.
func emojiLike(r rune) bool {
	if isEmojiModifier(r) || unicode.Is(wide, r) {
		return true
	}
	return r >= 0x2300 && r <= 0x23FF ||
		r >= 0x2600 && r <= 0x27BF ||
		r >= 0x2B00 && r <= 0x2BFF
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
