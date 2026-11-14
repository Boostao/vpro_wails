package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationUnitPreferencesStrictSharedClientWire(t *testing.T) {
	var wire struct {
		Review  LongVegetationUnitPreferencesReview  `json:"review"`
		Request LongVegetationUnitPreferencesRequest `json:"request"`
		Outcome LongVegetationUnitPreferencesOutcome `json:"outcome"`
	}
	raw, err := os.ReadFile(filepath.Join("frontend", "tests", "fixtures", "longVegetationUnitWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "unit wire", "review", "request", "outcome"); err != nil {
		t.Fatal(err)
	}
	contexts, state := vprotest.ReportServiceFixture(t, true)
	service := NewLongVegetationUnitPreferencesService(contexts)
	read, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	read.ContextID, read.ProjectPath, read.SUPath = wire.Review.ContextID, wire.Review.ProjectPath, wire.Review.SUPath
	if !reflect.DeepEqual(*read, wire.Review) {
		t.Fatal("production owned preference wire differs", read, wire.Review)
	}
	outcome := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 1, 3))
	outcome.ContextID = wire.Outcome.ContextID
	if *outcome != wire.Outcome {
		t.Fatal("production source3 receipt differs", outcome, wire.Outcome)
	}
	roundtrip, err := json.Marshal(struct {
		Review  LongVegetationUnitPreferencesReview  `json:"review"`
		Request LongVegetationUnitPreferencesRequest `json:"request"`
		Outcome LongVegetationUnitPreferencesOutcome `json:"outcome"`
	}{*read, wire.Request, *outcome})
	if err != nil {
		t.Fatal(err)
	}
	var reopened map[string]any
	var original map[string]any
	if err := json.Unmarshal(roundtrip, &reopened); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &original); err != nil || !reflect.DeepEqual(reopened, original) {
		t.Fatal("shared client fixture does not match production serialization", err)
	}
}
