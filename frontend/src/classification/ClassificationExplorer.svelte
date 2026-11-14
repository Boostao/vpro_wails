<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte';
  import { ReadRequests } from '../shared/readRequests';
  import { computeVirtualWindow } from '../datasheet/universalDatasheetGrid';
  import {
    createEmptyFlatHierarchyRow,
    defaultClassificationBackendPort,
    flattenVisibleHierarchyTree,
    flattenVisibleSUTree,
    isValidBECUnitMask,
    PHYTOSOCIOLOGICAL_RANK_COLUMNS,
    resolveAriaTreeKeyAction,
    UNASSIGNED_BUCKET_ID,
    validateFlatHierarchyRows,
    wouldIntroduceHierarchyCycle,
    type BECUnitPickerChoice,
    type ClassificationBackendPort,
    type CreateSUFromEnvFieldsResult,
    type ExplorerMode,
    type FlatHierarchyRow,
    type HierarchyExplorerView,
    type PlotInspectorSummary,
    type SUFromEnvMode,
    type SUPlotTree,
  } from './classificationExplorer';

  interface Props {
    contextId: string;
    project?: string;
    su?: string;
    hierarchy?: string;
    initialMode?: ExplorerMode;
    operationHeld?: boolean;
    port?: ClassificationBackendPort;
    onBusyChange?: (busy: boolean) => void;
    onDirtyChange?: (dirty: boolean) => void;
    onContextChanged?: (nextContextId: string, newState: unknown) => void;
    onOpenPlot?: (plotNumber: string) => void;
    onRequestSUSync?: (direction: 'su_to_env' | 'env_to_su') => void;
  }

  let {
    contextId,
    project = '',
    su = 'None',
    hierarchy = 'None',
    initialMode = 'su_tree',
    operationHeld = false,
    port = defaultClassificationBackendPort,
    onBusyChange,
    onDirtyChange,
    onContextChanged,
    onOpenPlot,
    onRequestSUSync,
  }: Props = $props();

  const reads = new ReadRequests();
  const TREE_ROW_HEIGHT = 34;

  let mode = $state<ExplorerMode>(untrack(() => initialMode));
  let effectiveContextId = $state(untrack(() => contextId));
  let prevPropContextId = untrack(() => contextId);

  // Keep effectiveContextId synchronized when parent contextId prop updates externally
  $effect(() => {
    if (contextId && contextId !== prevPropContextId) {
      prevPropContextId = contextId;
      if (contextId !== effectiveContextId) {
        effectiveContextId = contextId;
        void reloadActivePane();
      }
    }
  });

  // Left pane SU Tree state
  let suTree = $state.raw<SUPlotTree | null>(null);
  let suFilterText = $state('');
  let expandedSUIds = $state<string[]>([UNASSIGNED_BUCKET_ID]);
  let focusedTreeItemId = $state('');
  let selectedPlots = $state<string[]>([]);
  let lastClickedPlot = $state('');
  let lastHistoryId = $state(0);
  let dragOverBucket = $state<string | null>(null);

  // Virtualized scroll state for tree pane
  let treeScrollTop = $state(0);
  let treeViewportHeight = $state(480);
  let treeViewportEl = $state<HTMLDivElement | null>(null);

  // Right pane Plot Inspector & BECdb v13 Picker state
  let inspectedPlot = $state.raw<PlotInspectorSummary | null>(null);
  let becQuery = $state('');
  let becZone = $state('');
  let becSubZone = $state('');
  let becChoices = $state.raw<BECUnitPickerChoice[]>([]);
  let customTargetUnit = $state('');

  // SU Table New / Save As / Create from Env modals/drawers
  let activeDialog = $state<'none' | 'new_su' | 'save_su_as' | 'su_from_env' | 'new_hierarchy'>('none');
  let newSUName = $state('');
  let populateAllPlots = $state(true);
  let seedFromUserSU = $state(false);
  let saveAsNewName = $state('');

  // Create SU from Env Fields state
  const AVAILABLE_ENV_FIELDS = [
    'Zone',
    'SubZone',
    'SiteSeries',
    'SiteModifier1',
    'MoistureRegime',
    'NutrientRegime',
    'StructuralStage',
    'Ecosection',
    'UserSiteUnit',
    'GIS_BGC',
  ];
  let envConcatFields = $state<string[]>(['Zone', 'SubZone', 'SiteSeries']);
  let envSeparator = $state('/');
  let envUseBECSlash = $state(true);
  let envConcatMode = $state<SUFromEnvMode>('fill_unassigned_only');
  let envNewSUName = $state('');
  let envSkipIncomplete = $state(false);
  let envConcatPreview = $state.raw<CreateSUFromEnvFieldsResult | null>(null);

  // Hierarchy Tree & Flat Rank Table state
  let hierarchyView = $state.raw<HierarchyExplorerView | null>(null);
  let expandedHierarchyIds = $state<number[]>([]);
  let selectedHierarchyNodeId = $state<number | null>(null);
  let newHierarchyTableName = $state('');
  let addNodeName = $state('');
  let addNodeLevel = $state(11);
  let addNodeBreak = $state(false);
  let addAsRoot = $state(false);
  let renameDraft = $state('');
  let reparentTargetId = $state<string>('');

  let flatDraftRows = $state<FlatHierarchyRow[]>([]);
  let flatDirty = $state(false);

  let busy = $state(false);
  let mutating = $state(false);
  let errorMessage = $state('');
  let statusBanner = $state('');

  let generation = 0;

  const flatSUItems = $derived(flattenVisibleSUTree(suTree, new Set(expandedSUIds)));
  const suVirtualSlice = $derived(
    computeVirtualWindow(treeScrollTop, treeViewportHeight, TREE_ROW_HEIGHT, flatSUItems.length, 8)
  );
  const visibleSUItems = $derived(
    flatSUItems.slice(suVirtualSlice.startIndex, suVirtualSlice.endIndex)
  );

  const flatHierarchyItems = $derived(
    flattenVisibleHierarchyTree(hierarchyView?.roots ?? [], new Set(expandedHierarchyIds))
  );
  const hierVirtualSlice = $derived(
    computeVirtualWindow(
      treeScrollTop,
      treeViewportHeight,
      TREE_ROW_HEIGHT,
      flatHierarchyItems.length,
      8
    )
  );
  const visibleHierarchyItems = $derived(
    flatHierarchyItems.slice(hierVirtualSlice.startIndex, hierVirtualSlice.endIndex)
  );

  const flatValidationError = $derived(flatDirty ? validateFlatHierarchyRows(flatDraftRows) : '');

  const selectedHierarchyNode = $derived(
    hierarchyView?.flatNodes.find(n => n.id === selectedHierarchyNodeId) ?? null
  );

  $effect(() => {
    onBusyChange?.(busy || mutating || flatDirty || !!flatValidationError);
    onDirtyChange?.(!busy && !mutating && flatDirty);
  });

  async function reloadActivePane() {
    if (!effectiveContextId) return;
    if (mode === 'su_tree') {
      await loadSUTreeAndPicker();
    } else {
      await loadHierarchy();
    }
  }

  async function loadSUTreeAndPicker() {
    if (!effectiveContextId) return;
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const [tree, picker] = await Promise.all([
        reads.track(port.getSUPlotTree(effectiveContextId, suFilterText)),
        reads.track(
          port.searchBECUnitPicker(effectiveContextId, {
            query: becQuery,
            zone: becZone,
            subZone: becSubZone,
            limit: 80,
          })
        ),
      ]);
      if (reqGen !== generation) return;
      suTree = tree;
      becChoices = picker ?? [];
      // Auto-expand first 5 unit buckets on initial load for quick scanning
      if (expandedSUIds.length <= 1 && tree.units.length > 0) {
        expandedSUIds = [
          UNASSIGNED_BUCKET_ID,
          ...tree.units.slice(0, 5).map(u => `unit::${u.siteUnit}`),
        ];
      }
    } catch (err) {
      if (reqGen === generation) errorMessage = String(err);
    } finally {
      if (reqGen === generation) busy = false;
    }
  }

  async function refreshBECPicker() {
    if (!effectiveContextId) return;
    try {
      const choices = await reads.track(
        port.searchBECUnitPicker(effectiveContextId, {
          query: becQuery,
          zone: becZone,
          subZone: becSubZone,
          limit: 80,
        })
      );
      becChoices = choices ?? [];
    } catch (err) {
      errorMessage = String(err);
    }
  }

  async function inspectPlot(plotNumber: string) {
    if (!effectiveContextId || !plotNumber) return;
    try {
      const summary = await reads.track(
        port.getPlotInspectorSummary(effectiveContextId, plotNumber)
      );
      inspectedPlot = summary;
      if (summary.zone && !becZone) {
        becZone = summary.zone;
      }
    } catch (err) {
      errorMessage = `Plot inspector failed: ${String(err)}`;
    }
  }

  async function loadHierarchy() {
    if (!effectiveContextId) return;
    generation++;
    const reqGen = generation;
    reads.cancelAll();
    busy = true;
    errorMessage = '';
    try {
      const view = await reads.track(port.getHierarchyExplorer(effectiveContextId));
      if (reqGen !== generation) return;
      hierarchyView = view;
      if (expandedHierarchyIds.length === 0 && view.flatNodes.length > 0) {
        expandedHierarchyIds = view.flatNodes.filter(n => n.depth <= 2).map(n => n.id);
      }
      flatDraftRows = (view.flatRankRows ?? []).map(r => ({ ...r }));
      flatDirty = false;
    } catch (err) {
      if (reqGen === generation) errorMessage = String(err);
    } finally {
      if (reqGen === generation) busy = false;
    }
  }

  function toggleSUExpand(folderId: string) {
    if (expandedSUIds.includes(folderId)) {
      expandedSUIds = expandedSUIds.filter(id => id !== folderId);
    } else {
      expandedSUIds = [...expandedSUIds, folderId];
    }
  }

  function handlePlotClick(plotNumber: string, event: MouseEvent | KeyboardEvent) {
    if (event.ctrlKey || event.metaKey) {
      if (selectedPlots.includes(plotNumber)) {
        selectedPlots = selectedPlots.filter(p => p !== plotNumber);
      } else {
        selectedPlots = [...selectedPlots, plotNumber];
      }
    } else if (event.shiftKey && lastClickedPlot) {
      const leafPlots = flatSUItems
        .filter(i => i.kind === 'plot_leaf' && i.plot)
        .map(i => i.plot!.plotNumber);
      const a = leafPlots.indexOf(lastClickedPlot);
      const b = leafPlots.indexOf(plotNumber);
      if (a >= 0 && b >= 0) {
        const [lo, hi] = a < b ? [a, b] : [b, a];
        selectedPlots = Array.from(new Set([...selectedPlots, ...leafPlots.slice(lo, hi + 1)]));
      } else {
        selectedPlots = [plotNumber];
      }
    } else {
      selectedPlots = [plotNumber];
    }
    lastClickedPlot = plotNumber;
    void inspectPlot(plotNumber);
  }

  async function assignPlots(plotsToMove: string[], targetUnit: string | null) {
    if (plotsToMove.length === 0 || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.assignPlotsToSiteUnit(effectiveContextId, {
        plotNumbers: $state.snapshot(plotsToMove),
        targetSiteUnit: targetUnit,
        removeFromSuWhenUnassigned: false,
      });
      lastHistoryId = res.historyId;
      if (res.changedPlots === 0) {
        statusBanner = targetUnit
          ? `0 plot(s) changed in ${res.tableName} (plots are already assigned to ${targetUnit}, or the active Site Unit table is read-only/master).`
          : `0 plot(s) changed in ${res.tableName} (plots are already unassigned, or the active Site Unit table is read-only/master).`;
      } else {
        statusBanner = targetUnit
          ? `Assigned ${res.changedPlots} plot(s) to ${targetUnit}.`
          : `Moved ${res.changedPlots} plot(s) to Unassigned.`;
      }
      await loadSUTreeAndPicker();
      if (inspectedPlot && plotsToMove.includes(inspectedPlot.plotNumber)) {
        await inspectPlot(inspectedPlot.plotNumber);
      }
    } catch (err) {
      errorMessage = `Plot assignment failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function undoLastAssignment() {
    if (mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.undoSUAssignment(effectiveContextId, lastHistoryId);
      lastHistoryId = 0;
      statusBanner = `Reverted ${res.changedPlots} plot assignment(s).`;
      await loadSUTreeAndPicker();
    } catch (err) {
      errorMessage = `Undo failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  function handlePlotDragStart(event: DragEvent, plotNumber: string) {
    const batch = selectedPlots.includes(plotNumber) ? selectedPlots : [plotNumber];
    selectedPlots = batch;
    event.dataTransfer?.setData('application/x-vpro-plots', JSON.stringify(batch));
    event.dataTransfer?.setData('text/plain', batch.join(','));
  }

  function handleBucketDrop(event: DragEvent, targetUnit: string | null) {
    event.preventDefault();
    dragOverBucket = null;
    const raw = event.dataTransfer?.getData('application/x-vpro-plots');
    let batch: string[] = [];
    if (raw) {
      try {
        batch = JSON.parse(raw);
      } catch {
        batch = [];
      }
    }
    if (batch.length === 0) batch = selectedPlots;
    if (batch.length > 0) {
      void assignPlots(batch, targetUnit);
    }
  }

  function handleSUTreeKeyDown(event: KeyboardEvent) {
    const action = resolveAriaTreeKeyAction(flatSUItems, focusedTreeItemId, event.key);
    if (!action) return;
    event.preventDefault();
    if (action.type === 'focus') {
      focusedTreeItemId = action.targetId;
      queueMicrotask(() => {
        treeViewportEl
          ?.querySelector<HTMLElement>(`[data-tree-item-id="${CSS.escape(action.targetId)}"]`)
          ?.focus();
      });
    } else if (action.type === 'expand' || action.type === 'collapse') {
      toggleSUExpand(action.targetId);
    } else if (action.type === 'select') {
      const item = flatSUItems.find(i => i.id === action.targetId);
      if (!item) return;
      if (item.kind === 'plot_leaf' && item.plot) {
        handlePlotClick(item.plot.plotNumber, event);
      } else {
        toggleSUExpand(item.id);
      }
    }
  }

  function toggleHierarchyExpand(nodeId: number) {
    if (expandedHierarchyIds.includes(nodeId)) {
      expandedHierarchyIds = expandedHierarchyIds.filter(id => id !== nodeId);
    } else {
      expandedHierarchyIds = [...expandedHierarchyIds, nodeId];
    }
  }

  function handleHierarchyTreeKeyDown(event: KeyboardEvent) {
    const mapped = flatHierarchyItems.map(i => ({
      id: i.id,
      parentId: i.parentItemId,
      expandable: i.expandable,
      expanded: i.expanded,
    }));
    const action = resolveAriaTreeKeyAction(mapped, focusedTreeItemId, event.key);
    if (!action) return;
    event.preventDefault();
    const targetItem = flatHierarchyItems.find(i => i.id === action.targetId);
    if (!targetItem) return;
    if (action.type === 'focus') {
      focusedTreeItemId = action.targetId;
      selectedHierarchyNodeId = targetItem.nodeId;
      renameDraft = targetItem.name;
      queueMicrotask(() => {
        treeViewportEl
          ?.querySelector<HTMLElement>(`[data-tree-item-id="${CSS.escape(action.targetId)}"]`)
          ?.focus();
      });
    } else if (action.type === 'expand' || action.type === 'collapse') {
      toggleHierarchyExpand(targetItem.nodeId);
    } else if (action.type === 'select') {
      selectedHierarchyNodeId = targetItem.nodeId;
      renameDraft = targetItem.name;
    }
  }

  async function submitCreateSU() {
    if (!newSUName.trim() || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.createSUTable(effectiveContextId, {
        name: newSUName.trim(),
        destination: 'project',
        filePath: '',
        populateAllProjectPlots: populateAllPlots,
        seedFromUserSiteUnit: seedFromUserSU,
        activate: true,
      });
      if (res.activated) {
        effectiveContextId = res.contextId;
        onContextChanged?.(res.contextId, res.newState);
      }
      activeDialog = 'none';
      newSUName = '';
      statusBanner = `Created and activated ${res.tableName} (${res.plotCount} plot(s)).`;
      await loadSUTreeAndPicker();
    } catch (err) {
      errorMessage = `Create SU failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitSaveSUAs() {
    if (!saveAsNewName.trim() || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.saveSUTableAs(effectiveContextId, {
        sourceSu: '',
        newName: saveAsNewName.trim(),
        destination: 'project',
        filePath: '',
        activate: true,
      });
      if (res.activated) {
        effectiveContextId = res.contextId;
        onContextChanged?.(res.contextId, res.newState);
      }
      activeDialog = 'none';
      saveAsNewName = '';
      statusBanner = `Saved SU copy as ${res.tableName} (${res.plotCount} plot(s)).`;
      await loadSUTreeAndPicker();
    } catch (err) {
      errorMessage = `Save SU As failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function runSUFromEnv(previewOnly: boolean) {
    if (envConcatFields.length === 0 || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.createSUFromEnvFields(effectiveContextId, {
        fields: $state.snapshot(envConcatFields),
        separator: envSeparator,
        useBecSlashFormat: envUseBECSlash,
        mode: envConcatMode,
        targetSuName: envConcatMode === 'new_table' ? envNewSUName.trim() : undefined,
        destination: 'project',
        scopeToExistingSuRowsOnly: false,
        skipIncompleteRows: envSkipIncomplete,
        previewOnly,
        activate: envConcatMode === 'new_table',
      });
      if (previewOnly) {
        envConcatPreview = res;
      } else {
        if (res.activated) {
          effectiveContextId = res.contextId;
          onContextChanged?.(res.contextId, res.newState);
        }
        envConcatPreview = null;
        activeDialog = 'none';
        statusBanner = `Populated ${res.tableName}: ${res.insertedCount} inserted, ${res.updatedCount} updated, ${res.preservedCount} preserved.`;
        await loadSUTreeAndPicker();
      }
    } catch (err) {
      errorMessage = `Create SU from Env Fields failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitCreateHierarchyTable() {
    if (!newHierarchyTableName.trim() || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.createHierarchyTable(effectiveContextId, {
        name: newHierarchyTableName.trim(),
        destination: 'project',
        filePath: '',
        activate: true,
      });
      if (res.activated) {
        effectiveContextId = res.contextId;
        onContextChanged?.(res.contextId, res.newState);
      }
      activeDialog = 'none';
      newHierarchyTableName = '';
      statusBanner = `Created and activated ${res.tableName}.`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Create Hierarchy failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitAddHierarchyNode() {
    if (!addNodeName.trim() || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const parentId = addAsRoot ? null : selectedHierarchyNodeId;
      const res = await port.addHierarchyNode(effectiveContextId, {
        name: addNodeName.trim(),
        parentId,
        level: addNodeLevel,
        isBreakpoint: addNodeBreak,
      });
      addNodeName = '';
      if (parentId !== null && !expandedHierarchyIds.includes(parentId)) {
        expandedHierarchyIds = [...expandedHierarchyIds, parentId];
      }
      statusBanner = `Added hierarchy node "${res.nodeName}".`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Add node failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitToggleBreak(nodeId: number) {
    if (mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.toggleHierarchyBreakpoint(effectiveContextId, nodeId);
      statusBanner = `Updated breakpoint status: ${res.nodeName}.`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Breakpoint toggle failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitRenameNode() {
    if (selectedHierarchyNodeId === null || !renameDraft.trim() || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.renameHierarchyNode(effectiveContextId, {
        nodeId: selectedHierarchyNodeId,
        newName: renameDraft.trim(),
      });
      statusBanner = `Renamed node to "${res.nodeName}".`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Rename failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitReparentNode() {
    if (selectedHierarchyNodeId === null || mutating || !hierarchyView) return;
    const candidateParent = reparentTargetId === '' ? null : Number(reparentTargetId);
    if (
      wouldIntroduceHierarchyCycle(
        hierarchyView.flatNodes,
        selectedHierarchyNodeId,
        candidateParent
      )
    ) {
      errorMessage = 'Cannot reparent a node onto itself or one of its own descendants (cycle).';
      return;
    }
    mutating = true;
    errorMessage = '';
    try {
      await port.reparentHierarchyNode(effectiveContextId, {
        nodeId: selectedHierarchyNodeId,
        newParentId: candidateParent,
      });
      statusBanner = 'Reparented hierarchy node.';
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Reparent failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitDeleteNode(cascade: boolean) {
    if (selectedHierarchyNodeId === null || mutating) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.deleteHierarchyNode(effectiveContextId, {
        nodeId: selectedHierarchyNodeId,
        cascadeChildren: cascade,
      });
      selectedHierarchyNodeId = null;
      statusBanner = `Deleted ${res.affectedRows} hierarchy node(s).`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Delete node failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  async function submitSaveFlatRows() {
    if (mutating || flatValidationError) return;
    mutating = true;
    errorMessage = '';
    try {
      const res = await port.saveFlatHierarchyRows(effectiveContextId, {
        rows: $state.snapshot(flatDraftRows),
      });
      flatDirty = false;
      statusBanner = `Synchronized ${res.affectedRows} hierarchy node(s) from flat phytosociological table.`;
      await loadHierarchy();
    } catch (err) {
      errorMessage = `Save Flat Hierarchy failed: ${String(err)}`;
    } finally {
      mutating = false;
    }
  }

  onMount(() => {
    if (effectiveContextId) {
      void reloadActivePane();
    }
  });

  onDestroy(() => {
    generation++;
    reads.cancelAll();
    onDirtyChange?.(false);
    onBusyChange?.(false);
  });
</script>

<section class="vpro-cls-explorer" aria-label="Dual-Pane Classification Explorer">
  <header class="vpro-cls-header">
    <div>
      <p class="vpro-cls-eyebrow">BEC CLASSIFICATION WORKBENCH</p>
      <h2 class="vpro-cls-title">
        Classification Explorer
        {#if project}<span class="vpro-cls-pill">Project: {project}</span>{/if}
        <span class="vpro-cls-pill">SU: {suTree?.su || su}</span>
        <span class="vpro-cls-pill">Hierarchy: {hierarchyView?.hierarchy || hierarchy}</span>
      </h2>
    </div>

    <div class="vpro-cls-mode-bar" role="tablist" aria-label="Classification Explorer Views">
      <button
        type="button"
        role="tab"
        aria-selected={mode === 'su_tree'}
        class="vpro-cls-mode-btn"
        class:active={mode === 'su_tree'}
        onclick={() => {
          mode = 'su_tree';
          treeScrollTop = 0;
          void loadSUTreeAndPicker();
        }}
      >
        SU -&gt; Plots Tree
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={mode === 'hierarchy_tree'}
        class="vpro-cls-mode-btn"
        class:active={mode === 'hierarchy_tree'}
        onclick={() => {
          mode = 'hierarchy_tree';
          treeScrollTop = 0;
          void loadHierarchy();
        }}
      >
        Hierarchy Tree (~)
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={mode === 'hierarchy_flat'}
        class="vpro-cls-mode-btn"
        class:active={mode === 'hierarchy_flat'}
        onclick={() => {
          mode = 'hierarchy_flat';
          void loadHierarchy();
        }}
      >
        Flat Rank Table
      </button>
    </div>
  </header>

  {#if errorMessage}
    <p role="alert" class="vpro-cls-alert">{errorMessage}</p>
  {/if}
  {#if statusBanner}
    <p role="status" class="vpro-cls-status">{statusBanner}</p>
  {/if}

  {#if activeDialog === 'new_su'}
    <section class="vpro-cls-dialog" aria-label="Create New Site Unit Table">
      <h3>Create Standalone Site Unit (*_SU) Table</h3>
      <div class="vpro-cls-dialog-row">
        <label>
          <span>SU Name (without _SU)</span>
          <input type="text" bind:value={newSUName} placeholder="e.g. Working2026" />
        </label>
        <label class="vpro-cls-check">
          <input type="checkbox" bind:checked={populateAllPlots} />
          <span>Include all project plots</span>
        </label>
        <label class="vpro-cls-check">
          <input type="checkbox" bind:checked={seedFromUserSU} disabled={!populateAllPlots} />
          <span>Seed from Admin.UserSiteUnit</span>
        </label>
        <button type="button" class="vpro-btn-primary" disabled={mutating} onclick={submitCreateSU}>
          Create &amp; Activate
        </button>
        <button type="button" class="vpro-btn-secondary" onclick={() => (activeDialog = 'none')}>
          Cancel
        </button>
      </div>
    </section>
  {:else if activeDialog === 'save_su_as'}
    <section class="vpro-cls-dialog" aria-label="Save Site Unit Table As">
      <h3>Save Active SU Table Under New Name</h3>
      <div class="vpro-cls-dialog-row">
        <label>
          <span>New SU Name</span>
          <input type="text" bind:value={saveAsNewName} placeholder="e.g. RegionalSubset" />
        </label>
        <button type="button" class="vpro-btn-primary" disabled={mutating} onclick={submitSaveSUAs}>
          Save Copy &amp; Activate
        </button>
        <button type="button" class="vpro-btn-secondary" onclick={() => (activeDialog = 'none')}>
          Cancel
        </button>
      </div>
    </section>
  {:else if activeDialog === 'su_from_env'}
    <section class="vpro-cls-dialog" aria-label="Create Site Units from Environment Fields">
      <h3>Populate Site Units from Concatenated Env / Admin Fields</h3>
      <div class="vpro-cls-dialog-row">
        <label>
          <span>Target Mode</span>
          <select bind:value={envConcatMode}>
            <option value="fill_unassigned_only">Existing SU — Fill Unassigned Only</option>
            <option value="overwrite">Existing SU — Overwrite All Plots</option>
            <option value="new_table">Create New *_SU Table</option>
          </select>
        </label>
        {#if envConcatMode === 'new_table'}
          <label>
            <span>New SU Table Name</span>
            <input type="text" bind:value={envNewSUName} placeholder="e.g. FieldBEC" />
          </label>
        {/if}
        <label>
          <span>Separator</span>
          <input type="text" bind:value={envSeparator} style="width:5rem;" />
        </label>
        <label class="vpro-cls-check">
          <input type="checkbox" bind:checked={envUseBECSlash} />
          <span>BEC format (Zone+SubZone + "/" + SiteSeries)</span>
        </label>
        <label class="vpro-cls-check">
          <input type="checkbox" bind:checked={envSkipIncomplete} />
          <span>Skip incomplete rows</span>
        </label>
      </div>

      <div class="vpro-cls-field-chips" aria-label="Selected Env/Admin Columns">
        {#each AVAILABLE_ENV_FIELDS as fieldName (fieldName)}
          {@const selected = envConcatFields.includes(fieldName)}
          <button
            type="button"
            class="vpro-cls-chip"
            class:selected
            onclick={() => {
              if (selected) {
                envConcatFields = envConcatFields.filter(f => f !== fieldName);
              } else {
                envConcatFields = [...envConcatFields, fieldName];
              }
            }}
          >
            {selected ? `${envConcatFields.indexOf(fieldName) + 1}. ` : '+ '}{fieldName}
          </button>
        {/each}
      </div>

      <div class="vpro-cls-dialog-row">
        <button
          type="button"
          class="vpro-btn-secondary"
          disabled={mutating || envConcatFields.length === 0}
          onclick={() => runSUFromEnv(true)}
        >
          Preview Assignments
        </button>
        <button
          type="button"
          class="vpro-btn-primary"
          disabled={mutating || envConcatFields.length === 0}
          onclick={() => runSUFromEnv(false)}
        >
          Apply to SU
        </button>
        <button type="button" class="vpro-btn-secondary" onclick={() => (activeDialog = 'none')}>
          Close
        </button>
      </div>

      {#if envConcatPreview}
        <p class="vpro-cls-preview-note">
          Preview for <code>{envConcatPreview.tableName}</code>:
          <strong>{envConcatPreview.insertedCount}</strong> insert,
          <strong>{envConcatPreview.updatedCount}</strong> update,
          <strong>{envConcatPreview.preservedCount}</strong> preserved,
          <strong>{envConcatPreview.skippedCount}</strong> blank skipped.
        </p>
      {/if}
    </section>
  {:else if activeDialog === 'new_hierarchy'}
    <section class="vpro-cls-dialog" aria-label="Create New Hierarchy Table">
      <h3>Create Standalone *_Hierarchy Table</h3>
      <div class="vpro-cls-dialog-row">
        <label>
          <span>Hierarchy Name (without _Hierarchy)</span>
          <input type="text" bind:value={newHierarchyTableName} placeholder="e.g. Coast2026" />
        </label>
        <button
          type="button"
          class="vpro-btn-primary"
          disabled={mutating}
          onclick={submitCreateHierarchyTable}
        >
          Create &amp; Activate
        </button>
        <button type="button" class="vpro-btn-secondary" onclick={() => (activeDialog = 'none')}>
          Cancel
        </button>
      </div>
    </section>
  {/if}

  {#if mode === 'hierarchy_flat'}
    <section class="vpro-cls-flat-pane" aria-label="Editable Flat Phytosociological Hierarchy Table">
      <div class="vpro-cls-subtoolbar">
        <div>
          <strong>Wide Phytosociological Rank Editor</strong>
          <span class="vpro-cls-muted">
            Prefix any branch/unit name with <code>~</code> to mark a Summary Vegetation breakpoint.
          </span>
        </div>
        <div class="vpro-cls-actions">
          <button
            type="button"
            class="vpro-btn-secondary"
            onclick={() => (activeDialog = 'new_hierarchy')}
          >
            New Hierarchy
          </button>
          <button
            type="button"
            class="vpro-btn-secondary"
            onclick={() => {
              flatDraftRows = [...flatDraftRows, createEmptyFlatHierarchyRow()];
              flatDirty = true;
            }}
          >
            + Add Rank Row
          </button>
          <button
            type="button"
            class="vpro-btn-primary"
            disabled={!flatDirty || !!flatValidationError || mutating}
            onclick={submitSaveFlatRows}
          >
            {mutating ? 'Saving...' : 'Save Flat Hierarchy'}
          </button>
        </div>
      </div>

      {#if flatValidationError}
        <p role="alert" class="vpro-cls-alert">{flatValidationError}</p>
      {/if}

      <div class="vpro-cls-flat-scroll">
        <table class="vpro-cls-flat-table">
          <thead>
            <tr>
              <th>#</th>
              {#each PHYTOSOCIOLOGICAL_RANK_COLUMNS as col (col.key)}
                <th>{col.label}</th>
              {/each}
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each flatDraftRows as row, idx (idx)}
              <tr>
                <td class="mono">{idx + 1}</td>
                {#each PHYTOSOCIOLOGICAL_RANK_COLUMNS as col (col.key)}
                  {@const cellVal = row[col.key]}
                  <td class:breakpoint-cell={cellVal.trim().startsWith('~')}>
                    <input
                      type="text"
                      value={cellVal}
                      placeholder={col.label}
                      oninput={e => {
                        flatDraftRows[idx] = {
                          ...flatDraftRows[idx],
                          [col.key]: e.currentTarget.value,
                        };
                        flatDirty = true;
                      }}
                    />
                  </td>
                {/each}
                <td>
                  <button
                    type="button"
                    class="vpro-btn-mini"
                    onclick={() => {
                      flatDraftRows = flatDraftRows.filter((_, i) => i !== idx);
                      flatDirty = true;
                    }}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            {/each}
            {#if flatDraftRows.length === 0}
              <tr>
                <td colspan={PHYTOSOCIOLOGICAL_RANK_COLUMNS.length + 2} class="vpro-cls-empty">
                  No hierarchy rows defined yet. Click "+ Add Rank Row" or "New Hierarchy" to begin.
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      </div>
    </section>
  {:else}
    <div class="vpro-cls-dual-grid">
      <!-- LEFT PANE: Virtualized Accessible WAI-ARIA Tree -->
      <section class="vpro-cls-left-pane" aria-label="Classification Tree">
        {#if mode === 'su_tree'}
          <div class="vpro-cls-subtoolbar">
            <input
              type="search"
              class="vpro-cls-filter"
              placeholder="Filter units or plots..."
              bind:value={suFilterText}
              onkeydown={e => {
                if (e.key === 'Enter') void loadSUTreeAndPicker();
              }}
            />
            <button type="button" class="vpro-btn-secondary" onclick={() => loadSUTreeAndPicker()}>
              Filter
            </button>
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => (activeDialog = 'new_su')}
            >
              New SU
            </button>
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => (activeDialog = 'save_su_as')}
            >
              Save As
            </button>
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => (activeDialog = 'su_from_env')}
            >
              SU from Env
            </button>
            {#if lastHistoryId > 0}
              <button
                type="button"
                class="vpro-btn-secondary"
                disabled={mutating}
                onclick={undoLastAssignment}
              >
                Undo
              </button>
            {/if}
          </div>

          {#if suTree}
            <p class="vpro-cls-counts">
              <strong>{suTree.units.length}</strong> Site Units ·
              <strong>{suTree.totalAssigned}</strong> Assigned ·
              <strong>{suTree.totalUnassigned}</strong> Unassigned ·
              <strong>{selectedPlots.length}</strong> Selected
            </p>
          {/if}

          <div
            class="vpro-cls-tree-viewport"
            bind:this={treeViewportEl}
            bind:clientHeight={treeViewportHeight}
            onscroll={e => {
              treeScrollTop = e.currentTarget.scrollTop;
            }}
          >
            <div
              role="tree"
              aria-label="Site Unit to Plots Assignment Tree"
              aria-multiselectable="true"
              tabindex="-1"
              onkeydown={handleSUTreeKeyDown}
            >
              {#if suVirtualSlice.topSpacerPx > 0}
                <div style={`height:${suVirtualSlice.topSpacerPx}px;`} aria-hidden="true"></div>
              {/if}

              {#each visibleSUItems as item (item.id)}
                {#if item.kind === 'unit_folder' || item.kind === 'unassigned_folder'}
                  <div
                    role="treeitem"
                    tabindex={focusedTreeItemId === item.id || !focusedTreeItemId ? 0 : -1}
                    data-tree-item-id={item.id}
                    aria-level={1}
                    aria-expanded={item.expandable ? item.expanded : undefined}
                    aria-selected={false}
                    class="vpro-tree-folder"
                    class:drop-target={dragOverBucket === item.id}
                    class:unassigned={item.kind === 'unassigned_folder'}
                    style={`height:${TREE_ROW_HEIGHT}px;`}
                    onfocus={() => (focusedTreeItemId = item.id)}
                    onclick={() => toggleSUExpand(item.id)}
                    onkeydown={e => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        toggleSUExpand(item.id);
                      }
                    }}
                    ondragover={e => {
                      e.preventDefault();
                      dragOverBucket = item.id;
                    }}
                    ondragleave={() => {
                      if (dragOverBucket === item.id) dragOverBucket = null;
                    }}
                    ondrop={e => handleBucketDrop(e, item.unitCode)}
                  >
                    <span class="vpro-tree-caret">
                      {item.expandable ? (item.expanded ? '[-]' : '[+]') : '[ ]'}
                    </span>
                    <span class="vpro-tree-unit-badge">{item.kind === 'unit_folder' ? 'U' : '0'}</span>
                    <strong class="mono">{item.label}</strong>
                    {#if item.subtitle}
                      <span class="vpro-tree-sub">{item.subtitle}</span>
                    {/if}
                    <span class="vpro-tree-count">{item.count ?? 0}</span>
                    {#if selectedPlots.length > 0}
                      <button
                        type="button"
                        class="vpro-btn-mini"
                        onclick={e => {
                          e.stopPropagation();
                          void assignPlots(selectedPlots, item.unitCode);
                        }}
                      >
                        Move ({selectedPlots.length}) Here
                      </button>
                    {/if}
                  </div>
                {:else if item.plot}
                  {@const isSelected = selectedPlots.includes(item.plot.plotNumber)}
                  <div
                    role="treeitem"
                    tabindex={focusedTreeItemId === item.id ? 0 : -1}
                    data-tree-item-id={item.id}
                    aria-level={2}
                    aria-selected={isSelected}
                    draggable="true"
                    class="vpro-tree-leaf"
                    class:selected={isSelected}
                    style={`height:${TREE_ROW_HEIGHT}px;`}
                    onfocus={() => (focusedTreeItemId = item.id)}
                    onclick={e => handlePlotClick(item.plot!.plotNumber, e)}
                    onkeydown={e => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        handlePlotClick(item.plot!.plotNumber, e);
                      }
                    }}
                    ondragstart={e => handlePlotDragStart(e, item.plot!.plotNumber)}
                  >
                    <span class="vpro-tree-leaf-tag">P</span>
                    <span class="mono">{item.label}</span>
                    <span class="vpro-tree-sub">{item.subtitle}</span>
                    {#if onOpenPlot}
                      <button
                        type="button"
                        class="vpro-btn-mini"
                        onclick={e => {
                          e.stopPropagation();
                          onOpenPlot(item.plot!.plotNumber);
                        }}
                      >
                        Open
                      </button>
                    {/if}
                  </div>
                {/if}
              {/each}

              {#if suVirtualSlice.bottomSpacerPx > 0}
                <div style={`height:${suVirtualSlice.bottomSpacerPx}px;`} aria-hidden="true"></div>
              {/if}
            </div>
          </div>
        {:else}
          <!-- HIERARCHY TREE MODE -->
          <div class="vpro-cls-subtoolbar">
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => (activeDialog = 'new_hierarchy')}
            >
              New Hierarchy
            </button>
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => {
                expandedHierarchyIds = (hierarchyView?.flatNodes ?? []).map(n => n.id);
              }}
            >
              Expand All
            </button>
            <button
              type="button"
              class="vpro-btn-secondary"
              onclick={() => {
                expandedHierarchyIds = [];
              }}
            >
              Collapse All
            </button>
          </div>

          {#if hierarchyView}
            <p class="vpro-cls-counts">
              <strong>{hierarchyView.diagnostics.totalNodes}</strong> Nodes ·
              <strong>{hierarchyView.diagnostics.breakpointNodes}</strong> Breakpoints (~) ·
              <strong>{hierarchyView.diagnostics.cycleNodeIds.length}</strong> Cycles
            </p>
          {/if}

          <div
            class="vpro-cls-tree-viewport"
            bind:this={treeViewportEl}
            bind:clientHeight={treeViewportHeight}
            onscroll={e => {
              treeScrollTop = e.currentTarget.scrollTop;
            }}
          >
            <div
              role="tree"
              aria-label="Phytosociological Classification Hierarchy Tree"
              tabindex="-1"
              onkeydown={handleHierarchyTreeKeyDown}
            >
              {#if hierVirtualSlice.topSpacerPx > 0}
                <div style={`height:${hierVirtualSlice.topSpacerPx}px;`} aria-hidden="true"></div>
              {/if}

              {#each visibleHierarchyItems as item (item.id)}
                <div
                  role="treeitem"
                  tabindex={focusedTreeItemId === item.id || !focusedTreeItemId ? 0 : -1}
                  data-tree-item-id={item.id}
                  aria-level={item.depth}
                  aria-expanded={item.expandable ? item.expanded : undefined}
                  aria-selected={selectedHierarchyNodeId === item.nodeId}
                  class="vpro-tree-hnode"
                  class:selected={selectedHierarchyNodeId === item.nodeId}
                  class:breakpoint={item.isBreakpoint}
                  style={`height:${TREE_ROW_HEIGHT}px;padding-left:${(item.depth - 1) * 18 + 8}px;`}
                  onfocus={() => (focusedTreeItemId = item.id)}
                  onclick={() => {
                    selectedHierarchyNodeId = item.nodeId;
                    renameDraft = item.name;
                    reparentTargetId = item.parentNodeId === null ? '' : String(item.parentNodeId);
                  }}
                  onkeydown={e => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      selectedHierarchyNodeId = item.nodeId;
                      renameDraft = item.name;
                    }
                  }}
                >
                  {#if item.expandable}
                    <button
                      type="button"
                      class="vpro-tree-caret-btn"
                      aria-label={item.expanded ? 'Collapse branch' : 'Expand branch'}
                      onclick={e => {
                        e.stopPropagation();
                        toggleHierarchyExpand(item.nodeId);
                      }}
                    >
                      {item.expanded ? '[-]' : '[+]'}
                    </button>
                  {:else}
                    <span class="vpro-tree-caret">[ ]</span>
                  {/if}

                  <span class="vpro-rank-badge">{item.rankName}</span>
                  <span class="mono">{item.name}</span>

                  <button
                    type="button"
                    class="vpro-tilde-btn"
                    class:active={item.isBreakpoint}
                    title="Toggle '~' Report Breakpoint"
                    disabled={mutating}
                    onclick={e => {
                      e.stopPropagation();
                      void submitToggleBreak(item.nodeId);
                    }}
                  >
                    ~ Break
                  </button>
                </div>
              {/each}

              {#if hierVirtualSlice.bottomSpacerPx > 0}
                <div
                  style={`height:${hierVirtualSlice.bottomSpacerPx}px;`}
                  aria-hidden="true"
                ></div>
              {/if}
            </div>
          </div>
        {/if}
      </section>

      <!-- RIGHT PANE: Inspector, BECdb v13 Unit Picker & Hierarchy Node Controls -->
      <aside class="vpro-cls-right-pane" aria-label="Classification Inspector and Unit Picker">
        {#if mode === 'su_tree'}
          <section class="vpro-cls-card">
            <div class="vpro-cls-card-head">
              <h3>Plot Environment &amp; Veg Inspector</h3>
              {#if onRequestSUSync}
                <div class="vpro-cls-sync-btns">
                  <button
                    type="button"
                    class="vpro-btn-mini"
                    onclick={() => onRequestSUSync('su_to_env')}
                  >
                    SU -&gt; Env
                  </button>
                  <button
                    type="button"
                    class="vpro-btn-mini"
                    onclick={() => onRequestSUSync('env_to_su')}
                  >
                    Env -&gt; SU
                  </button>
                </div>
              {/if}
            </div>

            {#if inspectedPlot}
              <dl class="vpro-cls-kv">
                <div><dt>Plot</dt><dd class="mono">{inspectedPlot.plotNumber}</dd></div>
                <div>
                  <dt>Field BGC / Series</dt>
                  <dd class="mono">
                    {inspectedPlot.zone}{inspectedPlot.subZone || ''}{inspectedPlot.siteSeries
                      ? `/${inspectedPlot.siteSeries}`
                      : '—'}
                  </dd>
                </div>
                <div><dt>Active SU Unit</dt><dd class="mono">{inspectedPlot.currentSuUnit || '(Unassigned)'}</dd></div>
                <div><dt>UserSiteUnit</dt><dd class="mono">{inspectedPlot.userSiteUnit || '—'}</dd></div>
                <div>
                  <dt>rSMR / rSNR</dt>
                  <dd class="mono">
                    {inspectedPlot.moistureRegime || '—'} / {inspectedPlot.nutrientRegime || '—'}
                  </dd>
                </div>
                <div>
                  <dt>Elev / Slope / Aspect</dt>
                  <dd class="mono">
                    {inspectedPlot.elevation ?? '—'} m · {inspectedPlot.slope || '—'}% ·
                    {inspectedPlot.aspect ?? '—'} deg
                  </dd>
                </div>
              </dl>

              {#if inspectedPlot.topSpecies.length > 0}
                <table class="vpro-cls-spp-table">
                  <thead>
                    <tr>
                      <th>Species</th>
                      <th>A</th>
                      <th>B</th>
                      <th>C</th>
                      <th>D</th>
                      <th>Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each inspectedPlot.topSpecies.slice(0, 8) as sp (sp.species)}
                      <tr>
                        <td title={sp.scientificName}>
                          <strong class="mono">{sp.species}</strong>
                        </td>
                        <td class="mono">{sp.coverA}</td>
                        <td class="mono">{sp.coverB}</td>
                        <td class="mono">{sp.coverC}</td>
                        <td class="mono">{sp.coverD}</td>
                        <td class="mono"><strong>{sp.totalFormatted}</strong></td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              {/if}
            {:else}
              <p class="vpro-cls-muted">
                Click any plot leaf in the SU tree to inspect its field BEC, edatopic regime,
                topography, and dominant species.
              </p>
            {/if}
          </section>

          <section class="vpro-cls-card">
            <h3>BECdb v13 Unit Picker (ZZZZsssvp/##)</h3>
            <div class="vpro-cls-picker-filters">
              <input
                type="text"
                placeholder="Zone (e.g. CWH)"
                bind:value={becZone}
                style="width:5.5rem;"
                onkeydown={e => {
                  if (e.key === 'Enter') void refreshBECPicker();
                }}
              />
              <input
                type="text"
                placeholder="SubZone"
                bind:value={becSubZone}
                style="width:5.5rem;"
                onkeydown={e => {
                  if (e.key === 'Enter') void refreshBECPicker();
                }}
              />
              <input
                type="search"
                placeholder="Search code or name..."
                bind:value={becQuery}
                style="flex:1;"
                onkeydown={e => {
                  if (e.key === 'Enter') void refreshBECPicker();
                }}
              />
              <button type="button" class="vpro-btn-secondary" onclick={refreshBECPicker}>
                Search
              </button>
            </div>

            <div class="vpro-cls-custom-assign">
              <input
                type="text"
                class="mono"
                placeholder="Custom or ZZZZsssvp/## unit..."
                bind:value={customTargetUnit}
              />
              {#if customTargetUnit.trim()}
                <span
                  class="vpro-mask-badge"
                  class:valid={isValidBECUnitMask(customTargetUnit)}
                >
                  {isValidBECUnitMask(customTargetUnit) ? 'BEC Mask OK' : 'Working Unit'}
                </span>
              {/if}
              <button
                type="button"
                class="vpro-btn-primary"
                disabled={selectedPlots.length === 0 || !customTargetUnit.trim() || mutating}
                onclick={() => assignPlots(selectedPlots, customTargetUnit.trim())}
              >
                Assign ({selectedPlots.length})
              </button>
            </div>

            <div class="vpro-cls-bec-list">
              {#each becChoices as choice (choice.siteSeries)}
                <div class="vpro-cls-bec-row">
                  <div>
                    <strong class="mono">{choice.siteSeries}</strong>
                    <span class="vpro-cls-sub">{choice.longName}</span>
                  </div>
                  <button
                    type="button"
                    class="vpro-btn-mini"
                    disabled={selectedPlots.length === 0 || mutating}
                    onclick={() => assignPlots(selectedPlots, choice.siteSeries)}
                  >
                    Assign ({selectedPlots.length})
                  </button>
                </div>
              {/each}
            </div>
          </section>
        {:else}
          <!-- HIERARCHY NODE INSPECTOR & MUTATION CONTROLS -->
          <section class="vpro-cls-card">
            <h3>Add Classification Node</h3>
            <div class="vpro-cls-stack">
              <label>
                <span>Node Name</span>
                <input type="text" bind:value={addNodeName} placeholder="e.g. Tsugion or CWHvm1/01" />
              </label>
              <label>
                <span>Phytosociological Level (1..11)</span>
                <select bind:value={addNodeLevel}>
                  <option value={1}>1 — Superclass</option>
                  <option value={2}>2 — Class</option>
                  <option value={3}>3 — Subclass</option>
                  <option value={4}>4 — Order</option>
                  <option value={5}>5 — SubOrder</option>
                  <option value={6}>6 — Alliance</option>
                  <option value={7}>7 — SubAlliance</option>
                  <option value={8}>8 — Association</option>
                  <option value={9}>9 — SubAssociation</option>
                  <option value={10}>10 — Facies / WorkingUnit</option>
                  <option value={11}>11 — SiteSeries</option>
                </select>
              </label>
              <label class="vpro-cls-check">
                <input type="checkbox" bind:checked={addNodeBreak} />
                <span>Mark as '~' Report Breakpoint</span>
              </label>
              <label class="vpro-cls-check">
                <input type="checkbox" bind:checked={addAsRoot} />
                <span>Insert at Root (otherwise child of selected node)</span>
              </label>
              <button
                type="button"
                class="vpro-btn-primary"
                disabled={!addNodeName.trim() || mutating}
                onclick={submitAddHierarchyNode}
              >
                Add Node
              </button>
            </div>
          </section>

          {#if selectedHierarchyNode}
            <section class="vpro-cls-card">
              <h3>Selected Node #{selectedHierarchyNode.id} ({selectedHierarchyNode.rankName})</h3>
              <div class="vpro-cls-stack">
                <label>
                  <span>Rename Node</span>
                  <input type="text" bind:value={renameDraft} />
                </label>
                <button
                  type="button"
                  class="vpro-btn-secondary"
                  disabled={mutating || !renameDraft.trim()}
                  onclick={submitRenameNode}
                >
                  Apply Rename
                </button>

                <label>
                  <span>Reparent Under (Cycle-Checked)</span>
                  <select bind:value={reparentTargetId}>
                    <option value="">(Root — No Parent)</option>
                    {#each (hierarchyView?.flatNodes ?? []).filter(
                      n =>
                        !wouldIntroduceHierarchyCycle(
                          hierarchyView!.flatNodes,
                          selectedHierarchyNode!.id,
                          n.id
                        )
                    ) as candidate (candidate.id)}
                      <option value={String(candidate.id)}>
                        {candidate.name} ({candidate.rankName})
                      </option>
                    {/each}
                  </select>
                </label>
                <button
                  type="button"
                  class="vpro-btn-secondary"
                  disabled={mutating}
                  onclick={submitReparentNode}
                >
                  Move Branch
                </button>

                <div class="vpro-cls-dialog-row">
                  <button
                    type="button"
                    class="vpro-btn-secondary"
                    disabled={mutating}
                    onclick={() => submitDeleteNode(false)}
                  >
                    Delete &amp; Promote Kids
                  </button>
                  <button
                    type="button"
                    class="vpro-btn-danger"
                    disabled={mutating}
                    onclick={() => submitDeleteNode(true)}
                  >
                    Delete Subtree
                  </button>
                </div>
              </div>
            </section>
          {/if}
        {/if}
      </aside>
    </div>
  {/if}
</section>

<style>
  .vpro-cls-explorer {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    background: #ffffff;
    border: 1px solid #cbd5e1;
    border-top: 4px solid #003366;
    border-radius: 4px;
    padding: 1rem;
    min-width: 0;
  }
  .vpro-cls-header {
    display: flex;
    flex-wrap: wrap;
    justify-content: space-between;
    align-items: flex-end;
    gap: 0.75rem;
    border-bottom: 2px solid #e2e8f0;
    padding-bottom: 0.65rem;
  }
  .vpro-cls-eyebrow {
    margin: 0;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #003366;
  }
  .vpro-cls-title {
    margin: 0.15rem 0 0;
    font-size: 1.2rem;
    font-weight: 600;
    color: #003366;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.45rem;
  }
  .vpro-cls-pill {
    font-size: 0.74rem;
    font-weight: 500;
    background: #f0f4f8;
    border: 1px solid #cbd5e1;
    color: #003366;
    padding: 0.12rem 0.45rem;
    border-radius: 3px;
  }
  .vpro-cls-mode-bar {
    display: flex;
    gap: 0.35rem;
  }
  .vpro-cls-mode-btn {
    min-height: 34px;
    padding: 0.35rem 0.75rem;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    background: #f8fafc;
    color: #334155;
    font-size: 0.82rem;
    font-weight: 600;
  }
  .vpro-cls-mode-btn.active {
    background: #003366;
    color: #ffffff;
    border-color: #003366;
    border-bottom: 3px solid #fcba19;
  }
  .vpro-cls-dual-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.35fr) minmax(300px, 1fr);
    gap: 0.9rem;
    min-width: 0;
  }
  @media (max-width: 960px) {
    .vpro-cls-dual-grid {
      grid-template-columns: 1fr;
    }
  }
  .vpro-cls-left-pane,
  .vpro-cls-right-pane {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    min-width: 0;
  }
  .vpro-cls-subtoolbar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
  }
  .vpro-cls-filter {
    flex: 1;
    min-width: 140px;
    min-height: 32px;
    padding: 0.25rem 0.5rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    font-size: 0.82rem;
  }
  .vpro-cls-counts {
    margin: 0;
    font-size: 0.78rem;
    color: #334155;
  }
  .vpro-cls-tree-viewport {
    height: 480px;
    overflow: auto;
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    background: #ffffff;
  }
  .vpro-tree-folder,
  .vpro-tree-leaf,
  .vpro-tree-hnode {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0 0.6rem;
    border-bottom: 1px solid #f1f5f9;
    font-size: 0.82rem;
    user-select: none;
    cursor: pointer;
    box-sizing: border-box;
  }
  .vpro-tree-folder {
    background: #f8fafc;
    font-weight: 600;
    color: #003366;
  }
  .vpro-tree-folder.unassigned {
    background: #fffbeb;
    color: #78350f;
  }
  .vpro-tree-folder.drop-target {
    outline: 2px dashed #003366;
    background: #e0f2fe;
  }
  .vpro-tree-leaf {
    padding-left: 2.1rem;
    color: #1e293b;
  }
  .vpro-tree-leaf.selected,
  .vpro-tree-hnode.selected {
    background: #dbeafe;
    box-shadow: inset 3px 0 0 #003366;
  }
  .vpro-tree-hnode.breakpoint {
    background: #fef9c3;
  }
  .vpro-tree-caret,
  .vpro-tree-caret-btn {
    font-family: var(--font-mono, monospace);
    font-size: 0.75rem;
    color: #475569;
    background: transparent;
    border: 0;
    padding: 0;
  }
  .vpro-tree-unit-badge {
    display: inline-grid;
    place-items: center;
    width: 18px;
    height: 18px;
    border-radius: 3px;
    background: #003366;
    color: #fcba19;
    font-size: 0.7rem;
    font-weight: 700;
  }
  .vpro-tree-leaf-tag {
    display: inline-grid;
    place-items: center;
    width: 16px;
    height: 16px;
    border-radius: 2px;
    background: #e2e8f0;
    color: #003366;
    font-size: 0.68rem;
    font-weight: 700;
  }
  .vpro-tree-sub,
  .vpro-cls-sub {
    color: #64748b;
    font-size: 0.76rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }
  .vpro-tree-count {
    font-size: 0.72rem;
    background: #e2e8f0;
    color: #0f172a;
    padding: 0.05rem 0.35rem;
    border-radius: 999px;
  }
  .vpro-rank-badge {
    font-size: 0.68rem;
    font-weight: 600;
    background: #e2e8f0;
    color: #003366;
    padding: 0.05rem 0.35rem;
    border-radius: 3px;
  }
  .vpro-tilde-btn {
    margin-left: auto;
    padding: 1px 6px;
    font-size: 0.7rem;
    border: 1px solid #cbd5e1;
    border-radius: 3px;
    background: #ffffff;
    color: #334155;
  }
  .vpro-tilde-btn.active {
    background: #fcba19;
    color: #003366;
    border-color: #d99b00;
    font-weight: 700;
  }
  .vpro-cls-card {
    border: 1px solid #cbd5e1;
    border-radius: 4px;
    padding: 0.75rem;
    background: #f8fafc;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }
  .vpro-cls-card h3 {
    margin: 0;
    font-size: 0.9rem;
    font-weight: 700;
    color: #003366;
  }
  .vpro-cls-card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .vpro-cls-kv {
    margin: 0;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.35rem 0.6rem;
    font-size: 0.78rem;
  }
  .vpro-cls-kv dt {
    margin: 0;
    font-size: 0.68rem;
    color: #64748b;
  }
  .vpro-cls-kv dd {
    margin: 0;
    font-weight: 600;
    color: #0f172a;
  }
  .vpro-cls-spp-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.75rem;
    background: #ffffff;
    min-width: 0;
  }
  .vpro-cls-spp-table th,
  .vpro-cls-spp-table td {
    padding: 3px 6px;
    border: 1px solid #e2e8f0;
  }
  .vpro-cls-picker-filters,
  .vpro-cls-custom-assign,
  .vpro-cls-dialog-row {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    gap: 0.4rem;
  }
  .vpro-cls-picker-filters input,
  .vpro-cls-custom-assign input,
  .vpro-cls-dialog input[type='text'],
  .vpro-cls-dialog select,
  .vpro-cls-stack input[type='text'],
  .vpro-cls-stack select {
    min-height: 30px;
    padding: 0.2rem 0.45rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    font-size: 0.8rem;
    background: #ffffff;
  }
  .vpro-cls-bec-list {
    max-height: 195px;
    overflow: auto;
    border: 1px solid #cbd5e1;
    background: #ffffff;
  }
  .vpro-cls-bec-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    padding: 4px 8px;
    border-bottom: 1px solid #f1f5f9;
    font-size: 0.78rem;
  }
  .vpro-mask-badge {
    font-size: 0.7rem;
    padding: 2px 6px;
    border-radius: 3px;
    background: #e2e8f0;
    color: #334155;
  }
  .vpro-mask-badge.valid {
    background: #dcfce7;
    color: #166534;
    font-weight: 600;
  }
  .vpro-cls-dialog {
    background: #f0f4f8;
    border: 1px solid #94a3b8;
    border-left: 4px solid #fcba19;
    padding: 0.75rem;
    border-radius: 4px;
    display: grid;
    gap: 0.55rem;
  }
  .vpro-cls-dialog h3 {
    margin: 0;
    font-size: 0.92rem;
    color: #003366;
  }
  .vpro-cls-dialog label,
  .vpro-cls-stack label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.76rem;
    font-weight: 600;
    color: #003366;
  }
  .vpro-cls-check {
    flex-direction: row !important;
    align-items: center;
    gap: 0.35rem !important;
  }
  .vpro-cls-field-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .vpro-cls-chip {
    padding: 0.2rem 0.55rem;
    font-size: 0.76rem;
    border: 1px solid #94a3b8;
    border-radius: 999px;
    background: #ffffff;
    color: #334155;
  }
  .vpro-cls-chip.selected {
    background: #003366;
    color: #ffffff;
    border-color: #003366;
  }
  .vpro-cls-flat-scroll {
    overflow-x: auto;
    border: 1px solid #cbd5e1;
  }
  .vpro-cls-flat-table {
    border-collapse: collapse;
    width: max-content;
    min-width: 100%;
    font-size: 0.8rem;
  }
  .vpro-cls-flat-table th {
    background: #003366;
    color: #ffffff;
    border-bottom: 2px solid #fcba19;
    padding: 0.45rem 0.55rem;
    font-size: 0.76rem;
  }
  .vpro-cls-flat-table td {
    padding: 2px;
    border: 1px solid #e2e8f0;
  }
  .vpro-cls-flat-table td.breakpoint-cell {
    background: #fef9c3;
  }
  .vpro-cls-flat-table input {
    width: 132px;
    border: 0;
    padding: 0.3rem 0.4rem;
    background: transparent;
    font-size: 0.8rem;
  }
  .vpro-cls-stack {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .vpro-btn-primary {
    min-height: 32px;
    padding: 0.3rem 0.75rem;
    background: #003366;
    color: #ffffff;
    border: 1px solid #003366;
    border-bottom: 2px solid #fcba19;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 600;
  }
  .vpro-btn-secondary {
    min-height: 32px;
    padding: 0.3rem 0.65rem;
    background: #ffffff;
    color: #003366;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .vpro-btn-danger {
    min-height: 32px;
    padding: 0.3rem 0.65rem;
    background: #fef2f2;
    color: #991b1b;
    border: 1px solid #f87171;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .vpro-btn-mini {
    padding: 2px 6px;
    font-size: 0.7rem;
    border: 1px solid #94a3b8;
    border-radius: 3px;
    background: #ffffff;
    color: #003366;
  }
  .vpro-cls-alert {
    margin: 0;
    padding: 0.5rem 0.75rem;
    background: #fef2f2;
    border-left: 4px solid #b91c1c;
    color: #991b1b;
    font-size: 0.83rem;
  }
  .vpro-cls-status {
    margin: 0;
    padding: 0.45rem 0.75rem;
    background: #f0f4f8;
    border-left: 4px solid #fcba19;
    color: #003366;
    font-size: 0.82rem;
  }
  .vpro-cls-muted,
  .vpro-cls-preview-note {
    margin: 0;
    font-size: 0.78rem;
    color: #475569;
  }
  .vpro-cls-empty {
    padding: 1.5rem;
    text-align: center;
    color: #64748b;
  }
  .mono {
    font-family: var(--font-mono, monospace);
  }
</style>
