package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSIVIProjectChoiceRawProjectionPreservesDuplicateNULLAndEmpty(t *testing.T) {
	id, title, empty, blob := " DUP ", "  literal title  ", "", "00ff"
	original := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"ProjectTitle", "TEXT"}, {"ProjectID", "VARCHAR"}, {"Other", "BLOB"}},
		Rows: []ProjectMetadataRow{
			{"-7", []ProjectMetadataCell{{Storage: "text", Text: &title}, {Storage: "text", Text: &id}, {Storage: "blob", BlobHex: &blob}}},
			{"1", []ProjectMetadataCell{{Storage: "null"}, {Storage: "text", Text: &id}, {Storage: "null"}}},
			{"2", []ProjectMetadataCell{{Storage: "text", Text: &empty}, {Storage: "null"}, {Storage: "null"}}},
			{"3", []ProjectMetadataCell{{Storage: "null"}, {Storage: "text", Text: &empty}, {Storage: "null"}}},
		},
	}
	got, err := projectSIVIProjectChoices(context.Background(), original)
	expected := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"ProjectID", "VARCHAR"}, {"ProjectTitle", "TEXT"}},
		Rows: []ProjectMetadataRow{
			{"-7", []ProjectMetadataCell{{Storage: "text", Text: &id}, {Storage: "text", Text: &title}}},
			{"1", []ProjectMetadataCell{{Storage: "text", Text: &id}, {Storage: "null"}}},
			{"2", []ProjectMetadataCell{{Storage: "null"}, {Storage: "text", Text: &empty}}},
			{"3", []ProjectMetadataCell{{Storage: "text", Text: &empty}, {Storage: "null"}}},
		},
	}
	if err != nil || !reflect.DeepEqual(got, expected) {
		t.Fatal("source columns/order/types/duplicates/NULL/empty changed", got, err)
	}
	*got.Rows[0].Cells[0].Text = "changed"
	got.Columns[0].DeclaredType = "changed"
	if id != " DUP " || *got.Rows[1].Cells[0].Text != id || original.Columns[1].DeclaredType != "VARCHAR" {
		t.Fatal("caller changed original/duplicate ownership")
	}
}

func TestSIVIProjectChoiceProjectionRejectsUnavailableAndMalformedSource(t *testing.T) {
	for _, kind := range []string{"missing", "partial", "duplicate-row", "shadowed-rowid", "invalid-UTF", "invalid-cell"} {
		t.Run(kind, func(t *testing.T) {
			text := "literal"
			table := ProjectMetadataTable{
				Columns: []ProjectMetadataColumn{{"ProjectID", "TEXT"}, {"ProjectTitle", "TEXT"}},
				Rows:    []ProjectMetadataRow{{"1", []ProjectMetadataCell{{Storage: "text", Text: &text}, {Storage: "null"}}}},
			}
			switch kind {
			case "missing":
				table.Columns[0].Name = "Renamed"
			case "partial":
				table.Rows[0].Cells = table.Rows[0].Cells[:1]
			case "duplicate-row":
				table.Rows = append(table.Rows, table.Rows[0])
			case "shadowed-rowid":
				table.Columns = append(table.Columns, ProjectMetadataColumn{Name: "_ROWID_"})
				table.Rows[0].Cells = append(table.Rows[0].Cells, ProjectMetadataCell{Storage: "null"})
			case "invalid-UTF":
				text = string([]byte{0xff})
			case "invalid-cell":
				table.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "integer"}
			}
			got, err := projectSIVIProjectChoices(context.Background(), table)
			if err == nil || !reflect.DeepEqual(got, ProjectMetadataTable{}) {
				t.Fatal("invalid choice source returned partial result", got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := projectSIVIProjectChoices(ctx, ProjectMetadataTable{}); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, ProjectMetadataTable{}) {
		t.Fatal("cancelled projection returned success/partial output", got, err)
	}
}

func TestSIVIProjectChoiceOwnedYAMLSourceExternalProjectAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "project # metadata.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				next, err := service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
				state = next
			}
			mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Metadata;
				INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle) VALUES
					(1,'PROJECT','  project title  '),(2,'PROJECT',NULL),(3,NULL,''),(4,'',NULL)`)
			mutateContextFixture(t, service.projects.supportPaths["VMetaData"], `DELETE FROM ProjectMetadata;
				INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES
					('MASTER','  master title  '),(NULL,NULL),(NULL,''),('',NULL)`)
			for _, source := range []int{2, 1} {
				if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
					t.Fatal(err)
				}
				before := databaseBytes(t, service.projects.sqlite.attachments)
				config, err := os.ReadFile(service.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
				if err != nil || got == nil || len(got.Choices.Rows) != 4 ||
					got.ContextID != state.ContextID || got.Project != "Sample" || got.SourceOption != source {
					t.Fatal("YAML-selected physical choice read failed", got, err)
				}
				id, title, alias, table, label := "PROJECT", "  project title  ", "project", "Sample_Metadata", "Env"
				if source == 2 {
					id, title, alias, table, label = "MASTER", "  master title  ", "VMetaData", "ProjectMetadata", "Master"
				}
				if got.Alias != alias || !strings.EqualFold(got.Table, table) || got.Source != label ||
					*got.Choices.Rows[0].Cells[0].Text != id || *got.Choices.Rows[0].Cells[1].Text != title ||
					got.Choices.Rows[1].Cells[1].Storage != "null" ||
					got.Choices.Rows[2].Cells[0].Storage != "null" || *got.Choices.Rows[2].Cells[1].Text != "" ||
					*got.Choices.Rows[3].Cells[0].Text != "" || got.Choices.Rows[3].Cells[1].Storage != "null" ||
					got.Choices.Rows[0].RowID == got.Choices.Rows[1].RowID || got.Choices.Rows[1].RowID == got.Choices.Rows[2].RowID {
					t.Fatal("source fallback, duplicate collapse or NULL/empty repair", got)
				}
				if source == 1 && (got.Choices.Rows[1].Cells[0].Storage != "text" || *got.Choices.Rows[1].Cells[0].Text != id) ||
					source == 2 && got.Choices.Rows[1].Cells[0].Storage != "null" {
					t.Fatal("project duplicate ID or master duplicate NULL definition changed", got)
				}
				retry, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
				if err != nil || !reflect.DeepEqual(retry, got) {
					t.Fatal("repeat read changed physical choices", retry, err)
				}
				*got.Choices.Rows[0].Cells[0].Text = "changed"
				again, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
				if err != nil || !reflect.DeepEqual(again, retry) {
					t.Fatal("caller mutated persistent reader state", again, err)
				}
				assertProfileSUFiles(t, service, before)
				after, err := os.ReadFile(service.projects.preferences.path)
				if err != nil || !reflect.DeepEqual(config, after) {
					t.Fatal("choice read changed YAML bytes", err)
				}
			}
		})
	}
}

func TestSIVIProjectChoiceOwnedRejectsInvalidRetainedPreferenceAndStaleContext(t *testing.T) {
	service, state := contextServiceFixture(t)
	for _, source := range []any{nil, "2", 0, 3, 1.5, true} {
		if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := service.readSIVIProjectChoices(context.Background(), state.ContextID); err == nil || got != nil ||
			!strings.Contains(err.Error(), "ProjectIdSource") {
			t.Fatal("unsupported retained preference was silently defaulted", source, got, err)
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(after, config) {
			t.Fatal("invalid workflow preference was discarded", source, err)
		}
	}
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readSIVIProjectChoices(context.Background(), "stale"); err == nil || got != nil {
		t.Fatal("stale context returned metadata choices", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.readSIVIProjectChoices(ctx, state.ContextID); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled context returned metadata choices", got, err)
	}
	if got, err := service.readSIVIProjectChoices(context.Background(), state.ContextID); err != nil || got == nil {
		t.Fatal("rejected preferences/requests discarded pinned coordinator", got, err)
	}
}

func TestSIVIProjectChoiceOwnedMissingSourceCannotFallBack(t *testing.T) {
	service, state := contextServiceFixture(t)
	mutateContextFixture(t, state.ProjectPath, `ALTER TABLE Sample_Metadata RENAME TO Archived_Metadata;
		CREATE VIEW Sample_Metadata AS SELECT * FROM Archived_Metadata`)
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 1}); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.readSIVIProjectChoices(context.Background(), state.ContextID); err == nil || got != nil ||
		!strings.Contains(err.Error(), "physical metadata table") {
		t.Fatal("substituted view or master fallback impersonated local source", got, err)
	}
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	if got, err := service.readSIVIProjectChoices(context.Background(), state.ContextID); err != nil || got == nil || got.Source != "Master" {
		t.Fatal("unavailable local source disabled independent master source", got, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVIProjectChoiceOwnedBlockedCancellationKeepsAttachments(t *testing.T) {
	service, state := contextServiceFixture(t)
	if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 2}); err != nil {
		t.Fatal(err)
	}
	before, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
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
	if got, err := service.readSIVIProjectChoices(ctx, state.ContextID); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked master snapshot returned partial choices", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	retry, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(before, retry) {
		t.Fatal("cancelled choice snapshot discarded owned attachments", retry, err)
	}
	assertProfileSUFiles(t, service, files)
}
