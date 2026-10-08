import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { siviChildPhysicalRows, type SIVIProjection } from './siviHeightEditor';

export interface SIVIIdentityCell {
  raw: string;
  nullValue: boolean;
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
  error: string | null;
}
export type SIVIIdentityDrafts = Record<string, SIVIIdentityCell>;
export interface SIVIIdentityEdit {
  rowId: string;
  form: string;
  column: 'ID';
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
}

function signed32(raw: string): boolean {
  const value = Number(raw);
  return /^-?(0|[1-9]\d*)$/.test(raw) && Number.isInteger(value) &&
    value >= -2147483648 && value <= 2147483647 && String(value) === raw;
}

export function siviIdentityEditable(cell: ProjectMetadataCell): boolean {
  return completeCell(cell) && (cell.storage === 'null' ||
    cell.storage === 'integer' && cell.integer !== null && signed32(cell.integer));
}

export function siviIdentityRows(review: readonly SIVIProjection[]) {
  return siviChildPhysicalRows(review, 'SIVI identity');
}

function source(review: readonly SIVIProjection[], rowId: string) {
  const rows = siviIdentityRows(review).filter(({ row }) => row.rowId === rowId);
  const owner = rows[0];
  const group = owner?.contexts[0]?.group;
  const value = group && owner.row.cells[group.Columns.indexOf('ID')];
  if (!exactSigned64(rowId) || rows.length !== 1 || !group || !completeCell(value)) {
    throw new Error('SIVI identity source is unavailable; reload before editing.');
  }
  return { form: group.Form, value };
}

function parsed(raw: string, nullValue: boolean, expected: ProjectMetadataCell) {
  let value: ProjectMetadataCell = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  let error: string | null = null;
  if (!wellFormedUTF16(raw)) error = 'ID contains incomplete Unicode; raw input was not repaired.';
  else if (!nullValue && expected.storage !== 'null' && raw === metadataCellText(expected)) value = structuredClone(expected);
  else if (!siviIdentityEditable(expected)) error = 'Historical unsupported ID storage is read-only; no identity was repaired.';
  else if (!nullValue) {
    if (!signed32(raw)) error = 'ID requires an exact signed32 integer or explicit NULL; no trimming or coercion is applied.';
    else value = { ...value, storage: 'integer', integer: raw };
  }
  return { value, error };
}

export function stageSIVIIdentity(review: readonly SIVIProjection[], drafts: SIVIIdentityDrafts,
  rowId: string, raw: string, nullValue: boolean): SIVIIdentityDrafts {
  const original = source(review, rowId).value;
  const expected = drafts[rowId]?.expected ?? original;
  if (!completeCell(expected) || !equalCell(expected, original)) {
    throw new Error('SIVI original ID changed; retained drafts require explicit reload.');
  }
  const result = parsed(raw, nullValue, expected);
  return { ...drafts, [rowId]: { raw, nullValue, expected: structuredClone(expected), ...result } };
}

export function siviIdentityErrors(drafts: SIVIIdentityDrafts, review?: readonly SIVIProjection[]): string[] {
  const errors = Object.values(drafts).flatMap(cell => {
    if (!cell || typeof cell.raw !== 'string' || typeof cell.nullValue !== 'boolean' ||
        !completeCell(cell.expected) || !completeCell(cell.value)) {
      return ['SIVI identity draft has incomplete intent or raw cells; Undo/reload explicitly.'];
    }
    const result = parsed(cell.raw, cell.nullValue, cell.expected);
    if (result.error !== cell.error || !equalCell(result.value, cell.value)) {
      return ['SIVI identity draft differs from its retained raw intent; Undo/reload explicitly.'];
    }
    return cell.error ? [cell.error] : [];
  });
  if (errors.length) return errors;
  const claimed = new Set<string>();
  for (const cell of Object.values(drafts)) {
    if (!equalCell(cell.expected, cell.value) && cell.value.storage === 'integer' && cell.value.integer !== null) {
      if (claimed.has(cell.value.integer)) errors.push('SIVI ID is repeated in this draft; no identity is reassigned.');
      claimed.add(cell.value.integer);
    }
  }
  if (review && Object.keys(drafts).length) {
    const rows = review.some(group => group.Rows.length) ? siviIdentityRows(review) : [];
    for (const [rowId, cell] of Object.entries(drafts)) {
      const owner = rows.find(({ row }) => row.rowId === rowId);
      if (!exactSigned64(rowId) || !owner || !equalCell(owner.row.cells[0], cell.expected)) {
        errors.push('SIVI original ID changed; retained drafts require explicit reload.');
      } else if (!equalCell(cell.expected, cell.value) && cell.value.storage === 'integer' &&
          rows.some(({ row }) => row.rowId !== rowId && row.cells[0].storage === 'integer' &&
            row.cells[0].integer === cell.value.integer)) {
        errors.push('SIVI ID belongs to another source row; identity swaps are unavailable.');
      }
    }
  }
  return errors;
}

export function siviIdentityDirty(drafts: SIVIIdentityDrafts): boolean {
  return siviIdentityErrors(drafts).length > 0 || Object.values(drafts).some(cell => !equalCell(cell.expected, cell.value));
}

export function siviIdentityEdits(review: readonly SIVIProjection[], drafts: SIVIIdentityDrafts): SIVIIdentityEdit[] {
  const errors = siviIdentityErrors(drafts, review);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([rowId, cell]) => {
    const original = source(review, rowId);
    if (!equalCell(original.value, cell.expected)) throw new Error('SIVI original ID changed; reload explicitly.');
    if (equalCell(cell.expected, cell.value)) return [];
    return [{ rowId, form: original.form, column: 'ID' as const,
      expected: structuredClone(cell.expected), value: structuredClone(cell.value) }];
  });
}

export function siviIdentityRequestJSON(review: readonly SIVIProjection[], drafts: SIVIIdentityDrafts): string {
  const original = structuredClone(review);
  siviIdentityRows(original);
  return JSON.stringify({ original, edits: siviIdentityEdits(original, drafts) });
}
