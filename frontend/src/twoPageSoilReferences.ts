import type { ProjectMetadataCell, ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import { completeMetadataCellFromWire } from './siviParentTransport';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { ReadRequests, type CancellableRead } from './readRequests';
import type { TwoPageSoilOwner, TwoPageSoilForm } from './twoPageSoilEditor';

export interface SoilReferenceTable {
  columns: ProjectMetadataColumn[];
  rows: { rowId: string; cells: ProjectMetadataCell[] }[];
}
export interface SoilReferenceField {
  column: string;
  listName: string;
  definitions: SoilReferenceTable;
}
export interface SoilReferences extends TwoPageSoilOwner {
  source: string;
  fields: SoilReferenceField[];
}
export const soilReferenceBindings: Record<TwoPageSoilForm, readonly (readonly [string, string])[]> = {
  SoilHumusXL: [
    ['Horizon', 'HumusHorizon'], ['HumusStructureKind', 'HumusStructureKind'],
    ['HumusStructureDegree', 'HumusStructureDegree'], ['vonPost', 'vonPost'],
    ['MycelAbundance', 'MycelAbundance'], ['FecalAbundance', 'MycelAbundance'],
  ],
  SoilMineralXL: [
    ['ASP', 'MinSoilAspect'], ['Texture', 'SoilTexture'],
    ['MineralStructureClass', 'MineralStructureClass'], ['MineralStructureKind', 'MineralStructureKind'],
  ],
};
const source = 'configured-owned-imported-SQLite VLists.USysTableOfLists; physical rowid; ItemOrder then rowid';
const columns = [
  ['ListName', 'TEXT'], ['ListFilter', 'TEXT'], ['ItemOrder', 'REAL'], ['Item', 'TEXT'],
  ['ItemDescription', 'TEXT'], ['FieldUsedIn', 'TEXT'], ['ValidateLoops', 'TEXT'],
  ['Validate', 'BOOLEAN'], ['Note', 'TEXT'], ['Flag', 'BOOLEAN'],
] as const;
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const keys = (value: Record<string, unknown>, expected: readonly string[]): boolean =>
  Object.keys(value).length === expected.length && expected.every(key => Object.hasOwn(value, key));

export function soilReferencesFromWire(wire: unknown, owner: TwoPageSoilOwner): SoilReferences {
  const bindings = soilReferenceBindings[owner.form];
  if (!record(wire) || !keys(wire, ['contextId', 'project', 'plot', 'form', 'source', 'fields']) ||
      !bindings || ![owner.contextId, owner.project, owner.plot].every(value =>
        typeof value === 'string' && value !== '' && wellFormedUTF16(value)) ||
      wire.contextId !== owner.contextId || wire.project !== owner.project ||
      wire.plot !== owner.plot || wire.form !== owner.form || wire.source !== source ||
      !Array.isArray(wire.fields) || wire.fields.length !== bindings.length) {
    throw new Error('Soil references belong to another owner or lack the complete configured source bindings.');
  }
  const fields: SoilReferenceField[] = [];
  for (const [index, field] of Array.from(wire.fields).entries()) {
    const binding = bindings[index];
    if (!record(field) || !keys(field, ['column', 'listName', 'definitions']) ||
        field.column !== binding[0] || field.listName !== binding[1] || !record(field.definitions) ||
        !keys(field.definitions, ['columns', 'rows']) || !Array.isArray(field.definitions.columns) ||
        field.definitions.columns.length !== columns.length || !Array.isArray(field.definitions.rows) ||
        field.definitions.rows.length === 0) {
      throw new Error('Soil source field mapping or complete physical definitions were not returned.');
    }
    const schema: ProjectMetadataColumn[] = [];
    for (const [columnIndex, column] of Array.from(field.definitions.columns).entries()) {
      if (!record(column) || !keys(column, ['name', 'declaredType']) ||
          typeof column.name !== 'string' || typeof column.declaredType !== 'string' ||
          column.name !== columns[columnIndex][0] || column.declaredType !== columns[columnIndex][1]) {
        throw new Error('Soil references require the exact original ten-column declaredType schema.');
      }
      schema.push({ name: column.name, declaredType: column.declaredType });
    }
    const rows: SoilReferenceTable['rows'] = [];
    for (const row of Array.from(field.definitions.rows)) {
      if (!record(row) || !keys(row, ['rowId', 'cells']) || typeof row.rowId !== 'string' ||
          !exactSigned64(row.rowId) || !Array.isArray(row.cells) || row.cells.length !== columns.length) {
        throw new Error('Soil reference physical row identity or complete tagged storage is missing.');
      }
      const cells: ProjectMetadataCell[] = [];
      for (const cell of Array.from(row.cells)) {
        if (!record(cell) || !keys(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
            !completeMetadataCellFromWire(cell)) throw new Error('Soil reference tagged storage is malformed.');
        cells.push({ storage: cell.storage, text: cell.text, integer: cell.integer, real: cell.real, blobHex: cell.blobHex });
      }
      if (cells[0].storage !== 'text' || cells[0].text !== field.listName) {
        throw new Error('Soil reference physical definition belongs to another source list.');
      }
      rows.push({ rowId: row.rowId, cells });
    }
    if (new Set(rows.map(row => row.rowId)).size !== rows.length) throw new Error('Soil reference physical identities repeat.');
    fields.push({ column: field.column, listName: field.listName, definitions: { columns: schema, rows } });
  }
  return { ...owner, source, fields };
}

export interface SoilReferenceView { busy: boolean; data: SoilReferences | null; error: string | null }
export class SoilReferenceLookup {
  private requests = new ReadRequests();
  private generation = 0;
  private disposed = false;
  private state: SoilReferenceView = { busy: false, data: null, error: null };
  private owner: TwoPageSoilOwner;
  constructor(owner: TwoPageSoilOwner, private fetch: (owner: TwoPageSoilOwner) => CancellableRead<unknown>,
    private changed: (view: SoilReferenceView) => void) {
    this.owner = { ...owner };
  }
  snapshot(): SoilReferenceView { return { ...this.state }; }
  private publish(state: SoilReferenceView): void { this.state = state; this.changed(this.snapshot()); }
  async refresh(): Promise<boolean> {
    if (this.disposed) throw new Error('Disposed soil reference lookup cannot be refreshed.');
    const generation = ++this.generation;
    this.requests.cancelAll();
    this.publish({ busy: true, data: null, error: null });
    try {
      const wire = await this.requests.track(this.fetch(this.owner));
      if (this.disposed || generation !== this.generation) return false;
      const data = soilReferencesFromWire(wire, this.owner);
      if (this.disposed || generation !== this.generation) return false;
      this.publish({ busy: false, data, error: null });
      return true;
    } catch (cause) {
      if (!this.disposed && generation === this.generation) {
        this.publish({ busy: false, data: null, error: `Source soil choices unavailable: ${String(cause)}` });
      }
      return false;
    }
  }
  cancel(): void {
    ++this.generation;
    this.requests.cancelAll();
    if (!this.disposed) this.publish({ busy: false, data: null, error: 'Source soil choice read cancelled. Retry to load current definitions.' });
  }
  dispose(): void { this.disposed = true; ++this.generation; this.requests.cancelAll(); }
}
