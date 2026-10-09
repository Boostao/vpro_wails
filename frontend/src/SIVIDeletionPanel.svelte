<script lang="ts">
  import SIVIHistoricalRow from './SIVIHistoricalRow.svelte';
  import type { EditorCloseState } from './closeLifecycle';
  import type { SIVICreationForm } from './siviCreationEditor';
  import type { SIVIDeletionTarget } from './siviDeletionEditor';
  import type { SIVIDeletionView } from './siviDeletionSession';

  let { view, close, targets, disabled = false, recoveryDisabled = false, onreview, onconfirm, onoperation }: {
    view: SIVIDeletionView; close: EditorCloseState;
    targets: SIVIDeletionTarget[];
    disabled?: boolean; recoveryDisabled?: boolean;
    onreview: (form: SIVICreationForm, rowId: string) => void;
    onconfirm: (confirmed: boolean) => void;
    onoperation: (operation: 'load' | 'save' | 'undo' | 'resolve' | 'cancel') => void;
  } = $props();
  const unavailable = $derived(disabled || view.busy || view.blocked);
</script>

<section class="my-3 min-w-0 space-y-3 rounded border border-stone-300 p-3"
  data-sivi-deletion-panel aria-label="SIVI source row deletion">
  <h2 class="font-semibold">Delete a reviewed SIVI vegetation row</h2>
  {#if view.error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}
    <p role="alert" class="break-words text-sm">Request {view.requestId}: outcome unknown. Do not repeat Delete or infer failure from missing rows.</p>
    <button type="button" data-sivi-deletion-resolve class="rounded border px-3 py-2 disabled:opacity-50"
      disabled={recoveryDisabled || view.busy} onclick={() => onoperation('resolve')}>Resolve deletion history (read-only)</button>
  {/if}
  {#if view.busy}
    <button type="button" data-sivi-deletion-cancel class="rounded border px-3 py-2"
      onclick={() => onoperation('cancel')}>Cancel operation acknowledgement</button>
  {/if}
  <button type="button" data-sivi-deletion-load class="rounded border px-3 py-2 disabled:opacity-50"
    disabled={unavailable || view.original !== null} onclick={() => onoperation('load')}>Load owned source rows (read-only)</button>
  <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
    {#each targets as target (`${target.form}:${target.rowId}`)}
      <button type="button" data-sivi-deletion-review={target.rowId} data-sivi-deletion-form={target.form}
        class="min-w-0 break-words rounded border px-3 py-2 text-left disabled:opacity-50"
        disabled={unavailable || view.original !== null || target.unavailable !== null}
        title={target.unavailable ?? 'Read the complete fresh original before confirming any deletion.'}
        onclick={() => onreview(target.form, target.rowId)}>{target.unavailable !== null ? 'Deletion unavailable:' : 'Review deletion:'} {target.label}
        {#if target.unavailable}<span class="block text-sm">{target.unavailable}</span>{/if}</button>
    {/each}
  </div>
  {#if view.original}
    <p class="break-words text-sm">Source {view.original.form}; physical row {view.original.original.rowId}.
      Every stored field below belongs to the reviewed row, not merely its visible source controls.</p>
    <div data-sivi-deletion-original>
      <SIVIHistoricalRow columns={view.original.columns} row={view.original.original} />
    </div>
    <label class="flex items-start gap-2 text-sm">
      <input type="checkbox" data-sivi-deletion-confirm checked={view.confirmed} disabled={unavailable || close.blocked}
        onchange={event => onconfirm(event.currentTarget.checked)} />
      Confirm deletion of physical row {view.original.original.rowId} from {view.original.form}, including all44 stored fields.
    </label>
    <div class="flex flex-wrap gap-2">
      <button type="button" data-sivi-deletion-save class="rounded border border-red-700 px-3 py-2 text-red-800 disabled:opacity-50"
        disabled={unavailable || !close.canSave} onclick={() => onoperation('save')}>Delete reviewed vegetation row</button>
      <button type="button" data-sivi-deletion-undo class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={view.busy || view.blocked || recoveryDisabled}
        onclick={() => onoperation('undo')}>Undo unsubmitted deletion review</button>
    </div>
  {/if}
  {#if view.receipt}
    <p role="status" class="break-words text-sm" data-sivi-deletion-receipt>
      Verified {view.receipt.form} deletion: application ID {view.receipt.id}, physical row {view.receipt.rowId},
      history {view.receipt.historyId}.
      {view.receipt.replayed ? 'Observed an earlier commit; no deletion was repeated.' : 'One reviewed physical row was deleted.'}
    </p>
  {/if}
  <p class="text-xs text-stone-600">Deletion preserves complete typed historical evidence and permanently reserves the application ID.
    No historical value is trimmed, normalized or repaired. Deleted-row restoration requires a separate reviewed workflow;
    original Access audit restoration does not undelete missing rows.</p>
</section>
