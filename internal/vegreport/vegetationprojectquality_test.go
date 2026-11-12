package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func projectUnitQualityFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vegetationQualitySelection) {
	env, admin := projectUnitFixture()
	_, _, su := selectedUnitFixture()
	quality := vegetationQualitySelection{Occurrences: []vegetationQualityOccurrence{
		{Membership: reportcore.VegetationReportMembership{RowID: "21", PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText("old")},
			EnvRowID: "1", AdminRowID: "11"},
	}}
	return env, admin, su, quality
}

func TestProjectUnitQualityOriginalMembershipExistence(t *testing.T) {
	env, admin, su, quality := projectUnitQualityFixture()
	quality.Occurrences = append(quality.Occurrences, quality.Occurrences[0], quality.Occurrences[0])
	quality.Occurrences = append(quality.Occurrences, vegetationQualityOccurrence{
		Membership: reportcore.VegetationReportMembership{RowID: "22", PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprocore.ProjectMetadataCell{Storage: "null"}},
		EnvRowID:   "5", AdminRowID: "12",
	})
	before, err := json.Marshal([]vprocore.ProjectMetadataTable{env, admin, su})
	if err != nil {
		t.Fatal(err)
	}
	got, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone", "Comment"}, quality)
	want := []vegetationProjectSelectedUnitOccurrence{
		{"1", "11", vprotest.MetadataText("P"), vprotest.MetadataText("A first ")},
		{"1", "12", vprotest.MetadataText("P"), vprotest.MetadataText("Asecond")},
		{"5", "11", vprotest.MetadataText("P"), vprotest.MetadataText("A2 first ")},
		{"5", "12", vprotest.MetadataText("P"), vprotest.MetadataText("A2second")},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("original quality4 occurrences must retain project4 tuples by plot existence, not quality4*project4 or selected2*project4", got, err)
	}
	after, err := json.Marshal([]vprocore.ProjectMetadataTable{env, admin, su})
	if err != nil || string(before) != string(after) {
		t.Fatal("qualification changed physical originals", err)
	}
	*got[0].PlotNumber.Text = "changed"
	*got[0].SiteUnit.Text = "changed"
	if *env.Rows[0].Cells[0].Text != "P" || *su.Rows[0].Cells[0].Text != "P" ||
		*got[1].PlotNumber.Text != "P" || *got[1].SiteUnit.Text != "Asecond" ||
		*quality.Occurrences[0].Membership.PlotNumber.Text != "P" {
		t.Fatal("qualified result aliases original/peer/quality witness")
	}
	quality.Occurrences = nil
	got, err = planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty quality must not widen to full project", got, err)
	}
}

func TestProjectUnitQualityRefusesForgedWitnesses(t *testing.T) {
	for _, mutate := range []func(*vegetationQualityOccurrence){
		func(q *vegetationQualityOccurrence) { q.Membership.RowID = "missing" },
		func(q *vegetationQualityOccurrence) { q.Membership.RowID = "1" },
		func(q *vegetationQualityOccurrence) { q.Membership.PlotNumber = vprotest.MetadataText("Q") },
		func(q *vegetationQualityOccurrence) {
			q.Membership.PlotNumber = vprocore.ProjectMetadataCell{Storage: "null"}
		},
		func(q *vegetationQualityOccurrence) { q.Membership.SiteUnit = vprotest.MetadataText("changed") },
		func(q *vegetationQualityOccurrence) { q.Membership.SiteUnit = vprotest.MetadataText("") },
		func(q *vegetationQualityOccurrence) { q.EnvRowID = "2" },
		func(q *vegetationQualityOccurrence) { q.EnvRowID = "7" },
		func(q *vegetationQualityOccurrence) { q.AdminRowID = "13" },
		func(q *vegetationQualityOccurrence) { q.AdminRowID = "16" },
	} {
		env, admin, su, quality := projectUnitQualityFixture()
		mutate(&quality.Occurrences[0])
		if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality); err == nil || rows != nil {
			t.Fatal("project qualification accepted invented/wrong/NULL original SU/Env/Admin witness", quality, rows, err)
		}
	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable){
		func(s *vprocore.ProjectMetadataTable) { s.Columns[0].Name = "missing" },
		func(s *vprocore.ProjectMetadataTable) { s.Rows[0].Cells = nil },
		func(s *vprocore.ProjectMetadataTable) { s.Rows[1].RowID = "21" },
		func(s *vprocore.ProjectMetadataTable) { s.Rows[1].Cells[0] = vprotest.MetadataText("p") },
		func(s *vprocore.ProjectMetadataTable) { s.Rows[3].Cells[0] = vprotest.MetadataInteger("7") },
		func(s *vprocore.ProjectMetadataTable) {
			s.Rows[3].Cells[1] = vprotest.MetadataText(string([]byte{0xff}))
		},
	} {
		env, admin, su, quality := projectUnitQualityFixture()
		mutate(&su)
		if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality); err == nil || rows != nil {
			t.Fatal("quality ignored malformed original selected-SU peer", rows, err)
		}
	}
	env, admin, su, quality := projectUnitQualityFixture()
	su.Rows = nil
	if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality); err == nil || rows != nil {
		t.Fatal("quality invented an original membership in an empty SU", rows, err)
	}
	su.Columns = nil
	if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, vegetationQualitySelection{}); err == nil || rows != nil {
		t.Fatal("empty quality silently supplied missing None_SU authority", rows, err)
	}
}

func TestProjectUnitQualityBuildsCompleteProjectionBeforePrune(t *testing.T) {
	for _, qualityEmpty := range []bool{false, true} {
		env, admin, su, quality := projectUnitQualityFixture()
		if qualityEmpty {
			quality.Occurrences = nil
		}
		// R is not a qualified plot, but its original project label must still
		// pass the complete Build before pruning.
		env.Rows[2].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 256))
		if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality); err == nil || rows != nil {
			t.Fatal("quality hid invalid original project label", qualityEmpty, rows, err)
		}
		env.Rows[2].Cells[1] = vprotest.MetadataText("valid")
		env.Rows[5].Cells[1] = vprotest.MetadataText("bad\x00unmatched")
		if rows, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality); err == nil || rows != nil {
			t.Fatal("quality hid malformed unmatched selected scalar", qualityEmpty, rows, err)
		}
	}
	env, admin, su, quality := projectUnitQualityFixture()
	quality.Occurrences = []vegetationQualityOccurrence{
		{Membership: reportcore.VegetationReportMembership{RowID: "23", PlotNumber: vprotest.MetadataText("Q"), SiteUnit: vprotest.MetadataText("old")},
			EnvRowID: "2", AdminRowID: "13"},
	}
	got, err := planLongVegetationQualifiedProjectSelectedUnits(context.Background(), env, admin, su, []string{"Zone"}, quality)
	want := []vegetationProjectSelectedUnitOccurrence{{"2", "13", vprotest.MetadataText("Q"), vprocore.ProjectMetadataCell{Storage: "null"}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("valid qualified NULL label must remain NULL, not disappear or become All plots", got, err)
	}
}

func TestProjectUnitQualityCancellationEveryObservation(t *testing.T) {
	env, admin, su, quality := projectUnitQualityFixture()
	run := func(ctx context.Context) (bool, error) {
		rows, err := planLongVegetationQualifiedProjectSelectedUnits(ctx, env, admin, su, []string{"Zone", "Comment"}, quality)
		return rows == nil, err
	}
	if empty, err := run(nil); err == nil || !empty {
		t.Fatal("nil context accepted", empty, err)
	}
	probe := &projectUnitCancellationContext{Context: context.Background()}
	if empty, err := run(probe); err != nil || empty || probe.calls < 2 {
		t.Fatal("successful original quality probe failed", probe, err)
	}
	for step := 1; step <= probe.calls; step++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: step}
		if empty, err := run(ctx); !errors.Is(err, context.Canceled) || !empty {
			t.Fatal("qualification leaked partial output or success after cancellation", step, empty, err)
		}
	}
}
