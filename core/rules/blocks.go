package rules

import (
	"regexp"
	"strings"
	"unicode"

	"litagent/core/model"
)

// blockRule examines a whole paragraph (not a text run). Block rules are
// structural detectors (level C): they always report a suggestion, and if the
// user accepts the rule, apply mutates the paragraph.
type blockRule struct {
	id, name, category string
	level              Confidence
	// detect returns the proposed "after" label and whether the paragraph
	// matches. An empty proposal means "remove this paragraph".
	detect func(text string) (after string, ok bool)
	// apply, if non-nil, mutates the paragraph when the rule is accepted.
	apply func(p *model.Paragraph)
}

var pageNumberRe = regexp.MustCompile(`^\d{1,4}$`)

// page-number: a paragraph consisting solely of a small number — usually a page
// number left in by OCR. Suggest removal.
var rulePageNumber = blockRule{
	id: "page-number", name: "Номер страницы (OCR)", category: "structure", level: ConfC,
	detect: func(text string) (string, bool) {
		if pageNumberRe.MatchString(strings.TrimSpace(text)) {
			return "", true
		}
		return "", false
	},
}

// chapterWordRe matches lines beginning with a chapter/part keyword (Go's \b is
// ASCII-only, so a following \s/$ is used as the boundary).
var chapterWordRe = regexp.MustCompile(`(?i)^(глава|часть|том|книга|пролог|эпилог|приложение)(\s|$)`)

// heading-detect: a short line that is ALL-CAPS or starts with a chapter keyword
// and has no terminal punctuation — likely an unmarked heading. When accepted it
// marks the paragraph as a heading (rendered as <h> and added to the TOC).
var ruleHeadingDetect = blockRule{
	id: "heading-detect", name: "Похоже на заголовок", category: "structure", level: ConfC,
	detect: func(text string) (string, bool) {
		if looksLikeHeading(strings.TrimSpace(text)) {
			return "‹заголовок›", true
		}
		return "", false
	},
	apply: func(p *model.Paragraph) { p.Heading = true },
}

func looksLikeHeading(t string) bool {
	r := []rune(t)
	n := len(r)
	if n < 2 || n > 60 {
		return false
	}
	switch r[n-1] {
	case '.', '!', '?', ':', ';', ',', '…':
		return false
	}
	if chapterWordRe.MatchString(t) {
		return true
	}
	return isAllCaps(r)
}

func isAllCaps(r []rune) bool {
	hasUpper := false
	for _, c := range r {
		if unicode.IsLower(c) {
			return false
		}
		if unicode.IsUpper(c) {
			hasUpper = true
		}
	}
	return hasUpper
}

var blockRules = []blockRule{rulePageNumber, ruleHeadingDetect}

// runBlockRules applies the structural detectors to every content paragraph. It
// always reports findings; a matched rule that the user accepted also mutates.
func runBlockRules(b *model.Book, applyRule func(id string) bool, all *[]Finding) {
	walkParagraphs(b, func(p *model.Paragraph, secID string) {
		text := paragraphText(p)
		if strings.TrimSpace(text) == "" {
			return
		}
		for i := range blockRules {
			br := &blockRules[i]
			after, ok := br.detect(text)
			if !ok {
				continue
			}
			*all = append(*all, Finding{
				RuleID:   br.id,
				Category: br.category,
				Level:    br.level,
				Section:  secID,
				Before:   text,
				After:    after,
				Context:  "⟦" + clip(text, 40) + "⟧",
			})
			if br.apply != nil && applyRule(br.id) {
				br.apply(p)
			}
		}
	})
}

// paragraphText returns the concatenated plain text of a paragraph.
func paragraphText(p *model.Paragraph) string {
	var sb strings.Builder
	var rec func([]model.Inline)
	rec = func(xs []model.Inline) {
		for _, x := range xs {
			switch n := x.(type) {
			case *model.Text:
				sb.WriteString(n.Value)
			case *model.Styled:
				rec(n.Children)
			case *model.Link:
				rec(n.Children)
			}
		}
	}
	rec(p.Inlines)
	return sb.String()
}

// walkParagraphs visits every content paragraph in the book.
func walkParagraphs(b *model.Book, fn func(*model.Paragraph, string)) {
	for _, body := range b.Bodies {
		for _, s := range body.Sections {
			walkParaSection(s, fn)
		}
	}
	if b.Notes != nil {
		for _, s := range b.Notes.Sections {
			walkParaSection(s, fn)
		}
	}
}

func walkParaSection(s *model.Section, fn func(*model.Paragraph, string)) {
	for _, blk := range s.Content {
		if p, ok := blk.(*model.Paragraph); ok {
			fn(p, s.ID)
		}
	}
	for _, c := range s.Children {
		walkParaSection(c, fn)
	}
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
