// Command cli converts an FB2 file to EPUB3 from the terminal. It exercises the
// core pipeline (parse -> build) without the GUI and is handy for batch use.
//
//	litagent <input.fb2> <output.epub>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"litagent/core/epub"
	"litagent/core/fb2"
	"litagent/core/rules"
)

func main() {
	args := os.Args[1:]
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: litagent <input.fb2> [output.epub]")
		os.Exit(2)
	}
	in := args[0]
	out := ""
	if len(args) == 2 {
		out = args[1]
	} else {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".epub"
	}

	if err := run(in, out); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Printf("ok: %s -> %s\n", in, out)
}

func run(in, out string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()

	book, err := fb2.Parse(f)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	findings := rules.DefaultEngine().Run(book)
	reportFindings(findings)

	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()

	if err := epub.Build(book, epub.Options{}, w); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	return nil
}

// reportFindings prints a per-rule summary, separating applied edits (levels
// A/B) from review-only suggestions (level C).
func reportFindings(findings []rules.Finding) {
	if len(findings) == 0 {
		fmt.Println("rules: no changes")
		return
	}
	type stat struct {
		count int
		level rules.Confidence
	}
	stats := map[string]*stat{}
	applied, suggested := 0, 0
	for _, f := range findings {
		s := stats[f.RuleID]
		if s == nil {
			s = &stat{level: f.Level}
			stats[f.RuleID] = s
		}
		s.count++
		if f.Level == rules.ConfC {
			suggested++
		} else {
			applied++
		}
	}
	ids := make([]string, 0, len(stats))
	for id := range stats {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return stats[ids[i]].count > stats[ids[j]].count })

	fmt.Printf("rules: %d applied (A/B), %d suggested (C, review-only)\n", applied, suggested)
	for _, id := range ids {
		s := stats[id]
		kind := "applied"
		if s.level == rules.ConfC {
			kind = "suggest"
		}
		fmt.Printf("  [%s] %-16s %d  (%s)\n", s.level, id, s.count, kind)
	}
}
