package datasheet

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	DatasheetDefaultLimit = 100
	DatasheetMaxLimit     = 1000
	datasheetHistoryTable = "__VPRO_DatasheetEditHistory"
	datasheetHistorySQL   = `CREATE TABLE "__VPRO_DatasheetEditHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`
)

// Canonical 9 table keys supported by the Universal Project Datasheet Grid.
var canonicalDatasheetKeys = []string{
	"Env", "Admin", "Veg", "Humus", "Mineral", "Other", "SU", "Lump", "Hierarchy",
}

type DatasheetColumnMeta struct {
	Name          string `json:"name"`
	DeclaredType  string `json:"declaredType"`
	Kind          string `json:"kind"` // "text" | "integer" | "single" | "double" | "boolean"
	Pinned        bool   `json:"pinned"`
	Editable      bool   `json:"editable"`
	Protected     bool   `json:"protected"`
	MaxUTF16      int    `json:"maxUtf16"`
	IsCoverSingle bool   `json:"isCoverSingle"`
	IsBoolean     bool   `json:"isBoolean"`
}

type DatasheetTableInfo struct {
	Key               string                `json:"key"`
	Label             string                `json:"label"`
	Role              string                `json:"role"`
	TableName         string                `json:"tableName"`
	PinnedColumn      string                `json:"pinnedColumn"`
	Available         bool                  `json:"available"`
	UnavailableReason string                `json:"unavailableReason,omitempty"`
	Editable          bool                  `json:"editable"`
	RowCount          int64                 `json:"rowCount"`
	Columns           []DatasheetColumnMeta `json:"columns"`
}

type DatasheetSortSpec struct {
	Column    string `json:"column"`
	Direction string `json:"direction"` // "asc" | "desc"
}

type DatasheetFilterSpec struct {
	Column string `json:"column"`
	Op     string `json:"op"` // "contains" | "eq" | "starts_with" | "ends_with" | "is_null" | "not_null"
	Value  string `json:"value"`
}

type DatasheetPageRequest struct {
	TableKey        string                `json:"tableKey"`
	Offset          int                   `json:"offset"`
	Limit           int                   `json:"limit"`
	Sort            []DatasheetSortSpec   `json:"sort"`
	Filters         []DatasheetFilterSpec `json:"filters"`
	GlobalSearch    string                `json:"globalSearch"`
	ScopeToActiveSU bool                  `json:"scopeToActiveSU"`
}

type DatasheetRow struct {
	RowID string                         `json:"rowId"`
	Cells []vprocore.ProjectMetadataCell `json:"cells"`
}

type DatasheetPage struct {
	ContextID       string                `json:"contextId"`
	Project         string                `json:"project"`
	ProjectPath     string                `json:"projectPath"`
	SU              string                `json:"su"`
	Hierarchy       string                `json:"hierarchy"`
	TableKey        string                `json:"tableKey"`
	TableName       string                `json:"tableName"`
	PinnedColumn    string                `json:"pinnedColumn"`
	Columns         []DatasheetColumnMeta `json:"columns"`
	Rows            []DatasheetRow        `json:"rows"`
	Offset          int                   `json:"offset"`
	Limit           int                   `json:"limit"`
	TotalRows       int64                 `json:"totalRows"`
	CanEditMaster   bool                  `json:"canEditMaster"`
	ScopeToActiveSU bool                  `json:"scopeToActiveSU"`
}

type DatasheetCellEdit struct {
	RowID  string                       `json:"rowId"`
	Column string                       `json:"column"`
	Before vprocore.ProjectMetadataCell `json:"before"`
	After  vprocore.ProjectMetadataCell `json:"after"`
}

type DatasheetEditRequest struct {
	TableKey string              `json:"tableKey"`
	Edits    []DatasheetCellEdit `json:"edits"`
}

type DatasheetEditResult struct {
	ContextID    string `json:"contextId"`
	TableKey     string `json:"tableKey"`
	TableName    string `json:"tableName"`
	UpdatedRows  int    `json:"updatedRows"`
	UpdatedCells int    `json:"updatedCells"`
	AuditRows    int    `json:"auditRows"`
}

type DatasheetFindReplaceRequest struct {
	TableKey        string                `json:"tableKey"`
	Column          string                `json:"column"`
	FindText        string                `json:"findText"`
	ReplaceText     string                `json:"replaceText"`
	MatchCase       bool                  `json:"matchCase"`
	ExactCell       bool                  `json:"exactCell"`
	Filters         []DatasheetFilterSpec `json:"filters"`
	GlobalSearch    string                `json:"globalSearch"`
	ScopeToActiveSU bool                  `json:"scopeToActiveSU"`
	MaxPreview      int                   `json:"maxPreview"`
}

type DatasheetFindReplaceItem struct {
	RowID           string                       `json:"rowId"`
	PinnedIdentity  string                       `json:"pinnedIdentity"`
	Column          string                       `json:"column"`
	BeforeText      string                       `json:"beforeText"`
	AfterText       string                       `json:"afterText"`
	Edit            DatasheetCellEdit            `json:"edit"`
	ValidationError string                       `json:"validationError,omitempty"`
	BeforeCell      vprocore.ProjectMetadataCell `json:"beforeCell"`
	AfterCell       vprocore.ProjectMetadataCell `json:"afterCell"`
}

type DatasheetFindReplacePreview struct {
	ContextID       string                     `json:"contextId"`
	TableKey        string                     `json:"tableKey"`
	TableName       string                     `json:"tableName"`
	Column          string                     `json:"column"`
	MatchedRows     int                        `json:"matchedRows"`
	ValidCount      int                        `json:"validCount"`
	InvalidCount    int                        `json:"invalidCount"`
	Items           []DatasheetFindReplaceItem `json:"items"`
	Truncated       bool                       `json:"truncated"`
}

type resolvedDatasheetTarget struct {
	key               string
	label             string
	role              string
	tableName         string
	pinnedColumn      string
	available         bool
	unavailableReason string
}

func resolveDatasheetTargets(owner *vprocore.SqliteContext) []resolvedDatasheetTarget {
	p := owner.Selection.Project
	suAvail := owner.Selection.SU != "None" && owner.Selection.SU != ""
	suTable := ""
	suReason := ""
	if suAvail {
		suTable = owner.Selection.SU + "_SU"
	} else {
		suReason = "No active Site Unit (SU) table is selected in the current context"
	}
	hierAvail := owner.Selection.Hierarchy != "None" && owner.Selection.Hierarchy != ""
	hierTable := ""
	hierReason := ""
	if hierAvail {
		hierTable = owner.Selection.Hierarchy + "_Hierarchy"
	} else {
		hierReason = "No active Hierarchy table is selected in the current context"
	}
	return []resolvedDatasheetTarget{
		{key: "Env", label: p + "_Env (Site & Soil Header)", role: "project", tableName: p + "_Env", pinnedColumn: "PlotNumber", available: true},
		{key: "Admin", label: p + "_Admin (Admin & QA)", role: "project", tableName: p + "_Admin", pinnedColumn: "Plot", available: true},
		{key: "Veg", label: p + "_Veg (Vegetation Strata)", role: "project", tableName: p + "_Veg", pinnedColumn: "PlotNumber", available: true},
		{key: "Humus", label: p + "_Humus (Humus Horizons)", role: "project", tableName: p + "_Humus", pinnedColumn: "PlotNumber", available: true},
		{key: "Mineral", label: p + "_Mineral (Mineral Horizons)", role: "project", tableName: p + "_Mineral", pinnedColumn: "PlotNumber", available: true},
		{key: "Other", label: p + "_Other (Other Attributes)", role: "project", tableName: p + "_Other", pinnedColumn: "PlotNumber", available: true},
		{key: "SU", label: suLabel(owner.Selection.SU), role: "su", tableName: suTable, pinnedColumn: "PlotNumber", available: suAvail, unavailableReason: suReason},
		{key: "Lump", label: p + "_Lump (Combine Species)", role: "project", tableName: p + "_Lump", pinnedColumn: "SppCode", available: true},
		{key: "Hierarchy", label: hierLabel(owner.Selection.Hierarchy), role: "hierarchy", tableName: hierTable, pinnedColumn: "Name", available: hierAvail, unavailableReason: hierReason},
	}
}

func suLabel(su string) string {
	if su == "" || su == "None" {
		return "Active _SU (None Selected)"
	}
	return su + "_SU (Site Units)"
}

func hierLabel(h string) string {
	if h == "" || h == "None" {
		return "Active _Hierarchy (None Selected)"
	}
	return h + "_Hierarchy (Classification Hierarchy)"
}

func findDatasheetTarget(owner *vprocore.SqliteContext, tableKey string) (resolvedDatasheetTarget, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(tableKey), "_")
	for _, target := range resolveDatasheetTargets(owner) {
		if strings.EqualFold(target.key, trimmed) {
			return target, nil
		}
	}
	return resolvedDatasheetTarget{}, fmt.Errorf("unsupported datasheet table key %q; expected one of %s", tableKey, strings.Join(canonicalDatasheetKeys, ", "))
}

// Known Access VP08 UTF-16 text field widths across all 9 canonical tables.
var datasheetUTF16Limits = map[string]map[string]int{
	"Env": {
		"PlotNumber": 7, "FieldNumber": 50, "ProjectID": 30, "FSRegionDistrict": 7,
		"SiteSurveyor": 30, "PlotRepresenting": 255, "Location": 255, "Ecosection": 3, "NtsMapSheet": 8,
		"UTMZone": 2, "AirPhotoNum": 20, "Zone": 4, "SubZone": 8, "SiteSeries": 5,
		"SiteModifier1": 5, "SiteModifier2": 5, "TransDistrib": 3, "RealmClass": 5, "MapUnit": 15,
		"SnowCoverregime": 1, "MoistureRegime": 3, "NutrientRegime": 2, "SuccessionalStatus": 3,
		"StructuralStage": 6, "StructuralStageMod": 1, "MesoSlopePosition": 3, "SurfaceShape": 3,
		"SurfaceTopographyType": 3, "SurfaceTopographySize": 2, "WaterSource": 5, "Photo": 50,
		"Exposure1": 2, "Exposure2": 2, "SiteDisturbance1": 8, "SiteDisturbance2": 8, "SiteDisturbance3": 8,
		"SoilSurveyor": 30, "BedrockGeology1": 4, "BedrockGeology2": 4, "BedrockGeology3": 4,
		"CoarseFragLith1": 12, "CoarseFragLith2": 12, "CoarseFragLith3": 12,
		"TerrainTextureSurf": 3, "SurficialMaterialSurf": 6, "SurfaceExpSurf": 3, "GeoMorProSurf": 3,
		"TerrainTextureSubSurf": 3, "SurficialMaterialSubSurf": 6, "SurfaceExpSubSurf": 3, "GeoMorProSubSurf": 3,
		"FloodingRegimeFreq": 7, "MoistureRegimeSub": 3, "FloodingRegimeDur": 2, "SoilDrainage": 5,
		"RootRestrictingType": 1, "RootZoneParticleSize": 6, "SoilClassSubGroup": 4, "SoilClassGroup": 4,
		"HumusForm": 4, "HumusFormPhase": 50, "pHMethodCodeMineral": 3, "pHMethodCodeOrganic": 3,
		"HydroGeoSystem": 3, "HydroGeoSubSystem": 2, "VegSurveyor": 30, "Temporary": 100,
		"SV_PolygonNumber": 25, "SV_StandAgeEstMeas": 2, "SV_StandHeightEstMeas": 2,
		"SV_CanopyComposition": 50, "SV_RootZoneTexture": 100, "SV_FullCruiseCard": 50, "SV_AhorizonType": 5,
	},
	"Admin": {
		"Plot": 7, "PlotType": 10, "ProvinceStateTerritory": 255,
		"SitePlotQuality": 15, "VegPlotQuality": 15, "SoilPlotQuality": 15,
		"EnteredBy": 50, "UserSiteUnit": 100, "BECSiteUnit": 100,
		"SiteUnitShortName": 50, "SiteUnitLongName": 100, "GIS_BGC": 255, "BEC_Use": 255,
	},
	"Veg": {
		"PlotNumber": 7, "Species": 8, "Layer": 2, "HeightB": 255, "Collected": 1,
	},
	"Humus": {
		"PlotNumber": 7, "Horizon": 8, "HumusStructureDegree": 1, "HumusStructureKind": 5,
		"MycelAbundance": 1, "FecalAbundance": 1, "RootsAbundance": 6, "RootsSize": 6,
		"Consistence": 2, "Character": 2, "Fauna": 4,
	},
	"Mineral": {
		"PlotNumber": 7, "Horizon": 8, "PitDepthLimit": 1, "Colour": 14, "Texture": 4,
		"PercentCoarseFragsShape": 1, "RootsAbundance": 6, "RootsSize": 6,
		"MineralStructureClass": 3, "MineralStructureKind": 7,
		"MottlesAbundance": 1, "MottlesSize": 1, "MottlesContrast": 1,
		"ClayFilmsFreq": 2, "ClayFilmThickness": 3, "Effervescence": 2, "Porosity": 1,
	},
	"Other": {
		"PlotNumber": 7, "DataName": 50, "DataItem": 255, "UserItem1": 255, "UserItem2": 255, "UserItem3": 255,
	},
	"SU": {
		"PlotNumber": 7, "SiteUnit": 255,
	},
	"Lump": {
		"LumpCode": 50, "SppCode": 8,
	},
	"Hierarchy": {
		"Name": 255, "Tag": 255, "MyOrder": 255,
	},
}

// Cover columns subject to IEEE-754 SINGLE (float32) narrowing before `< 100 Or Is Null`.
var datasheetCoverSingleColumns = map[string]map[string]bool{
	"Env": {
		"StrataCoverTree": true, "StrataCoverShrub": true, "StrataCoverHerb": true, "StrataCoverMoss": true,
	},
	"Admin": {
		"StrataCoverTotal": true,
	},
	"Veg": {
		"Cover1": true, "Cover2": true, "Cover3": true, "TotalA": true,
		"Cover4": true, "Cover5": true, "Cover5a": true, "Cover5b": true, "Cover5c": true, "TotalB": true,
		"Cover6": true, "Cover7": true, "Cover8": true, "Cover9": true, "Cover10": true,
	},
}

// Access BOOLEAN columns stored as -1 (true) / 0 (false).
var datasheetBooleanColumns = map[string]map[string]bool{
	"Env":       {"SpeciesListComplete": true, "Flag": true, "SV_FloodPlain": true},
	"Admin":     {"UpdatedFromCards": true},
	"Veg":       {"Flag": true},
	"Humus":     {"Flag": true},
	"Mineral":   {"Flag": true},
	"Other":     {"UserFlag1": true, "UserFlag2": true, "UserFlag3": true, "Flag": true},
	"Lump":      {"Use": true},
	"Hierarchy": {"Flag": true},
}

// Double-precision columns (not narrowed to float32).
var datasheetDoubleColumns = map[string]map[string]bool{
	"Env": {"Latitude": true, "Longitude": true},
}

// 32-bit integer columns (default integer columns are INT16 unless listed here or declared INTEGER).
var datasheetInt32Columns = map[string]map[string]bool{
	"Veg":       {"ID": true, "LL": true, "PV": true},
	"Humus":     {"ID": true},
	"Mineral":   {"ID": true},
	"Other":     {"ID": true},
	"Hierarchy": {"ID": true, "Parent": true},
}

func classifyColumnMeta(tableKey string, col vprocore.ProjectMetadataColumn, pinnedColumn string, canEditMaster bool) DatasheetColumnMeta {
	upperType := strings.ToUpper(col.DeclaredType)
	isBool := datasheetBooleanColumns[tableKey][col.Name] || strings.Contains(upperType, "BOOL")
	isCover := datasheetCoverSingleColumns[tableKey][col.Name]
	isDouble := datasheetDoubleColumns[tableKey][col.Name] || strings.Contains(upperType, "DOUBLE")
	maxUTF16 := datasheetUTF16Limits[tableKey][col.Name]

	kind := "text"
	switch {
	case isBool:
		kind = "boolean"
	case isCover:
		kind = "single"
	case isDouble:
		kind = "double"
	case strings.Contains(upperType, "REAL") || strings.Contains(upperType, "FLOA") || strings.Contains(upperType, "SING"):
		kind = "single"
	case strings.Contains(upperType, "INT"):
		kind = "integer"
	}

	editable := true
	protected := false
	// Child row surrogate IDs, Hierarchy auto-IDs, and relational PlotNumber/Plot/SppCode keys are immutable in single-table grid mode.
	if col.Name == "ID" && (tableKey == "Veg" || tableKey == "Humus" || tableKey == "Mineral" || tableKey == "Other" || tableKey == "Hierarchy") {
		editable = false
		protected = true
	}
	if (col.Name == "PlotNumber" && (tableKey == "Env" || tableKey == "Veg" || tableKey == "Humus" || tableKey == "Mineral" || tableKey == "Other" || tableKey == "SU")) ||
		(col.Name == "Plot" && tableKey == "Admin") ||
		(col.Name == "SppCode" && tableKey == "Lump") {
		editable = false
		protected = true
	}
	// Calculated Admin fields and custodian-locked BECSiteUnit protection.
	if tableKey == "Admin" && (col.Name == "HumusThickness" || col.Name == "StrataCoverTotal") {
		protected = true
	}
	if tableKey == "Admin" && col.Name == "BECSiteUnit" {
		protected = true
		if !canEditMaster {
			editable = false
		}
	}

	return DatasheetColumnMeta{
		Name:          col.Name,
		DeclaredType:  col.DeclaredType,
		Kind:          kind,
		Pinned:        strings.EqualFold(col.Name, pinnedColumn),
		Editable:      editable,
		Protected:     protected,
		MaxUTF16:      maxUTF16,
		IsCoverSingle: isCover,
		IsBoolean:     isBool,
	}
}

func tableExistsInSnapshot(ctx context.Context, tx *sql.Tx, role, table string) (bool, error) {
	if role == "" || table == "" {
		return false, nil
	}
	var exists bool
	err := tx.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM "+vprocore.QuoteHeaderIdentifier(role)+".sqlite_master WHERE type='table' AND name COLLATE BINARY=?)",
		table,
	).Scan(&exists)
	return exists, err
}

func ListDatasheetTables(ctx context.Context, contexts *vprocore.ContextService, contextID string) ([]DatasheetTableInfo, error) {
	if ctx == nil {
		return nil, errors.New("datasheet table listing requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("datasheet context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ []DatasheetTableInfo, resultErr error) {
		owner := plots.Projects.Sqlite
		canEditMaster := plots.CanEditMasterBEC()
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
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
		}()

		targets := resolveDatasheetTargets(owner)
		out := make([]DatasheetTableInfo, 0, len(targets))
		for _, target := range targets {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			info := DatasheetTableInfo{
				Key:               target.key,
				Label:             target.label,
				Role:              target.role,
				TableName:         target.tableName,
				PinnedColumn:      target.pinnedColumn,
				Available:         target.available,
				UnavailableReason: target.unavailableReason,
				Editable:          target.available,
				Columns:           []DatasheetColumnMeta{},
			}
			if !target.available {
				out = append(out, info)
				continue
			}
			exists, err := tableExistsInSnapshot(ctx, tx, target.role, target.tableName)
			if err != nil {
				return nil, err
			}
			if !exists {
				info.Available = false
				info.Editable = false
				info.UnavailableReason = fmt.Sprintf("Table %s does not exist in %s database", target.tableName, target.role)
				out = append(out, info)
				continue
			}
			rawCols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, target.role, target.tableName)
			if err != nil {
				return nil, err
			}
			for _, col := range rawCols {
				info.Columns = append(info.Columns, classifyColumnMeta(target.key, col, target.pinnedColumn, canEditMaster))
			}
			relation := vprocore.QuoteHeaderIdentifier(target.role) + "." + vprocore.QuoteHeaderIdentifier(target.tableName)
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+relation).Scan(&info.RowCount); err != nil {
				return nil, err
			}
			out = append(out, info)
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
		return out, nil
	})
}

func escapeSQLiteLike(raw string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(raw)
}

func isDatasheetTextOrNoneAffinity(declType string) bool {
	u := strings.ToUpper(strings.TrimSpace(declType))
	if u == "" {
		return true
	}
	if strings.Contains(u, "INT") {
		return false
	}
	return strings.Contains(u, "CHAR") || strings.Contains(u, "CLOB") || strings.Contains(u, "TEXT") || strings.Contains(u, "BLOB")
}

func canMatchFormattedNumeric(needle string) bool {
	for _, r := range needle {
		switch {
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '+' || r == 'e' || r == 'E':
		default:
			return false
		}
	}
	return needle != ""
}

func canMatchFormattedBoolean(needle string) bool {
	if canMatchFormattedNumeric(needle) {
		return true
	}
	lower := strings.ToLower(needle)
	return strings.Contains("true", lower) || strings.Contains("false", lower)
}

func buildDatasheetWhereAndOrder(
	ctx context.Context,
	tx *sql.Tx,
	target resolvedDatasheetTarget,
	columns []DatasheetColumnMeta,
	filters []DatasheetFilterSpec,
	globalSearch string,
	scopeToActiveSU bool,
	owner *vprocore.SqliteContext,
	sortSpecs []DatasheetSortSpec,
) (string, []any, string, error) {
	colLookup := make(map[string]DatasheetColumnMeta, len(columns))
	for _, c := range columns {
		colLookup[strings.ToLower(c.Name)] = c
	}

	var clauses []string
	var args []any

	if scopeToActiveSU && owner.Selection.SU != "None" && owner.Selection.SU != "" {
		var plotCol string
		switch target.key {
		case "Env", "Veg", "Humus", "Mineral", "Other":
			plotCol = "PlotNumber"
		case "Admin":
			plotCol = "Plot"
		}
		if plotCol != "" {
			suTableName := owner.Selection.SU + "_SU"
			suRel := `"su".` + vprocore.QuoteHeaderIdentifier(suTableName)
			qPlot := `__t.` + vprocore.QuoteHeaderIdentifier(plotCol)
			outerMeta := colLookup[strings.ToLower(plotCol)]

			suCols, suColErr := vprocore.ReadSQLiteStorageColumns(ctx, tx, "su", suTableName)
			if suColErr != nil {
				return "", nil, "", suColErr
			}
			suPlotDecl := ""
			for _, sc := range suCols {
				if strings.EqualFold(sc.Name, "PlotNumber") {
					suPlotDecl = sc.DeclaredType
					break
				}
			}
			suHasTextAffinity := isDatasheetTextOrNoneAffinity(suPlotDecl)

			// Probe up to 901 distinct typeof='text' SU plot numbers for fast IN pushdown on large child tables.
			var suPlots []string
			allValidUTF8 := true
			suRows, suErr := tx.QueryContext(ctx, `SELECT DISTINCT CAST("PlotNumber" AS BLOB) FROM `+suRel+` WHERE typeof("PlotNumber")='text' LIMIT 901`)
			if suErr != nil {
				return "", nil, "", suErr
			}
			for suRows.Next() {
				var raw []byte
				if err := suRows.Scan(&raw); err != nil {
					suRows.Close()
					return "", nil, "", err
				}
				s := string(raw)
				if !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
					allValidUTF8 = false
				}
				suPlots = append(suPlots, s)
			}
			if err := errors.Join(suRows.Err(), suRows.Close()); err != nil {
				return "", nil, "", err
			}

			if len(suPlots) == 0 {
				clauses = append(clauses, "1=0")
			} else {
				idxEq := ""
				if suHasTextAffinity {
					idxEq = `__su."PlotNumber" COLLATE BINARY = ` + qPlot + ` COLLATE BINARY AND `
				}
				existsSQL := `EXISTS(SELECT 1 FROM ` + suRel + ` AS __su WHERE typeof(__su."PlotNumber")='text' AND typeof(` + qPlot + `)='text' AND ` + idxEq + `CAST(__su."PlotNumber" AS BLOB)=CAST(` + qPlot + ` AS BLOB))`
				if len(suPlots) <= 900 && allValidUTF8 && isDatasheetTextOrNoneAffinity(outerMeta.DeclaredType) {
					ph := make([]string, len(suPlots))
					for i, p := range suPlots {
						ph[i] = "?"
						args = append(args, p)
					}
					clauses = append(clauses, "("+qPlot+" IN ("+strings.Join(ph, ",")+") AND "+existsSQL+")")
				} else {
					clauses = append(clauses, existsSQL)
				}
			}
		}
	}

	for _, f := range filters {
		colMeta, ok := colLookup[strings.ToLower(strings.TrimSpace(f.Column))]
		if !ok {
			return "", nil, "", fmt.Errorf("unknown filter column %q on %s", f.Column, target.tableName)
		}
		if !utf8.ValidString(f.Value) || strings.ContainsRune(f.Value, 0) {
			return "", nil, "", fmt.Errorf("filter value for %q must be valid UTF-8 without NUL", colMeta.Name)
		}
		qCol := `__t.` + vprocore.QuoteHeaderIdentifier(colMeta.Name)
		op := strings.ToLower(strings.TrimSpace(f.Op))
		if op == "" {
			op = "contains"
		}
		switch op {
		case "is_null":
			clauses = append(clauses, "("+qCol+" IS NULL OR CAST("+qCol+" AS TEXT) = '')")
		case "not_null":
			clauses = append(clauses, "("+qCol+" IS NOT NULL AND CAST("+qCol+" AS TEXT) <> '')")
		case "eq":
			if f.Value == "" {
				continue
			}
			if colMeta.Name == "PlotNumber" || colMeta.Name == "Plot" {
				if isDatasheetTextOrNoneAffinity(colMeta.DeclaredType) {
					clauses = append(clauses, "("+qCol+" = ? AND typeof("+qCol+")='text' AND CAST("+qCol+" AS BLOB)=CAST(? AS BLOB))")
					args = append(args, f.Value, f.Value)
				} else {
					clauses = append(clauses, "(typeof("+qCol+")='text' AND CAST("+qCol+" AS BLOB)=CAST(? AS BLOB))")
					args = append(args, f.Value)
				}
			} else if colMeta.IsBoolean {
				bVal, err := parseFlexibleBoolean(f.Value)
				if err != nil {
					clauses = append(clauses, "CAST("+qCol+" AS TEXT) = ?")
					args = append(args, f.Value)
				} else if bVal {
					clauses = append(clauses, "CAST("+qCol+" AS INTEGER) <> 0")
				} else {
					clauses = append(clauses, "CAST("+qCol+" AS INTEGER) = 0")
				}
			} else {
				clauses = append(clauses, "CAST("+qCol+" AS TEXT) = ?")
				args = append(args, f.Value)
			}
		case "starts_with":
			if f.Value == "" {
				continue
			}
			clauses = append(clauses, "CAST("+qCol+" AS TEXT) LIKE ? ESCAPE '\\'")
			args = append(args, escapeSQLiteLike(f.Value)+"%")
		case "ends_with":
			if f.Value == "" {
				continue
			}
			clauses = append(clauses, "CAST("+qCol+" AS TEXT) LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeSQLiteLike(f.Value))
		case "contains":
			if f.Value == "" {
				continue
			}
			clauses = append(clauses, "CAST("+qCol+" AS TEXT) LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeSQLiteLike(f.Value)+"%")
		default:
			return "", nil, "", fmt.Errorf("unsupported filter operator %q", f.Op)
		}
	}

	trimmedGlobal := strings.TrimSpace(globalSearch)
	if trimmedGlobal != "" {
		if !utf8.ValidString(trimmedGlobal) || strings.ContainsRune(trimmedGlobal, 0) {
			return "", nil, "", errors.New("global search must be valid UTF-8 without NUL")
		}
		likeArg := "%" + escapeSQLiteLike(trimmedGlobal) + "%"
		matchNum := canMatchFormattedNumeric(trimmedGlobal)
		matchBool := canMatchFormattedBoolean(trimmedGlobal)
		var orParts []string
		for _, colMeta := range columns {
			qCol := `__t.` + vprocore.QuoteHeaderIdentifier(colMeta.Name)
			if colMeta.Kind == "text" {
				orParts = append(orParts, qCol+" LIKE ? ESCAPE '\\'")
				args = append(args, likeArg)
				continue
			}
			if colMeta.Kind == "boolean" {
				if !matchBool {
					continue
				}
			} else if !matchNum {
				continue
			}
			orParts = append(orParts, "CAST("+qCol+" AS TEXT) LIKE ? ESCAPE '\\'")
			args = append(args, likeArg)
		}
		if len(orParts) > 0 {
			clauses = append(clauses, "("+strings.Join(orParts, " OR ")+")")
		} else {
			clauses = append(clauses, "1=0")
		}
	}

	whereSQL := ""
	if len(clauses) > 0 {
		whereSQL = " WHERE " + strings.Join(clauses, " AND ")
	}

	var orderParts []string
	seenSort := map[string]bool{}
	for _, s := range sortSpecs {
		colMeta, ok := colLookup[strings.ToLower(strings.TrimSpace(s.Column))]
		if !ok {
			return "", nil, "", fmt.Errorf("unknown sort column %q on %s", s.Column, target.tableName)
		}
		lower := strings.ToLower(colMeta.Name)
		if seenSort[lower] {
			continue
		}
		seenSort[lower] = true
		dir := "ASC"
		if strings.EqualFold(strings.TrimSpace(s.Direction), "desc") {
			dir = "DESC"
		}
		qCol := `__t.` + vprocore.QuoteHeaderIdentifier(colMeta.Name)
		// NULLs always sort last for predictable spreadsheet ergonomics, with case-insensitive text ordering.
		if colMeta.Kind == "text" {
			orderParts = append(orderParts, "CASE WHEN "+qCol+" IS NULL THEN 1 ELSE 0 END ASC", qCol+" COLLATE NOCASE "+dir, qCol+" COLLATE BINARY "+dir)
		} else {
			orderParts = append(orderParts, "CASE WHEN "+qCol+" IS NULL THEN 1 ELSE 0 END ASC", qCol+" "+dir)
		}
	}
	if len(orderParts) == 0 {
		if pinned, ok := colLookup[strings.ToLower(target.pinnedColumn)]; ok {
			qPin := `__t.` + vprocore.QuoteHeaderIdentifier(pinned.Name)
			if target.key == "Env" || target.key == "Veg" {
				orderParts = append(orderParts, qPin+" COLLATE NOCASE ASC", qPin+" COLLATE BINARY ASC")
			} else {
				orderParts = append(orderParts, "CASE WHEN "+qPin+" IS NULL THEN 1 ELSE 0 END ASC", qPin+" COLLATE NOCASE ASC", qPin+" COLLATE BINARY ASC")
			}
		}
		if target.key == "Humus" {
			if _, ok := colLookup["upperdepth"]; ok {
				orderParts = append(orderParts, `CASE WHEN __t."UpperDepth" IS NULL THEN 1 ELSE 0 END ASC`, `__t."UpperDepth" DESC`)
			}
		} else if target.key == "Mineral" {
			if _, ok := colLookup["upperdepth"]; ok {
				orderParts = append(orderParts, `CASE WHEN __t."UpperDepth" IS NULL THEN 1 ELSE 0 END ASC`, `__t."UpperDepth" ASC`)
			}
		}
	}
	orderParts = append(orderParts, "__t.rowid ASC")
	orderSQL := " ORDER BY " + strings.Join(orderParts, ", ")

	return whereSQL, args, orderSQL, nil
}

func queryDatasheetWindowRows(
	ctx context.Context,
	tx *sql.Tx,
	role, tableName string,
	columns []DatasheetColumnMeta,
	whereSQL string,
	whereArgs []any,
	orderSQL string,
	offset, limit int,
) ([]DatasheetRow, error) {
	relation := vprocore.QuoteHeaderIdentifier(role) + "." + vprocore.QuoteHeaderIdentifier(tableName)

	selectFields := make([]string, 0, 1+len(columns)*2)
	selectFields = append(selectFields, "__t.rowid")
	for _, col := range columns {
		qCol := `__t.` + vprocore.QuoteHeaderIdentifier(col.Name)
		selectFields = append(selectFields,
			"typeof("+qCol+")",
			"CASE typeof("+qCol+") WHEN 'integer' THEN CAST("+qCol+" AS INTEGER) WHEN 'real' THEN CAST("+qCol+" AS REAL) ELSE CAST("+qCol+" AS BLOB) END",
		)
	}

	if limit <= 0 || limit > 900 {
		query := "SELECT " + strings.Join(selectFields, ",") + " FROM " + relation + " AS __t" + whereSQL + orderSQL
		args := append([]any{}, whereArgs...)
		if limit > 0 {
			query += " LIMIT ? OFFSET ?"
			args = append(args, limit, offset)
		}
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := []DatasheetRow{}
		storages := make([]string, len(columns))
		values := make([]any, len(columns))
		var rowID int64
		dest := make([]any, 0, 1+len(columns)*2)
		dest = append(dest, &rowID)
		for i := range columns {
			dest = append(dest, &storages[i], &values[i])
		}
		for rows.Next() {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			for i := range values {
				values[i] = nil
			}
			if err := rows.Scan(dest...); err != nil {
				return nil, err
			}
			cells := make([]vprocore.ProjectMetadataCell, len(columns))
			for i, val := range values {
				cell, err := vprocore.ProjectMetadataCellInternal(storages[i], val)
				if err != nil {
					return nil, fmt.Errorf("row %d column %s: %w", rowID, columns[i].Name, err)
				}
				cells[i] = cell
			}
			result = append(result, DatasheetRow{
				RowID: strconv.FormatInt(rowID, 10),
				Cells: cells,
			})
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Stage 1: fetch only the bounded window of rowids so SQLite's top-N sorter never materializes 88 cell expressions for non-window rows.
	idQuery := "SELECT __t.rowid FROM " + relation + " AS __t" + whereSQL + orderSQL + " LIMIT ? OFFSET ?"
	idArgs := append(append([]any{}, whereArgs...), limit, offset)
	idRows, err := tx.QueryContext(ctx, idQuery, idArgs...)
	if err != nil {
		return nil, err
	}
	var winRowIDs []int64
	for idRows.Next() {
		var rid int64
		if err := idRows.Scan(&rid); err != nil {
			idRows.Close()
			return nil, err
		}
		winRowIDs = append(winRowIDs, rid)
	}
	if err := errors.Join(idRows.Err(), idRows.Close()); err != nil {
		return nil, err
	}
	if len(winRowIDs) == 0 {
		return []DatasheetRow{}, nil
	}

	// Stage 2: fetch full typed cell storage only for the winning window rowids and preserve window order.
	ph := make([]string, len(winRowIDs))
	fetchArgs := make([]any, len(winRowIDs))
	posByRowID := make(map[int64]int, len(winRowIDs))
	for i, rid := range winRowIDs {
		ph[i] = "?"
		fetchArgs[i] = rid
		posByRowID[rid] = i
	}
	fetchQuery := "SELECT " + strings.Join(selectFields, ",") + " FROM " + relation + " AS __t WHERE __t.rowid IN (" + strings.Join(ph, ",") + ")"
	rows, err := tx.QueryContext(ctx, fetchQuery, fetchArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]DatasheetRow, len(winRowIDs))
	storages := make([]string, len(columns))
	values := make([]any, len(columns))
	var rowID int64
	dest := make([]any, 0, 1+len(columns)*2)
	dest = append(dest, &rowID)
	for i := range columns {
		dest = append(dest, &storages[i], &values[i])
	}

	seen := 0
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for i := range values {
			values[i] = nil
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		cells := make([]vprocore.ProjectMetadataCell, len(columns))
		for i, val := range values {
			cell, err := vprocore.ProjectMetadataCellInternal(storages[i], val)
			if err != nil {
				return nil, fmt.Errorf("row %d column %s: %w", rowID, columns[i].Name, err)
			}
			cells[i] = cell
		}
		if idx, ok := posByRowID[rowID]; ok {
			result[idx] = DatasheetRow{
				RowID: strconv.FormatInt(rowID, 10),
				Cells: cells,
			}
			seen++
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	if seen != len(winRowIDs) {
		filtered := make([]DatasheetRow, 0, seen)
		for _, r := range result {
			if r.RowID != "" {
				filtered = append(filtered, r)
			}
		}
		return filtered, nil
	}
	return result, nil
}

func GetDatasheetPage(ctx context.Context, contexts *vprocore.ContextService, contextID string, request DatasheetPageRequest) (*DatasheetPage, error) {
	if ctx == nil {
		return nil, errors.New("datasheet page read requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("datasheet context service is unavailable")
	}
	if request.Offset < 0 {
		return nil, errors.New("datasheet offset must be non-negative")
	}
	limit := request.Limit
	if limit == 0 {
		limit = DatasheetDefaultLimit
	}
	if limit < 1 || limit > DatasheetMaxLimit {
		return nil, fmt.Errorf("datasheet limit must be between 1 and %d", DatasheetMaxLimit)
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *DatasheetPage, resultErr error) {
		owner := plots.Projects.Sqlite
		canEditMaster := plots.CanEditMasterBEC()
		target, err := findDatasheetTarget(owner, request.TableKey)
		if err != nil {
			return nil, err
		}
		if !target.available {
			return nil, errors.New(target.unavailableReason)
		}

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
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
		}()

		exists, err := tableExistsInSnapshot(ctx, tx, target.role, target.tableName)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("table %s does not exist in %s database", target.tableName, target.role)
		}
		rawCols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, target.role, target.tableName)
		if err != nil {
			return nil, err
		}
		columns := make([]DatasheetColumnMeta, len(rawCols))
		for i, col := range rawCols {
			columns[i] = classifyColumnMeta(target.key, col, target.pinnedColumn, canEditMaster)
		}

		whereSQL, whereArgs, orderSQL, err := buildDatasheetWhereAndOrder(
			ctx, tx, target, columns, request.Filters, request.GlobalSearch, request.ScopeToActiveSU, owner, request.Sort,
		)
		if err != nil {
			return nil, err
		}

		relation := vprocore.QuoteHeaderIdentifier(target.role) + "." + vprocore.QuoteHeaderIdentifier(target.tableName)
		var rows []DatasheetRow
		var totalRows int64
		if request.Offset == 0 {
			rows, err = queryDatasheetWindowRows(ctx, tx, target.role, target.tableName, columns, whereSQL, whereArgs, orderSQL, 0, limit)
			if err != nil {
				return nil, err
			}
			if len(rows) < limit {
				totalRows = int64(len(rows))
			} else {
				if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+relation+" AS __t"+whereSQL, whereArgs...).Scan(&totalRows); err != nil {
					return nil, err
				}
			}
		} else {
			if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+relation+" AS __t"+whereSQL, whereArgs...).Scan(&totalRows); err != nil {
				return nil, err
			}
			if int64(request.Offset) < totalRows {
				rows, err = queryDatasheetWindowRows(ctx, tx, target.role, target.tableName, columns, whereSQL, whereArgs, orderSQL, request.Offset, limit)
				if err != nil {
					return nil, err
				}
			} else {
				rows = []DatasheetRow{}
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

		return &DatasheetPage{
			ContextID:       contextID,
			Project:         owner.Selection.Project,
			ProjectPath:     owner.Selection.ProjectPath,
			SU:              owner.Selection.SU,
			Hierarchy:       owner.Selection.Hierarchy,
			TableKey:        target.key,
			TableName:       target.tableName,
			PinnedColumn:    target.pinnedColumn,
			Columns:         columns,
			Rows:            rows,
			Offset:          request.Offset,
			Limit:           limit,
			TotalRows:       totalRows,
			CanEditMaster:   canEditMaster,
			ScopeToActiveSU: request.ScopeToActiveSU,
		}, nil
	})
}

func parseFlexibleBoolean(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "-1", "1", "yes", "y":
		return true, nil
	case "false", "0", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("invalid Access boolean value %q", raw)
	}
}

func cellToDisplayString(cell vprocore.ProjectMetadataCell, isBool bool) string {
	switch cell.Storage {
	case "null":
		return ""
	case "text":
		if cell.Text != nil {
			return *cell.Text
		}
	case "integer":
		if cell.Integer != nil {
			if isBool {
				if *cell.Integer == "0" {
					return "false"
				}
				return "true"
			}
			return *cell.Integer
		}
	case "real":
		if cell.Real != nil {
			return vprocore.SourceDoubleDecimalString(*cell.Real)
		}
	case "blob":
		if cell.BlobHex != nil {
			return *cell.BlobHex
		}
	}
	return ""
}

func buildReplacedCellFromString(colMeta DatasheetColumnMeta, replaced string) (vprocore.ProjectMetadataCell, error) {
	if replaced == "" {
		return vprocore.ProjectMetadataCell{Storage: "null"}, nil
	}
	switch colMeta.Kind {
	case "boolean":
		b, err := parseFlexibleBoolean(replaced)
		if err != nil {
			return vprocore.ProjectMetadataCell{}, err
		}
		stored := "0"
		if b {
			stored = "-1"
		}
		return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &stored}, nil
	case "integer":
		n, err := strconv.ParseInt(strings.TrimSpace(replaced), 10, 64)
		if err != nil {
			return vprocore.ProjectMetadataCell{}, fmt.Errorf("%s requires an integer value, got %q", colMeta.Name, replaced)
		}
		stored := strconv.FormatInt(n, 10)
		return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &stored}, nil
	case "single", "double":
		f, err := strconv.ParseFloat(strings.TrimSpace(replaced), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return vprocore.ProjectMetadataCell{}, fmt.Errorf("%s requires a finite number, got %q", colMeta.Name, replaced)
		}
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &f}, nil
	default:
		val := replaced
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &val}, nil
	}
}

// ValidateAndNormalizeDatasheetCell validates a proposed cell value against VP08 rules
// (UTF-16 bounds, IEEE-754 float32 <100 cover narrowing, Access BOOLEAN -1/0, integer ranges)
// and returns the SQLite driver storage value plus canonical audit string.
func validateAndNormalizeDatasheetCell(
	tableKey string,
	colMeta DatasheetColumnMeta,
	after vprocore.ProjectMetadataCell,
	canEditMaster bool,
) (any, *string, vprocore.ProjectMetadataCell, error) {
	if tableKey == "Admin" && colMeta.Name == "BECSiteUnit" && !canEditMaster {
		return nil, nil, vprocore.ProjectMetadataCell{}, errors.New("BEC Master editing is restricted to the source-authorized user; use UserSiteUnit instead")
	}
	if !colMeta.Editable {
		return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("column %s.%s is read-only in datasheet mode", tableKey, colMeta.Name)
	}

	rawVal, err := vprocore.MetadataCellValue(after)
	if err != nil {
		return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s: %w", tableKey, colMeta.Name, err)
	}
	if rawVal == nil {
		// Required non-NULL columns check
		if (tableKey == "Env" && colMeta.Name == "PlotNumber") ||
			(tableKey == "Admin" && colMeta.Name == "Plot") ||
			(tableKey == "Veg" && (colMeta.Name == "PlotNumber" || colMeta.Name == "Species")) ||
			(tableKey == "Humus" && colMeta.Name == "PlotNumber") ||
			(tableKey == "Mineral" && colMeta.Name == "PlotNumber") ||
			(tableKey == "Other" && colMeta.Name == "PlotNumber") ||
			(tableKey == "Hierarchy" && colMeta.Name == "Name") {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s cannot be NULL", tableKey, colMeta.Name)
		}
		return nil, nil, vprocore.ProjectMetadataCell{Storage: "null"}, nil
	}

	if colMeta.IsBoolean {
		var flag bool
		switch v := rawVal.(type) {
		case int64:
			flag = v != 0
		case string:
			parsed, pErr := parseFlexibleBoolean(v)
			if pErr != nil {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s: %w", tableKey, colMeta.Name, pErr)
			}
			flag = parsed
		default:
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires Access BOOLEAN (-1 or 0)", tableKey, colMeta.Name)
		}
		var stored int64
		auditText := "0"
		if flag {
			stored = -1
			auditText = "-1"
		}
		s := strconv.FormatInt(stored, 10)
		return stored, &auditText, vprocore.ProjectMetadataCell{Storage: "integer", Integer: &s}, nil
	}

	switch colMeta.Kind {
	case "text":
		var text string
		switch v := rawVal.(type) {
		case string:
			text = v
		case int64:
			text = strconv.FormatInt(v, 10)
		case float64:
			text = vprocore.SourceDoubleDecimalString(v)
		default:
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires valid Unicode text", tableKey, colMeta.Name)
		}
		if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires valid UTF-8 text without NUL", tableKey, colMeta.Name)
		}
		for _, r := range text {
			if r >= 0xD800 && r <= 0xDFFF {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s contains unpaired UTF-16 surrogate", tableKey, colMeta.Name)
			}
		}
		if strings.TrimSpace(text) == "" && (colMeta.Name == "PlotNumber" || colMeta.Name == "Plot" ||
			(tableKey == "Veg" && colMeta.Name == "Species") || (tableKey == "Hierarchy" && colMeta.Name == "Name")) {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s cannot be blank", tableKey, colMeta.Name)
		}
		if colMeta.MaxUTF16 > 0 && len(utf16.Encode([]rune(text))) > colMeta.MaxUTF16 {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s exceeds %d UTF-16 code units", tableKey, colMeta.Name, colMeta.MaxUTF16)
		}
		auditCopy := text
		cellCopy := text
		return text, &auditCopy, vprocore.ProjectMetadataCell{Storage: "text", Text: &cellCopy}, nil

	case "integer":
		var num int64
		switch v := rawVal.(type) {
		case int64:
			num = v
		case float64:
			if math.Trunc(v) != v || v < math.MinInt32 || v > math.MaxInt32 {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires a whole integer", tableKey, colMeta.Name)
			}
			num = int64(v)
		case string:
			parsed, pErr := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if pErr != nil {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires an integer", tableKey, colMeta.Name)
			}
			num = parsed
		default:
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires an integer", tableKey, colMeta.Name)
		}
		if datasheetInt32Columns[tableKey][colMeta.Name] || strings.EqualFold(colMeta.DeclaredType, "INTEGER") {
			if num < math.MinInt32 || num > math.MaxInt32 {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s exceeds Access Long range (-2147483648..2147483647)", tableKey, colMeta.Name)
			}
		} else {
			if num < -32768 || num > 32767 {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s exceeds Access Integer range (-32768..32767)", tableKey, colMeta.Name)
			}
		}
		if tableKey == "Hierarchy" && colMeta.Name == "Level" && (num < 1 || num > 11) {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("Hierarchy.Level must be between 1 and 11, got %d", num)
		}
		s := strconv.FormatInt(num, 10)
		auditCopy := s
		return num, &auditCopy, vprocore.ProjectMetadataCell{Storage: "integer", Integer: &s}, nil

	case "single":
		var num float64
		switch v := rawVal.(type) {
		case float64:
			num = v
		case int64:
			num = float64(v)
		case string:
			parsed, pErr := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if pErr != nil {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires a finite number", tableKey, colMeta.Name)
			}
			num = parsed
		default:
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires a finite number", tableKey, colMeta.Name)
		}
		if math.IsNaN(num) || math.IsInf(num, 0) || math.Abs(num) > math.MaxFloat32 {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s exceeds physical IEEE-754 Single range", tableKey, colMeta.Name)
		}
		// Narrow float64 to float32 BEFORE evaluating `< 100 Or Is Null`!
		narrowed32 := float32(num)
		if math.IsNaN(float64(narrowed32)) || math.IsInf(float64(narrowed32), 0) {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s overflows IEEE-754 Single", tableKey, colMeta.Name)
		}
		if colMeta.IsCoverSingle {
			if narrowed32 >= 100 {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s must narrow to IEEE-754 float32 < 100 (or NULL)", tableKey, colMeta.Name)
			}
			if narrowed32 < 0 {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s cover cannot be negative", tableKey, colMeta.Name)
			}
		}
		stored := float64(narrowed32)
		if stored == 0 {
			stored = 0 // normalize -0 to +0
		}
		auditCopy := vprocore.SourceDoubleDecimalString(stored)
		cellVal := stored
		return stored, &auditCopy, vprocore.ProjectMetadataCell{Storage: "real", Real: &cellVal}, nil

	case "double":
		var num float64
		switch v := rawVal.(type) {
		case float64:
			num = v
		case int64:
			num = float64(v)
		case string:
			parsed, pErr := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if pErr != nil {
				return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires a finite number", tableKey, colMeta.Name)
			}
			num = parsed
		default:
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s requires a finite number", tableKey, colMeta.Name)
		}
		if math.IsNaN(num) || math.IsInf(num, 0) {
			return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("%s.%s must be finite", tableKey, colMeta.Name)
		}
		if num == 0 {
			num = 0
		}
		auditCopy := vprocore.SourceDoubleDecimalString(num)
		cellVal := num
		return num, &auditCopy, vprocore.ProjectMetadataCell{Storage: "real", Real: &cellVal}, nil
	}

	return nil, nil, vprocore.ProjectMetadataCell{}, fmt.Errorf("unsupported column kind %q for %s.%s", colMeta.Kind, tableKey, colMeta.Name)
}

func auditTextForStoredCell(cell vprocore.ProjectMetadataCell, isBool bool) *string {
	switch cell.Storage {
	case "null":
		return nil
	case "text":
		if cell.Text == nil {
			return nil
		}
		s := *cell.Text
		return &s
	case "integer":
		if cell.Integer == nil {
			return nil
		}
		if isBool {
			if *cell.Integer == "0" {
				s := "0"
				return &s
			}
			s := "-1"
			return &s
		}
		s := *cell.Integer
		return &s
	case "real":
		if cell.Real == nil {
			return nil
		}
		s := vprocore.SourceDoubleDecimalString(*cell.Real)
		return &s
	case "blob":
		if cell.BlobHex == nil {
			return nil
		}
		s := *cell.BlobHex
		return &s
	}
	return nil
}

func datasheetCellsSemanticallyEqual(a, b vprocore.ProjectMetadataCell, colMeta DatasheetColumnMeta) bool {
	if a.Storage == "null" && b.Storage == "null" {
		return true
	}
	if colMeta.IsBoolean && a.Storage == "integer" && b.Storage == "integer" && a.Integer != nil && b.Integer != nil {
		aTrue := *a.Integer != "0"
		bTrue := *b.Integer != "0"
		return aTrue == bTrue
	}
	if colMeta.Kind == "single" && a.Storage == "real" && b.Storage == "real" && a.Real != nil && b.Real != nil {
		return float32(*a.Real) == float32(*b.Real) && math.Signbit(*a.Real) == math.Signbit(*b.Real)
	}
	return vprocore.MetadataCellEqual(a, b)
}

func validateSoilHorizonDepthSign(tableKey string, horizon string, upper, lower *float64) error {
	if upper == nil || lower == nil {
		return nil
	}
	trimmed := strings.TrimSpace(horizon)
	if trimmed == "" {
		return nil
	}
	first := strings.ToUpper(trimmed)
	if tableKey == "Humus" {
		// Upland L/F/H measure upward from 0 cm (UpperDepth > LowerDepth); wetland Of/Om/Oh measure downward (UpperDepth < LowerDepth).
		if strings.HasPrefix(first, "OF") || strings.HasPrefix(first, "OM") || strings.HasPrefix(first, "OH") {
			if !(*upper < *lower) {
				return fmt.Errorf("wetland humus horizon %q requires UpperDepth < LowerDepth (measured downward)", horizon)
			}
		} else if strings.HasPrefix(first, "L") || strings.HasPrefix(first, "F") || strings.HasPrefix(first, "H") {
			if !(*upper > *lower) {
				return fmt.Errorf("upland humus horizon %q requires UpperDepth > LowerDepth (measured upward from 0 cm)", horizon)
			}
		}
	} else if tableKey == "Mineral" {
		if strings.HasPrefix(first, "A") || strings.HasPrefix(first, "B") || strings.HasPrefix(first, "C") ||
			strings.HasPrefix(first, "R") || strings.HasPrefix(first, "W") {
			if !(*upper < *lower) {
				return fmt.Errorf("mineral horizon %q requires UpperDepth < LowerDepth (measured downward)", horizon)
			}
		}
	}
	return nil
}

func replaceSubstringInsensitive(haystack, needle, replacement string) (string, bool) {
	if needle == "" {
		return haystack, false
	}
	lowerNeedle := strings.ToLower(needle)
	// Handle general UTF-8 case-folding slice-by-rune safely if lengths match, else fallback to index scan.
	var out strings.Builder
	changed := false
	i := 0
	for i < len(haystack) {
		idx := strings.Index(strings.ToLower(haystack[i:]), lowerNeedle)
		if idx < 0 {
			out.WriteString(haystack[i:])
			break
		}
		out.WriteString(haystack[i : i+idx])
		// Determine byte length in original haystack matching lowerNeedle rune count.
		needleRunes := []rune(needle)
		restRunes := []rune(haystack[i+idx:])
		if len(restRunes) < len(needleRunes) {
			out.WriteString(haystack[i+idx:])
			break
		}
		matchedOriginal := string(restRunes[:len(needleRunes)])
		if strings.EqualFold(matchedOriginal, needle) {
			out.WriteString(replacement)
			changed = true
			i += idx + len(matchedOriginal)
		} else {
			_, width := utf8.DecodeRuneInString(haystack[i+idx:])
			out.WriteString(haystack[i+idx : i+idx+width])
			i += idx + width
		}
	}
	return out.String(), changed
}

func PreviewDatasheetFindReplace(ctx context.Context, contexts *vprocore.ContextService, contextID string, request DatasheetFindReplaceRequest) (*DatasheetFindReplacePreview, error) {
	if ctx == nil {
		return nil, errors.New("datasheet find/replace preview requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("datasheet context service is unavailable")
	}
	if !utf8.ValidString(request.FindText) || !utf8.ValidString(request.ReplaceText) ||
		strings.ContainsRune(request.FindText, 0) || strings.ContainsRune(request.ReplaceText, 0) {
		return nil, errors.New("find/replace text must be valid UTF-8 without NUL")
	}
	maxPreview := request.MaxPreview
	if maxPreview <= 0 || maxPreview > 2000 {
		maxPreview = 500
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (_ *DatasheetFindReplacePreview, resultErr error) {
		owner := plots.Projects.Sqlite
		canEditMaster := plots.CanEditMasterBEC()
		target, err := findDatasheetTarget(owner, request.TableKey)
		if err != nil {
			return nil, err
		}
		if !target.available {
			return nil, errors.New(target.unavailableReason)
		}

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
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
		}()

		rawCols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, target.role, target.tableName)
		if err != nil {
			return nil, err
		}
		columns := make([]DatasheetColumnMeta, len(rawCols))
		colIndex := -1
		pinnedIndex := -1
		for i, col := range rawCols {
			columns[i] = classifyColumnMeta(target.key, col, target.pinnedColumn, canEditMaster)
			if strings.EqualFold(col.Name, strings.TrimSpace(request.Column)) {
				colIndex = i
			}
			if strings.EqualFold(col.Name, target.pinnedColumn) {
				pinnedIndex = i
			}
		}
		if colIndex < 0 {
			return nil, fmt.Errorf("unknown column %q on %s", request.Column, target.tableName)
		}
		colMeta := columns[colIndex]
		if !colMeta.Editable {
			return nil, fmt.Errorf("column %s.%s is protected or read-only", target.key, colMeta.Name)
		}

		whereSQL, whereArgs, orderSQL, err := buildDatasheetWhereAndOrder(
			ctx, tx, target, columns, request.Filters, request.GlobalSearch, request.ScopeToActiveSU, owner, nil,
		)
		if err != nil {
			return nil, err
		}

		// Query only rowid, pinned column, and target column for fast scanning even on large tables.
		scanCols := []DatasheetColumnMeta{colMeta}
		if pinnedIndex >= 0 && pinnedIndex != colIndex {
			scanCols = append(scanCols, columns[pinnedIndex])
		}
		rows, err := queryDatasheetWindowRows(ctx, tx, target.role, target.tableName, scanCols, whereSQL, whereArgs, orderSQL, 0, 0)
		if err != nil {
			return nil, err
		}

		preview := &DatasheetFindReplacePreview{
			ContextID: contextID,
			TableKey:  target.key,
			TableName: target.tableName,
			Column:    colMeta.Name,
			Items:     []DatasheetFindReplaceItem{},
		}

		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			beforeCell := row.Cells[0]
			pinnedVal := row.RowID
			if len(row.Cells) > 1 {
				pinnedVal = cellToDisplayString(row.Cells[1], columns[pinnedIndex].IsBoolean)
			} else if pinnedIndex == colIndex {
				pinnedVal = cellToDisplayString(beforeCell, colMeta.IsBoolean)
			}

			beforeText := cellToDisplayString(beforeCell, colMeta.IsBoolean)
			var afterText string
			matched := false

			if request.FindText == "" {
				// Empty find matches empty/NULL cells when exactCell is true
				if request.ExactCell && beforeCell.Storage == "null" {
					afterText = request.ReplaceText
					matched = request.ReplaceText != ""
				}
			} else if request.ExactCell {
				if request.MatchCase {
					if beforeCell.Storage != "null" && beforeText == request.FindText {
						afterText = request.ReplaceText
						matched = beforeText != afterText || (afterText == "" && beforeCell.Storage != "null")
					}
				} else {
					if beforeCell.Storage != "null" && strings.EqualFold(beforeText, request.FindText) {
						afterText = request.ReplaceText
						matched = beforeText != afterText || (afterText == "" && beforeCell.Storage != "null")
					}
				}
			} else {
				if beforeCell.Storage != "null" {
					if request.MatchCase {
						if strings.Contains(beforeText, request.FindText) {
							afterText = strings.ReplaceAll(beforeText, request.FindText, request.ReplaceText)
							matched = true
						}
					} else {
						replaced, ok := replaceSubstringInsensitive(beforeText, request.FindText, request.ReplaceText)
						if ok {
							afterText = replaced
							matched = true
						}
					}
				}
			}

			if !matched {
				continue
			}
			preview.MatchedRows++

			candidateCell, buildErr := buildReplacedCellFromString(colMeta, afterText)
			var valErr string
			normalizedCell := candidateCell
			if buildErr != nil {
				valErr = buildErr.Error()
			} else {
				_, _, norm, vErr := validateAndNormalizeDatasheetCell(target.key, colMeta, candidateCell, canEditMaster)
				if vErr != nil {
					valErr = vErr.Error()
				} else {
					normalizedCell = norm
				}
			}
			if valErr != "" {
				preview.InvalidCount++
			} else {
				preview.ValidCount++
			}

			if len(preview.Items) < maxPreview {
				preview.Items = append(preview.Items, DatasheetFindReplaceItem{
					RowID:           row.RowID,
					PinnedIdentity:  pinnedVal,
					Column:          colMeta.Name,
					BeforeText:      beforeText,
					AfterText:       afterText,
					BeforeCell:      vprocore.CloneSiteUnitCell(beforeCell),
					AfterCell:       vprocore.CloneSiteUnitCell(normalizedCell),
					ValidationError: valErr,
					Edit: DatasheetCellEdit{
						RowID:  row.RowID,
						Column: colMeta.Name,
						Before: vprocore.CloneSiteUnitCell(beforeCell),
						After:  vprocore.CloneSiteUnitCell(normalizedCell),
					},
				})
			} else {
				preview.Truncated = true
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
		return preview, nil
	})
}

func readSinglePhysicalRowByRowID(ctx context.Context, tx *sql.Tx, tableName string, columns []DatasheetColumnMeta, rowID int64) ([]vprocore.ProjectMetadataCell, error) {
	selectFields := make([]string, 0, len(columns)*2)
	for _, col := range columns {
		qCol := vprocore.QuoteHeaderIdentifier(col.Name)
		selectFields = append(selectFields,
			"typeof("+qCol+")",
			"CASE typeof("+qCol+") WHEN 'integer' THEN CAST("+qCol+" AS INTEGER) WHEN 'real' THEN CAST("+qCol+" AS REAL) ELSE CAST("+qCol+" AS BLOB) END",
		)
	}
	query := "SELECT " + strings.Join(selectFields, ",") + " FROM main." + vprocore.QuoteHeaderIdentifier(tableName) + " WHERE rowid = ?"
	storages := make([]string, len(columns))
	values := make([]any, len(columns))
	dest := make([]any, 0, len(columns)*2)
	for i := range columns {
		dest = append(dest, &storages[i], &values[i])
	}
	if err := tx.QueryRowContext(ctx, query, rowID).Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("row %d no longer exists in %s; reload before saving", rowID, tableName)
		}
		return nil, err
	}
	cells := make([]vprocore.ProjectMetadataCell, len(columns))
	for i, val := range values {
		cell, err := vprocore.ProjectMetadataCellInternal(storages[i], val)
		if err != nil {
			return nil, fmt.Errorf("row %d column %s: %w", rowID, columns[i].Name, err)
		}
		cells[i] = cell
	}
	return cells, nil
}

func cellFloat64Ptr(cell vprocore.ProjectMetadataCell) *float64 {
	if cell.Storage == "null" {
		return nil
	}
	if cell.Real != nil {
		v := *cell.Real
		return &v
	}
	if cell.Integer != nil {
		if n, err := strconv.ParseFloat(*cell.Integer, 64); err == nil {
			return &n
		}
	}
	return nil
}

// ApplyDatasheetEdits validates and commits a batch of cell edits inside a single
// atomic SQLite BEGIN IMMEDIATE transaction, omitting unchanged historical columns
// and writing one <Project>_Audit row per mutated project cell.
func ApplyDatasheetEdits(ctx context.Context, contexts *vprocore.ContextService, contextID string, request DatasheetEditRequest) (*DatasheetEditResult, error) {
	if ctx == nil {
		return nil, errors.New("datasheet edit requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("datasheet context service is unavailable")
	}
	if len(request.Edits) == 0 {
		return nil, errors.New("datasheet edit requires at least one cell edit")
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*DatasheetEditResult, error) {
		owner := plots.Projects.Sqlite
		canEditMaster := plots.CanEditMasterBEC()
		plots.Mu.RLock()
		currentUser := plots.CurrentUser
		plots.Mu.RUnlock()
		if strings.TrimSpace(currentUser) == "" {
			currentUser = "VPro"
		}

		target, err := findDatasheetTarget(owner, request.TableKey)
		if err != nil {
			return nil, err
		}
		if !target.available {
			return nil, errors.New(target.unavailableReason)
		}

		// Group edits by rowID in deterministic submission order, rejecting duplicate (rowId, column).
		type orderedRowEdits struct {
			rowID int64
			edits []DatasheetCellEdit
		}
		rowOrder := []int64{}
		byRow := map[int64]*orderedRowEdits{}
		seenCell := map[string]bool{}
		for _, edit := range request.Edits {
			rid, parseErr := strconv.ParseInt(strings.TrimSpace(edit.RowID), 10, 64)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid datasheet rowId %q", edit.RowID)
			}
			colKey := fmt.Sprintf("%d:%s", rid, strings.ToLower(strings.TrimSpace(edit.Column)))
			if seenCell[colKey] {
				return nil, fmt.Errorf("duplicate datasheet edit for row %d column %q in one batch", rid, edit.Column)
			}
			seenCell[colKey] = true
			bucket := byRow[rid]
			if bucket == nil {
				bucket = &orderedRowEdits{rowID: rid}
				byRow[rid] = bucket
				rowOrder = append(rowOrder, rid)
			}
			bucket.edits = append(bucket.edits, edit)
		}

		validateFiles := func() error {
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return err
			}
			roleInfo := owner.AttachmentInfo[target.role]
			if roleInfo == nil {
				return fmt.Errorf("datasheet role %s is not attached", target.role)
			}
			for supportRole, info := range owner.AttachmentInfo {
				if strings.HasPrefix(supportRole, "V") && os.SameFile(roleInfo, info) {
					return fmt.Errorf("datasheet target %s shares a file with support database %s; writes are prohibited", target.tableName, supportRole)
				}
			}
			return nil
		}

		result := &DatasheetEditResult{
			ContextID: contextID,
			TableKey:  target.key,
			TableName: target.tableName,
		}
		sameAsProject := owner.AttachmentInfo[target.role] != nil &&
			owner.AttachmentInfo["project"] != nil &&
			os.SameFile(owner.AttachmentInfo[target.role], owner.AttachmentInfo["project"])

		err = owner.WithSelectedFileWriter(ctx, target.role, validateFiles, func(conn *sql.Conn) (txErr error) {
			tx, err := conn.BeginTx(ctx, nil) // _txlock=immediate -> BEGIN IMMEDIATE
			if err != nil {
				return err
			}
			defer func() {
				if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rbErr)
				}
			}()

			var physicalExists bool
			if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM main.sqlite_master WHERE type='table' AND name COLLATE BINARY=?)", target.tableName).Scan(&physicalExists); err != nil {
				return err
			}
			if !physicalExists {
				return fmt.Errorf("datasheet target %s must be a physical table, not a view", target.tableName)
			}

			rawCols, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "main", target.tableName)
			if err != nil {
				return err
			}
			columns := make([]DatasheetColumnMeta, len(rawCols))
			colIndexByLower := make(map[string]int, len(rawCols))
			for i, col := range rawCols {
				columns[i] = classifyColumnMeta(target.key, col, target.pinnedColumn, canEditMaster)
				colIndexByLower[strings.ToLower(col.Name)] = i
			}

			project := owner.Selection.Project
			auditTable := vprocore.QuoteHeaderIdentifier(project + "_Audit")
			now := time.Now().Format("2006-01-02 15:04:05")

			// For Hierarchy edits, load full hierarchy map if Parent or Name changes so we enforce cycle & uniqueness invariants.
			var hierarchyParentCheck bool
			if target.key == "Hierarchy" {
				for _, ed := range request.Edits {
					if strings.EqualFold(ed.Column, "Parent") {
						hierarchyParentCheck = true
					}
				}
			}

			for _, rid := range rowOrder {
				if err := ctx.Err(); err != nil {
					return err
				}
				bucket := byRow[rid]
				currentCells, err := readSinglePhysicalRowByRowID(ctx, tx, target.tableName, columns, rid)
				if err != nil {
					return err
				}

				type preparedCellMutation struct {
					colIndex   int
					colMeta    DatasheetColumnMeta
					beforeCell vprocore.ProjectMetadataCell
					afterCell  vprocore.ProjectMetadataCell
					driverVal  any
					beforeText *string
					afterText  *string
				}
				var mutations []preparedCellMutation

				for _, edit := range bucket.edits {
					idx, ok := colIndexByLower[strings.ToLower(strings.TrimSpace(edit.Column))]
					if !ok {
						return fmt.Errorf("unknown column %q on %s", edit.Column, target.tableName)
					}
					colMeta := columns[idx]
					storedCell := currentCells[idx]

					// Optimistic concurrency check against expected Before cell
					if !datasheetCellsSemanticallyEqual(storedCell, edit.Before, colMeta) {
						return fmt.Errorf("concurrency conflict on %s row %d column %s: stored value changed; reload before saving",
							target.tableName, rid, colMeta.Name)
					}

					driverVal, afterAudit, normalizedCell, err := validateAndNormalizeDatasheetCell(target.key, colMeta, edit.After, canEditMaster)
					if err != nil {
						return err
					}

					// Omit unchanged cells so untouched / no-op cells never trigger spurious updates or audit rows!
					if datasheetCellsSemanticallyEqual(storedCell, normalizedCell, colMeta) {
						continue
					}

					mutations = append(mutations, preparedCellMutation{
						colIndex:   idx,
						colMeta:    colMeta,
						beforeCell: storedCell,
						afterCell:  normalizedCell,
						driverVal:  driverVal,
						beforeText: auditTextForStoredCell(storedCell, colMeta.IsBoolean),
						afterText:  afterAudit,
					})
				}

				if len(mutations) == 0 {
					continue
				}

				// Determine plot identity & child ID before/after mutation for plot-lock check and <Project>_Audit logging.
				var plotBefore, plotAfter string
				var childID *int64
				plotColName := ""
				switch target.key {
				case "Env", "Veg", "Humus", "Mineral", "Other", "SU":
					plotColName = "PlotNumber"
				case "Admin":
					plotColName = "Plot"
				}
				if plotColName != "" {
					if pIdx, ok := colIndexByLower[strings.ToLower(plotColName)]; ok && currentCells[pIdx].Text != nil {
						plotBefore = *currentCells[pIdx].Text
						plotAfter = plotBefore
					}
				}
				if idIdx, ok := colIndexByLower["id"]; ok && (target.key == "Veg" || target.key == "Humus" || target.key == "Mineral" || target.key == "Other") {
					if currentCells[idIdx].Integer != nil {
						if parsedID, pErr := strconv.ParseInt(*currentCells[idIdx].Integer, 10, 64); pErr == nil {
							childID = &parsedID
						}
					}
				}

				// Apply candidate post-edit row projection for cross-column soil depth sign check and plot rename tracking.
				projectedCells := slices.Clone(currentCells)
				togglingEnvFlag := false
				for _, m := range mutations {
					projectedCells[m.colIndex] = m.afterCell
					if strings.EqualFold(m.colMeta.Name, plotColName) && m.afterCell.Text != nil {
						plotAfter = *m.afterCell.Text
					}
					if target.key == "Env" && m.colMeta.Name == "Flag" {
						togglingEnvFlag = true
					}
				}

				// Check locked plot guard on project plot tables unless the user is explicitly toggling _Env.Flag.
				if sameAsProject && plotBefore != "" && !togglingEnvFlag &&
					(target.key == "Env" || target.key == "Admin" || target.key == "Veg" || target.key == "Humus" || target.key == "Mineral" || target.key == "Other") {
					var lockedCount int
					if err := tx.QueryRowContext(ctx,
						`SELECT COUNT(*) FROM main.`+vprocore.QuoteHeaderIdentifier(project+"_Env")+
							` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB) AND "Flag" IS NOT NULL AND CAST("Flag" AS INTEGER) <> 0`,
						plotBefore,
					).Scan(&lockedCount); err != nil {
						return err
					}
					if lockedCount > 0 {
						return fmt.Errorf("plot %q is locked (Flag = -1); unlock the plot before editing its cells", plotBefore)
					}
				}

				// Soil horizon LMH25 depth sign check ONLY when Horizon, UpperDepth, or LowerDepth is part of this row's mutations!
				if target.key == "Humus" || target.key == "Mineral" {
					soilDepthTouched := false
					for _, m := range mutations {
						if m.colMeta.Name == "Horizon" || m.colMeta.Name == "UpperDepth" || m.colMeta.Name == "LowerDepth" {
							soilDepthTouched = true
							break
						}
					}
					if soilDepthTouched {
						hIdx, hasH := colIndexByLower["horizon"]
						uIdx, hasU := colIndexByLower["upperdepth"]
						lIdx, hasL := colIndexByLower["lowerdepth"]
						if hasH && hasU && hasL && projectedCells[hIdx].Text != nil {
							if err := validateSoilHorizonDepthSign(
								target.key,
								*projectedCells[hIdx].Text,
								cellFloat64Ptr(projectedCells[uIdx]),
								cellFloat64Ptr(projectedCells[lIdx]),
							); err != nil {
								return err
							}
						}
					}
				}

				// Execute UPDATE touching ONLY mutated columns (preserving untouched historical out-of-rule columns).
				assignments := make([]string, len(mutations))
				updateArgs := make([]any, 0, len(mutations)+1)
				for i, m := range mutations {
					assignments[i] = vprocore.QuoteHeaderIdentifier(m.colMeta.Name) + " = ?"
					updateArgs = append(updateArgs, m.driverVal)
				}
				updateArgs = append(updateArgs, rid)
				updateSQL := "UPDATE main." + vprocore.QuoteHeaderIdentifier(target.tableName) +
					" SET " + strings.Join(assignments, ", ") + " WHERE rowid = ?"
				execRes, err := tx.ExecContext(ctx, updateSQL, updateArgs...)
				if err != nil {
					return fmt.Errorf("update %s row %d: %w", target.tableName, rid, err)
				}
				if affected, aErr := execRes.RowsAffected(); aErr != nil || affected != 1 {
					return errors.Join(aErr, fmt.Errorf("update %s row %d affected %d rows (expected 1)", target.tableName, rid, affected))
				}

				result.UpdatedRows++
				result.UpdatedCells += len(mutations)

				// Write one <Project>_Audit row per mutated project cell inside the same BEGIN IMMEDIATE transaction!
				if sameAsProject && plotAfter != "" {
					canAudit := target.key == "Env" || target.key == "Admin" ||
						target.key == "Veg" || target.key == "Humus" || target.key == "Mineral" || target.key == "Other"
					if target.key == "SU" {
						var envMatch int
						if err := tx.QueryRowContext(ctx,
							`SELECT COUNT(*) FROM main.`+vprocore.QuoteHeaderIdentifier(project+"_Env")+
								` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`,
							plotAfter,
						).Scan(&envMatch); err != nil {
							return err
						}
						canAudit = envMatch == 1
					}
					if canAudit {
						tableSuffix := "_" + target.key
						for _, m := range mutations {
							if childID != nil {
								if _, err := tx.ExecContext(ctx,
									`INSERT INTO main.`+auditTable+
										` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
									project, currentUser, plotAfter, tableSuffix, m.colMeta.Name, now, m.beforeText, m.afterText, *childID,
								); err != nil {
									return fmt.Errorf("write child audit for %s.%s: %w", target.tableName, m.colMeta.Name, err)
								}
							} else {
								if _, err := tx.ExecContext(ctx,
									`INSERT INTO main.`+auditTable+
										` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,?,?,?,0,0)`,
									project, currentUser, plotAfter, tableSuffix, m.colMeta.Name, now, m.beforeText, m.afterText,
								); err != nil {
									return fmt.Errorf("write parent audit for %s.%s: %w", target.tableName, m.colMeta.Name, err)
								}
							}
							result.AuditRows++
						}
					}
				}
			}

			if hierarchyParentCheck {
				if err := verifyHierarchyAcyclicInTx(ctx, tx, target.tableName); err != nil {
					return err
				}
			}

			if result.UpdatedCells > 0 {
				proposalBytes, err := json.Marshal(struct {
					ContextID string               `json:"contextId"`
					User      string               `json:"user"`
					Request   DatasheetEditRequest `json:"request"`
				}{contextID, currentUser, request})
				if err != nil {
					return err
				}
				if err := vprocore.AppendTechnicalProvenance(ctx, tx, datasheetHistoryTable, datasheetHistorySQL, string(proposalBytes), "datasheet edit"); err != nil {
					return err
				}
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
		return result, nil
	})
}

func verifyHierarchyAcyclicInTx(ctx context.Context, tx *sql.Tx, tableName string) error {
	rows, err := tx.QueryContext(ctx, "SELECT ID, Parent FROM main."+vprocore.QuoteHeaderIdentifier(tableName))
	if err != nil {
		return err
	}
	defer rows.Close()
	parents := map[int64]*int64{}
	for rows.Next() {
		var id int64
		var parent sql.NullInt64
		if err := rows.Scan(&id, &parent); err != nil {
			return err
		}
		if parent.Valid {
			p := parent.Int64
			parents[id] = &p
		} else {
			parents[id] = nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, p := range parents {
		if p != nil {
			if _, ok := parents[*p]; !ok {
				return fmt.Errorf("Hierarchy node %d references non-existent Parent %d", id, *p)
			}
		}
		visited := map[int64]bool{id: true}
		curr := p
		for curr != nil {
			if visited[*curr] {
				return fmt.Errorf("Hierarchy edit introduces a cycle involving node %d", *curr)
			}
			visited[*curr] = true
			curr = parents[*curr]
		}
	}
	return nil
}
