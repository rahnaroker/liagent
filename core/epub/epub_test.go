package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	"litagent/core/fb2"
	"litagent/core/model"
)

// readEPUB builds the book and returns the EPUB entries as name->content.
func readEPUB(t *testing.T, book *model.Book) map[string]string {
	t.Helper()
	var buf bytes.Buffer
	if err := Build(book, Options{}, &buf); err != nil {
		t.Fatalf("build: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}
	return files
}

func TestBuildDetectedHeading(t *testing.T) {
	titleP := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "Глава"}}}
	heading := &model.Paragraph{Heading: true, Inlines: []model.Inline{&model.Text{Value: "ПОДЗАГОЛОВОК"}}}
	body := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "Обычный текст."}}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{
		ID:      "ch1",
		Title:   []model.Block{titleP},
		Content: []model.Block{heading, body},
	}}}}}

	files := readEPUB(t, book)
	chap := files["OEBPS/text/chap_0001.xhtml"]
	if !strings.Contains(chap, "<h4") || !strings.Contains(chap, "ПОДЗАГОЛОВОК") {
		t.Errorf("heading paragraph not rendered as <h4>:\n%s", chap)
	}
	if strings.Contains(chap, "<p>ПОДЗАГОЛОВОК") {
		t.Error("heading wrongly rendered as <p>")
	}
	nav := files["OEBPS/nav.xhtml"]
	if !strings.Contains(nav, "ПОДЗАГОЛОВОК") {
		t.Errorf("detected heading not in nav:\n%s", nav)
	}
}

func TestStableIdentifier(t *testing.T) {
	mk := func() *model.Book {
		return &model.Book{
			Meta: model.Metadata{Title: "Война и мир", Authors: []model.Person{{First: "Лев", Last: "Толстой"}}},
			Bodies: []*model.Body{{Sections: []*model.Section{{
				Content: []model.Block{&model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "Текст."}}}},
			}}}},
		}
	}
	idOf := func(files map[string]string) string {
		opf := files["OEBPS/content.opf"]
		const open = "<dc:identifier id=\"bookid\">"
		i := strings.Index(opf, open)
		if i < 0 {
			t.Fatalf("no dc:identifier in opf:\n%s", opf)
		}
		i += len(open)
		return opf[i : i+strings.Index(opf[i:], "<")]
	}
	id1 := idOf(readEPUB(t, mk()))
	id2 := idOf(readEPUB(t, mk()))
	if id1 != id2 {
		t.Errorf("identifier not stable across builds: %q vs %q", id1, id2)
	}
	if !strings.HasPrefix(id1, "urn:uuid:") || strings.TrimPrefix(id1, "urn:uuid:") == "" {
		t.Errorf("identifier malformed: %q", id1)
	}
	// A different book must get a different id.
	other := mk()
	other.Meta.Title = "Анна Каренина"
	if idOf(readEPUB(t, other)) == id1 {
		t.Errorf("different book shares identifier %q", id1)
	}
}

func TestAnchorCollisionDeduped(t *testing.T) {
	// "a/b" and "a:b" both sanitise to "a_b": the two sections must still get
	// distinct ids, and a cross-reference resolves to the second one's anchor.
	link := &model.Paragraph{Inlines: []model.Inline{
		&model.Link{Href: "#a:b", Children: []model.Inline{&model.Text{Value: "ссылка"}}},
	}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{
		{ID: "a/b", Content: []model.Block{&model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "первая"}}}}},
		{ID: "a:b", Content: []model.Block{link}},
	}}}}
	files := readEPUB(t, book)
	c1 := files["OEBPS/text/chap_0001.xhtml"]
	c2 := files["OEBPS/text/chap_0002.xhtml"]
	if !strings.Contains(c1, `id="a_b"`) {
		t.Errorf("first section missing id a_b:\n%s", c1)
	}
	if !strings.Contains(c2, `id="a_b-2"`) {
		t.Errorf("second section not deduped to a_b-2:\n%s", c2)
	}
	if !strings.Contains(c2, "#a_b-2") {
		t.Errorf("link not resolved to deduped anchor:\n%s", c2)
	}
}

func TestEmptyParagraphDropped(t *testing.T) {
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{
		Content: []model.Block{
			&model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "    "}}}, // empty/nbsp
			&model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "Настоящий абзац."}}},
			&model.Paragraph{ID: "keep", Inlines: []model.Inline{&model.Text{Value: " "}}}, // empty but anchored
		},
	}}}}}
	chap := readEPUB(t, book)["OEBPS/text/chap_0001.xhtml"]
	if strings.Contains(chap, "<p></p>") || strings.Contains(chap, "<p> </p>") {
		t.Errorf("empty paragraph emitted:\n%s", chap)
	}
	if !strings.Contains(chap, "Настоящий абзац.") {
		t.Errorf("real paragraph missing:\n%s", chap)
	}
	if !strings.Contains(chap, `id="keep"`) {
		t.Errorf("anchored empty paragraph wrongly dropped:\n%s", chap)
	}
}

func buildSample(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../fb2/testdata/sample.fb2")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	book, err := fb2.Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Build(book, Options{}, &buf); err != nil {
		t.Fatalf("build: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(b)
	}

	// mimetype must be first and stored uncompressed.
	if zr.File[0].Name != "mimetype" {
		t.Errorf("first entry = %q, want mimetype", zr.File[0].Name)
	}
	if zr.File[0].Method != zip.Store {
		t.Errorf("mimetype method = %d, want Store", zr.File[0].Method)
	}
	if files["mimetype"] != "application/epub+zip" {
		t.Errorf("mimetype content = %q", files["mimetype"])
	}
	return files
}

func TestBuildContainsRequiredFiles(t *testing.T) {
	files := buildSample(t)
	for _, name := range []string{
		"META-INF/container.xml",
		"OEBPS/content.opf",
		"OEBPS/nav.xhtml",
		"OEBPS/toc.ncx",
		"OEBPS/style.css",
		"OEBPS/text/cover.xhtml",
		"OEBPS/text/chap_0001.xhtml",
		"OEBPS/text/chap_0002.xhtml",
		"OEBPS/text/notes.xhtml",
		"OEBPS/images/cover.jpg",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
}

func TestBuildWellFormedXML(t *testing.T) {
	files := buildSample(t)
	for name, content := range files {
		if !strings.HasSuffix(name, ".xhtml") && !strings.HasSuffix(name, ".opf") &&
			!strings.HasSuffix(name, ".ncx") && !strings.HasSuffix(name, ".xml") {
			continue
		}
		dec := xml.NewDecoder(strings.NewReader(content))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%s not well-formed: %v", name, err)
				break
			}
		}
	}
}

func TestBuildPopupFootnotes(t *testing.T) {
	files := buildSample(t)
	chap := files["OEBPS/text/chap_0001.xhtml"]
	if !strings.Contains(chap, `epub:type="noteref"`) {
		t.Error("chapter missing noteref")
	}
	if !strings.Contains(chap, `href="notes.xhtml#note1"`) {
		t.Errorf("noteref href not resolved to notes.xhtml#note1:\n%s", chap)
	}
	notes := files["OEBPS/text/notes.xhtml"]
	if !strings.Contains(notes, `epub:type="footnote"`) || !strings.Contains(notes, `id="note1"`) {
		t.Errorf("notes missing footnote aside:\n%s", notes)
	}
}

func TestBuildOPFMetadata(t *testing.T) {
	files := buildSample(t)
	opf := files["OEBPS/content.opf"]
	for _, want := range []string{
		"<dc:title>Пример книги</dc:title>",
		"<dc:language>ru</dc:language>",
		"Толстой",
		`<meta name="cover" content="cover-image"/>`,
		`properties="cover-image"`,
		`<itemref idref="chap_0001"/>`,
		`property="dcterms:modified"`,
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("opf missing %q", want)
		}
	}
	// file-as should sort by surname.
	if !strings.Contains(opf, "Толстой, Лев") {
		t.Errorf("opf missing file-as 'Толстой, Лев'")
	}
}

func TestBuildNavHierarchy(t *testing.T) {
	files := buildSample(t)
	nav := files["OEBPS/nav.xhtml"]
	for _, want := range []string{"Глава первая", "Подглава", "Глава вторая"} {
		if !strings.Contains(nav, want) {
			t.Errorf("nav missing %q", want)
		}
	}
	// Nested subchapter should produce a nested <ol>.
	if strings.Count(nav, "<ol>") < 2 {
		t.Errorf("expected nested <ol> for subchapter, got:\n%s", nav)
	}
	if !strings.Contains(nav, `href="text/chap_0001.xhtml#ch1"`) {
		t.Errorf("nav href to chapter anchor missing:\n%s", nav)
	}
}

func TestBuildCoverImageRelPath(t *testing.T) {
	files := buildSample(t)
	cover := files["OEBPS/text/cover.xhtml"]
	if !strings.Contains(cover, `src="../images/cover.jpg"`) {
		t.Errorf("cover image rel path wrong:\n%s", cover)
	}
}

func TestBuildPoemPreserved(t *testing.T) {
	files := buildSample(t)
	chap := files["OEBPS/text/chap_0002.xhtml"]
	if strings.Count(chap, `class="v"`) != 2 {
		t.Errorf("expected 2 verse lines, got:\n%s", chap)
	}
}
