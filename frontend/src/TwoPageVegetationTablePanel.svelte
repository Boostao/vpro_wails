<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageVegetationTableService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { twoPageVegetationTableFromWire, twoPageVegetationTableCellLabel,
    type TwoPageVegetationTableOwner, type TwoPageVegetationTableView } from './twoPageVegetationTable';

  let { contextId, project, projectPath, su, suPath, plot, form, onBusyChange }: TwoPageVegetationTableOwner & {
    onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, projectPath, su, suPath, plot, form }));
  let review = $state.raw<TwoPageVegetationTableView | null>(null);
  let busy = $state(false);
  let error = $state('');
  let generation = 0;
  const reads = new ReadRequests();
  const pageSize = 50;

  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function load(offset = 0) {
    if (busy) return;
    cancel(); const request = generation;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(TwoPageVegetationTableService.GetReview(owner.contextId, owner.plot, owner.form, offset, pageSize));
      if (request !== generation) return;
      const validated = twoPageVegetationTableFromWire(value, owner, offset, pageSize);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Vegetation table unavailable; data, history and query objects unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="View Veg Table read-only saved data"
  data-two-page-vegetation-table-panel data-source-parent-form={owner.form}>
  <h3 class="font-semibold">View Veg Table: saved-data review</h3>
  <p class="break-words">Owned project {owner.project}; opening plot {JSON.stringify(owner.plot)} is not a collection filter.
    {owner.su === 'None' ? 'All physical project vegetation rows.' : `Selected SU ${owner.su}; duplicate membership expands rows.`}</p>
  <p class="break-all text-xs">Project: {owner.projectPath}{#if owner.su !== 'None'}; SU: {owner.suPath}{/if}</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <p class="text-sm">Read-only adaptation: controlled English-ignore-case order/membership, numeric covers descending,
    NULL covers last and physical-row ties. General ACE locale parity is unverified.
    Each physical input is limited to100000 rows; pages contain up to50 joined rows, without truncating the total.</p>
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={() => load()}>Load saved vegetation table</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel vegetation table read</button><p role="status">Reading owned vegetation page...</p>{/if}
    {#if review}
      <button type="button" disabled={busy || review.offset === 0} onclick={() => load(Math.max(0, review!.offset - pageSize))}>Previous vegetation page</button>
      <button type="button" disabled={busy || review.offset + review.rows.length >= review.totalRows}
        onclick={() => load(review!.offset + pageSize)}>Next vegetation page</button>
    {/if}
  </div>
  {#if review}
    <p role="status">{review.totalRows} joined rows; {review.rows.length ? `showing ${review.offset + 1} through ${review.offset + review.rows.length}` : 'empty page'}.</p>
    <div class="max-w-full overflow-x-auto" role="region" aria-label="Saved vegetation projected columns">
      <table class="w-full border-collapse text-sm">
        <thead><tr>{#each review.columns as column}<th scope="col" class="border p-2 text-left">{column.caption}</th>{/each}</tr></thead>
        <tbody>
          {#each review.rows as row, index (`${row.vegRowId}:${row.suRowId}`)}
            <tr aria-label={`Joined row ${review.offset + index + 1}; Veg physical row ${row.vegRowId}; SU physical row ${row.suRowId ?? 'none'}`}
              data-vegetation-table-row={index}>
              {#each row.cells as cell}<td class="whitespace-pre-wrap border p-2">{twoPageVegetationTableCellLabel(cell)}</td>{/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <details>
      <summary>Complete physical originals for this page</summary>
      {#each [{ name: 'Veg', table: review.original }, { name: 'SU', table: review.selectedOriginal }] as source}
        {#if source.table}
          {#each source.table.rows as row (row.rowId)}
            <article class="min-w-0 rounded border p-2" aria-label={`${source.name} physical row ${row.rowId}`}>
              <h4>{source.name} physical row {row.rowId}</h4>
              <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                {#each source.table.columns as column, index}
                  <div class="min-w-0"><dt class="font-medium">{column.name} ({row.cells[index].storage})</dt>
                    <dd class="whitespace-pre-wrap break-words">{twoPageVegetationTableCellLabel(row.cells[index])}</dd></div>
                {/each}
              </dl>
            </article>
          {/each}
        {/if}
      {/each}
    </details>
  {/if}
  <p class="text-xs text-stone-600">Explicit read of saved data, never an implicit parent Save.
    NULL/empty text, historical application IDs and complete typed originals remain distinct.
    Editable datasheets, TempVegView creation, query widths and export are unavailable.
    Each page is a new owned snapshot; concurrent saved-data changes can affect later pages.</p>
</section>
