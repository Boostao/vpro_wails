<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import {
    siviCoverDirty, siviCoverErrors, siviCoverHiddenDrafts, siviCoverGroups, siviCoverLabels,
    type SIVICoverColumn, type SIVIProjection, type SIVIRow,
  } from './siviCoverEditor';
  import type { SIVICoverView } from './siviCoverSession';

  let { view, disabled, canSave, onstage, onsave, onundo, onreload, onrestore }: {
    view: SIVICoverView;
    disabled: boolean;
    canSave: boolean;
    onstage: (rowId: string, column: SIVICoverColumn, raw: string, nullValue: boolean) => void;
    onsave: () => void;
    onundo: () => void;
    onreload: () => void;
    onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const groups = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
  const hidden = $derived(siviCoverHiddenDrafts(view.drafts, view.extended));
  const errors = $derived(siviCoverErrors(view.drafts));
  const dirty = $derived(siviCoverDirty(view.drafts) || hidden);
  const unavailable = $derived(disabled || view.busy || view.blocked);
  function text(group: SIVIProjection, row: SIVIRow, column: string): string {
    const cell = row.cells[group.Columns.indexOf(column)];
    return cell.storage === 'null' ? 'NULL' : metadataCellText(cell);
  }
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-cover-panel aria-label="SIVI covers and totals">
  <header><h2 class="font-semibold">SIVI covers and totals</h2></header>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; do not replay Save or restoration.</p>{/if}
  {#if hidden}<p class="text-sm text-red-700" role="alert">Extended B3/B4/B5 drafts are retained but hidden. Show extended controls before saving, or Undo.</p>{/if}
  {#each errors as error}<p class="text-sm text-red-700" role="alert">{error}</p>{/each}
  {#if view.busy}<p role="status">Loading or saving owned SIVI covers...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-save disabled={unavailable || !dirty || hidden || errors.length > 0 || !canSave} onclick={onsave}>Save SIVI covers</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-undo disabled={view.busy || (!dirty && !view.blocked)} onclick={onundo}>Undo / reload covers</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-reload disabled={unavailable || dirty} onclick={onreload}>Reload source rows</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-restore="retain" disabled={unavailable || dirty}
        onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore covers, retain audit</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-cover-restore="prune" disabled={unavailable || dirty}
        onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore covers, prune selected audit</button>
    {/if}
  </div>
  {#if view.review}
    {#each view.review as group, index (group.Query)}
      <section class="space-y-3" data-sivi-cover-group={group.Form} aria-label={groups[index]}>
        <h3 class="font-semibold">{groups[index]}</h3>
        {#if group.Rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
        {#each group.Rows as row (row.rowId)}
          <article class="min-w-0 space-y-3 rounded border border-stone-200 p-3" data-sivi-cover-row={row.rowId}>
            <header class="flex flex-wrap items-baseline justify-between gap-2">
              <h4 class="font-medium">{groups[index]}: {text(group, row, 'Species')}</h4>
              <span class="text-xs text-stone-500">Physical row {row.rowId}; application ID {text(group, row, 'ID')}</span>
            </header>
            <dl><dt class="text-xs text-stone-600">Collected (?)</dt><dd>{text(group, row, 'Collected')}</dd></dl>
            {#each siviCoverGroups(index, view.extended) as columns}
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {#each columns as column (column)}
                  {@const draft = view.drafts[row.rowId]?.[column]}
                  {@const original = row.cells[group.Columns.indexOf(column)]}
                  {@const raw = draft?.raw ?? metadataCellText(original)}
                  {@const id = `sivi-cover-${row.rowId}-${column}`}
                  <div class="min-w-0">
                    <label for={id} class="mb-1 block text-sm font-medium">{siviCoverLabels[column]}{column.startsWith('Total') ? ' total' : ' cover'}</label>
                    <input {id} type="text" value={raw} class="w-full min-w-0 rounded border border-stone-300 px-3 py-2"
                      data-sivi-cover-column={column} data-sivi-cover-identity={row.rowId}
                      aria-label={`${siviCoverLabels[column]} for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                      aria-invalid={Boolean(draft?.error)} disabled={unavailable}
                      oninput={(event) => onstage(row.rowId, column, event.currentTarget.value, false)} />
                  </div>
                {/each}
              </div>
            {/each}
          </article>
        {/each}
      </section>
    {/each}
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load the original source rows before editing.</p>
  {/if}
  <p class="text-xs text-stone-600">Source-scoped cover adaptation only: species, application ID, Collected and heights remain read-only. Changed numbers must be finite Access Single values strictly less than 100; zero and negative cover remain valid. Clear a field to NULL. No totals are calculated automatically.</p>
  <p class="text-xs text-stone-600">Extended presentation adds B3/B4/B5 without resetting drafts, errors or this session's isolated cover history. Save and restoration refresh the whole source. Undo discards drafts and reloads; it does not reverse a committed write.</p>
</section>
