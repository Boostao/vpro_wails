package main

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestProfileRuleLifecycleDetectsReservationDriftFromRuleMutationTriggers(t *testing.T) {
	for _, action := range []string{"INSERT", "DELETE"} {
		t.Run(action, func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.projects.sqlite
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				_, err := conn.ExecContext(context.Background(), `CREATE TRIGGER AlterRuleReservations AFTER `+action+
					` ON Sample_Profile BEGIN DELETE FROM __VPRO_ProfileIdentity WHERE RuleRowID=8; END`)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, c.attachments)
			var err error
			if action == "INSERT" {
				_, err = c.createProfileRule(context.Background(), blankProfileCreation(request.OriginalRules), "Trigger test")
			} else {
				err = c.deleteProfileRule(context.Background(), ProjectPlotProfileDeletion{
					OriginalRules: request.OriginalRules, RowID: "3", Confirmed: true}, "Trigger test")
			}
			if err == nil {
				t.Fatal("rule mutation silently removed historical identity reservation", action)
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("rule/reservation/history drift committed", role, err)
				}
			}
		})
	}
}

func TestProfileRuleLifecycleSeedsHistoricalIDsAndRejectsMalformedRestorationAliases(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(map[bool]string{false: "historical identity", true: "malformed alias"}[corrupt], func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.projects.sqlite
			if _, err := c.createProfileRule(context.Background(), blankProfileCreation(request.OriginalRules), "Initial test"); err != nil {
				t.Fatal(err)
			}
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				if _, err := conn.ExecContext(context.Background(),
					`DELETE FROM Sample_Profile WHERE rowid=9; DROP TABLE __VPRO_ProfileIdentity;`); err != nil {
					return err
				}
				if corrupt {
					_, err := conn.ExecContext(context.Background(), `UPDATE __VPRO_ProfileHistory
						SET AfterRules=replace(AfterRules,'"rowId":"9"','"rowId":"09"')`)
					return err
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			rules := profileLifecycleRules(t, c)
			before := databaseBytes(t, c.attachments)
			created, err := c.createProfileRule(context.Background(), blankProfileCreation(rules), "History test")
			if !corrupt {
				if err != nil || created.RowID != "10" {
					t.Fatal("historical deleted identity was reused when rebuilding reservations", created, err)
				}
				return
			}
			if err == nil {
				t.Fatal("nonliteral historical alias was silently repaired/ignored")
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("malformed historical identity created reservations or rules", role, err)
				}
			}
		})
	}
}
