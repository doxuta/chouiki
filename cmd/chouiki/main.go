// Command chouiki parses the 町域 field of Japan Post's postal code CSV.
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/doxuta/chouiki"
)

const usage = `chouiki — parse the 町域 field of Japan Post's postal code CSV

usage:
  chouiki explain <field>   parse one 町域 field and print it
  chouiki parse <file.csv>  print one JSON object per record
  chouiki stats <file.csv>  print a breakdown by record kind and item kind

<file.csv> is utf_ken_all.csv (UTF-8, one record per line). Use - for stdin.
A sample lives in testdata/sample.csv.
`

// townColumn is the 町域 (kanji) column of utf_ken_all.csv, zero-indexed.
const townColumn = 8

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "chouiki:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch args[0] {
	case "explain":
		return explain(os.Stdout, strings.Join(args[1:], " "))
	case "parse":
		return eachTown(args[1], func(t chouiki.Town) error {
			return json.NewEncoder(os.Stdout).Encode(t)
		})
	case "stats":
		return stats(os.Stdout, args[1])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
		return nil
	}
}

func explain(w io.Writer, field string) error {
	t := chouiki.Parse(field)
	fmt.Fprintf(w, "raw   %s\n", t.Raw)
	fmt.Fprintf(w, "name  %s\n", orDash(t.Name))
	fmt.Fprintf(w, "kind  %s\n", t.Kind)
	if len(t.Items) == 0 {
		return nil
	}
	fmt.Fprintf(w, "items %d\n", len(t.Items))
	for _, it := range t.Items {
		fmt.Fprintf(w, "  %-14s %s%s\n", it.Kind, it.Raw, detail(it))
	}
	return nil
}

func detail(it chouiki.Item) string {
	var parts []string
	switch it.Kind {
	case chouiki.ItemFloor:
		if it.FloorUnknown {
			parts = append(parts, "floor=unknown")
		} else {
			parts = append(parts, fmt.Sprintf("floor=%d", it.Floor))
		}
	case chouiki.ItemChome:
		if it.ChomeTo != 0 {
			parts = append(parts, fmt.Sprintf("chome=%d..%d", it.Chome, it.ChomeTo))
		} else {
			parts = append(parts, fmt.Sprintf("chome=%d", it.Chome))
		}
	case chouiki.ItemBanchi:
		parts = append(parts, "banchi="+nums(it.Banchi))
	case chouiki.ItemBanchiRange:
		to := "open"
		if it.To != nil {
			to = nums(it.To)
		}
		parts = append(parts, fmt.Sprintf("range=%s..%s", nums(it.From), to))
	}
	if it.NoteKind != chouiki.NoteNone {
		parts = append(parts, fmt.Sprintf("%s=「%s」", it.NoteKind, it.Note))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  [" + strings.Join(parts, " ") + "]"
}

func nums(b *chouiki.Banchi) string {
	if b == nil {
		return "?"
	}
	out := make([]string, len(b.Nums))
	for i, n := range b.Nums {
		out[i] = fmt.Sprint(n)
	}
	return strings.Join(out, "-")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func stats(w io.Writer, path string) error {
	records, qualified := 0, 0
	byKind := map[string]int{}
	byItem := map[string]int{}
	err := eachTown(path, func(t chouiki.Town) error {
		records++
		byKind[t.Kind.String()]++
		if len(t.Items) > 0 {
			qualified++
		}
		for _, it := range t.Items {
			byItem[it.Kind.String()]++
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "records            %d\n", records)
	fmt.Fprintf(w, "with a qualifier   %d\n\n", qualified)
	fmt.Fprintln(w, "record kinds")
	printCounts(w, byKind)
	fmt.Fprintln(w, "\nqualifier items")
	printCounts(w, byItem)
	return nil
}

func printCounts(w io.Writer, counts map[string]int) {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		fmt.Fprintf(w, "  %-16s %8d\n", k, counts[k])
	}
}

func eachTown(path string, fn func(chouiki.Town) error) error {
	f := io.ReadCloser(os.Stdin)
	if path != "-" {
		var err error
		if f, err = os.Open(path); err != nil {
			return err
		}
		defer f.Close()
	}

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(rec) <= townColumn {
			return fmt.Errorf("expected at least %d columns, got %d — is this utf_ken_all.csv?", townColumn+1, len(rec))
		}
		if err := fn(chouiki.Parse(rec[townColumn])); err != nil {
			return err
		}
	}
}
