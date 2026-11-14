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
	"testing"
)

func TestSIVIParentReviewFacadeOwnedReadonlyAndCancellation(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	got, err := GetSIVIParentOriginal(service, context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("public reader normalized originals", err)
	}
	*got.Rows[0].Env.Cells[0].Text = "caller"
	again, err := GetSIVIParentOriginal(service, context.Background(), state.ContextID, "108050")
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("reader shares source ownership", err)
	}
	if result, err := GetSIVIParentOriginal(service, context.Background(), "stale", "108050"); result != nil || err == nil {
		t.Fatal("reader accepted stale context", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := GetSIVIParentOriginal(service, cancelled, state.ContextID, "108050"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("reader lost cancellation", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !reflect.DeepEqual(config, after) {
		t.Fatal("readonly facade changed configuration", err)
	}
}
