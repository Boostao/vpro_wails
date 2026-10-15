<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { ReadRequests, type CancellableRead } from './readRequests';
  import type { SummaryVegetationPresentationPreferencesReview, SummaryVegetationPresentationPreferencesSession } from './summaryVegetationPresentationPreferences';
  import type { ReportTitlePreferencesOutcome } from './reportTitlePreferences';

  let { contextId, preferences, port, operationHeld = false, mutationHeld = false, onReadBusyChange }: {
    contextId: string;
    preferences: SummaryVegetationPresentationPreferencesSession;
    port: {
      read: (contextId: string) => CancellableRead<SummaryVegetationPresentationPreferencesReview | null>;
      save: (contextId: string, requestJSON: string) => PromiseLike<ReportTitlePreferencesOutcome | null>;
    };
    operationHeld?: boolean;
    mutationHeld?: boolean;
    onReadBusyChange: (busy: boolean) => void;
  } = $props();
  const flags = [
    { key: 'showCommon', control: 'rptSvShowCommon', label: 'Show english name' },
    { key: 'showGuide', control: 'optUseShortFieldGuideNames', label: 'Use short field guide names' },
    { key: 'includeEnv', control: 'optIncludeEnvFields', label: 'Include Environmental Fields?' },
    { key: 'createSummary', control: 'optCreateSummary', label: 'Create report summary' },
    { key: 'combineVariants', control: 'optCombineVariants', label: 'Treat subspecies/varieties as species?' },
    { key: 'constancy', control: 'optConstancyFormat', label: 'Use Constancy Format' },
  ] as const;
  let view = $state.raw(untrack(() => preferences.view()));
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
  function editTitle(raw: string) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = ''; preferences.editTitle(raw);
  }
  function editFlag(key: typeof flags[number]['key'], raw: boolean) {
    if (busy || mutationHeld || view.busy || view.blocked) return;
    cancel(); error = '';
    try {
      preferences.editFlag(key, raw);
    } catch (cause) {
      error = String(cause);
    }
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="summary-presentation" data-summary-presentation-preferences aria-label="Saved Summary presentation drafts">
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if view.readError}<p class="error" role="alert">{view.readError}</p>{/if}
  {#if view.draftError}<p class="error" role="alert">{view.draftError}</p>{/if}
  {#if view.error}<p class="error" role="alert">{view.error}</p>{/if}
  {#if view.busy}<p role="status">Saving Summary presentation; Save cannot be cancelled.</p>{/if}
  {#each view.receipts as receipt}
    <p data-summary-presentation-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
      {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
      {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
      Constancy-on callback plan: {String(receipt.request.constancyApplied)}.
    </p>
  {/each}
  <fieldset>
    <legend>Stored Summary presentation</legend>
    <label class="title" for="summary-presentation-title">
      <span>Report Title</span>
      <input id="summary-presentation-title" data-source-control="rptSvTitle" type="text"
        value={view.draft?.title ?? ''} aria-invalid={!!view.draftError}
        disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft}
        oninput={event => { const raw = event.currentTarget.value; editTitle(raw); }} />
    </label>
    <div class="flags">
      {#each flags as field}
        <label for={'summary-presentation-' + field.key}>
          <input id={'summary-presentation-' + field.key} data-source-control={field.control} type="checkbox"
            checked={view.draft ? view.draft[field.key] : false}
            disabled={busy || mutationHeld || view.busy || view.blocked || !view.draft || field.key === 'showCommon' && view.draft.constancy}
            onchange={event => { const raw = event.currentTarget.checked; editFlag(field.key, raw); }} />
          <span>{field.label}</span>
        </label>
      {/each}
      <label for="summary-presentation-hierarchyHeader">
        <input id="summary-presentation-hierarchyHeader" data-source-control="optIncludeHierarchyHeader"
          type="checkbox" checked={view.draft?.hierarchyHeader ?? false} disabled />
        <span>Include hierarchy header <small>(saved, read-only; hierarchy scope unavailable)</small></span>
      </label>
    </div>
    {#if view.draft}
      <dl data-summary-presentation-callback>
        <div><dt>Shared Group by</dt><dd>{view.draft.groupBy}</dd></div>
        <div><dt>Shared Display</dt><dd>{view.draft.display}</dd></div>
        <div><dt>Retained value limit</dt><dd>{view.draft.valueLimit}</dd></div>
        <div><dt>Staged constancy-on callback</dt><dd>{String(view.draft.constancyApplied)}</dd></div>
      </dl>
    {/if}
  </fieldset>
  <div class="actions">
    <button data-summary-presentation-load disabled={busy || operationHeld || view.busy || view.blocked}
      onclick={load}>Load saved presentation</button>
    <button data-summary-presentation-save
      disabled={busy || operationHeld || view.busy || view.blocked || view.authorityUnknown || !!view.draftError || !!view.readError || !view.saved}
      onclick={save}>Save presentation draft</button>
    <button data-summary-presentation-undo disabled={busy || mutationHeld || view.busy || view.blocked || !view.saved}
      onclick={() => { cancel(); preferences.undo(); }}>Undo presentation draft</button>
    {#if view.blocked && !view.busy}
      <button data-summary-presentation-acknowledge disabled={busy || mutationHeld}
        onclick={() => preferences.acknowledge()}>Acknowledge presentation outcome</button>
    {/if}
    {#if busy}<button data-summary-presentation-cancel onclick={cancel}>Cancel presentation read</button>
      <p role="status">Reading owned saved Summary presentation...</p>{/if}
  </div>
  <p class="guidance">Configuration-only drafts, not report-output formatting parity.
    Load before editing; unloaded controls do not represent saved values.
    Constancy-on stages Group by3, Display1 and english names off without clearing the stored limit;
    switching it off does not restore old choices. Reload shared Summary options after a committed callback plan.
    These drafts do not enable guide renames, environmental field-list editing, hierarchy reports,
    constancy-format output, report summaries, printing or workbooks.</p>
</section>

<style>
  .summary-presentation { min-width: 0; margin: 0.75rem 0; }
  fieldset { min-width: 0; border: 1px solid #9ca3af; border-radius: 0.5rem; padding: 0.75rem; }
  legend { font-weight: 600; }
  .title { display: grid; grid-template-columns: minmax(0, 1fr); gap: 0.25rem; }
  .title input { width: 100%; min-width: 0; box-sizing: border-box; border: 1px solid #6b7280; border-radius: 0.25rem; padding: 0.5rem; }
  .flags { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem 1rem; margin-top: 0.75rem; }
  .flags label { display: flex; align-items: flex-start; gap: 0.5rem; min-width: 0; min-height: 2.75rem; overflow-wrap: anywhere; }
  .flags input { flex: none; margin-top: 0.25rem; }
  small { display: block; }
  dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem 1rem; margin-top: 0.75rem; }
  dl div { min-width: 0; overflow-wrap: anywhere; }
  dt { font-weight: 600; }
  dd { margin: 0; }
  .actions { display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem; margin-top: 0.5rem; }
  button { min-height: 2.75rem; max-width: 100%; padding: 0.5rem 0.75rem; border: 1px solid #6b7280; border-radius: 0.25rem; }
  button:disabled { opacity: 0.55; }
  .error { color: #991b1b; overflow-wrap: anywhere; margin-bottom: 0.5rem; }
  .guidance { font-size: 0.875rem; margin-top: 0.75rem; overflow-wrap: anywhere; }
  @media (max-width: 640px) { .flags, dl { grid-template-columns: minmax(0, 1fr); } }
</style>
