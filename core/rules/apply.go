package rules

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// applyRegex rewrites s with a regex rule, returning the new string and a
// finding per actual change.
func applyRegex(m meta, re *regexp.Regexp, repl func([]string) string, s string) (string, []Finding) {
	locs := re.FindAllStringSubmatchIndex(s, -1)
	if locs == nil {
		return s, nil
	}
	var b strings.Builder
	var fs []Finding
	last := 0
	for _, loc := range locs {
		// loc holds [start,end] pairs: group 0 is the whole match, then submatches.
		groups := make([]string, len(loc)/2)
		for g := range groups {
			if loc[2*g] >= 0 {
				groups[g] = s[loc[2*g]:loc[2*g+1]]
			}
		}
		match := groups[0]
		replacement := repl(groups)
		b.WriteString(s[last:loc[0]])
		b.WriteString(replacement)
		last = loc[1]
		if replacement != match {
			fs = append(fs, m.find(match, replacement, contextAround(s, loc[0], loc[1])))
		}
	}
	b.WriteString(s[last:])
	return b.String(), fs
}

const trimCutset = " \t\r\n"

func trimLeftWS(s string) string  { return strings.TrimLeft(s, trimCutset) }
func trimRightWS(s string) string { return strings.TrimRight(s, trimCutset) }

func trimFinding(secID, before, after string) Finding {
	return Finding{
		RuleID:   "sp-trim",
		Category: "spacing",
		Level:    ConfA,
		Section:  secID,
		Context:  "⟦" + before + "⟧ → ⟦" + after + "⟧",
		Before:   before,
		After:    after,
	}
}

// contextAround returns up to 12 runes on each side of [start,end) (byte
// offsets) with the changed span marked, for display in the review UI.
func contextAround(s string, start, end int) string {
	return lastRunes(s[:start], 12) + "⟦" + s[start:end] + "⟧" + firstRunes(s[end:], 12)
}

// runeContext is the rune-index equivalent for function rules.
func runeContext(runes []rune, start, end int) string {
	lo := start - 12
	if lo < 0 {
		lo = 0
	}
	hi := end + 12
	if hi > len(runes) {
		hi = len(runes)
	}
	return string(runes[lo:start]) + "⟦" + string(runes[start:end]) + "⟧" + string(runes[end:hi])
}

func firstRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

func lastRunes(s string, n int) string {
	count := 0
	i := len(s)
	for i > 0 {
		_, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
		count++
		if count == n {
			return s[i:]
		}
	}
	return s
}
