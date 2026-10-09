import { wellFormedUTF16 } from './qualityEditor';
import type { OtherRecordUpdate } from '../bindings/github.com/boostao/vpro-wails';

export const otherFields = [
  { key: 'dataName', column: 'DataName', kind: 'text', maximum: 50 },
  { key: 'dataItem', column: 'DataItem', kind: 'text', maximum: 255 },
  { key: 'userItem1', column: 'UserItem1', kind: 'text', maximum: 255 },
  { key: 'userItem2', column: 'UserItem2', kind: 'text', maximum: 255 },
  { key: 'userItem3', column: 'UserItem3', kind: 'text', maximum: 255 },
  { key: 'userFlag1', column: 'UserFlag1', kind: 'flag' },
  { key: 'userFlag2', column: 'UserFlag2', kind: 'flag' },
  { key: 'userFlag3', column: 'UserFlag3', kind: 'flag' },
] as const;
export type OtherField = typeof otherFields[number];
export type OtherValue = string | boolean | null;
export interface OtherCell { raw: OtherValue; expected: OtherValue; value: OtherValue; error: string | null }
export type OtherDrafts = Record<string, Partial<Record<OtherField['key'], OtherCell>>>;
export function otherField(column: string): OtherField | undefined {
  return otherFields.find(field => field.column.toLowerCase() === column.toLowerCase());
}
export function stageOther(drafts: OtherDrafts, id: number, field: OtherField, raw: OtherValue, stored: OtherValue): OtherDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Other row needs an exact signed32 identity.');
  const previous = drafts[String(id)]?.[field.key];
  const expected = previous ? previous.expected : stored;
  let value: OtherValue = raw;
  let error: string | null = null;
  if (field.kind === 'text') {
    if ((raw !== null && typeof raw !== 'string') || (expected !== null && typeof expected !== 'string')) {
      throw new Error(`${field.column} requires nullable text.`);
    }
    value = raw === '' ? null : raw;
    if (value !== null && value !== expected) {
      if (!wellFormedUTF16(value)) error = `${field.column} contains incomplete Unicode; the raw entry was not repaired.`;
      else if (value.length > field.maximum) error = `${field.column} exceeds ${field.maximum} UTF-16 units; the raw entry was not truncated.`;
    }
  } else if ((raw !== null && typeof raw !== 'boolean') || (expected !== null && typeof expected !== 'boolean')) {
    throw new Error(`${field.column} requires a nullable flag.`);
  }
  return { ...drafts, [String(id)]: { ...drafts[String(id)], [field.key]: { raw, expected, value, error } } };
}
export function otherErrors(drafts: OtherDrafts): string[] {
  return Object.entries(drafts).flatMap(([id, row]) => Object.values(row).flatMap(cell => cell?.error ? [`Other row ${id}: ${cell.error}`] : []));
}
export function otherDirty(drafts: OtherDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row).some(cell => cell && (cell.error !== null || cell.value !== cell.expected)));
}
export function otherUpdates(drafts: OtherDrafts): OtherRecordUpdate[] {
  const errors = otherErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([id, row]) => {
    const text: OtherRecordUpdate['text'] = {}, flags: OtherRecordUpdate['flags'] = {};
    for (const field of otherFields) {
      const cell = row[field.key];
      if (!cell || cell.value === cell.expected) continue;
      if (field.kind === 'text') {
        if ((cell.value !== null && typeof cell.value !== 'string') || (cell.expected !== null && typeof cell.expected !== 'string')) {
          throw new Error(`${field.column} requires nullable text.`);
        }
        text[field.key] = { value: cell.value, expected: cell.expected };
      } else {
        if ((cell.value !== null && typeof cell.value !== 'boolean') || (cell.expected !== null && typeof cell.expected !== 'boolean')) {
          throw new Error(`${field.column} requires a nullable flag.`);
        }
        flags[field.key] = { value: cell.value, expected: cell.expected };
      }
    }
    return Object.keys(text).length || Object.keys(flags).length ? [{ id: Number(id), text, flags }] : [];
  });
}
