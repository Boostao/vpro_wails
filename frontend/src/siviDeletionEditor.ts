import type { ProjectMetadataColumn } from '../bindings/github.com/boostao/vpro-wails';
import defaults from '../../resources/sivi-new-row-defaults.json';
import { wellFormedUTF16 } from './qualityEditor';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { validSIVIRequestId } from './siviRequestId';
import type { SIVICreationForm } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVIDeletionOriginal extends SIVISpeciesOwner {
  form: SIVICreationForm;
  columns: ProjectMetadataColumn[];
  original: ReturnType<typeof siviMetadataTableFromWire>['rows'][number];
}
export interface SIVIDeletionRequest extends SIVIDeletionOriginal { requestId: string }
export interface SIVIDeletionTarget {
  form: SIVICreationForm;
  rowId: string;
  label: string;
  unavailable: string | null;
}
const names = new Set([...defaults.initialNullColumns, defaults.initialBoolean.column, defaults.identity.column]);
const forms = new Set(['SubVegA-SIVI', 'SubVegA-SIVI_BC', 'SubVegC-SIVI', 'SubVegD-SIVI']);
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export function siviDeletionOriginalFromWire(wire: unknown, owner: SIVISpeciesOwner,
  form: SIVICreationForm, rowId: string): SIVIDeletionOriginal {
  const diagnostic = 'SIVI deletion requires the exact source form, owner and complete44-cell physical original; no row was inferred.';
  if (!record(wire) || !forms.has(form) || wire.form !== form ||
      wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
      [owner.contextId, owner.project, owner.plot].some(value => !value || !wellFormedUTF16(value) || value.includes('\0')) ||
      owner.plot.length > 7 || !record(wire.original) || wire.original.rowId !== rowId) {
    throw new Error(diagnostic);
  }
  const table = siviMetadataTableFromWire({ columns: wire.columns, rows: [wire.original] });
  if (table.columns.length !== names.size || new Set(table.columns.map(column => column.name)).size !== names.size ||
      table.columns.some(column => !names.has(column.name) || !wellFormedUTF16(column.declaredType) ||
        Object.keys(column).some(key => !['name', 'declaredType'].includes(key))) ||
      table.rows[0].cells.some(cell => Object.keys(cell).some(key => !['storage', 'text', 'integer', 'real', 'blobHex'].includes(key)) ||
        cell.storage === 'text' && !wellFormedUTF16(cell.text ?? ''))) throw new Error(diagnostic);
  const original = table.rows[0];
  const parent = original.cells[table.columns.findIndex(column => column.name === 'PlotNumber')];
  if (parent.storage !== 'text' || parent.text !== owner.plot) throw new Error(diagnostic);
  const query = defaults.membershipQueries.find(query => query.name ===
    (form.startsWith('SubVegA-') ? 'USysVegA' : form === 'SubVegC-SIVI' ? 'USysVegC' : 'USysVegD'));
  if (!query || !query.anyNonNull.some(name => original.cells[table.columns.findIndex(column => column.name === name)].storage !== 'null')) {
    throw new Error('SIVI deletion target no longer belongs to the planned source view; zero and NULL remain distinct.');
  }
  return { contextId: owner.contextId, project: owner.project, plot: owner.plot, form, columns: table.columns, original };
}

export function siviDeletionUnavailable(original: SIVIDeletionOriginal): string | null {
  const cell = original.original.cells[original.columns.findIndex(column => column.name === 'ID')];
  if (cell.storage !== 'integer' || cell.integer === null ||
      BigInt(cell.integer) < -2147483648n || BigInt(cell.integer) > 2147483647n) {
    return 'Deletion with a NULL or non-signed32 application identity is unavailable; physical row identity does not repair it.';
  }
  if (original.original.cells.some(cell => cell.storage === 'blob')) return 'Deletion cannot safely produce source audits for a BLOB historical value.';
  if (original.columns.some((column, index) =>
    (column.name === 'Flag' || /^(BOOLEAN|BOOL|BIT)$/i.test(column.declaredType)) &&
    !['null', 'integer'].includes(original.original.cells[index].storage))) {
    return 'Deletion cannot safely produce BOOLEAN source audits from non-integer historical storage.';
  }
  return null;
}

export function siviDeletionRequest(original: SIVIDeletionOriginal, owner: SIVISpeciesOwner,
  requestId: string, confirmed: boolean): SIVIDeletionRequest {
  if (!validSIVIRequestId(requestId)) throw new Error('SIVI deletion requires one stable request identity; no retry was inferred.');
  const approved = siviDeletionOriginalFromWire(original, owner, original.form, original.original.rowId);
  const unavailable = siviDeletionUnavailable(approved);
  if (unavailable) throw new Error(unavailable);
  if (confirmed !== true) throw new Error('Confirm deletion of the exact reviewed physical row and all44 historical cells.');
  return { requestId, ...approved };
}

export function siviDeletionTargetsFromWire(wire: unknown, owner: SIVISpeciesOwner): SIVIDeletionTarget[] {
  if (!record(wire) || Object.keys(wire).some(key => !['contextId', 'project', 'plot', 'columns', 'rows'].includes(key)) ||
    wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plot !== owner.plot ||
    [owner.contextId, owner.project, owner.plot].some(value => !value || !wellFormedUTF16(value) || value.includes('\0')) ||
    owner.plot.length > 7) {
    throw new Error('Deletion source rows belong to another owned context/project/plot; no targets were inferred.');
  }
  const table = siviMetadataTableFromWire(wire);
  if (table.columns.length !== names.size || new Set(table.columns.map(column => column.name)).size !== names.size ||
    table.columns.some(column => !names.has(column.name) || !wellFormedUTF16(column.declaredType) ||
      Object.keys(column).some(key => !['name', 'declaredType'].includes(key)))) throw new Error('Deletion source rows require the complete44-column schema.');
  const paths: SIVICreationForm[] = ['SubVegA-SIVI_BC', 'SubVegA-SIVI', 'SubVegC-SIVI', 'SubVegD-SIVI'];
  const targets: SIVIDeletionTarget[] = [];
  for (const row of table.rows) {
    const parent = row.cells[table.columns.findIndex(column => column.name === 'PlotNumber')];
    if (parent.storage !== 'text' || parent.text !== owner.plot) throw new Error('Deletion target table contains a foreign physical parent.');
    for (const form of paths) {
      const query = defaults.membershipQueries.find(query => query.name ===
        (form.startsWith('SubVegA-') ? 'USysVegA' : form === 'SubVegC-SIVI' ? 'USysVegC' : 'USysVegD'));
      if (!query || !query.anyNonNull.some(name => row.cells[table.columns.findIndex(column => column.name === name)].storage !== 'null')) continue;
      const approved = siviDeletionOriginalFromWire({ ...owner, form, columns: table.columns, original: row }, owner, form, row.rowId);
      const species = row.cells[table.columns.findIndex(column => column.name === 'Species')];
      const id = row.cells[table.columns.findIndex(column => column.name === 'ID')];
      const literal = (cell: typeof species) => cell.storage === 'null' ? 'NULL'
        : cell.storage === 'text' ? cell.text === '' ? '(empty text)' : cell.text
        : cell.storage === 'integer' ? cell.integer : cell.storage === 'real' ? String(cell.real) : `hex:${cell.blobHex}`;
      targets.push({ form, rowId: row.rowId,
        label: `${form}; Species ${literal(species)}; application ID ${literal(id)}; physical row ${row.rowId}`,
        unavailable: siviDeletionUnavailable(approved) });
    }
  }
  return targets;
}
