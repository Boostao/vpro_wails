import { SIVIChildWriteSession, type SIVIChildWritePort, type SIVIChildWriteView } from './siviChildWriteSession';
import {
  stageSIVICombined, siviCombinedDirty, siviCombinedErrors, siviCombinedHiddenDrafts, siviCombinedEdits, siviCombinedRequestJSON,
  type SIVICombinedColumn, type SIVICombinedDrafts,
} from './siviCombinedEditor';
import { siviChildSourceNotices } from './siviChildSourceNotices';

export type SIVICombinedView = SIVIChildWriteView<SIVICombinedDrafts>;
export type SIVICombinedPort = SIVIChildWritePort;

export class SIVICombinedSession extends SIVIChildWriteSession<SIVICombinedColumn, SIVICombinedDrafts> {
  constructor(plot: string, port: SIVICombinedPort, notify: () => void, extended = false, sourceNotices = false) {
    super(plot, port, notify, {
      label: 'SIVI combined', pluralLabel: 'SIVI covers/heights', empty: () => ({ covers: {}, heights: {} }), stage: stageSIVICombined,
      dirty: siviCombinedDirty, errors: siviCombinedErrors, hidden: siviCombinedHiddenDrafts,
      count: (review, drafts) => siviCombinedEdits(review, drafts).length, requestJSON: siviCombinedRequestJSON,
      conservativeFailures: true,
      sourceNotices: sourceNotices ? (review, drafts) => siviChildSourceNotices(review, siviCombinedEdits(review, drafts)) : undefined,
    }, extended);
  }
}
