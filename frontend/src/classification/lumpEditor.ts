import { Call } from '@wailsio/runtime';

export interface LumpTableSummary {
  name: string;
  tableName: string;
  databasePath: string;
  isActiveProject: boolean;
  metadataVersion: string;
  rowCount: number;
  activeCount: number;
  distinctLumpCodes: number;
}

export interface LumpRow {
  lumpCode: string;
  sppCode: string;
  use: boolean;
  scientificName: string;
  englishName: string;
  lifeform?: number | null;
  sourceCatalog: string;
  lumpKnown: boolean;
  ruleOrigin?: string;
}

export interface LumpRowInput {
  lumpCode: string;
  sppCode: string;
  use: boolean;
  allowUnlisted?: boolean;
}

export interface LumpGroupImpact {
  lumpCode: string;
  sourceSpecies: string[];
  plotCount: number;
  rawObservations: number;
  foldedObservations: number;
  maxFoldedCover: number;
  cappedCells: number;
}

export interface LumpImpactPreview {
  tableName: string;
  databasePath: string;
  totalPlots: number;
  distinctRawSpecies: number;
  distinctFoldedSpecies: number;
  speciesFoldedCount: number;
  rawObservations: number;
  foldedObservations: number;
  mergedCells: number;
  cappedCells: number;
  coverCap: number;
  groups: LumpGroupImpact[];
}

export interface LumpSpec {
  preset?: string;
  collapseInfraspecific: boolean;
  collapseSeedlingAndDeadSuffixes: boolean;
  genusRollups?: string[];
  nonVascularLifeforms?: number[];
  baseLumpTable?: string;
  baseLumpDbPath?: string;
  customRules?: LumpRowInput[];
  scopeToProjectSpecies: boolean;
}

export interface LumpPresetInfo {
  id: string;
  title: string;
  description: string;
  spec: LumpSpec;
}

export interface LumpSpecPreview {
  effectiveSpec: LumpSpec;
  rows: LumpRow[];
  impact: LumpImpactPreview;
}

export interface LumpMaterializeResult {
  table: LumpTableSummary;
  rowsWritten: number;
  impact: LumpImpactPreview;
}

export interface LumpEditorPort {
  discoverLumpTables(contextID: string, externalPaths: string[]): Promise<LumpTableSummary[]>;
  createLumpTable(contextID: string, name: string): Promise<LumpTableSummary>;
  saveAsLumpTable(contextID: string, sourceTable: string, sourceDBPath: string, targetName: string): Promise<LumpTableSummary>;
  deleteLumpTable(contextID: string, name: string): Promise<void>;
  getLumpTableRows(contextID: string, tableName: string, dbPath: string): Promise<LumpRow[]>;
  upsertLumpRow(contextID: string, tableName: string, input: LumpRowInput): Promise<LumpRow>;
  updateLumpRow(contextID: string, tableName: string, existingSppCode: string, input: LumpRowInput): Promise<LumpRow>;
  deleteLumpRow(contextID: string, tableName: string, sppCode: string): Promise<void>;
  toggleLumpRow(contextID: string, tableName: string, sppCode: string, use: boolean): Promise<LumpRow>;
  setAllLumpRowsUse(contextID: string, tableName: string, use: boolean): Promise<number>;
  previewLumpImpact(contextID: string, tableName: string, dbPath: string): Promise<LumpImpactPreview>;
  listLumpPresets(): Promise<LumpPresetInfo[]>;
  previewLumpSpec(contextID: string, spec: LumpSpec): Promise<LumpSpecPreview>;
  materializeLumpSpec(contextID: string, targetName: string, spec: LumpSpec, replaceExisting: boolean): Promise<LumpMaterializeResult>;
}

export const defaultLumpEditorPort: LumpEditorPort = {
  discoverLumpTables: (contextID, externalPaths) =>
    Call.ByName('main.ImportExportService.DiscoverLumpTables', contextID, externalPaths),
  createLumpTable: (contextID, name) =>
    Call.ByName('main.ImportExportService.CreateLumpTable', contextID, name),
  saveAsLumpTable: (contextID, sourceTable, sourceDBPath, targetName) =>
    Call.ByName('main.ImportExportService.SaveAsLumpTable', contextID, sourceTable, sourceDBPath, targetName),
  deleteLumpTable: (contextID, name) =>
    Call.ByName('main.ImportExportService.DeleteLumpTable', contextID, name),
  getLumpTableRows: (contextID, tableName, dbPath) =>
    Call.ByName('main.ImportExportService.GetLumpTableRows', contextID, tableName, dbPath),
  upsertLumpRow: (contextID, tableName, input) =>
    Call.ByName('main.ImportExportService.UpsertLumpRow', contextID, tableName, input),
  updateLumpRow: (contextID, tableName, existingSppCode, input) =>
    Call.ByName('main.ImportExportService.UpdateLumpRow', contextID, tableName, existingSppCode, input),
  deleteLumpRow: (contextID, tableName, sppCode) =>
    Call.ByName('main.ImportExportService.DeleteLumpRow', contextID, tableName, sppCode),
  toggleLumpRow: (contextID, tableName, sppCode, use) =>
    Call.ByName('main.ImportExportService.ToggleLumpRow', contextID, tableName, sppCode, use),
  setAllLumpRowsUse: (contextID, tableName, use) =>
    Call.ByName('main.ImportExportService.SetAllLumpRowsUse', contextID, tableName, use),
  previewLumpImpact: (contextID, tableName, dbPath) =>
    Call.ByName('main.ImportExportService.PreviewLumpImpact', contextID, tableName, dbPath),
  listLumpPresets: () =>
    Call.ByName('main.ImportExportService.ListLumpPresets'),
  previewLumpSpec: (contextID, spec) =>
    Call.ByName('main.ImportExportService.PreviewLumpSpec', contextID, spec),
  materializeLumpSpec: (contextID, targetName, spec, replaceExisting) =>
    Call.ByName('main.ImportExportService.MaterializeLumpSpec', contextID, targetName, spec, replaceExisting),
};

const LUMP_NAME_PATTERN = /^[A-Za-z][A-Za-z0-9_]{0,30}$/;
const RESERVED_LUMP_NAMES = new Set(['none', 'combinedspp', 'attribute']);

export function hasWellFormedUnicode(value: string): boolean {
  for (let i = 0; i < value.length; i++) {
    const code = value.charCodeAt(i);
    if (code === 0) return false;
    if (code >= 0xd800 && code <= 0xdbff) {
      if (i + 1 >= value.length) return false;
      const low = value.charCodeAt(i + 1);
      if (low < 0xdc00 || low > 0xdfff) return false;
      i++;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return false;
    }
  }
  return true;
}

export function validateLumpBaseName(raw: string): { valid: boolean; normalized: string; error: string } {
  if (typeof raw !== 'string' || !hasWellFormedUnicode(raw)) {
    return { valid: false, normalized: '', error: 'Enter a valid lump table name.' };
  }
  let trimmed = raw.trim();
  if (trimmed.toLowerCase().endsWith('_lump')) {
    trimmed = trimmed.slice(0, -5);
  }
  if (!trimmed) {
    return { valid: false, normalized: '', error: 'Lump table name is required.' };
  }
  if (!LUMP_NAME_PATTERN.test(trimmed)) {
    return {
      valid: false,
      normalized: trimmed,
      error: 'Lump table name must start with a letter and contain only letters, digits, or underscores (max 31 chars).',
    };
  }
  const lower = trimmed.toLowerCase();
  if (RESERVED_LUMP_NAMES.has(lower) || lower.startsWith('usys')) {
    return { valid: false, normalized: trimmed, error: `Lump table name "${trimmed}" is reserved by VPro.` };
  }
  return { valid: true, normalized: trimmed, error: '' };
}

export function validateLumpRowDraft(lumpCode: string, sppCode: string): {
  valid: boolean;
  lumpCode: string;
  sppCode: string;
  error: string;
} {
  if (typeof lumpCode !== 'string' || typeof sppCode !== 'string' || !hasWellFormedUnicode(lumpCode) || !hasWellFormedUnicode(sppCode)) {
    return { valid: false, lumpCode: '', sppCode: '', error: 'Species and lump codes must contain valid Unicode.' };
  }
  const cleanLump = lumpCode.trim().toUpperCase();
  const cleanSpp = sppCode.trim().toUpperCase();
  if (!cleanLump) {
    return { valid: false, lumpCode: cleanLump, sppCode: cleanSpp, error: 'Target LumpCode is required.' };
  }
  if (!cleanSpp) {
    return { valid: false, lumpCode: cleanLump, sppCode: cleanSpp, error: 'Source SppCode is required.' };
  }
  // JavaScript string .length measures UTF-16 code units directly
  if (cleanLump.length > 50) {
    return { valid: false, lumpCode: cleanLump, sppCode: cleanSpp, error: 'LumpCode must be at most 50 UTF-16 code units.' };
  }
  if (cleanSpp.length > 8) {
    return { valid: false, lumpCode: cleanLump, sppCode: cleanSpp, error: 'SppCode must be at most 8 UTF-16 code units.' };
  }
  if (cleanLump === cleanSpp) {
    return { valid: false, lumpCode: cleanLump, sppCode: cleanSpp, error: 'LumpCode and SppCode must not be identical.' };
  }
  return { valid: true, lumpCode: cleanLump, sppCode: cleanSpp, error: '' };
}

export function parseGenusRollupTokens(raw: string): string[] {
  if (!raw || typeof raw !== 'string') return [];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const part of raw.split(/[,;\n]+/)) {
    const token = part.trim().toUpperCase();
    if (token && !seen.has(token)) {
      seen.add(token);
      out.push(token);
    }
  }
  return out;
}

export function groupLumpRowsByCode(rows: LumpRow[]): Array<{ lumpCode: string; activeCount: number; rows: LumpRow[] }> {
  const map = new Map<string, LumpRow[]>();
  for (const row of rows) {
    const key = row.lumpCode || '(UNASSIGNED)';
    const bucket = map.get(key);
    if (bucket) {
      bucket.push(row);
    } else {
      map.set(key, [row]);
    }
  }
  return Array.from(map.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([lumpCode, items]) => ({
      lumpCode,
      activeCount: items.filter((r) => r.use).length,
      rows: items.slice().sort((a, b) => a.sppCode.localeCompare(b.sppCode)),
    }));
}
