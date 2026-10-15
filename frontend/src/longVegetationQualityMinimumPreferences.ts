import type { ProjectMetadataTable } from '../bindings/github.com/boostao/vpro-wails';
import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { validateQualityMinimumReference } from './qualityMinimumReference';
import { wellFormedUTF16 } from './qualityEditor';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationQualityMinimumPreferenceValues {
  siteMinimum: string;
  vegMinimum: string;
  soilMinimum: string;
}
export interface LongVegetationQualityMinimumReference {
  source: string;
  definitions: ProjectMetadataTable;
}
export interface LongVegetationQualityMinimumPreferencesReview extends GoogleEarthScope, LongVegetationQualityMinimumReference {
  values: LongVegetationQualityMinimumPreferenceValues;
}
export const longVegetationQualityMinimumFields = [
  { key: 'siteMinimum', control: 'SitePlotQuality', label: 'Site >=' },
  { key: 'vegMinimum', control: 'VegPlotQuality', label: 'Veg >=' },
  { key: 'soilMinimum', control: 'SoilPlotQuality', label: 'Soil >=' },
] as const;
export const longVegetationQualityMinimumSource =
  'configured-owned-imported-SQLite VLists.USysTableOfLists; PlotQualitySite; ItemOrder then physical rowid; suggestions only';
const literal = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value);
const valid = (values: LongVegetationQualityMinimumPreferenceValues | null): values is LongVegetationQualityMinimumPreferenceValues =>
  !!values && longVegetationQualityMinimumFields.every(field => literal(values[field.key]));
const copy = (values: LongVegetationQualityMinimumPreferenceValues): LongVegetationQualityMinimumPreferenceValues => ({
  siteMinimum: values.siteMinimum, vegMinimum: values.vegMinimum, soilMinimum: values.soilMinimum,
});
function reference(value: LongVegetationQualityMinimumReference): LongVegetationQualityMinimumReference {
  if (value.source !== longVegetationQualityMinimumSource) {
    throw new Error('Owned quality minimum suggestion schema or provenance is incomplete; no copied catalogue applied.');
  }
  return { source: value.source, definitions: validateQualityMinimumReference(value.definitions, 'PlotQualitySite') };
}
export function validateLongVegetationQualityMinimumPreferences(value: LongVegetationQualityMinimumPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationQualityMinimumPreferencesReview {
  if (!value || !ownedScope(value, scope) || !valid(value.values)) {
    throw new Error('Saved Long Vegetation quality minimum strings are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: copy(value.values), ...reference(value) };
}
export class LongVegetationQualityMinimumPreferencesSession extends ReportPreferencesSession<
  LongVegetationQualityMinimumPreferenceValues, LongVegetationQualityMinimumPreferenceValues | null,
  LongVegetationQualityMinimumPreferencesReview> {
  private suggestions: LongVegetationQualityMinimumReference | null = null;
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: null, savedLabel: 'saved quality minimum strings',
      review: (value, owner) => validateLongVegetationQualityMinimumPreferences(value, owner).values,
      draft: copy,
      validate: (draft, saved) => !valid(draft) ||
        longVegetationQualityMinimumFields.some(field => draft[field.key] !== saved?.[field.key] && draft[field.key].includes('\0'))
        ? 'Changed quality minima require complete Unicode without NUL; correct them or Undo the draft.' : '',
      proposed: draft => {
        if (!valid(draft)) throw new Error('Load three complete quality minimum strings before Save.');
        return copy(draft);
      },
      equal: (a, b) => longVegetationQualityMinimumFields.every(field => a[field.key] === b[field.key]),
    });
  }
  override apply(value: LongVegetationQualityMinimumPreferencesReview | null) {
    const view = this.view();
    if (view.busy || view.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
    if (!value) throw new Error('Owned quality minimum review was not returned.');
    const next = reference(value);
    const previous = this.suggestions;
    this.suggestions = next;
    try {
      super.apply(value);
    } catch (error) {
      this.suggestions = previous;
      throw error;
    }
  }
  reference(): LongVegetationQualityMinimumReference | null { return structuredClone(this.suggestions); }
  editMinimum(key: keyof LongVegetationQualityMinimumPreferenceValues, value: string) {
    if (!longVegetationQualityMinimumFields.some(field => field.key === key)) throw new Error('Unsupported quality minimum field.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved quality minimum strings before editing.');
    this.editDraft({ ...draft, [key]: value });
  }
}
export const longVegetationQualityMinimumPreferencesSession = reportPreferencesRegistry(
  scope => new LongVegetationQualityMinimumPreferencesSession(scope));
