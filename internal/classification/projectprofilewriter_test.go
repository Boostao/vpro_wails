package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func independentProfileFixture(t *testing.T, otherTable bool) (*vprocore.ContextService, vprocore.ProjectState, string) {
	t.Helper()
	service, state, _ := profileRunFixture(t)
	path, name := state.ProjectPath, "Other"
	if !otherTable {
		path, name = filepath.Join(t.TempDir(), "external profile O'Brien #.db"), state.ActiveProject
		data, err := os.ReadFile(state.ProjectPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		db, _, release, err := service.Plots.GetActiveDB()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE Other_Profile AS SELECT * FROM Sample_Profile;
			INSERT INTO _table_metadata(table_name,description) VALUES('Other_Profile',NULL)`); err != nil {
			release()
			t.Fatal(err)
		}
		release()
	}
	next, err := service.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: name, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	return service, next, path
}

func authorizeProfileFixture(t *testing.T, service *vprocore.ContextService, state vprocore.ProjectState, enabled bool) vprocore.ProjectState {
	t.Helper()
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := SetPlotProfileEditing(service, context.Background(), state.ContextID,
		PlotProfileWriteRequest{Review: review, Enabled: enabled, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if next.ContextID == state.ContextID || next.ProjectPath != state.ProjectPath ||
		next.ActiveSU != state.ActiveSU || next.ActiveHierarchy != state.ActiveHierarchy ||
		next.PlotProfile.Source != state.PlotProfile.Source || next.PlotProfile.Writable != enabled {
		t.Fatal("authorization changed independent selection or failed to rotate editor identity", next)
	}
	return next
}

func TestProfileWriteOwnershipSelectedFileTableHistoryAndSessionLifetime(t *testing.T) {
	for _, otherTable := range []bool{false, true} {
		t.Run(map[bool]string{false: "external-file", true: "other-project-table"}[otherTable], func(t *testing.T) {
			service, state, path := independentProfileFixture(t, otherTable)
			c := service.Projects.Sqlite
			files := vprotest.DatabaseBytes(t, c.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			projectRules, err := vprocore.ReadSQLiteStorageRows(context.Background(), c.Conn, "project", "Sample_Profile", "", nil, "Order")
			if err != nil {
				t.Fatal(err)
			}
			next := authorizeProfileFixture(t, service, state, true)
			if _, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID); err == nil {
				t.Fatal("old identity retained authorization")
			}
			for role, original := range files {
				current, err := os.ReadFile(service.Projects.Sqlite.Attachments[role])
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("authorization wrote database bytes", role, err)
				}
			}
			review, err := ReviewProjectPlotProfile(service, context.Background(), next.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			literal := " Independent profile criteria "
			drafts := []profileRuleDraft{{RowID: "3", Changes: []vprocore.ProjectMetadataChange{
				{Column: "Criteria", Value: vprocore.ProjectMetadataCell{Storage: "text", Text: &literal}},
			}}}
			c = service.Projects.Sqlite
			if err := SaveProfileRuleDrafts(c, context.Background(), review.Rules, drafts, "Ownership test"); err != nil {
				t.Fatal(err)
			}
			role, table, err := c.ProfileLocation()
			if err != nil {
				t.Fatal(err)
			}
			var actualTable, actualUser string
			if err := c.Conn.QueryRowContext(context.Background(), `SELECT ProfileTable,User FROM `+
				vprocore.QuoteHeaderIdentifier(role)+`.__VPRO_ProfileHistory`).Scan(&actualTable, &actualUser); err != nil ||
				actualTable != table || actualUser != "Ownership test" {
				t.Fatal("history did not follow selected file/table", actualTable, actualUser, err)
			}
			unchanged, err := vprocore.ReadSQLiteStorageRows(context.Background(), c.Conn, "project", "Sample_Profile", "", nil, "Order")
			if err != nil || !sameProfileStorageRows(projectRules, unchanged) {
				t.Fatal("independent writer changed the parent project's profile", err)
			}
			for owner, original := range files {
				if os.SameFile(c.AttachmentInfo[owner], c.AttachmentInfo[role]) {
					continue
				}
				current, err := os.ReadFile(c.Attachments[owner])
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("profile save changed independent file", owner, err)
				}
			}
			currentConfig, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, currentConfig) {
				t.Fatal("session authorization changed persistent YAML", err)
			}
			reopened, err := vprocore.NewSQLiteProjectService(service.Projects.Root, service.Projects.Config, service.Projects.Preferences)
			if err != nil {
				t.Fatal(err)
			}
			info, err := reopened.Sqlite.ProfileInfo(context.Background())
			if closeErr := reopened.CloseSQLiteContext(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || info.Writable {
				t.Fatal("restart inferred write authority from a persisted path", info, err)
			}
			revoked := authorizeProfileFixture(t, service, next, false)
			review, err = ReviewProjectPlotProfile(service, context.Background(), revoked.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			if err := SaveProfileRuleDrafts(service.Projects.Sqlite, context.Background(), review.Rules, drafts, "Ownership test"); err == nil {
				t.Fatal("revoked context retained writer authority")
			}
			granted := authorizeProfileFixture(t, service, revoked, true)
			changed, err := service.SelectPlotProfile(granted.ContextID, vprocore.PlotProfileSource{Name: state.PlotProfile.Source.Name, Path: path})
			if err != nil || changed.PlotProfile.Writable {
				t.Fatal("selection change retained the session grant", changed, err)
			}
		})
	}
}

func TestProfileWriteOwnershipRejectsStaleUnconfirmedCancelledAndSupportAliases(t *testing.T) {
	service, state, _ := independentProfileFixture(t, false)
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	owner := service.Projects.Sqlite
	files := vprotest.DatabaseBytes(t, owner.Attachments)
	request := PlotProfileWriteRequest{Review: review, Enabled: true, Confirmed: true}
	for _, kind := range []string{"stale-context", "unconfirmed", "cancelled", "stale-description", "support-alias"} {
		t.Run(kind, func(t *testing.T) {
			ctx, id, proposal := context.Background(), state.ContextID, request
			switch kind {
			case "stale-context":
				id = "old"
			case "unconfirmed":
				proposal.Confirmed = false
			case "cancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "stale-description":
				proposal.Review.Table = "Other_Profile"
			case "support-alias":
				originalInfo, originalPath := owner.AttachmentInfo["VUser"], owner.Attachments["VUser"]
				owner.AttachmentInfo["VUser"], owner.Attachments["VUser"] = owner.AttachmentInfo["profile"], owner.Attachments["profile"]
				defer func() {
					owner.AttachmentInfo["VUser"], owner.Attachments["VUser"] = originalInfo, originalPath
				}()
			}
			if _, err := SetPlotProfileEditing(service, ctx, id, proposal); err == nil {
				t.Fatal("invalid write authorization published", kind)
			}
			if service.Projects.Sqlite != owner || service.Projects.ContextID != state.ContextID || owner.ProfileWrite {
				t.Fatal("failed authorization replaced the original context")
			}
		})
	}
	for role, original := range files {
		current, err := os.ReadFile(owner.Attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("rejected authorization wrote a database", role, err)
		}
	}
	authorizeProfileFixture(t, service, state, true)
}

func TestProfileWriteOwnershipExternalRollbackCancellationRetryAndReservedIDs(t *testing.T) {
	service, state, _ := independentProfileFixture(t, false)
	next := authorizeProfileFixture(t, service, state, true)
	c := service.Projects.Sqlite
	review, err := ReviewProjectPlotProfile(service, context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := WithProfileWriter(c, context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `CREATE TABLE __VPRO_ProfileHistory(
			ID INTEGER PRIMARY KEY AUTOINCREMENT,ProfileTable TEXT NOT NULL,User TEXT NOT NULL,
			EditWhen TEXT NOT NULL,BeforeRules TEXT NOT NULL,AfterRules TEXT NOT NULL);
			CREATE TRIGGER FailHistory BEFORE INSERT ON __VPRO_ProfileHistory
			BEGIN SELECT RAISE(ABORT,'external history failure'); END`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, c.Attachments)
	if _, err := CreateProfileRule(c, context.Background(), blankProfileCreation(review.Rules), "External lifecycle"); err == nil {
		t.Fatal("external history failure committed rule/reservation")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CreateProfileRule(c, cancelled, blankProfileCreation(review.Rules), "External lifecycle"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled external mutation not rejected", err)
	}
	for role, original := range before {
		current, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("failed mutation changed rules/history/reservations", role, err)
		}
	}
	if err := WithProfileWriter(c, context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `DROP TRIGGER FailHistory`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	first, err := CreateProfileRule(c, context.Background(), blankProfileCreation(review.Rules), "External lifecycle")
	if err != nil || first.RowID != "9" {
		t.Fatal("retained external creation did not retry", first, err)
	}
	review, err = ReviewProjectPlotProfile(service, context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteProfileRule(c, context.Background(), ProjectPlotProfileDeletion{OriginalRules: review.Rules, RowID: first.RowID, Confirmed: true}, "External lifecycle"); err != nil {
		t.Fatal(err)
	}
	review, err = ReviewProjectPlotProfile(service, context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateProfileRule(c, context.Background(), blankProfileCreation(review.Rules), "External lifecycle")
	if err != nil || second.RowID != "10" {
		t.Fatal("deleted external rule identity was reused", second, err)
	}
}

func TestProfileWriteOwnershipStrictTransport(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"review":null,"enabled":true,"confirmed":true}`,
		`{"review":{},"enabled":null,"confirmed":true}`,
		`{"review":{},"enabled":true}`,
		`{"review":{},"enabled":true,"confirmed":true,"repair":true}`,
		`{"review":{"project":"\ud800"},"enabled":true,"confirmed":true}`,
	} {
		var request PlotProfileWriteRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed/incomplete ownership request accepted", raw)
		}
	}
	service, state, _ := independentProfileFixture(t, false)
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(PlotProfileWriteRequest{Review: review, Enabled: true, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	var request PlotProfileWriteRequest
	if err := json.Unmarshal(raw, &request); err != nil || !strings.Contains(string(raw), `"confirmed":true`) {
		t.Fatal("complete ownership request did not roundtrip", err)
	}
}

func TestProfileWriteOwnershipReservationsAreTableQualified(t *testing.T) {
	service, state, path := independentProfileFixture(t, true)
	ctx := context.Background()
	project, err := service.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: "Sample", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	review, err := ReviewProjectPlotProfile(service, ctx, project.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	c := service.Projects.Sqlite
	first, err := CreateProfileRule(c, ctx, blankProfileCreation(review.Rules), "Project reservation")
	if err != nil || first.RowID != "9" {
		t.Fatal(first, err)
	}
	review, err = ReviewProjectPlotProfile(service, ctx, project.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteProfileRule(c, ctx, ProjectPlotProfileDeletion{OriginalRules: review.Rules, RowID: first.RowID, Confirmed: true}, "Project reservation"); err != nil {
		t.Fatal(err)
	}
	other, err := service.SelectPlotProfile(project.ContextID, vprocore.PlotProfileSource{Name: "Other", Path: path})
	if err != nil {
		t.Fatal(err)
	}
	other = authorizeProfileFixture(t, service, other, true)
	review, err = ReviewProjectPlotProfile(service, ctx, other.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	c = service.Projects.Sqlite
	second, err := CreateProfileRule(c, ctx, blankProfileCreation(review.Rules), "Other reservation")
	if err != nil || second.RowID != "9" {
		t.Fatal("another table inherited deleted project rule identities", second, err)
	}
	var tables int
	if err := c.Conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT ProfileTable) FROM project.__VPRO_ProfileIdentity WHERE RuleRowID=9`).Scan(&tables); err != nil || tables != 2 {
		t.Fatal("reservation qualification was lost", tables, err)
	}
}

func TestProfileWriteOwnershipReplacementBeforeCommitRollsBack(t *testing.T) {
	service, state, path := independentProfileFixture(t, false)
	state = authorizeProfileFixture(t, service, state, true)
	c := service.Projects.Sqlite
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(t.TempDir(), "replacement.db")
	if err := os.WriteFile(replacement, original, 0600); err != nil {
		t.Fatal(err)
	}
	err = MutateProfileRules(c, context.Background(), review.Rules, "Late replacement", func(tx *sql.Tx, table string, fresh vprocore.ProjectMetadataTable) (*vprocore.ProjectMetadataTable, error) {
		if _, err := tx.ExecContext(context.Background(), `UPDATE `+vprocore.QuoteHeaderIdentifier(table)+` SET Criteria='late mutation' WHERE rowid=3`); err != nil {
			return nil, err
		}
		observed, err := vprocore.ReadSQLiteStorageRows(context.Background(), tx, "main", table, "", nil, "Order")
		return &observed, err
	}, func(*sql.Tx) error {
		c.Attachments["profile"] = replacement
		return nil
	})
	c.Attachments["profile"] = path
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("late writer identity change was not rejected", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(original, after) {
		t.Fatal("late ownership failure committed mutation/history", readErr)
	}
	review, err = ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal("restored original writer could not retry", err)
	}
	if _, err := CreateProfileRule(c, context.Background(), blankProfileCreation(review.Rules), "Retained retry"); err != nil {
		t.Fatal(err)
	}
}
