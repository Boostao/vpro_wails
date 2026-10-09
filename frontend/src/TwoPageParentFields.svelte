<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { PaperControl } from './paperLayout';
  import { metadataCellText } from './projectMetadataEditor';
  import { twoPageParentCell, twoPageParentDirty, twoPageParentFields,
    type TwoPageParentForm, type TwoPageParentColumn, type TwoPageParentInput,
    type TwoPageParentDrafts } from './twoPageParentSession';
  import type { SIVIParentWriteView } from './siviParentWriteSession';

  type Field = ReturnType<typeof twoPageParentFields>[number];
  type Editor = { columns: readonly string[]; input: Snippet<[PaperControl, string]>; labelled: true };
  let { form, view, disabled = true, readDisabled = disabled, canSave = false, onstage, onoperation, oncancel, children }: {
    form: TwoPageParentForm; view: SIVIParentWriteView<TwoPageParentDrafts>;
    disabled?: boolean; readDisabled?: boolean; canSave?: boolean;
    onstage: (column: TwoPageParentColumn, input: TwoPageParentInput) => void;
    onoperation: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    oncancel: () => void;
    children?: Snippet<[Editor]>;
  } = $props();
  const fields = $derived(twoPageParentFields(form));
  const dirty = $derived(twoPageParentDirty(view.drafts));
  const pages = ['Site/Veg', 'Soil/Terrain'];
  function inputField(control: PaperControl): Field {
    const field = fields.find(field => field.column === control.column && field.controlId === control.controlId);
    if (!field) throw new Error('Additional-field input requires its exact source instance and owner.');
    return field;
  }
</script>

<section aria-label="Two-page additional-field editor" data-two-page-parent-panel={form}
  class="space-y-4 rounded border border-stone-200 bg-white p-4">
  <h3 class="font-semibold">{form} additional fields</h3>
  {#if view.error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}
    <p role="alert">Write outcome or refresh is unresolved. Undo / reload before continuing; never replay Save.</p>
  {/if}
  {#if view.busy}<p role="status">Additional-field operation: {view.operation}...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={readDisabled || view.busy || view.blocked || dirty}
      onclick={() => onoperation('load')}>Load additional fields</button>
    <button type="button" disabled={disabled || !canSave || view.busy || view.blocked}
      onclick={() => onoperation('save')}>Save additional fields</button>
    <button type="button" disabled={view.busy} onclick={() => onoperation('undo')}>Undo / reload</button>
    {#if view.historyId}
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty}
        onclick={() => onoperation('retain')}>Restore (retain audits)</button>
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty}
        onclick={() => onoperation('prune')}>Restore (prune audits)</button>
    {/if}
    {#if view.busy && view.operation === 'read'}
      <button type="button" onclick={oncancel}>Cancel additional-field read</button>
    {/if}
  </div>
  {#if view.original}
    {#if children}
      {@render children({ columns: fields.map(field => field.column), input, labelled: true })}
    {:else}
    {#each pages as page}
      {@const pageFields = fields.filter(field => field.page === `form:${form}/${page}`)}
      {#if pageFields.length}
        <section aria-label={page} class="space-y-2">
          <h4 class="font-medium">{page}</h4>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {#each pageFields as field (field.column)}
              {@render fieldInput(field)}
            {/each}
          </div>
        </section>
      {/if}
    {/each}
    {/if}
  {/if}
  <p class="text-xs text-stone-600">{fields.length} additional source-bound fields use typed audited storage.
    Explicit Save and restoration are desktop safety adaptations, not full two-page event parity.
    Existing parent fields, source actions and pictures are outside this panel.</p>
</section>

{#snippet input(control: PaperControl, _position: string)}
  {@render fieldInput(inputField(control))}
{/snippet}
{#snippet fieldInput(field: Field)}
  {#if view.original}
              {@const original = twoPageParentCell(view.original, field.column)}
              {@const draft = view.drafts[field.column]}
              {@const value = draft?.value ?? original}
              {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
              {@const id = `two-page-${field.column}`}
              <div class="min-w-0 space-y-1" data-two-page-field={field.column} data-source-control={field.controlId}>
                <label for={id} class="block text-sm font-medium">{field.label}</label>
                <input {id} type="text" class="w-full rounded border p-2" value={raw}
                  disabled={disabled || view.busy || view.blocked} aria-invalid={Boolean(draft?.error)}
                  aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })} />
                <div class="flex flex-wrap gap-2 text-xs">
                  <button type="button" disabled={disabled || view.busy || view.blocked}
                    onclick={() => onstage(field.column, { kind: 'clear' })}>Clear to NULL</button>
                  <button type="button" disabled={disabled || view.busy || view.blocked}
                    onclick={() => onstage(field.column, { kind: 'original' })}>Retain original</button>
                  <span data-storage={value.storage}>{value.storage === 'text' && value.text === '' ? 'empty text' : value.storage}</span>
                </div>
                {#if draft?.error}
                  <p id={`${id}-error`} role="alert" class="text-sm text-red-700">{draft.error}</p>
                {/if}
              </div>
  {/if}
{/snippet}
