<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { QualityService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { CatalogueLookup } from './catalogueLookup';
  import { qualityKey, qualityKeys, qualityLabels, qualityGroups, qualityDefinitions,
    qualityChanged, qualityLengthError, qualityValueError, qualityWarnings, qualityValidation, qualityAcknowledged,
    rememberQualityAcknowledgement, type QualityCodes } from './qualityEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onvalidation,
    onbusy, children }: {
    draft: FS882Header; original: QualityCodes | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onvalidation: (key: 'quality', message: string | null) => void;
    onbusy?: (busy: boolean) => void; children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_QUALITY_EDITING !== 'false';
  const listId = $props.id();
  const lookup = new CatalogueLookup(async (_force, requests) => {
    const rows = await requests.track(QualityService.ListPlotQualityChoices());
    if (rows === null) throw new Error('Quality service did not return choice rows.');
    return rows;
  }, next => { view = next; onbusy?.(next.busy); }, 'Quality');
  let view = $state(lookup.snapshot());
  let acknowledged = $state(false);
  const changed = $derived(qualityChanged(draft, original));
  const invalid = $derived(editingEnabled ? qualityLengthError(draft, original) : null);
  const warnings = $derived(qualityWarnings(draft, original, view));
  const groups = $derived(qualityGroups(view.choices));

  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { void lookup.refresh(); });
  });
  $effect(() => { acknowledged = qualityAcknowledged(draft, draft); });
  $effect(() => {
    const message = editingEnabled ? qualityValidation(draft, original, view, acknowledged) : null;
    untrack(() => onvalidation('quality', message));
  });
  onDestroy(() => { lookup.dispose(); onbusy?.(false); });

  function setCode(key: typeof qualityKeys[number], raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    onchange();
  }
  function acknowledge(accepted: boolean): void {
    rememberQualityAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
  }
</script>

{#if editingEnabled}
  <div class="quality-toolbar" aria-label="Data Quality safety controls">
    {#if view.busy}<span role="status">Refreshing quality choices...</span>{/if}
    {#if view.error}
      <span role="alert">{view.error}</span>
      <button type="button" disabled={disabled || view.busy} onclick={() => void lookup.refresh()}>Retry quality choices</button>
    {/if}
    {#if invalid}<span role="alert">{invalid}</span>{/if}
    {#if warnings.length}
      <ul aria-label="Quality code warnings">{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if changed && !view.busy && !invalid}
        <label><input id="quality-acknowledgement" type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep the quality codes exactly as entered; I have reviewed these warnings.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(qualityInputs)}
{#if editingEnabled}
    <FieldGuidance title="Data quality guidance and definitions">
    <p>Site, vegetation and soil use the same verified quality choices. NA is a stored code, not NULL; notes such as E/G/F/P are metadata, not replacement values.</p>
    {#each qualityKeys as key (key)}
      {@const definitions = qualityDefinitions(view.choices, draft[key])}
      {#if definitions.length}
        <details>
          <summary>{qualityLabels[key]}: {definitions.length} reference definition{definitions.length === 1 ? '' : 's'}</summary>
          <ul>
            {#each definitions as definition (definition.rowId)}
              <li>{definition.description ?? '(description unavailable)'} | Note: {definition.note ?? 'NULL'}
                <details><summary>All source metadata</summary><dl>
                  {#each Object.entries(definition) as [name, value]}
                    <dt>{name}</dt><dd>{value === null ? 'NULL' : value === '' ? '""' : String(value)}</dd>
                  {/each}
                </dl></details>
              </li>
            {/each}
          </ul>
        </details>
      {/if}
    {/each}
    </FieldGuidance>
  <datalist id={listId}>
    {#each groups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} source definitions`}</option>
    {/each}
  </datalist>
{/if}
{#snippet qualityInputs(only?: PaperControl)}
  {#each (only ? [only] : controls).filter(control => qualityKey(control.column) && ['ComboBox', 'TextBox'].includes(control.type)) as control (control.controlId)}
    {@const key = qualityKey(control.column)}
    {#if key}
      <input class="quality-control" id={`header-${key}`} data-column={control.column} data-source-control={control.controlName}
        style={position(control)} aria-label={qualityLabels[key]} value={draft[key] ?? ''}
        aria-invalid={editingEnabled && qualityValueError(key, draft[key], original?.[key] ?? null) !== null ? 'true' : undefined}
        list={editingEnabled ? listId : undefined}
        disabled={!editingEnabled || disabled || view.busy || capabilities[key] !== true || !control.enabled || control.locked}
        placeholder={capabilities[key] === true ? '' : 'Pending'}
        oninput={event => setCode(key, event.currentTarget.value)} />
    {/if}
  {/each}
{/snippet}
<style>
  .quality-toolbar { display: grid; gap: .3rem; padding: .5rem; margin-bottom: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .quality-toolbar ul { margin: 0; padding-left: 1.2rem; }
  .quality-toolbar button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  .quality-toolbar label { display: flex; align-items: center; gap: .35rem; }
  .quality-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .quality-control:disabled { color: #78716c; background: #f5f5f4; }
  .quality-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .quality-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
