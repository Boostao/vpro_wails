<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { SiteCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { QualityLookup, type PlotQualityChoice } from './qualityEditor';
  import { siteCodeKey, siteCodeKeys, siteCodeLabels, siteCodeList, siteCodeGroups, siteCodeDefinitions,
    siteCodeSuggestions, siteCodesChanged, siteCodeLengthError, siteCodeFieldError, siteCodeWarnings,
    siteCodeValidation, siteCodeAcknowledged, rememberSiteCodeAcknowledgement, type SiteCodes } from './siteCodeEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onvalidation,
    onbusy, children }: {
    draft: FS882Header; original: SiteCodes | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: 'siteCodes', message: string | null) => void;
    onbusy?: (busy: boolean) => void; children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_SITE_CODES_EDITING !== 'false';
  const listId = $props.id();
  const disturbanceLookup = new QualityLookup(async (force) => {
    if (force) await SiteCodeService.ReloadCatalogue();
    const rows = await SiteCodeService.ListSiteDisturbanceChoices();
    if (rows === null) throw new Error('Disturbance service did not return choice rows.');
    return rows;
  }, next => { disturbance = next; }, 'Disturbance');
  const exposureLookup = new QualityLookup(async (force) => {
    if (force) await SiteCodeService.ReloadCatalogue();
    const rows = await SiteCodeService.ListExposureChoices();
    if (rows === null) throw new Error('Exposure service did not return choice rows.');
    return rows;
  }, next => { exposure = next; }, 'Exposure');
  let disturbance = $state(disturbanceLookup.snapshot());
  let exposure = $state(exposureLookup.snapshot());
  const views = $derived({ SiteDisturbance: disturbance, Exposure: exposure });
  const lookupBusy = $derived(disturbance.busy || exposure.busy);
  let acknowledged = $state(false);
  const changed = $derived(siteCodesChanged(draft, original));
  const invalid = $derived(editingEnabled ? siteCodeLengthError(draft, original) : null);
  const warnings = $derived(siteCodeWarnings(draft, original, views));
  const validation = $derived(editingEnabled ? siteCodeValidation(draft, original, views, acknowledged) : null);

  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void disturbanceLookup.refresh(); void exposureLookup.refresh(); });
  });
  $effect(() => { acknowledged = siteCodeAcknowledged(draft, draft); });
  $effect(() => { onbusy?.(lookupBusy); });
  $effect(() => { const message = validation; untrack(() => onvalidation('siteCodes', message)); });
  onDestroy(() => { disturbanceLookup.dispose(); exposureLookup.dispose(); onbusy?.(false); });

  function setCode(key: typeof siteCodeKeys[number], raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    onchange();
  }
  function acknowledge(accepted: boolean): void {
    rememberSiteCodeAcknowledgement(draft, draft, accepted);
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
  <div class="site-code-toolbar" aria-label="Disturbance and exposure safety controls">
    {#if lookupBusy}<span role="status">Refreshing disturbance and exposure choices...</span>{/if}
    {#each [disturbance, exposure] as view, index}
      {#if view.error}
        <span role="alert">{view.error}</span>
        <button type="button" disabled={disabled || lookupBusy}
          onclick={() => void (index === 0 ? disturbanceLookup : exposureLookup).refresh(true)}>
          Retry {index === 0 ? 'disturbance' : 'exposure'} choices
        </button>
      {/if}
    {/each}
    {#if validation}<span role="alert">{validation}</span>{/if}
    {#if warnings.length}
      <ul aria-label="Disturbance code warnings">{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if changed && !lookupBusy && !invalid}
        <label><input id="site-code-acknowledgement" type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep the disturbance codes exactly as entered; I have reviewed these warnings.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(siteCodeInputs)}
{#if editingEnabled}
    <FieldGuidance title="Disturbance and exposure guidance and definitions">
    <p>Disturbance accepts raw manual codes (8 UTF-16 characters). Exposure requires a listed Item (2 characters).
      Choose a suggestion explicitly: prefixes and case are not silently changed. Descriptions and notes are metadata, not stored values.</p>
    {#each siteCodeKeys as key (key)}
      {@const list = siteCodeList(key)}
      {@const definitions = siteCodeDefinitions(views[list].choices, list, draft[key])}
      {@const suggestions = siteCodeSuggestions(views[list].choices, key, draft[key])}
      {#if suggestions.length}
        <details>
          <summary>Choose a full listed Item for {siteCodeLabels[key]}</summary>
          {#each suggestions as suggestion (suggestion.code)}
            <button type="button" disabled={disabled || lookupBusy || capabilities[key] !== true}
              onclick={() => setCode(key, suggestion.code)}>Use {suggestion.code} for {siteCodeLabels[key]}</button>
          {/each}
        </details>
      {/if}
      {#if definitions.length}
        <details>
          <summary>{siteCodeLabels[key]}: {definitions.length} reference definition{definitions.length === 1 ? '' : 's'}</summary>
          <ul>
            {#each definitions as definition (definition.rowId)}
              <li>{definition.description ?? '(description unavailable)'} | Note: {definition.note ?? 'NULL'}
                <details><summary>All source metadata</summary>{@render metadata(definition)}</details>
              </li>
            {/each}
          </ul>
        </details>
      {/if}
    {/each}
    <details class="site-code-reference">
      <summary>All frozen reference rows ({disturbance.choices.length + exposure.choices.length}); NULL and empty Items are not choices</summary>
      {#each [...disturbance.choices, ...exposure.choices] as definition (`${definition.listName}:${definition.rowId}`)}
        <details data-reference-row={definition.rowId} data-reference-list={definition.listName}>
          <summary>{definition.listName}: {definition.code === null ? 'NULL' : definition.code === '' ? '""' : definition.code}
            {definition.selectable ? '' : ` - ${definition.diagnostic}`}</summary>
          {@render metadata(definition)}
        </details>
      {/each}
    </details>
    </FieldGuidance>
  {#each ['SiteDisturbance', 'Exposure'] as list}
    {@const name = list === 'Exposure' ? 'Exposure' : 'SiteDisturbance'}
    <datalist id={`${listId}-${list}`}>
      {#each siteCodeGroups(views[name].choices, name) as group (group.code)}
        <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} source definitions`}</option>
      {/each}
    </datalist>
  {/each}
{/if}
{#snippet siteCodeInputs(only?: PaperControl)}
  {#each (only ? [only] : controls).filter(control => siteCodeKey(control.column) && control.type === 'ComboBox') as control (control.controlId)}
    {@const key = siteCodeKey(control.column)}
    {#if key}
      <input class="site-code-control" id={`header-${key}`} data-column={control.column} data-source-control={control.controlName}
        style={position(control)} aria-label={siteCodeLabels[key]} value={draft[key] ?? ''}
        aria-invalid={editingEnabled && siteCodeFieldError(key, draft[key], original?.[key] ?? null, views) !== null ? 'true' : undefined}
        list={editingEnabled ? `${listId}-${siteCodeList(key)}` : undefined}
        disabled={!editingEnabled || disabled || lookupBusy || capabilities[key] !== true || !control.enabled || control.locked}
        placeholder={capabilities[key] === true ? '' : 'Pending'}
        oninput={event => setCode(key, event.currentTarget.value)} />
    {/if}
  {/each}
{/snippet}
<style>
  .site-code-toolbar { display: grid; gap: .3rem; padding: .5rem; margin-bottom: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .site-code-toolbar ul { margin: 0; padding-left: 1.2rem; }
  .site-code-toolbar button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  .site-code-toolbar label { display: flex; align-items: center; gap: .35rem; }
  .site-code-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .site-code-control:disabled { color: #78716c; background: #f5f5f4; }
  .site-code-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .site-code-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
