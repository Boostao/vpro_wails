import { createReferenceCodeEditor } from './referenceCodeEditor';
import type { QualityView } from './qualityEditor';

export const geologyCodeKeys = ['bedrockGeology1', 'bedrockGeology2', 'bedrockGeology3'] as const;
export const geologyCodeColumns = ['BedrockGeology1', 'BedrockGeology2', 'BedrockGeology3'] as const;
export type GeologyCodeKey = typeof geologyCodeKeys[number];
export type GeologyCodes = Record<GeologyCodeKey, string | null>;
export const geologyCodeLabels: Record<GeologyCodeKey, string> = {
  bedrockGeology1: 'Bedrock type 1', bedrockGeology2: 'Bedrock type 2', bedrockGeology3: 'Bedrock type 3'
};
export function geologyCodeKey(column: string | undefined): GeologyCodeKey | undefined {
  return geologyCodeKeys.find((_, index) => geologyCodeColumns[index] === column);
}
const reference = createReferenceCodeEditor({
  keys: geologyCodeKeys, labels: geologyCodeLabels, list: () => 'BedrockType' as const,
  limits: { BedrockType: 4 }
});
export const geologyCodeValueError = reference.valueError;
export const geologyCodeGroups = reference.groups;
export const geologyCodeSelectable = reference.selectable;
export const geologyCodeAcknowledged = reference.acknowledged;
export const rememberGeologyCodeAcknowledgement = reference.rememberAcknowledgement;
export function geologyCodeBusy(codes: GeologyCodes, original: GeologyCodes | null, view: QualityView): boolean {
  return view.busy && geologyCodeKeys.some(key => codes[key] !== null && codes[key] !== (original?.[key] ?? null));
}
export function geologyCodeWarnings(codes: GeologyCodes, original: GeologyCodes | null, view: QualityView): string[] {
  return geologyCodeKeys.flatMap(key => {
    if (codes[key] === null || codes[key] === (original?.[key] ?? null) || view.busy) return [];
    if (!view.ready) return [view.error ?? `${geologyCodeLabels[key]} choices are unavailable; the raw code has not been checked.`];
    return reference.definitions(view.choices, 'BedrockType', codes[key]).length ? []
      : [`${geologyCodeLabels[key]} "${codes[key]}" is not a listed Item. It will be kept exactly as entered.`];
  });
}
export function geologyCodeValidation(codes: GeologyCodes, original: GeologyCodes | null, view: QualityView, accepted: boolean): string | null {
  const invalid = reference.lengthError(codes, original);
  if (invalid) return invalid;
  if (geologyCodeBusy(codes, original, view)) return 'Wait for changed bedrock choices to finish refreshing.';
  return geologyCodeWarnings(codes, original, view).length && !accepted
    ? 'Review and explicitly acknowledge the unmatched or unchecked bedrock codes before saving.' : null;
}
