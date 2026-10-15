<script lang="ts">
  import { onDestroy } from 'svelte';
  import { BreakReportService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { breakReportFields, breakReportFromWire, sameBreakReportSource, type BreakReportPage } from './breakReportReview';

  let { contextId, operationHeld = false, onBusyChange }: {
    contextId: string; operationHeld?: boolean; onBusyChange: (busy: boolean) => void;
  } = $props();
  let busy = $state(false);
  let error = $state('');
  let review = $state.raw<BreakReportPage | null>(null);
  const limit = 50;
  const reads = new ReadRequests();
  let generation = 0;

  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function show(offset = 0) {
    if (busy || operationHeld) { error = 'Wait for or cancel the current operation before reading break codes.'; return; }
    const previous = review;
    cancel();
    const request = generation, owner = contextId;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      if (!owner || offset > 0 && !previous) throw new Error('Review the first page in the current owned context before paging.');
      const value = await reads.track(BreakReportService.GetReview(owner, offset, limit));
      if (request !== generation || owner !== contextId) return;
      const validated = breakReportFromWire(value, owner, offset, limit);
      if (request !== generation || owner !== contextId) return;
      if (offset > 0 && previous && !sameBreakReportSource(previous, validated)) {
        throw new Error('The complete physical break-code source changed; review its first page again.');
      }
      review = validated;
    } catch (cause) {
      if (request === generation && owner === contextId) error = `PC-ORD Break Report unavailable; no data, audit or configuration writes invoked: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="break-report" data-break-report-review aria-label="PC-ORD Break Report">
  <h1>PC-ORD Break Report</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <p>Report caption: VPro Break Report. Saved break codes from the preserved VPro64 support database,
    not a newly generated export or a project/Site Unit/hierarchy filter.</p>
  <div class="actions">
    <button type="button" data-break-report-read disabled={busy || operationHeld || !contextId} onclick={() => show()}>Review break codes</button>
    {#if busy}
      <button type="button" data-break-report-cancel onclick={cancel}>Cancel break-code read</button>
      <p role="status">Reading the owned saved break codes...</p>
    {/if}
  </div>
  {#if review}
    <dl class="provenance">
      <div><dt>Owned VPro64 database</dt><dd>{review.sourcePath}</dd></div>
      <div><dt>Complete original source SHA256</dt><dd>{review.sourceSHA256}</dd></div>
    </dl>
    <p data-break-report-count>{review.total} distinct pairs; showing {review.rows.length ? review.offset + 1 : 0}
      through {review.rows.length ? review.offset + review.rows.length : 0}.</p>
    <div class="table-scroll">
      <table data-break-report-table>
        <caption>PC-ORD Break Report</caption>
        <thead><tr>{#each breakReportFields as field}<th scope="col">{field.caption}</th>{/each}</tr></thead>
        <tbody>
          {#each review.rows as row, index}
            <tr aria-label={`Break-code pair ${review.offset + index + 1}`}>
              {#each breakReportFields as field}
                <td>{row[field.key] === null ? '(NULL)' : row[field.key] === '' ? '(empty text)' : row[field.key]}</td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="actions">
      <button type="button" data-break-report-previous disabled={busy || operationHeld || review.offset === 0} onclick={() => show(Math.max(0, review!.offset - limit))}>Previous break-code pairs</button>
      <button type="button" data-break-report-next disabled={busy || operationHeld || review.offset + limit >= review.total} onclick={() => show(review!.offset + limit)}>Next break-code pairs</button>
    </div>
  {/if}
  <p class="guidance">Read-only saved DISTINCT Reference/BreakCode pairs; Order and PlotNumber do not group results.
    NULL and empty text remain distinct. Technical NULL-first code/reference order and paging are desktop adaptations.
    Ambiguous raw representatives, unsupported comparisons and sources above 10,000 physical rows are unavailable.
    Original timestamp/page footers, report printing and export generation are not implemented here.</p>
</section>

<style>
  .break-report { min-width: 0; }
  .actions { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin-block: 1rem; }
  .provenance { display: grid; grid-template-columns: minmax(0, 1fr); }
  dd { margin: 0; overflow-wrap: anywhere; }
  .table-scroll { max-width: 100%; overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  th, td { padding: .5rem; text-align: left; border-bottom: 1px solid #d1d5db; white-space: pre-wrap; overflow-wrap: anywhere; }
  th:first-child { width: 25%; }
  caption { text-align: left; font-weight: 600; }
  .guidance { margin-block-start: 1rem; }
</style>
