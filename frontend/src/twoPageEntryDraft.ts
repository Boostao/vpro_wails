import type { ProjectMetadataCell, TwoPageEntryFieldPolicy } from '../bindings/github.com/boostao/vpro-wails';
import { ordinaryNumberValue } from './ordinaryEditor';
import { coordinateDecimalValue } from './coordinateEditor';
import { parseSourceParentEntryValue, sourceParentEntryNull,
  type SourceParentEntryInput, type SourceParentEntryParsed } from './sourceParentEntryDraft';

export type TwoPageEntryInput = SourceParentEntryInput;
export type TwoPageEntryParsed = SourceParentEntryParsed;
export const twoPageEntryKinds = ['text', 'categorical', 'plot-type', 'date', 'integer', 'single',
  'latitude', 'longitude', 'boolean', 'option'] as const;
export const twoPageEntryNull = sourceParentEntryNull;

export function parseTwoPageEntryValue(policy: TwoPageEntryFieldPolicy, expected: ProjectMetadataCell,
  input: TwoPageEntryInput): TwoPageEntryParsed {
  return parseSourceParentEntryValue(policy, expected, input, {
    label: 'Complete-entry', kinds: twoPageEntryKinds,
    allowsEmpty: field => field.kind === 'categorical',
    number: (field, raw) => {
      if (field.kind !== 'integer' && field.kind !== 'single' && field.kind !== 'latitude' && field.kind !== 'longitude') {
        throw new Error(`Complete-entry ${field.column} has an unavailable number policy.`);
      }
      const parsed = field.kind === 'latitude' || field.kind === 'longitude'
        ? coordinateDecimalValue(field.kind, raw, null)
        : ordinaryNumberValue({ label: field.column, kind: field.kind }, raw, null);
      return { error: parsed.error, value: parsed.value === null ? twoPageEntryNull() : field.kind === 'integer'
        ? { ...twoPageEntryNull(), storage: 'integer', integer: String(parsed.value) }
        : { ...twoPageEntryNull(), storage: 'real', real: parsed.value } };
    },
  });
}
