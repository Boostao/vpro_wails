import type { LongEnvironmentPreferencesReview } from '../bindings/github.com/boostao/vpro-wails';
import type { GoogleEarthScope } from './googleEarthReview';
import {
  ReportTitlePreferencesSession, reportTitlePreferencesRegistry, validateReportTitlePreferences,
  type ReportTitlePreferenceRequest, type ReportTitlePreferenceReceipt, type ReportTitlePreferencesView,
} from './reportTitlePreferences';

export type LongEnvironmentPreferenceRequest = ReportTitlePreferenceRequest;
export type LongEnvironmentPreferenceReceipt = ReportTitlePreferenceReceipt;
export type LongEnvironmentPreferencesView = ReportTitlePreferencesView;
export function validateLongEnvironmentPreferences(value: LongEnvironmentPreferencesReview | null, scope: GoogleEarthScope): LongEnvironmentPreferencesReview {
  return validateReportTitlePreferences(value, scope, 'Long Environment');
}
export class LongEnvironmentPreferencesSession extends ReportTitlePreferencesSession {
  constructor(scope: GoogleEarthScope) { super(scope, 'Long Environment'); }
}
export const longEnvironmentPreferencesSession = reportTitlePreferencesRegistry(scope => new LongEnvironmentPreferencesSession(scope));
