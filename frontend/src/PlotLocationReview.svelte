<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ContextService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { validatePlotLocationReview, type ValidatedPlotLocationReview } from './plotLocationReview';
  import { reportCellText } from './longEnvironmentReport';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let review = $state<ValidatedPlotLocationReview | null>(null);
  let busy = $state(false);
  let error = $state('');
  let offset = $state(0);
  const pageSize = 50;
  const rows = $derived(review?.report.rows.slice(offset, offset + pageSize) ?? []);
  const reads = new ReadRequests();
  let generation = 0;

  function cancel() { generation++; reads.cancelAll(); busy = false; onBusyChange(false); }
  async function show() {
    if (busy) { error = 'Wait for or cancel the current location read before another review.'; return; }
    cancel(); const request = generation;
    review = null; error = ''; offset = 0; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.GetPlotLocationReview(contextId));
      if (request !== generation) return;
      const validated = await validatePlotLocationReview(value, contextId, project, projectPath, su, suPath);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Plot locations unavailable; no database, audits, configuration or output file written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="locations" data-plot-location-review aria-label="Plot location review">
  <h1>Plot location review</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <p>Project {project} / SU {su}. Selected context scope; current-plot and profile filters are not applied.</p>
  <dl><div><dt>Owned project database</dt><dd>{projectPath}</dd></div>
    {#if su !== 'None'}<div><dt>Owned SU database</dt><dd>{suPath}</dd></div>{/if}</dl>
  <div class="actions">
    <button type="button" data-plot-location-read disabled={busy} onclick={show}>Review plot locations</button>
    {#if busy}<button type="button" data-plot-location-cancel onclick={cancel}>Cancel location read</button>
      <p role="status">Reading and checking the owned location snapshot...</p>{/if}
  </div>
  {#if review}
    <p role="status" data-plot-location-summary>{review.report.rows.length} physical Env rows with an Admin match and both coordinates present.
      No data, audits, configuration or output files written.</p>
    <div class="table-scroll">
      <table>
        <caption>Source location report fields: latitude then negated stored longitude</caption>
        <thead><tr>{#each review.report.fields as field}<th id={`location-${field.key}`} scope="col">{field.label}</th>{/each}</tr></thead>
        <tbody>{#each rows as row (row.envRowId)}
          <tr data-plot-location-row={row.envRowId}>{#each review.report.fields as field, i}
            <td headers={`location-${field.key}`} aria-label={`${field.label} for plot ${reportCellText(row.values[0])}, Env row ${row.envRowId}`}>{reportCellText(row.values[i])}</td>
          {/each}</tr>
        {:else}<tr><td colspan="8">No qualifying coordinate rows. No coordinates were inferred.</td></tr>{/each}</tbody>
      </table>
    </div>
    <div class="actions" aria-label="Location review pages">
      <button type="button" disabled={offset === 0} onclick={() => { offset = Math.max(0, offset - pageSize); }}>Previous locations</button>
      <span>{review.report.rows.length ? offset + 1 : 0}-{Math.min(offset + pageSize, review.report.rows.length)} of {review.report.rows.length}</span>
      <button type="button" disabled={offset + pageSize >= review.report.rows.length} onclick={() => { offset += pageSize; }}>Next locations</button>
    </div>
    <details>
      <summary>Physical provenance for the displayed rows</summary>
      {#each rows as row (row.envRowId)}
        <p>Plot {reportCellText(row.values[0])}: Env row {row.envRowId}; Admin row {row.adminRowId};
          SU rows {row.membershipRowIds.length ? row.membershipRowIds.join(', ') : 'not selected'};
          stored longitude {reportCellText(row.storedLongitude)}.</p>
      {/each}
    </details>
  {/if}
  <p class="notice">Read-only migration preview. Longitude is the source report's numeric negation, not absolute-value normalization.
    Historical ranges are retained, not clamped or declared map-ready. UTM conversion, file publication, KML and Google Earth launch remain unavailable.</p>
</section>

<style>
  .locations { padding: 1rem; min-width: 0; max-width: 100%; }
  h1 { font-size: 1.5rem; margin-bottom: .75rem; }
  .actions { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; margin: 1rem 0; }
  button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; }
  button:disabled { opacity: .5; }
  dl > div { margin: .6rem 0; }
  dt { font-weight: 600; }
  dd, p { overflow-wrap: anywhere; }
  .table-scroll { overflow-x: auto; max-width: 100%; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #cbd5e1; padding: .4rem .6rem; text-align: left; vertical-align: top; white-space: pre-wrap; }
  th { background: #f1f5f9; }
  caption { text-align: left; margin-bottom: .5rem; }
  .notice { color: #475569; }
  .error { color: #991b1b; }
</style>
