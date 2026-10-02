package main

import (
	"database/sql"
	"testing"
)

func TestProjectMetadataEditorReferencesPreserveBoundNotesAndLiteralDuplicates(t *testing.T) {
	service, state := projectMetadataFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO USysTableOfLists(ListName,Item,Note,ItemDescription,ItemOrder) VALUES
		('PlotQualitySite','ZMETA1','Z exact Note',NULL,10000),
		('PlotQualitySite','ZMETA2','Z exact Note','',10001),
		('PlotQualitySite','ZMETA3',NULL,'NULL value',10002),
		('PlotQualitySite','ZMETA4','','Empty value',10003)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := metadataFileBytes(t, service)
	fields, err := service.ListProjectMetadataFields(t.Context(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 70 {
		t.Fatal("source editing fields missing")
	}
	for _, field := range fields {
		if field.Name != "DataQualityVeg" {
			continue
		}
		if field.ReferenceColumn != "Note" || field.ReferenceList != "PlotQualitySite" || !field.LimitToList {
			t.Fatal("metadata quality reference binding changed")
		}
		duplicates, nulls, empty := 0, 0, 0
		for _, option := range field.Options {
			if option.Value == nil {
				nulls++
			} else if *option.Value == "" {
				empty++
			} else if *option.Value == "Z exact Note" {
				duplicates++
			}
		}
		if duplicates != 2 || nulls < 1 || empty < 1 {
			t.Fatal("reference definitions/NULL/empty were collapsed:", duplicates, nulls, empty)
		}
	}
	assertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataEditorReferencesRejectMalformedTextAndStaleRequests(t *testing.T) {
	service, state := projectMetadataFixture(t)
	if _, err := service.ListProjectMetadataFields(t.Context(), "stale"); err == nil {
		t.Fatal("stale metadata field definitions accepted")
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO USysTableOfLists(ListName,Item,Note,ItemOrder)
		VALUES ('PlotQualitySite','ZMETABAD',CAST(x'FF' AS TEXT),10000)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before := metadataFileBytes(t, service)
	if _, err := service.ListProjectMetadataFields(t.Context(), state.ContextID); err == nil {
		t.Fatal("metadata reference Unicode was silently repaired or skipped")
	}
	assertMetadataFileBytes(t, service, before, "")
}
