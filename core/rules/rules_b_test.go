package rules

import (
	"strings"
	"testing"

	"litagent/core/model"
)

func TestQuotesBasic(t *testing.T) {
	got, _ := runPara(`Он сказал "привет" ей.`)
	want := `Он сказал «привет» ей.`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestQuotesNested(t *testing.T) {
	got, _ := runPara(`сказал "да "точно" верно"`)
	want := `сказал «да „точно“ верно»`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestQuotesAcrossMarkup(t *testing.T) {
	// "слово bold конец" where the opening quote is in run 1 and the closing in
	// run 3 — shared state across markup must still pair them.
	para := &model.Paragraph{Inlines: []model.Inline{
		&model.Text{Value: `он "`},
		&model.Styled{Kind: model.StyleStrong, Children: []model.Inline{&model.Text{Value: "очень"}}},
		&model.Text{Value: `" сказал`},
	}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{Content: []model.Block{para}}}}}}
	DefaultEngine().Run(book)

	if v := para.Inlines[0].(*model.Text).Value; v != "он «" {
		t.Errorf("run0 = %q, want %q", v, "он «")
	}
	if v := para.Inlines[2].(*model.Text).Value; v != "» сказал" {
		t.Errorf("run2 = %q, want %q", v, "» сказал")
	}
}

func TestDialogDash(t *testing.T) {
	for _, in := range []string{"- Привет", "– Привет", "— Привет"} {
		got, _ := runPara(in)
		if got != "— Привет" {
			t.Errorf("in %q: got %q, want %q", in, got, "— Привет")
		}
	}
	// A hyphen in the middle of a line is not a dialogue dash.
	got, _ := runPara("что-то")
	if got != "что-то" {
		t.Errorf("mid-line hyphen changed: %q", got)
	}
}

func TestNbspPrep(t *testing.T) {
	got, _ := runPara("в лесу родилась")
	want := "в" + nbsp + "лесу родилась"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// Consecutive short words must both get nbsp.
	got2, _ := runPara("и к дому")
	want2 := "и" + nbsp + "к" + nbsp + "дому"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}

func TestNbspInit(t *testing.T) {
	got, _ := runPara("А. С. Пушкин")
	want := "А." + nbsp + "С. Пушкин"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApostrophe(t *testing.T) {
	got, _ := runPara("д'Артаньян")
	if got != "д’Артаньян" {
		t.Errorf("got %q", got)
	}
}

func TestNbspBeforeDash(t *testing.T) {
	got, _ := runPara("он — я")
	want := "он" + nbsp + "— я"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDashEndash(t *testing.T) {
	got, _ := runPara("он – я")
	want := "он" + nbsp + "— я"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// A numeric range (unspaced en dash) must stay an en dash.
	got2, _ := runPara("1990–2000 годы")
	if got2 != "1990–2000 годы" {
		t.Errorf("range changed: %q", got2)
	}
}

func TestNbspAbbr(t *testing.T) {
	// "т. д." core gets a nbsp; the leading "и" gets one from nbsp-prep.
	got, _ := runPara("и т. д.")
	want := "и" + nbsp + "т." + nbsp + "д."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	got2, _ := runPara("то есть т. е. вот")
	if !strings.Contains(got2, "т."+nbsp+"е.") {
		t.Errorf("abbr not glued: %q", got2)
	}
	// A sentence boundary must NOT be glued.
	got3, _ := runPara("он. Та дверь")
	if strings.Contains(got3, nbsp) {
		t.Errorf("sentence boundary glued: %q", got3)
	}
}

func TestDialogDashGlue(t *testing.T) {
	for _, in := range []string{"—Привет", "-Привет", "–Привет"} {
		got, _ := runPara(in)
		if got != "— Привет" {
			t.Errorf("in %q: got %q, want %q", in, got, "— Привет")
		}
	}
	// A dash followed by punctuation/digit is not a glued reply.
	for _, in := range []string{"—!", "—2"} {
		got, _ := runPara(in)
		if got != in {
			t.Errorf("in %q changed to %q", in, got)
		}
	}
}

func TestDashRange(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1941-1945 годы", "1941–1945 годы"},
		{"страницы 5-7", "страницы 5–7"},
		{"Т-34", "Т-34"},          // letter-digit compound
		{"5-летний", "5-летний"},  // digit-letter compound
		{"тел 12-34-56", "тел 12-34-56"}, // phone: two hyphens
		{"дата 2020-01-02", "дата 2020-01-02"}, // ISO date: two hyphens
	}
	for _, c := range cases {
		got, _ := runPara(c.in)
		if got != c.want {
			t.Errorf("in %q: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRuleToggleDisablesQuotes(t *testing.T) {
	// Engine with every rule except quotes: straight quotes stay straight.
	eng := NewEngineFor(func(id string) bool { return id != "quotes" })
	para := &model.Paragraph{Inlines: []model.Inline{&model.Text{Value: `"привет"`}}}
	book := &model.Book{Bodies: []*model.Body{{Sections: []*model.Section{{Content: []model.Block{para}}}}}}
	eng.Run(book)
	if v := para.Inlines[0].(*model.Text).Value; !strings.Contains(v, `"`) {
		t.Errorf("quotes were converted despite being disabled: %q", v)
	}
}

func TestBFindingsLevels(t *testing.T) {
	_, fs := runPara(`- "в лесу"`)
	levels := map[string]Confidence{}
	for _, f := range fs {
		levels[f.RuleID] = f.Level
	}
	for _, id := range []string{"quotes", "dialog-dash", "nbsp-prep"} {
		if lvl, ok := levels[id]; !ok {
			t.Errorf("expected finding from %s", id)
		} else if lvl != ConfB {
			t.Errorf("%s level = %v, want B", id, lvl)
		}
	}
}
