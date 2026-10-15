import { TwoPageVegetationService, TwoPageVegetationSpeciesListService } from '../bindings/github.com/boostao/vpro-wails';
import { TwoPageVegetationSession } from './twoPageVegetationSession';
import type { TwoPageVegetationOwner } from './twoPageVegetationWire';
import { wellFormedUTF16 } from './qualityEditor';

export const twoPageVegetationReviewEnabled = import.meta.env.VITE_TWO_PAGE_VEGETATION_REVIEW === 'true';
export const twoPageVegetationWritingEnabled = twoPageVegetationReviewEnabled && import.meta.env.VITE_TWO_PAGE_VEGETATION_WRITING === 'true';
export const twoPageVegetationSpeciesListEnabled = twoPageVegetationWritingEnabled && import.meta.env.VITE_TWO_PAGE_VEGETATION_SPECIES_LIST === 'true';
const sessions = new Map<string, { owner: TwoPageVegetationOwner; session: TwoPageVegetationSession; unsubscribe: () => void }>();
const observers = new Set<(activity: TwoPageVegetationActivity) => void>();
export interface TwoPageVegetationActivity {
  busy: boolean;
  blocked: boolean;
  owner: TwoPageVegetationOwner | null;
}
function ownerKey(owner: TwoPageVegetationOwner): string {
  if (![owner.contextId, owner.project, owner.plot].every(value =>
    typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0')) ||
    !['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL'].includes(owner.form)) {
    throw new Error('Vegetation session requires an exact literal context/project/parent/source-form owner.');
  }
  return JSON.stringify([owner.contextId, owner.project, owner.plot, owner.form]);
}
export function twoPageVegetationActivity(excludingOwner?: TwoPageVegetationOwner): TwoPageVegetationActivity {
  const excludingKey = excludingOwner ? ownerKey(excludingOwner) : null;
  let busy = false, blocked = false, owner: TwoPageVegetationOwner | null = null;
  for (const [key, entry] of sessions) {
    if (key === excludingKey) continue;
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}
function changed() { for (const observer of observers) observer(twoPageVegetationActivity()); }
export function subscribeTwoPageVegetationActivity(observer: (activity: TwoPageVegetationActivity) => void) {
  observers.add(observer); observer(twoPageVegetationActivity());
  return () => { observers.delete(observer); };
}
export function twoPageVegetationSession(owner: TwoPageVegetationOwner): TwoPageVegetationSession {
  if (!twoPageVegetationReviewEnabled) throw new Error('Two-page vegetation review is disabled.');
  const key = ownerKey(owner);
  const existing = sessions.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plot: owner.plot, form: owner.form };
  const { contextId, plot, form } = retainedOwner;
  const session = new TwoPageVegetationSession(retainedOwner, {
    read: () => TwoPageVegetationService.GetOriginal(contextId, plot, form),
    save: request => TwoPageVegetationService.Save(contextId, request),
    saveSpeciesList: request => TwoPageVegetationSpeciesListService.Save(contextId, request),
    outcome: request => TwoPageVegetationService.RecoverOutcome(contextId, request),
    references: () => TwoPageVegetationService.GetSpeciesReferences(contextId, plot, form),
  }, twoPageVegetationWritingEnabled, twoPageVegetationSpeciesListEnabled);
  let initialized = false;
  const unsubscribe = session.subscribe(() => { if (initialized) changed(); });
  sessions.set(key, { owner: retainedOwner, session, unsubscribe });
  initialized = true;
  return session;
}
export function releaseTwoPageVegetationSession(owner: TwoPageVegetationOwner): boolean {
  const key = ownerKey(owner);
  const entry = sessions.get(key);
  if (!entry) return false;
  const state = entry.session.closeState();
  if (state.busy || state.blocked || state.unsaved) throw new Error('Resolve retained vegetation drafts/operations/receipts before releasing their owner.');
  if (entry.session.view().receipts.length) return false;
  entry.session.dispose(); entry.unsubscribe(); sessions.delete(key); changed();
  return true;
}
