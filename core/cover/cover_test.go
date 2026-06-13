package cover

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// writePNG creates a solid-colour PNG template of size w×h.
func writePNG(t *testing.T, path string, w, h int, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestListTemplates(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "b.png"), 4, 4, color.Black)
	writePNG(t, filepath.Join(dir, "a.jpg"), 4, 4, color.Black) // .jpg ext, png bytes — only the name matters here
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	tpls, err := ListTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(tpls) != 2 {
		t.Fatalf("got %d templates, want 2: %+v", len(tpls), tpls)
	}
	// Sorted by name: a.jpg before b.png.
	if tpls[0].Name != "a.jpg" || tpls[1].Name != "b.png" {
		t.Errorf("unexpected order: %+v", tpls)
	}
}

func TestListTemplatesMissingDir(t *testing.T) {
	tpls, err := ListTemplates(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if len(tpls) != 0 {
		t.Errorf("want 0 templates, got %d", len(tpls))
	}
}

func TestGenerate(t *testing.T) {
	cfg := withDefaults(Config{})
	if _, err := os.Stat(cfg.FontTitle); err != nil {
		t.Skipf("системный шрифт недоступен (%s), пропуск", cfg.FontTitle)
	}

	dir := t.TempDir()
	tpl := filepath.Join(dir, "bg.png")
	const tw, th = 800, 1200
	writePNG(t, tpl, tw, th, color.RGBA{40, 60, 90, 255}) // dark blue background

	data, ct, err := Generate(tpl, Options{Title: "Волоколамское шоссе", Author: "Александр Бек"})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "image/jpeg" || len(data) == 0 {
		t.Fatalf("bad output: ct=%q len=%d", ct, len(data))
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("output not valid JPEG: %v", err)
	}
	// Default keeps the template's own size and aspect ratio.
	if img.Bounds().Dx() != tw || img.Bounds().Dy() != th {
		t.Errorf("size = %v, want %dx%d (template size preserved)", img.Bounds(), tw, th)
	}

	// The white plate sits centred around PlateCenterY in the top half: sample a
	// pixel there and expect it to be near-white (the plate), not the dark bg.
	px := img.At(tw/2, int(float64(th)*cfg.PlateCenterY))
	r, g, b, _ := px.RGBA()
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Errorf("plate centre not white: rgb=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
}

func TestPickRandom(t *testing.T) {
	if PickRandom(0, -1) != -1 {
		t.Error("empty list should return -1")
	}
	if PickRandom(1, 0) != 0 {
		t.Error("single template should return 0")
	}
	for i := 0; i < 50; i++ {
		if got := PickRandom(3, 1); got == 1 || got < 0 || got > 2 {
			t.Fatalf("reshuffle returned %d (avoid=1)", got)
		}
	}
}
