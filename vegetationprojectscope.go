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

const (
	longVegetationProjectScopeFeatureEnvironment = "VPRO_LONG_VEGETATION_PROJECT_SCOPE"
	vegetationSelectedListScope                  = "selected-list"
	vegetationProjectFieldsScope                 = "project-selected-fields"
	vegetationProjectAllScope                    = "project-all-plots"
)

type LongVegetationProjectUnit struct {
	EnvRowID   string              `json:"envRowId"`
	AdminRowID *string             `json:"adminRowId"`
	PlotNumber ProjectMetadataCell `json:"plotNumber"`
	SiteUnit   ProjectMetadataCell `json:"siteUnit"`
}

type LongVegetationProjectBuild struct {
	ApprovalHash  string                      `json:"approvalHash"`
	ProjectTuples []LongVegetationProjectUnit `json:"projectTuples"`
}

type LongVegetationReportScope struct {
	ContextID    string                      `json:"contextId"`
	Project      string                      `json:"project"`
	ProjectPath  string                      `json:"projectPath"`
	SU           string                      `json:"su"`
	SUPath       string                      `json:"suPath"`
	SourceKind   string                      `json:"sourceKind"`
	Revision     string                      `json:"revision"`
	Fields       []string                    `json:"fields"`
	Build        *LongVegetationProjectBuild `json:"build"`
	BuildCurrent bool                        `json:"buildCurrent"`
}

type vegetationScopeRequest struct {
	Revision   string `json:"revision"`
	SourceKind string `json:"sourceKind"`
}

func (request *vegetationScopeRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationScopeRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "Long Vegetation report scope", "revision", "sourceKind"); err != nil {
		return err
	}
	if decoded.SourceKind != vegetationSelectedListScope && decoded.SourceKind != vegetationProjectFieldsScope &&
		decoded.SourceKind != vegetationProjectAllScope {
		return errors.New("Long Vegetation report scope requires its exact explicit source kind")
	}
	*request = vegetationScopeRequest(decoded)
	return nil
}

type vegetationProjectScopeEntry struct {
	revision   int64
	sourceKind string
	fields     []string
	build      *LongVegetationProjectBuild
}

type vegetationProjectScopeLedger struct {
	mu      sync.Mutex
	entries map[string]vegetationProjectScopeEntry
}

func cloneProjectScopeEntry(entry vegetationProjectScopeEntry) vegetationProjectScopeEntry {
	if entry.sourceKind == "" {
		entry.sourceKind = vegetationSelectedListScope
	}
	entry.fields = append([]string{}, entry.fields...)
	if entry.build != nil {
		build := &LongVegetationProjectBuild{ApprovalHash: entry.build.ApprovalHash,
			ProjectTuples: make([]LongVegetationProjectUnit, len(entry.build.ProjectTuples))}
		for i, row := range entry.build.ProjectTuples {
			row.AdminRowID = cloneCatalogueValue(row.AdminRowID)
			row.PlotNumber, row.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
			build.ProjectTuples[i] = row
		}
		entry.build = build
	}
	return entry
}

func (ledger *vegetationProjectScopeLedger) read(contextID string) vegetationProjectScopeEntry {
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	return cloneProjectScopeEntry(ledger.entries[contextID])
}

func (s *ContextService) requireSelectedListReportScope(contextID string) error {
	s.projectReportScopes.mu.Lock()
	kind := s.projectReportScopes.entries[contextID].sourceKind
	s.projectReportScopes.mu.Unlock()
	if kind != "" && kind != vegetationSelectedListScope {
		return errors.New("Long Vegetation has an explicit project report scope; this selected-list consumer is unavailable")
	}
	return nil
}

type LongVegetationReportScopeService struct {
	contexts      *ContextService
	enabled       bool
	snapshot      publicationReadSnapshotHooks
	beforeInstall func()
}

func NewLongVegetationReportScopeService(contexts *ContextService, enabled bool) *LongVegetationReportScopeService {
	return &LongVegetationReportScopeService{contexts: contexts, enabled: enabled}
}

func (s *LongVegetationReportScopeService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation report scope requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || !s.enabled {
		return errors.New("Long Vegetation project report scope is disabled in this session")
	}
	if s.contexts == nil || s.contexts.projects == nil || s.contexts.projects.sqlite == nil ||
		s.contexts.projects.preferences == nil || s.contexts.plots == nil {
		return errors.New("Long Vegetation report scope owned context/configuration is unavailable")
	}
	return validateGoogleEarthReviewStrings(contextID)
}

func projectScopeDTO(contextID string, owner *sqliteContext, entry vegetationProjectScopeEntry) *LongVegetationReportScope {
	entry = cloneProjectScopeEntry(entry)
	return &LongVegetationReportScope{ContextID: contextID, Project: owner.selection.Project,
		ProjectPath: owner.selection.ProjectPath, SU: owner.selection.SU, SUPath: owner.selection.SUPath,
		SourceKind: entry.sourceKind, Revision: strconv.FormatInt(entry.revision, 10), Fields: entry.fields, Build: entry.build}
}

func readProjectScopeTables(ctx context.Context, owner *sqliteContext, tx *sql.Tx,
	entry vegetationProjectScopeEntry) ([]ProjectMetadataTable, error) {
	if entry.sourceKind != vegetationProjectFieldsScope && entry.sourceKind != vegetationProjectAllScope {
		return nil, errors.New("project Build requires an explicit project report scope")
	}
	if entry.sourceKind == vegetationProjectAllScope && owner.selection.SU != "None" {
		return nil, errors.New("project All plots requires the original None SU context")
	}
	sources := []struct{ role, table string }{{"project", owner.selection.Project + "_Env"}}
	if entry.sourceKind == vegetationProjectFieldsScope {
		sources = append(sources, struct{ role, table string }{"project", owner.selection.Project + "_Admin"})
		if owner.selection.SU != "None" {
			sources = append(sources, struct{ role, table string }{"su", owner.selection.SU + "_SU"})
		}
	}
	tables := make([]ProjectMetadataTable, len(sources))
	for i, source := range sources {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, fmt.Errorf("project Build requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = readSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return nil, err
		}
	}
	return tables, nil
}

func projectScopeHash(contextID string, owner *sqliteContext, entry vegetationProjectScopeEntry,
	tables []ProjectMetadataTable, config configValues) (string, error) {
	group := 2
	if entry.sourceKind == vegetationProjectAllScope {
		group = 3
	}
	planned, err := selectedUnitConfigForChoice(config, group)
	if err != nil {
		return "", err
	}
	enforce, err := longVegetationConfigBool(planned, "DataQualityFilterEnforceLV")
	if err != nil {
		return "", err
	}
	if enforce && (entry.sourceKind == vegetationProjectAllScope || owner.selection.SU == "None") {
		return "", errors.New("project report quality requires actual original selected-SU authority")
	}
	material := struct {
		ContextID  string
		Selection  desktopSelection
		SourceKind string
		Revision   int64
		Fields     []string
		Tables     []ProjectMetadataTable
		Config     configValues
	}{contextID, owner.selection, entry.sourceKind, entry.revision, entry.fields, tables, planned}
	raw, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func buildProjectScope(ctx context.Context, contextID string, owner *sqliteContext, entry vegetationProjectScopeEntry,
	tables []ProjectMetadataTable, config configValues) (*LongVegetationProjectBuild, error) {
	var inputs []vegetationProjectUnitInput
	if entry.sourceKind == vegetationProjectFieldsScope {
		rows, err := planLongVegetationProjectSelectedUnits(ctx, tables[0], tables[1], entry.fields)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			admin := row.AdminRowID
			inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, &admin, row.PlotNumber, row.SiteUnit})
		}
	} else if entry.sourceKind == vegetationProjectAllScope {
		rows, err := planLongVegetationProjectNoneUnits(ctx, tables[0])
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, nil, row.PlotNumber, row.SiteUnit})
		}
	} else {
		return nil, errors.New("project Build requires one explicit original project source")
	}
	inputs, err := validateProjectUnitInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 || len(inputs) > 1048572 {
		return nil, errors.New("project Build requires nonempty bounded original project tuples; empty-unit output is unavailable")
	}
	hash, err := projectScopeHash(contextID, owner, entry, tables, config)
	if err != nil {
		return nil, err
	}
	build := &LongVegetationProjectBuild{ApprovalHash: hash, ProjectTuples: make([]LongVegetationProjectUnit, len(inputs))}
	for i, row := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		build.ProjectTuples[i] = LongVegetationProjectUnit{row.EnvRowID, row.AdminRowID, row.PlotNumber, row.SiteUnit}
	}
	return build, nil
}

func (s *LongVegetationReportScopeService) GetLongVegetationReportScope(ctx context.Context, contextID string) (*LongVegetationReportScope, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationReportScope, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationReportScope, error) {
			entry := s.contexts.projectReportScopes.read(contextID)
			result := projectScopeDTO(contextID, owner, entry)
			if entry.build != nil {
				tables, err := readProjectScopeTables(ctx, owner, tx, entry)
				if err != nil {
					return nil, err
				}
				config, err := plots.projects.preferences.snapshot()
				if err != nil {
					return nil, err
				}
				hash, err := projectScopeHash(contextID, owner, entry, tables, config)
				if err != nil {
					return nil, err
				}
				result.BuildCurrent = hash == entry.build.ApprovalHash
			}
			return result, nil
		})
	})
}

func withProjectScopeInstall[T any](ctx context.Context, plots *PlotService, hooks publicationReadSnapshotHooks,
	install func(*sqliteContext) (T, error)) (T, error) {
	var zero T
	if _, err := withOwnedPreferenceSnapshot(ctx, plots, hooks, func(*sqliteContext, *sql.Tx) (struct{}, error) {
		return struct{}{}, nil
	}); err != nil {
		return zero, err
	}
	owner := plots.projects.sqlite
	if err := acquireMutexLease(ctx, &owner.mu); err != nil {
		return zero, err
	}
	defer owner.mu.Unlock()
	if err := profileOwnedFiles(owner); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return install(owner)
}

func (s *LongVegetationReportScopeService) SetLongVegetationReportScope(ctx context.Context, contextID, requestJSON string) (*LongVegetationReportScope, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	var request vegetationScopeRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	expected, err := selectedUnitRevision(request.Revision)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationReportScope, error) {
		return withProjectScopeInstall(ctx, plots, s.snapshot, func(owner *sqliteContext) (*LongVegetationReportScope, error) {
			ledger := &s.contexts.projectReportScopes
			ledger.mu.Lock()
			defer ledger.mu.Unlock()
			entry := cloneProjectScopeEntry(ledger.entries[contextID])
			if entry.revision != expected {
				return nil, errors.New("Long Vegetation report scope changed; reload before switching")
			}
			if request.SourceKind == vegetationProjectAllScope && owner.selection.SU != "None" {
				return nil, errors.New("project All plots requires the original None SU context")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if request.SourceKind != entry.sourceKind {
				selected := &s.contexts.selectedUnitSelections
				selected.mu.Lock()
				defer selected.mu.Unlock()
				draft := selected.entries[contextID]
				if entry.revision == math.MaxInt64 || draft.revision == math.MaxInt64 {
					return nil, errors.New("Long Vegetation report scope revision exhausted")
				}
				draft.revision++
				draft.build = nil
				selected.entries = map[string]selectedUnitSelectionEntry{contextID: draft}
				entry.revision++
				entry.sourceKind, entry.build = request.SourceKind, nil
				ledger.entries = map[string]vegetationProjectScopeEntry{contextID: entry}
			}
			return projectScopeDTO(contextID, owner, entry), nil
		})
	})
}

func (s *LongVegetationReportScopeService) SetLongVegetationProjectFields(ctx context.Context, contextID, requestJSON string) (*LongVegetationReportScope, error) {
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
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationReportScope, error) {
		return withProjectScopeInstall(ctx, plots, s.snapshot, func(owner *sqliteContext) (*LongVegetationReportScope, error) {
			ledger := &s.contexts.projectReportScopes
			ledger.mu.Lock()
			defer ledger.mu.Unlock()
			entry := cloneProjectScopeEntry(ledger.entries[contextID])
			if entry.sourceKind != vegetationProjectFieldsScope || entry.revision != expected {
				return nil, errors.New("project field draft requires its current explicit source2 scope/revision")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if entry.revision == math.MaxInt64 {
				return nil, errors.New("project field draft revision exhausted")
			}
			entry.revision++
			entry.fields, entry.build = append([]string{}, request.Fields...), nil
			ledger.entries = map[string]vegetationProjectScopeEntry{contextID: entry}
			return projectScopeDTO(contextID, owner, entry), nil
		})
	})
}

func (s *LongVegetationReportScopeService) BuildLongVegetationProjectScope(ctx context.Context, contextID, revision string) (*LongVegetationReportScope, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	expected, err := selectedUnitRevision(revision)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*LongVegetationReportScope, error) {
		entry := s.contexts.projectReportScopes.read(contextID)
		if entry.revision != expected {
			return nil, errors.New("project report scope changed; reload before Build")
		}
		if entry.revision == math.MaxInt64 {
			return nil, errors.New("project report Build revision exhausted")
		}
		entry.revision++
		build, err := withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (*LongVegetationProjectBuild, error) {
			tables, err := readProjectScopeTables(ctx, owner, tx, entry)
			if err != nil {
				return nil, err
			}
			config, err := plots.projects.preferences.snapshot()
			if err != nil {
				return nil, err
			}
			return buildProjectScope(ctx, contextID, owner, entry, tables, config)
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
		if _, err := withPublicationReadSnapshot(ctx, owner, s.snapshot, func(tx *sql.Tx) (struct{}, error) {
			tables, err := readProjectScopeTables(ctx, owner, tx, entry)
			if err != nil {
				return struct{}{}, err
			}
			config, err := plots.projects.preferences.snapshot()
			if err != nil {
				return struct{}{}, err
			}
			hash, err := projectScopeHash(contextID, owner, entry, tables, config)
			if err != nil {
				return struct{}{}, err
			}
			if hash != build.ApprovalHash {
				return struct{}{}, errors.New("project report source/configuration changed during Build; stale result was discarded")
			}
			return struct{}{}, nil
		}); err != nil {
			return nil, err
		}
		ledger := &s.contexts.projectReportScopes
		ledger.mu.Lock()
		defer ledger.mu.Unlock()
		if ledger.entries[contextID].revision != expected {
			return nil, errors.New("project report scope changed during Build; stale result was discarded")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry.build = build
		ledger.entries = map[string]vegetationProjectScopeEntry{contextID: cloneProjectScopeEntry(entry)}
		result := projectScopeDTO(contextID, owner, entry)
		result.BuildCurrent = true
		return result, nil
	})
}

func (s *ContextService) approvedProjectScope(ctx context.Context, contextID string, owner *sqliteContext,
	tx *sql.Tx, config configValues) (vegetationProjectScopeEntry, error) {
	entry := s.projectReportScopes.read(contextID)
	if entry.build == nil {
		return vegetationProjectScopeEntry{}, errors.New("project report scope has no explicit current session Build")
	}
	tables, err := readProjectScopeTables(ctx, owner, tx, entry)
	if err != nil {
		return vegetationProjectScopeEntry{}, err
	}
	build, err := buildProjectScope(ctx, contextID, owner, entry, tables, config)
	if err != nil {
		return vegetationProjectScopeEntry{}, err
	}
	if build.ApprovalHash != entry.build.ApprovalHash {
		return vegetationProjectScopeEntry{}, errors.New("project report source/configuration/scope changed since Build; rebuild before output")
	}
	return entry, nil
}
