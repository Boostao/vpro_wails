package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestProfileChoicesSelectedProjectSourceSpeciesOwnershipAndReadOnly(t *testing.T) {
	service, state, _ := profileRunFixture(t)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	choices, err := service.ListProjectPlotProfileChoices(context.Background(), state.ContextID)
	if err != nil || len(choices.EnvFields) == 0 || len(choices.Species) == 0 {
		t.Fatal("source profile suggestions unavailable", choices, err)
	}
	columns, err := readSQLiteStorageColumns(context.Background(), c.conn, "project", c.selection.Project+"_Env")
	if err != nil {
		t.Fatal(err)
	}
	fields := []string{}
	for _, column := range columns {
		fields = append(fields, column.Name)
	}
	if !reflect.DeepEqual(fields, choices.EnvFields) {
		t.Fatal("selected project fields replaced by hardcoded Sample_Env", fields, choices.EnvFields)
	}
	var count int
	if err := c.conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM (SELECT Code FROM VLists.USysAllSpecs GROUP BY Code)`).Scan(&count); err != nil || count != len(choices.Species) {
		t.Fatal("source grouped species codes were filtered by lifeform or merged with personal definitions", count, err)
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("suggestion read changed stored data", role, err)
		}
	}
	if _, err := service.ListProjectPlotProfileChoices(context.Background(), "stale"); err == nil {
		t.Fatal("stale suggestion ownership accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListProjectPlotProfileChoices(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled suggestion read accepted", err)
	}
}
