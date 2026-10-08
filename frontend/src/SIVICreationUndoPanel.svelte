<script lang="ts">
  import SIVIHistoricalRow from './SIVIHistoricalRow.svelte';
  import type { EditorCloseState } from './closeLifecycle';
  import type { SIVICreationUndoView } from './siviCreationUndoSession';

  let { view, close, disabled = false, recoveryDisabled = false, onreview, onchoose, onconfirm, onoperation }: {
    view: SIVICreationUndoView; close: EditorCloseState;
    disabled?: boolean; recoveryDisabled?: boolean;
    onreview: (historyId: string) => void;
    onchoose: (action: 'retain' | 'prune') => void;
    onconfirm: (confirmed: boolean) => void;
    onoperation: (operation: 'history' | 'undo' | 'discard' | 'resolve' | 'cancel') => void;
  } = $props();
  const unavailable = $derived(disabled || view.busy || view.blocked);
</script>

<section class="my-3 min-w-0 space-y-3 rounded border border-stone-300 p-3"
  data-sivi-creation-undo-panel aria-label="SIVI historical creation Undo">
  <h2 class="font-semibold">Undo a reviewed historical SIVI creation</h2>
  {#if view.error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}
    <p role="alert" class="break-words text-sm">Request {view.requestId}: creation Undo outcome unknown.
      Do not repeat Undo, discard this request, close, or infer success from current rows.</p>
    <button type="button" data-sivi-creation-undo-resolve class="rounded border px-3 py-2 disabled:opacity-50"
      disabled={recoveryDisabled || view.busy} onclick={() => onoperation('resolve')}>Resolve Undo receipt (read-only)</button>
  {/if}
  {#if view.busy}
    <button type="button" data-sivi-creation-undo-cancel class="rounded border px-3 py-2"
      onclick={() => onoperation('cancel')}>Cancel operation acknowledgement</button>
  {/if}
  <button type="button" data-sivi-creation-undo-history class="rounded border px-3 py-2 disabled:opacity-50"
    disabled={unavailable || view.review !== null} onclick={() => onoperation('history')}>Load owned creation history (read-only)</button>
  {#if view.history === null}
    <p class="text-sm" data-sivi-creation-undo-history-status>Creation history has not been loaded. Select no creation by inference.</p>
  {:else}
    <p class="break-words text-sm" data-sivi-creation-undo-history-status>
      {#if !view.history.historyPresent}
        No durable creation history exists for this project. This does not resolve an unknown request.
      {:else if view.history.events.length === 0}
        Durable creation history exists, with no events for this owned plot.
      {:else}
        {view.history.events.length} owned historical events loaded. Consumed and legacy-identity creations remain visible but unavailable.
        Review availability permits only a fresh review request, not an inference of current row eligibility.
      {/if}
    </p>
    <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {#each view.history.events as target (target.historyId)}
        <button type="button" data-sivi-creation-undo-review={target.historyId}
          class="min-w-0 break-words rounded border px-3 py-2 text-left disabled:opacity-50"
          disabled={unavailable || view.review !== null || target.consumed || !target.reviewAvailable}
          onclick={() => onreview(target.historyId)}>{target.consumed ? 'Already consumed:' : !target.reviewAvailable ? 'Unavailable creation:' : 'Review creation:'}
          {target.species}; {target.form}; application ID {target.id}; physical row {target.rowId};
          {target.actor}; {target.editWhen}; history {target.historyId}{target.undone ? ' (undone)' : ''}
          {#if target.unavailableReason !== null}<span class="block text-sm">{target.unavailableReason}</span>{/if}</button>
      {/each}
    </div>
  {/if}
  {#if view.review}
    <p class="break-words text-sm">Source {view.review.form}; application ID {view.review.id},
      physical row {view.review.rowId}; history {view.review.historyId}. Undo removes only this exact unchanged created row.</p>
    <div data-sivi-creation-undo-committed>
      <SIVIHistoricalRow columns={view.review.columns} row={view.review.committed} />
    </div>
    <fieldset class="space-y-2">
      <legend class="text-sm font-medium">Creation source audits ({view.review.auditsBefore.length} exact rows)</legend>
      <label class="flex items-start gap-2 text-sm">
        <input type="radio" name={`sivi-creation-undo-action-${view.review.historyId}`} data-sivi-creation-undo-action="retain"
          checked={view.action === 'retain'} disabled={unavailable} onchange={() => onchoose('retain')} />
        Retain these exact creation source audits and mark them restored.
      </label>
      <label class="flex items-start gap-2 text-sm">
        <input type="radio" name={`sivi-creation-undo-action-${view.review.historyId}`} data-sivi-creation-undo-action="prune"
          checked={view.action === 'prune'} disabled={unavailable} onchange={() => onchoose('prune')} />
        Remove only these exact creation source audits after removing the row.
      </label>
    </fieldset>
    <label class="flex items-start gap-2 text-sm">
      <input type="checkbox" data-sivi-creation-undo-confirm checked={view.confirmed} disabled={unavailable || !view.action}
        onchange={event => onconfirm(event.currentTarget.checked)} />
      Confirm removing physical row {view.review.rowId}, application ID {view.review.id}, history {view.review.historyId}
      and all44 committed fields with the selected {view.action ?? 'unselected'} audit action.
    </label>
    <div class="flex flex-wrap gap-2">
      <button type="button" data-sivi-creation-undo-submit class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={unavailable || !close.canSave} onclick={() => onoperation('undo')}>Undo reviewed historical creation</button>
      <button type="button" data-sivi-creation-undo-discard class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={view.busy || view.blocked || recoveryDisabled}
        onclick={() => onoperation('discard')}>Discard unsubmitted creation Undo review</button>
    </div>
  {/if}
  {#if view.receipt}
    <p role="status" class="break-words text-sm" data-sivi-creation-undo-receipt>
      Verified creation Undo of physical row {view.receipt.rowId}, application ID {view.receipt.id},
      history {view.receipt.historyId}; {view.receipt.action} source audits.
      {view.receipt.replayed ? 'Observed an earlier commit; no Undo was repeated.' : 'One exact historical created row was removed.'}
    </p>
  {/if}
  <p class="text-xs text-stone-600">Historical creation Undo is an independent private safety adaptation, not a create/delete grant.
    Parent Save or Save-and-close must never submit it implicitly. Changed rows, source audits or review authority refuse Undo.
    Cancel stops acknowledgement delivery, not necessarily SQL; unknown requests can only be resolved read-only.</p>
</section>
