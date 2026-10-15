import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationSelectedFieldPreferenceValues { mode: number; selectedField: string }
export interface LongVegetationSelectedFieldDefinition { name: string; declaredType: string }
export interface LongVegetationSelectedFieldPreferencesReview extends GoogleEarthScope {
  values: LongVegetationSelectedFieldPreferenceValues;
  source: string;
  definitions: LongVegetationSelectedFieldDefinition[] | null;
}
export interface LongVegetationSelectedFieldDraft { mode: string; selectedField: string }
export const longVegetationSelectedFieldSuggestions = ['RedBlueList', 'Wetland_Ind', 'WeedStatus'] as const;
export const longVegetationSelectedFieldDefinitionsSource =
  'configured-owned-imported-SQLite VLists.USysAllSpecs physical columns; source Value List suggestions are not mandatory membership';
const mode = (value: unknown): value is 0 | 1 | 2 | 4 => value === 0 || value === 1 || value === 2 || value === 4;
function draftMode(value: unknown): 0 | 1 | 2 | 4 | null {
  switch (value) {
    case '0': return 0;
    case '1': return 1;
    case '2': return 2;
    case '4': return 4;
    default: return null;
  }
}
export function validateLongVegetationSelectedFieldPreferences(value: LongVegetationSelectedFieldPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationSelectedFieldPreferencesReview & { definitions: LongVegetationSelectedFieldDefinition[] } {
  if (!value || !ownedScope(value, scope) || !value.values || !mode(value.values.mode) ||
      typeof value.values.selectedField !== 'string' || !wellFormedUTF16(value.values.selectedField) ||
      value.source !== longVegetationSelectedFieldDefinitionsSource || !Array.isArray(value.definitions)) {
    throw new Error('Saved Long Vegetation selected field or physical definitions are missing, malformed or belong to another owned context; no defaults applied.');
  }
  const definitions = value.definitions.map(column => {
    if (!column || typeof column.name !== 'string' || !column.name || !wellFormedUTF16(column.name) ||
        column.name.includes('\0') || typeof column.declaredType !== 'string' ||
        !wellFormedUTF16(column.declaredType) || column.declaredType.includes('\0')) {
      throw new Error('Long Vegetation physical field metadata is malformed; no field identity or type was repaired.');
    }
    return { name: column.name, declaredType: column.declaredType };
  });
  return { ...scope, values: { mode: value.values.mode, selectedField: value.values.selectedField },
    source: value.source, definitions };
}
export class LongVegetationSelectedFieldPreferencesSession extends ReportPreferencesSession<
  LongVegetationSelectedFieldPreferenceValues, LongVegetationSelectedFieldDraft, LongVegetationSelectedFieldPreferencesReview> {
  private readonly columns: { value: LongVegetationSelectedFieldDefinition[] | null };
  constructor(scope: GoogleEarthScope) {
    const columns: { value: LongVegetationSelectedFieldDefinition[] | null } = { value: null };
    super(scope, {
      initialDraft: { mode: '', selectedField: '' }, savedLabel: 'saved selected field',
      review: (value, owner) => {
        const reviewed = validateLongVegetationSelectedFieldPreferences(value, owner);
        columns.value = reviewed.definitions;
        return reviewed.values;
      },
      draft: saved => ({ mode: String(saved.mode), selectedField: saved.selectedField }),
      validate: (draft, saved) => {
        const next = draftMode(draft.mode);
        if (next === null || typeof draft.selectedField !== 'string' || !wellFormedUTF16(draft.selectedField)) {
          return 'Selected field draft requires a complete literal Unicode field and source mode; correct it or Undo.';
        }
        if (next === saved?.mode && draft.selectedField === saved.selectedField) return '';
        if (draft.selectedField !== saved?.selectedField && draft.selectedField.includes('\0')) {
          return 'A newly assigned selected field must not contain NUL; correct it or Undo.';
        }
        if (draft.selectedField !== '' && next !== 4 ||
            draft.selectedField === '' && next !== saved?.mode && next !== 4) {
          return 'A nonempty selected field sets mode 4; clearing it retains the staged mode. Correct it or Undo.';
        }
        return '';
      },
      proposed: draft => {
        const next = draftMode(draft.mode);
        if (next === null) throw new Error('Selected field draft is not a supported source mode.');
        return { mode: next, selectedField: draft.selectedField };
      },
      equal: (a, b) => a.mode === b.mode && a.selectedField === b.selectedField,
    });
    this.columns = columns;
  }
  definitions(): LongVegetationSelectedFieldDefinition[] | null {
    return this.columns.value?.map(column => ({ ...column })) ?? null;
  }
  editField(raw: string) {
    const view = this.view();
    if (!view.saved) throw new Error('Load saved selected field before editing.');
    this.editDraft({ mode: raw !== '' ? '4' : view.draft.mode, selectedField: raw });
  }
}
export function createLongVegetationSelectedFieldPreferencesRegistry() {
  return reportPreferencesRegistry(scope => new LongVegetationSelectedFieldPreferencesSession(scope));
}
