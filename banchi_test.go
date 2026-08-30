package chouiki_test

import (
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
