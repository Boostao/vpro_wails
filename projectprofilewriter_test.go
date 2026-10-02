package main

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

func independentProfileFixture(t *testing.T, otherTable bool) (*ContextService, ProjectState, string) {
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
		db, _, release, err := service.plots.getActiveDB()
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
	next, err := service.SelectPlotProfile(state.ContextID, PlotProfileSource{name, path})
	if err != nil {
		t.Fatal(err)
	}
	return service, next, path
}

func authorizeProfileFixture(t *testing.T, service *ContextService, state ProjectState, enabled bool) ProjectState {
	t.Helper()
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := service.SetPlotProfileEditing(context.Background(), state.ContextID,
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
			c := service.projects.sqlite
			files := databaseBytes(t, c.attachments)
			config, err := os.ReadFile(service.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			projectRules, err := readSQLiteStorageRows(context.Background(), c.conn, "project", "Sample_Profile", "", nil, "Order")
			if err != nil {
				t.Fatal(err)
			}
			next := authorizeProfileFixture(t, service, state, true)
			if _, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID); err == nil {
				t.Fatal("old identity retained authorization")
			}
			for role, original := range files {
				current, err := os.ReadFile(service.projects.sqlite.attachments[role])
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("authorization wrote database bytes", role, err)
				}
			}
			review, err := service.ReviewProjectPlotProfile(context.Background(), next.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			literal := " Independent profile criteria "
			drafts := []profileRuleDraft{{RowID: "3", Changes: []ProjectMetadataChange{
				{Column: "Criteria", Value: ProjectMetadataCell{Storage: "text", Text: &literal}},
			}}}
			c = service.projects.sqlite
			if err := c.saveProfileRuleDrafts(context.Background(), review.Rules, drafts, "Ownership test"); err != nil {
				t.Fatal(err)
			}
			role, table, err := c.profileLocation()
			if err != nil {
				t.Fatal(err)
			}
			var actualTable, actualUser string
			if err := c.conn.QueryRowContext(context.Background(), `SELECT ProfileTable,User FROM `+
				quoteHeaderIdentifier(role)+`.__VPRO_ProfileHistory`).Scan(&actualTable, &actualUser); err != nil ||
				actualTable != table || actualUser != "Ownership test" {
				t.Fatal("history did not follow selected file/table", actualTable, actualUser, err)
			}
			unchanged, err := readSQLiteStorageRows(context.Background(), c.conn, "project", "Sample_Profile", "", nil, "Order")
			if err != nil || !sameProfileStorageRows(projectRules, unchanged) {
				t.Fatal("independent writer changed the parent project's profile", err)
			}
			for owner, original := range files {
				if os.SameFile(c.attachmentInfo[owner], c.attachmentInfo[role]) {
					continue
				}
				current, err := os.ReadFile(c.attachments[owner])
				if err != nil || !bytes.Equal(original, current) {
					t.Fatal("profile save changed independent file", owner, err)
				}
			}
			currentConfig, err := os.ReadFile(service.projects.preferences.path)
			if err != nil || !bytes.Equal(config, currentConfig) {
				t.Fatal("session authorization changed persistent YAML", err)
			}
			reopened, err := newSQLiteProjectService(service.projects.root, service.projects.config, service.projects.preferences)
			if err != nil {
				t.Fatal(err)
			}
			info, err := reopened.sqlite.profileInfo(context.Background())
			if closeErr := reopened.closeSQLiteContext(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || info.Writable {
				t.Fatal("restart inferred write authority from a persisted path", info, err)
			}
			revoked := authorizeProfileFixture(t, service, next, false)
			review, err = service.ReviewProjectPlotProfile(context.Background(), revoked.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.projects.sqlite.saveProfileRuleDrafts(context.Background(), review.Rules, drafts, "Ownership test"); err == nil {
				t.Fatal("revoked context retained writer authority")
			}
			granted := authorizeProfileFixture(t, service, revoked, true)
			changed, err := service.SelectPlotProfile(granted.ContextID, PlotProfileSource{state.PlotProfile.Source.Name, path})
			if err != nil || changed.PlotProfile.Writable {
				t.Fatal("selection change retained the session grant", changed, err)
			}
		})
	}
}

func TestProfileWriteOwnershipRejectsStaleUnconfirmedCancelledAndSupportAliases(t *testing.T) {
	service, state, _ := independentProfileFixture(t, false)
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	owner := service.projects.sqlite
	files := databaseBytes(t, owner.attachments)
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
				originalInfo, originalPath := owner.attachmentInfo["VUser"], owner.attachments["VUser"]
				owner.attachmentInfo["VUser"], owner.attachments["VUser"] = owner.attachmentInfo["profile"], owner.attachments["profile"]
				defer func() {
					owner.attachmentInfo["VUser"], owner.attachments["VUser"] = originalInfo, originalPath
				}()
			}
			if _, err := service.SetPlotProfileEditing(ctx, id, proposal); err == nil {
				t.Fatal("invalid write authorization published", kind)
			}
			if service.projects.sqlite != owner || service.projects.contextID != state.ContextID || owner.profileWrite {
				t.Fatal("failed authorization replaced the original context")
			}
		})
	}
	for role, original := range files {
		current, err := os.ReadFile(owner.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("rejected authorization wrote a database", role, err)
		}
	}
	authorizeProfileFixture(t, service, state, true)
}

func TestProfileWriteOwnershipExternalRollbackCancellationRetryAndReservedIDs(t *testing.T) {
	service, state, _ := independentProfileFixture(t, false)
	next := authorizeProfileFixture(t, service, state, true)
	c := service.projects.sqlite
	review, err := service.ReviewProjectPlotProfile(context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.withProfileWriter(context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `CREATE TABLE __VPRO_ProfileHistory(
			ID INTEGER PRIMARY KEY AUTOINCREMENT,ProfileTable TEXT NOT NULL,User TEXT NOT NULL,
			EditWhen TEXT NOT NULL,BeforeRules TEXT NOT NULL,AfterRules TEXT NOT NULL);
			CREATE TRIGGER FailHistory BEFORE INSERT ON __VPRO_ProfileHistory
			BEGIN SELECT RAISE(ABORT,'external history failure'); END`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, c.attachments)
	if _, err := c.createProfileRule(context.Background(), blankProfileCreation(review.Rules), "External lifecycle"); err == nil {
		t.Fatal("external history failure committed rule/reservation")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.createProfileRule(cancelled, blankProfileCreation(review.Rules), "External lifecycle"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled external mutation not rejected", err)
	}
	for role, original := range before {
		current, err := os.ReadFile(c.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("failed mutation changed rules/history/reservations", role, err)
		}
	}
	if err := c.withProfileWriter(context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `DROP TRIGGER FailHistory`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	first, err := c.createProfileRule(context.Background(), blankProfileCreation(review.Rules), "External lifecycle")
	if err != nil || first.RowID != "9" {
		t.Fatal("retained external creation did not retry", first, err)
	}
	review, err = service.ReviewProjectPlotProfile(context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.deleteProfileRule(context.Background(), ProjectPlotProfileDeletion{OriginalRules: review.Rules, RowID: first.RowID, Confirmed: true}, "External lifecycle"); err != nil {
		t.Fatal(err)
	}
	review, err = service.ReviewProjectPlotProfile(context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.createProfileRule(context.Background(), blankProfileCreation(review.Rules), "External lifecycle")
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
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
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
	project, err := service.SelectPlotProfile(state.ContextID, PlotProfileSource{"Sample", path})
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfile(ctx, project.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	c := service.projects.sqlite
	first, err := c.createProfileRule(ctx, blankProfileCreation(review.Rules), "Project reservation")
	if err != nil || first.RowID != "9" {
		t.Fatal(first, err)
	}
	review, err = service.ReviewProjectPlotProfile(ctx, project.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.deleteProfileRule(ctx, ProjectPlotProfileDeletion{OriginalRules: review.Rules, RowID: first.RowID, Confirmed: true}, "Project reservation"); err != nil {
		t.Fatal(err)
	}
	other, err := service.SelectPlotProfile(project.ContextID, PlotProfileSource{"Other", path})
	if err != nil {
		t.Fatal(err)
	}
	other = authorizeProfileFixture(t, service, other, true)
	review, err = service.ReviewProjectPlotProfile(ctx, other.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	c = service.projects.sqlite
	second, err := c.createProfileRule(ctx, blankProfileCreation(review.Rules), "Other reservation")
	if err != nil || second.RowID != "9" {
		t.Fatal("another table inherited deleted project rule identities", second, err)
	}
	var tables int
	if err := c.conn.QueryRowContext(ctx, `SELECT COUNT(DISTINCT ProfileTable) FROM project.__VPRO_ProfileIdentity WHERE RuleRowID=9`).Scan(&tables); err != nil || tables != 2 {
		t.Fatal("reservation qualification was lost", tables, err)
	}
}

func TestProfileWriteOwnershipReplacementBeforeCommitRollsBack(t *testing.T) {
	service, state, path := independentProfileFixture(t, false)
	state = authorizeProfileFixture(t, service, state, true)
	c := service.projects.sqlite
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
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
	err = c.mutateProfileRules(context.Background(), review.Rules, "Late replacement", func(tx *sql.Tx, table string, fresh ProjectMetadataTable) (*ProjectMetadataTable, error) {
		if _, err := tx.ExecContext(context.Background(), `UPDATE `+quoteHeaderIdentifier(table)+` SET Criteria='late mutation' WHERE rowid=3`); err != nil {
			return nil, err
		}
		observed, err := readSQLiteStorageRows(context.Background(), tx, "main", table, "", nil, "Order")
		return &observed, err
	}, func(*sql.Tx) error {
		c.attachments["profile"] = replacement
		return nil
	})
	c.attachments["profile"] = path
	if err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("late writer identity change was not rejected", err)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(original, after) {
		t.Fatal("late ownership failure committed mutation/history", readErr)
	}
	review, err = service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal("restored original writer could not retry", err)
	}
	if _, err := c.createProfileRule(context.Background(), blankProfileCreation(review.Rules), "Retained retry"); err != nil {
		t.Fatal(err)
	}
}
