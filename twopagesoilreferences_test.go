package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func twoPageSoilReferenceFixture(t *testing.T) (*ContextService, ProjectState, *sql.DB) {
	t.Helper()
	service, state, _ := twoPageSoilReadFixture(t, false, "TEXT")
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return service, state, db
}

func TestTwoPageSoilReferencesCanonicalSourceBindingsAndNoWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, _ := twoPageSoilReadFixture(t, external, "TEXT")
			before := databaseBytes(t, service.projects.sqlite.attachments)
			for _, test := range []struct {
				form     string
				bindings [][2]string
				counts   []int
			}{
				{"SoilHumusXL", [][2]string{
					{"Horizon", "HumusHorizon"}, {"HumusStructureKind", "HumusStructureKind"},
					{"HumusStructureDegree", "HumusStructureDegree"}, {"vonPost", "vonPost"},
					{"MycelAbundance", "MycelAbundance"}, {"FecalAbundance", "MycelAbundance"},
				}, []int{20, 9, 4, 11, 5, 5}},
				{"SoilMineralXL", [][2]string{
					{"ASP", "MinSoilAspect"}, {"Texture", "SoilTexture"},
					{"MineralStructureClass", "MineralStructureClass"}, {"MineralStructureKind", "MineralStructureKind"},
				}, []int{15, 30, 6, 10}},
			} {
				result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", test.form)
				if err != nil {
					t.Fatal(err)
				}
				if result.ContextID != state.ContextID || result.Project != "Sample" || result.Plot != "108050" ||
					result.Form != test.form || !strings.Contains(result.Source, "VLists.USysTableOfLists") ||
					len(result.Fields) != len(test.bindings) {
					t.Fatalf("source ownership/bindings: %+v", result)
				}
				for i, want := range test.bindings {
					got := result.Fields[i]
					if got.Column != want[0] || got.ListName != want[1] || len(got.Definitions.Rows) != test.counts[i] ||
						!reflect.DeepEqual(got.Definitions.Columns, []ProjectMetadataColumn{
							{"ListName", "TEXT"}, {"ListFilter", "TEXT"}, {"ItemOrder", "REAL"}, {"Item", "TEXT"},
							{"ItemDescription", "TEXT"}, {"FieldUsedIn", "TEXT"}, {"ValidateLoops", "TEXT"},
							{"Validate", "BOOLEAN"}, {"Note", "TEXT"}, {"Flag", "BOOLEAN"},
						}) {
						t.Fatalf("source field %d: %+v", i, got)
					}
				}
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
				t.Fatal("reference reading mutated a configured database")
			}
		})
	}
}

func TestTwoPageSoilReferencesPhysicalDefinitionsOrderAndFreshRetry(t *testing.T) {
	service, state, db := twoPageSoilReferenceFixture(t)
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='MycelAbundance';
		INSERT INTO USysTableOfLists(rowid,ListName,ItemOrder,Item,ItemDescription,Validate,Flag) VALUES
		(-9,'MycelAbundance',2,'A',NULL,-1,NULL),
		(-8,'MycelAbundance',2,'A','',0,-1),
		(-7,'MycelAbundance',NULL,NULL,'null',NULL,0),
		(-6,'MycelAbundance',1,'','empty',NULL,NULL)`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL")
	if err != nil {
		t.Fatal(err)
	}
	rows := result.Fields[4].Definitions.Rows
	var ids []string
	for _, row := range rows {
		ids = append(ids, row.RowID)
	}
	if !reflect.DeepEqual(ids, []string{"-7", "-6", "-9", "-8"}) ||
		!reflect.DeepEqual(rows, result.Fields[5].Definitions.Rows) {
		t.Fatal("NULL order, stable duplicate ties or shared fecal mapping lost", ids)
	}
	minusOne, zero := "-1", "0"
	if rows[0].Cells[3].Storage != "null" || !reflect.DeepEqual(rows[1].Cells[3], metadataText("")) ||
		rows[2].Cells[4].Storage != "null" || !reflect.DeepEqual(rows[3].Cells[4], metadataText("")) ||
		!reflect.DeepEqual(rows[2].Cells[7], ProjectMetadataCell{Storage: "integer", Integer: &minusOne}) ||
		!reflect.DeepEqual(rows[3].Cells[7], ProjectMetadataCell{Storage: "integer", Integer: &zero}) ||
		rows[2].Cells[9].Storage != "null" ||
		!reflect.DeepEqual(rows[3].Cells[9], ProjectMetadataCell{Storage: "integer", Integer: &minusOne}) {
		t.Fatal("NULL/empty/duplicate/Access BOOLEAN physical storage collapsed", rows)
	}
	if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
		t.Fatal("tagged reference reading wrote data")
	}
	if _, err := db.Exec(`UPDATE USysTableOfLists SET Item='C' WHERE rowid=-9`); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL")
	if err != nil || *fresh.Fields[4].Definitions.Rows[2].Cells[3].Text != "C" {
		t.Fatal("configured reference correction was replaced by a frozen catalogue", fresh, err)
	}
	if *result.Fields[4].Definitions.Rows[2].Cells[3].Text != "A" {
		t.Fatal("fresh read changed previously returned definitions")
	}
}

func TestTwoPageSoilReferencesMalformedTextRefusedBeforeTransportAndCorrectable(t *testing.T) {
	service, state, db := twoPageSoilReferenceFixture(t)
	for _, column := range []string{"ListFilter", "ItemOrder", "Item", "ItemDescription", "FieldUsedIn",
		"ValidateLoops", "Validate", "Note", "Flag"} {
		t.Run(column, func(t *testing.T) {
			var id int64
			if err := db.QueryRow(`INSERT INTO USysTableOfLists(ListName,ItemOrder,Item) VALUES ('MycelAbundance',-1,'A')
				RETURNING rowid`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE rowid=?`, id); err != nil {
					t.Error(err)
				}
			})
			if _, err := db.Exec(`UPDATE USysTableOfLists SET `+quoteHeaderIdentifier(column)+`=CAST(X'FF' AS TEXT) WHERE rowid=?`, id); err != nil {
				t.Fatal(err)
			}
			result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL")
			if result != nil || err == nil || !strings.Contains(err.Error(), "malformed Unicode") {
				t.Fatal("malformed source was transported/repaired or returned partial metadata", result, err)
			}
			if _, err := db.Exec(`UPDATE USysTableOfLists SET `+quoteHeaderIdentifier(column)+`=NULL WHERE rowid=?`, id); err != nil {
				t.Fatal(err)
			}
			if result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); result == nil || err != nil {
				t.Fatal("corrected malformed source could not be retried", result, err)
			}
		})
	}
}

func TestTwoPageSoilReferencesOwnershipErrorsCancellationAndMissingGroup(t *testing.T) {
	service, state, db := twoPageSoilReferenceFixture(t)
	for _, test := range []struct{ contextID, plot, form string }{
		{state.ContextID, "", "SoilHumusXL"}, {state.ContextID, "\xff", "SoilHumusXL"},
		{state.ContextID, "0108050", "SoilHumusXL"}, {state.ContextID, "108050 ", "SoilHumusXL"},
		{state.ContextID, "outside", "SoilMineralXL"}, {"stale", "108050", "SoilHumusXL"},
		{state.ContextID, "108050", "SoilHumus"}, {state.ContextID, "108050", "unknown"},
	} {
		result, err := service.readTwoPageSoilReferences(context.Background(), test.contextID, test.plot, test.form)
		if result != nil || err == nil {
			t.Fatal("invalid literal context/parent/source form accepted", test, result, err)
		}
	}
	if result, err := service.readTwoPageSoilReferences(nil, state.ContextID, "108050", "SoilHumusXL"); result != nil || err == nil {
		t.Fatal("nil request context accepted", result, err)
	}
	var absent *ContextService
	if result, err := absent.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); result != nil || err == nil {
		t.Fatal("nil service accepted", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.readTwoPageSoilReferences(ctx, state.ContextID, "108050", "SoilHumusXL"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read accepted", result, err)
	}
	if _, err := db.Exec(`DELETE FROM USysTableOfLists WHERE ListName='vonPost'`); err != nil {
		t.Fatal(err)
	}
	if result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); result != nil || err == nil || !strings.Contains(err.Error(), "vonPost") {
		t.Fatal("missing source group produced success-shaped partial result", result, err)
	}
}

func TestTwoPageSoilReferencesNumericCodesRemainSourceText(t *testing.T) {
	service, state, _ := twoPageSoilReferenceFixture(t)
	for _, test := range []struct {
		form        string
		field, last int
	}{
		{"SoilHumusXL", 3, 10},
		{"SoilMineralXL", 0, 14},
	} {
		result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", test.form)
		if err != nil {
			t.Fatal(err)
		}
		rows := result.Fields[test.field].Definitions.Rows
		if len(rows) != test.last+1 || rows[0].Cells[3].Storage != "null" {
			t.Fatal("numeric list's explicit NULL entry lost", rows)
		}
		for i := 1; i <= test.last; i++ {
			if !reflect.DeepEqual(rows[i].Cells[3], metadataText(strconv.Itoa(i))) {
				t.Fatal("numeric-looking Item text was coerced to field storage", rows[i])
			}
		}
	}
}

func TestTwoPageSoilReferencesIgnoreLegacyViewSubstitution(t *testing.T) {
	service, state, _ := twoPageSoilReferenceFixture(t)
	if _, err := service.projects.sqlite.conn.ExecContext(context.Background(), `DROP VIEW USysTableOfLists;
			CREATE TEMP VIEW USysTableOfLists AS SELECT 'fake' ListName, 'repaired' Item`); err != nil {
		t.Fatal(err)
	}
	result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilMineralXL")
	if err != nil || result == nil || len(result.Fields[1].Definitions.Rows) != 30 {
		t.Fatal("legacy view impersonated configured physical reference family", result, err)
	}
}

func TestTwoPageSoilReferencesInFlightCancellationRetainsCoordinator(t *testing.T) {
	service, state, _ := twoPageSoilReferenceFixture(t)
	owner := service.projects.sqlite
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	if err := owner.conn.Raw(func(driver any) error {
		conn, ok := driver.(*sqlite3.SQLiteConn)
		if !ok {
			return errors.New("soil reference cancellation requires the actual SQLite driver")
		}
		return conn.RegisterFunc("soilReferenceBarrier", func(value any) any {
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
			CREATE TEMP VIEW USysEnv AS SELECT soilReferenceBarrier(PlotNumber) PlotNumber FROM project.Sample_Env`); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() {
		result, err := service.readTwoPageSoilReferences(ctx, state.ContextID, "108050", "SoilHumusXL")
		if result != nil {
			err = errors.Join(err, errors.New("cancelled reference read returned definitions"))
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("actual reference read did not reach the barrier")
	}
	cancel()
	close(release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("actual in-flight reference read did not cancel", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled reference read did not finish")
	}
	if _, err := owner.conn.ExecContext(context.Background(), `DROP VIEW USysEnv; `+
		strings.Replace(originalView, "CREATE VIEW", "CREATE TEMP VIEW", 1)); err != nil {
		t.Fatal("caller-owned reference snapshot lost the coordinator", err)
	}
	if result, err := service.readTwoPageSoilReferences(context.Background(), state.ContextID, "108050", "SoilHumusXL"); result == nil || err != nil {
		t.Fatal("retry lost owned attachments/TEMP views after cancellation", result, err)
	}
}
