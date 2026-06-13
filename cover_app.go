package main

import (
	"encoding/base64"
	"fmt"

	"litagent/core/cover"
	"litagent/core/model"
)

// genCoverBinaryID is the synthetic binary id used for a generated cover.
const genCoverBinaryID = "gen-cover"

// CoverInfoView is the cover-tab state sent to the frontend.
type CoverInfoView struct {
	HasOriginal     bool   `json:"hasOriginal"`
	OriginalDataUri string `json:"originalDataUri"`
	TemplateCount   int    `json:"templateCount"`
	WallpaperDir    string `json:"wallpaperDir"`
	Applied         bool   `json:"applied"` // generated cover is set to be used on export
}

// CoverInfo returns the current book's cover (as a data URI) and how many
// templates are available in the wallpaper folder.
func (a *App) CoverInfo() CoverInfoView {
	a.mu.Lock()
	book := a.book
	applied := a.useGenCover
	a.mu.Unlock()

	dir := cover.WallpaperDir()
	tpls, _ := cover.ListTemplates(dir)

	view := CoverInfoView{
		TemplateCount: len(tpls),
		WallpaperDir:  dir,
		Applied:       applied,
	}
	if book != nil {
		if uri := originalCoverDataURI(book); uri != "" {
			view.HasOriginal = true
			view.OriginalDataUri = uri
		}
	}
	return view
}

// GenerateCover renders a cover from a random template and returns it as a data
// URI for preview. When reshuffle is true it picks a different template than the
// one last used.
func (a *App) GenerateCover(reshuffle bool) (string, error) {
	a.mu.Lock()
	book := a.book
	a.mu.Unlock()
	if book == nil {
		return "", fmt.Errorf("нет открытой книги")
	}

	dir := cover.WallpaperDir()
	tpls, err := cover.ListTemplates(dir)
	if err != nil {
		return "", fmt.Errorf("чтение папки шаблонов: %w", err)
	}
	if len(tpls) == 0 {
		return "", fmt.Errorf("в папке %s нет картинок-шаблонов", dir)
	}

	avoid := -1
	if reshuffle {
		a.mu.Lock()
		avoid = a.coverIdx
		a.mu.Unlock()
	}
	idx := cover.PickRandom(len(tpls), avoid)

	cfg, err := cover.LoadConfig(dir)
	if err != nil {
		return "", err
	}
	title, author := a.effectiveTitleAuthor(book)
	data, ct, err := cover.Generate(tpls[idx].Path, cover.Options{
		Title:  title,
		Author: author,
		Cfg:    cfg,
	})
	if err != nil {
		return "", fmt.Errorf("генерация обложки: %w", err)
	}

	a.mu.Lock()
	a.coverData = data
	a.coverCT = ct
	a.coverTpls = tpls
	a.coverIdx = idx
	a.useGenCover = true // generating a cover means using it on export
	a.mu.Unlock()

	return dataURI(ct, data), nil
}

// ApplyCover marks the generated cover to be embedded on export.
func (a *App) ApplyCover() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.coverData == nil {
		return fmt.Errorf("сначала сгенерируйте обложку")
	}
	a.useGenCover = true
	return nil
}

// ClearCover reverts to the book's original cover on export.
func (a *App) ClearCover() {
	a.mu.Lock()
	a.useGenCover = false
	a.mu.Unlock()
}

// prepareExportBook returns a shallow copy of book with the generated cover
// applied (when the user accepted it). The original a.book is never mutated, so
// previewing/exporting repeatedly and ClearCover all stay consistent.
func (a *App) prepareExportBook(book *model.Book) *model.Book {
	a.mu.Lock()
	use := a.useGenCover
	data := a.coverData
	ct := a.coverCT
	a.mu.Unlock()

	cp := *book // shallow copy; Meta is a value, so editing cp.Meta is isolated
	if use && data != nil {
		// Fresh Binaries slice without any prior generated cover, plus the current.
		bins := make([]model.Binary, 0, len(book.Binaries)+1)
		for _, b := range book.Binaries {
			if b.ID != genCoverBinaryID {
				bins = append(bins, b)
			}
		}
		cp.Binaries = append(bins, model.Binary{ID: genCoverBinaryID, ContentType: ct, Data: data})
		cp.Meta.CoverHref = genCoverBinaryID
	}
	a.applyMetaEdit(&cp) // overlay edited metadata (no-op if none)
	return &cp
}

// originalCoverDataURI returns the book's existing cover image as a data URI, or
// "" if the book has no cover binary.
func originalCoverDataURI(book *model.Book) string {
	href := book.Meta.CoverHref
	if href == "" {
		return ""
	}
	for _, b := range book.Binaries {
		if b.ID == href && len(b.Data) > 0 {
			ct := b.ContentType
			if ct == "" {
				ct = "image/jpeg"
			}
			return dataURI(ct, b.Data)
		}
	}
	return ""
}

func dataURI(ct string, data []byte) string {
	return "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)
}
