const assert = require('node:assert/strict');
const { existsSync, readdirSync, readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { componentFunctions } = require('./svelteTestHelpers.cjs');

const frontend = path.resolve(__dirname, '..');
const baselineBytes = 5_202_547;

const deferredViews = [
  'LongEnvironmentReport', 'SiteUnitSummary', 'LongVegetationReport', 'LifeformSummary',
  'SummaryVegetationReport', 'ProjectTableCSVReview', 'UserLogReview', 'SubzoneMatrixReport',
  'PlotLocationReview', 'PictureManager', 'PlotLabelPreview', 'GoogleEarthReview',
];

const deferredFS882SubEditors = [
  'TwoPageEntryEditor', 'TwoPageParentCommonEditor', 'TwoPageParentEditor',
  'TwoPageSoilPanel', 'TwoPageVegetationPanel', 'ProjectMetadataEditor',
  'ProjectPlotProfileReview', 'SpeciesCodeCheck', 'EnvironmentSUTransfer',
  'ExternalSUEnvironmentTransfer',
];

test('entry panels compile as guarded deferred components with explicit loading and failure states', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const compiled = compile(source, { filename: 'App.svelte', generate: 'client', dev: false });
  assert.deepEqual(compiled.warnings, []);
  for (const [component, owner] of [['FS882Form', 'fs882']]) {
    assert.doesNotMatch(source, new RegExp(`import ${component} from`));
    assert.match(compiled.js.code, new RegExp(`import\\(['"]\\./${component}\\.svelte['"]\\)`));
    assert.match(source, new RegExp(`role="status" data-entry-loading="${owner}"`));
    assert.match(source, new RegExp(`role="alert" data-entry-load-error="${owner}"`));
  }
  for (const component of deferredViews) {
    assert.doesNotMatch(source, new RegExp(`import ${component} from`));
    assert.match(compiled.js.code, new RegExp(`import\\(['"]\\./${component}\\.svelte['"]\\)`));
  }
  assert.doesNotMatch(source, /SIVICHARSForm/);
  assert.match(source, /\{#key \$projectState\?\.contextId\}[\s\S]*?\{#key editorEntryForm\}[\s\S]*?\{#key editorPlotNumber\}/);
  assert.equal((source.match(/bind:this=\{editor\}/g) ?? []).length, 1);

  const fs882Source = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  const fs882Compiled = compile(fs882Source, { filename: 'FS882Form.svelte', generate: 'client', dev: false });
  assert.deepEqual(fs882Compiled.warnings, []);
  for (const subEditor of deferredFS882SubEditors) {
    assert.doesNotMatch(fs882Source, new RegExp(`import ${subEditor} from`));
    assert.match(fs882Compiled.js.code, new RegExp(`import\\(['"]\\./${subEditor}\\.svelte['"]\\)`));
  }
});

test('only a resolved module awaiting its editor binding blocks navigation and close', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const expression = source.match(/const editorInitializing = \$derived\(([\s\S]*?)\);/);
  assert.ok(expression);
  const initializing = new Function('view', 'editor', 'fs882ModuleLoaded', `return (${expression[1]});`);
  for (const loaded of [false, true]) {
    assert.equal(initializing('fs882', undefined, loaded), loaded);
    assert.equal(initializing('fs882', {}, loaded), false);
    assert.equal(initializing('home', undefined, loaded), false);
  }
  assert.equal((source.match(/closeDisposition\([^\n]*editorInitializing/g) ?? []).length, 3);
  assert.match(source, /await import\('\.\/FS882Form\.svelte'\);\s*fs882ModuleLoaded = true/);
});

test('failed deferred FS882 sub-editor imports clear open and busy guards instead of wedging close', async () => {
  const host = componentFunctions('FS882Form.svelte', ['loadSubEditor', 'openProjectMetadata', 'canOpenProjectMetadata'], {
    subEditorGeneration: 1,
    error: null,
    successMsg: 'prior',
    metadataEnabled: true,
    twoPageOpen: false,
    childParentDisabled: false,
    childUnsaved: false,
    headerValidation: {},
    metadataOpen: false,
    metadataBusy: false,
  });
  host.actions.openProjectMetadata();
  assert.equal(host.metadataOpen, true);
  assert.equal(host.metadataBusy, true);

  await assert.rejects(
    host.actions.loadSubEditor(
      () => Promise.reject(new Error('network chunk missing')),
      'Project metadata editor',
      () => { host.metadataOpen = false; host.metadataBusy = false; },
    ),
    /network chunk missing/,
  );
  assert.equal(host.metadataOpen, false);
  assert.equal(host.metadataBusy, false);
  assert.match(host.error, /Project metadata editor could not load: Error: network chunk missing/);

  let staleResetCalled = false;
  let rejectStale;
  const stalePromise = host.actions.loadSubEditor(
    () => new Promise((_, reject) => { rejectStale = reject; }),
    'Two-page complete entry',
    () => { staleResetCalled = true; },
  );
  host.subEditorGeneration++;
  host.error = null;
  rejectStale(new Error('late teardown rejection'));
  await assert.rejects(stalePromise, /late teardown rejection/);
  assert.equal(staleResetCalled, false);
  assert.equal(host.error, null);
});

test('actual preview build halves initial transitive JS without removing entry layouts or inflating total JS', async t => {
  const { build } = await import('vite');
  const result = await build({
    root: frontend,
    configFile: path.join(frontend, 'vite.config.ts'),
    mode: 'production',
    logLevel: 'warn',
    build: { write: false, emptyOutDir: false, sourcemap: false },
  });
  assert.ok(!Array.isArray(result) && Array.isArray(result.output));
  const chunks = new Map(result.output.filter(output => output.type === 'chunk')
    .map(chunk => [chunk.fileName, chunk]));
  const entries = [...chunks.values()].filter(chunk => chunk.isEntry);
  assert.equal(entries.length, 1);
  const initial = new Set();
  function visit(name) {
    if (initial.has(name)) return;
    const chunk = chunks.get(name);
    assert.ok(chunk, `Missing static JavaScript dependency ${name}`);
    initial.add(name);
    for (const dependency of chunk.imports) visit(dependency);
  }
  visit(entries[0].fileName);
  const bytes = names => [...names].reduce((sum, name) =>
    sum + Buffer.byteLength(chunks.get(name).code, 'utf8'), 0);
  const entryBytes = Buffer.byteLength(entries[0].code, 'utf8');
  const initialBytes = bytes(initial);
  const totalBytes = bytes(chunks.keys());
  t.diagnostic(JSON.stringify({ baselineBytes, entryBytes, initialBytes, totalBytes,
    initialChunks: [...initial], deferredChunks: chunks.size - initial.size }));
  assert.ok(entryBytes < 500_000, `Initial entry chunk JS ${entryBytes} exceeds 500 KB ceiling`);
  assert.ok(initialBytes < 600_000, `Initial transitive startup JS ${initialBytes} exceeds 600 KB ceiling`);
  assert.ok(totalBytes <= baselineBytes * 3.0, `Total JS across deferred feature chunks ${totalBytes} exceeds budget`);
  const modules = names => new Set([...names].flatMap(name =>
    Object.keys(chunks.get(name).modules).map(id => id.replaceAll('\\', '/'))));
  const eager = modules(initial), all = modules(chunks.keys());
  for (const filename of [
    'FS882Form.svelte',
    'GeospatialWorkbench.svelte',
    ...deferredViews.map(viewName => `${viewName}.svelte`),
    'fs882-xl-layout.json',
    'fs882-two-page-layout.json',
    'fs882-two-page-chars-layout.json',
    'fs1333-sivi-layout.json',
    'project-metadata-layout.json',
  ]) {
    const suffix = '/' + filename;
    assert.ok([...all].some(id => id.endsWith(suffix)), `Required deferred source disappeared: ${filename}`);
    assert.ok(![...eager].some(id => id.endsWith(suffix)), `Deferred source still loads at startup: ${filename}`);
  }
  for (const vendorSegment of ['/node_modules/maplibre-gl/', '/node_modules/pmtiles/', '/node_modules/@deck.gl/', '/node_modules/@luma.gl/']) {
    assert.ok([...all].some(id => id.includes(vendorSegment)), `Expected vendor dependency missing from build: ${vendorSegment}`);
    assert.ok(![...eager].some(id => id.includes(vendorSegment)), `Vendor dependency leaked into startup payload: ${vendorSegment}`);
  }
  for (const vendorChunkPrefix of ['assets/vendor-maplibre-', 'assets/vendor-deckgl-']) {
    const matching = [...chunks.keys()].filter(name => name.startsWith(vendorChunkPrefix));
    assert.equal(matching.length, 1, `Expected dedicated chunk for ${vendorChunkPrefix}`);
    assert.ok(!initial.has(matching[0]), `Dedicated vendor chunk ${matching[0]} must not be in initial startup graph`);
  }
  const findChunkWithModule = suffix => {
    for (const [name, chunk] of chunks.entries()) {
      if (Object.keys(chunk.modules).some(id => id.replaceAll('\\', '/').endsWith(suffix))) return name;
    }
    return null;
  };
  const fs882Chunk = findChunkWithModule('/FS882Form.svelte');
  const twoPageEditorChunk = findChunkWithModule('/TwoPageEntryEditor.svelte');
  const twoPageLayoutChunk = findChunkWithModule('/fs882-two-page-layout.json');
  const twoPageCharsLayoutChunk = findChunkWithModule('/fs882-two-page-chars-layout.json');
  assert.ok(fs882Chunk && twoPageEditorChunk && twoPageLayoutChunk && twoPageCharsLayoutChunk);
  assert.notEqual(twoPageEditorChunk, fs882Chunk, 'TwoPageEntryEditor.svelte must split into a separate chunk from FS882Form.svelte');
  assert.notEqual(twoPageLayoutChunk, fs882Chunk, 'fs882-two-page-layout.json must split into a separate chunk from FS882Form.svelte');
  assert.notEqual(twoPageCharsLayoutChunk, fs882Chunk, 'fs882-two-page-chars-layout.json must split into a separate chunk from FS882Form.svelte');
  const fs882StaticGraph = new Set();
  (function visitFS882(name) {
    if (fs882StaticGraph.has(name)) return;
    fs882StaticGraph.add(name);
    for (const dependency of chunks.get(name).imports) visitFS882(dependency);
  })(fs882Chunk);
  assert.ok(!fs882StaticGraph.has(twoPageEditorChunk), 'FS882Form chunk must not statically import TwoPageEntryEditor chunk');
  assert.ok(!fs882StaticGraph.has(twoPageLayoutChunk), 'FS882Form chunk must not statically import fs882-two-page-layout.json chunk');
  assert.ok(!fs882StaticGraph.has(twoPageCharsLayoutChunk), 'FS882Form chunk must not statically import fs882-two-page-chars-layout.json chunk');
});

test('Red-triaged PC-ORD Break Report and standalone SIVI CHARS stay purged while inline extended shrub layouts remain', () => {
  for (const hostFile of ['Navigation.svelte', 'App.svelte', 'CloseConfirm.svelte']) {
    const source = readFileSync(path.join(__dirname, hostFile), 'utf8');
    for (const retiredPattern of [
      /PC-ORD Break Report/,
      /BreakReportReview/,
      /breakReportReview/,
      /break-report/,
      /SIVICHARSForm/,
      /SIVICHARSParentForm/,
      /siviCHARSParentEntry/,
      /CHARS SIVI parent/,
      /sivi-chars/,
    ]) {
      assert.doesNotMatch(source, retiredPattern, `${hostFile} must not reference ${retiredPattern}`);
    }
  }

  for (const deletedSrcFile of [
    'BreakReportReview.svelte',
    'breakReportReview.ts',
    'breakReportReview.test.cjs',
    'breakReportWire.json',
    'SIVICHARSForm.svelte',
    'SIVICHARSForm.test.cjs',
    'SIVICHARSParentForm.svelte',
    'SIVICHARSParentForm.test.cjs',
    'siviCHARSParentEntry.ts',
    'siviCHARSParentEntry.test.cjs',
    'siviCHARSParentEntrySession.ts',
    'siviCHARSParentEntrySession.test.cjs',
  ]) {
    assert.equal(existsSync(path.join(__dirname, deletedSrcFile)), false, `Retired file must stay deleted: src/${deletedSrcFile}`);
  }

  const testdataDir = path.resolve(frontend, '..', 'testdata');
  for (const deletedFixture of [
    'chars-sivi-entry-client-wire.json',
    'chars-sivi-entry-source.json',
    'chars-sivi-reference-source.json',
  ]) {
    assert.equal(existsSync(path.join(testdataDir, deletedFixture)), false, `Retired fixture must stay deleted: testdata/${deletedFixture}`);
  }

  const resourcesDir = path.resolve(frontend, '..', 'resources');
  assert.equal(existsSync(path.join(resourcesDir, 'fs1333-chars-sivi-layout.json')), false, 'Standalone fs1333-chars-sivi-layout.json must stay deleted');
  for (const retainedLayout of [
    'fs882-two-page-chars-layout.json',
    'fs882-extended-shrub-layout.json',
    'fs1333-extended-shrub-layout.json',
  ]) {
    assert.equal(existsSync(path.join(resourcesDir, retainedLayout)), true, `Active extended shrub layout must remain: ${retainedLayout}`);
  }
});

test('all legacy VITE_* per-slice feature flags remain retired across production frontend sources while releaseChannel gating remains intact', () => {
  assert.equal(existsSync(path.join(__dirname, 'fieldList-env.d.ts')), false, 'fieldList-env.d.ts must stay deleted');
  for (const entry of readdirSync(__dirname)) {
    if (!entry.endsWith('.svelte') && !entry.endsWith('.ts')) continue;
    const source = readFileSync(path.join(__dirname, entry), 'utf8');
    assert.doesNotMatch(source, /VITE_|import\.meta\.env/, `Production file src/${entry} must not contain legacy VITE_* env flags`);
  }
  assert.match(readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8'), /releaseChannel === 'experimental'/);
  assert.match(readFileSync(path.join(__dirname, 'App.svelte'), 'utf8'), /startupInfo\?\.releaseChannel \?\? 'stable'/);
});
