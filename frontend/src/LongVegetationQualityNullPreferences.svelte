<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationQualityNullPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationQualityNullFields, type LongVegetationQualityNullPreferencesSession, type LongVegetationQualityNullPreferenceValues } from './longVegetationQualityNullPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationQualityNullPreferencesSession;
    operationHeld?: boolean;
    mutationHeld?: boolean;
    onReadBusyChange: (busy: boolean) => void;
  } = $props();
  let view = $state(untrack(() => preferences.view()));
  let busy = $state(false);
  let error = $state('');
  const reads = new ReadRequests();
  let generation = 0;
  const unsubscribe = untrack(() => preferences.subscribe(() => { view = preferences.view(); }));
  function cancel() {
    generation++; reads.cancelAll(); preferences.cancelLoad(); busy = false; onReadBusyChange(false);
  }
  async function load() {
    if (busy || operationHeld || view.busy || view.blocked) return;
    cancel(); const request = generation, token = preferences.beginLoad();
    error = ''; busy = true; onReadBusyChange(true);
    try {
      const value = await reads.track(LongVegetationQualityNullPreferencesService.GetLongVegetationQualityNullPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved quality Include Nulls flags unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationQualityNullPreferencesService.SaveLongVegetationQualityNullPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(key: keyof LongVegetationQualityNullPreferenceValues, checked: boolean) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editNullFlag(key, checked);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="quality-nulls" aria-label="Saved Long Vegetation Data Quality Include Nulls" data-long-vegetation-quality-nulls>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving quality Include Nulls flags; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-quality-null-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <fieldset>
    <legend>Data Quality - Include Nulls</legend>
    <div class="fields">
      {#each longVegetationQualityNullFields as field}
        <label for={'long-vegetation-quality-null-' + field.key}>
          <input id={'long-vegetation-quality-null-' + field.key} type="checkbox" data-source-control={field.control}
            checked={view.draft ? view.draft[field.key] : false} aria-invalid={!!view.draftError}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft}
            onchange={event => { const checked = event.currentTarget.checked; edit(field.key, checked); }} />
          <span>{field.label} - Include Nulls</span>
        </label>
      {/each}
    </div>
  </fieldset>
  <div class="actions">
    <button data-long-vegetation-quality-null-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved quality Include Nulls</button>
    <button data-long-vegetation-quality-null-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save quality Include Nulls</button>
    <button data-long-vegetation-quality-null-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo quality Include Nulls draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-quality-null-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge quality Include Nulls outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel quality Include Nulls read</button><p role="status">Reading owned quality Include Nulls flags...</p>{/if}
  </div>
  <p class="guidance">Load saved flags before editing; unloaded disabled checkboxes do not represent saved values.
    Drafts do not change report output. Save explicitly, acknowledge and reload report options.
    These flags apply only when the existing saved quality enforcement is active; this panel does not enable it or change minimum criteria.
    Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only these three YAML preferences change; no registry, project data, source audits, new qualification logic or printing.</p>
</section>

<style>
  .quality-nulls { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  fieldset { min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .75rem; }
  legend { padding: 0 .35rem; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr)); gap: .75rem; }
  label { display: flex; align-items: flex-start; gap: .5rem; min-width: 0; }
  span { overflow-wrap: anywhere; }
  input { flex: none; margin-top: .2rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
