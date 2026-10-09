package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestTwoPageVegetationClientUnauditedSignedZeroHistory(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationReceipts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipts []twoPageVegetationWriteResult
	if err := json.Unmarshal(raw, &receipts); err != nil || len(receipts) != 5 {
		t.Fatal("shared source fixture differs", err)
	}
	for strength := 0; strength <= 3; strength++ {
		history := receipts[strength].History
		columns, err := siteUnitTransferColumns(history.Request.Original.Records, "Cover1")
		if err != nil {
			t.Fatal(err)
		}
		before := siviReal(math.Copysign(0, -1))
		history.Request.Original.Records.Rows[0].Cells[columns["Cover1"]] = before
		history.Request.Edits = history.Request.Edits[:1]
		history.Request.Edits[0].Expected, history.Request.Edits[0].Value = before, siviReal(0)
		raw, err := json.Marshal(history.Request.Original.Records)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &history.Committed); err != nil {
			t.Fatal(err)
		}
		history.Committed.Rows[0].Cells[columns["Cover1"]] = siviReal(0)
		history.Audits = []AuditEntry{}
		plan, err := planTwoPageVegetationEdits(context.Background(), history.Request.Original, history.Request.Edits)
		if err != nil || len(plan) != 1 || !math.Signbit(*plan[0].Before.Real) || math.Signbit(*plan[0].After.Real) {
			t.Fatal("signed-zero source plan must remain a complete typed change", plan, err)
		}
		if err := validateTwoPageVegetationHistory(context.Background(), history); err != nil {
			t.Fatal("source audit equality must omit only the sign-change audit, not its history", strength, err)
		}
	}
}
