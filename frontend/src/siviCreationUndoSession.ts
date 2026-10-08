import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { siviCreationUndoReviewFromWire, siviCreationUndoRequest, siviCreationUndoReceiptFromWire,
  type SIVICreationUndoReview, type SIVICreationUndoRequest, type SIVICreationUndoReceipt } from './siviCreationUndo';
import { siviCreationUndoHistoryFromWire, type SIVICreationUndoHistory } from './siviCreationUndoHistory';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVICreationUndoPort {
  history(): CancellableRead<unknown>;
  review(historyId: string): CancellableRead<unknown>;
  undo(requestJSON: string): CancellableRead<unknown>;
  receipt(requestJSON: string): CancellableRead<unknown>;
}
export interface SIVICreationUndoView {
  history: SIVICreationUndoHistory | null;
  review: SIVICreationUndoReview | null;
  action: 'retain' | 'prune' | null;
  confirmed: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  requestId: string | null;
  receipt: SIVICreationUndoReceipt | null;
  revision: number;
}
export class SIVICreationUndoSession {
  private readonly owner: SIVISpeciesOwner;
  private state: SIVICreationUndoView = { history: null, review: null, action: null, confirmed: false,
    busy: false, blocked: false, error: null, requestId: null, receipt: null, revision: 0 };
  private pending: SIVICreationUndoRequest | null = null;
  private generation = 0;
  private readonly requests = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: SIVISpeciesOwner, private readonly port: SIVICreationUndoPort) {
    this.owner = { contextId: owner.contextId, project: owner.project, plot: owner.plot };
  }
  subscribe(notify: () => void) {
    this.listeners.add(notify);
    notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): SIVICreationUndoView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const reason = this.state.busy ? 'Wait for or cancel the creation Undo acknowledgement.'
      : this.state.blocked ? 'Resolve the retained creation Undo request read-only; do not repeat Undo.'
      : !this.state.review ? 'Review an explicit unconsumed creation from loaded owned history.'
      : !this.state.action ? 'Choose whether to retain or prune the exact creation source audits.'
      : !this.state.confirmed ? 'Confirm removing the exact reviewed physical row and historical44 cells.' : '';
    return { unsaved: this.state.review !== null || this.state.blocked, busy: this.state.busy,
      blocked: this.state.blocked, canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private requireIdle() {
    if (this.state.busy) throw new Error('Wait for or cancel the creation Undo acknowledgement.');
  }
  private requireKnown() {
    this.requireIdle();
    if (this.state.blocked) throw new Error('Resolve the retained creation Undo outcome without replacing its review or request.');
  }
  async loadHistory(): Promise<boolean> {
    this.requireKnown();
    if (this.state.review) throw new Error('Explicitly discard the unsubmitted review before reloading history.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.history());
      if (generation !== this.generation) return false;
      this.state.history = siviCreationUndoHistoryFromWire(wire, this.owner);
      this.state.error = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Creation history read failed; prior owned choices retained without inference: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  async review(historyId: string): Promise<boolean> {
    this.requireKnown();
    if (this.state.review) throw new Error('Explicitly discard the unsubmitted review before selecting another creation.');
    const event = this.state.history?.events.find(event => event.historyId === historyId);
    if (!event || event.consumed || event.reviewAvailable !== true) {
      throw new Error('Select an explicit unconsumed review-available event from loaded owned creation history.');
    }
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.review(historyId));
      if (generation !== this.generation) return false;
      const review = siviCreationUndoReviewFromWire(wire, this.owner, historyId);
      if (review.form !== event.form || review.rowId !== event.rowId || review.id !== event.id ||
        review.creation.request.species !== event.species || review.creation.actor !== event.actor ||
        review.creation.editWhen !== event.editWhen) throw new Error('Fresh review differs from the explicitly selected historical identity.');
      this.state.review = review;
      this.state.action = null;
      this.state.confirmed = false;
      this.state.receipt = null;
      this.state.error = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Creation Undo review failed; no eligibility was inferred: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  choose(action: 'retain' | 'prune') {
    this.requireKnown();
    if (!this.state.review || action !== 'retain' && action !== 'prune') throw new Error('Choose exact retain or prune for the retained creation review.');
    this.state.action = action;
    this.state.confirmed = false;
    this.state.error = null;
    this.changed();
  }
  confirm(confirmed: boolean) {
    this.requireKnown();
    if (!this.state.review || !this.state.action || typeof confirmed !== 'boolean') {
      throw new Error('Confirmation requires the retained creation review and explicit audit action.');
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
    this.state.busy = false;
    if (this.pending) {
      this.state.blocked = true;
      this.state.error = 'Creation Undo acknowledgement cancelled; exact review/request remain unknown. Resolve read-only without replay. Cancellation is not proof of SQL cancellation.';
    } else this.state.error = 'Creation Undo read cancelled; no history or eligibility was inferred.';
    try { this.requests.cancelAll(); } finally { this.changed(); }
  }
  async undo(requestId: string): Promise<boolean> {
    this.requireKnown();
    const close = this.closeState();
    if (!close.canSave || !this.state.review || !this.state.action) throw new Error(close.saveReason);
    this.pending = siviCreationUndoRequest(this.state.review, this.owner, requestId, this.state.action, this.state.confirmed);
    this.state.requestId = requestId;
    return this.acceptReceipt('undo');
  }
  async resolve(): Promise<boolean> {
    this.requireIdle();
    if (!this.pending || !this.state.blocked) throw new Error('No unknown creation Undo request is retained to resolve.');
    return this.acceptReceipt('lookup');
  }
  private async acceptReceipt(operation: 'undo' | 'lookup'): Promise<boolean> {
    const request = this.pending, review = this.state.review;
    if (!request || !review) throw new Error('The retained creation Undo review/request is unavailable.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    try {
      const json = JSON.stringify(request);
      const wire = await this.requests.track(operation === 'undo' ? this.port.undo(json) : this.port.receipt(json));
      if (generation !== this.generation) return false;
      if (wire === null && operation === 'lookup') throw new Error('Missing creation Undo receipt is unresolved, not proof that Undo failed.');
      this.state.receipt = siviCreationUndoReceiptFromWire(wire, review, request, this.owner, operation);
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
        this.state.error = `Creation Undo outcome remains unknown; exact review/request retained without replay: ${String(cause)}`;
      }
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
}
