import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { siviDeletionOriginalFromWire, siviDeletionRequest, siviDeletionUnavailable, siviDeletionTargetsFromWire,
  type SIVIDeletionOriginal, type SIVIDeletionRequest, type SIVIDeletionTarget } from './siviDeletionEditor';
import { siviDeletionReceiptFromWire, type SIVIDeletionReceipt } from './siviDeletionReceipt';
import type { SIVICreationForm } from './siviCreationEditor';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

export interface SIVIDeletionPort {
  targets(): CancellableRead<unknown>;
  original(form: SIVICreationForm, rowId: string): CancellableRead<unknown>;
  delete(requestJSON: string): CancellableRead<unknown>;
  receipt(requestJSON: string): CancellableRead<unknown>;
}
export interface SIVIDeletionView {
  targets: SIVIDeletionTarget[] | null;
  original: SIVIDeletionOriginal | null;
  confirmed: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  requestId: string | null;
  receipt: SIVIDeletionReceipt | null;
  revision: number;
}
export class SIVIDeletionSession {
  private readonly owner: SIVISpeciesOwner;
  private state: SIVIDeletionView = { targets: null, original: null, confirmed: false, busy: false, blocked: false,
    error: null, requestId: null, receipt: null, revision: 0 };
  private pending: SIVIDeletionRequest | null = null;
  private generation = 0;
  private readonly requests = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: SIVISpeciesOwner, private readonly port: SIVIDeletionPort) {
    this.owner = { contextId: owner.contextId, project: owner.project, plot: owner.plot };
  }
  subscribe(notify: () => void) {
    this.listeners.add(notify);
    notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): SIVIDeletionView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const reason = this.state.busy ? 'Wait for or cancel the SIVI deletion operation.'
      : this.state.blocked ? 'Resolve the retained deletion request using read-only history; do not repeat Delete.'
      : !this.state.original ? 'Review one exact physical source row before deleting.'
      : siviDeletionUnavailable(this.state.original) ??
        (!this.state.confirmed ? 'Confirm deletion of the exact reviewed physical row and all44 historical cells.' : '');
    return { unsaved: this.state.original !== null || this.state.blocked, busy: this.state.busy,
      blocked: this.state.blocked || this.state.original !== null && siviDeletionUnavailable(this.state.original) !== null,
      canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private requireIdle() {
    if (this.state.busy) throw new Error('Wait for or cancel the SIVI deletion operation.');
  }
  private requireKnown() {
    this.requireIdle();
    if (this.state.blocked) throw new Error('Resolve the retained deletion outcome without replacing its request or original.');
  }
  async loadTargets(): Promise<boolean> {
    this.requireKnown();
    if (this.state.original) throw new Error('Explicitly Undo the reviewed deletion before reloading source targets.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.targets());
      if (generation !== this.generation) return false;
      this.state.targets = siviDeletionTargetsFromWire(wire, this.owner);
      this.state.error = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Deletion source read failed; prior targets retained without inference: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  async review(form: SIVICreationForm, rowId: string): Promise<boolean> {
    this.requireKnown();
    if (this.state.original) throw new Error('Explicitly Undo the reviewed deletion before choosing another source row.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.changed();
    try {
      const wire = await this.requests.track(this.port.original(form, rowId));
      if (generation !== this.generation) return false;
      this.state.original = siviDeletionOriginalFromWire(wire, this.owner, form, rowId);
      this.state.confirmed = false;
      this.state.receipt = null;
      this.state.error = siviDeletionUnavailable(this.state.original);
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `SIVI deletion review failed; no original was inferred: ${String(cause)}`;
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
  confirm(confirmed: boolean) {
    this.requireKnown();
    if (!this.state.original || typeof confirmed !== 'boolean') throw new Error('Deletion confirmation requires the retained reviewed original.');
    this.state.confirmed = confirmed;
    this.state.error = siviDeletionUnavailable(this.state.original);
    this.changed();
  }
  discard() {
    this.requireKnown();
    this.state.original = null;
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
      this.state.error = 'Deletion acknowledgement cancelled; exact original/request remain unknown. Resolve read-only history without replay.';
    } else this.state.error = 'Deletion original read cancelled; no original or confirmation was inferred.';
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.requireKnown();
    const close = this.closeState();
    if (!close.canSave || !this.state.original) throw new Error(close.saveReason);
    this.pending = siviDeletionRequest(this.state.original, this.owner, requestId, this.state.confirmed);
    this.state.requestId = requestId;
    return this.acceptReceipt('delete');
  }
  async resolve(): Promise<boolean> {
    this.requireIdle();
    if (!this.pending || !this.state.blocked) throw new Error('No unknown deletion request is retained to resolve.');
    return this.acceptReceipt('lookup');
  }
  private async acceptReceipt(operation: 'delete' | 'lookup'): Promise<boolean> {
    const request = this.pending;
    if (!request) throw new Error('The retained stable deletion request is unavailable.');
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    try {
      const json = JSON.stringify(request);
      const wire = await this.requests.track(operation === 'delete' ? this.port.delete(json) : this.port.receipt(json));
      if (generation !== this.generation) return false;
      if (wire === null && operation === 'lookup') throw new Error('Missing history is unresolved, not proof that deletion failed.');
      this.state.receipt = siviDeletionReceiptFromWire(wire, request, this.owner, operation);
      this.state.targets = null;
      this.state.original = null;
      this.state.confirmed = false;
      this.state.blocked = false;
      this.state.requestId = null;
      this.state.revision++;
      this.pending = null;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        this.state.blocked = true;
        this.state.error = `Deletion outcome remains unknown; original/request retained without automatic replay: ${String(cause)}`;
      }
      return false;
    } finally {
      if (generation === this.generation) { this.state.busy = false; this.changed(); }
    }
  }
}
