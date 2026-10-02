package main

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
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	if err := c.saveProfileRuleDrafts(context.Background(), request.OriginalRules, nil, "Noop test"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := c.conn.QueryRowContext(context.Background(),
		`SELECT count(*) FROM project.sqlite_master WHERE name='__VPRO_ProfileHistory'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("no-op created technical history", count, err)
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("no-op changed database bytes", role, err)
		}
	}
}

func TestProfileRuleRejectsIncompatibleHistoryAndReplacedOrAliasedSupportOwnership(t *testing.T) {
	for _, mode := range []string{"history", "replacement", "alias"} {
		t.Run(mode, func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.projects.sqlite
			value := "literal edit"
			drafts := []profileRuleDraft{{RowID: "3", Changes: []ProjectMetadataChange{
				{Column: "Criteria", Value: ProjectMetadataCell{Storage: "text", Text: &value}},
			}}}
			originalReference, originalInfo := c.attachments["VLists"], c.attachmentInfo["VLists"]
			switch mode {
			case "history":
				if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
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
				c.attachments["VLists"] = replacement
			case "alias":
				c.attachments["VLists"], c.attachmentInfo["VLists"] = c.attachments["project"], c.attachmentInfo["project"]
			}
			before := databaseBytes(t, c.attachments)
			err := c.saveProfileRuleDrafts(context.Background(), request.OriginalRules, drafts, "Ownership test")
			if err == nil || mode == "history" && !strings.Contains(err.Error(), "history schema") {
				t.Fatal("invalid history/replaced file/shared support writer accepted", mode, err)
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("rejected edit changed data/support bytes", mode, role, err)
				}
			}
			c.attachments["VLists"], c.attachmentInfo["VLists"] = originalReference, originalInfo
		})
	}
}
