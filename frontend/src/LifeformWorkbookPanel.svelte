<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { LifeformWorkbookService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { lifeformCellText, lifeformRatioText, type LifeformSummaryOwner } from './lifeformSummary';
  import { speciesAttributeCountText } from './speciesAttributeSummary';
  import {
    lifeformWorkbookOptions, lifeformWorkbookPublicationSession, validateLifeformWorkbookReview,
    type ValidatedLifeformWorkbookReview,
  } from './lifeformWorkbook';

  let { owner, disabled, onBusyChange }: {
    owner: LifeformSummaryOwner; disabled: boolean; onBusyChange: (held: boolean) => void;
  } = $props();
  const enabled = import.meta.env.VITE_LIFEFORM_WORKBOOK === 'true';
  const owned = untrack(() => structuredClone(owner));
  const session = lifeformWorkbookPublicationSession(owned);
  const reads = new ReadRequests();
  let generation = 0;
  let disposed = false;
  let busy = $state(false);
  let error = $state('');
  let details = $state<boolean[]>([false, false, false, false, false, false]);
  let review = $state.raw<ValidatedLifeformWorkbookReview | null>(null);
  let destination = $state('');
  let publication = $state(session.view());
  const barrier = $derived(publication.busy || publication.blocked);
  const held = $derived(disabled || busy || barrier);
  function reportBusy() { onBusyChange(busy || publication.busy || publication.blocked); }
  const unsubscribe = session.subscribe(() => {
    publication = session.view();
    if (publication.busy) review = null;
    reportBusy();
  });

  function cancel() {
    generation++; reads.cancelAll(); busy = false; review = null; reportBusy();
  }
  function changeDetail(index: number, selected: boolean) {
    if (!enabled || held) return;
    cancel(); details = details.map((value, i) => i === index ? selected : value); error = '';
  }
  async function getReview() {
    if (!enabled || held || owned.su === 'None' || owned.su === 'USysSuTableDynamic') return;
    cancel();
    const requestGeneration = generation, requestedDetails = [...details];
    busy = true; error = ''; reportBusy();
    try {
      const value = await reads.track(LifeformWorkbookService.GetReview(owned.contextId, JSON.stringify({ details: requestedDetails })));
      if (disposed || requestGeneration !== generation) return;
      review = validateLifeformWorkbookReview(value, owned, requestedDetails);
    } catch (cause) {
      if (!disposed && requestGeneration === generation) error = `Workbook review unavailable; no file published: ${String(cause)}`;
    } finally {
      if (!disposed && requestGeneration === generation) { busy = false; reportBusy(); }
    }
  }
  async function publish() {
    if (!enabled || held || !review) return;
    error = '';
    try {
      await session.publish(review, destination, request =>
        LifeformWorkbookService.ExportReviewed(owned.contextId, JSON.stringify(request)));
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

<section class="workbook" data-lifeform-workbook aria-label="Lifeform Summary workbook">
  <h2>Lifeform Summary workbook</h2>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
  {#if publication.busy}
    <p role="status">Publishing the reviewed workbook; publication cannot be cancelled. Navigation and close remain blocked.</p>
  {/if}
  {#if publication.outcome}
    <p data-lifeform-workbook-receipt role="status">Workbook receipt: {publication.outcome.status}.
      Requested destination: {publication.requestedDestination}. Output: {publication.outcome.path || '(no published path)'}.
      SHA256: {publication.outcome.sha256 || '(no acknowledged hash)'}.</p>
  {:else if publication.requestedDestination && !publication.busy}
    <p data-lifeform-workbook-unknown role="status">Workbook outcome unknown. Retain and inspect
      {publication.requestedDestination} before acknowledging; never repeat the destination blindly.</p>
  {/if}
  {#if publication.blocked && !publication.busy}
    <button data-lifeform-workbook-acknowledge onclick={acknowledge}>Acknowledge workbook outcome</button>
  {/if}
  {#if !enabled}
    <p>Workbook publication is unavailable. The independent Lifeform workbook feature is disabled.</p>
  {/if}
  <fieldset disabled={!enabled || held}>
    <legend>Optional detail tables (all six count summaries are always included)</legend>
    {#each lifeformWorkbookOptions as option, i}
      <label class="option">
        <input type="checkbox" data-lifeform-workbook-option={option.field}
          checked={details[i]} onchange={event => changeDetail(i, event.currentTarget.checked)} />
        {option.label}
      </label>
    {/each}
  </fieldset>
  <div class="actions">
    <button data-lifeform-workbook-review disabled={!enabled || held || owned.su === 'None' || owned.su === 'USysSuTableDynamic'}
      onclick={getReview}>Review summary workbook</button>
    <button data-lifeform-workbook-cancel disabled={!busy || barrier} onclick={cancel}>Cancel review</button>
  </div>
  {#if busy}<p role="status">Reading the source workbook plan…</p>{/if}
  {#if review}
    <div data-lifeform-workbook-plan>
      <h3>Reviewed source plan</h3>
      <p>Project: {review.lifeform.report.project}. Site Unit Table: {review.lifeform.report.su}.</p>
      <p>Planned bytes: {review.bytes}. SHA256: {review.workbookSHA256}. Approval: {review.approvalHash}.</p>
      {#each review.lifeform.report.units as unit, i}
        {@const attributeUnit = review.attributes.report.units[i]}
        <section aria-label={`Source unit ${lifeformCellText(unit.code)}`}>
          <h4>Unit {lifeformCellText(unit.code)} → worksheet {review.sheets[i].name}</h4>
          <p>Physical plots: {unit.nPlots}. Unique species: {unit.uniqueSpecies}. Total species occurrences: {unit.occurrences}.</p>
          <div class="table-scroll">
            <table>
              <caption>All six source attribute totals</caption>
              <thead><tr><th>Attribute</th><th>Count</th><th>Number of plot occurrences</th></tr></thead>
              <tbody>{#each attributeUnit.rows as row, j}
                <tr><th>{review.attributes.report.definitions[j].label}</th>
                  <td>{speciesAttributeCountText(row.count)}</td><td>{row.plotOccurrences}</td></tr>
              {/each}</tbody>
            </table>
            <table>
              <caption>Source lifeform summary</caption>
              <thead><tr><th>Lifeform</th><th>Presence</th><th>Mean Cover</th></tr></thead>
              <tbody>{#each unit.rows as row}
                <tr><th>{row.lifeform}</th><td>{lifeformRatioText(row.presence)}</td><td>{lifeformRatioText(row.meanCover)}</td></tr>
              {/each}</tbody>
            </table>
            {#each review.details as selected, j}
              {#if selected}
                <table>
                  <caption>{lifeformWorkbookOptions[j].label}</caption>
                  <thead><tr>{#each review.attributes.report.definitions[j].categories as category}<th>{category}</th>{/each}</tr></thead>
                  <tbody><tr>{#each attributeUnit.rows[j].categories as count}<td>{speciesAttributeCountText(count)}</td>{/each}</tr></tbody>
                </table>
              {/if}
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {/if}
  <label for="lifeform-workbook-destination">New XLSX output file (existing files are never replaced)</label>
  <textarea id="lifeform-workbook-destination" data-lifeform-workbook-destination rows="2"
    disabled={!enabled || held} bind:value={destination}></textarea>
  <button data-lifeform-workbook-export disabled={!enabled || held || !review || !destination}
    onclick={publish}>Export reviewed summary workbook</button>
  <p class="guidance">Review reads source snapshots without writing project data. Detail options default to off;
    they do not remove the six count totals. Enter the exact literal .xlsx destination without trimming.
    Export rechecks the approved source and never replaces an existing file. A settled publication must be acknowledged before another review or export.</p>
</section>

<style>
  .workbook { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; overflow-wrap: anywhere; }
  fieldset { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr)); gap: .5rem; min-width: 0; }
  .option { display: flex; align-items: center; gap: .5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; }
  input, textarea { box-sizing: border-box; min-width: 0; }
  textarea { width: 100%; }
  .table-scroll { overflow-x: auto; }
  table { border-collapse: collapse; margin-bottom: .75rem; }
  th, td { padding: .35rem .65rem; border: 1px solid #64748b; text-align: left; }
  caption { text-align: left; font-weight: 600; }
  .error { color: #b91c1c; font-weight: 600; }
  .guidance { font-size: .9rem; }
</style>
