<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { PictureService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { pictureMetadataFromWire, pictureImageFromWire, pictureCellLabel, type PictureReview, type PicturePreview } from './pictureRead';
  import PictureMetadataEditor from './PictureMetadataEditor.svelte';
  import type { PictureMetadataSession } from './pictureMetadataSession';
  import { pictureCreationSession } from './pictureCreationSessions';
  import type { PictureCreationReceipt } from './pictureCreationEditor';
  import { equalCell } from './projectMetadataEditor';
  import { pictureLifecycleSession } from './pictureLifecycleSessions';
  import type { PictureLifecycleCommit, PictureLifecycleSession } from './pictureLifecycleSession';
  import { pictureLifecycleOriginalMatches } from './pictureLifecycleEditor';

  let { contextId, project, plotNumber, view = 'child', disabled = false, metadataDisabled, onBusyChange, metadataSession,
    observeMutationReceipts = true }: {
    contextId: string; project: string; plotNumber: string; disabled?: boolean;
    metadataDisabled?: boolean;
    view?: 'child' | 'manager';
    onBusyChange: (busy: boolean) => void;
    metadataSession?: PictureMetadataSession;
    observeMutationReceipts?: boolean;
  } = $props();
  const reads = new ReadRequests();
  let generation = 0;
  let busy = $state(false);
  let error = $state('');
  let review = $state<PictureReview | null>(null);
  let image = $state<PicturePreview | null>(null);
  let selected = $state('');
  let viewerOpen = $state(false);
  let viewer = $state<HTMLDialogElement>();
  const ownerKey = $derived(JSON.stringify([contextId, project, plotNumber, view]));
  let reviewKey = $state('');
  const currentReview = $derived(reviewKey === ownerKey ? review : null);
  const currentImage = $derived(reviewKey === ownerKey ? image : null);
  const row = $derived(currentReview?.records.rows.find(item => item.rowId === selected));
  const creationSession = $derived(observeMutationReceipts && import.meta.env.VITE_PICTURE_CREATION_WRITING === 'true' && contextId && project && plotNumber
    ? pictureCreationSession({ contextId, project, plotNumber }) : undefined);
  let creationRevision = $state(0);
  $effect(() => creationSession?.subscribe(() => untrack(() => creationRevision++)));
  const creationView = $derived.by(() => { creationRevision; return creationSession?.view(); });
  let appliedCreationRequest = '';
  const deletionEnabled = import.meta.env.VITE_PICTURE_DELETION_WRITING === 'true';
  const lifecycleSession = $derived(observeMutationReceipts && (deletionEnabled || import.meta.env.VITE_PICTURE_DELETION_RESTORING === 'true') && contextId && project && plotNumber
    ? pictureLifecycleSession({ contextId, project, plotNumber }) : undefined);
  let lifecycleRevision = $state(0);
  $effect(() => lifecycleSession?.subscribe(() => untrack(() => lifecycleRevision++)));
  const lifecycleView = $derived.by(() => { lifecycleRevision; return lifecycleSession?.view(); });
  const lifecyclePending = $derived.by(() => {
    lifecycleRevision;
    const close = lifecycleSession?.closeState();
    return !!close && (close.unsaved || close.busy || close.blocked);
  });
  let appliedLifecycleSession: PictureLifecycleSession | undefined;
  let appliedLifecycleRevision = 0;
  let metadataRevision = $state(0);
  $effect(() => metadataSession?.subscribe(() => untrack(() => metadataRevision++)));
  const metadataClose = $derived.by(() => { metadataRevision; return metadataSession?.closeState(); });
  const metadataView = $derived.by(() => { metadataRevision; return metadataSession?.view(); });
  const metadataPending = $derived(!!metadataClose && (metadataClose.unsaved || metadataClose.busy));
  let appliedMetadataSession: PictureMetadataSession | undefined;
  let appliedMetadataRevision = 0;
  function synchronizeMetadataOriginal() {
    if (metadataSession !== appliedMetadataSession) {
      appliedMetadataSession = metadataSession;
      appliedMetadataRevision = 0;
    }
    const update = metadataView?.originalUpdate;
    if (!update || update.revision === appliedMetadataRevision) return;
    appliedMetadataRevision = update.revision;
    if (!review || reviewKey !== ownerKey || update.contextId !== contextId ||
        update.project !== project || update.plotNumber !== plotNumber) return;
    review = { ...review, records: { ...review.records,
      rows: review.records.rows.map(item => item.rowId === update.original.rowId ? update.original : item) } };
    image = null; viewerOpen = false;
  }
  $effect(synchronizeMetadataOriginal);

  async function refreshCreated(receipt: PictureCreationReceipt) {
    cancel();
    const request = generation, key = ownerKey, owner = { contextId, project, plotNumber };
    busy = true; error = ''; image = null; viewerOpen = false;
    onBusyChange(true);
    try {
      const value = await reads.track(PictureService.GetMetadata(contextId, plotNumber));
      if (request !== generation || key !== ownerKey) return;
      const refreshed = pictureMetadataFromWire(value, owner);
      const committed = refreshed.records.rows.find(item => item.rowId === receipt.rowId);
      if (!committed || committed.cells.some((cell, index) => !equalCell(cell, receipt.committed.cells[index]))) {
        throw new Error('The complete committed physical row differs from the refreshed owned picture list.');
      }
      review = refreshed; reviewKey = key; selected = receipt.rowId;
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Picture creation committed; linked-list refresh failed and receipt is retained: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  $effect(() => {
    const receipt = creationView?.receipts.at(-1);
    if (!receipt || metadataPending || receipt.requestId === appliedCreationRequest ||
        receipt.contextId !== contextId || receipt.project !== project || receipt.plotNumber !== plotNumber) return;
    appliedCreationRequest = receipt.requestId;
    void refreshCreated(receipt);
  });

  async function refreshLifecycle(commit: PictureLifecycleCommit) {
    cancel();
    const request = generation, key = ownerKey, owner = { contextId, project, plotNumber };
    busy = true; error = ''; review = null; reviewKey = ''; selected = ''; image = null; viewerOpen = false;
    onBusyChange(true);
    try {
      const value = await reads.track(PictureService.GetMetadata(contextId, plotNumber));
      if (request !== generation || key !== ownerKey) return;
      const refreshed = pictureMetadataFromWire(value, owner);
      const original = commit.kind === 'delete' ? commit.receipt.request.original : commit.receipt.request.deletion.original;
      if (commit.kind === 'delete') {
        if (refreshed.records.rows.some(row => row.rowId === original.rowId ||
            row.cells[0].storage === 'integer' && row.cells[0].integer === original.cells[0].integer)) {
          throw new Error('The deleted physical row/identity reappeared; no replacement is inferred.');
        }
      } else {
        const expected = [...commit.receipt.request.peers, original].sort((left, right) =>
          BigInt(left.rowId) < BigInt(right.rowId) ? -1 : BigInt(left.rowId) > BigInt(right.rowId) ? 1 : 0);
        if (refreshed.records.rows.length !== expected.length ||
            refreshed.records.rows.some((row, index) => !pictureLifecycleOriginalMatches(row, expected[index]))) {
          throw new Error('The complete restored physical row/peers differ from the retained committed plan.');
        }
        selected = original.rowId;
      }
      review = refreshed; reviewKey = key;
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Picture lifecycle committed; linked-list refresh failed and irreversible receipt is retained: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  $effect(() => {
    if (appliedLifecycleSession !== lifecycleSession) {
      appliedLifecycleSession = lifecycleSession; appliedLifecycleRevision = 0;
    }
    const state = lifecycleView, commit = state?.receipts.at(-1);
    if (!state || !commit || metadataPending || state.revision === appliedLifecycleRevision) return;
    appliedLifecycleRevision = state.revision;
    void refreshLifecycle(commit);
  });

  async function reviewDeletion() {
    if (!lifecycleSession || !row || !deletionEnabled || view !== 'manager' || disabled || busy || metadataPending || lifecyclePending) return;
    try { await lifecycleSession.reviewDeletion(row.rowId); error = ''; }
    catch (cause) { error = `Picture deletion review unavailable: ${String(cause)}`; }
  }

  function editMetadata() {
    if (!metadataSession || !currentReview || !row || disabled || busy || metadataPending || lifecyclePending) return;
    try { metadataSession.select(currentReview, row.rowId); error = ''; }
    catch (cause) { error = `Picture metadata editing unavailable: ${String(cause)}`; }
  }

  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    onBusyChange(false);
  }
  async function load() {
    if (disabled || busy || metadataPending || lifecyclePending) return;
    cancel();
    const request = generation, key = ownerKey, owner = { contextId, project, plotNumber };
    busy = true; error = ''; review = null; image = null; selected = ''; viewerOpen = false;
    onBusyChange(true);
    try {
      const value = await reads.track(PictureService.GetMetadata(contextId, plotNumber));
      if (request !== generation || key !== ownerKey) return;
      review = pictureMetadataFromWire(value, owner);
      reviewKey = key;
      selected = review.records.rows[0]?.rowId ?? '';
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Linked pictures unavailable; no files or data changed: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  function select(value: string) {
    if (metadataPending) { error = 'Finish or discard the retained picture metadata draft before changing selection.'; return; }
    if (lifecyclePending) { error = 'Finish the retained picture lifecycle review or resolve/acknowledge its receipt before changing selection.'; return; }
    cancel();
    selected = value; image = null; error = ''; viewerOpen = false;
  }
  async function preview() {
    if (disabled || busy || !row || lifecyclePending) return;
    cancel();
    const request = generation, key = ownerKey, owner = { contextId, project, plotNumber };
    busy = true; error = ''; image = null; viewerOpen = false;
    onBusyChange(true);
    try {
      const original = $state.snapshot(row);
      const value = await reads.track(PictureService.GetImage(contextId, plotNumber, JSON.stringify({ view, original })));
      if (request !== generation || key !== ownerKey || selected !== original.rowId) return;
      image = pictureImageFromWire(value, owner, original.rowId);
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Picture preview unavailable; metadata and original files retained: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  function measured(event: Event) {
    if (!(event.currentTarget instanceof HTMLImageElement) || !currentImage) return;
    if (event.currentTarget.naturalWidth !== currentImage.width || event.currentTarget.naturalHeight !== currentImage.height) {
      error = 'Displayed picture dimensions disagree with the owned decoded image; reload before viewing.';
      image = null; viewerOpen = false;
    }
  }
  function displayError() {
    error = 'The owned picture bytes could not be displayed; metadata and files are unchanged.';
    image = null; viewerOpen = false;
  }
  $effect(() => {
    if (viewerOpen && currentImage && viewer && !viewer.open) viewer.showModal();
    else if (!viewerOpen && viewer?.open) viewer.close();
  });
  onDestroy(cancel);
</script>

<section class="pictures" data-picture-panel aria-label="Linked plot pictures">
  <h3>Pictures</h3>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <div class="actions">
    <button onclick={load} disabled={disabled || busy || metadataPending || lifecyclePending}>Load linked pictures</button>
    {#if busy}<button onclick={cancel}>Cancel picture read</button><span role="status">Reading owned pictures…</span>{/if}
  </div>
  {#if currentReview}
    {#if currentReview.records.rows.length === 0}
      <p>No linked picture metadata for this literal plot.</p>
    {:else}
      <label for="picture-record">Linked picture record</label>
      <select id="picture-record" value={selected} disabled={busy || disabled || metadataPending || lifecyclePending} onchange={event => select(event.currentTarget.value)}>
        {#each currentReview.records.rows as item (item.rowId)}
          <option value={item.rowId}>{pictureCellLabel(item.cells[2])} — ID {pictureCellLabel(item.cells[0])}; physical row {item.rowId}</option>
        {/each}
      </select>
      {#if row}
        <dl class="metadata">
          <div><dt>{view === 'manager' ? 'Picture File Name' : 'PicName'}</dt><dd>{pictureCellLabel(row.cells[2])}</dd></div>
          <div><dt>{view === 'manager' ? 'Picture Directory' : 'PicDir'}</dt><dd>{pictureCellLabel(row.cells[1])}</dd></div>
          <div><dt>PicComment</dt><dd>{pictureCellLabel(row.cells[4])}</dd></div>
        </dl>
        <button onclick={preview} disabled={disabled || busy || lifecyclePending}>Preview selected picture</button>
        {#if metadataSession}<button onclick={editMetadata} disabled={disabled || busy || metadataPending || lifecyclePending}>Edit selected picture metadata</button>{/if}
        {#if deletionEnabled && view === 'manager'}
          <button data-picture-delete-review onclick={reviewDeletion} disabled={disabled || busy || metadataPending || lifecyclePending}>Review deletion of selected picture reference</button>
        {/if}
      {/if}
    {/if}
  {/if}
  {#if metadataSession}
    <PictureMetadataEditor session={metadataSession} disabled={(metadataDisabled ?? disabled) || busy || lifecyclePending} />
  {/if}
  {#if currentImage}
    <button class="preview" aria-label="Open full-size picture" ondblclick={() => viewerOpen = true} onclick={() => viewerOpen = true}>
      <img src={currentImage.dataUrl} alt={row ? pictureCellLabel(row.cells[2]) : 'Selected plot picture'} onload={measured} onerror={displayError} />
    </button>
    <p>{currentImage.width} × {currentImage.height}; verified {currentImage.mime}. Click or double-click to open the viewer.</p>
  {/if}
  <p class="guidance">Read-only linked metadata and JPEG/PNG preview. Each selected row uses its own PicDir.
    {#if view === 'manager'}Default uses the explicitly authorized manager directory, not the FS882 child directory.
    {:else}Default uses the explicitly authorized child directory, not the manager PictureDir.{/if}
    {#if deletionEnabled && view === 'manager'}Deletion requires the separate lifecycle review and backend permission.
    {:else}Record deletion remains unavailable here.{/if}
    {#if metadataSession}External directories and adding records remain unavailable here.
    {:else}External directories, adding records and metadata editing remain unavailable here.{/if}
    Files are never copied or repaired.</p>
</section>
{#if viewerOpen && currentImage}
    <dialog bind:this={viewer} class="viewer" aria-label="Pictures viewer" oncancel={() => viewerOpen = false} onclose={() => viewerOpen = false}>
      <button onclick={() => viewerOpen = false}>Close picture viewer</button>
      <img src={currentImage.dataUrl} alt={row ? pictureCellLabel(row.cells[2]) : 'Selected plot picture'} onload={measured} onerror={displayError} />
    </dialog>
{/if}

<style>
  .pictures { display: grid; gap: .65rem; min-width: 0; padding: .75rem; }
  .pictures h3 { font-weight: 600; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button, select { border: 1px solid #94a3b8; border-radius: .25rem; padding: .35rem .5rem; }
  button:disabled, select:disabled { opacity: .5; }
  select { min-width: 0; max-width: 100%; }
  .metadata { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(15rem, 100%), 1fr)); gap: .5rem; }
  dt { font-weight: 600; }
  dd { white-space: pre-wrap; overflow-wrap: anywhere; }
  .preview { width: fit-content; max-width: 100%; }
  img { max-width: 100%; height: auto; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; overflow-wrap: anywhere; }
  .guidance { font-size: .8rem; color: #475569; }
  .viewer::backdrop { background: #0008; }
  .viewer { background: white; padding: 1rem; max-width: calc(100% - 2rem); max-height: calc(100% - 2rem); overflow: auto; }
  .viewer[open] { display: grid; gap: .75rem; }
</style>
