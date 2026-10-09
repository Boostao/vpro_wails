import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationSwitchPreferenceValues {
  quickReport: boolean;
  spaceBetweenGroups: boolean;
  reportSummary: boolean;
  constantSpeciesList: boolean;
}
export interface LongVegetationSwitchPreferencesReview extends GoogleEarthScope {
  values: LongVegetationSwitchPreferenceValues;
}
export const longVegetationSwitchFields = [
  { key: 'quickReport', control: 'optScreenReport', label: 'Screen report (skip some print page formatting - faster)' },
  { key: 'spaceBetweenGroups', control: 'rptLvSpaceBetweenGroups', label: 'Space between groups' },
  { key: 'reportSummary', control: 'optCreateSummary', label: 'Create report summary' },
  { key: 'constantSpeciesList', control: 'optConstantSppList', label: 'Use constant species list' },
] as const;
const valid = (values: LongVegetationSwitchPreferenceValues | null): values is LongVegetationSwitchPreferenceValues =>
  !!values && longVegetationSwitchFields.every(field => typeof values[field.key] === 'boolean');
const copy = (values: LongVegetationSwitchPreferenceValues): LongVegetationSwitchPreferenceValues => ({
  quickReport: values.quickReport, spaceBetweenGroups: values.spaceBetweenGroups,
  reportSummary: values.reportSummary, constantSpeciesList: values.constantSpeciesList,
});
export function validateLongVegetationSwitchPreferences(value: LongVegetationSwitchPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationSwitchPreferencesReview {
  if (!value || !ownedScope(value, scope) || !valid(value.values)) {
    throw new Error('Saved Long Vegetation switches are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: copy(value.values) };
}
export class LongVegetationSwitchPreferencesSession extends ReportPreferencesSession<
  LongVegetationSwitchPreferenceValues, LongVegetationSwitchPreferenceValues | null, LongVegetationSwitchPreferencesReview> {
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: null, savedLabel: 'saved report switches',
      review: (value, owner) => validateLongVegetationSwitchPreferences(value, owner).values,
      draft: copy,
      validate: draft => valid(draft) ? '' : 'Correct all four boolean report switches or Undo the preference draft.',
      proposed: draft => {
        if (!valid(draft)) throw new Error('Report switches require four complete boolean values.');
        return copy(draft);
      },
      equal: (a, b) => longVegetationSwitchFields.every(field => a[field.key] === b[field.key]),
    });
  }
  editSwitch(key: keyof LongVegetationSwitchPreferenceValues, value: boolean) {
    if (!longVegetationSwitchFields.some(field => field.key === key)) throw new Error('Unsupported report switch field.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved report switches before editing.');
    this.editDraft({ ...draft, [key]: value });
  }
}
export const longVegetationSwitchPreferencesSession = reportPreferencesRegistry(
  scope => new LongVegetationSwitchPreferencesSession(scope));
