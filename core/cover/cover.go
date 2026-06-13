// Package cover generates a neat book cover from a random background template:
// a centred white plate in the top half holds the title and author in a serif
// face. It is pure Go (no Wails/GUI dependency) so it can be unit-tested and
// reused. See the "Генерация обложки" plan.
package cover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Template is one background image found in the wallpaper folder.
type Template struct {
	Name string `json:"name"` // base file name, e.g. "forest.jpg"
	Path string `json:"path"` // absolute path
}

// imageExts are the background formats we can decode (see draw.go).
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// ListTemplates returns the image files in dir, sorted by name. A missing
// directory yields an empty list (not an error) so the UI can prompt the user.
func ListTemplates(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Template
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !imageExts[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		out = append(out, Template{Name: name, Path: filepath.Join(dir, name)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// WallpaperDir resolves the templates folder, preferring (in order): the
// LITAGENT_WALLPAPER override, a "wallpaper" folder next to the executable, then
// one in the current working directory. The first that exists wins; otherwise
// the exe-adjacent path is returned so the UI can tell the user where to drop
// images.
func WallpaperDir() string {
	if env := strings.TrimSpace(os.Getenv("LITAGENT_WALLPAPER")); env != "" {
		return env
	}
	var exeDir string
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Join(filepath.Dir(exe), "wallpaper")
		if isDir(exeDir) {
			return exeDir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if d := filepath.Join(cwd, "wallpaper"); isDir(d) {
			return d
		}
	}
	if exeDir != "" {
		return exeDir
	}
	return "wallpaper"
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Config holds the tunable cover parameters. Zero fields fall back to defaults
// (see withDefaults). It may be loaded from an optional wallpaper/cover.json so
// the look can be changed without rebuilding.
type Config struct {
	Width       int     `json:"width"`        // canvas width in px
	Height      int     `json:"height"`       // canvas height in px
	FontTitle   string  `json:"fontTitle"`    // path to the bold serif TTF for the title
	FontAuthor  string  `json:"fontAuthor"`   // path to the regular serif TTF for the author
	JPEGQuality int     `json:"jpegQuality"`  // 1..100
	PlateWidth  float64 `json:"plateWidth"`   // plate width as a fraction of canvas width
	PlateCenterY float64 `json:"plateCenterY"` // plate vertical centre as a fraction of height
}

// configFileName is the optional per-folder tuning file.
const configFileName = "cover.json"

// LoadConfig reads dir/cover.json if present, filling unset fields with
// defaults. A missing file is not an error.
func LoadConfig(dir string) (Config, error) {
	var c Config
	data, err := os.ReadFile(filepath.Join(dir, configFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return withDefaults(c), nil
		}
		return withDefaults(c), err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return withDefaults(c), fmt.Errorf("cover.json: %w", err)
	}
	return withDefaults(c), nil
}

// Default cover parameters. Georgia is a Cyrillic-capable serif shipped with
// Windows; Arial is the fallback if Georgia is missing (see font.go).
const (
	defaultWidth      = 1200
	defaultHeight     = 1920 // 5:8, the common e-book cover ratio
	defaultQuality    = 88
	winFonts          = `C:\Windows\Fonts`
	defaultPlateWidth = 0.80
	defaultPlateCentY = 0.30
)

func withDefaults(c Config) Config {
	// Width/Height intentionally left at 0 when unset: Generate then keeps the
	// template's own size and aspect ratio. cover.json may set them to force a
	// fixed cover size (defaultWidth/defaultHeight are the last-resort fallback).
	if c.JPEGQuality <= 0 || c.JPEGQuality > 100 {
		c.JPEGQuality = defaultQuality
	}
	if c.FontTitle == "" {
		c.FontTitle = firstExistingFont(filepath.Join(winFonts, "georgiab.ttf"), filepath.Join(winFonts, "arialbd.ttf"))
	}
	if c.FontAuthor == "" {
		c.FontAuthor = firstExistingFont(filepath.Join(winFonts, "georgia.ttf"), filepath.Join(winFonts, "arial.ttf"))
	}
	if c.PlateWidth <= 0 || c.PlateWidth > 1 {
		c.PlateWidth = defaultPlateWidth
	}
	if c.PlateCenterY <= 0 || c.PlateCenterY > 1 {
		c.PlateCenterY = defaultPlateCentY
	}
	return c
}

// firstExistingFont returns the first path that exists, or the last candidate
// (so the error surfaces at face-load time with a clear message).
func firstExistingFont(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if len(paths) > 0 {
		return paths[len(paths)-1]
	}
	return ""
}
