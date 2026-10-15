<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationQualityMinimumPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationQualityMinimumFields, type LongVegetationQualityMinimumPreferencesSession,
    type LongVegetationQualityMinimumPreferenceValues } from './longVegetationQualityMinimumPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationQualityMinimumPreferencesSession;
    operationHeld?: boolean;
    mutationHeld?: boolean;
    onReadBusyChange: (busy: boolean) => void;
  } = $props();
  let view = $state(untrack(() => preferences.view()));
  let reference = $state.raw(untrack(() => preferences.reference()));
  let busy = $state(false);
  let error = $state('');
  const reads = new ReadRequests();
  let generation = 0;
  const rows = $derived(reference?.definitions.rows ?? []);
  const unsubscribe = untrack(() => preferences.subscribe(() => {
    view = preferences.view(); reference = preferences.reference();
  }));
  function cancel() {
    generation++; reads.cancelAll(); preferences.cancelLoad(); busy = false; onReadBusyChange(false);
  }
  async function load() {
    if (busy || operationHeld || view.busy || view.blocked) return;
    cancel(); const request = generation, token = preferences.beginLoad();
    error = ''; busy = true; onReadBusyChange(true);
    try {
      const value = await reads.track(LongVegetationQualityMinimumPreferencesService.GetLongVegetationQualityMinimumPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved quality minimum strings unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationQualityMinimumPreferencesService.SaveLongVegetationQualityMinimumPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(key: keyof LongVegetationQualityMinimumPreferenceValues, value: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editMinimum(key, value);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="quality-minima" aria-label="Saved Long Vegetation Data Quality minima" data-long-vegetation-quality-minima>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving quality minimum strings; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-quality-minimum-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <fieldset>
    <legend>Data Quality - Minimum</legend>
    <div class="fields">
      {#each longVegetationQualityMinimumFields as field}
        <label for={'long-vegetation-quality-minimum-' + field.key}>
          <span>{field.label}</span>
          <input id={'long-vegetation-quality-minimum-' + field.key} type="text"
            list="long-vegetation-quality-minimum-suggestions" data-source-control={field.control}
            value={view.draft ? view.draft[field.key] : ''} aria-invalid={!!view.draftError}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft}
            oninput={event => { const value = event.currentTarget.value; edit(field.key, value); }} />
        </label>
      {/each}
    </div>
    <datalist id="long-vegetation-quality-minimum-suggestions">
      {#each rows as row (row.rowId)}
        {#if row.cells && row.cells[3]?.storage === 'text' && typeof row.cells[3].text === 'string' && !row.cells[3].text.includes('\0')}
          <option value={row.cells[3].text}>{row.cells[4]?.text ?? ''}</option>
        {/if}
      {/each}
    </datalist>
  </fieldset>
  <div class="actions">
    <button data-long-vegetation-quality-minimum-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved quality minima</button>
    <button data-long-vegetation-quality-minimum-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save quality minima</button>
    <button data-long-vegetation-quality-minimum-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo quality minimum draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-quality-minimum-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge quality minimum outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel quality minimum read</button><p role="status">Reading owned quality minima and suggestions...</p>{/if}
  </div>
  {#if reference}
    <details data-long-vegetation-quality-minimum-definitions>
      <summary>Owned PlotQualitySite suggestions ({rows.length} physical rows)</summary>
      <p>{reference.source}</p>
      {#if rows.length === 0}<p>No suggestions; literal editing remains available.</p>{/if}
      <ul>
        {#each rows as row (row.rowId)}
          <li>Row {row.rowId}: Item {row.cells?.[3]?.storage} {JSON.stringify(row.cells?.[3]?.text)};
            description {row.cells?.[4]?.storage} {JSON.stringify(row.cells?.[4]?.text)}.</li>
        {/each}
      </ul>
    </details>
  {/if}
  <p class="guidance">Load saved strings before editing; unloaded disabled inputs do not represent saved values.
    All three controls suggest the same owned PlotQualitySite items, not active DataQuality ranks.
    Suggestions are optional; case, whitespace, empty strings and unchanged historical values are retained.
    Drafts do not change output. Save explicitly, acknowledge and reload report options.
    These minima apply only when existing quality enforcement is active; this panel does not enable it.
    Unknown outcomes require owned Load, never blind Save replay. Only these three YAML strings change;
    no registry, project data, source audits, new qualification logic, printer reset or printing.
    Explicit Save is a desktop adaptation, not proof of Access automatic Change typing timing.</p>
</section>

<style>
  .quality-minima { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  fieldset { min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .75rem; }
  legend { padding: 0 .35rem; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr)); gap: .75rem; }
  label { display: grid; grid-template-columns: minmax(0, 1fr); gap: .3rem; min-width: 0; }
  input { box-sizing: border-box; width: 100%; min-width: 0; border: 1px solid #cbd5e1; border-radius: .3rem; padding: .45rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled, input:disabled { opacity: .5; }
  details { min-width: 0; } summary { cursor: pointer; padding: .5rem 0; }
  p, li { overflow-wrap: anywhere; } ul { padding-left: 1.25rem; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
