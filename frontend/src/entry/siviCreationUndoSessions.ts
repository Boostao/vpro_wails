import { PlotEntryService } from '../../bindings/github.com/boostao/vpro-wails';
import { SIVICreationUndoSession } from './siviCreationUndoSession';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

const sessions = new Map<string, SIVICreationUndoSession>();
export function siviCreationUndoSession(owner: SIVISpeciesOwner): SIVICreationUndoSession {
  const identity = JSON.stringify([owner.contextId, owner.project, owner.plot]);
  let session = sessions.get(identity);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVICreationUndoSession({ contextId, project, plot }, {
      history: () => PlotEntryService.GetSIVICreationUndoHistory(contextId, plot),
      review: historyId => PlotEntryService.ReviewSIVICreationUndo(contextId, plot, historyId),
      undo: request => PlotEntryService.UndoSIVICreation(contextId, request),
      receipt: request => PlotEntryService.LookupSIVICreationUndoReceipt(contextId, request),
    });
    sessions.set(identity, session);
  }
  return session;
}
