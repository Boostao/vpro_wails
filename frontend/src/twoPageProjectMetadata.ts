import definition from '../../resources/project-metadata-layout.json';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';
import { metadataGroups, metadataLabel } from './projectMetadataPresentation';
import type { TwoPageEnvironmentOwner } from './twoPageEnvironmentTable';
export { twoPageVegetationTableCellLabel as twoPageProjectMetadataCellLabel } from './twoPageVegetationTable';

export type TwoPageProjectMetadataOwner = TwoPageEnvironmentOwner;
type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface TwoPageProjectMetadataView extends TwoPageProjectMetadataOwner {
  projectId: string;
  envOriginal: Table;
  adminOriginal: Table;
  metadataOriginal: Table;
  offset: number;
  limit: number;
  totalRows: number;
}
const matchingPolicy = 'DAO-General1033-controlled-ASCII-alphanumeric-space; ignore-case-terminal-U0020; physical-row-order';
const source = definition.forms.find(form => form.name === 'frmProjectMetaData');
const groups = [{ title: 'Identity and source stamps', names: ['ProjectID', 'DateLastEdited', 'AllSpecs', 'TableOfLists'] }, ...metadataGroups];
if (!source || source.recordSource !== 'UsysMetadata' || source.fields.filter(field => field.binding).length !== 74 ||
    new Set(groups.flatMap(group => group.names)).size !== 74) {
  throw new Error('Project metadata review requires all74 source bindings and existing presentation groups.');
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
function key(value: string): string {
  if (/[^A-Za-z0-9 ]/.test(value)) throw new Error('Metadata ProjectID is outside the verified comparison domain.');
  return value.replace(/ +$/, '').toUpperCase();
}
function table(value: unknown): Table {
  const result = siviMetadataTableFromWire(value);
  if (!result.columns.length || new Set(result.columns.map(column => column.name)).size !== result.columns.length ||
      result.columns.some(column => !column.name || !wellFormedUTF16(column.name) || !wellFormedUTF16(column.declaredType)) ||
      result.rows.some(row => row.cells.some(cell => cell.text !== null && !wellFormedUTF16(cell.text)))) {
    throw new Error('Metadata review requires complete unique physical columns and original Unicode storage.');
  }
  return result;
}
function column(table: Table, name: string): number {
  const indices = table.columns.flatMap((column, index) => column.name.toLowerCase() === name.toLowerCase() ? [index] : []);
  if (indices.length !== 1) throw new Error(`Metadata review lost an unambiguous physical ${name} binding.`);
  return indices[0];
}
export function twoPageProjectMetadataFromWire(value: unknown, owner: TwoPageProjectMetadataOwner,
  offset: number, limit: number): TwoPageProjectMetadataView {
  if (!record(value) || ![owner.contextId, owner.project, owner.projectPath, owner.su, owner.plot].every(text => text && wellFormedUTF16(text)) ||
      !wellFormedUTF16(owner.suPath) || owner.su !== 'None' && !owner.suPath ||
      !Number.isSafeInteger(offset) || offset < 0 || offset > 100000 ||
      !Number.isInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('Metadata review requires an immutable owner and bounded page request.');
  }
  if (Object.entries(owner).some(([name, expected]) => value[name] !== expected) ||
      typeof value.projectId !== 'string' || value.targetForm !== 'frmProjectMetaData' ||
      value.metadataTable !== `${owner.project}_Metadata` || value.offset !== offset || value.limit !== limit ||
      typeof value.totalRows !== 'number' || !Number.isSafeInteger(value.totalRows) ||
      value.totalRows < 0 || value.totalRows > 100000 || value.matchingPolicy !== matchingPolicy ||
      value.generalLocaleVerified !== false) {
    throw new Error('Metadata review has foreign ownership, unsupported source/matching or page bounds.');
  }
  const envOriginal = table(value.envOriginal), adminOriginal = table(value.adminOriginal),
    metadataOriginal = table(value.metadataOriginal);
  const envPlot = column(envOriginal, 'PlotNumber'), adminPlot = column(adminOriginal, 'Plot'),
    envProject = column(envOriginal, 'ProjectID'), metadataProject = column(metadataOriginal, 'ProjectID');
  if (envOriginal.rows.length !== 1 || adminOriginal.rows.length !== 1) {
    throw new Error('Metadata review requires one complete physical opening parent pair.');
  }
  const env = envOriginal.rows[0], admin = adminOriginal.rows[0];
  if (env.cells[envPlot].storage !== 'text' || env.cells[envPlot].text !== owner.plot ||
      admin.cells[adminPlot].storage !== 'text' || admin.cells[adminPlot].text !== owner.plot ||
      env.cells[envProject].storage !== 'text' || env.cells[envProject].text !== value.projectId ||
      metadataOriginal.rows.length !== Math.min(limit, Math.max(0, value.totalRows - offset))) {
    throw new Error('Metadata review lost literal parent ownership or a complete physical page.');
  }
  const projectKey = key(value.projectId);
  for (const [index, row] of metadataOriginal.rows.entries()) {
    const project = row.cells[metadataProject];
    if (project.storage !== 'text' || project.text === null || key(project.text) !== projectKey ||
        index > 0 && BigInt(metadataOriginal.rows[index - 1].rowId) >= BigInt(row.rowId)) {
      throw new Error('Metadata page has unrelated ProjectID storage or unsupported physical ordering.');
    }
  }
  return { ...owner, projectId: value.projectId, envOriginal, adminOriginal, metadataOriginal,
    offset, limit, totalRows: value.totalRows };
}
export function twoPageProjectMetadataGroups(view: TwoPageProjectMetadataView, row: Table['rows'][number]) {
  if (!view.metadataOriginal.rows.includes(row)) throw new Error('Metadata disclosure requires its reviewed physical row.');
  return groups.map(group => ({ title: group.title, fields: group.names.map(name => {
    const fields = source!.fields.filter(field => field.column?.toLowerCase() === name.toLowerCase());
    if (fields.length !== 1 || !fields[0].controlId) throw new Error(`Metadata ${name} lost its source binding.`);
    const indices = view.metadataOriginal.columns.flatMap((column, index) => column.name.toLowerCase() === name.toLowerCase() ? [index] : []);
    return { controlId: fields[0].controlId, name, label: metadataLabel(name),
      cell: indices.length === 1 ? row.cells[indices[0]] : null };
  }) }));
}
