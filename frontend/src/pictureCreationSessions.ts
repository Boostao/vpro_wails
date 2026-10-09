import { PictureCreationService } from '../bindings/github.com/boostao/vpro-wails';
import { PictureCreationSession } from './pictureCreationSession';
import type { PictureOwner } from './pictureRead';

const sessions = new Map<string, { owner: PictureOwner; session: PictureCreationSession }>();
const observers = new Set<(activity: PictureCreationActivity) => void>();
export interface PictureCreationActivity {
  busy: boolean;
  blocked: boolean;
  owner: PictureOwner | null;
}
function activity(): PictureCreationActivity {
  let busy = false, blocked = false, owner: PictureOwner | null = null;
  for (const entry of sessions.values()) {
    const state = entry.session.closeState();
    busy ||= state.busy;
    blocked ||= state.blocked === true || state.unsaved;
    if (owner === null && (state.busy || state.blocked || state.unsaved)) owner = { ...entry.owner };
  }
  return { busy, blocked, owner };
}
function changed() {
  for (const observer of observers) observer(activity());
}
export function subscribePictureCreationActivity(observer: (activity: PictureCreationActivity) => void) {
  observers.add(observer);
  observer(activity());
  return () => { observers.delete(observer); };
}
export function pictureCreationSession(owner: PictureOwner): PictureCreationSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plotNumber]);
  const existing = sessions.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  const { contextId, project, plotNumber } = retainedOwner;
  const session = new PictureCreationSession(retainedOwner, {
    choose: () => PictureCreationService.Choose(contextId, project, plotNumber),
    recoverSelection: () => PictureCreationService.RecoverSelection(contextId, project, plotNumber),
    create: (token, request) => PictureCreationService.Create(token, request),
    receipt: request => PictureCreationService.LookupReceipt(request),
    image: (token, request) => PictureCreationService.GetSelectedImage(token, request),
    release: token => PictureCreationService.ReleaseSelection(token),
    acknowledge: token => PictureCreationService.AcknowledgeReceipt(token),
  });
  sessions.set(key, { owner: retainedOwner, session });
  // Idle construction can run in a derived expression and does not change activity.
  let initialized = false;
  session.subscribe(() => {
    if (initialized) changed();
  });
  initialized = true;
  return session;
}
