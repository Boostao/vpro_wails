import type { SummaryVegetationWorkbookOptions, VegetationWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { lifeformShape as shape } from './lifeformSummary';
import { workbookText, workbookWorksheetName, workbookFoldedName } from './lifeformWorkbook';
import { summaryVegetationOptionError, validateSummaryVegetationAsync,
  type SummaryVegetationOwner, type SummaryVegetationPreview } from './summaryVegetation';
import { validateSummaryEnvironmentalLifeformOptions,
  type SummaryEnvironmentalLifeformPreview } from './summaryVegetationEnvironmental';
import { PublicationSession } from './publicationSession';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';

export const summaryVegetationWorkbookScope = 'Explicit read-only normal-SU layer inspection; literal XLSX, not original NewShortVeg orchestration, saved callbacks, ancillary sheets or formatting/printing parity';
export type SummaryWorkbookCalculation = Pick<SummaryVegetationWorkbookOptions, 'average' | 'display' | 'showClass'>;
export interface ValidatedSummaryVegetationWorkbookReview {
  preview: SummaryVegetationPreview; environment: SummaryEnvironmentalLifeformPreview | null;
  options: SummaryVegetationWorkbookOptions; scope: string; sheet: string; firstVegetationRow: number;
  approvalHash: string; workbookSHA256: string; bytes: number;
}
export type SummaryVegetationWorkbookExportRequest = {
  options: SummaryVegetationWorkbookOptions; approvalHash: string; destination: string;
};
export interface SummaryWorkbookRead extends PromiseLike<unknown> { cancel?(): void }
export interface SummaryWorkbookPort {
  review(contextId: string, options: SummaryVegetationWorkbookOptions): SummaryWorkbookRead;
  validationYield(): PromiseLike<void>;
}
const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const ownerKey = (owner: SummaryVegetationOwner) =>
  JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
function reject(): never {
  throw new Error('Summary inspection workbook review has inconsistent options, owned source, scope or frame; no repairs applied.');
}
function optionsValue(value: unknown): SummaryVegetationWorkbookOptions {
  if (!shape(value, ['title', 'average', 'display', 'showClass', 'includeEnv']) ||
      !workbookText(value.title) || typeof value.includeEnv !== 'boolean' ||
      typeof value.average !== 'number' || typeof value.display !== 'number' || typeof value.showClass !== 'number' ||
      summaryVegetationOptionError(value.average, value.display, value.showClass, 11)) reject();
  return { title: value.title, average: value.average, display: value.display,
    showClass: value.showClass, includeEnv: value.includeEnv };
}
export function sameSummaryWorkbookOptions(left: SummaryVegetationWorkbookOptions,
  right: SummaryVegetationWorkbookOptions): boolean {
  return left.title === right.title && left.average === right.average && left.display === right.display &&
    left.showClass === right.showClass && left.includeEnv === right.includeEnv;
}

export async function validateSummaryVegetationWorkbookReview(value: unknown, owner: SummaryVegetationOwner,
  requested: SummaryVegetationWorkbookOptions, validationYield: () => PromiseLike<void>,
  current: () => boolean = () => true): Promise<ValidatedSummaryVegetationWorkbookReview | null> {
  if (!current()) return null;
  const options = optionsValue(requested);
  if (!shape(value, ['preview', 'environment', 'options', 'scope', 'sheet', 'firstVegetationRow',
    'approvalHash', 'workbookSHA256', 'bytes']) || !sameSummaryWorkbookOptions(optionsValue(value.options), options) ||
      value.scope !== summaryVegetationWorkbookScope || value.sheet !== owner.project ||
      workbookWorksheetName(owner.project, 0) !== owner.project ||
      workbookFoldedName(owner.project) === workbookFoldedName('_vpro_source') ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) ||
      typeof value.bytes !== 'number' || !Number.isSafeInteger(value.bytes) || value.bytes <= 0 ||
      typeof value.firstVegetationRow !== 'number' || !Number.isSafeInteger(value.firstVegetationRow)) reject();
  const { approvalHash, workbookSHA256, bytes, sheet, firstVegetationRow } = value;
  const preview = await validateSummaryVegetationAsync(value.preview, owner,
    options.average, options.display, options.showClass, 11, {
      preview: () => Promise.reject(new Error('Workbook validation cannot dispatch a read')),
      cancel() {}, validationYield,
    }, current);
  if (!current() || preview === null) return null;
  const environment = options.includeEnv
    ? await validateSummaryEnvironmentalLifeformOptions(value.environment, owner, options, validationYield, current)
    : null;
  if (!current() || options.includeEnv && environment === null) return null;
  if (!options.includeEnv && value.environment !== null || !preview.rows.length || !preview.memberships.length ||
      !preview.units.length || preview.units.length > 250 || preview.columns.length !== preview.units.length ||
      preview.columns.some((code, i) => code !== preview.units[i].code) ||
      firstVegetationRow !== (environment ? environment.rows.length + 6 : 6) ||
      preview.rows.length > 32768 - firstVegetationRow) reject();
  if (environment) {
    const pairs: [unknown, unknown][] = [[preview, environment.vegetation]];
    for (let i = 0; pairs.length; i++) {
      if (i % 256 === 0) { await validationYield(); if (!current()) return null; }
      const [left, right] = pairs.pop()!;
      if (left === right) continue;
      if (!left || !right || typeof left !== 'object' || typeof right !== 'object' ||
          Array.isArray(left) !== Array.isArray(right)) reject();
      const entries = Object.entries(left);
      if (entries.length !== Object.keys(right).length) reject();
      for (const [key, child] of entries) {
        if (!Object.hasOwn(right, key)) reject();
        pairs.push([child, Reflect.get(right, key)]);
      }
    }
  }
  const result: ValidatedSummaryVegetationWorkbookReview = {
    preview, environment, options, approvalHash, workbookSHA256, bytes, sheet, firstVegetationRow,
    scope: summaryVegetationWorkbookScope,
  };
  const pending: unknown[] = [result];
  for (let i = 0; pending.length; i++) {
    if (i % 256 === 0) { await validationYield(); if (!current()) return null; }
    const item = pending.pop();
    if (typeof item === 'string') { if (!workbookText(item)) reject(); }
    else if (item && typeof item === 'object') {
      for (const child of Object.values(item)) pending.push(child);
      Object.freeze(item);
    }
  }
  return current() ? result : null;
}

export class SummaryVegetationWorkbookSession {
  private publication = new PublicationSession<VegetationWorkbookOutcome>('Summary inspection workbook');
  private listeners = new Set<() => void>();
  private generation = 0;
  private read: SummaryWorkbookRead | null = null;
  private state = { title: '', includeEnv: false, destination: '', draftError: '', error: '',
    reviewing: false, review: null as ValidatedSummaryVegetationWorkbookReview | null };
  constructor(private readonly owner: SummaryVegetationOwner) {
    this.owner = Object.freeze(structuredClone(owner));
    this.publication.subscribe(() => {
      if (this.publication.view().busy) this.state.review = null;
      this.notify();
    });
  }
  view() { return { ...this.state, publication: this.publication.view() }; }
  held(): boolean {
    const publication = this.publication.view();
    return this.state.reviewing || !!this.state.draftError || publication.busy || publication.blocked;
  }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener(); return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  private editable(): boolean {
    const publication = this.publication.view();
    return !this.state.reviewing && !publication.busy && !publication.blocked;
  }
  options(calculation: SummaryWorkbookCalculation): SummaryVegetationWorkbookOptions {
    return optionsValue({ ...calculation, title: this.state.title, includeEnv: this.state.includeEnv });
  }
  edit(field: 'title' | 'destination', raw: string): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before editing.');
    this.state[field] = raw; this.state.error = '';
    if (field === 'title') this.state.review = null;
    this.validateDraft(); this.notify();
  }
  includeEnvironment(include: boolean): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before editing.');
    this.state.includeEnv = include; this.state.review = null; this.state.error = '';
    this.notify();
  }
  private validateDraft() {
    this.state.draftError = !workbookText(this.state.title)
      ? 'Title requires valid literal workbook text of at most 32767 UTF-16 units.'
      : !workbookText(this.state.destination) || this.state.destination !== '' && !/\.xlsx$/i.test(this.state.destination)
        ? 'Destination requires an explicit literal .xlsx path; no trimming or replacement.' : '';
  }
  undoDraft(): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before Undo.');
    Object.assign(this.state, { title: '', includeEnv: false, destination: '', draftError: '', error: '', review: null });
    this.notify();
  }
  cancel(): void {
    if (!this.state.reviewing) return;
    this.generation++; this.read?.cancel?.(); this.state.review = null;
    this.state.error = 'Workbook review cancelled; waiting for the prior read/validation to settle.';
    this.notify();
  }
  async review(calculation: SummaryWorkbookCalculation, port: SummaryWorkbookPort): Promise<boolean> {
    if (this.held()) throw new Error('Resolve workbook validation or wait for the prior operation before review.');
    const options = this.options(calculation), generation = ++this.generation;
    this.state.reviewing = true; this.state.review = null; this.state.error = ''; this.notify();
    try {
      this.read = port.review(this.owner.contextId, options);
      const value = await this.read;
      if (generation !== this.generation) return false;
      const result = await validateSummaryVegetationWorkbookReview(value, this.owner, options,
        port.validationYield, () => generation === this.generation);
      if (generation !== this.generation || result === null) return false;
      this.state.review = result;
      return true;
    } catch (cause) {
      if (generation === this.generation) this.state.error = `Workbook review unavailable; no file published: ${String(cause)}`;
      return false;
    } finally {
      this.read = null; this.state.reviewing = false; this.notify();
    }
  }
  async publish(calculation: SummaryWorkbookCalculation,
    port: (request: SummaryVegetationWorkbookExportRequest) => PromiseLike<VegetationWorkbookOutcome | null>): Promise<void> {
    if (this.held()) throw new Error('Resolve workbook validation or acknowledge the prior outcome before publication.');
    const review = this.state.review, options = this.options(calculation);
    if (!review || !sameSummaryWorkbookOptions(review.options, options))
      throw new Error('Review the current explicit workbook options before publication.');
    if (!this.state.destination) throw new Error('Choose an explicit literal .xlsx destination before publication.');
    await this.publication.publish({ options, approvalHash: review.approvalHash, destination: this.state.destination },
      this.state.destination, port,
      value => validateEnvironmentWorkbookOutcome(value, this.state.destination, review.workbookSHA256));
  }
  acknowledge(): void { this.publication.acknowledge(); }
}

const sessions = new Map<string, { key: string; session: SummaryVegetationWorkbookSession }>();
export function summaryVegetationWorkbookSession(owner: SummaryVegetationOwner): SummaryVegetationWorkbookSession {
  const key = ownerKey(owner), existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key) throw new Error('Summary inspection workbook draft/receipt belongs to different owned paths.');
    return existing.session;
  }
  const session = new SummaryVegetationWorkbookSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}
