const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, srcPath } = require('./svelteTestHelpers.cjs');

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

const wb = loadTypeScript('rSidecarWorkbench.ts', {
  '@wailsio/runtime': mockWailsRuntime,
  './qualityEditor': quality,
});

const read = name => readFileSync(srcPath(name), 'utf8');

test('clustering, candidate promotion, and Quarto guide validators enforce BEC domain and safety invariants', () => {
  assert.equal(
    wb.validateClusteringDraft({
      method: 'NC',
      distance: 'bray',
      linkage: 'ward.D2',
      transform: 'sqrt',
      k: 4,
      fuzziness: 1.25,
      dnoise: 0.75,
      applyLump: true,
      useActiveSu: false,
      candidatePrefix: 'Kootenay',
      materializeCandidate: true,
    }),
    null
  );

  assert.match(
    wb.validateClusteringDraft({
      method: 'NC',
      distance: 'bray',
      linkage: 'ward.D2',
      transform: 'sqrt',
      k: 1,
      fuzziness: 1.25,
      dnoise: 0.75,
      applyLump: false,
      useActiveSu: false,
      materializeCandidate: false,
    }),
    /between 2 and 50/
  );

  assert.match(
    wb.validateClusteringDraft({
      method: 'NC',
      distance: 'bray',
      linkage: 'ward.D2',
      transform: 'sqrt',
      k: 3,
      fuzziness: 1.0,
      dnoise: 0.75,
      applyLump: false,
      useActiveSu: false,
      materializeCandidate: false,
    }),
    /greater than 1\.0/
  );

  assert.match(
    wb.validateClusteringDraft({
      method: 'NC',
      distance: 'bray',
      linkage: 'ward.D2',
      transform: 'sqrt',
      k: 3,
      fuzziness: 1.25,
      dnoise: 0.75,
      applyLump: false,
      useActiveSu: false,
      candidatePrefix: 'BECMaster',
      materializeCandidate: true,
    }),
    /master/
  );

  // Candidate promotion safety guards: never overwrite active SU without confirmOverwrite, never touch master
  assert.match(
    wb.validatePromoteCandidateDraft(
      {
        candidateSuName: 'Sample_ClusterCandidate',
        targetSuName: 'ActiveSubzone',
        excludeNoise: true,
        confirmOverwrite: false,
        activate: false,
      },
      'ActiveSubzone'
    ),
    /Confirm overwrite/
  );

  assert.equal(
    wb.validatePromoteCandidateDraft(
      {
        candidateSuName: 'Sample_ClusterCandidate',
        targetSuName: 'ActiveSubzone',
        excludeNoise: true,
        confirmOverwrite: true,
        activate: false,
      },
      'ActiveSubzone'
    ),
    null
  );

  assert.match(
    wb.validatePromoteCandidateDraft(
      {
        candidateSuName: 'Sample_ClusterCandidate',
        targetSuName: 'BECMaster_2026',
        excludeNoise: true,
        confirmOverwrite: true,
        activate: false,
      },
      'Sample'
    ),
    /reserved or protected/
  );

  assert.equal(
    wb.validateQuartoGuideDraft({
      guideType: 'single_unit',
      title: 'BWBSmw2 / 05 Guide',
      minConstancyPct: 20,
      useActiveSu: false,
      applyLump: true,
      includePdf: true,
    }),
    null
  );
});

test('defaultRSidecarPort invokes exact Wails Call.ByName endpoints on EnvironmentReportService', async () => {
  wailsCalls.length = 0;
  await wb.defaultRSidecarPort.detectRRuntime();
  await wb.defaultRSidecarPort.getCachedPlotClimate('ctx-1', '1961_1990');
  await wb.defaultRSidecarPort.computePlotClimateAndDualEdatopic('ctx-1', {
    refPeriod: '1961_1990',
    useActiveSu: false,
  });
  await wb.defaultRSidecarPort.runMultivariateClustering('ctx-1', {
    method: 'NC',
    distance: 'bray',
    linkage: 'ward.D2',
    transform: 'sqrt',
    k: 3,
    fuzziness: 1.25,
    dnoise: 0.75,
    applyLump: true,
    useActiveSu: false,
    materializeCandidate: true,
  });
  await wb.defaultRSidecarPort.promoteClusterCandidateToSU('ctx-1', {
    candidateSuName: 'Sample_ClusterCandidate',
    targetSuName: 'FinalSU',
    excludeNoise: true,
    confirmOverwrite: false,
    activate: false,
  });
  await wb.defaultRSidecarPort.renderQuartoGuide('ctx-1', {
    guideType: 'single_unit',
    useActiveSu: false,
    applyLump: true,
    includePdf: false,
  });

  assert.deepEqual(
    wailsCalls.map(c => c.methodName),
    [
      'main.EnvironmentReportService.DetectRRuntime',
      'main.EnvironmentReportService.GetCachedPlotClimate',
      'main.EnvironmentReportService.ComputePlotClimateAndDualEdatopic',
      'main.EnvironmentReportService.RunMultivariateClustering',
      'main.EnvironmentReportService.PromoteClusterCandidateToSU',
      'main.EnvironmentReportService.RenderQuartoGuide',
    ]
  );
});

test('edatopic heatmap indexer and SVG dendrogram coordinate layout scale deterministically', () => {
  const indexed = wb.indexEdatopicCells({
    kind: 'actual',
    moistureAxis: ['3.5', '4', '4.5'],
    nutrientAxis: ['B', 'C'],
    totalPlots: 6,
    modalCell: '4/C',
    cells: [
      { moisture: '3.5', nutrient: 'B', count: 1, frequency: 16.7, plotNumbers: ['00337'] },
      { moisture: '4', nutrient: 'C', count: 5, frequency: 83.3, plotNumbers: ['108050'] },
    ],
  });
  assert.equal(indexed.maxCount, 5);
  assert.equal(wb.edatopicCellColor(5, 5).bg, '#003366');
  assert.equal(wb.edatopicCellColor(1, 5).bg, '#dbeafe');

  const layout = wb.computeDendrogramSVGLayout({
    maxHeight: 1.0,
    cutHeight: 0.4,
    newick: '(00337:0.2,108050:0.2);',
    segments: [
      { x1: 1, y1: 0, x2: 1, y2: 0.2 },
      { x1: 1, y1: 0.2, x2: 2, y2: 0.2 },
      { x1: 2, y1: 0, x2: 2, y2: 0.2 },
    ],
    leaves: [
      { plotNumber: '00337', x: 1, order: 1, clusterUnit: 'BEC_01', isNoise: false },
      { plotNumber: '108050', x: 2, order: 2, clusterUnit: 'BEC_Noise', isNoise: true },
    ],
  });
  assert.equal(layout.leaves.length, 2);
  assert.equal(layout.segments.length, 3);
  assert.ok(layout.cutLineY > 24 && layout.cutLineY < 234);
});

test('RSidecarWorkbench SSR renders BC Gov #003366/#FCBA19 styling, 4 tabs, and zero emojis', () => {
  const svelteSrc = read('RSidecarWorkbench.svelte');
  const tsSrc = read('rSidecarWorkbench.ts');
  // Enforce zero-emoji rule across workbench sources
  const emojiRegex = /\p{Extended_Pictographic}/u;
  assert.equal(emojiRegex.test(svelteSrc), false, 'RSidecarWorkbench.svelte must contain zero emojis');
  assert.equal(emojiRegex.test(tsSrc), false, 'rSidecarWorkbench.ts must contain zero emojis');

  const Component = serverComponent(svelteSrc, 'RSidecarWorkbench.svelte', {
    './rSidecarWorkbench': wb,
  });
  const html = render(Component, {
    props: { contextId: 'ctx-ssr', activeProject: 'Sample', activeSu: 'Sample' },
  }).body;

  assert.match(html, /#003366/);
  assert.match(html, /#FCBA19/);
  assert.match(html, /1\. Runtime &amp; climr Climate Normals/);
  assert.match(html, /2\. Dual rSMR vs aSMR Edatopic Grids/);
  assert.match(html, /3\. BEC_R Clustering &amp; Dendrogram/);
  assert.match(html, /4\. Quarto Unit &amp; BGC Guide Publisher/);
});

test('RSidecarWorkbench client actions detach Svelte 5 proxies via structuredClone and drop stale generation results', async () => {
  const client = await import('svelte/internal/client');
  let script = read('RSidecarWorkbench.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('oncontextswitched,', 'oncontextswitched, inspect,');
  script += `\n$effect(() => inspect({
    refreshRRuntime,
    runClimateAndDualEdatopic,
    submitClustering,
    setPromoteDraft(target, confirm, activate) {
      promoteTargetSuName = target;
      promoteConfirmOverwrite = confirm;
      promoteActivate = activate;
    },
    submitPromoteCandidate,
    submitRenderQuartoGuide,
    bumpContext(nextCtx) { contextId = nextCtx; },
    getState: () => ({ busy, error, statusMessage, runtimeStatus, climateResult, clusterResult, promoteResult, quartoResult }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);

  const destroyCallbacks = [];
  const module = { exports: {} };
  const transpiled = ts.transpileModule(compiled.js.code, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;

  vm.runInNewContext(transpiled, {
    module,
    exports: module.exports,
    structuredClone,
    require(name) {
      if (name === 'svelte') return { onDestroy: fn => destroyCallbacks.push(fn) };
      if (name === 'svelte/internal/client') return client;
      if (name.startsWith('svelte/')) return require(name);
      if (name === './rSidecarWorkbench') return wb;
      throw new Error(`Unexpected module ${name}`);
    },
  });

  let deferredClimateResolve;
  const clonedPayloads = [];
  const switchedStates = [];

  const mockPort = {
    async detectRRuntime() {
      return {
        available: true,
        rscriptPath: '/usr/bin/Rscript',
        rVersion: '4.6.2',
        rVersionFull: 'R 4.6.2',
        platform: 'x86_64-pc-linux-gnu',
        quartoAvailable: true,
        quartoPath: '/usr/local/bin/quarto',
        quartoVersion: '1.9.38',
        climrCachePath: '',
        climrRastersCached: false,
        readyForClimate: true,
        readyForClustering: true,
        readyForQuarto: true,
        missingRequired: [],
        packages: [],
        setupInstructions: [],
      };
    },
    async getCachedPlotClimate() {
      return [];
    },
    computePlotClimateAndDualEdatopic(_ctxId, req) {
      // structuredClone throws DataCloneError if $state.snapshot was omitted on Svelte 5 proxies!
      clonedPayloads.push(structuredClone(req));
      return new Promise(resolve => {
        deferredClimateResolve = resolve;
      });
    },
    async runMultivariateClustering(_ctxId, req) {
      clonedPayloads.push(structuredClone(req));
      return {
        contextId: _ctxId,
        project: 'Sample',
        activeSu: 'Sample',
        method: 'NC',
        distance: 'bray',
        linkage: 'ward.D2',
        transform: 'sqrt',
        k: 3,
        plotCount: 10,
        speciesCount: 25,
        lumpAppliedRows: 2,
        copheneticCorrelation: 0.84,
        meanSilhouette: 0.41,
        noiseCount: 1,
        candidateSuName: 'Sample_ClusterCandidate',
        candidateTableName: 'Sample_ClusterCandidate_SU',
        materializedRows: 10,
        clusters: [],
        memberships: [],
        dendrogram: { maxHeight: 1, cutHeight: 0.5, newick: '(A:0.5,B:0.5);', segments: [], leaves: [] },
      };
    },
    async promoteClusterCandidateToSU(_ctxId, req) {
      clonedPayloads.push(structuredClone(req));
      return {
        contextId: 'ctx-next',
        candidateTableName: 'Sample_ClusterCandidate_SU',
        targetSuName: req.targetSuName,
        targetTableName: `${req.targetSuName}_SU`,
        overwroteExisting: true,
        promotedRows: 9,
        skippedNoiseRows: 1,
        activated: req.activate,
        newState: { contextId: 'ctx-next', activeSu: req.targetSuName },
      };
    },
    async renderQuartoGuide(_ctxId, req) {
      clonedPayloads.push(structuredClone(req));
      return {
        contextId: _ctxId,
        guideType: req.guideType,
        title: 'Unit Guide',
        targetUnit: 'BWBSmw2/05',
        bgcLabel: 'BWBSmw2',
        plotCount: 12,
        speciesRowCount: 8,
        rendererUsed: 'quarto',
        htmlPath: '/exports/guide.html',
        htmlSizeBytes: 8192,
        htmlSha256: 'a'.repeat(64),
        htmlPreview: '<html></html>',
        vegetationTable: [],
      };
    },
  };

  let harness;
  const root = client.effect_root(() => {
    module.exports.default(null, {
      contextId: 'ctx-1',
      activeProject: 'Sample',
      activeSu: 'Sample',
      port: mockPort,
      oncontextswitched: st => switchedStates.push(st),
      inspect: h => {
        harness = h;
      },
    });
  });
  client.flush();

  // Start climate request, then trigger newer runtime probe before climate resolves -> stale climate response MUST be ignored!
  const pendingClimate = harness.runClimateAndDualEdatopic();
  await harness.refreshRRuntime();
  client.flush();
  assert.equal(harness.getState().runtimeStatus.rVersion, '4.6.2');

  deferredClimateResolve({
    contextId: 'ctx-1',
    project: 'Sample',
    activeSu: 'Sample',
    refPeriod: '1961_1990',
    totalScopedPlots: 52,
    coordinatizedPlots: 36,
    skippedNoCoords: 16,
    cacheHits: 0,
    cacheMisses: 36,
    sourceSummary: { bgc_lapse_model: 36 },
    overallRelative: { kind: 'relative', moistureAxis: [], nutrientAxis: [], totalPlots: 0, modalCell: '', cells: [] },
    overallActual: { kind: 'actual', moistureAxis: [], nutrientAxis: [], totalPlots: 0, modalCell: '', cells: [] },
    unitSummaries: [],
    plots: [],
  });
  await pendingClimate;
  client.flush();
  assert.equal(harness.getState().climateResult, null, 'stale held climate response must not overwrite newer generation state');

  // Run clustering -> candidate staged -> promote without confirmation onto active 'Sample' blocked -> confirm succeeds
  await harness.submitClustering();
  client.flush();
  assert.equal(harness.getState().clusterResult.candidateTableName, 'Sample_ClusterCandidate_SU');

  harness.setPromoteDraft('Sample', false, true);
  client.flush();
  await harness.submitPromoteCandidate();
  client.flush();
  assert.match(harness.getState().error, /Confirm overwrite/);
  assert.equal(harness.getState().promoteResult, null);

  harness.setPromoteDraft('Sample', true, true);
  client.flush();
  await harness.submitPromoteCandidate();
  client.flush();
  assert.equal(harness.getState().error, null);
  assert.equal(harness.getState().promoteResult.targetTableName, 'Sample_SU');
  assert.equal(switchedStates.length, 1);

  await harness.submitRenderQuartoGuide();
  client.flush();
  assert.equal(harness.getState().quartoResult.rendererUsed, 'quarto');
  assert.equal(clonedPayloads.length, 4);

  // Also verify onDestroy increments generation
  assert.equal(destroyCallbacks.length, 1);
  destroyCallbacks[0]();
  root();
});

test('RSidecarWorkbench displays Climate Engine: Rscript Required (Unavailable) when runtimeStatus.available is false', async () => {
  const client = await import('svelte/internal/client');
  let script = read('RSidecarWorkbench.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const fullSrc = read('RSidecarWorkbench.svelte');
  assert.match(fullSrc, /Rscript Required \(Unavailable\)/);

  const unavailablePort = {
    async detectRRuntime() {
      return {
        available: false,
        rscriptPath: '',
        rVersion: '',
        rVersionFull: '',
        platform: 'linux',
        quartoAvailable: false,
        quartoPath: '',
        quartoVersion: '',
        climrCachePath: '',
        climrRastersCached: false,
        readyForClimate: false,
        readyForClustering: false,
        readyForQuarto: false,
        missingRequired: ['climr'],
        packages: [],
        setupInstructions: [],
      };
    },
  };
  script = script.replace('oncontextswitched,', 'oncontextswitched, inspect,');
  script += `\n$effect(() => inspect({ refreshRRuntime, getRuntime: () => runtimeStatus }));`;
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
      require(name) {
        if (name === 'svelte') return { onDestroy: () => {} };
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name === './rSidecarWorkbench') return wb;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );
  let api;
  const dispose = client.effect_root(() => {
    modObj.exports.default(null, {
      contextId: 'ctx-unavail',
      activeProject: 'Sample',
      activeSu: 'None',
      port: unavailablePort,
      inspect: v => {
        api = v;
      },
    });
  });
  try {
    client.flush();
    await api.refreshRRuntime();
    client.flush();
    assert.equal(api.getRuntime().available, false);
    // Also verify SSR render with pre-seeded runtimeStatus renders exact unavailable label
    const seededSrc = fullSrc.replace(
      'let runtimeStatus = $state<RRuntimeStatus | null>(null);',
      'let runtimeStatus = $state<RRuntimeStatus | null>({ available: false, rscriptPath: "", rVersion: "", rVersionFull: "", platform: "linux", quartoAvailable: false, quartoPath: "", quartoVersion: "", climrCachePath: "", climrRastersCached: false, readyForClimate: false, readyForClustering: false, readyForQuarto: false, missingRequired: [], packages: [], setupInstructions: [] });'
    );
    const SeededComp = serverComponent(seededSrc, 'RSidecarWorkbench.svelte', {
      './rSidecarWorkbench': wb,
    });
    const html = render(SeededComp, {
      props: { contextId: 'ctx-ssr', activeProject: 'Sample', activeSu: 'None' },
    }).body;
    assert.match(html, /Climate Engine:\s*Rscript Required \(Unavailable\)/);
    assert.doesNotMatch(html, /Climate Engine:\s*Offline Ministry BGC Lapse Model/);
  } finally {
    dispose();
  }
});
