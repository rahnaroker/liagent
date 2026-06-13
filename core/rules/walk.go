package rules

import "litagent/core/model"

// inlineFunc receives an inline container (a slice whose Text nodes will be
// rewritten in place) together with the enclosing section id.
type inlineFunc func(inlines []model.Inline, secID string)

// walkBook visits every inline container in the book (main bodies + notes).
func walkBook(b *model.Book, fn inlineFunc) {
	for _, body := range b.Bodies {
		walkBlocks(body.Title, "", fn)
		walkBlocks(body.Epigraph, "", fn)
		for _, s := range body.Sections {
			walkSection(s, fn)
		}
	}
	if b.Notes != nil {
		for _, s := range b.Notes.Sections {
			walkSection(s, fn)
		}
	}
}

func walkSection(s *model.Section, fn inlineFunc) {
	walkBlocks(s.Title, s.ID, fn)
	for _, ep := range s.Epigraphs {
		walkBlocks(ep, s.ID, fn)
	}
	walkBlocks(s.Annotation, s.ID, fn)
	walkBlocks(s.Content, s.ID, fn)
	for _, c := range s.Children {
		walkSection(c, fn)
	}
}

func walkBlocks(blocks []model.Block, secID string, fn inlineFunc) {
	for _, b := range blocks {
		walkBlock(b, secID, fn)
	}
}

func walkBlock(b model.Block, secID string, fn inlineFunc) {
	switch n := b.(type) {
	case *model.Paragraph:
		fn(n.Inlines, secID)
	case *model.Subtitle:
		fn(n.Inlines, secID)
	case *model.TextAuthor:
		fn(n.Inlines, secID)
	case *model.Poem:
		walkBlocks(n.Title, secID, fn)
		walkBlocks(n.Epigraph, secID, fn)
		for _, st := range n.Stanzas {
			walkBlocks(st.Title, secID, fn)
			if len(st.Subtitle) > 0 {
				fn(st.Subtitle, secID)
			}
			for _, v := range st.Verses {
				fn(v, secID)
			}
		}
		if len(n.TextAuthor) > 0 {
			fn(n.TextAuthor, secID)
		}
	case *model.Cite:
		walkBlocks(n.Content, secID, fn)
		if len(n.TextAuthor) > 0 {
			fn(n.TextAuthor, secID)
		}
	case *model.Epigraph:
		walkBlocks(n.Content, secID, fn)
		if len(n.TextAuthor) > 0 {
			fn(n.TextAuthor, secID)
		}
	case *model.Annotation:
		walkBlocks(n.Content, secID, fn)
	case *model.Table:
		for _, row := range n.Rows {
			for i := range row.Cells {
				fn(row.Cells[i].Inlines, secID)
			}
		}
	}
}
