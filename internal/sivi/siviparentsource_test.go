package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestSIVIASCIIJoinOracleDomain(t *testing.T) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, left := range alphabet {
		for _, right := range alphabet {
			a, validA := sourceform.SiviASCIIPlotKey(string(left))
			b, validB := sourceform.SiviASCIIPlotKey(strings.ToLower(string(right)))
			if !validA || !validB || (a == b) != (left == right) {
				t.Fatal("ASCII oracle comparison changed", left, right)
			}
		}
	}
	for _, invalid := range []string{"", " A", "A B", "A-B", "é", "e\u0301", "ABCDEFGH", "\x00", "   "} {
		if _, valid := sourceform.SiviASCIIPlotKey(invalid); valid {
			t.Fatal("unprobed identity was certified", invalid)
		}
	}
	a, valid := sourceform.SiviASCIIPlotKey("Ab12  ")
	if !valid || a != "AB12" {
		t.Fatal("source case/trailing U+0020 contract changed")
	}
}

func TestSIVIParentJoinDetectsPhysicalAliasesAndUnknowns(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	service.SiviParentReviewEnabled = true
	vprotest.MutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
		INSERT INTO Sample_Env(PlotNumber) VALUES('ABC1'); INSERT INTO Sample_Admin(Plot) VALUES('ABC1')`)
	for _, test := range []struct{ sql, restore, reason string }{
		{`INSERT INTO Sample_Admin(Plot) VALUES('abc1 ')`, `DELETE FROM Sample_Admin WHERE Plot='abc1 ' COLLATE BINARY`, "2 Admin"},
		{`INSERT INTO Sample_Env(PlotNumber) VALUES('abc1')`, `DELETE FROM Sample_Env WHERE PlotNumber='abc1' COLLATE BINARY`, "2 Env"},
		{`INSERT INTO Sample_Admin(Plot) VALUES('é')`, `DELETE FROM Sample_Admin WHERE Plot='é' COLLATE BINARY`, "unsupported"},
		{`INSERT INTO Sample_Admin(Plot) VALUES('')`, `DELETE FROM Sample_Admin WHERE Plot='' COLLATE BINARY`, "unsupported"},
	} {
		original, err := GetSIVIParentJoinReview(service, context.Background(), state.ContextID, "ABC1")
		if err != nil || !original.Verified || len(original.EnvRowIDs) != 1 || len(original.AdminRowIDs) != 1 {
			t.Fatal("single exact supported pair not certified", original, err)
		}
		vprotest.MutateContextFixture(t, state.ProjectPath, test.sql)
		before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
		got, err := GetSIVIParentJoinReview(service, context.Background(), state.ContextID, "ABC1")
		if err != nil || got.Verified || !strings.Contains(got.Diagnostic, test.reason) {
			t.Fatal("source alias/unknown was hidden by literal filtering", got, err)
		}
		vprotest.AssertProfileSUFiles(t, service, before)
		vprotest.MutateContextFixture(t, state.ProjectPath, test.restore)
	}
	if got, err := GetSIVIParentJoinReview(service, context.Background(), "stale", "ABC1"); got != nil || err == nil {
		t.Fatal("foreign context certified")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := GetSIVIParentJoinReview(service, ctx, state.ContextID, "ABC1"); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled review succeeded", err)
	}
}

func TestSIVIProjectIDSourceOwnedCASRollbackAndNoPlotWrites(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 2, 1); got != nil || err == nil {
		t.Fatal("default gate enabled preference writer")
	}
	service.SiviParentReviewEnabled = true
	for _, requested := range []int{0, 3} {
		if got, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 2, requested); got != nil || err == nil {
			t.Fatal("invalid source accepted")
		}
	}
	if got, err := SetSIVIProjectIDSource(service, ctx, "stale", 2, 1); got != nil || err == nil {
		t.Fatal("stale owner changed preference")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := SetSIVIProjectIDSource(service, cancelled, state.ContextID, 2, 1); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled source change committed", err)
	}
	replace := service.Projects.Preferences.Replace
	service.Projects.Preferences.Replace = func(string, string) error { return errors.New("injected source commit failure") }
	if got, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 2, 1); got != nil || err == nil {
		t.Fatal("failed config commit returned successful source")
	}
	service.Projects.Preferences.Replace = replace
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !reflect.DeepEqual(config, after) {
		t.Fatal("rejected preference changed YAML", err)
	}
	initial, err := service.Projects.Preferences.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	env, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 2, 1)
	if err != nil || env.Source != "Env" || env.SourceOption != 1 || env.Table != "Sample_Metadata" {
		t.Fatal("explicit Env source did not return physical choices", env, err)
	}
	if got, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 2, 2); got != nil || err == nil {
		t.Fatal("stale preference expectation overwrote current source")
	}
	master, err := SetSIVIProjectIDSource(service, ctx, state.ContextID, 1, 2)
	if err != nil || master.Source != "Master" || master.SourceOption != 2 || master.Alias != "VMetaData" {
		t.Fatal("Master restoration failed", master, err)
	}
	final, err := service.Projects.Preferences.Snapshot()
	if err != nil || !reflect.DeepEqual(initial, final) {
		t.Fatal("source event changed other YAML settings", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestSIVIProjectIDSourceConcurrentCollisionAndMetadataPreflight(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	service.SiviParentReviewEnabled = true
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := SetSIVIProjectIDSource(service, context.Background(), state.ContextID, 2, 1)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "source changed") {
			t.Fatal("unexpected collision error", err)
		}
	}
	if success != 1 {
		t.Fatal("source CAS did not elect one writer", success)
	}
	vprotest.MutateContextFixture(t, state.ProjectPath, `ALTER TABLE Sample_Metadata ADD COLUMN ExtraGenerated TEXT GENERATED ALWAYS AS ('x') VIRTUAL`)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := GetSIVIProjectIDChoices(service, context.Background(), state.ContextID); got != nil || err == nil {
		t.Fatal("hidden metadata column accepted")
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}
