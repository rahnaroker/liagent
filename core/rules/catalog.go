package rules

// pipeline is the ordered rule set (DESIGN.md §4.2): clean invisibles, collapse
// whitespace, convert quotes (stateful), normalise dashes/ellipsis, fix spacing,
// then non-breaking spaces. A and B levels are interleaved in execution order.
var pipeline = []step{
	tstep(ruleCtrlClean),
	tstep(ruleSpWs),
	tstep(ruleDehyph), // dictionary-gated: known words auto-join (B), rest suggested (C)
	cstep(ruleQuotes),
	tstep(ruleDashMulti),
	tstep(ruleEllDots),
	tstep(ruleEllSpaced),
	cstep(ruleDialogDash),
	tstep(ruleSpBeforePunct),
	tstep(ruleSpAfterPunct),
	tstep(ruleDashHyphen),
	tstep(ruleDashEndash),
	tstep(ruleApostrophe),
	tstep(ruleSymCopy),
	tstep(ruleSymReg),
	tstep(ruleSymTm),
	tstep(ruleDupComma),
	tstep(ruleNbspInit),
	tstep(ruleNbspPrep),
	tstep(ruleNbspNum),
	tstep(ruleNbspUnits),
	tstep(ruleNbspDash),
	// Level C — review-only suggestions (never auto-applied).
	// tstep(ruleYoRestore), // ОТКЛЮЧЕНО по запросу: восстановление «ё». Вернуть = раскомментировать.
	tstep(ruleEllTwo),
	tstep(ruleDotSpace),
	tstep(ruleMixedAlpha),
	tstep(ruleOCRDigit),
}

// defaultApply marks every non-C rule as applied; C rules default to detect-only.
var defaultApply = func() map[string]bool {
	m := make(map[string]bool, len(pipeline))
	for _, s := range pipeline {
		if s.level != ConfC {
			m[s.id] = true
		}
	}
	return m
}()

// DefaultEngine applies all level-A/B rules; level-C rules are detect-only.
func DefaultEngine() *Engine {
	return &Engine{apply: func(id string) bool { return defaultApply[id] }}
}

// NewEngineFor builds an engine that applies exactly the rules for which apply
// returns true. Level-C rules not selected still run in detect-only mode.
func NewEngineFor(apply func(id string) bool) *Engine {
	return &Engine{apply: apply}
}

// DefaultApplyIDs returns the ids applied by default (every non-C rule).
func DefaultApplyIDs() []string {
	out := make([]string, 0, len(pipeline))
	for _, s := range pipeline {
		if s.level != ConfC {
			out = append(out, s.id)
		}
	}
	return out
}

// RuleMeta describes a rule for the UI (no internals).
type RuleMeta struct {
	ID       string
	Name     string
	Category string
	Level    Confidence
}

// Catalog returns metadata for every rule (pipeline + structural), in order.
func Catalog() []RuleMeta {
	out := make([]RuleMeta, 0, len(pipeline)+len(blockRules))
	for _, s := range pipeline {
		out = append(out, RuleMeta{ID: s.id, Name: s.name, Category: s.category, Level: s.level})
	}
	for _, b := range blockRules {
		out = append(out, RuleMeta{ID: b.id, Name: b.name, Category: b.category, Level: b.level})
	}
	return out
}

// AllRuleIDs returns the ids of every rule.
func AllRuleIDs() []string {
	out := make([]string, 0, len(pipeline)+len(blockRules))
	for _, s := range pipeline {
		out = append(out, s.id)
	}
	for _, b := range blockRules {
		out = append(out, b.id)
	}
	return out
}
