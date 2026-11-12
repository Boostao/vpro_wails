package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestProfileLifecycleScopedAPIsRejectSnapshotAndSupportDrift(t *testing.T) {
	for _, action := range []string{"create", "delete"} {
		for _, mode := range []string{"stale snapshot", "support replacement", "support alias"} {
			t.Run(action+"/"+mode, func(t *testing.T) {
				service, state, request := profileRunFixture(t)
				c := service.Projects.Sqlite
				path, info := c.Attachments["VLists"], c.AttachmentInfo["VLists"]
				defer func() { c.Attachments["VLists"], c.AttachmentInfo["VLists"] = path, info }()
				switch mode {
				case "stale snapshot":
					if err := c.WithMetadataWriter(context.Background(), func(conn *sql.Conn) error {
						_, err := conn.ExecContext(context.Background(), `UPDATE Sample_Profile SET Criteria='independent edit' WHERE rowid=8`)
						return err
					}); err != nil {
						t.Fatal(err)
					}
				case "support replacement":
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					replacement := filepath.Join(t.TempDir(), "replacement.db")
					if err := os.WriteFile(replacement, data, 0600); err != nil {
						t.Fatal(err)
					}
					c.Attachments["VLists"] = replacement
				case "support alias":
					c.Attachments["VLists"], c.AttachmentInfo["VLists"] = c.Attachments["project"], c.AttachmentInfo["project"]
				}
				before := vprotest.DatabaseBytes(t, c.Attachments)
				var err error
				if action == "create" {
					_, err = CreateProjectPlotProfileRule(service, context.Background(), state.ContextID, blankProfileCreation(request.OriginalRules))
				} else {
					err = DeleteProjectPlotProfileRule(service, context.Background(), state.ContextID,
						ProjectPlotProfileDeletion{OriginalRules: request.OriginalRules, RowID: "3", Confirmed: true})
				}
				if err == nil {
					t.Fatal("scoped lifecycle API accepted stale snapshot/support ownership", action, mode)
				}
				for role, prior := range before {
					now, err := os.ReadFile(c.Attachments[role])
					if err != nil || !bytes.Equal(prior, now) {
						t.Fatal("rejected lifecycle API changed file bytes", action, mode, role, err)
					}
				}
			})
		}
	}
}

func TestProfileLifecycleDeletionHistoryFailureRetainsOriginalForRetry(t *testing.T) {
	service, state, request := profileRunFixture(t)
	c := service.Projects.Sqlite
	row, err := CreateProjectPlotProfileRule(service, context.Background(), state.ContextID, blankProfileCreation(request.OriginalRules))
	if err != nil {
		t.Fatal(err)
	}
	deletion := ProjectPlotProfileDeletion{OriginalRules: profileLifecycleRules(t, c), RowID: row.RowID, Confirmed: true}
	if err := c.WithMetadataWriter(context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `CREATE TRIGGER FailProfileDeletion BEFORE INSERT ON __VPRO_ProfileHistory
			BEGIN SELECT RAISE(ABORT,'deletion history rollback'); END`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, c.Attachments)
	if err := DeleteProjectPlotProfileRule(service, context.Background(), state.ContextID, deletion); err == nil {
		t.Fatal("failed deletion history committed rule/reservation changes")
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("failed deletion changed file bytes", role, err)
		}
	}
	if err := c.WithMetadataWriter(context.Background(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(context.Background(), `DROP TRIGGER FailProfileDeletion`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteProjectPlotProfileRule(service, context.Background(), state.ContextID, deletion); err != nil {
		t.Fatal("exact retained deletion could not retry", err)
	}
	if !sameProfileStorageRows(request.OriginalRules, profileLifecycleRules(t, c)) {
		t.Fatal("deletion retry changed surviving historical rules/counts")
	}
}
