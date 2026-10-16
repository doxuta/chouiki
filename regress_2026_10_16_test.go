package chouiki_test

import (
	"testing"

	"github.com/doxuta/chouiki"
)

// Every field below is copied from Japan Post's postal code data (looked up
// on post.japanpost.jp on 2026-10-16; the web listing spells 〜 as ～ and
// − as －, the CSV spellings are used here).

// TestChomeListSharesTrailingCounter: a 、-separated list carries one counter
// at its end that applies to every member — the same convention the README's
// own headline example relies on for １３−４、２０、６７８、６８７番地. When the
// counter is 丁目, the members before the last one are chōme, not banchi.
//
//	栄町（１、２丁目）                新潟県上越市       942-0072
//	五橋（１、２丁目）                宮城県仙台市青葉区 980-0022
//	自由が丘（１、２丁目）            北海道帯広市       080-0848
//	天神橋（７、８丁目）              大阪府大阪市北区   531-0041
//	西十九条南（３５〜３８、４１、４２丁目） 北海道帯広市   080-0029
func TestChomeListSharesTrailingCounter(t *testing.T) {
	tests := []struct {
		field string
		want  []chouiki.Item
	}{
		{"栄町（１、２丁目）", []chouiki.Item{
			{Kind: chouiki.ItemChome, Raw: "１", Chome: 1},
			{Kind: chouiki.ItemChome, Raw: "２丁目", Chome: 2},
		}},
		{"五橋（１、２丁目）", []chouiki.Item{
			{Kind: chouiki.ItemChome, Raw: "１", Chome: 1},
			{Kind: chouiki.ItemChome, Raw: "２丁目", Chome: 2},
		}},
		{"天神橋（７、８丁目）", []chouiki.Item{
			{Kind: chouiki.ItemChome, Raw: "７", Chome: 7},
			{Kind: chouiki.ItemChome, Raw: "８丁目", Chome: 8},
		}},
		{"西十九条南（３５〜３８、４１、４２丁目）", []chouiki.Item{
			{Kind: chouiki.ItemChome, Raw: "３５〜３８", Chome: 35, ChomeTo: 38},
			{Kind: chouiki.ItemChome, Raw: "４１", Chome: 41},
			{Kind: chouiki.ItemChome, Raw: "４２丁目", Chome: 42},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			got := chouiki.Parse(tt.field)
			if len(got.Items) != len(tt.want) {
				t.Fatalf("got %d items, want %d: %+v", len(got.Items), len(tt.want), got.Items)
			}
			for i, want := range tt.want {
				assertItem(t, got.Items[i], want)
			}
		})
	}

	// The misclassification is not cosmetic: the first member answers
	// Contains as if it were banchi 1 of 栄町.
	first := chouiki.Parse("栄町（１、２丁目）").Items[0]
	if first.Contains(mustBanchi(t, "１")) {
		t.Errorf("%q: chōme 1 of 栄町 must not report containing banchi 1", first.Raw)
	}
}

// TestAbbreviatedRangeEndWithNoContinuesStart: 大豆（１の２、３の２〜６、４の２・４・６、１１の１番地）
// (新潟県上越市 942-0083) lists sub-lots of lots 1, 3, 4 and 11. ３の２〜６
// is 3-2 through 3-6: read literally as 3-2 through 6 it would swallow lot 4
// and lot 5 whole, making the sibling item ４の２・４・６ redundant — Japan
// Post does not list what a neighbouring range already covers. The record's
// sibling 大豆（その他） (943-0802) holds everything else.
func TestAbbreviatedRangeEndWithNoContinuesStart(t *testing.T) {
	items := chouiki.Parse("大豆（１の２、３の２〜６、４の２・４・６、１１の１番地）").Items
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4: %+v", len(items), items)
	}
	it := items[1]
	if it.Kind != chouiki.ItemBanchiRange {
		t.Fatalf("%q: Kind = %v, want banchi-range", it.Raw, it.Kind)
	}
	assertBanchi(t, it.Raw, "From", it.From, &chouiki.Banchi{Nums: []int{3, 2}})
	assertBanchi(t, it.Raw, "To", it.To, &chouiki.Banchi{Nums: []int{3, 6}})

	for _, tc := range []struct {
		banchi string
		want   bool
	}{
		{"３の２", true},
		{"３の４", true},
		{"３の６", true},
		{"３の７", false},
		{"４の２", false}, // listed separately by the sibling item
		{"５", false},
		{"６", false},
	} {
		if got := it.Contains(mustBanchi(t, tc.banchi)); got != tc.want {
			t.Errorf("%q.Contains(%s) = %v, want %v", it.Raw, tc.banchi, got, tc.want)
		}
	}
}
