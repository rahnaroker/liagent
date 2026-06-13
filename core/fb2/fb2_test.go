package fb2

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"litagent/core/model"
)

func loadSample(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/sample.fb2")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	return b
}

// inlineText flattens an inline slice to plain text for assertions.
func inlineText(in []model.Inline) string {
	var sb strings.Builder
	var walk func([]model.Inline)
	walk = func(xs []model.Inline) {
		for _, x := range xs {
			switch n := x.(type) {
			case *model.Text:
				sb.WriteString(n.Value)
			case *model.Styled:
				walk(n.Children)
			case *model.Link:
				walk(n.Children)
			}
		}
	}
	walk(in)
	return sb.String()
}

func encodeTo(t *testing.T, s, label string) []byte {
	t.Helper()
	enc, err := htmlindex.Get(label)
	if err != nil {
		t.Fatalf("get encoder %s: %v", label, err)
	}
	out, err := io.ReadAll(transform.NewReader(strings.NewReader(s), enc.NewEncoder()))
	if err != nil {
		t.Fatalf("encode to %s: %v", label, err)
	}
	return out
}

func TestParseMetadata(t *testing.T) {
	book, err := Parse(bytes.NewReader(loadSample(t)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := book.Meta
	if m.Title != "Пример книги" {
		t.Errorf("title = %q", m.Title)
	}
	if len(m.Authors) != 1 || m.Authors[0].First != "Лев" || m.Authors[0].Last != "Толстой" {
		t.Errorf("authors = %+v", m.Authors)
	}
	if m.Lang != "ru" || m.Date != "1869" {
		t.Errorf("lang/date = %q/%q", m.Lang, m.Date)
	}
	if m.Publisher != "Издательство" || m.City != "Москва" || m.Year != "2020" {
		t.Errorf("publish = %q/%q/%q", m.Publisher, m.City, m.Year)
	}
	if m.ISBN != "978-5-00000-000-0" {
		t.Errorf("isbn = %q", m.ISBN)
	}
	if m.DocID != "abc-123" || m.ProgramUsed != "FB Editor" {
		t.Errorf("docinfo = %q/%q", m.DocID, m.ProgramUsed)
	}
	if m.CoverHref != "cover.jpg" {
		t.Errorf("cover = %q", m.CoverHref)
	}
	if len(m.Keywords) != 2 || m.Keywords[0] != "тест" {
		t.Errorf("keywords = %v", m.Keywords)
	}
}

func TestParseStructure(t *testing.T) {
	book, err := Parse(bytes.NewReader(loadSample(t)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(book.Bodies) != 1 {
		t.Fatalf("bodies = %d, want 1", len(book.Bodies))
	}
	main := book.Bodies[0]
	if len(main.Sections) != 2 {
		t.Fatalf("top sections = %d, want 2", len(main.Sections))
	}
	ch1 := main.Sections[0]
	if ch1.ID != "ch1" {
		t.Errorf("ch1.ID = %q", ch1.ID)
	}
	if got := inlineText(blocksInline(ch1.Title)); got != "Глава первая" {
		t.Errorf("ch1 title = %q", got)
	}
	// Content: paragraph, empty-line, paragraph (nested section is in Children).
	if len(ch1.Content) != 3 {
		t.Fatalf("ch1 content = %d, want 3", len(ch1.Content))
	}
	if _, ok := ch1.Content[1].(*model.EmptyLine); !ok {
		t.Errorf("ch1.Content[1] = %T, want EmptyLine", ch1.Content[1])
	}
	if len(ch1.Children) != 1 || ch1.Children[0].ID != "ch1-1" {
		t.Errorf("ch1 children = %+v", ch1.Children)
	}

	// Notes body.
	if book.Notes == nil || len(book.Notes.Sections) != 1 || book.Notes.Sections[0].ID != "note1" {
		t.Errorf("notes = %+v", book.Notes)
	}

	// Cover binary decoded.
	if len(book.Binaries) != 1 || book.Binaries[0].ID != "cover.jpg" || len(book.Binaries[0].Data) == 0 {
		t.Errorf("binaries = %+v", book.Binaries)
	}
}

func TestParseInlineRichness(t *testing.T) {
	book, err := Parse(bytes.NewReader(loadSample(t)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	para, ok := book.Bodies[0].Sections[0].Content[0].(*model.Paragraph)
	if !ok {
		t.Fatalf("Content[0] = %T, want Paragraph", book.Bodies[0].Sections[0].Content[0])
	}
	if got := inlineText(para.Inlines); got != "Это первый абзац с выделением и сноской[1]." {
		t.Errorf("flattened paragraph = %q", got)
	}
	// Find a strong run and a footnote link.
	var hasStrong, hasNote bool
	for _, in := range para.Inlines {
		if s, ok := in.(*model.Styled); ok && s.Kind == model.StyleStrong && inlineText(s.Children) == "первый" {
			hasStrong = true
		}
		if l, ok := in.(*model.Link); ok && l.Note && strings.Contains(l.Href, "note1") {
			hasNote = true
		}
	}
	if !hasStrong {
		t.Error("missing strong run 'первый'")
	}
	if !hasNote {
		t.Error("missing footnote link to note1")
	}

	// Poem verses preserved.
	poem, ok := book.Bodies[0].Sections[1].Content[0].(*model.Poem)
	if !ok {
		t.Fatalf("ch2 Content[0] = %T, want Poem", book.Bodies[0].Sections[1].Content[0])
	}
	if len(poem.Stanzas) != 1 || len(poem.Stanzas[0].Verses) != 2 {
		t.Fatalf("poem stanzas/verses = %+v", poem.Stanzas)
	}
	if inlineText(poem.Stanzas[0].Verses[0]) != "Строка раз" {
		t.Errorf("verse 0 = %q", inlineText(poem.Stanzas[0].Verses[0]))
	}
}

func TestDecodeWindows1251(t *testing.T) {
	utf := string(loadSample(t))
	cp := strings.Replace(utf, `encoding="UTF-8"`, `encoding="windows-1251"`, 1)
	raw := encodeTo(t, cp, "windows-1251")

	out, name, err := decodeToUTF8(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !utf8.Valid(out) {
		t.Fatal("decoded output is not valid UTF-8")
	}
	if name != "windows-1251" {
		t.Errorf("detected encoding = %q, want windows-1251", name)
	}
	if !strings.Contains(string(out), "Толстой") {
		t.Error("cyrillic not preserved through 1251 decode")
	}

	book, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse 1251: %v", err)
	}
	if book.Meta.Title != "Пример книги" {
		t.Errorf("1251 title = %q", book.Meta.Title)
	}
}

func TestDecodeLyingUTF8(t *testing.T) {
	// Bytes are windows-1251 but the declaration claims UTF-8: the safety net
	// (DESIGN.md §2.1 step 3) should detect invalid UTF-8 and transcode.
	utf := string(loadSample(t))
	raw := encodeTo(t, utf, "windows-1251") // declaration still says UTF-8

	out, name, err := decodeToUTF8(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !utf8.Valid(out) || !strings.Contains(string(out), "Толстой") {
		t.Errorf("lying-utf8 not recovered; name=%q", name)
	}
}

// blocksInline flattens the inlines contained in a slice of paragraph blocks
// (used for title text which is a []Block of paragraphs).
func blocksInline(blocks []model.Block) []model.Inline {
	var out []model.Inline
	for _, b := range blocks {
		if p, ok := b.(*model.Paragraph); ok {
			out = append(out, p.Inlines...)
		}
	}
	return out
}
