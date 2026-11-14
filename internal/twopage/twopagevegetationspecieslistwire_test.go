package twopage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTwoPageSpeciesListClientSharedActualReceipts(t *testing.T) {
	path := filepath.Join("frontend", "tests", "fixtures", "twoPageVegetationSpeciesListReceipts.json")
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
