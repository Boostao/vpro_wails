<script lang="ts">
  import { untrack } from 'svelte';
  import PicturePanel from './PicturePanel.svelte';
  import { plotLookupInputError } from './scopedPlotLookup';
  import { pictureMetadataSession } from './pictureMetadataSessions';
  import { pictureCreationSession } from './pictureCreationSessions';
  import PictureCreationPanel from './PictureCreationPanel.svelte';
  import { pictureLifecycleSession } from './pictureLifecycleSessions';
  import PictureLifecyclePanel from './PictureLifecyclePanel.svelte';

  let { contextId, project, initialPlot = '', onBusyChange }: {
    contextId: string; project: string; initialPlot?: string; onBusyChange: (busy: boolean) => void;
  } = $props();
  const startingPlot = untrack(() => initialPlot);
  let plot = $state(startingPlot);
  let selected = $state(startingPlot);
  let error = $state('');
  let busy = $state(false);
  const metadataWritingEnabled = import.meta.env.VITE_PICTURE_METADATA_WRITING === 'true';
  const creationEnabled = import.meta.env.VITE_PICTURE_CREATION_WRITING === 'true';
  const lifecycleEnabled = import.meta.env.VITE_PICTURE_DELETION_WRITING === 'true' || import.meta.env.VITE_PICTURE_DELETION_RESTORING === 'true';
  const lifecycleSession = $derived(lifecycleEnabled && selected && contextId && project
    ? pictureLifecycleSession({ contextId, project, plotNumber: selected }) : undefined);
  let lifecycleRevision = $state(0);
  $effect(() => lifecycleSession?.subscribe(() => untrack(() => lifecycleRevision++)));
  const lifecycleBlocked = $derived.by(() => {
    lifecycleRevision;
    const close = lifecycleSession?.closeState();
    return !!close && (close.busy || close.blocked || close.unsaved);
  });
  const creationSession = $derived(creationEnabled && selected && contextId && project
    ? pictureCreationSession({ contextId, project, plotNumber: selected }) : undefined);
  let creationRevision = $state(0);
  $effect(() => creationSession?.subscribe(() => untrack(() => creationRevision++)));
  const creationBlocked = $derived.by(() => {
    creationRevision;
    const close = creationSession?.closeState();
    return !!close && (close.busy || close.blocked || close.unsaved);
  });
  const metadataSession = $derived(metadataWritingEnabled && selected && contextId && project
    ? pictureMetadataSession({ contextId, project, plotNumber: selected }) : undefined);
  let metadataRevision = $state(0);
  const metadataBlocked = $derived.by(() => {
    metadataRevision;
    const close = metadataSession?.closeState();
    return !!close && (close.unsaved || close.busy || close.blocked === true);
  });
  $effect(() => metadataSession?.subscribe(() => untrack(() => metadataRevision++)));
  $effect(() => { onBusyChange(busy || metadataBlocked || creationBlocked || lifecycleBlocked); });

  function selectPlot() {
    if (busy) { error = 'Wait for or cancel the current picture read before selecting another plot.'; return; }
    if (metadataBlocked) { error = 'Finish or explicitly discard the retained picture metadata draft before replacing its literal parent.'; return; }
    if (creationBlocked) { error = 'Resolve the retained picture selection or creation receipt before replacing its literal parent.'; return; }
    if (lifecycleBlocked) { error = 'Finish the retained picture lifecycle review or resolve/acknowledge its receipt before replacing its literal parent.'; return; }
    const invalid = plotLookupInputError(plot);
    if (invalid) { error = invalid; return; }
    error = '';
    selected = plot;
  }
  function reading(value: boolean) {
    busy = value;
    onBusyChange(value || metadataBlocked || creationBlocked || lifecycleBlocked);
  }
</script>

<section class="manager" data-picture-manager aria-label="Plot Pictures">
  <h1>Plot Pictures</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Owned project {project}. Select an exact parent; Load checks its literal membership in the current context.</p>
  <form onsubmit={event => { event.preventDefault(); selectPlot(); }}>
    <label for="picture-manager-plot">Plot number
      <input id="picture-manager-plot" bind:value={plot} disabled={busy || metadataBlocked || creationBlocked || lifecycleBlocked} />
    </label>
    <button type="submit" disabled={busy || metadataBlocked || creationBlocked || lifecycleBlocked}>Select exact plot</button>
  </form>
  {#if selected}
    <p data-picture-manager-parent>Selected literal parent: {JSON.stringify(selected)}</p>
    {#key selected}
      {#if creationSession}<PictureCreationPanel session={creationSession} disabled={busy || metadataBlocked || lifecycleBlocked} />{/if}
      {#if lifecycleSession}<PictureLifecyclePanel session={lifecycleSession} disabled={busy || metadataBlocked || creationBlocked} />{/if}
      <PicturePanel {contextId} {project} plotNumber={selected} view="manager" onBusyChange={reading} {metadataSession}
        disabled={creationBlocked} />
    {/key}
  {/if}
  <p class="guidance">Adaptation of frmPlotPictureMaster / frmPlotPictures. Parent and child link by PlotNumber.
    This review does not apply current SU or profile filters. Selecting or loading never changes the parent or picture metadata.
    {#if creationEnabled}Add a Picture requires its independent backend permission and native chooser; it grants only the selected file.
    {:else}Add a Picture remains unavailable.{/if}
    {#if metadataWritingEnabled}Existing metadata editing requires its independent backend permission.
    {:else}Metadata editing remains unavailable.{/if}
    {#if lifecycleEnabled}Reference deletion/restoration requires independent backend permissions and complete physical review; source GUI confirmation remains unverified.
    {:else}Deletion/restoration remains unavailable.{/if}
    Directory-wide access and automatic installation remain unavailable.</p>
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
