import targets from './twoPageEnvironmentTargets.json';
import { siviMetadataTableFromWire } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';
export { twoPageVegetationTableCellLabel as twoPageEnvironmentCellLabel } from './twoPageVegetationTable';

export interface TwoPageEnvironmentOwner {
  contextId: string;
  project: string;
  projectPath: string;
  su: string;
  suPath: string;
  plot: string;
}
type Table = ReturnType<typeof siviMetadataTableFromWire>;
export interface TwoPageEnvironmentView extends TwoPageEnvironmentOwner {
  target: typeof targets[number];
  rows: { envRowId: string; adminRowId: string }[];
  envOriginal: Table;
  adminOriginal: Table;
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
    throw new Error('Environment table requires complete unique literal physical columns.');
  }
  return result;
}
export function twoPageEnvironmentTableFromWire(value: unknown, owner: TwoPageEnvironmentOwner,
  offset: number, limit: number): TwoPageEnvironmentView {
  if (!record(value) || ![owner.contextId, owner.project, owner.projectPath, owner.su, owner.plot].every(text => text && wellFormedUTF16(text)) ||
      !wellFormedUTF16(owner.suPath) || owner.su !== 'None' && !owner.suPath ||
      !Number.isSafeInteger(offset) || offset < 0 || offset > 10000000000 ||
      !Number.isInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('Environment table requires an immutable owner and bounded page request.');
  }
  const target = targets.find(target => target.form === value.targetForm);
  if (!target || Object.entries(owner).some(([key, expected]) => value[key] !== expected) ||
      value.envTable !== `${owner.project}_Env` || value.adminTable !== `${owner.project}_Admin` ||
      value.offset !== offset || value.limit !== limit || typeof value.totalRows !== 'number' ||
      !Number.isSafeInteger(value.totalRows) || value.totalRows < offset || value.totalRows > 10000000000 ||
      value.generalLocaleVerified !== false ||
      value.orderingPolicy !== 'declared-target-order; controlled-English-ignore-case; descending-numeric-NULL-last; physical-row-ties' ||
      !Array.isArray(value.rows) || value.rows.length !== Math.min(limit, value.totalRows - offset)) {
    throw new Error('Environment table has foreign ownership, unsupported target/order or incomplete page.');
  }
  const envOriginal = table(value.envOriginal), adminOriginal = table(value.adminOriginal);
  const envPlot = envOriginal.columns.findIndex(column => column.name === 'PlotNumber');
  const adminPlot = adminOriginal.columns.findIndex(column => column.name === 'Plot');
  const depths = target.form === 'FS882-6x4' ?
    ['RootingDepth', 'SeepageDepth'].map(name => envOriginal.columns.findIndex(column => column.name === name)) : [];
  if (envPlot < 0 || adminPlot < 0 || depths.some(index => index < 0)) {
    throw new Error('Environment table originals lost a required join/order binding.');
  }
  const env = new Map(envOriginal.rows.map(row => [row.rowId, row]));
  const admin = new Map(adminOriginal.rows.map(row => [row.rowId, row]));
  const usedEnv = new Set<string>(), usedAdmin = new Set<string>(), pairs = new Set<string>();
  const rows = Array.from(value.rows).map(row => {
    if (!record(row) || typeof row.envRowId !== 'string' || typeof row.adminRowId !== 'string' ||
        !env.has(row.envRowId) || !admin.has(row.adminRowId)) {
      throw new Error('Environment table pair lost a complete physical witness.');
    }
    const left = env.get(row.envRowId)!, right = admin.get(row.adminRowId)!;
    if (left.cells[envPlot].storage !== 'text' || right.cells[adminPlot].storage !== 'text' ||
        depths.some(index => !['null', 'integer', 'real'].includes(left.cells[index].storage))) {
      throw new Error('Environment table join/order original storage is unsupported.');
    }
    const pair = JSON.stringify([row.envRowId, row.adminRowId]);
    if (pairs.has(pair)) throw new Error('Environment table repeats a physical pair.');
    pairs.add(pair); usedEnv.add(row.envRowId); usedAdmin.add(row.adminRowId);
    return { envRowId: row.envRowId, adminRowId: row.adminRowId };
  });
  if (usedEnv.size !== env.size || usedAdmin.size !== admin.size) {
    throw new Error('Environment table contains unrelated physical page originals.');
  }
  return { ...owner, target: structuredClone(target), rows, envOriginal, adminOriginal, offset, limit, totalRows: value.totalRows };
}

export function twoPageEnvironmentGroups(view: TwoPageEnvironmentView, row: TwoPageEnvironmentView['rows'][number]) {
  const env = view.envOriginal.rows.find(original => original.rowId === row.envRowId);
  const admin = view.adminOriginal.rows.find(original => original.rowId === row.adminRowId);
  if (!env || !admin) throw new Error('Environment source group lost its physical pair.');
  return [{ controlId: '', caption: 'Header / shared fields' }, ...view.target.pages].map(page => ({
    controlId: page.controlId, caption: page.caption ?? page.controlId,
    fields: view.target.fields.filter(field => (field.pageId ?? '') === page.controlId).map(field => {
      const table = field.role === 'Env' ? view.envOriginal : view.adminOriginal;
      const original = field.role === 'Env' ? env : admin;
      const matches = table.columns.map((column, index) => ({ column, index }))
        .filter(({ column }) => column.name.toLowerCase() === field.column.toLowerCase());
      return { controlId: field.controlId, label: field.caption || field.controlName || field.binding,
        role: field.role, column: field.column, cell: matches.length === 1 ? original.cells[matches[0].index] : null };
    }),
  })).filter(group => group.fields.length);
}
