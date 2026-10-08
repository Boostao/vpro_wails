package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSIVIDeletionSourceMembershipAndPhysicalIdentity(t *testing.T) {
	service, state, db, request := siviDeletionFixture(t, false, 3)
	veg, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string][]string{
		"SubVegA-SIVI":    {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB"},
		"SubVegA-SIVI_BC": {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB"},
		"SubVegC-SIVI":    {"Cover6"},
		"SubVegD-SIVI":    {"Cover7", "Cover8", "Cover9"},
	}
	count := 0
	for form, fields := range forms {
		for _, field := range fields {
			count++
			t.Run(form+"/"+field, func(t *testing.T) {
				row := request.Original
				row.RowID = "-9223372036854775808"
				row.Cells = append([]ProjectMetadataCell{}, row.Cells...)
				for _, name := range siviCreationColumns {
					if strings.HasPrefix(name, "Cover") || strings.HasPrefix(name, "Total") {
						row.Cells[columns[name]] = ProjectMetadataCell{Storage: "null"}
					}
				}
				row.Cells[columns[field]] = siviReal(0)
				original, err := siviDeletionSource(context.Background(), request.Project, request.Plot, form, row.RowID,
					ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{row}})
				if err != nil || original == nil || !reflect.DeepEqual(original.Original, row) {
					t.Fatal("zero/hidden membership lost complete physical original", original, err)
				}
				row.Cells[columns[field]] = ProjectMetadataCell{Storage: "null"}
				if original, err := siviDeletionSource(context.Background(), request.Project, request.Plot, form, row.RowID,
					ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{row}}); original != nil || err == nil {
					t.Fatal("nonmember became deletable", original, err)
				}
			})
		}
	}
	if count != 24 {
		t.Fatal("source predicate coverage changed", count)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, rowID := range []string{"10000001", "0", "01", "+1", "9223372036854775808"} {
		if original, err := service.readSIVIDeletionOriginal(context.Background(), state.ContextID, request.Plot, request.Form, rowID); original != nil || err == nil {
			t.Fatal("logical/missing/malformed physical identity accepted", rowID, original, err)
		}
	}
	for _, form := range []string{"SubVegA-XL", "subvega-sivi", ""} {
		if original, err := service.readSIVIDeletionOriginal(context.Background(), state.ContextID, request.Plot, form, request.Original.RowID); original != nil || err == nil {
			t.Fatal("unsupported form accepted", form, original, err)
		}
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIDeletionStrictTransport(t *testing.T) {
	_, _, _, request := siviDeletionFixture(t, false, 3)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip siviDeletionRequest
	if err := json.Unmarshal(raw, &roundtrip); err != nil || !reflect.DeepEqual(roundtrip, request) {
		t.Fatal("exact typed request transport failed", err)
	}
	text := string(raw)
	for _, broken := range []string{
		strings.Replace(text, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(text, `"requestId":`, `"requestId":"00000000-0000-4000-8000-000000000099","requestId":`, 1),
		strings.Replace(text, `"columns":`, `"allocatedId":1,"columns":`, 1),
		strings.Replace(text, `"columns":[`, `"columns":null,"discard":[`, 1),
		strings.Replace(text, `"name":"PlotNumber"`, `"name":"PlotNumber","name":"Species"`, 1),
		strings.Replace(text, `"declaredType":`, `"DeclaredType":`, 1),
		strings.Replace(text, `"storage":"text"`, `"storage":"text","storage":"text"`, 1),
		strings.Replace(text, `"RAW"`, `"\ud800"`, 1),
		strings.Replace(text, `"RAW"`, "\"\xff\"", 1),
		text + `{}`,
	} {
		if err := json.Unmarshal([]byte(broken), &roundtrip); err == nil {
			t.Fatal("malformed authority transport accepted", broken)
		}
	}
	for _, id := range []string{"creation-1", strings.ToUpper(request.RequestID), "00000000-0000-5000-8000-000000000001", "00000000-0000-4000-7000-000000000001"} {
		draft := request
		draft.RequestID = id
		if err := validateSIVIDeletionRequest(draft); err == nil {
			t.Fatal("non-lowercase UUIDv4 accepted", id)
		}
	}
}

func TestSIVIDeletionReadonlyUnresolvedAndCancellation(t *testing.T) {
	service, state, db, request := siviDeletionFixture(t, true, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for i := 0; i < 2; i++ {
		if receipt, err := service.lookupSIVIDeletionReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
			t.Fatal("missing history is unresolved", receipt, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE;
		CREATE TABLE "__VPRO_SIVIDeletionHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
		INSERT INTO "__VPRO_SIVIDeletionHistory" VALUES('pending','pending','not committed')`); err != nil {
		t.Fatal(err)
	}
	receipt, lookupErr := service.lookupSIVIDeletionReceipt(context.Background(), state.ContextID, request)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if receipt != nil || lookupErr != nil {
		t.Fatal("uncommitted history was observed", receipt, lookupErr)
	}
	assertProfileSUFiles(t, service, before)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if receipt, err := service.lookupSIVIDeletionReceipt(ctx, state.ContextID, request); receipt != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled lookup ignored cancellation", receipt, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	receipt, err = service.lookupSIVIDeletionReceipt(ctx, state.ContextID, request)
	cancel()
	owner.mu.Unlock()
	if receipt != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("held snapshot ignored cancellation", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	var technical int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('__VPRO_SIVIDeletionHistory','__VPRO_ChildIdentity')`).Scan(&technical); err != nil || technical != 0 {
		t.Fatal("read-only lookup recreated technical tables", technical, err)
	}
	if receipt, err := service.deleteSIVIVegetation(context.Background(), state.ContextID, request); receipt == nil || err != nil {
		t.Fatal(receipt, err)
	}
	unknown := request
	unknown.RequestID = "00000000-0000-4000-8000-000000000002"
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVIDeletionReceipt(context.Background(), state.ContextID, unknown); receipt != nil || err != nil {
		t.Fatal("unknown request was not unresolved", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIDeletionReopenAndFreshParentOwnership(t *testing.T) {
	service, state, db, request := siviDeletionFixture(t, true, 3)
	initial, err := service.deleteSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil || reopened.ContextID == state.ContextID {
		t.Fatal("same project did not explicitly reopen", reopened, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	expected := *initial
	expected.ContextID, expected.DidCommit, expected.Replayed = reopened.ContextID, false, true
	receipt, err := service.lookupSIVIDeletionReceipt(context.Background(), reopened.ContextID, request)
	if err != nil || !reflect.DeepEqual(receipt, &expected) || receipt.Request.ContextID != state.ContextID {
		t.Fatal("lookup did not preserve durable context and current caller", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	newRequest := request
	newRequest.RequestID = "00000000-0000-4000-8000-000000000002"
	if receipt, err := service.deleteSIVIVegetation(context.Background(), reopened.ContextID, newRequest); receipt != nil || err == nil {
		t.Fatal("old context acquired new mutation authority", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	mutateContextFixture(t, reopened.SUPath, `UPDATE Report_SU SET PlotNumber='outside' WHERE PlotNumber='108050'`)
	assertSIVIDeletionRefused(t, service, reopened.ContextID, request, true)
	mutateContextFixture(t, reopened.SUPath, `UPDATE Report_SU SET PlotNumber='108050' WHERE PlotNumber='outside'`)
	if _, err := db.Exec(`UPDATE Sample_Admin SET Plot='outside' WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRefused(t, service, reopened.ContextID, request, true)
	if _, err := db.Exec(`UPDATE Sample_Admin SET Plot='108050' WHERE Plot='outside';
		INSERT INTO Sample_Veg(rowid,PlotNumber,Species,ID,Cover1) VALUES(?, '108050','NEW',123,0)`, request.Original.RowID); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionRefused(t, service, reopened.ContextID, request, true)
	if request.Original.RowID == strconv.FormatInt(initial.ID, 10) {
		t.Fatal("fixture conflated physical and logical identities")
	}
}
