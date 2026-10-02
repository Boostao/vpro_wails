package main

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed resources/Sample.db
var sampleFiles embed.FS

var projectNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,30}$`)
var coreTables = []string{"Admin", "Audit", "Env", "Humus", "Metadata", "Mineral", "Other", "Veg"}

type ProjectInfo struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Version    string `json:"version"`
	Compatible bool   `json:"compatible"`
	Path       string `json:"path,omitempty"`
}

type ProjectState struct {
	ActiveProject   string                `json:"activeProject"`
	ActiveSU        string                `json:"activeSU"`
	ActiveHierarchy string                `json:"activeHierarchy"`
	HierarchyFile   string                `json:"hierarchyFile"`
	Projects        []ProjectInfo         `json:"projects"`
	SUs             []SUInfo              `json:"sus"`
	Hierarchies     []HierarchyInfo       `json:"hierarchies"`
	Diagnostics     []ProjectDiagnostic   `json:"diagnostics"`
	ContextID       string                `json:"contextId,omitempty"`
	ProjectPath     string                `json:"projectPath,omitempty"`
	SUPath          string                `json:"suPath,omitempty"`
	HierarchyPath   string                `json:"hierarchyPath,omitempty"`
	PlotProfile     PlotProfileSourceInfo `json:"plotProfile"`
}

type SUInfo struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Compatible bool   `json:"compatible"`
	Path       string `json:"path,omitempty"`
}

type ProjectDiagnostic struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type PlotSummary struct {
	PlotNumber       string  `json:"plotNumber"`
	FieldNumber      *string `json:"fieldNumber"`
	PlotRepresenting *string `json:"plotRepresenting"`
	Zone             *string `json:"zone"`
	SubZone          *string `json:"subZone"`
	SiteSeries       *string `json:"siteSeries"`
}

type PlotPage struct {
	Total int           `json:"total"`
	Plots []PlotSummary `json:"plots"`
}

type ProjectService struct {
	operationMu     sync.RWMutex
	mu              sync.RWMutex
	root            string
	config          string
	active          string
	activeSU        string
	activeHierarchy string
	hierarchyFile   string
	preferences     *desktopConfig
	sqlite          *sqliteContext
	supportPaths    map[string]string
	contextID       string
}

func userDataDir() (string, error) {
	if directory := os.Getenv("VPRO_DATA_DIR"); directory != "" {
		return filepath.Abs(directory)
	}
	if runtime.GOOS == "windows" {
		if directory := os.Getenv("LOCALAPPDATA"); directory != "" {
			return filepath.Join(directory, "vpro"), nil
		}
	}
	if runtime.GOOS == "darwin" {
		directory, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(directory, "vpro"), nil
	}
	if runtime.GOOS == "linux" {
		if directory := os.Getenv("XDG_DATA_HOME"); directory != "" {
			return filepath.Join(directory, "vpro"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "vpro"), nil
	}
	return filepath.Join(home, ".local", "share", "vpro"), nil
}

func userConfigDir() (string, error) {
	if directory := os.Getenv("VPRO_CONFIG_DIR"); directory != "" {
		return filepath.Abs(directory)
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "vpro"), nil
}

func NewProjectService(root string) (*ProjectService, error) {
	return NewProjectServiceWithConfig(root, root)
}

func NewProjectServiceWithConfig(root, config string) (*ProjectService, error) {
	return newProjectServiceWithPreferences(root, config, nil)
}

func newProjectServiceWithPreferences(root, config string, preferences *desktopConfig) (*ProjectService, error) {
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0700); err != nil {
		return nil, err
	}
	if err := installSample(filepath.Join(root, "projects", "Sample.db")); err != nil {
		return nil, err
	}
	service := &ProjectService{root: root, config: config, active: "Sample", activeSU: "None", activeHierarchy: "None"}
	service.preferences = preferences
	var configured desktopSelection
	var settings []byte
	var err error
	if preferences != nil {
		configured, err = preferences.selection()
		if err != nil {
			return nil, err
		}
		service.active, service.activeSU, service.activeHierarchy = configured.Project, configured.SU, configured.Hierarchy
		if configured.HierarchyPath != "" {
			path, err := filepath.Abs(configured.HierarchyPath)
			if err != nil {
				return nil, err
			}
			directory, err := filepath.Abs(filepath.Join(root, "projects"))
			if err != nil {
				return nil, err
			}
			if !sameDesktopPath(filepath.Dir(path), directory) {
				return nil, errors.New("configured external hierarchy path requires foundation F2; YAML was retained")
			}
			service.hierarchyFile = filepath.Base(path)
		}
	} else {
		settings, err = os.ReadFile(filepath.Join(config, "desktop-selection.json"))
	}
	if errors.Is(err, os.ErrNotExist) && config != root {
		settings, err = os.ReadFile(filepath.Join(root, "desktop-selection.json"))
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(settings) > 0 {
		var stored struct {
			ActiveProject   string `json:"activeProject"`
			ActiveSU        string `json:"activeSU"`
			ActiveHierarchy string `json:"activeHierarchy"`
			HierarchyFile   string `json:"hierarchyFile"`
		}
		if err := json.Unmarshal(settings, &stored); err != nil {
			return nil, fmt.Errorf("invalid desktop selection: %w", err)
		}
		if stored.ActiveProject != "" {
			service.active = stored.ActiveProject
		}
		if stored.ActiveSU != "" {
			service.activeSU = stored.ActiveSU
		}
		if stored.ActiveHierarchy != "" {
			service.activeHierarchy = stored.ActiveHierarchy
			service.hierarchyFile = stored.HierarchyFile
		}
	}
	projects, _, err := service.discover()
	if err != nil {
		return nil, err
	}
	if !hasCompatible(projects, service.active) {
		if preferences != nil {
			return nil, fmt.Errorf("configured project %q is unavailable; YAML selection was retained", service.active)
		}
		service.active = "Sample"
		service.activeSU = "None"
		if len(settings) > 0 {
			if err := service.saveSelection("Sample", "None"); err != nil {
				return nil, err
			}
		}
	} else if service.activeSU != "None" {
		selected := compatibleProject(projects, service.active)
		sus, err := service.discoverSUs(selected)
		if err != nil {
			return nil, err
		}
		if !hasCompatibleSU(sus, service.activeSU) {
			if preferences != nil {
				return nil, fmt.Errorf("configured SU %q is unavailable; YAML selection was retained", service.activeSU)
			}
			service.activeSU = "None"
			if err := service.saveSelection(service.active, "None"); err != nil {
				return nil, err
			}
		}
	}
	if preferences != nil {
		selected := compatibleProject(projects, service.active)
		expected, err := filepath.Abs(filepath.Join(root, "projects", selected.File))
		if err != nil {
			return nil, err
		}
		for _, path := range []string{configured.ProjectPath, configured.SUPath} {
			if path == "" {
				continue
			}
			actual, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			if !sameDesktopPath(actual, expected) {
				return nil, errors.New("configured external project/SU path requires foundation F2; YAML was retained")
			}
		}
	}
	if service.activeHierarchy != "None" {
		hierarchies, err := discoverHierarchies(filepath.Join(root, "projects"))
		if err != nil {
			return nil, err
		}
		if preferences != nil && service.hierarchyFile == "" {
			for _, candidate := range hierarchies {
				if candidate.Name == service.activeHierarchy && candidate.Compatible {
					if service.hierarchyFile != "" {
						return nil, errors.New("configured hierarchy name is ambiguous; YAML was retained")
					}
					service.hierarchyFile = candidate.File
				}
			}
		}
		if !hasCompatibleHierarchy(hierarchies, service.activeHierarchy, service.hierarchyFile) {
			if preferences != nil {
				return nil, fmt.Errorf("configured hierarchy %q is unavailable; YAML selection was retained", service.activeHierarchy)
			}
			service.activeHierarchy = "None"
			service.hierarchyFile = ""
			if err := service.saveSelection(service.active, service.activeSU); err != nil {
				return nil, err
			}
		}
	}
	if preferences != nil && (configured.ProjectPath == "" ||
		(service.activeHierarchy != "None" && configured.HierarchyPath == "")) {
		path, err := filepath.Abs(filepath.Join(root, "projects", compatibleProject(projects, service.active).File))
		if err != nil {
			return nil, err
		}
		paths := map[string]any{"ProjectPath": path}
		if service.activeHierarchy != "None" && configured.HierarchyPath == "" {
			hierarchyPath, err := filepath.Abs(filepath.Join(root, "projects", service.hierarchyFile))
			if err != nil {
				return nil, err
			}
			paths["HierarchyPath"] = hierarchyPath
		}
		if err := preferences.update("Current", paths); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func sameDesktopPath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func installSample(destination string) error {
	if _, err := os.Stat(destination); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	source, err := sampleFiles.Open("resources/Sample.db")
	if err != nil {
		return err
	}
	defer source.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".sample-*.db")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := io.Copy(temporary, source); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporary.Name(), destination); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

func hasCompatible(projects []ProjectInfo, name string) bool {
	for _, project := range projects {
		if project.Name == name && project.Compatible {
			return true
		}
	}
	return false
}

func compatibleProject(projects []ProjectInfo, name string) ProjectInfo {
	for _, project := range projects {
		if project.Name == name && project.Compatible {
			return project
		}
	}
	return ProjectInfo{}
}

func hasCompatibleSU(sus []SUInfo, name string) bool {
	for _, su := range sus {
		if su.Name == name && su.Compatible {
			return true
		}
	}
	return false
}

func hasCompatibleHierarchy(hierarchies []HierarchyInfo, name, file string) bool {
	for _, hierarchy := range hierarchies {
		if hierarchy.Name == name && hierarchy.File == file && hierarchy.Compatible {
			return true
		}
	}
	return false
}

func (service *ProjectService) discoverSUs(project ProjectInfo) ([]SUInfo, error) {
	return service.discoverSUsContext(context.Background(), project)
}

func (service *ProjectService) discoverSUsContext(ctx context.Context, project ProjectInfo) ([]SUInfo, error) {
	database, err := openReadOnlyContext(ctx, service.projectFile(project))
	if err != nil {
		return nil, err
	}
	defer database.Close()
	reader := contextPlotDB{db: database, ctx: ctx}
	rows, err := reader.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE '%_SU'")
	if err != nil {
		return nil, err
	}
	names := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return nil, err
		}
		if name, ok := strings.CutSuffix(table, "_SU"); ok && projectNamePattern.MatchString(name) {
			names = append(names, name)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	var hasPolicy bool
	if err := reader.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = '_vpro_su_policy')").Scan(&hasPolicy); err != nil {
		return nil, err
	}
	sus := make([]SUInfo, 0, len(names))
	for _, name := range names {
		su := SUInfo{Name: name, Kind: "ordinary"}
		if hasPolicy {
			err := reader.QueryRow("SELECT kind FROM _vpro_su_policy WHERE table_name = ?", name+"_SU").Scan(&su.Kind)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if errors.Is(err, sql.ErrNoRows) {
				su.Kind = "ordinary"
			}
		}
		fields, err := reader.Query(`PRAGMA table_info("` + name + `_SU")`)
		if err != nil {
			return nil, err
		}
		valid := map[string]bool{}
		for fields.Next() {
			var index, notNull, primaryKey int
			var field, fieldType string
			var defaultValue sql.NullString
			if err := fields.Scan(&index, &field, &fieldType, &notNull, &defaultValue, &primaryKey); err != nil {
				fields.Close()
				return nil, err
			}
			upper := strings.ToUpper(fieldType)
			valid[field] = strings.Contains(upper, "CHAR") || strings.Contains(upper, "CLOB") || strings.Contains(upper, "TEXT")
		}
		err = fields.Err()
		fields.Close()
		if err != nil {
			return nil, err
		}
		su.Compatible = valid["PlotNumber"] && valid["SiteUnit"] && (su.Kind == "ordinary" || su.Kind == "working")
		sus = append(sus, su)
	}
	return sus, nil
}

func openReadOnly(path string) (*sql.DB, error) {
	return openReadOnlyContext(context.Background(), path)
}

func openReadOnlyContext(ctx context.Context, path string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite3", sqliteFileURI(absolute, "ro"))
	if err == nil {
		database.SetMaxOpenConns(1)
		err = database.PingContext(ctx)
	}
	if err != nil {
		if database != nil {
			database.Close()
		}
		return nil, err
	}
	return database, nil
}

func inspectFile(path string) ([]ProjectInfo, error) {
	return inspectFileContext(context.Background(), path)
}

func inspectFileContext(ctx context.Context, path string) ([]ProjectInfo, error) {
	database, err := openReadOnlyContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	reader := contextPlotDB{db: database, ctx: ctx}
	rows, err := reader.Query("SELECT name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		tables[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	projects := []ProjectInfo{}
	for table := range tables {
		name, ok := strings.CutSuffix(table, "_Env")
		if !ok || table == "Filtered_Env" || table == "USysEnv" || !projectNamePattern.MatchString(name) {
			continue
		}
		project := ProjectInfo{Name: name, File: filepath.Base(path), Version: "Unknown"}
		complete := true
		for _, suffix := range coreTables {
			complete = complete && tables[name+"_"+suffix]
		}
		if tables["_table_metadata"] {
			var version sql.NullString
			err := reader.QueryRow("SELECT description FROM _table_metadata WHERE table_name = ?", table).Scan(&version)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if version.Valid && version.String != "" {
				project.Version = version.String
			}
		}
		project.Compatible = complete && project.Version == "VP08"
		projects = append(projects, project)
	}
	slices.SortFunc(projects, func(left, right ProjectInfo) int { return strings.Compare(left.Name, right.Name) })
	return projects, nil
}

func (service *ProjectService) discover() ([]ProjectInfo, []ProjectDiagnostic, error) {
	return service.discoverContext(context.Background())
}

func (service *ProjectService) discoverContext(ctx context.Context) ([]ProjectInfo, []ProjectDiagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	files, err := os.ReadDir(filepath.Join(service.root, "projects"))
	if err != nil {
		return nil, nil, err
	}
	projects := []ProjectInfo{}
	diagnostics := []ProjectDiagnostic{}
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if !file.Type().IsRegular() || !strings.EqualFold(filepath.Ext(file.Name()), ".db") {
			continue
		}
		found, err := inspectFileContext(ctx, filepath.Join(service.root, "projects", file.Name()))
		if err != nil {
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				return nil, nil, ctx.Err()
			}
			if file.Name() == "Sample.db" {
				return nil, nil, labelledReadError("inspect "+file.Name(), err)
			}
			diagnostics = append(diagnostics, ProjectDiagnostic{File: file.Name(), Message: err.Error()})
			continue
		}
		projects = append(projects, found...)
	}
	slices.SortFunc(projects, func(left, right ProjectInfo) int { return strings.Compare(left.Name, right.Name) })
	for index := 1; index < len(projects); index++ {
		if projects[index].Name == projects[index-1].Name {
			return nil, nil, fmt.Errorf("duplicate project family %q", projects[index].Name)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return projects, diagnostics, nil
}

func (service *ProjectService) GetState(ctx context.Context) (ProjectState, error) {
	if err := acquireReadLease(ctx, &service.mu); err != nil {
		return ProjectState{}, err
	}
	defer service.mu.RUnlock()
	if service.sqlite != nil {
		return service.sqliteStateContextLocked(ctx)
	}
	projects, diagnostics, err := service.discoverContext(ctx)
	if err != nil {
		return ProjectState{}, err
	}
	active := service.active
	activeSU := service.activeSU
	activeHierarchy := service.activeHierarchy
	hierarchyFile := service.hierarchyFile
	selected := compatibleProject(projects, active)
	if selected.Name == "" {
		return ProjectState{}, errors.New("no compatible project is active")
	}
	sus, err := service.discoverSUsContext(ctx, selected)
	if err != nil {
		return ProjectState{}, err
	}
	hierarchies, err := discoverHierarchiesContext(ctx, filepath.Join(service.root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	return ProjectState{ActiveProject: active, ActiveSU: activeSU, ActiveHierarchy: activeHierarchy, HierarchyFile: hierarchyFile, Projects: projects, SUs: sus, Hierarchies: hierarchies, Diagnostics: diagnostics}, nil
}

func (service *ProjectService) SelectProject(name string) (ProjectState, error) {
	service.mu.Lock()
	if service.sqlite != nil {
		defer service.mu.Unlock()
		return ProjectState{}, errors.New("use an identity-bound context switch for the active SQLite application")
	}
	service.mu.Unlock()
	if !projectNamePattern.MatchString(name) {
		return ProjectState{}, errors.New("invalid VPRO project name")
	}
	projects, diagnostics, err := service.discover()
	if err != nil {
		return ProjectState{}, err
	}
	if !hasCompatible(projects, name) {
		return ProjectState{}, fmt.Errorf("project %q is not a complete VP08 family", name)
	}
	sus, err := service.discoverSUs(compatibleProject(projects, name))
	if err != nil {
		return ProjectState{}, err
	}
	hierarchies, err := discoverHierarchies(filepath.Join(service.root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if err := service.saveSelection(name, "None"); err != nil {
		return ProjectState{}, err
	}
	service.active = name
	service.activeSU = "None"
	return ProjectState{ActiveProject: name, ActiveSU: "None", ActiveHierarchy: service.activeHierarchy, HierarchyFile: service.hierarchyFile, Projects: projects, SUs: sus, Hierarchies: hierarchies, Diagnostics: diagnostics}, nil
}

func (service *ProjectService) SelectSU(name string) (ProjectState, error) {
	service.mu.Lock()
	if service.sqlite != nil {
		defer service.mu.Unlock()
		return ProjectState{}, errors.New("use an identity-bound context switch for the active SQLite application")
	}
	service.mu.Unlock()
	if name != "None" && !projectNamePattern.MatchString(name) {
		return ProjectState{}, errors.New("invalid VPRO SU name")
	}
	projects, diagnostics, err := service.discover()
	if err != nil {
		return ProjectState{}, err
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	selected := compatibleProject(projects, service.active)
	if selected.Name == "" {
		return ProjectState{}, errors.New("no compatible project is active")
	}
	sus, err := service.discoverSUs(selected)
	if err != nil {
		return ProjectState{}, err
	}
	hierarchies, err := discoverHierarchies(filepath.Join(service.root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	if name != "None" && !hasCompatibleSU(sus, name) {
		return ProjectState{}, fmt.Errorf("SU %q is not an available, authorized site unit in this project file", name)
	}
	if err := service.saveSelection(service.active, name); err != nil {
		return ProjectState{}, err
	}
	service.activeSU = name
	return ProjectState{ActiveProject: service.active, ActiveSU: name, ActiveHierarchy: service.activeHierarchy, HierarchyFile: service.hierarchyFile, Projects: projects, SUs: sus, Hierarchies: hierarchies, Diagnostics: diagnostics}, nil
}

func (service *ProjectService) SelectHierarchy(name, file string) (ProjectState, error) {
	service.mu.Lock()
	if service.sqlite != nil {
		defer service.mu.Unlock()
		return ProjectState{}, errors.New("use an identity-bound context switch for the active SQLite application")
	}
	service.mu.Unlock()
	if name != "None" && !projectNamePattern.MatchString(name) {
		return ProjectState{}, errors.New("invalid VPRO hierarchy name")
	}
	projects, diagnostics, err := service.discover()
	if err != nil {
		return ProjectState{}, err
	}
	hierarchies, err := discoverHierarchies(filepath.Join(service.root, "projects"))
	if err != nil {
		return ProjectState{}, err
	}
	if name == "None" {
		if file != "" {
			return ProjectState{}, errors.New("None hierarchy cannot have a source file")
		}
	} else if !hasCompatibleHierarchy(hierarchies, name, file) {
		return ProjectState{}, fmt.Errorf("hierarchy %q in %q is not an available hierarchy", name, file)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	selected := compatibleProject(projects, service.active)
	if selected.Name == "" {
		return ProjectState{}, errors.New("no compatible project is active")
	}
	sus, err := service.discoverSUs(selected)
	if err != nil {
		return ProjectState{}, err
	}
	if err := service.saveSelectionWithHierarchy(service.active, service.activeSU, name, file); err != nil {
		return ProjectState{}, err
	}
	service.activeHierarchy = name
	service.hierarchyFile = file
	return ProjectState{ActiveProject: service.active, ActiveSU: service.activeSU, ActiveHierarchy: name, HierarchyFile: file, Projects: projects, SUs: sus, Hierarchies: hierarchies, Diagnostics: diagnostics}, nil
}

func (service *ProjectService) GetHierarchyNodes(ctx context.Context) ([]HierarchyNode, error) {
	if err := acquireReadLease(ctx, &service.mu); err != nil {
		return nil, err
	}
	if service.sqlite != nil {
		defer service.mu.RUnlock()
		return service.sqliteHierarchyLocked(ctx)
	}
	service.mu.RUnlock()
	service.mu.RLock()
	name, file := service.activeHierarchy, service.hierarchyFile
	service.mu.RUnlock()
	if name == "None" {
		return []HierarchyNode{}, nil
	}
	hierarchies, err := discoverHierarchies(filepath.Join(service.root, "projects"))
	if err != nil {
		return nil, err
	}
	if !hasCompatibleHierarchy(hierarchies, name, file) {
		return nil, errors.New("active hierarchy is no longer available")
	}
	return listHierarchyNodesContext(ctx, filepath.Join(service.root, "projects", file), name)
}

func (service *ProjectService) saveSelection(name, su string) error {
	return service.saveSelectionWithHierarchy(name, su, service.activeHierarchy, service.hierarchyFile)
}

func (service *ProjectService) saveSelectionWithHierarchy(name, su, hierarchy, file string) error {
	if service.preferences != nil {
		projects, _, err := service.discover()
		if err != nil {
			return err
		}
		selected := compatibleProject(projects, name)
		if selected.Name == "" {
			return errors.New("cannot persist an unavailable project")
		}
		path, err := filepath.Abs(filepath.Join(service.root, "projects", selected.File))
		if err != nil {
			return err
		}
		suPath, hierarchyPath := "", ""
		if su != "None" {
			suPath = path
		}
		if hierarchy != "None" {
			hierarchyPath, err = filepath.Abs(filepath.Join(service.root, "projects", file))
			if err != nil {
				return err
			}
		}
		return service.preferences.update("Current", map[string]any{
			"CurrProject": name, "ProjectPath": path, "CurrPlotlist": su, "SUPath": suPath,
			"CurrHierarchy": hierarchy, "HierarchyPath": hierarchyPath,
		})
	}
	if su == "None" {
		su = ""
	}
	if hierarchy == "None" {
		hierarchy = ""
		file = ""
	}
	settings, err := json.Marshal(struct {
		ActiveProject   string `json:"activeProject"`
		ActiveSU        string `json:"activeSU,omitempty"`
		ActiveHierarchy string `json:"activeHierarchy,omitempty"`
		HierarchyFile   string `json:"hierarchyFile,omitempty"`
	}{name, su, hierarchy, file})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(service.config, 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(service.config, ".selection-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(settings); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary.Name(), filepath.Join(service.config, "desktop-selection.json")); err != nil {
		return err
	}
	return nil
}

func (service *ProjectService) ListPlots(ctx context.Context, offset, limit int) (PlotPage, error) {
	if err := ctx.Err(); err != nil {
		return PlotPage{}, err
	}
	if offset < 0 || limit < 1 || limit > 200 {
		return PlotPage{}, errors.New("plot page requires a nonnegative offset and a limit from 1 to 200")
	}
	if err := acquireReadLease(ctx, &service.mu); err != nil {
		return PlotPage{}, err
	}
	if service.sqlite != nil {
		defer service.mu.RUnlock()
		return service.sqlitePlotsLocked(ctx, offset, limit)
	}
	service.mu.RUnlock()
	service.mu.RLock()
	active := service.active
	activeSU := service.activeSU
	service.mu.RUnlock()
	projects, _, err := service.discover()
	if err != nil {
		return PlotPage{}, err
	}
	selected := compatibleProject(projects, active)
	if selected.Name == "" {
		return PlotPage{}, errors.New("no compatible project is active")
	}
	database, err := openReadOnly(filepath.Join(service.root, "projects", selected.File))
	if err != nil {
		return PlotPage{}, err
	}
	defer database.Close()
	table := `"` + active + `_Env"`
	from := " FROM " + table + ` AS env WHERE EXISTS (SELECT 1 FROM "` + active + `_Admin" AS admin WHERE admin.Plot = env.PlotNumber)`
	if activeSU != "None" {
		sus, err := service.discoverSUs(selected)
		if err != nil {
			return PlotPage{}, err
		}
		if !hasCompatibleSU(sus, activeSU) {
			return PlotPage{}, errors.New("active SU is no longer available")
		}
		from += ` AND EXISTS (SELECT 1 FROM "` + activeSU + `_SU" AS su WHERE su.PlotNumber = env.PlotNumber)`
	}
	page := PlotPage{Plots: []PlotSummary{}}
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*)"+from).Scan(&page.Total); err != nil {
		return PlotPage{}, err
	}
	rows, err := database.QueryContext(ctx, "SELECT env.PlotNumber, env.FieldNumber, env.PlotRepresenting, env.Zone, env.SubZone, env.SiteSeries"+from+" ORDER BY env.PlotNumber LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return PlotPage{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var plot PlotSummary
		if err := rows.Scan(&plot.PlotNumber, &plot.FieldNumber, &plot.PlotRepresenting, &plot.Zone, &plot.SubZone, &plot.SiteSeries); err != nil {
			return PlotPage{}, err
		}
		page.Plots = append(page.Plots, plot)
	}
	return page, rows.Err()
}
