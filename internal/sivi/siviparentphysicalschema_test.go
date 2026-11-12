package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
				service, state := vprotest.ContextServiceFixture(t)
				original, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				actions := sftest.SiviActionProposalEdits(original)
				db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(); err != nil {
						t.Error(err)
					}
				})
				if _, err := db.Exec("ALTER TABLE " + vprocore.QuoteHeaderIdentifier(table) + " ADD COLUMN " +
					vprocore.QuoteHeaderIdentifier(column) + " INTEGER GENERATED ALWAYS AS (99) VIRTUAL"); err != nil {
					t.Fatal(err)
				}
				files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				if got, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050"); err == nil || got != nil {
					t.Fatalf("generated/hidden/shadowed schema returned parent: result=%t error=%v", got != nil, err)
				}
				if got, err := sourceform.PrepareSIVIParentActionProposal(service, context.Background(), state.ContextID, "108050", nil, nil, nil, nil, actions); err == nil || got != nil {
					t.Fatalf("generated schema returned action proposal: result=%t error=%v", got != nil, err)
				}
				vprotest.AssertProfileSUFiles(t, service, files)
				if _, err := db.Exec("ALTER TABLE " + vprocore.QuoteHeaderIdentifier(table) + " DROP COLUMN " + vprocore.QuoteHeaderIdentifier(column)); err != nil {
					t.Fatal(err)
				}
				retry, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
				if err != nil || !reflect.DeepEqual(retry, original) {
					t.Fatal("physical schema restoration discarded attachments or changed originals", retry, original, err)
				}
				files = vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				proposal, err := sourceform.PrepareSIVIParentActionProposal(service, context.Background(), state.ContextID, "108050", nil, nil, nil, nil, actions)
				if err != nil || proposal == nil || !reflect.DeepEqual(proposal.Original, original) {
					t.Fatal("restored schema could not retry exact source planning", proposal, err)
				}
				vprotest.AssertProfileSUFiles(t, service, files)
			})
		}
	}
}
