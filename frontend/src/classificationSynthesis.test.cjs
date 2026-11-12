const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const calls = [];
const wailsStub = {
  Call: {
    ByName: (...args) => {
      calls.push(args);
      return Promise.resolve(null);
    },
  },
};

const syn = loadTypeScript('classificationSynthesis.ts', {
  '@wailsio/runtime': wailsStub,
});

const plain = value => JSON.parse(JSON.stringify(value));

function buildFixtureSnapshot() {
  const edatopicCells = [];
  for (const smr of syn.EDATOPIC_SMR_ROWS) {
    for (const snr of syn.EDATOPIC_SNR_COLS) {
      const isCore = smr === '4' && snr === 'C';
      edatopicCells.push({
        smr,
        snr,
        plotCount: isCore ? 5 : 0,
        unitPct: isCore ? 50 : 0,
        prominence: isCore ? 50 : 0,
        plotNumbers: isCore ? ['U01_01', 'U01_02'] : [],
      });
    }
  }

  return {
    contextId: 'ctx-wave4a',
    project: 'Sample',
    su: 'Working',
    suTableName: 'Working_SU',
    unitSourceUsed: 'su',
    suWritable: true,
    lumpTableUsed: 'CombineTest_Lump',
    lumpFoldedTaxa: 2,
    lumpCappedCells: 5,
    significanceBasis: 'all_plots',
    totalProjectPlots: 12,
    classifiedPlotCount: 10,
    unassignedPlotCount: 0,
    excludedPlotCount: 2,
    computeMs: 6.4,
    excludedUnits: [
      { siteUnit: 'CWHvm1/01.support', matchedPattern: '.support', plotCount: 2 },
    ],
    unassignedPlots: [],
    units: [
      {
        siteUnit: 'CWHvm1/01',
        longName: 'HwBa - Blueberry',
        plotCount: 6,
        speciesRichness: 2,
        diagnosticCount: 1,
        constantCount: 1,
        diagPotentialSum: 8.5,
        plots: [],
      },
      {
        siteUnit: 'CWHvm1/05',
        longName: 'BaCw - Foamflower',
        plotCount: 4,
        speciesRichness: 2,
        diagnosticCount: 1,
        constantCount: 1,
        diagPotentialSum: 9.1,
        plots: [],
      },
    ],
    species: [
      {
        species: 'TSUGHET',
        scientificName: 'Tsuga heterophylla',
        englishName: 'western hemlock',
        lifeform: 1,
        lifeformName: 'Conifer Tree',
        stratumGroup: 'Tree',
        sourceSpecies: ['TSUGHET1', 'TSUGHET2'],
        diagnosticUnit: 'CWHvm1/01',
        diagnosis: 'd, cd',
        diagnosticCodes: ['d', 'cd'],
        maxPresencePct: 83.3,
        maxMeanCover: 82.5,
        cells: [
          {
            siteUnit: 'CWHvm1/01',
            presentPlots: 5,
            plotCount: 6,
            presencePct: 83.3,
            presencePctText: '83',
            presenceClass: 5,
            presenceRoman: 'V',
            meanCoverAll: 82.5,
            meanCoverAllText: '82.5',
            meanCoverPresent: 99.0,
            meanCoverPresentText: '99.0',
            signifClass: '9',
            signifNumeric: 9,
            crosstabCode: '5 - 9',
            diagnosticPotential: 8.5,
            isPrimaryDiagnosticUnit: true,
            cellDiagnosticCodes: ['d', 'cd'],
          },
          {
            siteUnit: 'CWHvm1/05',
            presentPlots: 0,
            plotCount: 4,
            presencePct: 0,
            presencePctText: '',
            presenceClass: 0,
            presenceRoman: '',
            meanCoverAll: 0,
            meanCoverAllText: '',
            meanCoverPresent: 0,
            meanCoverPresentText: '',
            signifClass: '',
            signifNumeric: 0,
            crosstabCode: '',
            diagnosticPotential: 0,
            isPrimaryDiagnosticUnit: false,
            cellDiagnosticCodes: [],
          },
        ],
      },
      {
        species: 'HYLOCOM',
        scientificName: 'Hylocomium splendens',
        englishName: 'step moss',
        lifeform: 9,
        lifeformName: 'Moss',
        stratumGroup: 'Moss',
        diagnosticUnit: '',
        diagnosis: '',
        diagnosticCodes: [],
        maxPresencePct: 40.0,
        maxMeanCover: 4.0,
        cells: [
          {
            siteUnit: 'CWHvm1/01',
            presentPlots: 2,
            plotCount: 6,
            presencePct: 33.3,
            presencePctText: '33',
            presenceClass: 2,
            presenceRoman: 'II',
            meanCoverAll: 4.0,
            meanCoverAllText: '4.0',
            meanCoverPresent: 12.0,
            meanCoverPresentText: '12.0',
            signifClass: '3',
            signifNumeric: 3,
            crosstabCode: '2 - 3',
            diagnosticPotential: 0,
            isPrimaryDiagnosticUnit: false,
            cellDiagnosticCodes: [],
          },
          {
            siteUnit: 'CWHvm1/05',
            presentPlots: 1,
            plotCount: 4,
            presencePct: 25.0,
            presencePctText: '25',
            presenceClass: 2,
            presenceRoman: 'II',
            meanCoverAll: 2.0,
            meanCoverAllText: '2.0',
            meanCoverPresent: 8.0,
            meanCoverPresentText: '8.0',
            signifClass: '2',
            signifNumeric: 2,
            crosstabCode: '2 - 2',
            diagnosticPotential: 0,
            isPrimaryDiagnosticUnit: false,
            cellDiagnosticCodes: [],
          },
        ],
      },
    ],
    similarityMatrix: [
      {
        unit: 'CWHvm1/01',
        cells: [
          { unit: 'CWHvm1/01', similarity: 1, similarityPct: 100, similarityPctText: '100.0', becSimMean: 1, becSimMeanPct: 100 },
          { unit: 'CWHvm1/05', similarity: 0.12, similarityPct: 12, similarityPctText: '12.0', becSimMean: 0.08, becSimMeanPct: 8 },
        ],
      },
      {
        unit: 'CWHvm1/05',
        cells: [
          { unit: 'CWHvm1/01', similarity: 0.12, similarityPct: 12, similarityPctText: '12.0', becSimMean: 0.08, becSimMeanPct: 8 },
          { unit: 'CWHvm1/05', similarity: 1, similarityPct: 100, similarityPctText: '100.0', becSimMean: 1, becSimMeanPct: 100 },
        ],
      },
    ],
    pairwiseComparisons: [
      {
        unit1: 'CWHvm1/01',
        unit2: 'CWHvm1/05',
        plotsUnit1: 6,
        plotsUnit2: 4,
        sorensenSimilarity: 0.12,
        sorensenPct: 12.0,
        sharedDiagPotential: 0.5,
        diffTotalUnit1: 5.2,
        diffTotalUnit2: 4.8,
        diffRatio: 0.09,
        becSimMin: 0.07,
        becSimMean: 0.08,
        becSimMax: 0.09,
        differentialTaxa: [],
      },
    ],
    outlierQueue: [
      {
        rank: 1,
        plotNumber: '00337',
        fieldNumber: 'F-337',
        assignedUnit: 'CWHvm1/01',
        assignedUnitPlots: 6,
        singletonUnit: false,
        ownSimilarity: 0.0,
        ownSimilarityPct: 0.0,
        nearestRivalUnit: 'CWHvm1/05',
        rivalSimilarity: 0.96,
        rivalSimilarityPct: 96.0,
        deltaSimilarity: 0.96,
        deltaSimilarityPct: 96.0,
        misplaced: true,
        top5PercentAnomaly: true,
        moistureRegime: '5',
        nutrientRegime: 'D',
        zone: 'CWH',
        subZone: 'vm1',
        siteSeries: '05',
      },
      {
        rank: 2,
        plotNumber: 'U01_01',
        fieldNumber: 'F-01',
        assignedUnit: 'CWHvm1/01',
        assignedUnitPlots: 6,
        singletonUnit: false,
        ownSimilarity: 0.92,
        ownSimilarityPct: 92.0,
        nearestRivalUnit: 'CWHvm1/05',
        rivalSimilarity: 0.08,
        rivalSimilarityPct: 8.0,
        deltaSimilarity: -0.84,
        deltaSimilarityPct: -84.0,
        misplaced: false,
        top5PercentAnomaly: false,
        moistureRegime: '4',
        nutrientRegime: 'C',
        zone: 'CWH',
        subZone: 'vm1',
        siteSeries: '01',
      },
    ],
    overallEdatopicGrid: {
      siteUnit: '__OVERALL__',
      label: 'All Classified Plots',
      totalPlots: 10,
      plottedEdatopicCount: 5,
      missingEdatopicCount: 5,
      maxCellCount: 5,
      maxProminence: 50,
      cells: edatopicCells,
    },
    unitEdatopicGrids: [],
    canUndo: true,
    undoDepth: 1,
    latestUndo: {
      historyId: 4,
      plotNumber: 'U05_02',
      beforeSiteUnit: 'CWHvm1/01',
      afterSiteUnit: 'CWHvm1/05',
      created: '2026-11-12T19:00:00Z',
    },
  };
}

test('defaultSynthesisBackendPort wires all 5 ClassificationService Wails methods', async () => {
  calls.length = 0;
  const opts = syn.createDefaultSynthesisOptions();
  await syn.defaultSynthesisBackendPort.listSynthesisLumpTables('ctx-1');
  await syn.defaultSynthesisBackendPort.computeSynthesisSnapshot('ctx-1', opts);
  await syn.defaultSynthesisBackendPort.reassignSynthesisPlotUnit('ctx-1', {
    plotNumber: '00337',
    targetSiteUnit: 'CWHvm1/05',
    options: opts,
  });
  await syn.defaultSynthesisBackendPort.undoSynthesisReassignment('ctx-1', { options: opts });
  await syn.defaultSynthesisBackendPort.exportDiagnosticWorkbook('ctx-1', {
    outputPath: '/work/diag.xlsx',
    includeCommonNames: true,
    options: opts,
  });

  assert.deepEqual(
    calls.map(c => c[0]),
    [
      'main.ClassificationService.ListSynthesisLumpTables',
      'main.ClassificationService.ComputeSynthesisSnapshot',
      'main.ClassificationService.ReassignSynthesisPlotUnit',
      'main.ClassificationService.UndoSynthesisReassignment',
      'main.ClassificationService.ExportDiagnosticWorkbook',
    ]
  );
});

test('matrix cell formatting, diagnostic fill classes, and grouping modes match BEC rules', () => {
  const snap = buildFixtureSnapshot();
  const tsuCell = snap.species[0].cells[0];

  assert.equal(syn.formatSynthesisMatrixCell(tsuCell, 'roman_signif', 'all_plots'), 'V-9');
  assert.equal(syn.formatSynthesisMatrixCell(tsuCell, 'pct_cover', 'all_plots'), '83% / 82.5');
  assert.equal(syn.formatSynthesisMatrixCell(tsuCell, 'pct_cover', 'present_plots'), '83% / 99.0');
  assert.equal(syn.formatSynthesisMatrixCell(tsuCell, 'crosstab', 'all_plots'), '5 - 9');

  assert.equal(syn.resolveDiagnosticCellClass(['d', 'cd']), 'diag-cell-d-dom');
  assert.equal(syn.resolveDiagnosticCellClass(['dd']), 'diag-cell-dd');
  assert.equal(syn.resolveDiagnosticCellClass(['d']), 'diag-cell-d');
  assert.equal(syn.resolveDiagnosticCellClass(['cd']), 'diag-cell-cd');
  assert.equal(syn.resolveDiagnosticCellClass(['c']), 'diag-cell-c');
  assert.equal(syn.resolveDiagnosticCellClass(['ic']), 'diag-cell-ic');

  const diagSections = syn.filterAndGroupSpeciesRows(snap, {
    searchText: '',
    diagnosticOnly: false,
    grouping: 'diagnostic_unit',
  });
  assert.deepEqual(
    plain(diagSections.map(s => s.title)),
    ['~CWHvm1/01', 'Non-Diagnostic Species']
  );

  const stratumSections = syn.filterAndGroupSpeciesRows(snap, {
    searchText: '',
    diagnosticOnly: false,
    grouping: 'stratum',
  });
  assert.deepEqual(
    plain(stratumSections.map(s => s.title)),
    ['Tree Layer', 'Moss Layer']
  );
});

test('outlier queue filter, 9x6 edatopic matrix, and EqualFold workbook sheet validation', () => {
  const snap = buildFixtureSnapshot();

  assert.equal(syn.filterOutlierQueue(snap.outlierQueue, 'all').length, 2);
  assert.equal(syn.filterOutlierQueue(snap.outlierQueue, 'misplaced').length, 1);
  assert.equal(syn.filterOutlierQueue(snap.outlierQueue, 'misplaced')[0].plotNumber, '00337');

  assert.ok(syn.findPairwiseComparison(snap.pairwiseComparisons, 'CWHvm1/05', 'CWHvm1/01'));

  const edaMatrix = syn.getEdatopicCellMatrix(snap.overallEdatopicGrid);
  assert.equal(edaMatrix.length, 9);
  assert.equal(edaMatrix[0].length, 6);
  assert.equal(edaMatrix[4][2].plotCount, 5); // rSMR 4, rSNR C
  assert.equal(syn.edatopicHeatLevel(50, 5), 4);
  assert.equal(syn.edatopicHeatLevel(0, 0), 0);

  // Valid workbook draft
  assert.equal(
    syn.validateWorkbookExportDraft({
      outputPath: '/work/BEC_Summary.xlsx',
      diagnosticSheetName: 'Diagnostic Summary',
      similaritySheetName: 'Pairwise Similarity',
      outlierSheetName: 'Outlier Queue',
      edatopicSheetName: 'Edatopic Grid',
    }),
    ''
  );

  // Case-insensitive EqualFold collision rejected
  const collisionMsg = syn.validateWorkbookExportDraft({
    outputPath: '/work/BEC_Summary.xlsx',
    diagnosticSheetName: 'Outlier Queue',
    similaritySheetName: 'Pairwise Similarity',
    outlierSheetName: 'outlier queue',
    edatopicSheetName: 'Edatopic Grid',
  });
  assert.match(collisionMsg, /collide/i);
});

test('ClassificationSynthesisView SSR renders cleanly with zero warnings and zero emojis', () => {
  const svelteSource = readFileSync(
    path.join(__dirname, 'ClassificationSynthesisView.svelte'),
    'utf8'
  );
  const tsSource = readFileSync(path.join(__dirname, 'classificationSynthesis.ts'), 'utf8');

  // Strict zero-emoji policy verification
  const emojiRegex = /\p{Extended_Pictographic}/u;
  assert.equal(emojiRegex.test(svelteSource), false, 'Svelte view must contain zero emojis');
  assert.equal(emojiRegex.test(tsSource), false, 'TypeScript module must contain zero emojis');

  const ViewComponent = serverComponent(
    svelteSource,
    'ClassificationSynthesisView.svelte',
    { './classificationSynthesis': syn }
  );

  const snap = buildFixtureSnapshot();
  const rendered = render(ViewComponent, {
    props: {
      contextId: 'ctx-wave4a',
      project: 'Sample',
      su: 'Working',
      initialSnapshot: snap,
    },
  });

  assert.match(rendered.body, /Reactive BEC Differential &amp; Pairwise Synthesis/);
  assert.match(rendered.body, /6\.4 ms/);
  assert.match(rendered.body, /CombineTest_Lump/);
  assert.match(rendered.body, /Tsuga heterophylla/);
  assert.match(rendered.body, /Move to CWHvm1\/05/);
  assert.match(rendered.body, /Undo Move \(1\)/);
  assert.match(rendered.body, /00337/);
});
