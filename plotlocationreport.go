package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
)

type PlotLocationRow struct {
	EnvRowID         string                `json:"envRowId"`
	AdminRowID       string                `json:"adminRowId"`
	MembershipRowIDs []string              `json:"membershipRowIds"`
	StoredLongitude  ProjectMetadataCell   `json:"storedLongitude"`
	Values           []ProjectMetadataCell `json:"values"`
}

type PlotLocationReport struct {
	Project string                   `json:"project"`
	SU      string                   `json:"su"`
	Fields  []EnvironmentReportField `json:"fields"`
	Rows    []PlotLocationRow        `json:"rows"`
}

func plotLocationFields() []EnvironmentReportField {
	return []EnvironmentReportField{
		{"Env", "PlotNumber", "Plot Number", false},
		{"Env", "Zone", "Zone", false},
		{"Env", "SubZone", "Subzone", false},
		{"Env", "SiteSeries", "Site Series", false},
		{"Env", "LocationAccuracy", "Accuracy", false},
		{"Env", "Latitude", "Latitude", false},
		{"Env", "Longitude", "Longitude", false},
		{"Env", "Elevation", "Elevation", false},
	}
}

func negateLocationLongitude(cell ProjectMetadataCell) (ProjectMetadataCell, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return ProjectMetadataCell{}, err
	}
	switch cell.Storage {
	case "integer":
		value, err := strconv.ParseInt(*cell.Integer, 10, 64)
		if err != nil || value == math.MinInt64 {
			return ProjectMetadataCell{}, errors.New("location longitude negation exceeds exact signed64 representation; original storage unchanged")
		}
		text := strconv.FormatInt(-value, 10)
		return ProjectMetadataCell{Storage: "integer", Integer: &text}, nil
	case "real":
		value := -*cell.Real
		return ProjectMetadataCell{Storage: "real", Real: &value}, nil
	default:
		return ProjectMetadataCell{}, errors.New("location longitude requires original numeric storage; no text/BLOB coercion or inferred coordinates")
	}
}

func planPlotLocations(ctx context.Context, project, suName string, env, admin, su ProjectMetadataTable) (*PlotLocationReport, error) {
	if !projectNamePattern.MatchString(project) || !projectNamePattern.MatchString(suName) {
		return nil, errors.New("plot location report requires literal desktop project/SU identities")
	}
	fields := plotLocationFields()
	required := make([]string, len(fields))
	for i, field := range fields {
		required[i] = field.Key
	}
	ec, err := siteUnitTransferColumns(env, required...)
	if err != nil {
		return nil, fmt.Errorf("plot location Env schema: %w", err)
	}
	ac, err := siteUnitTransferColumns(admin, "Plot")
	if err != nil {
		return nil, fmt.Errorf("plot location Admin schema: %w", err)
	}
	parents, err := siteUnitTransferIndex(admin, ac["Plot"])
	if err != nil {
		return nil, err
	}
	envIndex, err := siteUnitTransferIndex(env, ec["PlotNumber"])
	if err != nil {
		return nil, err
	}
	for _, index := range []map[string][]ProjectMetadataRow{parents, envIndex} {
		for _, rows := range index {
			if len(rows) > 1 {
				return nil, errors.New("plot location report rejects ambiguous physical Env/Admin plot links")
			}
		}
	}
	memberships := map[string][]ProjectMetadataRow{}
	if suName != "None" {
		sc, err := siteUnitTransferColumns(su, "PlotNumber")
		if err != nil {
			return nil, fmt.Errorf("plot location selected SU schema: %w", err)
		}
		memberships, err = siteUnitTransferIndex(su, sc["PlotNumber"])
		if err != nil {
			return nil, err
		}
	}
	report := &PlotLocationReport{Project: project, SU: suName, Fields: fields, Rows: []PlotLocationRow{}}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot := row.Cells[ec["PlotNumber"]].Text
		if plot == nil || len(parents[*plot]) != 1 || suName != "None" && len(memberships[*plot]) == 0 {
			continue
		}
		longitude, latitude := row.Cells[ec["Longitude"]], row.Cells[ec["Latitude"]]
		for _, cell := range []ProjectMetadataCell{longitude, latitude} {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
		}
		if longitude.Storage == "null" || latitude.Storage == "null" {
			continue
		}
		if latitude.Storage != "integer" && latitude.Storage != "real" {
			return nil, errors.New("location latitude requires original numeric storage; no inferred or converted coordinates")
		}
		converted, err := negateLocationLongitude(longitude)
		if err != nil {
			return nil, err
		}
		entry := PlotLocationRow{EnvRowID: row.RowID, AdminRowID: parents[*plot][0].RowID,
			MembershipRowIDs: []string{}, StoredLongitude: cloneSiteUnitCell(longitude),
			Values: make([]ProjectMetadataCell, len(fields))}
		for i, field := range fields {
			cell := row.Cells[ec[field.Key]]
			if _, err := metadataCellValue(cell); err != nil {
				return nil, err
			}
			entry.Values[i] = cloneSiteUnitCell(cell)
		}
		entry.Values[6] = converted
		for _, member := range memberships[*plot] {
			entry.MembershipRowIDs = append(entry.MembershipRowIDs, member.RowID)
		}
		report.Rows = append(report.Rows, entry)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return report, nil
}
