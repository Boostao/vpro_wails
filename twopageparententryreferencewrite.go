package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Mandatory membership supplements, but never substitutes for, complete entry approval.
func (s *ContextService) writeTwoPageEntryWithRequiredReferences(ctx context.Context, contextID, plot, form string,
	original *siviParentProjection, edits []siviParentScalarEdit, masterAllowed bool,
	readers siviParentSharedReferenceReaders, approve twoPageEntryApproval) (*siviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("complete-entry mandatory references require separate full reference and role approval")
	}
	references := &SIVIParentSharedService{references: readers}
	lifecycle := twoPageEntryLifecycle{
		prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
			owner := plots.projects.sqlite
			if err := profileOwnedFiles(owner); err != nil {
				return nil, err
			}
			path, present := owner.attachments["VLists"]
			if !present || path == "" {
				return nil, errors.New("complete-entry required references need the configured owned VLists family")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS two_page_entry_refs`, sqliteFileURI(path, "ro")); err != nil {
				return nil, fmt.Errorf("complete-entry reference attachment: %w", err)
			}
			return func() {}, nil
		},
		verify: func(plots *PlotService, _ *sql.Tx) error {
			return profileOwnedFiles(plots.projects.sqlite)
		},
	}
	return s.writeTwoPageEntryWithLifecycle(ctx, contextID, plot, form, original, edits, masterAllowed,
		func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
			read := func(ctx context.Context, tx *sql.Tx, field siviParentSharedReferencePolicy) (SIVIParentSharedReference, error) {
				return references.readReference(ctx, tx, "two_page_entry_refs", field, ProjectMetadataCell{Storage: "null"})
			}
			if err := approveTwoPageEntryRequiredReferences(ctx, tx, observed, assignments, read); err != nil {
				return err
			}
			return approve(tx, observed, assignments)
		}, lifecycle)
}
