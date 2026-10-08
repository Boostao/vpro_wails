<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviIdentityDirty, siviIdentityEditable, siviIdentityErrors, siviIdentityRows } from './siviIdentityEditor';
  import type { SIVIIdentityView } from './siviIdentitySession';
  import SIVIChildSourceNotices from './SIVIChildSourceNotices.svelte';

  let { view, disabled, canSave, onstage, onsave, onundo, onreload, onrestore }: {
    view: SIVIIdentityView; disabled: boolean; canSave: boolean;
    onstage: (rowId: string, raw: string, nullValue: boolean) => void;
    onsave: () => void; onundo: () => void; onreload: () => void;
    onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const errors = $derived(siviIdentityErrors(view.drafts, view.review ?? undefined));
  const dirty = $derived(siviIdentityDirty(view.drafts));
  const unavailable = $derived(disabled || view.busy || view.blocked);
  const rows = $derived(view.review?.some(group => group.Rows.length) ? siviIdentityRows(view.review) : []);
  const names = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-identity-panel aria-label="SIVI physical-row ID editing">
  <h2 class="font-semibold">SIVI vegetation IDs</h2>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; never replay Save or restoration.</p>{/if}
  {#each errors as error}<p class="text-sm text-red-700" role="alert">{error}</p>{/each}
  {#if view.busy}<p role="status">Loading or saving owned SIVI ID rows...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-save
      disabled={unavailable || !dirty || errors.length > 0 || !canSave} onclick={onsave}>Save SIVI IDs</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-undo
      disabled={view.busy || !dirty && !view.blocked} onclick={onundo}>Undo / reload IDs</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-reload
      disabled={unavailable || dirty} onclick={onreload}>Reload ID source rows</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-restore="retain"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore original IDs</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-identity-restore="prune"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore IDs (no source audits to prune)</button>
    {/if}
  </div>
  {#if view.review}
    {#if rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {#each rows as owner (owner.row.rowId)}
        {@const row = owner.row}
        {@const original = row.cells[0]}
        {@const draft = view.drafts[row.rowId]}
        {@const raw = draft?.raw ?? metadataCellText(original)}
        {@const nullValue = draft?.nullValue ?? original.storage === 'null'}
        {@const editable = siviIdentityEditable(original)}
        {@const id = `sivi-identity-${row.rowId}`}
        <article class="min-w-0 space-y-2 rounded border border-stone-200 p-3" data-sivi-identity-row={row.rowId}>
          <h3 class="break-words font-medium">Species: {row.cells[2].storage === 'null' ? 'NULL' : metadataCellText(row.cells[2])}</h3>
          <p class="text-xs text-stone-500">Physical row {row.rowId}; {owner.contexts.map(context => names[context.index]).join(', ')}</p>
          <label for={id} class="block text-sm font-medium">ID</label>
          <input {id} type="text" inputmode="numeric" class="w-full min-w-0 rounded border px-3 py-2 disabled:opacity-50"
            value={raw} disabled={unavailable || !editable || nullValue}
            aria-invalid={draft?.error ? 'true' : undefined} data-sivi-identity-field={row.rowId}
            oninput={event => onstage(row.rowId, event.currentTarget.value, false)} />
          <label class="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={nullValue} disabled={unavailable || !editable}
              data-sivi-identity-null={row.rowId}
              onchange={event => onstage(row.rowId, raw, event.currentTarget.checked)} />
            NULL ID for physical row {row.rowId}
          </label>
          {#if !editable}<p class="text-sm text-amber-800">Historical unsupported ID storage is read-only; no identity is repaired.</p>{/if}
        </article>
      {/each}
    </div>
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load all three owned source groups before editing IDs.</p>
  {/if}
  <SIVIChildSourceNotices notices={view.sourceNotices ?? []} saved={view.sourceNoticesSaved ?? false} />
  <p class="text-xs text-stone-600">ID is a nullable signed32 value, not the physical row identifier.
    New collisions, swaps and deleted/reserved reuse are refused; unchanged historical duplicates are preserved.
    ID changes have separate technical restoration history, not ordinary field audits.
    Restoration does not prune source audits or delete vegetation. Species, covers, heights, parent identity and creation/deletion are unavailable here.</p>
</section>
