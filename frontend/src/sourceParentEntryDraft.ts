import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
import { metadataCellText, equalCell } from './projectMetadataEditor';
import { wellFormedUTF16 } from './qualityEditor';
import { ordinaryTextError, sourceParentBooleanNullError } from './ordinaryEditor';
import { siviDateTimestampError } from './siviDateTimestamp';

export type SourceParentEntryInput = { kind: 'original' | 'clear' } | { kind: 'text'; raw: string }
  | { kind: 'boolean'; value: boolean } | { kind: 'option'; option: number | null };
export interface SourceParentEntryParsed { value: ProjectMetadataCell; error: string | null }
export interface SourceParentEntryPolicy { column: string; kind: string; maximum: number }
export const sourceParentEntryNull = (): ProjectMetadataCell =>
  ({ storage: 'null', text: null, integer: null, real: null, blobHex: null });

export function parseSourceParentEntryValue<Policy extends SourceParentEntryPolicy>(policy: Policy,
  expected: ProjectMetadataCell, input: SourceParentEntryInput, parser: {
    label: string; kinds: readonly string[];
    allowsEmpty(policy: Policy): boolean;
    number(policy: Policy, raw: string): SourceParentEntryParsed;
  }): SourceParentEntryParsed {
  let value = structuredClone(expected), error: string | null = null;
  if (!parser.kinds.includes(policy.kind) || !Number.isInteger(policy.maximum) || policy.maximum < 0) {
    throw new Error(`${parser.label} ${policy.column} has an unavailable parser policy.`);
  }
  if (input.kind === 'original') return { value, error };
  if (input.kind === 'clear') value = sourceParentEntryNull();
  else if (policy.kind === 'boolean' && input.kind === 'boolean') {
    if (typeof input.value !== 'boolean') error = `${policy.column} requires an explicit Boolean value.`;
    else value = { ...sourceParentEntryNull(), storage: 'integer', integer: input.value ? '-1' : '0' };
  } else if (policy.kind === 'option' && input.kind === 'option') {
    if (input.option === null) value = sourceParentEntryNull();
    else if (input.option === 1 || input.option === 2) {
      value = { ...sourceParentEntryNull(), storage: 'text', text: String(input.option) };
    } else error = `${policy.column} requires source option1/2 or explicit NULL.`;
  } else if (input.kind === 'text') {
    if (typeof input.raw !== 'string' || !wellFormedUTF16(input.raw) || input.raw.includes('\0')) {
      error = `${policy.column} contains malformed Unicode or NUL; literal input was not repaired.`;
    } else if (expected.storage !== 'null' && input.raw === metadataCellText(expected)) {
      return { value, error };
    } else if (['text', 'categorical', 'plot-type', 'date'].includes(policy.kind)) {
      value = { ...sourceParentEntryNull(), storage: 'text', text: input.raw };
      error = policy.kind === 'date' ? siviDateTimestampError(input.raw)
        : parser.allowsEmpty(policy) && input.raw === '' ? null
        : ordinaryTextError(policy.maximum === 0 ? { label: policy.column, kind: 'memo' }
          : { label: policy.column, kind: 'text', maximum: policy.maximum }, input.raw, null);
    } else if (['integer', 'single', 'double', 'latitude', 'longitude'].includes(policy.kind)) {
      ({ value, error } = parser.number(policy, input.raw));
    } else error = `${policy.column} requires an explicit Boolean or source option selection.`;
  } else error = `${policy.column} input intent does not match its source parser policy.`;
  if (!error && expected.storage === 'blob' && !equalCell(expected, value)) {
    error = `${policy.column} historical BLOB replacement has no lossless source audit representation; retain original storage.`;
  }
  if (!error) error = sourceParentBooleanNullError(policy.column, expected.storage === 'null', value.storage === 'null');
  return { value, error };
}
