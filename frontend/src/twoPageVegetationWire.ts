import type { ProjectMetadataCell, ProjectMetadataColumn, TwoPageVegetationWriteResult } from '../bindings/github.com/boostao/vpro-wails';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { exactSigned64 } from './projectMetadataRestore';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import identifierFold from './twoPageVegetationIdentifierFold.json';
import type { VegetationReferences } from './twoPageVegetationReferences';

export type TwoPageVegetationForm = 'SubVegAXL_BC' | 'SubVegAXL' | 'SubVegCXL' | 'SubVegDXL';
export interface TwoPageVegetationOwner { contextId: string; project: string; plot: string; form: TwoPageVegetationForm }
export interface TwoPageVegetationOriginal extends TwoPageVegetationOwner {
  recordSource: string;
  table: string;
  records: { columns: ProjectMetadataColumn[]; rows: { rowId: string; cells: ProjectMetadataCell[] }[] };
  sourceRowIds: string[];
}
type WireRequest = TwoPageVegetationWriteResult['history']['Request'];
export type TwoPageVegetationEdit = NonNullable<WireRequest['edits']>[number];
export type TwoPageVegetationRequest = Omit<WireRequest, 'original' | 'edits' | 'speciesList'> & {
  original: TwoPageVegetationOriginal;
  edits: TwoPageVegetationEdit[];
  speciesList?: VegetationReferences;
};
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const keys = (value: Record<string, unknown>, expected: string[]): boolean =>
  Object.keys(value).length === expected.length && expected.every(name => Object.hasOwn(value, name));
const literal = (value: unknown): value is string =>
  typeof value === 'string' && value !== '' && wellFormedUTF16(value);
export const vegetationNullCell = (): ProjectMetadataCell =>
  ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
export const equalVegetationCell = (left: ProjectMetadataCell, right: ProjectMetadataCell): boolean =>
  equalCell(left, right) && Object.is(left.real, right.real);

const identifierKeys = new Map(identifierFold.groups.flatMap(group => {
  const runes = Array.from(group);
  return runes.map(rune => [rune, runes[0]] as const);
}));
export function vegetationIdentifierKey(name: string): string {
  return Array.from(name, rune => identifierKeys.get(rune) ?? rune).join('');
}

export function twoPageVegetationSource(form: TwoPageVegetationForm): { source: string; covers: string[] } {
  switch (form) {
    case 'SubVegAXL_BC':
    case 'SubVegAXL':
      return { source: 'USysVegA', covers: ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB', 'Cover5a', 'Cover5b', 'Cover5c'] };
    case 'SubVegCXL': return { source: 'USysVegC', covers: ['Cover6'] };
    case 'SubVegDXL': return { source: 'USysVegD', covers: ['Cover7', 'Cover8', 'Cover9'] };
    default: throw new Error('Vegetation source form is unavailable.');
  }
}

export function vegetationColumnIndex(original: TwoPageVegetationOriginal, column: string): number {
  const indices = original.records.columns.flatMap((field, index) => vegetationIdentifierKey(field.name) === vegetationIdentifierKey(column) ? [index] : []);
  if (indices.length !== 1) throw new Error(`Vegetation physical source requires one ${column} column.`);
  return indices[0];
}

export function twoPageVegetationOriginalFromWire(wire: unknown, owner: TwoPageVegetationOwner): TwoPageVegetationOriginal {
  if (!record(wire) || !keys(wire, ['contextId', 'project', 'plot', 'form', 'recordSource', 'table', 'records', 'sourceRowIds']) ||
    !literal(owner.contextId) || !literal(owner.project) || !literal(owner.plot) ||
    !['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL'].includes(owner.form) ||
    wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot || wire.form !== owner.form) {
    throw new Error('Vegetation original belongs to another exact context, project, parent or source form.');
  }
  const policy = twoPageVegetationSource(owner.form);
  if (wire.recordSource !== policy.source || wire.table !== `${owner.project}_Veg` ||
    !record(wire.records) || !keys(wire.records, ['columns', 'rows']) ||
    !Array.isArray(wire.records.columns) || !Array.isArray(wire.records.rows) || !Array.isArray(wire.sourceRowIds)) {
    throw new Error('Vegetation physical source, complete rows or separate source membership were not returned.');
  }
  const columns: ProjectMetadataColumn[] = [];
  for (const column of Array.from(wire.records.columns)) {
    if (!record(column) || !keys(column, ['name', 'declaredType']) || !literal(column.name) ||
      typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) {
      throw new Error('Vegetation original column identity or declaredType is incomplete.');
    }
    columns.push({ name: column.name, declaredType: column.declaredType });
  }
  if (new Set(columns.map(column => vegetationIdentifierKey(column.name))).size !== columns.length) {
    throw new Error('Vegetation original requires distinct physical column identities.');
  }
  const original: TwoPageVegetationOriginal = {
    ...owner, recordSource: policy.source, table: `${owner.project}_Veg`, records: { columns, rows: [] }, sourceRowIds: [],
  };
  const parentIndex = vegetationColumnIndex(original, 'PlotNumber');
  vegetationColumnIndex(original, 'ID');
  const coverIndices = policy.covers.map(column => vegetationColumnIndex(original, column));
  for (const row of Array.from(wire.records.rows)) {
    if (!record(row) || !keys(row, ['rowId', 'cells']) || typeof row.rowId !== 'string' ||
      !exactSigned64(row.rowId) || !Array.isArray(row.cells) || row.cells.length !== columns.length) {
      throw new Error('Vegetation original requires complete exact signed64 physical rows.');
    }
    const cells: ProjectMetadataCell[] = [];
    for (const cell of Array.from(row.cells)) {
      if (!record(cell) || !keys(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
        !completeMetadataCellFromWire(cell)) throw new Error('Vegetation original contains incomplete tagged storage.');
      cells.push({ storage: cell.storage, text: cell.text, integer: cell.integer, real: cell.real, blobHex: cell.blobHex });
    }
    if (cells[parentIndex].storage !== 'text' || cells[parentIndex].text !== owner.plot) {
      throw new Error('Vegetation original row does not belong to its exact literal TEXT parent.');
    }
    original.records.rows.push({ rowId: row.rowId, cells });
    if (coverIndices.some(index => cells[index].storage !== 'null')) original.sourceRowIds.push(row.rowId);
  }
  if (new Set(original.records.rows.map(row => row.rowId)).size !== original.records.rows.length ||
    wire.sourceRowIds.length !== original.sourceRowIds.length ||
    !Array.from(wire.sourceRowIds).every((rowId, index) => rowId === original.sourceRowIds[index])) {
    throw new Error('Vegetation physical identities or independent source membership differ.');
  }
  return original;
}

export function vegetationSourceCell(original: TwoPageVegetationOriginal, rowId: string, column: string): ProjectMetadataCell {
  const row = original.records.rows.find(row => row.rowId === rowId);
  if (!row) throw new Error('Vegetation original physical row is unavailable; do not invent an identity.');
  return row.cells[vegetationColumnIndex(original, column)];
}

// JSON.stringify normalizes -0; exact historical REAL witnesses must retain it.
export function serializeTwoPageVegetationRequest(request: TwoPageVegetationRequest): string {
  function encode(value: unknown): string {
    if (value === null) return 'null';
    if (typeof value === 'string') {
      if (!wellFormedUTF16(value)) throw new Error('Vegetation request contains incomplete Unicode.');
      return JSON.stringify(value);
    }
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    if (typeof value === 'number' && Number.isFinite(value)) return Object.is(value, -0) ? '-0' : String(value);
    if (Array.isArray(value)) return `[${Array.from(value, encode).join(',')}]`;
    if (record(value)) return `{${Object.entries(value).map(([key, item]) => `${encode(key)}:${encode(item)}`).join(',')}}`;
    throw new Error('Vegetation request contains unavailable scalar storage.');
  }
  return encode(request);
}
