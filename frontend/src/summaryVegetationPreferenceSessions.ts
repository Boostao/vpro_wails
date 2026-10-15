import { createSummaryVegetationPreferencesRegistry } from './summaryVegetationPreferences';

export const summaryVegetationPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_PREFERENCES === 'true' && import.meta.env.VITE_SUMMARY_VEGETATION === 'true'
    ? createSummaryVegetationPreferencesRegistry(true) : null;
