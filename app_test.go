package main

import (
	"strings"
	"testing"

	"litagent/core/rules"
)

const sampleFB2 = "core/fb2/testdata/sample.fb2"

func TestAppRunLoadsAndAnalyses(t *testing.T) {
	a := NewApp()
	a.path = sampleFB2

	res, err := a.run(rules.AllRuleIDs())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Meta.Title != "Пример книги" {
		t.Errorf("title = %q", res.Meta.Title)
	}
	if res.Meta.Author != "Лев Толстой" {
		t.Errorf("author = %q", res.Meta.Author)
	}
	if res.Meta.Sections != 2 {
		t.Errorf("sections = %d, want 2", res.Meta.Sections)
	}
	if !res.Meta.HasCover {
		t.Error("expected cover")
	}
	if len(res.Rules) != len(rules.AllRuleIDs()) {
		t.Errorf("rules = %d, want %d", len(res.Rules), len(rules.AllRuleIDs()))
	}
	for _, r := range res.Rules {
		if !r.Enabled {
			t.Errorf("rule %s should be enabled when all ids passed", r.ID)
		}
	}
	// The corrected book must be retained for export.
	if a.book == nil {
		t.Error("book not retained after run")
	}
}

func TestAppRunRespectsDisabledRules(t *testing.T) {
	a := NewApp()
	a.path = sampleFB2

	// Enable nothing: every rule disabled, zero corrections.
	res, err := a.run(nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Total != 0 {
		t.Errorf("total with no rules = %d, want 0", res.Total)
	}
	for _, r := range res.Rules {
		if r.Enabled {
			t.Errorf("rule %s should be disabled", r.ID)
		}
	}
}

func TestPreviewHTML(t *testing.T) {
	a := NewApp()
	a.path = sampleFB2
	if _, err := a.run(rules.DefaultApplyIDs()); err != nil {
		t.Fatalf("run: %v", err)
	}
	html := previewHTML(a.book)
	if !strings.Contains(html, "Глава первая") {
		t.Errorf("preview missing chapter heading:\n%s", html)
	}
	if !strings.Contains(html, "<p>") || !strings.Contains(html, "первый") {
		t.Errorf("preview missing paragraph text:\n%s", html)
	}
}

func TestAppRunMissingFile(t *testing.T) {
	a := NewApp()
	a.path = "does-not-exist.fb2"
	if _, err := a.run(rules.AllRuleIDs()); err == nil {
		t.Error("expected error for missing file")
	}
}
