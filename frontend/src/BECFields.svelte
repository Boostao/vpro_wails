<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { BECService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import { ReadRequests } from './readRequests';
  import { BECLookup, becKey, becLabels, becLengths, becGroups, becChanged, becWarnings,
    becValidation, becAcknowledged, rememberBECAcknowledgement, type BECKey, type BECCodes } from './becEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onvalidation, children }: {
    draft: FS882Header; original: BECCodes | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: 'bec', message: string | null) => void;
    children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_BEC_EDITING !== 'false';
  const listPrefix = $props.id();
  function catalogueRows<T>(rows: T[] | null): T[] {
    if (rows === null) throw new Error('The BEC service did not return catalogue rows.');
    return rows;
  }
  const reads = new ReadRequests();
  const lookup = new BECLookup({
    zones: async () => catalogueRows(await reads.track(BECService.ListBECZones())),
    subZones: async zone => catalogueRows(await reads.track(BECService.ListBECSubZones(zone))),
    siteSeries: async (zone, subZone) => catalogueRows(await reads.track(BECService.ListBECSiteSeries(zone, subZone)))
  }, next => view = next, () => reads.cancelAll());
  let view = $state(lookup.snapshot());
  let acknowledged = $state(false);
  const changed = $derived(becChanged(draft, original));
  const warnings = $derived(becWarnings(draft, view));
  const zoneGroups = $derived(becGroups(view.zones, row => row.zone));
  const subZoneGroups = $derived(becGroups(view.subZones, row => row.subZone));
  const seriesGroups = $derived(becGroups(view.siteSeries, row => row.selectable === false ? null : row.siteSeries));
  const unselectableRows = $derived(view.siteSeries.filter(row => row.selectable === false || row.siteSeries === null || row.siteSeries === '').length);
  const definitions = $derived(view.siteSeries.filter(row =>
    row.siteSeries !== null && draft.siteSeries !== null && row.siteSeries.toUpperCase() === draft.siteSeries.toUpperCase()));

  $effect(() => {
    const zone = draft.zone, subZone = draft.subZone;
    if (editingEnabled) untrack(() => { void lookup.refresh(zone, subZone); });
  });
  $effect(() => {
    const identity = draft;
    const codes = { zone: draft.zone, subZone: draft.subZone, siteSeries: draft.siteSeries };
    acknowledged = becAcknowledged(identity, codes);
  });
  $effect(() => {
    const message = editingEnabled ? becValidation(draft, original, view, acknowledged) : null;
    untrack(() => onvalidation('bec', message));
  });
  onDestroy(() => lookup.dispose());

  function unavailable(key: BECKey, control: PaperControl): boolean {
    return disabled || capabilities[key] !== true || !control.enabled || control.locked ||
      (!editingEnabled && key !== 'siteSeries');
  }
  function setCode(key: BECKey, value: string): void {
    draft[key] = value === '' ? null : value;
    acknowledged = becAcknowledged(draft, draft);
    onchange();
  }
  function acknowledge(accepted: boolean): void {
    rememberBECAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
  }
</script>

{#if editingEnabled}
  <div class="bec-toolbar" aria-label="BEC classification safety controls">
    <span>Zone changes preserve existing subzone and site-series codes. Catalogue variant/phase details are not separately stored.</span>
    {#if view.busy}<span role="status">Refreshing BEC choices...</span>{/if}
    {#if unselectableRows > 0}
      <span>{unselectableRows} matching catalogue definitions have no selectable code; their metadata is retained, not offered as blank choices.</span>
    {/if}
    {#if view.error}
      <span role="alert">{view.error}</span>
      <button type="button" disabled={disabled || view.busy} onclick={() => void lookup.refresh(draft.zone, draft.subZone)}>Retry BEC choices</button>
    {/if}
    {#if warnings.length > 0}
      <ul class="bec-warnings" aria-label="BEC classification warnings">
        {#each warnings as warning}<li>{warning}</li>{/each}
      </ul>
      {#if changed && !view.busy}
        <label class="bec-acknowledgement">
          <input id="bec-acknowledgement" type="checkbox" checked={acknowledged} disabled={disabled}
            onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep the unmatched classification codes exactly as entered; I have reviewed these warnings.
        </label>
      {/if}
    {/if}
    {#if definitions.length > 0}
      <details class="bec-definitions">
        <summary>{definitions.length} catalogue definition{definitions.length === 1 ? '' : 's'} for site series {draft.siteSeries}
          {definitions.length > 1 ? ' - stored code does not identify one definition' : ''}</summary>
        <ul>
          {#each definitions as definition (definition.rowId)}
            <li>
              {definition.description ?? '(description unavailable)'}
              | Region: {definition.region ?? 'NULL'} | Variant: {definition.variant ?? 'NULL'} | Phase: {definition.phase ?? 'NULL'}
              | Source: {definition.sourceId ?? 'NULL'} | Catalogue row: {definition.rowId}
              <details>
                <summary>All source metadata</summary>
                <dl>
                  {#each Object.entries(definition) as [key, value]}
                    <dt>{key}</dt><dd>{value === null ? 'NULL' : value === '' ? '""' : String(value)}</dd>
                  {/each}
                </dl>
              </details>
            </li>
          {/each}
        </ul>
      </details>
    {/if}
  </div>
  <datalist id={`${listPrefix}-zone`}>
    {#each zoneGroups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} catalogue definitions`}</option>
    {/each}
  </datalist>
  <datalist id={`${listPrefix}-subZone`}>
    {#each subZoneGroups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? `${group.records[0].zone}: ${group.records[0].description ?? ''}` : `${group.records.length} catalogue definitions; zone must be entered separately`}</option>
    {/each}
  </datalist>
  <datalist id={`${listPrefix}-siteSeries`}>
    {#each seriesGroups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} catalogue definitions; see details after choosing this code`}</option>
    {/each}
  </datalist>
{/if}

{#snippet becInputs(only?: PaperControl)}
  {#each only ? [only] : controls as control (control.controlId)}
    {@const key = becKey(control.column)}
    {#if key && (control.type === 'TextBox' || control.type === 'ComboBox')}
      <input class="bec-control" style={position(control)} id={`header-${key}`} data-source-control={control.controlName}
        data-column={control.column} aria-label={becLabels[key]} value={draft[key] ?? ''}
        disabled={unavailable(key, control)} maxlength={editingEnabled ? becLengths[key] : undefined}
        list={editingEnabled ? `${listPrefix}-${key}` : undefined}
        placeholder={capabilities[key] !== true ? 'Pending' : ''}
        title={capabilities[key] !== true ? `${becLabels[key]} is unsupported by the active schema`
          : !editingEnabled && key !== 'siteSeries' ? 'BEC editing has not been enabled by application configuration'
          : 'Nullable text code; choosing a suggestion does not clear other classification fields'}
        oninput={event => { if (!unavailable(key, control)) setCode(key, event.currentTarget.value); }} />
    {/if}
  {/each}
{/snippet}
{@render children(becInputs)}

<style>
  .bec-toolbar { display: flex; flex-direction: column; gap: .25rem; margin-bottom: .5rem; font-size: .75rem; color: #57534e; }
  .bec-toolbar [role="alert"] { color: #b91c1c; }
  .bec-toolbar button { align-self: flex-start; border: 1px solid #a8a29e; border-radius: 2px; background: white; padding: 0 .25rem; }
  .bec-warnings { padding-left: 1rem; color: #92400e; }
  .bec-acknowledgement { display: flex; align-items: center; gap: .35rem; }
  .bec-definitions ul { padding-left: 1rem; }
  .bec-definitions dl { display: grid; grid-template-columns: max-content 1fr; gap: .1rem .5rem; }
  .bec-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .bec-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .bec-control:disabled { color: #78716c; background: #f5f5f4; }
</style>
