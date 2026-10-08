package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func siviParentWriteFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, *siviParentProjection, siviParentDirectEdits) {
	t.Helper()
	service, state := reportServiceFixture(t, external)
	if err := service.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
		INSERT INTO Sample_Env(PlotNumber,SV_StandHeight,SV_AhorizonDepth,SV_GleyingMottlingCM,SV_PercentCoarseFrags,
			SV_SoilDepth,SV_FloodPlain,SV_StandAgeEstMeas,SV_StandHeightEstMeas,SV_PolygonNumber,SV_CanopyComposition,
			SnowCoverregime,SV_RootZoneTexture,SV_AhorizonType)
		VALUES('108050',2,2,2,2,2,0,'1','1','original polygon','original canopy','S','original texture','Ah');
		INSERT INTO Sample_Admin(Plot,StrataCoverTotal) VALUES('108050',3);
		INSERT INTO Sample_Env(PlotNumber) VALUES('108051'); INSERT INTO Sample_Admin(Plot) VALUES('108051')`); err != nil {
		t.Fatal(err)
	}
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	return service, state, db, parent, siviParentDirectEdits{scalars, options, siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent)}
}

func siviParentEdit(t *testing.T, parent *siviParentProjection, column string, value ProjectMetadataCell) siviParentScalarEdit {
	t.Helper()
	for _, binding := range parent.Bindings {
		if binding.Binding != column {
			continue
		}
		row := parent.Rows[0].Env
		if binding.Table == parent.AdminTable {
			row = parent.Rows[0].Admin
		}
		return siviParentScalarEdit{parent.ContextID, binding.Table, row.RowID, column, cloneSiteUnitCell(row.Cells[binding.Column]), value}
	}
	t.Fatal("controlled parent target missing", column)
	return siviParentScalarEdit{}
}

func siviParentCell(t *testing.T, parent *siviParentProjection, column string) ProjectMetadataCell {
	t.Helper()
	return siviParentEdit(t, parent, column, ProjectMetadataCell{Storage: "null"}).Expected
}

func TestSIVIParentDirectWriterFourDomainsAndExactAudits(t *testing.T) {
	for domain := 0; domain < 4; domain++ {
		t.Run(strconv.Itoa(domain), func(t *testing.T) {
			service, state, db, parent, all := siviParentWriteFixture(t, domain%2 == 0, 3)
			drafts := siviParentDirectEdits{}
			var edits []siviParentScalarEdit
			switch domain {
			case 0:
				drafts.Scalars, edits = all.Scalars, all.Scalars
			case 1:
				drafts.Options, edits = all.Options, all.Options
			case 2:
				drafts.Text, edits = all.Text, all.Text
			case 3:
				drafts.Categorical, edits = all.Categorical, all.Categorical
			}
			before, err := service.projects.sqlite.beginReadSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			env, err := readSQLiteStorageRows(context.Background(), before, "project", "Sample_Env", "", nil, "")
			if err := errors.Join(err, before.Rollback()); err != nil {
				t.Fatal(err)
			}
			support := databaseBytes(t, service.projects.sqlite.attachments)
			for role, path := range service.projects.sqlite.attachments {
				if path == state.ProjectPath {
					delete(support, role)
				}
			}
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			priorAudits, err := service.ListAuditEntries(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			priorIDs := map[string]bool{}
			for _, record := range priorAudits {
				priorIDs[record.RowID] = true
			}
			result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, drafts)
			if err != nil || result == nil || result.ChangedCells != len(edits) || result.HistoryID == "" {
				t.Fatal("direct domain did not commit its exact assignments", result, err)
			}
			fresh, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
			if err != nil {
				t.Fatal(err)
			}
			for _, edit := range edits {
				if got := siviParentCell(t, fresh, edit.Column); !reflect.DeepEqual(got, edit.Value) {
					t.Fatal("committed storage lost typed/literal value", edit.Column, got, edit.Value)
				}
			}
			audits, err := service.ListAuditEntries(context.Background(), state.ContextID, "108050")
			if err != nil || len(audits) != len(priorAudits)+len(edits) {
				t.Fatal("wrong parent audit count", len(audits), len(priorAudits), err)
			}
			records := []AuditEntry{}
			for _, record := range audits {
				if !priorIDs[record.RowID] {
					records = append(records, record)
				}
			}
			for _, record := range records {
				found := false
				for _, edit := range edits {
					if edit.Column != record.EditField {
						continue
					}
					found = true
					suffix := "_Env"
					if edit.Table == "Sample_Admin" {
						suffix = "_Admin"
					}
					prior, err := metadataCellValue(edit.Expected)
					if err != nil {
						t.Fatal(err)
					}
					value, err := metadataCellValue(edit.Value)
					if err != nil {
						t.Fatal(err)
					}
					if record.Table != suffix || record.ID != nil || record.Project != "Sample" || record.PlotNumber != "108050" ||
						record.User != service.plots.currentUser || record.Restore || record.Flag ||
						!metadataAuditTextEqual(record.BeforeEdit, prior) || !metadataAuditTextEqual(record.AfterEdit, value) {
						t.Fatal("parent audit changed identity/type/value", record, edit)
					}
				}
				if !found {
					t.Fatal("phantom parent audit", record)
				}
			}
			var proposal string
			if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVIParentHistory" WHERE ID=?`, result.HistoryID).Scan(&proposal); err != nil {
				t.Fatal(err)
			}
			var history siviParentHistory
			if err := json.Unmarshal([]byte(proposal), &history); err != nil || !reflect.DeepEqual(history.Original, parent) ||
				!reflect.DeepEqual(history.Committed, fresh) || len(history.Changes) != len(edits) {
				t.Fatal("history lost complete original/committed pair", history, err)
			}
			var sibling int
			if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Env WHERE PlotNumber='108051' AND SV_StandHeight IS NULL`).Scan(&sibling); err != nil || sibling != 1 || len(env.Rows) != 2 {
				t.Fatal("unrelated physical parent was altered", sibling, err)
			}
			assertProfileSUFiles(t, service, support)
			after, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !reflect.DeepEqual(config, after) {
				t.Fatal("parent writing changed YAML", err)
			}
		})
	}
}

func TestSIVIParentDirectWriterRejectsInvalidDomainsAndOwnership(t *testing.T) {
	service, state, _, parent, all := siviParentWriteFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, kind := range []string{"empty", "stale-context", "original-context", "original-row", "expected", "row", "table", "duplicate",
		"implicit", "project-id", "plot-type", "late-domain", "boolean", "single-overflow", "single-type", "option-caption", "option-integer",
		"text-empty", "text-length", "text-unicode", "text-malformed", "category-length", "category-type", "missing-pair"} {
		t.Run(kind, func(t *testing.T) {
			id, plot := state.ContextID, "108050"
			drafts := siviParentDirectEdits{
				append([]siviParentScalarEdit{}, all.Scalars...), append([]siviParentScalarEdit{}, all.Options...),
				append([]siviParentScalarEdit{}, all.Text...), append([]siviParentScalarEdit{}, all.Categorical...),
			}
			original := *parent
			switch kind {
			case "empty":
				drafts = siviParentDirectEdits{}
			case "stale-context":
				id = "stale"
			case "original-context":
				original.ContextID = "stale"
			case "original-row":
				original.Rows = nil
			case "expected":
				drafts.Text[0].Expected = metadataText("stale")
			case "row":
				drafts.Text[0].RowID = "-999"
			case "table":
				drafts.Text[0].Table = "Sample_Admin"
			case "duplicate":
				drafts.Text = append(drafts.Text, drafts.Text[0])
			case "implicit":
				drafts.Scalars = append(drafts.Scalars, siviParentEdit(t, parent, "SpeciesListComplete", metadataInteger("-1")))
			case "project-id":
				drafts.Text = append(drafts.Text, siviParentEdit(t, parent, "ProjectID", metadataText("unreviewed")))
			case "plot-type":
				drafts.Text = append(drafts.Text, siviParentEdit(t, parent, "PlotType", metadataText("Ground")))
			case "late-domain":
				drafts.Categorical = append(drafts.Categorical, all.Options[0])
			case "boolean":
				drafts.Scalars = []siviParentScalarEdit{siviParentEdit(t, parent, "SV_FloodPlain", metadataInteger("1"))}
			case "single-overflow":
				drafts.Scalars = []siviParentScalarEdit{siviParentEdit(t, parent, "SV_StandHeight", siviReal(math.MaxFloat64))}
			case "single-type":
				drafts.Scalars = []siviParentScalarEdit{siviParentEdit(t, parent, "SV_StandHeight", metadataInteger("5"))}
			case "option-caption":
				drafts.Options[0].Value = metadataText("Est.")
			case "option-integer":
				drafts.Options[0].Value = metadataInteger("2")
			case "text-empty":
				drafts.Text[0].Value = metadataText("")
			case "text-length":
				drafts.Text[0].Value = metadataText(strings.Repeat("x", 26))
			case "text-unicode":
				drafts.Text[0].Value = metadataText(strings.Repeat("\U0001f600", 13))
			case "text-malformed":
				drafts.Text[0].Value = metadataText(string([]byte{0xff}))
			case "category-length":
				drafts.Categorical[0].Value = metadataText("AB")
			case "category-type":
				drafts.Categorical[0].Value = metadataInteger("1")
			case "missing-pair":
				plot = "absent"
			}
			got, err := service.writeSIVIParentDirect(context.Background(), id, plot, &original, drafts)
			if err == nil || got != nil {
				t.Fatal("invalid direct batch returned success/partial result", got, err)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
	if result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err != nil || result == nil || result.ChangedCells != 14 {
		t.Fatal("rejections poisoned exact fourteen-field retry", result, err)
	}
}

func TestSIVIParentDirectWriterJoinRechecksAliasesAndContext(t *testing.T) {
	service, state, db, parent, all := siviParentWriteFixture(t, true, 3)
	for _, test := range []struct{ sql, restore string }{
		{`INSERT INTO Sample_Admin(Plot) VALUES('108050 ')`, `DELETE FROM Sample_Admin WHERE Plot='108050 ' COLLATE BINARY`},
		{`INSERT INTO Sample_Admin(Plot) VALUES('é')`, `DELETE FROM Sample_Admin WHERE Plot='é'`},
		{`INSERT INTO Sample_Admin(Plot) VALUES('')`, `DELETE FROM Sample_Admin WHERE Plot=''`},
		{`ALTER TABLE Sample_Admin ADD COLUMN ExtraGenerated TEXT GENERATED ALWAYS AS ('x') VIRTUAL`,
			`ALTER TABLE Sample_Admin DROP COLUMN ExtraGenerated`},
	} {
		if _, err := db.Exec(test.sql); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all)
		if err == nil || got != nil {
			t.Fatal("unreviewed alias/schema/domain authorized writing", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.restore); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES('a1'); INSERT INTO Sample_Admin(Plot) VALUES('A1')`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err != nil || got == nil {
		t.Fatal("unrelated supported join identities blocked writing", got, err)
	}
}

func TestSIVIParentDirectWriterStaleWholeRowTriggersAndProvenanceRollback(t *testing.T) {
	service, state, db, parent, all := siviParentWriteFixture(t, false, 3)
	for _, test := range []struct{ sql, restore string }{
		{`UPDATE Sample_Env SET FieldNumber='changed sibling field' WHERE PlotNumber='108050'`,
			`UPDATE Sample_Env SET FieldNumber=NULL WHERE PlotNumber='108050'`},
		{`CREATE TRIGGER ParentUnexpected AFTER UPDATE OF SV_StandHeight ON Sample_Env
			BEGIN UPDATE Sample_Admin SET PlotType='BAD' WHERE Plot='108051'; END`, `DROP TRIGGER ParentUnexpected`},
		{`CREATE TRIGGER ParentAuditUnexpected AFTER INSERT ON Sample_Audit
			BEGIN UPDATE Sample_Audit SET Flag=-1 WHERE rowid=NEW.rowid; END`, `DROP TRIGGER ParentAuditUnexpected`},
		{`CREATE TABLE "__VPRO_SIVIParentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT);
			CREATE TRIGGER ParentHistoryUnexpected AFTER INSERT ON "__VPRO_SIVIParentHistory"
			BEGIN UPDATE Sample_Env SET FieldNumber='BAD' WHERE PlotNumber='108051'; END`,
			`DROP TRIGGER ParentHistoryUnexpected; DROP TABLE "__VPRO_SIVIParentHistory"`},
		{`CREATE TABLE "__VPRO_SIVIParentHistory"(ID INTEGER PRIMARY KEY,Created TEXT,Proposal TEXT,Restored TEXT)`,
			`DROP TABLE "__VPRO_SIVIParentHistory"`},
	} {
		if _, err := db.Exec(test.sql); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err == nil || got != nil {
			t.Fatal("stale/trigger/provenance drift committed", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.restore); err != nil {
			t.Fatal(err)
		}
	}
	got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all)
	if err != nil || got == nil || got.ChangedCells != 14 {
		t.Fatal("rollback poisoned complete retry", got, err)
	}
}

func TestSIVIParentDirectWriterCancellationCollisionAndRetry(t *testing.T) {
	service, state, db, parent, all := siviParentWriteFixture(t, false, 3)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.writeSIVIParentDirect(cancelled, state.ContextID, "108050", parent, all); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("pre-cancelled parent write succeeded", got, err)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	got, writeErr := service.writeSIVIParentDirect(ctx, state.ContextID, "108050", parent, all)
	stop()
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(writeErr, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked write released success/partial result", got, writeErr)
	}
	assertProfileSUFiles(t, service, before)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "parents changed") {
			t.Fatal("collision returned unexpected failure", err)
		}
	}
	if success != 1 {
		t.Fatal("same original elected more than one writer", success)
	}
}

func TestSIVIParentDirectWriterHistoricalNoopAndBLOBReplacement(t *testing.T) {
	service, state, db, _, _ := siviParentWriteFixture(t, false, 3)
	if _, err := db.Exec(`UPDATE Sample_Env SET SV_PolygonNumber='',SV_CanopyComposition=?,SV_RootZoneTexture=X'00FF',
		SV_FloodPlain=1,SV_StandAgeEstMeas='bad' WHERE PlotNumber='108050'`, strings.Repeat("x", 51)); err != nil {
		t.Fatal(err)
	}
	parent, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	all := siviParentDirectEdits{
		Scalars: []siviParentScalarEdit{siviParentEdit(t, parent, "SV_FloodPlain", siviParentCell(t, parent, "SV_FloodPlain"))},
		Options: []siviParentScalarEdit{siviParentEdit(t, parent, "SV_StandAgeEstMeas", metadataText("bad"))},
		Text: []siviParentScalarEdit{
			siviParentEdit(t, parent, "SV_PolygonNumber", metadataText("")),
			siviParentEdit(t, parent, "SV_CanopyComposition", metadataText(strings.Repeat("x", 51))),
		},
		Categorical: []siviParentScalarEdit{siviParentEdit(t, parent, "SV_RootZoneTexture", siviParentCell(t, parent, "SV_RootZoneTexture"))},
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all)
	if err != nil || result == nil || result.ChangedCells != 0 || result.HistoryID != "" {
		t.Fatal("unchanged historical values created assignments/history", result, err)
	}
	assertProfileSUFiles(t, service, before)
	all.Categorical[0].Value = metadataText("correction")
	if result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err == nil || result != nil {
		t.Fatal("BLOB correction fabricated a source audit representation", result, err)
	}
	assertProfileSUFiles(t, service, before)
	all.Categorical = nil
	all.Scalars[0].Value = metadataInteger("-1")
	all.Options[0].Value = metadataText("2")
	result, err = service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all)
	if err != nil || result == nil || result.ChangedCells != 2 {
		t.Fatal("valid correction failed to omit unrelated invalid history", result, err)
	}
	if restored, err := service.restoreSIVIParentDirect(context.Background(), state.ContextID, "108050", result.HistoryID, AuditRestoreRetain); err != nil || restored == nil || restored.RestoredRows != 2 {
		t.Fatal("typed invalid BOOLEAN/option originals could not be restored", restored, err)
	}
	fresh, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(fresh, parent) {
		t.Fatal("restoration normalized unchanged/invalid historical originals", fresh, err)
	}
}

func TestSIVIParentDirectWriterFileReplacementAndSUDrift(t *testing.T) {
	service, state, _, parent, all := siviParentWriteFixture(t, true, 3)
	path := service.projects.sqlite.attachments["su"]
	su, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := su.Exec(`DELETE FROM Report_SU WHERE PlotNumber='108050'`); err != nil {
		t.Fatal(err)
	}
	if err := su.Close(); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err == nil || result != nil {
		t.Fatal("parent removed from external selected SU was written", result, err)
	}
	assertProfileSUFiles(t, service, before)
	info := service.projects.sqlite.attachmentInfo["project"]
	other := filepath.Join(t.TempDir(), "other.db")
	if err := os.WriteFile(other, []byte("different physical file"), 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Stat(other)
	if err != nil {
		t.Fatal(err)
	}
	service.projects.sqlite.attachmentInfo["project"] = replacement
	if result, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err == nil || result != nil {
		t.Fatal("replaced project file identity authorized writing", result, err)
	}
	service.projects.sqlite.attachmentInfo["project"] = info
	assertProfileSUFiles(t, service, before)
}

func TestSIVIParentDirectWriterPhysicalSUAndAuditSchema(t *testing.T) {
	service, state, db, parent, all := siviParentWriteFixture(t, false, 3)
	for _, test := range []struct{ sql, restore string }{
		{`ALTER TABLE Report_SU RENAME TO Report_SU_Saved; CREATE VIEW Report_SU AS SELECT * FROM Report_SU_Saved`,
			`DROP VIEW Report_SU; ALTER TABLE Report_SU_Saved RENAME TO Report_SU`},
		{`ALTER TABLE Report_SU ADD COLUMN ExtraGenerated TEXT GENERATED ALWAYS AS ('x') VIRTUAL`,
			`ALTER TABLE Report_SU DROP COLUMN ExtraGenerated`},
		{`ALTER TABLE Sample_Audit ADD COLUMN ExtraGenerated TEXT GENERATED ALWAYS AS ('x') VIRTUAL`,
			`ALTER TABLE Sample_Audit DROP COLUMN ExtraGenerated`},
		{`ALTER TABLE Sample_Env ADD COLUMN "_rowid_" TEXT`,
			`ALTER TABLE Sample_Env DROP COLUMN "_rowid_"`},
		{`DROP INDEX uidx_Sample_Admin_PlotNumber; INSERT INTO Sample_Admin SELECT * FROM Sample_Admin WHERE Plot='108050'`,
			`DELETE FROM Sample_Admin WHERE rowid=(SELECT MAX(rowid) FROM Sample_Admin WHERE Plot='108050');
			CREATE UNIQUE INDEX uidx_Sample_Admin_PlotNumber ON Sample_Admin(Plot)`},
	} {
		if _, err := db.Exec(test.sql); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err == nil || got != nil {
			t.Fatal("nonphysical/shadowed/duplicate membership or audit schema authorized writing", got, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(test.restore); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err != nil || got == nil || got.ChangedCells != 14 {
		t.Fatal("schema/duplicate rejection poisoned retry", got, err)
	}
}

func TestSIVIParentDirectWriterExternalProjectAndUnfilteredContext(t *testing.T) {
	service, state, _, _, _ := siviParentWriteFixture(t, true, 3)
	originalFiles := databaseBytes(t, service.projects.sqlite.attachments)
	data, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "owned external # parent.db")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(state)
	selection.ProjectPath, selection.SU, selection.SUPath = path, "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := service.readSIVIParent(context.Background(), next.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	scalars, options := siviProposalEdits(parent)
	drafts := siviParentDirectEdits{scalars, options, siviStorageTextEdits(parent), siviCategoricalProposalEdits(parent)}
	written, err := service.writeSIVIParentDirect(context.Background(), next.ContextID, "108050", parent, drafts)
	if err != nil || written == nil || written.ChangedCells != 14 {
		t.Fatal("owned external project/unfiltered context writer failed", written, err)
	}
	if restored, err := service.restoreSIVIParentDirect(context.Background(), next.ContextID, "108050", written.HistoryID, AuditRestoreRetain); err != nil || restored == nil || restored.RestoredRows != 14 {
		t.Fatal("owned external project restoration failed", restored, err)
	}
	for role, oldPath := range map[string]string{"project": state.ProjectPath, "su": state.SUPath} {
		got, err := os.ReadFile(oldPath)
		if err != nil || !reflect.DeepEqual(got, originalFiles[role]) {
			t.Fatal("external writer changed previous project or SU", role, err)
		}
	}
}

func TestSIVIParentDirectTransactionLateCancellationOwnershipRollbackAndRetry(t *testing.T) {
	service, state, _, parent, all := siviParentWriteFixture(t, true, 3)
	for _, kind := range []string{"cancel", "ownership", "operation"} {
		t.Run(kind, func(t *testing.T) {
			before := databaseBytes(t, service.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			originalInfo := service.projects.sqlite.attachmentInfo["project"]
			other := filepath.Join(t.TempDir(), "different-file")
			if err := os.WriteFile(other, []byte("other"), 0600); err != nil {
				t.Fatal(err)
			}
			replacement, err := os.Stat(other)
			if err != nil {
				t.Fatal(err)
			}
			sentinel := errors.New("injected rejection after parent and audit staging")
			_, writeErr := withContextPlotRequest(ctx, service, state.ContextID, func(plots *PlotService) (struct{}, error) {
				owner := plots.projects.sqlite
				err := withSIVIParentWriteTransaction(ctx, owner, "parent edit", func(tx *sql.Tx, suAlias string) error {
					observed, err := readSIVIParentForWrite(ctx, tx, owner, suAlias, state.ContextID, "108050")
					if err != nil || !reflect.DeepEqual(observed, parent) {
						return errors.Join(err, errors.New("late boundary original changed"))
					}
					assignments, err := planSIVIParentDirectEdits(ctx, observed, all)
					if err != nil {
						return err
					}
					tables, err := environmentSiteUnitTables(ctx, tx)
					if err != nil {
						return err
					}
					_, changes, _, err := applySIVIParentAssignments(ctx, tx, observed, tables, assignments, plots.currentUser, plots.auditStrength)
					if err != nil || len(changes) != 14 {
						return errors.Join(err, errors.New("late boundary did not stage all fourteen parent/audit changes"))
					}
					switch kind {
					case "cancel":
						cancel()
					case "ownership":
						owner.attachmentInfo["project"] = replacement
					case "operation":
						return sentinel
					}
					return nil
				})
				return struct{}{}, err
			})
			service.projects.sqlite.attachmentInfo["project"] = originalInfo
			if writeErr == nil || (kind == "cancel" && !errors.Is(writeErr, context.Canceled)) ||
				(kind == "operation" && !errors.Is(writeErr, sentinel)) {
				t.Fatal("late transaction rejection returned success/wrong failure", kind, writeErr)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
	if got, err := service.writeSIVIParentDirect(context.Background(), state.ContextID, "108050", parent, all); err != nil || got == nil || got.ChangedCells != 14 {
		t.Fatal("late rolled-back transaction poisoned owned writer retry", got, err)
	}
}
