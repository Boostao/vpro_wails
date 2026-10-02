import { heightField, heightValue, type HeightCell, type HeightField } from './heightEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { resolveSpeciesDecision, speciesEventError, type SpeciesChoices, type SpeciesDecision, type SpeciesDecisionKind } from './vegetationSpeciesEditor';
import type { VegetationCreationRequest, VegetationSpeciesOption } from '../bindings/github.com/boostao/vpro-wails';

export interface VegetationCreationDraft {
  editorKey: string;
  form: string;
  species: string;
  decision?: SpeciesDecision;
  cells: Partial<Record<HeightField, HeightCell>>;
}

let creationSequence = 0;

export function stageVegetationCreationSpecies(draft: VegetationCreationDraft, species: string): VegetationCreationDraft {
  return { ...draft, species, decision: undefined };
}

export function chooseVegetationCreationSpecies(draft: VegetationCreationDraft, choices: SpeciesChoices, kind: SpeciesDecisionKind, selected?: string): VegetationCreationDraft {
  if (draft.form !== choices.form || draft.species !== choices.entered || draft.decision) {
    throw new Error('Species choice no longer matches the original creation entry; review it again.');
  }
  const resolved = resolveSpeciesDecision(choices, kind, selected);
  return { ...draft, species: resolved.value, decision: resolved.decision };
}

export function vegetationCreationSpeciesError(draft: VegetationCreationDraft, options: readonly VegetationSpeciesOption[]): string | null {
  if (!wellFormedUTF16(draft.species)) return 'Species contains incomplete Unicode; the raw entry was not repaired.';
  if (draft.species.length > 8) return 'Species exceeds 8 UTF-16 units; the raw entry was not truncated.';
  if (draft.decision) {
    const { kind, entered, selected } = draft.decision;
    const target = kind === 'keep' ? entered : selected;
    const error = speciesEventError(entered) ?? speciesEventError(target ?? null);
    if (error) return error;
    if ((kind === 'keep' && selected !== undefined) || !['keep', 'replace', 'user'].includes(kind) ||
        target === undefined || draft.species !== target.toUpperCase()) {
      return 'Species no longer matches the explicit creation decision; review the entry again.';
    }
    return null;
  }
  return draft.species && options.some(option => option.code === draft.species) ? null
    : 'Select an exact canonical source-list code or review old-code/personal-list choices before saving the new row.';
}

export function beginVegetationCreation(form: string, columns: readonly string[]): VegetationCreationDraft {
  const fields = [...new Set(columns.flatMap(column => {
    const field = heightField(column);
    return field ? [field] : [];
  }))];
  if (!fields.length) throw new Error('The source form has no supported numeric creation fields.');
  return { editorKey: `creation-${++creationSequence}`, form, species: '', cells: Object.fromEntries(fields.map(field => [field, { raw: '', expected: null, value: null, error: null }])) };
}

export function stageVegetationCreation(draft: VegetationCreationDraft, column: string, raw: string): VegetationCreationDraft {
  const field = heightField(column);
  if (!field || !draft.cells[field]) throw new Error('The field does not belong to this source creation draft.');
  return { ...draft, cells: { ...draft.cells, [field]: { raw, expected: null, ...heightValue(field, raw) } } };
}

export function vegetationCreationErrors(draft: VegetationCreationDraft, options: readonly VegetationSpeciesOption[]): string[] {
  const errors = Object.values(draft.cells).flatMap(cell => cell?.error ? [cell.error] : []);
  const speciesError = vegetationCreationSpeciesError(draft, options);
  if (speciesError) errors.unshift(speciesError);
  const visible = draft.form === 'SubVegCXL' || draft.form === 'SubVegChtXL'
    ? draft.cells.cover6?.value != null
    : Object.values(draft.cells).some(cell => cell?.value != null);
  if (!visible) errors.push('Enter an explicit numeric value belonging to this source view; no zero cover is inferred.');
  return errors;
}

export function vegetationCreationRequest(draft: VegetationCreationDraft, options: readonly VegetationSpeciesOption[]): VegetationCreationRequest {
  const errors = vegetationCreationErrors(draft, options);
  if (errors.length) throw new Error(errors[0]);
  return { form: draft.form, species: draft.species,
    ...(draft.decision ? { decision: draft.decision.kind, entered: draft.decision.entered,
      ...(draft.decision.selected === undefined ? {} : { selected: draft.decision.selected }) } : {}),
    values: Object.fromEntries(Object.entries(draft.cells).map(([field, cell]) => {
      if (!cell) throw new Error('Vegetation creation contains a missing source field draft.');
      return [field, cell.value];
    })) };
}
