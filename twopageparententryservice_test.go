package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTwoPageParentEntryServiceIndependentGates(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, review := range []bool{false, true} {
		for _, editing := range []bool{false, true} {
			service, err := NewTwoPageParentEntryService(contexts, provider.readers, func(name string) (string, bool) {
				switch name {
				case twoPageParentReviewEnvironment:
					return strconv.FormatBool(review), true
				case twoPageParentEntryEditingEnvironment:
					return strconv.FormatBool(editing), true
				default:
					t.Fatal("complete entry inherited another editor's permission", name)
					return "", false
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
			if review {
				if err != nil || got == nil || len(got.Fields) != 56 || got.ProjectChoices == nil {
					t.Fatal("enabled complete entry could not read source", got, err)
				}
			} else if got != nil || err == nil {
				t.Fatal("disabled complete entry returned original", got, err)
			}
			if !review || !editing {
				if got, err := service.Save(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
					TwoPageParentEntryWrite{}); got != nil || err == nil {
					t.Fatal("independently disabled complete entry wrote", got, err)
				}
				if got, err := service.Restore(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
					"1", AuditRestoreCancel); got != nil || err == nil {
					t.Fatal("disabled complete entry restored", got, err)
				}
			}
			assertProfileSUFiles(t, contexts, before)
		}
	}
	for _, name := range []string{twoPageParentReviewEnvironment, twoPageParentEntryEditingEnvironment} {
		if _, err := NewTwoPageParentEntryService(contexts, provider.readers, func(key string) (string, bool) {
			if key == name {
				return "bad flag", true
			}
			return "true", true
		}); err == nil {
			t.Fatal("malformed entry gate defaulted", name)
		}
	}
	if _, err := NewTwoPageParentEntryService(nil, provider.readers, osAbsentEntryFlags); err == nil {
		t.Fatal("nil contexts accepted")
	}
	if _, err := NewTwoPageParentEntryService(contexts, provider.readers, nil); err == nil {
		t.Fatal("implicit flag lookup accepted")
	}
	service, err := NewTwoPageParentEntryService(contexts, provider.readers, osAbsentEntryFlags)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetReferences(context.Background(), state.ContextID, "108050", "FS882-8x6XL",
		TwoPageEntryReferenceFilters{}); got != nil || err == nil {
		t.Fatal("absent flags enabled references", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var absent *TwoPageParentEntryService
	for _, candidate := range []*TwoPageParentEntryService{absent, service} {
		if got, err := candidate.Save(ctx, state.ContextID, "108050", "FS882-8x6XL", TwoPageParentEntryWrite{}); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation lost to permission check", got, err)
		}
		if got, err := candidate.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); got != nil || err == nil {
			t.Fatal("nil/disabled service returned source", got, err)
		}
	}
	assertProfileSUFiles(t, contexts, before)
}

func osAbsentEntryFlags(string) (string, bool) { return "", false }

func TestTwoPageParentEntryServiceOwnedSnapshotHistoricalFiltersAndRetry(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		contexts, state, db, _, _ := twoPageWriteFixture(t, form, false)
		provider := twoPageEntryCatalogueProviderFixture(t, form)
		service, err := NewTwoPageParentEntryService(contexts, provider.readers, func(string) (string, bool) { return "true", true })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE Sample_Env SET Zone='BG', SubZone='xh1' WHERE PlotNumber='108050'`); err != nil {
			t.Fatal(err)
		}
		original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
		if err != nil || original == nil || !reflect.DeepEqual(original.Zone, metadataText("BG")) ||
			!reflect.DeepEqual(original.SubZone, metadataText("xh1")) {
			t.Fatal("initial snapshot lost owned original BEC pair", original, err)
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		draft, err := service.GetReferences(context.Background(), state.ContextID, "108050", form,
			TwoPageEntryReferenceFilters{metadataText("CWH"), metadataText("vm1")})
		if err != nil || draft == nil || !reflect.DeepEqual(draft.Original, original.Original) ||
			!reflect.DeepEqual(draft.Zone, metadataText("CWH")) || !reflect.DeepEqual(draft.SubZone, metadataText("vm1")) {
			t.Fatal("draft references replaced physical original", draft, err)
		}
		for _, source := range []int{1, 2} {
			if err := contexts.projects.preferences.update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
				t.Fatal(err)
			}
			snapshot, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
			if err != nil || snapshot == nil || snapshot.ProjectChoices == nil ||
				snapshot.ProjectChoices.SourceOption != source || !snapshot.ProjectAssignmentAvailable {
				t.Fatal("typed Project choices/source availability drifted", snapshot, err)
			}
		}
		assertProfileSUFiles(t, contexts, before)
		if _, err := db.Exec(`UPDATE Sample_Env SET Zone='historical long zone' WHERE PlotNumber='108050'`); err != nil {
			t.Fatal(err)
		}
		historical, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
		if err != nil || historical == nil || !reflect.DeepEqual(historical.Zone, metadataText("historical long zone")) {
			t.Fatal("historical invalid unchanged BEC value blocked original", historical, err)
		}
		unavailable := 0
		for _, reference := range historical.Fields {
			if reference.Column == "SubZone" || reference.Column == "SiteSeries" {
				if reference.Available || reference.Diagnostic == "" || len(reference.Choices) != 0 {
					t.Fatal("historical invalid filter silently normalized", reference)
				}
				unavailable++
			}
		}
		if unavailable != 2 {
			t.Fatal("historical diagnostics lost dependent fields", unavailable)
		}
		if got, err := service.GetReferences(context.Background(), state.ContextID, "108050", form,
			TwoPageEntryReferenceFilters{historical.Zone, historical.SubZone}); got != nil || err == nil {
			t.Fatal("invalid new draft filters accepted", got, err)
		}
		edits := []SIVIParentCellEdit{siviParentEdit(t, historical.Original, "FieldNumber", metadataText("  unchanged BEC  "))}
		written, err := service.Save(context.Background(), state.ContextID, "108050", form,
			TwoPageParentEntryWrite{Original: historical.Original, Edits: edits, ProjectSource: historical.ProjectSource,
				WorkingSource: historical.WorkingSource, Acknowledgements: []TwoPageEntryCodeAcknowledgement{}})
		if err != nil || written == nil || written.ChangedCells != 1 {
			t.Fatal("unrelated save rejected unchanged historical filter", written, err)
		}
		if got, err := service.Restore(context.Background(), state.ContextID, "108050", form, written.HistoryID, AuditRestorePrune); got == nil || err != nil || got.RestoredRows != 1 {
			t.Fatal("historical facade restore failed", got, err)
		}
		service.readers.soil = nil
		if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form); got != nil || err == nil {
			t.Fatal("missing catalogue produced partial original", got, err)
		}
		service.readers = provider.readers
		if got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form); got == nil || err != nil {
			t.Fatal("failed snapshot prevented correction retry", got, err)
		}
	}
}

func TestTwoPageParentEntryServiceAtomicAcknowledgementsAndAuthority(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		provider, contexts, state := twoPageEntryProviderFixture(t, form)
		service, err := NewTwoPageParentEntryService(contexts, provider.readers, func(string) (string, bool) { return "true", true })
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := service.GetReferences(context.Background(), state.ContextID, "108050", form,
			TwoPageEntryReferenceFilters{metadataText("BG"), metadataText("xh1")})
		if err != nil {
			t.Fatal(err)
		}
		edits, acknowledgements := twoPageEntryApprovalDraft(t, snapshot)
		request := TwoPageParentEntryWrite{Original: snapshot.Original, Edits: edits,
			ProjectSource: snapshot.ProjectSource, WorkingSource: snapshot.WorkingSource,
			Acknowledgements: []TwoPageEntryCodeAcknowledgement{}}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		if got, err := service.Save(context.Background(), state.ContextID, "108050", form, request); got != nil || err == nil {
			t.Fatal("facade skipped optional literal acknowledgement", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
		for _, ack := range acknowledgements {
			request.Acknowledgements = append(request.Acknowledgements, TwoPageEntryCodeAcknowledgement{
				ContextID: ack.ContextID, Project: ack.Project, Plot: ack.Plot, Form: ack.Form, Table: ack.Table,
				RowID: ack.RowID, Column: ack.Column, Expected: ack.Expected, Value: ack.Value, Reference: ack.Reference})
		}
		masterRequest := request
		masterRequest.Edits = append(append([]SIVIParentCellEdit{}, edits...), siviParentEdit(t, snapshot.Original, "BECSiteUnit", metadataText("Master")))
		contexts.plots.mu.Lock()
		contexts.plots.currentUser = "ordinary user"
		contexts.plots.mu.Unlock()
		if got, err := service.Save(context.Background(), state.ContextID, "108050", form, masterRequest); got != nil || err == nil {
			t.Fatal("facade supplied a client-independent Master grant instead of actual user authority", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
		wire, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var decoded TwoPageParentEntryWrite
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatal("actual full definitions cannot cross strict facade", err)
		}
		written, err := service.Save(context.Background(), state.ContextID, "108050", form, decoded)
		if err != nil || written == nil || written.ChangedCells != len(edits) || written.HistoryID == "" {
			t.Fatal("facade retry did not atomically commit all source scopes", written, err)
		}
		current := databaseBytes(t, contexts.projects.sqlite.attachments)
		otherForm := "FS882-8x6XL"
		if otherForm == form {
			otherForm = "FS882-8x6XL-CHARS"
		}
		if got, err := service.Restore(context.Background(), state.ContextID, "108050", otherForm, written.HistoryID, AuditRestorePrune); got != nil || err == nil {
			t.Fatal("facade restored another variant's history", got, err)
		}
		assertProfileSUFiles(t, contexts, current)
		restored, err := service.Restore(context.Background(), state.ContextID, "108050", form, written.HistoryID, AuditRestorePrune)
		if err != nil || restored == nil || restored.RestoredRows != len(edits) {
			t.Fatal("facade did not restore the single mixed history", restored, err)
		}
		got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
		if err != nil || !reflect.DeepEqual(got.Original, snapshot.Original) {
			t.Fatal("facade restore changed original identity", err)
		}
	}
}

func TestTwoPageParentEntryServiceTypedProjectSaveRestore(t *testing.T) {
	for _, source := range []int{1, 2} {
		contexts, state, _, _, choice := siviProjectAssignmentWriteFixture(t, true, source, 3, "old")
		form := "FS882-8x6XL"
		provider := twoPageEntryCatalogueProviderFixture(t, form)
		service, err := NewTwoPageParentEntryService(contexts, provider.readers, func(string) (string, bool) { return "true", true })
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
		if err != nil {
			t.Fatal(err)
		}
		choice.ControlID = "form:" + form + "/ProjectID"
		selection := SIVIProjectSelection{ContextID: choice.ContextID, ControlID: choice.ControlID, Table: choice.Table,
			RowID: choice.RowID, Expected: choice.Expected, SourceOption: choice.SourceOption,
			MetadataAlias: choice.MetadataAlias, MetadataTable: choice.MetadataTable,
			MetadataColumns: choice.MetadataColumns, MetadataOriginal: choice.MetadataOriginal}
		request := TwoPageParentEntryWrite{Original: snapshot.Original, ProjectSource: snapshot.ProjectSource,
			WorkingSource: snapshot.WorkingSource, Acknowledgements: []TwoPageEntryCodeAcknowledgement{},
			Edits: []SIVIParentCellEdit{siviParentEdit(t, snapshot.Original, "FieldNumber", metadataText("  mixed  ")),
				siviParentEdit(t, snapshot.Original, "ProjectID", choice.MetadataOriginal.Cells[0])}}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		if got, err := service.Save(context.Background(), state.ContextID, "108050", form, request); got != nil || err == nil {
			t.Fatal("facade bypassed physical Project selection", got, err)
		}
		assertProfileSUFiles(t, contexts, before)
		request.ProjectSelection = &selection
		wire, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var decoded TwoPageParentEntryWrite
		if err := json.Unmarshal(wire, &decoded); err != nil {
			t.Fatal(err)
		}
		written, err := service.Save(context.Background(), state.ContextID, "108050", form, decoded)
		if err != nil || written == nil || written.ChangedCells != 2 || written.HistoryID == "" {
			t.Fatal("strict mixed Project facade could not retry", written, err)
		}
		if got, err := service.Restore(context.Background(), state.ContextID, "108050", form, written.HistoryID, AuditRestorePrune); err != nil || got == nil || got.RestoredRows != 2 {
			t.Fatal("mixed facade restore lost typed source history", got, err)
		}
	}
}

func TestTwoPageParentEntryServiceWALProjectChoicesRemainReadable(t *testing.T) {
	provider, contexts, state := twoPageEntryProviderFixture(t, "FS882-8x6XL")
	if err := contexts.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	master, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := master.Close(); err != nil {
			t.Error(err)
		}
	})
	var mode string
	if err := master.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatal("disposable WAL fixture unavailable", mode, err)
	}
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	service, err := NewTwoPageParentEntryService(contexts, provider.readers, func(string) (string, bool) { return "true", true })
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil || snapshot == nil || snapshot.ProjectChoices == nil || snapshot.ProjectChoices.SourceOption != 2 ||
		snapshot.ProjectAssignmentAvailable || !strings.Contains(snapshot.ProjectAssignmentDiagnostic, "rollback-journal") {
		t.Fatal("readable WAL choices incorrectly authorized assignment or disappeared", snapshot, err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestTwoPageParentEntryServiceStrictTransport(t *testing.T) {
	_, _, _, original, _ := twoPageWriteFixture(t, "FS882-8x6XL", false)
	empty := ""
	ack := TwoPageEntryCodeAcknowledgement{ContextID: original.ContextID, Project: original.Project, Plot: original.Plot,
		Form: original.Form, Table: "Sample_Env", RowID: "1", Column: "PlotType", Expected: metadataText("old"),
		Value: metadataText("  literal  "), Reference: SIVIParentSharedReference{Column: "PlotType",
			Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Item", DeclaredType: "TEXT"}},
				Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("")}}}},
			Choices: []SIVIParentSharedReferenceChoice{{RowID: "1", Code: nil, Description: &empty}}}}
	request := TwoPageParentEntryWrite{Original: original, Edits: []SIVIParentCellEdit{},
		ProjectSource: 1, WorkingSource: 1, Acknowledgements: []TwoPageEntryCodeAcknowledgement{ack}}
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded TwoPageParentEntryWrite
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("complete entry strict valid request changed NULL/empty", err)
	}
	for _, name := range []string{"original", "edits", "projectSource", "workingSource", "acknowledgements"} {
		var properties map[string]json.RawMessage
		if err := json.Unmarshal(wire, &properties); err != nil {
			t.Fatal(err)
		}
		for _, missing := range []bool{false, true} {
			if missing {
				delete(properties, name)
			} else {
				properties[name] = json.RawMessage("null")
			}
			bad, err := json.Marshal(properties)
			if err != nil {
				t.Fatal(err)
			}
			decoded = request
			if err := json.Unmarshal(bad, &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
				t.Fatal("missing/null entry property defaulted or receiver changed", name, err)
			}
		}
	}
	for _, bad := range []string{`null`, `{}`, string(wire) + `{}`,
		strings.Replace(string(wire), `"projectSource":1`, `"projectSource":1,"projectSource":1`, 1),
		strings.Replace(string(wire), `"projectSource":1`, `"projectSource":1,"clientMasterAllowed":true`, 1),
		strings.Replace(string(wire), `"edits":[]`, `"edits":[],"Edits":null`, 1),
		strings.Replace(string(wire), `"projectSource":1`, `"projectSource":1,"ProjectSource":2`, 1),
		strings.Replace(string(wire), `"projectSelection":null`, `"projectSelection":null,"ProjectSelection":null`, 1),
		strings.Replace(string(wire), `"selectable":false`, `"selectable":false,"Selectable":true`, 1)} {
		decoded = request
		if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("duplicate/unknown/malformed entry transport accepted", bad, err)
		}
	}
	for _, marker := range []string{`"  literal  "`, strconv.Quote(original.ContextID), `"column":"PlotType"`} {
		for _, replacement := range []string{`"\ud800"`, `"\udfff"`, `"\ud800x"`, `"\udfff\ud800"`, "\"" + string([]byte{0xff}) + "\""} {
			if strings.HasPrefix(marker, `"column"`) {
				replacement = `"column":` + replacement
			}
			bad := strings.Replace(string(wire), marker, replacement, 1)
			if bad == string(wire) {
				t.Fatal("transport corruption marker missing", marker)
			}
			decoded = request
			if err := json.Unmarshal([]byte(bad), &decoded); err == nil || !reflect.DeepEqual(decoded, request) {
				t.Fatal("raw invalid Unicode repaired or receiver replaced", err)
			}
		}
	}
	valid := strings.Replace(string(wire), `"  literal  "`, `"\ud83d\ude00"`, 1)
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal("valid surrogate pair rejected", err)
	}
	for _, omission := range []string{`"required":false,`, `"available":false,`, `"diagnostic":"",`,
		`"declaredType":"TEXT"`, `"code":null,`, `"description":"",`, `"selectable":false,`} {
		bad := strings.Replace(string(wire), omission, "", 1)
		decoded = request
		if bad == string(wire) || json.Unmarshal([]byte(bad), &decoded) == nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("nested acknowledgement property silently defaulted", omission)
		}
	}
	for _, marker := range []string{`{"rowId":"1","cells"`, `{"rowId":"1","code"`} {
		position := strings.LastIndex(string(wire), marker)
		if position < 0 {
			t.Fatal("nested acknowledgement marker missing", marker)
		}
		bad := string(wire[:position]) + strings.Replace(marker, `"rowId":"1",`, "", 1) + string(wire[position+len(marker):])
		decoded = request
		if bad == string(wire) || json.Unmarshal([]byte(bad), &decoded) == nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("nested definition/choice identity defaulted", marker)
		}
	}
	for _, bad := range []string{`null`, `{}`, `{"zone":null,"subZone":{"storage":"null"}}`,
		`{"zone":{"storage":"null"},"subZone":{"storage":"null"},"zone":{"storage":"null"}}`} {
		filters := TwoPageEntryReferenceFilters{metadataText("BG"), metadataText("xh1")}
		before := filters
		if err := json.Unmarshal([]byte(bad), &filters); err == nil || !reflect.DeepEqual(filters, before) {
			t.Fatal("malformed filters defaulted or receiver replaced", err)
		}
	}
}
