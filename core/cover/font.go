package cover

import (
	"fmt"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// fontFace parses a TTF/OTF file and creates faces of arbitrary sizes from it.
// The parsed font is cached so re-sizing during the fit search is cheap.
type fontFace struct {
	font *sfnt.Font
}

func loadFont(path string) (*fontFace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("шрифт %s: %w", path, err)
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("разбор шрифта %s: %w", path, err)
	}
	return &fontFace{font: f}, nil
}

// face returns a font.Face at the given pixel size (96 DPI → 1 point = 1 px).
func (ff *fontFace) face(sizePx float64) (font.Face, error) {
	return opentype.NewFace(ff.font, &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72, // with DPI 72, Size is effectively pixels
		Hinting: font.HintingFull,
	})
}
