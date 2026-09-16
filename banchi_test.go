package chouiki_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/doxuta/chouiki"
)

func TestParseBanchi(t *testing.T) {
	tests := []struct {
		in   string
		want []int
		ok   bool
	}{
		{in: "２０", want: []int{20}, ok: true},
		{in: "６８７番地", want: []int{687}, ok: true},
		{in: "４９０５−１", want: []int{4905, 1}, ok: true},
		{in: "５３の１", want: []int{53, 1}, ok: true},
		{in: "５３１６番地１", want: []int{5316, 1}, ok: true},
		{in: "１０番地の２", want: []int{10, 2}, ok: true},
		// Place names that merely contain digits must not parse as banchi,
		// or Hokkaido's 号 blocks would be swallowed by the numeric kinds.
		{in: "南１３号"},
		{in: "第２地割"},
		{in: "大谷地"},
		{in: ""},
	}

	for _, tt := range tests {
		got, ok := chouiki.ParseBanchi(tt.in)
		if ok != tt.ok {
			t.Errorf("ParseBanchi(%q) ok = %v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if len(got.Nums) != len(tt.want) {
			t.Errorf("ParseBanchi(%q) = %v, want %v", tt.in, got.Nums, tt.want)
			continue
		}
		for i := range tt.want {
			if got.Nums[i] != tt.want[i] {
				t.Errorf("ParseBanchi(%q) = %v, want %v", tt.in, got.Nums, tt.want)
				break
			}
		}
		if got.Raw != tt.in {
			t.Errorf("ParseBanchi(%q).Raw = %q, want the input verbatim", tt.in, got.Raw)
		}
	}
}

func TestBanchiCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "１０", b: "９", want: 1},
		{a: "９", b: "１０", want: -1},
		{a: "４９０５", b: "４９０５−１", want: -1},
		{a: "４９０５−１", b: "４９０５", want: 1},
		{a: "４９０５−１", b: "４９０５の１", want: 0},
		{a: "１２３", b: "１２３番地", want: 0},
	}

	for _, tt := range tests {
		a := mustBanchi(t, tt.a)
		b := mustBanchi(t, tt.b)
		if got := a.Compare(b); got != tt.want {
			t.Errorf("%q.Compare(%q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestItemContains(t *testing.T) {
	items := func(field string) []chouiki.Item {
		t.Helper()
		it := chouiki.Parse(field).Items
		if len(it) == 0 {
			t.Fatalf("Parse(%q) produced no items", field)
		}
		return it
	}

	closed := items("大浦町（４７１３〜５２７７番地）")[0]
	for _, tc := range []struct {
		banchi string
		want   bool
	}{
		{"４７１２", false},
		{"４７１３", true},
		{"５０００", true},
		{"５２７７", true},
		{"５２７８", false},
	} {
		if got := closed.Contains(mustBanchi(t, tc.banchi)); got != tc.want {
			t.Errorf("closed range Contains(%s) = %v, want %v", tc.banchi, got, tc.want)
		}
	}

	// ４３０番地以上 has no upper bound, and carries an exclusion note whose
	// contents this package does not interpret — so it declines to answer.
	openWithNote := items("南山（４３０番地以上「１７７０−１〜２を除く」、大谷地）")[0]
	if openWithNote.Contains(mustBanchi(t, "５０００")) {
		t.Error("a range with an unparsed exclusion note must not claim to contain anything")
	}

	single := items("大江（６８７番地）")[0]
	if !single.Contains(mustBanchi(t, "６８７")) {
		t.Error("single banchi must contain itself")
	}
	if single.Contains(mustBanchi(t, "６８８")) {
		t.Error("single banchi must not contain its neighbour")
	}

	// Nothing else answers, because nothing else was parsed into bounds.
	for _, field := range []string{"一鍬田（その他）", "松本町（御幸町通竹屋町下る）", "音江町（更進）"} {
		if items(field)[0].Contains(mustBanchi(t, "１")) {
			t.Errorf("%s: non-numeric item must not report containment", field)
		}
	}
}

func mustBanchi(t *testing.T, s string) chouiki.Banchi {
	t.Helper()
	b, ok := chouiki.ParseBanchi(s)
	if !ok {
		t.Fatalf("ParseBanchi(%q) failed", s)
	}
	return b
}

// TestKindJSONRoundTrip pins the wire format of `chouiki parse`: kinds travel
// as names, and a Town survives a round trip through JSON.
func TestKindJSONRoundTrip(t *testing.T) {
	town := chouiki.Parse("南山（４３０番地以上「１７７０−１を除く」、大谷地）")

	b, err := json.Marshal(town)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"plain"`, `"kind":"banchi-range"`, `"note_kind":"except"`, `"kind":"sub-area"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("marshalled Town is missing %s:\n%s", want, b)
		}
	}

	var back chouiki.Town
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Kind != town.Kind || len(back.Items) != len(town.Items) {
		t.Fatalf("round trip changed the town: %+v", back)
	}
	for i := range town.Items {
		if back.Items[i].Kind != town.Items[i].Kind || back.Items[i].NoteKind != town.Items[i].NoteKind {
			t.Errorf("item %d round-tripped as %v/%v, want %v/%v",
				i, back.Items[i].Kind, back.Items[i].NoteKind, town.Items[i].Kind, town.Items[i].NoteKind)
		}
	}

	if err := json.Unmarshal([]byte(`"not-a-kind"`), &back.Kind); err == nil {
		t.Error("unmarshalling an unknown kind must fail")
	}
}

// TestBanchiRangeAbbreviatedEnd covers the short form Japan Post uses when the
// end of a range repeats the start's leading banchi. The spelling １７７０−１〜２
// is copied verbatim from the 南山 record in utf_ken_all.csv (it sits inside
// that record's exclusion group); it means 1770-1 through 1770-2, not 1770-1
// through 2, which would run backwards and match nothing.
func TestBanchiRangeAbbreviatedEnd(t *testing.T) {
	items := chouiki.Parse("南山（１７７０−１〜２番地）").Items
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1: %+v", len(items), items)
	}
	it := items[0]
	if it.Kind != chouiki.ItemBanchiRange {
		t.Fatalf("Kind = %v, want banchi-range", it.Kind)
	}
	assertBanchi(t, it.Raw, "From", it.From, &chouiki.Banchi{Nums: []int{1770, 1}})
	assertBanchi(t, it.Raw, "To", it.To, &chouiki.Banchi{Nums: []int{1770, 2}})

	for _, tc := range []struct {
		banchi string
		want   bool
	}{
		{"１７７０−１", true},
		{"１７７０−２", true},
		{"１７７０−３", false},
		{"１７６９−９", false},
		{"２", false},
	} {
		if got := it.Contains(mustBanchi(t, tc.banchi)); got != tc.want {
			t.Errorf("%q.Contains(%s) = %v, want %v", it.Raw, tc.banchi, got, tc.want)
		}
	}
}

// TestRangeBoundsAreOrdered pins the invariant at every site that produces a
// range: either the bounds are ordered, or the text was not classified as a
// range at all. A backwards range is never emitted.
func TestRangeBoundsAreOrdered(t *testing.T) {
	fields := []string{
		"南山（１７７０−１〜２番地）",
		"仁礼（３１５３−１〜１１００）",
		"仁礼（３１５３−１〜３１５３−１１００「峰の原高原」）",
		"大浦町（４７１３〜５２７７番地）",
		"大潟区潟守新田（５３の１〜６０の９番地）",
		"竹房（４５０番地以下）",
		"下甑町瀬々野浦（１７００番地〜）",
		"大通西（１〜１９丁目）",
		// Genuinely backwards text: not a range, so not classified as one.
		"甲（５０〜４０番地）",
		"大通西（１９〜１丁目）",
	}
	for _, field := range fields {
		for _, it := range chouiki.Parse(field).Items {
			if it.Kind == chouiki.ItemBanchiRange && it.From != nil && it.To != nil {
				if it.From.Compare(*it.To) > 0 {
					t.Errorf("%s: range %q runs backwards: from %v to %v",
						field, it.Raw, it.From.Nums, it.To.Nums)
				}
			}
			if it.Kind == chouiki.ItemChome && it.ChomeTo != 0 && it.ChomeTo < it.Chome {
				t.Errorf("%s: chome range %q runs backwards: %d..%d",
					field, it.Raw, it.Chome, it.ChomeTo)
			}
		}
	}
}

// TestBanchiRangeShortEndThatReadsForwardIsKept pins the other half of the
// abbreviated-end rule: the end is expanded only where taking it literally
// would run the range backwards. ５３の１〜６０番地 — the 大潟区潟守新田 spelling
// — already reads forward as 53-1 through 60, so it is kept as written. Reading
// it as 53-1 through 53-60 instead would silently narrow a range that was never
// broken, on nothing better than a guess about which of two readings was meant.
func TestBanchiRangeShortEndThatReadsForwardIsKept(t *testing.T) {
	it := onlyItem(t, "大潟区潟守新田（５３の１〜６０番地）")
	if it.Kind != chouiki.ItemBanchiRange {
		t.Fatalf("Kind = %v, want banchi-range", it.Kind)
	}
	assertBanchi(t, it.Raw, "From", it.From, &chouiki.Banchi{Nums: []int{53, 1}})
	assertBanchi(t, it.Raw, "To", it.To, &chouiki.Banchi{Nums: []int{60}})

	for _, tc := range []struct {
		banchi string
		want   bool
	}{
		{"５４", true}, // inside on the literal reading, outside on the expanded one
		{"６０", true},
		{"６１", false},
		{"５３", false}, // 53 sorts before 53-1, the start
	} {
		if got := it.Contains(mustBanchi(t, tc.banchi)); got != tc.want {
			t.Errorf("%q.Contains(%s) = %v, want %v", it.Raw, tc.banchi, got, tc.want)
		}
	}

	// The sharper case: here expanding would produce 5-9 through 5-6, which is
	// backwards, so an unconditional expansion does not merely narrow the range
	// — it drops the item out of banchi-range entirely. Read literally, it is an
	// ordinary forward range and stays one.
	wide := onlyItem(t, "甲（５−９〜６番地）")
	if wide.Kind != chouiki.ItemBanchiRange {
		t.Fatalf("%q: Kind = %v, want banchi-range", wide.Raw, wide.Kind)
	}
	assertBanchi(t, wide.Raw, "From", wide.From, &chouiki.Banchi{Nums: []int{5, 9}})
	assertBanchi(t, wide.Raw, "To", wide.To, &chouiki.Banchi{Nums: []int{6}})
}

// TestBanchiRangeDegenerate pins that a range whose bounds are equal is still a
// range, and still usable. The ordering check that rejects backwards text must
// reject only what is strictly backwards: loosening it to reject an equal pair
// would drop １００〜１００番地 to the verbatim catch-all.
func TestBanchiRangeDegenerate(t *testing.T) {
	it := onlyItem(t, "甲（１００〜１００番地）")
	if it.Kind != chouiki.ItemBanchiRange {
		t.Fatalf("Kind = %v, want banchi-range", it.Kind)
	}
	assertBanchi(t, it.Raw, "From", it.From, &chouiki.Banchi{Nums: []int{100}})
	assertBanchi(t, it.Raw, "To", it.To, &chouiki.Banchi{Nums: []int{100}})
	if !it.Contains(mustBanchi(t, "１００")) {
		t.Errorf("%q must contain the single banchi it spans", it.Raw)
	}
	if it.Contains(mustBanchi(t, "１０１")) {
		t.Errorf("%q must not contain 101", it.Raw)
	}
}

// onlyItem parses a field that is expected to hold exactly one qualifier item.
func onlyItem(t *testing.T, field string) chouiki.Item {
	t.Helper()
	items := chouiki.Parse(field).Items
	if len(items) != 1 {
		t.Fatalf("Parse(%q): got %d items, want 1: %+v", field, len(items), items)
	}
	return items[0]
}
