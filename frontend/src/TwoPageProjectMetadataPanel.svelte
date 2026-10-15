<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageProjectMetadataService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { twoPageProjectMetadataFromWire, twoPageProjectMetadataCellLabel, twoPageProjectMetadataGroups,
    type TwoPageProjectMetadataOwner, type TwoPageProjectMetadataView } from './twoPageProjectMetadata';

  let { contextId, project, projectPath, su, suPath, plot, form, onBusyChange }: TwoPageProjectMetadataOwner & {
    form: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, projectPath, su, suPath, plot }));
  const sourceForm = untrack(() => form);
  let review = $state.raw<TwoPageProjectMetadataView | null>(null);
  let busy = $state(false);
  let error = $state('');
  let disclosed = $state('');
  let generation = 0;
  const reads = new ReadRequests();
  const pageSize = 50;
  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function load(offset = 0) {
    if (busy) return;
    cancel(); const request = generation;
    review = null; error = ''; disclosed = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(TwoPageProjectMetadataService.GetReview(owner.contextId, owner.plot, offset, pageSize));
      if (request !== generation) return;
      const validated = twoPageProjectMetadataFromWire(value, owner, offset, pageSize);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Project metadata unavailable; data, history, preferences and query objects unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="Project metadata read-only saved data"
  data-two-page-project-metadata-panel data-source-parent-form={sourceForm}>
  <h3 class="font-semibold">Project Metadata: saved-data review</h3>
  <p class="break-words">Owned project {owner.project}; opening plot {JSON.stringify(owner.plot)} and SU {owner.su}.
    The saved parent ProjectID selects existing current-project metadata, not global templates.</p>
  <p class="break-all text-xs">Project: {owner.projectPath}{#if owner.su !== 'None'}; SU: {owner.suPath}{/if}</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <p class="text-sm">Read-only adaptation: controlled ASCII letters, digits and spaces; comparison ignores case and
    terminal ASCII spaces without modifying original text. General ACE locale parity is unverified.
    At most100000 physical metadata rows; pages contain up to50 matching rows in physical-row order.</p>
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={() => load()}>Load saved project metadata</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel project metadata read</button><p role="status">Reading owned metadata page...</p>{/if}
    {#if review}
      <button type="button" disabled={busy || review.offset === 0} onclick={() => load(Math.max(0, review!.offset - pageSize))}>Previous metadata page</button>
      <button type="button" disabled={busy || review.offset + review.metadataOriginal.rows.length >= review.totalRows}
        onclick={() => load(review!.offset + pageSize)}>Next metadata page</button>
    {/if}
  </div>
  {#if review}
    <p role="status">Saved ProjectID {JSON.stringify(review.projectId)}; {review.totalRows} matching physical rows;
      {review.metadataOriginal.rows.length ? `showing ${review.offset + 1} through ${review.offset + review.metadataOriginal.rows.length}` : 'empty page'}.</p>
    {#each review.metadataOriginal.rows as row, index (row.rowId)}
      <details class="min-w-0 rounded border p-2" open={disclosed === row.rowId}
        ontoggle={event => { if (event.currentTarget.open) disclosed = row.rowId; else if (disclosed === row.rowId) disclosed = ''; }}
        data-project-metadata-row={index}>
        <summary>Metadata row {review.offset + index + 1}; physical row {row.rowId}</summary>
        {#if disclosed === row.rowId}
          {#each twoPageProjectMetadataGroups(review, row) as group (group.title)}
            <section class="min-w-0 space-y-2" aria-label={group.title}>
              <h4 class="font-semibold">{group.title}</h4>
              <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                {#each group.fields as field (field.controlId)}
                  <div class="min-w-0" data-source-binding-control={field.controlId}>
                    <dt class="font-medium">{field.label}</dt>
                    <dd class="whitespace-pre-wrap break-words">{field.cell ? twoPageProjectMetadataCellLabel(field.cell) : `Unavailable: missing or ambiguous physical ${field.name}`}</dd>
                  </div>
                {/each}
              </dl>
            </section>
          {/each}
        {/if}
      </details>
    {/each}
    <details>
      <summary>Complete physical originals for this page</summary>
      {#each [{ name: 'Env', table: review.envOriginal }, { name: 'Admin', table: review.adminOriginal }, { name: 'Metadata', table: review.metadataOriginal }] as source}
        {#each source.table.rows as row (row.rowId)}
          <article class="min-w-0 rounded border p-2" aria-label={`${source.name} physical row ${row.rowId}`}>
            <h4>{source.name} physical row {row.rowId}</h4>
            <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
              {#each source.table.columns as column, index}
                <div class="min-w-0"><dt class="font-medium">{column.name} ({row.cells[index].storage})</dt>
                  <dd class="whitespace-pre-wrap break-words">{twoPageProjectMetadataCellLabel(row.cells[index])}</dd></div>
              {/each}
            </dl>
          </article>
        {/each}
      {/each}
    </details>
  {/if}
  <p class="text-xs text-stone-600">Explicit saved-data review, never metadata Save, Add/copy, template lookup, attachment
    or source callback execution. NULL/empty text, historical IDs and all physical originals stay distinct.
    Source-bound labels retain74 fields; missing historical columns remain explicitly unavailable.
    Each page reads a fresh owned snapshot; concurrent changes can affect later pages.</p>
</section>
