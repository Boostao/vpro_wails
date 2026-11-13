import type { TwoPageSoilSession } from './twoPageSoilSession';
import type { TwoPageSoilOwner } from './twoPageSoilEditor';
import { wellFormedUTF16 } from './qualityEditor';

export interface TwoPageSoilActivity {
  busy: boolean;
  blocked: boolean;
  owner: TwoPageSoilOwner | null;
}

export const twoPageSoilEntries = new Map<string, { owner: TwoPageSoilOwner; session: TwoPageSoilSession; unsubscribe: () => void }>();
const observers = new Set<(activity: TwoPageSoilActivity) => void>();

export function twoPageSoilOwnerKey(owner: TwoPageSoilOwner): string {
  if (![owner.contextId, owner.project, owner.plot].every(value =>
    typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0')) ||
    !['SoilHumusXL', 'SoilMineralXL'].includes(owner.form)) throw new Error('Soil session requires an exact literal context/project/parent/source-form owner.');
  return JSON.stringify([owner.contextId, owner.project, owner.plot, owner.form]);
}

export function twoPageSoilActivity(): TwoPageSoilActivity {
  let busy = false, blocked = false, owner: TwoPageSoilOwner | null = null;
  for (const entry of twoPageSoilEntries.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}

export function notifyTwoPageSoilActivity() {
  for (const observer of observers) observer(twoPageSoilActivity());
}

export function subscribeTwoPageSoilActivity(observer: (activity: TwoPageSoilActivity) => void) {
  observers.add(observer); observer(twoPageSoilActivity());
  return () => { observers.delete(observer); };
}
