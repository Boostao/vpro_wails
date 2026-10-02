import { finiteSingleValue } from './numericEditor';
import type { VegetationNumberUpdate } from '../bindings/github.com/boostao/vpro-wails';
export { singleMaximum } from './numericEditor';

export const heightFields = ['cover1', 'cover2', 'cover3', 'totalA', 'cover4', 'cover5', 'totalB',
  'cover6', 'cover7', 'cover8', 'cover9', 'height1', 'height2', 'height3', 'height4', 'height5', 'height6'] as const;
export type HeightField = typeof heightFields[number];
export interface HeightUpdate {
  id: number;
  values: Record<string, number | null>;
  expected: Record<string, number | null>;
}
export interface HeightCell {
  form?: string;
  raw: string;
  expected: number | null;
  value: number | null;
  error: string | null;
}
export type HeightDrafts = Record<string, Partial<Record<HeightField, HeightCell>>>;
export function heightField(column: string): HeightField | undefined {
  return heightFields.find(field => field.toLowerCase() === column.toLowerCase());
}
export function heightValue(field: HeightField, raw: string): { value: number | null; error: string | null } {
  const parsed = finiteSingleValue(field, raw);
  if (parsed.error || parsed.value === null) return parsed;
  const value = parsed.value;
  if (!field.startsWith('height') && value >= 100) {
    return { value: null, error: `${field} must be less than 100 or NULL; 100 is not truncated to 10.` };
  }
  return parsed;
}
export function stageHeight(drafts: HeightDrafts, id: number, field: HeightField, raw: string, stored: number | null, form?: string): HeightDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Height row needs an exact signed32 identity.');
  const previous = drafts[String(id)]?.[field];
  const expected = previous ? previous.expected : stored;
  const parsed = heightValue(field, raw);
  const row = { ...drafts[String(id)], [field]: { raw, expected, ...parsed, form: previous?.form ?? form } };
  return { ...drafts, [String(id)]: row };
}
export function heightErrors(drafts: HeightDrafts): string[] {
  return Object.values(drafts).flatMap(row => Object.values(row).flatMap(cell => cell?.error ? [cell.error] : []));
}
export function heightDirty(drafts: HeightDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row).some(cell => cell && (cell.error !== null || cell.value !== cell.expected)));
}
export function heightUpdates(drafts: HeightDrafts): HeightUpdate[] {
  const errors = heightErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([id, row]) => {
    const values: Record<string, number | null> = {};
    const expected: Record<string, number | null> = {};
    for (const field of heightFields) {
      const cell = row[field];
      if (cell && cell.value !== cell.expected) {
        values[field] = cell.value;
        expected[field] = cell.expected;
      }
    }
    return Object.keys(values).length ? [{ id: Number(id), values, expected }] : [];
  });
}
export function vegetationNumberUpdates(drafts: HeightDrafts): VegetationNumberUpdate[] {
  return heightUpdates(drafts).map(update => {
    const forms: Record<string, string> = {};
    for (const property of Object.keys(update.values)) {
      const field = heightField(property);
      const form = field ? drafts[String(update.id)]?.[field]?.form : undefined;
      if (!form) throw new Error('Vegetation numbers require the original source form for every changed field.');
      forms[property] = form;
    }
    return { ...update, forms };
  });
}
