package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type ownedPictureLibrary struct {
	requestedPath string
	path          string
	info          os.FileInfo
}

type pictureMetadataReview struct {
	ContextID  string               `json:"contextId"`
	Project    string               `json:"project"`
	PlotNumber string               `json:"plotNumber"`
	Records    ProjectMetadataTable `json:"records"`
}

func newOwnedPictureLibrary(path string) (*ownedPictureLibrary, error) {
	if !utf8.ValidString(path) || strings.ContainsRune(path, 0) {
		return nil, errors.New("picture library path contains malformed Unicode or NUL")
	}
	resolved, err := existingDatabasePath(path)
	if err != nil {
		return nil, fmt.Errorf("picture library: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	source := &ownedPictureLibrary{requestedPath: filepath.Clean(path), path: resolved, info: info}
	if err := source.checkFile(); err != nil {
		return nil, err
	}
	return source, nil
}

func (source *ownedPictureLibrary) checkFile() error {
	if source == nil || source.info == nil {
		return errors.New("picture library requires an explicitly owned existing SQLite source")
	}
	for _, path := range []string{source.requestedPath, source.path} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("picture library file unavailable: %w", err)
		}
		if !info.Mode().IsRegular() || !os.SameFile(source.info, info) {
			return errors.New("picture library file identity changed; reload the source")
		}
	}
	return nil
}

func (s *ContextService) readPictureMetadata(ctx context.Context, contextID, plot string, source *ownedPictureLibrary) (pictureMetadataReview, error) {
	return withOwnedPictureRead(ctx, s, contextID, plot, source, func(project string, records ProjectMetadataTable) (pictureMetadataReview, error) {
		return pictureMetadataReview{contextID, project, plot, records}, nil
	})
}

func withOwnedPictureRead[T any](ctx context.Context, s *ContextService, contextID, plot string, source *ownedPictureLibrary,
	read func(string, ProjectMetadataTable) (T, error)) (T, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (T, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (T, error) {
			var zero T
			if err := requirePictureParent(ctx, owner, tx, plot); err != nil {
				return zero, err
			}
			return withPictureLibrarySnapshot(ctx, source, plot, func(records ProjectMetadataTable) (T, error) {
				return read(owner.selection.Project, records)
			})
		})
	})
}

func requirePictureParent(ctx context.Context, owner *sqliteContext, tx *sql.Tx, plot string) error {
	if plot == "" || !utf8.ValidString(plot) || strings.ContainsRune(plot, 0) {
		return errors.New("picture metadata requires a literal valid plot identity")
	}
	var member bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM USysEnv WHERE typeof(PlotNumber)='text' AND CAST(PlotNumber AS BLOB)=CAST(? AS BLOB))`, plot).Scan(&member); err != nil {
		return fmt.Errorf("picture parent scope unavailable: %w", err)
	}
	if !member {
		return errors.New("picture parent is outside the selected context")
	}
	return nil
}

func readPictureLibrary(ctx context.Context, source *ownedPictureLibrary, plot string) (ProjectMetadataTable, error) {
	return withPictureLibrarySnapshot(ctx, source, plot, func(records ProjectMetadataTable) (ProjectMetadataTable, error) {
		return records, nil
	})
}

func withPictureLibrarySnapshot[T any](ctx context.Context, source *ownedPictureLibrary, plot string, read func(ProjectMetadataTable) (T, error)) (result T, resultErr error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := source.checkFile(); err != nil {
		return result, err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(source.path, "ro")+"&_query_only=on&_busy_timeout=5000")
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, db.Close())
		if resultErr != nil {
			result = zero
		}
	}()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, conn.Close())
	}()
	// Reuse snapshot cleanup without registering pictures as a mandatory family.
	reader := &sqliteContext{conn: conn}
	tx, err := reader.beginReadSnapshot(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	records, err := readPictureLibraryRows(ctx, tx, plot)
	if err != nil {
		return zero, err
	}
	result, err = read(records)
	if err != nil {
		return zero, err
	}
	if err := source.checkFile(); err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}

func readPictureLibraryRows(ctx context.Context, tx *sql.Tx, plot string) (ProjectMetadataTable, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE type='table' AND name COLLATE BINARY='tblVPics'`).Scan(&count); err != nil {
		return ProjectMetadataTable{}, fmt.Errorf("picture library SQLite schema unavailable: %w", err)
	}
	if count != 1 {
		return ProjectMetadataTable{}, errors.New("picture library requires the original physical tblVPics table; staging tables and views are not published libraries")
	}
	if err := validateSIVIPhysicalSchema(ctx, tx, "main", "tblVPics", "picture metadata"); err != nil {
		return ProjectMetadataTable{}, err
	}
	columns, err := readSQLiteStorageColumns(ctx, tx, "main", "tblVPics")
	if err != nil {
		return ProjectMetadataTable{}, err
	}
	names := []string{"ID", "PicDir", "PicName", "PlotNumber", "PicComment"}
	if len(columns) != len(names) {
		return ProjectMetadataTable{}, errors.New("picture library requires the five original source columns; no missing or additional binding was inferred")
	}
	for index, name := range names {
		if columns[index].Name != name {
			return ProjectMetadataTable{}, fmt.Errorf("picture library source column %d must be %q; no binding was repaired", index, name)
		}
	}
	return readSQLiteLiteralTextRows(ctx, tx, "main", "tblVPics", "PlotNumber", plot, "")
}
