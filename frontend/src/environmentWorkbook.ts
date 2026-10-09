import type { EnvironmentWorkbookReview, EnvironmentWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { validateLongEnvironmentPreview } from './longEnvironmentReport';
import { lifeformShape as shape, lifeformText as text, type LifeformSummaryOwner } from './lifeformSummary';
import { PublicationSession } from './publicationSession';

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const ownerKey = (owner: LifeformSummaryOwner) => JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);

export type ValidatedEnvironmentWorkbookReview = Omit<EnvironmentWorkbookReview, 'sheets'> & {
  sheets: NonNullable<EnvironmentWorkbookReview['sheets']>;
};

export function validateEnvironmentWorkbookReview(value: EnvironmentWorkbookReview | null,
  owner: LifeformSummaryOwner, title: string): ValidatedEnvironmentWorkbookReview {
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['preview', 'sheets', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) ||
      typeof value.bytes !== 'number' || !Number.isSafeInteger(value.bytes) || value.bytes <= 0 ||
      !Array.isArray(value.sheets)) throw new Error('Workbook review has incomplete source approval or byte metadata.');
  const preview = validateLongEnvironmentPreview(value.preview, owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath, title);
  const units = preview.report.units;
  if (!units || units.length === 0 || value.sheets.length !== units.length) {
    throw new Error('Workbook review omits complete report units.');
  }
  const names = new Set<string>();
  for (const [i, sheet] of value.sheets.entries()) {
    const unit = units[i];
    let expected = unit.code === '' ? `NoName${i}` : unit.code;
    expected = expected.slice(0, 31).replace(/[:/\\[\]*?]/g, '-');
    if (!shape(sheet, ['unit', 'name']) || sheet.unit !== unit.code || !text(sheet.name) || sheet.name !== expected ||
        names.has(sheet.name.toLowerCase()) || sheet.name.startsWith("'") || sheet.name.endsWith("'") ||
        sheet.name.toLowerCase() === '_vpro_source') throw new Error('Workbook worksheet mapping differs from the reviewed original units.');
    names.add(sheet.name.toLowerCase());
  }
  return { ...value, preview, sheets: value.sheets };
}

export function validateEnvironmentWorkbookOutcome(value: EnvironmentWorkbookOutcome | null,
  destination: string, expectedHash: string): EnvironmentWorkbookOutcome {
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['status', 'requestedDestination', 'path', 'sha256', 'errorMessage']) ||
      value.requestedDestination !== destination || !text(value.path) || !text(value.errorMessage) ||
      !['published', 'published-with-errors', 'not-published'].includes(value.status) ||
      value.sha256 !== '' && !hash(value.sha256)) throw new Error('Workbook acknowledgement is incomplete; outcome unknown, never replay.');
  const committed = value.status !== 'not-published';
  if (committed && (!value.path || value.sha256 !== expectedHash) ||
      value.status === 'published' && value.errorMessage !== '' ||
      value.status !== 'published' && value.errorMessage === '') throw new Error('Workbook acknowledgement contradicts the planned bytes/outcome; never replay.');
  return value;
}

export class EnvironmentWorkbookPublicationSession {
  private receipt = new PublicationSession<EnvironmentWorkbookOutcome>('Long Environment workbook');
  constructor(private readonly owner: LifeformSummaryOwner) { this.owner = structuredClone(owner); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  async publish(review: EnvironmentWorkbookReview, destination: string,
    port: (request: { title: string; approvalHash: string; destination: string }) => PromiseLike<EnvironmentWorkbookOutcome | null>) {
    if (!text(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Workbook publication requires an explicit literal .xlsx destination.');
    }
    const validated = validateEnvironmentWorkbookReview(review, this.owner, review.preview.report.title);
    const request = { title: validated.preview.report.title, approvalHash: validated.approvalHash, destination };
    await this.receipt.publish(request, destination, port,
      value => validateEnvironmentWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { key: string; session: EnvironmentWorkbookPublicationSession }>();
export function environmentWorkbookPublicationSession(owner: LifeformSummaryOwner): EnvironmentWorkbookPublicationSession {
  const key = ownerKey(owner);
  const existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key) throw new Error('Workbook receipt belongs to different owned paths.');
    return existing.session;
  }
  const session = new EnvironmentWorkbookPublicationSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}
