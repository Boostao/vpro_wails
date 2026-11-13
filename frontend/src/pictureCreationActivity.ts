import type { PictureCreationSession } from './pictureCreationSession';
import type { PictureOwner } from './pictureRead';

export interface PictureCreationActivity {
  busy: boolean;
  blocked: boolean;
  owner: PictureOwner | null;
}

export const pictureCreationEntries = new Map<string, { owner: PictureOwner; session: PictureCreationSession }>();
const observers = new Set<(activity: PictureCreationActivity) => void>();

function activity(): PictureCreationActivity {
  let busy = false, blocked = false, owner: PictureOwner | null = null;
  for (const entry of pictureCreationEntries.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}

export function notifyPictureCreationActivity() {
  for (const observer of observers) observer(activity());
}

export function subscribePictureCreationActivity(observer: (activity: PictureCreationActivity) => void) {
  observers.add(observer);
  observer(activity());
  return () => { observers.delete(observer); };
}
