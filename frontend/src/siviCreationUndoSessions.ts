import { SIVICreationUndoService } from '../bindings/github.com/boostao/vpro-wails';
import { SIVICreationUndoSession } from './siviCreationUndoSession';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

const sessions = new Map<string, SIVICreationUndoSession>();
export function siviCreationUndoSession(owner: SIVISpeciesOwner): SIVICreationUndoSession {
  const identity = JSON.stringify([owner.contextId, owner.project, owner.plot]);
  let session = sessions.get(identity);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVICreationUndoSession({ contextId, project, plot }, {
      history: () => SIVICreationUndoService.GetHistory(contextId, plot),
      review: historyId => SIVICreationUndoService.Review(contextId, plot, historyId),
      undo: request => SIVICreationUndoService.Undo(contextId, request),
      receipt: request => SIVICreationUndoService.LookupReceipt(contextId, request),
    });
    sessions.set(identity, session);
  }
  return session;
}
