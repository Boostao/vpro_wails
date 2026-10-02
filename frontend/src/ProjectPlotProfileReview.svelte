<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { bindContextPlots } from './contextPlots';
  import { ReadRequests } from './readRequests';
  import { profileCellLabel, validateProjectPlotProfileReview, validateProjectProfileLump, validateProfileRunResult,
    type ValidatedProjectPlotProfileReview, type ProfileReviewTable, type ProfileRunResult } from './projectPlotProfileReview';

  let { client, onclosed, onbusy, allowRun = false }: {
    client: ReturnType<typeof bindContextPlots>; onclosed: () => void; onbusy: (busy: boolean) => void;
    allowRun?: boolean;
  } = $props();
  let review = $state<ValidatedProjectPlotProfileReview | null>(null);
  let reading = $state(false);
  let running = $state(false);
  let error = $state<string | null>(null);
  let lump = $state<ProfileReviewTable | null>(null);
  let subvarieties = $state(false);
  let result = $state<ProfileRunResult | null>(null);
  let generation = 0;
  const reads = new ReadRequests();
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); });

  async function reload() {
    const request = ++generation;
    review = null; error = null; lump = null; result = null; reading = true; onbusy(true);
    try {
      const result = validateProjectPlotProfileReview(await reads.track(client.ReviewProjectPlotProfile()));
      if (request === generation) review = result;
    } catch (cause) {
      if (request === generation) error = `Project-local profile review failed; no rules, counts or filters changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; onbusy(false); }
    }
  }
  async function reviewLump() {
    if (!allowRun || reading || !review) return;
    const request = ++generation;
    error = null; lump = null; result = null; reading = true; onbusy(true);
    try {
      const source = validateProjectProfileLump(await reads.track(client.ReviewProjectPlotProfileLump()));
      if (request === generation) lump = source;
    } catch (cause) {
      if (request === generation) error = `Project-local lump review failed; no definitions changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; onbusy(false); }
    }
  }
  async function run() {
    if (!allowRun || reading || !review) return;
    const source = review, request = ++generation;
    result = null; error = null; reading = true; running = true; onbusy(true);
    try {
      const preview = await reads.track(client.RunProjectPlotProfile({
        originalRules: source.rules, projectLump: lump, subvarieties
      }));
      if (request === generation) result = validateProfileRunResult(preview, source);
    } catch (cause) {
      if (request === generation) error = `Profile preview failed; no stored rules, counts or filters changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; running = false; onbusy(false); }
    }
  }
  function cancelRun() {
    generation++; reads.cancelAll();
    running = false; reading = false; result = null; onbusy(false);
    error = 'Profile preview cancelled; reviewed inputs retained. No stored rules, counts or filters changed.';
  }
</script>

<section class="m-3 p-3 border border-stone-300 rounded-lg bg-stone-50" aria-label="Project-local plot profile review">
  <div class="flex flex-wrap gap-2 items-center justify-between">
    <h2 class="font-semibold text-stone-800">Project-local plot profile review (read-only)</h2>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={() => void reload()}>Reload profile review</button>
      {#if allowRun}
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading || !review} onclick={() => void run()}>Run stored profile preview</button>
        {#if running}<button type="button" class="px-3 py-2 border rounded bg-white" onclick={cancelRun}>Cancel profile preview</button>{/if}
      {:else}
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled title="Ordered profile execution is not implemented.">Run Profile (unavailable)</button>
      {/if}
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={onclosed}>Close profile review</button>
    </div>
  </div>
  {#if error}<p role="alert" class="mt-3 text-red-800">{error}</p>{/if}
  {#if reading}<p role="status" class="mt-3">Reading original inputs or evaluating stored profile rules...</p>{/if}
  {#if allowRun && review}
    <div class="flex flex-wrap gap-3 items-center mt-3">
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={() => void reviewLump()}>Review and select project lump table</button>
      {#if lump}
        <span>{review.project}_Lump explicitly selected: {lump.rows.length} physical definitions.</span>
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={reading} onclick={() => { lump = null; subvarieties = false; result = null; error = null; }}>Clear lump selection</button>
      {/if}
      <label class="flex gap-2 items-center"><input type="checkbox" disabled={reading || !lump} checked={subvarieties}
        onchange={event => { subvarieties = event.currentTarget.checked; result = null; error = null; }} />Combine source subvarieties</label>
    </div>
  {/if}
  {#if result}
    <section class="mt-3 p-3 border border-emerald-300 rounded bg-white" aria-label="Stored profile preview results">
      <h3 class="font-semibold">Preview only: {result.plotNumbers.length} matching plots / {result.totalPlots} stored scope plots</h3>
      <p>Scope: {result.project}; Working Unit list: {result.su}. No form filter or SU table has been changed.</p>
      <div class="overflow-x-auto">
        <table class="w-full text-xs border-collapse"><caption class="text-left py-2">Run-only counts, not stored PlotCount</caption>
          <thead><tr>{#each ['Physical rule', 'Order', 'Operation', 'Step count', 'Remaining'] as label}<th scope="col" class="border p-2 text-left">{label}</th>{/each}</tr></thead>
          <tbody>{#each result.steps as step (step.rowId)}<tr>
            <th scope="row" class="border p-2">{step.rowId}</th><td class="border p-2">{step.order}</td><td class="border p-2">{step.operation}</td>
            <td class="border p-2" aria-label={`Step count, rule ${step.rowId}`}>{step.plotCount}</td>
            <td class="border p-2" aria-label={`Remaining, rule ${step.rowId}`}>{step.remaining}</td>
          </tr>{/each}</tbody>
        </table>
      </div>
      <p class="mt-2 whitespace-pre-wrap break-words" data-profile-plots>{result.plotNumbers.join(', ') || 'No matching plots.'}</p>
    </section>
  {/if}
  {#if review}
    <p class="my-3">Source: {review.project} / {review.table}. Rules follow stored Order; duplicate Orders retain distinct physical records. PlotCount is historical storage, not a recalculated result.</p>
    {#each [{ title: 'Original profile rules', table: review.rules }, { title: 'Original table-object descriptions', table: review.descriptions },
      ...(allowRun && lump ? [{ title: 'Explicitly selected project-local lump definitions', table: lump }] : [])] as item}
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
  <p class="mt-3 text-sm text-stone-600">This uses only stored project-local rules and data, never unsaved plot drafts. Preview runs use isolated SQLite TEMP results and source-scoped Env/Veg/Lump operations; text matching supports ASCII literals and simple * / ? patterns only. Unsupported rules fail explicitly. Rule editing, other profile databases, applying filters and Save as SU remain unavailable. No stored PlotCount, Access registry preference or shipped scratch table is changed.</p>
</section>
