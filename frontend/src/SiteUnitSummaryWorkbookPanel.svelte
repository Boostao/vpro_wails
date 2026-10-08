<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SiteUnitSummaryWorkbookService } from '../bindings/github.com/boostao/vpro-wails';
  import type { GoogleEarthScope } from './googleEarthReview';
  import { ReadRequests } from './readRequests';
  import {
    siteUnitSummaryWorkbookPublicationSession, validateSiteUnitSummaryWorkbookReview,
    type ValidatedSiteUnitSummaryWorkbookReview,
  } from './siteUnitSummaryWorkbook';

  let { owner, method, disabled, onBusyChange }: {
    owner: GoogleEarthScope; method: number | null; disabled: boolean; onBusyChange: (held: boolean) => void;
  } = $props();
  const enabled = import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true';
  const owned = untrack(() => structuredClone(owner));
  const session = siteUnitSummaryWorkbookPublicationSession(owned);
  const reads = new ReadRequests();
  let generation = 0;
  let disposed = false;
  let busy = $state(false);
  let error = $state('');
  let review = $state.raw<ValidatedSiteUnitSummaryWorkbookReview | null>(null);
  let destination = $state('');
  let publication = $state(session.view());
  const validMethod = $derived(method === 1 || method === 2);
  const currentReview = $derived(review !== null && review.preview.report.method === method);
  const barrier = $derived(publication.busy || publication.blocked);
  const held = $derived(disabled || busy || barrier);
  const normalOwner = owned.su !== 'None' && owned.su !== 'USysSuTableDynamic';
  function reportBusy() { onBusyChange(busy || publication.busy || publication.blocked); }
  const unsubscribe = session.subscribe(() => {
    publication = session.view();
    if (publication.busy) review = null;
    reportBusy();
  });
  function cancel() {
    generation++; reads.cancelAll(); busy = false; review = null; reportBusy();
  }
  async function getReview() {
    if (!enabled || held || !normalOwner || !validMethod || method === null) return;
    cancel();
    const requestGeneration = generation, requestedMethod = method;
    busy = true; error = ''; reportBusy();
    try {
      const value = await reads.track(SiteUnitSummaryWorkbookService.GetReview(owned.contextId,
        JSON.stringify({ method: requestedMethod })));
      if (disposed || requestGeneration !== generation || method !== requestedMethod) return;
      review = validateSiteUnitSummaryWorkbookReview(value, owned, requestedMethod);
    } catch (cause) {
      if (!disposed && requestGeneration === generation && method === requestedMethod) {
        error = `Workbook review unavailable; no file published: ${String(cause)}`;
      }
    } finally {
      if (!disposed && requestGeneration === generation) { busy = false; reportBusy(); }
    }
  }
  async function publish() {
    if (!enabled || held || !review || !currentReview || !validMethod || method === null) return;
    error = '';
    try {
      await session.publish(review, destination, request =>
        SiteUnitSummaryWorkbookService.ExportReviewed(owned.contextId, JSON.stringify(request)), method);
    } catch (cause) {
      if (!disposed) error = String(cause);
    }
  }
  function acknowledge() {
    if (publication.busy || !publication.blocked) return;
    try { session.acknowledge(); review = null; }
    catch (cause) { error = String(cause); }
  }
  onDestroy(() => {
    disposed = true; generation++; reads.cancelAll(); busy = false;
    reportBusy(); unsubscribe();
  });
</script>

<section class="workbook" data-site-unit-workbook aria-label="Summary Environment workbook">
  <h2>Summary Environment workbook</h2>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
  {#if publication.busy}
    <p role="status">Publishing reviewed bytes. Publication cannot be cancelled; navigation and close remain blocked.</p>
  {/if}
  {#if publication.outcome}
    <p data-site-unit-workbook-receipt role="status">Workbook receipt: {publication.outcome.status}.
      Requested destination: {publication.requestedDestination}. Output: {publication.outcome.path || '(no published path)'}.
      SHA256: {publication.outcome.sha256 || '(no acknowledged hash)'}.</p>
  {:else if publication.requestedDestination && !publication.busy}
    <p data-site-unit-workbook-unknown role="alert">Workbook outcome unknown. Retain and inspect
      {publication.requestedDestination} before acknowledging; never repeat the destination blindly.</p>
  {/if}
  {#if publication.blocked && !publication.busy}
    <button data-site-unit-workbook-acknowledge onclick={acknowledge}>Acknowledge workbook outcome</button>
  {/if}
  {#if !enabled}
    <p>Workbook publication is unavailable. The independent Summary Environment workbook feature is disabled.</p>
  {/if}
  {#if review && !currentReview}
    <p role="alert" class="error">The selected method changed. Review the current method before publishing; the prior review cannot be used.</p>
  {/if}
  <p>Requested method: {method === 1 ? 'Mean (1)' : method === 2 ? 'Interquartile (2)' : 'Unavailable'}.</p>
  <div class="actions">
    <button data-site-unit-workbook-review disabled={!enabled || held || !normalOwner || !validMethod}
      onclick={getReview}>Review summary workbook</button>
    <button data-site-unit-workbook-cancel disabled={!busy || barrier} onclick={cancel}>Cancel review</button>
  </div>
  {#if busy}<p role="status">Reading the source workbook plan…</p>{/if}
  {#if review && currentReview}
    <div class="plan">
      <h3>Reviewed source plan</h3>
      <p>Project: {review.preview.report.project}. Site Unit Table: {review.preview.report.su}.
        Method: {review.preview.report.method === 1 ? 'Mean (1)' : 'Interquartile (2)'}.
        Saved method: {review.scope.savedMethod}. Normal Site Unit scope: {review.scope.siteUnitType}.
        Order: {review.scope.orderBy}. Species included: {review.scope.includeSpecies}.</p>
      <p>Original memberships: {review.preview.report.memberships.length}.
        Joined rows: {review.preview.report.units.reduce((sum, unit) => sum + unit.plots.length, 0)}.
        Worksheets: {review.sheets.length}. Planned bytes: {review.bytes}.</p>
      {#each review.preview.report.units as unit, unitIndex}
        <section class="unit" aria-label={`Site Unit ${unit.code || '(empty)'}`}>
          <h4>Site Unit: {unit.code === '' ? '(empty)' : unit.code}</h4>
          <p>Source long name: {unit.longName === null ? '(NULL / missing)' : unit.longName === '' ? '(empty)' : unit.longName}.
            Name status: {unit.nameStatus}. Name candidates: {unit.nameCandidates.length}.
            Joined rows: {unit.plots.length}. Worksheet: {review.sheets[unitIndex].name}.</p>
          <div class="summaries">
            {#each ['SITE', 'VEGETATION', 'SOILS'] as section}
              <table>
                <caption>{section} — {unit.code === '' ? '(empty unit)' : unit.code}</caption>
                <thead><tr><th scope="col">Summary field</th><th scope="col">Source value</th></tr></thead>
                <tbody>
                  {#each review.preview.report.fields as field, index}
                    {#if field.section === section}
                      <tr><th scope="row">{field.label}</th><td>{unit.values[index] === '' ? '(empty)' : unit.values[index]}</td></tr>
                    {/if}
                  {/each}
                </tbody>
              </table>
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {/if}
  <label class="destination">Workbook destination (.xlsx, literal path)
    <input data-site-unit-workbook-destination type="text" bind:value={destination} disabled={!enabled || held}
      autocomplete="off" spellcheck="false" />
  </label>
  <button data-site-unit-workbook-export disabled={!enabled || held || !currentReview || !validMethod || !destination}
    onclick={publish}>Publish reviewed workbook</button>
  <p class="guidance">Review is read-only and cancellable. The selected method is an explicit override, not a saved-option change.
    Only normal Site Unit summaries without species or layer reports are available. Enter an explicit .xlsx path;
    it is never trimmed or replaced automatically. Publication is irreversible; acknowledge every returned or unknown outcome
    before navigation, close or another publication. Unavailable workflows remain disabled.</p>
</section>

<style>
  .workbook { display: grid; gap: 0.75rem; min-width: 0; }
  h2, h3, h4, p { margin: 0; }
  .actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
  .plan, .unit { display: grid; gap: 0.75rem; min-width: 0; }
  .summaries { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 22rem), 1fr)); gap: 1rem; align-items: start; }
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  caption { text-align: left; font-weight: bold; padding: 0.5rem; }
  th, td { text-align: left; vertical-align: top; padding: 0.5rem; border-bottom: 1px solid #cbd5e1; overflow-wrap: anywhere; white-space: pre-wrap; }
  .destination { display: grid; gap: 0.25rem; }
  input { box-sizing: border-box; width: 100%; min-width: 0; }
  .error { color: #b91c1c; overflow-wrap: anywhere; }
  .guidance { font-size: 0.875rem; }
</style>
