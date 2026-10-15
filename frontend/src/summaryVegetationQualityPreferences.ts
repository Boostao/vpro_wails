import type { ProjectMetadataTable } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { lifeformShape } from './lifeformSummary';
import { wellFormedUTF16 } from './qualityEditor';
import { validateQualityMinimumReference } from './qualityMinimumReference';
import { ReportPreferencesSession, reportPreferencesRegistry, type ReportPreferenceRequest, type ReportTitlePreferencesOutcome } from './reportTitlePreferences';

export interface SummaryVegetationQualityPreferenceValues {
  enforce: boolean;
  siteMinimum: string;
  vegMinimum: string;
  soilMinimum: string;
  becMinimum: string;
  siteIncludeNulls: boolean;
  vegIncludeNulls: boolean;
  soilIncludeNulls: boolean;
  becIncludeNulls: boolean;
}
export interface SummaryVegetationQualityReference {
  source: string;
  qualityDefinitions: ProjectMetadataTable;
  becDefinitions: ProjectMetadataTable;
}
export interface SummaryVegetationQualityPreferencesReview extends GoogleEarthScope, SummaryVegetationQualityReference {
  values: SummaryVegetationQualityPreferenceValues;
}
export const summaryVegetationQualityMinimumFields = [
  { key: 'siteMinimum', control: 'SitePlotQuality', label: 'Site >=' },
  { key: 'vegMinimum', control: 'VegPlotQuality', label: 'Veg >=' },
  { key: 'soilMinimum', control: 'SoilPlotQuality', label: 'Soil >=' },
  { key: 'becMinimum', control: 'BEC_Use', label: 'BEC Use >=' },
] as const;
export const summaryVegetationQualityNullFields = [
  { key: 'siteIncludeNulls', control: 'optSiteQualityNull', label: 'Site' },
  { key: 'vegIncludeNulls', control: 'optVegQualityNull', label: 'Veg' },
  { key: 'soilIncludeNulls', control: 'optSoilQualityNull', label: 'Soil' },
  { key: 'becIncludeNulls', control: 'optBECUseNull', label: 'BEC Use' },
] as const;
type MinimumField = typeof summaryVegetationQualityMinimumFields[number]['key'];
type NullField = typeof summaryVegetationQualityNullFields[number]['key'];
const keys = ['enforce', ...summaryVegetationQualityMinimumFields.map(field => field.key),
  ...summaryVegetationQualityNullFields.map(field => field.key)];
export const summaryVegetationQualitySource =
  'configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite and BEC_Use; ItemOrder then physical rowid; suggestions only, not DataQuality qualification';
function valid(value: unknown): value is SummaryVegetationQualityPreferenceValues {
  return lifeformShape(value, keys) && typeof value.enforce === 'boolean' &&
    summaryVegetationQualityMinimumFields.every(field => {
      const raw = value[field.key];
      return typeof raw === 'string' && wellFormedUTF16(raw);
    }) &&
    summaryVegetationQualityNullFields.every(field => typeof value[field.key] === 'boolean');
}
function copy(value: SummaryVegetationQualityPreferenceValues): SummaryVegetationQualityPreferenceValues {
  return { enforce: value.enforce, siteMinimum: value.siteMinimum, vegMinimum: value.vegMinimum,
    soilMinimum: value.soilMinimum, becMinimum: value.becMinimum, siteIncludeNulls: value.siteIncludeNulls,
    vegIncludeNulls: value.vegIncludeNulls, soilIncludeNulls: value.soilIncludeNulls, becIncludeNulls: value.becIncludeNulls };
}
function equal(a: SummaryVegetationQualityPreferenceValues, b: SummaryVegetationQualityPreferenceValues): boolean {
  return a.enforce === b.enforce &&
    summaryVegetationQualityMinimumFields.every(field => a[field.key] === b[field.key]) &&
    summaryVegetationQualityNullFields.every(field => a[field.key] === b[field.key]);
}
export function validateSummaryVegetationQualityPreferences(value: unknown, scope: GoogleEarthScope): SummaryVegetationQualityPreferencesReview {
  if (!lifeformShape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'values', 'source', 'qualityDefinitions', 'becDefinitions']) ||
      typeof value.contextId !== 'string' || typeof value.project !== 'string' || typeof value.projectPath !== 'string' ||
      typeof value.su !== 'string' || typeof value.suPath !== 'string' ||
      !ownedScope({ contextId: value.contextId, project: value.project, projectPath: value.projectPath, su: value.su, suPath: value.suPath }, scope) ||
      !valid(value.values) || value.source !== summaryVegetationQualitySource) {
    throw new Error('Saved Summary quality options are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: copy(value.values), source: value.source,
    qualityDefinitions: validateQualityMinimumReference(value.qualityDefinitions, 'PlotQualitySite'),
    becDefinitions: validateQualityMinimumReference(value.becDefinitions, 'BEC_Use') };
}
export class SummaryVegetationQualityPreferencesSession extends ReportPreferencesSession<
  SummaryVegetationQualityPreferenceValues, SummaryVegetationQualityPreferenceValues | null, SummaryVegetationQualityPreferencesReview> {
  private suggestions: SummaryVegetationQualityReference | null = null;
  private readError = '';
  private readonly ownerScope: GoogleEarthScope;
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: null, savedLabel: 'saved Summary quality options',
      review: (value, owner) => validateSummaryVegetationQualityPreferences(value, owner).values,
      draft: copy,
      validate: (draft, saved) => {
        if (!valid(draft)) return 'Correct all four quality minima and Include Nulls flags or Undo the draft.';
        if (!saved || draft.enforce !== saved.enforce)
          return 'Summary quality enforcement invokes mutating source Form_Load; changes remain unavailable.';
        if (summaryVegetationQualityMinimumFields.some(field => draft[field.key] !== saved[field.key] && draft[field.key].includes('\0')))
          return 'Changed quality minima require complete Unicode without NUL; correct them or Undo the draft.';
        return '';
      },
      proposed: draft => {
        if (!valid(draft)) throw new Error('Load complete Summary quality options before Save.');
        return copy(draft);
      },
      equal,
    });
    this.ownerScope = structuredClone(scope);
  }
  override view() { return { ...super.view(), readError: this.readError }; }
  rejectRead(cause: unknown) {
    this.readError = `Saved Summary quality options unavailable; no defaults applied: ${String(cause)}`;
    this.notify();
  }
  override apply(value: SummaryVegetationQualityPreferencesReview | null, preserveDraft = false) {
    const view = this.view();
    if (view.busy || view.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
    const reviewed = validateSummaryVegetationQualityPreferences(value, this.ownerScope);
    const previousSuggestions = this.suggestions, previousError = this.readError;
    this.suggestions = { source: reviewed.source, qualityDefinitions: reviewed.qualityDefinitions, becDefinitions: reviewed.becDefinitions };
    this.readError = '';
    try {
      super.apply(reviewed, preserveDraft);
    } catch (error) {
      this.suggestions = previousSuggestions; this.readError = previousError;
      throw error;
    }
  }
  reference(): SummaryVegetationQualityReference | null { return structuredClone(this.suggestions); }
  override async save(port: (request: ReportPreferenceRequest<SummaryVegetationQualityPreferenceValues>) =>
    PromiseLike<ReportTitlePreferencesOutcome | null>): Promise<void> {
    if (this.readError) throw new Error('Explicitly Load saved Summary quality options successfully before Save; the last read failed.');
    await super.save(port);
  }
  editMinimum(key: MinimumField, raw: string) {
    if (!summaryVegetationQualityMinimumFields.some(field => field.key === key)) throw new Error('Unsupported Summary quality minimum field.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved Summary quality options before editing.');
    this.editDraft({ ...draft, [key]: raw });
  }
  editNullFlag(key: NullField, raw: boolean) {
    if (!summaryVegetationQualityNullFields.some(field => field.key === key)) throw new Error('Unsupported Summary quality Include Nulls field.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved Summary quality options before editing.');
    this.editDraft({ ...draft, [key]: raw });
  }
}
export const createSummaryVegetationQualityPreferencesRegistry = () =>
  reportPreferencesRegistry(scope => new SummaryVegetationQualityPreferencesSession(scope));
