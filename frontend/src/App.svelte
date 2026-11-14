<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { Database } from '@lucide/svelte';
  import { ContextService, LifecycleService, type StartupState, type ContextSelection, type HierarchyNode, type PlotPage, type PlotSummary, type ProjectPlotProfileFilterRequest } from '../bindings/github.com/boostao/vpro-wails';
  import { projectState } from './shared/state';
  import Navigation from './shell/Navigation.svelte';
  import HomeWorkspaceHub from './shell/HomeWorkspaceHub.svelte';
  import StartupRecoveryPanel from './shell/StartupRecoveryPanel.svelte';
  import WorkspaceSidebar from './shell/WorkspaceSidebar.svelte';
  import PlotProfilePanel from './shell/PlotProfilePanel.svelte';
  import PlotsBrowserPanel from './shell/PlotsBrowserPanel.svelte';
  import ResetSampleModal from './shell/ResetSampleModal.svelte';
  import WorkspaceStatusBanners from './shell/WorkspaceStatusBanners.svelte';
  import { summaryVegetationPreferenceSession } from './reports/summaryVegetationPreferenceSessions';
  import { summaryVegetationQualityPreferenceSession } from './reports/summaryVegetationQualityPreferenceSessions';
  import { summaryVegetationPresentationPreferenceSession } from './reports/summaryVegetationPresentationPreferenceSessions';
  import { summaryVegetationFieldListDesktop } from './reports/summaryVegetationFieldListDesktop';
  import { summaryEnvironmentalDesktop, summaryEnvironmentalEnabled } from './reports/summaryVegetationEnvironmentalDesktop';
  import { matrixPublicationSession } from './reports/subzoneMatrixReport';
  import CloseConfirm from './shell/CloseConfirm.svelte';
  import { closeDisposition, type CloseDecision, type EditorCloseState } from './shell/closeLifecycle';
  import { ReadRequests } from './shared/readRequests';
  import { validateProfileNavigation, profileNavigationPage, type ProfileNavigation } from './classification/projectPlotProfileReview';
  import { plotLookupInputError, validateScopedPlotLookup } from './shared/scopedPlotLookup';
  import { tableCSVArchivePublicationSession } from './interchange/tableCSVArchiveExport';
  import { vegetationWorkbookPublicationSession } from './reports/vegetationWorkbook';
  import { projectReportWorkspace } from './reports/longVegetationProjectWorkspaces';
  import { lifeformWorkbookPublicationSession } from './reports/lifeformWorkbook';
  import { siteUnitSummaryWorkbookPublicationSession } from './reports/siteUnitSummaryWorkbook';
  import { extendedSummaryWorkbookPublicationSession } from './reports/siteUnitSummaryExtendedWorkbook';
  import { summaryVegetationWorkbookSession } from './reports/summaryVegetationWorkbook';
  import { subscribePictureCreationActivity } from './pictures/pictureCreationActivity';
  import { subscribePictureLifecycleActivity } from './pictures/pictureLifecycleActivity';
  import { subscribeTwoPageSoilActivity } from './entry/twoPageSoilActivity';
  import { subscribeTwoPageVegetationActivity } from './entry/twoPageVegetationActivity';
  import { subscribeExternalSUEnvironmentActivity } from './classification/externalSUEnvironment';
  import { classifyGlobalHelpShortcut, defaultHelpPort, resolveContextualHelpTarget, type HelpDrawerTab, type HelpMenuAction, type WhatsNewStatus } from './help/helpSystem';

  const pageSize = 25;
  let page = $state<PlotPage | null>(null);
  let hierarchyNodes = $state<HierarchyNode[]>([]);
  let selected = $state<PlotSummary | null>(null);
  let profileNavigation = $state<ProfileNavigation | null>(null);
  const profileIndex = $derived(profileNavigation?.plots.findIndex(plot => plot.plotNumber === editorPlotNumber) ?? -1);
  let editorPlotNumber = $state<string | undefined>(undefined);
  let editorEntryForm = $state<'fs882' | 'sivi'>('fs882');
  const editorLabel = $derived(editorEntryForm === 'sivi' ? 'SIVI / FS1333' : 'FS882');
  const siviStandaloneEnabled = true;
  let offset = $state(0), busy = $state(false), error = $state('');
  let request = 0, hierarchyRequest = 0, stateRequest = 0;
  const plotReads = new ReadRequests(), hierarchyReads = new ReadRequests();
  const stateReads = new ReadRequests();
  onDestroy(() => {
    request++; hierarchyRequest++; stateRequest++;
    secondaryPanelBusy = false; siviModalBusy = false; workbenchDirtyLabel = '';
    plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
  });
  let view = $state<'home' | 'plots' | 'geospatial-map' | 'diagnostic-synthesis' | 'r-sidecar' | 'hierarchy' | 'fs882' | 'datasheet' | 'classification-explorer' | 'project-admin' | 'lump-editor' | 'interchange-hub' | 'long-environment' | 'summary-environment' | 'long-vegetation' | 'summary-vegetation' | 'lifeform-summary' | 'subzone-matrix' | 'table-csv' | 'user-log' | 'plot-locations' | 'google-earth-review' | 'picture-manager' | 'plot-label-preview'>('home');
  let projectAdminTab = $state<'lifecycle' | 'compliance' | 'optimizeveg' | 'merge' | 'cloudsync'>('lifecycle');
  let projectAdminTabNonce = $state(0), newSiviModalOpen = $state(false);
  let helpDrawerOpen = $state(false), helpDrawerTopicId = $state('site.plot_number'), helpDrawerTab = $state<HelpDrawerTab>('inspector');
  let tourOpen = $state(false), tourInitialId = $state<string | null>(null);
  let shortcutsModalOpen = $state(false), whatsNewModalOpen = $state(false), aboutModalOpen = $state(false);
  let resetSampleConfirmOpen = $state(false), resetSampleNotice = $state('');
  let whatsNewStatus = $state.raw<WhatsNewStatus | null>(null), whatsNewPromptDismissed = $state(false);
  let pictureManagerPlot = $state(''), plotLabelNumber = $state(''), contextExpanded = $state(false);
  let localEditorBusy = $state(false), matrixPublicationBusy = $state(false), pictureCreationBusy = $state(false);
  let pictureLifecycleBusy = $state(false), twoPageSoilBusy = $state(false), twoPageVegetationBusy = $state(false);
  let externalSUEnvironmentBusy = $state(false), projectReportBusy = $state(false), summaryPreferenceBusy = $state(false);
  let summaryFieldListBusy = $state(false), summaryEnvironmentalBusy = $state(false);
  const editorBusy = $derived(localEditorBusy || matrixPublicationBusy || pictureCreationBusy || pictureLifecycleBusy || twoPageSoilBusy || twoPageVegetationBusy || externalSUEnvironmentBusy || summaryPreferenceBusy || summaryFieldListBusy || summaryEnvironmentalBusy || projectReportBusy);
  let secondaryPanelBusy = $state(false), siviModalBusy = $state(false), workbenchDirtyLabel = $state('');
  const companionPanelBusy = $derived(secondaryPanelBusy || siviModalBusy);
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeHierarchy === 'None' || !state.hierarchyPath) { matrixPublicationBusy = false; return; }
    const session = matrixPublicationSession({ contextId: state.contextId, hierarchy: state.activeHierarchy, hierarchyPath: state.hierarchyPath });
    return session.subscribe(() => {
      const publication = session.view();
      matrixPublicationBusy = publication.busy || publication.blocked;
      if (publication.blocked && view === 'home') view = 'subzone-matrix';
    });
  });
  $effect(() => {
    if (!summaryEnvironmentalEnabled) return;
    return summaryEnvironmentalDesktop.subscribe(() => {
      summaryEnvironmentalBusy = summaryEnvironmentalDesktop.activity().busy;
      if (summaryEnvironmentalBusy && view === 'home') view = 'summary-vegetation';
    });
  });
  $effect(() => {
    if (!summaryVegetationFieldListDesktop.enabled) return;
    return summaryVegetationFieldListDesktop.subscribe(() => {
      const activity = summaryVegetationFieldListDesktop.activity();
      summaryFieldListBusy = activity.busy || activity.blocked;
      if (summaryFieldListBusy && view === 'home') view = 'summary-vegetation';
    });
  });
  $effect(() => {
    const state = $projectState;
    if ((!summaryVegetationPreferenceSession && !summaryVegetationQualityPreferenceSession && !summaryVegetationPresentationPreferenceSession) ||
        !state?.contextId || !state.projectPath || !state.suPath) { summaryPreferenceBusy = false; return; }
    const owner = { contextId: state.contextId, project: state.activeProject, projectPath: state.projectPath, su: state.activeSU, suPath: state.suPath };
    const preferences = summaryVegetationPreferenceSession?.(owner), quality = summaryVegetationQualityPreferenceSession?.(owner),
      presentation = summaryVegetationPresentationPreferenceSession?.(owner);
    const changed = () => {
      const views = [preferences?.view(), quality?.view(), presentation?.view()];
      summaryPreferenceBusy = views.some(current => !!current && (current.busy || current.blocked || !!current.draftError));
      if (summaryPreferenceBusy && view === 'home') view = 'summary-vegetation';
    };
    const r1 = preferences?.subscribe(changed), r2 = quality?.subscribe(changed), r3 = presentation?.subscribe(changed);
    return () => { r1?.(); r2?.(); r3?.(); };
  });
  $effect(() => {
    const state = $projectState;
    if (!projectReportWorkspace || !state?.contextId || !state.projectPath || state.activeSU !== 'None' && !state.suPath) { projectReportBusy = false; return; }
    const workspace = projectReportWorkspace({ contextId: state.contextId,
      project: state.activeProject, projectPath: state.projectPath, su: state.activeSU, suPath: state.suPath ?? '' });
    return workspace.subscribe(() => {
      projectReportBusy = workspace.view().closeBlocked;
      if (projectReportBusy && view === 'home') view = 'long-vegetation';
    });
  });
  $effect(() => subscribeTwoPageSoilActivity(activity => { twoPageSoilBusy = activity.busy || activity.blocked; }));
  $effect(() => subscribeTwoPageVegetationActivity(activity => { twoPageVegetationBusy = activity.busy || activity.blocked; }));
  $effect(() => subscribeExternalSUEnvironmentActivity(activity => { externalSUEnvironmentBusy = activity.busy || activity.blocked; }));
  $effect(() => {
    const state = $projectState;
    return subscribePictureLifecycleActivity(activity => {
      pictureLifecycleBusy = activity.busy || activity.blocked;
      if (activity.owner && activity.owner.contextId === state?.contextId && activity.owner.project === state.activeProject && view === 'home') {
        pictureManagerPlot = activity.owner.plotNumber; view = 'picture-manager';
      }
    });
  });
  $effect(() => {
    const state = $projectState;
    return subscribePictureCreationActivity(activity => {
      pictureCreationBusy = activity.busy || activity.blocked;
      if (activity.owner && activity.owner.contextId === state?.contextId && activity.owner.project === state.activeProject && view === 'home') {
        pictureManagerPlot = activity.owner.plotNumber; view = 'picture-manager';
      }
    });
  });
  let archivePublicationBusy = $state(false), vegetationPublicationBusy = $state(false), lifeformPublicationBusy = $state(false);
  let summaryEnvironmentPublicationBusy = $state(false), summaryVegetationPublicationBusy = $state(false);
  const summaryPublicationBusy = $derived(summaryEnvironmentPublicationBusy || summaryVegetationPublicationBusy);
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { vegetationPublicationBusy = false; return; }
    const session = vegetationWorkbookPublicationSession({ contextId: state.contextId,
      project: state.activeProject, projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath }, true);
    return session.subscribe(() => {
      const publication = session.view();
      vegetationPublicationBusy = publication.busy || publication.blocked;
      if (publication.blocked && view === 'home') view = 'long-vegetation';
    });
  });
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { lifeformPublicationBusy = false; return; }
    const session = lifeformWorkbookPublicationSession({ contextId: state.contextId,
      project: state.activeProject, projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath });
    return session.subscribe(() => {
      const publication = session.view();
      lifeformPublicationBusy = publication.busy || publication.blocked;
      if (publication.blocked && view === 'home') view = 'lifeform-summary';
    });
  });
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId) { archivePublicationBusy = false; return; }
    const session = tableCSVArchivePublicationSession({ contextId: state.contextId, project: state.activeProject, projectPath: state.projectPath ?? '' });
    return session.subscribe(() => {
      const publication = session.view();
      archivePublicationBusy = publication.busy || publication.blocked;
      if (publication.blocked && view === 'home') view = 'table-csv';
    });
  });
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { summaryEnvironmentPublicationBusy = false; return; }
    const owner = { contextId: state.contextId, project: state.activeProject, projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath };
    const sessions = [siteUnitSummaryWorkbookPublicationSession(owner), extendedSummaryWorkbookPublicationSession(owner)];
    const update = () => {
      const states = sessions.map(session => session.view());
      const publication = { busy: states.some(state => state.busy), blocked: states.some(state => state.blocked) };
      summaryEnvironmentPublicationBusy = publication.busy || publication.blocked;
      if (publication.blocked && view === 'home') view = 'summary-environment';
    };
    const unsubscribe = sessions.map(session => session.subscribe(update));
    return () => { for (const stop of unsubscribe) stop(); };
  });
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { summaryVegetationPublicationBusy = false; return; }
    const session = summaryVegetationWorkbookSession({ contextId: state.contextId, project: state.activeProject,
      projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath });
    return session.subscribe(() => {
      summaryVegetationPublicationBusy = session.held();
      if (session.held() && view === 'home') view = 'summary-vegetation';
    });
  });
  let editor: {
    getCloseState: () => EditorCloseState; saveForClose: () => Promise<boolean>;
    canOpenProjectMetadata: () => boolean; openProjectMetadata: () => void;
    canOpenMetadataInPlace?: () => boolean;
  } | undefined = $state();
  let fs882ModuleLoaded = $state(false);
  const editorInitializing = $derived(view === 'fs882' && !editor && fs882ModuleLoaded);

  async function loadFS882Editor() {
    const module = await import('./entry/FS882Form.svelte');
    fs882ModuleLoaded = true;
    return module;
  }

  let closeRequest = $state(''), closeWorking = $state(false), closeError = $state('');
  let pendingTransition = $state<{ contextId: string; run: (contextId: string) => Promise<void> } | null>(null);
  let transitionWorking = $state(false), transitionError = $state('');
  let plotProfilePanel: {
    reviewProfileSU: (proposal: ProjectPlotProfileFilterRequest, origin: string) => Promise<void>;
    clearCachedSources: () => void; resetAllReviews: () => void;
  } | undefined = $state();
  const plotFindEnabled = true;
  let plotFindOpen = $state(false), plotFindQuery = $state(''), plotFindError = $state('');
  let plotFindResult = $state<Awaited<ReturnType<typeof ContextService.LookupScopedPlot>> | null>(null);
  let startupInfo = $state<StartupState | null>(null), startupFailure = $state<StartupState | null>(null);
  const buildMode = $derived(startupInfo?.buildMode ?? ''), appVersion = $derived(startupInfo?.appVersion ?? '');
  const shortBuildCommit = $derived.by(() => {
    const raw = (startupInfo?.gitCommit ?? '').trim();
    return /^[0-9a-f]{8,40}$/i.test(raw) ? raw.slice(0, 7) : raw;
  });
  const releaseChannel = $derived(startupInfo?.releaseChannel ?? 'stable');
  let startupBackupNotice = $state('');
  const closeState = $derived(view === 'fs882' ? editor?.getCloseState() : undefined);

  async function handleCloseRequest(requestId: string) {
    if (closeWorking || closeRequest === requestId) return;
    try {
      if (await LifecycleService.GetPendingCloseRequest() !== requestId) return;
      if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
      await tick();
      const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
      const disposition = closeDisposition(state, busy || editorBusy || transitionWorking || archivePublicationBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || pendingTransition !== null || editorInitializing || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy));
      if (disposition === 'busy') {
        await LifecycleService.CancelClose(requestId);
        error = (typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel) ? 'Save or discard unsaved workbench edits before closing VPro.' : 'Wait for the current operation to finish before closing VPRO.';
      } else if (typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel) {
        await LifecycleService.CancelClose(requestId);
        error = 'Save or discard unsaved workbench edits before closing VPro.';
      } else if (disposition === 'clean') {
        await LifecycleService.ConfirmClose(requestId);
      } else { closeError = ''; closeRequest = requestId; }
    } catch (cause) { error = `Window close request failed: ${String(cause)}`; }
  }

  async function respondToClose(decision: CloseDecision) {
    if (!closeRequest || closeWorking) return;
    const requestId = closeRequest;
    closeWorking = true; closeError = '';
    try {
      if (decision === 'cancel') { await LifecycleService.CancelClose(requestId); closeRequest = ''; return; }
      const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
      if (closeDisposition(state, busy || editorBusy || transitionWorking || archivePublicationBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || pendingTransition !== null || editorInitializing || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
        closeError = 'Wait for the current operation to finish before closing VPRO.'; return;
      }
      if (decision === 'save') {
        if (!editor || !await editor.saveForClose()) {
          closeError = editor?.getCloseState().error ?? 'The draft could not be saved. VPRO remains open.'; return;
        }
      }
      await LifecycleService.ConfirmClose(requestId);
    } catch (cause) { closeError = `VPRO remains open: ${String(cause)}`; }
    finally { closeWorking = false; }
  }

  async function requestTransition(run: (contextId: string) => Promise<void>, ownedAction?: () => boolean) {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    await tick();
    if (ownedAction && !ownedAction()) { error = 'Owned action or editor availability changed; no editor replaced.'; return; }
    if (!ownedAction && typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel) {
      error = 'Save or discard unsaved workbench edits before changing context or view.'; return;
    }
    const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
    if (closeDisposition(ownedAction ? null : state, busy || !ownedAction && editorBusy || transitionWorking || archivePublicationBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || pendingTransition !== null || !!closeRequest || editorInitializing || (!ownedAction && typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
      error = 'Wait for the current operation or decision before changing context.'; return;
    }
    const contextId = $projectState?.contextId;
    if (!contextId) { error = 'A project context must finish loading before navigation.'; return; }
    transitionError = '';
    if (state?.unsaved && !ownedAction) { pendingTransition = { contextId, run }; return; }
    try { await run(contextId); } catch (cause) { error = String(cause); }
  }

  async function respondToTransition(decision: CloseDecision) {
    if (!pendingTransition || transitionWorking) return;
    if (decision === 'cancel') { pendingTransition = null; transitionError = ''; return; }
    const pending = pendingTransition;
    transitionWorking = true; transitionError = '';
    try {
      const state = editor?.getCloseState();
      if (!state || closeDisposition(state, busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
        transitionError = 'Wait for the current editor operation to finish.'; return;
      }
      if (decision === 'save' && state.unsaved && !await editor?.saveForClose()) {
        transitionError = editor?.getCloseState().error ?? 'The draft could not be saved; context and draft remain open.'; return;
      }
      await pending.run(pending.contextId);
      pendingTransition = null;
    } catch (cause) { transitionError = `Context remains unchanged: ${String(cause)}`; }
    finally { transitionWorking = false; }
  }

  function navigate(next: typeof view) {
    if (next === view && next !== 'project-admin') return;
    void requestTransition(async () => {
      error = '';
      if (next === 'project-admin') { projectAdminTab = 'lifecycle'; projectAdminTabNonce++; }
      if (next === 'fs882') { editorPlotNumber = selected?.plotNumber; editorEntryForm = 'fs882'; }
      if (next === 'picture-manager') pictureManagerPlot = selected?.plotNumber ?? '';
      if (next === 'plot-label-preview') plotLabelNumber = (view === 'fs882' ? editorPlotNumber : selected?.plotNumber) ?? '';
      view = next;
    });
  }

  async function openSIVI() {
    if (!siviStandaloneEnabled || !selected || !$projectState?.contextId) {
      error = 'Standalone SIVI requires its independent entry gate, parent review and a selected owned plot.'; return;
    }
    const plot = selected.plotNumber;
    await requestTransition(async () => {
      if (selected?.plotNumber !== plot) throw new Error('Selected plot changed before opening SIVI; no owner replaced.');
      editorPlotNumber = plot; editorEntryForm = 'sivi'; view = 'fs882'; error = '';
    });
  }

  async function openProjectMetadata() {
    if (view !== 'fs882' || !editor || !$projectState?.contextId || !editor.canOpenProjectMetadata()) {
      error = 'Project metadata requires its existing gate and a clean, unlocked open plot; finish or Undo drafts first.'; return;
    }
    const owner = editor, contextId = $projectState.contextId;
    const ownedAction = owner.canOpenMetadataInPlace?.()
      ? () => view === 'fs882' && editor === owner && $projectState?.contextId === contextId && owner.canOpenMetadataInPlace?.() === true
      : undefined;
    await requestTransition(async currentContext => {
      if (view !== 'fs882' || editor !== owner || currentContext !== contextId || $projectState?.contextId !== contextId || !owner.canOpenProjectMetadata()) {
        throw new Error('Project metadata owner or availability changed; no editor replaced.');
      }
      owner.openProjectMetadata();
    }, ownedAction);
  }

  async function resolveNavigation(proposal: ProjectPlotProfileFilterRequest, contextId: string): Promise<ProfileNavigation> {
    const response = await plotReads.track(ContextService.ResolveProjectPlotProfileNavigation(contextId, proposal));
    if ($projectState?.contextId !== contextId) throw new Error('Project/SU context changed; navigation was not published.');
    return validateProfileNavigation(response, proposal, contextId);
  }

  async function applyProfileNavigation(proposal: ProjectPlotProfileFilterRequest, origin: string) {
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Profile proposal belongs to an old context.');
      busy = true;
      try {
        await tick();
        const next = await resolveNavigation(proposal, contextId);
        const nextPage = profileNavigationPage(next, contextId, 0, pageSize);
        request++; plotReads.cancelAll();
        profileNavigation = next; page = nextPage; offset = 0; selected = null;
        editorPlotNumber = undefined; view = 'plots'; error = '';
      } finally { busy = false; }
    });
  }

  async function clearProfileNavigation() {
    await requestTransition(async contextId => {
      busy = true;
      try {
        await tick();
        const next = await plotReads.track(ContextService.ListPlots(0, pageSize));
        const state = await stateReads.track(ContextService.GetState());
        if (state.contextId !== contextId || $projectState?.contextId !== contextId) throw new Error('Context changed before clearing navigation.');
        request++; plotReads.cancelAll();
        profileNavigation = null; page = next; offset = 0; selected = null;
        editorPlotNumber = undefined; view = 'plots'; error = '';
      } finally { busy = false; }
    });
  }

  async function lookupPlot(number: string, contextId: string) {
    const invalid = plotLookupInputError(number);
    if (invalid) throw new Error(invalid);
    const source = profileNavigation;
    if (source) {
      if (source.contextId !== contextId) throw new Error('Reviewed profile navigation belongs to an old context.');
      const next = await resolveNavigation(source.proposal, contextId);
      if (!next.plots.some(plot => plot.plotNumber === number)) throw new Error('Literal plot is outside the reviewed profile navigation; current filter is unchanged.');
    }
    const result = await stateReads.track(ContextService.LookupScopedPlot(contextId, { plotNumber: number }));
    if ($projectState?.contextId !== contextId) throw new Error('Find belongs to an old context; current editor is unchanged.');
    return validateScopedPlotLookup(result, number, contextId);
  }

  async function findScopedPlot() {
    const contextId = $projectState?.contextId;
    if (!plotFindEnabled || !contextId || busy || editorBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) return;
    const number = plotFindQuery;
    busy = true; plotFindError = ''; plotFindResult = null;
    try { await tick(); plotFindResult = await lookupPlot(number, contextId); }
    catch (cause) { plotFindError = `No navigation or data change; find failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function openFoundPlot() {
    if (!plotFindEnabled || !plotFindResult || plotFindResult.contextId !== $projectState?.contextId) {
      plotFindError = 'Find an available plot in the current scope before opening it.'; return;
    }
    const proposal = $state.snapshot(plotFindResult);
    if (proposal.plot.plotNumber === editorPlotNumber) { plotFindError = 'Found plot is already open; no draft or navigation change.'; return; }
    await requestTransition(async contextId => {
      if (contextId !== proposal.contextId) throw new Error('Found plot belongs to an old context; find again.');
      busy = true; plotFindError = '';
      try {
        await tick();
        const current = await lookupPlot(proposal.plot.plotNumber, contextId);
        if (JSON.stringify(current) !== JSON.stringify(proposal)) throw new Error('Found plot summary changed since review; find again before opening.');
        editorPlotNumber = current.plot.plotNumber; view = 'fs882'; selected = null;
        plotFindOpen = false; plotFindResult = null; error = '';
      } catch (cause) {
        plotFindError = `Navigation failed; current editor and filters retained: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function navigateProfilePlot(delta: number) {
    const source = profileNavigation, target = source?.plots[profileIndex + delta]?.plotNumber;
    if (!source || !target) { error = 'Select an available plot in the reviewed navigation recordset.'; return; }
    await requestTransition(async contextId => {
      if (contextId !== source.contextId) throw new Error('Profile navigation belongs to an old context.');
      busy = true;
      try {
        await tick();
        const next = await resolveNavigation(source.proposal, contextId);
        if (!next.plots.some(plot => plot.plotNumber === target)) throw new Error('Reviewed navigation target is no longer available.');
        profileNavigation = next; editorPlotNumber = target; view = 'fs882'; error = '';
      } finally { busy = false; }
    });
  }

  async function loadPlots(nextOffset: number) {
    if (projectReportBusy || twoPageVegetationBusy || twoPageSoilBusy || pictureCreationBusy || pictureLifecycleBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || summaryPreferenceBusy || externalSUEnvironmentBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || (view === 'long-vegetation' || view === 'lifeform-summary' || view === 'summary-environment' || view === 'summary-vegetation' || view === 'user-log' || view === 'subzone-matrix') && editorBusy) return;
    const current = ++request;
    plotReads.cancelAll(); busy = true; error = '';
    try {
      const source = profileNavigation;
      const next = source ? await resolveNavigation(source.proposal, source.contextId) : null;
      const result = next ? profileNavigationPage(next, next.contextId, nextOffset, pageSize) :
        await plotReads.track(ContextService.ListPlots(nextOffset, pageSize));
      if (current !== request) return;
      if (next) profileNavigation = next;
      page = result; offset = nextOffset; selected = null;
    } catch (cause) { if (current === request) error = String(cause); }
    finally { if (current === request) busy = false; }
  }

  async function refresh() {
    if (projectReportBusy || twoPageVegetationBusy || twoPageSoilBusy || pictureCreationBusy || pictureLifecycleBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || summaryPreferenceBusy || externalSUEnvironmentBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || (view === 'long-vegetation' || view === 'lifeform-summary' || view === 'summary-environment' || view === 'summary-vegetation' || view === 'user-log' || view === 'subzone-matrix') && editorBusy) return;
    const current = ++stateRequest;
    stateReads.cancelAll(); busy = true; error = '';
    try {
      const startup = await stateReads.track(LifecycleService.GetStartupState());
      if (current !== stateRequest) return;
      startupInfo = startup;
      if (!startup.ready) { startupFailure = startup; busy = false; return; }
      startupFailure = null;
      const state = await stateReads.track(ContextService.GetState());
      if (current !== stateRequest) return;
      projectState.set(state);
      if (profileNavigation && profileNavigation.contextId !== state.contextId) profileNavigation = null;
      await loadPlots(0);
      if (current !== stateRequest) return;
      await loadHierarchy();
    } catch (cause) { if (current === stateRequest) { error = String(cause); busy = false; } }
  }

  async function loadHierarchy() {
    if (projectReportBusy || twoPageVegetationBusy || twoPageSoilBusy || pictureCreationBusy || pictureLifecycleBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || summaryPreferenceBusy || externalSUEnvironmentBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || (view === 'long-vegetation' || view === 'lifeform-summary' || view === 'summary-environment' || view === 'summary-vegetation' || view === 'user-log' || view === 'subzone-matrix') && editorBusy) return;
    const current = ++hierarchyRequest;
    hierarchyReads.cancelAll();
    try {
      const nodes = await hierarchyReads.track(ContextService.GetHierarchyNodes());
      if (current === hierarchyRequest) hierarchyNodes = nodes ?? [];
    } catch (cause) { if (current === hierarchyRequest) throw cause; }
  }

  function cancelAllReads() {
    request++; hierarchyRequest++; stateRequest++;
    plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
  }

  async function reviewProfileSU(proposal: ProjectPlotProfileFilterRequest, origin: string) {
    if (!plotProfilePanel) throw new Error('Plot profile panel is not ready.');
    await plotProfilePanel.reviewProfileSU(proposal, origin);
  }

  async function switchContext(contextId: string, selection: ContextSelection, next: 'plots' | 'hierarchy') {
    cancelAllReads(); busy = true; error = '';
    try {
      const state = await ContextService.SwitchContext(contextId, selection);
      request++; projectState.set(state); profileNavigation = null; plotProfilePanel?.clearCachedSources();
      page = null; selected = null; hierarchyNodes = []; editorPlotNumber = undefined; view = next;
      await loadPlots(0);
      if (next === 'hierarchy') {
        try { await loadHierarchy(); }
        catch (cause) { error = `Context changed, but hierarchy refresh failed: ${String(cause)}`; }
      }
    } finally { busy = false; }
  }

  async function handleAccessActivated() {
    profileNavigation = null; plotProfilePanel?.resetAllReviews(); editorPlotNumber = undefined; view = 'plots';
    await loadPlots(0);
  }

  function openContextualHelp(topicOverride?: string, tabOverride: HelpDrawerTab = 'inspector') {
    const activeEl = typeof document !== 'undefined' ? document.activeElement : null;
    helpDrawerTopicId = topicOverride ?? resolveContextualHelpTarget(activeEl, view, editorEntryForm);
    helpDrawerTab = tabOverride; helpDrawerOpen = true;
  }

  function handleGlobalHelpKeydown(event: KeyboardEvent) {
    if (startupFailure !== null) return;
    const activeEl = typeof document !== 'undefined' ? (document.activeElement as HTMLElement | null) : null;
    const action = classifyGlobalHelpShortcut(event, activeEl);
    if (action === 'f1') { event.preventDefault(); openContextualHelp(); return; }
    if (action === 'shortcuts') { event.preventDefault(); shortcutsModalOpen = !shortcutsModalOpen; return; }
    if (event.key === 'Escape' && helpDrawerOpen && !shortcutsModalOpen && !whatsNewModalOpen && !aboutModalOpen && !tourOpen && !resetSampleConfirmOpen && !closeRequest && !pendingTransition) {
      event.preventDefault(); helpDrawerOpen = false;
    }
  }

  function handleHelpMenuAction(action: HelpMenuAction) {
    resetSampleNotice = '';
    if (action === 'guide') openContextualHelp(undefined, 'inspector');
    else if (action === 'tours') { tourInitialId = null; tourOpen = true; }
    else if (action === 'shortcuts') shortcutsModalOpen = true;
    else if (action === 'whats-new') whatsNewModalOpen = true;
    else if (action === 'reset-sample') resetSampleConfirmOpen = true;
    else if (action === 'about') aboutModalOpen = true;
  }

  async function acknowledgeUnreadWhatsNew() {
    if (!whatsNewStatus?.latestVersion) return;
    try { whatsNewStatus = await defaultHelpPort.acknowledgeWhatsNew(whatsNewStatus.latestVersion); }
    catch (cause) { error = `Could not acknowledge What's New: ${String(cause)}`; }
  }

  async function confirmResetWorkspaceToSample() {
    resetSampleConfirmOpen = false;
    const ownedDiscardOnSample = $projectState?.activeProject === 'Sample' ? () => !editorBusy && !companionPanelBusy : undefined;
    await requestTransition(async (contextId) => {
      busy = true; error = ''; resetSampleNotice = '';
      try {
        const nextState = await defaultHelpPort.resetWorkspaceToSample(contextId);
        cancelAllReads(); projectState.set(nextState); profileNavigation = null; plotProfilePanel?.clearCachedSources();
        selected = null; page = null; hierarchyNodes = []; editorPlotNumber = undefined; resetSampleConfirmOpen = false; view = 'plots';
        resetSampleNotice = 'Workspace reset to pristine Sample.db (Project: Sample, SU: Sample, Hierarchy: Sample). User project .db files were preserved.';
        await loadPlots(0);
        try { await loadHierarchy(); }
        catch (cause) { error = `Sample workspace activated, but hierarchy refresh failed: ${String(cause)}`; }
      } finally { busy = false; }
    }, ownedDiscardOnSample);
  }

  onMount(() => {
    const stopCloseEvents = Events.On('vpro:close-request', (event) => {
      const requestId: unknown = event.data;
      if (typeof requestId !== 'string' || !requestId) { error = 'Invalid native window close request; VPRO remains open.'; return; }
      void handleCloseRequest(requestId);
    });
    void LifecycleService.GetPendingCloseRequest().then((requestId) => { if (requestId) void handleCloseRequest(requestId); })
      .catch((cause) => { error = `Window close handler could not initialise: ${String(cause)}`; });
    void defaultHelpPort.getWhatsNewStatus().then((st) => { whatsNewStatus = st; }).catch(() => {});
    void refresh();
    return () => { request++; stopCloseEvents(); };
  });
</script>

<svelte:window onkeydown={handleGlobalHelpKeydown} />

<div class="app-shell">
  <header class="topbar">
    <div class="brand">
      <span class="brand-symbol" aria-hidden="true">V</span><span>VPro</span>
      <WorkspaceStatusBanners region="badge" {buildMode} {appVersion} {shortBuildCommit} {releaseChannel}
        startupFailed={startupFailure !== null} onopenabout={() => { aboutModalOpen = true; }} />
    </div>
    <button class="context-toggle" type="button" disabled={startupFailure !== null} aria-expanded={contextExpanded} aria-controls="project-context" onclick={() => contextExpanded = !contextExpanded}>Projects &amp; context</button>
    {#if !startupFailure}<Navigation {view} onnavigate={navigate}
      siviAvailable={siviStandaloneEnabled && !!selected && !!$projectState?.contextId} onopensivi={() => void openSIVI()}
      newSiviAvailable={!!$projectState?.contextId && !busy && !editorBusy && !companionPanelBusy && !transitionWorking} onopennewsivi={() => { newSiviModalOpen = true; }}
      metadataAvailable={view === 'fs882' && !!$projectState?.contextId && !!editor?.canOpenProjectMetadata()} onopenmetadata={() => void openProjectMetadata()}
      onopencloudsync={() => { void requestTransition(async () => { projectAdminTab = 'cloudsync'; projectAdminTabNonce++; view = 'project-admin'; error = ''; }); }}
      whatsNewUnreadCount={whatsNewStatus?.unreadCount ?? 0} resetSampleAvailable={!!$projectState?.contextId && !busy && !editorBusy && !companionPanelBusy && !transitionWorking}
      {releaseChannel} onhelpaction={handleHelpMenuAction} />{/if}
    <div class="topbar-context"><Database size={16} strokeWidth={1.8} /><span>{$projectState?.activeProject ?? 'No project'}</span></div>
  </header>

  <div class="workspace" class:context-collapsed={!contextExpanded || startupFailure !== null}>
    <WorkspaceSidebar {busy} {editorBusy} {companionPanelBusy} {transitionWorking} {requestTransition} {switchContext} onAccessActivated={handleAccessActivated} />

    <main class="content">
      <WorkspaceStatusBanners region="banners" startupFailed={startupFailure !== null} {whatsNewStatus} {whatsNewPromptDismissed} {resetSampleNotice}
        onopenwhatsnew={() => { whatsNewModalOpen = true; }} onacknowledgewhatsnew={() => void acknowledgeUnreadWhatsNew()}
        ondismisswhatsnew={() => { whatsNewPromptDismissed = true; }} ondismissresetnotice={() => { resetSampleNotice = ''; }} />
      <PlotProfilePanel bind:this={plotProfilePanel} bind:view bind:busy bind:error bind:editorPlotNumber bind:profileNavigation bind:selected bind:page
        {editorBusy} {vegetationPublicationBusy} {lifeformPublicationBusy} {summaryPublicationBusy} {transitionWorking} {pendingTransition} {closeRequest}
        {profileIndex} {stateReads} {requestTransition} {loadPlots} {navigateProfilePlot} {clearProfileNavigation} {cancelAllReads} />
      <WorkspaceStatusBanners region="banners" startupFailed={true} diagnostics={$projectState?.diagnostics ?? []} />
      {#if startupFailure}
        <StartupRecoveryPanel {startupFailure} {busy} onrecovered={async (notice) => { startupBackupNotice = notice; startupFailure = null; await refresh(); }} />
      {:else if view === 'home'}
        <div class="content-heading"><div><div class="eyebrow">VPro</div><h1>Home</h1></div></div>
        {#if startupBackupNotice}
          <div role="status" data-startup-backup-notice class="mb-3 p-2.5 border rounded bg-emerald-50 text-emerald-900 text-xs flex items-center justify-between gap-2">
            <span>{startupBackupNotice}</span>
            <button type="button" class="underline text-xs" onclick={() => { startupBackupNotice = ''; }}>Dismiss</button>
          </div>
        {/if}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <HomeWorkspaceHub activeProject={$projectState?.activeProject ?? 'No project selected'} projectPath={$projectState?.projectPath ?? ''}
          activeSU={$projectState?.activeSU ?? 'None'} activeHierarchy={$projectState?.activeHierarchy ?? 'None'} plotCount={page?.total ?? 0}
          {buildMode} {appVersion} gitCommit={startupInfo?.gitCommit ?? ''} {releaseChannel}
          onnavigate={(nextView) => navigate(nextView)} onhelpaction={(action) => handleHelpMenuAction(action)} />
      {:else if view === 'fs882'}
        <div class="h-full flex flex-col p-2">
          {#if error}<p class="error" role="alert">{error}</p>{/if}
          {#if plotFindEnabled && plotFindOpen}
            <section aria-label="Find scoped plot" class="mb-2 p-3 border rounded">
              <form onsubmit={(event) => { event.preventDefault(); void findScopedPlot(); }} class="flex flex-wrap items-end gap-2">
                <label for="find-scoped-plot" class="flex flex-col gap-1 flex-1 min-w-0">Exact plot number
                  <input id="find-scoped-plot" class="px-2 py-1 border rounded min-w-0" bind:value={plotFindQuery}
                    oninput={() => { plotFindResult = null; plotFindError = ''; }} disabled={busy || editorBusy || transitionWorking || pendingTransition !== null} />
                </label>
                <button type="submit" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}>Find in current scope</button>
                <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || transitionWorking || pendingTransition !== null}
                  onclick={() => { plotFindOpen = false; plotFindResult = null; plotFindError = ''; }}>Dismiss find</button>
              </form>
              {#if plotFindError}<p role="alert" class="mt-2 text-red-800">{plotFindError}</p>{/if}
              {#if plotFindResult}
                <p role="status" class="mt-2">Found literal plot {JSON.stringify(plotFindResult.plot.plotNumber)} in the current recordset.</p>
                <button type="button" class="mt-2 px-3 py-1 border rounded disabled:opacity-50"
                  disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || plotFindResult.contextId !== $projectState?.contextId || plotFindResult.plot.plotNumber === editorPlotNumber}
                  onclick={() => void openFoundPlot()}>Open found plot</button>
              {/if}
              <p class="mt-2 text-sm">Find is read-only. Exact spaces and case are preserved, including historical long identifiers. Current SU and reviewed profile filters stay in effect. Open rechecks the target after Save/Discard/Cancel; no record is created.</p>
            </section>
          {/if}
          <fieldset disabled={busy} class="contents">
          {#key $projectState?.contextId}
          {#key editorEntryForm}
          {#key editorPlotNumber}
          {#await loadFS882Editor()}
            <p role="status" data-entry-loading="fs882">Loading {editorLabel} editor...</p>
          {:then { default: FS882Form }}
          <FS882Form bind:this={editor} plotNumber={editorPlotNumber} contextId={$projectState?.contextId ?? ''} entryForm={editorEntryForm}
            onSaved={async (p) => { editorPlotNumber = p; await loadPlots(offset); }} onClosed={() => navigate('plots')}
            onBusyChange={(busy) => { localEditorBusy = busy; }} onProfileNavigation={applyProfileNavigation}
            onProfileSUReview={reviewProfileSU} onFindPlot={plotFindEnabled ? () => { plotFindOpen = true; } : undefined} />
          {:catch error}
            <p class="error" role="alert" data-entry-load-error="fs882">{editorLabel} editor could not load: {String(error)}</p>
          {/await}
          {/key}
          {/key}
          {/key}
          </fieldset>
        </div>
        <CloseConfirm requestId={closeRequest || (pendingTransition ? 'context' : '')} purpose={closeRequest ? 'close' : 'context'} {editorLabel}
          working={closeWorking || transitionWorking} canSave={closeState?.canSave ?? false}
          saveReason={closeState?.saveReason ?? `Return to the ${editorLabel} editor before saving.`}
          error={closeRequest ? closeError : transitionError} onrespond={closeRequest ? respondToClose : respondToTransition} />
        {#if editorPlotNumber && $projectState?.contextId}
          {#await import('./diagnostics/LMH25DiagnosticsBar.svelte') then { default: LMH25DiagnosticsBar }}
            <div class="px-2 pb-2"><LMH25DiagnosticsBar contextId={$projectState.contextId} plotNumber={editorPlotNumber} /></div>
          {/await}
        {/if}
      {:else if view === 'datasheet' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./datasheet/UniversalDatasheetGrid.svelte')}
            <p role="status" class="p-4 text-sm">Loading Universal Datasheet Grid...</p>
          {:then { default: UniversalDatasheetGrid }}
            <UniversalDatasheetGrid contextId={$projectState.contextId ?? ''} project={$projectState.activeProject} su={$projectState.activeSU} hierarchy={$projectState.activeHierarchy}
              onBusyChange={(value) => { localEditorBusy = value; }} onDirtyChange={(dirty) => { workbenchDirtyLabel = dirty ? 'datasheet' : ''; }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }} />
          {/await}
        {/key}
      {:else if view === 'classification-explorer' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./classification/ClassificationExplorer.svelte')}
          <p role="status" class="p-4 text-sm">Loading Classification Explorer...</p>
        {:then { default: ClassificationExplorer }}
          <ClassificationExplorer contextId={$projectState.contextId ?? ''} project={$projectState.activeProject} su={$projectState.activeSU} hierarchy={$projectState.activeHierarchy}
            operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
            onBusyChange={(value) => { localEditorBusy = value; }} onDirtyChange={(dirty) => { workbenchDirtyLabel = dirty ? 'classification-explorer' : ''; }}
            onContextChanged={async () => { await refresh(); }}
            onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }} />
        {/await}
      {:else if view === 'project-admin' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <div class="p-3 space-y-4">
          {#await import('./admin/ProjectAdminWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading Project Admin Workbench...</p>
          {:then { default: ProjectAdminWorkbench }}
            <ProjectAdminWorkbench contextId={$projectState.contextId ?? ''} activeProject={$projectState.activeProject} activeSu={$projectState.activeSU}
              initialTab={projectAdminTab} initialTabNonce={projectAdminTabNonce}
              operationHeld={busy || companionPanelBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} oncontextswitched={async () => { await refresh(); }} />
          {/await}
          {#await import('./admin/ProjectMetadataPresets.svelte') then { default: ProjectMetadataPresets }}
            <ProjectMetadataPresets contextId={$projectState.contextId ?? ''} projectId=""
              operationHeld={busy || localEditorBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { secondaryPanelBusy = value; }} />
          {/await}
        </div>
      {:else if view === 'lump-editor' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./classification/LumpEditor.svelte')}
            <p role="status" class="p-4 text-sm">Loading Combine Species &amp; LumpSpec Editor...</p>
          {:then { default: LumpEditor }}
            <LumpEditor contextId={$projectState.contextId ?? ''} operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/await}
        {/key}
      {:else if view === 'diagnostic-synthesis' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./diagnostics/ClassificationSynthesisView.svelte')}
            <p role="status" class="p-4 text-sm">Loading BEC Diagnostic &amp; Pairwise Synthesis Workbench...</p>
          {:then { default: ClassificationSynthesisView }}
            <ClassificationSynthesisView contextId={$projectState.contextId ?? ''} project={$projectState.activeProject} su={$projectState.activeSU}
              onBusyChange={(value) => { localEditorBusy = value; }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }} />
          {/await}
        {/key}
      {:else if view === 'r-sidecar' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./rsidecar/RSidecarWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading R Sidecar, climr &amp; Quarto Workbench...</p>
          {:then { default: RSidecarWorkbench }}
            <RSidecarWorkbench contextId={$projectState.contextId ?? ''} activeProject={$projectState.activeProject} activeSu={$projectState.activeSU}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} oncontextswitched={async () => { await refresh(); }} />
          {/await}
        {/key}
      {:else if view === 'geospatial-map' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./spatial/GeospatialWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading 3D Geospatial Workbench...</p>
          {:then { default: GeospatialWorkbench }}
            <GeospatialWorkbench contextId={$projectState.contextId ?? ''} project={$projectState.activeProject} su={$projectState.activeSU}
              onBusyChange={(value) => { localEditorBusy = value; }} onContextChanged={async () => { await refresh(); }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }} />
          {/await}
        {/key}
      {:else if view === 'interchange-hub' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./interchange/InterchangeHub.svelte')}
            <p role="status" class="p-4 text-sm">Loading Interchange Hub...</p>
          {:then { default: InterchangeHub }}
            <InterchangeHub contextId={$projectState.contextId ?? ''} operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} onProjectActivated={async () => { await refresh(); }} />
          {/await}
        {/key}
      {:else if view === 'summary-environment' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/SiteUnitSummary.svelte')}
          <p role="status" class="p-4 text-sm">Loading Site Unit Summary...</p>
        {:then { default: SiteUnitSummary }}
          {#key $projectState.contextId}<SiteUnitSummary contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Site Unit Summary could not load: {String(loadError)}</p>{/await}
      {:else if view === 'long-environment' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/LongEnvironmentReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Long Environment Report...</p>
        {:then { default: LongEnvironmentReport }}
          {#key $projectState.contextId}<LongEnvironmentReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Long Environment Report could not load: {String(loadError)}</p>{/await}
      {:else if view === 'long-vegetation' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/LongVegetationReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Long Vegetation Report...</p>
        {:then { default: LongVegetationReport }}
          {#key $projectState.contextId}<LongVegetationReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Long Vegetation Report could not load: {String(loadError)}</p>{/await}
      {:else if view === 'summary-vegetation' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/SummaryVegetationReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Summary Vegetation Report...</p>
        {:then { default: SummaryVegetationReport }}
          {#key $projectState.contextId}
            <SummaryVegetationReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} />
            {#await import('./classification/SiteUnitExclusionFilter.svelte') then { default: SiteUnitExclusionFilter }}
              <div class="px-3 pb-4"><SiteUnitExclusionFilter contextId={$projectState.contextId ?? ''} su={$projectState.activeSU}
                hierarchy={$projectState.activeHierarchy} reportMode="summary" onBusyChange={(value) => { secondaryPanelBusy = value; }} /></div>
            {/await}
          {/key}
        {:catch loadError}<p class="error" role="alert">Summary Vegetation Report could not load: {String(loadError)}</p>{/await}
      {:else if view === 'lifeform-summary' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/LifeformSummary.svelte')}
          <p role="status" class="p-4 text-sm">Loading Lifeform Summary...</p>
        {:then { default: LifeformSummary }}
          {#key $projectState.contextId}<LifeformSummary contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Lifeform Summary could not load: {String(loadError)}</p>{/await}
      {:else if view === 'table-csv' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./interchange/ProjectTableCSVReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Table CSV Review...</p>
        {:then { default: ProjectTableCSVReview }}
          {#key $projectState.contextId}<ProjectTableCSVReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Table CSV Review could not load: {String(loadError)}</p>{/await}
      {:else if view === 'user-log' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./admin/UserLogReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading User Log Review...</p>
        {:then { default: UserLogReview }}
          {#key $projectState.contextId}<UserLogReview contextId={$projectState.contextId ?? ''}
            operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">User Log Review could not load: {String(loadError)}</p>{/await}
      {:else if view === 'plot-label-preview' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./interchange/PlotLabelPreview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Plot Label Preview...</p>
        {:then { default: PlotLabelPreview }}
          {#key $projectState.contextId}<PlotLabelPreview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            initialPlot={plotLabelNumber} onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Plot Label Preview could not load: {String(loadError)}</p>{/await}
      {:else if view === 'picture-manager' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./pictures/PictureManager.svelte')}
          <p role="status" class="p-4 text-sm">Loading Picture Manager...</p>
        {:then { default: PictureManager }}
          {#key $projectState.contextId}<PictureManager contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            initialPlot={pictureManagerPlot} onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Picture Manager could not load: {String(loadError)}</p>{/await}
      {:else if view === 'plot-locations' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./spatial/PlotLocationReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Plot Location Review...</p>
        {:then { default: PlotLocationReview }}
          {#key $projectState.contextId}<PlotLocationReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Plot Location Review could not load: {String(loadError)}</p>{/await}
      {:else if view === 'google-earth-review' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./spatial/GoogleEarthReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Google Earth Review...</p>
        {:then { default: GoogleEarthReview }}
          {#key $projectState.contextId}<GoogleEarthReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
            projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
            onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Google Earth Review could not load: {String(loadError)}</p>{/await}
      {:else if view === 'subzone-matrix' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./reports/SubzoneMatrixReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Subzone Matrix Report...</p>
        {:then { default: SubzoneMatrixReport }}
          {#key $projectState.contextId}<SubzoneMatrixReport contextId={$projectState.contextId ?? ''} hierarchy={$projectState.activeHierarchy}
            hierarchyPath={$projectState.hierarchyPath ?? ''} onBusyChange={(value) => { localEditorBusy = value; }} />{/key}
        {:catch loadError}<p class="error" role="alert">Subzone Matrix Report could not load: {String(loadError)}</p>{/await}
      {:else}
        <PlotsBrowserPanel region="main" {view} {busy} {error} {page} {offset} {pageSize} {hierarchyNodes} bind:selected {siviStandaloneEnabled} onrefresh={refresh} onloadpage={loadPlots} />
      {/if}
    </main>

    <PlotsBrowserPanel region="detail" {view} {busy} bind:selected {siviStandaloneEnabled} {editorBusy} {transitionWorking} {pendingTransition}
      onopenfs882={() => navigate('fs882')} onopensivi={() => void openSIVI()} />
    {#if newSiviModalOpen && $projectState?.contextId}
      {#await import('./entry/SIVIParentCreateModal.svelte') then { default: SIVIParentCreateModal }}
        <SIVIParentCreateModal contextId={$projectState.contextId} activeProject={$projectState.activeProject} activeSu={$projectState.activeSU}
          open={newSiviModalOpen} operationHeld={busy || editorBusy || secondaryPanelBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
          onBusyChange={(value) => { siviModalBusy = value; }}
          oncreated={async (res) => { newSiviModalOpen = false; await loadPlots(offset); editorPlotNumber = res.plotNumber; editorEntryForm = 'sivi'; view = 'fs882'; }}
          onundone={async () => { await loadPlots(offset); }} onclose={() => { newSiviModalOpen = false; }} />
      {/await}
    {/if}
  </div>

  <ResetSampleModal open={resetSampleConfirmOpen && !startupFailure} {busy} {editorBusy} {companionPanelBusy} {transitionWorking}
    contextAvailable={!!$projectState?.contextId} onconfirm={() => void confirmResetWorkspaceToSample()} onclose={() => { resetSampleConfirmOpen = false; }} />

  {#if helpDrawerOpen && !startupFailure}
    {#await import('./help/HelpDrawer.svelte') then { default: HelpDrawer }}
      <HelpDrawer open={helpDrawerOpen} topicId={helpDrawerTopicId} initialTab={helpDrawerTab} onclose={() => { helpDrawerOpen = false; }}
        onstarttour={(tid) => { helpDrawerOpen = false; tourInitialId = tid ?? null; tourOpen = true; }} />
    {/await}
  {/if}
  {#if tourOpen && !startupFailure}
    {#await import('./help/OnboardingTour.svelte') then { default: OnboardingTour }}
      <OnboardingTour open={tourOpen} initialTourId={tourInitialId} activeProject={$projectState?.activeProject ?? 'Sample'}
        onensurecontextopen={() => { contextExpanded = true; }} onnavigateview={(nextView) => { navigate(nextView as typeof view); }}
        onresetsample={async () => { tourOpen = false; resetSampleConfirmOpen = true; }} onclose={() => { tourOpen = false; tourInitialId = null; }} />
    {/await}
  {/if}
  {#if shortcutsModalOpen && !startupFailure}
    {#await import('./help/KeyboardShortcutsModal.svelte') then { default: KeyboardShortcutsModal }}
      <KeyboardShortcutsModal open={shortcutsModalOpen} onclose={() => { shortcutsModalOpen = false; }} />
    {/await}
  {/if}
  {#if whatsNewModalOpen && !startupFailure}
    {#await import('./help/WhatsNewModal.svelte') then { default: WhatsNewModal }}
      <WhatsNewModal open={whatsNewModalOpen} initialStatus={whatsNewStatus} onstatuschange={(updated) => { whatsNewStatus = updated; }} onclose={() => { whatsNewModalOpen = false; }} />
    {/await}
  {/if}
  {#if aboutModalOpen && !startupFailure}
    {#await import('./help/AboutVProModal.svelte') then { default: AboutVProModal }}
      <AboutVProModal open={aboutModalOpen} onclose={() => { aboutModalOpen = false; }} />
    {/await}
  {/if}
</div>
