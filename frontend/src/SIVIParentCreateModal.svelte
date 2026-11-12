<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import {
    defaultSIVIParentCreatePort,
    SIVI_PLOT_TYPES,
    validateCreateSIVIPlotDraft,
    type SIVIParentCreatePort,
    type SIVIPlotType,
    type CreateSIVIPlotResult,
    type SIVICreatedPlotEntry,
  } from './siviParentCreate';

  let {
    contextId,
    activeProject,
    activeSu = 'None',
    open = true,
    port = defaultSIVIParentCreatePort,
    oncreated,
    onundone,
    onclose,
  }: {
    contextId: string;
    activeProject: string;
    activeSu?: string;
    open?: boolean;
    port?: SIVIParentCreatePort;
    oncreated?: (result: CreateSIVIPlotResult) => void;
    onundone?: (plotNumber: string) => void;
    onclose?: () => void;
  } = $props();

  let plotNumber = $state('');
  let plotType = $state<SIVIPlotType>('Ground');
  let projectId = $state('');
  let fieldNumber = $state('');
  let date = $state('');
  let siteSurveyor = $state('');
  let svPolygonNumber = $state('');
  let zone = $state('');
  let subZone = $state('');
  let siteSeries = $state('');
  let userSiteUnit = $state('');
  let addToActiveSu = $state(false);

  let createdPlots = $state<SIVICreatedPlotEntry[]>([]);
  let confirmUndoId = $state<string | null>(null);
  let busy = $state(false);
  let error = $state<string | null>(null);
  let status = $state<string | null>(null);
  let generation = 0;

  const draftErrors = $derived(
    validateCreateSIVIPlotDraft({
      plotNumber,
      plotType,
      projectId: projectId || undefined,
      fieldNumber: fieldNumber || undefined,
      date: date || undefined,
      siteSurveyor: siteSurveyor || undefined,
      svPolygonNumber: svPolygonNumber || undefined,
      zone: zone || undefined,
      subZone: subZone || undefined,
      siteSeries: siteSeries || undefined,
      userSiteUnit: userSiteUnit || undefined,
      addToActiveSu,
    })
  );

  const hasValidationErrors = $derived(Object.keys(draftErrors).length > 0);

  onMount(() => {
    void refreshHistory();
  });

  onDestroy(() => {
    generation++;
  });

  async function refreshHistory() {
    const gen = ++generation;
    try {
      const list = await port.listCreatedPlots(contextId);
      if (gen !== generation) return;
      createdPlots = list ?? [];
    } catch (cause) {
      if (gen === generation) error = `Failed reading SIVI plot creation history: ${String(cause)}`;
    }
  }

  async function submitCreate() {
    const req = $state.snapshot({
      requestId: crypto.randomUUID(),
      plotNumber,
      plotType,
      projectId: projectId || undefined,
      fieldNumber: fieldNumber || undefined,
      date: date || undefined,
      siteSurveyor: siteSurveyor || undefined,
      svPolygonNumber: svPolygonNumber || undefined,
      zone: zone || undefined,
      subZone: subZone || undefined,
      siteSeries: siteSeries || undefined,
      userSiteUnit: userSiteUnit || undefined,
      addToActiveSu: activeSu !== 'None' ? addToActiveSu : false,
    });
    const errs = validateCreateSIVIPlotDraft(req);
    if (Object.keys(errs).length > 0) {
      error = Object.values(errs)[0];
      return;
    }
    const gen = ++generation;
    busy = true;
    error = null;
    status = null;
    try {
      const res = await port.createPlot(contextId, req);
      if (gen !== generation) return;
      status = `Created FS1333 SIVI ${res.plotType} plot "${res.plotNumber}" (${res.auditRowCount} audit row(s)).`;
      plotNumber = '';
      fieldNumber = '';
      const list = await port.listCreatedPlots(contextId);
      if (gen === generation) createdPlots = list ?? [];
      if (oncreated) oncreated(res);
    } catch (cause) {
      if (gen === generation) error = `Create SIVI plot failed: ${String(cause)}`;
    } finally {
      if (gen === generation) busy = false;
    }
  }

  async function executeUndo(entry: SIVICreatedPlotEntry) {
    const req = $state.snapshot({
      requestId: crypto.randomUUID(),
      creationRequestId: entry.requestId,
      plotNumber: entry.plotNumber,
    });
    const gen = ++generation;
    busy = true;
    error = null;
    status = null;
    try {
      const res = await port.undoCreatePlot(contextId, req);
      if (gen !== generation) return;
      confirmUndoId = null;
      status = `Undid SIVI plot creation "${res.plotNumber}" cleanly.`;
      const list = await port.listCreatedPlots(contextId);
      if (gen === generation) createdPlots = list ?? [];
      if (onundone) onundone(res.plotNumber);
    } catch (cause) {
      if (gen === generation) error = `Undo SIVI plot creation failed: ${String(cause)}`;
    } finally {
      if (gen === generation) busy = false;
    }
  }
</script>

{#if open}
  <div class="rounded border border-slate-300 bg-white shadow-lg" role="dialog" aria-modal="true" aria-label="Create FS1333 SIVI Parent Plot">
    <header class="flex items-center justify-between border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-3 text-white">
      <div>
        <p class="text-xs font-semibold uppercase tracking-wider text-[#FCBA19]">FS1333 Site Inspection &amp; Vegetation Inventory</p>
        <h3 class="m-0 text-base font-semibold text-white">Create SIVI Parent Plot ({activeProject})</h3>
      </div>
      {#if onclose}
        <button
          type="button"
          disabled={busy}
          class="rounded border border-white/40 px-3 py-1 text-xs font-medium text-white hover:bg-white/10"
          onclick={onclose}
        >
          Close
        </button>
      {/if}
    </header>

    <div class="space-y-4 p-5">
      {#if error}
        <p role="alert" class="rounded border-l-4 border-red-700 bg-red-50 p-3 text-xs text-red-900">{error}</p>
      {/if}
      {#if status}
        <p role="status" class="rounded border-l-4 border-[#003366] bg-slate-50 p-3 text-xs text-slate-900">{status}</p>
      {/if}

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-plot">PlotNumber (required, max 7 UTF-16)</label>
          <input
            id="sivi-create-plot"
            type="text"
            class="w-full rounded border border-slate-300 p-2 text-sm font-mono"
            bind:value={plotNumber}
            disabled={busy}
            aria-invalid={!!draftErrors.plotNumber && plotNumber !== ''}
          />
          {#if draftErrors.plotNumber && plotNumber !== ''}
            <p role="alert" class="text-xs text-red-700">{draftErrors.plotNumber}</p>
          {/if}
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-type">SIVI PlotType</label>
          <select id="sivi-create-type" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={plotType} disabled={busy}>
            {#each SIVI_PLOT_TYPES as pt}
              <option value={pt}>{pt}</option>
            {/each}
          </select>
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-polygon">SV_PolygonNumber (max 25)</label>
          <input id="sivi-create-polygon" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={svPolygonNumber} disabled={busy} />
          {#if draftErrors.svPolygonNumber}
            <p role="alert" class="text-xs text-red-700">{draftErrors.svPolygonNumber}</p>
          {/if}
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-fieldnum">FieldNumber (max 50)</label>
          <input id="sivi-create-fieldnum" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={fieldNumber} disabled={busy} />
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-date">Survey Date (YYYY-MM-DD)</label>
          <input id="sivi-create-date" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={date} disabled={busy} placeholder="2026-07-15" />
          {#if draftErrors.date}
            <p role="alert" class="text-xs text-red-700">{draftErrors.date}</p>
          {/if}
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-surveyor">SiteSurveyor (max 30)</label>
          <input id="sivi-create-surveyor" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={siteSurveyor} disabled={busy} />
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-zone">BEC Zone (max 4)</label>
          <input id="sivi-create-zone" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={zone} disabled={busy} />
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-subzone">SubZone (max 8)</label>
          <input id="sivi-create-subzone" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={subZone} disabled={busy} />
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-series">SiteSeries (max 5)</label>
          <input id="sivi-create-series" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={siteSeries} disabled={busy} />
        </div>

        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-pid">ProjectID (max 30, optional)</label>
          <input id="sivi-create-pid" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={projectId} disabled={busy} placeholder={activeProject} />
        </div>

        <div class="space-y-1 sm:col-span-2">
          <label class="block text-xs font-semibold text-slate-700" for="sivi-create-usu">UserSiteUnit (max 100, optional)</label>
          <input id="sivi-create-usu" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={userSiteUnit} disabled={busy} />
        </div>
      </div>

      {#if activeSu !== 'None'}
        <label class="flex items-center gap-2 text-xs text-slate-800">
          <input type="checkbox" bind:checked={addToActiveSu} disabled={busy} />
          Also append new plot into active Site Unit table ({activeSu}_SU)
        </label>
      {/if}

      <div class="flex items-center justify-end gap-3 border-t border-slate-200 pt-3">
        <button
          type="button"
          disabled={busy || hasValidationErrors}
          class="rounded border-2 border-[#FCBA19] bg-[#003366] px-5 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
          onclick={submitCreate}
        >
          Create SIVI Plot
        </button>
      </div>

      {#if createdPlots.length > 0}
        <fieldset class="space-y-2 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-3">
          <legend class="px-1 text-xs font-semibold text-[#003366]">Recent SIVI Plot Creations (__VPRO_SIVICreationPlotHistory)</legend>
          <div class="max-h-44 overflow-y-auto">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-100 text-slate-700">
                <tr>
                  <th class="p-2">PlotNumber</th>
                  <th class="p-2">PlotType</th>
                  <th class="p-2">Polygon</th>
                  <th class="p-2">Created</th>
                  <th class="p-2">Action</th>
                </tr>
              </thead>
              <tbody>
                {#each createdPlots as item (item.requestId)}
                  <tr class="border-t border-slate-100">
                    <td class="p-2 font-mono font-semibold">{item.plotNumber}</td>
                    <td class="p-2">{item.plotType}</td>
                    <td class="p-2 font-mono">{item.svPolygonNumber || '-'}</td>
                    <td class="p-2">{item.createdAt}</td>
                    <td class="p-2">
                      {#if item.undone}
                        <span class="text-slate-500">Undone ({item.undoneAt})</span>
                      {:else if !item.canUndo}
                        <span class="text-slate-500">Has child rows (locked)</span>
                      {:else if confirmUndoId === item.requestId}
                        <div class="flex items-center gap-1">
                          <button
                            type="button"
                            disabled={busy}
                            class="rounded bg-red-700 px-2 py-1 text-xs font-semibold text-white hover:bg-red-800"
                            onclick={() => executeUndo(item)}
                          >
                            Confirm Undo
                          </button>
                          <button
                            type="button"
                            disabled={busy}
                            class="rounded border border-slate-300 px-2 py-1 text-xs"
                            onclick={() => {
                              confirmUndoId = null;
                            }}
                          >
                            Cancel
                          </button>
                        </div>
                      {:else}
                        <button
                          type="button"
                          disabled={busy}
                          class="rounded border border-[#003366] px-2 py-1 text-xs font-medium text-[#003366] hover:bg-slate-50"
                          onclick={() => {
                            confirmUndoId = item.requestId;
                          }}
                        >
                          Undo Plot Creation
                        </button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </fieldset>
      {/if}
    </div>
  </div>
{/if}
