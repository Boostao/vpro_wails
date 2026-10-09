import type { ProjectMetadataCell, ProjectMetadataEditorField, ProjectMetadataRow, ProjectPlotProfileEdit, ProjectPlotProfileChoices,
  ProjectPlotProfileCreation, ProjectPlotProfileDeletion } from '../bindings/github.com/boostao/vpro-wails';
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
export interface ProfileRuleCreationDraft {
  review: ValidatedProjectPlotProfileReview;
  cells: Partial<Record<ProfileField, MetadataDraftCell>>;
}
const profileNullCell: ProjectMetadataCell = { storage: 'null', text: null, integer: null, real: null, blobHex: null };

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
  return profileSourceOptions(profileRuleValue(draft, rowId, 'Table').value.text, name, choices, lump);
}

function profileSourceOptions(tableText: string | null | undefined, name: ProfileField,
    choices?: ProfileRuleChoices | null, lump?: ProfileReviewTable | null): string[] {
  const table = tableText?.toLowerCase();
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

function parseProfileRuleField(name: ProfileField, raw: string, nullValue: boolean,
    original: ProjectMetadataCell, options: string[]): MetadataDraftCell {
  const field: ProjectMetadataEditorField = { name, kind: name === 'Order' ? 'integer' : 'text',
    maximum: name === 'Order' ? 16 : 255, collection: false, referenceList: 'source profile choices',
    referenceColumn: 'value', limitToList: name === 'Operator' || name === 'Layer',
    options: options.map((value, index) => ({ rowId: String(index), value, description: null })) };
  return parseMetadataCell(field, raw, nullValue, original);
}

export function stageProfileRule(draft: ProfileRulesDraft, rowId: string, name: ProfileField, raw: string, nullValue: boolean): ProfileRulesDraft {
  const parsed = parseProfileRuleField(name, raw, nullValue, originalCell(draft, rowId, name), profileRuleOptions(draft, rowId, name));
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

export function beginProfileRuleCreation(review: ValidatedProjectPlotProfileReview): ProfileRuleCreationDraft {
  if (review.rules.columns.length !== 9) throw new Error('Rule creation is unavailable for unmapped extra/default fields.');
  return { review, cells: {} };
}

export function profileCreationValue(draft: ProfileRuleCreationDraft, name: ProfileField): MetadataDraftCell {
  return draft.cells[name] ?? { value: profileNullCell, raw: '', nullValue: true, error: null };
}

export function profileCreationOptions(draft: ProfileRuleCreationDraft, name: ProfileField,
    choices?: ProfileRuleChoices | null, lump?: ProfileReviewTable | null): string[] {
  return profileSourceOptions(profileCreationValue(draft, 'Table').value.text, name, choices, lump);
}

export function stageProfileRuleCreation(draft: ProfileRuleCreationDraft, name: ProfileField, raw: string, nullValue: boolean): ProfileRuleCreationDraft {
  const parsed = parseProfileRuleField(name, raw, nullValue, profileNullCell, profileCreationOptions(draft, name));
  let next = { ...draft, cells: { ...draft.cells, [name]: parsed } };
  if (name === 'Table' && parsed.error === null && !nullValue && (raw.toLowerCase() === 'veg' || raw.toLowerCase() === 'lump')) {
    next = stageProfileRuleCreation(next, 'Field', raw.toLowerCase() === 'veg' ? 'Species' : 'LumpCode', false);
  }
  if (name === 'Table') {
    for (const dependent of ['Layer', 'Operator'] as const) {
      const cell = next.cells[dependent];
      if (cell) next = stageProfileRuleCreation(next, dependent, cell.raw, cell.nullValue);
    }
  }
  return next;
}

export function profileCreationErrors(draft: ProfileRuleCreationDraft): string[] {
  return Object.values(draft.cells).flatMap(cell => cell?.error ? [cell.error] : []);
}

export function profileRuleCreationRequest(draft: ProfileRuleCreationDraft): ProjectPlotProfileCreation {
  if (profileCreationErrors(draft).length) throw new Error('Correct or Undo every raw creation error before allocating a rule identity.');
  return { originalRules: draft.review.rules,
    values: profileEditableFields.map(name => ({ column: name, value: profileCreationValue(draft, name).value })) };
}

export function profileRuleDeletionRequest(review: ValidatedProjectPlotProfileReview, rowId: string, confirmed: boolean): ProjectPlotProfileDeletion {
  if (!confirmed || review.rules.rows.filter(row => row.rowId === rowId).length !== 1) {
    throw new Error('Explicitly confirm one reviewed physical rule; drafts and unrelated rows are not deleted.');
  }
  return { originalRules: review.rules, rowId, confirmed: true };
}

export function validateCreatedProfileRule(created: ProjectMetadataRow, proposal: ProfileRuleCreationDraft): void {
  const cells = created.cells;
  if (!/^[1-9]\d*$/.test(created.rowId) || created.rowId.length > 19 || BigInt(created.rowId) > 9223372036854775807n ||
      proposal.review.rules.rows.some(row => row.rowId === created.rowId) ||
      !Array.isArray(cells) || cells.length !== 9) throw new Error('Created profile response has an inconsistent new physical identity/schema.');
  cells.forEach(profileCellLabel);
  for (const [index, column] of proposal.review.rules.columns.entries()) {
    const name = profileEditableFields.find(field => field === column.name);
    const expected = name ? profileCreationValue(proposal, name).value : profileNullCell;
    if (!equalCell(cells[index], expected)) throw new Error('Created profile storage differs from the explicit nullable proposal.');
  }
}

export function validateProfileLifecycleReview(original: ValidatedProjectPlotProfileReview, next: ValidatedProjectPlotProfileReview,
    change: { created: ProjectMetadataRow } | { deleted: string }): void {
  if (next.project !== original.project || next.table !== original.table ||
      JSON.stringify(next.rules.columns) !== JSON.stringify(original.rules.columns)) {
    throw new Error('Refreshed profile ownership/schema differs from the reviewed lifecycle proposal.');
  }
  const expected = 'created' in change ? [...original.rules.rows, change.created] :
    original.rules.rows.filter(row => row.rowId !== change.deleted);
  if (next.rules.rows.length !== expected.length || expected.some(row => {
    const observed = next.rules.rows.find(item => item.rowId === row.rowId);
    return !observed || !Array.isArray(row.cells) || observed.cells.length !== row.cells.length ||
      row.cells.some((cell, index) => !equalCell(cell, observed.cells[index]));
  })) throw new Error('Refreshed rules/counts differ from the single reviewed creation/deletion.');
}
