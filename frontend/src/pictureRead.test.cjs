const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
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
    error: '', review: null, image: null, selected: '', viewerOpen: false, row: null, metadataPending: false, lifecyclePending: false,
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
  assert.match(root, /pictureEditor=\{pictureReadingEnabled && !twoPageOpen \? linkedPictures : undefined\}/);
  assert.match(root, /<TwoPageEntryEditor[\s\S]*?pictureReaderAvailable=\{pictureReadingEnabled\}/);
  const entry = readFileSync(path.join(__dirname, 'TwoPageEntryEditor.svelte'), 'utf8');
  assert.match(entry, /pictureReaderAvailable = false/);
  assert.match(entry, /<TwoPageEntryLayout \{form\} \{pictureReaderAvailable\}/);
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
    './projectMetadataEditor': metadata,
    './pictureLifecycleEditor': require('./pictureLifecycleTestHelpers.cjs').editor,
    './pictureLifecycleSessions': { pictureLifecycleSession: () => { throw new Error('Default-off lifecycle session created'); } },
    './pictureCreationSessions': { pictureCreationSession: () => { throw new Error('Default-off creation session created'); } },
    './PictureMetadataEditor.svelte': { default: () => { throw new Error('Default-off metadata editor rendered'); } },
  });
  const output = render(component, { props: { ...owner, disabled: true, onBusyChange() {} } }).body;
  assert.match(output, /Linked plot pictures/);
  assert.match(output, /Load linked pictures/);
  assert.match(output, /Read-only linked metadata/);
  assert.match(output, /External directories, adding records and metadata editing remain unavailable here/);
  assert.match(output, /Record deletion remains unavailable here/);
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

test('actual complete-entry picture child reuses owned readonly child policy only for the mapped source and reading gate', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  const snippet = form.match(/\{#snippet twoPageLinkedChild\([\s\S]*?\{\/snippet\}/)?.[0];
  assert.ok(snippet);
  const captured = [];
  const panel = serverComponent(readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8'), 'PicturePanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {} }, './pictureRead': picture,
    './projectMetadataEditor': metadata,
    './pictureLifecycleEditor': require('./pictureLifecycleTestHelpers.cjs').editor,
    './pictureLifecycleSessions': { pictureLifecycleSession: () => { throw new Error('No lifecycle authority'); } },
    './pictureCreationSessions': { pictureCreationSession: () => { throw new Error('No creation authority'); } },
    './PictureMetadataEditor.svelte': { default: () => { throw new Error('No metadata editor authority'); } },
  }, { VITE_PICTURE_CREATION_WRITING: 'true', VITE_PICTURE_DELETION_WRITING: 'true', VITE_PICTURE_DELETION_RESTORING: 'true' });
  const component = serverComponent(`<script lang="ts">
    import PicturePanel from './PicturePanel.svelte';
    import SourceChild from './SourceChild.svelte';
    import TwoPageSoilPanel from './TwoPageSoilPanel.svelte';
    import TwoPageVegetationPanel from './TwoPageVegetationPanel.svelte';
    type TwoPageEntryChild = { form: string };
    type PaperControl = { controlId: string };
    let { child, control, pictureReadingEnabled, twoPageScope = 'entry', twoPageProject,
      contextId, draft, twoPageLinkedParentBlocked = false, twoPageSoilBusy = false, twoPageSoilBlocked = false,
      twoPageVegetationBusy = false, twoPageVegetationBlocked = false } = $props();
    const twoPageSoilReviewEnabled = false, twoPageVegetationReviewEnabled = false;
    const ordinaryVegetationUnavailable = null, ordinarySoilUnavailable = null, childRevision = 0;
    let pictureBusy = false;
    function sourceRows() { return []; }
    function linkedVegetationParentBlocked() { return true; }
    const twoPageSoilParentBlocked = true;
  </script>${snippet}{@render twoPageLinkedChild(child, control)}`, 'MappedPictureChildCaller.svelte', {
    './PicturePanel.svelte': { default(payload, props) { captured.push(props); return panel(payload, props); } },
    './SourceChild.svelte': { default: serverComponent('<p data-generic-child>Readonly source metadata</p>', 'ReadonlyPictureFallback.svelte', {}) },
    './TwoPageSoilPanel.svelte': { default: () => { throw new Error('Unrelated soil editor'); } },
    './TwoPageVegetationPanel.svelte': { default: () => { throw new Error('Unrelated vegetation editor'); } },
  });
  for (const sourceForm of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const props = { child: { form: 'frmVPicsXL' }, control: { controlId: `form:${sourceForm}/frmVPics` },
      pictureReadingEnabled: true, twoPageProject: owner.project, contextId: owner.contextId, draft: { plotNumber: owner.plotNumber } };
    const html = render(component, { props }).body;
    assert.match(html, /data-two-page-picture-child="frmVPicsXL"/);
    assert.ok(html.includes(`data-source-control="form:${sourceForm}/frmVPics"`));
    assert.match(html, /data-source-image-control="form:frmVPicsXL\/imgCtl"/);
    assert.equal((html.match(/Load linked pictures/g) ?? []).length, 1);
    assert.doesNotMatch(html, /data-generic-child|Edit selected picture metadata|Review deletion|Save|Add a Picture/);
    const supplied = captured.at(-1);
    assert.equal(supplied.contextId, owner.contextId); assert.equal(supplied.project, owner.project);
    assert.equal(supplied.plotNumber, owner.plotNumber); assert.equal(supplied.view, 'child');
    assert.equal(supplied.metadataSession, undefined); assert.equal(supplied.disabled, false);
    assert.equal(supplied.observeMutationReceipts, false);
    for (const hold of ['twoPageLinkedParentBlocked', 'twoPageSoilBusy', 'twoPageSoilBlocked', 'twoPageVegetationBusy', 'twoPageVegetationBlocked']) {
      const held = render(component, { props: { ...props, [hold]: true } }).body;
      assert.match(held, /<button[^>]* disabled[^>]*>Load linked pictures/);
      assert.equal(captured.at(-1).disabled, true, hold);
    }
    for (const unavailable of [
      { pictureReadingEnabled: false }, { twoPageProject: null },
      { twoPageScope: 'common' }, { twoPageScope: 'extra' }, { child: { form: 'frmVPics' } },
    ]) {
      const before = captured.length;
      const fallback = render(component, { props: { ...props, ...unavailable } }).body;
      assert.match(fallback, /data-generic-child/);
      assert.doesNotMatch(fallback, /data-two-page-picture-child|Load linked pictures/);
      assert.equal(captured.length, before);
    }
  }
});

test('actual ordinary picture snippet is absent while complete-entry owns the source picture surface', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  const expression = form.match(/pictureEditor=\{([^}]+) \? linkedPictures : undefined\}/)?.[1];
  assert.ok(expression);
  const enabled = new Function('pictureReadingEnabled', 'twoPageOpen', `return ${expression};`);
  for (const reading of [false, true]) for (const open of [false, true]) {
    assert.equal(enabled(reading, open), reading && !open);
  }
});

async function mountReceiptReader(disabled, observeMutationReceipts) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('onBusyChange, metadataSession,', 'onBusyChange, metadataSession, inspect,');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, creationSession, lifecycleSession }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('picture-reader.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function').replaceAll('import.meta.env', '__environment');
  const calls = [], subscriptions = [];
  const creationReceipt = { ...owner, requestId: 'retained-acknowledged',
    rowId: review().records.rows[0].rowId, committed: review().records.rows[0] };
  const deleted = structuredClone(review().records.rows[0]);
  deleted.rowId = '999'; deleted.cells[0] = cell('integer', '999');
  const acknowledged = (kind, state) => ({
    subscribe(callback) { subscriptions.push(kind); callback(); return () => {}; },
    view: () => state,
    closeState: () => ({ unsaved: false, busy: false, blocked: false }),
  });
  const created = acknowledged('creation', { receipts: [creationReceipt] });
  const lifecycle = acknowledged('lifecycle', { receipts: [{ kind: 'delete', receipt: { request: { original: deleted } } }], revision: 1 });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'PictureService',
    'pictureMetadataFromWire', 'pictureImageFromWire', 'pictureCellLabel', 'pictureCreationSession',
    'equalCell', 'pictureLifecycleSession', 'pictureLifecycleOriginalMatches', '__environment',
    code + '\nreturn _unknown_;')(
    runtime, onDestroy, runtime.untrack, class { track(value) { return value; } cancelAll() {} },
    { GetMetadata(...args) { calls.push(['metadata', ...args]); return Promise.resolve(review()); } },
    picture.pictureMetadataFromWire, picture.pictureImageFromWire, picture.pictureCellLabel,
    () => { calls.push('creation-session'); return created; }, metadata.equalCell,
    () => { calls.push('lifecycle-session'); return lifecycle; },
    require('./pictureLifecycleTestHelpers.cjs').editor.pictureLifecycleOriginalMatches,
    { VITE_PICTURE_CREATION_WRITING: 'true', VITE_PICTURE_DELETION_WRITING: 'true', VITE_PICTURE_DELETION_RESTORING: 'true' });
  let actions;
  const props = { ...owner, view: 'child', disabled, onBusyChange() {}, inspect: value => { actions = value; } };
  if (observeMutationReceipts !== undefined) props.observeMutationReceipts = observeMutationReceipts;
  const dispose = runtime.effect_root(() => mount(null, props));
  runtime.flush();
  await Promise.resolve(); runtime.flush();
  return { actions: () => actions, calls, subscriptions, dispose, flush: runtime.flush };
}

test('actual client readonly source mount/remount ignores acknowledged mutation receipts with every mutation flag enabled', async () => {
  for (const disabled of [true, false]) for (const remount of [false, true]) {
    const mounted = await mountReceiptReader(disabled, false);
    try {
      assert.deepEqual(mounted.calls, [], `disabled=${disabled}, remount=${remount}`);
      assert.deepEqual(mounted.subscriptions, []);
      assert.equal(mounted.actions().review, null);
      await mounted.actions().load(); mounted.flush();
      assert.equal(mounted.calls.length, disabled ? 0 : 1);
      if (!disabled) assert.deepEqual(mounted.calls[0], ['metadata', owner.contextId, owner.plotNumber]);
    } finally { mounted.dispose(); }
  }
});

test('actual client default picture mounts preserve acknowledged creation/lifecycle receipt refresh behavior', async () => {
  const mounted = await mountReceiptReader(false, undefined);
  try {
    assert.ok(mounted.calls.includes('creation-session'));
    assert.ok(mounted.calls.includes('lifecycle-session'));
    assert.deepEqual(mounted.subscriptions.sort(), ['creation', 'lifecycle']);
    assert.equal(mounted.calls.filter(call => Array.isArray(call) && call[0] === 'metadata').length, 2);
    assert.equal(mounted.actions().review.records.rows.length, 2);
    assert.equal(mounted.actions().error, '');
  } finally { mounted.dispose(); }
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
    plot: '', selected: 'original', error: '', busy: false, metadataBlocked: false, creationBlocked: false, lifecycleBlocked: false,
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
    plot: 'next', selected: 'original', error: '', busy: false, metadataBlocked: false, creationBlocked: false, lifecycleBlocked: false,
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
  assert.match(app, /<PictureManager[^>]*onBusyChange=\{\(value\) => \{ localEditorBusy = value; \}\}/s);
  assert.match(app, /editorBusy = \$derived\(localEditorBusy \|\| pictureCreationBusy \|\| pictureLifecycleBusy \|\| twoPageSoilBusy \|\| twoPageVegetationBusy \|\| externalSUEnvironmentBusy \|\| summaryPreferenceBusy \|\| summaryFieldListBusy \|\| summaryEnvironmentalBusy \|\| projectReportBusy\)/);
  assert.match(manager, /<PicturePanel[^>]*view="manager" onBusyChange=\{reading\}/);
  assert.match(manager, /disabled=\{busy \|\| metadataBlocked \|\| creationBlocked \|\| lifecycleBlocked\}/);
  assert.match(manager, /\{#key selected\}/);
  assert.doesNotMatch(manager, /CreatePlot|SwitchContext|\.trim\(|\.toUpperCase\(/);
});

test('manager server render retains source caption, literal parent and separate directory guidance', () => {
  const panel = serverComponent(readFileSync(path.join(__dirname, 'PicturePanel.svelte'), 'utf8'), 'PicturePanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {} }, './pictureRead': picture,
    './projectMetadataEditor': metadata,
    './pictureLifecycleEditor': require('./pictureLifecycleTestHelpers.cjs').editor,
    './pictureLifecycleSessions': { pictureLifecycleSession: () => { throw new Error('Default-off lifecycle session created'); } },
    './pictureCreationSessions': { pictureCreationSession: () => { throw new Error('Default-off creation session created'); } },
    './PictureMetadataEditor.svelte': { default: () => { throw new Error('Default-off metadata editor rendered'); } },
  });
  const manager = serverComponent(readFileSync(path.join(__dirname, 'PictureManager.svelte'), 'utf8')
    .replace("import.meta.env.VITE_PICTURE_METADATA_WRITING === 'true'", 'false'), 'PictureManager.svelte', {
    './PicturePanel.svelte': { default: panel }, './scopedPlotLookup': lookup,
    './pictureMetadataSessions': { pictureMetadataSession: () => { throw new Error('Default-off metadata session created'); } },
    './pictureCreationSessions': { pictureCreationSession: () => { throw new Error('Default-off creation session created'); } },
    './PictureCreationPanel.svelte': { default: () => { throw new Error('Default-off Add panel rendered'); } },
    './pictureLifecycleSessions': { pictureLifecycleSession: () => { throw new Error('Default-off lifecycle session created'); } },
    './PictureLifecyclePanel.svelte': { default: () => { throw new Error('Default-off lifecycle panel rendered'); } },
  });
  const output = render(manager, { props: { contextId: owner.contextId, project: owner.project,
    initialPlot: owner.plotNumber, onBusyChange() {} } }).body;
  assert.match(output, /Plot Pictures/);
  assert.match(output, /for="picture-manager-plot"/);
  assert.match(output, /Selected literal parent/);
  assert.match(output, /explicitly authorized manager directory, not the FS882 child directory/);
  assert.match(output, /Add a Picture remains unavailable/);
  assert.match(output, /Metadata editing remains unavailable/);
  assert.doesNotMatch(output, /<button[^>]*>\s*(?:Save|Add a Picture|Delete)/);
});
