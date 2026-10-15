<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SubzoneMatrixService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { matrixPageFromWire, matrixWorkbookFromWire, sameMatrixSource, matrixCellText,
    matrixPublicationSession, type MatrixPage, type MatrixWorkbookReview } from './subzoneMatrixReport';

  let { contextId, hierarchy, hierarchyPath, onBusyChange }: {
    contextId: string; hierarchy: string; hierarchyPath: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, hierarchy, hierarchyPath }));
  const session = matrixPublicationSession(owner);
  let publication = $state(session.view());
  let preview = $state.raw<MatrixPage | null>(null);
  let workbook = $state.raw<MatrixWorkbookReview | null>(null);
  let busy = $state(false);
  let error = $state('');
  let destination = $state('');
  const barrier = $derived(publication.busy || publication.blocked);
  const available = $derived(!!contextId && hierarchy !== 'None' && !!hierarchyPath);
  const reads = new ReadRequests();
  const limit = 25;
  let generation = 0;
  function reportBusy() { onBusyChange(busy || barrier); }
  const unsubscribe = session.subscribe(() => { publication = session.view(); reportBusy(); });
  function cancel() {
    generation++; reads.cancelAll(); busy = false; reportBusy();
  }
  async function show(offset = 0) {
    if (busy || barrier || !available) return;
    const prior = offset === 0 ? null : preview;
    cancel(); const request = generation;
    error = ''; busy = true; reportBusy();
    try {
      const value = await reads.track(SubzoneMatrixService.GetPreview(contextId, JSON.stringify({ offset, limit })));
      if (request !== generation) return;
      const validated = matrixPageFromWire(value, owner, offset, limit);
      if (prior && !sameMatrixSource(prior, validated)) {
        throw new Error('Complete source changed between pages. Reload from the first page before continuing.');
      }
      if (request !== generation) return;
      if (workbook && !sameMatrixSource(workbook.preview, validated)) workbook = null;
      preview = validated;
    } catch (cause) {
      if (request === generation) {
        preview = null; workbook = null;
        error = `Matrix unavailable; no data or configuration writes invoked: ${String(cause)}`;
      }
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function reviewWorkbook() {
    if (busy || barrier || !available) return;
    cancel(); const request = generation;
    error = ''; workbook = null; busy = true; reportBusy();
    try {
      const value = await reads.track(SubzoneMatrixService.GetReview(contextId));
      if (request !== generation) return;
      const validated = matrixWorkbookFromWire(value, owner);
      if (request !== generation) return;
      workbook = validated; preview = validated.preview;
    } catch (cause) {
      if (request === generation) error = `Workbook review unavailable; no file published: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function publishWorkbook() {
    if (busy || barrier || !workbook) return;
    cancel(); error = '';
    try {
      await session.publish(workbook, destination, request =>
        SubzoneMatrixService.ExportReviewed(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function acknowledge() { session.acknowledge(); workbook = null; }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="matrix" data-subzone-matrix-report aria-label="Subzone Matrix of Units">
  <h1>Subzone Matrix of Units</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
  {#if !available}<p role="status">Select a physical hierarchy to review this report.</p>{/if}
  <dl class="provenance">
    <div><dt>Selected hierarchy</dt><dd>{hierarchy}</dd></div>
    <div><dt>Hierarchy database</dt><dd>{hierarchyPath}</dd></div>
    {#if preview}<div><dt>Owned VPro64 labels database</dt><dd>{preview.labelsPath}</dd></div>{/if}
  </dl>
  <div class="actions">
    <button type="button" data-matrix-read disabled={busy || barrier || !available} onclick={() => show()}>Review matrix</button>
    <button type="button" data-matrix-workbook-review disabled={busy || barrier || !available} onclick={reviewWorkbook}>Review XLSX workbook</button>
    {#if busy}
      <button type="button" data-matrix-cancel onclick={cancel}>Cancel matrix read</button>
      <p role="status">Reading or preparing the owned matrix...</p>
    {/if}
  </div>
  {#if preview}
    <p data-matrix-count>{preview.total} association rows; showing {preview.rows.length ? preview.offset + 1 : 0}
      through {preview.rows.length ? preview.offset + preview.rows.length : 0}.</p>
    {#if preview.total === 0}<p role="status">Nothing to report. Check the owned Subzone Matrix labels.</p>{/if}
    <div class="table-scroll">
      <table data-matrix-table>
        <caption>Subzone/Association Matrix: {hierarchy}</caption>
        <thead><tr><th scope="col">Name</th>{#each preview.columns as column}<th scope="col">{column}</th>{/each}</tr></thead>
        <tbody>
          {#each preview.rows as row, index}
            <tr aria-label={`Association row ${preview.offset + index + 1}`}>
              <th scope="row">{matrixCellText(row.name)}</th>
              {#each row.values as value}<td>{matrixCellText(value)}</td>{/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="actions">
      <button type="button" data-matrix-previous disabled={busy || barrier || preview.offset === 0} onclick={() => show(Math.max(0, preview!.offset - limit))}>Previous matrix rows</button>
      <button type="button" data-matrix-next disabled={busy || barrier || preview.offset + limit >= preview.total} onclick={() => show(preview!.offset + limit)}>Next matrix rows</button>
    </div>
  {/if}
  {#if workbook}
    <section class="workbook" data-matrix-workbook aria-label="Reviewed XLSX publication">
      <h2>Reviewed portable workbook</h2>
      <p>{workbook.bytes} bytes; worksheet qrySubzoneMatrix3.</p>
      <dl><div><dt>Exact workbook SHA256</dt><dd>{workbook.workbookSHA256}</dd></div>
        <div><dt>Source approval</dt><dd>{workbook.approvalHash}</dd></div></dl>
      <label for="matrix-destination">New XLSX destination
        <input id="matrix-destination" data-matrix-destination bind:value={destination} disabled={busy || barrier} />
      </label>
      <button type="button" data-matrix-publish disabled={busy || barrier || !destination} onclick={publishWorkbook}>Publish reviewed XLSX</button>
    </section>
  {/if}
  {#if publication.busy}<p role="status">Publishing the reviewed workbook. Publication cannot be cancelled or replayed.</p>{/if}
  {#if publication.outcome}
    <p data-matrix-receipt role="status">Workbook {publication.outcome.status}: {publication.outcome.path || publication.requestedDestination}
      {publication.outcome.sha256}</p>
  {/if}
  {#if publication.blocked && !publication.busy}
    <button type="button" data-matrix-acknowledge onclick={acknowledge}>Acknowledge workbook outcome</button>
  {/if}
  <p class="guidance">Uses the selected hierarchy and preserved VPro64 labels, not the current project or site-unit table.
    NULL and empty text remain distinct. Technical ASCII-key ordering and paged preview are desktop adaptations.
    Unsupported comparison keys or differing unordered First values are refused rather than guessed.
    Portable XLSX uses rotated headings and heuristic widths, not native Excel AutoFit or original Excel-host behavior.
    No source data or preferences are edited. Publication never overwrites a file.</p>
</section>

<style>
  .matrix { min-width: 0; }
  .actions { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin-block: 1rem; }
  .provenance, .workbook { display: grid; grid-template-columns: minmax(0, 1fr); gap: .5rem; }
  dd, [data-matrix-receipt] { margin: 0; overflow-wrap: anywhere; }
  label { display: grid; gap: .25rem; }
  input { box-sizing: border-box; width: 100%; min-width: 0; }
  .table-scroll { max-width: 100%; overflow-x: auto; }
  table { border-collapse: collapse; min-width: 100%; }
  th, td { min-width: 5rem; max-width: 24rem; padding: .5rem; text-align: left; border-bottom: 1px solid #d1d5db; white-space: pre-wrap; overflow-wrap: anywhere; }
  caption { text-align: left; font-weight: 600; }
  .workbook { margin-block: 1rem; }
  .workbook button { justify-self: start; }
  .guidance { margin-block-start: 1rem; }
</style>
