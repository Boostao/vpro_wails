import { SIVIChildWriteSession, type SIVIChildWritePort, type SIVIChildWriteView } from './siviChildWriteSession';
import {
  stageSIVIIdentity, siviIdentityDirty, siviIdentityErrors, siviIdentityEdits, siviIdentityRequestJSON,
  type SIVIIdentityDrafts,
} from './siviIdentityEditor';
import { siviChildSourceNotices } from './siviChildSourceNotices';

export type SIVIIdentityView = SIVIChildWriteView<SIVIIdentityDrafts>;
export type SIVIIdentityPort = SIVIChildWritePort;

export class SIVIIdentitySession extends SIVIChildWriteSession<'ID', SIVIIdentityDrafts> {
  constructor(plot: string, port: SIVIIdentityPort, notify: () => void, extended = false, sourceNotices = false) {
    super(plot, port, notify, {
      label: 'SIVI identity', pluralLabel: 'SIVI ID drafts', empty: () => ({}),
      stage: (review, drafts, rowId, column, raw, nullValue) => {
        if (column !== 'ID') throw new Error('SIVI identity accepts only ID assignments, not parent or other fields.');
        return stageSIVIIdentity(review, drafts, rowId, raw, nullValue);
      },
      dirty: siviIdentityDirty, errors: siviIdentityErrors, hidden: () => false,
      count: (review, drafts) => siviIdentityEdits(review, drafts).length,
      requestJSON: siviIdentityRequestJSON, conservativeFailures: true, auditFreeHistory: true,
      sourceNotices: sourceNotices ? (review, drafts) => siviChildSourceNotices(review, siviIdentityEdits(review, drafts)) : undefined,
    }, extended);
  }
}
