import { Call } from '@wailsio/runtime';
import { wellFormedUTF16 } from '../entry/qualityEditor';

export const PROJECT_NAME_PATTERN = /^[A-Za-z][A-Za-z0-9_]{0,30}$/;

export type MergeResolutionChoice = 'keep_target' | 'accept_source';

export interface NewProjectRequest {
  projectName: string;
  projectId?: string;
  projectTitle?: string;
  presetId?: string;
  destinationPath?: string;
  activateOnSuccess: boolean;
}

export interface NewProjectResult {
  projectName: string;
  projectId: string;
  destinationPath: string;
  destinationSha256: string;
  activated: boolean;
  state?: unknown;
}

export interface SaveProjectAsRequest {
  newProjectName: string;
  destinationPath?: string;
  activateOnSuccess: boolean;
}

export interface SaveProjectAsResult {
  sourceProject: string;
  newProjectName: string;
  destinationPath: string;
  destinationSha256: string;
  copiedTables: string[];
  companionTables: string[];
  activated: boolean;
  state?: unknown;
}

export interface ExportSplinterRequest {
  splinterProject: string;
  destinationPath?: string;
  useActiveSu: boolean;
  siteUnits?: string[];
  plotNumbers?: string[];
  activateOnSuccess: boolean;
}

export interface ExportSplinterResult {
  sourceProject: string;
  splinterProject: string;
  destinationPath: string;
  destinationSha256: string;
  sourcePlotCount: number;
  exportedPlotCount: number;
  createdSuTable: boolean;
  activated: boolean;
  state?: unknown;
}

export interface CompareProjectsRequest {
  sourcePath?: string;
  sourceProject?: string;
  targetPath: string;
  targetProject?: string;
}

export interface FieldDiff {
  table: string;
  plotNumber: string;
  key: string;
  field: string;
  sourceVal: string | null;
  targetVal: string | null;
}

export interface PlotDiff {
  plotNumber: string;
  status: 'Added' | 'Removed' | 'Identical' | 'Modified';
  diffCount: number;
  fieldDiffs: FieldDiff[];
}

export interface CompareProjectsResult {
  sourcePath: string;
  sourceProject: string;
  targetPath: string;
  targetProject: string;
  totalPlots: number;
  addedCount: number;
  removedCount: number;
  identicalCount: number;
  modifiedCount: number;
  plots: PlotDiff[];
  fieldDiffs: FieldDiff[];
}

export interface ComplianceOptions {
  useActiveSu: boolean;
}

export interface ComplianceSummaryCounts {
  envCodeErrors: number;
  boundsErrors: number;
  envAdminMismatches: number;
  orphanChildRows: number;
  duplicateSpeciesSplits: number;
  unlistedSpecies: number;
  synonymSpecies: number;
  userSpecies: number;
  plotsWithoutVeg: number;
}

export interface EnvironmentCodeFinding {
  listName: string;
  field: string;
  plotNumber: string;
  value: string;
}

export interface BoundsFinding {
  table: string;
  plotNumber: string;
  rowId: number;
  field: string;
  value: string;
  rule: string;
}

export interface EnvAdminCompletenessFinding {
  plotNumber: string;
  issue: string;
}

export interface OrphanChildFinding {
  table: string;
  plotNumber: string;
  rowCount: number;
}

export interface DuplicateVegSpeciesFinding {
  plotNumber: string;
  species: string;
  rowCount: number;
  overlappingLayers: string[];
}

export interface TaxonomyComplianceFinding {
  rowId: number;
  id: number;
  plotNumber: string;
  species: string;
  status: string;
  suggestedCode?: string;
}

export interface ComplianceValidationReport {
  project: string;
  su: string;
  useActiveSu: boolean;
  passed: boolean;
  summary: ComplianceSummaryCounts;
  envCodeFindings: EnvironmentCodeFinding[];
  boundsFindings: BoundsFinding[];
  envAdminFindings: EnvAdminCompletenessFinding[];
  orphanFindings: OrphanChildFinding[];
  plotsWithoutVegetation: string[];
  duplicateVegFindings: DuplicateVegSpeciesFinding[];
  taxonomyFindings: TaxonomyComplianceFinding[];
}

export interface QuarantineOrphanRequest {
  tables?: string[];
}

export interface QuarantinedTableSummary {
  table: string;
  quarantineTable: string;
  movedRows: number;
  plotNumbers: string[];
}

export interface QuarantineOrphanResult {
  project: string;
  batchId: string;
  totalQuarantined: number;
  reservedDeletedIds: number[];
  auditRowId?: number;
  tables: QuarantinedTableSummary[];
}

export interface OptimizeVegOptions {
  plotNumbers?: string[];
}

export interface OptimizeVegGroup {
  plotNumber: string;
  species: string;
  sourceRowCount: number;
  survivorId: number;
  retiredIds: number[];
  hasOverlappingLayerCovers: boolean;
  cappedCoverColumns: string[];
  mergedCovers: Record<string, number>;
  mergedHeights: Record<string, number>;
  mergedHeightB?: string;
}

export interface OptimizeVegPreview {
  project: string;
  beforeRowCount: number;
  afterRowCount: number;
  rowsToConsolidate: number;
  duplicateSpeciesCount: number;
  overlappingGroupCount: number;
  cappedCoverCount: number;
  previewDigest: string;
  groups: OptimizeVegGroup[];
}

export interface ApplyOptimizeVegRequest {
  plotNumbers?: string[];
  expectedDigest?: string;
}

export interface OptimizeVegResult {
  project: string;
  beforeRowCount: number;
  afterRowCount: number;
  consolidatedGroups: number;
  deletedRows: number;
  reservedDeletedIds: number[];
  auditRowsWritten: number;
  groups: OptimizeVegGroup[];
}

export interface ThreeWayMergePreviewRequest {
  sourcePath: string;
  sourceProject?: string;
  targetPath?: string;
  targetProject?: string;
  basePath?: string;
  baseProject?: string;
}

export interface MergeConflictItem {
  conflictId: string;
  plotNumber: string;
  table: string;
  key: string;
  field: string;
  baseVal?: string | null;
  sourceVal: string | null;
  targetVal: string | null;
  kind: 'FastForward' | 'Conflict' | 'TargetModifiedOnly';
  defaultResolution: MergeResolutionChoice;
}

export interface MergePlotPreview {
  plotNumber: string;
  category: 'Added' | 'Identical' | 'FastForward' | 'Conflict' | 'TargetOnly';
  items: MergeConflictItem[];
}

export interface ThreeWayMergePreview {
  sourcePath: string;
  sourceProject: string;
  targetPath: string;
  targetProject: string;
  basePath?: string;
  baseProject?: string;
  previewDigest: string;
  addedCount: number;
  identicalCount: number;
  fastForwardCount: number;
  conflictCount: number;
  targetOnlyCount: number;
  plots: MergePlotPreview[];
  conflicts: MergeConflictItem[];
}

export interface ThreeWayMergeExecuteRequest {
  sourcePath: string;
  sourceProject?: string;
  targetPath?: string;
  targetProject?: string;
  basePath?: string;
  baseProject?: string;
  expectedPreviewDigest?: string;
  defaultConflictResolution?: MergeResolutionChoice;
  plotResolutions?: Record<string, MergeResolutionChoice>;
  fieldResolutions?: Record<string, MergeResolutionChoice>;
}

export interface ThreeWayMergeResult {
  sourceProject: string;
  targetProject: string;
  targetPath: string;
  addedPlots: number;
  updatedPlots: number;
  skippedPlots: number;
  allocatedChildIds: number[];
  reservedDeletedIds: number[];
  auditRowsWritten: number;
}

export interface ProjectAdminPort {
  createNewProject(contextId: string, req: NewProjectRequest): Promise<NewProjectResult>;
  saveProjectAs(contextId: string, req: SaveProjectAsRequest): Promise<SaveProjectAsResult>;
  exportSplinterProject(contextId: string, req: ExportSplinterRequest): Promise<ExportSplinterResult>;
  compareProjects(contextId: string, req: CompareProjectsRequest): Promise<CompareProjectsResult>;
  runComplianceValidation(contextId: string, opts: ComplianceOptions): Promise<ComplianceValidationReport>;
  quarantineOrphanRecords(contextId: string, req: QuarantineOrphanRequest): Promise<QuarantineOrphanResult>;
  previewOptimizeVeg(contextId: string, opts: OptimizeVegOptions): Promise<OptimizeVegPreview>;
  applyOptimizeVeg(contextId: string, req: ApplyOptimizeVegRequest): Promise<OptimizeVegResult>;
  previewThreeWayMerge(contextId: string, req: ThreeWayMergePreviewRequest): Promise<ThreeWayMergePreview>;
  executeThreeWayMerge(contextId: string, req: ThreeWayMergeExecuteRequest): Promise<ThreeWayMergeResult>;
}

export const defaultProjectAdminPort: ProjectAdminPort = {
  createNewProject: (contextId, req) => Call.ByName('main.ContextService.CreateNewProject', contextId, req),
  saveProjectAs: (contextId, req) => Call.ByName('main.ContextService.SaveProjectAs', contextId, req),
  exportSplinterProject: (contextId, req) => Call.ByName('main.ContextService.ExportSplinterProject', contextId, req),
  compareProjects: (contextId, req) => Call.ByName('main.ContextService.CompareProjects', contextId, req),
  runComplianceValidation: (contextId, opts) => Call.ByName('main.ValidationService.RunComplianceValidation', contextId, opts),
  quarantineOrphanRecords: (contextId, req) => Call.ByName('main.ValidationService.QuarantineOrphanRecords', contextId, req),
  previewOptimizeVeg: (contextId, opts) => Call.ByName('main.ValidationService.PreviewOptimizeVeg', contextId, opts),
  applyOptimizeVeg: (contextId, req) => Call.ByName('main.ValidationService.ApplyOptimizeVeg', contextId, req),
  previewThreeWayMerge: (contextId, req) => Call.ByName('main.ValidationService.PreviewThreeWayMerge', contextId, req),
  executeThreeWayMerge: (contextId, req) => Call.ByName('main.ValidationService.ExecuteThreeWayMerge', contextId, req),
};

export function validateProjectNameInput(raw: string): string | null {
  if (typeof raw !== 'string' || !wellFormedUTF16(raw) || raw.includes('\0')) {
    return 'Project name must be well-formed Unicode without NUL characters.';
  }
  if (raw === '' || raw.trim() !== raw) {
    return 'Project name is required and cannot contain leading or trailing spaces.';
  }
  if (!PROJECT_NAME_PATTERN.test(raw)) {
    return 'Project name must start with a letter and use only ASCII letters, digits, or underscores (max 31 characters).';
  }
  const lower = raw.toLowerCase();
  if (lower === 'none' || lower === 'sample' || lower.includes('master')) {
    return `Project name "${raw}" is reserved (None, Sample, and master names are not permitted).`;
  }
  return null;
}

export function validateNewProjectDraft(draft: NewProjectRequest): string | null {
  const nameErr = validateProjectNameInput(draft.projectName);
  if (nameErr) return nameErr;
  if (draft.projectId !== undefined && draft.projectId !== '') {
    if (!wellFormedUTF16(draft.projectId) || draft.projectId.includes('\0')) {
      return 'ProjectID contains invalid Unicode.';
    }
    if (draft.projectId.length > 20) {
      return 'Metadata ProjectID cannot exceed 20 UTF-16 code units.';
    }
  }
  if (draft.projectTitle !== undefined && draft.projectTitle !== '') {
    if (!wellFormedUTF16(draft.projectTitle) || draft.projectTitle.includes('\0')) {
      return 'ProjectTitle contains invalid Unicode.';
    }
    if (draft.projectTitle.length > 255) {
      return 'ProjectTitle cannot exceed 255 UTF-16 code units.';
    }
  }
  return null;
}

export function parseLiteralTokenList(raw: string): string[] {
  if (!raw || !raw.trim()) return [];
  const items: string[] = [];
  for (const part of raw.split(/[\n,]+/)) {
    const trimmed = part.trim();
    if (trimmed !== '' && !items.includes(trimmed)) {
      items.push(trimmed);
    }
  }
  return items;
}

export function validateSplinterDraft(draft: ExportSplinterRequest, activeSu: string): string | null {
  const nameErr = validateProjectNameInput(draft.splinterProject);
  if (nameErr) return nameErr;
  const hasPlots = (draft.plotNumbers?.length ?? 0) > 0;
  const hasUnits = (draft.siteUnits?.length ?? 0) > 0;
  if (draft.useActiveSu && (!activeSu || activeSu === 'None') && !hasPlots && !hasUnits) {
    return 'Select an active Site Unit (SU) table or provide explicit Site Units or Plot Numbers.';
  }
  if (!draft.useActiveSu && !hasPlots && !hasUnits) {
    return 'Enable active SU filtering or specify at least one Site Unit or Plot Number.';
  }
  for (const plot of draft.plotNumbers ?? []) {
    if (!wellFormedUTF16(plot) || plot.includes('\0') || plot.length > 7) {
      return `Plot number "${plot}" exceeds 7 UTF-16 code units or contains invalid characters.`;
    }
  }
  return null;
}

export function buildInitialMergeFieldResolutions(preview: ThreeWayMergePreview): Record<string, MergeResolutionChoice> {
  const map: Record<string, MergeResolutionChoice> = {};
  for (const c of preview.conflicts) {
    map[c.conflictId] = c.defaultResolution;
  }
  return map;
}
