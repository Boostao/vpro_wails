package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSubzoneMatrixDesktopWire(t *testing.T) {
	service, state := subzoneMatrixServiceFixture(t)
	ctx := context.Background()
	first, err := service.GetPreview(ctx, state.ContextID, `{"offset":0,"limit":25}`)
	if err != nil {
		t.Fatal(err)
	}
	page, err := service.GetPreview(ctx, state.ContextID, `{"offset":1,"limit":1}`)
	if err != nil {
		t.Fatal(err)
	}
	end, err := service.GetPreview(ctx, state.ContextID, `{"offset":3,"limit":25}`)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetReview(ctx, state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(preview *SubzoneMatrixPreview) {
		preview.ContextID = "matrix-context"
		preview.HierarchyPath = `C:\fixture\hierarchy.db`
		preview.LabelsPath = `C:\fixture\VPro64.db`
	}
	published, err := service.ExportReviewed(ctx, state.ContextID, subzoneMatrixExportJSON(t, review.ApprovalHash,
		filepath.Join(t.TempDir(), "matrix.xlsx")))
	if err != nil || published.Status != "published" {
		t.Fatal(published, err)
	}
	notPublished, err := service.ExportReviewed(ctx, state.ContextID, subzoneMatrixExportJSON(t, review.ApprovalHash, published.Path))
	if err != nil || notPublished.Status != "not-published" {
		t.Fatal(notPublished, err)
	}
	reads := 0
	publishedWithErrors, err := service.exportReviewed(ctx, state.ContextID, subzoneMatrixExportJSON(t, review.ApprovalHash,
		filepath.Join(t.TempDir(), "matrix.xlsx")), subzoneMatrixPublicationHooks{snapshot: publicationReadSnapshotHooks{
		rollbackRead: func(*sql.Tx) error {
			reads++
			if reads == 3 {
				return errors.New("held original-source cleanup failure")
			}
			return nil
		},
	}})
	if err != nil || publishedWithErrors.Status != "published-with-errors" {
		t.Fatal(publishedWithErrors, err)
	}
	for _, outcome := range []*SubzoneMatrixWorkbookOutcome{published, notPublished, publishedWithErrors} {
		outcome.RequestedDestination = `C:\fixture\matrix.xlsx`
		if outcome.Path != "" {
			outcome.Path = outcome.RequestedDestination
		}
	}
	mutateContextFixture(t, service.contexts.projects.sqlite.attachments["VPro64"], `DELETE FROM USysSubzoneMatrixLabels`)
	empty, err := service.GetPreview(ctx, state.ContextID, `{"offset":0,"limit":25}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, preview := range []*SubzoneMatrixPreview{first, page, end, empty, &review.Preview} {
		normalize(preview)
	}
	fixtures := map[string]any{"first": first, "page": page, "end": end, "empty": empty,
		"review": review, "published": published, "notPublished": notPublished, "publishedWithErrors": publishedWithErrors}
	encoded, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "subzoneMatrixWire.json")
	if os.Getenv("VPRO_SUBZONE_MATRIX_GENERATE_WIRE") == "true" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wires map[string]json.RawMessage
	if err := json.Unmarshal(persisted, &wires); err != nil || len(wires) != len(fixtures) {
		t.Fatal("shared production wire fixture shape changed", err)
	}
	for _, name := range []string{"first", "page", "end", "empty"} {
		var decoded SubzoneMatrixPreview
		decoder := json.NewDecoder(bytes.NewReader(wires[name]))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(fixtures[name], &decoded) {
			t.Fatal("actual production preview differs from shared client wire", name, err)
		}
	}
	var decoded SubzoneMatrixWorkbookReview
	decoder := json.NewDecoder(bytes.NewReader(wires["review"]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(review.Preview, decoded.Preview) ||
		len(decoded.ApprovalHash) != 64 || len(decoded.WorkbookSHA256) != 64 || decoded.Bytes <= 0 {
		t.Fatal("shared workbook wire lost reviewed production shape", err)
	}
	for _, name := range []string{"published", "notPublished", "publishedWithErrors"} {
		var outcome SubzoneMatrixWorkbookOutcome
		decoder := json.NewDecoder(bytes.NewReader(wires[name]))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&outcome); err != nil ||
			outcome.Status != fixtures[name].(*SubzoneMatrixWorkbookOutcome).Status ||
			outcome.RequestedDestination != `C:\fixture\matrix.xlsx` ||
			outcome.Status != "not-published" && outcome.SHA256 != decoded.WorkbookSHA256 {
			t.Fatal("shared irreversible acknowledgement differs from production shape", name, err)
		}
	}
}
