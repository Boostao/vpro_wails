import { SIVIDeletionService } from '../bindings/github.com/boostao/vpro-wails';
import { SIVIDeletionSession } from './siviDeletionSession';
import { SIVIDeletionRestorationSession } from './siviDeletionRestorationSession';
import type { SIVISpeciesOwner } from './siviSpeciesEditor';

const deletions = new Map<string, SIVIDeletionSession>();
const restorations = new Map<string, SIVIDeletionRestorationSession>();
const key = (owner: SIVISpeciesOwner) => JSON.stringify([owner.contextId, owner.project, owner.plot]);
export function siviDeletionSession(owner: SIVISpeciesOwner): SIVIDeletionSession {
  const identity = key(owner);
  let session = deletions.get(identity);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVIDeletionSession({ contextId, project, plot }, {
      targets: () => SIVIDeletionService.GetTargets(contextId, plot),
      original: (form, rowId) => SIVIDeletionService.GetOriginal(contextId, plot, form, rowId),
      delete: request => SIVIDeletionService.Delete(contextId, request),
      receipt: request => SIVIDeletionService.LookupReceipt(contextId, request),
    });
    deletions.set(identity, session);
  }
  return session;
}
export function siviDeletionRestorationSession(owner: SIVISpeciesOwner): SIVIDeletionRestorationSession {
  const identity = key(owner);
  let session = restorations.get(identity);
  if (!session) {
    const { contextId, project, plot } = owner;
    session = new SIVIDeletionRestorationSession({ contextId, project, plot }, {
      history: () => SIVIDeletionService.GetHistory(contextId, plot),
      review: historyId => SIVIDeletionService.ReviewRestoration(contextId, plot, historyId),
      restore: request => SIVIDeletionService.Restore(contextId, request),
      receipt: request => SIVIDeletionService.LookupRestorationReceipt(contextId, request),
    });
    restorations.set(identity, session);
  }
  return session;
}
