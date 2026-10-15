import { siviMetadataTableFromWire } from './siviParentTransport';
import { wellFormedUTF16 } from './qualityEditor';
export { twoPageVegetationTableCellLabel as userLogCellLabel } from './twoPageVegetationTable';

export const userLogFields = [
  { name: 'User', caption: 'User' }, { name: 'InTime', caption: 'Login' },
  { name: 'OutTime', caption: 'OutTime' }, { name: 'LocalMachine', caption: 'Local Machine' },
] as const;
export interface UserLogView {
  contextId: string;
  sourcePath: string;
  mode: number;
  offset: number;
  limit: number;
  total: number;
  table: ReturnType<typeof siviMetadataTableFromWire>;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

export function userLogFromWire(value: unknown, contextId: string, mode: number, offset: number, limit: number): UserLogView {
  if (!contextId || !wellFormedUTF16(contextId) || mode !== 1 && mode !== 2 ||
      !Number.isSafeInteger(offset) || offset < 0 || !Number.isInteger(limit) || limit < 1 || limit > 200) {
    throw new Error('Saved user log review requires the current context, source mode and a bounded page.');
  }
  if (!record(value) || value.contextId !== contextId || value.mode !== mode || value.offset !== offset || value.limit !== limit ||
      typeof value.sourcePath !== 'string' || !value.sourcePath || value.sourcePath.includes('\0') || !wellFormedUTF16(value.sourcePath) ||
      typeof value.total !== 'number' || !Number.isInteger(value.total) || value.total < 0 || value.total > 10000) {
    throw new Error('Saved user log has foreign ownership, invalid provenance or unsupported page bounds.');
  }
  const table = siviMetadataTableFromWire(value.table);
  if (table.columns.length !== 4 || new Set(table.columns.map(column => column.name)).size !== 4 ||
      userLogFields.some(field => !table.columns.some(column => column.name === field.name)) ||
      table.columns.some(column => !wellFormedUTF16(column.declaredType)) ||
      table.rows.length !== Math.min(limit, Math.max(0, value.total - offset))) {
    throw new Error('Saved user log lost its four source bindings or complete physical page.');
  }
  const outTime = table.columns.findIndex(column => column.name === 'OutTime');
  for (const [index, row] of table.rows.entries()) {
    if (row.cells.some(cell => cell.text !== null && !wellFormedUTF16(cell.text)) ||
        mode === 2 && row.cells[outTime].storage !== 'null' ||
        index > 0 && BigInt(table.rows[index - 1].rowId) >= BigInt(row.rowId)) {
      throw new Error('Saved user log lost original Unicode, NULL-only Current filtering or technical row order.');
    }
  }
  return { contextId, sourcePath: value.sourcePath, mode, offset, limit, total: value.total, table };
}
