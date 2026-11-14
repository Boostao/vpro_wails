package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestTwoPageVegetationClientSharedActualReceipts(t *testing.T) {
	path := filepath.Join("frontend", "src", "twoPageVegetationReceipts.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []twoPageVegetationWriteResult
	if err := json.Unmarshal(raw, &receipts); err != nil || len(receipts) != 5 {
		t.Fatal("shared client receipts must be complete strict-Go actual writer results", err)
	}
	for index, receipt := range receipts {
		if !receipt.DidCommit || receipt.Replayed {
			t.Fatal("shared actual writer fixture must prove commit", index)
		}
		if err := validateTwoPageVegetationHistory(context.Background(), receipt.History); err != nil {
			t.Fatal("shared receipt differs from strict production history", index, err)
		}
		if index < 4 && (receipt.History.AuditStrength != index || len(receipt.History.Audits) != index ||
			len(receipt.History.Request.Edits) != 3) {
			t.Fatal("shared audit policies differ from independently expected counts", index)
		}
		if index == 4 {
			columns, err := vprocore.SiteUnitTransferColumns(receipt.History.Committed, "Cover6")
			if err != nil || receipt.History.Request.Original.Form != "SubVegCXL" ||
				receipt.History.Committed.Rows[0].RowID != "-7" ||
				receipt.History.Committed.Rows[0].Cells[columns["Cover6"]].Storage != "null" {
				t.Fatal("last C cover removal fixture differs", index, err)
			}
		}
	}
	blob := "ff00"
	for _, before := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("5"), {Storage: "blob", BlobHex: &blob}} {
		raw, err := json.Marshal(receipts[1])
		if err != nil {
			t.Fatal(err)
		}
		var receipt twoPageVegetationWriteResult
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		columns, err := vprocore.SiteUnitTransferColumns(receipt.History.Request.Original.Records, "Cover1", "cover3")
		if err != nil {
			t.Fatal(err)
		}
		receipt.History.Request.Original.Records.Rows[0].Cells[columns["Cover1"]] = before
		receipt.History.Request.Edits[0].Expected = before
		text := "5"
		if before.Storage == "blob" {
			text = "[255 0]"
		}
		receipt.History.Audits[0].BeforeEdit = &text
		negativeZero := vprotest.SiviReal(math.Copysign(0, -1))
		receipt.History.Request.Original.Records.Rows[1].Cells[columns["cover3"]] = negativeZero
		receipt.History.Committed.Rows[1].Cells[columns["cover3"]] = negativeZero
		if err := validateTwoPageVegetationHistory(context.Background(), receipt.History); err != nil {
			t.Fatal("independent typed/BLOB audit and signed-zero peer contract differs", err)
		}
	}
}

func TestTwoPageVegetationClientSharedOutcomeWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationReceipts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipts []twoPageVegetationWriteResult
	if err := json.Unmarshal(raw, &receipts); err != nil || len(receipts) != 5 {
		t.Fatal("shared client receipts differ", err)
	}
	outcomePath := filepath.Join("frontend", "src", "twoPageVegetationOutcomes.json")
	request := receipts[3].History.Request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(outcomePath)
	if err != nil {
		t.Fatal(err)
	}
	var outcomes []TwoPageVegetationAttemptOutcome
	if err := json.Unmarshal(raw, &outcomes); err != nil || len(outcomes) != 4 {
		t.Fatal("shared client outcome wire differs from Go model", err)
	}
	for index, status := range []string{"running", "unknown", "notCommitted", "committed"} {
		outcome := outcomes[index]
		if outcome.ContextID != request.Original.ContextID || outcome.RequestID != request.RequestID ||
			outcome.RequestJSON != string(requestJSON) || outcome.Status != status || outcome.Reason == "" ||
			(status == "committed") != (outcome.Receipt != nil) {
			t.Fatal("shared client outcome must retain complete exact request and status", index)
		}
		if outcome.Receipt != nil {
			if outcome.Receipt.DidCommit || !outcome.Receipt.Replayed {
				t.Fatal("outcome receipt must be readonly")
			}
			if err := validateTwoPageVegetationHistory(context.Background(), outcome.Receipt.History); err != nil {
				t.Fatal(err)
			}
		}
	}
}
