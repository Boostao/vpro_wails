import { SIVICreationService } from '../bindings/github.com/boostao/vpro-wails';
import { SIVICreationSession } from './siviCreationSession';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

const sessions = new Map<string, SIVICreationSession>();

export function siviCreationSession(owner: SIVISpeciesOwner): SIVICreationSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plot]);
  let session = sessions.get(key);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVICreationSession({ contextId, project, plot }, {
      references: () => SIVICreationService.GetReferences(contextId, plot),
      create: request => SIVICreationService.Create(contextId, request),
      receipt: request => SIVICreationService.LookupReceipt(contextId, request),
    });
    sessions.set(key, session);
  }
  return session;
}
