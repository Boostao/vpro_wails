import type { ProjectMetadataCell, TwoPageEntryFieldPolicy } from '../bindings/github.com/boostao/vpro-wails';
import { metadataCellText, equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { ordinaryNumberValue, ordinaryTextError } from './ordinaryEditor';
import { coordinateDecimalValue } from './coordinateEditor';
import { siviDateTimestampError } from './siviDateTimestamp';

export type TwoPageEntryInput = { kind: 'original' | 'clear' } | { kind: 'text'; raw: string }
  | { kind: 'boolean'; value: boolean } | { kind: 'option'; option: number | null };
export interface TwoPageEntryParsed { value: ProjectMetadataCell; error: string | null }
export const twoPageEntryKinds = ['text', 'categorical', 'plot-type', 'date', 'integer', 'single',
  'latitude', 'longitude', 'boolean', 'option'] as const;
export const twoPageEntryNull = (): ProjectMetadataCell =>
  ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

export function parseTwoPageEntryValue(policy: TwoPageEntryFieldPolicy, expected: ProjectMetadataCell,
  input: TwoPageEntryInput): TwoPageEntryParsed {
  let value = structuredClone(expected), error: string | null = null;
  if (!twoPageEntryKinds.some(kind => kind === policy.kind) || !Number.isInteger(policy.maximum) || policy.maximum < 0) {
    throw new Error(`Complete-entry ${policy.column} has an unavailable parser policy.`);
  }
  if (input.kind === 'original') return { value, error };
  if (input.kind === 'clear') value = twoPageEntryNull();
  else if (policy.kind === 'boolean' && input.kind === 'boolean') {
    if (typeof input.value !== 'boolean') error = `${policy.column} requires an explicit Boolean value.`;
    else value = { ...twoPageEntryNull(), storage: 'integer', integer: input.value ? '-1' : '0' };
  } else if (policy.kind === 'option' && input.kind === 'option') {
    if (input.option === null) value = twoPageEntryNull();
    else if (input.option === 1 || input.option === 2) {
      value = { ...twoPageEntryNull(), storage: 'text', text: String(input.option) };
    } else error = `${policy.column} requires source option1/2 or explicit NULL.`;
  } else if (input.kind === 'text') {
    if (typeof input.raw !== 'string' || !wellFormedUTF16(input.raw) || input.raw.includes('\0')) {
      error = `${policy.column} contains malformed Unicode or NUL; literal input was not repaired.`;
    } else if (expected.storage !== 'null' && input.raw === metadataCellText(expected)) {
      return { value, error };
    } else if (policy.kind === 'text' || policy.kind === 'categorical' || policy.kind === 'plot-type' || policy.kind === 'date') {
      value = { ...twoPageEntryNull(), storage: 'text', text: input.raw };
      error = policy.kind === 'date' ? siviDateTimestampError(input.raw)
        : policy.kind === 'categorical' && input.raw === '' ? null
        : ordinaryTextError(policy.maximum === 0 ? { label: policy.column, kind: 'memo' }
          : { label: policy.column, kind: 'text', maximum: policy.maximum }, input.raw, null);
    } else if (policy.kind === 'integer' || policy.kind === 'single' ||
      policy.kind === 'latitude' || policy.kind === 'longitude') {
      const parsed = policy.kind === 'latitude' || policy.kind === 'longitude'
        ? coordinateDecimalValue(policy.kind, input.raw, null)
        : ordinaryNumberValue({ label: policy.column, kind: policy.kind }, input.raw, null);
      error = parsed.error;
      value = parsed.value === null ? twoPageEntryNull() : policy.kind === 'integer'
        ? { ...twoPageEntryNull(), storage: 'integer', integer: String(parsed.value) }
        : { ...twoPageEntryNull(), storage: 'real', real: parsed.value };
    } else error = `${policy.column} requires an explicit Boolean or source option selection.`;
  } else error = `${policy.column} input intent does not match its source parser policy.`;
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${policy.column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  return { value, error };
}
