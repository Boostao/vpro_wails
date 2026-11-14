<script lang="ts">
  import { ChevronLeft, ChevronRight, RefreshCw } from '@lucide/svelte';
  import type { HierarchyNode, PlotPage, PlotSummary } from '../../bindings/github.com/boostao/vpro-wails';
  import { projectState } from '../shared/state';

  let {
    region = 'main',
    view = 'plots',
    busy = false,
    error = '',
    page = null,
    offset = 0,
    pageSize = 25,
    hierarchyNodes = [],
    selected = $bindable(null),
    siviStandaloneEnabled = true,
    editorBusy = false,
    transitionWorking = false,
    pendingTransition = null,
    onrefresh = () => {},
    onloadpage = () => {},
    onopenfs882 = () => {},
    onopensivi = () => {},
  }: {
    region?: 'main' | 'detail';
    view?: string;
    busy?: boolean;
    error?: string;
    page?: PlotPage | null;
    offset?: number;
    pageSize?: number;
    hierarchyNodes?: HierarchyNode[];
    selected?: PlotSummary | null;
    siviStandaloneEnabled?: boolean;
    editorBusy?: boolean;
    transitionWorking?: boolean;
    pendingTransition?: unknown;
    onrefresh?: () => void;
    onloadpage?: (nextOffset: number) => void;
    onopenfs882?: () => void;
    onopensivi?: () => void;
  } = $props();
</script>

{#if region === 'detail'}
  {#if selected && view === 'plots'}
    <aside class="detail" aria-label="Selected plot">
      <div class="eyebrow">SELECTED PLOT</div><h2>{selected.plotNumber}</h2>
      <dl>
        <dt>Field number</dt><dd>{selected.fieldNumber ?? '—'}</dd>
        <dt>Representing</dt><dd>{selected.plotRepresenting ?? '—'}</dd>
        <dt>Zone</dt><dd>{selected.zone ?? '—'}</dd>
        <dt>Subzone</dt><dd>{selected.subZone ?? '—'}</dd>
        <dt>Site series</dt><dd>{selected.siteSeries ?? '—'}</dd>
      </dl>
      <div class="mt-4 pt-3 border-t border-stone-200">
        <button
          type="button"
          class="w-full py-1.5 px-3 bg-emerald-700 hover:bg-emerald-800 text-white rounded text-xs font-semibold"
          onclick={onopenfs882}
        >
          Open in FS882 Editor
        </button>
        {#if siviStandaloneEnabled}
          <button
            type="button"
            data-sivi-standalone-open
            class="mt-2 w-full py-1.5 px-3 border rounded text-xs font-semibold"
            disabled={busy || editorBusy || transitionWorking || pendingTransition !== null}
            onclick={onopensivi}
          >Open SIVI / FS1333</button>
        {/if}
      </div>
    </aside>
  {/if}
{:else if view === 'hierarchy'}
  <div class="content-heading">
    <div><div class="eyebrow">{$projectState?.hierarchyFile ?? 'HIERARCHY'}</div><h1>Hierarchy</h1></div>
    <button class="icon-button" title="Refresh hierarchy" aria-label="Refresh hierarchy" disabled={busy} onclick={onrefresh}><RefreshCw size={18} strokeWidth={1.8} /></button>
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
    <button class="icon-button" title="Refresh projects and plots" aria-label="Refresh projects and plots" disabled={busy} onclick={onrefresh}><RefreshCw size={18} strokeWidth={1.8} /></button>
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
      <button class="icon-button" title="Previous page" aria-label="Previous page" disabled={busy || offset === 0} onclick={() => onloadpage(Math.max(0, offset - pageSize))}><ChevronLeft size={18} /></button>
      <button class="icon-button" title="Next page" aria-label="Next page" disabled={busy || offset + pageSize >= (page?.total ?? 0)} onclick={() => onloadpage(offset + pageSize)}><ChevronRight size={18} /></button>
    </div>
  </div>
{/if}
