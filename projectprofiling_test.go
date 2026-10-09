package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func profileRunFixture(t *testing.T) (*ContextService, ProjectState, ProjectPlotProfileRunRequest) {
	t.Helper()
	service, state := contextServiceFixture(t)
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	lump, err := service.ReviewProjectPlotProfileLump(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, ProjectPlotProfileRunRequest{OriginalRules: review.Rules, ProjectLump: &lump}
}

func TestProjectProfilingOriginalRulesRunWithoutStoredWrites(t *testing.T) {
	service, state, request := profileRunFixture(t)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, combine := range []bool{false, true} {
		request.Subvarieties = combine
		result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request)
		if err != nil {
			t.Fatal(err)
		}
		if result.Project != "Sample" || result.Table != "Sample_Profile" || result.SU != "None" || len(result.Steps) != 8 {
			t.Fatal("original ordered source run unavailable", result)
		}
		if result.TotalPlots != 52 || !reflect.DeepEqual(result.PlotNumbers, []string{
			"108050", "108050x", "8229723", "8428935", "8428942", "8428963", "8428970", "8428975", "8529670", "9003043", "9003104",
		}) {
			t.Fatal("original profile measurable output changed", combine, result)
		}
		counts := []int{}
		for _, step := range result.Steps {
			counts = append(counts, step.PlotCount)
		}
		if !reflect.DeepEqual(counts, []int{13, 0, 0, 0, 0, 0, 35, 24}) {
			t.Fatal("source step count output changed", counts)
		}
	}

	for role, original := range files {
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("profile preview changed stored rules/counts/scratch/audits/support", role, err)
		}
	}
}

func profileSQLFixture(t *testing.T) *sql.Conn {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := registerProfileFunctions(conn); err != nil {
		t.Fatal(err)
	}
	columns := []string{}
	for _, cover := range profileCovers {
		columns = append(columns, quoteHeaderIdentifier(cover)+" REAL")
	}
	_, err = conn.ExecContext(context.Background(), `CREATE TABLE USysEnv(PlotNumber TEXT PRIMARY KEY,Code TEXT,Flag BOOLEAN);
			INSERT INTO USysEnv VALUES('P','C',1),('Z','',0),('M',NULL,-1),('N','Other',NULL);
			CREATE TABLE USysVeg(PlotNumber TEXT,Species TEXT,`+strings.Join(columns, ",")+`);
			INSERT INTO USysVeg VALUES('P','TARGET',2,2,2,2,2,2,2,2,2,2),('Z','TARGET',0,0,0,0,0,0,0,0,0,0),
			('M','TARGET',NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL),
			('N','OTHER',4,4,4,4,4,4,4,4,4,4);
			CREATE TABLE Lumps(LumpCode TEXT,SppCode TEXT,"Use" BOOLEAN);
			INSERT INTO Lumps VALUES('LT','TARGET',0),('OTHERL','OTHER',-1);`)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestSourceProfileEquivalentLayerOperationMatrix(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	for _, table := range []string{"Veg", "Lump"} {
		for _, layer := range []string{"Any", "SumAll", "SumA", "SumB", "1", "2", "3", "4", "5", "5a", "5b", "5c", "6", "7"} {
			for _, operation := range []string{"Add plots", "Common plots", "Subtract plots"} {
				for _, operator := range []string{">", "<"} {
					tx, err := conn.BeginTx(ctx, nil)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE VProProfileRunPlots(PlotNumber TEXT PRIMARY KEY)`); err != nil {
						t.Fatal(err)
					}
					if operation != "Add plots" {
						if _, err := tx.ExecContext(ctx, `INSERT INTO VProProfileRunPlots SELECT PlotNumber FROM USysEnv`); err != nil {
							t.Fatal(err)
						}
					}
					species := "TARGET"
					if table == "Lump" {
						species = "LT"
					}
					rule := plotProfileRule{table: table, layer: layer, operation: operation, operator: operator, species: species, criterion: float64(1)}
					query, args := profileMatchQuery(rule, nil, "Lumps")
					matches, err := queryProfilePlots(ctx, tx, query, args)
					if err != nil {
						t.Fatal(table, layer, operation, operator, err)
					}
					if err := applyProfileStep(ctx, tx, rule, matches); err != nil {
						t.Fatal(err)
					}
					got, err := queryProfilePlots(ctx, tx, `SELECT PlotNumber FROM VProProfileRunPlots ORDER BY PlotNumber`, nil)
					if err != nil {
						t.Fatal(err)
					}
					expected := []string{"P"}
					aggregate := slices.Contains([]string{"Any", "SumAll", "SumA", "SumB"}, layer)
					if operator == "<" {
						expected = []string{"M", "N", "Z"}
					}
					if operation == "Common plots" {
						if operator == "<" {
							expected = []string{"Z"}
							if aggregate {
								expected = []string{"M", "Z"}
							}
						}
					}
					if operation == "Subtract plots" {
						expected = []string{"M", "Z"}
						if table == "Lump" {
							expected = []string{"M", "N", "Z"}
						}
						if operator == "<" {
							expected = []string{"M", "P"}
							if aggregate {
								expected = []string{"P"}
							}
						}
					}
					if !reflect.DeepEqual(got, expected) {
						t.Fatal("source layer/operation threshold mismatch", table, layer, operation, operator, got, expected)
					}
					if err := tx.Rollback(); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}

func TestSourceProfileSQLNullBooleanLiteralAndAbsentSumB(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	env := []ProjectMetadataColumn{{Name: "Code", DeclaredType: "TEXT"}, {Name: "Flag", DeclaredType: "BOOLEAN"}}
	for _, sample := range []struct {
		rule     plotProfileRule
		expected []string
	}{
		{plotProfileRule{table: "Env", field: "Code", operator: "=", criterion: "c"}, []string{"P"}},
		{plotProfileRule{table: "Env", field: "Code", operator: "Not Like", criterion: "*c*"}, []string{"N", "Z"}},
		{plotProfileRule{table: "Env", field: "Code", operator: ">", envNull: true}, []string{"M"}},
		{plotProfileRule{table: "Env", field: "Flag", operator: "=", criterion: float64(-1)}, []string{"M", "P"}},
		{plotProfileRule{table: "Env", field: "Code", operator: "=", criterion: "c' OR 1=1 --"}, []string{}},
	} {
		query, args := profileMatchQuery(sample.rule, env, "")
		got, err := queryProfilePlots(ctx, conn, query, args)
		slices.Sort(got)
		if err != nil || !reflect.DeepEqual(got, sample.expected) {
			t.Fatal("NULL/BOOLEAN/literal parameter semantics changed", sample, got, err)
		}
	}
	if _, err := conn.ExecContext(ctx, `UPDATE USysVeg SET Cover5a=4 WHERE PlotNumber='Z';
			CREATE TEMP TABLE VProProfileRunPlots(PlotNumber TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	query, args := profileMatchQuery(plotProfileRule{table: "Veg", layer: "SumB", species: "TARGET", operation: "Add plots", operator: "<", criterion: float64(1)}, nil, "")
	got, err := queryProfilePlots(ctx, conn, query, args)
	slices.Sort(got)
	if err != nil || !reflect.DeepEqual(got, []string{"M", "N", "Z"}) {
		t.Fatal("SumB absence must ignore5a/5b/5c like the exported branch", got, err)
	}
	for _, expression := range []string{"vpro_profile_number(x'')", "vpro_profile_text(x'')", "vpro_profile_bool(2)"} {
		var value any
		if err := conn.QueryRowContext(ctx, "SELECT "+expression).Scan(&value); err == nil {
			t.Fatal("empty BLOB/invalid BOOLEAN silently treated as SQL NULL", expression, value)
		}
	}
}
func TestProjectProfilingRejectsStaleCancelledChangedRulesAndLumps(t *testing.T) {
	service, state, request := profileRunFixture(t)
	if _, err := service.RunProjectPlotProfile(context.Background(), "stale", request); err == nil {
		t.Fatal("stale scope accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.RunProjectPlotProfile(ctx, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request succeeded", err)
	}
	without := request
	without.ProjectLump = nil
	if _, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, without); err == nil {
		t.Fatal("implicit lump selection accepted")
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`UPDATE Sample_Profile SET PlotCount=777 WHERE rowid=3`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("changed counts/original rules accepted")
	}
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request.OriginalRules = review.Rules
	if _, err := db.Exec(`UPDATE Sample_Lump SET LumpCode='Changed' WHERE rowid=(SELECT MIN(rowid) FROM Sample_Lump)`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("stale lump definitions accepted")
	}
}

func TestSourceProfileSingleMaxAndLayerDifferences(t *testing.T) {
	for _, sample := range []struct {
		values   []any
		expected float64
	}{
		{[]any{nil, nil}, 0}, {[]any{-1.0, -2.0}, 0}, {[]any{1.125, 2.5, nil}, 2.5},
		{[]any{16777217.0, 16777216.5}, float64(float32(16777216.5))},
	} {
		value, err := profileMax(sample.values...)
		if err != nil || value != sample.expected {
			t.Fatal("source Single comparison/NULL/zero semantics changed", sample, value, err)
		}

	}
	for _, raw := range []any{"5", []byte{1}, math.Inf(1)} {
		if _, err := profileMax(raw); err == nil {
			t.Fatal("numeric storage repaired", raw)
		}
	}
	if _, err := profileMax(math.MaxFloat64); err == nil {
		t.Fatal("Single overflow silently saturated")
	}
	if !reflect.DeepEqual(profileLayerColumns("SumB", false), []string{"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c"}) ||
		!reflect.DeepEqual(profileLayerColumns("SumB", true), []string{"Cover4", "Cover5"}) {
		t.Fatal("source SumB absence asymmetry was normalized")
	}
}

func TestSourceProfileCompilerRejectsUnobservedAmbiguousAndUnverifiedRules(t *testing.T) {
	makeRule := func(fields map[string]string) ProjectMetadataTable {
		names := []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"}
		values := map[string]string{"Table": "Veg", "Field": "Species", "Operator": ">", "Layer": "Any",
			"Species": "TARGET", "Criteria": "1", "Operation": "Add plots"}
		for key, value := range fields {
			values[key] = value
		}
		table := ProjectMetadataTable{Rows: []ProjectMetadataRow{{RowID: "1"}}}
		for _, name := range names {
			table.Columns = append(table.Columns, ProjectMetadataColumn{Name: name, DeclaredType: "TEXT"})
			cell := ProjectMetadataCell{Storage: "null"}
			if name == "Order" {
				value := "1"
				cell.Storage, cell.Integer = "integer", &value
			} else if value, present := values[name]; present {
				cell.Storage, cell.Text = "text", &value
			}
			table.Rows[0].Cells = append(table.Rows[0].Cells, cell)
		}
		return table
	}
	env := []ProjectMetadataColumn{{Name: "Code", DeclaredType: "VARCHAR"}, {Name: "Date", DeclaredType: "DATETIME"}}
	for _, fields := range []map[string]string{
		{"Table": "Veg "}, {"Table": "Other"}, {"Operator": ">="}, {"Operator": "= 1 OR 1=1"},
		{"Field": "Species; DROP TABLE USysVeg"}, {"Layer": "SumD"}, {"Species": "T\u00e9"},
		{"Criteria": "NaN"}, {"Criteria": "1e999"}, {"Criteria": "1; DELETE FROM Sample_Env"},
		{"Operation": "Add plots "}, {"Table": "Lump", "Field": "LumpCode", "Operator": "="},
		{"Table": "Env", "Field": "Unobserved"}, {"Table": "Env", "Field": "Date"},
		{"Table": "Env", "Field": "Code", "Operator": "Like", "Criteria": "[ABC]*"},
		{"Table": "Env", "Field": "Code", "Operator": "Like", "Criteria": "#"},
		{"Table": "Env", "Field": "Code", "Criteria": "1,25"},
		{"Table": "Env", "Field": "Code", "Criteria": "-1,25"},
		{"Table": "Env", "Field": "Code", "Criteria": " 1.25 "},
		{"Table": "Env", "Field": "Code", "Criteria": "\u00e9"},
		{"Table": "Env", "Field": "Code", "Criteria": "A\x00B"},
	} {
		if _, err := compilePlotProfileRules(makeRule(fields), env); err == nil {
			t.Fatal("unsupported rule guessed/repaired", fields)
		}
	}
	for _, change := range []func(*ProjectMetadataTable){
		func(table *ProjectMetadataTable) { table.Rows = nil },
		func(table *ProjectMetadataTable) { table.Columns[1].Name = "Order" },
		func(table *ProjectMetadataTable) { table.Columns[1].Name = "MissingTable" },
		func(table *ProjectMetadataTable) { table.Rows[0].RowID = "01" },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells = table.Rows[0].Cells[:8] },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"} },
		func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Integer = "32768" },
		func(table *ProjectMetadataTable) { *table.Rows[0].Cells[0].Integer = "1.0" },
		func(table *ProjectMetadataTable) { table.Rows = append(table.Rows, table.Rows[0]) },
		func(table *ProjectMetadataTable) {
			duplicate := table.Rows[0]
			duplicate.RowID = "2"
			table.Rows = append(table.Rows, duplicate)
		},
	} {
		table := makeRule(nil)
		change(&table)
		if _, err := compilePlotProfileRules(table, env); err == nil {
			t.Fatal("malformed schema/order/physical identity accepted", table)
		}
	}
	for _, fields := range []map[string]string{
		nil, {"Table": "veg", "Layer": "sumB", "Operation": "common plots"},
		{"Criteria": "NULL"}, {"Table": "Env", "Field": "Date", "Criteria": "Null"},
		{"Table": "Env", "Field": "Code", "Criteria": "O'Brien"},
		{"Table": "Env", "Field": "Code", "Operator": "Like", "Criteria": "A*?"},
	} {
		if rules, err := compilePlotProfileRules(makeRule(fields), env); err != nil || len(rules) != 1 {
			t.Fatal("supported literal source rule rejected", fields, rules, err)
		}
	}
}

func TestProjectProfilingMidRunFailureIsIsolatedAndRetryRetainsInputs(t *testing.T) {
	service, state, request := profileRunFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`CREATE TEMP TABLE ProfileTestCover AS SELECT rowid AS id,Cover1 FROM Sample_Veg`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1='unsupported historical cover'`); err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request)
	if err == nil || !strings.Contains(err.Error(), "profile row") || len(result.Steps) != 0 || len(result.PlotNumbers) != 0 {
		t.Fatal("failed middle-step computation returned success-shaped data", result, err)
	}
	for role, original := range files {
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("failed preview changed data/support", role, err)
		}
	}
	var leaked int
	if err := service.projects.sqlite.conn.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_temp_master WHERE name LIKE 'VProProfile%'`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatal("isolated job leaked TEMP state into resident context", leaked, err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Cover1=(SELECT Cover1 FROM ProfileTestCover b WHERE b.id=Sample_Veg.rowid)`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request); err != nil || len(result.PlotNumbers) != 11 {
		t.Fatal("retained original inputs cannot retry after isolated rollback", result, err)
	}
}
