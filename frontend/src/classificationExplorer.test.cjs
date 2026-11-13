const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const wailsStub = {
  Call: {
    ByName: () => Promise.resolve(null),
  },
};

const grid = loadTypeScript('universalDatasheetGrid.ts', {
  '@wailsio/runtime': wailsStub,
});
const cls = loadTypeScript('classificationExplorer.ts', {
  '@wailsio/runtime': wailsStub,
});
const plain = value => JSON.parse(JSON.stringify(value));

test('phytosociological wide rank schema exposes all 10 required columns', () => {
  assert.deepEqual(
    plain(cls.PHYTOSOCIOLOGICAL_RANK_COLUMNS.map(c => c.label)),
    [
      'Class',
      'Order',
      'SubOrder',
      'Alliance',
      'SubAlliance',
      'Association',
      'SubAssociation',
      'Facies',
      'WorkingUnit',
      'SiteSeries',
    ]
  );
});

test('flattenVisibleSUTree renders unit folders, draggable plot leaves, and Unassigned Plots bucket', () => {
  const tree = {
    contextId: 'ctx-1',
    project: 'Sample',
    projectPath: '/data/Sample.db',
    su: 'Sample',
    suPath: '/data/Sample.db',
    totalProject: 3,
    totalAssigned: 2,
    totalUnassigned: 1,
    orphanPlots: [],
    units: [
      {
        siteUnit: 'CWHvm1/01',
        longName: 'HwBa - Blueberry',
        origin: 'master',
        maskValid: true,
        plotCount: 2,
        plots: [
          { plotNumber: '00337', fieldNumber: 'F1', zone: 'CWH', subZone: 'vm1', siteSeries: '01', userSiteUnit: '', moistureRegime: '4', nutrientRegime: 'C', elevation: 320, siteUnit: 'CWHvm1/01', presentInSu: true, inProjectEnv: true, locked: false },
          { plotNumber: '108050', fieldNumber: 'F2', zone: 'CWH', subZone: 'vm1', siteSeries: '01', userSiteUnit: '', moistureRegime: '5', nutrientRegime: 'D', elevation: 410, siteUnit: 'CWHvm1/01', presentInSu: true, inProjectEnv: true, locked: false },
        ],
      },
    ],
    unassignedPlots: [
      { plotNumber: '202050', fieldNumber: 'F3', zone: 'MH', subZone: 'mm1', siteSeries: '', userSiteUnit: '', moistureRegime: '', nutrientRegime: '', elevation: null, siteUnit: '', presentInSu: false, inProjectEnv: true, locked: false },
    ],
  };

  const collapsed = cls.flattenVisibleSUTree(tree, new Set());
  assert.equal(collapsed.length, 2); // unit::CWHvm1/01 + bucket::__unassigned__

  const expanded = cls.flattenVisibleSUTree(
    tree,
    new Set(['unit::CWHvm1/01', cls.UNASSIGNED_BUCKET_ID])
  );
  assert.equal(expanded.length, 5);
  assert.equal(expanded[1].label, '00337');
  assert.equal(expanded[4].label, '202050');
});

test('WAI-ARIA tree keyboard navigation handles ArrowUp/Down/Left/Right/Home/End/Enter', () => {
  const items = [
    { id: 'unit::A', parentId: null, expandable: true, expanded: false },
    { id: 'unit::B', parentId: null, expandable: true, expanded: true },
    { id: 'plot::B::1', parentId: 'unit::B', expandable: false, expanded: false },
  ];

  // ArrowRight on collapsed folder -> expand
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'unit::A', 'ArrowRight')), {
    type: 'expand',
    targetId: 'unit::A',
  });
  // ArrowRight on expanded folder -> focus first child
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'unit::B', 'ArrowRight')), {
    type: 'focus',
    targetId: 'plot::B::1',
  });
  // ArrowLeft on child leaf -> focus parent folder
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'plot::B::1', 'ArrowLeft')), {
    type: 'focus',
    targetId: 'unit::B',
  });
  // ArrowLeft on expanded folder -> collapse
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'unit::B', 'ArrowLeft')), {
    type: 'collapse',
    targetId: 'unit::B',
  });
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'unit::A', 'End')), {
    type: 'focus',
    targetId: 'plot::B::1',
  });
  assert.deepEqual(plain(cls.resolveAriaTreeKeyAction(items, 'plot::B::1', 'Home')), {
    type: 'focus',
    targetId: 'unit::A',
  });
});

test('hierarchy cycle detection, ~ breakpoint toggle, BEC mask check, and flat row validation', () => {
  const nodes = [
    { id: 1, parent: null },
    { id: 2, parent: 1 },
    { id: 3, parent: 2 },
  ];
  assert.equal(cls.wouldIntroduceHierarchyCycle(nodes, 1, 1), true);
  assert.equal(cls.wouldIntroduceHierarchyCycle(nodes, 1, 3), true);
  assert.equal(cls.wouldIntroduceHierarchyCycle(nodes, 3, 1), false);
  assert.equal(cls.wouldIntroduceHierarchyCycle(nodes, 2, null), false);

  assert.equal(cls.toggleBreakpointName('Tsugion'), '~Tsugion');
  assert.equal(cls.toggleBreakpointName('~Tsugion'), 'Tsugion');

  assert.equal(cls.isValidBECUnitMask('CWHvm1/01'), true);
  assert.equal(cls.isValidBECUnitMask('SBSmk2/05'), true);
  assert.equal(cls.isValidBECUnitMask('MyWorkingUnit'), false);

  const okErr = cls.validateFlatHierarchyRows([
    { ...cls.createEmptyFlatHierarchyRow(), class: 'Vaccinio-Piceetea', order: '~Piceetalia', siteSeries: 'CWHvm1/01' },
    { ...cls.createEmptyFlatHierarchyRow(), class: 'Vaccinio-Piceetea', order: '~Piceetalia', siteSeries: 'CWHvm1/05' },
  ]);
  assert.equal(okErr, '');

  const conflictErr = cls.validateFlatHierarchyRows([
    { ...cls.createEmptyFlatHierarchyRow(), class: 'ClassA', order: 'SharedOrder', siteSeries: 'CWHvm1/01' },
    { ...cls.createEmptyFlatHierarchyRow(), class: 'ClassB', order: 'SharedOrder', siteSeries: 'CWHvm1/05' },
  ]);
  assert.match(conflictErr, /conflicting parents/);
});

test('ClassificationExplorer.svelte compiles with zero warnings, renders all 3 modes, and has zero emojis', () => {
  const svelteSource = readFileSync(path.join(__dirname, 'ClassificationExplorer.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(svelteSource), false, 'must contain zero emojis');

  const Component = serverComponent(svelteSource, 'ClassificationExplorer.svelte', {
    './classificationExplorer': cls,
    './universalDatasheetGrid': grid,
  });

  const suRender = render(Component, {
    props: { contextId: 'ctx-1', project: 'Sample', su: 'Sample', hierarchy: 'Sample', initialMode: 'su_tree' },
  });
  assert.match(suRender.body, /role="tree"/);
  assert.match(suRender.body, /BECdb v13 Unit Picker/);

  const flatRender = render(Component, {
    props: { contextId: 'ctx-1', project: 'Sample', su: 'Sample', hierarchy: 'Sample', initialMode: 'hierarchy_flat' },
  });
  assert.match(flatRender.body, /Wide Phytosociological Rank Editor/);
  assert.match(flatRender.body, /SubAlliance/);
});

test('ClassificationExplorer avoids duplicate reload races on self-activated SU creation and warns when 0 rows change on assignment', async () => {
  const vm = require('node:vm');
  const ts = require('typescript');
  const { compile } = require('svelte/compiler');
  const client = await import('svelte/internal/client');

  const appSrc = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const clsSectionStart = appSrc.indexOf("{:else if view === 'classification-explorer'");
  const clsSectionEnd = appSrc.indexOf("{:else if view === 'project-admin'");
  assert.ok(clsSectionStart > 0 && clsSectionEnd > clsSectionStart);
  assert.doesNotMatch(
    appSrc.slice(clsSectionStart, clsSectionEnd),
    /\{#key \$projectState\.contextId\}/,
    'classification-explorer must update in place without #key remount'
  );

  let script = readFileSync(path.join(__dirname, 'ClassificationExplorer.svelte'), 'utf8').match(
    /<script lang="ts">([\s\S]*?)<\/script>/
  )[1];
  script = script.replace(
    "onRequestSUSync?: (direction: 'su_to_env' | 'env_to_su') => void;",
    "onRequestSUSync?: (direction: 'su_to_env' | 'env_to_su') => void; inspect?: (v: any) => void;"
  );
  script = script.replace('onRequestSUSync,', 'onRequestSUSync, inspect,');
  script += `\n$effect(() => inspect?.({
    setNewSUName(name) { newSUName = name; },
    submitCreateSU,
    assignPlots,
    getState: () => ({ effectiveContextId, statusBanner, errorMessage }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const modObj = { exports: {} };
  vm.runInNewContext(
    ts.transpileModule(compiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
    {
      module: modObj,
      exports: modObj.exports,
      structuredClone,
      queueMicrotask,
      CSS: { escape: s => s },
      require(name) {
        if (name === 'svelte') {
          return {
            onMount: fn => fn(),
            onDestroy: () => {},
            untrack: client.untrack,
          };
        }
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name === './readRequests') return { ReadRequests: class { track(p) { return p; } cancelAll() {} } };
        if (name === './universalDatasheetGrid') return grid;
        if (name === './classificationExplorer') return cls;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );

  const treeLoads = [];
  const ctxProp = client.state('ctx-1');
  let api;
  const dispose = client.effect_root(() => {
    modObj.exports.default(null, {
      get contextId() {
        return client.get(ctxProp);
      },
      project: 'Sample',
      su: 'Sample',
      hierarchy: 'Sample',
      port: {
        getSUPlotTree: async cid => {
          treeLoads.push(cid);
          return {
            contextId: cid,
            project: 'Sample',
            projectPath: '/Sample.db',
            su: 'Sample',
            suPath: '/Sample.db',
            totalProject: 1,
            totalAssigned: 1,
            totalUnassigned: 0,
            orphanPlots: [],
            units: [],
            unassignedPlots: [],
          };
        },
        searchBECUnitPicker: async () => [],
        createSUTable: async () => ({
          contextId: 'ctx-2',
          name: 'Working2026',
          tableName: 'Working2026_SU',
          filePath: '/Sample.db',
          plotCount: 14,
          activated: true,
          newState: { contextId: 'ctx-2' },
        }),
        assignPlotsToSiteUnit: async cid => ({
          contextId: cid,
          su: 'Sample',
          tableName: 'Sample_SU',
          changedPlots: 0,
          historyId: 0,
        }),
      },
      onContextChanged: nextId => {
        // Simulate asynchronous parent refresh updating contextId prop on a later tick
        setTimeout(() => {
          client.set(ctxProp, nextId);
          client.flush();
        }, 5);
      },
      inspect: v => {
        api = v;
      },
    });
  });

  try {
    client.flush();
    await new Promise(r => setTimeout(r, 10));
    client.flush();
    assert.deepEqual(treeLoads, ['ctx-1']);
    treeLoads.length = 0;

    api.setNewSUName('Working2026');
    client.flush();
    await api.submitCreateSU();
    client.flush();
    // Wait for delayed parent contextId update
    await new Promise(r => setTimeout(r, 20));
    client.flush();

    // Must have loaded 'ctx-2' ONCE without reverting to 'ctx-1' or firing a duplicate 'ctx-2' reload!
    assert.equal(api.getState().effectiveContextId, 'ctx-2');
    assert.deepEqual(treeLoads, ['ctx-2']);
    assert.match(api.getState().statusBanner, /Created and activated Working2026_SU/);

    // 0 changed rows on AssignPlotsToSiteUnit must explain why 0 rows changed
    await api.assignPlots(['00337'], 'CWHvm1/01');
    client.flush();
    assert.match(api.getState().statusBanner, /0 plot\(s\) changed.*already assigned|read-only/i);
  } finally {
    dispose();
  }
});
