package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func strataFixture(values map[string]ProjectMetadataCell) VegetationReportPreparation {
	columns := longVegetationCoverColumns()
	row := VegetationReportReducedRow{PlotNumber: "P", Species: metadataText("S"),
		SourceRowIDs: []string{"1", "2"}, Covers: make([]ProjectMetadataCell, len(columns))}
	for i, column := range columns {
		row.Covers[i] = ProjectMetadataCell{Storage: "null"}
		if value, present := values[column]; present {
			row.Covers[i] = value
		}
	}
	return VegetationReportPreparation{Project: "Project", SU: "Selected",
		CoverColumns: columns, ReducedRows: []VegetationReportReducedRow{row}}
}

func strataReal(value float64) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "real", Real: &value}
}

func TestLongVegetationStrataOverridesFallbacksAndNonNULLChildren(t *testing.T) {
	for _, test := range []struct {
		name   string
		values map[string]ProjectMetadataCell
		want   map[string]float64
	}{
		{"all NULL", nil, map[string]float64{}},
		{"A cap", map[string]ProjectMetadataCell{"Cover1": strataReal(60), "Cover2": strataReal(60)}, map[string]float64{"1": 99}},
		{"A explicit uncapped", map[string]ProjectMetadataCell{"TotalA": metadataInteger("120"), "Cover1": strataReal(1)}, map[string]float64{"1": 120}},
		{"A explicit zero", map[string]ProjectMetadataCell{"TotalA": strataReal(0), "Cover1": strataReal(60)}, map[string]float64{}},
		{"A explicit negative", map[string]ProjectMetadataCell{"TotalA": strataReal(-1), "Cover1": strataReal(60)}, map[string]float64{}},
		{"A negative fallback", map[string]ProjectMetadataCell{"Cover1": strataReal(-1)}, map[string]float64{}},
		{"A positive after negatives", map[string]ProjectMetadataCell{"Cover1": strataReal(-5), "Cover3": strataReal(6)}, map[string]float64{"1": 1}},
		{"A ordered floating addition", map[string]ProjectMetadataCell{"Cover1": strataReal(1e16), "Cover2": strataReal(1), "Cover3": strataReal(-1e16)}, map[string]float64{}},
		{"B extended shrubs", map[string]ProjectMetadataCell{"Cover4": strataReal(1), "Cover5": strataReal(2),
			"Cover5a": strataReal(3), "Cover5b": strataReal(4), "Cover5c": strataReal(5)}, map[string]float64{"4": 15}},
		{"B extended cap", map[string]ProjectMetadataCell{"Cover5a": strataReal(40), "Cover5b": strataReal(40), "Cover5c": strataReal(40)}, map[string]float64{"4": 99}},
		{"B explicit uncapped", map[string]ProjectMetadataCell{"TotalB": strataReal(101), "Cover5a": strataReal(40)}, map[string]float64{"4": 101}},
		{"B explicit zero", map[string]ProjectMetadataCell{"TotalB": metadataInteger("0"), "Cover5c": strataReal(40)}, map[string]float64{}},
		{"C D nonNULL zero negative", map[string]ProjectMetadataCell{"Cover6": metadataInteger("0"), "Cover7": strataReal(-2)}, map[string]float64{"6": 0, "7": -2}},
		{"8 9 10 are not strata children", map[string]ProjectMetadataCell{"Cover8": strataReal(10), "Cover9": strataReal(20), "Cover10": strataReal(30)}, map[string]float64{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			prepared := strataFixture(test.values)
			got, err := prepareLongVegetationStrata(context.Background(), prepared)
			if err != nil || got == nil || len(got) != len(test.want) {
				t.Fatal("unexpected projection", got, err)
			}
			for _, row := range got {
				number, err := vegetationReportNumber(row.Cover)
				if err != nil || number == nil {
					t.Fatal(row, err)
				}
				value, _ := number.Float64()
				if expected, present := test.want[row.Layer]; !present || value != expected ||
					!reflect.DeepEqual(row.SourceRowIDs, []string{"1", "2"}) {
					t.Fatal("source totals/cap/NULL/provenance changed", row, test.want)
				}
			}
		})
	}
}

func TestLongVegetationStrataExistingMAXAndIndependentSQLiteProjection(t *testing.T) {
	veg, su, layers := vegetationReportFixture(t)
	for _, index := range []int{0, 1} {
		for i := 2; i < len(veg.Rows[index].Cells); i++ {
			veg.Rows[index].Cells[i] = ProjectMetadataCell{Storage: "null"}
		}
	}
	veg.Rows[0].Cells[2], veg.Rows[1].Cells[3] = strataReal(40), strataReal(40)
	prepared, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareLongVegetationStrata(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range got {
		if row.PlotNumber == "P1" && row.Species.Text != nil && *row.Species.Text == "A" && row.Layer == "1" {
			found = row.Cover.Real != nil && *row.Cover.Real == 80 && reflect.DeepEqual(row.SourceRowIDs, []string{"1", "2"})
		}
	}
	if !found {
		t.Fatal("strata summed raw rows or selected a first row instead of existing per-field MAX", got)
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "strata.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	fields := []string{"PlotNumber TEXT", "Species TEXT"}
	for _, column := range prepared.CoverColumns {
		fields = append(fields, column+" REAL")
	}
	if _, err := db.Exec("CREATE TABLE reduced(" + strings.Join(fields, ",") + ")"); err != nil {
		t.Fatal(err)
	}
	for _, row := range prepared.ReducedRows {
		values := []any{row.PlotNumber, row.Species.Text}
		for _, cell := range row.Covers {
			value, err := metadataCellValue(cell)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, value)
		}
		if _, err := db.Exec("INSERT INTO reduced VALUES("+strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")+")", values...); err != nil {
			t.Fatal(err)
		}
	}
	a := "COALESCE(TotalA,MIN(99,COALESCE(Cover1,0)+COALESCE(Cover2,0)+COALESCE(Cover3,0)))"
	b := "COALESCE(TotalB,MIN(99,COALESCE(Cover4,0)+COALESCE(Cover5,0)+COALESCE(Cover5a,0)+COALESCE(Cover5b,0)+COALESCE(Cover5c,0)))"
	rows, err := db.Query("SELECT PlotNumber,Species,'1'," + a + " FROM reduced WHERE " + a + ">0 UNION ALL " +
		"SELECT PlotNumber,Species,'4'," + b + " FROM reduced WHERE " + b + ">0 UNION ALL " +
		"SELECT PlotNumber,Species,'6',Cover6 FROM reduced WHERE Cover6 IS NOT NULL UNION ALL " +
		"SELECT PlotNumber,Species,'7',Cover7 FROM reduced WHERE Cover7 IS NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	expected, actual := []string{}, []string{}
	for rows.Next() {
		var plot, layer string
		var species sql.NullString
		var cover float64
		if err := rows.Scan(&plot, &species, &layer, &cover); err != nil {
			t.Fatal(err)
		}
		cell := ProjectMetadataCell{Storage: "null"}
		if species.Valid {
			cell = metadataText(species.String)
		}
		expected = append(expected, fmt.Sprintf("%q/%q/%s/%g", plot, vegetationTextKey(cell), layer, cover))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, row := range got {
		number, err := vegetationReportNumber(row.Cover)
		if err != nil {
			t.Fatal(err)
		}
		cover, _ := number.Float64()
		actual = append(actual, fmt.Sprintf("%q/%q/%s/%g", row.PlotNumber, vegetationTextKey(row.Species), row.Layer, cover))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("strata differs from independent typed-numeric source-SQL translation", actual, expected)
	}
}

func TestLongVegetationStrataCloneErrorsAndCancellation(t *testing.T) {
	prepared := strataFixture(map[string]ProjectMetadataCell{"TotalA": metadataInteger("111"), "Cover6": strataReal(-2)})
	got, err := prepareLongVegetationStrata(context.Background(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	*got[0].Species.Text, *got[0].Cover.Integer, got[0].SourceRowIDs[0] = "changed", "1", "changed"
	*got[1].Cover.Real = 8
	if *prepared.ReducedRows[0].Species.Text != "S" || *prepared.ReducedRows[0].Covers[13].Integer != "111" ||
		*prepared.ReducedRows[0].Covers[8].Real != -2 || prepared.ReducedRows[0].SourceRowIDs[0] != "1" ||
		*got[1].Species.Text != "S" || got[1].SourceRowIDs[0] != "1" {
		t.Fatal("strata source/output aliases")
	}
	for _, change := range []func(*VegetationReportPreparation){
		func(p *VegetationReportPreparation) { p.SU = "None" },
		func(p *VegetationReportPreparation) { p.Project = "\xff" },
		func(p *VegetationReportPreparation) { p.CoverColumns = p.CoverColumns[:14] },
		func(p *VegetationReportPreparation) { p.CoverColumns[0] = "Cover2" },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].Covers = nil },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].Species = metadataInteger("1") },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].PlotNumber = "bad\x00plot" },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].SourceRowIDs = nil },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].SourceRowIDs[1] = "1" },
		func(p *VegetationReportPreparation) { p.ReducedRows = append(p.ReducedRows, p.ReducedRows[0]) },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].Covers[0] = metadataText("1") },
		func(p *VegetationReportPreparation) { p.ReducedRows[0].Covers[0] = strataReal(math.Inf(1)) },
		func(p *VegetationReportPreparation) {
			p.ReducedRows[0].Covers[0], p.ReducedRows[0].Covers[1] = strataReal(1e308), strataReal(1e308)
		},
	} {
		input := strataFixture(nil)
		change(&input)
		result, err := prepareLongVegetationStrata(context.Background(), input)
		if err == nil || result != nil {
			t.Fatal("invalid strata input returned partial/success-shaped output", result, err)
		}
	}
	for _, remaining := range []int{1, 3, 5} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		result, err := prepareLongVegetationStrata(ctx, prepared)
		if !errors.Is(err, context.Canceled) || result != nil {
			t.Fatal("cancelled strata input returned partial output", result, err)
		}
	}
}
