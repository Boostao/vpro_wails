import type { QualityView } from './qualityEditor';
import { createReferenceCodeEditor } from './referenceCodeEditor';

export const regionCodeKeys = ['fsRegionDistrict', 'ecosection'] as const;
export type RegionCodeKey = typeof regionCodeKeys[number];
export type RegionCodes = Record<RegionCodeKey, string | null>;
export type RegionCodeList = 'Region' | 'Ecosection';
export type RegionCodeViews = Record<RegionCodeList, QualityView>;
export const regionCodeLabels: Record<RegionCodeKey, string> = {
  fsRegionDistrict: 'Region/district', ecosection: 'Ecosection'
};
export function regionCodeKey(column: string | undefined): RegionCodeKey | undefined {
  return column === 'FSRegionDistrict' ? 'fsRegionDistrict' : column === 'Ecosection' ? 'ecosection' : undefined;
}
export function regionCodeList(key: RegionCodeKey): RegionCodeList {
  return key === 'fsRegionDistrict' ? 'Region' : 'Ecosection';
}
const limits: Record<RegionCodeList, number> = { Region: 7, Ecosection: 3 };
export function regionCodeLimit(key: RegionCodeKey): number { return limits[regionCodeList(key)]; }
const reference = createReferenceCodeEditor({ keys: regionCodeKeys, labels: regionCodeLabels, list: regionCodeList, limits });
export const regionCodeValueError = reference.valueError;
export const regionCodesChanged = reference.changed;
export const regionCodeLengthError = reference.lengthError;
export const regionCodeGroups = reference.groups;
export const regionCodeDefinitions = reference.definitions;
export const regionCodeSuggestions = reference.suggestions;
export const rememberRegionCodeAcknowledgement = reference.rememberAcknowledgement;
export const regionCodeAcknowledged = reference.acknowledged;
export function regionCodeWarnings(codes: RegionCodes, original: RegionCodes | null, views: RegionCodeViews): string[] {
  return regionCodeKeys.flatMap(key => {
    if (codes[key] === null || codes[key] === (original?.[key] ?? null)) return [];
    const list = regionCodeList(key), view = views[list];
    if (view.busy) return [];
    if (!view.ready) return [view.error ?? `${regionCodeLabels[key]} choices are unavailable; the changed raw code has not been checked.`];
    return regionCodeDefinitions(view.choices, list, codes[key]).length ? []
      : [`${regionCodeLabels[key]} "${codes[key]}" is not a listed Item. It will be kept exactly as entered.`];
  });
}
export function regionCodeFieldError(key: RegionCodeKey, value: string | null, original: string | null,
  views: RegionCodeViews): string | null {
  const invalid = regionCodeValueError(key, value, original);
  if (invalid || value === null || value === original) return invalid;
  return views[regionCodeList(key)].busy ? `Wait for ${regionCodeLabels[key]} choices to finish refreshing.` : null;
}
export function regionCodeValidation(codes: RegionCodes, original: RegionCodes | null, views: RegionCodeViews,
  acknowledged: boolean): string | null {
  if (!regionCodesChanged(codes, original)) return null;
  for (const key of regionCodeKeys) {
    const error = regionCodeFieldError(key, codes[key], original?.[key] ?? null, views);
    if (error) return error;
  }
  return regionCodeWarnings(codes, original, views).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched or unchecked region/ecosection codes before saving.' : null;
}
