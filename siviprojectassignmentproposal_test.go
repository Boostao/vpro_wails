package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSIVIProjectAssignmentOwnedSingleSnapshotAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "external"}[external], func(t *testing.T) {
			service, state := contextServiceFixture(t)
			if external {
				path := filepath.Join(t.TempDir(), "external # assignment proposal.db")
				data, err := os.ReadFile(state.ProjectPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				selection := contextSelection(state)
				selection.ProjectPath = path
				state, err = service.SwitchContext(state.ContextID, selection)
				if err != nil {
					t.Fatal(err)
				}
			}
			mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET ProjectID='old' WHERE PlotNumber='108050';
				DELETE FROM Sample_Metadata; INSERT INTO Sample_Metadata(ID,ProjectID,ProjectTitle) VALUES(1,'PROJECT',''),(2,'PROJECT',NULL)`)
			mutateContextFixture(t, service.projects.supportPaths["VMetaData"], `DELETE FROM ProjectMetadata;
				INSERT INTO ProjectMetadata(ProjectID,ProjectTitle) VALUES('MASTER','  literal title  '),(NULL,NULL)`)
			for _, source := range []int{1, 2} {
				if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": source}); err != nil {
					t.Fatal(err)
				}
				before := databaseBytes(t, service.projects.sqlite.attachments)
				config, err := os.ReadFile(service.projects.preferences.path)
				if err != nil {
					t.Fatal(err)
				}
				original, err := service.readSIVIParent(context.Background(), state.ContextID, "108050")
				if err != nil {
					t.Fatal(err)
				}
				choices, err := service.readSIVIProjectChoices(context.Background(), state.ContextID)
				if err != nil {
					t.Fatal(err)
				}
				metadata := choices.Choices.Rows[0]
				if source == 1 {
					metadata = choices.Choices.Rows[1]
				}
				var expected ProjectMetadataCell
				for _, binding := range original.Bindings {
					if binding.Binding == "ProjectID" {
						expected = original.Rows[0].Env.Cells[binding.Column]
					}
				}
				selection := siviProjectSelection{ContextID: state.ContextID, ControlID: "form:frmSIVIsite/ProjectID",
					Table: original.EnvTable, RowID: original.Rows[0].Env.RowID, Expected: expected,
					SourceOption: source, MetadataAlias: choices.Alias, MetadataTable: choices.Table, MetadataOriginal: metadata}
				selection.MetadataColumns = append([]ProjectMetadataColumn{}, choices.Choices.Columns...)
				proposal, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", selection)
				if err != nil || proposal == nil || !reflect.DeepEqual(proposal.Original, original) ||
					len(proposal.Plan.Assignments) != 1 || !reflect.DeepEqual(proposal.Plan.MetadataOriginal, metadata) {
					t.Fatal("owned parent/source/provenance preview differs", proposal, err)
				}
				retry, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", selection)
				if err != nil || !reflect.DeepEqual(retry, proposal) {
					t.Fatal("stable proposal changed without writes", retry, err)
				}
				*proposal.Plan.MetadataOriginal.Cells[0].Text = "caller"
				proposal.Original.ContextID = "caller"
				again, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", selection)
				if err != nil || !reflect.DeepEqual(again, retry) {
					t.Fatal("caller changed proposal ownership", again, err)
				}
				if got, err := service.prepareSIVIProjectAssignment(context.Background(), "stale", "108050", selection); err == nil || got != nil {
					t.Fatal("stale context returned an assignment proposal", got, err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if got, err := service.prepareSIVIProjectAssignment(ctx, state.ContextID, "108050", selection); !errors.Is(err, context.Canceled) || got != nil {
					t.Fatal("cancelled proposal returned assignments", got, err)
				}
				changed := selection
				changed.MetadataOriginal = ProjectMetadataRow{RowID: metadata.RowID,
					Cells: []ProjectMetadataCell{metadata.Cells[0], metadataText("changed title")}}
				if got, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", changed); err == nil || got != nil {
					t.Fatal("title/source drift returned a proposal", got, err)
				}
				assertProfileSUFiles(t, service, before)
				after, err := os.ReadFile(service.projects.preferences.path)
				if err != nil || !reflect.DeepEqual(config, after) {
					t.Fatal("read-only assignment proposal changed config", err)
				}
				if err := service.projects.preferences.update("Current", map[string]any{"ProjectIdSource": 3 - source}); err != nil {
					t.Fatal(err)
				}
				if got, err := service.prepareSIVIProjectAssignment(context.Background(), state.ContextID, "108050", selection); err == nil || got != nil {
					t.Fatal("independently changed preference returned a proposal", got, err)
				}
			}
		})
	}
}
