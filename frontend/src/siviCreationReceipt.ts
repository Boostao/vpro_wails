import type { SIVICreationResult, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import defaults from '../../resources/sivi-new-row-defaults.json';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { completeMetadataCellFromWire, siviMetadataTableFromWire } from './siviParentTransport';
import type { SIVICreationRequest } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

type Table = ReturnType<typeof siviMetadataTableFromWire>;
export type SIVICreationReceipt = Omit<SIVICreationResult, 'columns' | 'original' | 'committed' | 'covers' | 'request'> & {
  columns: Table['columns'];
  original: Table['rows'][number];
  committed: Table['rows'][number];
  covers: SIVICreationRequest['covers'];
  request: SIVICreationRequest;
};
const names = new Set([...defaults.initialNullColumns, defaults.initialBoolean.column, defaults.identity.column]);
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value.length > 0 && wellFormedUTF16(value) && !value.includes('\0');
const only = (value: Record<string, unknown>, keys: string[]) => Object.keys(value).every(key => keys.includes(key));
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
const integerCell = (integer: string): ProjectMetadataCell => ({ ...nullCell(), storage: 'integer', integer });
const textCell = (text: string): ProjectMetadataCell => ({ ...nullCell(), storage: 'text', text });

function coversMatch(value: unknown, request: SIVICreationRequest): boolean {
  return Array.isArray(value) && value.length === request.covers.length && Array.from(value).every((cover, index) =>
    record(cover) && only(cover, ['column', 'value']) && cover.column === request.covers[index].column &&
    completeMetadataCellFromWire(cover.value) && equalCell(cover.value, request.covers[index].value));
}
function requestMatches(value: unknown, request: SIVICreationRequest): boolean {
  if (!record(value) || !only(value, ['requestId', 'contextId', 'project', 'plot', 'form', 'species', 'decision', 'covers']) ||
      value.requestId !== request.requestId || value.contextId !== request.contextId ||
      value.project !== request.project || value.plot !== request.plot || value.form !== request.form ||
      value.species !== request.species || !coversMatch(value.covers, request)) return false;
  if (!request.decision) return value.decision === undefined;
  return record(value.decision) && only(value.decision, ['kind', 'entered', 'selected']) &&
    value.decision.kind === request.decision.kind && value.decision.entered === request.decision.entered &&
    value.decision.selected === request.decision.selected;
}

export function siviCreationReceiptFromWire(wire: unknown, request: SIVICreationRequest,
  owner: SIVISpeciesOwner, operation: 'create' | 'lookup' = 'create'): SIVICreationReceipt {
  const diagnostic = 'SIVI creation receipt is incomplete or differs from its source-owned request; resolve durable history without replaying.';
  if (!record(wire) || wire.requestId !== request.requestId || wire.contextId !== owner.contextId ||
      request.project !== owner.project || request.plot !== owner.plot ||
      wire.project !== owner.project || wire.plot !== owner.plot || wire.form !== request.form ||
      typeof wire.rowId !== 'string' || !literal(wire.actor) || wire.actor.length > 100 ||
      !Number.isInteger(wire.id) || Number(wire.id) <= 0 || Number(wire.id) > 2147483647 ||
      wire.historyId !== request.requestId ||
      !Number.isInteger(wire.auditStrength) || Number(wire.auditStrength) < 0 || Number(wire.auditStrength) > 3 ||
      typeof wire.editWhen !== 'string' || !/^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d$/.test(wire.editWhen) ||
      wire.editWhen.length !== 19 || siviDateTimestampError(wire.editWhen) !== null ||
      !(wire.didCommit === true && wire.replayed === false || wire.didCommit === false && wire.replayed === true) ||
      operation === 'lookup' && wire.didCommit !== false ||
      !requestMatches(wire.request, request) || !coversMatch(wire.covers, request) ||
      !record(wire.original) || wire.original.rowId !== '' || !Array.isArray(wire.original.cells) ||
      wire.original.cells.length !== names.size) {
    throw new Error(diagnostic);
  }
  const originalCells: unknown[] = wire.original.cells;
  if (!originalCells.every(completeMetadataCellFromWire)) throw new Error(diagnostic);
  const table = siviMetadataTableFromWire({ columns: wire.columns, rows: [wire.committed] });
  if (table.columns.length !== names.size || new Set(table.columns.map(column => column.name)).size !== names.size ||
      table.columns.some(column => !names.has(column.name) || !wellFormedUTF16(column.declaredType)) ||
      table.rows[0].rowId !== wire.rowId) throw new Error(diagnostic);
  const committed = table.rows[0];
  const original = { rowId: '', cells: originalCells.map(cell => ({ ...cell })) };
  for (const [index, column] of table.columns.entries()) {
    const initial = column.name === defaults.initialBoolean.column ? integerCell(defaults.initialBoolean.integer) : nullCell();
    const assigned = column.name === defaults.identity.column ? integerCell(String(wire.id))
      : column.name === 'PlotNumber' ? textCell(request.plot)
      : column.name === 'Species' ? textCell(request.species)
      : request.covers.find(cover => cover.column === column.name)?.value ?? initial;
    if (!equalCell(original.cells[index], initial) || !equalCell(committed.cells[index], assigned)) throw new Error(diagnostic);
  }
  return {
    requestId: request.requestId, contextId: owner.contextId, project: owner.project, plot: owner.plot, form: request.form,
    rowId: wire.rowId, historyId: wire.historyId, id: Number(wire.id), actor: wire.actor,
    auditStrength: Number(wire.auditStrength), editWhen: wire.editWhen, columns: table.columns, original, committed,
    covers: structuredClone(request.covers), request: structuredClone(request), didCommit: wire.didCommit, replayed: wire.replayed,
  };
}
