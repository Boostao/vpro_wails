import { ownedScope, type GoogleEarthScope } from './googleEarthReview';
import { wellFormedUTF16 } from './qualityEditor';
import { ReportPreferencesSession, reportPreferencesRegistry, type ReportPreferenceRequest,
  type ReportTitlePreferencesOutcome } from './reportTitlePreferences';

export interface LongVegetationEnforcementPreferenceValues { enforce: boolean; unitGroups: number }
export interface PrinterObservation { names: string[] | null; default: string | null; source: string }
export interface LongVegetationEnforcementPreferencesReview extends GoogleEarthScope {
  values: LongVegetationEnforcementPreferenceValues;
  printers: PrinterObservation;
}
export interface LongVegetationEnforcementPreferencesOutcome extends ReportTitlePreferencesOutcome {
  printers: PrinterObservation | null;
}
export interface LongVegetationEnforcementDraft { enforce: boolean | null; unitGroups: number | null }
export const printerObservationSource = 'installed printers and current default observed read-only; separate host reads, not an atomic snapshot or permission to change the default';
const groups = (value: unknown): value is 1 | 2 | 3 => value === 1 || value === 2 || value === 3;
const printerName = (value: unknown): value is string => typeof value === 'string' && value.length > 0 &&
  wellFormedUTF16(value) && !value.includes('\0');
export function validatePrinterObservation(value: PrinterObservation | null): PrinterObservation & { names: string[] } {
  if (!value || !Array.isArray(value.names) || !value.names.every(printerName) ||
      value.default !== null && !printerName(value.default) || value.source !== printerObservationSource) {
    throw new Error('Read-only printer observation is missing or malformed; no printer names or default were inferred.');
  }
  return { names: value.names.map(name => name), default: value.default, source: value.source };
}
export function validateLongVegetationEnforcementPreferences(value: LongVegetationEnforcementPreferencesReview | null,
  scope: GoogleEarthScope): LongVegetationEnforcementPreferencesReview {
  if (!value || !ownedScope(value, scope) || !value.values || typeof value.values.enforce !== 'boolean' ||
      !groups(value.values.unitGroups)) {
    throw new Error('Saved enforcement preferences are missing, malformed or belong to another owned context; no defaults applied.');
  }
  return { ...scope, values: { enforce: value.values.enforce, unitGroups: value.values.unitGroups },
    printers: validatePrinterObservation(value.printers) };
}
export class LongVegetationEnforcementPreferencesSession extends ReportPreferencesSession<
  LongVegetationEnforcementPreferenceValues, LongVegetationEnforcementDraft, LongVegetationEnforcementPreferencesReview> {
  private readonly observation: { value: PrinterObservation | null; error: string };
  constructor(scope: GoogleEarthScope) {
    const observation: { value: PrinterObservation | null; error: string } = { value: null, error: '' };
    super(scope, {
      initialDraft: { enforce: null, unitGroups: null }, savedLabel: 'saved enforcement preferences',
      review: (value, owner) => {
        const reviewed = validateLongVegetationEnforcementPreferences(value, owner);
        observation.value = reviewed.printers; observation.error = '';
        return reviewed.values;
      },
      draft: saved => ({ enforce: saved.enforce, unitGroups: saved.unitGroups }),
      validate: (draft, saved) => {
        if (typeof draft.enforce !== 'boolean' || !groups(draft.unitGroups)) {
          return 'Enforce rules requires a complete Boolean and source unit group; correct it or Undo.';
        }
        if (saved && (draft.enforce !== saved.enforce || draft.unitGroups !== saved.unitGroups) && draft.unitGroups !== 1) {
          return 'An enforcement edit must reset unit groups to Site unit table; correct it or Undo.';
        }
        return '';
      },
      proposed: draft => {
        if (typeof draft.enforce !== 'boolean' || !groups(draft.unitGroups)) {
          throw new Error('Enforcement draft has no complete owned source pair.');
        }
        return { enforce: draft.enforce, unitGroups: draft.unitGroups };
      },
      equal: (a, b) => a.enforce === b.enforce && a.unitGroups === b.unitGroups,
    });
    this.observation = observation;
  }
  printerView(): { value: PrinterObservation | null; error: string } { return structuredClone(this.observation); }
  editEnforce(enforce: boolean) {
    const view = this.view();
    if (!view.saved) throw new Error('Load saved enforcement preferences before editing.');
    this.editDraft({ enforce, unitGroups: enforce === view.draft.enforce ? view.draft.unitGroups : 1 });
  }
  async saveWithPrinters(port: (request: ReportPreferenceRequest<LongVegetationEnforcementPreferenceValues>) =>
    PromiseLike<LongVegetationEnforcementPreferencesOutcome | null>): Promise<void> {
    await super.save(async request => {
      this.observation.value = null;
      this.observation.error = 'Printer refresh has not been confirmed; retain the preference receipt and do not replay.';
      const outcome = await port(request);
      if (!outcome) return null;
      try {
        this.observation.value = validatePrinterObservation(outcome.printers);
        this.observation.error = '';
      } catch (cause) {
        this.observation.error = `Printer refresh unavailable; retain the preference receipt and do not replay: ${String(cause)}`;
      }
      // The independently validated preference receipt survives a failed host observation.
      return { contextId: outcome.contextId, changed: outcome.changed, committed: outcome.committed,
        errorMessage: outcome.errorMessage };
    });
  }
}
export function createLongVegetationEnforcementPreferencesRegistry() {
  return reportPreferencesRegistry(scope => new LongVegetationEnforcementPreferencesSession(scope));
}
