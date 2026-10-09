import type { AuditEntry, ProjectMetadataCell, TwoPageVegetationAttemptOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { equalCell } from './projectMetadataEditor';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { singleMaximum } from './numericEditor';
import { validSIVIRequestId } from './siviRequestId';
import { siviSourceAuditText } from './siviDeletionReceipt';
import { twoPageVegetationFields, twoPageVegetationEditableID } from './twoPageVegetationEditor';
import {
  equalVegetationCell, vegetationColumnIndex, vegetationSourceCell, twoPageVegetationSource,
  twoPageVegetationOriginalFromWire, serializeTwoPageVegetationRequest,
  type TwoPageVegetationOwner, type TwoPageVegetationOriginal, type TwoPageVegetationRequest,
} from './twoPageVegetationWire';

export interface TwoPageVegetationReceipt {
  history: {
    Request: TwoPageVegetationRequest;
    Committed: TwoPageVegetationOriginal['records'];
    Actor: string;
    AuditStrength: number;
    EditWhen: string;
    Audits: AuditEntry[];
  };
  didCommit: boolean;
  replayed: boolean;
}
export type TwoPageVegetationOutcome = Omit<TwoPageVegetationAttemptOutcome, 'status' | 'receipt'> & {
  status: 'running' | 'unknown' | 'notCommitted' | 'committed';
  receipt: TwoPageVegetationReceipt | null;
};
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const exact = (value: Record<string, unknown>, keys: readonly string[]) =>
  Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const cell = (value: unknown): value is ProjectMetadataCell =>
  record(value) && exact(value, ['storage', 'text', 'integer', 'real', 'blobHex']) && completeMetadataCellFromWire(value);
const diagnostic = 'Vegetation receipt differs from its exact retained request/source plan; recover durable history without replay.';

function confirmedRequest(wire: unknown, owner: TwoPageVegetationOwner): TwoPageVegetationRequest {
  if (!record(wire) || !exact(wire, ['requestId', 'original', 'edits']) || typeof wire.requestId !== 'string' ||
    !validSIVIRequestId(wire.requestId) || !Array.isArray(wire.edits) || !wire.edits.length) throw new Error(diagnostic);
  const original = twoPageVegetationOriginalFromWire(wire.original, owner);
  const request: TwoPageVegetationRequest = { requestId: wire.requestId, original, edits: [] };
  const seen = new Set<string>();
  for (const edit of wire.edits) {
    if (!record(edit) || !exact(edit, ['contextId', 'form', 'rowId', 'column', 'expected', 'value']) ||
      edit.contextId !== owner.contextId || edit.form !== owner.form ||
      typeof edit.rowId !== 'string' || typeof edit.column !== 'string' ||
      !twoPageVegetationFields(original).includes(edit.column) || !cell(edit.expected) || !cell(edit.value)) throw new Error(diagnostic);
    const key = JSON.stringify([edit.rowId, edit.column]);
    twoPageVegetationEditableID(original, edit.rowId);
    if (seen.has(key) || !equalVegetationCell(vegetationSourceCell(original, edit.rowId, edit.column), edit.expected) ||
      equalVegetationCell(edit.expected, edit.value)) throw new Error(diagnostic);
    if (edit.value.storage !== 'null') {
      if (edit.column === 'Collected') {
        if (edit.value.storage !== 'text' || edit.value.text === null || edit.value.text.length > 1) throw new Error(diagnostic);
      } else if (edit.value.storage !== 'real' || edit.value.real === null || Object.is(edit.value.real, -0) ||
        Math.abs(edit.value.real) > singleMaximum || edit.value.real >= 100) throw new Error(diagnostic);
    }
    seen.add(key);
    request.edits.push({ contextId: owner.contextId, form: owner.form, rowId: edit.rowId, column: edit.column,
      expected: { ...edit.expected }, value: { ...edit.value } });
  }
  return request;
}

function sourceRows(original: TwoPageVegetationOriginal, records: TwoPageVegetationOriginal['records']): string[] {
  const indices = twoPageVegetationSource(original.form).covers.map(column => vegetationColumnIndex(original, column));
  return records.rows.filter(row => indices.some(index => row.cells[index].storage !== 'null')).map(row => row.rowId);
}
function auditText(column: string, value: ProjectMetadataCell): string | null {
  if (value.storage === 'blob' && value.blobHex !== null) {
    return `[${(value.blobHex.match(/../g) ?? []).map(byte => Number.parseInt(byte, 16)).join(' ')}]`;
  }
  return siviSourceAuditText({ name: column, declaredType: '' }, value);
}

export function twoPageVegetationReceiptFromWire(wire: unknown, retained: TwoPageVegetationRequest,
  owner: TwoPageVegetationOwner, recovery = false): TwoPageVegetationReceipt {
  const request = confirmedRequest(retained, owner);
  if (!record(wire) || !exact(wire, ['history', 'didCommit', 'replayed']) ||
    !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
    recovery && wire.didCommit !== false || !record(wire.history) ||
    !exact(wire.history, ['Request', 'Committed', 'Actor', 'AuditStrength', 'EditWhen', 'Audits'])) throw new Error(diagnostic);
  const history = wire.history;
  if (serializeTwoPageVegetationRequest(confirmedRequest(history.Request, owner)) !== serializeTwoPageVegetationRequest(request) ||
    typeof history.Actor !== 'string' || !history.Actor || history.Actor.length > 100 || !wellFormedUTF16(history.Actor) ||
    typeof history.AuditStrength !== 'number' || !Number.isInteger(history.AuditStrength) ||
    history.AuditStrength < 0 || history.AuditStrength > 3 ||
    typeof history.EditWhen !== 'string' || !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(history.EditWhen) ||
    !Array.isArray(history.Audits)) throw new Error(diagnostic);
  const time = new Date(`${history.EditWhen.replace(' ', 'T')}Z`);
  if (!Number.isFinite(time.getTime()) || time.toISOString().slice(0, 19).replace('T', ' ') !== history.EditWhen) throw new Error(diagnostic);
  const original = request.original;
  const expected = {
    columns: original.records.columns.map(column => ({ ...column })),
    rows: original.records.rows.map(row => ({ rowId: row.rowId, cells: row.cells.map(value => ({ ...value })) })),
  };
  for (const edit of request.edits) {
    const row = expected.rows.find(row => row.rowId === edit.rowId);
    if (!row) throw new Error(diagnostic);
    row.cells[vegetationColumnIndex(original, edit.column)] = { ...edit.value };
  }
  const committed = twoPageVegetationOriginalFromWire({
    ...original, records: history.Committed, sourceRowIds: sourceRows(original, expected),
  }, owner).records;
  if (JSON.stringify(committed.columns) !== JSON.stringify(expected.columns) || committed.rows.length !== expected.rows.length ||
    committed.rows.some((row, index) => row.rowId !== expected.rows[index].rowId ||
      row.cells.some((value, column) => !equalVegetationCell(value, expected.rows[index].cells[column])))) throw new Error(diagnostic);
  const strength = history.AuditStrength;
  // Audit value equality differs from the bit-exact source/history witnesses.
  const audited = request.edits.filter(edit => !equalCell(edit.expected, edit.value) &&
    (edit.expected.storage === 'null' ? strength >= 2 : edit.value.storage === 'null' ? strength >= 3 : strength >= 1));
  if (history.Audits.length !== audited.length) throw new Error(diagnostic);
  const audits: AuditEntry[] = [], seen = new Set<string>();
  for (const [index, audit] of history.Audits.entries()) {
    const edit = audited[index];
    if (!record(audit) || !exact(audit, ['rowId', 'project', 'user', 'plotNumber', 'table', 'editField',
      'editWhen', 'beforeEdit', 'afterEdit', 'restore', 'flag', 'id']) ||
      typeof audit.rowId !== 'string' || !exactSigned64(audit.rowId) || seen.has(audit.rowId) ||
      audit.project !== owner.project || audit.plotNumber !== owner.plot || audit.user !== history.Actor ||
      audit.table !== '_Veg' || audit.editField !== edit.column || audit.editWhen !== history.EditWhen ||
      audit.id !== twoPageVegetationEditableID(original, edit.rowId) || audit.restore !== false || audit.flag !== false ||
      audit.beforeEdit !== auditText(edit.column, edit.expected) || audit.afterEdit !== auditText(edit.column, edit.value)) throw new Error(diagnostic);
    seen.add(audit.rowId);
    audits.push({ rowId: audit.rowId, project: owner.project, plotNumber: owner.plot, user: history.Actor,
      table: '_Veg', editField: edit.column, editWhen: history.EditWhen, id: twoPageVegetationEditableID(original, edit.rowId),
      beforeEdit: auditText(edit.column, edit.expected), afterEdit: auditText(edit.column, edit.value), restore: false, flag: false });
  }
  return { history: { Request: request, Committed: committed, Actor: history.Actor,
    AuditStrength: strength, EditWhen: history.EditWhen, Audits: audits }, didCommit: wire.didCommit, replayed: wire.replayed };
}

export function twoPageVegetationCommittedOriginal(receipt: TwoPageVegetationReceipt, owner: TwoPageVegetationOwner): TwoPageVegetationOriginal {
  const original = receipt.history.Request.original;
  return twoPageVegetationOriginalFromWire({ ...original, records: receipt.history.Committed,
    sourceRowIds: sourceRows(original, receipt.history.Committed) }, owner);
}

export function twoPageVegetationOutcomeFromWire(wire: unknown, request: TwoPageVegetationRequest,
  requestJSON: string, owner: TwoPageVegetationOwner): TwoPageVegetationOutcome {
  const retainedOwner = { ...owner, contextId: request.original.contextId };
  const confirmed = confirmedRequest(request, retainedOwner);
  if (serializeTwoPageVegetationRequest(confirmed) !== requestJSON || !record(wire) ||
    !exact(wire, ['contextId', 'requestId', 'requestJSON', 'status', 'reason', 'receipt']) ||
    wire.contextId !== owner.contextId || wire.requestId !== request.requestId || wire.requestJSON !== requestJSON ||
    typeof wire.reason !== 'string' || !wire.reason || !wellFormedUTF16(wire.reason) ||
    !(wire.status === 'running' || wire.status === 'unknown' || wire.status === 'notCommitted' || wire.status === 'committed') ||
    wire.status === 'notCommitted' && request.original.contextId !== owner.contextId ||
    wire.status !== 'committed' && wire.receipt !== null) throw new Error('Vegetation outcome does not certify the exact retained attempt and owner.');
  return { contextId: owner.contextId, requestId: request.requestId, requestJSON, status: wire.status, reason: wire.reason,
    receipt: wire.status === 'committed' ? twoPageVegetationReceiptFromWire(wire.receipt, confirmed, retainedOwner, true) : null };
}
