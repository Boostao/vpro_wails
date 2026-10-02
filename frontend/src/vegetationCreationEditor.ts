import { heightField, heightValue, type HeightCell, type HeightField } from './heightEditor';
import type { VegetationCreationRequest, VegetationSpeciesOption } from '../bindings/github.com/boostao/vpro-wails';

export interface VegetationCreationDraft {
  form: string;
  species: string;
  cells: Partial<Record<HeightField, HeightCell>>;
}

export function beginVegetationCreation(form: string, columns: readonly string[]): VegetationCreationDraft {
  const fields = [...new Set(columns.flatMap(column => {
    const field = heightField(column);
    return field ? [field] : [];
  }))];
  if (!fields.length) throw new Error('The source form has no supported numeric creation fields.');
  return { form, species: '', cells: Object.fromEntries(fields.map(field => [field, { raw: '', expected: null, value: null, error: null }])) };
}

export function stageVegetationCreation(draft: VegetationCreationDraft, column: string, raw: string): VegetationCreationDraft {
  const field = heightField(column);
  if (!field || !draft.cells[field]) throw new Error('The field does not belong to this source creation draft.');
  return { ...draft, cells: { ...draft.cells, [field]: { raw, expected: null, ...heightValue(field, raw) } } };
}

export function vegetationCreationErrors(draft: VegetationCreationDraft, options: readonly VegetationSpeciesOption[]): string[] {
  const errors = Object.values(draft.cells).flatMap(cell => cell?.error ? [cell.error] : []);
  if (!draft.species || !options.some(option => option.code === draft.species)) {
    errors.unshift('Select an exact canonical source-list species code; unknown/personal creation and implicit alias decisions are unavailable.');
  }
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
    values: Object.fromEntries(Object.entries(draft.cells).map(([field, cell]) => {
      if (!cell) throw new Error('Vegetation creation contains a missing source field draft.');
      return [field, cell.value];
    })) };
}
