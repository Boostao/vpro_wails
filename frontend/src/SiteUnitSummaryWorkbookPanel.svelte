<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { SiteUnitSummaryWorkbookService, SiteUnitSummaryExtendedWorkbookService } from '../bindings/github.com/boostao/vpro-wails';
  import type { GoogleEarthScope } from './googleEarthReview';
  import { ReadRequests } from './readRequests';
  import {
    siteUnitSummaryWorkbookPublicationSession, validateSiteUnitSummaryWorkbookReview,
    type ValidatedSiteUnitSummaryWorkbookReview,
  } from './siteUnitSummaryWorkbook';
  import {
    extendedSummaryWorkbookPublicationSession, validateExtendedSummaryWorkbookReview, sameExtendedSummaryWorkbookOptions,
    type ExtendedSummaryWorkbookOptions, type ValidatedExtendedSummaryWorkbookReview,
  } from './siteUnitSummaryExtendedWorkbook';
  import { reportCellText } from './longEnvironmentReport';

  let { owner, method, disabled, onBusyChange, extendedOptions }: {
    owner: GoogleEarthScope; method: number | null; disabled: boolean; onBusyChange: (held: boolean) => void;
    extendedOptions?: ExtendedSummaryWorkbookOptions | null;
  } = $props();
  const extended = untrack(() => extendedOptions !== undefined);
  const enabled = extended ? import.meta.env.VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK === 'true'
    : import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true';
  const owned = untrack(() => structuredClone(owner));
  const originalSession = siteUnitSummaryWorkbookPublicationSession(owned);
  const extendedSession = extended ? extendedSummaryWorkbookPublicationSession(owned) : null;
  const session = extendedSession ?? originalSession;
  const reads = new ReadRequests();
  let generation = 0;
  let disposed = false;
  let busy = $state(false);
  let error = $state('');
  let review = $state.raw<ValidatedSiteUnitSummaryWorkbookReview | ValidatedExtendedSummaryWorkbookReview | null>(null);
  let destination = $state('');
  let publication = $state(session.view());
  const validMethod = $derived(method === 1 || method === 2);
  const currentReview = $derived(review !== null && ('environment' in review
    ? sameExtendedSummaryWorkbookOptions(review.options, extendedOptions ?? null)
    : review.preview.report.method === method));
  const reviewedPreview = $derived(review === null ? null : 'environment' in review ? review.environment : review.preview);
  const reviewedSpecies = $derived(review !== null && 'species' in review ? review.species : null);
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
    if (!enabled || held || !normalOwner || !validMethod || method === null || (extended && !extendedOptions)) return;
    cancel();
    const requestGeneration = generation, requestedMethod = method;
    const requestedOptions = extendedOptions ? structuredClone(extendedOptions) : null;
    busy = true; error = ''; reportBusy();
    try {
      if (extended && requestedOptions) {
        const value = await reads.track(SiteUnitSummaryExtendedWorkbookService.GetReview(owned.contextId,
          JSON.stringify(requestedOptions)));
        if (disposed || requestGeneration !== generation || method !== requestedMethod ||
            !sameExtendedSummaryWorkbookOptions(requestedOptions, extendedOptions ?? null)) return;
        review = validateExtendedSummaryWorkbookReview(value, owned, requestedOptions);
      } else {
        const value = await reads.track(SiteUnitSummaryWorkbookService.GetReview(owned.contextId,
          JSON.stringify({ method: requestedMethod })));
        if (disposed || requestGeneration !== generation || method !== requestedMethod) return;
        review = validateSiteUnitSummaryWorkbookReview(value, owned, requestedMethod);
      }
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
      if ('environment' in review && extendedSession && extendedOptions) {
        await extendedSession.publish(review, destination, request =>
          SiteUnitSummaryExtendedWorkbookService.ExportReviewed(owned.contextId, JSON.stringify(request)), extendedOptions);
      } else if ('preview' in review && originalSession) {
        await originalSession.publish(review, destination, request =>
          SiteUnitSummaryWorkbookService.ExportReviewed(owned.contextId, JSON.stringify(request)), method);
      }
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
    <p>Workbook publication is unavailable. The independent {extended ? 'extended Summary' : 'Summary Environment'} workbook feature is disabled.</p>
  {/if}
  {#if review && !currentReview}
    <p role="alert" class="error">The selected method or criteria changed. Review the current options before publishing; the prior review cannot be used.</p>
  {/if}
  <p>Requested method: {method === 1 ? 'Mean (1)' : method === 2 ? 'Interquartile (2)' : 'Unavailable'}.</p>
  <div class="actions">
    <button data-site-unit-workbook-review disabled={!enabled || held || !normalOwner || !validMethod || (extended && !extendedOptions)}
      onclick={getReview}>Review summary workbook</button>
    <button data-site-unit-workbook-cancel disabled={!busy || barrier} onclick={cancel}>Cancel review</button>
  </div>
  {#if busy}<p role="status">Reading the source workbook plan…</p>{/if}
  {#if review && currentReview && reviewedPreview}
    <div class="plan">
      <h3>Reviewed source plan</h3>
      <p>Project: {reviewedPreview.report.project}. Site Unit Table: {reviewedPreview.report.su}.
        Method: {reviewedPreview.report.method === 1 ? 'Mean (1)' : 'Interquartile (2)'}.
        Saved method: {review.scope.savedMethod}. Normal Site Unit scope: {review.scope.siteUnitType}.
        Saved order: {review.scope.orderBy}. Saved species included: {review.scope.includeSpecies}.</p>
      {#if 'options' in review}
        <p>Requested grouping: {review.options.orderBy === 1 ? 'Layers' : 'Lifeforms'}.
          Species included: {review.options.includeSpecies}.
          {#if review.options.includeSpecies === 1}Cover calculation: {review.options.coverCalculation === 1 ? 'All plots' : 'Present values'}.
            Criteria: {review.options.andOr === 1 ? 'AND' : 'OR'}; presence &gt; {review.options.presenceGreaterThan};
            cover &gt; {review.options.coverGreaterThan}.{/if}</p>
      {/if}
      <p>Original memberships: {reviewedPreview.report.memberships.length}.
        Joined rows: {reviewedPreview.report.units.reduce((sum, unit) => sum + unit.plots.length, 0)}.
        Worksheets: {review.sheets.length}. Planned bytes: {review.bytes}.</p>
      {#each reviewedPreview.report.units as unit, unitIndex}
        <section class="unit" aria-label={`Site Unit ${unit.code || '(empty)'}`}>
          <h4>Site Unit: {unit.code === '' ? '(empty)' : unit.code}</h4>
          <p>Source long name: {unit.longName === null ? '(NULL / missing)' : unit.longName === '' ? '(empty)' : unit.longName}.
            Name status: {unit.nameStatus}. Name candidates: {unit.nameCandidates.length}.
            Joined rows: {unit.plots.length}. Worksheet: {review.sheets[unitIndex].name}.</p>
          {#if reviewedSpecies}
            {#each reviewedSpecies.units[unitIndex].groups as group}
              <h5>{group.caption}</h5>
              {#if group.rows.some(row => row.included)}
                <div class="species-table"><table>
                  <thead><tr><th scope="col">Scientific Name</th><th scope="col">Common Name</th><th scope="col">%Cover</th><th scope="col">%Presence</th></tr></thead>
                  <tbody>{#each group.rows.filter(row => row.included) as row}
                    <tr><th scope="row" aria-label={`${reportCellText(row.scientificName)} (${row.species})`}>{reportCellText(row.scientificName)}</th>
                      <td>{reportCellText(row.englishName)}</td><td>{row.cover}</td><td>{row.presence}</td></tr>
                  {/each}</tbody>
                </table></div>
              {:else}<p>No species pass the strict formatted criteria.</p>{/if}
            {/each}
          {/if}
          <div class="summaries">
            {#each ['SITE', 'VEGETATION', 'SOILS'] as section}
              <table>
                <caption>{section} — {unit.code === '' ? '(empty unit)' : unit.code}</caption>
                <thead><tr><th scope="col">Summary field</th><th scope="col">Source value</th></tr></thead>
                <tbody>
                  {#each reviewedPreview.report.fields as field, index}
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
    {#if extended}Normal Site Unit summaries use the explicit grouping, inclusion and species criteria above.
    {:else}Only normal Site Unit layer-cover summaries without species are available.{/if} Enter an explicit .xlsx path;
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
