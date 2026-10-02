package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
)

func TestSoilSuggestionsCanonicalFamilyNullableDuplicateMetadataAndCancellation(t *testing.T) {
	service, state := contextServiceFixture(t)
	seals := map[string][]byte{}
	for _, file := range service.projects.supportPaths {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		seals[file] = content
	}
	rows, err := service.ListSoilSuggestions(context.Background(), state.ContextID)
	if err != nil || len(rows) != 110 {
		t.Fatalf("canonical soil suggestions: count=%d %v", len(rows), err)
	}
	conn := service.projects.sqlite.conn
	if _, err := conn.ExecContext(context.Background(), `CREATE TEMP TABLE SoilFixture AS SELECT * FROM USysTableOfLists;
		DROP VIEW USysTableOfLists; ALTER TABLE SoilFixture RENAME TO USysTableOfLists;
		INSERT INTO USysTableOfLists (ListName,Item,ItemDescription,Validate,Flag) VALUES
		('MycelAbundance','fixture',NULL,-1,NULL),('MycelAbundance','fixture','',0,-1)`); err != nil {
		t.Fatal(err)
	}
	rows, err = service.ListSoilSuggestions(context.Background(), state.ContextID)
	if err != nil || len(rows) != 112 {
		t.Fatal(rows, err)
	}
	var fixtures []SoilSuggestion
	for _, row := range rows {
		if row.Item != nil && *row.Item == "fixture" {
			fixtures = append(fixtures, row)
		}
	}
	if len(fixtures) != 2 || fixtures[0].ItemDescription != nil || fixtures[1].ItemDescription == nil ||
		*fixtures[1].ItemDescription != "" || fixtures[0].Validate == nil || !*fixtures[0].Validate ||
		fixtures[0].Flag != nil || fixtures[1].Flag == nil || !*fixtures[1].Flag {
		t.Fatal("NULL/empty/duplicate/Access BOOLEAN metadata collapsed:", fixtures)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListSoilSuggestions(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled reference read accepted:", err)
	}
	if _, err := conn.ExecContext(context.Background(), `DELETE FROM USysTableOfLists WHERE ListName='vonPost'`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ListSoilSuggestions(context.Background(), state.ContextID); err == nil {
		t.Fatal("missing source suggestion group silently accepted")
	}
	for file, original := range seals {
		current, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("reference read changed family file:", file, err)
		}
	}
}
