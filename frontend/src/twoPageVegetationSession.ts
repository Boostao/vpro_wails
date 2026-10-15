import type { EditorCloseState } from './closeLifecycle';
import { ReadRequests, type CancellableRead } from './readRequests';
import { equalVegetationCell, serializeTwoPageVegetationRequest, twoPageVegetationOriginalFromWire,
  type TwoPageVegetationOwner, type TwoPageVegetationOriginal, type TwoPageVegetationRequest } from './twoPageVegetationWire';
import { stageTwoPageVegetation, twoPageVegetationRequest, type TwoPageVegetationDrafts } from './twoPageVegetationEditor';
import { twoPageVegetationReceiptFromWire, twoPageVegetationOutcomeFromWire, twoPageVegetationCommittedOriginal,
  twoPageVegetationSpeciesListReceiptFromWire, twoPageVegetationSpeciesListOutcomeFromWire,
  type TwoPageVegetationReceipt } from './twoPageVegetationReceipt';
import { vegetationReferencesFromWire, type VegetationReferences } from './twoPageVegetationReferences';
import { requireVegetationSpeciesListCovers } from './twoPageVegetationSpeciesList';

export interface TwoPageVegetationPort {
  read(): CancellableRead<unknown>;
  save(requestJSON: string): CancellableRead<unknown>;
  saveSpeciesList?(requestJSON: string): CancellableRead<unknown>;
  outcome(requestJSON: string): CancellableRead<unknown>;
  references(): CancellableRead<unknown>;
  validationYield?(): PromiseLike<void>;
}
export interface TwoPageVegetationView {
  original: TwoPageVegetationOriginal | null;
  selectedRowId: string | null;
  drafts: TwoPageVegetationDrafts;
  busy: boolean;
  blocked: boolean;
  acknowledgementRequired: boolean;
  error: string | null;
  receipts: TwoPageVegetationReceipt[];
  revision: number;
  noCommitCertificate: { requestId: string; reason: string } | null;
  references: VegetationReferences | null;
  referenceError: string | null;
}

export class TwoPageVegetationSession {
  private readonly owner: TwoPageVegetationOwner;
  private state: TwoPageVegetationView = { original: null, selectedRowId: null, drafts: {}, busy: false, blocked: false,
    acknowledgementRequired: false, error: null, receipts: [], revision: 0, noCommitCertificate: null,
    references: null, referenceError: null };
  private pending: { request: TwoPageVegetationRequest; json: string } | null = null;
  private attempted = false;
  private dispatchSettled = true;
  private disposed = false;
  private generation = 0;
  private readonly reads = new ReadRequests();
  private readonly listeners = new Set<() => void>();
  private readonly retiredRequests = new Set<string>();
  private readingReferences = false;

  constructor(owner: TwoPageVegetationOwner, private readonly port: TwoPageVegetationPort, private readonly writingEnabled = false,
    private readonly speciesListEnabled = false) {
    this.owner = { ...owner };
  }
  subscribe(notify: () => void) {
    if (this.disposed) throw new Error('Vegetation session has been disposed.');
    this.listeners.add(notify); notify();
    return () => { this.listeners.delete(notify); };
  }
  private changed() { for (const notify of this.listeners) notify(); }
  view(): TwoPageVegetationView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const invalid = Object.values(this.state.drafts).find(draft => draft.error)?.error;
    const dirty = Object.values(this.state.drafts).some(draft => draft.error || !equalVegetationCell(draft.expected, draft.value));
    const reason = this.state.busy ? 'Wait for or cancel the vegetation operation.'
      : this.state.blocked ? 'Recover the exact unknown vegetation request from durable history; do not repeat Save.'
      : this.state.acknowledgementRequired ? 'Acknowledge the committed vegetation receipt without erasing it.'
      : invalid ?? (!this.writingEnabled ? 'Vegetation writing is disabled.' :
        !this.state.original ? 'Load the exact physical vegetation original.' : !dirty ? 'No changed vegetation data cells.' : '');
    return { unsaved: dirty || this.state.blocked || this.state.acknowledgementRequired,
      busy: this.state.busy, blocked: this.state.blocked || this.state.acknowledgementRequired || !!invalid,
      canSave: reason === '', saveReason: reason, error: this.state.error };
  }
  private idle() {
    if (this.disposed) throw new Error('Vegetation session has been disposed.');
    if (this.state.busy) throw new Error('Wait for or cancel the vegetation operation.');
  }
  private known() {
    this.idle();
    if (this.state.blocked) throw new Error('Resolve the retained unknown vegetation request without replacing it.');
    if (this.state.acknowledgementRequired) throw new Error('Acknowledge the committed vegetation receipt without erasing it.');
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
    if (this.closeState().unsaved) throw new Error('Correct or Undo retained vegetation drafts before reloading.');
    const generation = this.begin();
    try {
      if (generation !== this.generation) return false;
      const wire = await this.reads.track(this.port.read());
      if (generation !== this.generation) return false;
      const original = twoPageVegetationOriginalFromWire(wire, this.owner);
      await this.port.validationYield?.();
      if (generation !== this.generation) return false;
      this.state.original = original;
      if (this.state.selectedRowId === null) this.state.selectedRowId = original.sourceRowIds[0] ?? original.records.rows[0]?.rowId ?? null;
      this.state.drafts = {};
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Vegetation original unavailable; prior state retained: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  async loadReferences(): Promise<boolean> {
    this.known();
    if (Object.values(this.state.drafts).some(draft => draft.column === 'Species')) {
      throw new Error('Correct or Undo retained Species drafts before replacing their reviewed reference snapshot.');
    }
    const generation = ++this.generation;
    this.readingReferences = true;
    this.state.busy = true; this.state.references = null; this.state.referenceError = null; this.changed();
    try {
      if (generation !== this.generation) return false;
      if (!this.port.references) throw new Error('Species reference reading is unavailable on this vegetation port.');
      const wire = await this.reads.track(this.port.references());
      if (generation !== this.generation) return false;
      const references = await vegetationReferencesFromWire(wire, this.owner, this.port.validationYield);
      if (generation !== this.generation) return false;
      this.state.references = references;
      return true;
    } catch (cause) {
      if (generation === this.generation) {
        this.state.referenceError = `Physical Species definitions unavailable; no Species choices or writes enabled: ${String(cause)}`;
      }
      return false;
    } finally {
      if (generation === this.generation) this.readingReferences = false;
      this.finish(generation);
    }
  }
  stage(rowId: string, column: string, raw: string, nullValue: boolean) {
    this.known();
    if (!this.writingEnabled) throw new Error('Vegetation writing is disabled.');
    if (!this.state.original) throw new Error('Load the exact vegetation original before staging.');
    if (column === 'Species' && (!this.speciesListEnabled || !this.state.references)) {
      throw new Error('Species list selection requires its independent gate and reviewed physical references.');
    }
    const draft = stageTwoPageVegetation(this.state.original, rowId, column, raw, nullValue,
      this.speciesListEnabled ? this.state.references ?? undefined : undefined);
    const key = JSON.stringify([rowId, column]);
    if (!draft.error && equalVegetationCell(draft.expected, draft.value)) delete this.state.drafts[key];
    else this.state.drafts[key] = draft;
    this.revalidateSpeciesDrafts();
    this.changed();
  }
  private revalidateSpeciesDrafts() {
    if (this.state.original && this.state.references) {
      for (const [key, draft] of Object.entries(this.state.drafts)) {
        if (draft.column === 'Species') {
          this.state.drafts[key] = stageTwoPageVegetation(this.state.original, draft.rowId, draft.column,
            draft.raw, draft.nullValue, this.state.references);
        }
      }
      const validSpecies = Object.values(this.state.drafts).filter(draft => draft.column === 'Species' && !draft.error);
      if (validSpecies.length) {
        try { requireVegetationSpeciesListCovers(this.state.original, Object.values(this.state.drafts)); }
        catch (cause) {
          for (const draft of validSpecies) draft.error = String(cause);
        }
      }
    }
    this.state.error = Object.values(this.state.drafts).find(draft => draft.error)?.error ?? null;
  }
  select(rowId: string) {
    this.idle();
    if (!this.state.original?.records.rows.some(row => row.rowId === rowId)) {
      throw new Error('Select an observed physical vegetation row; no identity can be invented.');
    }
    this.state.selectedRowId = rowId; this.changed();
  }
  undo() {
    this.known();
    this.state.drafts = {}; this.state.error = null; this.changed();
  }
  cancel() {
    if (!this.state.busy) return;
    this.generation++;
    this.reads.cancelAll(); this.state.busy = false;
    if (this.readingReferences) {
      this.readingReferences = false;
      this.state.referenceError = 'Physical Species reference read cancelled; original, drafts and receipts retained. Retry to read current definitions.';
    } else if (this.pending && this.attempted) {
      this.state.blocked = true;
      this.state.error = 'Vegetation Save delivery cancelled; exact request retained. Recover durable history without replay.';
    } else {
      this.pending = null;
      this.state.error = 'Vegetation operation cancelled before mutation dispatch; prior original and drafts retained.';
    }
    this.changed();
  }
  async save(requestId: string): Promise<boolean> {
    this.known();
    if (this.retiredRequests.has(requestId)) throw new Error('This vegetation UUID was certified without commit; use a new UUID for an explicit new Save.');
    if (this.state.receipts.some(receipt => receipt.history.Request.requestId === requestId)) {
      throw new Error('This vegetation UUID already committed; use a new UUID for an explicit new Save.');
    }
    const close = this.closeState();
    if (!close.canSave || !this.state.original) throw new Error(close.saveReason);
    const request = twoPageVegetationRequest(this.state.original, this.owner, this.state.drafts, requestId,
      this.speciesListEnabled ? this.state.references ?? undefined : undefined);
    if (request.speciesList && !this.port.saveSpeciesList) throw new Error('Species list Save transport is unavailable; no request was dispatched.');
    this.pending = { request, json: serializeTwoPageVegetationRequest(request) };
    this.state.noCommitCertificate = null;
    this.attempted = false;
    return this.accept(false);
  }
  async resolve(): Promise<boolean> {
    this.idle();
    if (!this.pending || !this.state.blocked) throw new Error('No exact unknown vegetation request is retained.');
    if (!this.dispatchSettled) throw new Error('Wait for the retained Save transport to settle before read-only recovery.');
    return this.accept(true);
  }
  private async accept(recovery: boolean): Promise<boolean> {
    const pending = this.pending;
    if (!pending) throw new Error('Stable vegetation request is unavailable.');
    const generation = this.begin();
    try {
      if (generation !== this.generation) return false;
      if (!recovery) { this.attempted = true; this.dispatchSettled = false; }
      let wire: unknown;
      try {
        let operation: CancellableRead<unknown>;
        if (recovery) operation = this.port.outcome(pending.json);
        else if (pending.request.speciesList) {
          if (!this.port.saveSpeciesList) throw new Error('Species list Save transport is unavailable; recover the retained request without replay.');
          operation = this.port.saveSpeciesList(pending.json);
        } else operation = this.port.save(pending.json);
        wire = await this.reads.track(operation);
      } finally { if (!recovery) this.dispatchSettled = true; }
      if (generation !== this.generation) return false;
      if (wire === null) throw new Error('Missing history is unresolved; neither cancellation nor matching values authorizes replay.');
      let receipt: TwoPageVegetationReceipt | null = null;
      let noCommit: { requestId: string; reason: string } | null = null;
      if (recovery) {
        const outcome = pending.request.speciesList
          ? await twoPageVegetationSpeciesListOutcomeFromWire(wire, pending.request, pending.json, this.owner, this.port.validationYield)
          : twoPageVegetationOutcomeFromWire(wire, pending.request, pending.json, this.owner);
        if (outcome.status === 'notCommitted') noCommit = { requestId: outcome.requestId, reason: outcome.reason };
        else if (outcome.status === 'committed') receipt = outcome.receipt;
        else throw new Error(outcome.reason);
      } else receipt = pending.request.speciesList
        ? await twoPageVegetationSpeciesListReceiptFromWire(wire, pending.request, this.owner, false, this.port.validationYield)
        : twoPageVegetationReceiptFromWire(wire, pending.request, this.owner);
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
      if (!receipt) throw new Error('Committed vegetation outcome requires its complete durable receipt.');
      const original = twoPageVegetationCommittedOriginal(receipt, this.owner);
      this.state.receipts.push(receipt);
      this.state.original = original;
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
          this.state.error = `Vegetation Save outcome remains unknown; original/drafts/request retained without replay: ${String(cause)}`;
        } else {
          this.pending = null;
          this.state.error = `Vegetation request failed before dispatch; retained drafts require correction: ${String(cause)}`;
        }
      }
      return false;
    } finally { this.finish(generation); }
  }
  acknowledge() {
    this.idle();
    if (this.state.blocked || !this.state.acknowledgementRequired) throw new Error('Only a verified committed vegetation receipt can be acknowledged.');
    this.state.acknowledgementRequired = false; this.state.error = null; this.changed();
  }
  dispose() {
    this.known();
    if (this.closeState().unsaved) throw new Error('Correct or Undo retained vegetation drafts before disposal.');
    this.generation++; this.reads.cancelAll(); this.listeners.clear(); this.disposed = true;
  }
}
