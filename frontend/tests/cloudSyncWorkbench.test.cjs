const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions, srcPath } = require('./svelteTestHelpers.cjs');

const read = name => readFileSync(srcPath(name), 'utf8');

const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });

const wailsCalls = [];
const mockWailsRuntime = {
  Call: {
    ByName(methodName, ...args) {
      wailsCalls.push({ methodName, args });
      return Promise.resolve({ ok: true });
    },
  },
};

const projectAdmin = loadTypeScript('projectAdminWorkbench.ts', {
  '@wailsio/runtime': mockWailsRuntime,
  './qualityEditor': quality,
});

const cloudSync = loadTypeScript('cloudSyncWorkbench.ts', {
  '@wailsio/runtime': mockWailsRuntime,
  './qualityEditor': quality,
  './projectAdminWorkbench': projectAdmin,
});

test('all 19 defaultCloudSyncPort methods call exact main.ContextService.* Wails endpoints', async () => {
  wailsCalls.length = 0;
  const p = cloudSync.defaultCloudSyncPort;

  await p.getCloudSyncConfigStatus();
  await p.configureCloudSyncSession({ roleMode: 'contributor' });
  await p.testCloudConnection();
  await p.bootstrapCloudSchemas();
  await p.publishStandardsToCloud('ctx-1', { becVersion: '13', taxonomyVersion: '19' });
  await p.seedBECMasterBaseline({ sourceDbPath: '/data/BECMaster.db', replaceExisting: false });
  await p.pullCloudStandards('ctx-1');
  await p.previewBECMasterSubset({ zones: ['CWH'], plotNumbers: ['00337'] });
  await p.pullBECMasterSubset('ctx-1', {
    filter: { zones: ['CWH'] },
    targetProject: 'CWH_2026',
    activateOnSuccess: true,
  });
  await p.previewStagingSubmission('ctx-1', false);
  await p.submitProjectToStaging('ctx-1', { submittedBy: 'Ecologist' });
  await p.listStagingSubmissions({ status: 'pending', limit: 25 });
  await p.getStagingSubmissionDetail('sub-001');
  await p.previewStagingToMasterMerge({ submissionId: 'sub-001' });
  await p.executeStagingToMasterMerge({
    submissionId: 'sub-001',
    expectedPreviewDigest: 'sha256-preview-1',
  });
  await p.rejectStagingSubmission({ submissionId: 'sub-001', reviewerNotes: 'Missing horizon depths' });
  await p.createBECMasterSnapshot('ctx-1', { snapshotTag: 'BECMaster_2026_11' });
  await p.listBECMasterSnapshots();
  await p.listBECMasterDownloadLog(50);

  assert.equal(wailsCalls.length, 19);
  assert.deepEqual(
    wailsCalls.map(c => c.methodName),
    [
      'main.ContextService.GetCloudSyncConfigStatus',
      'main.ContextService.ConfigureCloudSyncSession',
      'main.ContextService.TestCloudConnection',
      'main.ContextService.BootstrapCloudSchemas',
      'main.ContextService.PublishStandardsToCloud',
      'main.ContextService.SeedBECMasterBaseline',
      'main.ContextService.PullCloudStandards',
      'main.ContextService.PreviewBECMasterSubset',
      'main.ContextService.PullBECMasterSubset',
      'main.ContextService.PreviewStagingSubmission',
      'main.ContextService.SubmitProjectToStaging',
      'main.ContextService.ListStagingSubmissions',
      'main.ContextService.GetStagingSubmissionDetail',
      'main.ContextService.PreviewStagingToMasterMerge',
      'main.ContextService.ExecuteStagingToMasterMerge',
      'main.ContextService.RejectStagingSubmission',
      'main.ContextService.CreateBECMasterSnapshot',
      'main.ContextService.ListBECMasterSnapshots',
      'main.ContextService.ListBECMasterDownloadLog',
    ]
  );

  assert.deepEqual(wailsCalls[4].args, ['ctx-1', { becVersion: '13', taxonomyVersion: '19' }]);
  assert.deepEqual(wailsCalls[6].args, ['ctx-1']);
  assert.deepEqual(wailsCalls[7].args, [{ zones: ['CWH'], plotNumbers: ['00337'] }]);
  assert.deepEqual(wailsCalls[9].args, ['ctx-1', false]);
  assert.deepEqual(wailsCalls[12].args, ['sub-001']);
  assert.deepEqual(wailsCalls[18].args, [50]);
});

test('subset filter, bbox, compliance gate, custodian-protected field, and snapshot tag helpers enforce VPro invariants', () => {
  // Literal leading-zero PlotNumber preservation
  const built = cloudSync.buildSubsetFilterDraft({
    zonesRaw: ' CWH , ICH ',
    subzonesRaw: 'vm1',
    projectIdsRaw: '',
    plotNumbersRaw: '00337, 108050',
    bboxRaw: '-133.2, 52.1, -131.0, 54.4',
    limitRaw: '250',
    allowFullCheckout: false,
  });
  assert.equal(built.error, null);
  assert.deepEqual(structuredClone(built.filter), {
    zones: ['CWH', 'ICH'],
    subzones: ['vm1'],
    plotNumbers: ['00337', '108050'],
    bbox: [-133.2, 52.1, -131.0, 54.4],
    limit: 250,
  });

  // Empty filter blocked unless allowFullCheckout is true
  const emptyBlocked = cloudSync.buildSubsetFilterDraft({
    zonesRaw: '',
    subzonesRaw: '',
    projectIdsRaw: '',
    plotNumbersRaw: '',
    bboxRaw: '',
  });
  assert.match(emptyBlocked.error, /Specify at least one regional filter/);

  const fullAllowed = cloudSync.buildSubsetFilterDraft({
    zonesRaw: '',
    subzonesRaw: '',
    projectIdsRaw: '',
    plotNumbersRaw: '',
    bboxRaw: '',
    allowFullCheckout: true,
  });
  assert.equal(fullAllowed.error, null);
  assert.deepEqual(structuredClone(fullAllowed.filter), { allowFullCheckout: true });

  // PlotNumber > 7 UTF-16 rejected
  const overlongPlot = cloudSync.buildSubsetFilterDraft({
    zonesRaw: '',
    subzonesRaw: '',
    projectIdsRaw: '',
    plotNumbersRaw: '12345678',
    bboxRaw: '',
  });
  assert.match(overlongPlot.error, /7 UTF-16/);

  // Invalid BBox rejected
  assert.match(cloudSync.parseBboxInput('-120, 55, -125, 56').error, /minLon < maxLon/);
  assert.match(cloudSync.parseBboxInput('-125, 58, -120, 55').error, /minLat < maxLat/);

  // Staging compliance gate
  assert.equal(cloudSync.canUploadStagingSubmission(null, 'custodian', true), false);
  assert.equal(cloudSync.canUploadStagingSubmission({ compliancePassed: false }, 'contributor', true), false);
  assert.equal(cloudSync.canUploadStagingSubmission({ compliancePassed: false }, 'custodian', false), false);
  assert.equal(cloudSync.canUploadStagingSubmission({ compliancePassed: false }, 'custodian', true), true);
  assert.equal(cloudSync.canUploadStagingSubmission({ compliancePassed: true }, 'contributor', false), true);

  // Custodian-protected BEC fields
  for (const field of ['BECSiteUnit', 'BEC_Use', 'GIS_BGC']) {
    assert.equal(cloudSync.isCustodianProtectedConflict({ field, custodianProtected: false }), true);
  }
  assert.equal(cloudSync.isCustodianProtectedConflict({ field: 'SlopeGradient', custodianProtected: false }), false);

  // Snapshot tag validator
  assert.equal(cloudSync.validateSnapshotTagInput('BECMaster_2026.11-rc1'), null);
  assert.match(cloudSync.validateSnapshotTagInput('bad tag/slash'), /ASCII letters/);
});

test('offline SSR render of CloudSyncWorkbench shows BC Gov bar, Contributor controls, password inputs, and zero leaked secrets', () => {
  const src = read('CloudSyncWorkbench.svelte');
  assert.doesNotMatch(src, /[\u{1F300}-\u{1FAFF}]/u, 'CloudSyncWorkbench must contain zero emojis');
  const compiledClient = compile(src, { filename: 'CloudSyncWorkbench.svelte', generate: 'client' });
  assert.deepEqual(compiledClient.warnings, []);

  let networkCalledDuringSSR = false;
  const ssrPort = new Proxy(
    {},
    {
      get() {
        return () => {
          networkCalledDuringSSR = true;
          throw new Error('SSR must never invoke CloudSyncPort RPCs');
        };
      },
    }
  );

  const Component = serverComponent(src, 'CloudSyncWorkbench.svelte', {
    './cloudSyncWorkbench': cloudSync,
  });
  const html = render(Component, {
    props: {
      contextId: 'ctx-offline-ssr',
      activeProject: 'HaidaGwaii_2026',
      activeSu: 'HGW_SU',
      port: ssrPort,
    },
  }).body;

  assert.equal(networkCalledDuringSSR, false);
  assert.match(html, /#003366/);
  assert.match(html, /#FCBA19/);
  assert.match(html, /Offline \/ Unprobed/);
  assert.match(html, /Contributor/);
  assert.match(html, /Checkout Regional Subset from BECMaster/);
  assert.match(html, /Submit Active Project to BECMaster Staging/);
  assert.match(html, /HaidaGwaii_2026/);
  assert.doesNotMatch(html, /data-custodian-panel/);

  // Both password inputs must use type="password" and autocomplete="off"
  const passwordInputs = [...html.matchAll(/<input\b[^>]*id="cloud-override-(?:admin-)?password"[^>]*>/g)].map(m => m[0]);
  assert.equal(passwordInputs.length, 2);
  for (const tag of passwordInputs) {
    assert.match(tag, /type="password"/);
    assert.match(tag, /autocomplete="off"/);
  }
  assert.doesNotMatch(html, /postgres:\/\/|postgresql:\/\/|PGPASSWORD/i);
});

test('CloudSyncWorkbench client rune actions are offline-first on mount, detach proxies, clear passwords, enforce compliance, pass merge digest, and honor operationHeld + onDestroy', async () => {
  const client = await import('svelte/internal/client');
  let script = read('CloudSyncWorkbench.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace(
    'oncontextswitched?: (state: unknown) => void;',
    'oncontextswitched?: (state: unknown) => void; inspect?: (v: any) => void;'
  );
  script = script.replace('oncontextswitched,', 'oncontextswitched, inspect,');
  script += `\n$effect(() => inspect?.({
    getState: () => ({
      busy,
      error,
      statusMessage,
      activeRoleMode,
      overridePassword,
      overrideAdminPassword,
      configStatus,
      connectionReport,
      subsetPreview,
      pullSubsetResult,
      stagingPreflight,
      submitStagingResult,
      submissions,
      selectedSubmissionDetail,
      mergePreview,
      mergeExecuteResult,
      snapshots,
      downloadLog,
    }),
    setSessionInputs(vals) {
      if (vals.roleMode !== undefined) selectedRoleMode = vals.roleMode;
      if (vals.host !== undefined) overrideHost = vals.host;
      if (vals.port !== undefined) overridePort = vals.port;
      if (vals.user !== undefined) overrideUser = vals.user;
      if (vals.password !== undefined) overridePassword = vals.password;
      if (vals.adminUser !== undefined) overrideAdminUser = vals.adminUser;
      if (vals.adminPassword !== undefined) overrideAdminPassword = vals.adminPassword;
    },
    applySessionConfig,
    switchRoleAndConnect,
    syncProvincialStandards,
    setSubsetInputs(vals) {
      if (vals.zones !== undefined) subsetZonesRaw = vals.zones;
      if (vals.plots !== undefined) subsetPlotNumbersRaw = vals.plots;
      if (vals.targetProject !== undefined) subsetTargetProject = vals.targetProject;
      if (vals.activate !== undefined) subsetActivateOnSuccess = vals.activate;
    },
    runPreviewSubset,
    runPullSubset,
    runStagingPreflight,
    setStagingInputs(vals) {
      if (vals.submittedBy !== undefined) stagingSubmittedBy = vals.submittedBy;
      if (vals.notes !== undefined) stagingSubmitterNotes = vals.notes;
      if (vals.custodianOverride !== undefined) stagingCustodianOverride = vals.custodianOverride;
    },
    runSubmitToStaging,
    refreshSubmissionsList,
    runBootstrapSchemas,
    runPublishStandards,
    setSeedPath(p) { seedSourceDbPath = p; },
    runSeedBaseline,
    inspectSubmission,
    runPreviewStagingMerge,
    setMergePlotResolution,
    setMergeFieldResolution,
    setMergeReviewerNotes(n) { mergeReviewerNotes = n; },
    runExecuteStagingMerge,
    runRejectSubmission,
    setSnapshotTag(t, fmt) { snapshotTag = t; if (fmt !== undefined) snapshotExportFormat = fmt; },
    runCreateSnapshot,
    refreshSnapshotsAndLedger,
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);

  const destroyCallbacks = [];
  const mountCallbacks = [];
  const cjsMod = { exports: {} };
  vm.runInNewContext(
    ts.transpileModule(compiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
    {
      module: cjsMod,
      exports: cjsMod.exports,
      structuredClone,
      queueMicrotask,
      Promise,
      Set,
      Number,
      String,
      Object,
      require(name) {
        if (name === 'svelte') {
          return {
            onMount: fn => mountCallbacks.push(fn),
            onDestroy: fn => destroyCallbacks.push(fn),
          };
        }
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name === './cloudSyncWorkbench') return cloudSync;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );

  const dispatches = [];
  const busyTransitions = [];
  const switchedStates = [];
  let preflightPasses = false;
  let heldConnectionResolver = null;

  function assertClonableArgs(method, args) {
    for (const arg of args) {
      assert.doesNotThrow(() => structuredClone(arg), `${method} received un-detached Svelte proxy`);
    }
    dispatches.push([method, ...args]);
  }

  const mockPort = {
    async getCloudSyncConfigStatus() {
      assertClonableArgs('getCloudSyncConfigStatus', []);
      return {
        configured: true,
        configSource: 'becmaster.env',
        envFileDetected: true,
        activeRoleMode: 'contributor',
        hasContributorCreds: true,
        hasCustodianCreds: true,
        sessionOverride: false,
        port: 5432,
        database: 'becmaster',
        sslMode: 'require',
      };
    },
    async configureCloudSyncSession(req) {
      assertClonableArgs('configureCloudSyncSession', [req]);
      return {
        configured: true,
        configSource: req.clearOverrides ? 'becmaster.env' : 'session_override',
        envFileDetected: true,
        activeRoleMode: req.roleMode,
        hasContributorCreds: true,
        hasCustodianCreds: true,
        sessionOverride: !req.clearOverrides,
        port: req.port ?? 5432,
        database: req.database ?? 'becmaster',
        sslMode: req.sslMode ?? 'require',
      };
    },
    async testCloudConnection() {
      assertClonableArgs('testCloudConnection', []);
      if (heldConnectionResolver) {
        return new Promise(resolve => {
          heldConnectionResolver = resolve;
        });
      }
      return {
        connected: true,
        activeRoleMode: 'custodian',
        configSource: 'session_override',
        latencyMs: 14,
        serverVersion: 'PostgreSQL 17.2',
        postgisVersion: '3.5',
        database: 'becmaster',
        sslMode: 'require',
        schemasBootstrapped: true,
        canReadMaster: true,
        canSubmitStaging: true,
        canMergeMaster: true,
        canBootstrapSchema: true,
        becmasterPlotCount: 71567,
        becmasterVegCount: 1651842,
        pendingSubmissionCount: 2,
        snapshotCount: 3,
        latestSnapshotTag: 'BECMaster_2026_05',
        activeStandardsRelease: '2026.05',
        activeBecVersion: '13',
        activeTaxonomyVersion: '19',
        checkedAt: '2026-11-14T14:00:00Z',
      };
    },
    async bootstrapCloudSchemas() {
      assertClonableArgs('bootstrapCloudSchemas', []);
      return {
        schemaVersion: 1,
        appliedMigrations: ['001_init'],
        schemasVerified: ['becmaster', 'staging', 'standards', 'governance'],
        postgisEnabled: true,
        pgcryptoEnabled: true,
        roleGrantsApplied: true,
        contributorRoleSet: true,
        bootstrappedAt: '2026-11-14T14:01:00Z',
      };
    },
    async publishStandardsToCloud(cid, req) {
      assertClonableArgs('publishStandardsToCloud', [cid, req]);
      return {
        releaseId: req.releaseId ?? 'BECv13_2026',
        becVersion: '13',
        taxonomyVersion: '19',
        vlistsSha256: 'aaaa1111bbbb2222cccc3333',
        becDbSha256: 'dddd4444eeee5555ffff6666',
        vmetaDataSha256: '777788889999000011112222',
        tableCounts: { USysAllSpecs: 7450 },
        elapsedMs: 42,
        publishedAt: '2026-11-14T14:02:00Z',
      };
    },
    async seedBECMasterBaseline(req) {
      assertClonableArgs('seedBECMasterBaseline', [req]);
      return {
        sourceProject: 'BECMaster_fixing',
        sourceSha256: '1234567890abcdef',
        snapshotTag: 'baseline_2026',
        plotCount: 71567,
        adminCount: 71567,
        vegCount: 1651842,
        humusCount: 50000,
        mineralCount: 90000,
        otherCount: 1200,
        metadataCount: 45,
        auditCount: 597277,
        childIdentityRows: 1800000,
        tailAuditHash: 'fedcba0987654321',
        elapsedMs: 950,
        seededAt: '2026-11-14T14:03:00Z',
        tableCounts: { Env: 71567 },
      };
    },
    async pullCloudStandards(cid) {
      assertClonableArgs('pullCloudStandards', [cid]);
      return {
        updated: true,
        alreadyCurrent: false,
        releaseId: '2026.05',
        becVersion: '13',
        taxonomyVersion: '19',
        localSha256: 'oldsha',
        cloudSha256: 'newsha',
        state: { contextId: 'ctx-std-updated', activeProject: 'Sample' },
      };
    },
    async previewBECMasterSubset(filter) {
      assertClonableArgs('previewBECMasterSubset', [filter]);
      return {
        matchingPlots: 18,
        matchingVeg: 412,
        samplePlots: ['00337', '108050'],
        zoneBreakdown: { CWH: 18 },
      };
    },
    async pullBECMasterSubset(cid, req) {
      assertClonableArgs('pullBECMasterSubset', [cid, req]);
      return {
        targetProject: req.targetProject,
        destinationPath: `/projects/${req.targetProject}.db`,
        destinationSha256: 'abcdef1234567890abcdef1234567890',
        plotCount: 18,
        vegCount: 412,
        humusCount: 24,
        mineralCount: 39,
        otherCount: 0,
        metadataCount: 1,
        downloadLogId: 7,
        activated: req.activateOnSuccess,
        state: { contextId: 'ctx-subset-activated', activeProject: req.targetProject },
      };
    },
    async previewStagingSubmission(cid, useActiveSu) {
      assertClonableArgs('previewStagingSubmission', [cid, useActiveSu]);
      return {
        project: 'Sample',
        projectId: 'SMP',
        plotCount: 12,
        vegRowCount: 240,
        sourceDbSha256: 'sha-stage-1',
        compliancePassed: preflightPasses,
        blockingErrorCount: preflightPasses ? 0 : 3,
        complianceSummary: { envCodeErrors: preflightPasses ? 0 : 3 },
        complianceReport: {},
        userSpeciesManifest: {
          schemaVersion: '1.0',
          exportedAt: '2026-11-14T14:04:00Z',
          sourceProject: 'Sample',
          rowCount: 1,
          payloadSha256: 'spp-sha',
          species: [{ code: 'CAREX_SP1', scientificName: 'Carex sp. nov.', lifeForm: 6, englishName: 'Sedge' }],
        },
        lumpRowCount: 2,
        suRowCount: 12,
      };
    },
    async submitProjectToStaging(cid, req) {
      assertClonableArgs('submitProjectToStaging', [cid, req]);
      return {
        submissionId: 'sub-900',
        projectName: 'Sample',
        projectId: 'SMP',
        submittedBy: req.submittedBy ?? 'anon',
        dbRole: 'vpro_contributor',
        status: 'pending',
        plotCount: 12,
        vegRowCount: 240,
        userSpeciesCount: 1,
        sourceDbSha256: 'sha-stage-1',
        compliancePassed: preflightPasses,
        submittedAt: '2026-11-14T14:05:00Z',
      };
    },
    async listStagingSubmissions(filter) {
      assertClonableArgs('listStagingSubmissions', [filter]);
      return [
        {
          submissionId: 'sub-900',
          projectName: 'Sample',
          submittedBy: 'Skeena Crew',
          dbRole: 'vpro_contributor',
          submittedAt: '2026-11-14T14:05:00Z',
          status: 'pending',
          plotCount: 12,
          vegRowCount: 240,
          sourceDbSha256: 'sha-stage-1',
          compliancePassed: true,
          userSpeciesCount: 1,
          lumpRowCount: 2,
          suRowCount: 12,
        },
      ];
    },
    async getStagingSubmissionDetail(submissionId) {
      assertClonableArgs('getStagingSubmissionDetail', [submissionId]);
      return {
        summary: {
          submissionId,
          projectName: 'Sample',
          submittedBy: 'Skeena Crew',
          dbRole: 'vpro_contributor',
          submittedAt: '2026-11-14T14:05:00Z',
          status: 'pending',
          plotCount: 12,
          vegRowCount: 240,
          sourceDbSha256: 'sha-stage-1',
          compliancePassed: true,
          userSpeciesCount: 1,
          lumpRowCount: 2,
          suRowCount: 12,
        },
        complianceReport: {},
        userSpeciesManifest: {
          schemaVersion: '1.0',
          exportedAt: '2026-11-14T14:04:00Z',
          sourceProject: 'Sample',
          rowCount: 1,
          payloadSha256: 'spp-sha',
          species: [{ code: 'CAREX_SP1', scientificName: 'Carex sp. nov.', lifeForm: 6, englishName: null }],
        },
        provisionalSpeciesDiffs: [
          { code: 'CAREX_SP1', scientificName: 'Carex sp. nov.', lifeForm: 6, status: 'unlisted' },
        ],
        lumpRows: [{ lumpCode: 'SALIX_SPP', sppCode: 'SALIBEB', use: true }],
        suRows: [{ plotNumber: '00337', siteUnit: 'CWHvm1/01' }],
      };
    },
    async previewStagingToMasterMerge(req) {
      assertClonableArgs('previewStagingToMasterMerge', [req]);
      return {
        submissionId: req.submissionId,
        sourceProject: 'Sample',
        targetProject: 'BECMaster',
        previewDigest: 'digest-3way-xyz',
        addedCount: 2,
        identicalCount: 8,
        fastForwardCount: 1,
        conflictCount: 1,
        targetOnlyCount: 71555,
        elapsedMs: 38,
        plots: [
          {
            plotNumber: '00337',
            category: 'Conflict',
            items: [],
          },
        ],
        conflicts: [
          {
            conflictId: '00337::Env::MoistureRegime',
            plotNumber: '00337',
            table: 'Env',
            key: 'MoistureRegime',
            field: 'MoistureRegime',
            baseVal: '4',
            sourceVal: '5',
            targetVal: '4',
            kind: 'Conflict',
            defaultResolution: 'keep_target',
            custodianProtected: false,
          },
        ],
        custodianProtectedDiffs: [
          {
            conflictId: '00337::Admin::BECSiteUnit',
            plotNumber: '00337',
            table: 'Admin',
            key: 'BECSiteUnit',
            field: 'BECSiteUnit',
            baseVal: 'CWHvm1/01',
            sourceVal: 'CWHvm1/05',
            targetVal: 'CWHvm1/01',
            kind: 'CustodianProtected',
            defaultResolution: 'keep_target',
            custodianProtected: true,
          },
        ],
      };
    },
    async executeStagingToMasterMerge(req) {
      assertClonableArgs('executeStagingToMasterMerge', [req]);
      return {
        mergeTxId: 'tx-merge-42',
        submissionId: req.submissionId,
        sourceProject: 'Sample',
        addedPlots: 2,
        updatedPlots: 2,
        skippedPlots: 8,
        allocatedChildIds: [9001, 9002],
        reservedDeletedIds: [450],
        auditRowsWritten: 17,
        tailAuditHash: '99887766554433221100',
        mergedAt: '2026-11-14T14:10:00Z',
      };
    },
    async rejectStagingSubmission(req) {
      assertClonableArgs('rejectStagingSubmission', [req]);
      return {
        submissionId: req.submissionId,
        projectName: 'Sample',
        submittedBy: 'Skeena Crew',
        dbRole: 'vpro_contributor',
        submittedAt: '2026-11-14T14:05:00Z',
        status: 'rejected',
        plotCount: 12,
        vegRowCount: 240,
        sourceDbSha256: 'sha-stage-1',
        compliancePassed: true,
        userSpeciesCount: 1,
        lumpRowCount: 2,
        suRowCount: 12,
        reviewerNotes: req.reviewerNotes,
      };
    },
    async createBECMasterSnapshot(cid, req) {
      assertClonableArgs('createBECMasterSnapshot', [cid, req]);
      return {
        snapshotTag: req.snapshotTag,
        createdAt: '2026-11-14T14:12:00Z',
        createdBy: 'Custodian',
        plotCount: 71569,
        vegCount: 1652082,
        becVersion: '13',
        taxonomyVersion: '19',
        manifestSha256: 'snapsha256abcdef123456',
        exportedPath: req.exportFormat ? `/exports/${req.snapshotTag}.${req.exportFormat}` : undefined,
      };
    },
    async listBECMasterSnapshots() {
      assertClonableArgs('listBECMasterSnapshots', []);
      return [
        {
          snapshotTag: 'BECMaster_2026_11',
          createdAt: '2026-11-14T14:12:00Z',
          createdBy: 'Custodian',
          plotCount: 71569,
          vegCount: 1652082,
          becVersion: '13',
          taxonomyVersion: '19',
          manifestSha256: 'snapsha256abcdef123456',
        },
      ];
    },
    async listBECMasterDownloadLog(limit) {
      assertClonableArgs('listBECMasterDownloadLog', [limit]);
      return [
        {
          id: 7,
          downloadedAt: '2026-11-14T14:06:00Z',
          actor: 'vpro_contributor',
          roleTier: 'contributor',
          filterJson: { zones: ['CWH'] },
          format: 'sqlite',
          plotCount: 18,
        },
      ];
    },
  };

  let api;
  const props = client.proxy({
    contextId: 'ctx-live',
    activeProject: 'Sample',
    activeSu: 'Sample',
    operationHeld: false,
    port: mockPort,
    onBusyChange: b => busyTransitions.push(b),
    oncontextswitched: st => switchedStates.push(st),
    inspect: v => {
      api = v;
    },
  });

  const dispose = client.effect_root(() => {
    cjsMod.exports.default(null, props);
  });
  for (const fn of mountCallbacks) fn();
  client.flush();
  await Promise.resolve();
  client.flush();

  try {
    // 1. Offline-first mount: ONLY getCloudSyncConfigStatus called on mount!
    assert.deepEqual(
      dispatches.map(d => d[0]),
      ['getCloudSyncConfigStatus'],
      'onMount must call ONLY getCloudSyncConfigStatus and zero live network endpoints'
    );
    assert.equal(api.getState().configStatus.envFileDetected, true);

    // 2. Session credential override clears password fields immediately and never leaks secrets
    const testSecretA = 'TempContribPass#91';
    const testSecretB = 'TempAdminPass#92';
    api.setSessionInputs({
      roleMode: 'contributor',
      host: 'db.internal.example',
      port: '5432',
      user: 'ecologist',
      password: testSecretA,
      adminUser: 'custodian',
      adminPassword: testSecretB,
    });
    client.flush();
    await api.applySessionConfig(false);
    client.flush();
    assert.equal(api.getState().overridePassword, '', 'contributor password must clear immediately');
    assert.equal(api.getState().overrideAdminPassword, '', 'admin password must clear immediately');
    assert.equal(String(api.getState().statusMessage).includes(testSecretA), false);
    assert.equal(String(api.getState().statusMessage).includes(testSecretB), false);

    // 3. operationHeld guard blocks actions before invoking Wails
    const countBeforeHeld = dispatches.length;
    props.operationHeld = true;
    client.flush();
    await api.syncProvincialStandards();
    client.flush();
    assert.equal(dispatches.length, countBeforeHeld, 'operationHeld must block network RPCs');
    assert.match(api.getState().error, /Wait for the current operation/);
    props.operationHeld = false;
    client.flush();

    // 4. Sync Provincial Standards & Regional Subset Checkout preserve '00337' and trigger oncontextswitched
    await api.syncProvincialStandards();
    client.flush();
    assert.equal(switchedStates.length, 1);
    assert.equal(switchedStates[0].contextId, 'ctx-std-updated');

    api.setSubsetInputs({
      zones: 'CWH',
      plots: '00337, 108050',
      targetProject: 'Skeena_Sub_2026',
      activate: true,
    });
    client.flush();
    await api.runPreviewSubset();
    client.flush();
    const previewSubsetCall = dispatches.find(d => d[0] === 'previewBECMasterSubset');
    assert.deepEqual(previewSubsetCall[1].plotNumbers, ['00337', '108050']);

    await api.runPullSubset();
    client.flush();
    assert.equal(switchedStates.length, 2);
    assert.equal(switchedStates[1].activeProject, 'Skeena_Sub_2026');

    // 5. Staging Pre-Flight compliance gate blocks non-compliant Contributor upload
    preflightPasses = false;
    await api.runSubmitToStaging();
    client.flush();
    assert.match(api.getState().error, /Run Pre-Flight Compliance/);
    assert.equal(dispatches.some(d => d[0] === 'submitProjectToStaging'), false);

    await api.runStagingPreflight();
    client.flush();
    assert.equal(api.getState().stagingPreflight.compliancePassed, false);
    await api.runSubmitToStaging();
    client.flush();
    assert.match(api.getState().error, /Upload blocked/);
    assert.equal(dispatches.some(d => d[0] === 'submitProjectToStaging'), false);

    // Compliance passes -> Contributor can upload staging bundle
    preflightPasses = true;
    await api.runStagingPreflight();
    client.flush();
    api.setStagingInputs({ submittedBy: 'Skeena Crew', notes: '2026 plots validated' });
    client.flush();
    await api.runSubmitToStaging();
    client.flush();
    assert.equal(api.getState().submitStagingResult.submissionId, 'sub-900');

    // 6. Switch to Custodian Admin, run maintenance, inspect detail, 3-way merge with digest + resolutions, reject, snapshots
    api.setSessionInputs({ roleMode: 'custodian' });
    client.flush();
    await api.switchRoleAndConnect();
    client.flush();
    assert.equal(api.getState().activeRoleMode, 'custodian');
    assert.equal(api.getState().connectionReport.becmasterPlotCount, 71567);

    await api.runBootstrapSchemas();
    await api.runPublishStandards();
    api.setSeedPath('/data/BECMaster_fixing.db');
    client.flush();
    await api.runSeedBaseline();
    await api.refreshSubmissionsList();
    await api.inspectSubmission('sub-900');
    client.flush();
    assert.equal(api.getState().selectedSubmissionDetail.provisionalSpeciesDiffs[0].code, 'CAREX_SP1');

    await api.runPreviewStagingMerge('sub-900');
    client.flush();
    assert.equal(api.getState().mergePreview.previewDigest, 'digest-3way-xyz');
    // Custodian-protected field defaults to keep_target; override MoistureRegime to accept_source
    api.setMergePlotResolution('00337', 'keep_target');
    api.setMergeFieldResolution('00337::Env::MoistureRegime', 'accept_source');
    api.setMergeReviewerNotes('Approved MoistureRegime update; kept BECSiteUnit');
    client.flush();
    await api.runExecuteStagingMerge();
    client.flush();

    const mergeExecCall = dispatches.find(d => d[0] === 'executeStagingToMasterMerge');
    assert.ok(mergeExecCall);
    assert.equal(mergeExecCall[1].expectedPreviewDigest, 'digest-3way-xyz');
    assert.equal(mergeExecCall[1].plotResolutions['00337'], 'keep_target');
    assert.equal(mergeExecCall[1].fieldResolutions['00337::Env::MoistureRegime'], 'accept_source');
    assert.equal(mergeExecCall[1].fieldResolutions['00337::Admin::BECSiteUnit'], 'keep_target');
    assert.equal(api.getState().mergeExecuteResult.mergeTxId, 'tx-merge-42');

    // Reject requires reviewer notes
    api.setMergeReviewerNotes('');
    client.flush();
    await api.runRejectSubmission('sub-900');
    client.flush();
    assert.match(api.getState().error, /Reviewer notes are required/);
    api.setMergeReviewerNotes('Duplicate plot numbers require field check');
    client.flush();
    await api.runRejectSubmission('sub-900');
    client.flush();
    assert.equal(api.getState().submissions[0].status, 'rejected');

    // Snapshots & Download Activity Log
    api.setSnapshotTag('BECMaster_2026_11', 'gpkg');
    client.flush();
    await api.runCreateSnapshot();
    await api.refreshSnapshotsAndLedger();
    client.flush();
    assert.equal(api.getState().snapshots.length, 1);
    assert.equal(api.getState().downloadLog.length, 1);

    // 7. onDestroy cancels in-flight async work and releases onBusyChange(false)
    heldConnectionResolver = true;
    const inflightPromise = api.switchRoleAndConnect();
    client.flush();
    assert.equal(busyTransitions.at(-1), true);
    for (const fn of destroyCallbacks) fn();
    assert.equal(busyTransitions.at(-1), false, 'onDestroy must release busy barrier immediately');
    const prevPlotCount = api.getState().connectionReport.becmasterPlotCount;
    heldConnectionResolver({
      connected: true,
      activeRoleMode: 'contributor',
      configSource: 'late',
      latencyMs: 999,
      serverVersion: 'late',
      database: 'becmaster',
      sslMode: 'require',
      schemasBootstrapped: true,
      canReadMaster: true,
      canSubmitStaging: true,
      canMergeMaster: false,
      canBootstrapSchema: false,
      becmasterPlotCount: 1,
      becmasterVegCount: 1,
      pendingSubmissionCount: 0,
      snapshotCount: 0,
      checkedAt: 'late',
    });
    await inflightPromise;
    client.flush();
    assert.equal(api.getState().connectionReport.becmasterPlotCount, prevPlotCount, 'post-destroy resolve must not mutate state');
  } finally {
    dispose();
  }
});

test('ProjectAdminWorkbench, Navigation, and App wire Cloud BECMaster Sync cleanly without new static script imports', () => {
  const adminSrc = read('ProjectAdminWorkbench.svelte');
  const scriptBlock = adminSrc.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const importCount = (scriptBlock.match(/\bimport\s+/g) ?? []).length;
  assert.equal(importCount, 2, 'ProjectAdminWorkbench <script> must keep only its 2 existing imports');
  assert.match(adminSrc, /data-workbench-tab=\{id\}/);
  assert.match(adminSrc, /\['cloudsync',\s*'Cloud BECMaster Sync'\]/);
  assert.match(adminSrc, /#await import\('\.\/CloudSyncWorkbench\.svelte'\)/);

  const navSrc = read('Navigation.svelte');
  const icons = new Proxy({}, { get: () => () => {} });
  const Nav = serverComponent(navSrc, 'Navigation.svelte', { '@lucide/svelte': icons });
  const navHtml = render(Nav, { props: { view: 'home', onnavigate: () => {} } }).body;
  const syncButtons = [...navHtml.matchAll(/<button\b([^>]*)>[\s\S]*?<span>Cloud BECMaster Sync<\/span>\s*<\/button>/g)];
  assert.equal(syncButtons.length, 2, 'Cloud BECMaster Sync must appear in both Forms and Interchange menus');
  for (const btn of syncButtons) {
    assert.equal(/\bdisabled(?:\s|=|$)/.test(btn[1]), false, 'Cloud BECMaster Sync menu buttons must be enabled');
  }

  // Verify callback vs fallback dispatch in Navigation.svelte
  const calls = [];
  const fallbackCtx = componentFunctions('Navigation.svelte', ['itemEnabled', 'activate', 'closeMenu', 'navigate'], {
    openMenu: 'Interchange',
    navigation: { querySelector: () => ({ querySelector: () => ({ focus: () => calls.push('focus') }) }) },
    onnavigate: v => calls.push(`nav:${v}`),
  });
  fallbackCtx.actions.activate({ action: 'cloud-sync' });
  assert.deepEqual(calls, ['focus', 'nav:project-admin']);

  calls.length = 0;
  const cbCtx = componentFunctions('Navigation.svelte', ['itemEnabled', 'activate', 'closeMenu', 'navigate'], {
    openMenu: 'Forms',
    navigation: { querySelector: () => ({ querySelector: () => ({ focus: () => calls.push('focus') }) }) },
    onnavigate: v => calls.push(`nav:${v}`),
    onopencloudsync: () => calls.push('open-cloud-sync'),
  });
  cbCtx.actions.activate({ action: 'cloud-sync' });
  assert.deepEqual(calls, ['focus', 'open-cloud-sync']);

  const appSrc = read('App.svelte');
  assert.match(appSrc, /onopencloudsync=\{\(\)\s*=>\s*\{\s*void requestTransition\(/);
  assert.match(appSrc, /projectAdminTab = 'cloudsync'/);
  assert.match(appSrc, /initialTab=\{projectAdminTab\}/);
});
