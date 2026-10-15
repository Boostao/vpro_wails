import type { SIVIParentOriginal } from './siviParentEditor';
import { completeMetadataCellFromWire, sourceParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import { SIVIParentScopedWriteSession, type SIVIParentWritePort } from './siviParentWriteSession';
import { conservativePort } from './siviParentSharedSession';
import { sourceParentReferenceFromWire, siviReferencePolicies, type SourceParentReferenceField } from './referenceEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { equalSIVICHARSParentCell, siviCHARSParentFields, siviCHARSParentPoliciesFromWire,
  siviCHARSParentEntryRequest, stageSIVICHARSParentEntry, validateSIVICHARSParentOriginal,
  type SIVICHARSParentEntryPolicy, type SIVICHARSParentEntryDrafts } from './siviCHARSParentEntry';
import type { SourceParentEntryInput } from './sourceParentEntryDraft';

export type SIVICHARSParentEntryWrite = ReturnType<typeof siviCHARSParentEntryRequest>;
export interface SIVICHARSParentEntrySnapshot {
  original: SIVIParentOriginal; policies: SIVICHARSParentEntryPolicy[]; fields: SourceParentReferenceField[];
}
const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
const literal = (value: string) => wellFormedUTF16(value) && !value.includes('\0');
const dirty = (drafts: SIVICHARSParentEntryDrafts) =>
  Object.values(drafts).some(row => row.error !== null || !equalSIVICHARSParentCell(row.expected, row.value));

function ahorizonDefinitionsFromWire(wire: unknown): SourceParentReferenceField['definitions'] {
  if (!record(wire) || !Array.isArray(wire.columns) || wire.columns.length !== 1 ||
      !record(wire.columns[0]) || wire.columns[0].name !== 'Item' || wire.columns[0].declaredType !== 'TEXT' ||
      !Array.isArray(wire.rows) || wire.rows.length !== 3) {
    throw new Error('CHARS Ahorizon literal value-list definitions are incomplete.');
  }
  const rows = wire.rows;
  return { columns: [{ name: 'Item', declaredType: 'TEXT' }], rows: ['Ah', 'Ae', ''].map((code, ordinal) => {
    const row: unknown = rows[ordinal];
    if (!record(row) || row.rowId !== `source-value-list:${ordinal}` || !Array.isArray(row.cells) ||
        row.cells.length !== 1 || !completeMetadataCellFromWire(row.cells[0]) ||
        row.cells[0].storage !== 'text' || row.cells[0].text !== code) {
      throw new Error('CHARS Ahorizon literal ordinals or original storage changed.');
    }
    return { rowId: row.rowId, cells: [structuredClone(row.cells[0])] };
  }) };
}

export function siviCHARSParentEntryFromWire(wire: unknown, owner: SIVIParentOwner): SIVICHARSParentEntrySnapshot {
  if (!record(wire)) throw new Error('CHARS entry review was not returned.');
  const original = sourceParentOriginalFromWire(wire.Original, owner, validateSIVICHARSParentOriginal);
  const policies = siviCHARSParentPoliciesFromWire(wire.Policies, original);
  const controls = siviCHARSParentFields.filter(field => field.type === 'ComboBox' && field.binding !== 'ProjectID');
  if (!Array.isArray(wire.Fields) || wire.Fields.length !== 31 || controls.length !== 31) {
    throw new Error('CHARS entry requires its complete31-reference source cohort.');
  }
  const received = wire.Fields;
  const fields = controls.map(control => {
    const column = control.binding!;
    const shared = siviReferencePolicies.find(row => row.column === column);
    const list = column === 'RootZoneParticleSize' ? 'RootZoneParticleSize' :
      column === 'SnowCoverregime' ? 'SnowCoverRegime' : column === 'SV_AhorizonType' ? 'SV_AhorizonType' : shared?.list;
    const limit = 'LimitToList' in control.properties ? control.properties.LimitToList : undefined;
    const matches = received.filter(field => record(field) && field.column === column);
    if (!list || matches.length !== 1 || limit && limit.value !== 'NotDefault') {
      throw new Error(`CHARS ${column} reference source identity changed.`);
    }
    const field = sourceParentReferenceFromWire(matches[0], { column, list, required: Boolean(limit) }, 'CHARS',
      column === 'SV_AhorizonType' ? ahorizonDefinitionsFromWire : undefined);
    if (!field.source || !literal(field.source) || !literal(field.diagnostic) ||
        field.choices.some(choice => !literal(choice.rowId) || !literal(choice.diagnostic) ||
          choice.code !== null && !literal(choice.code) || choice.description !== null && !literal(choice.description)) ||
        (column === 'Zone' ? field.choices.some(choice => choice.rowId !== '') ||
          field.definitions.columns.length !== 0 || field.definitions.rows.length !== 0
          : new Set(field.choices.map(choice => choice.rowId)).size !== field.choices.length) ||
        column === 'SubZone' && (field.available || field.choices.length !== 0)) {
      throw new Error(`CHARS ${column} reference literals or source availability changed.`);
    }
    return field;
  });
  return { original, policies, fields };
}

function issue(snapshot: SIVICHARSParentEntrySnapshot | null, draft: SIVICHARSParentEntryDrafts[string]) {
  if (draft.error) return { error: draft.error, advisory: null };
  if (equalSIVICHARSParentCell(draft.expected, draft.value) || draft.value.storage === 'null') {
    return { error: null, advisory: null };
  }
  const field = snapshot?.fields.find(row => row.column === draft.column);
  if (!field) return { error: null, advisory: null };
  const member = field.available && field.choices.some(choice =>
    choice.selectable && choice.code !== null && choice.code !== '' && choice.code === draft.value.text);
  if (member) return { error: null, advisory: null };
  const reason = field.diagnostic || `${draft.column} is not an exact selectable listed Item.`;
  return field.required ? { error: reason, advisory: null }
    : { error: null, advisory: `${reason} Free entry is retained literally; no source callback is implied.` };
}

export class SIVICHARSParentEntrySession extends SIVIParentScopedWriteSession<string, SIVICHARSParentEntryWrite,
  SIVICHARSParentEntryDrafts, SourceParentEntryInput> {
  private readonly review: { value: SIVICHARSParentEntrySnapshot | null };
  constructor(owner: SIVIParentOwner, port: SIVIParentWritePort<SIVICHARSParentEntryWrite>, notify: () => void) {
    const review: { value: SIVICHARSParentEntrySnapshot | null } = { value: null };
    const errors = (drafts: SIVICHARSParentEntryDrafts) => Object.values(drafts).flatMap(draft => {
      const error = issue(review.value, draft).error; return error ? [error] : [];
    });
    const policies = () => {
      if (!review.value) throw new Error('Load owned CHARS source policies before editing.');
      return review.value.policies;
    };
    super(owner, conservativePort(port, 'CHARS parent entry'), notify, {
      label: 'CHARS parent entry', accepts: (column): column is string => policies().some(field => field.column === column),
      unavailable: 'CHARS source field is readonly or unavailable.',
      drafts: {
        empty: () => ({}), dirty, errors,
        stage: (original, drafts, column, input) => stageSIVICHARSParentEntry(original, policies(), drafts, column, input),
      },
      decodeOriginal: (wire, owned) => {
        const snapshot = siviCHARSParentEntryFromWire(wire, owned);
        review.value = snapshot;
        return snapshot.original;
      },
      request: (original, drafts) => {
        const error = errors(drafts)[0]; if (error) throw new Error(error);
        return siviCHARSParentEntryRequest(original, policies(), drafts);
      },
      count: request => request.edits.length,
    });
    this.review = review;
  }
  override view() {
    const view = super.view();
    const snapshot = view.original ? structuredClone(this.review.value) : null;
    const advisories: Record<string, string> = {};
    for (const draft of Object.values(view.drafts)) {
      const result = issue(snapshot, draft);
      draft.error ||= result.error;
      if (result.advisory) advisories[draft.column] = result.advisory;
    }
    return { ...view, snapshot, advisories };
  }
}
