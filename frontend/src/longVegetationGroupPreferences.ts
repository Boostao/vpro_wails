import type { GoogleEarthScope } from './googleEarthReview';
import { ownedScope } from './googleEarthReview';
import { ReportPreferencesSession, reportPreferencesRegistry } from './reportTitlePreferences';

export interface LongVegetationGroupPreferenceValues { groupBy: number }
export interface LongVegetationGroupAvailability { layer: boolean; strata: boolean; lifeform: boolean; none: boolean }
export interface LongVegetationGroupClientAvailability { strata: boolean; lifeform: boolean }
export interface LongVegetationGroupPreferencesReview extends GoogleEarthScope {
  values: LongVegetationGroupPreferenceValues;
  availability: LongVegetationGroupAvailability;
}
export interface LongVegetationGroupDraft { groupBy: string }
export const longVegetationGroupChoices = [
  { value: 1, label: 'Layer', capability: 'layer' },
  { value: 2, label: 'Strata', capability: 'strata' },
  { value: 3, label: 'Lifeform', capability: 'lifeform' },
  { value: 4, label: 'None', capability: 'none' },
] as const;
const choice = (value: unknown): value is 1 | 2 | 3 | 4 => value === 1 || value === 2 || value === 3 || value === 4;
function draftChoice(value: unknown): 1 | 2 | 3 | 4 | null {
  switch (value) {
    case '1': return 1;
    case '2': return 2;
    case '3': return 3;
    case '4': return 4;
    default: return null;
  }
}
export function validateLongVegetationGroupPreferences(value: LongVegetationGroupPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationGroupPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || !choice(value.values.groupBy) ||
      !value.availability || value.availability.layer !== true ||
      [value.availability.strata, value.availability.lifeform, value.availability.none].some(flag => typeof flag !== 'boolean')) {
    throw new Error('Saved Long Vegetation grouping or availability is missing, unsupported or belongs to another owned context; no defaults applied.');
  }
  return { ...scope, values: { groupBy: value.values.groupBy }, availability: {
    layer: true, strata: value.availability.strata, lifeform: value.availability.lifeform, none: value.availability.none,
  } };
}
export class LongVegetationGroupPreferencesSession extends ReportPreferencesSession<
  LongVegetationGroupPreferenceValues, LongVegetationGroupDraft, LongVegetationGroupPreferencesReview> {
  private readonly capabilities: { value: LongVegetationGroupAvailability | null };
  constructor(scope: GoogleEarthScope, client: LongVegetationGroupClientAvailability) {
    if (typeof client?.strata !== 'boolean' || typeof client?.lifeform !== 'boolean') {
      throw new Error('Explicit immutable client grouping availability is required.');
    }
    const frontend = { strata: client.strata, lifeform: client.lifeform };
    const capabilities: { value: LongVegetationGroupAvailability | null } = { value: null };
    super(scope, {
      initialDraft: { groupBy: '' }, savedLabel: 'saved Group by choice',
      review: (value, owner) => {
        const reviewed = validateLongVegetationGroupPreferences(value, owner);
        capabilities.value = { ...reviewed.availability,
          strata: reviewed.availability.strata && frontend.strata, lifeform: reviewed.availability.lifeform && frontend.lifeform };
        return reviewed.values;
      },
      draft: saved => ({ groupBy: String(saved.groupBy) }),
      validate: (draft, saved) => {
        const proposed = draftChoice(draft.groupBy);
        if (proposed === null) return 'Choose a supported Group by value, or Undo the preference draft.';
        if (proposed === saved?.groupBy) return '';
        const mode = longVegetationGroupChoices.find(mode => mode.value === proposed);
        return mode && capabilities.value?.[mode.capability] ? '' :
          'This Group by choice is unavailable in this build/session; correct it or Undo. Saved values are not reset.';
      },
      proposed: draft => {
        const groupBy = draftChoice(draft.groupBy);
        if (groupBy === null) throw new Error('Group by draft is not a supported source choice.');
        return { groupBy };
      },
      equal: (a, b) => a.groupBy === b.groupBy,
    });
    this.capabilities = capabilities;
  }
  availability(): LongVegetationGroupAvailability | null {
    return this.capabilities.value ? { ...this.capabilities.value } : null;
  }
  editGroup(value: string) {
    if (!this.view().saved) throw new Error('Load saved Group by choice before editing.');
    this.editDraft({ groupBy: value });
  }
}
export function createLongVegetationGroupPreferencesRegistry(client: LongVegetationGroupClientAvailability) {
  const availability = { strata: client.strata, lifeform: client.lifeform };
  return reportPreferencesRegistry(scope => new LongVegetationGroupPreferencesSession(scope, availability));
}
