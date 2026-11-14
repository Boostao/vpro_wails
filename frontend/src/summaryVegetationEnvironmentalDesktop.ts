import { VegetationReportService } from '../bindings/github.com/boostao/vpro-wails';
import { summaryEnvironmentalLifeformRegistry, type SummaryEnvironmentalPort } from './summaryVegetationEnvironmental';

export const summaryEnvironmentalEnabled = true;
export const summaryEnvironmentalNumericEnabled = true;
export const summaryEnvironmentalLifeformEnabled = true;
export const summaryEnvironmentalDesktop = summaryEnvironmentalLifeformRegistry(true);
export const summaryEnvironmentalPort: SummaryEnvironmentalPort = {
  preview: contextId => VegetationReportService.PreviewSummaryVegetationEnvironmentalLifeform(contextId, 1, 6, 0),
  validationYield: () => new Promise<void>(resolve => setTimeout(resolve, 0)),
};
