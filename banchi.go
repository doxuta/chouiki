package chouiki

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Banchi is a lot number, split into its dash-separated components.
// ４９０５−１ parses to [4905 1]; ５３の１ parses to [53 1] — Japan Post uses
// both spellings for the same thing.
type Banchi struct {
	Raw  string `json:"raw"`
	Nums []int  `json:"nums"`
}

// Compare orders two banchi component-wise. A shorter banchi sorts before a
// longer one that shares its prefix, so 4905 < 4905-1.
func (b Banchi) Compare(other Banchi) int {
	for i := range b.Nums {
		if i >= len(other.Nums) {
			return 1
		}
		if d := b.Nums[i] - other.Nums[i]; d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	if len(b.Nums) < len(other.Nums) {
		return -1
	}
	return 0
}

// Contains reports whether b falls inside this item.
//
// It answers only what was actually parsed: a banchi range (open-ended ranges
// have no upper bound) or a single banchi. Every other item kind returns false,
// including a range carrying an exclusion note — the contents of a 「…」 group
// are kept raw, so honouring one is not possible here. Deciding whether a whole
// postal record applies to an address needs more than this; see the README.
func (it Item) Contains(b Banchi) bool {
	switch it.Kind {
	case ItemBanchi:
		return it.Banchi != nil && it.Banchi.Compare(b) == 0
	case ItemBanchiRange:
		if it.NoteKind == NoteExcept {
			return false
		}
		if it.From != nil && it.From.Compare(b) > 0 {
			return false
		}
		return it.To == nil || it.To.Compare(b) >= 0
	default:
		return false
	}
}

// dashes are every character Japan Post uses between banchi components.
const dashes = "−－‐-ー―–"

// rangeSeps are the wave dashes used for ranges. The file uses U+301C, but
// U+FF5E survives round-trips through some Windows toolchains.
const rangeSeps = "〜～"

// toASCIIDigits maps full-width digits onto ASCII and leaves everything else
// untouched.
func toASCIIDigits(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return '0' + (r - '０')
		}
		return r
	}, s)
}

// ParseBanchi parses a single banchi. It reports false for anything that is not
// entirely numeric once the counter suffix and separators are removed, which is
// what keeps place names such as 南１３号 out of the numeric item kinds.
func ParseBanchi(s string) (Banchi, bool) {
	raw := s
	body := toASCIIDigits(strings.TrimSpace(s))
	// A counter is a separator wherever it appears: ５３１６番地１ is 5316-1,
	// and a trailing one simply leaves nothing after the dash.
	for _, sep := range []string{"番地", "地番", "番", "の"} {
		body = strings.ReplaceAll(body, sep, "-")
	}
	for _, d := range dashes {
		body = strings.ReplaceAll(body, string(d), "-")
	}
	// １０番地の２ produces two separators in a row; both spell one boundary.
	for strings.Contains(body, "--") {
		body = strings.ReplaceAll(body, "--", "-")
	}
	body = strings.Trim(body, "-")
	if body == "" {
		return Banchi{}, false
	}

	var nums []int
	for _, part := range strings.Split(body, "-") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return Banchi{}, false
		}
		nums = append(nums, n)
	}
	return Banchi{Raw: raw, Nums: nums}, true
}

// parseFloor parses ２６階 and returns the floor number.
func parseFloor(s string) (int, bool) {
	if !strings.HasSuffix(s, "階") {
		return 0, false
	}
	n, err := strconv.Atoi(toASCIIDigits(strings.TrimSuffix(s, "階")))
	if err != nil {
		return 0, false
	}
	return n, true
}

// parseChome parses a leading chōme number or range: １丁目, １〜１９丁目, and
// ３丁目５ — which is chōme 3, banchi 5. The trailing banchi is left in Raw
// rather than decomposed.
func parseChome(s string) (from, to int, ok bool) {
	body := toASCIIDigits(s)
	i := strings.Index(body, "丁目")
	if i <= 0 {
		return 0, 0, false
	}
	head := body[:i]
	if sep := strings.IndexAny(head, rangeSeps); sep >= 0 {
		from, err := strconv.Atoi(head[:sep])
		if err != nil {
			return 0, 0, false
		}
		to, err := strconv.Atoi(head[sep+sepWidth(head[sep:]):])
		if err != nil {
			return 0, 0, false
		}
		if to < from {
			return 0, 0, false // a backwards range is not a range
		}
		return from, to, true
	}
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0, 0, false
	}
	return n, 0, true
}

// sepWidth returns the byte width of the leading rune of s.
func sepWidth(s string) int {
	_, size := utf8.DecodeRuneInString(s)
	return size
}

// lowerBounded and upperBounded are the suffixes that make a range open at one
// end: ４３０番地以上 has no upper bound, ４５０番地以下 has no lower bound.
var (
	lowerBounded = []string{"以上", "以降"}
	upperBounded = []string{"以下", "以内"}
)

// parseBanchiRange parses ４７１３〜５２７７番地, the abbreviated end form
// １７７０−１〜２ (which is 1770-1 through 1770-2), and the open-ended forms
// ４３０番地以上, ４５０番地以下 and １７００番地〜. A nil bound is unbounded.
// A closed range always comes back ordered: text whose bounds run backwards is
// reported as not a range, so the caller keeps it verbatim instead.
func parseBanchiRange(s string) (from, to *Banchi, ok bool) {
	for _, suffix := range lowerBounded {
		if rest, cut := strings.CutSuffix(s, suffix); cut {
			b, valid := ParseBanchi(rest)
			return &b, nil, valid
		}
	}
	for _, suffix := range upperBounded {
		if rest, cut := strings.CutSuffix(s, suffix); cut {
			b, valid := ParseBanchi(rest)
			return nil, &b, valid
		}
	}

	sep := strings.IndexAny(s, rangeSeps)
	if sep < 0 {
		return nil, nil, false
	}
	lo, valid := ParseBanchi(s[:sep])
	if !valid {
		return nil, nil, false
	}
	tail := s[sep+sepWidth(s[sep:]):]
	if tail == "" {
		return &lo, nil, true // １７００番地〜
	}
	hi, valid := ParseBanchi(tail)
	if !valid {
		return nil, nil, false
	}
	// An end written in short form continues the start rather than restarting:
	// １７７０−１〜２ is 1770-1 through 1770-2, not 1770-1 through 2. Expand it
	// against the start's leading components; Raw still holds it as written.
	if len(hi.Nums) < len(lo.Nums) {
		nums := make([]int, 0, len(lo.Nums))
		nums = append(nums, lo.Nums[:len(lo.Nums)-len(hi.Nums)]...)
		hi.Nums = append(nums, hi.Nums...)
	}
	if lo.Compare(hi) > 0 {
		return nil, nil, false // a backwards range is not a range
	}
	return &lo, &hi, true
}
