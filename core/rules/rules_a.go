package rules

// Level-A rules (DESIGN.md §4.5): deterministic, safe to auto-apply. Defined as
// vars so the pipeline (pipeline.go) can order them among the B rules.

var ruleCtrlClean = reRule(
	"ctrl-clean", "Невидимые/битые символы", "ocr", ConfA,
	`[\x{0000}-\x{0008}\x{000B}\x{000C}\x{000E}-\x{001F}\x{00AD}\x{200B}-\x{200D}\x{2060}\x{FEFF}\x{FFFD}]`,
	constRepl(""),
)

var ruleSpWs = reRule(
	"sp-ws", "Лишние пробелы и переносы", "spacing", ConfA,
	`[ \t\r\n]+`, constRepl(" "),
)

var ruleDashMulti = reRule(
	"dash-multi", "Двойной/тройной дефис → тире", "typography", ConfA,
	`-{2,}`, constRepl("—"),
)

var ruleEllDots = reRule(
	"ell-dots", "Три точки → многоточие", "typography", ConfA,
	`\.{3,}`, constRepl("…"),
)

// ell-spaced: an ellipsis stretched with spaces (". . .", ". ..") → "…". Requires
// at least three dots, so "т. е." (a dot followed by a letter) is never matched.
// Runs after ell-dots, which has already collapsed contiguous "...".
var ruleEllSpaced = reRule(
	"ell-spaced", "Растянутое многоточие → …", "typography", ConfA,
	`\.(?:[ \x{00A0}]*\.){2,}`, constRepl("…"),
)

var ruleSpBeforePunct = reRule(
	"sp-before-punct", "Пробел перед знаком препинания", "spacing", ConfA,
	`[ \x{00A0}]+([,.;:!?…)»])`, group(1),
)

var ruleSpAfterPunct = reRule(
	"sp-after-punct", "Нет пробела после знака", "spacing", ConfA,
	`([,;:!?])(\p{L})`, func(g []string) string { return g[1] + " " + g[2] },
)

var ruleDashHyphen = reRule(
	"dash-hyphen", "Дефис между пробелами → тире", "typography", ConfA,
	` - `, constRepl(" — "),
)

var ruleSymCopy = reRule("sym-copy", "(c) → ©", "typography", ConfA, `\([cC]\)`, constRepl("©"))
var ruleSymReg = reRule("sym-reg", "(r) → ®", "typography", ConfA, `\([rR]\)`, constRepl("®"))
var ruleSymTm = reRule("sym-tm", "(tm) → ™", "typography", ConfA, `\([tT][mM]\)`, constRepl("™"))

var ruleDupComma = reRule(
	"dup-comma", "Повтор запятых", "punctuation", ConfA,
	`,{2,}`, constRepl(","),
)
