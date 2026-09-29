package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAndListSampleHierarchy(t *testing.T) {
	projects := t.TempDir()
	data, err := os.ReadFile("resources/Sample.db")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(projects, "Sample.db")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	found, err := discoverHierarchies(projects)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != (HierarchyInfo{Name: "Sample", File: "Sample.db", Version: "VP04", Compatible: true}) {
		t.Fatalf("unexpected hierarchies: %+v", found)
	}
	nodes, err := listHierarchyNodes(path, "Sample")
	if err != nil || len(nodes) != 43 {
		t.Fatalf("unexpected sample nodes: %d, %v", len(nodes), err)
	}
	for index, node := range nodes {
		if node.Name == nil || (index > 0 && node.ID < nodes[index-1].ID) {
			t.Fatalf("invalid node ordering or label: %+v", node)
		}
	}
	if _, err := json.Marshal(nodes); err != nil {
		t.Fatalf("nodes cannot be serialized: %v", err)
	}
	if _, err := listHierarchyNodes(path, "../Sample"); err == nil {
		t.Fatal("accepted path-like hierarchy name")
	}
	if _, err := listHierarchyNodes(path, "Missing"); err == nil {
		t.Fatal("accepted missing hierarchy table")
	}
	if _, err := listHierarchyNodes(filepath.Join(projects, "missing.db"), "Sample"); err == nil {
		t.Fatal("opened nonexistent database")
	}
	if err := os.WriteFile(filepath.Join(projects, "Broken.db"), []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	found, err = discoverHierarchies(projects)
	if err != nil || len(found) != 1 {
		t.Fatalf("unrelated invalid database blocked hierarchy discovery: %+v: %v", found, err)
	}
}

func TestHierarchySchemaMetadataAndDuplicates(t *testing.T) {
	projects := t.TempDir()
	path := filepath.Join(projects, "One.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for _, statement := range []string{
		`CREATE TABLE Good_Hierarchy (ID INTEGER PRIMARY KEY, Name TEXT, Parent INTEGER, Level SMALLINT)`,
		`INSERT INTO Good_Hierarchy VALUES (1, NULL, NULL, NULL)`,
		`CREATE TABLE Bad_Hierarchy (ID INTEGER, Name BLOB, Parent TEXT, Level INTEGER)`,
		`CREATE TABLE "1Invalid_Hierarchy" (ID INTEGER, Name TEXT, Parent INTEGER, Level INTEGER)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	found, err := discoverHierarchies(projects)
	if err != nil || len(found) != 2 {
		t.Fatalf("unexpected discovered tables: %+v, %v", found, err)
	}
	if found[0].Name != "Bad" || found[0].Compatible || found[0].Version != "Unknown" || found[1].Name != "Good" || !found[1].Compatible {
		t.Fatalf("incorrect schema or metadata handling: %+v", found)
	}
	if _, err := listHierarchyNodes(path, "Bad"); err == nil {
		t.Fatal("listed incompatible hierarchy")
	}
	nodes, err := listHierarchyNodes(path, "Good")
	if err != nil || len(nodes) != 1 || nodes[0].Name != nil || nodes[0].Parent != nil || nodes[0].Level != nil {
		t.Fatalf("null hierarchy values lost: %+v, %v", nodes, err)
	}
	other, err := sql.Open("sqlite3", filepath.Join(projects, "Two.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Exec(`CREATE TABLE Good_Hierarchy (ID INTEGER, Name TEXT, Parent INTEGER, Level INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	found, err = discoverHierarchies(projects)
	if err != nil || len(found) != 3 || found[1].File == found[2].File {
		t.Fatalf("hierarchies with the same name lost their source identity: %+v: %v", found, err)
	}
}
