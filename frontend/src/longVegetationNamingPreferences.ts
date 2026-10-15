import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationNamingPreferenceValues { mode: number; selectedField: string }
export interface LongVegetationNamingAvailability { none: boolean; english: boolean; code: boolean; selectedField: boolean }
export interface LongVegetationNamingClientAvailability { code: boolean }
export interface LongVegetationNamingPreferencesReview extends GoogleEarthScope {
  values: LongVegetationNamingPreferenceValues;
  availability: LongVegetationNamingAvailability;
}
export interface LongVegetationNamingDraft { mode: string; selectedField: string }
export const longVegetationNamingChoices = [
  { value: 1, label: 'Show english name', capability: 'english' },
  { value: 2, label: 'Show 8 char. code', capability: 'code' },
  { value: 0, label: 'Show none', capability: 'none' },
] as const;
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
export function validateLongVegetationNamingPreferences(value: LongVegetationNamingPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationNamingPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !mode(value.values.mode) ||
      typeof value.values.selectedField !== 'string' || !wellFormedUTF16(value.values.selectedField) ||
      !value.availability || value.availability.none !== true || value.availability.english !== true ||
      typeof value.availability.code !== 'boolean' || value.availability.selectedField !== false) {
    throw new Error('Saved Long Vegetation Show values or availability are missing, unsupported or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: { mode: value.values.mode, selectedField: value.values.selectedField },
    availability: { none: true, english: true, code: value.availability.code, selectedField: false } };
}
export class LongVegetationNamingPreferencesSession extends ReportPreferencesSession<
  LongVegetationNamingPreferenceValues, LongVegetationNamingDraft, LongVegetationNamingPreferencesReview> {
  private readonly capabilities: { value: LongVegetationNamingAvailability | null };
  constructor(scope: GoogleEarthScope, client: LongVegetationNamingClientAvailability) {
    if (typeof client?.code !== 'boolean') throw new Error('Explicit immutable client naming availability is required.');
    const frontendCode = client.code;
    const capabilities: { value: LongVegetationNamingAvailability | null } = { value: null };
    super(scope, {
      initialDraft: { mode: '', selectedField: '' }, savedLabel: 'saved Show values',
      review: (value, owner) => {
        const reviewed = validateLongVegetationNamingPreferences(value, owner);
        capabilities.value = { ...reviewed.availability, code: reviewed.availability.code && frontendCode };
        return reviewed.values;
      },
      draft: saved => ({ mode: String(saved.mode), selectedField: saved.selectedField }),
      validate: (draft, saved) => {
        const next = draftMode(draft.mode);
        if (next === null || typeof draft.selectedField !== 'string' || !wellFormedUTF16(draft.selectedField)) {
          return 'Choose a supported Show value, or Undo the preference draft.';
        }
        if (next === saved?.mode && draft.selectedField === saved.selectedField) return '';
        if (draft.selectedField !== '') return 'A Show selection must clear the paired selected field; Undo this draft.';
        const selected = longVegetationNamingChoices.find(choice => choice.value === next);
        return selected && capabilities.value?.[selected.capability] ? '' :
          'This Show choice is unavailable in this build/session; correct it or Undo. Saved values are not reset.';
      },
      proposed: draft => {
        const next = draftMode(draft.mode);
        if (next === null) throw new Error('Show draft is not a supported source mode.');
        return { mode: next, selectedField: draft.selectedField };
      },
      equal: (a, b) => a.mode === b.mode && a.selectedField === b.selectedField,
    });
    this.capabilities = capabilities;
  }
  availability(): LongVegetationNamingAvailability | null {
    return this.capabilities.value ? { ...this.capabilities.value } : null;
  }
  editMode(raw: string) {
    const view = this.view();
    if (!view.saved) throw new Error('Load saved Show values before editing.');
    const selectedField = raw === '0' || raw === '1' || raw === '2' ? '' : view.draft.selectedField;
    this.editDraft({ mode: raw, selectedField });
  }
}
export function createLongVegetationNamingPreferencesRegistry(client: LongVegetationNamingClientAvailability) {
  const availability = { code: client.code };
  return reportPreferencesRegistry(scope => new LongVegetationNamingPreferencesSession(scope, availability));
}
