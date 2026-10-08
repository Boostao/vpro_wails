<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviCoverLabels } from './siviCoverEditor';
  import {
    siviCombinedGroups, siviCombinedDirty, siviCombinedErrors, siviCombinedHiddenDrafts, isSIVICoverColumn,
    type SIVICombinedColumn,
  } from './siviCombinedEditor';
  import type { SIVICombinedView } from './siviCombinedSession';
  import type { SIVIProjection, SIVIRow } from './siviHeightEditor';
  import SIVIChildSourceNotices from './SIVIChildSourceNotices.svelte';

  let { view, disabled, canSave, onstage, onsave, onundo, onreload, onrestore }: {
    view: SIVICombinedView; disabled: boolean; canSave: boolean;
    onstage: (rowId: string, column: SIVICombinedColumn, raw: string, nullValue: boolean) => void;
    onsave: () => void; onundo: () => void; onreload: () => void;
    onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const groups = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
  const heights = { HeightA: 'Ht A', HeightB: 'Ht B', Height6: 'Ht' };
  const hidden = $derived(siviCombinedHiddenDrafts(view.drafts, view.extended));
  const errors = $derived(siviCombinedErrors(view.drafts));
  const dirty = $derived(siviCombinedDirty(view.drafts) || hidden);
  const unavailable = $derived(disabled || view.busy || view.blocked);
  function text(group: SIVIProjection, row: SIVIRow, column: string): string {
    const value = row.cells[group.Columns.indexOf(column)];
    return value.storage === 'null' ? 'NULL' : metadataCellText(value);
  }
  function label(column: SIVICombinedColumn): string {
    return isSIVICoverColumn(column) ? `${siviCoverLabels[column]} ${column.startsWith('Total') ? 'total' : 'cover'}` : heights[column];
  }
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-combined-panel aria-label="Combined SIVI source covers and heights">
  <h2 class="font-semibold">SIVI covers and heights: one owned child draft</h2>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; never replay Save or restoration.</p>{/if}
  {#if hidden}<p class="text-sm text-red-700" role="alert">Extended B3/B4/B5 drafts remain hidden. Show their source controls before saving, or Undo.</p>{/if}
  {#each errors as error}<p class="text-sm text-red-700" role="alert">{error}</p>{/each}
  {#if view.busy}<p role="status">Loading or saving owned combined SIVI rows...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-save
      disabled={unavailable || !dirty || hidden || errors.length > 0 || !canSave} onclick={onsave}>Save SIVI covers &amp; heights</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-undo
      disabled={view.busy || !dirty && !view.blocked} onclick={onundo}>Undo / reload combined rows</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-reload
      disabled={unavailable || dirty} onclick={onreload}>Reload combined source rows</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-restore="retain"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore combined rows, retain audit</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-combined-restore="prune"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore combined rows, prune selected audit</button>
    {/if}
  </div>
  {#if view.review}
    {#each view.review as group, index (group.Query)}
      <section class="space-y-3" data-sivi-combined-group={group.Form} aria-label={groups[index]}>
        <h3 class="font-semibold">{groups[index]}</h3>
        {#if group.Rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
        {#each group.Rows as row (row.rowId)}
          <article class="min-w-0 space-y-3 rounded border border-stone-200 p-3" data-sivi-combined-row={row.rowId}>
            <header class="flex flex-wrap items-baseline justify-between gap-2">
              <h4 class="font-medium">{groups[index]}: {text(group, row, 'Species')}</h4>
              <span class="text-xs text-stone-500">Physical row {row.rowId}; ID {text(group, row, 'ID')}</span>
            </header>
            <dl><dt class="text-xs text-stone-600">Collected (?)</dt><dd>{text(group, row, 'Collected')}</dd></dl>
            {#each siviCombinedGroups(index, view.extended) as columns}
              <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5" data-sivi-combined-field-group>
                {#each columns as column (column)}
                  {@const draft = isSIVICoverColumn(column) ? view.drafts.covers[row.rowId]?.[column] : view.drafts.heights[row.rowId]?.[column]}
                  {@const original = row.cells[group.Columns.indexOf(column)]}
                  {@const raw = draft?.raw ?? metadataCellText(original)}
                  {@const nullValue = draft?.nullValue ?? original.storage === 'null'}
                  {@const id = `sivi-combined-${row.rowId}-${column}`}
                  <div class="min-w-0" data-sivi-combined-field={column}>
                    <label for={id} class="mb-1 block text-sm font-medium">{label(column)}{column === 'HeightB' ? ' (literal text)' : ''}</label>
                    {#if column === 'HeightB'}
                      <textarea {id} rows="2" value={raw} class="w-full min-w-0 rounded border border-stone-300 px-3 py-2"
                        data-sivi-combined-column={column} data-sivi-combined-identity={row.rowId}
                        aria-label={`${label(column)} for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                        aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                        disabled={unavailable || nullValue}
                        oninput={event => onstage(row.rowId, column, event.currentTarget.value, false)}></textarea>
                      <label class="mt-2 flex items-center gap-2 text-sm" for={`${id}-null`}>
                        <input id={`${id}-null`} type="checkbox" checked={nullValue} disabled={unavailable}
                          aria-label={`Ht B NULL for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                          onchange={event => onstage(row.rowId, column, raw, event.currentTarget.checked)} />
                        NULL (distinct from empty text)
                      </label>
                    {:else}
                      <input {id} type="text" value={raw} class="w-full min-w-0 rounded border border-stone-300 px-3 py-2"
                        data-sivi-combined-column={column} data-sivi-combined-identity={row.rowId}
                        aria-label={`${label(column)} for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                        aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                        disabled={unavailable} oninput={event => onstage(row.rowId, column, event.currentTarget.value, false)} />
                    {/if}
                    {#if draft?.error}<p id={`${id}-error`} class="mt-1 text-sm text-red-700" role="alert">{draft.error}</p>{/if}
                  </div>
                {/each}
              </div>
            {/each}
          </article>
        {/each}
      </section>
    {/each}
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load all three owned source groups before editing.</p>
  {/if}
  <SIVIChildSourceNotices notices={view.sourceNotices ?? []} saved={view.sourceNoticesSaved ?? false} />
  <p class="text-xs text-stone-600">One atomic child Save covers source-bound covers/totals and aggregate heights.
    Numeric fields may clear to NULL; cover values remain strictly less than100, while heights retain their own accepted domains.
    Ht B retains literal TEXT255 with distinct NULL/empty values. Species, Collected, IDs, creation/deletion, parent actions and automatic totals remain unavailable here.</p>
  <p class="text-xs text-stone-600">Changing extended presentation never resets drafts, errors or this session's isolated history.
    Undo reloads originals; it does not reverse a committed write. Restoration consumes only this verified combined history.</p>
</section>
