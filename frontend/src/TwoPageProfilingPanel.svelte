<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageProfilingService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { twoPageProfilingFromWire, twoPageProfilingCellLabel, twoPageProfilingRowFields,
    type TwoPageProfilingOwner, type TwoPageProfilingView } from './twoPageProfiling';

  let { contextId, project, projectPath, su, suPath, plot, profileSource, form, onBusyChange }: TwoPageProfilingOwner & {
    form: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, projectPath, su, suPath, plot, profileSource: { ...profileSource } }));
  const sourceForm = untrack(() => form);
  let review = $state.raw<TwoPageProfilingView | null>(null);
  let busy = $state(false);
  let error = $state('');
  let disclosed = $state('');
  let originalsOpen = $state(false);
  let descriptionOffset = $state(0);
  let generation = 0;
  const reads = new ReadRequests();
  const pageSize = 50;
  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function load(offset = 0) {
    if (busy) return;
    cancel(); const request = generation;
    review = null; error = ''; disclosed = ''; originalsOpen = false; descriptionOffset = 0; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(TwoPageProfilingService.GetReview(owner.contextId, owner.plot, offset, pageSize));
      if (request !== generation) return;
      const validated = await twoPageProfilingFromWire(value, owner, offset, pageSize, async () => {
        await new Promise<void>(resolve => setTimeout(resolve, 0));
        if (request !== generation) throw new Error('Cancelled owned profiling validation.');
      });
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Plot Profiling unavailable; data, history, preferences and query objects unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="Plot Profiling read-only saved data"
  data-two-page-profiling-panel data-source-parent-form={sourceForm}>
  <h3 class="font-semibold">Plot Profiling: saved-data review</h3>
  <p class="break-words">Owned project {owner.project}; opening plot {JSON.stringify(owner.plot)} and SU {owner.su}.
    Opening plot establishes ownership, not a profile-rule filter.</p>
  <p class="break-all text-xs">Selected profile {JSON.stringify(owner.profileSource.name)}: {owner.profileSource.path};
    project: {owner.projectPath}{#if owner.su !== 'None'}; SU: {owner.suPath}{/if}</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={() => load()}>Load saved profile</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel profile read</button><p role="status">Reading owned profile page...</p>{/if}
    {#if review}
      <button type="button" disabled={busy || review.offset === 0} onclick={() => load(Math.max(0, review!.offset - pageSize))}>Previous profile page</button>
      <button type="button" disabled={busy || review.offset + review.profileOriginal.rows.length >= review.totalRows}
        onclick={() => load(review!.offset + pageSize)}>Next profile page</button>
    {/if}
  </div>
  {#if review}
    <p role="status">{review.totalRows} saved physical rules;
      {review.profileOriginal.rows.length ? `showing ${review.offset + 1} through ${review.offset + review.profileOriginal.rows.length}` : 'empty page'}.
      {review.descriptionsPresent ? `${review.descriptionOriginal.rows.length} original Description definitions` : 'Description metadata absent'}.</p>
    {#each review.profileOriginal.rows as row, index (row.rowId)}
      <details class="min-w-0 rounded border p-2" open={disclosed === row.rowId}
        ontoggle={event => { if (event.currentTarget.open) disclosed = row.rowId; else if (disclosed === row.rowId) disclosed = ''; }}
        data-profile-row={index}>
        <summary>Profile rule {review.offset + index + 1}; physical row {row.rowId}</summary>
        {#if disclosed === row.rowId}
          <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {#each twoPageProfilingRowFields(review, row) as field (field.controlId)}
              <div class="min-w-0" data-source-binding-control={field.controlId}>
                <dt class="font-medium">{field.label}</dt>
                <dd class="whitespace-pre-wrap break-words">{twoPageProfilingCellLabel(field.cell)}</dd>
              </div>
            {/each}
          </dl>
        {/if}
      </details>
    {/each}
    <details open={originalsOpen} ontoggle={event => originalsOpen = event.currentTarget.open}>
      <summary>Complete physical originals for this page</summary>
      {#if originalsOpen}
      {#each [{ name: 'Env', table: review.envOriginal }, { name: 'Admin', table: review.adminOriginal },
        { name: 'Profile', table: review.profileOriginal }, { name: 'Description', table: review.descriptionOriginal }] as source}
        {#if source.name === 'Description'}
          <p role="status">Description originals: {source.table.rows.length} definitions;
            {source.table.rows.length ? `showing ${descriptionOffset + 1} through ${Math.min(descriptionOffset + pageSize, source.table.rows.length)}` : 'empty'}.</p>
          <div class="flex flex-wrap gap-2">
            <button type="button" disabled={descriptionOffset === 0}
              onclick={() => descriptionOffset = Math.max(0, descriptionOffset - pageSize)}>Previous Description originals</button>
            <button type="button" disabled={descriptionOffset + pageSize >= source.table.rows.length}
              onclick={() => descriptionOffset += pageSize}>Next Description originals</button>
          </div>
        {/if}
        {#each (source.name === 'Description' ? source.table.rows.slice(descriptionOffset, descriptionOffset + pageSize) : source.table.rows) as row (row.rowId)}
          <article class="min-w-0 rounded border p-2" aria-label={`${source.name} physical row ${row.rowId}`}>
            <h4>{source.name} physical row {row.rowId}</h4>
            <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
              {#each source.table.columns as column, index}
                <div class="min-w-0"><dt class="font-medium">{column.name} ({row.cells[index].storage})</dt>
                  <dd class="whitespace-pre-wrap break-words">{twoPageProfilingCellLabel(row.cells[index])}</dd></div>
              {/each}
            </dl>
          </article>
        {/each}
      {/each}
      {/if}
    </details>
  {/if}
  <p class="text-xs text-stone-600">Read-only saved inputs: no scratch reset, physical count writes, profile selection,
    editing, Run, species-only, form filters, SU creation, attachment or Help launch.
    Stored Plot Count is historical data, not a newly computed result; project/result totals are unavailable.
    NULL/empty, historical duplicate Order and extra originals are retained.
    At most100000 rows per physical source;50-rule pages use physical row order, not source Order execution.
    Description originals are disclosed50 at a time from that same immutable snapshot, without another source read.
    Each profile page reads a fresh owned snapshot; concurrent changes can affect later pages.</p>
</section>
