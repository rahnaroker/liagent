package fb2

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// declEncodingRe extracts the encoding from an XML declaration prolog. Applied
// to the leading bytes treated as latin1 (the prolog is always ASCII).
var declEncodingRe = regexp.MustCompile(`(?i)<\?xml[^>]*?encoding\s*=\s*["']([^"']+)["']`)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// decodeToUTF8 converts raw FB2 bytes to UTF-8, returning the converted bytes
// and a human-readable description of the detected encoding.
//
// Order of decisions (DESIGN.md §2.1):
//  1. Byte-order marks (UTF-8 / UTF-16 LE / BE).
//  2. The encoding declared in the XML prolog.
//  3. Safety net: if the prolog claims UTF-8 (or omits it) but the bytes are not
//     valid UTF-8, assume windows-1251 — the dominant case for OCR'd FB2.
func decodeToUTF8(raw []byte) ([]byte, string, error) {
	switch {
	case bytes.HasPrefix(raw, bomUTF8):
		return raw[len(bomUTF8):], "utf-8 (BOM)", nil
	case bytes.HasPrefix(raw, bomUTF16LE):
		b, err := transcode(raw[len(bomUTF16LE):], unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder())
		return b, "utf-16le", err
	case bytes.HasPrefix(raw, bomUTF16BE):
		b, err := transcode(raw[len(bomUTF16BE):], unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder())
		return b, "utf-16be", err
	}

	declared := canonEncoding(sniffDeclEncoding(raw))

	switch declared {
	case "", "utf-8":
		if utf8.Valid(raw) {
			return raw, "utf-8", nil
		}
		// Declaration lies (or is absent): the bytes aren't UTF-8. Try 1251.
		if enc, err := htmlindex.Get("windows-1251"); err == nil {
			if b, err := transcode(raw, enc.NewDecoder()); err == nil && utf8.Valid(b) {
				return b, "windows-1251 (declared utf-8/none)", nil
			}
		}
		return raw, "utf-8 (unverified)", nil

	default:
		enc, err := htmlindex.Get(declared)
		if err != nil || enc == nil {
			if utf8.Valid(raw) {
				return raw, declared + " (unknown, treated as utf-8)", nil
			}
			return nil, "", fmt.Errorf("fb2: unsupported encoding %q", declared)
		}
		b, err := transcode(raw, enc.NewDecoder())
		if err != nil {
			return nil, "", fmt.Errorf("fb2: transcoding from %s: %w", declared, err)
		}
		return b, declared, nil
	}
}

// sniffDeclEncoding reads the encoding name from the XML prolog, or "" if absent.
func sniffDeclEncoding(raw []byte) string {
	head := raw
	if len(head) > 1024 {
		head = head[:1024]
	}
	if m := declEncodingRe.FindSubmatch(head); m != nil {
		return string(m[1])
	}
	return ""
}

// canonEncoding maps common FB2 charset aliases to labels htmlindex understands.
func canonEncoding(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "utf8":
		return "utf-8"
	case "cp1251", "windows1251", "win-1251", "1251", "x-cp1251":
		return "windows-1251"
	case "cp1252", "windows1252":
		return "windows-1252"
	case "koi8r", "koi-8", "koi8":
		return "koi8-r"
	default:
		return n
	}
}

func transcode(raw []byte, dec *encoding.Decoder) ([]byte, error) {
	out, err := io.ReadAll(transform.NewReader(bytes.NewReader(raw), dec))
	if err != nil {
		return nil, err
	}
	return out, nil
}
