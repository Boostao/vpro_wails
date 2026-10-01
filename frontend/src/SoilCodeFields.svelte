<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { SoilCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { QualityLookup, type PlotQualityChoice } from './qualityEditor';
  import { soilCodeKey, soilCodeKeys, soilCodeColumns, soilCodeLabels, soilCodeList, soilCodeValueError,
    soilCodeDefinitions, soilCodeSuggestions, soilCodeLengthError, soilCodeBusy, soilCodeWarnings,
    soilCodeValidation, soilCodeAcknowledged, rememberSoilCodeAcknowledgement,
    soilCodeSelectable, type SoilCodes } from './soilCodeEditor';

  let { draft = $bindable(), original, capabilities, disabled, onchange, onvalidation, onbusy, children }: {
    draft: FS882Header; original: SoilCodes | null; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: string, message: string | null) => void;
    onbusy?: (busy: boolean) => void;
    children: Snippet<[{ columns: readonly string[]; input: Snippet<[PaperControl, string]> } | undefined]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_SOIL_CODES_EDITING !== 'false';
  const groupLookup = new QualityLookup(async (force) => {
    if (force) await SoilCodeService.ReloadCatalogue();
    const rows = await SoilCodeService.ListGreatGroupChoices();
    if (rows === null) throw new Error('Soil great-group service did not return reference rows.');
    return rows;
  }, next => { group = next; }, 'Soil great group');
  const subgroupLookup = new QualityLookup(async (force) => {
    if (force) await SoilCodeService.ReloadCatalogue();
    const rows = await SoilCodeService.ListSubgroupChoices();
    if (rows === null) throw new Error('Soil subgroup service did not return reference rows.');
    return rows;
  }, next => { subgroup = next; }, 'Soil subgroup');
  let group = $state(groupLookup.snapshot());
  let subgroup = $state(subgroupLookup.snapshot());
  const views = $derived({ SoilClassGroup: group, SoilClassSubgroup: subgroup });
  const lookupBusy = $derived(group.busy || subgroup.busy);
  let acknowledged = $state(false);
  const invalid = $derived(soilCodeLengthError(draft, original));
  const warnings = $derived(soilCodeWarnings(draft, original, views));
  const validation = $derived(soilCodeValidation(draft, original, views, acknowledged));

  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void groupLookup.refresh(); void subgroupLookup.refresh(); });
  });
  $effect(() => { acknowledged = soilCodeAcknowledged(draft, draft); });
  $effect(() => { onbusy?.(editingEnabled && soilCodeBusy(draft, original, views)); });
  $effect(() => { const message = editingEnabled ? validation : null; untrack(() => onvalidation('soilCodes', message)); });
  onDestroy(() => {
    groupLookup.dispose(); subgroupLookup.dispose();
    onbusy?.(false);
    if (editingEnabled) {
      // A pending request cannot validate a hidden field after disposal.
      const hiddenViews = {
        SoilClassGroup: group.busy ? { ...group, busy: false, ready: false } : group,
        SoilClassSubgroup: subgroup.busy ? { ...subgroup, busy: false, ready: false } : subgroup
      };
      onvalidation('soilCodes', soilCodeValidation(draft, original, hiddenViews, soilCodeAcknowledged(draft, draft)));
    }
  });
  function setCode(key: typeof soilCodeKeys[number], raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    acknowledged = soilCodeAcknowledged(draft, draft);
    onchange();
    onvalidation('soilCodes', soilCodeValidation(draft, original, views, acknowledged));
    onbusy?.(soilCodeBusy(draft, original, views));
  }
  function acknowledge(accepted: boolean): void {
    rememberSoilCodeAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
    onvalidation('soilCodes', soilCodeValidation(draft, original, views, accepted));
  }
</script>

{#snippet metadata(definition: PlotQualityChoice)}
  <dl>
    {#each Object.entries(definition) as [name, value]}
      <dt>{name}</dt><dd data-property={name} data-value={JSON.stringify(value)} data-kind={value === null ? 'null' : typeof value}
        data-empty={value === '' ? 'true' : undefined}
        data-negative-zero={typeof value === 'number' && Object.is(value, -0) ? 'true' : undefined}>
        {value === null ? 'NULL' : value === '' ? '""' : String(value)}
      </dd>
    {/each}
  </dl>
{/snippet}
{#if editingEnabled}
  <div class="soil-code-toolbar" aria-label="Soil classification safety controls">
    {#if lookupBusy}<p role="status">Refreshing soil classification choices...</p>{/if}
    {#each [group, subgroup] as view, index}
      {#if view.error}
        <p role="alert">{view.error}</p>
        <button type="button" disabled={disabled || view.busy}
          onclick={() => void (index === 0 ? groupLookup : subgroupLookup).refresh(true)}>
          Retry {index === 0 ? 'great group' : 'subgroup'} choices
        </button>
      {/if}
    {/each}
    {#if validation}<p role="alert">{validation}</p>{/if}
    {#if warnings.length}
      <ul aria-label="Soil classification code warnings">{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if !lookupBusy && !invalid}
        <label><input type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep these codes exactly as entered; I have reviewed the unmatched or unchecked values.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(editingEnabled ? { columns: soilCodeColumns, input: soilCodeInput } : undefined)}
{#if editingEnabled}
    <FieldGuidance title="Soil classification guidance and definitions">
    <p>Experimental parent editor: independent raw codes, at most 4 UTF-16 characters.
      Choose full Items explicitly; no case rewriting, automatic completion, truncation or partner autofill.
      Descriptions and empty Items are metadata, not selections.</p>
    {#each soilCodeKeys as key (key)}
      {@const list = soilCodeList(key)}
      {@const definitions = soilCodeDefinitions(views[list].choices, list, draft[key])}
      {@const suggestions = soilCodeSuggestions(views[list].choices, key, draft[key])}
      {#if suggestions.length}
        <details>
          <summary>Choose a full listed Item for {soilCodeLabels[key]}</summary>
          {#each suggestions as suggestion (suggestion.code)}
            <button type="button" disabled={disabled || views[list].busy || capabilities[key] !== true}
              onclick={() => setCode(key, suggestion.code)}>Use {suggestion.code} for {soilCodeLabels[key]}</button>
          {/each}
        </details>
      {/if}
      {#if definitions.length}
        <details>
          <summary>{soilCodeLabels[key]}: {definitions.length} reference definitions</summary>
          {#each definitions as definition (definition.rowId)}
            <details><summary>{definition.description ?? '(description unavailable)'} | Note: {definition.note ?? 'NULL'}</summary>
              {@render metadata(definition)}
            </details>
          {/each}
        </details>
      {/if}
    {/each}
    <details>
      <summary>All frozen reference rows ({group.choices.length + subgroup.choices.length}); NULL and empty Items are not choices</summary>
      {#each [group, subgroup] as view, index}
        {@const list = index === 0 ? 'SoilClassGroup' : 'SoilClassSubgroup'}
        {#each view.choices as definition (definition.rowId)}
          <details data-reference-row={definition.rowId} data-reference-list={definition.listName}>
            <summary>{definition.listName}: {definition.code === null ? 'NULL' : definition.code === '' ? '""' : definition.code}
              {definition.selectable ? '' : ` - ${definition.diagnostic}`}</summary>
            {@render metadata(definition)}
            {#if soilCodeSelectable(definition, list)}
              {@const key = index === 0 ? 'soilClassGroup' : 'soilClassSubGroup'}
              <button type="button" disabled={disabled || view.busy || capabilities[key] !== true}
                onclick={() => setCode(key, definition.code!)}>Use {definition.code} for {soilCodeLabels[key]}</button>
            {/if}
          </details>
        {/each}
      {/each}
    </details>
    </FieldGuidance>
{/if}
{#snippet soilCodeInput(control: PaperControl, position: string)}
  {@const key = soilCodeKey(control.column)}
  {#if key && control.type === 'ComboBox'}
    <input class="soil-code-control" id={`header-${key}`} data-column={control.column} data-source-control={control.controlName}
      style={position} aria-label={soilCodeLabels[key]} value={draft[key] ?? ''}
      disabled={disabled || capabilities[key] !== true} placeholder={capabilities[key] === true ? '' : 'Pending'}
      aria-invalid={soilCodeValueError(key, draft[key], original?.[key] ?? null) !== null}
      autocomplete="off" oninput={event => setCode(key, event.currentTarget.value)} />
  {/if}
{/snippet}
<style>
  .soil-code-toolbar { display: grid; gap: .3rem; margin-bottom: .5rem; padding: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .soil-code-toolbar p { margin: 0; }
  button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  dl { display: grid; grid-template-columns: max-content 1fr; gap: .1rem .6rem; }
  dd { margin: 0; overflow-wrap: anywhere; }
  .soil-code-control { border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; color: #065f46; font: inherit; line-height: 1.4; }
  .soil-code-control:disabled { color: #78716c; background: #f5f5f4; }
  .soil-code-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
