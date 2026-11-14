import { becGroups, equalCode } from './becEditor';

export type WorkingUnitMode = 'env' | 'master' | 'su';
export const workingUnitControls = ['UserSiteUnit', 'BECSiteUnit', 'btnCoptToWorkingUnit', 'Option455', 'Option457', 'Option459'] as const;
export function isWorkingUnitControl(name: string | undefined): boolean {
  return workingUnitControls.some(control => control === name);
}
export interface WorkingUnitCodes {
  userSiteUnit: string | null;
  becSiteUnit: string | null;
}
export interface WorkingUnitChoice {
  rowId: string;
  sourceId: string | null;
  origin: string;
  code: string | null;
  description: string | null;
  scientificName: string | null;
  level: number | null;
  selectable: boolean;
  diagnostic: string;
}
export interface WorkingUnitModeState {
  mode: string;
  warning: string | null;
}
export interface WorkingUnitSession {
  mode: WorkingUnitModeState | null;
}
export interface WorkingUnitAPI {
  getMode(): Promise<WorkingUnitModeState>;
  setMode(mode: WorkingUnitMode): Promise<WorkingUnitModeState>;
  choices(mode: WorkingUnitMode): Promise<WorkingUnitChoice[]>;
  master(): Promise<WorkingUnitChoice[]>;
}
export interface WorkingUnitView {
  mode: WorkingUnitMode | null;
  warning: string | null;
  choices: WorkingUnitChoice[];
  master: WorkingUnitChoice[];
  ready: boolean;
  masterReady: boolean;
  busy: boolean;
  error: string | null;
  masterError: string | null;
}
export function workingUnitMode(value: string): WorkingUnitMode {
  if (value === 'env' || value === 'master' || value === 'su') return value;
  throw new Error(`Unsupported Working Unit mode "${value}".`);
}
export function workingUnitLengthError(value: string | null, original?: string | null): string | null {
  if (value === original || value === null) return null;
  if (value === '') return 'Clear Working Unit to NULL instead of an empty string.';
  return value.length > 100 ? 'Working Unit must be at most 100 UTF-16 characters.' : null;
}
export function workingUnitChanged(codes: WorkingUnitCodes, original: WorkingUnitCodes | null): boolean {
  return codes.userSiteUnit !== (original?.userSiteUnit ?? null);
}
export function workingUnitGroups(rows: WorkingUnitChoice[]) {
  return becGroups(rows, row => row.selectable && row.code !== null && row.code.length <= 100 ? row.code : null);
}
export function workingUnitDefinitions(rows: WorkingUnitChoice[], code: string | null): WorkingUnitChoice[] {
  return rows.filter(row => row.selectable && equalCode(row.code, code));
}
export function workingUnitWarnings(code: string | null, view: WorkingUnitView): string[] {
  if (view.busy || code === null) return [];
  if (!view.ready) return [view.error ?? 'Working Unit choices are unavailable; this code has not been checked.'];
  return workingUnitDefinitions(view.choices, code).length === 0
    ? [`Working Unit "${code}" is not in the current ${view.mode} choices. No code or partner field will be replaced automatically.`] : [];
}
export function workingUnitValidation(codes: WorkingUnitCodes, original: WorkingUnitCodes | null,
  view: WorkingUnitView, acknowledged: boolean): string | null {
  if (!workingUnitChanged(codes, original)) return null;
  const invalid = workingUnitLengthError(codes.userSiteUnit, original?.userSiteUnit);
  if (invalid) return invalid;
  if (view.busy) return 'Wait for Working Unit choices and preferences to finish updating.';
  return workingUnitWarnings(codes.userSiteUnit, view).length && !acknowledged
    ? 'Review and explicitly acknowledge the unmatched Working Unit code before saving.' : null;
}
const acknowledgements = new WeakMap<object, string>();
export function rememberWorkingUnitAcknowledgement(draft: object, code: string | null, accepted: boolean): void {
  if (accepted) acknowledgements.set(draft, JSON.stringify(code));
  else acknowledgements.delete(draft);
}
export function workingUnitAcknowledged(draft: object, code: string | null): boolean {
  return acknowledgements.get(draft) === JSON.stringify(code);
}
export interface WorkingUnitCopy {
  kind: 'unchanged' | 'stage' | 'confirm' | 'invalid';
  value: string | null;
  message: string;
}
export function workingUnitCopy(codes: WorkingUnitCodes): WorkingUnitCopy {
  const value = codes.becSiteUnit;
  if (value === codes.userSiteUnit) return { kind: 'unchanged', value, message: 'Working Unit already equals BEC Master; no data changed.' };
  const invalid = workingUnitLengthError(value);
  if (invalid) return { kind: 'invalid', value, message: `Cannot copy BEC Master: ${invalid}` };
  if (codes.userSiteUnit !== null) {
    return { kind: 'confirm', value, message: value === null
      ? 'BEC Master is NULL. Confirm clearing the existing Working Unit in this draft.'
      : 'Confirm replacing the existing Working Unit with the exact BEC Master code in this draft.' };
  }
  return { kind: 'stage', value, message: 'BEC Master copied into the Working Unit draft. Save or Undo explicitly.' };
}

export class WorkingUnitLookup {
  private revision = 0;
  private disposed = false;
  private view: WorkingUnitView = { mode: null, warning: null, choices: [], master: [],
    ready: false, masterReady: false, busy: false, error: null, masterError: null };
  constructor(private api: WorkingUnitAPI, private state: (view: WorkingUnitView) => void,
    private session: WorkingUnitSession = { mode: null }, private cancel?: () => void) {}
  snapshot(): WorkingUnitView { return { ...this.view }; }
  private publish(update: Partial<WorkingUnitView>): void {
    this.view = { ...this.view, ...update };
    this.state(this.snapshot());
  }
  async refresh(mode?: WorkingUnitMode): Promise<void> {
    if (this.disposed) return;
    if (mode !== undefined && this.view.busy) throw new Error('Wait for the current Working Unit preference operation.');
    const revision = ++this.revision;
    this.cancel?.();
    this.publish({ busy: true, ready: false, masterReady: false, choices: [], master: [], error: null, masterError: null });
    try {
      const preference = mode === undefined ? this.session.mode ?? await this.api.getMode() : await this.api.setMode(mode);
      const selected = workingUnitMode(preference.mode);
      if (this.disposed || revision !== this.revision) return;
      this.session.mode = { mode: selected, warning: preference.warning };
      const [choices, master] = await Promise.allSettled([this.api.choices(selected), this.api.master()]);
      if (this.disposed || revision !== this.revision) return;
      const choicesValid = choices.status === 'fulfilled' && Array.isArray(choices.value);
      const masterValid = master.status === 'fulfilled' && Array.isArray(master.value);
      this.publish({ mode: selected, warning: preference.warning, busy: false,
        choices: choicesValid && choices.status === 'fulfilled' ? choices.value : [],
        master: masterValid && master.status === 'fulfilled' ? master.value : [],
        ready: choicesValid, masterReady: masterValid,
        error: choicesValid ? null : `Working Unit choices could not be loaded: ${choices.status === 'rejected' ? String(choices.reason) : 'service did not return an array'}`,
        masterError: masterValid ? null : `BEC Master metadata could not be loaded: ${master.status === 'rejected' ? String(master.reason) : 'service did not return an array'}`
      });
    } catch (cause) {
      if (this.disposed || revision !== this.revision) return;
      this.publish({ busy: false, ready: false, masterReady: false,
        error: `Working Unit preference could not be loaded or saved: ${String(cause)}` });
    }
  }
  dispose(): void { this.disposed = true; this.revision++; this.cancel?.(); }
}
