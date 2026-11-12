package interchange

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const (
	TurboVegFormatXML    = "xml"
	TurboVegFormatSQLite = "sqlite"
)

type turboVegLayerCrosswalk struct {
	LayerID     int
	LayerCode   string
	TVGroup     string
	VProCover   string
	VProHeight  string
	Description string
}

var turboVegCrosswalk = []turboVegLayerCrosswalk{
	{1, "1", "Tree", "Cover1", "Height1", "Dominant tree layer (A1)"},
	{2, "2", "Tree", "Cover2", "Height2", "Main canopy tree layer (A2)"},
	{3, "3", "Tree", "Cover3", "Height3", "Sub-canopy tree layer (A3)"},
	{11, "TotalA", "Tree", "TotalA", "HeightA", "Combined tree stratum (TotalA)"},
	{4, "4", "Shrub", "Cover4", "Height4", "Tall shrub layer (B1)"},
	{5, "5", "Shrub", "Cover5", "Height5", "Low shrub layer (B2)"},
	{51, "5a", "Shrub", "Cover5a", "Height5a", "Extended shrub sub-layer (B2a)"},
	{52, "5b", "Shrub", "Cover5b", "Height5b", "Extended shrub sub-layer (B2b)"},
	{53, "5c", "Shrub", "Cover5c", "Height5c", "Extended shrub sub-layer (B2c)"},
	{12, "TotalB", "Shrub", "TotalB", "HeightB", "Combined shrub stratum (TotalB)"},
	{6, "6", "Herb", "Cover6", "Height6", "Herbaceous layer (C)"},
	{7, "7", "Moss", "Cover7", "", "Terrestrial moss/bryoid layer (D/Dh)"},
	{8, "8", "Moss", "Cover8", "", "Epixylic bryoid layer on decaying wood (Dw)"},
	{9, "9", "Moss", "Cover9", "", "Epilithic bryoid layer on rock (Dr)"},
	{10, "10", "Epiphyte", "Cover10", "", "Epiphyte layer on trees/shrubs (Ep)"},
}

var vproSublayerCodeToTV = map[string]turboVegLayerCrosswalk{
	"A1": turboVegCrosswalk[0],
	"A2": turboVegCrosswalk[1],
	"A3": turboVegCrosswalk[2],
	"A":  turboVegCrosswalk[3],
	"B1": turboVegCrosswalk[4],
	"B2": turboVegCrosswalk[5],
	"B2a": turboVegCrosswalk[6],
	"B2b": turboVegCrosswalk[7],
	"B2c": turboVegCrosswalk[8],
	"B":  turboVegCrosswalk[9],
	"C":  turboVegCrosswalk[10],
	"D":  turboVegCrosswalk[11],
	"Dw": turboVegCrosswalk[12],
	"Dr": turboVegCrosswalk[13],
	"Ep": turboVegCrosswalk[14],
}

func resolveIncomingTurboVegLayer(layerCode, tvGroup string) (coverCol, heightCol string) {
	lc := strings.TrimSpace(layerCode)
	switch strings.ToUpper(lc) {
	case "1", "A1":
		return "Cover1", "Height1"
	case "2", "A2":
		return "Cover2", "Height2"
	case "3", "A3":
		return "Cover3", "Height3"
	case "TOTALA", "A":
		return "TotalA", "HeightA"
	case "4", "B1":
		return "Cover4", "Height4"
	case "5", "B2":
		return "Cover5", "Height5"
	case "5A", "B2A":
		return "Cover5a", "Height5a"
	case "5B", "B2B":
		return "Cover5b", "Height5b"
	case "5C", "B2C":
		return "Cover5c", "Height5c"
	case "TOTALB", "B":
		return "TotalB", "HeightB"
	case "6", "C", "HERB":
		return "Cover6", "Height6"
	case "7", "D", "DH", "MOSS", "CRYPTOGAM":
		return "Cover7", ""
	case "8", "DW":
		return "Cover8", ""
	case "9", "DR":
		return "Cover9", ""
	case "10", "EP", "EPIPHYTE":
		return "Cover10", ""
	}

	switch strings.ToUpper(strings.TrimSpace(tvGroup)) {
	case "TREE":
		return "TotalA", "HeightA"
	case "SHRUB":
		return "TotalB", "HeightB"
	case "HERB":
		return "Cover6", "Height6"
	case "MOSS", "CRYPTOGAM":
		return "Cover7", ""
	case "EPIPHYTE":
		return "Cover10", ""
	default:
		return "Cover6", "Height6"
	}
}

type TurboVegExportRequest struct {
	Format          string `json:"format"` // "xml" | "sqlite"
	DestinationPath string `json:"destinationPath"`
	LumpTable       string `json:"lumpTable,omitempty"`
	LumpDBPath      string `json:"lumpDbPath,omitempty"`
}

type TurboVegExportOutcome struct {
	Format           string `json:"format"`
	Path             string `json:"path"`
	SHA256           string `json:"sha256"`
	PlotCount        int    `json:"plotCount"`
	SpeciesCount     int    `json:"speciesCount"`
	ObservationCount int    `json:"observationCount"`
}

type TurboVegImportRequest struct {
	SourcePath        string `json:"sourcePath"`
	Format            string `json:"format,omitempty"` // "xml" | "sqlite" (inferred from extension if omitted)
	TargetProject     string `json:"targetProject"`
	DestinationPath   string `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

type TurboVegImportOutcome struct {
	SourcePath       string                 `json:"sourcePath"`
	Format           string                 `json:"format"`
	TargetProject    string                 `json:"targetProject"`
	DestinationPath  string                 `json:"destinationPath"`
	DestinationSHA   string                 `json:"destinationSha256"`
	PlotCount        int                    `json:"plotCount"`
	SpeciesCount     int                    `json:"speciesCount"`
	ObservationCount int                    `json:"observationCount"`
	Activated        bool                   `json:"activated"`
	State            *vprocore.ProjectState `json:"state,omitempty"`
}

type tvPlotHeader struct {
	PlotID            int
	PlotNumber        string
	FieldNumber       string
	ProjectID         string
	Date              string
	Latitude          *float64
	Longitude         *float64
	Elevation         *int
	Slope             *float64
	Aspect            *int
	PlotSize          *float64
	Zone              string
	SubZone           string
	SiteSeries        string
	SiteUnit          string
	MoistureRegime    string
	NutrientRegime    string
	MesoSlopePosition string
	SurfaceShape      string
	SoilDrainage      string
	HumusForm         string
	Surveyor          string
	Location          string
	SiteNotes         string
	VegNotes          string
	SoilNotes         string
}

type tvSpeciesObs struct {
	SpeciesNR   int
	SpeciesCode string
	SciName     string
	LayerCode   string
	TVGroup     string
	Cover       float64
	Height      *float64
	Collected   string
}

type tvDataset struct {
	Layers  []turboVegLayerCrosswalk
	Species []tvSpeciesItem
	Plots   []tvPlotBundle
}

type tvSpeciesItem struct {
	SpeciesNR      int
	SpeciesCode    string
	ScientificName string
	EnglishName    string
	Lifeform       *int
	Authority      string
}

type tvPlotBundle struct {
	Header       tvPlotHeader
	Observations []tvSpeciesObs
}

// XML Wire Structs for TurboVeg XML (<Turboveg>)
type xmlTurbovegDoc struct {
	XMLName     xml.Name          `xml:"Turboveg"`
	Version     string            `xml:"version,attr"`
	Generator   string            `xml:"generator,attr"`
	Layers      []xmlTVLayer      `xml:"Layers>Layer"`
	SpeciesList []xmlTVSpeciesDef `xml:"SpeciesList>Species"`
	Plots       []xmlTVPlot       `xml:"Plots>Plot"`
}

type xmlTVLayer struct {
	ID          int    `xml:"id,attr"`
	Code        string `xml:"code,attr"`
	TVGroup     string `xml:"tv_group,attr"`
	VProColumn  string `xml:"vpro_column,attr,omitempty"`
	Description string `xml:"description,attr,omitempty"`
}

type xmlTVSpeciesDef struct {
	NR             int    `xml:"nr,attr"`
	Code           string `xml:"code,attr"`
	ScientificName string `xml:"name,attr,omitempty"`
	EnglishName    string `xml:"english_name,attr,omitempty"`
	Lifeform       *int   `xml:"lifeform,attr,omitempty"`
	Authority      string `xml:"authority,attr,omitempty"`
}

type xmlTVPlot struct {
	ReleveNR   int           `xml:"releve_nr,attr"`
	PlotNumber string        `xml:"plot_number,attr"`
	Header     xmlTVHeader   `xml:"Header"`
	Species    []xmlTVObs    `xml:"SpeciesData>Species"`
}

type xmlTVHeader struct {
	FieldNumber       string   `xml:"field_number,attr,omitempty"`
	ProjectID         string   `xml:"project_id,attr,omitempty"`
	Date              string   `xml:"date,attr,omitempty"`
	Latitude          *float64 `xml:"latitude,attr,omitempty"`
	Longitude         *float64 `xml:"longitude,attr,omitempty"`
	Elevation         *int     `xml:"elevation,attr,omitempty"`
	Slope             *float64 `xml:"slope,attr,omitempty"`
	Aspect            *int     `xml:"aspect,attr,omitempty"`
	PlotSize          *float64 `xml:"plot_size,attr,omitempty"`
	Zone              string   `xml:"zone,attr,omitempty"`
	SubZone           string   `xml:"subzone,attr,omitempty"`
	SiteSeries        string   `xml:"site_series,attr,omitempty"`
	SiteUnit          string   `xml:"site_unit,attr,omitempty"`
	MoistureRegime    string   `xml:"moisture_regime,attr,omitempty"`
	NutrientRegime    string   `xml:"nutrient_regime,attr,omitempty"`
	MesoSlopePosition string   `xml:"meso_slope_position,attr,omitempty"`
	SurfaceShape      string   `xml:"surface_shape,attr,omitempty"`
	SoilDrainage      string   `xml:"soil_drainage,attr,omitempty"`
	HumusForm         string   `xml:"humus_form,attr,omitempty"`
	Surveyor          string   `xml:"surveyor,attr,omitempty"`
	Location          string   `xml:"location,attr,omitempty"`
	SiteNotes         string   `xml:"SiteNotes,omitempty"`
	VegNotes          string   `xml:"VegNotes,omitempty"`
	SoilNotes         string   `xml:"SoilNotes,omitempty"`
}

type xmlTVObs struct {
	NR        int      `xml:"nr,attr"`
	Code      string   `xml:"code,attr"`
	Name      string   `xml:"name,attr,omitempty"`
	Layer     string   `xml:"layer,attr"`
	TVGroup   string   `xml:"tv_group,attr,omitempty"`
	Cover     float64  `xml:"cover,attr"`
	Height    *float64 `xml:"height,attr,omitempty"`
	Collected string   `xml:"collected,attr,omitempty"`
}

func extractTVDatasetFromContext(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, lumpMap map[string]string) (tvDataset, int, error) {
	_, suByPlot, err := loadScopedPlotSiteUnits(ctx, tx, owner)
	if err != nil {
		return tvDataset{}, 0, err
	}
	sppMeta, err := loadSpeciesLookupMeta(ctx, tx)
	if err != nil {
		return tvDataset{}, 0, err
	}
	obs, err := extractLongVegObservations(ctx, tx, owner, suByPlot, sppMeta, lumpMap)
	if err != nil {
		return tvDataset{}, 0, err
	}

	hRows, err := tx.QueryContext(ctx, `
		SELECT
			PlotNumber,
			COALESCE(FieldNumber,''),
			COALESCE(ProjectID,''),
			COALESCE(CAST("Date" AS TEXT),''),
			Latitude,
			Longitude,
			Elevation,
			SlopeGradient,
			Aspect,
			PlotSize,
			COALESCE("Zone",''),
			COALESCE(SubZone,''),
			COALESCE(SiteSeries,''),
			COALESCE(MoistureRegime,''),
			COALESCE(NutrientRegime,''),
			COALESCE(MesoSlopePosition,''),
			COALESCE(SurfaceShape,''),
			COALESCE(SoilDrainage,''),
			COALESCE(HumusForm,''),
			COALESCE(SiteSurveyor,''),
			COALESCE(Location,''),
			COALESCE(SiteNotes,''),
			COALESCE(VegNotes,''),
			COALESCE(SoilNotes,'')
		FROM USysEnv
		ORDER BY PlotNumber`)
	if err != nil {
		return tvDataset{}, 0, fmt.Errorf("read USysEnv for TurboVeg export: %w", err)
	}
	defer hRows.Close()

	var headers []tvPlotHeader
	plotIdxByNum := map[string]int{}
	for hRows.Next() {
		var h tvPlotHeader
		var lat, lon, slope, pSize sql.NullFloat64
		var elev, aspect sql.NullInt64
		if err := hRows.Scan(
			&h.PlotNumber, &h.FieldNumber, &h.ProjectID, &h.Date,
			&lat, &lon, &elev, &slope, &aspect, &pSize,
			&h.Zone, &h.SubZone, &h.SiteSeries, &h.MoistureRegime, &h.NutrientRegime,
			&h.MesoSlopePosition, &h.SurfaceShape, &h.SoilDrainage, &h.HumusForm,
			&h.Surveyor, &h.Location, &h.SiteNotes, &h.VegNotes, &h.SoilNotes,
		); err != nil {
			return tvDataset{}, 0, err
		}
		if lat.Valid {
			v := lat.Float64
			h.Latitude = &v
		}
		if lon.Valid {
			v := lon.Float64
			h.Longitude = &v
		}
		if elev.Valid {
			v := int(elev.Int64)
			h.Elevation = &v
		}
		if slope.Valid {
			v := slope.Float64
			h.Slope = &v
		}
		if aspect.Valid {
			v := int(aspect.Int64)
			h.Aspect = &v
		}
		if pSize.Valid {
			v := pSize.Float64
			h.PlotSize = &v
		}
		h.SiteUnit = suByPlot[h.PlotNumber]
		h.PlotID = len(headers) + 1
		plotIdxByNum[h.PlotNumber] = len(headers)
		headers = append(headers, h)
	}
	if err := hRows.Err(); err != nil {
		return tvDataset{}, 0, err
	}

	// Build SpeciesList dictionary
	distinctCodes := map[string]struct{}{}
	for _, o := range obs {
		distinctCodes[o.Species] = struct{}{}
	}
	sortedCodes := make([]string, 0, len(distinctCodes))
	for c := range distinctCodes {
		sortedCodes = append(sortedCodes, c)
	}
	slices.Sort(sortedCodes)

	sppNRByCode := make(map[string]int, len(sortedCodes))
	sppDefs := make([]tvSpeciesItem, len(sortedCodes))
	for i, code := range sortedCodes {
		nr := i + 1
		sppNRByCode[code] = nr
		m := sppMeta[code]
		sppDefs[i] = tvSpeciesItem{
			SpeciesNR:      nr,
			SpeciesCode:    code,
			ScientificName: m.ScientificName,
			EnglishName:    m.EnglishName,
			Lifeform:       m.Lifeform,
			Authority:      m.Authority,
		}
	}

	bundles := make([]tvPlotBundle, len(headers))
	for i, h := range headers {
		bundles[i] = tvPlotBundle{Header: h}
	}

	totalObs := 0
	for _, o := range obs {
		pIdx, ok := plotIdxByNum[o.PlotNumber]
		if !ok {
			continue
		}
		cw, ok := vproSublayerCodeToTV[o.Layer]
		layerCode := o.Layer
		tvGrp := o.TVGroup
		if ok {
			layerCode = cw.LayerCode
			tvGrp = cw.TVGroup
		}
		bundles[pIdx].Observations = append(bundles[pIdx].Observations, tvSpeciesObs{
			SpeciesNR:   sppNRByCode[o.Species],
			SpeciesCode: o.Species,
			SciName:     o.ScientificName,
			LayerCode:   layerCode,
			TVGroup:     tvGrp,
			Cover:       o.Cover,
			Height:      o.Height,
			Collected:   o.Collected,
		})
		totalObs++
	}

	return tvDataset{
		Layers:  turboVegCrosswalk,
		Species: sppDefs,
		Plots:   bundles,
	}, totalObs, nil
}

func ExportTurboVeg(s *vprocore.ContextService, ctx context.Context, contextID string, request TurboVegExportRequest) (TurboVegExportOutcome, error) {
	if strings.TrimSpace(request.DestinationPath) == "" {
		return TurboVegExportOutcome{}, errors.New("TurboVeg export requires a non-empty destinationPath")
	}
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		if strings.HasSuffix(strings.ToLower(request.DestinationPath), ".xml") {
			format = TurboVegFormatXML
		} else {
			format = TurboVegFormatSQLite
		}
	}
	if format != TurboVegFormatXML && format != TurboVegFormatSQLite {
		return TurboVegExportOutcome{}, fmt.Errorf("unsupported TurboVeg export format %q (expected xml or sqlite)", request.Format)
	}

	dest := strings.TrimSpace(request.DestinationPath)
	if format == TurboVegFormatXML && !strings.HasSuffix(strings.ToLower(dest), ".xml") {
		dest += ".xml"
	} else if format == TurboVegFormatSQLite && !strings.HasSuffix(strings.ToLower(dest), ".sqlite") && !strings.HasSuffix(strings.ToLower(dest), ".db") {
		dest += ".sqlite"
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (TurboVegExportOutcome, error) {
		owner := plots.Projects.Sqlite
		lumpMap, err := loadOptionalLumpMap(ctx, owner.Selection.ProjectPath, request.LumpTable, request.LumpDBPath)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return TurboVegExportOutcome{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}
		defer func() { _ = tx.Rollback() }()

		ds, obsCount, err := extractTVDatasetFromContext(ctx, tx, owner, lumpMap)
		if err != nil {
			return TurboVegExportOutcome{}, err
		}
		if err := tx.Commit(); err != nil {
			return TurboVegExportOutcome{}, err
		}

		var writtenPath, shaHex string
		if format == TurboVegFormatXML {
			xmlBytes, encErr := encodeTurboVegXML(ctx, ds)
			if encErr != nil {
				return TurboVegExportOutcome{}, encErr
			}
			writtenPath, shaHex, err = writeAtomicExportFile(dest, xmlBytes)
			if err != nil {
				return TurboVegExportOutcome{}, err
			}
		} else {
			writtenPath, shaHex, err = writeTurboVeg3SQLite(ctx, dest, ds)
			if err != nil {
				return TurboVegExportOutcome{}, err
			}
		}

		return TurboVegExportOutcome{
			Format:           format,
			Path:             writtenPath,
			SHA256:           shaHex,
			PlotCount:        len(ds.Plots),
			SpeciesCount:     len(ds.Species),
			ObservationCount: obsCount,
		}, nil
	})
}

func encodeTurboVegXML(ctx context.Context, ds tvDataset) ([]byte, error) {
	doc := xmlTurbovegDoc{
		Version:     "3.0",
		Generator:   "VPro 2.0 (B.C. Ministry of Forests BEC Program)",
		Layers:      make([]xmlTVLayer, len(ds.Layers)),
		SpeciesList: make([]xmlTVSpeciesDef, len(ds.Species)),
		Plots:       make([]xmlTVPlot, len(ds.Plots)),
	}
	for i, l := range ds.Layers {
		doc.Layers[i] = xmlTVLayer{
			ID:          l.LayerID,
			Code:        l.LayerCode,
			TVGroup:     l.TVGroup,
			VProColumn:  l.VProCover,
			Description: l.Description,
		}
	}
	for i, sp := range ds.Species {
		doc.SpeciesList[i] = xmlTVSpeciesDef{
			NR:             sp.SpeciesNR,
			Code:           sp.SpeciesCode,
			ScientificName: sp.ScientificName,
			EnglishName:    sp.EnglishName,
			Lifeform:       sp.Lifeform,
			Authority:      sp.Authority,
		}
	}
	for i, pb := range ds.Plots {
		if i%128 == 0 {
			runtime.Gosched()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		h := pb.Header
		xp := xmlTVPlot{
			ReleveNR:   h.PlotID,
			PlotNumber: h.PlotNumber,
			Header: xmlTVHeader{
				FieldNumber:       h.FieldNumber,
				ProjectID:         h.ProjectID,
				Date:              h.Date,
				Latitude:          h.Latitude,
				Longitude:         h.Longitude,
				Elevation:         h.Elevation,
				Slope:             h.Slope,
				Aspect:            h.Aspect,
				PlotSize:          h.PlotSize,
				Zone:              h.Zone,
				SubZone:           h.SubZone,
				SiteSeries:        h.SiteSeries,
				SiteUnit:          h.SiteUnit,
				MoistureRegime:    h.MoistureRegime,
				NutrientRegime:    h.NutrientRegime,
				MesoSlopePosition: h.MesoSlopePosition,
				SurfaceShape:      h.SurfaceShape,
				SoilDrainage:      h.SoilDrainage,
				HumusForm:         h.HumusForm,
				Surveyor:          h.Surveyor,
				Location:          h.Location,
				SiteNotes:         h.SiteNotes,
				VegNotes:          h.VegNotes,
				SoilNotes:         h.SoilNotes,
			},
			Species: make([]xmlTVObs, len(pb.Observations)),
		}
		for j, o := range pb.Observations {
			xp.Species[j] = xmlTVObs{
				NR:        o.SpeciesNR,
				Code:      o.SpeciesCode,
				Name:      o.SciName,
				Layer:     o.LayerCode,
				TVGroup:   o.TVGroup,
				Cover:     o.Cover,
				Height:    o.Height,
				Collected: o.Collected,
			}
		}
		doc.Plots[i] = xp
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeTurboVeg3SQLite(ctx context.Context, destinationPath string, ds tvDataset) (string, string, error) {
	absDest, err := filepath.Abs(filepath.Clean(destinationPath))
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o755); err != nil {
		return "", "", err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(absDest), ".vpro-tv3-*.sqlite")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmpFile.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()
	_ = tmpFile.Close()

	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_foreign_keys=on")
	if err != nil {
		return "", "", err
	}
	db.SetMaxOpenConns(1)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		db.Close()
		return "", "", err
	}

	build := func() error {
		if _, err := tx.ExecContext(ctx, `
			CREATE TABLE Plots (
				PlotID INTEGER PRIMARY KEY,
				PlotNumber TEXT NOT NULL UNIQUE,
				FieldNumber TEXT,
				ProjectID TEXT,
				Date TEXT,
				Latitude DOUBLE,
				Longitude DOUBLE,
				Elevation INTEGER,
				Slope DOUBLE,
				Aspect INTEGER,
				PlotSize DOUBLE
			);
			CREATE TABLE HeaderData (
				PlotID INTEGER PRIMARY KEY REFERENCES Plots(PlotID) ON DELETE CASCADE,
				Zone TEXT,
				SubZone TEXT,
				SiteSeries TEXT,
				SiteUnit TEXT,
				MoistureRegime TEXT,
				NutrientRegime TEXT,
				MesoSlopePosition TEXT,
				SurfaceShape TEXT,
				SoilDrainage TEXT,
				HumusForm TEXT,
				Surveyor TEXT,
				Location TEXT,
				SiteNotes TEXT,
				VegNotes TEXT,
				SoilNotes TEXT,
				ExtraJSON TEXT
			);
			CREATE TABLE Layers (
				LayerID INTEGER PRIMARY KEY,
				LayerCode TEXT NOT NULL UNIQUE,
				TurboVegGroup TEXT NOT NULL,
				VProColumn TEXT,
				HeightColumn TEXT,
				Description TEXT
			);
			CREATE TABLE SpeciesList (
				SpeciesNR INTEGER PRIMARY KEY,
				SpeciesCode TEXT NOT NULL UNIQUE,
				ScientificName TEXT,
				EnglishName TEXT,
				Lifeform INTEGER,
				Authority TEXT
			);
			CREATE TABLE SpeciesData (
				ID INTEGER PRIMARY KEY AUTOINCREMENT,
				PlotID INTEGER NOT NULL REFERENCES Plots(PlotID) ON DELETE CASCADE,
				SpeciesNR INTEGER NOT NULL REFERENCES SpeciesList(SpeciesNR),
				SpeciesCode TEXT NOT NULL,
				LayerCode TEXT NOT NULL,
				TurboVegGroup TEXT NOT NULL,
				Cover DOUBLE NOT NULL,
				Height DOUBLE,
				Collected TEXT
			);
			CREATE INDEX idx_tv3_speciesdata_plot ON SpeciesData(PlotID);
			CREATE INDEX idx_tv3_speciesdata_spp ON SpeciesData(SpeciesCode);
		`); err != nil {
			return err
		}

		lStmt, err := tx.PrepareContext(ctx, `INSERT INTO Layers (LayerID, LayerCode, TurboVegGroup, VProColumn, HeightColumn, Description) VALUES (?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, l := range ds.Layers {
			if _, err := lStmt.ExecContext(ctx, l.LayerID, l.LayerCode, l.TVGroup, l.VProCover, l.VProHeight, l.Description); err != nil {
				lStmt.Close()
				return err
			}
		}
		lStmt.Close()

		spStmt, err := tx.PrepareContext(ctx, `INSERT INTO SpeciesList (SpeciesNR, SpeciesCode, ScientificName, EnglishName, Lifeform, Authority) VALUES (?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, sp := range ds.Species {
			var lfAny any
			if sp.Lifeform != nil {
				lfAny = *sp.Lifeform
			}
			if _, err := spStmt.ExecContext(ctx, sp.SpeciesNR, sp.SpeciesCode, sp.ScientificName, sp.EnglishName, lfAny, sp.Authority); err != nil {
				spStmt.Close()
				return err
			}
		}
		spStmt.Close()

		pStmt, err := tx.PrepareContext(ctx, `INSERT INTO Plots (PlotID, PlotNumber, FieldNumber, ProjectID, Date, Latitude, Longitude, Elevation, Slope, Aspect, PlotSize) VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer pStmt.Close()

		hStmt, err := tx.PrepareContext(ctx, `INSERT INTO HeaderData (PlotID, Zone, SubZone, SiteSeries, SiteUnit, MoistureRegime, NutrientRegime, MesoSlopePosition, SurfaceShape, SoilDrainage, HumusForm, Surveyor, Location, SiteNotes, VegNotes, SoilNotes, ExtraJSON) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer hStmt.Close()

		obsStmt, err := tx.PrepareContext(ctx, `INSERT INTO SpeciesData (PlotID, SpeciesNR, SpeciesCode, LayerCode, TurboVegGroup, Cover, Height, Collected) VALUES (?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer obsStmt.Close()

		for _, pb := range ds.Plots {
			h := pb.Header
			if _, err := pStmt.ExecContext(ctx, h.PlotID, h.PlotNumber, nullEmpty(h.FieldNumber), nullEmpty(h.ProjectID), nullEmpty(h.Date),
				h.Latitude, h.Longitude, h.Elevation, h.Slope, h.Aspect, h.PlotSize); err != nil {
				return err
			}
			extra, _ := json.Marshal(map[string]string{"generator": "VPro 2.0"})
			if _, err := hStmt.ExecContext(ctx, h.PlotID, nullEmpty(h.Zone), nullEmpty(h.SubZone), nullEmpty(h.SiteSeries), nullEmpty(h.SiteUnit),
				nullEmpty(h.MoistureRegime), nullEmpty(h.NutrientRegime), nullEmpty(h.MesoSlopePosition), nullEmpty(h.SurfaceShape),
				nullEmpty(h.SoilDrainage), nullEmpty(h.HumusForm), nullEmpty(h.Surveyor), nullEmpty(h.Location),
				nullEmpty(h.SiteNotes), nullEmpty(h.VegNotes), nullEmpty(h.SoilNotes), string(extra)); err != nil {
				return err
			}
			for _, o := range pb.Observations {
				if _, err := obsStmt.ExecContext(ctx, h.PlotID, o.SpeciesNR, o.SpeciesCode, o.LayerCode, o.TVGroup, o.Cover, o.Height, nullEmpty(o.Collected)); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := build(); err != nil {
		_ = tx.Rollback()
		_ = db.Close()
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		_ = db.Close()
		return "", "", err
	}
	if err := db.Close(); err != nil {
		return "", "", err
	}
	if err := os.Rename(tmpPath, absDest); err != nil {
		return "", "", err
	}
	published = true

	rawBytes, err := os.ReadFile(absDest)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(rawBytes)
	return absDest, hex.EncodeToString(sum[:]), nil
}

func nullEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func ImportTurboVeg(s *vprocore.ContextService, ctx context.Context, contextID string, request TurboVegImportRequest) (TurboVegImportOutcome, error) {
	src := strings.TrimSpace(request.SourcePath)
	if src == "" || !utf8.ValidString(src) || strings.ContainsRune(src, 0) {
		return TurboVegImportOutcome{}, errors.New("TurboVeg import requires a valid sourcePath")
	}
	absSrc, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return TurboVegImportOutcome{}, err
	}
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		if strings.EqualFold(filepath.Ext(absSrc), ".xml") {
			format = TurboVegFormatXML
		} else {
			format = TurboVegFormatSQLite
		}
	}

	var ds tvDataset
	switch format {
	case TurboVegFormatXML:
		ds, err = parseTurboVegXMLFile(ctx, absSrc)
	case TurboVegFormatSQLite:
		ds, err = readTurboVeg3SQLiteFile(ctx, absSrc)
	default:
		return TurboVegImportOutcome{}, fmt.Errorf("unsupported TurboVeg import format %q", format)
	}
	if err != nil {
		return TurboVegImportOutcome{}, err
	}
	if len(ds.Plots) == 0 {
		return TurboVegImportOutcome{}, errors.New("TurboVeg source contains zero plots")
	}

	sppSet := map[string]struct{}{}
	obsCount := 0
	for _, pb := range ds.Plots {
		for _, o := range pb.Observations {
			sppSet[strings.ToUpper(strings.TrimSpace(o.SpeciesCode))] = struct{}{}
			obsCount++
		}
	}

	destPath, destSHA, activated, newState, err := stageAndPublishVP08Project(
		s, ctx, contextID, request.TargetProject, request.DestinationPath, request.ActivateOnSuccess,
		func(ctx context.Context, tx *sql.Tx, project string) error {
			return populateVP08FromTVDataset(ctx, tx, project, ds)
		},
	)
	if err != nil {
		return TurboVegImportOutcome{}, err
	}

	return TurboVegImportOutcome{
		SourcePath:       absSrc,
		Format:           format,
		TargetProject:    strings.TrimSpace(request.TargetProject),
		DestinationPath:  destPath,
		DestinationSHA:   destSHA,
		PlotCount:        len(ds.Plots),
		SpeciesCount:     len(sppSet),
		ObservationCount: obsCount,
		Activated:        activated,
		State:            newState,
	}, nil
}

func parseTurboVegXMLFile(ctx context.Context, path string) (tvDataset, error) {
	f, err := os.Open(path)
	if err != nil {
		return tvDataset{}, err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	var ds tvDataset
	sppNameByNR := map[int]string{}
	plotCounter := 0

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return tvDataset{}, fmt.Errorf("parse TurboVeg XML: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(se.Name.Local) {
		case "specieslist":
			var list struct {
				Species []xmlTVSpeciesDef `xml:"Species"`
			}
			if err := dec.DecodeElement(&list, &se); err != nil {
				return tvDataset{}, err
			}
			for _, sp := range list.Species {
				code := strings.ToUpper(strings.TrimSpace(sp.Code))
				if code == "" && sp.ScientificName != "" {
					code = abbreviateSpeciesName(sp.ScientificName)
				}
				sppNameByNR[sp.NR] = code
				ds.Species = append(ds.Species, tvSpeciesItem{
					SpeciesNR:      sp.NR,
					SpeciesCode:    code,
					ScientificName: sp.ScientificName,
					EnglishName:    sp.EnglishName,
					Lifeform:       sp.Lifeform,
					Authority:      sp.Authority,
				})
			}
		case "plot", "releve":
			plotCounter++
			if plotCounter%64 == 0 {
				runtime.Gosched()
				if err := ctx.Err(); err != nil {
					return tvDataset{}, err
				}
			}
			var xp xmlTVPlot
			if err := dec.DecodeElement(&xp, &se); err != nil {
				return tvDataset{}, err
			}
			pNum := strings.TrimSpace(xp.PlotNumber)
			if pNum == "" {
				pNum = strconv.Itoa(xp.ReleveNR)
			}
			if len([]rune(pNum)) > 7 {
				pNum = string([]rune(pNum)[:7])
			}
			pb := tvPlotBundle{
				Header: tvPlotHeader{
					PlotID:            plotCounter,
					PlotNumber:        pNum,
					FieldNumber:       xp.Header.FieldNumber,
					ProjectID:         xp.Header.ProjectID,
					Date:              xp.Header.Date,
					Latitude:          xp.Header.Latitude,
					Longitude:         xp.Header.Longitude,
					Elevation:         xp.Header.Elevation,
					Slope:             xp.Header.Slope,
					Aspect:            xp.Header.Aspect,
					PlotSize:          xp.Header.PlotSize,
					Zone:              xp.Header.Zone,
					SubZone:           xp.Header.SubZone,
					SiteSeries:        xp.Header.SiteSeries,
					SiteUnit:          xp.Header.SiteUnit,
					MoistureRegime:    xp.Header.MoistureRegime,
					NutrientRegime:    xp.Header.NutrientRegime,
					MesoSlopePosition: xp.Header.MesoSlopePosition,
					SurfaceShape:      xp.Header.SurfaceShape,
					SoilDrainage:      xp.Header.SoilDrainage,
					HumusForm:         xp.Header.HumusForm,
					Surveyor:          xp.Header.Surveyor,
					Location:          xp.Header.Location,
					SiteNotes:         xp.Header.SiteNotes,
					VegNotes:          xp.Header.VegNotes,
					SoilNotes:         xp.Header.SoilNotes,
				},
			}
			for _, o := range xp.Species {
				code := strings.ToUpper(strings.TrimSpace(o.Code))
				if code == "" {
					code = sppNameByNR[o.NR]
				}
				if code == "" && o.Name != "" {
					code = abbreviateSpeciesName(o.Name)
				}
				if code == "" || o.Cover <= 0 {
					continue
				}
				pb.Observations = append(pb.Observations, tvSpeciesObs{
					SpeciesNR:   o.NR,
					SpeciesCode: code,
					SciName:     o.Name,
					LayerCode:   o.Layer,
					TVGroup:     o.TVGroup,
					Cover:       o.Cover,
					Height:      o.Height,
					Collected:   o.Collected,
				})
			}
			ds.Plots = append(ds.Plots, pb)
		}
	}
	return ds, nil
}

func abbreviateSpeciesName(sci string) string {
	parts := strings.Fields(strings.ToUpper(sci))
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		if len(parts[0]) > 8 {
			return parts[0][:8]
		}
		return parts[0]
	}
	g := parts[0]
	if len(g) > 4 {
		g = g[:4]
	}
	sp := parts[1]
	if len(sp) > 3 {
		sp = sp[:3]
	}
	return g + sp
}

func readTurboVeg3SQLiteFile(ctx context.Context, path string) (tvDataset, error) {
	resolved, err := vprocore.ExistingDatabasePath(path)
	if err != nil {
		return tvDataset{}, err
	}
	db, err := sqlutil.OpenReadOnlyContext(ctx, resolved)
	if err != nil {
		return tvDataset{}, err
	}
	defer db.Close()

	var ds tvDataset
	spRows, err := db.QueryContext(ctx, `SELECT SpeciesNR, SpeciesCode, COALESCE(ScientificName,''), COALESCE(EnglishName,''), Lifeform, COALESCE(Authority,'') FROM SpeciesList ORDER BY SpeciesNR`)
	if err == nil {
		for spRows.Next() {
			var sp tvSpeciesItem
			var lf sql.NullInt64
			if sErr := spRows.Scan(&sp.SpeciesNR, &sp.SpeciesCode, &sp.ScientificName, &sp.EnglishName, &lf, &sp.Authority); sErr == nil {
				if lf.Valid {
					v := int(lf.Int64)
					sp.Lifeform = &v
				}
				ds.Species = append(ds.Species, sp)
			}
		}
		spRows.Close()
	}

	pRows, err := db.QueryContext(ctx, `
		SELECT
			p.PlotID, p.PlotNumber, COALESCE(p.FieldNumber,''), COALESCE(p.ProjectID,''), COALESCE(p.Date,''),
			p.Latitude, p.Longitude, p.Elevation, p.Slope, p.Aspect, p.PlotSize,
			COALESCE(h.Zone,''), COALESCE(h.SubZone,''), COALESCE(h.SiteSeries,''), COALESCE(h.SiteUnit,''),
			COALESCE(h.MoistureRegime,''), COALESCE(h.NutrientRegime,''), COALESCE(h.MesoSlopePosition,''),
			COALESCE(h.SurfaceShape,''), COALESCE(h.SoilDrainage,''), COALESCE(h.HumusForm,''),
			COALESCE(h.Surveyor,''), COALESCE(h.Location,''),
			COALESCE(h.SiteNotes,''), COALESCE(h.VegNotes,''), COALESCE(h.SoilNotes,'')
		FROM Plots AS p
		LEFT JOIN HeaderData AS h ON h.PlotID = p.PlotID
		ORDER BY p.PlotID`)
	if err != nil {
		return tvDataset{}, fmt.Errorf("read TurboVeg3 Plots/HeaderData: %w", err)
	}
	plotIdxByID := map[int]int{}
	for pRows.Next() {
		var h tvPlotHeader
		var lat, lon, slope, pSize sql.NullFloat64
		var elev, aspect sql.NullInt64
		if err := pRows.Scan(
			&h.PlotID, &h.PlotNumber, &h.FieldNumber, &h.ProjectID, &h.Date,
			&lat, &lon, &elev, &slope, &aspect, &pSize,
			&h.Zone, &h.SubZone, &h.SiteSeries, &h.SiteUnit,
			&h.MoistureRegime, &h.NutrientRegime, &h.MesoSlopePosition,
			&h.SurfaceShape, &h.SoilDrainage, &h.HumusForm,
			&h.Surveyor, &h.Location, &h.SiteNotes, &h.VegNotes, &h.SoilNotes,
		); err != nil {
			pRows.Close()
			return tvDataset{}, err
		}
		if lat.Valid {
			v := lat.Float64
			h.Latitude = &v
		}
		if lon.Valid {
			v := lon.Float64
			h.Longitude = &v
		}
		if elev.Valid {
			v := int(elev.Int64)
			h.Elevation = &v
		}
		if slope.Valid {
			v := slope.Float64
			h.Slope = &v
		}
		if aspect.Valid {
			v := int(aspect.Int64)
			h.Aspect = &v
		}
		if pSize.Valid {
			v := pSize.Float64
			h.PlotSize = &v
		}
		plotIdxByID[h.PlotID] = len(ds.Plots)
		ds.Plots = append(ds.Plots, tvPlotBundle{Header: h})
	}
	if err := pRows.Close(); err != nil {
		return tvDataset{}, err
	}

	oRows, err := db.QueryContext(ctx, `
		SELECT PlotID, SpeciesNR, SpeciesCode, LayerCode, TurboVegGroup, Cover, Height, COALESCE(Collected,'')
		FROM SpeciesData
		ORDER BY PlotID, ID`)
	if err != nil {
		return tvDataset{}, fmt.Errorf("read TurboVeg3 SpeciesData: %w", err)
	}
	defer oRows.Close()
	for oRows.Next() {
		var plotID int
		var o tvSpeciesObs
		var ht sql.NullFloat64
		if err := oRows.Scan(&plotID, &o.SpeciesNR, &o.SpeciesCode, &o.LayerCode, &o.TVGroup, &o.Cover, &ht, &o.Collected); err != nil {
			return tvDataset{}, err
		}
		if ht.Valid {
			v := ht.Float64
			o.Height = &v
		}
		if idx, ok := plotIdxByID[plotID]; ok {
			ds.Plots[idx].Observations = append(ds.Plots[idx].Observations, o)
		}
	}
	return ds, oRows.Err()
}

func populateVP08FromTVDataset(ctx context.Context, tx *sql.Tx, project string, ds tvDataset) error {
	envTbl := vprocore.QuoteHeaderIdentifier(project + "_Env")
	adminTbl := vprocore.QuoteHeaderIdentifier(project + "_Admin")
	vegTbl := vprocore.QuoteHeaderIdentifier(project + "_Veg")
	metaTbl := vprocore.QuoteHeaderIdentifier(project + "_Metadata")
	auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")

	envStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+envTbl+` (
			"PlotNumber","FieldNumber","ProjectID","Date","Latitude","Longitude",
			"Elevation","SlopeGradient","Aspect","Zone","SubZone","SiteSeries",
			"MoistureRegime","NutrientRegime","MesoSlopePosition","SurfaceShape",
			"SoilDrainage","HumusForm","SiteSurveyor","Location","SiteNotes","VegNotes","SoilNotes"
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer envStmt.Close()

	adminStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+adminTbl+` ("Plot","PlotType","PlotSize","UserSiteUnit") VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer adminStmt.Close()

	vegColumns := []string{
		"ID", "PlotNumber", "Species", "Collected",
		"Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3", "TotalA", "HeightA",
		"Cover4", "Height4", "Cover5", "Height5", "Cover5a", "Height5a", "Cover5b", "Height5b", "Cover5c", "Height5c", "TotalB", "HeightB",
		"Cover6", "Height6", "Cover7", "Cover8", "Cover9", "Cover10",
	}
	qVegCols := make([]string, len(vegColumns))
	qMarks := make([]string, len(vegColumns))
	colPos := map[string]int{}
	for i, c := range vegColumns {
		qVegCols[i] = vprocore.QuoteHeaderIdentifier(c)
		qMarks[i] = "?"
		colPos[c] = i
	}
	vegStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+vegTbl+` (`+strings.Join(qVegCols, ", ")+`) VALUES (`+strings.Join(qMarks, ", ")+`)`)
	if err != nil {
		return err
	}
	defer vegStmt.Close()

	var nextVegID int64 = 1
	suRows := [][2]string{}
	seenPlots := map[string]bool{}
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for _, pb := range ds.Plots {
		h := pb.Header
		pNum := strings.TrimSpace(h.PlotNumber)
		if pNum == "" || seenPlots[pNum] {
			continue
		}
		seenPlots[pNum] = true

		projID := h.ProjectID
		if strings.TrimSpace(projID) == "" {
			projID = project
		}
		if len([]rune(projID)) > 20 {
			projID = string([]rune(projID)[:20])
		}

		if _, err := envStmt.ExecContext(ctx,
			pNum, nullEmpty(h.FieldNumber), projID, nullEmpty(h.Date), h.Latitude, h.Longitude,
			h.Elevation, h.Slope, h.Aspect, nullEmpty(h.Zone), nullEmpty(h.SubZone), nullEmpty(h.SiteSeries),
			nullEmpty(h.MoistureRegime), nullEmpty(h.NutrientRegime), nullEmpty(h.MesoSlopePosition), nullEmpty(h.SurfaceShape),
			nullEmpty(h.SoilDrainage), nullEmpty(h.HumusForm), nullEmpty(h.Surveyor), nullEmpty(h.Location),
			nullEmpty(h.SiteNotes), nullEmpty(h.VegNotes), nullEmpty(h.SoilNotes),
		); err != nil {
			return fmt.Errorf("insert plot %s into _Env: %w", pNum, err)
		}
		if _, err := adminStmt.ExecContext(ctx, pNum, "FS882", h.PlotSize, nullEmpty(h.SiteUnit)); err != nil {
			return fmt.Errorf("insert plot %s into _Admin: %w", pNum, err)
		}
		if strings.TrimSpace(h.SiteUnit) != "" {
			suRows = append(suRows, [2]string{pNum, strings.TrimSpace(h.SiteUnit)})
		}

		// Pivot SpeciesData per species into wide VP08 _Veg rows
		type wideRow struct {
			spp       string
			collected string
			covers    map[string]float64
			heights   map[string]float64
		}
		bySpp := map[string]*wideRow{}
		var sppOrder []string
		for _, o := range pb.Observations {
			code := strings.ToUpper(strings.TrimSpace(o.SpeciesCode))
			if code == "" || o.Cover <= 0 {
				continue
			}
			if len([]rune(code)) > 8 {
				code = string([]rune(code)[:8])
			}
			wr := bySpp[code]
			if wr == nil {
				wr = &wideRow{spp: code, covers: map[string]float64{}, heights: map[string]float64{}}
				bySpp[code] = wr
				sppOrder = append(sppOrder, code)
			}
			if wr.collected == "" && strings.TrimSpace(o.Collected) != "" {
				wr.collected = strings.TrimSpace(o.Collected)[:1]
			}
			cCol, hCol := resolveIncomingTurboVegLayer(o.LayerCode, o.TVGroup)
			sumCov := wr.covers[cCol] + o.Cover
			if float32(sumCov) >= 100 {
				sumCov = 99.0
			}
			wr.covers[cCol] = math.Round(sumCov*10000) / 10000
			if o.Height != nil && hCol != "" {
				if *o.Height > wr.heights[hCol] {
					wr.heights[hCol] = *o.Height
				}
			}
		}

		for _, code := range sppOrder {
			wr := bySpp[code]
			args := make([]any, len(vegColumns))
			args[colPos["ID"]] = nextVegID
			nextVegID++
			args[colPos["PlotNumber"]] = pNum
			args[colPos["Species"]] = wr.spp
			args[colPos["Collected"]] = nullEmpty(wr.collected)
			for cCol, cv := range wr.covers {
				if pos, ok := colPos[cCol]; ok {
					args[pos] = cv
				}
			}
			for hCol, hv := range wr.heights {
				if pos, ok := colPos[hCol]; ok {
					if hCol == "HeightB" {
						args[pos] = strconv.FormatFloat(hv, 'f', -1, 64)
					} else {
						args[pos] = hv
					}
				}
			}
			if _, err := vegStmt.ExecContext(ctx, args...); err != nil {
				return fmt.Errorf("insert veg %s/%s: %w", pNum, wr.spp, err)
			}
		}

		_, _ = tx.ExecContext(ctx, `
			INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen")
			VALUES (?, 'VPro', ?, 'Environment', 'Imported TurboVeg record', ?)`,
			project, pNum, nowStr)
	}

	if len(suRows) > 0 {
		suTbl := vprocore.QuoteHeaderIdentifier(project + "_SU")
		if _, err := tx.ExecContext(ctx, `
			CREATE TABLE `+suTbl+` ("PlotNumber" VARCHAR, "SiteUnit" VARCHAR);
			CREATE UNIQUE INDEX `+vprocore.QuoteHeaderIdentifier("uidx_"+project+"_SU_PlotNumber")+` ON `+suTbl+`("PlotNumber");
			CREATE INDEX `+vprocore.QuoteHeaderIdentifier("idx_"+project+"_SU_SiteUnit")+` ON `+suTbl+`("SiteUnit");
			INSERT INTO "_table_metadata" ("table_name", "description") VALUES (?, 'VP04');`, project+"_SU"); err != nil {
			return err
		}
		for _, pair := range suRows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+suTbl+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, pair[0], pair[1]); err != nil {
				return err
			}
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO `+metaTbl+` ("ID", "ProjectID", "ProjectTitle", "NumberOfFS882Plots", "VegCoverMethod", "Datum")
		VALUES (1, ?, ?, ?, '1', '2')`,
		project, project+" (TurboVeg Import)", len(seenPlots))
	return err
}
