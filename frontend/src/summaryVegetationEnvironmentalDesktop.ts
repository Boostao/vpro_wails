import { VegetationReportService } from '../bindings/github.com/boostao/vpro-wails';
import { summaryEnvironmentalRegistry, summaryEnvironmentalNumericRegistry, summaryEnvironmentalLifeformRegistry, type SummaryEnvironmentalPort } from './summaryVegetationEnvironmental';

export const summaryEnvironmentalEnabled = import.meta.env.VITE_SUMMARY_VEGETATION !== 'false' &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT !== 'false';
export const summaryEnvironmentalNumericEnabled = summaryEnvironmentalEnabled &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC !== 'false';
export const summaryEnvironmentalLifeformEnabled = summaryEnvironmentalNumericEnabled &&
  import.meta.env.VITE_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM !== 'false';
export const summaryEnvironmentalDesktop = summaryEnvironmentalLifeformEnabled ?
  summaryEnvironmentalLifeformRegistry(true) : summaryEnvironmentalNumericEnabled ?
  summaryEnvironmentalNumericRegistry(true) : summaryEnvironmentalRegistry(summaryEnvironmentalEnabled);
export const summaryEnvironmentalPort: SummaryEnvironmentalPort = {
  preview: contextId => summaryEnvironmentalLifeformEnabled ?
    VegetationReportService.PreviewSummaryVegetationEnvironmentalLifeform(contextId, 1, 6, 0) : summaryEnvironmentalNumericEnabled ?
    VegetationReportService.PreviewSummaryVegetationEnvironmentalNumeric(contextId, 1, 6, 0) :
    VegetationReportService.PreviewSummaryVegetationEnvironmental(contextId, 1, 6, 0),
  validationYield: () => new Promise<void>(resolve => setTimeout(resolve, 0)),
};
