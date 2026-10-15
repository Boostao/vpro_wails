<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageEnvironmentTableService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { twoPageEnvironmentTableFromWire, twoPageEnvironmentCellLabel, twoPageEnvironmentGroups,
    type TwoPageEnvironmentOwner, type TwoPageEnvironmentView } from './twoPageEnvironmentTable';

  let { contextId, project, projectPath, su, suPath, plot, form, onBusyChange }: TwoPageEnvironmentOwner & {
    form: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, projectPath, su, suPath, plot }));
  const sourceForm = untrack(() => form);
  let review = $state.raw<TwoPageEnvironmentView | null>(null);
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
      const value = await reads.track(TwoPageEnvironmentTableService.GetReview(owner.contextId, owner.plot, offset, pageSize));
      if (request !== generation) return;
      const validated = twoPageEnvironmentTableFromWire(value, owner, offset, pageSize);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Environment table unavailable; data, history, preferences and query objects unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="View Env Table read-only saved data"
  data-two-page-environment-table-panel data-source-parent-form={sourceForm}>
  <h3 class="font-semibold">View Env Table: saved-data review</h3>
  <p class="break-words">Owned project {owner.project}; opening plot {JSON.stringify(owner.plot)} and SU {owner.su}
    establish ownership, not collection filters. All saved Env/Admin inner-join pairs are reviewed.</p>
  <p class="break-all text-xs">Project: {owner.projectPath}{#if owner.su !== 'None'}; SU: {owner.suPath}{/if}</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <p class="text-sm">Read-only adaptation: the saved Current.DataFormName selects the target, not the invoking form.
    Declared target order uses controlled English-ignore-case and physical-row ties; general ACE locale parity is unverified.
    No saved form filter is activated. Each physical input is limited to100000 rows; pages contain up to50 pairs.</p>
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={() => load()}>Load saved environment table</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel environment table read</button><p role="status">Reading owned environment page...</p>{/if}
    {#if review}
      <button type="button" disabled={busy || review.offset === 0} onclick={() => load(Math.max(0, review!.offset - pageSize))}>Previous environment page</button>
      <button type="button" disabled={busy || review.offset + review.rows.length >= review.totalRows}
        onclick={() => load(review!.offset + pageSize)}>Next environment page</button>
    {/if}
  </div>
  {#if review}
    <p role="status">Saved target {review.target.form}; {review.totalRows} physical pairs;
      {review.rows.length ? `showing ${review.offset + 1} through ${review.offset + review.rows.length}` : 'empty page'}.</p>
    <p class="text-sm">{review.target.form === 'FS882-6x4' ? 'RootingDepth descending, then SeepageDepth descending, then PlotNumber; NULL depths last.' : 'PlotNumber order.'}
      Source-bound groups retain form labels, not independently verified datasheet headers, visibility or column order.</p>
    {#each review.rows as row, index (`${row.envRowId}:${row.adminRowId}`)}
      {@const pair = `${row.envRowId}:${row.adminRowId}`}
      <details class="min-w-0 rounded border p-2" open={disclosed === pair}
        ontoggle={event => { if (event.currentTarget.open) disclosed = pair; else if (disclosed === pair) disclosed = ''; }}
        data-environment-table-row={index}>
        <summary>Joined row {review.offset + index + 1}; Env physical row {row.envRowId}; Admin physical row {row.adminRowId}</summary>
        {#if disclosed === pair}
          {#each twoPageEnvironmentGroups(review, row) as group (group.controlId)}
            <section class="min-w-0 space-y-2" aria-label={group.caption}>
              <h4 class="font-semibold">{group.caption}</h4>
              <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                {#each group.fields as field (field.controlId)}
                  <div class="min-w-0" data-source-binding-control={field.controlId}>
                    <dt class="font-medium">{field.label}</dt>
                    <dd class="whitespace-pre-wrap break-words">{field.cell ? twoPageEnvironmentCellLabel(field.cell) : `Unavailable: missing or ambiguous physical ${field.role}.${field.column}`}</dd>
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
      {#each [{ name: 'Env', table: review.envOriginal }, { name: 'Admin', table: review.adminOriginal }] as source}
        {#each source.table.rows as row (row.rowId)}
          <article class="min-w-0 rounded border p-2" aria-label={`${source.name} physical row ${row.rowId}`}>
            <h4>{source.name} physical row {row.rowId}</h4>
            <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
              {#each source.table.columns as column, index}
                <div class="min-w-0"><dt class="font-medium">{column.name} ({row.cells[index].storage})</dt>
                  <dd class="whitespace-pre-wrap break-words">{twoPageEnvironmentCellLabel(row.cells[index])}</dd></div>
              {/each}
            </dl>
          </article>
        {/each}
      {/each}
    </details>
  {/if}
  <p class="text-xs text-stone-600">Explicit saved-data read, never parent Save, editable datasheet or registry/toolbar mutation.
    NULL/empty text and all physical originals stay distinct. Unbound/computed controls and loaded-form runtime state are unavailable.
    Each page reads a fresh snapshot and saved target; concurrent changes can affect later pages.</p>
</section>
