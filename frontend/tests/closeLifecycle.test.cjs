const { srcPath } = require('./svelteTestHelpers.cjs');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');

const read = name => readFileSync(srcPath(name), 'utf8');
const exportsObject = {};
vm.runInNewContext(ts.transpileModule(read('closeLifecycle.ts'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText, { exports: exportsObject });
const { closeDisposition } = exportsObject;
const state = (overrides = {}) => ({ unsaved: false, busy: false, canSave: true, saveReason: '', error: null, ...overrides });

test('clean native close is allowed only after context and editor operations finish', () => {
  assert.equal(closeDisposition(null, false), 'clean');
  assert.equal(closeDisposition(state(), false), 'clean');
  assert.equal(closeDisposition(null, true), 'busy');
  assert.equal(closeDisposition(state({ busy: true }), false), 'busy');
});
test('busy takes precedence over an unsaved draft and dirty requires a decision', () => {
  assert.equal(closeDisposition(state({ unsaved: true }), false), 'dirty');
  assert.equal(closeDisposition(state({ unsaved: true }), true), 'busy');
  assert.equal(closeDisposition(state({ unsaved: true, busy: true }), false), 'busy');
  assert.equal(closeDisposition(state({ unsaved: true, canSave: false }), false), 'dirty');
});
test('explicit SIVI safety blocks stop native close and context discard without pretending Undo is busy', () => {
  assert.equal(closeDisposition(state({ unsaved: true, blocked: true }), false), 'busy');
  assert.equal(closeDisposition(state({ blocked: true, busy: false }), false), 'busy');
  assert.equal(closeDisposition(state({ unsaved: true, blocked: false }), false), 'dirty');
  assert.equal(closeDisposition(state({ blocked: false }), false), 'clean');
});
test('native close dialog compiles and exposes explicit accessible decisions', () => {
  const source = read('CloseConfirm.svelte');
  const result = compile(source, { filename: 'CloseConfirm.svelte', generate: 'client' });
  assert.equal(result.warnings.length, 0);
  assert.match(source, /dialog\.showModal\(\)/);
  assert.match(source, /aria-labelledby="close-title"/);
  assert.match(source, /event\.preventDefault\(\)/);
  for (const decision of ['cancel', 'save', 'discard']) assert.ok(source.includes(`onrespond('${decision}')`));
  assert.match(source, /disabled=\{working \|\| !canSave\}/);
});
test('form refuses save-and-close on invalid, locked, loading or incomplete child entry', () => {
  const source = read('FS882Form.svelte');
  assert.match(source, /export function getCloseState\(\): EditorCloseState/);
  assert.match(source, /siviCreationPeerUnsaved = \$derived\(heightUnsaved \|\| siviUnsaved \|\| siviCoverUnsaved \|\| siviCombinedUnsaved \|\| siviCollectedUnsaved \|\| siviSpeciesUnsaved \|\| siviIdentityUnsaved \|\| otherUnsaved \|\| soilUnsaved \|\| attributeUnsaved \|\| collectedUnsaved \|\| speciesUnsaved \|\| deletionReview !== null \|\| creationDraft !== null \|\| codeCheckOpen \|\| metadataOpen \|\| profileReviewBlocked \|\| environmentSUOpen\)/);
  assert.match(source, /nonParentChildUnsaved = \$derived\(siviCreationPeerUnsaved \|\| siviCreationPending \|\| siviDeletionPending \|\| siviRestorationPending \|\| siviCreationUndoPending\)/);
  assert.match(source, /childUnsaved = \$derived\(nonParentChildUnsaved \|\| siviParentWriteUnsaved \|\| siviParentActionUnsaved \|\| siviProjectAssignmentUnsaved \|\| siviParentSharedUnsaved \|\| twoPageOpen \|\| twoPageSoilBusy \|\| twoPageSoilBlocked \|\| twoPageVegetationBusy \|\| twoPageVegetationBlocked\)/);
  assert.match(source, /unsaved: dirty \|\| childUnsaved \|\| invalid \|\| newChild !== null \|\| invalidChild/);
  assert.match(source, /const invalid = Object\.keys\(headerValidation\)\.length > 0 \|\| heightInvalid\.length > 0 \|\| otherInvalid\.length > 0/);
  assert.match(source, /if \(!state\.canSave\)/);
  assert.match(source, /return !dirty && !childUnsaved && Object\.keys\(headerValidation\)\.length === 0 && error === null/);
  assert.match(source, /headerRevision\+\+;[\s\S]*?successMsg = 'Changes undone.'/);
  assert.match(source, /\{#key headerRevision\}/);
});
test('root retrieves missed events, flushes focused edits, cancels busy close and awaits save before approval', () => {
  const source = read('App.svelte');
  assert.match(source, /Events\.On\('vpro:close-request'/);
  assert.match(source, /LifecycleService\.GetPendingCloseRequest\(\)/);
  assert.match(source, /document\.activeElement\.blur\(\)/);
  assert.match(source, /await tick\(\)/);
  assert.match(source, /disposition === 'busy'[\s\S]*?await LifecycleService\.CancelClose\(requestId\)/);
  assert.match(source, /await editor\.saveForClose\(\)/);
  assert.match(source, /stopCloseEvents\(\)/);
});

test('companionPanelBusy isolates secondary panels and modals from clobbering localEditorBusy and guards close/transitions/find', async () => {
  const { componentFunctions } = require('./svelteTestHelpers.cjs');
  const app = read('App.svelte');

  // Exact editorBusy line invariant must remain preserved
  assert.match(
    app,
    /const editorBusy = \$derived\(localEditorBusy \|\| matrixPublicationBusy \|\| pictureCreationBusy \|\| pictureLifecycleBusy \|\| twoPageSoilBusy \|\| twoPageVegetationBusy \|\| externalSUEnvironmentBusy \|\| summaryPreferenceBusy \|\| summaryFieldListBusy \|\| summaryEnvironmentalBusy \|\| projectReportBusy\);/
  );

  // SiteUnitExclusionFilter, ProjectMetadataPresets, and SIVIParentCreateModal must NOT clobber localEditorBusy
  const exclusionMatch = app.match(/<SiteUnitExclusionFilter[\s\S]*?\/>/);
  assert.ok(exclusionMatch);
  assert.doesNotMatch(exclusionMatch[0], /localEditorBusy =/);

  const presetsMatch = app.match(/<ProjectMetadataPresets[\s\S]*?\/>/);
  assert.ok(presetsMatch);
  assert.match(presetsMatch[0], /onBusyChange=/);
  assert.doesNotMatch(presetsMatch[0], /localEditorBusy =/);

  const siviModalMatch = app.match(/<SIVIParentCreateModal[\s\S]*?\/>/);
  assert.ok(siviModalMatch);
  assert.match(siviModalMatch[0], /onBusyChange=/);
  assert.doesNotMatch(siviModalMatch[0], /localEditorBusy =/);

  // Sidebar project/SU/hierarchy controls respect editorBusy || companionPanelBusy
  const sidebar = read('WorkspaceSidebar.svelte');
  assert.match(app, /<WorkspaceSidebar/);
  assert.match(sidebar, /<select id="active-su"[^>]*editorBusy \|\| companionPanelBusy/);
  assert.match(sidebar, /<select id="active-hierarchy"[^>]*editorBusy \|\| companionPanelBusy/);

  const cancelled = [];
  const confirmed = [];
  let lookupCalls = 0;
  let transitions = 0;
  const ctx = componentFunctions(
    'App.svelte',
    ['handleCloseRequest', 'respondToClose', 'requestTransition', 'respondToTransition', 'findScopedPlot'],
    {
      view: 'summary-vegetation',
      busy: false,
      editorBusy: false,
      companionPanelBusy: true,
      workbenchDirtyLabel: '',
      transitionWorking: false,
      archivePublicationBusy: false,
      vegetationPublicationBusy: false,
      lifeformPublicationBusy: false,
      summaryPublicationBusy: false,
      pendingTransition: null,
      editorInitializing: false,
      closeWorking: false,
      closeRequest: '',
      closeError: '',
      transitionError: '',
      error: '',
      plotFindEnabled: true,
      plotFindQuery: '00337',
      plotFindError: '',
      plotFindResult: null,
      $projectState: { contextId: 'ctx-1' },
      editor: { getCloseState: () => state({ unsaved: true }) },
      document: { activeElement: null },
      HTMLElement: class {},
      tick: async () => {},
      closeDisposition,
      lookupPlot: async () => {
        lookupCalls++;
        return null;
      },
      LifecycleService: {
        GetPendingCloseRequest: async () => 'req-companion',
        CancelClose: async id => {
          cancelled.push(id);
        },
        ConfirmClose: async id => {
          confirmed.push(id);
        },
      },
    }
  );

  await ctx.actions.handleCloseRequest('req-companion');
  assert.deepEqual(cancelled, ['req-companion']);
  assert.deepEqual(confirmed, []);

  ctx.closeRequest = 'req-companion';
  await ctx.actions.respondToClose('discard');
  assert.deepEqual(confirmed, []);
  assert.match(ctx.closeError, /Wait for the current operation/);
  ctx.closeRequest = '';

  await ctx.actions.requestTransition(async () => {
    transitions++;
  });
  assert.equal(transitions, 0);

  ctx.pendingTransition = {
    contextId: 'ctx-1',
    run: async () => {
      transitions++;
    },
  };
  await ctx.actions.respondToTransition('discard');
  assert.equal(transitions, 0);
  assert.match(ctx.transitionError, /Wait for the current editor operation/);
  ctx.pendingTransition = null;

  await ctx.actions.findScopedPlot();
  assert.equal(lookupCalls, 0);

  // Clearing companionPanelBusy restores clean close and navigation
  ctx.companionPanelBusy = false;
  await ctx.actions.handleCloseRequest('req-companion');
  assert.deepEqual(confirmed, ['req-companion']);
  await ctx.actions.requestTransition(async () => {
    transitions++;
  });
  assert.equal(transitions, 1);
});

test('unsaved valid drafts in UniversalDatasheetGrid and ClassificationExplorer block close/navigation and onOpenPlot routes via requestTransition', async () => {
  const { componentFunctions, loadTypeScript } = require('./svelteTestHelpers.cjs');
  const client = await import('svelte/internal/client');
  const app = read('App.svelte');

  // Every workbench onOpenPlot callback must route through requestTransition
  const rawOpenMatches = app.match(/onOpenPlot=\{\(p\)\s*=>\s*\{\s*editorPlotNumber/g) ?? [];
  assert.equal(rawOpenMatches.length, 0, 'no raw onOpenPlot view assignment may bypass requestTransition');
  const guardedOpenMatches = app.match(/onOpenPlot=\{\(p\)\s*=>\s*\{\s*void requestTransition\(/g) ?? [];
  assert.equal(guardedOpenMatches.length, 4, 'all 4 workbench onOpenPlot callbacks must use requestTransition');

  // Actionable dirty-workbench error messages on close and transition
  const cancelled = [];
  let navigated = 0;
  const ctx = componentFunctions('App.svelte', ['handleCloseRequest', 'requestTransition'], {
    view: 'datasheet',
    busy: false,
    editorBusy: true,
    companionPanelBusy: false,
    workbenchDirtyLabel: 'datasheet',
    transitionWorking: false,
    archivePublicationBusy: false,
    vegetationPublicationBusy: false,
    lifeformPublicationBusy: false,
    summaryPublicationBusy: false,
    pendingTransition: null,
    editorInitializing: false,
    closeWorking: false,
    closeRequest: '',
    closeError: '',
    transitionError: '',
    error: '',
    $projectState: { contextId: 'ctx-1' },
    editor: null,
    document: { activeElement: null },
    HTMLElement: class {},
    tick: async () => {},
    closeDisposition,
    LifecycleService: {
      GetPendingCloseRequest: async () => 'req-dirty-wb',
      CancelClose: async id => {
        cancelled.push(id);
      },
      ConfirmClose: async () => {
        throw new Error('Must not confirm close with dirty datasheet');
      },
    },
  });

  await ctx.actions.handleCloseRequest('req-dirty-wb');
  assert.deepEqual(cancelled, ['req-dirty-wb']);
  assert.equal(ctx.error, 'Save or discard unsaved workbench edits before closing VPro.');

  await ctx.actions.requestTransition(async () => {
    navigated++;
  });
  assert.equal(navigated, 0);
  assert.equal(ctx.error, 'Save or discard unsaved workbench edits before changing context or view.');

  // Client-mount UniversalDatasheetGrid and stage a VALID edit -> onBusyChange(true) & onDirtyChange(true)
  const gridMod = loadTypeScript('universalDatasheetGrid.ts', {
    '@wailsio/runtime': { Call: { ByName: () => Promise.resolve(null) } },
  });
  let dsScript = read('UniversalDatasheetGrid.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  dsScript = dsScript.replace('onOpenPlot?: (plotNumber: string) => void;', 'onOpenPlot?: (plotNumber: string) => void; inspect?: (v: any) => void;');
  dsScript = dsScript.replace('onOpenPlot,', 'onOpenPlot, inspect,');
  dsScript += `\n$effect(() => inspect?.({ setCellDraft, discardDrafts }));`;
  const dsCompiled = compile(`<script lang="ts">${dsScript}</script>`, { generate: 'client' });
  assert.equal(dsCompiled.warnings.length, 0);
  const dsDestroy = [];
  const dsMod = { exports: {} };
  vm.runInNewContext(
    ts.transpileModule(dsCompiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
    {
      module: dsMod,
      exports: dsMod.exports,
      structuredClone,
      queueMicrotask,
      require(name) {
        if (name === 'svelte') {
          return {
            onMount: fn => fn(),
            onDestroy: fn => dsDestroy.push(fn),
            untrack: client.untrack,
          };
        }
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name.endsWith('/readRequests')) return { ReadRequests: class { track(p) { return p; } cancelAll() {} } };
        if (name.endsWith('/universalDatasheetGrid')) return gridMod;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );

  let latestDsBusy = false;
  let latestDsDirty = false;
  let dsApi;
  const dsDispose = client.effect_root(() => {
    dsMod.exports.default(null, {
      contextId: 'ctx-ds',
      port: {
        listTables: async () => [{ key: 'Veg', tableName: 'Sample_Veg', available: true, rowCount: 1 }],
        getPage: async () => ({
          tableKey: 'Veg',
          tableName: 'Sample_Veg',
          totalRows: 1,
          offset: 0,
          limit: 250,
          canEditMaster: false,
          columns: [
            { name: 'PlotNumber', declaredType: 'TEXT', kind: 'text', pinned: true, editable: false, protected: false, maxUtf16: 7, isCoverSingle: false, isBoolean: false },
            { name: 'Cover1', declaredType: 'SINGLE', kind: 'single', pinned: false, editable: true, protected: false, maxUtf16: 0, isCoverSingle: true, isBoolean: false },
          ],
          rows: [{ rowId: '1', cells: [{ storage: 'text', text: '00337' }, { storage: 'real', real: 10 }] }],
        }),
        applyEdits: async () => ({ updatedCells: 1, updatedRows: 1, auditRows: 1 }),
        previewFindReplace: async () => null,
      },
      initialTableKey: 'Veg',
      onBusyChange: b => { latestDsBusy = b; },
      onDirtyChange: d => { latestDsDirty = d; },
      inspect: v => { dsApi = v; },
    });
  });
  client.flush();
  await new Promise(r => setTimeout(r, 10));
  client.flush();
  assert.equal(latestDsBusy, false);
  assert.equal(latestDsDirty, false);
  // Stage a VALID edit (25.5 is valid single < 100)
  dsApi.setCellDraft(0, 1, '25.5');
  client.flush();
  assert.equal(latestDsBusy, true, 'valid unsaved datasheet edit must hold busy guard');
  assert.equal(latestDsDirty, true, 'valid unsaved datasheet edit must report dirty');
  for (const fn of dsDestroy) fn();
  dsDispose();
  assert.equal(latestDsBusy, false);
  assert.equal(latestDsDirty, false);
});

test('ProjectAdminWorkbench, ProjectMetadataPresets, RSidecarWorkbench, SIVIParentCreateModal, and ClassificationSynthesisView report busy/operationHeld and clean up onDestroy', async () => {
  const client = await import('svelte/internal/client');
  for (const file of [
    'ProjectAdminWorkbench.svelte',
    'ProjectMetadataPresets.svelte',
    'RSidecarWorkbench.svelte',
    'SIVIParentCreateModal.svelte',
  ]) {
    const src = read(file);
    assert.match(src, /onBusyChange\?: \(busy: boolean\) => void/, `${file} must accept onBusyChange`);
    assert.match(src, /operationHeld\?: boolean/, `${file} must accept operationHeld`);
    assert.match(src, /onDestroy\(\(\) => \{[\s\S]*?onBusyChange\?\.\(false\)/, `${file} must clear busy onDestroy`);
  }

  const synSrc = read('ClassificationSynthesisView.svelte');
  assert.match(
    synSrc,
    /onDestroy\(\(\) => \{[\s\S]*?requestGeneration\+\+;[\s\S]*?onBusyChange\?\.\(false\);[\s\S]*?\}\);/,
    'ClassificationSynthesisView must bump requestGeneration and clear onBusyChange in onDestroy'
  );

  // Live client mount check on ClassificationExplorer flatDirty -> onBusyChange(true) & onDirtyChange(true)
  const { loadTypeScript } = require('./svelteTestHelpers.cjs');
  const gridMod = loadTypeScript('universalDatasheetGrid.ts', {
    '@wailsio/runtime': { Call: { ByName: () => Promise.resolve(null) } },
  });
  const clsMod = loadTypeScript('classificationExplorer.ts', {
    '@wailsio/runtime': { Call: { ByName: () => Promise.resolve(null) } },
  });
  let clsScript = read('ClassificationExplorer.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  clsScript = clsScript.replace('onRequestSUSync?: (direction: \'su_to_env\' | \'env_to_su\') => void;', 'onRequestSUSync?: (direction: \'su_to_env\' | \'env_to_su\') => void; inspect?: (v: any) => void;');
  clsScript = clsScript.replace('onRequestSUSync,', 'onRequestSUSync, inspect,');
  clsScript += `\n$effect(() => inspect?.({ markFlatDirty() { flatDirty = true; } }));`;
  const clsCompiled = compile(`<script lang="ts">${clsScript}</script>`, { generate: 'client' });
  assert.equal(clsCompiled.warnings.length, 0);
  const clsDestroy = [];
  const clsExports = { exports: {} };
  vm.runInNewContext(
    ts.transpileModule(clsCompiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
    {
      module: clsExports,
      exports: clsExports.exports,
      structuredClone,
      queueMicrotask,
      CSS: { escape: s => s },
      require(name) {
        if (name === 'svelte') {
          return {
            onMount: fn => fn(),
            onDestroy: fn => clsDestroy.push(fn),
            untrack: client.untrack,
          };
        }
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name.endsWith('/readRequests')) return { ReadRequests: class { track(p) { return p; } cancelAll() {} } };
        if (name.endsWith('/universalDatasheetGrid')) return gridMod;
        if (name.endsWith('/classificationExplorer')) return clsMod;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );
  let clsBusy = false;
  let clsDirty = false;
  let clsApi;
  const clsDispose = client.effect_root(() => {
    clsExports.exports.default(null, {
      contextId: 'ctx-cls',
      initialMode: 'hierarchy_flat',
      port: {
        getHierarchyExplorer: async () => ({
          contextId: 'ctx-cls',
          hierarchy: 'Sample',
          roots: [],
          flatNodes: [],
          flatRankRows: [{ ...clsMod.createEmptyFlatHierarchyRow(), class: 'C1', siteSeries: 'CWHvm1/01' }],
        }),
      },
      onBusyChange: b => { clsBusy = b; },
      onDirtyChange: d => { clsDirty = d; },
      inspect: v => { clsApi = v; },
    });
  });
  client.flush();
  await new Promise(r => setTimeout(r, 10));
  client.flush();
  assert.equal(clsBusy, false);
  assert.equal(clsDirty, false);
  clsApi.markFlatDirty();
  client.flush();
  assert.equal(clsBusy, true, 'valid unsaved flat hierarchy edit must hold busy guard');
  assert.equal(clsDirty, true, 'valid unsaved flat hierarchy edit must report dirty');
  for (const fn of clsDestroy) fn();
  clsDispose();
  assert.equal(clsBusy, false);
  assert.equal(clsDirty, false);
});
