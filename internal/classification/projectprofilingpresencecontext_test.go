package classification

import (
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
	"sync"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

func mutatePresenceContextFixture(t *testing.T, service *vprocore.ContextService, query string) {
	t.Helper()
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatal(err)
	}
}

func profilePresenceContextFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, ProjectPlotProfileRunRequest) {
	t.Helper()
	service, state := vprotest.ContextServiceFixture(t)
	mutatePresenceContextFixture(t, service, `
		DELETE FROM Sample_Profile;
		INSERT INTO Sample_Profile("Order","Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount")
			VALUES(1,'Veg',NULL,NULL,NULL,'TARGET','0e309',NULL,99);
		DELETE FROM Sample_Veg;
		INSERT INTO Sample_Veg(PlotNumber,Species,Cover1,TotalA,TotalB,Cover5a,ID)
			VALUES('108050','TARGET',0,NULL,NULL,NULL,2000000000),
			('8229723','TARGET',NULL,NULL,NULL,1,2000000001);
		DELETE FROM Sample_Lump;
		INSERT INTO Sample_Lump(LumpCode,SppCode,"Use") VALUES('LT','TARGET',-1)`)
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	lump, err := ReviewProjectPlotProfileLump(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, ProjectPlotProfileRunRequest{OriginalRules: review.Rules, ProjectLump: &lump}
}

func assertPresenceContextUnchanged(t *testing.T, service *vprocore.ContextService, state vprocore.ProjectState, request ProjectPlotProfileRunRequest,
	files map[string][]byte) {
	t.Helper()
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review.Rules, request.OriginalRules) {
		t.Fatal("species-only preview changed complete typed original rules/schema/counts", review, err)
	}
	if request.ProjectLump != nil {
		lump, err := ReviewProjectPlotProfileLump(service, context.Background(), state.ContextID)
		if err != nil || !reflect.DeepEqual(lump, *request.ProjectLump) {
			t.Fatal("species-only preview changed complete typed project Lump witness", lump, err)
		}
	}
	for role, expected := range files {
		actual, err := os.ReadFile(service.Projects.Sqlite.Attachments[role])
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("species-only preview changed physical database/history/support file", role, err)
		}
	}
}

func TestProjectProfileSpeciesPresenceOwnedPreviewWithoutWrites(t *testing.T) {
	service, state, request := profilePresenceContextFixture(t)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	result, err := PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	if err != nil || result.Project != "Sample" || result.Table != "Sample_Profile" || result.SU != "None" ||
		result.TotalPlots != 52 || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("owned presence ignored zero/non-NULL layer distinction or coerced irrelevant criteria", result, err)
	}
	assertPresenceContextUnchanged(t, service, state, request, files)
	request.ProjectLump = nil
	result, err = PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("Veg-only required irrelevant Lump ownership", result, err)
	}
	mutatePresenceContextFixture(t, service, `UPDATE Sample_Profile SET "Table"='Lump',Species='LT'`)
	review, err := ReviewProjectPlotProfile(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	lump, err := ReviewProjectPlotProfileLump(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request = ProjectPlotProfileRunRequest{OriginalRules: review.Rules, ProjectLump: &lump}
	files = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	result, err = PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("owned enabled Lump presence differs", result, err)
	}
	assertPresenceContextUnchanged(t, service, state, request, files)
}

func TestProjectProfileSpeciesPresenceOwnedRefusalCorrectionAndRetry(t *testing.T) {
	for _, failure := range []string{"rules", "count", "schema", "lump", "no-lump", "subvarieties", "observer"} {
		t.Run(failure, func(t *testing.T) {
			service, state, request := profilePresenceContextFixture(t)
			ctx := context.Background()
			var observe func(*vprocore.SqliteContext, *sql.Tx) error
			witness := errors.New("owned presence observation refused")
			switch failure {
			case "rules":
				mutatePresenceContextFixture(t, service, `UPDATE Sample_Profile SET Criteria='changed unused historical criterion'`)
			case "count":
				mutatePresenceContextFixture(t, service, `UPDATE Sample_Profile SET PlotCount=100`)
			case "schema":
				mutatePresenceContextFixture(t, service, `ALTER TABLE Sample_Profile ADD COLUMN Extra TEXT`)
			case "lump":
				mutatePresenceContextFixture(t, service, `UPDATE Sample_Profile SET "Table"='Lump',Species='LT'`)
				review, err := ReviewProjectPlotProfile(service, ctx, state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				request.OriginalRules = review.Rules
				mutatePresenceContextFixture(t, service, `UPDATE Sample_Lump SET "Use"=0`)
			case "no-lump":
				mutatePresenceContextFixture(t, service, `UPDATE Sample_Profile SET "Table"='Lump',Species='LT'`)
				review, err := ReviewProjectPlotProfile(service, ctx, state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				request.OriginalRules, request.ProjectLump = review.Rules, nil
			case "subvarieties":
				request.Subvarieties = true
			case "observer":
				observe = func(*vprocore.SqliteContext, *sql.Tx) error { return witness }
			}
			files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			result, err := PreviewProjectPlotProfilePresenceObserved(service, ctx, state.ContextID, request, observe)
			if err == nil || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) ||
				failure == "observer" && !errors.Is(err, witness) {
				t.Fatal("stale/unsupported/failed owned request returned partial or successful preview", result, err)
			}
			review, err := ReviewProjectPlotProfile(service, ctx, state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			lump, err := ReviewProjectPlotProfileLump(service, ctx, state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			request = ProjectPlotProfileRunRequest{OriginalRules: review.Rules, ProjectLump: &lump}
			result, err = PreviewProjectPlotProfilePresence(service, ctx, state.ContextID, request)
			expected := []string{"108050"}
			if failure == "lump" {
				expected = []string{}
			}
			if err != nil || !reflect.DeepEqual(result.PlotNumbers, expected) {
				t.Fatal("explicit refreshed witness/correction/retry failed", result, expected, err)
			}
			assertPresenceContextUnchanged(t, service, state, request, files)
		})
	}
}

func TestProjectProfileSpeciesPresenceInFlightCancelAndOwnedRetry(t *testing.T) {
	service, state, request := profilePresenceContextFixture(t)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	once := sync.Once{}
	cancelled := make(chan struct{})
	go func() {
		defer close(cancelled)
		select {
		case <-entered:
			cancel()
		case <-time.After(10 * time.Second):
			cancel()
		}
		close(release)
	}()
	observe := func(job *vprocore.SqliteContext, _ *sql.Tx) error {
		return job.Conn.Raw(func(raw any) error {
			driver, valid := raw.(*sqlite3.SQLiteConn)
			if !valid {
				return errors.New("owned presence cancellation requires actual SQLite connection")
			}
			return driver.RegisterFunc("vpro_profile_text", func(value any) (any, error) {
				if text, ok := value.(string); ok && text == "TARGET" {
					once.Do(func() { close(entered) })
					<-release
				}
				if vprocore.ProfileDriverNull(value) {
					return nil, nil
				}
				return value, nil
			}, true)
		})
	}
	result, err := PreviewProjectPlotProfilePresenceObserved(service, ctx, state.ContextID, request, observe)
	<-cancelled
	select {
	case <-entered:
	default:
		t.Fatal("cancellation did not enter the actual presence matching query")
	}
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) {
		t.Fatal("in-flight cancelled query returned partial/success-shaped preview", result, err)
	}
	result, err = PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("in-flight cancellation lost owned contexts/attachments or blocked explicit retry", result, err)
	}
	assertPresenceContextUnchanged(t, service, state, request, files)
}

func TestProjectProfileSpeciesPresenceRejectsChangedAttachmentIdentity(t *testing.T) {
	service, state, request := profilePresenceContextFixture(t)
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	owner := service.Projects.Sqlite
	original := owner.Attachments["project"]
	copyPath := filepath.Join(t.TempDir(), "byte-identical unrelated project.db")
	if err := os.WriteFile(copyPath, files["project"], 0600); err != nil {
		t.Fatal(err)
	}
	owner.Attachments["project"] = copyPath
	result, err := PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	owner.Attachments["project"] = original
	if err == nil || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) {
		t.Fatal("byte-identical different physical attachment supplied source authority", result, err)
	}
	result, err = PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("restored explicit owned attachment could not retry", result, err)
	}
	assertPresenceContextUnchanged(t, service, state, request, files)
}

func TestProjectProfileSpeciesPresenceRetiredAndMissingContexts(t *testing.T) {
	service, state, request := profilePresenceContextFixture(t)
	for _, ctx := range []context.Context{nil, func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}()} {
		if result, err := PreviewProjectPlotProfilePresence(service, ctx, state.ContextID, request); err == nil || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) {
			t.Fatal("missing/cancelled context returned preview", result, err)
		}
	}
	if result, err := PreviewProjectPlotProfilePresence(service, context.Background(), "foreign", request); err == nil || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) {
		t.Fatal("foreign context returned preview", result, err)
	}
	next, err := service.SelectPlotProfile(state.ContextID, vprocore.PlotProfileSource{Name: "Sample", Path: state.ProjectPath})
	if err != nil {
		t.Fatal(err)
	}
	if next.ContextID == state.ContextID {
		t.Fatal("profile source switch did not retire old owner")
	}
	if result, err := PreviewProjectPlotProfilePresence(service, context.Background(), state.ContextID, request); err == nil || !reflect.DeepEqual(result, projectPlotProfilePresencePreview{}) {
		t.Fatal("retired owner returned preview", result, err)
	}
	review, err := ReviewProjectPlotProfile(service, context.Background(), next.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request.OriginalRules = review.Rules
	result, err := PreviewProjectPlotProfilePresence(service, context.Background(), next.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("explicit current owner could not preview after switch", result, err)
	}
}
