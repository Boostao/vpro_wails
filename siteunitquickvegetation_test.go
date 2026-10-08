package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryQuickVegetationPhysicalInsertionNotMaxReduction(t *testing.T) {
	veg, su, _ := vegetationReportFixture(t)
	before := cloneLifeformReferenceFixture(veg)
	got, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 2, veg, su)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 27 || len(got.Groups) != 5 || len(got.Memberships) != 8 {
		t.Fatal("physical duplicates/zero/NULL membership changed", got)
	}
	expected := map[string]float64{"P1|A": 36, "P2|A": 99, "|": -5, "P1|": 3, "P1|NULL": 0}
	for _, group := range got.Groups {
		species := "NULL"
		if group.Species.Text != nil {
			species = *group.Species.Text
		}
		value, ok := expected[group.PlotNumber+"|"+species]
		if !ok || group.MyCover != value {
			t.Fatal("nine raw covers/SU weighting/cap changed", group, expected)
		}
		delete(expected, group.PlotNumber+"|"+species)
	}
	if len(expected) != 0 || !reflect.DeepEqual(veg, before) {
		t.Fatal("groups missing or borrowed input modified", expected)
	}
	for _, entry := range got.Entries {
		if entry.Layer < 1 || entry.Layer > 9 || entry.VegRowID == "" || entry.SURowID == "" {
			t.Fatal("insertion provenance missing", entry)
		}
	}
	*got.Entries[0].Species.Text = "changed"
	*got.Memberships[0].PlotNumber.Text = "changed"
	if !reflect.DeepEqual(veg, before) || *su.Rows[0].Cells[0].Text != "P1" {
		t.Fatal("result cells alias physical inputs")
	}
}

func TestSummaryQuickVegetationSingleAssignmentCapAndIgnoredFields(t *testing.T) {
	veg, su, _ := vegetationReportFixture(t)
	veg.Rows = veg.Rows[:1]
	su.Rows = su.Rows[:1]
	for i := 2; i < len(veg.Rows[0].Cells); i++ {
		veg.Rows[0].Cells[i] = ProjectMetadataCell{Storage: "null"}
	}
	value := float64(0.1)
	veg.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &value}
	for i, column := range veg.Columns {
		if column.Name == "Cover10" || column.Name == "Cover5a" || column.Name == "TotalA" {
			veg.Rows[0].Cells[i] = metadataText("unused malformed history")
		}
	}
	got, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 1, veg, su)
	if err != nil || len(got.Entries) != 1 || got.Groups[0].MyCover != float64(float32(value)) {
		t.Fatal("original SINGLE insertion or nine-cover domain changed", got, err)
	}
	for _, cover := range []float64{0, -4, 98.5, 99, 99.5, 125} {
		veg.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &cover}
		got, err = prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 2, veg, su)
		if err != nil || len(got.Groups) != 1 || got.Groups[0].MyCover != math.Min(cover, 99) {
			t.Fatal("strict >99 cap/zero/negative history changed", cover, got, err)
		}
	}
}

func TestSummaryQuickVegetationRefusalsAreAtomicAndRetryable(t *testing.T) {
	for name, mutate := range map[string]func(*ProjectMetadataTable, *ProjectMetadataTable){
		"text": func(v, _ *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataText("2x") },
		"blob": func(v, _ *ProjectMetadataTable) {
			v.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "blob", BlobHex: metadataText("00").Text}
		},
		"overflow": func(v, _ *ProjectMetadataTable) {
			x := math.MaxFloat64
			v.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &x}
		},
		"identity":  func(v, _ *ProjectMetadataTable) { v.Rows[0].Cells[1] = metadataInteger("1") },
		"length":    func(v, _ *ProjectMetadataTable) { v.Rows[0].Cells[1] = metadataText(strings.Repeat("x", 9)) },
		"duplicate": func(v, _ *ProjectMetadataTable) { v.Rows[1].RowID = v.Rows[0].RowID },
		"schema":    func(v, _ *ProjectMetadataTable) { v.Columns[2].Name = "CoverMissing" },
		"member":    func(_, s *ProjectMetadataTable) { s.Rows[0].Cells[1] = metadataInteger("1") },
	} {
		t.Run(name, func(t *testing.T) {
			veg, su, _ := vegetationReportFixture(t)
			mutate(&veg, &su)
			got, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 2, veg, su)
			if err == nil || !reflect.DeepEqual(got, siteUnitQuickVegetation{}) {
				t.Fatal("invalid physical source returned partial preparation", got, err)
			}
			veg, su, _ = vegetationReportFixture(t)
			if _, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", 2, veg, su); err != nil {
				t.Fatal("valid retry failed", err)
			}
		})
	}
	veg, su, _ := vegetationReportFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := prepareSiteUnitQuickVegetation(ctx, "Project", "Selected", 2, veg, su); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, siteUnitQuickVegetation{}) {
		t.Fatal("cancellation returned partial preparation", got, err)
	}
	for _, remaining := range []int{1, 13, 30, 92, 95} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		if got, err := prepareSiteUnitQuickVegetation(ctx, "Project", "Selected", 2, veg, su); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, siteUnitQuickVegetation{}) {
			t.Fatal("late cancellation returned partial entries/groups", remaining, got, err)
		}
	}
	for _, order := range []int{0, 3, 4} {
		if _, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "Selected", order, veg, su); err == nil {
			t.Fatal("unsupported grouping guessed", order)
		}
	}
	if _, err := prepareSiteUnitQuickVegetation(context.Background(), "Project", "None", 2, veg, su); err == nil {
		t.Fatal("missing SU guessed")
	}
}
