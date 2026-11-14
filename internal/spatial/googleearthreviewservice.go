package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type GoogleEarthReviewService struct {
	contexts *vprocore.ContextService
}

func NewGoogleEarthReviewService(contexts *vprocore.ContextService) *GoogleEarthReviewService {
	return &GoogleEarthReviewService{contexts: contexts}
}

type GoogleEarthReviewRequest struct {
	DescriptionField string `json:"descriptionField"`
	Offset           int    `json:"offset"`
	Limit            int    `json:"limit"`
}

func (request *GoogleEarthReviewRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthReviewRequest
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "descriptionField", "offset", "limit"); err != nil {
		return fmt.Errorf("Google Earth review request: %w", err)
	}
	*request = GoogleEarthReviewRequest(decoded)
	return nil
}

type GoogleEarthReviewRow struct {
	EnvRowID        string                       `json:"envRowId"`
	MembershipRowID string                       `json:"membershipRowId"`
	PlotNumber      vprocore.ProjectMetadataCell `json:"plotNumber"`
	StoredLongitude vprocore.ProjectMetadataCell `json:"storedLongitude"`
	Longitude       vprocore.ProjectMetadataCell `json:"longitude"`
	Latitude        vprocore.ProjectMetadataCell `json:"latitude"`
	Description     vprocore.ProjectMetadataCell `json:"description"`
}

type GoogleEarthReview struct {
	ContextID        string                            `json:"contextId"`
	Project          string                            `json:"project"`
	ProjectPath      string                            `json:"projectPath"`
	SU               string                            `json:"su"`
	SUPath           string                            `json:"suPath"`
	DescriptionField string                            `json:"descriptionField"`
	Fields           []vprocore.EnvironmentReportField `json:"fields"`
	Offset           int                               `json:"offset"`
	Limit            int                               `json:"limit"`
	TotalRows        int                               `json:"totalRows"`
	Rows             []GoogleEarthReviewRow            `json:"rows"`
}

type GoogleEarthDescriptionFields struct {
	ContextID   string                           `json:"contextId"`
	Project     string                           `json:"project"`
	ProjectPath string                           `json:"projectPath"`
	SU          string                           `json:"su"`
	SUPath      string                           `json:"suPath"`
	EnvTable    string                           `json:"envTable"`
	Fields      []vprocore.ProjectMetadataColumn `json:"fields"`
}

func (s *GoogleEarthReviewService) requireReview(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Google Earth review requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("Google Earth review context service is unavailable")
	}
	return nil
}

func (s *GoogleEarthReviewService) GetGoogleEarthDescriptionFields(ctx context.Context, contextID string) (*GoogleEarthDescriptionFields, error) {
	if err := s.requireReview(ctx); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*GoogleEarthDescriptionFields, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*GoogleEarthDescriptionFields, error) {
			// Read the literal current physical schema, not the form's Sample_Env precedent.
			table, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
			if err != nil {
				return nil, err
			}
			result := &GoogleEarthDescriptionFields{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, EnvTable: owner.Selection.Project + "_Env",
				Fields: append([]vprocore.ProjectMetadataColumn{}, table.Columns...),
			}
			if err := vprocore.ValidateGoogleEarthReviewStrings(result.ContextID, result.Project, result.ProjectPath,
				result.SU, result.SUPath, result.EnvTable); err != nil {
				return nil, err
			}
			for _, field := range result.Fields {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if err := vprocore.ValidateGoogleEarthReviewStrings(field.Name, field.DeclaredType); err != nil {
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.DescriptionField); err != nil {
		return nil, err
	}
	if request.DescriptionField == "" || strings.ContainsRune(request.DescriptionField, 0) ||
		request.Offset < 0 || request.Limit < 1 || request.Limit > 500 {
		return nil, errors.New("Google Earth review requires a literal description field, nonnegative offset and limit 1..500")
	}
	// The accepted reader owns its operation lease and snapshot; never nest its lease.
	source, err := ReadGoogleEarthLocations(s.contexts, ctx, contextID, request.DescriptionField)
	if err != nil {
		return nil, err
	}
	return googleEarthReviewTransport(ctx, source, request)
}

func googleEarthReviewTransport(ctx context.Context, source *ownedGoogleEarthLocations, request GoogleEarthReviewRequest) (*GoogleEarthReview, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(source.ContextID, source.ProjectPath, source.SUPath,
		source.Report.Project, source.Report.SU, source.Report.DescriptionField); err != nil {
		return nil, err
	}
	result := &GoogleEarthReview{
		ContextID: source.ContextID, Project: source.Report.Project, ProjectPath: source.ProjectPath,
		SU: source.Report.SU, SUPath: source.SUPath, DescriptionField: source.Report.DescriptionField,
		Fields: []vprocore.EnvironmentReportField{
			{Source: "Env", Key: "PlotNumber", Label: "Plot Number", Heading: false},
			{Source: "Env", Key: "Longitude", Label: "Longitude", Heading: false},
			{Source: "Env", Key: "Latitude", Label: "Latitude", Heading: false},
			{Source: "Env", Key: source.Report.DescriptionField, Label: source.Report.DescriptionField, Heading: false},
		},
		Offset: request.Offset, Limit: request.Limit, TotalRows: len(source.Report.Rows),
		Rows: []GoogleEarthReviewRow{},
	}
	// Validate the complete snapshot before paging so a bad hidden row cannot yield a partial review.
	for i, row := range source.Report.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := vprocore.ValidateGoogleEarthReviewStrings(row.EnvRowID, row.MembershipRowID); err != nil {
			return nil, err
		}
		for _, cell := range []vprocore.ProjectMetadataCell{row.PlotNumber, row.StoredLongitude, row.Longitude, row.Latitude, row.Description} {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if i < request.Offset || i-request.Offset >= request.Limit {
			continue
		}
		result.Rows = append(result.Rows, GoogleEarthReviewRow{
			EnvRowID: row.EnvRowID, MembershipRowID: row.MembershipRowID,
			PlotNumber: vprocore.CloneSiteUnitCell(row.PlotNumber), StoredLongitude: vprocore.CloneSiteUnitCell(row.StoredLongitude),
			Longitude: vprocore.CloneSiteUnitCell(row.Longitude), Latitude: vprocore.CloneSiteUnitCell(row.Latitude),
			Description: vprocore.CloneSiteUnitCell(row.Description),
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
