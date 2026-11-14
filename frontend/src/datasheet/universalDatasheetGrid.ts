import { Call } from '@wailsio/runtime';
import type { CancellableRead } from '../shared/readRequests';

export type DatasheetTableKey =
  | 'Env'
  | 'Admin'
  | 'Veg'
  | 'Humus'
  | 'Mineral'
  | 'Other'
  | 'SU'
  | 'Lump'
  | 'Hierarchy';

export const CANONICAL_DATASHEET_KEYS: readonly DatasheetTableKey[] = [
  'Env',
  'Admin',
  'Veg',
  'Humus',
  'Mineral',
  'Other',
  'SU',
  'Lump',
  'Hierarchy',
];

export interface DatasheetStorageCell {
  storage: 'null' | 'text' | 'integer' | 'real' | 'blob';
  text?: string | null;
  integer?: string | null;
  real?: number | null;
  blobHex?: string | null;
}

export interface DatasheetColumnMeta {
  name: string;
  declaredType: string;
  kind: 'text' | 'integer' | 'single' | 'double' | 'boolean';
  pinned: boolean;
  editable: boolean;
  protected: boolean;
  maxUtf16: number;
  isCoverSingle: boolean;
  isBoolean: boolean;
}

export interface DatasheetTableInfo {
  key: DatasheetTableKey;
  label: string;
  role: string;
  tableName: string;
  pinnedColumn: string;
  available: boolean;
  unavailableReason?: string;
  editable: boolean;
  rowCount: number;
  columns: DatasheetColumnMeta[];
}

export interface DatasheetSortSpec {
  column: string;
  direction: 'asc' | 'desc';
}

export interface DatasheetFilterSpec {
  column: string;
  op: 'contains' | 'eq' | 'starts_with' | 'ends_with' | 'is_null' | 'not_null';
  value: string;
}

export interface DatasheetPageRequest {
  tableKey: string;
  offset: number;
  limit: number;
  sort: DatasheetSortSpec[];
  filters: DatasheetFilterSpec[];
  globalSearch: string;
  scopeToActiveSU: boolean;
}

export interface DatasheetRow {
  rowId: string;
  cells: DatasheetStorageCell[];
}

export interface DatasheetPage {
  contextId: string;
  project: string;
  projectPath: string;
  su: string;
  hierarchy: string;
  tableKey: DatasheetTableKey;
  tableName: string;
  pinnedColumn: string;
  columns: DatasheetColumnMeta[];
  rows: DatasheetRow[];
  offset: number;
  limit: number;
  totalRows: number;
  canEditMaster: boolean;
  scopeToActiveSU: boolean;
}

export interface DatasheetCellEdit {
  rowId: string;
  column: string;
  before: DatasheetStorageCell;
  after: DatasheetStorageCell;
}

export interface DatasheetEditRequest {
  tableKey: string;
  edits: DatasheetCellEdit[];
}

export interface DatasheetEditResult {
  contextId: string;
  tableKey: string;
  tableName: string;
  updatedRows: number;
  updatedCells: number;
  auditRows: number;
}

export interface DatasheetFindReplaceRequest {
  tableKey: string;
  column: string;
  findText: string;
  replaceText: string;
  matchCase: boolean;
  exactCell: boolean;
  filters: DatasheetFilterSpec[];
  globalSearch: string;
  scopeToActiveSU: boolean;
  maxPreview: number;
}

export interface DatasheetFindReplaceItem {
  rowId: string;
  pinnedIdentity: string;
  column: string;
  beforeText: string;
  afterText: string;
  edit: DatasheetCellEdit;
  validationError?: string;
  beforeCell: DatasheetStorageCell;
  afterCell: DatasheetStorageCell;
}

export interface DatasheetFindReplacePreview {
  contextId: string;
  tableKey: string;
  tableName: string;
  column: string;
  matchedRows: number;
  validCount: number;
  invalidCount: number;
  items: DatasheetFindReplaceItem[];
  truncated: boolean;
}

export interface DatasheetBackendPort {
  listTables(contextId: string): CancellableRead<DatasheetTableInfo[]>;
  getPage(contextId: string, request: DatasheetPageRequest): CancellableRead<DatasheetPage>;
  previewFindReplace(
    contextId: string,
    request: DatasheetFindReplaceRequest
  ): CancellableRead<DatasheetFindReplacePreview>;
  applyEdits(contextId: string, request: DatasheetEditRequest): Promise<DatasheetEditResult>;
}

export const defaultDatasheetBackendPort: DatasheetBackendPort = {
  listTables: (contextId: string) =>
    Call.ByName('main.DatasheetService.ListDatasheetTables', contextId),
  getPage: (contextId: string, request: DatasheetPageRequest) =>
    Call.ByName('main.DatasheetService.GetDatasheetPage', contextId, request),
  previewFindReplace: (contextId: string, request: DatasheetFindReplaceRequest) =>
    Call.ByName('main.DatasheetService.PreviewDatasheetFindReplace', contextId, request),
  applyEdits: (contextId: string, request: DatasheetEditRequest) =>
    Call.ByName('main.DatasheetService.ApplyDatasheetEdits', contextId, request),
};

export interface VirtualWindowSlice {
  startIndex: number;
  endIndex: number;
  topSpacerPx: number;
  bottomSpacerPx: number;
}

export function computeVirtualWindow(
  scrollTop: number,
  viewportHeight: number,
  rowHeight: number,
  totalRows: number,
  overscan = 6
): VirtualWindowSlice {
  const safeRowHeight = Math.max(20, rowHeight);
  const safeTotal = Math.max(0, totalRows);
  if (safeTotal === 0) {
    return { startIndex: 0, endIndex: 0, topSpacerPx: 0, bottomSpacerPx: 0 };
  }
  const safeScroll = Math.max(0, scrollTop);
  const visibleCount = Math.max(1, Math.ceil(Math.max(safeRowHeight, viewportHeight) / safeRowHeight));
  const rawStart = Math.floor(safeScroll / safeRowHeight);
  const startIndex = Math.max(0, Math.min(safeTotal - 1, rawStart - overscan));
  const endIndex = Math.min(safeTotal, rawStart + visibleCount + overscan);
  const topSpacerPx = startIndex * safeRowHeight;
  const bottomSpacerPx = Math.max(0, (safeTotal - endIndex) * safeRowHeight);
  return { startIndex, endIndex, topSpacerPx, bottomSpacerPx };
}

export function formatCellForEditor(cell: DatasheetStorageCell | undefined, col: DatasheetColumnMeta): string {
  if (!cell || cell.storage === 'null') return '';
  if (cell.storage === 'text') return cell.text ?? '';
  if (cell.storage === 'integer') {
    if (col.isBoolean) {
      return cell.integer === '0' ? 'false' : 'true';
    }
    return cell.integer ?? '';
  }
  if (cell.storage === 'real') {
    if (cell.real === null || cell.real === undefined) return '';
    return Number(cell.real.toPrecision(15)).toString();
  }
  if (cell.storage === 'blob') return cell.blobHex ?? '';
  return '';
}

export interface ParsedCellResult {
  cell: DatasheetStorageCell | null;
  error: string;
}

const MAX_FLOAT32 = 3.4028234663852886e38;

function hasUnpairedSurrogate(text: string): boolean {
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = text.charCodeAt(i + 1);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return true;
      i++;
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      return true;
    }
  }
  return false;
}

export function parseAndValidateCellInput(
  tableKey: string,
  col: DatasheetColumnMeta,
  rawInput: string,
  canEditMaster: boolean
): ParsedCellResult {
  if (tableKey === 'Admin' && col.name === 'BECSiteUnit' && !canEditMaster) {
    return {
      cell: null,
      error: 'BEC Master editing is restricted to the custodian; edit UserSiteUnit instead.',
    };
  }
  if (!col.editable) {
    return { cell: null, error: `${col.name} is read-only in datasheet mode.` };
  }
  if (rawInput.includes('\u0000') || hasUnpairedSurrogate(rawInput)) {
    return { cell: null, error: `${col.name} contains invalid Unicode or NUL characters.` };
  }

  if (rawInput === '') {
    const nonNullRequired =
      (tableKey === 'Env' && col.name === 'PlotNumber') ||
      (tableKey === 'Admin' && col.name === 'Plot') ||
      (tableKey === 'Veg' && (col.name === 'PlotNumber' || col.name === 'Species')) ||
      (tableKey === 'Humus' && col.name === 'PlotNumber') ||
      (tableKey === 'Mineral' && col.name === 'PlotNumber') ||
      (tableKey === 'Other' && col.name === 'PlotNumber') ||
      (tableKey === 'Hierarchy' && col.name === 'Name');
    if (nonNullRequired) {
      return { cell: null, error: `${tableKey}.${col.name} cannot be blank.` };
    }
    return { cell: { storage: 'null' }, error: '' };
  }

  if (col.isBoolean) {
    const lower = rawInput.trim().toLowerCase();
    if (['true', '-1', '1', 'yes', 'y'].includes(lower)) {
      return { cell: { storage: 'integer', integer: '-1' }, error: '' };
    }
    if (['false', '0', 'no', 'n'].includes(lower)) {
      return { cell: { storage: 'integer', integer: '0' }, error: '' };
    }
    return { cell: null, error: `${col.name} requires boolean true/false (-1 or 0).` };
  }

  if (col.kind === 'text') {
    // JavaScript string .length measures exact UTF-16 code units!
    if (col.maxUtf16 > 0 && rawInput.length > col.maxUtf16) {
      return {
        cell: null,
        error: `${col.name} exceeds ${col.maxUtf16} UTF-16 code units (got ${rawInput.length}).`,
      };
    }
    if (
      rawInput.trim() === '' &&
      (col.name === 'PlotNumber' ||
        col.name === 'Plot' ||
        (tableKey === 'Veg' && col.name === 'Species') ||
        (tableKey === 'Hierarchy' && col.name === 'Name'))
    ) {
      return { cell: null, error: `${col.name} cannot be whitespace-only.` };
    }
    return { cell: { storage: 'text', text: rawInput }, error: '' };
  }

  if (col.kind === 'integer') {
    const trimmed = rawInput.trim();
    if (!/^-?\d+$/.test(trimmed)) {
      return { cell: null, error: `${col.name} requires a whole integer.` };
    }
    const num = Number(trimmed);
    if (!Number.isSafeInteger(num)) {
      return { cell: null, error: `${col.name} integer is out of bounds.` };
    }
    const isInt32 =
      col.declaredType.toUpperCase() === 'INTEGER' ||
      ['ID', 'LL', 'PV', 'Parent'].includes(col.name);
    if (isInt32) {
      if (num < -2147483648 || num > 2147483647) {
        return { cell: null, error: `${col.name} exceeds Access Long (-2147483648..2147483647).` };
      }
    } else if (num < -32768 || num > 32767) {
      return { cell: null, error: `${col.name} exceeds Access Integer (-32768..32767).` };
    }
    if (tableKey === 'Hierarchy' && col.name === 'Level' && (num < 1 || num > 11)) {
      return { cell: null, error: 'Hierarchy.Level must be between 1 and 11.' };
    }
    return { cell: { storage: 'integer', integer: String(num) }, error: '' };
  }

  if (col.kind === 'single') {
    const trimmed = rawInput.trim();
    const num = Number(trimmed);
    if (trimmed === '' || !Number.isFinite(num) || Math.abs(num) > MAX_FLOAT32) {
      return { cell: null, error: `${col.name} requires a finite IEEE-754 Single number.` };
    }
    // Narrow float64 -> float32 via Math.fround BEFORE checking `< 100`!
    const narrowed = Math.fround(num);
    if (!Number.isFinite(narrowed)) {
      return { cell: null, error: `${col.name} overflows IEEE-754 Single.` };
    }
    if (col.isCoverSingle) {
      if (narrowed >= 100) {
        return {
          cell: null,
          error: `${col.name} must narrow to IEEE-754 float32 < 100 (or blank).`,
        };
      }
      if (narrowed < 0) {
        return { cell: null, error: `${col.name} cover cannot be negative.` };
      }
    }
    const normalized = Object.is(narrowed, -0) ? 0 : narrowed;
    return { cell: { storage: 'real', real: normalized }, error: '' };
  }

  if (col.kind === 'double') {
    const trimmed = rawInput.trim();
    const num = Number(trimmed);
    if (trimmed === '' || !Number.isFinite(num)) {
      return { cell: null, error: `${col.name} requires a finite number.` };
    }
    const normalized = Object.is(num, -0) ? 0 : num;
    return { cell: { storage: 'real', real: normalized }, error: '' };
  }

  return { cell: { storage: 'text', text: rawInput }, error: '' };
}

export function cellsSemanticallyEqual(
  a: DatasheetStorageCell,
  b: DatasheetStorageCell,
  col: DatasheetColumnMeta
): boolean {
  if (a.storage === 'null' && b.storage === 'null') return true;
  if (a.storage !== b.storage) return false;
  if (a.storage === 'text') return (a.text ?? '') === (b.text ?? '');
  if (a.storage === 'integer') {
    if (col.isBoolean) {
      return (a.integer !== '0') === (b.integer !== '0');
    }
    return (a.integer ?? '') === (b.integer ?? '');
  }
  if (a.storage === 'real') {
    if (a.real === null || a.real === undefined || b.real === null || b.real === undefined) {
      return a.real === b.real;
    }
    if (col.kind === 'single') {
      return Math.fround(a.real) === Math.fround(b.real);
    }
    return a.real === b.real;
  }
  if (a.storage === 'blob') return (a.blobHex ?? '') === (b.blobHex ?? '');
  return false;
}

export interface GridSelectionRange {
  startRow: number;
  endRow: number;
  startCol: number;
  endCol: number;
}

export function normalizeGridRange(range: GridSelectionRange): GridSelectionRange {
  return {
    startRow: Math.min(range.startRow, range.endRow),
    endRow: Math.max(range.startRow, range.endRow),
    startCol: Math.min(range.startCol, range.endCol),
    endCol: Math.max(range.startCol, range.endCol),
  };
}

function escapeTSVCell(val: string): string {
  if (val.includes('\t') || val.includes('\n') || val.includes('\r') || val.includes('"')) {
    return `"${val.replaceAll('"', '""')}"`;
  }
  return val;
}

export function formatTSVRange(
  rows: readonly DatasheetRow[],
  columns: readonly DatasheetColumnMeta[],
  range: GridSelectionRange,
  draftLookup?: (rowId: string, colName: string) => string | undefined
): string {
  const norm = normalizeGridRange(range);
  const lines: string[] = [];
  for (let r = norm.startRow; r <= norm.endRow && r < rows.length; r++) {
    const row = rows[r];
    const rowValues: string[] = [];
    for (let c = norm.startCol; c <= norm.endCol && c < columns.length; c++) {
      const col = columns[c];
      const override = draftLookup?.(row.rowId, col.name);
      const val = override !== undefined ? override : formatCellForEditor(row.cells[c], col);
      rowValues.push(escapeTSVCell(val));
    }
    lines.push(rowValues.join('\t'));
  }
  return lines.join('\n');
}

export function parseTSVClipboard(raw: string): string[][] {
  if (!raw) return [];
  const normalized = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
  const matrix: string[][] = [];
  let currentRow: string[] = [];
  let field = '';
  let inQuotes = false;

  for (let i = 0; i < normalized.length; i++) {
    const ch = normalized[i];
    if (inQuotes) {
      if (ch === '"') {
        if (normalized[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          inQuotes = false;
        }
      } else {
        field += ch;
      }
    } else if (ch === '"' && field.length === 0) {
      inQuotes = true;
    } else if (ch === '\t') {
      currentRow.push(field);
      field = '';
    } else if (ch === '\n') {
      currentRow.push(field);
      field = '';
      // Ignore single trailing newline emitted by Excel/Calc on clipboard copy
      if (i < normalized.length - 1 || currentRow.length > 1 || currentRow[0] !== '') {
        matrix.push(currentRow);
      }
      currentRow = [];
    } else {
      field += ch;
    }
  }
  if (field.length > 0 || currentRow.length > 0) {
    currentRow.push(field);
    matrix.push(currentRow);
  }
  return matrix;
}

export function toggleMultiColumnSort(
  existing: readonly DatasheetSortSpec[],
  column: string,
  appendMulti: boolean
): DatasheetSortSpec[] {
  const idx = existing.findIndex(s => s.column.toLowerCase() === column.toLowerCase());
  if (!appendMulti) {
    if (idx === 0 && existing.length === 1) {
      if (existing[0].direction === 'asc') {
        return [{ column, direction: 'desc' }];
      }
      return [];
    }
    return [{ column, direction: 'asc' }];
  }
  const copy = existing.map(s => ({ ...s }));
  if (idx < 0) {
    copy.push({ column, direction: 'asc' });
  } else if (copy[idx].direction === 'asc') {
    copy[idx] = { column, direction: 'desc' };
  } else {
    copy.splice(idx, 1);
  }
  return copy;
}

export interface GridNavigationCoords {
  row: number;
  col: number;
}

export function computeNextGridCell(
  current: GridNavigationCoords,
  key: 'Enter' | 'Tab',
  shiftKey: boolean,
  rowCount: number,
  colCount: number
): GridNavigationCoords | null {
  if (rowCount <= 0 || colCount <= 0) return null;
  if (key === 'Enter') {
    // Vertical same-column progression (VPro muscle memory!)
    const nextRow = shiftKey ? current.row - 1 : current.row + 1;
    if (nextRow < 0 || nextRow >= rowCount) return null;
    return { row: nextRow, col: current.col };
  }
  // Horizontal Tab progression wrapping across rows
  if (!shiftKey) {
    if (current.col + 1 < colCount) return { row: current.row, col: current.col + 1 };
    if (current.row + 1 < rowCount) return { row: current.row + 1, col: 0 };
    return null;
  }
  if (current.col - 1 >= 0) return { row: current.row, col: current.col - 1 };
  if (current.row - 1 >= 0) return { row: current.row - 1, col: colCount - 1 };
  return null;
}
