package main

import (
	"fmt"
	"regexp"
	"strings"

	"litagent/core/model"
)

// MetaEdit is the flat, UI-editable view of a book's metadata. Authors and
// translators are one display name per line; keywords are comma-separated. Only
// fields that actually reach the EPUB output are exposed.
type MetaEdit struct {
	Title        string `json:"title"`
	Authors      string `json:"authors"`
	Translators  string `json:"translators"`
	Lang         string `json:"lang"`
	SeriesName   string `json:"seriesName"`
	SeriesNumber string `json:"seriesNumber"`
	Publisher    string `json:"publisher"`
	Date         string `json:"date"`
	ISBN         string `json:"isbn"`
	Annotation   string `json:"annotation"`
	Keywords     string `json:"keywords"`
}

// GetMetadata returns the current book's metadata as an editable struct.
func (a *App) GetMetadata() (MetaEdit, error) {
	a.mu.Lock()
	book := a.book
	a.mu.Unlock()
	if book == nil {
		return MetaEdit{}, fmt.Errorf("нет открытой книги")
	}
	m := book.Meta
	e := MetaEdit{
		Title:      m.Title,
		Authors:    personsToText(m.Authors),
		Translators: personsToText(m.Translators),
		Lang:       m.Lang,
		Publisher:  m.Publisher,
		Date:       m.Date,
		ISBN:       m.ISBN,
		Annotation: annotationText(book),
		Keywords:   strings.Join(m.Keywords, ", "),
	}
	if m.Sequence != nil {
		e.SeriesName = m.Sequence.Name
		e.SeriesNumber = m.Sequence.Number
	}
	return e, nil
}

// SetMetadata stores the edited metadata; it is applied to a copy of the book at
// export time (see prepareExportBook). The original a.book is never changed.
func (a *App) SetMetadata(e MetaEdit) error {
	a.mu.Lock()
	a.metaEdit = &e
	a.mu.Unlock()
	return nil
}

// CleanTitleTags returns title with bracketed tags ([...]/{...}) and a trailing
// "litres" marker removed — used by the UI "remove tags" button.
func (a *App) CleanTitleTags(title string) string { return cleanMetaTitle(title) }

// applyMetaEdit overlays the stored edit onto the export copy's metadata.
func (a *App) applyMetaEdit(cp *model.Book) {
	a.mu.Lock()
	e := a.metaEdit
	a.mu.Unlock()
	if e == nil {
		return
	}
	cp.Meta.Title = strings.TrimSpace(e.Title)
	cp.Meta.Authors = parsePersons(e.Authors)
	cp.Meta.Translators = parsePersons(e.Translators)
	cp.Meta.Lang = strings.TrimSpace(e.Lang)
	cp.Meta.Publisher = strings.TrimSpace(e.Publisher)
	cp.Meta.Date = strings.TrimSpace(e.Date)
	cp.Meta.ISBN = strings.TrimSpace(e.ISBN)
	cp.Meta.Keywords = splitCSV(e.Keywords)

	if name := strings.TrimSpace(e.SeriesName); name != "" {
		cp.Meta.Sequence = &model.Sequence{Name: name, Number: strings.TrimSpace(e.SeriesNumber)}
	} else {
		cp.Meta.Sequence = nil // no series → clean Kindle listing
	}

	if ann := strings.TrimSpace(e.Annotation); ann != "" {
		cp.Meta.Annotation = []model.Block{
			&model.Paragraph{Inlines: []model.Inline{&model.Text{Value: ann}}},
		}
	} else {
		cp.Meta.Annotation = nil
	}
}

// effectiveTitleAuthor returns the title/author after any pending edit, so the
// generated cover matches what will be exported.
func (a *App) effectiveTitleAuthor(book *model.Book) (string, string) {
	info := metaInfo(book)
	title, author := info.Title, info.Author
	a.mu.Lock()
	e := a.metaEdit
	a.mu.Unlock()
	if e != nil {
		if t := strings.TrimSpace(e.Title); t != "" {
			title = t
		}
		if as := parsePersons(e.Authors); len(as) > 0 {
			author = personDisplayName(as[0])
		}
	}
	return title, author
}

// --- helpers ---------------------------------------------------------------

// personsToText renders persons as one display name per line.
func personsToText(ps []model.Person) string {
	lines := make([]string, 0, len(ps))
	for _, p := range ps {
		if name := personDisplayName(p); name != "" {
			lines = append(lines, name)
		}
	}
	return strings.Join(lines, "\n")
}

// personDisplayName joins a person's parts into a display name (nick fallback).
func personDisplayName(p model.Person) string {
	parts := make([]string, 0, 3)
	for _, s := range []string{p.First, p.Middle, p.Last} {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return strings.TrimSpace(p.Nick)
	}
	return strings.Join(parts, " ")
}

// parsePersons turns one-name-per-line text into Person values, splitting each
// line so dc:creator and file-as sorting stay correct.
func parsePersons(text string) []model.Person {
	var out []model.Person
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, splitName(line))
		}
	}
	return out
}

// splitName splits "First [Middle...] Last" into name parts. A single token
// becomes the Last name (so file-as is sensible).
func splitName(name string) model.Person {
	fields := strings.Fields(name)
	switch len(fields) {
	case 0:
		return model.Person{}
	case 1:
		return model.Person{Last: fields[0]}
	case 2:
		return model.Person{First: fields[0], Last: fields[1]}
	default:
		return model.Person{
			First:  fields[0],
			Middle: strings.Join(fields[1:len(fields)-1], " "),
			Last:   fields[len(fields)-1],
		}
	}
}

// splitCSV splits a comma-separated list, trimming and dropping empties.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// annotationText flattens the book's annotation blocks to plain text.
func annotationText(book *model.Book) string {
	var sb strings.Builder
	var walkInlines func([]model.Inline)
	walkInlines = func(xs []model.Inline) {
		for _, x := range xs {
			switch n := x.(type) {
			case *model.Text:
				sb.WriteString(n.Value)
			case *model.Styled:
				walkInlines(n.Children)
			case *model.Link:
				walkInlines(n.Children)
			}
		}
	}
	var walkBlocks func([]model.Block)
	walkBlocks = func(bs []model.Block) {
		for _, b := range bs {
			switch n := b.(type) {
			case *model.Paragraph:
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				walkInlines(n.Inlines)
			case *model.Subtitle:
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				walkInlines(n.Inlines)
			}
		}
	}
	walkBlocks(book.Meta.Annotation)
	return strings.TrimSpace(sb.String())
}

// illegalFileChars are characters not allowed in Windows file names, plus control
// chars. Removed when deriving the output file name from the book title.
var illegalFileChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// sanitizeFilename turns a title into a safe file name (without extension):
// illegal characters dropped, whitespace collapsed, trailing dots/spaces removed
// (Windows forbids them), and length capped. Cyrillic letters are kept.
func sanitizeFilename(s string) string {
	s = illegalFileChars.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, " .")
	if r := []rune(s); len(r) > 120 {
		s = strings.TrimRight(string(r[:120]), " .")
	}
	return s
}

// metaTagRe matches bracketed tags like "[litres]" or "{fb2}" anywhere in a title.
var metaTagRe = regexp.MustCompile(`\s*[\[\{][^\]\}]*[\]\}]\s*`)

// cleanMetaTitle strips bracketed tags and a trailing "litres" marker.
func cleanMetaTitle(title string) string {
	out := metaTagRe.ReplaceAllString(title, " ")
	out = strings.TrimSpace(out)
	out = strings.TrimSuffix(out, "litres")
	out = strings.TrimSuffix(out, "ЛитРес")
	return strings.TrimSpace(out)
}
