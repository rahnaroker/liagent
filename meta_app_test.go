package main

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"litagent/core/epub"
	"litagent/core/rules"
)

func TestSplitName(t *testing.T) {
	cases := []struct {
		in                  string
		first, middle, last string
	}{
		{"Толстой", "", "", "Толстой"},
		{"Лев Толстой", "Лев", "", "Толстой"},
		{"Лев Николаевич Толстой", "Лев", "Николаевич", "Толстой"},
	}
	for _, c := range cases {
		p := splitName(c.in)
		if p.First != c.first || p.Middle != c.middle || p.Last != c.last {
			t.Errorf("splitName(%q) = %+v, want %s/%s/%s", c.in, p, c.first, c.middle, c.last)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"Волоколамское шоссе":          "Волоколамское шоссе",
		`Имя: "под/раздел"`:            "Имя под раздел",
		"Книга?*|<>":                   "Книга",
		"  лишние   пробелы  ":         "лишние пробелы",
		"точка в конце.":               "точка в конце",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
	// Tags are stripped before sanitising (as used for the export file name).
	if got := sanitizeFilename(cleanMetaTitle("Война и мир [litres]")); got != "Война и мир" {
		t.Errorf("filename from tagged title = %q", got)
	}
}

func TestCleanMetaTitle(t *testing.T) {
	cases := map[string]string{
		"Волоколамское шоссе [litres]":  "Волоколамское шоссе",
		"Книга {fb2}":                   "Книга",
		"[Серия] Название":              "Название",
		"Просто название":               "Просто название",
		"Название litres":               "Название",
	}
	for in, want := range cases {
		if got := cleanMetaTitle(in); got != want {
			t.Errorf("cleanMetaTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestMetaEditAppliedOnExport edits metadata and verifies the resulting EPUB OPF
// reflects the edits while a.book stays untouched.
func TestMetaEditAppliedOnExport(t *testing.T) {
	a := NewApp()
	a.path = sampleFB2
	if _, err := a.run(rules.DefaultApplyIDs()); err != nil {
		t.Fatalf("run: %v", err)
	}
	origTitle := a.book.Meta.Title

	if err := a.SetMetadata(MetaEdit{
		Title:      "Чистое название",
		Authors:    "Иван Петров",
		Lang:       "ru",
		Publisher:  "Моё издательство",
		Date:       "2026",
		ISBN:       "9781234567890",
		Keywords:   "война, история",
		SeriesName: "", // cleared → no series
		Annotation: "Краткое описание.",
	}); err != nil {
		t.Fatal(err)
	}

	opf := buildOPF(t, a)

	for _, want := range []string{
		"<dc:title>Чистое название</dc:title>",
		"<dc:creator", "Иван Петров",
		"Петров, Иван", // file-as
		"<dc:publisher>Моё издательство</dc:publisher>",
		"<dc:date>2026</dc:date>",
		"urn:isbn:9781234567890",
		"<dc:subject>война</dc:subject>",
		"<dc:subject>история</dc:subject>",
		"Краткое описание.",
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("OPF missing %q", want)
		}
	}
	if strings.Contains(opf, "belongs-to-collection") {
		t.Error("series should be absent after clearing SeriesName")
	}
	// Original book untouched.
	if a.book.Meta.Title != origTitle {
		t.Errorf("a.book mutated: title = %q, want %q", a.book.Meta.Title, origTitle)
	}
}

// buildOPF exports the prepared book to an in-memory EPUB and returns content.opf.
func buildOPF(t *testing.T, a *App) string {
	t.Helper()
	var buf bytes.Buffer
	if err := epub.Build(a.prepareExportBook(a.book), epub.Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if strings.HasSuffix(zf.Name, "content.opf") {
			rc, _ := zf.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return string(b)
		}
	}
	t.Fatal("content.opf not found")
	return ""
}
