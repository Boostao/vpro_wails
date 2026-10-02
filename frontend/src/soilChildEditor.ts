import { finiteSingleValue } from './numericEditor';
import { wellFormedUTF16 } from './qualityEditor';
import type { SoilRecordUpdate } from '../bindings/github.com/boostao/vpro-wails';

export type SoilKind = 'Humus' | 'Mineral';
export interface SoilField {
  key: string; column: string; kind: 'text' | 'memo' | 'single' | 'integer'; maximum?: number; group?: string;
}
export const soilFields: Record<SoilKind, readonly SoilField[]> = {
  Humus: [
    { key: 'horizon', column: 'Horizon', kind: 'text', maximum: 8, group: 'HumusHorizon' },
    { key: 'upperDepth', column: 'UpperDepth', kind: 'single' },
    { key: 'lowerDepth', column: 'LowerDepth', kind: 'single' },
    { key: 'humusStructureDegree', column: 'HumusStructureDegree', kind: 'text', maximum: 1, group: 'HumusStructureDegree' },
    { key: 'humusStructureKind', column: 'HumusStructureKind', kind: 'text', maximum: 5, group: 'HumusStructureKind' },
    { key: 'mycelAbundance', column: 'MycelAbundance', kind: 'text', maximum: 1, group: 'MycelAbundance' },
    { key: 'fecalAbundance', column: 'FecalAbundance', kind: 'text', maximum: 1, group: 'MycelAbundance' },
    { key: 'rootsAbundance', column: 'RootsAbundance', kind: 'text', maximum: 6 },
    { key: 'rootsSize', column: 'RootsSize', kind: 'text', maximum: 6 },
    { key: 'vonPost', column: 'vonPost', kind: 'integer', group: 'vonPost' },
    { key: 'ph', column: 'HumusFormpH', kind: 'single' },
    { key: 'comment', column: 'Comment', kind: 'memo' },
  ],
  Mineral: [
    { key: 'horizon', column: 'Horizon', kind: 'text', maximum: 8 },
    { key: 'upperDepth', column: 'UpperDepth', kind: 'single' },
    { key: 'lowerDepth', column: 'LowerDepth', kind: 'single' },
    { key: 'pitDepthLimit', column: 'PitDepthLimit', kind: 'text', maximum: 1 },
    { key: 'colour', column: 'Colour', kind: 'text', maximum: 14 },
    { key: 'asp', column: 'ASP', kind: 'integer', group: 'MinSoilAspect' },
    { key: 'texture', column: 'Texture', kind: 'text', maximum: 4, group: 'SoilTexture' },
    { key: 'percentCoarseFragsGravel', column: 'PercentCoarseFragsGravel', kind: 'integer' },
    { key: 'percentCoarseFragsCobbles', column: 'PercentCoarseFragsCobbles', kind: 'integer' },
    { key: 'percentCoarseFragsStones', column: 'PercentCoarseFragsStones', kind: 'integer' },
    { key: 'percentCoarseFragsTotal', column: 'PercentCoarseFragsTotal', kind: 'integer' },
    { key: 'percentCoarseFragsShape', column: 'PercentCoarseFragsShape', kind: 'text', maximum: 1 },
    { key: 'rootsAbundance', column: 'RootsAbundance', kind: 'text', maximum: 6 },
    { key: 'rootsSize', column: 'RootsSize', kind: 'text', maximum: 6 },
    { key: 'mineralStructureClass', column: 'MineralStructureClass', kind: 'text', maximum: 3, group: 'MineralStructureClass' },
    { key: 'mineralStructureKind', column: 'MineralStructureKind', kind: 'text', maximum: 7, group: 'MineralStructureKind' },
    { key: 'mineralFormpH', column: 'MineralFormpH', kind: 'single' },
    { key: 'comments', column: 'Comments', kind: 'memo' },
  ],
};
export type SoilValue = string | number | null;
export interface SoilCell { raw: string; expected: SoilValue; value: SoilValue; error: string | null }
export type SoilDrafts = Record<string, { kind: SoilKind; id: number; cells: Record<string, SoilCell> }>;
export function soilField(kind: SoilKind, column: string): SoilField | undefined {
  return soilFields[kind].find(field => field.column.toLowerCase() === column.toLowerCase());
}
export function soilCell(drafts: SoilDrafts, kind: SoilKind, id: number, key: string): SoilCell | undefined {
  return drafts[`${kind}/${id}`]?.cells[key];
}
export function stageSoil(drafts: SoilDrafts, kind: SoilKind, id: number, field: SoilField, raw: string, stored: SoilValue): SoilDrafts {
  if (!Number.isInteger(id) || id < -2147483648 || id > 2147483647) throw new Error('Soil row needs an exact signed32 identity.');
  const previous = soilCell(drafts, kind, id, field.key);
  const expected = previous ? previous.expected : stored;
  let value: SoilValue = raw === '' ? null : raw;
  let error: string | null = null;
  if (field.kind === 'text' || field.kind === 'memo') {
    if (expected !== null && typeof expected !== 'string') throw new Error(`${field.column} requires nullable text.`);
    if (value !== null && value !== expected) {
      if (!wellFormedUTF16(value)) error = `${field.column} contains incomplete Unicode; the raw entry was not repaired.`;
      else if (field.maximum !== undefined && value.length > field.maximum) error = `${field.column} exceeds ${field.maximum} UTF-16 units; the raw entry was not truncated.`;
    }
  } else {
    if (expected !== null && typeof expected !== 'number') throw new Error(`${field.column} requires a nullable number.`);
    const parsed = finiteSingleValue(field.column, raw, expected);
    value = parsed.value;
    error = parsed.error;
    if (!error && value !== null && field.kind === 'integer' &&
        (!Number.isSafeInteger(value) || (value !== expected && (value < -32768 || value > 32767)))) {
      error = `${field.column} requires an exact Access Integer (-32768 to 32767) or NULL.`;
    }
  }
  const identity = `${kind}/${id}`;
  return { ...drafts, [identity]: { kind, id, cells: { ...drafts[identity]?.cells, [field.key]: { raw, expected, value, error } } } };
}
export function soilErrors(drafts: SoilDrafts): string[] {
  return Object.values(drafts).flatMap(row => Object.values(row.cells).flatMap(cell => cell.error ? [`${row.kind} row ${row.id}: ${cell.error}`] : []));
}
export function soilDirty(drafts: SoilDrafts): boolean {
  return Object.values(drafts).some(row => Object.values(row.cells).some(cell => cell.error !== null || cell.value !== cell.expected));
}
export function soilUpdates(drafts: SoilDrafts): SoilRecordUpdate[] {
  const errors = soilErrors(drafts);
  if (errors.length) throw new Error(errors[0]);
  return Object.values(drafts).flatMap(row => {
    const text: SoilRecordUpdate['text'] = {}, numbers: SoilRecordUpdate['numbers'] = {};
    for (const field of soilFields[row.kind]) {
      const cell = row.cells[field.key];
      if (!cell || cell.value === cell.expected) continue;
      if (field.kind === 'text' || field.kind === 'memo') {
        if ((cell.value !== null && typeof cell.value !== 'string') || (cell.expected !== null && typeof cell.expected !== 'string')) {
          throw new Error(`${field.column} requires nullable text.`);
        }
        text[field.key] = { value: cell.value, expected: cell.expected };
      } else {
        if ((cell.value !== null && typeof cell.value !== 'number') || (cell.expected !== null && typeof cell.expected !== 'number')) {
          throw new Error(`${field.column} requires a nullable number.`);
        }
        numbers[field.key] = { value: cell.value, expected: cell.expected };
      }
    }
    return Object.keys(text).length || Object.keys(numbers).length ? [{ kind: row.kind, id: row.id, text, numbers }] : [];
  });
}
