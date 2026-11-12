package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSIVIParentScalarPlanAllSourceFieldsAndBounds(t *testing.T) {
	columns := []string{"SV_StandHeight", "SV_AhorizonDepth", "SV_GleyingMottlingCM", "SV_PercentCoarseFrags", "SV_SoilDepth", "StrataCoverTotal"}
	for _, column := range columns {
		t.Run(column, func(t *testing.T) {
			env, admin, edit := sftest.SiviScalarFixture(t, column, vprotest.SiviReal(2))
			for _, number := range []float64{-math.MaxFloat32, math.MaxFloat32, -3, 0, 125, 0.1} {
				edit.Value = vprotest.SiviReal(number)
				got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
				if err != nil || len(got) != 1 || got[0].Value != number || got[0].Column != column ||
					got[0].ContextID != "owned" || got[0].RowID != edit.RowID || got[0].Table != edit.Table {
					t.Fatal("SINGLE range/precision or physical ownership changed", got, err)
				}
			}
			edit.Value = vprocore.ProjectMetadataCell{Storage: "null"}
			got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || got[0].Value != nil {
				t.Fatal("explicit nullable numeric change lost", got, err)
			}
			for _, invalid := range []vprocore.ProjectMetadataCell{
				vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
				vprotest.SiviReal(math.Nextafter(-float64(math.MaxFloat32), math.Inf(-1))),
				vprotest.SiviReal(math.NaN()), vprotest.SiviReal(math.Inf(1)), vprotest.MetadataText("3"), vprotest.MetadataInteger("3"),
			} {
				edit.Value = invalid
				if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("invalid new numeric value returned a plan", got, err)
				}
			}
		})
	}
}

func TestSIVIParentScalarFloodplainStorageAndHistoricalOmission(t *testing.T) {
	env, admin, edit := sftest.SiviScalarFixture(t, "SV_FloodPlain", vprotest.MetadataInteger("2"))
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("-1"), vprotest.MetadataInteger("0"), {Storage: "null"}} {
		edit.Value = cell
		got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].Before, vprotest.MetadataInteger("2")) || !reflect.DeepEqual(got[0].After, cell) {
			t.Fatal("BOOLEAN normalization rewrote original history or new storage", got, err)
		}
		if cell.Storage == "integer" && got[0].Value != map[string]int64{"-1": -1, "0": 0}[*cell.Integer] {
			t.Fatal("new true must remain-1", got)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.MetadataInteger("3"), vprotest.SiviReal(-1), vprotest.MetadataText("true")} {
		edit.Value = cell
		if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
			t.Fatal("non-Access BOOLEAN accepted as a new assignment", got, err)
		}
	}
	blob := "00ff"
	for _, column := range []string{"SV_StandHeight", "SV_FloodPlain", "StrataCoverTotal"} {
		for _, historical := range []vprocore.ProjectMetadataCell{
			vprotest.MetadataInteger("2"), vprotest.MetadataText("  historical  "), vprotest.SiviReal(math.MaxFloat64),
			{Storage: "blob", BlobHex: &blob}, {Storage: "null"},
		} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, historical)
			edit.Value = historical
			got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged invalid historical cell became an assignment", column, got, err)
			}
		}
	}
}

func TestSIVIParentScalarPlanRejectsForeignRepeatedStaleAndExcludedTargets(t *testing.T) {
	env, admin, valid := sftest.SiviScalarFixture(t, "SV_StandHeight", vprotest.SiviReal(2))
	valid.Value = vprotest.SiviReal(3)
	for _, kind := range []string{"context", "table", "row", "row-alias", "expected", "duplicate", "malformed", "text", "option", "plot-type", "implicit"} {
		t.Run(kind, func(t *testing.T) {
			invalid := valid
			switch kind {
			case "context":
				invalid.ContextID = "stale"
			case "table":
				invalid.Table = "Sample_Admin"
			case "row":
				invalid.RowID = "-777"
			case "row-alias":
				invalid.RowID = "0" + valid.RowID
			case "expected":
				invalid.Expected = vprotest.SiviReal(4)
			case "malformed":
				invalid.Value = vprocore.ProjectMetadataCell{Storage: "real"}
			case "text":
				invalid.Column = "SV_PolygonNumber"
			case "option":
				invalid.Column = "SV_StandAgeEstMeas"
			case "plot-type":
				invalid.Column = "PlotType"
			case "implicit":
				invalid.Column = "SpeciesListComplete"
			}
			if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{valid, invalid}); err == nil || got != nil {
				t.Fatal("invalid tail leaked partial plan", kind, got, err)
			}
		})
	}
	for _, duplicate := range []string{"env", "admin"} {
		env, admin, edit := sftest.SiviScalarFixture(t, "SV_StandHeight", vprotest.SiviReal(2))
		if duplicate == "env" {
			env.Rows = append(env.Rows, env.Rows[0])
			env.Rows[1].RowID = "-900"
		} else {
			admin.Rows = append(admin.Rows, admin.Rows[0])
			admin.Rows[1].RowID = "-900"
		}
		if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
			t.Fatal("ambiguous joined parents authorized a scalar plan", got, err)
		}
	}
	for _, plot := range []string{"missing", "108050 "} {
		if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", plot, env, admin, []sourceform.SiviParentScalarEdit{valid}); err == nil || got != nil {
			t.Fatal("foreign/orphan literal parent accepted", got, err)
		}
	}
	if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{valid}); err == nil || got != nil {
		t.Fatal("unowned context accepted", got, err)
	}
	if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, nil); err == nil || got != nil {
		t.Fatal("implicit empty edit request accepted", got, err)
	}
}

func TestSIVIParentScalarPlanCloneIsolationAndInFlightCancellation(t *testing.T) {
	env, admin, edit := sftest.SiviScalarFixture(t, "SV_StandHeight", vprotest.SiviReal(2))
	edit.Value = vprotest.SiviReal(3)
	got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}

	*got[0].Before.Real, *got[0].After.Real = 8, 9
	if *edit.Expected.Real != 2 || *edit.Value.Real != 3 {
		t.Fatal("assignment aliases source/edit storage")
	}
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000}
	if _, err := sourceform.ProjectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []int{1, 10000 - probe.Remaining + 2} {
		ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: remaining}
		got, err := sourceform.PlanSIVIParentScalars(ctx, "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled scalar planning returned output", got, err)
		}
	}
}

func TestSIVIParentScalarMixedEnvAdminPlanRetainsSeparateOwners(t *testing.T) {
	env, admin, height := sftest.SiviScalarFixture(t, "SV_StandHeight", vprotest.SiviReal(2))
	height.Value = vprotest.SiviReal(0.1)
	var total sourceform.SiviParentScalarEdit
	for i, column := range admin.Columns {
		if column.Name == "StrataCoverTotal" {
			admin.Rows[0].Cells[i] = vprotest.SiviReal(-7)
			total = sourceform.SiviParentScalarEdit{
				ContextID: "owned", Table: "Sample_Admin", RowID: admin.Rows[0].RowID,
				Column: column.Name, Expected: vprotest.SiviReal(-7), Value: vprotest.SiviReal(125),
			}
		}
	}
	got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{height, total})
	if err != nil || len(got) != 2 || got[0].Table != "Sample_Env" || got[1].Table != "Sample_Admin" ||
		got[0].RowID != env.Rows[0].RowID || got[1].RowID != admin.Rows[0].RowID ||
		got[0].Value != 0.1 || got[1].Value != float64(125) {
		t.Fatal("mixed parent plan collapsed table ownership or invented percent/precision constraints", got, err)
	}
}
