import type {
  ProjectMetadataCell, SIVIParentCellEdit, SIVIProjectSelection, TwoPageEntryCodeAcknowledgement,
  TwoPageEntryFieldPolicy, TwoPageEntryReferenceFilters, TwoPageEntryReferenceSnapshot, TwoPageParentEntryWrite,
} from '../bindings/github.com/boostao/vpro-wails';
import type { SIVIParentOriginal } from './siviParentEditor';
import { equalCell } from './projectMetadataEditor';
import { siviMetadataTableFromWire, sourceParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort } from './siviParentWriteSession';
import { conservativePort } from './siviParentSharedSession';
import { siviProjectChoicesFromWire } from './siviParentSourceSession';
import { siviProjectAssignmentValueError } from './siviProjectAssignmentSession';
import { twoPageParentSource, type TwoPageParentForm } from './twoPageParentSession';
import { validateTwoPageParentCommonOriginal } from './twoPageParentCommonSession';
import { parseTwoPageEntryValue, twoPageEntryKinds, type TwoPageEntryInput } from './twoPageEntryDraft';

export type TwoPageEntrySessionCache = Map<string, Map<TwoPageParentForm, TwoPageEntrySession>>;
export interface TwoPageEntryPort extends SIVIParentWritePort<TwoPageParentEntryWrite> {
  readReferences(filters: TwoPageEntryReferenceFilters): PromiseLike<unknown>;
}
export interface TwoPageEntryDraft extends SIVIParentCellEdit {
  input: TwoPageEntryInput; error: string | null; selection?: SIVIProjectSelection;
  acknowledgement?: TwoPageEntryCodeAcknowledgement;
  acknowledgementRevision?: number;
}
export type TwoPageEntryDrafts = Record<string, TwoPageEntryDraft>;
export type TwoPageEntrySnapshot = Omit<TwoPageEntryReferenceSnapshot, 'Original' | 'Fields' | 'Policies' | 'ProjectChoices'> & {
  Original: SIVIParentOriginal;
  Fields: NonNullable<TwoPageEntryReferenceSnapshot['Fields']>;
  Policies: TwoPageEntryFieldPolicy[];
  ProjectChoices: ReturnType<typeof siviProjectChoicesFromWire>;
};
interface ReferenceState { snapshot: TwoPageEntrySnapshot | null; busy: boolean; error: string | null; revision: number; disposed: boolean;
  pending: TwoPageEntryCodeAcknowledgement | null; projectRowId: string | null }
const same = (a: unknown, b: unknown): boolean => JSON.stringify(a) === JSON.stringify(b);
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const textOrNull = (value: unknown) => value === null || typeof value === 'string';
function cellFromWire(cell: unknown): ProjectMetadataCell {
  return siviMetadataTableFromWire({ columns: [{ name: 'value', declaredType: '' }],
    rows: [{ rowId: '1', cells: [cell] }] }).rows[0].cells[0];
}
function target(original: SIVIParentOriginal, column: string) {
  const binding = original.Bindings.find(binding => binding.Binding === column);
  if (!binding || binding.Implicit || column === 'PlotNumber') throw new Error('Complete-entry field is not editable.');
  const row = original.Rows[0][binding.Table === original.EnvTable ? 'Env' : 'Admin'];
  return { binding, row, expected: structuredClone(row.cells[binding.Column]) };
}
export function twoPageEntryCell(original: SIVIParentOriginal, column: string): ProjectMetadataCell {
  const binding = original.Bindings.find(binding => binding.Binding === column);
  if (!binding || binding.Implicit || ![original.EnvTable, original.AdminTable].includes(binding.Table)) {
    throw new Error('Complete-entry source-binding read is unavailable.');
  }
  const row = original.Rows[0][binding.Table === original.EnvTable ? 'Env' : 'Admin'];
  const value = row?.cells[binding.Column];
  if (!value) throw new Error('Complete-entry source-binding original cell is unavailable.');
  return structuredClone(value);
}
function effective(original: SIVIParentOriginal, drafts: TwoPageEntryDrafts): TwoPageEntryReferenceFilters {
  return { zone: structuredClone(drafts.Zone?.value ?? target(original, 'Zone').expected),
    subZone: structuredClone(drafts.SubZone?.value ?? target(original, 'SubZone').expected) };
}
function decode(wire: unknown, owner: SIVIParentOwner, form: TwoPageParentForm): TwoPageEntrySnapshot {
  if (!record(wire)) throw new Error('Complete-entry snapshot is unavailable.');
  const original = sourceParentOriginalFromWire(wire.Original, owner, value => validateTwoPageParentCommonOriginal(value, form));
  if (original.Rows.length !== 1 || ![1, 2].includes(wire.ProjectSource as number) ||
      ![1, 2, 3].includes(wire.WorkingSource as number) || typeof wire.MasterEditingAvailable !== 'boolean' ||
      typeof wire.ProjectAssignmentAvailable !== 'boolean' || typeof wire.ProjectAssignmentDiagnostic !== 'string' ||
      !wire.ProjectAssignmentDiagnostic || !Array.isArray(wire.Policies) || !Array.isArray(wire.Fields)) {
    throw new Error('Complete-entry ownership, availability or policy snapshot is incomplete.');
  }
  const controls = twoPageParentSource(form).fields.filter(field => field.binding);
  const columns = new Set(controls.map(field => field.binding).filter(column => column !== 'PlotNumber'));
  if (wire.Policies.length !== (form.endsWith('-CHARS') ? 119 : 117) ||
      new Set(wire.Policies.map(policy => policy?.column)).size !== columns.size) {
    throw new Error('Complete-entry editable policy cohort changed.');
  }
  for (const policy of wire.Policies) {
    if (!record(policy) || !columns.delete(policy.column as string) || !['Env', 'Admin'].includes(policy.owner as string) ||
        !twoPageEntryKinds.some(kind => kind === policy.kind) || !Number.isInteger(policy.maximum) ||
        (policy.maximum as number) < 0 || original.Bindings.filter(binding => binding.Binding === policy.column)
          .some(binding => binding.Table !== `${owner.project}_${policy.owner}`)) {
      throw new Error('Complete-entry policy identity/domain or physical ownership changed.');
    }
  }
  const combos = controls.filter(field => field.type === 'ComboBox');
  if (wire.Fields.length !== (form.endsWith('-CHARS') ? 55 : 56) ||
      new Set(wire.Fields.map(field => field?.column)).size !== combos.length) {
    throw new Error('Complete-entry reference cohort changed.');
  }
  const fields = wire.Fields.map(field => {
    const control = combos.find(control => control.binding === field?.column);
    const limit = control && 'LimitToList' in control.properties ? control.properties.LimitToList : undefined;
    if (!control || limit && limit.value !== 'NotDefault' || !record(field) ||
        field.required !== Boolean(limit) || typeof field.listName !== 'string' || !field.listName ||
        typeof field.source !== 'string' || !field.source || typeof field.available !== 'boolean' ||
        typeof field.diagnostic !== 'string' || !field.available && !field.diagnostic || !Array.isArray(field.choices)) {
      throw new Error('Complete-entry reference source/required receipt is incomplete.');
    }
    const definitions = siviMetadataTableFromWire(field.definitions);
    if (new Set(definitions.columns.map(column => column.name.toLowerCase())).size !== definitions.columns.length ||
        definitions.columns.some(column => !column.name)) throw new Error('Complete-entry definition schema is ambiguous.');
    // Zone is SELECT DISTINCT Zone/ZoneDescription, not a physical definition row.
    const zoneValues = field.column === 'Zone' && field.listName === 'Zone' &&
      field.source === 'verified BEC catalogue; Zone value identities and SubZone source-ordinal identities';
    if (zoneValues && (definitions.columns.length !== 0 || definitions.rows.length !== 0)) {
      throw new Error('Complete-entry Zone value provenance cannot contain physical definitions.');
    }
    const seen = new Set<string>();
    for (const choice of field.choices) {
      if (!record(choice) || typeof choice.rowId !== 'string' || !textOrNull(choice.code) ||
          !textOrNull(choice.description) || typeof choice.selectable !== 'boolean' || typeof choice.diagnostic !== 'string') {
        throw new Error('Complete-entry typed reference choices are incomplete.');
      }
      const identity = zoneValues ? JSON.stringify([choice.code, choice.description]) : choice.rowId;
      if (seen.has(identity) || (zoneValues ? choice.rowId !== ''
        : !definitions.rows.some(row => row.rowId === choice.rowId))) {
        throw new Error('Complete-entry reference choice identity/provenance is incomplete.');
      }
      seen.add(identity);
    }
    return { ...field, definitions };
  });
  const choices = siviProjectChoicesFromWire(wire.ProjectChoices, owner);
  if (choices.SourceOption !== wire.ProjectSource) throw new Error('Complete-entry project source proof changed.');
  return structuredClone({ ...wire, Original: original, Fields: fields, Policies: wire.Policies, ProjectChoices: choices,
    Zone: cellFromWire(wire.Zone), SubZone: cellFromWire(wire.SubZone) }) as TwoPageEntrySnapshot;
}
function receipt(snapshot: TwoPageEntrySnapshot, draft: TwoPageEntryDraft): TwoPageEntryCodeAcknowledgement {
  const reference = snapshot.Fields.find(field => field.column === draft.column);
  if (!reference) throw new Error('Complete-entry acknowledgement requires a reference identity.');
  const original = snapshot.Original;
  return structuredClone({ contextId: original.ContextID, project: original.Project, plot: original.Plot,
    form: original.Form, table: draft.table, rowId: draft.rowId, column: draft.column,
    expected: draft.expected, value: draft.value, reference });
}
function needsAcknowledgement(snapshot: TwoPageEntrySnapshot, draft: TwoPageEntryDraft): boolean {
  const field = snapshot.Fields.find(field => field.column === draft.column);
  if (!field || field.required || draft.column === 'ProjectID' || draft.value.storage === 'null' ||
      equalCell(draft.expected, draft.value)) return false;
  const code = draft.value.storage === 'integer' ? draft.value.integer : draft.value.text;
  return !(field.available && field.choices?.some(choice =>
    choice.selectable && choice.code !== null && choice.code !== '' && choice.code === code));
}
function issue(snapshot: TwoPageEntrySnapshot | null, draft: TwoPageEntryDraft, revision: number): { error: string | null; advisory: string | null } {
  if (draft.error) return { error: draft.error, advisory: null };
  if (equalCell(draft.expected, draft.value)) return { error: null, advisory: null };
  if (!snapshot) return { error: 'Reload complete-entry source/reference proofs.', advisory: null };
  if (draft.column === 'BECSiteUnit' && !snapshot.MasterEditingAvailable) {
    return { error: 'Master BECSiteUnit editing is unavailable.', advisory: null };
  }
  if (draft.column === 'ProjectID') {
    const selection = draft.selection;
    const row = snapshot.ProjectChoices.Choices.rows.find(row => row.rowId === selection?.metadataOriginal.rowId);
    return { error: !snapshot.ProjectAssignmentAvailable || !selection ||
      selection.sourceOption !== snapshot.ProjectSource || selection.metadataAlias !== snapshot.ProjectChoices.Alias ||
      selection.metadataTable !== snapshot.ProjectChoices.Table ||
      !same(selection.metadataColumns, snapshot.ProjectChoices.Choices.columns) || !same(selection.metadataOriginal, row)
      ? 'Reload observed physical ProjectID selection/availability.' : null, advisory: null };
  }
  const field = snapshot.Fields.find(field => field.column === draft.column);
  if (!field || draft.value.storage === 'null') return { error: null, advisory: null };
  const code = draft.value.storage === 'integer' ? draft.value.integer : draft.value.text;
  if (field.available && field.choices?.some(choice => choice.selectable && choice.code !== null && choice.code !== '' && choice.code === code)) {
    return { error: null, advisory: null };
  }
  const advisory = field.diagnostic || `${draft.column} is not an exact selectable nonempty reference member.`;
  return { error: field.required ? advisory : draft.acknowledgementRevision === revision &&
    same(draft.acknowledgement, receipt(snapshot, draft)) ? null
    : `${draft.column} requires explicit fresh reference acknowledgement.`, advisory: field.required ? null : advisory };
}

export class TwoPageEntrySession extends SIVIParentScopedWriteSession<string, TwoPageParentEntryWrite, TwoPageEntryDrafts, TwoPageEntryInput> {
  private readonly references: ReferenceState;
  private readonly entryOwner: SIVIParentOwner;
  private readonly validationErrors: (drafts: TwoPageEntryDrafts) => string[];
  constructor(owner: SIVIParentOwner, private readonly form: TwoPageParentForm,
    private readonly entryPort: TwoPageEntryPort, private readonly entryNotify: () => void) {
    const state: ReferenceState = { snapshot: null, busy: false, error: null, revision: 0, disposed: false,
      pending: null, projectRowId: null };
    const errors = (drafts: TwoPageEntryDrafts) => {
      if (state.error && Object.values(drafts).some(draft => !equalCell(draft.expected, draft.value))) return [state.error];
      const snapshot = state.snapshot;
      if (snapshot) {
        const filters = effective(snapshot.Original, drafts);
        if (!equalCell(filters.zone, snapshot.Zone) || !equalCell(filters.subZone, snapshot.SubZone)) {
          return ['Reload references for the effective Zone/SubZone before Save.'];
        }
      }
      return Object.values(drafts).flatMap(draft => {
        const error = issue(snapshot, draft, state.revision).error; return error ? [error] : [];
      });
    };
    super(owner, conservativePort(entryPort, 'Two-page complete entry'), entryNotify, {
      label: 'Two-page complete entry', accepts: (column): column is string => typeof column === 'string' && column !== 'PlotNumber',
      unavailable: 'Load owned complete-entry originals before editing.',
      drafts: {
        empty: () => ({}), errors,
        dirty: drafts => Object.values(drafts).some(draft => draft.error !== null || !equalCell(draft.expected, draft.value)),
        stage: (original, drafts, column, input) => {
          const snapshot = state.snapshot, policy = snapshot?.Policies.find(policy => policy.column === column);
          if (!snapshot || !policy || column === 'BECSiteUnit' && !snapshot.MasterEditingAvailable) {
            throw new Error('Complete-entry field policy or MasterEditingAvailable is unavailable.');
          }
          const { binding, row, expected } = target(original, column);
          const draft: TwoPageEntryDraft = { contextId: original.ContextID, table: binding.Table, rowId: row.rowId,
            column, expected, value: expected, input: structuredClone(input), error: null };
          if (column === 'ProjectID' && (state.projectRowId !== null || input.kind !== 'original')) {
            const choice = snapshot.ProjectChoices.Choices.rows.find(row => row.rowId === state.projectRowId);
            if (!snapshot.ProjectAssignmentAvailable || !choice || binding.ControlID !== `form:${form}/ProjectID` ||
                binding.Table !== original.EnvTable) {
              draft.error = 'ProjectID requires an available observed PHYSICAL Project row; no free text or NULL assignment.';
            } else {
              draft.value = structuredClone(choice.cells[0]);
              draft.input = { kind: 'text', raw: choice.cells[0].text ?? '' };
              draft.error = siviProjectAssignmentValueError(expected, draft.value);
              draft.selection = structuredClone({ contextId: original.ContextID, controlId: binding.ControlID,
                table: binding.Table, rowId: row.rowId, expected, sourceOption: snapshot.ProjectSource,
                metadataAlias: snapshot.ProjectChoices.Alias, metadataTable: snapshot.ProjectChoices.Table,
                metadataColumns: snapshot.ProjectChoices.Choices.columns, metadataOriginal: choice });
            }
          } else Object.assign(draft, parseTwoPageEntryValue(policy, expected, input));
          if (state.pending && same(state.pending, receipt(snapshot, draft))) {
            draft.acknowledgement = structuredClone(state.pending); draft.acknowledgementRevision = state.revision;
          }
          return { ...drafts, [column]: draft };
        },
      },
      decodeOriginal: (wire, owned) => {
        const snapshot = decode(wire, owned, form), filters = effective(snapshot.Original, {});
        if (!equalCell(filters.zone, snapshot.Zone) || !equalCell(filters.subZone, snapshot.SubZone)) {
          throw new Error('Initial complete-entry filter receipt does not match its physical original.');
        }
        state.snapshot = snapshot; state.error = null; state.revision++;
        return snapshot.Original;
      },
      request: (original, drafts) => {
        const snapshot = state.snapshot;
        if (!snapshot || !same(original, snapshot.Original)) throw new Error('Complete-entry original/source proof is stale.');
        const invalid = errors(drafts)[0]; if (invalid) throw new Error(invalid);
        const changed = Object.values(drafts).filter(draft => !equalCell(draft.expected, draft.value));
        return { original: structuredClone(original), projectSource: snapshot.ProjectSource, workingSource: snapshot.WorkingSource,
          edits: changed.map(({ contextId, table, rowId, column, expected, value }) =>
            structuredClone({ contextId, table, rowId, column, expected, value })),
          projectSelection: changed.find(draft => draft.column === 'ProjectID')?.selection ?? null,
          acknowledgements: changed.flatMap(draft => {
            if (!needsAcknowledgement(snapshot, draft)) return [];
            if (draft.acknowledgementRevision !== state.revision || !same(draft.acknowledgement, receipt(snapshot, draft))) {
              throw new Error(`${draft.column} requires explicit fresh reference acknowledgement.`);
            }
            return [structuredClone(draft.acknowledgement!)];
          }) };
      },
      count: request => request.edits?.length ?? 0,
    });
    this.references = state; this.entryOwner = structuredClone(owner); this.validationErrors = errors;
  }
  override view() {
    const view = super.view(), snapshot = view.original ? structuredClone(this.references.snapshot) : null;
    const advisories: Record<string, string> = {};
    for (const draft of Object.values(view.drafts)) {
      const result = issue(snapshot, draft, this.references.revision); draft.error ||= result.error;
      if (result.advisory) advisories[draft.column] = result.advisory;
    }
    const error = !view.blocked && !view.busy && Object.keys(view.drafts).length
      ? this.validationErrors(view.drafts)[0] ?? null : view.error;
    return { ...view, error, busy: view.busy || this.references.busy, snapshot, references: snapshot?.Fields ?? null,
      projectRowId: view.drafts.ProjectID?.selection?.metadataOriginal.rowId ?? null,
      referencesBusy: this.references.busy, referenceError: this.references.error, advisories };
  }
  override closeState() {
    const view = super.view(), state = super.closeState();
    if (!view.blocked && !view.busy && Object.keys(view.drafts).length) state.error = this.validationErrors(view.drafts)[0] ?? null;
    return this.references.busy ? { ...state, busy: true, canSave: false, saveReason: 'Wait for complete-entry reference choices.' } : state;
  }
  override stage(column: string, input: TwoPageEntryInput) {
    const before = super.view();
    super.stage(column, input);
    const after = super.view();
    if ((column === 'Zone' || column === 'SubZone') && before.original && after.original &&
        !same(effective(before.original, before.drafts), effective(after.original, after.drafts))) void this.loadReferences();
  }
  selectProject(rowId: string | null) {
    this.references.projectRowId = rowId;
    try { this.stage('ProjectID', { kind: 'original' }); } finally { this.references.projectRowId = null; }
  }
  acknowledge(column: string) {
    const view = super.view(), draft = view.drafts[column], snapshot = this.references.snapshot;
    if (column === 'ProjectID' || view.busy || view.blocked || this.references.busy || !draft || draft.error || !snapshot ||
        equalCell(draft.expected, draft.value) || draft.value.storage === 'null' ||
        !snapshot.Fields.some(field => field.column === column && !field.required)) {
      throw new Error('Only an idle valid optional literal can receive a fresh reference acknowledgement.');
    }
    // Re-stage through the inherited draft owner; no second mutable draft store.
    const acknowledgement = receipt(snapshot, draft);
    this.references.pending = acknowledgement;
    try { super.stage(column, draft.input); } finally { this.references.pending = null; }
  }
  async loadReferences(): Promise<boolean> {
    const view = super.view(), state = this.references;
    if (!view.original || view.busy || view.blocked || state.disposed) throw new Error('Load idle owned complete-entry originals before references.');
    const original = view.original, prior = state.snapshot, filters = effective(original, view.drafts), revision = ++state.revision;
    state.busy = true; state.error = null; this.entryNotify();
    try {
      const snapshot = decode(await this.entryPort.readReferences(filters), this.entryOwner, this.form);
      if (state.disposed || revision !== state.revision) return false;
      if (!same(snapshot.Original, original) || !equalCell(snapshot.Zone, filters.zone) || !equalCell(snapshot.SubZone, filters.subZone) ||
          !same(effective(original, super.view().drafts), filters) || !prior ||
          snapshot.ProjectSource !== prior.ProjectSource || snapshot.WorkingSource !== prior.WorkingSource ||
          !same(snapshot.Policies, prior.Policies) || !same(snapshot.ProjectChoices, prior.ProjectChoices)) {
        throw new Error('Complete-entry original/filter/source/preference/project receipt is stale; Undo/reload.');
      }
      state.snapshot = snapshot;
      return true;
    } catch (cause) {
      if (state.disposed || revision !== state.revision) return false;
      state.error = `Complete-entry reference read failed: ${String(cause)}`;
      return false;
    } finally {
      if (!state.disposed && revision === state.revision) { state.busy = false; this.entryNotify(); }
    }
  }
  override cancel() {
    super.cancel();
    if (this.references.busy) {
      this.references.revision++; this.references.busy = false; this.entryPort.cancelRead();
      this.references.error = 'Complete-entry reference read cancelled; reload explicitly.'; this.entryNotify();
    }
  }
  override async load() {
    if (this.references.busy) throw new Error('Wait for complete-entry reference choices.');
    return super.load();
  }
  override async restore(action: Parameters<TwoPageEntryPort['restore']>[1]) {
    if (this.references.busy) throw new Error('Wait for complete-entry reference choices.');
    return super.restore(action);
  }
  override async undo() {
    if (super.view().busy) throw new Error('Wait for complete-entry operation.');
    this.cancel();
    return super.undo();
  }
  override dispose() {
    super.dispose(); this.references.revision++; this.references.disposed = true; this.references.busy = false;
  }
}
