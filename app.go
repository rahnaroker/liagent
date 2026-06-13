package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"litagent/core/cover"
	"litagent/core/epub"
	"litagent/core/fb2"
	"litagent/core/model"
	"litagent/core/rules"
)

// App is the Wails backend: it holds the currently loaded book and exposes the
// load / re-analyse / export operations to the Svelte frontend.
type App struct {
	ctx  context.Context
	mu   sync.Mutex
	path string       // currently open FB2 path
	book *model.Book  // corrected book from the last run (for export)

	// Generated cover state (see cover_app.go).
	coverData   []byte           // last generated JPEG, nil if none
	coverCT     string           // its content type
	coverTpls   []cover.Template // templates from the wallpaper folder
	coverIdx    int              // index of the template last used (-1 if none)
	useGenCover bool             // embed the generated cover on export

	// Edited metadata, applied to the export copy (see meta_app.go). nil = none.
	metaEdit *MetaEdit
}

// NewApp creates a new App application struct.
func NewApp() *App { return &App{coverIdx: -1} }

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// --- types returned to the frontend (JSON-serialised by Wails) -------------

type MetaInfo struct {
	Title    string `json:"title"`
	Author   string `json:"author"`
	Lang     string `json:"lang"`
	Sections int    `json:"sections"`
	HasCover bool   `json:"hasCover"`
}

type RuleStat struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Level    string `json:"level"`
	Count    int    `json:"count"`
	Enabled  bool   `json:"enabled"`
}

type FindingView struct {
	RuleID  string `json:"ruleId"`
	Section string `json:"section"`
	Context string `json:"context"`
	Before  string `json:"before"`
	After   string `json:"after"`
}

type LoadResult struct {
	Path      string        `json:"path"`
	Name      string        `json:"name"`
	Meta      MetaInfo      `json:"meta"`
	Rules     []RuleStat    `json:"rules"`
	Findings  []FindingView `json:"findings"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
}

// maxFindingsSent caps the per-load instance payload; counts are always exact.
const maxFindingsSent = 4000

// --- bound methods ---------------------------------------------------------

// Open shows a file picker, loads the chosen FB2 and runs all rules.
func (a *App) Open() (*LoadResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Открыть FB2",
		Filters: []runtime.FileFilter{{DisplayName: "FB2 книги (*.fb2)", Pattern: "*.fb2"}},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	a.mu.Lock()
	a.path = path
	// Reset per-book export state for the newly opened file.
	a.coverData = nil
	a.coverCT = ""
	a.coverIdx = -1
	a.useGenCover = false
	a.metaEdit = nil
	a.mu.Unlock()
	// Default: apply A/B rules; C rules are surfaced as suggestions only.
	return a.run(rules.DefaultApplyIDs())
}

// Reload re-parses the open file and applies only the enabled rules. This is how
// the UI reverts an entire rule: untick it and reload.
func (a *App) Reload(enabled []string) (*LoadResult, error) {
	a.mu.Lock()
	path := a.path
	a.mu.Unlock()
	if path == "" {
		return nil, fmt.Errorf("файл не открыт")
	}
	return a.run(enabled)
}

// Export writes the corrected book to an EPUB chosen via a save dialog.
func (a *App) Export() (string, error) {
	a.mu.Lock()
	book := a.book
	path := a.path
	a.mu.Unlock()
	if book == nil {
		return "", fmt.Errorf("нет открытой книги")
	}
	// Default file name = the book title from metadata (edited + tags stripped),
	// falling back to the FB2 file name when the title is empty.
	title, _ := a.effectiveTitleAuthor(book)
	name := sanitizeFilename(cleanMetaTitle(title))
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	def := name + ".epub"
	out, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Сохранить EPUB",
		DefaultFilename: def,
		Filters:         []runtime.FileFilter{{DisplayName: "EPUB (*.epub)", Pattern: "*.epub"}},
	})
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", nil // cancelled
	}
	exportBook := a.prepareExportBook(book)

	w, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer w.Close()
	if err := epub.Build(exportBook, epub.Options{}, w); err != nil {
		return "", fmt.Errorf("сборка EPUB: %w", err)
	}
	return out, nil
}

// Preview renders the first chapter of the corrected book to HTML so the user
// can see the result of the applied rules.
func (a *App) Preview() (string, error) {
	a.mu.Lock()
	book := a.book
	a.mu.Unlock()
	if book == nil {
		return "", fmt.Errorf("нет открытой книги")
	}
	return previewHTML(book), nil
}

// --- internals -------------------------------------------------------------

func (a *App) run(enabled []string) (*LoadResult, error) {
	a.mu.Lock()
	path := a.path
	a.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	book, err := fb2.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("разбор FB2: %w", err)
	}

	enabledSet := make(map[string]bool, len(enabled))
	for _, id := range enabled {
		enabledSet[id] = true
	}
	eng := rules.NewEngineFor(func(id string) bool { return enabledSet[id] })
	findings := eng.Run(book)

	a.mu.Lock()
	a.book = book
	a.mu.Unlock()

	return buildResult(path, book, findings, enabledSet), nil
}

func buildResult(path string, book *model.Book, findings []rules.Finding, enabled map[string]bool) *LoadResult {
	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.RuleID]++
	}

	stats := make([]RuleStat, 0)
	for _, m := range rules.Catalog() {
		stats = append(stats, RuleStat{
			ID:       m.ID,
			Name:     m.Name,
			Category: m.Category,
			Level:    m.Level.String(),
			Count:    counts[m.ID],
			Enabled:  enabled[m.ID],
		})
	}

	views := make([]FindingView, 0, min(len(findings), maxFindingsSent))
	for _, f := range findings {
		if len(views) >= maxFindingsSent {
			break
		}
		views = append(views, FindingView{
			RuleID:  f.RuleID,
			Section: f.Section,
			Context: f.Context,
			Before:  f.Before,
			After:   f.After,
		})
	}

	return &LoadResult{
		Path:      path,
		Name:      filepath.Base(path),
		Meta:      metaInfo(book),
		Rules:     stats,
		Findings:  views,
		Total:     len(findings),
		Truncated: len(findings) > len(views),
	}
}

func metaInfo(book *model.Book) MetaInfo {
	m := book.Meta
	author := ""
	if len(m.Authors) > 0 {
		a := m.Authors[0]
		author = strings.TrimSpace(strings.Join(nonEmpty(a.First, a.Middle, a.Last), " "))
		if author == "" {
			author = a.Nick
		}
	}
	sections := 0
	for _, b := range book.Bodies {
		sections += len(b.Sections)
	}
	return MetaInfo{
		Title:    m.Title,
		Author:   author,
		Lang:     m.Lang,
		Sections: sections,
		HasCover: m.CoverHref != "",
	}
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
