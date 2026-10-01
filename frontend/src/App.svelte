<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { Events } from '@wailsio/runtime';
  import { ChevronLeft, ChevronRight, Database, FolderOpen, RefreshCw } from '@lucide/svelte';
  import { CloseService, ContextService, ProjectService, StartupService, type StartupState, type ContextSelection, type ProjectInfo, type HierarchyNode, type PlotPage, type PlotSummary } from '../bindings/github.com/boostao/vpro-wails';
  import { projectState } from './state';
  import FS882Form from './FS882Form.svelte';
  import Navigation from './Navigation.svelte';
  import CloseConfirm from './CloseConfirm.svelte';
  import { closeDisposition, type CloseDecision, type EditorCloseState } from './closeLifecycle';

  const pageSize = 25;
  let page = $state<PlotPage | null>(null);
  let hierarchyNodes = $state<HierarchyNode[]>([]);
  let selected = $state<PlotSummary | null>(null);
  let editorPlotNumber = $state<string | undefined>(undefined);
  let offset = $state(0);
  let busy = $state(false);
  let error = $state('');
  let request = 0;
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

  async function loadPlots(nextOffset: number) {
    const current = ++request;
    busy = true;
    error = '';
    try {
      const result = await ProjectService.ListPlots(nextOffset, pageSize);
      if (current !== request) return;
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
    busy = true;
    error = '';
    try {
      const startup = await StartupService.GetState();
      if (!startup.ready) {
        startupFailure = startup;
        busy = false;
        return;
      }
      projectState.set(await ProjectService.GetState());
      await loadPlots(0);
      hierarchyNodes = await ProjectService.GetHierarchyNodes() ?? [];
    } catch (cause) {
      error = String(cause);
      busy = false;
    }
  }

  function selectedContext(): ContextSelection {
    const state = $projectState;
    if (!state?.projectPath) throw new Error('The active project file identity is unavailable.');
    return { project: state.activeProject, projectPath: state.projectPath,
      su: state.activeSU, suPath: state.suPath ?? '',
      hierarchy: state.activeHierarchy, hierarchyPath: state.hierarchyPath ?? '' };
  }

  async function switchContext(contextId: string, selection: ContextSelection, next: 'plots' | 'hierarchy') {
    busy = true;
    error = '';
    try {
      const state = await ContextService.SwitchContext(contextId, selection);
      request++;
      projectState.set(state);
      page = null;
      selected = null;
      hierarchyNodes = [];
      editorPlotNumber = undefined;
      view = next;
      await loadPlots(0);
      if (next === 'hierarchy') {
        try { hierarchyNodes = await ProjectService.GetHierarchyNodes() ?? []; }
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
          {#key $projectState?.contextId}
          <FS882Form
            bind:this={editor}
            plotNumber={editorPlotNumber}
            contextId={$projectState?.contextId ?? ''}
            onSaved={async (p) => { editorPlotNumber = p; await loadPlots(offset); }}
            onClosed={() => navigate('plots')}
            onBusyChange={(busy) => { editorBusy = busy; }}
          />
          {/key}
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
