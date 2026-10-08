import type { PictureMetadataWriteResult } from '../bindings/github.com/boostao/vpro-wails';
import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { equalCell } from './projectMetadataEditor';
import { pictureMetadataFromWire, type PictureOwner, type PictureReview } from './pictureRead';
import { beginPictureMetadataDraft, pictureMetadataDraftErrors, pictureMetadataChanges, pictureMetadataRequest,
  stagePictureMetadata, type PictureMetadataColumn, type PictureMetadataDraft } from './pictureMetadataEditor';

export interface PictureMetadataPort {
  read(): CancellableRead<unknown>;
  save(requestJSON: string): CancellableRead<unknown>;
  receipt(requestJSON: string): CancellableRead<unknown>;
}
export interface PictureMetadataView {
  draft: PictureMetadataDraft | null;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  receipt: PictureMetadataReceipt | null;
  originalUpdate: (PictureOwner & { revision: number; original: PictureReview['records']['rows'][number] }) | null;
}
type Request = ReturnType<typeof pictureMetadataRequest>;
export type PictureMetadataReceipt = Omit<PictureMetadataWriteResult, 'columns' | 'original' | 'committed' | 'changes' | 'ownedFiles'> & {
  columns: PictureReview['records']['columns'];
  original: PictureReview['records']['rows'][number];
  committed: PictureReview['records']['rows'][number];
  changes: Request['changes'];
  ownedFiles: Record<string, string>;
};
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

export function pictureMetadataReceiptFromWire(wire: unknown, request: Request, owner: PictureOwner): PictureMetadataReceipt {
  const diagnostic = 'Picture Save receipt is incomplete or differs from its owned request; resolve durable history before replaying.';
  if (!record(wire) || wire.requestId !== request.requestId || wire.id !== request.id ||
      wire.contextId !== owner.contextId || wire.project !== owner.project || wire.plotNumber !== owner.plotNumber ||
      wire.didCommit !== true || typeof wire.replayed !== 'boolean' ||
      typeof wire.historyId !== 'string' || !/^[1-9][0-9]*$/.test(wire.historyId) ||
      BigInt(wire.historyId) > 9223372036854775807n || typeof wire.source !== 'string' || !wire.source ||
      typeof wire.requestedSource !== 'string' || !wire.requestedSource ||
      typeof wire.actor !== 'string' || !wire.actor || wire.actor.length > 100 ||
      !Number.isInteger(wire.auditStrength) || Number(wire.auditStrength) < 0 || Number(wire.auditStrength) > 3 ||
      typeof wire.editWhen !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$/.test(wire.editWhen) ||
      !Number.isFinite(Date.parse(wire.editWhen)) || !record(wire.ownedFiles) || !Object.keys(wire.ownedFiles).length ||
      Object.entries(wire.ownedFiles).some(([role, path]) => !role || typeof path !== 'string' || !path) ||
      !same(wire.columns, request.columns) || !same(wire.original, request.original) || !same(wire.changes, request.changes)) {
    throw new Error(diagnostic);
  }
  const review = pictureMetadataFromWire({ ...owner, records: { columns: wire.columns, rows: [wire.committed] } }, owner);
  const committed = review.records.rows[0];
  const expected = request.original.cells.map(cell => ({ ...cell }));
  for (const change of request.changes) {
    const index = request.columns.findIndex(column => column.name === change.column);
    if (index < 0) throw new Error(diagnostic);
    expected[index] = { ...change.value };
  }
  if (!committed || committed.rowId !== request.original.rowId ||
      committed.cells.some((cell, index) => !equalCell(cell, expected[index]))) throw new Error(diagnostic);
  const ownedFiles: Record<string, string> = {};
  for (const [role, path] of Object.entries(wire.ownedFiles)) {
    if (typeof path !== 'string') throw new Error(diagnostic);
    ownedFiles[role] = path;
  }
  return { requestId: request.requestId, contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber,
    requestedSource: wire.requestedSource, source: wire.source, ownedFiles, actor: wire.actor,
    auditStrength: Number(wire.auditStrength), id: request.id, columns: review.records.columns,
    original: { rowId: request.original.rowId, cells: request.original.cells.map(cell => ({ ...cell })) },
    committed, changes: request.changes.map(change => ({ column: change.column, value: { ...change.value } })),
    editWhen: wire.editWhen, historyId: wire.historyId, didCommit: true, replayed: wire.replayed };
}

export class PictureMetadataSession {
  private readonly owner: PictureOwner;
  private state: PictureMetadataView = { draft: null, busy: false, blocked: false, error: null, receipt: null, originalUpdate: null };
  private originalRevision = 0;
  private pending: Request | null = null;
  private generation = 0;
  private readonly requests = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: PictureOwner, private readonly port: PictureMetadataPort) {
    this.owner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  }

  subscribe(notify: () => void) {
    this.listeners.add(notify);
    notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): PictureMetadataView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const invalid = this.state.draft ? pictureMetadataDraftErrors(this.state.draft) : [];
    const dirty = !!this.state.draft && (invalid.length > 0 || pictureMetadataChanges(this.state.draft).length > 0);
    const reason = this.state.busy ? 'Wait for or cancel the picture metadata operation.'
      : this.state.blocked ? 'Resolve the picture Save receipt or explicitly discard and reload; do not replay.'
      : invalid[0] ?? (!this.state.draft ? 'Select one reviewed picture before editing.' : '');
    return { unsaved: dirty || this.state.blocked, busy: this.state.busy, blocked: this.state.blocked || invalid.length > 0,
      canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private requireIdle() {
    if (this.state.busy) throw new Error('Wait for or cancel the picture metadata operation.');
  }
  select(review: PictureReview, rowId: string) {
    this.requireIdle();
    if (this.closeState().unsaved) throw new Error('Finish or explicitly discard the retained picture draft before replacing its original.');
    this.state.draft = beginPictureMetadataDraft(review, rowId, this.owner);
    this.state.error = null;
    this.state.receipt = null;
    this.changed();
  }
  stage(column: PictureMetadataColumn, raw: string, nullValue: boolean) {
    this.requireIdle();
    if (!this.state.draft || this.state.blocked) throw new Error('Select a reviewed picture or resolve its unknown Save before editing.');
    this.state.draft = stagePictureMetadata(this.state.draft, column, raw, nullValue);
    this.state.error = pictureMetadataDraftErrors(this.state.draft)[0] ?? null;
    this.changed();
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++;
    this.requests.cancelAll();
    this.state.busy = false;
    if (this.pending) {
      this.state.blocked = true;
      this.state.error = 'Picture Save acknowledgement was cancelled; its outcome is unknown. Resolve durable history, not matching values.';
    } else this.state.error = 'Picture metadata reload cancelled; original draft retained.';
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.requireIdle();
    const close = this.closeState();
    if (!close.canSave || !this.state.draft) throw new Error(close.saveReason);
    const request = pictureMetadataRequest(this.state.draft, requestId, this.owner);
    if (!request.changes.length) {
      this.state.draft.cells = {};
      this.state.error = null;
      this.changed();
      return true;
    }
    this.pending = request;
    return this.writeReceipt(() => this.port.save(JSON.stringify(request)));
  }
  async resolve(): Promise<boolean> {
    this.requireIdle();
    if (!this.pending || !this.state.blocked) throw new Error('No unknown picture request is retained to resolve.');
    const request = this.pending;
    return this.writeReceipt(() => this.port.receipt(JSON.stringify(request)));
  }
  private async writeReceipt(operation: () => CancellableRead<unknown>): Promise<boolean> {
    const request = this.pending;
    if (!request) throw new Error('The retained picture request identity is unavailable.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    try {
      const wire = await this.requests.track(operation());
      if (generation !== this.generation) return false;
      const receipt = pictureMetadataReceiptFromWire(wire, request, this.owner);
      this.state.receipt = receipt;
      this.state.draft = beginPictureMetadataDraft({ ...this.owner, records: {
        columns: receipt.columns, rows: [receipt.committed],
      } }, receipt.committed.rowId, this.owner);
      this.state.originalUpdate = { ...this.owner, revision: ++this.originalRevision, original: receipt.committed };
      this.state.blocked = false;
      this.pending = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        this.state.blocked = true;
        this.state.error = `Picture Save outcome is unknown; draft/request retained and no automatic replay: ${String(cause)}`;
      }
      return false;
    } finally {
      if (generation === this.generation) {
        this.state.busy = false;
        this.changed();
      }
    }
  }
  async discardAndReload(): Promise<boolean> {
    this.requireIdle();
    if (!this.state.draft) throw new Error('No owned picture original is retained for reload.');
    const rowId = this.state.draft.original.rowId;
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.read());
      if (generation !== this.generation) return false;
      const review = pictureMetadataFromWire(wire, this.owner);
      this.state.draft = beginPictureMetadataDraft(review, rowId, this.owner);
      this.state.originalUpdate = { ...this.owner, revision: ++this.originalRevision, original: this.state.draft.original };
      this.state.blocked = false;
      this.state.error = null;
      this.pending = null;
      this.state.receipt = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Picture discard/reload failed; draft and unknown state retained: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
}
