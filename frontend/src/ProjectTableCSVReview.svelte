<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ContextService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { coreTableSuffixes, validateProjectTableCSVReview, type ValidatedProjectTableCSVReview } from './projectTableCSV';
  import { reportCellText } from './longEnvironmentReport';

  let { contextId, project, projectPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let suffix = $state<string>('Env');
  let busy = $state(false);
  let error = $state('');
  let review = $state<ValidatedProjectTableCSVReview | null>(null);
  const reads = new ReadRequests();
  let generation = 0;

  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function show() {
    if (busy) { error = 'Wait for or cancel the current table read before another review.'; return; }
    cancel(); const request = generation, table = `${project}_${suffix}`;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.GetProjectTableCSVReview(contextId, table));
      if (request !== generation) return;
      const validated = await validateProjectTableCSVReview(value, contextId, project, projectPath, table);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Table review unavailable; no data, audits, configuration or export files written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="table-review" data-project-table-csv-review aria-label="Project table CSV review">
  <h1>Project table CSV review</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <p>Project {project}. Whole physical project table; selected SU and plot/profile filters are not applied.</p>
  <dl><div><dt>Owned project database</dt><dd>{projectPath}</dd></div></dl>
  <div class="actions">
    <label for="csv-core-table">Physical core table
      <select id="csv-core-table" bind:value={suffix} disabled={busy} onchange={() => { review = null; error = ''; }}>
        {#each coreTableSuffixes as value}<option value={value}>{project}_{value}</option>{/each}
      </select>
    </label>
    <button type="button" data-table-csv-read disabled={busy} onclick={show}>Review core table CSV</button>
    {#if busy}<button type="button" data-table-csv-cancel onclick={cancel}>Cancel table read</button>
      <p role="status">Reading and checking the owned table snapshot...</p>{/if}
  </div>
  <p class="notice">Read-only migration preview. CSV text fields use JSON string escaping; storage tags distinguish NULL, empty text and BLOBs.
    File publication, import, RDS and TurboVeg remain unavailable.</p>
  {#if review}
    <p role="status" data-table-csv-summary>{review.manifest.rowIds.length} physical rows;
      {review.manifest.columns.length} columns. No data, audits, configuration or export files written.</p>
    <dl>
      <div><dt>Description metadata</dt><dd>{review.descriptionMetadataPresent ? 'Present physical table' : 'Absent physical table'}</dd></div>
      <div><dt>CSV SHA256</dt><dd data-table-csv-checksum>{review.manifest.sha256}</dd></div>
    </dl>
    <details>
      <summary>Physical Description candidates ({review.manifest.descriptions.length})</summary>
      {#each review.manifest.descriptions as candidate (candidate.rowId)}
        <p>Row {candidate.rowId}: {reportCellText(candidate.value)}</p>
      {:else}<p>No matching candidates. No description was inferred.</p>{/each}
    </details>
    <label for="csv-value-preview">JSON-escaped value CSV (read-only)</label>
    <textarea id="csv-value-preview" readonly rows="12" value={review.csv}></textarea>
    <details>
      <summary>Version1 typed manifest</summary>
      <pre>{JSON.stringify(review.manifest, null, 2)}</pre>
    </details>
  {/if}
</section>

<style>
  .table-review { padding: 1rem; max-width: 100%; min-width: 0; }
  h1 { font-size: 1.5rem; margin-bottom: .75rem; }
  .actions { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin: 1rem 0; }
  .actions label { display: flex; flex-direction: column; gap: .3rem; min-width: 0; }
  select, button, textarea { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; }
  button:disabled { opacity: .5; }
  textarea { display: block; width: 100%; margin-top: .4rem; font-family: monospace; }
  dl > div { margin: .6rem 0; }
  dt { font-weight: 600; }
  dd, p { overflow-wrap: anywhere; }
  details { margin: .75rem 0; }
  pre { overflow: auto; max-height: 24rem; padding: .5rem; background: #f1f5f9; }
  .notice { color: #475569; }
  .error { color: #991b1b; }
</style>
