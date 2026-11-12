import { createSummaryVegetationQualityPreferencesRegistry } from './summaryVegetationQualityPreferences';

export const summaryVegetationQualityPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_QUALITY_PREFERENCES !== 'false' && import.meta.env.VITE_SUMMARY_VEGETATION !== 'false'
    ? createSummaryVegetationQualityPreferencesRegistry() : null;
