<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { ChevronLeft, ChevronRight, Database, FolderOpen, RefreshCw } from '@lucide/svelte';
  import { CloseService, ContextService, ProjectService, StartupService, type StartupState, type ContextSelection, type ProjectInfo, type HierarchyNode, type PlotPage, type PlotSummary, type ProjectPlotProfileFilterRequest, type PlotProfileSource, type PlotProfileSourceInfo } from '../bindings/github.com/boostao/vpro-wails';
  import { projectState } from './state';
  import FS882Form from './FS882Form.svelte';
  import Navigation from './Navigation.svelte';
  import CloseConfirm from './CloseConfirm.svelte';
  import { closeDisposition, type CloseDecision, type EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { validateProfileNavigation, profileNavigationPage, validatePlotProfileSource, validateProjectPlotProfileReview, type ValidatedProjectPlotProfileReview, type ProfileNavigation } from './projectPlotProfileReview';
  import { validateProfileSUReview, validateProfileSUCreated, type ReviewedProfileSU } from './profileSU';

  const pageSize = 25;
  let page = $state<PlotPage | null>(null);
  let hierarchyNodes = $state<HierarchyNode[]>([]);
  let selected = $state<PlotSummary | null>(null);
  let profileNavigation = $state<ProfileNavigation | null>(null);
  const profileIndex = $derived(profileNavigation?.plots.findIndex(plot => plot.plotNumber === editorPlotNumber) ?? -1);
  let editorPlotNumber = $state<string | undefined>(undefined);
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
    plotReads.cancelAll(); hierarchyReads.cancelAll(); stateReads.cancelAll();
  });
  let view = $state<'home' | 'plots' | 'hierarchy' | 'fs882'>('home');
  let contextExpanded = $state(false);
  let editorBusy = $state(false);
  let editor: { getCloseState: () => EditorCloseState; saveForClose: () => Promise<boolean> } | undefined = $state();
  let closeRequest = $state('');
  let closeWorking = $state(false);
  let closeError = $state('');
  let pendingTransition = $state<{ contextId: string; run: (contextId: string) => Promise<void> } | null>(null);
  let transitionWorking = $state(false);
  let transitionError = $state('');
  let externalProject = $state('');
  let externalPath = $state('');
  const externalProjectsEnabled = import.meta.env.VITE_EXTERNAL_PROJECTS !== 'false';
  const profileSelectionEnabled = import.meta.env.VITE_PLOT_PROFILE_SELECTION === 'true';
  const profileWriteOwnershipEnabled = import.meta.env.VITE_PLOT_PROFILE_WRITE_OWNERSHIP === 'true';
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
  let startupFailure = $state<StartupState | null>(null);
  const closeState = $derived(view === 'fs882' ? editor?.getCloseState() : undefined);

  async function handleCloseRequest(requestId: string) {
    if (closeWorking || closeRequest === requestId) return;
    try {
      if (await CloseService.GetPendingCloseRequest() !== requestId) return;
      if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
      await tick();
      const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
      const disposition = closeDisposition(state, busy || transitionWorking || pendingTransition !== null || (view === 'fs882' && !editor));
      if (disposition === 'busy') {
        await CloseService.CancelClose(requestId);
        error = 'Wait for the current operation to finish before closing VPRO.';
      } else if (disposition === 'clean') {
        await CloseService.ConfirmClose(requestId);
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
        await CloseService.CancelClose(requestId);
        closeRequest = '';
        return;
      }
      const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
      if (closeDisposition(state, busy || (view === 'fs882' && !editor)) === 'busy') {
        closeError = 'Wait for the current operation to finish before closing VPRO.';
        return;
      }
      if (decision === 'save') {
        if (!editor || !await editor.saveForClose()) {
          closeError = editor?.getCloseState().error ?? 'The draft could not be saved. VPRO remains open.';
          return;
        }
      }
      await CloseService.ConfirmClose(requestId);
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

  async function requestTransition(run: (contextId: string) => Promise<void>) {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    await tick();
    const state = view === 'fs882' ? editor?.getCloseState() ?? null : null;
    if (closeDisposition(state, busy || editorBusy || transitionWorking || pendingTransition !== null || !!closeRequest || (view === 'fs882' && !editor)) === 'busy') {
      error = 'Wait for the current operation or decision before changing context.';
      return;
    }
    const contextId = $projectState?.contextId;
    if (!contextId) { error = 'A project context must finish loading before navigation.'; return; }
    transitionError = '';
    if (state?.unsaved) {
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
      if (!state || closeDisposition(state, busy) === 'busy') {
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
    if (next === view) return;
    void requestTransition(async () => {
      error = '';
      if (next === 'fs882') editorPlotNumber = selected?.plotNumber;
      view = next;
    });
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
        const next = await plotReads.track(ProjectService.ListPlots(0, pageSize));
        const state = await stateReads.track(ProjectService.GetState());
        if (state.contextId !== contextId || $projectState?.contextId !== contextId) throw new Error('Context changed before clearing navigation.');
        request++; plotReads.cancelAll();
        profileNavigation = null; page = next; offset = 0; selected = null;
        editorPlotNumber = undefined; view = 'plots'; error = '';
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
    const current = ++request;
    plotReads.cancelAll();
    busy = true;
    error = '';
    try {
      const source = profileNavigation;
      const next = source ? await resolveNavigation(source.proposal, source.contextId) : null;
      const result = next ? profileNavigationPage(next, next.contextId, nextOffset, pageSize) :
        await plotReads.track(ProjectService.ListPlots(nextOffset, pageSize));
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
    const current = ++stateRequest;
    stateReads.cancelAll();
    busy = true;
    error = '';
    try {
      const startup = await stateReads.track(StartupService.GetState());
      if (current !== stateRequest) return;
      if (!startup.ready) {
        startupFailure = startup;
        busy = false;
        return;
      }
      const state = await stateReads.track(ProjectService.GetState());
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
    const current = ++hierarchyRequest;
    hierarchyReads.cancelAll();
    try {
      const nodes = await hierarchyReads.track(ProjectService.GetHierarchyNodes());
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
    if (!contextId || busy || editorBusy || transitionWorking || pendingTransition !== null || closeRequest) {
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
    if (busy || editorBusy || transitionWorking || pendingTransition !== null || closeRequest ||
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
    if (!contextId || busy || editorBusy || transitionWorking || pendingTransition !== null || closeRequest) return;
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

  onMount(() => {
    const stopCloseEvents = Events.On('vpro:close-request', (event) => {
      const requestId: unknown = event.data;
      if (typeof requestId !== 'string' || !requestId) {
        error = 'Invalid native window close request; VPRO remains open.';
        return;
      }
      void handleCloseRequest(requestId);
    });
    void CloseService.GetPendingCloseRequest().then((requestId) => {
      if (requestId) void handleCloseRequest(requestId);
    }).catch((cause) => { error = `Window close handler could not initialise: ${String(cause)}`; });
    void refresh();
    return () => { request++; stopCloseEvents(); };
  });
</script>

<div class="app-shell">
  <header class="topbar">
    <div class="brand"><span class="brand-symbol" aria-hidden="true">V</span><span>VPRO</span></div>
    <button class="context-toggle" type="button" disabled={startupFailure !== null} aria-expanded={contextExpanded} aria-controls="project-context" onclick={() => contextExpanded = !contextExpanded}>Projects &amp; context</button>
    {#if !startupFailure}<Navigation {view} onnavigate={navigate} />{/if}
    <div class="topbar-context"><Database size={16} strokeWidth={1.8} /><span>{$projectState?.activeProject ?? 'No project'}</span></div>
  </header>

  <div class="workspace" class:context-collapsed={!contextExpanded || startupFailure !== null}>
    <aside id="project-context" class="sidebar" aria-label="Projects">
      <div class="sidebar-heading"><span>PROJECTS</span><FolderOpen size={17} strokeWidth={1.8} /></div>
      <nav aria-label="Project list">
        {#each $projectState?.projects ?? [] as project ((project.path ?? project.file) + project.name)}
          <button class:active={project.name === $projectState?.activeProject && project.path === $projectState?.projectPath} disabled={!project.compatible || busy || transitionWorking} title={project.compatible ? project.path : `${project.version} is not a complete VP08 project`} onclick={() => chooseProject(project)}>
            <span class="project-name">{project.name}
              <span class="project-location">{$projectState?.projects?.filter(item => item.name === project.name).length === 1 ? project.file : project.path}</span>
            </span>
            <span class="project-version">{project.version}</span>
          </button>
        {/each}
      </nav>
      <div class="su-picker">
        <label for="active-su">SITE UNIT</label>
        <select id="active-su" value={String(activeSUIndex)} onchange={chooseSU} disabled={busy || transitionWorking || !$projectState}>
          <option value="-1">None</option>
          {#each $projectState?.sus ?? [] as su, index ((su.path ?? '') + su.name)}
            <option value={String(index)} disabled={!su.compatible} title={su.path}>{su.name}{su.kind === 'master' ? ' (master)' : ''}</option>
          {/each}
        </select>
      </div>
      <div class="su-picker">
        <label for="active-hierarchy">HIERARCHY</label>
        <select id="active-hierarchy" value={String(activeHierarchyIndex)} onchange={chooseHierarchy} disabled={busy || transitionWorking || !$projectState}>
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
            <label for="external-project">PROJECT PREFIX</label>
            <input id="external-project" bind:value={externalProject} required disabled={busy || transitionWorking} />
            <label for="external-path">FULL SQLITE FILE PATH</label>
            <input id="external-path" bind:value={externalPath} required disabled={busy || transitionWorking} />
            <p>Attach does not copy or alter the file. Verified editors write to the attached file. Use disposable copies only.</p>
            <button type="submit" disabled={busy || transitionWorking}>Attach project</button>
          </form>
        </details>
      {/if}
      <div class="sidebar-footer">{$projectState?.projects?.length ?? 0} project{($projectState?.projects?.length ?? 0) === 1 ? '' : 's'}</div>
    </aside>

    <main class="content">
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
            {#if suReview.plots.length > 25}<p>Showing the first25 of {suReview.plots.length} explicitly reviewed rows.</p>{/if}
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50"
              disabled={busy || editorBusy || transitionWorking || pendingTransition !== null || suCommitted || !suName || !suPath || suReviewContext !== $projectState?.contextId}
              onclick={() => void saveProfileSU()}>Create reviewed SU file</button>
            <button type="button" class="px-3 py-2 border rounded disabled:opacity-50" disabled={busy || transitionWorking || pendingTransition !== null}
              onclick={() => { suReview = null; suError = ''; suSuccess = ''; }}>Dismiss SU review</button>
          </div>
          <p class="mt-3 text-sm">Only a new file is supported; existing destinations are never replaced. Stored inputs and SiteUnit values are rechecked after Save/Discard/Cancel. No implicit project/SU switch or original database/configuration write. Names use the existing31-character desktop family policy; None, Sample and master names are reserved.</p>
        </section>
      {/if}
      {#if profileSelectionEnabled && $projectState?.contextId}
        <details class="m-2 p-3 border rounded" aria-label="Stored plot profile selection">
          <summary class="font-semibold cursor-pointer">Plot profile: {$projectState.plotProfile.source.name}</summary>
          <p class="mt-2 text-sm">Select only disposable SQLite files during migration.</p>
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
        <section class="startup-recovery" role="alert">
          <h1>Saved context could not be opened</h1>
          <p>{startupFailure.message}</p>
          <p>Saved preferences and existing databases have not been reset or replaced. No editor is enabled.</p>
          <p>Correct the unavailable paths or malformed preferences in <code>{startupFailure.configPath}</code>, then restart VPRO.</p>
        </section>
      {:else if view === 'home'}
        <div class="content-heading"><div><div class="eyebrow">VPRO</div><h1>Home</h1></div></div>
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <div class="home-summary">
          <div><span class="eyebrow">CURRENT PROJECT</span><h2>{$projectState?.activeProject ?? 'No project selected'}</h2></div>
          <button class="open-plots" onclick={() => view = 'plots'}>View plots <ChevronRight size={16} /></button>
        </div>
        <p class="home-count">{page?.total ?? 0} plots in the current project. Select a project on the left to browse its plots.</p>
      {:else if view === 'fs882'}
        <div class="h-full flex flex-col p-2">
          <p class="project-warning"><strong>Experimental FS882 editor</strong> Only verified workflows are writable. Use disposable projects only.</p>
          {#if error}<p class="error" role="alert">{error}</p>{/if}
          <fieldset disabled={busy} class="contents">
          {#key $projectState?.contextId}
          {#key editorPlotNumber}
          <FS882Form
            bind:this={editor}
            plotNumber={editorPlotNumber}
            contextId={$projectState?.contextId ?? ''}
            onSaved={async (p) => { editorPlotNumber = p; await loadPlots(offset); }}
            onClosed={() => navigate('plots')}
            onBusyChange={(busy) => { editorBusy = busy; }}
            onProfileNavigation={applyProfileNavigation}
            onProfileSUReview={reviewProfileSU}
          />
          {/key}
          {/key}
          </fieldset>
        </div>

        <CloseConfirm requestId={closeRequest || (pendingTransition ? 'context' : '')}
          purpose={closeRequest ? 'close' : 'context'}
          working={closeWorking || transitionWorking} canSave={closeState?.canSave ?? false}
          saveReason={closeState?.saveReason ?? 'Return to the FS882 editor before saving.'}
          error={closeRequest ? closeError : transitionError}
          onrespond={closeRequest ? respondToClose : respondToTransition} />
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
            Open in FS882 Editor (Experimental)
          </button>
        </div>
      </aside>
    {/if}
  </div>
</div>
