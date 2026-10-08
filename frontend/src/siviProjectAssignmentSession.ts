import type { ProjectMetadataCell, SIVIProjectAssignmentWrite, SIVIProjectSelection } from '../bindings/github.com/boostao/vpro-wails';
import type { SIVIParentOriginal } from './siviParentEditor';
import type { EditorCloseState } from './closeLifecycle';
import { equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { siviParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import { siviProjectChoicesFromWire, type SIVIProjectPhysicalChoices } from './siviParentSourceSession';
import {
  SIVIParentScopedWriteSession, type SIVIParentWritePort, type SIVIParentWriteView,
} from './siviParentWriteSession';

export type SIVIProjectAssignmentInput = { kind: 'original' } | { kind: 'selection'; rowId: string };
export interface SIVIProjectAssignmentDrafts {
  rowId: string | null;
  selection: SIVIProjectSelection | null;
  error: string | null;
}
export interface SIVIProjectAssignmentView extends SIVIParentWriteView<SIVIProjectAssignmentDrafts> {
  choices: SIVIProjectPhysicalChoices | null;
  available: boolean;
  diagnostic: string | null;
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const empty = (): SIVIProjectAssignmentDrafts => ({ rowId: null, selection: null, error: null });

export function siviProjectAssignmentValueError(expected: ProjectMetadataCell, value: ProjectMetadataCell): string | null {
  if (equalCell(expected, value)) return null;
  if (expected.storage === 'blob') return 'ProjectID historical BLOB replacement has no lossless source audit representation; retain original storage.';
  if (value.storage !== 'text' || value.text === null || value.text === '') {
    return 'New ProjectID assignments require an existing nonempty TEXT definition; NULL/empty completion is unavailable.';
  }
  if (!wellFormedUTF16(value.text) || value.text.includes('\0') || value.text.length > 30) {
    return 'ProjectID exceeds30 UTF-16 units or contains malformed text; literal metadata was not repaired.';
  }
  return null;
}

function stage(original: SIVIParentOriginal, choices: SIVIProjectPhysicalChoices | null,
  input: SIVIProjectAssignmentInput): SIVIProjectAssignmentDrafts {
  if (input.kind === 'original') return empty();
  const row = choices?.Choices.rows.find(value => value.rowId === input.rowId);
  if (!choices || !row) {
    return { rowId: input.rowId, selection: null, error: 'Select an observed physical ProjectID definition; free text/metadata creation is unavailable.' };
  }
  const binding = original.Bindings.find(value => value.Binding === 'ProjectID');
  if (!binding || binding.ControlID !== 'form:frmSIVIsite/ProjectID' || binding.Table !== original.EnvTable) {
    throw new Error('ProjectID requires its exact owned Env binding.');
  }
  const expected = original.Rows[0].Env.cells[binding.Column], value = row.cells[0];
  const error = siviProjectAssignmentValueError(expected, value);
  return {
    rowId: input.rowId, error, selection: structuredClone({
      contextId: original.ContextID, controlId: binding.ControlID, table: binding.Table, rowId: original.Rows[0].Env.rowId,
      expected, sourceOption: choices.SourceOption, metadataAlias: choices.Alias, metadataTable: choices.Table,
      metadataColumns: choices.Choices.columns, metadataOriginal: row,
    }),
  };
}

function selectedValue(selection: SIVIProjectSelection): ProjectMetadataCell {
  const value = selection.metadataOriginal.cells?.[0];
  if (!value) throw new Error('ProjectID selection lost its physical metadata value.');
  return value;
}

export function siviProjectAssignmentDirty(drafts: SIVIProjectAssignmentDrafts): boolean {
  return drafts.error !== null || drafts.selection !== null &&
    !equalCell(drafts.selection.expected, selectedValue(drafts.selection));
}

export class SIVIProjectAssignmentSession extends SIVIParentScopedWriteSession<
  'ProjectID', SIVIProjectAssignmentWrite | null, SIVIProjectAssignmentDrafts, SIVIProjectAssignmentInput
> {
  private readonly currentChoices: () => SIVIProjectPhysicalChoices | null;
  private readonly availability: () => { available: boolean; diagnostic: string | null };

  constructor(owner: SIVIParentOwner, port: SIVIParentWritePort<SIVIProjectAssignmentWrite>, notify: () => void) {
    let choices: SIVIProjectPhysicalChoices | null = null;
    let available = false, diagnostic: string | null = null;
    super(owner, {
      read: () => port.read(),
      cancelRead: () => port.cancelRead(),
      restore: (historyId, action) => port.restore(historyId, action),
      refreshParent: () => port.refreshParent(),
      save: request => {
        if (!request) throw new Error('ProjectID Save requires an explicit physical selection.');
        return port.save(request);
      },
    }, notify, {
      accepts: (column): column is 'ProjectID' => column === 'ProjectID',
      unavailable: 'ProjectID assignment is unavailable; load the independently enabled owned selection editor.',
      drafts: {
        empty, dirty: siviProjectAssignmentDirty, errors: drafts => drafts.error ? [drafts.error] : [],
        stage: (original, _, __, input) => {
          if (!available) throw new Error(diagnostic ?? 'ProjectID assignment availability was not observed.');
          return stage(original, choices, input);
        },
      },
      decodeOriginal: (wire, owned) => {
        if (!record(wire)) throw new Error('Complete ProjectID parent and metadata snapshot was not returned.');
        const original = siviParentOriginalFromWire(wire.Original, owned);
        const observed = siviProjectChoicesFromWire(wire.Choices, owned);
        if (typeof wire.AssignmentAvailable !== 'boolean' || typeof wire.AssignmentDiagnostic !== 'string' ||
          wire.AssignmentDiagnostic === '' || !wellFormedUTF16(wire.AssignmentDiagnostic)) {
          throw new Error('Explicit ProjectID assignment availability and diagnostic were not returned.');
        }
        choices = observed;
        available = wire.AssignmentAvailable;
        diagnostic = wire.AssignmentDiagnostic;
        return original;
      },
      request: (original, drafts) => drafts.selection
        ? { original: structuredClone(original), selection: structuredClone(drafts.selection) } : null,
      count: request => request &&
        !equalCell(request.selection.expected, selectedValue(request.selection)) ? 1 : 0,
    });
    this.currentChoices = () => choices;
    this.availability = () => ({ available, diagnostic });
  }

  override view(): SIVIProjectAssignmentView {
    const state = super.view();
    return { ...state, choices: state.original ? structuredClone(this.currentChoices()) : null,
      available: state.original !== null && this.availability().available,
      diagnostic: state.original ? this.availability().diagnostic : null };
  }

  override closeState(): EditorCloseState {
    const state = super.closeState();
    const availability = this.availability();
    return state.canSave && !availability.available
      ? { ...state, canSave: false, saveReason: availability.diagnostic ?? 'Load explicit ProjectID assignment availability.' } : state;
  }
}
