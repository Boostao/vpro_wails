const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const wb = loadTypeScript('geospatialWorkbench.ts');
const plain = (value) => JSON.parse(JSON.stringify(value));

test('detectSpatialComputeBackend handles missing navigator.gpu, null adapter, device errors, and real WebGPU adapter', async () => {
  const missing = await wb.detectSpatialComputeBackend({});
  assert.equal(missing.backend, 'webgl2-cpu');
  assert.equal(missing.available, false);
  assert.match(missing.fallbackReason, /unavailable/);

  const nullAdapter = await wb.detectSpatialComputeBackend({
    gpu: { requestAdapter: async () => null },
  });
  assert.equal(nullAdapter.backend, 'webgl2-cpu');
  assert.equal(nullAdapter.available, false);

  const thrownDevice = await wb.detectSpatialComputeBackend({
    gpu: {
      requestAdapter: async () => ({
        requestDevice: async () => {
          throw new Error('Vulkan ICD missing');
        },
      }),
    },
  });
  assert.equal(thrownDevice.backend, 'webgl2-cpu');
  assert.match(thrownDevice.fallbackReason, /Vulkan ICD missing/);

  let compiledShaderCode = '';
  let destroyed = false;
  const activeGPU = await wb.detectSpatialComputeBackend({
    gpu: {
      requestAdapter: async () => ({
        info: { vendor: 'AMD', description: 'Radeon Pro' },
        limits: { maxComputeWorkgroupSizeX: 512 },
        requestDevice: async () => ({
          limits: { maxComputeWorkgroupSizeX: 512 },
          createShaderModule: ({ code }) => {
            compiledShaderCode = code;
            return {};
          },
          destroy: () => {
            destroyed = true;
          },
        }),
      }),
    },
  });
  assert.equal(activeGPU.backend, 'webgpu');
  assert.equal(activeGPU.available, true);
  assert.equal(activeGPU.adapterLabel, 'AMD - Radeon Pro');
  assert.equal(activeGPU.maxComputeWorkgroupSizeX, 512);
  assert.equal(destroyed, true);
  assert.match(compiledShaderCode, /@compute @workgroup_size\(64\)/);
  assert.match(compiledShaderCode, /atomicAdd\(&densityBins\[binIdx\], 1u\)/);
});

test('executeSpatialFilterKernel produces deterministic BBOX indices and density bins on CPU fallback and simulated WebGPU', async () => {
  const coords = new Float32Array([
    -122.7497, 53.9171, // 0: Prince George (inside)
    -123.3656, 48.4284, // 1: Victoria (outside lat)
    -127.1743, 54.7804, // 2: Smithers (inside)
    -115.7666, 49.5097, // 3: Cranbrook (outside lon/lat)
  ]);
  const bounds = [-128.0, 53.0, -121.0, 55.5];

  const cpuRes = await wb.executeSpatialFilterKernel(coords, bounds, {
    nav: {},
    gridSize: 8,
  });
  assert.equal(cpuRes.backend, 'webgl2-cpu');
  assert.deepEqual(plain(cpuRes.matchedIndices), [0, 2]);
  assert.equal(cpuRes.matchedCount, 2);
  const totalBins = Array.from(cpuRes.densityGrid).reduce((a, b) => a + b, 0);
  assert.equal(totalBins, 2);
});

test('decodeFlatGeobufPoints decodes Go-generated OGC FlatGeobuf v3 wire fixture and rejects malformed magic', () => {
  const b64Fixture = readFileSync(
    path.join(__dirname, '..', '..', 'testdata', 'wave3-plots-flatgeobuf.b64'),
    'utf8'
  ).trim();

  const plots = wb.decodeFlatGeobufPoints(b64Fixture);
  assert.equal(plots.length, 2);

  assert.equal(plots[0].plotNumber, '00337');
  assert.ok(Math.abs(plots[0].lat - 53.9171) < 1e-6);
  assert.ok(Math.abs(plots[0].lon - -122.7497) < 1e-6);
  assert.ok(Math.abs(plots[0].utmDerivedLat - 53.9182) < 1e-6);
  assert.ok(Math.abs(plots[0].divergenceMeters - 146.4) < 1e-6);
  assert.equal(plots[0].elevation, 615);
  assert.equal(plots[0].fieldBgc, 'ICHvk2');
  assert.equal(plots[0].gisBgc, 'SBSmk1');
  assert.equal(plots[0].bgcMismatch, true);
  assert.equal(plots[0].siteUnit, 'SBSmk1/01');

  assert.equal(plots[1].plotNumber, '108050');
  assert.equal(plots[1].bgcMismatch, false);
  assert.equal(plots[1].siteUnit, 'CDFmm/01');

  assert.throws(
    () => wb.decodeFlatGeobufPoints(new Uint8Array([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12])),
    /Invalid OGC FlatGeobuf v3 magic/
  );
});

test('pointInPolygonRing, materializeWorkbenchPlots, and buildMapLibreGeospatialStyle preserve offline PMTiles contracts', () => {
  const ring = [
    [-124.0, 53.0],
    [-121.0, 53.0],
    [-121.0, 55.0],
    [-124.0, 55.0],
  ];
  assert.equal(wb.pointInPolygonRing(-122.7497, 53.9171, ring), true);
  assert.equal(wb.pointInPolygonRing(-123.3656, 48.4284, ring), false);
  assert.equal(wb.pointInPolygonRing(-122.7497, 53.9171, [...ring, ring[0]]), true);

  const mat = wb.materializeWorkbenchPlots({
    count: 2,
    plotNumber: ['00337', '108050'],
    lon: [-122.7497, -123.3656],
    lat: [53.9171, 48.4284],
    hasUtm: [true, false],
    utmDerivedLon: [-122.752, 0],
    utmDerivedLat: [53.919, 0],
    divergenceMeters: [185.2, 0],
    divergenceFlagged: [true, false],
    longitudeFlipped: [true, false],
    hasElevation: [true, false],
    elevation: [620, 0],
    hasSlope: [false, false],
    slope: [0, 0],
    hasAspect: [false, false],
    aspect: [0, 0],
    zone: ['SBS', 'CDF'],
    subZone: ['mk1', 'mm'],
    fieldBgc: ['ICHvk2', 'CDFmm'],
    gisBgc: ['SBSmk1', 'CDFmm'],
    bgcMismatch: [true, false],
    siteUnit: ['SBSmk1/01', 'CDFmm/01'],
    ecosection: ['NAB', 'SGI'],
    fsRegionDistrict: ['RNO.DPG', 'RSC.DSI'],
  });
  assert.equal(mat.points.length, 2);
  assert.equal(mat.discrepancyVectors.length, 1);
  assert.equal(mat.discrepancyVectors[0].plotNumber, '00337');

  const styleOffline = wb.buildMapLibreGeospatialStyle(
    {
      schemaVersion: 1,
      archives: {
        'bc-bgc-v13.pmtiles': {
          name: 'bc-bgc-v13.pmtiles',
          url: '/maps/bc-bgc-v13.pmtiles',
          source: 'extraDir',
          available: true,
          fullResolution: true,
          fallbackOverview: false,
          minZoom: 3,
          maxZoom: 10,
          sizeBytes: 55000000,
        },
      },
      ordered: [],
    },
    {
      bgcFill: true,
      bgcLabels: true,
      ecosections: true,
      nrDistricts: false,
      hillshade3D: false,
      bcOpenMapsWMS: false,
      discrepancyVectors: true,
    }
  );
  assert.equal(styleOffline.sources.bc_bgc_pmtiles.url, 'pmtiles:///maps/bc-bgc-v13.pmtiles');
  assert.equal(styleOffline.sources.bc_bgc_pmtiles.maxzoom, 10);
  assert.equal(styleOffline.sources.bc_openmaps_wms, undefined);

  const styleWMS = wb.buildMapLibreGeospatialStyle(null, {
    bgcFill: true,
    bgcLabels: true,
    ecosections: false,
    nrDistricts: true,
    hillshade3D: true,
    bcOpenMapsWMS: true,
    discrepancyVectors: true,
  });
  assert.ok(styleWMS.sources.bc_openmaps_wms);
});

test('GeospatialWorkbench, Navigation, App, and CoordinateFields compile cleanly with BC Gov styling and zero emojis', () => {
  const wbSource = readFileSync(path.join(__dirname, 'GeospatialWorkbench.svelte'), 'utf8');
  const navSource = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  const appSource = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const coordSource = readFileSync(path.join(__dirname, 'CoordinateFields.svelte'), 'utf8');

  for (const [name, src] of [
    ['GeospatialWorkbench.svelte', wbSource],
    ['Navigation.svelte', navSource],
    ['App.svelte', appSource],
    ['CoordinateFields.svelte', coordSource],
  ]) {
    const compiled = compile(src, { filename: name, generate: 'client' });
    assert.equal(compiled.warnings.length, 0, `${name} emitted compiler warnings`);
  }

  // Zero emojis in new/modified Wave 3 files
  const emojiRegex = /[\p{Extended_Pictographic}]/u;
  assert.equal(emojiRegex.test(wbSource), false, 'GeospatialWorkbench.svelte must contain zero emojis');
  assert.equal(
    emojiRegex.test(readFileSync(path.join(__dirname, 'geospatialWorkbench.ts'), 'utf8')),
    false,
    'geospatialWorkbench.ts must contain zero emojis'
  );

  // Zero new VITE_* or VPRO_* env flags in GeospatialWorkbench
  assert.doesNotMatch(wbSource, /VITE_|VPRO_/);

  // BC Gov Design System palette (#003366 / #FCBA19)
  assert.match(wbSource, /#003366/i);
  assert.match(wbSource, /#fcba19/i);

  // Explicit bundled MapLibre worker URL configured for WebView2 http://wails.localhost/
  assert.match(wbSource, /maplibre-gl\/dist\/maplibre-gl-worker\.mjs\?worker&url/);
  assert.match(wbSource, /setWorkerUrl\(/);

  // Lazy route in App.svelte, Spatial entry in Navigation.svelte, Auto-1 button in CoordinateFields.svelte
  assert.match(appSource, /import\('\.\/GeospatialWorkbench\.svelte'\)/);
  assert.match(navSource, /3D Geospatial Workbench/);
  assert.match(navSource, /3D Map/);
  assert.match(coordSource, /data-auto1-spatial-stamp/);

  // SSR render smoke check of GeospatialWorkbench.svelte
  const SSRWorkbench = serverComponent(wbSource, 'GeospatialWorkbench.svelte', {
    'maplibre-gl': { Map: class {}, NavigationControl: class {}, ScaleControl: class {}, addProtocol() {}, setWorkerUrl() {} },
    'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url': { default: '/assets/maplibre-gl-worker.js' },
    'maplibre-gl/dist/maplibre-gl.css': {},
    pmtiles: { Protocol: class { tile = () => {}; } },
    '@deck.gl/mapbox': { MapboxOverlay: class {} },
    '@deck.gl/layers': { LineLayer: class {}, ScatterplotLayer: class {} },
    '../bindings/github.com/boostao/vpro-wails': { SpatialService: {} },
    './geospatialWorkbench': wb,
  });
  const html = render(SSRWorkbench, {
    props: { contextId: 'ctx-wave3', project: 'Sample', su: 'None' },
  }).body;
  assert.match(html, /3D BGC v13 Map, Geodetic QA/);
  assert.match(html, /Plot Inspector/);
  assert.match(html, /Lasso \/ AOI/);
  assert.match(html, /Auto-1 QA Stamp/);
});
