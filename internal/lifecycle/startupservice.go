package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"gopkg.in/yaml.v3"
)

type StartupState struct {
	Ready               bool   `json:"ready"`
	Message             string `json:"message"`
	ConfigPath          string `json:"configPath"`
	ReleaseChannel      string `json:"releaseChannel"`
	BackupPath          string `json:"backupPath"`
	ConfiguredProject   string `json:"configuredProject"`
	ProjectPath         string `json:"projectPath"`
	ConfiguredSU        string `json:"configuredSu"`
	SUPath              string `json:"suPath"`
	ConfiguredHierarchy string `json:"configuredHierarchy"`
	HierarchyPath       string `json:"hierarchyPath"`
}

type StartupRecoveryRequest struct {
	ProjectPath    string `json:"projectPath"`
	ProjectName    string `json:"projectName"`
	SUPath         string `json:"suPath"`
	ClearSU        bool   `json:"clearSu"`
	HierarchyPath  string `json:"hierarchyPath"`
	ClearHierarchy bool   `json:"clearHierarchy"`
}

type StartupService struct {
	mu      sync.RWMutex
	State   StartupState                       `json:"-" yaml:"-"`
	DataDir string                             `json:"-" yaml:"-"`
	Now     func() time.Time                   `json:"-" yaml:"-"`
	Chooser func(context.Context) (string, error) `json:"-" yaml:"-"`
	Activate func() (StartupState, error)      `json:"-" yaml:"-"`
}

func (s *StartupService) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *StartupService) GetState() StartupState {
	if s == nil {
		return StartupState{ReleaseChannel: vprocore.ReleaseChannelStable}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *StartupService) SetState(next StartupState) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.State = next
	s.mu.Unlock()
}

// PopulateRecoveryHints extracts best-effort selection hints from an existing config.yml
// without mutating any files on disk.
func PopulateRecoveryHints(state *StartupState) {
	if state == nil || strings.TrimSpace(state.ConfigPath) == "" {
		return
	}
	raw, err := os.ReadFile(state.ConfigPath)
	if err != nil {
		return
	}
	values, err := vprocore.DecodeConfig(raw)
	if err != nil {
		return
	}
	if ch, ok := values["release_channel"].(string); ok && (ch == vprocore.ReleaseChannelStable || ch == vprocore.ReleaseChannelExperimental) {
		state.ReleaseChannel = ch
	}
	current, err := vprocore.ConfigSection(values, "Current")
	if err != nil {
		return
	}
	str := func(k string) string {
		v, _ := current[k].(string)
		return v
	}
	state.ConfiguredProject = str("CurrProject")
	state.ProjectPath = str("ProjectPath")
	state.ConfiguredSU = str("CurrPlotlist")
	state.SUPath = str("SUPath")
	state.ConfiguredHierarchy = str("CurrHierarchy")
	state.HierarchyPath = str("HierarchyPath")
}

// ChooseDatabaseFile opens a native file chooser for relocating a missing .db file.
func (s *StartupService) ChooseDatabaseFile(ctx context.Context) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	if s != nil && s.Chooser != nil {
		return s.Chooser(ctx)
	}
	path, err := vprocore.ChooseNativeSQLiteDatabaseFile(ctx, "Locate VPro SQLite Project Database")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(path), nil
}

// ResetToSample backs up the existing config.yml to config.backup-<timestamp>.yml,
// writes canonical Sample defaults, and activates the live application runtime.
func (s *StartupService) ResetToSample(ctx context.Context) (StartupState, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return StartupState{}, err
		}
	}
	if s == nil {
		return StartupState{}, errors.New("startup service is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State.Ready {
		return s.State, nil
	}
	configPath := strings.TrimSpace(s.State.ConfigPath)
	if configPath == "" {
		return StartupState{}, errors.New("startup configuration path is unavailable")
	}
	priorBytes, priorReadErr := os.ReadFile(configPath)
	hadPrior := priorReadErr == nil

	backupPath, err := vprocore.BackupDesktopConfigFile(configPath, s.now())
	if err != nil {
		return StartupState{}, err
	}

	defaults, err := vprocore.DecodeConfig(vprocore.DesktopDefaults)
	if err != nil {
		return StartupState{}, err
	}
	defaults["Desktop"] = map[string]any{
		"SchemaVersion": 1,
		"LegacyImports": vprocore.StampLegacyImportHashes(s.DataDir, filepath.Dir(configPath)),
	}
	if err := vprocore.ValidateDesktopConfig(defaults); err != nil {
		return StartupState{}, err
	}
	encoded, err := yaml.Marshal(defaults)
	if err != nil {
		return StartupState{}, err
	}
	if err := vprocore.WriteDesktopConfigAtomically(configPath, encoded); err != nil {
		return StartupState{}, fmt.Errorf("write Sample recovery config.yml: %w", err)
	}

	if s.Activate != nil {
		activated, actErr := s.Activate()
		if actErr != nil {
			if hadPrior {
				_ = vprocore.WriteDesktopConfigAtomically(configPath, priorBytes)
			}
			return StartupState{}, fmt.Errorf("activate Sample project: %w", actErr)
		}
		activated.BackupPath = backupPath
		s.State = activated
		return s.State, nil
	}

	s.State = StartupState{
		Ready:          true,
		ConfigPath:     configPath,
		ReleaseChannel: vprocore.ReleaseChannelStable,
		BackupPath:     backupPath,
	}
	return s.State, nil
}

// RecoverWithPaths validates the relocated SQLite project path, backs up config.yml
// to config.backup-<timestamp>.yml, updates missing paths cleanly, and activates the runtime.
func (s *StartupService) RecoverWithPaths(ctx context.Context, req StartupRecoveryRequest) (StartupState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return StartupState{}, err
	}
	if s == nil {
		return StartupState{}, errors.New("startup service is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State.Ready {
		return s.State, nil
	}
	configPath := strings.TrimSpace(s.State.ConfigPath)
	if configPath == "" {
		return StartupState{}, errors.New("startup configuration path is unavailable")
	}

	rawProjPath := strings.TrimSpace(req.ProjectPath)
	if rawProjPath == "" {
		return StartupState{}, errors.New("select or enter an existing SQLite project database (.db) path")
	}
	absProjPath, err := filepath.Abs(rawProjPath)
	if err != nil {
		return StartupState{}, fmt.Errorf("resolve project database path: %w", err)
	}
	resolvedProjPath, err := vprocore.ExistingDatabasePath(absProjPath)
	if err != nil {
		return StartupState{}, fmt.Errorf("project database unavailable: %w", err)
	}
	inspected, err := vprocore.InspectFileContext(ctx, resolvedProjPath)
	if err != nil {
		return StartupState{}, fmt.Errorf("inspect project database: %w", err)
	}
	var compatible []vprocore.ProjectInfo
	for _, p := range inspected {
		if p.Compatible {
			compatible = append(compatible, p)
		}
	}
	if len(compatible) == 0 {
		return StartupState{}, fmt.Errorf("no compatible VP08 project tables (<Project>_Env..Admin) found in %s", resolvedProjPath)
	}

	priorBytes, priorReadErr := os.ReadFile(configPath)
	hadPrior := priorReadErr == nil

	var values vprocore.ConfigValues
	if hadPrior {
		if decoded, decErr := vprocore.DecodeConfig(priorBytes); decErr == nil {
			values = decoded
		}
	}
	if values == nil {
		defaults, defErr := vprocore.DecodeConfig(vprocore.DesktopDefaults)
		if defErr != nil {
			return StartupState{}, defErr
		}
		values = defaults
	}
	stampedImports := vprocore.StampLegacyImportHashes(s.DataDir, filepath.Dir(configPath))
	if _, ok := values["Desktop"].(map[string]any); !ok {
		values["Desktop"] = map[string]any{
			"SchemaVersion": 1,
			"LegacyImports": stampedImports,
		}
	} else {
		desktop := values["Desktop"].(map[string]any)
		desktop["SchemaVersion"] = 1
		desktop["LegacyImports"] = stampedImports
		// Drop invalid/unreachable custom DatabasePaths so standard family databases can open cleanly
		if rawDBPaths, hasDBPaths := desktop["DatabasePaths"].(map[string]any); hasDBPaths {
			for role, v := range rawDBPaths {
				pStr, ok := v.(string)
				if !ok || !filepath.IsAbs(pStr) {
					delete(rawDBPaths, role)
					continue
				}
				if info, stErr := os.Stat(pStr); stErr != nil || !info.Mode().IsRegular() {
					delete(rawDBPaths, role)
				}
			}
			if len(rawDBPaths) == 0 {
				delete(desktop, "DatabasePaths")
			}
		}
	}

	defaults, err := vprocore.DecodeConfig(vprocore.DesktopDefaults)
	if err != nil {
		return StartupState{}, err
	}
	for sec, defVal := range defaults {
		if _, exists := values[sec]; !exists {
			values[sec] = defVal
			continue
		}
		if defMap, isMap := defVal.(map[string]any); isMap {
			if curMap, ok := values[sec].(map[string]any); ok {
				for k, v := range defMap {
					if _, exists := curMap[k]; !exists {
						curMap[k] = v
					}
				}
			} else {
				values[sec] = defMap
			}
		}
	}

	current := values["Current"].(map[string]any)
	requestedName := strings.TrimSpace(req.ProjectName)
	chosenProject := ""
	if requestedName != "" {
		for _, p := range compatible {
			if p.Name == requestedName {
				chosenProject = p.Name
				break
			}
		}
		if chosenProject == "" {
			return StartupState{}, fmt.Errorf("project %q was not found as a compatible VP08 project in %s", requestedName, resolvedProjPath)
		}
	} else {
		cfgProj, _ := current["CurrProject"].(string)
		for _, p := range compatible {
			if p.Name == cfgProj {
				chosenProject = p.Name
				break
			}
		}
		if chosenProject == "" {
			if len(compatible) == 1 {
				chosenProject = compatible[0].Name
			} else {
				names := make([]string, 0, len(compatible))
				for _, p := range compatible {
					names = append(names, p.Name)
				}
				return StartupState{}, fmt.Errorf("database contains multiple VP08 projects (%s); enter the target project name", strings.Join(names, ", "))
			}
		}
	}

	current["CurrProject"] = chosenProject
	current["ProjectPath"] = resolvedProjPath

	if req.ClearSU {
		current["CurrPlotlist"] = "None"
		current["SUPath"] = ""
	} else if suTrimmed := strings.TrimSpace(req.SUPath); suTrimmed != "" {
		absSU, absErr := filepath.Abs(suTrimmed)
		if absErr != nil {
			return StartupState{}, fmt.Errorf("resolve SU path: %w", absErr)
		}
		resolvedSU, exErr := vprocore.ExistingDatabasePath(absSU)
		if exErr != nil {
			return StartupState{}, fmt.Errorf("SU database unavailable: %w", exErr)
		}
		current["SUPath"] = resolvedSU
	}

	if req.ClearHierarchy {
		current["CurrHierarchy"] = "None"
		current["HierarchyPath"] = ""
	} else if hierTrimmed := strings.TrimSpace(req.HierarchyPath); hierTrimmed != "" {
		absHier, absErr := filepath.Abs(hierTrimmed)
		if absErr != nil {
			return StartupState{}, fmt.Errorf("resolve Hierarchy path: %w", absErr)
		}
		resolvedHier, exErr := vprocore.ExistingDatabasePath(absHier)
		if exErr != nil {
			return StartupState{}, fmt.Errorf("Hierarchy database unavailable: %w", exErr)
		}
		current["HierarchyPath"] = resolvedHier
	}

	// Clear unreachable explicit ProfilePath so a missing external profile does not block recovery
	if profPath, ok := current["ProfilePath"].(string); ok && strings.TrimSpace(profPath) != "" {
		if info, stErr := os.Stat(profPath); stErr != nil || !info.Mode().IsRegular() {
			delete(current, "ProfilePath")
			current["CurrVegProfile"] = chosenProject
		}
	}

	if user, _ := current["User"].(string); strings.TrimSpace(user) == "" {
		current["User"] = "Admin"
	}
	if ch, hasCh := values["release_channel"]; hasCh {
		if chStr, ok := ch.(string); !ok || (chStr != vprocore.ReleaseChannelStable && chStr != vprocore.ReleaseChannelExperimental) {
			values["release_channel"] = vprocore.ReleaseChannelStable
		}
	}

	if err := vprocore.ValidateDesktopConfig(values); err != nil {
		return StartupState{}, fmt.Errorf("validate recovered configuration: %w", err)
	}
	encoded, err := yaml.Marshal(values)
	if err != nil {
		return StartupState{}, err
	}

	backupPath, err := vprocore.BackupDesktopConfigFile(configPath, s.now())
	if err != nil {
		return StartupState{}, err
	}
	if err := vprocore.WriteDesktopConfigAtomically(configPath, encoded); err != nil {
		return StartupState{}, fmt.Errorf("write recovered config.yml: %w", err)
	}

	if s.Activate != nil {
		activated, actErr := s.Activate()
		if actErr != nil {
			if hadPrior {
				_ = vprocore.WriteDesktopConfigAtomically(configPath, priorBytes)
			}
			return StartupState{}, actErr
		}
		activated.BackupPath = backupPath
		s.State = activated
		return s.State, nil
	}

	next := StartupState{
		Ready:          true,
		ConfigPath:     configPath,
		ReleaseChannel: vprocore.ReleaseChannelStable,
		BackupPath:     backupPath,
	}
	PopulateRecoveryHints(&next)
	s.State = next
	return s.State, nil
}
