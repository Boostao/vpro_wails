package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

type LongVegetationSelectedUnitField struct {
	Table        string `json:"table"`
	Name         string `json:"name"`
	DeclaredType string `json:"declaredType"`
	Available    bool   `json:"available"`
	Reason       string `json:"reason"`
}

type LongVegetationSelectedUnitSelection struct {
	ContextID    string                                    `json:"contextId"`
	Project      string                                    `json:"project"`
	ProjectPath  string                                    `json:"projectPath"`
	SU           string                                    `json:"su"`
	SUPath       string                                    `json:"suPath"`
	Revision     string                                    `json:"revision"`
	Fields       []string                                  `json:"fields"`
	Definitions  []LongVegetationSelectedUnitField         `json:"definitions"`
	Build        *vprocore.LongVegetationSelectedUnitBuild `json:"build"`
	BuildCurrent bool                                      `json:"buildCurrent"`
}

type selectedUnitSelectionDraft struct {
	Revision string   `json:"revision"`
	Fields   []string `json:"fields"`
}

func (request *selectedUnitSelectionDraft) UnmarshalJSON(data []byte) error {
	type plain selectedUnitSelectionDraft
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "selected report unit field draft", "revision", "fields"); err != nil {
		return err
	}
	if decoded.Fields == nil {
		return errors.New("selected report unit draft requires an explicit field list, including an empty draft")
	}
	*request = selectedUnitSelectionDraft(decoded)
	return nil
}

type LongVegetationSelectedUnitSelectionService struct {
	contexts      *vprocore.ContextService
	snapshot      vprocore.PublicationReadSnapshotHooks
	beforeInstall func()
	beforeRead    func()
}

func NewLongVegetationSelectedUnitSelectionService(contexts *vprocore.ContextService) *LongVegetationSelectedUnitSelectionService {
	return &LongVegetationSelectedUnitSelectionService{contexts: contexts}
}

func (s *LongVegetationSelectedUnitSelectionService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("selected report unit selection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil ||
		s.contexts.Plots == nil || s.contexts.Projects.Preferences == nil {
		return errors.New("selected report unit selection owned context/configuration is unavailable")
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return err
	}
	return s.contexts.RequireSelectedListReportScope(contextID)
}

func selectedUnitRevision(value string) (int64, error) {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 0 || strconv.FormatInt(number, 10) != value {
		return 0, errors.New("selected report unit selection requires its exact nonnegative revision")
	}
	return number, nil
}

func readSelectedUnitSelectionTables(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx) ([3]vprocore.ProjectMetadataTable, error) {
	var tables [3]vprocore.ProjectMetadataTable
	if owner.Selection.SU == "None" {
		return tables, errors.New("selected report units require an explicit selected SU; all-project fallback is unavailable")
	}
	for i, source := range []struct{ role, name string }{
		{"project", owner.Selection.Project + "_Env"},
		{"project", owner.Selection.Project + "_Admin"},
		{"su", owner.Selection.SU + "_SU"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.name).Scan(&count); err != nil {
			return tables, err
		}
		if count != 1 {
			return tables, fmt.Errorf("selected report unit fields require physical table %s.%s", source.role, source.name)
		}
		table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.name, "", nil, "")
		if err != nil {
			return tables, err
		}
		tables[i] = table
	}
	return tables, nil
}

func reportUnitFieldDefinitions(env, admin vprocore.ProjectMetadataTable) []LongVegetationSelectedUnitField {
	definitions := []LongVegetationSelectedUnitField{}
	for i, table := range []vprocore.ProjectMetadataTable{env, admin} {
		for _, column := range table.Columns {
			field := LongVegetationSelectedUnitField{Table: []string{"Env", "Admin"}[i], Name: column.Name, DeclaredType: column.DeclaredType}
			if _, err := selectedUnitFields(env, admin, []string{column.Name}); err != nil {
				field.Reason = err.Error()
			} else {
				field.Available = true
			}
			definitions = append(definitions, field)
		}
	}
	return definitions
}

func selectedUnitSelectionDTO(contextID string, owner *vprocore.SqliteContext, tables [3]vprocore.ProjectMetadataTable,
	entry vprocore.SelectedUnitSelectionEntry) *LongVegetationSelectedUnitSelection {
	return &LongVegetationSelectedUnitSelection{
		ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
		SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Revision: strconv.FormatInt(entry.Revision, 10),
		Fields: append([]string{}, entry.Fields...), Definitions: reportUnitFieldDefinitions(tables[0], tables[1]),
		Build: vprocore.CloneSelectedUnitBuild(entry.Build),
	}
}

func (s *LongVegetationSelectedUnitSelectionService) GetLongVegetationSelectedUnitSelection(ctx context.Context, contextID string) (*LongVegetationSelectedUnitSelection, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	if s.beforeRead != nil {
		s.beforeRead()
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationSelectedUnitSelection, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationSelectedUnitSelection, error) {
			if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
				return nil, err
			}
			tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
			if err != nil {
				return nil, err
			}
			entry := s.contexts.SelectedUnitSelections.Read(contextID)
			result := selectedUnitSelectionDTO(contextID, owner, tables, entry)
			if entry.Build != nil {
				config, err := plots.Projects.Preferences.Snapshot()
				if err != nil {
					return nil, err
				}
				current, err := selectedUnitSelectionHash(contextID, owner, tables, entry.Fields, config)
				if err != nil {
					return nil, err
				}
				result.BuildCurrent = current == entry.Build.ApprovalHash
			}
			return result, nil
		})
	})
}

func (s *LongVegetationSelectedUnitSelectionService) SetLongVegetationSelectedUnitFields(ctx context.Context, contextID, requestJSON string) (*LongVegetationSelectedUnitSelection, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	var request selectedUnitSelectionDraft
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	expected, err := selectedUnitRevision(request.Revision)
	if err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationSelectedUnitSelection, error) {
		tables, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) ([3]vprocore.ProjectMetadataTable, error) {
			return readSelectedUnitSelectionTables(ctx, owner, tx)
		})
		if err != nil {
			return nil, err
		}
		if s.beforeInstall != nil {
			s.beforeInstall()
		}
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
			return nil, err
		}
		ledger := &s.contexts.SelectedUnitSelections
		ledger.Mu.Lock()
		defer ledger.Mu.Unlock()
		entry := ledger.Entries[contextID]
		if entry.Revision != expected {
			return nil, errors.New("selected report unit draft changed; reload before replacing fields")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.Revision == math.MaxInt64 {
			return nil, errors.New("selected report unit draft revision exhausted")
		}
		entry = vprocore.SelectedUnitSelectionEntry{Revision: entry.Revision + 1, Fields: append([]string{}, request.Fields...)}
		ledger.Entries = map[string]vprocore.SelectedUnitSelectionEntry{contextID: entry}
		return selectedUnitSelectionDTO(contextID, plots.Projects.Sqlite, tables, entry), nil
	})
}

func selectedUnitSelectionHash(contextID string, owner *vprocore.SqliteContext,
	tables [3]vprocore.ProjectMetadataTable, fields []string, config vprocore.ConfigValues) (string, error) {
	planned, err := selectedUnitConfigForChoice(config, 2)
	if err != nil {
		return "", err
	}
	material := struct {
		ContextID string
		Selection vprocore.DesktopSelection
		Fields    []string
		Tables    [3]vprocore.ProjectMetadataTable
		Config    vprocore.ConfigValues
	}{contextID, owner.Selection, fields, tables, planned}
	raw, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func buildSelectedUnitSelection(ctx context.Context, contextID string, owner *vprocore.SqliteContext,
	tables [3]vprocore.ProjectMetadataTable, fields []string, config vprocore.ConfigValues) (*vprocore.LongVegetationSelectedUnitBuild, error) {
	occurrences, err := planSelectedUnitProjection(ctx, tables[0], tables[1], tables[2], fields, nil, true)
	if err != nil {
		return nil, err
	}
	if len(occurrences) == 0 || len(occurrences) > 1048572 {
		return nil, errors.New("selected report unit Build requires nonempty bounded original derived memberships")
	}
	hash, err := selectedUnitSelectionHash(contextID, owner, tables, fields, config)
	if err != nil {
		return nil, err
	}
	result := &vprocore.LongVegetationSelectedUnitBuild{ApprovalHash: hash,
		Occurrences: make([]vprocore.LongVegetationSelectedUnitOccurrence, len(occurrences))}
	for i, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.Occurrences[i] = vprocore.LongVegetationSelectedUnitOccurrence{MembershipID: row.MembershipRowID,
			EnvRowID: sqlutil.CloneCatalogueValue(row.EnvRowID), AdminRowID: sqlutil.CloneCatalogueValue(row.AdminRowID),
			PlotNumber: vprocore.CloneSiteUnitCell(row.PlotNumber), SiteUnit: vprocore.CloneSiteUnitCell(row.SiteUnit)}
	}
	return result, nil
}

func (s *LongVegetationSelectedUnitSelectionService) BuildLongVegetationSelectedUnitSelection(ctx context.Context, contextID, revision string) (*LongVegetationSelectedUnitSelection, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	expected, err := selectedUnitRevision(revision)
	if err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationSelectedUnitSelection, error) {
		entry := s.contexts.SelectedUnitSelections.Read(contextID)
		if entry.Revision != expected {
			return nil, errors.New("selected report unit draft changed; reload before Build")
		}
		type candidate struct {
			tables [3]vprocore.ProjectMetadataTable
			build  *vprocore.LongVegetationSelectedUnitBuild
		}
		built, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (candidate, error) {
			tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
			if err != nil {
				return candidate{}, err
			}
			config, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return candidate{}, err
			}
			build, err := buildSelectedUnitSelection(ctx, contextID, owner, tables, entry.Fields, config)
			return candidate{tables, build}, err
		})
		if err != nil {
			return nil, err
		}
		if s.beforeInstall != nil {
			s.beforeInstall()
		}
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := s.contexts.RequireSelectedListReportScope(contextID); err != nil {
			return nil, err
		}
		ledger := &s.contexts.SelectedUnitSelections
		ledger.Mu.Lock()
		defer ledger.Mu.Unlock()
		current := ledger.Entries[contextID]
		if current.Revision != expected {
			return nil, errors.New("selected report unit draft changed during Build; stale result was discarded")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if current.Revision == math.MaxInt64 {
			return nil, errors.New("selected report unit Build revision exhausted")
		}
		current.Revision++
		current.Build = vprocore.CloneSelectedUnitBuild(built.build)
		ledger.Entries = map[string]vprocore.SelectedUnitSelectionEntry{contextID: current}
		result := selectedUnitSelectionDTO(contextID, plots.Projects.Sqlite, built.tables, current)
		result.BuildCurrent = true
		return result, nil
	})
}

func ApprovedSelectedUnitFields(s *vprocore.ContextService, ctx context.Context, contextID string, owner *vprocore.SqliteContext,
	tx *sql.Tx, config vprocore.ConfigValues) ([]string, error) {
	if err := s.RequireSelectedListReportScope(contextID); err != nil {
		return nil, err
	}
	entry := s.SelectedUnitSelections.Read(contextID)
	return validateSelectedUnitApproval(ctx, contextID, owner, tx, config, entry)
}

func validateSelectedUnitApproval(ctx context.Context, contextID string, owner *vprocore.SqliteContext,
	tx *sql.Tx, config vprocore.ConfigValues, entry vprocore.SelectedUnitSelectionEntry) ([]string, error) {
	if entry.Build == nil {
		return nil, errors.New("selected report unit fields have no explicit current session Build approval")
	}
	tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
	if err != nil {
		return nil, err
	}
	current, err := buildSelectedUnitSelection(ctx, contextID, owner, tables, entry.Fields, config)
	if err != nil {
		return nil, err
	}
	if current.ApprovalHash != entry.Build.ApprovalHash {
		return nil, errors.New("selected report unit source/configuration changed since Build; rebuild explicitly before output")
	}
	return append([]string{}, entry.Fields...), nil
}
