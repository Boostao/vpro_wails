package main

import (
	"context"
	"errors"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type SpatialService struct {
	contexts               *ContextService
	coordinates            *CoordinateService
	googleEarthReview      *GoogleEarthReviewService
	googleEarthKML         *GoogleEarthKMLService
	googleEarthKMLExport   *GoogleEarthKMLExportService
	googleEarthPreferences *GoogleEarthPreferencesService
	twoPageGoogleEarth     *TwoPageGoogleEarthService
}

func (s *SpatialService) GetCoordinateMode() (CoordinateMode, error) {
	if s == nil || s.coordinates == nil {
		return "", errors.New("coordinate service is unavailable")
	}
	m, err := s.coordinates.GetCoordinateMode()
	return CoordinateMode(m), err
}

func (s *SpatialService) SetCoordinateMode(mode CoordinateMode) error {
	if s == nil || s.coordinates == nil {
		return errors.New("coordinate service is unavailable")
	}
	return s.coordinates.SetCoordinateMode(vprocorepkg.CoordinateMode(mode))
}

func (s *SpatialService) DecomposeCoordinate(value *float64, mode CoordinateMode) (CoordinateParts, error) {
	if s == nil || s.coordinates == nil {
		return CoordinateParts{}, errors.New("coordinate service is unavailable")
	}
	return s.coordinates.DecomposeCoordinate(value, vprocorepkg.CoordinateMode(mode))
}

func (s *SpatialService) ConvertCoordinate(axis string, mode CoordinateMode, parts CoordinateParts) (*float64, error) {
	if s == nil || s.coordinates == nil {
		return nil, errors.New("coordinate service is unavailable")
	}
	return s.coordinates.ConvertCoordinate(axis, vprocorepkg.CoordinateMode(mode), parts)
}

func (s *SpatialService) GetGoogleEarthDescriptionFields(ctx context.Context, contextID string) (*GoogleEarthDescriptionFields, error) {
	if s == nil || s.googleEarthReview == nil {
		return nil, errors.New("Google Earth review service is unavailable")
	}
	return s.googleEarthReview.GetGoogleEarthDescriptionFields(ctx, contextID)
}

func (s *SpatialService) GetGoogleEarthReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthReview, error) {
	if s == nil || s.googleEarthReview == nil {
		return nil, errors.New("Google Earth review service is unavailable")
	}
	return s.googleEarthReview.GetGoogleEarthReview(ctx, contextID, requestJSON)
}

func (s *SpatialService) GetGoogleEarthKMLReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLReview, error) {
	if s == nil || s.googleEarthKML == nil {
		return nil, errors.New("Google Earth KML service is unavailable")
	}
	return s.googleEarthKML.GetGoogleEarthKMLReview(ctx, contextID, requestJSON)
}

func (s *SpatialService) GetGoogleEarthKMLExportReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLExportReview, error) {
	if s == nil || s.googleEarthKMLExport == nil {
		return nil, errors.New("Google Earth KML export service is unavailable")
	}
	return s.googleEarthKMLExport.GetGoogleEarthKMLExportReview(ctx, contextID, requestJSON)
}

func (s *SpatialService) ExportReviewedGoogleEarthKML(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLExportOutcome, error) {
	if s == nil || s.googleEarthKMLExport == nil {
		return nil, errors.New("Google Earth KML export service is unavailable")
	}
	return s.googleEarthKMLExport.ExportReviewedGoogleEarthKML(ctx, contextID, requestJSON)
}

func (s *SpatialService) GetGoogleEarthPreferences(ctx context.Context, contextID string) (*GoogleEarthPreferencesReview, error) {
	if s == nil || s.googleEarthPreferences == nil {
		return nil, errors.New("Google Earth preferences service is unavailable")
	}
	return s.googleEarthPreferences.GetGoogleEarthPreferences(ctx, contextID)
}

func (s *SpatialService) SaveGoogleEarthPreferences(ctx context.Context, contextID, requestJSON string) *GoogleEarthPreferencesOutcome {
	if s == nil || s.googleEarthPreferences == nil {
		return &GoogleEarthPreferencesOutcome{ContextID: contextID, ErrorMessage: "Google Earth preferences service is unavailable"}
	}
	return s.googleEarthPreferences.SaveGoogleEarthPreferences(ctx, contextID, requestJSON)
}

func (s *SpatialService) GetTwoPageGoogleEarthOptions(ctx context.Context, contextID, plot, form string) (*TwoPageGoogleEarthOptions, error) {
	if s == nil || s.twoPageGoogleEarth == nil {
		return nil, errors.New("two-page Google Earth service is unavailable")
	}
	return s.twoPageGoogleEarth.GetOptions(ctx, contextID, plot, form)
}

func (s *SpatialService) GetTwoPageGoogleEarthReview(ctx context.Context, contextID, plot, form, requestJSON string) (*TwoPageGoogleEarthReview, error) {
	if s == nil || s.twoPageGoogleEarth == nil {
		return nil, errors.New("two-page Google Earth service is unavailable")
	}
	return s.twoPageGoogleEarth.GetReview(ctx, contextID, plot, form, requestJSON)
}

func (s *SpatialService) GetPlotLocationReview(ctx context.Context, contextID string) (*PlotLocationReview, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return s.contexts.GetPlotLocationReview(ctx, contextID)
}
