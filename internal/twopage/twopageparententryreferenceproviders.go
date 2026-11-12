package twopage

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

type TwoPageEntryReferenceReaders struct {
	Shared     sourceform.SiviParentSharedReferenceReaders `json:"-" yaml:"-"`
	Ecosection interface {
		ListEcosectionChoices(context.Context) ([]referencespkg.RegionCodeChoice, error)
	} `json:"-" yaml:"-"`
	Quality interface {
		ListPlotQualityChoices(context.Context) ([]referencespkg.PlotQualityChoice, error)
	} `json:"-" yaml:"-"`
	Soil interface {
		ListGreatGroupChoices(context.Context) ([]referencespkg.SoilCodeChoice, error)
		ListSubgroupChoices(context.Context) ([]referencespkg.SoilCodeChoice, error)
	} `json:"-" yaml:"-"`
	Series interface {
		ListBECSiteSeries(context.Context, *string, *string) ([]referencespkg.BECSiteSeries, error)
	} `json:"-" yaml:"-"`
}

type TwoPageEntryReferenceProvider struct {
	Fields  map[string]TwoPageEntryReferenceField `json:"-" yaml:"-"`
	Readers TwoPageEntryReferenceReaders          `json:"-" yaml:"-"`
}

func NewTwoPageEntryReferenceProvider(form string, readers TwoPageEntryReferenceReaders) (*TwoPageEntryReferenceProvider, error) {
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return nil, err
	}
	result := &TwoPageEntryReferenceProvider{Fields: map[string]TwoPageEntryReferenceField{}, Readers: readers}
	for _, field := range fields {
		result.Fields[field.Column] = field
	}
	return result, nil
}

func twoPageEntryBECFilter(column string, cell vprocore.ProjectMetadataCell, maximum int) (*string, error) {
	if _, err := vprocore.MetadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage == "null" {
		return nil, nil
	}
	if cell.Storage != "text" || cell.Text == nil {
		return nil, fmt.Errorf("historical nontext %s cannot filter complete-entry BEC choices", column)
	}
	if err := sqlutil.ValidateSiteCodeText(column, cell.Text, maximum); err != nil {
		return nil, err
	}
	return cell.Text, nil
}

func (s *TwoPageEntryReferenceProvider) Read(ctx context.Context, tx *sql.Tx, alias string, field TwoPageEntryReferenceField,
	zone, subZone vprocore.ProjectMetadataCell) (sourceform.SIVIParentSharedReference, error) {
	result := sourceform.SIVIParentSharedReference{Column: field.Column, ListName: field.List, Required: field.Required,
		Source:      "frozen-DAO-catalogue; source-ordinal identities",
		Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Choices:     []sourceform.SIVIParentSharedReferenceChoice{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	expected, present := s.Fields[field.Column]
	if !present || expected != field {
		return result, errors.New("complete-entry reference provider requires its exact source field policy")
	}
	if field.Reader == "bec" && field.Column == "SubZone" {
		filter, err := twoPageEntryBECFilter("Zone", zone, 4)
		if err != nil {
			return result, err
		}
		// The source helper's literal All and NULL both mean no Zone filter.
		if filter != nil && *filter == "All" {
			filter = nil
		}
		if s.Readers.Shared.Bec == nil {
			return result, errors.New("borrowed BEC catalogue is unavailable")
		}
		values, err := s.Readers.Shared.Bec.ListBECSubZones(ctx, filter)
		if err != nil {
			return result, err
		}
		result.Source = "verified BEC catalogue; source SubZoneList NULL/All selection; source-ordinal identities"
		result.Definitions.Columns = []vprocore.ProjectMetadataColumn{
			{Name: "Zone", DeclaredType: "TEXT"}, {Name: "SubZone", DeclaredType: "TEXT"}, {Name: "ZoneDescription", DeclaredType: "TEXT"}, {Name: "Description", DeclaredType: "TEXT"},
		}
		for _, row := range values {
			result.Definitions.Rows = append(result.Definitions.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID,
				Cells: []vprocore.ProjectMetadataCell{sourceform.SiviReferenceText(row.Zone), sourceform.SiviReferenceText(row.SubZone),
					sourceform.SiviReferenceText(row.ZoneDescription), sourceform.SiviReferenceText(row.Description)}})
			choice := sourceform.SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SubZone, Description: row.Description}
			if row.SubZone == nil {
				choice.Diagnostic = "Item is NULL"
			} else if err := sqlutil.ValidateSiteCodeText(field.Column, row.SubZone, field.Maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	} else if field.Reader == "bec-series" {
		zoneFilter, err := twoPageEntryBECFilter("Zone", zone, 4)
		if err != nil {
			return result, err
		}
		subZoneFilter, err := twoPageEntryBECFilter("SubZone", subZone, 8)
		if err != nil {
			return result, err
		}
		if s.Readers.Series == nil {
			return result, errors.New("borrowed BEC SiteSeries catalogue is unavailable")
		}
		values, err := s.Readers.Series.ListBECSiteSeries(ctx, zoneFilter, subZoneFilter)
		if err != nil {
			return result, err
		}
		result.Source = "verified BEC catalogue; current Zone/SubZone pair; source-ordinal identities; no stale RowSource reuse"
		result.Definitions = twoPageEntrySeriesDefinitions(values)
		for _, row := range values {
			choice := sourceform.SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SiteSeries, Description: row.Description}
			if !row.Selectable {
				choice.Diagnostic = row.Diagnostic
				if choice.Diagnostic == "" {
					choice.Diagnostic = "SiteSeries is not selectable in the verified source catalogue"
				}
			} else if err := sqlutil.ValidateSiteCodeText(field.Column, row.SiteSeries, field.Maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	} else {
		var rows []listcatalog.Choice
		switch field.Reader {
		case "ecosection":
			if s.Readers.Ecosection == nil {
				return result, errors.New("borrowed Ecosection catalogue is unavailable")
			}
			values, err := s.Readers.Ecosection.ListEcosectionChoices(ctx)
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "quality":
			if s.Readers.Quality == nil {
				return result, errors.New("borrowed plot quality catalogue is unavailable")
			}
			values, err := s.Readers.Quality.ListPlotQualityChoices(ctx)
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "soil":
			if s.Readers.Soil == nil {
				return result, errors.New("borrowed soil classification catalogue is unavailable")
			}
			var values []referencespkg.SoilCodeChoice
			var err error
			if field.List == "SoilClassGroup" {
				values, err = s.Readers.Soil.ListGreatGroupChoices(ctx)
			} else {
				values, err = s.Readers.Soil.ListSubgroupChoices(ctx)
			}
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "project", "master-unit", "working-unit":
			return result, fmt.Errorf("complete-entry %s requires the unfinished owned context-selection provider", field.Column)
		default:
			if field.Reader == "family" && (tx == nil || alias == "") {
				return result, errors.New("complete-entry family references require an owned transaction and attachment alias")
			}
			shared := &sourceform.SIVIParentSharedService{References: s.Readers.Shared}
			policy := field.SiviParentSharedReferencePolicy
			if field.Column == "LocationAccuracy" {
				policy.List = "Accuracy"
			}
			var err error
			result, err = shared.ReadReference(ctx, tx, alias, policy, zone)
			if err != nil {
				return result, err
			}
			result.ListName = field.List
		}
		if field.Reader == "ecosection" || field.Reader == "quality" || field.Reader == "soil" {
			policy := field.SiviParentSharedReferencePolicy
			if field.Reader == "ecosection" {
				policy.List = "Ecosection"
			}
			if err := sourceform.ProjectSIVIReferenceCatalogue(&result, policy, rows); err != nil {
				return result, err
			}
		}
	}
	if field.Column == "SurfaceTopographyType" {
		for i := range result.Choices {
			choice := &result.Choices[i]
			for _, excluded := range []string{"cc", "cv", "st"} {
				if twoPageEntryOptionalReferenceMatch(field, excluded, *choice) {
					choice.Selectable = false
					choice.Diagnostic = "Item is excluded by the source SurfaceTopographyType RowSource"
				}
			}
		}
	}
	if field.storage == "integer" {
		for i := range result.Choices {
			choice := &result.Choices[i]
			if !choice.Selectable {
				continue
			}
			if _, err := TwoPageEntryReferenceCode(field, vprocore.ProjectMetadataCell{Storage: "integer", Integer: choice.Code}); err != nil {
				choice.Selectable = false
				choice.Diagnostic = err.Error()
			}
		}
	}
	result.Available = true
	return result, ctx.Err()
}

func twoPageEntrySeriesDefinitions(rows []referencespkg.BECSiteSeries) vprocore.ProjectMetadataTable {
	result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, name := range strings.Split(referencespkg.BecSeriesColumns, ",")[1:] {
		name = strings.TrimSpace(name)
		typ := "TEXT"
		if name == "ReferenceID" {
			typ = "REAL"
		} else if name == "Flag" {
			typ = "BOOLEAN"
		}
		result.Columns = append(result.Columns, vprocore.ProjectMetadataColumn{Name: name, DeclaredType: typ})
	}
	for _, row := range rows {
		cells := []vprocore.ProjectMetadataCell{}
		for _, value := range []*string{row.SourceID, row.SSCode, row.Name, row.Region, row.Zone, row.SubZone,
			row.BaseSubZone, row.Variant, row.Phase, row.SiteSeries, row.SiteSeriesPhase, row.Variation, row.Seral,
			row.Description, row.PlantAssociation, row.Comments} {
			cells = append(cells, sourceform.SiviReferenceText(value))
		}
		reference := vprocore.ProjectMetadataCell{Storage: "null"}
		if row.ReferenceID != nil {
			value := *row.ReferenceID
			reference = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
		}
		cells = append(cells, reference)
		for _, value := range []*string{row.AddedDate, row.ExpiredDate, row.OriginalSourceID, row.MergedBGC,
			row.Suballiance, row.Alliance, row.MissingNpeNa, row.TransferID} {
			cells = append(cells, sourceform.SiviReferenceText(value))
		}
		cells = append(cells, sourceform.SiviReferenceBool(row.Flag))
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result
}
