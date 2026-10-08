import source from '../../resources/fs1333-sivi-layout.json';
import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { ordinaryField, ordinaryNumberValue, ordinaryTextError } from './ordinaryEditor';
import { coordinateDecimalValue } from './coordinateEditor';
import { siviDateTimestampError } from './siviDateTimestamp';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { validateSIVIParentOriginal, type SIVIParentOriginal } from './siviParentEditor';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort, type SIVIParentWriteView } from './siviParentWriteSession';
import type { SIVIParentOwner } from './siviParentTransport';
import { siviReferencePolicies, siviReferenceIssue, siviReferencesFromWire,
  type SIVIReferenceColumn, type SIVIReferences } from './referenceEditor';

export const siviParentSharedColumns = [
  'FieldNumber', 'SiteSurveyor', 'Date', 'StartDate',
  'Location', 'UTMZone', 'UTMEasting', 'UTMNorthing', 'NtsMapSheet',
  'Latitude', 'Longitude', 'LocationAccuracy', 'Elevation',
  'AirPhotoNum', 'XCoord', 'YCoord',
  'PlotRepresenting', 'SiteSeries', 'MapUnit',
  'SlopeGradient', 'Aspect', 'StandAge',
  'HumusThickness', 'SeepageDepth', 'RootingDepth', 'RootRestrictingDepth',
  'StrataCoverTree', 'StrataCoverShrub', 'StrataCoverHerb', 'StrataCoverMoss', 'VegNotes',
  'SiteNotes',
] as const;
const sharedDatePolicy = { column: 'Date', kind: 'date', scope: 'plot' } as const;
const sharedNumberPolicies = [
  { column: 'StartDate', kind: 'integer', scope: 'plot' },
  { column: 'UTMEasting', kind: 'single', scope: 'site' },
  { column: 'UTMNorthing', kind: 'single', scope: 'site' },
  { column: 'Latitude', kind: 'double', axis: 'latitude', scope: 'site' },
  { column: 'Longitude', kind: 'double', axis: 'longitude', scope: 'site' },
  { column: 'LocationAccuracy', kind: 'integer', scope: 'site' },
  { column: 'Elevation', kind: 'integer', scope: 'site' },
  { column: 'SlopeGradient', kind: 'single', scope: 'topography' },
  { column: 'Aspect', kind: 'integer', scope: 'topography' },
  { column: 'StandAge', kind: 'integer', scope: 'topography' },
] as const;
const sharedTextPolicies = [
  { column: 'FieldNumber', kind: 'text', maximum: 50, scope: 'plot' },
  { column: 'SiteSurveyor', kind: 'text', maximum: 30, scope: 'plot' },
  { column: 'Location', kind: 'text', maximum: 255, scope: 'site' },
  { column: 'UTMZone', kind: 'text', maximum: 2, scope: 'site' },
  { column: 'NtsMapSheet', kind: 'text', maximum: 8, scope: 'site' },
  { column: 'PlotRepresenting', kind: 'text', maximum: 255, scope: 'site' },
  { column: 'SiteSeries', kind: 'text', maximum: 5, scope: 'site' },
  { column: 'MapUnit', kind: 'text', maximum: 15, scope: 'site' },
  { column: 'SiteNotes', kind: 'memo', scope: 'notes' },
] as const;
export type SIVIParentSharedColumn = typeof siviParentSharedColumns[number] | SIVIReferenceColumn;
export type SIVIParentSharedInput = { kind: 'original' | 'clear' } | { kind: 'text'; raw: string };
export interface SIVIParentSharedEdit {
  contextId: string; table: string; rowId: string; column: SIVIParentSharedColumn;
  expected: ProjectMetadataCell; value: ProjectMetadataCell;
}
export interface SIVIParentSharedDraft extends SIVIParentSharedEdit {
  input: SIVIParentSharedInput; error: string | null;
}
export type SIVIParentSharedDrafts = Partial<Record<SIVIParentSharedColumn, SIVIParentSharedDraft>>;
export interface SIVIParentSharedWrite { original: SIVIParentOriginal; edits: SIVIParentSharedEdit[] }
export type SIVIParentSharedPort = SIVIParentWritePort<SIVIParentSharedWrite> & {
  readReferences?(zone: ProjectMetadataCell): PromiseLike<unknown>;
};
export type SIVIParentSharedView = SIVIParentWriteView<SIVIParentSharedDrafts> & {
  referencesEnabled?: boolean; referencesBusy?: boolean; referenceError?: string | null;
  references?: SIVIReferences | null; advisories?: Partial<Record<SIVIParentSharedColumn, string>>;
};
export function isSIVIParentSharedColumn(value: string, referencesEnabled = false): value is SIVIParentSharedColumn {
  return siviParentSharedColumns.some(column => column === value) ||
    referencesEnabled && siviReferencePolicies.some(field => field.column === value);
}

export const siviParentSharedFields = siviParentSharedColumns.map(column => {
  const binding = column === 'SiteNotes' ? 'siteNotes' : column;
  const fields = source.forms[0].fields.filter(field => field.binding === binding);
  const sharedPolicy = sharedTextPolicies.find(field => field.column === column) ??
    sharedNumberPolicies.find(field => field.column === column) ??
    (column === 'Date' ? sharedDatePolicy : undefined);
  const policy = sharedPolicy ? { ...sharedPolicy, label: fields[0]?.caption || fields[0]?.controlName || column }
    : ordinaryField(column);
  if (fields.length !== 1 || !policy) throw new Error(`SIVI shared field ${column} has no unique source/proven policy.`);
  const separateLabel = column === 'RootRestrictingDepth'
    ? source.forms[0].fields.find(field => field.controlName === 'Label601' && field.type === 'Label') : null;
  if (column === 'RootRestrictingDepth' && !separateLabel?.caption) {
    throw new Error('SIVI root-restricting depth requires its original separate label.');
  }
  const owner: 'Env' | 'Admin' = column === 'HumusThickness' || column === 'StartDate' ? 'Admin' : 'Env';
  return { column, binding, owner, controlId: fields[0].controlId,
    label: separateLabel?.caption || fields[0].caption || fields[0].controlName,
    page: 'pageId' in fields[0] ? fields[0].pageId : null, policy };
});
const sharedReferenceFields = siviReferencePolicies.map(policy => {
  const controls = source.forms[0].fields.filter(field => field.binding === policy.column);
  if (controls.length !== 1 || controls[0].type !== 'ComboBox') throw new Error(`SIVI reference ${policy.column} has no unique source control.`);
  return { column: policy.column, binding: policy.column, owner: 'Env' as const, controlId: controls[0].controlId,
    label: policy.label, page: 'pageId' in controls[0] ? controls[0].pageId : null,
    policy: { ...policy, kind: 'reference' as const } };
});
export function siviParentSharedFieldsFor(referencesEnabled = false) {
  return referencesEnabled ? [...siviParentSharedFields, ...sharedReferenceFields] : [...siviParentSharedFields];
}

function target(original: SIVIParentOriginal, column: SIVIParentSharedColumn) {
  const field = siviParentSharedFieldsFor(true).find(field => field.column === column);
  const binding = original.Bindings.find(binding => binding.Binding === field?.binding);
  if (!field || !binding || binding.Implicit || binding.ControlID !== field.controlId ||
    binding.Table !== (field.owner === 'Admin' ? original.AdminTable : original.EnvTable)) {
    throw new Error('SIVI shared-field source/physical ownership changed.');
  }
  if (column === 'SiteNotes' && original.EnvColumns[binding.Column]?.name !== 'SiteNotes') {
    throw new Error('SIVI Notes requires literal siteNotes bound to physical Env.SiteNotes.');
  }
  const row = original.Rows[0][field.owner];
  return { binding, row, cell: structuredClone(row.cells[binding.Column]) };
}
export function siviParentSharedCell(original: SIVIParentOriginal, column: SIVIParentSharedColumn): ProjectMetadataCell {
  return target(original, column).cell;
}
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
export function siviParentSharedErrors(drafts: SIVIParentSharedDrafts): string[] {
  return Object.values(drafts).flatMap(draft => draft?.error ? [draft.error] : []);
}
export function siviParentSharedDirty(drafts: SIVIParentSharedDrafts): boolean {
  return Object.values(drafts).some(draft => draft && (draft.error !== null || !equalCell(draft.expected, draft.value)));
}
function stage(original: SIVIParentOriginal, drafts: SIVIParentSharedDrafts,
  column: SIVIParentSharedColumn, input: SIVIParentSharedInput, referencesEnabled = false): SIVIParentSharedDrafts {
  if (!input || !['original', 'clear', 'text'].includes(input.kind) ||
    input.kind === 'text' && typeof input.raw !== 'string') {
    throw new Error('SIVI shared fields require an explicit raw entry, NULL or original storage.');
  }
  const field = siviParentSharedFieldsFor(referencesEnabled).find(field => field.column === column);
  if (!field) throw new Error('SIVI shared field is outside the bounded editor.');
  const { binding, row, cell: expected } = target(original, column);
  let value = structuredClone(expected), error: string | null = null;
  // Compare textarea display only; never normalize a deliberately changed memo.
  const unchangedText = input.kind === 'text' && expected.storage !== 'null' &&
    (input.raw === metadataCellText(expected) ||
      field.policy.kind === 'memo' && expected.storage === 'text' &&
      input.raw === expected.text?.replace(/\r\n?/g, '\n'));
  if (input.kind === 'clear') value = nullCell();
  else if (input.kind === 'text' && !unchangedText) {
    const policy = field.policy;
    if (policy.kind === 'reference') {
      error = ordinaryTextError({ ...policy, kind: 'text' }, input.raw, expected.storage === 'text' ? expected.text : null);
      value = { ...nullCell(), storage: 'text', text: input.raw };
    } else if (policy.kind === 'text' || policy.kind === 'memo') {
      error = ordinaryTextError(policy, input.raw, expected.storage === 'text' ? expected.text : null);
      value = { ...nullCell(), storage: 'text', text: input.raw };
    } else if (policy.kind === 'date') {
      error = siviDateTimestampError(input.raw);
      value = { ...nullCell(), storage: 'text', text: input.raw };
    } else {
      const unchanged = expected.storage === 'real' ? expected.real
        : expected.storage === 'integer' && expected.integer !== null &&
          Number.isSafeInteger(Number(expected.integer)) ? Number(expected.integer) : null;
      const parsed = policy.kind === 'double' ? coordinateDecimalValue(policy.axis, input.raw, unchanged)
        : ordinaryNumberValue(policy, input.raw, unchanged);
      error = parsed.error;
      value = parsed.value !== null && parsed.value === unchanged && !error ? structuredClone(expected)
        : parsed.value === null ? nullCell() : policy.kind === 'integer'
        ? { ...nullCell(), storage: 'integer', integer: String(parsed.value) }
        : { ...nullCell(), storage: 'real', real: parsed.value };
    }
  }
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  return { ...drafts, [column]: { contextId: original.ContextID, table: binding.Table,
    rowId: row.rowId, column, expected, value, input: structuredClone(input), error } };
}

export function siviParentSharedRequest(original: SIVIParentOriginal, drafts: SIVIParentSharedDrafts,
  referencesEnabled = false, references: SIVIReferences | null = null): SIVIParentSharedWrite {
  const reviewed = validateSIVIParentOriginal(original);
  const edits: SIVIParentSharedEdit[] = [];
  for (const [column, draft] of Object.entries(drafts)) {
    if (!isSIVIParentSharedColumn(column, referencesEnabled) || !draft || draft.column !== column) {
      throw new Error('SIVI shared writes exclude callbacks, ProjectID and other header fields.');
    }
    const verified = stage(reviewed, {}, column, draft.input, referencesEnabled)[column]!;
    if (!equalCell(verified.expected, draft.expected) || !equalCell(verified.value, draft.value) ||
      verified.contextId !== draft.contextId || verified.table !== draft.table || verified.rowId !== draft.rowId) {
      throw new Error('SIVI shared draft belongs to another original/physical owner.');
    }
    if (verified.error) throw new Error(verified.error);
    const referenceError = siviReferenceIssue(column, verified.expected, verified.value, references).error;
    if (referenceError) throw new Error(referenceError);
    if (!equalCell(verified.expected, verified.value)) {
      const { contextId, table, rowId, expected, value } = verified;
      edits.push({ contextId, table, rowId, column, expected, value });
    }
  }
  return { original: reviewed, edits };
}

// Rejected transport promises cannot establish rollback. Reuse the proven
// lifecycle's commit-uncertainty recovery path, retiring requests before replay.
function conservativePort(port: SIVIParentWritePort<SIVIParentSharedWrite>): SIVIParentWritePort<SIVIParentSharedWrite> {
  return {
    read: () => port.read(), cancelRead: () => port.cancelRead(), refreshParent: () => port.refreshParent(),
    save: async request => {
      try { return await port.save(request); }
      catch (cause) { throw new Error(`SIVI parent edit committed but cleanup failed; outcome unknown; reload, never replay: ${String(cause)}`); }
    },
    restore: async (history, action) => {
      try { return await port.restore(history, action); }
      catch (cause) { throw new Error(`SIVI parent restoration committed but cleanup failed; outcome unknown; reload, never replay: ${String(cause)}`); }
    },
  };
}

export class SIVIParentSharedSession extends SIVIParentScopedWriteSession<SIVIParentSharedColumn,
  SIVIParentSharedWrite, SIVIParentSharedDrafts, SIVIParentSharedInput> {
  private readonly referenceState: { value: SIVIReferences | null; busy: boolean; error: string | null; revision: number; disposed: boolean };
  private readonly referenceEnabled: boolean;
  constructor(private readonly referenceOwner: SIVIParentOwner, private readonly sharedPort: SIVIParentSharedPort,
    private readonly referenceNotify: () => void, referenceEditingEnabled: boolean = false) {
    const enabled = referenceEditingEnabled === true;
    if (enabled && typeof sharedPort.readReferences !== 'function') {
      throw new Error('Enabled SIVI reference editing requires a reference read port.');
    }
    const state = { value: null as SIVIReferences | null, busy: false, error: null as string | null, revision: 0, disposed: false };
    const errors = (drafts: SIVIParentSharedDrafts) => Object.values(drafts).flatMap(draft => !draft ? [] :
      draft.error ? [draft.error] : siviReferenceIssue(draft.column, draft.expected, draft.value, state.value).error
        ? [siviReferenceIssue(draft.column, draft.expected, draft.value, state.value).error!] : []);
    super(referenceOwner, conservativePort(sharedPort), referenceNotify, {
      accepts: (value): value is SIVIParentSharedColumn => isSIVIParentSharedColumn(value, enabled),
      unavailable: 'SIVI shared-field editor requires owned originals; callback actions and XL lifecycle are unavailable.',
      drafts: { empty: () => ({}), stage: (original, drafts, column, input) => stage(original, drafts, column, input, enabled),
        errors, dirty: siviParentSharedDirty },
      request: (original, drafts) => siviParentSharedRequest(original, drafts, enabled, state.value), count: request => request.edits.length,
    });
    this.referenceEnabled = enabled;
    this.referenceState = state;
    this.referenceOwner = structuredClone(referenceOwner);
  }
  override view(): SIVIParentSharedView {
    const view = super.view(), advisories: Partial<Record<SIVIParentSharedColumn, string>> = {};
    for (const draft of Object.values(view.drafts)) {
      if (!draft) continue;
      const issue = siviReferenceIssue(draft.column, draft.expected, draft.value, this.referenceState.value);
      draft.error ||= issue.error;
      if (issue.advisory) advisories[draft.column] = issue.advisory;
    }
    if (this.referenceEnabled && view.original && view.drafts.Zone &&
        !equalCell(view.drafts.Zone.expected, view.drafts.Zone.value)) {
      const subZone = view.drafts.SubZone?.value ?? siviParentSharedCell(view.original, 'SubZone');
      const issue = siviReferenceIssue('SubZone', nullCell(), subZone, this.referenceState.value);
      if (issue.advisory) advisories.SubZone = issue.advisory;
    }
    return { ...view, error: view.error ?? Object.values(view.drafts).find(draft => draft?.error)?.error ?? null,
      busy: view.busy || this.referenceState.busy, operation: view.operation ?? (this.referenceState.busy ? 'read' : null),
      referencesEnabled: this.referenceEnabled, referencesBusy: this.referenceState.busy,
      referenceError: this.referenceState.error, references: structuredClone(this.referenceState.value), advisories };
  }
  override closeState() {
    const state = super.closeState();
    return this.referenceState.busy ? { ...state, busy: true, canSave: false, saveReason: 'Wait for SIVI reference choices.' } : state;
  }
  override stage(column: SIVIParentSharedColumn, input: SIVIParentSharedInput) {
    super.stage(column, input);
    if (column === 'Zone' && this.referenceEnabled && !this.referenceState.disposed) void this.refreshReferences();
  }
  async refreshReferences(): Promise<boolean> {
    if (!this.referenceEnabled || !this.sharedPort.readReferences) throw new Error('SIVI reference editing is independently disabled.');
    const original = super.view().original;
    if (!original || this.referenceState.disposed || super.view().busy) throw new Error('Load idle owned originals before reference choices.');
    const state = this.referenceState, revision = ++state.revision;
    this.sharedPort.cancelRead();
    state.value = null; state.busy = true; state.error = null;
    this.referenceNotify();
    const zone = super.view().drafts.Zone?.value ?? siviParentSharedCell(original, 'Zone');
    try {
      const wire = await this.sharedPort.readReferences(zone);
      if (state.disposed || state.revision !== revision) return false;
      state.value = siviReferencesFromWire(wire, this.referenceOwner, zone);
      if (!super.view().blocked) {
        for (const draft of Object.values(super.view().drafts)) {
          if (draft && siviReferencePolicies.some(field => field.column === draft.column)) super.stage(draft.column, draft.input);
        }
      }
      return true;
    } catch (cause) {
      if (state.disposed || state.revision !== revision) return false;
      state.error = `SIVI reference read failed: ${String(cause)}`;
      return false;
    } finally {
      if (!state.disposed && state.revision === revision) { state.busy = false; this.referenceNotify(); }
    }
  }
  override async load() {
    if (this.referenceState.busy) throw new Error('Wait for SIVI reference choices.');
    const result = await super.load();
    if (result && this.referenceEnabled) await this.refreshReferences();
    return result;
  }
  override async undo() {
    if (super.view().busy) throw new Error('Wait for the SIVI parent operation to finish.');
    this.cancelReferences();
    const result = await super.undo();
    if (result && this.referenceEnabled) await this.refreshReferences();
    return result;
  }
  override async save() {
    if (this.referenceState.busy) throw new Error('Wait for SIVI reference choices.');
    const result = await super.save();
    if (result && this.referenceEnabled) await this.refreshReferences();
    return result;
  }
  override async restore(action: Parameters<SIVIParentWritePort['restore']>[1]) {
    if (this.referenceState.busy) throw new Error('Wait for SIVI reference choices.');
    const result = await super.restore(action);
    if (result && this.referenceEnabled) await this.refreshReferences();
    return result;
  }
  private cancelReferences() {
    const state = this.referenceState;
    state.revision++; state.value = null; state.busy = false;
    if (this.referenceEnabled) this.sharedPort.cancelRead();
  }
  override cancel() {
    const wasBusy = this.referenceState.busy;
    const originalReadBusy = super.view().busy;
    super.cancel();
    if (wasBusy || originalReadBusy) this.cancelReferences();
    if (wasBusy) { this.referenceState.error = 'SIVI reference read cancelled; retry choices explicitly.'; this.referenceNotify(); }
  }
  override dispose() {
    super.dispose();
    this.cancelReferences();
    this.referenceState.disposed = true;
  }
}
