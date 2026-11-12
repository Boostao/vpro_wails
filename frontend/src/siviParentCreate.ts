import { Call } from '@wailsio/runtime';
import { wellFormedUTF16 } from './qualityEditor';

export const SIVI_PLOT_TYPES = ['Ground', 'Visual', 'Note', 'Other'] as const;
export type SIVIPlotType = (typeof SIVI_PLOT_TYPES)[number];

export interface CreateSIVIPlotRequest {
  requestId?: string;
  plotNumber: string;
  plotType: SIVIPlotType;
  projectId?: string;
  fieldNumber?: string;
  date?: string;
  siteSurveyor?: string;
  svPolygonNumber?: string;
  zone?: string;
  subZone?: string;
  siteSeries?: string;
  userSiteUnit?: string;
  addToActiveSu?: boolean;
}

export interface CreateSIVIPlotResult {
  requestId: string;
  contextId: string;
  project: string;
  plotNumber: string;
  plotType: SIVIPlotType;
  addedToSu: boolean;
  auditRowCount: number;
  createdAt: string;
  actor: string;
  replayed: boolean;
}

export interface UndoCreateSIVIPlotRequest {
  requestId?: string;
  creationRequestId: string;
  plotNumber: string;
}

export interface UndoCreateSIVIPlotResult {
  requestId: string;
  creationRequestId: string;
  contextId: string;
  project: string;
  plotNumber: string;
  removedFromSu: boolean;
  undoneAt: string;
  actor: string;
  replayed: boolean;
}

export interface SIVICreatedPlotEntry {
  requestId: string;
  plotNumber: string;
  plotType: SIVIPlotType;
  projectId?: string;
  svPolygonNumber?: string;
  createdAt: string;
  actor: string;
  undone: boolean;
  undoneAt?: string;
  canUndo: boolean;
}

export interface SIVIParentCreatePort {
  createPlot(contextId: string, req: CreateSIVIPlotRequest): Promise<CreateSIVIPlotResult>;
  undoCreatePlot(contextId: string, req: UndoCreateSIVIPlotRequest): Promise<UndoCreateSIVIPlotResult>;
  listCreatedPlots(contextId: string): Promise<SIVICreatedPlotEntry[]>;
}

export const defaultSIVIParentCreatePort: SIVIParentCreatePort = {
  createPlot: (contextId, req) => Call.ByName('main.ContextService.CreateSIVIPlot', contextId, req),
  undoCreatePlot: (contextId, req) => Call.ByName('main.ContextService.UndoCreateSIVIPlot', contextId, req),
  listCreatedPlots: contextId => Call.ByName('main.ContextService.ListCreatedSIVIPlots', contextId),
};

export function validateSIVIFieldUTF16(label: string, value: string | undefined, maxUnits: number, required = false): string | null {
  const text = value ?? '';
  if (!wellFormedUTF16(text) || text.includes('\0')) {
    return `${label} contains malformed Unicode or NUL bytes.`;
  }
  if (required && text.trim() === '') {
    return `${label} is required.`;
  }
  if (text.length > maxUnits) {
    return `${label} exceeds ${maxUnits} UTF-16 code units (entered ${text.length}).`;
  }
  return null;
}

export function validateCreateSIVIPlotDraft(draft: CreateSIVIPlotRequest): Record<string, string> {
  const errors: Record<string, string> = {};
  const plotErr = validateSIVIFieldUTF16('PlotNumber', draft.plotNumber, 7, true);
  if (plotErr) errors.plotNumber = plotErr;

  if (!SIVI_PLOT_TYPES.includes(draft.plotType)) {
    errors.plotType = 'PlotType must be Ground, Visual, Note, or Other.';
  }
  for (const [key, label, max] of [
    ['projectId', 'ProjectID', 30],
    ['fieldNumber', 'FieldNumber', 50],
    ['date', 'Date', 32],
    ['siteSurveyor', 'SiteSurveyor', 30],
    ['svPolygonNumber', 'SV_PolygonNumber', 25],
    ['zone', 'Zone', 4],
    ['subZone', 'SubZone', 8],
    ['siteSeries', 'SiteSeries', 5],
    ['userSiteUnit', 'UserSiteUnit', 100],
  ] as const) {
    const err = validateSIVIFieldUTF16(label, draft[key], max, false);
    if (err) errors[key] = err;
  }
  if (draft.date && draft.date.trim() !== '' && !/^\d{4}-\d{2}-\d{2}$/.test(draft.date.trim())) {
    errors.date = 'Date must use ISO format YYYY-MM-DD.';
  }
  return errors;
}
