import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { pictureImageFromWire, type PictureOwner, type PicturePreview } from './pictureRead';
import { validSIVIRequestId } from './siviRequestId';
import { pictureSelectionFromWire, pictureCreationRequest, pictureCreationReceiptFromWire,
  type PictureSelection, type PictureCreationRequest, type PictureCreationReceipt } from './pictureCreationEditor';

export interface PictureCreationPort {
  choose(): PromiseLike<unknown>;
  recoverSelection(): CancellableRead<unknown>;
  create(token: string, request: string): CancellableRead<unknown>;
  receipt(request: string): CancellableRead<unknown>;
  image(token: string, request: string): CancellableRead<unknown>;
  release(token: string): PromiseLike<void>;
  acknowledge(token: string): PromiseLike<void>;
}
export interface PictureCreationView {
  selection: PictureSelection | null;
  request: PictureCreationRequest | null;
  receipt: PictureCreationReceipt | null;
  receipts: PictureCreationReceipt[];
  image: PicturePreview | null;
  phase: 'idle' | 'choosing' | 'recovering-selection' | 'creating' | 'recovering-receipt' | 'previewing' | 'releasing' | 'acknowledging';
  busy: boolean;
  blocked: boolean;
  error: string | null;
  acknowledged: boolean;
  attempted: boolean;
  noCommitVerified: boolean;
}

export class PictureCreationSession {
  private readonly owner: PictureOwner;
  private state: Omit<PictureCreationView, 'attempted' | 'noCommitVerified'> = { selection: null, request: null, receipt: null, receipts: [], image: null,
    phase: 'idle', busy: false, blocked: false, error: null, acknowledged: false };
  private plannedId: string | null = null;
  private attempted = false;
  private noCommitVerified = false;
  private selectionAbsentVerified = false;
  private generation = 0;
  private readonly reads = new ReadRequests();
  private readonly listeners = new Set<() => void>();

  constructor(owner: PictureOwner, private readonly port: PictureCreationPort) {
    pictureSelectionFromWire(null, owner);
    this.owner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  }
  subscribe(notify: () => void) {
    this.listeners.add(notify);
    notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): PictureCreationView {
    return structuredClone({ ...this.state, attempted: this.attempted, noCommitVerified: this.noCommitVerified });
  }
  closeState(): EditorCloseState {
    return { unsaved: this.state.blocked, busy: this.state.busy, blocked: this.state.blocked, canSave: false,
      saveReason: this.state.phase === 'choosing' ? 'Finish or Cancel the native picture chooser.'
        : this.state.blocked ? 'Recover, release or acknowledge the retained picture creation outcome before leaving.' : '',
      error: this.state.error };
  }
  private idle() {
    if (this.state.busy) throw new Error('Finish the current picture operation first.');
  }
  private begin(phase: PictureCreationView['phase']) {
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.phase = phase;
    this.state.error = null;
    this.changed();
    return generation;
  }
  private finish(generation: number) {
    if (generation !== this.generation) return;
    this.state.busy = false;
    this.state.phase = 'idle';
    this.changed();
  }
  private committed(wire: unknown, request: PictureCreationRequest) {
    this.state.receipt = pictureCreationReceiptFromWire(wire, request, this.owner);
    if (!this.state.receipts.some(receipt => receipt.requestId === request.requestId)) {
      this.state.receipts = [...this.state.receipts, this.state.receipt];
    }
    this.state.blocked = true;
    this.state.acknowledged = false;
    this.noCommitVerified = false;
  }
  cancel() {
    if (!this.state.busy) return;
    if (this.state.phase === 'choosing' || this.state.phase === 'releasing' || this.state.phase === 'acknowledging') {
      throw new Error('Finish this operation in its native dialog or wait for its capability acknowledgement; promise cancellation cannot dismiss it.');
    }
    const phase = this.state.phase;
    this.generation++;
    this.reads.cancelAll();
    this.state.busy = false;
    this.state.phase = 'idle';
    this.state.blocked = true;
    this.state.error = phase === 'previewing' ? 'Picture preview cancelled; the committed receipt is retained.'
      : 'Picture operation acknowledgement cancelled; recover the retained outcome without repeating Create.';
    this.changed();
  }
  async add(requestId: string): Promise<boolean> {
    this.idle();
    if (this.state.blocked || !validSIVIRequestId(requestId)) {
      throw new Error('Resolve the retained picture outcome before starting a new stable creation request.');
    }
    this.plannedId = requestId;
    this.attempted = false;
    this.noCommitVerified = false;
    this.selectionAbsentVerified = false;
    this.state = { ...this.state, selection: null, request: null, receipt: null, image: null, acknowledged: false, blocked: true };
    const generation = this.begin('choosing');
    let selected = false;
    try {
      if (generation !== this.generation) return false;
      const wire = await this.port.choose();
      if (generation !== this.generation) return false;
      this.state.selection = pictureSelectionFromWire(wire, this.owner);
      if (this.state.selection) {
        this.state.request = pictureCreationRequest(this.state.selection, this.owner, requestId);
        selected = true;
      } else {
        this.state.blocked = false;
        this.plannedId = null;
      }
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Picture selection outcome unavailable; recover the selection, not a new chooser: ${String(cause)}`;
    } finally { this.finish(generation); }
    return selected ? this.create() : false;
  }
  async recoverSelection(): Promise<boolean> {
    this.idle();
    if (!this.state.blocked) throw new Error('No retained picture selection requires recovery.');
    const generation = this.begin('recovering-selection');
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.recoverSelection());
      if (generation !== this.generation) return false;
      const recovered = pictureSelectionFromWire(wire, this.owner);
      if (recovered && ((this.state.request && recovered.selectedPath !== this.state.request.selectedPath) ||
          (this.state.selection && recovered.token !== this.state.selection.token))) {
        throw new Error('Recovered selection differs from the retained literal or capability; do not replace its identity.');
      }
      this.state.selection = recovered;
      this.selectionAbsentVerified = recovered === null;
      if (this.state.selection && !this.state.request) {
        if (!this.plannedId) throw new Error('The original planned creation UUID is unavailable; do not infer a new request.');
        this.state.request = pictureCreationRequest(this.state.selection, this.owner, this.plannedId);
      } else if (!this.state.selection && !this.attempted) {
        this.state.request = null;
        this.state.blocked = false;
        this.plannedId = null;
      }
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Picture selection recovery failed; retained state remains blocked: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async create(): Promise<boolean> {
    this.idle();
    const selection = this.state.selection, request = this.state.request;
    if (!selection || !request || this.attempted || this.state.receipt) {
      throw new Error('Create requires one recovered, unattempted selection; resolve durable history instead of replaying.');
    }
    this.attempted = true;
    const generation = this.begin('creating');
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.create(selection.token, JSON.stringify(request)));
      if (generation !== this.generation) return false;
      this.committed(wire, request);
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Picture Create outcome is unknown; recover its exact durable receipt, never repeat Create: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async recoverReceipt(): Promise<boolean> {
    this.idle();
    const request = this.state.request;
    if (!request || !this.attempted) throw new Error('No attempted picture creation request requires receipt recovery.');
    this.noCommitVerified = false;
    const generation = this.begin('recovering-receipt');
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.receipt(JSON.stringify(request)));
      if (generation !== this.generation) return false;
      if (wire === null) {
        this.noCommitVerified = true;
        this.state.error = 'No durable receipt was found; explicitly release the selection rather than repeat this request.';
      } else this.committed(wire, request);
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Picture receipt recovery failed; the original request remains blocked: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async preview(): Promise<boolean> {
    this.idle();
    const selection = this.state.selection, request = this.state.request, receipt = this.state.receipt;
    if (!selection || !request || !receipt || this.state.acknowledged) {
      throw new Error('Selected-file preview requires the retained capability and verified committed receipt.');
    }
    const generation = this.begin('previewing');
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.image(selection.token, JSON.stringify(request)));
      if (generation !== this.generation) return false;
      this.state.image = pictureImageFromWire(wire, this.owner, receipt.rowId);
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Selected-file preview unavailable; metadata commit and receipt retained: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async release(): Promise<boolean> {
    this.idle();
    const selection = this.state.selection;
    if (this.state.receipt || (this.attempted && !this.noCommitVerified) || (!selection && !this.selectionAbsentVerified)) {
      throw new Error('Resolve durable history before releasing an attempted selection; committed receipts require acknowledgement.');
    }
    const generation = this.begin('releasing');
    try {
      if (generation !== this.generation) return false;
      if (selection) await this.port.release(selection.token);
      this.state.selection = null;
      this.state.request = null;
      this.state.blocked = false;
      this.plannedId = null;
      return true;
    } catch (cause) {
      this.state.error = `Picture selection release acknowledgement failed; recover selection before leaving: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async acknowledge(): Promise<boolean> {
    this.idle();
    if (!this.state.receipt || !this.state.request || (!this.state.selection && !this.selectionAbsentVerified)) {
      throw new Error('Acknowledge requires the verified committed receipt and resolved capability state.');
    }
    const generation = this.begin('acknowledging');
    try {
      if (generation !== this.generation) return false;
      if (this.state.selection) await this.port.acknowledge(this.state.selection.token);
      this.state.selection = null;
      this.state.acknowledged = true;
      this.state.blocked = false;
      return true;
    } catch (cause) {
      this.state.error = `Picture receipt acknowledgement failed; receipt retained and close remains blocked: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
}
