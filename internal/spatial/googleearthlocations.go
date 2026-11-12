package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
