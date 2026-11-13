import { PictureService } from '../bindings/github.com/boostao/vpro-wails';
import { PictureCreationSession } from './pictureCreationSession';
import { notifyPictureCreationActivity, pictureCreationEntries, subscribePictureCreationActivity, type PictureCreationActivity } from './pictureCreationActivity';
import type { PictureOwner } from './pictureRead';

export { subscribePictureCreationActivity, type PictureCreationActivity };
export function pictureCreationSession(owner: PictureOwner): PictureCreationSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plotNumber]);
  const existing = pictureCreationEntries.get(key);
  if (existing) return existing.session;
  const retainedOwner = { contextId: owner.contextId, project: owner.project, plotNumber: owner.plotNumber };
  const { contextId, project, plotNumber } = retainedOwner;
  const session = new PictureCreationSession(retainedOwner, {
    choose: () => PictureService.ChooseCreationImage(contextId, project, plotNumber),
    recoverSelection: () => PictureService.RecoverCreationSelection(contextId, project, plotNumber),
    create: (token, request) => PictureService.CreatePicture(token, request),
    receipt: request => PictureService.LookupCreationReceipt(request),
    image: (token, request) => PictureService.GetSelectedCreationImage(token, request),
    release: token => PictureService.ReleaseCreationSelection(token),
    acknowledge: token => PictureService.AcknowledgeCreationReceipt(token),
  });
  pictureCreationEntries.set(key, { owner: retainedOwner, session });
  // Idle construction can run in a derived expression and does not change activity.
  let initialized = false;
  session.subscribe(() => {
    if (initialized) notifyPictureCreationActivity();
  });
  initialized = true;
  return session;
}
