<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { TwoPageSourceAuditService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { sourceAuditFields, twoPageAuditFromWire, twoPageAuditLabel, type TwoPageAuditOwner, type TwoPageAuditReview } from './twoPageAudit';

  let { contextId, project, plot, form, onBusyChange }: TwoPageAuditOwner & {
    onBusyChange: (busy: boolean) => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot, form }));
  let review = $state.raw<TwoPageAuditReview | null>(null);
  let busy = $state(false);
  let error = $state('');
  let generation = 0;
  const reads = new ReadRequests();

  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    onBusyChange(false);
  }
  async function load() {
    if (busy) return;
    cancel();
    const request = generation;
    review = null; error = ''; busy = true;
    onBusyChange(true);
    try {
      const value = await reads.track(TwoPageSourceAuditService.GetOriginal(owner.contextId, owner.plot, owner.form));
      if (request !== generation) return;
      review = twoPageAuditFromWire(value, owner);
    } catch (cause) {
      if (request === generation) error = `Source audit unavailable; history and data unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label="VPro Data History"
  data-two-page-audit-panel data-source-form="USysAuditXL" data-source-parent-form={owner.form}>
  <h3 class="font-semibold">VPro Data History</h3>
  <p class="break-words">Owned project {owner.project}; literal plot {JSON.stringify(owner.plot)}.</p>
  {#if error}<p role="alert" class="break-words text-red-700">{error}</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={busy} onclick={load}>Load source audit history</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel audit read</button><p role="status">Reading owned history...</p>{/if}
  </div>
  {#if review}
    <p role="status">{review.records.rows.length} physical audit rows. Ascending SQLite EditWhen storage order; ties retain physical row order.</p>
    {#each review.records.rows as row (row.rowId)}
      <article class="min-w-0 rounded border p-3" data-source-audit-row={row.rowId} aria-label={`Physical audit row ${row.rowId}`}>
        <h4 class="break-words font-medium">Physical audit row {row.rowId}</h4>
        <dl class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {#each sourceAuditFields as [column, label]}
            <div class="min-w-0" data-source-audit-column={column}>
              <dt class="font-medium">{label}</dt><dd class="whitespace-pre-wrap break-words">{twoPageAuditLabel(review, row, column)}</dd>
            </div>
          {/each}
        </dl>
      </article>
    {:else}<p>No audit history for this literal plot.</p>{/each}
  {/if}
  <p class="text-xs text-stone-600">Read-only adaptation of USysAuditXL / UsysAuditTrail.
    NULL, empty text, child identity and stored BOOLEAN values remain distinct.
    Restore/Flag editing, audit-strength preferences and restoration are unavailable in this source review.
    Loading or returning never saves the parent, changes history or prunes audits.</p>
</section>
