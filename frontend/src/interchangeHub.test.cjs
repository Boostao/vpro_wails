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
  // Single-plot <Plot_package> must count strictly 1 <Plot>, never matching <Plot_package> or <Plots>, even across tiny chunk boundaries
  const singlePkg = '<Plot_package><Plots><Plot releve_nr="1"><header_data><standard_record releve_nr="1"/></header_data></Plot></Plots></Plot_package>';
  const singleRes = await hub.validateXMLSnippetChunked(singlePkg, 7, async () => {});
  assert.equal(singleRes.valid, true);
  assert.equal(singleRes.plotTagCount, 1);

  let yields = 0;
  const bigXml = '<Plot_package><Plots>' + '<Plot releve_nr="1"><header_data/></Plot>'.repeat(120) + '</Plots></Plot_package>';
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
  const badSurrogate = await hub.validateXMLSnippetChunked('<Plot_package>\uD800</Plot_package>', 512);
  assert.equal(badSurrogate.valid, false);
  assert.match(badSurrogate.error, /surrogate/);

  // Canonical Alterra Turboveg 3 crosswalk documents 9 = ml (Cryptogam) and 10 = ep (Epiphyte)
  assert.ok(hub.TURBOVEG_LAYER_CROSSWALK.some((cw) => cw.tvGroup.includes('9 = ml')));
  assert.ok(hub.TURBOVEG_LAYER_CROSSWALK.some((cw) => cw.tvGroup.includes('10 = ep')));
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
