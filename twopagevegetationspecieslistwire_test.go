package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTwoPageSpeciesListClientSharedActualReceipts(t *testing.T) {
	path := filepath.Join("frontend", "src", "twoPageVegetationSpeciesListReceipts.json")
	if output := os.Getenv("VPRO_TWO_PAGE_SPECIES_LIST_FIXTURE_OUTPUT"); output != "" {
		receipts := []twoPageVegetationWriteResult{}
		for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
			for strength := 0; strength <= 3; strength++ {
				contexts, state, _, request := twoPageSpeciesListWriteFixture(t, false, form, strength)
				result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
				if err != nil || result == nil || !result.DidCommit {
					t.Fatal("actual shared list fixture did not commit", result, err)
				}
				receipts = append(receipts, *result)
			}
		}
		raw, err := json.MarshalIndent(receipts, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := file.Write(append(raw, '\n'))
		if err := errors.Join(writeErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		path = output
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []twoPageVegetationWriteResult
	if err := json.Unmarshal(raw, &receipts); err != nil || len(receipts) != 8 {
		t.Fatal("shared list fixture requires eight strict-Go actual receipts", err)
	}
	counts := []int{0, 2, 3, 3}
	for index, receipt := range receipts {
		form := "SubVegAXL_BC"
		if index >= 4 {
			form = "SubVegAXL"
		}
		if !receipt.DidCommit || receipt.Replayed || receipt.History.Request.SpeciesList == nil ||
			receipt.History.Request.Original.Form != form || receipt.History.AuditStrength != index%4 ||
			len(receipt.History.Audits) != counts[index%4] || len(receipt.History.Request.Edits) != 3 {
			t.Fatal("shared list fixture ownership/counts differ", index)
		}
		if err := validateTwoPageVegetationHistory(context.Background(), receipt.History); err != nil {
			t.Fatal("shared list fixture fails strict production history", index, err)
		}
	}
}
