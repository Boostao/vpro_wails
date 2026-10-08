import type { SIVIVegetationProjection } from '../bindings/github.com/boostao/vpro-wails';
import { SIVIChildWriteSession, type SIVIChildWritePort, type SIVIChildWriteView } from './siviChildWriteSession';
import { siviProjectionFromWire } from './siviHeightSession';
import {
  validateSIVISpeciesReferences, stageSIVISpecies, contextSIVISpecies, chooseSIVISpecies,
  siviSpeciesErrors, siviSpeciesDirty, siviSpeciesEdits, siviSpeciesRequestJSON,
  type SIVISpeciesDrafts, type SIVISpeciesReferences, type SIVISpeciesOwner,
} from './siviSpeciesEditor';
import type { SpeciesDecisionKind } from './vegetationSpeciesEditor';
import { siviChildSourceNotices } from './siviChildSourceNotices';

export interface SIVISpeciesPort extends Omit<SIVIChildWritePort, 'read'> {
  read(extended: boolean): PromiseLike<{ original: SIVIVegetationProjection[] | null; references: unknown } | null>;
}
export interface SIVISpeciesView extends SIVIChildWriteView<SIVISpeciesDrafts> { references: SIVISpeciesReferences | null }
type Command = 'Species' | 'context' | SpeciesDecisionKind;
interface ReferenceHolder { live: boolean; references: SIVISpeciesReferences | null }

export class SIVISpeciesSession extends SIVIChildWriteSession<Command, SIVISpeciesDrafts> {
  private readonly holder: ReferenceHolder;
  constructor(owner: SIVISpeciesOwner, port: SIVISpeciesPort, notify: () => void, extended = false, sourceNotices = false) {
    const identity = structuredClone(owner);
    const holder: ReferenceHolder = { live: true, references: null };
    const references = () => {
      if (!holder.references) throw new Error('SIVI Species definitions are unavailable; reload explicitly.');
      return holder.references;
    };
    super(identity.plot, {
      save: (extended, request) => port.save(extended, request),
      restore: (historyId, action) => port.restore(historyId, action),
      refreshParent: () => port.refreshParent(),
      read: async extended => {
        holder.references = null;
        const bundle = await port.read(extended);
        if (!holder.live) throw new Error('SIVI Species editor ownership ended during its source/reference read.');
        if (!bundle) throw new Error('SIVI Species original/reference bundle was not returned.');
        siviProjectionFromWire(bundle.original, identity.plot, extended);
        const checked = validateSIVISpeciesReferences(bundle.references, identity);
        holder.references = checked;
        return bundle.original;
      },
    }, notify, {
      label: 'SIVI Species', pluralLabel: 'SIVI Species drafts', empty: () => ({}),
      stage: (review, drafts, rowId, command, raw, nullValue) => {
        if (nullValue) throw new Error('SIVI Species does not assign NULL or coerce an unavailable field.');
        if (command === 'Species') return stageSIVISpecies(review, holder.references, drafts, rowId, raw);
        if (command === 'context') {
          if (!/^[012]$/.test(raw)) throw new Error('Select an explicit eligible SIVI Species source group.');
          return contextSIVISpecies(review, holder.references, drafts, rowId, Number(raw));
        }
        if (!['keep', 'replace', 'user'].includes(command) || command === 'keep' && raw !== '') {
          throw new Error('SIVI Species source decision is unavailable.');
        }
        return chooseSIVISpecies(review, references(), drafts, rowId, command, command === 'keep' ? undefined : raw);
      },
      errors: siviSpeciesErrors, dirty: siviSpeciesDirty, hidden: () => false,
      count: (review, drafts) => siviSpeciesEdits(review, references(), drafts).length,
      requestJSON: (review, drafts) => siviSpeciesRequestJSON(review, references(), identity, drafts),
      conservativeFailures: true,
      sourceNotices: sourceNotices ? (review, drafts) => siviChildSourceNotices(review,
        siviSpeciesEdits(review, references(), drafts).map(edit => ({
          rowId: edit.rowId, form: edit.form, column: 'Species', expected: edit.expected,
          value: { storage: 'text', text: edit.value, integer: null, real: null, blobHex: null },
        }))) : undefined,
    }, extended);
    this.holder = holder;
  }

  view(): SIVISpeciesView {
    return { ...super.view(), references: this.holder.live ? structuredClone(this.holder.references) : null };
  }
  context(rowId: string, group: number) { this.stage(rowId, 'context', String(group), false); }
  choose(rowId: string, kind: SpeciesDecisionKind, selected?: string) {
    if (kind !== 'keep' && selected === undefined) throw new Error('Select an explicit SIVI Species replacement/personal code.');
    this.stage(rowId, kind, selected ?? '', false);
  }
  dispose() {
    this.holder.live = false;
    this.holder.references = null;
    super.dispose();
  }
}
