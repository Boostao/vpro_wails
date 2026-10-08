package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
)

// Private preparation only: no desktop approval, feature gate or export API.
func (s *ContextService) publishOwnedProjectTableCSV(ctx context.Context, contextID string, expected ownedTableCSVReview, requested string) (tableCSVPublication, error) {
	return s.publishOwnedProjectTableCSVWithHooks(ctx, contextID, expected, requested, tableCSVOwnedPublicationHooks{})
}

type tableCSVOwnedPublicationHooks struct {
	observe      func(phase string) error
	commitRead   func(*sql.Tx) error
	rollbackRead func(*sql.Tx) error
	publication  tableCSVPublicationHooks
}

func (s *ContextService) publishOwnedProjectTableCSVWithHooks(ctx context.Context, contextID string, expected ownedTableCSVReview, requested string, hooks tableCSVOwnedPublicationHooks) (tableCSVPublication, error) {
	return s.publishOwnedProjectTableCSVUsing(ctx, contextID, expected, requested, hooks,
		func(ctx context.Context, requested string, review ownedTableCSVReview, validate func() error, hooks tableCSVPublicationHooks) (tableCSVPublication, error) {
			return publishTableCSVBundleChecked(ctx, requested, review.Document, validate, hooks)
		})
}

func (s *ContextService) publishOwnedProjectTableCSVArchive(ctx context.Context, contextID string, expected ownedTableCSVReview, requested string) (tableCSVPublication, error) {
	return s.publishOwnedProjectTableCSVArchiveWithHooks(ctx, contextID, expected, requested, tableCSVOwnedPublicationHooks{})
}

func (s *ContextService) publishOwnedProjectTableCSVArchiveWithHooks(ctx context.Context, contextID string, expected ownedTableCSVReview, requested string, hooks tableCSVOwnedPublicationHooks) (tableCSVPublication, error) {
	return s.publishOwnedProjectTableCSVUsing(ctx, contextID, expected, requested, hooks, publishOwnedTableCSVArchiveChecked)
}

func (s *ContextService) publishOwnedProjectTableCSVUsing(ctx context.Context, contextID string, expected ownedTableCSVReview, requested string, hooks tableCSVOwnedPublicationHooks,
	publish func(context.Context, string, ownedTableCSVReview, func() error, tableCSVPublicationHooks) (tableCSVPublication, error)) (tableCSVPublication, error) {
	// Detach every nested slice and tagged-value pointer before any lease,
	// cancellation or fault callback can execute caller code.
	expected.Document = snapshotTableCSVBundleDocument(expected.Document)
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result tableCSVPublication, resultErr error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return result, err
		}
		// Serialize owner-mutex writers and new project-pool borrows. Previously
		// borrowed or external connections are not excluded by this mutex.
		defer owner.mu.Unlock()
		defer func() {
			if result.Published && resultErr != nil {
				resultErr = fmt.Errorf("owned table CSV bundle published; do not replay publication: %w", resultErr)
			}
		}()
		observe := func(phase string) error {
			if hooks.observe != nil {
				return hooks.observe(phase)
			}
			return nil
		}
		if err := observe("snapshot"); err != nil {
			return result, err
		}
		if expected.ContextID != contextID || expected.Project != owner.selection.Project ||
			expected.ProjectPath != owner.selection.ProjectPath {
			return result, errors.New("owned table CSV publication context/project/path differs; reload source review")
		}
		validate := func() error {
			actual, err := readOwnedTableCSVPublicationSnapshot(ctx, owner, contextID, expected.Document.Manifest.Table, hooks)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("owned table CSV publication source schema/rows/values/Description metadata differs; reload source review")
			}
			return nil
		}
		if err := validate(); err != nil {
			return result, err
		}
		if err := observe("validated"); err != nil {
			return result, err
		}
		result, resultErr = publish(ctx, requested, expected, func() error {
			if err := observe("precommit"); err != nil {
				return err
			}
			// A new transaction, not another query in the first frozen snapshot.
			return validate()
		}, hooks.publication)
		if result.Published {
			resultErr = errors.Join(resultErr, observe("published"), validate())
		}
		return result, resultErr
	})
}

// Never publish inside a generic snapshot callback: read cleanup may zero its
// result. Here each fresh snapshot is fully closed before the file commit.
func readOwnedTableCSVPublicationSnapshot(ctx context.Context, owner *sqliteContext, contextID, table string, hooks tableCSVOwnedPublicationHooks) (result ownedTableCSVReview, resultErr error) {
	return withPublicationReadSnapshot(ctx, owner, publicationReadSnapshotHooks{
		commitRead: hooks.commitRead, rollbackRead: hooks.rollbackRead,
	}, func(tx *sql.Tx) (ownedTableCSVReview, error) {
		return readOwnedProjectTableCSV(ctx, owner, tx, contextID, table)
	})
}
