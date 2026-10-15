import { fieldListOwner, SummaryVegetationFieldListSession, type FieldListOwner, type FieldListPort } from './summaryVegetationFieldList';

export interface FieldListEnvironment {
  VITE_SUMMARY_VEGETATION?: unknown;
  VITE_SUMMARY_VEGETATION_FIELD_LIST?: unknown;
}
export function fieldListEnabled(environment: FieldListEnvironment): boolean {
  return environment.VITE_SUMMARY_VEGETATION === 'true' && environment.VITE_SUMMARY_VEGETATION_FIELD_LIST === 'true';
}
export const summaryVegetationFieldListEnabled = fieldListEnabled(import.meta.env);
export interface FieldListActivity { busy: boolean; blocked: boolean; owner: FieldListOwner | null }
export function fieldListRegistry(createPort: (owner: FieldListOwner) => FieldListPort,
  environment: FieldListEnvironment = import.meta.env, newRequestId?: () => string) {
  const enabled = fieldListEnabled(environment);
  const sessions = new Map<string, { owner: FieldListOwner; session: SummaryVegetationFieldListSession; unsubscribe: () => void }>();
  const listeners = new Set<(activity: FieldListActivity) => void>();
  const key = (owner: FieldListOwner) => JSON.stringify(fieldListOwner(owner));
  const activity = (): FieldListActivity => {
    let busy = false, blocked = false, owner: FieldListOwner | null = null;
    for (const entry of sessions.values()) {
      const state = entry.session.closeState();
      busy ||= state.busy; blocked ||= state.blocked || state.unsaved;
      if (!owner && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
    }
    return { busy, blocked, owner };
  };
  const changed = () => { for (const listener of listeners) listener(activity()); };
  return {
    enabled, activity,
    subscribe(listener: (activity: FieldListActivity) => void) {
      listeners.add(listener); listener(activity());
      return () => { listeners.delete(listener); };
    },
    session(ownerValue: FieldListOwner) {
      if (!enabled) throw new Error('Private Summary vegetation field-list flag/precondition disabled.');
      const owner = fieldListOwner(ownerValue), id = key(owner), existing = sessions.get(id);
      if (existing) return existing.session;
      if (sessions.size >= 16) throw new Error('Retained field-list context capacity reached; no state is evicted.');
      const session = new SummaryVegetationFieldListSession(owner, createPort(owner), enabled, newRequestId);
      let initialized = false;
      const unsubscribe = session.subscribe(() => { if (initialized) changed(); });
      sessions.set(id, { owner, session, unsubscribe }); initialized = true; changed();
      return session;
    },
    detach(owner: FieldListOwner) { sessions.get(key(owner))?.session.detach(); },
    release(owner: FieldListOwner) {
      const id = key(owner), entry = sessions.get(id);
      if (!entry) return true;
      const state = entry.session.closeState();
      if (state.busy || state.blocked || state.unsaved) throw new Error('Resolve retained field-list lifecycle holds before release.');
      if (entry.session.view().receipts.length) return false;
      entry.session.dispose(); entry.unsubscribe(); sessions.delete(id); changed(); return true;
    },
  };
}
