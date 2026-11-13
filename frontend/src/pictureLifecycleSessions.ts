import { PictureService } from '../bindings/github.com/boostao/vpro-wails';
import { PictureLifecycleSession } from './pictureLifecycleSession';
import { notifyPictureLifecycleActivity, pictureLifecycleEntries, subscribePictureLifecycleActivity, type PictureLifecycleActivity } from './pictureLifecycleActivity';
import type { PictureOwner } from './pictureRead';

export { subscribePictureLifecycleActivity, type PictureLifecycleActivity };
export function pictureLifecycleSession(owner: PictureOwner): PictureLifecycleSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plotNumber]);
  const existing = pictureLifecycleEntries.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  const { contextId, plotNumber } = retainedOwner;
  const session = new PictureLifecycleSession(retainedOwner, {
    read: () => PictureService.GetMetadata(contextId, plotNumber),
    history: () => PictureService.GetDeletionHistory(contextId, plotNumber),
    review: historyId => PictureService.ReviewDeletionRestoration(contextId, plotNumber, historyId),
    delete: request => PictureService.DeletePicture(contextId, request),
    deletionReceipt: request => PictureService.LookupDeletionReceipt(contextId, request),
    restore: request => PictureService.RestoreDeletedPicture(contextId, request),
    restorationReceipt: request => PictureService.LookupDeletionRestorationReceipt(contextId, request),
  });
  pictureLifecycleEntries.set(key, { owner: retainedOwner, session });
  let initialized = false;
  session.subscribe(() => { if (initialized) notifyPictureLifecycleActivity(); });
  initialized = true;
  return session;
}
