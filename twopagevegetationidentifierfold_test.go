package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func TestTwoPageVegetationIdentifierFoldExhaustiveCompilerContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationIdentifierFold.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Version string   `json:"version"`
		Groups  []string `json:"groups"`
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if table.Version != unicode.Version || len(table.Groups) == 0 {
		t.Fatal("identifier data must match the compiler's Unicode contract; regenerate explicitly", table.Version, unicode.Version)
	}
	keys := map[rune]rune{}
	for _, group := range table.Groups {
		runes := []rune(group)
		if len(runes) < 2 {
			t.Fatal("nontrivial identifier class requires at least two members")
		}
		for i, r := range runes {
			if _, duplicate := keys[r]; duplicate || i > 0 && r <= runes[i-1] ||
				!strings.EqualFold(string(r), string(runes[0])) {
				t.Fatal("identifier class repeats, expands or merges unrelated compiler identities", group, r)
			}
			keys[r] = runes[0]
		}
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		peer := unicode.SimpleFold(r)
		key, present := keys[r]
		if peer == r {
			if present {
				t.Fatal("singleton compiler identity was spuriously folded", r)
			}
		} else if peerKey, peerPresent := keys[peer]; !present || !peerPresent || key != peerKey {
			t.Fatal("client table misses a compiler simple-fold equivalence", r, peer)
		}
	}
}
