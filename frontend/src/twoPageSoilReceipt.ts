import type { AuditEntry, ProjectMetadataCell, TwoPageSoilAttemptOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { siviSourceAuditText } from './siviDeletionReceipt';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { validSIVIRequestId } from './siviRequestId';
import { soilFields } from './soilChildEditor';
import { twoPageSoilOriginalFromWire, twoPageSoilEditableID, twoPageSoilKind, twoPageSoilRequest,
  twoPageSoilCreationRequest, twoPageSoilCreationDefaults,
  type TwoPageSoilOwner, type TwoPageSoilOriginal } from './twoPageSoilEditor';

export type TwoPageSoilRequest = ReturnType<typeof twoPageSoilRequest>;
export interface TwoPageSoilReceipt {
  history: {
    Request: TwoPageSoilRequest;
    Committed: TwoPageSoilOriginal['records'];
    Actor: string;
    AuditStrength: number;
    EditWhen: string;
    Audits: AuditEntry[];
    CreatedRowID?: string;
  };
  didCommit: boolean;
  replayed: boolean;
}
export type TwoPageSoilOutcome = Omit<TwoPageSoilAttemptOutcome, 'status' | 'receipt'> & {
  status: 'running' | 'unknown' | 'notCommitted' | 'committed';
  receipt: TwoPageSoilReceipt | null;
};
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const exact = (value: Record<string, unknown>, keys: readonly string[]) =>
  Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const canonicalCell = (value: unknown): value is ProjectMetadataCell =>
  record(value) && exact(value, ['storage', 'text', 'integer', 'real', 'blobHex']) && completeMetadataCellFromWire(value);

export function twoPageSoilOutcomeFromWire(wire: unknown, request: TwoPageSoilRequest,
  requestJSON: string, owner: TwoPageSoilOwner): TwoPageSoilOutcome {
  if (!record(wire) || !exact(wire, ['contextId', 'requestId', 'requestJSON', 'status', 'reason', 'receipt']) ||
      wire.contextId !== owner.contextId || wire.requestId !== request.requestId || wire.requestJSON !== requestJSON ||
      typeof wire.reason !== 'string' || !wire.reason || !wellFormedUTF16(wire.reason) || wire.reason.includes('\0') ||
      !(wire.status === 'running' || wire.status === 'unknown' || wire.status === 'notCommitted' || wire.status === 'committed') ||
      wire.status === 'notCommitted' && request.original.contextId !== owner.contextId ||
      wire.status !== 'committed' && wire.receipt !== null) throw new Error('Soil outcome does not certify the exact retained request and original owner.');
  return { contextId: owner.contextId, requestId: request.requestId, requestJSON, status: wire.status,
    reason: wire.reason, receipt: wire.status === 'committed' ? twoPageSoilReceiptFromWire(wire.receipt, request, owner, true) : null };
}

function auditText(column: string, cell: ProjectMetadataCell): string | null {
  if (cell.storage === 'blob' && cell.blobHex !== null) {
    return `[${(cell.blobHex.match(/../g) ?? []).map(byte => Number.parseInt(byte, 16)).join(' ')}]`;
  }
  return siviSourceAuditText({ name: column, declaredType: '' }, cell);
}
export function twoPageSoilReceiptFromWire(wire: unknown, request: TwoPageSoilRequest,
  owner: TwoPageSoilOwner, recovery = false): TwoPageSoilReceipt {
  if (request.horizon !== undefined) return soilCreationReceipt(wire, request, owner, recovery);
  const diagnostic = 'Soil receipt differs from its retained complete request/source plan; recover durable history without replay.';
  const original = twoPageSoilOriginalFromWire(request.original, owner);
  if (!validSIVIRequestId(request.requestId) || !Array.isArray(request.edits) || !request.edits.length ||
      !record(wire) || !exact(wire, ['history', 'didCommit', 'replayed']) ||
      !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
      recovery && wire.didCommit !== false || !record(wire.history) ||
      !exact(wire.history, ['Request', 'Committed', 'Actor', 'AuditStrength', 'EditWhen', 'Audits'])) throw new Error(diagnostic);
  const history = wire.history;
  if (!record(history.Request) || !exact(history.Request, ['requestId', 'original', 'edits']) ||
      history.Request.requestId !== request.requestId || !Array.isArray(history.Request.edits) ||
      typeof history.Actor !== 'string' || !history.Actor || history.Actor.length > 100 ||
      !wellFormedUTF16(history.Actor) || history.Actor.includes('\0') ||
      typeof history.AuditStrength !== 'number' || !Number.isInteger(history.AuditStrength) ||
      history.AuditStrength < 0 || history.AuditStrength > 3 ||
      typeof history.EditWhen !== 'string' || !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(history.EditWhen) ||
      !Array.isArray(history.Audits)) throw new Error(diagnostic);
  const time = new Date(`${history.EditWhen.replace(' ', 'T')}Z`);
  if (!Number.isFinite(time.getTime()) ||
      time.toISOString().slice(0, 19).replace('T', ' ') !== history.EditWhen) throw new Error(diagnostic);
  const returnedOriginal = twoPageSoilOriginalFromWire(history.Request.original, owner);
  if (JSON.stringify(returnedOriginal) !== JSON.stringify(original)) throw new Error(diagnostic);
  const confirmed: TwoPageSoilRequest = { requestId: request.requestId, original, edits: [] };
  const seenEdits = new Set<string>();
  for (const [index, edit] of history.Request.edits.entries()) {
    if (!record(edit) || !exact(edit, ['contextId', 'form', 'rowId', 'column', 'expected', 'value']) ||
        typeof edit.rowId !== 'string' || typeof edit.column !== 'string') throw new Error(diagnostic);
    const retained = request.edits[index];
    if (!retained || edit.contextId !== retained.contextId || edit.form !== retained.form ||
        edit.rowId !== retained.rowId || edit.column !== retained.column ||
        !canonicalCell(edit.expected) || !canonicalCell(edit.value) ||
        !canonicalCell(retained.expected) || !canonicalCell(retained.value) ||
        !equalCell(edit.expected, retained.expected) || !equalCell(edit.value, retained.value)) throw new Error(diagnostic);
    const key = JSON.stringify([retained.rowId, retained.column]);
    const row = original.records.rows.find(row => row.rowId === retained.rowId);
    const column = original.records.columns.findIndex(column => column.name === retained.column);
    if (seenEdits.has(key) || !row || column < 0 || retained.contextId !== owner.contextId || retained.form !== owner.form ||
        !soilFields[twoPageSoilKind(owner.form)].some(field => field.column === retained.column) ||
        !equalCell(row.cells[column], retained.expected)) throw new Error(diagnostic);
    twoPageSoilEditableID(original, retained.rowId);
    seenEdits.add(key);
    confirmed.edits.push({ contextId: retained.contextId, form: retained.form, rowId: retained.rowId,
      column: retained.column, expected: { ...retained.expected }, value: { ...retained.value } });
  }

  function soilCreationReceipt(wire: unknown, request: TwoPageSoilRequest,
    owner: TwoPageSoilOwner, recovery: boolean): TwoPageSoilReceipt {
    const diagnostic = 'Add Horizon receipt differs from its exact complete creation plan; retain the request and recover without replay.';
    if (typeof request.horizon !== 'string' || request.edits.length !== 0 ||
        !record(wire) || !exact(wire, ['history', 'didCommit', 'replayed']) || !record(wire.history) ||
        !exact(wire.history, ['Request', 'Committed', 'Actor', 'AuditStrength', 'EditWhen', 'Audits', 'CreatedRowID'])) {
      throw new Error(diagnostic);
    }
    const retained = twoPageSoilCreationRequest(request.original, owner, request.horizon, request.requestId);
    const history = wire.history;
    if (!record(history.Request) || !exact(history.Request, ['requestId', 'original', 'edits', 'horizon']) ||
        history.Request.requestId !== retained.requestId || history.Request.horizon !== retained.horizon ||
        !Array.isArray(history.Request.edits) || history.Request.edits.length !== 0 ||
        typeof history.CreatedRowID !== 'string' || !exactSigned64(history.CreatedRowID) ||
        JSON.stringify(twoPageSoilOriginalFromWire(history.Request.original, owner)) !== JSON.stringify(retained.original)) {
      throw new Error(diagnostic);
    }
    const committed = twoPageSoilOriginalFromWire({ ...retained.original, records: history.Committed }, owner).records;
    const created = committed.rows.find(row => row.rowId === history.CreatedRowID);
    const idIndex = retained.original.records.columns.findIndex(column => column.name === 'ID');
    const horizonIndex = retained.original.records.columns.findIndex(column => column.name === 'Horizon');
    if (!created || created.cells[idIndex].storage !== 'integer' || created.cells[idIndex].integer === null) throw new Error(diagnostic);
    const cells = twoPageSoilCreationDefaults(retained.original, request.horizon, created.cells[idIndex].integer);
    if (created.cells.some((cell, index) => !equalCell(cell, cells[index])) ||
        retained.original.records.rows.some(row => row.rowId === created.rowId)) throw new Error(diagnostic);
    const value = cells[horizonIndex];
    cells[horizonIndex] = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
    const rows = [...retained.original.records.rows, { rowId: created.rowId, cells }];
    rows.sort((a, b) => BigInt(a.rowId) < BigInt(b.rowId) ? -1 : BigInt(a.rowId) > BigInt(b.rowId) ? 1 : 0);
    const original = { ...retained.original, records: { columns: retained.original.records.columns, rows } };
    const editRequest: TwoPageSoilRequest = { requestId: retained.requestId, original, edits: [{
      contextId: owner.contextId, form: owner.form, rowId: created.rowId, column: 'Horizon',
      expected: cells[horizonIndex], value,
    }] };
    const checked = twoPageSoilReceiptFromWire({
      didCommit: wire.didCommit, replayed: wire.replayed,
      history: { Request: editRequest, Committed: committed, Actor: history.Actor,
        AuditStrength: history.AuditStrength, EditWhen: history.EditWhen, Audits: history.Audits },
    }, editRequest, owner, recovery);
    return { ...checked, history: { ...checked.history, Request: retained, CreatedRowID: created.rowId } };
  }
  if (history.Request.edits.length !== request.edits.length) throw new Error(diagnostic);
  const strength = history.AuditStrength;
  const committed = twoPageSoilOriginalFromWire({ ...original, records: history.Committed }, owner).records;
  const expected = original.records.rows.map(row => ({ rowId: row.rowId, cells: row.cells.map(cell => ({ ...cell })) }));
  for (const edit of confirmed.edits) {
    const row = expected.find(row => row.rowId === edit.rowId);
    const index = original.records.columns.findIndex(column => column.name === edit.column);
    if (!row || index < 0) throw new Error(diagnostic);
    row.cells[index] = { ...edit.value };
  }
  if (JSON.stringify(committed.columns) !== JSON.stringify(original.records.columns) ||
      committed.rows.length !== expected.length ||
      committed.rows.some((row, index) => row.rowId !== expected[index].rowId ||
        row.cells.some((cell, column) => !equalCell(cell, expected[index].cells[column])))) throw new Error(diagnostic);
  const audited = confirmed.edits.filter(edit => !equalCell(edit.expected, edit.value) &&
    (edit.expected.storage === 'null' ? strength >= 2 : edit.value.storage === 'null' ? strength === 3 : strength >= 1));
  if (history.Audits.length !== audited.length) throw new Error(diagnostic);
  const audits: AuditEntry[] = [], seen = new Set<string>();
  for (const [index, audit] of history.Audits.entries()) {
    const edit = audited[index];
    if (!record(audit) || !exact(audit, ['rowId', 'project', 'user', 'plotNumber', 'table', 'editField',
      'editWhen', 'beforeEdit', 'afterEdit', 'restore', 'flag', 'id']) ||
      typeof audit.rowId !== 'string' || !exactSigned64(audit.rowId) || seen.has(audit.rowId) ||
      audit.project !== owner.project || audit.plotNumber !== owner.plot || audit.user !== history.Actor ||
      audit.table !== (owner.form === 'SoilHumusXL' ? '_Humus' : '_Mineral') ||
      audit.editField !== edit.column || audit.editWhen !== history.EditWhen ||
      audit.id !== twoPageSoilEditableID(original, edit.rowId) || audit.restore !== false || audit.flag !== false ||
      audit.beforeEdit !== auditText(edit.column, edit.expected) || audit.afterEdit !== auditText(edit.column, edit.value)) throw new Error(diagnostic);
    seen.add(audit.rowId);
    audits.push({ rowId: audit.rowId, project: owner.project, user: history.Actor, plotNumber: owner.plot,
      table: owner.form === 'SoilHumusXL' ? '_Humus' : '_Mineral', editField: edit.column,
      editWhen: history.EditWhen, beforeEdit: auditText(edit.column, edit.expected),
      afterEdit: auditText(edit.column, edit.value), restore: false, flag: false,
      id: twoPageSoilEditableID(original, edit.rowId) });
  }
  return { history: { Request: confirmed, Committed: committed, Actor: history.Actor,
    AuditStrength: history.AuditStrength, EditWhen: history.EditWhen, Audits: audits },
    didCommit: wire.didCommit, replayed: wire.replayed };
}
