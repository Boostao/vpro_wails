import type {
  LongVegetationProjectUnitOutcome, LongVegetationProjectUnitPreferences, LongVegetationReportScope,
  LongVegetationUnitPreferenceValues,
} from '../bindings/github.com/boostao/vpro-wails';
import type { ProjectReportOwner } from './longVegetationProjectReport';
import { projectReportOwnedScope } from './longVegetationProjectReport';
import type { CurrentProjectScope } from './longVegetationProjectScope';
import { currentProjectScope, validateProjectUnitOutcome, validateProjectUnitPreferences } from './longVegetationProjectScope';
import type { LongVegetationUnitDraft } from './longVegetationUnitPreferences';
import { draftChoice } from './longVegetationUnitPreferences';
import { ReportPreferencesSession } from './reportTitlePreferences';

export type ProjectUnitSaveRequest = {
  revision: string;
  approvalHash: string;
  expected: LongVegetationUnitPreferenceValues;
  proposed: LongVegetationUnitPreferenceValues;
};
export type ProjectUnitReceipt =
  | { kind: 'known'; contextId: string; request: ProjectUnitSaveRequest; outcome: LongVegetationProjectUnitOutcome }
  | { kind: 'unknown'; contextId: string; request: ProjectUnitSaveRequest; errorMessage: string };

const snapshotApproval = (value: LongVegetationReportScope, owner: ProjectReportOwner): CurrentProjectScope => {
  const scope = currentProjectScope(value, owner, true);
  return {
    ...scope, fields: [...scope.fields], build: {
      approvalHash: scope.build.approvalHash,
      projectTuples: scope.build.projectTuples.map(row => ({
        ...row, plotNumber: { ...row.plotNumber }, siteUnit: { ...row.siteUnit },
      })),
    },
  };
};

export class ProjectUnitPreferencesSession extends ReportPreferencesSession<
  LongVegetationUnitPreferenceValues, LongVegetationUnitDraft, LongVegetationProjectUnitPreferences> {
  private readonly owner: ProjectReportOwner;
  private readonly authority: { scope: CurrentProjectScope | null };
  private readonly fullReceipts: ProjectUnitReceipt[] = [];
  private readGeneration = 0;
  private readBusy = false;
  private readReady = false;
  private readError = '';

  constructor(owner: ProjectReportOwner, enabled = false) {
    if (enabled !== true) throw new Error('Whole-project Unit preference session is disabled in this build.');
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project Unit session requires complete frozen physical owner paths.');
    const frozen = { contextId: owner.contextId, project: owner.project, projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath };
    const authority: { scope: CurrentProjectScope | null } = { scope: null };
    super(frozen, {
      initialDraft: { unitGroups: '' }, savedLabel: 'actual saved project Unit Groups',
      review: (value, physicalOwner) => {
        if (!authority.scope || !value) throw new Error('Load requires its independent current project scope.');
        return validateProjectUnitPreferences(value, authority.scope, physicalOwner, true).preferences.values;
      },
      draft: saved => ({ unitGroups: String(saved.unitGroups) }),
      validate: (draft, saved) => {
        const proposed = draftChoice(draft.unitGroups);
        if (proposed === null) return 'Choose a supported project Unit Groups value, or Undo the preference draft.';
        if (proposed === saved?.unitGroups) return '';
        if (!authority.scope) return 'Load current project scope before changing Unit Groups, or Undo.';
        const desired = authority.scope.sourceKind === 'project-selected-fields' ? 2 : 3;
        return proposed === desired ? '' : 'This choice belongs to a different report scope; correct it or Undo, then switch scope explicitly.';
      },
      proposed: draft => {
        const unitGroups = draftChoice(draft.unitGroups);
        if (unitGroups === null) throw new Error('Project Unit Groups draft is not a supported source choice.');
        return { unitGroups };
      },
      equal: (left, right) => left.unitGroups === right.unitGroups,
    });
    this.owner = Object.freeze(frozen);
    this.authority = authority;
  }

  projectView() {
    const view = this.view(), changed = !!view.saved && view.draft.unitGroups !== String(view.saved.unitGroups);
    const desired = this.authority.scope?.sourceKind === 'project-selected-fields' ? 2 :
      this.authority.scope?.sourceKind === 'project-all-plots' ? 3 : null;
    return {
      ...view, scope: this.authority.scope ? snapshotApproval(this.authority.scope, this.owner) : null,
      readBusy: this.readBusy, readReady: this.readReady, readError: this.readError,
      projectReceipts: structuredClone(this.fullReceipts),
      saveReady: this.readReady && !this.readBusy && !view.busy && !view.blocked && !view.authorityUnknown &&
        !view.draftError && !!view.saved && desired !== null && draftChoice(view.draft.unitGroups) === desired,
      closeBlocked: this.readBusy || view.busy || view.blocked || view.authorityUnknown || changed || !!view.draftError,
    };
  }

  private invalidateRead(): void {
    this.readGeneration++;
    this.cancelLoad();
    this.readBusy = false;
  }

  cancelRead(): void {
    this.invalidateRead();
    this.notify();
  }

  invalidateObservation(): void {
    this.readReady = false;
    this.cancelRead();
  }

  setApproval(value: LongVegetationReportScope | null): void {
    const view = this.view();
    if (view.busy || view.blocked) throw new Error('Wait for and acknowledge the project Unit Save receipt before changing its scope.');
    const scope = value ? snapshotApproval(value, this.owner) : null;
    this.invalidateRead();
    this.authority.scope = scope;
    this.readReady = false;
    this.readError = '';
    this.notify();
  }

  async load(fetch: (request: { revision: string; approvalHash: string }) =>
    PromiseLike<LongVegetationProjectUnitPreferences>, validationYield: () => PromiseLike<void> =
    () => new Promise<void>(resolve => setTimeout(resolve, 0))): Promise<void> {
    if (!this.authority.scope || this.readBusy) throw new Error('Load current project scope before reading actual saved Unit Groups.');
    const expected = snapshotApproval(this.authority.scope, this.owner), before = this.view();
    const preserveDraft = before.initialized && (before.authorityUnknown || !!before.draftError ||
      !!before.saved && before.draft.unitGroups !== String(before.saved.unitGroups));
    const generation = this.beginLoad(), request = ++this.readGeneration;
    this.readBusy = true;
    this.readReady = false;
    this.readError = '';
    this.notify();
    try {
      const wire = await fetch({ revision: expected.revision, approvalHash: expected.build.approvalHash });
      if (request !== this.readGeneration) return;
      const reviewed = validateProjectUnitPreferences(wire, expected, this.owner, true);
      const detached: LongVegetationProjectUnitPreferences = {
        scope: snapshotApproval(reviewed.scope, this.owner),
        preferences: { ...reviewed.preferences, values: { ...reviewed.preferences.values }, availability: { ...reviewed.preferences.availability } },
      };
      await validationYield();
      if (request !== this.readGeneration) return;
      if (this.applyLoaded(detached, generation, preserveDraft)) this.readReady = true;
    } catch (cause) {
      if (request === this.readGeneration) this.readError = `Project Unit preferences unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === this.readGeneration) { this.readBusy = false; this.notify(); }
    }
  }

  editUnitGroups(value: string): void {
    if (!this.view().saved || this.readBusy) throw new Error('Load actual saved project Unit Groups before editing and finish or cancel the current read.');
    this.invalidateRead();
    this.editDraft({ unitGroups: value });
  }

  override undo(): void {
    if (this.readBusy) throw new Error('Finish or cancel the project Unit read before Undo.');
    this.invalidateRead();
    super.undo();
  }

  async saveProject(port: (request: ProjectUnitSaveRequest) => PromiseLike<LongVegetationProjectUnitOutcome>): Promise<void> {
    if (!this.projectView().saveReady || !this.authority.scope) {
      throw new Error('Load and correct the actual saved Unit choice for its current project scope before Save.');
    }
    const scope = snapshotApproval(this.authority.scope, this.owner);
    this.invalidateRead();
    await super.save(async request => {
      const full: ProjectUnitSaveRequest = {
        revision: scope.revision, approvalHash: scope.build.approvalHash,
        expected: { ...request.expected }, proposed: { ...request.proposed },
      };
      try {
        const outcome = validateProjectUnitOutcome(await port(structuredClone(full)), scope, this.owner,
          full.expected.unitGroups !== full.proposed.unitGroups, true);
        const detached: LongVegetationProjectUnitOutcome = {
          sourceKind: outcome.sourceKind, revision: outcome.revision, outcome: { ...outcome.outcome },
        };
        this.fullReceipts.push({ kind: 'known', contextId: this.owner.contextId, request: full, outcome: detached });
        return detached.outcome;
      } catch (cause) {
        this.readReady = false;
        this.fullReceipts.push({ kind: 'unknown', contextId: this.owner.contextId, request: full, errorMessage: String(cause) });
        throw cause;
      }
    });
  }

  override async save(): Promise<void> {
    throw new Error('Project Unit Save requires its full current scope request; use SaveProject rather than generic preference Save.');
  }
}

export function createProjectUnitPreferencesRegistry(enabled = false) {
  if (enabled !== true) throw new Error('Whole-project Unit preference registry is disabled in this build.');
  const sessions = new Map<string, { owner: ProjectReportOwner; session: ProjectUnitPreferencesSession }>();
  return (owner: ProjectReportOwner): ProjectUnitPreferencesSession => {
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project Unit registry requires complete owned physical paths.');
    const existing = sessions.get(owner.contextId);
    if (existing && !projectReportOwnedScope(existing.owner, owner)) throw new Error('Project Unit session identity changed physical paths.');
    if (existing) return existing.session;
    const frozen = { contextId: owner.contextId, project: owner.project, projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath };
    const session = new ProjectUnitPreferencesSession(frozen, enabled);
    sessions.set(frozen.contextId, { owner: frozen, session });
    return session;
  };
}
