<script lang="ts">
  import { onDestroy } from 'svelte';
  import { PictureService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { pictureMetadataFromWire, pictureImageFromWire, pictureCellLabel, type PictureReview, type PicturePreview } from './pictureRead';
  import PictureMetadataEditor from './PictureMetadataEditor.svelte';
  import type { PictureMetadataSession } from './pictureMetadataSession';

  let { contextId, project, plotNumber, view = 'child', disabled = false, metadataDisabled, onBusyChange, metadataSession }: {
    contextId: string; project: string; plotNumber: string; disabled?: boolean;
    metadataDisabled?: boolean;
    view?: 'child' | 'manager';
    onBusyChange: (busy: boolean) => void;
    metadataSession?: PictureMetadataSession;
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
  let metadataRevision = $state(0);
  $effect(() => metadataSession?.subscribe(() => metadataRevision++));
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

  function editMetadata() {
    if (!metadataSession || !currentReview || !row || disabled || busy || metadataPending) return;
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
    if (disabled || busy || metadataPending) return;
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
    cancel();
    selected = value; image = null; error = ''; viewerOpen = false;
  }
  async function preview() {
    if (disabled || busy || !row) return;
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
    <button onclick={load} disabled={disabled || busy || metadataPending}>Load linked pictures</button>
    {#if busy}<button onclick={cancel}>Cancel picture read</button><span role="status">Reading owned pictures…</span>{/if}
  </div>
  {#if currentReview}
    {#if currentReview.records.rows.length === 0}
      <p>No linked picture metadata for this literal plot.</p>
    {:else}
      <label for="picture-record">Linked picture record</label>
      <select id="picture-record" value={selected} disabled={busy || disabled || metadataPending} onchange={event => select(event.currentTarget.value)}>
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
        <button onclick={preview} disabled={disabled || busy}>Preview selected picture</button>
        {#if metadataSession}<button onclick={editMetadata} disabled={disabled || busy || metadataPending}>Edit selected picture metadata</button>{/if}
      {/if}
    {/if}
  {/if}
  {#if metadataSession}
    <PictureMetadataEditor session={metadataSession} disabled={(metadataDisabled ?? disabled) || busy} />
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
    {#if metadataSession}External directories and adding/deleting records remain unavailable.
    {:else}External directories, adding/deleting records and metadata editing remain unavailable.{/if}
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
