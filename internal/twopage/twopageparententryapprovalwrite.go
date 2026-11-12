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
	"reflect"
)

func requireTwoPageEntryReferenceReadLock(ctx context.Context, tx *sql.Tx, alias string) error {
	return vprocore.RequireRollbackJournalReadLock(ctx, tx, alias, "complete-entry "+alias+" references")
}

func WriteTwoPageEntryWithOwnedReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool,
	readers TwoPageEntryReferenceReaders, expected TwoPageEntryReferenceSelection,
	acknowledgements []TwoPageEntryCodeAcknowledgementInternal) (*sourceform.SiviParentWriteResult, error) {
	return WriteTwoPageEntryWithOwnedReferencesAndProject(s, ctx, contextID, plot, form, original, edits,
		masterAllowed, readers, expected, acknowledgements, nil)
}

func WriteTwoPageEntryWithOwnedReferencesAndProject(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string,
	original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool,
	readers TwoPageEntryReferenceReaders, expected TwoPageEntryReferenceSelection,
	acknowledgements []TwoPageEntryCodeAcknowledgementInternal, projectSelection *sourceform.SiviProjectSelection) (*sourceform.SiviParentWriteResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	provider, err := NewTwoPageEntryReferenceProvider(form, readers)
	if err != nil {
		return nil, err
	}
	if expected.ProjectSource < 1 || expected.ProjectSource > 2 || expected.WorkingSource < 1 || expected.WorkingSource > 3 {
		return nil, errors.New("complete-entry writing requires the observed reference source selections")
	}
	if projectSelection != nil && projectSelection.SourceOption != expected.ProjectSource {
		return nil, errors.New("complete-entry ProjectID selection differs from the observed source preference")
	}
	var owner *vprocore.SqliteContext
	var preferences *vprocore.DesktopConfig
	var evidence *sourceform.SiviProjectAssignmentHistory
	var projectPlan *sourceform.SiviProjectAssignmentPlan
	aliases := TwoPageEntryReferenceAliases{"main", "two_page_entry_refs", ""}
	checkSources := func() error {
		values, err := preferences.ReadLocked()
		if err != nil {
			return err
		}
		observed, err := TwoPageEntryReferenceSelectionFromConfig(values)
		if err != nil {
			return err
		}
		if observed != expected {
			return errors.New("complete-entry Project/Working source preferences changed; reload before writing")
		}
		return nil
	}
	lifecycle := TwoPageEntryLifecycle{
		Prepare: func(plots *vprocore.PlotService, conn *sql.Conn) (func(), error) {
			if projectSelection != nil && plots.CurrentUser == "" {
				return nil, errors.New("complete-entry ProjectID assignment requires an explicit audit user")
			}
			owner, preferences = plots.Projects.Sqlite, plots.Projects.Preferences
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return nil, err
			}
			if err := sqlutil.AcquireMutexLease(ctx, &preferences.Mu); err != nil {
				return nil, err
			}
			release := preferences.Mu.Unlock
			if err := checkSources(); err != nil {
				release()
				return nil, err
			}
			if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "two_page_entry_refs"`,
				sqlutil.SqliteFileURI(owner.Attachments["VLists"], "ro")); err != nil {
				release()
				return nil, fmt.Errorf("complete-entry owned reference attachment: %w", err)
			}
			if projectSelection != nil && expected.ProjectSource == 2 {
				if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "VMetaData"`,
					sqlutil.SqliteFileURI(owner.Attachments["VMetaData"], "ro")); err != nil {
					release()
					return nil, fmt.Errorf("complete-entry owned ProjectID metadata attachment: %w", err)
				}
			}
			aliases.Su = sourceform.SourceParentWriterSUAlias(owner)
			return release, nil
		},
		Verify: func(_ *vprocore.PlotService, tx *sql.Tx) error {
			if err := checkSources(); err != nil {
				return err
			}
			if evidence != nil {
				fresh, err := sourceform.ReadSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, *projectSelection)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(evidence, fresh) {
					return errors.New("complete-entry ProjectID source metadata/schema changed inside the owned transaction")
				}
			}
			return vprocore.ProfileOwnedFiles(owner)
		},
		Decorate: func(event *sourceform.SiviParentHistory) {
			if projectPlan != nil && len(projectPlan.Assignments) == 1 {
				event.ProjectAssignment = evidence
			}
		},
	}
	return WriteTwoPageEntryWithLifecycle(s, ctx, contextID, plot, form, original, edits, masterAllowed,
		func(tx *sql.Tx, observed *sourceform.SiviParentProjection, assignments []sourceform.SiviParentScalarAssignment) error {
			if err := checkSources(); err != nil {
				return err
			}
			if projectSelection != nil {
				var err error
				evidence, err = sourceform.ReadSIVIProjectAssignmentEvidence(ctx, owner, tx, contextID, *projectSelection)
				if err != nil {
					return err
				}
				projectPlan, err = PlanTwoPageEntryProjectAssignment(ctx, observed, evidence.Choices, *projectSelection)
				if err != nil {
					return err
				}
			}
			listsRequired, suRequired := false, false
			for _, assignment := range assignments {
				field, reference := provider.Fields[assignment.Column]
				if !reference || assignment.After.Storage == "null" {
					continue
				}
				listsRequired = listsRequired || field.Reader == "family" || field.Reader == "master-unit" ||
					(field.Reader == "working-unit" && expected.WorkingSource == 2)
				suRequired = suRequired || (field.Reader == "working-unit" && expected.WorkingSource == 3)
			}
			if listsRequired {
				if err := requireTwoPageEntryReferenceReadLock(ctx, tx, aliases.Lists); err != nil {
					return err
				}
			}
			if suRequired && aliases.Su != "" && aliases.Su != "main" {
				if err := requireTwoPageEntryReferenceReadLock(ctx, tx, aliases.Su); err != nil {
					return err
				}
			}
			return approveTwoPageEntryReferencesWithProject(ctx, tx, owner, provider, contextID, observed,
				assignments, acknowledgements, expected, aliases, projectPlan)
		}, lifecycle)
}
