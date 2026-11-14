import { Call } from '@wailsio/runtime';
import type { CancellableRead } from '../shared/readRequests';

export const EDATOPIC_SMR_ROWS = ['0', '1', '2', '3', '4', '5', '6', '7', '8'] as const;
export const EDATOPIC_SNR_COLS = ['A', 'B', 'C', 'D', 'E', 'F'] as const;

export const SMR_DESCRIPTIONS: Record<string, string> = {
  '0': 'Very Xeric',
  '1': 'Xeric',
  '2': 'Subxeric',
  '3': 'Submesic',
  '4': 'Mesic',
  '5': 'Subhygric',
  '6': 'Hygric',
  '7': 'Subhydric',
  '8': 'Hydric',
};

export const SNR_DESCRIPTIONS: Record<string, string> = {
  A: 'Very Poor',
  B: 'Poor',
  C: 'Medium',
  D: 'Rich',
  E: 'Very Rich',
  F: 'Alkalitrophic',
};

export const DIAGNOSTIC_CODE_META: Record<
  string,
  { label: string; shortLabel: string; badgeClass: string }
> = {
  d: {
    label: 'Differential (Constancy >= III, >= 2 classes above others)',
    shortLabel: 'd',
    badgeClass: 'diag-badge-d',
  },
  dd: {
    label: 'Dominant Differential (Significance >= 5, >= 2 classes above others)',
    shortLabel: 'dd',
    badgeClass: 'diag-badge-dd',
  },
  cd: {
    label: 'Constant Dominant (Constancy V, Significance >= 5)',
    shortLabel: 'cd',
    badgeClass: 'diag-badge-cd',
  },
  c: {
    label: 'Constant (Constancy V, Significance < 5)',
    shortLabel: 'c',
    badgeClass: 'diag-badge-c',
  },
  ic: {
    label: 'Important Companion (Constancy >= II, unique presence)',
    shortLabel: 'ic',
    badgeClass: 'diag-badge-ic',
  },
};

export type MatrixDisplayMode = 'roman_signif' | 'pct_cover' | 'crosstab';
export type MatrixGroupingMode = 'diagnostic_unit' | 'stratum' | 'lifeform' | 'flat';
export type SynthesisWorkspaceTab = 'matrix' | 'similarity' | 'edatopic';
export type OutlierFilterMode = 'all' | 'misplaced' | 'top5';

export interface SynthesisLumpTableOption {
  name: string;
  tableName: string;
  dbPath: string;
  rowCount: number;
  activeCount: number;
}

export interface PairwiseParams {
  minConstancy?: number;
  noiseConstancy?: number;
  minPlots?: number;
  minDiff?: number;
  minDd?: number;
  minorCover?: number;
  dominantCover?: number;
}

export interface SynthesisOptions {
  unitSource?: 'auto' | 'su' | 'user_site_unit';
  lumpTable?: string;
  lumpDbPath?: string;
  excludeStandardModifiers: boolean;
  excludePatterns?: string[];
  significanceBasis?: 'all_plots' | 'present_plots';
  unitOrder?: string[];
  minMaxPresencePct?: number;
  pairwise?: PairwiseParams;
}

export interface ExcludedUnitSummary {
  siteUnit: string;
  matchedPattern: string;
  plotCount: number;
}

export interface SynthesisUnitPlot {
  plotNumber: string;
  fieldNumber: string;
  siteUnit: string;
  zone: string;
  subZone: string;
  siteSeries: string;
  userSiteUnit: string;
  moistureRegime: string;
  nutrientRegime: string;
}

export interface SynthesisUnitSummary {
  siteUnit: string;
  longName: string;
  plotCount: number;
  speciesRichness: number;
  diagnosticCount: number;
  constantCount: number;
  diagPotentialSum: number;
  plots: SynthesisUnitPlot[];
}

export interface SynthesisUnitSpeciesCell {
  siteUnit: string;
  presentPlots: number;
  plotCount: number;
  presencePct: number;
  presencePctText: string;
  presenceClass: number;
  presenceRoman: string;
  meanCoverAll: number;
  meanCoverAllText: string;
  meanCoverPresent: number;
  meanCoverPresentText: string;
  signifClass: string;
  signifNumeric: number;
  crosstabCode: string;
  diagnosticPotential: number;
  isPrimaryDiagnosticUnit: boolean;
  cellDiagnosticCodes: string[];
}

export interface SynthesisSpeciesRow {
  species: string;
  scientificName: string;
  englishName: string;
  lifeform: number | null;
  lifeformName: string;
  stratumGroup: string;
  sourceSpecies?: string[];
  diagnosticUnit: string;
  diagnosis: string;
  diagnosticCodes: string[];
  maxPresencePct: number;
  maxMeanCover: number;
  cells: SynthesisUnitSpeciesCell[];
}

export interface PairwiseMatrixCell {
  unit: string;
  similarity: number;
  similarityPct: number;
  similarityPctText: string;
  becSimMean: number;
  becSimMeanPct: number;
}

export interface PairwiseMatrixRow {
  unit: string;
  cells: PairwiseMatrixCell[];
}

export interface PairwiseDifferentialTaxon {
  species: string;
  scientificName: string;
  constancyUnit1: number;
  meanCoverUnit1: number;
  constancyUnit2: number;
  meanCoverUnit2: number;
  diffPtsUnit1: number;
  diffPtsUnit2: number;
  favoursUnit: string;
}

export interface PairwiseUnitComparison {
  unit1: string;
  unit2: string;
  plotsUnit1: number;
  plotsUnit2: number;
  sorensenSimilarity: number;
  sorensenPct: number;
  sharedDiagPotential: number;
  diffTotalUnit1: number;
  diffTotalUnit2: number;
  diffRatio: number;
  becSimMin: number;
  becSimMean: number;
  becSimMax: number;
  differentialTaxa: PairwiseDifferentialTaxon[];
}

export interface OutlierPlotCandidate {
  rank: number;
  plotNumber: string;
  fieldNumber: string;
  assignedUnit: string;
  assignedUnitPlots: number;
  singletonUnit: boolean;
  ownSimilarity: number;
  ownSimilarityPct: number;
  nearestRivalUnit: string;
  rivalSimilarity: number;
  rivalSimilarityPct: number;
  deltaSimilarity: number;
  deltaSimilarityPct: number;
  misplaced: boolean;
  top5PercentAnomaly: boolean;
  moistureRegime: string;
  nutrientRegime: string;
  zone: string;
  subZone: string;
  siteSeries: string;
}

export interface EdatopicCell {
  smr: string;
  snr: string;
  plotCount: number;
  unitPct: number;
  prominence: number;
  plotNumbers: string[];
}

export interface EdatopicUnitGrid {
  siteUnit: string;
  label: string;
  totalPlots: number;
  plottedEdatopicCount: number;
  missingEdatopicCount: number;
  maxCellCount: number;
  maxProminence: number;
  cells: EdatopicCell[];
}

export interface SynthesisUndoEntry {
  historyId: number;
  plotNumber: string;
  beforeSiteUnit: string;
  afterSiteUnit: string;
  created: string;
}

export interface SynthesisSnapshot {
  contextId: string;
  project: string;
  su: string;
  suTableName: string;
  unitSourceUsed: 'su' | 'user_site_unit';
  suWritable: boolean;
  suReadOnlyReason?: string;
  lumpTableUsed?: string;
  lumpFoldedTaxa: number;
  lumpCappedCells: number;
  significanceBasis: 'all_plots' | 'present_plots' | string;
  totalProjectPlots: number;
  classifiedPlotCount: number;
  unassignedPlotCount: number;
  excludedPlotCount: number;
  computeMs: number;
  excludedUnits: ExcludedUnitSummary[];
  unassignedPlots: SynthesisUnitPlot[];
  units: SynthesisUnitSummary[];
  species: SynthesisSpeciesRow[];
  similarityMatrix: PairwiseMatrixRow[];
  pairwiseComparisons: PairwiseUnitComparison[];
  outlierQueue: OutlierPlotCandidate[];
  overallEdatopicGrid: EdatopicUnitGrid;
  unitEdatopicGrids: EdatopicUnitGrid[];
  canUndo: boolean;
  undoDepth: number;
  latestUndo?: SynthesisUndoEntry;
}

export interface ReassignSynthesisPlotRequest {
  plotNumber: string;
  targetSiteUnit: string;
  options: SynthesisOptions;
}

export interface UndoSynthesisReassignmentRequest {
  historyId?: number;
  options: SynthesisOptions;
}

export interface ReassignSynthesisPlotResult {
  contextId: string;
  su: string;
  tableName: string;
  plotNumber: string;
  beforeSiteUnit: string;
  afterSiteUnit: string;
  changed: boolean;
  historyId: number;
  latencyMs: number;
  snapshot: SynthesisSnapshot;
}

export interface ExportDiagnosticWorkbookRequest {
  outputPath: string;
  reportTitle?: string;
  includeCommonNames: boolean;
  diagnosticSheetName?: string;
  similaritySheetName?: string;
  outlierSheetName?: string;
  edatopicSheetName?: string;
  includeUnitEdatopicTabs?: boolean;
  options: SynthesisOptions;
}

export interface ExportDiagnosticWorkbookResult {
  path: string;
  sha256: string;
  sheetNames: string[];
  unitCount: number;
  speciesRows: number;
  outlierRows: number;
  latencyMs: number;
}

export interface SynthesisBackendPort {
  listSynthesisLumpTables(contextId: string): CancellableRead<SynthesisLumpTableOption[]>;
  computeSynthesisSnapshot(
    contextId: string,
    opts: SynthesisOptions
  ): CancellableRead<SynthesisSnapshot>;
  reassignSynthesisPlotUnit(
    contextId: string,
    req: ReassignSynthesisPlotRequest
  ): Promise<ReassignSynthesisPlotResult>;
  undoSynthesisReassignment(
    contextId: string,
    req: UndoSynthesisReassignmentRequest
  ): Promise<ReassignSynthesisPlotResult>;
  exportDiagnosticWorkbook(
    contextId: string,
    req: ExportDiagnosticWorkbookRequest
  ): Promise<ExportDiagnosticWorkbookResult>;
}

export const defaultSynthesisBackendPort: SynthesisBackendPort = {
  listSynthesisLumpTables: contextId =>
    Call.ByName('main.ClassificationService.ListSynthesisLumpTables', contextId),
  computeSynthesisSnapshot: (contextId, opts) =>
    Call.ByName('main.ClassificationService.ComputeSynthesisSnapshot', contextId, opts),
  reassignSynthesisPlotUnit: (contextId, req) =>
    Call.ByName('main.ClassificationService.ReassignSynthesisPlotUnit', contextId, req),
  undoSynthesisReassignment: (contextId, req) =>
    Call.ByName('main.ClassificationService.UndoSynthesisReassignment', contextId, req),
  exportDiagnosticWorkbook: (contextId, req) =>
    Call.ByName('main.ClassificationService.ExportDiagnosticWorkbook', contextId, req),
};

export function createDefaultSynthesisOptions(): SynthesisOptions {
  return {
    unitSource: 'auto',
    lumpTable: '',
    excludeStandardModifiers: true,
    excludePatterns: [],
    significanceBasis: 'all_plots',
    minMaxPresencePct: 0,
  };
}

export interface SpeciesMatrixSection {
  key: string;
  title: string;
  subtitle: string;
  rows: SynthesisSpeciesRow[];
}

export function formatSynthesisMatrixCell(
  cell: SynthesisUnitSpeciesCell,
  displayMode: MatrixDisplayMode,
  significanceBasis: string
): string {
  if (!cell || cell.presentPlots <= 0) {
    return '';
  }
  switch (displayMode) {
    case 'crosstab':
      return cell.crosstabCode;
    case 'pct_cover': {
      const cov =
        significanceBasis === 'present_plots' ? cell.meanCoverPresentText : cell.meanCoverAllText;
      return `${cell.presencePctText}% / ${cov}`;
    }
    case 'roman_signif':
    default:
      return `${cell.presenceRoman}-${cell.signifClass}`;
  }
}

export function resolveDiagnosticCellClass(codes: readonly string[]): string {
  if (!codes || codes.length === 0) return '';
  if (codes.includes('d') && (codes.includes('cd') || codes.includes('dd'))) {
    return 'diag-cell-d-dom';
  }
  if (codes.includes('dd')) return 'diag-cell-dd';
  if (codes.includes('d')) return 'diag-cell-d';
  if (codes.includes('cd')) return 'diag-cell-cd';
  if (codes.includes('c')) return 'diag-cell-c';
  if (codes.includes('ic')) return 'diag-cell-ic';
  return '';
}

export function filterAndGroupSpeciesRows(
  snapshot: SynthesisSnapshot | null,
  options: {
    searchText: string;
    diagnosticOnly: boolean;
    grouping: MatrixGroupingMode;
  }
): SpeciesMatrixSection[] {
  if (!snapshot || snapshot.species.length === 0) return [];
  const needle = options.searchText.trim().toLowerCase();

  const filtered = snapshot.species.filter(row => {
    if (options.diagnosticOnly && !row.diagnosticUnit) return false;
    if (!needle) return true;
    const hay = [
      row.species,
      row.scientificName,
      row.englishName,
      row.lifeformName,
      row.stratumGroup,
      row.diagnosticUnit,
      row.diagnosis,
    ]
      .join(' ')
      .toLowerCase();
    return hay.includes(needle);
  });

  if (filtered.length === 0) return [];

  if (options.grouping === 'flat') {
    return [
      {
        key: 'all',
        title: 'All Filtered Taxa',
        subtitle: `${filtered.length} taxa`,
        rows: filtered,
      },
    ];
  }

  if (options.grouping === 'stratum') {
    const order = ['Tree', 'Shrub', 'Herb', 'Moss', 'Unclassified'];
    const buckets = new Map<string, SynthesisSpeciesRow[]>();
    for (const r of filtered) {
      const k = order.includes(r.stratumGroup) ? r.stratumGroup : 'Unclassified';
      const list = buckets.get(k) ?? [];
      list.push(r);
      buckets.set(k, list);
    }
    return order
      .filter(k => (buckets.get(k)?.length ?? 0) > 0)
      .map(k => {
        const rows = buckets.get(k)!;
        return {
          key: `stratum::${k}`,
          title: `${k} Layer`,
          subtitle: `${rows.length} taxa`,
          rows,
        };
      });
  }

  if (options.grouping === 'lifeform') {
    const buckets = new Map<string, { lf: number; rows: SynthesisSpeciesRow[] }>();
    for (const r of filtered) {
      const key = r.lifeformName || 'Unclassified';
      const lf = r.lifeform ?? 999;
      const existing = buckets.get(key);
      if (existing) {
        existing.rows.push(r);
      } else {
        buckets.set(key, { lf, rows: [r] });
      }
    }
    return Array.from(buckets.entries())
      .sort((a, b) => a[1].lf - b[1].lf || a[0].localeCompare(b[0]))
      .map(([name, info]) => ({
        key: `lf::${name}`,
        title: name,
        subtitle: `${info.rows.length} taxa`,
        rows: info.rows,
      }));
  }

  // Default: 'diagnostic_unit' matching V7mdlReportsShortVeg.DiagSpp (~Unit blocks + Non-Diagnostic Species)
  const sections: SpeciesMatrixSection[] = [];
  const byUnit = new Map<string, SynthesisSpeciesRow[]>();
  const nonDiag: SynthesisSpeciesRow[] = [];

  for (const r of filtered) {
    if (r.diagnosticUnit) {
      const list = byUnit.get(r.diagnosticUnit) ?? [];
      list.push(r);
      byUnit.set(r.diagnosticUnit, list);
    } else {
      nonDiag.push(r);
    }
  }

  for (const u of snapshot.units) {
    const rows = byUnit.get(u.siteUnit);
    if (rows && rows.length > 0) {
      sections.push({
        key: `diag::${u.siteUnit}`,
        title: `~${u.siteUnit}`,
        subtitle: u.longName ? `${u.longName} (n=${u.plotCount})` : `n=${u.plotCount}`,
        rows,
      });
    }
  }

  if (nonDiag.length > 0) {
    sections.push({
      key: 'diag::__nondiagnostic__',
      title: 'Non-Diagnostic Species',
      subtitle: `${nonDiag.length} companion taxa`,
      rows: nonDiag,
    });
  }

  return sections;
}

export function filterOutlierQueue(
  queue: readonly OutlierPlotCandidate[],
  mode: OutlierFilterMode,
  searchText = ''
): OutlierPlotCandidate[] {
  const needle = searchText.trim().toLowerCase();
  return queue.filter(item => {
    if (mode === 'misplaced' && !item.misplaced) return false;
    if (mode === 'top5' && !item.top5PercentAnomaly) return false;
    if (!needle) return true;
    const hay = [
      item.plotNumber,
      item.fieldNumber,
      item.assignedUnit,
      item.nearestRivalUnit,
      item.zone,
      item.subZone,
    ]
      .join(' ')
      .toLowerCase();
    return hay.includes(needle);
  });
}

export function findPairwiseComparison(
  comparisons: readonly PairwiseUnitComparison[],
  unitA: string,
  unitB: string
): PairwiseUnitComparison | null {
  if (!unitA || !unitB || unitA === unitB) return null;
  for (const comp of comparisons) {
    if (
      (comp.unit1 === unitA && comp.unit2 === unitB) ||
      (comp.unit1 === unitB && comp.unit2 === unitA)
    ) {
      return comp;
    }
  }
  return null;
}

export function getEdatopicCellMatrix(grid: EdatopicUnitGrid | null): EdatopicCell[][] {
  const rows: EdatopicCell[][] = [];
  for (let r = 0; r < EDATOPIC_SMR_ROWS.length; r++) {
    const smr = EDATOPIC_SMR_ROWS[r];
    const rowCells: EdatopicCell[] = [];
    for (let c = 0; c < EDATOPIC_SNR_COLS.length; c++) {
      const snr = EDATOPIC_SNR_COLS[c];
      const idx = r * EDATOPIC_SNR_COLS.length + c;
      const cell = grid?.cells[idx];
      if (cell && cell.smr === smr && cell.snr === snr) {
        rowCells.push(cell);
      } else {
        rowCells.push({
          smr,
          snr,
          plotCount: 0,
          unitPct: 0,
          prominence: 0,
          plotNumbers: [],
        });
      }
    }
    rows.push(rowCells);
  }
  return rows;
}

export function edatopicHeatLevel(prominence: number, plotCount: number): number {
  if (plotCount <= 0) return 0;
  if (prominence >= 35) return 4;
  if (prominence >= 20) return 3;
  if (prominence >= 10) return 2;
  return 1;
}

export function sanitizeExcelSheetPreviewName(raw: string): string {
  const trimmed = raw.trim().slice(0, 31);
  return trimmed.replace(/[:/\\[\]*?]/g, '-');
}

export function validateWorkbookExportDraft(draft: {
  outputPath: string;
  diagnosticSheetName: string;
  similaritySheetName: string;
  outlierSheetName: string;
  edatopicSheetName: string;
}): string {
  const pathTrim = draft.outputPath.trim();
  if (!pathTrim) {
    return 'Enter an absolute .xlsx destination path.';
  }
  if (!pathTrim.toLowerCase().endsWith('.xlsx')) {
    return 'Destination path must end with .xlsx.';
  }
  const sheets = [
    draft.diagnosticSheetName.trim() || 'Diagnostic Summary',
    draft.similaritySheetName.trim() || 'Pairwise Similarity',
    draft.outlierSheetName.trim() || 'Outlier Queue',
    draft.edatopicSheetName.trim() || 'Edatopic Grid',
  ];
  const seenLower = new Map<string, string>();
  for (const s of sheets) {
    if (s.startsWith("'") || s.endsWith("'")) {
      return `Worksheet name "${s}" cannot start or end with an apostrophe.`;
    }
    const clean = sanitizeExcelSheetPreviewName(s);
    const key = clean.toLowerCase();
    const prev = seenLower.get(key);
    if (prev !== undefined) {
      return `Worksheet names "${prev}" and "${s}" collide (Excel matches sheet names case-insensitively).`;
    }
    seenLower.set(key, s);
  }
  return '';
}
