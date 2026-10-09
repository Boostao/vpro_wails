<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { ParentCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import { CatalogueLookup } from './catalogueLookup';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { drainageError, drainageGroups } from './drainageEditor';
  let { draft = $bindable(), original, capabilities, disabled, onchange, onvalidation, onbusy, children }: {
    draft: FS882Header; original: FS882Header | null; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: string, message: string | null) => void;
    onbusy: (busy: boolean) => void;
    children: Snippet<[{ columns: readonly string[]; input: Snippet<[PaperControl, string]> } | undefined]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_SOIL_DRAINAGE_EDITING !== 'false';
  const lookup = new CatalogueLookup(async (force, requests) => {
    if (force) await requests.track(ParentCodeService.ReloadCatalogue());
    const rows = await requests.track(ParentCodeService.ListChoices('SoilDrainage'));
    if (rows === null) throw new Error('Soil drainage service did not return reference rows.');
    return rows;
  }, next => { view = next; }, 'Soil drainage');
  let view = $state(lookup.snapshot());
  const changed = $derived(draft.soilDrainage !== null && draft.soilDrainage !== (original?.soilDrainage ?? null));
  const validation = $derived(drainageError(draft.soilDrainage, original?.soilDrainage ?? null, view));
  const groups = $derived(drainageGroups(view.choices, 'SoilDrainage'));
  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void lookup.refresh(); });
  });
  $effect(() => { onbusy(editingEnabled && changed && view.busy); });
  $effect(() => { const message = editingEnabled ? validation : null; untrack(() => onvalidation('soilDrainage', message)); });
  onDestroy(() => {
    lookup.dispose(); onbusy(false);
    if (editingEnabled) onvalidation('soilDrainage', drainageError(draft.soilDrainage,
      original?.soilDrainage ?? null, view.busy ? { ...view, busy: false, ready: false } : view));
  });
  function setCode(raw: string) {
    const value = raw === '' ? null : raw;
    if (draft.soilDrainage === value) return;
    draft.soilDrainage = value; onchange();
    onvalidation('soilDrainage', drainageError(value, original?.soilDrainage ?? null, view));
  }
</script>
{#if editingEnabled}
  <div class="drainage-toolbar">
    {#if view.busy}<p role="status">Refreshing soil drainage choices...</p>{/if}
    {#if view.error}
      <p role="alert">{view.error}</p>
      <button type="button" disabled={disabled || view.busy} onclick={() => void lookup.refresh(true)}>Retry soil drainage choices</button>
    {/if}
    {#if validation}<p role="alert">{validation}</p>{/if}
  </div>
{/if}
{@render children(editingEnabled ? { columns: ['SoilDrainage'], input } : undefined)}
{#if editingEnabled}
  <FieldGuidance title="Soil drainage choices and definitions">
    <p>Source LimitToList is enforced. Select a complete canonical Item or clear to NULL.
      Stored historical codes are preserved unchanged; new unmatched or unchecked codes cannot be acknowledged into validity.</p>
    {#each groups as group (group.code)}
      <button type="button" disabled={disabled || view.busy || !view.ready || capabilities.soilDrainage !== true}
        data-choice-for="soilDrainage" data-choice-code={group.code} onclick={() => setCode(group.code)}>
        {group.code}: {group.records[0].description ?? 'NULL'}
      </button>
    {/each}
    <details><summary>All frozen drainage rows ({view.choices.length}); empty and NULL Items remain metadata</summary>
      {#each view.choices as row (row.rowId)}
        <details data-reference-row={row.rowId} data-reference-list={row.listName}>
          <summary>{row.code === null ? 'NULL' : row.code === '' ? '""' : row.code}: {row.description ?? 'NULL'}</summary>
          <dl>{#each Object.entries(row) as [name, value]}<dt>{name}</dt><dd>{value === null ? 'NULL' : value === '' ? '""' : String(value)}</dd>{/each}</dl>
        </details>
      {/each}
    </details>
  </FieldGuidance>
{/if}
{#snippet input(control: PaperControl, position: string)}
  {#if control.column === 'SoilDrainage' && control.type === 'ComboBox'}
    <input id="header-soilDrainage" data-column="SoilDrainage" data-source-control={control.controlName}
      style={position} aria-label="Soil drainage" autocomplete="off" value={draft.soilDrainage ?? ''}
      disabled={disabled || capabilities.soilDrainage !== true || !control.enabled || control.locked}
      aria-invalid={validation !== null} oninput={event => setCode(event.currentTarget.value)} />
  {/if}
{/snippet}
<style>
  input { border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; }
  input:disabled { color: #78716c; background: #f5f5f4; }
  input:focus { outline: 2px solid #047857; outline-offset: 1px; }
  input[aria-invalid="true"] { border-color: #b91c1c; }
  .drainage-toolbar { font-size: .75rem; }
  p[role="alert"] { color: #b91c1c; }
</style>
