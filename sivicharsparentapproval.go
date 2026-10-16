package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

func verifySIVICHARSReferenceOwner(ctx context.Context, tx *sql.Tx, owner *sqliteContext) error {
	if tx == nil || owner == nil {
		return errors.New("CHARS reference ownership requires the current transaction and context")
	}
	for alias, role := range map[string]string{"main": "project", "chars_entry_refs": "VLists"} {
		var path string
		if err := tx.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name=?`, alias).Scan(&path); err != nil {
			return fmt.Errorf("CHARS %s reference attachment: %w", alias, err)
		}
		if path == "" || owner.attachmentInfo[role] == nil {
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
		if !os.SameFile(info, owner.attachmentInfo[role]) {
			return fmt.Errorf("CHARS %s reference attachment changed its owned %s identity", alias, role)
		}
	}
	return ctx.Err()
}

func approveSIVICHARSRequiredReferences(ctx context.Context, tx *sql.Tx, owner *sqliteContext,
	readers siviParentSharedReferenceReaders, assignments []siviParentScalarAssignment) error {
	if err := verifySIVICHARSReferenceOwner(ctx, tx, owner); err != nil {
		return err
	}
	fields, err := siviCHARSParentReferenceFields()
	if err != nil {
		return err
	}
	for _, field := range fields {
		if !field.required {
			continue
		}
		for _, assignment := range assignments {
			if assignment.Column != field.column || assignment.After.Storage == "null" {
				continue
			}
			if assignment.After.Storage != "text" || assignment.After.Text == nil {
				return fmt.Errorf("CHARS %s reference approval requires nullable TEXT", field.column)
			}
			if field.reader == "family" {
				if err := requireRollbackJournalReadLock(ctx, tx, "chars_entry_refs", "CHARS mandatory references"); err != nil {
					return err
				}
			}
			reference, err := readSIVICHARSParentReferenceAtAlias(ctx, tx, "chars_entry_refs", readers, field)
			if err != nil {
				return err
			}
			if !reference.Available || !reference.Required || reference.Column != field.column || reference.ListName != field.list {
				return fmt.Errorf("CHARS %s requires its available exact source catalogue", field.column)
			}
			membership := map[string]bool{}
			for _, choice := range reference.Choices {
				if choice.Selectable && choice.Code != nil && validateSiteCodeText(field.column, choice.Code, field.maximum) == nil {
					membership[*choice.Code] = true
				}
			}
			if err := canonicalItemError(field.column, field.list, *assignment.After.Text, membership); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (s *ContextService) writeSIVICHARSParentOwnedEntry(ctx context.Context, contextID, plot string,
	original *siviParentProjection, edits []siviParentScalarEdit, readers siviParentSharedReferenceReaders) (*siviParentWriteResult, error) {
	return s.writeSIVICHARSParentOwnedProposal(ctx, contextID, plot, original, edits, nil, readers)
}

func (s *ContextService) writeSIVICHARSParentOwnedProposal(ctx context.Context, contextID, plot string,
	original *siviParentProjection, edits []siviParentScalarEdit, actions []siviParentActionEdit,
	readers siviParentSharedReferenceReaders) (*siviParentWriteResult, error) {
	var owner *sqliteContext
	lifecycle := sourceParentEntryLifecycle{
		prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
			owner = plots.projects.sqlite
			if err := profileOwnedFiles(owner); err != nil {
				return nil, err
			}
			path, present := owner.attachments["VLists"]
			if !present || path == "" {
				return nil, errors.New("CHARS entry requires the configured owned VLists family")
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "chars_entry_refs"`, sqliteFileURI(path, "ro")); err != nil {
				return nil, fmt.Errorf("CHARS owned reference attachment: %w", err)
			}
			return func() {}, nil
		},
		verify: func(_ *PlotService, tx *sql.Tx) error {
			if err := verifySIVICHARSReferenceOwner(ctx, tx, owner); err != nil {
				return err
			}
			return profileOwnedFiles(owner)
		},
	}
	return s.writeSIVICHARSParentProposal(ctx, contextID, plot, original, edits, actions,
		func(tx *sql.Tx, _ *siviParentProjection, assignments []siviParentScalarAssignment) error {
			return approveSIVICHARSRequiredReferences(ctx, tx, owner, readers, assignments)
		}, lifecycle)
}
