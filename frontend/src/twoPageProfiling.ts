import definition from '../../resources/two-page-plot-profiling-fields.json';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';
import type { TwoPageEnvironmentOwner } from './twoPageEnvironmentTable';
export { twoPageVegetationTableCellLabel as twoPageProfilingCellLabel } from './twoPageVegetationTable';

type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface TwoPageProfilingOwner extends TwoPageEnvironmentOwner {
  profileSource: { name: string; path: string };
}
export interface TwoPageProfilingView extends TwoPageProfilingOwner {
  envOriginal: Table;
  adminOriginal: Table;
  profileOriginal: Table;
  descriptionOriginal: Table;
  descriptionsPresent: boolean;
  offset: number;
  limit: number;
  totalRows: number;
}
export const twoPageProfilingFields = definition.fields;
const ordering = 'physical-rowid; not source Order execution';
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const yieldControl = () => new Promise<void>(resolve => setTimeout(resolve, 0));
if (definition.targetForm !== 'USysPlotProfiling' || definition.fields.length !== 9 ||
    new Set(definition.fields.map(field => field.column)).size !== 9) {
  throw new Error('Plot Profiling requires all nine unique source bindings.');
}
function column(table: Table, name: string): number {
  const indices = table.columns.flatMap((column, index) => column.name === name ? [index] : []);
  if (indices.length !== 1) throw new Error(`Plot Profiling lost its exact physical ${name} binding.`);
  return indices[0];
}
async function table(value: unknown, maximum: number, yieldRead: () => Promise<void>): Promise<Table> {
  if (!record(value) || !Array.isArray(value.rows) || value.rows.length > maximum) {
    throw new Error('Plot Profiling requires complete bounded original rows.');
  }
  const result = siviMetadataTableFromWire({ columns: value.columns, rows: [] });
  if (new Set(result.columns.map(column => column.name.toLowerCase())).size !== result.columns.length ||
      result.columns.some(column => !column.name || !wellFormedUTF16(column.name) || !wellFormedUTF16(column.declaredType))) {
    throw new Error('Plot Profiling requires unique original Unicode columns.');
  }
  const identities = new Set<string>();
  for (let offset = 0; offset < value.rows.length; offset += 200) {
    await yieldRead();
    const batch = siviMetadataTableFromWire({ columns: value.columns, rows: value.rows.slice(offset, offset + 200) });
    for (const row of batch.rows) {
      if (identities.has(row.rowId) || row.cells.some(cell => cell.text !== null && !wellFormedUTF16(cell.text)) ||
          result.rows.length > 0 && BigInt(result.rows[result.rows.length - 1].rowId) >= BigInt(row.rowId)) {
        throw new Error('Plot Profiling lost original Unicode or unique ascending physical row order.');
      }
      identities.add(row.rowId); result.rows.push(row);
    }
  }
  return result;
}
export async function twoPageProfilingFromWire(value: unknown, owner: TwoPageProfilingOwner,
  offset: number, limit: number, yieldRead: () => Promise<void> = yieldControl): Promise<TwoPageProfilingView> {
  if (!record(value) || ![owner.contextId, owner.project, owner.projectPath, owner.su, owner.plot,
      owner.profileSource.name, owner.profileSource.path].every(text => text && wellFormedUTF16(text)) ||
      owner.profileSource.name === 'None' || !wellFormedUTF16(owner.suPath) || owner.su !== 'None' && !owner.suPath ||
      !Number.isSafeInteger(offset) || offset < 0 || offset > 100000 ||
      !Number.isInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('Plot Profiling requires an immutable explicit source owner and bounded page request.');
  }
  const { profileSource, ...opening } = owner;
  if (Object.entries(opening).some(([name, expected]) => value[name] !== expected) ||
      !record(value.source) || value.source.name !== profileSource.name || value.source.path !== profileSource.path ||
      value.targetForm !== definition.targetForm || value.profileTable !== `${profileSource.name}_Profile` ||
      value.offset !== offset || value.limit !== limit || value.orderingPolicy !== ordering ||
      typeof value.totalRows !== 'number' || !Number.isSafeInteger(value.totalRows) || value.totalRows < 0 || value.totalRows > 100000 ||
      typeof value.descriptionsPresent !== 'boolean') {
    throw new Error('Plot Profiling has foreign owner/source, unsupported target or page bounds.');
  }
  const envOriginal = await table(value.envOriginal, 1, yieldRead);
  const adminOriginal = await table(value.adminOriginal, 1, yieldRead);
  const profileOriginal = await table(value.profileOriginal, limit, yieldRead);
  const descriptionOriginal = await table(value.descriptionOriginal, 100000, yieldRead);
  if (envOriginal.rows.length !== 1 || adminOriginal.rows.length !== 1 ||
      profileOriginal.rows.length !== Math.min(limit, Math.max(0, value.totalRows - offset))) {
    throw new Error('Plot Profiling requires one original physical parent pair and a complete page.');
  }
  const env = envOriginal.rows[0].cells[column(envOriginal, 'PlotNumber')];
  const admin = adminOriginal.rows[0].cells[column(adminOriginal, 'Plot')];
  if (env.storage !== 'text' || env.text !== owner.plot || admin.storage !== 'text' || admin.text !== owner.plot) {
    throw new Error('Plot Profiling lost literal opening parent identity.');
  }
  for (const field of definition.fields) column(profileOriginal, field.column);
  if (!value.descriptionsPresent) {
    if (descriptionOriginal.columns.length || descriptionOriginal.rows.length) {
      throw new Error('Absent Description metadata unexpectedly contains original data.');
    }
  } else {
    const identity = column(descriptionOriginal, 'table_name');
    column(descriptionOriginal, 'description');
    if (descriptionOriginal.rows.some(row => row.cells[identity].storage !== 'text' ||
        row.cells[identity].text !== value.profileTable)) {
      throw new Error('Plot Profiling Description witnesses belong to another literal table.');
    }
  }
  return { ...opening, profileSource: { ...profileSource }, envOriginal, adminOriginal, profileOriginal,
    descriptionOriginal, descriptionsPresent: value.descriptionsPresent, offset, limit, totalRows: value.totalRows };
}
export function twoPageProfilingRowFields(view: TwoPageProfilingView, row: Table['rows'][number]) {
  if (!view.profileOriginal.rows.includes(row)) throw new Error('Plot Profiling disclosure requires its reviewed physical row.');
  return definition.fields.map(field => ({ ...field, cell: row.cells[column(view.profileOriginal, field.column)] }));
}
