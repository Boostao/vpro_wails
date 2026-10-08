<script lang="ts">
  import SIVIHistoricalRow from './SIVIHistoricalRow.svelte';
  import type { EditorCloseState } from './closeLifecycle';
  import type { SIVIDeletionRestorationView } from './siviDeletionRestorationSession';

  let { view, close, targets, disabled = false, recoveryDisabled = false, onreview, onchoose, onconfirm, onoperation }: {
    view: SIVIDeletionRestorationView; close: EditorCloseState;
    targets: { historyId: string; label: string; restored: boolean }[];
    disabled?: boolean; recoveryDisabled?: boolean;
    onreview: (historyId: string) => void;
    onchoose: (action: 'retain' | 'prune') => void;
    onconfirm: (confirmed: boolean) => void;
    onoperation: (operation: 'history' | 'save' | 'undo' | 'resolve' | 'cancel') => void;
  } = $props();
  const unavailable = $derived(disabled || view.busy || view.blocked);
</script>

<section class="my-3 min-w-0 space-y-3 rounded border border-stone-300 p-3"
  data-sivi-restoration-panel aria-label="SIVI deleted row restoration">
  <h2 class="font-semibold">Restore a reviewed deleted SIVI row</h2>
  {#if view.error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}
    <p role="alert" class="break-words text-sm">Request {view.requestId}: restoration outcome unknown. Do not repeat Restore or infer success from matching rows.</p>
    <button type="button" data-sivi-restoration-resolve class="rounded border px-3 py-2 disabled:opacity-50"
      disabled={recoveryDisabled || view.busy} onclick={() => onoperation('resolve')}>Resolve restoration history (read-only)</button>
  {/if}
  {#if view.busy}
    <button type="button" data-sivi-restoration-cancel class="rounded border px-3 py-2"
      onclick={() => onoperation('cancel')}>Cancel operation acknowledgement</button>
  {/if}
  <button type="button" data-sivi-restoration-history class="rounded border px-3 py-2 disabled:opacity-50"
    disabled={unavailable || view.review !== null} onclick={() => onoperation('history')}>Load owned deletion history (read-only)</button>
  {#if view.history}
    <p class="break-words text-sm" data-sivi-restoration-history-status>
      {#if !view.history.historyPresent}
        No durable deletion history exists for this project. This does not resolve an unknown request.
      {:else if view.history.events.length === 0}
        Durable deletion history exists, with no events for this owned plot.
      {:else}
        {view.history.events.length} owned historical events loaded. Already restored events remain visible but cannot be restored again.
      {/if}
    </p>
  {/if}
  <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
    {#each targets as target (target.historyId)}
      <button type="button" data-sivi-restoration-review={target.historyId}
        class="min-w-0 break-words rounded border px-3 py-2 text-left disabled:opacity-50"
        disabled={unavailable || view.review !== null || target.restored}
        onclick={() => onreview(target.historyId)}>{target.restored ? 'Already restored:' : 'Review restoration:'} {target.label}</button>
    {/each}
  </div>
  {#if view.review}
    <p class="break-words text-sm">Source {view.review.form}; application ID {view.review.id},
      physical row {view.review.rowId}; history {view.review.historyId}. The original identities must both remain vacant.</p>
    <div data-sivi-restoration-original>
      <SIVIHistoricalRow columns={view.review.columns} row={view.review.original} />
    </div>
    <fieldset class="space-y-2">
      <legend class="text-sm font-medium">Deletion source audits ({view.review.auditsBefore.length} exact rows)</legend>
      <label class="flex items-start gap-2 text-sm">
        <input type="radio" name={`sivi-restoration-action-${view.review.historyId}`} data-sivi-restoration-action="retain"
          checked={view.action === 'retain'} disabled={unavailable}
          onchange={() => onchoose('retain')} />
        Retain these source audits and mark them restored.
      </label>
      <label class="flex items-start gap-2 text-sm">
        <input type="radio" name={`sivi-restoration-action-${view.review.historyId}`} data-sivi-restoration-action="prune"
          checked={view.action === 'prune'} disabled={unavailable}
          onchange={() => onchoose('prune')} />
        Remove only these source audits after restoring the exact row.
      </label>
    </fieldset>
    <label class="flex items-start gap-2 text-sm">
      <input type="checkbox" data-sivi-restoration-confirm checked={view.confirmed} disabled={unavailable || !view.action}
        onchange={event => onconfirm(event.currentTarget.checked)} />
      Confirm restoring physical row {view.review.rowId}, application ID {view.review.id} and all44 original fields with this audit action.
    </label>
    <div class="flex flex-wrap gap-2">
      <button type="button" data-sivi-restoration-save class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={unavailable || !close.canSave} onclick={() => onoperation('save')}>Restore reviewed deleted row</button>
      <button type="button" data-sivi-restoration-undo class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={view.busy || view.blocked || recoveryDisabled}
        onclick={() => onoperation('undo')}>Undo unsubmitted restoration review</button>
    </div>
  {/if}
  {#if view.receipt}
    <p role="status" class="break-words text-sm" data-sivi-restoration-receipt>
      Verified restoration of physical row {view.receipt.rowId}, application ID {view.receipt.id},
      history {view.receipt.historyId}; {view.receipt.action} source audits.
      {view.receipt.replayed ? 'Observed an earlier commit; no restoration was repeated.' : 'One exact historical row was restored.'}
    </p>
  {/if}
  <p class="text-xs text-stone-600">Typed deleted-row restoration is a desktop safety adaptation, not original Access undelete.
    It preserves historical values and permanent ID reservations without allocating replacement identities or running destructive plot cleanup.
    A changed review token, occupied identity or changed schema refuses restoration.</p>
</section>
