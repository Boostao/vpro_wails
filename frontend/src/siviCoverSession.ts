import {
  SIVIChildWriteSession, type SIVIChildWritePort, type SIVIChildWriteResult, type SIVIChildWriteView,
} from './siviChildWriteSession';
import {
  stageSIVICover, siviCoverDirty, siviCoverErrors, siviCoverHiddenDrafts, siviCoverEdits, siviCoverRequestJSON,
  type SIVICoverColumn, type SIVICoverDrafts,
} from './siviCoverEditor';
import { siviChildSourceNotices } from './siviChildSourceNotices';

export type SIVICoverWriteResult = SIVIChildWriteResult;
export type SIVICoverPort = SIVIChildWritePort;
export type SIVICoverView = SIVIChildWriteView<SIVICoverDrafts>;

export class SIVICoverSession extends SIVIChildWriteSession<SIVICoverColumn, SIVICoverDrafts> {
  constructor(plot: string, port: SIVICoverPort, notify: () => void, extended = false, sourceNotices = false) {
    super(plot, port, notify, {
      label: 'SIVI cover', pluralLabel: 'SIVI covers', empty: () => ({}), stage: stageSIVICover, dirty: siviCoverDirty, errors: siviCoverErrors,
      hidden: siviCoverHiddenDrafts, count: (review, drafts) => siviCoverEdits(review, drafts).length,
      requestJSON: siviCoverRequestJSON,
      sourceNotices: sourceNotices ? (review, drafts) => siviChildSourceNotices(review, siviCoverEdits(review, drafts)) : undefined,
    }, extended);
  }
}
