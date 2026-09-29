<script lang="ts">
  import { onMount } from 'svelte';
  import { ChevronDown, ChevronLeft, ChevronRight, Database, FolderOpen, RefreshCw } from '@lucide/svelte';
  import { ProjectService, type HierarchyNode, type PlotPage, type PlotSummary } from '../bindings/github.com/boostao/vpro-wails';
  import { projectState } from './state';

  const pageSize = 25;
  let page = $state<PlotPage | null>(null);
  let hierarchyNodes = $state<HierarchyNode[]>([]);
  let selected = $state<PlotSummary | null>(null);
  let offset = $state(0);
  let busy = $state(false);
  let error = $state('');
  let request = 0;
  let view = $state<'home' | 'plots' | 'hierarchy'>('home');
  let activeHierarchyIndex = $derived($projectState?.hierarchies?.findIndex(
    (hierarchy) => hierarchy.name === $projectState?.activeHierarchy && hierarchy.file === $projectState?.hierarchyFile
  ) ?? -1);

  const menus = [
    { label: 'Forms', groups: [
      { label: 'Data Entry', items: ['FS882 Data Forms (6x4)', 'Enter/Edit SIVI Data'] },
      { label: 'Others', items: ['Metadata', 'Combine Species', 'Herbarium', 'Colour-theme', 'User setup', 'User log'] }
    ] },
    { label: 'Reports', groups: [
      { label: 'Vegetation', items: ['Long Vegetation', 'Summary Vegetation'] },
      { label: 'Environment', items: ['Long Environment', 'Summary Environment'] },
      { label: 'Others', items: ['Subzone Matrix of Units', 'Hierarchy Diagram', 'Print a Plot Label', 'Create Plot Locations File', 'Show Plot Locations in Google Earth'] }
    ] },
    { label: 'Help', groups: [{ label: 'Help', items: ["What's New"] }] }
  ];

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
      projectState.set(await ProjectService.GetState());
      await loadPlots(0);
      hierarchyNodes = await ProjectService.GetHierarchyNodes() ?? [];
    } catch (cause) {
      error = String(cause);
      busy = false;
    }
  }

  async function chooseProject(name: string) {
    busy = true;
    error = '';
    try {
      projectState.set(await ProjectService.SelectProject(name));
      await loadPlots(0);
      view = 'plots';
    } catch (cause) {
      error = String(cause);
      busy = false;
    }
  }

  async function chooseSU(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    busy = true;
    error = '';
    try {
      projectState.set(await ProjectService.SelectSU(select.value));
      await loadPlots(0);
    } catch (cause) {
      error = String(cause);
      select.value = $projectState?.activeSU ?? 'None';
      busy = false;
    }
  }

  async function chooseHierarchy(event: Event) {
    const select = event.currentTarget as HTMLSelectElement;
    const chosen = $projectState?.hierarchies?.[Number(select.value)];
    busy = true;
    error = '';
    try {
      projectState.set(await ProjectService.SelectHierarchy(chosen?.name ?? 'None', chosen?.file ?? ''));
      hierarchyNodes = await ProjectService.GetHierarchyNodes() ?? [];
      view = 'hierarchy';
    } catch (cause) {
      error = String(cause);
      select.value = String(activeHierarchyIndex);
    } finally {
      busy = false;
    }
  }

  onMount(() => {
    void refresh();
    return () => { request++; };
  });
</script>

<div class="app-shell">
  <header class="topbar">
    <div class="brand"><span class="brand-symbol" aria-hidden="true">V</span><span>VPRO</span></div>
    <nav class="main-nav" aria-label="Main navigation">
      <button class:current={view === 'home'} onclick={() => view = 'home'}>Home</button>
      <button class:current={view === 'plots'} onclick={() => view = 'plots'}>Plots</button>
      <button class:current={view === 'hierarchy'} onclick={() => view = 'hierarchy'}>Hierarchy</button>
      {#each menus as menu (menu.label)}
        <details class="nav-menu">
          <summary>{menu.label}<ChevronDown size={14} aria-hidden="true" /></summary>
          <div class="nav-menu-content">
            <div class="nav-unavailable">Not yet available in the desktop app</div>
            {#each menu.groups as group (group.label)}
              <div class="nav-group-label">{group.label}</div>
              {#each group.items as item (item)}
                <button disabled title="Not yet available in the desktop app">{item}</button>
              {/each}
            {/each}
          </div>
        </details>
      {/each}
    </nav>
    <div class="topbar-context"><Database size={16} strokeWidth={1.8} /><span>{$projectState?.activeProject ?? 'No project'}</span></div>
  </header>

  <div class="workspace">
    <aside class="sidebar" aria-label="Projects">
      <div class="sidebar-heading"><span>PROJECTS</span><FolderOpen size={17} strokeWidth={1.8} /></div>
      <nav aria-label="Project list">
        {#each $projectState?.projects ?? [] as project (project.name)}
          <button class:active={project.name === $projectState?.activeProject} disabled={!project.compatible || busy} title={project.compatible ? project.file : `${project.version} is not a complete VP08 project`} onclick={() => chooseProject(project.name)}>
            <span class="project-name">{project.name}</span>
            <span class="project-version">{project.version}</span>
          </button>
        {/each}
      </nav>
      <div class="su-picker">
        <label for="active-su">SITE UNIT</label>
        <select id="active-su" value={$projectState?.activeSU ?? 'None'} onchange={chooseSU} disabled={busy || !$projectState}>
          <option value="None">None</option>
          {#each $projectState?.sus ?? [] as su (su.name)}
            <option value={su.name} disabled={!su.compatible}>{su.name}{su.kind === 'master' ? ' (master)' : ''}</option>
          {/each}
        </select>
      </div>
      <div class="su-picker">
        <label for="active-hierarchy">HIERARCHY</label>
        <select id="active-hierarchy" value={String(activeHierarchyIndex)} onchange={chooseHierarchy} disabled={busy || !$projectState}>
          <option value="-1">None</option>
          {#each $projectState?.hierarchies ?? [] as hierarchy, index (hierarchy.file + hierarchy.name)}
            <option value={String(index)} disabled={!hierarchy.compatible}>{hierarchy.name} ({hierarchy.file})</option>
          {/each}
        </select>
      </div>
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
      {#if view === 'home'}
        <div class="content-heading"><div><div class="eyebrow">VPRO</div><h1>Home</h1></div></div>
        {#if error}<p class="error" role="alert">{error}</p>{/if}
        <div class="home-summary">
          <div><span class="eyebrow">CURRENT PROJECT</span><h2>{$projectState?.activeProject ?? 'No project selected'}</h2></div>
          <button class="open-plots" onclick={() => view = 'plots'}>View plots <ChevronRight size={16} /></button>
        </div>
        <p class="home-count">{page?.total ?? 0} plots in the current project. Select a project on the left to browse its plots.</p>
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
      </aside>
    {/if}
  </div>
</div>
