package sivi

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
	"os"
)

func verifySIVICHARSReferenceOwner(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext) error {
	if tx == nil || owner == nil {
		return errors.New("CHARS reference ownership requires the current transaction and context")
	}
	for alias, role := range map[string]string{"main": "project", "chars_entry_refs": "VLists"} {
		var path string
		if err := tx.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name=?`, alias).Scan(&path); err != nil {
			return fmt.Errorf("CHARS %s reference attachment: %w", alias, err)
		}
		if path == "" || owner.AttachmentInfo[role] == nil {
			return fmt.Errorf("CHARS %s reference attachment has no owned %s identity", alias, role)
		}
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("CHARS %s reference identity: %w", alias, err)
		}
		info, statErr := file.Stat()
		if err := errors.Join(statErr, file.Close()); err != nil {
			return fmt.Errorf("CHARS %s reference identity capture: %w", alias, err)
		}
		if !os.SameFile(info, owner.AttachmentInfo[role]) {
			return fmt.Errorf("CHARS %s reference attachment changed its owned %s identity", alias, role)
		}
	}
	return ctx.Err()
}

func approveSIVICHARSRequiredReferences(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext,
	readers sourceform.SiviParentSharedReferenceReaders, assignments []sourceform.SiviParentScalarAssignment) error {
	if err := verifySIVICHARSReferenceOwner(ctx, tx, owner); err != nil {
		return err
	}
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return err
	}
	for _, field := range fields {
		if !field.Required {
			continue
		}
		for _, assignment := range assignments {
			if assignment.Column != field.Column || assignment.After.Storage == "null" {
				continue
			}
			if assignment.After.Storage != "text" || assignment.After.Text == nil {
				return fmt.Errorf("CHARS %s reference approval requires nullable TEXT", field.Column)
			}
			if field.Reader == "family" {
				if err := vprocore.RequireRollbackJournalReadLock(ctx, tx, "chars_entry_refs", "CHARS mandatory references"); err != nil {
					return err
				}
			}
			reference, err := readSIVICHARSParentReferenceAtAlias(ctx, tx, "chars_entry_refs", readers, field)
			if err != nil {
				return err
			}
			if !reference.Available || !reference.Required || reference.Column != field.Column || reference.ListName != field.List {
				return fmt.Errorf("CHARS %s requires its available exact source catalogue", field.Column)
			}
			membership := map[string]bool{}
			for _, choice := range reference.Choices {
				if choice.Selectable && choice.Code != nil && sqlutil.ValidateSiteCodeText(field.Column, choice.Code, field.Maximum) == nil {
					membership[*choice.Code] = true
				}
			}
			if err := vprocore.CanonicalItemError(field.Column, field.List, *assignment.After.Text, membership); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func WriteSIVICHARSParentOwnedEntry(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, readers sourceform.SiviParentSharedReferenceReaders) (*sourceform.SiviParentWriteResult, error) {
	return WriteSIVICHARSParentOwnedProposal(s, ctx, contextID, plot, original, edits, nil, readers)
}

func WriteSIVICHARSParentOwnedProposal(s *vprocore.ContextService, ctx context.Context, contextID, plot string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, actions []sourceform.SiviParentActionEdit,
	readers sourceform.SiviParentSharedReferenceReaders) (*sourceform.SiviParentWriteResult, error) {
	var owner *vprocore.SqliteContext
	lifecycle := sourceform.SourceParentEntryLifecycle{
		Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
			owner = plots.Projects.Sqlite
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return nil, err
			}
			path, present := owner.Attachments["VLists"]
			if !present || path == "" {
				return nil, errors.New("CHARS entry requires the configured owned VLists family")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "chars_entry_refs"`, sqlutil.SqliteFileURI(path, "ro")); err != nil {
				return nil, fmt.Errorf("CHARS owned reference attachment: %w", err)
			}
			return func() {}, nil
		},
		Verify: func(_ *vprocore.PlotService, tx *sql.Tx) error {
			if err := verifySIVICHARSReferenceOwner(ctx, tx, owner); err != nil {
				return err
			}
			return vprocore.ProfileOwnedFiles(owner)
		},
	}
	return WriteSIVICHARSParentProposal(s, ctx, contextID, plot, original, edits, actions,
		func(tx *sql.Tx, _ *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			return approveSIVICHARSRequiredReferences(ctx, tx, owner, readers, assignments)
		}, lifecycle)
}
