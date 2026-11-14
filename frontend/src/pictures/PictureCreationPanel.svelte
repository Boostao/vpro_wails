<script lang="ts">
  import { untrack } from 'svelte';
  import type { PictureCreationSession } from './pictureCreationSession';
  let { session, disabled = false }: { session: PictureCreationSession; disabled?: boolean } = $props();
  let revision = $state(0);
  $effect(() => session.subscribe(() => untrack(() => revision++)));
  const view = $derived.by(() => { revision; return session.view(); });
  let error = $state('');
  async function operate(operation: 'add' | 'selection' | 'create' | 'receipt' | 'preview' | 'release' | 'acknowledge' | 'cancel') {
    error = '';
    try {
      if (operation === 'cancel') session.cancel();
      else if (operation === 'add') {
        if (disabled) throw new Error('Finish the current picture read or metadata draft before adding.');
        if (await session.add(crypto.randomUUID())) await session.preview();
      } else if (operation === 'create') {
        if (await session.create()) await session.preview();
      } else if (operation === 'selection') await session.recoverSelection();
      else if (operation === 'receipt') await session.recoverReceipt();
      else if (operation === 'preview') await session.preview();
      else if (operation === 'release') await session.release();
      else await session.acknowledge();
    } catch (cause) { error = `Picture operation unavailable; retained state is unchanged: ${String(cause)}`; }
  }
</script>

<section class="creation" data-picture-creation aria-label="Add a Picture">
  <h2>Add a Picture</h2>
  {#if error || view.error}<p role="alert" class="error">{error || view.error}</p>{/if}
  {#if view.phase === 'choosing'}
    <p role="status">Finish or Cancel the native file chooser. Cancelling a promise does not close that dialog.</p>
  {:else if view.busy}<p role="status">Picture operation: {view.phase}. Original request and receipts are retained.</p>{/if}
  <div class="actions">
    <button data-picture-add disabled={disabled || view.busy || view.blocked} onclick={() => operate('add')}>Add a Picture</button>
    {#if view.blocked}
      <button data-picture-recover-selection disabled={view.busy} onclick={() => operate('selection')}>Recover selection (read-only)</button>
      {#if view.request && view.attempted}
        <button data-picture-recover-receipt disabled={view.busy} onclick={() => operate('receipt')}>Resolve creation history (read-only)</button>
      {/if}
      {#if view.selection && view.request && !view.receipt && !view.attempted}
        <button data-picture-create-recovered disabled={view.busy} onclick={() => operate('create')}>Create recovered selection once</button>
      {/if}
      {#if !view.receipt}
        <button data-picture-release disabled={view.busy || (view.attempted && !view.noCommitVerified)} onclick={() => operate('release')}>Release verified uncommitted selection</button>
      {/if}
    {/if}
    {#if view.receipt && !view.acknowledged}
      <button data-picture-selected-preview disabled={view.busy || !view.selection} onclick={() => operate('preview')}>Preview this selected file</button>
      <button data-picture-acknowledge disabled={view.busy} onclick={() => operate('acknowledge')}>Acknowledge retained creation receipt</button>
    {/if}
    {#if view.busy && !['choosing', 'releasing', 'acknowledging'].includes(view.phase)}
      <button data-picture-cancel onclick={() => operate('cancel')}>Cancel operation acknowledgement</button>
    {/if}
  </div>
  {#if view.selection}
    <p data-picture-selected-path>Selected literal file: {view.selection.selectedPath}</p>
  {/if}
  {#if view.request}<p data-picture-creation-request>Stable request: {view.request.requestId}</p>{/if}
  {#if view.image}
    <img data-picture-selected-image src={view.image.dataUrl} alt="Selected file for the retained committed row" />
    <p>{view.image.width} × {view.image.height}; {view.image.mime}; SHA256 {view.image.sha256}</p>
  {/if}
  {#if view.receipts.length}
    <details data-picture-creation-receipts open>
      <summary>Retained committed creation receipts ({view.receipts.length})</summary>
      {#each view.receipts as receipt (receipt.requestId)}
        <p data-picture-creation-receipt>Request {receipt.requestId}: ID {receipt.id}, physical row {receipt.rowId},
          history {receipt.historyId}; stored {receipt.selectedPath}; PicComment remains NULL.</p>
      {/each}
    </details>
  {/if}
  <p class="guidance">Source-bound, independently gated creation. Accepting a file appends its literal path and parent once;
    Cancel makes no assignments. Files are not copied. This selection never authorizes its directory or sibling files.
    Preview or refresh failures retain the metadata commit. Deletion and restoration remain unavailable.</p>
</section>

<style>
  .creation { display: grid; grid-template-columns: minmax(0, 1fr); gap: .65rem; margin: 1rem 0; padding: .75rem; border: 1px solid #94a3b8; border-radius: .25rem; min-width: 0; }
  h2 { font-weight: 600; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; }
  button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .5rem .65rem; min-width: 0; overflow-wrap: anywhere; }
  button:disabled { opacity: .5; }
  p, summary { overflow-wrap: anywhere; }
  img { max-width: 100%; max-height: 28rem; object-fit: contain; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .guidance { font-size: .8rem; color: #475569; }
</style>
