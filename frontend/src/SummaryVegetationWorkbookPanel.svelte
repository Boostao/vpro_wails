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
  const ancillaryEnabled = import.meta.env.VITE_SUMMARY_VEGETATION_ANCILLARY_WORKBOOK === 'true';
  const referencesEnabled = import.meta.env.VITE_SUMMARY_VEGETATION_EFFECTIVE_REFERENCES === 'true';
  const groupedEnabled = import.meta.env.VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK === 'true';
  const environmentEnabled = summaryEnvironmentalLifeformEnabled;
  let viewState = $state.raw(session.view());
  let error = $state('');
  let unitPage = $state(0);
  let referencePage = $state(0);
  const unitPageSize = 50;
  const barrier = $derived(viewState.publication.busy || viewState.publication.blocked);
  const held = $derived(disabled || viewState.reviewing || barrier);
  const grouped = $derived(calculation.groupAndOrder !== undefined);
  const groupingUnavailable = $derived(grouped && (!groupedEnabled || viewState.includeReferences || viewState.includeAncillary));
  const groupLabel = $derived(!grouped ? 'Layer' : calculation.groupAndOrder === 12 ? 'Strata' :
    calculation.groupAndOrder === 13 ? 'Lifeform' : calculation.groupAndOrder === 14 ? 'None' : 'Unsupported');
  const currentReview = $derived(!!viewState.review && sameSummaryWorkbookOptions(viewState.review.options, {
    average: calculation.average, display: calculation.display, showClass: calculation.showClass,
    title: viewState.title, includeEnv: viewState.includeEnv,
    ...(grouped ? { groupAndOrder: calculation.groupAndOrder } : {}),
    ...(viewState.includeAncillary ? { ancillary: { createSummary: viewState.createSummary } } : {}),
    ...(viewState.includeReferences ? { references: {
      showCommon: viewState.showCommon, useGuideNames: viewState.useGuideNames } } : {}),
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
  function selectAncillary(field: 'sheets' | 'summary', event: Event) {
    if (held || !ancillaryEnabled) return;
    const checked = (event.currentTarget as HTMLInputElement).checked;
    error = '';
    try {
      if (field === 'sheets') session.includeAncillary(checked);
      else session.includeSummary(checked);
    } catch (cause) { error = String(cause); }
  }
  function undo() {
    if (held) return;
    error = '';
    try { session.undoDraft(); } catch (cause) { error = String(cause); }
  }
  function selectReferences(field: 'include' | 'common' | 'guide', event: Event) {
    if (held || !referencesEnabled) return;
    const checked = (event.currentTarget as HTMLInputElement).checked;
    error = '';
    try { session.selectReferences(field, checked); } catch (cause) { error = String(cause); }
  }
  async function review() {
    if (!enabled || held || groupingUnavailable || viewState.draftError || viewState.includeEnv && !environmentEnabled ||
        viewState.includeAncillary && !ancillaryEnabled || viewState.includeReferences && !referencesEnabled) return;
    error = '';
    unitPage = 0;
    referencePage = 0;
    try {
      await session.review(calculation, {
        review: (contextId, options) => SummaryVegetationWorkbookService.GetReview(contextId, JSON.stringify(options)),
        validationYield: () => new Promise(resolve => setTimeout(resolve, 0)),
      });
    } catch (cause) { error = String(cause); }
  }
  async function publish() {
    if (!enabled || held || groupingUnavailable || viewState.draftError || !currentReview ||
        viewState.includeReferences && !referencesEnabled) return;
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
  <p data-summary-workbook-group>Workbook grouping: {groupLabel}.
    {#if groupedEnabled}Uses the Summary Group by selection.
    {:else}Layer only; grouped workbook publication is disabled in this build.{/if}</p>
  {#if groupingUnavailable}
    <p role="alert">Grouped workbook publication is unavailable with effective references or source worksheets,
      or when its independent build flag is disabled. Clear those workbook modes or choose Layer.
      No options were changed automatically.</p>
  {/if}
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
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-ancillary checked={viewState.includeAncillary}
        disabled={!enabled || held || !ancillaryEnabled} onchange={event => selectAncillary('sheets', event)} />
      Include source worksheets (SU Table)
    </label>
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-summary checked={viewState.createSummary}
        disabled={!enabled || held || !ancillaryEnabled || !viewState.includeAncillary}
        onchange={event => selectAncillary('summary', event)} />
      Include ReportSummary counts, unit names and version
    </label>
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-references checked={viewState.includeReferences}
        disabled={!enabled || held || !referencesEnabled} onchange={event => selectReferences('include', event)} />
      Project effective source references and attribute descriptions
    </label>
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-common checked={viewState.showCommon}
        disabled={!enabled || held || !referencesEnabled || !viewState.includeReferences || viewState.useGuideNames}
        onchange={event => selectReferences('common', event)} />
      Include common names in grouping and the workbook
    </label>
    <label class="checkbox">
      <input type="checkbox" data-summary-workbook-guide checked={viewState.useGuideNames}
        disabled={!enabled || held || !referencesEnabled || !viewState.includeReferences}
        onchange={event => selectReferences('guide', event)} />
      Use master CombinedEnglishName (also includes common names)
    </label>
    <label>New workbook destination (.xlsx)
      <input data-summary-workbook-destination value={viewState.destination} disabled={!enabled || held}
        oninput={event => edit('destination', event.currentTarget.value)} />
    </label>
  </div>
  <div class="actions">
    <button data-summary-workbook-review disabled={!enabled || held || groupingUnavailable || !!viewState.draftError} onclick={review}>Review inspection workbook</button>
    <button data-summary-workbook-cancel disabled={!viewState.reviewing || barrier} onclick={() => session.cancel()}>Cancel review</button>
    <button data-summary-workbook-undo disabled={!enabled || held} onclick={undo}>Undo export options</button>
    <button data-summary-workbook-publish disabled={!enabled || held || groupingUnavailable || !!viewState.draftError || !currentReview || !viewState.destination}
      onclick={publish}>Publish reviewed workbook</button>
  </div>
  {#if viewState.reviewing}<p role="status">Reading and validating the complete owned workbook plan...</p>{/if}
  {#if viewState.review}
    <div data-summary-workbook-plan class="plan">
      {#if !currentReview}<p role="alert">Calculation options changed; review the current options before publication.</p>{/if}
      <p>Sheet: {viewState.review.sheet}. Group field: {viewState.review.preview.groupField?.name ?? 'Layer'}.
        {viewState.review.preview.rows.length} vegetation rows;
        {viewState.review.environment?.rows.length ?? 0} environmental rows; first vegetation row {viewState.review.firstVegetationRow}.
        {viewState.review.bytes} bytes.</p>
      <p>Approval: {viewState.review.approvalHash}<br />Workbook SHA256: {viewState.review.workbookSHA256}</p>
      {#if viewState.review.preview.references}
        {@const reference = viewState.review.preview.references}
        <div data-summary-workbook-reference-plan class="plan">
          <p>Effective references: {reference.masterRows} physical master rows;
            {reference.removedMasterRows} old attribute rows excluded; {reference.personalRows} personal rows.
            {reference.derivedRows} attribute rows projected from {reference.descriptionRows} description rows;
            {reference.unionRows} distinct literal reference rows.
            Common names: {reference.showCommon ? 'included' : 'not included'}.
            Guide names: {reference.useGuideNames ? 'master CombinedEnglishName' : 'not selected'}.</p>
          <p>Derived origins identify physical attribute-description rows, not invented master records.
            Name groups are shown below; cover values are in the workbook. NULL and empty names remain distinct.</p>
          <div class="unit-table">
            <table aria-label="Reviewed effective reference groups">
              <thead><tr><th scope="col">Layer</th><th scope="col">Scientific name/code</th>
                {#if reference.showCommon}<th scope="col">Common name</th>{/if}<th scope="col">Order</th></tr></thead>
              <tbody>{#each viewState.review.preview.rows.slice(referencePage * unitPageSize, (referencePage + 1) * unitPageSize) as row}
                <tr><td>{row.layer.text === null ? '(NULL)' : row.layer.text === '' ? '(empty)' : row.layer.text}</td>
                  <th scope="row">{row.species.text === null ? '(NULL)' : row.species.text === '' ? '(empty)' : row.species.text}</th>
                  {#if reference.showCommon}
                    {#if row.commonName}
                      <td>{row.commonName.text === null ? '(NULL)' : row.commonName.text === '' ? '(empty)' : row.commonName.text}</td>
                    {:else}
                      <td><span role="alert">Reviewed common name is missing.</span></td>
                    {/if}
                  {/if}
                  <td>{row.order.text === null ? '(NULL)' : row.order.text === '' ? '(empty)' : row.order.text}</td></tr>
              {/each}</tbody>
            </table>
          </div>
          <div class="actions">
            <button data-summary-reference-previous disabled={held || referencePage === 0}
              onclick={() => referencePage--}>Previous name groups</button>
            <span>{viewState.review.preview.rows.length} name groups; page {referencePage + 1} of
              {Math.max(1, Math.ceil(viewState.review.preview.rows.length / unitPageSize))}.</span>
            <button data-summary-reference-next disabled={held || (referencePage + 1) * unitPageSize >= viewState.review.preview.rows.length}
              onclick={() => referencePage++}>Next name groups</button>
          </div>
        </div>
      {/if}
      {#if viewState.review.ancillary}
        <div data-summary-workbook-ancillary-plan class="plan">
          <p>Source sheets: {viewState.review.ancillary.sheets.join(', ')}.
            Reviewed creation time: {viewState.review.ancillary.createdAt}.
            {viewState.review.ancillary.membershipRows} physical SU membership rows.</p>
          {#if viewState.review.ancillary.summary}
            {@const summary = viewState.review.ancillary.summary}
            <p>Plots in database: {summary.environmentCountAvailable ? summary.environmentRows : 'N/A'}.
              Non-NULL selected plot rows: {summary.selectedPlotRows}.
              AllSpecs version: {summary.version === '' ? '(empty)' : summary.version} ({summary.versionStatus}).</p>
            <div class="unit-table">
              <table aria-label="Reviewed ReportSummary units">
                <thead><tr><th scope="col">Short name</th><th scope="col">Units</th><th scope="col">Long name</th></tr></thead>
                <tbody>{#each summary.units.slice(unitPage * unitPageSize, (unitPage + 1) * unitPageSize) as unit (unit.code)}
                  <tr><th scope="row">{unit.code === '' ? '(empty)' : unit.code}</th><td>{unit.numPlots}</td>
                    <td>{unit.longName === null ? '(NULL)' : unit.longName === '' ? '(empty)' : unit.longName}</td></tr>
                {/each}</tbody>
              </table>
            </div>
            <div class="actions">
              <button data-summary-units-previous disabled={held || unitPage === 0}
                onclick={() => unitPage--}>Previous units</button>
              <span>{summary.units.length} units; page {unitPage + 1} of {Math.max(1, Math.ceil(summary.units.length / unitPageSize))}.</span>
              <button data-summary-units-next disabled={held || (unitPage + 1) * unitPageSize >= summary.units.length}
                onclick={() => unitPage++}>Next units</button>
            </div>
          {/if}
        </div>
      {/if}
      <p>{viewState.review.scope}</p>
    </div>
  {/if}
  <p>Explicit Layer inspection export using Calculate, Display Values and Show above, plus these title/environment options.
    Saved presentation callbacks are not applied. Source data, preferences and audit history are not changed.
    This is not the original Access Report action: quality/lumping callbacks, original formatting, Excel coercion,
    printing and macros remain unavailable. Optional attribute/common/guide projection is read-only and uses literal
    comparison, not Access collation or physical preflight mutations. Source worksheets are a separate inspection
    option. Existing destinations are never replaced.</p>
  {#if !ancillaryEnabled}<p>Source worksheet publication is disabled in this build.</p>{/if}
  {#if !referencesEnabled}<p>Effective reference projection is disabled in this build.</p>{/if}
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
  .unit-table { min-width: 0; overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: .5rem; border: 1px solid #d1d5db; text-align: left; overflow-wrap: anywhere; }
  [role="alert"] { color: var(--color-red-700, #b91c1c); }
</style>
