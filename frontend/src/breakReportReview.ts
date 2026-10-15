import { lifeformShape as shape, lifeformText as scopeText } from './lifeformSummary';

export type BreakReportPair = { reference: string | null; breakCode: string | null };
export type BreakReportPage = {
  contextId: string; sourcePath: string; sourceSHA256: string;
  offset: number; limit: number; total: number; rows: BreakReportPair[];
};
export const breakReportFields = [
  { key: 'breakCode', caption: 'PC-ORD Break Code' },
  { key: 'reference', caption: 'VPro Reference' },
] as const;
const count = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const cell = (value: unknown): value is string | null =>
  value === null || typeof value === 'string' && value.length <= 255 && !/[^\x20-\x7e]/.test(value);

function compare(left: string | null, right: string | null): number {
  if (left === null || right === null) return left === right ? 0 : left === null ? -1 : 1;
  const a = left.toLowerCase().replace(/ +$/, ''), b = right.toLowerCase().replace(/ +$/, '');
  return a < b ? -1 : a > b ? 1 : 0;
}

export function breakReportFromWire(value: unknown, contextId: string, offset: number, limit: number): BreakReportPage {
  if (!scopeText(contextId) || !contextId || !count(offset) || !count(limit) || limit < 1 || limit > 100) {
    throw new Error('PC-ORD Break Report requires the current context and a bounded literal page.');
  }
  if (!shape(value, ['contextId', 'sourcePath', 'sourceSHA256', 'offset', 'limit', 'total', 'rows']) ||
      value.contextId !== contextId || !scopeText(value.sourcePath) || !value.sourcePath ||
      typeof value.sourceSHA256 !== 'string' || value.sourceSHA256.length !== 64 || !/^[0-9a-f]{64}$/.test(value.sourceSHA256) ||
      value.offset !== offset || value.limit !== limit || !count(value.total) || value.total > 10000 ||
      !Array.isArray(value.rows) || value.rows.length !== Math.min(limit, Math.max(0, value.total - offset))) {
    throw new Error('PC-ORD Break Report has foreign ownership, invalid original provenance or incomplete page bounds.');
  }
  const rows: BreakReportPair[] = [];
  for (const row of value.rows) {
    if (!shape(row, ['reference', 'breakCode']) || !cell(row.reference) || !cell(row.breakCode)) {
      throw new Error('PC-ORD Break Report lost its complete nullable literal TEXT255 pair.');
    }
    const previous = rows.at(-1);
    if (previous) {
      const order = compare(previous.breakCode, row.breakCode) || compare(previous.reference, row.reference);
      if (order >= 0) throw new Error('PC-ORD Break Report repeated or reordered its technical DISTINCT pairs.');
    }
    rows.push({ reference: row.reference, breakCode: row.breakCode });
  }
  return { contextId, sourcePath: value.sourcePath, sourceSHA256: value.sourceSHA256, offset, limit, total: value.total, rows };
}

export function sameBreakReportSource(left: BreakReportPage, right: BreakReportPage): boolean {
  return left.contextId === right.contextId && left.sourcePath === right.sourcePath &&
    left.sourceSHA256 === right.sourceSHA256 && left.total === right.total;
}
