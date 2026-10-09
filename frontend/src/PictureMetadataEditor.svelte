<script lang="ts">
  import type { PictureMetadataSession } from './pictureMetadataSession';
  import { pictureMetadataColumns, pictureMetadataFieldValue, type PictureMetadataColumn } from './pictureMetadataEditor';

  let { session, disabled = false }: { session: PictureMetadataSession; disabled?: boolean } = $props();
  let revision = $state(0);
  $effect(() => session.subscribe(() => revision++));
  const view = $derived.by(() => { revision; return session.view(); });
  const close = $derived.by(() => { revision; return session.closeState(); });
  let error = $state('');

  function stage(column: PictureMetadataColumn, raw: string, nullValue: boolean) {
    try { session.stage(column, raw, nullValue); error = ''; }
    catch (cause) { error = String(cause); }
  }
  async function operation(action: 'save' | 'resolve' | 'discard') {
    try {
      error = '';
      if (action === 'save') await session.save(crypto.randomUUID());
      else if (action === 'resolve') await session.resolve();
      else await session.discardAndReload();
    } catch (cause) { error = String(cause); }
  }
</script>

{#if view.draft}
  <section class="editor" data-picture-metadata-editor aria-label="Existing picture metadata">
    <h4>Existing picture metadata</h4>
    {#if error || view.error}<p role="alert" class="error">{error || view.error}</p>{/if}
    <p>Literal parent {JSON.stringify(view.draft.plotNumber)}; ID {view.draft.id}; physical row {view.draft.original.rowId}.</p>
    {#if view.blocked}
      <p role="alert">The picture Save outcome is unknown. The original and draft remain retained.
        Matching current values do not prove this request committed. Resolve its durable receipt or explicitly discard and reload.</p>
    {/if}
    <div class="fields">
      {#each pictureMetadataColumns as column}
        {@const cell = pictureMetadataFieldValue(view.draft, column)}
        <div>
          <label for={`picture-metadata-${column}`}>{column === 'PicDir' ? 'Picture Directory' : 'Picture File Name'}</label>
          <input id={`picture-metadata-${column}`} value={cell.raw} disabled={disabled || view.busy || view.blocked || cell.nullValue}
            aria-invalid={cell.error !== null}
            oninput={event => stage(column, event.currentTarget.value, false)} />
          <label class="null"><input type="checkbox" checked={cell.nullValue} disabled={disabled || view.busy || view.blocked}
            onchange={event => stage(column, cell.raw, event.currentTarget.checked)} />NULL {column}</label>
          {#if cell.error}<p role="alert" class="error">{cell.error}</p>{/if}
        </div>
      {/each}
    </div>
    <div class="actions">
      <button onclick={() => void operation('save')} disabled={disabled || !close.canSave || !close.unsaved}>Save picture metadata only</button>
      {#if view.blocked}<button onclick={() => void operation('resolve')} disabled={disabled || view.busy}>Resolve picture Save receipt</button>{/if}
      <button onclick={() => void operation('discard')} disabled={disabled || view.busy}>Discard draft and reload picture</button>
      {#if view.busy}<button onclick={() => session.cancel()}>Cancel picture metadata operation</button>{/if}
    </div>
    {#if view.receipt}<p role="status">Picture metadata saved with durable history {view.receipt.historyId}. Image files were not changed.</p>{/if}
    <p class="guidance">Literal text, at most255 UTF-16 units. Directory permits empty text or NULL; File Name permits NULL, not new empty text.
      This saves only PicDir/PicName. It neither copies/renames image files nor authorizes external directories.
      Discard reloads the current stored row; it does not undo a completed Save.</p>
  </section>
{/if}

<style>
  .editor { display: grid; gap: .65rem; border: 1px solid #94a3b8; border-radius: .25rem; padding: .75rem; min-width: 0; }
  h4 { font-weight: 600; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(16rem, 100%), 1fr)); gap: .75rem; }
  .fields > div { display: grid; gap: .3rem; min-width: 0; }
  .null { display: flex; gap: .4rem; align-items: center; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; }
  input, button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .35rem .5rem; min-width: 0; }
  button:disabled, input:disabled { opacity: .5; }
  p { overflow-wrap: anywhere; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .guidance { font-size: .8rem; color: #475569; }
</style>
