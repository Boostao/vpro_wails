<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { ReadRequests, type CancellableRead } from './readRequests';
  import { summaryVegetationQualityMinimumFields, summaryVegetationQualityNullFields,
    type SummaryVegetationQualityPreferencesSession, type SummaryVegetationQualityPreferencesReview } from './summaryVegetationQualityPreferences';
  import type { ReportTitlePreferencesOutcome } from './reportTitlePreferences';

  let { contextId, preferences, port, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: SummaryVegetationQualityPreferencesSession;
    port: {
      read: (contextId: string) => CancellableRead<SummaryVegetationQualityPreferencesReview | null>;
      save: (contextId: string, requestJSON: string) => PromiseLike<ReportTitlePreferencesOutcome | null>;
    };
    operationHeld?: boolean;
    mutationHeld?: boolean;
    onReadBusyChange: (busy: boolean) => void;
  } = $props();
  let view = $state.raw(untrack(() => preferences.view()));
  let reference = $state.raw(untrack(() => preferences.reference()));
  let busy = $state(false);
  let error = $state('');
  const reads = new ReadRequests();
  let generation = 0;
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
      const value = await reads.track(port.read(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) preferences.rejectRead(cause);
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError || view.readError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => port.save(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function editMinimum(key: typeof summaryVegetationQualityMinimumFields[number]['key'], raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editMinimum(key, raw);
  }
  function editNull(key: typeof summaryVegetationQualityNullFields[number]['key'], raw: boolean) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editNullFlag(key, raw);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="summary-quality" data-summary-quality-preferences aria-label="Saved Summary Data Quality options">
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.readError}<p class="error" role="alert">{view.readError}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving Summary quality options; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-summary-quality-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <fieldset>
    <legend>Data Quality Filter</legend>
    <label class="flag"><input type="checkbox" checked={view.saved?.enforce ?? false} disabled
      data-source-control="optEnforceQC" />Enforce QC (saved, read-only)</label>
    <div class="fields">
      {#each summaryVegetationQualityMinimumFields as field}
        <label for={'summary-quality-' + field.key}>
          <span>{field.label}</span>
          <input id={'summary-quality-' + field.key} type="text" data-source-control={field.control}
            list={field.key === 'becMinimum' ? 'summary-quality-bec-suggestions' : 'summary-quality-suggestions'}
            value={view.draft ? view.draft[field.key] : ''} aria-invalid={!!view.draftError}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft}
            oninput={event => { const raw = event.currentTarget.value; editMinimum(field.key, raw); }} />
        </label>
      {/each}
    </div>
    <fieldset class="null-options">
      <legend>Include Nulls</legend>
      {#each summaryVegetationQualityNullFields as field}
        <label class="flag" for={'summary-quality-' + field.key}>
          <input id={'summary-quality-' + field.key} type="checkbox" data-source-control={field.control}
            checked={view.draft ? view.draft[field.key] : false}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft}
            onchange={event => { const raw = event.currentTarget.checked; editNull(field.key, raw); }} />
          {field.label}
        </label>
      {/each}
    </fieldset>
    {#each [{ id: 'summary-quality-suggestions', rows: reference?.qualityDefinitions.rows ?? [] },
      { id: 'summary-quality-bec-suggestions', rows: reference?.becDefinitions.rows ?? [] }] as list}
      <datalist id={list.id}>
        {#each list.rows as row (row.rowId)}
          {#if row.cells && row.cells[3]?.storage === 'text' && typeof row.cells[3].text === 'string' && !row.cells[3].text.includes('\0')}
            <option value={row.cells[3].text}>{row.cells[4]?.text ?? ''}</option>
          {/if}
        {/each}
      </datalist>
    {/each}
  </fieldset>
  <div class="actions">
    <button data-summary-quality-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved quality options</button>
    <button data-summary-quality-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !!view.readError || !view.saved}
      onclick={save}>Save quality options</button>
    <button data-summary-quality-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo quality draft</button>
    {#if view.blocked && !view.busy}
      <button data-summary-quality-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge quality outcome</button>
    {/if}
    {#if busy}<button data-summary-quality-cancel onclick={cancel}>Cancel quality read</button>
      <p role="status">Reading owned Summary quality options and suggestions...</p>{/if}
  </div>
  {#if reference}
    <details data-summary-quality-definitions>
      <summary>Owned quality and BEC suggestions</summary>
      <p>{reference.source}</p>
      {#each [{ name: 'PlotQualitySite', rows: reference.qualityDefinitions.rows ?? [] },
        { name: 'BEC_Use', rows: reference.becDefinitions.rows ?? [] }] as list}
        <h3>{list.name}: {list.rows.length} physical rows</h3>
        <ul>{#each list.rows as row (row.rowId)}
          <li>Row {row.rowId}: Item {row.cells?.[3]?.storage} {JSON.stringify(row.cells?.[3]?.text)};
            description {row.cells?.[4]?.storage} {JSON.stringify(row.cells?.[4]?.text)}.</li>
        {/each}</ul>
      {/each}
    </details>
  {/if}
  <p class="guidance">Load before editing; disabled unloaded controls do not represent saved values.
    Suggestions are optional, not DataQuality ranks. Literal text and unchanged historical values are retained.
    Save changes only these quality YAML options; the BEC minimum is shared with other report preferences.
    Enforcement is read-only because its original callback reloads the form and changes other settings.
    This panel does not enable QC or claim automatic Access Change typing timing.
    Unknown outcomes require explicit owned Load, never blind Save replay.</p>
</section>

<style>
  .summary-quality { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  fieldset { min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .75rem; }
  legend { padding: 0 .35rem; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr)); gap: .75rem; margin-top: .75rem; }
  label { display: grid; grid-template-columns: minmax(0, 1fr); gap: .3rem; min-width: 0; }
  input[type="text"] { box-sizing: border-box; width: 100%; min-width: 0; border: 1px solid #cbd5e1; border-radius: .3rem; padding: .45rem; }
  .flag { display: flex; gap: .5rem; align-items: center; }
  .null-options, .actions { display: flex; flex-wrap: wrap; gap: .75rem; align-items: center; }
  .null-options { margin-top: .75rem; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled, input:disabled { opacity: .5; }
  .error { color: #991b1b; overflow-wrap: anywhere; }
  p, li { overflow-wrap: anywhere; }
  .guidance { color: #475569; font-size: .85rem; }
</style>
