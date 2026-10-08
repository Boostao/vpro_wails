const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const client = require('svelte/internal/client');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const picture = loadTypeScript('pictureRead.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata,
});
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const lookup = loadTypeScript('scopedPlotLookup.ts', { './qualityEditor': quality });
const owner = { contextId: 'owned', project: 'Sample', plotNumber: ' literal plot ' };
function review() {
  return { ...owner, records: {
    columns: ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'].map(name => ({ name, declaredType: 'TEXT' })),
    rows: [
      { rowId: '9223372036854775806', cells: [cell('integer', '42'), cell('text', 'Default'), cell('text', 'Pic01.jpg'), cell('text', owner.plotNumber), cell()] },
      { rowId: '-9', cells: [cell('integer', '42'), cell('text', ''), cell(), cell('text', owner.plotNumber), cell('text', '')] },
    ],
  } };
}
function image(rowId = review().records.rows[0].rowId) {
  return { ...owner, rowId, mime: 'image/jpeg', width: 448, height: 300, sha256: 'a'.repeat(64), dataUrl: 'data:image/jpeg;base64,YQ==' };
}
function host(service = {}) {
  const calls = [], busy = [];
  const context = componentFunctions('PicturePanel.svelte', ['cancel', 'load', 'select', 'preview'], {
    ...owner, view: 'child', ownerKey: 'owner', reviewKey: '', generation: 0, busy: false, disabled: false,
    error: '', review: null, image: null, selected: '', viewerOpen: false, row: null,
    structuredClone, JSON, $state: { snapshot: client.snapshot },
    pictureMetadataFromWire: picture.pictureMetadataFromWire, pictureImageFromWire: picture.pictureImageFromWire,
    onBusyChange(value) { busy.push(value); },
    reads: { track(value) { return value; }, cancelAll() { calls.push('cancel'); } },
    PictureService: {
      GetMetadata(...args) { calls.push(['metadata', ...args]); return service.metadata ? service.metadata(...args) : Promise.resolve(review()); },
      GetImage(...args) { calls.push(['image', ...args]); return service.image ? service.image(...args) : Promise.resolve(image()); },
    },
  });
  return { context, calls, busy };
}

test('picture metadata preserves complete raw storage, duplicate IDs and exact physical64 identities', () => {
  const source = review(), before = JSON.stringify(source);
  const value = picture.pictureMetadataFromWire(source, owner);
  assert.equal(JSON.stringify(value), before);
  assert.equal(value.records.rows[0].rowId, '9223372036854775806');
  assert.equal(value.records.rows[1].cells[0].integer, '42');
  assert.equal(picture.pictureCellLabel(value.records.rows[1].cells[1]), '(empty text)');
  assert.equal(picture.pictureCellLabel(value.records.rows[1].cells[2]), '(NULL)');
  value.records.rows[0].cells[2].text = 'caller';
  assert.equal(JSON.stringify(source), before);
});

test('picture metadata rejects incomplete, foreign, repaired and duplicated physical rows', () => {
  const mutations = [
    value => value.contextId = 'foreign',
    value => value.project = 'foreign',
    value => value.plotNumber = owner.plotNumber.trim(),
    value => value.records.columns.reverse(),
    value => value.records.columns[1].name = 'picdir',
    value => value.records.rows[0].cells.pop(),
    value => value.records.rows[0].cells[3] = cell('integer', '1'),
    value => value.records.rows[0].cells[2].text = '\ud800',
    value => value.records.rows[0].rowId = '09223372036854775806',
    value => value.records.rows[1].rowId = value.records.rows[0].rowId,
    value => value.records.rows = null,
  ];
  for (const change of mutations) {
    const value = review(); change(value);
    assert.throws(() => picture.pictureMetadataFromWire(value, owner));
  }
  const empty = review(); empty.records.rows = [];
  assert.equal(picture.pictureMetadataFromWire(empty, owner).records.rows.length, 0);
});

test('picture preview requires owned measured bounded embedded JPEG/PNG bytes', () => {
  const value = image(), rowId = value.rowId;
  assert.equal(JSON.stringify(picture.pictureImageFromWire(value, owner, rowId)), JSON.stringify(value));
  for (const change of [
    item => item.contextId = 'foreign', item => item.project = 'foreign', item => item.plotNumber = 'foreign',
    item => item.rowId = '-9', item => item.mime = 'image/svg+xml', item => item.width = 0,
    item => item.width = 100000, item => item.height = 100000, item => item.height = 1.5,
    item => item.sha256 = 'unknown', item => item.dataUrl = 'https://example.invalid/photo.jpg',
    item => item.dataUrl = 'data:image/png;base64,YQ==', item => item.dataUrl = 'data:image/jpeg;base64,',
    item => item.dataUrl = 'data:image/jpeg;base64,broken', item => item.dataUrl = 'data:image/jpeg;base64,YQ== ',
  ]) {
    const changed = image(); change(changed);
    assert.throws(() => picture.pictureImageFromWire(changed, owner, rowId));
  }
});

test('picture component loads one owned source and previews exact raw selected row using child policy', async () => {
  const { context, calls, busy } = host();
  await context.actions.load();
  assert.equal(context.review.records.rows.length, 2);
  assert.equal(context.selected, '9223372036854775806');
  context.row = context.review.records.rows[0];
  await context.actions.preview();
  const command = calls.find(call => Array.isArray(call) && call[0] === 'image');
  assert.equal(command[1], owner.contextId);
  assert.equal(command[2], owner.plotNumber);
  assert.deepEqual(JSON.parse(command[3]), { view: 'child', original: review().records.rows[0] });
  assert.equal(context.image.width, 448);
  assert.equal(context.busy, false);
  assert.equal(busy.at(-1), false);
});

test('picture component cancels held metadata and discards late originals without replay', async () => {
  let release;
  const held = new Promise(resolve => release = resolve);
  const { context, calls, busy } = host({ metadata: () => held });
  const pending = context.actions.load();
  assert.equal(context.busy, true);
  context.actions.cancel();
  release(review());
  await pending;
  assert.equal(context.review, null);
  assert.equal(context.error, '');
  assert.equal(context.busy, false);
  assert.equal(busy.at(-1), false);
  assert.equal(calls.filter(call => Array.isArray(call)).length, 1);
});

test('picture preview snapshots actual client-reactive Svelte rows instead of cloning proxies', async () => {
  const { context, calls } = host();
  await context.actions.load();
  context.review = client.proxy(structuredClone(context.review));
  context.row = context.review.records.rows[0];
  assert.throws(() => structuredClone(context.row), { name: 'DataCloneError' });
  await context.actions.preview();
  assert.equal(context.error, '');
  assert.equal(context.image.width, 448);
  const command = calls.find(call => Array.isArray(call) && call[0] === 'image');
  assert.deepEqual(JSON.parse(command[3]).original, review().records.rows[0]);
});

test('picture selection changes invalidate held previews and retain metadata', async () => {
  let release;
  const { context } = host({ image: () => new Promise(resolve => release = resolve) });
  await context.actions.load();
  context.row = context.review.records.rows[0];
  const pending = context.actions.preview();
  assert.equal(context.busy, true);
  context.actions.select('-9');
  release(image());
  await pending;
  assert.equal(context.selected, '-9');
  assert.equal(context.image, null);
  assert.equal(context.review.records.rows.length, 2);
  assert.equal(context.busy, false);
});

test('picture transport/backend errors remain visible and can be explicitly retried', async () => {
  let fail = true;
  const { context } = host({ metadata: () => fail ? Promise.reject(new Error('missing explicit library')) : Promise.resolve(review()) });
  await context.actions.load();
  assert.match(context.error, /missing explicit library/);
  assert.equal(context.review, null);
  fail = false;
  await context.actions.load();
  assert.equal(context.error, '');
  assert.equal(context.review.records.rows.length, 2);
});

test('picture source slot is independently gated and participates in root pending lifecycle', () => {
  const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  const header = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  const panel = readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8');
  assert.match(root, /VITE_PICTURE_READING === 'true'/);
  assert.match(root, /pictureEditor=\{pictureReadingEnabled \? linkedPictures : undefined\}/);
  assert.match(root, /const headerWorkflowBusy = \$derived\([^;]*pictureBusy/);
  assert.match(root, /busy: busy \|\| headerWorkflowBusy/);
  assert.match(root, /plotNumber=\{original\.plotNumber\}/);
  assert.match(header, /control\.controlName === 'frmVPics'/);
  assert.match(header, /Form\.frmVPicsXL/);
  assert.match(panel, /onDestroy\(cancel\)/);
  assert.match(panel, /ondblclick/);
  assert.match(panel, /viewer\.showModal\(\)/);
});

test('picture panel renders honest read-only guidance and cancellation-aware controls without write actions', () => {
  const source = readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8');
  const component = serverComponent(source, 'PicturePanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {} }, './pictureRead': picture,
  });
  const output = render(component, { props: { ...owner, disabled: true, onBusyChange() {} } }).body;
  assert.match(output, /Linked plot pictures/);
  assert.match(output, /Load linked pictures/);
  assert.match(output, /Read-only linked metadata/);
  assert.match(output, /External directories, adding\/deleting records and metadata editing remain unavailable/);
  assert.doesNotMatch(output, />Save|>Add|>Delete/);
});

test('manager previews use explicit manager policy without inheriting child authority', async () => {
  const { context, calls } = host();
  context.view = 'manager';
  await context.actions.load();
  context.row = context.review.records.rows[0];
  await context.actions.preview();
  const command = calls.find(call => Array.isArray(call) && call[0] === 'image');
  assert.deepEqual(JSON.parse(command[3]), { view: 'manager', original: review().records.rows[0] });
  assert.equal(context.image.sha256, image().sha256);
  const source = readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8');
  assert.match(source, /JSON\.stringify\(\[contextId, project, plotNumber, view\]\)/);
});

test('manager policy changes discard held image results rather than displaying another grant', async () => {
  let release;
  const { context } = host({ image: () => new Promise(resolve => release = resolve) });
  context.view = 'manager';
  await context.actions.load();
  context.row = context.review.records.rows[0];
  const pending = context.actions.preview();
  context.view = 'child'; context.ownerKey = 'child-owner';
  release(image());
  await pending;
  assert.equal(context.image, null);
  assert.equal(context.busy, false);
});

test('manager literal parent selection preserves text and explicitly rejects malformed input', () => {
  const context = componentFunctions('PictureManager.svelte', ['selectPlot', 'reading'], {
    plot: '', selected: 'original', error: '', busy: false,
    plotLookupInputError: lookup.plotLookupInputError, onBusyChange() {},
  });
  for (const invalid of ['', '\ud800', '\udc00', '\0']) {
    context.plot = invalid; context.actions.selectPlot();
    assert.equal(context.selected, 'original');
    assert.match(context.error, /complete literal plot/);
  }
  for (const literal of [" A' # ", '00337', '108050', 'É😀', 'historical long plot identifier']) {
    context.plot = literal; context.actions.selectPlot();
    assert.equal(context.selected, literal);
    assert.equal(context.error, '');
  }
});

test('manager pending reads block parent changes and aggregate root busy state', () => {
  const changes = [];
  const context = componentFunctions('PictureManager.svelte', ['selectPlot', 'reading'], {
    plot: 'next', selected: 'original', error: '', busy: false,
    plotLookupInputError: lookup.plotLookupInputError, onBusyChange(value) { changes.push(value); },
  });
  context.actions.reading(true);
  context.actions.selectPlot();
  assert.equal(context.selected, 'original');
  assert.match(context.error, /cancel the current picture read/);
  context.actions.reading(false);
  context.actions.selectPlot();
  assert.equal(context.selected, 'next');
  assert.deepEqual(changes, [true, false]);
});

test('standalone manager has independent default-off navigation and shares root transition/close ownership', () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const nav = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  const manager = readFileSync(path.join(__dirname, 'PictureManager.svelte'), 'utf8');
  assert.match(nav, /VITE_PICTURE_MANAGER === 'true' \? 'picture-manager' : undefined/);
  assert.match(app, /view === 'picture-manager' && import\.meta\.env\.VITE_PICTURE_MANAGER === 'true'/);
  assert.match(app, /if \(import\.meta\.env\.VITE_PICTURE_MANAGER !== 'true'\)/);
  assert.match(app, /<PictureManager[^>]*onBusyChange=\{\(value\) => \{ editorBusy = value; \}\}/s);
  assert.match(manager, /<PicturePanel[^>]*view="manager" onBusyChange=\{reading\}/);
  assert.match(manager, /disabled=\{busy\}/);
  assert.match(manager, /\{#key selected\}/);
  assert.doesNotMatch(manager, /CreatePlot|SwitchContext|\.trim\(|\.toUpperCase\(/);
});

test('manager server render retains source caption, literal parent and separate directory guidance', () => {
  const panel = serverComponent(readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8'), 'PicturePanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {} }, './pictureRead': picture,
  });
  const manager = serverComponent(readFileSync(path.join(__dirname, 'PictureManager.svelte'), 'utf8'), 'PictureManager.svelte', {
    './PicturePanel.svelte': { default: panel }, './scopedPlotLookup': lookup,
  });
  const output = render(manager, { props: { contextId: owner.contextId, project: owner.project,
    initialPlot: owner.plotNumber, onBusyChange() {} } }).body;
  assert.match(output, /Plot Pictures/);
  assert.match(output, /for="picture-manager-plot"/);
  assert.match(output, /Selected literal parent/);
  assert.match(output, /explicitly authorized manager directory, not the FS882 child directory/);
  assert.match(output, /Add a Picture, deletion, metadata editing/);
  assert.doesNotMatch(output, />Save|>Add a Picture|>Delete/);
});
