package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestSummaryVegetationClassOriginalCrosstabs(t *testing.T) {
	for _, scenario := range []string{"ordinary", "over-one", "null"} {
		for _, display := range []int{4, 5} {
			for _, show := range []int{1, 10, 20} {
				for _, average := range []int{1, 2} {
					raw, su, stage, master, personal, layers := summaryLayerFixture()
					if scenario != "ordinary" {
						personal = sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("X", "Alpha", vprotest.MetadataInteger("1"), vprotest.MetadataText("alternate"), vprotest.MetadataText("U")))
					}
					if scenario == "null" {
						for i := range stage.Rows {
							stage.Rows[i].Cells[3] = vprocore.ProjectMetadataCell{Storage: "null"}
						}
					}
					got, err := planSummaryVegetationLayerOptions(context.Background(), raw, su, stage, master, personal, layers, average, display, show)
					if err != nil {
						t.Fatal(scenario, display, show, average, err)
					}
					if scenario == "null" {
						assertSummaryLayerRows(t, got, []string{"A", "B"}, nil)
						continue
					}
					a, b, s := "3", "2", "1"
					if display == 5 {
						a, b = "4", "4"
					}
					if scenario == "over-one" {
						a, b = "5", "4"
						if display == 5 {
							a, b = "6", "6"
						}
					}
					if show == 20 {
						a, b, s = "86.60254", "50", "2.5"
						if display == 5 {
							a, b, s = "237.1708", "223.6068", "17.67767"
						}
						if scenario == "over-one" {
							a, b = "244.949", "141.4214"
							if display == 5 {
								a, b = "670.8204", "632.4555"
							}
						}
					}
					order := a
					if b > a {
						order = b
					}
					assertSummaryLayerRows(t, got, []string{"A", "B"}, [][]vprocore.ProjectMetadataCell{
						{vprotest.MetadataText("A1"), vprotest.MetadataText("Alpha"), vprotest.MetadataText(order), vprotest.MetadataText(a), vprotest.MetadataText(b)},
						{vprotest.MetadataText("A1"), vprotest.MetadataText("S"), vprotest.MetadataText(s), vprotest.MetadataText(s), {Storage: "null"}},
					})
				}
			}
		}
	}
}

func TestSummaryVegetationClassServiceOwnedDraftAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryVegetationServiceFixture(t, external)
		before := vprotest.DatabaseBytes(t, service.contexts.Projects.Sqlite.Attachments)
		config, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, display := range []int{4, 5} {
			for _, show := range []int{1, 10, 20} {
				for _, average := range []int{1, 2} {
					got, err := service.PreviewClass(context.Background(), state.ContextID, average, display, show)
					if err != nil || got.ContextID != state.ContextID || got.Average != average ||
						got.Display != display || got.ShowClass != show || len(got.Rows) != 2 ||
						len(got.Rows[0].Witnesses) != 4 || got.ProjectPath != state.ProjectPath || got.SUPath != state.SUPath {
						t.Fatal(external, average, display, show, got, err)
					}
					again, err := service.PreviewClass(context.Background(), state.ContextID, average, display, show)
					if err != nil || !reflect.DeepEqual(got, again) {
						t.Fatal("class reread differs", err)
					}
					*got.Rows[0].Covers[0].Text = "changed"
					got.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
					if *again.Rows[0].Covers[0].Text == "changed" || again.Rows[0].Witnesses[0].RawVegRowIDs[0] == "changed" {
						t.Fatal("class DTO aliases another read")
					}
					if _, err := service.Preview(context.Background(), state.ContextID, average, display); err == nil {
						t.Fatal("legacy API silently enabled class without Show")
					}
				}
			}
		}
		vprotest.AssertProfileSUFiles(t, service.contexts, before)
		after, err := os.ReadFile(service.contexts.Projects.Preferences.Path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("class drafts persisted preferences", err)
		}
	}
}

func TestSummaryVegetationClassServiceRefusalCleanupAndRetry(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, true)
	owner := service.contexts.Projects.Sqlite
	for _, options := range [][3]int{{1, 4, 0}, {1, 4, 2}, {0, 4, 10}, {1, 6, 10}, {1, 3, 20}} {
		got, err := service.PreviewClass(context.Background(), state.ContextID, options[0], options[1], options[2])
		if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
			t.Fatal("invalid explicit class options produced data", options, got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := service.PreviewClass(ctx, state.ContextID, 1, 4, 20)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal(got, err)
	}
	for _, contextID := range []string{"foreign", ""} {
		got, err := service.PreviewClass(context.Background(), contextID, 1, 4, 20)
		if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
			t.Fatal("foreign class context accepted", got, err)
		}
	}
	service.Enabled = false
	if _, err := service.PreviewClass(context.Background(), state.ContextID, 1, 4, 20); err == nil {
		t.Fatal("default gate bypassed")
	}
	service.Enabled = true
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	got, err = service.PreviewClass(context.Background(), state.ContextID, 1, 4, 20)
	if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("changed physical identity accepted", got, err)
	}
	owner.AttachmentInfo["su"] = original
	service.snapshot.CommitRead = func(*sql.Tx) error { return errors.New("class read completion failed") }
	got, err = service.PreviewClass(context.Background(), state.ContextID, 1, 4, 20)
	if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("failed class read completion published", got, err)
	}
	service.snapshot.CommitRead = nil
	service.snapshot.RollbackRead = func(*sql.Tx) error { return errors.New("class read rollback failed") }
	got, err = service.PreviewClass(context.Background(), state.ContextID, 1, 4, 20)
	if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("failed class read cleanup published", got, err)
	}
	service.snapshot.RollbackRead = nil
	if _, err := service.PreviewClass(context.Background(), state.ContextID, 2, 5, 10); err != nil {
		t.Fatal("class cleanup prevented retry", err)
	}
}

func TestSummaryVegetationClassPivotCancellationAndErrors(t *testing.T) {
	raw, su, stage, master, personal, layers := summaryLayerFixture()
	probe := &projectUnitCancellationContext{Context: context.Background()}
	if _, err := planSummaryVegetationLayerOptions(probe, raw, su, stage, master, personal, layers, 2, 5, 20); err != nil {
		t.Fatal(err)
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		got, err := planSummaryVegetationLayerOptions(ctx, raw, su, stage, master, personal, layers, 2, 5, 20)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, summaryVegetationLayerReport{}) {
			t.Fatal("cancelled class pivot leaked partial result", point, got, err)
		}
	}
	for i := range stage.Rows {
		stage.Rows[i].Cells[3] = vprotest.StrataReal(-1)
	}
	got, err := planSummaryVegetationLayerOptions(context.Background(), raw, su, stage, master, personal, layers, 1, 5, 20)
	if err == nil || !reflect.DeepEqual(got, summaryVegetationLayerReport{}) {
		t.Fatal("negative Goldstream sqrt silently succeeded", got, err)
	}
	got, err = planSummaryVegetationLayerOptions(context.Background(), raw, su, stage, master, personal, layers, 1, 4, 10)
	if err != nil || len(got.Rows) != 0 || len(got.Columns) != 2 {
		t.Fatal("negative linear class failed to retain NULL pivot distinction", got, err)
	}
}
