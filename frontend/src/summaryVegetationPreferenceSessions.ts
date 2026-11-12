import { createSummaryVegetationPreferencesRegistry } from './summaryVegetationPreferences';

export const summaryVegetationPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_PREFERENCES !== 'false' && import.meta.env.VITE_SUMMARY_VEGETATION !== 'false'
    ? createSummaryVegetationPreferencesRegistry(true) : null;
