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

type LongVegetationReportScope struct {
	ContextID    string                               `json:"contextId"`
	Project      string                               `json:"project"`
	ProjectPath  string                               `json:"projectPath"`
	SU           string                               `json:"su"`
	SUPath       string                               `json:"suPath"`
	SourceKind   string                               `json:"sourceKind"`
	Revision     string                               `json:"revision"`
	Fields       []string                             `json:"fields"`
	Build        *vprocore.LongVegetationProjectBuild `json:"build"`
	BuildCurrent bool                                 `json:"buildCurrent"`
}

type vegetationScopeRequest struct {
	Revision   string `json:"revision"`
	SourceKind string `json:"sourceKind"`
}

func (request *vegetationScopeRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationScopeRequest
	var decoded plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "Long Vegetation report scope", "revision", "sourceKind"); err != nil {
		return err
	}
	if decoded.SourceKind != vprocore.VegetationSelectedListScope && decoded.SourceKind != vprocore.VegetationProjectFieldsScope &&
		decoded.SourceKind != vprocore.VegetationProjectAllScope {
		return errors.New("Long Vegetation report scope requires its exact explicit source kind")
	}
	*request = vegetationScopeRequest(decoded)
	return nil
}

type LongVegetationReportScopeService struct {
	contexts      *vprocore.ContextService
	snapshot      vprocore.PublicationReadSnapshotHooks
	beforeInstall func()
}

func NewLongVegetationReportScopeService(contexts *vprocore.ContextService) *LongVegetationReportScopeService {
	return &LongVegetationReportScopeService{contexts: contexts}
}

func (s *LongVegetationReportScopeService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Long Vegetation report scope requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil ||
		s.contexts.Projects.Preferences == nil || s.contexts.Plots == nil {
		return errors.New("Long Vegetation report scope owned context/configuration is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func projectScopeDTO(contextID string, owner *vprocore.SqliteContext, entry vprocore.VegetationProjectScopeEntry) *LongVegetationReportScope {
	entry = vprocore.CloneProjectScopeEntry(entry)
	return &LongVegetationReportScope{ContextID: contextID, Project: owner.Selection.Project,
		ProjectPath: owner.Selection.ProjectPath, SU: owner.Selection.SU, SUPath: owner.Selection.SUPath,
		SourceKind: entry.SourceKind, Revision: strconv.FormatInt(entry.Revision, 10), Fields: entry.Fields, Build: entry.Build}
}

func readProjectScopeTables(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx,
	entry vprocore.VegetationProjectScopeEntry) ([]vprocore.ProjectMetadataTable, error) {
	if entry.SourceKind != vprocore.VegetationProjectFieldsScope && entry.SourceKind != vprocore.VegetationProjectAllScope {
		return nil, errors.New("project Build requires an explicit project report scope")
	}
	if entry.SourceKind == vprocore.VegetationProjectAllScope && owner.Selection.SU != "None" {
		return nil, errors.New("project All plots requires the original None SU context")
	}
	sources := []struct{ role, table string }{{"project", owner.Selection.Project + "_Env"}}
	if entry.SourceKind == vprocore.VegetationProjectFieldsScope {
		sources = append(sources, struct{ role, table string }{"project", owner.Selection.Project + "_Admin"})
		if owner.Selection.SU != "None" {
			sources = append(sources, struct{ role, table string }{"su", owner.Selection.SU + "_SU"})
		}
	}
	tables := make([]vprocore.ProjectMetadataTable, len(sources))
	for i, source := range sources {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, fmt.Errorf("project Build requires original physical table %s.%s", source.role, source.table)
		}
		var err error
		tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return nil, err
		}
	}
	return tables, nil
}

func projectScopeHash(contextID string, owner *vprocore.SqliteContext, entry vprocore.VegetationProjectScopeEntry,
	tables []vprocore.ProjectMetadataTable, config vprocore.ConfigValues) (string, error) {
	group := 2
	if entry.SourceKind == vprocore.VegetationProjectAllScope {
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
	if enforce && (entry.SourceKind == vprocore.VegetationProjectAllScope || owner.Selection.SU == "None") {
		return "", errors.New("project report quality requires actual original selected-SU authority")
	}
	material := struct {
		ContextID  string
		Selection  vprocore.DesktopSelection
		SourceKind string
		Revision   int64
		Fields     []string
		Tables     []vprocore.ProjectMetadataTable
		Config     vprocore.ConfigValues
	}{contextID, owner.Selection, entry.SourceKind, entry.Revision, entry.Fields, tables, planned}
	raw, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

func buildProjectScope(ctx context.Context, contextID string, owner *vprocore.SqliteContext, entry vprocore.VegetationProjectScopeEntry,
	tables []vprocore.ProjectMetadataTable, config vprocore.ConfigValues) (*vprocore.LongVegetationProjectBuild, error) {
	var inputs []vegetationProjectUnitInput
	if entry.SourceKind == vprocore.VegetationProjectFieldsScope {
		rows, err := planLongVegetationProjectSelectedUnits(ctx, tables[0], tables[1], entry.Fields)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			admin := row.AdminRowID
			inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, &admin, row.PlotNumber, row.SiteUnit})
		}
	} else if entry.SourceKind == vprocore.VegetationProjectAllScope {
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
	build := &vprocore.LongVegetationProjectBuild{ApprovalHash: hash, ProjectTuples: make([]vprocore.LongVegetationProjectUnit, len(inputs))}
	for i, row := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		build.ProjectTuples[i] = vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: row.AdminRowID, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit}
	}
	return build, nil
}

func (s *LongVegetationReportScopeService) GetLongVegetationReportScope(ctx context.Context, contextID string) (*LongVegetationReportScope, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationReportScope, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*LongVegetationReportScope, error) {
			entry := s.contexts.ProjectReportScopes.Read(contextID)
			result := projectScopeDTO(contextID, owner, entry)
			if entry.Build != nil {
				tables, err := readProjectScopeTables(ctx, owner, tx, entry)
				if err != nil {
					return nil, err
				}
				config, err := plots.Projects.Preferences.Snapshot()
				if err != nil {
					return nil, err
				}
				hash, err := projectScopeHash(contextID, owner, entry, tables, config)
				if err != nil {
					return nil, err
				}
				result.BuildCurrent = hash == entry.Build.ApprovalHash
			}
			return result, nil
		})
	})
}

func withProjectScopeInstall[T any](ctx context.Context, plots *vprocore.PlotService, hooks vprocore.PublicationReadSnapshotHooks,
	install func(*vprocore.SqliteContext) (T, error)) (T, error) {
	var zero T
	if _, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, hooks, func(*vprocore.SqliteContext, *sql.Tx) (struct{}, error) {
		return struct{}{}, nil
	}); err != nil {
		return zero, err
	}
	owner := plots.Projects.Sqlite
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return zero, err
	}
	defer owner.Mu.Unlock()
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
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
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationReportScope, error) {
		return withProjectScopeInstall(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext) (*LongVegetationReportScope, error) {
			ledger := &s.contexts.ProjectReportScopes
			ledger.Mu.Lock()
			defer ledger.Mu.Unlock()
			entry := vprocore.CloneProjectScopeEntry(ledger.Entries[contextID])
			if entry.Revision != expected {
				return nil, errors.New("Long Vegetation report scope changed; reload before switching")
			}
			if request.SourceKind == vprocore.VegetationProjectAllScope && owner.Selection.SU != "None" {
				return nil, errors.New("project All plots requires the original None SU context")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if request.SourceKind != entry.SourceKind {
				selected := &s.contexts.SelectedUnitSelections
				selected.Mu.Lock()
				defer selected.Mu.Unlock()
				draft := selected.Entries[contextID]
				if entry.Revision == math.MaxInt64 || draft.Revision == math.MaxInt64 {
					return nil, errors.New("Long Vegetation report scope revision exhausted")
				}
				draft.Revision++
				draft.Build = nil
				selected.Entries = map[string]vprocore.SelectedUnitSelectionEntry{contextID: draft}
				entry.Revision++
				entry.SourceKind, entry.Build = request.SourceKind, nil
				ledger.Entries = map[string]vprocore.VegetationProjectScopeEntry{contextID: entry}
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
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationReportScope, error) {
		return withProjectScopeInstall(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext) (*LongVegetationReportScope, error) {
			ledger := &s.contexts.ProjectReportScopes
			ledger.Mu.Lock()
			defer ledger.Mu.Unlock()
			entry := vprocore.CloneProjectScopeEntry(ledger.Entries[contextID])
			if entry.SourceKind != vprocore.VegetationProjectFieldsScope || entry.Revision != expected {
				return nil, errors.New("project field draft requires its current explicit source2 scope/revision")
			}
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if entry.Revision == math.MaxInt64 {
				return nil, errors.New("project field draft revision exhausted")
			}
			entry.Revision++
			entry.Fields, entry.Build = append([]string{}, request.Fields...), nil
			ledger.Entries = map[string]vprocore.VegetationProjectScopeEntry{contextID: entry}
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
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*LongVegetationReportScope, error) {
		entry := s.contexts.ProjectReportScopes.Read(contextID)
		if entry.Revision != expected {
			return nil, errors.New("project report scope changed; reload before Build")
		}
		if entry.Revision == math.MaxInt64 {
			return nil, errors.New("project report Build revision exhausted")
		}
		entry.Revision++
		build, err := vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*vprocore.LongVegetationProjectBuild, error) {
			tables, err := readProjectScopeTables(ctx, owner, tx, entry)
			if err != nil {
				return nil, err
			}
			config, err := plots.Projects.Preferences.Snapshot()
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
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if _, err := vprocore.WithPublicationReadSnapshot(ctx, owner, s.snapshot, func(tx *sql.Tx) (struct{}, error) {
			tables, err := readProjectScopeTables(ctx, owner, tx, entry)
			if err != nil {
				return struct{}{}, err
			}
			config, err := plots.Projects.Preferences.Snapshot()
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
		ledger := &s.contexts.ProjectReportScopes
		ledger.Mu.Lock()
		defer ledger.Mu.Unlock()
		if ledger.Entries[contextID].Revision != expected {
			return nil, errors.New("project report scope changed during Build; stale result was discarded")
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry.Build = build
		ledger.Entries = map[string]vprocore.VegetationProjectScopeEntry{contextID: vprocore.CloneProjectScopeEntry(entry)}
		result := projectScopeDTO(contextID, owner, entry)
		result.BuildCurrent = true
		return result, nil
	})
}

func ApprovedProjectScope(s *vprocore.ContextService, ctx context.Context, contextID string, owner *vprocore.SqliteContext,
	tx *sql.Tx, config vprocore.ConfigValues) (vprocore.VegetationProjectScopeEntry, error) {
	entry := s.ProjectReportScopes.Read(contextID)
	if entry.Build == nil {
		return vprocore.VegetationProjectScopeEntry{}, errors.New("project report scope has no explicit current session Build")
	}
	tables, err := readProjectScopeTables(ctx, owner, tx, entry)
	if err != nil {
		return vprocore.VegetationProjectScopeEntry{}, err
	}
	build, err := buildProjectScope(ctx, contextID, owner, entry, tables, config)
	if err != nil {
		return vprocore.VegetationProjectScopeEntry{}, err
	}
	if build.ApprovalHash != entry.Build.ApprovalHash {
		return vprocore.VegetationProjectScopeEntry{}, errors.New("project report source/configuration/scope changed since Build; rebuild before output")
	}
	return entry, nil
}
