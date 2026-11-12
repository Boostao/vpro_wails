package main

import (
	"context"
	"errors"

	spatialpkg "github.com/boostao/vpro-wails/internal/spatial"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type PMTilesArchiveStatus = spatialpkg.PMTilesArchiveStatus
type PMTilesManifest = spatialpkg.PMTilesManifest
type NAD83UTMProjection = spatialpkg.NAD83UTMProjection
type NAD83LatLonProjection = spatialpkg.NAD83LatLonProjection
type PlotGeodesyInput = spatialpkg.PlotGeodesyInput
type PlotGeodesyEvaluation = spatialpkg.PlotGeodesyEvaluation
type SpatialAttributeSuggestion = spatialpkg.SpatialAttributeSuggestion
type SpatialPlotQARow = spatialpkg.SpatialPlotQARow
type SpatialBatchAuditReport = spatialpkg.SpatialBatchAuditReport
type ApplySpatialAttributesRequest = spatialpkg.ApplySpatialAttributesRequest
type ApplySpatialAttributesResult = spatialpkg.ApplySpatialAttributesResult
type SpatialPlotRecord = spatialpkg.SpatialPlotRecord
type SpatialGeoArrowColumns = spatialpkg.SpatialGeoArrowColumns
type SpatialWorkbenchPayload = spatialpkg.SpatialWorkbenchPayload
type SpatialAOISelectionRequest = spatialpkg.SpatialAOISelectionRequest
type SpatialAOIMatchedPlot = spatialpkg.SpatialAOIMatchedPlot
type SpatialAOISelectionResult = spatialpkg.SpatialAOISelectionResult
type AssignSpatialAOIToSURequest = spatialpkg.AssignSpatialAOIToSURequest
type AssignSpatialAOIToSUResult = spatialpkg.AssignSpatialAOIToSUResult

var NewPMTilesAssetHandler = spatialpkg.NewPMTilesAssetHandler
var NewPMTilesAttributor = spatialpkg.NewPMTilesAttributor

type SpatialService struct {
	contexts               *ContextService
	coordinates            *CoordinateService
	googleEarthReview      *GoogleEarthReviewService
	googleEarthKML         *GoogleEarthKMLService
	googleEarthKMLExport   *GoogleEarthKMLExportService
	googleEarthPreferences *GoogleEarthPreferencesService
	twoPageGoogleEarth     *TwoPageGoogleEarthService
	dataDir                string
	extraDirs              []string
	attributor             *spatialpkg.PMTilesAttributor
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

func (s *SpatialService) GetPMTilesManifest(ctx context.Context) (PMTilesManifest, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return PMTilesManifest{}, err
		}
	}
	if s == nil {
		return spatialpkg.BuildPMTilesManifest("", spatialpkg.DefaultSpatialAssetsDir), nil
	}
	return spatialpkg.BuildPMTilesManifest(s.dataDir, s.extraDirs...), nil
}

func (s *SpatialService) ProjectNAD83LatLonToUTM(ctx context.Context, lat, lon float64) (NAD83UTMProjection, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return NAD83UTMProjection{}, err
		}
	}
	normLat, normLon, _, _ := spatialpkg.NormalizeBCWesternCoordinates(lat, lon)
	z, e, n, err := spatialpkg.ProjectNAD83LatLonToUTM(normLat, normLon)
	if err != nil {
		return NAD83UTMProjection{}, err
	}
	return NAD83UTMProjection{
		Zone:        z,
		Easting:     e,
		Northing:    n,
		NtsMapSheet: spatialpkg.ComputeBCGSMapSheet(normLat, normLon),
	}, nil
}

func (s *SpatialService) ProjectNAD83UTMToLatLon(ctx context.Context, zone int, easting, northing float64) (NAD83LatLonProjection, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return NAD83LatLonProjection{}, err
		}
	}
	lat, lon, err := spatialpkg.ProjectNAD83UTMToLatLon(zone, easting, northing)
	if err != nil {
		return NAD83LatLonProjection{}, err
	}
	return NAD83LatLonProjection{
		Latitude:    lat,
		Longitude:   lon,
		NtsMapSheet: spatialpkg.ComputeBCGSMapSheet(lat, lon),
	}, nil
}

func (s *SpatialService) ComputeBCGSMapSheet(ctx context.Context, lat, lon float64) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	return spatialpkg.ComputeBCGSMapSheet(lat, lon), nil
}

func (s *SpatialService) EvaluatePlotGeodesy(ctx context.Context, input PlotGeodesyInput) (PlotGeodesyEvaluation, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return PlotGeodesyEvaluation{}, err
		}
	}
	return spatialpkg.EvaluatePlotGeodesy(input), nil
}

func (s *SpatialService) SuggestSpatialAttributes(ctx context.Context, lat, lon float64) (*SpatialAttributeSuggestion, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if s != nil && s.attributor != nil {
		return s.attributor.SuggestSpatialAttributes(lat, lon)
	}
	return spatialpkg.SuggestSpatialAttributes(lat, lon)
}

func (s *SpatialService) BatchAuditSpatialQA(ctx context.Context, contextID string) (*SpatialBatchAuditReport, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.BatchAuditSpatialQA(ctx, unwrapContextService(s.contexts), contextID, s.attributor)
}

func (s *SpatialService) ApplySpatialAttributes(ctx context.Context, contextID string, request ApplySpatialAttributesRequest) (*ApplySpatialAttributesResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.ApplySpatialAttributes(ctx, unwrapContextService(s.contexts), contextID, request, s.attributor)
}

func (s *SpatialService) GetSpatialWorkbenchPayload(ctx context.Context, contextID string) (*SpatialWorkbenchPayload, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.GetSpatialWorkbenchPayload(ctx, unwrapContextService(s.contexts), contextID, s.dataDir, s.extraDirs, s.attributor)
}

func (s *SpatialService) GetSpatialPlotsFlatGeobuf(ctx context.Context, contextID string) ([]byte, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.GetSpatialPlotsFlatGeobuf(ctx, unwrapContextService(s.contexts), contextID, s.attributor)
}

func (s *SpatialService) SelectPlotsInPolygonAOI(ctx context.Context, contextID string, request SpatialAOISelectionRequest) (*SpatialAOISelectionResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.SelectPlotsInPolygonAOI(ctx, unwrapContextService(s.contexts), contextID, request, s.attributor)
}

func (s *SpatialService) AssignSpatialAOIToSU(ctx context.Context, contextID string, request AssignSpatialAOIToSURequest) (*AssignSpatialAOIToSUResult, error) {
	if s == nil || s.contexts == nil {
		return nil, errors.New("spatial context is unavailable")
	}
	return spatialpkg.AssignSpatialAOIToSU(ctx, unwrapContextService(s.contexts), contextID, request, s.attributor)
}
