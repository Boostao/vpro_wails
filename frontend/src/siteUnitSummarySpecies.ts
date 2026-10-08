import type { SiteUnitSpeciesListPreview, SiteUnitSpeciesListRequest, SiteUnitSpeciesListUnit,
  SiteUnitSpeciesListGroup, SiteUnitSpeciesListRow } from '../bindings/github.com/boostao/vpro-wails';
import { completeCell, exactSigned64 } from './projectMetadataRestore';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { validateSiteUnitSummary, summaryLifeformCaptions, type SiteUnitSummaryMode, type ValidatedSiteUnitSummary } from './siteUnitSummary';

export function summarySpeciesCriterion(value: string, label: string): number {
  if (!/^[+-]?\d+$/.test(value) || !Number.isSafeInteger(Number(value)) || Number(value) < -32768 || Number(value) > 32767) {
    throw new Error(`${label} requires a complete source INTEGER (-32768 through 32767); no trimming or repair.`);
  }
  return Number(value);
}

interface CriteriaView {
  enabled: boolean;
  coverCalculation: number;
  andOr: number;
  presenceGreaterThan: string;
  coverGreaterThan: string;
  error: string;
}
const initial = (): CriteriaView => ({ enabled: false, coverCalculation: 1, andOr: 1,
  presenceGreaterThan: '0', coverGreaterThan: '0', error: '' });

export class SummarySpeciesCriteria {
  private state = initial();
  private listeners = new Set<() => void>();
  view(): CriteriaView { return structuredClone(this.state); }
  subscribe(listener: () => void) { this.listeners.add(listener); listener(); return () => { this.listeners.delete(listener); }; }
  private notify() { for (const listener of this.listeners) listener(); }
  private validate() {
    try {
      this.request(1, 'layer'); this.state.error = '';
    } catch (cause) { this.state.error = String(cause); }
    this.notify();
  }
  setEnabled(enabled: boolean) {
    if (this.state.error) throw new Error('Correct or reset the species criteria before changing its visibility.');
    this.state.enabled = enabled; this.notify();
  }
  editText(field: 'presenceGreaterThan' | 'coverGreaterThan', value: string) { this.state[field] = value; this.validate(); }
  editChoice(field: 'coverCalculation' | 'andOr', value: number) { this.state[field] = value; this.validate(); }
  reset() { const enabled = this.state.enabled; this.state = initial(); this.state.enabled = enabled; this.notify(); }
  request(method: number, mode: SiteUnitSummaryMode): SiteUnitSpeciesListRequest {
    if (![1, 2].includes(method) || ![1, 2].includes(this.state.coverCalculation) || ![1, 2].includes(this.state.andOr) ||
        !['layer', 'lifeform'].includes(mode)) throw new Error('Choose an available species calculation, comparison and grouping.');
    return { method, orderBy: mode === 'layer' ? 1 : 2, coverCalculation: this.state.coverCalculation, andOr: this.state.andOr,
      presenceGreaterThan: summarySpeciesCriterion(this.state.presenceGreaterThan, 'Presence threshold'),
      coverGreaterThan: summarySpeciesCriterion(this.state.coverGreaterThan, 'Cover threshold') };
  }
}
const sessions = new Map<string, { scope: GoogleEarthScope; session: SummarySpeciesCriteria }>();
export function summarySpeciesCriteria(scope: GoogleEarthScope): SummarySpeciesCriteria {
  const previous = sessions.get(scope.contextId);
  if (previous) {
    if (!ownedScope(previous.scope, scope)) throw new Error('Species criteria belong to different owned paths.');
    return previous.session;
  }
  const session = new SummarySpeciesCriteria();
  sessions.set(scope.contextId, { scope: structuredClone(scope), session }); return session;
}

type ValidatedSpeciesRow = Omit<SiteUnitSpeciesListRow, 'referenceRowIds'> & {
  referenceRowIds: NonNullable<SiteUnitSpeciesListRow['referenceRowIds']>;
};
type ValidatedSpeciesGroup = Omit<SiteUnitSpeciesListGroup, 'rows'> & { rows: ValidatedSpeciesRow[] };
type ValidatedSpeciesUnit = Omit<SiteUnitSpeciesListUnit, 'groups'> & { groups: ValidatedSpeciesGroup[] };
export type ValidatedSummarySpecies = Omit<SiteUnitSpeciesListPreview, 'environment' | 'units'> & {
  environment: ValidatedSiteUnitSummary; units: ValidatedSpeciesUnit[];
};
function hasSpeciesArrays(value: SiteUnitSpeciesListPreview): value is Omit<ValidatedSummarySpecies, 'environment'> & {
  environment: SiteUnitSpeciesListPreview['environment'];
} {
  return !!value && Array.isArray(value.units) && value.units.every(unit => !!unit && Array.isArray(unit.groups) &&
    unit.groups.every(group => !!group && Array.isArray(group.rows) &&
      group.rows.every(row => !!row && Array.isArray(row.referenceRowIds))));
}
const text = (value: unknown): value is string => typeof value === 'string' &&
  completeCell({ storage: 'text', text: value, integer: null, real: null, blobHex: null });
const nullableText = (value: SiteUnitSpeciesListRow['scientificName']) =>
  completeCell(value) && ['text', 'null'].includes(value.storage);
function formatted(value: unknown, decimals: number): value is string {
  return text(value) && new RegExp(`^-?(?:0|[1-9]\\d*)\\.\\d{${decimals}}$`).test(value) &&
    Number.isFinite(Number(value)) && value !== `-0.${'0'.repeat(decimals)}`;
}
function sourcePresence(count: number, plots: number): string {
  const [mantissa, exponent = '0'] = (count / plots * 100).toPrecision(15).split('e');
  const [integer, fraction = ''] = mantissa.split('.');
  const digits = BigInt(integer + fraction), power = Number(exponent) - fraction.length + 1;
  const denominator = power < 0 ? 10n ** BigInt(-power) : 1n;
  const numerator = power > 0 ? digits * 10n ** BigInt(power) : digits;
  const rounded = (numerator * 2n + denominator) / (denominator * 2n);
  return `${rounded / 10n}.${rounded % 10n}`;
}
export function validateSummarySpecies(value: SiteUnitSpeciesListPreview, owner: GoogleEarthScope,
  request: SiteUnitSpeciesListRequest): ValidatedSummarySpecies {
  if (!hasSpeciesArrays(value) || !value.options ||
      ![1, 2].includes(request.orderBy) || ![1, 2].includes(request.coverCalculation) || ![1, 2].includes(request.andOr) ||
      (['method', 'orderBy', 'coverCalculation', 'andOr', 'presenceGreaterThan', 'coverGreaterThan'] as const)
        .some(key => value.options[key] !== request[key]) ||
      !Number.isSafeInteger(request.presenceGreaterThan) || request.presenceGreaterThan < -32768 || request.presenceGreaterThan > 32767 ||
      !Number.isSafeInteger(request.coverGreaterThan) || request.coverGreaterThan < -32768 || request.coverGreaterThan > 32767) {
    throw new Error('Species preview arrays or explicit criteria are incomplete or different.');
  }
  const mode = request.orderBy === 1 ? 'layer' : 'lifeform';
  const environment = validateSiteUnitSummary(value.environment, owner.contextId, owner.project, owner.projectPath,
    owner.su, owner.suPath, request.method, mode);
  if (value.units.length !== environment.report.units.length) throw new Error('Species and Environment units differ.');
  for (let i = 0; i < value.units.length; i++) {
    const unit = value.units[i], source = environment.report.units[i];
    if (unit.code !== source.code || unit.nPlots !== source.plots.length) throw new Error('Species physical plot denominator differs.');
    let previous = request.orderBy === 1 ? 0 : -1;
    for (const group of unit.groups) {
      if (!Number.isInteger(group.index) || group.index <= previous || group.index > (request.orderBy === 1 ? 7 : 13) ||
          group.caption !== (request.orderBy === 1 ? `Layer ${group.index}` : summaryLifeformCaptions[group.index]) || group.rows.length === 0) {
        throw new Error('Species grouping/header differs from the source loop.');
      }
      previous = group.index;
      const rows = new Set<string>();
      for (const row of group.rows) {
        if (!text(row.species) || row.species.length > 8 || !nullableText(row.scientificName) || !nullableText(row.englishName) ||
            !formatted(row.cover, 2) || !formatted(row.presence, 1) || Number(row.presence) < 0 ||
            !Number.isSafeInteger(row.physicalValues) || row.physicalValues < 1 ||
            row.presence !== sourcePresence(row.physicalValues, unit.nPlots) || typeof row.included !== 'boolean' ||
            !row.referenceRowIds.length || row.referenceRowIds.some(id => !exactSigned64(id)) ||
            new Set(row.referenceRowIds).size !== row.referenceRowIds.length ||
            (request.orderBy === 1 ? row.codeType !== null : !row.codeType || !completeCell(row.codeType) ||
              row.codeType.storage !== 'text' || row.codeType.text?.toLowerCase() === 's')) {
          throw new Error('Species values, definitions or physical/reference provenance are incomplete.');
        }
        const identity = JSON.stringify([row.species, row.scientificName, row.englishName, row.codeType]);
        const cover = Number(row.cover) > request.coverGreaterThan, presence = Number(row.presence) > request.presenceGreaterThan;
        if (rows.has(identity) || row.included !== (request.andOr === 1 ? cover && presence : cover || presence)) {
          throw new Error('Species duplicate identity or formatted strict threshold decision differs.');
        }
        rows.add(identity);
      }
    }
  }
  return { ...value, environment };
}
