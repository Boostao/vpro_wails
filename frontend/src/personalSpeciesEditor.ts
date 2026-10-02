import { wellFormedUTF16 } from './qualityEditor';
import { speciesEventError, type SpeciesCell, type SpeciesChoices } from './vegetationSpeciesEditor';
import type { PersonalSpeciesDefinitionRequest, VegetationSpeciesOption } from '../bindings/github.com/boostao/vpro-wails';

export const personalLifeforms = [
  [1, 'Coniferous trees - TREE/SHRUB/SEEDLINGS'], [2, 'Broad-leaved trees - TREE/SHRUB/SEEDLINGS'],
  [3, 'Evergreen shrubs - SHRUB'], [4, 'Deciduous shrubs - SHRUB'], [5, 'Ferns - HERB'],
  [6, 'Graminoids - HERB'], [7, 'Herbs - HERB'], [8, 'Parasitic and saprophytic - HERB'],
  [9, 'Mosses - MOSS'], [10, 'Liverworts - MOSS'], [11, 'Lichens - MOSS/LICHEN'],
  [12, 'Dwarf woody plants - HERB'],
] as const;
export const personalTextFields = [
  { field: 'scientificName', label: 'Scientific Name' }, { field: 'englishName', label: 'English Name:' },
] as const;

interface PersonalTextCell { raw: string; isNull: boolean; error: string | null }
export interface PersonalSpeciesDraft {
  id: number;
  form: string;
  entered: string;
  expected: string;
  scientificName: PersonalTextCell;
  englishName: PersonalTextCell;
  lifeform: number | null;
}

export function beginPersonalSpecies(id: number, cell: SpeciesCell, choices: SpeciesChoices): PersonalSpeciesDraft {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647 || !cell.error || cell.decision ||
      cell.form !== choices.form || cell.raw !== choices.entered || speciesEventError(cell.raw) ||
      choices.aliases.some(option => option.code !== null) || choices.users.some(option => option.code !== null)) {
    throw new Error('Review an unknown source code with no usable alias or existing personal definition before creating metadata.');
  }
  return { id, form: cell.form, entered: cell.raw, expected: cell.expected,
    scientificName: { raw: '', isNull: true, error: null },
    englishName: { raw: '', isNull: true, error: null }, lifeform: null };
}

export function matchesPersonalSpeciesSource(draft: PersonalSpeciesDraft, cell: SpeciesCell | undefined): boolean {
  return cell !== undefined && cell.form === draft.form && cell.expected === draft.expected &&
    cell.raw === draft.entered && cell.decision === undefined;
}

export function stagePersonalText(draft: PersonalSpeciesDraft, field: 'scientificName' | 'englishName', raw: string, isNull: boolean): PersonalSpeciesDraft {
  const error = isNull ? null : !wellFormedUTF16(raw) ? 'Personal species metadata contains incomplete Unicode; raw input was not repaired.'
    : raw.length > 255 ? 'Personal species metadata exceeds 255 UTF-16 units; raw input was not truncated.' : null;
  return { ...draft, [field]: { raw, isNull, error } };
}

export function personalSpeciesErrors(draft: PersonalSpeciesDraft): string[] {
  return [speciesEventError(draft.entered), draft.scientificName.error, draft.englishName.error,
    draft.lifeform !== null && !personalLifeforms.some(([value]) => value === draft.lifeform)
      ? 'Select a source Lifeform or NULL; no classification is inferred.' : null].filter((error): error is string => error !== null);
}

export function personalSpeciesRequest(draft: PersonalSpeciesDraft): PersonalSpeciesDefinitionRequest {
  const errors = personalSpeciesErrors(draft);
  if (errors.length) throw new Error(errors[0]);
  return { entered: draft.entered, scientificName: draft.scientificName.isNull ? null : draft.scientificName.raw,
    englishName: draft.englishName.isNull ? null : draft.englishName.raw, lifeform: draft.lifeform };
}

export function personalSpeciesMatches(option: VegetationSpeciesOption, request: PersonalSpeciesDefinitionRequest): boolean {
  return option.code === request.entered.toUpperCase() && option.scientificName === request.scientificName &&
    option.englishName === request.englishName && option.lifeform === request.lifeform && option.codeType === null;
}

export function personalSpeciesCommittedError(cause: unknown): boolean {
  return typeof cause === 'object' && cause !== null && 'message' in cause && typeof cause.message === 'string' &&
    cause.message.startsWith('personal species definition committed, but writer cleanup failed;');
}
