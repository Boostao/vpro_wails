package classification

import (
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
	"testing"
)

func environmentSiteUnitFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, EnvironmentSiteUnitTransfer) {
	t.Helper()
	service, state := vprotest.ContextServiceFixture(t)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM Sample_SU;
		INSERT INTO Sample_SU(PlotNumber,SiteUnit) VALUES('8229723','Original'),('8428935',''),('Foreign','Untouched');
		UPDATE Sample_Admin SET UserSiteUnit='  Literal '' unit  ' WHERE Plot='8229723';
		UPDATE Sample_Admin SET UserSiteUnit=NULL WHERE Plot='8428935'`); err != nil {
		t.Fatal(err)
	}
	state, err = service.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project: state.ActiveProject, ProjectPath: state.ProjectPath, SU: "Sample", SUPath: state.ProjectPath,
		Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReviewEnvironmentSiteUnits(service, t.Context(), state.ContextID)
	if err != nil || len(review.Changes) != 2 {
		t.Fatal("explicit current SU review unavailable", review.Changes, err)
	}
	return service, state, EnvironmentSiteUnitTransfer{review, true}
}

func TestEnvironmentSiteUnitTransferReviewCommitHistoryAndReplay(t *testing.T) {
	service, state, request := environmentSiteUnitFixture(t)
	c := service.Projects.Sqlite
	files := vprotest.DatabaseBytes(t, c.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReviewEnvironmentSiteUnits(service, t.Context(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review, request.Review) {
		t.Fatal("independent review changed plan", err)
	}
	vprotest.AssertProfileSUFiles(t, service, files)
	before := map[string]vprocore.ProjectMetadataTable{}
	for _, name := range []string{"Sample_Env", "Sample_Admin", "Sample_Audit", "_table_metadata"} {
		before[name], err = vprocore.ReadSQLiteStorageRows(t.Context(), c.Conn, "project", name, "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request)
	if err != nil || result == nil || result.ChangedRows != 2 || result.HistoryID != "1" {
		t.Fatal("reviewed transfer not committed", result, err)
	}
	for name, original := range before {
		actual, err := vprocore.ReadSQLiteStorageRows(t.Context(), c.Conn, "project", name, "", nil, "")
		if err != nil || !reflect.DeepEqual(actual, original) {
			t.Fatal("original source/audits/descriptions changed", name, err)
		}
	}
	next, err := ReviewEnvironmentSiteUnits(service, t.Context(), state.ContextID)
	if err != nil || len(next.Changes) != 0 {
		t.Fatal("committed values were not exact typed assignments", err)
	}
	var raw string
	if err := c.Conn.QueryRowContext(t.Context(), `SELECT Proposal FROM project.__VPRO_EnvironmentSUHistory WHERE ID=1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Request EnvironmentSiteUnitTransfer `json:"request"`
		User    string                      `json:"user"`
	}
	if err := json.Unmarshal([]byte(raw), &provenance); err != nil || !reflect.DeepEqual(provenance.Request, request) {
		t.Fatal("transactional provenance lost complete originals", err)
	}
	after := vprotest.DatabaseBytes(t, c.Attachments)
	if _, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed transfer replay accepted")
	}
	vprotest.AssertProfileSUFiles(t, service, after)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_SU SET SiteUnit='Original' WHERE PlotNumber='8229723';
		UPDATE Sample_SU SET SiteUnit='' WHERE PlotNumber='8428935'`); err != nil {
		t.Fatal(err)
	}
	after = vprotest.DatabaseBytes(t, c.Attachments)
	if _, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed review replayed after independent target reset")
	}
	vprotest.AssertProfileSUFiles(t, service, after)
	newReview, err := ReviewEnvironmentSiteUnits(service, t.Context(), state.ContextID)
	if err != nil || newReview.HistoryHash == "" {
		t.Fatal("independent new review lacks the technical history revision", err)
	}
	result, err = TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, EnvironmentSiteUnitTransfer{newReview, true})
	if err != nil || result.HistoryID != "2" {
		t.Fatal("separate new review did not preserve and append original history", result, err)
	}
	afterConfig, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || string(afterConfig) != string(config) {
		t.Fatal("transfer changed YAML", err)
	}
	for role, original := range files {
		if os.SameFile(c.AttachmentInfo[role], c.AttachmentInfo["project"]) {
			continue
		}
		actual, err := os.ReadFile(c.Attachments[role])
		if err != nil || !reflect.DeepEqual(actual, original) {
			t.Fatal("support or independent database changed", role, err)
		}
	}
}

func TestEnvironmentSiteUnitTransferRollbackRetainedRetryAndDrift(t *testing.T) {
	for name, sqlText := range map[string]string{
		"source": `UPDATE Sample_Admin SET UserSiteUnit='Independent' WHERE Plot='8229723'`,
		"target": `UPDATE Sample_SU SET SiteUnit='Independent' WHERE PlotNumber='8229723'`,
		"duplicate": `ALTER TABLE Sample_SU RENAME TO OriginalSU; CREATE TABLE Sample_SU AS SELECT * FROM OriginalSU;
			INSERT INTO Sample_SU SELECT * FROM OriginalSU WHERE PlotNumber='8229723'; DROP TABLE OriginalSU`,
		"provenance": `CREATE TABLE __VPRO_EnvironmentSUHistory(Protected TEXT)`,
		"second-abort": `CREATE TRIGGER su_transfer_guard BEFORE UPDATE ON Sample_SU
			WHEN OLD.PlotNumber='8428935' BEGIN SELECT RAISE(ABORT,'Independent rejection'); END`,
		"late-other-table": `CREATE TRIGGER su_transfer_guard AFTER UPDATE ON Sample_SU
			BEGIN UPDATE Sample_Admin SET EnteredBy='Unreviewed trigger' WHERE Plot='8229723'; END`,
		"late-other-row": `CREATE TRIGGER su_transfer_guard AFTER UPDATE ON Sample_SU
			WHEN OLD.PlotNumber<>'Foreign' BEGIN UPDATE Sample_SU SET SiteUnit='Unreviewed trigger' WHERE PlotNumber='Foreign'; END`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, request := environmentSiteUnitFixture(t)
			db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(sqlText); err != nil {
				t.Fatal(err)
			}
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			if _, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request); err == nil {
				t.Fatal("stale/trigger/provenance mutation accepted")
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			if name == "second-abort" || name == "late-other-table" || name == "late-other-row" {
				if _, err := db.Exec(`DROP TRIGGER su_transfer_guard`); err != nil {
					t.Fatal(err)
				}
				if result, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request); err != nil || result.ChangedRows != 2 {
					t.Fatal("retained proposal retry failed", result, err)
				}
			}
		})
	}
}

func TestEnvironmentSiteUnitTransferStrictTransportCancellationAndOwnership(t *testing.T) {
	service, state, request := environmentSiteUnitFixture(t)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded EnvironmentSiteUnitTransfer
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("complete transport changed", err)
	}
	for _, raw := range []string{`{}`, `{"review":null,"confirmed":true}`,
		`{"review":{},"confirmed":true,"overwrite":true}`,
		`{"review":{"path":"\ud800"},"confirmed":true}`} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("partial/unknown/repaired transport accepted", raw)
		}
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if _, err := TransferEnvironmentSiteUnits(service, t.Context(), "stale", request); err == nil {
		t.Fatal("stale context accepted")
	}
	request.Confirmed = false
	if _, err := TransferEnvironmentSiteUnits(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("unconfirmed request accepted")
	}
	request.Confirmed = true
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := TransferEnvironmentSiteUnits(service, ctx, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled transfer did not reject", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	next, err := service.SwitchContext(state.ContextID, vprocore.ContextSelection{
		Project: state.ActiveProject, ProjectPath: state.ProjectPath, SU: "None",
		Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReviewEnvironmentSiteUnits(service, t.Context(), next.ContextID); err == nil {
		t.Fatal("implicit/no SU selection accepted")
	}
}
