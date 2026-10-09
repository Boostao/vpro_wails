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

func assertSIVIDeletionHistoryRefused(t *testing.T, contexts *ContextService, contextID, plot string) {
	t.Helper()
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	service := siviDeletionServiceFixture(t, contexts, false, true)
	if list, err := service.GetHistory(context.Background(), contextID, plot); list != nil || err == nil {
		t.Fatal("corrupt/unowned history became an empty or partial successful list", list, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVIDeletionHistoryMissingEmptyUncommittedAndStrictListShape(t *testing.T) {
	contexts, state, db, deletion := siviDeletionFixture(t, true, 3)
	service := siviDeletionServiceFixture(t, contexts, false, true)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	var list *SIVIDeletionHistoryList
	var err error
	list, err = service.GetHistory(context.Background(), state.ContextID, deletion.Plot)
	if err != nil || list == nil || list.HistoryPresent || list.Events == nil || len(list.Events) != 0 ||
		list.ContextID != state.ContextID || list.Project != deletion.Project || list.Plot != deletion.Plot {
		t.Fatal("missing history not distinguished from an empty existing table", list, err)
	}
	assertProfileSUFiles(t, contexts, before)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE;
		CREATE TABLE "__VPRO_SIVIDeletionHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
		INSERT INTO "__VPRO_SIVIDeletionHistory" VALUES('uncommitted','invalid','invalid')`); err != nil {
		t.Fatal(err)
	}
	uncommitted, readErr := service.GetHistory(context.Background(), state.ContextID, deletion.Plot)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || !reflect.DeepEqual(uncommitted, list) {
		t.Fatal("read-only list observed uncommitted history", uncommitted, readErr)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`CREATE TABLE "__VPRO_SIVIDeletionHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	list, err = service.GetHistory(context.Background(), state.ContextID, deletion.Plot)
	if err != nil || list == nil || !list.HistoryPresent || list.Events == nil || len(list.Events) != 0 {
		t.Fatal("existing empty history not explicit", list, err)
	}
	raw := siviDeletionServiceJSON(t, list)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	keys := []string{"contextId", "project", "plot", "historyPresent", "events"}
	if len(fields) != len(keys) || string(fields["events"]) != "[]" {
		t.Fatal("history list wire shape has implicit fields or NULL events", raw)
	}
	for _, key := range keys {
		if fields[key] == nil {
			t.Fatal("missing lowerCamel list property", key)
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVIDeletionHistoryDurableReopenSelectionAndConsumption(t *testing.T) {
	for _, action := range []AuditRestoreAction{AuditRestoreRetain, AuditRestorePrune} {
		contexts, state, db, restoration, deleted := siviDeletionRestorationFixture(t, true, 3)
		reopened, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
		if err != nil {
			t.Fatal(err)
		}
		service := siviDeletionServiceFixture(t, contexts, false, true)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		list, err := service.GetHistory(context.Background(), reopened.ContextID, restoration.Plot)
		if err != nil || list == nil || !list.HistoryPresent || list.ContextID != reopened.ContextID || len(list.Events) != 1 {
			t.Fatal("fresh facade/reopened owner could not discover persisted history without a UUID", list, err)
		}
		var event SIVIDeletionHistoryEvent = list.Events[0]
		if event.HistoryID != deleted.HistoryID || event.Form != deleted.Form || event.RowID != deleted.RowID ||
			event.ID != deleted.ID || event.Species == nil || *event.Species != "RAW" || event.Actor != deleted.Actor ||
			event.EditWhen != deleted.EditWhen || event.Restored || event.Consumed {
			t.Fatal("raw historical summary differs from durable deletion", event)
		}
		raw := siviDeletionServiceJSON(t, event)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &fields); err != nil {
			t.Fatal(err)
		}
		keys := []string{"historyId", "form", "rowId", "id", "species", "actor", "editWhen", "restored", "consumed"}
		if len(fields) != len(keys) {
			t.Fatal("summary shape contains implicit/full-row authority", raw)
		}
		for _, key := range keys {
			if fields[key] == nil {
				t.Fatal("missing lowerCamel summary property", key)
			}
		}
		assertProfileSUFiles(t, contexts, before)
		review, err := service.ReviewRestoration(context.Background(), reopened.ContextID, list.Plot, event.HistoryID)
		if err != nil || review == nil || !reflect.DeepEqual(review.Original, deleted.Original) {
			t.Fatal("explicit historical selection did not acquire a fresh full44 CAS review", review, err)
		}
		restoration.ContextID, restoration.Expected, restoration.Action = reopened.ContextID, review.Expected, action
		if receipt, err := service.Restore(context.Background(), reopened.ContextID, siviDeletionServiceJSON(t, restoration)); receipt == nil || err != nil {
			t.Fatal(receipt, err)
		}
		before = databaseBytes(t, contexts.projects.sqlite.attachments)
		consumed, err := service.GetHistory(context.Background(), reopened.ContextID, restoration.Plot)
		if err != nil || consumed == nil || len(consumed.Events) != 1 || !consumed.Events[0].Consumed ||
			!consumed.Events[0].Restored || consumed.Events[0].HistoryID != event.HistoryID {
			t.Fatal("restored history was hidden or still presented as unconsumed", consumed, err)
		}
		assertProfileSUFiles(t, contexts, before)
		if review, err := service.ReviewRestoration(context.Background(), reopened.ContextID, list.Plot, event.HistoryID); review != nil || err == nil {
			t.Fatal("history listing bypassed one-winner consumption", review, err)
		}
		if _, err := db.Exec(`UPDATE Sample_Veg SET Species='later' WHERE rowid=?`, deleted.RowID); err != nil {
			t.Fatal(err)
		}
		before = databaseBytes(t, contexts.projects.sqlite.attachments)
		later, err := service.GetHistory(context.Background(), reopened.ContextID, restoration.Plot)
		if err != nil || !reflect.DeepEqual(later, consumed) {
			t.Fatal("historical restoration fact was confused with current-row eligibility", later, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestSIVIDeletionHistoryRawNullableSpeciesAndNoPartialUnsupportedSuccess(t *testing.T) {
	contexts, state, db, request, _ := siviDeletionRestorationFixture(t, false, 0)
	service := siviDeletionServiceFixture(t, contexts, false, true)
	var original string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, request.HistoryID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for _, cell := range []ProjectMetadataCell{
		{Storage: "null"}, metadataText(""), metadataText(" Raw Historical overlength species "),
	} {
		var history siviDeletionHistory
		if err := json.Unmarshal([]byte(original), &history); err != nil {
			t.Fatal(err)
		}
		columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: history.Columns}, "Species")
		if err != nil {
			t.Fatal(err)
		}
		history.Original.Cells[columns["Species"]], history.Request.Original.Cells[columns["Species"]] = cell, cell
		if err := validateSIVIDeletionHistory(context.Background(), history); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=? WHERE RequestID=?`,
			siviDeletionServiceJSON(t, history), request.HistoryID); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		list, err := service.GetHistory(context.Background(), state.ContextID, request.Plot)
		if err != nil || list == nil || len(list.Events) != 1 || !reflect.DeepEqual(list.Events[0].Species, cell.Text) {
			t.Fatal("historical Species NULL/empty/literal was coerced, truncated or hidden", list, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
	var invalid siviDeletionHistory
	if err := json.Unmarshal([]byte(original), &invalid); err != nil {
		t.Fatal(err)
	}
	columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: invalid.Columns}, "Species")
	if err != nil {
		t.Fatal(err)
	}
	invalid.Original.Cells[columns["Species"]] = metadataInteger("123")
	invalid.Request.Original.Cells[columns["Species"]] = metadataInteger("123")
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=?`, siviDeletionServiceJSON(t, invalid)); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionHistoryRefused(t, contexts, state.ContextID, request.Plot)
}

func TestSIVIDeletionHistoryAllEventsStableOrderAndLiteralPlotFilter(t *testing.T) {
	contexts, state, db, request, _ := siviDeletionRestorationFixture(t, false, 0)
	var original string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory"`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	insert := func(id, plot, when string) {
		t.Helper()
		var history siviDeletionHistory
		if err := json.Unmarshal([]byte(original), &history); err != nil {
			t.Fatal(err)
		}
		history.Request.RequestID, history.Result.RequestID, history.Result.HistoryID = id, id, id
		history.Request.Plot, history.Result.Plot, history.When = plot, plot, when
		columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: history.Columns}, "PlotNumber")
		if err != nil {
			t.Fatal(err)
		}
		history.Request.Original.Cells[columns["PlotNumber"]] = metadataText(plot)
		history.Original.Cells[columns["PlotNumber"]] = metadataText(plot)
		if err := validateSIVIDeletionHistory(context.Background(), history); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO "__VPRO_SIVIDeletionHistory"(RequestID,Created,Proposal) VALUES(?,?,?)`,
			id, when, siviDeletionServiceJSON(t, history)); err != nil {
			t.Fatal(err)
		}
	}
	first := "00000000-0000-4000-8000-000000000001"
	second := "00000000-0000-4000-8000-000000000002"
	insert(second, request.Plot, "2099-01-01 00:00:00")
	insert(first, request.Plot, "2099-01-01 00:00:00")
	insert("00000000-0000-4000-8000-000000000003", "OTHER01", "2099-01-02 00:00:00")
	service := siviDeletionServiceFixture(t, contexts, false, true)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for i := 0; i < 3; i++ {
		list, err := service.GetHistory(context.Background(), state.ContextID, request.Plot)
		if err != nil || list == nil || len(list.Events) != 3 || list.Events[0].HistoryID != first ||
			list.Events[1].HistoryID != second || list.Events[2].HistoryID != request.HistoryID {
			t.Fatal("listing collapsed historical events, included another plot, or has unstable order", list, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestSIVIDeletionHistoryOwnershipCancellationAndSourceDrift(t *testing.T) {
	contexts, state, db, request, _ := siviDeletionRestorationFixture(t, true, 3)
	service := siviDeletionServiceFixture(t, contexts, false, true)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if list, err := service.GetHistory(ctx, state.ContextID, request.Plot); list != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled history read returned a list", list, err)
	}
	if list, err := contexts.readSIVIDeletionHistory(nil, state.ContextID, request.Plot); list != nil || err == nil {
		t.Fatal("private reader accepted nil context", list, err)
	}
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	list, err := service.GetHistory(ctx, state.ContextID, request.Plot)
	cancel()
	owner.mu.Unlock()
	if list != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("history lease ignored cancellation", list, err)
	}
	assertProfileSUFiles(t, contexts, before)
	for _, plot := range []string{"", "108050 ", "outside", "10805\x00", "10805\xff", "10805000"} {
		assertSIVIDeletionHistoryRefused(t, contexts, state.ContextID, plot)
	}
	assertSIVIDeletionHistoryRefused(t, contexts, "foreign-context", request.Plot)
	mutateContextFixture(t, state.SUPath, `UPDATE Report_SU SET PlotNumber='outside' WHERE PlotNumber='108050'`)
	assertSIVIDeletionHistoryRefused(t, contexts, state.ContextID, request.Plot)
	mutateContextFixture(t, state.SUPath, `UPDATE Report_SU SET PlotNumber='108050' WHERE PlotNumber='outside'`)
	if _, err := db.Exec(`UPDATE Sample_Admin SET Plot='outside' WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionHistoryRefused(t, contexts, state.ContextID, request.Plot)
}

func TestSIVIDeletionHistorySharedLibraryGlobalValidationBeforeFiltering(t *testing.T) {
	contexts, state, db, firstDeletion := siviDeletionFixture(t, false, 0)
	for _, suffix := range coreTables {
		if _, err := db.Exec(`CREATE TABLE ` + quoteHeaderIdentifier("Second_"+suffix) + ` AS SELECT * FROM ` + quoteHeaderIdentifier("Sample_"+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO _table_metadata(table_name,description) VALUES('Second_Env','VP08')`); err != nil {
		t.Fatal(err)
	}
	if result, err := contexts.deleteSIVIVegetation(context.Background(), state.ContextID, firstDeletion); result == nil || err != nil {
		t.Fatal(result, err)
	}
	selection := contextSelection(state)
	selection.Project = "Second"
	secondState, err := contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	service := siviDeletionServiceFixture(t, contexts, false, true)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	empty, err := service.GetHistory(context.Background(), secondState.ContextID, firstDeletion.Plot)
	if err != nil || empty == nil || !empty.HistoryPresent || len(empty.Events) != 0 {
		t.Fatal("foreign-only history not distinguished from missing history", empty, err)
	}
	assertProfileSUFiles(t, contexts, before)
	second := firstDeletion
	second.RequestID, second.ContextID, second.Project = "00000000-0000-4000-8000-000000000002", secondState.ContextID, "Second"
	original, err := contexts.readSIVIDeletionOriginal(context.Background(), secondState.ContextID, second.Plot, second.Form, second.Original.RowID)
	if err != nil {
		t.Fatal(err)
	}
	second.Columns, second.Original = original.Columns, original.Original
	if result, err := contexts.deleteSIVIVegetation(context.Background(), secondState.ContextID, second); result == nil || err != nil {
		t.Fatal(result, err)
	}
	review, err := service.ReviewRestoration(context.Background(), secondState.ContextID, second.Plot, second.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	restoration := siviDeletionRestorationRequest{RequestID: "00000000-0000-4000-8000-000000000011",
		ContextID: secondState.ContextID, Project: second.Project, Plot: second.Plot, HistoryID: second.RequestID,
		Action: AuditRestorePrune, Expected: review.Expected}
	if result, err := service.Restore(context.Background(), secondState.ContextID, siviDeletionServiceJSON(t, restoration)); result == nil || err != nil {
		t.Fatal(result, err)
	}
	secondList, err := service.GetHistory(context.Background(), secondState.ContextID, second.Plot)
	if err != nil || secondList == nil || len(secondList.Events) != 1 || secondList.Events[0].HistoryID != second.RequestID ||
		!secondList.Events[0].Consumed {
		t.Fatal("shared-library list mixed project ownership or consumption", secondList, err)
	}
	reopened, err := contexts.SwitchContext(secondState.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	firstList, err := service.GetHistory(context.Background(), reopened.ContextID, firstDeletion.Plot)
	if err != nil || firstList == nil || len(firstList.Events) != 1 || firstList.Events[0].HistoryID != firstDeletion.RequestID ||
		firstList.Events[0].Consumed {
		t.Fatal("foreign consumed event leaked into current owner list", firstList, err)
	}
	assertProfileSUFiles(t, contexts, before)
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionHistory" WHERE RequestID=?`, second.RequestID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var corrupt siviDeletionHistory
	if err := json.Unmarshal([]byte(proposal), &corrupt); err != nil {
		t.Fatal(err)
	}
	corrupt.Result.Project = "Sample"
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=? WHERE RequestID=?`,
		siviDeletionServiceJSON(t, corrupt), second.RequestID); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionHistoryRefused(t, contexts, reopened.ContextID, firstDeletion.Plot)
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionHistory" SET Proposal=? WHERE RequestID=?`, proposal, second.RequestID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIDeletionRestorationHistory"`).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var corruptRestoration siviDeletionRestorationHistory
	if err := json.Unmarshal([]byte(proposal), &corruptRestoration); err != nil {
		t.Fatal(err)
	}
	corruptRestoration.Request.Expected = strings.Repeat("0", 64)
	if _, err := db.Exec(`UPDATE "__VPRO_SIVIDeletionRestorationHistory" SET Proposal=?`, siviDeletionServiceJSON(t, corruptRestoration)); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionHistoryRefused(t, contexts, reopened.ContextID, firstDeletion.Plot)
}

func assertSIVIDeletionTargetsRefused(t *testing.T, contexts *ContextService, contextID, plot string) {
	t.Helper()
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	service := siviDeletionServiceFixture(t, contexts, true, false)
	if targets, err := service.GetTargets(context.Background(), contextID, plot); targets != nil || err == nil {
		t.Fatal("invalid targets became an empty or partial successful table", targets, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVIDeletionTargetsRaw44UnavailableIDsBlobsAndExactParentScope(t *testing.T) {
	contexts, state, db, deletion := siviDeletionFixture(t, true, 3)
	service, err := NewSIVIDeletionService(contexts, func(name string) (string, bool) {
		return "true", name == siviDeletionWritingFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Flag=1 WHERE rowid=?;
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,Other2) VALUES('108050','NULLID',NULL,0,X'00FF');
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover6) VALUES('108050','WIDE',2147483648,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover7) VALUES('108050','TEXTID','bad',0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover5a) VALUES('108050','REALID',1.5,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','BLOBID',X'0001',0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover10) VALUES('108050','NONMEM',3001,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050 ','SPACED',3002,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES(CAST('108050' AS BLOB),'BLOBPLOT',3003,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('outside','OTHER',3004,0)`,
		deletion.Original.RowID); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	var targets *SIVIDeletionTargets
	targets, err = service.GetTargets(context.Background(), state.ContextID, deletion.Plot)
	if err != nil || targets == nil || targets.ContextID != state.ContextID || targets.Project != deletion.Project ||
		targets.Plot != deletion.Plot || len(targets.Columns) != 44 || targets.Rows == nil {
		t.Fatal("writing-only discovery lost exact owner or raw44 table", targets, err)
	}
	table := ProjectMetadataTable{Columns: targets.Columns, Rows: targets.Rows}
	columns, err := siteUnitTransferColumns(table, siviCreationColumns...)
	if err != nil {
		t.Fatal(err)
	}
	var literalCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Veg WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB)`,
		deletion.Plot).Scan(&literalCount); err != nil || len(targets.Rows) != literalCount {
		t.Fatal("targets included physical parent aliases or omitted unavailable rows", len(targets.Rows), literalCount, err)
	}
	seen := map[string]ProjectMetadataRow{}
	for i, row := range targets.Rows {
		if len(row.Cells) != 44 || row.Cells[columns["PlotNumber"]].Text == nil ||
			*row.Cells[columns["PlotNumber"]].Text != deletion.Plot {
			t.Fatal("row has incomplete cells or a foreign parent", row)
		}
		if i > 0 {
			previous, _ := strconv.ParseInt(targets.Rows[i-1].RowID, 10, 64)
			current, err := strconv.ParseInt(row.RowID, 10, 64)
			if err != nil || current <= previous {
				t.Fatal("targets lost exact ordered signed64 physical identities", row.RowID, err)
			}
		}
		if cell := row.Cells[columns["Species"]]; cell.Text != nil {
			seen[*cell.Text] = row
		}
	}
	blobID, blobOther := "0001", "00ff"
	expectedIDs := map[string]ProjectMetadataCell{
		"NULLID": {Storage: "null"}, "WIDE": metadataInteger("2147483648"), "TEXTID": metadataText("bad"),
		"REALID": siviReal(1.5), "BLOBID": {Storage: "blob", BlobHex: &blobID}, "NONMEM": metadataInteger("3001"),
	}
	for species, expected := range expectedIDs {
		row, present := seen[species]
		if !present || !reflect.DeepEqual(row.Cells[columns["ID"]], expected) {
			t.Fatal("read-only targets hid/repaired/coerced unavailable historical identity", species, row, expected)
		}
		if original, err := service.GetOriginal(context.Background(), state.ContextID, deletion.Plot, "SubVegA-SIVI", row.RowID); original != nil || err == nil {
			t.Fatal("discovery bypassed backend source/identity/BLOB mutation guards", species, original, err)
		}
	}
	if !reflect.DeepEqual(seen["NULLID"].Cells[columns["Other2"]], ProjectMetadataCell{Storage: "blob", BlobHex: &blobOther}) ||
		!reflect.DeepEqual(seen["RAW"].Cells[columns["Flag"]], metadataInteger("1")) {
		t.Fatal("raw BLOB bytes or historical Flag1 changed in read-only targets")
	}
	raw := siviDeletionServiceJSON(t, targets)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	keys := []string{"contextId", "project", "plot", "columns", "rows"}
	if len(fields) != len(keys) {
		t.Fatal("target DTO shape has implicit fields", raw)
	}
	for _, key := range keys {
		if fields[key] == nil {
			t.Fatal("missing lowerCamel target property", key)
		}
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','DUP',10000001,0)`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	targets, err = service.GetTargets(context.Background(), state.ContextID, deletion.Plot)
	if err != nil || targets == nil || len(targets.Rows) != literalCount+1 {
		t.Fatal("discovery silently collapsed ambiguous logical identities", targets, err)
	}
	if original, err := service.GetOriginal(context.Background(), state.ContextID, deletion.Plot, deletion.Form, deletion.Original.RowID); original != nil || err == nil {
		t.Fatal("targets bypassed exact original ambiguity checks", original, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVIDeletionTargetsStrictSchemaAndMalformedScopedStorage(t *testing.T) {
	for _, change := range []string{
		`ALTER TABLE Sample_Veg ADD COLUMN Unexpected TEXT`,
		`ALTER TABLE Sample_Veg RENAME COLUMN Other2 TO Missing`,
		`ALTER TABLE Sample_Veg RENAME COLUMN HeightA TO heighta`,
		`DROP TABLE Sample_Veg`,
		`DROP TABLE Sample_Veg; CREATE VIEW Sample_Veg AS SELECT 1 AS PlotNumber WHERE 0`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Other2) VALUES('108050','BAD',9001,CAST(X'FF' AS TEXT))`,
		`INSERT INTO Sample_Veg(PlotNumber,Species,ID,HeightA) VALUES('108050','BAD',9001,1e999)`,
	} {
		t.Run(change, func(t *testing.T) {
			contexts, state, db, deletion := siviDeletionFixture(t, false, 3)
			if _, err := db.Exec(change); err != nil {
				t.Fatal(err)
			}
			assertSIVIDeletionTargetsRefused(t, contexts, state.ContextID, deletion.Plot)
		})
	}
}

func TestSIVIDeletionTargetsEmptyTableReopenAndUnavailableMembership(t *testing.T) {
	contexts, state, db, deletion := siviDeletionFixture(t, false, 3)
	if _, err := db.Exec(`DELETE FROM Sample_Veg WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	service := siviDeletionServiceFixture(t, contexts, true, false)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	targets, err := service.GetTargets(context.Background(), state.ContextID, deletion.Plot)
	if err != nil || targets == nil || len(targets.Columns) != 44 || targets.Rows == nil || len(targets.Rows) != 0 {
		t.Fatal("valid empty parent lost full schema or returned NULL rows", targets, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover5a) VALUES('108050','HIDDEN',3001,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover6) VALUES('108050','C',3002,0);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover9) VALUES('108050','D',3003,0)`); err != nil {
		t.Fatal(err)
	}
	reopened, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	targets, err = service.GetTargets(context.Background(), reopened.ContextID, deletion.Plot)
	if err != nil || targets == nil || targets.ContextID != reopened.ContextID || len(targets.Rows) != 3 {
		t.Fatal("reopened writing-only facade required remembered physical identities", targets, err)
	}
	columns, err := siteUnitTransferColumns(ProjectMetadataTable{Columns: targets.Columns, Rows: targets.Rows}, "Species")
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string][]string{"HIDDEN": {"SubVegA-SIVI", "SubVegA-SIVI_BC"}, "C": {"SubVegC-SIVI"}, "D": {"SubVegD-SIVI"}}
	for _, row := range targets.Rows {
		species := *row.Cells[columns["Species"]].Text
		for _, form := range forms[species] {
			original, err := service.GetOriginal(context.Background(), reopened.ContextID, deletion.Plot, form, row.RowID)
			if err != nil || original == nil || !reflect.DeepEqual(original.Original, row) || !reflect.DeepEqual(original.Columns, targets.Columns) {
				t.Fatal("discovered source path could not acquire exact private original", species, form, original, err)
			}
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestSIVIDeletionTargetsCancellationOwnershipAndLiteralParentGuards(t *testing.T) {
	contexts, state, db, deletion := siviDeletionFixture(t, true, 3)
	service := siviDeletionServiceFixture(t, contexts, true, false)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if targets, err := service.GetTargets(ctx, state.ContextID, deletion.Plot); targets != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled target read returned data", targets, err)
	}
	if targets, err := contexts.readSIVIDeletionTargets(nil, state.ContextID, deletion.Plot); targets != nil || err == nil {
		t.Fatal("private target reader accepted nil context", targets, err)
	}
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	targets, err := service.GetTargets(ctx, state.ContextID, deletion.Plot)
	cancel()
	owner.mu.Unlock()
	if targets != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("target snapshot lease ignored cancellation", targets, err)
	}
	assertProfileSUFiles(t, contexts, before)
	for _, plot := range []string{"", "108050 ", "outside", "10805\x00", "10805\xff", "10805000"} {
		assertSIVIDeletionTargetsRefused(t, contexts, state.ContextID, plot)
	}
	assertSIVIDeletionTargetsRefused(t, contexts, "foreign-context", deletion.Plot)
	mutateContextFixture(t, state.SUPath, `UPDATE Report_SU SET PlotNumber='outside' WHERE PlotNumber='108050'`)
	assertSIVIDeletionTargetsRefused(t, contexts, state.ContextID, deletion.Plot)
	mutateContextFixture(t, state.SUPath, `UPDATE Report_SU SET PlotNumber='108050' WHERE PlotNumber='outside'`)
	if _, err := db.Exec(`DELETE FROM Sample_Admin WHERE Plot='108050'`); err != nil {
		t.Fatal(err)
	}
	assertSIVIDeletionTargetsRefused(t, contexts, state.ContextID, deletion.Plot)
}
