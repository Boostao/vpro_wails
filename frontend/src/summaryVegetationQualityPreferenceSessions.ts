import { createSummaryVegetationQualityPreferencesRegistry } from './summaryVegetationQualityPreferences';

export const summaryVegetationQualityPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_QUALITY_PREFERENCES === 'true' && import.meta.env.VITE_SUMMARY_VEGETATION === 'true'
    ? createSummaryVegetationQualityPreferencesRegistry() : null;
