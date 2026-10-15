import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { lifeformShape } from './lifeformSummary';
import { ReportPreferencesSession, reportPreferencesRegistry, type ReportPreferenceRequest, type ReportTitlePreferencesOutcome } from './reportTitlePreferences';
import { summaryVegetationOptionError } from './summaryVegetation';

export interface SummaryVegetationPreferenceValues {
  groupBy: number;
  average: number;
  display: number;
  showClass: number;
  valueLimit: number;
}
export interface SummaryVegetationPreferencesReview extends GoogleEarthScope {
  values: SummaryVegetationPreferenceValues;
  previewEnabled: boolean;
  selectionAvailable: boolean;
}
export interface SummaryVegetationPreferenceDraft {
  groupBy: string;
  average: string;
  display: string;
  showClass: string;
  valueLimit: number;
}
type EditableSummaryPreference = 'groupBy' | 'average' | 'display' | 'showClass';
const preferenceKeys = ['groupBy', 'average', 'display', 'showClass', 'valueLimit'];
function valuesValid(value: unknown): value is SummaryVegetationPreferenceValues {
  return lifeformShape(value, preferenceKeys) && typeof value.groupBy === 'number' && [1, 2, 3, 4].includes(value.groupBy) &&
    typeof value.average === 'number' && [1, 2].includes(value.average) &&
    typeof value.display === 'number' && [1, 2, 3, 4, 5, 6].includes(value.display) &&
    typeof value.showClass === 'number' && [1, 10, 20].includes(value.showClass) &&
    typeof value.valueLimit === 'number' && Number.isFinite(value.valueLimit);
}
function selectionAvailable(value: SummaryVegetationPreferenceValues): boolean {
  return !((value.groupBy === 2 || value.groupBy === 3) && value.display === 2);
}
function equal(a: SummaryVegetationPreferenceValues, b: SummaryVegetationPreferenceValues): boolean {
  return a.groupBy === b.groupBy && a.average === b.average && a.display === b.display &&
    a.showClass === b.showClass && a.valueLimit === b.valueLimit;
}
function draftValues(draft: SummaryVegetationPreferenceDraft): SummaryVegetationPreferenceValues | null {
  const source = (raw: unknown, choices: readonly number[]): number | null =>
    typeof raw === 'string' ? choices.find(choice => String(choice) === raw) ?? null : null;
  const groupBy = source(draft.groupBy, [1, 2, 3, 4]), average = source(draft.average, [1, 2]),
    display = source(draft.display, [1, 2, 3, 4, 5, 6]), showClass = source(draft.showClass, [1, 10, 20]);
  if (groupBy === null || average === null || display === null || showClass === null ||
      typeof draft.valueLimit !== 'number' || !Number.isFinite(draft.valueLimit)) return null;
  return { groupBy, average, display, showClass, valueLimit: draft.valueLimit };
}
export function validateSummaryVegetationPreferences(value: unknown, scope: GoogleEarthScope): SummaryVegetationPreferencesReview {
  if (!lifeformShape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'values', 'previewEnabled', 'selectionAvailable']) ||
      typeof value.contextId !== 'string' || typeof value.project !== 'string' || typeof value.projectPath !== 'string' ||
      typeof value.su !== 'string' || typeof value.suPath !== 'string' ||
      !ownedScope({ contextId: value.contextId, project: value.project, projectPath: value.projectPath, su: value.su, suPath: value.suPath }, scope) ||
      !valuesValid(value.values) || typeof value.previewEnabled !== 'boolean' ||
      value.selectionAvailable !== selectionAvailable(value.values)) {
    throw new Error('Saved Summary Vegetation options are missing, malformed or belong to another owned context; no defaults applied.');
  }
  const values = value.values;
  return { ...scope, values: { groupBy: values.groupBy, average: values.average, display: values.display,
    showClass: values.showClass, valueLimit: values.valueLimit },
    previewEnabled: value.previewEnabled, selectionAvailable: value.selectionAvailable };
}
export class SummaryVegetationPreferencesSession extends ReportPreferencesSession<
  SummaryVegetationPreferenceValues, SummaryVegetationPreferenceDraft, SummaryVegetationPreferencesReview> {
  private readonly capability: { preview: boolean | null };
  private readError = '';
  constructor(scope: GoogleEarthScope, clientPreviewEnabled: boolean) {
    if (typeof clientPreviewEnabled !== 'boolean') throw new Error('Explicit immutable Summary preview client availability is required.');
    const capability: { preview: boolean | null } = { preview: null };
    super(scope, {
      initialDraft: { groupBy: '', average: '', display: '', showClass: '', valueLimit: 0 },
      savedLabel: 'saved Summary options',
      review: (value, owner) => {
        const reviewed = validateSummaryVegetationPreferences(value, owner);
        capability.preview = reviewed.previewEnabled && clientPreviewEnabled;
        return reviewed.values;
      },
      draft: saved => ({ groupBy: String(saved.groupBy), average: String(saved.average), display: String(saved.display),
        showClass: String(saved.showClass), valueLimit: saved.valueLimit }),
      validate: (draft, saved) => {
        const proposed = draftValues(draft);
        if (!proposed) return 'Choose original Summary options, or Undo the preference draft.';
        if (saved && equal(proposed, saved)) return '';
        if (!capability.preview) return 'Summary preview is unavailable in this build/session; correct or Undo. Saved options are not reset.';
        const optionError = summaryVegetationOptionError(proposed.average, proposed.display, proposed.showClass, proposed.groupBy + 10);
        if (optionError) return optionError;
        if (saved && proposed.valueLimit !== saved.valueLimit && proposed.valueLimit !== 0)
          return 'Only the original display callback limit clear is available; Undo this draft.';
        if (saved && proposed.display !== saved.display && proposed.display !== 1 && proposed.display !== 3 && proposed.valueLimit !== 0)
          return 'A changed Display option must clear the paired saved limit; Undo this draft.';
        return '';
      },
      proposed: draft => {
        const values = draftValues(draft);
        if (!values) throw new Error('Summary preference draft is not an original option selection.');
        return values;
      },
      equal,
    });
    this.capability = capability;
  }
  previewEnabled(): boolean | null { return this.capability.preview; }
  override view() { return { ...super.view(), readError: this.readError }; }
  rejectRead(cause: unknown) {
    this.readError = `Saved Summary options unavailable; no defaults applied: ${String(cause)}`;
    this.notify();
  }
  override apply(value: SummaryVegetationPreferencesReview | null, preserveDraft = false) {
    super.apply(value, preserveDraft);
    this.readError = ''; this.notify();
  }
  override async save(port: (request: ReportPreferenceRequest<SummaryVegetationPreferenceValues>) =>
    PromiseLike<ReportTitlePreferencesOutcome | null>): Promise<void> {
    if (this.readError) throw new Error('Explicitly Load saved Summary options successfully before Save; the last read failed.');
    await super.save(port);
  }
  editOption(field: EditableSummaryPreference, raw: string) {
    if (!['groupBy', 'average', 'display', 'showClass'].includes(field)) throw new Error('Only original editable Summary options are available.');
    const view = this.view();
    if (!view.saved) throw new Error('Load saved Summary options before editing.');
    const valueLimit = field === 'display' && raw !== view.draft.display && ['2', '4', '5', '6'].includes(raw)
      ? 0 : view.draft.valueLimit;
    this.editDraft({ ...view.draft, [field]: raw, valueLimit });
  }
}
export function createSummaryVegetationPreferencesRegistry(clientPreviewEnabled: boolean) {
  return reportPreferencesRegistry(scope => new SummaryVegetationPreferencesSession(scope, clientPreviewEnabled));
}
