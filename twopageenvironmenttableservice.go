package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const twoPageEnvironmentTableEnvironment = "VPRO_TWO_PAGE_ENVIRONMENT_TABLE_REVIEW"
const twoPageEnvironmentTableScanMaximum = 100000
const twoPageEnvironmentTablePageMaximum = 200
const twoPageEnvironmentTableOrdering = "declared-target-order; controlled-English-ignore-case; descending-numeric-NULL-last; physical-row-ties"

type TwoPageEnvironmentTableRow struct {
	EnvRowID   string `json:"envRowId"`
	AdminRowID string `json:"adminRowId"`
}

type TwoPageEnvironmentTableReview struct {
	ContextID             string                       `json:"contextId"`
	Project               string                       `json:"project"`
	ProjectPath           string                       `json:"projectPath"`
	SU                    string                       `json:"su"`
	SUPath                string                       `json:"suPath"`
	Plot                  string                       `json:"plot"`
	TargetForm            string                       `json:"targetForm"`
	EnvTable              string                       `json:"envTable"`
	AdminTable            string                       `json:"adminTable"`
	Rows                  []TwoPageEnvironmentTableRow `json:"rows"`
	EnvOriginal           ProjectMetadataTable         `json:"envOriginal"`
	AdminOriginal         ProjectMetadataTable         `json:"adminOriginal"`
	Offset                int                          `json:"offset"`
	Limit                 int                          `json:"limit"`
	TotalRows             int64                        `json:"totalRows"`
	OrderingPolicy        string                       `json:"orderingPolicy"`
	GeneralLocaleVerified bool                         `json:"generalLocaleVerified"`
}

type TwoPageEnvironmentTableService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageEnvironmentTableService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageEnvironmentTableService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page environment table requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(twoPageEnvironmentTableEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageEnvironmentTableService{contexts, enabled}, nil
}

func (s *desktopConfig) readEnvironmentTableTarget(ctx context.Context) (string, error) {
	if s == nil {
		return "", errors.New("two-page environment table configuration is unavailable")
	}
	if err := acquireMutexLease(ctx, &s.mu); err != nil {
		return "", err
	}
	defer s.mu.Unlock()
	values, err := s.readLocked()
	if err != nil {
		return "", err
	}
	current, err := configSection(values, "Current")
	if err != nil {
		return "", err
	}
	target := "FS882-6x4"
	if _, present := current["DataFormName"]; present {
		target, err = configString(values, "Current", "DataFormName")
		if err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	switch target {
	case "FS882-6x4", "FS882-6x4XL", "FS882-8x6XL", "FS882-8x6XL-CHARS":
		return target, nil
	default:
		return "", fmt.Errorf("two-page environment table does not support saved Current.DataFormName %q", target)
	}
}

func (s *TwoPageEnvironmentTableService) GetReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageEnvironmentTableReview, error) {
	if ctx == nil {
		return nil, errors.New("two-page environment table requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.review {
		return nil, errors.New("two-page environment table requires independently enabled saved-data review")
	}
	if plot == "" || !utf8.ValidString(plot) || offset < 0 || int64(offset) > int64(twoPageEnvironmentTableScanMaximum)*twoPageEnvironmentTableScanMaximum ||
		limit < 1 || limit > twoPageEnvironmentTablePageMaximum {
		return nil, errors.New("two-page environment table requires a valid literal opening parent, offset0..10000000000 and limit1..200")
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageEnvironmentTableReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageEnvironmentTableReview, error) {
			project := owner.selection.Project
			envTable, adminTable := project+"_Env", project+"_Admin"
			if err := requireTwoPageChildReadScope(ctx, tx, project, envTable, plot, "environment table"); err != nil {
				return nil, err
			}
			target, err := plots.projects.preferences.readEnvironmentTableTarget(ctx)
			if err != nil {
				return nil, err
			}
			read := func(name string) (ProjectMetadataTable, error) {
				if err := validateSIVIPhysicalSchema(ctx, tx, "project", name, "two-page environment table"); err != nil {
					return ProjectMetadataTable{}, err
				}
				var count int64
				if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM project."+quoteHeaderIdentifier(name)).Scan(&count); err != nil {
					return ProjectMetadataTable{}, err
				}
				if count < 0 || count > twoPageEnvironmentTableScanMaximum {
					return ProjectMetadataTable{}, errors.New("two-page environment table source exceeds the100000-row physical snapshot bound; no partial review")
				}
				return readPhysicalLocationTable(ctx, tx, "project", name)
			}
			env, err := read(envTable)
			if err != nil {
				return nil, fmt.Errorf("two-page environment table Env original: %w", err)
			}
			admin, err := read(adminTable)
			if err != nil {
				return nil, fmt.Errorf("two-page environment table Admin original: %w", err)
			}
			result, err := planTwoPageEnvironmentTable(ctx, env, admin, target, offset, limit)
			if err != nil {
				return nil, err
			}
			result.ContextID, result.Project, result.ProjectPath = contextID, project, owner.selection.ProjectPath
			result.SU, result.SUPath, result.Plot = owner.selection.SU, owner.selection.SUPath, plot
			result.EnvTable, result.AdminTable = envTable, adminTable
			return result, nil
		})
	})
}

func planTwoPageEnvironmentTable(ctx context.Context, env, admin ProjectMetadataTable, target string, offset, limit int) (*TwoPageEnvironmentTableReview, error) {
	required := []string{"PlotNumber"}
	if target == "FS882-6x4" {
		required = append(required, "RootingDepth", "SeepageDepth")
	}
	envIndices, err := siteUnitTransferColumns(env, required...)
	if err != nil {
		return nil, fmt.Errorf("two-page environment table Env schema: %w", err)
	}
	adminIndices, err := siteUnitTransferColumns(admin, "Plot")
	if err != nil {
		return nil, fmt.Errorf("two-page environment table Admin schema: %w", err)
	}
	comparator := collate.New(language.English, collate.IgnoreCase)
	var buffer collate.Buffer
	key := func(cell ProjectMetadataCell) (string, bool, error) {
		value, err := metadataCellValue(cell)
		if err != nil || value == nil {
			return "", value == nil, err
		}
		text, ok := value.(string)
		if !ok {
			return "", false, errors.New("two-page environment table requires original TEXT/NULL join-key storage; no identity coerced")
		}
		result := string(comparator.KeyFromString(&buffer, text))
		buffer.Reset()
		return result, false, nil
	}
	members := map[string][]int{}
	for i, row := range admin.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, null, err := key(row.Cells[adminIndices["Plot"]])
		if err != nil {
			return nil, err
		}
		if !null {
			members[value] = append(members[value], i)
		}
	}
	type preparedRow struct {
		index   int
		key     string
		numbers []*big.Rat
	}
	prepared := make([]preparedRow, 0, len(env.Rows))
	for i, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, null, err := key(row.Cells[envIndices["PlotNumber"]])
		if err != nil {
			return nil, err
		}
		if null || len(members[value]) == 0 {
			continue
		}
		numbers := make([]*big.Rat, len(required)-1)
		for j, name := range required[1:] {
			number, err := vegetationReportNumber(row.Cells[envIndices[name]])
			if err != nil {
				return nil, fmt.Errorf("two-page environment table row %s %s: %w", row.RowID, name, err)
			}
			numbers[j] = number
		}
		prepared = append(prepared, preparedRow{i, value, numbers})
	}
	sort.SliceStable(prepared, func(i, j int) bool {
		a, b := prepared[i], prepared[j]
		for n, x := range a.numbers {
			y := b.numbers[n]
			if x == nil && y == nil {
				continue
			}
			if x == nil {
				return false
			}
			if y == nil {
				return true
			}
			if comparison := x.Cmp(y); comparison != 0 {
				return comparison > 0
			}
		}
		return a.key < b.key
	})
	result := &TwoPageEnvironmentTableReview{TargetForm: target, Rows: []TwoPageEnvironmentTableRow{},
		EnvOriginal:   ProjectMetadataTable{Columns: env.Columns, Rows: []ProjectMetadataRow{}},
		AdminOriginal: ProjectMetadataTable{Columns: admin.Columns, Rows: []ProjectMetadataRow{}},
		Offset:        offset, Limit: limit, OrderingPolicy: twoPageEnvironmentTableOrdering}
	seenEnv, seenAdmin := map[string]bool{}, map[string]bool{}
	for _, row := range prepared {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		membership := members[row.key]
		position, count := result.TotalRows, int64(len(membership))
		result.TotalRows += count
		start, end := int64(offset)-position, int64(offset)+int64(limit)-position
		if start < 0 {
			start = 0
		}
		if end > count {
			end = count
		}
		for i := start; i < end; i++ {
			left, right := env.Rows[row.index], admin.Rows[membership[i]]
			result.Rows = append(result.Rows, TwoPageEnvironmentTableRow{left.RowID, right.RowID})
			if !seenEnv[left.RowID] {
				result.EnvOriginal.Rows = append(result.EnvOriginal.Rows, left)
				seenEnv[left.RowID] = true
			}
			if !seenAdmin[right.RowID] {
				result.AdminOriginal.Rows = append(result.AdminOriginal.Rows, right)
				seenAdmin[right.RowID] = true
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(offset) > result.TotalRows {
		return nil, errors.New("two-page environment table offset exceeds the saved source row count")
	}
	return result, nil
}
