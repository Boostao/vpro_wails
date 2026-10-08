<script lang="ts">
  import { untrack } from 'svelte';
  import PicturePanel from './PicturePanel.svelte';
  import { plotLookupInputError } from './scopedPlotLookup';

  let { contextId, project, initialPlot = '', onBusyChange }: {
    contextId: string; project: string; initialPlot?: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const startingPlot = untrack(() => initialPlot);
  let plot = $state(startingPlot);
  let selected = $state(startingPlot);
  let error = $state('');
  let busy = $state(false);

  function selectPlot() {
    if (busy) { error = 'Wait for or cancel the current picture read before selecting another plot.'; return; }
    const invalid = plotLookupInputError(plot);
    if (invalid) { error = invalid; return; }
    error = '';
    selected = plot;
  }
  function reading(value: boolean) {
    busy = value;
    onBusyChange(value);
  }
</script>

<section class="manager" data-picture-manager aria-label="Plot Pictures">
  <h1>Plot Pictures</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Owned project {project}. Select an exact parent; Load checks its literal membership in the current context.</p>
  <form onsubmit={event => { event.preventDefault(); selectPlot(); }}>
    <label for="picture-manager-plot">Plot number
      <input id="picture-manager-plot" bind:value={plot} disabled={busy} />
    </label>
    <button type="submit" disabled={busy}>Select exact plot</button>
  </form>
  {#if selected}
    <p data-picture-manager-parent>Selected literal parent: {JSON.stringify(selected)}</p>
    {#key selected}
      <PicturePanel {contextId} {project} plotNumber={selected} view="manager" onBusyChange={reading} />
    {/key}
  {/if}
  <p class="guidance">Read-only adaptation of frmPlotPictureMaster / frmPlotPictures. Parent and child link by PlotNumber.
    This review does not apply current SU or profile filters. Selecting or loading never changes the parent or picture metadata.
    Add a Picture, deletion, metadata editing, external directories and automatic installation remain unavailable.</p>
</section>

<style>
  .manager { padding: 1rem; min-width: 0; max-width: 100%; }
  h1 { font-size: 1.5rem; font-weight: 600; margin-bottom: .75rem; }
  form { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin: .75rem 0; }
  label { display: grid; gap: .3rem; min-width: 0; flex: 1; }
  input, button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem .6rem; min-width: 0; }
  button:disabled, input:disabled { opacity: .5; }
  p { overflow-wrap: anywhere; }
  .error { color: #991b1b; background: #fef2f2; padding: .5rem; }
  .guidance { font-size: .8rem; color: #475569; margin-top: 1rem; }
</style>
