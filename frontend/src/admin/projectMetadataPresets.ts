import { Call } from '@wailsio/runtime';
import { wellFormedUTF16 } from '../entry/qualityEditor';

export interface ProjectMetadataPreset {
  id: string;
  label: string;
  description: string;
  ecosysCollectionStandard: string;
  vegCoverMethod: string;
  plotMethod: string;
  datum: string;
  coverDescriptions: Record<string, string>;
}

export interface DerivedProjectMetadata {
  project: string;
  projectId: string;
  totalPlotsInScope: number;
  startDate: number | null;
  endDate: number | null;
  numberOfFs882Plots: number;
  numberOfSiteVisits: number;
  unclassifiedPlotTypes: number;
  geographicStudyRegion: string;
  regionCodesMatched: string[];
  collectedSite: number;
  collectedVeg: number;
  collectedSoil: number;
  collectedTerrain: number;
  allSpecs: string;
  tableOfLists: string;
  vlistsSha256Short: string;
}

export interface ApplyProjectMetadataAutomationRequest {
  projectId: string;
  applyDerived: boolean;
  presetId?: string;
}

export interface ApplyProjectMetadataAutomationResult {
  project: string;
  projectId: string;
  metadataId: number;
  createdRow: boolean;
  updatedColumns: string[];
  auditRowCount: number;
  derived: DerivedProjectMetadata;
}

export interface ProjectMetadataAutomationPort {
  listPresets(): Promise<ProjectMetadataPreset[]>;
  deriveFromPlots(contextId: string, projectId: string): Promise<DerivedProjectMetadata>;
  applyAutomation(contextId: string, req: ApplyProjectMetadataAutomationRequest): Promise<ApplyProjectMetadataAutomationResult>;
}

export const defaultProjectMetadataAutomationPort: ProjectMetadataAutomationPort = {
  listPresets: () => Call.ByName('main.ContextService.ListProjectMetadataPresets'),
  deriveFromPlots: (contextId, projectId) => Call.ByName('main.ContextService.DeriveProjectMetadataFromPlots', contextId, projectId),
  applyAutomation: (contextId, req) => Call.ByName('main.ContextService.ApplyProjectMetadataAutomation', contextId, req),
};

export const COVER_DESCRIPTION_ORDER: readonly string[] = [
  'CoverA1Description',
  'CoverA2Description',
  'CoverA3Description',
  'CoverADescription',
  'CoverB1Description',
  'CoverB2Description',
  'CoverB2aDescription',
  'CoverB2bDescription',
  'CoverB2cDescription',
  'CoverBDescription',
  'CoverCDescription',
  'CoverDDescription',
  'Cover8Description',
  'Cover9Description',
  'Cover10Description',
];

export function formatCollectionCode(code: number): string {
  switch (code) {
    case 1:
      return '1 - Complete (100% of plots)';
    case 2:
      return '2 - Partial (subset of plots)';
    case 3:
      return '3 - None (0% of plots)';
    default:
      return String(code);
  }
}

export function formatDatumCode(datum: string): string {
  if (datum === '2') return '2 (NAD83)';
  if (datum === '1') return '1 (NAD27)';
  return datum;
}

export function validateMetadataAutomationRequest(req: ApplyProjectMetadataAutomationRequest): string | null {
  if (!wellFormedUTF16(req.projectId) || req.projectId.includes('\0')) {
    return 'ProjectID must be well-formed Unicode without NUL bytes.';
  }
  if (req.projectId.length > 20) {
    return 'Metadata ProjectID cannot exceed 20 UTF-16 code units.';
  }
  if (!req.applyDerived && (!req.presetId || req.presetId.trim() === '')) {
    return 'Select Auto-Derived Plot Aggregations, a Standards Preset, or both before applying.';
  }
  return null;
}
