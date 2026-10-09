import type { LongVegetationPreferencesReview } from '../bindings/github.com/boostao/vpro-wails';
import type { GoogleEarthScope } from './googleEarthReview';
import {
  ReportTitlePreferencesSession, reportTitlePreferencesRegistry, validateReportTitlePreferences,
  type ReportTitlePreferenceRequest, type ReportTitlePreferenceReceipt, type ReportTitlePreferencesView,
} from './reportTitlePreferences';

export type LongVegetationPreferenceRequest = ReportTitlePreferenceRequest;
export type LongVegetationPreferenceReceipt = ReportTitlePreferenceReceipt;
export type LongVegetationPreferencesView = ReportTitlePreferencesView;
export function validateLongVegetationPreferences(value: LongVegetationPreferencesReview | null, scope: GoogleEarthScope): LongVegetationPreferencesReview {
  return validateReportTitlePreferences(value, scope, 'Long Vegetation');
}
export class LongVegetationPreferencesSession extends ReportTitlePreferencesSession {
  constructor(scope: GoogleEarthScope) { super(scope, 'Long Vegetation'); }
}
export const longVegetationPreferencesSession = reportTitlePreferencesRegistry(scope => new LongVegetationPreferencesSession(scope));
