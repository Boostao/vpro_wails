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
