import type { PictureLifecycleSession } from './pictureLifecycleSession';
import type { PictureOwner } from './pictureRead';

export interface PictureLifecycleActivity {
  busy: boolean;
  blocked: boolean;
  owner: PictureOwner | null;
}

export const pictureLifecycleEntries = new Map<string, { owner: PictureOwner; session: PictureLifecycleSession }>();
const observers = new Set<(activity: PictureLifecycleActivity) => void>();

function activity(): PictureLifecycleActivity {
  let busy = false, blocked = false, owner: PictureOwner | null = null;
  for (const entry of pictureLifecycleEntries.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}

export function notifyPictureLifecycleActivity() {
  for (const observer of observers) observer(activity());
}

export function subscribePictureLifecycleActivity(observer: (activity: PictureLifecycleActivity) => void) {
  observers.add(observer);
  observer(activity());
  return () => { observers.delete(observer); };
}
