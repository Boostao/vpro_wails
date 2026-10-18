import { TwoPageSoilService } from '../bindings/github.com/boostao/vpro-wails';
import { TwoPageSoilSession } from './twoPageSoilSession';
import type { TwoPageSoilOwner } from './twoPageSoilEditor';
import { wellFormedUTF16 } from './qualityEditor';

export const twoPageSoilReviewEnabled = import.meta.env.VITE_TWO_PAGE_SOIL_REVIEW === 'true';
export const twoPageSoilWritingEnabled = twoPageSoilReviewEnabled && import.meta.env.VITE_TWO_PAGE_SOIL_WRITING === 'true';
export const twoPageSoilCreationEnabled = twoPageSoilWritingEnabled && import.meta.env.VITE_TWO_PAGE_SOIL_CREATION === 'true';
const sessions = new Map<string, { owner: TwoPageSoilOwner; session: TwoPageSoilSession; unsubscribe: () => void }>();
const observers = new Set<(activity: TwoPageSoilActivity) => void>();
export interface TwoPageSoilActivity {
  busy: boolean;
  blocked: boolean;
  owner: TwoPageSoilOwner | null;
}
function ownerKey(owner: TwoPageSoilOwner): string {
  if (![owner.contextId, owner.project, owner.plot].every(value =>
    typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0')) ||
    !['SoilHumusXL', 'SoilMineralXL'].includes(owner.form)) throw new Error('Soil session requires an exact literal context/project/parent/source-form owner.');
  return JSON.stringify([owner.contextId, owner.project, owner.plot, owner.form]);
}
export function twoPageSoilActivity(): TwoPageSoilActivity {
  let busy = false, blocked = false, owner: TwoPageSoilOwner | null = null;
  for (const entry of sessions.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}
function changed() { for (const observer of observers) observer(twoPageSoilActivity()); }
export function subscribeTwoPageSoilActivity(observer: (activity: TwoPageSoilActivity) => void) {
  observers.add(observer); observer(twoPageSoilActivity());
  return () => { observers.delete(observer); };
}
export function twoPageSoilSession(owner: TwoPageSoilOwner): TwoPageSoilSession {
  if (!twoPageSoilReviewEnabled) throw new Error('Two-page soil review is disabled.');
  const key = ownerKey(owner);
  const existing = sessions.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plot: owner.plot, form: owner.form };
  const { contextId, plot, form } = retainedOwner;
  const session = new TwoPageSoilSession(retainedOwner, {
    read: () => TwoPageSoilService.GetOriginal(contextId, plot, form),
    save: request => TwoPageSoilService.Save(contextId, request),
    create: request => TwoPageSoilService.CreateHorizon(contextId, request),
    outcome: request => TwoPageSoilService.RecoverOutcome(contextId, request),
  }, twoPageSoilWritingEnabled, twoPageSoilCreationEnabled);
  let initialized = false;
  const unsubscribe = session.subscribe(() => { if (initialized) changed(); });
  sessions.set(key, { owner: retainedOwner, session, unsubscribe });
  initialized = true;
  return session;
}
export function releaseTwoPageSoilSession(owner: TwoPageSoilOwner): boolean {
  const key = ownerKey(owner);
  const entry = sessions.get(key);
  if (!entry) return false;
  const state = entry.session.closeState();
  if (state.busy || state.blocked || state.unsaved) throw new Error('Resolve retained soil drafts/operations/receipts before releasing their owner.');
  if (entry.session.view().receipts.length) return false;
  entry.session.dispose(); entry.unsubscribe(); sessions.delete(key); changed();
  return true;
}
