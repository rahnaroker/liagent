package epub

import (
	"strings"

	"litagent/core/model"
)

// renderDoc produces the full XHTML for one spine document.
func (b *builder) renderDoc(pl *plan, d *doc) string {
	var w strings.Builder
	w.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	w.WriteString(`<!DOCTYPE html>` + "\n")
	w.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + b.lang() + `" lang="` + b.lang() + `">` + "\n")
	w.WriteString("<head>\n")
	w.WriteString(`<meta charset="utf-8"/>` + "\n")
	w.WriteString("<title>" + escText(d.title) + "</title>\n")
	w.WriteString(`<link rel="stylesheet" type="text/css" href="../style.css"/>` + "\n")
	w.WriteString("</head>\n<body>\n")

	r := &renderer{b: b, pl: pl, file: d.file, w: &w}
	switch d.kind {
	case kindCover:
		r.renderCover()
	case kindNotes:
		r.renderNotes(d.secs)
	default: // front matter and chapters
		for _, sec := range d.secs {
			r.renderSection(sec)
		}
	}

	w.WriteString("</body>\n</html>\n")
	return w.String()
}

type renderer struct {
	b    *builder
	pl   *plan
	file string
	w    *strings.Builder
}

func (r *renderer) renderCover() {
	src := r.imgSrc(r.pl.coverImg)
	r.w.WriteString(`<div class="cover"><img src="` + escAttr(src) + `" alt="Обложка"/></div>` + "\n")
}

func (r *renderer) renderNotes(secs []*model.Section) {
	for _, n := range secs {
		r.w.WriteString(`<aside epub:type="footnote" id="` + safeID(n.ID) + `" class="note">` + "\n")
		if len(n.Title) > 0 {
			r.w.WriteString(`<p class="note-title">`)
			r.renderInlines(blocksInline(n.Title))
			r.w.WriteString("</p>\n")
		}
		r.renderBlocks(n.Content)
		r.w.WriteString("</aside>\n")
	}
}

func (r *renderer) renderSection(sec *model.Section) {
	r.w.WriteString("<section")
	if frag := r.pl.fragOf[sec]; frag != "" {
		r.w.WriteString(` id="` + frag + `"`)
	}
	r.w.WriteString(">\n")

	if len(sec.Title) > 0 {
		level := r.pl.depthOf[sec]
		if level == 0 {
			level = 1
		}
		r.renderHeading(sec.Title, level)
	}
	for _, ep := range sec.Epigraphs {
		r.w.WriteString(`<div class="epigraph">` + "\n")
		r.renderBlocks(ep)
		r.w.WriteString("</div>\n")
	}
	if sec.Image != nil {
		r.renderBlock(sec.Image)
	}
	if len(sec.Annotation) > 0 {
		r.w.WriteString(`<div class="annotation">` + "\n")
		r.renderBlocks(sec.Annotation)
		r.w.WriteString("</div>\n")
	}
	r.renderBlocks(sec.Content)
	// Recurse only into children that live in the same file; children pushed to
	// their own file (split point) are rendered by their own document.
	for _, c := range sec.Children {
		if r.pl.fileOf[c] == r.file {
			r.renderSection(c)
		}
	}
	r.w.WriteString("</section>\n")
}

func (r *renderer) renderHeading(title []model.Block, depth int) {
	level := depth
	if level > 6 {
		level = 6
	}
	tag := "h" + string(rune('0'+level))
	r.w.WriteString("<" + tag + ">")
	first := true
	for _, b := range title {
		p, ok := b.(*model.Paragraph)
		if !ok {
			continue
		}
		if !first {
			r.w.WriteString("<br/>")
		}
		r.renderInlines(p.Inlines)
		first = false
	}
	r.w.WriteString("</" + tag + ">\n")
}

// renderBlocks renders a block slice, collapsing consecutive empty lines.
func (r *renderer) renderBlocks(blocks []model.Block) {
	prevEmpty := false
	for _, b := range blocks {
		if _, ok := b.(*model.EmptyLine); ok {
			if prevEmpty {
				continue
			}
			prevEmpty = true
		} else {
			prevEmpty = false
		}
		r.renderBlock(b)
	}
}

func (r *renderer) renderBlock(b model.Block) {
	switch n := b.(type) {
	case *model.Paragraph:
		frag := r.pl.paraFrag[n]
		if frag == "" {
			frag = safeID(n.ID)
		}
		if n.Heading {
			// User-accepted unmarked heading → render as a heading (in the TOC).
			r.w.WriteString("<h4")
			if frag != "" {
				r.w.WriteString(` id="` + frag + `"`)
			}
			r.w.WriteString(` class="detected-heading">`)
			r.renderInlines(n.Inlines)
			r.w.WriteString("</h4>\n")
			return
		}
		r.w.WriteString("<p")
		if frag != "" {
			r.w.WriteString(` id="` + frag + `"`)
		}
		if n.Style != "" {
			r.w.WriteString(` class="st-` + escAttr(n.Style) + `"`)
		}
		r.w.WriteString(">")
		r.renderInlines(n.Inlines)
		r.w.WriteString("</p>\n")
	case *model.Subtitle:
		r.w.WriteString(`<p class="subtitle">`)
		r.renderInlines(n.Inlines)
		r.w.WriteString("</p>\n")
	case *model.EmptyLine:
		r.w.WriteString(`<div class="empty-line"></div>` + "\n")
	case *model.Poem:
		r.renderPoem(n)
	case *model.Cite:
		r.w.WriteString(`<blockquote class="cite">` + "\n")
		r.renderBlocks(n.Content)
		r.renderTextAuthor(n.TextAuthor)
		r.w.WriteString("</blockquote>\n")
	case *model.Epigraph:
		r.w.WriteString(`<div class="epigraph">` + "\n")
		r.renderBlocks(n.Content)
		r.renderTextAuthor(n.TextAuthor)
		r.w.WriteString("</div>\n")
	case *model.Annotation:
		r.w.WriteString(`<div class="annotation">` + "\n")
		r.renderBlocks(n.Content)
		r.w.WriteString("</div>\n")
	case *model.Table:
		r.renderTable(n)
	case *model.Image:
		src := r.imgSrc(n.Href)
		if src == "" {
			return
		}
		alt := n.Alt
		if alt == "" {
			alt = n.Title
		}
		r.w.WriteString(`<div class="image"><img src="` + escAttr(src) + `" alt="` + escAttr(alt) + `"/></div>` + "\n")
	case *model.TextAuthor:
		r.renderTextAuthor(n.Inlines)
	}
}

func (r *renderer) renderPoem(p *model.Poem) {
	r.w.WriteString(`<div class="poem">` + "\n")
	if len(p.Title) > 0 {
		r.renderHeading(p.Title, 4)
	}
	for _, ep := range p.Epigraph {
		r.renderBlock(ep)
	}
	for _, st := range p.Stanzas {
		r.w.WriteString(`<div class="stanza">` + "\n")
		for _, v := range st.Verses {
			r.w.WriteString(`<p class="v">`)
			r.renderInlines(v)
			r.w.WriteString("</p>\n")
		}
		r.w.WriteString("</div>\n")
	}
	r.renderTextAuthor(p.TextAuthor)
	r.w.WriteString("</div>\n")
}

func (r *renderer) renderTable(t *model.Table) {
	r.w.WriteString("<table>\n")
	for _, row := range t.Rows {
		r.w.WriteString("<tr>")
		for _, c := range row.Cells {
			tag := "td"
			if c.Header {
				tag = "th"
			}
			r.w.WriteString("<" + tag)
			if c.ColSpan > 1 {
				r.w.WriteString(` colspan="` + itoa(c.ColSpan) + `"`)
			}
			if c.RowSpan > 1 {
				r.w.WriteString(` rowspan="` + itoa(c.RowSpan) + `"`)
			}
			if c.Align != "" {
				r.w.WriteString(` style="text-align:` + escAttr(c.Align) + `"`)
			}
			r.w.WriteString(">")
			r.renderInlines(c.Inlines)
			r.w.WriteString("</" + tag + ">")
		}
		r.w.WriteString("</tr>\n")
	}
	r.w.WriteString("</table>\n")
}

func (r *renderer) renderTextAuthor(in []model.Inline) {
	if len(in) == 0 {
		return
	}
	r.w.WriteString(`<p class="text-author">`)
	r.renderInlines(in)
	r.w.WriteString("</p>\n")
}

func (r *renderer) renderInlines(in []model.Inline) {
	for _, x := range in {
		switch n := x.(type) {
		case *model.Text:
			r.w.WriteString(escText(n.Value))
		case *model.Styled:
			open, close := styleTags(n)
			r.w.WriteString(open)
			r.renderInlines(n.Children)
			r.w.WriteString(close)
		case *model.Link:
			href := r.resolveHref(n.Href)
			r.w.WriteString(`<a href="` + escAttr(href) + `"`)
			if n.Note {
				r.w.WriteString(` epub:type="noteref"`)
			}
			r.w.WriteString(">")
			r.renderInlines(n.Children)
			r.w.WriteString("</a>")
		case *model.InlineImage:
			src := r.imgSrc(n.Href)
			if src != "" {
				r.w.WriteString(`<img class="inline" src="` + escAttr(src) + `" alt="` + escAttr(n.Alt) + `"/>`)
			}
		}
	}
}

func styleTags(s *model.Styled) (string, string) {
	switch s.Kind {
	case model.StyleStrong:
		return "<strong>", "</strong>"
	case model.StyleEmphasis:
		return "<em>", "</em>"
	case model.StyleStrikethrough:
		return "<s>", "</s>"
	case model.StyleSub:
		return "<sub>", "</sub>"
	case model.StyleSup:
		return "<sup>", "</sup>"
	case model.StyleCode:
		return "<code>", "</code>"
	case model.StyleNamed:
		return `<span class="st-` + escAttr(s.Name) + `">`, "</span>"
	default:
		return "<span>", "</span>"
	}
}

// resolveHref maps an FB2 href to an EPUB-relative href from the current file.
func (r *renderer) resolveHref(href string) string {
	if strings.HasPrefix(href, "#") {
		if rf, ok := r.pl.idMap[href[1:]]; ok {
			return relHref(r.file, rf.file, rf.frag)
		}
		return href // dangling — surfaced by validation
	}
	return href
}

// imgSrc maps a binary id to an EPUB-relative image src from the current file.
func (r *renderer) imgSrc(binID string) string {
	target, ok := r.pl.imgMap[binID]
	if !ok {
		return ""
	}
	return relHref(r.file, target, "")
}

// blocksInline flattens paragraphs in a title-like block slice to inline nodes.
func blocksInline(blocks []model.Block) []model.Inline {
	var out []model.Inline
	for _, b := range blocks {
		if p, ok := b.(*model.Paragraph); ok {
			out = append(out, p.Inlines...)
		}
	}
	return out
}
