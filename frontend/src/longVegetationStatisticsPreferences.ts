import type { GoogleEarthScope } from './googleEarthReview';
import { ownedScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationStatisticsPreferenceValues { avgType: number; orderBy: number }
export interface LongVegetationStatisticsPreferencesReview extends GoogleEarthScope {
  values: LongVegetationStatisticsPreferenceValues;
}
export interface LongVegetationStatisticsDraft { avgType: string; orderBy: string }
const choice = (value: unknown): value is 10 | 20 => value === 10 || value === 20;
const draftChoice = (value: unknown): value is '10' | '20' => value === '10' || value === '20';
function parseDraftChoice(value: string): 10 | 20 {
  if (!draftChoice(value)) throw new Error('Average/Order draft is not a supported source choice.');
  return value === '10' ? 10 : 20;
}
export function validateLongVegetationStatisticsPreferences(value: LongVegetationStatisticsPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationStatisticsPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !choice(value.values.avgType) || !choice(value.values.orderBy)) {
    throw new Error('Saved Long Vegetation Average/Order choices are missing, unsupported or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: { avgType: value.values.avgType, orderBy: value.values.orderBy } };
}
export class LongVegetationStatisticsPreferencesSession extends ReportPreferencesSession<
  LongVegetationStatisticsPreferenceValues, LongVegetationStatisticsDraft, LongVegetationStatisticsPreferencesReview> {
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: { avgType: '', orderBy: '' }, savedLabel: 'saved Average/Order choices',
      review: (value, owner) => validateLongVegetationStatisticsPreferences(value, owner).values,
      draft: saved => ({ avgType: String(saved.avgType), orderBy: String(saved.orderBy) }),
      validate: draft => draftChoice(draft.avgType) && draftChoice(draft.orderBy) ? ''
        : 'Choose a supported Average and Order by value, or Undo the preference draft.',
      proposed: draft => ({ avgType: parseDraftChoice(draft.avgType), orderBy: parseDraftChoice(draft.orderBy) }),
      equal: (a, b) => a.avgType === b.avgType && a.orderBy === b.orderBy,
    });
  }
  editAverage(value: string) { this.editDraft({ ...this.view().draft, avgType: value }); }
  editOrder(value: string) { this.editDraft({ ...this.view().draft, orderBy: value }); }
}
export const longVegetationStatisticsPreferencesSession = reportPreferencesRegistry(
  scope => new LongVegetationStatisticsPreferencesSession(scope));
