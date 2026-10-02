package main

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

var plotProfileColumns = []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"}

func (source *PlotProfileSource) UnmarshalJSON(data []byte) error {
	type plain PlotProfileSource
	var decoded plain
	if err := decodeProfileLifecycleJSON(data, &decoded, "name", "path"); err != nil {
		return err
	}
	*source = PlotProfileSource(decoded)
	return source.validate()
}

func (source PlotProfileSource) validate() error {
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

func (c *sqliteContext) profileLocation() (string, string, error) {
	if c.profile == nil {
		return "project", c.selection.Project + "_Profile", nil
	}
	if c.profile.Name == "None" {
		return "", "", errors.New("no plot profile is selected; select a stored profile explicitly")
	}
	return c.profileRole, c.profile.Name + "_Profile", nil
}

func (c *sqliteContext) profileInfo(ctx context.Context) (PlotProfileSourceInfo, error) {
	source := PlotProfileSource{c.selection.Project, c.selection.ProjectPath}
	if c.profile != nil {
		source = *c.profile
	}
	info := PlotProfileSourceInfo{Source: source}
	if source.Name == "None" {
		info.Reason = "No plot profile is selected"
		return info, nil
	}
	role, table, err := c.profileLocation()
	if err != nil {
		return info, err
	}
	info.Table = table
	var physical bool
	if err := c.conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+quoteHeaderIdentifier(role)+".sqlite_master WHERE type='table' AND name=?)", table).Scan(&physical); err != nil {
		return info, err
	}
	if !physical {
		info.Reason = "Original physical plot-profile table is unavailable"
		return info, nil
	}
	columns, err := readSQLiteStorageColumns(ctx, c.conn, role, table)
	if err != nil {
		return info, err
	}
	metadata, err := profileMetadataColumns(ctx, c.conn, role)
	if err != nil {
		return info, err
	}
	info.Reason = profileSchemaReason(columns, metadata)
	info.Available = info.Reason == ""
	info.Writable = info.Available && (c.profileWrite ||
		source.Name == c.selection.Project && sameDesktopPath(source.Path, c.selection.ProjectPath))
	return info, nil
}

func profileSchemaReason(columns, metadata []ProjectMetadataColumn) string {
	for _, required := range plotProfileColumns {
		if !slices.ContainsFunc(columns, func(column ProjectMetadataColumn) bool { return column.Name == required }) {
			return fmt.Sprintf("Missing original plot-profile column %s; no repair or inferred defaults", required)
		}
	}
	return profileMetadataReason(metadata)
}

func profileMetadataReason(metadata []ProjectMetadataColumn) string {
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

func (c *sqliteContext) attachProfile(ctx context.Context, requested *PlotProfileSource) error {
	if requested == nil {
		return nil
	}
	source := *requested
	if err := source.validate(); err != nil {
		return err
	}
	c.profile = &source
	if source.Name == "None" {
		return nil
	}
	path, err := existingDatabasePath(source.Path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	for _, role := range []string{"project", "VPro64", "VLists", "VUser", "VMetaData", "VMessageBoard", "su", "hierarchy"} {
		if original, present := c.attachmentInfo[role]; present && os.SameFile(info, original) {
			c.profileRole = role
			source.Path = c.attachments[role]
			break
		}
	}
	if c.profileRole == "" {
		source.Path, err = c.attach(ctx, "profile", path)
		if err != nil {
			return err
		}
		c.profileRole = "profile"
	}
	table := source.Name + "_Profile"
	if err := c.requireTable(ctx, c.profileRole, table); err != nil {
		return err
	}
	if err := c.requireColumns(ctx, c.profileRole, table, plotProfileColumns); err != nil {
		return err
	}
	metadata, err := profileMetadataColumns(ctx, c.conn, c.profileRole)
	if err != nil {
		return err
	}
	if metadata == nil {
		return nil
	}
	return c.requireColumns(ctx, c.profileRole, "_table_metadata", []string{"table_name", "description"})
}

func (store *desktopConfig) plotProfileSelection() (*PlotProfileSource, error) {
	values, err := store.snapshot()
	if err != nil {
		return nil, err
	}
	current, err := configSection(values, "Current")
	if err != nil {
		return nil, err
	}
	// Older desktop configuration retained CurrVegProfile without implementing it.
	// An explicit ProfilePath activates selection without changing that baseline.
	if _, active := current["ProfilePath"]; !active {
		return nil, nil
	}
	name, err := configString(values, "Current", "CurrVegProfile")
	if err != nil {
		return nil, err
	}
	path, err := configString(values, "Current", "ProfilePath")
	if err != nil {
		return nil, err
	}
	source := &PlotProfileSource{name, path}
	return source, source.validate()
}

func (s *ContextService) SelectPlotProfile(expectedID string, requested PlotProfileSource) (ProjectState, error) {
	if err := requested.validate(); err != nil {
		return ProjectState{}, err
	}
	s.projects.operationMu.Lock()
	defer s.projects.operationMu.Unlock()
	s.projects.mu.Lock()
	defer s.projects.mu.Unlock()
	if expectedID == "" || expectedID != s.projects.contextID {
		return ProjectState{}, errors.New("profile context changed; reload before selecting")
	}
	if s.projects.sqlite == nil || s.projects.sqlite.conn == nil {
		return ProjectState{}, errors.New("profile context is closed")
	}
	if err := profileOwnedFiles(s.projects.sqlite); err != nil {
		return ProjectState{}, err
	}
	return s.projects.switchSQLiteProfileLocked(s.projects.sqlite.selection, &requested)
}

func (s *ContextService) ListPlotProfileSources(ctx context.Context, contextID, path string) ([]PlotProfileSourceInfo, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) ([]PlotProfileSourceInfo, error) {
		owner := plots.projects.sqlite
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		paths := []string{owner.selection.ProjectPath, owner.attachments["VPro64"]}
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
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func inspectPlotProfileSources(ctx context.Context, owner *sqliteContext, path string) (_ []PlotProfileSourceInfo, resultErr error) {
	path, err := existingDatabasePath(path)
	if err != nil {
		return nil, err
	}
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	db, err := openReadOnlyContext(ctx, path)
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
		columns, err := readSQLiteStorageColumns(ctx, db, "main", table)
		if err != nil {
			return nil, err
		}
		metadata, err := profileMetadataColumns(ctx, db, "main")
		if err != nil {
			return nil, err
		}
		info.Reason = profileSchemaReason(columns, metadata)
		info.Available = info.Reason == ""
		info.Writable = info.Available && info.Source.Name == owner.selection.Project &&
			os.SameFile(before, owner.attachmentInfo["project"])
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

func (service *ProjectService) persistSQLiteContext(candidate *sqliteContext) error {
	current, err := service.preferences.selection()
	if err != nil {
		return err
	}
	profile, err := service.preferences.plotProfileSelection()
	if err != nil {
		return err
	}
	if current == candidate.selection && reflect.DeepEqual(profile, candidate.profile) {
		return nil
	}
	selection := candidate.selection
	changes := map[string]any{
		"CurrProject": selection.Project, "ProjectPath": selection.ProjectPath,
		"CurrPlotlist": selection.SU, "SUPath": selection.SUPath,
		"CurrHierarchy": selection.Hierarchy, "HierarchyPath": selection.HierarchyPath,
	}
	if candidate.profile != nil {
		changes["CurrVegProfile"], changes["ProfilePath"] = candidate.profile.Name, candidate.profile.Path
	}
	return service.preferences.update("Current", changes)
}
