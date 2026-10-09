import { finiteSingleValue } from './numericEditor';
import type { VegetationAttributeUpdate } from '../bindings/github.com/boostao/vpro-wails';

export interface VegetationAttributeField { key: string; column: string; long?: boolean; group?: string }
export const vegetationAttributeFields: readonly VegetationAttributeField[] = [
  { key: 'll', column: 'LL', long: true, group: 'ArborealLichenLoading' },
  { key: 'af', column: 'AF' },
  { key: 'dc', column: 'DC', group: 'DistributionCode' },
  { key: 'ut', column: 'UT', group: 'UtilizationCode' },
  { key: 'vi', column: 'VI', group: 'VigourCode' },
  { key: 'pv', column: 'PV', long: true, group: 'phenologyCodeVeg' },
  { key: 'pg', column: 'PG', group: 'PhenologyCodeGen' },
  { key: 'ffa', column: 'FFA', group: 'FruitFlowerAbundance' },
  { key: 'cultural1', column: 'Cultural1', group: 'Cultural1' },
  { key: 'cultural2', column: 'Cultural2', group: 'Cultural1' },
  { key: 'other1', column: 'Other1', group: 'VegOther1' },
  { key: 'other2', column: 'Other2', group: 'VegOther2' },
];
export interface VegetationAttributeCell { raw: string; expected: number | null; value: number | null; error: string | null }
export type VegetationAttributeDrafts = Record<string, Record<string, VegetationAttributeCell>>;
export function vegetationAttributeField(column: string): VegetationAttributeField | undefined {
  return vegetationAttributeFields.find(field => field.column.toLowerCase() === column.toLowerCase());
}
export function stageVegetationAttribute(drafts: VegetationAttributeDrafts, id: number, field: VegetationAttributeField, raw: string, stored: number | null): VegetationAttributeDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Vegetation row needs an exact signed32 identity.');
  const previous = drafts[String(id)]?.[field.key];
  const expected = previous ? previous.expected : stored;
  const parsed = finiteSingleValue(field.column, raw, expected);
  let error = parsed.error;
  const value = parsed.value;
  const minimum = field.long ? -2147483648 : -32768, maximum = field.long ? 2147483647 : 32767;
  if (!error && value !== null &&
      (!Number.isSafeInteger(value) || (value !== expected && (value < minimum || value > maximum)))) {
    error = `${field.column} requires an exact Access ${field.long ? 'Long' : 'Integer'} (${minimum} to ${maximum}) or NULL.`;
  }
  return { ...drafts, [String(id)]: { ...drafts[String(id)], [field.key]: { raw, expected, value, error } } };
}
export function vegetationAttributeErrors(drafts: VegetationAttributeDrafts): string[] {
  return Object.entries(drafts).flatMap(([id, row]) => Object.values(row).flatMap(cell => cell.error ? [`Vegetation row ${id}: ${cell.error}`] : []));
}
export function vegetationAttributeDirty(drafts: VegetationAttributeDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row).some(cell => cell.error !== null || cell.value !== cell.expected));
}
export function vegetationAttributeUpdates(drafts: VegetationAttributeDrafts): VegetationAttributeUpdate[] {
  const errors = vegetationAttributeErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([identity, row]) => {
    const values: VegetationAttributeUpdate['values'] = {}, expected: VegetationAttributeUpdate['expected'] = {};
    for (const field of vegetationAttributeFields) {
      const cell = row[field.key];
      if (!cell || cell.value === cell.expected) continue;
      values[field.key] = cell.value;
      expected[field.key] = cell.expected;
    }
    return Object.keys(values).length ? [{ id: Number(identity), values, expected }] : [];
  });
}
