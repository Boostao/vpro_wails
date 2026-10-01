import type { QualityView } from './qualityEditor';
import { createReferenceCodeEditor } from './referenceCodeEditor';

export const soilCodeKeys = ['soilClassGroup', 'soilClassSubGroup'] as const;
export const soilCodeColumns = ['SoilClassGroup', 'SoilClassSubGroup'] as const;
export type SoilCodeKey = typeof soilCodeKeys[number];
export type SoilCodes = Record<SoilCodeKey, string | null>;
export type SoilCodeList = 'SoilClassGroup' | 'SoilClassSubgroup';
export type SoilCodeViews = Record<SoilCodeList, QualityView>;
export const soilCodeLabels: Record<SoilCodeKey, string> = {
  soilClassGroup: 'Soil great group', soilClassSubGroup: 'Soil subgroup'
};
export function soilCodeKey(column: string | undefined): SoilCodeKey | undefined {
  return column === 'SoilClassGroup' ? 'soilClassGroup' : column === 'SoilClassSubGroup' ? 'soilClassSubGroup' : undefined;
}
export function soilCodeList(key: SoilCodeKey): SoilCodeList {
  return key === 'soilClassGroup' ? 'SoilClassGroup' : 'SoilClassSubgroup';
}
const reference = createReferenceCodeEditor({
  keys: soilCodeKeys, labels: soilCodeLabels, list: soilCodeList,
  limits: { SoilClassGroup: 4, SoilClassSubgroup: 4 }
});
export const soilCodeValueError = reference.valueError;
export const soilCodeSelectable = reference.selectable;
export const soilCodeGroups = reference.groups;
export const soilCodeDefinitions = reference.definitions;
export const soilCodeSuggestions = reference.suggestions;
export const soilCodeLengthError = reference.lengthError;
export const rememberSoilCodeAcknowledgement = reference.rememberAcknowledgement;
export const soilCodeAcknowledged = reference.acknowledged;
export function soilCodeBusy(codes: SoilCodes, original: SoilCodes | null, views: SoilCodeViews): boolean {
  return soilCodeKeys.some(key => codes[key] !== null && codes[key] !== (original?.[key] ?? null) && views[soilCodeList(key)].busy);
}
export function soilCodeWarnings(codes: SoilCodes, original: SoilCodes | null, views: SoilCodeViews): string[] {
  return soilCodeKeys.flatMap(key => {
    if (codes[key] === null || codes[key] === (original?.[key] ?? null)) return [];
    const list = soilCodeList(key), view = views[list];
    if (view.busy) return [];
    if (!view.ready) return [view.error ?? `${soilCodeLabels[key]} choices are unavailable; the changed raw code has not been checked.`];
    return soilCodeDefinitions(view.choices, list, codes[key]).length ? []
      : [`${soilCodeLabels[key]} "${codes[key]}" is not a listed Item. It will be kept exactly as entered.`];
  });
}
export function soilCodeValidation(codes: SoilCodes, original: SoilCodes | null, views: SoilCodeViews, acknowledged: boolean): string | null {
  const invalid = soilCodeLengthError(codes, original);
  if (invalid) return invalid;
  if (soilCodeBusy(codes, original, views)) return 'Wait for changed soil classification choices to finish refreshing.';
  return soilCodeWarnings(codes, original, views).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched or unchecked soil classification codes before saving.' : null;
}
