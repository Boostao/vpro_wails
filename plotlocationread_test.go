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

func TestOwnedPlotLocationsMatchIndependentSourceSQLAndPreserveEveryFile(t *testing.T) {
	service, state := contextServiceFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.readPlotLocations(context.Background(), state.ContextID)
	if err != nil || review == nil || review.ContextID != state.ContextID || review.ProjectPath != state.ProjectPath ||
		review.SUPath != "" || review.Report.SU != "None" {
		t.Fatal("owned project-only location scope", review, err)
	}
	db, err := openReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT e.rowid,a.rowid,e.PlotNumber,e.Zone,e.SubZone,e.SiteSeries,e.LocationAccuracy,
		e.Latitude,e.Longitude*-1,e.Elevation FROM Sample_Env e INNER JOIN Sample_Admin a
		ON e.PlotNumber=a.Plot WHERE e.Latitude IS NOT NULL AND e.Longitude IS NOT NULL ORDER BY e.rowid`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		values := make([]any, 10)
		pointers := make([]any, 10)
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if count >= len(review.Report.Rows) {
			t.Fatal("independent SQL rows omitted")
		}
		got := review.Report.Rows[count]
		if got.EnvRowID != strconv.FormatInt(values[0].(int64), 10) ||
			got.AdminRowID != strconv.FormatInt(values[1].(int64), 10) {
			t.Fatal("physical parent provenance differs", got)
		}
		for i, cell := range got.Values {
			value, err := metadataCellValue(cell)
			if err != nil || !reflect.DeepEqual(value, values[i+2]) {
				t.Fatal("source8-field SQL projection differs", count, i, value, values[i+2], err)
			}
		}
		count++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	if count == 0 || count != len(review.Report.Rows) {
		t.Fatal("source sample/cardinality not actually exercised", count, len(review.Report.Rows))
	}
	*review.Report.Rows[0].Values[0].Text = "caller"
	again, err := service.readPlotLocations(context.Background(), state.ContextID)
	if err != nil || *again.Report.Rows[0].Values[0].Text == "caller" {
		t.Fatal("caller aliases future owned snapshots", err)
	}
	assertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, after) || service.projects.contextID != state.ContextID {
		t.Fatal("location preparation changed YAML/context")
	}
}

func TestOwnedPlotLocationsExternalSelectedSUIncludesNullableClassificationsOnce(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, err := openReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var plot string
	err = db.QueryRow(`SELECT e.PlotNumber FROM Sample_Env e INNER JOIN Sample_Admin a ON e.PlotNumber=a.Plot
		WHERE e.Latitude IS NOT NULL AND e.Longitude IS NOT NULL ORDER BY e.rowid LIMIT 1`).Scan(&plot)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "Location O'Brien #.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`CREATE TABLE Location_SU(PlotNumber VARCHAR,SiteUnit VARCHAR)`)
	if err == nil {
		_, err = writer.Exec(`INSERT INTO Location_SU(PlotNumber,SiteUnit) VALUES(?,NULL),(?,''),(?,'different'),(NULL,'null plot')`, plot, plot, plot)
	}
	if err := errors.Join(err, writer.Close()); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Location", SUPath: path, Hierarchy: "None", HierarchyPath: ""})
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	got, err := service.readPlotLocations(context.Background(), next.ContextID)
	if err != nil || got == nil || got.SUPath != next.SUPath || got.Report.SU != "Location" ||
		len(got.Report.Rows) != 1 || *got.Report.Rows[0].Values[0].Text != plot ||
		!reflect.DeepEqual(got.Report.Rows[0].MembershipRowIDs, []string{"1", "2", "3"}) {
		t.Fatal("selected-SU location scope imposed grouping/classification or membership multiplicity", got, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestOwnedPlotLocationsStalePhysicalImpostorOwnershipCancellationAndRetry(t *testing.T) {
	service, state := contextServiceFixture(t)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if got, err := service.readPlotLocations(context.Background(), "stale"); got != nil || err == nil {
		t.Fatal("stale scope returned locations", got, err)
	}
	owner := service.projects.sqlite
	prior := owner.attachmentInfo["project"]
	owner.attachmentInfo["project"] = owner.attachmentInfo["VPro64"]
	got, err := service.readPlotLocations(context.Background(), state.ContextID)
	owner.attachmentInfo["project"] = prior
	if got != nil || err == nil {
		t.Fatal("wrong project file ownership returned locations", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.readPlotLocations(ctx, state.ContextID); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("pre-request cancellation lost", got, err)
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
		value, err := service.readPlotLocations(queued, state.ContextID)
		if value != nil {
			done <- errors.New("cancelled owned locations returned partial value")
			return
		}
		done <- err
	}()
	select {
	case <-queued.queued:
	case <-time.After(5 * time.Second):
		t.Fatal("location request did not reach held snapshot mutex")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued location cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("location cancellation did not release leases")
	}
	owner.mu.Unlock()
	locked = false
	if value, err := service.readPlotLocations(context.Background(), state.ContextID); value == nil || err != nil {
		t.Fatal("location cancellation discarded pinned context", err)
	}
	assertProfileSUFiles(t, service, before)
	service2, state2 := contextServiceFixture(t)
	mutateContextFixture(t, state2.ProjectPath, `DROP TABLE Sample_Admin; CREATE VIEW Sample_Admin AS SELECT PlotNumber AS Plot FROM Sample_Env`)
	if value, err := service2.readPlotLocations(context.Background(), state2.ContextID); value != nil || err == nil {
		t.Fatal("Admin physical view impostor accepted", value, err)
	}
}
