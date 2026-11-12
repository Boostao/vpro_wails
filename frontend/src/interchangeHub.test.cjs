const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const calls = [];
const mockRuntime = {
  Call: {
    ByName: (name, ...args) => {
      calls.push([name, ...args]);
      return Promise.resolve(null);
    },
  },
};

const hub = loadTypeScript('interchangeHub.ts', { '@wailsio/runtime': mockRuntime });

test('validateTargetProjectName rejects None, Sample, master, and non-identifiers', () => {
  assert.deepEqual(JSON.parse(JSON.stringify(hub.validateTargetProjectName('SkeenaImport_2026'))), {
    valid: true,
    normalized: 'SkeenaImport_2026',
    error: '',
  });
  assert.equal(hub.validateTargetProjectName('Sample').valid, false);
  assert.equal(hub.validateTargetProjectName('None').valid, false);
  assert.equal(hub.validateTargetProjectName('BECMasterClone').valid, false);
  assert.equal(hub.validateTargetProjectName('99Invalid').valid, false);
});

test('validateXMLSnippetChunked yields cooperatively across chunks and aborts when cancelled mid-yield', async () => {
  let yields = 0;
  const bigXml = '<Turboveg><Plots>' + '<Plot plot_number="001"><Header/></Plot>'.repeat(120) + '</Plots></Turboveg>';
  const ok = await hub.validateXMLSnippetChunked(
    bigXml,
    512,
    async () => {
      yields++;
    },
    () => false
  );
  assert.equal(ok.valid, true);
  assert.equal(ok.plotTagCount, 120);
  assert.ok(yields >= 8, `expected multiple chunk yields, got ${yields}`);

  // Held yield released after remount/cancel: must reject and never produce stale state
  let releaseGate;
  const gate = new Promise((resolve) => {
    releaseGate = resolve;
  });
  let cancelled = false;
  const pending = hub.validateXMLSnippetChunked(
    bigXml,
    512,
    () => gate,
    () => cancelled
  );
  cancelled = true;
  releaseGate();
  await assert.rejects(pending, /cancelled/);

  // Malformed surrogate in XML is rejected cleanly
  const badSurrogate = await hub.validateXMLSnippetChunked('<Turboveg>\uD800</Turboveg>', 512);
  assert.equal(badSurrogate.valid, false);
  assert.match(badSurrogate.error, /surrogate/);
});

test('defaultInterchangeHubPort invokes all expected ImportExportService methods via Call.ByName', async () => {
  calls.length = 0;
  await hub.defaultInterchangeHubPort.inspectVenusSource('ctx-1', '/a/venus.mdb');
  await hub.defaultInterchangeHubPort.exportTurboVeg('ctx-1', { format: 'sqlite', destinationPath: '/a/tv.sqlite' });
  await hub.defaultInterchangeHubPort.exportUserSpeciesList('ctx-1', { format: 'json' });
  await hub.defaultInterchangeHubPort.exportLongTidyCSV('ctx-1', {});
  await hub.defaultInterchangeHubPort.exportWideMatrixCSV('ctx-1', { layerMode: 'rollup_all', zeroFill: true, includeSiteUnit: true });
  await hub.defaultInterchangeHubPort.exportProjectRDS('ctx-1', { destinationPath: '/a/out.rds' });
  await hub.defaultInterchangeHubPort.exportGeoPackage('ctx-1', {
    destinationPath: '/a/out.gpkg',
    crs: 'EPSG:3005',
    includeAttributeTables: true,
    includeUnlocatedPlots: false,
  });

  assert.deepEqual(
    calls.map((c) => c[0]),
    [
      'main.ImportExportService.InspectVenusSource',
      'main.ImportExportService.ExportTurboVeg',
      'main.ImportExportService.ExportUserSpeciesList',
      'main.ImportExportService.ExportLongTidyCSV',
      'main.ImportExportService.ExportWideMatrixCSV',
      'main.ImportExportService.ExportProjectRDS',
      'main.ImportExportService.ExportGeoPackage',
    ]
  );
});

test('InterchangeHub.svelte compiles cleanly, contains zero emojis, and renders BC Gov 5-tab workbench', () => {
  const source = readFileSync(path.join(__dirname, 'InterchangeHub.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(source), false);
  assert.match(source, /#003366/);
  assert.match(source, /#fcba19/i);

  const Component = serverComponent(source, 'InterchangeHub.svelte', { './interchangeHub': hub });
  const rendered = render(Component, { props: { contextId: 'ctx-hub' } });
  assert.match(rendered.body, /VENUS 5\.0/);
  assert.match(rendered.body, /TurboVeg 3 \/ XML/);
  assert.match(rendered.body, /User Species List/);
  assert.match(rendered.body, /GeoPackage \(\.gpkg\)/);
});
