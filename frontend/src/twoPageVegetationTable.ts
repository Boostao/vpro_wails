import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { metadataCellText } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';

export interface TwoPageVegetationTableOwner {
  contextId: string;
  project: string;
  projectPath: string;
  su: string;
  suPath: string;
  plot: string;
  form: 'FS882-8x6XL' | 'FS882-8x6XL-CHARS';
}
const columns = [
  ['PlotNumber', 'PlotNumber'], ['Species', 'Species'], ['Cover1', 'A1'], ['Cover2', 'A2'],
  ['Cover3', 'A3'], ['TotalA', 'Total A'], ['Cover4', 'B1'], ['Cover5', 'B2'],
  ['Cover5a', 'CHARS shrubs a'], ['Cover5b', 'CHARS shrubs b'], ['Cover5c', 'CHARS shrubs c'],
  ['TotalB', 'Total B'], ['Cover6', 'C'], ['Cover7', 'D'], ['Cover8', 'Dr/Dw'],
  ['Cover9', 'Ep'], ['Cover10', 'Cover10'],
] as const;
type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface TwoPageVegetationTableView extends TwoPageVegetationTableOwner {
  columns: { name: string; caption: string }[];
  rows: { vegRowId: string; suRowId: string | null; cells: ProjectMetadataCell[] }[];
  original: Table;
  selectedOriginal: Table | null;
  offset: number;
  limit: number;
  totalRows: number;
}
const record = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value);
function table(value: unknown): Table {
  const result = siviMetadataTableFromWire(value);
  if (!result.columns.length || new Set(result.columns.map(column => column.name)).size !== result.columns.length ||
      result.columns.some(column => !column.name || !wellFormedUTF16(column.name) || !wellFormedUTF16(column.declaredType))) {
    throw new Error('Vegetation table requires complete unique literal physical columns.');
  }
  return result;
}
function sameCell(left: unknown, right: ProjectMetadataCell): boolean {
  return record(left) && left.storage === right.storage && left.text === right.text &&
    left.integer === right.integer && Object.is(left.real, right.real) && left.blobHex === right.blobHex;
}
export function twoPageVegetationTableFromWire(value: unknown, owner: TwoPageVegetationTableOwner,
  offset: number, limit: number): TwoPageVegetationTableView {
  if (!record(value) || ![owner.contextId, owner.project, owner.projectPath, owner.su, owner.plot].every(text => text && wellFormedUTF16(text)) ||
      !wellFormedUTF16(owner.suPath) || !['FS882-8x6XL', 'FS882-8x6XL-CHARS'].includes(owner.form) ||
      owner.su !== 'None' && !owner.suPath ||
      !Number.isSafeInteger(offset) || offset < 0 || !Number.isInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('Vegetation table requires a valid immutable owner and bounded page request.');
  }
  if (Object.entries(owner).some(([key, expected]) => value[key] !== expected) ||
      value.table !== `${owner.project}_Veg` || value.suTable !== (owner.su === 'None' ? '' : `${owner.su}_SU`) ||
      value.offset !== offset || value.limit !== limit || !Number.isSafeInteger(value.totalRows) ||
      typeof value.totalRows !== 'number' || value.totalRows < offset ||
      value.totalRows > (owner.su === 'None' ? 100000 : 10000000000) ||
      value.generalLocaleVerified !== false ||
      value.orderingPolicy !== 'controlled-English-ignore-case; numeric-descending-NULL-last; physical-row-ties') {
    throw new Error('Vegetation table response has foreign ownership, page or unsupported ordering policy.');
  }
  const expected = columns.slice(0, owner.form.endsWith('-CHARS') ? 14 : 17);
  const wireColumns = value.columns;
  if (!Array.isArray(wireColumns) || wireColumns.length !== expected.length ||
      expected.some(([name, caption], index) => !record(wireColumns[index]) ||
        wireColumns[index].name !== name || wireColumns[index].caption !== caption) ||
      !Array.isArray(value.rows) || value.rows.length !== Math.min(limit, value.totalRows - offset)) {
    throw new Error('Vegetation table source columns or complete joined page were not returned.');
  }
  const original = table(value.original);
  const selectedOriginal = owner.su === 'None' ? null : table(value.selectedOriginal);
  if (owner.su === 'None' && value.selectedOriginal !== null) throw new Error('None scope returned invented SU originals.');
  const indices = expected.map(([name]) => original.columns.findIndex(column => column.name === name));
  if (indices.some(index => index < 0)) throw new Error('Vegetation table original lost a source binding.');
  const veg = new Map(original.rows.map(row => [row.rowId, row]));
  const su = new Map(selectedOriginal?.rows.map(row => [row.rowId, row]) ?? []);
  const suPlot = selectedOriginal?.columns.findIndex(column => column.name === 'PlotNumber') ?? -1;
  if (selectedOriginal && suPlot < 0) throw new Error('Vegetation table selected original lost PlotNumber.');
  const usedVeg = new Set<string>(), usedSU = new Set<string>(), pairs = new Set<string>();
  const rows: TwoPageVegetationTableView['rows'] = Array.from(value.rows).map(row => {
    if (!record(row) || typeof row.vegRowId !== 'string' || !veg.has(row.vegRowId) || !Array.isArray(row.cells)) {
      throw new Error('Vegetation table projection differs from its complete physical original.');
    }
    const source = veg.get(row.vegRowId)!;
    const cells = row.cells;
    if (cells.length !== expected.length || indices.some((index, position) => !sameCell(cells[position], source.cells[index]))) {
      throw new Error('Vegetation table projection differs from its complete physical original.');
    }
    if (!['null', 'text'].includes(source.cells[indices[0]].storage) ||
        source.cells[indices[0]].storage === 'null' && owner.su !== 'None' ||
        indices.slice(2).some(index => !['null', 'integer', 'real'].includes(source.cells[index].storage))) {
      throw new Error('Vegetation table projected plot or cover storage is unsupported.');
    }
    const suRowId = row.suRowId;
    if (suRowId !== null && typeof suRowId !== 'string' || (owner.su === 'None' ? suRowId !== null :
      typeof suRowId !== 'string' || !su.has(suRowId) || su.get(suRowId)!.cells[suPlot].storage !== 'text')) {
      throw new Error('Vegetation table joined row lost its selected physical witness.');
    }
    const pair = JSON.stringify([row.vegRowId, row.suRowId]);
    if (pairs.has(pair)) throw new Error('Vegetation table returned a repeated physical join pair.');
    pairs.add(pair); usedVeg.add(row.vegRowId);
    if (typeof row.suRowId === 'string') usedSU.add(row.suRowId);
    return { vegRowId: row.vegRowId, suRowId, cells: indices.map(index => structuredClone(source.cells[index])) };
  });
  if (usedVeg.size !== veg.size || usedSU.size !== su.size) throw new Error('Vegetation table contains unrelated page originals.');
  return { ...owner, columns: expected.map(([name, caption]) => ({ name, caption })), rows,
    original, selectedOriginal, offset, limit, totalRows: value.totalRows };
}
export function twoPageVegetationTableCellLabel(cell: ProjectMetadataCell): string {
  if (cell.storage === 'null') return '(NULL)';
  if (cell.storage === 'text' && cell.text === '') return '(empty text)';
  return metadataCellText(cell);
}
