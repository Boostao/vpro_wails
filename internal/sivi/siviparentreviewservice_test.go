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
	"testing"
)

func TestSIVIParentReviewFeatureRequiresIndependentLiteralOptIn(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{
		{"", false, true, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true},
	} {
		enabled, err := SiviParentReviewFeature(func(name string) (string, bool) {
			if name != "VPRO_SIVI_PARENT_REVIEW" {
				t.Fatal("wrong independent feature", name)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
}

func TestSIVIParentReviewFacadeOwnedReadonlyAndCancellation(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	service.SiviHeightEnabled = true
	if result, err := GetSIVIParentOriginal(service, context.Background(), state.ContextID, "108050"); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("height opt-in enabled parent reader", err)
	}
	service.SiviHeightEnabled = false
	service.SiviParentReviewEnabled = true
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
	if result, err := GetSIVIVegetation(service, context.Background(), state.ContextID, "108050", false); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("parent review enabled height workflow", err)
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
