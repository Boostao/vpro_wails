package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestSIVICategoricalSourceScopeAndValueList(t *testing.T) {
	var layout fs882layout.Layout
	if err := json.Unmarshal(siviParentLayoutJSON, &layout); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []struct {
		binding, control, source, list string
		distinct                       bool
	}{
		{"SnowCoverregime", "SnowCoverregime", "Table/Query", "SnowCoverRegime", false},
		{"SV_RootZoneTexture", "RootZoneTexture", "Table/Query", "SoilTexture", true},
		{"SV_AhorizonType", "AhorizonType", "Value List", "", false},
	} {
		got, err := siviCategoricalSource(spec.binding)
		if err != nil || got.ControlName != spec.control || got.Binding != spec.binding ||
			got.SourceType != spec.source || got.ListName != spec.list || got.SourceDistinctRow != spec.distinct {
			t.Fatal("literal source scope changed", spec, got, err)
		}
		found := false
		for _, field := range layout.Forms[0].Fields {
			if field.Binding == spec.binding {
				found = true
				if field.ControlName != spec.control || field.ControlID != got.ControlID ||
					field.Properties["RowSourceType"].Value != spec.source {
					t.Fatal("source control/type changed", field)
				}
				if spec.source == "Value List" && field.Properties["RowSource"].Value != `Ah;Ae;""` {
					t.Fatal("source explicit blank changed", field)
				}
			}
		}
		if !found {
			t.Fatal("source binding absent", spec)
		}
	}
	first, err := siviCategoricalSource("SV_AhorizonType")
	if err != nil || !reflect.DeepEqual(first.Values, []ProjectMetadataCell{metadataText("Ah"), metadataText("Ae"), metadataText("")}) ||
		first.Alias != "" || first.Table != "" || first.Ordering != "source-order" {
		t.Fatal("source value list became NULL or physical definitions", first, err)
	}
	*first.Values[0].Text = "caller"
	second, err := siviCategoricalSource("SV_AhorizonType")
	if err != nil || *second.Values[0].Text != "Ah" || *first.Values[1].Text != "Ae" {
		t.Fatal("value-list values share mutable ownership", second, err)
	}
	for _, binding := range []string{"", "SnowCoverRegime", "SV_StandHeight", "SV_AhorizonType ", "PlotType"} {
		if got, err := siviCategoricalSource(binding); err == nil || got != nil {
			t.Fatal("unsupported binding accepted", binding, got, err)
		}
	}
}

func TestSIVICategoricalOwnedPhysicalDefinitionsAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # categorical project.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				state, err = service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`DELETE FROM USysTableOfLists WHERE ListName COLLATE BINARY IN ('SnowCoverRegime','SoilTexture');
				INSERT INTO USysTableOfLists(rowid,ListName,ItemOrder,Item,ItemDescription,Validate,Flag) VALUES
				(900003,'SnowCoverRegime',NULL,NULL,'Null',-1,-1),
				(900001,'SnowCoverRegime',3.5,'',NULL,NULL,NULL),
				(900002,'SnowCoverRegime',3.5,'',NULL,NULL,NULL),
				(900004,'SnowCoverRegime',4.25,'LONG',X'00FF',0,0),
				(900005,'SnowCoverRegime ',1,'X','excluded',NULL,NULL),
				(900011,'SoilTexture',1.1,NULL,'null',NULL,NULL),
				(900012,'SoilTexture',1.2,'FSL','',NULL,NULL),
				(900013,'SoilTexture',1.2,'FSL','',NULL,NULL);`)
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range []string{"SnowCoverregime", "SV_RootZoneTexture", "SV_AhorizonType"} {
				got, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, binding)
				if err != nil || got == nil || got.ContextID != state.ContextID || got.Project != "Sample" || got.Binding != binding {
					t.Fatal("owned source provenance changed", binding, got, err)
				}
				if binding == "SV_AhorizonType" {
					if !reflect.DeepEqual(got.Values, []ProjectMetadataCell{metadataText("Ah"), metadataText("Ae"), metadataText("")}) ||
						len(got.Definitions.Rows) != 0 {
						t.Fatal("value list repaired blank or invented rowids", got)
					}
					continue
				}
				if got.Alias != "VLists" || got.Table != "USysTableOfLists" ||
					got.Membership != "literal-binary-list-name" || got.Ordering != "sqlite-ItemOrder-then-physical-rowid" {
					t.Fatal("source adaptation lost", got)
				}
				expectedIDs := []string{"900003", "900001", "900002", "900004"}
				expectedItems := []ProjectMetadataCell{{Storage: "null"}, metadataText(""), metadataText(""), metadataText("LONG")}
				blob := "00ff"
				expectedDescriptions := []ProjectMetadataCell{metadataText("Null"), {Storage: "null"}, {Storage: "null"}, {Storage: "blob", BlobHex: &blob}}
				expectedOrder := []ProjectMetadataCell{{Storage: "null"}, siviReal(3.5), siviReal(3.5), siviReal(4.25)}
				if binding == "SV_RootZoneTexture" {
					expectedIDs = []string{"900011", "900012", "900013"}
					expectedItems = []ProjectMetadataCell{{Storage: "null"}, metadataText("FSL"), metadataText("FSL")}
					expectedDescriptions = []ProjectMetadataCell{metadataText("null"), metadataText(""), metadataText("")}
					expectedOrder = []ProjectMetadataCell{siviReal(1.1), siviReal(1.2), siviReal(1.2)}
				}
				expectedColumns := []ProjectMetadataColumn{
					{"ListName", "TEXT"}, {"ListFilter", "TEXT"}, {"ItemOrder", "REAL"},
					{"Item", "TEXT"}, {"ItemDescription", "TEXT"}, {"FieldUsedIn", "TEXT"},
					{"ValidateLoops", "TEXT"}, {"Validate", "BOOLEAN"}, {"Note", "TEXT"}, {"Flag", "BOOLEAN"},
				}
				if len(got.Definitions.Rows) != len(expectedIDs) || !reflect.DeepEqual(got.Definitions.Columns, expectedColumns) {
					t.Fatal("physical definitions collapsed or dropped metadata", got)
				}
				for i, id := range expectedIDs {
					if got.Definitions.Rows[i].RowID != id ||
						!reflect.DeepEqual(metadataTestCell(t, got.Definitions, id, "Item"), expectedItems[i]) ||
						!reflect.DeepEqual(metadataTestCell(t, got.Definitions, id, "ItemDescription"), expectedDescriptions[i]) ||
						!reflect.DeepEqual(metadataTestCell(t, got.Definitions, id, "ItemOrder"), expectedOrder[i]) {
						t.Fatal("independent physical order/NULL/empty/duplicate/raw expectation changed", got)
					}
					if !reflect.DeepEqual(metadataTestCell(t, got.Definitions, id, "ListName"), metadataText(got.ListName)) {
						t.Fatal("source literal list identity changed", got)
					}
					for _, column := range []string{"ListFilter", "FieldUsedIn", "ValidateLoops", "Note"} {
						if !reflect.DeepEqual(metadataTestCell(t, got.Definitions, id, column), ProjectMetadataCell{Storage: "null"}) {
							t.Fatal("nullable reference metadata changed", column, got)
						}
					}
				}
				if binding == "SnowCoverregime" &&
					(!reflect.DeepEqual(metadataTestCell(t, got.Definitions, "900003", "Validate"), metadataInteger("-1")) ||
						!reflect.DeepEqual(metadataTestCell(t, got.Definitions, "900003", "Flag"), metadataInteger("-1"))) {
					t.Fatal("raw reference BOOLEAN was normalized", got)
				}
				*got.Definitions.Rows[1].Cells[0].Text = "caller"
				got.Definitions.Columns[0].Name = "caller"
				again, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, binding)
				if err != nil || again.Definitions.Columns[0].Name != "ListName" ||
					*again.Definitions.Rows[1].Cells[0].Text == "caller" {
					t.Fatal("caller changed source ownership", again, err)
				}
			}
			assertProfileSUFiles(t, service, files)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("choices changed runtime YAML", err)
			}
		})
	}
}

func TestSIVICategoricalOwnedUnavailableSourcesAndRetry(t *testing.T) {
	for _, kind := range []string{"missing-list", "missing-table", "view", "missing-column", "shadow-rowid", "identifier-recase"} {
		t.Run(kind, func(t *testing.T) {
			service, state := contextServiceFixture(t)
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			commands := map[string]string{
				"missing-list":  `DELETE FROM USysTableOfLists WHERE ListName='SnowCoverRegime'`,
				"missing-table": `ALTER TABLE USysTableOfLists RENAME TO ArchiveLists`,
				"view": `ALTER TABLE USysTableOfLists RENAME TO ArchiveLists;
					CREATE VIEW USysTableOfLists AS SELECT * FROM ArchiveLists`,
				"missing-column": `ALTER TABLE USysTableOfLists RENAME COLUMN Item TO RenamedItem`,
				"shadow-rowid":   `ALTER TABLE USysTableOfLists ADD COLUMN oid INTEGER`,
				"identifier-recase": `ALTER TABLE USysTableOfLists RENAME TO ArchiveLists;
					ALTER TABLE ArchiveLists RENAME TO usystableoflists`,
			}
			_, err = db.Exec(commands[kind])
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			got, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SnowCoverregime")
			if kind == "identifier-recase" {
				if err != nil || got == nil || got.Table != "usystableoflists" {
					t.Fatal("actual physical identifier was not retained", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("unavailable/malformed physical source returned success", got, err)
			}
			ah, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SV_AhorizonType")
			if err != nil || ah == nil || len(ah.Values) != 3 {
				t.Fatal("unrelated value-list source silently fell back or became unavailable", ah, err)
			}
			assertProfileSUFiles(t, service, files)
		})
	}
}

func TestSIVICategoricalOwnedCancellationStaleAndBlockedRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	parent, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SV_RootZoneTexture")
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, binding := range []string{"SnowCoverregime", "SV_RootZoneTexture", "SV_AhorizonType"} {
		if got, err := service.readSIVICategoricalChoices(context.Background(), "stale", binding); err == nil || got != nil {
			t.Fatal("stale context returned reference choices", got, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := service.readSIVICategoricalChoices(ctx, state.ContextID, binding); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled reference request returned choices", got, err)
		}
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if got, err := service.readSIVICategoricalChoices(ctx, state.ContextID, "SV_RootZoneTexture"); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked choices returned success", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	again, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SV_RootZoneTexture")
	if err != nil || !reflect.DeepEqual(parent, again) {
		t.Fatal("cancelled read discarded pinned attachments", again, err)
	}
	if got, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, strings.Repeat("x", 10)); err == nil || got != nil {
		t.Fatal("unknown source returned a fallback", got, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVICategoricalGeneratedSchemaRejectsBeforeReadAndRetries(t *testing.T) {
	for _, column := range []string{"rowid", "_rowid_", "oid", "ExtraGenerated"} {
		t.Run(column, func(t *testing.T) {
			service, state := contextServiceFixture(t)
			original, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SnowCoverregime")
			if err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='SnowCoverRegime'
				AND rowid <> (SELECT rowid FROM USysTableOfLists WHERE ListName='SnowCoverRegime' ORDER BY ItemOrder,rowid LIMIT 1)`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("ALTER TABLE USysTableOfLists ADD COLUMN " + quoteHeaderIdentifier(column) + " INTEGER GENERATED ALWAYS AS (42) VIRTUAL"); err != nil {
				t.Fatal(err)
			}
			files := databaseBytes(t, service.projects.sqlite.attachments)
			if got, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SnowCoverregime"); err == nil || got != nil ||
				!strings.Contains(err.Error(), "unshadowed physical identities and ordinary columns") {
				t.Fatal("generated/hidden schema escaped the pre-read guard", got, err)
			}
			assertProfileSUFiles(t, service, files)
			if _, err := db.Exec("ALTER TABLE USysTableOfLists DROP COLUMN " + quoteHeaderIdentifier(column)); err != nil {
				t.Fatal(err)
			}
			files = databaseBytes(t, service.projects.sqlite.attachments)
			again, err := service.readSIVICategoricalChoices(context.Background(), state.ContextID, "SnowCoverregime")
			if err != nil || again == nil || len(again.Definitions.Rows) != 1 ||
				!reflect.DeepEqual(again.Definitions.Rows[0], original.Definitions.Rows[0]) {
				t.Fatal("rejected schema discarded attachments or physical provenance", again, err)
			}
			assertProfileSUFiles(t, service, files)
		})
	}
}
