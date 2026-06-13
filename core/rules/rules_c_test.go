package rules

import (
	"testing"

	"litagent/core/model"
)

func findByRule(fs []Finding, id string) *Finding {
	for i := range fs {
		if fs[i].RuleID == id {
			return &fs[i]
		}
	}
	return nil
}

// yo-restore is DISABLED in the pipeline (see catalog.go). The rule code and the
// dictionary are retained, so these tests cover lookupYo directly instead of via
// the pipeline.
func TestYoLookupRestoresAndPreservesCase(t *testing.T) {
	if got, ok := lookupYo("еще"); !ok || got != "ещё" {
		t.Errorf("lookupYo(\"еще\") = %q,%v; want ещё,true", got, ok)
	}
	if got, ok := lookupYo("Еще"); !ok || got != "Ещё" {
		t.Errorf("case not preserved: lookupYo(\"Еще\") = %q,%v; want Ещё,true", got, ok)
	}
}

func TestYoLookupSkipsAmbiguous(t *testing.T) {
	// "все" is ambiguous (все/всё) and must NOT be in the dictionary.
	if got, ok := lookupYo("все"); ok {
		t.Errorf("ambiguous word in dict: lookupYo(\"все\") = %q,%v; want _,false", got, ok)
	}
}

func TestMixedAlphabet(t *testing.T) {
	latinO := "o" // ASCII look-alike of Cyrillic о
	got, fs := runPara("хор" + latinO + "шо")
	if got != "хор"+latinO+"шо" {
		t.Errorf("text mutated by review-only rule: %q", got)
	}
	f := findByRule(fs, "mixed-alpha")
	if f == nil {
		t.Fatal("no mixed-alpha finding")
	}
	if f.After != "хорошо" {
		t.Errorf("proposed = %q, want хорошо", f.After)
	}
}

func TestOCRDigit(t *testing.T) {
	got, fs := runPara("это д0м большой")
	if got != "это д0м большой" {
		t.Errorf("text mutated by review-only rule: %q", got)
	}
	f := findByRule(fs, "ocr-digit")
	if f == nil {
		t.Fatal("no ocr-digit finding")
	}
	if f.Before != "д0м" || f.After != "дом" {
		t.Errorf("finding = %q -> %q, want д0м -> дом", f.Before, f.After)
	}
}

func TestDehyphDictGated(t *testing.T) {
	// Known word → auto-joined (level B), across a space or a line break.
	for _, in := range []string{"сло- во", "сло-\nво"} {
		got, fs := runPara(in)
		if got != "слово" {
			t.Errorf("in %q: known word not auto-joined: %q", in, got)
		}
		if f := findByRule(fs, "dehyph"); f == nil || f.Level != ConfB {
			t.Errorf("in %q: expected B-level dehyph, got %+v", in, f)
		}
	}
	// Unknown joined form → left as-is, suggested only (level C).
	got, fs := runPara("ккк- ппп")
	if got != "ккк- ппп" {
		t.Errorf("unknown join applied: %q", got)
	}
	if f := findByRule(fs, "dehyph"); f == nil || f.Level != ConfC {
		t.Errorf("expected C-level dehyph suggestion, got %+v", f)
	}
	// A real compound (hyphen, no space) is never matched.
	if _, fs3 := runPara("что-то там"); findByRule(fs3, "dehyph") != nil {
		t.Error("compound word wrongly flagged")
	}
}

func TestPageNumberDetect(t *testing.T) {
	if _, fs := runPara("42"); findByRule(fs, "page-number") == nil {
		t.Error("standalone number not flagged as page number")
	}
	if _, fs := runPara("Глава 42 началась"); findByRule(fs, "page-number") != nil {
		t.Error("normal paragraph wrongly flagged as page number")
	}
}

func TestHeadingDetect(t *testing.T) {
	// ALL-CAPS short line and a chapter-keyword line are detected.
	if _, fs := runPara("ГЛАВА ПЯТАЯ"); findByRule(fs, "heading-detect") == nil {
		t.Error("all-caps heading not detected")
	}
	if _, fs := runPara("Глава пятая"); findByRule(fs, "heading-detect") == nil {
		t.Error("chapter-keyword heading not detected")
	}
	// A normal sentence is not flagged.
	if _, fs := runPara("Был поздний вечер."); findByRule(fs, "heading-detect") != nil {
		t.Error("normal sentence flagged as heading")
	}
}

func TestHeadingDetectApplyGated(t *testing.T) {
	mk := func() (*model.Paragraph, *model.Book) {
		p := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: "ГЛАВА"}}}
		b := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{Content: []model.Block{p}}}}}}
		return p, b
	}
	// Default: detected but not applied.
	p1, b1 := mk()
	DefaultEngine().Run(b1)
	if p1.Heading {
		t.Error("heading applied without acceptance")
	}
	// Accepted: the paragraph is marked as a heading.
	p2, b2 := mk()
	NewEngineFor(func(id string) bool { return id == "heading-detect" }).Run(b2)
	if !p2.Heading {
		t.Error("heading not applied when accepted")
	}
}

func TestLevelBApplies(t *testing.T) {
	// Dialogue dash (B) applies; «еще» is left unchanged (yo-restore is disabled).
	got, fs := runPara("- еще не время")
	want := "— еще не" + nbsp + "время" // dialog-dash + nbsp-prep (B)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if findByRule(fs, "dialog-dash") == nil {
		t.Error("expected dialog-dash to apply")
	}
}
