import type { GoogleEarthScope } from './googleEarthReview';
import { ownedScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationUnitPreferenceValues { unitGroups: number }
export interface LongVegetationUnitAvailability { siteUnitTable: boolean; selectedFields: boolean; none: boolean }
export interface LongVegetationUnitPreferencesReview extends GoogleEarthScope {
  values: LongVegetationUnitPreferenceValues;
  availability: LongVegetationUnitAvailability;
}
export interface LongVegetationUnitDraft { unitGroups: string }
export const longVegetationUnitChoices = [
  { value: 1, label: 'Site unit table', capability: 'siteUnitTable' },
  { value: 2, label: 'Selected fields', capability: 'selectedFields' },
  { value: 3, label: 'None', capability: 'none' },
] as const;
const choice = (value: unknown): value is 1 | 2 | 3 => value === 1 || value === 2 || value === 3;
export function draftChoice(value: unknown): 1 | 2 | 3 | null {
  switch (value) {
    case '1': return 1;
    case '2': return 2;
    case '3': return 3;
    default: return null;
  }
}
export function validateLongVegetationUnitPreferences(value: LongVegetationUnitPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationUnitPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !choice(value.values.unitGroups) ||
      !value.availability || value.availability.siteUnitTable !== true ||
      typeof value.availability.selectedFields !== 'boolean' || typeof value.availability.none !== 'boolean') {
    throw new Error('Saved Long Vegetation Unit Groups or availability is incomplete, unsupported or belongs to another owned context; no defaults applied.');
  }
  return { ...scope, values: { unitGroups: value.values.unitGroups },
    availability: { siteUnitTable: true, selectedFields: value.availability.selectedFields, none: value.availability.none } };
}
export class LongVegetationUnitPreferencesSession extends ReportPreferencesSession<
  LongVegetationUnitPreferenceValues, LongVegetationUnitDraft, LongVegetationUnitPreferencesReview> {
  private readonly capabilities: { value: LongVegetationUnitAvailability | null };
  constructor(scope: GoogleEarthScope, noneUnitsEnabled: boolean, selectedUnitsEnabled = false) {
    if (typeof noneUnitsEnabled !== 'boolean' || typeof selectedUnitsEnabled !== 'boolean') {
      throw new Error('Explicit immutable client None/selected-field report unit availability is required.');
    }
    const capabilities: { value: LongVegetationUnitAvailability | null } = { value: null };
    super(scope, {
      initialDraft: { unitGroups: '' }, savedLabel: 'saved Unit Groups choice',
      review: (value, owner) => {
        const reviewed = validateLongVegetationUnitPreferences(value, owner);
        capabilities.value = { ...reviewed.availability, none: reviewed.availability.none && noneUnitsEnabled,
          selectedFields: reviewed.availability.selectedFields && selectedUnitsEnabled };
        return reviewed.values;
      },
      draft: saved => ({ unitGroups: String(saved.unitGroups) }),
      validate: (draft, saved) => {
        const proposed = draftChoice(draft.unitGroups);
        if (proposed === null) return 'Choose a supported Unit Groups value, or Undo the preference draft.';
        if (proposed === saved?.unitGroups) return '';
        const mode = longVegetationUnitChoices.find(mode => mode.value === proposed);
        return mode && capabilities.value?.[mode.capability] ? '' :
          'This Unit Groups choice is unavailable in this build/session; correct it or Undo. Saved values are not reset.';
      },
      proposed: draft => {
        const unitGroups = draftChoice(draft.unitGroups);
        if (unitGroups === null) throw new Error('Unit Groups draft is not a supported source choice.');
        return { unitGroups };
      },
      equal: (a, b) => a.unitGroups === b.unitGroups,
    });
    this.capabilities = capabilities;
  }
  availability(): LongVegetationUnitAvailability | null {
    return this.capabilities.value ? { ...this.capabilities.value } : null;
  }
  editUnitGroups(value: string) {
    if (!this.view().saved) throw new Error('Load saved Unit Groups choice before editing.');
    this.editDraft({ unitGroups: value });
  }
}
export function createLongVegetationUnitPreferencesRegistry(noneUnitsEnabled: boolean, selectedUnitsEnabled = false) {
  return reportPreferencesRegistry(scope => new LongVegetationUnitPreferencesSession(scope, noneUnitsEnabled, selectedUnitsEnabled));
}
