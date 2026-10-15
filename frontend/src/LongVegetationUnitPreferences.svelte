<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LongVegetationUnitPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { longVegetationUnitChoices, type LongVegetationUnitPreferencesSession } from './longVegetationUnitPreferences';

  let { contextId, preferences, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: LongVegetationUnitPreferencesSession;
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
      const value = await reads.track(LongVegetationUnitPreferencesService.GetLongVegetationUnitPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved Unit Groups unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onReadBusyChange(false); }
    }
  }
  async function save() {
    if (busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || view.draftError) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationUnitPreferencesService.SaveLongVegetationUnitPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function edit(raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editUnitGroups(raw);
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="unit-preferences" aria-label="Saved Long Vegetation Unit Groups" data-long-vegetation-unit-preferences>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving Unit Groups; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-long-vegetation-unit-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
    </p>
  {/each}
  <label for="long-vegetation-unit-groups">
    <span>Unit Groups</span>
    <select id="long-vegetation-unit-groups" data-source-control="rptLvUnitGroups" value={view.draft.unitGroups}
      aria-invalid={!!view.draftError}
      disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onchange={event => { const raw = event.currentTarget.value; edit(raw); }}>
      <option value="" disabled>Load saved Unit Groups</option>
      {#each longVegetationUnitChoices as mode}
        <option value={String(mode.value)} disabled={!availability?.[mode.capability] && view.saved?.unitGroups !== mode.value}>
          {mode.label}
        </option>
      {/each}
    </select>
  </label>
  <div class="actions">
    <button data-long-vegetation-unit-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved Unit Groups</button>
    <button data-long-vegetation-unit-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !view.saved}
      onclick={save}>Save Unit Groups</button>
    <button data-long-vegetation-unit-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo Unit Groups draft</button>
    {#if view.blocked && !view.busy}
      <button data-long-vegetation-unit-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge Unit Groups outcome</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel Unit Groups read</button><p role="status">Reading owned Unit Groups...</p>{/if}
  </div>
  <p class="guidance">Load saved Unit Groups before editing; no defaults are applied. Drafts do not change report output.
    Save explicitly, acknowledge and reload report options. Selected fields require a current owned session Build
    and independently enabled consumers; all-project fallback remains unavailable.
    None uses Env/current selected-list joins, not None species grouping.
    Retaining an unchanged unavailable saved choice does not enable its report.
    Changing Enforce rules resets the saved Unit Groups to Site unit table; retained drafts require owned Load or correction.
    Receipts remain after acknowledgement; unknown outcomes require explicit owned Load, never blind Save replay.
    Only LVUnitGroups changes in YAML; no registry, project data, source scratch tables, audits or printing.</p>
</section>

<style>
  .unit-preferences { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  label { display: grid; gap: .35rem; min-width: 0; }
  span { overflow-wrap: anywhere; }
  select { width: 100%; box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .error { color: #b91c1c; } .guidance { color: #57534e; font-size: .875rem; }
</style>
