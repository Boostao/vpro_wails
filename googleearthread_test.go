package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func TestOwnedGoogleEarthLocationsMatchDirectEnvSQLWithoutAdmin(t *testing.T) {
	service, state := contextServiceFixture(t)
	mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Admin`)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.readGoogleEarthLocations(context.Background(), state.ContextID, "Zone")
	if err != nil || got == nil || got.ContextID != state.ContextID ||
		got.ProjectPath != state.ProjectPath || got.SUPath != "" || got.Report.SU != "None" {
		t.Fatal("owned direct Env scope unavailable", got, err)
	}
	db, err := openReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT rowid,PlotNumber,Longitude,Longitude*-1,Latitude,Zone
FROM Sample_Env WHERE Latitude IS NOT NULL AND Longitude IS NOT NULL ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var id int64
		values := make([]any, 5)
		if err := rows.Scan(&id, &values[0], &values[1], &values[2], &values[3], &values[4]); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if count >= len(got.Report.Rows) {
			t.Fatal("source direct Env row omitted")
		}
		row := got.Report.Rows[count]
		if row.EnvRowID != strconv.FormatInt(id, 10) || row.MembershipRowID != "" {
			t.Fatal("physical source provenance changed", row)
		}
		cells := []ProjectMetadataCell{row.PlotNumber, row.StoredLongitude, row.Longitude, row.Latitude, row.Description}
		for i, cell := range cells {
			value, err := metadataCellValue(cell)
			if err != nil || !reflect.DeepEqual(value, values[i]) {
				t.Fatal("independent source projection differs", count, i, value, values[i], err)
			}
		}
		count++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	if count == 0 || count != len(got.Report.Rows) {
		t.Fatal("direct source rows not actually exercised", count, len(got.Report.Rows))
	}
	report, err := service.readPlotLocations(context.Background(), state.ContextID)
	if err != nil || len(report.Report.Rows) != 0 {
		t.Fatal("existing ReportLocation Admin requirement changed", report, err)
	}
	*got.Report.Rows[0].PlotNumber.Text = "caller mutation"
	again, err := service.readGoogleEarthLocations(context.Background(), state.ContextID, "Zone")
	if err != nil || *again.Report.Rows[0].PlotNumber.Text == "caller mutation" {
		t.Fatal("caller aliases future snapshots", again, err)
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, after) || service.projects.contextID != state.ContextID {
		t.Fatal("read changed preferences/context", err)
	}
}

func TestOwnedGoogleEarthLocationsExternalSUFanoutAndRawDescription(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, err := openReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var plot string
	err = db.QueryRow(`SELECT PlotNumber FROM Sample_Env WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL ORDER BY rowid LIMIT 1`).Scan(&plot)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Earth O'Brien #.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`CREATE TABLE Earth_SU(PlotNumber VARCHAR,SiteUnit VARCHAR);
INSERT INTO Earth_SU(PlotNumber,SiteUnit) VALUES(?,NULL),(?,''),(?,'different'),(NULL,'ignored')`, plot, plot, plot)
	if err := errors.Join(err, writer.Close()); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Earth", SUPath: path, Hierarchy: "None"})
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	got, err := service.readGoogleEarthLocations(context.Background(), next.ContextID, "PlotNumber")
	if err != nil || got == nil || got.SUPath != next.SUPath || len(got.Report.Rows) != 3 {
		t.Fatal("owned selected-SU multiplicity lost", got, err)
	}
	for i, row := range got.Report.Rows {
		if row.MembershipRowID != strconv.Itoa(i+1) || *row.PlotNumber.Text != plot || *row.Description.Text != plot {
			t.Fatal("source duplicate membership or selected raw field changed", row)
		}
	}
	*got.Report.Rows[0].Description.Text = "caller"
	if *got.Report.Rows[1].Description.Text != plot || *got.Report.Rows[0].PlotNumber.Text != plot {
		t.Fatal("emitted pairs/name/description share mutable pointers")
	}
	assertProfileSUFiles(t, service, before)
}

func TestOwnedGoogleEarthLocationsOwnershipCancellationAndPhysicalScope(t *testing.T) {
	service, state := contextServiceFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, request := range [][2]string{{"stale", "Zone"}, {state.ContextID, "zone"}, {state.ContextID, "Zone;DROP TABLE Sample_Env"}} {
		if got, err := service.readGoogleEarthLocations(context.Background(), request[0], request[1]); got != nil || err == nil {
			t.Fatal("stale/inferred description scope accepted", got, err)
		}
	}
	owner := service.projects.sqlite
	prior := owner.attachmentInfo["project"]
	owner.attachmentInfo["project"] = owner.attachmentInfo["VPro64"]
	got, err := service.readGoogleEarthLocations(context.Background(), state.ContextID, "Zone")
	owner.attachmentInfo["project"] = prior
	if got != nil || err == nil {
		t.Fatal("unowned source accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.readGoogleEarthLocations(ctx, state.ContextID, "Zone"); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("initial cancellation lost", got, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	queued := &tableCSVLeaseContext{Context: ctx, queued: make(chan struct{})}
	owner.mu.Lock()
	locked := true
	defer func() {
		if locked {
			owner.mu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		value, err := service.readGoogleEarthLocations(queued, state.ContextID, "Zone")
		if value != nil {
			err = errors.Join(err, errors.New("cancelled owned projection returned a value"))
		}
		done <- err
	}()
	select {
	case <-queued.queued:
	case <-time.After(5 * time.Second):
		t.Fatal("owned read did not reach held snapshot mutex")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owned cancellation retained a lease")
	}
	owner.mu.Unlock()
	locked = false
	if got, err := service.readGoogleEarthLocations(context.Background(), state.ContextID, "Zone"); got == nil || err != nil {
		t.Fatal("cancelled request discarded pinned context", got, err)
	}
	assertProfileSUFiles(t, service, before)
	impostor, selected := contextServiceFixture(t)
	mutateContextFixture(t, selected.ProjectPath, `ALTER TABLE Sample_Env RENAME TO Hidden_Env; CREATE VIEW Sample_Env AS SELECT * FROM Hidden_Env`)
	if got, err := impostor.readGoogleEarthLocations(context.Background(), selected.ContextID, "Zone"); got != nil || err == nil {
		t.Fatal("physical Env view impostor accepted", got, err)
	}
}
