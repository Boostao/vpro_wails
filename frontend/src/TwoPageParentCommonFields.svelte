<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { PaperControl } from './paperLayout';
  import type { ListItem } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { twoPageParentCommonCell, twoPageParentCommonDirty, twoPageParentCommonFields,
    type TwoPageParentCommonColumn, type TwoPageParentCommonInput,
    type TwoPageParentCommonDrafts } from './twoPageParentCommonSession';
  import type { TwoPageParentForm } from './twoPageParentSession';
  import type { SIVIParentWriteView } from './siviParentWriteSession';

  type Field = ReturnType<typeof twoPageParentCommonFields>[number];
  type Editor = { columns: readonly string[]; input: Snippet<[PaperControl, string]>; labelled: true };
  let { form, view, choices, choicesBusy, choicesError, disabled = true, readDisabled = disabled,
    canSave = false, onstage, onoperation, oncancel, onchoices, children }: {
    form: TwoPageParentForm; view: SIVIParentWriteView<TwoPageParentCommonDrafts>;
    choices: ListItem[] | null; choicesBusy: boolean; choicesError: string | null;
    disabled?: boolean; readDisabled?: boolean; canSave?: boolean;
    onstage: (column: TwoPageParentCommonColumn, input: TwoPageParentCommonInput) => void;
    onoperation: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    oncancel: () => void; onchoices: () => void;
    children?: Snippet<[Editor]>;
  } = $props();
  const fields = $derived(twoPageParentCommonFields(form));
  const dirty = $derived(twoPageParentCommonDirty(view.drafts));
  function inputField(control: PaperControl): Field {
    const field = fields.find(field => field.column === control.column);
    if (!field || !field.sourceInstances.includes(control.controlId)) {
      throw new Error('Common-field input requires its exact source instance and owner.');
    }
    return field;
  }
</script>

<section aria-label="Two-page common-field editor" data-two-page-common-panel={form}
  class="space-y-4 rounded border border-stone-200 bg-white p-4">
  <h3 class="font-semibold">{form} common fields beyond XL</h3>
  {#if view.error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.blocked}<p role="alert">Write outcome or refresh is unresolved. Undo / reload before continuing; never replay Save.</p>{/if}
  {#if view.busy}<p role="status">Common-field operation: {view.operation}...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={readDisabled || view.busy || view.blocked || dirty}
      onclick={() => onoperation('load')}>Load common fields</button>
    <button type="button" disabled={disabled || !canSave || view.busy || view.blocked}
      onclick={() => onoperation('save')}>Save common fields</button>
    <button type="button" disabled={view.busy} onclick={() => onoperation('undo')}>Undo / reload</button>
    {#if view.historyId}
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty}
        onclick={() => onoperation('retain')}>Restore (retain audits)</button>
      <button type="button" disabled={disabled || !canSave || view.busy || view.blocked || dirty}
        onclick={() => onoperation('prune')}>Restore (prune audits)</button>
    {/if}
    {#if view.busy && view.operation === 'read'}
      <button type="button" onclick={oncancel}>Cancel common-field read</button>
    {/if}
  </div>
  {#if view.original}
    {#if children}
      {@render children({ columns: fields.map(field => field.column), input, labelled: true })}
    {:else}
    {#each ['Site/Veg', 'Soil/Terrain'] as page}
      <section aria-label={page} class="space-y-2">
        <h4 class="font-medium">{page}</h4>
        <div class="two-page-common-grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each fields.filter(field => field.page === `form:${form}/${page}`) as field (field.column)}
            {@render fieldInput(field)}
          {/each}
        </div>
      </section>
    {/each}
    {/if}
  {/if}
  <p class="text-xs text-stone-600">{fields.length} common source-bound fields use typed audited storage.
    PlotType choices are suggestions, not the SIVI five-option action. Literal TEXT10 remains editable when choices are unavailable.
    Explicit Save/restoration and a single age editor are desktop adaptations, not complete two-page event parity.</p>
</section>

{#snippet input(control: PaperControl, _position: string)}
  {@render fieldInput(inputField(control))}
{/snippet}
{#snippet fieldInput(field: Field)}
  {#if view.original}
            {@const draft = view.drafts[field.column]}
            {@const value = draft?.value ?? twoPageParentCommonCell(view.original, field.column)}
            {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
            {@const id = `two-page-common-${field.column}`}
            {@const unavailable = disabled || view.busy || view.blocked}
            <div class="min-w-0 space-y-1"
              class:group-start={field.column === 'SV_StandAgeEstMeas' || field.column === 'StrataCoverTotal'}
              class:height-estimation={field.column === 'SV_StandHeightEstMeas'}
              data-two-page-common-field={field.column} data-source-control={field.controlId}>
              <label for={id} class="block text-sm font-medium">{field.label}</label>
              {#if field.policy.kind === 'option'}
                {@const selected = value.storage === 'null' ? '' : value.storage === 'text' && (value.text === '1' || value.text === '2') ? value.text : 'historical'}
                <select {id} value={selected} class="w-full rounded border p-2" disabled={unavailable}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  onchange={event => onstage(field.column, { kind: 'option', option: event.currentTarget.value === '' ? null : Number(event.currentTarget.value) })}>
                  <option value="">NULL</option>
                  {#each field.options as option}<option value={String(option.value)}>{option.label}</option>{/each}
                  {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
                </select>
              {:else if field.policy.kind === 'boolean'}
                {@const selected = value.storage === 'null' ? '' : value.storage === 'integer' && value.integer === '-1' ? 'true' : value.storage === 'integer' && value.integer === '0' ? 'false' : 'historical'}
                <select {id} value={selected} class="w-full rounded border p-2" disabled={unavailable}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  onchange={event => onstage(field.column, event.currentTarget.value === '' ? { kind: 'clear' }
                    : event.currentTarget.value === 'true' || event.currentTarget.value === 'false'
                    ? { kind: 'boolean', value: event.currentTarget.value === 'true' } : { kind: 'text', raw: event.currentTarget.value })}>
                  <option value="">NULL</option><option value="true">Yes</option><option value="false">No</option>
                  {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
                </select>
              {:else}
                <input {id} type="text" value={raw} class="w-full rounded border p-2" disabled={unavailable}
                  list={field.column === 'PlotType' ? 'two-page-plot-type-choices' : undefined}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => onstage(field.column, { kind: 'text', raw: event.currentTarget.value })} />
                {#if field.column === 'PlotType'}
                  <datalist id="two-page-plot-type-choices">
                    {#each choices ?? [] as choice}<option value={choice.item}>{choice.itemDescription}</option>{/each}
                  </datalist>
                  {#if choicesError}<p role="alert" class="text-sm text-red-700">{choicesError}</p>{/if}
                  {#if choicesBusy}<p role="status">Loading PlotType choices...</p>{/if}
                  <button type="button" disabled={readDisabled || choicesBusy} onclick={onchoices}>Reload PlotType choices</button>
                {/if}
              {/if}
              <div class="flex flex-wrap gap-2 text-xs">
                <button type="button" disabled={unavailable} onclick={() => onstage(field.column, { kind: 'clear' })}>Clear to NULL</button>
                {#if field.policy.kind === 'categorical'}
                  <button type="button" disabled={unavailable} onclick={() => onstage(field.column, { kind: 'empty' })}>Set empty TEXT</button>
                {/if}
                <button type="button" disabled={unavailable} onclick={() => onstage(field.column, { kind: 'original' })}>Retain original</button>
                <span data-storage={value.storage}>{value.storage === 'text' && value.text === '' ? 'empty text' : value.storage}</span>
              </div>
              {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
              {#if field.sourceInstances.length > 1}
                <p class="text-xs text-stone-600">Both source Est/Meas controls bind Stand Age. One shared editor preserves that binding; it does not edit Stand Height.</p>
              {/if}
            </div>
  {/if}
{/snippet}

<style>
  .two-page-common-grid { display: grid; }
  @media (min-width: 640px) {
    .group-start { grid-column-start: 1; }
    .height-estimation { grid-column-start: 2; }
  }
  @media (min-width: 1024px) {
    .height-estimation { grid-column-start: 3; }
  }
</style>
