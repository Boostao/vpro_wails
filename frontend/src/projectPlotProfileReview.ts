import type { ProjectMetadataCell, ProjectMetadataColumn, ProjectMetadataTable, ProjectPlotProfileReview, ProjectPlotProfileResult } from '../bindings/github.com/boostao/vpro-wails';
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

function validateTable(table: ProjectMetadataTable, required: readonly string[]): ProfileReviewTable {
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

export function validateProfileRunResult(result: ProjectPlotProfileResult, review: ValidatedProjectPlotProfileReview): ProfileRunResult {
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
  if (!review.project || review.table !== `${review.project}_Profile`) {
    throw new Error('Profile review must identify the selected project-owned source table.');
  }
  return { project: review.project, table: review.table,
    rules: validateTable(review.rules, plotProfileFields),
    descriptions: validateTable(review.descriptions, ['table_name', 'description']) };
}
