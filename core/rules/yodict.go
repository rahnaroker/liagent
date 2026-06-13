package rules

import (
	"strings"
	"unicode"
)

// yoWords lists words whose spelling is unambiguously with «ё» — the «е» variant
// is never correct, so restoring «ё» is safe to suggest. Ambiguous pairs
// (все/всё, осел/осёл, небо/нёбо, узнаем/узнаём, …) are deliberately excluded.
// The list is curated and easily extended; restoration is review-only anyway.
var yoWords = []string{
	// pronouns / function words (invariant, high frequency)
	"ещё", "её", "неё", "моё", "твоё", "своё", "чьё", "чём", "причём", "нём",
	// -ём / -ьё nouns
	"объём", "приём", "подъём", "съём", "приёмник", "остриё", "жильё", "бельё",
	"ружьё", "копьё", "питьё", "шитьё", "враньё", "бытиё", "забытьё",
	// flight / count families
	"полёт", "налёт", "перелёт", "самолёт", "вертолёт", "взлёт",
	"счёт", "расчёт", "учёт", "отчёт", "зачёт", "почёт", "счёты",
	// colours / qualities (common forms)
	"чёрный", "чёрная", "чёрное", "чёрные", "чёрного", "чёрной", "чёрным",
	"жёлтый", "жёлтая", "жёлтое", "жёлтые",
	"зелёный", "зелёная", "зелёное", "зелёные",
	"тёмный", "тёмная", "тёмное", "тёмные",
	"тёплый", "тёплая", "тёплое",
	"лёгкий", "лёгкая", "лёгкое",
	"надёжный", "далёкий", "весёлый", "серьёзный", "тяжёлый", "чёткий",
	// nouns
	"слёзы", "слёз", "звёзды", "звёзд", "звёздный", "сёстры", "сестёр",
	"вёдра", "гнёзда", "сёла", "рёбра", "бёдра", "вёсла",
	"козёл", "орёл", "ковёр", "костёр", "шофёр", "актёр", "боксёр",
	"чёлка", "пчёлы", "щётка", "тётя", "тётка", "мёд", "лёд",
	"ёлка", "ёж", "ёжик", "посёлок", "новосёл", "утёс", "берёза", "берёзы",
	// verbs (past masculine etc.)
	"нёс", "вёл", "шёл", "пришёл", "ушёл", "нашёл", "пошёл", "прошёл",
	"вошёл", "произошёл", "подошёл", "перешёл", "обошёл",
	"найдём", "придём", "пойдём", "войдём", "идём", "ведём", "несём", "берём",
}

// yoMap maps the (lowercase) «е»-spelling to the correct «ё»-spelling.
var yoMap = buildYoMap()

func buildYoMap() map[string]string {
	m := make(map[string]string, len(yoWords))
	for _, w := range yoWords {
		lw := strings.ToLower(w)
		key := strings.ReplaceAll(lw, "ё", "е")
		if key != lw {
			m[key] = lw
		}
	}
	return m
}

// lookupYo returns the «ё»-restored form of word (preserving case) if word is a
// known unambiguous «е»-spelling.
func lookupYo(word string) (string, bool) {
	lower := strings.ToLower(word)
	yo, ok := yoMap[lower]
	if !ok {
		return "", false
	}
	ow := []rune(word)
	yw := []rune(yo)
	if len(ow) == len(yw) {
		for k := range yw {
			if unicode.IsUpper(ow[k]) {
				yw[k] = unicode.ToUpper(yw[k])
			}
		}
	}
	return string(yw), true
}
