import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationThresholdPreferenceValues { presenceGreaterThan: number; meanCoverGreaterThan: number }
export interface LongVegetationThresholdDraft { presenceGreaterThan: string; meanCoverGreaterThan: string }
export interface LongVegetationThresholdPreferencesReview extends GoogleEarthScope {
  values: LongVegetationThresholdPreferenceValues;
}
export const longVegetationThresholdFields = [
  { key: 'presenceGreaterThan', control: 'rptLvShowSppPres', label: 'Show species with presence > than' },
  { key: 'meanCoverGreaterThan', control: 'rptLvShowSppMC', label: 'Show species with mean cover > than' },
] as const;
const finite = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value);
export function longVegetationThresholdValue(raw: string, label: string): number {
  if (typeof raw !== 'string' || !/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(raw)) {
    throw new Error(`${label} requires a finite number without added whitespace; NULL is unavailable.`);
  }
  const value = Number(raw);
  if (!finite(value) || value === 0 && /[1-9]/.test(raw.split(/[eE]/)[0])) {
    throw new Error(`${label} is outside the finite DOUBLE threshold domain; its value has not been repaired.`);
  }
  return value;
}
export function validateLongVegetationThresholdPreferences(value: LongVegetationThresholdPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationThresholdPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values ||
      !finite(value.values.presenceGreaterThan) || !finite(value.values.meanCoverGreaterThan)) {
    throw new Error('Saved Long Vegetation thresholds are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: { presenceGreaterThan: value.values.presenceGreaterThan, meanCoverGreaterThan: value.values.meanCoverGreaterThan } };
}
function proposed(draft: LongVegetationThresholdDraft): LongVegetationThresholdPreferenceValues {
  return {
    presenceGreaterThan: longVegetationThresholdValue(draft.presenceGreaterThan, longVegetationThresholdFields[0].label),
    meanCoverGreaterThan: longVegetationThresholdValue(draft.meanCoverGreaterThan, longVegetationThresholdFields[1].label),
  };
}
export class LongVegetationThresholdPreferencesSession extends ReportPreferencesSession<
  LongVegetationThresholdPreferenceValues, LongVegetationThresholdDraft, LongVegetationThresholdPreferencesReview> {
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: { presenceGreaterThan: '', meanCoverGreaterThan: '' }, savedLabel: 'saved report thresholds',
      review: (value, owner) => validateLongVegetationThresholdPreferences(value, owner).values,
      draft: saved => ({ presenceGreaterThan: String(saved.presenceGreaterThan), meanCoverGreaterThan: String(saved.meanCoverGreaterThan) }),
      validate: draft => {
        try { proposed(draft); return ''; }
        catch (cause) { return String(cause); }
      },
      proposed,
      equal: (a, b) => a.presenceGreaterThan === b.presenceGreaterThan && a.meanCoverGreaterThan === b.meanCoverGreaterThan,
    });
  }
  editThreshold(key: keyof LongVegetationThresholdDraft, raw: string) {
    if (!longVegetationThresholdFields.some(field => field.key === key)) throw new Error('Unsupported report threshold.');
    if (!this.view().initialized) throw new Error('Load saved report thresholds before editing.');
    this.editDraft({ ...this.view().draft, [key]: raw });
  }
}
export const longVegetationThresholdPreferencesSession = reportPreferencesRegistry(
  scope => new LongVegetationThresholdPreferencesSession(scope));
