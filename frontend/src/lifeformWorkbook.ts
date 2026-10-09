import type { LifeformWorkbookReview, LifeformWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import {
  validateLifeformSummary, lifeformShape as shape, lifeformText as text,
  type LifeformSummaryOwner, type LifeformSummaryPreview,
} from './lifeformSummary';
import { validateSpeciesAttributeSummary, type SpeciesAttributeSummaryPreview } from './speciesAttributeSummary';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';
import { PublicationSession } from './publicationSession';

export const lifeformWorkbookOptions = [
  { field: 'SppAttributeSRank', label: 'SRank detail' },
  { field: 'SppAttributeWetland', label: 'Wetland indicator detail' },
  { field: 'SppAttributeWeedStatus', label: 'Weed status detail' },
  { field: 'SppAttributeRedBlue', label: 'Red Blue list detail' },
  { field: 'SppAttributeASMR', label: 'ASMR detail' },
  { field: 'SppAttributeClimate', label: 'Climate detail' },
] as const;

export type ValidatedLifeformWorkbookReview = Omit<LifeformWorkbookReview, 'lifeform' | 'attributes' | 'details' | 'sheets'> & {
  lifeform: LifeformSummaryPreview;
  attributes: SpeciesAttributeSummaryPreview;
  details: boolean[];
  sheets: NonNullable<LifeformWorkbookReview['sheets']>;
};
export type LifeformWorkbookExportRequest = { details: boolean[]; approvalHash: string; destination: string };
const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
function same(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (!a || !b || typeof a !== 'object' || typeof b !== 'object' || Array.isArray(a) !== Array.isArray(b)) return false;
  const left = Object.entries(a), right = Object.entries(b);
  return left.length === right.length && left.every(([key, value]) => Object.hasOwn(b, key) &&
    same(value, right.find(([other]) => other === key)?.[1]));
}
const cellKey = (cell: { storage: string; text: string | null }) => JSON.stringify([cell.storage, cell.text]);
const ownerKey = (owner: LifeformSummaryOwner) => JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
function reject(): never { throw new Error('Lifeform workbook review has incomplete or inconsistent source approval, units or worksheet mappings; no repairs applied.'); }
function workbookText(value: unknown): value is string {
  return text(value) && value.length <= 32767 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\ufffe\uffff]/u.test(value);
}
function sourceText(value: unknown): void {
  if (typeof value === 'string') { if (!workbookText(value)) reject(); }
  else if (value && typeof value === 'object') for (const child of Object.values(value)) sourceText(child);
}
function freeze<T>(value: T): T {
  if (value && typeof value === 'object') {
    for (const child of Object.values(value)) freeze(child);
    Object.freeze(value);
  }
  return value;
}
function detailsValid(value: unknown): value is boolean[] {
  return Array.isArray(value) && value.length === 6 && Array.from(value).every(item => typeof item === 'boolean');
}
function worksheetName(code: string | null, index: number): string {
  let name = code === null || code === '' ? `NoName${index}` : code;
  if (!workbookText(name)) reject();
  if (name.length > 31) {
    const boundary = name.charCodeAt(30);
    if (boundary >= 0xd800 && boundary <= 0xdbff) reject();
    name = name.slice(0, 31);
  }
  name = name.replace(/[:/\\[\]*?]/g, '-');
  if (name.startsWith("'") || name.endsWith("'") || name.toLowerCase() === '_vpro_source') reject();
  return name;
}
// Match simple Unicode folding, not full folds such as sharp s → "ss".
const foldedName = (name: string) => [...name].map(rune => {
  if (rune === '\u0130' || rune === '\u0131') return rune;
  const upper = rune.toUpperCase();
  return ([...upper].length === 1 ? upper : rune).toLowerCase();
}).join('');

export function validateLifeformWorkbookReview(value: LifeformWorkbookReview | null,
  owner: LifeformSummaryOwner, requestedDetails: boolean[]): ValidatedLifeformWorkbookReview {
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['lifeform', 'attributes', 'details', 'sheets', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      !detailsValid(requestedDetails) || !detailsValid(value.details) || !same(value.details, requestedDetails) ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) ||
      typeof value.bytes !== 'number' || !Number.isSafeInteger(value.bytes) || value.bytes <= 0 ||
      !Array.isArray(value.sheets)) reject();
  const lifeform = validateLifeformSummary(value.lifeform, owner);
  const attributes = validateSpeciesAttributeSummary(value.attributes, owner);
  sourceText(lifeform); sourceText(attributes);
  const lf = lifeform.report, at = attributes.report;
  if (!lf.units.length || !lf.catalogue.length || !same(lf.memberships, at.memberships) ||
      lf.units.length !== at.units.length || value.sheets.length !== lf.units.length) reject();
  const groups = new Map<string, string[]>();
  for (const member of lf.memberships) {
    const key = cellKey(member.siteUnit);
    const ids = groups.get(key) ?? [];
    ids.push(member.rowId); groups.set(key, ids);
  }
  const groupOrder = [...groups.keys()], names = new Set<string>();
  const boundedCount = (n: number | null) => n === null || n <= 999999999999999;
  for (const [i, unit] of lf.units.entries()) {
    const attributeUnit = at.units[i], sheet = value.sheets[i];
    if (cellKey(unit.code) !== groupOrder[i] || !same(unit.suRowIds, groups.get(groupOrder[i])) ||
        !same(unit.code, attributeUnit.code) || !same(unit.suRowIds, attributeUnit.suRowIds) ||
        unit.nPlots !== attributeUnit.nPlots || !boundedCount(unit.nPlots) ||
        !boundedCount(unit.uniqueSpecies) || !boundedCount(unit.occurrences) ||
        unit.rows.length > 1048576 - 18 ||
        unit.rows.some(row => !boundedCount(row.plotGroups) || !boundedCount(row.coverCount)) ||
        attributeUnit.rows.some(row => !boundedCount(row.count) || !boundedCount(row.plotOccurrences) ||
          row.categories.some(n => !boundedCount(n)))) reject();
    const expected = worksheetName(unit.code.text, i);
    if (!shape(sheet, ['unit', 'name']) || !same(sheet.unit, unit.code) ||
        !text(sheet.name) || sheet.name !== expected || names.has(foldedName(sheet.name))) reject();
    names.add(foldedName(sheet.name));
  }
  return freeze({ ...value, lifeform, attributes, details: value.details, sheets: value.sheets });
}

export class LifeformWorkbookPublicationSession {
  private receipt = new PublicationSession<LifeformWorkbookOutcome>('Lifeform Summary workbook');
  constructor(private readonly owner: LifeformSummaryOwner) { this.owner = freeze(structuredClone(owner)); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  async publish(review: LifeformWorkbookReview, destination: string,
    port: (request: LifeformWorkbookExportRequest) => PromiseLike<LifeformWorkbookOutcome | null>) {
    if (!text(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Workbook publication requires an explicit literal .xlsx destination; no trimming or replacement.');
    }
    const validated = validateLifeformWorkbookReview(review, this.owner, review.details ?? []);
    await this.receipt.publish({ details: [...validated.details], approvalHash: validated.approvalHash, destination },
      destination, port, value => validateEnvironmentWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { key: string; session: LifeformWorkbookPublicationSession }>();
export function lifeformWorkbookPublicationSession(owner: LifeformSummaryOwner): LifeformWorkbookPublicationSession {
  const key = ownerKey(owner), existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key) throw new Error('Lifeform workbook receipt belongs to different owned paths.');
    return existing.session;
  }

  const session = new LifeformWorkbookPublicationSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}

export { worksheetName as workbookWorksheetName, foldedName as workbookFoldedName, workbookText,
  freeze as freezeWorkbookReview, same as sameWorkbookSource };
