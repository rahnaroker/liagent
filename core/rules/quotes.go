package rules

import (
	"unicode"

	"litagent/core/model"
)

// Russian quotation marks: outer «…», nested „…“ (DESIGN.md §4.4). Straight and
// curly quotes are converted; existing guillemets/low-quotes only update depth.
const (
	laquo = '«' // U+00AB outer open
	raquo = '»' // U+00BB outer close
	bdquo = '„' // U+201E inner open
	ldquo = '“' // U+201C inner close (Russian nested closing)
	rdquo = '”' // U+201D straight curly close
)

// ruleQuotes is a container rule: it walks the container's text runs in order,
// sharing open/close state across markup boundaries, so nesting and cross-run
// quotes resolve correctly. This is far more reliable than a per-run regex.
var ruleQuotes = containerRule{
	id: "quotes", name: "Кавычки-ёлочки «…»", category: "typography", level: ConfB,
	apply: func(inlines []model.Inline) []Finding {
		st := &quoteState{}
		m := meta{id: "quotes", category: "typography", level: ConfB}
		var all []Finding
		var rec func([]model.Inline)
		rec = func(xs []model.Inline) {
			for _, x := range xs {
				switch n := x.(type) {
				case *model.Text:
					ns, fs := st.process(m, n.Value)
					all = append(all, fs...)
					n.Value = ns
				case *model.Styled:
					rec(n.Children)
				case *model.Link:
					rec(n.Children)
				}
			}
		}
		rec(inlines)
		return all
	},
}

type quoteState struct {
	depth int
	prev  rune // last emitted rune (0 at container start)
}

func (st *quoteState) process(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	var fs []Finding
	for i, r := range runes {
		repl, changed := st.convert(r)
		if changed {
			fs = append(fs, m.find(string(r), string(repl), runeContext(runes, i, i+1)))
		}
		out = append(out, repl)
		st.prev = repl
	}
	return string(out), fs
}

// convert decides the replacement for a straight quote (converting it) or just
// updates nesting depth for already-typographic quotes.
func (st *quoteState) convert(r rune) (rune, bool) {
	switch r {
	case '"':
		if st.isOpening() {
			glyph := laquo
			if st.depth > 0 {
				glyph = bdquo
			}
			st.depth++
			return glyph, true
		}
		glyph := raquo
		if st.depth >= 2 {
			glyph = ldquo
		}
		if st.depth > 0 {
			st.depth--
		}
		return glyph, true
	case laquo, bdquo:
		st.depth++
		return r, false
	case raquo, ldquo:
		if st.depth > 0 {
			st.depth--
		}
		return r, false
	default:
		return r, false
	}
}

// isOpening decides whether a straight quote opens (true) or closes, based on the
// previously emitted character.
func (st *quoteState) isOpening() bool {
	if st.prev == 0 || unicode.IsSpace(st.prev) {
		return true
	}
	switch st.prev {
	case '(', '[', '{', '<', laquo, bdquo, '—', '–', '-', '/', '*':
		return true
	}
	return false
}
