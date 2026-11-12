package spatial

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestOwnedGoogleEarthKMLMatchesDirectSQLAndPreservesFiles(t *testing.T) {
	service, state := vprotest.PrepareKMLContextFixture(t)
	vprotest.MutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Admin`)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	title := " Title <&\U0001f332\r\n "
	got, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", title)
	if err != nil || got == nil || got.ContextID != state.ContextID || got.ProjectPath != state.ProjectPath ||
		got.SUPath != "" || got.Project != state.ActiveProject || got.SU != "None" || got.DescriptionField != "Zone" || got.Title != title {
		t.Fatal("owned scope/title unavailable", got, err)
	}
	var observed observedGoogleEarthKML
	if err := xml.Unmarshal(got.Bytes, &observed); err != nil || observed.Document.Name != title {
		t.Fatal("owned complete literal document missing", err)
	}
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT PlotNumber,Longitude*-1,Latitude FROM Sample_Env
WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var name string
		var longitude, latitude float64
		if err := rows.Scan(&name, &longitude, &latitude); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if count >= len(observed.Document.Marks) {
			t.Fatal("source SQL row omitted")
		}
		mark := observed.Document.Marks[count]
		expected := strconv.FormatFloat(longitude, 'f', -1, 64) + "," + strconv.FormatFloat(latitude, 'f', -1, 64) + ",0"
		if mark.Name != name || mark.Point.Coordinates != expected || html.UnescapeString(mark.Description) != "." {
			t.Fatal("independent SQL/source NULL concatenation differs", mark, expected)
		}
		count++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil || count == 0 || count != got.PlacemarkCount ||
		count != len(observed.Document.Marks) {
		t.Fatal("owned source count incomplete", count, got.PlacemarkCount, err)
	}
	expected := append([]byte(nil), got.Bytes...)
	got.Bytes[0] = 'X'
	again, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", title)
	if err != nil || !bytes.Equal(again.Bytes, expected) {
		t.Fatal("caller bytes alias future owned read", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !bytes.Equal(config, after) || service.Projects.ContextID != state.ContextID {
		t.Fatal("KML preparation wrote preferences/context", err)
	}
}

func TestOwnedGoogleEarthKMLExternalSUFanoutAndRejectedRawDescription(t *testing.T) {
	service, state := vprotest.PrepareKMLContextFixture(t)
	db, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	var plot string
	err = db.QueryRow(`SELECT PlotNumber FROM Sample_Env WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL ORDER BY rowid LIMIT 1`).Scan(&plot)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "KML O'Brien #.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	writer, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`CREATE TABLE Earth_SU(PlotNumber VARCHAR,SiteUnit VARCHAR);
INSERT INTO Earth_SU VALUES(?,NULL),(?,''),(?,'other')`, plot, plot, plot)
	if err := errors.Join(err, writer.Close()); err != nil {
		t.Fatal(err)
	}
	next, err := service.SwitchContext(state.ContextID, vprocore.ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Earth", SUPath: path, Hierarchy: "None"})
	if err != nil {
		t.Fatal(err)
	}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	got, err := ReadGoogleEarthKML(service, context.Background(), next.ContextID, "PlotNumber", "")
	var observed observedGoogleEarthKML
	if err != nil || got == nil || got.SUPath != next.SUPath || got.SU != "Earth" || got.PlacemarkCount != 3 ||
		xml.Unmarshal(got.Bytes, &observed) != nil || len(observed.Document.Marks) != 3 || observed.Document.Name != "" {
		t.Fatal("owned external SU multiplicity/explicit empty title changed", got, err)
	}
	for _, mark := range observed.Document.Marks {
		if mark.Name != plot || html.UnescapeString(mark.Description) != plot+"." {
			t.Fatal("selected literal alias/raw field changed", mark)
		}
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Zone=X'00ff'`)
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := ReadGoogleEarthKML(service, context.Background(), next.ContextID, "Zone", "title"); got != nil || err == nil {
		t.Fatal("unavailable BLOB text produced owned partial success", got, err)
	}
	raw, err := ReadGoogleEarthLocations(service, context.Background(), next.ContextID, "Zone")
	if err != nil || raw.Report.Rows[0].Description.Storage != "blob" {
		t.Fatal("KML rejection changed accepted raw preview", err)
	}
	if got, err := ReadGoogleEarthKML(service, context.Background(), next.ContextID, "PlotNumber", "title"); got == nil || err != nil {
		t.Fatal("valid retry after unsupported field failed", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func TestOwnedGoogleEarthKMLCancellationOwnershipAndRejectedCoordinates(t *testing.T) {
	service, state := vprotest.PrepareKMLContextFixture(t)
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	for _, request := range [][3]string{{"stale", "Zone", "title"}, {state.ContextID, "zone", "title"}, {state.ContextID, "Zone", "\x00"}} {
		if got, err := ReadGoogleEarthKML(service, context.Background(), request[0], request[1], request[2]); got != nil || err == nil {
			t.Fatal("invalid owned KML scope/text accepted", got, err)
		}
	}
	owner := service.Projects.Sqlite
	prior := owner.AttachmentInfo["project"]
	owner.AttachmentInfo["project"] = owner.AttachmentInfo["VPro64"]
	got, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", "title")
	owner.AttachmentInfo["project"] = prior
	if got != nil || err == nil {
		t.Fatal("unowned KML source accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := ReadGoogleEarthKML(service, ctx, state.ContextID, "Zone", "title"); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("initial cancellation lost", got, err)
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
		value, err := ReadGoogleEarthKML(service, queued, state.ContextID, "Zone", "title")
		if value != nil {
			err = errors.Join(err, errors.New("cancelled KML returned output"))
		}
		done <- err
	}()
	select {
	case <-queued.Queued:
	case <-time.After(5 * time.Second):
		t.Fatal("KML preparation did not reach held snapshot")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued KML cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("KML cancellation retained operation lease")
	}
	owner.Mu.Unlock()
	locked = false
	if got, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", "title"); got == nil || err != nil {
		t.Fatal("cancellation discarded owned context", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	vprotest.MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Longitude=181 WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL`)
	before = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	if got, err := ReadGoogleEarthKML(service, context.Background(), state.ContextID, "Zone", "title"); got != nil || err == nil {
		t.Fatal("historical out-of-range KML coordinates clamped/accepted", got, err)
	}
	if raw, err := ReadGoogleEarthLocations(service, context.Background(), state.ContextID, "Zone"); raw == nil || err != nil {
		t.Fatal("KML-only bounds altered raw preview", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	impostor, selected := vprotest.PrepareKMLContextFixture(t)
	vprotest.MutateContextFixture(t, selected.ProjectPath, `ALTER TABLE Sample_Env RENAME TO Hidden_Env; CREATE VIEW Sample_Env AS SELECT * FROM Hidden_Env`)
	if got, err := ReadGoogleEarthKML(impostor, context.Background(), selected.ContextID, "Zone", "title"); got != nil || err == nil {
		t.Fatal("physical Env view accepted for owned KML", got, err)
	}
}
