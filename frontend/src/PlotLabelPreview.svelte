<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { ContextService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { plotLookupInputError } from './scopedPlotLookup';
  import { plotLabelFromHeader, type PlotLabelPreview } from './plotLabelPreview';

  let { contextId, project, initialPlot = '', onBusyChange }: {
    contextId: string; project: string; initialPlot?: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  let plot = $state(untrack(() => initialPlot));
  let preview = $state<PlotLabelPreview | null>(null);
  let labelDate = $state('');
  let error = $state('');
  let busy = $state(false);
  const reads = new ReadRequests();
  const ownerKey = $derived(JSON.stringify([contextId, project]));
  let generation = 0;

  function cancel() {
    generation++;
    reads.cancelAll();
    busy = false;
    onBusyChange(false);
  }
  async function load() {
    if (busy) { error = 'Wait for or cancel the current saved-label read before another preview.'; return; }
    const invalid = plotLookupInputError(plot);
    if (invalid) { error = invalid; return; }
    cancel();
    const request = generation, key = ownerKey, requestedPlot = plot;
    preview = null; error = ''; labelDate = ''; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.GetPlot(contextId, requestedPlot));
      if (request !== generation || key !== ownerKey) return;
      const candidate = plotLabelFromHeader(value, requestedPlot);
      const date = new Date().toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
      preview = candidate; labelDate = date;
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Plot label preview unavailable; saved data and printing are unchanged: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="labels" data-plot-label-preview aria-label="VPro Plot Labels">
  <h1>VPro Plot Labels</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Owned project {project}. Preview an exact saved plot, not unsaved form controls.</p>
  <form onsubmit={event => { event.preventDefault(); void load(); }}>
    <label for="plot-label-number">Plot number
      <input id="plot-label-number" bind:value={plot} disabled={busy} />
    </label>
    <button type="submit" disabled={busy}>Preview saved plot label</button>
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
    </details>
  {/if}
  <p class="guidance">Read-only adaptation of the single-plot label preview. No BecLabels rows, database, audits,
    configuration, files or printer jobs are changed. Source report designs and printer geometry are not available;
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
  h2 { font-size: 1.25rem; font-weight: 600; }
  p, dd { white-space: pre-wrap; overflow-wrap: anywhere; }
  .zone { font-family: monospace; }
  footer { display: flex; justify-content: space-between; flex-wrap: wrap; gap: .5rem; }
  dl > div { margin: .5rem 0; }
  dt { font-weight: 600; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .guidance { margin-top: 1rem; font-size: .8rem; color: #475569; }
</style>
