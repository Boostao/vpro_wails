import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { completeMetadataCellFromWire, siviMetadataTableFromWire } from './siviParentTransport';
import { siviCreationReceiptFromWire, type SIVICreationReceipt } from './siviCreationReceipt';
import { siviCoverGroups } from './siviCoverEditor';
import { siviSourceAuditText } from './siviDeletionReceipt';
import { sameSIVIHistoricalColumns, sameSIVIHistoricalRow, siviHistoricalAuditTable } from './siviDeletionRestoration';
import { validSIVIRequestId } from './siviRequestId';
import type { SIVICreationRequest } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface SIVICreationUndoReview extends SIVISpeciesOwner {
  historyId: string;
  expected: string;
  form: SIVICreationReceipt['form'];
  rowId: string;
  id: number;
  columns: Table['columns'];
  committed: Table['rows'][number];
  creation: SIVICreationReceipt;
  auditColumns: Table['columns'];
  auditsBefore: Table['rows'];
}
export interface SIVICreationUndoRequest extends SIVISpeciesOwner {
  requestId: string;
  historyId: string;
  expected: string;
  action: 'retain' | 'prune';
}
export interface SIVICreationUndoReceipt extends SIVICreationUndoReview {
  requestId: string;
  undoId: string;
  action: 'retain' | 'prune';
  actor: string;
  auditStrength: number;
  editWhen: string;
  request: SIVICreationUndoRequest;
  auditsAfter: Table['rows'];
  cancelled: false;
  removedRows: 1;
  prunedAuditRows: number;
  didCommit: boolean;
  replayed: boolean;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const only = (value: Record<string, unknown>, keys: readonly string[]) => Object.keys(value).every(key => keys.includes(key));
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value.length > 0 && wellFormedUTF16(value) && !value.includes('\0');
const reviewKeys = ['contextId', 'project', 'plot', 'historyId', 'expected', 'form', 'rowId', 'id',
  'columns', 'committed', 'creation', 'auditColumns', 'auditsBefore'];
const requestKeys = ['requestId', 'contextId', 'project', 'plot', 'historyId', 'expected', 'action'] as const;
const receiptKeys = [...reviewKeys, 'requestId', 'undoId', 'action', 'actor', 'auditStrength', 'editWhen',
  'request', 'auditsAfter', 'cancelled', 'removedRows', 'prunedAuditRows', 'didCommit', 'replayed'];
const creationKeys = ['requestId', 'contextId', 'project', 'plot', 'form', 'rowId', 'historyId', 'id',
  'actor', 'auditStrength', 'editWhen', 'columns', 'original', 'committed', 'covers', 'request', 'didCommit', 'replayed'];
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
const textCell = (text: string): ProjectMetadataCell => ({ ...nullCell(), storage: 'text', text });
const integerCell = (integer: string): ProjectMetadataCell => ({ ...nullCell(), storage: 'integer', integer });

function rowShape(value: unknown): boolean {
  return record(value) && only(value, ['rowId', 'cells']) && Array.isArray(value.cells) &&
    value.cells.every(cell => record(cell) && only(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) &&
      completeMetadataCellFromWire(cell));
}
function columnsShape(value: unknown): boolean {
  return Array.isArray(value) && value.every(column => record(column) && only(column, ['name', 'declaredType']));
}

function historicalRequest(value: unknown, owner: SIVISpeciesOwner, historyId: string): SIVICreationRequest {
  const diagnostic = 'Creation Undo requires complete immutable creation request evidence, not current reference inference.';
  if (!record(value) || !only(value, ['requestId', 'contextId', 'project', 'plot', 'form', 'species', 'covers', 'decision']) ||
    value.requestId !== historyId || !literal(value.contextId) || value.project !== owner.project || value.plot !== owner.plot ||
    !literal(value.species) || value.species.length > 8 || !Array.isArray(value.covers) ||
    value.form !== 'SubVegA-SIVI' && value.form !== 'SubVegA-SIVI_BC' && value.form !== 'SubVegC-SIVI' && value.form !== 'SubVegD-SIVI') {
    throw new Error(diagnostic);
  }
  const group = value.form === 'SubVegC-SIVI' ? 1 : value.form === 'SubVegD-SIVI' ? 2 : 0;
  const permitted = siviCoverGroups(group, value.form === 'SubVegA-SIVI').flat();
  const covers: SIVICreationRequest['covers'] = [], seen = new Set<string>();
  for (const cover of value.covers) {
    if (!record(cover) || !only(cover, ['column', 'value'])) throw new Error(diagnostic);
    const column = permitted.find(column => column === cover.column);
    if (!column || seen.has(column) || !record(cover.value) ||
      !only(cover.value, ['storage', 'text', 'integer', 'real', 'blobHex']) || !completeMetadataCellFromWire(cover.value) ||
      cover.value.storage !== 'real' || cover.value.real === null || !Number.isFinite(cover.value.real) ||
      cover.value.real >= 100) throw new Error(diagnostic);
    seen.add(column);
    covers.push({ column, value: { ...cover.value } });
  }
  let decision: SIVICreationRequest['decision'];
  if (value.decision !== undefined) {
    const choice = value.decision;
    if (!record(choice) || !only(choice, ['kind', 'entered', 'selected']) ||
      choice.kind !== 'replace' && choice.kind !== 'keep' && choice.kind !== 'user' ||
      !literal(choice.entered) || choice.entered.length > 8 ||
      choice.selected !== undefined && (!literal(choice.selected) || choice.selected.length > 8) ||
      choice.kind === 'keep' && choice.selected !== undefined ||
      choice.kind !== 'keep' && choice.selected === undefined) throw new Error(diagnostic);
    decision = { kind: choice.kind, entered: choice.entered,
      ...(choice.selected === undefined ? {} : { selected: choice.selected }) };
  }
  return { ...owner, contextId: value.contextId, requestId: historyId, form: value.form, species: value.species,
    covers, ...(decision === undefined ? {} : { decision }) };
}

function sameRequest(value: unknown, request: SIVICreationUndoRequest): boolean {
  return record(value) && only(value, requestKeys) && requestKeys.every(key => value[key] === request[key]);
}

function sameCreation(left: SIVICreationReceipt, right: SIVICreationReceipt): boolean {
  const scalars = ['requestId', 'contextId', 'project', 'plot', 'form', 'rowId', 'historyId', 'id',
    'actor', 'auditStrength', 'editWhen', 'didCommit', 'replayed'] as const;
  const requestScalars = ['requestId', 'contextId', 'project', 'plot', 'form', 'species'] as const;
  return scalars.every(key => left[key] === right[key]) &&
    requestScalars.every(key => left.request[key] === right.request[key]) &&
    left.request.decision?.kind === right.request.decision?.kind &&
    left.request.decision?.entered === right.request.decision?.entered &&
    left.request.decision?.selected === right.request.decision?.selected &&
    sameSIVIHistoricalColumns(left.columns, right.columns) &&
    sameSIVIHistoricalRow(left.original, right.original) &&
    sameSIVIHistoricalRow(left.committed, right.committed) &&
    left.covers.length === right.covers.length && left.covers.every((cover, index) =>
      cover.column === right.covers[index].column && equalCell(cover.value, right.covers[index].value));
}

function validateCreationAudits(table: Table, creation: SIVICreationReceipt) {
  const expected = new Map<string, string>();
  if (creation.auditStrength >= 2) {
    expected.set('Species', creation.request.species);
    for (const cover of creation.covers) {
      const column = creation.columns.find(column => column.name === cover.column);
      if (!column) throw new Error('Creation audit field lacks reviewed physical schema.');
      const value = siviSourceAuditText(column, cover.value);
      if (value === null) throw new Error('Creation audit field lacks its exact added value.');
      expected.set(cover.column, value);
    }
  }
  for (const row of table.rows) {
    const field = row.cells[table.columns.findIndex(column => column.name === 'EditField')];
    if (field.storage !== 'text' || field.text === null || !expected.has(field.text)) {
      throw new Error('Creation Undo source audits contain missing, duplicate or foreign creation fields.');
    }
    const values: Record<string, ProjectMetadataCell> = {
      Project: textCell(creation.project), User: textCell(creation.actor), PlotNumber: textCell(creation.plot),
      EditField: textCell(field.text), EditWhen: textCell(creation.editWhen), BeforeEdit: nullCell(),
      AfterEdit: textCell(expected.get(field.text)!), Restore: integerCell('0'), Flag: integerCell('0'),
      ID: integerCell(String(creation.id)),
    };
    if (table.columns.some((column, index) => column.name === 'Table'
      ? row.cells[index].storage !== 'text' || row.cells[index].text === null ||
        !['_veg', `${creation.project}_veg`.toLowerCase()].includes(row.cells[index].text.toLowerCase())
      : !equalCell(row.cells[index], values[column.name]))) throw new Error('Creation Undo source audit authority differs.');
    expected.delete(field.text);
  }
  if (expected.size) throw new Error('Creation Undo source audit authority is incomplete.');
}

export function siviCreationUndoReviewFromWire(wire: unknown, owner: SIVISpeciesOwner, historyId: string): SIVICreationUndoReview {
  const diagnostic = 'Creation Undo review must bind the exact created physical row, full44 history and SHA256 token.';
  if (!record(wire) || !only(wire, reviewKeys) || !validSIVIRequestId(historyId) || wire.historyId !== historyId ||
    wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
    typeof wire.expected !== 'string' || !/^[0-9a-f]{64}$/.test(wire.expected) ||
    !record(wire.creation) || !only(wire.creation, creationKeys) ||
    !columnsShape(wire.columns) || !columnsShape(wire.creation.columns) ||
    !rowShape(wire.committed) || !rowShape(wire.creation.original) || !rowShape(wire.creation.committed) ||
    !Array.isArray(wire.creation.covers) || wire.creation.covers.some(cover => !record(cover) ||
      !only(cover, ['column', 'value']) || !record(cover.value) ||
      !only(cover.value, ['storage', 'text', 'integer', 'real', 'blobHex']))) throw new Error(diagnostic);
  const request = historicalRequest(wire.creation.request, owner, historyId);
  const creation = siviCreationReceiptFromWire(wire.creation, request, owner, 'lookup');
  const table = siviMetadataTableFromWire({ columns: wire.columns, rows: [wire.committed] });
  if (wire.form !== creation.form || wire.rowId !== creation.rowId || wire.id !== creation.id ||
    !sameSIVIHistoricalColumns(table.columns, creation.columns) ||
    !sameSIVIHistoricalRow(table.rows[0], creation.committed)) throw new Error(diagnostic);
  const audits = siviHistoricalAuditTable(wire.auditColumns, wire.auditsBefore);
  validateCreationAudits(audits, creation);
  return { ...owner, historyId, expected: wire.expected, form: creation.form, rowId: creation.rowId, id: creation.id,
    columns: table.columns, committed: table.rows[0], creation, auditColumns: audits.columns, auditsBefore: audits.rows };
}

export function siviCreationUndoRequest(review: SIVICreationUndoReview, owner: SIVISpeciesOwner,
  requestId: string, action: 'retain' | 'prune', confirmed: boolean): SIVICreationUndoRequest {
  if (!validSIVIRequestId(requestId) || requestId === review.historyId) throw new Error('Creation Undo requires a separate stable UUIDv4.');
  siviCreationUndoReviewFromWire(review, owner, review.historyId);
  if (action !== 'retain' && action !== 'prune' || confirmed !== true) throw new Error('Explicitly confirm retain or prune for the exact created row.');
  return { ...owner, requestId, historyId: review.historyId, expected: review.expected, action };
}

export function siviCreationUndoReceiptFromWire(wire: unknown, review: SIVICreationUndoReview,
  request: SIVICreationUndoRequest, owner: SIVISpeciesOwner, operation: 'undo' | 'lookup' = 'undo'): SIVICreationUndoReceipt {
  const diagnostic = 'Creation Undo receipt differs from retained authority; resolve read-only without repeating Undo.';
  siviCreationUndoRequest(review, { ...owner, contextId: request.contextId }, request.requestId, request.action, true);
  if (!record(wire) || !only(wire, receiptKeys) || wire.requestId !== request.requestId || wire.undoId !== request.requestId ||
    wire.expected !== request.expected || wire.historyId !== request.historyId || wire.contextId !== owner.contextId ||
    wire.project !== request.project || wire.plot !== request.plot || wire.action !== request.action ||
    !literal(wire.actor) || wire.actor.length > 100 || wire.cancelled !== false || wire.removedRows !== 1 ||
    wire.prunedAuditRows !== (request.action === 'prune' ? review.auditsBefore.length : 0) ||
    !Number.isInteger(wire.auditStrength) || Number(wire.auditStrength) < 0 || Number(wire.auditStrength) > 3 ||
    typeof wire.editWhen !== 'string' || wire.editWhen.length !== 19 ||
    !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(wire.editWhen) || siviDateTimestampError(wire.editWhen) !== null ||
    !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
    operation === 'lookup' && wire.didCommit !== false || !sameRequest(wire.request, request)) throw new Error(diagnostic);
  const observed = siviCreationUndoReviewFromWire(Object.fromEntries(reviewKeys.map(key => [key, wire[key]])), owner, request.historyId);
  if (observed.expected !== review.expected || !sameCreation(observed.creation, review.creation) ||
    !sameSIVIHistoricalColumns(observed.columns, review.columns) ||
    !sameSIVIHistoricalRow(observed.committed, review.committed) ||
    !sameSIVIHistoricalColumns(observed.auditColumns, review.auditColumns) ||
    observed.auditsBefore.length !== review.auditsBefore.length ||
    review.auditsBefore.some(row => !observed.auditsBefore.some(actual => sameSIVIHistoricalRow(actual, row)))) throw new Error(diagnostic);
  const after = siviHistoricalAuditTable(wire.auditColumns, wire.auditsAfter);
  const restoreIndex = observed.auditColumns.findIndex(column => column.name === 'Restore');
  if (request.action === 'prune' ? after.rows.length !== 0 : after.rows.length !== observed.auditsBefore.length ||
    observed.auditsBefore.some(before => {
      const actual = after.rows.find(row => row.rowId === before.rowId);
      return !actual || actual.cells.some((cell, index) => !equalCell(cell,
        index === restoreIndex ? integerCell('-1') : before.cells[index]));
    })) throw new Error(diagnostic);
  return { ...observed, requestId: request.requestId, undoId: request.requestId, action: request.action,
    actor: wire.actor, auditStrength: Number(wire.auditStrength), editWhen: wire.editWhen,
    request: structuredClone(request), auditsAfter: after.rows, cancelled: false, removedRows: 1,
    prunedAuditRows: Number(wire.prunedAuditRows), didCommit: wire.didCommit, replayed: wire.replayed };
}
