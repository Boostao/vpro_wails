<script lang="ts">
  import { metadataCellText } from './projectMetadataEditor';
  import { siviParentSharedFieldsFor, siviParentSharedCell, siviParentSharedDirty, type SIVIParentSharedColumn,
    type SIVIParentSharedInput, type SIVIParentSharedView } from './siviParentSharedSession';
  let { view, disabled = true, canSave = false, onstage, onoperation, oncancel, onreferences }: {
    view: SIVIParentSharedView; disabled?: boolean; canSave?: boolean;
    onstage: (column: SIVIParentSharedColumn, input: SIVIParentSharedInput) => void;
    onoperation: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    oncancel: () => void;
    onreferences?: () => void;
  } = $props();
  const groups = [
    { label: 'Plot & survey', scope: 'plot' },
    { label: 'Site description', scope: 'site' },
    { label: 'Topography & stand', scope: 'topography' },
    { label: 'TERRAIN', scope: 'terrain' },
    { label: 'SOIL', scope: 'soils' },
    { label: 'Vegetation', scope: 'veg' },
    { label: 'Notes', scope: 'notes' },
  ];
  const dirty = $derived(siviParentSharedDirty(view.drafts));
  const fields = $derived(siviParentSharedFieldsFor(Boolean(view.referencesEnabled) &&
    import.meta.env.VITE_SIVI_PARENT_REFERENCE_EDITING === 'true'));
</script>

<section class="space-y-4 rounded border border-stone-200 bg-white p-4" aria-label="SIVI shared-field editor" data-sivi-shared-panel>
  <h3 class="font-semibold">SIVI / FS1333 shared fields</h3>
  {#if view.error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.referenceError}<p role="alert" data-sivi-reference-error>{view.referenceError}</p>{/if}
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
    {#if fields.length > 32 && onreferences}
      <button type="button" disabled={disabled || view.busy || view.blocked || !view.original}
        onclick={onreferences}>Reload reference choices</button>
    {/if}
  </div>
  {#if view.original}
    {#each groups.filter(group => fields.some(field => field.policy.scope === group.scope)) as group}
      <section aria-label={group.label} class="space-y-2">
        <h4 class="font-medium">{group.label}</h4>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each fields.filter(field => field.policy.scope === group.scope) as field (field.column)}
            {@const original = siviParentSharedCell(view.original, field.column)}
            {@const draft = view.drafts[field.column]}
            {@const value = draft?.value ?? original}
            {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
            {@const id = `sivi-shared-${field.column}`}
            {@const reference = view.references?.fields.find(row => row.column === field.column)}
            <div class="min-w-0 space-y-1" data-sivi-shared-field={field.column} data-source-control={field.controlId}>
              <label for={id} class="block text-sm font-medium">{field.label}</label>
              {#if field.policy.kind === 'memo'}
                <textarea {id} class="w-full rounded border p-2" value={raw}
                  disabled={disabled || view.busy || view.blocked} aria-invalid={Boolean(draft?.error)}
                  aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })}></textarea>
              {:else}
                <input {id} type="text" class="w-full rounded border p-2" value={raw}
                  list={field.policy.kind === 'reference' ? `${id}-options` : undefined}
                  disabled={disabled || view.busy || view.blocked} aria-invalid={Boolean(draft?.error)}
                  aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })} />
              {/if}
              {#if field.policy.kind === 'reference'}
                <datalist id={`${id}-options`}>
                  {#each reference?.available ? reference.choices.filter(row => row.selectable && row.code !== null) : [] as choice}
                    <option value={choice.code ?? ''} data-reference-row={choice.rowId}>{choice.description ?? '(NULL description)'}</option>
                  {/each}
                </datalist>
                {#if reference && !reference.available}
                  <p role="status" data-sivi-reference-unavailable>{reference.diagnostic}</p>
                {/if}
                {#if view.advisories?.[field.column]}
                  <p role="status" data-sivi-reference-advisory={field.column}>{view.advisories[field.column]}</p>
                {/if}
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
  <p class="text-xs text-stone-600">{fields.length} source-bound fields reuse storage validation and owned audited transactions.
    Explicit Save, Undo and restoration are desktop safety adaptations, not SIVI parent-event parity.
    ProjectID, source actions and XL callbacks are not part of this editor.</p>
  <p class="text-xs text-stone-600">Latitude and longitude use signed decimal degrees with changed-value limits of
    +/-90 and +/-180: a desktop safety adaptation. Source Fixed/six-decimal formatting is presentation evidence;
    editable and stored precision is not rounded. Yr. is an Admin integer, not a date or metadata update.
    Stand Age does not change Est./Meas. options.</p>
  <p class="text-xs text-stone-600">Date uses exact YYYY-MM-DD HH:MM:SS with an optional 1-9 digit fraction,
    years 0100-9999, and explicit NULL. This desktop wall-clock entry adaptation shows the full stored timestamp
    rather than source Medium Date formatting hiding time/precision. No timezone conversion, localized coercion,
    date-only completion, trimming or precision normalization is applied.</p>
  {#if fields.length > 32}
    <p class="text-xs text-stone-600">Reference suggestions preserve duplicate definitions. Only Exposure 1/2,
      Soil Drainage and Successional Status require exact listed Items; other reference fields retain physically
      valid free entry without acknowledgement. Changing BGC refreshes SubZone suggestions without clearing
      SubZone or changing Site Unit. Configured imported VLists ownership is not frozen DAO provenance.</p>
  {/if}
</section>
