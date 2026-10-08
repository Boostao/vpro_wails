import type { AuditRestoreAction, AuditRestoreResult, SIVIHeightWriteResult, SIVIVegetationProjection } from '../bindings/github.com/boostao/vpro-wails';
import type { EditorCloseState } from './closeLifecycle';
import { exactSigned64 } from './projectMetadataRestore';
import {
  validateSIVIProjection, siviHeightPresentation, stageSIVIHeight, siviHeightDirty,
  siviHeightErrors, siviHeightEdits, type SIVIProjection, type SIVIHeightDrafts,
  type SIVIHeightColumn, type SIVIHeightEdit,
} from './siviHeightEditor';

export interface SIVIHeightPort {
  read(extended: boolean): PromiseLike<SIVIVegetationProjection[] | null>;
  save(extended: boolean, original: SIVIProjection[], edits: SIVIHeightEdit[]): PromiseLike<SIVIHeightWriteResult | null>;
  restore(historyId: string, action: AuditRestoreAction): PromiseLike<AuditRestoreResult | null>;
  refreshParent(): PromiseLike<void>;
}
export interface SIVIHeightView {
  review: SIVIProjection[] | null;
  drafts: SIVIHeightDrafts;
  extended: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  historyId: string | null;
}

export function siviProjectionFromWire(wire: SIVIVegetationProjection[] | null, plot: string, extended: boolean): SIVIProjection[] {
  if (!Array.isArray(wire)) throw new Error('SIVI source groups were not returned.');
  const review = wire.map(group => {
    if (!group || !Array.isArray(group.Columns) || !Array.isArray(group.Rows)) {
      throw new Error('SIVI source columns or rows were not returned.');
    }
    const rows = group.Rows.map(row => {
      if (!row || !Array.isArray(row.cells)) throw new Error('SIVI original physical cells were not returned.');
      return { rowId: row.rowId, cells: row.cells };
    });
    return { Form: group.Form, Query: group.Query, Columns: group.Columns, Rows: rows };
  });
  return validateSIVIProjection(review, plot, extended);
}

export class SIVIHeightSession {
  private state: SIVIHeightView;
  private disposed = false;

  constructor(private readonly plot: string, private readonly port: SIVIHeightPort,
    private readonly notify: () => void, extended = false) {
    this.state = { review: null, drafts: {}, extended, busy: false, blocked: false, error: null, historyId: null };
  }

  view(): SIVIHeightView {
    return structuredClone(this.state);
  }

  closeState(): EditorCloseState {
    const invalid = siviHeightErrors(this.state.drafts);
    const saveReason = this.state.busy ? 'Wait for the SIVI operation to finish.'
      : this.state.blocked ? 'SIVI acknowledgement or refresh failed. Undo/reload explicitly before continuing; do not replay.'
      : invalid.length ? `Correct invalid SIVI heights or Undo before saving. ${invalid[0]}`
      : siviHeightDirty(this.state.drafts) && this.state.review === null ? 'Reload the original SIVI rows before saving.' : '';
    return { unsaved: this.state.blocked || siviHeightDirty(this.state.drafts), busy: this.state.busy,
      blocked: this.state.blocked || invalid.length > 0,
      canSave: saveReason === '', saveReason, error: this.state.error };
  }

  private changed() {
    if (!this.disposed) this.notify();
  }

  private requireIdle() {
    if (this.disposed) throw new Error('SIVI editor ownership ended; reopen the plot.');
    if (this.state.busy) throw new Error('Wait for the SIVI operation to finish.');
  }

  stage(rowId: string, column: SIVIHeightColumn, raw: string, nullValue: boolean) {
    this.requireIdle();
    if (this.state.blocked || !this.state.review) throw new Error('SIVI source is unavailable; Undo/reload before editing.');
    this.state.drafts = stageSIVIHeight(this.state.review, this.state.drafts, rowId, column, raw, nullValue);
    this.state.error = siviHeightErrors(this.state.drafts)[0] ?? null;
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
    if (this.closeState().unsaved) throw new Error('Finish or Undo SIVI drafts before reloading original rows.');
    return this.reload(false);
  }

  private async readCurrent() {
    if (this.disposed) throw new Error('SIVI editor ownership ended before source loading started.');
    const wire = await this.port.read(this.state.extended);
    if (this.disposed) throw new Error('SIVI editor ownership ended before source loading completed.');
    this.state.review = siviProjectionFromWire(wire, this.plot, this.state.extended);
  }

  private async reload(refreshParent: boolean): Promise<boolean> {
    this.state.busy = true;
    this.changed();
    try {
      if (refreshParent) await this.port.refreshParent();
      await this.readCurrent();
      this.state.blocked = false;
      this.state.error = null;
      return true;
    } catch (cause) {
      this.state.review = null;
      this.state.error = `SIVI reload failed: ${String(cause)}`;
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
    if (!close.canSave || !this.state.review) throw new Error(close.saveReason || 'Load original SIVI rows before saving.');
    const edits = siviHeightEdits(this.state.review, this.state.drafts);
    if (!edits.length) {
      this.state.drafts = {};
      this.state.error = null;
      this.changed();
      return true;
    }
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.save(this.state.extended, structuredClone(this.state.review), edits);
      if (!result || result.ChangedCells !== edits.length || typeof result.HistoryID !== 'string' ||
          result.HistoryID !== '' && (!exactSigned64(result.HistoryID) || BigInt(result.HistoryID) <= 0)) {
        this.state.blocked = true;
        this.state.historyId = null;
        throw new Error('SIVI Save acknowledgement is incomplete; Undo/reload before retrying.');
      }
      committed = true;
      this.state.historyId = result.HistoryID || null;
      this.state.drafts = {};
      if (this.disposed) throw new Error('SIVI editor ownership ended after Save committed.');
      await this.port.refreshParent();
      await this.readCurrent();
      return true;
    } catch (cause) {
      const committedCleanup = String(cause).includes('SIVI height edit committed but cleanup failed;');
      this.state.blocked ||= committed || committedCleanup;
      if (committedCleanup) this.state.historyId = null;
      this.state.error = committed || committedCleanup
        ? `SIVI changes committed, but acknowledgement/refresh failed. Undo/reload; do not replay: ${String(cause)}`
        : `SIVI Save failed; drafts retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }

  async restore(action: AuditRestoreAction): Promise<boolean> {
    this.requireIdle();
    if (action !== 'retain' && action !== 'prune') throw new Error('SIVI restoration requires explicit retain or prune.');
    if (this.closeState().unsaved || !this.state.historyId || !this.state.review) {
      throw new Error('Finish or Undo SIVI drafts and select this session\'s verified height history before restoring.');
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
        throw new Error('SIVI restoration acknowledgement is incomplete; Undo/reload, never replay.');
      }
      committed = true;
      this.state.historyId = null;
      if (this.disposed) throw new Error('SIVI editor ownership ended after restoration committed.');
      await this.port.refreshParent();
      await this.readCurrent();
      return true;
    } catch (cause) {
      const committedCleanup = String(cause).includes('SIVI restoration committed but cleanup failed;');
      this.state.blocked ||= committed || committedCleanup;
      if (committedCleanup) this.state.historyId = null;
      this.state.error = committed || committedCleanup
        ? `SIVI restoration committed, but refresh failed. Undo/reload; do not replay: ${String(cause)}`
        : `SIVI restoration failed; history retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }

  dispose() {
    this.disposed = true;
  }
}
