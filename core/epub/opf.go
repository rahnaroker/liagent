package epub

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	"litagent/core/model"
)

const containerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// kindleCSS is a minimal, Kindle-safe stylesheet (DESIGN.md §3.8).
const kindleCSS = `body { margin: 0 5%; line-height: 1.4; }
p { margin: 0; text-indent: 1.2em; text-align: justify; }
h1, h2, h3, h4, h5, h6 { text-align: center; text-indent: 0; margin: 1em 0 0.6em; page-break-before: auto; }
h1 { page-break-before: always; }
.subtitle { text-indent: 0; text-align: center; font-weight: bold; margin: 0.8em 0; }
.empty-line { height: 1em; }
.epigraph { margin: 1em 2em 1em 30%; font-style: italic; }
.cite { margin: 1em 2em; }
.annotation { margin: 1em 2em; font-size: 0.95em; }
.text-author { text-indent: 0; text-align: right; font-style: italic; margin-top: 0.3em; }
.poem { margin: 1em 1.5em; }
.stanza { margin: 0 0 0.8em; }
.v { text-indent: 0; text-align: left; }
.note-title { text-indent: 0; font-weight: bold; }
.image { text-align: center; text-indent: 0; margin: 1em 0; }
.image img, img.inline { max-width: 100%; }
.cover { text-align: center; margin: 0; padding: 0; }
.cover img { max-width: 100%; height: 100%; }
table { border-collapse: collapse; margin: 1em 0; }
td, th { border: 1px solid #888; padding: 0.2em 0.5em; }
`

// lang returns the book language for xml:lang, defaulting to "ru".
func (b *builder) lang() string {
	if l := strings.TrimSpace(b.book.Meta.Lang); l != "" {
		return l
	}
	return "ru"
}

// bookID returns a stable unique identifier for dc:identifier.
func (b *builder) bookID() string {
	if id := strings.TrimSpace(b.book.Meta.DocID); id != "" {
		return "urn:uuid:" + id
	}
	return "urn:uuid:" + pseudoUUID()
}

func (b *builder) renderOPF(pl *plan) string {
	m := b.book.Meta
	var w strings.Builder
	w.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	w.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="` + b.lang() + `">` + "\n")

	// --- metadata ---
	w.WriteString(`  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n")
	w.WriteString(`    <dc:identifier id="bookid">` + escText(b.bookID()) + "</dc:identifier>\n")
	title := m.Title
	if title == "" {
		title = "Без названия"
	}
	w.WriteString(`    <dc:title>` + escText(title) + "</dc:title>\n")
	w.WriteString(`    <dc:language>` + escText(b.lang()) + "</dc:language>\n")

	for i, a := range m.Authors {
		id := "creator" + strconv.Itoa(i)
		w.WriteString(`    <dc:creator id="` + id + `">` + escText(personDisplay(a)) + "</dc:creator>\n")
		w.WriteString(`    <meta refines="#` + id + `" property="role" scheme="marc:relators">aut</meta>` + "\n")
		w.WriteString(`    <meta refines="#` + id + `" property="file-as">` + escText(personFileAs(a)) + "</meta>\n")
	}
	for i, t := range m.Translators {
		id := "translator" + strconv.Itoa(i)
		w.WriteString(`    <dc:contributor id="` + id + `">` + escText(personDisplay(t)) + "</dc:contributor>\n")
		w.WriteString(`    <meta refines="#` + id + `" property="role" scheme="marc:relators">trl</meta>` + "\n")
	}
	if m.Publisher != "" {
		w.WriteString(`    <dc:publisher>` + escText(m.Publisher) + "</dc:publisher>\n")
	}
	for _, kw := range m.Keywords {
		if kw != "" {
			w.WriteString(`    <dc:subject>` + escText(kw) + "</dc:subject>\n")
		}
	}
	if m.Date != "" {
		w.WriteString(`    <dc:date>` + escText(m.Date) + "</dc:date>\n")
	}
	if desc := blocksText(m.Annotation); desc != "" {
		w.WriteString(`    <dc:description>` + escText(desc) + "</dc:description>\n")
	}
	if m.ISBN != "" {
		w.WriteString(`    <dc:source>urn:isbn:` + escText(m.ISBN) + "</dc:source>\n")
	}
	if m.Sequence != nil && m.Sequence.Name != "" {
		w.WriteString(`    <meta property="belongs-to-collection" id="series">` + escText(m.Sequence.Name) + "</meta>\n")
		w.WriteString(`    <meta refines="#series" property="collection-type">series</meta>` + "\n")
		if m.Sequence.Number != "" {
			w.WriteString(`    <meta refines="#series" property="group-position">` + escText(m.Sequence.Number) + "</meta>\n")
		}
		w.WriteString(`    <meta name="calibre:series" content="` + escAttr(m.Sequence.Name) + `"/>` + "\n")
		if m.Sequence.Number != "" {
			w.WriteString(`    <meta name="calibre:series_index" content="` + escAttr(m.Sequence.Number) + `"/>` + "\n")
		}
	}
	if pl.coverImg != "" {
		w.WriteString(`    <meta name="cover" content="` + escAttr(coverManifestID) + `"/>` + "\n")
	}
	w.WriteString(`    <meta property="dcterms:modified">` + time.Now().UTC().Format("2006-01-02T15:04:05Z") + "</meta>\n")
	w.WriteString("  </metadata>\n")

	// --- manifest ---
	w.WriteString("  <manifest>\n")
	w.WriteString(`    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	w.WriteString(`    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")
	w.WriteString(`    <item id="css" href="style.css" media-type="text/css"/>` + "\n")
	for _, d := range pl.docs {
		w.WriteString(`    <item id="` + d.id + `" href="` + d.file + `" media-type="application/xhtml+xml"/>` + "\n")
	}
	for binID, imgPath := range pl.imgMap {
		id := imageManifestID(binID, pl)
		props := ""
		if binID == pl.coverImg {
			props = ` properties="cover-image"`
		}
		w.WriteString(`    <item id="` + id + `" href="` + escAttr(imgPath) + `" media-type="` + mediaTypeForPath(imgPath) + `"` + props + "/>\n")
	}
	w.WriteString("  </manifest>\n")

	// --- spine ---
	w.WriteString(`  <spine toc="ncx">` + "\n")
	for _, d := range pl.docs {
		w.WriteString(`    <itemref idref="` + d.id + `"/>` + "\n")
	}
	w.WriteString("  </spine>\n")

	// --- guide (EPUB2 compatibility for older Kindle) ---
	w.WriteString("  <guide>\n")
	if pl.coverImg != "" {
		w.WriteString(`    <reference type="cover" title="Обложка" href="text/cover.xhtml"/>` + "\n")
	}
	w.WriteString(`    <reference type="toc" title="Содержание" href="nav.xhtml"/>` + "\n")
	if first := firstChapter(pl); first != "" {
		w.WriteString(`    <reference type="text" title="Текст" href="` + escAttr(first) + `"/>` + "\n")
	}
	w.WriteString("  </guide>\n")

	w.WriteString("</package>\n")
	return w.String()
}

const coverManifestID = "cover-image"

func imageManifestID(binID string, pl *plan) string {
	if binID == pl.coverImg {
		return coverManifestID
	}
	return "img-" + safeID(binID)
}

func (b *builder) renderNav(pl *plan) string {
	var w strings.Builder
	w.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	w.WriteString(`<!DOCTYPE html>` + "\n")
	w.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + b.lang() + `" lang="` + b.lang() + `">` + "\n")
	w.WriteString("<head>\n<meta charset=\"utf-8\"/>\n<title>Содержание</title>\n</head>\n<body>\n")
	w.WriteString(`<nav epub:type="toc" id="toc">` + "\n<h1>Содержание</h1>\n")
	writeNavList(&w, pl.nav, "nav.xhtml")
	w.WriteString("</nav>\n")

	// Landmarks help readers jump to cover/text.
	w.WriteString(`<nav epub:type="landmarks" id="landmarks" hidden="">` + "\n<ol>\n")
	if pl.coverImg != "" {
		w.WriteString(`<li><a epub:type="cover" href="text/cover.xhtml">Обложка</a></li>` + "\n")
	}
	if first := firstChapter(pl); first != "" {
		w.WriteString(`<li><a epub:type="bodymatter" href="` + escAttr(first) + `">Текст</a></li>` + "\n")
	}
	w.WriteString("</ol>\n</nav>\n")
	w.WriteString("</body>\n</html>\n")
	return w.String()
}

func writeNavList(w *strings.Builder, nodes []*navNode, fromFile string) {
	if len(nodes) == 0 {
		return
	}
	w.WriteString("<ol>\n")
	for _, n := range nodes {
		w.WriteString(`<li><a href="` + escAttr(relHref(fromFile, n.file, n.frag)) + `">` + escText(n.title) + "</a>")
		if len(n.kids) > 0 {
			w.WriteString("\n")
			writeNavList(w, n.kids, fromFile)
		}
		w.WriteString("</li>\n")
	}
	w.WriteString("</ol>\n")
}

func (b *builder) renderNCX(pl *plan) string {
	var w strings.Builder
	w.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	w.WriteString(`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1" xml:lang="` + b.lang() + `">` + "\n")
	w.WriteString("  <head>\n")
	w.WriteString(`    <meta name="dtb:uid" content="` + escAttr(b.bookID()) + `"/>` + "\n")
	w.WriteString(`    <meta name="dtb:depth" content="` + strconv.Itoa(navDepth(pl.nav)) + `"/>` + "\n")
	w.WriteString(`    <meta name="dtb:totalPageCount" content="0"/>` + "\n")
	w.WriteString(`    <meta name="dtb:maxPageNumber" content="0"/>` + "\n")
	w.WriteString("  </head>\n")
	title := b.book.Meta.Title
	if title == "" {
		title = "Без названия"
	}
	w.WriteString("  <docTitle><text>" + escText(title) + "</text></docTitle>\n")
	w.WriteString("  <navMap>\n")
	order := 0
	writeNavPoints(&w, pl.nav, &order)
	w.WriteString("  </navMap>\n")
	w.WriteString("</ncx>\n")
	return w.String()
}

func writeNavPoints(w *strings.Builder, nodes []*navNode, order *int) {
	for _, n := range nodes {
		*order++
		id := "navpoint-" + strconv.Itoa(*order)
		w.WriteString(`    <navPoint id="` + id + `" playOrder="` + strconv.Itoa(*order) + `">` + "\n")
		w.WriteString("      <navLabel><text>" + escText(n.title) + "</text></navLabel>\n")
		w.WriteString(`      <content src="` + escAttr(relHref("toc.ncx", n.file, n.frag)) + `"/>` + "\n")
		writeNavPoints(w, n.kids, order)
		w.WriteString("    </navPoint>\n")
	}
}

func navDepth(nodes []*navNode) int {
	d := 0
	for _, n := range nodes {
		if cd := 1 + navDepth(n.kids); cd > d {
			d = cd
		}
	}
	if d == 0 {
		return 1
	}
	return d
}

func firstChapter(pl *plan) string {
	for _, d := range pl.docs {
		if d.kind == kindChapter || d.kind == kindFront {
			return d.file
		}
	}
	return ""
}

func mediaTypeForPath(p string) string {
	switch {
	case strings.HasSuffix(p, ".jpg"), strings.HasSuffix(p, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".gif"):
		return "image/gif"
	case strings.HasSuffix(p, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(p, ".webp"):
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func personDisplay(p model.Person) string {
	if p.Nick != "" && p.First == "" && p.Last == "" {
		return p.Nick
	}
	parts := make([]string, 0, 3)
	for _, s := range []string{p.First, p.Middle, p.Last} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

func personFileAs(p model.Person) string {
	if p.Last == "" {
		return personDisplay(p)
	}
	rest := strings.TrimSpace(p.First + " " + p.Middle)
	if rest == "" {
		return p.Last
	}
	return p.Last + ", " + rest
}

func itoa(i int) string { return strconv.Itoa(i) }

func pseudoUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
