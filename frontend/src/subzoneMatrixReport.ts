import { lifeformShape as shape, lifeformText as scopeText } from './lifeformSummary';
import { wellFormedUTF16 } from './qualityEditor';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';
import { PublicationSession } from './publicationSession';

export type SubzoneMatrixOwner = { contextId: string; hierarchy: string; hierarchyPath: string };
export type MatrixPage = SubzoneMatrixOwner & {
  labelsPath: string; sourceSHA256: string; columns: string[];
  rows: { name: string | null; values: (string | null)[] }[];
  total: number; offset: number; limit: number;
};
export type MatrixWorkbookReview = {
  preview: MatrixPage; approvalHash: string; workbookSHA256: string; bytes: number;
};
const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const cell = (value: unknown): value is string | null =>
  value === null || typeof value === 'string' && wellFormedUTF16(value);
const pageBound = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;

export function matrixCellText(value: string | null): string {
  return value === null ? '(NULL)' : value === '' ? '(empty text)' : value;
}

export function matrixPageFromWire(input: unknown, owner: SubzoneMatrixOwner, offset: number, limit: number): MatrixPage {
  if (!scopeText(owner.contextId) || !owner.contextId || !scopeText(owner.hierarchy) || !owner.hierarchy ||
      owner.hierarchy === 'None' || !scopeText(owner.hierarchyPath) || !owner.hierarchyPath ||
      !pageBound(offset) || !pageBound(limit) || limit < 1 || limit > 100) {
    throw new Error('Subzone Matrix requires the selected hierarchy and a bounded literal page.');
  }
  if (!shape(input, ['contextId', 'hierarchy', 'hierarchyPath', 'labelsPath', 'sourceSHA256', 'columns', 'rows', 'total', 'offset', 'limit']) ||
      input.contextId !== owner.contextId || input.hierarchy !== owner.hierarchy || input.hierarchyPath !== owner.hierarchyPath ||
      !scopeText(input.labelsPath) || !input.labelsPath || !hash(input.sourceSHA256) ||
      input.offset !== offset || input.limit !== limit || !pageBound(input.total) || input.total > 100000 ||
      !Array.isArray(input.columns) || input.columns.length > 254 || !Array.isArray(input.rows) ||
      input.total * input.columns.length > 1000000 || input.columns.length === 0 && input.total !== 0 ||
      input.rows.length !== Math.min(limit, Math.max(0, input.total - offset))) {
    throw new Error('Subzone Matrix has foreign scope, invalid source provenance or incomplete page bounds.');
  }
  const columns: string[] = [];
  for (const column of input.columns) {
    if (!scopeText(column) || !column || columns.includes(column)) {
      throw new Error('Subzone Matrix has missing or duplicate pivot captions.');
    }
    columns.push(column);
  }
  const rows: MatrixPage['rows'] = [];
  for (const row of input.rows) {
    if (!shape(row, ['name', 'values']) || !cell(row.name) || !Array.isArray(row.values) ||
        row.values.length !== columns.length) throw new Error('Subzone Matrix lost a complete nullable row.');
    const values: (string | null)[] = [];
    for (const value of row.values) {
      if (!cell(value)) throw new Error('Subzone Matrix repaired or omitted literal source text.');
      values.push(value);
    }
    rows.push({ name: row.name, values });
  }
  return { ...owner, labelsPath: input.labelsPath, sourceSHA256: input.sourceSHA256, columns, rows,
    total: input.total, offset, limit };
}

export function matrixWorkbookFromWire(input: unknown, owner: SubzoneMatrixOwner): MatrixWorkbookReview {
  if (!shape(input, ['preview', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      !hash(input.approvalHash) || !hash(input.workbookSHA256) ||
      !pageBound(input.bytes) || input.bytes === 0) {
    throw new Error('Subzone Matrix workbook review has incomplete approval or exact byte metadata.');
  }
  const preview = matrixPageFromWire(input.preview, owner, 0, 25);
  if (preview.total === 0) throw new Error('Subzone Matrix workbook review has nothing to report.');
  return { preview, approvalHash: input.approvalHash, workbookSHA256: input.workbookSHA256, bytes: input.bytes };
}

export function sameMatrixSource(left: MatrixPage, right: MatrixPage): boolean {
  return left.contextId === right.contextId && left.hierarchy === right.hierarchy &&
    left.hierarchyPath === right.hierarchyPath && left.labelsPath === right.labelsPath &&
    left.sourceSHA256 === right.sourceSHA256 && left.total === right.total &&
    left.columns.length === right.columns.length && left.columns.every((column, index) => column === right.columns[index]);
}

export class MatrixPublicationSession {
  private readonly receipt = new PublicationSession<ReturnType<typeof validateEnvironmentWorkbookOutcome>>('Subzone Matrix workbook');
  constructor(private readonly owner: SubzoneMatrixOwner) { this.owner = structuredClone(owner); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  async publish(review: MatrixWorkbookReview, destination: string,
    port: (request: { approvalHash: string; destination: string }) => PromiseLike<Parameters<typeof validateEnvironmentWorkbookOutcome>[0]>) {
    if (!scopeText(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Subzone Matrix requires an explicit literal .xlsx destination.');
    }
    const validated = matrixWorkbookFromWire(review, this.owner);
    await this.receipt.publish({ approvalHash: validated.approvalHash, destination }, destination, port,
      value => validateEnvironmentWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { key: string; session: MatrixPublicationSession }>();
export function matrixPublicationSession(owner: SubzoneMatrixOwner): MatrixPublicationSession {
  const key = JSON.stringify([owner.contextId, owner.hierarchy, owner.hierarchyPath]);
  const prior = sessions.get(owner.contextId);
  if (prior) {
    if (prior.key !== key) throw new Error('Subzone Matrix receipt belongs to a different selected hierarchy.');
    return prior.session;
  }
  const session = new MatrixPublicationSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}
