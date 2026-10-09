import type { ProjectMetadataCell, ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import type { TwoPageVegetationOwner } from './twoPageVegetationWire';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';

export interface VegetationReferenceTable {
  columns: ProjectMetadataColumn[];
  rows: { rowId: string; cells: ProjectMetadataCell[] }[];
}
export interface VegetationReferences extends TwoPageVegetationOwner {
  source: string;
  master: VegetationReferenceTable;
  personal: VegetationReferenceTable;
}
const source = 'configured-owned-imported-SQLite VLists.USysAllSpecs/VUser.USysUserSpp; physical definitions, not source union membership';
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const keys = (value: Record<string, unknown>, expected: readonly string[]): boolean =>
  Object.keys(value).length === expected.length && expected.every(key => Object.hasOwn(value, key));
const yieldToClient = () => new Promise<void>(resolve => setTimeout(resolve, 0));

export async function vegetationReferencesFromWire(wire: unknown, owner: TwoPageVegetationOwner,
  validationYield: () => PromiseLike<void> = yieldToClient): Promise<VegetationReferences> {
  if (!record(wire) || !keys(wire, ['contextId', 'project', 'plot', 'form', 'source', 'master', 'personal']) ||
      ![owner.contextId, owner.project, owner.plot].every(value =>
        typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0')) ||
      !['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL'].includes(owner.form) ||
      wire.contextId !== owner.contextId || wire.project !== owner.project ||
      wire.plot !== owner.plot || wire.form !== owner.form || wire.source !== source) {
    throw new Error('Vegetation Species definitions require the exact context/project/parent/source-form owner and physical source.');
  }
  const tables: VegetationReferenceTable[] = [];
  for (const [role, names, minimum, maximum] of [
    ['master', ['Code', 'ScientificName', 'Lifeform', 'EnglishName', 'Codetype', 'OldCode'], -2147483648n, 2147483647n],
    ['personal', ['Code', 'ScientificName', 'LifeForm', 'EnglishName', 'Codetype'], -32768n, 32767n],
  ] as const) {
    const table = wire[role];
    if (!record(table) || !keys(table, ['columns', 'rows']) || !Array.isArray(table.columns) ||
        table.columns.length !== names.length || !Array.isArray(table.rows)) {
      throw new Error(`Vegetation ${role} Species definitions lack their complete projected physical schema.`);
    }
    const columns: ProjectMetadataColumn[] = [];
    for (const [index, column] of Array.from(table.columns).entries()) {
      if (!record(column) || !keys(column, ['name', 'declaredType']) ||
          typeof column.name !== 'string' || !/^[A-Za-z]+$/.test(column.name) ||
          column.name.toLowerCase() !== names[index].toLowerCase() ||
          typeof column.declaredType !== 'string' || !wellFormedUTF16(column.declaredType)) {
        throw new Error(`Vegetation ${role} Species definitions require original column names and declaredType metadata.`);
      }
      columns.push({ name: column.name, declaredType: column.declaredType });
    }
    const rows: VegetationReferenceTable['rows'] = [];
    const identities = new Set<string>();
    for (const row of Array.from(table.rows)) {
      if (!record(row) || !keys(row, ['rowId', 'cells']) || typeof row.rowId !== 'string' ||
          !exactSigned64(row.rowId) || identities.has(row.rowId) ||
          !Array.isArray(row.cells) || row.cells.length !== names.length) {
        throw new Error(`Vegetation ${role} Species physical identity or complete tagged cells are missing or repeated.`);
      }
      identities.add(row.rowId);
      const cells: ProjectMetadataCell[] = [];
      for (const [index, cell] of Array.from(row.cells).entries()) {
        if (!record(cell) || !keys(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
            !completeMetadataCellFromWire(cell)) {
          throw new Error(`Vegetation ${role} Species tagged storage is malformed.`);
        }
        if (index === 2) {
          if (cell.storage !== 'null' && (cell.storage !== 'integer' ||
              BigInt(cell.integer!) < minimum || BigInt(cell.integer!) > maximum)) {
            throw new Error(`Vegetation ${role} Lifeform requires exact nullable source ${role === 'master' ? 'signed32 LONG' : 'signed16 INTEGER'} storage.`);
          }
        } else if (cell.storage !== 'null' && cell.storage !== 'text') {
          throw new Error(`Vegetation ${role} Species text metadata cannot be coerced from another storage type.`);
        }
        cells.push({ storage: cell.storage, text: cell.text, integer: cell.integer, real: cell.real, blobHex: cell.blobHex });
      }
      rows.push({ rowId: row.rowId, cells });
      if (rows.length % 128 === 0) await validationYield();
    }
    tables.push({ columns, rows });
  }
  await validationYield();
  return { ...owner, source, master: tables[0], personal: tables[1] };
}
