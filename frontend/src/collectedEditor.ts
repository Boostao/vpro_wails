export interface CollectedCell {
  form?: string;
  expected: string | null;
  value: string | null;
  clicks: number;
}
export type CollectedDrafts = Record<string, CollectedCell>;
export interface CollectedUpdate {
  id: number;
  expected: string | null;
  clicks: number;
}
export function collectedNext(value: string | null): string | null {
  if (value === null) return 'C';
  if (value === 'C' || value === 'c' || value === '\uFF23' || value === '\uFF43') return 'V';
  if (value === 'V' || value === 'v' || value === '\uFF36' || value === '\uFF56') return null;
  return value;
}
export function stageCollected(drafts: CollectedDrafts, id: number, stored: string | null, form?: string): CollectedDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Collected row needs an exact signed32 identity.');
  const previous = drafts[String(id)];
  const expected = previous ? previous.expected : stored;
  return { ...drafts, [String(id)]: {
    form: previous?.form ?? form,
    expected, value: collectedNext(previous ? previous.value : stored), clicks: previous ? previous.clicks % 3 + 1 : 1,
  } };
}
export function collectedDirty(drafts: CollectedDrafts): boolean {
  return Object.values(drafts).some(cell => cell.value !== cell.expected);
}
export function collectedUpdates(drafts: CollectedDrafts): CollectedUpdate[] {
  return Object.entries(drafts).flatMap(([id, cell]) => cell.value === cell.expected ? []
    : [{ id: Number(id), expected: cell.expected, clicks: cell.clicks }]);
}
