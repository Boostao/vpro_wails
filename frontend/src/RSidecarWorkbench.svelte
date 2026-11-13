<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    defaultRSidecarPort,
    validateClusteringDraft,
    validatePromoteCandidateDraft,
    validateQuartoGuideDraft,
    edatopicCellColor,
    indexEdatopicCells,
    computeDendrogramSVGLayout,
    type RSidecarBackendPort,
    type RRuntimeStatus,
    type ClimateRefPeriod,
    type ClimateGroupBy,
    type ClimateEdatopicResult,
    type PlotClimateRow,
    type EdatopicGridCell,
    type ClusteringMethod,
    type EcologicalDistance,
    type HierarchicalLinkage,
    type AbundanceTransform,
    type ClusteringResult,
    type PromoteCandidateSUResult,
    type QuartoGuideType,
    type QuartoGuideResult,
  } from './rSidecarWorkbench';

  let {
    contextId,
    activeProject,
    activeSu = 'None',
    operationHeld = false,
    port = defaultRSidecarPort,
    onBusyChange,
    oncontextswitched,
  }: {
    contextId: string;
    activeProject: string;
    activeSu?: string;
    operationHeld?: boolean;
    port?: RSidecarBackendPort;
    onBusyChange?: (busy: boolean) => void;
    oncontextswitched?: (state: unknown) => void;
  } = $props();

  type SidecarTab = 'runtime' | 'edatopic' | 'clustering' | 'quarto';
  let activeTab = $state<SidecarTab>('runtime');
  let busy = $state(false);
  let error = $state<string | null>(null);
  let statusMessage = $state<string | null>(null);
  let generation = 0;
  let trackedContextId = '';

  function setBusy(next: boolean) {
    busy = next;
    onBusyChange?.(next);
  }

  // Tab 1: Runtime & Cached Plot Climate state
  let runtimeStatus = $state<RRuntimeStatus | null>(null);
  let refPeriod = $state<ClimateRefPeriod>('1961_1990');
  let climateGroupBy = $state<ClimateGroupBy>('bgc');
  let climateBgcFilter = $state('');
  let climateUseSu = $state(false);
  let climateForceRefresh = $state(false);
  let climateResult = $state<ClimateEdatopicResult | null>(null);
  let cachedRows = $state<PlotClimateRow[]>([]);

  // Tab 2: Dual Relative vs Actual Edatopic Grids state
  let selectedUnitGroup = $state<string>('__overall__');
  let selectedCellDetail = $state<{ gridKind: string; cell: EdatopicGridCell } | null>(null);

  // Tab 3: BEC_R Clustering & SVG Dendrogram state
  let clusterMethod = $state<ClusteringMethod>('NC');
  let clusterDistance = $state<EcologicalDistance>('bray');
  let clusterLinkage = $state<HierarchicalLinkage>('ward.D2');
  let clusterTransform = $state<AbundanceTransform>('sqrt');
  let clusterK = $state(3);
  let clusterFuzziness = $state(1.25);
  let clusterDnoise = $state(0.75);
  let clusterCutHeight = $state<number>(0);
  let clusterApplyLump = $state(true);
  let clusterUseSu = $state(false);
  let clusterBgcFilter = $state('');
  let clusterUnitPrefix = $state('BEC');
  let clusterCandidatePrefix = $state('');
  let clusterMaterializeCandidate = $state(true);
  let clusterResult = $state<ClusteringResult | null>(null);
  let selectedDendroPlot = $state<string | null>(null);

  let promoteTargetSuName = $state('');
  let promoteExcludeNoise = $state(true);
  let promoteConfirmOverwrite = $state(false);
  let promoteActivate = $state(false);
  let promoteResult = $state<PromoteCandidateSUResult | null>(null);

  // Tab 4: Quarto Guide Publication state
  let quartoGuideType = $state<QuartoGuideType>('single_unit');
  let quartoTitle = $state('');
  let quartoTargetUnit = $state('');
  let quartoBgcFilter = $state('');
  let quartoConceptNotes = $state('');
  let quartoMinConstancy = $state(20);
  let quartoUseSu = $state(false);
  let quartoApplyLump = $state(true);
  let quartoIncludePdf = $state(false);
  let quartoDestinationPath = $state('');
  let quartoResult = $state<QuartoGuideResult | null>(null);

  $effect(() => {
    if (trackedContextId !== '' && trackedContextId !== contextId) {
      generation++;
      setBusy(false);
      selectedCellDetail = null;
    }
    trackedContextId = contextId;
  });

  onDestroy(() => {
    generation++;
    onBusyChange?.(false);
  });

  const displayedClimateRows = $derived(
    climateResult ? climateResult.plots : cachedRows
  );

  const activeEdatopicView = $derived.by(() => {
    if (!climateResult) return null;
    if (selectedUnitGroup === '__overall__') {
      return {
        label: `All Scoped Plots (${climateResult.project})`,
        plotCount: climateResult.coordinatizedPlots,
        meanShift: null as number | null,
        meanCmd: null as number | null,
        relativeGrid: climateResult.overallRelative,
        actualGrid: climateResult.overallActual,
      };
    }
    const found = climateResult.unitSummaries.find(u => u.groupKey === selectedUnitGroup);
    if (!found) {
      return {
        label: `All Scoped Plots (${climateResult.project})`,
        plotCount: climateResult.coordinatizedPlots,
        meanShift: null as number | null,
        meanCmd: null as number | null,
        relativeGrid: climateResult.overallRelative,
        actualGrid: climateResult.overallActual,
      };
    }
    return {
      label: found.groupKey,
      plotCount: found.plotCount,
      meanShift: found.meanShift ?? null,
      meanCmd: found.meanCmd,
      relativeGrid: found.relativeGrid,
      actualGrid: found.actualGrid,
    };
  });

  const relIndexed = $derived(indexEdatopicCells(activeEdatopicView?.relativeGrid));
  const actIndexed = $derived(indexEdatopicCells(activeEdatopicView?.actualGrid));

  const dendrogramLayout = $derived(
    computeDendrogramSVGLayout(
      clusterResult?.dendrogram,
      clusterCutHeight > 0 ? clusterCutHeight : undefined
    )
  );

  const clusteringValidationError = $derived(
    validateClusteringDraft({
      method: clusterMethod,
      distance: clusterDistance,
      linkage: clusterLinkage,
      transform: clusterTransform,
      k: Number(clusterK),
      fuzziness: Number(clusterFuzziness),
      dnoise: Number(clusterDnoise),
      cutHeight: clusterCutHeight > 0 ? Number(clusterCutHeight) : undefined,
      applyLump: clusterApplyLump,
      useActiveSu: clusterUseSu,
      bgcFilter: clusterBgcFilter || undefined,
      unitPrefix: clusterUnitPrefix || undefined,
      candidatePrefix: clusterCandidatePrefix || undefined,
      materializeCandidate: clusterMaterializeCandidate,
    })
  );

  const promoteValidationError = $derived(
    promoteTargetSuName === ''
      ? null
      : validatePromoteCandidateDraft(
          {
            candidateSuName: clusterResult?.candidateSuName || `${activeProject}_ClusterCandidate`,
            targetSuName: promoteTargetSuName,
            excludeNoise: promoteExcludeNoise,
            confirmOverwrite: promoteConfirmOverwrite,
            activate: promoteActivate,
          },
          activeSu
        )
  );

  const quartoValidationError = $derived(
    validateQuartoGuideDraft({
      guideType: quartoGuideType,
      title: quartoTitle || undefined,
      targetUnit: quartoTargetUnit || undefined,
      bgcFilter: quartoBgcFilter || undefined,
      conceptNotes: quartoConceptNotes || undefined,
      refPeriod,
      minConstancyPct: Number(quartoMinConstancy),
      useActiveSu: quartoUseSu,
      applyLump: quartoApplyLump,
      includePdf: quartoIncludePdf,
      destinationPath: quartoDestinationPath || undefined,
    })
  );

  async function refreshRRuntime() {
    if (operationHeld) return;
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const status = await port.detectRRuntime();
      if (gen !== generation) return;
      runtimeStatus = status;
      statusMessage = status.available
        ? `Detected R ${status.rVersion} (${status.rscriptPath})${status.quartoAvailable ? ` and Quarto ${status.quartoVersion}` : ''}.`
        : 'R runtime was not detected on this workstation. Review setup instructions below.';
    } catch (cause) {
      if (gen === generation) error = `Runtime probe failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function loadCachedPlotClimate() {
    if (operationHeld) return;
    const periodSnap = $state.snapshot(refPeriod);
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const rows = await port.getCachedPlotClimate(contextId, periodSnap);
      if (gen !== generation) return;
      cachedRows = rows || [];
      statusMessage = `Loaded ${cachedRows.length} cached __VPRO_PlotClimate records (${periodSnap}).`;
    } catch (cause) {
      if (gen === generation) error = `Load cached plot climate failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runClimateAndDualEdatopic() {
    if (operationHeld) {
      error = 'Wait for the current operation or decision before computing climate normals.';
      return;
    }
    const req = $state.snapshot({
      refPeriod,
      groupBy: climateGroupBy,
      bgcFilter: climateBgcFilter.trim() || undefined,
      useActiveSu: climateUseSu,
      forceRefresh: climateForceRefresh,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.computePlotClimateAndDualEdatopic(contextId, req);
      if (gen !== generation) return;
      climateResult = res;
      cachedRows = res.plots;
      selectedUnitGroup = '__overall__';
      selectedCellDetail = null;
      statusMessage = `Evaluated ${res.coordinatizedPlots} plots (${res.cacheHits} cache hits, ${res.cacheMisses} computed) for ${res.refPeriod}.`;
    } catch (cause) {
      if (gen === generation) error = `Climate & aSMR computation failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function submitClustering() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before running BEC_R clustering.';
      return;
    }
    const req = $state.snapshot({
      method: clusterMethod,
      distance: clusterDistance,
      linkage: clusterLinkage,
      transform: clusterTransform,
      k: Number(clusterK),
      fuzziness: Number(clusterFuzziness),
      dnoise: Number(clusterDnoise),
      cutHeight: clusterCutHeight > 0 ? Number(clusterCutHeight) : undefined,
      applyLump: clusterApplyLump,
      useActiveSu: clusterUseSu,
      bgcFilter: clusterBgcFilter.trim() || undefined,
      unitPrefix: clusterUnitPrefix.trim() || undefined,
      candidatePrefix: clusterCandidatePrefix.trim() || undefined,
      materializeCandidate: clusterMaterializeCandidate,
    });
    const vErr = validateClusteringDraft(req);
    if (vErr) {
      error = vErr;
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.runMultivariateClustering(contextId, req);
      if (gen !== generation) return;
      clusterResult = res;
      promoteResult = null;
      clusterCutHeight = res.dendrogram.cutHeight;
      statusMessage = res.candidateTableName
        ? `Clustered ${res.plotCount} plots into ${res.clusters.length} groups and staged ${res.candidateTableName} (active SU preserved).`
        : `Clustered ${res.plotCount} plots into ${res.clusters.length} groups.`;
    } catch (cause) {
      if (gen === generation) error = `BEC_R clustering failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function submitPromoteCandidate() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before promoting candidate SU.';
      return;
    }
    const candName = clusterResult?.candidateSuName || `${activeProject}_ClusterCandidate`;
    const req = $state.snapshot({
      candidateSuName: candName,
      targetSuName: promoteTargetSuName.trim(),
      excludeNoise: promoteExcludeNoise,
      confirmOverwrite: promoteConfirmOverwrite,
      activate: promoteActivate,
    });
    const vErr = validatePromoteCandidateDraft(req, activeSu);
    if (vErr) {
      error = vErr;
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.promoteClusterCandidateToSU(contextId, req);
      if (gen !== generation) return;
      promoteResult = res;
      statusMessage = `Promoted ${res.promotedRows} plots from ${res.candidateTableName} into ${res.targetTableName}.`;
      if (res.activated && res.newState && oncontextswitched) {
        oncontextswitched(res.newState);
      }
    } catch (cause) {
      if (gen === generation) error = `Promote candidate SU failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function submitRenderQuartoGuide() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before rendering Quarto guide.';
      return;
    }
    const req = $state.snapshot({
      guideType: quartoGuideType,
      title: quartoTitle.trim() || undefined,
      targetUnit: quartoTargetUnit.trim() || undefined,
      bgcFilter: quartoBgcFilter.trim() || undefined,
      conceptNotes: quartoConceptNotes || undefined,
      refPeriod,
      minConstancyPct: Number(quartoMinConstancy),
      useActiveSu: quartoUseSu,
      applyLump: quartoApplyLump,
      includePdf: quartoIncludePdf,
      destinationPath: quartoDestinationPath.trim() || undefined,
    });
    const vErr = validateQuartoGuideDraft(req);
    if (vErr) {
      error = vErr;
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.renderQuartoGuide(contextId, req);
      if (gen !== generation) return;
      quartoResult = res;
      statusMessage = `Published ${res.guideType} HTML guide via ${res.rendererUsed} to ${res.htmlPath}.`;
    } catch (cause) {
      if (gen === generation) error = `Quarto guide publication failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }
</script>

<section class="rsidecar-workbench" aria-label="BEC R Sidecar Climate Clustering and Quarto Workbench">
  <header class="bcgov-header" style="background:#003366;border-bottom:4px solid #FCBA19;color:#ffffff">
    <div>
      <h2>BEC_R Sidecar, Climate Normals &amp; Quarto Publication Workbench</h2>
      <p class="subtitle">
        Active Project: <strong>{activeProject}</strong> | Active SU: <strong>{activeSu}</strong> | Out-of-Process R / climr / vegclust / Quarto (Auto-7)
      </p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-gold" disabled={busy} onclick={refreshRRuntime}>
        Probe R &amp; Quarto Runtime
      </button>
    </div>
  </header>

  <div class="tab-bar" role="tablist" aria-label="R Sidecar Sections">
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'runtime'}
      class:active={activeTab === 'runtime'}
      onclick={() => (activeTab = 'runtime')}
    >
      1. Runtime &amp; climr Climate Normals
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'edatopic'}
      class:active={activeTab === 'edatopic'}
      onclick={() => (activeTab = 'edatopic')}
    >
      2. Dual rSMR vs aSMR Edatopic Grids
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'clustering'}
      class:active={activeTab === 'clustering'}
      onclick={() => (activeTab = 'clustering')}
    >
      3. BEC_R Clustering &amp; Dendrogram
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'quarto'}
      class:active={activeTab === 'quarto'}
      onclick={() => (activeTab = 'quarto')}
    >
      4. Quarto Unit &amp; BGC Guide Publisher
    </button>
  </div>

  {#if error}
    <div class="banner-error" role="alert">{error}</div>
  {/if}
  {#if statusMessage}
    <div class="banner-status" role="status">{statusMessage}</div>
  {/if}

  {#if activeTab === 'runtime'}
    <div class="panel">
      <div class="card">
        <h3>Out-of-Process R &amp; Quarto Runtime Status</h3>
        {#if runtimeStatus}
          <div class="badge-strip">
            <span class="badge" class:ok={runtimeStatus.available} class:warn={!runtimeStatus.available}>
              Rscript: {runtimeStatus.available ? `Ready (${runtimeStatus.rVersion})` : 'Not Detected'}
            </span>
            <span class="badge" class:ok={runtimeStatus.quartoAvailable} class:warn={!runtimeStatus.quartoAvailable}>
              Quarto CLI: {runtimeStatus.quartoAvailable ? `Ready (${runtimeStatus.quartoVersion})` : 'Fallback RMarkdown'}
            </span>
            <span
              class="badge"
              class:warn={!runtimeStatus.available}
              class:ok={runtimeStatus.available && runtimeStatus.climrRastersCached}
              class:neutral={runtimeStatus.available && !runtimeStatus.climrRastersCached}
            >
              Climate Engine: {!runtimeStatus.available ? 'Rscript Required (Unavailable)' : runtimeStatus.climrRastersCached ? 'Local climr Rasters' : 'Offline Ministry BGC Lapse Model'}
            </span>
            <span class="badge" class:ok={runtimeStatus.readyForClustering} class:warn={!runtimeStatus.readyForClustering}>
              BEC_R Clustering: {runtimeStatus.readyForClustering ? 'Ready' : 'Missing Packages'}
            </span>
          </div>

          {#if runtimeStatus.packages.length > 0}
            <table class="data-table compact">
              <thead>
                <tr>
                  <th>R Package</th>
                  <th>Status</th>
                  <th>Version</th>
                  <th>Role</th>
                </tr>
              </thead>
              <tbody>
                {#each runtimeStatus.packages as pkg}
                  <tr>
                    <td><code>{pkg.name}</code></td>
                    <td>{pkg.installed ? 'Installed' : 'Missing'}</td>
                    <td>{pkg.version || '—'}</td>
                    <td>{pkg.required ? 'Required' : 'Optional / Sidecar'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}

          {#if runtimeStatus.setupInstructions.length > 0}
            <div class="setup-box">
              <strong>Workstation Setup Guidance:</strong>
              <ul>
                {#each runtimeStatus.setupInstructions as step}
                  <li>{step}</li>
                {/each}
              </ul>
            </div>
          {/if}
        {:else}
          <p class="muted">Click "Probe R &amp; Quarto Runtime" to inspect installed Rscript, Quarto CLI, and BEC_R package versions.</p>
        {/if}
      </div>

      <div class="card">
        <h3>Cached Plot Climate Normals (__VPRO_PlotClimate)</h3>
        <div class="form-row">
          <label>
            Reference Period
            <select bind:value={refPeriod}>
              <option value="1961_1990">1961-1990 Historical Baseline</option>
              <option value="1991_2020">1991-2020 Modern Normal</option>
            </select>
          </label>
          <label>
            Group Grids By
            <select bind:value={climateGroupBy}>
              <option value="bgc">BGC Subzone / Variant</option>
              <option value="site_unit">Assigned Site Unit (*_SU)</option>
            </select>
          </label>
          <label>
            BGC Prefix Filter
            <input type="text" bind:value={climateBgcFilter} placeholder="e.g. SBS, BWBS, MH" />
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={climateUseSu} />
            Scope to Active SU ({activeSu})
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={climateForceRefresh} />
            Force Re-compute Cache
          </label>
        </div>

        <div class="button-row">
          <button type="button" class="btn-primary" disabled={busy} onclick={runClimateAndDualEdatopic}>
            Compute Climate &amp; CMD to aSMR
          </button>
          <button type="button" class="btn-secondary" disabled={busy} onclick={loadCachedPlotClimate}>
            Load Cached __VPRO_PlotClimate
          </button>
        </div>

        {#if climateResult}
          <div class="kpi-row">
            <div class="kpi"><span>Scoped Plots</span><strong>{climateResult.totalScopedPlots}</strong></div>
            <div class="kpi"><span>Coordinatized</span><strong>{climateResult.coordinatizedPlots}</strong></div>
            <div class="kpi"><span>Cache Hits</span><strong>{climateResult.cacheHits}</strong></div>
            <div class="kpi"><span>Cache Misses</span><strong>{climateResult.cacheMisses}</strong></div>
          </div>
        {/if}

        {#if displayedClimateRows.length > 0}
          <div class="table-scroll">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Plot</th>
                  <th>BGC</th>
                  <th>Lat / Lon</th>
                  <th>Elev (m)</th>
                  <th>MAT (C)</th>
                  <th>MAP (mm)</th>
                  <th>MSP (mm)</th>
                  <th>PAS (mm)</th>
                  <th>CMD</th>
                  <th>CMD.site</th>
                  <th>rSMR</th>
                  <th>aSMR</th>
                  <th>Source</th>
                </tr>
              </thead>
              <tbody>
                {#each displayedClimateRows.slice(0, 100) as row}
                  <tr>
                    <td><code>{row.plotNumber}</code></td>
                    <td>{row.bgc || '—'}</td>
                    <td>{row.latitude.toFixed(4)}, {row.longitude.toFixed(4)}</td>
                    <td>{row.elevation.toFixed(0)}</td>
                    <td>{row.mat.toFixed(1)}</td>
                    <td>{row.map.toFixed(0)}</td>
                    <td>{row.msp.toFixed(0)}</td>
                    <td>{row.pas.toFixed(0)}</td>
                    <td>{row.cmd.toFixed(0)}</td>
                    <td>{row.cmdSite !== undefined ? row.cmdSite.toFixed(0) : '—'}</td>
                    <td>{row.rsmr || '—'}</td>
                    <td><strong>{row.asmr !== undefined ? row.asmr : '—'}</strong></td>
                    <td><code>{row.source}</code></td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    </div>
  {:else if activeTab === 'edatopic'}
    <div class="panel">
      <div class="card">
        <h3>Ministry CMD to aSMR Dual Edatopic Grids</h3>
        {#if !climateResult || !activeEdatopicView}
          <p class="muted">Run "Compute Climate &amp; CMD to aSMR" on Tab 1 first to populate Dual Relative and Actual Edatopic Grids.</p>
        {:else}
          <div class="form-row">
            <label>
              BGC / Site Unit Scope
              <select bind:value={selectedUnitGroup}>
                <option value="__overall__">Overall Project Scope ({climateResult.coordinatizedPlots} plots)</option>
                {#each climateResult.unitSummaries as unit}
                  <option value={unit.groupKey}>
                    {unit.groupKey} ({unit.plotCount} plots{unit.meanAsmr !== undefined ? `, mean aSMR ${unit.meanAsmr}` : ''})
                  </option>
                {/each}
              </select>
            </label>
            {#if activeEdatopicView.meanShift !== null}
              <div class="kpi">
                <span>Mean Moisture Shift (aSMR - rSMR)</span>
                <strong>{activeEdatopicView.meanShift > 0 ? `+${activeEdatopicView.meanShift}` : activeEdatopicView.meanShift}</strong>
              </div>
            {/if}
          </div>

          <div class="dual-grid-wrap">
            <div class="edatopic-panel">
              <h4>Relative Edatopic Grid (rSMR x rSNR) — Modal: {activeEdatopicView.relativeGrid.modalCell || '—'}</h4>
              <table class="edatopic-table">
                <thead>
                  <tr>
                    <th>rSMR \ rSNR</th>
                    {#each activeEdatopicView.relativeGrid.nutrientAxis as snr}
                      <th>{snr}</th>
                    {/each}
                  </tr>
                </thead>
                <tbody>
                  {#each activeEdatopicView.relativeGrid.moistureAxis as smr}
                    <tr>
                      <th>{smr}</th>
                      {#each activeEdatopicView.relativeGrid.nutrientAxis as snr}
                        {@const cell = relIndexed.byKey[`${smr}|${snr}`]}
                        {@const colors = edatopicCellColor(cell?.count || 0, relIndexed.maxCount)}
                        <td style="background:{colors.bg};color:{colors.fg}">
                          {#if cell}
                            <button
                              type="button"
                              class="edatopic-cell-btn"
                              style="color:{colors.fg}"
                              onclick={() => {
                                selectedCellDetail = { gridKind: 'Relative (rSMR)', cell };
                              }}
                            >
                              <strong>{cell.count}</strong>
                              <small>{cell.frequency}%</small>
                            </button>
                          {:else}
                            ·
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <div class="edatopic-panel">
              <h4>Actual Edatopic Grid (aSMR x rSNR) — Modal: {activeEdatopicView.actualGrid.modalCell || '—'}</h4>
              <table class="edatopic-table compact-edatope">
                <thead>
                  <tr>
                    <th>aSMR \ rSNR</th>
                    {#each activeEdatopicView.actualGrid.nutrientAxis as snr}
                      <th>{snr}</th>
                    {/each}
                  </tr>
                </thead>
                <tbody>
                  {#each activeEdatopicView.actualGrid.moistureAxis as asmr}
                    <tr>
                      <th>{asmr}</th>
                      {#each activeEdatopicView.actualGrid.nutrientAxis as snr}
                        {@const cell = actIndexed.byKey[`${asmr}|${snr}`]}
                        {@const colors = edatopicCellColor(cell?.count || 0, actIndexed.maxCount)}
                        <td style="background:{colors.bg};color:{colors.fg}">
                          {#if cell}
                            <button
                              type="button"
                              class="edatopic-cell-btn"
                              style="color:{colors.fg}"
                              onclick={() => {
                                selectedCellDetail = { gridKind: 'Actual (aSMR)', cell };
                              }}
                            >
                              <strong>{cell.count}</strong>
                            </button>
                          {:else}
                            ·
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>

          {#if selectedCellDetail}
            <div class="cell-detail-box">
              <strong>
                {selectedCellDetail.gridKind} Cell {selectedCellDetail.cell.moisture}/{selectedCellDetail.cell.nutrient}:
              </strong>
              {selectedCellDetail.cell.count} plot(s) ({selectedCellDetail.cell.frequency}%)
              {#if selectedCellDetail.cell.meanCmdSite !== undefined}
                | Mean CMD.site: {selectedCellDetail.cell.meanCmdSite} mm
              {/if}
              <div>Plots: <code>{selectedCellDetail.cell.plotNumbers.join(', ')}</code></div>
            </div>
          {/if}
        {/if}
      </div>
    </div>
  {:else if activeTab === 'clustering'}
    <div class="panel">
      <div class="card">
        <h3>BEC_R Multivariate Clustering (vegclust Fuzzy/Noise &amp; vegan Hierarchical)</h3>
        <div class="form-row">
          <label>
            Algorithm
            <select bind:value={clusterMethod}>
              <option value="NC">vegclust Noise Clustering (NC)</option>
              <option value="KM">vegclust Crisp K-Means (KM)</option>
              <option value="FCM">vegclust Fuzzy C-Means (FCM)</option>
              <option value="HCLUST">vegan Hierarchical Clustering</option>
            </select>
          </label>
          <label>
            Dissimilarity
            <select bind:value={clusterDistance}>
              <option value="bray">Bray-Curtis</option>
              <option value="chord">Chord Normalized</option>
              <option value="jaccard">Jaccard</option>
              <option value="kulczynski">Kulczynski</option>
            </select>
          </label>
          <label>
            Linkage
            <select bind:value={clusterLinkage}>
              <option value="ward.D2">Ward.D2</option>
              <option value="average">UPGMA (Average)</option>
              <option value="complete">Complete</option>
            </select>
          </label>
          <label>
            Cover Transform
            <select bind:value={clusterTransform}>
              <option value="sqrt">Square Root</option>
              <option value="log1p">Log(1 + Cover)</option>
              <option value="hellinger">Hellinger</option>
              <option value="chord">Chord</option>
              <option value="raw">Raw % Cover</option>
            </select>
          </label>
          <label>
            Clusters (k)
            <input type="number" min="2" max="50" bind:value={clusterK} />
          </label>
          <label>
            Fuzziness (m)
            <input type="number" step="0.05" min="1.05" max="5" bind:value={clusterFuzziness} />
          </label>
          <label>
            Noise Distance (dnoise)
            <input type="number" step="0.05" min="0.05" max="5" bind:value={clusterDnoise} />
          </label>
          <label>
            Candidate Prefix
            <input type="text" bind:value={clusterCandidatePrefix} placeholder={activeProject} />
          </label>
        </div>

        <div class="form-row">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={clusterApplyLump} />
            Fold Taxonomic Lump Table (*_Lump)
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={clusterUseSu} />
            Scope to Active SU ({activeSu})
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={clusterMaterializeCandidate} />
            Stage &lt;Name&gt;_ClusterCandidate_SU (Never Overwrites Active SU)
          </label>
        </div>

        {#if clusteringValidationError}
          <div class="inline-error">{clusteringValidationError}</div>
        {/if}

        <div class="button-row">
          <button
            type="button"
            class="btn-primary"
            disabled={busy || Boolean(clusteringValidationError)}
            onclick={submitClustering}
          >
            Run BEC_R Clustering
          </button>
        </div>

        {#if clusterResult}
          <div class="kpi-row">
            <div class="kpi"><span>Clustered Plots</span><strong>{clusterResult.plotCount}</strong></div>
            <div class="kpi"><span>Species Taxa</span><strong>{clusterResult.speciesCount}</strong></div>
            <div class="kpi"><span>Cophenetic r</span><strong>{clusterResult.copheneticCorrelation}</strong></div>
            <div class="kpi"><span>Mean Silhouette</span><strong>{clusterResult.meanSilhouette}</strong></div>
            <div class="kpi"><span>Noise Outliers</span><strong>{clusterResult.noiseCount}</strong></div>
            {#if clusterResult.candidateTableName}
              <div class="kpi"><span>Staged Table</span><strong>{clusterResult.candidateTableName}</strong></div>
            {/if}
          </div>

          <h4>Interactive Hierarchical Dendrogram</h4>
          <div class="form-row">
            <label>
              Reference Cut Height ({clusterCutHeight.toFixed(3)} / Max {clusterResult.dendrogram.maxHeight})
              <input
                type="range"
                min="0.01"
                max={clusterResult.dendrogram.maxHeight}
                step="0.01"
                bind:value={clusterCutHeight}
              />
            </label>
            {#if selectedDendroPlot}
              <span>Selected Leaf Plot: <code>{selectedDendroPlot}</code></span>
            {/if}
          </div>

          <div class="dendrogram-scroll">
            <svg
              viewBox="0 0 {dendrogramLayout.width} {dendrogramLayout.height}"
              width={dendrogramLayout.width}
              height={dendrogramLayout.height}
              role="img"
              aria-label="BEC Hierarchical Cluster Dendrogram"
            >
              <rect x="0" y="0" width={dendrogramLayout.width} height={dendrogramLayout.height} fill="#ffffff" />
              {#each dendrogramLayout.segments as seg}
                <line x1={seg.x1} y1={seg.y1} x2={seg.x2} y2={seg.y2} stroke="#003366" stroke-width="1.4" />
              {/each}
              <line
                x1="36"
                y1={dendrogramLayout.cutLineY}
                x2={dendrogramLayout.width - 16}
                y2={dendrogramLayout.cutLineY}
                stroke="#FCBA19"
                stroke-width="2"
                stroke-dasharray="6,4"
              />
              {#each dendrogramLayout.leaves as leaf}
                <g
                  class="dendro-leaf"
                  role="button"
                  tabindex="0"
                  aria-label="Select plot {leaf.plotNumber}"
                  transform="translate({leaf.cx}, {leaf.cy})"
                  onclick={() => (selectedDendroPlot = leaf.plotNumber)}
                  onkeydown={e => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      selectedDendroPlot = leaf.plotNumber;
                    }
                  }}
                >
                  <circle r="3.2" cy="-8" fill={leaf.isNoise ? '#dc2626' : '#003366'} />
                  <text
                    transform="rotate(65)"
                    text-anchor="start"
                    font-family="monospace"
                    font-size="10"
                    fill={selectedDendroPlot === leaf.plotNumber ? '#b45309' : '#1e293b'}
                  >
                    {leaf.plotNumber} ({leaf.clusterUnit})
                  </text>
                </g>
              {/each}
            </svg>
          </div>

          <h4>Cluster Units &amp; Indicator Species Fidelity (IndVal)</h4>
          <table class="data-table">
            <thead>
              <tr>
                <th>Cluster Unit</th>
                <th>Plots</th>
                <th>Mean Richness</th>
                <th>Mean Silhouette</th>
                <th>Top Diagnostic / Indicator Species</th>
              </tr>
            </thead>
            <tbody>
              {#each clusterResult.clusters as cl}
                <tr>
                  <td><strong>{cl.clusterUnit}</strong></td>
                  <td>{cl.plotCount}</td>
                  <td>{cl.meanRichness}</td>
                  <td>{cl.meanSilhouette}</td>
                  <td>
                    {#each cl.indicators.slice(0, 5) as ind, idx}
                      <span>
                        <code>{ind.species}</code>
                        {#if ind.scientificName}<em>({ind.scientificName})</em>{/if}
                        [{ind.constancyPct}% | IndVal {ind.indVal}]{idx < Math.min(4, cl.indicators.length - 1) ? ', ' : ''}
                      </span>
                    {/each}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <div class="card">
        <h3>Promote Staged Cluster Candidate to Site Unit Table (*_SU)</h3>
        <div class="form-row">
          <label>
            Target SU Name
            <input type="text" bind:value={promoteTargetSuName} placeholder="e.g. SubzoneRev2026" />
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={promoteExcludeNoise} />
            Exclude *_Noise Outliers
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={promoteConfirmOverwrite} />
            Confirm overwrite if target *_SU exists
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={promoteActivate} />
            Activate promoted *_SU immediately
          </label>
        </div>

        {#if promoteValidationError}
          <div class="inline-error">{promoteValidationError}</div>
        {/if}

        <div class="button-row">
          <button
            type="button"
            class="btn-primary"
            disabled={busy || !promoteTargetSuName.trim() || Boolean(promoteValidationError)}
            onclick={submitPromoteCandidate}
          >
            Promote Candidate to *_SU
          </button>
        </div>

        {#if promoteResult}
          <div class="banner-status">
            Promoted {promoteResult.promotedRows} plot assignments into <strong>{promoteResult.targetTableName}</strong>
            (skipped {promoteResult.skippedNoiseRows} noise plots).
          </div>
        {/if}
      </div>
    </div>
  {:else if activeTab === 'quarto'}
    <div class="panel">
      <div class="card">
        <h3>Automated Quarto Single-Unit &amp; BGC Guide Publisher</h3>
        <div class="form-row">
          <label>
            Guide Template
            <select bind:value={quartoGuideType}>
              <option value="single_unit">Single Site Unit Ecosystem Guide (4.d)</option>
              <option value="bgc_unit">Full BGC Subzone / Variant Synthesis Guide (4.e)</option>
            </select>
          </label>
          <label>
            Guide Title (Optional)
            <input type="text" bind:value={quartoTitle} placeholder="Auto-titled from unit or BGC" />
          </label>
          <label>
            Target Site Unit Filter
            <input type="text" bind:value={quartoTargetUnit} placeholder="e.g. BWBSmw 2 /05" />
          </label>
          <label>
            BGC Filter
            <input type="text" bind:value={quartoBgcFilter} placeholder="e.g. BWBS, MH, SBSdk" />
          </label>
          <label>
            Min Constancy (%)
            <input type="number" min="0" max="100" step="5" bind:value={quartoMinConstancy} />
          </label>
          <label>
            Destination .html Path (Optional)
            <input type="text" bind:value={quartoDestinationPath} placeholder="Defaults to <ProjectDir>/exports/" />
          </label>
        </div>

        <div class="form-row">
          <label class="full-width">
            Authored Ecological Concept &amp; Diagnostic Narrative
            <textarea
              rows="3"
              bind:value={quartoConceptNotes}
              placeholder="Enter ecosystem concept statement, characteristicphysiography, soils, and seral dynamics..."
            ></textarea>
          </label>
        </div>

        <div class="form-row">
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={quartoUseSu} />
            Scope to Active SU ({activeSu})
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={quartoApplyLump} />
            Fold Taxonomic Lump Table (*_Lump)
          </label>
          <label class="checkbox-label">
            <input type="checkbox" bind:checked={quartoIncludePdf} />
            Attempt Companion PDF Export (Print-Ready HTML Always Embedded)
          </label>
        </div>

        {#if quartoValidationError}
          <div class="inline-error">{quartoValidationError}</div>
        {/if}

        <div class="button-row">
          <button
            type="button"
            class="btn-primary"
            disabled={busy || Boolean(quartoValidationError)}
            onclick={submitRenderQuartoGuide}
          >
            Render &amp; Publish Guide
          </button>
        </div>

        {#if quartoResult}
          <div class="kpi-row">
            <div class="kpi"><span>Renderer</span><strong>{quartoResult.rendererUsed}</strong></div>
            <div class="kpi"><span>Plots Summarized</span><strong>{quartoResult.plotCount}</strong></div>
            <div class="kpi"><span>Characteristic Taxa</span><strong>{quartoResult.speciesRowCount}</strong></div>
            <div class="kpi"><span>HTML Size</span><strong>{(quartoResult.htmlSizeBytes / 1024).toFixed(1)} KB</strong></div>
            {#if quartoResult.pdfStatus}
              <div class="kpi"><span>PDF Status</span><strong>{quartoResult.pdfStatus}</strong></div>
            {/if}
          </div>
          <p class="receipt-line">
            Published Path: <code>{quartoResult.htmlPath}</code> | SHA-256: <code>{quartoResult.htmlSha256.slice(0, 16)}...</code>
          </p>

          <h4>Standalone HTML Guide Preview</h4>
          <iframe
            class="quarto-preview-frame"
            title="Published BEC Quarto Guide Preview"
            sandbox=""
            srcdoc={quartoResult.htmlPreview}
          ></iframe>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .rsidecar-workbench {
    display: flex;
    flex-direction: column;
    gap: 12px;
    color: #0f172a;
    background: #f8fafc;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    overflow: hidden;
  }
  .bcgov-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    background: #003366;
    color: #ffffff;
    border-bottom: 4px solid #FCBA19;
    padding: 14px 18px;
  }
  .bcgov-header h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 700;
  }
  .bcgov-header .subtitle {
    margin: 4px 0 0;
    font-size: 12px;
    color: #e2e8f0;
  }
  .tab-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 0 16px;
    border-bottom: 1px solid #cbd5e1;
    background: #f1f5f9;
  }
  .tab-bar button {
    background: transparent;
    border: none;
    border-bottom: 3px solid transparent;
    padding: 10px 14px;
    font-size: 13px;
    font-weight: 600;
    color: #334155;
    cursor: pointer;
  }
  .tab-bar button.active {
    color: #003366;
    border-bottom-color: #FCBA19;
    background: #ffffff;
  }
  .panel {
    padding: 4px 16px 16px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .card {
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-radius: 6px;
    padding: 14px 16px;
  }
  .card h3 {
    margin: 0 0 12px;
    font-size: 15px;
    color: #003366;
    border-bottom: 2px solid #FCBA19;
    padding-bottom: 4px;
  }
  .card h4 {
    margin: 14px 0 8px;
    font-size: 13px;
    color: #003366;
  }
  .form-row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: flex-end;
    margin-bottom: 10px;
  }
  .form-row label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    font-weight: 600;
    color: #334155;
  }
  .form-row label.full-width {
    flex: 1 1 100%;
  }
  .form-row label.checkbox-label {
    flex-direction: row;
    align-items: center;
    gap: 6px;
    padding-bottom: 6px;
  }
  input[type='text'],
  input[type='number'],
  select,
  textarea {
    border: 1px solid #94a3b8;
    border-radius: 4px;
    padding: 6px 8px;
    font-size: 12px;
    background: #ffffff;
    color: #0f172a;
  }
  .button-row {
    display: flex;
    gap: 10px;
    margin: 8px 0;
  }
  .btn-primary {
    background: #003366;
    color: #ffffff;
    border: 1px solid #003366;
    border-radius: 4px;
    padding: 7px 14px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-secondary {
    background: #ffffff;
    color: #003366;
    border: 1px solid #003366;
    border-radius: 4px;
    padding: 7px 14px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .btn-gold {
    background: #FCBA19;
    color: #003366;
    border: 1px solid #d99b00;
    border-radius: 4px;
    padding: 7px 14px;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
  }
  button:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }
  .banner-error,
  .inline-error {
    margin: 4px 16px;
    padding: 8px 12px;
    background: #fef2f2;
    border-left: 4px solid #dc2626;
    color: #991b1b;
    font-size: 12px;
  }
  .inline-error {
    margin: 6px 0;
  }
  .banner-status {
    margin: 4px 16px;
    padding: 8px 12px;
    background: #eff6ff;
    border-left: 4px solid #003366;
    color: #1e3a8a;
    font-size: 12px;
  }
  .badge-strip,
  .kpi-row {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 12px;
  }
  .badge {
    padding: 4px 10px;
    border-radius: 999px;
    font-size: 12px;
    font-weight: 600;
  }
  .badge.ok {
    background: #dcfce7;
    color: #166534;
  }
  .badge.warn {
    background: #fef3c7;
    color: #92400e;
  }
  .badge.neutral {
    background: #e0f2fe;
    color: #075985;
  }
  .kpi {
    background: #f1f5f9;
    border-left: 3px solid #003366;
    padding: 6px 10px;
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    font-size: 11px;
  }
  .kpi strong {
    font-size: 14px;
    color: #0f172a;
  }
  .table-scroll,
  .dendrogram-scroll {
    overflow-x: auto;
    border: 1px solid #e2e8f0;
    border-radius: 4px;
  }
  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }
  .data-table th {
    background: #003366;
    color: #ffffff;
    text-align: left;
    padding: 6px 8px;
  }
  .data-table td {
    padding: 5px 8px;
    border-bottom: 1px solid #e2e8f0;
  }
  .dual-grid-wrap {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: 16px;
  }
  .edatopic-table {
    width: 100%;
    border-collapse: collapse;
    text-align: center;
    font-size: 11px;
  }
  .edatopic-table th,
  .edatopic-table td {
    border: 1px solid #cbd5e1;
    padding: 5px 4px;
  }
  .edatopic-table thead th {
    background: #003366;
    color: #ffffff;
  }
  .compact-edatope td,
  .compact-edatope th {
    padding: 2px 4px;
  }
  .edatopic-cell-btn {
    background: transparent;
    border: none;
    width: 100%;
    padding: 0;
    cursor: pointer;
    font: inherit;
  }
  .edatopic-table td small {
    display: block;
    font-size: 9px;
    opacity: 0.85;
  }
  .cell-detail-box,
  .setup-box {
    margin-top: 10px;
    padding: 10px 12px;
    background: #f8fafc;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    font-size: 12px;
  }
  .dendro-leaf {
    cursor: pointer;
  }
  .quarto-preview-frame {
    width: 100%;
    height: 440px;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    background: #ffffff;
  }
  .receipt-line,
  .muted {
    font-size: 12px;
    color: #475569;
  }
</style>
