package rules

import (
	"regexp"
	"strings"
	"unicode"
)

// Level-C rules (DESIGN.md §4.5): review-only. They never mutate the text — the
// engine reports their findings as suggestions and leaves the book unchanged.

// dehyph: a word split by a hyphen + whitespace (a line-break artefact). Gated
// by the embedded dictionary: if the joined form is a known Russian word it is
// auto-joined (level B); otherwise it is only suggested (level C). A hyphen with
// NO following space (что-то, из-за) is a real compound and is never matched.
var dehyphRe = regexp.MustCompile(`(\p{L}+)-[ \t\r\n]+(\p{Ll}[\p{L}]*)`)

var ruleDehyph = fnRule(
	"dehyph", "Склейка переноса слова", "ocr", ConfB, dehyphFn,
)

func dehyphFn(m meta, s string) (string, []Finding) {
	locs := dehyphRe.FindAllStringSubmatchIndex(s, -1)
	if locs == nil {
		return s, nil
	}
	var b strings.Builder
	var fs []Finding
	last := 0
	for _, loc := range locs {
		ms, me := loc[0], loc[1]
		left := s[loc[2]:loc[3]]
		right := s[loc[4]:loc[5]]
		match := s[ms:me]
		joined := left + right
		b.WriteString(s[last:ms])
		if dictHas(joined) {
			// Known word → safe to auto-join (B).
			b.WriteString(joined)
			fs = append(fs, Finding{RuleID: m.id, Category: m.category, Level: ConfB,
				Before: match, After: joined, Context: contextAround(s, ms, me)})
		} else {
			// Unknown → leave the text, only suggest (C).
			b.WriteString(match)
			fs = append(fs, Finding{RuleID: m.id, Category: m.category, Level: ConfC,
				Before: match, After: joined, Context: contextAround(s, ms, me)})
		}
		last = me
	}
	b.WriteString(s[last:])
	return b.String(), fs
}

// ell-two: a run of exactly two dots ("да..нет") — often a mangled ellipsis, but
// also a typo, so it is review-only (C). Three-plus dots are handled by ell-dots
// / ell-spaced. When accepted, the run becomes "…".
var ruleEllTwo = fnRule(
	"ell-two", "Две точки → многоточие?", "typography", ConfC, ellTwoFn,
)

func ellTwoFn(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	var fs []Finding
	for i := 0; i < len(runes); {
		if runes[i] != '.' {
			out = append(out, runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) && runes[j] == '.' {
			j++
		}
		// "?.." and "!.." are valid Russian (the mark plus a two-dot ellipsis), so
		// a two-dot run right after ? or ! is correct and must not be flagged.
		prevPunct := i > 0 && (runes[i-1] == '?' || runes[i-1] == '!')
		if j-i == 2 && !prevPunct {
			fs = append(fs, m.find("..", "…", runeContext(runes, i, j)))
			out = append(out, '…')
		} else {
			out = append(out, runes[i:j]...)
		}
		i = j
	}
	if len(fs) == 0 {
		return s, nil
	}
	return string(out), fs
}

// dotSpaceAbbrev are lowercase abbreviations that legitimately precede a capital
// without a space (см.Рис, г.Москва); they must not be flagged by dot-space.
var dotSpaceAbbrev = map[string]bool{
	"т": true, "тт": true, "см": true, "напр": true, "рис": true, "табл": true,
	"стр": true, "гл": true, "ср": true, "им": true, "г": true, "гг": true,
	"в": true, "вв": true, "до": true, "н": true, "э": true, "п": true, "пп": true,
	"др": true, "проч": true, "ул": true, "д": true, "кв": true,
}

// dot-space: a missing space after a sentence period — a lowercase letter, a dot,
// then an uppercase letter ("конец.Начало"). Review-only (C): the period may end
// an abbreviation. Initials are excluded automatically (they have an uppercase
// letter before the dot); known abbreviations and single letters are gated out.
var ruleDotSpace = fnRule(
	"dot-space", "Пропущен пробел после точки", "punctuation", ConfC, dotSpaceFn,
)

func dotSpaceFn(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes)+4)
	var fs []Finding
	for i := 0; i < len(runes); i++ {
		out = append(out, runes[i])
		if runes[i] != '.' || i == 0 || i+1 >= len(runes) {
			continue
		}
		if !unicode.IsLower(runes[i-1]) || !unicode.IsUpper(runes[i+1]) {
			continue
		}
		// Token of letters ending right before the dot.
		j := i - 1
		for j > 0 && unicode.IsLetter(runes[j-1]) {
			j--
		}
		tok := strings.ToLower(string(runes[j:i]))
		if len([]rune(tok)) <= 1 || dotSpaceAbbrev[tok] {
			continue // initial or known abbreviation
		}
		before := string(runes[i-1]) + "." + string(runes[i+1])
		after := string(runes[i-1]) + ". " + string(runes[i+1])
		fs = append(fs, m.find(before, after, runeContext(runes, i-1, i+2)))
		out = append(out, ' ') // applied only if the user accepts the rule
	}
	if len(fs) == 0 {
		return s, nil
	}
	return string(out), fs
}

// yo-restore: suggest restoring «ё» for unambiguous dictionary words.
var ruleYoRestore = fnRule(
	"yo-restore", "Восстановление «ё»", "spelling", ConfC, yoRestoreFn,
)

func yoRestoreFn(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	var fs []Finding
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) {
			out = append(out, runes[i])
			i++
			continue
		}
		j := i
		for j < len(runes) && unicode.IsLetter(runes[j]) {
			j++
		}
		word := string(runes[i:j])
		if repl, ok := lookupYo(word); ok {
			fs = append(fs, m.find(word, repl, runeContext(runes, i, j)))
			out = append(out, []rune(repl)...)
		} else {
			out = append(out, runes[i:j]...)
		}
		i = j
	}
	return string(out), fs
}

// mixed-alpha: a word mixing Cyrillic and Latin look-alikes (OCR artefact).
var ruleMixedAlpha = fnRule(
	"mixed-alpha", "Смешанный алфавит (лат/кир)", "ocr", ConfC, mixedAlphaFn,
)

func mixedAlphaFn(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	var fs []Finding
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) {
			out = append(out, runes[i])
			i++
			continue
		}
		j := i
		cyr, lat := 0, 0
		for j < len(runes) && unicode.IsLetter(runes[j]) {
			switch {
			case isCyrillic(runes[j]):
				cyr++
			case isLatin(runes[j]):
				lat++
			}
			j++
		}
		word := runes[i:j]
		if cyr > 0 && lat > 0 {
			repl := normalizeAlphabet(word, cyr >= lat)
			if repl != string(word) {
				fs = append(fs, m.find(string(word), repl, runeContext(runes, i, j)))
			}
		}
		out = append(out, word...)
		i = j
	}
	return string(out), fs
}

// normalizeAlphabet converts the minority script's look-alikes to the majority
// script. toCyr=true means the word is mostly Cyrillic.
func normalizeAlphabet(word []rune, toCyr bool) string {
	b := make([]rune, len(word))
	for i, r := range word {
		if toCyr {
			if c, ok := latinToCyr[r]; ok {
				b[i] = c
				continue
			}
		} else {
			if c, ok := cyrToLat[r]; ok {
				b[i] = c
				continue
			}
		}
		b[i] = r
	}
	return string(b)
}

// ocr-digit: a digit standing in for a look-alike letter inside a word.
var ruleOCRDigit = fnRule(
	"ocr-digit", "Цифра вместо буквы в слове", "ocr", ConfC, ocrDigitFn,
)

func ocrDigitFn(m meta, s string) (string, []Finding) {
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	var fs []Finding
	for i := 0; i < len(runes); {
		if !unicode.IsLetter(runes[i]) && !unicode.IsDigit(runes[i]) {
			out = append(out, runes[i])
			i++
			continue
		}
		j := i
		letters, digits, mappable := 0, 0, 0
		for j < len(runes) && (unicode.IsLetter(runes[j]) || unicode.IsDigit(runes[j])) {
			switch {
			case isCyrillic(runes[j]):
				letters++
			case unicode.IsDigit(runes[j]):
				digits++
				if _, ok := digitToCyr[runes[j]]; ok {
					mappable++
				}
			}
			j++
		}
		word := runes[i:j]
		// A digit look-alike embedded in a mostly-Cyrillic word.
		if letters >= 2 && digits >= 1 && mappable >= 1 && letters >= digits {
			repl := make([]rune, len(word))
			for k, r := range word {
				if c, ok := digitToCyr[r]; ok {
					repl[k] = c
				} else {
					repl[k] = r
				}
			}
			fs = append(fs, m.find(string(word), string(repl), runeContext(runes, i, j)))
		}
		out = append(out, word...)
		i = j
	}
	return string(out), fs
}

// --- look-alike tables -----------------------------------------------------

func isCyrillic(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }
func isLatin(r rune) bool    { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }

var latinToCyr = map[rune]rune{
	'a': 'а', 'c': 'с', 'e': 'е', 'o': 'о', 'p': 'р', 'x': 'х', 'y': 'у', 'k': 'к', 'i': 'і',
	'A': 'А', 'B': 'В', 'C': 'С', 'E': 'Е', 'H': 'Н', 'K': 'К', 'M': 'М',
	'O': 'О', 'P': 'Р', 'T': 'Т', 'X': 'Х', 'Y': 'У',
}

var cyrToLat = reverseRuneMap(latinToCyr)

func reverseRuneMap(m map[rune]rune) map[rune]rune {
	out := make(map[rune]rune, len(m))
	for k, v := range m {
		if _, exists := out[v]; !exists {
			out[v] = k
		}
	}
	return out
}

// digitToCyr maps digits to the Cyrillic letters they are commonly mis-OCR'd for.
var digitToCyr = map[rune]rune{
	'0': 'о', '3': 'з', '6': 'б',
}
