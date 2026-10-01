<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { RegionCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { QualityLookup, type PlotQualityChoice } from './qualityEditor';
  import { regionCodeKey, regionCodeKeys, regionCodeLabels, regionCodeList, regionCodeGroups,
    regionCodeDefinitions, regionCodeSuggestions, regionCodesChanged, regionCodeLengthError,
    regionCodeFieldError, regionCodeWarnings, regionCodeValidation, regionCodeAcknowledged,
    rememberRegionCodeAcknowledgement, type RegionCodes } from './regionCodeEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onvalidation,
    onbusy, children }: {
    draft: FS882Header; original: RegionCodes | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: 'regionCodes', message: string | null) => void;
    onbusy?: (busy: boolean) => void; children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_REGION_CODES_EDITING !== 'false';
  const listId = $props.id();
  const regionLookup = new QualityLookup(async () => {
    const rows = await RegionCodeService.ListRegionChoices();
    if (rows === null) throw new Error('Region service did not return choice rows.');
    return rows;
  }, next => { region = next; }, 'Region');
  const ecosectionLookup = new QualityLookup(async () => {
    const rows = await RegionCodeService.ListEcosectionChoices();
    if (rows === null) throw new Error('Ecosection service did not return choice rows.');
    return rows;
  }, next => { ecosection = next; }, 'Ecosection');
  let region = $state(regionLookup.snapshot());
  let ecosection = $state(ecosectionLookup.snapshot());
  const views = $derived({ Region: region, Ecosection: ecosection });
  const lookupBusy = $derived(region.busy || ecosection.busy);
  let acknowledged = $state(false);
  const changed = $derived(regionCodesChanged(draft, original));
  const invalid = $derived(editingEnabled ? regionCodeLengthError(draft, original) : null);
  const warnings = $derived(regionCodeWarnings(draft, original, views));
  const validation = $derived(editingEnabled ? regionCodeValidation(draft, original, views, acknowledged) : null);

  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void regionLookup.refresh(); void ecosectionLookup.refresh(); });
  });
  $effect(() => { acknowledged = regionCodeAcknowledged(draft, draft); });
  $effect(() => { onbusy?.(lookupBusy); });
  $effect(() => { const message = validation; untrack(() => onvalidation('regionCodes', message)); });
  onDestroy(() => { regionLookup.dispose(); ecosectionLookup.dispose(); onbusy?.(false); });

  function setCode(key: typeof regionCodeKeys[number], raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    onchange();
  }
  function acknowledge(accepted: boolean): void {
    rememberRegionCodeAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
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
  <div class="region-code-toolbar" aria-label="Region and ecosection safety controls">
    {#if lookupBusy}<span role="status">Refreshing region and ecosection choices...</span>{/if}
    {#each [region, ecosection] as view, index}
      {#if view.error}
        <span role="alert">{view.error}</span>
        <button type="button" disabled={disabled || lookupBusy}
          onclick={() => void (index === 0 ? regionLookup : ecosectionLookup).refresh()}>
          Retry {index === 0 ? 'region' : 'ecosection'} choices
        </button>
      {/if}
    {/each}
    {#if validation}<span role="alert">{validation}</span>{/if}
    {#if warnings.length}
      <ul aria-label="Region and ecosection code warnings">{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if changed && !lookupBusy && !invalid}
        <label><input id="region-code-acknowledgement" type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep these codes exactly as entered; I have reviewed the unmatched or unchecked values.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(regionCodeInputs)}
{#if editingEnabled}
    <FieldGuidance title="Region and ecosection guidance and definitions">
    <p>Region/district and ecosection accept independent raw manual codes (7 and 3 UTF-16 characters).
      Choose suggestions explicitly; no case rewriting, prefix completion or partner autofill occurs.
      Descriptions and notes are metadata, not stored values.</p>
    {#each regionCodeKeys as key (key)}
      {@const list = regionCodeList(key)}
      {@const definitions = regionCodeDefinitions(views[list].choices, list, draft[key])}
      {@const suggestions = regionCodeSuggestions(views[list].choices, key, draft[key])}
      {#if suggestions.length}
        <details>
          <summary>Choose a full listed Item for {regionCodeLabels[key]}</summary>
          {#each suggestions as suggestion (suggestion.code)}
            <button type="button" disabled={disabled || lookupBusy || capabilities[key] !== true}
              onclick={() => setCode(key, suggestion.code)}>Use {suggestion.code} for {regionCodeLabels[key]}</button>
          {/each}
        </details>
      {/if}
      {#if definitions.length}
        <details>
          <summary>{regionCodeLabels[key]}: {definitions.length} reference definition{definitions.length === 1 ? '' : 's'}</summary>
          {#each definitions as definition (definition.rowId)}
            <details><summary>{definition.description ?? '(description unavailable)'} | Note: {definition.note ?? 'NULL'}</summary>
              {@render metadata(definition)}
            </details>
          {/each}
        </details>
      {/if}
    {/each}
    <details class="region-code-reference">
      <summary>All frozen reference rows ({region.choices.length + ecosection.choices.length}); NULL and empty Items are not choices</summary>
      {#each [...region.choices, ...ecosection.choices] as definition (`${definition.listName}:${definition.rowId}`)}
        <details data-reference-row={definition.rowId} data-reference-list={definition.listName}>
          <summary>{definition.listName}: {definition.code === null ? 'NULL' : definition.code === '' ? '""' : definition.code}
            {definition.selectable ? '' : ` - ${definition.diagnostic}`}</summary>
          {@render metadata(definition)}
        </details>
      {/each}
    </details>
    </FieldGuidance>
  {#each ['Region', 'Ecosection'] as list}
    {@const name = list === 'Region' ? 'Region' : 'Ecosection'}
    <datalist id={`${listId}-${list}`}>
      {#each regionCodeGroups(views[name].choices, name) as group (group.code)}
        <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} source definitions`}</option>
      {/each}
    </datalist>
  {/each}
{/if}
{#snippet regionCodeInputs(only?: PaperControl)}
  {#each (only ? [only] : controls).filter(control => regionCodeKey(control.column) && control.type === 'ComboBox') as control (control.controlId)}
    {@const key = regionCodeKey(control.column)}
    {#if key}
      <input class="region-code-control" id={`header-${key}`} data-column={control.column} data-source-control={control.controlName}
        style={position(control)} aria-label={regionCodeLabels[key]} value={draft[key] ?? ''}
        aria-invalid={editingEnabled && regionCodeFieldError(key, draft[key], original?.[key] ?? null, views) !== null ? 'true' : undefined}
        list={editingEnabled ? `${listId}-${regionCodeList(key)}` : undefined}
        disabled={!editingEnabled || disabled || lookupBusy || capabilities[key] !== true || !control.enabled || control.locked}
        placeholder={capabilities[key] === true ? '' : 'Pending'}
        oninput={event => setCode(key, event.currentTarget.value)} />
    {/if}
  {/each}
{/snippet}
<style>
  .region-code-toolbar { display: grid; gap: .3rem; padding: .5rem; margin-bottom: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .region-code-toolbar ul { margin: 0; padding-left: 1.2rem; }
  .region-code-toolbar button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  .region-code-toolbar label { display: flex; align-items: center; gap: .35rem; }
  .region-code-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .region-code-control:disabled { color: #78716c; background: #f5f5f4; }
  .region-code-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .region-code-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
