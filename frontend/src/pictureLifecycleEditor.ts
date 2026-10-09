import type { PictureOwner, PictureReview } from './pictureRead';
import { pictureMetadataFromWire } from './pictureRead';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { validSIVIRequestId } from './siviRequestId';

type Row = PictureReview['records']['rows'][number];
export interface PictureDeletionRequest extends PictureOwner {
  requestId: string;
  columns: PictureReview['records']['columns'];
  original: Row;
}
export interface PictureDeletionReceipt {
  request: PictureDeletionRequest;
  id: number;
  requestedSource: string;
  source: string;
  ownedFiles: Record<string, string>;
  actor: string;
  auditStrength: number;
  deletedWhen: string;
  historyId: string;
  didCommit: true;
  replayed: boolean;
}
export interface PictureRestorationRequest {
  requestId: string;
  contextId: string;
  historyId: string;
  deletion: PictureDeletionRequest;
  peers: Row[];
}
export interface PictureRestorationReceipt {
  request: PictureRestorationRequest;
  deletion: PictureDeletionReceipt;
  actor: string;
  auditStrength: number;
  restoredWhen: string;
  didCommit: true;
  replayed: boolean;
}
export interface PictureRestorationReview extends PictureOwner {
  historyId: string;
  deletion: PictureDeletionReceipt;
  peers: Row[];
}
export interface PictureLifecycleHistory extends PictureOwner {
  events: { deletion: PictureDeletionReceipt; restoration: PictureRestorationReceipt | null }[];
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const literal = (value: unknown, limit?: number): value is string =>
  typeof value === 'string' && value !== '' && !value.includes('\0') && wellFormedUTF16(value) &&
  (limit === undefined || value.length <= limit);
const historyId = (value: unknown): value is string =>
  typeof value === 'string' && /^[1-9][0-9]*$/.test(value) && BigInt(value) <= 9223372036854775807n;
const utc = (value: unknown): value is string =>
  typeof value === 'string' && /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(value) && Number.isFinite(Date.parse(value));
function canonicalJSON(value: unknown): string | undefined {
  return JSON.stringify(value, (_key, item: unknown) => record(item)
    ? Object.fromEntries(Object.keys(item).sort().map(key => [key, item[key]])) : item);
}
const same = (left: unknown, right: unknown) => canonicalJSON(left) === canonicalJSON(right);

function requireOwner(value: PictureOwner, owner: PictureOwner) {
  if (!literal(owner.contextId, 100) || !literal(owner.project, 255) || !literal(owner.plotNumber) ||
      value.contextId !== owner.contextId || value.project !== owner.project || value.plotNumber !== owner.plotNumber) {
    throw new Error('Picture lifecycle requires the exact current context, project and literal parent.');
  }
}
function detachedRow(row: Row): Row {
  return { rowId: row.rowId, cells: row.cells.map(cell => ({ ...cell })) };
}
function requestFromWire(wire: unknown, owner?: PictureOwner): PictureDeletionRequest {
  if (!record(wire) || !validSIVIRequestId(wire.requestId) || !literal(wire.contextId, 100) ||
      !literal(wire.project, 255) || !literal(wire.plotNumber)) throw new Error('Complete immutable picture deletion request is missing.');
  const origin = { contextId: wire.contextId, project: wire.project, plotNumber: wire.plotNumber };
  if (owner) requireOwner(origin, owner);
  const parsed = pictureMetadataFromWire({ ...origin, records: { columns: wire.columns, rows: [wire.original] } }, origin);
  const original = parsed.records.rows[0], id = original.cells[0];
  if (id.storage !== 'integer' || typeof id.integer !== 'string' ||
      BigInt(id.integer) < -2147483648n || BigInt(id.integer) > 2147483647n) {
    throw new Error('NULL or noninteger picture IDs are unavailable; no identity repair or ID=0 inference.');
  }
  return { ...origin, requestId: wire.requestId, columns: parsed.records.columns, original };
}
export function pictureDeletionRequest(review: PictureReview, owner: PictureOwner, rowId: string, requestId: string): PictureDeletionRequest {
  const detached = { contextId: review.contextId, project: review.project, plotNumber: review.plotNumber,
    records: { columns: review.records.columns.map(column => ({ name: column.name, declaredType: column.declaredType })),
      rows: review.records.rows.map(detachedRow) } };
  const parsed = pictureMetadataFromWire(detached, owner);
  const original = parsed.records.rows.find(row => row.rowId === rowId);
  if (!original) throw new Error('Review one complete existing physical picture row before deleting.');
  return requestFromWire({ ...owner, requestId, columns: parsed.records.columns, original }, owner);
}
function actorPolicy(wire: Record<string, unknown>) {
  if (!literal(wire.actor, 255) || typeof wire.auditStrength !== 'number' || !Number.isInteger(wire.auditStrength) ||
      wire.auditStrength < 0 || wire.auditStrength > 3) throw new Error('Picture lifecycle actor/audit policy is incomplete.');
  return { actor: wire.actor, auditStrength: wire.auditStrength };
}
export function pictureDeletionReceiptFromWire(wire: unknown, expected?: PictureDeletionRequest): PictureDeletionReceipt {
  if (!record(wire) || !literal(wire.requestedSource) || !literal(wire.source) || !record(wire.ownedFiles) ||
      Object.keys(wire.ownedFiles).length === 0 || !utc(wire.deletedWhen) || !historyId(wire.historyId) ||
      wire.didCommit !== true || typeof wire.replayed !== 'boolean') throw new Error('Complete durable picture deletion receipt is missing.');
  const request = requestFromWire(wire.request), id = Number(request.original.cells[0].integer);
  if (wire.id !== id || expected && !same(request, expected)) throw new Error('Picture deletion receipt differs from the exact retained request.');
  const ownedFiles: Record<string, string> = {};
  for (const [role, path] of Object.entries(wire.ownedFiles)) {
    if (!literal(role) || !literal(path)) throw new Error('Picture lifecycle owned file provenance is incomplete.');
    ownedFiles[role] = path;
  }
  return { request, id, requestedSource: wire.requestedSource, source: wire.source, ownedFiles, ...actorPolicy(wire),
    deletedWhen: wire.deletedWhen, historyId: wire.historyId, didCommit: true, replayed: wire.replayed };
}
function restorationRequestFromWire(wire: unknown): PictureRestorationRequest {
  if (!record(wire) || !validSIVIRequestId(wire.requestId) || !literal(wire.contextId, 100) || !historyId(wire.historyId)) {
    throw new Error('Complete stable picture restoration request is missing.');
  }
  const deletion = requestFromWire(wire.deletion);
  const records = pictureMetadataFromWire({ ...deletion, records: { columns: deletion.columns, rows: wire.peers } }, deletion).records;
  if (wire.requestId === deletion.requestId || records.rows.some(row => row.rowId === deletion.original.rowId)) {
    throw new Error('Picture restoration requires a distinct request and complete peers excluding the deleted row.');
  }
  return { requestId: wire.requestId, contextId: wire.contextId, historyId: wire.historyId, deletion, peers: records.rows };
}
export function pictureRestorationReceiptFromWire(wire: unknown, expected?: PictureRestorationRequest): PictureRestorationReceipt {
  if (!record(wire) || !utc(wire.restoredWhen) || wire.didCommit !== true || typeof wire.replayed !== 'boolean') {
    throw new Error('Complete durable picture restoration receipt is missing.');
  }
  const request = restorationRequestFromWire(wire.request), deletion = pictureDeletionReceiptFromWire(wire.deletion);
  if (deletion.replayed || request.historyId !== deletion.historyId || !same(request.deletion, deletion.request) ||
      expected && !same(request, expected)) throw new Error('Picture restoration receipt differs from the immutable deletion/review.');
  return { request, deletion, ...actorPolicy(wire), restoredWhen: wire.restoredWhen, didCommit: true, replayed: wire.replayed };
}
export function pictureRestorationReviewFromWire(wire: unknown, owner: PictureOwner, expectedHistoryId: string): PictureRestorationReview {
  if (!record(wire) || wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plotNumber !== owner.plotNumber ||
      wire.historyId !== expectedHistoryId || !historyId(wire.historyId)) throw new Error('Picture restoration review belongs to another owner/history.');
  requireOwner(owner, owner);
  const deletion = pictureDeletionReceiptFromWire(wire.deletion);
  if (deletion.replayed || deletion.historyId !== wire.historyId || deletion.request.project !== owner.project ||
      deletion.request.plotNumber !== owner.plotNumber) throw new Error('Historical picture deletion does not belong to this literal owner.');
  const parsed = pictureMetadataFromWire({ ...owner, records: { columns: deletion.request.columns, rows: wire.peers } }, owner);
  if (parsed.records.rows.some(row => row.rowId === deletion.request.original.rowId)) throw new Error('Historical picture rowid is already occupied.');
  return { ...owner, historyId: wire.historyId, deletion, peers: parsed.records.rows };
}
export function pictureRestorationRequest(review: PictureRestorationReview, owner: PictureOwner, requestId: string): PictureRestorationRequest {
  requireOwner(review, owner);
  const origin = review.deletion.request;
  const deletion = { ...review.deletion, ownedFiles: { ...review.deletion.ownedFiles },
    request: { ...origin, columns: origin.columns.map(column => ({ name: column.name, declaredType: column.declaredType })),
      original: detachedRow(origin.original) } };
  const detached = { ...owner, historyId: review.historyId, deletion, peers: review.peers.map(detachedRow) };
  const parsed = pictureRestorationReviewFromWire(detached, owner, review.historyId);
  return restorationRequestFromWire({ requestId, contextId: owner.contextId, historyId: parsed.historyId,
    deletion: parsed.deletion.request, peers: parsed.peers });
}
export function pictureLifecycleHistoryFromWire(wire: unknown, owner: PictureOwner): PictureLifecycleHistory {
  if (!record(wire) || wire.contextId !== owner.contextId || wire.project !== owner.project ||
      wire.plotNumber !== owner.plotNumber || !Array.isArray(wire.events)) throw new Error('Owned picture lifecycle history is incomplete.');
  requireOwner(owner, owner);
  const historyIds = new Set<string>(), requestIds = new Set<string>();
  const events = wire.events.map(event => {
    if (!record(event)) throw new Error('Typed picture lifecycle event is missing.');
    const deletion = pictureDeletionReceiptFromWire(event.deletion);
    if (deletion.replayed || deletion.request.project !== owner.project || deletion.request.plotNumber !== owner.plotNumber ||
        historyIds.has(deletion.historyId) || requestIds.has(deletion.request.requestId)) throw new Error('Picture history has duplicate or foreign immutable events.');
    historyIds.add(deletion.historyId); requestIds.add(deletion.request.requestId);
    const restoration = event.restoration === null ? null : pictureRestorationReceiptFromWire(event.restoration);
    if (restoration && (restoration.replayed || !same(restoration.deletion, deletion) || requestIds.has(restoration.request.requestId))) {
      throw new Error('Picture history restoration does not match its immutable deletion.');
    }
    if (restoration) requestIds.add(restoration.request.requestId);
    return { deletion, restoration };
  });
  return { ...owner, events };
}
export function pictureLifecycleOriginalMatches(row: Row, original: Row): boolean {
  return row.rowId === original.rowId && row.cells.length === original.cells.length &&
    row.cells.every((cell, index) => equalCell(cell, original.cells[index]));
}
