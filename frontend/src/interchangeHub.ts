import { Call } from '@wailsio/runtime';

export interface VenusProjectCandidate {
  projectId: string;
  projectName: string;
  suggestedTarget: string;
  suggestedDestPath: string;
  plotCount: number;
  vegObsCount: number;
  humusRowCount: number;
  mineralRowCount: number;
}

export interface VenusSourceInspection {
  sourcePath: string;
  sourceFormat: string;
  venusVersion: string;
  sourceSha256: string;
  projects: VenusProjectCandidate[];
}

export interface VenusImportRequest {
  sourcePath: string;
  sourceProject?: string;
  targetProject: string;
  destinationPath?: string;
  activateOnSuccess: boolean;
}

export interface VenusImportResult {
  sourcePath: string;
  sourceFormat: string;
  venusVersion: string;
  sourceProject: string;
  targetProject: string;
  destinationPath: string;
  destinationSha256: string;
  plotCount: number;
  vegRowCount: number;
  humusRowCount: number;
  mineralRowCount: number;
  activated: boolean;
  state?: Record<string, unknown> | null;
}

export interface TurboVegExportRequest {
  format: 'xml' | 'sqlite';
  destinationPath: string;
  lumpTable?: string;
  lumpDbPath?: string;
}

export interface TurboVegExportOutcome {
  format: string;
  path: string;
  sha256: string;
  plotCount: number;
  speciesCount: number;
  observationCount: number;
}

export interface TurboVegImportRequest {
  sourcePath: string;
  format?: 'xml' | 'sqlite' | '';
  targetProject: string;
  destinationPath?: string;
  activateOnSuccess: boolean;
}

export interface TurboVegImportOutcome {
  sourcePath: string;
  format: string;
  targetProject: string;
  destinationPath: string;
  destinationSha256: string;
  plotCount: number;
  speciesCount: number;
  observationCount: number;
  activated: boolean;
  state?: Record<string, unknown> | null;
}

export interface UserSpeciesRecord {
  code: string;
  scientificName: string | null;
  lifeForm: number | null;
  englishName: string | null;
  report?: number | null;
  sppNumber?: number | null;
  codetype?: string | null;
}

export interface UserSpeciesExportRequest {
  format: 'json' | 'csv';
  destinationPath?: string;
}

export interface UserSpeciesExportResult {
  format: string;
  rowCount: number;
  path?: string;
  fileSha256: string;
  payloadSha256: string;
  content: string;
  species: UserSpeciesRecord[];
}

export type UserSpeciesCollisionPolicy = 'skip' | 'overwrite';

export interface UserSpeciesImportRequest {
  sourcePath?: string;
  format?: 'json' | 'csv' | '';
  content?: string;
  collisionPolicy: UserSpeciesCollisionPolicy;
}

export interface UserSpeciesImportItemReview {
  code: string;
  scientificName: string;
  lifeForm?: number | null;
  status: 'new' | 'collision_user' | 'collision_master';
  action: 'insert' | 'overwrite' | 'skip';
  reason?: string;
}

export interface UserSpeciesImportPreview {
  totalRows: number;
  newCount: number;
  userCollisions: number;
  masterCollisions: number;
  payloadSha256: string;
  items: UserSpeciesImportItemReview[];
}

export interface UserSpeciesImportResult {
  preview: UserSpeciesImportPreview;
  insertedCount: number;
  overwrittenCount: number;
  skippedCount: number;
}

export interface LongTidyCSVRequest {
  destinationPath?: string;
  lumpTable?: string;
  lumpDbPath?: string;
}

export interface WideMatrixCSVRequest {
  destinationPath?: string;
  layerMode: 'rollup_all' | 'strata_4' | 'species_layer';
  zeroFill: boolean;
  includeSiteUnit: boolean;
  lumpTable?: string;
  lumpDbPath?: string;
}

export interface CSVExportOutcome {
  mode: string;
  rowCount: number;
  plotCount: number;
  speciesCount: number;
  columnCount: number;
  path?: string;
  sha256: string;
  previewCsv: string;
}

export interface RDSExportRequest {
  destinationPath: string;
  lumpTable?: string;
  lumpDbPath?: string;
}

export interface RDSTableSummary {
  name: string;
  rowCount: number;
  columnCount: number;
}

export interface RDSExportOutcome {
  path: string;
  sha256: string;
  byteSize: number;
  plotCount: number;
  tables: RDSTableSummary[];
}

export interface GeoPackageExportRequest {
  destinationPath: string;
  crs: 'EPSG:3005' | 'EPSG:4326';
  includeAttributeTables: boolean;
  includeUnlocatedPlots: boolean;
  lumpTable?: string;
  lumpDbPath?: string;
}

export interface GeoPackageExportOutcome {
  path: string;
  sha256: string;
  crs: string;
  srsId: number;
  featureCount: number;
  locatedPlotCount: number;
  skippedUnlocatedCount: number;
  normalizedWestLongitudes: number;
  attributeTables: string[];
}

export interface InterchangeHubPort {
  inspectVenusSource(contextID: string, sourcePath: string): Promise<VenusSourceInspection>;
  importVenusProject(contextID: string, request: VenusImportRequest): Promise<VenusImportResult>;
  exportTurboVeg(contextID: string, request: TurboVegExportRequest): Promise<TurboVegExportOutcome>;
  importTurboVeg(contextID: string, request: TurboVegImportRequest): Promise<TurboVegImportOutcome>;
  exportUserSpeciesList(contextID: string, request: UserSpeciesExportRequest): Promise<UserSpeciesExportResult>;
  previewUserSpeciesImport(contextID: string, request: UserSpeciesImportRequest): Promise<UserSpeciesImportPreview>;
  importUserSpeciesList(contextID: string, request: UserSpeciesImportRequest): Promise<UserSpeciesImportResult>;
  exportLongTidyCSV(contextID: string, request: LongTidyCSVRequest): Promise<CSVExportOutcome>;
  exportWideMatrixCSV(contextID: string, request: WideMatrixCSVRequest): Promise<CSVExportOutcome>;
  exportProjectRDS(contextID: string, request: RDSExportRequest): Promise<RDSExportOutcome>;
  exportGeoPackage(contextID: string, request: GeoPackageExportRequest): Promise<GeoPackageExportOutcome>;
}

export const defaultInterchangeHubPort: InterchangeHubPort = {
  inspectVenusSource: (contextID, sourcePath) =>
    Call.ByName('main.ImportExportService.InspectVenusSource', contextID, sourcePath),
  importVenusProject: (contextID, request) =>
    Call.ByName('main.ImportExportService.ImportVenusProject', contextID, request),
  exportTurboVeg: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportTurboVeg', contextID, request),
  importTurboVeg: (contextID, request) =>
    Call.ByName('main.ImportExportService.ImportTurboVeg', contextID, request),
  exportUserSpeciesList: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportUserSpeciesList', contextID, request),
  previewUserSpeciesImport: (contextID, request) =>
    Call.ByName('main.ImportExportService.PreviewUserSpeciesImport', contextID, request),
  importUserSpeciesList: (contextID, request) =>
    Call.ByName('main.ImportExportService.ImportUserSpeciesList', contextID, request),
  exportLongTidyCSV: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportLongTidyCSV', contextID, request),
  exportWideMatrixCSV: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportWideMatrixCSV', contextID, request),
  exportProjectRDS: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportProjectRDS', contextID, request),
  exportGeoPackage: (contextID, request) =>
    Call.ByName('main.ImportExportService.ExportGeoPackage', contextID, request),
};

const PROJECT_IDENT_RE = /^[A-Za-z][A-Za-z0-9_]{0,30}$/;

export function validateTargetProjectName(raw: string): { valid: boolean; normalized: string; error: string } {
  const trimmed = (raw ?? '').trim();
  if (!trimmed) {
    return { valid: false, normalized: '', error: 'Target project name is required.' };
  }
  if (!PROJECT_IDENT_RE.test(trimmed)) {
    return {
      valid: false,
      normalized: trimmed,
      error: 'Target project name must start with a letter and use only letters, digits, or underscores (max 31 chars).',
    };
  }
  const lower = trimmed.toLowerCase();
  if (lower === 'none' || lower === 'sample' || lower.includes('master')) {
    return { valid: false, normalized: trimmed, error: 'Target project name cannot be None, Sample, or contain master.' };
  }
  return { valid: true, normalized: trimmed, error: '' };
}

export function defaultYieldScheduler(): Promise<void> {
  if (typeof globalThis.setTimeout === 'function') {
    return new Promise((resolve) => globalThis.setTimeout(resolve, 0));
  }
  return Promise.resolve();
}

// Cooperative chunked client-side XML sanity check that yields every chunkSize characters
// so Cancel / remount / close guards can preempt large client-side XML inspection.
export async function validateXMLSnippetChunked(
  xmlText: string,
  chunkSize = 16384,
  yieldFn: () => Promise<void> = defaultYieldScheduler,
  isCancelled: () => boolean = () => false
): Promise<{ valid: boolean; plotTagCount: number; error: string }> {
  if (!xmlText || !xmlText.trim()) {
    return { valid: false, plotTagCount: 0, error: 'XML content is empty.' };
  }
  const step = Math.max(1, chunkSize);
  let plotTagCount = 0;
  let offset = 0;
  while (offset < xmlText.length) {
    if (isCancelled()) {
      throw new Error('XML validation cancelled');
    }
    const end = Math.min(xmlText.length, offset + step);
    let i = offset;
    for (; i < end; i++) {
      const code = xmlText.charCodeAt(i);
      if (code === 0) {
        return { valid: false, plotTagCount: 0, error: 'XML contains NUL byte.' };
      }
      if (code >= 0xd800 && code <= 0xdbff) {
        if (i + 1 < xmlText.length) {
          const low = xmlText.charCodeAt(i + 1);
          if (low < 0xdc00 || low > 0xdfff) {
            return { valid: false, plotTagCount: 0, error: 'XML contains unpaired UTF-16 high surrogate.' };
          }
          i++;
        } else {
          return { valid: false, plotTagCount: 0, error: 'XML ends with unpaired UTF-16 high surrogate.' };
        }
      } else if (code >= 0xdc00 && code <= 0xdfff) {
        return { valid: false, plotTagCount: 0, error: 'XML contains unpaired UTF-16 low surrogate.' };
      }
      if (code === 0x3c) {
        const probe = xmlText.slice(i, Math.min(xmlText.length, i + 8));
        if (/^<Plot(?=[\s/>]|$)/i.test(probe)) {
          plotTagCount++;
        }
      }
    }
    offset = i;
    await yieldFn();
    if (isCancelled()) {
      throw new Error('XML validation cancelled');
    }
  }
  return { valid: true, plotTagCount, error: '' };
}

export const TURBOVEG_LAYER_CROSSWALK: ReadonlyArray<{
  vproLayers: string;
  tvGroup: string;
  description: string;
}> = [
  { vproLayers: 'Cover1..Cover3, TotalA', tvGroup: '1..3 = t1..t3 (Tree)', description: 'Dominant (A1), main canopy (A2), sub-canopy (A3); TotalA via VPRO_LAYER UDF' },
  { vproLayers: 'Cover4..Cover5c, TotalB', tvGroup: '4..5 = s1..s2 (Shrub)', description: 'Tall (B1) & low (B2) shrubs; CHARS 5a..5c & TotalB via VPRO_LAYER UDF' },
  { vproLayers: 'Cover6', tvGroup: '6 = hl (Herb)', description: 'Herbaceous layer (C); untagged external 7=jl non-woody & 8=sl seedlings' },
  { vproLayers: 'Cover5 (woody) / Cover6', tvGroup: '7..8 = jl, sl (Juvenile)', description: 'External Turboveg juvenile woody -> Cover5; non-woody & seedlings -> Cover6' },
  { vproLayers: 'Cover7..Cover9', tvGroup: '9 = ml (Cryptogam)', description: 'Terrestrial (D), epixylic (Dw), and epilithic (Dr) bryoids/lichens via VPRO_LAYER UDF' },
  { vproLayers: 'Cover10', tvGroup: '10 = ep (Epiphyte)', description: 'Arboreal lichens and epiphytes (Ep)' },
];
