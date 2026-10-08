<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviCollectedDirty, siviCollectedErrors, siviCollectedRows } from './siviCollectedEditor';
  import type { SIVICollectedView } from './siviCollectedSession';
  import type { SIVIProjection, SIVIRow } from './siviHeightEditor';
  import SIVIChildSourceNotices from './SIVIChildSourceNotices.svelte';

  let { view, disabled, canSave, oncycle, onsave, onundo, onreload, onrestore }: {
    view: SIVICollectedView; disabled: boolean; canSave: boolean;
    oncycle: (rowId: string) => void; onsave: () => void; onundo: () => void; onreload: () => void;
    onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const groups = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
  const errors = $derived(siviCollectedErrors(view.drafts));
  const dirty = $derived(siviCollectedDirty(view.drafts));
  const unavailable = $derived(disabled || view.busy || view.blocked);
  const owners = $derived(view.review?.some(group => group.Rows.length)
    ? new Map(siviCollectedRows(view.review).map(({ group, row }) => [row.rowId, group.Form]))
    : new Map<string, string>());
  function text(group: SIVIProjection, row: SIVIRow, column: string): string {
    const cell = row.cells[group.Columns.indexOf(column)];
    return cell.storage === 'null' ? 'NULL' : metadataCellText(cell);
  }
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-collected-panel aria-label="SIVI Collected source cycles">
  <h2 class="font-semibold">SIVI Collected (?) cycles</h2>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; never replay Save or restoration.</p>{/if}
  {#each errors as error}<p class="text-sm text-red-700" role="alert">{error}</p>{/each}
  {#if view.busy}<p role="status">Loading or saving owned SIVI Collected rows...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-save
      disabled={unavailable || !dirty || errors.length > 0 || !canSave} onclick={onsave}>Save SIVI Collected</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-undo
      disabled={view.busy || !dirty && !view.blocked} onclick={onundo}>Undo / reload Collected</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-reload
      disabled={unavailable || dirty} onclick={onreload}>Reload Collected source rows</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-restore="retain"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore Collected, retain audit</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-collected-restore="prune"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore Collected, prune selected audit</button>
    {/if}
  </div>
  {#if view.review}
    {#each view.review as group, index (group.Query)}
      <section class="space-y-3" data-sivi-collected-group={group.Form} aria-label={groups[index]}>
        <h3 class="font-semibold">{groups[index]}</h3>
        {#if group.Rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each group.Rows as row (row.rowId)}
            {@const original = row.cells[group.Columns.indexOf('Collected')]}
            {@const value = view.drafts[row.rowId]?.value ?? original}
            {@const valueText = value.storage === 'null' ? 'NULL' : metadataCellText(value)}
            {@const id = `sivi-collected-${row.rowId}`}
            <article class="min-w-0 space-y-2 rounded border border-stone-200 p-3" data-sivi-collected-row={row.rowId}>
              <h4 class="break-words font-medium">Species: {text(group, row, 'Species')}</h4>
              <p class="text-xs text-stone-500">Physical row {row.rowId}; ID {text(group, row, 'ID')}</p>
              {#if owners.get(row.rowId) === group.Form}
                <label for={id} class="block text-sm font-medium">Collected (?)</label>
                <button {id} type="button" class="w-full min-w-0 break-words rounded border px-3 py-2 disabled:opacity-50"
                  data-sivi-collected-identity={row.rowId} aria-label={`Cycle Collected (?) for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                  disabled={unavailable || original.storage !== 'null' && original.storage !== 'text'}
                  onclick={() => oncycle(row.rowId)}>{valueText}</button>
                {#if original.storage !== 'null' && original.storage !== 'text'}
                  <p class="text-sm text-amber-800">Historical non-text Collected is read-only; no value is coerced.</p>
                {/if}
              {:else}
                <dl><dt class="text-sm font-medium">Collected (?)</dt><dd class="break-words">{valueText}</dd></dl>
                <p class="text-xs text-stone-600">The same physical field is edited in its first source group above.</p>
              {/if}
            </article>
          {/each}
        </div>
      </section>
    {/each}
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load all three owned source groups before cycling.</p>
  {/if}
  <SIVIChildSourceNotices notices={view.sourceNotices ?? []} saved={view.sourceNoticesSaved ?? false} />
  <p class="text-xs text-stone-600">Click cycles NULL, C, V, then NULL. Other stored text remains unchanged.
    One live control owns each physical field, even when a row appears in multiple source groups.
    Save applies reviewed cycle intent only; Species, covers, heights, IDs, metadata and creation/deletion are not editable here.</p>
</section>
