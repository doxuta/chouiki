package chouiki

import (
	"encoding/json"
	"fmt"
)

// The three enums below all behave the same way: they print as a name and
// marshal to that name, so `chouiki parse` emits "kind":"banchi-range" rather
// than an integer whose meaning depends on this file's declaration order.

var (
	kindNames     = []string{"plain", "no-town", "banchi-follows", "whole-town"}
	itemKindNames = []string{"sub-area", "floor", "chome", "banchi", "banchi-range", "except", "kyoto-crossing", "other"}
	noteKindNames = []string{"none", "except", "annotation"}
)

func (k Kind) String() string     { return name(kindNames, int(k)) }
func (k ItemKind) String() string { return name(itemKindNames, int(k)) }
func (k NoteKind) String() string { return name(noteKindNames, int(k)) }

func (k Kind) MarshalJSON() ([]byte, error)     { return json.Marshal(k.String()) }
func (k ItemKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }
func (k NoteKind) MarshalJSON() ([]byte, error) { return json.Marshal(k.String()) }

func (k *Kind) UnmarshalJSON(b []byte) error {
	return unmarshalName(b, kindNames, "Kind", (*int)(k))
}

func (k *ItemKind) UnmarshalJSON(b []byte) error {
	return unmarshalName(b, itemKindNames, "ItemKind", (*int)(k))
}

func (k *NoteKind) UnmarshalJSON(b []byte) error {
	return unmarshalName(b, noteKindNames, "NoteKind", (*int)(k))
}

func name(names []string, i int) string {
	if i < 0 || i >= len(names) {
		return fmt.Sprintf("unknown(%d)", i)
	}
	return names[i]
}

func unmarshalName(b []byte, names []string, typ string, out *int) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	for i, n := range names {
		if n == s {
			*out = i
			return nil
		}
	}
	return fmt.Errorf("chouiki: %q is not a %s", s, typ)
}
