package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"

	"encoding/json"
	"os"
	"testing"
)

func TestProjectMetadataReadonlyReviewPreservesShapesTypesAndDuplicateCandidates(t *testing.T) {
	service, state := vprotest.ProjectMetadataFixture(t)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	review, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	if review.Project != "Sample" || review.PlotNumber != "META1" || review.ProjectID == nil || *review.ProjectID != "ZMETATEST" ||
		len(review.ProjectRecords.Columns) != 75 || len(review.ProjectRecords.Rows) != 2 ||
		len(review.MasterTemplates.Columns) != 42 || len(review.MasterTemplates.Rows) != 1 {
		t.Fatal("metadata ownership/schema/duplicate candidates changed:", review)
	}
	title := vprotest.MetadataTestCell(t, review.ProjectRecords, "-200", "ProjectTitle")
	empty := vprotest.MetadataTestCell(t, review.ProjectRecords, "-201", "ProjectTitle")
	if title.Storage != "null" || title.Text != nil || empty.Storage != "text" || empty.Text == nil || *empty.Text != "" {
		t.Fatal("NULL/empty metadata collapsed")
	}
	date := vprotest.MetadataTestCell(t, review.ProjectRecords, "-200", "StartDate")
	templateDate := vprotest.MetadataTestCell(t, review.MasterTemplates, review.MasterTemplates.Rows[0].RowID, "StartDate")
	code := vprotest.MetadataTestCell(t, review.MasterTemplates, review.MasterTemplates.Rows[0].RowID, "GeoRefMethod")
	flag := vprotest.MetadataTestCell(t, review.ProjectRecords, "-200", "CollectedSite")
	if date.Integer == nil || *date.Integer != "2000" || templateDate.Text == nil || *templateDate.Text != "2000-01-02 03:04:05" ||
		code.Storage != "integer" || code.Integer == nil || *code.Integer != "7" || flag.Integer == nil || *flag.Integer != "-1" {
		t.Fatal("dates/codes/historical collection options were implicitly converted")
	}
	if _, err := json.Marshal(review); err != nil {
		t.Fatal("typed metadata transport unavailable:", err)
	}
	for role, original := range files {
		path := service.Projects.Sqlite.Attachments[role]
		current, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("readonly metadata review changed data/schema/descriptions:", path, err)
		}
	}
}

func TestProjectMetadataNullEmptyLiteralIdentityAndStaleCancelledRequests(t *testing.T) {
	service, state := vprotest.ProjectMetadataFixture(t)
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	for _, projectID := range []any{nil, "", "zmetatest"} {
		if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID=? WHERE PlotNumber='META1'`, projectID); err != nil {
			t.Fatal(err)
		}
		review, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, "META1")
		if err != nil {
			t.Fatal(err)
		}
		if projectID == nil && review.ProjectID != nil || projectID != nil && (review.ProjectID == nil || *review.ProjectID != projectID) {
			t.Fatal("parent NULL/empty/literal identity changed:", review.ProjectID)
		}
		if len(review.ProjectRecords.Rows) != 0 || len(review.MasterTemplates.Rows) != 0 {
			t.Fatal("literal review selected another project's metadata")
		}
	}
	for _, plot := range []string{"", "META1' OR 1=1 --", "meta1", string([]byte{0xff})} {
		if _, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, plot); err == nil {
			t.Fatal("invalid/foreign parent accepted:", plot)
		}
	}
	if _, err := service.ReviewProjectMetadata(context.Background(), "stale", "META1"); err == nil {
		t.Fatal("stale metadata context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ReviewProjectMetadata(ctx, state.ContextID, "META1"); err == nil {
		t.Fatal("cancelled metadata review returned success")
	}
}

func TestProjectMetadataHistoricalExtraCellsRemainTypedWithoutDecoderRepair(t *testing.T) {
	service, state := vprotest.ProjectMetadataFixture(t)
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`ALTER TABLE Sample_Metadata ADD COLUMN HistoricalExtra;
		UPDATE Sample_Metadata SET HistoricalExtra=x'00FF',NumberOfFS882Plots=9007199254740993,Notes=1.125 WHERE ID=-200`); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	blob := vprotest.MetadataTestCell(t, review.ProjectRecords, "-200", "HistoricalExtra")
	count := vprotest.MetadataTestCell(t, review.ProjectRecords, "-200", "NumberOfFS882Plots")
	if blob.BlobHex == nil || *blob.BlobHex != "00ff" || count.Integer == nil || *count.Integer != "9007199254740993" {
		t.Fatal("historical blob/large integer lost its exact storage")
	}
	if _, err := db.Exec(`UPDATE Sample_Metadata SET Notes=CAST(x'FF' AS TEXT) WHERE ID=-200`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, "META1"); err == nil {
		t.Fatal("malformed metadata Unicode was repaired")
	}
	for _, value := range []struct {
		storage string
		value   any
	}{{"real", 1.125}, {"integer", int64(-1)}, {"null", nil}, {"text", []byte("")}, {"blob", []byte{0xff}}} {
		cell, err := vprocore.ProjectMetadataCellInternal(value.storage, value.value)
		if err != nil || cell.Storage != value.storage {
			t.Fatal("typed cell failed:", cell, err)
		}
		if value.storage == "real" && (cell.Real == nil || *cell.Real != 1.125) {
			t.Fatal("real value changed")
		}
	}
}

func TestProjectMetadataRejectsMalformedAndAmbiguousPhysicalIdentities(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Metadata SET ID=NULL WHERE ProjectID='ZMETATEST'`,
		`UPDATE Sample_Metadata SET ID=0.5 WHERE ID=-200`,
		`UPDATE Sample_Metadata SET ID=2147483648 WHERE ID=-200`,
		`UPDATE Sample_Metadata SET ID=-200 WHERE ID=-201`,
		`UPDATE Sample_Env SET ProjectID=x'FF' WHERE PlotNumber='META1'`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state := vprotest.ProjectMetadataFixture(t)
			db, _, release, err := service.Plots.GetActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(`ALTER TABLE Sample_Metadata RENAME TO MetadataSource;
				CREATE TABLE Sample_Metadata AS SELECT * FROM MetadataSource;
				DROP TABLE MetadataSource;` + mutation); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(state.ProjectPath)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ReviewProjectMetadata(context.Background(), state.ContextID, "META1"); err == nil {
				t.Fatal("unsupported/ambiguous physical identity returned successful metadata")
			}
			after, err := os.ReadFile(state.ProjectPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejected metadata read changed project bytes")
			}
		})
	}
}
