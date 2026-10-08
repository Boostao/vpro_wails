package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const googleEarthReviewFeatureEnvironment = "VPRO_GOOGLE_EARTH_REVIEW"

func googleEarthReviewFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(googleEarthReviewFeatureEnvironment, lookup)
}

type GoogleEarthReviewService struct {
	contexts *ContextService
	enabled  bool
}

func NewGoogleEarthReviewService(contexts *ContextService, enabled bool) *GoogleEarthReviewService {
	return &GoogleEarthReviewService{contexts: contexts, enabled: enabled}
}

type GoogleEarthReviewRequest struct {
	DescriptionField string `json:"descriptionField"`
	Offset           int    `json:"offset"`
	Limit            int    `json:"limit"`
}

func (request *GoogleEarthReviewRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthReviewRequest
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "descriptionField", "offset", "limit"); err != nil {
		return fmt.Errorf("Google Earth review request: %w", err)
	}
	*request = GoogleEarthReviewRequest(decoded)
	return nil
}

type GoogleEarthReviewRow struct {
	EnvRowID        string              `json:"envRowId"`
	MembershipRowID string              `json:"membershipRowId"`
	PlotNumber      ProjectMetadataCell `json:"plotNumber"`
	StoredLongitude ProjectMetadataCell `json:"storedLongitude"`
	Longitude       ProjectMetadataCell `json:"longitude"`
	Latitude        ProjectMetadataCell `json:"latitude"`
	Description     ProjectMetadataCell `json:"description"`
}

type GoogleEarthReview struct {
	ContextID        string                   `json:"contextId"`
	Project          string                   `json:"project"`
	ProjectPath      string                   `json:"projectPath"`
	SU               string                   `json:"su"`
	SUPath           string                   `json:"suPath"`
	DescriptionField string                   `json:"descriptionField"`
	Fields           []EnvironmentReportField `json:"fields"`
	Offset           int                      `json:"offset"`
	Limit            int                      `json:"limit"`
	TotalRows        int                      `json:"totalRows"`
	Rows             []GoogleEarthReviewRow   `json:"rows"`
}

type GoogleEarthDescriptionFields struct {
	ContextID   string                  `json:"contextId"`
	Project     string                  `json:"project"`
	ProjectPath string                  `json:"projectPath"`
	SU          string                  `json:"su"`
	SUPath      string                  `json:"suPath"`
	EnvTable    string                  `json:"envTable"`
	Fields      []ProjectMetadataColumn `json:"fields"`
}

func (s *GoogleEarthReviewService) requireReview(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Google Earth read-only review is disabled in this session")
	}
	if s.contexts == nil {
		return errors.New("Google Earth review context service is unavailable")
	}
	return nil
}

func validateGoogleEarthReviewStrings(values ...string) error {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return errors.New("Google Earth review requires valid UTF-8; no transport text was repaired")
		}
	}
	return nil
}

func (s *GoogleEarthReviewService) GetGoogleEarthDescriptionFields(ctx context.Context, contextID string) (*GoogleEarthDescriptionFields, error) {
	if err := s.requireReview(ctx); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*GoogleEarthDescriptionFields, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*GoogleEarthDescriptionFields, error) {
			// Read the literal current physical schema, not the form's Sample_Env precedent.
			table, err := readPhysicalLocationTable(ctx, tx, "project", owner.selection.Project+"_Env")
			if err != nil {
				return nil, err
			}
			result := &GoogleEarthDescriptionFields{
				ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
				SU: owner.selection.SU, SUPath: owner.selection.SUPath, EnvTable: owner.selection.Project + "_Env",
				Fields: append([]ProjectMetadataColumn{}, table.Columns...),
			}
			if err := validateGoogleEarthReviewStrings(result.ContextID, result.Project, result.ProjectPath,
				result.SU, result.SUPath, result.EnvTable); err != nil {
				return nil, err
			}
			for _, field := range result.Fields {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if err := validateGoogleEarthReviewStrings(field.Name, field.DeclaredType); err != nil {
					return nil, err
				}
			}
			return result, nil
		})
	})
}

func (s *GoogleEarthReviewService) GetGoogleEarthReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthReview, error) {
	if err := s.requireReview(ctx); err != nil {
		return nil, err
	}
	var request GoogleEarthReviewRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.DescriptionField); err != nil {
		return nil, err
	}
	if request.DescriptionField == "" || strings.ContainsRune(request.DescriptionField, 0) ||
		request.Offset < 0 || request.Limit < 1 || request.Limit > 500 {
		return nil, errors.New("Google Earth review requires a literal description field, nonnegative offset and limit 1..500")
	}
	// The accepted reader owns its operation lease and snapshot; never nest its lease.
	source, err := s.contexts.readGoogleEarthLocations(ctx, contextID, request.DescriptionField)
	if err != nil {
		return nil, err
	}
	return googleEarthReviewTransport(ctx, source, request)
}

func googleEarthReviewTransport(ctx context.Context, source *ownedGoogleEarthLocations, request GoogleEarthReviewRequest) (*GoogleEarthReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(source.ContextID, source.ProjectPath, source.SUPath,
		source.Report.Project, source.Report.SU, source.Report.DescriptionField); err != nil {
		return nil, err
	}
	result := &GoogleEarthReview{
		ContextID: source.ContextID, Project: source.Report.Project, ProjectPath: source.ProjectPath,
		SU: source.Report.SU, SUPath: source.SUPath, DescriptionField: source.Report.DescriptionField,
		Fields: []EnvironmentReportField{
			{"Env", "PlotNumber", "Plot Number", false},
			{"Env", "Longitude", "Longitude", false},
			{"Env", "Latitude", "Latitude", false},
			{"Env", source.Report.DescriptionField, source.Report.DescriptionField, false},
		},
		Offset: request.Offset, Limit: request.Limit, TotalRows: len(source.Report.Rows),
		Rows: []GoogleEarthReviewRow{},
	}
	// Validate the complete snapshot before paging so a bad hidden row cannot yield a partial review.
	for i, row := range source.Report.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateGoogleEarthReviewStrings(row.EnvRowID, row.MembershipRowID); err != nil {
			return nil, err
		}
		for _, cell := range []ProjectMetadataCell{row.PlotNumber, row.StoredLongitude, row.Longitude, row.Latitude, row.Description} {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if i < request.Offset || i-request.Offset >= request.Limit {
			continue
		}
		result.Rows = append(result.Rows, GoogleEarthReviewRow{
			EnvRowID: row.EnvRowID, MembershipRowID: row.MembershipRowID,
			PlotNumber: cloneSiteUnitCell(row.PlotNumber), StoredLongitude: cloneSiteUnitCell(row.StoredLongitude),
			Longitude: cloneSiteUnitCell(row.Longitude), Latitude: cloneSiteUnitCell(row.Latitude),
			Description: cloneSiteUnitCell(row.Description),
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
