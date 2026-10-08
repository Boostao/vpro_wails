package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
)

func snapshotGoogleEarthKMLPreparation(expected ownedGoogleEarthKMLPreparation) ownedGoogleEarthKMLPreparation {
	rows := slices.Clone(expected.Source.Report.Rows)
	for i, row := range rows {
		row.PlotNumber = cloneSiteUnitCell(row.PlotNumber)
		row.StoredLongitude = cloneSiteUnitCell(row.StoredLongitude)
		row.Longitude = cloneSiteUnitCell(row.Longitude)
		row.Latitude = cloneSiteUnitCell(row.Latitude)
		row.Description = cloneSiteUnitCell(row.Description)
		rows[i] = row
	}
	expected.Source.Report.Rows = rows
	expected.KML.Bytes = bytes.Clone(expected.KML.Bytes)
	return expected
}

func (s *ContextService) readGoogleEarthKMLPublicationReview(ctx context.Context, contextID, descriptionField, title string) (*ownedGoogleEarthKMLPreparation, error) {
	if err := validateGoogleEarthXMLText(title); err != nil {
		return nil, fmt.Errorf("Google Earth document name: %w", err)
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*ownedGoogleEarthKMLPreparation, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
			return readGoogleEarthKMLPreparationSnapshot(ctx, contextID, descriptionField, title, owner, tx)
		})
	})
}

type googleEarthKMLOwnedPublicationHooks struct {
	observe     func(string) error
	snapshot    publicationReadSnapshotHooks
	publication artifactPublicationHooks
}

func (s *ContextService) publishOwnedGoogleEarthKML(ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string) (artifactPublication, error) {
	return s.publishOwnedGoogleEarthKMLWithHooks(ctx, contextID, expected, requested, googleEarthKMLOwnedPublicationHooks{})
}

func (s *ContextService) publishOwnedGoogleEarthKMLWithHooks(ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string, hooks googleEarthKMLOwnedPublicationHooks) (artifactPublication, error) {
	expected = snapshotGoogleEarthKMLPreparation(expected)
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (result artifactPublication, resultErr error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return result, err
		}
		defer owner.mu.Unlock()
		defer func() {
			if result.Published && resultErr != nil {
				resultErr = fmt.Errorf("owned Google Earth KML published; do not replay publication: %w", resultErr)
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
		if expected.KML.ContextID != contextID || expected.KML.Project != owner.selection.Project ||
			expected.KML.ProjectPath != owner.selection.ProjectPath ||
			expected.KML.SU != owner.selection.SU || expected.KML.SUPath != owner.selection.SUPath {
			return result, errors.New("owned Google Earth KML context/project/SU/path differs; reload source review")
		}
		validate := func() error {
			actual, err := withPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
				return readGoogleEarthKMLPreparationSnapshot(ctx, contextID, expected.KML.DescriptionField, expected.KML.Title, owner, tx)
			})
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(*actual, expected) {
				return errors.New("owned Google Earth KML raw source/physical rows/scope/title/bytes differs; reload source review")
			}
			return nil
		}
		if err := validate(); err != nil {
			return result, err
		}
		if err := observe("validated"); err != nil {
			return result, err
		}
		points, err := planGoogleEarthKMLPoints(ctx, expected.Source.Report.Rows)
		if err != nil {
			return result, err
		}
		result, resultErr = publishGoogleEarthKMLChecked(ctx, requested, expected.KML.Title, points, func() error {
			if err := observe("precommit"); err != nil {
				return err
			}
			return validate()
		}, hooks.publication)
		if result.Published {
			resultErr = errors.Join(resultErr, observe("published"), validate())
		}
		return result, resultErr
	})
}
