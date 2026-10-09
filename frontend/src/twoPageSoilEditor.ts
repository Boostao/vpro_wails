import type {
  ProjectMetadataCell, ProjectMetadataColumn, TwoPageSoilWriteResult,
} from '../bindings/github.com/boostao/vpro-wails';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { exactSigned64 } from './projectMetadataRestore';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';
import { soilFields, type SoilKind } from './soilChildEditor';
import { validSIVIRequestId } from './siviRequestId';

export type TwoPageSoilForm = 'SoilHumusXL' | 'SoilMineralXL';
type SoilWireRequest = TwoPageSoilWriteResult['history']['Request'];
export type TwoPageSoilRequest = Omit<SoilWireRequest, 'original' | 'edits'> & {
  original: TwoPageSoilOriginal;
  edits: NonNullable<SoilWireRequest['edits']>;
};
type TwoPageSoilEdit = NonNullable<SoilWireRequest['edits']>[number];
export interface TwoPageSoilOwner { contextId: string; project: string; plot: string; form: TwoPageSoilForm }
export interface TwoPageSoilOriginal extends TwoPageSoilOwner {
  recordSource: string;
  table: string;
  records: { columns: ProjectMetadataColumn[]; rows: { rowId: string; cells: ProjectMetadataCell[] }[] };
}
export interface TwoPageSoilDraft extends TwoPageSoilEdit {
  raw: string;
  nullValue: boolean;
  error: string | null;
}
export type TwoPageSoilDrafts = Record<string, TwoPageSoilDraft>;
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
function keys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  return Object.keys(value).length === expected.length && expected.every(name => Object.hasOwn(value, name));
}
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value !== '' && wellFormedUTF16(value);
export const twoPageSoilKind = (form: TwoPageSoilForm): SoilKind => form === 'SoilHumusXL' ? 'Humus' : 'Mineral';
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

export function twoPageSoilOriginalFromWire(wire: unknown, owner: TwoPageSoilOwner): TwoPageSoilOriginal {
  if (!record(wire) || !keys(wire, ['contextId', 'project', 'plot', 'form', 'recordSource', 'table', 'records']) ||
      !literal(owner.contextId) || !literal(owner.project) || !literal(owner.plot) ||
      !['SoilHumusXL', 'SoilMineralXL'].includes(owner.form) ||
      wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot || wire.form !== owner.form) {
    throw new Error('Soil original belongs to another exact context, project, parent or source form.');
  }
  const kind = twoPageSoilKind(owner.form);
  if (wire.recordSource !== `Usys${kind}` || wire.table !== `${owner.project}_${kind}` ||
      !record(wire.records) || !keys(wire.records, ['columns', 'rows']) ||
      !Array.isArray(wire.records.columns) || !Array.isArray(wire.records.rows)) {
    throw new Error('Soil original physical source or complete schema/rows were not returned.');
  }
  const columns: ProjectMetadataColumn[] = [];
  for (const column of Array.from(wire.records.columns)) {
    if (!record(column) || !keys(column, ['name', 'declaredType']) || !literal(column.name) ||
        typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) {
      throw new Error('Soil original column identity or declaredType is incomplete.');
    }
    columns.push({ name: column.name, declaredType: column.declaredType });
  }
  if (new Set(columns.map(column => column.name)).size !== columns.length ||
      !['ID', 'PlotNumber'].every(name => columns.some(column => column.name === name))) {
    throw new Error('Soil original requires distinct columns and explicit ID/PlotNumber storage.');
  }
  const rows: TwoPageSoilOriginal['records']['rows'] = [];
  const parentIndex = columns.findIndex(column => column.name === 'PlotNumber');
  for (const row of Array.from(wire.records.rows)) {
    if (!record(row) || !keys(row, ['rowId', 'cells']) || typeof row.rowId !== 'string' ||
        !exactSigned64(row.rowId) || !Array.isArray(row.cells) || row.cells.length !== columns.length) {
      throw new Error('Soil original requires complete exact signed64 physical rows.');
    }
    const cells: ProjectMetadataCell[] = [];
    for (const cell of Array.from(row.cells)) {
      if (!record(cell) || !keys(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
          !completeMetadataCellFromWire(cell)) throw new Error('Soil original contains incomplete tagged storage.');
      cells.push({ storage: cell.storage, text: cell.text, integer: cell.integer, real: cell.real, blobHex: cell.blobHex });
    }
    if (cells[parentIndex].storage !== 'text' || cells[parentIndex].text !== owner.plot) {
      throw new Error('Soil original row does not belong to its exact literal TEXT parent.');
    }
    rows.push({ rowId: row.rowId, cells });
  }
  if (new Set(rows.map(row => row.rowId)).size !== rows.length) throw new Error('Soil original repeats physical rowids.');
  return { ...owner, recordSource: `Usys${kind}`, table: `${owner.project}_${kind}`, records: { columns, rows } };
}

function sourceCell(original: TwoPageSoilOriginal, rowId: string, column: string): ProjectMetadataCell {
  const row = original.records.rows.find(row => row.rowId === rowId);
  const index = original.records.columns.findIndex(field => field.name === column);
  if (!row || index < 0) throw new Error('Soil source row/field is unavailable; reload without inventing an identity.');
  return row.cells[index];
}
function identityMatches(cell: ProjectMetadataCell, id: string): boolean {
  if (cell.storage === 'integer') return cell.integer === id;
  const expected = Number(id);
  if (cell.storage === 'real') return cell.real === expected;
  if (cell.storage !== 'text' || cell.text === null) return false;
  // Match strconv.ParseFloat/strings.TrimSpace, not JavaScript's broader numeric coercion.
  const text = cell.text.replace(/^[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+|[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/g, '');
  const decimal = /^[+-]?(?:(?:[0-9](?:_?[0-9])*)(?:\.(?:[0-9](?:_?[0-9])*)?)?|\.[0-9](?:_?[0-9])*)(?:[eE][+-]?[0-9](?:_?[0-9])*)?$/;
  if (decimal.test(text)) return Number(text.replaceAll('_', '')) === expected;
  const hex = /^[+-]?0[xX](?:_(?=[0-9a-fA-F]))?(?:[0-9a-fA-F](?:_?[0-9a-fA-F])*(?:\.(?:[0-9a-fA-F](?:_?[0-9a-fA-F])*)?)?|\.[0-9a-fA-F](?:_?[0-9a-fA-F])*)[pP][+-]?[0-9](?:_?[0-9])*$/;
  if (!hex.test(text)) return false;
  const [mantissa, exponent] = text.replaceAll('_', '').split(/[pP]/);
  const unsigned = mantissa.replace(/^[+-]?0[xX]/, '');
  const fractionLength = unsigned.includes('.') ? unsigned.length - unsigned.indexOf('.') - 1 : 0;
  const binary = BigInt(`0x${unsigned.replace('.', '')}`).toString(2);
  if (binary === '0') return expected === 0;
  const exactPower = Number(exponent) - 4 * fractionLength;
  if (expected === 0) {
    const highestPower = exactPower + binary.length - 1;
    return highestPower < -1075 || (highestPower === -1075 && !binary.slice(1).includes('1'));
  }
  const discarded = Math.max(0, binary.length - 53);
  let significand = BigInt(`0b${binary.slice(0, 53)}`);
  if (discarded && binary[53] === '1' &&
      (binary.slice(54).includes('1') || (significand & 1n) === 1n)) significand++;
  const power = exactPower + discarded;
  const magnitude = Number(significand) * 2 ** power;
  return (mantissa.startsWith('-') ? -magnitude : magnitude) === expected;
}
export function twoPageSoilEditableID(original: TwoPageSoilOriginal, rowId: string): number {
  const identity = sourceCell(original, rowId, 'ID');
  if (identity.storage !== 'integer' || identity.integer === null || !exactSigned64(identity.integer) ||
      BigInt(identity.integer) < -2147483648n || BigInt(identity.integer) > 2147483647n) {
    throw new Error('NULL/noninteger soil IDs remain read-only; ID=0 is never inferred.');
  }
  const id = identity.integer;
  if (original.records.rows.some(row => row.rowId !== rowId &&
      identityMatches(sourceCell(original, row.rowId, 'ID'), id))) {
    throw new Error('Soil ID belongs to multiple original physical rows.');
  }
  return Number(id);
}

export function stageTwoPageSoil(original: TwoPageSoilOriginal, rowId: string, column: string,
  raw: string, nullValue: boolean): TwoPageSoilDraft {
  twoPageSoilEditableID(original, rowId);
  const field = soilFields[twoPageSoilKind(original.form)].find(field => field.column === column);
  if (!field) throw new Error('Soil field is outside its exact visible source data bindings.');
  const expected = sourceCell(original, rowId, column);
  let value = nullCell(), error: string | null = null;
  if (!nullValue) {
    if (!wellFormedUTF16(raw)) error = `${column} contains incomplete Unicode; raw input was not repaired.`;
    else if (expected.storage !== 'null' && raw === metadataCellText(expected)) value = { ...expected };
    else if (field.kind === 'text' || field.kind === 'memo') {
      value = { ...value, storage: 'text', text: raw };
      if (field.maximum !== undefined && raw.length > field.maximum) error = `${column} exceeds ${field.maximum} UTF-16 units.`;
    } else if (field.kind === 'integer') {
      if (raw.length > 6 || !/^-?(0|[1-9]\d*)$/.test(raw) || raw === '-0' || BigInt(raw) < -32768n || BigInt(raw) > 32767n) {
        error = `${column} requires an exact Access Integer (-32768 to 32767); raw input was not rounded or completed.`;
      } else value = { ...value, storage: 'integer', integer: raw };
    } else {
      const parsed = finiteSingleValue(column, raw);
      if (raw !== raw.trim() || raw === '' || parsed.error || parsed.value === null) {
        error = parsed.error ?? `${column} requires a finite number without added whitespace, or explicit NULL.`;
      } else value = { ...value, storage: 'real', real: parsed.value };
    }
  }
  return { contextId: original.contextId, form: original.form, rowId, column,
    expected: { ...expected }, value, raw, nullValue, error };
}

export function twoPageSoilRequest(original: TwoPageSoilOriginal, owner: TwoPageSoilOwner,
  drafts: TwoPageSoilDrafts, requestId: string): TwoPageSoilRequest {
  const reviewed = twoPageSoilOriginalFromWire(original, owner);
  if (!validSIVIRequestId(requestId)) throw new Error('Soil Save requires a retained lowercase UUIDv4.');
  const edits: TwoPageSoilEdit[] = [], seen = new Set<string>();
  for (const draft of Object.values(drafts)) {
    const staged = stageTwoPageSoil(reviewed, draft.rowId, draft.column, draft.raw, draft.nullValue);
    const key = JSON.stringify([draft.rowId, draft.column]);
    if (draft.contextId !== owner.contextId || draft.form !== owner.form || seen.has(key) ||
        !equalCell(draft.expected, staged.expected) || draft.error || staged.error || !equalCell(draft.value, staged.value)) {
      throw new Error(draft.error ?? staged.error ?? 'Soil draft belongs to another original or has inconsistent raw/value state.');
    }
    seen.add(key);
    if (equalCell(staged.expected, staged.value)) continue;
    edits.push({ contextId: owner.contextId, form: owner.form, rowId: staged.rowId, column: staged.column,
      expected: { ...staged.expected }, value: { ...staged.value } });
  }
  if (!edits.length) throw new Error('Soil Save requires at least one explicit changed data cell.');
  return { requestId, original: reviewed, edits };
}
