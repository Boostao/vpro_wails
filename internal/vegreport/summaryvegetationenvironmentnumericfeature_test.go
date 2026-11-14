package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestSummaryEnvironmentalNumericReadSnapshotAndContextValidation(t *testing.T) {
	service, state := summaryEnvironmentalNumericServiceFixture(t, false)
	owner := service.report.contexts.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	snapshots := 0
	service.snapshot.CommitRead = func(*sql.Tx) error { snapshots++; return nil }
	result, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0)
	if result == nil || err != nil || snapshots != 1 {
		t.Fatal(result, err, snapshots)
	}
	if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, owner.Attachments)) {
		t.Fatal("numeric read mutated source")
	}
	empty := &SummaryVegetationEnvironmentalService{}
	if result, err := empty.PreviewEnvironmentalNumeric(nil, "owned", 1, 6, 0); result != nil || err == nil {
		t.Fatal("nil request context accepted", result, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := empty.PreviewEnvironmentalNumeric(ctx, "owned", 1, 6, 0); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request lost cancellation", result, err)
	}
}
