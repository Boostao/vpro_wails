package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func summaryFieldListOwnedFixture(t *testing.T) (*SummaryVegetationFieldListService, string) {
	t.Helper()
	contexts, state := contextServiceFixture(t)
	path := contexts.projects.sqlite.attachments["VPro64"]
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := db.Exec(`DROP TABLE IF EXISTS USysShortVegSummaryFieldList;` +
		strings.ReplaceAll(summaryFieldListTestSchema, "VPro64.", "") + `;
		INSERT INTO USysShortVegSummaryFieldList VALUES
		(1,'Env','duplicate','A',0,'sum','F1'),
		(1,'Env','duplicate','A',0,'sum','F1'),
		(-1,'Admin','hidden','','-1',NULL,'H'),
		(NULL,NULL,'NULL order',NULL,-1,NULL,NULL),
		(2,'Env',NULL,NULL,NULL,NULL,'F2')`)
	if err := errors.Join(writeErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	return NewSummaryVegetationFieldListService(contexts, true), state.ContextID
}

func summaryFieldListAssertOwnedUnchanged(t *testing.T, service *SummaryVegetationFieldListService, before map[string][]byte, config []byte) {
	t.Helper()
	if !reflect.DeepEqual(databaseBytes(t, service.contexts.projects.sqlite.attachments), before) {
		t.Fatal("field-list read changed database, audit, metadata or history bytes")
	}
	after, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(after, config) {
		t.Fatal("field-list read changed YAML configuration", err)
	}
}

func TestSummaryVegetationFieldListFacadeOwnedPhysicalAndWire(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	owner := service.contexts.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil || review == nil || review.ContextID != id || review.Project != owner.selection.Project ||
		review.ProjectPath != owner.selection.ProjectPath || review.SU != owner.selection.SU ||
		review.SUPath != owner.selection.SUPath || review.SourcePath != owner.attachments["VPro64"] ||
		review.Source != summaryVegetationFieldListSource || len(review.Physical.Rows) != 5 {
		t.Fatal("owned physical review provenance/shape changed", review, err)
	}
	independent, err := openReadOnly(review.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	var hiddenReport int
	readErr := independent.QueryRow(`SELECT COUNT(*) FROM USysShortVegSummaryFieldList`).Scan(&count)
	hiddenErr := independent.QueryRow(`SELECT Report FROM USysShortVegSummaryFieldList WHERE "Order"=-1`).Scan(&hiddenReport)
	if err := errors.Join(readErr, hiddenErr, independent.Close()); err != nil || count != 5 || hiddenReport != -1 {
		t.Fatal("independent physical reopen differs", count, hiddenReport, err)
	}
	if review.Physical.Rows[0].RowID != "1" || review.Physical.Rows[1].RowID != "2" ||
		!reflect.DeepEqual(review.Physical.Rows[0].Cells, review.Physical.Rows[1].Cells) ||
		!reflect.DeepEqual(review.Physical.Rows[2].Cells, summaryFieldListExpected(t,
			[]any{int64(-1), "Admin", "hidden", "", int64(-1), nil, "H"})) {
		t.Fatal("complete physical duplicates/NULL/empty hidden cells collapsed", review)
	}
	selection, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id)
	if err != nil || !reflect.DeepEqual(selection.Visible, []string{"1", "2", "5"}) ||
		!reflect.DeepEqual(selection.Selected, []string{"4", "3"}) || !reflect.DeepEqual(selection.Review, *review) {
		t.Fatal("owned source indexes/complete original differ", selection, err)
	}
	raw, err := json.Marshal(selection)
	var wire map[string]json.RawMessage
	if err != nil || json.Unmarshal(raw, &wire) != nil || len(wire) != 3 ||
		wire["review"] == nil || wire["visible"] == nil || wire["selected"] == nil {
		t.Fatal("source selection wire shape changed", string(raw), err)
	}
	*review.Physical.Rows[0].Cells[2].Text = "caller"
	review.Physical.Columns[0].DeclaredType = "caller"
	again, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil || *again.Physical.Rows[0].Cells[2].Text != "duplicate" ||
		again.Physical.Columns[0].DeclaredType != "INTEGER" {
		t.Fatal("owned reviews alias caller data", again, err)
	}
	summaryFieldListAssertOwnedUnchanged(t, service, before, config)
}

func TestSummaryVegetationFieldListFacadeGateAndRequestRefusals(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}} {
		enabled, err := summaryVegetationFieldListFeature(func(key string) (string, bool) {
			if key != "VPRO_SUMMARY_VEGETATION_FIELD_LIST" {
				t.Fatal(key)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
	for _, service := range []*SummaryVegetationFieldListService{nil, {}, NewSummaryVegetationFieldListService(nil, true)} {
		if got, err := service.GetSummaryVegetationFieldList(t.Context(), "id"); got != nil || err == nil {
			t.Fatal(got, err)
		}
		if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), "id"); got != nil || err == nil {
			t.Fatal(got, err)
		}
	}
	service, id := summaryFieldListOwnedFixture(t)
	for _, invalid := range []string{"", "stale", id + "\x00", string([]byte{0xff})} {
		if got, err := service.GetSummaryVegetationFieldList(t.Context(), invalid); got != nil || err == nil {
			t.Fatal("invalid context accepted physical read", got, err)
		}
		if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), invalid); got != nil || err == nil {
			t.Fatal("invalid context accepted source index", got, err)
		}
	}
	if got, err := service.GetSummaryVegetationFieldList(nil, id); got != nil || err == nil {
		t.Fatal("nil request context accepted", got, err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := service.GetSummaryVegetationFieldListIndex(cancelled, id); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled source index accepted", got, err)
	}
}

func TestSummaryVegetationFieldListFacadeCleanupAndCancellationRetry(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("field-list commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("field-list cleanup failure") }},
	} {
		service.snapshot = hooks
		if got, err := service.GetSummaryVegetationFieldList(t.Context(), id); got != nil || err == nil {
			t.Fatal("cleanup failure returned success-shaped physical result", got, err)
		}
		if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id); got != nil || err == nil {
			t.Fatal("cleanup failure returned success-shaped index result", got, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if got, err := service.GetSummaryVegetationFieldList(cancelled, id); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("in-flight cancelled read accepted", got, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id); got == nil || err != nil {
		t.Fatal("read cancellation/cleanup lost owned attachments", got, err)
	}
	summaryFieldListAssertOwnedUnchanged(t, service, before, config)
}

func TestSummaryVegetationFieldListFacadeUnownedAttachmentRefusesAndRestores(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	owner := service.contexts.projects.sqlite
	path, identity := owner.attachments["VPro64"], owner.attachmentInfo["VPro64"]
	owner.attachments["VPro64"] = owner.attachments["VLists"]
	if got, err := service.GetSummaryVegetationFieldList(t.Context(), id); got != nil || err == nil {
		t.Fatal("different physical attachment label accepted", got, err)
	}
	owner.attachments["VPro64"] = path
	owner.attachmentInfo["VPro64"] = owner.attachmentInfo["VLists"]
	if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id); got != nil || err == nil {
		t.Fatal("different opened attachment identity accepted", got, err)
	}
	owner.attachmentInfo["VPro64"] = identity
	if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id); got == nil || err != nil {
		t.Fatal("exact restored ownership did not retry", got, err)
	}
}

func TestSummaryVegetationFieldListFacadePhysicalAvailableDespiteIndexStorageRefusal(t *testing.T) {
	service, id := summaryFieldListOwnedFixture(t)
	path := service.contexts.projects.sqlite.attachments["VPro64"]
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	writeErr := func() error {
		_, err := db.Exec(`UPDATE USysShortVegSummaryFieldList SET Report=1 WHERE rowid=1`)
		return err
	}()
	if err := errors.Join(writeErr, db.Close()); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(service.contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil || read == nil || len(read.Physical.Rows) != 5 ||
		*read.Physical.Rows[0].Cells[4].Integer != "1" {
		t.Fatal("unsupported historical Boolean storage erased physical review", read, err)
	}
	if got, err := service.GetSummaryVegetationFieldListIndex(t.Context(), id); got != nil || err == nil ||
		!strings.Contains(err.Error(), "Report") {
		t.Fatal("unsupported historical Boolean source index was silently repaired", got, err)
	}
	again, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil || !reflect.DeepEqual(again, read) {
		t.Fatal("index refusal discarded physical access", again, err)
	}
	summaryFieldListAssertOwnedUnchanged(t, service, before, config)
}
