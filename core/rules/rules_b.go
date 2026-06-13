package rules

import (
	"regexp"
	"strings"
	"unicode"

	"litagent/core/model"
)

// Level-B rules (DESIGN.md §4.5): heuristic, auto-applied. The nbsp rules are
// non-destructive (an invisible space), so false positives are low-risk.

const (
	nbspRune = rune(0x00A0) // NO-BREAK SPACE
	nbsp     = string(nbspRune)
)

// quotes-apos: word-internal apostrophe → typographic ’ (d'Artagnan, O'Henry).
var ruleApostrophe = reRule(
	"quotes-apos", "Апостроф ’", "typography", ConfB,
	`(\p{L})['\x{02BC}](\p{L})`, func(g []string) string { return g[1] + "’" + g[2] },
)

// nbsp-init: non-breaking space between initials (А. С. Пушкин).
var ruleNbspInit = reRule(
	"nbsp-init", "Неразрывный пробел в инициалах", "typography", ConfB,
	`(\p{Lu}\.)[ \x{00A0}]+(\p{Lu}\.)`, func(g []string) string { return g[1] + nbsp + g[2] },
)

// nbsp-abbr: non-breaking space inside fixed two-letter abbreviations so they do
// not wrap across a line (т. д., т. е., н. э. …). A whitelist of exact token pairs
// is used — a generic "letter. Letter" pattern would wrongly glue sentence
// boundaries ("он. Та"). The leading "и"/"до" of "и т. д." / "до н. э." already
// gets a nbsp from nbsp-prep (both are in shortWords), so only the core is joined.
var abbrRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(т)\.[ \x{00A0}]+(д|п|е|к|н|о)\.`),
	regexp.MustCompile(`(?i)(н)\.[ \x{00A0}]+(э)\.`),
	regexp.MustCompile(`(?i)(и)\.[ \x{00A0}]+(о)\.`),
}

func abbrRepl(g []string) string { return g[1] + "." + nbsp + g[2] + "." }

var ruleNbspAbbr = fnRule(
	"nbsp-abbr", "Неразрывный пробел в сокращениях", "typography", ConfB, nbspAbbr,
)

func nbspAbbr(m meta, s string) (string, []Finding) {
	var fs []Finding
	for _, re := range abbrRes {
		var rf []Finding
		s, rf = applyRegex(m, re, abbrRepl, s)
		fs = append(fs, rf...)
	}
	return s, fs
}

// nbsp-num: non-breaking space around numbers and signs (5 %, № 7).
var ruleNbspNum = reRule(
	"nbsp-num", "Неразрывный пробел с числами", "typography", ConfB,
	`(\d)[ \x{00A0}]+([%\x{2030}\x{00B0}\x{20BD}])|(\x{2116})[ \x{00A0}]+(\d)`,
	func(g []string) string {
		if g[1] != "" {
			return g[1] + nbsp + g[2]
		}
		return g[3] + nbsp + g[4]
	},
)

// nbsp-dash: non-breaking space before an em dash so it never starts a line.
var ruleNbspDash = reRule(
	"nbsp-dash", "Неразрывный пробел перед тире", "typography", ConfB,
	`([^\s\x{00A0}])[ ]+(\x{2014})`, func(g []string) string { return g[1] + nbsp + g[2] },
)

// dash-endash: a spaced en dash (Russian uses the em dash in sentences; the en
// dash is reserved for numeric ranges, which are unspaced and untouched).
var ruleDashEndash = reRule(
	"dash-endash", "Короткое тире – → длинное —", "typography", ConfB,
	` \x{2013} `, constRepl(" — "),
)

// shortWords are prepositions/conjunctions that should not hang at a line end.
var shortWords = map[string]bool{
	"в": true, "во": true, "на": true, "по": true, "за": true, "до": true,
	"из": true, "изо": true, "от": true, "ото": true, "об": true, "обо": true,
	"о": true, "у": true, "к": true, "ко": true, "с": true, "со": true,
	"и": true, "а": true, "но": true, "да": true, "или": true, "не": true,
	"ни": true, "же": true, "бы": true, "ли": true, "то": true, "что": true,
	"как": true, "для": true, "при": true, "под": true, "над": true, "без": true,
}

// nbsp-prep: replace the space after a short function word with nbsp. Done as a
// scan (not regex) so consecutive short words ("и к дому") are all handled and
// Cyrillic word boundaries work (Go's \b is ASCII-only).
var ruleNbspPrep = fnRule(
	"nbsp-prep", "Неразрывный пробел после предлогов", "typography", ConfB, nbspPrep,
)

func nbspPrep(m meta, s string) (string, []Finding) {
	if !strings.ContainsRune(s, ' ') {
		return s, nil
	}
	runes := []rune(s)
	var fs []Finding
	for i := 0; i < len(runes); i++ {
		if runes[i] != ' ' {
			continue
		}
		// Word immediately before the space.
		j := i
		for j > 0 && isWordRune(runes[j-1]) {
			j--
		}
		if j == i {
			continue // no word directly before the space
		}
		word := strings.ToLower(string(runes[j:i]))
		if !shortWords[word] {
			continue
		}
		// Something word-like must follow the space.
		if i+1 >= len(runes) || !isWordRune(runes[i+1]) {
			continue
		}
		fs = append(fs, m.find(" ", nbsp, runeContext(runes, j, i+1)))
		runes[i] = nbspRune
	}
	if len(fs) == 0 {
		return s, nil
	}
	return string(runes), fs
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// unitWords are abbreviated units/quantities that should stay glued to their
// preceding number (12 кг, 5 км, 2024 г., 10 руб). Spelled-out words are not
// listed (no nbsp needed there). Single-letter entries (в, с, г, т, м, л, р, ч,
// ц) collide with prepositions/short words ("66 в Форт-Лодердейле"), so they are
// only accepted with a trailing dot (see nbspUnits) — real abbreviations carry
// one: "2024 г.", "XX в.".
var unitWords = map[string]bool{
	"кг": true, "г": true, "т": true, "ц": true, "мг": true,
	"км": true, "м": true, "см": true, "мм": true,
	"л": true, "мл": true,
	"руб": true, "р": true, "коп": true,
	"ч": true, "мин": true, "сек": true, "с": true,
	"тыс": true, "млн": true, "млрд": true,
	"в": true,
}

// nbsp-units: glue an abbreviated unit to the number before it with a nbsp. Done
// as a scan (like nbspPrep) for correct Cyrillic word boundaries.
var ruleNbspUnits = fnRule(
	"nbsp-units", "Неразрывный пробел с единицами", "typography", ConfB, nbspUnits,
)

func nbspUnits(m meta, s string) (string, []Finding) {
	if !strings.ContainsRune(s, ' ') {
		return s, nil
	}
	runes := []rune(s)
	var fs []Finding
	for i := 0; i < len(runes); i++ {
		if runes[i] != ' ' {
			continue
		}
		// A digit must end the run immediately before the space.
		if i == 0 || !unicode.IsDigit(runes[i-1]) {
			continue
		}
		// Read the letter token after the space.
		j := i + 1
		k := j
		for k < len(runes) && unicode.IsLetter(runes[k]) {
			k++
		}
		if k == j {
			continue // nothing word-like after the space
		}
		// Whole token only: must not be followed by another letter/digit.
		if k < len(runes) && isWordRune(runes[k]) {
			continue
		}
		if !unitWords[strings.ToLower(string(runes[j:k]))] {
			continue
		}
		// A single-letter unit must be followed by a dot ("2024 г.", "XX в.");
		// otherwise it is almost certainly a preposition/short word, not a unit
		// ("66 в Форт-Лодердейле"), and must not be glued to the number.
		if k-j == 1 && (k >= len(runes) || runes[k] != '.') {
			continue
		}
		fs = append(fs, m.find(" ", nbsp, runeContext(runes, i-1, k)))
		runes[i] = nbspRune
	}
	if len(fs) == 0 {
		return s, nil
	}
	return string(runes), fs
}

// dialog-dash: a hyphen/en-dash starting a paragraph (dialogue) → em dash.
var ruleDialogDash = containerRule{
	id: "dialog-dash", name: "Тире в начале реплики", category: "typography", level: ConfB,
	apply: func(inlines []model.Inline) []Finding {
		if len(inlines) == 0 {
			return nil
		}
		t, ok := inlines[0].(*model.Text)
		if !ok {
			return nil
		}
		runes := []rune(t.Value)
		k := 0
		for k < len(runes) && isSpaceRune(runes[k]) {
			k++
		}
		if k+1 >= len(runes) {
			return nil
		}
		if !isDialogueDash(runes[k]) || runes[k] == '—' {
			return nil
		}
		if !isSpaceRune(runes[k+1]) {
			return nil
		}
		before := string(runes[k])
		m := meta{id: "dialog-dash", category: "typography", level: ConfB}
		f := m.find(before, "—", runeContext(runes, k, k+1))
		runes[k] = '—'
		t.Value = string(runes)
		return []Finding{f}
	},
}

func isDialogueDash(r rune) bool {
	switch r {
	case '-', '–', '—', '−':
		return true
	}
	return false
}

func isSpaceRune(r rune) bool { return r == ' ' || r == nbspRune || r == '\t' }

// dialog-dash-glue: a dialogue dash glued to the first word of a paragraph
// (—Текст / -Текст) → em dash + space. Complements dialog-dash, which only fires
// when a space already follows; here the next rune must be a letter (so "—!" or
// "—2" are left untouched). The two rules are mutually exclusive on that rune.
var ruleDialogDashGlue = containerRule{
	id: "dialog-dash-glue", name: "Пробел после тире реплики", category: "typography", level: ConfB,
	apply: func(inlines []model.Inline) []Finding {
		if len(inlines) == 0 {
			return nil
		}
		t, ok := inlines[0].(*model.Text)
		if !ok {
			return nil
		}
		runes := []rune(t.Value)
		k := 0
		for k < len(runes) && isSpaceRune(runes[k]) {
			k++
		}
		if k+1 >= len(runes) || !isDialogueDash(runes[k]) || !unicode.IsLetter(runes[k+1]) {
			return nil
		}
		m := meta{id: "dialog-dash-glue", category: "typography", level: ConfB}
		f := m.find(string(runes[k]), "— ", runeContext(runes, k, k+1))
		out := append([]rune{}, runes[:k]...)
		out = append(out, '—', ' ')
		out = append(out, runes[k+1:]...)
		t.Value = string(out)
		return []Finding{f}
	},
}

// enDash (U+2013) joins numeric ranges; the em dash — is reserved for sentences.
const enDash = '–'

// dash-range: a numeric range written with a hyphen → en dash, no spaces
// (1941-1945 → 1941–1945). Guards: the token must be digits joined by exactly one
// hyphen, each side 1–4 digits. Phone numbers / ISO dates (two+ hyphens) and
// letter-digit compounds (Т-34, 5-летний) are excluded because the token is
// digit-only and a side touching a letter is never a pure number token.
var ruleDashRange = fnRule(
	"dash-range", "Числовой диапазон через тире", "typography", ConfB, dashRange,
)

func dashRange(m meta, s string) (string, []Finding) {
	if !strings.ContainsRune(s, '-') {
		return s, nil
	}
	runes := []rune(s)
	var fs []Finding
	for i := 0; i < len(runes); {
		if !unicode.IsDigit(runes[i]) {
			i++
			continue
		}
		// Maximal [0-9-] token starting at this digit.
		j := i
		for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '-') {
			j++
		}
		// Token must end on a digit (a letter right after rules it out as a range).
		end := j
		for end > i && runes[end-1] == '-' {
			end--
		}
		if j >= len(runes) || !unicode.IsLetter(runes[j]) {
			convertRange(m, runes, i, end, &fs)
		}
		i = j
	}
	if len(fs) == 0 {
		return s, nil
	}
	return string(runes), fs
}

// convertRange rewrites runes[a:b] in place if it is a single-hyphen range with
// 1–4 digits on each side, recording a finding.
func convertRange(m meta, runes []rune, a, b int, fs *[]Finding) {
	hyphen, count := -1, 0
	for k := a; k < b; k++ {
		if runes[k] == '-' {
			hyphen = k
			count++
		}
	}
	if count != 1 {
		return
	}
	left, right := hyphen-a, b-hyphen-1
	if left < 1 || left > 4 || right < 1 || right > 4 {
		return
	}
	before := string(runes[a:b])
	runes[hyphen] = enDash
	*fs = append(*fs, m.find(before, string(runes[a:b]), runeContext(runes, a, b)))
}
