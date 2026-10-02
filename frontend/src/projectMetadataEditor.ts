import { wellFormedUTF16 } from './qualityEditor';
import standardDefaults from '../../resources/project-metadata-standard.json';
import type { ProjectMetadataCell, ProjectMetadataCreate, ProjectMetadataEdit, ProjectMetadataEditorField, ProjectMetadataReview, ProjectMetadataRow } from '../bindings/github.com/boostao/vpro-wails';

export interface MetadataDraftCell {
  raw: string;
  nullValue: boolean;
  value: ProjectMetadataCell;
  error: string | null;
}
export interface MetadataDraft {
  review: ProjectMetadataReview;
  original: ProjectMetadataRow;
  id: number;
  cells: Record<string, MetadataDraftCell>;
  standardPopulation: '' | 'keep' | 'populate';
}

export function metadataBlankRequest(review: ProjectMetadataReview): ProjectMetadataCreate {
  const table = review.projectRecords;
  if (!review.plotNumber || !review.projectId || !wellFormedUTF16(review.projectId) || review.projectId.length > 20) {
    throw new Error('Blank creation requires the selected plot\'s existing literal ProjectID within20 UTF-16 units; no parent identity is inferred.');
  }
  if (!table.columns || table.columns.length !== 75 ||
      new Set(table.columns.map(column => column.name)).size !== 75 ||
      !table.columns.some(column => column.name === 'ID') || !table.columns.some(column => column.name === 'ProjectID') ||
      !table.rows || table.rows.length !== 0) {
    throw new Error('Review a complete empty metadata schema before blank creation; existing candidates must be selected instead.');
  }
  return { plotNumber: review.plotNumber, projectId: review.projectId, original: table };
}

export function metadataCellText(cell: ProjectMetadataCell): string {
  return cell.storage === 'text' ? cell.text ?? '' : cell.storage === 'integer' ? cell.integer ?? ''
    : cell.storage === 'real' ? String(cell.real) : cell.storage === 'blob' ? `hex:${cell.blobHex}` : '';
}
function equalCell(left: ProjectMetadataCell, right: ProjectMetadataCell): boolean {
  return left.storage === right.storage && left.text === right.text && left.integer === right.integer &&
    left.real === right.real && left.blobHex === right.blobHex;
}
function sourceCell(draft: MetadataDraft, name: string): ProjectMetadataCell {
  const columns = draft.review.projectRecords.columns;
  const indices = columns?.flatMap((column, index) => column.name === name ? [index] : []) ?? [];
  if (indices.length !== 1 || !draft.original.cells || !draft.original.cells[indices[0]]) {
    throw new Error(`Metadata source ${name} is missing or ambiguous; reload before editing.`);
  }
  return draft.original.cells[indices[0]];
}

export function beginMetadataDraft(review: ProjectMetadataReview, rowId: string): MetadataDraft {
  const table = review.projectRecords;
  if (!review.project || !review.plotNumber || review.projectId === null || review.projectId === '' ||
      !wellFormedUTF16(review.projectId) || !table.columns || !table.rows ||
      new Set(table.columns.map(column => column.name)).size !== table.columns.length) {
    throw new Error('Metadata editing requires the literal parent ProjectID and complete unambiguous schema.');
  }
  const rows = table.rows.filter(row => row.rowId === rowId);
  const row = rows[0];
  if (rows.length !== 1 || !row.cells || row.cells.length !== table.columns.length || !/^-?(0|[1-9]\d*)$/.test(rowId)) {
    throw new Error('Select one explicit physical metadata record; first-row selection is not supported.');
  }
  const idIndex = table.columns.findIndex(column => column.name === 'ID');
  const projectIndex = table.columns.findIndex(column => column.name === 'ProjectID');
  const identity = row.cells[idIndex];
  const owner = row.cells[projectIndex];
  const id = identity?.storage === 'integer' && identity.integer !== null ? Number(identity.integer) : NaN;
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647 || String(id) !== identity.integer ||
      owner?.storage !== 'text' || owner.text !== review.projectId) {
    throw new Error('Metadata physical ID/ProjectID is not a literal supported existing-record identity.');
  }
  return { review, original: row, id, cells: {}, standardPopulation: '' };
}

export function metadataFieldValue(draft: MetadataDraft, field: ProjectMetadataEditorField): MetadataDraftCell {
  return draft.cells[field.name] ?? (() => {
    const value = sourceCell(draft, field.name);
    return { raw: metadataCellText(value), nullValue: value.storage === 'null', value, error: null };
  })();
}

export function stageMetadata(draft: MetadataDraft, field: ProjectMetadataEditorField, raw: string, nullValue: boolean): MetadataDraft {
  const original = sourceCell(draft, field.name);
  let value: ProjectMetadataCell = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  let error: string | null = null;
  if (!nullValue) {
    if (!wellFormedUTF16(raw)) error = `${field.name} contains incomplete Unicode; raw input was not repaired.`;
    else if (raw === metadataCellText(original) && original.storage !== 'null') value = original;
    else if (field.kind === 'text') {
      value = { ...value, storage: 'text', text: raw };
      if (field.maximum > 0 && raw.length > field.maximum) error = `${field.name} exceeds ${field.maximum} UTF-16 units; raw input was not truncated.`;
      else if (field.limitToList && !(field.options ?? []).some(option => option.value === raw)) {
        error = `${field.name} requires a literal registered ${field.referenceColumn} from ${field.referenceList}.`;
      }
    } else if (!/^-?(0|[1-9]\d*)$/.test(raw) || raw === '-0') error = `${field.name} requires an exact integer; raw input was not rounded or completed.`;
    else {
      const number = Number(raw);
      const maximum = field.maximum === 16 ? 32767 : 2147483647;
      if (!Number.isInteger(number) || number < -maximum - 1 || number > maximum) error = `${field.name} exceeds the signed${field.maximum} integer domain.`;
      else if (field.collection && ![1, 2, 3].includes(number)) error = `${field.name} requires 1 complete, 2 partial or 3 none—not a BOOLEAN.`;
      else value = { ...value, storage: 'integer', integer: raw };
    }
  }
  const next = { ...draft, cells: { ...draft.cells, [field.name]: { raw, nullValue, value, error } } };
  if (field.name === 'EcosysCollectionStandard') next.standardPopulation = '';
  return next;
}

export function metadataErrors(draft: MetadataDraft): string[] {
  return Object.values(draft.cells).flatMap(cell => cell.error ? [cell.error] : []);
}
export function metadataDirty(draft: MetadataDraft): boolean {
  return Object.entries(draft.cells).some(([name, cell]) => cell.error !== null || !equalCell(cell.value, sourceCell(draft, name)));
}
export function metadataStandardDecisionRequired(draft: MetadataDraft): boolean {
  const cell = draft.cells.EcosysCollectionStandard;
  if (!cell || cell.error || cell.value.text === null || equalCell(cell.value, sourceCell(draft, 'EcosysCollectionStandard'))) return false;
  return /^(DEIF|DTE)/i.test(cell.value.text) || /^LMH25$/i.test(cell.value.text);
}
export const metadataStandardDefaults: Readonly<Record<string, string>> = Object.freeze(standardDefaults);

export function populateMetadataStandard(draft: MetadataDraft, fields: ProjectMetadataEditorField[]): MetadataDraft {
  if (!metadataStandardDecisionRequired(draft)) throw new Error('Source population requires a changed recognized collection standard.');
  let next = draft;
  for (const [name, value] of Object.entries(metadataStandardDefaults)) {
    const matches = fields.filter(field => field.name === name && field.kind === 'text');
    if (matches.length !== 1) throw new Error(`Source population field ${name} is unavailable or ambiguous; drafts were retained.`);
    next = stageMetadata(next, matches[0], value, false);
    if (next.cells[name].error) throw new Error(`Source population was not applied; drafts were retained: ${next.cells[name].error}`);
  }
  return { ...next, standardPopulation: 'populate' };
}
export function metadataEditRequest(draft: MetadataDraft): ProjectMetadataEdit {
  const errors = metadataErrors(draft);
  if (errors.length) throw new Error(errors[0]);
  if (metadataStandardDecisionRequired(draft) && draft.standardPopulation !== 'keep' && draft.standardPopulation !== 'populate') {
    throw new Error('Explicitly keep current drafts or confirm source-default population for this collection standard.');
  }
  const changes = Object.entries(draft.cells).flatMap(([column, cell]) =>
    equalCell(cell.value, sourceCell(draft, column)) ? [] : [{ column, value: cell.value }]);
  if (!changes.length) throw new Error('No changed metadata assignment is ready; unchanged history and stamps are not rewritten.');
  return { plotNumber: draft.review.plotNumber, projectId: draft.review.projectId, id: draft.id,
    columns: draft.review.projectRecords.columns, original: draft.original, changes, standardPopulation: draft.standardPopulation };
}
