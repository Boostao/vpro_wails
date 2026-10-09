package main

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

func TestSIVIParentPhysicalSchemaPreflight(t *testing.T) {
	for _, table := range []string{"Sample_Env", "Sample_Admin"} {
		for _, column := range []string{"rowid", "_rowid_", "oid", "ExtraGenerated"} {
			t.Run(table+"/"+column, func(t *testing.T) {
				service, state := contextServiceFixture(t)
				original, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				actions := siviActionProposalEdits(original)
				db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(); err != nil {
						t.Error(err)
					}
				})
				if _, err := db.Exec("ALTER TABLE " + quoteHeaderIdentifier(table) + " ADD COLUMN " +
					quoteHeaderIdentifier(column) + " INTEGER GENERATED ALWAYS AS (99) VIRTUAL"); err != nil {
					t.Fatal(err)
				}
				files := databaseBytes(t, service.projects.sqlite.attachments)
				if got, err := service.readSIVIParent(context.Background(), state.ContextID, "108050"); err == nil || got != nil {
					t.Fatalf("generated/hidden/shadowed schema returned parent: result=%t error=%v", got != nil, err)
				}
				if got, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, nil, actions); err == nil || got != nil {
					t.Fatalf("generated schema returned action proposal: result=%t error=%v", got != nil, err)
				}
				assertProfileSUFiles(t, service, files)
				if _, err := db.Exec("ALTER TABLE " + quoteHeaderIdentifier(table) + " DROP COLUMN " + quoteHeaderIdentifier(column)); err != nil {
					t.Fatal(err)
				}
				retry, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(retry, original) {
					t.Fatal("physical schema restoration discarded attachments or changed originals", retry, original, err)
				}
				files = databaseBytes(t, service.projects.sqlite.attachments)
				proposal, err := service.prepareSIVIParentActionProposal(context.Background(), state.ContextID, "108050", nil, nil, nil, nil, actions)
				if err != nil || proposal == nil || !reflect.DeepEqual(proposal.Original, original) {
					t.Fatal("restored schema could not retry exact source planning", proposal, err)
				}
				assertProfileSUFiles(t, service, files)
			})
		}
	}
}
