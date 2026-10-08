package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func TestPictureMetadataServiceSaveReceiptAndStableReplay(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	contexts.plots.currentUser = "picture metadata service"
	contexts.plots.auditStrength = 3
	_, path := pictureLibraryFixture(t, pictureFixtureSchema)
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO tblVPics(ID,PicDir,PicName,PlotNumber,PicComment)
		VALUES(12345,'Default','Original.jpg','108050',NULL)`)
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	pictures, err := NewPictureService(contexts, pictureServiceLookup(map[string]string{
		pictureReadingEnvironment: "true",
		pictureLibraryEnvironment: path,
	}))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPictureMetadataService(pictures, pictureServiceLookup(map[string]string{
		pictureMetadataWritingEnvironment: "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	review, err := pictures.GetMetadata(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	var original ProjectMetadataRow
	for _, row := range review.Records.Rows {
		if row.Cells[0].Integer != nil && *row.Cells[0].Integer == "12345" {
			original = row
		}
	}
	if original.RowID == "" {
		t.Fatal("explicit unambiguous fixture row missing")
	}
	id, err := strconv.ParseInt(*original.Cells[0].Integer, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	name := "literal-edited.jpg"
	request, err := json.Marshal(map[string]any{
		"requestId": "00000000-0000-4000-8000-000000000001",
		"id":        id,
		"columns":   review.Records.Columns,
		"original":  original,
		"changes": []ProjectMetadataChange{{
			Column: "PicName", Value: ProjectMetadataCell{Storage: "text", Text: &name},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt, err := service.LookupReceipt(context.Background(), state.ContextID, "108050", string(request)); err != nil || receipt != nil {
		t.Fatal("unsaved request acquired a fabricated receipt", receipt, err)
	}
	result, err := service.Save(context.Background(), state.ContextID, "108050", string(request))
	if err != nil || result == nil || !result.DidCommit || result.HistoryID == "" {
		t.Fatal("complete owned Save receipt unavailable", result, err)
	}
	receipt, err := service.LookupReceipt(context.Background(), state.ContextID, "108050", string(request))
	if err != nil || receipt == nil || receipt.HistoryID != result.HistoryID ||
		!reflect.DeepEqual(receipt.Committed, result.Committed) {
		t.Fatal("durable receipt did not match committed row", receipt, err)
	}
	replayed, err := service.Save(context.Background(), state.ContextID, "108050", string(request))
	if err != nil || replayed == nil || !replayed.Replayed || replayed.HistoryID != result.HistoryID {
		t.Fatal("stable retry wrote again or lost its history identity", replayed, err)
	}
	if receipt, err := service.LookupReceipt(context.Background(), "stale", "108050", string(request)); err == nil || receipt != nil {
		t.Fatal("foreign context acquired picture history", receipt, err)
	}
}
