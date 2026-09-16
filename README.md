# chouiki

Parse the 町域 (chōiki, "town area") field of Japan Post's postal code CSV into typed qualifiers, instead of deleting them.

```
$ go run github.com/doxuta/chouiki/cmd/chouiki@latest explain '大江（１丁目、２丁目「６５１、６６２、６６８番地」以外、３丁目５、１３−４、２０、６７８、６８７番地）'
raw   大江（１丁目、２丁目「６５１、６６２、６６８番地」以外、３丁目５、１３−４、２０、６７８、６８７番地）
name  大江
kind  plain
items 7
  chome          １丁目  [chome=1]
  chome          ２丁目「６５１、６６２、６６８番地」以外  [chome=2 except=「６５１、６６２、６６８番地」]
  chome          ３丁目５  [chome=3]
  banchi         １３−４  [banchi=13-4]
  banchi         ２０  [banchi=20]
  banchi         ６７８  [banchi=678]
  banchi         ６８７番地  [banchi=687]
```

No file, no network, no key. That is one real record out of Hokkaido.

## Why this exists

Japan Post's postal code file is famous for being hard to parse, and in June 2023 Japan Post fixed most of what made it famous: `utf_ken_all.csv` is UTF-8, one record per line, with no half-width katakana and no records split at 38 characters. The transport problems are gone.

What the revision did not fix is the 町域 field itself. It is not a place name — it is a place name followed, sometimes, by a note written for a human reader:

| Field | What it means |
|---|---|
| `以下に掲載がない場合` | the postal code covers the whole municipality; there is no town |
| `名駅ミッドランドスクエア（高層棟）（４７階）` | the 47th floor of the high-rise wing |
| `大浦町（４７１３〜５２７７番地）` | only banchi 4713 through 5277 |
| `南山（４３０番地以上「…を除く」、大谷地…）` | banchi 430 and up, minus a list, plus three named hamlets |
| `松本町（御幸町通竹屋町下る…）` | Kyoto, addressed by street crossing |

The record kinds follow Japan Post's own [field definitions](https://www.post.japanpost.jp/zipcode/dl/readme.html): 「○○の次に番地がくる場合」 marks an address with no town between the municipality and the banchi, and 「○○一円」 marks a municipality with no town names at all.

[A Memorandum, July 2023](https://blog1.mammb.com/entry/2023/07/12/090000) argues that this residue is the actual problem and that the revision deliberately skipped the roadmap item that would have specified it machine-readably. Existing libraries agree in practice: they normalise the field by throwing the note away. `chouiki` keeps it and gives it a type.

## What it produces

Over the full 2026-07-31 release — 124,513 records, parsed in 0.05 s:

| | |
|---|---|
| records | 124,513 |
| carrying a qualifier | 6,504 |
| qualifier items | 8,686 |

Record kinds: 122,600 plain, 1,870 no-town, 22 whole-town, 21 banchi-follows.

Item kinds:

| kind | count | example |
|---|---:|---|
| `floor` | 3,437 | `４７階`, `地階・階層不明` |
| `sub-area` | 2,664 | `更進`, `上高地` — free-form, **kept verbatim, not interpreted** |
| `kyoto-crossing` | 1,133 | `御幸町通竹屋町下る` |
| `other` | 564 | `その他`, `全域` |
| `banchi` | 470 | `４９０５−１`, `６８７番地` |
| `chome` | 218 | `１丁目`, `１〜１９丁目` |
| `banchi-range` | 163 | `４７１３〜５２７７番地`, `４３０番地以上` |
| `except` | 37 | `次のビルを除く` |

The largest single bucket after floors is the catch-all. That is the honest shape of the data: roughly 31% of qualifier items are free-form Japanese place names drawn from an unbounded vocabulary, and `chouiki` does not pretend to understand them — it preserves them exactly and labels them as unanalysed.

What is guaranteed instead is that nothing is silently dropped. For a plain record, `Name` plus the item `Raw` values rebuild the input character for character; this was checked across all 122,600 plain records in the release, with 0 mismatches, and a test pins it over the fixture. The other three record kinds deliberately move a phrase out of `Name` — `以下に掲載がない場合` is not a town name — and `Raw` still holds the original field.

## Install

```
go get github.com/doxuta/chouiki
```

```
go install github.com/doxuta/chouiki/cmd/chouiki@latest
```

## Use

As a library:

```go
town := chouiki.Parse("大浦町（４７１３〜５２７７番地）")
// town.Name == "大浦町"
// town.Items[0].Kind == chouiki.ItemBanchiRange
// town.Items[0].From.Nums == []int{4713}

b, _ := chouiki.ParseBanchi("５０００")
town.Items[0].Contains(b) // true
```

As a command, over the real file:

```
curl -O https://www.post.japanpost.jp/service/search/zipcode/download/utf/zip/utf_ken_all.zip
unzip utf_ken_all.zip
chouiki stats utf_ken_all.csv     # the breakdown above
chouiki parse utf_ken_all.csv     # one JSON object per record
```

`parse` emits kinds as names, not as integers:

```json
{"raw":"大浦町（４７１３〜５２７７番地）","name":"大浦町","kind":"plain","items":[{"raw":"４７１３〜５２７７番地","kind":"banchi-range","text":"４７１３〜５２７７番地","from":{"raw":"４７１３","nums":[4713]},"to":{"raw":"５２７７番地","nums":[5277]}}]}
```

A 23-record fixture covering every kind ships in `testdata/sample.csv`, so `chouiki stats testdata/sample.csv` works with nothing downloaded.

## Limitations

Stated before you find them.

- **The free-form bucket is 31% of items.** `sub-area` means "this package did not analyse this"; it does not mean "this is a hamlet name". Read `Raw`.
- **Kyoto crossings are recognised, not understood.** `御幸町通竹屋町下る` is labelled and kept whole. The 上る/下る/西入/東入 direction grammar is not interpreted, and turning it into coordinates is a separate project.
- **`「…」` groups are classified, not parsed.** A group is marked as an exclusion (`を除く`, or a trailing `以外`) or an annotation, and its contents stay raw. An item with an unparsed exclusion refuses to answer `Contains` rather than answering wrongly.
- **No record-level matcher.** There is no "does 〒XXX-XXXX with banchi N resolve to this record". `Contains` answers only about a single parsed range or banchi. Doing it properly needs the exclusion contents parsed and sibling records compared; that is v2.
- **A chōme item with a trailing banchi keeps it in `Raw`.** `３丁目５` reports `chome=3`; the 5 is not decomposed.
- **An abbreviated range end is expanded against its start.** `１７７０−１〜２` is read as 1770-1 through 1770-2, not 1770-1 through 2; `To.Raw` still holds `２` as written. A closed range is always ordered — text whose bounds run backwards is not classified as a range at all, and stays verbatim as `sub-area`.
- **Legacy `KEN_ALL.CSV` is not an input format.** Shift-JIS, half-width katakana, and records split across lines are not handled here on purpose. Run [`inouet/ken-all`](https://github.com/inouet/ken-all) first and feed the result in, or use the UTF-8 file.
- **Two `「…」` groups in one item are left alone.** `第２地割「７０〜１３６」〜第４地割「３〜１１」` classifies as `sub-area`. There are a handful of these, all in Iwate's 地割 system.
- Column 8 is assumed to be the 町域 field, which is true of `utf_ken_all.csv` and of `KEN_ALL.CSV` once joined.

## Prior art

Credit where it is due — all three of these solve the adjacent problem well, and none of them is trying to do what this does:

- [`inouet/ken-all`](https://github.com/inouet/ken-all) by Taku Inoue — joins split records, transcodes Shift-JIS, normalises kana, and drops qualifiers deliberately. If you want a clean address master rather than the qualifiers, use this.
- [`oirik/gokenall`](https://github.com/oirik/gokenall) by oirik — downloads and flattens the legacy file.
- [`naotsugu/jpostal`](https://github.com/naotsugu/jpostal) by Naotsugu Kobayashi — the same normalise-and-discard approach, in Java.

The problem statement comes from [Paul O'Leary McCann's "Parsing the Infamous Japanese Postal CSV"](https://www.dampfkraft.com/posuto.html) and from [A Memorandum's post on the 2023 revision](https://blog1.mammb.com/entry/2023/07/12/090000).

## Data

`testdata/sample.csv` contains 23 records copied verbatim from Japan Post's `utf_ken_all.csv`. Japan Post's [readme](https://www.post.japanpost.jp/zipcode/dl/readme.html) states: 「郵便番号データに限っては日本郵便株式会社は著作権を主張しません。自由に配布していただいて結構です。」 — no copyright is claimed over the postal code data and it may be freely redistributed. The full file is not vendored here.

## Built with

Written with an AI coding agent (Claude) as part of a daily open-source routine, reviewed by hand. The classification counts in this README were produced by running the tool over the real file, not estimated.

## License

MIT — see [LICENSE](LICENSE).
