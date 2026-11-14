const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, srcPath } = require('./svelteTestHelpers.cjs');

const calls = [];
const mockRuntime = {
  Call: {
    ByName: (name, ...args) => {
      calls.push([name, ...args]);
      return Promise.resolve(null);
    },
  },
};

const diag = loadTypeScript('lmh25Diagnostics.ts', { '@wailsio/runtime': mockRuntime });

function sampleDiagWire(overrides = {}) {
  return {
    plotNumber: '00337',
    derivedHumusThickness: 12.0,
    headerHumusThickness: 8.0,
    humusThicknessDiff: -4.0,
    substratePopulated: 5,
    substrateSum: 95.0,
    substrateValid: false,
    isOrganicSoilOrder: false,
    keyedSmr: '6',
    keyedSmrClass: 6,
    fieldSmr: '3',
    fieldSmrClass: 3,
    smrDivergence: 3,
    smrDivergent: true,
    smrTrace: [{ key: '9.4', step: 'Lower/Toe', condition: 'WSI >= 30 cm', outcome: '6' }],
    keyedSnr: 'D',
    keyedSnrClass: 3,
    fieldSnr: 'B',
    fieldSnrClass: 1,
    snrDivergence: 2,
    snrDivergent: true,
    snrTrace: [{ key: '9.6', step: 'Moving water', condition: 'Ah <= 5 cm', outcome: 'D' }],
    issues: [
      { code: 'SUBSTRATE_SUM_NOT_100', severity: 'warning', category: 'substrate', message: 'Sum is 95%', lmh25Ref: 'LMH25 pp. 40-41' },
      { code: 'HORIZON_SIGN_UPLAND_LFH', severity: 'error', category: 'soil_horizon', message: 'UpperDepth <= LowerDepth', lmh25Ref: 'LMH25 p. 71' },
    ],
    ...overrides,
  };
}

test('substrateProgressPercent, substrateStatusLabel, and formatHumusComparison format edge cases accurately', () => {
  assert.equal(diag.substrateProgressPercent(null), 0);
  assert.equal(diag.substrateProgressPercent(-5), 0);
  assert.equal(diag.substrateProgressPercent(95.25), 95.25);
  assert.equal(diag.substrateProgressPercent(112.0), 100);

  const wire = diag.normalizeDiagnosticsWire(sampleDiagWire());
  assert.equal(diag.substrateStatusLabel(wire), '95.0% (Expected 100%)');
  assert.match(diag.formatHumusComparison(wire), /Derived 12\.0 cm vs Header 8\.0 cm \(-4\.0 cm\)/);

  const validWire = diag.normalizeDiagnosticsWire(
    sampleDiagWire({ substrateSum: 100.0, substrateValid: true, headerHumusThickness: 12.0, humusThicknessDiff: 0 })
  );
  assert.equal(diag.substrateStatusLabel(validWire), '100.0% (Valid 100%)');
  assert.equal(diag.formatHumusComparison(validWire), '12.0 cm (Matches Header)');

  assert.deepEqual(JSON.parse(JSON.stringify(diag.countDiagnosticsBySeverity(wire.issues))), {
    errors: 1,
    warnings: 1,
    infos: 0,
  });
});

test('normalizeDiagnosticsWire rejects malformed payloads and default port calls PlotEntryService', async () => {
  assert.throws(() => diag.normalizeDiagnosticsWire(null));
  assert.throws(() => diag.normalizeDiagnosticsWire({ plotNumber: 123 }));

  calls.length = 0;
  await diag.defaultLMH25DiagnosticsPort.evaluatePlotDiagnostics({ plotNumber: '00337' }, [], []);
  await diag.defaultLMH25DiagnosticsPort.evaluateStoredPlotDiagnostics('ctx-1', '00337');
  assert.equal(calls[0][0], 'main.PlotEntryService.EvaluatePlotDiagnostics');
  assert.equal(calls[1][0], 'main.PlotEntryService.EvaluateStoredPlotDiagnostics');
});

test('LMH25DiagnosticsBar.svelte compiles cleanly, contains zero emojis, and renders BC Gov banner', () => {
  const source = readFileSync(srcPath('LMH25DiagnosticsBar.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(source), false);
  assert.match(source, /#003366/);
  assert.match(source, /#fcba19/i);

  const Component = serverComponent(source, 'LMH25DiagnosticsBar.svelte', {
    './lmh25Diagnostics': diag,
  });
  const rendered = render(Component, {
    props: {
      contextId: 'ctx-1',
      plotNumber: '00337',
      header: { plotNumber: '00337', moistureRegime: '4' },
    },
  });
  assert.match(rendered.body, /LMH25 \(2023\)/);
  assert.match(rendered.body, /Evaluate LMH25 Rules/);
});
