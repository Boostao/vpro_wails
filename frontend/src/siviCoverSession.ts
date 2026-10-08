import type { AuditRestoreAction, AuditRestoreResult, SIVIVegetationProjection } from '../bindings/github.com/boostao/vpro-wails';
import type { EditorCloseState } from './closeLifecycle';
import { exactSigned64 } from './projectMetadataRestore';
import { siviHeightPresentation, type SIVIProjection } from './siviHeightEditor';
import { siviProjectionFromWire } from './siviHeightSession';
import {
  stageSIVICover, siviCoverDirty, siviCoverErrors, siviCoverHiddenDrafts, siviCoverEdits, siviCoverRequestJSON,
  type SIVICoverColumn, type SIVICoverDrafts,
} from './siviCoverEditor';

export interface SIVICoverWriteResult { ChangedCells: number; HistoryID: string }
// The primary owns the context/plot-bound adapter and generated service bindings.
export interface SIVICoverPort {
  read(extended: boolean): PromiseLike<SIVIVegetationProjection[] | null>;
  save(extended: boolean, requestJSON: string): PromiseLike<SIVICoverWriteResult | null>;
  restore(historyId: string, action: AuditRestoreAction): PromiseLike<AuditRestoreResult | null>;
  refreshParent(): PromiseLike<void>;
}
export interface SIVICoverView {
  review: SIVIProjection[] | null;
  drafts: SIVICoverDrafts;
  extended: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  historyId: string | null;
}

export class SIVICoverSession {
  private state: SIVICoverView;
  private disposed = false;

  constructor(private readonly plot: string, private readonly port: SIVICoverPort,
    private readonly notify: () => void, extended = false) {
    this.state = { review: null, drafts: {}, extended, busy: false, blocked: false, error: null, historyId: null };
  }

  view(): SIVICoverView { return structuredClone(this.state); }

  closeState(): EditorCloseState {
    const invalid = siviCoverErrors(this.state.drafts);
    const hidden = siviCoverHiddenDrafts(this.state.drafts, this.state.extended);
    const saveReason = this.state.busy ? 'Wait for the SIVI cover operation to finish.'
      : this.state.blocked ? 'SIVI cover acknowledgement or refresh failed. Undo/reload explicitly; do not replay.'
      : hidden ? 'Show extended B3/B4/B5 controls before saving their retained drafts.'
      : invalid.length ? `Correct invalid SIVI covers or Undo before saving. ${invalid[0]}`
      : !this.state.review ? 'Load original SIVI source rows before saving.' : '';
    return { unsaved: this.state.blocked || hidden || siviCoverDirty(this.state.drafts), busy: this.state.busy,
      blocked: this.state.blocked || hidden || invalid.length > 0,
      canSave: saveReason === '', saveReason, error: this.state.error };
  }

  private changed() { if (!this.disposed) this.notify(); }
  private requireIdle() {
    if (this.disposed) throw new Error('SIVI cover editor ownership ended; reopen the plot.');
    if (this.state.busy) throw new Error('Wait for the SIVI cover operation to finish.');
  }
  private requireOwned() {
    if (this.disposed) throw new Error('SIVI cover editor ownership ended during the operation.');
  }

  stage(rowId: string, column: SIVICoverColumn, raw: string, nullValue: boolean) {
    this.requireIdle();
    if (this.state.blocked || !this.state.review) throw new Error('SIVI cover source is unavailable; Undo/reload before editing.');
    this.state.drafts = stageSIVICover(this.state.review, this.state.drafts, rowId, column, raw, nullValue);
    this.state.error = siviCoverErrors(this.state.drafts)[0] ?? null;
    this.changed();
  }

  presentation(extended: boolean) {
    this.requireIdle();
    if (this.state.review) this.state.review = siviHeightPresentation(this.state.review, this.plot, extended);
    this.state.extended = extended;
    this.changed();
  }

  async load(): Promise<boolean> {
    this.requireIdle();
    if (this.closeState().unsaved) throw new Error('Finish or Undo SIVI cover drafts before reloading original rows.');
    return this.reload(false);
  }
  private async readCurrent() {
    this.requireOwned();
    const wire = await this.port.read(this.state.extended);
    this.requireOwned();
    this.state.review = siviProjectionFromWire(wire, this.plot, this.state.extended);
  }
  private async refresh() {
    this.requireOwned();
    await this.port.refreshParent();
    this.requireOwned();
    await this.readCurrent();
  }
  private async reload(refreshParent: boolean): Promise<boolean> {
    this.state.busy = true;
    this.changed();
    try {
      if (refreshParent) await this.refresh();
      else await this.readCurrent();
      this.state.blocked = false;
      this.state.error = null;
      return true;
    } catch (cause) {
      this.state.review = null;
      this.state.error = `SIVI cover reload failed: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }
  async undo(): Promise<boolean> {
    this.requireIdle();
    this.state.drafts = {};
    return this.reload(true);
  }

  async save(): Promise<boolean> {
    this.requireIdle();
    const close = this.closeState();
    if (!close.canSave || !this.state.review) throw new Error(close.saveReason);
    const count = siviCoverEdits(this.state.review, this.state.drafts).length;
    if (!count) {
      this.state.drafts = {};
      this.state.error = null;
      this.changed();
      return true;
    }
    const requestJSON = siviCoverRequestJSON(this.state.review, this.state.drafts);
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.save(this.state.extended, requestJSON);
      if (!result || result.ChangedCells !== count || typeof result.HistoryID !== 'string' ||
          result.HistoryID !== '' && (!exactSigned64(result.HistoryID) || BigInt(result.HistoryID) <= 0)) {
        this.state.blocked = true;
        this.state.historyId = null;
        throw new Error('SIVI cover Save acknowledgement is incomplete; Undo/reload, never replay.');
      }
      committed = true;
      this.state.historyId = result.HistoryID || null;
      this.state.drafts = {};
      await this.refresh();
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes('SIVI cover edit committed but cleanup failed;');
      this.state.blocked ||= committed || cleanup;
      if (cleanup) this.state.historyId = null;
      if (this.state.blocked) this.state.review = null;
      this.state.error = this.state.blocked
        ? `SIVI cover acknowledgement/refresh unresolved. Undo/reload; do not replay: ${String(cause)}`
        : `SIVI cover Save failed; drafts retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }

  async restore(action: AuditRestoreAction): Promise<boolean> {
    this.requireIdle();
    if (action !== 'retain' && action !== 'prune') throw new Error('SIVI cover restoration requires explicit retain or prune.');
    if (this.closeState().unsaved || !this.state.historyId || !this.state.review) {
      throw new Error('Finish or Undo drafts and select this session\'s verified cover history before restoring.');
    }
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.restore(this.state.historyId, action);
      if (!result || !Number.isSafeInteger(result.restoredRows) || result.restoredRows <= 0 || result.cleanedVegRows !== 0 ||
          result.cancelled !== false || result.prunedAuditRows !== (action === 'prune' ? result.restoredRows : 0)) {
        this.state.blocked = true;
        this.state.historyId = null;
        throw new Error('SIVI cover restoration acknowledgement is incomplete; Undo/reload, never replay.');
      }
      committed = true;
      this.state.historyId = null;
      await this.refresh();
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes('SIVI restoration committed but cleanup failed;') ||
        String(cause).includes('SIVI cover restoration committed but cleanup failed;');
      this.state.blocked ||= committed || cleanup;
      if (cleanup) this.state.historyId = null;
      if (this.state.blocked) this.state.review = null;
      this.state.error = this.state.blocked
        ? `SIVI cover restoration acknowledgement/refresh unresolved. Undo/reload; do not replay: ${String(cause)}`
        : `SIVI cover restoration failed; history retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }

  dispose() { this.disposed = true; }
}
