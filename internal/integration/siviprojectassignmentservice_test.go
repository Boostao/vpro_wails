package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func siviPublicProjectAssignmentRequest(parent *sourceform.SiviParentProjection, selection sourceform.SiviProjectSelection) sourceform.SIVIProjectAssignmentWrite {
	return sourceform.SIVIProjectAssignmentWrite{Original: parent, Selection: sourceform.SIVIProjectSelection{
		ContextID: selection.ContextID, ControlID: selection.ControlID, Table: selection.Table, RowID: selection.RowID,
		Expected: selection.Expected, SourceOption: selection.SourceOption, MetadataAlias: selection.MetadataAlias,
		MetadataTable: selection.MetadataTable, MetadataColumns: selection.MetadataColumns, MetadataOriginal: selection.MetadataOriginal,
	}}
}

func siviEnablePublicAssignment(*vprocore.ContextService) {}

func TestSIVIProjectAssignmentPublicSurfacesWithoutWrites(t *testing.T) {
	service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	siviEnablePublicAssignment(service)
	got, err := sourceform.GetSIVIProjectAssignmentOriginal(service, context.Background(), state.ContextID, parent.Plot)
	if err != nil || !got.AssignmentAvailable || got.AssignmentDiagnostic == "" || !reflect.DeepEqual(got.Original, parent) ||
		got.Choices.SourceOption != selection.SourceOption || !reflect.DeepEqual(got.Choices.Choices.Columns, selection.MetadataColumns) ||
		!reflect.DeepEqual(got.Choices.Choices.Rows[len(got.Choices.Choices.Rows)-1], selection.MetadataOriginal) {
		t.Fatal("atomic owned parent/metadata original differs", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
	fresh, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !reflect.DeepEqual(fresh, config) {
		t.Fatal("assignment initialization/gates changed source preference", err)
	}
}

func TestSIVIProjectAssignmentPublicAvailabilityIsObservedNotAuthorization(t *testing.T) {
	service, state, _, parent, _ := sftest.SiviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	siviEnablePublicAssignment(service)
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL fixture unavailable", mode, err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	got, err := sourceform.GetSIVIProjectAssignmentOriginal(service, context.Background(), state.ContextID, parent.Plot)
	if err != nil || got == nil || got.AssignmentAvailable || !strings.Contains(got.AssignmentDiagnostic, "rollback-journal") ||
		!reflect.DeepEqual(got.Original, parent) || got.Choices.SourceOption != 2 {
		t.Fatal("unavailable master disappeared or was authorized", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIProjectAssignmentPublicAvailableReadDoesNotAuthorizeLaterJournalDrift(t *testing.T) {
	service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	siviEnablePublicAssignment(service)
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	got, err := sourceform.GetSIVIProjectAssignmentOriginal(service, context.Background(), state.ContextID, parent.Plot)
	if err != nil || got == nil || !got.AssignmentAvailable || got.AssignmentDiagnostic == "" {
		t.Fatal("safe master availability retry failed", got, err)
	}
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL drift fixture unavailable", mode, err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if result, err := sourceform.SaveSIVIProjectAssignment(service, context.Background(), state.ContextID, parent.Plot,
		siviPublicProjectAssignmentRequest(got.Original, selection)); err == nil || result != nil ||
		!strings.Contains(err.Error(), "rollback-journal") {
		t.Fatal("prior available read authorized later WAL metadata", result, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIProjectAssignmentPublicStrictWireSaveRestore(t *testing.T) {
	for _, external := range []bool{false, true} {
		for source := 1; source <= 2; source++ {
			service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, external, source, 3, "old")
			siviEnablePublicAssignment(service)
			body, err := json.Marshal(siviPublicProjectAssignmentRequest(parent, selection))
			if err != nil {
				t.Fatal(err)
			}
			var request sourceform.SIVIProjectAssignmentWrite
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(request.Selection.Private(), selection) {
				t.Fatal("wire lost literal/schema/title/duplicate provenance", request.Selection, selection)
			}
			result, err := sourceform.SaveSIVIProjectAssignment(service, context.Background(), state.ContextID, parent.Plot, request)
			if err != nil || result.ChangedCells != 1 || result.HistoryID == "" {
				t.Fatal("public existing-definition assignment failed", external, source, result, err)
			}
			restored, err := sourceform.RestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, parent.Plot, result.HistoryID, vprocore.AuditRestorePrune)
			if err != nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 || restored.CleanedVegRows != 0 {
				t.Fatal("public assignment typed restoration failed", restored, err)
			}
			fresh, err := sourceform.GetSIVIProjectAssignmentOriginal(service, context.Background(), state.ContextID, parent.Plot)
			if err != nil || !reflect.DeepEqual(fresh.Original, parent) {
				t.Fatal("public assignment original did not recover exactly", fresh, err)
			}
			if _, err := sourceform.RestoreSIVIProjectAssignment(service, context.Background(), state.ContextID, parent.Plot, result.HistoryID, vprocore.AuditRestorePrune); err == nil {
				t.Fatal("public assignment replay accepted")
			}
		}
	}
}

func TestSIVIProjectAssignmentPublicCancellationAndSourceDrift(t *testing.T) {
	service, state, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	siviEnablePublicAssignment(service)
	request := siviPublicProjectAssignmentRequest(parent, selection)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.GetSIVIProjectAssignmentOriginal(service, ctx, state.ContextID, parent.Plot); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("original discarded cancellation", got, err)
	}
	if got, err := sourceform.SaveSIVIProjectAssignment(service, ctx, state.ContextID, parent.Plot, request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("save discarded cancellation", got, err)
	}
	if got, err := sourceform.RestoreSIVIProjectAssignment(service, ctx, state.ContextID, parent.Plot, "1", vprocore.AuditRestoreCancel); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("restore discarded cancellation", got, err)
	}
	if got, err := sourceform.GetSIVIProjectAssignmentOriginal(service, context.Background(), "foreign", parent.Plot); err == nil || got != nil {
		t.Fatal("original discarded context ownership", got, err)
	}
	if err := service.Projects.Preferences.Update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	if got, err := sourceform.SaveSIVIProjectAssignment(service, context.Background(), state.ContextID, parent.Plot, request); err == nil || got != nil {
		t.Fatal("public save trusted stale source preference", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
}

func TestSIVIProjectAssignmentPublicTransportRequiresCompleteTokens(t *testing.T) {
	_, _, _, parent, selection := sftest.SiviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	request := siviPublicProjectAssignmentRequest(parent, selection)
	selectionBody, err := json.Marshal(request.Selection)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	checkRequired := func(raw []byte, names []string, decode func([]byte) error) {
		t.Helper()
		for _, name := range names {
			for _, mode := range []string{"missing", "null"} {
				var object map[string]json.RawMessage
				if err := json.Unmarshal(raw, &object); err != nil {
					t.Fatal(err)
				}
				if mode == "missing" {
					delete(object, name)
				} else {
					object[name] = json.RawMessage("null")
				}
				changed, err := json.Marshal(object)
				if err != nil {
					t.Fatal(err)
				}
				if err := decode(changed); err == nil {
					t.Fatal("transport silently defaulted token", name, mode)
				}
			}
		}
	}
	checkRequired(body, []string{"original", "selection"}, func(raw []byte) error {
		var decoded sourceform.SIVIProjectAssignmentWrite
		return json.Unmarshal(raw, &decoded)
	})
	checkRequired(selectionBody, []string{"contextId", "controlId", "table", "rowId", "expected", "sourceOption",
		"metadataAlias", "metadataTable", "metadataColumns", "metadataOriginal"}, func(raw []byte) error {
		var decoded sourceform.SIVIProjectSelection
		return json.Unmarshal(raw, &decoded)
	})
	for _, field := range []string{"name", "declaredType"} {
		for _, column := range []string{`{}`, `{"name":"ProjectID"}`, `{"declaredType":"TEXT"}`} {
			if strings.Contains(column, `"`+field+`"`) {
				continue
			}
			var object map[string]json.RawMessage
			if err := json.Unmarshal(selectionBody, &object); err != nil {
				t.Fatal(err)
			}
			object["metadataColumns"] = json.RawMessage("[" + column + "]")
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			var decoded sourceform.SIVIProjectSelection
			if err := json.Unmarshal(raw, &decoded); err == nil {
				t.Fatal("projected column silently defaulted", field, column)
			}
		}
	}
	for _, row := range []string{`{}`, `{"rowId":"2"}`, `{"cells":[]}`, `{"rowId":"2","cells":null}`} {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(selectionBody, &object); err != nil {
			t.Fatal(err)
		}
		object["metadataOriginal"] = json.RawMessage(row)
		raw, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		var decoded sourceform.SIVIProjectSelection
		if err := json.Unmarshal(raw, &decoded); err == nil {
			t.Fatal("selected physical row silently defaulted", row)
		}
	}
	for _, raw := range []string{
		strings.TrimSuffix(string(body), "}") + `,"actions":[]}`,
		strings.Replace(string(body), `"sourceOption":1`, `"sourceOption":"1"`, 1),
		strings.Replace(string(body), `"sourceOption":1`, `"sourceOption":1.5`, 1),
		strings.Replace(string(body), `"controlId":"form:frmSIVIsite/ProjectID"`, `"controlId":"\ud800"`, 1),
		strings.Replace(string(body), `"rowId":"2"`, `"rowId":2`, 1),
		strings.Replace(string(body), `"metadataAlias":"project"`, `"metadataAlias":"`+string([]byte{0xff})+`"`, 1),
		string(body) + `{}`,
	} {
		var decoded sourceform.SIVIProjectAssignmentWrite
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("unknown/coerced/malformed assignment transport accepted", raw[:100])
		}
	}
	request.Selection.MetadataOriginal.RowID = "9007199254740993"
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded sourceform.SIVIProjectAssignmentWrite
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.Selection.MetadataOriginal.RowID != "9007199254740993" {
		t.Fatal("transport rounded the physical metadata identity", decoded.Selection.MetadataOriginal.RowID, err)
	}
}
