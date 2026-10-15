<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { ContextService, PlotLabelRegionalService, PlotLabelPDFExportService } from '../bindings/github.com/boostao/vpro-wails';
  import type { PlotLabelPDFExportReview, PlotLabelPDFExportOutcome } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { plotLookupInputError } from './scopedPlotLookup';
  import { plotLabelFromHeader, type PlotLabelPreview } from './plotLabelPreview';
  import { validatePlotLabelRegionalPreview, type PlotLabelRegionalEvidence } from './plotLabelRegional';
  import { validatePlotLabelPDFReview, plotLabelPDFPublicationSession } from './plotLabelPDFExport';
  import type { PublicationView } from './publicationSession';

  let { contextId, project, projectPath = '', su = '', suPath = '', initialPlot = '', onBusyChange }: {
    contextId: string; project: string; projectPath?: string; su?: string; suPath?: string; initialPlot?: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let plot = $state(untrack(() => initialPlot));
  let preview = $state<PlotLabelPreview | null>(null);
  let labelDate = $state('');
  let regionalEvidence = $state.raw<PlotLabelRegionalEvidence | null>(null);
  let error = $state('');
  let busy = $state(false);
  let pdfReview = $state.raw<PlotLabelPDFExportReview | null>(null);
  let destination = $state('');
  const reads = new ReadRequests();
  const regionalDateEnabled = import.meta.env.VITE_PLOT_LABEL_REGIONAL_DATE === 'true';
  const pdfEnabled = import.meta.env.VITE_PLOT_LABEL_PDF_EXPORT === 'true';
  const pdfSession = untrack(() => pdfEnabled ? plotLabelPDFPublicationSession({ contextId, project, projectPath, su, suPath }) : null);
  let publication = $state.raw<PublicationView<PlotLabelPDFExportOutcome>>(pdfSession?.view() ??
    { busy: false, blocked: false, outcome: null, error: '', requestedDestination: '' });
  const locked = $derived(busy || publication.busy || publication.blocked);
  const ownerKey = $derived(JSON.stringify([contextId, project, projectPath, su, suPath]));
  let generation = 0;
  function updateBusy() { onBusyChange(busy || publication.busy || publication.blocked); }
  const unsubscribe = pdfSession?.subscribe(() => { publication = pdfSession.view(); updateBusy(); });

  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    updateBusy();
  }
  async function load() {
    if (busy || publication.busy || publication.blocked) { error = 'Resolve the current saved-label read or retained PDF outcome before another preview.'; return; }
    const invalid = plotLookupInputError(plot);
    if (invalid) { error = invalid; return; }
    cancel();
    const request = generation, key = ownerKey, requestedPlot = plot;
    preview = null; error = ''; labelDate = ''; regionalEvidence = null; pdfReview = null; busy = true; updateBusy();
    try {
      if (pdfEnabled) {
        const value = await reads.track(PlotLabelPDFExportService.GetReview(contextId, JSON.stringify({ plotNumber: requestedPlot })));
        if (request !== generation || key !== ownerKey) return;
        const candidate = validatePlotLabelPDFReview(value, { contextId, project, projectPath, su, suPath }, requestedPlot);
        const native = validatePlotLabelRegionalPreview(candidate.preview, contextId, requestedPlot);
        if (request !== generation || key !== ownerKey) return;
        pdfReview = candidate; preview = native.label; labelDate = native.labelDate; regionalEvidence = native.evidence;
      } else if (regionalDateEnabled) {
        const value = await reads.track(PlotLabelRegionalService.GetPreview(contextId, requestedPlot));
        if (request !== generation || key !== ownerKey) return;
        const candidate = validatePlotLabelRegionalPreview(value, contextId, requestedPlot);
        if (request !== generation || key !== ownerKey) return;
        preview = candidate.label; labelDate = candidate.labelDate; regionalEvidence = candidate.evidence;
      } else {
        const value = await reads.track(ContextService.GetPlot(contextId, requestedPlot));
        if (request !== generation || key !== ownerKey) return;
        const candidate = plotLabelFromHeader(value, requestedPlot);
        const date = new Date().toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
        preview = candidate; labelDate = date;
      }
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Plot label preview unavailable; saved data and printing are unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; updateBusy(); }
    }
  }
  async function publishPDF() {
    if (!pdfSession || !pdfReview || busy || publication.busy || publication.blocked) { error = 'Prepare an owned PDF review and resolve the prior operation first.'; return; }
    error = '';
    try {
      await pdfSession.publish(pdfReview, destination, request => PlotLabelPDFExportService.ExportReviewed(contextId, JSON.stringify(request)));
    } catch (cause) { error = `Label PDF publication unavailable: ${String(cause)}`; }
  }
  async function recoverPDF() {
    if (!pdfSession) return;
    error = '';
    try {
      await pdfSession.recover(request => PlotLabelPDFExportService.RecoverOutcome(contextId, JSON.stringify(request)));
    } catch (cause) { error = `Readonly PDF recovery unavailable: ${String(cause)}`; }
  }
  function acknowledgePDF() {
    if (!pdfSession) return;
    try { pdfSession.acknowledge(); } catch (cause) { error = String(cause); }
  }
  onDestroy(() => { unsubscribe?.(); cancel(); });
</script>

<section class="labels" data-plot-label-preview aria-label="VPro Plot Labels">
  <h1>VPro Plot Labels</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Owned project {project}. Preview an exact saved plot, not unsaved form controls.</p>
  <form onsubmit={event => { event.preventDefault(); void load(); }}>
    <label for="plot-label-number">Plot number
      <input id="plot-label-number" bind:value={plot} disabled={locked} />
    </label>
    <button type="submit" disabled={locked}>{pdfEnabled ? 'Review saved plot label PDF' : 'Preview saved plot label'}</button>
    {#if busy}<button type="button" onclick={cancel}>Cancel label read</button><span role="status">Reading saved label fields...</span>{/if}
  </form>
  {#if preview}
    <section class="label" data-plot-label-card aria-label={`Label for saved plot ${preview.plotNumber1}`}>
      <h2>{preview.displayPlotNumber}</h2>
      <p class="zone">{preview.zone1}</p>
      <p>{preview.projectId1 ?? ''}</p>
      <p>{preview.displayRepresenting ?? ''}</p>
      <footer><span>Label Date: {labelDate}</span><span>(VPro)</span></footer>
    </section>
    <details>
      <summary>Original label assignments and display rules</summary>
      <dl>
        <div><dt>PlotNumber1</dt><dd>{preview.plotNumber1}</dd></div>
        <div><dt>Zone1</dt><dd>{preview.zone1}</dd></div>
        <div><dt>Representing1</dt><dd>{preview.representing1 === null ? '(NULL)' : preview.representing1 === '' ? '(empty text)' : preview.representing1}</dd></div>
        <div><dt>ProjectID1</dt><dd>{preview.projectId1 === null ? '(NULL)' : preview.projectId1 === '' ? '(empty text)' : preview.projectId1}</dd></div>
      </dl>
      <p>Only the source display expressions remove leading/trailing ASCII spaces from PlotNumber and Representing.
        Assignments retain exact text/NULL. Label date uses the desktop locale and current clock.</p>
      {#if regionalEvidence}
        <p data-plot-label-regional-date>Native Windows Long Date: {regionalEvidence.isoDate}, locale {regionalEvidence.locale},
          pattern {regionalEvidence.pattern}. No browser-locale fallback.</p>
      {/if}
    </details>
  {/if}
      {#if pdfEnabled}
        <section class="pdf" data-plot-label-pdf-export aria-label="Portable saved label PDF">
          <h2>Portable saved label PDF</h2>
          {#if publication.error}<p role="alert" class="error">{publication.error}</p>{/if}
          {#if pdfReview}
            <dl>
              <div><dt>Source approval</dt><dd>{pdfReview.approvalHash}</dd></div>
              <div><dt>PDF SHA256</dt><dd>{pdfReview.pdfSHA256}</dd></div>
            </dl>
            <label for="plot-label-pdf-destination">New PDF destination
              <input id="plot-label-pdf-destination" bind:value={destination} disabled={locked} />
            </label>
            <button type="button" disabled={locked || !destination} onclick={() => void publishPDF()}>Export reviewed label PDF</button>
          {/if}
          {#if publication.busy}<p role="status">Retaining the exact PDF attempt; publication is not replayed or cancelled.</p>{/if}
          {#if publication.requestedDestination}
            <p data-plot-label-pdf-destination>Requested destination: {publication.requestedDestination}</p>
          {/if}
          {#if publication.outcome}
            <dl data-plot-label-pdf-receipt>
              <div><dt>Publication outcome</dt><dd>{publication.outcome.status}</dd></div>
              <div><dt>Published path</dt><dd>{publication.outcome.path || '(no published path)'}</dd></div>
              <div><dt>Artifact SHA256</dt><dd>{publication.outcome.sha256 || '(no artifact hash)'}</dd></div>
            </dl>
          {/if}
          {#if publication.blocked && !publication.outcome}
            <button type="button" disabled={publication.busy} onclick={() => void recoverPDF()}>Recover PDF outcome readonly</button>
          {/if}
          {#if publication.blocked && publication.outcome}
            <button type="button" disabled={publication.busy} onclick={acknowledgePDF}>Acknowledge retained PDF outcome</button>
          {/if}
          <p class="guidance">One portable Letter page, real runtime Arial and fixed source-relative fields, not Access printer/stock alignment.
            Only calibrated en-US dates and unclipped single-line printable ASCII are supported. Export never overwrites a file or changes saved data.
            Retained receipts survive remount; unknown attempts are recovered readonly, never replayed. Print All and physical printing remain unavailable.</p>
        </section>
      {/if}
  <p class="guidance">Read-only adaptation of the single-plot label preview. No BecLabels rows, database, audits,
    configuration or printer jobs are changed by preview. {pdfEnabled ? 'The separate reviewed PDF action creates one new file only.' : 'No files are created.'} Source report designs are mapped; physical printer output is not verified;
    six-position printing and Print All Plots remain unavailable. SU/profile filters are not applied to this exact lookup.</p>
</section>

<style>
  .labels { padding: 1rem; min-width: 0; max-width: 100%; }
  h1 { font-size: 1.5rem; font-weight: 600; }
  form { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin: 1rem 0; }
  label { display: grid; gap: .3rem; min-width: 0; flex: 1; }
  input, button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; min-width: 0; }
  input:disabled, button:disabled { opacity: .5; }
  .label { border: 1px solid #64748b; padding: 1rem; margin: 1rem 0; display: grid; gap: .5rem; min-width: 0; }
  .pdf { display: grid; grid-template-columns: minmax(0, 1fr); gap: .6rem; min-width: 0; }
  .pdf input { width: 100%; box-sizing: border-box; }
  h2 { font-size: 1.25rem; font-weight: 600; }
  p, dd { white-space: pre-wrap; overflow-wrap: anywhere; }
  .zone { font-family: monospace; }
  footer { display: flex; justify-content: space-between; flex-wrap: wrap; gap: .5rem; }
  footer span { min-width: 0; overflow-wrap: anywhere; white-space: pre-wrap; }
  dl > div { margin: .5rem 0; }
  dt { font-weight: 600; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .guidance { margin-top: 1rem; font-size: .8rem; color: #475569; }
</style>
