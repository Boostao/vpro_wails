import { wellFormedUTF16 } from './qualityEditor';
import type { VegetationSpeciesAlias, VegetationSpeciesOption, VegetationSpeciesUpdate } from '../bindings/github.com/boostao/vpro-wails';

export const speciesForms = ['SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL', 'SubVegAhtXL', 'SubVegChtXL'] as const;
export type SpeciesDecisionKind = 'replace' | 'keep' | 'user';
export interface SpeciesDecision { kind: SpeciesDecisionKind; entered: string; selected?: string }
export interface SpeciesCell { form: string; raw: string; expected: string; error: string | null; decision?: SpeciesDecision }
export interface SpeciesChoices { form: string; entered: string; aliases: VegetationSpeciesAlias[]; users: VegetationSpeciesOption[] }
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
      error = 'Select an exact source-list code or review old-code/personal-list choices before saving the plot.';
    }
  }
  return { ...drafts, [String(id)]: { form, raw, expected, error } };
}
export function speciesEventError(code: string | null): string | null {
  if (code === null || code === '') return 'The source code is NULL or empty.';
  if (!wellFormedUTF16(code)) return 'Species contains incomplete Unicode; the raw entry was not repaired.';
  if (code.length > 8) return 'Species exceeds 8 UTF-16 units; the raw entry was not truncated.';
  if (/[^\x00-\x7f]/.test(code)) return 'Species event case conversion for non-ASCII codes is not yet verified; select an exact source-list code instead.';
  return null;
}
export function chooseSpecies(drafts: SpeciesDrafts, id: number, choices: SpeciesChoices, kind: SpeciesDecisionKind, selected?: string): SpeciesDrafts {
  const cell = drafts[String(id)];
  if (!cell || cell.form !== choices.form || cell.raw !== choices.entered || cell.decision) {
    throw new Error('Species choice no longer matches the original editor entry; review it again.');
  }
  const aliases = choices.aliases.filter(option => option.code !== null);
  if ((kind === 'keep' && (aliases.length === 0 || selected !== undefined)) ||
      (kind === 'replace' && !aliases.some(option => option.code === selected)) ||
      (kind === 'user' && (aliases.length > 0 || !choices.users.some(option => option.code !== null && option.code === selected)))) {
    throw new Error('Species decision is not available in these source definitions.');
  }
  const enteredError = speciesEventError(choices.entered);
  const target = kind === 'keep' ? choices.entered : selected;
  const targetError = speciesEventError(target ?? null);
  if (enteredError || targetError || target === undefined) throw new Error(enteredError ?? targetError ?? 'Select an explicit species code.');
  return { ...drafts, [String(id)]: {
    ...cell, raw: target.toUpperCase(), error: null, decision: { kind, entered: choices.entered, ...(selected === undefined ? {} : { selected }) },
  } };
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
    : [{ id: Number(id), form: cell.form, expected: cell.expected, value: cell.raw,
      ...(cell.decision ? { decision: cell.decision.kind, entered: cell.decision.entered,
        ...(cell.decision.selected === undefined ? {} : { selected: cell.decision.selected }) } : {}),
    }]);
}
