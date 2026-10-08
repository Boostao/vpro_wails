import type { ProjectMetadataCell, VegetationWorkbookReview, VegetationWorkbookOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { validateLongVegetationPreview, type ValidatedLongVegetationPreview } from './longVegetationReport';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { lifeformShape as shape, lifeformText as text, lifeformCount as count, type LifeformSummaryOwner } from './lifeformSummary';
import { PublicationSession } from './publicationSession';

const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const cellKey = (cell: ProjectMetadataCell) => JSON.stringify([cell.storage, cell.text]);
const identity = (cell: ProjectMetadataCell) => shape(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) &&
  completeCell(cell) && (cell.storage === 'null' || cell.storage === 'text' && text(cell.text));
const ownerKey = (owner: LifeformSummaryOwner) => JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
function freeze(value: object): void {
  for (const child of Object.values(value)) if (child && typeof child === 'object') freeze(child);
  Object.freeze(value);
}

export type ValidatedVegetationWorkbookReview = Omit<VegetationWorkbookReview, 'preview' | 'sheets' | 'skippedUnits'> & {
  preview: ValidatedLongVegetationPreview;
  sheets: NonNullable<VegetationWorkbookReview['sheets']>;
  skippedUnits: NonNullable<VegetationWorkbookReview['skippedUnits']>;
};

function calendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || Number(value.slice(0, 4)) < 100) return false;
  const date = new Date(`${value}T00:00:00Z`);
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value;
}

function validateSummary(value: VegetationWorkbookReview, preview: ValidatedLongVegetationPreview): void {
  const summary = value.summary;
  if (!value.options.reportSummary) {
    if (summary !== null) throw new Error('Unrequested workbook summary was supplied.');
    return;
  }
  if (!shape(summary, ['createdDate', 'environmentRows', 'selectedPlotRows', 'speciesVersion', 'versionStatus', 'versionDefinitions']) ||
      summary.createdDate !== value.createdDate || !count(summary.environmentRows) || !count(summary.selectedPlotRows) ||
      !identity(summary.speciesVersion) ||
      !shape(summary.versionDefinitions, ['columns', 'rows'])) throw new Error('Workbook summary evidence is incomplete.');
  const { columns, rows } = summary.versionDefinitions;
  if (columns !== null && !Array.isArray(columns) || rows !== null && !Array.isArray(rows)) {
    throw new Error('Workbook version definitions have malformed nullable arrays.');
  }
  const definitions = rows ?? [], schema = columns ?? [];
  if (definitions.length > 1 || schema.some(column => !shape(column, ['name', 'declaredType']) ||
      !text(column.name) || !column.name || !text(column.declaredType)) ||
      new Set(schema.map(column => column.name.toLowerCase())).size !== schema.length) {
    throw new Error('Workbook version definitions are duplicated or malformed.');
  }
  const tableIndex = schema.findIndex(column => column.name === 'table_name');
  const descriptionIndex = schema.findIndex(column => column.name === 'description');
  if (schema.length && (tableIndex < 0 || descriptionIndex < 0)) throw new Error('Workbook version schema lacks original Description ownership.');
  const definition = definitions[0];
  let expectedStatus = schema.length ? 'missing-definition' : 'metadata-unavailable';
  let expectedVersion: ProjectMetadataCell = { storage: 'text', text: 'Unknown', integer: null, real: null, blobHex: null };
  if (definitions.length) {
    if (!definition) throw new Error('Workbook version definition is missing.');
    if (!schema.length || !shape(definition, ['rowId', 'cells']) || !exactSigned64(definition.rowId) ||
        !Array.isArray(definition.cells) || definition.cells.length !== schema.length ||
        definition.cells.some(cell => !shape(cell, ['storage', 'text', 'integer', 'real', 'blobHex']) || !completeCell(cell))) {
      throw new Error('Workbook version definition lost typed cells or physical identity.');
    }
    const table = definition.cells[tableIndex], version = definition.cells[descriptionIndex];
    if (!identity(table) || table.text !== 'USysAllSpecs' || !identity(version)) {
      throw new Error('Workbook version definition belongs to another table or unsupported storage.');
    }
    expectedStatus = version.storage === 'null' ? 'null' : 'literal';
    expectedVersion = version;
  }
  if (summary.versionStatus !== expectedStatus || cellKey(summary.speciesVersion) !== cellKey(expectedVersion) ||
      preview.report.quality && summary.selectedPlotRows !== preview.report.quality.occurrences.length) {
    throw new Error('Workbook summary contradicts original version/quality evidence.');
  }
}

export function validateVegetationWorkbookReview(value: VegetationWorkbookReview | null,
  owner: LifeformSummaryOwner): ValidatedVegetationWorkbookReview {
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['preview', 'options', 'summary', 'scope', 'createdDate', 'sheets', 'skippedUnits', 'approvalHash', 'workbookSHA256', 'bytes']) ||
      value.scope !== 'unlumped' || !text(value.createdDate) || !calendarDate(value.createdDate) ||
      !hash(value.approvalHash) || !hash(value.workbookSHA256) || !count(value.bytes) || value.bytes === 0 ||
      !Array.isArray(value.sheets) || !Array.isArray(value.skippedUnits) ||
      !shape(value.options, ['quickReport', 'spaceBetweenGroups', 'reportSummary']) ||
      typeof value.options.quickReport !== 'boolean' || typeof value.options.spaceBetweenGroups !== 'boolean' ||
      typeof value.options.reportSummary !== 'boolean' || owner.su === 'USysSuTableDynamic') {
    throw new Error('Workbook review has incomplete or unsupported unlumped source approval/layout.');
  }
  const preview = validateLongVegetationPreview(value.preview, owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath);
  validateSummary(value, preview);
  if (!preview.report.units.length || value.sheets.length !== preview.report.units.length) {
    throw new Error('Workbook review omits complete original unit worksheets.');
  }
  const names = new Set<string>(), units = new Set<string>();
  for (const [index, sheet] of value.sheets.entries()) {
    const unit = preview.report.units[index];
    let expected = unit.code.storage === 'null' ? 'Unassigned' : unit.code.text === '' ? `NoName${index}` : unit.code.text!;
    expected = expected.slice(0, 31).replace(/[:/\\[\]*?]/g, '-');
    // Go EqualFold includes these Unicode equivalents, unlike JS lowercasing alone.
    const folded = (name: string) => name.toLowerCase().replace(/ſ/g, 's').replace(/ς/g, 'σ');
    const name = text(sheet?.name) ? folded(sheet.name) : '';
    if (!shape(sheet, ['unit', 'name']) || !identity(sheet.unit) || cellKey(sheet.unit) !== cellKey(unit.code) ||
        sheet.name !== expected || !text(expected) || !name || names.has(name) || name === '_vpro_source' ||
        value.options.reportSummary && name === 'reportsummary' || sheet.name.startsWith("'") || sheet.name.endsWith("'") ||
        unit.numPlots > 250 || !unit.rows.length || !unit.rows[0].plots.length) {
      throw new Error('Workbook worksheet mapping differs from original unit/pivot evidence or collides.');
    }
    names.add(name); units.add(cellKey(sheet.unit));
  }
  for (const skipped of value.skippedUnits) {
    if (!shape(skipped, ['unit', 'reason']) || !identity(skipped.unit) || skipped.reason !== 'source NumPlots < 1' ||
        units.has(cellKey(skipped.unit))) throw new Error('Workbook skipped-unit evidence is malformed or repeated.');
    units.add(cellKey(skipped.unit));
  }
  const validated = { ...value, preview, sheets: value.sheets, skippedUnits: value.skippedUnits };
  freeze(validated);
  return validated;
}

export function validateVegetationWorkbookOutcome(value: VegetationWorkbookOutcome | null,
  destination: string, expectedHash: string): VegetationWorkbookOutcome {
  value = value ? structuredClone(value) : null;
  if (!shape(value, ['status', 'requestedDestination', 'path', 'sha256', 'errorMessage']) ||
      value.requestedDestination !== destination || !text(value.path) || !text(value.errorMessage) ||
      !['published', 'published-with-errors', 'not-published'].includes(value.status) ||
      value.sha256 !== '' && !hash(value.sha256)) throw new Error('Workbook acknowledgement incomplete; outcome unknown, never replay.');
  if (value.status !== 'not-published' && (!value.path || value.sha256 !== expectedHash) ||
      value.status === 'published' && value.errorMessage !== '' ||
      value.status !== 'published' && value.errorMessage === '') {
    throw new Error('Workbook acknowledgement contradicts reviewed bytes/outcome; never replay.');
  }
  return value;
}

export class VegetationWorkbookPublicationSession {
  private receipt = new PublicationSession<VegetationWorkbookOutcome>('Long Vegetation workbook');
  private consumed = new Set<string>();
  constructor(private readonly owner: LifeformSummaryOwner) { this.owner = structuredClone(owner); }
  view() { return this.receipt.view(); }
  subscribe(listener: () => void) { return this.receipt.subscribe(listener); }
  prepare(review: VegetationWorkbookReview) {
    if (this.view().busy || this.view().blocked) throw new Error('Acknowledge the prior workbook outcome before reviewing again.');
    const validated = validateVegetationWorkbookReview(review, this.owner);
    this.consumed.delete(validated.approvalHash);
    return validated;
  }
  async publish(review: VegetationWorkbookReview, destination: string,
    port: (request: { scope: 'unlumped'; createdDate: string; approvalHash: string; destination: string }) => PromiseLike<VegetationWorkbookOutcome | null>) {
    if (!text(destination) || !destination || !/\.xlsx$/i.test(destination)) {
      throw new Error('Workbook publication requires an explicit literal .xlsx destination.');
    }
    const validated = validateVegetationWorkbookReview(review, this.owner);
    if (this.view().busy || this.view().blocked) throw new Error('Acknowledge the prior workbook outcome before a new publication.');
    if (this.consumed.has(validated.approvalHash)) throw new Error('Obtain a fresh explicit workbook review before retrying.');
    this.consumed.add(validated.approvalHash);
    const request = { scope: 'unlumped' as const, createdDate: validated.createdDate, approvalHash: validated.approvalHash, destination };
    await this.receipt.publish(request, destination, port,
      value => validateVegetationWorkbookOutcome(value, destination, validated.workbookSHA256));
  }
  acknowledge() { this.receipt.acknowledge(); }
}

const sessions = new Map<string, { key: string; session: VegetationWorkbookPublicationSession }>();
export function vegetationWorkbookPublicationSession(owner: LifeformSummaryOwner): VegetationWorkbookPublicationSession {
  const key = ownerKey(owner), existing = sessions.get(owner.contextId);
  if (existing) {
    if (existing.key !== key) throw new Error('Workbook receipt belongs to different owned paths.');
    return existing.session;
  }
  const session = new VegetationWorkbookPublicationSession(owner);
  sessions.set(owner.contextId, { key, session });
  return session;
}
