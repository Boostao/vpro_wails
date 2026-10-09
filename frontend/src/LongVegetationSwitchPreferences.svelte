<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationSwitchPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationSwitchFields, type LongVegetationSwitchPreferencesSession, type LongVegetationSwitchPreferenceValues } from './longVegetationSwitchPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationSwitchPreferencesSession;
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
      const value = await reads.track(LongVegetationSwitchPreferencesService.GetLongVegetationSwitchPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved report switches unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationSwitchPreferencesService.SaveLongVegetationSwitchPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(key: keyof LongVegetationSwitchPreferenceValues, value: boolean) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editSwitch(key, value);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="switches" aria-label="Saved Long Vegetation report switches" data-long-vegetation-switches>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving report switches; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-switch-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <div class="fields">
    {#each longVegetationSwitchFields as field}
      <label for={'long-vegetation-switch-' + field.key}>
        <input id={'long-vegetation-switch-' + field.key} type="checkbox" data-source-control={field.control}
          checked={view.draft?.[field.key] === true} aria-invalid={!!view.draftError}
          disabled={busy || mutationHeld || view.busy || view.blocked || !view.initialized}
          onchange={event => { const value = event.currentTarget.checked; edit(field.key, value); }} />
        <span>{field.label}</span>
      </label>
    {/each}
  </div>
  <div class="actions">
    <button data-long-vegetation-switch-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved report switches</button>
    <button data-long-vegetation-switch-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save report switches</button>
    <button data-long-vegetation-switch-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo report switch draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-switch-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge report switch outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel report switch read</button><p role="status">Reading owned report switches...</p>{/if}
  </div>
  <p class="guidance">{#if !view.initialized}Load saved switches before editing; unloaded boxes do not represent saved values. {/if}
    Drafts do not change report output. Save explicitly, acknowledge and reload report options.
    Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only these four YAML preferences change; no registry, project data, source audits or printing.</p>
</section>

<style>
  .switches { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr)); gap: .75rem; }
  label { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: .5rem; align-items: start; }
  span { overflow-wrap: anywhere; }
  input { margin-top: .2rem; width: 1rem; height: 1rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
