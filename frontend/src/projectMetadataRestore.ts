import type { AuditRestoreAction, AuditRestoreResult, ProjectMetadataCell, ProjectMetadataRestore, ProjectMetadataRestoreHistory, ProjectMetadataRestoreReview } from '../bindings/github.com/boostao/vpro-wails';
import { beginMetadataDraft, equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';

function exactSigned64(value: string): boolean {
  if (typeof value !== 'string' || !/^-?(0|[1-9]\d*)$/.test(value)) return false;
  const number = BigInt(value);
  return number.toString() === value && number >= -(1n << 63n) && number < (1n << 63n);
}

function completeCell(cell: ProjectMetadataCell): boolean {
  if (!cell) return false;
  const { storage, text, integer, real, blobHex } = cell;
  if (storage === 'null') return text === null && integer === null && real === null && blobHex === null;
  if (storage === 'text') return typeof text === 'string' && wellFormedUTF16(text) && integer === null && real === null && blobHex === null;
  if (storage === 'integer') return typeof integer === 'string' && exactSigned64(integer) && text === null && real === null && blobHex === null;
  if (storage === 'real') return typeof real === 'number' && Number.isFinite(real) && text === null && integer === null && blobHex === null;
  return storage === 'blob' && typeof blobHex === 'string' && /^(?:[0-9a-f]{2})*$/.test(blobHex) && text === null && integer === null && real === null;
}

export function validateMetadataRestorationHistory(history: ProjectMetadataRestoreHistory): ProjectMetadataRestoreHistory {
  if (!history || typeof history.historyPresent !== 'boolean' || !Array.isArray(history.events) ||
      !history.historyPresent && history.events.length !== 0 ||
      new Set(history.events.map(event => event.historyId)).size !== history.events.length ||
      history.events.some(event => !exactSigned64(event.historyId) || !exactSigned64(event.rowId) ||
        !Number.isInteger(event.id) || event.id < -2147483648 || event.id > 2147483647 ||
        typeof event.created !== 'string' || !wellFormedUTF16(event.created) || typeof event.restored !== 'boolean' ||
        !event.fields?.length || new Set(event.fields).size !== event.fields.length ||
        event.fields.some(field => typeof field !== 'string' || !field || !wellFormedUTF16(field)))) {
    throw new Error('Complete typed metadata history with exact distinct identities was not returned.');
  }
  return history;
}

export function validateMetadataRestorationReview(review: ProjectMetadataRestoreReview, contextId: string, plot: string): ProjectMetadataRestoreReview {
  if (!review || !contextId || review.contextId !== contextId || review.plotNumber !== plot ||
      !exactSigned64(review.historyId) || !review.columns?.length || !review.current || !review.restored ||
      !exactSigned64(review.current.rowId) || review.restored.rowId !== review.current.rowId ||
      !review.audits?.length || new Set(review.audits.map(audit => audit.rowId)).size !== review.audits.length) {
    throw new Error('Metadata restoration requires a complete exact current-context typed review.');
  }
  for (const row of [review.current, review.restored]) {
    if (!row.cells || row.cells.length !== review.columns.length || !row.cells.every(completeCell)) {
      throw new Error('Metadata restoration contains incomplete or normalized storage-class cells.');
    }
    const draft = beginMetadataDraft({ project: review.project, plotNumber: plot, projectId: review.projectId,
      projectRecords: { columns: review.columns, rows: [row] }, masterTemplates: { columns: [], rows: [] } }, row.rowId);
    if (draft.id !== review.id) throw new Error('Metadata restoration changes the physical record identity.');
  }
  const names = new Set<string>();
  for (const audit of review.audits) {
    if (!exactSigned64(audit.rowId) || audit.project !== review.project || audit.plotNumber !== plot ||
        audit.id !== review.id || !['_metadata', `${review.project}_metadata`.toLowerCase()].includes(audit.table.toLowerCase()) ||
        names.has(audit.editField) || !review.columns.some(column => column.name === audit.editField) ||
        ['ID', 'ProjectID'].includes(audit.editField)) {
      throw new Error('Metadata restoration has foreign, repeated or identity-changing audit fields.');
    }
    names.add(audit.editField);
  }
  for (let i = 0; i < review.columns.length; i++) {
    const changed = !equalCell(review.current.cells![i], review.restored.cells![i]);
    if (changed !== names.has(review.columns[i].name)) {
      throw new Error('Metadata restoration must change exactly its proven audited fields; unaudited values stay unchanged.');
    }
  }
  return review;
}

export function metadataRestorationRequest(review: ProjectMetadataRestoreReview, action: AuditRestoreAction,
  contextId: string, plot: string): ProjectMetadataRestore {
  if (action !== 'retain' && action !== 'prune') throw new Error('Explicit retain or prune confirmation is required.');
  return { review: structuredClone(validateMetadataRestorationReview(review, contextId, plot)), action, confirmed: true };
}

export function validateMetadataRestorationResult(result: AuditRestoreResult | null, request: ProjectMetadataRestore): asserts result is AuditRestoreResult {
  if (!result || result.cancelled !== false || result.cleanedVegRows !== 0 ||
      result.restoredRows !== request.review.audits!.length ||
      result.prunedAuditRows !== (request.action === 'prune' ? request.review.audits!.length : 0)) {
    throw new Error('Metadata restoration committed but returned an unexpected result; reload without replaying it.');
  }
}
