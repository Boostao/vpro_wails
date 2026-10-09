<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationThresholdPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationThresholdFields, type LongVegetationThresholdPreferencesSession, type LongVegetationThresholdDraft } from './longVegetationThresholdPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationThresholdPreferencesSession;
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
      const value = await reads.track(LongVegetationThresholdPreferencesService.GetLongVegetationThresholdPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved report thresholds unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationThresholdPreferencesService.SaveLongVegetationThresholdPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(key: keyof LongVegetationThresholdDraft, raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editThreshold(key, raw);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="thresholds" aria-label="Saved Long Vegetation report thresholds" data-long-vegetation-thresholds>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving report thresholds; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-threshold-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <div class="fields">
    {#each longVegetationThresholdFields as field}
      <label for={'long-vegetation-threshold-' + field.key}>
        <span>{field.label}</span>
        <div class="value">
          <input id={'long-vegetation-threshold-' + field.key} type="text" inputmode="decimal" data-source-control={field.control}
            value={view.draft[field.key]} aria-invalid={!!view.draftError}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.initialized}
            oninput={event => { const raw = event.currentTarget.value; edit(field.key, raw); }} />
          <span>%</span>
        </div>
      </label>
    {/each}
  </div>
  <div class="actions">
    <button data-long-vegetation-threshold-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved report thresholds</button>
    <button data-long-vegetation-threshold-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save report thresholds</button>
    <button data-long-vegetation-threshold-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo report threshold draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-threshold-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge report threshold outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel report threshold read</button><p role="status">Reading owned report thresholds...</p>{/if}
  </div>
  <p class="guidance">Load saved thresholds before editing; no defaults are applied. Drafts do not change report output.
    Save explicitly, acknowledge and reload report options. Source comparisons are strict; constant-list mode bypasses these thresholds.
    No new range is imposed. Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only these two YAML preferences change; no registry, project data, source audits or printing.</p>
</section>

<style>
  .thresholds { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr)); gap: .75rem; }
  label { display: grid; gap: .35rem; min-width: 0; }
  .value { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: .5rem; align-items: center; }
  span { overflow-wrap: anywhere; }
  input { width: 100%; box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
