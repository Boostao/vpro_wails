package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type vegetationSelectedUnitOccurrence struct {
	MembershipRowID string
	EnvRowID        *string
	AdminRowID      *string
	PlotNumber      ProjectMetadataCell
	SiteUnit        ProjectMetadataCell
}

type vegetationSelectedUnitField struct {
	admin bool
	index int
	kind  string
}

func selectedUnitFields(env, admin ProjectMetadataTable, names []string) ([]vegetationSelectedUnitField, error) {
	if len(names) == 0 {
		return nil, errors.New("selected report units require at least one explicit ordered field; no scratch rows were deleted")
	}
	result := make([]vegetationSelectedUnitField, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			return nil, errors.New("selected report unit fields require distinct nonempty physical identifiers")
		}
		for _, char := range name {
			if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_') {
				return nil, fmt.Errorf("selected report unit field %q has an uncalibrated source identifier", name)
			}
		}
		seen[key] = true
		matches := []vegetationSelectedUnitField{}
		for _, table := range []struct {
			admin bool
			data  ProjectMetadataTable
		}{{false, env}, {true, admin}} {
			for i, column := range table.data.Columns {
				if strings.ToLower(column.Name) == key {
					kind := ""
					switch column.DeclaredType {
					case "TEXT", "VARCHAR":
						kind = "text"
					case "INTEGER", "LONG":
						kind = "integer"
					case "BOOLEAN", "BIT":
						kind = "boolean"
					default:
						return nil, fmt.Errorf("selected report unit field %q has uncalibrated physical type %q", name, column.DeclaredType)
					}
					matches = append(matches, vegetationSelectedUnitField{admin: table.admin, index: i, kind: kind})
				}
			}
		}
		if len(matches) != 1 {
			return nil, fmt.Errorf("selected report unit field %q is absent or ambiguous in physical Env/Admin", name)
		}
		result = append(result, matches[0])
	}
	return result, nil
}

func selectedUnitScalar(cell ProjectMetadataCell, kind string) (*string, error) {
	value, err := metadataCellValue(cell)
	if err != nil || value == nil {
		return nil, err
	}
	switch kind {
	case "text":
		text, ok := value.(string)
		if !ok || strings.ContainsRune(text, 0) {
			return nil, errors.New("selected report unit text requires original valid Unicode without NUL")
		}
		return &text, nil
	case "integer", "boolean":
		number, ok := value.(int64)
		if !ok || kind == "integer" && (number < math.MinInt32 || number > math.MaxInt32) ||
			kind == "boolean" && number != -1 && number != 0 {
			return nil, errors.New("selected report unit scalar requires original Long or Boolean -1/0 storage")
		}
		text := strconv.FormatInt(number, 10)
		return &text, nil
	default:
		return nil, errors.New("selected report unit scalar type is unavailable")
	}
}

func selectedUnitIdentityReader() func(ProjectMetadataCell) (*string, error) {
	literals := map[string]string{}
	return func(cell ProjectMetadataCell) (*string, error) {
		value, err := vegetationReportIdentity(cell)
		if err != nil || value == nil {
			return value, err
		}
		for _, char := range *value {
			if char == 0 || char > 127 {
				return nil, errors.New("selected report unit joins require calibrated ASCII plot identities without NUL")
			}
		}
		key := strings.ToLower(*value)
		if previous, found := literals[key]; found && previous != *value {
			return nil, errors.New("selected report unit joins have unverified distinct case-equivalent plot identities")
		}
		literals[key] = *value
		return value, nil
	}
}

func selectedUnitLabel(values []*string) (ProjectMetadataCell, error) {
	var label strings.Builder
	present := false
	for _, value := range values {
		if value != nil {
			present = true
			label.WriteString(*value)
		}
	}
	if !present {
		return ProjectMetadataCell{Storage: "null"}, nil
	}
	text := label.String()
	if err := validateChildPhysicalText("selected report unit label", text, 255); err != nil {
		return ProjectMetadataCell{}, err
	}
	return ProjectMetadataCell{Storage: "text", Text: &text}, nil
}

// Source2 builds SU LEFT JOIN (Env INNER JOIN Admin), then prunes by qualified
// plot existence. It must not reuse source3's quality-occurrence multiplication.
func planLongVegetationSelectedUnits(ctx context.Context, env, admin, su ProjectMetadataTable,
	names []string, quality *vegetationQualitySelection) ([]vegetationSelectedUnitOccurrence, error) {
	return planSelectedUnitProjection(ctx, env, admin, su, names, quality, false)
}

func planSelectedUnitProjection(ctx context.Context, env, admin, su ProjectMetadataTable,
	names []string, quality *vegetationQualitySelection, destination bool) ([]vegetationSelectedUnitOccurrence, error) {
	if ctx == nil {
		return nil, errors.New("selected report units require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ec, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("selected report units Env: %w", err)
	}
	ac, err := siteUnitTransferColumns(admin, "Plot")
	if err != nil {
		return nil, fmt.Errorf("selected report units Admin: %w", err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("selected report units SU: %w", err)
	}
	fields, err := selectedUnitFields(env, admin, names)
	if err != nil {
		return nil, err
	}
	identity := selectedUnitIdentityReader()
	type parent struct {
		row    ProjectMetadataRow
		values map[int]*string
	}
	read := func(table ProjectMetadataTable, plotIndex int, isAdmin bool) (map[string]parent, map[string][]parent, error) {
		byID, byPlot := map[string]parent{}, map[string][]parent{}
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			plot, err := identity(row.Cells[plotIndex])
			if err != nil {
				return nil, nil, err
			}
			p := parent{row: row, values: map[int]*string{}}
			for _, field := range fields {
				if field.admin == isAdmin {
					value, err := selectedUnitScalar(row.Cells[field.index], field.kind)
					if err != nil {
						return nil, nil, fmt.Errorf("physical row %s selected field %s: %w", row.RowID, table.Columns[field.index].Name, err)
					}
					p.values[field.index] = value
				}
			}
			byID[row.RowID] = p
			if plot != nil {
				byPlot[*plot] = append(byPlot[*plot], p)
			}
		}
		return byID, byPlot, nil
	}
	envIDs, envPlots, err := read(env, ec["PlotNumber"], false)
	if err != nil {
		return nil, err
	}
	adminIDs, adminPlots, err := read(admin, ac["Plot"], true)
	if err != nil {
		return nil, err
	}
	members := map[string]ProjectMetadataRow{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := identity(row.Cells[sc["PlotNumber"]]); err != nil {
			return nil, err
		}
		if _, err := metadataCellValue(row.Cells[sc["SiteUnit"]]); err != nil {
			return nil, err
		}
		members[row.RowID] = row
	}
	qualified := map[string]bool{}
	if quality != nil {
		for _, occurrence := range quality.Occurrences {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			member, memberOK := members[occurrence.Membership.RowID]
			e, envOK := envIDs[occurrence.EnvRowID]
			a, adminOK := adminIDs[occurrence.AdminRowID]
			plot := occurrence.Membership.PlotNumber
			if !memberOK || !envOK || !adminOK || plot.Storage != "text" ||
				!reflect.DeepEqual(member.Cells[sc["PlotNumber"]], plot) ||
				!reflect.DeepEqual(member.Cells[sc["SiteUnit"]], occurrence.Membership.SiteUnit) ||
				!reflect.DeepEqual(e.row.Cells[ec["PlotNumber"]], plot) ||
				!reflect.DeepEqual(a.row.Cells[ac["Plot"]], plot) {
				return nil, errors.New("selected report unit qualification is not an original SU/Env/Admin join")
			}
			qualified[*plot.Text] = true
		}
	}
	result := []vegetationSelectedUnitOccurrence{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot := row.Cells[sc["PlotNumber"]]
		joined := false
		if plot.Storage == "text" {
			for _, e := range envPlots[*plot.Text] {
				for _, a := range adminPlots[*plot.Text] {
					if err := ctx.Err(); err != nil {
						return nil, err
					}
					values := make([]*string, 0, len(fields))
					for _, field := range fields {
						value := e.values[field.index]
						if field.admin {
							value = a.values[field.index]
						}
						values = append(values, value)
					}
					cell, err := selectedUnitLabel(values)
					if err != nil {
						return nil, err
					}
					envID, adminID := e.row.RowID, a.row.RowID
					result = append(result, vegetationSelectedUnitOccurrence{MembershipRowID: row.RowID,
						EnvRowID: &envID, AdminRowID: &adminID, PlotNumber: cloneSiteUnitCell(plot), SiteUnit: cell})
					joined = true
				}
			}
		}
		if !joined {
			result = append(result, vegetationSelectedUnitOccurrence{MembershipRowID: row.RowID,
				PlotNumber: cloneSiteUnitCell(plot), SiteUnit: ProjectMetadataCell{Storage: "null"}})
		}
	}
	if destination {
		for _, row := range result {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := validateSelectedUnitDestination(row); err != nil {
				return nil, err
			}
		}
	}
	if quality != nil {
		retained := []vegetationSelectedUnitOccurrence{}
		for _, row := range result {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if row.PlotNumber.Storage == "text" && qualified[*row.PlotNumber.Text] {
				retained = append(retained, row)
			}
		}
		result = retained
	}
	return result, nil
}
