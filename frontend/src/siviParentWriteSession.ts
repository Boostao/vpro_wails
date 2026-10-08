import type { AuditRestoreAction, SIVIParentDirectWrite } from '../bindings/github.com/boostao/vpro-wails';
import type { EditorCloseState } from './closeLifecycle';
import { exactSigned64 } from './projectMetadataRestore';
import { siviParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import {
  stageSIVIParent, siviParentDirty, siviParentErrors, siviParentProposal,
  type SIVIParentOriginal, type SIVIParentDrafts, type SIVIParentInput, type SIVIParentColumn,
} from './siviParentEditor';

export const siviParentDirectColumns = [
  'SV_StandHeight', 'SV_AhorizonDepth', 'SV_GleyingMottlingCM', 'SV_PercentCoarseFrags', 'SV_SoilDepth',
  'StrataCoverTotal', 'SV_FloodPlain', 'SV_StandAgeEstMeas', 'SV_StandHeightEstMeas',
  'SV_PolygonNumber', 'SV_CanopyComposition', 'SnowCoverregime', 'SV_RootZoneTexture', 'SV_AhorizonType',
] as const satisfies readonly SIVIParentColumn[];
export type SIVIParentDirectColumn = typeof siviParentDirectColumns[number];
export function isSIVIParentDirectColumn(value: string): value is SIVIParentDirectColumn {
  return siviParentDirectColumns.some(column => column === value);
}
export interface SIVIParentWritePort {
  read(): PromiseLike<unknown>;
  cancelRead(): void;
  save(request: SIVIParentDirectWrite): PromiseLike<unknown>;
  restore(historyId: string, action: AuditRestoreAction): PromiseLike<unknown>;
  refreshParent(): PromiseLike<void>;
}
type Operation = 'read' | 'save' | 'restore' | 'recover' | null;
export interface SIVIParentWriteView {
  original: SIVIParentOriginal | null;
  drafts: SIVIParentDrafts;
  busy: boolean;
  operation: Operation;
  blocked: boolean;
  error: string | null;
  historyId: string | null;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

function directErrors(drafts: SIVIParentDrafts): string[] {
  return [...siviParentErrors(drafts), ...Object.values(drafts).flatMap(draft =>
    draft?.expected.storage === 'blob' && siviParentDirty({ [draft.column]: draft })
      ? [`${draft.column} historical BLOB replacement has no lossless source audit representation; retain original storage.`] : [])];
}

export function siviParentDirectRequest(original: SIVIParentOriginal, drafts: SIVIParentDrafts): SIVIParentDirectWrite {
  if (Object.keys(drafts).some(column => !isSIVIParentDirectColumn(column))) {
    throw new Error('SIVI directly bound writes exclude callback actions and ProjectID assignment.');
  }
  const invalid = directErrors(drafts)[0];
  if (invalid) throw new Error(invalid);
  const proposal = siviParentProposal(original, drafts);
  if (proposal.actions.length) throw new Error('SIVI source callbacks remain unavailable.');
  const cells = (edits: typeof proposal.scalars) => edits.map(edit => ({
    contextId: edit.contextId, table: edit.table, rowId: edit.rowId, column: edit.column,
    expected: structuredClone(edit.expected), value: structuredClone(edit.value),
  }));
  return { original: structuredClone(proposal.original), scalars: cells(proposal.scalars), options: cells(proposal.options),
    text: cells(proposal.text), categorical: cells(proposal.categorical) };
}

export class SIVIParentWriteSession {
  private state: SIVIParentWriteView = {
    original: null, drafts: {}, busy: false, operation: null, blocked: false, error: null, historyId: null,
  };
  private readonly owner: SIVIParentOwner;
  private disposed = false;
  private generation = 0;
  private historyMaximum = 0;

  constructor(owner: SIVIParentOwner, private readonly port: SIVIParentWritePort, private readonly notify: () => void) {
    this.owner = structuredClone(owner);
  }

  view(): SIVIParentWriteView { return structuredClone(this.state); }
  closeState(): EditorCloseState {
    const invalid = directErrors(this.state.drafts);
    const saveReason = this.state.busy ? 'Wait for the SIVI parent operation to finish.'
      : this.state.blocked ? 'Recover SIVI parent acknowledgement/refresh explicitly with Undo/reload; do not replay.'
      : invalid[0] ?? (!this.state.original ? 'Load owned SIVI parent originals before editing.' : '');
    return { unsaved: this.state.blocked || siviParentDirty(this.state.drafts), busy: this.state.busy,
      blocked: this.state.blocked || invalid.length > 0, canSave: saveReason === '', saveReason, error: this.state.error };
  }
  private idle() {
    if (this.disposed) throw new Error('SIVI parent editor ownership ended.');
    if (this.state.busy) throw new Error('Wait for the SIVI parent operation to finish.');
  }
  private changed() { if (!this.disposed) this.notify(); }
  stage(column: SIVIParentDirectColumn, input: SIVIParentInput) {
    this.idle();
    if (!isSIVIParentDirectColumn(column) || this.state.blocked || !this.state.original) {
      throw new Error('SIVI directly bound source is unavailable; callback actions remain disabled.');
    }
    this.state.drafts = stageSIVIParent(this.state.original, this.state.drafts, column, input);
    this.state.error = directErrors(this.state.drafts)[0] ?? null;
    this.changed();
  }
  async load(): Promise<boolean> {
    this.idle();
    if (this.closeState().unsaved) throw new Error('Save or Undo SIVI parent drafts before reloading originals.');
    return this.reload(false);
  }
  private async readCurrent(generation: number) {
    const wire = await this.port.read();
    if (this.disposed || this.generation !== generation) return false;
    this.state.original = siviParentOriginalFromWire(wire, this.owner);
    return true;
  }
  private async reload(recover: boolean): Promise<boolean> {
    const generation = ++this.generation;
    this.state.busy = true;
    this.state.operation = recover ? 'recover' : 'read';
    this.state.original = null;
    this.changed();
    try {
      if (recover) await this.port.refreshParent();
      if (this.disposed || this.generation !== generation || !await this.readCurrent(generation)) return false;
      this.state.blocked = false;
      this.state.error = null;
      return true;
    } catch (cause) {
      if (this.disposed || this.generation !== generation) return false;
      this.state.error = `SIVI parent reload failed: ${String(cause)}`;
      return false;
    } finally {
      if (!this.disposed && this.generation === generation) {
        this.state.busy = false;
        this.state.operation = null;
        this.changed();
      }
    }
  }
  cancel() {
    this.idleUnlessRead();
    if (!this.state.busy) return;
    this.generation++;
    this.port.cancelRead();
    this.state.original = null;
    this.state.busy = false;
    this.state.operation = null;
    this.state.error = 'SIVI parent read cancelled; reload explicitly.';
    this.changed();
  }
  private idleUnlessRead() {
    if (this.disposed || this.state.busy && this.state.operation !== 'read') {
      throw new Error('SIVI parent commit/recovery cannot be cancelled or disposed.');
    }
  }
  async undo(): Promise<boolean> {
    this.idle();
    this.state.drafts = {};
    return this.reload(true);
  }
  async save(): Promise<boolean> {
    this.idle();
    const close = this.closeState();
    if (!close.canSave || !this.state.original) throw new Error(close.saveReason);
    const request = siviParentDirectRequest(this.state.original, this.state.drafts);
    const count = [request.scalars, request.options, request.text, request.categorical].reduce((sum, edits) => sum + (edits?.length ?? 0), 0);
    if (count === 0) {
      this.state.drafts = {};
      this.state.error = null;
      this.changed();
      return true;
    }
    this.state.busy = true;
    this.state.operation = 'save';
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.save(request);
      committed = true;
      this.state.drafts = {};
      this.state.historyId = null;
      if (!record(result) || result.ChangedCells !== count || typeof result.HistoryID !== 'string' ||
        result.HistoryID !== '' && (!exactSigned64(result.HistoryID) || BigInt(result.HistoryID) <= 0)) {
        throw new Error('SIVI parent Save acknowledgement is incomplete; observe current storage, never replay.');
      }
      this.state.historyId = result.HistoryID || null;
      this.historyMaximum = count;
      await this.port.refreshParent();
      if (!await this.readCurrent(this.generation)) throw new Error('SIVI parent ownership ended after Save.');
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes('SIVI parent edit committed but cleanup failed;');
      if (cleanup) {
        this.state.drafts = {};
        this.state.historyId = null;
      }
      this.state.blocked ||= committed || cleanup;
      this.state.error = committed || cleanup
        ? `SIVI parent changes committed or acknowledged, but verification failed; Undo/reload, do not replay: ${String(cause)}`
        : `SIVI parent Save failed; drafts retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.state.operation = null;
      this.changed();
    }
  }
  async restore(action: AuditRestoreAction): Promise<boolean> {
    this.idle();
    if (action !== 'retain' && action !== 'prune' || this.closeState().unsaved || !this.state.historyId || !this.state.original) {
      throw new Error('Finish or Undo drafts and select this session\'s verified parent history before restoring.');
    }
    this.state.busy = true;
    this.state.operation = 'restore';
    this.state.error = null;
    this.changed();
    let committed = false;
    try {
      const result = await this.port.restore(this.state.historyId, action);
      committed = true;
      this.state.historyId = null;
      if (!record(result) || !Number.isSafeInteger(result.restoredRows) || typeof result.restoredRows !== 'number' ||
        result.restoredRows <= 0 || result.restoredRows > this.historyMaximum || result.cleanedVegRows !== 0 ||
        result.cancelled !== false || result.prunedAuditRows !== (action === 'prune' ? result.restoredRows : 0)) {
        throw new Error('SIVI parent restoration acknowledgement is incomplete; observe current storage, never replay.');
      }
      await this.port.refreshParent();
      if (!await this.readCurrent(this.generation)) throw new Error('SIVI parent ownership ended after restoration.');
      return true;
    } catch (cause) {
      const cleanup = String(cause).includes('SIVI parent restoration committed but cleanup failed;');
      if (cleanup) this.state.historyId = null;
      this.state.blocked ||= committed || cleanup;
      this.state.error = committed || cleanup
        ? `SIVI parent restoration committed or acknowledged, but verification failed; Undo/reload, do not replay: ${String(cause)}`
        : `SIVI parent restoration failed; history retained: ${String(cause)}`;
      return false;
    } finally {
      this.state.busy = false;
      this.state.operation = null;
      this.changed();
    }
  }
  dispose() {
    this.idleUnlessRead();
    this.disposed = true;
    this.generation++;
    this.port.cancelRead();
  }
}
