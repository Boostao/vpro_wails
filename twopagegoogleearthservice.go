package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

const twoPageGoogleEarthReviewEnvironment = "VPRO_TWO_PAGE_GOOGLE_EARTH_REVIEW"

type TwoPageGoogleEarthReview struct {
	ContextID        string               `json:"contextId"`
	Project          string               `json:"project"`
	ProjectPath      string               `json:"projectPath"`
	Plot             string               `json:"plot"`
	Form             string               `json:"form"`
	DescriptionField string               `json:"descriptionField"`
	Title            string               `json:"title"`
	Original         ProjectMetadataTable `json:"original"`
	ByteCount        int                  `json:"byteCount"`
	KML              string               `json:"kml"`
}

type TwoPageGoogleEarthOptions struct {
	ContextID   string                      `json:"contextId"`
	Project     string                      `json:"project"`
	ProjectPath string                      `json:"projectPath"`
	Plot        string                      `json:"plot"`
	Form        string                      `json:"form"`
	Values      GoogleEarthPreferenceValues `json:"values"`
	Fields      []ProjectMetadataColumn     `json:"fields"`
}

type TwoPageGoogleEarthService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageGoogleEarthService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageGoogleEarthService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page Google Earth requires owned contexts and explicit feature lookup")
	}
	review, err := siviFeature(twoPageGoogleEarthReviewEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageGoogleEarthService{contexts: contexts, review: review}, nil
}

func (s *TwoPageGoogleEarthService) requireReview(ctx context.Context, plot, form string) error {
	if ctx == nil {
		return errors.New("two-page Google Earth requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.review {
		return errors.New("two-page Google Earth requires independently enabled saved-plot review")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return errors.New("two-page Google Earth requires an exact valid literal parent")
	}
	if form != "FS882-8x6XL" && form != "FS882-8x6XL-CHARS" {
		return fmt.Errorf("two-page Google Earth does not support source form %q", form)
	}
	return nil
}

func (s *TwoPageGoogleEarthService) GetOptions(ctx context.Context, contextID, plot, form string) (*TwoPageGoogleEarthOptions, error) {
	if err := s.requireReview(ctx, plot, form); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageGoogleEarthOptions, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageGoogleEarthOptions, error) {
			project, table := owner.selection.Project, owner.selection.Project+"_Env"
			if err := requireTwoPageChildReadScope(ctx, tx, project, table, plot, "Google Earth options"); err != nil {
				return nil, err
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page Google Earth options"); err != nil {
				return nil, err
			}
			fields, err := readSQLiteStorageColumns(ctx, tx, "project", table)
			if err != nil {
				return nil, err
			}
			values, err := plots.projects.preferences.readGoogleEarthPreferences(ctx)
			if err != nil {
				return nil, fmt.Errorf("two-page Google Earth saved options: %w", err)
			}
			if err := validateGoogleEarthReviewStrings(contextID, project, owner.selection.ProjectPath, values.PlaceName, values.DescriptionField); err != nil {
				return nil, err
			}
			return &TwoPageGoogleEarthOptions{ContextID: contextID, Project: project, ProjectPath: owner.selection.ProjectPath,
				Plot: plot, Form: form, Fields: fields,
				Values: GoogleEarthPreferenceValues{Title: values.PlaceName, DescriptionField: values.DescriptionField}}, nil
		})
	})
}

func (s *TwoPageGoogleEarthService) GetReview(ctx context.Context, contextID, plot, form, requestJSON string) (*TwoPageGoogleEarthReview, error) {
	if err := s.requireReview(ctx, plot, form); err != nil {
		return nil, err
	}
	var request GoogleEarthKMLRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthXMLText(request.Title); err != nil {
		return nil, fmt.Errorf("two-page Google Earth Place Name: %w", err)
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageGoogleEarthReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageGoogleEarthReview, error) {
			project, table := owner.selection.Project, owner.selection.Project+"_Env"
			if err := requireTwoPageChildReadScope(ctx, tx, project, table, plot, "Google Earth"); err != nil {
				return nil, err
			}
			if err := validateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page Google Earth"); err != nil {
				return nil, err
			}
			original, err := readSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "")
			if err != nil {
				return nil, fmt.Errorf("two-page Google Earth original: %w", err)
			}
			if len(original.Rows) != 1 {
				return nil, errors.New("two-page Google Earth requires one saved literal Env row")
			}
			columns, err := siteUnitTransferColumns(original, "PlotNumber", "Longitude", "Latitude", request.DescriptionField)
			if err != nil {
				return nil, fmt.Errorf("two-page Google Earth source schema: %w", err)
			}
			longitude, err := googleEarthKMLNumber(original.Rows[0].Cells[columns["Longitude"]])
			if err != nil {
				return nil, fmt.Errorf("two-page Google Earth stored Longitude: %w", err)
			}
			if math.Signbit(longitude) {
				return nil, errors.New("two-page Google Earth source minus-prefix requires nonnegative stored Longitude; no coordinate repaired")
			}
			// The source reads one current-project Env row, not SU-expanded locations.
			locations, err := planGoogleEarthLocations(ctx, project, "None", request.DescriptionField, original, ProjectMetadataTable{})
			if err != nil {
				return nil, err
			}
			if len(locations.Rows) != 1 {
				return nil, errors.New("two-page Google Earth requires saved non-NULL Longitude and Latitude")
			}
			points, err := planGoogleEarthKMLPoints(ctx, locations.Rows)
			if err != nil {
				return nil, err
			}
			data, err := prepareGoogleEarthKML(ctx, request.Title, points)
			if err != nil {
				return nil, err
			}
			if err := validateGoogleEarthReviewStrings(contextID, project, owner.selection.ProjectPath, plot, form); err != nil {
				return nil, err
			}
			return &TwoPageGoogleEarthReview{ContextID: contextID, Project: project, ProjectPath: owner.selection.ProjectPath,
				Plot: plot, Form: form, DescriptionField: request.DescriptionField, Title: request.Title,
				Original: original, ByteCount: len(data), KML: string(data)}, nil
		})
	})
}
