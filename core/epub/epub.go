// Package epub builds clean EPUB3 output from a model.Book: chapter splitting,
// semantic XHTML, nav.xhtml + toc.ncx, popup footnotes and Kindle-safe CSS.
// See DESIGN.md §3. The build is two-pass to resolve cross-references.
package epub

import (
	"archive/zip"
	"errors"
	"io"

	"litagent/core/model"
)

// Options controls EPUB generation (chapter split threshold, etc.).
type Options struct {
	// SplitThresholdBytes splits an over-large chapter file to keep Kindle happy.
	// Zero means use the default. (Deep-splitting is a planned refinement;
	// currently one file per top-level section.)
	SplitThresholdBytes int
}

type builder struct {
	book *model.Book
	opts Options
}

// Build writes a complete EPUB3 (a ZIP container) for the given Book to w.
func Build(b *model.Book, opts Options, w io.Writer) error {
	if b == nil {
		return errors.New("epub: nil book")
	}
	bd := &builder{book: b, opts: opts}
	pl := bd.buildPlan()

	zw := zip.NewWriter(w)

	// mimetype MUST be the first entry and stored uncompressed.
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(mw, "application/epub+zip"); err != nil {
		return err
	}

	write := func(name, content string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, content)
		return err
	}

	if err := write("META-INF/container.xml", containerXML); err != nil {
		return err
	}
	if err := write("OEBPS/content.opf", bd.renderOPF(pl)); err != nil {
		return err
	}
	if err := write("OEBPS/nav.xhtml", bd.renderNav(pl)); err != nil {
		return err
	}
	if err := write("OEBPS/toc.ncx", bd.renderNCX(pl)); err != nil {
		return err
	}
	if err := write("OEBPS/style.css", kindleCSS); err != nil {
		return err
	}
	for _, d := range pl.docs {
		if err := write("OEBPS/"+d.file, bd.renderDoc(pl, d)); err != nil {
			return err
		}
	}

	// Image binaries.
	for _, bin := range b.Binaries {
		imgPath, ok := pl.imgMap[bin.ID]
		if !ok {
			continue
		}
		f, err := zw.Create("OEBPS/" + imgPath)
		if err != nil {
			return err
		}
		if _, err := f.Write(bin.Data); err != nil {
			return err
		}
	}

	return zw.Close()
}
