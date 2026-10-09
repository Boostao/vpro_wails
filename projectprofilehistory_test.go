package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestProfileRuleHistoryIgnoredRewrittenOrLateRuleDriftRollsBack(t *testing.T) {
	for index, effect := range []string{
		`SELECT RAISE(IGNORE);`,
		`UPDATE __VPRO_ProfileHistory SET AfterRules='lost' WHERE ID=NEW.ID;`,
		`UPDATE Sample_Profile SET PlotCount=999 WHERE rowid=3;`,
	} {
		t.Run(effect, func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.projects.sqlite
			value := "changed once"
			drafts := []profileRuleDraft{{RowID: "3", Changes: []ProjectMetadataChange{
				{Column: "Criteria", Value: ProjectMetadataCell{Storage: "text", Text: &value}},
			}}}
			timing := "AFTER"
			if index == 0 {
				timing = "BEFORE"
			}
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				_, err := conn.ExecContext(context.Background(), `CREATE TABLE __VPRO_ProfileHistory(
					ID INTEGER PRIMARY KEY AUTOINCREMENT,ProfileTable TEXT NOT NULL,User TEXT NOT NULL,
					EditWhen TEXT NOT NULL,BeforeRules TEXT NOT NULL,AfterRules TEXT NOT NULL);
					CREATE TRIGGER AlterProfileHistory `+timing+` INSERT ON __VPRO_ProfileHistory BEGIN `+effect+` END;`)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, c.attachments)
			if err := c.saveProfileRuleDrafts(context.Background(), request.OriginalRules, drafts, "History test"); err == nil {
				t.Fatal("missing, rewritten or drifting history committed rule changes")
			}
			for role, prior := range before {
				current, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, current) {
					t.Fatal("failed history observation changed stored bytes", role, err)
				}
			}
		})
	}
}
