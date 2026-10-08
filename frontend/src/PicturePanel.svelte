<script lang="ts">
  import { onDestroy } from 'svelte';
  import { PictureService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { pictureMetadataFromWire, pictureImageFromWire, pictureCellLabel, type PictureReview, type PicturePreview } from './pictureRead';

  let { contextId, project, plotNumber, disabled = false, onBusyChange }: {
    contextId: string; project: string; plotNumber: string; disabled?: boolean;
    onBusyChange: (busy: boolean) => void;
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
  const ownerKey = $derived(JSON.stringify([contextId, project, plotNumber]));
  let reviewKey = $state('');
  const currentReview = $derived(reviewKey === ownerKey ? review : null);
  const currentImage = $derived(reviewKey === ownerKey ? image : null);
  const row = $derived(currentReview?.records.rows.find(item => item.rowId === selected));

  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    onBusyChange(false);
  }
  async function load() {
    if (disabled || busy) return;
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
      const value = await reads.track(PictureService.GetImage(contextId, plotNumber, JSON.stringify({ view: 'child', original })));
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
    <button onclick={load} disabled={disabled || busy}>Load linked pictures</button>
    {#if busy}<button onclick={cancel}>Cancel picture read</button><span role="status">Reading owned pictures…</span>{/if}
  </div>
  {#if currentReview}
    {#if currentReview.records.rows.length === 0}
      <p>No linked picture metadata for this literal plot.</p>
    {:else}
      <label for="picture-record">Linked picture record</label>
      <select id="picture-record" value={selected} disabled={busy || disabled} onchange={event => select(event.currentTarget.value)}>
        {#each currentReview.records.rows as item (item.rowId)}
          <option value={item.rowId}>{pictureCellLabel(item.cells[2])} — ID {pictureCellLabel(item.cells[0])}; physical row {item.rowId}</option>
        {/each}
      </select>
      {#if row}
        <dl class="metadata">
          <div><dt>PicName</dt><dd>{pictureCellLabel(row.cells[2])}</dd></div>
          <div><dt>PicDir</dt><dd>{pictureCellLabel(row.cells[1])}</dd></div>
          <div><dt>PicComment</dt><dd>{pictureCellLabel(row.cells[4])}</dd></div>
        </dl>
        <button onclick={preview} disabled={disabled || busy}>Preview selected picture</button>
      {/if}
    {/if}
  {/if}
  {#if currentImage}
    <button class="preview" aria-label="Open full-size picture" ondblclick={() => viewerOpen = true} onclick={() => viewerOpen = true}>
      <img src={currentImage.dataUrl} alt={row ? pictureCellLabel(row.cells[2]) : 'Selected plot picture'} onload={measured} onerror={displayError} />
    </button>
    <p>{currentImage.width} × {currentImage.height}; verified {currentImage.mime}. Click or double-click to open the viewer.</p>
  {/if}
  <p class="guidance">Read-only linked metadata and JPEG/PNG preview. Each selected row uses its own PicDir. Default uses the explicitly authorized child directory, not the manager PictureDir. External directories, adding/deleting records and metadata editing remain unavailable. Files are never copied or repaired.</p>
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
