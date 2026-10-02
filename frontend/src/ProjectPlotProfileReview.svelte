<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { bindContextPlots } from './contextPlots';
  import { ReadRequests } from './readRequests';
  import { profileCellLabel, validateProjectPlotProfileReview, type ValidatedProjectPlotProfileReview } from './projectPlotProfileReview';

  let { client, onclosed, onbusy }: {
    client: ReturnType<typeof bindContextPlots>; onclosed: () => void; onbusy: (busy: boolean) => void;
  } = $props();
  let review = $state<ValidatedProjectPlotProfileReview | null>(null);
  let reading = $state(false);
  let error = $state<string | null>(null);
  let generation = 0;
  const reads = new ReadRequests();
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); });

  async function reload() {
    const request = ++generation;
    review = null; error = null; reading = true; onbusy(true);
    try {
      const result = validateProjectPlotProfileReview(await reads.track(client.ReviewProjectPlotProfile()));
      if (request === generation) review = result;
    } catch (cause) {
      if (request === generation) error = `Project-local profile review failed; no rules, counts or filters changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; onbusy(false); }
    }
  }
</script>

<section class="m-3 p-3 border border-stone-300 rounded-lg bg-stone-50" aria-label="Project-local plot profile review">
  <div class="flex flex-wrap gap-2 items-center justify-between">
    <h2 class="font-semibold text-stone-800">Project-local plot profile review (read-only)</h2>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={() => void reload()}>Reload profile review</button>
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled title="Ordered profile execution is not implemented.">Run Profile (unavailable)</button>
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={onclosed}>Close profile review</button>
    </div>
  </div>
  {#if error}<p role="alert" class="mt-3 text-red-800">{error}</p>{/if}
  {#if reading}<p role="status" class="mt-3">Reading original project profile rules and table-object descriptions...</p>{/if}
  {#if review}
    <p class="my-3">Source: {review.project} / {review.table}. Rules follow stored Order; duplicate Orders retain distinct physical records. PlotCount is historical storage, not a recalculated result.</p>
    {#each [{ title: 'Original profile rules', table: review.rules }, { title: 'Original table-object descriptions', table: review.descriptions }] as item}
      <div class="overflow-x-auto mt-3">
        <table class="w-full border-collapse text-xs">
          <caption class="text-left font-semibold py-2">{item.title}</caption>
          <thead><tr><th scope="col" class="border p-2 text-left">Physical row</th>
            {#each item.table.columns as column}<th scope="col" class="border p-2 text-left">{column.name}<span class="block font-normal text-stone-500">{column.declaredType}</span></th>{/each}
          </tr></thead>
          <tbody>{#each item.table.rows as row (row.rowId)}
            <tr><th scope="row" class="border p-2 text-left">{row.rowId}</th>
              {#each row.cells as cell, index}<td class="border p-2 whitespace-pre-wrap min-w-24" aria-label={`${item.table.columns[index].name}, record ${row.rowId}`}>{profileCellLabel(cell)}</td>{/each}
            </tr>
          {/each}</tbody>
        </table>
        {#if item.table.rows.length === 0}<p class="py-2">No physical records in this source table.</p>{/if}
      </div>
    {/each}
  {/if}
  <p class="mt-3 text-sm text-stone-600">This reviews only the selected project's original profile table. Selecting another profile database, editing rules, ordered Env/Veg/Lump execution, scratch results, filtering and Save as SU remain unavailable. No Access registry preference or shipped scratch table is changed.</p>
</section>
