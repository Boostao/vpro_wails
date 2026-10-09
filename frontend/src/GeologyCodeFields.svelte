<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { GeologyCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import { CatalogueLookup } from './catalogueLookup';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { geologyCodeKeys, geologyCodeColumns, geologyCodeKey, geologyCodeLabels, geologyCodeValueError,
    geologyCodeGroups, geologyCodeSelectable, geologyCodeWarnings, geologyCodeBusy, geologyCodeValidation,
    geologyCodeAcknowledged, rememberGeologyCodeAcknowledgement, type GeologyCodes, type GeologyCodeKey } from './geologyCodeEditor';

  let { draft = $bindable(), original, capabilities, disabled, onchange, onvalidation, onbusy, children }: {
    draft: FS882Header; original: GeologyCodes | null; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: string, message: string | null) => void;
    onbusy?: (busy: boolean) => void;
    children: Snippet<[{ columns: readonly string[]; input: Snippet<[PaperControl, string]> } | undefined]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_GEOLOGY_CODES_EDITING !== 'false';
  const lookup = new CatalogueLookup(async (force, requests) => {
    if (force) await requests.track(GeologyCodeService.ReloadCatalogue());
    const rows = await requests.track(GeologyCodeService.ListBedrockChoices());
    if (rows === null) throw new Error('Bedrock service did not return reference rows.');
    return rows;
  }, next => { view = next; }, 'Bedrock type');
  let view = $state(lookup.snapshot());
  let acknowledged = $state(false);
  const warnings = $derived(geologyCodeWarnings(draft, original, view));
  const validation = $derived(geologyCodeValidation(draft, original, view, acknowledged));
  const groups = $derived(geologyCodeGroups(view.choices, 'BedrockType'));

  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void lookup.refresh(); });
  });
  $effect(() => { acknowledged = geologyCodeAcknowledged(draft, draft); });
  $effect(() => { onbusy?.(editingEnabled && geologyCodeBusy(draft, original, view)); });
  $effect(() => { const message = editingEnabled ? validation : null; untrack(() => onvalidation('geologyCodes', message)); });
  onDestroy(() => {
    lookup.dispose();
    onbusy?.(false);
    if (editingEnabled) onvalidation('geologyCodes', geologyCodeValidation(draft, original,
      view.busy ? { ...view, busy: false, ready: false } : view, geologyCodeAcknowledged(draft, draft)));
  });
  function setCode(key: GeologyCodeKey, raw: string) {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    acknowledged = geologyCodeAcknowledged(draft, draft);
    onchange();
    onvalidation('geologyCodes', geologyCodeValidation(draft, original, view, acknowledged));
    onbusy?.(geologyCodeBusy(draft, original, view));
  }
  function acknowledge(accepted: boolean) {
    rememberGeologyCodeAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
    onvalidation('geologyCodes', geologyCodeValidation(draft, original, view, accepted));
  }
</script>

{#if editingEnabled}
  <div class="geology-code-toolbar" aria-label="Bedrock code safety controls">
    {#if view.busy}<p role="status">Refreshing bedrock choices...</p>{/if}
    {#if view.error}
      <p role="alert">{view.error}</p>
      <button type="button" disabled={disabled || view.busy} onclick={() => void lookup.refresh(true)}>Retry bedrock choices</button>
    {/if}
    {#if validation}<p role="alert">{validation}</p>{/if}
    {#if warnings.length}
      <ul>{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if !view.busy && geologyCodeKeys.every(key => geologyCodeValueError(key, draft[key], original?.[key] ?? null) === null)}
        <label><input type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep these bedrock codes exactly as entered; I have reviewed the unmatched or unchecked values.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(editingEnabled ? { columns: geologyCodeColumns, input } : undefined)}
{#if editingEnabled}
    <FieldGuidance title="Bedrock code guidance and definitions">
    <p>Independent bedrock codes, at most 4 UTF-16 characters. Select full Items explicitly;
      raw casing is preserved, without completion, truncation or partner autofill.</p>
    <details>
      <summary>All frozen bedrock reference rows ({view.choices.length}); NULL and empty Items are metadata</summary>
      {#each view.choices as definition (definition.rowId)}
        <details data-reference-row={definition.rowId} data-reference-list={definition.listName}>
          <summary>{definition.code === null ? 'NULL' : definition.code === '' ? '""' : definition.code}: {definition.description ?? 'NULL'}</summary>
          <dl>{#each Object.entries(definition) as [name, value]}
            <dt>{name}</dt><dd data-property={name} data-value={JSON.stringify(value)}
              data-kind={value === null ? 'null' : typeof value} data-empty={value === '' ? 'true' : undefined}
              data-negative-zero={typeof value === 'number' && Object.is(value, -0) ? 'true' : undefined}>
              {value === null ? 'NULL' : value === '' ? '""' : String(value)}
            </dd>
          {/each}</dl>
          {#if geologyCodeSelectable(definition, 'BedrockType')}
            {#each geologyCodeKeys as key}
              <button type="button" disabled={disabled || view.busy || capabilities[key] !== true}
                onclick={() => setCode(key, definition.code!)}>Use {definition.code} for {geologyCodeLabels[key]}</button>
            {/each}
          {/if}
        </details>
      {/each}
    </details>
    {#each geologyCodeKeys as key}
      <details><summary>Choose a full listed Item for {geologyCodeLabels[key]}</summary>
        {#each groups as group (group.code)}
          <button type="button" disabled={disabled || view.busy || capabilities[key] !== true}
            onclick={() => setCode(key, group.code)}>{group.code}: {group.records[0].description ?? 'NULL'}</button>
        {/each}
      </details>
    {/each}
    </FieldGuidance>
{/if}
{#snippet input(control: PaperControl, position: string)}
  {@const key = geologyCodeKey(control.column)}
  {#if key && control.type === 'ComboBox'}
    <input class="geology-code-control" id={`header-${key}`} data-column={control.column}
      data-source-control={control.controlName} style={position} aria-label={geologyCodeLabels[key]}
      value={draft[key] ?? ''} disabled={disabled || capabilities[key] !== true}
      placeholder={capabilities[key] === true ? '' : 'Pending'} autocomplete="off"
      aria-invalid={geologyCodeValueError(key, draft[key], original?.[key] ?? null) !== null}
      oninput={event => setCode(key, event.currentTarget.value)} />
  {/if}
{/snippet}
<style>
  .geology-code-toolbar { display: grid; gap: .3rem; margin-bottom: .5rem; padding: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  p { margin: 0; }
  button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  dl { display: grid; grid-template-columns: max-content 1fr; gap: .1rem .6rem; }
  dd { margin: 0; overflow-wrap: anywhere; }
  .geology-code-control { border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; color: #065f46; font: inherit; line-height: 1.4; }
  .geology-code-control:disabled { color: #78716c; background: #f5f5f4; }
  .geology-code-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
