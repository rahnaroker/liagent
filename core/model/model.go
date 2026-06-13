// Package model defines the format-neutral document AST shared by the FB2
// parser, the rule engine and the EPUB builder. See DESIGN.md §2.2.
package model

// Book is the root of the parsed document.
type Book struct {
	Meta     Metadata
	Bodies   []*Body  // main body first; additional bodies follow
	Notes    *Body    // body[name="notes"] extracted separately, may be nil
	Binaries []Binary // images and other embedded resources
}

// Metadata holds description/title-info/document-info/publish-info fields.
type Metadata struct {
	Title       string
	Authors     []Person
	Translators []Person
	Annotation  []Block // free-form blocks
	Keywords    []string
	Lang        string
	SrcLang     string
	Date        string
	Sequence    *Sequence // series name + number
	CoverHref   string    // binary id of the cover image, "" if none

	// document-info / publish-info
	DocID        string // FB2 document id; used as dc:identifier when present
	ProgramUsed  string
	Publisher    string
	City         string
	Year         string
	ISBN         string
	OriginalName string
}

// Person is an author or translator with split name parts (for file-as sorting).
type Person struct {
	First  string
	Middle string
	Last   string
	Nick   string
}

// Sequence is an FB2 <sequence name=".." number=".."/>.
type Sequence struct {
	Name   string
	Number string
}

// Body is one FB2 <body>. Name is "" for the main body, "notes" for footnotes.
type Body struct {
	Name     string
	Title    []Block // body-level title (book/part title page)
	Epigraph []Block
	Image    *Image
	Sections []*Section
}

// Section maps to an FB2 <section>; nests recursively (part -> chapter -> ...).
type Section struct {
	ID         string
	Title      []Block // title lines, used for nav/TOC text
	Epigraphs  [][]Block
	Image      *Image
	Annotation []Block
	Content    []Block   // block-level content of this section
	Children   []*Section // nested sections
}

// Block is any block-level node. Use a type switch / the Kind field to handle.
type Block interface{ block() }

// BlockKind discriminates concrete block types without reflection.
type BlockKind int

const (
	KindParagraph BlockKind = iota
	KindSubtitle
	KindEmptyLine
	KindPoem
	KindStanza
	KindCite
	KindEpigraph
	KindAnnotation
	KindTable
	KindImage
	KindTextAuthor
)

// Paragraph is a <p>; Inlines carries its rich content.
type Paragraph struct {
	ID      string
	Style   string // FB2 <p style=".."> class name, "" if none
	Inlines []Inline
	// Heading marks a paragraph that the user accepted as an (unmarked) heading;
	// the EPUB builder renders it as a heading and adds it to the navigation.
	Heading bool
}

// Subtitle is an FB2 <subtitle>.
type Subtitle struct{ Inlines []Inline }

// EmptyLine is an FB2 <empty-line/>; consecutive runs are collapsed on render.
type EmptyLine struct{}

// Poem groups stanzas with an optional title/epigraph and trailing author.
type Poem struct {
	Title      []Block
	Epigraph   []Block
	Stanzas    []*Stanza
	TextAuthor []Inline
}

// Stanza is one stanza of a poem; each verse line is preserved.
type Stanza struct {
	Title    []Block
	Subtitle []Inline
	Verses   [][]Inline // one slice of inlines per <v>
}

// Cite is a quoted block, optionally attributed.
type Cite struct {
	Content    []Block
	TextAuthor []Inline
}

// Epigraph is an epigraph block, optionally attributed.
type Epigraph struct {
	Content    []Block
	TextAuthor []Inline
}

// Annotation is an annotation block (e.g. on a section).
type Annotation struct{ Content []Block }

// Table is a simple table mapped to <table>.
type Table struct{ Rows []TableRow }

// TableRow is a <tr>.
type TableRow struct{ Cells []TableCell }

// TableCell is a <th>/<td> with optional spans and alignment.
type TableCell struct {
	Header  bool
	Align   string
	ColSpan int
	RowSpan int
	Inlines []Inline
}

// Image is a block-level image reference (href is the binary id without '#').
type Image struct {
	Href string
	Alt  string
	Title string
}

// TextAuthor is a standalone attribution line.
type TextAuthor struct{ Inlines []Inline }

func (*Paragraph) block()  {}
func (*Subtitle) block()   {}
func (*EmptyLine) block()  {}
func (*Poem) block()       {}
func (*Stanza) block()     {}
func (*Cite) block()       {}
func (*Epigraph) block()   {}
func (*Annotation) block() {}
func (*Table) block()      {}
func (*Image) block()      {}
func (*TextAuthor) block() {}

// Inline is any inline-level node within a paragraph or verse.
type Inline interface{ inline() }

// Text is a run of plain text. This is what the rule engine rewrites.
type Text struct{ Value string }

// Styled wraps inlines with a semantic style (strong/em/sub/sup/code/...).
type Styled struct {
	Kind     StyleKind
	Name     string // for KindNamedStyle: the FB2 <style name="..">
	Children []Inline
}

// StyleKind enumerates inline formatting.
type StyleKind int

const (
	StyleStrong StyleKind = iota
	StyleEmphasis
	StyleStrikethrough
	StyleSub
	StyleSup
	StyleCode
	StyleNamed
)

// Link is an <a l:href="..">; Note marks footnote references (type="note").
type Link struct {
	Href     string
	Note     bool
	Children []Inline
}

// InlineImage is an image used inline within text.
type InlineImage struct {
	Href string
	Alt  string
}

func (*Text) inline()        {}
func (*Styled) inline()      {}
func (*Link) inline()        {}
func (*InlineImage) inline() {}

// Binary is a decoded-on-demand embedded resource (image).
type Binary struct {
	ID          string
	ContentType string
	Data        []byte
}
