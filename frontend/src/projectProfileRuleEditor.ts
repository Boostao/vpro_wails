import type { ProjectMetadataCell, ProjectMetadataEditorField, ProjectPlotProfileEdit, ProjectPlotProfileChoices } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell, metadataCellText, parseMetadataCell, type MetadataDraftCell } from './projectMetadataEditor';
import { profileCellLabel, type ValidatedProjectPlotProfileReview, type ProfileReviewTable } from './projectPlotProfileReview';

export const profileEditableFields = ['Order', 'Table', 'Field', 'Operator', 'Layer', 'Species', 'Criteria', 'Operation'] as const;
export type ProfileField = typeof profileEditableFields[number];
export const profileLayers = ['SumAll', 'SumA', 'SumB', 'Any', '1', '2', '3', '4', '5', '5a', '5b', '5c', '6', '7'];
export interface ProfileRulesDraft {
  review: ValidatedProjectPlotProfileReview;
  cells: Record<string, Partial<Record<ProfileField, MetadataDraftCell>>>;
}
export interface ProfileRuleChoices {
  envFields: string[];
  species: ProjectMetadataCell[];
}

function originalCell(draft: ProfileRulesDraft, rowId: string, name: ProfileField): ProjectMetadataCell {
  const rows = draft.review.rules.rows.filter(row => row.rowId === rowId);
  const index = draft.review.rules.columns.findIndex(column => column.name === name);
  if (rows.length !== 1 || index < 0 || !rows[0].cells[index]) throw new Error('Select one reviewed physical profile rule and literal field.');
  return rows[0].cells[index];
}

export function profileRuleValue(draft: ProfileRulesDraft, rowId: string, name: ProfileField): MetadataDraftCell {
  const staged = draft.cells[rowId]?.[name];
  if (staged) return staged;
  const value = originalCell(draft, rowId, name);
  return { value, raw: metadataCellText(value), nullValue: value.storage === 'null', error: null };
}

export function profileRuleOptions(draft: ProfileRulesDraft, rowId: string, name: ProfileField,
    choices?: ProfileRuleChoices | null, lump?: ProfileReviewTable | null): string[] {
  const table = profileRuleValue(draft, rowId, 'Table').value.text?.toLowerCase();
  if (name === 'Table') return ['Env', 'Veg', 'Lump'];
  if (name === 'Operation') return ['Add plots', 'Subtract plots', 'Common plots'];
  if (name === 'Operator') return table === 'env' ? ['=', '>', '<', 'Like', 'Not Like'] : table === 'veg' || table === 'lump' ? ['=', '>', '<'] : [];
  if (name === 'Layer') return table === 'veg' || table === 'lump' ? profileLayers : [];
  if (name === 'Field') return table === 'veg' ? ['Species'] : table === 'lump' ? ['LumpCode'] : table === 'env' ? choices?.envFields ?? [] : [];
  if (name === 'Species') {
    const index = lump?.columns.findIndex(column => column.name === 'LumpCode') ?? -1;
    const cells = table === 'veg' ? choices?.species ?? [] : table === 'lump' && index >= 0
      ? lump?.rows.map(row => row.cells[index]) ?? [] : [];
    return [...new Set(cells.flatMap(cell => cell.storage === 'text' && cell.text !== null ? [cell.text] : []))];
  }
  return [];
}

export function stageProfileRule(draft: ProfileRulesDraft, rowId: string, name: ProfileField, raw: string, nullValue: boolean): ProfileRulesDraft {
  const options = profileRuleOptions(draft, rowId, name);
  const field: ProjectMetadataEditorField = { name, kind: name === 'Order' ? 'integer' : 'text',
    maximum: name === 'Order' ? 16 : 255, collection: false, referenceList: 'source profile choices',
    referenceColumn: 'value', limitToList: name === 'Operator' || name === 'Layer',
    options: options.map((value, index) => ({ rowId: String(index), value, description: null })) };
  const parsed = parseMetadataCell(field, raw, nullValue, originalCell(draft, rowId, name));
  let next: ProfileRulesDraft = { ...draft, cells: { ...draft.cells, [rowId]: { ...draft.cells[rowId], [name]: parsed } } };
  if (name === 'Table' && parsed.error === null && !nullValue &&
      !equalCell(parsed.value, originalCell(draft, rowId, name)) && (raw.toLowerCase() === 'veg' || raw.toLowerCase() === 'lump')) {
    next = stageProfileRule(next, rowId, 'Field', raw.toLowerCase() === 'veg' ? 'Species' : 'LumpCode', false);
  }
  if (name === 'Table') {
    for (const dependent of ['Layer', 'Operator'] as const) {
      const cell = next.cells[rowId]?.[dependent];
      if (cell) next = stageProfileRule(next, rowId, dependent, cell.raw, cell.nullValue);
    }
  }
  return next;
}

export function profileRuleErrors(draft: ProfileRulesDraft): string[] {
  return Object.values(draft.cells).flatMap(fields => Object.values(fields).flatMap(cell => cell?.error ? [cell.error] : []));
}

export function profileRulesDirty(draft: ProfileRulesDraft): boolean {
  return draft.review.rules.rows.some(row => profileEditableFields.some(name => {
    const cell = draft.cells[row.rowId]?.[name];
    return !!cell && (cell.error !== null || !equalCell(cell.value, originalCell(draft, row.rowId, name)));
  }));
}

export function profileRuleEditRequest(draft: ProfileRulesDraft): ProjectPlotProfileEdit {
  if (profileRuleErrors(draft).length) throw new Error('Correct or Undo every raw profile error before saving.');
  const drafts = draft.review.rules.rows.flatMap(row => {
    const changes = profileEditableFields.flatMap(name => {
      const cell = draft.cells[row.rowId]?.[name];
      return cell && !equalCell(cell.value, originalCell(draft, row.rowId, name)) ? [{ column: name, value: cell.value }] : [];
    });
    return changes.length ? [{ rowId: row.rowId, changes }] : [];
  });
  if (!drafts.length) throw new Error('No changed profile assignment is ready; stored counts and unchanged historical values are not rewritten.');
  return { originalRules: draft.review.rules, drafts };
}

export function validateProfileChoices(choices: ProjectPlotProfileChoices): ProfileRuleChoices {
  if (!Array.isArray(choices.envFields) || choices.envFields.length === 0 ||
      choices.envFields.some(field => typeof field !== 'string' || !field) ||
      new Set(choices.envFields).size !== choices.envFields.length || !Array.isArray(choices.species)) {
    throw new Error('Complete selected-project fields and source species suggestions are required.');
  }
  choices.species.forEach(profileCellLabel);
  return { envFields: choices.envFields, species: choices.species };
}
