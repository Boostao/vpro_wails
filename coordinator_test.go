package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineDuckDBSQLiteCoordinator(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data'with-quote")
	if _, err := NewProjectService(root); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(root, "projects", "Sample.db")
	before, err := os.ReadFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewCoordinator(context.Background(), projectPath, "Sample")
	if err != nil {
		if strings.Contains(err.Error(), "not installed") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	defer coordinator.Close()
	count, err := coordinator.CountPlots(context.Background())
	if err != nil || count != 52 {
		t.Fatalf("unexpected temporary view count: %d: %v", count, err)
	}
	expected := []struct {
		name  string
		count int
	}{{"USysEnv", 52}, {"USysVeg", 1633}, {"USysHumus", 65}, {"USysMineral", 99}, {"USysAuditTrail", 389}, {"USysOther", 1}, {"USysMetadata", 5}, {"USysVegA", 447}, {"USysVegB", 384}, {"USysVegC", 853}, {"USysVegD", 337}}
	for _, view := range expected {
		var rows int
		if err := coordinator.conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM "`+view.name+`"`).Scan(&rows); err != nil || rows != view.count {
			t.Fatalf("view %s: got %d, expected %d: %v", view.name, rows, view.count, err)
		}
	}
	after, err := os.ReadFile(projectPath)
	if err != nil || string(before) != string(after) {
		t.Fatalf("read-only attachment changed the project file: %v", err)
	}
}
