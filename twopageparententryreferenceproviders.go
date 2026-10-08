package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

type twoPageEntryReferenceReaders struct {
	shared     siviParentSharedReferenceReaders
	ecosection interface {
		ListEcosectionChoices(context.Context) ([]RegionCodeChoice, error)
	}
	quality interface {
		ListPlotQualityChoices(context.Context) ([]PlotQualityChoice, error)
	}
	soil interface {
		ListGreatGroupChoices(context.Context) ([]SoilCodeChoice, error)
		ListSubgroupChoices(context.Context) ([]SoilCodeChoice, error)
	}
	series interface {
		ListBECSiteSeries(context.Context, *string, *string) ([]BECSiteSeries, error)
	}
}

type twoPageEntryReferenceProvider struct {
	fields  map[string]twoPageEntryReferenceField
	readers twoPageEntryReferenceReaders
}

func newTwoPageEntryReferenceProvider(form string, readers twoPageEntryReferenceReaders) (*twoPageEntryReferenceProvider, error) {
	fields, err := twoPageEntryReferenceFields(form)
	if err != nil {
		return nil, err
	}
	result := &twoPageEntryReferenceProvider{fields: map[string]twoPageEntryReferenceField{}, readers: readers}
	for _, field := range fields {
		result.fields[field.column] = field
	}
	return result, nil
}

func twoPageEntryBECFilter(column string, cell ProjectMetadataCell, maximum int) (*string, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage == "null" {
		return nil, nil
	}
	if cell.Storage != "text" || cell.Text == nil {
		return nil, fmt.Errorf("historical nontext %s cannot filter complete-entry BEC choices", column)
	}
	if err := validateSiteCodeText(column, cell.Text, maximum); err != nil {
		return nil, err
	}
	return cell.Text, nil
}

func (s *twoPageEntryReferenceProvider) read(ctx context.Context, tx *sql.Tx, alias string, field twoPageEntryReferenceField,
	zone, subZone ProjectMetadataCell) (SIVIParentSharedReference, error) {
	result := SIVIParentSharedReference{Column: field.column, ListName: field.list, Required: field.required,
		Source:      "frozen-DAO-catalogue; source-ordinal identities",
		Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Choices:     []SIVIParentSharedReferenceChoice{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	expected, present := s.fields[field.column]
	if !present || expected != field {
		return result, errors.New("complete-entry reference provider requires its exact source field policy")
	}
	if field.reader == "bec" && field.column == "SubZone" {
		filter, err := twoPageEntryBECFilter("Zone", zone, 4)
		if err != nil {
			return result, err
		}
		// The source helper's literal All and NULL both mean no Zone filter.
		if filter != nil && *filter == "All" {
			filter = nil
		}
		if s.readers.shared.bec == nil {
			return result, errors.New("borrowed BEC catalogue is unavailable")
		}
		values, err := s.readers.shared.bec.ListBECSubZones(ctx, filter)
		if err != nil {
			return result, err
		}
		result.Source = "verified BEC catalogue; source SubZoneList NULL/All selection; source-ordinal identities"
		result.Definitions.Columns = []ProjectMetadataColumn{
			{"Zone", "TEXT"}, {"SubZone", "TEXT"}, {"ZoneDescription", "TEXT"}, {"Description", "TEXT"},
		}
		for _, row := range values {
			result.Definitions.Rows = append(result.Definitions.Rows, ProjectMetadataRow{RowID: row.RowID,
				Cells: []ProjectMetadataCell{siviReferenceText(row.Zone), siviReferenceText(row.SubZone),
					siviReferenceText(row.ZoneDescription), siviReferenceText(row.Description)}})
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SubZone, Description: row.Description}
			if row.SubZone == nil {
				choice.Diagnostic = "Item is NULL"
			} else if err := validateSiteCodeText(field.column, row.SubZone, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	} else if field.reader == "bec-series" {
		zoneFilter, err := twoPageEntryBECFilter("Zone", zone, 4)
		if err != nil {
			return result, err
		}
		subZoneFilter, err := twoPageEntryBECFilter("SubZone", subZone, 8)
		if err != nil {
			return result, err
		}
		if s.readers.series == nil {
			return result, errors.New("borrowed BEC SiteSeries catalogue is unavailable")
		}
		values, err := s.readers.series.ListBECSiteSeries(ctx, zoneFilter, subZoneFilter)
		if err != nil {
			return result, err
		}
		result.Source = "verified BEC catalogue; current Zone/SubZone pair; source-ordinal identities; no stale RowSource reuse"
		result.Definitions = twoPageEntrySeriesDefinitions(values)
		for _, row := range values {
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SiteSeries, Description: row.Description}
			if !row.Selectable {
				choice.Diagnostic = row.Diagnostic
				if choice.Diagnostic == "" {
					choice.Diagnostic = "SiteSeries is not selectable in the verified source catalogue"
				}
			} else if err := validateSiteCodeText(field.column, row.SiteSeries, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	} else {
		var rows []listcatalog.Choice
		switch field.reader {
		case "ecosection":
			if s.readers.ecosection == nil {
				return result, errors.New("borrowed Ecosection catalogue is unavailable")
			}
			values, err := s.readers.ecosection.ListEcosectionChoices(ctx)
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "quality":
			if s.readers.quality == nil {
				return result, errors.New("borrowed plot quality catalogue is unavailable")
			}
			values, err := s.readers.quality.ListPlotQualityChoices(ctx)
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "soil":
			if s.readers.soil == nil {
				return result, errors.New("borrowed soil classification catalogue is unavailable")
			}
			var values []SoilCodeChoice
			var err error
			if field.list == "SoilClassGroup" {
				values, err = s.readers.soil.ListGreatGroupChoices(ctx)
			} else {
				values, err = s.readers.soil.ListSubgroupChoices(ctx)
			}
			if err != nil {
				return result, err
			}
			for _, row := range values {
				rows = append(rows, listcatalog.Choice(row))
			}
		case "project", "master-unit", "working-unit":
			return result, fmt.Errorf("complete-entry %s requires the unfinished owned context-selection provider", field.column)
		default:
			if field.reader == "family" && (tx == nil || alias == "") {
				return result, errors.New("complete-entry family references require an owned transaction and attachment alias")
			}
			shared := &SIVIParentSharedService{references: s.readers.shared}
			policy := field.siviParentSharedReferencePolicy
			if field.column == "LocationAccuracy" {
				policy.list = "Accuracy"
			}
			var err error
			result, err = shared.readReference(ctx, tx, alias, policy, zone)
			if err != nil {
				return result, err
			}
			result.ListName = field.list
		}
		if field.reader == "ecosection" || field.reader == "quality" || field.reader == "soil" {
			policy := field.siviParentSharedReferencePolicy
			if field.reader == "ecosection" {
				policy.list = "Ecosection"
			}
			if err := projectSIVIReferenceCatalogue(&result, policy, rows); err != nil {
				return result, err
			}
		}
	}
	if field.column == "SurfaceTopographyType" {
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
			if _, err := twoPageEntryReferenceCode(field, ProjectMetadataCell{Storage: "integer", Integer: choice.Code}); err != nil {
				choice.Selectable = false
				choice.Diagnostic = err.Error()
			}
		}
	}
	result.Available = true
	return result, ctx.Err()
}

func twoPageEntrySeriesDefinitions(rows []BECSiteSeries) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	for _, name := range strings.Split(becSeriesColumns, ",")[1:] {
		name = strings.TrimSpace(name)
		typ := "TEXT"
		if name == "ReferenceID" {
			typ = "REAL"
		} else if name == "Flag" {
			typ = "BOOLEAN"
		}
		result.Columns = append(result.Columns, ProjectMetadataColumn{name, typ})
	}
	for _, row := range rows {
		cells := []ProjectMetadataCell{}
		for _, value := range []*string{row.SourceID, row.SSCode, row.Name, row.Region, row.Zone, row.SubZone,
			row.BaseSubZone, row.Variant, row.Phase, row.SiteSeries, row.SiteSeriesPhase, row.Variation, row.Seral,
			row.Description, row.PlantAssociation, row.Comments} {
			cells = append(cells, siviReferenceText(value))
		}
		reference := ProjectMetadataCell{Storage: "null"}
		if row.ReferenceID != nil {
			value := *row.ReferenceID
			reference = ProjectMetadataCell{Storage: "real", Real: &value}
		}
		cells = append(cells, reference)
		for _, value := range []*string{row.AddedDate, row.ExpiredDate, row.OriginalSourceID, row.MergedBGC,
			row.Suballiance, row.Alliance, row.MissingNpeNa, row.TransferID} {
			cells = append(cells, siviReferenceText(value))
		}
		cells = append(cells, siviReferenceBool(row.Flag))
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result
}
