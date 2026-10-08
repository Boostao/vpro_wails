package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

type googleEarthLocation struct {
	EnvRowID        string
	MembershipRowID string
	PlotNumber      ProjectMetadataCell
	StoredLongitude ProjectMetadataCell
	Longitude       ProjectMetadataCell
	Latitude        ProjectMetadataCell
	Description     ProjectMetadataCell
}

type googleEarthLocations struct {
	Project, SU, DescriptionField string
	Rows                          []googleEarthLocation
}

func snapshotGoogleEarthTable(table ProjectMetadataTable) ProjectMetadataTable {
	table.Columns = append([]ProjectMetadataColumn(nil), table.Columns...)
	rows := make([]ProjectMetadataRow, len(table.Rows))
	for i, row := range table.Rows {
		cells := make([]ProjectMetadataCell, len(row.Cells))
		for j, cell := range row.Cells {
			cells[j] = cloneSiteUnitCell(cell)
		}
		rows[i] = ProjectMetadataRow{RowID: row.RowID, Cells: cells}
	}
	table.Rows = rows
	return table
}

func planGoogleEarthLocations(ctx context.Context, project, suName, descriptionField string, env, su ProjectMetadataTable) (*googleEarthLocations, error) {
	env = snapshotGoogleEarthTable(env)
	if suName != "None" {
		su = snapshotGoogleEarthTable(su)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !projectNamePattern.MatchString(project) || !projectNamePattern.MatchString(suName) ||
		descriptionField == "" || !utf8.ValidString(descriptionField) || strings.ContainsRune(descriptionField, 0) {
		return nil, errors.New("Google Earth preparation requires literal project/SU and a physical description column")
	}
	columns, err := siteUnitTransferColumns(env, "PlotNumber", "Longitude", "Latitude", descriptionField)
	if err != nil {
		return nil, fmt.Errorf("Google Earth Env schema: %w", err)
	}
	memberships := map[string][]ProjectMetadataRow{}
	if suName != "None" {
		suColumns, err := siteUnitTransferColumns(su, "PlotNumber")
		if err != nil {
			return nil, fmt.Errorf("Google Earth SU schema: %w", err)
		}
		memberships, err = siteUnitTransferIndex(su, suColumns["PlotNumber"])
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
		if _, err := metadataCellValue(plot); err != nil {
			return nil, err
		}
		var selected []ProjectMetadataRow
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
		for _, cell := range []ProjectMetadataCell{longitude, latitude} {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if longitude.Storage == "null" || latitude.Storage == "null" {
			continue
		}
		if latitude.Storage != "integer" && latitude.Storage != "real" {
			return nil, errors.New("Google Earth latitude requires original numeric storage")
		}
		converted, err := negateLocationLongitude(longitude)
		if err != nil {
			return nil, err
		}
		description := row.Cells[columns[descriptionField]]
		if _, err := metadataCellValue(description); err != nil {
			return nil, err
		}
		appendLocation := func(membershipID string) {
			result.Rows = append(result.Rows, googleEarthLocation{
				EnvRowID: row.RowID, MembershipRowID: membershipID,
				PlotNumber: cloneSiteUnitCell(plot), StoredLongitude: cloneSiteUnitCell(longitude),
				Longitude: cloneSiteUnitCell(converted), Latitude: cloneSiteUnitCell(latitude),
				Description: cloneSiteUnitCell(description),
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
