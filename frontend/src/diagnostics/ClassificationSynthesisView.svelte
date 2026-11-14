<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte';
  import { ReadRequests } from '../shared/readRequests';
  import {
    createDefaultSynthesisOptions,
    defaultSynthesisBackendPort,
    DIAGNOSTIC_CODE_META,
    EDATOPIC_SMR_ROWS,
    EDATOPIC_SNR_COLS,
    edatopicHeatLevel,
    filterAndGroupSpeciesRows,
    filterOutlierQueue,
    findPairwiseComparison,
    formatSynthesisMatrixCell,
    getEdatopicCellMatrix,
    resolveDiagnosticCellClass,
    SMR_DESCRIPTIONS,
    SNR_DESCRIPTIONS,
    validateWorkbookExportDraft,
    type EdatopicCell,
    type ExportDiagnosticWorkbookResult,
    type MatrixDisplayMode,
    type MatrixGroupingMode,
    type OutlierFilterMode,
    type SynthesisBackendPort,
    type SynthesisLumpTableOption,
    type SynthesisOptions,
    type SynthesisSnapshot,
    type SynthesisWorkspaceTab,
  } from './classificationSynthesis';

  interface Props {
    contextId: string;
    project?: string;
    su?: string;
    initialSnapshot?: SynthesisSnapshot | null;
    port?: SynthesisBackendPort;
    onBusyChange?: (busy: boolean) => void;
    onOpenPlot?: (plotNumber: string) => void;
  }

  let {
    contextId,
    project = '',
    su = 'None',
    initialSnapshot = null,
    port = defaultSynthesisBackendPort,
    onBusyChange,
    onOpenPlot,
  }: Props = $props();

  const reads = new ReadRequests();

  let effectiveContextId = $state(untrack(() => contextId));
  let snapshot = $state.raw<SynthesisSnapshot | null>(untrack(() => initialSnapshot));
  let lumpOptions = $state.raw<SynthesisLumpTableOption[]>([]);

  let options = $state<SynthesisOptions>(createDefaultSynthesisOptions());
  let activeTab = $state<SynthesisWorkspaceTab>('matrix');
  let matrixDisplay = $state<MatrixDisplayMode>('roman_signif');
  let matrixGrouping = $state<MatrixGroupingMode>('diagnostic_unit');
  let diagnosticOnly = $state(false);
  let speciesSearch = $state('');

  let outlierFilter = $state<OutlierFilterMode>('all');
  let outlierSearch = $state('');
  let manualTargetByPlot = $state<Record<string, string>>({});

  let selectedPairUnits = $state<[string, string]>(['', '']);
  let similarityMetric = $state<'sorensen' | 'bec_diff'>('sorensen');

  let selectedEdatopicScope = $state<string>('__OVERALL__');
  let inspectedEdatopicCell = $state.raw<EdatopicCell | null>(null);

  // Drag-and-drop plot reassignment state
  let draggingPlotNumber = $state<string | null>(null);
  let dragOverUnit = $state<string | null>(null);

  // Workbook export modal state
  let exportModalOpen = $state(false);
  let exportPath = $state('');
  let exportTitle = $state('');
  let exportIncludeCommon = $state(true);
  let exportDiagSheet = $state('Diagnostic Summary');
  let exportSimSheet = $state('Pairwise Similarity');
  let exportOutlierSheet = $state('Outlier Queue');
  let exportEdatopicSheet = $state('Edatopic Grid');
  let exportUnitEdatopicTabs = $state(false);
  let exportingWorkbook = $state(false);
  let lastExportResult = $state.raw<ExportDiagnosticWorkbookResult | null>(null);

  let loading = $state(false);
  let mutating = $state(false);
  let lastLatencyMs = $state<number | null>(
    untrack(() => (initialSnapshot ? initialSnapshot.computeMs : null))
  );
  let statusNotice = $state('');
  let errorMessage = $state('');

  let requestGeneration = 0;

  $effect(() => {
    if (contextId && contextId !== effectiveContextId) {
      effectiveContextId = contextId;
      void refreshAll();
    }
  });

  function setBusyState(nextLoading: boolean, nextMutating: boolean) {
    loading = nextLoading;
    mutating = nextMutating;
    onBusyChange?.(nextLoading || nextMutating || exportingWorkbook);
  }

  async function refreshAll(): Promise<void> {
    if (!effectiveContextId) return;
    reads.cancelAll();
    const gen = ++requestGeneration;
    setBusyState(true, mutating);
    errorMessage = '';

    try {
      const plainOpts = $state.snapshot(options);
      const [lumps, snap] = await Promise.all([
        reads.track(port.listSynthesisLumpTables(effectiveContextId)),
        reads.track(port.computeSynthesisSnapshot(effectiveContextId, plainOpts)),
      ]);
      if (gen !== requestGeneration) return;
      lumpOptions = lumps ?? [];
      applyIncomingSnapshot(snap, snap.computeMs);
    } catch (err) {
      if (gen !== requestGeneration) return;
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      if (gen === requestGeneration) {
        setBusyState(false, mutating);
      }
    }
  }

  async function recomputeSnapshotOnly(): Promise<void> {
    if (!effectiveContextId) return;
    reads.cancelAll();
    const gen = ++requestGeneration;
    setBusyState(true, mutating);
    errorMessage = '';

    try {
      const plainOpts = $state.snapshot(options);
      const snap = await reads.track(port.computeSynthesisSnapshot(effectiveContextId, plainOpts));
      if (gen !== requestGeneration) return;
      applyIncomingSnapshot(snap, snap.computeMs);
    } catch (err) {
      if (gen !== requestGeneration) return;
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      if (gen === requestGeneration) {
        setBusyState(false, mutating);
      }
    }
  }

  function applyIncomingSnapshot(snap: SynthesisSnapshot, latencyMs: number) {
    snapshot = snap;
    lastLatencyMs = latencyMs;
    if (
      selectedEdatopicScope !== '__OVERALL__' &&
      !snap.units.some(u => u.siteUnit === selectedEdatopicScope)
    ) {
      selectedEdatopicScope = '__OVERALL__';
    }
    if (
      snap.units.length >= 2 &&
      (!selectedPairUnits[0] ||
        !selectedPairUnits[1] ||
        !snap.units.some(u => u.siteUnit === selectedPairUnits[0]) ||
        !snap.units.some(u => u.siteUnit === selectedPairUnits[1]))
    ) {
      selectedPairUnits = [snap.units[0].siteUnit, snap.units[1].siteUnit];
    }
  }

  async function handleReassignPlot(plotNumber: string, targetUnit: string): Promise<void> {
    const cleanTarget = targetUnit.trim();
    if (!effectiveContextId || !plotNumber || !cleanTarget || mutating) return;
    if (!snapshot?.suWritable) {
      errorMessage =
        snapshot?.suReadOnlyReason ||
        'Select a writable Site Unit (SU) table before reassigning plots.';
      return;
    }

    const gen = ++requestGeneration;
    setBusyState(loading, true);
    errorMessage = '';
    try {
      const plainOpts = $state.snapshot(options);
      const res = await port.reassignSynthesisPlotUnit(effectiveContextId, {
        plotNumber,
        targetSiteUnit: cleanTarget,
        options: plainOpts,
      });
      if (gen !== requestGeneration) return;
      applyIncomingSnapshot(res.snapshot, res.latencyMs);
      statusNotice = res.changed
        ? `Moved plot ${res.plotNumber} from ${res.beforeSiteUnit || 'Unassigned'} to ${res.afterSiteUnit} in ${res.latencyMs.toFixed(1)} ms.`
        : `Plot ${res.plotNumber} is already assigned to ${res.afterSiteUnit}.`;
    } catch (err) {
      if (gen !== requestGeneration) return;
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      if (gen === requestGeneration) {
        setBusyState(loading, false);
      }
    }
  }

  async function handleUndoReassignment(): Promise<void> {
    if (!effectiveContextId || mutating || !snapshot?.canUndo) return;
    const gen = ++requestGeneration;
    setBusyState(loading, true);
    errorMessage = '';
    try {
      const plainOpts = $state.snapshot(options);
      const res = await port.undoSynthesisReassignment(effectiveContextId, {
        options: plainOpts,
      });
      if (gen !== requestGeneration) return;
      applyIncomingSnapshot(res.snapshot, res.latencyMs);
      statusNotice = `Restored plot ${res.plotNumber} to ${res.afterSiteUnit || 'Unassigned'} in ${res.latencyMs.toFixed(1)} ms.`;
    } catch (err) {
      if (gen !== requestGeneration) return;
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      if (gen === requestGeneration) {
        setBusyState(loading, false);
      }
    }
  }

  async function handlePublishWorkbook(): Promise<void> {
    const validationErr = validateWorkbookExportDraft({
      outputPath: exportPath,
      diagnosticSheetName: exportDiagSheet,
      similaritySheetName: exportSimSheet,
      outlierSheetName: exportOutlierSheet,
      edatopicSheetName: exportEdatopicSheet,
    });
    if (validationErr) {
      errorMessage = validationErr;
      return;
    }
    const gen = ++requestGeneration;
    exportingWorkbook = true;
    onBusyChange?.(true);
    errorMessage = '';
    try {
      const plainOpts = $state.snapshot(options);
      const res = await port.exportDiagnosticWorkbook(effectiveContextId, {
        outputPath: exportPath.trim(),
        reportTitle: exportTitle.trim(),
        includeCommonNames: exportIncludeCommon,
        diagnosticSheetName: exportDiagSheet.trim(),
        similaritySheetName: exportSimSheet.trim(),
        outlierSheetName: exportOutlierSheet.trim(),
        edatopicSheetName: exportEdatopicSheet.trim(),
        includeUnitEdatopicTabs: exportUnitEdatopicTabs,
        options: plainOpts,
      });
      if (gen !== requestGeneration) return;
      lastExportResult = res;
      statusNotice = `Published 4-sheet BEC Diagnostic workbook (${res.path}) in ${res.latencyMs.toFixed(1)} ms.`;
    } catch (err) {
      if (gen !== requestGeneration) return;
      errorMessage = err instanceof Error ? err.message : String(err);
    } finally {
      if (gen === requestGeneration) {
        exportingWorkbook = false;
        onBusyChange?.(loading || mutating);
      }
    }
  }

  function onPlotDragStart(ev: DragEvent, plotNumber: string) {
    if (!snapshot?.suWritable) return;
    draggingPlotNumber = plotNumber;
    if (ev.dataTransfer) {
      ev.dataTransfer.effectAllowed = 'move';
      ev.dataTransfer.setData('text/plain', plotNumber);
    }
  }

  function onPlotDragEnd() {
    draggingPlotNumber = null;
    dragOverUnit = null;
  }

  function onUnitDragOver(ev: DragEvent, unitCode: string) {
    if (!snapshot?.suWritable || !draggingPlotNumber) return;
    ev.preventDefault();
    dragOverUnit = unitCode;
    if (ev.dataTransfer) {
      ev.dataTransfer.dropEffect = 'move';
    }
  }

  function onUnitDrop(ev: DragEvent, unitCode: string) {
    ev.preventDefault();
    const plotNum = draggingPlotNumber || ev.dataTransfer?.getData('text/plain') || '';
    draggingPlotNumber = null;
    dragOverUnit = null;
    if (plotNum && unitCode) {
      void handleReassignPlot(plotNum, unitCode);
    }
  }

  onMount(() => {
    if (!initialSnapshot && effectiveContextId) {
      void refreshAll();
    }
  });

  onDestroy(() => {
    requestGeneration++;
    reads.cancelAll();
    onBusyChange?.(false);
  });

  const matrixSections = $derived(
    filterAndGroupSpeciesRows(snapshot, {
      searchText: speciesSearch,
      diagnosticOnly,
      grouping: matrixGrouping,
    })
  );

  const visibleOutliers = $derived(
    filterOutlierQueue(snapshot?.outlierQueue ?? [], outlierFilter, outlierSearch)
  );

  const misplacedCount = $derived(
    (snapshot?.outlierQueue ?? []).filter(item => item.misplaced).length
  );

  const activePairComparison = $derived(
    findPairwiseComparison(
      snapshot?.pairwiseComparisons ?? [],
      selectedPairUnits[0],
      selectedPairUnits[1]
    )
  );

  const activeEdatopicGrid = $derived.by(() => {
    if (!snapshot) return null;
    if (selectedEdatopicScope === '__OVERALL__') {
      return snapshot.overallEdatopicGrid;
    }
    return (
      snapshot.unitEdatopicGrids.find(g => g.siteUnit === selectedEdatopicScope) ??
      snapshot.overallEdatopicGrid
    );
  });

  const edatopicRows = $derived(getEdatopicCellMatrix(activeEdatopicGrid));

  const exportValidationMessage = $derived(
    exportModalOpen
      ? validateWorkbookExportDraft({
          outputPath: exportPath,
          diagnosticSheetName: exportDiagSheet,
          similaritySheetName: exportSimSheet,
          outlierSheetName: exportOutlierSheet,
          edatopicSheetName: exportEdatopicSheet,
        })
      : ''
  );
</script>

<section class="synthesis-workbench" aria-label="BEC Differential and Pairwise Synthesis Workbench">
  <!-- BC Gov #003366 / #FCBA19 Synthesis Command Header -->
  <header class="syn-header">
    <div class="syn-header-brand">
      <div class="syn-eyebrow">B.C. MINISTRY OF FORESTS · BEC CLASSIFICATION SYNTHESIS</div>
      <div class="syn-title-row">
        <h2>Reactive BEC Differential &amp; Pairwise Synthesis</h2>
        <span class="syn-chip syn-chip-gold">
          {snapshot?.project || project || 'Active Project'} · SU: {snapshot?.su || su || 'None'}
        </span>
        {#if snapshot}
          <span class="syn-chip">
            {snapshot.classifiedPlotCount} Classified Plots · {snapshot.units.length} Units
          </span>
          {#if lastLatencyMs !== null}
            <span class="syn-chip syn-latency-badge" data-testid="synthesis-latency-badge">
              {lastLatencyMs.toFixed(1)} ms
            </span>
          {/if}
        {/if}
      </div>
    </div>

    <div class="syn-header-actions">
      <button
        type="button"
        class="syn-btn syn-btn-ghost"
        disabled={loading || mutating}
        onclick={() => void refreshAll()}
      >
        Refresh
      </button>

      <button
        type="button"
        class="syn-btn syn-btn-ghost"
        disabled={!snapshot?.canUndo || mutating || loading}
        title={snapshot?.latestUndo
          ? `Undo move of plot ${snapshot.latestUndo.plotNumber} (${snapshot.latestUndo.afterSiteUnit} -> ${snapshot.latestUndo.beforeSiteUnit || 'Unassigned'})`
          : 'No synthesis SU reassignment to undo'}
        onclick={() => void handleUndoReassignment()}
      >
        Undo Move{snapshot?.undoDepth ? ` (${snapshot.undoDepth})` : ''}
      </button>

      <button
        type="button"
        class="syn-btn syn-btn-gold"
        disabled={!snapshot || loading}
        onclick={() => {
          lastExportResult = null;
          exportModalOpen = true;
        }}
      >
        Publish Diagnostic .xlsx
      </button>
    </div>
  </header>

  <!-- Synthesis Controls & Filter Bar -->
  <div class="syn-toolbar" role="region" aria-label="Synthesis Options">
    <label class="syn-control">
      <span>Species Lumping (99% Cap)</span>
      <select
        value={options.lumpTable ?? ''}
        disabled={loading || mutating}
        onchange={e => {
          options.lumpTable = (e.currentTarget as HTMLSelectElement).value;
          void recomputeSnapshotOnly();
        }}
      >
        <option value="">None (Raw Species)</option>
        {#each lumpOptions as opt (opt.tableName)}
          <option value={opt.name}>
            {opt.tableName} ({opt.activeCount} active)
          </option>
        {/each}
      </select>
    </label>

    <label class="syn-control">
      <span>Significance Basis</span>
      <select
        value={options.significanceBasis ?? 'all_plots'}
        disabled={loading || mutating}
        onchange={e => {
          options.significanceBasis = (e.currentTarget as HTMLSelectElement).value as
            | 'all_plots'
            | 'present_plots';
          void recomputeSnapshotOnly();
        }}
      >
        <option value="all_plots">Mean Cover (All Unit Plots)</option>
        <option value="present_plots">Characteristic Cover (Present Plots)</option>
      </select>
    </label>

    <label class="syn-control">
      <span>Cell Notation</span>
      <select
        value={matrixDisplay}
        onchange={e => {
          matrixDisplay = (e.currentTarget as HTMLSelectElement).value as MatrixDisplayMode;
        }}
      >
        <option value="roman_signif">BEC Class (I..V - +..9)</option>
        <option value="pct_cover">Constancy % / Mean Cover</option>
        <option value="crosstab">Diagnostic10 Code (1..5 - +..9)</option>
      </select>
    </label>

    <label class="syn-control">
      <span>Row Grouping</span>
      <select
        value={matrixGrouping}
        onchange={e => {
          matrixGrouping = (e.currentTarget as HTMLSelectElement).value as MatrixGroupingMode;
        }}
      >
        <option value="diagnostic_unit">Diagnostic Unit (~Unit + Companions)</option>
        <option value="stratum">Stratum (Tree / Shrub / Herb / Moss)</option>
        <option value="lifeform">Lifeform (1..13)</option>
        <option value="flat">Flat Taxonomic List</option>
      </select>
    </label>

    <label class="syn-checkbox-control">
      <input
        type="checkbox"
        checked={options.excludeStandardModifiers}
        disabled={loading || mutating}
        onchange={e => {
          options.excludeStandardModifiers = (e.currentTarget as HTMLInputElement).checked;
          void recomputeSnapshotOnly();
        }}
      />
      <span>
        Exclude <code>.support</code> / <code>.poor</code> / <code>.nudum</code>
        {#if snapshot && snapshot.excludedPlotCount > 0}
          ({snapshot.excludedPlotCount} excluded)
        {/if}
      </span>
    </label>

    <label class="syn-checkbox-control">
      <input
        type="checkbox"
        checked={diagnosticOnly}
        onchange={e => {
          diagnosticOnly = (e.currentTarget as HTMLInputElement).checked;
        }}
      />
      <span>Diagnostic Taxa Only</span>
    </label>
  </div>

  {#if errorMessage}
    <div class="syn-alert syn-alert-error" role="alert">{errorMessage}</div>
  {/if}

  {#if statusNotice}
    <div class="syn-alert syn-alert-info" role="status">
      <span>{statusNotice}</span>
      <button type="button" class="syn-dismiss" onclick={() => (statusNotice = '')}>Dismiss</button>
    </div>
  {/if}

  {#if snapshot && !snapshot.suWritable && snapshot.suReadOnlyReason}
    <div class="syn-readonly-note">
      Read-only classification view: {snapshot.suReadOnlyReason}
    </div>
  {/if}

  <!-- Workspace Split Grid: Primary Synthesis Canvas + Docked Outlier Queue -->
  <div class="syn-workspace-grid">
    <div class="syn-primary-pane">
      <!-- Primary View Switcher -->
      <div class="syn-tabbar" role="tablist" aria-label="Synthesis Views">
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'matrix'}
          class="syn-tab"
          class:active={activeTab === 'matrix'}
          onclick={() => (activeTab = 'matrix')}
        >
          Diagnostic Matrix ({snapshot?.species.length ?? 0} Taxa)
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'similarity'}
          class="syn-tab"
          class:active={activeTab === 'similarity'}
          onclick={() => (activeTab = 'similarity')}
        >
          Pairwise Similarity ({snapshot?.units.length ?? 0}x{snapshot?.units.length ?? 0})
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'edatopic'}
          class="syn-tab"
          class:active={activeTab === 'edatopic'}
          onclick={() => (activeTab = 'edatopic')}
        >
          Edatopic Grid (rSMR 0..8 x rSNR A..F)
        </button>

        {#if activeTab === 'matrix'}
          <input
            type="search"
            class="syn-inline-search"
            placeholder="Filter species, code, lifeform, or unit..."
            value={speciesSearch}
            oninput={e => (speciesSearch = (e.currentTarget as HTMLInputElement).value)}
          />
        {/if}
      </div>

      <!-- Legend Strip for BEC Differential Classes -->
      {#if activeTab === 'matrix'}
        <div class="syn-legend-bar">
          <span class="syn-legend-title">BEC Diagnostic Classes:</span>
          {#each Object.entries(DIAGNOSTIC_CODE_META) as [code, meta] (code)}
            <span class="diag-badge {meta.badgeClass}" title={meta.label}>
              {meta.shortLabel}
            </span>
          {/each}
          {#if snapshot?.lumpTableUsed}
            <span class="syn-lump-pill">
              Lumped via {snapshot.lumpTableUsed} ({snapshot.lumpFoldedTaxa} folded, {snapshot.lumpCappedCells} capped at 99%)
            </span>
          {/if}
          {#if snapshot?.suWritable}
            <span class="syn-drop-hint">
              Tip: Drag any outlier plot card onto a Site Unit column header to reassign live.
            </span>
          {/if}
        </div>
      {/if}

      <!-- TAB 1: Diagnostic Species x SiteUnit Matrix -->
      {#if activeTab === 'matrix'}
        <div class="syn-table-container">
          {#if !snapshot || snapshot.units.length === 0}
            <div class="syn-empty">
              {loading
                ? 'Computing BEC synthesis snapshot...'
                : 'No classified Site Units available in the current scope.'}
            </div>
          {:else}
            <table class="syn-matrix-table">
              <thead>
                <tr>
                  <th class="sticky-col col-code">Code</th>
                  <th class="sticky-col col-sci">Scientific Name</th>
                  <th class="col-lf">Stratum</th>
                  <th class="col-diag">Diagnosis</th>
                  {#each snapshot.units as unit (unit.siteUnit)}
                    <th
                      class="col-unit"
                      class:drop-active={dragOverUnit === unit.siteUnit}
                      ondragover={ev => onUnitDragOver(ev, unit.siteUnit)}
                      ondragleave={() => {
                        if (dragOverUnit === unit.siteUnit) dragOverUnit = null;
                      }}
                      ondrop={ev => onUnitDrop(ev, unit.siteUnit)}
                      title={unit.longName
                        ? `${unit.siteUnit} — ${unit.longName} (n=${unit.plotCount} plots, ${unit.diagnosticCount} diagnostic taxa)`
                        : `${unit.siteUnit} (n=${unit.plotCount} plots)`}
                    >
                      <div class="unit-th-code">{unit.siteUnit}</div>
                      <div class="unit-th-sub">n={unit.plotCount} · diag={unit.diagnosticCount}</div>
                    </th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#each matrixSections as section (section.key)}
                  <tr class="syn-section-row">
                    <td colspan={4 + snapshot.units.length}>
                      <strong>{section.title}</strong>
                      {#if section.subtitle}
                        <span class="syn-section-sub">{section.subtitle}</span>
                      {/if}
                    </td>
                  </tr>
                  {#each section.rows as sp (sp.species)}
                    <tr>
                      <td class="sticky-col col-code mono-cell">
                        {sp.species}
                        {#if sp.sourceSpecies && sp.sourceSpecies.length > 0}
                          <span
                            class="lump-origin-tag"
                            title={`Includes lumped: ${sp.sourceSpecies.join(', ')}`}
                          >
                            +{sp.sourceSpecies.length}
                          </span>
                        {/if}
                      </td>
                      <td class="sticky-col col-sci" title={sp.englishName}>
                        <em>{sp.scientificName}</em>
                      </td>
                      <td class="col-lf">{sp.stratumGroup}</td>
                      <td class="col-diag">
                        {#if sp.diagnosticCodes.length > 0}
                          <div class="diag-cell-badges">
                            {#each sp.diagnosticCodes as dc (dc)}
                              <span
                                class="diag-badge {DIAGNOSTIC_CODE_META[dc]?.badgeClass ?? ''}"
                                title={`${sp.diagnosticUnit}: ${DIAGNOSTIC_CODE_META[dc]?.label ?? dc}`}
                              >
                                {dc}
                              </span>
                            {/each}
                            <span class="diag-unit-ref">{sp.diagnosticUnit}</span>
                          </div>
                        {/if}
                      </td>
                      {#each sp.cells as cell (cell.siteUnit)}
                        <td
                          class="col-unit-val {resolveDiagnosticCellClass(cell.cellDiagnosticCodes)}"
                          title={cell.presentPlots > 0
                            ? `${sp.species} in ${cell.siteUnit}: ${cell.presentPlots}/${cell.plotCount} plots (${cell.presencePctText}%, Class ${cell.presenceRoman}), Mean All=${cell.meanCoverAllText}%, Mean Present=${cell.meanCoverPresentText}%, Signif=${cell.signifClass}`
                            : ''}
                        >
                          {#if cell.presentPlots > 0}
                            <span>
                              {formatSynthesisMatrixCell(
                                cell,
                                matrixDisplay,
                                snapshot.significanceBasis
                              )}
                            </span>
                            {#if cell.isPrimaryDiagnosticUnit && cell.cellDiagnosticCodes.length > 0}
                              <sup class="cell-diag-sup">{cell.cellDiagnosticCodes.join(',')}</sup>
                            {/if}
                          {/if}
                        </td>
                      {/each}
                    </tr>
                  {/each}
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      {/if}

      <!-- TAB 2: Pairwise Unit Similarity Matrix & do_pairwise Differential Inspector -->
      {#if activeTab === 'similarity'}
        <div class="syn-similarity-pane">
          {#if !snapshot || snapshot.similarityMatrix.length === 0}
            <div class="syn-empty">Need at least one classified Site Unit for similarity analysis.</div>
          {:else}
            <div class="syn-subbar">
              <span class="syn-subbar-label">Similarity Metric:</span>
              <button
                type="button"
                class="syn-pill-btn"
                class:active={similarityMetric === 'sorensen'}
                onclick={() => (similarityMetric = 'sorensen')}
              >
                Sorensen / Diagnostic10 (2*min / sum)
              </button>
              <button
                type="button"
                class="syn-pill-btn"
                class:active={similarityMetric === 'bec_diff'}
                onclick={() => (similarityMetric = 'bec_diff')}
              >
                BEC Differential Similarity (BEC.sim.mean)
              </button>
            </div>

            <div class="syn-table-container">
              <table class="syn-sim-table">
                <thead>
                  <tr>
                    <th>Site Unit</th>
                    {#each snapshot.units as u (u.siteUnit)}
                      <th>{u.siteUnit}</th>
                    {/each}
                  </tr>
                </thead>
                <tbody>
                  {#each snapshot.similarityMatrix as row, rIdx (row.unit)}
                    <tr>
                      <th class="sim-row-hdr">{row.unit}</th>
                      {#each row.cells as cell, cIdx (cell.unit)}
                        {@const pct =
                          similarityMetric === 'sorensen' ? cell.similarityPct : cell.becSimMeanPct}
                        <td
                          class="sim-cell"
                          class:sim-diag={rIdx === cIdx}
                          class:sim-high={rIdx !== cIdx && pct >= 70}
                          class:sim-med={rIdx !== cIdx && pct >= 50 && pct < 70}
                          class:sim-selected={
                            (selectedPairUnits[0] === row.unit &&
                              selectedPairUnits[1] === cell.unit) ||
                            (selectedPairUnits[1] === row.unit &&
                              selectedPairUnits[0] === cell.unit)
                          }
                          onclick={() => {
                            if (row.unit !== cell.unit) {
                              selectedPairUnits = [row.unit, cell.unit];
                            }
                          }}
                        >
                          {pct.toFixed(1)}%
                        </td>
                      {/each}
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            {#if activePairComparison}
              <div class="syn-pair-detail">
                <div class="syn-pair-detail-hdr">
                  <strong>
                    Pairwise Differential Breakdown: {activePairComparison.unit1} (n={activePairComparison.plotsUnit1})
                    vs {activePairComparison.unit2} (n={activePairComparison.plotsUnit2})
                  </strong>
                  <span>
                    Sorensen: {activePairComparison.sorensenPct.toFixed(1)}% · BEC.sim.mean:{' '}
                    {(activePairComparison.becSimMean * 100).toFixed(1)}% · Shared Diag Pot:{' '}
                    {activePairComparison.sharedDiagPotential.toFixed(1)}
                  </span>
                </div>
                {#if activePairComparison.differentialTaxa.length === 0}
                  <p class="syn-muted">No species exceed BEC pairwise differential thresholds between these two units.</p>
                {:else}
                  <table class="syn-pair-taxa-table">
                    <thead>
                      <tr>
                        <th>Species</th>
                        <th>Scientific Name</th>
                        <th>{activePairComparison.unit1} (Const% / Cov%)</th>
                        <th>{activePairComparison.unit2} (Const% / Cov%)</th>
                        <th>Diff Pts ({activePairComparison.unit1})</th>
                        <th>Diff Pts ({activePairComparison.unit2})</th>
                        <th>Favours</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each activePairComparison.differentialTaxa.slice(0, 15) as dt (dt.species)}
                        <tr>
                          <td class="mono-cell">{dt.species}</td>
                          <td><em>{dt.scientificName}</em></td>
                          <td>{dt.constancyUnit1.toFixed(0)}% / {dt.meanCoverUnit1.toFixed(1)}%</td>
                          <td>{dt.constancyUnit2.toFixed(0)}% / {dt.meanCoverUnit2.toFixed(1)}%</td>
                          <td>{dt.diffPtsUnit1.toFixed(2)}</td>
                          <td>{dt.diffPtsUnit2.toFixed(2)}</td>
                          <td><strong>{dt.favoursUnit}</strong></td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {/if}
              </div>
            {/if}
          {/if}
        </div>
      {/if}

      <!-- TAB 3: Live 9x6 (rSMR 0..8 x rSNR A..F) Relative Edatopic Grid Heatmap -->
      {#if activeTab === 'edatopic'}
        <div class="syn-edatopic-pane">
          {#if !snapshot}
            <div class="syn-empty">No edatopic data loaded.</div>
          {:else}
            <div class="syn-subbar">
              <span class="syn-subbar-label">Unit Scope:</span>
              <button
                type="button"
                class="syn-pill-btn"
                class:active={selectedEdatopicScope === '__OVERALL__'}
                onclick={() => {
                  selectedEdatopicScope = '__OVERALL__';
                  inspectedEdatopicCell = null;
                }}
              >
                All Classified ({snapshot.overallEdatopicGrid.plottedEdatopicCount}/{snapshot.overallEdatopicGrid.totalPlots})
              </button>
              {#each snapshot.unitEdatopicGrids as ug (ug.siteUnit)}
                <button
                  type="button"
                  class="syn-pill-btn"
                  class:active={selectedEdatopicScope === ug.siteUnit}
                  class:drop-active={dragOverUnit === ug.siteUnit}
                  ondragover={ev => onUnitDragOver(ev, ug.siteUnit)}
                  ondrop={ev => onUnitDrop(ev, ug.siteUnit)}
                  onclick={() => {
                    selectedEdatopicScope = ug.siteUnit;
                    inspectedEdatopicCell = null;
                  }}
                >
                  {ug.siteUnit} ({ug.plottedEdatopicCount}/{ug.totalPlots})
                </button>
              {/each}
            </div>

            {#if activeEdatopicGrid}
              <div class="edatopic-layout">
                <div class="edatopic-grid-card">
                  <div class="edatopic-card-hdr">
                    <strong>{activeEdatopicGrid.label} — Relative Edatopic Grid (9 x 6)</strong>
                    <span>
                      Plotted: {activeEdatopicGrid.plottedEdatopicCount} · Missing SMR/SNR:{' '}
                      {activeEdatopicGrid.missingEdatopicCount}
                    </span>
                  </div>
                  <table class="edatopic-table" aria-label="Relative Edatopic Grid">
                    <thead>
                      <tr>
                        <th class="eda-corner">rSMR \ rSNR</th>
                        {#each EDATOPIC_SNR_COLS as snr (snr)}
                          <th title={SNR_DESCRIPTIONS[snr]}>
                            <div>{snr}</div>
                            <div class="eda-axis-sub">{SNR_DESCRIPTIONS[snr]}</div>
                          </th>
                        {/each}
                      </tr>
                    </thead>
                    <tbody>
                      {#each edatopicRows as rowCells, rIdx (EDATOPIC_SMR_ROWS[rIdx])}
                        {@const smrCode = EDATOPIC_SMR_ROWS[rIdx]}
                        <tr>
                          <th class="eda-row-hdr" title={SMR_DESCRIPTIONS[smrCode]}>
                            <span class="mono-cell">{smrCode}</span>
                            <span class="eda-axis-sub">{SMR_DESCRIPTIONS[smrCode]}</span>
                          </th>
                          {#each rowCells as cell (cell.snr)}
                            {@const heat = edatopicHeatLevel(cell.prominence, cell.plotCount)}
                            <td
                              class="eda-cell eda-heat-{heat}"
                              class:eda-selected={
                                inspectedEdatopicCell?.smr === cell.smr &&
                                inspectedEdatopicCell?.snr === cell.snr
                              }
                              onclick={() => (inspectedEdatopicCell = cell)}
                              title={`rSMR ${cell.smr} / rSNR ${cell.snr}: ${cell.plotCount} plots (${cell.prominence.toFixed(1)}% prominence)`}
                            >
                              {#if cell.plotCount > 0}
                                <div class="eda-count">{cell.plotCount}</div>
                                <div class="eda-prom">{cell.prominence.toFixed(0)}%</div>
                              {/if}
                            </td>
                          {/each}
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>

                <div class="edatopic-inspector">
                  <h3>Cell Plot Membership</h3>
                  {#if !inspectedEdatopicCell}
                    <p class="syn-muted">
                      Select any cell in the 9x6 (rSMR 0..8 x rSNR A..F) grid to inspect contributing plots.
                    </p>
                  {:else}
                    <p class="eda-inspected-title">
                      <strong>rSMR {inspectedEdatopicCell.smr}{inspectedEdatopicCell.snr}</strong>
                      ({SMR_DESCRIPTIONS[inspectedEdatopicCell.smr]} / {SNR_DESCRIPTIONS[inspectedEdatopicCell.snr]})
                      — {inspectedEdatopicCell.plotCount} plots ({inspectedEdatopicCell.prominence.toFixed(1)}% prominence)
                    </p>
                    {#if inspectedEdatopicCell.plotNumbers.length === 0}
                      <p class="syn-muted">No plots occupy this edatopic position.</p>
                    {:else}
                      <div class="eda-plot-chips">
                        {#each inspectedEdatopicCell.plotNumbers as pNum (pNum)}
                          <button
                            type="button"
                            class="plot-jump-chip"
                            onclick={() => onOpenPlot?.(pNum)}
                          >
                            {pNum}
                          </button>
                        {/each}
                      </div>
                    {/if}
                  {/if}
                </div>
              </div>
            {/if}
          {/if}
        </div>
      {/if}
    </div>

    <!-- Right Docked Pane: Candidate Misplaced Plots (LOO Outlier Queue) -->
    <aside class="syn-outlier-pane" aria-label="Candidate Misplaced Plots Queue">
      <div class="outlier-pane-hdr">
        <div>
          <h3>Candidate Misplaced Plots</h3>
          <p>
            Leave-One-Out centroid similarity · {misplacedCount} misplaced of{' '}
            {snapshot?.outlierQueue.length ?? 0}
          </p>
        </div>
      </div>

      <div class="outlier-filter-bar">
        <div class="outlier-pills">
          <button
            type="button"
            class="syn-pill-btn"
            class:active={outlierFilter === 'all'}
            onclick={() => (outlierFilter = 'all')}
          >
            All ({snapshot?.outlierQueue.length ?? 0})
          </button>
          <button
            type="button"
            class="syn-pill-btn"
            class:active={outlierFilter === 'misplaced'}
            onclick={() => (outlierFilter = 'misplaced')}
          >
            Misplaced ({misplacedCount})
          </button>
          <button
            type="button"
            class="syn-pill-btn"
            class:active={outlierFilter === 'top5'}
            onclick={() => (outlierFilter = 'top5')}
          >
            Top 5%
          </button>
        </div>
        <input
          type="search"
          class="outlier-search"
          placeholder="Search plot or unit..."
          value={outlierSearch}
          oninput={e => (outlierSearch = (e.currentTarget as HTMLInputElement).value)}
        />
      </div>

      <div class="outlier-list">
        {#if visibleOutliers.length === 0}
          <div class="syn-empty-small">No plots match the selected outlier filter.</div>
        {:else}
          {#each visibleOutliers as cand (cand.plotNumber)}
            {@const chosenUnit =
              manualTargetByPlot[cand.plotNumber] || cand.nearestRivalUnit || ''}
            <article
              class="outlier-card"
              class:is-misplaced={cand.misplaced}
              class:is-top5={cand.top5PercentAnomaly}
              draggable={Boolean(snapshot?.suWritable)}
              ondragstart={ev => onPlotDragStart(ev, cand.plotNumber)}
              ondragend={onPlotDragEnd}
            >
              <div class="outlier-card-top">
                <div class="outlier-id-group">
                  <span class="outlier-rank">#{cand.rank}</span>
                  <button
                    type="button"
                    class="outlier-plot-link"
                    title={`Open plot ${cand.plotNumber}`}
                    onclick={() => onOpenPlot?.(cand.plotNumber)}
                  >
                    {cand.plotNumber}
                  </button>
                  {#if cand.moistureRegime || cand.nutrientRegime}
                    <span class="outlier-eda-tag">
                      {cand.moistureRegime || '?'}{cand.nutrientRegime || '?'}
                    </span>
                  {/if}
                </div>

                {#if cand.misplaced}
                  <span class="outlier-flag flag-misplaced">Rival Closer (+{cand.deltaSimilarityPct.toFixed(1)}%)</span>
                {:else if cand.singletonUnit}
                  <span class="outlier-flag flag-singleton">Singleton Unit</span>
                {:else if cand.top5PercentAnomaly}
                  <span class="outlier-flag flag-top5">Top 5%</span>
                {/if}
              </div>

              <div class="outlier-metrics">
                <div>
                  <span class="metric-lbl">Assigned:</span>
                  <strong>{cand.assignedUnit}</strong>
                  <span>({cand.ownSimilarityPct.toFixed(1)}% LOO)</span>
                </div>
                {#if cand.nearestRivalUnit}
                  <div>
                    <span class="metric-lbl">Nearest Rival:</span>
                    <strong>{cand.nearestRivalUnit}</strong>
                    <span>({cand.rivalSimilarityPct.toFixed(1)}%)</span>
                  </div>
                {/if}
              </div>

              {#if snapshot && snapshot.units.length > 1}
                <div class="outlier-actions">
                  {#if cand.nearestRivalUnit}
                    <button
                      type="button"
                      class="syn-btn-reassign"
                      disabled={!snapshot.suWritable || mutating}
                      onclick={() => void handleReassignPlot(cand.plotNumber, cand.nearestRivalUnit)}
                    >
                      Move to {cand.nearestRivalUnit}
                    </button>
                  {/if}

                  <select
                    aria-label={`Select target unit for plot ${cand.plotNumber}`}
                    value={chosenUnit}
                    disabled={!snapshot.suWritable || mutating}
                    onchange={e => {
                      const val = (e.currentTarget as HTMLSelectElement).value;
                      manualTargetByPlot[cand.plotNumber] = val;
                      if (val && val !== cand.assignedUnit) {
                        void handleReassignPlot(cand.plotNumber, val);
                      }
                    }}
                  >
                    <option value="">Assign unit...</option>
                    {#each snapshot.units as u (u.siteUnit)}
                      {#if u.siteUnit !== cand.assignedUnit}
                        <option value={u.siteUnit}>{u.siteUnit}</option>
                      {/if}
                    {/each}
                  </select>
                </div>
              {/if}
            </article>
          {/each}
        {/if}
      </div>
    </aside>
  </div>

  <!-- Diagnostic10.xla-Compatible .xlsx Publication Modal -->
  {#if exportModalOpen}
    <div class="syn-modal-backdrop" role="dialog" aria-modal="true" aria-label="Publish BEC Diagnostic Workbook">
      <div class="syn-modal">
        <div class="syn-modal-hdr">
          <h3>Publish BEC Diagnostic Synthesis Workbook (.xlsx)</h3>
          <button type="button" class="syn-dismiss" onclick={() => (exportModalOpen = false)}>
            Close
          </button>
        </div>

        <div class="syn-modal-body">
          <p class="syn-modal-desc">
            Publishes an atomic, non-overwriting 4-sheet workbook compatible with legacy{' '}
            <code>Diagnostic10.xla</code>: BEC colour-coded Diagnostic Summary, Pairwise Similarity
            Matrix, Leave-One-Out Outlier Queue, and 9x6 Edatopic Grids.
          </p>

          <label class="syn-modal-field">
            <span>Output .xlsx File Path (Absolute Path, No Overwrite)</span>
            <input
              type="text"
              placeholder="/path/to/BEC_Diagnostic_Synthesis.xlsx"
              value={exportPath}
              oninput={e => (exportPath = (e.currentTarget as HTMLInputElement).value)}
            />
          </label>

          <label class="syn-modal-field">
            <span>Report Title (Optional)</span>
            <input
              type="text"
              placeholder="VPro BEC Diagnostic Summary Vegetation Report"
              value={exportTitle}
              oninput={e => (exportTitle = (e.currentTarget as HTMLInputElement).value)}
            />
          </label>

          <div class="syn-sheet-grid">
            <label class="syn-modal-field">
              <span>Sheet 1 (Diagnostic)</span>
              <input
                type="text"
                maxlength="31"
                value={exportDiagSheet}
                oninput={e => (exportDiagSheet = (e.currentTarget as HTMLInputElement).value)}
              />
            </label>
            <label class="syn-modal-field">
              <span>Sheet 2 (Similarity)</span>
              <input
                type="text"
                maxlength="31"
                value={exportSimSheet}
                oninput={e => (exportSimSheet = (e.currentTarget as HTMLInputElement).value)}
              />
            </label>
            <label class="syn-modal-field">
              <span>Sheet 3 (Outliers)</span>
              <input
                type="text"
                maxlength="31"
                value={exportOutlierSheet}
                oninput={e => (exportOutlierSheet = (e.currentTarget as HTMLInputElement).value)}
              />
            </label>
            <label class="syn-modal-field">
              <span>Sheet 4 (Edatopic)</span>
              <input
                type="text"
                maxlength="31"
                value={exportEdatopicSheet}
                oninput={e => (exportEdatopicSheet = (e.currentTarget as HTMLInputElement).value)}
              />
            </label>
          </div>

          <div class="syn-modal-checks">
            <label class="syn-checkbox-control">
              <input
                type="checkbox"
                checked={exportIncludeCommon}
                onchange={e => (exportIncludeCommon = (e.currentTarget as HTMLInputElement).checked)}
              />
              <span>Include English Common Names Column</span>
            </label>
            <label class="syn-checkbox-control">
              <input
                type="checkbox"
                checked={exportUnitEdatopicTabs}
                onchange={e =>
                  (exportUnitEdatopicTabs = (e.currentTarget as HTMLInputElement).checked)}
              />
              <span>Append Individual Unit Edatopic Tabs</span>
            </label>
          </div>

          {#if exportValidationMessage}
            <div class="syn-validation-hint">{exportValidationMessage}</div>
          {/if}

          {#if lastExportResult}
            <div class="syn-export-success">
              <strong>Published:</strong> <code>{lastExportResult.path}</code>
              <div>SHA-256: <code>{lastExportResult.sha256.slice(0, 16)}...</code> · Sheets: {lastExportResult.sheetNames.join(', ')}</div>
            </div>
          {/if}
        </div>

        <div class="syn-modal-ftr">
          <button
            type="button"
            class="syn-btn syn-btn-ghost-dark"
            onclick={() => (exportModalOpen = false)}
          >
            Cancel
          </button>
          <button
            type="button"
            class="syn-btn syn-btn-navy"
            disabled={exportingWorkbook || Boolean(exportValidationMessage)}
            onclick={() => void handlePublishWorkbook()}
          >
            {exportingWorkbook ? 'Publishing...' : 'Publish .xlsx Workbook'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</section>

<style>
  .synthesis-workbench {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    min-width: 0;
    color: #0f172a;
  }

  .syn-header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: center;
    gap: 0.85rem;
    padding: 0.9rem 1.15rem;
    background: #003366;
    color: #ffffff;
    border-bottom: 4px solid #fcba19;
    border-radius: 5px 5px 0 0;
  }

  .syn-eyebrow {
    font-size: 0.68rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #fcba19;
  }

  .syn-title-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.55rem;
    margin-top: 0.2rem;
  }

  .syn-title-row h2 {
    margin: 0;
    font-size: 1.15rem;
    font-weight: 700;
    color: #ffffff;
  }

  .syn-chip {
    display: inline-flex;
    align-items: center;
    padding: 0.18rem 0.55rem;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.14);
    color: #f8fafc;
    font-size: 0.75rem;
    font-weight: 600;
  }

  .syn-chip-gold {
    background: #fcba19;
    color: #003366;
  }

  .syn-latency-badge {
    font-family: var(--font-mono, monospace);
    border: 1px solid rgba(252, 186, 25, 0.65);
  }

  .syn-header-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .syn-btn {
    min-height: 36px;
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
    border: 1px solid transparent;
  }

  .syn-btn-ghost {
    background: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.35);
    color: #ffffff;
  }

  .syn-btn-ghost:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
  }

  .syn-btn-gold {
    background: #fcba19;
    color: #003366;
    font-weight: 700;
  }

  .syn-btn-gold:hover:not(:disabled) {
    background: #ffd15c;
  }

  .syn-btn-navy {
    background: #003366;
    color: #ffffff;
  }

  .syn-btn-ghost-dark {
    background: #f1f5f9;
    border-color: #cbd5e1;
    color: #1e293b;
  }

  .syn-btn:disabled {
    opacity: 0.5;
  }

  .syn-toolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: end;
    gap: 0.75rem;
    padding: 0.7rem 0.9rem;
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
  }

  .syn-control {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: #003366;
  }

  .syn-control select {
    min-height: 34px;
    padding: 0.3rem 0.55rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    background: #ffffff;
    font-size: 0.8rem;
    color: #0f172a;
  }

  .syn-checkbox-control {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    min-height: 34px;
    font-size: 0.8rem;
    color: #1e293b;
  }

  .syn-alert {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    padding: 0.55rem 0.85rem;
    border-radius: 4px;
    font-size: 0.82rem;
  }

  .syn-alert-error {
    background: #fef2f2;
    border-left: 4px solid #b91c1c;
    color: #7f1d1d;
  }

  .syn-alert-info {
    background: #eff6ff;
    border-left: 4px solid #003366;
    color: #003366;
  }

  .syn-readonly-note {
    padding: 0.45rem 0.75rem;
    background: #fffbeb;
    border-left: 4px solid #fcba19;
    font-size: 0.78rem;
    color: #78350f;
  }

  .syn-dismiss {
    padding: 0.2rem 0.5rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    background: #ffffff;
    font-size: 0.74rem;
  }

  .syn-workspace-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 340px;
    gap: 0.85rem;
    align-items: start;
  }

  @media (max-width: 1120px) {
    .syn-workspace-grid {
      grid-template-columns: minmax(0, 1fr);
    }
  }

  .syn-primary-pane {
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-radius: 5px;
    min-width: 0;
    overflow: hidden;
  }

  .syn-tabbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.25rem;
    padding: 0.45rem 0.65rem;
    background: #f1f5f9;
    border-bottom: 1px solid #cbd5e1;
  }

  .syn-tab {
    min-height: 34px;
    padding: 0.35rem 0.75rem;
    border: 1px solid transparent;
    border-radius: 4px;
    background: transparent;
    font-size: 0.8rem;
    font-weight: 600;
    color: #334155;
  }

  .syn-tab.active {
    background: #003366;
    color: #ffffff;
    border-bottom: 2px solid #fcba19;
  }

  .syn-inline-search {
    margin-left: auto;
    min-height: 32px;
    min-width: 210px;
    padding: 0.25rem 0.55rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.8rem;
    background: #ffffff;
  }

  .syn-legend-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    padding: 0.45rem 0.75rem;
    background: #f8fafc;
    border-bottom: 1px solid #e2e8f0;
    font-size: 0.75rem;
  }

  .syn-legend-title {
    font-weight: 700;
    color: #003366;
  }

  .diag-badge {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-family: var(--font-mono, monospace);
    font-size: 0.72rem;
    font-weight: 700;
    border: 1px solid rgba(15, 23, 42, 0.18);
  }

  .diag-badge-d {
    background: #bbf7d0;
    color: #14532d;
  }
  .diag-badge-dd {
    background: #fcba19;
    color: #003366;
  }
  .diag-badge-cd {
    background: #bfdbfe;
    color: #1e3a8a;
  }
  .diag-badge-c {
    background: #e0f2fe;
    color: #0c4a6e;
  }
  .diag-badge-ic {
    background: #f5efe0;
    color: #57411d;
  }

  .syn-lump-pill {
    padding: 0.12rem 0.45rem;
    background: #e0f2fe;
    color: #003366;
    border-radius: 3px;
    font-weight: 600;
  }

  .syn-drop-hint {
    margin-left: auto;
    color: #475569;
    font-style: italic;
  }

  .syn-table-container {
    overflow: auto;
    max-height: 68vh;
  }

  .syn-matrix-table,
  .syn-sim-table,
  .syn-pair-taxa-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
  }

  .syn-matrix-table th,
  .syn-matrix-table td {
    padding: 0.38rem 0.55rem;
    border-bottom: 1px solid #e2e8f0;
    border-right: 1px solid #f1f5f9;
  }

  .syn-matrix-table thead th {
    position: sticky;
    top: 0;
    z-index: 3;
    background: #003366;
    color: #ffffff;
    border-bottom: 2px solid #fcba19;
  }

  .col-unit {
    text-align: center;
    min-width: 88px;
  }

  .col-unit.drop-active,
  .syn-pill-btn.drop-active {
    outline: 3px solid #fcba19;
    background: #1d4ed8;
    color: #ffffff;
  }

  .unit-th-code {
    font-weight: 700;
    font-size: 0.8rem;
  }

  .unit-th-sub {
    font-size: 0.68rem;
    color: #cbd5e1;
    font-weight: 400;
  }

  .syn-section-row td {
    background: #e2e8f0;
    color: #003366;
    font-size: 0.78rem;
    padding: 0.35rem 0.65rem;
  }

  .syn-section-sub {
    margin-left: 0.5rem;
    color: #475569;
    font-weight: 400;
  }

  .mono-cell {
    font-family: var(--font-mono, monospace);
    font-size: 0.76rem;
  }

  .lump-origin-tag {
    margin-left: 0.25rem;
    padding: 0.05rem 0.28rem;
    background: #fef3c7;
    color: #92400e;
    border-radius: 3px;
    font-size: 0.66rem;
  }

  .diag-cell-badges {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
  }

  .diag-unit-ref {
    font-size: 0.7rem;
    color: #475569;
  }

  .col-unit-val {
    text-align: center;
    font-family: var(--font-mono, monospace);
    font-size: 0.76rem;
  }

  .cell-diag-sup {
    margin-left: 0.15rem;
    font-weight: 700;
    color: #003366;
  }

  .diag-cell-d-dom {
    background: #fde68a;
    font-weight: 700;
  }
  .diag-cell-dd {
    background: #fef08a;
    font-weight: 700;
  }
  .diag-cell-d {
    background: #dcfce7;
    font-weight: 700;
  }
  .diag-cell-cd {
    background: #dbeafe;
    font-weight: 600;
  }
  .diag-cell-c {
    background: #f0f9ff;
  }
  .diag-cell-ic {
    background: #faf5eb;
  }

  .syn-subbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.4rem;
    padding: 0.6rem 0.8rem;
    border-bottom: 1px solid #e2e8f0;
    background: #f8fafc;
  }

  .syn-subbar-label {
    font-size: 0.76rem;
    font-weight: 700;
    color: #003366;
  }

  .syn-pill-btn {
    padding: 0.25rem 0.6rem;
    border: 1px solid #cbd5e1;
    border-radius: 999px;
    background: #ffffff;
    font-size: 0.75rem;
    font-weight: 600;
    color: #1e293b;
  }

  .syn-pill-btn.active {
    background: #003366;
    border-color: #003366;
    color: #fcba19;
  }

  .syn-sim-table th,
  .syn-sim-table td {
    padding: 0.45rem 0.6rem;
    text-align: center;
    border: 1px solid #e2e8f0;
  }

  .syn-sim-table thead th,
  .sim-row-hdr {
    background: #003366;
    color: #ffffff;
  }

  .sim-cell {
    cursor: pointer;
    font-family: var(--font-mono, monospace);
  }
  .sim-diag {
    background: #f1f5f9;
    color: #64748b;
    cursor: default;
  }
  .sim-high {
    background: #bfdbfe;
    font-weight: 700;
  }
  .sim-med {
    background: #e0f2fe;
  }
  .sim-selected {
    outline: 2px solid #fcba19;
    outline-offset: -2px;
  }

  .syn-pair-detail {
    padding: 0.8rem;
    border-top: 2px solid #003366;
  }

  .syn-pair-detail-hdr {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.5rem;
    margin-bottom: 0.6rem;
    font-size: 0.82rem;
    color: #003366;
  }

  .edatopic-layout {
    display: grid;
    grid-template-columns: minmax(0, 1.4fr) minmax(220px, 1fr);
    gap: 0.85rem;
    padding: 0.8rem;
  }

  @media (max-width: 860px) {
    .edatopic-layout {
      grid-template-columns: 1fr;
    }
  }

  .edatopic-grid-card,
  .edatopic-inspector {
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    padding: 0.7rem;
    background: #ffffff;
  }

  .edatopic-card-hdr {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    gap: 0.4rem;
    margin-bottom: 0.55rem;
    font-size: 0.8rem;
    color: #003366;
  }

  .edatopic-table {
    width: 100%;
    min-width: 420px;
    border-collapse: collapse;
  }

  .edatopic-table th,
  .edatopic-table td {
    border: 1px solid #cbd5e1;
    padding: 0.35rem;
    text-align: center;
  }

  .edatopic-table thead th,
  .eda-corner {
    background: #003366;
    color: #ffffff;
    font-size: 0.75rem;
  }

  .eda-row-hdr {
    background: #f1f5f9;
    color: #003366;
    text-align: left;
    font-size: 0.75rem;
    white-space: nowrap;
  }

  .eda-axis-sub {
    display: block;
    font-size: 0.64rem;
    font-weight: 400;
    opacity: 0.85;
  }

  .eda-cell {
    height: 44px;
    min-width: 54px;
    cursor: pointer;
    vertical-align: middle;
  }

  .eda-heat-0 {
    background: #ffffff;
  }
  .eda-heat-1 {
    background: #fef9c3;
  }
  .eda-heat-2 {
    background: #fde047;
  }
  .eda-heat-3 {
    background: #fcba19;
    font-weight: 700;
    color: #003366;
  }
  .eda-heat-4 {
    background: #003366;
    font-weight: 700;
    color: #fcba19;
  }

  .eda-selected {
    outline: 3px solid #0f172a;
    outline-offset: -3px;
  }

  .eda-count {
    font-family: var(--font-mono, monospace);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .eda-prom {
    font-size: 0.66rem;
  }

  .edatopic-inspector h3 {
    margin: 0 0 0.45rem;
    font-size: 0.88rem;
    color: #003366;
  }

  .eda-plot-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    margin-top: 0.5rem;
  }

  .plot-jump-chip {
    padding: 0.22rem 0.5rem;
    border: 1px solid #003366;
    border-radius: 3px;
    background: #eff6ff;
    color: #003366;
    font-family: var(--font-mono, monospace);
    font-size: 0.75rem;
    font-weight: 600;
  }

  /* Docked Outlier Queue */
  .syn-outlier-pane {
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-top: 4px solid #003366;
    border-radius: 5px;
    display: flex;
    flex-direction: column;
    max-height: 78vh;
  }

  .outlier-pane-hdr {
    padding: 0.65rem 0.8rem;
    border-bottom: 1px solid #e2e8f0;
  }

  .outlier-pane-hdr h3 {
    margin: 0;
    font-size: 0.92rem;
    color: #003366;
  }

  .outlier-pane-hdr p {
    margin: 0.18rem 0 0;
    font-size: 0.74rem;
    color: #475569;
  }

  .outlier-filter-bar {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    padding: 0.55rem 0.75rem;
    background: #f8fafc;
    border-bottom: 1px solid #e2e8f0;
  }

  .outlier-pills {
    display: flex;
    flex-wrap: wrap;
    gap: 0.3rem;
  }

  .outlier-search {
    min-height: 30px;
    padding: 0.25rem 0.5rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.78rem;
  }

  .outlier-list {
    overflow-y: auto;
    padding: 0.55rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .outlier-card {
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    padding: 0.55rem 0.65rem;
    background: #ffffff;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .outlier-card.is-misplaced {
    border-left: 4px solid #fcba19;
    background: #fffdf5;
  }

  .outlier-card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.35rem;
  }

  .outlier-id-group {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  .outlier-rank {
    font-family: var(--font-mono, monospace);
    font-size: 0.72rem;
    color: #64748b;
  }

  .outlier-plot-link {
    border: 0;
    padding: 0;
    background: transparent;
    color: #003366;
    font-family: var(--font-mono, monospace);
    font-weight: 700;
    font-size: 0.82rem;
    text-decoration: underline;
  }

  .outlier-eda-tag {
    padding: 0.06rem 0.3rem;
    background: #e2e8f0;
    border-radius: 3px;
    font-family: var(--font-mono, monospace);
    font-size: 0.7rem;
  }

  .outlier-flag {
    padding: 0.1rem 0.38rem;
    border-radius: 3px;
    font-size: 0.68rem;
    font-weight: 700;
  }

  .flag-misplaced {
    background: #fcba19;
    color: #003366;
  }
  .flag-singleton {
    background: #e2e8f0;
    color: #334155;
  }
  .flag-top5 {
    background: #dbeafe;
    color: #1e3a8a;
  }

  .outlier-metrics {
    font-size: 0.75rem;
    color: #334155;
    display: grid;
    gap: 0.15rem;
  }

  .metric-lbl {
    color: #64748b;
  }

  .outlier-actions {
    display: flex;
    gap: 0.35rem;
    align-items: center;
  }

  .syn-btn-reassign {
    flex: 1;
    min-height: 28px;
    padding: 0.2rem 0.45rem;
    background: #003366;
    color: #fcba19;
    border: 1px solid #003366;
    border-radius: 3px;
    font-size: 0.73rem;
    font-weight: 700;
  }

  .outlier-actions select {
    max-width: 118px;
    min-height: 28px;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    font-size: 0.73rem;
    background: #ffffff;
  }

  .syn-empty,
  .syn-empty-small {
    padding: 2rem;
    text-align: center;
    color: #64748b;
    font-size: 0.85rem;
  }

  .syn-muted {
    color: #64748b;
    font-size: 0.8rem;
  }

  /* Export Modal */
  .syn-modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
    background: rgba(15, 23, 42, 0.55);
    display: grid;
    place-items: center;
    padding: 1rem;
  }

  .syn-modal {
    width: min(620px, 100%);
    background: #ffffff;
    border-top: 5px solid #fcba19;
    border-radius: 5px;
    box-shadow: 0 18px 42px rgba(0, 0, 0, 0.28);
    overflow: hidden;
  }

  .syn-modal-hdr {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.85rem 1.1rem;
    background: #003366;
    color: #ffffff;
  }

  .syn-modal-hdr h3 {
    margin: 0;
    font-size: 0.98rem;
  }

  .syn-modal-body {
    padding: 1rem 1.1rem;
    display: grid;
    gap: 0.75rem;
  }

  .syn-modal-desc {
    margin: 0;
    font-size: 0.8rem;
    color: #334155;
    line-height: 1.45;
  }

  .syn-modal-field {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    font-size: 0.76rem;
    font-weight: 600;
    color: #003366;
  }

  .syn-modal-field input {
    min-height: 36px;
    padding: 0.35rem 0.6rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.82rem;
  }

  .syn-sheet-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.6rem;
  }

  .syn-modal-checks {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
  }

  .syn-validation-hint {
    padding: 0.45rem 0.65rem;
    background: #fef2f2;
    border-left: 3px solid #b91c1c;
    color: #7f1d1d;
    font-size: 0.78rem;
  }

  .syn-export-success {
    padding: 0.55rem 0.75rem;
    background: #f0fdf4;
    border-left: 3px solid #15803d;
    color: #14532d;
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  .syn-modal-ftr {
    display: flex;
    justify-content: flex-end;
    gap: 0.55rem;
    padding: 0.75rem 1.1rem;
    background: #f8fafc;
    border-top: 1px solid #e2e8f0;
  }
</style>
