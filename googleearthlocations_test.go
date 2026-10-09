package main

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestGoogleEarthLocationsDirectEnvAndSourceJoinFanout(t *testing.T) {
	env, admin, su := locationTestTables()
	project, err := planGoogleEarthLocations(context.Background(), "Project", "None", "Zone", env, ProjectMetadataTable{})
	if err != nil || len(project.Rows) != 3 {
		t.Fatal("direct Env scope must include plot without Admin", project, err)
	}
	if project.Rows[2].EnvRowID != "5" || *project.Rows[2].PlotNumber.Text != "NOADMIN" ||
		project.Rows[0].MembershipRowID != "" || *project.Rows[0].Description.Text != "  Zone  " ||
		project.Rows[1].Description.Storage != "null" || *project.Rows[1].Longitude.Real != 123.75 ||
		!math.Signbit(*project.Rows[1].Latitude.Real) {
		t.Fatal("source scope, raw descriptions, numeric sign or signed zero changed", project)
	}
	report, err := planPlotLocations(context.Background(), "Project", "None", env, admin, su)
	if err != nil || len(report.Rows) != 2 {
		t.Fatal("ReportLocation Env/Admin rule changed", report, err)
	}
	env.Rows = append(env.Rows, ProjectMetadataRow{RowID: "9007199254740993", Cells: append([]ProjectMetadataCell(nil), env.Rows[0].Cells...)})
	selected, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su)
	if err != nil || len(selected.Rows) != 6 {
		t.Fatal("source INNER JOIN must retain both physical Env rows and all three memberships", selected, err)
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE Env(PlotNumber TEXT,Longitude REAL,Latitude REAL);
CREATE TABLE SU(PlotNumber TEXT);
INSERT INTO Env(rowid,PlotNumber,Longitude,Latitude) VALUES(-9007199254740993,'000001',123.75,49.25),(9007199254740993,'000001',123.75,49.25),(5,'NOADMIN',130,50);
INSERT INTO SU(rowid,PlotNumber) VALUES(1,'000001'),(2,'000001'),(3,'000001'),(4,NULL)`); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT CAST(Env.rowid AS TEXT),CAST(SU.rowid AS TEXT)
FROM Env INNER JOIN SU ON Env.PlotNumber=SU.PlotNumber WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]string]bool{}
	for rows.Next() {
		var envID, suID string
		if err := rows.Scan(&envID, &suID); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		want[[2]string{envID, suID}] = true
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]string]bool{}
	for _, row := range selected.Rows {
		got[[2]string{row.EnvRowID, row.MembershipRowID}] = true
	}
	if !reflect.DeepEqual(got, want) || len(want) != 6 {
		t.Fatal("independent source SQL fanout differs", got, want)
	}
	*selected.Rows[0].Description.Text = "output mutation"
	if *selected.Rows[1].Description.Text != "  Zone  " || *env.Rows[0].Cells[1].Text != "  Zone  " {
		t.Fatal("duplicated placemarks share description pointers")
	}
}

type googleEarthCheckpointContext struct {
	context.Context
	calls    int
	cancelAt int
	mutate   func()
}

func (c *googleEarthCheckpointContext) Err() error {
	c.calls++
	if c.calls == 1 && c.mutate != nil {
		c.mutate()
	}
	if c.cancelAt > 0 && c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestGoogleEarthLocationsDetachedInputsAndCancellation(t *testing.T) {
	env, _, su := locationTestTables()
	ctx := &googleEarthCheckpointContext{Context: context.Background(), mutate: func() {
		*env.Rows[0].Cells[1].Text = "caller mutation"
		su.Rows[0].RowID = "changed"
	}}
	got, err := planGoogleEarthLocations(ctx, "Project", "Subset", "Zone", env, su)
	if err != nil || len(got.Rows) != 3 || got.Rows[0].MembershipRowID != "1" ||
		*got.Rows[0].Description.Text != "  Zone  " {
		t.Fatal("caller aliases changed before a fallible context checkpoint", got, err)
	}
	for checkpoint := 1; checkpoint <= 10; checkpoint++ {
		env, _, su = locationTestTables()
		ctx = &googleEarthCheckpointContext{Context: context.Background(), cancelAt: checkpoint}
		result, err := planGoogleEarthLocations(ctx, "Project", "Subset", "Zone", env, su)
		if ctx.calls >= checkpoint {
			if result != nil || !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation returned partial success", checkpoint, result, err)
			}
		} else if err != nil || result == nil {
			t.Fatal("uncancelled preparation failed", checkpoint, result, err)
		}
	}
}

func TestGoogleEarthLocationsRawDescriptionsAndRejectedInputs(t *testing.T) {
	for _, cell := range []ProjectMetadataCell{metadataText(""), metadataText("]]><&\x00"),
		metadataInteger("9007199254740993"), {Storage: "null"}, {Storage: "blob", BlobHex: qualityString("00ff")}} {
		env, _, su := locationTestTables()
		env.Rows[0].Cells[1] = cell
		got, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su)
		if err != nil || !reflect.DeepEqual(got.Rows[0].Description, cell) {
			t.Fatal("raw descriptions must not receive XML/CDATA/string coercion", cell, got, err)
		}
		for _, plot := range []ProjectMetadataCell{metadataInteger("9007199254740993"), {Storage: "null"}} {
			env, _, su := locationTestTables()
			env.Rows[0].Cells[0] = plot
			got, err := planGoogleEarthLocations(context.Background(), "Project", "None", "Zone", env, su)
			if err != nil || len(got.Rows) != 3 || !reflect.DeepEqual(got.Rows[0].PlotNumber, plot) {
				t.Fatal("direct Env preparation must preserve raw included plot names", got, err)
			}
			selected, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su)
			if err != nil || len(selected.Rows) != 0 {
				t.Fatal("exact-text SU membership must not coerce nontext plot names", selected, err)
			}
		}
	}
	for _, change := range []func(*ProjectMetadataTable, *ProjectMetadataTable){
		func(env, su *ProjectMetadataTable) { env.Columns[1].Name = "zone" },
		func(env, su *ProjectMetadataTable) { env.Rows[0].RowID = "01" },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells = env.Rows[0].Cells[:2] },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "text"} },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells[6] = metadataInteger("-9223372036854775808") },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells[5] = metadataText("49") },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells[6] = siviReal(math.NaN()) },
		func(env, su *ProjectMetadataTable) { env.Rows[0].Cells[1] = metadataText(string([]byte{0xff})) },
		func(env, su *ProjectMetadataTable) { su.Rows[1].RowID = su.Rows[0].RowID },
	} {
		env, _, su := locationTestTables()
		change(&env, &su)
		if got, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su); got != nil || err == nil {
			t.Fatal("malformed preparation returned success", got, err)
		}
	}
	env, _, su := locationTestTables()
	for _, field := range []string{"", "Zone;DROP TABLE Env", "Zone\x00", "zone"} {
		if got, err := planGoogleEarthLocations(context.Background(), "Project", "None", field, env, su); got != nil || err == nil {
			t.Fatal("description selector inferred or repaired", field, got, err)
		}
		for _, identities := range [][2]string{{"", "None"}, {"Project", ""}, {"Bad Name", "None"}, {"Project", "Bad Name"}} {
			if got, err := planGoogleEarthLocations(context.Background(), identities[0], identities[1], "Zone", env, su); got != nil || err == nil {
				t.Fatal("project/SU identity inferred or repaired", identities, got, err)
			}
		}
	}
	env.Rows[0].Cells[5] = siviReal(100)
	env.Rows[0].Cells[6] = siviReal(200)
	got, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su)
	if err != nil || *got.Rows[0].Longitude.Real != -200 || *got.Rows[0].Latitude.Real != 100 {
		t.Fatal("historical coordinate ranges must not be clamped during raw preparation", got, err)
	}
}
