import type { SummaryVegetationWorkbookOptions as GeneratedSummaryWorkbookOptions, SummaryVegetationWorkbookAncillaryReview,
  SummaryVegetationWorkbookReportSummary, VegetationWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { lifeformShape as shape } from './lifeformSummary';
import { workbookText, workbookWorksheetName, workbookFoldedName } from './lifeformWorkbook';
import { summaryVegetationOptionError, summaryVegetationReferenceGroupingAvailable, summaryVegetationUnanimousCommonAvailable, validateSummaryVegetationAsync,
  type SummaryVegetationOwner, type SummaryVegetationPreview, type SummaryVegetationReferenceMode } from './summaryVegetation';
import { validateSummaryEnvironmentalLifeformOptions,
  type SummaryEnvironmentalLifeformPreview } from './summaryVegetationEnvironmental';
import { PublicationSession } from './publicationSession';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';

export const summaryVegetationWorkbookScope = 'Explicit read-only normal-SU layer inspection; literal XLSX, not original NewShortVeg orchestration, saved callbacks, ancillary sheets or formatting/printing parity';
export const summaryVegetationAncillaryWorkbookScope = 'Explicit read-only normal-SU layer inspection with SU Table and optional ReportSummary; literal XLSX, not original NewShortVeg preflight/callbacks or formatting/printing parity';
export const summaryVegetationEffectiveReferencesScope = 'Explicit read-only normal-SU layer inspection with projected attribute/common/guide references; literal comparison and XLSX, not original Report callbacks, Access collation or formatting/printing parity';
export const summaryVegetationEffectiveAncillaryScope = 'Explicit read-only normal-SU layer inspection with projected attribute/common/guide references, SU Table and optional ReportSummary; literal comparison and XLSX, not original Report callbacks, Access collation or formatting/printing parity';
export const summaryVegetationGroupedWorkbookScope = 'Explicit read-only normal-SU Strata/Lifeform/None inspection; typed XLSX, not grouped common/guide references, ancillary worksheets, original Report callbacks or formatting/printing parity';
export const summaryVegetationStrataReferencesScope = 'Explicit read-only normal-SU Strata effective-reference inspection; typed XLSX, not Lifeform/None common names, grouped ancillary worksheets, original Report callbacks or formatting/printing parity';
export const summaryVegetationGroupedAttributesScope = 'Explicit read-only normal-SU Lifeform/None effective-attribute inspection; typed XLSX, no common/guide names, grouped ancillary worksheets, original Report callbacks or formatting/printing parity';
export const summaryVegetationGroupedAncillaryScope = 'Explicit read-only normal-SU Strata/Lifeform/None inspection with optional effective references, SU Table and optional ReportSummary; typed XLSX, no Lifeform/None common names, original Report callbacks or formatting/printing parity';
export const summaryVegetationConstancyScope = 'Explicit read-only normal-SU Constancy inspection with numeric lifeform and raw-species grouping; typed XLSX with optional environment/ancillary composition, adapted raw-code tie ordering, not original Report callbacks or formatting/printing parity';
export const summaryVegetationUnanimousCommonScope = 'Explicit read-only normal-SU Lifeform/None unanimous common-name inspection; typed XLSX with optional environment/ancillary composition, source First empty-to-NULL output and explicit mixed-name refusal, not original Report/guide callbacks, Access collation or formatting/printing parity';
type SummaryVegetationWorkbookOptions = Omit<GeneratedSummaryWorkbookOptions, 'references' | 'groupAndOrder' | 'constancy' | 'unanimousCommon'> & {
  references?: SummaryVegetationReferenceMode; groupAndOrder?: number; constancy?: true; unanimousCommon?: true;
};
export type SummaryWorkbookCalculation = Pick<SummaryVegetationWorkbookOptions, 'average' | 'display' | 'showClass' | 'groupAndOrder' | 'constancy'>;
type SummaryWorkbookReportSummary = Omit<SummaryVegetationWorkbookReportSummary, 'units'> & {
  units: NonNullable<SummaryVegetationWorkbookReportSummary['units']>;
};
type SummaryWorkbookAncillaryReview = Omit<SummaryVegetationWorkbookAncillaryReview, 'sheets' | 'summary'> & {
  sheets: string[]; summary: SummaryWorkbookReportSummary | null;
};
export interface ValidatedSummaryVegetationWorkbookReview {
  preview: SummaryVegetationPreview; environment: SummaryEnvironmentalLifeformPreview | null;
  options: SummaryVegetationWorkbookOptions; scope: string; sheet: string; firstVegetationRow: number;
  approvalHash: string; workbookSHA256: string; bytes: number;
  ancillary?: SummaryWorkbookAncillaryReview;
}
export type SummaryVegetationWorkbookExportRequest = {
  options: SummaryVegetationWorkbookOptions; approvalHash: string; destination: string;
  createdAt?: string;
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
  const keys = ['title', 'average', 'display', 'showClass', 'includeEnv'];
  const optional = ['ancillary', 'references', 'groupAndOrder', 'constancy', 'unanimousCommon'].filter(key =>
    value !== null && typeof value === 'object' && Object.hasOwn(value, key));
  if (!shape(value, [...keys, ...optional]) ||
      !workbookText(value.title) || typeof value.includeEnv !== 'boolean' ||
      typeof value.average !== 'number' || typeof value.display !== 'number' || typeof value.showClass !== 'number') reject();
  const result: SummaryVegetationWorkbookOptions = { title: value.title, average: value.average, display: value.display,
    showClass: value.showClass, includeEnv: value.includeEnv };
  if (Object.hasOwn(value, 'groupAndOrder')) {
    if (value.groupAndOrder !== 12 && value.groupAndOrder !== 13 && value.groupAndOrder !== 14) reject();
    result.groupAndOrder = value.groupAndOrder;
  }
  if (summaryVegetationOptionError(value.average, value.display, value.showClass, result.groupAndOrder ?? 11)) reject();
  if (Object.hasOwn(value, 'ancillary')) {
    if (!shape(value.ancillary, ['createSummary']) || typeof value.ancillary.createSummary !== 'boolean') reject();
    result.ancillary = { createSummary: value.ancillary.createSummary };
  }
  if (Object.hasOwn(value, 'references')) {
    if (!shape(value.references, ['showCommon', 'useGuideNames']) ||
        typeof value.references.showCommon !== 'boolean' || typeof value.references.useGuideNames !== 'boolean') reject();
    result.references = { showCommon: value.references.showCommon, useGuideNames: value.references.useGuideNames };
  }
  if (Object.hasOwn(value, 'constancy')) {
    if (value.constancy !== true || result.groupAndOrder !== 13 || result.display !== 1 ||
        !result.references || result.references.showCommon || result.references.useGuideNames) reject();
    result.constancy = true;
  }
  if (Object.hasOwn(value, 'unanimousCommon')) {
    if (value.unanimousCommon !== true || result.constancy || !result.references ||
        !summaryVegetationUnanimousCommonAvailable(result.groupAndOrder ?? 11, result.display,
          result.references.showCommon || result.references.useGuideNames)) reject();
    result.unanimousCommon = true;
  }
  if (result.references && !result.unanimousCommon && !summaryVegetationReferenceGroupingAvailable(result.groupAndOrder ?? 11,
    result.references.showCommon || result.references.useGuideNames)) reject();
  return result;
}
export function sameSummaryWorkbookOptions(left: SummaryVegetationWorkbookOptions,
  right: SummaryVegetationWorkbookOptions): boolean {
  return left.title === right.title && left.average === right.average && left.display === right.display &&
    left.showClass === right.showClass && left.includeEnv === right.includeEnv && left.groupAndOrder === right.groupAndOrder &&
    left.constancy === right.constancy && left.unanimousCommon === right.unanimousCommon &&
    (left.ancillary === undefined ? right.ancillary === undefined :
      !!left.ancillary && !!right.ancillary && left.ancillary.createSummary === right.ancillary.createSummary) &&
    (left.references === undefined ? right.references === undefined : !!left.references && !!right.references &&
      left.references.showCommon === right.references.showCommon && left.references.useGuideNames === right.references.useGuideNames);
}

async function ancillaryValue(value: unknown, options: SummaryVegetationWorkbookOptions,
  preview: SummaryVegetationPreview, validationYield: () => PromiseLike<void>,
  current: () => boolean): Promise<SummaryWorkbookAncillaryReview | null> {
  const selected = options.ancillary;
  if (!selected || !shape(value, ['createdAt', 'membershipRows', 'sheets', 'summary']) ||
      typeof value.createdAt !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value.createdAt) ||
      value.membershipRows !== preview.memberships.length ||
      preview.memberships.length > (selected.createSummary ? 1048572 : 1048575) ||
      !Array.isArray(value.sheets)) reject();
  const created = new Date(value.createdAt);
  if (!Number.isFinite(created.getTime()) || created.getUTCFullYear() < 100 ||
      created.toISOString().replace('.000Z', 'Z') !== value.createdAt) reject();
  const sheets = [preview.project, 'SU Table', ...(selected.createSummary ? ['ReportSummary'] : [])];
  if (value.sheets.length !== sheets.length || value.sheets.some((sheet, i) => sheet !== sheets[i]) ||
      new Set([...sheets, '_VPRO_Source'].map(workbookFoldedName)).size !== sheets.length + 1) reject();
  let summary: SummaryWorkbookReportSummary | null = null;
  if (selected.createSummary) {
    const raw = value.summary;
    if (!shape(raw, ['environmentRows', 'environmentCountAvailable', 'selectedPlotRows', 'version', 'versionStatus', 'units']) ||
        typeof raw.environmentRows !== 'number' || !Number.isSafeInteger(raw.environmentRows) || raw.environmentRows < 0 ||
        raw.environmentCountAvailable !== (raw.environmentRows !== 0) || !workbookText(raw.version) ||
        typeof raw.versionStatus !== 'string' ||
        !['metadata-unavailable', 'missing-definition', 'null', 'literal'].includes(raw.versionStatus) ||
        raw.versionStatus !== 'literal' && raw.version !== 'Unknown' ||
        !Array.isArray(raw.units) || raw.units.length > 32762) reject();
    let selectedRows = 0;
    for (let i = 0; i < preview.memberships.length; i++) {
      if (i % 256 === 0) { await validationYield(); if (!current()) return null; }
      if (preview.memberships[i].plotNumber.storage !== 'null') selectedRows++;
    }
    if (raw.selectedPlotRows !== selectedRows) reject();
    const units: SummaryWorkbookReportSummary['units'] = [], codes = new Set<string>();
    for (let i = 0; i < raw.units.length; i++) {
      if (i % 256 === 0) { await validationYield(); if (!current()) return null; }
      const unit = raw.units[i];
      if (!shape(unit, ['code', 'numPlots', 'longName']) || !workbookText(unit.code) || codes.has(unit.code) ||
          typeof unit.numPlots !== 'number' || !Number.isSafeInteger(unit.numPlots) || unit.numPlots <= 0 ||
          unit.longName !== null && !workbookText(unit.longName)) reject();
      codes.add(unit.code);
      units.push({ code: unit.code, numPlots: unit.numPlots, longName: unit.longName });
    }
    summary = { environmentRows: raw.environmentRows, environmentCountAvailable: raw.environmentCountAvailable,
      selectedPlotRows: selectedRows, version: raw.version, versionStatus: raw.versionStatus, units };
  } else if (value.summary !== null) reject();
  return { createdAt: value.createdAt, membershipRows: preview.memberships.length, sheets, summary };
}

export async function validateSummaryVegetationWorkbookReview(value: unknown, owner: SummaryVegetationOwner,
  requested: SummaryVegetationWorkbookOptions, validationYield: () => PromiseLike<void>,
  current: () => boolean = () => true): Promise<ValidatedSummaryVegetationWorkbookReview | null> {
  if (!current()) return null;
  const options = optionsValue(requested);
  const scope = options.constancy ? summaryVegetationConstancyScope : options.unanimousCommon ? summaryVegetationUnanimousCommonScope : options.groupAndOrder !== undefined
    ? options.ancillary ? summaryVegetationGroupedAncillaryScope :
      options.references ? options.groupAndOrder === 12 ? summaryVegetationStrataReferencesScope :
      summaryVegetationGroupedAttributesScope : summaryVegetationGroupedWorkbookScope : options.references
    ? options.ancillary ? summaryVegetationEffectiveAncillaryScope : summaryVegetationEffectiveReferencesScope
    : options.ancillary ? summaryVegetationAncillaryWorkbookScope : summaryVegetationWorkbookScope;
  if (!shape(value, ['preview', 'environment', 'options', 'scope', 'sheet', 'firstVegetationRow',
    'approvalHash', 'workbookSHA256', 'bytes', ...(options.ancillary ? ['ancillary'] : [])]) ||
      !sameSummaryWorkbookOptions(optionsValue(value.options), options) ||
      value.scope !== scope || value.sheet !== owner.project ||
      workbookWorksheetName(owner.project, 0) !== owner.project ||
      workbookFoldedName(owner.project) === workbookFoldedName('_vpro_source') ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) ||
      typeof value.bytes !== 'number' || !Number.isSafeInteger(value.bytes) || value.bytes <= 0 ||
      typeof value.firstVegetationRow !== 'number' || !Number.isSafeInteger(value.firstVegetationRow)) reject();
  const { approvalHash, workbookSHA256, bytes, sheet, firstVegetationRow } = value;
  const preview = await validateSummaryVegetationAsync(value.preview, owner,
    options.average, options.display, options.showClass, options.groupAndOrder ?? 11, {
      preview: () => Promise.reject(new Error('Workbook validation cannot dispatch a read')),
      cancel() {}, validationYield,
    }, current, options.references, options.constancy, options.unanimousCommon);
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
    scope,
  };
  if (options.ancillary) {
    const ancillary = await ancillaryValue(value.ancillary, options, preview, validationYield, current);
    if (!current() || ancillary === null) return null;
    result.ancillary = ancillary;
  }
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
  private state = { title: '', includeEnv: false, includeAncillary: false, createSummary: false,
    includeReferences: false, showCommon: false, useGuideNames: false,
    destination: '', draftError: '', error: '',
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
    const unanimousCommon = !calculation.constancy && this.state.includeReferences &&
      summaryVegetationUnanimousCommonAvailable(calculation.groupAndOrder ?? 11, calculation.display,
        this.state.showCommon || this.state.useGuideNames);
    return optionsValue({ average: calculation.average, display: calculation.display, showClass: calculation.showClass,
      ...(calculation.groupAndOrder !== undefined ? { groupAndOrder: calculation.groupAndOrder } : {}),
      ...(calculation.constancy !== undefined ? { constancy: calculation.constancy } : {}),
      ...(unanimousCommon ? { unanimousCommon: true } : {}),
      title: this.state.title, includeEnv: this.state.includeEnv,
      ...(this.state.includeAncillary ? { ancillary: { createSummary: this.state.createSummary } } : {}),
      ...(calculation.constancy ? { references: { showCommon: false, useGuideNames: false } } :
        this.state.includeReferences ? { references: {
        showCommon: this.state.showCommon, useGuideNames: this.state.useGuideNames } } : {}) });
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
  includeAncillary(include: boolean): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before editing.');
    this.state.includeAncillary = include; this.state.review = null; this.state.error = '';
    this.notify();
  }
  includeSummary(include: boolean): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before editing.');
    if (!this.state.includeAncillary) throw new Error('Select source worksheets before selecting ReportSummary.');
    this.state.createSummary = include; this.state.review = null; this.state.error = '';
    this.notify();
  }
  selectReferences(field: 'include' | 'common' | 'guide', selected: boolean): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before editing.');
    if (field !== 'include' && !this.state.includeReferences)
      throw new Error('Select effective source references before selecting common or guide names.');
    if (field === 'include') this.state.includeReferences = selected;
    else if (field === 'guide') {
      this.state.useGuideNames = selected;
      if (selected) this.state.showCommon = true;
    } else {
      if (!selected && this.state.useGuideNames) throw new Error('Guide names require common-name output; clear guide names first.');
      this.state.showCommon = selected;
    }
    this.state.review = null; this.state.error = ''; this.notify();
  }
  private validateDraft() {
    this.state.draftError = !workbookText(this.state.title)
      ? 'Title requires valid literal workbook text of at most 32767 UTF-16 units.'
      : !workbookText(this.state.destination) || this.state.destination !== '' && !/\.xlsx$/i.test(this.state.destination)
        ? 'Destination requires an explicit literal .xlsx path; no trimming or replacement.' : '';
  }
  undoDraft(): void {
    if (!this.editable()) throw new Error('Wait for review or acknowledge the workbook publication before Undo.');
    Object.assign(this.state, { title: '', includeEnv: false, includeAncillary: false, createSummary: false,
      includeReferences: false, showCommon: false, useGuideNames: false,
      destination: '', draftError: '', error: '', review: null });
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
    await this.publication.publish({ options, approvalHash: review.approvalHash, destination: this.state.destination,
      ...(review.ancillary ? { createdAt: review.ancillary.createdAt } : {}) },
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
