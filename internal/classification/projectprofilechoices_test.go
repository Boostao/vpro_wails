package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	c := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, c.Attachments)
	choices, err := ListProjectPlotProfileChoices(service, context.Background(), state.ContextID)
	if err != nil || len(choices.EnvFields) == 0 || len(choices.Species) == 0 {
		t.Fatal("source profile suggestions unavailable", choices, err)
	}
	columns, err := vprocore.ReadSQLiteStorageColumns(context.Background(), c.Conn, "project", c.Selection.Project+"_Env")
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
	if err := c.Conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM (SELECT Code FROM VLists.USysAllSpecs GROUP BY Code)`).Scan(&count); err != nil || count != len(choices.Species) {
		t.Fatal("source grouped species codes were filtered by lifeform or merged with personal definitions", count, err)
	}
	for role, prior := range before {
		now, err := os.ReadFile(c.Attachments[role])
		if err != nil || !bytes.Equal(prior, now) {
			t.Fatal("suggestion read changed stored data", role, err)
		}
	}
	if _, err := ListProjectPlotProfileChoices(service, context.Background(), "stale"); err == nil {
		t.Fatal("stale suggestion ownership accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ListProjectPlotProfileChoices(service, ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled suggestion read accepted", err)
	}
}
