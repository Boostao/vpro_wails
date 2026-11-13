package interchange

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func validateFreshTargetProjectName(raw string) (string, error) {
	target := strings.TrimSpace(raw)
	if !vprocore.ProjectNamePattern.MatchString(target) ||
		strings.EqualFold(target, "None") ||
		strings.EqualFold(target, "Sample") ||
		strings.Contains(strings.ToLower(target), "master") {
		return "", errors.New("target project name must start with a letter, use only letters/digits/underscore (max 31 chars), and not be None, Sample, or master")
	}
	return target, nil
}

func stageAndPublishVP08Project(
	s *vprocore.ContextService,
	ctx context.Context,
	contextID string,
	targetProject string,
	destinationPath string,
	activateOnSuccess bool,
	populate func(ctx context.Context, tx *sql.Tx, project string) error,
) (resolvedDest string, sha256Hex string, activated bool, newState *vprocore.ProjectState, resultErr error) {
	targetProject, err := validateFreshTargetProjectName(targetProject)
	if err != nil {
		return "", "", false, nil, err
	}
	destPath := strings.TrimSpace(destinationPath)
	if destPath == "" {
		var projectsDir string
		if _, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
			projectsDir = filepath.Join(plots.Projects.Root, "projects")
			return struct{}{}, nil
		}); err != nil {
			return "", "", false, nil, err
		}
		destPath = filepath.Join(projectsDir, targetProject+".db")
	} else if !strings.EqualFold(filepath.Ext(destPath), ".db") {
		destPath += ".db"
	}

	resolvedDest, parentInfo, err := vprocore.FreshSQLiteDestination(destPath, "Project")
	if err != nil {
		return "", "", false, nil, err
	}
	review, err := s.ReadNewProjectTemplates(ctx, contextID)
	if err != nil {
		return "", "", false, nil, err
	}
	ddlPlan, err := vprocore.PlanNewProjectDDL(ctx, targetProject, review)
	if err != nil {
		return "", "", false, nil, err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(resolvedDest), ".vpro-stage-*.db")
	if err != nil {
		return "", "", false, nil, err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		return "", "", false, nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return "", "", false, nil, err
	}

	sqlDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL")
	if err != nil {
		return "", "", false, nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		sqlDB.Close()
		return "", "", false, nil, err
	}

	build := func() error {
		for _, tblPlan := range ddlPlan.Tables {
			for _, mapping := range tblPlan.Schema {
				if mapping.SQL != nil {
					if _, err := tx.ExecContext(ctx, *mapping.SQL); err != nil {
						return fmt.Errorf("create schema for %s: %w", tblPlan.TargetTable, err)
					}
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE "_table_metadata" ("table_name" TEXT PRIMARY KEY, "description" TEXT)`); err != nil {
			return err
		}
		for _, suffix := range vprocore.CoreTables {
			if _, err := tx.ExecContext(ctx, `INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP08')`, targetProject+"_"+suffix); err != nil {
				return err
			}
		}
		return populate(ctx, tx, targetProject)
	}

	if err := build(); err != nil {
		_ = tx.Rollback()
		_ = sqlDB.Close()
		return "", "", false, nil, err
	}
	if err := tx.Commit(); err != nil {
		_ = sqlDB.Close()
		return "", "", false, nil, err
	}
	if err := sqlDB.Close(); err != nil {
		return "", "", false, nil, err
	}

	inspected, err := vprocore.InspectFileContext(ctx, tmpPath)
	if err != nil {
		return "", "", false, nil, fmt.Errorf("verify staged SQLite project: %w", err)
	}
	compatible := false
	for _, p := range inspected {
		if p.Name == targetProject && p.Compatible {
			compatible = true
			break
		}
	}
	if !compatible {
		return "", "", false, nil, errors.New("staged SQLite database did not pass VP08 compatibility validation")
	}

	currentParent, err := os.Stat(filepath.Dir(resolvedDest))
	if err != nil || !os.SameFile(parentInfo, currentParent) {
		return "", "", false, nil, errors.New("destination parent directory changed during import; publication aborted")
	}
	if err := sqlutil.LinkOrCreateExclusive(tmpPath, resolvedDest); err != nil {
		return "", "", false, nil, fmt.Errorf("publish SQLite database %s: %w", resolvedDest, err)
	}
	published = true
	_ = os.Remove(tmpPath)

	if err := vprocore.InitializeSQLiteWAL(ctx, resolvedDest); err != nil {
		_ = os.Remove(resolvedDest + "-wal")
		_ = os.Remove(resolvedDest + "-shm")
		_ = os.Remove(resolvedDest)
		return "", "", false, nil, fmt.Errorf("initialize SQLite WAL on %s: %w", resolvedDest, err)
	}

	destBytes, err := os.ReadFile(resolvedDest)
	if err != nil {
		return "", "", false, nil, err
	}
	digest := sha256.Sum256(destBytes)
	shaHex := hex.EncodeToString(digest[:])

	if activateOnSuccess {
		state, switchErr := s.SwitchContext(contextID, vprocore.ContextSelection{
			Project:     targetProject,
			ProjectPath: resolvedDest,
			SU:          "None",
			Hierarchy:   "None",
		})
		if switchErr == nil {
			_ = s.Projects.EnsureActiveProjectWAL(ctx)
			activated = true
			newState = &state
		}
	}
	return resolvedDest, shaHex, activated, newState, nil
}

func writeAtomicExportFile(destinationPath string, data []byte) (string, string, error) {
	if strings.TrimSpace(destinationPath) == "" {
		sum := sha256.Sum256(data)
		return "", hex.EncodeToString(sum[:]), nil
	}
	if !utf8.ValidString(destinationPath) || strings.ContainsRune(destinationPath, 0) {
		return "", "", errors.New("destination path must be valid UTF-8 without NUL")
	}
	abs, err := filepath.Abs(filepath.Clean(destinationPath))
	if err != nil {
		return "", "", err
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create destination directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".vpro-export-*")
	if err != nil {
		return "", "", err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return "", "", err
	}
	committed = true
	sum := sha256.Sum256(data)
	return abs, hex.EncodeToString(sum[:]), nil
}
