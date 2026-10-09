<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviHeightDirty, type SIVIHeightColumn, type SIVIProjection, type SIVIRow } from './siviHeightEditor';
  import type { SIVIHeightView } from './siviHeightSession';
  import SIVIChildSourceNotices from './SIVIChildSourceNotices.svelte';

  let { view, disabled, canSave, onstage, onsave, onundo, onreload, onrestore }: {
    view: SIVIHeightView;
    disabled: boolean;
    canSave: boolean;
    onstage: (rowId: string, column: SIVIHeightColumn, raw: string, nullValue: boolean) => void;
    onsave: () => void;
    onundo: () => void;
    onreload: () => void;
    onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const groups = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
  const coverLabels: Record<string, string> = {
    Cover1: 'A1', Cover2: 'A2', Cover3: 'A3', TotalA: 'A',
    Cover4: 'B1', Cover5: 'B2', Cover5a: 'B3', Cover5b: 'B4', Cover5c: 'B5', TotalB: 'B',
    Cover6: 'C', Cover7: 'D', Cover8: 'Dr/Dw', Cover9: 'Ep',
  };
  const heightLabels = { HeightA: 'Ht A', HeightB: 'Ht B', Height6: 'Ht' };
  function text(group: SIVIProjection, row: SIVIRow, column: string): string {
    const cell = row.cells[group.Columns.indexOf(column)];
    return cell.storage === 'null' ? 'NULL' : metadataCellText(cell);
  }
  function heights(index: number): SIVIHeightColumn[] {
    return index === 0 ? ['HeightA', 'HeightB'] : index === 1 ? ['Height6'] : [];
  }
  const dirty = $derived(siviHeightDirty(view.drafts));
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-height-panel aria-label="SIVI aggregate heights">
  <header>
    <h2 class="font-semibold">SIVI aggregate heights</h2>
    <p class="text-sm text-stone-600">Height-only source adaptation, not the complete FS1333 form. Species, cover, Collected and parent workflows are not editable here.</p>
  </header>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; do not replay Save or restoration.</p>{/if}
  {#if view.busy}<p role="status">Loading or saving owned SIVI heights...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-save disabled={disabled || !dirty || !canSave} onclick={onsave}>Save SIVI heights</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-undo disabled={view.busy || (!dirty && !view.blocked)} onclick={onundo}>Undo / reload heights</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-reload disabled={disabled || dirty || view.blocked} onclick={onreload}>Reload source rows</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-restore="retain" disabled={disabled || dirty || view.blocked}
        onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore heights, retain audit</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-restore="prune" disabled={disabled || dirty || view.blocked}
        onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore heights, prune selected audit</button>
    {/if}
  </div>
  {#if view.review}
    {#each view.review as group, index (group.Query)}
      <section class="space-y-3" data-sivi-group={group.Form} aria-label={groups[index]}>
        <h3 class="font-semibold">{groups[index]}</h3>
        {#if group.Rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
        {#each group.Rows as row (row.rowId)}
          <article class="min-w-0 space-y-3 rounded border border-stone-200 p-3" data-sivi-row={row.rowId}>
            <header class="flex flex-wrap items-baseline justify-between gap-2">
              <h4 class="font-medium">{groups[index]}: {text(group, row, 'Species')}</h4>
              <span class="text-xs text-stone-500">Physical row {row.rowId}; ID {text(group, row, 'ID')}</span>
            </header>
            <dl class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6">
              {#each group.Columns.filter(column => coverLabels[column] && (view.extended || !['Cover5a', 'Cover5b', 'Cover5c'].includes(column))) as column}
                <div class="min-w-0"><dt class="text-xs text-stone-600">{coverLabels[column]}</dt><dd class="break-words">{text(group, row, column)}</dd></div>
              {/each}
              <div class="min-w-0"><dt class="text-xs text-stone-600">Collected (?)</dt><dd class="break-words">{text(group, row, 'Collected')}</dd></div>
            </dl>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {#each heights(index) as column}
                {@const draft = view.drafts[row.rowId]?.[column]}
                {@const original = row.cells[group.Columns.indexOf(column)]}
                {@const nullValue = draft?.nullValue ?? original.storage === 'null'}
                {@const raw = draft?.raw ?? metadataCellText(original)}
                {@const id = `sivi-${row.rowId}-${column}`}
                <div class="min-w-0">
                  <label for={id} class="mb-1 block text-sm font-medium">{heightLabels[column]}{column === 'HeightB' ? ' (literal text)' : ''}</label>
                  {#if column === 'HeightB'}
                    <textarea {id} rows="2" value={raw} class="w-full min-w-0 rounded border border-stone-300 px-3 py-2"
                      data-sivi-column={column} data-sivi-identity={row.rowId}
                      aria-label={`Ht B for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                      aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                      disabled={disabled || view.blocked || nullValue}
                      oninput={(event) => onstage(row.rowId, column, event.currentTarget.value, false)}></textarea>
                    <label class="mt-2 flex items-center gap-2 text-sm" for={`${id}-null`}>
                      <input id={`${id}-null`} type="checkbox" checked={nullValue} disabled={disabled || view.blocked}
                        aria-label={`Ht B NULL for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                        onchange={(event) => onstage(row.rowId, column, raw, event.currentTarget.checked)} />
                      NULL (distinct from empty text)
                    </label>
                  {:else}
                    <input {id} type="text" value={raw} class="w-full min-w-0 rounded border border-stone-300 px-3 py-2"
                      data-sivi-column={column} data-sivi-identity={row.rowId}
                      aria-label={`${heightLabels[column]} for ${text(group, row, 'Species')}, physical row ${row.rowId}`}
                      aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                      disabled={disabled || view.blocked}
                      oninput={(event) => onstage(row.rowId, column, event.currentTarget.value, false)} />
                  {/if}
                  {#if draft?.error}<p id={`${id}-error`} class="mt-1 text-sm text-red-700" role="alert">{draft.error}</p>{/if}
                </div>
              {/each}
            </div>
          </article>
        {/each}
      </section>
    {/each}
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load the original source rows before editing.</p>
  {/if}
  <SIVIChildSourceNotices notices={view.sourceNotices ?? []} saved={view.sourceNoticesSaved ?? false} />
  {#if view.extended}
    <p class="text-xs text-stone-600" data-sivi-extended-guidance>Extended shrub presentation shows B3/B4/B5 cover context; aggregate height editing remains available. Switching presentation never saves or resets drafts.</p>
  {/if}
  <p class="text-xs text-stone-600">Numeric heights may be cleared to NULL; Ht B retains literal text up to 255 UTF-16 units, including empty text. No height units, totals or metadata updates are inferred. Changing shrub presentation never saves or resets drafts.</p>
</section>
