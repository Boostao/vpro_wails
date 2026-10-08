import { wellFormedUTF16 } from './qualityEditor';
import { plotLookupInputError } from './scopedPlotLookup';

export interface PlotLabelPreview {
  plotNumber1: string;
  zone1: string;
  representing1: string | null;
  projectId1: string | null;
  displayPlotNumber: string;
  displayRepresenting: string | null;
}

const trimLabelDisplay = (value: string): string => value.replace(/^ +| +(?![\s\S])/g, '');

function labelText(value: unknown, field: string, limit: number): string | null {
  if (value === null) return null;
  if (typeof value !== 'string' || !wellFormedUTF16(value) || value.includes('\0') || value.length > limit) {
    throw new Error(`${field} cannot be assigned to the source label without repairing text or exceeding its UTF-16 bound.`);
  }
  return value;
}

export function plotLabelFromHeader(value: unknown, requestedPlot: string): PlotLabelPreview {
  if (plotLookupInputError(requestedPlot) || !value || typeof value !== 'object' || Array.isArray(value) ||
      !('plotNumber' in value) || value.plotNumber !== requestedPlot) {
    throw new Error('A complete saved header for this exact literal plot was not returned.');
  }
  if (!('zone' in value) || !('subZone' in value) || !('siteSeries' in value) ||
      !('plotRepresenting' in value) || !('projectId' in value)) {
    throw new Error('The saved header is missing source label fields; no blank defaults were inferred.');
  }
  const plot = labelText(value.plotNumber, 'PlotNumber1', 7);
  const zone = labelText(value.zone, 'Zone', 4);
  if (plot === null || zone === null) {
    throw new Error('Source label preparation requires a non-NULL plot and Zone for Len/Space; no NULL padding was inferred.');
  }
  const subZone = labelText(value.subZone, 'SubZone', 50);
  const siteSeries = labelText(value.siteSeries, 'SiteSeries', 50);
  const combinedZone = `${zone}${' '.repeat(4 - zone.length)}${subZone ?? ''}/${siteSeries ?? ''}`;
  labelText(combinedZone, 'Zone1', 50);
  const representing = labelText(value.plotRepresenting, 'Representing1', 255);
  const projectId = labelText(value.projectId, 'ProjectID1', 50);
  return {
    plotNumber1: plot, zone1: combinedZone, representing1: representing, projectId1: projectId,
    displayPlotNumber: trimLabelDisplay(plot),
    displayRepresenting: representing === null ? null : trimLabelDisplay(representing),
  };
}
