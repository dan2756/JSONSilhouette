package shape

import (
	"io"
	"slices"
	"strings"
)

// The type column of RenderPaths starts after the longest path, but no
// further right than this.
const maxPathColumn = 48

// RenderPaths writes s to w with one line per position: a jq-style path
// and the type found there.
//
//	.                array
//	.[]              object
//	.[].id           int
//	.[].tags         array
//	.[].tags[]       string
//	.[].owner        object | null
//	.[].owner.login  string
//	.[].deleted_at?  string
//
// Keys missing from some objects carry a "?", which keeps the path valid
// for jq, and so does the first [] below such a key: jq gives null for a
// missing key but cannot iterate over it.
func RenderPaths(w io.Writer, s *Shape, opts Options) error {
	p := pathRenderer{opts: opts}
	p.walk(nil, s, 1, samples(s), false)

	col := 0
	for _, e := range p.entries {
		col = max(col, e.path.width())
	}
	col = min(col, maxPathColumn) + 2

	lines := make([]line, len(p.entries))
	for i, e := range p.entries {
		l := line{segs: slices.Clone(e.path.segs), note: e.note}
		l.add(strings.Repeat(" ", max(2, col-l.width())), plain)
		l.segs = append(l.segs, e.typ.segs...)
		lines[i] = l
	}
	return writeLines(w, lines, opts.Color)
}

type pathEntry struct {
	path, typ line
	note      string
}

type pathRenderer struct {
	opts    Options
	entries []pathEntry
}

// walk lists the position at path, holding s at the given nesting level,
// and everything below it. The root's path is empty and printed as ".".
// first leads the position's --counts note. missing is set when path goes
// through an optional key, so that jq may give null for it; the [] below
// is then written []? to skip that null instead of failing on it.
func (p *pathRenderer) walk(path []seg, s *Shape, level int, first string, missing bool) {
	e := pathEntry{path: line{segs: path}, typ: typeSummary(s, p.opts.SplitNumbers)}
	if len(path) == 0 {
		e.path.add(".", plain)
	}
	if p.opts.Counts {
		e.note = joinNote(first, breakdown(s))
	}
	p.entries = append(p.entries, e)
	if p.opts.collapsed(level) {
		return
	}
	if o := s.Object; o != nil {
		for _, f := range o.Fields {
			child := childKey(path, f.Key)
			if o.Optional(f) {
				child = append(child, seg{"?", optionalStyle})
			}
			p.walk(child, f.Shape, level+1, presence(f.Count, o.Count), missing || o.Optional(f))
		}
	}
	if a := s.Array; a != nil && a.Items.Count > 0 {
		text := "[]"
		if len(path) == 0 {
			text = ".[]"
		}
		child := append(slices.Clip(path), seg{text, plain})
		if missing {
			child = append(child, seg{"?", optionalStyle})
		}
		p.walk(child, a.Items, level+1, countOf(a.Items.Count, "item"), false)
	}
}

// childKey returns the path of key inside the object at path: .key for
// identifiers and ["key"] for anything else.
func childKey(path []seg, key string) []seg {
	var text string
	switch {
	case isIdent(key):
		text = "." + key
	case len(path) == 0:
		text = ".[" + quote(key) + "]"
	default:
		text = "[" + quote(key) + "]"
	}
	return append(slices.Clip(path), seg{text, plain})
}

// isIdent reports whether key can follow a dot in a jq path.
func isIdent(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range key {
		switch {
		case c == '_', 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case '0' <= c && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// typeSummary names the types in s on one line, with containers written
// as object and array, or {} and [] when they were always empty.
func typeSummary(s *Shape, splitNumbers bool) line {
	var l line
	for i, m := range s.members(splitNumbers) {
		if i > 0 {
			l.add(" | ", plain)
		}
		switch {
		case m.kind == kindObject && len(s.Object.Fields) == 0:
			l.add("{}", plain)
		case m.kind == kindArray && s.Array.Items.Count == 0:
			l.add("[]", plain)
		default:
			l.add(m.kind.String(), m.kind.style())
		}
	}
	return l
}
