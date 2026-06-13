package rules

import (
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
// listed (no nbsp needed there).
var unitWords = map[string]bool{
	"кг": true, "г": true, "т": true, "ц": true, "мг": true,
	"км": true, "м": true, "см": true, "мм": true,
	"л": true, "мл": true,
	"руб": true, "р": true, "коп": true,
	"ч": true, "мин": true, "сек": true, "с": true,
	"тыс": true, "млн": true, "млрд": true,
	"в": true, // 2024 г. / XX в. (needs a digit before, so low risk)
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
