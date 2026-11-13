package sourceform

import (
	"github.com/boostao/vpro-wails/internal/fs882layout"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSIVIParentRawBindingsPairsAndCloneSafety(t *testing.T) {
	env, admin := vprotest.SiviParentTables(t)
	env.Rows = append(env.Rows, env.Rows[0])
	env.Rows[1].RowID = "-9"
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "9223372036854775807"
	got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
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
		!reflect.DeepEqual(got.Rows[0], SiviParentRow{env.Rows[0], admin.Rows[0]}) ||
		!reflect.DeepEqual(got.Rows[1], SiviParentRow{env.Rows[0], admin.Rows[1]}) ||
		!reflect.DeepEqual(got.Rows[2], SiviParentRow{env.Rows[1], admin.Rows[0]}) ||
		!reflect.DeepEqual(got.Rows[3], SiviParentRow{env.Rows[1], admin.Rows[1]}) {
		t.Fatal("raw columns/types/row identities or deterministic pair order changed", got)
	}
	*got.Rows[0].Env.Cells[0].Text = "changed"
	got.EnvColumns[0].DeclaredType = "changed"
	got.Bindings[0].Binding = "changed"
	if *got.Rows[1].Env.Cells[0].Text != "108050" || *env.Rows[0].Cells[0].Text != "108050" {
		t.Fatal("duplicate pair shares mutable cell ownership")
	}
	again, err := ProjectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil || again.Bindings[0].Binding == "changed" || again.EnvColumns[0].DeclaredType == "changed" {
		t.Fatal("caller mutated cached source metadata or original columns", again, err)
	}
}

func TestSIVIParentLiteralJoinOrphansAndUnavailableSchema(t *testing.T) {
	for _, identities := range [][2]string{{"case", "Case"}, {"é", "É"}, {"é", "e\u0301"}, {"space", "space "}, {" 'hé#' ", " 'hé#' "}} {
		env, admin := vprotest.SiviParentTables(t)
		env.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "text", Text: &identities[0]}
		admin.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "text", Text: &identities[1]}
		got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", identities[0], env, admin)
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
			env, admin := vprotest.SiviParentTables(t)
			env.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "text", Text: &plot}
			got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", plot, env, admin)
			if err != nil || got == nil || len(got.Rows) != 0 {
				t.Fatal("orphan/case/space join was repaired or completed", got, err)
			}
		})
	}
	for _, missing := range []string{"env", "admin"} {
		env, admin := vprotest.SiviParentTables(t)
		if missing == "env" {
			env.Rows = nil
		} else {
			admin.Rows = nil
		}
		got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
		if err != nil || got == nil || got.Rows == nil || len(got.Rows) != 0 {
			t.Fatal("inner join invented a missing physical parent", got, err)
		}
	}
	for _, kind := range []string{"missing-binding", "ambiguous-binding", "shadow-rowid", "duplicate-rowid", "partial-row", "malformed-cell", "nontext-join", "null-join"} {
		t.Run(kind, func(t *testing.T) {
			env, admin := vprotest.SiviParentTables(t)
			switch kind {
			case "missing-binding":
				env.Columns[0].Name = "RenamedPlotNumber"
			case "ambiguous-binding":
				admin.Columns = append(admin.Columns, env.Columns[0])
				admin.Rows[0].Cells = append(admin.Rows[0].Cells, env.Rows[0].Cells[0])
			case "shadow-rowid":
				env.Columns = append(env.Columns, vprocore.ProjectMetadataColumn{Name: "RoWiD"})
				env.Rows[0].Cells = append(env.Rows[0].Cells, vprocore.ProjectMetadataCell{Storage: "null"})
			case "duplicate-rowid":
				env.Rows = append(env.Rows, env.Rows[0])
			case "partial-row":
				env.Rows[0].Cells = env.Rows[0].Cells[:1]
			case "malformed-cell":
				env.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "text"}
			case "nontext-join":
				id := "108050"
				env.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &id}
			case "null-join":
				env.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "null"}
			}
			got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
			if kind == "null-join" {
				if err != nil || got == nil || len(got.Rows) != 0 {
					t.Fatal("NULL join fabricated a parent", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("unsupported/ambiguous physical source returned success", got, err)
			}
		})
	}
	env, admin := vprotest.SiviParentTables(t)
	for _, plot := range []string{"", "bad\x00", string([]byte{0xff})} {
		if got, err := ProjectSIVIParent(context.Background(), "owned", "Sample", plot, env, admin); err == nil || got != nil {
			t.Fatal("invalid literal identity returned partial output", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := ProjectSIVIParent(ctx, "owned", "Sample", "108050", env, admin); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled projection returned partial output", got, err)
	}
}

func TestSIVIParentOwnedRawStorageExternalContextAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := vprotest.ContextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # parent.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := vprotest.ContextSelection(state)
				selection.ProjectPath = path
				next, err := service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
				state = next
			}
			owner := service.Projects.Sqlite
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(owner.Attachments["project"], "rw"))
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
			before := vprotest.DatabaseBytes(t, owner.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil || got == nil || len(got.Rows) != 2 || got.ContextID != state.ContextID {
				t.Fatal("owned duplicate parent read failed", got, err)
			}
			values := map[string]vprocore.ProjectMetadataCell{}
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
				orphan, err := ReadSIVIParent(service, context.Background(), state.ContextID, plot)
				if err != nil || orphan == nil || len(orphan.Rows) != 0 {
					t.Fatal("physical orphan/BINARY membership changed", orphan, err)
				}
			}
			if stale, err := ReadSIVIParent(service, context.Background(), "stale", "108050"); err == nil || stale != nil {
				t.Fatal("stale context returned parent data", stale, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if cancelled, err := ReadSIVIParent(service, ctx, state.ContextID, "108050"); !errors.Is(err, context.Canceled) || cancelled != nil {
				t.Fatal("cancelled owned reader returned data", cancelled, err)
			}
			retry, err := ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
			if err != nil || !reflect.DeepEqual(retry, got) {
				t.Fatal("rejected request discarded attachments", retry, err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("raw read changed YAML configuration", err)
			}
		})
	}
}

func TestSIVIParentOwnedBlockedCancellationAndRetry(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	before, err := ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	owner := service.Projects.Sqlite
	files := vprotest.DatabaseBytes(t, owner.Attachments)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(owner.Attachments["project"], "rw"))
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
	if got, err := ReadSIVIParent(service, ctx, state.ContextID, "108050"); !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked snapshot returned partial/success-shaped parent", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	again, err := ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("in-flight cancellation discarded pinned coordinator", again, err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
}

func TestSIVIParentOwnedRejectsPhysicalViewWithoutWrites(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ALTER TABLE Sample_Admin RENAME TO Archived_Admin;
		CREATE VIEW Sample_Admin AS SELECT * FROM Archived_Admin`)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	got, err := ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err == nil || got != nil || !strings.Contains(err.Error(), "physical project table") {
		t.Fatal("view impersonated the original physical parent", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSourceSIVIParentBindingsRejectsMalformedCatalogue(t *testing.T) {
	for _, kind := range []string{"duplicate-binding", "missing-control", "writable", "mapped", "implicit-direct", "count"} {
		t.Run(kind, func(t *testing.T) {
			layout, err := SiviParentSourceLayout()
			if err != nil {
				t.Fatal(err)
			}
			fields := []fs882layout.Field{}
			for _, field := range layout.Forms[0].Fields {
				if field.Binding != "" {
					fields = append(fields, field)
				}
			}
			switch kind {
			case "duplicate-binding":
				fields[1].Binding = strings.ToUpper(fields[0].Binding)
			case "missing-control":
				fields[0].ControlID = ""
			case "writable":
				fields[0].ReadOnly = false
			case "mapped":
				fields[0].Implementation = "implemented"
			case "implicit-direct":
				fields[0].Binding = "SpeciesListComplete"
			case "count":
				fields = fields[1:]
			}
			if got, err := SourceSIVIParentBindings(fields, 77); err == nil || got != nil {
				t.Fatal("invalid source catalogue returned success", got, err)
			}
		})
	}
}

func TestSourceParentProjectionStorageEqualityMatchesExactRealBits(t *testing.T) {
	env, admin := vprotest.SiviParentTables(t)
	posZero := 0.0
	negZero := math.Copysign(0.0, -1.0)
	for i, col := range env.Columns {
		if col.Name == "SlopeGradient" {
			env.Rows[0].Cells[i] = vprotest.SiviReal(posZero)
		}
	}
	original, err := ProjectSIVIParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var cloned SiviParentProjection
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	if !SourceParentProjectionStorageEqual(original, &cloned) || !SourceParentProjectionStorageEqual(nil, nil) {
		t.Fatal("equal sources were rejected")
	}
	for i, col := range cloned.EnvColumns {
		if col.Name == "SlopeGradient" {
			cloned.Rows[0].Env.Cells[i] = vprotest.SiviReal(negZero)
		}
	}
	if SourceParentProjectionStorageEqual(original, &cloned) {
		t.Fatal("signed-zero bit difference compared equal")
	}
	cloned = *original
	cloned.Rows = append([]SiviParentRow{}, original.Rows...)
	cloned.Rows[0].Env.RowID = "different"
	if SourceParentProjectionStorageEqual(original, &cloned) || SourceParentProjectionStorageEqual(original, nil) {
		t.Fatal("different source owner compared equal")
	}
}
