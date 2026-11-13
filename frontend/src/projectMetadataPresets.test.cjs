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
const presetsModule = loadTypeScript('projectMetadataPresets.ts', {
  '@wailsio/runtime': {
    Call: {
      ByName(methodName, ...args) {
        wailsCalls.push({ methodName, args });
        return Promise.resolve([]);
      },
    },
  },
  './qualityEditor': quality,
});

const read = name => readFileSync(path.join(__dirname, name), 'utf8');

const samplePresets = [
  {
    id: 'LMH25_2023_FS882',
    label: 'LMH25 (2023) Standard FS882',
    description: 'Standard 20x20m BEC plot protocol.',
    ecosysCollectionStandard: 'LMH25',
    vegCoverMethod: 'Percent',
    plotMethod: '20x20',
    datum: '2',
    coverDescriptions: Object.fromEntries(presetsModule.COVER_DESCRIPTION_ORDER.map(k => [k, `Desc ${k}`])),
  },
  {
    id: 'CHARS_Arctic',
    label: 'CHARS Arctic',
    description: 'Extended Arctic shrub sub-layers.',
    ecosysCollectionStandard: 'CHAR',
    vegCoverMethod: 'Percent',
    plotMethod: '20x20',
    datum: '2',
    coverDescriptions: Object.fromEntries(presetsModule.COVER_DESCRIPTION_ORDER.map(k => [k, `Arctic ${k}`])),
  },
];

const sampleDerived = {
  project: 'Sample',
  projectId: 'SMP26',
  totalPlotsInScope: 14,
  startDate: 2023,
  endDate: 2026,
  numberOfFs882Plots: 10,
  numberOfSiteVisits: 4,
  unclassifiedPlotTypes: 0,
  geographicStudyRegion: 'RCB.DMH; RSK.DSS',
  regionCodesMatched: ['RCB.DMH', 'RSK.DSS'],
  collectedSite: 1,
  collectedVeg: 2,
  collectedSoil: 2,
  collectedTerrain: 1,
  allSpecs: 'BCPlantTaxa_v19_2025 (2026-02-09) [sha256:81e259244106]',
  tableOfLists: 'LMH25_2023 + BECdb_v13_2026 [sha256:81e259244106]',
  vlistsSha256Short: '81e259244106',
};

test('metadata preset helpers cover all 15 Cover*Description columns, NAD83 datum 2, and UTF-16 validation', () => {
  assert.equal(presetsModule.COVER_DESCRIPTION_ORDER.length, 15);
  assert.equal(presetsModule.formatDatumCode('2'), '2 (NAD83)');
  assert.match(presetsModule.formatCollectionCode(1), /Complete/);
  assert.match(presetsModule.formatCollectionCode(2), /Partial/);
  assert.match(presetsModule.formatCollectionCode(3), /None/);

  assert.match(
    presetsModule.validateMetadataAutomationRequest({
      projectId: 'TOO_LONG_PROJECT_ID_123',
      applyDerived: true,
    }),
    /20 UTF-16/
  );
  assert.match(
    presetsModule.validateMetadataAutomationRequest({
      projectId: 'SMP26',
      applyDerived: false,
      presetId: '',
    }),
    /Select Auto-Derived/
  );
  assert.equal(
    presetsModule.validateMetadataAutomationRequest({
      projectId: 'SMP26',
      applyDerived: true,
      presetId: 'LMH25_2023_FS882',
    }),
    null
  );
});

test('defaultProjectMetadataAutomationPort maps to ContextService Wails methods', async () => {
  wailsCalls.length = 0;
  await presetsModule.defaultProjectMetadataAutomationPort.listPresets();
  await presetsModule.defaultProjectMetadataAutomationPort.deriveFromPlots('ctx-1', 'SMP');
  await presetsModule.defaultProjectMetadataAutomationPort.applyAutomation('ctx-1', {
    projectId: 'SMP',
    applyDerived: true,
    presetId: 'LMH25_2023_FS882',
  });
  assert.deepEqual(
    wailsCalls.map(c => c.methodName),
    [
      'main.ContextService.ListProjectMetadataPresets',
      'main.ContextService.DeriveProjectMetadataFromPlots',
      'main.ContextService.ApplyProjectMetadataAutomation',
    ]
  );
});

test('ProjectMetadataPresets SSR and client mount preview derived plot metrics and detach proxies on Apply', async () => {
  const Component = serverComponent(read('ProjectMetadataPresets.svelte'), 'ProjectMetadataPresets.svelte', {
    './projectMetadataPresets': presetsModule,
  });
  const ssrHtml = render(Component, { props: { contextId: 'ctx-meta', projectId: 'SMP26' } }).body;
  assert.match(ssrHtml, /Auto-Populate Project Metadata from Plots/);
  assert.match(ssrHtml, /Apply to ProjectMetadata/);

  const client = await import('svelte/internal/client');
  let script = read('ProjectMetadataPresets.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('onapplied,', 'onapplied, inspect,');
  script += `\n$effect(() => inspect({
    loadPreviewAndPresets,
    commitAutomation,
    selectPreset(id) { selectedPresetId = id; },
    getState: () => ({ derivedPreview, selectedPreset, status, error, lastResult }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('presets.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements
    .filter(n => !ts.isImportDeclaration(n))
    .map(n => n.getText(ast))
    .join('\n')
    .replace('export default function', 'function');

  const mountFn = new Function(
    '$',
    'onDestroy',
    'onMount',
    'defaultProjectMetadataAutomationPort',
    'COVER_DESCRIPTION_ORDER',
    'formatCollectionCode',
    'formatDatumCode',
    'validateMetadataAutomationRequest',
    code + '\nreturn _unknown_;'
  );

  let appliedPayload = null;
  let notified = null;
  const mockPort = {
    listPresets: () => Promise.resolve(structuredClone(samplePresets)),
    deriveFromPlots: () => Promise.resolve(structuredClone(sampleDerived)),
    applyAutomation: (cid, req) => {
      assert.equal(cid, 'ctx-meta');
      assert.doesNotThrow(() => structuredClone(req));
      appliedPayload = req;
      return Promise.resolve({
        project: 'Sample',
        projectId: req.projectId,
        metadataId: 3,
        createdRow: false,
        updatedColumns: ['StartDate', 'EndDate', 'Datum', 'CoverB2aDescription'],
        auditRowCount: 25,
        derived: structuredClone(sampleDerived),
      });
    },
  };

  let api;
  const dispose = client.effect_root(() =>
    mountFn(
      client,
      () => {},
      fn => fn(),
      presetsModule.defaultProjectMetadataAutomationPort,
      presetsModule.COVER_DESCRIPTION_ORDER,
      presetsModule.formatCollectionCode,
      presetsModule.formatDatumCode,
      presetsModule.validateMetadataAutomationRequest
    )(null, {
      contextId: 'ctx-meta',
      projectId: 'SMP26',
      port: mockPort,
      onapplied: res => {
        notified = res;
      },
      inspect: v => {
        api = v;
      },
    })
  );
  client.flush();
  await new Promise(r => setTimeout(r, 10));
  client.flush();

  try {
    assert.equal(api.getState().derivedPreview.totalPlotsInScope, 14);
    assert.equal(api.getState().derivedPreview.vlistsSha256Short, '81e259244106');
    api.selectPreset('CHARS_Arctic');
    client.flush();
    assert.equal(api.getState().selectedPreset.ecosysCollectionStandard, 'CHAR');
    await api.commitAutomation();
    client.flush();
    assert.deepEqual(appliedPayload, {
      projectId: 'SMP26',
      applyDerived: true,
      presetId: 'CHARS_Arctic',
    });
    assert.equal(notified.metadataId, 3);
  } finally {
    dispose();
  }
});

test('ProjectMetadataPresets refreshes derived preview and auto-detects ProjectID when contextId switches in place', async () => {
  const client = await import('svelte/internal/client');
  let script = read('ProjectMetadataPresets.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('onapplied,', 'onapplied, inspect,');
  script += `\n$effect(() => inspect({
    getState: () => ({ targetProjectId, derivedPreview, status }),
  }));`;
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('presetsInPlace.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements
    .filter(n => !ts.isImportDeclaration(n))
    .map(n => n.getText(ast))
    .join('\n')
    .replace('export default function', 'function');

  const mountFn = new Function(
    '$',
    'onDestroy',
    'onMount',
    'defaultProjectMetadataAutomationPort',
    'COVER_DESCRIPTION_ORDER',
    'formatCollectionCode',
    'formatDatumCode',
    'validateMetadataAutomationRequest',
    code + '\nreturn _unknown_;'
  );

  const deriveCalls = [];
  const mockPort = {
    listPresets: () => Promise.resolve(structuredClone(samplePresets)),
    deriveFromPlots: (cid, pid) => {
      deriveCalls.push([cid, pid]);
      return Promise.resolve({
        ...structuredClone(sampleDerived),
        projectId: cid === 'ctx-2' ? 'BULK26' : 'SMP26',
      });
    },
    applyAutomation: () => Promise.resolve(null),
  };

  const ctxProp = client.state('ctx-1');
  let api;
  const dispose = client.effect_root(() =>
    mountFn(
      client,
      () => {},
      fn => fn(),
      presetsModule.defaultProjectMetadataAutomationPort,
      presetsModule.COVER_DESCRIPTION_ORDER,
      presetsModule.formatCollectionCode,
      presetsModule.formatDatumCode,
      presetsModule.validateMetadataAutomationRequest
    )(null, {
      get contextId() {
        return client.get(ctxProp);
      },
      projectId: '',
      port: mockPort,
      inspect: v => {
        api = v;
      },
    })
  );
  try {
    client.flush();
    await new Promise(r => setTimeout(r, 10));
    client.flush();
    assert.equal(api.getState().targetProjectId, 'SMP26');

    client.set(ctxProp, 'ctx-2');
    client.flush();
    await new Promise(r => setTimeout(r, 10));
    client.flush();
    assert.equal(api.getState().targetProjectId, 'BULK26');
    assert.deepEqual(deriveCalls, [
      ['ctx-1', ''],
      ['ctx-2', ''],
    ]);
  } finally {
    dispose();
  }
});
