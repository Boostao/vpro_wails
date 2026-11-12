package spatial

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
		row.PlotNumber = vprocore.CloneSiteUnitCell(row.PlotNumber)
		row.StoredLongitude = vprocore.CloneSiteUnitCell(row.StoredLongitude)
		row.Longitude = vprocore.CloneSiteUnitCell(row.Longitude)
		row.Latitude = vprocore.CloneSiteUnitCell(row.Latitude)
		row.Description = vprocore.CloneSiteUnitCell(row.Description)
		rows[i] = row
	}
	expected.Source.Report.Rows = rows
	expected.KML.Bytes = bytes.Clone(expected.KML.Bytes)
	return expected
}

func ReadGoogleEarthKMLPublicationReview(s *vprocore.ContextService, ctx context.Context, contextID, descriptionField, title string) (*ownedGoogleEarthKMLPreparation, error) {
	if err := validateGoogleEarthXMLText(title); err != nil {
		return nil, fmt.Errorf("Google Earth document name: %w", err)
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*ownedGoogleEarthKMLPreparation, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
			return readGoogleEarthKMLPreparationSnapshot(ctx, contextID, descriptionField, title, owner, tx)
		})
	})
}

type googleEarthKMLOwnedPublicationHooks struct {
	observe     func(string) error
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func PublishOwnedGoogleEarthKML(s *vprocore.ContextService, ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string) (vprocore.ArtifactPublication, error) {
	return PublishOwnedGoogleEarthKMLWithHooks(s, ctx, contextID, expected, requested, googleEarthKMLOwnedPublicationHooks{})
}

func PublishOwnedGoogleEarthKMLWithHooks(s *vprocore.ContextService, ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string, hooks googleEarthKMLOwnedPublicationHooks) (vprocore.ArtifactPublication, error) {
	expected = snapshotGoogleEarthKMLPreparation(expected)
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (result vprocore.ArtifactPublication, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return result, err
		}
		defer owner.Mu.Unlock()
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
		if expected.KML.ContextID != contextID || expected.KML.Project != owner.Selection.Project ||
			expected.KML.ProjectPath != owner.Selection.ProjectPath ||
			expected.KML.SU != owner.Selection.SU || expected.KML.SUPath != owner.Selection.SUPath {
			return result, errors.New("owned Google Earth KML context/project/SU/path differs; reload source review")
		}
		validate := func() error {
			actual, err := vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
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
