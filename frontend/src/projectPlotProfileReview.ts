import type { ProjectMetadataCell, ProjectMetadataColumn, ProjectMetadataTable, ProjectPlotProfileReview } from '../bindings/github.com/boostao/vpro-wails';
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

export function validateProjectPlotProfileReview(review: ProjectPlotProfileReview): ValidatedProjectPlotProfileReview {
  if (!review.project || review.table !== `${review.project}_Profile`) {
    throw new Error('Profile review must identify the selected project-owned source table.');
  }
  return { project: review.project, table: review.table,
    rules: validateTable(review.rules, plotProfileFields),
    descriptions: validateTable(review.descriptions, ['table_name', 'description']) };
}
