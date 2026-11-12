package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestProfileRuleEditsAtomicSnapshotHistoryNoopAndStaleRetry(t *testing.T) {
	service, state, request := profileRunFixture(t)
	c := service.Projects.Sqlite
	files := vprotest.DatabaseBytes(t, c.Attachments)
	value := "  >500 literal draft  "
	drafts := []profileRuleDraft{{RowID: "3", Changes: []vprocore.ProjectMetadataChange{
		{Column: "Criteria", Value: vprocore.ProjectMetadataCell{Storage: "text", Text: &value}},
	}}}
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Profile test"); err != nil {
		t.Fatal(err)
	}
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	planned, _, err := prepareProfileRuleDrafts(request.OriginalRules, drafts)
	if err != nil || !sameProfileStorageRows(planned, review.Rules) {
		t.Fatal("profile literal/NULL/counts/other rows changed outside the plan", review, err)
	}
	var before, after string
	if err := c.Conn.QueryRowContext(context.Background(),
		`SELECT BeforeRules,AfterRules FROM project.__VPRO_ProfileHistory WHERE ProfileTable='Sample_Profile'`).Scan(&before, &after); err != nil {
		t.Fatal(err)
	}
	var old, current vprocore.ProjectMetadataTable
	if err := json.Unmarshal([]byte(before), &old); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(after), &current); err != nil {
		t.Fatal(err)
	}
	if !sameProfileStorageRows(old, request.OriginalRules) || !sameProfileStorageRows(current, planned) {
		t.Fatal("transactional history lost typed original/new storage")
	}
	now := vprotest.DatabaseBytes(t, c.Attachments)
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Profile test"); err == nil {
		t.Fatal("stale original rule snapshot accepted")
	}
	if err := SaveProfileRuleDrafts(c, context.Background(), review.Rules, drafts, "Profile test"); err != nil {
		t.Fatal("unchanged assignment did not omit a no-op", err)
	}
	for role, prior := range now {
		latest, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(prior, latest) {
			t.Fatal("stale rejection/no-op committed history or data", role, err)
		}
	}
	for role, prior := range files {
		if !os.SameFile(c.AttachmentInfo[role], c.AttachmentInfo["project"]) && !bytes.Equal(prior, now[role]) {
			t.Fatal("profile editing changed attached support data", role)
		}
	}
}

func TestProfileRuleEditsHistoryFailureFinalDriftCancellationAndOrderCollisionRollback(t *testing.T) {
	service, state, request := profileRunFixture(t)
	c := service.Projects.Sqlite
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`CREATE TABLE __VPRO_ProfileHistory(
		ID INTEGER PRIMARY KEY AUTOINCREMENT,ProfileTable TEXT NOT NULL,User TEXT NOT NULL,
		EditWhen TEXT NOT NULL,BeforeRules TEXT NOT NULL,AfterRules TEXT NOT NULL);
		CREATE TRIGGER FailProfileHistory BEFORE INSERT ON __VPRO_ProfileHistory
		BEGIN SELECT RAISE(ABORT,'profile history failure'); END`); err != nil {
		t.Fatal(err)
	}
	value := "changed"
	drafts := []profileRuleDraft{{RowID: "3", Changes: []vprocore.ProjectMetadataChange{
		{Column: "Criteria", Value: vprocore.ProjectMetadataCell{Storage: "text", Text: &value}},
	}}}
	before := vprotest.DatabaseBytes(t, c.Attachments)
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Profile test"); err == nil {
		t.Fatal("history failure committed profile changes")
	}
	for role, prior := range before {
		latest, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(prior, latest) {
			t.Fatal("failed history committed any data", role, err)
		}
	}

	if _, err := db.Exec(`DROP TRIGGER FailProfileHistory;
		CREATE TRIGGER DriftProfile AFTER UPDATE OF Criteria ON Sample_Profile
		BEGIN UPDATE Sample_Profile SET PlotCount=123 WHERE rowid=NEW.rowid; END`); err != nil {
		t.Fatal(err)
	}
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Profile test"); err == nil {
		t.Fatal("unplanned final count drift committed")
	}
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review.Rules, request.OriginalRules) {
		t.Fatal("profile mutation/count/history failed to roll back", review, err)
	}
	if _, err := db.Exec(`DROP TRIGGER DriftProfile`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SaveProfileRuleDrafts(c, ctx, request.OriginalRules, drafts, "Profile test"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled profile mutation accepted", err)
	}
	order := "3"
	collision := []profileRuleDraft{{RowID: "3", Changes: []vprocore.ProjectMetadataChange{
		{Column: "Order", Value: vprocore.ProjectMetadataCell{Storage: "integer", Integer: &order}},
	}}}
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, collision, "Profile test"); err == nil {
		t.Fatal("new duplicate source Order accepted")
	}
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Profile test"); err != nil {
		t.Fatal("retained snapshot could not retry after history/observation/cancellation rollback", err)
	}
}
