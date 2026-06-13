// Package rules implements the correction engine. Rules carry a confidence
// level (A/B auto-applied, C review-only) and come in three shapes:
//   - text rules: a transform over a single text run (regex or custom function);
//   - container rules: a position-aware/stateful pass over a whole inline
//     container (e.g. stateful quote conversion, dialogue dash at line start).
//
// See DESIGN.md §4.
package rules

import (
	"regexp"

	"litagent/core/model"
)

// Confidence is a rule's safety tier.
type Confidence int

const (
	// ConfA: deterministic, safe to auto-apply.
	ConfA Confidence = iota
	// ConfB: heuristic, auto-applied (irreversible ones are guarded).
	ConfB
	// ConfC: review-only; never applied silently.
	ConfC
)

func (c Confidence) String() string {
	switch c {
	case ConfA:
		return "A"
	case ConfB:
		return "B"
	default:
		return "C"
	}
}

// Finding is one applied (or proposed) edit, surfaced in the GUI for review.
type Finding struct {
	RuleID   string
	Category string
	Level    Confidence
	Section  string // section id for navigation
	Context  string // snippet with the change marked
	Before   string
	After    string
}

// meta carries a rule's identity so transforms can build findings.
type meta struct {
	id, category string
	level        Confidence
}

func (m meta) find(before, after, context string) Finding {
	return Finding{RuleID: m.id, Category: m.category, Level: m.level, Before: before, After: after, Context: context}
}

// textRule transforms a single text run in place.
type textRule struct {
	id, name, category string
	level              Confidence
	transform          func(s string) (string, []Finding)
}

// containerRule operates on a whole inline container (sees node order/position).
type containerRule struct {
	id, name, category string
	level              Confidence
	apply              func(inlines []model.Inline) []Finding
}

// step is one pipeline stage; exactly one of text/cont is set.
type step struct {
	id, name, category string
	level              Confidence
	text               *textRule
	cont               *containerRule
}

func tstep(t textRule) step {
	r := t
	return step{id: r.id, name: r.name, category: r.category, level: r.level, text: &r}
}

func cstep(c containerRule) step {
	r := c
	return step{id: r.id, name: r.name, category: r.category, level: r.level, cont: &r}
}

// --- rule constructors -----------------------------------------------------

// reRule builds a regex text rule whose replacement is computed from submatch
// groups (groups[0] is the whole match).
func reRule(id, name, category string, level Confidence, pattern string, repl func(groups []string) string) textRule {
	re := regexp.MustCompile(pattern)
	m := meta{id: id, category: category, level: level}
	return textRule{
		id: id, name: name, category: category, level: level,
		transform: func(s string) (string, []Finding) { return applyRegex(m, re, repl, s) },
	}
}

// fnRule builds a text rule backed by a custom function.
func fnRule(id, name, category string, level Confidence, fn func(m meta, s string) (string, []Finding)) textRule {
	m := meta{id: id, category: category, level: level}
	return textRule{
		id: id, name: name, category: category, level: level,
		transform: func(s string) (string, []Finding) { return fn(m, s) },
	}
}

func constRepl(s string) func([]string) string { return func([]string) string { return s } }

func group(n int) func([]string) string {
	return func(g []string) string {
		if n < len(g) {
			return g[n]
		}
		return ""
	}
}

// --- engine ----------------------------------------------------------------

// Engine runs the shared pipeline, deciding per rule whether to apply it.
//
// Semantics (DESIGN.md §4): a rule is "applied" (mutates text) iff apply(id) is
// true. Level-A/B rules that are not applied are skipped entirely (reverted).
// Level-C rules that are not applied still run in detect-only mode, so their
// suggestions are always surfaced for review even when not accepted.
type Engine struct {
	apply func(id string) bool
}

// Run applies the pipeline to every inline container in the book in place and
// returns the findings in document order. Markup is preserved.
func (e *Engine) Run(b *model.Book) []Finding {
	var all []Finding
	walkBook(b, func(inlines []model.Inline, secID string) {
		e.processInlines(inlines, secID, &all)
	})
	// Structural detectors run on whole paragraphs (applied only if accepted).
	runBlockRules(b, e.apply, &all)
	return all
}

func (e *Engine) processInlines(inlines []model.Inline, secID string, all *[]Finding) {
	// Trim outer whitespace first so position-aware rules (dialogue dash) see
	// the real first/last character.
	trimContainer(inlines, secID, all)

	for i := range pipeline {
		st := &pipeline[i]
		applyThis := e.apply(st.id)
		if st.level != ConfC && !applyThis {
			continue // A/B rule turned off → skip (reverted)
		}
		switch {
		case st.text != nil:
			// C runs always (applyThis=false → detect-only).
			applyTextStep(st.text, inlines, secID, all, applyThis)
		case st.cont != nil:
			if !applyThis {
				continue // container rules are all A/B and have no detect-only mode
			}
			fs := st.cont.apply(inlines)
			for j := range fs {
				fs[j].Section = secID
			}
			*all = append(*all, fs...)
		}
	}
}

// applyTextStep runs a text rule over every text run, recursing into markup. If
// apply is false (review-only rules) the proposed change is reported but the
// text is left unchanged.
func applyTextStep(tr *textRule, inlines []model.Inline, secID string, all *[]Finding, apply bool) {
	var rec func([]model.Inline)
	rec = func(xs []model.Inline) {
		for _, x := range xs {
			switch n := x.(type) {
			case *model.Text:
				ns, fs := tr.transform(n.Value)
				for j := range fs {
					fs[j].Section = secID
				}
				*all = append(*all, fs...)
				if apply {
					n.Value = ns
				}
			case *model.Styled:
				rec(n.Children)
			case *model.Link:
				rec(n.Children)
			}
		}
	}
	rec(inlines)
}

func trimContainer(inlines []model.Inline, secID string, all *[]Finding) {
	if len(inlines) == 0 {
		return
	}
	if t, ok := inlines[0].(*model.Text); ok {
		if trimmed := trimLeftWS(t.Value); trimmed != t.Value {
			*all = append(*all, trimFinding(secID, t.Value, trimmed))
			t.Value = trimmed
		}
	}
	last := len(inlines) - 1
	if t, ok := inlines[last].(*model.Text); ok {
		if trimmed := trimRightWS(t.Value); trimmed != t.Value {
			*all = append(*all, trimFinding(secID, t.Value, trimmed))
			t.Value = trimmed
		}
	}
}

// applyTextRules runs only the text steps over a single string (used in tests),
// mirroring Run's apply/detect semantics.
func (e *Engine) applyTextRules(s string) (string, []Finding) {
	var fs []Finding
	for i := range pipeline {
		st := &pipeline[i]
		if st.text == nil {
			continue
		}
		applyThis := e.apply(st.id)
		if st.level != ConfC && !applyThis {
			continue
		}
		ns, rf := st.text.transform(s)
		fs = append(fs, rf...)
		if applyThis {
			s = ns
		}
	}
	return s, fs
}
