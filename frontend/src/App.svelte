<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { ChevronLeft, ChevronRight, Database, FolderOpen, RefreshCw } from '@lucide/svelte';
  import { ContextService, ImportExportService, LifecycleService, type StartupState, type StartupRecoveryRequest, type ContextSelection, type ProjectInfo, type HierarchyNode, type PlotPage, type PlotSummary, type ProjectPlotProfileFilterRequest, type PlotProfileSource, type PlotProfileSourceInfo, type AccessDatabaseInspection, type AccessProjectCandidate, type AccessProjectImportResult } from '../bindings/github.com/boostao/vpro-wails';
  import { projectState } from './state';
  import Navigation from './Navigation.svelte';
  import HomeWorkspaceHub from './HomeWorkspaceHub.svelte';
  import { summaryVegetationPreferenceSession } from './summaryVegetationPreferenceSessions';
  import { summaryVegetationQualityPreferenceSession } from './summaryVegetationQualityPreferenceSessions';
  import { summaryVegetationPresentationPreferenceSession } from './summaryVegetationPresentationPreferenceSessions';
  import { summaryVegetationFieldListDesktop } from './summaryVegetationFieldListDesktop';
  import { summaryEnvironmentalDesktop, summaryEnvironmentalEnabled } from './summaryVegetationEnvironmentalDesktop';
  import { matrixPublicationSession } from './subzoneMatrixReport';
  import CloseConfirm from './CloseConfirm.svelte';
  import { closeDisposition, type CloseDecision, type EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { validateProfileNavigation, profileNavigationPage, validatePlotProfileSource, validateProjectPlotProfileReview, type ValidatedProjectPlotProfileReview, type ProfileNavigation } from './projectPlotProfileReview';
  import { validateProfileSUReview, validateProfileSUCreated, type ReviewedProfileSU } from './profileSU';
  import { validateProfileFileReview, validateProfileFileCreated, type ReviewedProfileFile } from './profileFile';
  import { validateProfileTableReview, validateProfileTableCreated, type ReviewedProfileTable } from './profileTable';
  import { validateProjectSUReview, validateProjectSUCreated, type ReviewedProjectSU } from './profileSUProject';
  import { plotLookupInputError, validateScopedPlotLookup } from './scopedPlotLookup';
  import { tableCSVArchivePublicationSession } from './tableCSVArchiveExport';
  import { vegetationWorkbookPublicationSession } from './vegetationWorkbook';
  import { projectReportWorkspace } from './longVegetationProjectWorkspaces';
  import { lifeformWorkbookPublicationSession } from './lifeformWorkbook';
  import { siteUnitSummaryWorkbookPublicationSession } from './siteUnitSummaryWorkbook';
  import { extendedSummaryWorkbookPublicationSession } from './siteUnitSummaryExtendedWorkbook';
  import { summaryVegetationWorkbookSession } from './summaryVegetationWorkbook';
  import { subscribePictureCreationActivity } from './pictureCreationActivity';
  import { subscribePictureLifecycleActivity } from './pictureLifecycleActivity';
  import { subscribeTwoPageSoilActivity } from './twoPageSoilActivity';
  import { subscribeTwoPageVegetationActivity } from './twoPageVegetationActivity';
  import { subscribeExternalSUEnvironmentActivity } from './externalSUEnvironment';
  import { classifyGlobalHelpShortcut, defaultHelpPort, resolveContextualHelpTarget, type HelpDrawerTab, type HelpMenuAction, type WhatsNewStatus } from './helpSystem';

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
  let offset = $state(0);
  let busy = $state(false);
  let error = $state('');
  let request = 0;
  const plotReads = new ReadRequests();
  const hierarchyReads = new ReadRequests();
  const stateReads = new ReadRequests();
  let hierarchyRequest = 0;
  let stateRequest = 0;
  onDestroy(() => {
    request++; hierarchyRequest++; stateRequest++;
    secondaryPanelBusy = false; siviModalBusy = false; workbenchDirtyLabel = '';
    plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
  });
  let view = $state<'home' | 'plots' | 'geospatial-map' | 'diagnostic-synthesis' | 'r-sidecar' | 'hierarchy' | 'fs882' | 'datasheet' | 'classification-explorer' | 'project-admin' | 'lump-editor' | 'interchange-hub' | 'long-environment' | 'summary-environment' | 'long-vegetation' | 'summary-vegetation' | 'lifeform-summary' | 'subzone-matrix' | 'table-csv' | 'user-log' | 'plot-locations' | 'google-earth-review' | 'picture-manager' | 'plot-label-preview'>('home');
  let projectAdminTab = $state<'lifecycle' | 'compliance' | 'optimizeveg' | 'merge' | 'cloudsync'>('lifecycle');
  let projectAdminTabNonce = $state(0);
  let newSiviModalOpen = $state(false);
  let helpDrawerOpen = $state(false);
  let helpDrawerTopicId = $state('site.plot_number');
  let helpDrawerTab = $state<HelpDrawerTab>('inspector');
  let tourOpen = $state(false);
  let tourInitialId = $state<string | null>(null);
  let shortcutsModalOpen = $state(false);
  let whatsNewModalOpen = $state(false);
  let aboutModalOpen = $state(false);
  let resetSampleConfirmOpen = $state(false);
  let resetSampleNotice = $state('');
  let whatsNewStatus = $state.raw<WhatsNewStatus | null>(null);
  let whatsNewPromptDismissed = $state(false);
  let pictureManagerPlot = $state('');
  let plotLabelNumber = $state('');
  let contextExpanded = $state(false);
  let localEditorBusy = $state(false);
  let matrixPublicationBusy = $state(false);
  let pictureCreationBusy = $state(false);
  let pictureLifecycleBusy = $state(false);
  let twoPageSoilBusy = $state(false);
  let twoPageVegetationBusy = $state(false);
  let externalSUEnvironmentBusy = $state(false);
  let projectReportBusy = $state(false);
  let summaryPreferenceBusy = $state(false);
  let summaryFieldListBusy = $state(false);
  let summaryEnvironmentalBusy = $state(false);
  const editorBusy = $derived(localEditorBusy || matrixPublicationBusy || pictureCreationBusy || pictureLifecycleBusy || twoPageSoilBusy || twoPageVegetationBusy || externalSUEnvironmentBusy || summaryPreferenceBusy || summaryFieldListBusy || summaryEnvironmentalBusy || projectReportBusy);
  let secondaryPanelBusy = $state(false);
  let siviModalBusy = $state(false);
  const companionPanelBusy = $derived(secondaryPanelBusy || siviModalBusy);
  let workbenchDirtyLabel = $state('');
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId ||
        state.activeHierarchy === 'None' || !state.hierarchyPath) { matrixPublicationBusy = false; return; }
    const session = matrixPublicationSession({ contextId: state.contextId,
      hierarchy: state.activeHierarchy, hierarchyPath: state.hierarchyPath });
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
        !state?.contextId || !state.projectPath || !state.suPath) {
      summaryPreferenceBusy = false; return;
    }
    const owner = { contextId: state.contextId, project: state.activeProject,
      projectPath: state.projectPath, su: state.activeSU, suPath: state.suPath };
    const preferences = summaryVegetationPreferenceSession?.(owner);
    const quality = summaryVegetationQualityPreferenceSession?.(owner);
    const presentation = summaryVegetationPresentationPreferenceSession?.(owner);
    const changed = () => {
      const views = [preferences?.view(), quality?.view(), presentation?.view()];
      summaryPreferenceBusy = views.some(current => !!current && (current.busy || current.blocked || !!current.draftError));
      if (summaryPreferenceBusy && view === 'home') view = 'summary-vegetation';
    };
    const releasePreferences = preferences?.subscribe(changed), releaseQuality = quality?.subscribe(changed),
      releasePresentation = presentation?.subscribe(changed);
    return () => { releasePreferences?.(); releaseQuality?.(); releasePresentation?.(); };
  });
  $effect(() => {
    const state = $projectState;
    if (!projectReportWorkspace ||
        !state?.contextId || !state.projectPath || state.activeSU !== 'None' && !state.suPath) {
      projectReportBusy = false;
      return;
    }
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
      if (activity.owner && activity.owner.contextId === state?.contextId &&
          activity.owner.project === state.activeProject && view === 'home') {
        pictureManagerPlot = activity.owner.plotNumber;
        view = 'picture-manager';
      }
    });
  });
  $effect(() => {
    const state = $projectState;
    return subscribePictureCreationActivity(activity => {
      pictureCreationBusy = activity.busy || activity.blocked;
      if (activity.owner && activity.owner.contextId === state?.contextId &&
          activity.owner.project === state.activeProject && view === 'home') {
        pictureManagerPlot = activity.owner.plotNumber;
        view = 'picture-manager';
      }
    });
  });
  let archivePublicationBusy = $state(false);
  let vegetationPublicationBusy = $state(false);
  let lifeformPublicationBusy = $state(false);
  let summaryEnvironmentPublicationBusy = $state(false);
  let summaryVegetationPublicationBusy = $state(false);
  const summaryPublicationBusy = $derived(summaryEnvironmentPublicationBusy || summaryVegetationPublicationBusy);
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { vegetationPublicationBusy = false; return; }
    const session = vegetationWorkbookPublicationSession({ contextId: state.contextId,
      project: state.activeProject, projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath },
      true);
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
    const session = tableCSVArchivePublicationSession({ contextId: state.contextId,
      project: state.activeProject, projectPath: state.projectPath ?? '' });
    return session.subscribe(() => {
      const publication = session.view();
      archivePublicationBusy = publication.busy || publication.blocked;
      // Reloaded unacknowledged attempts must be visible even before this tab has mounted.
      if (publication.blocked && view === 'home') view = 'table-csv';
    });
  });
  $effect(() => {
    const state = $projectState;
    if (!state?.contextId || state.activeSU === 'None' || !state.suPath) { summaryEnvironmentPublicationBusy = false; return; }
    const owner = { contextId: state.contextId, project: state.activeProject,
      projectPath: state.projectPath ?? '', su: state.activeSU, suPath: state.suPath };
    const sessions = [
      siteUnitSummaryWorkbookPublicationSession(owner),
      extendedSummaryWorkbookPublicationSession(owner),
    ];
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
    const module = await import('./FS882Form.svelte');
    fs882ModuleLoaded = true;
    return module;
  }

  let closeRequest = $state('');
  let closeWorking = $state(false);
  let closeError = $state('');
  let pendingTransition = $state<{ contextId: string; run: (contextId: string) => Promise<void> } | null>(null);
  let transitionWorking = $state(false);
  let transitionError = $state('');
  let externalProject = $state('');
  let externalPath = $state('');
  let externalDiscoveredProjects = $state<ProjectInfo[]>([]);
  let externalPickMessage = $state('');
  let externalPickError = $state('');
  let revealMessage = $state('');
  let revealError = $state('');
  let accessSourcePath = $state('');
  let accessInspection = $state<AccessDatabaseInspection | null>(null);
  let accessSelectedSourceProject = $state('');
  let accessTargetProject = $state('');
  let accessDestinationPath = $state('');
  let accessActivateOnSuccess = $state(true);
  let accessWorking = $state(false);
  let accessError = $state('');
  let accessResult = $state<AccessProjectImportResult | null>(null);
  const accessSelectedCandidate = $derived(accessInspection?.projects?.find(p => p.sourceProject === accessSelectedSourceProject) ?? null);
  const externalProjectsEnabled = true;
  const profileSelectionEnabled = true;
  const profileWriteOwnershipEnabled = true;
  const profileFileCreationEnabled = true;
  const profileTableCreationEnabled = true;
  let profileTableReview = $state<ReviewedProfileTable | null>(null);
  let profileTableContext = $state('');
  let profileTableName = $state('');
  let profileTableError = $state('');
  let profileTableSuccess = $state('');
  let profileTableCommitted = $state(false);
  let profileFileReview = $state<ReviewedProfileFile | null>(null);
  let profileFileContext = $state('');
  let profileFileName = $state('');
  let profileFilePath = $state('');
  let profileFileError = $state('');
  let profileFileSuccess = $state('');
  let profileFileCommitted = $state(false);
  let profileWriteReview = $state<ValidatedProjectPlotProfileReview | null>(null);
  let profileWriteContext = $state('');
  let profileWriteError = $state('');
  let profilePath = $state('');
  let profileSources = $state<PlotProfileSourceInfo[]>([]);
  let profileSourceContext = $state('');
  let profileSourceError = $state('');
  let suReview = $state<ReviewedProfileSU | null>(null);
  let suReviewContext = $state('');
  let suName = $state('');
  let suPath = $state('');
  let suError = $state('');
  let suSuccess = $state('');
  let suCommitted = $state(false);
  const suProjectEnabled = true;
  let suProjectReview = $state<ReviewedProjectSU | null>(null);
  let suProjectContext = $state('');
  let suProjectName = $state('');
  let suProjectError = $state('');
  let suProjectSuccess = $state('');
  let suProjectCommitted = $state(false);
  const plotFindEnabled = true;
  let plotFindOpen = $state(false);
  let plotFindQuery = $state('');
  let plotFindResult = $state<Awaited<ReturnType<typeof ContextService.LookupScopedPlot>> | null>(null);
  let plotFindError = $state('');
  let startupInfo = $state<StartupState | null>(null);
  let startupFailure = $state<StartupState | null>(null);
  const buildMode = $derived(startupInfo?.buildMode ?? '');
  const appVersion = $derived(startupInfo?.appVersion ?? '');
  const shortBuildCommit = $derived.by(() => {
    const raw = (startupInfo?.gitCommit ?? '').trim();
    return /^[0-9a-f]{8,40}$/i.test(raw) ? raw.slice(0, 7) : raw;
  });
  const releaseChannel = $derived(startupInfo?.releaseChannel ?? 'stable');
  let recoveryWorking = $state(false);
  let recoveryError = $state('');
  let startupBackupNotice = $state('');
  let recoveryProjectPath = $state('');
  let recoveryProjectName = $state('');
  let recoverySUPath = $state('');
  let recoveryClearSU = $state(false);
  let recoveryHierarchyPath = $state('');
  let recoveryClearHierarchy = $state(false);

  async function fallbackStartupToSample() {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const recovered = await LifecycleService.ResetStartupToSample();
      startupBackupNotice = recovered.backupPath
        ? `Recovered with built-in Sample project. Previous configuration was backed up to ${recovered.backupPath}.`
        : 'Recovered with built-in Sample project.';
      startupFailure = null;
      await refresh();
    } catch (cause) {
      recoveryError = `Could not fall back to Sample project: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }

  async function browseStartupDatabase(target: 'project' | 'su' | 'hierarchy') {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const picked = await LifecycleService.ChooseStartupDatabaseFile();
      if (!picked) return;
      if (target === 'project') {
        recoveryProjectPath = picked;
      } else if (target === 'su') {
        recoverySUPath = picked;
        recoveryClearSU = false;
      } else {
        recoveryHierarchyPath = picked;
        recoveryClearHierarchy = false;
      }
    } catch (cause) {
      recoveryError = `File picker unavailable: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }

  async function recoverStartupPaths() {
    if (recoveryWorking || busy) return;
    recoveryWorking = true;
    recoveryError = '';
    try {
      const payload: StartupRecoveryRequest = $state.snapshot({
        projectPath: recoveryProjectPath.trim(),
        projectName: recoveryProjectName.trim(),
        suPath: recoverySUPath.trim(),
        clearSu: recoveryClearSU,
        hierarchyPath: recoveryHierarchyPath.trim(),
        clearHierarchy: recoveryClearHierarchy,
      });
      const recovered = await LifecycleService.RecoverStartupWithPaths(payload);
      startupBackupNotice = recovered.backupPath
        ? `Relocated project opened cleanly. Previous configuration was backed up to ${recovered.backupPath}.`
        : 'Relocated project opened cleanly.';
      startupFailure = null;
      await refresh();
    } catch (cause) {
      recoveryError = `Could not open relocated database: ${String(cause)}`;
    } finally {
      recoveryWorking = false;
    }
  }
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
        error = (typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel)
          ? 'Save or discard unsaved workbench edits before closing VPro.'
          : 'Wait for the current operation to finish before closing VPRO.';
      } else if (typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel) {
        await LifecycleService.CancelClose(requestId);
        error = 'Save or discard unsaved workbench edits before closing VPro.';
      } else if (disposition === 'clean') {
        await LifecycleService.ConfirmClose(requestId);
      } else {
        closeError = '';
        closeRequest = requestId;
      }
    } catch (cause) {
      error = `Window close request failed: ${String(cause)}`;
    }
  }

  async function respondToClose(decision: CloseDecision) {
    if (!closeRequest || closeWorking) return;
    const requestId = closeRequest;
    closeWorking = true;
    closeError = '';
    try {
      if (decision === 'cancel') {
        await LifecycleService.CancelClose(requestId);
        closeRequest = '';
        return;
      }
      const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
      if (closeDisposition(state, busy || editorBusy || transitionWorking || archivePublicationBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || pendingTransition !== null || editorInitializing || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
        closeError = 'Wait for the current operation to finish before closing VPRO.';
        return;
      }
      if (decision === 'save') {
        if (!editor || !await editor.saveForClose()) {
          closeError = editor?.getCloseState().error ?? 'The draft could not be saved. VPRO remains open.';
          return;
        }
      }
      await LifecycleService.ConfirmClose(requestId);
    } catch (cause) {
      closeError = `VPRO remains open: ${String(cause)}`;
    } finally {
      closeWorking = false;
    }
  }
  let activeHierarchyIndex = $derived($projectState?.hierarchies?.findIndex(
    (hierarchy) => hierarchy.name === $projectState?.activeHierarchy && hierarchy.path === $projectState?.hierarchyPath
  ) ?? -1);
  let activeSUIndex = $derived($projectState?.sus?.findIndex(
    (su) => su.name === $projectState?.activeSU && su.path === $projectState?.suPath
  ) ?? -1);

  async function requestTransition(run: (contextId: string) => Promise<void>, ownedAction?: () => boolean) {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    await tick();
    if (ownedAction && !ownedAction()) {
      error = 'Owned action or editor availability changed; no editor replaced.';
      return;
    }
    if (!ownedAction && typeof workbenchDirtyLabel !== 'undefined' && workbenchDirtyLabel) {
      error = 'Save or discard unsaved workbench edits before changing context or view.';
      return;
    }
    const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
    if (closeDisposition(ownedAction ? null : state, busy || !ownedAction && editorBusy || transitionWorking || archivePublicationBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || pendingTransition !== null || !!closeRequest || editorInitializing || (!ownedAction && typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
      error = 'Wait for the current operation or decision before changing context.';
      return;
    }
    const contextId = $projectState?.contextId;
    if (!contextId) { error = 'A project context must finish loading before navigation.'; return; }
    transitionError = '';
    if (state?.unsaved && !ownedAction) {
      pendingTransition = { contextId, run };
      return;
    }
    try { await run(contextId); }
    catch (cause) { error = String(cause); }
  }

  async function respondToTransition(decision: CloseDecision) {
    if (!pendingTransition || transitionWorking) return;
    if (decision === 'cancel') { pendingTransition = null; transitionError = ''; return; }
    const pending = pendingTransition;
    transitionWorking = true;
    transitionError = '';
    try {
      const state = editor?.getCloseState();
      if (!state || closeDisposition(state, busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy)) === 'busy') {
        transitionError = 'Wait for the current editor operation to finish.';
        return;
      }
      if (decision === 'save' && state.unsaved && !await editor?.saveForClose()) {
        transitionError = editor?.getCloseState().error ?? 'The draft could not be saved; context and draft remain open.';
        return;
      }
      await pending.run(pending.contextId);
      pendingTransition = null;
    } catch (cause) {
      transitionError = `Context remains unchanged: ${String(cause)}`;
    } finally {
      transitionWorking = false;
    }
  }

  function navigate(next: typeof view) {
    if (next === view && next !== 'project-admin') return;
    void requestTransition(async () => {
      error = '';
      if (next === 'project-admin') { projectAdminTab = 'lifecycle'; projectAdminTabNonce++; }
      if (next === 'fs882') { editorPlotNumber = selected?.plotNumber; editorEntryForm = 'fs882'; }
      if (next === 'picture-manager') {
        pictureManagerPlot = selected?.plotNumber ?? '';
      }
      if (next === 'plot-label-preview') {
        plotLabelNumber = (view === 'fs882' ? editorPlotNumber : selected?.plotNumber) ?? '';
      }
      view = next;
    });
  }

  async function openSIVI() {
    if (!siviStandaloneEnabled || !selected || !$projectState?.contextId) {
      error = 'Standalone SIVI requires its independent entry gate, parent review and a selected owned plot.';
      return;
    }
    const plot = selected.plotNumber;
    await requestTransition(async () => {
      if (selected?.plotNumber !== plot) throw new Error('Selected plot changed before opening SIVI; no owner replaced.');
      editorPlotNumber = plot; editorEntryForm = 'sivi'; view = 'fs882'; error = '';
    });
  }

  async function openProjectMetadata() {
    if (view !== 'fs882' || !editor || !$projectState?.contextId || !editor.canOpenProjectMetadata()) {
      error = 'Project metadata requires its existing gate and a clean, unlocked open plot; finish or Undo drafts first.';
      return;
    }
    const owner = editor;
    const contextId = $projectState.contextId;
    const ownedAction = owner.canOpenMetadataInPlace?.()
      ? () => view === 'fs882' && editor === owner && $projectState?.contextId === contextId &&
        owner.canOpenMetadataInPlace?.() === true
      : undefined;
    await requestTransition(async currentContext => {
      if (view !== 'fs882' || editor !== owner || currentContext !== contextId ||
          $projectState?.contextId !== contextId || !owner.canOpenProjectMetadata()) {
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
      if (!next.plots.some(plot => plot.plotNumber === number)) {
        throw new Error('Literal plot is outside the reviewed profile navigation; current filter is unchanged.');
      }
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
    try {
      await tick();
      plotFindResult = await lookupPlot(number, contextId);
    } catch (cause) { plotFindError = `No navigation or data change; find failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function openFoundPlot() {
    if (!plotFindEnabled || !plotFindResult || plotFindResult.contextId !== $projectState?.contextId) {
      plotFindError = 'Find an available plot in the current scope before opening it.'; return;
    }
    const proposal = $state.snapshot(plotFindResult);
    if (proposal.plot.plotNumber === editorPlotNumber) {
      plotFindError = 'Found plot is already open; no draft or navigation change.'; return;
    }
    await requestTransition(async contextId => {
      if (contextId !== proposal.contextId) throw new Error('Found plot belongs to an old context; find again.');
      busy = true; plotFindError = '';
      try {
        await tick();
        const current = await lookupPlot(proposal.plot.plotNumber, contextId);
        if (JSON.stringify(current) !== JSON.stringify(proposal)) {
          throw new Error('Found plot summary changed since review; find again before opening.');
        }
        editorPlotNumber = current.plot.plotNumber; view = 'fs882'; selected = null;
        plotFindOpen = false; plotFindResult = null; error = '';
      } catch (cause) {
        plotFindError = `Navigation failed; current editor and filters retained: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function navigateProfilePlot(delta: number) {
    const source = profileNavigation;
    const target = source?.plots[profileIndex + delta]?.plotNumber;
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
    plotReads.cancelAll();
    busy = true;
    error = '';
    try {
      const source = profileNavigation;
      const next = source ? await resolveNavigation(source.proposal, source.contextId) : null;
      const result = next ? profileNavigationPage(next, next.contextId, nextOffset, pageSize) :
        await plotReads.track(ContextService.ListPlots(nextOffset, pageSize));
      if (current !== request) return;
      if (next) profileNavigation = next;
      page = result;
      offset = nextOffset;
      selected = null;
    } catch (cause) {
      if (current === request) error = String(cause);
    } finally {
      if (current === request) busy = false;
    }
  }

  async function refresh() {
    if (projectReportBusy || twoPageVegetationBusy || twoPageSoilBusy || pictureCreationBusy || pictureLifecycleBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || summaryPreferenceBusy || externalSUEnvironmentBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || (view === 'long-vegetation' || view === 'lifeform-summary' || view === 'summary-environment' || view === 'summary-vegetation' || view === 'user-log' || view === 'subzone-matrix') && editorBusy) return;
    const current = ++stateRequest;
    stateReads.cancelAll();
    busy = true;
    error = '';
    try {
      const startup = await stateReads.track(LifecycleService.GetStartupState());
      if (current !== stateRequest) return;
      startupInfo = startup;
      if (!startup.ready) {
        startupFailure = startup;
        recoveryProjectPath = startup.projectPath ?? '';
        recoveryProjectName = startup.configuredProject ?? '';
        recoverySUPath = startup.suPath ?? '';
        recoveryClearSU = (startup.configuredSu ?? 'None') !== 'None';
        recoveryHierarchyPath = startup.hierarchyPath ?? '';
        recoveryClearHierarchy = (startup.configuredHierarchy ?? 'None') !== 'None';
        busy = false;
        return;
      }
      startupFailure = null;
      const state = await stateReads.track(ContextService.GetState());
      if (current !== stateRequest) return;
      projectState.set(state);
      if (profileNavigation && profileNavigation.contextId !== state.contextId) profileNavigation = null;
      await loadPlots(0);
      if (current !== stateRequest) return;
      await loadHierarchy();
    } catch (cause) {
      if (current === stateRequest) {
        error = String(cause);
        busy = false;
      }
    }
  }

  async function loadHierarchy() {
    if (projectReportBusy || twoPageVegetationBusy || twoPageSoilBusy || pictureCreationBusy || pictureLifecycleBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || summaryPreferenceBusy || externalSUEnvironmentBusy || (typeof companionPanelBusy !== 'undefined' && companionPanelBusy) || (view === 'long-vegetation' || view === 'lifeform-summary' || view === 'summary-environment' || view === 'summary-vegetation' || view === 'user-log' || view === 'subzone-matrix') && editorBusy) return;
    const current = ++hierarchyRequest;
    hierarchyReads.cancelAll();
    try {
      const nodes = await hierarchyReads.track(ContextService.GetHierarchyNodes());
      if (current === hierarchyRequest) hierarchyNodes = nodes ?? [];
    } catch (cause) {
      if (current === hierarchyRequest) throw cause;
    }
  }

  function selectedContext(): ContextSelection {
    const state = $projectState;
    if (!state?.projectPath) throw new Error('The active project file identity is unavailable.');
    return { project: state.activeProject, projectPath: state.projectPath,
      su: state.activeSU, suPath: state.suPath ?? '',
      hierarchy: state.activeHierarchy, hierarchyPath: state.hierarchyPath ?? '' };
  }

  async function inspectProfileSources() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileSourceError = 'Wait for the current operation or decision before inspecting profile sources.'; return;
    }
    busy = true; profileSourceError = '';
    try {
      await tick();
      const sources = await stateReads.track(ContextService.ListPlotProfileSources(contextId, profilePath));
      if ($projectState?.contextId !== contextId) throw new Error('Profile inspection belongs to an old context; inspect again.');
      if (!Array.isArray(sources)) throw new Error('Profile source inspection returned an incomplete collection.');
      const next = sources.map(validatePlotProfileSource);
      profileSources = next; profileSourceContext = contextId;
      if (next.length === 0) profileSourceError = 'No original physical *_Profile tables were found; selection remains unchanged.';
    } catch (cause) { profileSourceError = `Profile sources were not replaced: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function reviewProfileSU(proposal: ProjectPlotProfileFilterRequest, origin: string) {
    if (busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest ||
        $projectState?.contextId !== origin) throw new Error('Wait for the current owned operation before reviewing Save as SU.');
    busy = true; suError = '';
    try {
      await tick();
      const source = await stateReads.track(ContextService.ReviewProjectPlotProfileSU(origin, proposal));
      if ($projectState?.contextId !== origin) throw new Error('Save as SU review belongs to an old context.');
      suReview = validateProfileSUReview(source, proposal); suReviewContext = origin;
      suCommitted = false; suSuccess = '';
    } finally { busy = false; }
  }

  async function saveProfileSU() {
    if (!suReview || suCommitted || suReviewContext !== $projectState?.contextId) {
      suError = 'Review the current stored profile before creating one new SU file.'; return;
    }
    const proposal = $state.snapshot({ review: suReview, name: suName, path: suPath, confirmed: true });
    const origin = suReviewContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Save as SU proposal belongs to an old context.');
      busy = true; suError = '';
      try {
        await tick();
        const response = await ContextService.SaveProjectPlotProfileSU(contextId, proposal);
        suCommitted = true;
        const created = validateProfileSUCreated(response, proposal.name, proposal.review.plots.length);
        suSuccess = `Created ${created.name}_SU with ${created.plotCount} stored plots in ${created.path}. Current project/SU selection is unchanged.`;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `SU file published, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (suCommitted || String(cause).includes('Profile SU file published, but')) {
          suCommitted = true; suError = `SU file published; do not replay creation: ${String(cause)}`;
        } else suError = `No SU file published; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function reviewProfileSUProject() {
    const state = $projectState;
    if (!state?.contextId || !suReview || suReviewContext !== state.contextId ||
        busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      suProjectError = 'Wait for the current operation and review the current profile before reviewing a project-file SU.'; return;
    }
    if (!state.projectPath) {
      suProjectError = 'Current project has no owned physical destination; no SU table can be created.'; return;
    }
    const proposal = $state.snapshot(suReview.filter);
    busy = true; suProjectError = '';
    try {
      await tick();
      const source = await stateReads.track(ContextService.ReviewProjectPlotProfileSUInProject(state.contextId, proposal));
      if ($projectState?.contextId !== state.contextId) throw new Error('Project-file SU review belongs to an old context.');
      suProjectReview = validateProjectSUReview(source, proposal, state.activeProject, state.projectPath);
      suProjectContext = state.contextId; suProjectCommitted = false; suProjectSuccess = '';
    } catch (cause) { suProjectError = `No SU table created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function saveProfileSUProject() {
    if (!suProjectReview || suProjectCommitted || suProjectContext !== $projectState?.contextId) {
      suProjectError = 'Review the current project destination before creating one SU table.'; return;
    }
    const proposal = $state.snapshot({ review: suProjectReview, name: suProjectName, confirmed: true });
    const origin = suProjectContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Project-file SU proposal belongs to an old context.');
      busy = true; suProjectError = '';
      try {
        await tick();
        const response = await ContextService.SaveProjectPlotProfileSUInProject(contextId, proposal);
        suProjectCommitted = true;
        const created = validateProjectSUCreated(response, proposal.name, proposal.review.su.plots.length, proposal.review.path);
        suProjectSuccess = `Created ${created.name}_SU with ${created.plotCount} stored plots in ${created.path}. Existing data/descriptions and project/SU selection are unchanged; select it separately.`;
        view = 'plots'; editorPlotNumber = undefined;
        const state = await stateReads.track(ContextService.GetState());
        if (state.contextId !== contextId || $projectState?.contextId !== contextId) {
          throw new Error('SU table committed, but context changed before destination discovery refresh; do not replay creation.');
        }
        projectState.set(state);
        await loadPlots(0);
        if (error) error = `SU table committed, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (suProjectCommitted || String(cause).includes('Profile SU table committed, but')) {
          suProjectCommitted = true; suProjectError = `SU table committed; do not replay creation: ${String(cause)}`;
        } else suProjectError = `No SU table created; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function choosePlotProfile(source: PlotProfileSource, origin: string) {
    const proposal = { name: source.name, path: source.path };
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Profile source belongs to an old context; inspect again.');
      busy = true;
      try {
        await tick();
        const state = await ContextService.SelectPlotProfile(contextId, proposal);
        request++; hierarchyRequest++; stateRequest++;
        plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
        projectState.set(state); profileNavigation = null; profileSources = []; profileSourceContext = '';
        selected = null; page = null; editorPlotNumber = undefined; view = 'plots'; error = '';
        await loadPlots(0);
        if (error) error = `Profile selection committed, but refresh failed; do not replay selection: ${error}`;
      } finally { busy = false; }
    });
  }

  async function reviewProfileWriteOwnership() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) return;
    busy = true; profileWriteError = '';
    try {
      await tick();
      const review = await stateReads.track(ContextService.ReviewProjectPlotProfile(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Profile ownership review belongs to an old context.');
      profileWriteReview = validateProjectPlotProfileReview(review); profileWriteContext = contextId;
    } catch (cause) { profileWriteError = `Profile editing authorization unchanged: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function publishProfileWriteOwnership() {
    if (!profileWriteReview || profileWriteContext !== $projectState?.contextId) {
      profileWriteError = 'Review the current selected profile before changing write authorization.'; return;
    }
    const proposal = $state.snapshot({ review: profileWriteReview, enabled: !profileWriteReview.source.writable, confirmed: true });
    const origin = profileWriteContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Profile ownership review belongs to an old context.');
      busy = true; profileWriteError = '';
      let published = false;
      try {
        await tick();
        const state = await ContextService.SetPlotProfileEditing(contextId, proposal);
        published = true;
        request++; hierarchyRequest++; stateRequest++;
        plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
        projectState.set(state); profileNavigation = null; profileSources = []; profileSourceContext = '';
        profileWriteReview = null; profileWriteContext = '';
        selected = null; page = null; editorPlotNumber = undefined; view = 'plots'; error = '';
        await loadPlots(0);
        if (error) error = `Profile authorization committed, but refresh failed; do not replay authorization: ${error}`;
      } catch (cause) {
        profileWriteError = published ? `Profile authorization committed; do not replay: ${String(cause)}`
          : `Profile authorization unchanged; review retained for retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function switchContext(contextId: string, selection: ContextSelection, next: 'plots' | 'hierarchy') {
    request++;
    hierarchyRequest++;
    stateRequest++;
    plotReads.cancelAll();
    hierarchyReads.cancelAll();
    stateReads.cancelAll();
    busy = true;
    error = '';
    try {
      const state = await ContextService.SwitchContext(contextId, selection);
      request++;
      projectState.set(state);
      profileNavigation = null;
      profileSources = []; profileSourceContext = '';
      page = null;
      selected = null;
      hierarchyNodes = [];
      editorPlotNumber = undefined;
      view = next;
      await loadPlots(0);
      if (next === 'hierarchy') {
        try { await loadHierarchy(); }
        catch (cause) { error = `Context changed, but hierarchy refresh failed: ${String(cause)}`; }
      }
    } finally {
      busy = false;
    }
  }

  async function reviewBlankProfileFile() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileFileError = 'Wait for the current operation or decision before reviewing a new profile file.'; return;
    }
    busy = true; profileFileError = '';
    try {
      await tick();
      const response = await stateReads.track(ContextService.ReviewPlotProfileFileCreation(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Blank profile review belongs to an old context.');
      profileFileReview = validateProfileFileReview(response); profileFileContext = contextId;
      profileFileCommitted = false; profileFileSuccess = '';
    } catch (cause) { profileFileError = `No profile file created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function createBlankProfileFile() {
    if (!profileFileReview || profileFileCommitted || profileFileContext !== $projectState?.contextId) {
      profileFileError = 'Review the current original empty template before creating one new profile file.'; return;
    }
    const proposal = $state.snapshot({ review: profileFileReview, name: profileFileName, path: profileFilePath, confirmed: true });
    const origin = profileFileContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Blank profile proposal belongs to an old context.');
      busy = true; profileFileError = '';
      try {
        await tick();
        const response = await ContextService.CreatePlotProfileFile(contextId, proposal);
        profileFileCommitted = true;
        const created = validateProfileFileCreated(response, proposal.name);
        profileFileSuccess = `Created ${created.table} with zero rules in ${created.source.path}. Inspect, select and authorize it explicitly to begin editing; current context selection is unchanged.`;
        profilePath = created.source.path;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `Profile file published, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (profileFileCommitted || String(cause).includes('Profile file published, but')) {
          profileFileCommitted = true; profileFileError = `Profile file published; do not replay creation: ${String(cause)}`;
        } else profileFileError = `No profile file published; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function reviewBlankProfileTable() {
    const contextId = $projectState?.contextId;
    if (!contextId || busy || editorBusy || vegetationPublicationBusy || lifeformPublicationBusy || summaryPublicationBusy || transitionWorking || pendingTransition !== null || closeRequest) {
      profileTableError = 'Wait for the current operation or decision before reviewing an existing-file profile.'; return;
    }
    busy = true; profileTableError = '';
    try {
      await tick();
      const response = await stateReads.track(ContextService.ReviewPlotProfileTableCreation(contextId));
      if ($projectState?.contextId !== contextId) throw new Error('Existing-file profile review belongs to an old context.');
      profileTableReview = validateProfileTableReview(response); profileTableContext = contextId;
      profileTableCommitted = false; profileTableSuccess = '';
    } catch (cause) { profileTableError = `No profile table created; review failed: ${String(cause)}`; }
    finally { busy = false; }
  }

  async function createBlankProfileTable() {
    if (!profileTableReview || profileTableCommitted || profileTableContext !== $projectState?.contextId) {
      profileTableError = 'Review current selected-profile ownership and the original template before creating one table.'; return;
    }
    const proposal = $state.snapshot({ review: profileTableReview, name: profileTableName, confirmed: true });
    const origin = profileTableContext;
    await requestTransition(async contextId => {
      if (contextId !== origin) throw new Error('Existing-file profile proposal belongs to an old context.');
      busy = true; profileTableError = '';
      try {
        await tick();
        const response = await ContextService.CreatePlotProfileTable(contextId, proposal);
        profileTableCommitted = true;
        const created = validateProfileTableCreated(response, proposal.name, proposal.review.profile.source.source.path);
        profileTableSuccess = `Created ${created.table} with zero rules in ${created.source.path}. Existing data/descriptions and current selection are unchanged. Inspect and select it explicitly; selection clears external session grants.`;
        profilePath = created.source.path;
        view = 'plots'; editorPlotNumber = undefined;
        await loadPlots(0);
        if (error) error = `Profile table committed, but navigation refresh failed; do not replay creation: ${error}`;
      } catch (cause) {
        if (profileTableCommitted || String(cause).includes('Profile table committed, but')) {
          profileTableCommitted = true; profileTableError = `Profile table committed; do not replay creation: ${String(cause)}`;
        } else profileTableError = `No profile table created; proposal retained for correction or retry: ${String(cause)}`;
        throw cause;
      } finally { busy = false; }
    });
  }

  async function chooseProject(project: ProjectInfo) {
    await requestTransition(async (contextId) => {
      if (!project.path) throw new Error('Choose a project with an explicit file identity.');
      const selection = selectedContext();
      selection.project = project.name;
      selection.projectPath = project.path;
      selection.su = 'None';
      selection.suPath = '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function chooseSU(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    const chosen = $projectState?.sus?.[Number(select.value)];
    select.value = String(activeSUIndex);
    await requestTransition(async (contextId) => {
      const selection = selectedContext();
      selection.su = chosen?.name ?? 'None';
      selection.suPath = chosen?.path ?? '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function chooseHierarchy(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    const chosen = $projectState?.hierarchies?.[Number(select.value)];
    select.value = String(activeHierarchyIndex);
    await requestTransition(async (contextId) => {
      const selection = selectedContext();
      selection.hierarchy = chosen?.name ?? 'None';
      selection.hierarchyPath = chosen?.path ?? '';
      await switchContext(contextId, selection, 'hierarchy');
    });
  }

  async function attachExternalProject(event: SubmitEvent) {
    event.preventDefault();
    const name = externalProject, path = externalPath;
    await requestTransition(async (contextId) => {
      const selection = selectedContext();
      selection.project = name;
      selection.projectPath = path;
      selection.su = 'None';
      selection.suPath = '';
      await switchContext(contextId, selection, 'plots');
    });
  }

  async function browseExternalSQLiteProject() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    externalPickError = '';
    externalPickMessage = '';
    try {
      const inspection = await ContextService.ChooseExternalSQLiteFile(contextId);
      if (!inspection.selectedPath) return;
      externalPath = inspection.selectedPath;
      externalDiscoveredProjects = inspection.projects ?? [];
      const firstCompatible = externalDiscoveredProjects.find(p => p.compatible) ?? externalDiscoveredProjects[0];
      if (firstCompatible) {
        externalProject = firstCompatible.name;
        externalPickMessage = `Found ${externalDiscoveredProjects.length} project(s) in ${firstCompatible.file}.`;
      } else {
        externalPickMessage = 'Selected SQLite file contains no *_Env project table.';
      }
    } catch (cause) {
      externalPickError = `Could not inspect SQLite file: ${String(cause)}`;
    }
  }

  async function revealInExplorer(target: 'root' | 'projects' | 'project' | 'su' | 'hierarchy') {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    revealError = '';
    revealMessage = '';
    try {
      const opened = await ContextService.RevealInFileExplorer(contextId, target);
      revealMessage = `Opened in File Explorer: ${opened}`;
    } catch (cause) {
      revealError = `Could not open File Explorer: ${String(cause)}`;
    }
  }

  function applyAccessCandidate(candidate: AccessProjectCandidate | null) {
    if (!candidate) return;
    accessSelectedSourceProject = candidate.sourceProject;
    accessTargetProject = candidate.suggestedTarget;
    accessDestinationPath = candidate.suggestedDestPath;
  }

  async function browseAccessDatabase() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    accessError = '';
    accessResult = null;
    try {
      const chosen = await ImportExportService.ChooseAccessDatabaseFile(contextId);
      if (!chosen) return;
      accessSourcePath = chosen;
      await inspectAccessSource(chosen);
    } catch (cause) {
      accessError = `Could not select Access file: ${String(cause)}`;
    }
  }

  async function inspectAccessSource(pathToInspect = accessSourcePath) {
    const contextId = $projectState?.contextId;
    if (!contextId || !pathToInspect) return;
    accessWorking = true;
    accessError = '';
    accessResult = null;
    try {
      const inspection = await ImportExportService.InspectAccessDatabase(contextId, pathToInspect);
      accessInspection = inspection;
      accessSourcePath = inspection.sourcePath;
      const projects = inspection.projects ?? [];
      const first = projects.find(p => p.compatible) ?? projects[0] ?? null;
      applyAccessCandidate(first);
      if (!first) {
        accessError = 'No *_Env project tables were found in the selected Access database.';
      }
    } catch (cause) {
      accessInspection = null;
      accessError = `Access inspection failed: ${String(cause)}`;
    } finally {
      accessWorking = false;
    }
  }

  async function browseAccessDestinationPath() {
    const contextId = $projectState?.contextId;
    if (!contextId) return;
    accessError = '';
    try {
      const chosen = await ContextService.ChooseNewSQLiteProjectFile(contextId, accessTargetProject);
      if (chosen) accessDestinationPath = chosen;
    } catch (cause) {
      accessError = `Could not choose destination .db file: ${String(cause)}`;
    }
  }

  async function runAccessProjectImport(event: SubmitEvent) {
    event.preventDefault();
    if (!accessInspection || !accessSelectedSourceProject || !accessTargetProject || !accessDestinationPath) return;
    const sourcePath = accessInspection.sourcePath;
    const sourceProject = accessSelectedSourceProject;
    const targetProject = accessTargetProject;
    const destinationPath = accessDestinationPath;
    const activateOnSuccess = accessActivateOnSuccess;
    await requestTransition(async (contextId) => {
      accessWorking = true;
      accessError = '';
      accessResult = null;
      try {
        const result = await ImportExportService.ImportAccessProject(contextId, {
          sourcePath,
          sourceProject,
          targetProject,
          destinationPath,
          activateOnSuccess,
        });
        accessResult = result;
        if (result.state) {
          projectState.set(result.state);
        }
        if (result.activated) {
          profileNavigation = null;
          profileWriteReview = null; profileWriteContext = ''; profileWriteError = '';
          suReview = null; suReviewContext = ''; suError = ''; suSuccess = ''; suCommitted = false;
          suProjectReview = null; suProjectContext = ''; suProjectError = ''; suProjectSuccess = ''; suProjectCommitted = false;
          editorPlotNumber = undefined;
          view = 'plots';
          await loadPlots(0);
        }
      } catch (cause) {
        accessError = `Import failed: ${String(cause)}`;
        throw cause;
      } finally {
        accessWorking = false;
      }
    });
  }

  function openContextualHelp(topicOverride?: string, tabOverride: HelpDrawerTab = 'inspector') {
    const activeEl = typeof document !== 'undefined' ? document.activeElement : null;
    helpDrawerTopicId = topicOverride ?? resolveContextualHelpTarget(activeEl, view, editorEntryForm);
    helpDrawerTab = tabOverride;
    helpDrawerOpen = true;
  }

  function handleGlobalHelpKeydown(event: KeyboardEvent) {
    if (startupFailure !== null) return;
    const activeEl = typeof document !== 'undefined' ? (document.activeElement as HTMLElement | null) : null;
    const action = classifyGlobalHelpShortcut(event, activeEl);
    if (action === 'f1') {
      event.preventDefault();
      openContextualHelp();
      return;
    }
    if (action === 'shortcuts') {
      event.preventDefault();
      shortcutsModalOpen = !shortcutsModalOpen;
      return;
    }
    if (event.key === 'Escape' && helpDrawerOpen && !shortcutsModalOpen && !whatsNewModalOpen && !aboutModalOpen && !tourOpen && !resetSampleConfirmOpen && !closeRequest && !pendingTransition) {
      event.preventDefault();
      helpDrawerOpen = false;
    }
  }

  function handleHelpMenuAction(action: HelpMenuAction) {
    resetSampleNotice = '';
    if (action === 'guide') {
      openContextualHelp(undefined, 'inspector');
    } else if (action === 'tours') {
      tourInitialId = null;
      tourOpen = true;
    } else if (action === 'shortcuts') {
      shortcutsModalOpen = true;
    } else if (action === 'whats-new') {
      whatsNewModalOpen = true;
    } else if (action === 'reset-sample') {
      resetSampleConfirmOpen = true;
    } else if (action === 'about') {
      aboutModalOpen = true;
    }
  }

  async function acknowledgeUnreadWhatsNew() {
    if (!whatsNewStatus?.latestVersion) return;
    try {
      const updated = await defaultHelpPort.acknowledgeWhatsNew(whatsNewStatus.latestVersion);
      whatsNewStatus = updated;
    } catch (cause) {
      error = `Could not acknowledge What's New: ${String(cause)}`;
    }
  }

  async function confirmResetWorkspaceToSample() {
    resetSampleConfirmOpen = false;
    const ownedDiscardOnSample = $projectState?.activeProject === 'Sample' ? () => !editorBusy && !companionPanelBusy : undefined;
    await requestTransition(async (contextId) => {
      busy = true;
      error = '';
      resetSampleNotice = '';
      try {
        const nextState = await defaultHelpPort.resetWorkspaceToSample(contextId);
        request++;
        hierarchyRequest++;
        stateRequest++;
        plotReads.cancelAll();
        hierarchyReads.cancelAll();
        stateReads.cancelAll();
        projectState.set(nextState);
        profileNavigation = null;
        profileSources = [];
        profileSourceContext = '';
        selected = null;
        page = null;
        hierarchyNodes = [];
        editorPlotNumber = undefined;
        resetSampleConfirmOpen = false;
        view = 'plots';
        resetSampleNotice = 'Workspace reset to pristine Sample.db (Project: Sample, SU: Sample, Hierarchy: Sample). User project .db files were preserved.';
        await loadPlots(0);
        try {
          await loadHierarchy();
        } catch (cause) {
          error = `Sample workspace activated, but hierarchy refresh failed: ${String(cause)}`;
        }
      } finally {
        busy = false;
      }
    }, ownedDiscardOnSample);
  }

  onMount(() => {
    const stopCloseEvents = Events.On('vpro:close-request', (event) => {
      const requestId: unknown = event.data;
      if (typeof requestId !== 'string' || !requestId) {
        error = 'Invalid native window close request; VPRO remains open.';
        return;
      }
      void handleCloseRequest(requestId);
    });
    void LifecycleService.GetPendingCloseRequest().then((requestId) => {
      if (requestId) void handleCloseRequest(requestId);
    }).catch((cause) => { error = `Window close handler could not initialise: ${String(cause)}`; });
    void defaultHelpPort.getWhatsNewStatus().then((st) => {
      whatsNewStatus = st;
    }).catch(() => {});
    void refresh();
    return () => { request++; stopCloseEvents(); };
  });
</script>

<svelte:window onkeydown={handleGlobalHelpKeydown} />

<div class="app-shell">
  <header class="topbar">
    <div class="brand">
      <span class="brand-symbol" aria-hidden="true">V</span><span>VPro</span>
      {#if buildMode === 'dev'}
        <button
          type="button"
          data-build-tier-badge="dev"
          title="Local developer build — open About VPro & diagnostics"
          class="ml-1.5 rounded border border-[#FCBA19] bg-[#003366] px-2 py-0.5 font-mono text-[11px] font-semibold tracking-tight text-[#FCBA19] hover:bg-[#002244]"
          onclick={() => { if (!startupFailure) aboutModalOpen = true; }}
        >
          DEV BUILD · v{appVersion || '2.0.1-dev'}{shortBuildCommit ? ` · ${shortBuildCommit}` : ''}
        </button>
      {:else if buildMode === 'alpha'}
        <button
          type="button"
          data-build-tier-badge="alpha"
          title="Alpha preview build — open About VPro & verification details"
          class="ml-1.5 rounded border border-[#003366] bg-[#FCBA19] px-2 py-0.5 font-mono text-[11px] font-bold tracking-tight text-[#003366] hover:brightness-95"
          onclick={() => { if (!startupFailure) aboutModalOpen = true; }}
        >
          ALPHA PREVIEW (v{appVersion})
        </button>
      {:else if buildMode === 'release' && releaseChannel === 'experimental'}
        <button
          type="button"
          data-build-tier-badge="experimental"
          title="Experimental release channel enabled in config.yml"
          class="ml-1.5 rounded border border-white/40 bg-white/10 px-2 py-0.5 font-mono text-[11px] font-semibold uppercase tracking-tight text-white hover:bg-white/20"
          onclick={() => { if (!startupFailure) aboutModalOpen = true; }}
        >
          EXPERIMENTAL CHANNEL
        </button>
      {/if}
    </div>
    <button class="context-toggle" type="button" disabled={startupFailure !== null} aria-expanded={contextExpanded} aria-controls="project-context" onclick={() => contextExpanded = !contextExpanded}>Projects &amp; context</button>
    {#if !startupFailure}<Navigation {view} onnavigate={navigate}
      siviAvailable={siviStandaloneEnabled && !!selected && !!$projectState?.contextId}
      onopensivi={() => void openSIVI()}
      newSiviAvailable={!!$projectState?.contextId && !busy && !editorBusy && !companionPanelBusy && !transitionWorking}
      onopennewsivi={() => { newSiviModalOpen = true; }}
      metadataAvailable={view === 'fs882' && !!$projectState?.contextId && !!editor?.canOpenProjectMetadata()}
      onopenmetadata={() => void openProjectMetadata()}
      onopencloudsync={() => { void requestTransition(async () => { projectAdminTab = 'cloudsync'; projectAdminTabNonce++; view = 'project-admin'; error = ''; }); }}
      whatsNewUnreadCount={whatsNewStatus?.unreadCount ?? 0}
      resetSampleAvailable={!!$projectState?.contextId && !busy && !editorBusy && !companionPanelBusy && !transitionWorking}
      {releaseChannel}
      onhelpaction={handleHelpMenuAction} />{/if}
    <div class="topbar-context"><Database size={16} strokeWidth={1.8} /><span>{$projectState?.activeProject ?? 'No project'}</span></div>
  </header>

  <div class="workspace" class:context-collapsed={!contextExpanded || startupFailure !== null}>
    <aside id="project-context" class="sidebar" aria-label="Projects">
      <div class="sidebar-heading">
        <span>PROJECTS</span>
        <button type="button" class="icon-button" title="Reveal VPRO database folder in File Explorer" aria-label="Reveal in File Explorer" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('projects')}>
          <FolderOpen size={16} strokeWidth={1.8} />
        </button>
      </div>
      {#if revealMessage}<p role="status" class="px-3 py-1 text-xs text-emerald-800 break-words">{revealMessage}</p>{/if}
      {#if revealError}<p role="alert" class="px-3 py-1 text-xs text-red-800 break-words">{revealError}</p>{/if}
      <nav aria-label="Project list">
        {#each $projectState?.projects ?? [] as project ((project.path ?? project.file) + project.name)}
          <button class:active={project.name === $projectState?.activeProject && project.path === $projectState?.projectPath} disabled={!project.compatible || busy || editorBusy || companionPanelBusy || transitionWorking} title={project.compatible ? project.path : `${project.version} is not a complete VP08 project`} onclick={() => chooseProject(project)}>
            <span class="project-name">{project.name}
              <span class="project-location">{$projectState?.projects?.filter(item => item.name === project.name).length === 1 ? project.file : project.path}</span>
            </span>
            <span class="project-version">{project.version}</span>
          </button>
        {/each}
      </nav>
      <div class="su-picker">
        <label for="active-su">SITE UNIT</label>
        <select id="active-su" value={String(activeSUIndex)} onchange={chooseSU} disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState}>
          <option value="-1">None</option>
          {#each $projectState?.sus ?? [] as su, index ((su.path ?? '') + su.name)}
            <option value={String(index)} disabled={!su.compatible} title={su.path}>{su.name}{su.kind === 'master' ? ' (master)' : ''}</option>
          {/each}
        </select>
      </div>
      <div class="su-picker">
        <label for="active-hierarchy">HIERARCHY</label>
        <select id="active-hierarchy" value={String(activeHierarchyIndex)} onchange={chooseHierarchy} disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState}>
          <option value="-1">None</option>
          {#each $projectState?.hierarchies ?? [] as hierarchy, index ((hierarchy.path ?? hierarchy.file) + hierarchy.name)}
            <option value={String(index)} disabled={!hierarchy.compatible} title={hierarchy.path}>{hierarchy.name} ({hierarchy.file})</option>
          {/each}
        </select>
      </div>
      {#if externalProjectsEnabled}
        <details class="su-picker">
          <summary>Attach external SQLite project</summary>
          <form onsubmit={attachExternalProject}>
            <button type="button" id="browse-external-sqlite" disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState?.contextId} onclick={() => void browseExternalSQLiteProject()}>Browse SQLite .db file…</button>
            {#if externalPickMessage}<p role="status" class="text-emerald-800 break-words">{externalPickMessage}</p>{/if}
            {#if externalPickError}<p role="alert" class="text-red-800 break-words">{externalPickError}</p>{/if}
            {#if externalDiscoveredProjects.length > 1}
              <label for="external-project-select">DISCOVERED PROJECTS IN FILE</label>
              <select id="external-project-select" value={externalProject} onchange={(e) => { externalProject = (e.currentTarget as HTMLSelectElement).value; }} disabled={busy || editorBusy || companionPanelBusy || transitionWorking}>
                {#each externalDiscoveredProjects as p (p.name)}
                  <option value={p.name} disabled={!p.compatible}>{p.name} ({p.version})</option>
                {/each}
              </select>
            {/if}
            <label for="external-project">PROJECT PREFIX</label>
            <input id="external-project" bind:value={externalProject} required disabled={busy || editorBusy || companionPanelBusy || transitionWorking} />
            <label for="external-path">FULL SQLITE FILE PATH</label>
            <input id="external-path" bind:value={externalPath} required disabled={busy || editorBusy || companionPanelBusy || transitionWorking} />
            <p>Attaching opens the SQLite database in place without copying it; edits write directly to the attached file.</p>
            <button type="submit" disabled={busy || editorBusy || companionPanelBusy || transitionWorking}>Attach project</button>
          </form>
        </details>
        <details class="su-picker" id="access-import-wizard">
          <summary>Import Access (.mdb / .accdb) wizard</summary>
          <form onsubmit={runAccessProjectImport}>
            <button type="button" id="browse-access-db" disabled={busy || transitionWorking || accessWorking || !$projectState?.contextId} onclick={() => void browseAccessDatabase()}>Browse Access (.mdb / .accdb)…</button>
            <label for="access-source-path">ACCESS SOURCE FILE (.mdb / .accdb)</label>
            <input id="access-source-path" bind:value={accessSourcePath} placeholder="C:\path\to\project.accdb" disabled={busy || transitionWorking || accessWorking} />
            <button type="button" id="inspect-access-db" disabled={busy || transitionWorking || accessWorking || !accessSourcePath || !$projectState?.contextId} onclick={() => void inspectAccessSource()}>Inspect Access file</button>
            {#if accessInspection && (accessInspection.projects?.length ?? 0) > 0}
              <label for="access-source-project">SOURCE PROJECT IN ACCESS DB</label>
              <select id="access-source-project" value={accessSelectedSourceProject} onchange={(e) => {
                const chosen = (accessInspection?.projects ?? []).find(p => p.sourceProject === (e.currentTarget as HTMLSelectElement).value) ?? null;
                applyAccessCandidate(chosen);
              }} disabled={busy || transitionWorking || accessWorking}>
                {#each accessInspection.projects ?? [] as cand (cand.sourceProject)}
                  <option value={cand.sourceProject}>{cand.sourceProject} ({cand.sourceVersion} · {cand.plotCount} plots)</option>
                {/each}
              </select>
              {#if accessSelectedCandidate}
                <p class="text-xs break-words"><strong>Autoconversion:</strong> {accessSelectedCandidate.conversionMode}</p>
                <p class="text-xs break-words">{accessSelectedCandidate.statusMessage}</p>
                {#if (accessSelectedCandidate.optionalSUs?.length ?? 0) > 0}
                  <p class="text-xs break-words">SU tables included: {(accessSelectedCandidate.optionalSUs ?? []).join(', ')}</p>
                {/if}
              {/if}
              <label for="access-target-project">NEW VPRO PROJECT PREFIX</label>
              <input id="access-target-project" bind:value={accessTargetProject} required disabled={busy || transitionWorking || accessWorking} />
              <label for="access-dest-path">NEW SQLITE DESTINATION (.db)</label>
              <input id="access-dest-path" bind:value={accessDestinationPath} required disabled={busy || transitionWorking || accessWorking} />
              <button type="button" id="browse-access-dest" disabled={busy || transitionWorking || accessWorking || !$projectState?.contextId} onclick={() => void browseAccessDestinationPath()}>Browse destination .db…</button>
              <label class="flex items-center gap-2 text-xs">
                <input type="checkbox" style="width:auto;min-height:auto;" bind:checked={accessActivateOnSuccess} disabled={busy || transitionWorking || accessWorking} />
                Switch to imported project on completion
              </label>
              <button type="submit" id="convert-import-access-btn" disabled={busy || transitionWorking || accessWorking || !accessSelectedCandidate?.compatible || !accessTargetProject || !accessDestinationPath}>
                {accessWorking ? 'Converting & importing…' : 'Convert & import to VPRO DB'}
              </button>
            {/if}
            {#if accessError}<p role="alert" class="text-red-800 break-words">{accessError}</p>{/if}
            {#if accessResult}
              <p role="status" class="text-emerald-800 break-words">
                Imported {accessResult.targetProject} ({accessResult.plotCount} plots, {accessResult.conversionMode}) into {accessResult.destinationPath}.
              </p>
            {/if}
          </form>
        </details>
      {/if}
      <div class="su-picker">
        <label for="reveal-explorer-btn">DATA ON DISK</label>
        <div class="grid gap-1.5">
          <button type="button" id="reveal-explorer-btn" class="px-2 py-1.5 border rounded bg-white text-xs text-left" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('project')}>
            Reveal active project .db in File Explorer
          </button>
          <button type="button" id="reveal-root-explorer-btn" class="px-2 py-1.5 border rounded bg-white text-xs text-left" disabled={busy || transitionWorking || !$projectState?.contextId} onclick={() => void revealInExplorer('root')}>
            Open VPRO DBs root folder in File Explorer
          </button>
        </div>
      </div>
      <div class="sidebar-footer">{$projectState?.projects?.length ?? 0} project{($projectState?.projects?.length ?? 0) === 1 ? '' : 's'}</div>
    </aside>

    <main class="content">
      {#if !startupFailure && whatsNewStatus?.hasUnread && !whatsNewPromptDismissed}
        <section
          aria-label="Unread release notes"
          data-whats-new-banner
          class="mb-4 flex flex-wrap items-center justify-between gap-2 rounded border border-[#003366] border-l-[4px] border-l-[#FCBA19] bg-white px-3.5 py-2.5 text-xs text-[#313132]"
        >
          <div>
            <strong class="font-semibold text-[#003366]">What's New in VPro v{whatsNewStatus.latestVersion}:</strong>
            <span> {whatsNewStatus.unreadCount} unread release update{whatsNewStatus.unreadCount === 1 ? '' : 's'} available.</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="rounded bg-[#003366] px-2.5 py-1 text-xs font-semibold text-white hover:bg-[#002244]"
              onclick={() => { whatsNewModalOpen = true; }}
            >
              Read Release Notes
            </button>
            <button
              type="button"
              class="rounded border border-[#003366] bg-white px-2.5 py-1 text-xs font-semibold text-[#003366] hover:bg-slate-50"
              onclick={() => void acknowledgeUnreadWhatsNew()}
            >
              Mark as Read
            </button>
            <button
              type="button"
              class="rounded border border-slate-300 bg-white px-2 py-1 text-xs text-slate-600 hover:bg-slate-50"
              onclick={() => { whatsNewPromptDismissed = true; }}
            >
              Dismiss
            </button>
          </div>
        </section>
      {/if}
      {#if resetSampleNotice}
        <div role="status" data-reset-sample-notice class="mb-4 flex items-center justify-between gap-2 rounded border border-[#2E8540] bg-emerald-50 px-3.5 py-2 text-xs text-[#1E5128]">
          <span>{resetSampleNotice}</span>
          <button type="button" class="rounded border border-emerald-700/30 bg-white px-2 py-0.5 text-[11px]" onclick={() => { resetSampleNotice = ''; }}>Dismiss</button>
        </div>
      {/if}
      {#if suReview}
        <section class="m-2 p-3 border rounded" aria-label="Reviewed Save as SU">
          <h2 class="font-semibold">Save reviewed profile as a new SU file</h2>
          <p class="mt-2">{suReview.plots.length} stored plots; SiteUnit values {suReview.sourceSU ? 'copied from the independently reviewed selected SU' : 'remain NULL because no SU is selected'}.</p>
          <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
            <label for="profile-su-name" class="flex flex-col gap-1">New SU name
              <input id="profile-su-name" class="px-2 py-1 border rounded min-w-0" bind:value={suName} disabled={busy || transitionWorking || pendingTransition !== null || suCommitted} />
            </label>
            <label for="profile-su-path" class="flex flex-col gap-1">New SQLite file (absolute .db path)
              <input id="profile-su-path" class="px-2 py-1 border rounded min-w-0" bind:value={suPath} disabled={busy || transitionWorking || pendingTransition !== null || suCommitted} />
            </label>
          </div>
          {#if suError}<p role="alert" class="mt-2 text-red-800">{suError}</p>{/if}
          {#if suSuccess}<p role="status" class="mt-2 text-emerald-800">{suSuccess}</p>{/if}
          <div class="mt-3 overflow-x-auto">
            <table class="w-full text-sm"><thead><tr><th class="text-left">Plot number</th><th class="text-left">SiteUnit</th></tr></thead>
              <tbody>{#each suReview.plots.slice(0, 25) as plot}<tr><td>{plot.plotNumber}</td><td>{plot.siteUnit === null ? 'NULL (no value)' : plot.siteUnit === '' ? 'Empty text' : plot.siteUnit}</td></tr>{/each}</tbody>
            </table>
            {#if suReview.plots.length > 25}<p>Showing the first 25 of {suReview.plots.length} explicitly reviewed rows.</p>{/if}
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suCommitted || !suName || !suPath || suReviewContext !== $projectState?.contextId}
              onclick={() => void saveProfileSU()}>Create reviewed SU file</button>
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50" disabled={busy || transitionWorking || pendingTransition !== null}
              onclick={() => { suReview = null; suError = ''; suSuccess = ''; }}>Dismiss SU review</button>
          </div>
          <p class="mt-3 text-sm">Creates a new SQLite SU database without overwriting existing files or switching the active project/SU context. Table names follow the 31-character VPro identifier rule (None, Sample, and master names are reserved).</p>
          {#if suProjectEnabled}
            <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suReviewContext !== $projectState?.contextId}
              onclick={() => void reviewProfileSUProject()}>Review Save as SU in current project file</button>
            {#if suProjectReview}
              <section class="mt-3 p-3 border rounded" aria-label="Reviewed project-file SU creation">
                <h3 class="font-semibold">Add one SU table to the current owned project</h3>
                <p class="break-words">Destination: {suProjectReview.path}</p>
                <p>{suProjectReview.su.plots.length} stored plots and {suProjectReview.metadata.rows.length} original table descriptions reviewed. No descriptions are synthesized or removed.</p>
                <label for="profile-su-project-name" class="mt-3 flex flex-col gap-1">New project-file SU name
                  <input id="profile-su-project-name" class="px-2 py-1 border rounded min-w-0" bind:value={suProjectName}
                    disabled={busy || transitionWorking || pendingTransition !== null || suProjectCommitted} />
                </label>
                {#if suProjectSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{suProjectSuccess}</p>{/if}
                <div class="mt-3 flex flex-wrap gap-2">
                  <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
                    disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suProjectCommitted || !suProjectName || suProjectContext !== $projectState?.contextId}
                    onclick={() => void saveProfileSUProject()}>Create reviewed SU in project file</button>
                  <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
                    onclick={() => { suProjectReview = null; suProjectError = ''; suProjectSuccess = ''; }}>Dismiss project-file SU review</button>
                </div>
                <p class="mt-3 text-sm">Adds only the reviewed SU, its original indexes and transactional creation provenance. No implicit selection/attachment, Master authorization or YAML write. Arbitrary destinations and present template-description copying are unavailable; use new-file creation for present metadata. The existing desktop name policy applies.</p>
              </section>
            {/if}
            {#if suProjectError}<p role="alert" class="mt-2 text-red-800">{suProjectError}</p>{/if}
          {/if}
        </section>
      {/if}
      {#if profileSelectionEnabled && $projectState?.contextId}
        <details class="m-2 p-3 border rounded" aria-label="Stored plot profile selection">
          <summary class="font-semibold cursor-pointer">Plot profile: {$projectState.plotProfile.source.name}</summary>
          <p class="mt-2 text-sm">Select an optional SQLite database containing saved plot profile tables.</p>
          <div class="mt-2 flex flex-wrap items-end gap-2">
            <label for="plot-profile-path" class="flex flex-col gap-1 flex-1 min-w-0">SQLite profile database path (blank inspects project and core)
              <input id="plot-profile-path" class="px-2 py-1 border rounded min-w-0" bind:value={profilePath} disabled={busy || transitionWorking || pendingTransition !== null} />
            </label>
            <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void inspectProfileSources()}>Inspect stored profiles</button>
            <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void choosePlotProfile({name: 'None', path: ''}, $projectState?.contextId ?? '')}>Select no profile</button>
          </div>
          {#if profileSourceError}<p role="alert" class="mt-2 text-red-800">{profileSourceError}</p>{/if}
          <ul class="mt-2 space-y-2">
            {#each profileSources as info}
              <li class="flex flex-wrap items-center gap-2">
                <span>{info.table} - {info.source.path} ({info.writable ? 'project-owned editing' : 'read-only'})</span>
                <button type="button" class="px-3 py-1 border rounded disabled:opacity-50"
                  disabled={!info.available || profileSourceContext !== $projectState.contextId || busy || editorBusy || transitionWorking || pendingTransition !== null}
                  onclick={() => void choosePlotProfile(info.source, profileSourceContext)}>Select {info.table}</button>
                {#if !info.available}<span role="alert">{info.reason}</span>{/if}
              </li>
            {/each}
          </ul>
          {#if profileWriteOwnershipEnabled && $projectState.plotProfile.available &&
            ($projectState.plotProfile.source.name !== $projectState.activeProject || $projectState.plotProfile.source.path !== $projectState.projectPath)}
            <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void reviewProfileWriteOwnership()}>Review profile write ownership</button>
            {#if profileWriteReview}
              <section class="mt-3 p-3 border rounded" aria-label="Reviewed profile write ownership">
                <p>{profileWriteReview.table} in {profileWriteReview.source.source.path}</p>
                <p>{profileWriteReview.rules.rows.length} physical rules and {profileWriteReview.descriptions.rows.length} table descriptions reviewed.</p>
                {#if profileWriteReview.descriptions.columns.length === 0}<p>Description metadata is absent; no descriptions are inferred or created.</p>{/if}
                <p>Authorization applies only to this selected profile file/table for the current session. Context changes or restart clear it. Parent/child writes remain project-owned; support database writes are unavailable.</p>
                <button type="button" class="mt-2 px-3 py-2 border rounded disabled:opacity-50"
                  disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileWriteContext !== $projectState.contextId}
                  onclick={() => void publishProfileWriteOwnership()}>{profileWriteReview.source.writable ? 'Stop selected profile editing' : 'Authorize selected profile editing'}</button>
                <button type="button" class="mt-2 ml-2 px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
                  onclick={() => { profileWriteReview = null; profileWriteError = ''; }}>Dismiss ownership review</button>
              </section>
            {/if}
            {#if profileWriteError}<p role="alert" class="mt-2 text-red-800">{profileWriteError}</p>{/if}
          {/if}
          <p class="mt-2 text-sm">Profile rules are independent of project/SU selection. External and other-table profiles are read-only unless explicitly authorized for this session.</p>
          {#if profileFileCreationEnabled}
            <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void reviewBlankProfileFile()}>Review new blank profile file</button>
            {#if profileFileReview}
              <section class="mt-3 p-3 border rounded" aria-label="Reviewed blank profile file creation">
                <h2 class="font-semibold">Create one new empty profile file</h2>
                <p>Original nine nullable fields, zero rules, no template indexes/triggers and absent description metadata. No descriptions or legacy population values are invented.</p>
                <div class="mt-3 grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <label for="profile-file-name" class="flex flex-col gap-1">New profile name
                    <input id="profile-file-name" class="px-2 py-1 border rounded min-w-0" bind:value={profileFileName} disabled={busy || transitionWorking || pendingTransition !== null || profileFileCommitted} />
                  </label>
                  <label for="profile-file-path" class="flex flex-col gap-1">New SQLite file (absolute .db path)
                    <input id="profile-file-path" class="px-2 py-1 border rounded min-w-0" bind:value={profileFilePath} disabled={busy || transitionWorking || pendingTransition !== null || profileFileCommitted} />
                  </label>
                </div>
                {#if profileFileSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{profileFileSuccess}</p>{/if}
                <div class="mt-3 flex flex-wrap gap-2">
                  <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
                    disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileFileCommitted || !profileFileName || !profileFilePath || profileFileContext !== $projectState.contextId}
                    onclick={() => void createBlankProfileFile()}>Create reviewed blank profile file</button>
                  <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
                    onclick={() => { profileFileReview = null; profileFileError = ''; profileFileSuccess = ''; }}>Dismiss blank profile review</button>
                </div>
                <p class="mt-3 text-sm">Creates a new SQLite profile database without overwriting existing files or altering active session selections. Profile names follow the 31-character ASCII VPro identifier rule (None, Sample, and master names are reserved).</p>
              </section>
            {/if}
            {#if profileFileError}<p role="alert" class="mt-2 text-red-800">{profileFileError}</p>{/if}
          {/if}
          {#if profileTableCreationEnabled && $projectState.plotProfile.writable}
            <button type="button" class="mt-3 px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void reviewBlankProfileTable()}>Review blank profile in selected file</button>
            {#if profileTableReview}
              <section class="mt-3 p-3 border rounded" aria-label="Reviewed existing-file profile creation">
                <h2 class="font-semibold">Create one empty profile in the owned selected file</h2>
                <p class="break-words">Destination: {profileTableReview.profile.source.source.path}</p>
                <p>Current ownership, selected rules/descriptions and the original empty nine-field template are reviewed independently.</p>
                <label for="profile-table-name" class="mt-3 flex flex-col gap-1">New profile table name
                  <input id="profile-table-name" class="px-2 py-1 border rounded min-w-0" bind:value={profileTableName}
                    disabled={busy || transitionWorking || pendingTransition !== null || profileTableCommitted} />
                </label>
                {#if profileTableSuccess}<p role="status" class="mt-2 text-emerald-800 break-words">{profileTableSuccess}</p>{/if}
                <div class="mt-3 flex flex-wrap gap-2">
                  <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
                    disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileTableCommitted || !profileTableName || profileTableContext !== $projectState.contextId}
                    onclick={() => void createBlankProfileTable()}>Create reviewed profile in selected file</button>
                  <button type="button" class="px-3 py-2 border rounded" disabled={busy || transitionWorking || pendingTransition !== null}
                    onclick={() => { profileTableReview = null; profileTableError = ''; profileTableSuccess = ''; }}>Dismiss existing-file profile review</button>
                </div>
                <p class="mt-3 text-sm">Adds one table and transactional creation provenance; never replaces existing objects or creates descriptions. No automatic selection/attachment or new write grant. Arbitrary unselected destinations and legacy population remain unavailable. The existing desktop name policy applies.</p>
              </section>
            {/if}
            {#if profileTableError}<p role="alert" class="mt-2 text-red-800">{profileTableError}</p>{/if}
          {/if}
        </details>
      {/if}
      {#if profileNavigation}
        <section class="m-2 p-2 border rounded flex flex-wrap items-center gap-3" aria-label="Active profile navigation">
          <span>{profileNavigation.plots.length} of {profileNavigation.result.totalPlots} stored plots in reviewed profile navigation.</span>
          {#if view === 'fs882'}
            <span>Record {profileIndex + 1} of {profileNavigation.plots.length}</span>
            <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileIndex <= 0}
              onclick={() => void navigateProfilePlot(-1)}>Previous profile plot</button>
            <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || profileIndex < 0 || profileIndex >= profileNavigation.plots.length - 1}
              onclick={() => void navigateProfilePlot(1)}>Next profile plot</button>
          {/if}
          <button type="button" class="px-3 py-1 border rounded disabled:opacity-50" disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
            onclick={() => void clearProfileNavigation()}>Clear profile navigation</button>
        </section>
      {/if}
      {#if $projectState?.diagnostics?.length}
        <div class="project-warning" role="status">
          <strong>Some project files could not be opened.</strong>
          {#each $projectState.diagnostics as diagnostic (diagnostic.file)}
            <div>{diagnostic.file}: {diagnostic.message}</div>
          {/each}
        </div>
      {/if}
      {#if startupFailure}
        <section class="startup-recovery space-y-4 p-5 border rounded bg-white max-w-3xl" role="alert" data-startup-recovery>
          <div>
            <div class="eyebrow">Startup Recovery</div>
            <h1 class="text-xl font-semibold">Saved context could not be opened</h1>
          </div>
          <p class="text-sm">{startupFailure.message}</p>
          <p class="text-xs text-slate-600">
            Saved preferences in <code>{startupFailure.configPath}</code> and existing databases have not been modified yet.
            Choose an option below — VPro will automatically back up your current <code>config.yml</code> to
            <code>config.backup-&lt;timestamp&gt;.yml</code> before updating settings.
          </p>
          {#if recoveryError}
            <p role="alert" class="error text-xs" data-startup-recovery-error>{recoveryError}</p>
          {/if}

          <div class="p-3.5 border rounded bg-slate-50 space-y-2">
            <h2 class="text-sm font-semibold">Option 1: Open built-in Sample project</h2>
            <p class="text-xs text-slate-700">
              Backs up your prior <code>config.yml</code> and opens VPro immediately with the built-in <strong>Sample</strong> project.
              None of your existing <code>.db</code> project files are deleted or altered.
            </p>
            <button
              type="button"
              data-startup-fallback-sample
              disabled={recoveryWorking || busy}
              class="px-3.5 py-1.5 rounded bg-[#003366] text-white text-xs font-semibold hover:bg-[#002244] disabled:opacity-50"
              onclick={() => void fallbackStartupToSample()}
            >
              {recoveryWorking ? 'Recovering...' : 'Fall back to Sample project (backup config.yml)'}
            </button>
          </div>

          <form
            class="p-3.5 border rounded space-y-3"
            onsubmit={(event) => { event.preventDefault(); void recoverStartupPaths(); }}
          >
            <h2 class="text-sm font-semibold">Option 2: Relocate missing database path</h2>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
              <label for="recovery-project-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
                Project SQLite database (.db) path
                <input
                  id="recovery-project-path"
                  class="px-2.5 py-1.5 border rounded text-xs font-mono"
                  bind:value={recoveryProjectPath}
                  placeholder="/path/to/Project.db"
                  disabled={recoveryWorking || busy}
                />
              </label>
              <button
                type="button"
                data-startup-browse-project
                disabled={recoveryWorking || busy}
                class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
                onclick={() => void browseStartupDatabase('project')}
              >
                Browse .db...
              </button>
            </div>
            <label for="recovery-project-name" class="flex flex-col gap-1 text-xs font-medium max-w-xs">
              Project name inside file (optional when single-project .db)
              <input
                id="recovery-project-name"
                class="px-2.5 py-1.5 border rounded text-xs"
                bind:value={recoveryProjectName}
                placeholder={startupFailure.configuredProject || 'Auto-detect'}
                disabled={recoveryWorking || busy}
              />
            </label>

            <div class="flex flex-wrap gap-4 text-xs">
              <label class="inline-flex items-center gap-1.5">
                <input type="checkbox" bind:checked={recoveryClearSU} disabled={recoveryWorking || busy} />
                <span>Reset missing Site Unit (SU) selection to None</span>
              </label>
              <label class="inline-flex items-center gap-1.5">
                <input type="checkbox" bind:checked={recoveryClearHierarchy} disabled={recoveryWorking || busy} />
                <span>Reset missing Hierarchy selection to None</span>
              </label>
            </div>

            {#if !recoveryClearSU && (startupFailure.configuredSu || 'None') !== 'None'}
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
                <label for="recovery-su-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
                  Relocated SU database (.db) path ({startupFailure.configuredSu})
                  <input
                    id="recovery-su-path"
                    class="px-2.5 py-1.5 border rounded text-xs font-mono"
                    bind:value={recoverySUPath}
                    disabled={recoveryWorking || busy}
                  />
                </label>
                <button
                  type="button"
                  disabled={recoveryWorking || busy}
                  class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
                  onclick={() => void browseStartupDatabase('su')}
                >
                  Browse SU .db...
                </button>
              </div>
            {/if}

            {#if !recoveryClearHierarchy && (startupFailure.configuredHierarchy || 'None') !== 'None'}
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-2 items-end">
                <label for="recovery-hierarchy-path" class="sm:col-span-2 flex flex-col gap-1 text-xs font-medium">
                  Relocated Hierarchy database (.db) path ({startupFailure.configuredHierarchy})
                  <input
                    id="recovery-hierarchy-path"
                    class="px-2.5 py-1.5 border rounded text-xs font-mono"
                    bind:value={recoveryHierarchyPath}
                    disabled={recoveryWorking || busy}
                  />
                </label>
                <button
                  type="button"
                  disabled={recoveryWorking || busy}
                  class="px-3 py-1.5 border rounded text-xs font-medium hover:bg-slate-50 disabled:opacity-50"
                  onclick={() => void browseStartupDatabase('hierarchy')}
                >
                  Browse Hierarchy .db...
                </button>
              </div>
            {/if}

            <div>
              <button
                type="submit"
                data-startup-relocate-open
                disabled={recoveryWorking || busy || !recoveryProjectPath.trim()}
                class="px-3.5 py-1.5 rounded border border-[#003366] text-[#003366] text-xs font-semibold hover:bg-slate-50 disabled:opacity-50"
              >
                {recoveryWorking ? 'Opening...' : 'Update paths & open project'}
              </button>
            </div>
          </form>
        </section>
      {:else if view === 'home'}
        <div class="content-heading"><div><div class="eyebrow">VPro</div><h1>Home</h1></div></div>
        {#if startupBackupNotice}
          <div role="status" data-startup-backup-notice class="mb-3 p-2.5 border rounded bg-emerald-50 text-emerald-900 text-xs flex items-center justify-between gap-2">
            <span>{startupBackupNotice}</span>
            <button type="button" class="underline text-xs" onclick={() => { startupBackupNotice = ''; }}>Dismiss</button>
          </div>
        {/if}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <HomeWorkspaceHub
          activeProject={$projectState?.activeProject ?? 'No project selected'}
          projectPath={$projectState?.projectPath ?? ''}
          activeSU={$projectState?.activeSU ?? 'None'}
          activeHierarchy={$projectState?.activeHierarchy ?? 'None'}
          plotCount={page?.total ?? 0}
          {buildMode}
          {appVersion}
          gitCommit={startupInfo?.gitCommit ?? ''}
          {releaseChannel}
          onnavigate={(nextView) => navigate(nextView)}
          onhelpaction={(action) => handleHelpMenuAction(action)}
        />
      {:else if view === 'fs882'}
        <div class="h-full flex flex-col p-2">
          {#if error}<p class="error" role="alert">{error}</p>{/if}
          {#if plotFindEnabled && plotFindOpen}
            <section aria-label="Find scoped plot" class="mb-2 p-3 border rounded">
              <form onsubmit={(event) => { event.preventDefault(); void findScopedPlot(); }} class="flex flex-wrap items-end gap-2">
                <label for="find-scoped-plot" class="flex flex-col gap-1 flex-1 min-w-0">Exact plot number
                  <input id="find-scoped-plot" class="px-2 py-1 border rounded min-w-0" bind:value={plotFindQuery}
                    oninput={() => { plotFindResult = null; plotFindError = ''; }}
                    disabled={busy || editorBusy || transitionWorking || pendingTransition !== null} />
                </label>
                <button type="submit" class="px-3 py-1 border rounded disabled:opacity-50"
                  disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}>Find in current scope</button>
                <button type="button" class="px-3 py-1 border rounded disabled:opacity-50"
                  disabled={busy || transitionWorking || pendingTransition !== null}
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
          <FS882Form
            bind:this={editor}
            plotNumber={editorPlotNumber}
            contextId={$projectState?.contextId ?? ''}
            entryForm={editorEntryForm}
            onSaved={async (p) => { editorPlotNumber = p; await loadPlots(offset); }}
            onClosed={() => navigate('plots')}
            onBusyChange={(busy) => { localEditorBusy = busy; }}
            onProfileNavigation={applyProfileNavigation}
            onProfileSUReview={reviewProfileSU}
            onFindPlot={plotFindEnabled ? () => { plotFindOpen = true; } : undefined}
          />
          {:catch error}
            <p class="error" role="alert" data-entry-load-error="fs882">{editorLabel} editor could not load: {String(error)}</p>
          {/await}
          {/key}
          {/key}
          {/key}
          </fieldset>
        </div>

          <CloseConfirm requestId={closeRequest || (pendingTransition ? 'context' : '')}
          purpose={closeRequest ? 'close' : 'context'}
          {editorLabel}
          working={closeWorking || transitionWorking} canSave={closeState?.canSave ?? false}
          saveReason={closeState?.saveReason ?? `Return to the ${editorLabel} editor before saving.`}
          error={closeRequest ? closeError : transitionError}
          onrespond={closeRequest ? respondToClose : respondToTransition} />
        {#if editorPlotNumber && $projectState?.contextId}
          {#await import('./LMH25DiagnosticsBar.svelte') then { default: LMH25DiagnosticsBar }}
            <div class="px-2 pb-2">
              <LMH25DiagnosticsBar contextId={$projectState.contextId} plotNumber={editorPlotNumber} />
            </div>
          {/await}
        {/if}
      {:else if view === 'datasheet' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./UniversalDatasheetGrid.svelte')}
            <p role="status" class="p-4 text-sm">Loading Universal Datasheet Grid...</p>
          {:then { default: UniversalDatasheetGrid }}
            <UniversalDatasheetGrid
              contextId={$projectState.contextId ?? ''}
              project={$projectState.activeProject}
              su={$projectState.activeSU}
              hierarchy={$projectState.activeHierarchy}
              onBusyChange={(value) => { localEditorBusy = value; }}
              onDirtyChange={(dirty) => { workbenchDirtyLabel = dirty ? 'datasheet' : ''; }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }}
            />
          {/await}
        {/key}
      {:else if view === 'classification-explorer' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./ClassificationExplorer.svelte')}
          <p role="status" class="p-4 text-sm">Loading Classification Explorer...</p>
        {:then { default: ClassificationExplorer }}
          <ClassificationExplorer
            contextId={$projectState.contextId ?? ''}
            project={$projectState.activeProject}
            su={$projectState.activeSU}
            hierarchy={$projectState.activeHierarchy}
            operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
            onBusyChange={(value) => { localEditorBusy = value; }}
            onDirtyChange={(dirty) => { workbenchDirtyLabel = dirty ? 'classification-explorer' : ''; }}
            onContextChanged={async () => { await refresh(); }}
            onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }}
          />
        {/await}
      {:else if view === 'project-admin' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <div class="p-3 space-y-4">
          {#await import('./ProjectAdminWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading Project Admin Workbench...</p>
          {:then { default: ProjectAdminWorkbench }}
            <ProjectAdminWorkbench
              contextId={$projectState.contextId ?? ''}
              activeProject={$projectState.activeProject}
              activeSu={$projectState.activeSU}
              initialTab={projectAdminTab}
              initialTabNonce={projectAdminTabNonce}
              operationHeld={busy || companionPanelBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }}
              oncontextswitched={async () => { await refresh(); }}
            />
          {/await}
          {#await import('./ProjectMetadataPresets.svelte') then { default: ProjectMetadataPresets }}
            <ProjectMetadataPresets
              contextId={$projectState.contextId ?? ''}
              projectId=""
              operationHeld={busy || localEditorBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { secondaryPanelBusy = value; }}
            />
          {/await}
        </div>
      {:else if view === 'lump-editor' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./LumpEditor.svelte')}
            <p role="status" class="p-4 text-sm">Loading Combine Species &amp; LumpSpec Editor...</p>
          {:then { default: LumpEditor }}
            <LumpEditor
              contextId={$projectState.contextId ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }}
            />
          {/await}
        {/key}
      {:else if view === 'diagnostic-synthesis' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./ClassificationSynthesisView.svelte')}
            <p role="status" class="p-4 text-sm">Loading BEC Diagnostic &amp; Pairwise Synthesis Workbench...</p>
          {:then { default: ClassificationSynthesisView }}
            <ClassificationSynthesisView
              contextId={$projectState.contextId ?? ''}
              project={$projectState.activeProject}
              su={$projectState.activeSU}
              onBusyChange={(value) => { localEditorBusy = value; }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }}
            />
          {/await}
        {/key}
      {:else if view === 'r-sidecar' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./RSidecarWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading R Sidecar, climr &amp; Quarto Workbench...</p>
          {:then { default: RSidecarWorkbench }}
            <RSidecarWorkbench
              contextId={$projectState.contextId ?? ''}
              activeProject={$projectState.activeProject}
              activeSu={$projectState.activeSU}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }}
              oncontextswitched={async () => { await refresh(); }}
            />
          {/await}
        {/key}
      {:else if view === 'geospatial-map' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./GeospatialWorkbench.svelte')}
            <p role="status" class="p-4 text-sm">Loading 3D Geospatial Workbench...</p>
          {:then { default: GeospatialWorkbench }}
            <GeospatialWorkbench
              contextId={$projectState.contextId ?? ''}
              project={$projectState.activeProject}
              su={$projectState.activeSU}
              onBusyChange={(value) => { localEditorBusy = value; }}
              onContextChanged={async () => { await refresh(); }}
              onOpenPlot={(p) => { void requestTransition(async () => { editorPlotNumber = p; editorEntryForm = 'fs882'; view = 'fs882'; error = ''; }); }}
            />
          {/await}
        {/key}
      {:else if view === 'interchange-hub' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#key $projectState.contextId}
          {#await import('./InterchangeHub.svelte')}
            <p role="status" class="p-4 text-sm">Loading Interchange Hub...</p>
          {:then { default: InterchangeHub }}
            <InterchangeHub
              contextId={$projectState.contextId ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }}
              onProjectActivated={async () => { await refresh(); }}
            />
          {/await}
        {/key}
      {:else if view === 'summary-environment' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./SiteUnitSummary.svelte')}
          <p role="status" class="p-4 text-sm">Loading Site Unit Summary...</p>
        {:then { default: SiteUnitSummary }}
          {#key $projectState.contextId}
            <SiteUnitSummary contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU}
              suPath={$projectState.suPath ?? ''} onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Site Unit Summary could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'long-environment' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./LongEnvironmentReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Long Environment Report...</p>
        {:then { default: LongEnvironmentReport }}
          {#key $projectState.contextId}
            <LongEnvironmentReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Long Environment Report could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'long-vegetation' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./LongVegetationReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Long Vegetation Report...</p>
        {:then { default: LongVegetationReport }}
          {#key $projectState.contextId}
            <LongVegetationReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Long Vegetation Report could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'summary-vegetation' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./SummaryVegetationReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Summary Vegetation Report...</p>
        {:then { default: SummaryVegetationReport }}
          {#key $projectState.contextId}
            <SummaryVegetationReport contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} />
            {#await import('./SiteUnitExclusionFilter.svelte') then { default: SiteUnitExclusionFilter }}
              <div class="px-3 pb-4">
                <SiteUnitExclusionFilter
                  contextId={$projectState.contextId ?? ''}
                  su={$projectState.activeSU}
                  hierarchy={$projectState.activeHierarchy}
                  reportMode="summary"
                  onBusyChange={(value) => { secondaryPanelBusy = value; }}
                />
              </div>
            {/await}
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Summary Vegetation Report could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'lifeform-summary' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./LifeformSummary.svelte')}
          <p role="status" class="p-4 text-sm">Loading Lifeform Summary...</p>
        {:then { default: LifeformSummary }}
          {#key $projectState.contextId}
            <LifeformSummary contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Lifeform Summary could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'table-csv' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./ProjectTableCSVReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Table CSV Review...</p>
        {:then { default: ProjectTableCSVReview }}
          {#key $projectState.contextId}
            <ProjectTableCSVReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Table CSV Review could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'user-log' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./UserLogReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading User Log Review...</p>
        {:then { default: UserLogReview }}
          {#key $projectState.contextId}
            <UserLogReview contextId={$projectState.contextId ?? ''}
              operationHeld={busy || transitionWorking || pendingTransition !== null || !!closeRequest}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">User Log Review could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'plot-label-preview' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./PlotLabelPreview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Plot Label Preview...</p>
        {:then { default: PlotLabelPreview }}
          {#key $projectState.contextId}
            <PlotLabelPreview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              initialPlot={plotLabelNumber} onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Plot Label Preview could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'picture-manager' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./PictureManager.svelte')}
          <p role="status" class="p-4 text-sm">Loading Picture Manager...</p>
        {:then { default: PictureManager }}
          {#key $projectState.contextId}
            <PictureManager contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              initialPlot={pictureManagerPlot} onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Picture Manager could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'plot-locations' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./PlotLocationReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Plot Location Review...</p>
        {:then { default: PlotLocationReview }}
          {#key $projectState.contextId}
            <PlotLocationReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Plot Location Review could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'google-earth-review' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./GoogleEarthReview.svelte')}
          <p role="status" class="p-4 text-sm">Loading Google Earth Review...</p>
        {:then { default: GoogleEarthReview }}
          {#key $projectState.contextId}
            <GoogleEarthReview contextId={$projectState.contextId ?? ''} project={$projectState.activeProject}
              projectPath={$projectState.projectPath ?? ''} su={$projectState.activeSU} suPath={$projectState.suPath ?? ''}
              onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Google Earth Review could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'subzone-matrix' && $projectState}
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        {#await import('./SubzoneMatrixReport.svelte')}
          <p role="status" class="p-4 text-sm">Loading Subzone Matrix Report...</p>
        {:then { default: SubzoneMatrixReport }}
          {#key $projectState.contextId}
            <SubzoneMatrixReport contextId={$projectState.contextId ?? ''} hierarchy={$projectState.activeHierarchy}
              hierarchyPath={$projectState.hierarchyPath ?? ''} onBusyChange={(value) => { localEditorBusy = value; }} />
          {/key}
        {:catch loadError}
          <p class="error" role="alert">Subzone Matrix Report could not load: {String(loadError)}</p>
        {/await}
      {:else if view === 'hierarchy'}
      <div class="content-heading">
        <div><div class="eyebrow">{$projectState?.hierarchyFile ?? 'HIERARCHY'}</div><h1>Hierarchy</h1></div>
        <button class="icon-button" title="Refresh hierarchy" aria-label="Refresh hierarchy" disabled={busy} onclick={refresh}><RefreshCw size={18} strokeWidth={1.8} /></button>
      </div>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div class="table-bar"><span>{hierarchyNodes.length} units</span><span>{$projectState?.activeHierarchy ?? 'None'}</span></div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>Unit</th><th>ID</th><th>Parent ID</th><th>Level</th></tr></thead>
          <tbody>
            {#each hierarchyNodes as node (node.id)}
              <tr><td>{node.name ?? 'Unnamed'}</td><td>{node.id}</td><td>{node.parent ?? '—'}</td><td>{node.level ?? '—'}</td></tr>
            {:else}
              <tr><td colspan="4" class="empty-row">No hierarchy selected.</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
      {:else}
      <div class="content-heading">
        <div><div class="eyebrow">{$projectState?.activeProject ?? 'PROJECT'}</div><h1>Plots</h1></div>
        <button class="icon-button" title="Refresh projects and plots" aria-label="Refresh projects and plots" disabled={busy} onclick={refresh}><RefreshCw size={18} strokeWidth={1.8} /></button>
      </div>
      {#if error}<p class="error" role="alert">{error}</p>{/if}
      <div class="table-bar"><span>{page?.total ?? 0} plots</span><span>{busy ? 'Loading...' : 'Project data'}</span></div>
      <div class="table-scroll">
        <table>
          <thead><tr><th>Plot number</th><th>Field number</th><th>Representing</th><th>Zone</th><th>Subzone</th><th>Site series</th></tr></thead>
          <tbody>
            {#each page?.plots ?? [] as plot (plot.plotNumber)}
              <tr class:selected={selected?.plotNumber === plot.plotNumber}>
                <td><button class="plot-link" onclick={() => selected = plot}>{plot.plotNumber}</button></td>
                <td>{plot.fieldNumber ?? '—'}</td><td>{plot.plotRepresenting ?? '—'}</td>
                <td>{plot.zone ?? '—'}</td><td>{plot.subZone ?? '—'}</td><td>{plot.siteSeries ?? '—'}</td>
              </tr>
            {:else}
              <tr><td colspan="6" class="empty-row">{busy ? 'Loading plots...' : 'No plots in this project.'}</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
      <div class="table-footer">
        <span>{page?.total ? `${offset + 1}–${Math.min(offset + pageSize, page.total)} of ${page.total}` : '0 of 0'}</span>
        <div class="page-controls">
          <button class="icon-button" title="Previous page" aria-label="Previous page" disabled={busy || offset === 0} onclick={() => loadPlots(Math.max(0, offset - pageSize))}><ChevronLeft size={18} /></button>
          <button class="icon-button" title="Next page" aria-label="Next page" disabled={busy || offset + pageSize >= (page?.total ?? 0)} onclick={() => loadPlots(offset + pageSize)}><ChevronRight size={18} /></button>
        </div>
      </div>
      {/if}
    </main>

    {#if selected && view === 'plots'}
      <aside class="detail" aria-label="Selected plot">
        <div class="eyebrow">SELECTED PLOT</div><h2>{selected.plotNumber}</h2>
        <dl><dt>Field number</dt><dd>{selected.fieldNumber ?? '—'}</dd><dt>Representing</dt><dd>{selected.plotRepresenting ?? '—'}</dd><dt>Zone</dt><dd>{selected.zone ?? '—'}</dd><dt>Subzone</dt><dd>{selected.subZone ?? '—'}</dd><dt>Site series</dt><dd>{selected.siteSeries ?? '—'}</dd></dl>
        <div class="mt-4 pt-3 border-t border-stone-200">
          <button
            type="button"
            class="w-full py-1.5 px-3 bg-emerald-700 hover:bg-emerald-800 text-white rounded text-xs font-semibold"
            onclick={() => navigate('fs882')}
          >
            Open in FS882 Editor
          </button>
          {#if siviStandaloneEnabled}
            <button type="button" data-sivi-standalone-open
              class="mt-2 w-full py-1.5 px-3 border rounded text-xs font-semibold"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
              onclick={() => void openSIVI()}>Open SIVI / FS1333</button>
          {/if}
        </div>
      </aside>
    {/if}
    {#if newSiviModalOpen && $projectState?.contextId}
      {#await import('./SIVIParentCreateModal.svelte') then { default: SIVIParentCreateModal }}
        <SIVIParentCreateModal
          contextId={$projectState.contextId}
          activeProject={$projectState.activeProject}
          activeSu={$projectState.activeSU}
          open={newSiviModalOpen}
          operationHeld={busy || editorBusy || secondaryPanelBusy || transitionWorking || pendingTransition !== null || !!closeRequest}
          onBusyChange={(value) => { siviModalBusy = value; }}
          oncreated={async (res) => {
            newSiviModalOpen = false;
            await loadPlots(offset);
            editorPlotNumber = res.plotNumber;
            editorEntryForm = 'sivi';
            view = 'fs882';
          }}
          onundone={async () => { await loadPlots(offset); }}
          onclose={() => { newSiviModalOpen = false; }}
        />
      {/await}
    {/if}
  </div>

  {#if resetSampleConfirmOpen && !startupFailure}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" data-reset-sample-modal>
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="reset-sample-modal-title"
        class="w-full max-w-md overflow-hidden rounded border border-slate-300 bg-white text-[#313132] shadow-2xl"
      >
        <header class="border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-4 py-3 text-white">
          <div class="font-mono text-[11px] uppercase tracking-wider text-[#FCBA19]">Safe Non-Destructive Recovery</div>
          <h2 id="reset-sample-modal-title" class="text-sm font-semibold text-white">Reset Workspace to Sample</h2>
        </header>
        <div class="space-y-3 p-4 text-xs leading-relaxed">
          <p>
            Materialize a fresh, pristine copy of embedded <code class="font-mono text-[#003366]">Sample.db</code> and switch the active session cleanly to
            <strong>Project = Sample</strong>, <strong>SU = Sample</strong>, and <strong>Hierarchy = Sample</strong>?
          </p>
          <p class="rounded border border-slate-200 bg-[#F2F2F2] p-2.5 text-slate-700">
            Existing user project <code class="font-mono">.db</code> files in your projects directory are <strong>never deleted or modified</strong>.
          </p>
          <div class="flex justify-end gap-2 pt-2">
            <button
              type="button"
              disabled={busy || transitionWorking}
              class="rounded border border-slate-300 bg-white px-3 py-1.5 text-xs font-medium text-[#313132] hover:bg-slate-50"
              onclick={() => { resetSampleConfirmOpen = false; }}
            >
              Cancel
            </button>
            <button
              type="button"
              data-confirm-reset-sample
              disabled={busy || editorBusy || companionPanelBusy || transitionWorking || !$projectState?.contextId}
              class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={() => void confirmResetWorkspaceToSample()}
            >
              {busy ? 'Resetting...' : 'Reset Workspace to Sample'}
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  {#if helpDrawerOpen && !startupFailure}
    {#await import('./HelpDrawer.svelte') then { default: HelpDrawer }}
      <HelpDrawer
        open={helpDrawerOpen}
        topicId={helpDrawerTopicId}
        initialTab={helpDrawerTab}
        onclose={() => { helpDrawerOpen = false; }}
        onstarttour={(tid) => {
          helpDrawerOpen = false;
          tourInitialId = tid ?? null;
          tourOpen = true;
        }}
      />
    {/await}
  {/if}

  {#if tourOpen && !startupFailure}
    {#await import('./OnboardingTour.svelte') then { default: OnboardingTour }}
      <OnboardingTour
        open={tourOpen}
        initialTourId={tourInitialId}
        activeProject={$projectState?.activeProject ?? 'Sample'}
        onensurecontextopen={() => { contextExpanded = true; }}
        onnavigateview={(nextView) => { navigate(nextView as typeof view); }}
        onresetsample={async () => { tourOpen = false; resetSampleConfirmOpen = true; }}
        onclose={() => { tourOpen = false; tourInitialId = null; }}
      />
    {/await}
  {/if}

  {#if shortcutsModalOpen && !startupFailure}
    {#await import('./KeyboardShortcutsModal.svelte') then { default: KeyboardShortcutsModal }}
      <KeyboardShortcutsModal
        open={shortcutsModalOpen}
        onclose={() => { shortcutsModalOpen = false; }}
      />
    {/await}
  {/if}

  {#if whatsNewModalOpen && !startupFailure}
    {#await import('./WhatsNewModal.svelte') then { default: WhatsNewModal }}
      <WhatsNewModal
        open={whatsNewModalOpen}
        initialStatus={whatsNewStatus}
        onstatuschange={(updated) => { whatsNewStatus = updated; }}
        onclose={() => { whatsNewModalOpen = false; }}
      />
    {/await}
  {/if}

  {#if aboutModalOpen && !startupFailure}
    {#await import('./AboutVProModal.svelte') then { default: AboutVProModal }}
      <AboutVProModal
        open={aboutModalOpen}
        onclose={() => { aboutModalOpen = false; }}
      />
    {/await}
  {/if}
</div>
