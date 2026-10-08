import type { AuditRestoreAction, AuditRestoreResult, SIVIVegetationProjection } from '../bindings/github.com/boostao/vpro-wails';
import type { EditorCloseState } from './closeLifecycle';
import { exactSigned64 } from './projectMetadataRestore';
import { siviHeightPresentation, type SIVIProjection } from './siviHeightEditor';
import { siviProjectionFromWire } from './siviHeightSession';
import type { SIVIChildSourceNotice } from './siviChildSourceNotices';

export interface SIVIChildWriteResult { ChangedCells: number; HistoryID: string }
export interface SIVIChildWritePort {
  read(extended: boolean): PromiseLike<SIVIVegetationProjection[] | null>;
  save(extended: boolean, requestJSON: string): PromiseLike<SIVIChildWriteResult | null>;
  restore(historyId: string, action: AuditRestoreAction): PromiseLike<AuditRestoreResult | null>;
  refreshParent(): PromiseLike<void>;
}
export interface SIVIChildWriteView<Drafts> {
  review: SIVIProjection[] | null;
  drafts: Drafts;
  extended: boolean;
  busy: boolean;
  blocked: boolean;
  error: string | null;
  historyId: string | null;
  sourceNotices: SIVIChildSourceNotice[];
  sourceNoticesSaved: boolean;
}
export interface SIVIChildWriteScope<Column extends string, Drafts> {
  label: string;
  pluralLabel: string;
  empty(): Drafts;
  stage(review: SIVIProjection[], drafts: Drafts, rowId: string, column: Column, raw: string, nullValue: boolean): Drafts;
  errors(drafts: Drafts, review?: readonly SIVIProjection[]): string[];
  dirty(drafts: Drafts): boolean;
  hidden(drafts: Drafts, extended: boolean): boolean;
  count(review: SIVIProjection[], drafts: Drafts): number;
  requestJSON(review: SIVIProjection[], drafts: Drafts): string;
  sourceNotices?(review: SIVIProjection[], drafts: Drafts): SIVIChildSourceNotice[];
  conservativeFailures?: boolean;
  auditFreeHistory?: boolean;
}

export class SIVIChildWriteSession<Column extends string, Drafts> {
  private state: SIVIChildWriteView<Drafts>;
  private disposed = false;
  private get label() { return this.scope.label; }

  constructor(private readonly plot: string, private readonly port: SIVIChildWritePort,
    private readonly notify: () => void, private readonly scope: SIVIChildWriteScope<Column, Drafts>, extended = false) {
    this.state = { review: null, drafts: scope.empty(), extended, busy: false, blocked: false, error: null, historyId: null,
      sourceNotices: [], sourceNoticesSaved: false };
  }

  view(): SIVIChildWriteView<Drafts> {
    const view = structuredClone(this.state);
    if (this.disposed) view.sourceNotices = [];
    else if (view.review && !view.blocked && this.scope.dirty(view.drafts) && !this.scope.errors(view.drafts, view.review).length) {
      view.sourceNotices = this.scope.sourceNotices?.(view.review, view.drafts) ?? [];
      view.sourceNoticesSaved = false;
    }
    return view;
  }
  closeState(): EditorCloseState {
    const invalid = this.scope.errors(this.state.drafts, this.state.review ?? undefined);
    const hidden = this.scope.hidden(this.state.drafts, this.state.extended);
    const saveReason = this.state.busy ? `Wait for the ${this.label} operation to finish.`
      : this.state.blocked ? `${this.label} acknowledgement or refresh failed. Undo/reload explicitly; do not replay.`
      : hidden ? 'Show extended B3/B4/B5 controls before saving their retained drafts.'
      : invalid.length ? `Correct invalid ${this.scope.pluralLabel} or Undo before saving. ${invalid[0]}`
      : !this.state.review ? 'Load original SIVI source rows before saving.' : '';
    return { unsaved: this.state.blocked || hidden || this.scope.dirty(this.state.drafts), busy: this.state.busy,
      blocked: this.state.blocked || hidden || invalid.length > 0, canSave: saveReason === '', saveReason, error: this.state.error };
  }
  private changed() { if (!this.disposed) this.notify(); }
  private requireIdle() {
    if (this.disposed) throw new Error(`${this.label} editor ownership ended; reopen the plot.`);
    if (this.state.busy) throw new Error(`Wait for the ${this.label} operation to finish.`);
  }
  private requireOwned() {
    if (this.disposed) throw new Error(`${this.label} editor ownership ended during the operation.`);
  }
  stage(rowId: string, column: Column, raw: string, nullValue: boolean) {
    this.requireIdle();
    if (this.state.blocked || !this.state.review) throw new Error(`${this.label} source is unavailable; Undo/reload before editing.`);
    this.state.drafts = this.scope.stage(this.state.review, this.state.drafts, rowId, column, raw, nullValue);
    this.state.error = this.scope.errors(this.state.drafts, this.state.review)[0] ?? null;
    this.changed();
  }
  presentation(extended: boolean) {
    this.requireIdle();
    if (this.state.review) this.state.review = siviHeightPresentation(this.state.review, this.plot, extended);
    this.state.extended = extended;
    this.changed();
  }
  load(): Promise<boolean> { return this.loadSource(false); }
  refreshSource(): Promise<boolean> { return this.loadSource(true); }
  private async loadSource(preserveNotices: boolean): Promise<boolean> {
    this.requireIdle();
    if (this.closeState().unsaved) throw new Error(`Finish or Undo ${this.label} drafts before reloading original rows.`);
    return this.reload(false, preserveNotices);
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
  private async reload(refreshParent: boolean, preserveNotices = false): Promise<boolean> {
    this.state.busy = true;
    this.changed();
    try {
      if (refreshParent) await this.refresh();
      else await this.readCurrent();
      this.state.blocked = false;
      this.state.error = null;
      if (!preserveNotices) {
        this.state.sourceNotices = [];
        this.state.sourceNoticesSaved = false;
      }
      return true;
    } catch (cause) {
      this.state.review = null;
      this.state.error = `${this.label} reload failed: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }
  async undo(): Promise<boolean> {
    this.requireIdle();
    this.state.drafts = this.scope.empty();
    return this.reload(true);
  }
  async save(): Promise<boolean> {
    this.requireIdle();
    const close = this.closeState();
    if (!close.canSave || !this.state.review) throw new Error(close.saveReason);
    const count = this.scope.count(this.state.review, this.state.drafts);
    if (!count) {
      this.state.drafts = this.scope.empty();
      this.state.error = null;
      this.state.sourceNotices = [];
      this.state.sourceNoticesSaved = false;
      this.changed();
      return true;
    }
    const requestJSON = this.scope.requestJSON(this.state.review, this.state.drafts);
    const sourceNotices = this.scope.sourceNotices?.(this.state.review, this.state.drafts) ?? [];
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.save(this.state.extended, requestJSON);
      if (this.disposed) {
        this.state.blocked = true;
        this.state.historyId = null;
      }
      this.requireOwned();
      if (!result || result.ChangedCells !== count || typeof result.HistoryID !== 'string' ||
          this.scope.auditFreeHistory === true && result.HistoryID === '' ||
          result.HistoryID !== '' && (!exactSigned64(result.HistoryID) || BigInt(result.HistoryID) <= 0)) {
        this.state.blocked = true;
        this.state.historyId = null;
        throw new Error(`${this.label} Save acknowledgement is incomplete; Undo/reload, never replay.`);
      }
      committed = true;
      this.state.historyId = result.HistoryID || null;
      this.state.drafts = this.scope.empty();
      this.state.sourceNotices = sourceNotices;
      this.state.sourceNoticesSaved = true;
      await this.refresh();
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes(`${this.label} edit committed but cleanup failed;`);
      this.state.blocked ||= committed || cleanup || this.scope.conservativeFailures === true;
      if (cleanup || this.scope.conservativeFailures === true && !committed) this.state.historyId = null;
      if (this.state.blocked) this.state.review = null;
      this.state.error = this.state.blocked
        ? `${this.label} acknowledgement/refresh unresolved. Undo/reload; do not replay: ${String(cause)}`
        : `${this.label} Save failed; drafts retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }
  async restore(action: AuditRestoreAction): Promise<boolean> {
    this.requireIdle();
    if (action !== 'retain' && action !== 'prune') throw new Error(`${this.label} restoration requires explicit retain or prune.`);
    if (this.closeState().unsaved || !this.state.historyId || !this.state.review) {
      throw new Error(`Finish or Undo drafts and select this session's verified ${this.label.replace(/^SIVI /, '')} history before restoring.`);
    }
    this.state.busy = true;
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.restore(this.state.historyId, action);
      if (this.disposed) {
        this.state.blocked = true;
        this.state.historyId = null;
      }
      this.requireOwned();
      if (!result || !Number.isSafeInteger(result.restoredRows) || result.restoredRows <= 0 || result.cleanedVegRows !== 0 ||
          result.cancelled !== false || result.prunedAuditRows !==
            (action === 'prune' && this.scope.auditFreeHistory !== true ? result.restoredRows : 0)) {
        this.state.blocked = true;
        this.state.historyId = null;
        throw new Error(`${this.label} restoration acknowledgement is incomplete; Undo/reload, never replay.`);
      }
      committed = true;
      this.state.historyId = null;
      this.state.sourceNotices = [];
      this.state.sourceNoticesSaved = false;
      await this.refresh();
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes('SIVI restoration committed but cleanup failed;') ||
        String(cause).includes(`${this.label} restoration committed but cleanup failed;`);
      this.state.blocked ||= committed || cleanup || this.scope.conservativeFailures === true;
      if (cleanup || this.scope.conservativeFailures === true) this.state.historyId = null;
      if (this.state.blocked) this.state.review = null;
      this.state.error = this.state.blocked
        ? `${this.label} restoration acknowledgement/refresh unresolved. Undo/reload; do not replay: ${String(cause)}`
        : `${this.label} restoration failed; history retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.changed();
    }
  }
  dispose() { this.disposed = true; }
}
