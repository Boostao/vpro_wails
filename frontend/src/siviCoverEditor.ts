import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';
import { validateSIVIProjection, type SIVIProjection } from './siviHeightEditor';

export type { SIVIProjection, SIVIRow } from './siviHeightEditor';
export const siviCoverColumns = [
  'Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB',
  'Cover6', 'Cover7', 'Cover8', 'Cover9',
] as const;
export type SIVICoverColumn = typeof siviCoverColumns[number];
export const siviCoverLabels: Record<SIVICoverColumn, string> = {
  Cover1: 'A1', Cover2: 'A2', Cover3: 'A3', TotalA: 'A',
  Cover4: 'B1', Cover5: 'B2', Cover5a: 'B3', Cover5b: 'B4', Cover5c: 'B5', TotalB: 'B',
  Cover6: 'C', Cover7: 'D', Cover8: 'Dr/Dw', Cover9: 'Ep',
};
const extendedColumns: SIVICoverColumn[] = ['Cover5a', 'Cover5b', 'Cover5c'];
export interface SIVICoverCell {
  raw: string;
  nullValue: boolean;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
  error: string | null;
}
export type SIVICoverDrafts = Record<string, Partial<Record<SIVICoverColumn, SIVICoverCell>>>;
export interface SIVICoverEdit {
  rowId: string;
  form: string;
  column: SIVICoverColumn;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
}
export interface SIVICoverRequest {
  original: SIVIProjection[];
  edits: SIVICoverEdit[];
}

export function siviCoverGroups(index: number, extended: boolean): SIVICoverColumn[][] {
  return index === 0 ? [
    ['Cover1', 'Cover2', 'Cover3', 'TotalA'],
    extended ? ['Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB'] : ['Cover4', 'Cover5', 'TotalB'],
  ] : index === 1 ? [['Cover6']] : index === 2 ? [['Cover7', 'Cover8', 'Cover9']] : [];
}

function source(review: SIVIProjection[], rowId: string, column: SIVICoverColumn) {
  if (!siviCoverColumns.includes(column) || !exactSigned64(rowId)) {
    throw new Error('Only source-scoped SIVI cover and total controls are editable.');
  }
  const groupIndex = column === 'Cover6' ? 1 : ['Cover7', 'Cover8', 'Cover9'].includes(column) ? 2 : 0;
  const group = review[groupIndex];
  const extended = review[0]?.Form === 'SubVegA-SIVI';
  const index = group?.Columns.indexOf(column) ?? -1;
  const rows = group?.Rows.filter(row => row.rowId === rowId) ?? [];
  if (!group || group.Form !== [extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC', 'SubVegC-SIVI', 'SubVegD-SIVI'][groupIndex] ||
      group.Query !== ['USysVegA', 'USysVegC', 'USysVegD'][groupIndex] ||
      !siviCoverGroups(groupIndex, extended).flat().includes(column) || index < 0 ||
      group.Columns.filter(name => name === column).length !== 1 || rows.length !== 1 ||
      !completeCell(rows[0].cells[index])) {
    throw new Error('SIVI cover source is unavailable or hidden; show the source control or reload before editing.');
  }
  return { group, original: rows[0].cells[index] };
}

export function parseSIVICover(column: SIVICoverColumn, raw: string, nullValue: boolean,
  original: ProjectMetadataCell): SIVICoverCell {
  if (!siviCoverColumns.includes(column) || typeof raw !== 'string' || typeof nullValue !== 'boolean' || !completeCell(original)) {
    throw new Error('SIVI cover parsing requires a source column, literal text, explicit NULL choice and complete original cell.');
  }
  const expected = structuredClone(original);
  let value: ProjectMetadataCell = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  let error: string | null = null;
  if (!nullValue) {
    if (!wellFormedUTF16(raw)) error = `${column} contains incomplete Unicode; raw input was not repaired.`;
    else if (expected.storage !== 'null' && raw === metadataCellText(expected)) value = structuredClone(expected);
    // The shared numeric parser accepts surrounding spaces; this source boundary must not normalize them.
    else if (raw !== raw.trim()) error = `${column} requires literal numeric input without surrounding whitespace.`;
    else {
      const parsed = finiteSingleValue(column, raw);
      error = parsed.error;
      if (parsed.value !== null) {
        value = { ...value, storage: 'real', real: parsed.value };
        if (parsed.value >= 100) error = `${column} must be strictly less than 100 or NULL.`;
      }
    }
  }
  if (expected.storage === 'blob' && !equalCell(expected, value)) {
    error = 'Historical SIVI BLOB correction is unavailable; retain the unchanged original.';
  }
  return { raw, nullValue, expected, value, error };
}

export function stageSIVICover(review: SIVIProjection[], drafts: SIVICoverDrafts, rowId: string,
  column: SIVICoverColumn, raw: string, nullValue: boolean): SIVICoverDrafts {
  const { original } = source(review, rowId, column);
  const expected = drafts[rowId]?.[column]?.expected ?? original;
  if (!equalCell(original, expected)) throw new Error('SIVI original cover changed; drafts retained, reload explicitly.');
  const cell = parseSIVICover(column, raw, nullValue, expected);
  return { ...drafts, [rowId]: { ...drafts[rowId], [column]: cell } };
}

export function siviCoverErrors(drafts: SIVICoverDrafts): string[] {
  return Object.values(drafts).flatMap(row => Object.values(row).flatMap(cell => cell?.error ? [cell.error] : []));
}
export function siviCoverDirty(drafts: SIVICoverDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row).some(cell => cell &&
    (cell.error !== null || !equalCell(cell.expected, cell.value))));
}
export function siviCoverHiddenDrafts(drafts: SIVICoverDrafts, extended: boolean): boolean {
  return !extended && Object.values(drafts).some(row => extendedColumns.some(column => row[column] !== undefined));
}

export function siviCoverEdits(review: SIVIProjection[], drafts: SIVICoverDrafts): SIVICoverEdit[] {
  const errors = siviCoverErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  if (siviCoverHiddenDrafts(drafts, review[0]?.Form === 'SubVegA-SIVI')) {
    throw new Error('Show extended B3/B4/B5 controls before saving their retained drafts.');
  }
  const edits: SIVICoverEdit[] = [];
  const targets = new Set<string>();
  for (const [rowId, row] of Object.entries(drafts)) {
    for (const key of Object.keys(row)) {
      const column = siviCoverColumns.find(column => column === key);
      if (!column) throw new Error('Only source-scoped SIVI cover and total controls are editable.');
      const cell = row[column];
      if (!cell) throw new Error('SIVI cover draft is incomplete.');
      const { group, original } = source(review, rowId, column);
      if (!completeCell(cell.expected) || !completeCell(cell.value) || !equalCell(original, cell.expected)) {
        throw new Error('SIVI original cover changed or draft cells are incomplete; reload explicitly.');
      }
      const target = `${rowId}:${column}`;
      if (targets.has(target)) throw new Error('Repeated physical SIVI cover targets are not permitted.');
      targets.add(target);
      const staged = stageSIVICover(review, {}, rowId, column, cell.raw, cell.nullValue)[rowId][column];
      if (!staged || staged.error || !equalCell(staged.value, cell.value)) {
        throw new Error(staged?.error || 'SIVI cover value differs from its literal draft.');
      }
      if (equalCell(cell.expected, cell.value)) continue;
      edits.push({ rowId, form: group.Form, column, expected: structuredClone(cell.expected), value: structuredClone(cell.value) });
    }
  }
  return edits;
}

export function siviCoverRequestJSON(review: SIVIProjection[], drafts: SIVICoverDrafts): string {
  const snapshot = structuredClone(review);
  const first = snapshot.flatMap(group => group.Rows)[0];
  const plot = first ? first.cells[1]?.text : '';
  if (typeof plot !== 'string') throw new Error('SIVI original plot is incomplete.');
  validateSIVIProjection(snapshot, plot, snapshot[0]?.Form === 'SubVegA-SIVI');
  return JSON.stringify({ original: snapshot, edits: siviCoverEdits(snapshot, structuredClone(drafts)) } satisfies SIVICoverRequest);
}
