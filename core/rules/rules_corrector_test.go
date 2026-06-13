package rules

import (
	"strings"
	"testing"

	"litagent/core/model"
)

// acceptOnly builds an engine that applies exactly one rule (used to check the
// accepted behaviour of review-only rules).
func acceptOnly(id, text string) string {
	para := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: text}}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{Content: []model.Block{para}}}}}}
	NewEngineFor(func(rid string) bool { return rid == id }).Run(book)
	return para.Inlines[0].(*model.Text).Value
}

func TestEllSpaced(t *testing.T) {
	got, _ := runPara("вот . . . и дальше")
	if strings.Contains(got, ". .") || !strings.Contains(got, "…") {
		t.Errorf("spaced ellipsis not collapsed: %q", got)
	}
	// A dot followed by a letter (abbreviation spacing) must stay intact.
	if got := applyA("аб т. е. вг"); strings.Contains(got, "…") {
		t.Errorf("abbreviation wrongly turned into ellipsis: %q", got)
	}
}

func TestEllTwo(t *testing.T) {
	// Default: detect-only — text unchanged, suggestion present.
	got, fs := runPara("да..нет")
	if got != "да..нет" {
		t.Errorf("review-only rule mutated text: %q", got)
	}
	f := findByRule(fs, "ell-two")
	if f == nil || f.Before != ".." || f.After != "…" {
		t.Fatalf("ell-two finding = %+v", f)
	}
	// Three dots are not ell-two's business (ell-dots handles them).
	if _, fs3 := runPara("да...нет"); findByRule(fs3, "ell-two") != nil {
		t.Error("three dots wrongly flagged as ell-two")
	}
	// "?.." and "!.." are valid Russian and must not be flagged.
	if _, fs4 := runPara("Что?.. Кто?!.."); findByRule(fs4, "ell-two") != nil {
		t.Error("valid ?.. / !.. wrongly flagged as ell-two")
	}
	// Accepted: the run becomes an ellipsis.
	if got := acceptOnly("ell-two", "да..нет"); got != "да…нет" {
		t.Errorf("accepted ell-two = %q, want да…нет", got)
	}
}

func TestNbspUnits(t *testing.T) {
	got, _ := runPara("вес 5 кг ровно")
	if !strings.Contains(got, "5"+nbsp+"кг") {
		t.Errorf("unit not glued: %q", got)
	}
	got2, _ := runPara("в 2024 г. вышла")
	if !strings.Contains(got2, "2024"+nbsp+"г") {
		t.Errorf("year not glued: %q", got2)
	}
	// A spelled-out word is not a unit and must keep an ordinary space.
	got3, _ := runPara("дал 5 яблок")
	if strings.Contains(got3, "5"+nbsp) {
		t.Errorf("non-unit wrongly glued: %q", got3)
	}
}

func TestDotSpace(t *testing.T) {
	// Default: detect-only — suggestion present, text unchanged.
	got, fs := runPara("конец.Начало")
	if got != "конец.Начало" {
		t.Errorf("review-only rule mutated text: %q", got)
	}
	f := findByRule(fs, "dot-space")
	if f == nil || !strings.Contains(f.After, ". ") {
		t.Fatalf("dot-space finding = %+v", f)
	}
	// Initials (uppercase before the dot) are not flagged.
	if _, fs2 := runPara("инициалы А.С. рядом"); findByRule(fs2, "dot-space") != nil {
		t.Error("initials wrongly flagged")
	}
	// Known abbreviation before a capital is not flagged.
	if _, fs3 := runPara("текст см.Рис здесь"); findByRule(fs3, "dot-space") != nil {
		t.Error("abbreviation wrongly flagged")
	}
	// Accepted: a space is inserted.
	if got := acceptOnly("dot-space", "конец.Начало"); got != "конец. Начало" {
		t.Errorf("accepted dot-space = %q, want 'конец. Начало'", got)
	}
}
