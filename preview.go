package main

import (
	"html"
	"strings"

	"litagent/core/model"
)

// previewBlockCap limits how much of the first chapter is rendered for preview.
const previewBlockCap = 400

// previewHTML renders the first chapter of the (corrected) book to simple HTML
// so the user can see the result of the applied rules.
func previewHTML(book *model.Book) string {
	var sec *model.Section
	for _, b := range book.Bodies {
		if len(b.Sections) > 0 {
			sec = b.Sections[0]
			break
		}
	}
	if sec == nil {
		return "<p>В книге нет глав для предпросмотра.</p>"
	}

	r := &previewRenderer{}
	r.section(sec, 1)
	if r.count >= previewBlockCap {
		r.w.WriteString(`<p class="more">… (предпросмотр первой главы усечён)</p>`)
	}
	return r.w.String()
}

type previewRenderer struct {
	w     strings.Builder
	count int
}

func (r *previewRenderer) section(s *model.Section, depth int) {
	if len(s.Title) > 0 {
		level := depth
		if level > 6 {
			level = 6
		}
		tag := "h" + string(rune('0'+level))
		r.w.WriteString("<" + tag + ">")
		for i, b := range s.Title {
			if p, ok := b.(*model.Paragraph); ok {
				if i > 0 {
					r.w.WriteString("<br/>")
				}
				r.inlines(p.Inlines)
			}
		}
		r.w.WriteString("</" + tag + ">")
	}
	r.blocks(s.Content)
	for _, c := range s.Children {
		if r.count >= previewBlockCap {
			return
		}
		r.section(c, depth+1)
	}
}

func (r *previewRenderer) blocks(blocks []model.Block) {
	for _, b := range blocks {
		if r.count >= previewBlockCap {
			return
		}
		r.count++
		r.block(b)
	}
}

func (r *previewRenderer) block(b model.Block) {
	switch n := b.(type) {
	case *model.Paragraph:
		r.w.WriteString("<p>")
		r.inlines(n.Inlines)
		r.w.WriteString("</p>")
	case *model.Subtitle:
		r.w.WriteString(`<p class="sub">`)
		r.inlines(n.Inlines)
		r.w.WriteString("</p>")
	case *model.EmptyLine:
		r.w.WriteString(`<div class="el"></div>`)
	case *model.Poem:
		r.w.WriteString(`<div class="poem">`)
		for _, st := range n.Stanzas {
			for _, v := range st.Verses {
				r.w.WriteString(`<p class="v">`)
				r.inlines(v)
				r.w.WriteString("</p>")
			}
		}
		r.w.WriteString("</div>")
	case *model.Cite:
		r.w.WriteString("<blockquote>")
		r.blocks(n.Content)
		if len(n.TextAuthor) > 0 {
			r.w.WriteString(`<p class="author">`)
			r.inlines(n.TextAuthor)
			r.w.WriteString("</p>")
		}
		r.w.WriteString("</blockquote>")
	case *model.Epigraph:
		r.w.WriteString(`<div class="epi">`)
		r.blocks(n.Content)
		r.w.WriteString("</div>")
	case *model.TextAuthor:
		r.w.WriteString(`<p class="author">`)
		r.inlines(n.Inlines)
		r.w.WriteString("</p>")
	case *model.Image:
		r.w.WriteString(`<p class="img">[изображение]</p>`)
	}
}

func (r *previewRenderer) inlines(in []model.Inline) {
	for _, x := range in {
		switch n := x.(type) {
		case *model.Text:
			r.w.WriteString(html.EscapeString(n.Value))
		case *model.Styled:
			tag := previewTag(n.Kind)
			if tag != "" {
				r.w.WriteString("<" + tag + ">")
			}
			r.inlines(n.Children)
			if tag != "" {
				r.w.WriteString("</" + tag + ">")
			}
		case *model.Link:
			r.inlines(n.Children)
		}
	}
}

func previewTag(k model.StyleKind) string {
	switch k {
	case model.StyleStrong:
		return "b"
	case model.StyleEmphasis:
		return "i"
	case model.StyleStrikethrough:
		return "s"
	case model.StyleSub:
		return "sub"
	case model.StyleSup:
		return "sup"
	case model.StyleCode:
		return "code"
	default:
		return ""
	}
}
