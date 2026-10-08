<script lang="ts">
  import SIVICreationFields from './SIVICreationFields.svelte';
  import type { SIVICreationView } from './siviCreationSession';
  import type { SIVICreationDraft, SIVICreationForm } from './siviCreationEditor';
  import type { SIVISpeciesOwner } from './siviSpeciesEditor';
  import type { EditorCloseState } from './closeLifecycle';

  let { view, close, owner, disabled = false, recoveryDisabled = false, onoperation, onchange }: {
    view: SIVICreationView; close: EditorCloseState; owner: SIVISpeciesOwner;
    disabled?: boolean; recoveryDisabled?: boolean;
    onoperation: (operation: 'load' | 'start' | 'save' | 'undo' | 'resolve' | 'cancel', form?: SIVICreationForm) => void;
    onchange: (draft: SIVICreationDraft) => void;
  } = $props();
  const forms: { form: SIVICreationForm; label: string }[] = [
    { form: 'SubVegA-SIVI_BC', label: 'A/B standard shrubs' },
    { form: 'SubVegA-SIVI', label: 'A/B extended shrubs' },
    { form: 'SubVegC-SIVI', label: 'C herbs' },
    { form: 'SubVegD-SIVI', label: 'D mosses' },
  ];
</script>

<section class="my-3 space-y-3 rounded border border-stone-300 p-3" data-sivi-creation-panel aria-label="SIVI source row creation">
  <h2 class="font-semibold">Create a source-bound SIVI row</h2>
  {#if view.error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}
    <p role="alert" class="break-words text-sm">Request {view.requestId}: outcome unknown. Do not repeat Create or infer success from matching rows.</p>
    <button type="button" data-sivi-creation-resolve class="rounded border px-3 py-2 disabled:opacity-50"
      disabled={recoveryDisabled || view.busy} onclick={() => onoperation('resolve')}>Resolve creation history (read-only)</button>
  {/if}
  {#if view.busy}
    <button type="button" data-sivi-creation-cancel class="rounded border px-3 py-2"
      onclick={() => onoperation('cancel')}>Cancel operation acknowledgement</button>
  {/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" data-sivi-creation-references class="rounded border px-3 py-2 disabled:opacity-50"
      disabled={disabled || view.busy || view.blocked} onclick={() => onoperation('load')}>Load creation Species definitions</button>
    {#each forms as source}
      <button type="button" data-sivi-creation-start={source.form} class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={disabled || view.busy || view.blocked || !view.references || view.draft !== null}
        onclick={() => onoperation('start', source.form)}>New {source.label} row</button>
    {/each}
  </div>
  {#if view.draft && view.references}
    <SIVICreationFields draft={view.draft} {owner} references={view.references}
      disabled={disabled || view.busy || view.blocked} {onchange} />
    <div class="flex flex-wrap gap-2">
      <button type="button" data-sivi-creation-save class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={disabled || !close.canSave} onclick={() => onoperation('save')}>Create reviewed row</button>
      <button type="button" data-sivi-creation-undo class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={view.busy || view.blocked || recoveryDisabled} onclick={() => onoperation('undo')}>Undo unsubmitted creation draft</button>
    </div>
  {/if}
  {#if view.receipt}
    <p role="status" class="break-words text-sm" data-sivi-creation-receipt>
      Verified {view.receipt.form} creation: application ID {view.receipt.id}, physical row {view.receipt.rowId},
      history {view.receipt.historyId}. {view.receipt.replayed ? 'Observed an earlier commit; no mutation was repeated.' : 'One reviewed row was committed.'}
    </p>
  {/if}
  <p class="text-xs text-stone-600">Creation assigns only exact parent, Species and reviewed source covers/totals.
    Untouched fields stay NULL, Flag stays false, and no heights or automatic sums are inferred.
    Positive signed32 identity allocation is a desktop adaptation, not Access random-number equivalence.
    Deletion and historical creation restoration remain unavailable.</p>
</section>
