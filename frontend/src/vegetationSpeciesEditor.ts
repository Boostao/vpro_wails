import { wellFormedUTF16 } from './qualityEditor';
import type { VegetationSpeciesOption, VegetationSpeciesUpdate } from '../bindings/github.com/boostao/vpro-wails';

export const speciesForms = ['SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL', 'SubVegAhtXL', 'SubVegChtXL'] as const;
export interface SpeciesCell { form: string; raw: string; expected: string; error: string | null }
export type SpeciesDrafts = Record<string, SpeciesCell>;
export type SpeciesLists = Record<string, VegetationSpeciesOption[]>;

export function stageSpecies(drafts: SpeciesDrafts, form: string, id: number, raw: string, stored: string, lists: SpeciesLists): SpeciesDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Species row needs an exact signed32 identity.');
  if (!speciesForms.some(name => name === form)) throw new Error('Species source form is unavailable.');
  const expected = drafts[String(id)]?.expected ?? stored;
  let error: string | null = null;
  if (raw !== expected) {
    if (!wellFormedUTF16(raw)) error = 'Species contains incomplete Unicode; the raw entry was not repaired.';
    else if (raw.length > 8) error = 'Species exceeds 8 UTF-16 units; the raw entry was not truncated.';
    else if (!lists[form]) error = 'Species references are unavailable; Retry before saving.';
    else if (!lists[form].some(option => option.code !== null && option.code === raw)) {
      error = 'Select an exact species code from this source list. Old-code decisions and personal-list creation are not yet available.';
    }
  }
  return { ...drafts, [String(id)]: { form, raw, expected, error } };
}
export function speciesErrors(drafts: SpeciesDrafts): string[] {
  return Object.entries(drafts).flatMap(([id, cell]) => cell.error ? [`Vegetation row ${id}: ${cell.error}`] : []);
}
export function speciesDirty(drafts: SpeciesDrafts): boolean {
  return Object.values(drafts).some(cell => cell.error !== null || cell.raw !== cell.expected);
}
export function speciesUpdates(drafts: SpeciesDrafts): VegetationSpeciesUpdate[] {
  const errors = speciesErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.entries(drafts).flatMap(([id, cell]) => cell.raw === cell.expected ? []
    : [{ id: Number(id), form: cell.form, expected: cell.expected, value: cell.raw }]);
}
