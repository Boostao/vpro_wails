<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ContextService, TableCSVArchiveService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { coreTableSuffixes, validateProjectTableCSVReview, type ValidatedProjectTableCSVReview } from './projectTableCSV';
  import { reportCellText } from './longEnvironmentReport';
  import { chooseTableCSVArchiveFile, tableCSVArchivePublicationSession, validateTableCSVArchiveReview,
    validTableCSVArchiveDestination, type ValidatedTableCSVArchiveReview } from './tableCSVArchiveExport';

  let { contextId, project, projectPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let suffix = $state<string>('Env');
  let busy = $state(false);
  let error = $state('');
  let review = $state<ValidatedProjectTableCSVReview | null>(null);
  const archiveEnabled = import.meta.env.VITE_TABLE_CSV_ARCHIVE_EXPORT === 'true';
  const scope = () => ({ contextId, project, projectPath });
  const publicationSession = archiveEnabled ? tableCSVArchivePublicationSession(scope()) : null;
  let publication = $state(publicationSession?.view() ?? null);
  let approval = $state<ValidatedTableCSVArchiveReview | null>(null);
  let destination = $state('');
  const unsubscribe = publicationSession?.subscribe(() => {
    publication = publicationSession.view();
    updateBusy();
  });
  const reads = new ReadRequests();
  let generation = 0;

  function updateBusy() { onBusyChange(busy || !!publication?.busy || !!publication?.blocked); }
  function cancel() {
    generation++; reads.cancelAll(); busy = false; approval = null; updateBusy();
  }
  async function show() {
    if (busy || publication?.busy || publication?.blocked) { error = 'Wait for or cancel the current table read and acknowledge any archive attempt before another review.'; return; }
    cancel(); const request = generation, table = `${project}_${suffix}`;
    review = null; error = ''; busy = true; updateBusy();
    try {
      if (archiveEnabled) {
        const value = await reads.track(TableCSVArchiveService.GetTableCSVArchiveReview(contextId, JSON.stringify({ table })));
        if (request !== generation) return;
        const validated = await validateTableCSVArchiveReview(value, { contextId, project, projectPath }, table);
        if (request !== generation) return;
        approval = validated; review = validated.review;
        return;
      }
      const value = await reads.track(ContextService.GetProjectTableCSVReview(contextId, table));
      if (request !== generation) return;
      const validated = await validateProjectTableCSVReview(value, contextId, project, projectPath, table);
      if (request !== generation) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Table review unavailable; no data, audits, configuration or export files written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  function changeTable() { cancel(); review = null; error = ''; }
  async function chooseDestination() {
    if (!publicationSession || busy || publication?.busy || publication?.blocked) return;
    error = '';
    try { destination = await publicationSession.chooseDestination(destination, chooseTableCSVArchiveFile); }
    catch (cause) { error = `Destination chooser unavailable; destination unchanged, no publication invoked: ${String(cause)}`; }
  }
  async function publish() {
    const table = `${project}_${suffix}`;
    if (!publicationSession || !approval || approval.review.manifest.table !== table || busy ||
        publication?.busy || publication?.blocked || !validTableCSVArchiveDestination(destination)) return;
    const approved = $state.snapshot(approval);
    approval = null; review = null; error = '';
    try {
      await publicationSession.publish(approved, table, destination, request =>
        TableCSVArchiveService.ExportReviewedTableCSVArchive(contextId, JSON.stringify(request)));
    } catch (cause) { error = `Archive export not invoked; review again before proceeding: ${String(cause)}`; }
  }
  function acknowledge() {
    try { publicationSession?.acknowledge(); changeTable(); }
    catch (cause) { error = String(cause); }
  }
  onDestroy(cancel);
  onDestroy(() => unsubscribe?.());
</script>

<section class="table-review" data-project-table-csv-review aria-label="Project table CSV review">
  <h1>Project table CSV review</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if publication}
    {#if publication.storageError}<p class="error" role="alert">{publication.storageError}</p>{/if}
    {#each publication.receipts as receipt, index}
      <section class="receipt" data-table-csv-archive-attempt
        data-table-csv-archive-receipt={receipt.phase === 'known' ? '' : undefined}
        data-table-csv-archive-unknown-receipt={receipt.phase === 'unknown' ? '' : undefined}
        aria-label={`Archive attempt ${index + 1}`}>
        <h2>Archive attempt {index + 1}: {receipt.phase === 'known' ? receipt.outcome?.status : receipt.phase === 'pending' ? 'awaiting receipt — outcome not yet known' : 'outcome unknown'}</h2>
        <p>{receipt.table}; requested literal destination: {receipt.requestedDestination}</p>
        <p>Expected archive SHA256: {receipt.archiveSHA256}</p>
        {#if receipt.outcome?.path}<p>Observed published path: {receipt.outcome.path}</p>{/if}
        {#if receipt.error}<p class="error" role="alert">{receipt.error}</p>{/if}
        <p>{receipt.acknowledged ? 'Acknowledged; historical evidence retained.' : 'Unacknowledged. Close and navigation are blocked. Publication cannot be cancelled or automatically replayed.'}</p>
      </section>
    {/each}
    {#if publication.blocked && !publication.storageError}
      <button type="button" data-table-csv-archive-acknowledge disabled={publication.busy} onclick={acknowledge}>Acknowledge archive outcome; retain all receipts</button>
    {/if}
    {#if publication.busy}<p role="status">Archive operation in progress; close and navigation are blocked.</p>{/if}
  {/if}
  <p>Project {project}. Whole physical project table; selected SU and plot/profile filters are not applied.</p>
  <dl><div><dt>Owned project database</dt><dd>{projectPath}</dd></div></dl>
  <div class="actions">
    <label for="csv-core-table">Physical core table
      <select id="csv-core-table" bind:value={suffix} disabled={(busy && !archiveEnabled) || !!publication?.busy || !!publication?.blocked} onchange={changeTable}>
        {#each coreTableSuffixes as value}<option value={value}>{project}_{value}</option>{/each}
      </select>
    </label>
    <button type="button" data-table-csv-read disabled={busy || !!publication?.busy || !!publication?.blocked} onclick={show}>Review core table CSV</button>
    {#if busy}<button type="button" data-table-csv-cancel onclick={cancel}>Cancel table read</button>
      <p role="status">Reading and checking the owned table snapshot...</p>{/if}
  </div>
  {#if archiveEnabled}
    {#if destination && !validTableCSVArchiveDestination(destination)}
      <p class="error" role="alert">Destination contains NUL or malformed UTF-16; export is blocked, and no spelling repair is applied.</p>
    {/if}
    <label for="table-csv-archive-destination">Literal migration archive destination</label>
    <textarea id="table-csv-archive-destination" rows="3" bind:value={destination} disabled={busy || !!publication?.busy || !!publication?.blocked}></textarea>
    <div class="actions">
      <button type="button" data-table-csv-archive-choose disabled={busy || !!publication?.busy || !!publication?.blocked} onclick={chooseDestination}>Choose ZIP destination...</button>
      <button type="button" data-table-csv-archive-publish data-table-csv-archive-export disabled={busy || !!publication?.busy || !!publication?.blocked ||
        !approval || approval.review.manifest.table !== `${project}_${suffix}` || !validTableCSVArchiveDestination(destination)} onclick={publish}>
        Export reviewed {project}_{suffix} migration archive
      </button>
    </div>
    {#if approval}
      <dl>
        <div><dt>Approved physical table</dt><dd>{approval.review.manifest.table}</dd></div>
        <div><dt>Backend source approval (not independently verified by browser)</dt><dd>{approval.approvalHash}</dd></div>
        <div><dt>Backend archive SHA256 (not independently computed by browser)</dt><dd>{approval.archiveSHA256}</dd></div>
        <div><dt>Backend archive byte count / format / version</dt><dd>{approval.byteCount} / {approval.format} / {approval.version}</dd></div>
      </dl>
    {/if}
    <p class="notice">Opt-in owned-table migration ZIP, not a plain analysis CSV. CSV text uses JSON escaping and typed storage tags.
      Destination spelling is preserved exactly: no trimming, recasing or extension completion. A native chooser selection is your explicit path choice.
      Review is read-only; only the explicit reviewed export button publishes. Import, RDS, viewer and TurboVeg remain unavailable.</p>
  {:else}
    <p class="notice">Read-only migration preview. CSV text fields use JSON string escaping; storage tags distinguish NULL, empty text and BLOBs.
      File publication, import, RDS and TurboVeg remain unavailable.</p>
  {/if}
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
  .receipt { border: 1px solid #94a3b8; padding: .75rem; margin: .75rem 0; }
  .receipt p { white-space: pre-wrap; }
</style>
