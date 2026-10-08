const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { render } = require('svelte/server');
const { serverComponent, componentFunctions, loadTypeScript } = require('./svelteTestHelpers.cjs');
const { metadata, cell, transport } = require('./siviParentTestHelpers.cjs');
const picture = loadTypeScript('pictureRead.ts', { './projectMetadataEditor': metadata, './siviParentTransport': transport });
const editor = loadTypeScript('pictureMetadataEditor.ts', {
  './projectMetadataEditor': metadata, './pictureRead': picture,
  '../../resources/picture-metadata-policy.json': require('../../resources/picture-metadata-policy.json'),
});
const owner = { contextId: 'C', project: 'Sample', plotNumber: 'P' };
function draft() {
  return editor.beginPictureMetadataDraft({ ...owner, records: {
    columns: ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'].map(name => ({ name, declaredType: 'TEXT' })),
    rows: [{ rowId: '9223372036854775806', cells: [cell('integer', '-3'), cell('text', 'Default'),
      cell('text', 'Original.jpg'), cell('text', 'P'), cell()] }],
  } }, '9223372036854775806', owner);
}
function source(name) { return readFileSync(path.join(__dirname, name), 'utf8'); }

test('root Save/Lock/close and replacement retain picture errors and require explicit scoped Save', async () => {
  const close = { unsaved: true, busy: false, blocked: true, canSave: false, saveReason: 'retained invalid picture',
    error: 'raw error' };
  const root = componentFunctions('FS882Form.svelte', ['getCloseState', 'load'], {
    pictureMetadataPending: true, pictureMetadataClose: close, siviCreationPending: false, error: null,
  });
  const state = root.actions.getCloseState();
  assert.equal(state.blocked, true);
  assert.equal(state.canSave, false);
  assert.equal(state.error, 'raw error');
  await root.actions.load('another');
  assert.match(root.error, /before replacing their plot owner/);
  close.blocked = false; close.canSave = true;
  assert.match(root.actions.getCloseState().saveReason, /Save picture metadata explicitly/);
});

test('manager retains pending metadata through cancelled picture reads and rejects parent replacement', () => {
  const changes = [];
  const manager = componentFunctions('PictureManager.svelte', ['selectPlot', 'reading'], {
    busy: false, metadataBlocked: true, error: '', selected: 'P', plot: 'other',
    onBusyChange: value => changes.push(value),
  });
  manager.actions.reading(false);
  assert.deepEqual(changes, [true]);
  manager.actions.selectPlot();
  assert.equal(manager.selected, 'P');
  assert.match(manager.error, /retained picture metadata draft/);
});

test('picture editor renders associated source labels, identity and safety feedback before fields', () => {
  const original = draft();
  const retained = editor.stagePictureMetadata(original, 'PicName', '', false);
  const component = serverComponent(source('PictureMetadataEditor.svelte'), 'PictureMetadataEditor.svelte', {
    './pictureMetadataEditor': editor,
  });
  const output = render(component, { props: { session: {
    subscribe: () => () => {},
    view: () => ({ draft: retained, busy: false, blocked: true, error: 'unknown receipt', receipt: null }),
    closeState: () => ({ unsaved: true, busy: false, blocked: true, canSave: false }),
  } } }).body;
  assert.match(output, /Literal parent/);
  assert.match(output, /physical row 9223372036854775806/);
  for (const column of ['PicDir', 'PicName']) {
    assert.match(output, new RegExp(`for="picture-metadata-${column}"`));
    assert.match(output, new RegExp(`id="picture-metadata-${column}"`));
  }
  assert.ok(output.indexOf('unknown receipt') < output.indexOf('picture-metadata-PicDir'));
  assert.ok(output.indexOf('does not undo a completed Save') > output.indexOf('picture-metadata-PicName'));
  assert.match(output, /Resolve picture Save receipt/);
});

test('remounted picture panel exposes retained correction and recovery before its read review is loaded', () => {
  const retained = editor.stagePictureMetadata(draft(), 'PicName', '', false);
  const metadataEditor = serverComponent(source('PictureMetadataEditor.svelte'), 'PictureMetadataEditor.svelte', {
    './pictureMetadataEditor': editor,
  });
  const panel = serverComponent(source('PicturePanel.svelte'), 'PicturePanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {} },
    './readRequests': { ReadRequests: class { cancelAll() {} } },
    './pictureRead': picture, './PictureMetadataEditor.svelte': { default: metadataEditor },
  });
  const output = render(panel, { props: { ...owner, onBusyChange() {}, metadataSession: {
    subscribe: () => () => {},
    view: () => ({ draft: retained, busy: false, blocked: true, error: 'retained remount error',
      receipt: null, originalUpdate: null }),
    closeState: () => ({ unsaved: true, busy: false, blocked: true, canSave: false }),
  } } }).body;
  assert.match(output, /retained remount error/);
  assert.match(output, /id="picture-metadata-PicName"/);
  assert.match(output, /Discard draft and reload picture/);
  assert.match(output, /Resolve picture Save receipt/);
  assert.doesNotMatch(output, /id="picture-record"/);
  assert.match(output, /<button[^>]*disabled[^>]*>Load linked pictures/);
});

test('metadata wiring does not broaden image, creation/deletion or parent write authority', () => {
  const root = source('FS882Form.svelte'), panel = source('PicturePanel.svelte'), manager = source('PictureManager.svelte');
  assert.match(root, /VITE_PICTURE_METADATA_WRITING === 'true'/);
  assert.match(manager, /VITE_PICTURE_METADATA_WRITING === 'true'/);
  assert.match(root, /headerWorkflowBusy = \$derived\(otherHeaderWorkflowBusy \|\| pictureBusy \|\| pictureMetadataPending\)/);
  assert.match(root, /disabled=\{busy \|\| otherHeaderWorkflowBusy \|\| !capabilitiesReady \|\| dirty \|\| childUnsaved\}/);
  assert.match(panel, /metadataPending/);
  assert.match(panel, /if \(!update \|\| update.revision === appliedMetadataRevision\) return/);
  assert.doesNotMatch(source('pictureMetadataSessions.ts'), /CreatePlot|SwitchContext|GetImage|Delete|Restore/);
});

test('every ordinary parent editor waits for picture ownership while retained picture recovery stays reachable', () => {
  const root = source('FS882Form.svelte');
  const expression = root.match(/const headerInputsDisabled = \$derived\(([^;\n]+)\);/)[1];
  const base = { draft: { locked: false }, busy: false, siviSourceBarrier: false, capabilitiesReady: true,
    childUnsaved: false, pictureBusy: false, pictureMetadataPending: false };
  assert.equal(vm.runInNewContext(expression, base), false);
  for (const name of ['pictureBusy', 'pictureMetadataPending', 'childUnsaved', 'busy', 'siviSourceBarrier']) {
    assert.equal(vm.runInNewContext(expression, { ...base, [name]: true }), true, name);
  }
  for (const name of ['ParentCodeFields', 'OrdinaryFields', 'HeaderEditor',
    'SoilCodeFields', 'GeologyCodeFields', 'DrainageFields']) {
    const elements = [...root.matchAll(new RegExp(`<${name}\\b[^>]*`, 'g'))];
    assert.ok(elements.length, name);
    for (const [element] of elements) assert.match(element, /disabled=\{headerInputsDisabled(?: \|\||\})/, name);
  }
  const recovery = root.match(/metadataDisabled=\{(busy \|\| otherHeaderWorkflowBusy[^}\n]+)\}/)[1];
  const state = { busy: false, otherHeaderWorkflowBusy: false, capabilitiesReady: true,
    pictureMetadataPending: false, dirty: true, childUnsaved: false };
  assert.equal(vm.runInNewContext(recovery, state), true);
  assert.equal(vm.runInNewContext(recovery, { ...state, pictureMetadataPending: true }), false);
  assert.equal(vm.runInNewContext(recovery, { ...state, pictureMetadataPending: true, otherHeaderWorkflowBusy: true }), true);
  assert.match(source('PicturePanel.svelte'), /disabled=\{\(metadataDisabled \?\? disabled\) \|\| busy\}/);
  const tabs = root.match(/const tabWorkflowBusy = \$derived\(([^;\n]+)\);/)[1];
  const idle = { otherHeaderWorkflowBusy: false, pictureBusy: false, pictureMetadataClose: { busy: false, unsaved: true, blocked: true } };
  assert.equal(vm.runInNewContext(tabs, idle), false);
  assert.equal(vm.runInNewContext(tabs, { ...idle, pictureMetadataClose: { busy: true } }), true);
  const navigation = root.match(/<nav aria-label=\{siviStandalone[\s\S]*?<\/nav>/)[0];
  assert.doesNotMatch(navigation, /disabled=\{headerWorkflowBusy/);
  assert.equal([...navigation.matchAll(/disabled=\{tabWorkflowBusy/g)].length, 7);
});

test('successful reload synchronizes selectable original and clears preview without replaying a consumed update', () => {
  const old = draft().original;
  const refreshed = structuredClone(old);
  refreshed.cells[2].text = 'CommittedButUnacknowledged.jpg';
  const session = { select: (review, rowId) => {
    assert.equal(rowId, refreshed.rowId);
    assert.equal(review.records.rows[0].cells[2].text, refreshed.cells[2].text);
  } };
  const review = { ...owner, records: { columns: draft().columns, rows: [old] } };
  const panel = componentFunctions('PicturePanel.svelte', ['synchronizeMetadataOriginal', 'editMetadata'], {
    ...owner, ownerKey: 'owned', reviewKey: 'owned', review, metadataSession: session,
    metadataView: { originalUpdate: { ...owner, revision: 1, original: refreshed } },
    appliedMetadataSession: undefined, appliedMetadataRevision: 0,
    image: { sha256: 'old-preview' }, viewerOpen: true, disabled: false, busy: false, metadataPending: false,
  });
  panel.actions.synchronizeMetadataOriginal();
  assert.equal(panel.review.records.rows[0].cells[2].text, refreshed.cells[2].text);
  assert.equal(panel.image, null);
  assert.equal(panel.viewerOpen, false);
  panel.currentReview = panel.review;
  panel.row = panel.review.records.rows[0];
  panel.actions.editMetadata();
  panel.image = { sha256: 'new-preview' };
  panel.actions.synchronizeMetadataOriginal();
  assert.equal(panel.image.sha256, 'new-preview');
  panel.metadataView.originalUpdate.revision++;
  panel.actions.synchronizeMetadataOriginal();
  assert.equal(panel.image, null);

  panel.review = null;
  panel.metadataView.originalUpdate.revision++;
  panel.actions.synchronizeMetadataOriginal();
  const fresh = structuredClone(review);
  fresh.records.rows[0].cells[2].text = 'LaterFreshRead.jpg';
  panel.review = fresh;
  panel.actions.synchronizeMetadataOriginal();
  assert.equal(panel.review.records.rows[0].cells[2].text, 'LaterFreshRead.jpg');
  panel.metadataSession = { select: session.select };
  panel.actions.synchronizeMetadataOriginal();
  assert.equal(panel.review.records.rows[0].cells[2].text, refreshed.cells[2].text);
});
