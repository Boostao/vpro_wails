import { PlotEntryService } from '../../bindings/github.com/boostao/vpro-wails';
import { TwoPageVegetationSession } from './twoPageVegetationSession';
import type { TwoPageVegetationOwner } from './twoPageVegetationWire';
import { notifyTwoPageVegetationActivity, subscribeTwoPageVegetationActivity, twoPageVegetationActivity, twoPageVegetationEntries, twoPageVegetationOwnerKey, type TwoPageVegetationActivity } from './twoPageVegetationActivity';

export const twoPageVegetationReviewEnabled = true;
export const twoPageVegetationWritingEnabled = true;
export const twoPageVegetationSpeciesListEnabled = true;
export { subscribeTwoPageVegetationActivity, twoPageVegetationActivity, type TwoPageVegetationActivity };
export function twoPageVegetationSession(owner: TwoPageVegetationOwner): TwoPageVegetationSession {
  const key = twoPageVegetationOwnerKey(owner);
  const existing = twoPageVegetationEntries.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plot: owner.plot, form: owner.form };
  const { contextId, plot, form } = retainedOwner;
  const session = new TwoPageVegetationSession(retainedOwner, {
    read: () => PlotEntryService.GetTwoPageVegetationOriginal(contextId, plot, form),
    save: request => PlotEntryService.SaveTwoPageVegetation(contextId, request),
    saveSpeciesList: request => PlotEntryService.SaveTwoPageVegetationSpeciesList(contextId, request),
    outcome: request => PlotEntryService.RecoverTwoPageVegetationOutcome(contextId, request),
    references: () => PlotEntryService.GetTwoPageVegetationSpeciesReferences(contextId, plot, form),
  }, twoPageVegetationWritingEnabled, twoPageVegetationSpeciesListEnabled);
  let initialized = false;
  const unsubscribe = session.subscribe(() => { if (initialized) notifyTwoPageVegetationActivity(); });
  twoPageVegetationEntries.set(key, { owner: retainedOwner, session, unsubscribe });
  initialized = true;
  return session;
}
export function releaseTwoPageVegetationSession(owner: TwoPageVegetationOwner): boolean {
  const key = twoPageVegetationOwnerKey(owner);
  const entry = twoPageVegetationEntries.get(key);
  if (!entry) return false;
  const state = entry.session.closeState();
  if (state.busy || state.blocked || state.unsaved) throw new Error('Resolve retained vegetation drafts/operations/receipts before releasing their owner.');
  if (entry.session.view().receipts.length) return false;
  entry.session.dispose(); entry.unsubscribe(); twoPageVegetationEntries.delete(key); notifyTwoPageVegetationActivity();
  return true;
}
