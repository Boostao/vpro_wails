import type { AuditEntry, ProjectMetadataColumn, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { equalCell } from './projectMetadataEditor';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { siviDeletionOriginalFromWire, siviDeletionRequest, type SIVIDeletionRequest } from './siviDeletionEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVIDeletionReceipt extends SIVIDeletionRequest {
  rowId: string;
  id: number;
  historyId: string;
  actor: string;
  editWhen: string;
  auditStrength: number;
  request: SIVIDeletionRequest;
  audits: AuditEntry[];
  didCommit: boolean;
  replayed: boolean;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const only = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).every(key => keys.includes(key));
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value.length > 0 && wellFormedUTF16(value) && !value.includes('\0');
const requestKeys = ['requestId', 'contextId', 'project', 'plot', 'form', 'columns', 'original'];
const receiptKeys = [...requestKeys.filter(key => key !== 'requestId'), 'requestId', 'rowId', 'id', 'historyId',
  'actor', 'editWhen', 'auditStrength', 'request', 'audits', 'didCommit', 'replayed'];
const auditKeys = ['rowId', 'project', 'user', 'plotNumber', 'table', 'editField', 'editWhen',
  'beforeEdit', 'afterEdit', 'restore', 'flag', 'id'];

function sameOriginal(value: unknown, request: SIVIDeletionRequest): boolean {
  if (!record(value) || !record(value.original) || !only(value.original, ['rowId', 'cells'])) return false;
  const original = siviDeletionOriginalFromWire(value, request, request.form, request.original.rowId);
  return original.columns.every((column, index) => column.name === request.columns[index].name &&
    column.declaredType === request.columns[index].declaredType &&
    equalCell(original.original.cells[index], request.original.cells[index]));
}

export function siviSourceAuditText(column: ProjectMetadataColumn, cell: ProjectMetadataCell): string | null {
  if (cell.storage === 'null') return null;
  if (cell.storage === 'text') return cell.text;
  if (cell.storage === 'integer') {
    return column.name === 'Flag' || /^(BOOLEAN|BOOL|BIT)$/i.test(column.declaredType)
      ? cell.integer === '0' ? '0' : '-1' : cell.integer;
  }
  if (cell.storage !== 'real' || cell.real === null) throw new Error('SIVI deletion audit has unavailable original storage.');
  if (Object.is(cell.real, -0)) return '-0';
  // fmt.Sprint uses shortest Go g-format: scientific outside exponents [-4, 6).
  const [coefficient, exponentText] = cell.real.toExponential().split('e');
  const exponent = Number(exponentText);
  return exponent >= -4 && exponent < 6 ? String(cell.real)
    : `${coefficient}e${exponent < 0 ? '-' : '+'}${String(Math.abs(exponent)).padStart(2, '0')}`;
}

export function siviDeletionReceiptFromWire(wire: unknown, request: SIVIDeletionRequest,
  owner: SIVISpeciesOwner, operation: 'delete' | 'lookup' = 'delete'): SIVIDeletionReceipt {
  const diagnostic = 'SIVI deletion receipt differs from the exact retained original/request; resolve durable history without replaying.';
  siviDeletionRequest(request, request, request.requestId, true);
  if (!record(wire) || !only(wire, receiptKeys) ||
    wire.requestId !== request.requestId || wire.historyId !== request.requestId ||
    wire.contextId !== owner.contextId || request.project !== owner.project || request.plot !== owner.plot ||
    wire.project !== owner.project || wire.plot !== owner.plot || wire.form !== request.form ||
    wire.rowId !== request.original.rowId || !Number.isInteger(wire.id) ||
    Number(wire.id) < -2147483648 || Number(wire.id) > 2147483647 ||
    !literal(wire.actor) || wire.actor.length > 100 ||
    typeof wire.editWhen !== 'string' || wire.editWhen.length !== 19 ||
    !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(wire.editWhen) || siviDateTimestampError(wire.editWhen) !== null ||
    !Number.isInteger(wire.auditStrength) || Number(wire.auditStrength) < 0 || Number(wire.auditStrength) > 3 ||
    !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
    operation === 'lookup' && wire.didCommit !== false ||
    !record(wire.request) || !only(wire.request, requestKeys) ||
    wire.request.requestId !== request.requestId || wire.request.contextId !== request.contextId ||
    !sameOriginal(wire.request, request) ||
    !sameOriginal({ ...wire, contextId: request.contextId }, request) || !Array.isArray(wire.audits)) {
    throw new Error(diagnostic);
  }
  const logical = request.original.cells[request.columns.findIndex(column => column.name === 'ID')];
  if (String(wire.id) !== logical.integer) throw new Error(diagnostic);
  const expected = new Map<string, string>();
  if (wire.auditStrength === 3) {
    for (const [index, column] of request.columns.entries()) {
      if (['ID', 'PlotNumber'].includes(column.name)) continue;
      const value = siviSourceAuditText(column, request.original.cells[index]);
      if (value !== null) expected.set(column.name, value);
    }
  }
  const audits: AuditEntry[] = [], seen = new Set<string>();
  for (const audit of wire.audits) {
    if (!record(audit) || !only(audit, auditKeys) || typeof audit.rowId !== 'string' ||
      !exactSigned64(audit.rowId) || seen.has(audit.rowId) ||
      audit.project !== owner.project || audit.plotNumber !== owner.plot || audit.user !== wire.actor ||
      audit.editWhen !== wire.editWhen || audit.table !== '_Veg' || audit.id !== wire.id ||
      typeof audit.editField !== 'string' || !expected.has(audit.editField) ||
      audit.beforeEdit !== expected.get(audit.editField) ||
      audit.afterEdit !== null || audit.restore !== false || audit.flag !== false) throw new Error(diagnostic);
    seen.add(audit.rowId);
    expected.delete(audit.editField);
    audits.push({ rowId: audit.rowId, project: owner.project, plotNumber: owner.plot, user: wire.actor,
      editWhen: wire.editWhen, table: '_Veg', id: Number(wire.id), editField: audit.editField,
      beforeEdit: String(audit.beforeEdit), afterEdit: null, restore: false, flag: false });
  }
  if (expected.size) throw new Error(diagnostic);
  return { ...structuredClone(request), contextId: owner.contextId, rowId: request.original.rowId,
    id: Number(wire.id), historyId: request.requestId, actor: wire.actor, editWhen: wire.editWhen,
    auditStrength: Number(wire.auditStrength), request: structuredClone(request), audits,
    didCommit: wire.didCommit, replayed: wire.replayed };
}
