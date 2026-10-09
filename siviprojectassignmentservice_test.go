package main

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

func siviPublicProjectAssignmentRequest(parent *siviParentProjection, selection siviProjectSelection) SIVIProjectAssignmentWrite {
	return SIVIProjectAssignmentWrite{Original: parent, Selection: SIVIProjectSelection{
		ContextID: selection.ContextID, ControlID: selection.ControlID, Table: selection.Table, RowID: selection.RowID,
		Expected: selection.Expected, SourceOption: selection.SourceOption, MetadataAlias: selection.MetadataAlias,
		MetadataTable: selection.MetadataTable, MetadataColumns: selection.MetadataColumns, MetadataOriginal: selection.MetadataOriginal,
	}}
}

func siviEnablePublicAssignment(service *ContextService) {
	service.siviParentReviewEnabled, service.siviParentEditingEnabled, service.siviProjectAssignmentEnabled = true, true, true
	// Source actions are a separate, unnecessary permission.
	service.siviParentActionEditingEnabled = false
}

func TestSIVIProjectAssignmentFeatureLiteralAndIndependent(t *testing.T) {
	for _, test := range []struct {
		value   string
		present bool
		enabled bool
		valid   bool
	}{{"", false, false, true}, {"true", true, true, true}, {"false", true, false, true},
		{"TRUE", true, false, false}, {" true", true, false, false}, {"1", true, false, false}, {"", true, false, false}} {
		result, err := siviProjectAssignmentFeature(func(name string) (string, bool) {
			if name != siviProjectAssignmentFeatureEnvironment {
				t.Fatal("assignment flag inherited another workflow", name)
			}
			return test.value, test.present
		})
		if (err == nil) != test.valid || result != test.enabled {
			t.Fatal("assignment feature accepted a silent/coerced flag", test, result, err)
		}
	}
}

func TestSIVIProjectAssignmentPublicGatesAllSurfacesWithoutWrites(t *testing.T) {
	service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	request := siviPublicProjectAssignmentRequest(parent, selection)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	for mask := 0; mask < 7; mask++ {
		service.siviParentReviewEnabled = mask&1 != 0
		service.siviParentEditingEnabled = mask&2 != 0
		service.siviProjectAssignmentEnabled = mask&4 != 0
		service.siviParentActionEditingEnabled = true
		if got, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), state.ContextID, parent.Plot); err == nil || got != nil {
			t.Fatal("partially enabled assignment initialized", mask, got, err)
		}
		if got, err := service.SaveSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, request); err == nil || got != nil {
			t.Fatal("partially enabled assignment wrote", mask, got, err)
		}
		if got, err := service.RestoreSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, "1", AuditRestoreCancel); err == nil || got != nil {
			t.Fatal("partially enabled assignment restore succeeded", mask, got, err)
		}
		assertProfileSUFiles(t, service, files)
	}
	siviEnablePublicAssignment(service)
	got, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), state.ContextID, parent.Plot)
	if err != nil || !got.AssignmentAvailable || got.AssignmentDiagnostic == "" || !reflect.DeepEqual(got.Original, parent) ||
		got.Choices.SourceOption != selection.SourceOption || !reflect.DeepEqual(got.Choices.Choices.Columns, selection.MetadataColumns) ||
		!reflect.DeepEqual(got.Choices.Choices.Rows[len(got.Choices.Choices.Rows)-1], selection.MetadataOriginal) {
		t.Fatal("atomic owned parent/metadata original differs", got, err)
	}
	assertProfileSUFiles(t, service, files)
	fresh, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !reflect.DeepEqual(fresh, config) {
		t.Fatal("assignment initialization/gates changed source preference", err)
	}
}

func TestSIVIProjectAssignmentPublicAvailabilityIsObservedNotAuthorization(t *testing.T) {
	service, state, _, parent, _ := siviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	siviEnablePublicAssignment(service)
	master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL fixture unavailable", mode, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	got, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), state.ContextID, parent.Plot)
	if err != nil || got == nil || got.AssignmentAvailable || !strings.Contains(got.AssignmentDiagnostic, "rollback-journal") ||
		!reflect.DeepEqual(got.Original, parent) || got.Choices.SourceOption != 2 {
		t.Fatal("unavailable master disappeared or was authorized", got, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIProjectAssignmentPublicAvailableReadDoesNotAuthorizeLaterJournalDrift(t *testing.T) {
	service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 2, 3, "old")
	siviEnablePublicAssignment(service)
	master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	got, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), state.ContextID, parent.Plot)
	if err != nil || got == nil || !got.AssignmentAvailable || got.AssignmentDiagnostic == "" {
		t.Fatal("safe master availability retry failed", got, err)
	}
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("WAL drift fixture unavailable", mode, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.SaveSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot,
		siviPublicProjectAssignmentRequest(got.Original, selection)); err == nil || result != nil ||
		!strings.Contains(err.Error(), "rollback-journal") {
		t.Fatal("prior available read authorized later WAL metadata", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIProjectAssignmentPublicStrictWireSaveRestore(t *testing.T) {
	for _, external := range []bool{false, true} {
		for source := 1; source <= 2; source++ {
			service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, external, source, 3, "old")
			siviEnablePublicAssignment(service)
			body, err := json.Marshal(siviPublicProjectAssignmentRequest(parent, selection))
			if err != nil {
				t.Fatal(err)
			}
			var request SIVIProjectAssignmentWrite
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(request.Selection.private(), selection) {
				t.Fatal("wire lost literal/schema/title/duplicate provenance", request.Selection, selection)
			}
			result, err := service.SaveSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, request)
			if err != nil || result.ChangedCells != 1 || result.HistoryID == "" {
				t.Fatal("public existing-definition assignment failed", external, source, result, err)
			}
			restored, err := service.RestoreSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, result.HistoryID, AuditRestorePrune)
			if err != nil || restored.RestoredRows != 1 || restored.PrunedAuditRows != 1 || restored.CleanedVegRows != 0 {
				t.Fatal("public assignment typed restoration failed", restored, err)
			}
			fresh, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), state.ContextID, parent.Plot)
			if err != nil || !reflect.DeepEqual(fresh.Original, parent) {
				t.Fatal("public assignment original did not recover exactly", fresh, err)
			}
			if _, err := service.RestoreSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, result.HistoryID, AuditRestorePrune); err == nil {
				t.Fatal("public assignment replay accepted")
			}
		}
	}
}

func TestSIVIProjectAssignmentPublicCancellationAndSourceDrift(t *testing.T) {
	service, state, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
	siviEnablePublicAssignment(service)
	request := siviPublicProjectAssignmentRequest(parent, selection)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.GetSIVIProjectAssignmentOriginal(ctx, state.ContextID, parent.Plot); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("original discarded cancellation", got, err)
	}
	if got, err := service.SaveSIVIProjectAssignment(ctx, state.ContextID, parent.Plot, request); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("save discarded cancellation", got, err)
	}
	if got, err := service.RestoreSIVIProjectAssignment(ctx, state.ContextID, parent.Plot, "1", AuditRestoreCancel); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("restore discarded cancellation", got, err)
	}
	if got, err := service.GetSIVIProjectAssignmentOriginal(context.Background(), "foreign", parent.Plot); err == nil || got != nil {
		t.Fatal("original discarded context ownership", got, err)
	}
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.SaveSIVIProjectAssignment(context.Background(), state.ContextID, parent.Plot, request); err == nil || got != nil {
		t.Fatal("public save trusted stale source preference", got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIProjectAssignmentPublicTransportRequiresCompleteTokens(t *testing.T) {
	_, _, _, parent, selection := siviProjectAssignmentWriteFixture(t, false, 1, 3, "old")
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
		var decoded SIVIProjectAssignmentWrite
		return json.Unmarshal(raw, &decoded)
	})
	checkRequired(selectionBody, []string{"contextId", "controlId", "table", "rowId", "expected", "sourceOption",
		"metadataAlias", "metadataTable", "metadataColumns", "metadataOriginal"}, func(raw []byte) error {
		var decoded SIVIProjectSelection
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
			var decoded SIVIProjectSelection
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
		var decoded SIVIProjectSelection
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
		var decoded SIVIProjectAssignmentWrite
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("unknown/coerced/malformed assignment transport accepted", raw[:100])
		}
	}
	request.Selection.MetadataOriginal.RowID = "9007199254740993"
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SIVIProjectAssignmentWrite
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.Selection.MetadataOriginal.RowID != "9007199254740993" {
		t.Fatal("transport rounded the physical metadata identity", decoded.Selection.MetadataOriginal.RowID, err)
	}
}
