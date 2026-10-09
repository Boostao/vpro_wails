import { metadataCellText } from './projectMetadataEditor';
import { exactSigned64 } from './projectMetadataRestore';
import { wellFormedUTF16 } from './qualityEditor';
import { finiteSingleValue } from './numericEditor';
import { validSIVIRequestId } from './siviRequestId';
import {
  equalVegetationCell, vegetationNullCell, vegetationSourceCell, twoPageVegetationSource,
  twoPageVegetationOriginalFromWire, type TwoPageVegetationEdit, type TwoPageVegetationOriginal,
  type TwoPageVegetationOwner, type TwoPageVegetationRequest,
} from './twoPageVegetationWire';

export interface TwoPageVegetationDraft extends TwoPageVegetationEdit {
  raw: string;
  nullValue: boolean;
  error: string | null;
}
export type TwoPageVegetationDrafts = Record<string, TwoPageVegetationDraft>;

export function twoPageVegetationFields(original: TwoPageVegetationOriginal): string[] {
  const { covers } = twoPageVegetationSource(original.form);
  return [...(original.form === 'SubVegAXL_BC' ? covers.slice(0, 7) : covers), 'Collected'];
}

export function twoPageVegetationEditableID(original: TwoPageVegetationOriginal, rowId: string): number {
  if (!original.sourceRowIds.includes(rowId)) throw new Error('Vegetation row is outside this original source query.');
  const identity = vegetationSourceCell(original, rowId, 'ID');
  if (identity.storage !== 'integer' || identity.integer === null || !exactSigned64(identity.integer) ||
    BigInt(identity.integer) < -2147483648n || BigInt(identity.integer) > 2147483647n) {
    throw new Error('NULL/noninteger vegetation IDs remain read-only; a signed32 original ID is required.');
  }
  return Number(identity.integer);
}

export function stageTwoPageVegetation(original: TwoPageVegetationOriginal, rowId: string, column: string,
  raw: string, nullValue: boolean): TwoPageVegetationDraft {
  twoPageVegetationEditableID(original, rowId);
  if (!twoPageVegetationFields(original).includes(column)) {
    throw new Error('Vegetation field is outside its source bindings; Species callbacks remain unavailable.');
  }
  const expected = vegetationSourceCell(original, rowId, column);
  let value = vegetationNullCell(), error: string | null = null;
  if (!nullValue) {
    if (!wellFormedUTF16(raw)) error = `${column} contains incomplete Unicode; raw input was not repaired.`;
    else if (expected.storage !== 'null' && raw === metadataCellText(expected)) value = { ...expected };
    else if (column === 'Collected') {
      value = { ...value, storage: 'text', text: raw };
      if (raw.length > 1) error = 'Collected exceeds1 UTF-16 unit.';
    } else {
      const parsed = finiteSingleValue(column, raw, expected.storage === 'real' ? expected.real : undefined);
      if (raw !== raw.trim() || raw === '' || parsed.error || parsed.value === null) {
        error = parsed.error ?? `${column} requires a finite number without added whitespace, or explicit NULL.`;
      } else if (equalVegetationCell(expected, { ...value, storage: 'real', real: parsed.value })) {
        value = { ...expected };
      } else if (Object.is(Math.fround(parsed.value), -0)) {
        error = `${column} new negative-zero values require lossless source/storage calibration.`;
      } else if (Math.fround(parsed.value) >= 100) {
        error = `${column} requires a value below100 or NULL (<100 Or Is Null).`;
      } else value = { ...value, storage: 'real', real: Math.fround(parsed.value) };
    }
  }
  return { contextId: original.contextId, form: original.form, rowId, column,
    expected: { ...expected }, value, raw, nullValue, error };
}

export function twoPageVegetationRequest(original: TwoPageVegetationOriginal, owner: TwoPageVegetationOwner,
  drafts: TwoPageVegetationDrafts, requestId: string): TwoPageVegetationRequest {
  const reviewed = twoPageVegetationOriginalFromWire(original, owner);
  if (!validSIVIRequestId(requestId)) throw new Error('Vegetation Save requires a retained lowercase UUIDv4.');
  const edits: TwoPageVegetationEdit[] = [], seen = new Set<string>();
  for (const draft of Object.values(drafts)) {
    const staged = stageTwoPageVegetation(reviewed, draft.rowId, draft.column, draft.raw, draft.nullValue);
    const key = JSON.stringify([draft.rowId, draft.column]);
    if (draft.contextId !== owner.contextId || draft.form !== owner.form || seen.has(key) ||
      !equalVegetationCell(draft.expected, staged.expected) || draft.error || staged.error ||
      !equalVegetationCell(draft.value, staged.value)) {
      throw new Error(draft.error ?? staged.error ?? 'Vegetation draft belongs to another original or has inconsistent raw/value state.');
    }
    seen.add(key);
    if (equalVegetationCell(staged.expected, staged.value)) continue;
    edits.push({ contextId: owner.contextId, form: owner.form, rowId: staged.rowId, column: staged.column,
      expected: { ...staged.expected }, value: { ...staged.value } });
  }
  if (!edits.length) throw new Error('Vegetation Save requires at least one explicit changed data cell.');
  return { requestId, original: reviewed, edits };
}
