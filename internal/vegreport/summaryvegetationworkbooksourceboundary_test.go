package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestSummaryWorkbookOwnedSourceIndependentGatesAndOptionalTables(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	options := summaryVegetationWorkbookOptions{Average: 1, Display: 6, IncludeEnv: true}
	for _, gate := range []*bool{&service.enabled, &service.environment.enabled, &service.environment.numericEnabled} {
		*gate = false
		got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
		if err == nil || !strings.Contains(err.Error(), "requires enabled environmental") ||
			!reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
			t.Fatal("independent inspection gate was bypassed", err)
		}
		*gate = true
	}
	owner := service.environment.report.contexts.Projects.Sqlite
	service.enabled, service.environment.enabled, service.environment.numericEnabled = false, false, false
	vprotest.MutateContextFixture(t, owner.Attachments["VPro64"],
		"ALTER TABLE USysShortVegSummaryFieldList RENAME TO MissingFields")
	vprotest.MutateContextFixture(t, owner.Attachments["project"], "ALTER TABLE "+
		vprocore.QuoteHeaderIdentifier(state.ActiveProject+"_Env")+" RENAME TO MissingEnv")
	options.IncludeEnv = false
	got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || got.Input.Environment != nil || got.Environment.FieldList.Rows != nil || len(got.Raw.Personal.Columns) == 0 {
		t.Fatal("vegetation-only source borrowed an environmental gate/table or lost required personal schema", err)
	}
	service.environment.report.Enabled = false
	got, err = summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("vegetation gate was bypassed", err)
	}
}

func TestSummaryWorkbookOwnedSourceMalformedPeerAndReadErrorRetry(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	owner := service.environment.report.contexts.Projects.Sqlite
	options := summaryVegetationWorkbookOptions{Average: 1, Display: 6}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `INSERT INTO USysUserSpp
		(Code,ScientificName,LifeForm,CodeType) VALUES('UNUSED',CAST(X'FF' AS TEXT),1,'U')`)
	got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !strings.Contains(err.Error(), "malformed Unicode") ||
		!reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("malformed unused physical text could be repaired in later source approval JSON", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], `UPDATE USysUserSpp SET ScientificName='' WHERE Code='UNUSED'`)
	got, err = summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err != nil || len(got.Raw.Personal.Rows) != 1 {
		t.Fatal("source did not recover after literal data correction", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], "ALTER TABLE USysUserSpp RENAME TO MissingPersonal")
	got, err = summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !strings.Contains(err.Error(), "VUser.USysUserSpp") ||
		!reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("missing required physical source returned partial data", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VUser"], "ALTER TABLE MissingPersonal RENAME TO USysUserSpp")
	if _, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, options, vprocore.PublicationReadSnapshotHooks{}); err != nil {
		t.Fatal("source did not recover after physical table restoration", err)
	}
}

func TestSummaryWorkbookOwnedSourceContextAndOptionAdmission(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, false)
	options := summaryVegetationWorkbookOptions{Average: 1, Display: 6}
	got, err := readSummaryVegetationWorkbookSource(nil, service, nil, nil, nil, state.ContextID, options)
	if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("nil request context admitted", err)
	}
	for _, invalid := range []summaryVegetationWorkbookOptions{
		{Average: 0, Display: 6}, {Average: 1, Display: 7},
		{Average: 1, Display: 4, ShowClass: 9},
		{Average: 1, Display: 6, Title: string([]byte{0xff})},
	} {
		got, err := summaryWorkbookFixtureSource(t.Context(), service, state.ContextID, invalid, vprocore.PublicationReadSnapshotHooks{})
		if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
			t.Fatal("invalid explicit source option admitted", invalid, err)
		}
	}
	got, err = summaryWorkbookFixtureSource(t.Context(), service, "stale", options, vprocore.PublicationReadSnapshotHooks{})
	if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
		t.Fatal("stale context admitted", err)
	}
	contexts := service.environment.report.contexts
	_, err = vprocore.WithContextPlotRequest(t.Context(), contexts, state.ContextID, func(plots *vprocore.PlotService) (struct{}, error) {
		return vprocore.WithOwnedPreferenceSnapshot(t.Context(), plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (struct{}, error) {
			got, err := readSummaryVegetationWorkbookSource(t.Context(), service, plots, owner, tx, "other", options)
			if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
				t.Fatal("different scoped report identity admitted", err)
			}
			plots.ContextScoped = false
			got, err = readSummaryVegetationWorkbookSource(t.Context(), service, plots, owner, tx, state.ContextID, options)
			if err == nil || !reflect.DeepEqual(got, summaryVegetationWorkbookSource{}) {
				t.Fatal("unscoped pool owner admitted", err)
			}
			return struct{}{}, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}
