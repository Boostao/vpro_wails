import { PlotEntryService } from '../../bindings/github.com/boostao/vpro-wails';
import { TwoPageSoilSession } from './twoPageSoilSession';
import type { TwoPageSoilOwner } from './twoPageSoilEditor';
import { notifyTwoPageSoilActivity, subscribeTwoPageSoilActivity, twoPageSoilActivity, twoPageSoilEntries, twoPageSoilOwnerKey, type TwoPageSoilActivity } from './twoPageSoilActivity';

export const twoPageSoilReviewEnabled = true;
export const twoPageSoilWritingEnabled = true;
export const twoPageSoilCreationEnabled = true;
export { subscribeTwoPageSoilActivity, twoPageSoilActivity, type TwoPageSoilActivity };
export function twoPageSoilSession(owner: TwoPageSoilOwner): TwoPageSoilSession {
  const key = twoPageSoilOwnerKey(owner);
  const existing = twoPageSoilEntries.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plot: owner.plot, form: owner.form };
  const { contextId, plot, form } = retainedOwner;
  const session = new TwoPageSoilSession(retainedOwner, {
    read: () => PlotEntryService.GetTwoPageSoilOriginal(contextId, plot, form),
    save: request => PlotEntryService.SaveTwoPageSoil(contextId, request),
    create: request => PlotEntryService.CreateTwoPageSoilHorizon(contextId, request),
    outcome: request => PlotEntryService.RecoverTwoPageSoilOutcome(contextId, request),
  }, twoPageSoilWritingEnabled, twoPageSoilCreationEnabled);
  let initialized = false;
  const unsubscribe = session.subscribe(() => { if (initialized) notifyTwoPageSoilActivity(); });
  twoPageSoilEntries.set(key, { owner: retainedOwner, session, unsubscribe });
  initialized = true;
  return session;
}
export function releaseTwoPageSoilSession(owner: TwoPageSoilOwner): boolean {
  const key = twoPageSoilOwnerKey(owner);
  const entry = twoPageSoilEntries.get(key);
  if (!entry) return false;
  const state = entry.session.closeState();
  if (state.busy || state.blocked || state.unsaved) throw new Error('Resolve retained soil drafts/operations/receipts before releasing their owner.');
  if (entry.session.view().receipts.length) return false;
  entry.session.dispose(); entry.unsubscribe(); twoPageSoilEntries.delete(key); notifyTwoPageSoilActivity();
  return true;
}
