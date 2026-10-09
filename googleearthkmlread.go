package main

import (
	"context"
	"database/sql"
	"fmt"
)

type ownedGoogleEarthKML struct {
	ContextID, ProjectPath, SUPath string
	Project, SU, DescriptionField  string
	Title                          string
	PlacemarkCount                 int
	Bytes                          []byte
}

type ownedGoogleEarthKMLPreparation struct {
	Source ownedGoogleEarthLocations
	KML    ownedGoogleEarthKML
}

func readGoogleEarthKMLPreparationSnapshot(ctx context.Context, contextID, descriptionField, title string, owner *sqliteContext, tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
	source, err := readGoogleEarthLocationsSnapshot(ctx, contextID, descriptionField, owner, tx)
	if err != nil {
		return nil, err
	}
	points, err := planGoogleEarthKMLPoints(ctx, source.Report.Rows)
	if err != nil {
		return nil, err
	}
	data, err := prepareGoogleEarthKML(ctx, title, points)
	if err != nil {
		return nil, err
	}
	return &ownedGoogleEarthKMLPreparation{Source: *source, KML: ownedGoogleEarthKML{
		ContextID: source.ContextID, ProjectPath: source.ProjectPath,
		SUPath: source.SUPath, Project: source.Report.Project, SU: source.Report.SU,
		DescriptionField: source.Report.DescriptionField, Title: title,
		PlacemarkCount: len(points), Bytes: data,
	}}, nil
}

func (s *ContextService) readGoogleEarthKML(ctx context.Context, contextID, descriptionField, title string) (*ownedGoogleEarthKML, error) {
	if err := validateGoogleEarthXMLText(title); err != nil {
		return nil, fmt.Errorf("Google Earth document name: %w", err)
	}
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (*ownedGoogleEarthKML, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*ownedGoogleEarthKML, error) {
			preparation, err := readGoogleEarthKMLPreparationSnapshot(ctx, contextID, descriptionField, title, owner, tx)
			if err != nil {
				return nil, err
			}
			return &preparation.KML, nil
		})
	})
}
