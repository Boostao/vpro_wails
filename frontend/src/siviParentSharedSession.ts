import source from '../../resources/fs1333-sivi-layout.json';
import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { ordinaryField, ordinaryNumberValue, ordinaryTextError } from './ordinaryEditor';
import { equalCell, metadataCellText } from './projectMetadataEditor';
import { validateSIVIParentOriginal, type SIVIParentOriginal } from './siviParentEditor';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort, type SIVIParentWriteView } from './siviParentWriteSession';
import type { SIVIParentOwner } from './siviParentTransport';

export const siviParentSharedColumns = [
  'AirPhotoNum', 'XCoord', 'YCoord',
  'StrataCoverTree', 'StrataCoverShrub', 'StrataCoverHerb', 'StrataCoverMoss', 'VegNotes',
] as const;
export type SIVIParentSharedColumn = typeof siviParentSharedColumns[number];
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
export type SIVIParentSharedView = SIVIParentWriteView<SIVIParentSharedDrafts>;
export function isSIVIParentSharedColumn(value: string): value is SIVIParentSharedColumn {
  return siviParentSharedColumns.some(column => column === value);
}

export const siviParentSharedFields = siviParentSharedColumns.map(column => {
  const fields = source.forms[0].fields.filter(field => field.binding === column);
  const policy = ordinaryField(column);
  if (fields.length !== 1 || !policy) throw new Error(`SIVI shared field ${column} has no unique source/proven policy.`);
  return { column, controlId: fields[0].controlId, label: fields[0].caption || fields[0].controlName,
    page: 'pageId' in fields[0] ? fields[0].pageId : null, policy };
});

export function siviParentSharedCell(original: SIVIParentOriginal, column: SIVIParentSharedColumn): ProjectMetadataCell {
  const binding = original.Bindings.find(binding => binding.Binding === column);
  if (!binding || binding.Implicit || binding.Table !== original.EnvTable) {
    throw new Error('SIVI shared-field source/physical ownership changed.');
  }
  return structuredClone(original.Rows[0].Env.cells[binding.Column]);
}
const nullCell = (): ProjectMetadataCell => ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });
export function siviParentSharedErrors(drafts: SIVIParentSharedDrafts): string[] {
  return Object.values(drafts).flatMap(draft => draft?.error ? [draft.error] : []);
}
export function siviParentSharedDirty(drafts: SIVIParentSharedDrafts): boolean {
  return Object.values(drafts).some(draft => draft && (draft.error !== null || !equalCell(draft.expected, draft.value)));
}
function stage(original: SIVIParentOriginal, drafts: SIVIParentSharedDrafts,
  column: SIVIParentSharedColumn, input: SIVIParentSharedInput): SIVIParentSharedDrafts {
  if (!input || !['original', 'clear', 'text'].includes(input.kind) ||
    input.kind === 'text' && typeof input.raw !== 'string') {
    throw new Error('SIVI shared fields require an explicit raw entry, NULL or original storage.');
  }
  const field = siviParentSharedFields.find(field => field.column === column);
  if (!field) throw new Error('SIVI shared field is outside the bounded editor.');
  const expected = siviParentSharedCell(original, column);
  let value = structuredClone(expected), error: string | null = null;
  if (input.kind === 'clear') value = nullCell();
  else if (input.kind === 'text' && !(expected.storage !== 'null' && input.raw === metadataCellText(expected))) {
    const policy = field.policy;
    if (policy.kind === 'text' || policy.kind === 'memo') {
      error = ordinaryTextError(policy, input.raw, expected.storage === 'text' ? expected.text : null);
      value = { ...nullCell(), storage: 'text', text: input.raw };
    } else {
      const parsed = ordinaryNumberValue(policy, input.raw,
        expected.storage === 'real' ? expected.real : null);
      error = parsed.error;
      value = parsed.value === null ? nullCell() : { ...nullCell(), storage: 'real', real: parsed.value };
    }
  }
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  return { ...drafts, [column]: { contextId: original.ContextID, table: original.EnvTable,
    rowId: original.Rows[0].Env.rowId, column, expected, value, input: structuredClone(input), error } };
}

export function siviParentSharedRequest(original: SIVIParentOriginal, drafts: SIVIParentSharedDrafts): SIVIParentSharedWrite {
  const reviewed = validateSIVIParentOriginal(original);
  const edits: SIVIParentSharedEdit[] = [];
  for (const [column, draft] of Object.entries(drafts)) {
    if (!isSIVIParentSharedColumn(column) || !draft || draft.column !== column) {
      throw new Error('SIVI shared writes exclude callbacks, ProjectID and other header fields.');
    }
    const verified = stage(reviewed, {}, column, draft.input)[column]!;
    if (!equalCell(verified.expected, draft.expected) || !equalCell(verified.value, draft.value) ||
      verified.contextId !== draft.contextId || verified.table !== draft.table || verified.rowId !== draft.rowId) {
      throw new Error('SIVI shared draft belongs to another original/physical owner.');
    }
    if (verified.error) throw new Error(verified.error);
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
  constructor(owner: SIVIParentOwner, port: SIVIParentWritePort<SIVIParentSharedWrite>, notify: () => void) {
    super(owner, conservativePort(port), notify, {
      accepts: isSIVIParentSharedColumn,
      unavailable: 'SIVI shared-field editor requires owned originals; callback actions and XL lifecycle are unavailable.',
      drafts: { empty: () => ({}), stage, errors: siviParentSharedErrors, dirty: siviParentSharedDirty },
      request: siviParentSharedRequest, count: request => request.edits.length,
    });
  }
}
