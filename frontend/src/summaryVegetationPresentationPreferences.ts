import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { lifeformShape } from './lifeformSummary';
import { wellFormedUTF16 } from './qualityEditor';
import { ReportPreferencesSession, reportPreferencesRegistry, type ReportPreferenceRequest, type ReportTitlePreferencesOutcome } from './reportTitlePreferences';

export interface SummaryVegetationPresentationPreferenceValues {
  title: string;
  showCommon: boolean;
  showGuide: boolean;
  hierarchyHeader: boolean;
  includeEnv: boolean;
  createSummary: boolean;
  combineVariants: boolean;
  constancy: boolean;
  groupBy: number;
  display: number;
  valueLimit: number;
}
export interface SummaryVegetationPresentationPreferencesReview extends GoogleEarthScope {
  values: SummaryVegetationPresentationPreferenceValues;
  source: string;
}
export interface SummaryVegetationPresentationPreferenceDraft extends SummaryVegetationPresentationPreferenceValues {
  constancyApplied: boolean;
}
export interface SummaryVegetationPresentationPreferencesRequest extends ReportPreferenceRequest<SummaryVegetationPresentationPreferenceValues> {
  constancyApplied: boolean;
}
const booleanKeys = ['showCommon', 'showGuide', 'hierarchyHeader', 'includeEnv', 'createSummary', 'combineVariants', 'constancy'] as const;
type BooleanField = typeof booleanKeys[number];
const valueKeys = ['title', ...booleanKeys, 'groupBy', 'display', 'valueLimit'];
export const summaryVegetationPresentationSource =
  'owned native YAML saved Summary presentation; explicit Save adaptation; constancy-on forces Group by3/Display1/common names off without clearing limit; no guide renames, field-list writes or report-consumer parity';
function valid(value: unknown): value is SummaryVegetationPresentationPreferenceValues {
  return lifeformShape(value, valueKeys) && typeof value.title === 'string' && wellFormedUTF16(value.title) &&
    booleanKeys.every(key => typeof value[key] === 'boolean') &&
    typeof value.groupBy === 'number' && [1, 2, 3, 4].includes(value.groupBy) &&
    typeof value.display === 'number' && [1, 2, 3, 4, 5, 6].includes(value.display) &&
    typeof value.valueLimit === 'number' && Number.isFinite(value.valueLimit);
}
function copy(value: SummaryVegetationPresentationPreferenceValues): SummaryVegetationPresentationPreferenceValues {
  return { title: value.title, showCommon: value.showCommon, showGuide: value.showGuide,
    hierarchyHeader: value.hierarchyHeader, includeEnv: value.includeEnv, createSummary: value.createSummary,
    combineVariants: value.combineVariants, constancy: value.constancy,
    groupBy: value.groupBy, display: value.display, valueLimit: value.valueLimit };
}
function draftValid(value: unknown): value is SummaryVegetationPresentationPreferenceDraft {
  if (!lifeformShape(value, [...valueKeys, 'constancyApplied']) || typeof value.constancyApplied !== 'boolean') return false;
  const { constancyApplied: ignored, ...values } = value;
  return valid(values);
}
function equal(a: SummaryVegetationPresentationPreferenceValues, b: SummaryVegetationPresentationPreferenceValues): boolean {
  return a.title === b.title && booleanKeys.every(key => a[key] === b[key]) &&
    a.groupBy === b.groupBy && a.display === b.display && a.valueLimit === b.valueLimit;
}
export function validateSummaryVegetationPresentationPreferences(value: unknown, scope: GoogleEarthScope): SummaryVegetationPresentationPreferencesReview {
  if (!lifeformShape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'values', 'source']) ||
      typeof value.contextId !== 'string' || typeof value.project !== 'string' || typeof value.projectPath !== 'string' ||
      typeof value.su !== 'string' || typeof value.suPath !== 'string' ||
      !ownedScope({ contextId: value.contextId, project: value.project, projectPath: value.projectPath, su: value.su, suPath: value.suPath }, scope) ||
      !valid(value.values) || value.source !== summaryVegetationPresentationSource) {
    throw new Error('Saved Summary presentation is missing, malformed or belongs to another owned context; no defaults applied.');
  }
  return { ...scope, values: copy(value.values), source: value.source };
}
export class SummaryVegetationPresentationPreferencesSession extends ReportPreferencesSession<
  SummaryVegetationPresentationPreferenceValues, SummaryVegetationPresentationPreferenceDraft | null, SummaryVegetationPresentationPreferencesReview> {
  private readError = '';
  private readonly ownerScope: GoogleEarthScope;
  private readonly receiptPlans: boolean[] = [];
  constructor(scope: GoogleEarthScope) {
    super(scope, {
      initialDraft: null, savedLabel: 'saved Summary presentation',
      review: (value, owner) => validateSummaryVegetationPresentationPreferences(value, owner).values,
      draft: saved => ({ ...copy(saved), constancyApplied: false }),
      validate: (draft, saved) => {
        if (!draftValid(draft) || !saved) return 'Load complete Summary presentation, or correct/Undo the draft.';
        if (draft.title !== saved.title && draft.title.includes('\0'))
          return 'Changed title requires complete Unicode without NUL; correct it or Undo the draft.';
        if (draft.valueLimit !== saved.valueLimit) return 'The original constancy callback retains the historical limit; Undo this draft.';
        if (draft.constancyApplied) {
          if (draft.groupBy !== 3 || draft.display !== 1 || draft.constancy && draft.showCommon)
            return 'Constancy-on requires Group by3/Display1; common names stay off while constancy is selected.';
        } else {
          if (!saved.constancy && draft.constancy) return 'Constancy-on requires its original coupled callback plan.';
          if (draft.groupBy !== saved.groupBy || draft.display !== saved.display)
            return 'Shared Group by/Display are not independently editable in presentation.';
          if (draft.constancy && draft.showCommon !== saved.showCommon)
            return 'Common-name editing is disabled while constancy is selected.';
        }
        return '';
      },
      proposed: draft => {
        if (!draftValid(draft)) throw new Error('Load complete Summary presentation before Save.');
        return copy(draft);
      },
      equal,
    });
    this.ownerScope = structuredClone(scope);
  }
  override view() {
    const view = super.view();
    const receipts = view.receipts.map((receipt, index) => {
      const constancyApplied = this.receiptPlans[index];
      if (typeof constancyApplied !== 'boolean') throw new Error('Retained Summary presentation receipt lacks its source callback plan.');
      return { ...receipt, request: { ...receipt.request, constancyApplied } };
    });
    return { ...view, receipts, readError: this.readError };
  }
  rejectRead(cause: unknown) {
    this.readError = `Saved Summary presentation unavailable; no defaults applied: ${String(cause)}`;
    this.notify();
  }
  override apply(value: SummaryVegetationPresentationPreferencesReview | null, preserveDraft = false) {
    const view = this.view();
    if (view.busy || view.blocked) throw new Error('Wait for and acknowledge the preference save outcome first.');
    const reviewed = validateSummaryVegetationPresentationPreferences(value, this.ownerScope);
    const previousError = this.readError;
    this.readError = '';
    try {
      super.apply(reviewed, preserveDraft);
    } catch (error) {
      this.readError = previousError;
      throw error;
    }
  }
  override async save(port: (request: SummaryVegetationPresentationPreferencesRequest) =>
    PromiseLike<ReportTitlePreferencesOutcome | null>): Promise<void> {
    if (this.readError) throw new Error('Explicitly Load saved Summary presentation successfully before Save; the last read failed.');
    const draft = this.view().draft;
    if (!draftValid(draft)) throw new Error('Load complete Summary presentation before Save.');
    const constancyApplied = draft.constancyApplied;
    await super.save(request => {
      this.receiptPlans.push(constancyApplied);
      return port({ ...request, constancyApplied });
    });
  }
  override acknowledge() {
    const view = this.view();
    super.acknowledge();
    if (view.blocked && view.outcome && (view.outcome.committed || !view.outcome.errorMessage) && view.draft)
      this.editDraft({ ...view.draft, constancyApplied: false });
  }
  editTitle(raw: string) {
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved Summary presentation before editing.');
    this.editDraft({ ...draft, title: raw });
  }
  editFlag(key: BooleanField, raw: boolean) {
    if (!booleanKeys.some(field => field === key) || typeof raw !== 'boolean')
      throw new Error('Only original Summary presentation Boolean controls are editable.');
    const draft = this.view().draft;
    if (!draft) throw new Error('Load saved Summary presentation before editing.');
    if (key === 'showCommon' && draft.constancy) throw new Error('Common-name editing is disabled while constancy is selected.');
    if (key === 'constancy' && raw && !draft.constancy) {
      this.editDraft({ ...draft, constancy: true, groupBy: 3, display: 1, showCommon: false, constancyApplied: true });
    } else {
      this.editDraft({ ...draft, [key]: raw });
    }
  }
}
export function createSummaryVegetationPresentationPreferencesRegistry() {
  return reportPreferencesRegistry(scope => new SummaryVegetationPresentationPreferencesSession(scope));
}
