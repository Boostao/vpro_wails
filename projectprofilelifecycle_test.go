package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func blankProfileCreation(rules ProjectMetadataTable) ProjectPlotProfileCreation {
	values := []ProjectMetadataChange{}
	for _, name := range profileRuleColumns[:8] {
		values = append(values, ProjectMetadataChange{Column: name, Value: ProjectMetadataCell{Storage: "null"}})
	}
	return ProjectPlotProfileCreation{OriginalRules: rules, Values: values}
}

func profileLifecycleRules(t *testing.T, c *sqliteContext) ProjectMetadataTable {
	t.Helper()
	result, err := readSQLiteStorageRows(context.Background(), c.conn, "project", c.selection.Project+"_Profile", "", nil, "Order")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProfileRuleLifecycleBlankCreationDeletionHistoryAndDeletedIDsReserved(t *testing.T) {
	service, _, request := profileRunFixture(t)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	created, err := c.createProfileRule(context.Background(), blankProfileCreation(request.OriginalRules), "Lifecycle test")
	if err != nil || created.RowID != "9" || len(created.Cells) != 9 {
		t.Fatal("explicit blank creation did not allocate one new physical identity", created, err)
	}
	for _, cell := range created.Cells {
		if !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "null"}) {
			t.Fatal("blank creation invented an Order, input or historical count", cell)
		}
	}
	rules := profileLifecycleRules(t, c)
	if len(rules.Rows) != 9 {
		t.Fatal("creation did not produce exactly one new rule")
	}
	deletion := ProjectPlotProfileDeletion{OriginalRules: rules, RowID: created.RowID, Confirmed: true}
	if err := c.deleteProfileRule(context.Background(), deletion, "Lifecycle test"); err != nil {
		t.Fatal(err)
	}
	rules = profileLifecycleRules(t, c)
	if !sameProfileStorageRows(request.OriginalRules, rules) {
		t.Fatal("deletion changed any surviving historical rule/count")
	}
	next, err := c.createProfileRule(context.Background(), blankProfileCreation(rules), "Lifecycle test")
	if err != nil || next.RowID != "10" {
		t.Fatal("deleted physical identity was reused", next, err)
	}
	var count, reserved int
	if err := c.conn.QueryRowContext(context.Background(), `SELECT count(*) FROM project.__VPRO_ProfileHistory`).Scan(&count); err != nil || count != 3 {
		t.Fatal("creation/deletion/recreation did not each commit one history event", count, err)
	}
	if err := c.conn.QueryRowContext(context.Background(), `SELECT count(*) FROM project.__VPRO_ProfileIdentity WHERE RuleRowID=9`).Scan(&reserved); err != nil || reserved != 1 {
		t.Fatal("deleted identity was not durably reserved", reserved, err)
	}
	rows, err := c.conn.QueryContext(context.Background(), `SELECT BeforeRules,AfterRules FROM project.__VPRO_ProfileHistory ORDER BY ID`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var before, after string
		if err := rows.Scan(&before, &after); err != nil {
			t.Fatal(err)
		}
		var old, current ProjectMetadataTable
		if err := json.Unmarshal([]byte(before), &old); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(after), &current); err != nil {
			t.Fatal(err)
		}
		wantBefore, wantAfter := 8, 9
		if index == 1 {
			wantBefore, wantAfter = 9, 8
		}
		if len(old.Rows) != wantBefore || len(current.Rows) != wantAfter {
			t.Fatal("lifecycle history omitted typed created/deleted rows", index, old, current)
		}
		index++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	now := databaseBytes(t, c.attachments)
	for role, prior := range before {
		if !os.SameFile(c.attachmentInfo[role], c.attachmentInfo["project"]) && !bytes.Equal(prior, now[role]) {
			t.Fatal("rule lifecycle changed support files", role)
		}
	}
}

func TestProfileRuleLifecycleRejectedRequestsAndCancellationPreserveBytes(t *testing.T) {
	service, _, request := profileRunFixture(t)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	invalid := []ProjectPlotProfileCreation{blankProfileCreation(request.OriginalRules)}
	invalid[0].Values = invalid[0].Values[:7]
	duplicate := blankProfileCreation(request.OriginalRules)
	duplicate.Values[7] = duplicate.Values[0]
	invalid = append(invalid, duplicate)
	collision := blankProfileCreation(request.OriginalRules)
	number := "1"
	collision.Values[0].Value = ProjectMetadataCell{Storage: "integer", Integer: &number}
	invalid = append(invalid, collision)
	for _, creation := range invalid {
		if _, err := c.createProfileRule(context.Background(), creation, "Rejected test"); err == nil {
			t.Fatal("incomplete/repeated/colliding creation accepted")
		}
	}
	for _, deletion := range []ProjectPlotProfileDeletion{
		{OriginalRules: request.OriginalRules, RowID: "3"},
		{OriginalRules: request.OriginalRules, RowID: "99999", Confirmed: true},
		{OriginalRules: request.OriginalRules, RowID: "03", Confirmed: true},
	} {
		if err := c.deleteProfileRule(context.Background(), deletion, "Rejected test"); err == nil {
			t.Fatal("unconfirmed/foreign/aliased deletion accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.createProfileRule(ctx, blankProfileCreation(request.OriginalRules), "Cancelled test"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled creation accepted", err)
	}
	if err := c.deleteProfileRule(ctx, ProjectPlotProfileDeletion{OriginalRules: request.OriginalRules, RowID: "3", Confirmed: true}, "Cancelled test"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled deletion accepted", err)
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("rejection/cancellation allocated identity, history or stored changes", role, err)
		}
	}
}

func TestProfileRuleLifecycleHistoryFailureReservationDriftAndRetry(t *testing.T) {
	for _, effect := range []string{
		`SELECT RAISE(ABORT,'lifecycle history rollback');`,
		`DELETE FROM __VPRO_ProfileIdentity WHERE ProfileTable='Sample_Profile' AND RuleRowID=9;`,
	} {
		t.Run(effect, func(t *testing.T) {
			service, _, request := profileRunFixture(t)
			c := service.projects.sqlite
			seed := "technical history seed"
			if err := c.saveProfileRuleDrafts(context.Background(), request.OriginalRules, []profileRuleDraft{{
				RowID: "3", Changes: []ProjectMetadataChange{{Column: "Criteria", Value: ProjectMetadataCell{Storage: "text", Text: &seed}}},
			}}, "Seed test"); err != nil {
				t.Fatal(err)
			}
			fresh := profileLifecycleRules(t, c)
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				_, err := conn.ExecContext(context.Background(), `CREATE TRIGGER FailProfileLifecycle AFTER INSERT ON __VPRO_ProfileHistory BEGIN `+effect+` END`)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, c.attachments)
			if _, err := c.createProfileRule(context.Background(), blankProfileCreation(fresh), "Rollback test"); err == nil {
				t.Fatal("failed history/reservation drift committed creation")
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("lifecycle failure did not roll back history/reservations/rules", role, err)
				}
			}
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				_, err := conn.ExecContext(context.Background(), `DROP TRIGGER FailProfileLifecycle`)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			row, err := c.createProfileRule(context.Background(), blankProfileCreation(fresh), "Retry test")
			if err != nil || row.RowID != "9" {
				t.Fatal("failed creation consumed identity or could not retry", row, err)
			}
		})
	}
}

func TestProfileRuleLifecycleRejectsExhaustionAndUnmappedCreationSchema(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Profile SET rowid=` + strconv.FormatInt(math.MaxInt64, 10) + ` WHERE rowid=8`,
		`ALTER TABLE Sample_Profile ADD COLUMN HistoricalExtra TEXT DEFAULT 'do not inherit'`,
		`ALTER TABLE Sample_Profile ADD COLUMN GeneratedExtra TEXT GENERATED ALWAYS AS (Criteria) VIRTUAL`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, _, _ := profileRunFixture(t)
			c := service.projects.sqlite
			if err := c.withMetadataWriter(context.Background(), func(conn *sql.Conn) error {
				_, err := conn.ExecContext(context.Background(), mutation)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			rules := profileLifecycleRules(t, c)
			before := databaseBytes(t, c.attachments)
			if _, err := c.createProfileRule(context.Background(), blankProfileCreation(rules), "Unavailable test"); err == nil {
				t.Fatal("exhausted identity/unmapped defaults/generated fields accepted")
			}
			for role, prior := range before {
				now, err := os.ReadFile(c.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("unavailable creation changed bytes", role, err)
				}
			}
		})
	}
}
