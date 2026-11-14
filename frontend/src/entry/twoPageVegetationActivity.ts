import type { TwoPageVegetationSession } from './twoPageVegetationSession';
import type { TwoPageVegetationOwner } from './twoPageVegetationWire';
import { wellFormedUTF16 } from './qualityEditor';

export interface TwoPageVegetationActivity {
  busy: boolean;
  blocked: boolean;
  owner: TwoPageVegetationOwner | null;
}

export const twoPageVegetationEntries = new Map<string, { owner: TwoPageVegetationOwner; session: TwoPageVegetationSession; unsubscribe: () => void }>();
const observers = new Set<(activity: TwoPageVegetationActivity) => void>();

export function twoPageVegetationOwnerKey(owner: TwoPageVegetationOwner): string {
  if (![owner.contextId, owner.project, owner.plot].every(value =>
    typeof value === 'string' && value !== '' && wellFormedUTF16(value) && !value.includes('\0')) ||
    !['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL'].includes(owner.form)) {
    throw new Error('Vegetation session requires an exact literal context/project/parent/source-form owner.');
  }
  return JSON.stringify([owner.contextId, owner.project, owner.plot, owner.form]);
}

export function twoPageVegetationActivity(excludingOwner?: TwoPageVegetationOwner): TwoPageVegetationActivity {
  const excludingKey = excludingOwner ? twoPageVegetationOwnerKey(excludingOwner) : null;
  let busy = false, blocked = false, owner: TwoPageVegetationOwner | null = null;
  for (const [key, entry] of twoPageVegetationEntries) {
    if (key === excludingKey) continue;
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}

export function notifyTwoPageVegetationActivity() {
  for (const observer of observers) observer(twoPageVegetationActivity());
}

export function subscribeTwoPageVegetationActivity(observer: (activity: TwoPageVegetationActivity) => void) {
  observers.add(observer); observer(twoPageVegetationActivity());
  return () => { observers.delete(observer); };
}
