const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

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

const workbench = loadTypeScript('projectAdminWorkbench.ts', {
  '@wailsio/runtime': mockWailsRuntime,
  './qualityEditor': quality,
});

const read = name => readFileSync(path.join(__dirname, name), 'utf8');

test('project naming and splinter draft validators enforce VP08 rules, reserved names, and literal leading-zero plots', () => {
  assert.equal(workbench.validateProjectNameInput('Kootenay_2026'), null);
  assert.match(workbench.validateProjectNameInput(''), /required/);
  assert.match(workbench.validateProjectNameInput(' LeadSpace'), /spaces/);
  assert.match(workbench.validateProjectNameInput('9StartsWithDigit'), /start with a letter/);
  assert.match(workbench.validateProjectNameInput('Sample'), /reserved/);
  assert.match(workbench.validateProjectNameInput('sample'), /reserved/);
  assert.match(workbench.validateProjectNameInput('None'), /reserved/);
  assert.match(workbench.validateProjectNameInput('BECMaster_2026'), /reserved/);
  assert.match(workbench.validateProjectNameInput('A'.repeat(32)), /max 31/);

  assert.match(
    workbench.validateNewProjectDraft({
      projectName: 'ValidProj',
      projectId: 'X'.repeat(21),
      activateOnSuccess: false,
    }),
    /20 UTF-16/
  );

  const tokens = workbench.parseLiteralTokenList(' 00337, 108050 \n 00337 , P001 ');
  assert.deepEqual(structuredClone(tokens), ['00337', '108050', 'P001']);

  assert.match(
    workbench.validateSplinterDraft(
      { splinterProject: 'SubProj', useActiveSu: true, activateOnSuccess: false },
      'None'
    ),
    /Select an active Site Unit/
  );
  assert.equal(
    workbench.validateSplinterDraft(
      { splinterProject: 'SubProj', useActiveSu: false, plotNumbers: ['00337'], activateOnSuccess: false },
      'None'
    ),
    null
  );
  assert.match(
    workbench.validateSplinterDraft(
      { splinterProject: 'SubProj', useActiveSu: false, plotNumbers: ['12345678'], activateOnSuccess: false },
      'None'
    ),
    /7 UTF-16/
  );
});

test('defaultProjectAdminPort dispatches exact Wails Call.ByName endpoints on ContextService and ValidationService', async () => {
  wailsCalls.length = 0;
  await workbench.defaultProjectAdminPort.createNewProject('ctx-1', { projectName: 'P1', activateOnSuccess: false });
  await workbench.defaultProjectAdminPort.saveProjectAs('ctx-1', { newProjectName: 'P2', activateOnSuccess: false });
  await workbench.defaultProjectAdminPort.exportSplinterProject('ctx-1', { splinterProject: 'P3', useActiveSu: true, activateOnSuccess: false });
  await workbench.defaultProjectAdminPort.compareProjects('ctx-1', { targetPath: '/db/Target.db' });
  await workbench.defaultProjectAdminPort.runComplianceValidation('ctx-1', { useActiveSu: false });
  await workbench.defaultProjectAdminPort.quarantineOrphanRecords('ctx-1', {});
  await workbench.defaultProjectAdminPort.previewOptimizeVeg('ctx-1', {});
  await workbench.defaultProjectAdminPort.applyOptimizeVeg('ctx-1', { expectedDigest: 'abc' });
  await workbench.defaultProjectAdminPort.previewThreeWayMerge('ctx-1', { sourcePath: '/db/Stage.db' });
  await workbench.defaultProjectAdminPort.executeThreeWayMerge('ctx-1', { sourcePath: '/db/Stage.db' });

  assert.deepEqual(
    wailsCalls.map(c => c.methodName),
    [
      'main.ContextService.CreateNewProject',
      'main.ContextService.SaveProjectAs',
      'main.ContextService.ExportSplinterProject',
      'main.ContextService.CompareProjects',
      'main.ValidationService.RunComplianceValidation',
      'main.ValidationService.QuarantineOrphanRecords',
      'main.ValidationService.PreviewOptimizeVeg',
      'main.ValidationService.ApplyOptimizeVeg',
      'main.ValidationService.PreviewThreeWayMerge',
      'main.ValidationService.ExecuteThreeWayMerge',
    ]
  );
});

test('ProjectAdminWorkbench SSR renders BC Gov header, active context, and lifecycle controls cleanly', () => {
  const Component = serverComponent(read('ProjectAdminWorkbench.svelte'), 'ProjectAdminWorkbench.svelte', {
    './projectAdminWorkbench': workbench,
  });
  const html = render(Component, {
    props: { contextId: 'ctx-ssr', activeProject: 'Sample', activeSu: 'Sample' },
  }).body;
  assert.match(html, /#003366/);
  assert.match(html, /#FCBA19/);
  assert.match(html, /Create New VP08 Project/);
  assert.match(html, /Save Active Project As/);
  assert.match(html, /Export Splinter Project/);
});

test('ProjectAdminWorkbench client actions detach Svelte 5 proxies and enforce quarantine, OptimizeVeg digest, and 3-way merge resolutions', async () => {
  const client = await import('svelte/internal/client');
  let script = read('ProjectAdminWorkbench.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('oncontextswitched,', 'oncontextswitched, inspect,');
  script += `\n$effect(() => inspect({
    setNewProj(name, preset, activate) { newProjName = name; newProjPreset = preset; newProjActivate = activate; },
    submitCreateNewProject,
    runCompliance,
    setConfirmQuarantine(v) { confirmQuarantine = v; },
    runQuarantine,
    runPreviewOptimizeVeg,
    runApplyOptimizeVeg,
    setMergeSource(p) { mergeSourcePath = p; },
    runPreviewMerge,
    setFieldResolution,
    runExecuteMerge,
    getState: () => ({ error, statusMessage, complianceReport, optPreview, optResult, mergePreview, mergeResult }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('workbench.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements
    .filter(n => !ts.isImportDeclaration(n))
    .map(n => n.getText(ast))
    .join('\n')
    .replace('export default function', 'function');

  const mountFn = new Function(
    '$',
    'onDestroy',
    'defaultProjectAdminPort',
    'validateNewProjectDraft',
    'validateProjectNameInput',
    'validateSplinterDraft',
    'parseLiteralTokenList',
    'buildInitialMergeFieldResolutions',
    code + '\nreturn _unknown_;'
  );

  const dispatches = [];
  let switchedState = null;
  const mockPort = {
    async createNewProject(cid, req) {
      assert.doesNotThrow(() => structuredClone(req));
      dispatches.push(['createNewProject', cid, req]);
      return {
        projectName: req.projectName,
        projectId: req.projectName,
        destinationPath: `/projects/${req.projectName}.db`,
        destinationSha256: 'deadbeef12345678',
        activated: req.activateOnSuccess,
        state: { activeProject: req.projectName, contextId: 'ctx-next' },
      };
    },
    async runComplianceValidation(cid, opts) {
      assert.doesNotThrow(() => structuredClone(opts));
      dispatches.push(['runComplianceValidation', cid, opts]);
      return {
        project: 'Sample',
        su: 'None',
        useActiveSu: false,
        passed: false,
        summary: {
          envCodeErrors: 1,
          boundsErrors: 0,
          envAdminMismatches: 0,
          orphanChildRows: 2,
          duplicateSpeciesSplits: 1,
          unlistedSpecies: 0,
          synonymSpecies: 0,
          userSpecies: 0,
          plotsWithoutVeg: 0,
        },
        envCodeFindings: [{ listName: 'MoistureRegime', field: 'MoistureRegime', plotNumber: '001', value: 'ZZ' }],
        boundsFindings: [],
        envAdminFindings: [],
        orphanFindings: [{ table: 'Veg', plotNumber: 'ORPH1', rowCount: 2 }],
        plotsWithoutVegetation: [],
        duplicateVegFindings: [],
        taxonomyFindings: [],
      };
    },
    async quarantineOrphanRecords(cid, req) {
      assert.doesNotThrow(() => structuredClone(req));
      dispatches.push(['quarantineOrphanRecords', cid, req]);
      return {
        project: 'Sample',
        batchId: 'b123',
        totalQuarantined: 2,
        reservedDeletedIds: [77, 78],
        tables: [{ table: 'Veg', quarantineTable: '__VPRO_Quarantine_Veg', movedRows: 2, plotNumbers: ['ORPH1'] }],
      };
    },
    async previewOptimizeVeg(cid, opts) {
      assert.doesNotThrow(() => structuredClone(opts));
      dispatches.push(['previewOptimizeVeg', cid, opts]);
      return {
        project: 'Sample',
        beforeRowCount: 10,
        afterRowCount: 8,
        rowsToConsolidate: 2,
        duplicateSpeciesCount: 1,
        overlappingGroupCount: 1,
        cappedCoverCount: 1,
        previewDigest: 'digest-opt-1',
        groups: [
          {
            plotNumber: '001',
            species: 'PSEUMEN',
            sourceRowCount: 3,
            survivorId: 10,
            retiredIds: [11, 12],
            hasOverlappingLayerCovers: true,
            cappedCoverColumns: ['Cover4'],
            mergedCovers: { Cover4: 99, Cover6: 20 },
            mergedHeights: { Height6: 0.65 },
          },
        ],
      };
    },
    async applyOptimizeVeg(cid, req) {
      assert.doesNotThrow(() => structuredClone(req));
      assert.equal(req.expectedDigest, 'digest-opt-1');
      dispatches.push(['applyOptimizeVeg', cid, req]);
      return {
        project: 'Sample',
        beforeRowCount: 10,
        afterRowCount: 8,
        consolidatedGroups: 1,
        deletedRows: 2,
        reservedDeletedIds: [11, 12],
        auditRowsWritten: 5,
        groups: [],
      };
    },
    async previewThreeWayMerge(cid, req) {
      assert.doesNotThrow(() => structuredClone(req));
      dispatches.push(['previewThreeWayMerge', cid, req]);
      return {
        sourcePath: req.sourcePath,
        sourceProject: 'Stage',
        targetPath: '/projects/Sample.db',
        targetProject: 'Sample',
        previewDigest: 'merge-digest-99',
        addedCount: 1,
        identicalCount: 4,
        fastForwardCount: 1,
        conflictCount: 1,
        targetOnlyCount: 0,
        plots: [],
        conflicts: [
          {
            conflictId: '0102::Env::Zone',
            plotNumber: '0102',
            table: 'Env',
            key: 'Zone',
            field: 'Zone',
            sourceVal: 'ICH',
            targetVal: 'IDF',
            kind: 'Conflict',
            defaultResolution: 'keep_target',
          },
        ],
      };
    },
    async executeThreeWayMerge(cid, req) {
      assert.doesNotThrow(() => structuredClone(req));
      dispatches.push(['executeThreeWayMerge', cid, req]);
      return {
        sourceProject: 'Stage',
        targetProject: 'Sample',
        targetPath: '/projects/Sample.db',
        addedPlots: 1,
        updatedPlots: 2,
        skippedPlots: 4,
        allocatedChildIds: [101],
        reservedDeletedIds: [],
        auditRowsWritten: 9,
      };
    },
  };

  let api;
  const dispose = client.effect_root(() =>
    mountFn(
      client,
      () => {},
      workbench.defaultProjectAdminPort,
      workbench.validateNewProjectDraft,
      workbench.validateProjectNameInput,
      workbench.validateSplinterDraft,
      workbench.parseLiteralTokenList,
      workbench.buildInitialMergeFieldResolutions
    )(null, {
      contextId: 'ctx-live',
      activeProject: 'Sample',
      activeSu: 'Sample',
      port: mockPort,
      oncontextswitched: st => {
        switchedState = st;
      },
      inspect: v => {
        api = v;
      },
    })
  );
  client.flush();

  try {
    // 1. Create new project with context switch
    api.setNewProj('Bulkley2026', 'LMH25_2023_FS882', true);
    client.flush();
    await api.submitCreateNewProject();
    client.flush();
    assert.equal(switchedState.activeProject, 'Bulkley2026');

    // 2. Quarantine requires explicit confirmation checkbox
    await api.runCompliance();
    client.flush();
    await api.runQuarantine();
    client.flush();
    assert.match(api.getState().error, /Confirm orphan quarantine/);
    api.setConfirmQuarantine(true);
    client.flush();
    await api.runQuarantine();
    client.flush();
    assert.equal(dispatches.some(d => d[0] === 'quarantineOrphanRecords'), true);

    // 3. OptimizeVeg preview -> apply carries expectedDigest
    await api.runPreviewOptimizeVeg();
    client.flush();
    assert.equal(api.getState().optPreview.groups[0].mergedHeights.Height6, 0.65);
    await api.runApplyOptimizeVeg();
    client.flush();
    assert.equal(api.getState().optResult.deletedRows, 2);

    // 4. 3-Way Merge preview -> override conflict resolution -> execute
    api.setMergeSource('/staging/Stage.db');
    client.flush();
    await api.runPreviewMerge();
    client.flush();
    api.setFieldResolution('0102::Env::Zone', 'accept_source');
    client.flush();
    await api.runExecuteMerge();
    client.flush();
    const execCall = dispatches.find(d => d[0] === 'executeThreeWayMerge');
    assert.ok(execCall);
    assert.equal(execCall[2].expectedPreviewDigest, 'merge-digest-99');
    assert.equal(execCall[2].fieldResolutions['0102::Env::Zone'], 'accept_source');
  } finally {
    dispose();
  }
});

test('App.svelte keeps ProjectAdminWorkbench mounted across context activation so lifecycle receipts remain visible', () => {
  const appSrc = read('App.svelte');
  const sectionStart = appSrc.indexOf("{:else if view === 'project-admin'");
  const sectionEnd = appSrc.indexOf("{:else if view === 'lump-editor'");
  assert.ok(sectionStart > 0 && sectionEnd > sectionStart);
  const block = appSrc.slice(sectionStart, sectionEnd);
  assert.doesNotMatch(block, /\{#key \$projectState\.contextId\}/, 'project-admin must update in place without #key remount');
  assert.match(block, /projectId=""/, 'ProjectMetadataPresets must receive empty projectId to auto-detect existing _Metadata row');
});
