package cover

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"

	// Decoders for the supported template formats. webp registers itself via init.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/font"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

// Options carries the per-book inputs plus the (defaulted) visual config.
type Options struct {
	Title  string
	Author string
	Cfg    Config
}

// Generate composes a cover from the template image at templatePath and returns
// the encoded JPEG bytes and its content type.
func Generate(templatePath string, opts Options) ([]byte, string, error) {
	cfg := withDefaults(opts.Cfg)

	src, err := decodeImage(templatePath)
	if err != nil {
		return nil, "", err
	}

	// By default keep the template's own size and aspect ratio (so covers sized
	// for a specific Kindle screen are not re-cropped / letterboxed). cover.json
	// may override Width/Height to force a fixed size.
	sb := src.Bounds()
	if cfg.Width <= 0 {
		cfg.Width = sb.Dx()
	}
	if cfg.Height <= 0 {
		cfg.Height = sb.Dy()
	}
	if cfg.Width <= 0 {
		cfg.Width = defaultWidth
	}
	if cfg.Height <= 0 {
		cfg.Height = defaultHeight
	}

	canvas := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	drawCover(canvas, src)

	if err := drawPlate(canvas, cfg, opts.Title, opts.Author); err != nil {
		return nil, "", err
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: cfg.JPEGQuality}); err != nil {
		return nil, "", fmt.Errorf("кодирование JPEG: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}

func decodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("шаблон %s: %w", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("декод шаблона %s: %w", path, err)
	}
	return img, nil
}

// drawCover scales src to fill dst, centre-cropping the overflow (CSS object-fit:
// cover) so the background always covers the whole canvas with no distortion.
func drawCover(dst *image.RGBA, src image.Image) {
	db := dst.Bounds()
	sb := src.Bounds()
	dw, dh := db.Dx(), db.Dy()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 {
		return
	}
	// Same size → straight copy (no resampling, keeps the template crisp).
	if dw == sw && dh == sh {
		xdraw.Copy(dst, db.Min, src, sb, xdraw.Src, nil)
		return
	}
	// Source sub-rectangle matching the destination aspect ratio.
	dstAspect := float64(dw) / float64(dh)
	cropW, cropH := sw, sh
	if float64(sw)/float64(sh) > dstAspect {
		cropW = int(float64(sh) * dstAspect)
	} else {
		cropH = int(float64(sw) / dstAspect)
	}
	ox := sb.Min.X + (sw-cropW)/2
	oy := sb.Min.Y + (sh-cropH)/2
	srcRect := image.Rect(ox, oy, ox+cropW, oy+cropH)
	xdraw.CatmullRom.Scale(dst, db, src, srcRect, xdraw.Over, nil)
}

// drawPlate measures the title/author, draws the white plate (warm frame + soft
// shadow) in the top half and renders the text centred inside it.
func drawPlate(dst *image.RGBA, cfg Config, title, author string) error {
	if strings.TrimSpace(title) == "" {
		title = "Без названия"
	}
	titleFont, err := loadFont(cfg.FontTitle)
	if err != nil {
		return err
	}
	authorFont, err := loadFont(cfg.FontAuthor)
	if err != nil {
		return err
	}

	W, H := cfg.Width, cfg.Height
	plateW := int(float64(W) * cfg.PlateWidth)
	pad := int(float64(plateW) * 0.07)
	frame := maxi(2, W/400)
	innerW := plateW - 2*pad - 2*frame

	// Fit the title: shrink the face until it wraps to <= maxLines within innerW.
	titleMax := float64(H) * 0.052
	titleMin := float64(H) * 0.028
	const maxTitleLines = 4
	titleSize, titleFace, titleLines := fitText(titleFont, title, innerW, titleMax, titleMin, maxTitleLines)
	defer titleFace.Close()

	// Author: smaller, uppercased (small-caps feel), wrapped to <= 2 lines.
	authorSize := maxf(titleSize*0.5, float64(H)*0.020)
	aFace, err := authorFont.face(authorSize)
	if err != nil {
		return err
	}
	defer aFace.Close()
	var authorLines []string
	authorUpper := strings.ToUpper(strings.TrimSpace(author))
	if authorUpper != "" {
		authorLines = wrapLines(aFace, authorUpper, innerW, authorTracking(authorSize))
	}

	titleLineH := int(titleSize * 1.22)
	authorLineH := int(authorSize * 1.35)
	gap := int(titleSize * 0.55) // space between title block and author

	contentH := len(titleLines) * titleLineH
	if len(authorLines) > 0 {
		contentH += gap + len(authorLines)*authorLineH
	}
	plateH := contentH + 2*pad + 2*frame

	cx := W / 2
	cy := int(float64(H) * cfg.PlateCenterY)
	plateRect := image.Rect(cx-plateW/2, cy-plateH/2, cx+plateW/2, cy+plateH/2)
	radius := maxi(8, W/40)

	// Soft shadow, warm frame, white plate (drawn back-to-front).
	shadow := plateRect.Add(image.Pt(maxi(6, W/120), maxi(8, H/140)))
	fillRoundRect(dst, shadow, radius, color.RGBA{0, 0, 0, 70})
	fillRoundRect(dst, plateRect, radius, color.RGBA{182, 158, 110, 255}) // warm gold frame
	white := plateRect.Inset(frame)
	fillRoundRect(dst, white, maxi(2, radius-frame), color.RGBA{255, 255, 255, 255})

	// Text, vertically stacked from the inner top, each line horizontally centred.
	titleCol := image.NewUniform(color.RGBA{26, 26, 26, 255})
	authorCol := image.NewUniform(color.RGBA{90, 72, 40, 255})
	y := plateRect.Min.Y + frame + pad
	for _, ln := range titleLines {
		baseline := y + int(titleSize*0.82)
		drawCenteredLine(dst, titleFace, titleCol, ln, cx, baseline, 0)
		y += titleLineH
	}
	if len(authorLines) > 0 {
		y += gap
		tr := authorTracking(authorSize)
		for _, ln := range authorLines {
			baseline := y + int(authorSize*0.82)
			drawCenteredLine(dst, aFace, authorCol, ln, cx, baseline, tr)
			y += authorLineH
		}
	}
	return nil
}

// authorTracking is the extra advance (px) inserted between author glyphs for an
// airy small-caps look. Scales with the font size.
func authorTracking(sizePx float64) fixed.Int26_6 {
	return fixed.I(int(sizePx * 0.08))
}

// fitText finds the largest face size in [min,max] whose wrapped lines fit within
// maxW and number <= maxLines, returning the size, face and lines. Falls back to
// the minimum size if nothing fits.
func fitText(ff *fontFace, text string, maxW int, max, min float64, maxLines int) (float64, font.Face, []string) {
	var lastFace font.Face
	for size := max; size >= min; size -= 2 {
		face, err := ff.face(size)
		if err != nil {
			continue
		}
		lines := wrapLines(face, text, maxW, 0)
		if len(lines) <= maxLines && widestLine(face, lines, 0) <= maxW {
			if lastFace != nil {
				lastFace.Close()
			}
			return size, face, lines
		}
		if lastFace != nil {
			lastFace.Close()
		}
		lastFace = face
	}
	if lastFace != nil {
		lines := wrapLines(lastFace, text, maxW, 0)
		return min, lastFace, lines
	}
	face, _ := ff.face(min)
	return min, face, wrapLines(face, text, maxW, 0)
}

// wrapLines greedily word-wraps text so each line's measured width (plus optional
// per-glyph tracking) stays within maxW. A single over-long word is kept on its
// own line rather than dropped.
func wrapLines(face font.Face, text string, maxW int, tracking fixed.Int26_6) []string {
	words := strings.Fields(text)
	var lines []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if cur == "" || measure(face, try, tracking) <= maxW {
			cur = try
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func widestLine(face font.Face, lines []string, tracking fixed.Int26_6) int {
	w := 0
	for _, ln := range lines {
		if m := measure(face, ln, tracking); m > w {
			w = m
		}
	}
	return w
}

// measure returns the rendered width of s in px, including inter-glyph tracking.
func measure(face font.Face, s string, tracking fixed.Int26_6) int {
	adv := font.MeasureString(face, s)
	if tracking > 0 {
		if n := len([]rune(s)); n > 1 {
			adv += tracking * fixed.Int26_6(n-1)
		}
	}
	return adv.Round()
}

// drawCenteredLine draws s centred horizontally on cx with its baseline at y.
// When tracking > 0 the glyphs are drawn one by one with extra spacing.
func drawCenteredLine(dst *image.RGBA, face font.Face, src image.Image, s string, cx, y int, tracking fixed.Int26_6) {
	w := measure(face, s, tracking)
	startX := cx - w/2
	d := &font.Drawer{Dst: dst, Src: src, Face: face, Dot: fixed.P(startX, y)}
	if tracking == 0 {
		d.DrawString(s)
		return
	}
	for _, r := range s {
		d.DrawString(string(r))
		d.Dot.X += tracking
	}
}

// fillRoundRect fills a rounded rectangle in col, alpha-blending onto dst so the
// translucent shadow reads softly over the background.
func fillRoundRect(dst *image.RGBA, rect image.Rectangle, r int, col color.RGBA) {
	rect = rect.Intersect(dst.Bounds())
	if rect.Empty() {
		return
	}
	if r > rect.Dx()/2 {
		r = rect.Dx() / 2
	}
	if r > rect.Dy()/2 {
		r = rect.Dy() / 2
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		minX, maxX := rect.Min.X, rect.Max.X
		// Shrink the row inside the corner arcs.
		var dy int
		switch {
		case y < rect.Min.Y+r:
			dy = (rect.Min.Y + r) - y
		case y >= rect.Max.Y-r:
			dy = y - (rect.Max.Y - r - 1)
		}
		if dy > 0 {
			inset := r - isqrt(r*r-dy*dy)
			minX += inset
			maxX -= inset
		}
		for x := minX; x < maxX; x++ {
			blend(dst, x, y, col)
		}
	}
}

// blend composites col (straight alpha) onto the pixel at (x,y).
func blend(dst *image.RGBA, x, y int, col color.RGBA) {
	if col.A == 255 {
		dst.SetRGBA(x, y, col)
		return
	}
	bg := dst.RGBAAt(x, y)
	a := uint32(col.A)
	ia := 255 - a
	dst.SetRGBA(x, y, color.RGBA{
		R: uint8((uint32(col.R)*a + uint32(bg.R)*ia) / 255),
		G: uint8((uint32(col.G)*a + uint32(bg.G)*ia) / 255),
		B: uint8((uint32(col.B)*a + uint32(bg.B)*ia) / 255),
		A: 255,
	})
}

func isqrt(n int) int {
	if n < 0 {
		return 0
	}
	x := int(0)
	for (x+1)*(x+1) <= n {
		x++
	}
	return x
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
