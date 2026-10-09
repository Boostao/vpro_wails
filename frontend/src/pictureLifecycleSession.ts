import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { pictureMetadataFromWire, type PictureOwner, type PictureReview } from './pictureRead';
import { pictureDeletionRequest, pictureDeletionReceiptFromWire, pictureRestorationRequest,
  pictureRestorationReviewFromWire, pictureRestorationReceiptFromWire, pictureLifecycleHistoryFromWire,
  type PictureDeletionRequest, type PictureDeletionReceipt, type PictureRestorationRequest,
  type PictureRestorationReceipt, type PictureRestorationReview, type PictureLifecycleHistory } from './pictureLifecycleEditor';

export interface PictureLifecyclePort {
  read(): CancellableRead<unknown>;
  history(): CancellableRead<unknown>;
  review(historyId: string): CancellableRead<unknown>;
  delete(request: string): CancellableRead<unknown>;
  deletionReceipt(request: string): CancellableRead<unknown>;
  restore(request: string): CancellableRead<unknown>;
  restorationReceipt(request: string): CancellableRead<unknown>;
}
type Pending = { kind: 'delete'; request: PictureDeletionRequest } | { kind: 'restore'; request: PictureRestorationRequest };
export type PictureLifecycleCommit = { kind: 'delete'; receipt: PictureDeletionReceipt } | { kind: 'restore'; receipt: PictureRestorationReceipt };
export interface PictureLifecycleView {
  original: PictureReview | null;
  rowId: string | null;
  restoration: PictureRestorationReview | null;
  history: PictureLifecycleHistory | null;
  confirmed: boolean;
  busy: boolean;
  blocked: boolean;
  acknowledgementRequired: boolean;
  error: string | null;
  receipts: PictureLifecycleCommit[];
  revision: number;
}
export class PictureLifecycleSession {
  private readonly owner: PictureOwner;
  private state: PictureLifecycleView = { original: null, rowId: null, restoration: null, history: null,
    confirmed: false, busy: false, blocked: false, acknowledgementRequired: false, error: null, receipts: [], revision: 0 };
  private pending: Pending | null = null;
  private attempted = false;
  private generation = 0;
  private readonly requests = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: PictureOwner, private readonly port: PictureLifecyclePort) {
    this.owner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  }
  subscribe(notify: () => void) {
    this.listeners.add(notify); notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): PictureLifecycleView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const reason = this.state.busy ? 'Wait for or cancel the picture lifecycle operation.'
      : this.state.blocked ? 'Resolve the retained unknown mutation using read-only history; do not repeat it.'
      : this.state.acknowledgementRequired ? 'Acknowledge the committed picture receipt; acknowledgement retains it.'
      : !this.state.original && !this.state.restoration ? 'Review one exact picture row or historical deletion.'
      : !this.state.confirmed ? 'Explicitly confirm the reviewed physical operation and retained history.' : '';
    return { busy: this.state.busy, blocked: this.state.blocked,
      unsaved: this.state.original !== null || this.state.restoration !== null || this.state.blocked || this.state.acknowledgementRequired,
      canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private requireIdle() {
    if (this.state.busy) throw new Error('Wait for or cancel the picture lifecycle operation.');
  }
  private requireKnown() {
    this.requireIdle();
    if (this.state.blocked) throw new Error('Resolve the exact unknown picture request without replacing it.');
    if (this.state.acknowledgementRequired) throw new Error('Acknowledge the committed receipt without erasing it.');
  }
  private requireEmpty() {
    this.requireKnown();
    if (this.state.original || this.state.restoration) throw new Error('Explicitly discard the retained review before replacing it.');
  }
  async reviewDeletion(rowId: string): Promise<boolean> {
    this.requireEmpty();
    return this.read('Picture deletion review', this.port.read, wire => {
      const original = pictureMetadataFromWire(wire, this.owner);
      pictureDeletionRequest(original, this.owner, rowId, '00000000-0000-4000-8000-000000000001');
      this.state.original = original; this.state.rowId = rowId; this.state.confirmed = false;
    });
  }
  async loadHistory(): Promise<boolean> {
    this.requireEmpty();
    return this.read('Picture lifecycle history', this.port.history, wire => {
      this.state.history = pictureLifecycleHistoryFromWire(wire, this.owner);
    });
  }
  async reviewRestoration(historyId: string): Promise<boolean> {
    this.requireEmpty();
    return this.read('Picture restoration review', () => this.port.review(historyId), wire => {
      this.state.restoration = pictureRestorationReviewFromWire(wire, this.owner, historyId);
      this.state.confirmed = false;
    });
  }
  private async read(label: string, operation: () => CancellableRead<unknown>, accept: (wire: unknown) => void): Promise<boolean> {
    const generation = ++this.generation;
    this.state.busy = true; this.state.error = null; this.changed();
    try {
      if (generation !== this.generation) return false;
      const wire = await this.requests.track(operation.call(this.port));
      if (generation !== this.generation) return false;
      accept(wire);
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `${label} failed; prior state retained without inference: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  confirm(value: boolean) {
    this.requireKnown();
    if (typeof value !== 'boolean' || !this.state.original && !this.state.restoration) throw new Error('Confirmation requires a retained complete physical review.');
    this.state.confirmed = value; this.state.error = null; this.changed();
  }
  discard() {
    this.requireKnown();
    this.state.original = null; this.state.rowId = null; this.state.restoration = null;
    this.state.confirmed = false; this.state.error = null; this.changed();
  }
  acknowledge() {
    this.requireIdle();
    if (this.state.blocked || !this.state.acknowledgementRequired) throw new Error('Only a known committed receipt can be acknowledged.');
    this.state.acknowledgementRequired = false; this.state.error = null; this.changed();
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++; this.requests.cancelAll(); this.state.busy = false;
    if (this.pending && this.attempted) {
      this.state.blocked = true;
      this.state.error = 'Picture mutation delivery cancelled; stable request retained. Resolve read-only history without replay.';
    } else {
      this.pending = null;
      this.state.confirmed = false;
      this.state.error = 'Picture operation cancelled before mutation dispatch; no new confirmation was inferred.';
    }
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.requireKnown();
    const close = this.closeState();
    if (!close.canSave) throw new Error(close.saveReason);
    if (this.state.restoration) {
      this.pending = { kind: 'restore', request: pictureRestorationRequest(this.state.restoration, this.owner, requestId) };
    } else if (this.state.original && this.state.rowId !== null) {
      this.pending = { kind: 'delete', request: pictureDeletionRequest(this.state.original, this.owner, this.state.rowId, requestId) };
    } else throw new Error('Complete retained picture review is missing.');
    this.attempted = false;
    return this.acceptReceipt(false);
  }
  async resolve(): Promise<boolean> {
    this.requireIdle();
    if (!this.pending || !this.state.blocked) throw new Error('No exact unknown picture request is retained.');
    return this.acceptReceipt(true);
  }
  private async acceptReceipt(lookup: boolean): Promise<boolean> {
    const pending = this.pending;
    if (!pending) throw new Error('Stable picture mutation request is unavailable.');
    const generation = ++this.generation;
    this.state.busy = true; this.state.error = null; this.changed();
    try {
      if (generation !== this.generation) return false;
      const json = JSON.stringify(pending.request);
      if (!lookup) this.attempted = true;
      const operation = pending.kind === 'delete'
        ? lookup ? this.port.deletionReceipt(json) : this.port.delete(json)
        : lookup ? this.port.restorationReceipt(json) : this.port.restore(json);
      const wire = await this.requests.track(operation);
      if (generation !== this.generation) return false;
      if (wire === null) throw new Error('Missing history is unresolved; it does not authorize automatic replay.');
      const commit: PictureLifecycleCommit = pending.kind === 'delete'
        ? { kind: 'delete', receipt: pictureDeletionReceiptFromWire(wire, pending.request) }
        : { kind: 'restore', receipt: pictureRestorationReceiptFromWire(wire, pending.request) };
      this.state.receipts.push(commit); this.state.revision++;
      this.pending = null; this.state.blocked = false; this.state.acknowledgementRequired = true;
      this.state.original = null; this.state.rowId = null; this.state.restoration = null;
      this.state.history = null; this.state.confirmed = false;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        if (this.attempted) {
          this.state.blocked = true;
          this.state.error = `Picture mutation outcome remains unknown; original/request retained without replay: ${String(cause)}`;
        } else {
          this.pending = null; this.state.confirmed = false;
          this.state.error = `Picture request preparation failed before dispatch; retained review requires correction: ${String(cause)}`;
        }
      }
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
}
