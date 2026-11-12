package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func validateSelectedUnitDestination(row vegetationSelectedUnitOccurrence) error {
	for _, field := range []struct {
		name string
		cell vprocore.ProjectMetadataCell
		max  int
	}{{"PlotNumber", row.PlotNumber, 7}, {"SiteUnit", row.SiteUnit, 255}} {
		value, err := vprocore.VegetationReportIdentity(field.cell)
		if err != nil {
			return fmt.Errorf("selected report unit destination %s: %w", field.name, err)
		}
		if value != nil {
			if *value == "" || strings.ContainsRune(*value, 0) {
				return fmt.Errorf("selected report unit destination %s refuses empty/NUL text; no derived row was dropped or normalized", field.name)
			}
			if err := vprocore.ValidateChildPhysicalText("selected report unit destination "+field.name, *value, field.max); err != nil {
				return err
			}
		}
	}
	return nil
}

func prepareLongVegetationSelectedUnits(ctx context.Context, project, suName string,
	veg, su, layers, env, admin vprocore.ProjectMetadataTable, names []string,
	quality *vegetationQualitySelection) (VegetationReportPreparation, []vegetationSelectedUnitOccurrence, error) {
	fail := func(err error) (VegetationReportPreparation, []vegetationSelectedUnitOccurrence, error) {
		return VegetationReportPreparation{}, nil, err
	}
	occurrences, err := planSelectedUnitProjection(ctx, env, admin, su, names, quality, true)
	if err != nil {
		return fail(err)
	}
	included := map[string]bool{}
	for _, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		included[row.MembershipRowID] = true
	}
	filtered := su
	filtered.Rows = []vprocore.ProjectMetadataRow{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if included[row.RowID] {
			filtered.Rows = append(filtered.Rows, row)
		}
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, filtered, layers)
	if err != nil {
		return fail(err)
	}
	prepared.selectedUnitOccurrences = make([]vegetationSelectedUnitOccurrence, len(occurrences))
	for i, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		row.EnvRowID, row.AdminRowID = sqlutil.CloneCatalogueValue(row.EnvRowID), sqlutil.CloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		prepared.selectedUnitOccurrences[i] = row
	}
	return prepared, occurrences, nil
}

func selectedUnitReportMemberships(ctx context.Context, prepared VegetationReportPreparation) ([]reportcore.VegetationReportMembership, error) {
	if prepared.selectedUnitOccurrences == nil {
		return prepared.Memberships, nil
	}
	if prepared.noneUnitMembershipCounts != nil || prepared.qualityMembershipCounts != nil {
		return nil, errors.New("selected report units cannot apply None/quality occurrence weights")
	}
	original := map[string]reportcore.VegetationReportMembership{}
	for _, member := range prepared.Memberships {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := original[member.RowID]; exists {
			return nil, errors.New("selected report units require unique original membership identities")
		}
		original[member.RowID] = member
	}
	type witness struct {
		member, env, admin string
	}
	seen := map[witness]bool{}
	result := make([]reportcore.VegetationReportMembership, 0, len(prepared.selectedUnitOccurrences))
	for _, row := range prepared.selectedUnitOccurrences {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		member, exists := original[row.MembershipRowID]
		if !exists || !reflect.DeepEqual(member.PlotNumber, row.PlotNumber) {
			return nil, errors.New("selected report unit derivation lost original physical membership")
		}
		if err := validateSelectedUnitDestination(row); err != nil {
			return nil, err
		}
		key := witness{member: row.MembershipRowID}
		if (row.EnvRowID == nil) != (row.AdminRowID == nil) {
			return nil, errors.New("selected report unit derivation requires both original Env/Admin witnesses or neither")
		}
		if row.EnvRowID == nil {
			if row.SiteUnit.Storage != "null" {
				return nil, errors.New("selected report unit unmatched membership cannot invent a label")
			}
		} else {
			if row.PlotNumber.Storage != "text" {
				return nil, errors.New("selected report unit matched parents require original non-NULL plot")
			}
			for _, id := range []*string{row.EnvRowID, row.AdminRowID} {
				number, err := strconv.ParseInt(*id, 10, 64)
				if err != nil || strconv.FormatInt(number, 10) != *id {
					return nil, errors.New("selected report unit derivation requires exact physical parent identities")
				}
			}
			key.env, key.admin = *row.EnvRowID, *row.AdminRowID
		}
		if seen[key] {
			return nil, errors.New("selected report unit derivation repeats an original SU/Env/Admin join")
		}
		seen[key] = true
		result = append(result, reportcore.VegetationReportMembership{RowID: member.RowID,
			PlotNumber: vprocore.CloneSiteUnitCell(row.PlotNumber), SiteUnit: vprocore.CloneSiteUnitCell(row.SiteUnit)})
	}
	return result, nil
}
