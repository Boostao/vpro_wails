package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type HierarchyInfo struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Version    string `json:"version"`
	Compatible bool   `json:"compatible"`
}

type HierarchyNode struct {
	ID     int64   `json:"id"`
	Name   *string `json:"name"`
	Parent *int64  `json:"parent"`
	Level  *int64  `json:"level"`
}

func hierarchyCompatible(database *sql.DB, table string) (bool, error) {
	rows, err := database.Query(`PRAGMA table_info("` + table + `")`)
	if err != nil {
		return false, err
	}
	fields := make(map[string]string)
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, fieldType string
		var defaultValue sql.NullString
		if err := rows.Scan(&index, &name, &fieldType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return false, err
		}
		fields[name] = strings.ToUpper(fieldType)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	return strings.Contains(fields["ID"], "INT") &&
		(strings.Contains(fields["Name"], "CHAR") || strings.Contains(fields["Name"], "CLOB") || strings.Contains(fields["Name"], "TEXT")) &&
		strings.Contains(fields["Parent"], "INT") && strings.Contains(fields["Level"], "INT"), nil
}

func discoverHierarchies(projectsDir string) ([]HierarchyInfo, error) {
	files, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}
	hierarchies := []HierarchyInfo{}
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.EqualFold(filepath.Ext(file.Name()), ".db") {
			continue
		}
		database, err := openReadOnly(filepath.Join(projectsDir, file.Name()))
		if err != nil {
			if file.Name() == "Sample.db" {
				return nil, fmt.Errorf("inspect %s: %w", file.Name(), err)
			}
			continue
		}
		var schemaVersion int
		if err := database.QueryRow("PRAGMA schema_version").Scan(&schemaVersion); err != nil {
			database.Close()
			if file.Name() == "Sample.db" {
				return nil, fmt.Errorf("inspect %s: %w", file.Name(), err)
			}
			continue
		}
		found, err := inspectHierarchies(database, file.Name())
		database.Close()
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", file.Name(), err)
		}
		hierarchies = append(hierarchies, found...)
	}
	slices.SortFunc(hierarchies, func(left, right HierarchyInfo) int {
		if order := strings.Compare(left.Name, right.Name); order != 0 {
			return order
		}
		return strings.Compare(left.File, right.File)
	})
	return hierarchies, nil
}

func inspectHierarchies(database *sql.DB, file string) ([]HierarchyInfo, error) {
	rows, err := database.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE '%_Hierarchy'")
	if err != nil {
		return nil, err
	}
	tables := []string{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return nil, err
		}
		if name, ok := strings.CutSuffix(table, "_Hierarchy"); ok && projectNamePattern.MatchString(name) {
			tables = append(tables, table)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var hasMetadata bool
	if err := database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = '_table_metadata')").Scan(&hasMetadata); err != nil {
		return nil, err
	}
	found := make([]HierarchyInfo, 0, len(tables))
	for _, table := range tables {
		name, _ := strings.CutSuffix(table, "_Hierarchy")
		compatible, err := hierarchyCompatible(database, table)
		if err != nil {
			return nil, err
		}
		info := HierarchyInfo{Name: name, File: file, Version: "Unknown", Compatible: compatible}
		if hasMetadata {
			var version sql.NullString
			err := database.QueryRow("SELECT description FROM _table_metadata WHERE table_name = ?", table).Scan(&version)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if version.Valid && version.String != "" {
				info.Version = version.String
			}
		}
		found = append(found, info)
	}
	return found, nil
}

func listHierarchyNodes(path, name string) ([]HierarchyNode, error) {
	if !projectNamePattern.MatchString(name) {
		return nil, fmt.Errorf("invalid hierarchy name %q", name)
	}
	database, err := openReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	table := name + "_Hierarchy"
	var exists bool
	if err := database.QueryRow("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)", table).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("hierarchy table %q does not exist", table)
	}
	compatible, err := hierarchyCompatible(database, table)
	if err != nil {
		return nil, err
	}
	if !compatible {
		return nil, fmt.Errorf("hierarchy table %q has incompatible fields", table)
	}
	rows, err := database.Query(`SELECT ID, Name, Parent, Level FROM "` + table + `" ORDER BY ID`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := []HierarchyNode{}
	for rows.Next() {
		var node HierarchyNode
		var label sql.NullString
		var parent, level sql.NullInt64
		if err := rows.Scan(&node.ID, &label, &parent, &level); err != nil {
			return nil, err
		}
		if label.Valid {
			node.Name = &label.String
		}
		if parent.Valid {
			node.Parent = &parent.Int64
		}
		if level.Valid {
			node.Level = &level.Int64
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}
