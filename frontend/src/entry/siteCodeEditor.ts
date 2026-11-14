import type { QualityView } from './qualityEditor';
import { createReferenceCodeEditor } from './referenceCodeEditor';

export const siteCodeKeys = ['siteDisturbance1', 'siteDisturbance2', 'siteDisturbance3', 'exposure1', 'exposure2'] as const;
export type SiteCodeKey = typeof siteCodeKeys[number];
export type SiteCodes = Record<SiteCodeKey, string | null>;
export type SiteCodeList = 'SiteDisturbance' | 'Exposure';
export type SiteCodeViews = Record<SiteCodeList, QualityView>;
export const siteCodeLabels: Record<SiteCodeKey, string> = {
  siteDisturbance1: 'Site disturbance 1', siteDisturbance2: 'Site disturbance 2', siteDisturbance3: 'Site disturbance 3',
  exposure1: 'Exposure 1', exposure2: 'Exposure 2'
};
export function siteCodeKey(column: string | undefined): SiteCodeKey | undefined {
  return column === 'SiteDisturbance1' ? 'siteDisturbance1' : column === 'SiteDisturbance2' ? 'siteDisturbance2'
    : column === 'SiteDisturbance3' ? 'siteDisturbance3' : column === 'Exposure1' ? 'exposure1'
    : column === 'Exposure2' ? 'exposure2' : undefined;
}
export function siteCodeList(key: SiteCodeKey): SiteCodeList {
  return key === 'exposure1' || key === 'exposure2' ? 'Exposure' : 'SiteDisturbance';
}
const limits: Record<SiteCodeList, number> = { SiteDisturbance: 8, Exposure: 2 };
export function siteCodeLimit(key: SiteCodeKey): number { return limits[siteCodeList(key)]; }
const reference = createReferenceCodeEditor({ keys: siteCodeKeys, labels: siteCodeLabels, list: siteCodeList, limits });
export const siteCodeValueError = reference.valueError;
export const siteCodeLengthError = reference.lengthError;
export const siteCodesChanged = reference.changed;
export const siteCodeGroups = reference.groups;
export const siteCodeDefinitions = reference.definitions;
export const siteCodeSuggestions = reference.suggestions;
export const rememberSiteCodeAcknowledgement = reference.rememberAcknowledgement;
export const siteCodeAcknowledged = reference.acknowledged;
export function siteCodeWarnings(codes: SiteCodes, original: SiteCodes | null, views: SiteCodeViews): string[] {
  const view = views.SiteDisturbance;
  if (view.busy) return [];
  const changed = siteCodeKeys.filter(key => siteCodeList(key) === 'SiteDisturbance' &&
    codes[key] !== null && codes[key] !== (original?.[key] ?? null));
  if (!changed.length) return [];
  if (!view.ready) return [view.error ?? 'Disturbance choices are unavailable; the changed raw codes have not been checked.'];
  return changed.filter(key => !siteCodeDefinitions(view.choices, 'SiteDisturbance', codes[key]).length)
    .map(key => `${siteCodeLabels[key]} "${codes[key]}" is not a listed Item. It will be kept exactly as entered.`);
}
export function siteCodeFieldError(key: SiteCodeKey, value: string | null, original: string | null, views: SiteCodeViews): string | null {
  const invalid = siteCodeValueError(key, value, original);
  if (invalid) return invalid;
  if (value === null || value === original) return null;
  const list = siteCodeList(key), view = views[list];
  if (view.busy) return `Wait for ${list === 'Exposure' ? 'exposure' : 'disturbance'} choices to finish refreshing.`;
  if (list !== 'Exposure') return null;
  if (!view.ready) return view.error ?? 'Exposure choices are unavailable; new exposure codes cannot be checked or saved.';
  return view.choices.some(row => reference.selectable(row, list) && row.code === value) ? null
    : `${siteCodeLabels[key]} must use an exact Item from the Exposure list. Select the full canonical code; case and prefixes are not silently rewritten.`;
}
export function siteCodeValidation(codes: SiteCodes, original: SiteCodes | null, views: SiteCodeViews, acknowledged: boolean): string | null {
  if (!siteCodesChanged(codes, original)) return null;
  const invalid = siteCodeLengthError(codes, original);
  if (invalid) return invalid;
  for (const key of siteCodeKeys) {
    const error = siteCodeFieldError(key, codes[key], original?.[key] ?? null, views);
    if (error) return error;
  }
  return siteCodeWarnings(codes, original, views).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched disturbance codes before saving.' : null;
}
