package interchange

import (
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
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/accessreader"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type VenusProjectCandidate struct {
	ProjectID         string `json:"projectId"`
	ProjectName       string `json:"projectName"`
	SuggestedTarget   string `json:"suggestedTarget"`
	SuggestedDestPath string `json:"suggestedDestPath"`
	PlotCount         int    `json:"plotCount"`
	VegObsCount       int    `json:"vegObsCount"`
	HumusRowCount     int    `json:"humusRowCount"`
	MineralRowCount   int    `json:"mineralRowCount"`
}

type VenusSourceInspection struct {
	SourcePath   string                  `json:"sourcePath"`
	SourceFormat string                  `json:"sourceFormat"` // "mdb" | "xml"
	VenusVersion string                  `json:"venusVersion"`
	SourceSHA256 string                  `json:"sourceSha256"`
	Projects     []VenusProjectCandidate `json:"projects"`
}

type VenusImportRequest struct {
	SourcePath        string `json:"sourcePath"`
	SourceProject     string `json:"sourceProject,omitempty"`
	TargetProject     string `json:"targetProject"`
	DestinationPath   string `json:"destinationPath,omitempty"`
	ActivateOnSuccess bool   `json:"activateOnSuccess"`
}

func (r *VenusImportRequest) UnmarshalJSON(data []byte) error {
	type plain VenusImportRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type VenusImportResult struct {
	SourcePath      string                 `json:"sourcePath"`
	SourceFormat    string                 `json:"sourceFormat"`
	VenusVersion    string                 `json:"venusVersion"`
	SourceProject   string                 `json:"sourceProject"`
	TargetProject   string                 `json:"targetProject"`
	DestinationPath string                 `json:"destinationPath"`
	DestinationSHA  string                 `json:"destinationSha256"`
	PlotCount       int                    `json:"plotCount"`
	VegRowCount     int                    `json:"vegRowCount"`
	HumusRowCount   int                    `json:"humusRowCount"`
	MineralRowCount int                    `json:"mineralRowCount"`
	Activated       bool                   `json:"activated"`
	State           *vprocore.ProjectState `json:"state,omitempty"`
}

type venusNormalizedArchive struct {
	Format   string
	Version  string
	SHA256   string
	Projects map[string]string // projectKey -> projectName
	Plots    []venusPlotRecord
}

type venusPlotRecord struct {
	PlotID          string
	ProjectKey      string
	ProjectName     string
	PlotQuality     string
	PlotNumber      string
	Graviti         bool
	Fields          map[string]string
	VegObservations []venusVegObs
	HumusRows       []map[string]string
	MineralRows     []map[string]string
}

type venusVegObs struct {
	Species   string
	Layer     string
	Cover     float64
	Height    *float64
	Collected string
}

func InspectVenusSource(s *vprocore.ContextService, ctx context.Context, contextID, sourcePath string) (VenusSourceInspection, error) {
	var projectsDir string
	if _, err := vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (struct{}, error) {
		projectsDir = filepath.Join(plots.Projects.Root, "projects")
		return struct{}{}, nil
	}); err != nil {
		return VenusSourceInspection{}, err
	}

	absPath, archive, err := loadVenusArchive(ctx, sourcePath)
	if err != nil {
		return VenusSourceInspection{}, err
	}

	byProj := map[string]*VenusProjectCandidate{}
	for _, pl := range archive.Plots {
		pName := strings.TrimSpace(pl.ProjectName)
		if pName == "" {
			pName = "VENUS5"
		}
		cand := byProj[pName]
		if cand == nil {
			suggested := sanitizeProjectSuggestion(pName)
			dest := filepath.Join(projectsDir, suggested+".db")
			cand = &VenusProjectCandidate{
				ProjectID:         pl.ProjectKey,
				ProjectName:       pName,
				SuggestedTarget:   suggested,
				SuggestedDestPath: dest,
			}
			byProj[pName] = cand
		}
		cand.PlotCount++
		cand.VegObsCount += len(pl.VegObservations)
		cand.HumusRowCount += len(pl.HumusRows)
		cand.MineralRowCount += len(pl.MineralRows)
	}

	list := make([]VenusProjectCandidate, 0, len(byProj))
	for _, c := range byProj {
		list = append(list, *c)
	}
	slices.SortFunc(list, func(a, b VenusProjectCandidate) int {
		return strings.Compare(a.ProjectName, b.ProjectName)
	})

	return VenusSourceInspection{
		SourcePath:   absPath,
		SourceFormat: archive.Format,
		VenusVersion: archive.Version,
		SourceSHA256: archive.SHA256,
		Projects:     list,
	}, nil
}

func ImportVenusProject(s *vprocore.ContextService, ctx context.Context, contextID string, request VenusImportRequest) (VenusImportResult, error) {
	absPath, archive, err := loadVenusArchive(ctx, request.SourcePath)
	if err != nil {
		return VenusImportResult{}, err
	}

	selectedProject := strings.TrimSpace(request.SourceProject)
	var filteredPlots []venusPlotRecord
	for _, pl := range archive.Plots {
		pName := strings.TrimSpace(pl.ProjectName)
		if pName == "" {
			pName = "VENUS5"
		}
		if selectedProject == "" || strings.EqualFold(selectedProject, pName) || strings.EqualFold(selectedProject, pl.ProjectKey) {
			if selectedProject == "" {
				selectedProject = pName
			}
			filteredPlots = append(filteredPlots, pl)
		}
	}
	if len(filteredPlots) == 0 {
		return VenusImportResult{}, fmt.Errorf("no VENUS plots matched project %q in %s", request.SourceProject, filepath.Base(absPath))
	}

	var vegRowsWritten, humusRowsWritten, mineralRowsWritten int
	destPath, destSHA, activated, newState, err := stageAndPublishVP08Project(
		s, ctx, contextID, request.TargetProject, request.DestinationPath, request.ActivateOnSuccess,
		func(ctx context.Context, tx *sql.Tx, targetProject string) error {
			var wErr error
			vegRowsWritten, humusRowsWritten, mineralRowsWritten, wErr = writeVenusToVP08Tx(ctx, tx, targetProject, selectedProject, archive.Version, filteredPlots)
			return wErr
		},
	)
	if err != nil {
		return VenusImportResult{}, err
	}

	// Verify source file was not mutated
	afterBytes, err := os.ReadFile(absPath)
	if err != nil {
		return VenusImportResult{}, err
	}
	afterHash := sha256.Sum256(afterBytes)
	if hex.EncodeToString(afterHash[:]) != archive.SHA256 {
		return VenusImportResult{}, errors.New("VENUS source file changed during import; publication aborted")
	}

	return VenusImportResult{
		SourcePath:      absPath,
		SourceFormat:    archive.Format,
		VenusVersion:    archive.Version,
		SourceProject:   selectedProject,
		TargetProject:   strings.TrimSpace(request.TargetProject),
		DestinationPath: destPath,
		DestinationSHA:  destSHA,
		PlotCount:       len(filteredPlots),
		VegRowCount:     vegRowsWritten,
		HumusRowCount:   humusRowsWritten,
		MineralRowCount: mineralRowsWritten,
		Activated:       activated,
		State:           newState,
	}, nil
}

func loadVenusArchive(ctx context.Context, sourcePath string) (string, venusNormalizedArchive, error) {
	if strings.TrimSpace(sourcePath) == "" || !utf8.ValidString(sourcePath) || strings.ContainsRune(sourcePath, 0) {
		return "", venusNormalizedArchive{}, errors.New("VENUS sourcePath must be a non-empty valid UTF-8 path")
	}
	abs, err := filepath.Abs(filepath.Clean(sourcePath))
	if err != nil {
		return "", venusNormalizedArchive{}, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", venusNormalizedArchive{}, fmt.Errorf("resolve VENUS source %q: %w", sourcePath, err)
	}
	rawBytes, err := os.ReadFile(resolved)
	if err != nil {
		return "", venusNormalizedArchive{}, err
	}
	sum := sha256.Sum256(rawBytes)
	shaHex := hex.EncodeToString(sum[:])

	ext := strings.ToLower(filepath.Ext(resolved))
	switch ext {
	case ".xml":
		arch, err := parseVenusXMLArchive(ctx, rawBytes, shaHex)
		return resolved, arch, err
	case ".mdb", ".accdb":
		arch, err := parseVenusAccessArchive(ctx, resolved, shaHex)
		return resolved, arch, err
	default:
		return "", venusNormalizedArchive{}, fmt.Errorf("unsupported VENUS file extension %q (expected .mdb, .accdb, or .xml)", ext)
	}
}

func parseVenusAccessArchive(ctx context.Context, resolvedPath, shaHex string) (venusNormalizedArchive, error) {
	db, err := accessreader.Open(resolvedPath)
	if err != nil {
		return venusNormalizedArchive{}, fmt.Errorf("open VENUS Access database: %w", err)
	}
	tables, err := db.Tables()
	if err != nil {
		return venusNormalizedArchive{}, err
	}
	byLower := map[string]string{}
	for _, t := range tables {
		if !t.Linked {
			byLower[strings.ToLower(t.Name)] = t.Name
		}
	}

	readStringTable := func(candidates ...string) ([]map[string]string, bool, error) {
		for _, cand := range candidates {
			actual, ok := byLower[strings.ToLower(cand)]
			if !ok {
				continue
			}
			cols, rows, err := db.ReadAll(actual)
			if err != nil {
				return nil, false, err
			}
			adaptedIdx, adaptedRows, err := adaptAccessRows(ctx, cols, rows)
			if err != nil {
				return nil, false, err
			}
			out := make([]map[string]string, 0, len(adaptedRows))
			for _, r := range adaptedRows {
				m := make(map[string]string, len(adaptedIdx))
				for k, idx := range adaptedIdx {
					if r[idx] != nil {
						m[k] = strings.TrimSpace(fmt.Sprint(r[idx]))
					}
				}
				out = append(out, m)
			}
			return out, true, nil
		}
		return nil, false, nil
	}

	version := "5.0"
	if vRows, ok, _ := readStringTable("USysVersion"); ok && len(vRows) > 0 {
		if v := vRows[0]["version"]; v != "" {
			version = v
			if b := vRows[0]["beta"]; b != "" && b != "0" {
				version += "." + b
			}
		}
	}

	projRows, _, err := readStringTable("tblProjectManager", "tblProject")
	if err != nil {
		return venusNormalizedArchive{}, err
	}
	plotRows, hasPlotTbl, err := readStringTable("tblPlotData", "tblPlot", "tblEnv")
	if err != nil {
		return venusNormalizedArchive{}, err
	}
	if !hasPlotTbl {
		return venusNormalizedArchive{}, errors.New("VENUS database is missing tblPlotData / tblPlot / tblEnv")
	}
	soilRows, _, _ := readStringTable("tblSoil")
	vegMainRows, _, _ := readStringTable("tblVegMain")
	vegLayerRows, _, _ := readStringTable("tblVegLayer")
	vegSppRows, _, _ := readStringTable("tblVegSpecies")
	humusRows, _, _ := readStringTable("tblOrganicHorizons", "tblSoilHumus")
	mineralRows, _, _ := readStringTable("tblMineralHorizons", "tblSoilMineral")

	return assembleRelationalVenusArchive("mdb", version, shaHex, projRows, plotRows, soilRows, vegMainRows, vegLayerRows, vegSppRows, humusRows, mineralRows), nil
}

func firstNonEmpty(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(m[strings.ToLower(k)]); v != "" {
			return v
		}
	}
	return ""
}

func assembleRelationalVenusArchive(
	format, version, shaHex string,
	projRows, plotRows, soilRows, vegMainRows, vegLayerRows, vegSppRows, humusRows, mineralRows []map[string]string,
) venusNormalizedArchive {
	arch := venusNormalizedArchive{
		Format:   format,
		Version:  version,
		SHA256:   shaHex,
		Projects: map[string]string{},
	}

	projQuality := map[string]string{}
	for _, pr := range projRows {
		pid := firstNonEmpty(pr, "projectid", "id")
		pname := firstNonEmpty(pr, "projectname", "name", "projectid")
		if pid == "" {
			pid = pname
		}
		if pname == "" {
			pname = "VENUS5"
		}
		arch.Projects[pid] = pname
		projQuality[pid] = firstNonEmpty(pr, "plotquality", "siteplotquality")
	}

	soilByPlotID := map[string]map[string]string{}
	for _, sr := range soilRows {
		if pid := firstNonEmpty(sr, "plotid", "plotnumber"); pid != "" {
			soilByPlotID[pid] = sr
		}
	}
	vegMainByPlotID := map[string]map[string]string{}
	for _, vr := range vegMainRows {
		if pid := firstNonEmpty(vr, "plotid", "plotnumber"); pid != "" {
			vegMainByPlotID[pid] = vr
		}
	}

	plotIdxByKey := map[string]int{}
	for i, pr := range plotRows {
		plotNum := firstNonEmpty(pr, "plotnumber", "plot")
		plotID := firstNonEmpty(pr, "plotid", "id")
		if plotNum == "" {
			plotNum = plotID
		}
		if plotNum == "" {
			plotNum = fmt.Sprintf("VP%05d", i+1)
		}
		plotNum = truncateUTF16Units(plotNum, 7)
		if plotID == "" {
			plotID = plotNum
		}

		merged := make(map[string]string, len(pr)+16)
		if sr, ok := soilByPlotID[plotID]; ok {
			for k, v := range sr {
				merged[k] = v
			}
		}
		if vm, ok := vegMainByPlotID[plotID]; ok {
			for k, v := range vm {
				merged[k] = v
			}
		}
		for k, v := range pr {
			merged[k] = v
		}

		projKey := firstNonEmpty(pr, "projectid", "projectname")
		projName := arch.Projects[projKey]
		if projName == "" {
			projName = firstNonEmpty(pr, "projectname", "projectid")
			if projName == "" {
				projName = "VENUS5"
			}
		}
		grav := isAccessTruthy(firstNonEmpty(pr, "graviti"))

		rec := venusPlotRecord{
			PlotID:      plotID,
			ProjectKey:  projKey,
			ProjectName: projName,
			PlotQuality: firstNonEmpty(merged, "siteplotquality", "plotquality"),
			PlotNumber:  plotNum,
			Graviti:     grav,
			Fields:      merged,
		}
		if rec.PlotQuality == "" {
			rec.PlotQuality = projQuality[projKey]
		}
		idx := len(arch.Plots)
		arch.Plots = append(arch.Plots, rec)
		plotIdxByKey[plotID] = idx
		plotIdxByKey["PN:"+plotNum] = idx
	}

	findPlotIdx := func(m map[string]string) (int, bool) {
		if pid := firstNonEmpty(m, "plotid"); pid != "" {
			if idx, ok := plotIdxByKey[pid]; ok {
				return idx, true
			}
		}
		if pn := firstNonEmpty(m, "plotnumber", "plot"); pn != "" {
			if idx, ok := plotIdxByKey["PN:"+pn]; ok {
				return idx, true
			}
			if idx, ok := plotIdxByKey[pn]; ok {
				return idx, true
			}
		}
		return -1, false
	}

	// Map VegLayerID -> (plotIdx, layerCode)
	type layerOwner struct {
		plotIdx int
		layer   string
		height  *float64
	}
	layerMap := map[string]layerOwner{}
	for _, lr := range vegLayerRows {
		lid := firstNonEmpty(lr, "veglayerid", "layerid", "id")
		pIdx, ok := findPlotIdx(lr)
		if !ok || lid == "" {
			continue
		}
		var htPtr *float64
		if hs := firstNonEmpty(lr, "height", "layerheight"); hs != "" {
			if hv, err := strconv.ParseFloat(hs, 64); err == nil {
				htPtr = &hv
			}
		}
		layerMap[lid] = layerOwner{
			plotIdx: pIdx,
			layer:   firstNonEmpty(lr, "layer", "layercode"),
			height:  htPtr,
		}
	}

	for _, sr := range vegSppRows {
		spp := strings.ToUpper(firstNonEmpty(sr, "species", "speciescode", "code"))
		covStr := firstNonEmpty(sr, "cover", "percentcover")
		cov, err := strconv.ParseFloat(covStr, 64)
		if spp == "" || err != nil || cov == 0 {
			continue
		}
		var htPtr *float64
		if hs := firstNonEmpty(sr, "height", "speciesheight"); hs != "" {
			if hv, hErr := strconv.ParseFloat(hs, 64); hErr == nil {
				htPtr = &hv
			}
		}
		coll := firstNonEmpty(sr, "collected", "voucher")

		lid := firstNonEmpty(sr, "veglayerid", "layerid")
		if lo, ok := layerMap[lid]; ok {
			if htPtr == nil {
				htPtr = lo.height
			}
			arch.Plots[lo.plotIdx].VegObservations = append(arch.Plots[lo.plotIdx].VegObservations, venusVegObs{
				Species:   spp,
				Layer:     lo.layer,
				Cover:     cov,
				Height:    htPtr,
				Collected: coll,
			})
			continue
		}
		if pIdx, ok := findPlotIdx(sr); ok {
			arch.Plots[pIdx].VegObservations = append(arch.Plots[pIdx].VegObservations, venusVegObs{
				Species:   spp,
				Layer:     firstNonEmpty(sr, "layer", "layercode"),
				Cover:     cov,
				Height:    htPtr,
				Collected: coll,
			})
		}
	}

	for _, hr := range humusRows {
		if pIdx, ok := findPlotIdx(hr); ok {
			arch.Plots[pIdx].HumusRows = append(arch.Plots[pIdx].HumusRows, hr)
		}
	}
	for _, mr := range mineralRows {
		if pIdx, ok := findPlotIdx(mr); ok {
			arch.Plots[pIdx].MineralRows = append(arch.Plots[pIdx].MineralRows, mr)
		}
	}

	return arch
}

// parseVenusXMLArchive parses both hierarchical VENUS 5.0 XML (<VenusArchive>/<VenusDataset>/<VENUS>)
// AND Access/ADO relational table-row XML (<dataroot><tblPlotData>...</tblPlotData>...</dataroot>).
func parseVenusXMLArchive(ctx context.Context, data []byte, shaHex string) (venusNormalizedArchive, error) {
	if !utf8.Valid(data) {
		return venusNormalizedArchive{}, errors.New("VENUS XML contains invalid UTF-8")
	}
	dec := xml.NewDecoder(strings.NewReader(string(data)))

	var (
		version      = "5.0"
		projRows     []map[string]string
		plotRows     []map[string]string
		soilRows     []map[string]string
		vegMainRows  []map[string]string
		vegLayerRows []map[string]string
		vegSppRows   []map[string]string
		humusRows    []map[string]string
		mineralRows  []map[string]string
		hierPlots    []venusPlotRecord
		curProjName  = "VENUS5"
	)

	for {
		if err := ctx.Err(); err != nil {
			return venusNormalizedArchive{}, err
		}
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return venusNormalizedArchive{}, fmt.Errorf("parse VENUS XML: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		tag := strings.ToLower(se.Name.Local)
		for _, attr := range se.Attr {
			al := strings.ToLower(attr.Name.Local)
			if al == "version" && strings.TrimSpace(attr.Value) != "" {
				version = strings.TrimSpace(attr.Value)
			}
			if (tag == "project" || tag == "venus" || tag == "venusdataset" || tag == "venusarchive") && (al == "name" || al == "projectname" || al == "projectid") {
				if strings.TrimSpace(attr.Value) != "" {
					curProjName = strings.TrimSpace(attr.Value)
				}
			}
		}

		switch tag {
		case "usysversion":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			if v := firstNonEmpty(m, "version"); v != "" {
				version = v
			}
		case "tblprojectmanager", "tblproject":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			projRows = append(projRows, m)
		case "tblplotdata", "tblenv", "tblplot":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			plotRows = append(plotRows, m)
		case "tblsoil":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			soilRows = append(soilRows, m)
		case "tblvegmain":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			vegMainRows = append(vegMainRows, m)
		case "tblveglayer":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			vegLayerRows = append(vegLayerRows, m)
		case "tblvegspecies":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			vegSppRows = append(vegSppRows, m)
		case "tblorganichorizons", "tblsoilhumus":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			humusRows = append(humusRows, m)
		case "tblmineralhorizons", "tblsoilmineral":
			m, err := decodeFlatXMLElement(dec, se)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			mineralRows = append(mineralRows, m)
		case "plot":
			hp, err := decodeHierarchicalVenusPlot(dec, se, curProjName, len(hierPlots)+1)
			if err != nil {
				return venusNormalizedArchive{}, err
			}
			hierPlots = append(hierPlots, hp)
		}
	}

	if len(hierPlots) > 0 && len(plotRows) == 0 {
		projs := map[string]string{}
		for _, hp := range hierPlots {
			projs[hp.ProjectName] = hp.ProjectName
		}
		return venusNormalizedArchive{
			Format:   "xml",
			Version:  version,
			SHA256:   shaHex,
			Projects: projs,
			Plots:    hierPlots,
		}, nil
	}
	if len(plotRows) == 0 {
		return venusNormalizedArchive{}, errors.New("VENUS XML contains no Plot / tblPlotData / tblEnv records")
	}
	return assembleRelationalVenusArchive("xml", version, shaHex, projRows, plotRows, soilRows, vegMainRows, vegLayerRows, vegSppRows, humusRows, mineralRows), nil
}

func decodeFlatXMLElement(dec *xml.Decoder, start xml.StartElement) (map[string]string, error) {
	m := map[string]string{}
	for _, a := range start.Attr {
		m[strings.ToLower(a.Name.Local)] = strings.TrimSpace(a.Value)
	}
	var currentField string
	var textBuf strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				currentField = strings.ToLower(t.Name.Local)
				textBuf.Reset()
			}
		case xml.CharData:
			if depth == 2 {
				textBuf.Write(t)
			}
		case xml.EndElement:
			if depth == 2 && currentField != "" {
				m[currentField] = strings.TrimSpace(textBuf.String())
				currentField = ""
			}
			depth--
		}
	}
	return m, nil
}

func decodeHierarchicalVenusPlot(dec *xml.Decoder, start xml.StartElement, defaultProj string, index int) (venusPlotRecord, error) {
	rec := venusPlotRecord{
		ProjectName: defaultProj,
		ProjectKey:  defaultProj,
		Fields:      map[string]string{},
	}
	for _, a := range start.Attr {
		rec.Fields[strings.ToLower(a.Name.Local)] = strings.TrimSpace(a.Value)
	}

	depth := 1
	var curLayerCode string
	var curLayerHeight *float64
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return rec, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sub := strings.ToLower(t.Name.Local)
			switch sub {
			case "env", "site", "soil", "vegmain", "header":
				subMap, sErr := decodeFlatXMLElement(dec, t)
				if sErr != nil {
					return rec, sErr
				}
				for k, v := range subMap {
					rec.Fields[k] = v
				}
			case "veglayer", "layer":
				curLayerCode = ""
				curLayerHeight = nil
				for _, a := range t.Attr {
					al := strings.ToLower(a.Name.Local)
					if al == "layer" || al == "code" || al == "id" {
						curLayerCode = strings.TrimSpace(a.Value)
					} else if al == "height" {
						if hv, pErr := strconv.ParseFloat(strings.TrimSpace(a.Value), 64); pErr == nil {
							curLayerHeight = &hv
						}
					}
				}
				depth++
			case "vegspecies", "species", "observation":
				m, sErr := decodeFlatXMLElement(dec, t)
				if sErr != nil {
					return rec, sErr
				}
				spp := strings.ToUpper(firstNonEmpty(m, "species", "code", "speciescode"))
				cov, cErr := strconv.ParseFloat(firstNonEmpty(m, "cover", "percentcover"), 64)
				if spp != "" && cErr == nil && cov != 0 {
					lyr := firstNonEmpty(m, "layer", "layercode")
					if lyr == "" {
						lyr = curLayerCode
					}
					var ht *float64
					if hs := firstNonEmpty(m, "height"); hs != "" {
						if hv, hErr := strconv.ParseFloat(hs, 64); hErr == nil {
							ht = &hv
						}
					} else {
						ht = curLayerHeight
					}
					rec.VegObservations = append(rec.VegObservations, venusVegObs{
						Species:   spp,
						Layer:     lyr,
						Cover:     cov,
						Height:    ht,
						Collected: firstNonEmpty(m, "collected", "voucher"),
					})
				}
			case "humus", "organichorizon", "humushorizon", "tblsoilhumus", "tblorganichorizons":
				m, sErr := decodeFlatXMLElement(dec, t)
				if sErr != nil {
					return rec, sErr
				}
				if firstNonEmpty(m, "horizon") != "" {
					rec.HumusRows = append(rec.HumusRows, m)
				}
			case "mineral", "mineralhorizon", "tblsoilmineral", "tblmineralhorizons":
				m, sErr := decodeFlatXMLElement(dec, t)
				if sErr != nil {
					return rec, sErr
				}
				if firstNonEmpty(m, "horizon") != "" {
					rec.MineralRows = append(rec.MineralRows, m)
				}
			case "veglayers", "vegetation", "humushorizons", "mineralhorizons", "organichorizons", "soils":
				depth++
			default:
				// Direct scalar child element of <Plot>
				m, sErr := decodeFlatXMLElement(dec, t)
				if sErr != nil {
					return rec, sErr
				}
				if len(m) == 0 {
					// Wait: decodeFlatXMLElement handles attributes & grandchildren, let's handle scalar leaf text if any!
				}
				for k, v := range m {
					rec.Fields[sub+"_"+k] = v
				}
			}
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "veglayer") || strings.EqualFold(t.Name.Local, "layer") {
				curLayerCode = ""
				curLayerHeight = nil
			}
			depth--
		}
	}

	pNum := firstNonEmpty(rec.Fields, "plotnumber", "plot", "id")
	if pNum == "" {
		pNum = fmt.Sprintf("VP%05d", index)
	}
	pNum = truncateUTF16Units(pNum, 7)
	rec.PlotNumber = pNum
	rec.PlotID = pNum
	if pName := firstNonEmpty(rec.Fields, "projectname", "projectid"); pName != "" {
		rec.ProjectName = pName
		rec.ProjectKey = pName
	}
	rec.PlotQuality = firstNonEmpty(rec.Fields, "siteplotquality", "plotquality")
	rec.Graviti = isAccessTruthy(firstNonEmpty(rec.Fields, "graviti"))
	return rec, nil
}

// VENUS 5.0 domain code converters (matching V7mdlImportVenus.txt)
func venusOrgSoilTexture(code string) string {
	switch strings.TrimSpace(code) {
	case "1":
		return "Fi"
	case "2":
		return "Me"
	case "3":
		return "Hu"
	default:
		return ""
	}
}

func venusHumusThickness(code string) *float64 {
	switch strings.TrimSpace(code) {
	case "0":
		v := 0.0
		return &v
	case "1":
		v := 10.0
		return &v
	case "2":
		v := 40.0
		return &v
	default:
		return nil
	}
}

func venusCoarseFrags(code string) *float64 {
	switch strings.TrimSpace(code) {
	case "0":
		v := 0.0
		return &v
	case "1":
		v := 10.0
		return &v
	case "2":
		v := 27.0
		return &v
	case "3":
		v := 52.0
		return &v
	case "4":
		v := 85.0
		return &v
	default:
		return nil
	}
}

func combineVenusTerrain(maxUnits int, a, b, c string) any {
	joined := strings.TrimSpace(a) + strings.TrimSpace(b) + strings.TrimSpace(c)
	if maxUnits > 0 {
		joined = truncateUTF16Units(joined, maxUnits)
	}
	if joined == "" {
		return nil
	}
	return joined
}

func parseVenusCoordDMSOrDD(f map[string]string, ddKey, degKey, minKey, secKey string, isWestLon bool) *float64 {
	if dd := firstNonEmpty(f, ddKey); dd != "" {
		if v, err := strconv.ParseFloat(dd, 64); err == nil && v != 0 {
			if isWestLon && v >= 100 && v <= 170 {
				v = -v
			}
			return &v
		}
	}
	degStr := firstNonEmpty(f, degKey)
	if degStr == "" {
		return nil
	}
	deg, err := strconv.ParseFloat(degStr, 64)
	if err != nil || deg == 0 {
		return nil
	}
	min, _ := strconv.ParseFloat(firstNonEmpty(f, minKey), 64)
	sec, _ := strconv.ParseFloat(firstNonEmpty(f, secKey), 64)
	sign := 1.0
	if deg < 0 {
		sign = -1.0
		deg = -deg
	}
	val := sign * (deg + math.Abs(min)/60.0 + math.Abs(sec)/3600.0)
	if isWestLon && val > 0 && val >= 100 && val <= 170 {
		val = -val
	}
	return &val
}

func optFltField(f map[string]string, keys ...string) *float64 {
	s := firstNonEmpty(f, keys...)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func optIntField(f map[string]string, keys ...string) *int {
	s := firstNonEmpty(f, keys...)
	if s == "" {
		return nil
	}
	if iv, err := strconv.Atoi(s); err == nil {
		return &iv
	}
	if fv, err := strconv.ParseFloat(s, 64); err == nil {
		iv := int(math.Round(fv))
		return &iv
	}
	return nil
}

func optStrField(f map[string]string, maxUTF16 int, keys ...string) any {
	s := firstNonEmpty(f, keys...)
	if s == "" {
		return nil
	}
	if maxUTF16 > 0 {
		s = truncateUTF16Units(s, maxUTF16)
		if s == "" {
			return nil
		}
	}
	return s
}

func optSingleCoverField(f map[string]string, plotNum, label string, keys ...string) (*float64, error) {
	raw := optFltField(f, keys...)
	if raw == nil {
		return nil, nil
	}
	v := *raw
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || math.Abs(v) > math.MaxFloat32 || float32(v) >= 100 {
		return nil, fmt.Errorf("VENUS plot %s %s %v violates IEEE-754 SINGLE [0, 100) bounds", plotNum, label, v)
	}
	narrowed := float64(float32(v))
	return &narrowed, nil
}

func normalizeLMH25HorizonDepths(horizon string, isMineral bool, upper, lower *float64) (normUpper, normLower *float64, quarantineReason string) {
	if upper == nil || lower == nil {
		return upper, lower, ""
	}
	u, l := *upper, *lower
	if math.IsNaN(u) || math.IsInf(u, 0) || math.IsNaN(l) || math.IsInf(l, 0) {
		return upper, lower, "non-finite horizon depth"
	}
	if u < 0 || l < 0 {
		u = math.Abs(u)
		l = math.Abs(l)
	}
	if u == l {
		return upper, lower, "degenerate zero-thickness horizon (UpperDepth == LowerDepth)"
	}
	upward := false
	if !isMineral {
		hz := strings.ToUpper(strings.TrimSpace(horizon))
		if !strings.HasPrefix(hz, "OF") && !strings.HasPrefix(hz, "OM") && !strings.HasPrefix(hz, "OH") &&
			(strings.HasPrefix(hz, "L") || strings.HasPrefix(hz, "F") || strings.HasPrefix(hz, "H")) {
			upward = true
		}
	}
	if upward {
		if u < l {
			u, l = l, u
		}
	} else {
		if u > l {
			u, l = l, u
		}
	}
	return &u, &l, ""
}

func writeVenusToVP08Tx(
	ctx context.Context,
	tx *sql.Tx,
	targetProject string,
	sourceProject string,
	venusVer string,
	plots []venusPlotRecord,
) (vegCount, humusCount, mineralCount int, err error) {
	envTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Env")
	adminTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Admin")
	vegTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Veg")
	humusTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Humus")
	mineralTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Mineral")
	metaTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Metadata")
	auditTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_Audit")

	envCols := []string{
		"PlotNumber", "FieldNumber", "ProjectID", "Date", "SiteSurveyor", "Location",
		"FSRegionDistrict", "NtsMapSheet", "UTMZone", "UTMNorthing", "UTMEasting",
		"Latitude", "Longitude", "AirPhotoNum", "XCoord", "YCoord", "MapUnit", "PlotRepresenting",
		"Zone", "SubZone", "SiteSeries", "TransDistrib", "Ecosection", "MoistureRegime",
		"MoistureRegimeSub", "NutrientRegime", "SuccessionalStatus", "StructuralStage", "RealmClass",
		"SiteDisturbance1", "SiteDisturbance2", "Elevation", "SlopeGradient", "Aspect",
		"MesoSlopePosition", "SurfaceShape", "SurfaceTopographySize", "SurfaceTopographyType",
		"Exposure1", "Photo", "SubstrateDecWood", "SubstrateBedRock", "SubstrateRocks",
		"SubstrateMineralSoil", "SubstrateOrganicMatter", "SubstrateWater", "SiteNotes",
		"StrataCoverHerb", "StrataCoverMoss", "StrataCoverShrub", "StrataCoverTree",
		"VegSurveyor", "VegNotes", "BedrockGeology1", "BedrockGeology2", "BedrockGeology3",
		"CoarseFragLith1", "CoarseFragLith2", "CoarseFragLith3", "SoilSurveyor",
		"SoilClassGroup", "SoilClassSubGroup", "HumusForm", "HydroGeoSystem",
		"RootingDepth", "RootZoneParticleSize", "RootRestrictingDepth", "RootRestrictingType",
		"WaterSource", "SeepageDepth", "SoilDrainage", "FloodingRegimeFreq",
		"pHMethodCodeOrganic", "pHMethodCodeMineral", "SoilNotes",
		"TerrainTextureSurf", "TerrainTextureSubSurf", "SurficialMaterialSurf", "SurficialMaterialSubSurf",
		"SurfaceExpSurf", "SurfaceExpSubSurf", "GeoMorProSurf", "GeoMorProSubSurf", "SV_PercentCoarseFrags",
	}
	qEnvCols := make([]string, len(envCols))
	qEnvMarks := make([]string, len(envCols))
	for i, c := range envCols {
		qEnvCols[i] = vprocore.QuoteHeaderIdentifier(c)
		qEnvMarks[i] = "?"
	}
	envStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+envTbl+` (`+strings.Join(qEnvCols, ",")+`) VALUES (`+strings.Join(qEnvMarks, ",")+`)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer envStmt.Close()

	adminStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+adminTbl+` ("Plot","PlotType","SitePlotQuality","UserSiteUnit","HumusThickness")
		VALUES (?,?,?,?,?)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer adminStmt.Close()

	vegColumns := []string{
		"ID", "PlotNumber", "Species", "Collected",
		"Cover1", "Height1", "Cover2", "Height2", "Cover3", "Height3", "TotalA", "HeightA",
		"Cover4", "Height4", "Cover5", "Height5", "Cover5a", "Height5a", "Cover5b", "Height5b", "Cover5c", "Height5c", "TotalB", "HeightB",
		"Cover6", "Height6", "Cover7", "Cover8", "Cover9", "Cover10",
	}
	qVegCols := make([]string, len(vegColumns))
	qVegMarks := make([]string, len(vegColumns))
	vegColPos := map[string]int{}
	for i, c := range vegColumns {
		qVegCols[i] = vprocore.QuoteHeaderIdentifier(c)
		qVegMarks[i] = "?"
		vegColPos[c] = i
	}
	vegStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+vegTbl+` (`+strings.Join(qVegCols, ", ")+`) VALUES (`+strings.Join(qVegMarks, ", ")+`)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer vegStmt.Close()

	humusStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+humusTbl+` (
			"ID","PlotNumber","Horizon","UpperDepth","LowerDepth",
			"HumusStructureKind","HumusStructureDegree","vonPost",
			"MycelAbundance","FecalAbundance","RootsAbundance","RootsSize","HumusFormpH","Comment"
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer humusStmt.Close()

	mineralStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO `+mineralTbl+` (
			"ID","PlotNumber","Horizon","UpperDepth","LowerDepth","Colour","ASP","Texture",
			"PercentCoarseFragsGravel","PercentCoarseFragsCobbles","PercentCoarseFragsStones",
			"PercentCoarseFragsTotal","PercentCoarseFragsShape","RootsAbundance","RootsSize",
			"MineralStructureClass","MineralStructureKind","MineralFormpH","Comments"
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer mineralStmt.Close()

	var nextVegID, nextHumusID, nextMineralID int64 = 1, 1, 1
	seenPlots := map[string]bool{}
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	projIDVal := truncateUTF16Units(sourceProject, 20)

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL,
		"ID" INTEGER NOT NULL,
		PRIMARY KEY ("ChildTable", "ID")
	)`); err != nil {
		return 0, 0, 0, err
	}

	quarantineSoilHorizon := func(suffix string, id int64, plotNum string, hz any, upper, lower *float64, reason string, rawRow map[string]string) error {
		qTbl := vprocore.QuoteHeaderIdentifier("__VPRO_Quarantine_" + suffix)
		srcTbl := vprocore.QuoteHeaderIdentifier(targetProject + "_" + suffix)
		if rawRow == nil {
			rawRow = map[string]string{}
		}
		payloadBytes, err := json.Marshal(rawRow)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+qTbl+` (
			"__QuarantinedAt" TEXT NOT NULL,
			"__Reason" TEXT NOT NULL,
			"ID" INTEGER,
			"PlotNumber" VARCHAR,
			"Horizon" VARCHAR,
			"UpperDepth" DOUBLE,
			"LowerDepth" DOUBLE,
			"PayloadJSON" TEXT NOT NULL
		)`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO `+qTbl+` ("__QuarantinedAt","__Reason","ID","PlotNumber","Horizon","UpperDepth","LowerDepth","PayloadJSON") VALUES (?,?,?,?,?,?,?,?)`,
			nowStr, reason, id, plotNum, hz, upper, lower, string(payloadBytes),
		); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO "__VPRO_ChildIdentity" ("ChildTable","ID") VALUES (?,?) ON CONFLICT DO NOTHING`,
			srcTbl, id,
		)
		return err
	}

	for _, pl := range plots {
		pNum := truncateUTF16Units(strings.TrimSpace(pl.PlotNumber), 7)
		if pNum == "" || seenPlots[pNum] {
			continue
		}
		seenPlots[pNum] = true
		f := pl.Fields

		lat := parseVenusCoordDMSOrDD(f, "latitude", "latitudedegree", "latitudeminute", "latitudesecond", false)
		lon := parseVenusCoordDMSOrDD(f, "longitude", "longitudedegree", "longitudeminute", "longitudesecond", true)

		rzps := truncateUTF16Units(firstNonEmpty(f, "rootzoneparticlesize"), 6)
		if orgTex := venusOrgSoilTexture(firstNonEmpty(f, "organicsoiltexture")); orgTex != "" && rzps == "" {
			rzps = orgTex
		}
		var rzpsAny any
		if rzps != "" {
			rzpsAny = rzps
		}

		humThick := optFltField(f, "humusthickness")
		if ht := venusHumusThickness(firstNonEmpty(f, "organichorizonthickness")); ht != nil {
			humThick = ht
		}
		svCF := optFltField(f, "sv_percentcoarsefrags")
		if cf := venusCoarseFrags(firstNonEmpty(f, "coarsefragcontent")); cf != nil {
			svCF = cf
		}

		scHerb, err := optSingleCoverField(f, pNum, "StrataCoverHerb", "stratacoverherb")
		if err != nil {
			return 0, 0, 0, err
		}
		scMoss, err := optSingleCoverField(f, pNum, "StrataCoverMoss", "stratacovermoss")
		if err != nil {
			return 0, 0, 0, err
		}
		scShrub, err := optSingleCoverField(f, pNum, "StrataCoverShrub", "stratacovershrub")
		if err != nil {
			return 0, 0, 0, err
		}
		scTree, err := optSingleCoverField(f, pNum, "StrataCoverTree", "stratacovertree")
		if err != nil {
			return 0, 0, 0, err
		}

		tTexSurf := optStrField(f, 3, "terraintexturesurf")
		if tTexSurf == nil {
			tTexSurf = combineVenusTerrain(3, f["terrainstexture1"], f["terrainstexture2"], f["terrainstexture3"])
		}
		tTexSub := optStrField(f, 3, "terraintexturesubsurf")
		if tTexSub == nil {
			tTexSub = combineVenusTerrain(3, f["terrainutexture1"], f["terrainutexture2"], f["terrainutexture3"])
		}
		sMatSurf := optStrField(f, 6, "surficialmaterialsurf")
		if sMatSurf == nil {
			sMatSurf = combineVenusTerrain(6, f["terrainsmaterial1"], f["terrainsmaterial2"], f["terrainsmaterial3"])
		}
		sMatSub := optStrField(f, 6, "surficialmaterialsubsurf")
		if sMatSub == nil {
			sMatSub = combineVenusTerrain(6, f["terrainumaterial1"], f["terrainumaterial2"], f["terrainumaterial3"])
		}
		sExpSurf := optStrField(f, 3, "surfaceexpsurf")
		if sExpSurf == nil {
			sExpSurf = combineVenusTerrain(3, f["terrainsexpression1"], f["terrainsexpression2"], f["terrainsexpression3"])
		}
		sExpSub := optStrField(f, 3, "surfaceexpsubsurf")
		if sExpSub == nil {
			sExpSub = combineVenusTerrain(3, f["terrainuexpression1"], f["terrainuexpression2"], f["terrainuexpression3"])
		}
		geoSurf := optStrField(f, 3, "geomorprosurf")
		if geoSurf == nil {
			geoSurf = combineVenusTerrain(3, f["terrainsgeo1"], f["terrainsgeo2"], f["terrainsgeo3"])
		}
		geoSub := optStrField(f, 3, "geomorprosubsurf")
		if geoSub == nil {
			geoSub = combineVenusTerrain(3, f["terrainugeo1"], f["terrainugeo2"], f["terrainugeo3"])
		}

		if _, err := envStmt.ExecContext(ctx,
			pNum,
			optStrField(f, 50, "fieldnumber", "sitenumber"),
			projIDVal,
			optStrField(f, 40, "date"),
			optStrField(f, 30, "sitesurveyor"),
			optStrField(f, 255, "location"),
			optStrField(f, 15, "fsregiondistrict", "region"),
			optStrField(f, 8, "ntsmapsheet"),
			optStrField(f, 2, "utmzone"),
			optFltField(f, "utmnorthing"),
			optFltField(f, "utmeasting"),
			lat, lon,
			optStrField(f, 20, "airphotonum"),
			optFltField(f, "xcoord"),
			optFltField(f, "ycoord"),
			optStrField(f, 15, "mapunit"),
			optStrField(f, 255, "plotrepresenting"),
			optStrField(f, 4, "zone"),
			optStrField(f, 8, "subzone"),
			optStrField(f, 5, "siteseries"),
			optStrField(f, 3, "transdistrib"),
			optStrField(f, 3, "ecosection"),
			optStrField(f, 3, "moistureregime"),
			optStrField(f, 3, "moistureregimesub"),
			optStrField(f, 2, "nutrientregime"),
			optStrField(f, 3, "successionalstatus"),
			optStrField(f, 6, "structuralstage"),
			optStrField(f, 5, "realmclass"),
			optStrField(f, 8, "sitedisturbance1"),
			optStrField(f, 8, "sitedisturbance2"),
			optIntField(f, "elevation"),
			optFltField(f, "slopegradient", "slope"),
			optIntField(f, "aspect"),
			optStrField(f, 3, "mesoslopeposition"),
			optStrField(f, 3, "surfaceshape", "surfacetopography"),
			optStrField(f, 2, "surfacetopographysize", "surfacesize"),
			optStrField(f, 3, "surfacetopographytype", "surfacetype"),
			optStrField(f, 2, "exposure1", "exposure"),
			optStrField(f, 50, "photo", "photorollnum"),
			optFltField(f, "substratedecwood"),
			optFltField(f, "substratebedrock"),
			optFltField(f, "substraterocks"),
			optFltField(f, "substratemineralsoil"),
			optFltField(f, "substrateorganicmatter"),
			optFltField(f, "substratewater"),
			optStrField(f, 0, "sitenotes"),
			scHerb,
			scMoss,
			scShrub,
			scTree,
			optStrField(f, 30, "vegsurveyor"),
			optStrField(f, 0, "vegnotes"),
			optStrField(f, 4, "bedrockgeology1", "bedrockgeology"),
			optStrField(f, 4, "bedrockgeology2"),
			optStrField(f, 4, "bedrockgeology3"),
			optStrField(f, 12, "coarsefraglith1", "coarsefraglith"),
			optStrField(f, 12, "coarsefraglith2"),
			optStrField(f, 12, "coarsefraglith3"),
			optStrField(f, 30, "soilsurveyor"),
			optStrField(f, 4, "soilclassgroup"),
			optStrField(f, 4, "soilclasssubgroup"),
			optStrField(f, 4, "humusform"),
			optStrField(f, 3, "hydrogeosystem", "hydrogeo"),
			optIntField(f, "rootingdepth"),
			rzpsAny,
			optIntField(f, "rootrestrictingdepth"),
			optStrField(f, 1, "rootrestrictingtype"),
			optStrField(f, 5, "watersource"),
			optIntField(f, "seepagedepth"),
			optStrField(f, 5, "soildrainage"),
			optStrField(f, 7, "floodingregimefreq", "floodingregime"),
			optStrField(f, 3, "phmethodcodeorganic"),
			optStrField(f, 3, "phmethodcodemineral"),
			optStrField(f, 0, "soilnotes"),
			tTexSurf, tTexSub, sMatSurf, sMatSub, sExpSurf, sExpSub, geoSurf, geoSub, svCF,
		); err != nil {
			return 0, 0, 0, fmt.Errorf("insert VENUS plot %s _Env: %w", pNum, err)
		}

		plotType := "FS882"
		if pl.Graviti {
			plotType = "1"
		}
		qualAny := nullTruncatedUTF16(pl.PlotQuality, 15)
		if _, err := adminStmt.ExecContext(ctx, pNum, plotType, qualAny, optStrField(f, 100, "usersiteunit", "assignedbecunit"), humThick); err != nil {
			return 0, 0, 0, fmt.Errorf("insert VENUS plot %s _Admin: %w", pNum, err)
		}

		// Pivot normalized VENUS VegObservations into wide VP08 _Veg
		type wideVeg struct {
			spp       string
			collected string
			covers    map[string]float64
			heights   map[string]float64
		}
		bySpp := map[string]*wideVeg{}
		var sppOrder []string
		for _, vo := range pl.VegObservations {
			code := truncateUTF16Units(strings.ToUpper(strings.TrimSpace(vo.Species)), 8)
			if code == "" || vo.Cover == 0 {
				continue
			}
			wv := bySpp[code]
			if wv == nil {
				wv = &wideVeg{spp: code, covers: map[string]float64{}, heights: map[string]float64{}}
				bySpp[code] = wv
				sppOrder = append(sppOrder, code)
			}
			if wv.collected == "" && strings.TrimSpace(vo.Collected) != "" {
				wv.collected = truncateUTF16Units(strings.TrimSpace(vo.Collected), 1)
			}
			cCol, hCol := resolveIncomingTurboVegLayer(vo.Layer, "")
			if err := accumulateImportedSingleCover(wv.covers, cCol, vo.Cover, pNum, code); err != nil {
				return 0, 0, 0, err
			}
			if vo.Height != nil && hCol != "" && *vo.Height > wv.heights[hCol] {
				wv.heights[hCol] = *vo.Height
			}
		}

		for _, code := range sppOrder {
			wv := bySpp[code]
			args := make([]any, len(vegColumns))
			args[vegColPos["ID"]] = nextVegID
			nextVegID++
			args[vegColPos["PlotNumber"]] = pNum
			args[vegColPos["Species"]] = wv.spp
			args[vegColPos["Collected"]] = nullEmpty(wv.collected)
			for cCol, cv := range wv.covers {
				if pos, ok := vegColPos[cCol]; ok {
					args[pos] = cv
				}
			}
			for hCol, hv := range wv.heights {
				if pos, ok := vegColPos[hCol]; ok {
					if hCol == "HeightB" {
						args[pos] = strconv.FormatFloat(hv, 'f', -1, 64)
					} else {
						args[pos] = hv
					}
				}
			}
			if _, err := vegStmt.ExecContext(ctx, args...); err != nil {
				return 0, 0, 0, fmt.Errorf("insert VENUS veg %s/%s: %w", pNum, wv.spp, err)
			}
			vegCount++
		}

		for _, hr := range pl.HumusRows {
			hzAny := optStrField(hr, 8, "horizon")
			hzStr, _ := hzAny.(string)
			rawU := optFltField(hr, "depthupper", "upperdepth")
			rawL := optFltField(hr, "depthlower", "lowerdepth")
			normU, normL, qReason := normalizeLMH25HorizonDepths(hzStr, false, rawU, rawL)
			id := nextHumusID
			nextHumusID++
			if qReason != "" {
				if err := quarantineSoilHorizon("Humus", id, pNum, hzAny, rawU, rawL, qReason, hr); err != nil {
					return 0, 0, 0, err
				}
				continue
			}
			if _, err := humusStmt.ExecContext(ctx,
				id,
				pNum,
				hzAny,
				normU,
				normL,
				optStrField(hr, 5, "structurekind", "humusstructurekind"),
				optStrField(hr, 1, "structuredegree", "humusstructuredegree"),
				optIntField(hr, "vonpost"),
				optStrField(hr, 1, "mycelabundance"),
				optStrField(hr, 1, "fecalabundance"),
				optStrField(hr, 6, "rootsabundance"),
				optStrField(hr, 6, "rootssize"),
				optFltField(hr, "phorganic", "humusformph"),
				optStrField(hr, 0, "comment"),
			); err != nil {
				return 0, 0, 0, fmt.Errorf("insert VENUS humus %s: %w", pNum, err)
			}
			humusCount++
		}

		for _, mr := range pl.MineralRows {
			hzAny := optStrField(mr, 8, "horizon")
			hzStr, _ := hzAny.(string)
			rawU := optFltField(mr, "depthupper", "upperdepth")
			rawL := optFltField(mr, "depthlower", "lowerdepth")
			normU, normL, qReason := normalizeLMH25HorizonDepths(hzStr, true, rawU, rawL)
			id := nextMineralID
			nextMineralID++
			if qReason != "" {
				if err := quarantineSoilHorizon("Mineral", id, pNum, hzAny, rawU, rawL, qReason, mr); err != nil {
					return 0, 0, 0, err
				}
				continue
			}
			if _, err := mineralStmt.ExecContext(ctx,
				id,
				pNum,
				hzAny,
				normU,
				normL,
				optStrField(mr, 14, "colour"),
				optIntField(mr, "colouraspect", "asp"),
				optStrField(mr, 4, "texture"),
				optIntField(mr, "percentcoarsefragsgravel"),
				optIntField(mr, "percentcoarsefragscobbles"),
				optIntField(mr, "percentcoarsefragsstones"),
				optIntField(mr, "percentcoarsefragstotal"),
				optStrField(mr, 1, "percentcoarsefragsshape"),
				optStrField(mr, 6, "rootsabundance"),
				optStrField(mr, 6, "rootssize"),
				optStrField(mr, 3, "structureclass", "mineralstructureclass"),
				optStrField(mr, 7, "structurekind", "mineralstructurekind"),
				optFltField(mr, "phmineral", "mineralformph"),
				optStrField(mr, 0, "comments", "comment"),
			); err != nil {
				return 0, 0, 0, fmt.Errorf("insert VENUS mineral %s: %w", pNum, err)
			}
			mineralCount++
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO `+auditTbl+` ("Project","User","PlotNumber","Table","EditField","EditWhen","Restore","Flag")
			VALUES (?, 'VPro', ?, 'Environment', ?, ?, 0, 0)`,
			targetProject, pNum, fmt.Sprintf("Imported VENUS ver. %s record. Project: %s", venusVer, sourceProject), nowStr); err != nil {
			return 0, 0, 0, fmt.Errorf("insert VENUS audit for plot %s: %w", pNum, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO `+metaTbl+` ("ID", "ProjectID", "ProjectTitle", "NumberOfFS882Plots", "VegCoverMethod", "Datum")
		VALUES (1, ?, ?, ?, '1', '2')`,
		projIDVal, sourceProject+" (VENUS "+venusVer+" Import)", len(seenPlots))
	return vegCount, humusCount, mineralCount, err
}
