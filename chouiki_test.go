package chouiki_test

import (
	"encoding/csv"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/doxuta/chouiki"
)

// Every field below is copied verbatim from utf_ken_all.csv.
func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		field string
		want  chouiki.Town
	}{
		{
			name:  "plain town has no qualifier",
			field: "旭ケ丘",
			want:  chouiki.Town{Name: "旭ケ丘", Kind: chouiki.KindPlain},
		},
		{
			name:  "no town named",
			field: "以下に掲載がない場合",
			want:  chouiki.Town{Name: "", Kind: chouiki.KindNoTown},
		},
		{
			name:  "banchi follows the municipality",
			field: "境町の次に番地がくる場合",
			want:  chouiki.Town{Name: "", Kind: chouiki.KindBanchiFollows},
		},
		{
			name:  "counted banchi-follows keeps its qualifier",
			field: "琴平町の次に１〜４２６番地がくる場合（川東）",
			want: chouiki.Town{Name: "", Kind: chouiki.KindBanchiFollows, Items: []chouiki.Item{
				{Kind: chouiki.ItemSubArea, Raw: "川東"},
			}},
		},
		{
			name:  "whole municipality",
			field: "利島村一円",
			want:  chouiki.Town{Name: "利島村", Kind: chouiki.KindWholeTown},
		},
		{
			// The one record in the file where 一円 is the town's actual name.
			name:  "bare 一円 is a place name, not a whole-municipality marker",
			field: "一円",
			want:  chouiki.Town{Name: "一円", Kind: chouiki.KindPlain},
		},
		{
			// The building name contains its own （…）; the qualifier is the
			// last group, not everything after the first bracket.
			name:  "building name containing parentheses",
			field: "名駅ミッドランドスクエア（高層棟）（４７階）",
			want: chouiki.Town{Name: "名駅ミッドランドスクエア（高層棟）", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemFloor, Raw: "４７階", Floor: 47},
			}},
		},
		{
			name:  "floor unknown",
			field: "丸の内グラントウキョウノースタワー（地階・階層不明）",
			want: chouiki.Town{Name: "丸の内グラントウキョウノースタワー", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemFloor, Raw: "地階・階層不明", FloorUnknown: true},
			}},
		},
		{
			name:  "chome range",
			field: "大通西（１〜１９丁目）",
			want: chouiki.Town{Name: "大通西", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemChome, Raw: "１〜１９丁目", Chome: 1, ChomeTo: 19},
			}},
		},
		{
			name:  "banchi range spelled with の",
			field: "大潟区潟守新田（５３の１〜６０の９番地）",
			want: chouiki.Town{Name: "大潟区潟守新田", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemBanchiRange, Raw: "５３の１〜６０の９番地",
					From: &chouiki.Banchi{Nums: []int{53, 1}}, To: &chouiki.Banchi{Nums: []int{60, 9}}},
			}},
		},
		{
			name:  "kyoto street crossings",
			field: "松本町（御幸町通竹屋町下る、御幸町通夷川上る）",
			want: chouiki.Town{Name: "松本町", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemKyoto, Raw: "御幸町通竹屋町下る"},
				{Kind: chouiki.ItemKyoto, Raw: "御幸町通夷川上る"},
			}},
		},
		{
			name:  "standalone exclusion",
			field: "中央（次のビルを除く）",
			want: chouiki.Town{Name: "中央", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemExcept, Raw: "次のビルを除く"},
			}},
		},
		{
			name:  "その他 is its own kind",
			field: "一鍬田（その他）",
			want: chouiki.Town{Name: "一鍬田", Kind: chouiki.KindPlain, Items: []chouiki.Item{
				{Kind: chouiki.ItemOther, Raw: "その他"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chouiki.Parse(tt.field)
			if got.Raw != tt.field {
				t.Errorf("Raw = %q, want the field verbatim", got.Raw)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Kind != tt.want.Kind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.want.Kind)
			}
			if len(got.Items) != len(tt.want.Items) {
				t.Fatalf("got %d items, want %d: %+v", len(got.Items), len(tt.want.Items), got.Items)
			}
			for i, want := range tt.want.Items {
				assertItem(t, got.Items[i], want)
			}
		})
	}
}

// TestParseExclusionQualifier covers the field the mammb blog post uses as its
// example of what the 2023 format revision left unsolved.
func TestParseExclusionQualifier(t *testing.T) {
	const field = "大江（１丁目、２丁目「６５１、６６２、６６８番地」以外、３丁目５、１３−４、２０、６７８、６８７番地）"

	got := chouiki.Parse(field)
	if got.Name != "大江" {
		t.Fatalf("Name = %q, want 大江", got.Name)
	}
	want := []chouiki.Item{
		{Kind: chouiki.ItemChome, Raw: "１丁目", Chome: 1},
		{Kind: chouiki.ItemChome, Raw: "２丁目「６５１、６６２、６６８番地」以外", Chome: 2,
			Note: "６５１、６６２、６６８番地", NoteKind: chouiki.NoteExcept},
		{Kind: chouiki.ItemChome, Raw: "３丁目５", Chome: 3},
		{Kind: chouiki.ItemBanchi, Raw: "１３−４", Banchi: &chouiki.Banchi{Nums: []int{13, 4}}},
		{Kind: chouiki.ItemBanchi, Raw: "２０", Banchi: &chouiki.Banchi{Nums: []int{20}}},
		{Kind: chouiki.ItemBanchi, Raw: "６７８", Banchi: &chouiki.Banchi{Nums: []int{678}}},
		{Kind: chouiki.ItemBanchi, Raw: "６８７番地", Banchi: &chouiki.Banchi{Nums: []int{687}}},
	}
	if len(got.Items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(got.Items), len(want), got.Items)
	}
	for i := range want {
		assertItem(t, got.Items[i], want[i])
	}
}

// TestParseNestedAnnotationIsNotSplit guards the 、 split: a comma inside a
// 「…」 group belongs to that group, not to the qualifier.
func TestParseNestedAnnotationIsNotSplit(t *testing.T) {
	const field = "南山（４３０番地以上「１７７０−１〜２、１８６２−４２、１９２３−５を除く」、大谷地、折渡）"

	got := chouiki.Parse(field)
	if len(got.Items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(got.Items), got.Items)
	}
	first := got.Items[0]
	if first.Kind != chouiki.ItemBanchiRange {
		t.Errorf("Kind = %v, want banchi-range", first.Kind)
	}
	if first.From == nil || first.From.Nums[0] != 430 {
		t.Errorf("From = %v, want 430", first.From)
	}
	if first.To != nil {
		t.Errorf("To = %v, want nil (以上 is open-ended)", first.To)
	}
	if first.NoteKind != chouiki.NoteExcept {
		t.Errorf("NoteKind = %v, want except", first.NoteKind)
	}
}

// TestParseNeverLosesText is the invariant that makes the catch-all safe: the
// concatenated item Raw values plus the name must account for the whole field.
func TestParseNeverLosesText(t *testing.T) {
	forEachSampleTown(t, func(field string) {
		town := chouiki.Parse(field)
		if town.Kind != chouiki.KindPlain {
			// The other kinds deliberately drop a phrase from Name; Raw keeps it.
			return
		}
		raws := make([]string, 0, len(town.Items))
		for _, it := range town.Items {
			raws = append(raws, it.Raw)
		}
		rebuilt := town.Name
		if len(raws) > 0 {
			rebuilt += "（" + strings.Join(raws, "、") + "）"
		}
		if rebuilt != field {
			t.Errorf("round trip lost text:\n got %q\nwant %q", rebuilt, field)
		}
	})
}

// TestSampleBreakdown pins the classification of a fixture of real records.
// It fails if any change moves a record between kinds.
func TestSampleBreakdown(t *testing.T) {
	wantRecords := map[chouiki.Kind]int{
		chouiki.KindPlain:         19,
		chouiki.KindNoTown:        1,
		chouiki.KindBanchiFollows: 2,
		chouiki.KindWholeTown:     1,
	}
	wantItems := map[chouiki.ItemKind]int{
		chouiki.ItemBanchi:      53,
		chouiki.ItemSubArea:     13,
		chouiki.ItemBanchiRange: 7,
		chouiki.ItemChome:       6,
		chouiki.ItemKyoto:       6,
		chouiki.ItemFloor:       2,
		chouiki.ItemExcept:      1,
		chouiki.ItemOther:       1,
	}

	gotRecords := map[chouiki.Kind]int{}
	gotItems := map[chouiki.ItemKind]int{}
	forEachSampleTown(t, func(field string) {
		town := chouiki.Parse(field)
		gotRecords[town.Kind]++
		for _, it := range town.Items {
			gotItems[it.Kind]++
		}
	})

	for kind, want := range wantRecords {
		if gotRecords[kind] != want {
			t.Errorf("record kind %v: got %d, want %d", kind, gotRecords[kind], want)
		}
	}
	for kind, want := range wantItems {
		if gotItems[kind] != want {
			t.Errorf("item kind %v: got %d, want %d", kind, gotItems[kind], want)
		}
	}
}

func assertItem(t *testing.T, got, want chouiki.Item) {
	t.Helper()
	if got.Kind != want.Kind {
		t.Errorf("item %q: Kind = %v, want %v", got.Raw, got.Kind, want.Kind)
	}
	if want.Raw != "" && got.Raw != want.Raw {
		t.Errorf("Raw = %q, want %q", got.Raw, want.Raw)
	}
	if got.Floor != want.Floor || got.FloorUnknown != want.FloorUnknown {
		t.Errorf("item %q: floor = (%d, unknown=%v), want (%d, unknown=%v)",
			got.Raw, got.Floor, got.FloorUnknown, want.Floor, want.FloorUnknown)
	}
	if got.Chome != want.Chome || got.ChomeTo != want.ChomeTo {
		t.Errorf("item %q: chome = %d..%d, want %d..%d", got.Raw, got.Chome, got.ChomeTo, want.Chome, want.ChomeTo)
	}
	if want.NoteKind != chouiki.NoteNone {
		if got.NoteKind != want.NoteKind {
			t.Errorf("item %q: NoteKind = %v, want %v", got.Raw, got.NoteKind, want.NoteKind)
		}
		if got.Note != want.Note {
			t.Errorf("item %q: Note = %q, want %q", got.Raw, got.Note, want.Note)
		}
	}
	assertBanchi(t, got.Raw, "Banchi", got.Banchi, want.Banchi)
	assertBanchi(t, got.Raw, "From", got.From, want.From)
	assertBanchi(t, got.Raw, "To", got.To, want.To)
}

func assertBanchi(t *testing.T, item, field string, got, want *chouiki.Banchi) {
	t.Helper()
	if want == nil {
		return
	}
	if got == nil {
		t.Errorf("item %q: %s = nil, want %v", item, field, want.Nums)
		return
	}
	if len(got.Nums) != len(want.Nums) {
		t.Errorf("item %q: %s = %v, want %v", item, field, got.Nums, want.Nums)
		return
	}
	for i := range want.Nums {
		if got.Nums[i] != want.Nums[i] {
			t.Errorf("item %q: %s = %v, want %v", item, field, got.Nums, want.Nums)
			return
		}
	}
}

// townColumn matches the CLI's view of utf_ken_all.csv.
const townColumn = 8

func forEachSampleTown(t *testing.T, fn func(field string)) {
	t.Helper()
	f, err := os.Open("testdata/sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		fn(rec[townColumn])
	}
}
