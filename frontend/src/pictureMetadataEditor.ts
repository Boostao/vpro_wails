import type { ProjectMetadataChange, ProjectMetadataEditorField } from '../bindings/github.com/boostao/vpro-wails';
import sourcePolicy from '../../resources/picture-metadata-policy.json';
import { equalCell, metadataCellText, parseMetadataCell, type MetadataDraftCell } from './projectMetadataEditor';
import { pictureMetadataFromWire, type PictureOwner, type PictureReview } from './pictureRead';

export const pictureMetadataColumns = ['PicDir', 'PicName'] as const;
export type PictureMetadataColumn = typeof pictureMetadataColumns[number];
export interface PictureMetadataDraft extends PictureOwner {
  columns: PictureReview['records']['columns'];
  original: PictureReview['records']['rows'][number];
  id: number;
  cells: Partial<Record<PictureMetadataColumn, MetadataDraftCell>>;
}

const policies = pictureMetadataColumns.map(name => {
  const matches = sourcePolicy.fields.filter(field => field.name === name);
  const field = matches[0];
  if (matches.length !== 1 || !field || field.daoType !== 10 || field.size !== 255 ||
      field.required || field.validationRule !== '' || field.validationText !== '') {
    throw new Error(`The packaged ${name} picture source policy is incomplete or differs from the reviewed definition.`);
  }
  const editor: ProjectMetadataEditorField = { name, kind: 'text', maximum: field.size, collection: false,
    referenceList: '', referenceColumn: '', limitToList: false, options: null };
  return { name, allowEmpty: field.allowZeroLength, editor, index: name === 'PicDir' ? 1 : 2 };
});

function policy(column: PictureMetadataColumn) {
  const result = policies.find(field => field.name === column);
  if (!result) throw new Error('Only existing picture Directory and File Name are editable; identity/comments are not implicit changes.');
  return result;
}

export function beginPictureMetadataDraft(review: PictureReview, rowId: string, owner: PictureOwner): PictureMetadataDraft {
  if (!owner.contextId || !owner.project || !owner.plotNumber) {
    throw new Error('Picture metadata editing requires an explicit active context, project and literal parent.');
  }
  const received = pictureMetadataFromWire({ ...review, records: {
    columns: review.records.columns.map(column => ({ ...column })),
    rows: review.records.rows.map(row => ({ rowId: row.rowId, cells: row.cells.map(cell => ({ ...cell })) })),
  } }, owner);
  const matches = received.records.rows.filter(row => row.rowId === rowId);
  const original = matches[0];
  const identity = original?.cells[0];
  const id = identity?.storage === 'integer' && identity.integer !== null ? Number(identity.integer) : NaN;
  if (matches.length !== 1 || !original || !identity || !Number.isInteger(id) || id < -2147483648 || id > 2147483647 ||
      String(id) !== identity?.integer || received.records.rows.filter(row => equalCell(row.cells[0], identity)).length !== 1) {
    throw new Error('Select one reviewed physical picture row with an unambiguous signed Long ID; no first row or identity is inferred.');
  }
  return { ...owner, columns: received.records.columns, original, id, cells: {} };
}

export function pictureMetadataFieldValue(draft: PictureMetadataDraft, column: PictureMetadataColumn): MetadataDraftCell {
  const field = policy(column);
  const original = draft.original.cells?.[field.index];
  if (!original) throw new Error(`Original ${column} picture cell is missing; reload before editing.`);
  return draft.cells[column] ?? { raw: metadataCellText(original), nullValue: original.storage === 'null',
    value: original, error: null };
}

function parsed(draft: PictureMetadataDraft, column: PictureMetadataColumn, raw: string, nullValue: boolean): MetadataDraftCell {
  if (typeof raw !== 'string' || typeof nullValue !== 'boolean') {
    throw new Error('Picture metadata draft input requires exact text and an explicit NULL choice.');
  }
  const field = policy(column);
  const original = draft.original.cells?.[field.index];
  if (!original) throw new Error(`Original ${column} picture cell is missing; reload before editing.`);
  const candidate = parseMetadataCell(field.editor, raw, nullValue, original);
  if (!candidate.error && !equalCell(candidate.value, original)) {
    if (!nullValue && raw.includes('\0')) candidate.error = `${column} contains NUL; raw input was not repaired.`;
    else if (!field.allowEmpty && candidate.value.storage === 'text' && candidate.value.text === '') {
      candidate.error = `${column} permits NULL but not new empty text; choose NULL explicitly or enter a file name.`;
    }
  }
  return candidate;
}

export function stagePictureMetadata(draft: PictureMetadataDraft, column: PictureMetadataColumn, raw: string, nullValue: boolean): PictureMetadataDraft {
  const cell = parsed(draft, column, raw, nullValue);
  return { ...draft, cells: { ...draft.cells, [column]: cell } };
}

export function pictureMetadataDraftErrors(draft: PictureMetadataDraft): string[] {
  return policies.flatMap(({ name }) => {
    const cell = draft.cells[name];
    const error = cell && parsed(draft, name, cell.raw, cell.nullValue).error;
    return error ? [error] : [];
  });
}

export function pictureMetadataChanges(draft: PictureMetadataDraft): ProjectMetadataChange[] {
  const changes: ProjectMetadataChange[] = [];
  for (const field of policies) {
    const staged = draft.cells[field.name];
    if (!staged) continue;
    const cell = parsed(draft, field.name, staged.raw, staged.nullValue);
    if (cell.error) throw new Error(cell.error);
    const original = draft.original.cells?.[field.index];
    if (!original) throw new Error(`Original ${field.name} picture cell is missing; reload before editing.`);
    if (!equalCell(original, cell.value)) changes.push({ column: field.name, value: cell.value });
  }
  return changes;
}

export function pictureMetadataRequest(draft: PictureMetadataDraft, requestId: string, owner: PictureOwner) {
  if (draft.contextId !== owner.contextId || draft.project !== owner.project || draft.plotNumber !== owner.plotNumber ||
      !/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(requestId)) {
    throw new Error('Picture Save requires the exact active owner and one stable request identity; no context or retry was inferred.');
  }
  const original = beginPictureMetadataDraft({ ...owner, records: {
    columns: draft.columns, rows: [draft.original],
  } }, draft.original.rowId, owner);
  if (original.id !== draft.id) throw new Error('Picture application identity changed inside the draft; reload explicitly.');
  return { requestId, id: original.id, columns: original.columns, original: original.original,
    changes: pictureMetadataChanges(draft) };
}
