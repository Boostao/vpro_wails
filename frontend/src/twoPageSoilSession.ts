import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { equalCell } from './projectMetadataEditor';
import { twoPageSoilOriginalFromWire, stageTwoPageSoil, twoPageSoilRequest,
  type TwoPageSoilOwner, type TwoPageSoilOriginal, type TwoPageSoilDrafts } from './twoPageSoilEditor';
import { twoPageSoilReceiptFromWire, twoPageSoilOutcomeFromWire, type TwoPageSoilRequest, type TwoPageSoilReceipt } from './twoPageSoilReceipt';

export interface TwoPageSoilPort {
  read(): CancellableRead<unknown>;
  save(requestJSON: string): CancellableRead<unknown>;
  outcome(requestJSON: string): CancellableRead<unknown>;
  validationYield?(): PromiseLike<void>;
}
export interface TwoPageSoilView {
  original: TwoPageSoilOriginal | null;
  drafts: TwoPageSoilDrafts;
  busy: boolean;
  blocked: boolean;
  acknowledgementRequired: boolean;
  error: string | null;
  receipts: TwoPageSoilReceipt[];
  revision: number;
  noCommitCertificate: { requestId: string; reason: string } | null;
}
export class TwoPageSoilSession {
  private readonly owner: TwoPageSoilOwner;
  private state: TwoPageSoilView = { original: null, drafts: {}, busy: false, blocked: false,
    acknowledgementRequired: false, error: null, receipts: [], revision: 0, noCommitCertificate: null };
  private pending: { request: TwoPageSoilRequest; json: string } | null = null;
  private attempted = false;
  private dispatchSettled = true;
  private disposed = false;
  private generation = 0;
  private readonly reads = new ReadRequests();
  private readonly listeners = new Set<() => void>();
  private readonly retiredRequests = new Set<string>();

  constructor(owner: TwoPageSoilOwner, private readonly port: TwoPageSoilPort, private readonly writingEnabled = true) {
    this.owner = { ...owner };
  }
  subscribe(notify: () => void) {
    if (this.disposed) throw new Error('Soil session has been disposed.');
    this.listeners.add(notify); notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): TwoPageSoilView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const invalid = Object.values(this.state.drafts).find(draft => draft.error)?.error;
    const dirty = Object.values(this.state.drafts).some(draft => draft.error || !equalCell(draft.expected, draft.value));
    const reason = this.state.busy ? 'Wait for or cancel the soil operation.'
      : this.state.blocked ? 'Recover the exact unknown soil request from durable history; do not repeat Save.'
      : this.state.acknowledgementRequired ? 'Acknowledge the committed soil receipt without erasing it.'
      : invalid ?? (!this.writingEnabled ? 'Soil writing is disabled.' :
        !this.state.original ? 'Load the exact physical soil original.' : !dirty ? 'No changed soil data cells.' : '');
    return { unsaved: dirty || this.state.blocked || this.state.acknowledgementRequired,
      busy: this.state.busy, blocked: this.state.blocked || this.state.acknowledgementRequired || !!invalid,
      canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private idle() {
    if (this.disposed) throw new Error('Soil session has been disposed.');
    if (this.state.busy) throw new Error('Wait for or cancel the soil operation.');
  }
  private known() {
    this.idle();
    if (this.state.blocked) throw new Error('Resolve the retained unknown soil request without replacing it.');
    if (this.state.acknowledgementRequired) throw new Error('Acknowledge the committed soil receipt without erasing it.');
  }
  private begin() {
    const generation = ++this.generation;
    this.state.busy = true; this.state.error = null; this.changed();
    return generation;
  }
  private finish(generation: number) {
    if (generation !== this.generation) return;
    this.state.busy = false; this.changed();
  }
  async load(): Promise<boolean> {
    this.known();
    if (this.closeState().unsaved) throw new Error('Correct or Undo retained soil drafts before reloading.');
    const generation = this.begin();
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.read());
      if (generation !== this.generation) return false;
      const original = twoPageSoilOriginalFromWire(wire, this.owner);
      await this.port.validationYield?.();
      if (generation !== this.generation) return false;
      this.state.original = original;
      this.state.drafts = {};
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Soil original unavailable; prior state retained: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  stage(rowId: string, column: string, raw: string, nullValue: boolean) {
    this.known();
    if (!this.writingEnabled) throw new Error('Soil writing is disabled.');
    if (!this.state.original) throw new Error('Load the exact soil original before staging.');
    const draft = stageTwoPageSoil(this.state.original, rowId, column, raw, nullValue);
    const key = JSON.stringify([rowId, column]);
    if (!draft.error && equalCell(draft.expected, draft.value)) delete this.state.drafts[key];
    else this.state.drafts[key] = draft;
    this.state.error = Object.values(this.state.drafts).find(draft => draft.error)?.error ?? null;
    this.changed();
  }
  undo() {
    this.known();
    this.state.drafts = {}; this.state.error = null; this.changed();
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++;
    this.reads.cancelAll(); this.state.busy = false;
    if (this.pending && this.attempted) {
      this.state.blocked = true;
      this.state.error = 'Soil Save delivery cancelled; exact request retained. Recover durable history without replay.';
    } else {
      this.pending = null;
      this.state.error = 'Soil operation cancelled before mutation dispatch; prior original and drafts retained.';
    }
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.known();
    if (this.retiredRequests.has(requestId)) throw new Error('This soil UUID was certified without commit; use a new UUID for an explicit new Save.');
    const close = this.closeState();
    if (!close.canSave || !this.state.original) throw new Error(close.saveReason);
    const request = twoPageSoilRequest(this.state.original, this.owner, this.state.drafts, requestId);
    this.pending = { request, json: JSON.stringify(request) };
    this.attempted = false;
    return this.accept(false);
  }
  async resolve(): Promise<boolean> {
    this.idle();
    if (!this.pending || !this.state.blocked) throw new Error('No exact unknown soil request is retained.');
    if (!this.dispatchSettled) throw new Error('Wait for the retained Save transport to settle before read-only recovery.');
    return this.accept(true);
  }
  private async accept(recovery: boolean): Promise<boolean> {
    const pending = this.pending;
    if (!pending) throw new Error('Stable soil request is unavailable.');
    const generation = this.begin();
    try {
      if (generation !== this.generation) return false;
      if (!recovery) { this.attempted = true; this.dispatchSettled = false; }
      let wire: unknown;
      try {
        wire = await this.reads.track(recovery ? this.port.outcome(pending.json) : this.port.save(pending.json));
      } finally { if (!recovery) this.dispatchSettled = true; }
      if (generation !== this.generation) return false;
      if (wire === null) throw new Error('Missing history is unresolved; neither cancellation nor matching values authorizes replay.');
      let receipt: TwoPageSoilReceipt | null = null;
      let noCommit: { requestId: string; reason: string } | null = null;
      if (recovery) {
        const outcome = twoPageSoilOutcomeFromWire(wire, pending.request, pending.json, this.owner);
        if (outcome.status === 'notCommitted') noCommit = { requestId: outcome.requestId, reason: outcome.reason };
        else if (outcome.status === 'committed') receipt = outcome.receipt;
        else throw new Error(outcome.reason);
      } else receipt = twoPageSoilReceiptFromWire(wire, pending.request, this.owner);
      await this.port.validationYield?.();
      if (generation !== this.generation) return false;
      if (noCommit) {
        this.retiredRequests.add(pending.request.requestId);
        this.state.noCommitCertificate = noCommit;
        this.state.blocked = false;
        this.state.error = `${noCommit.reason} Drafts retained for correction or Undo; an explicit new Save requires a new UUID.`;
        this.pending = null; this.attempted = false;
        return true;
      }
      if (!receipt) throw new Error('Committed soil outcome requires its complete durable receipt.');
      this.state.receipts.push(receipt);
      this.state.original = { ...pending.request.original, records: receipt.history.Committed };
      this.state.drafts = {};
      this.state.revision++;
      this.state.blocked = false;
      this.state.acknowledgementRequired = true;
      this.pending = null; this.attempted = false;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        if (this.attempted) {
          this.state.blocked = true;
          this.state.error = `Soil Save outcome remains unknown; original/drafts/request retained without replay: ${String(cause)}`;
        } else {
          this.pending = null;
          this.state.error = `Soil request failed before dispatch; retained drafts require correction: ${String(cause)}`;
        }
      }
      return false;
    } finally { this.finish(generation); }
  }
  acknowledge() {
    this.idle();
    if (this.state.blocked || !this.state.acknowledgementRequired) throw new Error('Only a verified committed soil receipt can be acknowledged.');
    this.state.acknowledgementRequired = false; this.state.error = null; this.changed();
  }
  dispose() {
    this.known();
    if (this.closeState().unsaved) throw new Error('Correct or Undo retained soil drafts before disposal.');
    this.generation++; this.reads.cancelAll(); this.listeners.clear(); this.disposed = true;
  }
}
