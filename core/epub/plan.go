package epub

import (
	"fmt"
	"path"
	"strings"

	"litagent/core/model"
)

// defaultSplitThreshold is the approximate rendered-text size above which a
// chapter is split at nested-section boundaries to stay Kindle-friendly.
const defaultSplitThreshold = 256 * 1024

// docKind classifies a spine document for rendering.
type docKind int

const (
	kindCover docKind = iota
	kindFront
	kindChapter
	kindNotes
)

// doc is one XHTML file in the EPUB.
type doc struct {
	file  string // OEBPS-root-relative, e.g. "text/chap_0001.xhtml"
	id    string
	title string
	kind  docKind
	secs  []*model.Section // sections that begin in this file
}

// ref locates an anchor target within the EPUB.
type ref struct {
	file string
	frag string
}

// navNode is a hierarchical table-of-contents entry.
type navNode struct {
	title string
	file  string
	frag  string
	kids  []*navNode
}

// plan is the result of pass 1: file layout, anchor/image maps and the data the
// renderer needs to honour split points (which file each section lives in).
type plan struct {
	docs     []*doc
	nav      []*navNode
	idMap    map[string]ref
	imgMap   map[string]string          // binary id -> "images/<file>"
	coverImg string                     // binary id of the cover, "" if none
	fileOf   map[*model.Section]string    // section -> its XHTML file
	depthOf  map[*model.Section]int       // section -> tree depth (heading level)
	fragOf   map[*model.Section]string    // section -> stable anchor id
	paraFrag map[*model.Paragraph]string  // heading/id paragraph -> anchor id
	usedFrag map[string]bool              // anchor ids already handed out (uniqueness)
}

// uniqueFrag returns base if unused, otherwise base-2, base-3, … so every anchor
// id is unique across the book (two FB2 ids can sanitise to the same NCName).
func (pl *plan) uniqueFrag(base string) string {
	if base == "" {
		return ""
	}
	if !pl.usedFrag[base] {
		pl.usedFrag[base] = true
		return base
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", base, i)
		if !pl.usedFrag[c] {
			pl.usedFrag[c] = true
			return c
		}
	}
}

// buildPlan walks the book, assigns sections to files (splitting oversized
// chapters), and builds the maps needed to resolve cross-references in pass 2
// (DESIGN.md §3.1–3.2).
func (b *builder) buildPlan() *plan {
	pl := &plan{
		idMap:   map[string]ref{},
		imgMap:  map[string]string{},
		fileOf:   map[*model.Section]string{},
		depthOf:  map[*model.Section]int{},
		fragOf:   map[*model.Section]string{},
		paraFrag: map[*model.Paragraph]string{},
		usedFrag: map[string]bool{},
	}

	// Images first: render passes need the binary id -> path mapping.
	usedNames := map[string]bool{}
	for _, bin := range b.book.Binaries {
		pl.imgMap[bin.ID] = "images/" + b.imageFileName(bin, usedNames)
	}
	if href := b.book.Meta.CoverHref; href != "" {
		if _, ok := pl.imgMap[href]; ok {
			pl.coverImg = href
		}
	}

	// Cover document.
	if pl.coverImg != "" {
		pl.docs = append(pl.docs, &doc{file: "text/cover.xhtml", id: "cover", title: "Обложка", kind: kindCover})
	}

	sp := &splitter{pl: pl, threshold: b.threshold(), sizes: map[*model.Section]int{}}

	for _, body := range b.book.Bodies {
		// Body-level front matter (book/part title page), if any.
		if len(body.Title) > 0 || len(body.Epigraph) > 0 {
			f := fmt.Sprintf("text/front_%02d.xhtml", len(pl.docs))
			pl.docs = append(pl.docs, &doc{
				file:  f,
				id:    "front_" + idxStr(len(pl.docs)),
				title: blocksText(body.Title),
				kind:  kindFront,
				secs: []*model.Section{{
					Title:     body.Title,
					Epigraphs: epigraphSlices(body.Epigraph),
				}},
			})
		}

		prev := len(sp.units)
		for _, sec := range body.Sections {
			sp.processTop(sec)
			pl.nav = append(pl.nav, sp.navFor(sec))
		}
		for _, u := range sp.units[prev:] {
			pl.docs = append(pl.docs, &doc{file: u.file, id: u.id, title: u.title, kind: kindChapter, secs: u.roots})
		}
	}

	// Notes document with popup-footnote targets.
	if b.book.Notes != nil && len(b.book.Notes.Sections) > 0 {
		f := "text/notes.xhtml"
		pl.docs = append(pl.docs, &doc{file: f, id: "notes", title: "Примечания", kind: kindNotes, secs: b.book.Notes.Sections})
		for i, n := range b.book.Notes.Sections {
			frag := safeID(n.ID)
			if frag == "" {
				frag = fmt.Sprintf("note_%d", i+1)
			}
			frag = pl.uniqueFrag(frag)
			pl.fragOf[n] = frag
			if n.ID != "" {
				pl.idMap[n.ID] = ref{file: f, frag: frag}
			}
		}
	}

	return pl
}

func (b *builder) threshold() int {
	if b.opts.SplitThresholdBytes > 0 {
		return b.opts.SplitThresholdBytes
	}
	return defaultSplitThreshold
}

// fileUnit is one chapter XHTML file and the section(s) that begin in it.
type fileUnit struct {
	file  string
	id    string
	title string
	roots []*model.Section
}

// splitter assigns sections to files, breaking oversized chapters apart.
type splitter struct {
	pl        *plan
	threshold int
	counter   int
	auto      int
	units     []*fileUnit
	sizes     map[*model.Section]int
}

func (s *splitter) newFile() *fileUnit {
	s.counter++
	u := &fileUnit{
		file: fmt.Sprintf("text/chap_%04d.xhtml", s.counter),
		id:   fmt.Sprintf("chap_%04d", s.counter),
	}
	s.units = append(s.units, u)
	return u
}

// processTop lays out one top-level section, opening a fresh chapter file.
func (s *splitter) processTop(sec *model.Section) {
	u := s.newFile()
	u.roots = append(u.roots, sec)
	u.title = sectionTitle(sec)
	s.assign(sec, u.file, 1)
}

// assign places sec in `file`; if the subtree is too big it pushes children to
// their own files (batching small siblings together).
func (s *splitter) assign(sec *model.Section, file string, depth int) {
	s.record(sec, file, depth)

	if s.size(sec) <= s.threshold {
		for _, c := range sec.Children {
			s.assignInline(c, file, depth+1)
		}
		return
	}

	var cur *fileUnit
	var curSize int
	for _, c := range sec.Children {
		cs := s.size(c)
		if cs > s.threshold {
			cu := s.newFile()
			cu.roots = append(cu.roots, c)
			cu.title = sectionTitle(c)
			s.assign(c, cu.file, depth+1)
			cur, curSize = nil, 0
			continue
		}
		if cur == nil || curSize+cs > s.threshold {
			cur = s.newFile()
			curSize = 0
		}
		cur.roots = append(cur.roots, c)
		if cur.title == "" {
			cur.title = sectionTitle(c)
		}
		s.assignInline(c, cur.file, depth+1)
		curSize += cs
	}
}

// assignInline keeps an entire subtree within one file (no further splitting).
func (s *splitter) assignInline(sec *model.Section, file string, depth int) {
	s.record(sec, file, depth)
	for _, c := range sec.Children {
		s.assignInline(c, file, depth+1)
	}
}

// record stores the per-section placement and registers anchor targets.
func (s *splitter) record(sec *model.Section, file string, depth int) {
	s.pl.fileOf[sec] = file
	s.pl.depthOf[sec] = depth

	frag := safeID(sec.ID)
	if frag == "" {
		s.auto++
		frag = fmt.Sprintf("sec_%d", s.auto)
	}
	frag = s.pl.uniqueFrag(frag)
	s.pl.fragOf[sec] = frag
	if sec.ID != "" {
		s.pl.idMap[sec.ID] = ref{file: file, frag: frag}
	}
	for _, blk := range sec.Content {
		p, ok := blk.(*model.Paragraph)
		if !ok {
			continue
		}
		if p.ID == "" && !p.Heading {
			continue
		}
		frag := safeID(p.ID)
		if frag == "" {
			s.auto++
			frag = fmt.Sprintf("h_%d", s.auto)
		}
		frag = s.pl.uniqueFrag(frag)
		s.pl.paraFrag[p] = frag
		if p.ID != "" {
			s.pl.idMap[p.ID] = ref{file: file, frag: frag}
		}
	}
}

func (s *splitter) navFor(sec *model.Section) *navNode {
	n := &navNode{title: sectionTitle(sec), file: s.pl.fileOf[sec], frag: s.pl.fragOf[sec]}
	// Accepted in-content headings become nav children (document order).
	for _, blk := range sec.Content {
		if p, ok := blk.(*model.Paragraph); ok && p.Heading {
			n.kids = append(n.kids, &navNode{
				title: inlinesText(p.Inlines),
				file:  s.pl.fileOf[sec],
				frag:  s.pl.paraFrag[p],
			})
		}
	}
	for _, c := range sec.Children {
		n.kids = append(n.kids, s.navFor(c))
	}
	return n
}

// size returns a memoised estimate of a section subtree's rendered text size.
func (s *splitter) size(sec *model.Section) int {
	if v, ok := s.sizes[sec]; ok {
		return v
	}
	total := blocksSize(sec.Title) + blocksSize(sec.Content) + blocksSize(sec.Annotation)
	for _, ep := range sec.Epigraphs {
		total += blocksSize(ep)
	}
	for _, c := range sec.Children {
		total += s.size(c)
	}
	s.sizes[sec] = total
	return total
}

// --- size estimation ------------------------------------------------------

func blocksSize(blocks []model.Block) int {
	n := 0
	for _, b := range blocks {
		n += blockSize(b)
	}
	return n
}

func blockSize(b model.Block) int {
	const tagOverhead = 30
	switch n := b.(type) {
	case *model.Paragraph:
		return tagOverhead + inlinesSize(n.Inlines)
	case *model.Subtitle:
		return tagOverhead + inlinesSize(n.Inlines)
	case *model.EmptyLine:
		return tagOverhead
	case *model.Poem:
		sz := tagOverhead
		for _, st := range n.Stanzas {
			for _, v := range st.Verses {
				sz += tagOverhead + inlinesSize(v)
			}
		}
		return sz
	case *model.Cite:
		return tagOverhead + blocksSize(n.Content) + inlinesSize(n.TextAuthor)
	case *model.Epigraph:
		return tagOverhead + blocksSize(n.Content) + inlinesSize(n.TextAuthor)
	case *model.Annotation:
		return tagOverhead + blocksSize(n.Content)
	case *model.TextAuthor:
		return tagOverhead + inlinesSize(n.Inlines)
	case *model.Table:
		sz := tagOverhead
		for _, row := range n.Rows {
			for _, c := range row.Cells {
				sz += tagOverhead + inlinesSize(c.Inlines)
			}
		}
		return sz
	default:
		return tagOverhead
	}
}

func inlinesSize(in []model.Inline) int {
	n := 0
	for _, x := range in {
		switch v := x.(type) {
		case *model.Text:
			n += len(v.Value)
		case *model.Styled:
			n += 15 + inlinesSize(v.Children)
		case *model.Link:
			n += 30 + inlinesSize(v.Children)
		case *model.InlineImage:
			n += 40
		}
	}
	return n
}

// --- shared helpers -------------------------------------------------------

func (b *builder) imageFileName(bin model.Binary, used map[string]bool) string {
	name := sanitizeFileName(bin.ID)
	if path.Ext(name) == "" {
		name += extForContentType(bin.ContentType)
	}
	base, ext := strings.TrimSuffix(name, path.Ext(name)), path.Ext(name)
	candidate := name
	for i := 1; used[candidate]; i++ {
		candidate = fmt.Sprintf("%s_%d%s", base, i, ext)
	}
	used[candidate] = true
	return candidate
}

func sectionTitle(sec *model.Section) string {
	if t := blocksText(sec.Title); t != "" {
		return t
	}
	return "Без названия"
}

func blocksText(blocks []model.Block) string {
	var parts []string
	for _, b := range blocks {
		switch n := b.(type) {
		case *model.Paragraph:
			if s := inlinesText(n.Inlines); s != "" {
				parts = append(parts, s)
			}
		case *model.Subtitle:
			if s := inlinesText(n.Inlines); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func inlinesText(in []model.Inline) string {
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
	return strings.TrimSpace(sb.String())
}

func epigraphSlices(blocks []model.Block) [][]model.Block {
	if len(blocks) == 0 {
		return nil
	}
	return [][]model.Block{blocks}
}

func idxStr(i int) string { return fmt.Sprintf("%02d", i) }

func sanitizeFileName(name string) string {
	name = path.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || out == "." {
		out = "img"
	}
	return out
}

func extForContentType(ct string) string {
	switch strings.ToLower(strings.TrimSpace(ct)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	case "image/webp":
		return ".webp"
	default:
		return ".img"
	}
}
