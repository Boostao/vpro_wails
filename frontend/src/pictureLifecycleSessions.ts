import { PictureDeletionService, PictureService } from '../bindings/github.com/boostao/vpro-wails';
import { PictureLifecycleSession } from './pictureLifecycleSession';
import type { PictureOwner } from './pictureRead';

const sessions = new Map<string, { owner: PictureOwner; session: PictureLifecycleSession }>();
const observers = new Set<(activity: PictureLifecycleActivity) => void>();
export interface PictureLifecycleActivity {
  busy: boolean;
  blocked: boolean;
  owner: PictureOwner | null;
}
function activity(): PictureLifecycleActivity {
  let busy = false, blocked = false, owner: PictureOwner | null = null;
  for (const entry of sessions.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}
function changed() { for (const observer of observers) observer(activity()); }
export function subscribePictureLifecycleActivity(observer: (activity: PictureLifecycleActivity) => void) {
  observers.add(observer); observer(activity());
  return () => { observers.delete(observer); };
}
export function pictureLifecycleSession(owner: PictureOwner): PictureLifecycleSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plotNumber]);
  const existing = sessions.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  const { contextId, plotNumber } = retainedOwner;
  const session = new PictureLifecycleSession(retainedOwner, {
    read: () => PictureService.GetMetadata(contextId, plotNumber),
    history: () => PictureDeletionService.GetHistory(contextId, plotNumber),
    review: historyId => PictureDeletionService.ReviewRestoration(contextId, plotNumber, historyId),
    delete: request => PictureDeletionService.Delete(contextId, request),
    deletionReceipt: request => PictureDeletionService.LookupReceipt(contextId, request),
    restore: request => PictureDeletionService.Restore(contextId, request),
    restorationReceipt: request => PictureDeletionService.LookupRestorationReceipt(contextId, request),
  });
  sessions.set(key, { owner: retainedOwner, session });
  let initialized = false;
  session.subscribe(() => { if (initialized) changed(); });
  initialized = true;
  return session;
}
