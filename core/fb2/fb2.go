// Package fb2 parses FB2 files into the format-neutral model.Book.
//
// FB2 in the wild is frequently encoded in windows-1251 and is often produced
// by OCR, so parsing must detect/transcode the encoding (DESIGN.md §2.1) and be
// tolerant of malformed markup. Parsing is done with a manual token walker over
// encoding/xml so element order and rich inline content are preserved exactly.
package fb2

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	"litagent/core/model"
)

// ErrNotFB2 is returned when no <FictionBook> root is found.
var ErrNotFB2 = errors.New("fb2: no FictionBook root element")

// Parse reads an FB2 document and returns the parsed Book.
func Parse(r io.Reader) (*model.Book, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	utf8Bytes, _, err := decodeToUTF8(raw)
	if err != nil {
		return nil, err
	}

	dec := xml.NewDecoder(bytes.NewReader(utf8Bytes))
	dec.Strict = false           // tolerate malformed FB2
	dec.Entity = xml.HTMLEntity  // accept &nbsp; &mdash; etc.
	// Bytes are already UTF-8; treat any declared charset as identity so the
	// decoder never rejects an unsupported-encoding declaration.
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	p := &parser{dec: dec, book: &model.Book{}}
	return p.parse()
}

type parser struct {
	dec  *xml.Decoder
	book *model.Book
}

// token returns the next token, or nil on EOF / unrecoverable error.
func (p *parser) token() xml.Token {
	t, err := p.dec.Token()
	if err != nil {
		return nil
	}
	return t
}

// skip consumes the remaining content of an already-opened element.
func (p *parser) skip() {
	depth := 1
	for depth > 0 {
		switch p.token().(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case nil:
			return
		}
	}
}

func (p *parser) parse() (*model.Book, error) {
	for {
		t := p.token()
		if t == nil {
			break
		}
		if se, ok := t.(xml.StartElement); ok && local(se.Name) == "FictionBook" {
			p.parseFictionBook()
			break
		}
	}
	if len(p.book.Bodies) == 0 && p.book.Meta.Title == "" {
		return nil, ErrNotFB2
	}
	return p.book, nil
}

func (p *parser) parseFictionBook() {
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "description":
				p.parseDescription()
			case "body":
				b := p.parseBody(e)
				if b.Name == "notes" && p.book.Notes == nil {
					p.book.Notes = b
				} else {
					p.book.Bodies = append(p.book.Bodies, b)
				}
			case "binary":
				p.parseBinary(e)
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "FictionBook" {
				return
			}
		case nil:
			return
		}
	}
}

// --- metadata -------------------------------------------------------------

func (p *parser) parseDescription() {
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "title-info":
				p.parseTitleInfo()
			case "document-info":
				p.parseDocInfo()
			case "publish-info":
				p.parsePublishInfo()
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "description" {
				return
			}
		case nil:
			return
		}
	}
}

func (p *parser) parseTitleInfo() {
	m := &p.book.Meta
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "book-title":
				m.Title = p.textOf("book-title")
			case "author":
				m.Authors = append(m.Authors, p.parsePerson("author"))
			case "translator":
				m.Translators = append(m.Translators, p.parsePerson("translator"))
			case "annotation":
				m.Annotation = p.parseBlockChildren("annotation", nil)
			case "keywords":
				m.Keywords = splitKeywords(p.textOf("keywords"))
			case "date":
				m.Date = p.textOf("date")
			case "lang":
				m.Lang = p.textOf("lang")
			case "src-lang":
				m.SrcLang = p.textOf("src-lang")
			case "sequence":
				m.Sequence = &model.Sequence{Name: attr(e, "name"), Number: attr(e, "number")}
				p.skip()
			case "coverpage":
				p.parseCoverpage()
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "title-info" {
				return
			}
		case nil:
			return
		}
	}
}

func (p *parser) parseCoverpage() {
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			if local(e.Name) == "image" {
				p.book.Meta.CoverHref = trimHash(xlinkHref(e))
			}
			p.skip()
		case xml.EndElement:
			if local(e.Name) == "coverpage" {
				return
			}
		case nil:
			return
		}
	}
}

func (p *parser) parsePerson(end string) model.Person {
	var person model.Person
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "first-name":
				person.First = p.textOf("first-name")
			case "middle-name":
				person.Middle = p.textOf("middle-name")
			case "last-name":
				person.Last = p.textOf("last-name")
			case "nickname":
				person.Nick = p.textOf("nickname")
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == end {
				return person
			}
		case nil:
			return person
		}
	}
}

func (p *parser) parseDocInfo() {
	m := &p.book.Meta
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "id":
				m.DocID = p.textOf("id")
			case "program-used":
				m.ProgramUsed = p.textOf("program-used")
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "document-info" {
				return
			}
		case nil:
			return
		}
	}
}

func (p *parser) parsePublishInfo() {
	m := &p.book.Meta
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "publisher":
				m.Publisher = p.textOf("publisher")
			case "city":
				m.City = p.textOf("city")
			case "year":
				m.Year = p.textOf("year")
			case "isbn":
				m.ISBN = p.textOf("isbn")
			case "book-name":
				m.OriginalName = p.textOf("book-name")
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "publish-info" {
				return
			}
		case nil:
			return
		}
	}
}

// --- body / sections ------------------------------------------------------

func (p *parser) parseBody(start xml.StartElement) *model.Body {
	b := &model.Body{Name: attr(start, "name")}
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "title":
				b.Title = p.parseBlockChildren("title", nil)
			case "epigraph":
				if blk := p.parseBlockElem(e); blk != nil {
					b.Epigraph = append(b.Epigraph, blk)
				}
			case "image":
				b.Image = &model.Image{Href: trimHash(xlinkHref(e)), Alt: attr(e, "alt"), Title: attr(e, "title")}
				p.skip()
			case "section":
				b.Sections = append(b.Sections, p.parseSection(e))
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "body" {
				return b
			}
		case nil:
			return b
		}
	}
}

func (p *parser) parseSection(start xml.StartElement) *model.Section {
	sec := &model.Section{ID: attr(start, "id")}
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "title":
				sec.Title = p.parseBlockChildren("title", nil)
			case "epigraph":
				if blk := p.parseBlockElem(e); blk != nil {
					sec.Epigraphs = append(sec.Epigraphs, []model.Block{blk})
				}
			case "annotation":
				sec.Annotation = p.parseBlockChildren("annotation", nil)
			case "image":
				sec.Image = &model.Image{Href: trimHash(xlinkHref(e)), Alt: attr(e, "alt"), Title: attr(e, "title")}
				p.skip()
			case "section":
				sec.Children = append(sec.Children, p.parseSection(e))
			default:
				if blk := p.parseBlockElem(e); blk != nil {
					sec.Content = append(sec.Content, blk)
				}
			}
		case xml.EndElement:
			if local(e.Name) == "section" {
				return sec
			}
		case nil:
			return sec
		}
	}
}

// parseBlockChildren collects block-level children until the matching end tag.
// If onTextAuthor is non-nil, <text-author> is routed to it instead of becoming
// a block (used by cite/epigraph attribution).
func (p *parser) parseBlockChildren(end string, onTextAuthor func([]model.Inline)) []model.Block {
	var blocks []model.Block
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			if local(e.Name) == "text-author" && onTextAuthor != nil {
				onTextAuthor(p.parseInlines("text-author"))
				continue
			}
			if blk := p.parseBlockElem(e); blk != nil {
				blocks = append(blocks, blk)
			}
		case xml.EndElement:
			if local(e.Name) == end {
				return blocks
			}
		case nil:
			return blocks
		}
	}
}

// parseBlockElem dispatches a single block-level element that has just been
// opened. Returns nil for unrecognised elements (whose subtree is skipped).
func (p *parser) parseBlockElem(e xml.StartElement) model.Block {
	switch local(e.Name) {
	case "p":
		return &model.Paragraph{ID: attr(e, "id"), Style: attr(e, "style"), Inlines: p.parseInlines("p")}
	case "subtitle":
		return &model.Subtitle{Inlines: p.parseInlines("subtitle")}
	case "empty-line":
		p.skip()
		return &model.EmptyLine{}
	case "poem":
		return p.parsePoem()
	case "cite":
		c := &model.Cite{}
		c.Content = p.parseBlockChildren("cite", func(ta []model.Inline) { c.TextAuthor = ta })
		return c
	case "epigraph":
		ep := &model.Epigraph{}
		ep.Content = p.parseBlockChildren("epigraph", func(ta []model.Inline) { ep.TextAuthor = ta })
		return ep
	case "annotation":
		return &model.Annotation{Content: p.parseBlockChildren("annotation", nil)}
	case "table":
		return p.parseTable()
	case "image":
		p.skip()
		return &model.Image{Href: trimHash(xlinkHref(e)), Alt: attr(e, "alt"), Title: attr(e, "title")}
	case "text-author":
		return &model.TextAuthor{Inlines: p.parseInlines("text-author")}
	default:
		p.skip()
		return nil
	}
}

func (p *parser) parsePoem() *model.Poem {
	poem := &model.Poem{}
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "title":
				poem.Title = p.parseBlockChildren("title", nil)
			case "epigraph":
				if blk := p.parseBlockElem(e); blk != nil {
					poem.Epigraph = append(poem.Epigraph, blk)
				}
			case "stanza":
				poem.Stanzas = append(poem.Stanzas, p.parseStanza())
			case "text-author":
				poem.TextAuthor = p.parseInlines("text-author")
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "poem" {
				return poem
			}
		case nil:
			return poem
		}
	}
}

func (p *parser) parseStanza() *model.Stanza {
	st := &model.Stanza{}
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			switch local(e.Name) {
			case "title":
				st.Title = p.parseBlockChildren("title", nil)
			case "subtitle":
				st.Subtitle = p.parseInlines("subtitle")
			case "v":
				st.Verses = append(st.Verses, p.parseInlines("v"))
			default:
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "stanza" {
				return st
			}
		case nil:
			return st
		}
	}
}

func (p *parser) parseTable() *model.Table {
	t := &model.Table{}
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			if local(e.Name) == "tr" {
				t.Rows = append(t.Rows, p.parseTableRow())
			} else {
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "table" {
				return t
			}
		case nil:
			return t
		}
	}
}

func (p *parser) parseTableRow() model.TableRow {
	var row model.TableRow
	for {
		switch e := p.token().(type) {
		case xml.StartElement:
			name := local(e.Name)
			if name == "th" || name == "td" {
				row.Cells = append(row.Cells, model.TableCell{
					Header:  name == "th",
					Align:   attr(e, "align"),
					ColSpan: atoi(attr(e, "colspan")),
					RowSpan: atoi(attr(e, "rowspan")),
					Inlines: p.parseInlines(name),
				})
			} else {
				p.skip()
			}
		case xml.EndElement:
			if local(e.Name) == "tr" {
				return row
			}
		case nil:
			return row
		}
	}
}

// --- inline content -------------------------------------------------------

// parseInlines collects inline nodes until the matching end tag, coalescing
// adjacent text runs.
func (p *parser) parseInlines(end string) []model.Inline {
	var out []model.Inline
	addText := func(s string) {
		if s == "" {
			return
		}
		if n := len(out); n > 0 {
			if t, ok := out[n-1].(*model.Text); ok {
				t.Value += s
				return
			}
		}
		out = append(out, &model.Text{Value: s})
	}
	for {
		switch e := p.token().(type) {
		case xml.CharData:
			addText(string(e))
		case xml.StartElement:
			out = append(out, p.parseInlineElems(e)...)
		case xml.EndElement:
			if local(e.Name) == end {
				return out
			}
		case nil:
			return out
		}
	}
}

// parseInlineElems handles one inline element. Unknown wrappers are flattened to
// their inline children so no text is lost.
func (p *parser) parseInlineElems(e xml.StartElement) []model.Inline {
	name := local(e.Name)
	styled := func(kind model.StyleKind) []model.Inline {
		return []model.Inline{&model.Styled{Kind: kind, Children: p.parseInlines(name)}}
	}
	switch name {
	case "strong":
		return styled(model.StyleStrong)
	case "emphasis":
		return styled(model.StyleEmphasis)
	case "strikethrough":
		return styled(model.StyleStrikethrough)
	case "sub":
		return styled(model.StyleSub)
	case "sup":
		return styled(model.StyleSup)
	case "code":
		return styled(model.StyleCode)
	case "style":
		return []model.Inline{&model.Styled{Kind: model.StyleNamed, Name: attr(e, "name"), Children: p.parseInlines("style")}}
	case "a":
		return []model.Inline{&model.Link{
			Href:     xlinkHref(e),
			Note:     attr(e, "type") == "note",
			Children: p.parseInlines("a"),
		}}
	case "image":
		p.parseInlines("image") // consume to end (usually empty)
		return []model.Inline{&model.InlineImage{Href: trimHash(xlinkHref(e)), Alt: attr(e, "alt")}}
	default:
		// Unknown inline wrapper: keep its children, drop the wrapper.
		return p.parseInlines(name)
	}
}

// --- binaries -------------------------------------------------------------

func (p *parser) parseBinary(start xml.StartElement) {
	id := attr(start, "id")
	ct := attr(start, "content-type")
	var sb strings.Builder
	for {
		switch e := p.token().(type) {
		case xml.CharData:
			sb.Write(e)
		case xml.EndElement:
			if local(e.Name) == "binary" {
				data, err := base64.StdEncoding.DecodeString(stripSpace(sb.String()))
				if err == nil && id != "" {
					p.book.Binaries = append(p.book.Binaries, model.Binary{ID: id, ContentType: ct, Data: data})
				}
				return
			}
		case nil:
			return
		}
	}
}

// --- small helpers --------------------------------------------------------

// textOf reads the text content of the current element, ignoring nested markup.
func (p *parser) textOf(end string) string {
	var sb strings.Builder
	for {
		switch e := p.token().(type) {
		case xml.CharData:
			sb.Write(e)
		case xml.StartElement:
			p.skip()
		case xml.EndElement:
			if local(e.Name) == end {
				return strings.TrimSpace(sb.String())
			}
		case nil:
			return strings.TrimSpace(sb.String())
		}
	}
}

func local(n xml.Name) string { return n.Local }

// attr returns the value of an attribute by local name, ignoring namespace.
func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// xlinkHref returns the href attribute (xlink-namespaced or plain).
func xlinkHref(e xml.StartElement) string { return attr(e, "href") }

func trimHash(s string) string { return strings.TrimPrefix(s, "#") }

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, s)
}

func splitKeywords(s string) []string {
	var out []string
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
