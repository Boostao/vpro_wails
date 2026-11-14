import { PlotEntryService } from '../../bindings/github.com/boostao/vpro-wails';
import { SIVICreationSession } from './siviCreationSession';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

const sessions = new Map<string, SIVICreationSession>();

export function siviCreationSession(owner: SIVISpeciesOwner): SIVICreationSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plot]);
  let session = sessions.get(key);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVICreationSession({ contextId, project, plot }, {
      references: () => PlotEntryService.GetSIVICreationReferences(contextId, plot),
      create: request => PlotEntryService.CreateSIVIVegetation(contextId, request),
      receipt: request => PlotEntryService.LookupSIVICreationReceipt(contextId, request),
    });
    sessions.set(key, session);
  }
  return session;
}
