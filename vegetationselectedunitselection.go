package main

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
	"sync"
)

const longVegetationSelectedUnitsFeatureEnvironment = "VPRO_LONG_VEGETATION_SELECTED_UNITS"
const longVegetationSelectedUnitSelectionFeatureEnvironment = "VPRO_LONG_VEGETATION_SELECTED_UNIT_SELECTION"

type LongVegetationSelectedUnitField struct {
	Table        string `json:"table"`
	Name         string `json:"name"`
	DeclaredType string `json:"declaredType"`
	Available    bool   `json:"available"`
	Reason       string `json:"reason"`
}

type LongVegetationSelectedUnitBuild struct {
	ApprovalHash string                                 `json:"approvalHash"`
	Occurrences  []LongVegetationSelectedUnitOccurrence `json:"occurrences"`
}

type LongVegetationSelectedUnitSelection struct {
	ContextID    string                            `json:"contextId"`
	Project      string                            `json:"project"`
	ProjectPath  string                            `json:"projectPath"`
	SU           string                            `json:"su"`
	SUPath       string                            `json:"suPath"`
	Revision     string                            `json:"revision"`
	Fields       []string                          `json:"fields"`
	Definitions  []LongVegetationSelectedUnitField `json:"definitions"`
	Build        *LongVegetationSelectedUnitBuild  `json:"build"`
	BuildCurrent bool                              `json:"buildCurrent"`
}

type selectedUnitSelectionDraft struct {
	Revision string   `json:"revision"`
	Fields   []string `json:"fields"`
}

func (request *selectedUnitSelectionDraft) UnmarshalJSON(data []byte) error {
	type plain selectedUnitSelectionDraft
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "selected report unit field draft", "revision", "fields"); err != nil {
		return err
	}
	if decoded.Fields == nil {
		return errors.New("selected report unit draft requires an explicit field list, including an empty draft")
	}
	*request = selectedUnitSelectionDraft(decoded)
	return nil
}

type selectedUnitSelectionEntry struct {
	revision int64
	fields   []string
	build    *LongVegetationSelectedUnitBuild
}

type selectedUnitSelectionLedger struct {
	mu      sync.Mutex
	entries map[string]selectedUnitSelectionEntry
}

type LongVegetationSelectedUnitSelectionService struct {
	contexts      *ContextService
	enabled       bool
	snapshot      publicationReadSnapshotHooks
	beforeInstall func()
	beforeRead    func()
}

func NewLongVegetationSelectedUnitSelectionService(contexts *ContextService, enabled bool) *LongVegetationSelectedUnitSelectionService {
	return &LongVegetationSelectedUnitSelectionService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationSelectedUnitSelectionService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("selected report unit selection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("selected report unit selection is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil ||
		s.contexts.plots == nil || s.contexts.projects.preferences == nil {
		return errors.New("selected report unit selection owned context/configuration is unavailable")
	}
	if err := validateGoogleEarthReviewStrings(contextID); err != nil {
		return err
	}
	return s.contexts.requireSelectedListReportScope(contextID)
}

func selectedUnitRevision(value string) (int64, error) {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 0 || strconv.FormatInt(number, 10) != value {
		return 0, errors.New("selected report unit selection requires its exact nonnegative revision")
	}
	return number, nil
}

func cloneSelectedUnitBuild(build *LongVegetationSelectedUnitBuild) *LongVegetationSelectedUnitBuild {
	if build == nil {
		return nil
	}
	result := &LongVegetationSelectedUnitBuild{ApprovalHash: build.ApprovalHash,
		Occurrences: make([]LongVegetationSelectedUnitOccurrence, len(build.Occurrences))}
	for i, row := range build.Occurrences {
		row.EnvRowID, row.AdminRowID = cloneCatalogueValue(row.EnvRowID), cloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
		result.Occurrences[i] = row
	}
	return result
}

func (ledger *selectedUnitSelectionLedger) read(contextID string) selectedUnitSelectionEntry {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	entry := ledger.entries[contextID]
	entry.fields = append([]string{}, entry.fields...)
	entry.build = cloneSelectedUnitBuild(entry.build)
	return entry
}

func readSelectedUnitSelectionTables(ctx context.Context, owner *sqliteContext, tx *sql.Tx) ([3]ProjectMetadataTable, error) {
	var tables [3]ProjectMetadataTable
	if owner.selection.SU == "None" {
		return tables, errors.New("selected report units require an explicit selected SU; all-project fallback is unavailable")
	}
	for i, source := range []struct{ role, name string }{
		{"project", owner.selection.Project + "_Env"},
		{"project", owner.selection.Project + "_Admin"},
		{"su", owner.selection.SU + "_SU"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.name).Scan(&count); err != nil {
			return tables, err
		}
		if count != 1 {
			return tables, fmt.Errorf("selected report unit fields require physical table %s.%s", source.role, source.name)
		}
		table, err := readSQLiteStorageRows(ctx, tx, source.role, source.name, "", nil, "")
		if err != nil {
			return tables, err
		}
		tables[i] = table
	}
	return tables, nil
}

func reportUnitFieldDefinitions(env, admin ProjectMetadataTable) []LongVegetationSelectedUnitField {
	definitions := []LongVegetationSelectedUnitField{}
	for i, table := range []ProjectMetadataTable{env, admin} {
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

func selectedUnitSelectionDTO(contextID string, owner *sqliteContext, tables [3]ProjectMetadataTable,
	entry selectedUnitSelectionEntry) *LongVegetationSelectedUnitSelection {
	return &LongVegetationSelectedUnitSelection{
		ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
		SU: owner.selection.SU, SUPath: owner.selection.SUPath, Revision: strconv.FormatInt(entry.revision, 10),
		Fields: append([]string{}, entry.fields...), Definitions: reportUnitFieldDefinitions(tables[0], tables[1]),
		Build: cloneSelectedUnitBuild(entry.build),
	}
}

func (s *LongVegetationSelectedUnitSelectionService) GetLongVegetationSelectedUnitSelection(ctx context.Context, contextID string) (*LongVegetationSelectedUnitSelection, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	if s.beforeRead != nil {
		s.beforeRead()
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationSelectedUnitSelection, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationSelectedUnitSelection, error) {
			if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
				return nil, err
			}
			tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
			if err != nil {
				return nil, err
			}
			entry := s.contexts.selectedUnitSelections.read(contextID)
			result := selectedUnitSelectionDTO(contextID, owner, tables, entry)
			if entry.build != nil {
				config, err := plots.projects.preferences.snapshot()
				if err != nil {
					return nil, err
				}
				current, err := selectedUnitSelectionHash(contextID, owner, tables, entry.fields, config)
				if err != nil {
					return nil, err
				}
				result.BuildCurrent = current == entry.build.ApprovalHash
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
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationSelectedUnitSelection, error) {
		tables, err := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) ([3]ProjectMetadataTable, error) {
			return readSelectedUnitSelectionTables(ctx, owner, tx)
		})
		if err != nil {
			return nil, err
		}
		if s.beforeInstall != nil {
			s.beforeInstall()
		}
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
			return nil, err
		}
		ledger := &s.contexts.selectedUnitSelections
		ledger.mu.Lock()
		defer ledger.mu.Unlock()
		entry := ledger.entries[contextID]
		if entry.revision != expected {
			return nil, errors.New("selected report unit draft changed; reload before replacing fields")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.revision == math.MaxInt64 {
			return nil, errors.New("selected report unit draft revision exhausted")
		}
		entry = selectedUnitSelectionEntry{revision: entry.revision + 1, fields: append([]string{}, request.Fields...)}
		ledger.entries = map[string]selectedUnitSelectionEntry{contextID: entry}
		return selectedUnitSelectionDTO(contextID, plots.projects.sqlite, tables, entry), nil
	})
}

func selectedUnitSelectionHash(contextID string, owner *sqliteContext,
	tables [3]ProjectMetadataTable, fields []string, config configValues) (string, error) {
	planned, err := selectedUnitConfigForChoice(config, 2)
	if err != nil {
		return "", err
	}
	material := struct {
		ContextID string
		Selection desktopSelection
		Fields    []string
		Tables    [3]ProjectMetadataTable
		Config    configValues
	}{contextID, owner.selection, fields, tables, planned}
	raw, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func buildSelectedUnitSelection(ctx context.Context, contextID string, owner *sqliteContext,
	tables [3]ProjectMetadataTable, fields []string, config configValues) (*LongVegetationSelectedUnitBuild, error) {
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
	result := &LongVegetationSelectedUnitBuild{ApprovalHash: hash,
		Occurrences: make([]LongVegetationSelectedUnitOccurrence, len(occurrences))}
	for i, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.Occurrences[i] = LongVegetationSelectedUnitOccurrence{MembershipID: row.MembershipRowID,
			EnvRowID: cloneCatalogueValue(row.EnvRowID), AdminRowID: cloneCatalogueValue(row.AdminRowID),
			PlotNumber: cloneSiteUnitCell(row.PlotNumber), SiteUnit: cloneSiteUnitCell(row.SiteUnit)}
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
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationSelectedUnitSelection, error) {
		entry := s.contexts.selectedUnitSelections.read(contextID)
		if entry.revision != expected {
			return nil, errors.New("selected report unit draft changed; reload before Build")
		}
		type candidate struct {
			tables [3]ProjectMetadataTable
			build  *LongVegetationSelectedUnitBuild
		}
		built, err := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (candidate, error) {
			tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
			if err != nil {
				return candidate{}, err
			}
			config, err := plots.projects.preferences.snapshot()
			if err != nil {
				return candidate{}, err
			}
			build, err := buildSelectedUnitSelection(ctx, contextID, owner, tables, entry.fields, config)
			return candidate{tables, build}, err
		})
		if err != nil {
			return nil, err
		}
		if s.beforeInstall != nil {
			s.beforeInstall()
		}
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := s.contexts.requireSelectedListReportScope(contextID); err != nil {
			return nil, err
		}
		ledger := &s.contexts.selectedUnitSelections
		ledger.mu.Lock()
		defer ledger.mu.Unlock()
		current := ledger.entries[contextID]
		if current.revision != expected {
			return nil, errors.New("selected report unit draft changed during Build; stale result was discarded")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if current.revision == math.MaxInt64 {
			return nil, errors.New("selected report unit Build revision exhausted")
		}
		current.revision++
		current.build = cloneSelectedUnitBuild(built.build)
		ledger.entries = map[string]selectedUnitSelectionEntry{contextID: current}
		result := selectedUnitSelectionDTO(contextID, plots.projects.sqlite, built.tables, current)
		result.BuildCurrent = true
		return result, nil
	})
}

func (s *ContextService) approvedSelectedUnitFields(ctx context.Context, contextID string, owner *sqliteContext,
	tx *sql.Tx, config configValues) ([]string, error) {
	if err := s.requireSelectedListReportScope(contextID); err != nil {
		return nil, err
	}
	entry := s.selectedUnitSelections.read(contextID)
	return validateSelectedUnitApproval(ctx, contextID, owner, tx, config, entry)
}

func validateSelectedUnitApproval(ctx context.Context, contextID string, owner *sqliteContext,
	tx *sql.Tx, config configValues, entry selectedUnitSelectionEntry) ([]string, error) {
	if entry.build == nil {
		return nil, errors.New("selected report unit fields have no explicit current session Build approval")
	}
	tables, err := readSelectedUnitSelectionTables(ctx, owner, tx)
	if err != nil {
		return nil, err
	}
	current, err := buildSelectedUnitSelection(ctx, contextID, owner, tables, entry.fields, config)
	if err != nil {
		return nil, err
	}
	if current.ApprovalHash != entry.build.ApprovalHash {
		return nil, errors.New("selected report unit source/configuration changed since Build; rebuild explicitly before output")
	}
	return append([]string{}, entry.fields...), nil
}
