package spatial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

func GetPlotLocationReview(s *vprocore.ContextService, ctx context.Context, contextID string) (*vprocore.PlotLocationReview, error) {
	if ctx == nil {
		return nil, errors.New("plot location review requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil {
		return nil, errors.New("plot location review requires owned contexts")
	}
	return s.ReadPlotLocations(ctx, contextID)
}

type googleEarthLocation struct {
	EnvRowID        string
	MembershipRowID string
	PlotNumber      vprocore.ProjectMetadataCell
	StoredLongitude vprocore.ProjectMetadataCell
	Longitude       vprocore.ProjectMetadataCell
	Latitude        vprocore.ProjectMetadataCell
	Description     vprocore.ProjectMetadataCell
}

type googleEarthLocations struct {
	Project, SU, DescriptionField string
	Rows                          []googleEarthLocation
}

func snapshotGoogleEarthTable(table vprocore.ProjectMetadataTable) vprocore.ProjectMetadataTable {
	table.Columns = append([]vprocore.ProjectMetadataColumn(nil), table.Columns...)
	rows := make([]vprocore.ProjectMetadataRow, len(table.Rows))
	for i, row := range table.Rows {
		cells := make([]vprocore.ProjectMetadataCell, len(row.Cells))
		for j, cell := range row.Cells {
			cells[j] = vprocore.CloneSiteUnitCell(cell)
		}
		rows[i] = vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells}
	}
	table.Rows = rows
	return table
}

func planGoogleEarthLocations(ctx context.Context, project, suName, descriptionField string, env, su vprocore.ProjectMetadataTable) (*googleEarthLocations, error) {
	env = snapshotGoogleEarthTable(env)
	if suName != "None" {
		su = snapshotGoogleEarthTable(su)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !vprocore.ProjectNamePattern.MatchString(project) || !vprocore.ProjectNamePattern.MatchString(suName) ||
		descriptionField == "" || !utf8.ValidString(descriptionField) || strings.ContainsRune(descriptionField, 0) {
		return nil, errors.New("Google Earth preparation requires literal project/SU and a physical description column")
	}
	columns, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber", "Longitude", "Latitude", descriptionField)
	if err != nil {
		return nil, fmt.Errorf("Google Earth Env schema: %w", err)
	}
	memberships := map[string][]vprocore.ProjectMetadataRow{}
	if suName != "None" {
		suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber")
		if err != nil {
			return nil, fmt.Errorf("Google Earth SU schema: %w", err)
		}
		memberships, err = vprocore.SiteUnitTransferIndex(su, suColumns["PlotNumber"])
		if err != nil {
			return nil, err
		}
	}
	result := &googleEarthLocations{Project: project, SU: suName, DescriptionField: descriptionField,
		Rows: []googleEarthLocation{}}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot := row.Cells[columns["PlotNumber"]]
		if _, err := vprocore.MetadataCellValue(plot); err != nil {
			return nil, err
		}
		var selected []vprocore.ProjectMetadataRow
		if suName != "None" {
			if plot.Storage != "text" {
				continue
			}
			selected = memberships[*plot.Text]
			if len(selected) == 0 {
				continue
			}
		}
		longitude, latitude := row.Cells[columns["Longitude"]], row.Cells[columns["Latitude"]]
		for _, cell := range []vprocore.ProjectMetadataCell{longitude, latitude} {
			if _, err := vprocore.MetadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if longitude.Storage == "null" || latitude.Storage == "null" {
			continue
		}
		if latitude.Storage != "integer" && latitude.Storage != "real" {
			return nil, errors.New("Google Earth latitude requires original numeric storage")
		}
		converted, err := vprocore.NegateLocationLongitude(longitude)
		if err != nil {
			return nil, err
		}
		description := row.Cells[columns[descriptionField]]
		if _, err := vprocore.MetadataCellValue(description); err != nil {
			return nil, err
		}
		appendLocation := func(membershipID string) {
			result.Rows = append(result.Rows, googleEarthLocation{
				EnvRowID: row.RowID, MembershipRowID: membershipID,
				PlotNumber: vprocore.CloneSiteUnitCell(plot), StoredLongitude: vprocore.CloneSiteUnitCell(longitude),
				Longitude: vprocore.CloneSiteUnitCell(converted), Latitude: vprocore.CloneSiteUnitCell(latitude),
				Description: vprocore.CloneSiteUnitCell(description),
			})
		}
		if suName == "None" {
			appendLocation("")
			continue
		}
		for _, membership := range selected {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			appendLocation(membership.RowID)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

type ownedGoogleEarthLocations struct {
	ContextID, ProjectPath, SUPath string
	Report                         googleEarthLocations
}

func readGoogleEarthLocationsSnapshot(ctx context.Context, contextID, descriptionField string, owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
	env, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
	if err != nil {
		return nil, err
	}
	selected := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	if owner.Selection.SU != "None" {
		selected, err = vprocore.ReadPhysicalLocationTable(ctx, tx, "su", owner.Selection.SU+"_SU")
		if err != nil {
			return nil, err
		}
	}
	report, err := planGoogleEarthLocations(ctx, owner.Selection.Project, owner.Selection.SU, descriptionField, env, selected)
	if err != nil {
		return nil, err
	}
	return &ownedGoogleEarthLocations{ContextID: contextID, ProjectPath: owner.Selection.ProjectPath,
		SUPath: owner.Selection.SUPath, Report: *report}, nil
}

func ReadGoogleEarthLocations(s *vprocore.ContextService, ctx context.Context, contextID, descriptionField string) (*ownedGoogleEarthLocations, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*ownedGoogleEarthLocations, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthLocations, error) {
			return readGoogleEarthLocationsSnapshot(ctx, contextID, descriptionField, owner, tx)
		})
	})
}

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
