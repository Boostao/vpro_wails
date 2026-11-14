<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import type { SummaryEnvironmentalPort, SummaryEnvironmentalSession, SummaryEnvironmentalNumericSession, SummaryEnvironmentalLifeformSession } from './summaryVegetationEnvironmental';
  import type { FieldListCell } from './summaryVegetationFieldList';
  import type { SummaryEnvironmentalValue } from './summaryVegetationEnvironmentalNumeric';
  let { session, port, numeric = false, lifeform = false, operationHeld = false, onBusyChange }: {
    session: SummaryEnvironmentalSession | SummaryEnvironmentalNumericSession | SummaryEnvironmentalLifeformSession; port: SummaryEnvironmentalPort;
    numeric?: boolean;
    lifeform?: boolean;
    operationHeld?: boolean; onBusyChange: (busy: boolean) => void;
  } = $props();
  const retained = untrack(() => session);
  let view = $state.raw(retained.view());
  let error = $state('');
  const unsubscribe = retained.subscribe(() => {
    view = retained.view();
    onBusyChange(view.busy || view.settling);
  });
  async function preview() {
    if (operationHeld || view.busy || view.settling) return;
    error = '';
    try { await retained.show(port); } catch (cause) { error = String(cause); }
  }
  function cancel() { retained.cancel(); }
  function environmentalText(value: FieldListCell | SummaryEnvironmentalValue) {
    if ('kind' in value && value.kind === 'empty') return 'Empty (no numeric value)';
    const cell = 'kind' in value ? value.cell : value;
    return cell.storage === 'integer' ? cell.integer : cell.text === '' ? '""' : cell.text;
  }
  onDestroy(() => {
    if (view.busy) retained.cancel();
    unsubscribe();
    onBusyChange(false);
  });
</script>

<section class="environmental" data-summary-environment aria-label="Combined Summary environmental inspection">
  <h2>Environmental rows — current Site Unit Table</h2>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if view.error}<p role="alert">{view.error}</p>{/if}
  <div class="actions">
    <button data-summary-environment-preview disabled={operationHeld || view.busy || view.settling} onclick={preview}>Preview environmental rows</button>
    {#if view.busy}<button data-summary-environment-cancel onclick={cancel}>Cancel environmental preview</button>{/if}
  </div>
  {#if view.busy}<p role="status">Reading vegetation, selected fields and environmental values in one owned snapshot…</p>{/if}
  {#if view.settling && !view.busy}<p role="status">Cancelled dispatch or validation is still settling. Navigation, close and another Preview must wait.</p>{/if}
  <p>This read-only inspection uses Layer, By n Plots and % Cover to establish the unit frame.
    It does not consume the controls or saved Include Env preference above. Every selected physical
    Report=Yes field is read afresh, including hidden and NULL-order rows.
    Count and nineteen direct category summaries are available, along with Bedrock arguments 1/2/3
    and Disturbance arguments 1/2.
    {#if numeric}The numeric inspection also supports Seepage, Rooting Depth, Stand Age, Elevation,
      Slope and the six substrate/four stratum covers. Empty numeric results are explicitly labelled,
      not converted to zero or an empty string.
    {:else}Numeric functions remain unavailable in this categorical inspection.{/if}
    {#if lifeform}Lifeform cover arguments 1 through 13 use the raw physical vegetation and master
      species reference from the same owned snapshot. A lifeform String empty remains an empty string,
      not an Empty numeric result.{:else}Lifeform environmental rows remain unavailable in this inspection.{/if}
    Unsupported selections refuse the whole result. Grouped environmental reports, formatting,
    workbooks and printing remain unavailable.
    No data or settings are saved.</p>
  {#if view.result && !operationHeld}
    <article data-summary-environment-result>
      <p class="paths">Field-list source: {view.result.sourcePath}<br />Environmental source: {view.result.envSourcePath}</p>
      {#if lifeform && 'masterSourcePath' in view.result}<p class="paths">Lifeform master source: {view.result.masterSourcePath}</p>{/if}
      <div class="table-scroll">
        <table>
          <caption>Environmental inspection — {retained.owner.project}, {retained.owner.su}</caption>
          <thead><tr><th scope="col">Source</th><th scope="col">Label</th>
            {#each view.result.vegetation.units as unit}<th scope="col">{unit.code === '' ? '"" (empty unit)' : unit.code}</th>{/each}
          </tr></thead>
          <tbody>
            {#each view.result.rows as row}
              <tr data-environment-field-row={row.field.rowId}>
                <td>{row.field.cells[1].text === null ? 'NULL' : row.field.cells[1].text === '' ? '""' : row.field.cells[1].text}</td>
                <th scope="row">{row.field.cells[2].text === null ? 'NULL label' : row.field.cells[2].text === '' ? '""' : row.field.cells[2].text}
                  <small>Physical row {row.field.rowId}</small></th>
                {#each row.values as value}<td>{environmentalText(value)}</td>{/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if view.result.rows.length === 0}<p>No fields have Report=Yes. This is an empty selection, not a missing source.</p>{/if}
      <p>{view.result.source}</p>
    </article>
  {/if}
</section>

<style>
  .environmental { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; overflow-wrap: anywhere; }
  .actions { display: flex; flex-wrap: wrap; gap: .75rem; }
  button { min-height: 2.5rem; padding: .5rem .75rem; }
  .paths { overflow-wrap: anywhere; }
  .table-scroll { max-width: 100%; overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: .5rem; border: 1px solid #d1d5db; text-align: left; vertical-align: top; white-space: pre-wrap; }
  small { display: block; font-weight: normal; }
  article { min-width: 0; }
</style>
