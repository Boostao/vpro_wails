<script lang="ts">
  import { onDestroy } from 'svelte';
  import {
    defaultProjectAdminPort,
    validateNewProjectDraft,
    validateProjectNameInput,
    validateSplinterDraft,
    parseLiteralTokenList,
    buildInitialMergeFieldResolutions,
    type ProjectAdminPort,
    type NewProjectResult,
    type SaveProjectAsResult,
    type ExportSplinterResult,
    type CompareProjectsResult,
    type ComplianceValidationReport,
    type QuarantineOrphanResult,
    type OptimizeVegPreview,
    type OptimizeVegResult,
    type ThreeWayMergePreview,
    type ThreeWayMergeResult,
    type MergeResolutionChoice,
  } from './projectAdminWorkbench';

  let {
    contextId,
    activeProject,
    activeSu = 'None',
    operationHeld = false,
    port = defaultProjectAdminPort,
    onBusyChange,
    oncontextswitched,
  }: {
    contextId: string;
    activeProject: string;
    activeSu?: string;
    operationHeld?: boolean;
    port?: ProjectAdminPort;
    onBusyChange?: (busy: boolean) => void;
    oncontextswitched?: (state: unknown) => void;
  } = $props();

  type WorkbenchTab = 'lifecycle' | 'compliance' | 'optimizeveg' | 'merge';
  let activeTab = $state<WorkbenchTab>('lifecycle');
  let busy = $state(false);
  let error = $state<string | null>(null);
  let statusMessage = $state<string | null>(null);
  let generation = 0;

  function setBusy(next: boolean) {
    busy = next;
    onBusyChange?.(next);
  }

  // Lifecycle state
  let newProjName = $state('');
  let newProjId = $state('');
  let newProjTitle = $state('');
  let newProjPreset = $state('LMH25_2023_FS882');
  let newProjDest = $state('');
  let newProjActivate = $state(false);
  let newProjResult = $state<NewProjectResult | null>(null);

  let saveAsName = $state('');
  let saveAsDest = $state('');
  let saveAsActivate = $state(false);
  let saveAsResult = $state<SaveProjectAsResult | null>(null);

  let splinterName = $state('');
  let splinterDest = $state('');
  let splinterUseSu = $state(true);
  let splinterUnitsRaw = $state('');
  let splinterPlotsRaw = $state('');
  let splinterActivate = $state(false);
  let splinterResult = $state<ExportSplinterResult | null>(null);

  // Compliance & Quarantine state
  let complianceUseSu = $state(false);
  let complianceReport = $state<ComplianceValidationReport | null>(null);
  let confirmQuarantine = $state(false);
  let quarantineResult = $state<QuarantineOrphanResult | null>(null);

  // OptimizeVeg state
  let optPreview = $state<OptimizeVegPreview | null>(null);
  let optResult = $state<OptimizeVegResult | null>(null);

  // Compare & 3-Way Merge state
  let cmpTargetPath = $state('');
  let cmpTargetProject = $state('');
  let cmpSourcePath = $state('');
  let cmpSourceProject = $state('');
  let cmpResult = $state<CompareProjectsResult | null>(null);

  let mergeSourcePath = $state('');
  let mergeSourceProject = $state('');
  let mergeBasePath = $state('');
  let mergeBaseProject = $state('');
  let mergeTargetPath = $state('');
  let mergeTargetProject = $state('');
  let mergeDefaultResolution = $state<MergeResolutionChoice>('keep_target');
  let mergePreview = $state<ThreeWayMergePreview | null>(null);
  let mergePlotResolutions = $state<Record<string, MergeResolutionChoice>>({});
  let mergeFieldResolutions = $state<Record<string, MergeResolutionChoice>>({});
  let mergeResult = $state<ThreeWayMergeResult | null>(null);

  const newProjValidationError = $derived(
    newProjName === ''
      ? null
      : validateNewProjectDraft({
          projectName: newProjName,
          projectId: newProjId || undefined,
          projectTitle: newProjTitle || undefined,
          presetId: newProjPreset || undefined,
          destinationPath: newProjDest || undefined,
          activateOnSuccess: newProjActivate,
        })
  );

  const saveAsValidationError = $derived(saveAsName === '' ? null : validateProjectNameInput(saveAsName));

  const splinterValidationError = $derived(
    splinterName === ''
      ? null
      : validateSplinterDraft(
          {
            splinterProject: splinterName,
            destinationPath: splinterDest || undefined,
            useActiveSu: splinterUseSu,
            siteUnits: parseLiteralTokenList(splinterUnitsRaw),
            plotNumbers: parseLiteralTokenList(splinterPlotsRaw),
            activateOnSuccess: splinterActivate,
          },
          activeSu
        )
  );

  onDestroy(() => {
    generation++;
    onBusyChange?.(false);
  });

  async function submitCreateNewProject() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before creating a project.';
      return;
    }
    const req = $state.snapshot({
      projectName: newProjName,
      projectId: newProjId || undefined,
      projectTitle: newProjTitle || undefined,
      presetId: newProjPreset || undefined,
      destinationPath: newProjDest || undefined,
      activateOnSuccess: newProjActivate,
    });
    const vErr = validateNewProjectDraft(req);
    if (vErr) {
      error = vErr;
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.createNewProject(contextId, req);
      if (gen !== generation) return;
      newProjResult = res;
      statusMessage = `Created VP08 project "${res.projectName}" at ${res.destinationPath}.`;
      if (res.activated && res.state && oncontextswitched) {
        oncontextswitched(res.state);
      }
    } catch (cause) {
      if (gen === generation) error = `Create project failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function submitSaveProjectAs() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before saving project copy.';
      return;
    }
    const vErr = validateProjectNameInput(saveAsName);
    if (vErr) {
      error = vErr;
      return;
    }
    const req = $state.snapshot({
      newProjectName: saveAsName,
      destinationPath: saveAsDest || undefined,
      activateOnSuccess: saveAsActivate,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.saveProjectAs(contextId, req);
      if (gen !== generation) return;
      saveAsResult = res;
      statusMessage = `Saved project "${res.sourceProject}" as "${res.newProjectName}" (${res.copiedTables.length} tables).`;
      if (res.activated && res.state && oncontextswitched) {
        oncontextswitched(res.state);
      }
    } catch (cause) {
      if (gen === generation) error = `Save Project As failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function submitExportSplinter() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before exporting splinter project.';
      return;
    }
    const req = $state.snapshot({
      splinterProject: splinterName,
      destinationPath: splinterDest || undefined,
      useActiveSu: splinterUseSu,
      siteUnits: parseLiteralTokenList(splinterUnitsRaw),
      plotNumbers: parseLiteralTokenList(splinterPlotsRaw),
      activateOnSuccess: splinterActivate,
    });
    const vErr = validateSplinterDraft(req, activeSu);
    if (vErr) {
      error = vErr;
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.exportSplinterProject(contextId, req);
      if (gen !== generation) return;
      splinterResult = res;
      statusMessage = `Exported ${res.exportedPlotCount} of ${res.sourcePlotCount} plots to "${res.splinterProject}".`;
      if (res.activated && res.state && oncontextswitched) {
        oncontextswitched(res.state);
      }
    } catch (cause) {
      if (gen === generation) error = `Export Splinter Project failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runCompliance() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before running compliance validation.';
      return;
    }
    const opts = $state.snapshot({ useActiveSu: complianceUseSu });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const report = await port.runComplianceValidation(contextId, opts);
      if (gen !== generation) return;
      complianceReport = report;
      confirmQuarantine = false;
      statusMessage = report.passed
        ? `Compliance validation passed for project "${report.project}".`
        : `Compliance validation identified issues requiring review in "${report.project}".`;
    } catch (cause) {
      if (gen === generation) error = `Compliance validation failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runQuarantine() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before quarantining orphan rows.';
      return;
    }
    if (!confirmQuarantine) {
      error = 'Confirm orphan quarantine before moving orphan child rows.';
      return;
    }
    const req = $state.snapshot({});
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.quarantineOrphanRecords(contextId, req);
      if (gen !== generation) return;
      quarantineResult = res;
      confirmQuarantine = false;
      statusMessage = `Quarantined ${res.totalQuarantined} orphan child row(s) into __VPRO_Quarantine_* tables (batch ${res.batchId}).`;
      const refreshed = await port.runComplianceValidation(contextId, $state.snapshot({ useActiveSu: complianceUseSu }));
      if (gen === generation) complianceReport = refreshed;
    } catch (cause) {
      if (gen === generation) error = `Orphan quarantine failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPreviewOptimizeVeg() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before previewing OptimizeVeg.';
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const preview = await port.previewOptimizeVeg(contextId, $state.snapshot({}));
      if (gen !== generation) return;
      optPreview = preview;
      optResult = null;
      statusMessage = `OptimizeVeg preview: ${preview.duplicateSpeciesCount} multi-row species group(s) (${preview.beforeRowCount} -> ${preview.afterRowCount} rows).`;
    } catch (cause) {
      if (gen === generation) error = `Preview OptimizeVeg failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runApplyOptimizeVeg() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before applying OptimizeVeg.';
      return;
    }
    if (!optPreview) {
      error = 'Run Preview OptimizeVeg before applying consolidation.';
      return;
    }
    const req = $state.snapshot({ expectedDigest: optPreview.previewDigest });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.applyOptimizeVeg(contextId, req);
      if (gen !== generation) return;
      optResult = res;
      optPreview = null;
      statusMessage = `OptimizeVeg consolidated ${res.consolidatedGroups} group(s), removed ${res.deletedRows} duplicate row(s), and wrote ${res.auditRowsWritten} audit row(s).`;
    } catch (cause) {
      if (gen === generation) error = `Apply OptimizeVeg failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runCompareProjects() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before comparing projects.';
      return;
    }
    if (!cmpTargetPath.trim()) {
      error = 'Target .db file path is required to compare projects.';
      return;
    }
    const req = $state.snapshot({
      sourcePath: cmpSourcePath.trim() || undefined,
      sourceProject: cmpSourceProject.trim() || undefined,
      targetPath: cmpTargetPath.trim(),
      targetProject: cmpTargetProject.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.compareProjects(contextId, req);
      if (gen !== generation) return;
      cmpResult = res;
      statusMessage = `Compared ${res.sourceProject} vs ${res.targetProject}: ${res.addedCount} Added, ${res.removedCount} Removed, ${res.modifiedCount} Modified, ${res.identicalCount} Identical.`;
    } catch (cause) {
      if (gen === generation) error = `Compare Projects failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPreviewMerge() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before previewing merge.';
      return;
    }
    if (!mergeSourcePath.trim()) {
      error = 'Staging source .db path is required for 3-way merge preview.';
      return;
    }
    const req = $state.snapshot({
      sourcePath: mergeSourcePath.trim(),
      sourceProject: mergeSourceProject.trim() || undefined,
      targetPath: mergeTargetPath.trim() || undefined,
      targetProject: mergeTargetProject.trim() || undefined,
      basePath: mergeBasePath.trim() || undefined,
      baseProject: mergeBaseProject.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const preview = await port.previewThreeWayMerge(contextId, req);
      if (gen !== generation) return;
      mergePreview = preview;
      mergeResult = null;
      mergePlotResolutions = {};
      mergeFieldResolutions = buildInitialMergeFieldResolutions(preview);
      statusMessage = `Merge preview ready: ${preview.addedCount} Added, ${preview.fastForwardCount} Fast-Forward, ${preview.conflictCount} Conflict plot(s).`;
    } catch (cause) {
      if (gen === generation) error = `Preview 3-Way Merge failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runExecuteMerge() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before executing merge.';
      return;
    }
    if (!mergePreview) {
      error = 'Run Preview 3-Way Merge before executing merge.';
      return;
    }
    const req = $state.snapshot({
      sourcePath: mergePreview.sourcePath,
      sourceProject: mergePreview.sourceProject,
      targetPath: mergePreview.targetPath,
      targetProject: mergePreview.targetProject,
      basePath: mergePreview.basePath || undefined,
      baseProject: mergePreview.baseProject || undefined,
      expectedPreviewDigest: mergePreview.previewDigest,
      defaultConflictResolution: mergeDefaultResolution,
      plotResolutions: mergePlotResolutions,
      fieldResolutions: mergeFieldResolutions,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.executeThreeWayMerge(contextId, req);
      if (gen !== generation) return;
      mergeResult = res;
      mergePreview = null;
      statusMessage = `3-Way Merge committed: ${res.addedPlots} plot(s) added, ${res.updatedPlots} plot(s) updated, ${res.auditRowsWritten} audit row(s) recorded.`;
    } catch (cause) {
      if (gen === generation) error = `Execute 3-Way Merge failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  function setFieldResolution(conflictId: string, choice: MergeResolutionChoice) {
    mergeFieldResolutions = { ...mergeFieldResolutions, [conflictId]: choice };
  }
</script>

<section class="min-w-0 space-y-4 rounded border border-slate-300 bg-white shadow-sm" aria-label="Project Lifecycle and Pre-Flight Workbench">
  <header class="border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-5 py-4 text-white">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div>
        <p class="text-xs font-semibold uppercase tracking-wider text-[#FCBA19]">VPro Project Governance</p>
        <h2 class="m-0 text-lg font-semibold text-white">Project Lifecycle, Compliance Pre-Flight &amp; 3-Way Merge</h2>
      </div>
      <div class="text-xs text-slate-200">
        Active Project: <span class="font-mono font-semibold text-white">{activeProject}</span>
        {#if activeSu !== 'None'}
          | Active SU: <span class="font-mono font-semibold text-[#FCBA19]">{activeSu}</span>
        {/if}
      </div>
    </div>
  </header>

  <nav class="flex flex-wrap gap-1 border-b border-slate-200 px-5 pt-2" aria-label="Project workbench sections">
    {#each [
      ['lifecycle', 'Project Lifecycle (New / Save As / Splinter)'],
      ['compliance', 'Compliance Validator & Quarantine'],
      ['optimizeveg', 'Safe OptimizeVeg'],
      ['merge', 'Compare & 3-Way Merge'],
    ] as [id, label]}
      <button
        type="button"
        disabled={busy}
        class={`rounded-t px-4 py-2 text-sm font-medium transition-colors ${
          activeTab === id
            ? 'border-b-[3px] border-b-[#FCBA19] bg-[#003366] text-white'
            : 'bg-slate-100 text-slate-800 hover:bg-slate-200'
        }`}
        onclick={() => {
          activeTab = id as WorkbenchTab;
          error = null;
        }}
      >
        {label}
      </button>
    {/each}
  </nav>

  <div class="space-y-4 px-5 pb-5">
    {#if error}
      <p role="alert" class="rounded border-l-4 border-red-700 bg-red-50 p-3 text-sm text-red-900">{error}</p>
    {/if}
    {#if statusMessage}
      <p role="status" class="rounded border-l-4 border-[#003366] bg-slate-50 p-3 text-sm text-slate-900">{statusMessage}</p>
    {/if}

    {#if activeTab === 'lifecycle'}
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <!-- Create New Project -->
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Create New VP08 Project</legend>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="new-project-name">Project Name (A-Z, 0-9, _, max 31)</label>
            <input id="new-project-name" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={newProjName} disabled={busy} aria-invalid={!!newProjValidationError} />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="new-project-id">Metadata ProjectID (optional, max 20)</label>
            <input id="new-project-id" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={newProjId} disabled={busy} />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="new-project-title">Project Title (optional)</label>
            <input id="new-project-title" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={newProjTitle} disabled={busy} />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="new-project-preset">Standards Preset</label>
            <select id="new-project-preset" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={newProjPreset} disabled={busy}>
              <option value="LMH25_2023_FS882">LMH25 (2023) Standard FS882</option>
              <option value="LMH25_FS1333_SIVI">LMH25 FS1333 SIVI Ground</option>
              <option value="DEIF_1980">DEIF 1980 Historical</option>
              <option value="CHARS_Arctic">CHARS Arctic</option>
              <option value="">None (Blank Metadata)</option>
            </select>
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="new-project-dest">Destination .db Path (optional)</label>
            <input id="new-project-dest" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={newProjDest} disabled={busy} placeholder="Defaults to projects/<Name>.db" />
          </div>
          <label class="flex items-center gap-2 text-xs text-slate-800">
            <input type="checkbox" bind:checked={newProjActivate} disabled={busy} />
            Switch active context to new project immediately
          </label>
          {#if newProjValidationError}
            <p role="alert" class="text-xs text-red-700">{newProjValidationError}</p>
          {/if}
          <button
            type="button"
            disabled={busy || !newProjName || !!newProjValidationError}
            class="w-full rounded bg-[#003366] px-3 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={submitCreateNewProject}
          >
            Create VP08 Project
          </button>
          {#if newProjResult}
            <p class="break-all text-xs text-slate-600">SHA-256: <span class="font-mono">{newProjResult.destinationSha256.slice(0, 16)}</span></p>
          {/if}
        </fieldset>

        <!-- Save Project As -->
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Save Active Project As</legend>
          <p class="text-xs text-slate-600">Lossless copy of <span class="font-mono font-semibold">{activeProject}</span> renaming prefix across core and companion (_SU, _Hierarchy, _Lump, _Profile) tables.</p>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="saveas-name">New Project Prefix</label>
            <input id="saveas-name" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={saveAsName} disabled={busy} aria-invalid={!!saveAsValidationError} />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="saveas-dest">Destination .db Path (optional)</label>
            <input id="saveas-dest" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={saveAsDest} disabled={busy} placeholder="Defaults to projects/<NewPrefix>.db" />
          </div>
          <label class="flex items-center gap-2 text-xs text-slate-800">
            <input type="checkbox" bind:checked={saveAsActivate} disabled={busy} />
            Switch active context to saved copy
          </label>
          {#if saveAsValidationError}
            <p role="alert" class="text-xs text-red-700">{saveAsValidationError}</p>
          {/if}
          <button
            type="button"
            disabled={busy || !saveAsName || !!saveAsValidationError}
            class="w-full rounded bg-[#003366] px-3 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={submitSaveProjectAs}
          >
            Save Project As
          </button>
          {#if saveAsResult && saveAsResult.companionTables.length > 0}
            <p class="text-xs text-slate-600">Preserved companions: <span class="font-mono">{saveAsResult.companionTables.join(', ')}</span></p>
          {/if}
        </fieldset>

        <!-- Export Splinter Project -->
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Export Splinter Project</legend>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="splinter-name">Splinter Project Name</label>
            <input id="splinter-name" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={splinterName} disabled={busy} aria-invalid={!!splinterValidationError} />
          </div>
          <label class="flex items-center gap-2 text-xs text-slate-800">
            <input type="checkbox" bind:checked={splinterUseSu} disabled={busy} />
            Filter by active Site Unit table ({activeSu})
          </label>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="splinter-units">Site Unit Filter (comma-separated, optional)</label>
            <input id="splinter-units" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={splinterUnitsRaw} disabled={busy} placeholder="e.g. CWHvm1/01, CWHvm1/05" />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="splinter-plots">Explicit Plot Subset (comma-separated, optional)</label>
            <input id="splinter-plots" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={splinterPlotsRaw} disabled={busy} placeholder="e.g. 00337, 108050" />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="splinter-dest">Destination .db Path (optional)</label>
            <input id="splinter-dest" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={splinterDest} disabled={busy} />
          </div>
          <label class="flex items-center gap-2 text-xs text-slate-800">
            <input type="checkbox" bind:checked={splinterActivate} disabled={busy} />
            Switch active context to splinter project
          </label>
          {#if splinterValidationError}
            <p role="alert" class="text-xs text-red-700">{splinterValidationError}</p>
          {/if}
          <button
            type="button"
            disabled={busy || !splinterName || !!splinterValidationError}
            class="w-full rounded bg-[#003366] px-3 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={submitExportSplinter}
          >
            Export Splinter Project
          </button>
          {#if splinterResult}
            <p class="text-xs text-slate-600">Exported {splinterResult.exportedPlotCount} / {splinterResult.sourcePlotCount} plots.</p>
          {/if}
        </fieldset>
      </div>
    {:else if activeTab === 'compliance'}
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3 rounded border border-slate-200 bg-slate-50 p-3">
          <label class="flex items-center gap-2 text-sm text-slate-800">
            <input type="checkbox" bind:checked={complianceUseSu} disabled={busy || activeSu === 'None'} />
            Restrict code &amp; bounds checks to active SU ({activeSu})
          </label>
          <button
            type="button"
            disabled={busy}
            class="rounded bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={runCompliance}
          >
            Run Batch Compliance Check
          </button>
        </div>

        {#if complianceReport}
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">USysTableOfLists Errors</div>
              <div class="text-xl font-bold text-[#003366]">{complianceReport.summary.envCodeErrors}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">Numeric / UTF-16 / Cover Bounds</div>
              <div class="text-xl font-bold text-[#003366]">{complianceReport.summary.boundsErrors}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">1:1 Env/Admin &amp; 1:N Orphans</div>
              <div class="text-xl font-bold text-[#003366]">{complianceReport.summary.envAdminMismatches + complianceReport.summary.orphanChildRows}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">Veg Splits / Synonyms / Unlisted</div>
              <div class="text-xl font-bold text-[#003366]">{complianceReport.summary.duplicateSpeciesSplits + complianceReport.summary.synonymSpecies + complianceReport.summary.unlistedSpecies}</div>
            </div>
          </div>

          {#if complianceReport.summary.orphanChildRows > 0}
            <div class="space-y-2 rounded border border-amber-400 bg-amber-50 p-4">
              <h3 class="m-0 text-sm font-semibold text-slate-900">Orphan Child Records Detected ({complianceReport.summary.orphanChildRows} rows)</h3>
              <p class="text-xs text-slate-700">Quarantining moves orphan child rows (_Veg, _Humus, _Mineral, _Other missing from _Env) into __VPRO_Quarantine_&lt;Table&gt;, reserves their positive child IDs permanently in __VPRO_ChildIdentity, and writes an audit receipt so zero data is silently lost.</p>
              <label class="flex items-center gap-2 text-xs font-medium text-slate-900">
                <input type="checkbox" bind:checked={confirmQuarantine} disabled={busy} />
                Confirm transactional move to __VPRO_Quarantine_* tables
              </label>
              <button
                type="button"
                disabled={busy || !confirmQuarantine}
                class="rounded bg-[#003366] px-3 py-2 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
                onclick={runQuarantine}
              >
                Quarantine Orphan Child Rows
              </button>
            </div>
          {/if}

          {#if quarantineResult}
            <p class="text-xs text-slate-700">Last quarantine batch <span class="font-mono">{quarantineResult.batchId}</span> moved {quarantineResult.totalQuarantined} row(s).</p>
          {/if}

          {#if complianceReport.envCodeFindings.length > 0}
            <div class="overflow-x-auto rounded border border-slate-200">
              <table class="w-full text-left text-xs">
                <thead class="bg-slate-100 text-slate-700">
                  <tr><th class="p-2">Plot</th><th class="p-2">Field</th><th class="p-2">ListName</th><th class="p-2">Invalid Value</th></tr>
                </thead>
                <tbody>
                  {#each complianceReport.envCodeFindings as item}
                    <tr class="border-t border-slate-100">
                      <td class="p-2 font-mono">{item.plotNumber}</td>
                      <td class="p-2">{item.field}</td>
                      <td class="p-2">{item.listName}</td>
                      <td class="p-2 font-mono text-red-700">{item.value}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}

          {#if complianceReport.boundsFindings.length > 0}
            <div class="overflow-x-auto rounded border border-slate-200">
              <table class="w-full text-left text-xs">
                <thead class="bg-slate-100 text-slate-700">
                  <tr><th class="p-2">Table</th><th class="p-2">Plot</th><th class="p-2">Field</th><th class="p-2">Value</th><th class="p-2">Rule</th></tr>
                </thead>
                <tbody>
                  {#each complianceReport.boundsFindings as b}
                    <tr class="border-t border-slate-100">
                      <td class="p-2">{b.table}</td>
                      <td class="p-2 font-mono">{b.plotNumber}</td>
                      <td class="p-2">{b.field}</td>
                      <td class="p-2 font-mono text-red-700">{b.value}</td>
                      <td class="p-2">{b.rule}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        {/if}
      </div>
    {:else if activeTab === 'optimizeveg'}
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3 rounded border border-slate-200 bg-slate-50 p-3">
          <div class="text-xs text-slate-700">
            Consolidates split multi-row <span class="font-mono">(PlotNumber, Species)</span> records atomically using <span class="font-semibold">MAX(Height*)</span> and IEEE-754 SINGLE &lt;100 cover capping (99%), retiring deleted child IDs in <span class="font-mono">__VPRO_ChildIdentity</span>.
          </div>
          <div class="flex gap-2">
            <button
              type="button"
              disabled={busy}
              class="rounded bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runPreviewOptimizeVeg}
            >
              Preview OptimizeVeg
            </button>
            {#if optPreview && optPreview.duplicateSpeciesCount > 0}
              <button
                type="button"
                disabled={busy}
                class="rounded border-2 border-[#FCBA19] bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
                onclick={runApplyOptimizeVeg}
              >
                Apply Consolidation ({optPreview.rowsToConsolidate} duplicate rows)
              </button>
            {/if}
          </div>
        </div>

        {#if optPreview}
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">Before Rows</div>
              <div class="text-lg font-bold text-[#003366]">{optPreview.beforeRowCount}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">After Rows</div>
              <div class="text-lg font-bold text-[#003366]">{optPreview.afterRowCount}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">Overlapping Layer Groups</div>
              <div class="text-lg font-bold text-[#003366]">{optPreview.overlappingGroupCount}</div>
            </div>
            <div class="rounded border border-slate-300 p-3">
              <div class="text-xs text-slate-600">Covers Capped at 99%</div>
              <div class="text-lg font-bold text-[#003366]">{optPreview.cappedCoverCount}</div>
            </div>
          </div>

          {#if optPreview.groups.length > 0}
            <div class="overflow-x-auto rounded border border-slate-200">
              <table class="w-full text-left text-xs">
                <thead class="bg-slate-100 text-slate-700">
                  <tr>
                    <th class="p-2">Plot</th>
                    <th class="p-2">Species</th>
                    <th class="p-2">Rows</th>
                    <th class="p-2">Survivor ID</th>
                    <th class="p-2">Retired IDs</th>
                    <th class="p-2">Merged Covers</th>
                    <th class="p-2">MAX Heights</th>
                  </tr>
                </thead>
                <tbody>
                  {#each optPreview.groups as grp}
                    <tr class="border-t border-slate-100">
                      <td class="p-2 font-mono">{grp.plotNumber}</td>
                      <td class="p-2 font-mono font-semibold">{grp.species}</td>
                      <td class="p-2">{grp.sourceRowCount}</td>
                      <td class="p-2 font-mono">{grp.survivorId}</td>
                      <td class="p-2 font-mono">{grp.retiredIds.join(', ')}</td>
                      <td class="p-2 font-mono">{Object.entries(grp.mergedCovers).map(([k, v]) => `${k}=${v}`).join(', ')}</td>
                      <td class="p-2 font-mono">{Object.entries(grp.mergedHeights).map(([k, v]) => `${k}=${v}`).join(', ')}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        {/if}

        {#if optResult}
          <p class="text-xs text-slate-700">Committed OptimizeVeg: {optResult.beforeRowCount} -&gt; {optResult.afterRowCount} rows; retired IDs reserved: {optResult.reservedDeletedIds.join(', ') || 'none'}.</p>
        {/if}
      </div>
    {:else if activeTab === 'merge'}
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <!-- Compare Projects -->
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Compare Two Projects</legend>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cmp-target-path">Target .db Path (required)</label>
            <input id="cmp-target-path" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={cmpTargetPath} disabled={busy} />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="cmp-target-proj">Target Project (optional)</label>
              <input id="cmp-target-proj" type="text" class="w-full rounded border border-slate-300 p-2 text-sm" bind:value={cmpTargetProject} disabled={busy} />
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="cmp-source-path">Source .db Path (blank = active)</label>
              <input id="cmp-source-path" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={cmpSourcePath} disabled={busy} />
            </div>
          </div>
          <button
            type="button"
            disabled={busy || !cmpTargetPath.trim()}
            class="rounded bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={runCompareProjects}
          >
            Compare Projects
          </button>

          {#if cmpResult}
            <p class="text-xs text-slate-700">
              Total Plots: {cmpResult.totalPlots} | Added: {cmpResult.addedCount} | Removed: {cmpResult.removedCount} | Modified: {cmpResult.modifiedCount} | Identical: {cmpResult.identicalCount}
            </p>
            {#if cmpResult.fieldDiffs.length > 0}
              <div class="max-h-60 overflow-auto rounded border border-slate-200">
                <table class="w-full text-left text-xs">
                  <thead class="bg-slate-100 text-slate-700">
                    <tr><th class="p-2">Plot</th><th class="p-2">Table</th><th class="p-2">Key</th><th class="p-2">Source</th><th class="p-2">Target</th></tr>
                  </thead>
                  <tbody>
                    {#each cmpResult.fieldDiffs as d}
                      <tr class="border-t border-slate-100">
                        <td class="p-2 font-mono">{d.plotNumber}</td>
                        <td class="p-2">{d.table}</td>
                        <td class="p-2 font-mono">{d.key}</td>
                        <td class="p-2 font-mono">{d.sourceVal ?? 'NULL'}</td>
                        <td class="p-2 font-mono">{d.targetVal ?? 'NULL'}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          {/if}
        </fieldset>

        <!-- 3-Way Merge -->
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Controlled 3-Way Project Merge</legend>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="merge-source-path">Staging Source .db Path (required)</label>
            <input id="merge-source-path" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={mergeSourcePath} disabled={busy} />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="merge-base-path">Base Ancestor .db Path (optional)</label>
              <input id="merge-base-path" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={mergeBasePath} disabled={busy} />
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="merge-target-path">Target .db Path (blank = active)</label>
              <input id="merge-target-path" type="text" class="w-full rounded border border-slate-300 p-2 text-sm font-mono" bind:value={mergeTargetPath} disabled={busy} />
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              disabled={busy || !mergeSourcePath.trim()}
              class="rounded bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runPreviewMerge}
            >
              Preview 3-Way Merge
            </button>
            {#if mergePreview}
              <select class="rounded border border-slate-300 p-2 text-xs" bind:value={mergeDefaultResolution} disabled={busy} aria-label="Default conflict resolution">
                <option value="keep_target">Default: Keep Target</option>
                <option value="accept_source">Default: Accept Source</option>
              </select>
              <button
                type="button"
                disabled={busy}
                class="rounded border-2 border-[#FCBA19] bg-[#003366] px-4 py-2 text-sm font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
                onclick={runExecuteMerge}
              >
                Execute Atomic Merge
              </button>
            {/if}
          </div>

          {#if mergePreview}
            <p class="text-xs text-slate-700">
              Added: {mergePreview.addedCount} | Fast-Forward: {mergePreview.fastForwardCount} | Conflicts: {mergePreview.conflictCount} | Identical: {mergePreview.identicalCount}
            </p>
            {#if mergePreview.conflicts.length > 0}
              <div class="max-h-60 overflow-auto rounded border border-slate-200">
                <table class="w-full text-left text-xs">
                  <thead class="bg-slate-100 text-slate-700">
                    <tr>
                      <th class="p-2">Plot / Field</th>
                      <th class="p-2">Source</th>
                      <th class="p-2">Target</th>
                      <th class="p-2">Resolution</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each mergePreview.conflicts as c}
                      <tr class="border-t border-slate-100">
                        <td class="p-2 font-mono">{c.plotNumber} {c.table}.{c.key}</td>
                        <td class="p-2 font-mono">{c.sourceVal ?? 'NULL'}</td>
                        <td class="p-2 font-mono">{c.targetVal ?? 'NULL'}</td>
                        <td class="p-2">
                          <select
                            class="rounded border border-slate-300 p-1 text-xs"
                            value={mergeFieldResolutions[c.conflictId] ?? mergeDefaultResolution}
                            onchange={e => setFieldResolution(c.conflictId, e.currentTarget.value as MergeResolutionChoice)}
                          >
                            <option value="keep_target">Keep Target</option>
                            <option value="accept_source">Accept Source</option>
                          </select>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          {/if}

          {#if mergeResult}
            <p class="text-xs text-slate-700">
              Merged {mergeResult.sourceProject} -&gt; {mergeResult.targetProject}: +{mergeResult.addedPlots} added, {mergeResult.updatedPlots} updated, {mergeResult.auditRowsWritten} audit entries.
            </p>
          {/if}
        </fieldset>
      </div>
    {/if}
  </div>
</section>
