import type { SiteUnitSummaryWorkbookReview, SiteUnitSummaryWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import type { GoogleEarthScope } from './googleEarthReview';
import { validateSiteUnitSummary, type ValidatedSiteUnitSummary } from './siteUnitSummary';
import { completeCell } from './projectMetadataRestore';
import { lifeformShape as shape } from './lifeformSummary';
import { workbookWorksheetName, workbookFoldedName, workbookText, freezeWorkbookReview } from './lifeformWorkbook';
import { PublicationSession } from './publicationSession';
import { validateEnvironmentWorkbookOutcome } from './environmentWorkbook';

export type SiteUnitSummaryWorkbookOwner = GoogleEarthScope;
export type ValidatedSiteUnitSummaryWorkbookReview = Omit<SiteUnitSummaryWorkbookReview, 'preview' | 'sheets'> & {
  preview: ValidatedSiteUnitSummary;
  sheets: NonNullable<SiteUnitSummaryWorkbookReview['sheets']>;
};
export type SiteUnitSummaryWorkbookExportRequest = { method: number; approvalHash: string; destination: string };

export const siteUnitSummaryWorkbookFields = [
  ['Zone', 'Biogeoclimatic zone', 'category'],
  ['SubZone', 'Biogeoclimatic unit', 'bgc-unit'],
  ['Elevation', 'Elevation', 'numeric'],
  ['Aspect', 'Aspect', 'aspect'],
  ['SlopeGradient', 'Slope Gradient(%)', 'numeric'],
  ['MesoSlopePosition', 'Meso Slope Position', 'category'],
  ['MoistureRegime', 'Moisture Regime', 'moisture'],
  ['NutrientRegime', 'Nutrient Regime', 'category'],
  ['SiteDisturbance2', 'Site Disturbance 1', 'category'],
  ['SiteDisturbance2', 'Site Disturbance 2', 'category'],
  ['Exposure1', 'Exposure', 'category'],
  ['SurfaceTopographyType', 'Micro Topography Type', 'category'],
  ['SubstrateOrganicMatter', '%Substrate Org. Matter', 'numeric'],
  ['SubstrateRocks', '%Substrate Rocks', 'numeric'],
  ['SubstrateDecWood', '%Substrate Dec. Wood', 'numeric'],
  ['SubstrateMineralSoil', '%Substrate Mineral Soil', 'numeric'],
  ['SubstrateBedRock', '%Substrate Bedrock', 'numeric'],
  ['SubstrateWater', '%Substrate Water', 'numeric'],
  ['HydroGeoSystem', 'Hydr Geo System', 'category'],
  ['HydroGeoSubSystem', 'Hydro Geo Sub System', 'category'],
  ['StandAge', 'Stand Age', 'numeric'],
  ['SuccessionalStatus', 'Successional Status', 'category'],
  ['StructuralStage', 'Structure Stage', 'category'],
  ['StrataCoverTree', 'Tree Layer Cover', 'numeric'],
  ['StrataCoverShrub', 'Shrub Layer Cover', 'numeric'],
  ['StrataCoverHerb', 'Herb Layer Cover', 'numeric'],
  ['StrataCoverMoss', 'Moss LayerCover', 'numeric'],
  ['SoilClassGroup', 'Soil Great Group', 'category'],
  ['HumusForm', 'Humus Form', 'category'],
  ['HumusThickness', 'Humus Depth', 'numeric'],
  ['SoilDrainage', 'Soil Drainage', 'category'],
  ['SeepageDepth', 'Seepage(cm)', 'numeric'],
  ['SurficialMaterialSurf', 'Surficial Material', 'category'],
  ['RootZoneParticleSize', 'Root Zone Particle Size', 'category'],
  ['RootingDepth', 'Rooting Depth', 'numeric'],
  ['RootRestrictingType', 'Rooting Restricting Type', 'category'],
  ['BedrockGeology1', 'Bedrock Type 1', 'category'],
  ['BedrockGeology2', 'Bedrock Type 2', 'category'],
  ['BedrockGeology3', 'Bedrock Type 3', 'category'],
].map(([key, label, kind], i) => Object.freeze({
  source: i === 29 ? 'Admin' : 'Env', key, label, kind,
  section: i < 21 ? 'SITE' : i < 27 ? 'VEGETATION' : 'SOILS',
}));
Object.freeze(siteUnitSummaryWorkbookFields);

function reject(): never {
  throw new Error('Summary Environment workbook review has incomplete or inconsistent owned source, scope or worksheet evidence; no repairs applied.');
}
const hash = (value: unknown) => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const ownerKey = (owner: GoogleEarthScope) => JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
function sourceSafe(value: unknown): void {
  if (typeof value === 'string') { if (!workbookText(value)) reject(); }
  else if (value && typeof value === 'object') for (const child of Object.values(value)) sourceSafe(child);
}
function cellShape(value: unknown): boolean {
  return shape(value, ['storage', 'text', 'integer', 'real', 'blobHex']);
}
function sourceGreater(left: string, right: string): boolean {
  // Go's literal UTF-8 ordering follows scalar values, not JavaScript's UTF-16 code units.
  const a = Array.from(left, c => c.codePointAt(0)!), b = Array.from(right, c => c.codePointAt(0)!);
  for (let i = 0; i < Math.min(a.length, b.length); i++) if (a[i] !== b[i]) return a[i] > b[i];
  return a.length > b.length;
}
function ordered(keys: string[][]): boolean {
  return keys.every((key, i) => i === 0 || key.some((part, j) =>
    sourceGreater(part, keys[i - 1][j]) && key.slice(0, j).every((earlier, k) => earlier === keys[i - 1][k])));
}

export function validateSiteUnitSummaryWorkbookReview(value: SiteUnitSummaryWorkbookReview | null,
  owner: GoogleEarthScope, requestedMethod: number): ValidatedSiteUnitSummaryWorkbookReview {
  value = value ? structuredClone(value) : null;
  if (![owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath].every(v => workbookText(v) && v !== '') ||
      owner.su === 'None' || owner.su === 'USysSuTableDynamic' ||
      !shape(value, ['preview', 'scope', 'sheets', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) ||
      !Number.isSafeInteger(value.bytes) || value.bytes <= 0 ||
      !shape(value.scope, ['savedMethod', 'siteUnitType', 'orderBy', 'includeSpecies']) ||
      ![1, 2].includes(value.scope.savedMethod) || value.scope.siteUnitType !== 1 ||
      ![1, 3].includes(value.scope.orderBy) || value.scope.includeSpecies !== 0 ||
      !shape(value.preview, ['contextId', 'projectPath', 'suPath', 'report']) ||
      !shape(value.preview.report, ['project', 'su', 'method', 'querySource', 'fields', 'memberships', 'units']) ||
      !Array.isArray(value.sheets)) reject();
  sourceSafe(value);
  const preview = validateSiteUnitSummary(value.preview, owner.contextId, owner.project,
    owner.projectPath, owner.su, owner.suPath, requestedMethod);
  const report = preview.report;
  if (!report.units.length || !report.memberships.length || value.sheets.length !== report.units.length) reject();
  for (const [i, field] of report.fields.entries()) {
    const expected = siteUnitSummaryWorkbookFields[i];
    if (!shape(field, ['source', 'key', 'label', 'section', 'kind']) ||
        Object.entries(expected).some(([key, item]) => field[key] !== item)) reject();
  }
  if (!ordered(report.memberships.map(m => [m.rowId])) ||
      !report.units.every((unit, i) => i === 0 || sourceGreater(report.units[i - 1].code, unit.code))) reject();
  const envOwners = new Map<string, string>(), adminOwners = new Map<string, string>();
  const nameOwners = new Set<string>(), sheetNames = new Set<string>(), joinedPlots = new Set<string>();
  for (const member of report.memberships) {
    if (!shape(member, ['rowId', 'plotNumber', 'siteUnit', 'joinedRows', 'status']) ||
        !cellShape(member.plotNumber) || !cellShape(member.siteUnit)) reject();
  }
  for (const [i, unit] of report.units.entries()) {
    if (!shape(unit, ['code', 'longName', 'nameStatus', 'nameCandidates', 'plots', 'values']) ||
        !['unique', 'missing'].includes(unit.nameStatus) || unit.values[8] !== unit.values[9] ||
        !ordered(unit.nameCandidates.map(n => [n.rowId])) ||
        !ordered(unit.plots.map(p => [p.plotNumber, p.suRowId, p.envRowId, p.adminRowId]))) reject();
    for (const name of unit.nameCandidates) {
      if (!shape(name, ['rowId', 'value']) || !cellShape(name.value) || nameOwners.has(name.rowId)) reject();
      nameOwners.add(name.rowId);
    }
    for (const plot of unit.plots) {
      if (!shape(plot, ['plotNumber', 'suRowId', 'envRowId', 'adminRowId'])) reject();
      for (const [id, owners] of [[plot.envRowId, envOwners], [plot.adminRowId, adminOwners]] as const) {
        if (owners.has(id) && owners.get(id) !== plot.plotNumber) reject();
        owners.set(id, plot.plotNumber);
      }
      joinedPlots.add(plot.plotNumber);
    }
    const sheet = value.sheets[i], expectedName = workbookWorksheetName(unit.code, i);
    if (!shape(sheet, ['unit', 'name']) || !cellShape(sheet.unit) || !completeCell(sheet.unit) ||
        sheet.unit.storage !== 'text' || sheet.unit.text !== unit.code || sheet.name !== expectedName ||
        sheetNames.has(workbookFoldedName(sheet.name))) reject();
    sheetNames.add(workbookFoldedName(sheet.name));
  }
  const envCounts = new Map<string, number>(), adminCounts = new Map<string, number>();
  for (const plot of envOwners.values()) envCounts.set(plot, (envCounts.get(plot) ?? 0) + 1);
  for (const plot of adminOwners.values()) adminCounts.set(plot, (adminCounts.get(plot) ?? 0) + 1);
  for (const member of report.memberships) {
    const plot = member.plotNumber.text;
    if (plot !== null && member.siteUnit.text !== null && member.status !== 'joined' && joinedPlots.has(plot)) reject();
    if (member.status === 'joined') {
      const envCount = envCounts.get(plot!) ?? 0, adminCount = adminCounts.get(plot!) ?? 0;
      if (member.joinedRows !== envCount * adminCount) reject();
    }
  }
  return freezeWorkbookReview({ ...value, preview, sheets: value.sheets });
}

export class SiteUnitSummaryWorkbookPublicationSession {
  private receipt = new PublicationSession<SiteUnitSummaryWorkbookOutcome>('Summary Environment workbook');
  constructor(private readonly owner: GoogleEarthScope) { this.owner = freezeWorkbookReview(structuredClone(owner)); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  async publish(review: SiteUnitSummaryWorkbookReview, destination: string,
    port: (request: SiteUnitSummaryWorkbookExportRequest) => PromiseLike<SiteUnitSummaryWorkbookOutcome | null>,
    requestedMethod: number) {
    if (!workbookText(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Workbook publication requires an explicit literal .xlsx destination; no trimming or replacement.');
    }
    const validated = validateSiteUnitSummaryWorkbookReview(review, this.owner, requestedMethod);
    await this.receipt.publish({ method: requestedMethod, approvalHash: validated.approvalHash, destination },
      destination, port, value => validateEnvironmentWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}
const sessions = new Map<string, { key: string; session: SiteUnitSummaryWorkbookPublicationSession }>();
export function siteUnitSummaryWorkbookPublicationSession(owner: GoogleEarthScope): SiteUnitSummaryWorkbookPublicationSession {
  const key = ownerKey(owner), existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key) throw new Error('Summary Environment workbook receipt belongs to different owned paths.');
    return existing.session;
  }
  const session = new SiteUnitSummaryWorkbookPublicationSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}
