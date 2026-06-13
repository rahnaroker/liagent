package epub

import (
	"path"
	"strings"
)

var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
)

// escText escapes character data for XHTML text content.
func escText(s string) string { return textEscaper.Replace(s) }

// escAttr escapes a value for use inside a double-quoted attribute.
func escAttr(s string) string { return attrEscaper.Replace(s) }

// safeID turns an arbitrary FB2 id into a valid XML NCName: it must not start
// with a digit and may only contain a restricted character set.
func safeID(id string) string {
	if id == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case (r >= '0' && r <= '9' || r == '-' || r == '.') && i > 0:
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "id_" + out
	}
	return out
}

// relHref returns href to target (an OEBPS-root-relative path + optional frag)
// as seen from the document at fromFile (also OEBPS-root-relative).
func relHref(fromFile, targetFile, frag string) string {
	href := relPath(fromFile, targetFile)
	if frag != "" {
		href += "#" + frag
	}
	return href
}

// relPath computes a forward-slash relative path from fromFile to target, both
// given as OEBPS-root-relative paths.
func relPath(fromFile, target string) string {
	var from []string
	if d := path.Dir(fromFile); d != "." {
		from = strings.Split(d, "/")
	}
	var tdir []string
	if d := path.Dir(target); d != "." {
		tdir = strings.Split(d, "/")
	}
	base := path.Base(target)

	i := 0
	for i < len(from) && i < len(tdir) && from[i] == tdir[i] {
		i++
	}
	var out []string
	for j := i; j < len(from); j++ {
		out = append(out, "..")
	}
	out = append(out, tdir[i:]...)
	out = append(out, base)
	return strings.Join(out, "/")
}
