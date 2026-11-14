package envreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
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

type siteUnitDetailOwnedScope struct {
	ContextID   string
	ProjectPath string
	SUPath      string
	Scope       siteUnitDetailScope
}

func ReadSiteUnitDetailNormalSUScope(s *vprocore.ContextService, ctx context.Context, contextID string,
	maxJoinedRows int, hooks vprocore.PublicationReadSnapshotHooks) (siteUnitDetailOwnedScope, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (siteUnitDetailOwnedScope, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (siteUnitDetailOwnedScope, error) {
			if owner.Selection.SU == "None" {
				return siteUnitDetailOwnedScope{}, errors.New("site unit detail scope requires an explicitly selected normal SU")
			}
			env, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Env")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			admin, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "project", owner.Selection.Project+"_Admin")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			su, err := vprocore.ReadPhysicalLocationTable(ctx, tx, "su", owner.Selection.SU+"_SU")
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			scope, err := prepareSiteUnitDetailScope(ctx, owner.Selection.Project, owner.Selection.SU,
				siteUnitDetailSelectedSU, maxJoinedRows, env, admin, su)
			if err != nil {
				return siteUnitDetailOwnedScope{}, err
			}
			return siteUnitDetailOwnedScope{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, scope}, nil
		})
	})
}

type SiteUnitSummaryRequest struct {
	Method int `json:"method"`
}

func (request *SiteUnitSummaryRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryRequest
	var value plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &value, "Summary Environment preview", "method"); err != nil {
		return err
	}
	*request = SiteUnitSummaryRequest(value)
	return nil
}

type SiteUnitSummaryPreview struct {
	ContextID   string                `json:"contextId"`
	ProjectPath string                `json:"projectPath"`
	SUPath      string                `json:"suPath"`
	Report      SiteUnitSummaryReport `json:"report"`
}

type SiteUnitSummaryOptions struct {
	ContextID    string `json:"contextId"`
	Project      string `json:"project"`
	ProjectPath  string `json:"projectPath"`
	SU           string `json:"su"`
	SUPath       string `json:"suPath"`
	Method       int    `json:"method"`
	SiteUnitType int    `json:"siteUnitType"`
}

func GetSiteUnitSummaryOptions(s *vprocore.ContextService, ctx context.Context, contextID string) (SiteUnitSummaryOptions, error) {
	if ctx == nil {
		return SiteUnitSummaryOptions{}, errors.New("Summary Environment options require a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryOptions{}, err
	}
	if s == nil {
		return SiteUnitSummaryOptions{}, errors.New("Summary Environment options require owned contexts")
	}
	return ReadSiteUnitSummaryOptions(s, ctx, contextID, vprocore.PublicationReadSnapshotHooks{})
}

func ReadSiteUnitSummaryOptions(s *vprocore.ContextService, ctx context.Context, contextID string,
	hooks vprocore.PublicationReadSnapshotHooks) (SiteUnitSummaryOptions, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (SiteUnitSummaryOptions, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SiteUnitSummaryOptions, error) {
			if owner.Selection.SU == "None" {
				return SiteUnitSummaryOptions{}, errors.New("Summary Environment requires an explicitly selected normal SU")
			}
			config := plots.Projects.Preferences
			if config == nil {
				return SiteUnitSummaryOptions{}, errors.New("Summary Environment option configuration is unavailable")
			}
			if err := sqlutil.AcquireMutexLease(ctx, &config.Mu); err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			defer config.Mu.Unlock()
			values, err := config.ReadLocked()
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			method, err := vprocore.ConfigInt(values, "ReportOptions", "SEOptValueMethod", 1, 2)
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			unitType, err := vprocore.ConfigInt(values, "ReportOptions", "SESuType", 1, 3)
			if err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			if err := ctx.Err(); err != nil {
				return SiteUnitSummaryOptions{}, err
			}
			return SiteUnitSummaryOptions{contextID, owner.Selection.Project, owner.Selection.ProjectPath,
				owner.Selection.SU, owner.Selection.SUPath, method, unitType}, nil
		})
	})
}

func PreviewSiteUnitSummary(s *vprocore.ContextService, ctx context.Context, contextID string, request SiteUnitSummaryRequest) (SiteUnitSummaryPreview, error) {
	if ctx == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSummaryPreview{}, err
	}
	if s == nil {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment preview requires owned contexts")
	}
	return ReadSiteUnitSummary(s, ctx, contextID, request, vprocore.PublicationReadSnapshotHooks{})
}

func ReadSiteUnitSummary(s *vprocore.ContextService, ctx context.Context, contextID string, request SiteUnitSummaryRequest,
	hooks vprocore.PublicationReadSnapshotHooks) (SiteUnitSummaryPreview, error) {
	if request.Method != 1 && request.Method != 2 {
		return SiteUnitSummaryPreview{}, errors.New("Summary Environment requires Mean (1) or Interquartile (2)")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (SiteUnitSummaryPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SiteUnitSummaryPreview, error) {
			input, err := readSiteUnitSummaryWorkbookSource(ctx, owner, tx, contextID, request.Method)
			if err != nil {
				return SiteUnitSummaryPreview{}, err
			}
			return input.Preview, nil
		})
	})
}
