package classification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	suCreationHistoryTable   = "__VPRO_SUCreationHistory"
	suCreationHistorySQL     = `CREATE TABLE "__VPRO_SUCreationHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
	suAssignmentHistoryTable = "__VPRO_SUEnvironmentHistory"
	suAssignmentHistorySQL   = `CREATE TABLE "__VPRO_SUEnvironmentHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
)

// BECdb v13 mask pattern: e.g. CWHvm1/01, SBSmk1/05, ESSFwc4/101, BGxh1/00, etc.
var becUnitMaskPattern = regexp.MustCompile(`^[A-Z]{2,4}[a-z]{1,3}[0-9a-z]{0,3}/[A-Za-z0-9.\-]{1,12}$`)

type CreateSUTableRequest struct {
	Name                    string `json:"name"`
	Destination             string `json:"destination"` // "project" (default) | "new_file"
	FilePath                string `json:"filePath"`
	PopulateAllProjectPlots bool   `json:"populateAllProjectPlots"`
	SeedFromUserSiteUnit    bool   `json:"seedFromUserSiteUnit"`
	Activate                bool   `json:"activate"`
}

func (r *CreateSUTableRequest) UnmarshalJSON(data []byte) error {
	type plain CreateSUTableRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type SaveSUTableAsRequest struct {
	SourceSU    string `json:"sourceSu"` // defaults to active SU when empty
	NewName     string `json:"newName"`
	Destination string `json:"destination"` // "project" (default) | "new_file"
	FilePath    string `json:"filePath"`
	Activate    bool   `json:"activate"`
}

func (r *SaveSUTableAsRequest) UnmarshalJSON(data []byte) error {
	type plain SaveSUTableAsRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type SUTableLifecycleResult struct {
	ContextID string                 `json:"contextId"`
	Name      string                 `json:"name"`
	TableName string                 `json:"tableName"`
	FilePath  string                 `json:"filePath"`
	PlotCount int                    `json:"plotCount"`
	Activated bool                   `json:"activated"`
	NewState  *vprocore.ProjectState `json:"newState,omitempty"`
}

type SUPlotLeaf struct {
	PlotNumber     string `json:"plotNumber"`
	FieldNumber    string `json:"fieldNumber"`
	Zone           string `json:"zone"`
	SubZone        string `json:"subZone"`
	SiteSeries     string `json:"siteSeries"`
	UserSiteUnit   string `json:"userSiteUnit"`
	MoistureRegime string `json:"moistureRegime"`
	NutrientRegime string `json:"nutrientRegime"`
	Elevation      *int64 `json:"elevation"`
	SiteUnit       string `json:"siteUnit"`
	PresentInSU    bool   `json:"presentInSu"`
	InProjectEnv   bool   `json:"inProjectEnv"`
	Locked         bool   `json:"locked"`
}

type SUUnitBucket struct {
	SiteUnit  string       `json:"siteUnit"`
	LongName  string       `json:"longName"`
	Origin    string       `json:"origin"` // "master" | "user" | ""
	MaskValid bool         `json:"maskValid"`
	PlotCount int          `json:"plotCount"`
	Plots     []SUPlotLeaf `json:"plots"`
}

type SUPlotTree struct {
	ContextID       string         `json:"contextId"`
	Project         string         `json:"project"`
	ProjectPath     string         `json:"projectPath"`
	SU              string         `json:"su"`
	SUPath          string         `json:"suPath"`
	Units           []SUUnitBucket `json:"units"`
	UnassignedPlots []SUPlotLeaf   `json:"unassignedPlots"`
	OrphanPlots     []SUPlotLeaf   `json:"orphanPlots"`
	TotalProject    int            `json:"totalProject"`
	TotalAssigned   int            `json:"totalAssigned"`
	TotalUnassigned int            `json:"totalUnassigned"`
}

type PlotInspectorSpecies struct {
	Species        string  `json:"species"`
	ScientificName string  `json:"scientificName"`
	EnglishName    string  `json:"englishName"`
	CoverA         string  `json:"coverA"`
	CoverB         string  `json:"coverB"`
	CoverC         string  `json:"coverC"`
	CoverD         string  `json:"coverD"`
	TotalCover     float64 `json:"totalCover"`
	TotalFormatted string  `json:"totalFormatted"`
}

type PlotInspectorSummary struct {
	ContextID       string                 `json:"contextId"`
	PlotNumber      string                 `json:"plotNumber"`
	FieldNumber     string                 `json:"fieldNumber"`
	ProjectID       string                 `json:"projectId"`
	Zone            string                 `json:"zone"`
	SubZone         string                 `json:"subZone"`
	SiteSeries      string                 `json:"siteSeries"`
	UserSiteUnit    string                 `json:"userSiteUnit"`
	BECSiteUnit     string                 `json:"becSiteUnit"`
	CurrentSUUnit   string                 `json:"currentSuUnit"`
	MoistureRegime  string                 `json:"moistureRegime"`
	NutrientRegime  string                 `json:"nutrientRegime"`
	Elevation       *int64                 `json:"elevation"`
	Slope           string                 `json:"slope"`
	Aspect          *int64                 `json:"aspect"`
	Latitude        *float64               `json:"latitude"`
	Longitude       *float64               `json:"longitude"`
	TopSpecies      []PlotInspectorSpecies `json:"topSpecies"`
}

type BECUnitPickerRequest struct {
	Query   string `json:"query"`
	Zone    string `json:"zone"`
	SubZone string `json:"subZone"`
	Limit   int    `json:"limit"`
}

func (r *BECUnitPickerRequest) UnmarshalJSON(data []byte) error {
	type plain BECUnitPickerRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type BECUnitPickerChoice struct {
	SiteSeries     string `json:"siteSeries"`
	LongName       string `json:"longName"`
	ScientificName string `json:"scientificName"`
	Zone           string `json:"zone"`
	SubZone        string `json:"subZone"`
	Alliance       string `json:"alliance,omitempty"`
	Suballiance    string `json:"suballiance,omitempty"`
	Source         string `json:"source"` // "master" | "user" | "becdb_v13"
	MaskValid      bool   `json:"maskValid"`
}

type AssignPlotsToSiteUnitRequest struct {
	PlotNumbers                []string `json:"plotNumbers"`
	TargetSiteUnit             *string  `json:"targetSiteUnit"` // nil or "" means Unassigned
	RemoveFromSUWhenUnassigned bool     `json:"removeFromSuWhenUnassigned"`
}

func (r *AssignPlotsToSiteUnitRequest) UnmarshalJSON(data []byte) error {
	type plain AssignPlotsToSiteUnitRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type suPlotAssignmentDelta struct {
	PlotNumber     string  `json:"plotNumber"`
	ExistedBefore  bool    `json:"existedBefore"`
	BeforeSiteUnit *string `json:"beforeSiteUnit"`
	ExistedAfter   bool    `json:"existedAfter"`
	AfterSiteUnit  *string `json:"afterSiteUnit"`
}

type suPlotAssignmentReceipt struct {
	Kind      string                  `json:"kind"`
	ContextID string                  `json:"contextId"`
	Project   string                  `json:"project"`
	SU        string                  `json:"su"`
	Deltas    []suPlotAssignmentDelta `json:"deltas"`
}

type AssignPlotsToSiteUnitResult struct {
	ContextID    string `json:"contextId"`
	SU           string `json:"su"`
	TableName    string `json:"tableName"`
	ChangedPlots int    `json:"changedPlots"`
	HistoryID    int64  `json:"historyId"`
}

func validateClassificationObjectName(rawName, label string) (string, error) {
	trimmed := strings.TrimSpace(rawName)
	if !utf8.ValidString(trimmed) || !vprocore.ProjectNamePattern.MatchString(trimmed) {
		return "", fmt.Errorf("%s name %q is invalid; use letters, digits, hyphens, or underscores", label, rawName)
	}
	if strings.EqualFold(trimmed, "None") || strings.EqualFold(trimmed, "Sample") || strings.EqualFold(trimmed, "USysSuTableDynamic") {
		return "", fmt.Errorf("%s name %q is reserved and cannot be created or overwritten", label, trimmed)
	}
	return trimmed, nil
}

func createSUTableObjectsInTx(ctx context.Context, tx *sql.Tx, suName string) error {
	tableName := suName + "_SU"
	var collision int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.sqlite_master WHERE name = ? COLLATE NOCASE`, tableName).Scan(&collision); err != nil {
		return err
	}
	if collision != 0 {
		return fmt.Errorf("table %s already exists in target database", tableName)
	}
	qTable := vprocore.QuoteHeaderIdentifier(tableName)
	qUniq := vprocore.QuoteHeaderIdentifier("uidx_" + tableName + "_PlotNumber")
	qIdx := vprocore.QuoteHeaderIdentifier("idx_" + tableName + "_SiteUnit")

	if _, err := tx.ExecContext(ctx, `CREATE TABLE main.`+qTable+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX main.`+qUniq+` ON `+qTable+` ("PlotNumber")`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX main.`+qIdx+` ON `+qTable+` ("SiteUnit")`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS main._table_metadata (table_name TEXT PRIMARY KEY, description TEXT)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO main._table_metadata (table_name, description) VALUES (?, 'VP04')`, tableName); err != nil {
		return err
	}
	return nil
}

func CreateSUTable(ctx context.Context, contexts *vprocore.ContextService, contextID string, request CreateSUTableRequest) (*SUTableLifecycleResult, error) {
	if ctx == nil {
		return nil, errors.New("SU creation requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	suName, err := validateClassificationObjectName(request.Name, "SU")
	if err != nil {
		return nil, err
	}
	dest := strings.ToLower(strings.TrimSpace(request.Destination))
	if dest == "" {
		if strings.TrimSpace(request.FilePath) != "" {
			dest = "new_file"
		} else {
			dest = "project"
		}
	}
	if dest != "project" && dest != "new_file" {
		return nil, fmt.Errorf("unsupported SU destination %q; expected 'project' or 'new_file'", request.Destination)
	}

	var out *SUTableLifecycleResult
	var targetPath string
	var currentSelection vprocore.DesktopSelection

	_, err = vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		currentSelection = owner.Selection
		project := owner.Selection.Project

		type seedPlot struct {
			plot string
			unit *string
		}
		var seeds []seedPlot

		if request.PopulateAllProjectPlots {
			if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
				return struct{}{}, err
			}
			tx, snapErr := owner.BeginReadSnapshot(ctx)
			if snapErr != nil {
				owner.Mu.Unlock()
				return struct{}{}, snapErr
			}
			adminUnits := map[string]string{}
			if request.SeedFromUserSiteUnit {
				aRows, aErr := tx.QueryContext(ctx,
					`SELECT CAST("Plot" AS BLOB), CAST("UserSiteUnit" AS TEXT)
					 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Admin")+`
					 WHERE typeof("Plot")='text' AND "UserSiteUnit" IS NOT NULL`)
				if aErr != nil {
					_ = tx.Rollback()
					owner.Mu.Unlock()
					return struct{}{}, aErr
				}
				for aRows.Next() {
					var rawPlot []byte
					var u sql.NullString
					if sErr := aRows.Scan(&rawPlot, &u); sErr != nil {
						aRows.Close()
						_ = tx.Rollback()
						owner.Mu.Unlock()
						return struct{}{}, sErr
					}
					key := string(rawPlot)
					if _, exists := adminUnits[key]; !exists && u.Valid {
						adminUnits[key] = u.String
					}
				}
				if aErr := errors.Join(aRows.Err(), aRows.Close()); aErr != nil {
					_ = tx.Rollback()
					owner.Mu.Unlock()
					return struct{}{}, aErr
				}
			}

			rows, qErr := tx.QueryContext(ctx,
				`SELECT CAST(env."PlotNumber" AS BLOB), CAST(env."PlotNumber" AS TEXT)
				 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Env")+` AS env
				 WHERE typeof(env."PlotNumber")='text' AND TRIM(env."PlotNumber") <> ''
				 ORDER BY env."PlotNumber" COLLATE NOCASE ASC, env.rowid ASC`)
			if qErr != nil {
				_ = tx.Rollback()
				owner.Mu.Unlock()
				return struct{}{}, qErr
			}
			seen := map[string]bool{}
			for rows.Next() {
				var rawPlot []byte
				var p string
				if sErr := rows.Scan(&rawPlot, &p); sErr != nil {
					rows.Close()
					_ = tx.Rollback()
					owner.Mu.Unlock()
					return struct{}{}, sErr
				}
				rawKey := string(rawPlot)
				if seen[rawKey] || len(utf16.Encode([]rune(p))) > 7 {
					continue
				}
				seen[rawKey] = true
				var unitPtr *string
				if request.SeedFromUserSiteUnit {
					if uStr, ok := adminUnits[rawKey]; ok && strings.TrimSpace(uStr) != "" && len(utf16.Encode([]rune(uStr))) <= 255 {
						val := uStr
						unitPtr = &val
					}
				}
				seeds = append(seeds, seedPlot{plot: p, unit: unitPtr})
			}
			rowsErr := rows.Err()
			rows.Close()
			_ = tx.Rollback()
			owner.Mu.Unlock()
			if rowsErr != nil {
				return struct{}{}, rowsErr
			}
		}

		tableName := suName + "_SU"
		proposalBytes, err := json.Marshal(request)
		if err != nil {
			return struct{}{}, err
		}

		if dest == "project" {
			targetPath = owner.Selection.ProjectPath
			err = owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
				tx, bErr := conn.BeginTx(ctx, nil)
				if bErr != nil {
					return bErr
				}
				defer func() {
					if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
						txErr = errors.Join(txErr, rb)
					}
				}()
				if err := createSUTableObjectsInTx(ctx, tx, suName); err != nil {
					return err
				}
				qTable := vprocore.QuoteHeaderIdentifier(tableName)
				if len(seeds) > 0 {
					stmt, pErr := tx.PrepareContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`)
					if pErr != nil {
						return pErr
					}
					defer stmt.Close()
					for _, s := range seeds {
						if _, err := stmt.ExecContext(ctx, s.plot, s.unit); err != nil {
							return err
						}
					}
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, suCreationHistoryTable, suCreationHistorySQL, string(proposalBytes), "SU table creation"); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return tx.Commit()
			})
			if err != nil {
				return struct{}{}, err
			}
		} else {
			freshPath, _, fErr := vprocore.FreshSQLiteDestination(request.FilePath, "SU")
			if fErr != nil {
				return struct{}{}, fErr
			}
			targetPath = freshPath
			committed := false
			db, oErr := sql.Open("sqlite3", sqlutil.SqliteFileURI(freshPath, "rwc")+"&_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate")
			if oErr != nil {
				return struct{}{}, oErr
			}
			defer func() {
				_ = db.Close()
				if !committed {
					_ = os.Remove(freshPath)
				}
			}()
			db.SetMaxOpenConns(1)
			tx, bErr := db.BeginTx(ctx, nil)
			if bErr != nil {
				return struct{}{}, bErr
			}
			if err := createSUTableObjectsInTx(ctx, tx, suName); err != nil {
				_ = tx.Rollback()
				return struct{}{}, err
			}
			qTable := vprocore.QuoteHeaderIdentifier(tableName)
			if len(seeds) > 0 {
				stmt, pErr := tx.PrepareContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`)
				if pErr != nil {
					_ = tx.Rollback()
					return struct{}{}, pErr
				}
				for _, s := range seeds {
					if _, err := stmt.ExecContext(ctx, s.plot, s.unit); err != nil {
						_ = stmt.Close()
						_ = tx.Rollback()
						return struct{}{}, err
					}
				}
				_ = stmt.Close()
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, suCreationHistoryTable, suCreationHistorySQL, string(proposalBytes), "SU file creation"); err != nil {
				_ = tx.Rollback()
				return struct{}{}, err
			}
			if err := tx.Commit(); err != nil {
				return struct{}{}, err
			}
			committed = true
		}

		out = &SUTableLifecycleResult{
			ContextID: contextID,
			Name:      suName,
			TableName: tableName,
			FilePath:  targetPath,
			PlotCount: len(seeds),
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	if request.Activate {
		nextState, swErr := contexts.SwitchContext(contextID, vprocore.ContextSelection{
			Project:       currentSelection.Project,
			ProjectPath:   currentSelection.ProjectPath,
			SU:            suName,
			SUPath:        targetPath,
			Hierarchy:     currentSelection.Hierarchy,
			HierarchyPath: currentSelection.HierarchyPath,
		})
		if swErr != nil {
			return nil, swErr
		}
		out.Activated = true
		out.ContextID = nextState.ContextID
		out.NewState = &nextState
	}
	return out, nil
}

func SaveSUTableAs(ctx context.Context, contexts *vprocore.ContextService, contextID string, request SaveSUTableAsRequest) (*SUTableLifecycleResult, error) {
	if ctx == nil {
		return nil, errors.New("SU save-as requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	newName, err := validateClassificationObjectName(request.NewName, "SU")
	if err != nil {
		return nil, err
	}
	dest := strings.ToLower(strings.TrimSpace(request.Destination))
	if dest == "" {
		if strings.TrimSpace(request.FilePath) != "" {
			dest = "new_file"
		} else {
			dest = "project"
		}
	}
	if dest != "project" && dest != "new_file" {
		return nil, fmt.Errorf("unsupported SU save-as destination %q", request.Destination)
	}

	var out *SUTableLifecycleResult
	var targetPath string
	var currentSelection vprocore.DesktopSelection

	_, err = vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		owner := plots.Projects.Sqlite
		currentSelection = owner.Selection
		srcName := strings.TrimSpace(request.SourceSU)
		if srcName == "" {
			srcName = owner.Selection.SU
		}
		if srcName == "" || srcName == "None" {
			return struct{}{}, errors.New("an active or explicit source SU table is required for Save As")
		}

		type suRowPair struct {
			plot *string
			unit *string
		}
		var copiedRows []suRowPair

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return struct{}{}, err
		}
		tx, snapErr := owner.BeginReadSnapshot(ctx)
		if snapErr != nil {
			owner.Mu.Unlock()
			return struct{}{}, snapErr
		}
		role := "su"
		if !strings.EqualFold(srcName, owner.Selection.SU) {
			role = "project"
		}
		srcTable := srcName + "_SU"
		rows, qErr := tx.QueryContext(ctx,
			`SELECT CAST("PlotNumber" AS TEXT), CAST("SiteUnit" AS TEXT) FROM `+
				vprocore.QuoteHeaderIdentifier(role)+`.`+vprocore.QuoteHeaderIdentifier(srcTable)+` ORDER BY rowid ASC`)
		if qErr != nil {
			_ = tx.Rollback()
			owner.Mu.Unlock()
			return struct{}{}, fmt.Errorf("read source SU %s: %w", srcTable, qErr)
		}
		for rows.Next() {
			var p, u sql.NullString
			if sErr := rows.Scan(&p, &u); sErr != nil {
				rows.Close()
				_ = tx.Rollback()
				owner.Mu.Unlock()
				return struct{}{}, sErr
			}
			var pp, up *string
			if p.Valid {
				v := p.String
				pp = &v
			}
			if u.Valid {
				v := u.String
				up = &v
			}
			copiedRows = append(copiedRows, suRowPair{plot: pp, unit: up})
		}
		rowsErr := rows.Err()
		rows.Close()
		_ = tx.Rollback()
		owner.Mu.Unlock()
		if rowsErr != nil {
			return struct{}{}, rowsErr
		}

		targetTable := newName + "_SU"
		proposalBytes, err := json.Marshal(request)
		if err != nil {
			return struct{}{}, err
		}

		if dest == "project" {
			targetPath = owner.Selection.ProjectPath
			err = owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
				wtx, bErr := conn.BeginTx(ctx, nil)
				if bErr != nil {
					return bErr
				}
				defer func() {
					if rb := wtx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
						txErr = errors.Join(txErr, rb)
					}
				}()
				if err := createSUTableObjectsInTx(ctx, wtx, newName); err != nil {
					return err
				}
				qTarget := vprocore.QuoteHeaderIdentifier(targetTable)
				for _, r := range copiedRows {
					if _, err := wtx.ExecContext(ctx, `INSERT INTO main.`+qTarget+` ("PlotNumber","SiteUnit") VALUES (?,?)`, r.plot, r.unit); err != nil {
						return err
					}
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, wtx, suCreationHistoryTable, suCreationHistorySQL, string(proposalBytes), "SU save-as"); err != nil {
					return err
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				return wtx.Commit()
			})
			if err != nil {
				return struct{}{}, err
			}
		} else {
			freshPath, _, fErr := vprocore.FreshSQLiteDestination(request.FilePath, "SU")
			if fErr != nil {
				return struct{}{}, fErr
			}
			targetPath = freshPath
			committed := false
			db, oErr := sql.Open("sqlite3", sqlutil.SqliteFileURI(freshPath, "rwc")+"&_foreign_keys=on&_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate")
			if oErr != nil {
				return struct{}{}, oErr
			}
			defer func() {
				_ = db.Close()
				if !committed {
					_ = os.Remove(freshPath)
				}
			}()
			db.SetMaxOpenConns(1)
			wtx, bErr := db.BeginTx(ctx, nil)
			if bErr != nil {
				return struct{}{}, bErr
			}
			if err := createSUTableObjectsInTx(ctx, wtx, newName); err != nil {
				_ = wtx.Rollback()
				return struct{}{}, err
			}
			qTarget := vprocore.QuoteHeaderIdentifier(targetTable)
			for _, r := range copiedRows {
				if _, err := wtx.ExecContext(ctx, `INSERT INTO main.`+qTarget+` ("PlotNumber","SiteUnit") VALUES (?,?)`, r.plot, r.unit); err != nil {
					_ = wtx.Rollback()
					return struct{}{}, err
				}
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, wtx, suCreationHistoryTable, suCreationHistorySQL, string(proposalBytes), "SU file save-as"); err != nil {
				_ = wtx.Rollback()
				return struct{}{}, err
			}
			if err := wtx.Commit(); err != nil {
				return struct{}{}, err
			}
			committed = true
		}

		out = &SUTableLifecycleResult{
			ContextID: contextID,
			Name:      newName,
			TableName: targetTable,
			FilePath:  targetPath,
			PlotCount: len(copiedRows),
		}
		return struct{}{}, nil
	})
	if err != nil {
		return nil, err
	}

	if request.Activate {
		nextState, swErr := contexts.SwitchContext(contextID, vprocore.ContextSelection{
			Project:       currentSelection.Project,
			ProjectPath:   currentSelection.ProjectPath,
			SU:            newName,
			SUPath:        targetPath,
			Hierarchy:     currentSelection.Hierarchy,
			HierarchyPath: currentSelection.HierarchyPath,
		})
		if swErr != nil {
			return nil, swErr
		}
		out.Activated = true
		out.ContextID = nextState.ContextID
		out.NewState = &nextState
	}
	return out, nil
}

func BECUnitMaskValid(code string) bool {
	return becUnitMaskPattern.MatchString(strings.TrimSpace(code))
}

func GetSUPlotTree(ctx context.Context, contexts *vprocore.ContextService, contextID, filterText string) (*SUPlotTree, error) {
	if ctx == nil {
		return nil, errors.New("SU plot tree requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	if !utf8.ValidString(filterText) || strings.ContainsRune(filterText, 0) {
		return nil, errors.New("filter text must be valid UTF-8 without NUL")
	}
	needle := strings.ToLower(strings.TrimSpace(filterText))

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *SUPlotTree, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, rb)
			}
		}()

		project := owner.Selection.Project
		adminUserUnits := map[string]string{}
		adminRows, err := tx.QueryContext(ctx,
			`SELECT CAST("Plot" AS BLOB), COALESCE(CAST("UserSiteUnit" AS TEXT), '')
			 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Admin")+`
			 WHERE typeof("Plot")='text'`)
		if err != nil {
			return nil, err
		}
		for adminRows.Next() {
			var rawPlot []byte
			var userUnit string
			if err := adminRows.Scan(&rawPlot, &userUnit); err != nil {
				adminRows.Close()
				return nil, err
			}
			key := string(rawPlot)
			if _, exists := adminUserUnits[key]; !exists {
				adminUserUnits[key] = userUnit
			}
		}
		if err := errors.Join(adminRows.Err(), adminRows.Close()); err != nil {
			return nil, err
		}

		envMap := map[string]SUPlotLeaf{}
		envOrder := []string{}

		envRows, err := tx.QueryContext(ctx,
			`SELECT typeof(env."PlotNumber"),
			        CAST(env."PlotNumber" AS BLOB),
			        CAST(env."PlotNumber" AS TEXT),
			        COALESCE(CAST(env."FieldNumber" AS TEXT), ''),
			        COALESCE(CAST(env."Zone" AS TEXT), ''),
			        COALESCE(CAST(env."SubZone" AS TEXT), ''),
			        COALESCE(CAST(env."SiteSeries" AS TEXT), ''),
			        COALESCE(CAST(env."MoistureRegime" AS TEXT), ''),
			        COALESCE(CAST(env."NutrientRegime" AS TEXT), ''),
			        env."Elevation",
			        COALESCE(CAST(env."Flag" AS INTEGER), 0)
			 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Env")+` AS env
			 WHERE env."PlotNumber" IS NOT NULL
			 ORDER BY env."PlotNumber" COLLATE NOCASE ASC, env.rowid ASC`)
		if err != nil {
			return nil, err
		}
		for envRows.Next() {
			var plotType string
			var rawPlot []byte
			var leaf SUPlotLeaf
			var elev sql.NullInt64
			var flagInt int64
			if err := envRows.Scan(
				&plotType, &rawPlot, &leaf.PlotNumber, &leaf.FieldNumber, &leaf.Zone, &leaf.SubZone, &leaf.SiteSeries,
				&leaf.MoistureRegime, &leaf.NutrientRegime, &elev, &flagInt,
			); err != nil {
				envRows.Close()
				return nil, err
			}
			if elev.Valid {
				v := elev.Int64
				leaf.Elevation = &v
			}
			leaf.InProjectEnv = true
			leaf.Locked = flagInt != 0
			typedKey := plotType + ":" + leaf.PlotNumber
			if plotType == "text" {
				rawKey := string(rawPlot)
				typedKey = "text:" + rawKey
				leaf.UserSiteUnit = adminUserUnits[rawKey]
			}
			if _, exists := envMap[typedKey]; !exists {
				envOrder = append(envOrder, typedKey)
			}
			envMap[typedKey] = leaf
		}
		if err := errors.Join(envRows.Err(), envRows.Close()); err != nil {
			return nil, err
		}

		// Load unit long names from MasterSiteUnitList & UserSiteUnitList
		type unitCatalogMeta struct {
			longName string
			origin   string
		}
		unitNames := map[string]unitCatalogMeta{}
		for _, src := range []struct {
			role, table, origin string
		}{
			{"VLists", "MasterSiteUnitList", "master"},
			{"VUser", "UserSiteUnitList", "user"},
		} {
			uRows, uErr := tx.QueryContext(ctx,
				`SELECT COALESCE(CAST("SiteSeries" AS TEXT), ''), COALESCE(CAST("SiteSeriesLongName" AS TEXT), '')
				 FROM `+vprocore.QuoteHeaderIdentifier(src.role)+`.`+vprocore.QuoteHeaderIdentifier(src.table))
			if uErr == nil {
				for uRows.Next() {
					var code, lname string
					if err := uRows.Scan(&code, &lname); err == nil && strings.TrimSpace(code) != "" {
						unitNames[code] = unitCatalogMeta{longName: lname, origin: src.origin}
					}
				}
				uRows.Close()
			}
		}

		tree := &SUPlotTree{
			ContextID:       contextID,
			Project:         project,
			ProjectPath:     owner.Selection.ProjectPath,
			SU:              owner.Selection.SU,
			SUPath:          owner.Selection.SUPath,
			Units:           []SUUnitBucket{},
			UnassignedPlots: []SUPlotLeaf{},
			OrphanPlots:     []SUPlotLeaf{},
			TotalProject:    len(envOrder),
		}

		matchesFilter := func(leaf SUPlotLeaf, unitLongName string) bool {
			if needle == "" {
				return true
			}
			hay := strings.ToLower(strings.Join([]string{
				leaf.PlotNumber, leaf.FieldNumber, leaf.SiteUnit, unitLongName,
				leaf.Zone, leaf.SubZone, leaf.SiteSeries, leaf.UserSiteUnit,
			}, " "))
			return strings.Contains(hay, needle)
		}

		if owner.Selection.SU == "None" || owner.Selection.SU == "" {
			for _, plotNum := range envOrder {
				leaf := envMap[plotNum]
				tree.TotalUnassigned++
				if matchesFilter(leaf, "") {
					tree.UnassignedPlots = append(tree.UnassignedPlots, leaf)
				}
			}
		} else {
			suTable := owner.Selection.SU + "_SU"
			suRows, sErr := tx.QueryContext(ctx,
				`SELECT typeof("PlotNumber"), CAST("PlotNumber" AS BLOB), CAST("PlotNumber" AS TEXT), CAST("SiteUnit" AS TEXT)
				 FROM "su".`+vprocore.QuoteHeaderIdentifier(suTable)+` ORDER BY rowid ASC`)
			if sErr != nil {
				return nil, sErr
			}
			seenSUPlot := map[string]bool{}
			bucketMap := map[string]*SUUnitBucket{}

			for suRows.Next() {
				var plotType string
				var rawPlot []byte
				var p, u sql.NullString
				if err := suRows.Scan(&plotType, &rawPlot, &p, &u); err != nil {
					suRows.Close()
					return nil, err
				}
				if plotType == "null" || !p.Valid || strings.TrimSpace(p.String) == "" {
					continue
				}
				plotNum := p.String
				unitCode := ""
				if u.Valid {
					unitCode = strings.TrimSpace(u.String)
				}
				if plotType != "text" {
					orphan := SUPlotLeaf{
						PlotNumber:   plotNum,
						SiteUnit:     unitCode,
						PresentInSU:  true,
						InProjectEnv: false,
					}
					if matchesFilter(orphan, unitNames[unitCode].longName) {
						tree.OrphanPlots = append(tree.OrphanPlots, orphan)
					}
					continue
				}
				typedKey := "text:" + string(rawPlot)
				seenSUPlot[typedKey] = true
				baseLeaf, inEnv := envMap[typedKey]
				if !inEnv {
					orphan := SUPlotLeaf{
						PlotNumber:   plotNum,
						SiteUnit:     unitCode,
						PresentInSU:  true,
						InProjectEnv: false,
					}
					if matchesFilter(orphan, unitNames[unitCode].longName) {
						tree.OrphanPlots = append(tree.OrphanPlots, orphan)
					}
					continue
				}
				baseLeaf.PresentInSU = true
				baseLeaf.SiteUnit = unitCode

				if unitCode == "" {
					tree.TotalUnassigned++
					if matchesFilter(baseLeaf, "") {
						tree.UnassignedPlots = append(tree.UnassignedPlots, baseLeaf)
					}
					continue
				}

				tree.TotalAssigned++
				meta := unitNames[unitCode]
				if !matchesFilter(baseLeaf, meta.longName) {
					continue
				}
				bucket := bucketMap[unitCode]
				if bucket == nil {
					bucket = &SUUnitBucket{
						SiteUnit:  unitCode,
						LongName:  meta.longName,
						Origin:    meta.origin,
						MaskValid: BECUnitMaskValid(unitCode),
						Plots:     []SUPlotLeaf{},
					}
					bucketMap[unitCode] = bucket
				}
				bucket.Plots = append(bucket.Plots, baseLeaf)
				bucket.PlotCount = len(bucket.Plots)
			}
			if err := errors.Join(suRows.Err(), suRows.Close()); err != nil {
				return nil, err
			}

			// Any project plot absent from <SU>_SU belongs in UnassignedPlots!
			for _, plotNum := range envOrder {
				if !seenSUPlot[plotNum] {
					leaf := envMap[plotNum]
					leaf.PresentInSU = false
					tree.TotalUnassigned++
					if matchesFilter(leaf, "") {
						tree.UnassignedPlots = append(tree.UnassignedPlots, leaf)
					}
				}
			}

			unitKeys := make([]string, 0, len(bucketMap))
			for k := range bucketMap {
				unitKeys = append(unitKeys, k)
			}
			slices.SortFunc(unitKeys, func(a, b string) int {
				if cmp := strings.Compare(strings.ToLower(a), strings.ToLower(b)); cmp != 0 {
					return cmp
				}
				return strings.Compare(a, b)
			})
			for _, k := range unitKeys {
				b := bucketMap[k]
				slices.SortFunc(b.Plots, func(x, y SUPlotLeaf) int {
					return strings.Compare(x.PlotNumber, y.PlotNumber)
				})
				tree.Units = append(tree.Units, *b)
			}
		}

		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return tree, nil
	})
}

func GetPlotInspectorSummary(ctx context.Context, contexts *vprocore.ContextService, contextID, plotNumber string) (*PlotInspectorSummary, error) {
	if ctx == nil {
		return nil, errors.New("plot inspector requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	if plotNumber == "" || !utf8.ValidString(plotNumber) || strings.ContainsRune(plotNumber, 0) {
		return nil, errors.New("plot inspector requires a valid literal plot number")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *PlotInspectorSummary, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, rb)
			}
		}()

		project := owner.Selection.Project
		summary := &PlotInspectorSummary{
			ContextID:  contextID,
			PlotNumber: plotNumber,
			TopSpecies: []PlotInspectorSpecies{},
		}

		var elev, aspect sql.NullInt64
		var slope, lat, lon sql.NullFloat64
		err = tx.QueryRowContext(ctx,
			`SELECT CAST(env."PlotNumber" AS TEXT),
			        COALESCE(CAST(env."FieldNumber" AS TEXT), ''),
			        COALESCE(CAST(env."ProjectID" AS TEXT), ''),
			        COALESCE(CAST(env."Zone" AS TEXT), ''),
			        COALESCE(CAST(env."SubZone" AS TEXT), ''),
			        COALESCE(CAST(env."SiteSeries" AS TEXT), ''),
			        COALESCE(CAST(admin."UserSiteUnit" AS TEXT), ''),
			        COALESCE(CAST(admin."BECSiteUnit" AS TEXT), ''),
			        COALESCE(CAST(env."MoistureRegime" AS TEXT), ''),
			        COALESCE(CAST(env."NutrientRegime" AS TEXT), ''),
			        env."Elevation",
			        env."SlopeGradient",
			        env."Aspect",
			        env."Latitude",
			        env."Longitude"
			 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Env")+` AS env
			 LEFT JOIN "project".`+vprocore.QuoteHeaderIdentifier(project+"_Admin")+` AS admin
			   ON admin."Plot" = env."PlotNumber" AND typeof(admin."Plot")='text' AND CAST(admin."Plot" AS BLOB)=CAST(env."PlotNumber" AS BLOB)
			 WHERE env."PlotNumber" = ? AND typeof(env."PlotNumber")='text' AND CAST(env."PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
			plotNumber, plotNumber,
		).Scan(
			&summary.PlotNumber, &summary.FieldNumber, &summary.ProjectID,
			&summary.Zone, &summary.SubZone, &summary.SiteSeries,
			&summary.UserSiteUnit, &summary.BECSiteUnit,
			&summary.MoistureRegime, &summary.NutrientRegime,
			&elev, &slope, &aspect, &lat, &lon,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("plot %q not found in project %s", plotNumber, project)
		}
		if err != nil {
			return nil, err
		}
		if elev.Valid {
			v := elev.Int64
			summary.Elevation = &v
		}
		if slope.Valid {
			summary.Slope = reportcore.SiteUnitFixedNumber(slope.Float64, 1, 1)
		}
		if aspect.Valid {
			v := aspect.Int64
			summary.Aspect = &v
		}
		if lat.Valid {
			v := lat.Float64
			summary.Latitude = &v
		}
		if lon.Valid {
			v := lon.Float64
			summary.Longitude = &v
		}

		if owner.Selection.SU != "None" && owner.Selection.SU != "" {
			var suUnit sql.NullString
			suErr := tx.QueryRowContext(ctx,
				`SELECT CAST("SiteUnit" AS TEXT) FROM "su".`+vprocore.QuoteHeaderIdentifier(owner.Selection.SU+"_SU")+
					` WHERE "PlotNumber" = ? AND typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
				plotNumber, plotNumber,
			).Scan(&suUnit)
			if suErr == nil && suUnit.Valid {
				summary.CurrentSUUnit = suUnit.String
			}
		}

		// Aggregate top species covers across A/B/C/D strata with calibrated ACE 15-sig-digit formatting
		vegRows, err := tx.QueryContext(ctx,
			`SELECT COALESCE(CAST(v."Species" AS TEXT), ''),
			        COALESCE(v."TotalA", COALESCE(v."Cover1",0)+COALESCE(v."Cover2",0)+COALESCE(v."Cover3",0)),
			        COALESCE(v."TotalB", COALESCE(v."Cover4",0)+COALESCE(v."Cover5",0)+COALESCE(v."Cover5a",0)+COALESCE(v."Cover5b",0)+COALESCE(v."Cover5c",0)),
			        COALESCE(v."Cover6", 0),
			        COALESCE(v."Cover7", 0)+COALESCE(v."Cover8", 0)+COALESCE(v."Cover9", 0)+COALESCE(v."Cover10", 0)
			 FROM "project".`+vprocore.QuoteHeaderIdentifier(project+"_Veg")+` AS v
			 WHERE v."PlotNumber" = ? AND typeof(v."PlotNumber")='text' AND CAST(v."PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
			plotNumber, plotNumber,
		)
		if err != nil {
			return nil, err
		}
		type aggSp struct {
			code       string
			a, b, c, d float64
		}
		aggMap := map[string]*aggSp{}
		for vegRows.Next() {
			var sp string
			var ca, cb, cc, cd float64
			if err := vegRows.Scan(&sp, &ca, &cb, &cc, &cd); err != nil {
				vegRows.Close()
				return nil, err
			}
			upper := strings.ToUpper(strings.TrimSpace(sp))
			if upper == "" {
				continue
			}
			item := aggMap[upper]
			if item == nil {
				item = &aggSp{code: strings.TrimSpace(sp)}
				aggMap[upper] = item
			}
			item.a += ca
			item.b += cb
			item.c += cc
			item.d += cd
		}
		if err := errors.Join(vegRows.Err(), vegRows.Close()); err != nil {
			return nil, err
		}

		aggList := make([]*aggSp, 0, len(aggMap))
		for _, item := range aggMap {
			aggList = append(aggList, item)
		}
		slices.SortFunc(aggList, func(x, y *aggSp) int {
			txTotal := x.a + x.b + x.c + x.d
			tyTotal := y.a + y.b + y.c + y.d
			if txTotal > tyTotal {
				return -1
			}
			if txTotal < tyTotal {
				return 1
			}
			return strings.Compare(x.code, y.code)
		})
		if len(aggList) > 15 {
			aggList = aggList[:15]
		}

		for _, item := range aggList {
			var sci, eng sql.NullString
			_ = tx.QueryRowContext(ctx,
				`SELECT CAST("ScientificName" AS TEXT), CAST("EnglishName" AS TEXT)
				 FROM "VLists"."USysAllSpecs" WHERE UPPER("Code") = UPPER(?) AND COALESCE("Codetype",'u') <> 's' LIMIT 1`,
				item.code,
			).Scan(&sci, &eng)
			if !sci.Valid {
				_ = tx.QueryRowContext(ctx,
					`SELECT CAST("ScientificName" AS TEXT), CAST("EnglishName" AS TEXT)
					 FROM "VUser"."USysUserSpp" WHERE UPPER("Code") = UPPER(?) LIMIT 1`,
					item.code,
				).Scan(&sci, &eng)
			}
			tot := item.a + item.b + item.c + item.d
			fmtStratum := func(v float64) string {
				if v == 0 {
					return ""
				}
				return reportcore.SiteUnitFixedNumber(v, 1, 1)
			}
			summary.TopSpecies = append(summary.TopSpecies, PlotInspectorSpecies{
				Species:        item.code,
				ScientificName: sci.String,
				EnglishName:    eng.String,
				CoverA:         fmtStratum(item.a),
				CoverB:         fmtStratum(item.b),
				CoverC:         fmtStratum(item.c),
				CoverD:         fmtStratum(item.d),
				TotalCover:     tot,
				TotalFormatted: reportcore.SiteUnitFixedNumber(tot, 1, 1),
			})
		}

		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return summary, nil
	})
}

func splitZoneSubZoneFromUnitCode(code string) (string, string) {
	slashIdx := strings.IndexByte(code, '/')
	prefix := code
	if slashIdx >= 0 {
		prefix = code[:slashIdx]
	}
	i := 0
	for i < len(prefix) && prefix[i] >= 'A' && prefix[i] <= 'Z' {
		i++
	}
	return prefix[:i], prefix[i:]
}

func SearchBECUnitPicker(
	ctx context.Context,
	contexts *vprocore.ContextService,
	bec *references.BECService,
	contextID string,
	request BECUnitPickerRequest,
) ([]BECUnitPickerChoice, error) {
	if ctx == nil {
		return nil, errors.New("BEC unit picker requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := request.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := strings.ToLower(strings.TrimSpace(request.Query))
	zFilter := strings.ToUpper(strings.TrimSpace(request.Zone))
	szFilter := strings.ToLower(strings.TrimSpace(request.SubZone))

	seen := map[string]bool{}
	choices := []BECUnitPickerChoice{}

	// 1. Include official BECdb v13 units from BECService when available
	if bec != nil {
		var zPtr, szPtr *string
		if zFilter != "" {
			zCopy := zFilter
			zPtr = &zCopy
		}
		if szFilter != "" {
			szCopy := strings.TrimSpace(request.SubZone)
			szPtr = &szCopy
		}
		if seriesList, bErr := bec.ListBECSiteSeries(ctx, zPtr, szPtr); bErr == nil {
			for _, item := range seriesList {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if item.SSCode == nil || strings.TrimSpace(*item.SSCode) == "" {
					continue
				}
				code := strings.TrimSpace(*item.SSCode)
				lname := ""
				if item.Name != nil {
					lname = *item.Name
				}
				sci := ""
				if item.PlantAssociation != nil {
					sci = *item.PlantAssociation
				}
				zoneVal := ""
				if item.Zone != nil {
					zoneVal = *item.Zone
				}
				subVal := ""
				if item.SubZone != nil {
					subVal = *item.SubZone
				}
				alliance := ""
				if item.Alliance != nil {
					alliance = *item.Alliance
				}
				suballiance := ""
				if item.Suballiance != nil {
					suballiance = *item.Suballiance
				}
				if q != "" {
					hay := strings.ToLower(code + " " + lname + " " + sci + " " + alliance)
					if !strings.Contains(hay, q) {
						continue
					}
				}
				upper := strings.ToUpper(code)
				if seen[upper] {
					continue
				}
				seen[upper] = true
				choices = append(choices, BECUnitPickerChoice{
					SiteSeries:     code,
					LongName:       lname,
					ScientificName: sci,
					Zone:           zoneVal,
					SubZone:        subVal,
					Alliance:       alliance,
					Suballiance:    suballiance,
					Source:         "becdb_v13",
					MaskValid:      BECUnitMaskValid(code),
				})
				if len(choices) >= limit {
					return choices, nil
				}
			}
		}
	}

	// 2. Supplement with VUser.UserSiteUnitList and VLists.MasterSiteUnitList from active context
	if contexts != nil && contextID != "" {
		extra, err := vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ []BECUnitPickerChoice, resultErr error) {
			owner := plots.Projects.Sqlite
			if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
				return nil, err
			}
			defer owner.Mu.Unlock()
			tx, sErr := owner.BeginReadSnapshot(ctx)
			if sErr != nil {
				return nil, sErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					resultErr = errors.Join(resultErr, rb)
				}
			}()

			var found []BECUnitPickerChoice
			for _, src := range []struct {
				role, table, tag string
			}{
				{"VUser", "UserSiteUnitList", "user"},
				{"VLists", "MasterSiteUnitList", "master"},
			} {
				rows, qErr := tx.QueryContext(ctx,
					`SELECT COALESCE(CAST("SiteSeries" AS TEXT),''),
					        COALESCE(CAST("SiteSeriesLongName" AS TEXT),''),
					        COALESCE(CAST("SiteSeriesScientificName" AS TEXT),'')
					 FROM `+vprocore.QuoteHeaderIdentifier(src.role)+`.`+vprocore.QuoteHeaderIdentifier(src.table)+
						` ORDER BY "SiteSeries" COLLATE NOCASE ASC`)
				if qErr != nil {
					continue
				}
				for rows.Next() {
					var code, lname, sci string
					if err := rows.Scan(&code, &lname, &sci); err != nil {
						continue
					}
					code = strings.TrimSpace(code)
					if code == "" {
						continue
					}
					z, sz := splitZoneSubZoneFromUnitCode(code)
					if zFilter != "" && !strings.EqualFold(z, zFilter) {
						continue
					}
					if szFilter != "" && !strings.HasPrefix(strings.ToLower(sz), szFilter) {
						continue
					}
					if q != "" && !strings.Contains(strings.ToLower(code+" "+lname+" "+sci), q) {
						continue
					}
					upper := strings.ToUpper(code)
					if seen[upper] {
						continue
					}
					seen[upper] = true
					found = append(found, BECUnitPickerChoice{
						SiteSeries:     code,
						LongName:       lname,
						ScientificName: sci,
						Zone:           z,
						SubZone:        sz,
						Source:         src.tag,
						MaskValid:      BECUnitMaskValid(code),
					})
					if len(choices)+len(found) >= limit {
						rows.Close()
						_ = tx.Commit()
						return found, nil
					}
				}
				rows.Close()
			}
			_ = tx.Commit()
			return found, nil
		})
		if err == nil {
			choices = append(choices, extra...)
		}
	}

	return choices, nil
}

func AssignPlotsToSiteUnit(ctx context.Context, contexts *vprocore.ContextService, contextID string, request AssignPlotsToSiteUnitRequest) (*AssignPlotsToSiteUnitResult, error) {
	if ctx == nil {
		return nil, errors.New("SU plot assignment requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}
	if len(request.PlotNumbers) == 0 {
		return nil, errors.New("select at least one plot to assign")
	}

	var normalizedUnit *string
	if request.TargetSiteUnit != nil {
		trimmed := strings.TrimSpace(*request.TargetSiteUnit)
		if !utf8.ValidString(trimmed) || strings.ContainsRune(trimmed, 0) {
			return nil, errors.New("SiteUnit must be valid UTF-8 text without NUL")
		}
		if len(utf16.Encode([]rune(trimmed))) > 255 {
			return nil, errors.New("SiteUnit exceeds 255 UTF-16 code units")
		}
		if trimmed != "" {
			normalizedUnit = &trimmed
		}
	}

	uniquePlots := make([]string, 0, len(request.PlotNumbers))
	seenPlot := map[string]bool{}
	for _, p := range request.PlotNumbers {
		if p == "" || !utf8.ValidString(p) || strings.ContainsRune(p, 0) {
			return nil, fmt.Errorf("invalid plot number %q", p)
		}
		if len(utf16.Encode([]rune(p))) > 7 {
			return nil, fmt.Errorf("plot number %q exceeds 7 UTF-16 code units", p)
		}
		if !seenPlot[p] {
			seenPlot[p] = true
			uniquePlots = append(uniquePlots, p)
		}
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*AssignPlotsToSiteUnitResult, error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.SU == "" || owner.Selection.SU == "None" {
			return nil, errors.New("an active Site Unit (SU) table must be selected before assigning plots")
		}
		suName := owner.Selection.SU
		suTable := suName + "_SU"
		qTable := vprocore.QuoteHeaderIdentifier(suTable)

		validateFiles := func() error {
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return err
			}
			suInfo := owner.AttachmentInfo["su"]
			if suInfo == nil {
				return errors.New("active SU file is not attached")
			}
			for role, info := range owner.AttachmentInfo {
				if strings.HasPrefix(role, "V") && os.SameFile(suInfo, info) {
					return fmt.Errorf("SU database shares a file with support role %s; writes are prohibited", role)
				}
			}
			return nil
		}

		res := &AssignPlotsToSiteUnitResult{
			ContextID: contextID,
			SU:        suName,
			TableName: suTable,
		}

		err := owner.WithSelectedFileWriter(ctx, "su", validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil) // BEGIN IMMEDIATE
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			deltas := make([]suPlotAssignmentDelta, 0, len(uniquePlots))
			for _, plotNum := range uniquePlots {
				if err := ctx.Err(); err != nil {
					return err
				}
				var rowID int64
				var existing sql.NullString
				qErr := tx.QueryRowContext(ctx,
					`SELECT rowid, CAST("SiteUnit" AS TEXT) FROM main.`+qTable+
						` WHERE "PlotNumber" = ? AND typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
					plotNum, plotNum,
				).Scan(&rowID, &existing)

				existedBefore := qErr == nil
				if qErr != nil && !errors.Is(qErr, sql.ErrNoRows) {
					return qErr
				}
				var beforePtr *string
				if existedBefore && existing.Valid {
					v := existing.String
					beforePtr = &v
				}

				if normalizedUnit == nil {
					// Moving to Unassigned
					if !existedBefore {
						continue
					}
					if request.RemoveFromSUWhenUnassigned {
						if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+qTable+` WHERE rowid = ?`, rowID); err != nil {
							return err
						}
						deltas = append(deltas, suPlotAssignmentDelta{
							PlotNumber:     plotNum,
							ExistedBefore:  true,
							BeforeSiteUnit: beforePtr,
							ExistedAfter:   false,
							AfterSiteUnit:  nil,
						})
						res.ChangedPlots++
					} else {
						if beforePtr == nil || *beforePtr == "" {
							continue
						}
						if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "SiteUnit" = NULL WHERE rowid = ?`, rowID); err != nil {
							return err
						}
						deltas = append(deltas, suPlotAssignmentDelta{
							PlotNumber:     plotNum,
							ExistedBefore:  true,
							BeforeSiteUnit: beforePtr,
							ExistedAfter:   true,
							AfterSiteUnit:  nil,
						})
						res.ChangedPlots++
					}
				} else {
					// Moving to TargetSiteUnit
					afterCopy := *normalizedUnit
					if existedBefore {
						if beforePtr != nil && *beforePtr == afterCopy {
							continue
						}
						if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "SiteUnit" = ? WHERE rowid = ?`, afterCopy, rowID); err != nil {
							return err
						}
						deltas = append(deltas, suPlotAssignmentDelta{
							PlotNumber:     plotNum,
							ExistedBefore:  true,
							BeforeSiteUnit: beforePtr,
							ExistedAfter:   true,
							AfterSiteUnit:  &afterCopy,
						})
						res.ChangedPlots++
					} else {
						if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`, plotNum, afterCopy); err != nil {
							return err
						}
						deltas = append(deltas, suPlotAssignmentDelta{
							PlotNumber:     plotNum,
							ExistedBefore:  false,
							BeforeSiteUnit: nil,
							ExistedAfter:   true,
							AfterSiteUnit:  &afterCopy,
						})
						res.ChangedPlots++
					}
				}
			}

			if len(deltas) > 0 {
				receipt := suPlotAssignmentReceipt{
					Kind:      "su_plot_assignment",
					ContextID: contextID,
					Project:   owner.Selection.Project,
					SU:        suName,
					Deltas:    deltas,
				}
				rawReceipt, mErr := json.Marshal(receipt)
				if mErr != nil {
					return mErr
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, suAssignmentHistoryTable, suAssignmentHistorySQL, string(rawReceipt), "SU plot assignment"); err != nil {
					return err
				}
				_ = tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM main.`+vprocore.QuoteHeaderIdentifier(suAssignmentHistoryTable)).Scan(&res.HistoryID)
			}

			if err := validateFiles(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return tx.Commit()
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	})
}

func UndoSUAssignment(ctx context.Context, contexts *vprocore.ContextService, contextID string, historyID int64) (*AssignPlotsToSiteUnitResult, error) {
	if ctx == nil {
		return nil, errors.New("SU undo requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("classification context service is unavailable")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*AssignPlotsToSiteUnitResult, error) {
		owner := plots.Projects.Sqlite
		if owner.Selection.SU == "" || owner.Selection.SU == "None" {
			return nil, errors.New("an active Site Unit (SU) table must be selected before undoing an assignment")
		}
		suName := owner.Selection.SU
		suTable := suName + "_SU"
		qTable := vprocore.QuoteHeaderIdentifier(suTable)

		validateFiles := func() error {
			return owner.ValidateMetadataWriterFiles()
		}

		res := &AssignPlotsToSiteUnitResult{
			ContextID: contextID,
			SU:        suName,
			TableName: suTable,
		}

		err := owner.WithSelectedFileWriter(ctx, "su", validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil)
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			var targetID int64
			var proposal string
			if historyID > 0 {
				err := tx.QueryRowContext(ctx,
					`SELECT ID, Proposal FROM main.`+vprocore.QuoteHeaderIdentifier(suAssignmentHistoryTable)+` WHERE ID = ?`,
					historyID,
				).Scan(&targetID, &proposal)
				if err != nil {
					return fmt.Errorf("SU assignment history #%d not found: %w", historyID, err)
				}
			} else {
				err := tx.QueryRowContext(ctx,
					`SELECT ID, Proposal FROM main.`+vprocore.QuoteHeaderIdentifier(suAssignmentHistoryTable)+
						` WHERE Proposal LIKE '%"kind":"su_plot_assignment"%' ORDER BY ID DESC LIMIT 1`,
				).Scan(&targetID, &proposal)
				if err != nil {
					return fmt.Errorf("no reversible SU plot assignment found: %w", err)
				}
			}

			var receipt suPlotAssignmentReceipt
			if err := json.Unmarshal([]byte(proposal), &receipt); err != nil || receipt.Kind != "su_plot_assignment" || receipt.SU != suName {
				return errors.New("selected history entry is not a reversible assignment for the active SU")
			}

			reverseDeltas := make([]suPlotAssignmentDelta, 0, len(receipt.Deltas))
			for i := len(receipt.Deltas) - 1; i >= 0; i-- {
				d := receipt.Deltas[i]
				if !d.ExistedBefore {
					if _, err := tx.ExecContext(ctx,
						`DELETE FROM main.`+qTable+` WHERE "PlotNumber" = ? AND typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
						d.PlotNumber, d.PlotNumber,
					); err != nil {
						return err
					}
				} else {
					var existingRowID int64
					rErr := tx.QueryRowContext(ctx,
						`SELECT rowid FROM main.`+qTable+` WHERE "PlotNumber" = ? AND typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
						d.PlotNumber, d.PlotNumber,
					).Scan(&existingRowID)
					if errors.Is(rErr, sql.ErrNoRows) {
						if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qTable+` ("PlotNumber","SiteUnit") VALUES (?,?)`, d.PlotNumber, d.BeforeSiteUnit); err != nil {
							return err
						}
					} else if rErr != nil {
						return rErr
					} else {
						if _, err := tx.ExecContext(ctx, `UPDATE main.`+qTable+` SET "SiteUnit" = ? WHERE rowid = ?`, d.BeforeSiteUnit, existingRowID); err != nil {
							return err
						}
					}
				}
				reverseDeltas = append(reverseDeltas, suPlotAssignmentDelta{
					PlotNumber:     d.PlotNumber,
					ExistedBefore:  d.ExistedAfter,
					BeforeSiteUnit: d.AfterSiteUnit,
					ExistedAfter:   d.ExistedBefore,
					AfterSiteUnit:  d.BeforeSiteUnit,
				})
				res.ChangedPlots++
			}

			undoReceipt, err := json.Marshal(struct {
				Kind      string                  `json:"kind"`
				UndoneID  string                  `json:"undoneId"`
				SU        string                  `json:"su"`
				Deltas    []suPlotAssignmentDelta `json:"deltas"`
			}{"su_plot_assignment_undo", strconv.FormatInt(targetID, 10), suName, reverseDeltas})
			if err != nil {
				return err
			}
			if err := vprocore.AppendTechnicalProvenance(ctx, tx, suAssignmentHistoryTable, suAssignmentHistorySQL, string(undoReceipt), "SU assignment undo"); err != nil {
				return err
			}
			_ = tx.QueryRowContext(ctx, `SELECT MAX(ID) FROM main.`+vprocore.QuoteHeaderIdentifier(suAssignmentHistoryTable)).Scan(&res.HistoryID)
			return tx.Commit()
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	})
}
