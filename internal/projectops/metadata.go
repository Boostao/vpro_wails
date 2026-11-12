package projectops

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type ProjectMetadataPreset struct {
	ID                       string            `json:"id"`
	Label                    string            `json:"label"`
	Description              string            `json:"description"`
	EcosysCollectionStandard string            `json:"ecosysCollectionStandard"`
	VegCoverMethod           string            `json:"vegCoverMethod"`
	PlotMethod               string            `json:"plotMethod"`
	Datum                    string            `json:"datum"`
	CoverDescriptions        map[string]string `json:"coverDescriptions"`
}

var coverDescriptionColumns = []string{
	"CoverA1Description",
	"CoverA2Description",
	"CoverA3Description",
	"CoverADescription",
	"CoverB1Description",
	"CoverB2Description",
	"CoverB2aDescription",
	"CoverB2bDescription",
	"CoverB2cDescription",
	"CoverBDescription",
	"CoverCDescription",
	"CoverDDescription",
	"Cover8Description",
	"Cover9Description",
	"Cover10Description",
}

var builtinMetadataPresets = []ProjectMetadataPreset{
	{
		ID:                       "LMH25_2023_FS882",
		Label:                    "LMH25 (2023) Standard FS882",
		Description:              "B.C. Ministry of Forests LMH 25 (2nd Ed., 2023) full ecosystem field form (20x20 m, NAD83, standard tree/shrub/herb/moss strata).",
		EcosysCollectionStandard: "LMH25",
		VegCoverMethod:           "Percent",
		PlotMethod:               "20x20",
		Datum:                    "2",
		CoverDescriptions: map[string]string{
			"CoverA1Description":  "Dominant / emergent / veteran trees (>10 m)",
			"CoverA2Description":  "Main tree canopy (>10 m)",
			"CoverA3Description":  "Sub-canopy / suppressed trees (>10 m)",
			"CoverADescription":   "Total of all tree layers (>10 m)",
			"CoverB1Description":  "Tall shrubs and tree regeneration (2 to 10 m)",
			"CoverB2Description":  "Low shrubs and woody regeneration (0.15 to 2 m)",
			"CoverB2aDescription": "X",
			"CoverB2bDescription": "X",
			"CoverB2cDescription": "X",
			"CoverBDescription":   "Total of all shrub layers (<10 m)",
			"CoverCDescription":   "Herbaceous species and specified dwarf woody species (<15 cm)",
			"CoverDDescription":   "Bryophytes, terrestrial lichens, liverworts and seedlings (<2 yr) on humus/soil",
			"Cover8Description":   "Epixyls - bryophytes and lichens on decaying wood",
			"Cover9Description":   "Epiliths - bryophytes and lichens on rock",
			"Cover10Description":  "Epiphytes - bryophytes and lichens on trees and shrubs",
		},
	},
	{
		ID:                       "LMH25_FS1333_SIVI",
		Label:                    "LMH25 FS1333 SIVI Ground",
		Description:              "LMH 25 FS1333 Site Inspection and Vegetation Inventory rapid ground/visual verification protocol (NAD83).",
		EcosysCollectionStandard: "LMH25",
		VegCoverMethod:           "Percent",
		PlotMethod:               "20x20",
		Datum:                    "2",
		CoverDescriptions: map[string]string{
			"CoverA1Description":  "SIVI dominant trees (>10 m)",
			"CoverA2Description":  "SIVI main canopy trees (>10 m)",
			"CoverA3Description":  "SIVI sub-canopy trees (>10 m)",
			"CoverADescription":   "SIVI combined tree stratum total (>10 m)",
			"CoverB1Description":  "SIVI tall shrub / regen layer (2 to 10 m)",
			"CoverB2Description":  "SIVI low shrub layer (<2 m)",
			"CoverB2aDescription": "X",
			"CoverB2bDescription": "X",
			"CoverB2cDescription": "X",
			"CoverBDescription":   "SIVI combined shrub stratum total (<10 m)",
			"CoverCDescription":   "SIVI herb and dwarf shrub layer",
			"CoverDDescription":   "SIVI moss, lichen and seedling layer on forest floor",
			"Cover8Description":   "SIVI epixyls on decaying wood",
			"Cover9Description":   "SIVI epiliths on rock substrates",
			"Cover10Description":  "SIVI epiphytes on standing bark/branches",
		},
	},
	{
		ID:                       "DEIF_1980",
		Label:                    "DEIF 1980 Historical",
		Description:              "Describing Ecosystems in the Field (RAB Technical Paper 2, Walmsley et al. 1980) historical BEC plot standard normalized to NAD83.",
		EcosysCollectionStandard: "DEIF 1980",
		VegCoverMethod:           "Percent",
		PlotMethod:               "20x20",
		Datum:                    "2",
		CoverDescriptions: map[string]string{
			"CoverA1Description":  "DEIF 1980 A1 veteran/emergent tree sublayer (>10 m)",
			"CoverA2Description":  "DEIF 1980 A2 main tree canopy (>10 m)",
			"CoverA3Description":  "DEIF 1980 A3 lower tree canopy (>10 m)",
			"CoverADescription":   "DEIF 1980 total A tree layer (>10 m)",
			"CoverB1Description":  "DEIF 1980 B1 tall shrub layer (2 to 10 m)",
			"CoverB2Description":  "DEIF 1980 B2 low shrub layer (<2 m)",
			"CoverB2aDescription": "X",
			"CoverB2bDescription": "X",
			"CoverB2cDescription": "X",
			"CoverBDescription":   "DEIF 1980 total B shrub layer (<10 m)",
			"CoverCDescription":   "DEIF 1980 C herb layer",
			"CoverDDescription":   "DEIF 1980 D moss, lichen and seedling layer",
			"Cover8Description":   "DEIF 1980 Dh/Dw decaying wood bryophytes",
			"Cover9Description":   "DEIF 1980 Dr rock bryophytes and lichens",
			"Cover10Description":  "DEIF 1980 E epiphytic bryophytes and lichens",
		},
	},
	{
		ID:                       "CHARS_Arctic",
		Label:                    "CHARS Arctic",
		Description:              "Canadian High Arctic Research Station (CHARS) tundra/subarctic standard with extended shrub sub-layers Cover5a..5c (NAD83).",
		EcosysCollectionStandard: "CHAR",
		VegCoverMethod:           "Percent",
		PlotMethod:               "20x20",
		Datum:                    "2",
		CoverDescriptions: map[string]string{
			"CoverA1Description":  "Dominant trees (>10 m, subarctic treeline)",
			"CoverA2Description":  "Main tree canopy (>10 m, subarctic treeline)",
			"CoverA3Description":  "Sub-canopy trees (>10 m, subarctic treeline)",
			"CoverADescription":   "Total tree stratum (>10 m)",
			"CoverB1Description":  "Tall shrubs and krummholz (2 to 10 m)",
			"CoverB2Description":  "Undivided low shrubs (0.15 to 2 m)",
			"CoverB2aDescription": "CHARS tall-low Arctic shrubs (0.40 to 2.0 m)",
			"CoverB2bDescription": "CHARS medium-low Arctic shrubs (0.15 to 0.40 m)",
			"CoverB2cDescription": "CHARS prostrate and erect dwarf shrubs (<0.15 m)",
			"CoverBDescription":   "Total shrub stratum including extended Arctic layers",
			"CoverCDescription":   "Graminoids, forbs and non-shrub vascular plants",
			"CoverDDescription":   "Terricolous bryophytes, lichens and biological soil crust",
			"Cover8Description":   "Lignicolous / epixylic bryophytes and lichens",
			"Cover9Description":   "Saxicolous / epilithic bryophytes and lichens",
			"Cover10Description":  "Epiphytic lichens and bryophytes on shrubs/trees",
		},
	},
}

func ListProjectMetadataPresets() []ProjectMetadataPreset {
	out := make([]ProjectMetadataPreset, len(builtinMetadataPresets))
	for i, p := range builtinMetadataPresets {
		copied := make(map[string]string, len(p.CoverDescriptions))
		for k, v := range p.CoverDescriptions {
			copied[k] = v
		}
		p.CoverDescriptions = copied
		out[i] = p
	}
	return out
}

func GetProjectMetadataPreset(id string) (ProjectMetadataPreset, bool) {
	trimmed := strings.TrimSpace(id)
	for _, p := range ListProjectMetadataPresets() {
		if strings.EqualFold(p.ID, trimmed) {
			return p, true
		}
	}
	return ProjectMetadataPreset{}, false
}

type DerivedProjectMetadata struct {
	Project               string   `json:"project"`
	ProjectID             string   `json:"projectId"`
	TotalPlotsInScope     int      `json:"totalPlotsInScope"`
	StartDate             *int     `json:"startDate"`
	EndDate               *int     `json:"endDate"`
	NumberOfFS882Plots    int      `json:"numberOfFs882Plots"`
	NumberOfSiteVisits    int      `json:"numberOfSiteVisits"`
	UnclassifiedPlotTypes int      `json:"unclassifiedPlotTypes"`
	GeographicStudyRegion string   `json:"geographicStudyRegion"`
	RegionCodesMatched    []string `json:"regionCodesMatched"`
	CollectedSite         int      `json:"collectedSite"`
	CollectedVeg          int      `json:"collectedVeg"`
	CollectedSoil         int      `json:"collectedSoil"`
	CollectedTerrain      int      `json:"collectedTerrain"`
	AllSpecs              string   `json:"allSpecs"`
	TableOfLists          string   `json:"tableOfLists"`
	VListsSHA256Short     string   `json:"vlistsSha256Short"`
}

var fourDigitYearRegex = regexp.MustCompile(`\b(19\d{2}|2[0-4]\d{2}|2500)\b`)

func parseSurveyYear(raw string) (int, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	if len(trimmed) >= 4 {
		if yr, err := strconv.Atoi(trimmed[:4]); err == nil && yr >= 1900 && yr <= 2500 {
			return yr, true
		}
	}
	if m := fourDigitYearRegex.FindStringSubmatch(trimmed); len(m) == 2 {
		if yr, err := strconv.Atoi(m[1]); err == nil && yr >= 1900 && yr <= 2500 {
			return yr, true
		}
	}
	return 0, false
}

// DeriveProjectMetadataFromPlots computes Auto-6 metadata aggregations from plots matching projectID
// (or all plots in the active project when projectID is empty) plus active VLists 2026 version stamps.
func DeriveProjectMetadataFromPlots(s *vprocore.ContextService, ctx context.Context, contextID, projectID string) (DerivedProjectMetadata, error) {
	if !utf8.ValidString(projectID) || strings.ContainsRune(projectID, 0) {
		return DerivedProjectMetadata{}, errors.New("projectID must be valid UTF-8")
	}
	snap, err := inspectContextSnapshot(ctx, s, contextID)
	if err != nil {
		return DerivedProjectMetadata{}, err
	}

	allSpecsStamp, listsStamp, shortDigest, err := readVListsVersionStamps(ctx, snap.VListsPath)
	if err != nil {
		return DerivedProjectMetadata{}, err
	}

	regionSvc, err := references.NewRegionCodeService(filepath.Dir(snap.ProjectPath))
	var knownRegions map[string]string
	if err == nil {
		defer regionSvc.Close()
		choices, rErr := regionSvc.ListRegionChoices(ctx)
		if rErr == nil {
			knownRegions = make(map[string]string, len(choices))
			for _, ch := range choices {
				if ch.Code != nil && strings.TrimSpace(*ch.Code) != "" {
					code := strings.TrimSpace(*ch.Code)
					label := code
					if ch.Description != nil && strings.TrimSpace(*ch.Description) != "" {
						label = fmt.Sprintf("%s (%s)", code, strings.TrimSpace(*ch.Description))
					}
					knownRegions[strings.ToUpper(code)] = label
				}
			}
		}
	}

	db, err := sqlutil.OpenReadOnlyContext(ctx, snap.ProjectPath)
	if err != nil {
		return DerivedProjectMetadata{}, err
	}
	defer db.Close()

	envTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Env")
	adminTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Admin")
	vegTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Veg")
	humusTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Humus")
	mineralTbl := vprocore.QuoteHeaderIdentifier(snap.Project + "_Mineral")

	targetPID := strings.TrimSpace(projectID)
	var whereClause string
	var args []any
	if targetPID != "" {
		whereClause = `WHERE typeof(e."ProjectID")='text' AND CAST(e."ProjectID" AS BLOB)=CAST(? AS BLOB)`
		args = append(args, targetPID)
	}

	query := `SELECT
		CAST(e."PlotNumber" AS TEXT),
		COALESCE(CAST(e."ProjectID" AS TEXT), ''),
		COALESCE(CAST(e."Date" AS TEXT), ''),
		COALESCE(CAST(e."FSRegionDistrict" AS TEXT), ''),
		CASE WHEN (e."Zone" IS NOT NULL AND CAST(e."Zone" AS TEXT) <> '')
		       OR e."Elevation" IS NOT NULL
		       OR e."SlopeGradient" IS NOT NULL
		       OR e."Aspect" IS NOT NULL
		       OR (e."MoistureRegime" IS NOT NULL AND CAST(e."MoistureRegime" AS TEXT) <> '')
		       OR (e."NutrientRegime" IS NOT NULL AND CAST(e."NutrientRegime" AS TEXT) <> '')
		     THEN 1 ELSE 0 END AS has_site,
		CASE WHEN e."SpeciesListComplete" IS NOT NULL AND CAST(e."SpeciesListComplete" AS INTEGER) <> 0 THEN 1 ELSE 0 END AS spp_complete,
		EXISTS(SELECT 1 FROM ` + vegTbl + ` v WHERE typeof(v."PlotNumber")='text' AND CAST(v."PlotNumber" AS BLOB)=CAST(e."PlotNumber" AS BLOB)) AS has_veg,
		(EXISTS(SELECT 1 FROM ` + humusTbl + ` h WHERE typeof(h."PlotNumber")='text' AND CAST(h."PlotNumber" AS BLOB)=CAST(e."PlotNumber" AS BLOB))
		 OR EXISTS(SELECT 1 FROM ` + mineralTbl + ` m WHERE typeof(m."PlotNumber")='text' AND CAST(m."PlotNumber" AS BLOB)=CAST(e."PlotNumber" AS BLOB))) AS has_soil,
		CASE WHEN e."SurficialMaterialSurf" IS NOT NULL AND TRIM(CAST(e."SurficialMaterialSurf" AS TEXT)) <> '' THEN 1 ELSE 0 END AS has_terrain,
		COALESCE(CAST(a."PlotType" AS TEXT), ''),
		a."StartDate"
	FROM ` + envTbl + ` e
	LEFT JOIN ` + adminTbl + ` a ON typeof(a."Plot")='text' AND CAST(a."Plot" AS BLOB)=CAST(e."PlotNumber" AS BLOB)
	` + whereClause + `
	ORDER BY e."PlotNumber"`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return DerivedProjectMetadata{}, err
	}
	defer rows.Close()

	effectivePID := targetPID
	if effectivePID == "" {
		effectivePID = snap.Project
	}
	out := DerivedProjectMetadata{
		Project:            snap.Project,
		ProjectID:          effectivePID,
		RegionCodesMatched: []string{},
		CollectedSite:      3,
		CollectedVeg:       3,
		CollectedSoil:      3,
		CollectedTerrain:   3,
		AllSpecs:           allSpecsStamp,
		TableOfLists:       listsStamp,
		VListsSHA256Short:  shortDigest,
	}

	var minYear, maxYear int
	hasYear := false
	sitePlots, vegPlots, vegCompletePlots, soilPlots, terrainPlots := 0, 0, 0, 0, 0
	regionSet := map[string]string{}
	distinctPIDs := map[string]bool{}

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return DerivedProjectMetadata{}, err
		}
		var plotNum, pid, dateStr, fsReg, plotType string
		var hasSite, sppComplete int
		var hasVeg, hasSoil bool
		var hasTerrain int
		var adminStart sql.NullInt64
		if err := rows.Scan(&plotNum, &pid, &dateStr, &fsReg, &hasSite, &sppComplete, &hasVeg, &hasSoil, &hasTerrain, &plotType, &adminStart); err != nil {
			return DerivedProjectMetadata{}, err
		}
		out.TotalPlotsInScope++
		if pid != "" {
			distinctPIDs[pid] = true
		}
		if yr, ok := parseSurveyYear(dateStr); ok {
			if !hasYear || yr < minYear {
				minYear = yr
			}
			if !hasYear || yr > maxYear {
				maxYear = yr
			}
			hasYear = true
		} else if adminStart.Valid && adminStart.Int64 >= 1900 && adminStart.Int64 <= 2500 {
			yr := int(adminStart.Int64)
			if !hasYear || yr < minYear {
				minYear = yr
			}
			if !hasYear || yr > maxYear {
				maxYear = yr
			}
			hasYear = true
		}

		switch strings.ToUpper(strings.TrimSpace(plotType)) {
		case "FS882", "FULL":
			out.NumberOfFS882Plots++
		case "GROUND", "VISUAL", "NOTE", "OTHER", "G", "V", "N", "O":
			out.NumberOfSiteVisits++
		default:
			out.UnclassifiedPlotTypes++
		}

		regTrim := strings.TrimSpace(fsReg)
		if regTrim != "" {
			upper := strings.ToUpper(regTrim)
			if knownRegions != nil {
				if _, ok := knownRegions[upper]; ok {
					regionSet[upper] = regTrim
				} else {
					prefix := strings.SplitN(upper, ".", 2)[0]
					matchedPrefix := false
					for code := range knownRegions {
						if strings.HasPrefix(code, prefix+".") || code == prefix {
							regionSet[regTrim] = regTrim
							matchedPrefix = true
							break
						}
					}
					if !matchedPrefix {
						regionSet[regTrim] = regTrim
					}
				}
			} else {
				regionSet[upper] = regTrim
			}
		}

		if hasSite > 0 {
			sitePlots++
		}
		if hasVeg {
			vegPlots++
			if sppComplete > 0 {
				vegCompletePlots++
			}
		}
		if hasSoil {
			soilPlots++
		}
		if hasTerrain > 0 {
			terrainPlots++
		}
	}
	if err := rows.Err(); err != nil {
		return DerivedProjectMetadata{}, err
	}

	if targetPID == "" && len(distinctPIDs) == 1 {
		for k := range distinctPIDs {
			out.ProjectID = k
		}
	}
	if hasYear {
		sYr, eYr := minYear, maxYear
		out.StartDate = &sYr
		out.EndDate = &eYr
	}

	codes := make([]string, 0, len(regionSet))
	for _, val := range regionSet {
		codes = append(codes, val)
	}
	slices.Sort(codes)
	out.RegionCodesMatched = codes
	joinedRegions := strings.Join(codes, "; ")
	if utf16Len(joinedRegions) > 255 {
		for len(codes) > 0 && utf16Len(strings.Join(codes, "; ")) > 255 {
			codes = codes[:len(codes)-1]
		}
		joinedRegions = strings.Join(codes, "; ")
	}
	out.GeographicStudyRegion = joinedRegions

	classifyCompleteness := func(populated, total int) int {
		if total == 0 || populated == 0 {
			return 3
		}
		if populated == total {
			return 1
		}
		return 2
	}
	out.CollectedSite = classifyCompleteness(sitePlots, out.TotalPlotsInScope)
	if out.TotalPlotsInScope == 0 || vegPlots == 0 {
		out.CollectedVeg = 3
	} else if vegPlots == out.TotalPlotsInScope && vegCompletePlots == out.TotalPlotsInScope {
		out.CollectedVeg = 1
	} else {
		out.CollectedVeg = 2
	}
	out.CollectedSoil = classifyCompleteness(soilPlots, out.TotalPlotsInScope)
	out.CollectedTerrain = classifyCompleteness(terrainPlots, out.TotalPlotsInScope)

	return out, nil
}

type ApplyProjectMetadataAutomationRequest struct {
	ProjectID    string `json:"projectId"`
	ApplyDerived bool   `json:"applyDerived"`
	PresetID     string `json:"presetId,omitempty"`
}

type ApplyProjectMetadataAutomationResult struct {
	Project        string                 `json:"project"`
	ProjectID      string                 `json:"projectId"`
	MetadataID     int64                  `json:"metadataId"`
	CreatedRow     bool                   `json:"createdRow"`
	UpdatedColumns []string               `json:"updatedColumns"`
	AuditRowCount  int                    `json:"auditRowCount"`
	Derived        DerivedProjectMetadata `json:"derived"`
}

// ApplyProjectMetadataAutomation writes derived plot aggregations and/or an LMH25/DEIF/CHARS preset
// into <Project>_Metadata inside a single atomic BEGIN IMMEDIATE transaction with <Project>_Audit entries.
func ApplyProjectMetadataAutomation(s *vprocore.ContextService, ctx context.Context, contextID string, req ApplyProjectMetadataAutomationRequest) (ApplyProjectMetadataAutomationResult, error) {
	if !req.ApplyDerived && strings.TrimSpace(req.PresetID) == "" {
		return ApplyProjectMetadataAutomationResult{}, errors.New("at least one of applyDerived or presetId must be specified")
	}
	var preset ProjectMetadataPreset
	hasPreset := false
	if strings.TrimSpace(req.PresetID) != "" {
		var ok bool
		preset, ok = GetProjectMetadataPreset(req.PresetID)
		if !ok {
			return ApplyProjectMetadataAutomationResult{}, fmt.Errorf("unknown metadata preset %q", req.PresetID)
		}
		hasPreset = true
	}

	derived, err := DeriveProjectMetadataFromPlots(s, ctx, contextID, req.ProjectID)
	if err != nil {
		return ApplyProjectMetadataAutomationResult{}, err
	}
	targetPID := strings.TrimSpace(req.ProjectID)
	if targetPID == "" {
		targetPID = derived.ProjectID
	}
	if utf16Len(targetPID) > 20 {
		targetPID = string([]rune(targetPID)[:20])
	}
	if err := validateUTF16Max("Metadata.ProjectID", targetPID, 20); err != nil {
		return ApplyProjectMetadataAutomationResult{}, err
	}

	var result ApplyProjectMetadataAutomationResult
	err = s.Edit(contextID, func(plots *vprocore.PlotService) error {
		owner := plots.Projects.Sqlite
		user := strings.TrimSpace(plots.CurrentUser)
		if user == "" {
			user = "VProUser"
		}
		return owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			project := owner.Selection.Project
			metaTbl := vprocore.QuoteHeaderIdentifier(project + "_Metadata")
			auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")
			now := time.Now().Format("2006-01-02 15:04:05")

			// Locate existing row by literal ProjectID or create one with monotonic ID.
			var rowID int64
			var metaID int64
			created := false
			qErr := tx.QueryRowContext(ctx, `SELECT rowid, "ID" FROM `+metaTbl+` WHERE typeof("ProjectID")='text' AND CAST("ProjectID" AS BLOB)=CAST(? AS BLOB) ORDER BY "ID" LIMIT 1`, targetPID).Scan(&rowID, &metaID)
			if errors.Is(qErr, sql.ErrNoRows) {
				allocID, aErr := allocateSafeMetadataID(ctx, tx, project)
				if aErr != nil {
					return aErr
				}
				res, iErr := tx.ExecContext(ctx, `INSERT INTO `+metaTbl+` ("ID", "ProjectID", "DateLastEdited") VALUES (?, ?, ?)`, allocID, targetPID, now)
				if iErr != nil {
					return iErr
				}
				rowID, iErr = res.LastInsertId()
				if iErr != nil {
					return iErr
				}
				metaID = allocID
				created = true
			} else if qErr != nil {
				return qErr
			}

			desired := map[string]any{}
			var orderedCols []string
			addCol := func(col string, val any) {
				if _, exists := desired[col]; !exists {
					orderedCols = append(orderedCols, col)
				}
				desired[col] = val
			}

			if req.ApplyDerived {
				if derived.StartDate != nil {
					addCol("StartDate", int64(*derived.StartDate))
				} else {
					addCol("StartDate", nil)
				}
				if derived.EndDate != nil {
					addCol("EndDate", int64(*derived.EndDate))
				} else {
					addCol("EndDate", nil)
				}
				addCol("NumberOfFS882Plots", int64(derived.NumberOfFS882Plots))
				addCol("NumberOfSiteVisits", int64(derived.NumberOfSiteVisits))
				if derived.GeographicStudyRegion != "" {
					addCol("GeographicStudyRegion", derived.GeographicStudyRegion)
				}
				addCol("CollectedSite", int64(derived.CollectedSite))
				addCol("CollectedVeg", int64(derived.CollectedVeg))
				addCol("CollectedSoil", int64(derived.CollectedSoil))
				addCol("CollectedTerrain", int64(derived.CollectedTerrain))
				addCol("AllSpecs", derived.AllSpecs)
				addCol("TableOfLists", derived.TableOfLists)
			}

			if hasPreset {
				addCol("EcosysCollectionStandard", preset.EcosysCollectionStandard)
				addCol("VegCoverMethod", preset.VegCoverMethod)
				addCol("PlotMethod", preset.PlotMethod)
				addCol("Datum", preset.Datum)
				for _, col := range coverDescriptionColumns {
					if val, ok := preset.CoverDescriptions[col]; ok {
						addCol(col, val)
					}
				}
			}

			var updatedCols []string
			auditCount := 0
			if created {
				if _, aErr := tx.ExecContext(ctx, `INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
					project, user, targetPID, "_Metadata", "ProjectID", now, nil, targetPID, metaID); aErr != nil {
					return aErr
				}
				auditCount++
			}

			for _, col := range orderedCols {
				newVal := desired[col]
				var oldRaw sql.NullString
				if sErr := tx.QueryRowContext(ctx, `SELECT CAST(`+vprocore.QuoteHeaderIdentifier(col)+` AS TEXT) FROM `+metaTbl+` WHERE rowid = ?`, rowID).Scan(&oldRaw); sErr != nil {
					return sErr
				}
				var newText *string
				if newVal != nil {
					sVal := fmt.Sprint(newVal)
					newText = &sVal
				}
				same := (!oldRaw.Valid && newText == nil) || (oldRaw.Valid && newText != nil && oldRaw.String == *newText)
				if same {
					continue
				}
				if _, uErr := tx.ExecContext(ctx, `UPDATE `+metaTbl+` SET `+vprocore.QuoteHeaderIdentifier(col)+` = ?, "DateLastEdited" = ? WHERE rowid = ?`, newVal, now, rowID); uErr != nil {
					return uErr
				}
				var beforeAudit any
				if oldRaw.Valid {
					beforeAudit = oldRaw.String
				}
				var afterAudit any
				if newText != nil {
					afterAudit = *newText
				}
				if _, aErr := tx.ExecContext(ctx, `INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
					project, user, targetPID, "_Metadata", col, now, beforeAudit, afterAudit, metaID); aErr != nil {
					return aErr
				}
				updatedCols = append(updatedCols, col)
				auditCount++
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
			result = ApplyProjectMetadataAutomationResult{
				Project:        project,
				ProjectID:      targetPID,
				MetadataID:     metaID,
				CreatedRow:     created,
				UpdatedColumns: updatedCols,
				AuditRowCount:  auditCount,
				Derived:        derived,
			}
			return nil
		})
	})
	if err != nil {
		return ApplyProjectMetadataAutomationResult{}, err
	}
	return result, nil
}

func allocateSafeMetadataID(ctx context.Context, tx *sql.Tx, project string) (int64, error) {
	return allocateSafeChildID(ctx, tx, project, "Metadata")
}
