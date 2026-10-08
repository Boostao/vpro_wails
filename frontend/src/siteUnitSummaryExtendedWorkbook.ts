import type {
  SiteUnitSummaryExtendedWorkbookOptions, SiteUnitSummaryExtendedWorkbookReview,
  SiteUnitSummaryWorkbookOutcome,
} from '../bindings/github.com/boostao/vpro-wails';
import type { GoogleEarthScope } from './googleEarthReview';
import { ownedScope } from './googleEarthReview';
import { lifeformShape as shape } from './lifeformSummary';
import { workbookText, sameWorkbookSource, freezeWorkbookReview } from './lifeformWorkbook';
import { validateSiteUnitSummaryWorkbookEvidence, validateSummaryWorkbookText } from './siteUnitSummaryWorkbook';
import { validateSummarySpecies, type ValidatedSummarySpecies } from './siteUnitSummarySpecies';
import type { ValidatedSiteUnitSummary } from './siteUnitSummary';
import { PublicationSession } from './publicationSession';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';

const optionKeys = ['method', 'orderBy', 'includeSpecies', 'coverCalculation', 'andOr',
  'presenceGreaterThan', 'coverGreaterThan'] as const;
export type ExtendedSummaryWorkbookOptions = SiteUnitSummaryExtendedWorkbookOptions;
export type ValidatedExtendedSummaryWorkbookReview = Omit<SiteUnitSummaryExtendedWorkbookReview, 'environment' | 'species' | 'sheets'> & {
  environment: ValidatedSiteUnitSummary;
  species: ValidatedSummarySpecies | null;
  sheets: NonNullable<SiteUnitSummaryExtendedWorkbookReview['sheets']>;
};
export type ExtendedSummaryWorkbookExportRequest = ExtendedSummaryWorkbookOptions & {
  approvalHash: string; destination: string;
};
function reject(): never {
  throw new Error('Extended Summary workbook review has inconsistent explicit options, species or owned Environment evidence; no repairs applied.');
}
export function validateExtendedSummaryWorkbookOptions(value: ExtendedSummaryWorkbookOptions): void {
  if (!shape(value, [...optionKeys]) || ![1, 2].includes(value.method) || ![1, 2].includes(value.orderBy) ||
      ![0, 1].includes(value.includeSpecies) || ![1, 2].includes(value.coverCalculation) || ![1, 2].includes(value.andOr) ||
      !Number.isSafeInteger(value.presenceGreaterThan) || value.presenceGreaterThan < -32768 || value.presenceGreaterThan > 32767 ||
      !Number.isSafeInteger(value.coverGreaterThan) || value.coverGreaterThan < -32768 || value.coverGreaterThan > 32767 ||
      (value.includeSpecies === 0 && (value.orderBy !== 2 || value.coverCalculation !== 1 || value.andOr !== 1 ||
        value.presenceGreaterThan !== 0 || value.coverGreaterThan !== 0))) reject();
}
export function sameExtendedSummaryWorkbookOptions(left: ExtendedSummaryWorkbookOptions | null,
  right: ExtendedSummaryWorkbookOptions | null): boolean {
  return !!left && !!right && optionKeys.every(key => left[key] === right[key]);
}
export function validateExtendedSummaryWorkbookReview(value: SiteUnitSummaryExtendedWorkbookReview | null,
  owner: GoogleEarthScope, requested: ExtendedSummaryWorkbookOptions): ValidatedExtendedSummaryWorkbookReview {
  validateExtendedSummaryWorkbookOptions(requested);
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['environment', 'species', 'scope', 'options', 'sheets', 'approvalHash', 'workbookSHA256', 'bytes'])) reject();
  validateExtendedSummaryWorkbookOptions(value.options);
  if (!sameExtendedSummaryWorkbookOptions(value.options, requested)) reject();
  validateSummaryWorkbookText(value);
  const evidence = validateSiteUnitSummaryWorkbookEvidence({
    preview: value.environment, scope: value.scope, sheets: value.sheets,
    approvalHash: value.approvalHash, workbookSHA256: value.workbookSHA256, bytes: value.bytes,
  }, owner, requested.method, requested.orderBy === 1 ? 'layer' : 'lifeform', true);
  let species: ValidatedSummarySpecies | null = null;
  if (requested.includeSpecies === 0) {
    if (value.species !== null) reject();
  } else {
    const raw = value.species;
    if (!raw || !shape(raw, ['environment', 'options', 'units']) ||
        !shape(raw.options, optionKeys.filter(key => key !== 'includeSpecies')) ||
        !Array.isArray(raw.units) || !sameWorkbookSource(raw.environment, value.environment)) reject();
    const { includeSpecies, ...speciesRequest } = requested;
    species = validateSummarySpecies(raw, owner, speciesRequest);
    for (const unit of species.units) {
      if (!shape(unit, ['code', 'nPlots', 'groups'])) reject();
      let rows = 4;
      for (const group of unit.groups) {
        if (!shape(group, ['index', 'caption', 'rows'])) reject();
        rows += 2 + group.rows.filter(row => row.included).length;
        for (const row of group.rows) {
          if (!shape(row, ['species', 'scientificName', 'englishName', 'codeType', 'cover', 'presence',
            'physicalValues', 'referenceRowIds', 'included']) ||
              !shape(row.scientificName, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
              !shape(row.englishName, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
              (row.codeType !== null && !shape(row.codeType, ['storage', 'text', 'integer', 'real', 'blobHex']))) reject();
        }
      }
      if (rows > 1048576) reject();
    }
  }
  return freezeWorkbookReview({ ...value, environment: evidence.preview, species, sheets: evidence.sheets });
}
export class ExtendedSummaryWorkbookPublicationSession {
  private receipt = new PublicationSession<SiteUnitSummaryWorkbookOutcome>('Extended Summary workbook');
  private requested: ExtendedSummaryWorkbookOptions | null = null;
  constructor(private readonly owner: GoogleEarthScope) { this.owner = freezeWorkbookReview(structuredClone(owner)); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  requestedOptions() { return this.requested ? structuredClone(this.requested) : null; }
  async publish(review: SiteUnitSummaryExtendedWorkbookReview, destination: string,
    port: (request: ExtendedSummaryWorkbookExportRequest) => PromiseLike<SiteUnitSummaryWorkbookOutcome | null>,
    requested: ExtendedSummaryWorkbookOptions) {
    if (!workbookText(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Workbook publication requires an explicit literal .xlsx destination; no trimming or replacement.');
    }
    const validated = validateExtendedSummaryWorkbookReview(review, this.owner, requested);
    if (this.receipt.view().busy || this.receipt.view().blocked) {
      throw new Error('Acknowledge the prior Extended Summary workbook outcome before a new publication.');
    }
    this.requested = freezeWorkbookReview(structuredClone(validated.options));
    await this.receipt.publish({ ...validated.options, approvalHash: validated.approvalHash, destination },
      destination, port, value => validateEnvironmentWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}
const sessions = new Map<string, { owner: GoogleEarthScope; session: ExtendedSummaryWorkbookPublicationSession }>();
export function extendedSummaryWorkbookPublicationSession(owner: GoogleEarthScope): ExtendedSummaryWorkbookPublicationSession {
  const previous = sessions.get(owner.contextId);
  if (previous) {
    if (!ownedScope(previous.owner, owner)) throw new Error('Extended Summary workbook receipt belongs to different owned paths.');
    return previous.session;
  }
  const session = new ExtendedSummaryWorkbookPublicationSession(owner);
  sessions.set(owner.contextId, { owner: structuredClone(owner), session }); return session;
}
