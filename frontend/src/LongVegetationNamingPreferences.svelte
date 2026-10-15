<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationNamingPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationNamingChoices, type LongVegetationNamingPreferencesSession } from './longVegetationNamingPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, selectedFieldEnabled = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationNamingPreferencesSession;
    operationHeld?: boolean;
    mutationHeld?: boolean;
    selectedFieldEnabled?: boolean;
    onReadBusyChange: (busy: boolean) => void;
  } = $props();
  let view = $state(untrack(() => preferences.view()));
  let availability = $state(untrack(() => preferences.availability()));
  let busy = $state(false);
  let error = $state('');
  const reads = new ReadRequests();
  let generation = 0;
  const unsubscribe = untrack(() => preferences.subscribe(() => {
    view = preferences.view(); availability = preferences.availability();
  }));
  function cancel() {
    generation++; reads.cancelAll(); preferences.cancelLoad(); busy = false; onReadBusyChange(false);
  }
  async function load() {
    if (busy || operationHeld || view.busy || view.blocked) return;
    cancel(); const request = generation, token = preferences.beginLoad();
    error = ''; busy = true; onReadBusyChange(true);
    try {
      const value = await reads.track(LongVegetationNamingPreferencesService.GetLongVegetationNamingPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved Show values unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationNamingPreferencesService.SaveLongVegetationNamingPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editMode(raw);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="naming" aria-label="Saved Long Vegetation Show preferences" data-long-vegetation-naming>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving Show preferences; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-naming-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <label for="long-vegetation-naming">
    <span>Show</span>
    <select id="long-vegetation-naming" data-source-control="rptLvShowCommon" value={view.draft.mode}
      aria-invalid={!!view.draftError}
      disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onchange={event => { const raw = event.currentTarget.value; edit(raw); }}>
      <option value="" disabled>Load saved Show</option>
      {#each longVegetationNamingChoices as choice}
        <option value={String(choice.value)} disabled={!availability?.[choice.capability] && view.saved?.mode !== choice.value}>
          {choice.label}
        </option>
      {/each}
      {#if view.saved?.mode === 4}
        <option value="4" disabled>Saved selected-field mode ({selectedFieldEnabled ? 'read-only here' : 'unavailable'})</option>
      {/if}
    </select>
  </label>
  {#if view.saved}
    <dl><div><dt>Saved selected field (read-only)</dt>
      <dd data-source-control="cmbShowField">{JSON.stringify(view.saved.selectedField)}</dd></div></dl>
    {#if view.draft.selectedField !== view.saved.selectedField}
      <p role="status">The staged Show selection clears the selected field on Save.</p>
    {/if}
  {/if}
  <div class="actions">
    <button data-long-vegetation-naming-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved Show</button>
    <button data-long-vegetation-naming-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save Show preferences</button>
    <button data-long-vegetation-naming-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo Show draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-naming-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge Show outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel Show read</button><p role="status">Reading owned Show values...</p>{/if}
  </div>
  <p class="guidance">Load saved Show values before editing; no defaults are applied. A staged source Show selection
    clears its paired selected field, even after returning to the original choice. Undo restores the saved pair.
    Drafts do not change report output. Save explicitly, acknowledge and reload report options.
    Historical unavailable modes can remain unchanged without enabling their report.
    Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only LVShowEnglishName and the source-required empty LVShowField change in YAML; no registry, project data,
    source audits, custom-field selection, new calculations or printing.</p>
</section>

<style>
  .naming { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  label { display: grid; gap: .35rem; min-width: 0; }
  span, dd { overflow-wrap: anywhere; } dd { margin: .25rem 0 0; }
  select { width: 100%; box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
