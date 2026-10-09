<script lang="ts">
  import { untrack } from 'svelte';
  import type { PictureLifecycleSession, PictureLifecycleView } from './pictureLifecycleSession';
  import { pictureCellLabel } from './pictureRead';

  let { session, disabled = false }: { session: PictureLifecycleSession; disabled?: boolean } = $props();
  const restoringEnabled = import.meta.env.VITE_PICTURE_DELETION_RESTORING === 'true';
  let view = $state.raw<PictureLifecycleView>(untrack(() => session.view()));
  let localError = $state('');
  $effect(() => session.subscribe(() => { view = session.view(); }));
  const close = $derived.by(() => { view; return session.closeState(); });
  const original = $derived(view.restoration?.deletion.request.original ??
    view.original?.records.rows.find(row => row.rowId === view.rowId));
  const columns = $derived(view.restoration?.deletion.request.columns ?? view.original?.records.columns);
  const blocked = $derived(disabled || view.busy || view.blocked || view.acknowledgementRequired);
  async function run(operation: () => void | Promise<unknown>) {
    localError = '';
    try { await operation(); }
    catch (cause) { localError = `Picture lifecycle action unavailable: ${String(cause)}`; }
  }
</script>

<section class="lifecycle" data-picture-lifecycle aria-label="Picture deletion and historical restoration">
  <h3>Picture reference lifecycle</h3>
  {#if view.error || localError}<p role="alert" class="error">{view.error || localError}</p>{/if}
  {#if view.busy}
    <button data-picture-lifecycle-cancel onclick={() => session.cancel()}>Cancel acknowledgement or read</button>
  {/if}
  {#if view.blocked}
    <p class="safety">Mutation outcome is unknown. The exact original/request is retained; do not repeat Delete or Restore.</p>
    <button data-picture-lifecycle-recover disabled={view.busy} onclick={() => run(() => session.resolve())}>Recover picture receipt read-only</button>
  {/if}
  {#if original && columns}
    <p data-picture-lifecycle-row>Reviewed physical rowid {JSON.stringify(original.rowId)}.
      {view.restoration ? 'Restore the historical reference.' : 'Delete this reference only; its image file is not deleted.'}</p>
    <div class="fields">
      {#each columns as column, index}
        <label>{column.name}<output>{pictureCellLabel(original.cells[index])}</output></label>
      {/each}
    </div>
    <label class="confirmation">
      <input type="checkbox" data-picture-lifecycle-confirm checked={view.confirmed} disabled={blocked}
        onchange={event => { const checked = event.currentTarget.checked; void run(() => session.confirm(checked)); }} />
      {view.restoration ? 'Restore exactly this historical physical row; retain all histories and ID reservations.'
        : 'Delete exactly this reviewed physical reference; permanently reserve its ID and retain technical history.'}
    </label>
    <div class="actions">
      <button data-picture-lifecycle-save disabled={disabled || !close.canSave}
        onclick={() => run(() => session.save(crypto.randomUUID()))}>
        {view.restoration ? 'Restore historical picture reference' : 'Delete reviewed picture reference'}
      </button>
      <button data-picture-lifecycle-discard disabled={blocked} onclick={() => run(() => session.discard())}>Discard uncommitted review</button>
    </div>
    {#if !close.canSave}<p class="safety">{close.saveReason}</p>{/if}
  {/if}
  {#if view.acknowledgementRequired}
    <p class="safety">The physical operation committed. Acknowledge its receipt before navigating; the receipt remains retained.</p>
    <button data-picture-lifecycle-acknowledge disabled={view.busy || view.blocked} onclick={() => run(() => session.acknowledge())}>
      Acknowledge committed picture receipt
    </button>
  {/if}
  {#if view.receipts.length}
    <details data-picture-lifecycle-receipts>
      <summary>Retained irreversible receipts ({view.receipts.length})</summary>
      {#each view.receipts as commit}
        <p>{commit.kind === 'delete' ? 'Deleted' : 'Restored'} physical row
          {JSON.stringify(commit.kind === 'delete' ? commit.receipt.request.original.rowId : commit.receipt.request.deletion.original.rowId)};
          history {commit.kind === 'delete' ? commit.receipt.historyId : commit.receipt.request.historyId};
          actor {JSON.stringify(commit.receipt.actor)}.</p>
      {/each}
    </details>
  {/if}
  {#if restoringEnabled}
    <details data-picture-lifecycle-history>
      <summary>Historical picture deletions</summary>
      <button data-picture-lifecycle-history-load disabled={blocked || !!original} onclick={() => run(() => session.loadHistory())}>Load deletion history</button>
      {#if view.history}
        {#if view.history.events.length === 0}<p>No owned deletion history was returned.</p>{/if}
        {#each view.history.events as event}
          <div class="history">
            <p>History {event.deletion.historyId}; physical row {JSON.stringify(event.deletion.request.original.rowId)};
              ID {event.deletion.id}; original actor {JSON.stringify(event.deletion.actor)}.
              {event.restoration ? 'Already restored; acknowledgement retained.' : 'Not restored.'}</p>
            <button disabled={blocked || !!original || event.restoration !== null}
              aria-label={`Review picture restoration history ${event.deletion.historyId}`}
              onclick={() => run(() => session.reviewRestoration(event.deletion.historyId))}>Review historical restoration</button>
          </div>
        {/each}
      {/if}
    </details>
  {/if}
  <p class="guidance">Independently gated desktop safety adaptation. This does not establish Access GUI confirmation or Current parity.
    Only the five-cell physical reference is mutated; image files are untouched. Restoration never replaces an occupied rowid/ID,
    repairs identities or prunes source audits/history.</p>
</section>

<style>
  .lifecycle { display: grid; gap: .65rem; min-width: 0; max-width: 100%; padding: .75rem; border: 1px solid #cbd5e1; }
  h3, summary { font-weight: 600; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 16rem), 1fr)); gap: .65rem; }
  label { display: grid; gap: .25rem; min-width: 0; }
  output { display: block; min-width: 0; padding: .4rem; background: #f1f5f9; overflow-wrap: anywhere; }
  .confirmation { display: flex; gap: .5rem; align-items: start; }
  .confirmation input { margin-top: .25rem; flex: none; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; }
  button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; min-width: 0; min-height: 2rem; overflow-wrap: anywhere; }
  button:disabled, input:disabled { opacity: .5; }
  details, .history { min-width: 0; display: block; }
  summary { cursor: pointer; min-height: 2rem; padding: .35rem 0; overflow-wrap: anywhere; }
  .history { padding: .5rem 0; border-bottom: 1px solid #e2e8f0; }
  p { overflow-wrap: anywhere; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .safety { color: #92400e; background: #fffbeb; padding: .5rem; }
  .guidance { color: #475569; font-size: .8rem; }
</style>
