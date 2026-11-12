package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
)

type PlotProfileSource struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type PlotProfileSourceInfo struct {
	Source    PlotProfileSource `json:"source"`
	Table     string            `json:"table"`
	Available bool              `json:"available"`
	Writable  bool              `json:"writable"`
	Reason    string            `json:"reason"`
}

var PlotProfileColumns = []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"}

func (source *PlotProfileSource) UnmarshalJSON(data []byte) error {
	type plain PlotProfileSource
	var decoded plain
	if err := DecodeProfileLifecycleJSON(data, &decoded, "name", "path"); err != nil {
		return err
	}
	*source = PlotProfileSource(decoded)
	return source.Validate()
}

func (source PlotProfileSource) Validate() error {
	if source.Name == "" || !utf8.ValidString(source.Name) || strings.ContainsRune(source.Name, 0) || !utf8.ValidString(source.Path) {
		return errors.New("profile selection requires an exact name and valid Unicode path")
	}
	if source.Name == "None" && source.Path != "" {
		return errors.New("None profile must have an empty path")
	}
	if source.Name != "None" && source.Path == "" {
		return errors.New("profile selection requires an explicit absolute existing SQLite path")
	}
	return nil
}

func (c *SqliteContext) ProfileLocation() (string, string, error) {
	if c.Profile == nil {
		return "project", c.Selection.Project + "_Profile", nil
	}
	if c.Profile.Name == "None" {
		return "", "", errors.New("no plot profile is selected; select a stored profile explicitly")
	}
	return c.profileRole, c.Profile.Name + "_Profile", nil
}

func (c *SqliteContext) ProfileInfo(ctx context.Context) (PlotProfileSourceInfo, error) {
	source := PlotProfileSource{c.Selection.Project, c.Selection.ProjectPath}
	if c.Profile != nil {
		source = *c.Profile
	}
	info := PlotProfileSourceInfo{Source: source}
	if source.Name == "None" {
		info.Reason = "No plot profile is selected"
		return info, nil
	}
	role, table, err := c.ProfileLocation()
	if err != nil {
		return info, err
	}
	info.Table = table
	var physical bool
	if err := c.Conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+QuoteHeaderIdentifier(role)+".sqlite_master WHERE type='table' AND name=?)", table).Scan(&physical); err != nil {
		return info, err
	}
	if !physical {
		info.Reason = "Original physical plot-profile table is unavailable"
		return info, nil
	}
	columns, err := ReadSQLiteStorageColumns(ctx, c.Conn, role, table)
	if err != nil {
		return info, err
	}
	metadata, err := ProfileMetadataColumns(ctx, c.Conn, role)
	if err != nil {
		return info, err
	}
	info.Reason = ProfileSchemaReason(columns, metadata)
	info.Available = info.Reason == ""
	info.Writable = info.Available && (c.ProfileWrite ||
		source.Name == c.Selection.Project && SameDesktopPath(source.Path, c.Selection.ProjectPath))
	return info, nil
}

func ProfileSchemaReason(columns, metadata []ProjectMetadataColumn) string {
	for _, required := range PlotProfileColumns {
		if !slices.ContainsFunc(columns, func(column ProjectMetadataColumn) bool { return column.Name == required }) {
			return fmt.Sprintf("Missing original plot-profile column %s; no repair or inferred defaults", required)
		}
	}
	return ProfileMetadataReason(metadata)
}

func ProfileMetadataReason(metadata []ProjectMetadataColumn) string {
	if metadata == nil {
		return ""
	}
	for _, required := range []string{"table_name", "description"} {
		if !slices.ContainsFunc(metadata, func(column ProjectMetadataColumn) bool { return column.Name == required }) {
			return "Original table-object description metadata is unavailable"
		}
	}
	return ""
}

func (c *SqliteContext) attachProfile(ctx context.Context, requested *PlotProfileSource) error {
	if requested == nil {
		return nil
	}
	source := *requested
	if err := source.Validate(); err != nil {
		return err
	}
	c.Profile = &source
	if source.Name == "None" {
		return nil
	}
	path, err := ExistingDatabasePath(source.Path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	for _, role := range []string{"project", "VPro64", "VLists", "VUser", "VMetaData", "VMessageBoard", "su", "hierarchy"} {
		if original, present := c.AttachmentInfo[role]; present && os.SameFile(info, original) {
			c.profileRole = role
			source.Path = c.Attachments[role]
			break
		}
	}
	if c.profileRole == "" {
		source.Path, err = c.Attach(ctx, "profile", path)
		if err != nil {
			return err
		}
		c.profileRole = "profile"
	}
	table := source.Name + "_Profile"
	if err := c.requireTable(ctx, c.profileRole, table); err != nil {
		return err
	}
	if err := c.RequireColumns(ctx, c.profileRole, table, PlotProfileColumns); err != nil {
		return err
	}
	metadata, err := ProfileMetadataColumns(ctx, c.Conn, c.profileRole)
	if err != nil {
		return err
	}
	if metadata == nil {
		return nil
	}
	return c.RequireColumns(ctx, c.profileRole, "_table_metadata", []string{"table_name", "description"})
}

func (store *DesktopConfig) PlotProfileSelection() (*PlotProfileSource, error) {
	values, err := store.Snapshot()
	if err != nil {
		return nil, err
	}
	current, err := ConfigSection(values, "Current")
	if err != nil {
		return nil, err
	}
	// Older desktop configuration retained CurrVegProfile without implementing it.
	// An explicit ProfilePath activates selection without changing that baseline.
	if _, active := current["ProfilePath"]; !active {
		return nil, nil
	}
	name, err := ConfigString(values, "Current", "CurrVegProfile")
	if err != nil {
		return nil, err
	}
	path, err := ConfigString(values, "Current", "ProfilePath")
	if err != nil {
		return nil, err
	}
	source := &PlotProfileSource{name, path}
	return source, source.Validate()
}

func (s *ContextService) SelectPlotProfile(expectedID string, requested PlotProfileSource) (ProjectState, error) {
	if err := requested.Validate(); err != nil {
		return ProjectState{}, err
	}
	s.Projects.OperationMu.Lock()
	defer s.Projects.OperationMu.Unlock()
	s.Projects.Mu.Lock()
	defer s.Projects.Mu.Unlock()
	if expectedID == "" || expectedID != s.Projects.ContextID {
		return ProjectState{}, errors.New("profile context changed; reload before selecting")
	}
	if s.Projects.Sqlite == nil || s.Projects.Sqlite.Conn == nil {
		return ProjectState{}, errors.New("profile context is closed")
	}
	if err := ProfileOwnedFiles(s.Projects.Sqlite); err != nil {
		return ProjectState{}, err
	}
	return s.Projects.switchSQLiteProfileLocked(s.Projects.Sqlite.Selection, &requested)
}

func (s *ContextService) ListPlotProfileSources(ctx context.Context, contextID, path string) ([]PlotProfileSourceInfo, error) {
	return WithContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]PlotProfileSourceInfo, error) {
		owner := plots.Projects.Sqlite
		if err := ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		paths := []string{owner.Selection.ProjectPath, owner.Attachments["VPro64"]}
		if path != "" {
			paths = []string{path}
		}
		result := []PlotProfileSourceInfo{}
		for _, path := range paths {
			found, err := inspectPlotProfileSources(ctx, owner, path)
			if err != nil {
				return nil, err
			}
			for _, info := range found {
				if !slices.ContainsFunc(result, func(existing PlotProfileSourceInfo) bool {
					return existing.Source == info.Source
				}) {
					result = append(result, info)
				}
			}
		}
		if err := ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func inspectPlotProfileSources(ctx context.Context, owner *SqliteContext, path string) (_ []PlotProfileSourceInfo, resultErr error) {
	path, err := ExistingDatabasePath(path)
	if err != nil {
		return nil, err
	}
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	db, err := sqlutil.OpenReadOnlyContext(ctx, path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name COLLATE BINARY`)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		if strings.HasSuffix(name, "_Profile") && name != "_Profile" {
			names = append(names, name)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	result := []PlotProfileSourceInfo{}
	for _, table := range names {
		info := PlotProfileSourceInfo{Source: PlotProfileSource{strings.TrimSuffix(table, "_Profile"), path}, Table: table}
		columns, err := ReadSQLiteStorageColumns(ctx, db, "main", table)
		if err != nil {
			return nil, err
		}
		metadata, err := ProfileMetadataColumns(ctx, db, "main")
		if err != nil {
			return nil, err
		}
		info.Reason = ProfileSchemaReason(columns, metadata)
		info.Available = info.Reason == ""
		info.Writable = info.Available && info.Source.Name == owner.Selection.Project &&
			os.SameFile(before, owner.AttachmentInfo["project"])
		result = append(result, info)
	}
	after, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) {
		return nil, errors.New("profile inspection file identity changed; results were not published")
	}
	return result, nil
}

func (service *ProjectService) persistSQLiteContext(candidate *SqliteContext) error {
	current, err := service.Preferences.Selection()
	if err != nil {
		return err
	}
	profile, err := service.Preferences.PlotProfileSelection()
	if err != nil {
		return err
	}
	if current == candidate.Selection && reflect.DeepEqual(profile, candidate.Profile) {
		return nil
	}
	selection := candidate.Selection
	changes := map[string]any{
		"CurrProject": selection.Project, "ProjectPath": selection.ProjectPath,
		"CurrPlotlist": selection.SU, "SUPath": selection.SUPath,
		"CurrHierarchy": selection.Hierarchy, "HierarchyPath": selection.HierarchyPath,
	}
	if candidate.Profile != nil {
		changes["CurrVegProfile"], changes["ProfilePath"] = candidate.Profile.Name, candidate.Profile.Path
	}
	return service.Preferences.Update("Current", changes)
}
