import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { pictureMetadataFromWire, type PictureOwner, type PictureReview } from './pictureRead';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { validSIVIRequestId } from './siviRequestId';

export interface PictureSelection extends PictureOwner {
  token: string;
  selectedPath: string;
  windowId: number;
}
export interface PictureCreationRequest extends PictureOwner {
  requestId: string;
  selectedPath: string;
}
export interface PictureCreationReceipt extends PictureCreationRequest {
  requestedSource: string;
  source: string;
  ownedFiles: Record<string, string>;
  actor: string;
  auditStrength: number;
  id: number;
  rowId: string;
  columns: PictureReview['records']['columns'];
  committed: PictureReview['records']['rows'][number];
  createdWhen: string;
  historyId: string;
  didCommit: true;
  replayed: boolean;
}

const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const literal = (value: unknown, limit?: number): value is string =>
  typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0') &&
  (limit === undefined || value.length <= limit);

function requireOwner(value: PictureOwner, owner: PictureOwner) {
  if (!literal(owner.contextId, 255) || !literal(owner.project, 255) || !literal(owner.plotNumber, 7) ||
      value.contextId !== owner.contextId || value.project !== owner.project || value.plotNumber !== owner.plotNumber) {
    throw new Error('Picture creation requires the active context, project and exact literal source parent.');
  }
}

function selectedParts(path: unknown): { directory: string; name: string } {
  if (!literal(path) || path.includes('/') || !/^(?:[A-Za-z]:\\|\\\\[^\\]+\\[^\\]+\\)/.test(path) ||
      path.startsWith('\\\\?\\') || path.startsWith('\\\\.\\')) {
    throw new Error('Picture selection requires the complete literal Windows file path returned by the owned chooser.');
  }
  const last = path.lastIndexOf('\\'), directory = path.slice(0, last + 1), name = path.slice(last + 1);
  if (!literal(directory, 255) || !literal(name, 255)) {
    throw new Error('Selected picture directory and filename must each fit the source 255 UTF-16 units without repair.');
  }
  return { directory, name };
}

export function pictureSelectionFromWire(wire: unknown, owner: PictureOwner): PictureSelection | null {
  requireOwner(owner, owner);
  if (wire === null) return null;
  if (!record(wire) || typeof wire.token !== 'string' || !/^[0-9a-f]{64}$/.test(wire.token) ||
      typeof wire.windowId !== 'number' || !Number.isSafeInteger(wire.windowId) || wire.windowId < 0 ||
      typeof wire.contextId !== 'string' || typeof wire.project !== 'string' || typeof wire.plotNumber !== 'string') {
    throw new Error('Picture selection did not return a complete owned native-file capability.');
  }
  requireOwner({ contextId: wire.contextId, project: wire.project, plotNumber: wire.plotNumber }, owner);
  selectedParts(wire.selectedPath);
  if (typeof wire.selectedPath !== 'string') throw new Error('Picture selection path is missing.');
  return { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber,
    token: wire.token, selectedPath: wire.selectedPath, windowId: wire.windowId };
}

export function pictureCreationRequest(selection: PictureSelection, owner: PictureOwner, requestId: string): PictureCreationRequest {
  const approved = pictureSelectionFromWire(selection, owner);
  if (!approved || !validSIVIRequestId(requestId)) {
    throw new Error('Picture creation requires one owned selection and stable lowercase UUIDv4 request identity.');
  }
  return { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber,
    requestId, selectedPath: approved.selectedPath };
}

export function pictureCreationReceiptFromWire(wire: unknown, request: PictureCreationRequest,
  owner: PictureOwner): PictureCreationReceipt {
  const diagnostic = 'Picture creation receipt differs from its complete owned request; resolve durable history before replaying.';
  requireOwner(request, owner);
  const parts = selectedParts(request.selectedPath);
  if (!validSIVIRequestId(request.requestId) || !record(wire) || wire.requestId !== request.requestId ||
      wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plotNumber !== owner.plotNumber ||
      wire.selectedPath !== request.selectedPath || wire.didCommit !== true || typeof wire.replayed !== 'boolean' ||
      typeof wire.id !== 'number' || !Number.isInteger(wire.id) || wire.id < 1 || wire.id > 2147483647 ||
      !literal(wire.rowId) || !literal(wire.requestedSource) || !literal(wire.source) || !literal(wire.actor, 255) ||
      !record(wire.ownedFiles) || Object.keys(wire.ownedFiles).length === 0 ||
      typeof wire.auditStrength !== 'number' || !Number.isInteger(wire.auditStrength) ||
      wire.auditStrength < 0 || wire.auditStrength > 3 ||
      typeof wire.historyId !== 'string' || !/^[1-9][0-9]*$/.test(wire.historyId) ||
      BigInt(wire.historyId) > 9223372036854775807n ||
      typeof wire.createdWhen !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(wire.createdWhen) ||
      !Number.isFinite(Date.parse(wire.createdWhen))) {
    throw new Error(diagnostic);
  }
  const ownedEntries: [string, string][] = [];
  for (const [role, path] of Object.entries(wire.ownedFiles)) {
    if (!literal(role) || !literal(path)) throw new Error(diagnostic);
    ownedEntries.push([role, path]);
  }
  const review = pictureMetadataFromWire({ ...owner, records: { columns: wire.columns, rows: [wire.committed] } }, owner);
  const committed = review.records.rows[0];
  const empty = { text: null, integer: null, real: null, blobHex: null };
  const expected: ProjectMetadataCell[] = [
    { ...empty, storage: 'integer', integer: String(wire.id) },
    { ...empty, storage: 'text', text: parts.directory },
    { ...empty, storage: 'text', text: parts.name },
    { ...empty, storage: 'text', text: owner.plotNumber },
    { ...empty, storage: 'null' },
  ];
  if (!committed || committed.rowId !== wire.rowId ||
      committed.cells.some((cell, index) => !equalCell(cell, expected[index]))) throw new Error(diagnostic);
  return { ...request, requestedSource: wire.requestedSource, source: wire.source,
    ownedFiles: Object.fromEntries(ownedEntries), actor: wire.actor, auditStrength: wire.auditStrength,
    id: wire.id, rowId: wire.rowId, columns: review.records.columns, committed, createdWhen: wire.createdWhen,
    historyId: wire.historyId, didCommit: true, replayed: wire.replayed };
}
