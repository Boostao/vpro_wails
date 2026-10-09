<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationGroupPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationGroupChoices, type LongVegetationGroupPreferencesSession } from './longVegetationGroupPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationGroupPreferencesSession;
    operationHeld?: boolean;
    mutationHeld?: boolean;
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
      const value = await reads.track(LongVegetationGroupPreferencesService.GetLongVegetationGroupPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved report grouping unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationGroupPreferencesService.SaveLongVegetationGroupPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editGroup(raw);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="grouping" aria-label="Saved Long Vegetation report grouping" data-long-vegetation-grouping>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving report grouping; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-group-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <label for="long-vegetation-group">
    <span>Group by</span>
    <select id="long-vegetation-group" data-source-control="rptLvGroupBy" value={view.draft.groupBy}
      aria-invalid={!!view.draftError}
      disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onchange={event => { const raw = event.currentTarget.value; edit(raw); }}>
      <option value="" disabled>Load saved Group by</option>
      {#each longVegetationGroupChoices as mode}
        <option value={String(mode.value)} disabled={!availability?.[mode.capability] && view.saved?.groupBy !== mode.value}>
          {mode.label}
        </option>
      {/each}
    </select>
  </label>
  <div class="actions">
    <button data-long-vegetation-group-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved report grouping</button>
    <button data-long-vegetation-group-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save report grouping</button>
    <button data-long-vegetation-group-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo report grouping draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-group-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge report grouping outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel report grouping read</button><p role="status">Reading owned report grouping...</p>{/if}
  </div>
  <p class="guidance">Load saved grouping before editing; no defaults are applied. Drafts do not change report output.
    Save explicitly, acknowledge and reload report options. Unavailable modes cannot be newly selected;
    retaining an unchanged saved mode does not enable its report.
    Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only LVGroupBy changes in YAML; no registry, project data, source audits, new calculations or printing.</p>
</section>

<style>
  .grouping { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  label { display: grid; gap: .35rem; min-width: 0; }
  span { overflow-wrap: anywhere; }
  select { width: 100%; box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
