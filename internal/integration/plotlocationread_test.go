package integration

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	service, state := vprotest.ContextServiceFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReadPlotLocations(context.Background(), state.ContextID)
	if err != nil || review == nil || review.ContextID != state.ContextID || review.ProjectPath != state.ProjectPath ||
		review.SUPath != "" || review.Report.SU != "None" {
		t.Fatal("owned project-only location scope", review, err)
	}
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
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
			value, err := vprocore.MetadataCellValue(cell)
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
	again, err := service.ReadPlotLocations(context.Background(), state.ContextID)
	if err != nil || *again.Report.Rows[0].Values[0].Text == "caller" {
		t.Fatal("caller aliases future owned snapshots", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(config, after) || service.Projects.ContextID != state.ContextID {
		t.Fatal("location preparation changed YAML/context")
	}
}

func TestOwnedPlotLocationsExternalSelectedSUIncludesNullableClassificationsOnce(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
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
	writer, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
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
	next, err := service.SwitchContext(state.ContextID, vprocore.ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Location", SUPath: path, Hierarchy: "None", HierarchyPath: ""})
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	got, err := service.ReadPlotLocations(context.Background(), next.ContextID)
	if err != nil || got == nil || got.SUPath != next.SUPath || got.Report.SU != "Location" ||
		len(got.Report.Rows) != 1 || *got.Report.Rows[0].Values[0].Text != plot ||
		!reflect.DeepEqual(got.Report.Rows[0].MembershipRowIDs, []string{"1", "2", "3"}) {
		t.Fatal("selected-SU location scope imposed grouping/classification or membership multiplicity", got, err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestOwnedPlotLocationsStalePhysicalImpostorOwnershipCancellationAndRetry(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := service.ReadPlotLocations(context.Background(), "stale"); got != nil || err == nil {
		t.Fatal("stale scope returned locations", got, err)
	}
	owner := service.Projects.Sqlite
	prior := owner.AttachmentInfo["project"]
	owner.AttachmentInfo["project"] = owner.AttachmentInfo["VPro64"]
	got, err := service.ReadPlotLocations(context.Background(), state.ContextID)
	owner.AttachmentInfo["project"] = prior
	if got != nil || err == nil {
		t.Fatal("wrong project file ownership returned locations", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := service.ReadPlotLocations(ctx, state.ContextID); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("pre-request cancellation lost", got, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	queued := &vprotest.TableCSVLeaseContext{Context: ctx, Queued: make(chan struct{})}
	owner.Mu.Lock()
	locked := true
	defer func() {
		if locked {
			owner.Mu.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() {
		value, err := service.ReadPlotLocations(queued, state.ContextID)
		if value != nil {
			done <- errors.New("cancelled owned locations returned partial value")
			return
		}
		done <- err
	}()
	select {
	case <-queued.Queued:
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
	owner.Mu.Unlock()
	locked = false
	if value, err := service.ReadPlotLocations(context.Background(), state.ContextID); value == nil || err != nil {
		t.Fatal("location cancellation discarded pinned context", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	service2, state2 := vprotest.ContextServiceFixture(t)
	vprotest.MutateContextFixture(t, state2.ProjectPath, `DROP TABLE Sample_Admin; CREATE VIEW Sample_Admin AS SELECT PlotNumber AS Plot FROM Sample_Env`)
	if value, err := service2.ReadPlotLocations(context.Background(), state2.ContextID); value != nil || err == nil {
		t.Fatal("Admin physical view impostor accepted", value, err)
	}
}
