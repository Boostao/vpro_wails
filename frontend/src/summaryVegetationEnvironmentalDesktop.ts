import { SummaryVegetationEnvironmentalService, SummaryVegetationEnvironmentalLifeformService } from '../bindings/github.com/boostao/vpro-wails';
import { summaryEnvironmentalRegistry, summaryEnvironmentalNumericRegistry, summaryEnvironmentalLifeformRegistry, type SummaryEnvironmentalPort } from './summaryVegetationEnvironmental';

export const summaryEnvironmentalEnabled = import.meta.env.VITE_SUMMARY_VEGETATION === 'true' &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT === 'true';
export const summaryEnvironmentalNumericEnabled = summaryEnvironmentalEnabled &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC === 'true';
export const summaryEnvironmentalLifeformEnabled = summaryEnvironmentalNumericEnabled &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM === 'true';
export const summaryEnvironmentalDesktop = summaryEnvironmentalLifeformEnabled ?
  summaryEnvironmentalLifeformRegistry(true) : summaryEnvironmentalNumericEnabled ?
  summaryEnvironmentalNumericRegistry(true) : summaryEnvironmentalRegistry(summaryEnvironmentalEnabled);
export const summaryEnvironmentalPort: SummaryEnvironmentalPort = {
  preview: contextId => summaryEnvironmentalLifeformEnabled ?
    SummaryVegetationEnvironmentalLifeformService.PreviewEnvironmentalLifeform(contextId, 1, 6, 0) : summaryEnvironmentalNumericEnabled ?
    SummaryVegetationEnvironmentalService.PreviewEnvironmentalNumeric(contextId, 1, 6, 0) :
    SummaryVegetationEnvironmentalService.PreviewEnvironmental(contextId, 1, 6, 0),
  validationYield: () => new Promise<void>(resolve => setTimeout(resolve, 0)),
};
