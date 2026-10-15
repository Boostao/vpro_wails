import type {
  LongVegetationProjectOptions, LongVegetationProjectPreview, LongVegetationProjectUnitOutcome,
  LongVegetationProjectUnitPreferences, LongVegetationProjectWorkbookReview, LongVegetationSettings,
  VegetationWorkbookOutcome,
} from '../bindings/github.com/boostao/vpro-wails';
import type { ProjectReportOwner, ValidatedProjectPreview } from './longVegetationProjectReport';
import { projectReportOwnedScope } from './longVegetationProjectReport';
import type { CurrentProjectScope, ReportSourceScope } from './longVegetationProjectScope';
import { currentProjectScope, validateProjectOptions, validateProjectPreview } from './longVegetationProjectScope';
import { ProjectScopeSelectionSession, type ProjectScopeSessionPort } from './longVegetationProjectScopeSession';
import { ProjectUnitPreferencesSession, type ProjectUnitSaveRequest } from './longVegetationProjectUnitSession';
import {
  ProjectWorkbookPublicationSession, projectWorkbookReadRequest, validateProjectWorkbookReview,
  type ProjectWorkbookExportRequest, type ProjectWorkbookReadRequest, type ValidatedProjectWorkbookReview,
} from './longVegetationProjectWorkbook.svelte';
import { requireLongVegetationGrouping } from './longVegetationReport';

export type ProjectWorkspaceCapabilities = {
  enabled: boolean; workbook: boolean; lifeform: boolean; strata: boolean; code: boolean; selectedField: boolean;
};
type ProjectReadRequest = { revision: string; approvalHash: string };
export interface ProjectWorkspacePort {
  scope: ProjectScopeSessionPort;
  options(request: ProjectReadRequest): PromiseLike<LongVegetationProjectOptions>;
  preview(request: ProjectReadRequest): PromiseLike<LongVegetationProjectPreview>;
  unitPreferences(request: ProjectReadRequest): PromiseLike<LongVegetationProjectUnitPreferences>;
  saveUnits(request: ProjectUnitSaveRequest): PromiseLike<LongVegetationProjectUnitOutcome>;
  workbookReview(request: ProjectWorkbookReadRequest): PromiseLike<LongVegetationProjectWorkbookReview | null>;
  publishWorkbook(request: ProjectWorkbookExportRequest): PromiseLike<VegetationWorkbookOutcome | null>;
  cancelScopeReads(): void;
  cancelUnitReads(): void;
  cancelReportReads(): void;
  validationYield?(): PromiseLike<void>;
}

class WorkspaceScopeSelection extends ProjectScopeSelectionSession {
  private loadGeneration = 0;
  constructor(owner: ProjectReportOwner, port: ProjectScopeSessionPort,
    private readonly guardRead: () => void, private readonly guardMutation: () => void,
    private readonly reviewed: () => void, invalidate: () => void) {
    super(owner, port, true, invalidate);
  }
  override cancel(): void { this.loadGeneration++; super.cancel(); }
  override async load(): Promise<void> {
    this.guardRead();
    const generation = ++this.loadGeneration;
    await super.load();
    const view = this.view();
    if (generation === this.loadGeneration && !view.busy && !view.error && !view.authorityUnknown) this.reviewed();
  }
  override async switchTo(source: ReportSourceScope): Promise<void> { this.guardMutation(); await super.switchTo(source); }
  override async build(): Promise<void> {
    this.guardMutation();
    const generation = ++this.loadGeneration;
    await super.build();
    const view = this.view();
    if (generation === this.loadGeneration && !view.busy && !view.error && !view.authorityUnknown) this.reviewed();
  }
  override edit(fields: string[]): void { this.guardMutation(); super.edit(fields); }
  override undo(): void { this.guardMutation(); super.undo(); }
}

class WorkspaceUnitPreferences extends ProjectUnitPreferencesSession {
  constructor(owner: ProjectReportOwner, private readonly guardRead: () => void, private readonly guardSave: () => void) {
    super(owner, true);
  }
  override async load(fetch: (request: ProjectReadRequest) => PromiseLike<LongVegetationProjectUnitPreferences>,
    validationYield?: () => PromiseLike<void>): Promise<void> {
    this.guardRead();
    await super.load(fetch, validationYield);
  }
  override async saveProject(port: (request: ProjectUnitSaveRequest) => PromiseLike<LongVegetationProjectUnitOutcome>): Promise<void> {
    this.guardSave();
    await super.saveProject(port);
  }
}

export class ProjectReportWorkspace {
  readonly selection: ProjectScopeSelectionSession;
  readonly units: ProjectUnitPreferencesSession;
  readonly workbook: ProjectWorkbookPublicationSession | null;
  private readonly owner: ProjectReportOwner;
  private readonly capabilities: ProjectWorkspaceCapabilities;
  private readonly listeners = new Set<() => void>();
  private approvalKey = '';
  private generation = 0;
  private readBusy = false;
  private error = '';
  private settings: LongVegetationSettings | null = null;
  private preview: ValidatedProjectPreview | null = null;
  private workbookReview: ValidatedProjectWorkbookReview | null = null;
  private sourceRevalidationRequired = false;

  constructor(owner: ProjectReportOwner, private readonly port: ProjectWorkspacePort, capabilities: ProjectWorkspaceCapabilities) {
    if (capabilities.enabled !== true) throw new Error('Whole-project report workspace is disabled in this build.');
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project report workspace requires complete frozen physical owner paths.');
    this.owner = Object.freeze({ ...owner });
    this.capabilities = Object.freeze({ ...capabilities });
    this.units = new WorkspaceUnitPreferences(this.owner, () => this.requireUnitRead(), () => this.requireUnitMutation());
    this.workbook = capabilities.workbook ? new ProjectWorkbookPublicationSession(this.owner, true) : null;
    this.selection = new WorkspaceScopeSelection(this.owner, {
      scope: () => { this.requireScopeRead(); return port.scope.scope(); },
      catalogue: () => { this.requireScopeRead(); return port.scope.catalogue(); },
      switchScope: request => { this.requireScopeMutation(); return port.scope.switchScope(request); },
      fields: request => { this.requireScopeMutation(); return port.scope.fields(request); },
      build: revision => { this.requireScopeMutation(); return port.scope.build(revision); },
      validationYield: port.scope.validationYield ? () => port.scope.validationYield!() : undefined,
    }, () => this.requireScopeRead(), () => this.requireScopeMutation(), () => {
      this.sourceRevalidationRequired = false;
      this.synchronizeUnitApproval();
      this.notify();
    }, () => this.invalidateOutput());
    this.units.subscribe(() => { this.invalidateOutput(); this.notify(); });
    this.workbook?.subscribe(() => { this.invalidateOutput(); this.notify(); });
    this.selection.subscribe(() => {
      this.invalidateOutput();
      this.synchronizeUnitApproval();
      this.notify();
    });
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    listener();
    return () => this.listeners.delete(listener);
  }
  private notify(): void { for (const listener of this.listeners) listener(); }

  view() {
    const scope = this.selection.view(), units = this.units.projectView(), publication = this.workbook?.view() ?? null;
    const publicationHeld = !!publication && (publication.busy || publication.blocked);
    const actualMode = scope.scope?.sourceKind === 'project-selected-fields' ? 2 :
      scope.scope?.sourceKind === 'project-all-plots' ? 3 : null;
    return {
      scope, units, publication, readBusy: this.readBusy, error: this.error,
      sourceRevalidationRequired: this.sourceRevalidationRequired,
      settings: this.settings ? structuredClone(this.settings) : null,
      preview: this.preview ? structuredClone(this.preview) : null,
      workbookReview: this.workbookReview ? structuredClone(this.workbookReview) : null,
      projectActive: !!scope.scope && scope.scope.sourceKind !== 'selected-list',
      outputReady: !this.sourceRevalidationRequired && scope.outputReady && units.readReady && !units.closeBlocked && actualMode !== null &&
        units.saved?.unitGroups === actualMode && !this.readBusy && !publicationHeld,
      scopeOperationHeld: this.readBusy || publicationHeld || units.readBusy || units.busy || units.blocked,
      scopeMutationHeld: this.readBusy || publicationHeld || units.closeBlocked,
      unitOperationHeld: this.readBusy || publicationHeld || scope.busy,
      unitMutationHeld: this.readBusy || publicationHeld || scope.busy || scope.authorityUnknown ||
        scope.scope?.sourceKind === 'project-selected-fields' && (scope.dirty || !!scope.draftError),
      closeBlocked: this.readBusy || publicationHeld || scope.closeBlocked || units.closeBlocked,
    };
  }

  private invalidateOutput(): void {
    this.generation++;
    this.port.cancelReportReads();
    this.readBusy = false;
    this.settings = null;
    this.preview = null;
    this.workbookReview = null;
  }
  invalidate(): void {
    this.sourceRevalidationRequired = true;
    this.approvalKey = '';
    this.port.cancelScopeReads();
    this.selection.cancel();
    this.port.cancelUnitReads();
    this.units.invalidateObservation();
    this.synchronizeUnitApproval();
    this.invalidateOutput();
    this.notify();
  }
  cancelReportRead(): void { this.invalidateOutput(); this.notify(); }
  cancelScopeReads(): void { this.port.cancelScopeReads(); }
  cancelUnitReads(): void { this.port.cancelUnitReads(); }

  private synchronizeUnitApproval(): void {
    const view = this.selection.view();
    const scope = !this.sourceRevalidationRequired && view.scope && view.scope.sourceKind !== 'selected-list' && view.scope.buildCurrent &&
      view.scope.build && !view.authorityUnknown ? view.scope : null;
    const key = JSON.stringify(scope);
    if (key === this.approvalKey) return;
    const units = this.units.view();
    if (units.busy || units.blocked) {
      this.error = 'Project scope changed while a Unit Groups receipt was pending. Acknowledge it, then reload owned scope before output.';
      return;
    }
    this.port.cancelUnitReads();
    this.units.setApproval(scope);
    this.approvalKey = key;
  }

  private requireScopeRead(): void {
    if (this.view().scopeOperationHeld) throw new Error('Finish or acknowledge project Unit Groups/workbook work before loading scope.');
  }
  private requireScopeMutation(): void {
    if (this.view().scopeMutationHeld) throw new Error('Correct or Undo project Unit Groups, and finish or acknowledge workbook work before changing scope.');
  }
  private requireUnitRead(): void {
    if (this.sourceRevalidationRequired) throw new Error('Reload owned project scope after sibling changes before reading actual saved Unit Groups.');
    if (this.view().unitOperationHeld) throw new Error('Finish project scope/report/workbook work before reading actual saved Unit Groups.');
  }
  private requireUnitMutation(): void {
    if (this.sourceRevalidationRequired) throw new Error('Reload owned project scope after sibling changes before saving Unit Groups.');
    if (this.view().unitMutationHeld) throw new Error('Finish or correct project scope/report/workbook work before saving Unit Groups.');
  }
  private currentOutputScope(): CurrentProjectScope {
    const view = this.view();
    if (!view.outputReady || !view.scope.scope) {
      throw new Error('Project output requires its current Build and an independently loaded, actually saved matching Unit Groups choice.');
    }
    return currentProjectScope(view.scope.scope, this.owner, true);
  }
  private guardSettings(settings: LongVegetationSettings): void {
    requireLongVegetationGrouping(settings, this.capabilities.lifeform, this.capabilities.strata,
      this.capabilities.code, this.capabilities.selectedField, true, true);
  }

  fetchUnitPreferences = (request: ProjectReadRequest): PromiseLike<LongVegetationProjectUnitPreferences> => {
    this.requireUnitRead();
    return this.port.unitPreferences(request);
  };
  saveUnitPreferences = (request: ProjectUnitSaveRequest): PromiseLike<LongVegetationProjectUnitOutcome> => {
    this.requireUnitMutation();
    return this.port.saveUnits(request);
  };

  private async read(kind: 'options' | 'preview' | 'workbook'): Promise<void> {
    const expected = this.currentOutputScope();
    if (kind === 'workbook' && !this.workbook) throw new Error('Project workbook publication is independently disabled in this build.');
    this.invalidateOutput();
    const generation = this.generation;
    this.readBusy = true;
    this.error = '';
    this.notify();
    try {
      const request = { revision: expected.revision, approvalHash: expected.build.approvalHash };
      if (kind === 'options') {
        const wire = await this.port.options(request);
        if (generation !== this.generation) return;
        const value = validateProjectOptions(structuredClone($state.snapshot(wire)), expected, this.owner, true);
        this.guardSettings(value.options.settings);
        await this.validationBoundary();
        if (generation !== this.generation) return;
        this.settings = value.options.settings;
      } else if (kind === 'preview') {
        const wire = await this.port.preview(request);
        if (generation !== this.generation) return;
        const value = validateProjectPreview(structuredClone($state.snapshot(wire)), expected, this.owner, true);
        this.guardSettings(value.preview.settings);
        await this.validationBoundary();
        if (generation !== this.generation) return;
        this.preview = value.preview;
        this.settings = value.preview.settings;
      } else {
        const wire = await this.port.workbookReview(projectWorkbookReadRequest(expected, this.owner, true));
        if (generation !== this.generation) return;
        const value = validateProjectWorkbookReview(wire, expected, this.owner, true);
        this.guardSettings(value.preview.settings);
        await this.validationBoundary();
        if (generation !== this.generation) return;
        const prepared = this.workbook!.prepare(value, expected);
        this.workbookReview = prepared;
        this.preview = prepared.preview;
        this.settings = prepared.preview.settings;
      }
    } catch (cause) {
      if (generation === this.generation) this.error = `Original project ${kind} unavailable; no data or file written: ${String(cause)}`;
    } finally {
      if (generation === this.generation) { this.readBusy = false; this.notify(); }
    }
  }
  private validationBoundary(): PromiseLike<void> {
    return this.port.validationYield?.() ?? new Promise<void>(resolve => setTimeout(resolve, 0));
  }
  loadOptions(): Promise<void> { return this.read('options'); }
  showPreview(): Promise<void> { return this.read('preview'); }
  reviewWorkbook(): Promise<void> { return this.read('workbook'); }

  async publishWorkbook(destination: string): Promise<void> {
    const expected = this.currentOutputScope(), review = this.workbookReview, workbook = this.workbook;
    if (!review || !workbook) throw new Error('Obtain an explicit original project workbook review before publication.');
    await workbook.publish(review, expected, destination, request => this.port.publishWorkbook(request));
  }
}

export function createProjectReportWorkspaceRegistry(
  port: (owner: ProjectReportOwner) => ProjectWorkspacePort, capabilities: ProjectWorkspaceCapabilities) {
  if (capabilities.enabled !== true) throw new Error('Whole-project report registry is disabled in this build.');
  const frozenCapabilities = Object.freeze({ ...capabilities });
  const sessions = new Map<string, { owner: ProjectReportOwner; workspace: ProjectReportWorkspace }>();
  return (owner: ProjectReportOwner): ProjectReportWorkspace => {
    if (!projectReportOwnedScope(owner, owner)) throw new Error('Project workspace registry requires complete owned physical paths.');
    const existing = sessions.get(owner.contextId);
    if (existing && !projectReportOwnedScope(existing.owner, owner)) throw new Error('Project workspace identity changed physical paths.');
    if (existing) return existing.workspace;
    const frozen = Object.freeze({ ...owner });
    const workspace = new ProjectReportWorkspace(frozen, port({ ...frozen }), frozenCapabilities);
    sessions.set(owner.contextId, { owner: frozen, workspace });
    return workspace;
  };
}
