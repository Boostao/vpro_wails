package vprocore

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"database/sql"

	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var WorkingUnitDatabase = resources.WorkingUnitDB

var WorkingUnitProvenanceJSON = resources.WorkingUnitProvenanceJSON

type WorkingUnitChoice struct {
	RowID          string  `json:"rowId"`
	Origin         string  `json:"origin"`
	SourceID       *string `json:"sourceId"`
	Code           *string `json:"code"`
	Description    *string `json:"description"`
	ScientificName *string `json:"scientificName"`
	Level          *int    `json:"level"`
	Selectable     bool    `json:"selectable"`
	Diagnostic     string  `json:"diagnostic"`
}

type WorkingUnitModeState struct {
	Mode    string  `json:"mode"`
	Warning *string `json:"warning"`
}

type WorkingUnitService struct {
	Mu           sync.RWMutex `json:"-" yaml:"-"`
	projects     *ProjectService
	Db           *sql.DB                    `json:"-" yaml:"-"`
	SettingsPath string                     `json:"-" yaml:"-"`
	ReplaceFile  func(string, string) error `json:"-" yaml:"-"`
}

func NewWorkingUnitService(projects *ProjectService, configDir string) (*WorkingUnitService, error) {
	if projects == nil || strings.TrimSpace(configDir) == "" {
		return nil, errors.New("Working Unit project service and configuration directory are required")
	}
	configDir, err := filepath.Abs(configDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, err
	}
	var provenance WorkingUnitProvenance
	if err := DecodeWorkingUnitJSON(WorkingUnitProvenanceJSON, &provenance); err != nil {
		return nil, fmt.Errorf("Working Unit catalogue provenance: %w", err)
	}
	if sqlutil.BecHash(WorkingUnitDatabase) != provenance.DatabaseSHA256 {
		return nil, errors.New("embedded Working Unit catalogue checksum mismatch")
	}
	target := filepath.Join(projects.Root, "working-unit.db")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(WorkingUnitDatabase)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return nil, errors.Join(writeErr, closeErr, os.Remove(target))
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if sqlutil.BecHash(data) == workingUnitLegacyDatabaseSHA256 && provenance.DatabaseSHA256 != workingUnitLegacyDatabaseSHA256 {
		if err := sqlutil.ReplaceBundledCatalogueFile(target, WorkingUnitDatabase, 0644); err != nil {
			return nil, err
		}
		data, err = os.ReadFile(target)
		if err != nil {
			return nil, err
		}
	}
	if sqlutil.BecHash(data) != provenance.DatabaseSHA256 {
		return nil, errors.New("existing Working Unit catalogue checksum mismatch; file was not replaced")
	}
	db, err := sqlutil.OpenReadOnly(target)
	if err != nil {
		return nil, err
	}
	if err := ValidateWorkingUnitDatabase(db, provenance); err != nil {
		db.Close()
		return nil, err
	}
	service := &WorkingUnitService{projects: projects, Db: db,
		SettingsPath: filepath.Join(configDir, "working-unit-settings.json"), ReplaceFile: os.Rename}
	if _, err := service.GetWorkingUnitMode(); err != nil {
		db.Close()
		return nil, err
	}
	return service, nil
}

func (s *WorkingUnitService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Db == nil {
		return nil
	}
	err := s.Db.Close()
	s.Db = nil
	return err
}

func validateWorkingUnitMode(mode string) error {
	switch mode {
	case "env", "master", "su":
		return nil
	default:
		return fmt.Errorf("invalid Working Unit mode %q; expected env, master or su", mode)
	}
}

func workingUnitChoiceStatus(row *WorkingUnitChoice) {
	row.Selectable = row.Code != nil && references.ValidateBECCode("UserSiteUnit", row.Code, 100) == nil
	row.Diagnostic = ""
	if !row.Selectable {
		row.Diagnostic = "Working Unit code is NULL, empty, or exceeds the stored 100-character limit"
	}
}

func (s *WorkingUnitService) masterChoices(ctx context.Context) ([]WorkingUnitChoice, error) {
	rows, err := s.Db.QueryContext(ctx, `SELECT RowID,Origin,SourceID,Code,Description,ScientificName,Level
 FROM WorkingUnits WHERE Origin='master' AND Level=11
 ORDER BY Code COLLATE NOCASE,Code,CAST(RowID AS INTEGER)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkingUnitChoice{}
	for rows.Next() {
		var row WorkingUnitChoice
		if err := rows.Scan(&row.RowID, &row.Origin, &row.SourceID, &row.Code, &row.Description, &row.ScientificName, &row.Level); err != nil {
			return nil, err
		}
		workingUnitChoiceStatus(&row)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *WorkingUnitService) GetMasterWorkingUnitChoices(ctx context.Context) ([]WorkingUnitChoice, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.Mu); err != nil {
		return nil, err
	}
	defer s.Mu.RUnlock()
	if s.Db == nil {
		return nil, errors.New("Working Unit catalogue is closed")
	}
	return s.masterChoices(ctx)
}

// Caller holds the project selection read lock through the complete operation.
func (s *WorkingUnitService) context() (ProjectInfo, string, error) {
	return s.contextWithCancellation(context.Background())
}

func (s *WorkingUnitService) contextWithCancellation(ctx context.Context) (ProjectInfo, string, error) {
	active, su := s.projects.Active, s.projects.ActiveSU
	if !ProjectNamePattern.MatchString(active) || (su != "None" && !ProjectNamePattern.MatchString(su)) {
		return ProjectInfo{}, "", errors.New("invalid active Working Unit project/SU context")
	}
	if s.projects.Sqlite != nil {
		if s.projects.Sqlite.Conn == nil {
			return ProjectInfo{}, "", errors.New("Working Unit project context is closed")
		}
		selection := s.projects.Sqlite.Selection
		return ProjectInfo{Name: selection.Project, File: filepath.Base(selection.ProjectPath),
			Path: selection.ProjectPath, Compatible: true, Version: "VP08"}, selection.SU, nil
	}
	projects, _, err := s.projects.DiscoverContext(ctx)
	if err != nil {
		return ProjectInfo{}, "", err
	}
	selected := CompatibleProject(projects, active)
	if selected.Name == "" {
		return ProjectInfo{}, "", errors.New("no compatible Working Unit project is active")
	}
	if su != "None" {
		units, err := s.projects.DiscoverSUsContext(ctx, selected)
		if err != nil {
			return ProjectInfo{}, "", err
		}
		if !hasCompatibleSU(units, su) {
			return ProjectInfo{}, "", fmt.Errorf("active SU %q is unavailable or unauthorized", su)
		}
	}
	return selected, su, nil
}

func workingUnitTextColumns(db workingUnitDB, table string, names ...string) error {
	rows, err := db.Query(`PRAGMA table_info(` + QuoteHeaderIdentifier(table) + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, typ string
		var value sql.NullString
		if err := rows.Scan(&index, &name, &typ, &notNull, &value, &primaryKey); err != nil {
			return err
		}
		typ = strings.ToUpper(typ)
		columns[name] = strings.Contains(typ, "CHAR") || strings.Contains(typ, "CLOB") || strings.Contains(typ, "TEXT")
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, name := range names {
		if !columns[name] {
			return fmt.Errorf("Working Unit project schema is missing verified text column %s.%s", table, name)
		}
	}
	return nil
}

func workingUnitSUAuthorized(db workingUnitDB, name string) error {
	if err := workingUnitTextColumns(db, name+"_SU", "PlotNumber", "SiteUnit"); err != nil {
		return err
	}
	var policy bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='_vpro_su_policy')`).Scan(&policy); err != nil {
		return err
	}
	if !policy {
		return nil
	}
	var kind string
	err := db.QueryRow(`SELECT kind FROM _vpro_su_policy WHERE table_name=?`, name+"_SU").Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if kind != "ordinary" && kind != "working" {
		return fmt.Errorf("SU %q is not authorized for Working Unit lookup", name)
	}
	return nil
}

func (s *WorkingUnitService) GetWorkingUnitChoices(ctx context.Context, mode string) ([]WorkingUnitChoice, error) {
	if err := validateWorkingUnitMode(mode); err != nil {
		return nil, err
	}
	if err := sqlutil.AcquireReadLease(ctx, &s.Mu); err != nil {
		return nil, err
	}
	defer s.Mu.RUnlock()
	if s.Db == nil {
		return nil, errors.New("Working Unit catalogue is closed")
	}
	if mode == "master" {
		return s.masterChoices(ctx)
	}
	if err := sqlutil.AcquireReadLease(ctx, &s.projects.Mu); err != nil {
		return nil, err
	}
	defer s.projects.Mu.RUnlock()
	project, su, err := s.contextWithCancellation(ctx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if mode == "su" && su == "None" {
		return nil, errors.New("select an authorized SU table before requesting Working Unit SU choices")
	}
	path := s.projects.ProjectFile(project)
	if mode == "su" && s.projects.Sqlite != nil {
		path = s.projects.Sqlite.Selection.SUPath
	}
	db, err := sqlutil.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	reader := sqlutil.ContextPlotDB{Db: tx, Ctx: ctx}
	query := ""
	if mode == "env" {
		if err := workingUnitTextColumns(reader, project.Name+"_Env", "PlotNumber"); err != nil {
			return nil, err
		}
		if err := workingUnitTextColumns(reader, project.Name+"_Admin", "Plot", "UserSiteUnit"); err != nil {
			return nil, err
		}
		query = `SELECT DISTINCT a.UserSiteUnit FROM ` + QuoteHeaderIdentifier(project.Name+"_Admin") +
			` AS a INNER JOIN ` + QuoteHeaderIdentifier(project.Name+"_Env") +
			` AS e ON e.PlotNumber=a.Plot WHERE a.UserSiteUnit IS NOT NULL ORDER BY a.UserSiteUnit COLLATE NOCASE,a.UserSiteUnit`
	} else {
		if err := workingUnitSUAuthorized(reader, su); err != nil {
			return nil, err
		}
		query = `SELECT DISTINCT SiteUnit FROM ` + QuoteHeaderIdentifier(su+"_SU") +
			` WHERE SiteUnit IS NOT NULL ORDER BY SiteUnit COLLATE NOCASE,SiteUnit`
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkingUnitChoice{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		row := WorkingUnitChoice{RowID: strconv.Itoa(len(result) + 1), Origin: mode, Code: &code}
		workingUnitChoiceStatus(&row)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
