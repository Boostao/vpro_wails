import { PictureService } from '../../bindings/github.com/boostao/vpro-wails';
import { PictureMetadataSession } from './pictureMetadataSession';
import type { PictureOwner } from './pictureRead';

const sessions = new Map<string, PictureMetadataSession>();

export function pictureMetadataSession(owner: PictureOwner): PictureMetadataSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.plotNumber]);
  let session = sessions.get(key);
  if (!session) {
    const { contextId, project, plotNumber } = owner;
    session = new PictureMetadataSession({ contextId, project, plotNumber }, {
      read: () => PictureService.GetMetadata(contextId, plotNumber),
      save: request => PictureService.SaveMetadata(contextId, plotNumber, request),
      receipt: request => PictureService.LookupMetadataReceipt(contextId, plotNumber, request),
    });
    sessions.set(key, session);
  }
  return session;
}
