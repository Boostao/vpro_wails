import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationQualityNullPreferenceValues {
  siteIncludeNulls: boolean;
  vegIncludeNulls: boolean;
  soilIncludeNulls: boolean;
}
export interface LongVegetationQualityNullPreferencesReview extends GoogleEarthScope {
  values: LongVegetationQualityNullPreferenceValues;
}
export const longVegetationQualityNullFields = [
  { key: 'siteIncludeNulls', control: 'optSiteQualityNull', label: 'Site' },
  { key: 'vegIncludeNulls', control: 'optVegQualityNull', label: 'Veg' },
  { key: 'soilIncludeNulls', control: 'optSoilQualityNull', label: 'Soil' },
] as const;
const valid = (values: LongVegetationQualityNullPreferenceValues | null): values is LongVegetationQualityNullPreferenceValues =>
  !!values && longVegetationQualityNullFields.every(field => typeof values[field.key] === 'boolean');
const copy = (values: LongVegetationQualityNullPreferenceValues): LongVegetationQualityNullPreferenceValues => ({
  siteIncludeNulls: values.siteIncludeNulls, vegIncludeNulls: values.vegIncludeNulls, soilIncludeNulls: values.soilIncludeNulls,
});
export function validateLongVegetationQualityNullPreferences(value: LongVegetationQualityNullPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationQualityNullPreferencesReview {
  if (!value || !ownedScope(value, scope) || !valid(value.values)) {
    throw new Error('Saved Long Vegetation quality Include Nulls flags are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: copy(value.values) };
}
export class LongVegetationQualityNullPreferencesSession extends ReportPreferencesSession<
  LongVegetationQualityNullPreferenceValues, LongVegetationQualityNullPreferenceValues | null, LongVegetationQualityNullPreferencesReview> {
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: null, savedLabel: 'saved quality Include Nulls flags',
      review: (value, owner) => validateLongVegetationQualityNullPreferences(value, owner).values,
      draft: copy,
      validate: draft => valid(draft) ? '' : 'Correct all three boolean Include Nulls flags or Undo the preference draft.',
      proposed: draft => {
        if (!valid(draft)) throw new Error('Quality Include Nulls requires three complete boolean values.');
        return copy(draft);
      },
      equal: (a, b) => longVegetationQualityNullFields.every(field => a[field.key] === b[field.key]),
    });
  }
  editNullFlag(key: keyof LongVegetationQualityNullPreferenceValues, value: boolean) {
    if (!longVegetationQualityNullFields.some(field => field.key === key)) throw new Error('Unsupported quality Include Nulls field.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved quality Include Nulls flags before editing.');
    this.editDraft({ ...draft, [key]: value });
  }
}
export const longVegetationQualityNullPreferencesSession = reportPreferencesRegistry(
  scope => new LongVegetationQualityNullPreferencesSession(scope));
