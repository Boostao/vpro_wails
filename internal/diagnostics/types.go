package diagnostics

// SMRRows defines the canonical 9 BEC Relative Soil Moisture Regime rows (0 = Very Xeric .. 8 = Hydric).
var SMRRows = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"}

// SNRCols defines the canonical 6 BEC Relative Soil Nutrient Regime columns (A = Very Poor .. F = Alkalitrophic).
var SNRCols = []string{"A", "B", "C", "D", "E", "F"}

// DefaultExcludedUnitModifiers defines standard BEC non-typological working modifiers excluded by default.
var DefaultExcludedUnitModifiers = []string{".support", ".poor", ".nudum"}

// SynthesisLumpTableOption describes an available *_Lump table for species folding in synthesis.
type SynthesisLumpTableOption struct {
	Name        string `json:"name"`
	TableName   string `json:"tableName"`
	DBPath      string `json:"dbPath"`
	RowCount    int    `json:"rowCount"`
	ActiveCount int    `json:"activeCount"`
}

// PairwiseParams configures BEC pairwise differential potential & comparison (porting _do_pairwise.R).
type PairwiseParams struct {
	MinConstancy   float64 `json:"minConstancy,omitempty"`
	NoiseConstancy float64 `json:"noiseConstancy,omitempty"`
	MinPlots       int     `json:"minPlots,omitempty"`
	MinDiff        float64 `json:"minDiff,omitempty"`
	MinDD          float64 `json:"minDd,omitempty"`
	MinorCover     float64 `json:"minorCover,omitempty"`
	DominantCover  float64 `json:"dominantCover,omitempty"`
}

// SynthesisOptions configures ComputeSynthesisSnapshot.
type SynthesisOptions struct {
	UnitSource               string         `json:"unitSource,omitempty"`               // "auto" (default) | "su" | "user_site_unit"
	LumpTable                string         `json:"lumpTable,omitempty"`                // optional <Name> or <Name>_Lump
	LumpDBPath               string         `json:"lumpDbPath,omitempty"`               // optional external SQLite DB path for LumpTable
	ExcludeStandardModifiers bool           `json:"excludeStandardModifiers"`           // exclude .support, .poor, .nudum
	ExcludePatterns          []string       `json:"excludePatterns,omitempty"`          // optional extra unit exclusion patterns
	SignificanceBasis        string         `json:"significanceBasis,omitempty"`        // "all_plots" (default) | "present_plots"
	UnitOrder                []string       `json:"unitOrder,omitempty"`                // optional explicit unit column order
	MinMaxPresencePct        float64        `json:"minMaxPresencePct,omitempty"`        // optional display threshold (0 keeps all)
	Pairwise                 PairwiseParams `json:"pairwise,omitempty"`                 // optional BEC do_pairwise overrides
}

// OrderedUnitCode represents a single ordered (Unit, PresSignifCode) pair for V7mdlDiagnostic.Diagnostic.
type OrderedUnitCode struct {
	Unit string
	Code *string
}

// DiagnosticClassification captures the exact V7mdlDiagnostic.Diagnostic / vpro_diagnostic_classify output.
type DiagnosticClassification struct {
	Unit      string   `json:"unit"`
	Diagnosis string   `json:"diagnosis"`
	Codes     []string `json:"codes"`
}

// SpeciesTaxonMeta holds taxonomy attributes resolved from VLists.USysAllSpecs and VUser.USysUserSpp.
type SpeciesTaxonMeta struct {
	Species         string `json:"species"`
	ScientificName  string `json:"scientificName"`
	EnglishName     string `json:"englishName"`
	Lifeform        *int   `json:"lifeform"`
	LifeformName    string `json:"lifeformName"`
	StratumGroup    string `json:"stratumGroup"` // "Tree" | "Shrub" | "Herb" | "Moss" | "Unclassified"
	SourceCatalogue string `json:"sourceCatalogue"`
}

// ExcludedUnitSummary reports units filtered out by .support / .poor / .nudum or custom patterns.
type ExcludedUnitSummary struct {
	SiteUnit       string `json:"siteUnit"`
	MatchedPattern string `json:"matchedPattern"`
	PlotCount      int    `json:"plotCount"`
}

// SynthesisUnitPlot summarizes an individual plot assigned to a SiteUnit (or unassigned).
type SynthesisUnitPlot struct {
	PlotNumber     string `json:"plotNumber"`
	FieldNumber    string `json:"fieldNumber"`
	SiteUnit       string `json:"siteUnit"`
	Zone           string `json:"zone"`
	SubZone        string `json:"subZone"`
	SiteSeries     string `json:"siteSeries"`
	UserSiteUnit   string `json:"userSiteUnit"`
	MoistureRegime string `json:"moistureRegime"`
	NutrientRegime string `json:"nutrientRegime"`
}

// SynthesisUnitSummary describes one ordered SiteUnit column in the synthesis matrix.
type SynthesisUnitSummary struct {
	SiteUnit         string              `json:"siteUnit"`
	LongName         string              `json:"longName"`
	PlotCount        int                 `json:"plotCount"`
	SpeciesRichness  int                 `json:"speciesRichness"`
	DiagnosticCount  int                 `json:"diagnosticCount"`
	ConstantCount    int                 `json:"constantCount"`
	DiagPotentialSum float64             `json:"diagPotentialSum"`
	Plots            []SynthesisUnitPlot `json:"plots"`
}

// SynthesisUnitSpeciesCell holds exact constancy, cover, class, and diagnostic values for (Species, SiteUnit).
type SynthesisUnitSpeciesCell struct {
	SiteUnit                string   `json:"siteUnit"`
	PresentPlots            int      `json:"presentPlots"`
	PlotCount               int      `json:"plotCount"`
	PresencePct             float64  `json:"presencePct"`
	PresencePctText         string   `json:"presencePctText"`
	PresenceClass           int      `json:"presenceClass"` // 0 when absent, 1..5 when present
	PresenceRoman           string   `json:"presenceRoman"` // "" when absent, "I".."V" when present
	MeanCoverAll            float64  `json:"meanCoverAll"`
	MeanCoverAllText        string   `json:"meanCoverAllText"`
	MeanCoverPresent        float64  `json:"meanCoverPresent"`
	MeanCoverPresentText    string   `json:"meanCoverPresentText"`
	SignifClass             string   `json:"signifClass"`   // "" when absent, "+", "1".."9" when present
	SignifNumeric           int      `json:"signifNumeric"` // 0 for absent/"+", 1..9
	CrosstabCode            string   `json:"crosstabCode"`  // e.g. "5 - 6" or ""
	DiagnosticPotential     float64  `json:"diagnosticPotential"`
	IsPrimaryDiagnosticUnit bool     `json:"isPrimaryDiagnosticUnit"`
	CellDiagnosticCodes     []string `json:"cellDiagnosticCodes"`
}

// SynthesisSpeciesRow represents one row in the BEC Differential & Diagnostic Matrix.
type SynthesisSpeciesRow struct {
	Species         string                     `json:"species"`
	ScientificName  string                     `json:"scientificName"`
	EnglishName     string                     `json:"englishName"`
	Lifeform        *int                       `json:"lifeform"`
	LifeformName    string                     `json:"lifeformName"`
	StratumGroup    string                     `json:"stratumGroup"`
	SourceSpecies   []string                   `json:"sourceSpecies,omitempty"`
	DiagnosticUnit  string                     `json:"diagnosticUnit"`
	Diagnosis       string                     `json:"diagnosis"`
	DiagnosticCodes []string                   `json:"diagnosticCodes"`
	MaxPresencePct  float64                    `json:"maxPresencePct"`
	MaxMeanCover    float64                    `json:"maxMeanCover"`
	Cells           []SynthesisUnitSpeciesCell `json:"cells"`
}

// PairwiseMatrixCell holds one cell in the symmetric U x U pairwise unit similarity matrix.
type PairwiseMatrixCell struct {
	Unit              string  `json:"unit"`
	Similarity        float64 `json:"similarity"`        // 0..1 Sorensen / Bray-Curtis
	SimilarityPct     float64 `json:"similarityPct"`     // 0..100
	SimilarityPctText string  `json:"similarityPctText"` // ACE 15-sig-digit rounded text
	BECSimMean        float64 `json:"becSimMean"`        // 0..1 BEC differential similarity
	BECSimMeanPct     float64 `json:"becSimMeanPct"`     // 0..100
}

// PairwiseMatrixRow represents one row of the symmetric pairwise similarity matrix.
type PairwiseMatrixRow struct {
	Unit  string               `json:"unit"`
	Cells []PairwiseMatrixCell `json:"cells"`
}

// PairwiseDifferentialTaxon captures a species differentiating Unit1 vs Unit2 in do_pairwise.
type PairwiseDifferentialTaxon struct {
	Species        string  `json:"species"`
	ScientificName string  `json:"scientificName"`
	ConstancyUnit1 float64 `json:"constancyUnit1"`
	MeanCoverUnit1 float64 `json:"meanCoverUnit1"`
	ConstancyUnit2 float64 `json:"constancyUnit2"`
	MeanCoverUnit2 float64 `json:"meanCoverUnit2"`
	DiffPtsUnit1   float64 `json:"diffPtsUnit1"`
	DiffPtsUnit2   float64 `json:"diffPtsUnit2"`
	FavoursUnit    string  `json:"favoursUnit"`
}

// PairwiseUnitComparison captures full pairwise comparison metrics between Unit1 and Unit2.
type PairwiseUnitComparison struct {
	Unit1               string                      `json:"unit1"`
	Unit2               string                      `json:"unit2"`
	PlotsUnit1          int                         `json:"plotsUnit1"`
	PlotsUnit2          int                         `json:"plotsUnit2"`
	SorensenSimilarity  float64                     `json:"sorensenSimilarity"`
	SorensenPct         float64                     `json:"sorensenPct"`
	SharedDiagPotential float64                     `json:"sharedDiagPotential"`
	DiffTotalUnit1      float64                     `json:"diffTotalUnit1"`
	DiffTotalUnit2      float64                     `json:"diffTotalUnit2"`
	DiffRatio           float64                     `json:"diffRatio"`
	BECSimMin           float64                     `json:"becSimMin"`
	BECSimMean          float64                     `json:"becSimMean"`
	BECSimMax           float64                     `json:"becSimMax"`
	DifferentialTaxa    []PairwiseDifferentialTaxon `json:"differentialTaxa"`
}

// OutlierPlotCandidate describes a classified plot evaluated by Leave-One-Out centroid similarity.
type OutlierPlotCandidate struct {
	Rank               int     `json:"rank"`
	PlotNumber         string  `json:"plotNumber"`
	FieldNumber        string  `json:"fieldNumber"`
	AssignedUnit       string  `json:"assignedUnit"`
	AssignedUnitPlots  int     `json:"assignedUnitPlots"`
	SingletonUnit      bool    `json:"singletonUnit"`
	OwnSimilarity      float64 `json:"ownSimilarity"`      // 0..1 LOO Sorensen to AssignedUnit centroid
	OwnSimilarityPct   float64 `json:"ownSimilarityPct"`   // 0..100
	NearestRivalUnit   string  `json:"nearestRivalUnit"`   // "" if <2 units exist
	RivalSimilarity    float64 `json:"rivalSimilarity"`    // 0..1 Sorensen to NearestRivalUnit centroid
	RivalSimilarityPct float64 `json:"rivalSimilarityPct"` // 0..100
	DeltaSimilarity    float64 `json:"deltaSimilarity"`    // RivalSimilarity - OwnSimilarity
	DeltaSimilarityPct float64 `json:"deltaSimilarityPct"` // (RivalSimilarity - OwnSimilarity) * 100
	Misplaced          bool    `json:"misplaced"`          // DeltaSimilarity > 0 && NearestRivalUnit != ""
	Top5PercentAnomaly bool    `json:"top5PercentAnomaly"` // top ceil(5%) of sorted outlier queue
	MoistureRegime     string  `json:"moistureRegime"`
	NutrientRegime     string  `json:"nutrientRegime"`
	Zone               string  `json:"zone"`
	SubZone            string  `json:"subZone"`
	SiteSeries         string  `json:"siteSeries"`
}

// EdatopicCell represents one (rSMR, rSNR) cell in a 9 x 6 Relative Edatopic Grid.
type EdatopicCell struct {
	SMR         string   `json:"smr"`        // "0".."8"
	SNR         string   `json:"snr"`        // "A".."F"
	PlotCount   int      `json:"plotCount"`
	UnitPct     float64  `json:"unitPct"`    // % of all plots in unit (0..100)
	Prominence  float64  `json:"prominence"` // % of edatopically-attributed plots in unit (0..100)
	PlotNumbers []string `json:"plotNumbers"`
}

// EdatopicUnitGrid holds the 9 x 6 (rSMR 0..8 x rSNR A..F) heatmap for a SiteUnit or Overall.
type EdatopicUnitGrid struct {
	SiteUnit             string         `json:"siteUnit"` // "__OVERALL__" for overall project grid
	Label                string         `json:"label"`
	TotalPlots           int            `json:"totalPlots"`
	PlottedEdatopicCount int            `json:"plottedEdatopicCount"`
	MissingEdatopicCount int            `json:"missingEdatopicCount"`
	MaxCellCount         int            `json:"maxCellCount"`
	MaxProminence        float64        `json:"maxProminence"`
	Cells                []EdatopicCell `json:"cells"` // 54 cells in row-major order (SMR 0..8, SNR A..F)
}

// SynthesisUndoEntry summarizes the latest reversible reassignment in __VPRO_SynthesisSUHistory.
type SynthesisUndoEntry struct {
	HistoryID      int64  `json:"historyId"`
	PlotNumber     string `json:"plotNumber"`
	BeforeSiteUnit string `json:"beforeSiteUnit"`
	AfterSiteUnit  string `json:"afterSiteUnit"`
	Created        string `json:"created"`
}

// SynthesisSnapshot is the complete reactive payload for the BEC Synthesis Workbench.
type SynthesisSnapshot struct {
	ContextID           string                   `json:"contextId"`
	Project             string                   `json:"project"`
	SU                  string                   `json:"su"`
	SUTableName         string                   `json:"suTableName"`
	UnitSourceUsed      string                   `json:"unitSourceUsed"` // "su" | "user_site_unit"
	SUWritable          bool                     `json:"suWritable"`
	SUReadOnlyReason    string                   `json:"suReadOnlyReason,omitempty"`
	LumpTableUsed       string                   `json:"lumpTableUsed,omitempty"`
	LumpFoldedTaxa      int                      `json:"lumpFoldedTaxa"`
	LumpCappedCells     int                      `json:"lumpCappedCells"`
	SignificanceBasis   string                   `json:"significanceBasis"`
	TotalProjectPlots   int                      `json:"totalProjectPlots"`
	ClassifiedPlotCount int                      `json:"classifiedPlotCount"`
	UnassignedPlotCount int                      `json:"unassignedPlotCount"`
	ExcludedPlotCount   int                      `json:"excludedPlotCount"`
	ComputeMs           float64                  `json:"computeMs"`
	ExcludedUnits       []ExcludedUnitSummary    `json:"excludedUnits"`
	UnassignedPlots     []SynthesisUnitPlot      `json:"unassignedPlots"`
	Units               []SynthesisUnitSummary   `json:"units"`
	Species             []SynthesisSpeciesRow    `json:"species"`
	SimilarityMatrix    []PairwiseMatrixRow      `json:"similarityMatrix"`
	PairwiseComparisons []PairwiseUnitComparison `json:"pairwiseComparisons"`
	OutlierQueue        []OutlierPlotCandidate   `json:"outlierQueue"`
	OverallEdatopicGrid EdatopicUnitGrid         `json:"overallEdatopicGrid"`
	UnitEdatopicGrids   []EdatopicUnitGrid       `json:"unitEdatopicGrids"`
	CanUndo             bool                     `json:"canUndo"`
	UndoDepth           int                      `json:"undoDepth"`
	LatestUndo          *SynthesisUndoEntry      `json:"latestUndo,omitempty"`
}

// ReassignSynthesisPlotRequest requests atomic plot -> TargetSiteUnit reassignment + instant recompute.
type ReassignSynthesisPlotRequest struct {
	PlotNumber     string           `json:"plotNumber"`
	TargetSiteUnit string           `json:"targetSiteUnit"`
	Options        SynthesisOptions `json:"options"`
}

// UndoSynthesisReassignmentRequest requests undoing a synthesis SU reassignment + instant recompute.
type UndoSynthesisReassignmentRequest struct {
	HistoryID int64            `json:"historyId,omitempty"` // 0 = undo latest reversible synthesis reassignment
	Options   SynthesisOptions `json:"options"`
}

// ReassignSynthesisPlotResult returns the provenance receipt, elapsed latencyMs, and updated SynthesisSnapshot.
type ReassignSynthesisPlotResult struct {
	ContextID      string             `json:"contextId"`
	SU             string             `json:"su"`
	TableName      string             `json:"tableName"`
	PlotNumber     string             `json:"plotNumber"`
	BeforeSiteUnit string             `json:"beforeSiteUnit"`
	AfterSiteUnit  string             `json:"afterSiteUnit"`
	Changed        bool               `json:"changed"`
	HistoryID      int64              `json:"historyId"`
	LatencyMs      float64            `json:"latencyMs"`
	Snapshot       *SynthesisSnapshot `json:"snapshot"`
}

// ExportDiagnosticWorkbookRequest configures publication of a Diagnostic10.xla-compatible .xlsx workbook.
type ExportDiagnosticWorkbookRequest struct {
	OutputPath              string           `json:"outputPath"`
	ReportTitle             string           `json:"reportTitle,omitempty"`
	IncludeCommonNames      bool             `json:"includeCommonNames"`
	DiagnosticSheetName     string           `json:"diagnosticSheetName,omitempty"`
	SimilaritySheetName     string           `json:"similaritySheetName,omitempty"`
	OutlierSheetName        string           `json:"outlierSheetName,omitempty"`
	EdatopicSheetName       string           `json:"edatopicSheetName,omitempty"`
	IncludeUnitEdatopicTabs bool             `json:"includeUnitEdatopicTabs,omitempty"`
	Options                 SynthesisOptions `json:"options"`
}

// ExportDiagnosticWorkbookResult reports the atomically published .xlsx artifact metadata.
type ExportDiagnosticWorkbookResult struct {
	Path        string   `json:"path"`
	SHA256      string   `json:"sha256"`
	SheetNames  []string `json:"sheetNames"`
	UnitCount   int      `json:"unitCount"`
	SpeciesRows int      `json:"speciesRows"`
	OutlierRows int      `json:"outlierRows"`
	LatencyMs   float64  `json:"latencyMs"`
}
