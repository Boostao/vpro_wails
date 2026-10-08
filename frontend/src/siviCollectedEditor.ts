import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { collectedNext } from './collectedEditor';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { equalCell } from './projectMetadataEditor';
import { validateSIVIProjection, type SIVIProjection, type SIVIRow } from './siviHeightEditor';

export interface SIVICollectedCell {
  expected: ProjectMetadataCell;
  value: ProjectMetadataCell;
  clicks: number;
}
export type SIVICollectedDrafts = Record<string, SIVICollectedCell>;
export interface SIVICollectedClick {
  rowId: string;
  form: string;
  expected: ProjectMetadataCell;
  clicks: number;
}

export function siviCollectedRows(review: readonly SIVIProjection[]): { group: SIVIProjection; row: SIVIRow }[] {
  if (!Array.isArray(review)) throw new Error('SIVI Collected source groups are unavailable.');
  const first = review.flatMap(group => Array.isArray(group?.Rows) ? group.Rows : [])[0];
  const plot = first?.cells?.[1]?.text;
  if (typeof plot !== 'string') throw new Error('SIVI Collected source plot is unavailable.');
  const snapshot = validateSIVIProjection([...review], plot, review[0]?.Form === 'SubVegA-SIVI');
  const seen = new Set<string>();
  return snapshot.flatMap(group => group.Rows.flatMap(row => {
    if (seen.has(row.rowId)) return [];
    seen.add(row.rowId);
    return [{ group, row }];
  }));
}

function original(review: readonly SIVIProjection[], rowId: string) {
  const owned = siviCollectedRows(review).filter(({ row }) => row.rowId === rowId);
  const value = owned[0]?.row.cells[owned[0].group.Columns.indexOf('Collected')];
  if (!exactSigned64(rowId) || owned.length !== 1 || !completeCell(value)) {
    throw new Error('SIVI Collected physical source row is unavailable; reload before cycling.');
  }
  return { form: owned[0].group.Form, value };
}

function afterClicks(expected: ProjectMetadataCell, clicks: number): ProjectMetadataCell {
  if (!completeCell(expected) || !Number.isInteger(clicks) || clicks < 1 || clicks > 3) {
    throw new Error('SIVI Collected requires complete original cells and one to three cycle clicks.');
  }
  if (expected.storage !== 'text' && expected.storage !== 'null') {
    throw new Error('Historical non-text SIVI Collected is read-only; no value was coerced.');
  }
  let value = expected.storage === 'null' ? null : expected.text;
  for (let index = 0; index < clicks; index++) value = collectedNext(value);
  if (value === (expected.storage === 'null' ? null : expected.text)) return structuredClone(expected);
  return { storage: value === null ? 'null' : 'text', text: value, integer: null, real: null, blobHex: null };
}

export function stageSIVICollected(review: readonly SIVIProjection[], drafts: SIVICollectedDrafts, rowId: string): SIVICollectedDrafts {
  const source = original(review, rowId);
  const previous = drafts[rowId];
  const expected = previous?.expected ?? source.value;
  if (!completeCell(expected) || !equalCell(expected, source.value)) {
    throw new Error('SIVI Collected original changed; retained clicks require explicit reload.');
  }
  if (previous && !equalCell(previous.value, afterClicks(expected, previous.clicks))) {
    throw new Error('SIVI Collected draft differs from its retained cycle intent.');
  }
  const clicks = previous ? previous.clicks % 3 + 1 : 1;
  return { ...drafts, [rowId]: { expected: structuredClone(expected), value: afterClicks(expected, clicks), clicks } };
}

export function siviCollectedErrors(drafts: SIVICollectedDrafts): string[] {
  return Object.values(drafts).flatMap(cell => {
    if (!cell || !completeCell(cell.expected) || !completeCell(cell.value) ||
        !Number.isInteger(cell.clicks) || cell.clicks < 1 || cell.clicks > 3) {
      return ['SIVI Collected draft has incomplete cells or invalid cycle intent; Undo/reload explicitly.'];
    }
    if (cell.expected.storage !== 'text' && cell.expected.storage !== 'null') {
      return ['Historical non-text SIVI Collected is read-only; no value was coerced.'];
    }
    if (!equalCell(afterClicks(cell.expected, cell.clicks), cell.value)) {
      return ['SIVI Collected draft differs from its retained cycle intent; Undo/reload explicitly.'];
    }
    return [];
  });
}

export function siviCollectedDirty(drafts: SIVICollectedDrafts): boolean {
  return siviCollectedErrors(drafts).length > 0 || Object.values(drafts).some(cell => !equalCell(cell.expected, cell.value));
}

export function siviCollectedEdits(review: readonly SIVIProjection[], drafts: SIVICollectedDrafts): SIVICollectedClick[] {
  const errors = siviCollectedErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([rowId, cell]) => {
    const source = original(review, rowId);
    if (!equalCell(source.value, cell.expected) || !equalCell(afterClicks(cell.expected, cell.clicks), cell.value)) {
      throw new Error('SIVI Collected original or retained cycle intent changed; reload explicitly.');
    }
    return equalCell(cell.expected, cell.value) ? [] : [{
      rowId, form: source.form, expected: structuredClone(cell.expected), clicks: cell.clicks,
    }];
  });
}

export function siviCollectedRequestJSON(review: readonly SIVIProjection[], drafts: SIVICollectedDrafts): string {
  const snapshot = structuredClone(review);
  siviCollectedRows(snapshot);
  return JSON.stringify({ original: snapshot, edits: siviCollectedEdits(snapshot, drafts) });
}
