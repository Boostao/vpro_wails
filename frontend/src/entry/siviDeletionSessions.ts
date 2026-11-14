import { PlotEntryService } from '../../bindings/github.com/boostao/vpro-wails';
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
      targets: () => PlotEntryService.GetSIVIDeletionTargets(contextId, plot),
      original: (form, rowId) => PlotEntryService.GetSIVIDeletionOriginal(contextId, plot, form, rowId),
      delete: request => PlotEntryService.DeleteSIVIVegetation(contextId, request),
      receipt: request => PlotEntryService.LookupSIVIDeletionReceipt(contextId, request),
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
      history: () => PlotEntryService.GetSIVIDeletionHistory(contextId, plot),
      review: historyId => PlotEntryService.ReviewSIVIDeletionRestoration(contextId, plot, historyId),
      restore: request => PlotEntryService.RestoreSIVIDeletion(contextId, request),
      receipt: request => PlotEntryService.LookupSIVIDeletionRestorationReceipt(contextId, request),
    });
    restorations.set(identity, session);
  }
  return session;
}
