import { Call } from '@wailsio/runtime';
import { wellFormedUTF16 } from './qualityEditor';

export const CANDIDATE_PREFIX_PATTERN = /^[A-Za-z][A-Za-z0-9_]{0,13}$/;
export const SU_IDENTIFIER_PATTERN = /^[A-Za-z][A-Za-z0-9_]{0,30}$/;

export type ClimateRefPeriod = '1961_1990' | '1991_2020';
export type ClimateGroupBy = 'bgc' | 'site_unit';
export type ClusteringMethod = 'NC' | 'KM' | 'FCM' | 'HCLUST';
export type EcologicalDistance = 'bray' | 'chord' | 'jaccard' | 'kulczynski' | 'euclidean';
export type HierarchicalLinkage = 'ward.D2' | 'average' | 'complete' | 'single';
export type AbundanceTransform = 'sqrt' | 'log1p' | 'hellinger' | 'chord' | 'raw';
export type QuartoGuideType = 'single_unit' | 'bgc_unit';

export interface RPackageStatus {
  name: string;
  installed: boolean;
  version: string;
  required: boolean;
}

export interface RRuntimeStatus {
  available: boolean;
  rscriptPath: string;
  rVersion: string;
  rVersionFull: string;
  platform: string;
  quartoAvailable: boolean;
  quartoPath: string;
  quartoVersion: string;
  climrCachePath: string;
  climrRastersCached: boolean;
  readyForClimate: boolean;
  readyForClustering: boolean;
  readyForQuarto: boolean;
  missingRequired: string[];
  packages: RPackageStatus[];
  setupInstructions: string[];
  diagnosticMessage?: string;
}

export interface PlotClimateRow {
  plotNumber: string;
  coordElevHash: string;
  refPeriod: string;
  source: string;
  latitude: number;
  longitude: number;
  elevation: number;
  bgc: string;
  siteUnit: string;
  slopeGradient?: number;
  aspect?: number;
  rsmr: string;
  rsnr: string;
  mat: number;
  mwmt: number;
  mcmt: number;
  map: number;
  msp: number;
  pas: number;
  cmd: number;
  ppt05: number;
  ppt06: number;
  ppt07: number;
  ppt08: number;
  ppt09: number;
  pptAt: number;
  pptWt: number;
  dd5: number;
  nffd: number;
  moistMonths: number;
  dormantDeficit: number;
  aspectFactor: number;
  cmdTopo: number;
  cmdSite?: number;
  asmr?: number;
  computedAt: string;
}

export interface EdatopicGridCell {
  moisture: string;
  nutrient: string;
  count: number;
  frequency: number;
  meanCmd?: number;
  meanCmdSite?: number;
  plotNumbers: string[];
}

export interface EdatopicGridMatrix {
  kind: 'relative' | 'actual' | string;
  moistureAxis: string[];
  nutrientAxis: string[];
  totalPlots: number;
  modalCell: string;
  cells: EdatopicGridCell[];
}

export interface DualUnitEdatopicSummary {
  groupKey: string;
  bgc: string;
  siteUnit: string;
  plotCount: number;
  meanMat: number;
  meanMap: number;
  meanCmd: number;
  meanAsmr?: number;
  meanShift?: number;
  relativeGrid: EdatopicGridMatrix;
  actualGrid: EdatopicGridMatrix;
}

export interface ClimateEdatopicRequest {
  refPeriod: ClimateRefPeriod;
  groupBy?: ClimateGroupBy;
  bgcFilter?: string;
  siteUnitFilter?: string;
  useActiveSu: boolean;
  plotNumbers?: string[];
  forceRefresh?: boolean;
  forceOfflineModel?: boolean;
}

export interface ClimateEdatopicResult {
  contextId: string;
  project: string;
  activeSu: string;
  refPeriod: string;
  totalScopedPlots: number;
  coordinatizedPlots: number;
  skippedNoCoords: number;
  cacheHits: number;
  cacheMisses: number;
  sourceSummary: Record<string, number>;
  overallRelative: EdatopicGridMatrix;
  overallActual: EdatopicGridMatrix;
  unitSummaries: DualUnitEdatopicSummary[];
  plots: PlotClimateRow[];
}

export interface ClusterIndicatorSpecies {
  species: string;
  scientificName: string;
  englishName: string;
  lifeform: number;
  constancyPct: number;
  meanCover: number;
  indVal: number;
}

export interface ClusterUnitSummary {
  clusterUnit: string;
  isNoise: boolean;
  plotCount: number;
  meanRichness: number;
  meanSilhouette: number;
  indicators: ClusterIndicatorSpecies[];
}

export interface ClusterPlotMembership {
  plotNumber: string;
  clusterUnit: string;
  membership: number;
  isNoise: boolean;
  silhouette: number;
  speciesRichness: number;
}

export interface DendrogramSegment {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
}

export interface DendrogramLeaf {
  plotNumber: string;
  x: number;
  order: number;
  clusterUnit: string;
  isNoise: boolean;
}

export interface DendrogramGeometry {
  maxHeight: number;
  cutHeight: number;
  newick: string;
  segments: DendrogramSegment[];
  leaves: DendrogramLeaf[];
}

export interface ClusteringRequest {
  method: ClusteringMethod;
  distance: EcologicalDistance;
  linkage: HierarchicalLinkage;
  transform: AbundanceTransform;
  k: number;
  fuzziness: number;
  dnoise: number;
  cutHeight?: number;
  applyLump: boolean;
  lumpName?: string;
  useActiveSu: boolean;
  bgcFilter?: string;
  plotNumbers?: string[];
  unitPrefix?: string;
  candidatePrefix?: string;
  materializeCandidate: boolean;
}

export interface ClusteringResult {
  contextId: string;
  project: string;
  activeSu: string;
  method: string;
  distance: string;
  linkage: string;
  transform: string;
  k: number;
  plotCount: number;
  speciesCount: number;
  lumpAppliedRows: number;
  copheneticCorrelation: number;
  meanSilhouette: number;
  noiseCount: number;
  candidateSuName?: string;
  candidateTableName?: string;
  materializedRows: number;
  clusters: ClusterUnitSummary[];
  memberships: ClusterPlotMembership[];
  dendrogram: DendrogramGeometry;
}

export interface PromoteCandidateSURequest {
  candidateSuName: string;
  targetSuName: string;
  excludeNoise: boolean;
  confirmOverwrite: boolean;
  activate: boolean;
}

export interface PromoteCandidateSUResult {
  contextId: string;
  candidateTableName: string;
  targetSuName: string;
  targetTableName: string;
  overwroteExisting: boolean;
  promotedRows: number;
  skippedNoiseRows: number;
  activated: boolean;
  newState?: unknown;
}

export interface QuartoVegSummaryRow {
  layer: string;
  species: string;
  scientificName: string;
  englishName: string;
  constancyPct: number;
  constancyClass: string;
  meanCover: number;
}

export interface QuartoGuideRequest {
  guideType: QuartoGuideType;
  title?: string;
  targetUnit?: string;
  bgcFilter?: string;
  conceptNotes?: string;
  refPeriod?: ClimateRefPeriod;
  minConstancyPct?: number;
  useActiveSu: boolean;
  applyLump: boolean;
  includePdf: boolean;
  destinationPath?: string;
}

export interface QuartoGuideResult {
  contextId: string;
  guideType: string;
  title: string;
  targetUnit: string;
  bgcLabel: string;
  plotCount: number;
  speciesRowCount: number;
  rendererUsed: 'quarto' | 'rmarkdown_fallback' | string;
  htmlPath: string;
  htmlSizeBytes: number;
  htmlSha256: string;
  pdfPath?: string;
  pdfStatus?: string;
  htmlPreview: string;
  vegetationTable: QuartoVegSummaryRow[];
}

export interface RSidecarBackendPort {
  detectRRuntime(): Promise<RRuntimeStatus>;
  getCachedPlotClimate(contextId: string, refPeriod: string): Promise<PlotClimateRow[]>;
  computePlotClimateAndDualEdatopic(contextId: string, request: ClimateEdatopicRequest): Promise<ClimateEdatopicResult>;
  runMultivariateClustering(contextId: string, request: ClusteringRequest): Promise<ClusteringResult>;
  promoteClusterCandidateToSU(contextId: string, request: PromoteCandidateSURequest): Promise<PromoteCandidateSUResult>;
  renderQuartoGuide(contextId: string, request: QuartoGuideRequest): Promise<QuartoGuideResult>;
}

export const defaultRSidecarPort: RSidecarBackendPort = {
  detectRRuntime() {
    return Call.ByName('main.EnvironmentReportService.DetectRRuntime');
  },
  getCachedPlotClimate(contextId: string, refPeriod: string) {
    return Call.ByName('main.EnvironmentReportService.GetCachedPlotClimate', contextId, refPeriod);
  },
  computePlotClimateAndDualEdatopic(contextId: string, request: ClimateEdatopicRequest) {
    return Call.ByName('main.EnvironmentReportService.ComputePlotClimateAndDualEdatopic', contextId, request);
  },
  runMultivariateClustering(contextId: string, request: ClusteringRequest) {
    return Call.ByName('main.EnvironmentReportService.RunMultivariateClustering', contextId, request);
  },
  promoteClusterCandidateToSU(contextId: string, request: PromoteCandidateSURequest) {
    return Call.ByName('main.EnvironmentReportService.PromoteClusterCandidateToSU', contextId, request);
  },
  renderQuartoGuide(contextId: string, request: QuartoGuideRequest) {
    return Call.ByName('main.EnvironmentReportService.RenderQuartoGuide', contextId, request);
  },
};

export function validateClusteringDraft(req: ClusteringRequest): string | null {
  if (!Number.isInteger(req.k) || req.k < 2 || req.k > 50) {
    return 'Cluster count (k) must be an integer between 2 and 50.';
  }
  if (!Number.isFinite(req.fuzziness) || req.fuzziness <= 1.0 || req.fuzziness > 5.0) {
    return 'Fuzziness exponent (m) must be greater than 1.0 and at most 5.0.';
  }
  if (!Number.isFinite(req.dnoise) || req.dnoise < 0.05 || req.dnoise > 5.0) {
    return 'Noise distance (dnoise) must be between 0.05 and 5.0.';
  }
  if (req.unitPrefix !== undefined && req.unitPrefix !== '') {
    if (!wellFormedUTF16(req.unitPrefix) || req.unitPrefix.length > 24 || /[\x00\r\n]/.test(req.unitPrefix)) {
      return 'Cluster unit prefix must be well-formed UTF-16 up to 24 characters.';
    }
  }
  if (req.materializeCandidate && req.candidatePrefix !== undefined && req.candidatePrefix.trim() !== '') {
    const clean = req.candidatePrefix.trim().replace(/_SU$/, '').replace(/_ClusterCandidate$/, '');
    if (!CANDIDATE_PREFIX_PATTERN.test(clean)) {
      return 'Candidate prefix must start with a letter and use ASCII letters, digits, or underscores (max 14 characters).';
    }
    if (clean.toLowerCase().includes('master')) {
      return 'Candidate prefix must not contain reserved word "master".';
    }
  }
  return null;
}

export function validatePromoteCandidateDraft(
  req: PromoteCandidateSURequest,
  activeSu: string
): string | null {
  const cand = req.candidateSuName.trim().replace(/_SU$/, '');
  if (!cand.endsWith('_ClusterCandidate') || !SU_IDENTIFIER_PATTERN.test(cand)) {
    return 'Select or materialize a valid *_ClusterCandidate_SU table before promoting.';
  }
  const target = req.targetSuName.trim().replace(/_SU$/, '');
  if (!target) {
    return 'Target SU table name is required.';
  }
  if (target !== req.targetSuName.trim() && !req.targetSuName.trim().endsWith('_SU')) {
    return 'Target SU name must not contain leading or trailing spaces.';
  }
  if (!SU_IDENTIFIER_PATTERN.test(target)) {
    return 'Target SU name must start with a letter and contain only ASCII letters, digits, or underscores (max 31 characters).';
  }
  const lower = target.toLowerCase();
  if (lower === 'none' || lower === 'usyssutabledynamic' || lower.includes('master')) {
    return `Target SU name "${target}" is reserved or protected and cannot be overwritten.`;
  }
  if (lower.endsWith('_clustercandidate')) {
    return 'Target SU name must be a permanent SU name rather than another _ClusterCandidate staging table.';
  }
  if (activeSu && activeSu !== 'None' && activeSu.toLowerCase() === lower && !req.confirmOverwrite) {
    return `Target "${target}_SU" matches the active SU table; check "Confirm overwrite if target *_SU exists" to proceed.`;
  }
  return null;
}

export function validateQuartoGuideDraft(req: QuartoGuideRequest): string | null {
  if (req.guideType !== 'single_unit' && req.guideType !== 'bgc_unit') {
    return 'Guide type must be "single_unit" or "bgc_unit".';
  }
  if (req.title && (!wellFormedUTF16(req.title) || req.title.length > 200)) {
    return 'Guide title must be well-formed UTF-16 up to 200 characters.';
  }
  if (req.conceptNotes && (!wellFormedUTF16(req.conceptNotes) || req.conceptNotes.length > 16000)) {
    return 'Concept notes must be well-formed UTF-16 up to 16,000 characters.';
  }
  if (req.minConstancyPct !== undefined && (!Number.isFinite(req.minConstancyPct) || req.minConstancyPct < 0 || req.minConstancyPct > 100)) {
    return 'Minimum constancy threshold must be between 0% and 100%.';
  }
  return null;
}

export function edatopicCellColor(count: number, maxCount: number): { bg: string; fg: string } {
  if (!count || count <= 0 || maxCount <= 0) {
    return { bg: '#f8fafc', fg: '#94a3b8' };
  }
  const ratio = count / maxCount;
  if (ratio >= 0.75) {
    return { bg: '#003366', fg: '#ffffff' };
  }
  if (ratio >= 0.4) {
    return { bg: '#1d4ed8', fg: '#ffffff' };
  }
  return { bg: '#dbeafe', fg: '#0f172a' };
}

export function indexEdatopicCells(matrix: EdatopicGridMatrix | null | undefined): {
  byKey: Record<string, EdatopicGridCell>;
  maxCount: number;
} {
  const byKey: Record<string, EdatopicGridCell> = {};
  let maxCount = 0;
  if (!matrix || !Array.isArray(matrix.cells)) {
    return { byKey, maxCount };
  }
  for (const cell of matrix.cells) {
    const key = `${cell.moisture}|${cell.nutrient}`;
    byKey[key] = cell;
    if (cell.count > maxCount) {
      maxCount = cell.count;
    }
  }
  return { byKey, maxCount };
}

export interface ScaledDendrogramLayout {
  width: number;
  height: number;
  cutLineY: number;
  segments: Array<{ x1: number; y1: number; x2: number; y2: number }>;
  leaves: Array<{
    plotNumber: string;
    clusterUnit: string;
    isNoise: boolean;
    cx: number;
    cy: number;
  }>;
}

export function computeDendrogramSVGLayout(
  dendro: DendrogramGeometry | null | undefined,
  customCutHeight?: number
): ScaledDendrogramLayout {
  const padLeft = 48;
  const padRight = 24;
  const padTop = 24;
  const padBottom = 86;
  const plotHeight = 210;

  if (!dendro || !Array.isArray(dendro.leaves) || dendro.leaves.length === 0) {
    return { width: 640, height: padTop + plotHeight + padBottom, cutLineY: padTop + plotHeight * 0.5, segments: [], leaves: [] };
  }

  const leafCount = dendro.leaves.length;
  const stepX = Math.max(18, Math.min(34, Math.floor(760 / Math.max(1, leafCount))));
  const innerWidth = Math.max(480, leafCount * stepX);
  const width = padLeft + innerWidth + padRight;
  const height = padTop + plotHeight + padBottom;
  const maxH = Math.max(0.0001, dendro.maxHeight || 1);

  const mapX = (xVal: number) => {
    if (leafCount <= 1) return padLeft + innerWidth / 2;
    return padLeft + ((xVal - 1) / (leafCount - 1)) * innerWidth;
  };
  const mapY = (hVal: number) => {
    const clamped = Math.max(0, Math.min(maxH, hVal));
    return padTop + plotHeight * (1 - clamped / maxH);
  };

  const effectiveCut =
    customCutHeight !== undefined && Number.isFinite(customCutHeight) && customCutHeight > 0
      ? customCutHeight
      : dendro.cutHeight || maxH * 0.55;

  const segments = (dendro.segments || []).map(seg => ({
    x1: Number(mapX(seg.x1).toFixed(1)),
    y1: Number(mapY(seg.y1).toFixed(1)),
    x2: Number(mapX(seg.x2).toFixed(1)),
    y2: Number(mapY(seg.y2).toFixed(1)),
  }));

  const leaves = dendro.leaves.map(leaf => ({
    plotNumber: leaf.plotNumber,
    clusterUnit: leaf.clusterUnit,
    isNoise: leaf.isNoise,
    cx: Number(mapX(leaf.x).toFixed(1)),
    cy: padTop + plotHeight + 8,
  }));

  return {
    width,
    height,
    cutLineY: Number(mapY(effectiveCut).toFixed(1)),
    segments,
    leaves,
  };
}
