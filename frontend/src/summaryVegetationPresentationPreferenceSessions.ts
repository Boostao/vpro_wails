import { createSummaryVegetationPresentationPreferencesRegistry } from './summaryVegetationPresentationPreferences';

export const summaryVegetationPresentationPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES !== 'false' && import.meta.env.VITE_SUMMARY_VEGETATION !== 'false'
    ? createSummaryVegetationPresentationPreferencesRegistry() : null;
