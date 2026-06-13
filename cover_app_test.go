package main

import (
	"os"
	"strings"
	"testing"

	"litagent/core/rules"
)

// TestGenerateCoverAutoApplies locks in the UX fix: generating a cover marks it
// for export, so the user does not have to click "Применить" separately.
func TestGenerateCoverAutoApplies(t *testing.T) {
	if _, err := os.Stat(`C:\Windows\Fonts\georgiab.ttf`); err != nil {
		t.Skip("no system font")
	}
	if _, err := os.Stat("wallpaper"); err != nil {
		t.Skip("no wallpaper folder")
	}
	a := NewApp()
	a.path = sampleFB2
	if _, err := a.run(rules.DefaultApplyIDs()); err != nil {
		t.Fatalf("run: %v", err)
	}
	uri, err := a.GenerateCover(false)
	if err != nil {
		t.Fatalf("GenerateCover: %v", err)
	}
	if !strings.HasPrefix(uri, "data:image/jpeg;base64,") {
		t.Errorf("unexpected data uri: %.40s", uri)
	}
	if !a.useGenCover {
		t.Error("generating a cover should auto-mark it for export")
	}
	// And it is actually injected into the export copy.
	if a.prepareExportBook(a.book).Meta.CoverHref != genCoverBinaryID {
		t.Error("generated cover not present in export book")
	}
}

// TestPrepareExportBookDoesNotMutate verifies the A1 fix: applying a generated
// cover for export must not mutate the retained a.book, and ClearCover must
// truly revert to the original cover on the next export.
func TestPrepareExportBookDoesNotMutate(t *testing.T) {
	a := NewApp()
	a.path = sampleFB2
	if _, err := a.run(rules.DefaultApplyIDs()); err != nil {
		t.Fatalf("run: %v", err)
	}
	origHref := a.book.Meta.CoverHref // "cover.jpg" from the sample
	origBins := len(a.book.Binaries)

	// Simulate a generated, applied cover.
	a.coverData = []byte{0xFF, 0xD8, 0xFF, 0xD9} // tiny JPEG-ish blob
	a.coverCT = "image/jpeg"
	a.useGenCover = true

	exp := a.prepareExportBook(a.book)
	if exp.Meta.CoverHref != genCoverBinaryID {
		t.Errorf("export cover href = %q, want %q", exp.Meta.CoverHref, genCoverBinaryID)
	}
	// The retained book is untouched.
	if a.book.Meta.CoverHref != origHref {
		t.Errorf("a.book mutated: href = %q, want %q", a.book.Meta.CoverHref, origHref)
	}
	if len(a.book.Binaries) != origBins {
		t.Errorf("a.book.Binaries mutated: %d, want %d", len(a.book.Binaries), origBins)
	}

	// After ClearCover, export reverts to the original cover.
	a.ClearCover()
	exp2 := a.prepareExportBook(a.book)
	if exp2.Meta.CoverHref != origHref {
		t.Errorf("after ClearCover export href = %q, want original %q", exp2.Meta.CoverHref, origHref)
	}
}
