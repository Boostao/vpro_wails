import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';

export interface SIVIRow {
  rowId: string;
  cells: ProjectMetadataCell[];
}
export interface SIVIProjection {
  Form: string;
  Query: string;
  Columns: string[];
  Rows: SIVIRow[];
}
export type SIVIHeightColumn = 'HeightA' | 'HeightB' | 'Height6';
export interface SIVIHeightCell {
  raw: string;
  nullValue: boolean;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
  error: string | null;
}
export type SIVIHeightDrafts = Record<string, Partial<Record<SIVIHeightColumn, SIVIHeightCell>>>;
export interface SIVIHeightEdit {
  rowId: string;
  form: string;
  column: SIVIHeightColumn;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
}

const columns = [
  ['ID', 'PlotNumber', 'Species', 'Cover1', 'Cover2', 'Cover3', 'TotalA', 'HeightA',
    'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB', 'HeightB', 'Collected'],
  ['ID', 'PlotNumber', 'Species', 'Cover6', 'Height6', 'Collected'],
  ['ID', 'PlotNumber', 'Species', 'Cover7', 'Cover8', 'Cover9', 'Collected'],
];
const predicates = [
  ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB'],
  ['Cover6'], ['Cover7', 'Cover8', 'Cover9'],
];
const heightColumns: SIVIHeightColumn[] = ['HeightA', 'HeightB', 'Height6'];

export function validateSIVIProjection(review: SIVIProjection[], plot: string, extended: boolean): SIVIProjection[] {
  if (!wellFormedUTF16(plot) || plot.includes('\0') || !Array.isArray(review) || review.length !== 3) {
    throw new Error('SIVI requires the literal plot and all three complete source groups.');
  }
  const forms = [extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC', 'SubVegC-SIVI', 'SubVegD-SIVI'];
  const queries = ['USysVegA', 'USysVegC', 'USysVegD'];
  const shared = new Map<string, Map<string, ProjectMetadataCell>>();
  review.forEach((group, index) => {
    if (!group || group.Form !== forms[index] || group.Query !== queries[index] ||
        !Array.isArray(group.Columns) || group.Columns.length !== columns[index].length ||
        group.Columns.some((column, i) => column !== columns[index][i]) ||
        !Array.isArray(group.Rows) || new Set(group.Rows.map(row => row.rowId)).size !== group.Rows.length) {
      throw new Error('SIVI source identity, complete ordered columns or physical rows were not returned.');
    }
    for (const row of group.Rows) {
      if (!exactSigned64(row.rowId) || !Array.isArray(row.cells) || row.cells.length !== group.Columns.length ||
          !row.cells.every(completeCell) || row.cells[1].storage !== 'text' || row.cells[1].text !== plot ||
          !predicates[index].some(column => row.cells[group.Columns.indexOf(column)].storage !== 'null')) {
        throw new Error('SIVI row has incomplete raw cells, foreign identity or invented height-only membership.');
      }
      const physical = shared.get(row.rowId) ?? new Map<string, ProjectMetadataCell>();
      group.Columns.forEach((column, i) => {
        const previous = physical.get(column);
        if (previous && !equalCell(previous, row.cells[i])) throw new Error('SIVI groups disagree about an original physical cell.');
        physical.set(column, row.cells[i]);
      });
      shared.set(row.rowId, physical);
    }
  });
  return structuredClone(review);
}

export function siviHeightPresentation(review: SIVIProjection[], plot: string, extended: boolean): SIVIProjection[] {
  const current = review[0]?.Form === 'SubVegA-SIVI';
  const result = validateSIVIProjection(review, plot, current);
  result[0].Form = extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC';
  return result;
}

export function stageSIVIHeight(review: SIVIProjection[], drafts: SIVIHeightDrafts, rowId: string,
  column: SIVIHeightColumn, raw: string, nullValue: boolean): SIVIHeightDrafts {
  const group = review[column === 'Height6' ? 1 : 0];
  const index = group?.Columns.indexOf(column) ?? -1;
  const rows = group?.Rows.filter(row => row.rowId === rowId) ?? [];
  if (!heightColumns.includes(column) || !exactSigned64(rowId) || rows.length !== 1 || index < 0 ||
      !completeCell(rows[0].cells[index])) throw new Error('SIVI height source is unavailable; reload before editing.');
  const expected = structuredClone(drafts[rowId]?.[column]?.expected ?? rows[0].cells[index]);
  let value: ProjectMetadataCell = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  let error: string | null = null;
  if (!nullValue) {
    if (!wellFormedUTF16(raw)) error = `${column} contains incomplete Unicode; raw input was not repaired.`;
    else if (expected.storage !== 'null' && raw === metadataCellText(expected)) value = structuredClone(expected);
    else if (column === 'HeightB') {
      value = { ...value, storage: 'text', text: raw };
      if (raw.length > 255) error = 'HeightB exceeds255 UTF-16 units; literal input was not truncated.';
    } else {
      const parsed = finiteSingleValue(column, raw);
      error = parsed.error;
      if (parsed.value !== null) value = { ...value, storage: 'real', real: parsed.value };
    }
  }
  if (expected.storage === 'blob' && !equalCell(expected, value)) {
    error = 'Historical SIVI BLOB correction is unavailable; retain the unchanged original.';
  }
  return { ...drafts, [rowId]: { ...drafts[rowId], [column]: { raw, nullValue, expected, value, error } } };
}

export function siviHeightErrors(drafts: SIVIHeightDrafts): string[] {
  return Object.values(drafts).flatMap(row => Object.values(row).flatMap(cell => cell?.error ? [cell.error] : []));
}
export function siviHeightDirty(drafts: SIVIHeightDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row).some(cell => cell &&
    (cell.error !== null || !equalCell(cell.expected, cell.value))));
}
export function siviHeightEdits(review: SIVIProjection[], drafts: SIVIHeightDrafts): SIVIHeightEdit[] {
  const errors = siviHeightErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  const edits: SIVIHeightEdit[] = [];
  for (const [rowId, row] of Object.entries(drafts)) {
    for (const column of heightColumns) {
      const cell = row[column];
      if (!cell || equalCell(cell.expected, cell.value)) continue;
      const group = review[column === 'Height6' ? 1 : 0];
      const source = group?.Rows.filter(row => row.rowId === rowId) ?? [];
      const index = group?.Columns.indexOf(column) ?? -1;
      if (source.length !== 1 || index < 0 || !equalCell(source[0].cells[index], cell.expected)) {
        throw new Error('SIVI original height changed; drafts retained, reload explicitly.');
      }
      edits.push({ rowId, form: group.Form, column, expected: structuredClone(cell.expected), value: structuredClone(cell.value) });
    }
  }
  return edits;
}
