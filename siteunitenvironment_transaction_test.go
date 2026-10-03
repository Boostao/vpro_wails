package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func siteUnitEnvironmentFixture(t *testing.T) (*ContextService, ProjectState, SiteUnitEnvironmentTransfer) {
	t.Helper()
	service, state, _ := environmentSiteUnitFixture(t)
	c := service.projects.sqlite
	for _, item := range []struct{ role, sql string }{
		{"project", `UPDATE Sample_Env SET Flag=0 WHERE PlotNumber IN ('8229723','8428935');
			UPDATE Sample_SU SET SiteUnit='  Review '' unit  ' WHERE PlotNumber='8229723';
			UPDATE Sample_Admin SET UserSiteUnit='Old',SiteUnitShortName='Old short',SiteUnitLongName='Old long'
			WHERE Plot='8229723'`},
		{"VLists", `INSERT INTO MasterSiteUnitList(ID,SiteSeries,SiteSeriesLongName)
			SELECT COALESCE(MAX(ID),0)+1,'  Review '' unit  ','Master long' FROM MasterSiteUnitList`},
		{"VUser", `INSERT INTO UserSiteUnitList(ID,SiteSeries,SiteSeriesLongName)
			SELECT COALESCE(MAX(ID),0)+1,'  Review '' unit  ','Personal long' FROM UserSiteUnitList`},
	} {
		db, err := sql.Open("sqlite3", sqliteFileURI(c.attachments[item.role], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := db.Exec(item.sql)
		if err := errors.Join(writeErr, db.Close()); err != nil {
			t.Fatal(err)
		}
	}
	review, err := service.ReviewSiteUnitEnvironment(t.Context(), state.ContextID)
	if err != nil || len(review.Changes) != 4 {
		t.Fatal("original owned reverse review unavailable", len(review.Changes), err)
	}
	return service, state, SiteUnitEnvironmentTransfer{review, true}
}

func TestSiteUnitEnvironmentTransferCommitPreservationAndReplay(t *testing.T) {
	service, state, request := siteUnitEnvironmentFixture(t)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	review, err := service.ReviewSiteUnitEnvironment(t.Context(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review, request.Review) {
		t.Fatal("independent review changed", err)
	}
	assertProfileSUFiles(t, service, before)
	tables := map[string]ProjectMetadataTable{}
	for _, name := range []string{"Sample_Env", "Sample_SU", "Sample_Audit", "_table_metadata"} {
		tables[name], err = readSQLiteStorageRows(t.Context(), c.conn, "project", name, "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request)
	if err != nil || result == nil || result.ChangedRows != 2 || result.ChangedCells != 4 || result.HistoryID != "1" {
		t.Fatal("reverse transfer not committed exactly", result, err)
	}
	for name, original := range tables {
		actual, err := readSQLiteStorageRows(t.Context(), c.conn, "project", name, "", nil, "")
		if err != nil || !reflect.DeepEqual(original, actual) {
			t.Fatal("unrelated data/audits/descriptions changed", name, err)
		}
	}
	after := databaseBytes(t, c.attachments)
	for role, bytes := range before {
		if !os.SameFile(c.attachmentInfo[role], c.attachmentInfo["project"]) && !reflect.DeepEqual(bytes, after[role]) {
			t.Fatal("support data changed", role)
		}
	}
	if _, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed proposal replay accepted")
	}
	assertProfileSUFiles(t, service, after)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE Sample_Admin SET UserSiteUnit='Old',SiteUnitShortName='Old short',SiteUnitLongName='Old long'
		WHERE Plot='8229723'; UPDATE Sample_Admin SET UserSiteUnit=NULL WHERE Plot='8428935'`); err != nil {
		t.Fatal(err)
	}
	after = databaseBytes(t, c.attachments)
	if _, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("old proposal replayed after independent target reset")
	}
	assertProfileSUFiles(t, service, after)
	fresh, err := service.ReviewSiteUnitEnvironment(t.Context(), state.ContextID)
	if err != nil || fresh.HistoryHash == "" {
		t.Fatal("fresh review lost history revision", err)
	}
	result, err = service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, SiteUnitEnvironmentTransfer{fresh, true})
	if err != nil || result.HistoryID != "2" {
		t.Fatal("new reviewed event did not preserve prior history", result, err)
	}
}

func TestSiteUnitEnvironmentTransferDriftRollbackAndRetainedRetry(t *testing.T) {
	for _, variant := range []string{"source", "target", "master", "personal", "duplicate", "locked", "abort", "late-other", "protected-history"} {
		t.Run(variant, func(t *testing.T) {
			service, state, request := siteUnitEnvironmentFixture(t)
			c := service.projects.sqlite
			role, statement := "project", ""
			switch variant {
			case "source":
				statement = `UPDATE Sample_SU SET SiteUnit='Drift' WHERE PlotNumber='8229723'`
			case "target":
				statement = `UPDATE Sample_Admin SET SiteUnitLongName='Drift' WHERE Plot='8229723'`
			case "master":
				role, statement = "VLists", `UPDATE MasterSiteUnitList SET SiteSeriesLongName='Drift' WHERE SiteSeries='  Review '' unit  '`
			case "personal":
				role, statement = "VUser", `UPDATE UserSiteUnitList SET SiteSeriesLongName='Drift' WHERE SiteSeries='  Review '' unit  '`
			case "duplicate":
				role, statement = "VUser", `INSERT INTO UserSiteUnitList(ID,SiteSeries,SiteSeriesLongName)
					SELECT MAX(ID)+1,'  Review '' unit  ','Duplicate' FROM UserSiteUnitList`
			case "locked":
				statement = `UPDATE Sample_Env SET Flag=-1 WHERE PlotNumber='8229723'`
			case "abort":
				statement = `CREATE TRIGGER reverse_guard BEFORE UPDATE ON Sample_Admin
					WHEN OLD.Plot='8428935' BEGIN SELECT RAISE(ABORT,'Independent rejection'); END`
			case "late-other":
				statement = `CREATE TRIGGER reverse_guard AFTER UPDATE ON Sample_Admin
					BEGIN UPDATE Sample_SU SET SiteUnit='Unreviewed' WHERE PlotNumber='Foreign'; END`
			case "protected-history":
				statement = siteUnitEnvironmentHistorySQL + `;
					CREATE TRIGGER reverse_guard AFTER INSERT ON __VPRO_SUEnvironmentHistory
					BEGIN UPDATE Sample_Admin SET EnteredBy='Unreviewed' WHERE Plot='8229723'; END`
			}
			db, err := sql.Open("sqlite3", sqliteFileURI(c.attachments[role], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(statement); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, c.attachments)
			if _, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("drift or unreviewed mutation accepted")
			}
			assertProfileSUFiles(t, service, before)
			if variant == "abort" || variant == "late-other" || variant == "protected-history" {
				if _, err := db.Exec(`DROP TRIGGER reverse_guard`); err != nil {
					t.Fatal(err)
				}
				if variant == "protected-history" {
					if _, err := db.Exec(`DROP TABLE __VPRO_SUEnvironmentHistory`); err != nil {
						t.Fatal(err)
					}
				}
				if result, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request); err != nil || result.ChangedCells != 4 {
					t.Fatal("retained reverse retry failed", result, err)
				}
			}
		})
	}
}

func TestSiteUnitEnvironmentTransferStrictTransportAndCancellation(t *testing.T) {
	service, state, request := siteUnitEnvironmentFixture(t)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded SiteUnitEnvironmentTransfer
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("typed review transport changed", err)
	}
	for _, raw := range []string{`{}`, `{"review":{},"confirmed":true}`, `{"review":null,"confirmed":true}`,
		`{"review":{"path":"\ud800"},"confirmed":true}`, `{"review":{},"confirmed":true,"overwrite":true}`} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("partial/malformed transport accepted")
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.TransferSiteUnitEnvironment(ctx, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled reverse transfer accepted", err)
	}
	if _, err := service.TransferSiteUnitEnvironment(t.Context(), "stale", request); err == nil {
		t.Fatal("stale identity accepted")
	}
	request.Confirmed = false
	if _, err := service.TransferSiteUnitEnvironment(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("unconfirmed transfer accepted")
	}
	assertProfileSUFiles(t, service, before)
}
