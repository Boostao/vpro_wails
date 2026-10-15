package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

func twoPageGoogleEarthFixture(t *testing.T, external bool) (*TwoPageGoogleEarthService, ProjectState, *sql.DB) {
	t.Helper()
	contexts, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	if _, err := db.Exec(`UPDATE Sample_Env SET Longitude=123.25,Latitude=53.5,
		PlotRepresenting='<b>Historical & café</b>' WHERE PlotNumber='108050';
		UPDATE Sample_Env SET PlotRepresenting=CAST(X'FF' AS TEXT) WHERE PlotNumber<>'108050'`); err != nil {
		t.Fatal(err)
	}
	mutateContextFixture(t, state.SUPath, `INSERT INTO Report_SU VALUES('108050','duplicate membership')`)
	service, err := NewTwoPageGoogleEarthService(contexts, func(name string) (string, bool) {
		return "true", name == twoPageGoogleEarthReviewEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, db
}

const twoPageGoogleEarthRequest = `{"descriptionField":"PlotRepresenting","title":"A & B"}`

func TestTwoPageGoogleEarthOwnedOptionsAndHistoricalValues(t *testing.T) {
	service, state, _ := twoPageGoogleEarthFixture(t, false)
	store := service.contexts.projects.preferences
	values, err := store.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	historical := GoogleEarthPreferenceValues{Title: "historical\x01title", DescriptionField: "missing\x00field"}
	values["ReportOptions"].(map[string]any)["GoogleEarthPlaceName"] = historical.Title
	values["ReportOptions"].(map[string]any)["GoogleEarthDescField"] = historical.DescriptionField
	raw, err := yaml.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	check := assertPlotLocationFacadeReadOnly(t, service.contexts)
	defer check()
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		got, err := service.GetOptions(context.Background(), state.ContextID, "108050", form)
		if err != nil || got == nil || got.ContextID != state.ContextID || got.Project != "Sample" ||
			got.ProjectPath != state.ProjectPath || got.Plot != "108050" || got.Form != form || got.Values != historical ||
			len(got.Fields) < 2 || got.Fields[0].Name != "PlotNumber" {
			t.Fatal("owned historical options defaulted/repaired or physical fields lost", got, err)
		}
		got.Fields[0].Name = "caller"
		again, err := service.GetOptions(context.Background(), state.ContextID, "108050", form)
		if err != nil || again.Fields[0].Name != "PlotNumber" {
			t.Fatal("option field snapshots are shared", err)
		}
	}
	for _, request := range []struct{ context, plot, form string }{
		{"stale", "108050", "FS882-8x6XL"}, {state.ContextID, "outside", "FS882-8x6XL"},
		{state.ContextID, "108050", "FS882-6x4XL"}, {state.ContextID, "\xff", "FS882-8x6XL"},
	} {
		if got, err := service.GetOptions(context.Background(), request.context, request.plot, request.form); err == nil || got != nil {
			t.Fatal("unowned source options returned data", got, err)
		}
	}
	service.review = false
	if got, err := service.GetOptions(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err == nil || got != nil {
		t.Fatal("source options inherited another feature", got, err)
	}
}

func TestTwoPageGoogleEarthOptionsPersistenceErrorAndRetry(t *testing.T) {
	service, state, _ := twoPageGoogleEarthFixture(t, false)
	path := service.contexts.projects.preferences.path
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ReportOptions: [broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetOptions(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err == nil || got != nil {
		t.Fatal("invalid saved options became successful defaults", got, err)
	}
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	check := assertPlotLocationFacadeReadOnly(t, service.contexts)
	defer check()
	if got, err := service.GetOptions(context.Background(), state.ContextID, "108050", "FS882-8x6XL"); err != nil || got == nil {
		t.Fatal("corrected saved options could not retry", err)
	}
	if got, err := service.GetOptions(nil, state.ContextID, "108050", "FS882-8x6XL"); err == nil || got != nil {
		t.Fatal("nil source-options context accepted", got, err)
	}
}

func TestTwoPageGoogleEarthSingleSavedPlotAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, _ := twoPageGoogleEarthFixture(t, external)
			check := assertPlotLocationFacadeReadOnly(t, service.contexts)
			defer check()
			for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", form, twoPageGoogleEarthRequest)
				if err != nil {
					t.Fatal(err)
				}
				if got.ContextID != state.ContextID || got.Project != "Sample" || got.ProjectPath != state.ProjectPath ||
					got.Plot != "108050" || got.Form != form || got.DescriptionField != "PlotRepresenting" ||
					got.Title != "A & B" || got.ByteCount != len([]byte(got.KML)) || len(got.Original.Rows) != 1 {
					t.Fatalf("saved single-plot provenance = %+v", got)
				}
				var document struct {
					XMLName  xml.Name
					Document struct {
						Name       string `xml:"name"`
						Placemarks []struct {
							Name        string `xml:"name"`
							Description string `xml:"description"`
							Point       struct {
								Coordinates string `xml:"coordinates"`
							} `xml:"Point"`
						} `xml:"Placemark"`
					} `xml:"Document"`
				}
				if err := xml.Unmarshal([]byte(got.KML), &document); err != nil {
					t.Fatal(err)
				}
				if document.XMLName.Space != "http://earth.google.com/kml/2.1" || document.Document.Name != "A & B" ||
					len(document.Document.Placemarks) != 1 {
					t.Fatalf("namespace/title/current plot or duplicate SU expansion = %+v", document)
				}
				mark := document.Document.Placemarks[0]
				if mark.Name != "108050" || mark.Point.Coordinates != "-123.25,53.5,0" ||
					mark.Description != "&lt;b&gt;Historical &amp; café&lt;/b&gt;." {
					t.Fatalf("single source placemark = %+v", mark)
				}
				for index, column := range got.Original.Columns {
					if column.Name == "Longitude" && !reflect.DeepEqual(got.Original.Rows[0].Cells[index], siviReal(123.25)) {
						t.Fatal("stored longitude was negated or repaired")
					}
				}
				raw, err := json.Marshal(got)
				var roundTrip TwoPageGoogleEarthReview
				if err != nil || json.Unmarshal(raw, &roundTrip) != nil || !reflect.DeepEqual(*got, roundTrip) {
					t.Fatal("actual Go transport round-trip changed", err)
				}
				*got.Original.Rows[0].Cells[0].Text = "caller"
				again, err := service.GetReview(context.Background(), state.ContextID, "108050", form, twoPageGoogleEarthRequest)
				if err != nil || again.Original.Rows[0].Cells[0].Text == nil || *again.Original.Rows[0].Cells[0].Text != "108050" {
					t.Fatal("caller-owned snapshot changed a subsequent read", err)
				}
			}
		})
	}
}

func TestTwoPageGoogleEarthNullEmptyAndLiteralDescriptionColumn(t *testing.T) {
	service, state, db := twoPageGoogleEarthFixture(t, false)
	const field = `Description ] ; " café`
	if _, err := db.Exec(`ALTER TABLE Sample_Env ADD COLUMN ` + quoteHeaderIdentifier(field)); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{nil, "", "literal"} {
		if _, err := db.Exec(`UPDATE Sample_Env SET `+quoteHeaderIdentifier(field)+`=? WHERE PlotNumber='108050'`, value); err != nil {
			t.Fatal(err)
		}
		check := assertPlotLocationFacadeReadOnly(t, service.contexts)
		raw, err := json.Marshal(GoogleEarthKMLRequest{DescriptionField: field, Title: ""})
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", string(raw))
		if err != nil {
			t.Fatal(err)
		}
		cell := got.Original.Rows[0].Cells[len(got.Original.Columns)-1]
		wantCell, wantDescription := ProjectMetadataCell{Storage: "null"}, "."
		if text, ok := value.(string); ok {
			wantCell, wantDescription = metadataText(text), text+"."
		}
		if !reflect.DeepEqual(cell, wantCell) || !strings.Contains(got.KML, "<description>"+wantDescription+"</description>") {
			t.Fatalf("NULL/empty/literal description changed: %+v, %s", cell, got.KML)
		}
		check()
	}
}

func TestTwoPageGoogleEarthLiteralParentStorageAcrossAffinities(t *testing.T) {
	for _, parent := range []string{"Env", "Admin"} {
		for _, affinity := range []string{"TEXT", "BLOB", "", "INTEGER", "NUMERIC", "REAL"} {
			t.Run(parent+"-"+affinity, func(t *testing.T) {
				service, state, db := twoPageGoogleEarthFixture(t, false)
				sqlText := `ALTER TABLE Sample_Env RENAME TO Archived_Env;
					CREATE TABLE Sample_Env(PlotNumber ` + affinity + `,Longitude,Latitude,PlotRepresenting);
					INSERT INTO Sample_Env VALUES('108050',123.25,53.5,'literal')`
				if parent == "Admin" {
					sqlText = `ALTER TABLE Sample_Admin RENAME TO Archived_Admin;
						CREATE TABLE Sample_Admin(Plot ` + affinity + `);
						INSERT INTO Sample_Admin VALUES('108050')`
				}
				if _, err := db.Exec(sqlText); err != nil {
					t.Fatal(err)
				}
				check := assertPlotLocationFacadeReadOnly(t, service.contexts)
				got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest)
				want := affinity == "TEXT" || affinity == "BLOB" || affinity == ""
				if want && (err != nil || got == nil) || !want && (err == nil || got != nil) {
					t.Fatalf("literal %s %s ownership = %+v, %v", parent, affinity, got, err)
				}
				check()
			})
		}
	}
}

func TestTwoPageGoogleEarthSelectedMembershipAndNoSUContext(t *testing.T) {
	service, state, db := twoPageGoogleEarthFixture(t, false)
	if _, err := db.Exec(`CREATE TEMP TABLE PeerEnv AS SELECT * FROM Sample_Env WHERE PlotNumber='108050';
		UPDATE PeerEnv SET PlotNumber='outside';
		INSERT INTO Sample_Env SELECT * FROM PeerEnv;
		CREATE TEMP TABLE PeerAdmin AS SELECT * FROM Sample_Admin WHERE Plot='108050';
		UPDATE PeerAdmin SET Plot='outside';
		INSERT INTO Sample_Admin SELECT * FROM PeerAdmin`); err != nil {
		t.Fatal(err)
	}
	check := assertPlotLocationFacadeReadOnly(t, service.contexts)
	if got, err := service.GetReview(context.Background(), state.ContextID, "outside", "FS882-8x6XL", twoPageGoogleEarthRequest); err == nil ||
		got != nil || !strings.Contains(err.Error(), "outside the selected context") {
		t.Fatal("otherwise valid physical parent bypassed selected membership", got, err)
	}
	check()
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	check = assertPlotLocationFacadeReadOnly(t, service.contexts)
	if got, err := service.GetReview(context.Background(), next.ContextID, "outside", "FS882-8x6XL", twoPageGoogleEarthRequest); err != nil ||
		got == nil || got.ContextID != next.ContextID || got.Plot != "outside" {
		t.Fatal("owned no-SU single-plot review failed", got, err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err == nil || got != nil {
		t.Fatal("actually retired source context remained readable", got, err)
	}
	check()
}

func TestTwoPageGoogleEarthCoordinateBoundariesAndIntegerStorage(t *testing.T) {
	for _, test := range []struct {
		name, coordinates   string
		longitude, latitude any
		integer             bool
	}{
		{"zero-north", "-0,90,0", 0.0, 90.0, false},
		{"west-south", "-180,-90,0", 180.0, -90.0, false},
		{"integer-west-north", "-180,90,0", int64(180), int64(90), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, state, db := twoPageGoogleEarthFixture(t, false)
			if test.integer {
				if _, err := db.Exec(`ALTER TABLE Sample_Env RENAME TO Archived_Env;
					CREATE TABLE Sample_Env(PlotNumber TEXT,Longitude,Latitude,PlotRepresenting);
					INSERT INTO Sample_Env VALUES('108050',0,0,'integer')`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`UPDATE Sample_Env SET Longitude=?,Latitude=? WHERE PlotNumber='108050'`,
				test.longitude, test.latitude); err != nil {
				t.Fatal(err)
			}
			check := assertPlotLocationFacadeReadOnly(t, service.contexts)
			got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest)
			if err != nil || got == nil || !strings.Contains(got.KML, "<coordinates>"+test.coordinates+"</coordinates>") {
				t.Fatal("valid coordinate boundary/storage refused or changed", err)
			}
			if test.integer && (got.Original.Rows[0].Cells[1].Storage != "integer" || got.Original.Rows[0].Cells[2].Storage != "integer") {
				t.Fatal("integer coordinate originals changed", got.Original.Rows)
			}
			check()
		})
	}
}

func TestTwoPageGoogleEarthRefusesInvalidPhysicalSourceAndRetries(t *testing.T) {
	failures := map[string]string{
		"null-longitude": `UPDATE Sample_Env SET Longitude=NULL WHERE PlotNumber='108050'`,
		"null-latitude":  `UPDATE Sample_Env SET Latitude=NULL WHERE PlotNumber='108050'`,
		"longitude-text": `UPDATE Sample_Env SET Longitude='not numeric' WHERE PlotNumber='108050'`,
		"latitude-text":  `UPDATE Sample_Env SET Latitude='not numeric' WHERE PlotNumber='108050'`,
		"negative-west":  `UPDATE Sample_Env SET Longitude=-123.25 WHERE PlotNumber='108050'`,
		"west-range":     `UPDATE Sample_Env SET Longitude=181 WHERE PlotNumber='108050'`,
		"latitude-range": `UPDATE Sample_Env SET Latitude=91 WHERE PlotNumber='108050'`,
		"description":    `UPDATE Sample_Env SET PlotRepresenting=CAST(X'FF' AS TEXT) WHERE PlotNumber='108050'`,
		"other-text":     `UPDATE Sample_Env SET Location=CAST(X'FF' AS TEXT) WHERE PlotNumber='108050'`,
		"numeric-desc":   `UPDATE Sample_Env SET PlotRepresenting=X'00FF' WHERE PlotNumber='108050'`,
		"missing-field":  `DROP VIEW qryHerbariumReportData; ALTER TABLE Sample_Env RENAME COLUMN Longitude TO Unsupported`,
		"rowid-shadow":   `ALTER TABLE Sample_Env ADD COLUMN rowid TEXT`,
		"hidden-column":  `ALTER TABLE Sample_Env ADD COLUMN Hidden TEXT GENERATED ALWAYS AS ('hidden') VIRTUAL`,
		"missing-admin":  `DELETE FROM Sample_Admin WHERE Plot='108050'`,
		"duplicate-env":  `ALTER TABLE Sample_Env RENAME TO Archived_Env; CREATE TABLE Sample_Env AS SELECT * FROM Archived_Env; INSERT INTO Sample_Env SELECT * FROM Archived_Env WHERE PlotNumber='108050'`,
		"duplicate-admin": `ALTER TABLE Sample_Admin RENAME TO Archived_Admin; CREATE TABLE Sample_Admin AS SELECT * FROM Archived_Admin;
			INSERT INTO Sample_Admin SELECT * FROM Archived_Admin WHERE Plot='108050'`,
		"env-view": `ALTER TABLE Sample_Env RENAME TO Archived_Env; CREATE VIEW Sample_Env AS SELECT * FROM Archived_Env`,
	}
	for name, mutation := range failures {
		t.Run(name, func(t *testing.T) {
			service, state, db := twoPageGoogleEarthFixture(t, false)
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			check := assertPlotLocationFacadeReadOnly(t, service.contexts)
			if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err == nil || got != nil {
				t.Fatalf("invalid physical source returned a successful document: %+v, %v", got, err)
			}
			check()
			if name == "description" {
				if _, err := db.Exec(`UPDATE Sample_Env SET PlotRepresenting='' WHERE PlotNumber='108050'`); err != nil {
					t.Fatal(err)
				}
				if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err != nil || got == nil {
					t.Fatal("source correction cannot retry", err)
				}
			}
		})
	}
}

func TestTwoPageGoogleEarthIndependentGateAndStrictRequests(t *testing.T) {
	service, state, _ := twoPageGoogleEarthFixture(t, false)
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"TRUE", true, false, true}, {"1", true, false, true}, {" true ", true, false, true}} {
		got, err := NewTwoPageGoogleEarthService(service.contexts, func(name string) (string, bool) {
			if name != "VPRO_TWO_PAGE_GOOGLE_EARTH_REVIEW" {
				t.Fatal("unrelated feature lookup", name)
			}
			return test.value, test.present
		})
		if (err != nil) != test.invalid || err == nil && got.review != test.enabled {
			t.Fatal("nonliteral or inherited feature", test, got, err)
		}
		if err == nil && !got.review {
			if result, err := got.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err == nil || result != nil {
				t.Fatal("default-off facade returned data", result, err)
			}
		}
	}
	for _, request := range []string{
		`{}`, `{"descriptionField":"PlotRepresenting","title":null}`, `{"descriptionField":"","title":"x"}`,
		`{"descriptionField":"Missing","title":"x"}`, `{"descriptionField":"PlotRepresenting","title":"x","unknown":1}`,
		`{"descriptionField":"PlotRepresenting","title":"x","title":"y"}`,
		`{"descriptionField":"PlotRepresenting","title":"x","\u0074itle":"y"}`,
		`{"descriptionField":"PlotRepresenting","title":"\ud800"}`,
		`{"descriptionField":"PlotRepresenting","title":"\u0000"}`, twoPageGoogleEarthRequest + `{}`,
	} {
		if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", request); err == nil || got != nil {
			t.Fatalf("invalid strict request crossed boundary: %s, %+v, %v", request, got, err)
		}
	}
	for _, request := range []struct{ context, plot, form string }{
		{"stale", "108050", "FS882-8x6XL"}, {state.ContextID, "108050x", "FS882-8x6XL"},
		{state.ContextID, "0108050", "FS882-8x6XL"}, {state.ContextID, "108050 ", "FS882-8x6XL"},
		{state.ContextID, "", "FS882-8x6XL"}, {state.ContextID, "\xff", "FS882-8x6XL"},
		{state.ContextID, "108050", "FS882-6x4XL"},
	} {
		if got, err := service.GetReview(context.Background(), request.context, request.plot, request.form, twoPageGoogleEarthRequest); err == nil || got != nil {
			t.Fatal("unowned source request returned data", request, got, err)
		}
	}
	if got, err := NewTwoPageGoogleEarthService(nil, nil); err == nil || got != nil {
		t.Fatal("missing ownership accepted", got, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, cancelled} {
		for _, unavailable := range []*TwoPageGoogleEarthService{nil, {}, service} {
			if got, err := unavailable.GetReview(ctx, state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err == nil || got != nil ||
				ctx != nil && !errors.Is(err, context.Canceled) {
				t.Fatal("missing/cancelled context returned data", got, err)
			}
		}
	}
}

func TestTwoPageGoogleEarthInFlightCancellationAndOwnedRetry(t *testing.T) {
	service, state, _ := twoPageGoogleEarthFixture(t, false)
	owner := service.contexts.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("single-plot cancellation requires actual SQLite")
		}
		return conn.RegisterFunc("singlePlotReadBarrier", func(value any) any {
			once.Do(func() { close(entered) })
			<-release
			return value
		}, false)
	}); err != nil {
		t.Fatal(err)
	}
	var originalView string
	if err := owner.conn.QueryRowContext(context.Background(), `SELECT sql FROM sqlite_temp_master WHERE name='USysEnv'`).Scan(&originalView); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv;
		CREATE TEMP VIEW USysEnv AS SELECT singlePlotReadBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		got, err := service.GetReview(ctx, state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest)
		if got != nil {
			err = errors.Join(err, errors.New("cancelled single-plot read returned data"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("single-plot read did not reach SQLite")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("in-flight cancellation lost", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("single-plot cancellation did not settle")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv; `+
		strings.Replace(originalView, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal("cancelled single-plot read lost owned coordinator", err)
	}
	if got, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", twoPageGoogleEarthRequest); err != nil || got == nil {
		t.Fatal("owned single-plot retry failed", err)
	}
}
