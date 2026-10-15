import type { LongVegetationProjectFieldDefinitions, LongVegetationReportScope } from '../bindings/github.com/boostao/vpro-wails';
import type { ProjectReportOwner } from './longVegetationProjectReport';
import { projectReportOwnedScope } from './longVegetationProjectReport';
import type { ReportSourceScope, ValidatedReportScope } from './longVegetationProjectScope';
import { validateProjectFieldCatalogue, validateReportScope } from './longVegetationProjectScope';
import { selectedUnitFieldError } from './longVegetationSelectedUnitSelection';

export interface ProjectScopeSessionPort {
  scope(): PromiseLike<LongVegetationReportScope>;
  catalogue(): PromiseLike<LongVegetationProjectFieldDefinitions>;
  switchScope(request: { revision: string; sourceKind: ReportSourceScope }): PromiseLike<LongVegetationReportScope>;
  fields(request: { revision: string; fields: string[] }): PromiseLike<LongVegetationReportScope>;
  build(revision: string): PromiseLike<LongVegetationReportScope>;
  validationYield?(): PromiseLike<void>;
}

export type ProjectScopeSessionView = {
  scope: ValidatedReportScope | null;
  definitions: NonNullable<LongVegetationProjectFieldDefinitions['definitions']>;
  fields: string[];
  dirty: boolean;
  draftError: string;
  error: string;
  busy: boolean;
  authorityUnknown: boolean;
  outputReady: boolean;
  closeBlocked: boolean;
};

const sameFields = (left: string[], right: string[]): boolean =>
  left.length === right.length && left.every((field, i) => field === right[i]);
const copyScope = (scope: ValidatedReportScope): ValidatedReportScope => ({
  ...scope, fields: [...scope.fields], build: scope.build ? {
    approvalHash: scope.build.approvalHash,
    projectTuples: scope.build.projectTuples.map(row => ({
      ...row, plotNumber: { ...row.plotNumber }, siteUnit: { ...row.siteUnit },
    })),
  } : null,
});
const copyCatalogue = (catalogue: LongVegetationProjectFieldDefinitions): LongVegetationProjectFieldDefinitions => ({
  ...catalogue, definitions: catalogue.definitions!.map(field => ({ ...field })),
});
const maxRevision = 9223372036854775807n;

export class ProjectScopeSelectionSession {
  private readonly owner: ProjectReportOwner;
  private review: ValidatedReportScope | null = null;
  private definitions: NonNullable<LongVegetationProjectFieldDefinitions['definitions']> = [];
  private fields: string[] = [];
  private dirty = false;
  private busy = false;
  private changingSelection = false;
  private mutationsInFlight = 0;
  private mutationSettlement = 0;
  private authorityUnknown = false;
  private error = '';
  private generation = 0;
  private readonly listeners = new Set<() => void>();

  constructor(owner: ProjectReportOwner, private readonly port: ProjectScopeSessionPort,
    enabled = false, private readonly invalidate: () => void = () => {}) {
    if (enabled !== true) throw new Error('Whole-project scope selection is disabled in this build.');
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project scope session requires complete frozen physical owner paths.');
    this.owner = Object.freeze({
      contextId: owner.contextId, project: owner.project, projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath,
    });
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    listener();
    return () => this.listeners.delete(listener);
  }

  private notify(): void { for (const listener of this.listeners) listener(); }

  view(): ProjectScopeSessionView {
    const draftError = this.review ? selectedUnitFieldError(this.fields, this.definitions) : '';
    const scope = this.review ? copyScope(this.review) : null;
    return {
      scope, definitions: this.definitions.map(field => ({ ...field })), fields: [...this.fields],
      dirty: this.dirty, draftError, error: this.error, busy: this.busy, authorityUnknown: this.authorityUnknown,
      outputReady: !!scope && scope.sourceKind !== 'selected-list' && !!scope.build && scope.buildCurrent &&
        !this.busy && this.mutationsInFlight === 0 && !this.authorityUnknown && !this.error &&
        (scope.sourceKind === 'project-all-plots' || !this.dirty && !draftError),
      closeBlocked: this.busy || this.mutationsInFlight !== 0 || this.authorityUnknown || this.dirty,
    };
  }

  cancel(): void {
    this.invalidate();
    if (this.busy && this.changingSelection || this.mutationsInFlight !== 0) this.authorityUnknown = true;
    this.generation++;
    this.busy = false;
    this.notify();
  }

  edit(fields: string[]): void {
    if (!this.review || this.busy) throw new Error('Load the owned physical Field List before editing and finish or cancel the current read.');
    this.invalidate();
    this.generation++;
    this.fields = [...fields];
    this.dirty = !sameFields(this.fields, this.review.fields);
    this.error = '';
    this.notify();
  }

  undo(): void {
    if (!this.review || this.busy) throw new Error('Load the owned field draft before Undo and finish or cancel the current read.');
    this.invalidate();
    this.generation++;
    this.fields = [...this.review.fields];
    this.dirty = false;
    this.error = '';
    this.notify();
  }

  private begin(changing: boolean): number {
    if (this.busy) throw new Error('Project scope selection is already running.');
    if (changing && (this.authorityUnknown || this.mutationsInFlight !== 0)) {
      throw new Error('Wait for the pending mutation and reload owned project scope before another mutation; do not replay an unknown acknowledgement.');
    }
    this.invalidate();
    const request = ++this.generation;
    this.busy = true;
    this.changingSelection = changing;
    this.error = '';
    this.notify();
    return request;
  }

  private apply(scope: ValidatedReportScope, catalogue: LongVegetationProjectFieldDefinitions, preserveDraft: boolean,
    reconciled = true): void {
    this.review = copyScope(scope);
    this.definitions = catalogue.definitions!.map(field => ({ ...field }));
    if (!preserveDraft || !this.dirty) this.fields = [...scope.fields];
    this.dirty = !sameFields(this.fields, scope.fields);
    this.authorityUnknown = !reconciled || this.mutationsInFlight !== 0;
    this.error = !this.authorityUnknown ? '' : this.mutationsInFlight !== 0 ?
      'A cancelled project scope mutation is still pending. Wait for settlement, then Load owned scope before output or another mutation.' :
      'Project session mutation settled during the scope read. Reload owned scope before output or another mutation.';
  }

  private async validatedBoundary(): Promise<void> {
    if (this.port.validationYield) await this.port.validationYield();
    else await new Promise<void>(resolve => setTimeout(resolve, 0));
  }

  private async mutationResult<T>(operation: () => PromiseLike<T>): Promise<T> {
    this.mutationsInFlight++;
    try {
      return await operation();
    } finally {
      this.mutationsInFlight--;
      this.mutationSettlement++;
      if (this.authorityUnknown && this.mutationsInFlight === 0) {
        this.error = 'Project session mutation settled after cancellation. Reload owned scope before output or another mutation.';
        this.notify();
      }
    }
  }

  async load(): Promise<void> {
    const request = this.begin(false);
    const settlement = this.mutationSettlement, pending = this.mutationsInFlight !== 0;
    try {
      const scope = copyScope(validateReportScope(await this.port.scope(), this.owner, true));
      if (request !== this.generation) return;
      const catalogue = copyCatalogue(validateProjectFieldCatalogue(await this.port.catalogue(), scope, this.owner, true));
      await this.validatedBoundary();
      if (request !== this.generation) return;
      this.apply(scope, catalogue, true, !pending && settlement === this.mutationSettlement);
    } catch (cause) {
      if (request === this.generation) this.error = `Project scope unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }

  async switchTo(sourceKind: ReportSourceScope): Promise<void> {
    if (!this.review || this.busy) throw new Error('Load the current owned report scope before switching.');
    if (!['selected-list', 'project-selected-fields', 'project-all-plots'].includes(sourceKind) ||
        sourceKind === 'project-all-plots' && this.owner.su !== 'None') {
      throw new Error('Choose an explicit available report scope; All plots requires the original None SU context.');
    }
    if (this.authorityUnknown) throw new Error('Reload owned project scope before switching; the previous mutation acknowledgement is unknown.');
    if (sourceKind === this.review.sourceKind) return;
    if (BigInt(this.review.revision) === maxRevision) throw new Error('Report scope revision exhausted; no switch was requested.');
    const before = copyScope(this.review), request = this.begin(true);
    try {
      const scope = copyScope(validateReportScope(await this.mutationResult(() =>
        this.port.switchScope({ revision: before.revision, sourceKind })), this.owner, true));
      if (request !== this.generation) return;
      if (scope.revision !== String(BigInt(before.revision) + 1n) || scope.sourceKind !== sourceKind ||
          scope.build !== null || scope.buildCurrent || !sameFields(scope.fields, before.fields)) {
        throw new Error('Report scope switch acknowledgement contradicts its explicit request or retained independent field draft.');
      }
      const catalogue = copyCatalogue(validateProjectFieldCatalogue(await this.port.catalogue(), scope, this.owner, true));
      await this.validatedBoundary();
      if (request !== this.generation) return;
      this.apply(scope, catalogue, true);
    } catch (cause) {
      if (request === this.generation) {
        this.authorityUnknown = true;
        this.error = `Report scope switch outcome unknown; reload owned scope before retry: ${String(cause)}`;
      }
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }

  async build(): Promise<void> {
    if (!this.review || this.busy || this.authorityUnknown || this.review.sourceKind === 'selected-list' ||
        this.review.sourceKind === 'project-selected-fields' && selectedUnitFieldError(this.fields, this.definitions)) {
      throw new Error('Load and correct the explicit project scope before Build.');
    }
    const sourceKind = this.review.sourceKind, fields = [...this.fields];
    const increments = sourceKind === 'project-selected-fields' ? 2n : 1n;
    if (BigInt(this.review.revision) > maxRevision - increments) throw new Error('Project Build revision exhausted; no field draft was requested.');
    const before = copyScope(this.review), request = this.begin(true);
    try {
      let revision = before.revision;
      if (sourceKind === 'project-selected-fields') {
        const draft = validateReportScope(await this.mutationResult(() => this.port.fields({ revision, fields })), this.owner, true);
        if (request !== this.generation) return;
        if (draft.revision !== String(BigInt(revision) + 1n) || draft.sourceKind !== sourceKind ||
            draft.build !== null || draft.buildCurrent || !sameFields(draft.fields, fields)) {
          throw new Error('Project field draft acknowledgement differs from the explicit ordered request.');
        }
        revision = draft.revision;
      }
      const scope = copyScope(validateReportScope(await this.mutationResult(() => this.port.build(revision)), this.owner, true));
      if (request !== this.generation) return;
      if (scope.revision !== String(BigInt(revision) + 1n) || scope.sourceKind !== sourceKind || !scope.build || !scope.buildCurrent ||
          !sameFields(scope.fields, sourceKind === 'project-selected-fields' ? fields : before.fields)) {
        throw new Error('Project Build acknowledgement lost current authority or changed its independent field draft.');
      }
      const catalogue = copyCatalogue(validateProjectFieldCatalogue(await this.port.catalogue(), scope, this.owner, true));
      await this.validatedBoundary();
      if (request !== this.generation) return;
      this.apply(scope, catalogue, sourceKind === 'project-all-plots');
    } catch (cause) {
      if (request === this.generation) {
        this.authorityUnknown = true;
        this.error = `Project Build outcome unknown; reload owned scope before retry: ${String(cause)}`;
      }
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }
}

export function createProjectScopeSelectionRegistry(
  port: (owner: ProjectReportOwner) => ProjectScopeSessionPort, enabled = false,
  invalidate: (owner: ProjectReportOwner) => void = () => {}) {
  if (enabled !== true) throw new Error('Whole-project scope registry is disabled in this build.');
  const sessions = new Map<string, { owner: ProjectReportOwner; session: ProjectScopeSelectionSession }>();
  return (owner: ProjectReportOwner): ProjectScopeSelectionSession => {
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project scope registry requires complete owned physical paths.');
    const existing = sessions.get(owner.contextId);
    if (existing && !projectReportOwnedScope(existing.owner, owner)) throw new Error('Project scope session identity changed physical paths.');
    if (existing) return existing.session;
    const frozen = { contextId: owner.contextId, project: owner.project, projectPath: owner.projectPath, su: owner.su, suPath: owner.suPath };
    const session = new ProjectScopeSelectionSession(frozen, port({ ...frozen }), enabled, () => invalidate({ ...frozen }));
    sessions.set(frozen.contextId, { owner: frozen, session });
    return session;
  };
}
