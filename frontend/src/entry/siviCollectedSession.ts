import { SIVIChildWriteSession, type SIVIChildWritePort, type SIVIChildWriteView } from './siviChildWriteSession';
import {
  stageSIVICollected, siviCollectedDirty, siviCollectedErrors, siviCollectedEdits, siviCollectedRequestJSON,
  type SIVICollectedDrafts,
} from './siviCollectedEditor';
import { siviChildSourceNotices } from './siviChildSourceNotices';

export type SIVICollectedView = SIVIChildWriteView<SIVICollectedDrafts>;
export type SIVICollectedPort = SIVIChildWritePort;

export class SIVICollectedSession extends SIVIChildWriteSession<'Collected', SIVICollectedDrafts> {
  constructor(plot: string, port: SIVICollectedPort, notify: () => void, extended = false, sourceNotices = false) {
    super(plot, port, notify, {
      label: 'SIVI Collected', pluralLabel: 'SIVI Collected cycles', empty: () => ({}),
      stage: (review, drafts, rowId, column, command, nullValue) => {
        if (column !== 'Collected' || command !== 'cycle' || nullValue) {
          throw new Error('SIVI Collected accepts only its source cycle action, not assigned text.');
        }
        return stageSIVICollected(review, drafts, rowId);
      },
      dirty: siviCollectedDirty, errors: siviCollectedErrors, hidden: () => false,
      count: (review, drafts) => siviCollectedEdits(review, drafts).length,
      requestJSON: siviCollectedRequestJSON, conservativeFailures: true,
      sourceNotices: sourceNotices ? (review, drafts) => siviChildSourceNotices(review,
        siviCollectedEdits(review, drafts).map(edit => ({
          rowId: edit.rowId, form: edit.form, column: 'Collected', expected: edit.expected, value: drafts[edit.rowId].value,
        }))) : undefined,
    }, extended);
  }

  cycle(rowId: string) { this.stage(rowId, 'Collected', 'cycle', false); }
}
