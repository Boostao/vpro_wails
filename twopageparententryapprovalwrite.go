package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

func requireTwoPageEntryReferenceReadLock(ctx context.Context, tx *sql.Tx, alias string) error {
	var journal string
	if err := tx.QueryRowContext(ctx, `PRAGMA `+quoteHeaderIdentifier(alias)+`.journal_mode`).Scan(&journal); err != nil {
		return err
	}
	switch strings.ToLower(journal) {
	case "delete", "truncate", "persist":
		return nil
	default:
		return fmt.Errorf("complete-entry %s references require rollback-journal read locks through commit", alias)
	}
}

func (s *ContextService) writeTwoPageEntryWithOwnedReferences(ctx context.Context, contextID, plot, form string,
	original *siviParentProjection, edits []siviParentScalarEdit, masterAllowed bool,
	readers twoPageEntryReferenceReaders, expected twoPageEntryReferenceSelection,
	acknowledgements []twoPageEntryCodeAcknowledgement) (*siviParentWriteResult, error) {
	return s.writeTwoPageEntryWithOwnedReferencesAndProject(ctx, contextID, plot, form, original, edits,
		masterAllowed, readers, expected, acknowledgements, nil)
}

func (s *ContextService) writeTwoPageEntryWithOwnedReferencesAndProject(ctx context.Context, contextID, plot, form string,
	original *siviParentProjection, edits []siviParentScalarEdit, masterAllowed bool,
	readers twoPageEntryReferenceReaders, expected twoPageEntryReferenceSelection,
	acknowledgements []twoPageEntryCodeAcknowledgement, projectSelection *siviProjectSelection) (*siviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	provider, err := newTwoPageEntryReferenceProvider(form, readers)
	if err != nil {
		return nil, err
	}
	if expected.projectSource < 1 || expected.projectSource > 2 || expected.workingSource < 1 || expected.workingSource > 3 {
		return nil, errors.New("complete-entry writing requires the observed reference source selections")
	}
	if projectSelection != nil && projectSelection.SourceOption != expected.projectSource {
		return nil, errors.New("complete-entry ProjectID selection differs from the observed source preference")
	}
	var owner *sqliteContext
	var preferences *desktopConfig
	var evidence *siviProjectAssignmentHistory
	var projectPlan *siviProjectAssignmentPlan
	aliases := twoPageEntryReferenceAliases{"main", "two_page_entry_refs", ""}
	checkSources := func() error {
		values, err := preferences.readLocked()
		if err != nil {
			return err
		}
		observed, err := twoPageEntryReferenceSelectionFromConfig(values)
		if err != nil {
			return err
		}
		if observed != expected {
			return errors.New("complete-entry Project/Working source preferences changed; reload before writing")
		}
		return nil
	}
	lifecycle := twoPageEntryLifecycle{
		prepare: func(plots *PlotService, conn *sql.Conn) (func(), error) {
			if projectSelection != nil && plots.currentUser == "" {
				return nil, errors.New("complete-entry ProjectID assignment requires an explicit audit user")
			}
			owner, preferences = plots.projects.sqlite, plots.projects.preferences
			if err := profileOwnedFiles(owner); err != nil {
				return nil, err
			}
			if err := acquireMutexLease(ctx, &preferences.mu); err != nil {
				return nil, err
			}
			release := preferences.mu.Unlock
			if err := checkSources(); err != nil {
				release()
				return nil, err
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "two_page_entry_refs"`,
				sqliteFileURI(owner.attachments["VLists"], "ro")); err != nil {
				release()
				return nil, fmt.Errorf("complete-entry owned reference attachment: %w", err)
			}
			if projectSelection != nil && expected.projectSource == 2 {
				if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "VMetaData"`,
					sqliteFileURI(owner.attachments["VMetaData"], "ro")); err != nil {
					release()
					return nil, fmt.Errorf("complete-entry owned ProjectID metadata attachment: %w", err)
				}
			}
			aliases.su = sourceParentWriterSUAlias(owner)
			return release, nil
		},
		verify: func(_ *PlotService, tx *sql.Tx) error {
			if err := checkSources(); err != nil {
				return err
			}
			if evidence != nil {
				fresh, err := readSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, *projectSelection)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(evidence, fresh) {
					return errors.New("complete-entry ProjectID source metadata/schema changed inside the owned transaction")
				}
			}
			return profileOwnedFiles(owner)
		},
		decorate: func(event *siviParentHistory) {
			if projectPlan != nil && len(projectPlan.Assignments) == 1 {
				event.ProjectAssignment = evidence
			}
		},
	}
	return s.writeTwoPageEntryWithLifecycle(ctx, contextID, plot, form, original, edits, masterAllowed,
		func(tx *sql.Tx, observed *siviParentProjection, assignments []siviParentScalarAssignment) error {
			if err := checkSources(); err != nil {
				return err
			}
			if projectSelection != nil {
				var err error
				evidence, err = readSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, *projectSelection)
				if err != nil {
					return err
				}
				projectPlan, err = planTwoPageEntryProjectAssignment(ctx, observed, evidence.Choices, *projectSelection)
				if err != nil {
					return err
				}
			}
			listsRequired, suRequired := false, false
			for _, assignment := range assignments {
				field, reference := provider.fields[assignment.Column]
				if !reference || assignment.After.Storage == "null" {
					continue
				}
				listsRequired = listsRequired || field.reader == "family" || field.reader == "master-unit" ||
					(field.reader == "working-unit" && expected.workingSource == 2)
				suRequired = suRequired || (field.reader == "working-unit" && expected.workingSource == 3)
			}
			if listsRequired {
				if err := requireTwoPageEntryReferenceReadLock(ctx, tx, aliases.lists); err != nil {
					return err
				}
			}
			if suRequired && aliases.su != "" && aliases.su != "main" {
				if err := requireTwoPageEntryReferenceReadLock(ctx, tx, aliases.su); err != nil {
					return err
				}
			}
			return approveTwoPageEntryReferencesWithProject(ctx, tx, owner, provider, contextID, observed,
				assignments, acknowledgements, expected, aliases, projectPlan)
		}, lifecycle)
}
