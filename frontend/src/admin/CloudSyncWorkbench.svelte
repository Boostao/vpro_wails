<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    defaultCloudSyncPort,
    validateProjectNameInput,
    buildSubsetFilterDraft,
    canUploadStagingSubmission,
    isCustodianProtectedConflict,
    buildInitialCloudMergeFieldResolutions,
    validateSnapshotTagInput,
    CUSTODIAN_PROTECTED_FIELDS,
    type CloudSyncPort,
    type CloudRoleMode,
    type CloudMergeResolution,
    type CloudConfigStatus,
    type CloudConnectionReport,
    type SchemaBootstrapResult,
    type PublishStandardsResult,
    type SeedBaselineResult,
    type PullStandardsResult,
    type SubsetPreview,
    type PullSubsetResult,
    type StagingPreflightPreview,
    type SubmitStagingResult,
    type StagingSubmissionSummary,
    type StagingSubmissionDetail,
    type StagingMergePreview,
    type StagingMergeResult,
    type SnapshotRecord,
    type DownloadLogEntry,
    type CloudMergeConflictItem,
  } from './cloudSyncWorkbench';

  let {
    contextId,
    activeProject,
    activeSu = 'None',
    operationHeld = false,
    port = defaultCloudSyncPort,
    onBusyChange,
    oncontextswitched,
  }: {
    contextId: string;
    activeProject: string;
    activeSu?: string;
    operationHeld?: boolean;
    port?: CloudSyncPort;
    onBusyChange?: (busy: boolean) => void;
    oncontextswitched?: (state: unknown) => void;
  } = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);
  let statusMessage = $state<string | null>(null);
  let generation = 0;

  function setBusy(next: boolean) {
    busy = next;
    onBusyChange?.(next);
  }

  // 1. Connection & Session Role state (offline-first: zero live network probes on mount)
  let configStatus = $state.raw<CloudConfigStatus | null>(null);
  let connectionReport = $state.raw<CloudConnectionReport | null>(null);
  let selectedRoleMode = $state<CloudRoleMode>('contributor');

  let overrideHost = $state('');
  let overridePort = $state('5432');
  let overrideDatabase = $state('');
  let overrideSslMode = $state('require');
  let overrideUser = $state('');
  let overridePassword = $state('');
  let overrideAdminUser = $state('');
  let overrideAdminPassword = $state('');

  let pullStandardsResult = $state.raw<PullStandardsResult | null>(null);

  // 2. Contributor: Regional Subset Checkout state
  let subsetZonesRaw = $state('');
  let subsetSubzonesRaw = $state('');
  let subsetProjectIdsRaw = $state('');
  let subsetPlotNumbersRaw = $state('');
  let subsetBboxRaw = $state('');
  let subsetLimitRaw = $state('');
  let subsetAllowFull = $state(false);
  let subsetPreview = $state.raw<SubsetPreview | null>(null);

  let subsetTargetProject = $state('');
  let subsetDestinationPath = $state('');
  let subsetActivateOnSuccess = $state(true);
  let pullSubsetResult = $state.raw<PullSubsetResult | null>(null);

  // 2b. Contributor: Staging Pre-Flight & Upload state
  let stagingUseActiveSu = $state(false);
  let stagingPreflight = $state.raw<StagingPreflightPreview | null>(null);
  let stagingSubmittedBy = $state('');
  let stagingSubmitterNotes = $state('');
  let stagingBaseSnapshotTag = $state('');
  let stagingCustodianOverride = $state(false);
  let submitStagingResult = $state.raw<SubmitStagingResult | null>(null);

  // Shared / Custodian Staging Queue state
  let submissionsStatusFilter = $state('');
  let submissionsSubmittedByFilter = $state('');
  let submissionsLimit = $state(50);
  let submissions = $state.raw<StagingSubmissionSummary[]>([]);

  // 3. Custodian Admin Maintenance, Review, 3-Way Merge & Snapshots state
  let bootstrapResult = $state.raw<SchemaBootstrapResult | null>(null);

  let pubReleaseId = $state('');
  let pubBecVersion = $state('13');
  let pubTaxonomyVersion = $state('19');
  let pubVlistsPath = $state('');
  let pubBecDbPath = $state('');
  let pubVmetaDataPath = $state('');
  let pubNotes = $state('');
  let publishStandardsResult = $state.raw<PublishStandardsResult | null>(null);

  let seedSourceDbPath = $state('');
  let seedSourceProject = $state('');
  let seedSnapshotTag = $state('');
  let seedReplaceExisting = $state(false);
  let seedSkipAudit = $state(false);
  let seedNotes = $state('');
  let seedBaselineResult = $state.raw<SeedBaselineResult | null>(null);

  let selectedSubmissionDetail = $state.raw<StagingSubmissionDetail | null>(null);
  let mergeBaseDbPath = $state('');
  let mergeBaseProject = $state('');
  let mergePreview = $state.raw<StagingMergePreview | null>(null);
  let mergeDefaultResolution = $state<CloudMergeResolution>('keep_target');
  let mergePlotResolutions = $state<Record<string, CloudMergeResolution>>({});
  let mergeFieldResolutions = $state<Record<string, CloudMergeResolution>>({});
  let mergeReviewerNotes = $state('');
  let mergeActor = $state('');
  let mergeExecuteResult = $state.raw<StagingMergeResult | null>(null);

  let snapshotTag = $state('');
  let snapshotNotes = $state('');
  let snapshotExportFormat = $state('');
  let snapshotDestinationPath = $state('');
  let snapshotZoneFilter = $state('');
  let latestSnapshotCreated = $state.raw<SnapshotRecord | null>(null);
  let snapshots = $state.raw<SnapshotRecord[]>([]);
  let downloadLogLimit = $state(50);
  let downloadLog = $state.raw<DownloadLogEntry[]>([]);

  // Reset pre-flight compliance & override state whenever active project or context switches
  let observedContextKey = '';
  $effect(() => {
    const key = `${contextId}::${activeProject}`;
    if (observedContextKey !== '' && observedContextKey !== key) {
      stagingPreflight = null;
      stagingCustodianOverride = false;
      submitStagingResult = null;
    }
    observedContextKey = key;
  });

  const activeRoleMode = $derived<CloudRoleMode>(
    connectionReport?.activeRoleMode ?? configStatus?.activeRoleMode ?? selectedRoleMode
  );

  const subsetTargetValidationError = $derived(
    subsetTargetProject === '' ? null : validateProjectNameInput(subsetTargetProject)
  );

  const snapshotTagValidationError = $derived(
    snapshotTag === '' ? null : validateSnapshotTagInput(snapshotTag)
  );

  const canUploadStaging = $derived(
    canUploadStagingSubmission(stagingPreflight, activeRoleMode, stagingCustodianOverride)
  );

  const combinedMergeItems = $derived.by<CloudMergeConflictItem[]>(() => {
    if (!mergePreview) return [];
    const seen = new Set<string>();
    const list: CloudMergeConflictItem[] = [];
    for (const item of mergePreview.custodianProtectedDiffs ?? []) {
      if (!seen.has(item.conflictId)) {
        seen.add(item.conflictId);
        list.push(item);
      }
    }
    for (const item of mergePreview.conflicts ?? []) {
      if (!seen.has(item.conflictId)) {
        seen.add(item.conflictId);
        list.push(item);
      }
    }
    return list;
  });

  onMount(() => {
    const mountGen = generation;
    void port
      .getCloudSyncConfigStatus()
      .then(status => {
        if (mountGen !== generation || !status) return;
        configStatus = status;
        selectedRoleMode = status.activeRoleMode;
        if (status.port > 0) overridePort = String(status.port);
        if (status.database) overrideDatabase = status.database;
        if (status.sslMode) overrideSslMode = status.sslMode;
      })
      .catch(cause => {
        if (mountGen === generation) {
          error = `Could not read local cloud sync configuration status: ${String(cause)}`;
        }
      });
  });

  onDestroy(() => {
    generation++;
    onBusyChange?.(false);
  });

  async function applySessionConfig(clearOverrides = false) {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before changing cloud session settings.';
      return;
    }
    let parsedPort: number | undefined;
    if (!clearOverrides && overridePort.trim() !== '') {
      const p = Number(overridePort.trim());
      if (!Number.isInteger(p) || p < 1 || p > 65535) {
        error = 'PostgreSQL port must be an integer between 1 and 65535.';
        return;
      }
      parsedPort = p;
    }

    const req = $state.snapshot(
      clearOverrides
        ? {
            roleMode: selectedRoleMode,
            clearOverrides: true,
          }
        : {
            roleMode: selectedRoleMode,
            host: overrideHost.trim() || undefined,
            port: parsedPort,
            database: overrideDatabase.trim() || undefined,
            sslMode: overrideSslMode.trim() || undefined,
            user: overrideUser.trim() || undefined,
            password: overridePassword !== '' ? overridePassword : undefined,
            adminUser: overrideAdminUser.trim() || undefined,
            adminPassword: overrideAdminPassword !== '' ? overrideAdminPassword : undefined,
          }
    );

    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const status = await port.configureCloudSyncSession(req);
      if (gen !== generation) return;
      overrideHost = '';
      overridePassword = '';
      overrideAdminPassword = '';
      if (clearOverrides) {
        overrideUser = '';
        overrideAdminUser = '';
      }
      configStatus = status;
      selectedRoleMode = status.activeRoleMode;
      if (connectionReport && connectionReport.activeRoleMode !== status.activeRoleMode) {
        connectionReport = null;
      }
      statusMessage = clearOverrides
        ? `Cleared in-memory session credential overrides (active role: ${status.activeRoleMode}).`
        : `Updated in-memory cloud session (${status.activeRoleMode} role; password inputs cleared).`;
    } catch (cause) {
      if (gen === generation) error = `Configure cloud session failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function switchRoleAndConnect() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before probing cloud status.';
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const currentRole = configStatus?.activeRoleMode ?? connectionReport?.activeRoleMode;
      if (currentRole !== selectedRoleMode) {
        const statusReq = $state.snapshot({ roleMode: selectedRoleMode });
        const status = await port.configureCloudSyncSession(statusReq);
        if (gen !== generation) return;
        configStatus = status;
      }
      const report = await port.testCloudConnection();
      if (gen !== generation) return;
      connectionReport = report;
      selectedRoleMode = report.activeRoleMode;
      statusMessage = report.connected
        ? `Connected to ${report.database} as ${report.activeRoleMode} (${report.latencyMs} ms, ${report.becmasterPlotCount} BECMaster plots).`
        : 'Cloud connection probe completed: server reported not connected.';
    } catch (cause) {
      if (gen === generation) error = `Cloud connection probe failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function syncProvincialStandards() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before syncing provincial standards.';
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.pullCloudStandards(contextId);
      if (gen !== generation) return;
      pullStandardsResult = res;
      statusMessage = res.alreadyCurrent
        ? `Provincial standards already current (Release ${res.releaseId}, BEC v${res.becVersion}, Taxa v${res.taxonomyVersion}).`
        : `Updated local provincial standards to Release ${res.releaseId} (BEC v${res.becVersion}, Taxa v${res.taxonomyVersion}).`;
      if (res.updated && res.state && oncontextswitched) {
        oncontextswitched(res.state);
      }
    } catch (cause) {
      if (gen === generation) error = `Sync Provincial Standards failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPreviewSubset() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before previewing BECMaster subset.';
      return;
    }
    const built = buildSubsetFilterDraft({
      zonesRaw: subsetZonesRaw,
      subzonesRaw: subsetSubzonesRaw,
      projectIdsRaw: subsetProjectIdsRaw,
      plotNumbersRaw: subsetPlotNumbersRaw,
      bboxRaw: subsetBboxRaw,
      limitRaw: subsetLimitRaw,
      allowFullCheckout: subsetAllowFull,
    });
    if (built.error || !built.filter) {
      error = built.error ?? 'Invalid subset filter.';
      return;
    }
    const filterPayload = $state.snapshot(built.filter);
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const preview = await port.previewBECMasterSubset(filterPayload);
      if (gen !== generation) return;
      subsetPreview = preview;
      statusMessage = `Subset preview matched ${preview.matchingPlots} plot(s) and ${preview.matchingVeg} vegetation row(s).`;
    } catch (cause) {
      if (gen === generation) error = `Preview BECMaster Subset failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPullSubset() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before downloading BECMaster subset.';
      return;
    }
    const projErr = validateProjectNameInput(subsetTargetProject);
    if (projErr) {
      error = projErr;
      return;
    }
    const built = buildSubsetFilterDraft({
      zonesRaw: subsetZonesRaw,
      subzonesRaw: subsetSubzonesRaw,
      projectIdsRaw: subsetProjectIdsRaw,
      plotNumbersRaw: subsetPlotNumbersRaw,
      bboxRaw: subsetBboxRaw,
      limitRaw: subsetLimitRaw,
      allowFullCheckout: subsetAllowFull,
    });
    if (built.error || !built.filter) {
      error = built.error ?? 'Invalid subset filter.';
      return;
    }
    const req = $state.snapshot({
      filter: built.filter,
      targetProject: subsetTargetProject,
      destinationPath: subsetDestinationPath.trim() || undefined,
      activateOnSuccess: subsetActivateOnSuccess,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.pullBECMasterSubset(contextId, req);
      if (gen !== generation) return;
      pullSubsetResult = res;
      statusMessage = `Checked out ${res.plotCount} plot(s) (${res.vegCount} veg rows) into "${res.targetProject}" at ${res.destinationPath}.`;
      if (res.activated && res.state && oncontextswitched) {
        oncontextswitched(res.state);
      }
    } catch (cause) {
      if (gen === generation) error = `Download Regional Subset failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runStagingPreflight() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before running staging pre-flight.';
      return;
    }
    const useSu = $state.snapshot(stagingUseActiveSu);
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const preflight = await port.previewStagingSubmission(contextId, useSu);
      if (gen !== generation) return;
      stagingPreflight = preflight;
      stagingCustodianOverride = false;
      statusMessage = preflight.compliancePassed
        ? `Pre-flight compliance passed for "${preflight.project}" (${preflight.plotCount} plots, ${preflight.userSpeciesManifest.rowCount} personal species).`
        : `Pre-flight detected ${preflight.blockingErrorCount} blocking compliance error(s) in "${preflight.project}".`;
    } catch (cause) {
      if (gen === generation) error = `Staging pre-flight preview failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runSubmitToStaging() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before uploading to staging.';
      return;
    }
    if (!stagingPreflight) {
      error = 'Run Pre-Flight Compliance & Package Preview before uploading a staging bundle.';
      return;
    }
    if (!canUploadStagingSubmission(stagingPreflight, activeRoleMode, stagingCustodianOverride)) {
      error = 'Upload blocked: resolve pre-flight compliance errors before submitting to BECMaster staging.';
      return;
    }
    const req = $state.snapshot({
      useActiveSu: stagingUseActiveSu,
      submittedBy: stagingSubmittedBy.trim() || undefined,
      submitterNotes: stagingSubmitterNotes.trim() || undefined,
      baseSnapshotTag: stagingBaseSnapshotTag.trim() || undefined,
      custodianOverride: !stagingPreflight.compliancePassed && activeRoleMode === 'custodian' ? stagingCustodianOverride : undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.submitProjectToStaging(contextId, req);
      if (gen !== generation) return;
      submitStagingResult = res;
      statusMessage = `Uploaded staging submission ${res.submissionId} for "${res.projectName}" (${res.plotCount} plots, status: ${res.status}).`;
    } catch (cause) {
      if (gen === generation) error = `Upload Staging Bundle failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function refreshSubmissionsList() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before refreshing staging submissions.';
      return;
    }
    const filter = $state.snapshot({
      status: submissionsStatusFilter || undefined,
      submittedBy: submissionsSubmittedByFilter.trim() || undefined,
      limit: submissionsLimit > 0 ? submissionsLimit : 50,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const rows = await port.listStagingSubmissions(filter);
      if (gen !== generation) return;
      submissions = rows ?? [];
      statusMessage = `Loaded ${submissions.length} staging submission(s).`;
    } catch (cause) {
      if (gen === generation) error = `Refresh Submissions failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runBootstrapSchemas() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before bootstrapping cloud schemas.';
      return;
    }
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.bootstrapCloudSchemas();
      if (gen !== generation) return;
      bootstrapResult = res;
      statusMessage = `Verified cloud schemas v${res.schemaVersion} (${res.schemasVerified.join(', ')}); PostGIS=${res.postgisEnabled}, pgcrypto=${res.pgcryptoEnabled}.`;
    } catch (cause) {
      if (gen === generation) error = `Bootstrap Cloud Schemas failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPublishStandards() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before publishing provincial standards.';
      return;
    }
    const req = $state.snapshot({
      releaseId: pubReleaseId.trim() || undefined,
      becVersion: pubBecVersion.trim() || undefined,
      taxonomyVersion: pubTaxonomyVersion.trim() || undefined,
      vlistsPath: pubVlistsPath.trim() || undefined,
      becDbPath: pubBecDbPath.trim() || undefined,
      vmetaDataPath: pubVmetaDataPath.trim() || undefined,
      notes: pubNotes.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.publishStandardsToCloud(contextId, req);
      if (gen !== generation) return;
      publishStandardsResult = res;
      statusMessage = `Published provincial standards release "${res.releaseId}" (BEC v${res.becVersion}, Taxonomy v${res.taxonomyVersion}) in ${res.elapsedMs} ms.`;
    } catch (cause) {
      if (gen === generation) error = `Publish Standards to Cloud failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runSeedBaseline() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before seeding BECMaster baseline.';
      return;
    }
    if (!seedSourceDbPath.trim()) {
      error = 'Source VP08 .db file path is required to seed the BECMaster baseline.';
      return;
    }
    const req = $state.snapshot({
      sourceDbPath: seedSourceDbPath.trim(),
      sourceProject: seedSourceProject.trim() || undefined,
      snapshotTag: seedSnapshotTag.trim() || undefined,
      replaceExisting: seedReplaceExisting,
      skipAudit: seedSkipAudit || undefined,
      notes: seedNotes.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.seedBECMasterBaseline(req);
      if (gen !== generation) return;
      seedBaselineResult = res;
      statusMessage = `Seeded BECMaster baseline from "${res.sourceProject}" (${res.plotCount} plots, ${res.vegCount} veg rows, snapshot ${res.snapshotTag}).`;
    } catch (cause) {
      if (gen === generation) error = `Seed BECMaster Baseline failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function inspectSubmission(submissionId: string) {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before inspecting submission details.';
      return;
    }
    const idSnap = $state.snapshot(submissionId);
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const detail = await port.getStagingSubmissionDetail(idSnap);
      if (gen !== generation) return;
      selectedSubmissionDetail = detail;
      statusMessage = `Loaded submission ${detail.summary.submissionId} (${detail.provisionalSpeciesDiffs.length} provisional species crosswalk item(s)).`;
    } catch (cause) {
      if (gen === generation) error = `Inspect Submission Detail failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runPreviewStagingMerge(submissionId: string) {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before previewing 3-way staging merge.';
      return;
    }
    const req = $state.snapshot({
      submissionId,
      baseDbPath: mergeBaseDbPath.trim() || undefined,
      baseProject: mergeBaseProject.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const preview = await port.previewStagingToMasterMerge(req);
      if (gen !== generation) return;
      mergePreview = preview;
      mergeExecuteResult = null;
      mergePlotResolutions = {};
      mergeFieldResolutions = buildInitialCloudMergeFieldResolutions(preview);
      statusMessage = `3-Way Merge preview for ${preview.submissionId}: ${preview.addedCount} Added, ${preview.fastForwardCount} Fast-Forward, ${preview.conflictCount} Conflict, ${preview.identicalCount} Identical.`;
    } catch (cause) {
      if (gen === generation) error = `Preview Staging-to-Master Merge failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  function setMergePlotResolution(plotNumber: string, value: string) {
    const next = { ...mergePlotResolutions };
    if (value === 'keep_target' || value === 'accept_source') {
      next[plotNumber] = value;
    } else {
      delete next[plotNumber];
    }
    mergePlotResolutions = next;
  }

  function setMergeFieldResolution(conflictId: string, choice: string) {
    const next = { ...mergeFieldResolutions };
    if (choice === 'keep_target' || choice === 'accept_source') {
      next[conflictId] = choice;
    } else {
      delete next[conflictId];
    }
    mergeFieldResolutions = next;
  }

  async function runExecuteStagingMerge() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before executing BECMaster merge.';
      return;
    }
    if (!mergePreview) {
      error = 'Run Review & 3-Way Diff vs BECMaster before approving merge.';
      return;
    }
    const req = $state.snapshot({
      submissionId: mergePreview.submissionId,
      baseDbPath: mergePreview.baseDbPath || mergeBaseDbPath.trim() || undefined,
      baseProject: mergePreview.baseProject || mergeBaseProject.trim() || undefined,
      expectedPreviewDigest: mergePreview.previewDigest,
      defaultConflictResolution: mergeDefaultResolution,
      plotResolutions: mergePlotResolutions,
      fieldResolutions: mergeFieldResolutions,
      reviewerNotes: mergeReviewerNotes.trim() || undefined,
      actor: mergeActor.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const res = await port.executeStagingToMasterMerge(req);
      if (gen !== generation) return;
      mergeExecuteResult = res;
      mergePreview = null;
      statusMessage = `Merged submission ${res.submissionId} (tx ${res.mergeTxId}): +${res.addedPlots} added, ${res.updatedPlots} updated, ${res.auditRowsWritten} audit row(s).`;
    } catch (cause) {
      if (gen === generation) error = `Approve & Merge into BECMaster failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runRejectSubmission(submissionId?: string) {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before rejecting a staging submission.';
      return;
    }
    const targetId = submissionId ?? mergePreview?.submissionId ?? selectedSubmissionDetail?.summary.submissionId;
    if (!targetId) {
      error = 'Select a staging submission before rejecting.';
      return;
    }
    if (!mergeReviewerNotes.trim()) {
      error = 'Reviewer notes are required when rejecting a staging submission.';
      return;
    }
    const req = $state.snapshot({
      submissionId: targetId,
      reviewerNotes: mergeReviewerNotes.trim(),
      actor: mergeActor.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const summary = await port.rejectStagingSubmission(req);
      if (gen !== generation) return;
      submissions = submissions.map(s => (s.submissionId === summary.submissionId ? summary : s));
      if (mergePreview?.submissionId === summary.submissionId) mergePreview = null;
      statusMessage = `Rejected submission ${summary.submissionId} (status: ${summary.status}).`;
    } catch (cause) {
      if (gen === generation) error = `Reject Staging Submission failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function runCreateSnapshot() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before cutting a BECMaster snapshot.';
      return;
    }
    const tagErr = validateSnapshotTagInput(snapshotTag);
    if (tagErr) {
      error = tagErr;
      return;
    }
    const req = $state.snapshot({
      snapshotTag,
      notes: snapshotNotes.trim() || undefined,
      exportFormat: snapshotExportFormat || undefined,
      destinationPath: snapshotDestinationPath.trim() || undefined,
      zoneFilter: snapshotZoneFilter.trim() || undefined,
    });
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const rec = await port.createBECMasterSnapshot(contextId, req);
      if (gen !== generation) return;
      latestSnapshotCreated = rec;
      snapshots = [rec, ...snapshots.filter(s => s.snapshotTag !== rec.snapshotTag)];
      statusMessage = `Created BECMaster snapshot "${rec.snapshotTag}" (${rec.plotCount} plots, ${rec.vegCount} veg rows).`;
    } catch (cause) {
      if (gen === generation) error = `Create BECMaster Snapshot failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }

  async function refreshSnapshotsAndLedger() {
    if (busy || operationHeld) {
      error = 'Wait for the current operation or decision before loading snapshots and download log.';
      return;
    }
    const limitSnap = $state.snapshot(downloadLogLimit > 0 ? downloadLogLimit : 50);
    const gen = ++generation;
    setBusy(true);
    error = null;
    statusMessage = null;
    try {
      const [snapRows, logRows] = await Promise.all([
        port.listBECMasterSnapshots(),
        port.listBECMasterDownloadLog(limitSnap),
      ]);
      if (gen !== generation) return;
      snapshots = snapRows ?? [];
      downloadLog = logRows ?? [];
      statusMessage = `Loaded ${snapshots.length} snapshot(s) and ${downloadLog.length} download log record(s).`;
    } catch (cause) {
      if (gen === generation) error = `Load Snapshots & Download Log failed: ${String(cause)}`;
    } finally {
      if (gen === generation) setBusy(false);
    }
  }
</script>

<div class="min-w-0 space-y-5 text-[#313132]" data-cloud-sync-workbench aria-label="Cloud BECMaster Sync Workbench">
  <!-- 1. Connection & Role Bar -->
  <section class="overflow-hidden rounded border border-slate-300 bg-white shadow-sm" aria-label="Cloud Connection and Session Role">
    <div class="border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-4 py-3 text-white">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs font-semibold uppercase tracking-wider text-[#FCBA19]">Cloud BECMaster Sync</span>
          {#if connectionReport?.connected}
            <span class="rounded bg-emerald-700 px-2 py-0.5 text-xs font-semibold text-white" data-cloud-status-badge="connected">
              Connected ({connectionReport.latencyMs} ms)
            </span>
          {:else}
            <span class="rounded bg-slate-700 px-2 py-0.5 text-xs font-semibold text-slate-100" data-cloud-status-badge="offline">
              Offline / Unprobed
            </span>
          {/if}
          <span class="rounded bg-[#FCBA19] px-2 py-0.5 text-xs font-bold text-[#003366]" data-cloud-role-badge={activeRoleMode}>
            {activeRoleMode === 'custodian' ? 'Custodian Admin' : 'Contributor'}
          </span>
          <span class="rounded border border-slate-400/50 bg-[#002244] px-2 py-0.5 font-mono text-[11px] text-slate-200" data-cloud-env-indicator>
            {#if configStatus?.sessionOverride}
              Session Override ({configStatus.configSource})
            {:else if configStatus?.envFileDetected}
              becmaster.env detected ({configStatus.configSource})
            {:else if configStatus}
              Source: {configStatus.configSource || 'unconfigured'}
            {:else}
              Local config unprobed
            {/if}
          </span>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <label class="sr-only" for="cloud-role-mode-select">Active Cloud Role</label>
          <select
            id="cloud-role-mode-select"
            class="rounded border border-slate-400 bg-white px-2.5 py-1.5 text-xs font-semibold text-[#003366]"
            bind:value={selectedRoleMode}
            disabled={busy || operationHeld}
          >
            <option value="contributor">Role: Contributor</option>
            <option value="custodian">Role: Custodian Admin</option>
          </select>
          <button
            type="button"
            disabled={busy || operationHeld}
            class="rounded border border-[#FCBA19] bg-[#FCBA19] px-3 py-1.5 text-xs font-bold text-[#003366] hover:bg-amber-400 disabled:opacity-50"
            onclick={switchRoleAndConnect}
          >
            Test Connection / Refresh Cloud Status
          </button>
          <button
            type="button"
            disabled={busy || operationHeld}
            class="rounded border border-white/70 bg-[#002244] px-3 py-1.5 text-xs font-semibold text-white hover:bg-slate-800 disabled:opacity-50"
            onclick={syncProvincialStandards}
          >
            Sync Provincial Standards
          </button>
        </div>
      </div>
    </div>

    <!-- Cloud Counters & Telemetry Strip -->
    <div class="grid grid-cols-2 gap-2 border-b border-slate-200 bg-slate-50 px-4 py-2.5 text-xs sm:grid-cols-5">
      <div>
        <div class="text-[11px] text-slate-600">BECMaster Plots</div>
        <div class="font-mono text-sm font-bold text-[#003366]">
          {connectionReport ? connectionReport.becmasterPlotCount.toLocaleString() : '—'}
        </div>
      </div>
      <div>
        <div class="text-[11px] text-slate-600">BECMaster Veg Rows</div>
        <div class="font-mono text-sm font-bold text-[#003366]">
          {connectionReport ? connectionReport.becmasterVegCount.toLocaleString() : '—'}
        </div>
      </div>
      <div>
        <div class="text-[11px] text-slate-600">Pending Staging Queue</div>
        <div class="font-mono text-sm font-bold text-[#003366]">
          {connectionReport ? connectionReport.pendingSubmissionCount : '—'}
        </div>
      </div>
      <div>
        <div class="text-[11px] text-slate-600">Dated Snapshots</div>
        <div class="font-mono text-sm font-bold text-[#003366]">
          {connectionReport ? `${connectionReport.snapshotCount}${connectionReport.latestSnapshotTag ? ` (${connectionReport.latestSnapshotTag})` : ''}` : '—'}
        </div>
      </div>
      <div>
        <div class="text-[11px] text-slate-600">Provincial Standards</div>
        <div class="font-mono text-xs font-semibold text-[#003366]">
          {#if connectionReport?.activeStandardsRelease}
            {connectionReport.activeStandardsRelease} (BEC v{connectionReport.activeBecVersion ?? '?'} / Taxa v{connectionReport.activeTaxonomyVersion ?? '?'})
          {:else if pullStandardsResult}
            {pullStandardsResult.releaseId} (BEC v{pullStandardsResult.becVersion} / Taxa v{pullStandardsResult.taxonomyVersion})
          {:else}
            —
          {/if}
        </div>
      </div>
    </div>

    <!-- Collapsible Session Credential Override Panel -->
    <details class="px-4 py-2.5 text-xs" data-session-override-panel>
      <summary class="cursor-pointer font-semibold text-[#003366] select-none">
        Session Credential Override (In-Memory Only — Never Persisted to Disk)
      </summary>
      <div class="mt-3 space-y-3 border-t border-slate-200 pt-3">
        <p class="text-xs text-slate-600">
          Credentials entered here apply strictly to the current desktop memory session and are never written to <span class="font-mono">config.yml</span> or echoed after submission.
        </p>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-4">
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-host">PostgreSQL Host</label>
            <input
              id="cloud-override-host"
              type="text"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overrideHost}
              disabled={busy || operationHeld}
              placeholder="Hostname or IPv4/IPv6"
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-port">Port</label>
            <input
              id="cloud-override-port"
              type="number"
              min="1"
              max="65535"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overridePort}
              disabled={busy || operationHeld}
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-db">Database</label>
            <input
              id="cloud-override-db"
              type="text"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overrideDatabase}
              disabled={busy || operationHeld}
              placeholder="becmaster"
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-ssl">SSL Mode</label>
            <select
              id="cloud-override-ssl"
              class="w-full rounded border border-slate-300 p-1.5 text-xs"
              bind:value={overrideSslMode}
              disabled={busy || operationHeld}
            >
              <option value="require">require</option>
              <option value="verify-full">verify-full</option>
              <option value="verify-ca">verify-ca</option>
              <option value="prefer">prefer</option>
              <option value="disable">disable</option>
            </select>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-4">
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-user">Contributor User</label>
            <input
              id="cloud-override-user"
              type="text"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overrideUser}
              disabled={busy || operationHeld}
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-password">Contributor Password</label>
            <input
              id="cloud-override-password"
              type="password"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overridePassword}
              disabled={busy || operationHeld}
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-admin-user">Custodian Admin User</label>
            <input
              id="cloud-override-admin-user"
              type="text"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overrideAdminUser}
              disabled={busy || operationHeld}
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="cloud-override-admin-password">Custodian Admin Password</label>
            <input
              id="cloud-override-admin-password"
              type="password"
              autocomplete="off"
              class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs"
              bind:value={overrideAdminPassword}
              disabled={busy || operationHeld}
            />
          </div>
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            disabled={busy || operationHeld}
            class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={() => applySessionConfig(false)}
          >
            Apply Session Override
          </button>
          <button
            type="button"
            disabled={busy || operationHeld}
            class="rounded border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-800 hover:bg-slate-100 disabled:opacity-50"
            onclick={() => applySessionConfig(true)}
          >
            Clear Session Override
          </button>
        </div>
      </div>
    </details>
  </section>

  {#if error}
    <p role="alert" class="rounded border-l-4 border-red-700 bg-red-50 p-3 text-sm text-red-900">{error}</p>
  {/if}
  {#if statusMessage}
    <p role="status" class="rounded border-l-4 border-[#003366] bg-slate-50 p-3 text-sm text-slate-900">{statusMessage}</p>
  {/if}

  <!-- 2. Contributor Panel -->
  <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
    <!-- 2a. Checkout Regional Subset from BECMaster -->
    <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] bg-white p-4 shadow-sm">
      <legend class="px-1 text-sm font-semibold text-[#003366]">Checkout Regional Subset from BECMaster</legend>
      <p class="text-xs text-slate-600">
        Query canonical <span class="font-mono">becmaster</span> plots by BGC Zone/Subzone, ProjectID, literal PlotNumbers (preserving leading zeros such as <span class="font-mono">'00337'</span>), or WGS84 bounding box into a local <span class="font-mono">VP08</span> SQLite project.
      </p>

      <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="subset-zones">BGC Zones (comma-separated)</label>
          <input
            id="subset-zones"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetZonesRaw}
            disabled={busy || operationHeld}
            placeholder="e.g. CWH, ICH, SBS"
          />
        </div>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="subset-subzones">Subzones / Variants (comma-separated)</label>
          <input
            id="subset-subzones"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetSubzonesRaw}
            disabled={busy || operationHeld}
            placeholder="e.g. vm1, mk1"
          />
        </div>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="subset-project-ids">Project IDs (comma-separated)</label>
          <input
            id="subset-project-ids"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetProjectIdsRaw}
            disabled={busy || operationHeld}
            placeholder="e.g. HGW2019, MANNING"
          />
        </div>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="subset-plot-numbers">Literal Plot Numbers (max 7 UTF-16)</label>
          <input
            id="subset-plot-numbers"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetPlotNumbersRaw}
            disabled={busy || operationHeld}
            placeholder="e.g. 00337, 108050"
          />
        </div>
      </div>

      <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
        <div class="space-y-1 sm:col-span-2">
          <label class="block text-xs font-semibold text-slate-700" for="subset-bbox">Spatial BBox (minLon, minLat, maxLon, maxLat)</label>
          <input
            id="subset-bbox"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetBboxRaw}
            disabled={busy || operationHeld}
            placeholder="e.g. -133.2, 52.0, -131.0, 54.3"
          />
        </div>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="subset-limit">Plot Limit (optional)</label>
          <input
            id="subset-limit"
            type="number"
            min="1"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={subsetLimitRaw}
            disabled={busy || operationHeld}
            placeholder="5000"
          />
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-2">
        <label class="flex items-center gap-2 text-xs text-slate-800">
          <input type="checkbox" bind:checked={subsetAllowFull} disabled={busy || operationHeld} />
          Allow full provincial BECMaster checkout without regional filter
        </label>
        <button
          type="button"
          disabled={busy || operationHeld}
          class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
          onclick={runPreviewSubset}
        >
          Preview Matching Plots
        </button>
      </div>

      {#if subsetPreview}
        <div class="space-y-1.5 rounded border border-slate-200 bg-slate-50 p-3 text-xs">
          <div class="font-semibold text-[#003366]">
            Matched {subsetPreview.matchingPlots.toLocaleString()} plot(s) &middot; {subsetPreview.matchingVeg.toLocaleString()} vegetation row(s)
          </div>
          {#if subsetPreview.samplePlots.length > 0}
            <div>Sample Plots: <span class="font-mono">{subsetPreview.samplePlots.join(', ')}</span></div>
          {/if}
          {#if Object.keys(subsetPreview.zoneBreakdown).length > 0}
            <div>
              Zone Breakdown:
              <span class="font-mono">
                {Object.entries(subsetPreview.zoneBreakdown)
                  .map(([z, cnt]) => `${z}: ${cnt}`)
                  .join(' | ')}
              </span>
            </div>
          {/if}
        </div>
      {/if}

      <div class="space-y-2 border-t border-slate-200 pt-3">
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="subset-target-project">Target VP08 Project Name</label>
            <input
              id="subset-target-project"
              type="text"
              class="w-full rounded border border-slate-300 p-2 text-xs"
              bind:value={subsetTargetProject}
              disabled={busy || operationHeld}
              aria-invalid={!!subsetTargetValidationError}
              placeholder="e.g. Skeena_Checkout_2026"
            />
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="subset-dest-path">Destination .db Path (optional)</label>
            <input
              id="subset-dest-path"
              type="text"
              class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
              bind:value={subsetDestinationPath}
              disabled={busy || operationHeld}
              placeholder="Defaults to projects/<Target>.db"
            />
          </div>
        </div>
        {#if subsetTargetValidationError}
          <p role="alert" class="text-xs text-red-700">{subsetTargetValidationError}</p>
        {/if}
        <label class="flex items-center gap-2 text-xs text-slate-800">
          <input type="checkbox" bind:checked={subsetActivateOnSuccess} disabled={busy || operationHeld} />
          Activate checked-out project on success
        </label>
        <button
          type="button"
          disabled={busy || operationHeld || !subsetTargetProject || !!subsetTargetValidationError}
          class="w-full rounded border-2 border-[#FCBA19] bg-[#003366] px-3 py-2 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
          onclick={runPullSubset}
        >
          Download Regional Subset (.db)
        </button>
        {#if pullSubsetResult}
          <p class="break-all text-xs text-slate-700">
            Saved <span class="font-mono">{pullSubsetResult.targetProject}</span> ({pullSubsetResult.plotCount} plots, log #{pullSubsetResult.downloadLogId}, SHA-256 <span class="font-mono">{pullSubsetResult.destinationSha256.slice(0, 16)}</span>).
          </p>
        {/if}
      </div>
    </fieldset>

    <!-- 2b. Submit Active Project to BECMaster Staging -->
    <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] bg-white p-4 shadow-sm">
      <legend class="px-1 text-sm font-semibold text-[#003366]">Submit Active Project to BECMaster Staging</legend>
      <p class="text-xs text-slate-600">
        Packages <span class="font-mono font-semibold">{activeProject}</span> with automated compliance validation, <span class="font-mono">VUser.USysUserSpp</span> personal species manifest, and <span class="font-mono">_Lump</span> / <span class="font-mono">_SU</span> companions into <span class="font-mono">staging.submissions</span>.
      </p>

      <div class="flex flex-wrap items-center justify-between gap-2 rounded border border-slate-200 bg-slate-50 p-2.5">
        <label class="flex items-center gap-2 text-xs text-slate-800">
          <input
            type="checkbox"
            bind:checked={stagingUseActiveSu}
            disabled={busy || operationHeld || activeSu === 'None'}
          />
          Scope pre-flight &amp; staging bundle to active SU ({activeSu})
        </label>
        <button
          type="button"
          disabled={busy || operationHeld}
          class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
          onclick={runStagingPreflight}
        >
          Run Pre-Flight Compliance &amp; Package Preview
        </button>
      </div>

      {#if stagingPreflight}
        <div class="space-y-2 rounded border border-slate-200 bg-slate-50 p-3 text-xs" data-staging-preflight-summary>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <span class="font-semibold text-[#003366]">
              Project: <span class="font-mono">{stagingPreflight.project}</span> ({stagingPreflight.plotCount} plots, {stagingPreflight.vegRowCount} veg rows)
            </span>
            {#if stagingPreflight.compliancePassed}
              <span class="rounded bg-emerald-700 px-2 py-0.5 text-xs font-semibold text-white" data-preflight-badge="passed">
                Compliance Passed (0 Blocking)
              </span>
            {:else}
              <span class="rounded bg-red-700 px-2 py-0.5 text-xs font-semibold text-white" data-preflight-badge="failed">
                Compliance Failed ({stagingPreflight.blockingErrorCount} Blocking)
              </span>
            {/if}
          </div>
          <div class="grid grid-cols-3 gap-2 text-[11px] text-slate-700">
            <div>Personal Species (<span class="font-mono">USysUserSpp</span>): <span class="font-bold">{stagingPreflight.userSpeciesManifest.rowCount}</span></div>
            <div>Lump Rows: <span class="font-bold">{stagingPreflight.lumpRowCount}</span></div>
            <div>SU Rows: <span class="font-bold">{stagingPreflight.suRowCount}</span></div>
          </div>

          {#if stagingPreflight.userSpeciesManifest.species.length > 0}
            <div class="max-h-32 overflow-auto rounded border border-slate-200 bg-white">
              <table class="w-full text-left text-[11px]">
                <thead class="bg-slate-100 text-slate-700">
                  <tr><th class="p-1.5">Code</th><th class="p-1.5">Scientific Name</th><th class="p-1.5">LifeForm</th></tr>
                </thead>
                <tbody>
                  {#each stagingPreflight.userSpeciesManifest.species as sp (sp.code)}
                    <tr class="border-t border-slate-100">
                      <td class="p-1.5 font-mono font-semibold">{sp.code}</td>
                      <td class="p-1.5">{sp.scientificName ?? '—'}</td>
                      <td class="p-1.5 font-mono">{sp.lifeForm ?? '—'}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}

          {#if !stagingPreflight.compliancePassed && activeRoleMode === 'custodian'}
            <label class="flex items-center gap-2 rounded border border-amber-400 bg-amber-50 p-2 text-xs font-medium text-amber-950">
              <input type="checkbox" bind:checked={stagingCustodianOverride} disabled={busy || operationHeld} />
              Custodian Admin Override: permit staging upload despite blocking compliance findings
            </label>
          {/if}
        </div>
      {/if}

      <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="staging-submitted-by">Submitter Name / ID</label>
          <input
            id="staging-submitted-by"
            type="text"
            class="w-full rounded border border-slate-300 p-2 text-xs"
            bind:value={stagingSubmittedBy}
            disabled={busy || operationHeld}
            placeholder="e.g. Regional Ecologist"
          />
        </div>
        <div class="space-y-1">
          <label class="block text-xs font-semibold text-slate-700" for="staging-base-tag">Base Snapshot Tag (optional)</label>
          <input
            id="staging-base-tag"
            type="text"
            class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
            bind:value={stagingBaseSnapshotTag}
            disabled={busy || operationHeld}
            placeholder="e.g. v13_2026_05"
          />
        </div>
      </div>

      <div class="space-y-1">
        <label class="block text-xs font-semibold text-slate-700" for="staging-notes">Submitter Notes</label>
        <textarea
          id="staging-notes"
          rows="2"
          class="w-full rounded border border-slate-300 p-2 text-xs"
          bind:value={stagingSubmitterNotes}
          disabled={busy || operationHeld}
          placeholder="Field season summary, plot range, or provisional taxonomy notes"
        ></textarea>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          disabled={busy || operationHeld || !canUploadStaging}
          class="rounded border-2 border-[#FCBA19] bg-[#003366] px-3 py-2 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
          onclick={runSubmitToStaging}
        >
          Upload Staging Bundle
        </button>
        <button
          type="button"
          disabled={busy || operationHeld}
          class="rounded border border-slate-300 bg-white px-3 py-2 text-xs font-semibold text-[#003366] hover:bg-slate-100 disabled:opacity-50"
          onclick={refreshSubmissionsList}
        >
          Refresh Submissions
        </button>
      </div>

      {#if submitStagingResult}
        <p class="text-xs text-emerald-900">
          Submission <span class="font-mono font-semibold">{submitStagingResult.submissionId}</span> recorded ({submitStagingResult.status} at {submitStagingResult.submittedAt}).
        </p>
      {/if}

      {#if submissions.length > 0 && activeRoleMode !== 'custodian'}
        <div class="max-h-48 overflow-auto rounded border border-slate-200">
          <table class="w-full text-left text-xs">
            <thead class="bg-slate-100 text-slate-700">
              <tr>
                <th class="p-2">Submission</th>
                <th class="p-2">Project</th>
                <th class="p-2">Plots</th>
                <th class="p-2">Status</th>
                <th class="p-2">Reviewer Notes</th>
              </tr>
            </thead>
            <tbody>
              {#each submissions as sub (sub.submissionId)}
                <tr class="border-t border-slate-100">
                  <td class="p-2 font-mono">{sub.submissionId.slice(0, 12)}</td>
                  <td class="p-2 font-mono">{sub.projectName}</td>
                  <td class="p-2 font-mono">{sub.plotCount}</td>
                  <td class="p-2 font-semibold">{sub.status}</td>
                  <td class="p-2 text-slate-600">{sub.reviewerNotes ?? '—'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </fieldset>
  </div>

  <!-- 3. Custodian Inbox, 3-Way Merge, Standards & Snapshots Panel -->
  {#if activeRoleMode === 'custodian'}
    <section class="space-y-4 rounded border border-slate-300 bg-white p-4 shadow-sm" data-custodian-panel aria-label="Custodian Inbox, 3-Way Merge, Standards and Snapshots">
      <header class="flex flex-wrap items-center justify-between gap-2 border-b-[3px] border-b-[#FCBA19] bg-[#003366] px-4 py-3 text-white">
        <div>
          <p class="text-xs font-semibold uppercase tracking-wider text-[#FCBA19]">Custodian Governance Console</p>
          <h3 class="m-0 text-base font-semibold text-white">Schema Bootstrap, 2026 Standards Publishing, Staging 3-Way Merge &amp; Dated Snapshots</h3>
        </div>
        <button
          type="button"
          disabled={busy || operationHeld}
          class="rounded bg-[#FCBA19] px-3 py-1.5 text-xs font-bold text-[#003366] hover:bg-amber-400 disabled:opacity-50"
          onclick={runBootstrapSchemas}
        >
          Bootstrap / Verify Schemas
        </button>
      </header>

      {#if bootstrapResult}
        <div class="rounded border border-slate-200 bg-slate-50 p-2.5 text-xs">
          Schema v<span class="font-mono font-bold">{bootstrapResult.schemaVersion}</span> verified ({bootstrapResult.schemasVerified.join(', ')}) &middot; PostGIS: {bootstrapResult.postgisEnabled ? 'Enabled' : 'No'} &middot; pgcrypto: {bootstrapResult.pgcryptoEnabled ? 'Enabled' : 'No'} &middot; Role Grants: {bootstrapResult.roleGrantsApplied ? 'Applied' : 'Skipped'}
        </div>
      {/if}

      <!-- Admin Maintenance Bar: Publish 2026 Standards & Collapsible Seed Baseline -->
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <fieldset class="space-y-2.5 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-3">
          <legend class="px-1 text-xs font-semibold text-[#003366]">Publish 2026 Standards to Cloud</legend>
          <div class="grid grid-cols-3 gap-2">
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-release-id">Release ID</label>
              <input id="pub-release-id" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubReleaseId} disabled={busy || operationHeld} placeholder="BECv13_Taxa19_2026" />
            </div>
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-bec-ver">BEC Version</label>
              <input id="pub-bec-ver" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubBecVersion} disabled={busy || operationHeld} />
            </div>
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-taxa-ver">Taxonomy Version</label>
              <input id="pub-taxa-ver" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubTaxonomyVersion} disabled={busy || operationHeld} />
            </div>
          </div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-vlists-path">VLists.db Path (opt)</label>
              <input id="pub-vlists-path" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubVlistsPath} disabled={busy || operationHeld} />
            </div>
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-becdb-path">BECdb Path (opt)</label>
              <input id="pub-becdb-path" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubBecDbPath} disabled={busy || operationHeld} />
            </div>
            <div class="space-y-1">
              <label class="block text-[11px] font-semibold text-slate-700" for="pub-vmeta-path">VMetaData.db Path (opt)</label>
              <input id="pub-vmeta-path" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={pubVmetaDataPath} disabled={busy || operationHeld} />
            </div>
          </div>
          <div class="flex items-center justify-between gap-2">
            <input
              aria-label="Standards release notes"
              type="text"
              class="flex-1 rounded border border-slate-300 p-1.5 text-xs"
              bind:value={pubNotes}
              disabled={busy || operationHeld}
              placeholder="Release notes (optional)"
            />
            <button
              type="button"
              disabled={busy || operationHeld}
              class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runPublishStandards}
            >
              Publish 2026 Standards to Cloud
            </button>
          </div>
          {#if publishStandardsResult}
            <p class="text-[11px] text-slate-700">
              Published <span class="font-mono">{publishStandardsResult.releaseId}</span> (VLists SHA <span class="font-mono">{publishStandardsResult.vlistsSha256.slice(0, 12)}</span>).
            </p>
          {/if}
        </fieldset>

        <details class="rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-3 text-xs">
          <summary class="cursor-pointer font-semibold text-[#003366] select-none">
            Seed Baseline VP08 .db into BECMaster
          </summary>
          <div class="mt-3 space-y-2">
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div class="space-y-1">
                <label class="block text-[11px] font-semibold text-slate-700" for="seed-db-path">Source VP08 .db Path (required)</label>
                <input id="seed-db-path" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={seedSourceDbPath} disabled={busy || operationHeld} />
              </div>
              <div class="space-y-1">
                <label class="block text-[11px] font-semibold text-slate-700" for="seed-source-proj">Source Project Prefix (opt)</label>
                <input id="seed-source-proj" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={seedSourceProject} disabled={busy || operationHeld} />
              </div>
            </div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <div class="space-y-1">
                <label class="block text-[11px] font-semibold text-slate-700" for="seed-snap-tag">Initial Snapshot Tag (opt)</label>
                <input id="seed-snap-tag" type="text" class="w-full rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={seedSnapshotTag} disabled={busy || operationHeld} placeholder="baseline_2026" />
              </div>
              <div class="space-y-1">
                <label class="block text-[11px] font-semibold text-slate-700" for="seed-notes">Notes (opt)</label>
                <input id="seed-notes" type="text" class="w-full rounded border border-slate-300 p-1.5 text-xs" bind:value={seedNotes} disabled={busy || operationHeld} />
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-4">
              <label class="flex items-center gap-1.5 text-xs text-slate-800">
                <input type="checkbox" bind:checked={seedReplaceExisting} disabled={busy || operationHeld} />
                Replace existing becmaster tables
              </label>
              <label class="flex items-center gap-1.5 text-xs text-slate-800">
                <input type="checkbox" bind:checked={seedSkipAudit} disabled={busy || operationHeld} />
                Skip historical audit rows
              </label>
            </div>
            <button
              type="button"
              disabled={busy || operationHeld || !seedSourceDbPath.trim()}
              class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runSeedBaseline}
            >
              Seed Baseline VP08 .db
            </button>
            {#if seedBaselineResult}
              <p class="text-[11px] text-slate-700">
                Seeded {seedBaselineResult.plotCount.toLocaleString()} plots / {seedBaselineResult.vegCount.toLocaleString()} veg rows (snapshot <span class="font-mono">{seedBaselineResult.snapshotTag}</span>).
              </p>
            {/if}
          </div>
        </details>
      </div>

      <!-- Staging Submissions Queue -->
      <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
        <legend class="px-1 text-sm font-semibold text-[#003366]">Staging Submissions Queue (staging.submissions)</legend>
        <div class="flex flex-wrap items-end gap-2">
          <div class="space-y-1">
            <label class="block text-[11px] font-semibold text-slate-700" for="inbox-status-filter">Status</label>
            <select id="inbox-status-filter" class="rounded border border-slate-300 p-1.5 text-xs" bind:value={submissionsStatusFilter} disabled={busy || operationHeld}>
              <option value="">All Statuses</option>
              <option value="pending">pending</option>
              <option value="merged">merged</option>
              <option value="rejected">rejected</option>
            </select>
          </div>
          <div class="space-y-1">
            <label class="block text-[11px] font-semibold text-slate-700" for="inbox-submitter-filter">Submitted By</label>
            <input id="inbox-submitter-filter" type="text" class="rounded border border-slate-300 p-1.5 text-xs" bind:value={submissionsSubmittedByFilter} disabled={busy || operationHeld} placeholder="Any contributor" />
          </div>
          <div class="space-y-1">
            <label class="block text-[11px] font-semibold text-slate-700" for="merge-base-db-path">Optional Base .db Path</label>
            <input id="merge-base-db-path" type="text" class="rounded border border-slate-300 p-1.5 font-mono text-xs" bind:value={mergeBaseDbPath} disabled={busy || operationHeld} placeholder="Auto-resolve from snapshot" />
          </div>
          <button
            type="button"
            disabled={busy || operationHeld}
            class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
            onclick={refreshSubmissionsList}
          >
            Refresh Staging Queue
          </button>
        </div>

        {#if submissions.length === 0}
          <p class="text-xs text-slate-600">No staging submissions loaded. Click Refresh Staging Queue to query <span class="font-mono">staging.submissions</span>.</p>
        {:else}
          <div class="overflow-x-auto rounded border border-slate-200">
            <table class="w-full text-left text-xs">
              <thead class="bg-slate-100 text-slate-700">
                <tr>
                  <th class="p-2">Submission ID</th>
                  <th class="p-2">Project</th>
                  <th class="p-2">Submitter</th>
                  <th class="p-2">Plots / Veg</th>
                  <th class="p-2">Compliance</th>
                  <th class="p-2">Provisional Spp</th>
                  <th class="p-2">Status</th>
                  <th class="p-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {#each submissions as sub (sub.submissionId)}
                  <tr class="border-t border-slate-100">
                    <td class="p-2 font-mono">{sub.submissionId}</td>
                    <td class="p-2 font-mono font-semibold">{sub.projectName}</td>
                    <td class="p-2">{sub.submittedBy}</td>
                    <td class="p-2 font-mono">{sub.plotCount} / {sub.vegRowCount}</td>
                    <td class="p-2">
                      {#if sub.compliancePassed}
                        <span class="rounded bg-emerald-100 px-1.5 py-0.5 font-semibold text-emerald-900">Passed</span>
                      {:else}
                        <span class="rounded bg-red-100 px-1.5 py-0.5 font-semibold text-red-900">Failed</span>
                      {/if}
                    </td>
                    <td class="p-2">
                      {#if sub.userSpeciesCount > 0}
                        <span class="rounded bg-amber-100 px-1.5 py-0.5 font-mono font-semibold text-amber-950">{sub.userSpeciesCount} USysUserSpp</span>
                      {:else}
                        <span class="text-slate-500">0</span>
                      {/if}
                    </td>
                    <td class="p-2 font-semibold">{sub.status}</td>
                    <td class="p-2">
                      <div class="flex flex-wrap gap-1.5">
                        <button
                          type="button"
                          disabled={busy || operationHeld}
                          class="rounded border border-slate-300 bg-white px-2 py-1 text-[11px] font-semibold text-[#003366] hover:bg-slate-100 disabled:opacity-50"
                          onclick={() => inspectSubmission(sub.submissionId)}
                        >
                          Inspect Details
                        </button>
                        <button
                          type="button"
                          disabled={busy || operationHeld}
                          class="rounded bg-[#003366] px-2 py-1 text-[11px] font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
                          onclick={() => runPreviewStagingMerge(sub.submissionId)}
                        >
                          Review &amp; 3-Way Diff vs BECMaster
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}

        <!-- Submission Detail & Provisional Species Crosswalk -->
        {#if selectedSubmissionDetail}
          <div class="space-y-2 rounded border border-slate-300 bg-slate-50 p-3 text-xs" data-submission-detail>
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h4 class="m-0 text-xs font-bold text-[#003366]">
                Submission Detail: <span class="font-mono">{selectedSubmissionDetail.summary.submissionId}</span> ({selectedSubmissionDetail.summary.projectName})
              </h4>
              <span class="font-mono text-[11px] text-slate-600">
                Lump Rows: {selectedSubmissionDetail.lumpRows.length} | SU Assignments: {selectedSubmissionDetail.suRows.length}
              </span>
            </div>
            {#if selectedSubmissionDetail.summary.submitterNotes}
              <p class="m-0 text-xs text-slate-700">Submitter Notes: {selectedSubmissionDetail.summary.submitterNotes}</p>
            {/if}
            {#if selectedSubmissionDetail.provisionalSpeciesDiffs.length > 0}
              <div class="overflow-x-auto rounded border border-slate-200 bg-white">
                <table class="w-full text-left text-[11px]">
                  <thead class="bg-slate-100 text-slate-700">
                    <tr>
                      <th class="p-1.5">Submitted Code</th>
                      <th class="p-1.5">Submitted Scientific</th>
                      <th class="p-1.5">LifeForm</th>
                      <th class="p-1.5">USysAllSpecs Status</th>
                      <th class="p-1.5">Official Match</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each selectedSubmissionDetail.provisionalSpeciesDiffs as diff (diff.code)}
                      <tr class="border-t border-slate-100">
                        <td class="p-1.5 font-mono font-semibold">{diff.code}</td>
                        <td class="p-1.5">{diff.scientificName ?? '—'}</td>
                        <td class="p-1.5 font-mono">{diff.lifeForm ?? '—'}</td>
                        <td class="p-1.5 font-semibold">{diff.status}</td>
                        <td class="p-1.5 font-mono">{diff.officialCode ? `${diff.officialCode} (${diff.officialScientific ?? ''})` : '—'}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </div>
        {/if}
      </fieldset>

      <!-- Interactive 3-Way Merge Resolution Table -->
      {#if mergePreview || mergeExecuteResult || selectedSubmissionDetail}
        <fieldset class="space-y-3 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4" data-cloud-merge-panel>
          <legend class="px-1 text-sm font-semibold text-[#003366]">Interactive 3-Way Merge Resolution vs BECMaster</legend>

          {#if mergePreview}
            <div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
              <div class="rounded border border-slate-300 p-2.5">
                <div class="text-[11px] text-slate-600">Added Plots</div>
                <div class="font-mono text-lg font-bold text-[#003366]">{mergePreview.addedCount}</div>
              </div>
              <div class="rounded border border-slate-300 p-2.5">
                <div class="text-[11px] text-slate-600">Identical Plots</div>
                <div class="font-mono text-lg font-bold text-[#003366]">{mergePreview.identicalCount}</div>
              </div>
              <div class="rounded border border-slate-300 p-2.5">
                <div class="text-[11px] text-slate-600">Fast-Forward</div>
                <div class="font-mono text-lg font-bold text-[#003366]">{mergePreview.fastForwardCount}</div>
              </div>
              <div class="rounded border border-slate-300 p-2.5">
                <div class="text-[11px] text-slate-600">Conflicts</div>
                <div class="font-mono text-lg font-bold text-red-800">{mergePreview.conflictCount}</div>
              </div>
              <div class="rounded border border-amber-400 bg-amber-50 p-2.5">
                <div class="text-[11px] font-semibold text-amber-950">Custodian-Protected</div>
                <div class="font-mono text-lg font-bold text-amber-950">{mergePreview.custodianProtectedDiffs.length}</div>
              </div>
            </div>

            <p class="text-xs text-slate-600">
              Digest: <span class="font-mono">{mergePreview.previewDigest}</span> &middot; Protected BEC fields (<span class="font-mono">{CUSTODIAN_PROTECTED_FIELDS.join(', ')}</span>) default to keeping canonical BECMaster values unless explicitly overridden below.
            </p>

            {#if mergePreview.plots.length > 0}
              <div class="max-h-40 overflow-auto rounded border border-slate-200">
                <table class="w-full text-left text-xs">
                  <thead class="bg-slate-100 text-slate-700">
                    <tr>
                      <th class="p-2">Plot</th>
                      <th class="p-2">Category</th>
                      <th class="p-2">Diff Items</th>
                      <th class="p-2">Plot-Level Resolution Override</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each mergePreview.plots as plot (plot.plotNumber)}
                      <tr class="border-t border-slate-100">
                        <td class="p-2 font-mono font-semibold">{plot.plotNumber}</td>
                        <td class="p-2">{plot.category}</td>
                        <td class="p-2 font-mono">{plot.items.length}</td>
                        <td class="p-2">
                          <select
                            aria-label={`Plot resolution for ${plot.plotNumber}`}
                            class="rounded border border-slate-300 p-1 text-xs"
                            value={mergePlotResolutions[plot.plotNumber] ?? ''}
                            disabled={busy || operationHeld}
                            onchange={e => setMergePlotResolution(plot.plotNumber, e.currentTarget.value)}
                          >
                            <option value="">Use Field / Default Resolution</option>
                            <option value="keep_target">Keep BECMaster</option>
                            <option value="accept_source">Accept Staging</option>
                          </select>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}

            {#if combinedMergeItems.length > 0}
              <div class="max-h-72 overflow-auto rounded border border-slate-200">
                <table class="w-full text-left text-xs">
                  <thead class="bg-slate-100 text-slate-700">
                    <tr>
                      <th class="p-2">Plot / Table.Field</th>
                      <th class="p-2">Base</th>
                      <th class="p-2">Staging (Source)</th>
                      <th class="p-2">BECMaster (Target)</th>
                      <th class="p-2">Governance</th>
                      <th class="p-2">Resolution</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each combinedMergeItems as item (item.conflictId)}
                      {@const protectedField = isCustodianProtectedConflict(item)}
                      <tr class={`border-t border-slate-100 ${protectedField ? 'bg-amber-50' : ''}`}>
                        <td class="p-2 font-mono">
                          {item.plotNumber} {item.table}.{item.field}
                          {#if item.key && item.key !== item.field}
                            <span class="text-slate-500">[{item.key}]</span>
                          {/if}
                        </td>
                        <td class="p-2 font-mono text-slate-600">{item.baseVal ?? 'NULL'}</td>
                        <td class="p-2 font-mono">{item.sourceVal ?? 'NULL'}</td>
                        <td class="p-2 font-mono">{item.targetVal ?? 'NULL'}</td>
                        <td class="p-2">
                          {#if protectedField}
                            <span class="rounded border border-amber-500 bg-[#FCBA19]/30 px-1.5 py-0.5 text-[11px] font-bold text-[#003366]" data-custodian-protected-badge>
                              Custodian Protected
                            </span>
                          {:else}
                            <span class="text-slate-600">{item.kind}</span>
                          {/if}
                        </td>
                        <td class="p-2">
                          <select
                            aria-label={`Field resolution for ${item.conflictId}`}
                            class="rounded border border-slate-300 bg-white p-1 text-xs"
                            value={mergeFieldResolutions[item.conflictId] ?? ''}
                            disabled={busy || operationHeld}
                            onchange={e => setMergeFieldResolution(item.conflictId, e.currentTarget.value)}
                          >
                            {#if !protectedField}
                              <option value="">Follow Plot / Default ({mergePlotResolutions[item.plotNumber] ?? mergeDefaultResolution})</option>
                            {/if}
                            <option value="keep_target">Keep BECMaster</option>
                            <option value="accept_source">Accept Staging</option>
                          </select>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          {/if}

          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="merge-default-res">Default Conflict Policy</label>
              <select id="merge-default-res" class="w-full rounded border border-slate-300 p-2 text-xs" bind:value={mergeDefaultResolution} disabled={busy || operationHeld}>
                <option value="keep_target">Keep BECMaster (keep_target)</option>
                <option value="accept_source">Accept Staging (accept_source)</option>
              </select>
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="merge-actor">Custodian Reviewer ID (opt)</label>
              <input id="merge-actor" type="text" class="w-full rounded border border-slate-300 p-2 text-xs" bind:value={mergeActor} disabled={busy || operationHeld} placeholder="Provincial Correlation Lead" />
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="merge-reviewer-notes">Reviewer Notes (required to reject)</label>
              <input id="merge-reviewer-notes" type="text" class="w-full rounded border border-slate-300 p-2 text-xs" bind:value={mergeReviewerNotes} disabled={busy || operationHeld} placeholder="Audit / rejection rationale" />
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              disabled={busy || operationHeld || !mergePreview}
              class="rounded border-2 border-[#FCBA19] bg-[#003366] px-4 py-2 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runExecuteStagingMerge}
            >
              Approve &amp; Merge into BECMaster
            </button>
            <button
              type="button"
              disabled={busy || operationHeld || (!mergePreview && !selectedSubmissionDetail)}
              class="rounded border border-red-700 bg-red-700 px-4 py-2 text-xs font-semibold text-white hover:bg-red-800 disabled:opacity-50"
              onclick={() => runRejectSubmission()}
            >
              Reject with Notes
            </button>
          </div>

          {#if mergeExecuteResult}
            <p class="text-xs text-emerald-900">
              Committed Merge <span class="font-mono font-bold">{mergeExecuteResult.mergeTxId}</span>: +{mergeExecuteResult.addedPlots} added, {mergeExecuteResult.updatedPlots} updated, {mergeExecuteResult.auditRowsWritten} audit rows (tail hash <span class="font-mono">{mergeExecuteResult.tailAuditHash.slice(0, 16)}</span>).
            </p>
          {/if}
        </fieldset>
      {/if}

      <!-- Snapshots & Download Activity Ledger -->
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <fieldset class="space-y-2.5 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Cut Dated BECMaster Snapshot</legend>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="snapshot-tag-input">Snapshot Tag</label>
              <input
                id="snapshot-tag-input"
                type="text"
                class="w-full rounded border border-slate-300 p-2 font-mono text-xs"
                bind:value={snapshotTag}
                disabled={busy || operationHeld}
                aria-invalid={!!snapshotTagValidationError}
                placeholder="e.g. BECMaster_2026_11"
              />
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="snapshot-export-format">Optional Archive Export</label>
              <select id="snapshot-export-format" class="w-full rounded border border-slate-300 p-2 text-xs" bind:value={snapshotExportFormat} disabled={busy || operationHeld}>
                <option value="">Cloud Snapshot Only</option>
                <option value="sqlite">SQLite VP08 (.db)</option>
                <option value="rds">R Serialized Dataset (.rds)</option>
                <option value="gpkg">OGC GeoPackage (.gpkg)</option>
              </select>
            </div>
          </div>
          {#if snapshotTagValidationError}
            <p role="alert" class="text-xs text-red-700">{snapshotTagValidationError}</p>
          {/if}
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="snapshot-dest-path">Export Destination Path (opt)</label>
              <input id="snapshot-dest-path" type="text" class="w-full rounded border border-slate-300 p-2 font-mono text-xs" bind:value={snapshotDestinationPath} disabled={busy || operationHeld} />
            </div>
            <div class="space-y-1">
              <label class="block text-xs font-semibold text-slate-700" for="snapshot-zone-filter">Zone Filter (opt)</label>
              <input id="snapshot-zone-filter" type="text" class="w-full rounded border border-slate-300 p-2 font-mono text-xs" bind:value={snapshotZoneFilter} disabled={busy || operationHeld} placeholder="All zones" />
            </div>
          </div>
          <div class="space-y-1">
            <label class="block text-xs font-semibold text-slate-700" for="snapshot-notes-input">Release Notes</label>
            <input id="snapshot-notes-input" type="text" class="w-full rounded border border-slate-300 p-2 text-xs" bind:value={snapshotNotes} disabled={busy || operationHeld} />
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              disabled={busy || operationHeld || !snapshotTag || !!snapshotTagValidationError}
              class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244] disabled:opacity-50"
              onclick={runCreateSnapshot}
            >
              Cut Dated Snapshot
            </button>
            <button
              type="button"
              disabled={busy || operationHeld}
              class="rounded border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-[#003366] hover:bg-slate-100 disabled:opacity-50"
              onclick={refreshSnapshotsAndLedger}
            >
              Refresh Snapshots &amp; Download Log
            </button>
          </div>
          {#if latestSnapshotCreated}
            <p class="text-xs text-slate-700">
              Snapshot <span class="font-mono font-semibold">{latestSnapshotCreated.snapshotTag}</span> manifest SHA-256: <span class="font-mono">{latestSnapshotCreated.manifestSha256.slice(0, 16)}</span>
            </p>
          {/if}
        </fieldset>

        <fieldset class="space-y-2.5 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-4">
          <legend class="px-1 text-sm font-semibold text-[#003366]">Snapshots &amp; Download Activity Ledger</legend>
          {#if snapshots.length > 0}
            <div class="max-h-36 overflow-auto rounded border border-slate-200">
              <table class="w-full text-left text-[11px]">
                <thead class="bg-slate-100 text-slate-700">
                  <tr><th class="p-1.5">Tag</th><th class="p-1.5">Plots</th><th class="p-1.5">Standards</th><th class="p-1.5">Created</th></tr>
                </thead>
                <tbody>
                  {#each snapshots as snap (snap.snapshotTag)}
                    <tr class="border-t border-slate-100">
                      <td class="p-1.5 font-mono font-semibold">{snap.snapshotTag}</td>
                      <td class="p-1.5 font-mono">{snap.plotCount.toLocaleString()}</td>
                      <td class="p-1.5 font-mono">BEC v{snap.becVersion} / Taxa v{snap.taxonomyVersion}</td>
                      <td class="p-1.5">{snap.createdAt}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}

          {#if downloadLog.length > 0}
            <div class="max-h-36 overflow-auto rounded border border-slate-200">
              <table class="w-full text-left text-[11px]">
                <thead class="bg-slate-100 text-slate-700">
                  <tr><th class="p-1.5">#</th><th class="p-1.5">Actor</th><th class="p-1.5">Role</th><th class="p-1.5">Format</th><th class="p-1.5">Plots</th><th class="p-1.5">Timestamp</th></tr>
                </thead>
                <tbody>
                  {#each downloadLog as entry (entry.id)}
                    <tr class="border-t border-slate-100">
                      <td class="p-1.5 font-mono">{entry.id}</td>
                      <td class="p-1.5">{entry.actor}</td>
                      <td class="p-1.5 font-mono">{entry.roleTier}</td>
                      <td class="p-1.5 font-mono">{entry.format}</td>
                      <td class="p-1.5 font-mono">{entry.plotCount}</td>
                      <td class="p-1.5">{entry.downloadedAt}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {:else if snapshots.length === 0}
            <p class="text-xs text-slate-600">Click Refresh Snapshots &amp; Download Log to inspect historical provincial releases and subset checkouts.</p>
          {/if}
        </fieldset>
      </div>
    </section>
  {:else}
    <div class="rounded border border-slate-200 bg-slate-50 px-4 py-3 text-xs text-slate-700">
      Custodian Admin tools (Schema Bootstrap, 2026 Standards Publishing, Staging Inbox 3-Way Merge, and Dated Snapshots) appear when the active session role is switched to <span class="font-semibold text-[#003366]">Custodian Admin</span>.
    </div>
  {/if}
</div>
