package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func vegetationReportFixture(t *testing.T) (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "long-vegetation-cover-columns.txt"))
	if err != nil {
		t.Fatal(err)
	}
	veg := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}}
	for _, field := range strings.Fields(string(source)) {
		veg.Columns = append(veg.Columns, ProjectMetadataColumn{Name: field})
	}
	add := func(plot, species ProjectMetadataCell, values map[string]ProjectMetadataCell) {
		row := ProjectMetadataRow{RowID: fmt.Sprint(len(veg.Rows) + 1), Cells: []ProjectMetadataCell{plot, species}}
		for _, column := range veg.Columns[2:] {
			cell := ProjectMetadataCell{Storage: "null"}
			if value, present := values[column.Name]; present {
				cell = value
			}
			row.Cells = append(row.Cells, cell)
		}
		veg.Rows = append(veg.Rows, row)
	}
	real := func(value float64) ProjectMetadataCell { return ProjectMetadataCell{Storage: "real", Real: &value} }
	add(metadataText("P1"), metadataText("A"), map[string]ProjectMetadataCell{
		"Cover1": metadataInteger("-2"), "Cover2": metadataInteger("0"), "Cover5a": real(2),
		"Cover8": real(8), "Cover9": real(9), "Cover10": real(10), "TotalA": real(111)})
	add(metadataText("P1"), metadataText("A"), map[string]ProjectMetadataCell{
		"Cover1": real(-3), "Cover2": real(0), "Cover5a": real(4), "Cover5b": real(0), "Cover5c": real(-1), "TotalB": real(33)})
	add(metadataText("P2"), metadataText("A"), map[string]ProjectMetadataCell{
		"Cover1": metadataInteger("9007199254740993"), "Cover5c": real(120)})
	add(metadataText("P2"), metadataText("A"), map[string]ProjectMetadataCell{
		"Cover1": real(9007199254740992), "Cover5a": real(-2)})
	add(metadataText("OUT"), metadataText("A"), map[string]ProjectMetadataCell{"Cover1": metadataText("bad outside scope")})
	add(metadataText(""), metadataText(""), map[string]ProjectMetadataCell{"Cover7": real(-5)})
	add(metadataText("P1"), ProjectMetadataCell{Storage: "null"}, map[string]ProjectMetadataCell{"Cover3": real(0)})
	add(ProjectMetadataCell{Storage: "null"}, metadataText("A"), map[string]ProjectMetadataCell{"Cover1": metadataText("no SQL NULL join")})
	add(metadataText("P1"), metadataText(""), map[string]ProjectMetadataCell{"Cover3": real(1)})
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for _, values := range [][2]ProjectMetadataCell{
		{metadataText("P1"), metadataText("U")}, {metadataText("P1"), metadataText("U")},
		{metadataText("P1"), metadataText("V")}, {metadataText("P2"), {Storage: "null"}},
		{metadataText("P3"), metadataText("Orphan")}, {metadataText(""), metadataText("")},
		{{Storage: "null"}, metadataText("Unassigned")}, {metadataText("p1"), metadataText("lowercase")},
	} {
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: fmt.Sprint(len(su.Rows) + 1), Cells: values[:]})
	}
	layers := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "LayerText"}}}
	for _, layer := range []string{"1", "2", "3", "4", "5", "5A", "5b", "5c", "6", "7", "8", "9", "10"} {
		layers.Rows = append(layers.Rows, ProjectMetadataRow{RowID: fmt.Sprint(len(layers.Rows) + 1), Cells: []ProjectMetadataCell{metadataText(layer)}})
	}
	layers.Rows = append(layers.Rows, ProjectMetadataRow{RowID: "14", Cells: []ProjectMetadataCell{{Storage: "null"}}})
	return veg, su, layers
}

func TestLongVegetationPreparationSourceReductionAndTypedObservations(t *testing.T) {
	veg, su, layers := vegetationReportFixture(t)
	result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "long-vegetation-cover-columns.txt"))
	if err != nil || strings.Join(result.CoverColumns, "\n")+"\n" != strings.ReplaceAll(string(expected), "\r\n", "\n") {
		t.Fatal("original fifteen cover/total reduction fields changed", err)
	}
	if len(result.Memberships) != 8 || len(result.ReducedRows) != 5 || len(result.Layers) != 13 || len(result.Observations) != 14 {
		t.Fatal("NULL/duplicate/orphan membership or source conversion lost", result)
	}
	if result.Memberships[6].PlotNumber.Storage != "null" || *result.Memberships[5].PlotNumber.Text != "" ||
		result.Memberships[3].SiteUnit.Storage != "null" {
		t.Fatal("NULL/empty physical identities repaired", result.Memberships)
	}
	for _, row := range result.ReducedRows {
		if row.PlotNumber == "P1" && row.Species.Text != nil && *row.Species.Text == "A" {
			if !reflect.DeepEqual(row.SourceRowIDs, []string{"1", "2"}) || *row.Covers[0].Integer != "-2" ||
				*row.Covers[1].Integer != "0" || *row.Covers[5].Real != 4 || *row.Covers[13].Real != 111 || *row.Covers[14].Real != 33 {
				t.Fatal("per-field MAX, ties or independent totals changed", row)
			}
		}
		if row.PlotNumber == "P2" && *row.Covers[0].Integer != "9007199254740993" {
			t.Fatal("integer MAX was rounded through float64", row)
		}
	}
	seen := map[string]bool{}
	for _, observation := range result.Observations {
		seen[observation.Layer] = true
		if observation.Layer == "5A" && observation.PlotNumber == "P1" && *observation.Cover.Real != 4 {
			t.Fatal("literal layer spelling or cover mapping changed", observation)
		}
		if observation.Cover.Storage == "null" {
			t.Fatal("NULL imputed as cover")
		}
	}
	for _, layer := range []string{"2", "5A", "5b", "5c", "8", "9", "10"} {
		if !seen[layer] {
			t.Fatal("source non-NULL layer omitted", layer)
		}
	}
	layers.Rows = append(layers.Rows, ProjectMetadataRow{RowID: "15", Cells: []ProjectMetadataCell{metadataText("5a")}})
	repeated, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil || len(repeated.Observations) != 16 || len(repeated.Layers) != 14 {
		t.Fatal("duplicate layer metadata silently collapsed", repeated, err)
	}
}

func TestLongVegetationPreparationMatchesDisposableSQLiteMAX(t *testing.T) {
	veg, su, layers := vegetationReportFixture(t)
	result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for name, table := range map[string]ProjectMetadataTable{"Veg": veg, "Selected_SU": su} {
		columns, placeholders := []string{}, []string{}
		for _, column := range table.Columns {
			columns = append(columns, quoteHeaderIdentifier(column.Name))
			placeholders = append(placeholders, "?")
		}
		if _, err := db.Exec("CREATE TABLE " + name + "(" + strings.Join(columns, ",") + ")"); err != nil {
			t.Fatal(err)
		}
		for _, row := range table.Rows {
			args := []any{}
			for _, cell := range row.Cells {
				value, err := metadataCellValue(cell)
				if err != nil {
					t.Fatal(err)
				}
				args = append(args, value)
			}
			if _, err := db.Exec("INSERT INTO "+name+" VALUES("+strings.Join(placeholders, ",")+")", args...); err != nil {
				t.Fatal(err)
			}
		}
	}
	fields := []string{"Veg.PlotNumber", "Veg.Species"}
	for _, column := range veg.Columns[2:] {
		name := quoteHeaderIdentifier(column.Name)
		fields = append(fields, "MAX(Veg."+name+") AS "+name)
	}
	if _, err := db.Exec(`CREATE TABLE Reduced AS SELECT ` + strings.Join(fields, ",") +
		` FROM Veg INNER JOIN Selected_SU ON Veg.PlotNumber=Selected_SU.PlotNumber GROUP BY Veg.PlotNumber,Veg.Species`); err != nil {
		t.Fatal(err)
	}
	observed, err := readSQLiteStorageRows(context.Background(), db, "main", "Reduced", "", nil, "")
	if err != nil || len(observed.Rows) != len(result.ReducedRows) {
		t.Fatal("disposable SQL reduction differs", observed, err)
	}
	for _, row := range observed.Rows {
		found := false
		for _, planned := range result.ReducedRows {
			if *row.Cells[0].Text == planned.PlotNumber && reflect.DeepEqual(row.Cells[1], planned.Species) {
				found = true
				if !reflect.DeepEqual(row.Cells[2:], planned.Covers) {
					t.Fatal("independent SQL MAX differs", row.Cells[2:], planned.Covers)
				}
			}
		}
		if !found {
			t.Fatal("SQL group missing", row)
		}
	}
}

func TestLongVegetationPreparationOriginalLayerMetadataReadOnly(t *testing.T) {
	veg, su, _ := vegetationReportFixture(t)
	path, err := filepath.Abs(filepath.Join("resources", "database-family", "VPro64.db"))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	layers, err := readSQLiteStorageRows(context.Background(), db, "main", "LayerCode", "", nil, "")
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil || len(layers.Rows) != 17 || len(result.Layers) != 13 || len(result.Observations) != 14 {
		t.Fatal("bundled native layer metadata incompatible", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("read-only source metadata changed", err)
	}
}

func TestLongVegetationPreparationDeterminismIndependenceAndZeroTies(t *testing.T) {
	veg, su, layers := vegetationReportFixture(t)
	expected, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(882))
	for n := 0; n < 30; n++ {
		for _, table := range []*ProjectMetadataTable{&veg, &su, &layers} {
			rng.Shuffle(len(table.Rows), func(i, j int) { table.Rows[i], table.Rows[j] = table.Rows[j], table.Rows[i] })
		}
		result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
		if err != nil || !reflect.DeepEqual(result, expected) {
			t.Fatal("physical order changes preparation", n, err)
		}
	}
	before, _ := json.Marshal([]ProjectMetadataTable{veg, su, layers})
	*expected.Memberships[0].SiteUnit.Text = "changed"
	expected.CoverColumns[0] = "changed"
	expected.Layers[0].Layer = "changed"
	expected.ReducedRows[0].SourceRowIDs[0] = "changed"
	for _, row := range expected.ReducedRows {
		if row.Species.Text != nil {
			*row.Species.Text = "changed"
		}
		for _, cell := range row.Covers {
			if cell.Real != nil {
				*cell.Real = 999
			}
			if cell.Integer != nil {
				*cell.Integer = "999"
			}
		}
	}
	after, _ := json.Marshal([]ProjectMetadataTable{veg, su, layers})
	if !reflect.DeepEqual(before, after) {
		t.Fatal("output aliases original physical snapshots")
	}
	if expected.Observations[0].Cover.Real != nil && *expected.Observations[0].Cover.Real == 999 {
		t.Fatal("observations alias reduced cover values")
	}
	forward, backward := []byte(nil), []byte(nil)
	for n := 0; n < 2; n++ {
		veg, su, layers = vegetationReportFixture(t)
		zero, negativeZero := 0.0, math.Copysign(0, -1)
		veg.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &zero}
		veg.Rows[1].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &negativeZero}
		if n == 1 {
			veg.Rows[0], veg.Rows[1] = veg.Rows[1], veg.Rows[0]
		}
		result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			forward = raw
		} else {
			backward = raw
		}
	}
	if !reflect.DeepEqual(forward, backward) {
		t.Fatal("signed-zero numeric tie changes JSON by row order")
	}
}

type vegetationCancelContext struct {
	context.Context
	remaining int
}

func (ctx *vegetationCancelContext) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestLongVegetationPreparationCancellationAndMalformedStorage(t *testing.T) {
	for _, change := range []func(*ProjectMetadataTable, *ProjectMetadataTable, *ProjectMetadataTable){
		func(v, s, l *ProjectMetadataTable) { v.Columns = v.Columns[:len(v.Columns)-1] },
		func(v, s, l *ProjectMetadataTable) { v.Columns[2].Name = v.Columns[3].Name },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].RowID = "01" },
		func(v, s, l *ProjectMetadataTable) { v.Rows[1].RowID = v.Rows[0].RowID },
		func(v, s, l *ProjectMetadataTable) { s.Rows[0].Cells[0] = metadataInteger("1") },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[1] = metadataInteger("1") },
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataText("1") },
		func(v, s, l *ProjectMetadataTable) {
			blob := "00"
			v.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "blob", BlobHex: &blob}
		},
		func(v, s, l *ProjectMetadataTable) { v.Rows[0].Cells[2] = metadataInteger("9223372036854775808") },
		func(v, s, l *ProjectMetadataTable) {
			value := math.Inf(1)
			v.Rows[0].Cells[2] = ProjectMetadataCell{Storage: "real", Real: &value}
		},
		func(v, s, l *ProjectMetadataTable) { l.Rows[0].Cells[0] = metadataText(" 1") },
		func(v, s, l *ProjectMetadataTable) { l.Rows[0].Cells[0] = metadataText("A") },
		func(v, s, l *ProjectMetadataTable) { l.Rows[0].Cells = nil },
	} {
		veg, su, layers := vegetationReportFixture(t)
		change(&veg, &su, &layers)
		result, err := prepareLongVegetation(context.Background(), "Project", "Selected", veg, su, layers)
		if err == nil || !reflect.DeepEqual(result, VegetationReportPreparation{}) {
			t.Fatal("invalid snapshot produced success-shaped partial output", result, err)
		}
	}
	veg, su, layers := vegetationReportFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{ctx, &vegetationCancelContext{context.Background(), 25}} {
		result, err := prepareLongVegetation(ctx, "Project", "Selected", veg, su, layers)
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, VegetationReportPreparation{}) {
			t.Fatal("cancellation returned partial success", result, err)
		}
	}
	for _, identity := range []string{"", "None", "\x00", "\xff"} {
		if _, err := prepareLongVegetation(context.Background(), "Project", identity, veg, su, layers); err == nil {
			t.Fatal("invalid selected SU accepted", identity)
		}
	}
}
