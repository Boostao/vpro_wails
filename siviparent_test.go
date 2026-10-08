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

func siviParentTables(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("resources", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	plot := "108050"
	env, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Env", "PlotNumber", &plot, "")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Admin", "Plot", &plot, "")
	if err != nil || len(env.Rows) != 1 || len(admin.Rows) != 1 {
		t.Fatal("source fixture requires one physical Env/Admin pair", err)
	}
	return env, admin
}

func TestSIVIParentRawBindingsPairsAndCloneSafety(t *testing.T) {
	env, admin := siviParentTables(t)
	env.Rows = append(env.Rows, env.Rows[0])
	env.Rows[1].RowID = "-9"
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "9223372036854775807"
	got, err := projectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil || got == nil || len(got.Bindings) != 78 || len(got.Rows) != 4 {
		t.Fatal("source bindings or physical duplicate pairs collapsed", got, err)
	}
	if got.ContextID != "owned" || got.Form != "frmSIVIsite" || got.Query != "USysEnv" ||
		got.Membership != "literal-binary-inner-pairs" || got.EnvTable != "Sample_Env" || got.AdminTable != "Sample_Admin" {
		t.Fatal("source/context provenance changed", got)
	}
	implicit := got.Bindings[77]
	if !implicit.Implicit || implicit.ControlID != "" || implicit.Binding != "SpeciesListComplete" ||
		implicit.Table != "Sample_Env" || got.EnvColumns[implicit.Column].Name != "SpeciesListComplete" {
		t.Fatal("implicit source target was presented as a directly bound control", implicit)
	}
	if !reflect.DeepEqual(got.EnvColumns, env.Columns) || !reflect.DeepEqual(got.AdminColumns, admin.Columns) ||
		!reflect.DeepEqual(got.Rows[0], siviParentRow{env.Rows[0], admin.Rows[0]}) ||
		!reflect.DeepEqual(got.Rows[1], siviParentRow{env.Rows[0], admin.Rows[1]}) ||
		!reflect.DeepEqual(got.Rows[2], siviParentRow{env.Rows[1], admin.Rows[0]}) ||
		!reflect.DeepEqual(got.Rows[3], siviParentRow{env.Rows[1], admin.Rows[1]}) {
		t.Fatal("raw columns/types/row identities or deterministic pair order changed", got)
	}
	*got.Rows[0].Env.Cells[0].Text = "changed"
	got.EnvColumns[0].DeclaredType = "changed"
	got.Bindings[0].Binding = "changed"
	if *got.Rows[1].Env.Cells[0].Text != "108050" || *env.Rows[0].Cells[0].Text != "108050" {
		t.Fatal("duplicate pair shares mutable cell ownership")
	}
	again, err := projectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil || again.Bindings[0].Binding == "changed" || again.EnvColumns[0].DeclaredType == "changed" {
		t.Fatal("caller mutated cached source metadata or original columns", again, err)
	}
}

func TestSIVIParentLiteralJoinOrphansAndUnavailableSchema(t *testing.T) {
	for _, identities := range [][2]string{{"case", "Case"}, {"é", "É"}, {"é", "e\u0301"}, {"space", "space "}, {" 'hé#' ", " 'hé#' "}} {
		env, admin := siviParentTables(t)
		env.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &identities[0]}
		admin.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &identities[1]}
		got, err := projectSIVIParent(context.Background(), "owned", "Sample", identities[0], env, admin)
		expected := 0
		if identities[0] == identities[1] {
			expected = 1
		}
		if err != nil || got == nil || len(got.Rows) != expected {
			t.Fatal("literal case/Unicode/space join adaptation changed", identities, got, err)
		}
	}
	for _, plot := range []string{"108050 ", "108050x", "10805", " 108050", "é", "É", "missing"} {
		t.Run(plot, func(t *testing.T) {
			env, admin := siviParentTables(t)
			env.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &plot}
			got, err := projectSIVIParent(context.Background(), "owned", "Sample", plot, env, admin)
			if err != nil || got == nil || len(got.Rows) != 0 {
				t.Fatal("orphan/case/space join was repaired or completed", got, err)
			}
		})
	}
	for _, missing := range []string{"env", "admin"} {
		env, admin := siviParentTables(t)
		if missing == "env" {
			env.Rows = nil
		} else {
			admin.Rows = nil
		}
		got, err := projectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
		if err != nil || got == nil || got.Rows == nil || len(got.Rows) != 0 {
			t.Fatal("inner join invented a missing physical parent", got, err)
		}
	}
	for _, kind := range []string{"missing-binding", "ambiguous-binding", "shadow-rowid", "duplicate-rowid", "partial-row", "malformed-cell", "nontext-join", "null-join"} {
		t.Run(kind, func(t *testing.T) {
			env, admin := siviParentTables(t)
			switch kind {
			case "missing-binding":
				env.Columns[0].Name = "RenamedPlotNumber"
			case "ambiguous-binding":
				admin.Columns = append(admin.Columns, env.Columns[0])
				admin.Rows[0].Cells = append(admin.Rows[0].Cells, env.Rows[0].Cells[0])
			case "shadow-rowid":
				env.Columns = append(env.Columns, ProjectMetadataColumn{Name: "RoWiD"})
				env.Rows[0].Cells = append(env.Rows[0].Cells, ProjectMetadataCell{Storage: "null"})
			case "duplicate-rowid":
				env.Rows = append(env.Rows, env.Rows[0])
			case "partial-row":
				env.Rows[0].Cells = env.Rows[0].Cells[:1]
			case "malformed-cell":
				env.Rows[0].Cells[1] = ProjectMetadataCell{Storage: "text"}
			case "nontext-join":
				id := "108050"
				env.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "integer", Integer: &id}
			case "null-join":
				env.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"}
			}
			got, err := projectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
			if kind == "null-join" {
				if err != nil || got == nil || len(got.Rows) != 0 {
					t.Fatal("NULL join fabricated a parent", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("unsupported/ambiguous physical source returned success", got, err)
			}
		})
	}
	env, admin := siviParentTables(t)
	for _, plot := range []string{"", "bad\x00", string([]byte{0xff})} {
		if got, err := projectSIVIParent(context.Background(), "owned", "Sample", plot, env, admin); err == nil || got != nil {
			t.Fatal("invalid literal identity returned partial output", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := projectSIVIParent(ctx, "owned", "Sample", "108050", env, admin); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled projection returned partial output", got, err)
	}
}

func TestSIVIParentOwnedRawStorageExternalContextAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # parent.db")
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
			owner := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`DROP INDEX uidx_Sample_Admin_PlotNumber;
				UPDATE Sample_Env SET SpeciesListComplete=2,SV_FloodPlain=-1,SV_StandHeight=-2,
					SV_StandHeightEstMeas='??',SV_RootZoneTexture=?,SV_AhorizonDepth=NULL,SiteNotes=X'0001',VegNotes=''
					WHERE PlotNumber='108050';
				UPDATE Sample_Admin SET PlotType='  unknown  ',StartDate=NULL WHERE Plot='108050';
				INSERT INTO Sample_Admin(Plot,PlotType) VALUES('108050',''),('Case','Ground');
				INSERT INTO Sample_Env(PlotNumber) VALUES('case'),('ORPHAN');`, strings.Repeat("x", 300))
			if err := errors.Join(err, db.Close()); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, owner.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil || got == nil || len(got.Rows) != 2 || got.ContextID != state.ContextID {
				t.Fatal("owned duplicate parent read failed", got, err)
			}
			values := map[string]ProjectMetadataCell{}
			for i, column := range got.EnvColumns {
				values[column.Name] = got.Rows[0].Env.Cells[i]
			}
			if *values["SpeciesListComplete"].Integer != "2" || *values["SV_FloodPlain"].Integer != "-1" ||
				*values["SV_StandHeight"].Real != -2 || *values["SV_StandHeightEstMeas"].Text != "??" ||
				*values["SV_RootZoneTexture"].Text != strings.Repeat("x", 300) ||
				*values["SiteNotes"].BlobHex != "0001" || *values["VegNotes"].Text != "" ||
				values["SV_AhorizonDepth"].Storage != "null" {
				t.Fatal("raw historical BOOLEAN/text/real/blob/empty/NULL was normalized", values)
			}
			for i, column := range got.AdminColumns {
				if column.Name == "StartDate" && got.Rows[0].Admin.Cells[i].Storage != "null" ||
					column.Name == "PlotType" && *got.Rows[0].Admin.Cells[i].Text != "  unknown  " {
					t.Fatal("Admin identity/NULL was completed", got.Rows[0].Admin)
				}
			}
			for _, plot := range []string{"case", "ORPHAN", "108050 "} {
				orphan, err := service.readSIVIParent(context.Background(), state.ContextID, plot)
				if err != nil || orphan == nil || len(orphan.Rows) != 0 {
					t.Fatal("physical orphan/BINARY membership changed", orphan, err)
				}
			}
			if stale, err := service.readSIVIParent(context.Background(), "stale", "108050"); err == nil || stale != nil {
				t.Fatal("stale context returned parent data", stale, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if cancelled, err := service.readSIVIParent(ctx, state.ContextID, "108050"); !errors.Is(err, context.Canceled) || cancelled != nil {
				t.Fatal("cancelled owned reader returned data", cancelled, err)
			}
			retry, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(retry, got) {
				t.Fatal("rejected request discarded attachments", retry, err)
			}
			assertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("raw read changed YAML configuration", err)
			}
		})
	}
}

func TestSIVIParentOwnedBlockedCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	before, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	owner := service.projects.sqlite
	files := databaseBytes(t, owner.attachments)
	db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
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
	if got, err := service.readSIVIParent(ctx, state.ContextID, "108050"); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked snapshot returned partial/success-shaped parent", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	again, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("in-flight cancellation discarded pinned coordinator", again, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIParentOwnedRejectsPhysicalViewWithoutWrites(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ALTER TABLE Sample_Admin RENAME TO Archived_Admin;
		CREATE VIEW Sample_Admin AS SELECT * FROM Archived_Admin`)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	got, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err == nil || got != nil || !strings.Contains(err.Error(), "physical project table") {
		t.Fatal("view impersonated the original physical parent", got, err)
	}
	assertProfileSUFiles(t, service, before)
}
