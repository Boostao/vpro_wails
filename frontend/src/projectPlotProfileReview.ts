import type { ProjectMetadataCell, ProjectMetadataColumn, ProjectMetadataTable, ProjectPlotProfileReview, ProjectPlotProfileResult,
  ProjectPlotProfileFilterRequest, ProjectPlotProfileNavigation, PlotSummary, PlotPage, PlotProfileSourceInfo } from '../bindings/github.com/boostao/vpro-wails';
import { metadataCellText } from './projectMetadataEditor';

export const plotProfileFields = ['Order', 'Table', 'Field', 'Operator', 'Layer', 'Species', 'Criteria', 'Operation', 'PlotCount'] as const;

export interface ProfileReviewTable {
  columns: ProjectMetadataColumn[];
  rows: { rowId: string; cells: ProjectMetadataCell[] }[];
}
export interface ValidatedProjectPlotProfileReview {
  project: string;
  table: string;
  rules: ProfileReviewTable;
  descriptions: ProfileReviewTable;
  source: PlotProfileSourceInfo;
}

export function profileCellLabel(cell: ProjectMetadataCell): string {
  const keys = ['text', 'integer', 'real', 'blobHex'] as const;
  const populated = keys.filter(key => cell[key] !== null && cell[key] !== undefined);
  if (cell.storage === 'null' && populated.length === 0) return 'NULL';
  const key = cell.storage === 'text' ? 'text' : cell.storage === 'integer' ? 'integer'
    : cell.storage === 'real' ? 'real' : cell.storage === 'blob' ? 'blobHex' : null;
  if (key === null || populated.length !== 1 || populated[0] !== key ||
      key === 'text' && typeof cell.text !== 'string' ||
      key === 'integer' && (typeof cell.integer !== 'string' || !/^-?(0|[1-9]\d*)$/.test(cell.integer)) ||
      key === 'real' && (typeof cell.real !== 'number' || !Number.isFinite(cell.real)) ||
      key === 'blobHex' && (typeof cell.blobHex !== 'string' || !/^(?:[0-9a-f]{2})*$/.test(cell.blobHex))) {
    throw new Error('Profile review contains an inconsistent typed SQLite cell; reload without repairing it.');
  }
  const raw = metadataCellText(cell);
  return `${cell.storage}: ${key === 'text' ? JSON.stringify(raw) : raw}`;
}

export function validateTable(table: ProjectMetadataTable, required: readonly string[]): ProfileReviewTable {
  const { columns, rows } = table;
  if (!Array.isArray(columns) || !Array.isArray(rows) ||
      new Set(columns.map(column => column.name)).size !== columns.length ||
      required.some(name => !columns.some(column => column.name === name))) {
    throw new Error('Profile review requires its complete original physical schema.');
  }
  const seen = new Set<string>();
  const validatedRows = rows.map(row => {
    const cells = row.cells;
    if (!/^-?(0|[1-9]\d*)$/.test(row.rowId) || seen.has(row.rowId) ||
        !Array.isArray(cells) || cells.length !== columns.length) {
      throw new Error('Profile review requires distinct physical records with complete typed cells.');
    }
    seen.add(row.rowId);
    cells.forEach(profileCellLabel);
    return { rowId: row.rowId, cells };
  });
  return { columns, rows: validatedRows };
}

export function validateProjectProfileLump(table: ProjectMetadataTable): ProfileReviewTable {
  return validateTable(table, ['LumpCode', 'SppCode', 'Use']);
}

export interface ProfileRunResult {
  project: string;
  table: string;
  su: string;
  totalPlots: number;
  plotNumbers: string[];
  steps: { rowId: string; order: number; operation: string; plotCount: number; remaining: number }[];
}

export function validateProfileRunResult(result: ProjectPlotProfileResult, review: Pick<ValidatedProjectPlotProfileReview, 'project' | 'table' | 'rules'>): ProfileRunResult {
  const { plotNumbers, steps } = result;
  const ids = new Set(review.rules.rows.map(row => row.rowId));
  if (result.project !== review.project || result.table !== review.table || !result.su ||
      !Number.isInteger(result.totalPlots) || result.totalPlots < 0 ||
      !Array.isArray(plotNumbers) || plotNumbers.some(plot => typeof plot !== 'string' || !plot) ||
      new Set(plotNumbers).size !== plotNumbers.length || plotNumbers.length > result.totalPlots ||
      !Array.isArray(steps) || steps.length !== ids.size || new Set(steps.map(step => step.rowId)).size !== ids.size ||
      steps.some(step => !ids.has(step.rowId) || !Number.isInteger(step.order) || step.order < -32768 || step.order > 32767 ||
        !['Add plots', 'Subtract plots', 'Common plots'].includes(step.operation) ||
        !Number.isInteger(step.plotCount) || step.plotCount < 0 || step.plotCount > result.totalPlots ||
        !Number.isInteger(step.remaining) || step.remaining < 0 || step.remaining > result.totalPlots) ||
      steps.some((step, index) => index > 0 && step.order <= steps[index - 1].order) ||
      steps.length === 0 || steps[steps.length - 1].remaining !== plotNumbers.length) {
    throw new Error('Profile run returned an inconsistent scoped result; no filter was applied.');
  }
  return { project: result.project, table: result.table, su: result.su, totalPlots: result.totalPlots, plotNumbers, steps };
}

export function validateProjectPlotProfileReview(review: ProjectPlotProfileReview): ValidatedProjectPlotProfileReview {
  const source = validatePlotProfileSource(review.source);
  if (!review.project || !source.available || review.table !== source.table) {
    throw new Error('Profile review must identify the explicitly selected source table.');
  }
  const descriptions = review.descriptions;
  const absent = Array.isArray(descriptions?.columns) && descriptions.columns.length === 0 &&
    Array.isArray(descriptions.rows) && descriptions.rows.length === 0;
  return { project: review.project, table: review.table,
    rules: validateTable(review.rules, plotProfileFields),
    descriptions: absent ? { columns: [], rows: [] } : validateTable(descriptions, ['table_name', 'description']), source };
}

export function validatePlotProfileSource(info: PlotProfileSourceInfo): PlotProfileSourceInfo {
  if (!info || !info.source || typeof info.source.name !== 'string' || !info.source.name ||
      typeof info.source.path !== 'string' || typeof info.table !== 'string' ||
      typeof info.available !== 'boolean' || typeof info.writable !== 'boolean' || typeof info.reason !== 'string' ||
      (info.source.name === 'None' ? info.source.path !== '' || info.table !== '' || info.available || info.writable :
        !info.source.path || info.table !== `${info.source.name}_Profile` || info.writable && !info.available) ||
      !info.available && info.source.name !== 'None' && !info.reason) {
    throw new Error('Profile source requires an exact physical table/path and explicit availability/write authority.');
  }
  return info;
}

export interface ProfileNavigation {
    contextId: string;
    proposal: ProjectPlotProfileFilterRequest;
    result: ProfileRunResult;
    plots: PlotSummary[];
  }

export function profileFilterUnavailable(result: ProfileRunResult): string | null {
    return result.plotNumbers.length === 0 ? 'Profile resulted in zero plots; current navigation filter remains unchanged.' :
      result.plotNumbers.length > 200 ? 'Profile exceeds 200 plots. Current navigation filter remains unchanged; Save as SU requires its separate opt-in and explicit review.' : null;
  }

export function validateProfileNavigation(navigation: ProjectPlotProfileNavigation, proposal: ProjectPlotProfileFilterRequest,
      contextId: string): ProfileNavigation {
    const source = { project: proposal.preview.project, table: proposal.preview.table,
      rules: validateTable(proposal.input.originalRules, plotProfileFields) };
    if (!contextId || navigation.contextId !== contextId || !source.table.endsWith('_Profile') || source.table === '_Profile') {
      throw new Error('Profile navigation belongs to another context/project; current filter remains unchanged.');
    }
    const expected = validateProfileRunResult(proposal.preview, source);
    const result = validateProfileRunResult(navigation.result, source);
    if (profileFilterUnavailable(result) || result.su !== expected.su || result.totalPlots !== expected.totalPlots ||
        JSON.stringify(result.plotNumbers) !== JSON.stringify(expected.plotNumbers) ||
        result.steps.some((step, index) => {
          const original = expected.steps[index];
          return step.rowId !== original.rowId || step.order !== original.order || step.operation !== original.operation ||
            step.plotCount !== original.plotCount || step.remaining !== original.remaining;
        })) throw new Error('Profile navigation differs from the reviewed execution; rerun before applying.');
    const plots = navigation.plots;
    if (!Array.isArray(plots) || plots.length !== result.plotNumbers.length ||
        plots.some((plot, index) => plot.plotNumber !== result.plotNumbers[index] ||
          (['fieldNumber', 'plotRepresenting', 'zone', 'subZone', 'siteSeries'] as const).some(name => {
            const value = plot[name];
            return value !== null && typeof value !== 'string';
          }))) throw new Error('Profile navigation requires exactly one complete scoped summary per literal plot identity.');
    return { contextId, proposal, result, plots };
  }

export function profileNavigationPage(navigation: ProfileNavigation, contextId: string, offset: number, limit: number): PlotPage {
    if (navigation.contextId !== contextId || !Number.isInteger(offset) || offset < 0 ||
        !Number.isInteger(limit) || limit < 1 || limit > 200) throw new Error('Profile navigation page is stale or out of range.');
    return { total: navigation.plots.length, plots: navigation.plots.slice(offset, offset + limit) };
  }
