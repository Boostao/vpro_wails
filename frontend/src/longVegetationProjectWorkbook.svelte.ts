import type {
  LongVegetationProjectWorkbookReview, LongVegetationReportScope, VegetationWorkbookOutcome,
} from '../bindings/github.com/boostao/vpro-wails';
import type { ProjectReportOwner } from './longVegetationProjectReport';
import { projectReportOwnedScope, validateProjectUnitTuples } from './longVegetationProjectReport';
import type { CurrentProjectScope } from './longVegetationProjectScope';
import { currentProjectScope, sameProjectTuples, validateProjectPreview } from './longVegetationProjectScope';
import type { ValidatedProjectPreview } from './longVegetationProjectReport';
import { lifeformShape as shape, lifeformText as text, lifeformCount as count } from './lifeformSummary';
import {
  workbookCalendarDate, validateWorkbookVersionEvidence, validateWorkbookWorksheetEvidence,
  validateVegetationWorkbookOutcome,
} from './vegetationWorkbook';
import { PublicationSession } from './publicationSession';

export type ValidatedProjectWorkbookReview = Omit<LongVegetationProjectWorkbookReview,
  'scope' | 'preview' | 'sheets' | 'skippedUnits'> & {
  scope: CurrentProjectScope;
  preview: ValidatedProjectPreview;
  sheets: NonNullable<LongVegetationProjectWorkbookReview['sheets']>;
  skippedUnits: NonNullable<LongVegetationProjectWorkbookReview['skippedUnits']>;
};

export type ProjectWorkbookReadRequest = { revision: string; buildApprovalHash: string; scope: 'unlumped' };
export type ProjectWorkbookExportRequest = ProjectWorkbookReadRequest & {
  createdDate: string; approvalHash: string; destination: string;
};

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const ownerKey = (owner: ProjectReportOwner) =>
  JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);

function freeze(value: object): void {
  for (const child of Object.values(value)) if (child && typeof child === 'object') freeze(child);
  Object.freeze(value);
}

export function projectWorkbookReadRequest(scope: LongVegetationReportScope, owner: ProjectReportOwner,
  enabled = false): ProjectWorkbookReadRequest {
  const current = currentProjectScope(scope, owner, enabled);
  return { revision: current.revision, buildApprovalHash: current.build.approvalHash, scope: 'unlumped' };
}

export function validateProjectWorkbookReview(value: LongVegetationProjectWorkbookReview | null,
  expected: LongVegetationReportScope, owner: ProjectReportOwner, enabled = false): ValidatedProjectWorkbookReview {
  if (enabled !== true) throw new Error('Whole-project workbook client is disabled in this build.');
  value = value ? structuredClone($state.snapshot(value)) : null;
  if (!shape(value, ['scope', 'preview', 'options', 'summary', 'reportScope', 'createdDate',
      'sheets', 'skippedUnits', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      value.reportScope !== 'unlumped' || !text(value.createdDate) || !workbookCalendarDate(value.createdDate) ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) || !count(value.bytes) || !value.bytes ||
      !Array.isArray(value.sheets) || !Array.isArray(value.skippedUnits) ||
      !shape(value.options, ['quickReport', 'spaceBetweenGroups', 'reportSummary']) ||
      typeof value.options.quickReport !== 'boolean' || typeof value.options.spaceBetweenGroups !== 'boolean' ||
      typeof value.options.reportSummary !== 'boolean') {
    throw new Error('Project workbook review lost its explicit original scope or reviewed bytes/layout.');
  }
  const { scope, preview } = validateProjectPreview({ scope: value.scope, preview: value.preview }, expected, owner, enabled);
  if (value.options.reportSummary) {
    const summary = value.summary;
    if (!shape(summary, ['createdDate', 'environmentRows', 'sourcePlotRows', 'sourceKind',
        'projectTuples', 'speciesVersion', 'versionStatus', 'versionDefinitions']) ||
        summary.createdDate !== value.createdDate || !count(summary.environmentRows) || !count(summary.sourcePlotRows) ||
        summary.sourceKind !== scope.sourceKind || !Array.isArray(summary.projectTuples) ||
        !sameProjectTuples(summary.projectTuples, preview.report.projectUnits) ||
        summary.sourcePlotRows !== preview.report.projectUnits.filter(row => row.plotNumber.storage !== 'null').length ||
        summary.environmentRows < new Set(scope.build.projectTuples.map(row => row.envRowId)).size) {
      throw new Error('Project workbook summary contradicts complete original Build, output tuples or physical Env counts.');
    }
    validateProjectUnitTuples(summary.projectTuples, scope.sourceKind);
    validateWorkbookVersionEvidence(summary);
  } else if (value.summary !== null) {
    throw new Error('Unrequested project workbook summary was supplied.');
  }
  validateWorkbookWorksheetEvidence(value, preview);
  const result = { ...value, scope, preview, sheets: value.sheets, skippedUnits: value.skippedUnits };
  freeze(result);
  return result;
}

export class ProjectWorkbookPublicationSession {
  private readonly owner: ProjectReportOwner;
  private readonly receipt = new PublicationSession<VegetationWorkbookOutcome>('Original project Long Vegetation workbook');
  private readonly consumed = new Set<string>();

  constructor(owner: ProjectReportOwner, private readonly enabled = false) {
    if (enabled !== true) throw new Error('Whole-project workbook client is disabled in this build.');
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project workbook receipt requires complete frozen physical owner paths.');
    this.owner = Object.freeze({ ...owner });
  }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }

  prepare(review: LongVegetationProjectWorkbookReview, expected: LongVegetationReportScope): ValidatedProjectWorkbookReview {
    if (this.view().busy || this.view().blocked) throw new Error('Acknowledge the prior project workbook outcome before reviewing again.');
    const validated = validateProjectWorkbookReview(review, expected, this.owner, this.enabled);
    this.consumed.delete(validated.approvalHash);
    return validated;
  }

  async publish(review: LongVegetationProjectWorkbookReview, expected: LongVegetationReportScope, destination: string,
    port: (request: ProjectWorkbookExportRequest) => PromiseLike<VegetationWorkbookOutcome | null>): Promise<void> {
    if (!text(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Project workbook publication requires an explicit literal .xlsx destination.');
    }
    const validated = validateProjectWorkbookReview(review, expected, this.owner, this.enabled);
    if (this.view().busy || this.view().blocked) throw new Error('Acknowledge the prior project workbook outcome before a new publication.');
    if (this.consumed.has(validated.approvalHash)) throw new Error('Obtain a fresh explicit project workbook review before retrying.');
    this.consumed.add(validated.approvalHash);
    const request: ProjectWorkbookExportRequest = {
      ...projectWorkbookReadRequest(validated.scope, this.owner, this.enabled),
      createdDate: validated.createdDate, approvalHash: validated.approvalHash, destination,
    };
    await this.receipt.publish(request, destination, port,
      value => validateVegetationWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { key: string; enabled: boolean; session: ProjectWorkbookPublicationSession }>();
export function projectWorkbookPublicationSession(owner: ProjectReportOwner, enabled = false): ProjectWorkbookPublicationSession {
  const key = ownerKey(owner), existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key || existing.enabled !== enabled) {
      throw new Error('Project workbook receipt belongs to different physical paths or immutable capability.');
    }
    return existing.session;
  }
  const session = new ProjectWorkbookPublicationSession(owner, enabled);
  sessions.set(owner.contextId, { key, enabled, session });
  return session;
}
