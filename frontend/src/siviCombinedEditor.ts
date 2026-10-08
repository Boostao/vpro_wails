import {
  siviCoverColumns, siviCoverGroups, stageSIVICover, siviCoverDirty, siviCoverErrors, siviCoverHiddenDrafts, siviCoverEdits,
  type SIVICoverColumn, type SIVICoverDrafts, type SIVICoverEdit,
} from './siviCoverEditor';
import {
  stageSIVIHeight, siviHeightDirty, siviHeightErrors, siviHeightEdits, validateSIVIProjection,
  type SIVIHeightColumn, type SIVIHeightDrafts, type SIVIHeightEdit, type SIVIProjection,
} from './siviHeightEditor';

export type SIVICombinedColumn = SIVICoverColumn | SIVIHeightColumn;
export interface SIVICombinedDrafts { covers: SIVICoverDrafts; heights: SIVIHeightDrafts }
export interface SIVICombinedRequest { original: SIVIProjection[]; edits: (SIVICoverEdit | SIVIHeightEdit)[] }

export function isSIVICoverColumn(column: string): column is SIVICoverColumn {
  return siviCoverColumns.some(value => value === column);
}
export function isSIVIHeightColumn(column: string): column is SIVIHeightColumn {
  return column === 'HeightA' || column === 'HeightB' || column === 'Height6';
}
export function siviCombinedGroups(index: number, extended: boolean): SIVICombinedColumn[][] {
  const covers = siviCoverGroups(index, extended);
  return index === 0 ? covers.map<SIVICombinedColumn[]>((columns, position) => [...columns, position === 0 ? 'HeightA' : 'HeightB'])
    : index === 1 ? covers.map<SIVICombinedColumn[]>(columns => [...columns, 'Height6']) : covers;
}
export function stageSIVICombined(review: SIVIProjection[], drafts: SIVICombinedDrafts, rowId: string,
  column: SIVICombinedColumn, raw: string, nullValue: boolean): SIVICombinedDrafts {
  if (isSIVICoverColumn(column)) {
    return { ...drafts, covers: stageSIVICover(review, drafts.covers, rowId, column, raw, nullValue) };
  }
  if (!isSIVIHeightColumn(column)) throw new Error('Only source-bound combined SIVI cover/height columns are editable.');
  return { ...drafts, heights: stageSIVIHeight(review, drafts.heights, rowId, column, raw, nullValue) };
}
export function siviCombinedErrors(drafts: SIVICombinedDrafts): string[] {
  return [...siviCoverErrors(drafts.covers), ...siviHeightErrors(drafts.heights)];
}
export function siviCombinedDirty(drafts: SIVICombinedDrafts): boolean {
  return siviCoverDirty(drafts.covers) || siviHeightDirty(drafts.heights);
}
export function siviCombinedHiddenDrafts(drafts: SIVICombinedDrafts, extended: boolean): boolean {
  return siviCoverHiddenDrafts(drafts.covers, extended);
}
export function siviCombinedEdits(review: SIVIProjection[], drafts: SIVICombinedDrafts): (SIVICoverEdit | SIVIHeightEdit)[] {
  return [...siviCoverEdits(review, drafts.covers), ...siviHeightEdits(review, drafts.heights)];
}
export function siviCombinedRequestJSON(review: SIVIProjection[], drafts: SIVICombinedDrafts): string {
  const snapshot = structuredClone(review);
  const first = snapshot.flatMap(group => group.Rows)[0];
  const plot = first ? first.cells[1]?.text : '';
  if (typeof plot !== 'string') throw new Error('SIVI original plot is incomplete.');
  validateSIVIProjection(snapshot, plot, snapshot[0]?.Form === 'SubVegA-SIVI');
  return JSON.stringify({ original: snapshot, edits: siviCombinedEdits(snapshot, structuredClone(drafts)) } satisfies SIVICombinedRequest);
}
