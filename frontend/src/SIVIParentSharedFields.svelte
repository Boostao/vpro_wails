<script lang="ts">
  import { metadataCellText } from './projectMetadataEditor';
  import { siviParentSharedFields, siviParentSharedCell, siviParentSharedDirty, type SIVIParentSharedColumn,
    type SIVIParentSharedInput, type SIVIParentSharedView } from './siviParentSharedSession';
  let { view, disabled = true, canSave = false, onstage, onoperation, oncancel }: {
    view: SIVIParentSharedView; disabled?: boolean; canSave?: boolean;
    onstage: (column: SIVIParentSharedColumn, input: SIVIParentSharedInput) => void;
    onoperation: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    oncancel: () => void;
  } = $props();
  const groups = [
    { label: 'Site description', page: 'form:frmSIVIsite/Site' },
    { label: 'Vegetation', page: 'form:frmSIVIsite/Veg' },
  ];
  const dirty = $derived(siviParentSharedDirty(view.drafts));
</script>

<section class="space-y-4 rounded border border-stone-200 bg-white p-4" aria-label="SIVI shared-field editor" data-sivi-shared-panel>
  <h3 class="font-semibold">SIVI / FS1333 shared fields</h3>
  {#if view.error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}<p role="alert">Commit outcome or refresh is unresolved. Undo / reload before continuing; never replay Save.</p>{/if}
  {#if view.busy}<p role="status">Shared-field operation: {view.operation}...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={disabled || view.busy || view.blocked || dirty} onclick={() => onoperation('load')}>Load shared fields</button>
    <button type="button" disabled={disabled || !canSave || view.busy || view.blocked} onclick={() => onoperation('save')}>Save shared fields</button>
    <button type="button" disabled={view.busy} onclick={() => onoperation('undo')}>Undo / reload</button>
    {#if view.historyId}
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty} onclick={() => onoperation('retain')}>Restore (retain audits)</button>
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty} onclick={() => onoperation('prune')}>Restore (prune audits)</button>
    {/if}
    {#if view.busy && view.operation === 'read'}<button type="button" onclick={oncancel}>Cancel shared read</button>{/if}
  </div>
  {#if view.original}
    {#each groups as group}
      <section aria-label={group.label} class="space-y-2">
        <h4 class="font-medium">{group.label}</h4>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each siviParentSharedFields.filter(field => field.page === group.page) as field (field.column)}
            {@const original = siviParentSharedCell(view.original, field.column)}
            {@const draft = view.drafts[field.column]}
            {@const value = draft?.value ?? original}
            {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
            {@const id = `sivi-shared-${field.column}`}
            <div class="min-w-0 space-y-1" data-sivi-shared-field={field.column} data-source-control={field.controlId}>
              <label for={id} class="block text-sm font-medium">{field.label}</label>
              {#if field.policy.kind === 'memo'}
                <textarea {id} class="w-full rounded border p-2" value={raw}
                  disabled={disabled || view.busy || view.blocked} aria-invalid={Boolean(draft?.error)}
                  aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })}></textarea>
              {:else}
                <input {id} type="text" class="w-full rounded border p-2" value={raw}
                  disabled={disabled || view.busy || view.blocked} aria-invalid={Boolean(draft?.error)}
                  aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })} />
              {/if}
              <div class="flex gap-2 text-xs">
                <button type="button" disabled={disabled || view.busy || view.blocked}
                  onclick={() => onstage(field.column, { kind: 'clear' })}>Clear to NULL</button>
                <button type="button" disabled={disabled || view.busy || view.blocked}
                  onclick={() => onstage(field.column, { kind: 'original' })}>Retain original</button>
                <span data-storage={value.storage}>{value.storage === 'text' && value.text === '' ? 'empty text' : value.storage}</span>
              </div>
              {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
            </div>
          {/each}
        </div>
      </section>
    {/each}
  {/if}
  <p class="text-xs text-stone-600">Eight source-bound fields reuse FS882 storage validation and owned audited transactions.
    Explicit Save, Undo and restoration are desktop safety adaptations, not SIVI parent-event parity.
    ProjectID, source actions and XL callbacks are not part of this editor.</p>
</section>
