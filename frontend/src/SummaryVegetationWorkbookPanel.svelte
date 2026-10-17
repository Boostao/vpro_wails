<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SummaryVegetationWorkbookService } from '../bindings/github.com/boostao/vpro-wails';
  import type { SummaryVegetationOwner } from './summaryVegetation';
  import { summaryEnvironmentalLifeformEnabled } from './summaryVegetationEnvironmentalDesktop';
  import { summaryVegetationWorkbookSession, sameSummaryWorkbookOptions,
    type SummaryWorkbookCalculation } from './summaryVegetationWorkbook';

  let { owner, calculation, disabled, onBusyChange }: {
    owner: SummaryVegetationOwner; calculation: SummaryWorkbookCalculation;
    disabled: boolean; onBusyChange: (held: boolean) => void;
  } = $props();
  const owned = untrack(() => structuredClone(owner));
  const session = summaryVegetationWorkbookSession(owned);
  const enabled = import.meta.env.VITE_SUMMARY_VEGETATION_WORKBOOK === 'true';
  const environmentEnabled = summaryEnvironmentalLifeformEnabled;
  let viewState = $state.raw(session.view());
  let error = $state('');
  const barrier = $derived(viewState.publication.busy || viewState.publication.blocked);
  const held = $derived(disabled || viewState.reviewing || barrier);
  const currentReview = $derived(!!viewState.review && sameSummaryWorkbookOptions(viewState.review.options, {
    ...calculation, title: viewState.title, includeEnv: viewState.includeEnv,
  }));
  const unsubscribe = session.subscribe(() => { viewState = session.view(); onBusyChange(session.held()); });
  function edit(field: 'title' | 'destination', raw: string) {
    if (held) return;
    error = '';
    try { session.edit(field, raw); } catch (cause) { error = String(cause); }
  }
  function includeEnvironment(event: Event) {
    if (held || !environmentEnabled) return;
    error = '';
    try { session.includeEnvironment((event.currentTarget as HTMLInputElement).checked); }
    catch (cause) { error = String(cause); }
  }
  function undo() {
    if (held) return;
    error = '';
    try { session.undoDraft(); } catch (cause) { error = String(cause); }
  }
  async function review() {
    if (!enabled || held || viewState.draftError || viewState.includeEnv && !environmentEnabled) return;
    error = '';
    try {
      await session.review(calculation, {
        review: (contextId, options) => SummaryVegetationWorkbookService.GetReview(contextId, JSON.stringify(options)),
        validationYield: () => new Promise(resolve => setTimeout(resolve, 0)),
      });
    } catch (cause) { error = String(cause); }
  }
  async function publish() {
    if (!enabled || held || viewState.draftError || !currentReview) return;
    error = '';
    try {
      await session.publish(calculation, request =>
        SummaryVegetationWorkbookService.ExportReviewed(owned.contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function acknowledge() {
    if (viewState.publication.busy || !viewState.publication.blocked) return;
    error = '';
    try { session.acknowledge(); } catch (cause) { error = String(cause); }
  }
  onDestroy(() => { session.cancel(); unsubscribe(); });
</script>

<section class="workbook" data-summary-vegetation-workbook aria-label="Summary inspection workbook">
  <h2>Summary inspection workbook (draft)</h2>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if viewState.error}<p role="alert">{viewState.error}</p>{/if}
  {#if viewState.draftError}<p role="alert">{viewState.draftError}</p>{/if}
  {#if viewState.publication.error}<p role="alert">{viewState.publication.error}</p>{/if}
  {#if viewState.publication.busy}
    <p role="status">Publishing reviewed bytes. Publication cannot be cancelled; navigation and close remain blocked.</p>
  {/if}
  {#if viewState.publication.outcome}
    <p data-summary-workbook-receipt role="status">Workbook receipt: {viewState.publication.outcome.status}.
      Requested: {viewState.publication.requestedDestination}. Output: {viewState.publication.outcome.path || '(no published path)'}.
      SHA256: {viewState.publication.outcome.sha256 || '(no acknowledged hash)'}.</p>
  {:else if viewState.publication.requestedDestination && !viewState.publication.busy}
    <p data-summary-workbook-unknown role="alert">Workbook outcome unknown. Retain and inspect
      {viewState.publication.requestedDestination}; never repeat the destination blindly.</p>
  {/if}
  {#if viewState.publication.blocked && !viewState.publication.busy}
    <button data-summary-workbook-acknowledge onclick={acknowledge}>Acknowledge workbook outcome</button>
  {/if}
  {#if !enabled}<p>Inspection workbook publication is disabled in this build.</p>{/if}
  <div class="fields">
    <label>Workbook title
      <input data-summary-workbook-title value={viewState.title} disabled={!enabled || held}
        oninput={event => edit('title', event.currentTarget.value)} />
    </label>
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-environment checked={viewState.includeEnv}
        disabled={!enabled || held || !environmentEnabled} onchange={includeEnvironment} />
      Include selected environmental rows
    </label>
    <label>New workbook destination (.xlsx)
      <input data-summary-workbook-destination value={viewState.destination} disabled={!enabled || held}
        oninput={event => edit('destination', event.currentTarget.value)} />
    </label>
  </div>
  <div class="actions">
    <button data-summary-workbook-review disabled={!enabled || held || !!viewState.draftError} onclick={review}>Review inspection workbook</button>
    <button data-summary-workbook-cancel disabled={!viewState.reviewing || barrier} onclick={() => session.cancel()}>Cancel review</button>
    <button data-summary-workbook-undo disabled={!enabled || held} onclick={undo}>Undo export options</button>
    <button data-summary-workbook-publish disabled={!enabled || held || !!viewState.draftError || !currentReview || !viewState.destination}
      onclick={publish}>Publish reviewed workbook</button>
  </div>
  {#if viewState.reviewing}<p role="status">Reading and validating the complete owned workbook plan...</p>{/if}
  {#if viewState.review}
    <div data-summary-workbook-plan class="plan">
      {#if !currentReview}<p role="alert">Calculation options changed; review the current options before publication.</p>{/if}
      <p>Sheet: {viewState.review.sheet}. {viewState.review.preview.rows.length} vegetation rows;
        {viewState.review.environment?.rows.length ?? 0} environmental rows; first vegetation row {viewState.review.firstVegetationRow}.
        {viewState.review.bytes} bytes.</p>
      <p>Approval: {viewState.review.approvalHash}<br />Workbook SHA256: {viewState.review.workbookSHA256}</p>
      <p>{viewState.review.scope}</p>
    </div>
  {/if}
  <p>Explicit Layer inspection export using Calculate, Display Values and Show above, plus these title/environment options.
    Saved presentation callbacks are not applied. Source data, preferences and audit history are not changed.
    This is not the original Access Report action: attribute-description preflight, ancillary quality/SU sheets,
    original formatting, printing and macros remain unavailable. Existing destinations are never replaced.</p>
</section>

<style>
  .workbook, .plan { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; overflow-wrap: anywhere; }
  .workbook { padding: 1rem; border: 1px solid #d1d5db; border-radius: .5rem; }
  .fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr)); gap: 1rem; }
  label { display: flex; flex-direction: column; gap: .35rem; min-width: 0; }
  input:not([type="checkbox"]) { width: 100%; box-sizing: border-box; min-width: 0; padding: .5rem; }
  .checkbox { flex-direction: row; align-items: center; }
  .actions { display: flex; flex-wrap: wrap; gap: .75rem; }
  button { min-height: 2.5rem; padding: .5rem .75rem; }
  [role="alert"] { color: var(--color-red-700, #b91c1c); }
</style>
