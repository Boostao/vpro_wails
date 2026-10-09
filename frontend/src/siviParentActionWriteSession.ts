import type { SIVIParentActionWrite } from '../bindings/github.com/boostao/vpro-wails';
import type { SIVIParentColumn, SIVIParentDrafts, SIVIParentOriginal } from './siviParentEditor';
import { siviParentProposal } from './siviParentEditor';
import type { SIVIParentOwner } from './siviParentTransport';
import { SIVIParentScopedWriteSession, siviParentWriteDraftScope, siviParentWriteErrors, type SIVIParentWritePort } from './siviParentWriteSession';

export const siviParentActionColumns = ['PlotType', 'SpeciesListComplete'] as const satisfies readonly SIVIParentColumn[];
export type SIVIParentActionColumn = typeof siviParentActionColumns[number];
export function isSIVIParentActionColumn(value: string): value is SIVIParentActionColumn {
  return siviParentActionColumns.some(column => column === value);
}

export function siviParentActionRequest(original: SIVIParentOriginal, drafts: SIVIParentDrafts): SIVIParentActionWrite {
  if (Object.keys(drafts).some(column => !isSIVIParentActionColumn(column))) {
    throw new Error('SIVI source actions exclude direct fields and ProjectID assignment.');
  }
  const invalid = siviParentWriteErrors(drafts)[0];
  if (invalid) throw new Error(invalid);
  const proposal = siviParentProposal(original, drafts);
  if (proposal.scalars.length || proposal.options.length || proposal.text.length || proposal.categorical.length) {
    throw new Error('SIVI source action transport contains an unavailable direct domain.');
  }
  return { original: structuredClone(proposal.original), actions: proposal.actions.map(action => ({
    contextId: action.contextId, controlId: action.controlId, table: action.table, rowId: action.rowId,
    expected: structuredClone(action.expected), option: action.option,
  })) };
}

export class SIVIParentActionWriteSession extends SIVIParentScopedWriteSession<SIVIParentActionColumn, SIVIParentActionWrite> {
  constructor(owner: SIVIParentOwner, port: SIVIParentWritePort<SIVIParentActionWrite>, notify: () => void) {
    super(owner, port, notify, {
      accepts: isSIVIParentActionColumn,
      drafts: siviParentWriteDraftScope,
      unavailable: 'SIVI source action editing is unavailable; load the independently enabled owned editor.',
      request: siviParentActionRequest,
      count: request => request.actions?.length ?? 0,
      verifyAcknowledgement: (result, request) => {
        const refresh = request.actions?.some(action => action.controlId === 'form:frmSIVIsite/optPlotType') ?? false;
        if (typeof result.SourceRefreshRequired !== 'boolean' || result.SourceRefreshRequired !== refresh) {
          throw new Error('SIVI source Refresh acknowledgement differs from the planned callback; observe storage, never replay.');
        }
      },
    });
  }
}
