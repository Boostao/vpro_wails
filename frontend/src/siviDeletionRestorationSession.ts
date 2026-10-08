import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { siviDeletionRestorationReviewFromWire, siviDeletionRestorationRequest,
  siviDeletionRestorationReceiptFromWire, type SIVIDeletionRestorationReview,
  type SIVIDeletionRestorationRequest, type SIVIDeletionRestorationReceipt } from './siviDeletionRestoration';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';
import { siviDeletionHistoryFromWire, type SIVIDeletionHistory } from './siviDeletionHistory';

export interface SIVIDeletionRestorationPort {
  history(): CancellableRead<unknown>;
  review(historyId: string): CancellableRead<unknown>;
  restore(requestJSON: string): CancellableRead<unknown>;
  receipt(requestJSON: string): CancellableRead<unknown>;
}
export interface SIVIDeletionRestorationView {
  history: SIVIDeletionHistory | null;
  review: SIVIDeletionRestorationReview | null;
  action: 'retain' | 'prune' | null;
  confirmed: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  requestId: string | null;
  receipt: SIVIDeletionRestorationReceipt | null;
  revision: number;
}
export class SIVIDeletionRestorationSession {
  private readonly owner: SIVISpeciesOwner;
  private state: SIVIDeletionRestorationView = { history: null, review: null, action: null, confirmed: false,
    busy: false, blocked: false, error: null, requestId: null, receipt: null, revision: 0 };
  private pending: SIVIDeletionRestorationRequest | null = null;
  private generation = 0;
  private readonly requests = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: SIVISpeciesOwner, private readonly port: SIVIDeletionRestorationPort) {
    this.owner = { contextId: owner.contextId, project: owner.project, plot: owner.plot };
  }
  subscribe(notify: () => void) {
    this.listeners.add(notify);
    notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): SIVIDeletionRestorationView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const reason = this.state.busy ? 'Wait for or cancel the deleted-row restoration operation.'
      : this.state.blocked ? 'Resolve the retained restoration request read-only; do not repeat Restore.'
      : !this.state.review ? 'Review the exact deleted row and its current vacancy before restoring.'
      : !this.state.action ? 'Choose whether to retain or prune the exact deletion source audits.'
      : !this.state.confirmed ? 'Confirm restoring the exact reviewed physical row and historical44 cells.' : '';
    return { unsaved: this.state.review !== null || this.state.blocked, busy: this.state.busy,
      blocked: this.state.blocked, canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private requireIdle() {
    if (this.state.busy) throw new Error('Wait for or cancel the deleted-row restoration operation.');
  }
  private requireKnown() {
    this.requireIdle();
    if (this.state.blocked) throw new Error('Resolve the retained restoration outcome without replacing its review or request.');
  }
  async loadHistory(): Promise<boolean> {
    this.requireKnown();
    if (this.state.review) throw new Error('Explicitly Undo the reviewed restoration before reloading historical choices.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.history());
      if (generation !== this.generation) return false;
      this.state.history = siviDeletionHistoryFromWire(wire, this.owner);
      this.state.error = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Deletion history read failed; prior owned choices retained without inference: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  async review(historyId: string): Promise<boolean> {
    this.requireKnown();
    if (this.state.review) throw new Error('Explicitly Undo the unsubmitted restoration review before choosing another history.');
    const event = this.state.history?.events.find(event => event.historyId === historyId);
    if (!event || event.consumed) throw new Error('Select an explicit unconsumed event from loaded owned deletion history.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.review(historyId));
      if (generation !== this.generation) return false;
      this.state.review = siviDeletionRestorationReviewFromWire(wire, this.owner, historyId);
      this.state.action = null;
      this.state.confirmed = false;
      this.state.receipt = null;
      this.state.error = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Deleted-row restoration review failed; no vacancy or history was inferred: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  choose(action: 'retain' | 'prune') {
    this.requireKnown();
    if (!this.state.review || action !== 'retain' && action !== 'prune') throw new Error('Choose exact retain or prune for the retained restoration review.');
    this.state.action = action;
    this.state.confirmed = false;
    this.state.error = null;
    this.changed();
  }
  confirm(confirmed: boolean) {
    this.requireKnown();
    if (!this.state.review || !this.state.action || typeof confirmed !== 'boolean') {
      throw new Error('Restoration confirmation requires the retained review and explicit audit action.');
    }
    this.state.confirmed = confirmed;
    this.state.error = null;
    this.changed();
  }
  discard() {
    this.requireKnown();
    this.state.review = null;
    this.state.action = null;
    this.state.confirmed = false;
    this.state.error = null;
    this.changed();
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++;
    this.requests.cancelAll();
    this.state.busy = false;
    if (this.pending) {
      this.state.blocked = true;
      this.state.error = 'Restoration acknowledgement cancelled; exact review/request remain unknown. Resolve read-only history without replay.';
    } else this.state.error = 'Restoration review read cancelled; no vacancy or confirmation was inferred.';
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.requireKnown();
    const close = this.closeState();
    if (!close.canSave || !this.state.review || !this.state.action) throw new Error(close.saveReason);
    this.pending = siviDeletionRestorationRequest(this.state.review, this.owner, requestId, this.state.action, this.state.confirmed);
    this.state.requestId = requestId;
    return this.acceptReceipt('restore');
  }
  async resolve(): Promise<boolean> {
    this.requireIdle();
    if (!this.pending || !this.state.blocked) throw new Error('No unknown restoration request is retained to resolve.');
    return this.acceptReceipt('lookup');
  }
  private async acceptReceipt(operation: 'restore' | 'lookup'): Promise<boolean> {
    const request = this.pending, review = this.state.review;
    if (!request || !review) throw new Error('The retained restoration review/request is unavailable.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    try {
      const json = JSON.stringify(request);
      const wire = await this.requests.track(operation === 'restore' ? this.port.restore(json) : this.port.receipt(json));
      if (generation !== this.generation) return false;
      if (wire === null && operation === 'lookup') throw new Error('Missing restoration history is unresolved, not proof that Restore failed.');
      this.state.receipt = siviDeletionRestorationReceiptFromWire(wire, review, request, this.owner, operation);
      this.state.history = null;
      this.state.review = null;
      this.state.action = null;
      this.state.confirmed = false;
      this.state.blocked = false;
      this.state.requestId = null;
      this.state.revision++;
      this.pending = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        this.state.blocked = true;
        this.state.error = `Restoration outcome remains unknown; exact review/request retained without replay: ${String(cause)}`;
      }
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
}
