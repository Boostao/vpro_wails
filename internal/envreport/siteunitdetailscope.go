package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type siteUnitDetailQuerySource string

const (
	siteUnitDetailProjectJoin siteUnitDetailQuerySource = "project-env-admin"
	siteUnitDetailSelectedSU  siteUnitDetailQuerySource = "selected-su-filtered-env-admin"
)

type siteUnitDetailJoinedPlot struct {
	PlotNumber string
	SURowID    string
	EnvRowID   string
	AdminRowID string
}

type siteUnitDetailScopeUnit struct {
	Code  string
	Plots []siteUnitDetailJoinedPlot
}

type siteUnitDetailScopeMembership struct {
	RowID      string
	PlotNumber vprocore.ProjectMetadataCell
	SiteUnit   vprocore.ProjectMetadataCell
	JoinedRows int
	Status     string
}

type siteUnitDetailScope struct {
	Project     string
	SU          string
	QuerySource siteUnitDetailQuerySource
	Units       []siteUnitDetailScopeUnit
	Memberships []siteUnitDetailScopeMembership
}

// This prepares physical normal-SU join weights, not source collation, numeric
// summaries, reference names, hierarchy/field-derived units or report output.
func prepareSiteUnitDetailScope(ctx context.Context, project, suName string, source siteUnitDetailQuerySource,
	maxJoinedRows int, env, admin, su vprocore.ProjectMetadataTable) (siteUnitDetailScope, error) {
	fail := func(err error) (siteUnitDetailScope, error) { return siteUnitDetailScope{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project) ||
		!utf8.ValidString(suName) || strings.ContainsRune(project+suName, 0) {
		return fail(errors.New("site unit detail scope requires explicit valid project/normal-SU identities"))
	}
	if source != siteUnitDetailProjectJoin && source != siteUnitDetailSelectedSU {
		return fail(errors.New("site unit detail scope requires known project or current selected-SU query provenance"))
	}
	if maxJoinedRows <= 0 {
		return fail(errors.New("site unit detail scope requires a positive joined-row budget"))
	}
	envColumns, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return fail(fmt.Errorf("site unit detail Env schema: %w", err))
	}
	adminColumns, err := vprocore.SiteUnitTransferColumns(admin, "Plot")
	if err != nil {
		return fail(fmt.Errorf("site unit detail Admin schema: %w", err))
	}
	suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("site unit detail selected SU schema: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	envRows, err := vprocore.SiteUnitTransferIndex(env, envColumns["PlotNumber"])
	if err != nil {
		return fail(fmt.Errorf("site unit detail Env identity: %w", err))
	}
	adminRows, err := vprocore.SiteUnitTransferIndex(admin, adminColumns["Plot"])
	if err != nil {
		return fail(fmt.Errorf("site unit detail Admin identity: %w", err))
	}
	result := siteUnitDetailScope{Project: project, SU: suName, QuerySource: source,
		Units: []siteUnitDetailScopeUnit{}, Memberships: []siteUnitDetailScopeMembership{}}
	units := map[string][]siteUnitDetailJoinedPlot{}
	joined := 0
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plotCell, unitCell := row.Cells[suColumns["PlotNumber"]], row.Cells[suColumns["SiteUnit"]]
		plot, err := vprocore.VegetationReportIdentity(plotCell)
		if err != nil {
			return fail(fmt.Errorf("site unit detail SU row %s plot identity: %w", row.RowID, err))
		}
		unit, err := vprocore.VegetationReportIdentity(unitCell)
		if err != nil {
			return fail(fmt.Errorf("site unit detail SU row %s unit identity: %w", row.RowID, err))
		}
		membership := siteUnitDetailScopeMembership{RowID: row.RowID, PlotNumber: vprocore.CloneSiteUnitCell(plotCell),
			SiteUnit: vprocore.CloneSiteUnitCell(unitCell)}
		switch {
		case unit == nil:
			membership.Status = "null-unit"
		case plot == nil:
			membership.Status = "null-plot"
		case len(envRows[*plot]) == 0:
			membership.Status = "missing-env"
		case len(adminRows[*plot]) == 0:
			membership.Status = "missing-admin"
		default:
			membership.Status = "joined"
			// Filtered_Env DISTINCTROW projects Env/Admin, not SU: its
			// selected-SU filter does not multiply this final SU join again.
			for _, environment := range envRows[*plot] {
				for _, administration := range adminRows[*plot] {
					if err := ctx.Err(); err != nil {
						return fail(err)
					}
					if joined == maxJoinedRows {
						return fail(fmt.Errorf("site unit detail scope exceeds joined-row budget %d", maxJoinedRows))
					}
					units[*unit] = append(units[*unit], siteUnitDetailJoinedPlot{
						*plot, row.RowID, environment.RowID, administration.RowID})
					joined++
					membership.JoinedRows++
				}
			}
		}
		result.Memberships = append(result.Memberships, membership)
	}
	for unit, plots := range units {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		sort.Slice(plots, func(i, j int) bool {
			a, b := plots[i], plots[j]
			if a.PlotNumber != b.PlotNumber {
				return a.PlotNumber < b.PlotNumber
			}
			if a.SURowID != b.SURowID {
				return a.SURowID < b.SURowID
			}
			if a.EnvRowID != b.EnvRowID {
				return a.EnvRowID < b.EnvRowID
			}
			return a.AdminRowID < b.AdminRowID
		})
		result.Units = append(result.Units, siteUnitDetailScopeUnit{unit, plots})
	}
	sort.Slice(result.Units, func(i, j int) bool { return result.Units[i].Code > result.Units[j].Code })
	sort.Slice(result.Memberships, func(i, j int) bool { return result.Memberships[i].RowID < result.Memberships[j].RowID })
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
