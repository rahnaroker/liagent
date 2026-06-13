package rules

import (
	"strings"
	"testing"

	"litagent/core/model"
)

// aEngine builds an engine with only level-A rules, isolating A tests from B.
func aEngine() *Engine {
	aset := map[string]bool{}
	for _, m := range Catalog() {
		if m.Level == ConfA {
			aset[m.ID] = true
		}
	}
	return NewEngineFor(func(id string) bool { return aset[id] })
}

// applyA runs only the level-A text rules over a string.
func applyA(s string) string {
	out, _ := aEngine().applyTextRules(s)
	return out
}

// runPara runs the full pipeline over a single-paragraph book and returns the
// resulting text plus findings.
func runPara(text string) (string, []Finding) {
	para := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: text}}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{Content: []model.Block{para}}}}}}
	fs := DefaultEngine().Run(book)
	return para.Inlines[0].(*model.Text).Value, fs
}

func TestLevelARules(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"double space", "Это  двойной   пробел", "Это двойной пробел"},
		{"newline to space", "строка\nещё", "строка ещё"},
		{"space before comma", "слово , и", "слово, и"},
		{"space before period", "конец .", "конец."},
		{"no space after comma", "да,нет", "да, нет"},
		{"no space after colon", "так:вот", "так: вот"},
		{"decimal comma untouched", "цена 1,5 кг", "цена 1,5 кг"},
		{"triple hyphen", "что---то", "что—то"},
		{"double hyphen", "да--нет", "да—нет"},
		{"ellipsis", "вот...", "вот…"},
		{"spaced hyphen to dash", "он - я", "он — я"},
		{"copyright", "(c) 2020", "© 2020"},
		{"registered", "Бренд(r)", "Бренд®"},
		{"trademark", "Марка(tm)", "Марка™"},
		{"dup comma", "текст,, ещё", "текст, ещё"},
		{"space inside parens", "( текст )", "(текст)"},
		{"space inside brackets", "[ 1 ]", "[1]"},
		{"space after opening quote", "« текст", "«текст"},
		{"nbsp preserved", "а б", "а б"},
		{"combo", "Текст  ,...и  вот - так", "Текст,…и вот — так"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := applyA(c.in); got != c.want {
				t.Errorf("in %q: got %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestFindingsRecorded(t *testing.T) {
	_, findings := DefaultEngine().applyTextRules("Это  двойной   пробел...")
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	var sawWS, sawEll bool
	for _, f := range findings {
		if f.RuleID == "" || f.Before == "" || f.After == f.Before {
			t.Errorf("malformed finding: %+v", f)
		}
		switch f.RuleID {
		case "sp-ws":
			sawWS = true
		case "ell-dots":
			sawEll = true
		}
	}
	if !sawWS || !sawEll {
		t.Errorf("missing expected rule findings: ws=%v ell=%v", sawWS, sawEll)
	}
}

func TestRunPreservesMarkupAndTrims(t *testing.T) {
	// "  Текст  " + bold "важно" + " ,конец  " -> trimmed ends, fixes inside,
	// markup (strong) intact.
	para := &model.Paragraph{Inlines: []model.Inline{
		&model.Text{Value: "  Текст  "},
		&model.Styled{Kind: model.StyleStrong, Children: []model.Inline{&model.Text{Value: "важно"}}},
		&model.Text{Value: " ,конец  "},
	}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{
		ID: "s1", Content: []model.Block{para},
	}}}}}

	findings := DefaultEngine().Run(book)
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}

	// First text run: leading spaces trimmed, internal double space collapsed.
	if got := para.Inlines[0].(*model.Text).Value; got != "Текст " {
		t.Errorf("first run = %q, want %q", got, "Текст ")
	}
	// Bold markup preserved.
	st, ok := para.Inlines[1].(*model.Styled)
	if !ok || st.Kind != model.StyleStrong || st.Children[0].(*model.Text).Value != "важно" {
		t.Errorf("markup not preserved: %+v", para.Inlines[1])
	}
	// Last run: leading space-before-comma removed, space inserted after comma,
	// trailing spaces trimmed -> renders "важно, конец".
	if got := para.Inlines[2].(*model.Text).Value; got != ", конец" {
		t.Errorf("last run = %q, want %q", got, ", конец")
	}
	// Findings carry the section id.
	for _, f := range findings {
		if f.Section != "s1" {
			t.Errorf("finding section = %q, want s1", f.Section)
		}
	}
}

func TestContextMarked(t *testing.T) {
	_, findings := DefaultEngine().applyTextRules("длинный текст да,нет ещё длинный текст")
	for _, f := range findings {
		if f.RuleID == "sp-after-punct" {
			if !strings.Contains(f.Context, "⟦") || !strings.Contains(f.Context, "⟧") {
				t.Errorf("context not marked: %q", f.Context)
			}
			return
		}
	}
	t.Fatal("sp-after-punct finding not found")
}
