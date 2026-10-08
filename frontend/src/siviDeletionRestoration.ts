import type { ProjectMetadataCell, ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { siviDeletionOriginalFromWire, siviDeletionRequest } from './siviDeletionEditor';
import { siviDeletionReceiptFromWire, type SIVIDeletionReceipt } from './siviDeletionReceipt';
import { validSIVIRequestId } from './siviRequestId';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface SIVIDeletionRestorationReview extends SIVISpeciesOwner {
  historyId: string;
  expected: string;
  form: SIVIDeletionReceipt['form'];
  rowId: string;
  id: number;
  columns: ProjectMetadataColumn[];
  original: Table['rows'][number];
  deletion: SIVIDeletionReceipt;
  auditColumns: ProjectMetadataColumn[];
  auditsBefore: Table['rows'];
}
export interface SIVIDeletionRestorationRequest extends SIVISpeciesOwner {
  requestId: string;
  historyId: string;
  expected: string;
  action: 'retain' | 'prune';
}
export interface SIVIDeletionRestorationReceipt extends SIVIDeletionRestorationReview {
  requestId: string;
  restorationId: string;
  action: 'retain' | 'prune';
  actor: string;
  auditStrength: number;
  editWhen: string;
  restored: Table['rows'][number];
  request: SIVIDeletionRestorationRequest;
  auditsAfter: Table['rows'];
  cancelled: false;
  restoredRows: 1;
  prunedAuditRows: number;
  didCommit: boolean;
  replayed: boolean;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const only = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).every(key => keys.includes(key));
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value.length > 0 && wellFormedUTF16(value) && !value.includes('\0');
const reviewKeys = ['contextId', 'project', 'plot', 'historyId', 'expected', 'form', 'rowId', 'id',
  'columns', 'original', 'deletion', 'auditColumns', 'auditsBefore'];
const requestKeys: (keyof SIVIDeletionRestorationRequest)[] =
  ['requestId', 'contextId', 'project', 'plot', 'historyId', 'expected', 'action'];
const receiptKeys = [...reviewKeys, 'requestId', 'restorationId', 'action', 'actor', 'auditStrength',
  'editWhen', 'restored', 'request', 'auditsAfter', 'cancelled', 'restoredRows', 'prunedAuditRows', 'didCommit', 'replayed'];
const auditNames = ['Project', 'User', 'PlotNumber', 'Table', 'EditField', 'EditWhen',
  'BeforeEdit', 'AfterEdit', 'Restore', 'Flag', 'ID'];
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
const textCell = (text: string | null): ProjectMetadataCell => text === null ? nullCell() : { ...nullCell(), storage: 'text', text };
const integerCell = (integer: string): ProjectMetadataCell => ({ ...nullCell(), storage: 'integer', integer });
const alias = (cell: ProjectMetadataCell, project: string) => cell.storage === 'text' &&
  cell.text !== null && ['_veg', `${project}_veg`.toLowerCase()].includes(cell.text.toLowerCase());

function sameColumns(left: ProjectMetadataColumn[], right: ProjectMetadataColumn[]): boolean {
  return left.length === right.length && left.every((column, index) =>
    column.name === right[index].name && column.declaredType === right[index].declaredType);
}
function sameRow(left: Table['rows'][number], right: Table['rows'][number]): boolean {
  return left.rowId === right.rowId && left.cells.length === right.cells.length &&
    left.cells.every((cell, index) => equalCell(cell, right.cells[index]));
}
function sameRequest(value: unknown, request: SIVIDeletionRestorationRequest): boolean {
  return record(value) && only(value, requestKeys) && requestKeys.every(key => value[key] === request[key]);
}
function auditTable(columns: unknown, rows: unknown): Table {
  if (!Array.isArray(columns) || !Array.isArray(rows) ||
    columns.some(column => !record(column) || !only(column, ['name', 'declaredType']) ||
      typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) ||
    rows.some(row => !record(row) || !only(row, ['rowId', 'cells']) || !Array.isArray(row.cells) ||
      row.cells.some(cell => !record(cell) || !only(cell, ['storage', 'text', 'integer', 'real', 'blobHex'])))) {
    throw new Error('Restoration requires complete typed physical audit evidence.');
  }
  const table = siviMetadataTableFromWire({ columns, rows });
  if (table.columns.length !== auditNames.length || new Set(table.columns.map(column => column.name)).size !== auditNames.length ||
    table.columns.some(column => !auditNames.includes(column.name))) throw new Error('Restoration audit schema differs from the complete source table.');
  return table;
}
export { sameColumns as sameSIVIHistoricalColumns, sameRow as sameSIVIHistoricalRow,
  auditTable as siviHistoricalAuditTable };
function validateAuditBefore(table: Table, deletion: SIVIDeletionReceipt) {
  if (table.rows.length !== deletion.audits.length) throw new Error('Restoration audit evidence is incomplete.');
  const seen = new Set<string>();
  for (const row of table.rows) {
    const audit = deletion.audits.find(audit => audit.rowId === row.rowId);
    if (!audit || seen.has(row.rowId)) throw new Error('Restoration audit evidence belongs to another physical event.');
    seen.add(row.rowId);
    const expected: Record<string, ProjectMetadataCell> = {
      Project: textCell(audit.project), User: textCell(audit.user), PlotNumber: textCell(audit.plotNumber),
      EditField: textCell(audit.editField), EditWhen: textCell(audit.editWhen),
      BeforeEdit: textCell(audit.beforeEdit), AfterEdit: nullCell(),
      Restore: integerCell('0'), Flag: integerCell('0'), ID: integerCell(String(audit.id)),
    };
    if (table.columns.some((column, index) => column.name === 'Table'
      ? !alias(row.cells[index], deletion.project) : !equalCell(row.cells[index], expected[column.name]))) {
      throw new Error('Restoration audit originals differ from the exact deletion authority.');
    }
  }
}

export function siviDeletionRestorationReviewFromWire(wire: unknown, owner: SIVISpeciesOwner,
  historyId: string): SIVIDeletionRestorationReview {
  const diagnostic = 'Restoration review must bind the exact deleted physical row, full44 history and SHA256 expected token.';
  if (!record(wire) || !only(wire, reviewKeys) || !validSIVIRequestId(historyId) ||
    wire.historyId !== historyId || wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
    typeof wire.expected !== 'string' || !/^[0-9a-f]{64}$/.test(wire.expected) || typeof wire.rowId !== 'string' ||
    !record(wire.deletion) || !record(wire.deletion.request) || !literal(wire.deletion.request.contextId) ||
    wire.deletion.request.requestId !== historyId) throw new Error(diagnostic);
  const durableOwner = { ...owner, contextId: wire.deletion.request.contextId };
  const form = wire.deletion.request.form;
  if (form !== 'SubVegA-SIVI' && form !== 'SubVegA-SIVI_BC' && form !== 'SubVegC-SIVI' && form !== 'SubVegD-SIVI') throw new Error(diagnostic);
  const original = siviDeletionOriginalFromWire(wire.deletion.request, durableOwner, form, wire.rowId);
  const plan = siviDeletionRequest(original, durableOwner, historyId, true);
  const deletion = siviDeletionReceiptFromWire(wire.deletion, plan, owner, 'lookup');
  if (wire.form !== deletion.form || wire.rowId !== deletion.rowId || wire.id !== deletion.id ||
    !record(wire.original) || !only(wire.original, ['rowId', 'cells'])) throw new Error(diagnostic);
  const table = siviMetadataTableFromWire({ columns: wire.columns, rows: [wire.original] });
  if (!sameColumns(table.columns, deletion.columns) || !sameRow(table.rows[0], deletion.original)) throw new Error(diagnostic);
  const audits = auditTable(wire.auditColumns, wire.auditsBefore);
  validateAuditBefore(audits, deletion);
  return { ...owner, historyId, expected: wire.expected, form: deletion.form, rowId: deletion.rowId,
    id: deletion.id, columns: deletion.columns, original: deletion.original, deletion,
    auditColumns: audits.columns, auditsBefore: audits.rows };
}

export function siviDeletionRestorationRequest(review: SIVIDeletionRestorationReview, owner: SIVISpeciesOwner,
  requestId: string, action: 'retain' | 'prune', confirmed: boolean): SIVIDeletionRestorationRequest {
  if (!validSIVIRequestId(requestId) || requestId === review.historyId) throw new Error('Restoration requires a separate stable UUIDv4; no retry identity was inferred.');
  siviDeletionRestorationReviewFromWire(review, owner, review.historyId);
  if (action !== 'retain' && action !== 'prune' || confirmed !== true) throw new Error('Explicitly confirm retain or prune for this reviewed deleted row.');
  return { ...owner, requestId, historyId: review.historyId, expected: review.expected, action };
}

export function siviDeletionRestorationReceiptFromWire(wire: unknown, review: SIVIDeletionRestorationReview,
  request: SIVIDeletionRestorationRequest, owner: SIVISpeciesOwner,
  operation: 'restore' | 'lookup' = 'restore'): SIVIDeletionRestorationReceipt {
  const diagnostic = 'Restoration receipt is incomplete or differs from the retained review/request; resolve read-only without repeating Restore.';
  siviDeletionRestorationRequest(review, { ...owner, contextId: request.contextId }, request.requestId, request.action, true);
  if (!record(wire) || !only(wire, receiptKeys) || wire.requestId !== request.requestId ||
    wire.restorationId !== request.requestId || wire.expected !== request.expected ||
    wire.historyId !== request.historyId || wire.contextId !== owner.contextId ||
    wire.project !== request.project || wire.plot !== request.plot || !literal(wire.actor) || wire.actor.length > 100 ||
    wire.action !== request.action || wire.cancelled !== false || wire.restoredRows !== 1 ||
    wire.prunedAuditRows !== (request.action === 'prune' ? review.auditsBefore.length : 0) ||
    !Number.isInteger(wire.auditStrength) || Number(wire.auditStrength) < 0 || Number(wire.auditStrength) > 3 ||
    typeof wire.editWhen !== 'string' || wire.editWhen.length !== 19 ||
    !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(wire.editWhen) || siviDateTimestampError(wire.editWhen) !== null ||
    !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
    operation === 'lookup' && wire.didCommit !== false || !sameRequest(wire.request, request)) throw new Error(diagnostic);
  const embedded = Object.fromEntries(reviewKeys.map(key => [key, wire[key]]));
  const observed = siviDeletionRestorationReviewFromWire(embedded, owner, request.historyId);
  if (!sameColumns(observed.columns, review.columns) || !sameRow(observed.original, review.original) ||
    observed.expected !== review.expected || !sameColumns(observed.auditColumns, review.auditColumns)) throw new Error(diagnostic);
  const restored = siviMetadataTableFromWire({ columns: observed.columns, rows: [wire.restored] }).rows[0];
  if (!record(wire.restored) || !only(wire.restored, ['rowId', 'cells']) || !sameRow(restored, observed.original)) throw new Error(diagnostic);
  const after = auditTable(wire.auditColumns, wire.auditsAfter);
  if (request.action === 'prune') {
    if (after.rows.length !== 0) throw new Error(diagnostic);
  } else {
    const restoreIndex = observed.auditColumns.findIndex(column => column.name === 'Restore');
    if (after.rows.length !== observed.auditsBefore.length || observed.auditsBefore.some(before => {
      const actual = after.rows.find(row => row.rowId === before.rowId);
      return !actual || actual.cells.some((cell, index) => !equalCell(cell,
        index === restoreIndex ? integerCell('-1') : before.cells[index]));
    })) throw new Error(diagnostic);
  }
  return { ...observed, requestId: request.requestId, restorationId: request.requestId, action: request.action,
    actor: wire.actor, auditStrength: Number(wire.auditStrength), editWhen: wire.editWhen, restored,
    request: structuredClone(request), auditsAfter: after.rows, cancelled: false, restoredRows: 1,
    prunedAuditRows: Number(wire.prunedAuditRows), didCommit: wire.didCommit, replayed: wire.replayed };
}
