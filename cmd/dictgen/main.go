// Command dictgen builds the embedded Russian word-form set used to gate
// auto de-hyphenation. It reads a windows-1251 word list (one word per line)
// and writes a gzipped, UTF-8, lowercased, ё→е-normalised, Cyrillic-only,
// deduplicated list to core/rules/dict/ru_words.txt.gz.
//
//	go run ./cmd/dictgen <russian.txt> [out.gz]
package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dictgen <russian.txt> [out.gz]")
		os.Exit(2)
	}
	in := os.Args[1]
	out := "core/rules/dict/ru_words.txt.gz"
	if len(os.Args) >= 3 {
		out = os.Args[2]
	}
	if err := run(in, out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()

	r := transform.NewReader(f, charmap.Windows1251.NewDecoder())
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	set := make(map[string]struct{}, 2_000_000)
	for sc.Scan() {
		w := normalize(sc.Text())
		if w != "" {
			set[w] = struct{}{}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	words := make([]string, 0, len(set))
	for w := range set {
		words = append(words, w)
	}
	sort.Strings(words)

	if err := os.MkdirAll("core/rules/dict", 0o755); err != nil {
		return err
	}
	of, err := os.Create(out)
	if err != nil {
		return err
	}
	defer of.Close()
	gw, _ := gzip.NewWriterLevel(of, gzip.BestCompression)
	bw := bufio.NewWriter(gw)
	for _, w := range words {
		bw.WriteString(w)
		bw.WriteByte('\n')
	}
	bw.Flush()
	gw.Close()

	fmt.Printf("wrote %d words to %s\n", len(words), out)
	return nil
}

// normalize lowercases, maps ё→е and keeps only pure-Cyrillic words of length≥2.
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "ё", "е")
	if len([]rune(s)) < 2 {
		return ""
	}
	for _, r := range s {
		if !unicode.Is(unicode.Cyrillic, r) {
			return ""
		}
	}
	return s
}
