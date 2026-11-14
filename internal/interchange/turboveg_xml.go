package interchange

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

func normalizeTVDateString(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "null") {
		return ""
	}
	if len(s) == 8 {
		allDigits := true
		for i := 0; i < 8; i++ {
			if s[i] < '0' || s[i] > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			y := s[0:4]
			m := s[4:6]
			d := s[6:8]
			if m == "00" {
				return y
			}
			if d == "00" {
				return y + "-" + m
			}
			return y + "-" + m + "-" + d
		}
	}
	if len(s) == 10 && s[4] == '/' && s[7] == '/' {
		return strings.ReplaceAll(s, "/", "-")
	}
	return s
}

func formatTVStandardDateAttr(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("20060102")
		}
	}
	if len(s) <= 8 {
		return s
	}
	return truncateUTF16Units(s, 8)
}

func defaultBraunBlanquetMidpoint(code string) (float64, bool) {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "r":
		return 0.1, true
	case "+":
		return 0.5, true
	case "x":
		return 1.0, true
	case "1":
		return 3.0, true
	case "2m":
		return 4.0, true
	case "2a":
		return 8.75, true
	case "2", "2b":
		return 18.75, true
	case "3":
		return 37.5, true
	case "4":
		return 62.5, true
	case "5":
		return 87.5, true
	default:
		return 0, false
	}
}

func turboVegXMLCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	norm := strings.ToLower(strings.TrimSpace(charset))
	norm = strings.ReplaceAll(norm, "_", "-")
	switch norm {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return input, nil
	case "windows-1252", "cp1252", "ansi":
		return charmap.Windows1252.NewDecoder().Reader(input), nil
	case "windows-1250", "cp1250":
		return charmap.Windows1250.NewDecoder().Reader(input), nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1.NewDecoder().Reader(input), nil
	case "iso-8859-15", "iso8859-15", "latin9":
		return charmap.ISO8859_15.NewDecoder().Reader(input), nil
	default:
		return nil, fmt.Errorf("unsupported TurboVeg XML charset %q", charset)
	}
}

// Wire structs for official Turboveg <Plot_package> Standard XML.

type xmlPlotPackageDoc struct {
	XMLName              xml.Name              `xml:"Plot_package"`
	UserName             string                `xml:"UserName,attr"`
	EmailAddress         string                `xml:"EmailAddress,attr"`
	TurbovegVersion      string                `xml:"TurbovegVersion,attr"`
	Dictionary           string                `xml:"Dictionary,attr"`
	SpeciesList          string                `xml:"SpeciesList,attr"`
	Description          string                `xml:"Description,attr"`
	ShareWithMembersOnly string                `xml:"ShareWithMembersOnly,attr"`
	CreationDate         string                `xml:"CreationDate,attr"`
	Plots                xmlPlotPackagePlots   `xml:"Plots"`
	LookupTables         xmlPlotPackageLookups `xml:"Lookup_tables"`
	Template             xmlPlotPackageTmpl    `xml:"Template"`
	Metadata             xmlPlotPackageMeta    `xml:"Metadata"`
}

type xmlPlotPackagePlots struct {
	Plot []xmlPkgPlot `xml:"Plot"`
}

type xmlPkgPlot struct {
	Database   string            `xml:"database,attr"`
	ReleveNR   int               `xml:"releve_nr,attr"`
	GUID       string            `xml:"guid,attr"`
	CreateUser string            `xml:"create_user,attr"`
	CreateDate string            `xml:"create_date,attr"`
	ModifyUser string            `xml:"modify_user,attr"`
	ModifyDate string            `xml:"modify_date,attr"`
	NDFFQual   string            `xml:"ndff_qual,attr"`
	HeaderData xmlPkgHeaderData  `xml:"header_data"`
	SpeciesDat xmlPkgSpeciesData `xml:"species_data"`
}

type xmlPkgHeaderData struct {
	Standard xmlPkgHeaderStd `xml:"standard_record"`
	UDFs     []xmlPkgUDF     `xml:"udf_record"`
}

type xmlPkgHeaderStd struct {
	ReleveNR   int    `xml:"releve_nr,attr"`
	Country    string `xml:"country,attr,omitempty"`
	Coverscale string `xml:"coverscale,attr"`
	Date       string `xml:"date,attr,omitempty"`
	SurfArea   string `xml:"surf_area,attr,omitempty"`
	Altitude   string `xml:"altitude,attr,omitempty"`
	Exposition string `xml:"exposition,attr,omitempty"`
	Inclinatio string `xml:"inclinatio,attr,omitempty"`
	Author     string `xml:"author,attr,omitempty"`
	Project    string `xml:"project,attr,omitempty"`
	Syntaxon   string `xml:"syntaxon,attr,omitempty"`
	Remarks    string `xml:"remarks,attr,omitempty"`
}

type xmlPkgUDF struct {
	IsPredefined string `xml:"ispredefined,attr"`
	Name         string `xml:"name,attr"`
	Type         string `xml:"type,attr"`
	Len          int    `xml:"len,attr"`
	Dec          int    `xml:"dec,attr"`
	Value        string `xml:"value,attr"`
}

type xmlPkgSpeciesData struct {
	Species []xmlPkgSpeciesEntry `xml:"species"`
}

type xmlPkgSpeciesEntry struct {
	Standard xmlPkgSpeciesStd `xml:"standard_record"`
	UDFs     []xmlPkgUDF      `xml:"udf_record"`
}

type xmlPkgSpeciesStd struct {
	NR    int    `xml:"nr,attr"`
	Cover string `xml:"cover,attr"`
	Layer int    `xml:"layer,attr"`
}

type xmlPlotPackageLookups struct {
	CoverscaleList []xmlCoverscaleRecord `xml:"Coverscale_list>coverscale_record"`
	CountryList    []xmlSimpleLookupRec  `xml:"Country_list>country_record"`
	AuthorList     []xmlSimpleLookupRec  `xml:"Author_list>author_record"`
	ProjectList    []xmlSimpleLookupRec  `xml:"Project_list>project_record"`
	SyntaxonList   []xmlSimpleLookupRec  `xml:"Syntaxon_list>syntaxon_record"`
	SpeciesList    []xmlSpeciesRecord    `xml:"Species_list>species_record"`
}

type xmlCoverscaleRecord struct {
	Code        string              `xml:"code"`
	Description string              `xml:"description"`
	DataRecords []xmlCoverscaleData `xml:"data_record"`
}

type xmlCoverscaleData struct {
	Code       string `xml:"code,attr"`
	Percentage string `xml:"percentage,attr"`
}

type xmlSimpleLookupRec struct {
	Code        string `xml:"code,attr"`
	Description string `xml:"description,attr"`
}

type xmlSpeciesRecord struct {
	NR          int    `xml:"nr,attr"`
	Code        string `xml:"code,attr"`
	Name        string `xml:"name,attr"`
	Author      string `xml:"author,attr"`
	ValidNR     string `xml:"valid_nr,attr"`
	ValidName   string `xml:"valid_name,attr"`
	ValidAuthor string `xml:"valid_author,attr"`
	Synonym     string `xml:"synonym,attr"`
	NativeName  string `xml:"nativename,attr"`
	VProCode    string `xml:"vpro_code,attr,omitempty"`
}

type xmlPlotPackageTmpl struct {
	Files    []xmlTemplateFileRec `xml:"Files>record"`
	Dbasedic []xmlDbasedicRec     `xml:"Dbasedic>record"`
}

type xmlTemplateFileRec struct {
	FileNR     int    `xml:"file_nr,attr"`
	FileName   string `xml:"file_name,attr"`
	FileDesc   string `xml:"file_desc,attr"`
	FieldIndex int    `xml:"fieldindex,attr"`
	FileEdit   string `xml:"file_edit,attr"`
	ArrayPos   int    `xml:"array_pos,attr"`
	ProgPath   string `xml:"progpath,attr"`
	PrintIt    string `xml:"print_it,attr"`
}

type xmlDbasedicRec struct {
	FileNR    int    `xml:"file_nr,attr"`
	FieldName string `xml:"field_name,attr"`
	FieldType string `xml:"field_type,attr"`
	FieldLen  int    `xml:"field_len,attr"`
	FieldDec  int    `xml:"field_dec,attr"`
	FieldPict string `xml:"field_pict,attr"`
	FieldTran string `xml:"field_tran,attr"`
	FieldEval string `xml:"field_eval,attr"`
	FieldEsel string `xml:"field_esel,attr"`
	FieldWhen string `xml:"field_when,attr"`
	FieldCarg string `xml:"field_carg,attr"`
	FieldDesc string `xml:"field_desc,attr"`
	FieldMenu string `xml:"field_menu,attr"`
	FieldSel  int    `xml:"field_sel,attr"`
	FieldList int    `xml:"field_list,attr"`
}

type xmlPlotPackageMeta struct {
	Records []string `xml:"record"`
}

var canonicalTemplateFiles = []xmlTemplateFileRec{
	{1, "tvflora", "FLORA", 0, "false", 0, "false", "false"},
	{2, "tvhabita", "HEADER DATA", 0, "false", 0, "false", "false"},
	{3, "tvabund", "SPECIES DATA", 0, "false", 0, "false", "false"},
	{4, "remarks", "REMARKS", 1, "false", 0, "false", "false"},
	{5, "tvrefenc", "REFERENCES", 1, "true", 0, "true", "false"},
	{6, "tvscale", "COVER SCALES", 1, "true", 1, "true", "true"},
	{7, "country", "COUNTRIES", 2, "true", 2, "true", "true"},
	{8, "tvauthor", "AUTHORS", 2, "true", 3, "true", "true"},
	{9, "tvprojct", "PROJECTS", 1, "true", 4, "true", "true"},
	{10, "syntaxa", "SYNTAXA", 1, "true", 5, "true", "true"},
	{11, "exposit", "ASPECT", 0, "true", 6, "true", "true"},
}

var canonicalDbasedicRecords = []xmlDbasedicRec{
	{1, "SPECIES_NR", "N", 5, 0, "@ 99999", "", "", "", "", "", "Species number code", "", 0, 0},
	{1, "LETTERCODE", "C", 7, 0, "", "", "", "", "", "", "Species letter code", "", 0, 0},
	{1, "SHORTNAME", "C", 22, 0, "", "", "", "", "", "", "Short name (correct)", "", 0, 0},
	{1, "ABBREVIAT", "C", 37, 0, "", "", "", "", "", "", "Species name", "", 0, 0},
	{1, "SYNONYM", "L", 1, 0, "", "", "", "", "", "", "", "", 0, 0},
	{2, "RELEVE_NR", "N", 6, 0, "@ 999999", "", "", "", "", "", "Releve number", "Releve number", 3, 0},
	{2, "COUNTRY", "C", 2, 0, "@ !!", "", "", "", "", "", "Country code", "Country code", 2, 2},
	{2, "REFERENCE", "C", 6, 0, "@ ######", "", "", "", "", "", "Biblio reference", "Biblioreferen.", 3, -1},
	{2, "COVERSCALE", "C", 2, 0, "@ ##", "", "", "", "", "", "Cover abundance scale", "Cover scale", 2, 1},
	{2, "PROJECT", "C", 3, 0, "@ ###", "", "", "", "", "", "Project code", "Project code", 2, 4},
	{2, "AUTHOR", "C", 4, 0, "@ ####", "", "", "", "", "", "Author code", "Author code", 2, 3},
	{2, "DATE", "C", 8, 0, "@R ####/##/##", "@R ####/##/##", "", "", "", "", "Date (year/month/day)", "Date", 3, 0},
	{2, "SYNTAXON", "C", 7, 0, "@ ##!!##X", "", "", "", "", "", "Syntaxon code", "Syntaxon code", 2, 5},
	{2, "SURF_AREA", "N", 8, 2, "@Z 99999.99", "", "", "", "", "", "Releve area (m2)", "Releve area", 1, 0},
	{2, "ALTITUDE", "C", 4, 0, "@ ####", "", "", "", "", "", "Altitude (m)", "Altitude", 1, 0},
	{2, "EXPOSITION", "C", 3, 0, "@ ###", "", "", "", "", "", "Aspect (degrees)", "Aspect", 1, 6},
	{2, "INCLINATIO", "C", 2, 0, "@ ##", "", "", "", "", "", "Slope (degrees)", "Slope", 1, 0},
	{2, "REMARKS", "C", 56, 0, "@S56", "", "", "", "", "", "Remarks", "Remarks", 4, 0},
	{3, "RELEVE_NR", "N", 6, 0, "", "", "", "", "", "", "", "", 0, 0},
	{3, "SPECIES_NR", "N", 5, 0, "", "", "", "", "", "", "", "", 0, 0},
	{3, "COVER_CODE", "C", 2, 0, "", "", "", "", "", "", "", "", 0, 0},
	{3, "LAYER", "N", 1, 0, "", "", "", "", "", "", "", "", 0, 0},
	{4, "RELEVE_NR", "N", 6, 0, "", "", "", "", "", "", "", "", 0, 0},
	{4, "REMARKS", "C", 25, 0, "", "", "", "", "", "", "", "", 0, 0},
	{5, "REFERENCE", "C", 6, 0, "@ 999999", "", "", "", "", "", "Reference", "", 0, 0},
	{5, "AUTHOR", "C", 80, 0, "@S59", "", "", "", "", "", "Author of publication", "", 0, 0},
	{5, "YEAR", "C", 4, 0, "@ 9999", "", "", "", "", "", "Year", "", 0, 0},
	{5, "TITLE", "C", 200, 0, "@S59", "", "", "", "", "", "Title", "", 0, 0},
	{5, "PUBLISHED", "C", 200, 0, "@S59", "", "", "", "", "", "Published in...", "", 0, 0},
	{6, "SCALE_NR", "C", 2, 0, "@ ##", "", "", "", "", "", "Cover scale code", "", 0, 0},
	{6, "SCALE_NAME", "C", 22, 0, "", "", "", "", "", "", "Cover scale name", "", 0, 0},
	{6, "SCALE_CODE", "C", 6, 0, "", "", "", "", "", "", "Cover scale abbr.", "", 0, 0},
	{7, "CODE", "C", 2, 0, "@ !!", "", "", "", "", "", "Country code", "", 0, 0},
	{7, "COUNTRY", "C", 25, 0, "", "", "", "", "", "", "Country name", "", 0, 0},
	{8, "AUTH_CODE", "C", 4, 0, "@ ####", "", "", "", "", "", "Author code", "", 0, 0},
	{8, "AUTH_NAME", "C", 30, 0, "", "", "", "", "", "", "Author name", "", 0, 0},
	{9, "PROJECT", "C", 3, 0, "@ ###", "", "", "", "", "", "Project code", "", 0, 0},
	{9, "PROJ_NAME", "C", 30, 0, "", "", "", "", "", "", "Project name", "", 0, 0},
	{10, "CODE", "C", 7, 0, "@ ##!!99!", "", "", "", "", "", "Syntaxon code", "", 0, 0},
	{10, "SYNTAXON", "C", 56, 0, "", "", "", "", "", "", "Syntaxon name", "", 0, 0},
	{11, "EXPOSITION", "C", 3, 0, "@ ###", "", "", "", "", "", "Exposition (degrees)", "", 0, 0},
	{11, "CODE", "C", 3, 0, "@ !!!", "", "", "", "", "", "Exposition", "", 0, 0},
}

func makeStringUDF(name, val string, maxLen int) (xmlPkgUDF, bool) {
	if strings.TrimSpace(val) == "" {
		return xmlPkgUDF{}, false
	}
	l := len(val)
	if l < maxLen {
		l = maxLen
	}
	return xmlPkgUDF{
		IsPredefined: "false",
		Name:         name,
		Type:         "C",
		Len:          l,
		Dec:          0,
		Value:        val,
	}, true
}

func makeFloatUDF(name string, val *float64, width, dec int) (xmlPkgUDF, bool) {
	if val == nil {
		return xmlPkgUDF{}, false
	}
	return xmlPkgUDF{
		IsPredefined: "false",
		Name:         name,
		Type:         "N",
		Len:          width,
		Dec:          dec,
		Value:        strconv.FormatFloat(*val, 'f', -1, 64),
	}, true
}

func allocateUniqueLookupCode(fullValue, baseSeed string, maxUnits int, codeByValue, descByCode map[string]string) string {
	normFull := strings.TrimSpace(fullValue)
	if existing, ok := codeByValue[normFull]; ok {
		return existing
	}
	cand := truncateUTF16Units(strings.TrimSpace(baseSeed), maxUnits)
	if cand != "" {
		if prevDesc, used := descByCode[cand]; !used || prevDesc == normFull {
			codeByValue[normFull] = cand
			descByCode[cand] = normFull
			return cand
		}
	}
	for seq := 1; ; seq++ {
		suffix := strconv.FormatInt(int64(seq), 36)
		suffix = strings.ToUpper(suffix)
		var disambig string
		if maxPrefix := maxUnits - len(suffix); maxPrefix > 0 {
			prefix := truncateUTF16Units(strings.TrimSpace(baseSeed), maxPrefix)
			disambig = prefix + suffix
		} else {
			disambig = truncateUTF16Units(strings.Repeat("0", max(0, maxUnits-len(suffix)))+suffix, maxUnits)
		}
		if disambig == "" {
			continue
		}
		if prevDesc, used := descByCode[disambig]; !used || prevDesc == normFull {
			codeByValue[normFull] = disambig
			descByCode[disambig] = normFull
			return disambig
		}
	}
}

func encodeTurboVegXML(ctx context.Context, ds tvDataset) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	projName := strings.TrimSpace(ds.ProjectName)
	if projName == "" {
		projName = "VProExport"
	}
	todayStr := time.Now().UTC().Format("02/01/2006")
	projCode := strings.ToUpper(truncateUTF16Units(projName, 3))
	if projCode == "" {
		projCode = "VPR"
	}

	doc := xmlPlotPackageDoc{
		UserName:             "VPro",
		EmailAddress:         "",
		TurbovegVersion:      "3.0",
		Dictionary:           "vpro_bec_dict",
		SpeciesList:          "BCPlantTaxonomy",
		Description:          "VPro BEC Export",
		ShareWithMembersOnly: "no",
		CreationDate:         todayStr,
		Plots: xmlPlotPackagePlots{
			Plot: make([]xmlPkgPlot, len(ds.Plots)),
		},
		Template: xmlPlotPackageTmpl{
			Files:    canonicalTemplateFiles,
			Dbasedic: canonicalDbasedicRecords,
		},
		Metadata: xmlPlotPackageMeta{
			Records: []string{"Generated by VPro (B.C. Ministry of Forests BEC Program)"},
		},
	}

	coverSet := map[string]float64{"r": 0.05, "+": 0.50}
	authorCodeByFull := map[string]string{}
	authorSet := map[string]string{}
	syntaxonCodeByFull := map[string]string{}
	syntaxonSet := map[string]string{}

	for i, pb := range ds.Plots {
		if i%128 == 0 {
			runtime.Gosched()
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		h := pb.Header
		releveNR := h.PlotID
		if releveNR <= 0 {
			releveNR = i + 1
		}
		guid := strings.ToUpper(strings.Trim(strings.TrimSpace(h.GUID), "{}"))
		if guid == "" {
			guid = strings.ToUpper(newTurboVegUUID())
		}

		dbName := strings.TrimSpace(h.ProjectID)
		if dbName == "" {
			dbName = projName
		}

		std := xmlPkgHeaderStd{
			ReleveNR:   releveNR,
			Country:    "CA",
			Coverscale: "00",
			Date:       formatTVStandardDateAttr(h.Date),
			Project:    projCode,
		}
		if h.PlotSize != nil {
			std.SurfArea = strconv.FormatFloat(*h.PlotSize, 'f', 2, 64)
		}
		if h.Elevation != nil {
			std.Altitude = strconv.Itoa(*h.Elevation)
		}
		if h.Aspect != nil {
			std.Exposition = strconv.Itoa(*h.Aspect)
		}
		if h.Slope != nil {
			std.Inclinatio = strconv.FormatFloat(*h.Slope, 'f', -1, 64)
		}
		if surv := strings.TrimSpace(h.Surveyor); surv != "" {
			seed := strings.ToUpper(strings.Join(strings.Fields(surv), ""))
			std.Author = allocateUniqueLookupCode(surv, seed, 4, authorCodeByFull, authorSet)
		}
		if su := strings.TrimSpace(h.SiteUnit); su != "" {
			seed := strings.Join(strings.Fields(su), "")
			std.Syntaxon = allocateUniqueLookupCode(su, seed, 7, syntaxonCodeByFull, syntaxonSet)
		}
		if strings.TrimSpace(h.SiteNotes) != "" {
			std.Remarks = truncateUTF16Units(strings.TrimSpace(h.SiteNotes), 56)
		}

		udfs := make([]xmlPkgUDF, 0, 20)
		appendStr := func(name, val string, maxLen int) {
			if u, ok := makeStringUDF(name, val, maxLen); ok {
				udfs = append(udfs, u)
			}
		}
		appendFlt := func(name string, val *float64, width, dec int) {
			if u, ok := makeFloatUDF(name, val, width, dec); ok {
				udfs = append(udfs, u)
			}
		}

		appendStr("vpro_plot_number", h.PlotNumber, 7)
		appendStr("vpro_date", h.Date, 40)
		appendStr("field_number", h.FieldNumber, 50)
		appendStr("project_id", h.ProjectID, 20)
		appendStr("surveyor", h.Surveyor, 30)
		appendFlt("latitude", h.Latitude, 13, 8)
		appendFlt("longitude", h.Longitude, 13, 8)
		appendStr("zone", h.Zone, 4)
		appendStr("subzone", h.SubZone, 8)
		appendStr("site_series", h.SiteSeries, 5)
		appendStr("site_unit", h.SiteUnit, 100)
		appendStr("moisture_regime", h.MoistureRegime, 3)
		appendStr("nutrient_regime", h.NutrientRegime, 2)
		appendStr("meso_slope_position", h.MesoSlopePosition, 3)
		appendStr("surface_shape", h.SurfaceShape, 3)
		appendStr("soil_drainage", h.SoilDrainage, 5)
		appendStr("humus_form", h.HumusForm, 4)
		appendStr("location", h.Location, 255)
		appendStr("site_notes", h.SiteNotes, 255)
		appendStr("veg_notes", h.VegNotes, 255)
		appendStr("soil_notes", h.SoilNotes, 255)

		sppEntries := make([]xmlPkgSpeciesEntry, len(pb.Observations))
		for j, o := range pb.Observations {
			tvLayer := o.TVLayerID
			if tvLayer < 0 || tvLayer > 10 {
				tvLayer = 6
			}
			covStr := formatCoverCodeKey(o.Cover)
			coverSet[covStr] = o.Cover

			sUDFs := make([]xmlPkgUDF, 0, 4)
			if u, ok := makeStringUDF("vpro_code", o.SpeciesCode, 8); ok {
				sUDFs = append(sUDFs, u)
			}
			if u, ok := makeStringUDF("vpro_layer", o.VProLayer, 8); ok {
				sUDFs = append(sUDFs, u)
			}
			if u, ok := makeFloatUDF("height", o.Height, 6, 2); ok {
				sUDFs = append(sUDFs, u)
			}
			if u, ok := makeStringUDF("collected", o.Collected, 1); ok {
				sUDFs = append(sUDFs, u)
			}

			sppEntries[j] = xmlPkgSpeciesEntry{
				Standard: xmlPkgSpeciesStd{
					NR:    o.SpeciesNR,
					Cover: covStr,
					Layer: tvLayer,
				},
				UDFs: sUDFs,
			}
		}

		doc.Plots.Plot[i] = xmlPkgPlot{
			Database:   dbName,
			ReleveNR:   releveNR,
			GUID:       "{" + guid + "}",
			CreateUser: "VPro",
			CreateDate: todayStr,
			ModifyUser: "",
			ModifyDate: "",
			NDFFQual:   "0",
			HeaderData: xmlPkgHeaderData{
				Standard: std,
				UDFs:     udfs,
			},
			SpeciesDat: xmlPkgSpeciesData{
				Species: sppEntries,
			},
		}
	}

	// Build Lookup_tables
	covKeys := make([]string, 0, len(coverSet))
	for k := range coverSet {
		covKeys = append(covKeys, k)
	}
	slices.SortFunc(covKeys, func(a, b string) int {
		va, vb := coverSet[a], coverSet[b]
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
		return strings.Compare(a, b)
	})
	covData := make([]xmlCoverscaleData, len(covKeys))
	for i, k := range covKeys {
		covData[i] = xmlCoverscaleData{
			Code:       k,
			Percentage: strconv.FormatFloat(coverSet[k], 'f', -1, 64),
		}
	}
	doc.LookupTables.CoverscaleList = []xmlCoverscaleRecord{
		{
			Code:        "00",
			Description: "Percentage (%)",
			DataRecords: covData,
		},
	}
	doc.LookupTables.CountryList = []xmlSimpleLookupRec{
		{Code: "CA", Description: "Canada"},
	}
	doc.LookupTables.ProjectList = []xmlSimpleLookupRec{
		{Code: projCode, Description: projName},
	}
	authorKeys := make([]string, 0, len(authorSet))
	for k := range authorSet {
		authorKeys = append(authorKeys, k)
	}
	slices.Sort(authorKeys)
	for _, k := range authorKeys {
		doc.LookupTables.AuthorList = append(doc.LookupTables.AuthorList, xmlSimpleLookupRec{Code: k, Description: authorSet[k]})
	}
	sxKeys := make([]string, 0, len(syntaxonSet))
	for k := range syntaxonSet {
		sxKeys = append(sxKeys, k)
	}
	slices.Sort(sxKeys)
	for _, k := range sxKeys {
		doc.LookupTables.SyntaxonList = append(doc.LookupTables.SyntaxonList, xmlSimpleLookupRec{Code: k, Description: syntaxonSet[k]})
	}

	doc.LookupTables.SpeciesList = make([]xmlSpeciesRecord, len(ds.Species))
	for i, sp := range ds.Species {
		sci := strings.TrimSpace(sp.ScientificName)
		if sci == "" {
			sci = sp.SpeciesCode
		}
		letter := truncateUTF16Units(strings.TrimSpace(sp.LetterCode), 7)
		if letter == "" {
			letter = truncateUTF16Units(strings.TrimSpace(sp.SpeciesCode), 7)
		}
		doc.LookupTables.SpeciesList[i] = xmlSpeciesRecord{
			NR:         sp.SpeciesNR,
			Code:       letter,
			Name:       sci,
			Author:     sp.Authority,
			ValidNR:    strconv.Itoa(sp.SpeciesNR),
			Synonym:    "no",
			NativeName: sp.EnglishName,
			VProCode:   sp.SpeciesCode,
		}
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

func cleanTVXMLAttr(val string) string {
	t := strings.TrimSpace(val)
	if strings.EqualFold(t, "null") {
		return ""
	}
	return t
}

func attrMapFromElement(se xml.StartElement) map[string]string {
	m := make(map[string]string, len(se.Attr))
	for _, a := range se.Attr {
		m[strings.ToLower(a.Name.Local)] = cleanTVXMLAttr(a.Value)
	}
	return m
}

type rawXMLSpeciesObs struct {
	nr        int
	rawCover  string
	rawLayer  string
	vproCode  string
	vproLayer string
	height    *float64
	collected string
}

type rawXMLPlot struct {
	header          tvPlotHeader
	coverscale      string
	authorCode      string
	projectCode     string
	syntaxonCode    string
	expositionRaw   string
	hasExplicitPlot bool
	hasExplicitAuth bool
	hasExplicitProj bool
	hasExplicitSU   bool
	hasExplicitNote bool
	remarksFallback string
	observations    []rawXMLSpeciesObs
}

type rawXMLSpeciesLookup struct {
	nr        int
	code      string
	name      string
	validNR   int
	validName string
	synonym   bool
	vproCode  string
}

func decodeRawXMLPlot(dec *xml.Decoder, se xml.StartElement, plotSeq int) (rawXMLPlot, error) {
	pAttrs := attrMapFromElement(se)
	releveNR, _ := strconv.Atoi(pAttrs["releve_nr"])
	if releveNR <= 0 {
		releveNR = plotSeq
	}
	rp := rawXMLPlot{
		coverscale: "00",
		header: tvPlotHeader{
			PlotID: releveNR,
		},
	}
	if dbName := pAttrs["database"]; dbName != "" {
		rp.projectCode = dbName
	}

	depth := 1
	inHeaderData := false
	inSpeciesData := false
	var curSpp *rawXMLSpeciesObs

	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return rawXMLPlot{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			tag := strings.ToLower(t.Name.Local)
			switch {
			case tag == "header_data":
				inHeaderData = true
			case tag == "species_data":
				inSpeciesData = true
			case inHeaderData && tag == "standard_record":
				a := attrMapFromElement(t)
				if rStr := a["releve_nr"]; rStr != "" && rp.header.PlotNumber == "" {
					rp.header.PlotNumber = rStr
				}
				if cs := a["coverscale"]; cs != "" {
					rp.coverscale = cs
				} else if cs2 := a["cover_scale"]; cs2 != "" {
					rp.coverscale = cs2
				}
				if d := a["date"]; d != "" && rp.header.Date == "" {
					rp.header.Date = normalizeTVDateString(d)
				}
				if sa := a["surf_area"]; sa != "" {
					if f, pErr := strconv.ParseFloat(sa, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.PlotSize = &f
					}
				}
				if alt := a["altitude"]; alt != "" {
					if f, pErr := strconv.ParseFloat(alt, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						iv := int(math.Round(f))
						rp.header.Elevation = &iv
					}
				}
				if exp := a["exposition"]; exp != "" {
					rp.expositionRaw = exp
					if deg, ok := parseCompassAspectDegrees(exp); ok {
						rp.header.Aspect = &deg
					}
				}
				if inc := a["inclinatio"]; inc != "" {
					if f, pErr := strconv.ParseFloat(inc, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Slope = &f
					}
				}
				if lat := a["latitude"]; lat != "" {
					if f, pErr := strconv.ParseFloat(lat, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Latitude = &f
					}
				}
				if lon := a["longitude"]; lon != "" {
					if f, pErr := strconv.ParseFloat(lon, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Longitude = &f
					}
				}
				if nrTab := a["nr_in_tab"]; nrTab != "" && rp.header.FieldNumber == "" {
					rp.header.FieldNumber = nrTab
				}
				if auth := a["author"]; auth != "" {
					rp.authorCode = auth
				}
				if prj := a["project"]; prj != "" {
					rp.projectCode = prj
				}
				if sx := a["syntaxon"]; sx != "" {
					rp.syntaxonCode = sx
				}
				if rem := a["remarks"]; rem != "" {
					rp.remarksFallback = rem
				}
			case inHeaderData && tag == "udf_record":
				a := attrMapFromElement(t)
				uName := strings.ToLower(strings.TrimSpace(a["name"]))
				uVal := a["value"]
				if uName == "" || uVal == "" {
					break
				}
				switch uName {
				case "vpro_plot_number", "vpro_plot", "plot_number":
					rp.header.PlotNumber = uVal
					rp.hasExplicitPlot = true
				case "vpro_date":
					rp.header.Date = uVal
				case "date":
					rp.header.Date = normalizeTVDateString(uVal)
				case "field_number", "field_nr":
					rp.header.FieldNumber = uVal
				case "project_id":
					rp.header.ProjectID = uVal
					rp.hasExplicitProj = true
				case "surveyor", "site_surveyor":
					rp.header.Surveyor = uVal
					rp.hasExplicitAuth = true
				case "latitude", "deg_lat":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Latitude = &f
					}
				case "longitude", "deg_lon":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Longitude = &f
					}
				case "elevation", "altitude":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						iv := int(math.Round(f))
						rp.header.Elevation = &iv
					}
				case "slope", "slope_gradient":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.Slope = &f
					}
				case "aspect", "exposition":
					if deg, ok := parseCompassAspectDegrees(uVal); ok {
						rp.header.Aspect = &deg
					}
				case "plot_size", "surf_area":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						rp.header.PlotSize = &f
					}
				case "zone":
					rp.header.Zone = uVal
				case "subzone":
					rp.header.SubZone = uVal
				case "site_series":
					rp.header.SiteSeries = uVal
				case "site_unit":
					rp.header.SiteUnit = uVal
					rp.hasExplicitSU = true
				case "moisture_regime", "moisture":
					rp.header.MoistureRegime = uVal
				case "nutrient_regime", "nutrient":
					rp.header.NutrientRegime = uVal
				case "meso_slope_position", "meso_slope":
					rp.header.MesoSlopePosition = uVal
				case "surface_shape", "surf_shape":
					rp.header.SurfaceShape = uVal
				case "soil_drainage", "drainage":
					rp.header.SoilDrainage = uVal
				case "humus_form":
					rp.header.HumusForm = uVal
				case "location", "locality":
					rp.header.Location = uVal
				case "site_notes":
					rp.header.SiteNotes = uVal
					rp.hasExplicitNote = true
				case "veg_notes":
					rp.header.VegNotes = uVal
				case "soil_notes":
					rp.header.SoilNotes = uVal
				}
			case inSpeciesData && tag == "species":
				curSpp = &rawXMLSpeciesObs{}
			case inSpeciesData && curSpp != nil && tag == "standard_record":
				a := attrMapFromElement(t)
				curSpp.nr, _ = strconv.Atoi(a["nr"])
				curSpp.rawCover = a["cover"]
				curSpp.rawLayer = a["layer"]
			case inSpeciesData && curSpp != nil && tag == "udf_record":
				a := attrMapFromElement(t)
				uName := strings.ToLower(strings.TrimSpace(a["name"]))
				uVal := a["value"]
				switch uName {
				case "vpro_code":
					curSpp.vproCode = uVal
				case "vpro_layer":
					curSpp.vproLayer = uVal
				case "height":
					if f, pErr := strconv.ParseFloat(uVal, 64); pErr == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
						curSpp.height = &f
					}
				case "collected":
					curSpp.collected = uVal
				}
			}
		case xml.EndElement:
			tag := strings.ToLower(t.Name.Local)
			switch tag {
			case "header_data":
				inHeaderData = false
			case "species_data":
				inSpeciesData = false
			case "species":
				if curSpp != nil {
					rp.observations = append(rp.observations, *curSpp)
					curSpp = nil
				}
			}
			depth--
		}
	}

	if !rp.hasExplicitPlot && strings.TrimSpace(rp.header.PlotNumber) == "" && releveNR > 0 {
		rp.header.PlotNumber = strconv.Itoa(releveNR)
	}
	return rp, nil
}

func parseTurboVegXMLFile(ctx context.Context, path string, resolver *speciesImportResolver) ([]tvImportPlot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	br := bufio.NewReaderSize(f, 64*1024)
	if bom, peekErr := br.Peek(3); peekErr == nil && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		_, _ = br.Discard(3)
	}

	dec := xml.NewDecoder(br)
	dec.CharsetReader = turboVegXMLCharsetReader

	var rawPlots []rawXMLPlot
	sppLookup := map[int]rawXMLSpeciesLookup{}
	var sppOrder []int
	coverscales := map[string]map[string]float64{}
	authors := map[string]string{}
	projects := map[string]string{}
	expositions := map[string]int{}

	sawRoot := false
	plotCounter := 0

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse TurboVeg Plot_package XML: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !sawRoot {
			sawRoot = true
			if !strings.EqualFold(se.Name.Local, "Plot_package") {
				return nil, fmt.Errorf("expected Turboveg standard XML root <Plot_package>, got <%s>", se.Name.Local)
			}
			continue
		}

		switch strings.ToLower(se.Name.Local) {
		case "plot":
			plotCounter++
			if plotCounter == 1 || plotCounter%128 == 0 {
				runtime.Gosched()
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			rp, pErr := decodeRawXMLPlot(dec, se, plotCounter)
			if pErr != nil {
				return nil, fmt.Errorf("decode <Plot> #%d: %w", plotCounter, pErr)
			}
			rawPlots = append(rawPlots, rp)

		case "coverscale_record":
			var cs xmlCoverscaleRecord
			if err := dec.DecodeElement(&cs, &se); err != nil {
				return nil, err
			}
			scaleCode := strings.TrimSpace(cs.Code)
			if scaleCode == "" {
				scaleCode = "00"
			}
			if coverscales[scaleCode] == nil {
				coverscales[scaleCode] = map[string]float64{}
			}
			for _, dr := range cs.DataRecords {
				if pct, pErr := strconv.ParseFloat(strings.TrimSpace(dr.Percentage), 64); pErr == nil && !math.IsNaN(pct) && !math.IsInf(pct, 0) {
					coverscales[scaleCode][strings.TrimSpace(dr.Code)] = pct
					coverscales[scaleCode][strings.ToLower(strings.TrimSpace(dr.Code))] = pct
				}
			}

		case "species_list":
			depth := 1
			sppTokSeq := 0
			for depth > 0 {
				sppTokSeq++
				if sppTokSeq%512 == 0 {
					runtime.Gosched()
					if err := ctx.Err(); err != nil {
						return nil, err
					}
				}
				sTok, sErr := dec.Token()
				if sErr != nil {
					return nil, sErr
				}
				switch st := sTok.(type) {
				case xml.StartElement:
					depth++
					rTag := strings.ToLower(st.Name.Local)
					if rTag == "species_record" || rTag == "record" {
						a := attrMapFromElement(st)
						nr, _ := strconv.Atoi(a["nr"])
						valNR, _ := strconv.Atoi(a["valid_nr"])
						syn := strings.EqualFold(a["synonym"], "yes") || strings.EqualFold(a["synonym"], "true") || a["synonym"] == "1"
						if nr > 0 {
							if _, exists := sppLookup[nr]; !exists {
								sppOrder = append(sppOrder, nr)
							}
							sppLookup[nr] = rawXMLSpeciesLookup{
								nr:        nr,
								code:      a["code"],
								name:      a["name"],
								validNR:   valNR,
								validName: a["valid_name"],
								synonym:   syn,
								vproCode:  a["vpro_code"],
							}
						}
					}
				case xml.EndElement:
					depth--
				}
			}

		case "author_record":
			a := attrMapFromElement(se)
			if a["code"] != "" && a["description"] != "" {
				authors[a["code"]] = a["description"]
			}
		case "project_record":
			a := attrMapFromElement(se)
			if a["code"] != "" && a["description"] != "" {
				projects[a["code"]] = a["description"]
			}
		case "exposition_record":
			a := attrMapFromElement(se)
			if a["code"] != "" {
				if deg, ok := parseCompassAspectDegrees(a["description"]); ok {
					expositions[strings.ToUpper(a["code"])] = deg
				}
			}
		}
	}

	if !sawRoot {
		return nil, errors.New("TurboVeg XML file is empty")
	}

	// Post-stream resolution pass: resolve <Species_list>, <Coverscale_list>, and lookup references across all plots.
	type resolvedXMLTaxon struct {
		code  string
		woody bool
	}
	resolvedByNR := make(map[int]resolvedXMLTaxon, len(sppLookup))

	for idx, nr := range sppOrder {
		if idx%512 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		sp := sppLookup[nr]
		validSci := sp.validName
		if sp.synonym && validSci == "" && sp.validNR > 0 {
			if target, ok := sppLookup[sp.validNR]; ok {
				validSci = target.name
			}
		}
		code, lf := resolver.resolveTaxon(sp.vproCode, sp.name, validSci, sp.code)
		resolvedByNR[nr] = resolvedXMLTaxon{
			code:  code,
			woody: isWoodyVProLifeform(lf),
		}
	}

	plots := make([]tvImportPlot, len(rawPlots))
	for i, rp := range rawPlots {
		if i%128 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		h := rp.header
		if !rp.hasExplicitAuth && h.Surveyor == "" && rp.authorCode != "" {
			if full, ok := authors[rp.authorCode]; ok {
				h.Surveyor = full
			} else {
				h.Surveyor = rp.authorCode
			}
		}
		if !rp.hasExplicitProj && h.ProjectID == "" && rp.projectCode != "" {
			if full, ok := projects[rp.projectCode]; ok {
				h.ProjectID = full
			} else {
				h.ProjectID = rp.projectCode
			}
		}
		if !rp.hasExplicitSU && h.SiteUnit == "" && rp.syntaxonCode != "" {
			h.SiteUnit = rp.syntaxonCode
		}
		if !rp.hasExplicitNote && h.SiteNotes == "" && rp.remarksFallback != "" {
			h.SiteNotes = rp.remarksFallback
		}
		if h.Aspect == nil && rp.expositionRaw != "" {
			if deg, ok := expositions[strings.ToUpper(rp.expositionRaw)]; ok {
				h.Aspect = &deg
			}
		}

		scale := strings.TrimSpace(rp.coverscale)
		if scale == "" {
			scale = "00"
		}
		scaleMap := coverscales[scale]

		obsOut := make([]tvImportObs, 0, len(rp.observations))
		for _, ro := range rp.observations {
			rawCov := strings.TrimSpace(ro.rawCover)
			if rawCov == "" {
				continue
			}
			var cov float64
			var haveCov bool
			if scale != "00" && scaleMap != nil {
				if v, ok := scaleMap[rawCov]; ok {
					cov, haveCov = v, true
				} else if v, ok := scaleMap[strings.ToLower(rawCov)]; ok {
					cov, haveCov = v, true
				}
			}
			if !haveCov {
				if f, pErr := strconv.ParseFloat(rawCov, 64); pErr == nil {
					if scale != "00" {
						if bb, ok := defaultBraunBlanquetMidpoint(rawCov); ok {
							cov, haveCov = bb, true
						} else {
							cov, haveCov = f, true
						}
					} else {
						cov, haveCov = f, true
					}
				} else if scaleMap != nil {
					if v, ok := scaleMap[rawCov]; ok {
						cov, haveCov = v, true
					} else if v, ok := scaleMap[strings.ToLower(rawCov)]; ok {
						cov, haveCov = v, true
					}
				}
			}
			if !haveCov {
				if bb, ok := defaultBraunBlanquetMidpoint(rawCov); ok {
					cov, haveCov = bb, true
				}
			}
			if !haveCov {
				return nil, fmt.Errorf("plot %s species nr=%d has unresolvable cover code %q in coverscale %q", h.PlotNumber, ro.nr, rawCov, scale)
			}

			var code string
			var woody bool
			if strings.TrimSpace(ro.vproCode) != "" {
				c, lf := resolver.resolveTaxon(ro.vproCode, "", "", "")
				code = c
				woody = isWoodyVProLifeform(lf)
				if lk, ok := resolvedByNR[ro.nr]; ok && lf == nil {
					woody = lk.woody
				}
			} else if lk, ok := resolvedByNR[ro.nr]; ok {
				code = lk.code
				woody = lk.woody
			}
			if code == "" {
				continue
			}

			obsOut = append(obsOut, tvImportObs{
				SpeciesCode: code,
				TVLayerRaw:  ro.rawLayer,
				VProLayer:   ro.vproLayer,
				IsWoody:     woody,
				Cover:       cov,
				Height:      ro.height,
				Collected:   ro.collected,
			})
		}

		plots[i] = tvImportPlot{
			Header:       h,
			Observations: obsOut,
		}
	}

	return plots, nil
}
