import { createSummaryVegetationPresentationPreferencesRegistry } from './summaryVegetationPresentationPreferences';

export const summaryVegetationPresentationPreferenceSession =
  import.meta.env.VITE_SUMMARY_VEGETATION_PRESENTATION_PREFERENCES === 'true' && import.meta.env.VITE_SUMMARY_VEGETATION === 'true'
    ? createSummaryVegetationPresentationPreferencesRegistry() : null;
