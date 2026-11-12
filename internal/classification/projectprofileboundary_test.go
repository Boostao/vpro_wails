package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileRuleNoopDoesNotCreateTechnicalHistory(t *testing.T) {
	service, _, request := profileRunFixture(t)
	c := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, c.Attachments)
	if err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, nil, "Noop test"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := c.Conn.QueryRowContext(context.Background(),
		`SELECT count(*) FROM project.sqlite_master WHERE name='__VPRO_ProfileHistory'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("no-op created technical history", count, err)
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("no-op changed database bytes", role, err)
		}
	}
}

func TestProfileRuleRejectsIncompatibleHistoryAndReplacedOrAliasedSupportOwnership(t *testing.T) {
	for _, mode := range []string{"history", "replacement", "alias"} {
		t.Run(mode, func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.Projects.Sqlite
			value := "literal edit"
			drafts := []profileRuleDraft{{RowID: "3", Changes: []vprocore.ProjectMetadataChange{
				{Column: "Criteria", Value: vprocore.ProjectMetadataCell{Storage: "text", Text: &value}},
			}}}
			originalReference, originalInfo := c.Attachments["VLists"], c.AttachmentInfo["VLists"]
			switch mode {
			case "history":
				if err := c.WithMetadataWriter(context.Background(), func(conn *sql.Conn) error {
					_, err := conn.ExecContext(context.Background(), `CREATE TABLE __VPRO_ProfileHistory(ID TEXT,AfterRules TEXT)`)
					return err
				}); err != nil {
					t.Fatal(err)
				}
			case "replacement":
				data, err := os.ReadFile(originalReference)
				if err != nil {
					t.Fatal(err)
				}
				replacement := filepath.Join(t.TempDir(), "replacement.db")
				if err := os.WriteFile(replacement, data, 0600); err != nil {
					t.Fatal(err)
				}
				c.Attachments["VLists"] = replacement
			case "alias":
				c.Attachments["VLists"], c.AttachmentInfo["VLists"] = c.Attachments["project"], c.AttachmentInfo["project"]
			}
			before := vprotest.DatabaseBytes(t, c.Attachments)
			err := SaveProfileRuleDrafts(c, context.Background(), request.OriginalRules, drafts, "Ownership test")
			if err == nil || mode == "history" && !strings.Contains(err.Error(), "history schema") {
				t.Fatal("invalid history/replaced file/shared support writer accepted", mode, err)
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.Attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("rejected edit changed data/support bytes", mode, role, err)
				}
			}
			c.Attachments["VLists"], c.AttachmentInfo["VLists"] = originalReference, originalInfo
		})
	}
}
