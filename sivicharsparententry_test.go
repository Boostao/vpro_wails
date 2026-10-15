package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type charsEntrySourceField struct {
	Binding, Column, Owner string
	Type, Size             int
	Required               bool
	AllowZeroLength        *bool
	ValidationRule         string
}

func charsEntrySourceFields(t *testing.T) []charsEntrySourceField {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "chars-sivi-entry-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SourceSHA256 string
		Fields       []charsEntrySourceField
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Fields) != 72 || len(fixture.SourceSHA256) != 64 {
		t.Fatal("independent source metadata fixture is incomplete")
	}
	return fixture.Fields
}

func charsEntryOriginal(t *testing.T) *siviParentProjection {
	t.Helper()
	env, admin := siviParentTables(t)
	for _, table := range []*ProjectMetadataTable{&env, &admin} {
		for i, column := range table.Columns {
			if column.Name != "PlotNumber" && column.Name != "Plot" {
				table.Rows[0].Cells[i] = ProjectMetadataCell{Storage: "null"}
			}
		}
	}
	original, err := projectSIVICHARSParent(context.Background(), "owned", "Sample", "108050", env, admin)
	if err != nil {
		t.Fatal(err)
	}
	return original
}

func charsEntryEdit(t *testing.T, original *siviParentProjection, column string, value ProjectMetadataCell) siviParentScalarEdit {
	t.Helper()
	for _, binding := range original.Bindings {
		if !strings.EqualFold(binding.Binding, column) {
			continue
		}
		row := original.Rows[0].Env
		if binding.Table == original.AdminTable {
			row = original.Rows[0].Admin
		}
		return siviParentScalarEdit{original.ContextID, binding.Table, row.RowID, column,
			cloneSiteUnitCell(row.Cells[binding.Column]), value}
	}
	t.Fatal("missing independent test target", column)
	return siviParentScalarEdit{}
}

func TestSIVICHARSEntryPoliciesMatchIndependentDAOFieldMetadata(t *testing.T) {
	policies, err := siviCHARSParentFields()
	if err != nil || len(policies) != 68 {
		t.Fatal("source editable cohort changed", err)
	}
	count := 0
	for _, source := range charsEntrySourceFields(t) {
		if source.Column == "PlotNumber" || source.Column == "ProjectID" ||
			source.Column == "PlotType" || source.Column == "SpeciesListComplete" {
			if _, exists := policies[source.Column]; exists {
				t.Fatal("readonly/callback source acquired direct editing", source.Column)
			}
			continue
		}
		count++
		policy, exists := policies[source.Column]
		if !exists || policy.column != source.Column || policy.binding != source.Binding || policy.owner != source.Owner || source.Required {
			t.Fatal("literal source field owner/nullable policy changed", source, policy)
		}
		expectedKind := map[int]string{1: "boolean", 3: "integer", 6: "single", 7: "double", 8: "date", 10: "text", 12: "text"}[source.Type]
		if source.Column == "SV_StandAgeEstMeas" || source.Column == "SV_StandHeightEstMeas" {
			expectedKind = "option"
		}
		if expectedKind == "" || policy.kind != expectedKind {
			t.Fatal("source storage type was inferred incorrectly", source, policy)
		}
		if source.Type == 10 || source.Type == 12 {
			if policy.maximum != source.Size {
				t.Fatal("source UTF16/memo bound changed", source, policy)
			}
			if expectedKind != "option" && (source.AllowZeroLength == nil || policy.allowEmpty != *source.AllowZeroLength) {
				t.Fatal("source NULL/empty policy changed", source, policy)
			}
		}
		if source.Column == "StartDate" {
			if source.ValidationRule != "Between 1900 And 2500" {
				t.Fatal("actual source year rule changed", source)
			}
		} else if source.ValidationRule != "" {
			t.Fatal("unreviewed source field rule", source)
		}
	}
	if count != 68 {
		t.Fatal("independent source cohort check was incomplete", count)
	}
}

func TestSIVICHARSEntryAll68DomainsAndDetachedAssignments(t *testing.T) {
	original := charsEntryOriginal(t)
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	edits := []siviParentScalarEdit{}
	expected := map[string]ProjectMetadataCell{}
	for _, source := range charsEntrySourceFields(t) {
		if source.Column == "PlotNumber" || source.Column == "ProjectID" ||
			source.Column == "PlotType" || source.Column == "SpeciesListComplete" {
			continue
		}
		var value, after ProjectMetadataCell
		switch source.Type {
		case 1:
			value = metadataInteger("-1")
		case 3:
			value = metadataInteger("-7")
			if source.Column == "StartDate" {
				value = metadataInteger("2000")
			}
		case 6:
			value, after = siviReal(0.1), siviReal(0.10000000149011612)
		case 7:
			value = siviReal(361.5)
		case 8:
			value = metadataText("2026-02-03 04:05:06")
		case 10, 12:
			value = metadataText("A")
			if source.Column == "SV_StandAgeEstMeas" || source.Column == "SV_StandHeightEstMeas" {
				value = metadataText("1")
			}
		default:
			t.Fatal("missing independent domain case", source)
		}
		if after.Storage == "" {
			after = cloneSiteUnitCell(value)
		}
		expected[source.Column] = after
		edits = append(edits, charsEntryEdit(t, original, source.Column, value))
	}
	got, err := planSIVICHARSParentEntry(context.Background(), original, edits)
	if err != nil || len(got) != 68 {
		t.Fatal("complete typed source plan failed", len(got), err)
	}
	for _, assignment := range got {
		if !metadataCellEqual(assignment.After, expected[assignment.Column]) || assignment.Before.Storage != "null" {
			t.Fatal("typed source assignment/narrowing changed", assignment)
		}
		value, err := metadataCellValue(assignment.After)
		if err != nil || !reflect.DeepEqual(assignment.Value, value) {
			t.Fatal("typed receipt and database parameter disagree", assignment, err)
		}
	}
	after, err := json.Marshal(original)
	if err != nil || string(before) != string(after) {
		t.Fatal("pure plan mutated reviewed source", err)
	}
	for _, assignment := range got {
		if assignment.After.Text != nil {
			*assignment.After.Text = "mutated"
		}
		if assignment.After.Real != nil {
			*assignment.After.Real = -99
		}
	}
	for _, edit := range edits {
		if edit.Value.Text != nil && *edit.Value.Text == "mutated" ||
			edit.Value.Real != nil && *edit.Value.Real == -99 {
			t.Fatal("assignments share caller draft ownership")
		}
	}
}

func TestSIVICHARSEntrySingleNarrowingAndSQLiteRepresentability(t *testing.T) {
	for _, test := range []struct {
		name          string
		input, output float64
		refuse        bool
	}{
		{"decimal", 0.1, 0.10000000149011612, false},
		{"no-vegetation-rule", 99.999999, 100, false},
		{"negative", -3.25, -3.25, false},
		{"positive-underflow", 1e-46, 0, false},
		{"subnormal", 1.401298464324817e-45, 1.401298464324817e-45, false},
		{"maximum", math.MaxFloat32, math.MaxFloat32, false},
		{"adjacent-above-maximum", math.Nextafter(math.MaxFloat32, math.Inf(1)), math.MaxFloat32, false},
		{"overflow", 1e39, 0, true},
		{"nonfinite", math.Inf(1), 0, true},
		{"nan", math.NaN(), 0, true},
		{"negative-zero", math.Copysign(0, -1), 0, true},
		{"negative-underflow", -1e-46, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := charsEntryOriginal(t)
			edit := charsEntryEdit(t, original, "SV_WaterTableCM", siviReal(test.input))
			got, err := planSIVICHARSParentEntry(context.Background(), original, []siviParentScalarEdit{edit})
			if test.refuse {
				if err == nil || got != nil {
					t.Fatal("unsupported new scalar produced a plan", got, err)
				}
			} else if err != nil || len(got) != 1 || got[0].After.Real == nil ||
				math.Float64bits(*got[0].After.Real) != math.Float64bits(test.output) {
				t.Fatal("independent source SINGLE witness changed", got, err)
			}
		})
	}
}

func TestSIVICHARSEntryHistoricalOmissionBeforeNewValidation(t *testing.T) {
	for _, test := range []struct {
		column string
		value  ProjectMetadataCell
	}{
		{"RootZoneParticleSize", metadataText(" old invalid width ")},
		{"RootZoneParticleSize", metadataText("")},
		{"SV_WaterTableCM", siviReal(1e100)},
		{"SV_WaterTableCM", siviReal(0.1)},
		{"SV_WaterTableCM", siviReal(math.Copysign(0, -1))},
		{"SV_FloodPlain", metadataInteger("2")},
		{"StartDate", metadataInteger("1800")},
		{"SV_StandAgeEstMeas", metadataText("?")},
		{"Date", metadataText("historical invalid date")},
	} {
		t.Run(test.column+"/"+test.value.Storage, func(t *testing.T) {
			original := charsEntryOriginal(t)
			edit := charsEntryEdit(t, original, test.column, test.value)
			for _, binding := range original.Bindings {
				if strings.EqualFold(binding.Binding, test.column) {
					if binding.Table == original.EnvTable {
						original.Rows[0].Env.Cells[binding.Column] = cloneSiteUnitCell(test.value)
					} else {
						original.Rows[0].Admin.Cells[binding.Column] = cloneSiteUnitCell(test.value)
					}
				}
			}
			edit.Expected = cloneSiteUnitCell(test.value)
			got, err := planSIVICHARSParentEntry(context.Background(), original, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged invalid historical value was reinterpreted", got, err)
			}
			edit.Value = ProjectMetadataCell{Storage: "null"}
			got, err = planSIVICHARSParentEntry(context.Background(), original, []siviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || !metadataCellEqual(got[0].Before, test.value) || got[0].After.Storage != "null" {
				t.Fatal("explicit correction lost the exact historical original", got, err)
			}
		})
	}
	original := charsEntryOriginal(t)
	edit := charsEntryEdit(t, original, "SV_WaterTableCM", siviReal(0.1))
	for _, binding := range original.Bindings {
		if binding.Binding == "SV_WaterTableCM" {
			original.Rows[0].Env.Cells[binding.Column] = siviReal(0.10000000149011612)
		}
	}
	edit.Expected = siviReal(0.10000000149011612)
	got, err := planSIVICHARSParentEntry(context.Background(), original, []siviParentScalarEdit{edit})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("rounding to unchanged storage invented an assignment/audit", got, err)
	}
}

func TestSIVICHARSEntrySourceRulesUnicodeAndNewInvalidValues(t *testing.T) {
	for _, test := range []struct {
		column string
		value  ProjectMetadataCell
		valid  bool
	}{
		{"RootZoneParticleSize", metadataText(" A b "), true},
		{"RootZoneParticleSize", metadataText("ABCDEF"), true},
		{"RootZoneParticleSize", metadataText("\U0001f600\U0001f600\U0001f600"), true},
		{"RootZoneParticleSize", metadataText("\U0001f600\U0001f600\U0001f600\U0001f600"), false},
		{"RootZoneParticleSize", metadataText("ABCDEFG"), false},
		{"RootZoneParticleSize", metadataText(""), false},
		{"RootZoneParticleSize", metadataText("bad\x00"), false},
		{"RootZoneParticleSize", metadataText(string([]byte{0xff})), false},
		{"RootZoneParticleSize", metadataInteger("1"), false},
		{"SnowCoverregime", metadataText(""), true},
		{"SV_AhorizonType", metadataText(""), true},
		{"VegNotes", metadataText(""), false},
		{"SV_StandAgeEstMeas", metadataText("1"), true},
		{"SV_StandAgeEstMeas", metadataText(""), false},
		{"SV_FloodPlain", metadataInteger("-1"), true},
		{"SV_FloodPlain", metadataInteger("1"), false},
		{"StartDate", metadataInteger("1900"), true},
		{"StartDate", metadataInteger("2500"), true},
		{"StartDate", metadataInteger("1899"), false},
		{"StartDate", metadataInteger("2501"), false},
		{"Elevation", metadataInteger("-32768"), true},
		{"Elevation", metadataInteger("32768"), false},
		{"Latitude", siviReal(99), true},
		{"Longitude", siviReal(361), true},
		{"Date", metadataText("2024-02-29 01:02:03.25"), true},
		{"Date", metadataText("2023-02-29 01:02:03"), false},
	} {
		t.Run(test.column, func(t *testing.T) {
			original := charsEntryOriginal(t)
			edit := charsEntryEdit(t, original, test.column, test.value)
			got, err := planSIVICHARSParentEntry(context.Background(), original, []siviParentScalarEdit{edit})
			if test.valid {
				if err != nil || len(got) != 1 {
					t.Fatal("valid source-domain value refused", got, err)
				}
			} else if err == nil || got != nil {
				t.Fatal("new invalid source-domain value returned success", got, err)
			}
		})
	}
}

func TestSIVICHARSEntryRefusesForgedIdentityAndStalePlans(t *testing.T) {
	for _, kind := range []string{"normal-form", "query", "env-table", "binding", "binding-index",
		"missing-binding", "duplicate-pair", "context", "table", "row", "expected", "duplicate-edit",
		"readonly", "normal-only", "signed-zero-expected", "signed-zero-new"} {
		t.Run(kind, func(t *testing.T) {
			original := charsEntryOriginal(t)
			edit := charsEntryEdit(t, original, "SV_WaterTableCM", siviReal(0.1))
			edits := []siviParentScalarEdit{edit}
			switch kind {
			case "normal-form":
				original.Form = "frmSIVIsite"
			case "query":
				original.Query = "invented"
			case "env-table":
				original.EnvTable = "foreign"
			case "binding":
				original.Bindings[0].ControlID = "foreign"
			case "binding-index":
				original.Bindings[0].Column++
			case "missing-binding":
				original.Bindings = original.Bindings[1:]
			case "duplicate-pair":
				original.Rows = append(original.Rows, original.Rows[0])
			case "context":
				edits[0].ContextID = "stale"
			case "table":
				edits[0].Table = "foreign_Env"
			case "row":
				edits[0].RowID = "9999999"
			case "expected":
				edits[0].Expected = siviReal(0)
			case "duplicate-edit":
				edits = append(edits, edit)
			case "readonly":
				edits[0] = charsEntryEdit(t, original, "SpeciesListComplete", metadataInteger("-1"))
			case "normal-only":
				edits[0].Column = "SeepageDepth"
			case "signed-zero-expected", "signed-zero-new":
				for _, binding := range original.Bindings {
					if binding.Binding == "SV_WaterTableCM" {
						original.Rows[0].Env.Cells[binding.Column] = siviReal(0)
					}
				}
				edits[0].Expected = siviReal(0)
				if kind == "signed-zero-expected" {
					edits[0].Expected = siviReal(math.Copysign(0, -1))
				} else {
					edits[0].Value = siviReal(math.Copysign(0, -1))
				}
			}
			if got, err := planSIVICHARSParentEntry(context.Background(), original, edits); err == nil || got != nil {
				t.Fatal("forged/stale/unsupported target returned partial plan", got, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	original := charsEntryOriginal(t)
	edit := charsEntryEdit(t, original, "SV_WaterTableCM", siviReal(0.1))
	if got, err := planSIVICHARSParentEntry(ctx, original, []siviParentScalarEdit{edit}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled request returned a plan", got, err)
	}
	for _, parent := range []*siviParentProjection{nil, original} {
		if got, err := planSIVICHARSParentEntry(context.Background(), parent, nil); err == nil || got != nil {
			t.Fatal("empty planning request returned success", got, err)
		}
	}
}
