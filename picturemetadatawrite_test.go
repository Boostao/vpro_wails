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
	"strconv"
	"strings"
	"testing"
	"time"
)

const pictureMetadataWriteFixtureSQL = `CREATE TABLE tblVPics(ID,PicDir,PicName,PlotNumber,PicComment);
	INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) VALUES
	(9223372036854775806,2147483647,'Default','positive.jpg','108050',x'00ff'),
	(-9223372036854775808,-2147483648,NULL,NULL,'108050',3.5),
	(3,0,'','historical.jpg','108050',9223372036854775807),
	(4,9,'Default','foreign.jpg','108050x','keep'),
	(5,10,'Default','numeric.jpg',108050,NULL);`

func pictureMetadataWriteFixture(t *testing.T) (*ContextService, ProjectState, *ownedPictureLibrary, pictureMetadataEdit) {
	t.Helper()
	service, state := reportServiceFixture(t, false)
	source, _ := pictureLibraryFixture(t, pictureMetadataWriteFixtureSQL)
	if err := service.plots.SetCurrentUser(" picture actor "); err != nil {
		t.Fatal(err)
	}
	request := pictureMetadataWriteRequest(t, service, state, source, 2147483647)
	return service, state, source, request
}

func pictureMetadataWriteRequest(t *testing.T, service *ContextService, state ProjectState, source *ownedPictureLibrary, id int64) pictureMetadataEdit {
	t.Helper()
	review, err := service.readPictureMetadata(t.Context(), state.ContextID, "108050", source)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range review.Records.Rows {
		if row.Cells[0].Integer != nil && *row.Cells[0].Integer == strconv.FormatInt(id, 10) {
			return pictureMetadataEdit{RequestID: "request-" + strconv.FormatInt(id, 10), ID: id,
				Columns: review.Records.Columns, Original: row,
				Changes: []ProjectMetadataChange{{Column: "PicName", Value: metadataText("  literal edited.jpg  ")}}}
		}
	}
	t.Fatal("missing selected fixture ID", id)
	return pictureMetadataEdit{}
}

func pictureMetadataFixtureExec(t *testing.T, source *ownedPictureLibrary, statement string) {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(statement)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
}

func pictureMetadataHistoryRecords(t *testing.T, source *ownedPictureLibrary) []pictureMetadataHistory {
	t.Helper()
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT Proposal FROM "__VPRO_PictureMetadataHistory" ORDER BY ID`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []pictureMetadataHistory
	for rows.Next() {
		var proposal string
		var event pictureMetadataHistory
		if err := rows.Scan(&proposal); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(proposal), &event); err != nil {
			t.Fatal(err)
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPictureMetadataWriteSigned32Physical64AtomicAuditAndReceipt(t *testing.T) {
	service, state, source, _ := pictureMetadataWriteFixture(t)
	parentBytes := databaseBytes(t, service.projects.sqlite.attachments)
	for _, id := range []int64{2147483647, -2147483648, 0} {
		request := pictureMetadataWriteRequest(t, service, state, source, id)
		request.Changes = append(request.Changes, ProjectMetadataChange{Column: "PicDir", Value: metadataText("  literal external directory  ")})
		result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
		if err != nil {
			t.Fatal(err)
		}
		if !result.DidCommit || result.Replayed || result.HistoryID == "" || result.RequestID != request.RequestID ||
			result.Actor != " picture actor " || result.ContextID != state.ContextID || result.Project != "Sample" ||
			result.PlotNumber != "108050" || result.Source != source.path || result.RequestedSource != source.requestedPath ||
			len(result.OwnedFiles) == 0 || result.EditWhen == "" || result.ID != id ||
			!reflect.DeepEqual(result.Original, request.Original) || !reflect.DeepEqual(result.Columns, request.Columns) ||
			!reflect.DeepEqual(result.Committed.Cells[0], request.Original.Cells[0]) ||
			!reflect.DeepEqual(result.Committed.Cells[3:], request.Original.Cells[3:]) ||
			*result.Committed.Cells[1].Text != "  literal external directory  " || *result.Committed.Cells[2].Text != "  literal edited.jpg  " {
			t.Fatal("incomplete or repaired committed receipt", result)
		}
		history := pictureMetadataHistoryRecords(t, source)
		last := history[len(history)-1]
		if !reflect.DeepEqual(last.Request, request) || !reflect.DeepEqual(last.Result.Committed, result.Committed) ||
			!reflect.DeepEqual(last.Result.Original, result.Original) || last.Result.Actor != result.Actor {
			t.Fatal("tagged audit did not preserve complete original and committed cells", last)
		}
		pictureBytes := databaseBytes(t, map[string]string{"pictures": source.path})
		replayed, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
		if err != nil || !replayed.Replayed || !replayed.DidCommit || replayed.HistoryID != result.HistoryID ||
			!reflect.DeepEqual(replayed.Committed, result.Committed) {
			t.Fatal("lost-receipt retry did not return the independently verified durable receipt", replayed, err)
		}
		if !reflect.DeepEqual(pictureBytes, databaseBytes(t, map[string]string{"pictures": source.path})) {
			t.Fatal("receipt replay repeated a mutation/history")
		}
		collision := request
		collision.Changes = []ProjectMetadataChange{{Column: "PicDir", Value: metadataText("collision")}}
		if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, collision); err == nil || !reflect.DeepEqual(got, pictureMetadataWriteResult{}) {
			t.Fatal("request identity collision accepted", got, err)
		}
	}
	assertProfileSUFiles(t, service, parentBytes)
}

func TestPictureMetadataWriteNullEmptyAndUnchangedInvalidOmission(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	for _, historical := range []string{"NULL", "''", "123", "x'ff00'", "'" + strings.Repeat("x", 300) + "'"} {
		t.Run(historical[:min(12, len(historical))], func(t *testing.T) {
			pictureMetadataFixtureExec(t, source, `UPDATE tblVPics SET PicDir=`+historical+`,PicName='' WHERE ID=2147483647`)
			request = pictureMetadataWriteRequest(t, service, state, source, 2147483647)
			request.RequestID = "unchanged-" + historical[:min(12, len(historical))]
			request.Changes = []ProjectMetadataChange{
				{Column: "PicDir", Value: request.Original.Cells[1]},
				{Column: "PicName", Value: request.Original.Cells[2]},
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
			if err != nil || result.DidCommit || len(result.Changes) != 0 || !reflect.DeepEqual(result.Committed, request.Original) {
				t.Fatal("unchanged historical invalid assignment was not omitted", result, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("unchanged request created an audit or changed data")
			}
			request.Changes[1].Value = metadataText("valid.jpg")
			result, err = service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
			if err != nil || !result.DidCommit || len(result.Changes) != 1 ||
				!reflect.DeepEqual(result.Committed.Cells[1], request.Original.Cells[1]) ||
				!reflect.DeepEqual(result.Committed.Cells[4], request.Original.Cells[4]) {
				t.Fatal("sibling edit rewrote invalid historical storage", result, err)
			}
		})
	}
	for i, cell := range []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataText("Default")} {
		request = pictureMetadataWriteRequest(t, service, state, source, 2147483647)
		request.RequestID = "nullable-directory-" + strconv.Itoa(i)
		request.Changes = []ProjectMetadataChange{{Column: "PicDir", Value: cell}, {Column: "PicName", Value: ProjectMetadataCell{Storage: "null"}}}
		result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
		if err != nil || !reflect.DeepEqual(result.Committed.Cells[1], cell) || result.Committed.Cells[2].Storage != "null" {
			t.Fatal("NULL and empty were collapsed", result, err)
		}
	}
	pictureMetadataFixtureExec(t, source, `UPDATE tblVPics SET PicName='`+strings.Repeat("x", 300)+`' WHERE ID=2147483647`)
	request = pictureMetadataWriteRequest(t, service, state, source, 2147483647)
	request.RequestID = "unchanged-overlength-name"
	request.Changes = []ProjectMetadataChange{{Column: "PicName", Value: request.Original.Cells[2]}, {Column: "PicDir", Value: metadataText("")}}
	result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
	if err != nil || !reflect.DeepEqual(result.Committed.Cells[2], request.Original.Cells[2]) || len(result.Changes) != 1 {
		t.Fatal("unchanged overlength PicName prevented a valid sibling edit or was repaired", result, err)
	}
}

func TestPictureMetadataWriteRefusesStaleAmbiguousNonintegerAndForeignRows(t *testing.T) {
	for name, modify := range map[string]string{
		"duplicate-owned":   `INSERT INTO tblVPics VALUES(2147483647,'Default','duplicate.jpg','108050',NULL)`,
		"duplicate-foreign": `INSERT INTO tblVPics VALUES(2147483647,'Default','duplicate.jpg','other',NULL)`,
		"text-alias":        `INSERT INTO tblVPics VALUES('2147483647','Default','duplicate.jpg','other',NULL)`,
		"real-alias":        `INSERT INTO tblVPics VALUES(2147483647.0,'Default','duplicate.jpg','other',NULL)`,
		"noninteger":        `UPDATE tblVPics SET ID='2147483647' WHERE ID=2147483647`,
		"foreign-parent":    `UPDATE tblVPics SET PlotNumber='108050x' WHERE ID=2147483647`,
		"numeric-parent":    `UPDATE tblVPics SET PlotNumber=108050 WHERE ID=2147483647`,
		"rowid":             `UPDATE tblVPics SET rowid=100 WHERE ID=2147483647`,
		"stale-comment":     `UPDATE tblVPics SET PicComment='' WHERE ID=2147483647`,
		"stale-sibling":     `UPDATE tblVPics SET PicDir='changed' WHERE ID=2147483647`,
		"schema-extra":      `ALTER TABLE tblVPics ADD COLUMN Extra`,
		"schema-type":       `ALTER TABLE tblVPics RENAME TO old; CREATE TABLE tblVPics(ID INTEGER,PicDir,PicName,PlotNumber,PicComment); INSERT INTO tblVPics(rowid,ID,PicDir,PicName,PlotNumber,PicComment) SELECT rowid,* FROM old`,
		"source-trigger":    `CREATE TRIGGER surprise AFTER UPDATE ON tblVPics BEGIN DELETE FROM tblVPics WHERE ID=9; END`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, source, request := pictureMetadataWriteFixture(t)
			pictureMetadataFixtureExec(t, source, modify)
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
			if err == nil || !reflect.DeepEqual(got, pictureMetadataWriteResult{}) {
				t.Fatal("ambiguous, stale, repaired or nonmember edit accepted", got, err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("rejected edit changed source or audit")
			}
			if name == "duplicate-owned" {
				review, err := service.readPictureMetadata(t.Context(), state.ContextID, "108050", source)
				if err != nil || len(review.Records.Rows) != 4 {
					t.Fatal("writer narrowed historical duplicate read behavior", review, err)
				}
			}
		})
	}
}

func TestPictureMetadataWriteStrictJSONAndNewValuePolicy(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	valid, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pictureMetadataEdit
	if err := json.Unmarshal(valid, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("strict valid request failed", err)
	}
	invalidJSON := [][]byte{
		bytes.Replace(valid, []byte(`"requestId":`), []byte(`"requestId":"duplicate","requestId":`), 1),
		bytes.Replace(valid, []byte(`"id":2147483647`), []byte(`"id":null`), 1),
		bytes.Replace(valid, []byte(`"id":2147483647`), []byte(`"id":2147483648`), 1),
		bytes.Replace(valid, []byte(`"requestId":`), []byte(`"unknown":`), 1),
		bytes.Replace(valid, []byte(`"rowId":`), []byte(`"rowId":"3","rowId":`), 1),
		bytes.Replace(valid, []byte(`"declaredType":""`), []byte(`"declaredType":"","name":"ID"`), 1),
		bytes.Replace(valid, []byte(`"storage":"text"`), []byte(`"storage":"text","unknown":1`), 1),
		bytes.Replace(valid, []byte(`"column":"PicName"`), []byte(`"column":"PicName","column":"PicDir"`), 1),
		bytes.Replace(valid, []byte(`"  literal edited.jpg  "`), []byte(`"\ud800"`), 1),
		bytes.Replace(valid, []byte(`"  literal edited.jpg  "`), []byte{'"', 0xff, '"'}, 1),
		append(append([]byte{}, valid...), []byte(` {}`)...),
		[]byte(`null`),
		[]byte(`{}`),
	}
	for i, raw := range invalidJSON {
		if err := json.Unmarshal(raw, &decoded); err == nil {
			t.Fatal("malformed/unknown/duplicate/raw Unicode JSON accepted", i, string(raw))
		}
	}
	for name, change := range map[string]ProjectMetadataChange{
		"ID":       {Column: "ID", Value: metadataInteger("1")},
		"parent":   {Column: "PlotNumber", Value: metadataText("108050x")},
		"comment":  {Column: "PicComment", Value: metadataText("forbidden")},
		"recased":  {Column: "picName", Value: metadataText("new.jpg")},
		"integer":  {Column: "PicName", Value: metadataInteger("1")},
		"empty":    {Column: "PicName", Value: metadataText("")},
		"overlong": {Column: "PicName", Value: metadataText(strings.Repeat("x", 256))},
		"utf16":    {Column: "PicDir", Value: metadataText(strings.Repeat("😀", 128))},
		"NUL":      {Column: "PicDir", Value: metadataText("a\x00b")},
		"unicode":  {Column: "PicDir", Value: metadataText(string([]byte{0xff}))},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := request
			sibling := ProjectMetadataChange{Column: "PicDir", Value: metadataText("must rollback")}
			if change.Column == "PicDir" {
				sibling = ProjectMetadataChange{Column: "PicName", Value: metadataText("must rollback.jpg")}
			}
			candidate.Changes = []ProjectMetadataChange{sibling, change}
			if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, candidate); err == nil {
				t.Fatal("new value policy accepted unavailable or invalid value")
			}
			for _, mutate := range []func(*pictureMetadataEdit){
				func(r *pictureMetadataEdit) { r.ID = -2147483649 },
				func(r *pictureMetadataEdit) { r.ID = 2147483648 },
				func(r *pictureMetadataEdit) { r.Original.RowID = "09223372036854775806" },
				func(r *pictureMetadataEdit) { r.Original.RowID = "9223372036854775808" },
				func(r *pictureMetadataEdit) { r.Original.Cells = nil },
				func(r *pictureMetadataEdit) { r.Changes = nil },
				func(r *pictureMetadataEdit) { r.RequestID = "" },
				func(r *pictureMetadataEdit) { r.RequestID = "invalid\x00identity" },
				func(r *pictureMetadataEdit) {
					r.Changes = []ProjectMetadataChange{r.Changes[0], r.Changes[0]}
				},
			} {
				var candidate pictureMetadataEdit
				if err := json.Unmarshal(valid, &candidate); err != nil {
					t.Fatal(err)
				}
				mutate(&candidate)
				if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, candidate); err == nil {
					t.Fatal("invalid authority/default identity accepted", candidate)
				}
			}
			raw, _ := json.Marshal(candidate)
			if name != "unicode" && json.Unmarshal(raw, &decoded) == nil {
				t.Fatal("strict request accepted invalid new value", string(raw))
			}
		})
	}
	request.Changes = []ProjectMetadataChange{{Column: "PicDir", Value: metadataText(strings.Repeat("😀", 127) + "x")}}
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err != nil {
		t.Fatal("255 UTF16-unit boundary rejected", err)
	}
}

func TestPictureMetadataWriteAuditFailureRollbackAndRetry(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	pictureMetadataFixtureExec(t, source, `CREATE TABLE "__VPRO_PictureMetadataHistory"(wrong TEXT)`)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	request.Changes = append(request.Changes, ProjectMetadataChange{Column: "PicDir", Value: metadataText("")})
	if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil || got.DidCommit {
		t.Fatal("audit failure reported committed success", got, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("audit failure changed source")
	}
	pictureMetadataFixtureExec(t, source, `DROP TABLE "__VPRO_PictureMetadataHistory"`)
	if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err != nil || !got.DidCommit {
		t.Fatal("rolled-back request could not retry", got, err)
	}
	pictureMetadataFixtureExec(t, source, `UPDATE tblVPics SET PicName='subsequent.jpg' WHERE ID=2147483647`)
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil {
		t.Fatal("old receipt replay ignored later row changes")
	}
}

func TestPictureMetadataWriteRefusesMalformedLostReceiptHistory(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(pictureMetadataHistoryRecords(t, source)[0])
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, corrupt := range [][]byte{
		bytes.Replace(valid, []byte(`"didCommit":true`), []byte(`"didCommit":false,"didCommit":true`), 1),
		bytes.Replace(valid, []byte(`"actor":`), []byte(`"unknown":`), 1),
		bytes.Replace(valid, []byte(`"editWhen":"`+result.EditWhen+`"`), []byte(`"editWhen":"\ud800"`), 1),
		bytes.Replace(valid, []byte(`"committed":`), []byte(`"committed":null,"unknown":`), 1),
	} {
		if _, err := db.Exec(`UPDATE "__VPRO_PictureMetadataHistory" SET Proposal=?`, string(corrupt)); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, map[string]string{"pictures": source.path})
		if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil || got.DidCommit {
			t.Fatal("malformed history was decoder-repaired or defaulted into success", got, err)
		}
		if got, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, request); err == nil || got != nil {
			t.Fatal("malformed history resolved an unknown save", got, err)
		}
		if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
			t.Fatal("malformed receipt was overwritten")
		}
	}
}

func TestPictureMetadataWriteRefusesInconsistentResultRequestIdentity(t *testing.T) {
	for name, identity := range map[string]string{"empty": "", "different": "another-request"} {
		t.Run(name, func(t *testing.T) {
			service, state, source, request := pictureMetadataWriteFixture(t)
			if result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err != nil || !result.DidCommit {
				t.Fatal("could not establish committed history", err)
			}
			history := pictureMetadataHistoryRecords(t, source)[0]
			history.Result.RequestID = identity
			proposal, err := json.Marshal(history)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`UPDATE "__VPRO_PictureMetadataHistory" SET Proposal=?`, string(proposal))
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
			if err == nil || !reflect.DeepEqual(result, pictureMetadataWriteResult{}) {
				t.Errorf("edit replay accepted inconsistent result request identity %q: error=%v, committed=%v", result.RequestID, err, result.DidCommit)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Error("rejected edit replay modified library data or history")
			}
			receipt, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, request)
			if err == nil || receipt != nil {
				t.Errorf("read-only lookup accepted inconsistent result request identity %q: error=%v, receiptPresent=%v", identity, err, receipt != nil)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Error("rejected receipt lookup modified library data or history")
			}
		})
	}
}

func TestPictureMetadataWriteReadOnlyReceiptLookupNeverInfersCommit(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	if receipt, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, request); err != nil || receipt != nil {
		t.Fatal("absent history returned an inferred successful receipt", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("receipt lookup created history or mutated data")
	}
	result, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request)
	if err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, map[string]string{"pictures": source.path})
	receipt, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, request)
	if err != nil || receipt == nil || !receipt.Replayed || !receipt.DidCommit || receipt.HistoryID != result.HistoryID ||
		!reflect.DeepEqual(receipt.Original, request.Original) || !reflect.DeepEqual(receipt.Committed, result.Committed) {
		t.Fatal("durable verified history could not resolve the save", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("receipt lookup wrote source")
	}
	unknownRequest := request
	unknownRequest.RequestID = "different-unknown-save"
	if receipt, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, unknownRequest); err != nil || receipt != nil {
		t.Fatal("matching current values were mistaken for this request's commit", receipt, err)
	}
	pictureMetadataFixtureExec(t, source, `DELETE FROM "__VPRO_PictureMetadataHistory"`)
	before = databaseBytes(t, map[string]string{"pictures": source.path})
	if receipt, err := service.lookupPictureMetadataReceipt(t.Context(), state.ContextID, "108050", source, request); err != nil || receipt != nil {
		t.Fatal("lost history was guessed from matching stored values", receipt, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("lost receipt lookup recreated a successful event")
	}
	for _, contextID := range []string{"", "stale"} {
		if receipt, err := service.lookupPictureMetadataReceipt(t.Context(), contextID, "108050", source, request); err == nil || receipt != nil {
			t.Fatal("stale context received a receipt", receipt, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if receipt, err := service.lookupPictureMetadataReceipt(cancelled, state.ContextID, "108050", source, request); !errors.Is(err, context.Canceled) || receipt != nil {
		t.Fatal("cancelled receipt lookup returned a successful projection", receipt, err)
	}
}

func TestPictureMetadataWriteFinalPhysicalVerificationAndOwnershipRollback(t *testing.T) {
	service, state, _, _ := pictureMetadataWriteFixture(t)
	source, _ := pictureLibraryFixture(t, `CREATE TABLE tblVPics(ID,PicDir NUMERIC,PicName,PlotNumber,PicComment);
		INSERT INTO tblVPics VALUES(1,'Default','original.jpg','108050',NULL)`)
	request := pictureMetadataWriteRequest(t, service, state, source, 1)
	request.Changes = []ProjectMetadataChange{{Column: "PicDir", Value: metadataText("001")}, {Column: "PicName", Value: metadataText("new.jpg")}}
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil || got.DidCommit ||
		!strings.Contains(err.Error(), "complete plan") {
		t.Fatal("SQLite affinity repaired the explicit text assignment", got, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("failed physical verification did not roll back both fields")
	}
	request.Changes[0].Value = metadataText("literal-directory")
	rowID, changes, err := preparePictureMetadataEdit(request)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := pictureLibraryFixture(t, pictureMetadataWriteFixtureSQL)
	originalInfo := source.info
	validations := 0
	got, err := writePictureMetadata(t.Context(), source, request, rowID, changes,
		pictureMetadataWriteResult{RequestID: request.RequestID, PlotNumber: "108050"}, func() error {
			validations++
			if validations == 2 {
				source.info = other.info
			}
			return source.checkFile()
		})
	source.info = originalInfo
	if err == nil || got.DidCommit || validations != 2 || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("post-mutation identity replacement inherited authorization", got, err, validations)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("ownership failure left mutation/history")
	}
	if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err != nil || !got.DidCommit {
		t.Fatal("ownership rollback prevented safe retry", got, err)
	}
}

func TestPictureMetadataWriteCancellationCollisionRollbackAndRetry(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	before := databaseBytes(t, map[string]string{"pictures": source.path})
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.editPictureMetadata(cancelled, state.ContextID, "108050", source, request); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-cancelled edit was not cancelled", err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	waiting, stop := context.WithTimeout(t.Context(), 30*time.Millisecond)
	_, err := service.editPictureMetadata(waiting, state.ContextID, "108050", source, request)
	stop()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("ownership lease ignored cancellation", err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rw")+"&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	blocked, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	got, err := service.editPictureMetadata(blocked, state.ContextID, "108050", source, request)
	stop()
	if got.DidCommit || err == nil {
		t.Fatal("SQLite write collision/cancellation accepted", got, err)
	}
	if err := errors.Join(blocker.Rollback(), db.Close()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("cancelled collision created data/audit")
	}
	rowID, changes, err := preparePictureMetadataEdit(request)
	if err != nil {
		t.Fatal(err)
	}
	inFlight, stop := context.WithCancel(t.Context())
	validations := 0
	got, err = writePictureMetadata(inFlight, source, request, rowID, changes,
		pictureMetadataWriteResult{RequestID: request.RequestID, PlotNumber: "108050"}, func() error {
			validations++
			if validations == 2 {
				stop()
			}
			return nil
		})
	stop()
	if !errors.Is(err, context.Canceled) || got.DidCommit || validations != 2 {
		t.Fatal("precommit cancellation did not roll back mutation and new audit", got, err, validations)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
		t.Fatal("post-mutation cancellation failed atomic rollback")
	}
	if got, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err != nil || !got.DidCommit {
		t.Fatal("cancelled request could not retry", got, err)
	}
}

func TestPictureMetadataWriteOwnershipAliasesReplacementAndNoCreate(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	for _, test := range []struct{ contextID, plot string }{
		{"", "108050"}, {"stale", "108050"}, {state.ContextID, "108050 "},
		{state.ContextID, "missing"}, {state.ContextID, ""}, {state.ContextID, "108050\x00"},
	} {
		if _, err := service.editPictureMetadata(t.Context(), test.contextID, test.plot, source, request); err == nil {
			t.Fatal("stale/nonliteral/outside owner accepted", test)
		}
	}
}

func TestPictureMetadataWriteFreshWALParentAndSUObservations(t *testing.T) {
	for _, role := range []string{"project", "su"} {
		t.Run(role, func(t *testing.T) {
			service, state := reportServiceFixture(t, true)
			source, _ := pictureLibraryFixture(t, pictureMetadataWriteFixtureSQL)
			request := pictureMetadataWriteRequest(t, service, state, source, 2147483647)
			owner := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var journal string
			if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
				t.Fatal("could not establish independent WAL fixture", journal, err)
			}
			before := databaseBytes(t, map[string]string{"pictures": source.path})
			_, err = withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (struct{}, error) {
				return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, snapshot *sql.Tx) (struct{}, error) {
					if err := requirePictureParent(t.Context(), owner, snapshot, "108050"); err != nil {
						t.Fatal(err)
					}
					statement := `DELETE FROM Sample_Admin WHERE Plot='108050'`
					if role == "su" {
						statement = `DELETE FROM Report_SU WHERE PlotNumber='108050'`
					}
					if _, err := db.Exec(statement); err != nil {
						t.Fatal("independent WAL edit did not commit", err)
					}
					if err := requirePictureParent(t.Context(), owner, snapshot, "108050"); err != nil {
						t.Fatal("fixture failed to reproduce stale parent membership in the old snapshot", err)
					}
					return withPictureMetadataParentGuards(t.Context(), owner, "108050", func(validate func() error) (struct{}, error) {
						t.Fatal("stale WAL parent reached the library mutation boundary")
						return struct{}{}, validate()
					})
				})
			})
			if err == nil || !strings.Contains(err.Error(), "independently reserved") {
				t.Fatal("fresh reserved scope inherited the old WAL snapshot", err)
			}
			if !reflect.DeepEqual(before, databaseBytes(t, map[string]string{"pictures": source.path})) {
				t.Fatal("independent parent loss wrote picture metadata or history")
			}
			if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil {
				t.Fatal("removed parent later authorized a picture edit")
			}
		})
	}
}

func TestPictureMetadataWriteParentReservationsBlockIndependentWALWriters(t *testing.T) {
	service, state := reportServiceFixture(t, true)
	source, _ := pictureLibraryFixture(t, pictureMetadataWriteFixtureSQL)
	request := pictureMetadataWriteRequest(t, service, state, source, 2147483647)
	owner := service.projects.sqlite
	connections := map[string]*sql.DB{}
	for _, role := range []string{"project", "su"} {
		db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw")+"&_busy_timeout=0&_txlock=immediate")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var journal string
		if err := db.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&journal); err != nil || journal != "wal" {
			t.Fatal(journal, err)
		}
		connections[role] = db
	}
	rowID, changes, err := preparePictureMetadataEdit(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := withContextPlotRequest(t.Context(), service, state.ContextID, func(plots *PlotService) (pictureMetadataWriteResult, error) {
		return withOwnedSIVISnapshot(t.Context(), plots, func(owner *sqliteContext, snapshot *sql.Tx) (pictureMetadataWriteResult, error) {
			if err := requirePictureParent(t.Context(), owner, snapshot, "108050"); err != nil {
				return pictureMetadataWriteResult{}, err
			}
			return withPictureMetadataParentGuards(t.Context(), owner, "108050", func(validate func() error) (pictureMetadataWriteResult, error) {
				checkLocked := func() {
					t.Helper()
					for role, db := range connections {
						tx, err := db.BeginTx(t.Context(), nil)
						if err == nil {
							tx.Rollback()
							t.Fatal("independent writer acquired reserved parent file", role)
						}
						if !strings.Contains(err.Error(), "locked") {
							t.Fatal("unexpected independent writer failure", role, err)
						}
					}
				}
				checkLocked()
				result, err := writePictureMetadata(t.Context(), source, request, rowID, changes,
					pictureMetadataWriteResult{RequestID: request.RequestID, PlotNumber: "108050"}, validate)
				if err != nil {
					return result, err
				}
				checkLocked()
				return result, validate()
			})
		})
	})
	if err != nil || !result.DidCommit {
		t.Fatal("guarded library transaction failed", result, err)
	}
	for role, db := range connections {
		tx, err := db.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatal("parent reservation leaked after library commit", role, err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPictureMetadataWriteSourceAliasesReplacementAndNoCreate(t *testing.T) {
	service, state, source, request := pictureMetadataWriteFixture(t)
	for role, path := range service.projects.sqlite.attachments {
		t.Run("alias-"+role, func(t *testing.T) {
			alias, err := newOwnedPictureLibrary(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", alias, request); err == nil || !strings.Contains(err.Error(), "aliases") {
				t.Fatal("context file promoted to picture writable library", err)
			}
			link := filepath.Join(filepath.Dir(source.path), role+"-hardlink.sqlite")
			if err := os.Link(path, link); err != nil {
				t.Fatal(err)
			}
			alias, err = newOwnedPictureLibrary(link)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", alias, request); err == nil || !strings.Contains(err.Error(), "aliases") {
				t.Fatal("hardlink alias authorized", err)
			}
		})
	}
	service.projects.supportPaths["unattached-test"] = source.path
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatal("unattached support file alias authorized", err)
	}
	delete(service.projects.supportPaths, "unattached-test")
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", nil, request); err == nil {
		t.Fatal("nil optional source authorized")
	}
	if err := os.Rename(source.path, source.path+".owned"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil {
		t.Fatal("missing owned source was created")
	}
	if _, err := os.Stat(source.path); !os.IsNotExist(err) {
		t.Fatal("rw writer created missing source", err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(pictureMetadataWriteFixtureSQL)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.editPictureMetadata(t.Context(), state.ContextID, "108050", source, request); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("replacement file inherited authorization", err)
	}
	cause := errors.New("cleanup failure")
	committedErr := &pictureMetadataCommittedError{cause}
	if !errors.Is(committedErr, cause) || !strings.Contains(committedErr.Error(), "committed") {
		t.Fatal("committed cleanup error lost distinct receipt semantics")
	}
}
