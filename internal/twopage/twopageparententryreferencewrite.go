package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Mandatory membership supplements, but never substitutes for, complete entry approval.
func WriteTwoPageEntryWithRequiredReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool,
	readers sourceform.SiviParentSharedReferenceReaders, approve TwoPageEntryApproval) (*sourceform.SiviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("complete-entry mandatory references require separate full reference and role approval")
	}
	references := &sourceform.SIVIParentSharedService{References: readers}
	lifecycle := TwoPageEntryLifecycle{
		Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
			owner := plots.Projects.Sqlite
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return nil, err
			}
			path, present := owner.Attachments["VLists"]
			if !present || path == "" {
				return nil, errors.New("complete-entry required references need the configured owned VLists family")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS two_page_entry_refs`, sqlutil.SqliteFileURI(path, "ro")); err != nil {
				return nil, fmt.Errorf("complete-entry reference attachment: %w", err)
			}
			return func() {}, nil
		},
		Verify: func(plots *vprocore.PlotService, _ *sql.Tx) error {
			return vprocore.ProfileOwnedFiles(plots.Projects.Sqlite)
		},
	}
	return WriteTwoPageEntryWithLifecycle(s, ctx, contextID, plot, form, original, edits, masterAllowed,
		func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			read := func(ctx context.Context, tx *sql.Tx, field sourceform.SiviParentSharedReferencePolicy) (sourceform.SIVIParentSharedReference, error) {
				return references.ReadReference(ctx, tx, "two_page_entry_refs", field, vprocore.ProjectMetadataCell{Storage: "null"})
			}
			if err := ApproveTwoPageEntryRequiredReferences(ctx, tx, observed, assignments, read); err != nil {
				return err
			}
			return approve(tx, observed, assignments)
		}, lifecycle)
}
