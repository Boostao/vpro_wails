import { Call } from '@wailsio/runtime';
import { wellFormedUTF16 } from './qualityEditor';
import { validateProjectNameInput, parseLiteralTokenList } from './projectAdminWorkbench';

export { validateProjectNameInput, parseLiteralTokenList };

export type CloudRoleMode = 'contributor' | 'custodian';
export type CloudMergeResolution = 'keep_target' | 'accept_source';

export const CUSTODIAN_PROTECTED_FIELDS = ['BECSiteUnit', 'BEC_Use', 'GIS_BGC'] as const;

export interface CloudConfigStatus {
  configured: boolean;
  configSource: string;
  envFileDetected: boolean;
  activeRoleMode: CloudRoleMode;
  hasContributorCreds: boolean;
  hasCustodianCreds: boolean;
  sessionOverride: boolean;
  port: number;
  database: string;
  sslMode: string;
}

export interface SessionConfigRequest {
  roleMode: CloudRoleMode;
  clearOverrides?: boolean;
  host?: string;
  port?: number;
  database?: string;
  sslMode?: string;
  user?: string;
  password?: string;
  adminUser?: string;
  adminPassword?: string;
}

export interface CloudConnectionReport {
  connected: boolean;
  activeRoleMode: CloudRoleMode;
  configSource: string;
  latencyMs: number;
  serverVersion: string;
  postgisVersion?: string;
  database: string;
  sslMode: string;
  schemasBootstrapped: boolean;
  canReadMaster: boolean;
  canSubmitStaging: boolean;
  canMergeMaster: boolean;
  canBootstrapSchema: boolean;
  becmasterPlotCount: number;
  becmasterVegCount: number;
  pendingSubmissionCount: number;
  snapshotCount: number;
  latestSnapshotTag?: string;
  activeStandardsRelease?: string;
  activeBecVersion?: string;
  activeTaxonomyVersion?: string;
  activeVListsSha256?: string;
  checkedAt: string;
}

export interface SchemaBootstrapResult {
  schemaVersion: number;
  appliedMigrations: string[];
  schemasVerified: string[];
  postgisEnabled: boolean;
  pgcryptoEnabled: boolean;
  roleGrantsApplied: boolean;
  contributorRoleSet: boolean;
  bootstrappedAt: string;
}

export interface PublishStandardsRequest {
  releaseId?: string;
  becVersion?: string;
  taxonomyVersion?: string;
  vlistsPath?: string;
  becDbPath?: string;
  vmetaDataPath?: string;
  notes?: string;
}

export interface PublishStandardsResult {
  releaseId: string;
  becVersion: string;
  taxonomyVersion: string;
  vlistsSha256: string;
  becDbSha256: string;
  vmetaDataSha256: string;
  tableCounts: Record<string, number>;
  elapsedMs: number;
  publishedAt: string;
}

export interface SeedBaselineRequest {
  sourceDbPath: string;
  sourceProject?: string;
  snapshotTag?: string;
  replaceExisting: boolean;
  skipAudit?: boolean;
  notes?: string;
}

export interface SeedBaselineResult {
  sourceProject: string;
  sourceSha256: string;
  snapshotTag: string;
  plotCount: number;
  adminCount: number;
  vegCount: number;
  humusCount: number;
  mineralCount: number;
  otherCount: number;
  metadataCount: number;
  auditCount: number;
  childIdentityRows: number;
  tailAuditHash: string;
  elapsedMs: number;
  seededAt: string;
  tableCounts: Record<string, number>;
}

export interface PullStandardsResult {
  updated: boolean;
  alreadyCurrent: boolean;
  releaseId: string;
  becVersion: string;
  taxonomyVersion: string;
  localSha256: string;
  cloudSha256: string;
  publishedAt?: string;
  state?: unknown;
}

export interface SubsetFilter {
  zones?: string[];
  subzones?: string[];
  projectIds?: string[];
  plotNumbers?: string[];
  bbox?: number[];
  limit?: number;
  allowFullCheckout?: boolean;
}

export interface SubsetPreview {
  matchingPlots: number;
  matchingVeg: number;
  samplePlots: string[];
  zoneBreakdown: Record<string, number>;
}

export interface PullSubsetRequest {
  filter: SubsetFilter;
  targetProject: string;
  destinationPath?: string;
  activateOnSuccess: boolean;
}

export interface PullSubsetResult {
  targetProject: string;
  destinationPath: string;
  destinationSha256: string;
  plotCount: number;
  vegCount: number;
  humusCount: number;
  mineralCount: number;
  otherCount: number;
  metadataCount: number;
  downloadLogId: number;
  activated: boolean;
  state?: unknown;
}

export interface CloudUserSpeciesRecord {
  code: string;
  scientificName: string | null;
  lifeForm: number | null;
  englishName: string | null;
  report?: number;
  sppNumber?: number;
  codetype?: string;
}

export interface CloudUserSpeciesManifest {
  schemaVersion: string;
  exportedAt: string;
  sourceProject: string;
  rowCount: number;
  payloadSha256: string;
  species: CloudUserSpeciesRecord[];
}

export interface StagingPreflightPreview {
  project: string;
  projectId: string;
  plotCount: number;
  vegRowCount: number;
  sourceDbSha256: string;
  compliancePassed: boolean;
  blockingErrorCount: number;
  complianceSummary: Record<string, number>;
  complianceReport: unknown;
  userSpeciesManifest: CloudUserSpeciesManifest;
  lumpRowCount: number;
  suRowCount: number;
}

export interface SubmitStagingRequest {
  useActiveSu?: boolean;
  submitterNotes?: string;
  submittedBy?: string;
  baseSnapshotTag?: string;
  custodianOverride?: boolean;
}

export interface SubmitStagingResult {
  submissionId: string;
  projectName: string;
  projectId: string;
  submittedBy: string;
  dbRole: string;
  status: string;
  plotCount: number;
  vegRowCount: number;
  userSpeciesCount: number;
  sourceDbSha256: string;
  compliancePassed: boolean;
  submittedAt: string;
}

export interface ListSubmissionsFilter {
  status?: string;
  submittedBy?: string;
  limit?: number;
}

export interface StagingSubmissionSummary {
  submissionId: string;
  projectName: string;
  projectId?: string;
  submittedBy: string;
  dbRole: string;
  submittedAt: string;
  status: string;
  plotCount: number;
  vegRowCount: number;
  sourceDbSha256: string;
  compliancePassed: boolean;
  userSpeciesCount: number;
  lumpRowCount: number;
  suRowCount: number;
  submitterNotes?: string;
  baseSnapshotTag?: string;
  reviewerNotes?: string;
  reviewedBy?: string;
  reviewedAt?: string;
  mergedTxId?: string;
}

export interface ProvisionalSpeciesDiff {
  code: string;
  scientificName?: string;
  lifeForm?: number;
  status: string;
  officialCode?: string;
  officialScientific?: string;
}

export interface StagingSubmissionDetail {
  summary: StagingSubmissionSummary;
  complianceReport: unknown;
  userSpeciesManifest: CloudUserSpeciesManifest;
  provisionalSpeciesDiffs: ProvisionalSpeciesDiff[];
  lumpRows: { lumpCode: string; sppCode: string; use: boolean }[];
  suRows: { plotNumber: string; siteUnit: string | null }[];
}

export interface CloudMergeConflictItem {
  conflictId: string;
  plotNumber: string;
  table: string;
  key: string;
  field: string;
  baseVal?: string | null;
  sourceVal: string | null;
  targetVal: string | null;
  kind: string;
  defaultResolution: CloudMergeResolution;
  custodianProtected: boolean;
}

export interface CloudMergePlotPreview {
  plotNumber: string;
  category: string;
  items: CloudMergeConflictItem[];
}

export interface StagingMergePreviewRequest {
  submissionId: string;
  baseDbPath?: string;
  baseProject?: string;
}

export interface StagingMergePreview {
  submissionId: string;
  sourceProject: string;
  targetProject: string;
  baseDbPath?: string;
  baseProject?: string;
  previewDigest: string;
  addedCount: number;
  identicalCount: number;
  fastForwardCount: number;
  conflictCount: number;
  targetOnlyCount: number;
  elapsedMs: number;
  plots: CloudMergePlotPreview[];
  conflicts: CloudMergeConflictItem[];
  custodianProtectedDiffs: CloudMergeConflictItem[];
}

export interface StagingMergeExecuteRequest {
  submissionId: string;
  baseDbPath?: string;
  baseProject?: string;
  expectedPreviewDigest: string;
  defaultConflictResolution?: CloudMergeResolution;
  plotResolutions?: Record<string, CloudMergeResolution>;
  fieldResolutions?: Record<string, CloudMergeResolution>;
  reviewerNotes?: string;
  actor?: string;
}

export interface StagingMergeResult {
  mergeTxId: string;
  submissionId: string;
  sourceProject: string;
  addedPlots: number;
  updatedPlots: number;
  skippedPlots: number;
  allocatedChildIds: number[];
  reservedDeletedIds: number[];
  auditRowsWritten: number;
  tailAuditHash: string;
  mergedAt: string;
}

export interface RejectSubmissionRequest {
  submissionId: string;
  reviewerNotes: string;
  actor?: string;
}

export interface CreateSnapshotRequest {
  snapshotTag: string;
  notes?: string;
  exportFormat?: string;
  destinationPath?: string;
  zoneFilter?: string;
}

export interface SnapshotRecord {
  snapshotTag: string;
  createdAt: string;
  createdBy: string;
  plotCount: number;
  vegCount: number;
  becVersion: string;
  taxonomyVersion: string;
  manifestSha256: string;
  notes?: string;
  exportedPath?: string;
}

export interface DownloadLogEntry {
  id: number;
  downloadedAt: string;
  actor: string;
  roleTier: string;
  filterJson: unknown;
  format: string;
  plotCount: number;
}

export interface CloudSyncPort {
  getCloudSyncConfigStatus(): Promise<CloudConfigStatus>;
  configureCloudSyncSession(req: SessionConfigRequest): Promise<CloudConfigStatus>;
  testCloudConnection(): Promise<CloudConnectionReport>;
  bootstrapCloudSchemas(): Promise<SchemaBootstrapResult>;
  publishStandardsToCloud(contextId: string, req: PublishStandardsRequest): Promise<PublishStandardsResult>;
  seedBECMasterBaseline(req: SeedBaselineRequest): Promise<SeedBaselineResult>;
  pullCloudStandards(contextId: string): Promise<PullStandardsResult>;
  previewBECMasterSubset(filter: SubsetFilter): Promise<SubsetPreview>;
  pullBECMasterSubset(contextId: string, req: PullSubsetRequest): Promise<PullSubsetResult>;
  previewStagingSubmission(contextId: string, useActiveSu: boolean): Promise<StagingPreflightPreview>;
  submitProjectToStaging(contextId: string, req: SubmitStagingRequest): Promise<SubmitStagingResult>;
  listStagingSubmissions(filter: ListSubmissionsFilter): Promise<StagingSubmissionSummary[]>;
  getStagingSubmissionDetail(submissionId: string): Promise<StagingSubmissionDetail>;
  previewStagingToMasterMerge(req: StagingMergePreviewRequest): Promise<StagingMergePreview>;
  executeStagingToMasterMerge(req: StagingMergeExecuteRequest): Promise<StagingMergeResult>;
  rejectStagingSubmission(req: RejectSubmissionRequest): Promise<StagingSubmissionSummary>;
  createBECMasterSnapshot(contextId: string, req: CreateSnapshotRequest): Promise<SnapshotRecord>;
  listBECMasterSnapshots(): Promise<SnapshotRecord[]>;
  listBECMasterDownloadLog(limit: number): Promise<DownloadLogEntry[]>;
}

export const defaultCloudSyncPort: CloudSyncPort = {
  getCloudSyncConfigStatus: () => Call.ByName('main.ContextService.GetCloudSyncConfigStatus'),
  configureCloudSyncSession: req => Call.ByName('main.ContextService.ConfigureCloudSyncSession', req),
  testCloudConnection: () => Call.ByName('main.ContextService.TestCloudConnection'),
  bootstrapCloudSchemas: () => Call.ByName('main.ContextService.BootstrapCloudSchemas'),
  publishStandardsToCloud: (contextId, req) => Call.ByName('main.ContextService.PublishStandardsToCloud', contextId, req),
  seedBECMasterBaseline: req => Call.ByName('main.ContextService.SeedBECMasterBaseline', req),
  pullCloudStandards: contextId => Call.ByName('main.ContextService.PullCloudStandards', contextId),
  previewBECMasterSubset: filter => Call.ByName('main.ContextService.PreviewBECMasterSubset', filter),
  pullBECMasterSubset: (contextId, req) => Call.ByName('main.ContextService.PullBECMasterSubset', contextId, req),
  previewStagingSubmission: (contextId, useActiveSu) => Call.ByName('main.ContextService.PreviewStagingSubmission', contextId, useActiveSu),
  submitProjectToStaging: (contextId, req) => Call.ByName('main.ContextService.SubmitProjectToStaging', contextId, req),
  listStagingSubmissions: filter => Call.ByName('main.ContextService.ListStagingSubmissions', filter),
  getStagingSubmissionDetail: submissionId => Call.ByName('main.ContextService.GetStagingSubmissionDetail', submissionId),
  previewStagingToMasterMerge: req => Call.ByName('main.ContextService.PreviewStagingToMasterMerge', req),
  executeStagingToMasterMerge: req => Call.ByName('main.ContextService.ExecuteStagingToMasterMerge', req),
  rejectStagingSubmission: req => Call.ByName('main.ContextService.RejectStagingSubmission', req),
  createBECMasterSnapshot: (contextId, req) => Call.ByName('main.ContextService.CreateBECMasterSnapshot', contextId, req),
  listBECMasterSnapshots: () => Call.ByName('main.ContextService.ListBECMasterSnapshots'),
  listBECMasterDownloadLog: limit => Call.ByName('main.ContextService.ListBECMasterDownloadLog', limit),
};

export interface SubsetFilterDraftInput {
  zonesRaw: string;
  subzonesRaw: string;
  projectIdsRaw: string;
  plotNumbersRaw: string;
  bboxRaw: string;
  limitRaw?: string;
  allowFullCheckout?: boolean;
}

export function parseBboxInput(raw: string): { bbox?: number[]; error: string | null } {
  const trimmed = raw.trim();
  if (trimmed === '') {
    return { bbox: undefined, error: null };
  }
  const parts = trimmed.split(/[\s,]+/).filter(Boolean);
  if (parts.length !== 4) {
    return { error: 'Bounding box requires 4 numbers: minLon, minLat, maxLon, maxLat.' };
  }
  const nums = parts.map(Number);
  if (nums.some(n => !Number.isFinite(n))) {
    return { error: 'Bounding box values must be finite decimal degrees.' };
  }
  const [minLon, minLat, maxLon, maxLat] = nums;
  if (minLon < -180 || maxLon > 180 || minLon >= maxLon) {
    return { error: 'Bounding box longitudes must satisfy -180 <= minLon < maxLon <= 180.' };
  }
  if (minLat < -90 || maxLat > 90 || minLat >= maxLat) {
    return { error: 'Bounding box latitudes must satisfy -90 <= minLat < maxLat <= 90.' };
  }
  return { bbox: [minLon, minLat, maxLon, maxLat], error: null };
}

export function buildSubsetFilterDraft(input: SubsetFilterDraftInput): { filter: SubsetFilter | null; error: string | null } {
  const zones = parseLiteralTokenList(input.zonesRaw);
  const subzones = parseLiteralTokenList(input.subzonesRaw);
  const projectIds = parseLiteralTokenList(input.projectIdsRaw);
  const plotNumbers = parseLiteralTokenList(input.plotNumbersRaw);

  for (const plot of plotNumbers) {
    if (!wellFormedUTF16(plot) || plot.includes('\0') || plot.length > 7) {
      return { filter: null, error: `Plot number "${plot}" exceeds 7 UTF-16 code units or contains invalid characters.` };
    }
  }

  const bboxParsed = parseBboxInput(input.bboxRaw);
  if (bboxParsed.error) {
    return { filter: null, error: bboxParsed.error };
  }

  let limit: number | undefined;
  const limitTrimmed = (input.limitRaw ?? '').trim();
  if (limitTrimmed !== '') {
    const parsed = Number(limitTrimmed);
    if (!Number.isInteger(parsed) || parsed <= 0) {
      return { filter: null, error: 'Subset plot limit must be a positive integer.' };
    }
    limit = parsed;
  }

  const allowFullCheckout = !!input.allowFullCheckout;
  const hasFilter =
    zones.length > 0 ||
    subzones.length > 0 ||
    projectIds.length > 0 ||
    plotNumbers.length > 0 ||
    !!bboxParsed.bbox ||
    limit !== undefined;

  if (!hasFilter && !allowFullCheckout) {
    return {
      filter: null,
      error: 'Specify at least one regional filter (Zone, Subzone, ProjectID, PlotNumber, BBox, or Limit), or enable full checkout.',
    };
  }

  const filter: SubsetFilter = {};
  if (zones.length > 0) filter.zones = zones;
  if (subzones.length > 0) filter.subzones = subzones;
  if (projectIds.length > 0) filter.projectIds = projectIds;
  if (plotNumbers.length > 0) filter.plotNumbers = plotNumbers;
  if (bboxParsed.bbox) filter.bbox = bboxParsed.bbox;
  if (limit !== undefined) filter.limit = limit;
  if (allowFullCheckout) filter.allowFullCheckout = true;

  return { filter, error: null };
}

export function canUploadStagingSubmission(
  preflight: StagingPreflightPreview | null,
  activeRoleMode: CloudRoleMode,
  custodianOverride: boolean
): boolean {
  if (!preflight) return false;
  if (preflight.compliancePassed) return true;
  return activeRoleMode === 'custodian' && custodianOverride;
}

export function isCustodianProtectedConflict(item: Pick<CloudMergeConflictItem, 'field' | 'custodianProtected'>): boolean {
  if (item.custodianProtected) return true;
  const lower = item.field.toLowerCase();
  return CUSTODIAN_PROTECTED_FIELDS.some(f => f.toLowerCase() === lower);
}

export function buildInitialCloudMergeFieldResolutions(preview: StagingMergePreview): Record<string, CloudMergeResolution> {
  const map: Record<string, CloudMergeResolution> = {};
  for (const item of preview.custodianProtectedDiffs ?? []) {
    map[item.conflictId] = 'keep_target';
  }
  return map;
}

export function validateSnapshotTagInput(raw: string): string | null {
  if (typeof raw !== 'string' || !wellFormedUTF16(raw) || raw.includes('\0')) {
    return 'Snapshot tag must be well-formed Unicode without NUL characters.';
  }
  const trimmed = raw.trim();
  if (trimmed === '' || trimmed !== raw) {
    return 'Snapshot tag is required and cannot contain leading or trailing spaces.';
  }
  if (!/^[A-Za-z0-9._-]{1,64}$/.test(trimmed)) {
    return 'Snapshot tag may use only ASCII letters, digits, dots, underscores, or hyphens (max 64 characters).';
  }
  return null;
}
