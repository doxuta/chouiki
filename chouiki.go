// Package chouiki parses the 町域 (chōiki, "town area") field of Japan Post's
// postal code file into typed qualifiers.
//
// The field is not a place name. It is a place name followed, sometimes, by a
// note written for a human reader: a floor of a building, a list of chōme, a
// range of banchi, an exclusion set, or a Kyoto street crossing. Existing
// libraries normalise the field by deleting those notes. This package keeps
// them and gives them types.
//
// The expected input is utf_ken_all.csv (UTF-8, one record per line), the
// format Japan Post added in June 2023. Column 8 (zero-indexed) is the field
// this package parses.
package chouiki

import (
	"strings"
)

// Kind classifies a record as a whole.
type Kind int

const (
	// KindPlain is an ordinary town name, with or without a qualifier.
	KindPlain Kind = iota
	// KindNoTown is 以下に掲載がない場合 — the postal code covers the
	// municipality and no town is named.
	KindNoTown
	// KindBanchiFollows is ○○の次に番地がくる場合 — a banchi follows the
	// municipality name directly, with no town in between.
	KindBanchiFollows
	// KindWholeTown is ○○一円 — the postal code covers the whole municipality.
	KindWholeTown
)

// ItemKind classifies one comma-separated element of a qualifier.
type ItemKind int

const (
	// ItemSubArea is free-form text kept verbatim: a koaza, a hamlet, a
	// landmark. This is the catch-all — the vocabulary is unbounded, so no
	// attempt is made to interpret it.
	ItemSubArea ItemKind = iota
	// ItemFloor is a floor of a named building: ２６階, or 地階・階層不明
	// ("basement or floor unknown").
	ItemFloor
	// ItemChome is a chōme number or a range of them: １丁目, １〜１９丁目.
	ItemChome
	// ItemBanchi is a single banchi: ４９０５−１, ６７８番地.
	ItemBanchi
	// ItemBanchiRange is a banchi range: ４７１３〜５２７７番地, or an
	// open-ended one, ４３０番地以上.
	ItemBanchiRange
	// ItemExcept is an exclusion that stands on its own: 次のビルを除く.
	ItemExcept
	// ItemKyoto is a Kyoto street-crossing designation, recognisable by
	// 上る/下る/西入/東入. Kept whole; the direction grammar is not interpreted.
	ItemKyoto
	// ItemOther is その他 or 全域 — "everything in this town not named by a
	// sibling record".
	ItemOther
)

// NoteKind describes a 「…」 group attached to an item.
type NoteKind int

const (
	// NoteNone means the item carried no 「…」 group.
	NoteNone NoteKind = iota
	// NoteExcept means the group lists banchi that are excluded, either
	// because it ends in を除く or because 以外 follows the closing bracket.
	NoteExcept
	// NoteAnnotation means the group names or narrows the item rather than
	// excluding anything: 「峰の原高原」, 「９番地の４、１２番地」.
	NoteAnnotation
)

// Item is one comma-separated element of a qualifier. Only the fields relevant
// to Kind are populated; Raw always holds the element verbatim.
type Item struct {
	Raw  string   `json:"raw"`
	Kind ItemKind `json:"kind"`
	// Text is Raw with any 「…」 group and a trailing 以外 removed.
	Text string `json:"text"`

	Note     string   `json:"note,omitempty"`
	NoteKind NoteKind `json:"note_kind,omitempty"`

	// Floor is set for ItemFloor. FloorUnknown reports 地階・階層不明, in
	// which case Floor is 0.
	Floor        int  `json:"floor,omitempty"`
	FloorUnknown bool `json:"floor_unknown,omitempty"`

	// Chome is set for ItemChome. ChomeTo is non-zero only for a range.
	Chome   int `json:"chome,omitempty"`
	ChomeTo int `json:"chome_to,omitempty"`

	// Banchi is set for ItemBanchi. From and To are set for ItemBanchiRange;
	// To is nil when the range is open-ended (○○番地以上).
	Banchi *Banchi `json:"banchi,omitempty"`
	From   *Banchi `json:"from,omitempty"`
	To     *Banchi `json:"to,omitempty"`
}

// Town is a parsed 町域 field.
type Town struct {
	Raw  string `json:"raw"`
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	// Items is the parsed qualifier, nil when the field had no （…） group.
	Items []Item `json:"items,omitempty"`
}

const (
	noTown       = "以下に掲載がない場合"
	banchiSuffix = "の次に番地がくる場合"
	wholeSuffix  = "一円"
	floorUnknown = "地階・階層不明"
)

// kyotoMarkers are the crossing directions used in Kyoto street addresses.
// 入 alone is deliberately not a marker: it appears inside ordinary place
// names, whereas these compounds do not.
var kyotoMarkers = []string{"上る", "下る", "西入", "東入", "上ル", "下ル"}

// Parse parses one 町域 field. It never fails: anything it cannot classify is
// preserved verbatim as ItemSubArea, so the caller keeps the original text.
func Parse(field string) Town {
	t := Town{Raw: field, Name: field}

	if name, body, ok := splitQualifier(field); ok {
		t.Name = name
		t.Items = parseItems(body)
	}

	switch {
	case t.Name == noTown:
		t.Kind, t.Name = KindNoTown, ""
	case strings.HasSuffix(t.Name, "がくる場合") && strings.Contains(t.Name, "の次に"):
		// 境町の次に番地がくる場合, and the counted variant
		// 琴平町の次に１〜４２６番地がくる場合.
		t.Kind, t.Name = KindBanchiFollows, ""
	case strings.HasSuffix(t.Name, wholeSuffix) && t.Name != wholeSuffix:
		// 利島村一円 covers a whole municipality. A field that is exactly
		// 一円 is not that: it is the Shiga hamlet named Ichien.
		t.Kind = KindWholeTown
		t.Name = strings.TrimSuffix(t.Name, wholeSuffix)
	}
	return t
}

// splitQualifier separates the town name from its trailing （…） group.
//
// The group is found by matching backwards from the last ）, because a building
// name can itself contain one: 名駅ミッドランドスクエア（高層棟）（１階） is the
// first floor of the high-rise wing, not a group named 高層棟）（１階.
func splitQualifier(field string) (name, body string, ok bool) {
	close := strings.LastIndex(field, "）")
	if close < 0 {
		return "", "", false
	}
	depth := 0
	runes := []rune(field)
	closeRune := len([]rune(field[:close]))
	for i := closeRune; i >= 0; i-- {
		switch runes[i] {
		case '）':
			depth++
		case '（':
			depth--
			if depth == 0 {
				return string(runes[:i]), string(runes[i+1 : closeRune]), true
			}
		}
	}
	return "", "", false
}

// parseItems splits a qualifier body on 、 at bracket depth zero and classifies
// each element.
func parseItems(body string) []Item {
	parts := splitTopLevel(body)
	items := make([]Item, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			items = append(items, classify(p))
		}
	}
	if len(items) == 0 {
		return nil
	}
	// A list shares the counter written on its last member — １３−４、２０、
	// ６７８、６８７番地 and 栄町（１、２丁目） alike — so a bare number before
	// a 丁目 item is a chōme, not a banchi. (For 番地 the bare reading already
	// is a banchi, so only 丁目 needs the carry.)
	if last := items[len(items)-1]; last.Kind == ItemChome && strings.HasSuffix(last.Text, "丁目") {
		for i := range items[:len(items)-1] {
			it := &items[i]
			if it.Kind != ItemBanchi && it.Kind != ItemBanchiRange {
				continue
			}
			if from, to, ok := parseChome(it.Text + "丁目"); ok {
				it.Kind, it.Chome, it.ChomeTo = ItemChome, from, to
				it.Banchi, it.From, it.To = nil, nil, nil
			}
		}
	}
	return items
}

// splitTopLevel splits on 、 outside any 「…」 or （…） group.
func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, r := range s {
		switch r {
		case '「', '（', '〔':
			depth++
		case '」', '）', '〕':
			if depth > 0 {
				depth--
			}
		case '、':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + len("、")
			}
		}
	}
	return append(parts, s[start:])
}

func classify(raw string) Item {
	it := Item{Raw: raw, Text: raw}

	// A 「…」 group (or its 〔…〕 spelling) qualifies the text before it. Only a
	// single group is interpreted; two or more, as in
	// 第２地割「７０〜１３６」〜第４地割「３〜１１」, are left alone.
	if o, c, ok := singleNote(raw); ok {
		it.Note = raw[o+len("「") : c]
		rest := raw[:o] + raw[c+len("」"):]
		if strings.HasSuffix(rest, "以外") {
			it.NoteKind = NoteExcept
			rest = strings.TrimSuffix(rest, "以外")
		} else if strings.HasSuffix(it.Note, "を除く") {
			it.NoteKind = NoteExcept
		} else {
			it.NoteKind = NoteAnnotation
		}
		it.Text = rest
	}

	text := it.Text
	switch {
	case text == floorUnknown:
		it.Kind, it.FloorUnknown = ItemFloor, true
		return it
	case text == "その他" || text == "全域":
		it.Kind = ItemOther
		return it
	case hasAny(text, kyotoMarkers):
		it.Kind = ItemKyoto
		return it
	case strings.HasSuffix(text, "を除く") || text == "以外":
		it.Kind = ItemExcept
		return it
	}

	if n, ok := parseFloor(text); ok {
		it.Kind, it.Floor = ItemFloor, n
		return it
	}
	if from, to, ok := parseChome(text); ok {
		it.Kind, it.Chome, it.ChomeTo = ItemChome, from, to
		return it
	}
	if from, to, ok := parseBanchiRange(text); ok {
		it.Kind, it.From, it.To = ItemBanchiRange, from, to
		return it
	}
	if b, ok := ParseBanchi(text); ok {
		it.Kind, it.Banchi = ItemBanchi, &b
		return it
	}
	it.Kind = ItemSubArea
	return it
}

// noteBrackets are the bracket pairs Japan Post uses for a sub-annotation.
// Both members of a pair are three bytes wide in UTF-8.
var noteBrackets = [][2]string{{"「", "」"}, {"〔", "〕"}}

// singleNote returns the byte offsets of a lone bracketed group in s.
func singleNote(s string) (open, close int, ok bool) {
	for _, pair := range noteBrackets {
		if strings.Count(s, pair[0]) != 1 {
			continue
		}
		o, c := strings.Index(s, pair[0]), strings.LastIndex(s, pair[1])
		if o >= 0 && c > o {
			return o, c, true
		}
	}
	return 0, 0, false
}

func hasAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
