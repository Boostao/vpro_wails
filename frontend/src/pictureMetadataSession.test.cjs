const assert = require('node:assert/strict');
const { test } = require('node:test');
const { pathToFileURL } = require('node:url');
const path = require('node:path');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { cell, metadata, transport } = require('./siviParentTestHelpers.cjs');
const picture = loadTypeScript('pictureRead.ts', { './siviParentTransport': transport, './projectMetadataEditor': metadata });
const editor = loadTypeScript('pictureMetadataEditor.ts', {
  '../../resources/picture-metadata-policy.json': require('../../resources/picture-metadata-policy.json'),
  './projectMetadataEditor': metadata, './pictureRead': picture,
});
const reads = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const session = loadTypeScript('pictureMetadataSession.ts', {
  './readRequests': reads, './projectMetadataEditor': metadata, './pictureRead': picture, './pictureMetadataEditor': editor,
});
const owner = { contextId: 'C', project: 'Sample', plotNumber: 'P' };
const token = '00000000-0000-4000-8000-000000000001';
const copy = value => JSON.parse(JSON.stringify(value));
function review() {
  return { ...owner, records: {
    columns: ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'].map(name => ({ name, declaredType: 'TEXT' })),
    rows: [{ rowId: '9223372036854775806', cells: [
      cell('integer', '-2147483648'), cell('text', 'Default'), cell('text', 'Original.jpg'), cell('text', 'P'), cell(),
    ] }],
  } };
}
function receipt(request) {
  const committed = copy(request.original);
  for (const change of request.changes) {
    committed.cells[request.columns.findIndex(column => column.name === change.column)] = copy(change.value);
  }
  return { ...copy(request), ...owner, requestedSource: 'owned-requested', source: 'owned-source',
    ownedFiles: { 'attachment:project': 'owned-project', 'support:VLists': 'owned-support' }, actor: 'user',
    auditStrength: 3, committed, editWhen: '2026-10-07T19:20:00.123456789Z',
    historyId: '9223372036854775807', didCommit: true, replayed: false };
}
function resolved(value) {
  const promise = Promise.resolve(value);
  promise.cancel = () => {};
  return promise;
}
function fixture(overrides = {}) {
  const calls = { save: 0, read: 0, receipt: 0, request: null };
  const port = {
    read: () => { calls.read++; return resolved(review()); },
    save: json => { calls.save++; calls.request = JSON.parse(json); return resolved(receipt(calls.request)); },
    receipt: json => { calls.receipt++; return resolved(receipt(JSON.parse(json))); },
    ...overrides,
  };
  const current = new session.PictureMetadataSession(owner, port);
  current.select(review(), review().records.rows[0].rowId);
  return { current, calls, port };
}

test('picture request preserves all five columns/full original and large physical identity with stable token', () => {
  let draft = editor.beginPictureMetadataDraft(review(), review().records.rows[0].rowId, owner);
  draft = editor.stagePictureMetadata(draft, 'PicName', ' literal.jpg ', false);
  const request = editor.pictureMetadataRequest(draft, token, owner);
  assert.equal(request.requestId, token);
  assert.equal(request.id, -2147483648);
  assert.equal(request.original.rowId, '9223372036854775806');
  assert.equal(request.columns.length, 5);
  assert.equal(request.original.cells.length, 5);
  request.original.cells[2].text = 'changed';
  request.columns[0].name = 'changed';
  assert.equal(draft.original.cells[2].text, 'Original.jpg');
  assert.equal(draft.columns[0].name, 'ID');
  assert.throws(() => editor.pictureMetadataRequest(draft, 'missing', owner), /stable request/);
  assert.throws(() => editor.pictureMetadataRequest(draft, token, { ...owner, project: 'foreign' }), /exact active owner/);
  draft.id = 1;
  assert.throws(() => editor.pictureMetadataRequest(draft, token, owner), /identity changed/);
});

test('complete source-bound durable receipt adopts only planned cells and detaches original/view state', async () => {
  const { current, calls } = fixture();
  current.stage('PicName', ' renamed.jpg ', false);
  assert.equal(current.closeState().unsaved, true);
  assert.equal(await current.save(token), true);
  assert.equal(calls.save, 1);
  assert.equal(current.closeState().unsaved, false);
  assert.equal(current.view().draft.original.cells[2].text, ' renamed.jpg ');
  assert.equal(current.view().receipt.historyId, '9223372036854775807');
  assert.equal(current.view().originalUpdate.revision, 1);
  assert.equal(current.view().originalUpdate.original.cells[2].text, ' renamed.jpg ');
  const view = current.view();
  view.draft.original.cells[2].text = 'tampered';
  view.receipt.committed.cells[2].text = 'tampered';
  view.originalUpdate.original.cells[2].text = 'tampered';
  assert.equal(current.view().draft.original.cells[2].text, ' renamed.jpg ');
  assert.equal(current.view().originalUpdate.original.cells[2].text, ' renamed.jpg ');
});

test('discard after lost commit publishes a fresh scoped original only after successful owned reload', async () => {
  const stored = review();
  stored.records.rows[0].cells[2].text = 'committed-with-lost-receipt.jpg';
  const { current, calls, port } = fixture({
    save: () => { calls.save++; return resolved(null); },
    read: () => { calls.read++; return resolved(stored); },
  });
  current.stage('PicName', stored.records.rows[0].cells[2].text, false);
  assert.equal(await current.save(token), false);
  assert.equal(current.view().originalUpdate, null);
  assert.equal(await current.discardAndReload(), true);
  const update = current.view().originalUpdate;
  assert.deepEqual({ contextId: update.contextId, project: update.project, plotNumber: update.plotNumber }, owner);
  assert.equal(update.revision, 1);
  assert.equal(update.original.cells[2].text, stored.records.rows[0].cells[2].text);
  assert.equal(current.view().receipt, null);
  assert.equal(current.closeState().unsaved, false);
  port.read = () => resolved({ ...stored, contextId: 'foreign' });
  assert.equal(await current.discardAndReload(), false);
  assert.deepEqual(current.view().originalUpdate, update);
  port.read = () => resolved(stored);
  assert.equal(await current.discardAndReload(), true);
  assert.equal(current.view().originalUpdate.revision, 2);
  assert.equal(calls.save, 1);
});

test('incomplete/conflicting receipt shapes never establish success, including unplanned comment/identity changes', () => {
  const draft = editor.stagePictureMetadata(editor.beginPictureMetadataDraft(review(), review().records.rows[0].rowId, owner),
    'PicName', 'new.jpg', false);
  const request = editor.pictureMetadataRequest(draft, token, owner);
  for (const change of [
    wire => delete wire.requestId, wire => wire.contextId = 'foreign', wire => wire.id = 2,
    wire => wire.original.cells[2].text = 'foreign', wire => wire.columns[0].name = 'foreign',
    wire => wire.changes = [], wire => wire.historyId = '0', wire => wire.historyId = '9223372036854775808',
    wire => wire.editWhen = '', wire => wire.actor = '', wire => wire.source = '', wire => wire.ownedFiles = {},
    wire => wire.ownedFiles.profile = null, wire => wire.auditStrength = 4, wire => wire.didCommit = false,
    wire => wire.committed.rowId = '1', wire => wire.committed.cells[0] = cell('integer', '1'),
    wire => wire.committed.cells[4] = cell('text', 'unplanned'), wire => wire.committed.cells[2].text = 'other.jpg',
  ]) {
    const wire = receipt(request); change(wire);
    assert.throws(() => session.pictureMetadataReceiptFromWire(wire, request, owner));
  }
});

test('errors and unknown state survive remount/view; readonly history resolution never repeats the write', async () => {
  const { current, calls, port } = fixture({ save: () => { calls.save++; return resolved(null); } });
  current.stage('PicName', '', false);
  current.stage('PicDir', 'valid directory', false);
  assert.equal(current.view().draft.cells.PicName.raw, '');
  assert.equal(current.closeState().blocked, true);
  await assert.rejects(current.save(token), /permits NULL/);
  current.stage('PicName', 'new.jpg', false);
  let notified = 0;
  const stop = current.subscribe(() => notified++);
  assert.equal(await current.save(token), false);
  stop();
  const before = current.view();
  const remount = current.subscribe(() => notified++);
  assert.equal(current.view().error, before.error);
  assert.equal(current.view().draft.cells.PicName.raw, 'new.jpg');
  assert.equal(current.closeState().blocked, true);
  await assert.rejects(current.save(token), /Resolve/);
  assert.throws(() => current.select(review(), review().records.rows[0].rowId), /retained picture draft/);
  port.receipt = () => { calls.receipt++; return resolved(null); };
  assert.equal(await current.resolve(), false);
  assert.equal(current.closeState().blocked, true);
  port.receipt = json => { calls.receipt++; return resolved(receipt(JSON.parse(json))); };
  assert.equal(await current.resolve(), true);
  assert.equal(calls.save, 1);
  assert.equal(calls.receipt, 2);
  assert.equal(current.closeState().blocked, false);
  remount();
  assert.ok(notified > 2);
});

test('actual Wails cancellation retains unknown authority and explicit lookup proves a held commit', async () => {
  const { CancellablePromise } = await import(pathToFileURL(path.join(__dirname, '..', 'node_modules',
    '@wailsio', 'runtime', 'dist', 'cancellable.js')).href);
  let complete, request, cancelled = 0;
  const { current, calls } = fixture({
    save: json => {
      calls.save++; request = JSON.parse(json);
      return new CancellablePromise(resolve => { complete = resolve; }, () => cancelled++);
    },
  });
  current.stage('PicName', 'saved.jpg', false);
  const saving = current.save(token);
  assert.equal(current.closeState().busy, true);
  current.cancel();
  assert.equal(current.closeState().busy, false);
  assert.equal(current.closeState().blocked, true);
  complete(receipt(request));
  assert.equal(await saving, false);
  assert.equal(current.view().receipt, null);
  assert.equal(cancelled, 1);
  assert.equal(await current.resolve(), true);
  assert.equal(calls.save, 1);
});

test('failed discard/reload preserves draft; valid explicit reload observes current row without claiming a lost commit', async () => {
  const { current, calls, port } = fixture();
  current.stage('PicName', 'x'.repeat(256), false);
  port.read = () => { calls.read++; return resolved({ ...review(), project: 'foreign' }); };
  assert.equal(await current.discardAndReload(), false);
  assert.equal(current.view().draft.cells.PicName.raw.length, 256);
  assert.equal(current.closeState().blocked, true);
  port.read = () => { calls.read++; return resolved(review()); };
  assert.equal(await current.discardAndReload(), true);
  assert.equal(current.closeState().unsaved, false);
  assert.equal(current.view().receipt, null);
  assert.equal(calls.save, 0);
});

test('unchanged historical-invalid assignments do not generate a write or fabricated history receipt', async () => {
  const { current, calls } = fixture();
  const source = review(); source.records.rows[0].cells[2] = cell('text', 'x'.repeat(256));
  current.select(source, source.records.rows[0].rowId);
  current.stage('PicName', 'x'.repeat(256), false);
  assert.equal(await current.save(token), true);
  assert.equal(calls.save, 0);
  assert.equal(current.view().receipt, null);
});

test('manager and child reuse one owner-scoped session without inheriting a foreign context or project', () => {
  const cached = loadTypeScript('pictureMetadataSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { PictureService: {}, PictureMetadataService: {} },
    './pictureMetadataSession': session,
  });
  const first = cached.pictureMetadataSession(owner);
  first.select(review(), review().records.rows[0].rowId);
  first.stage('PicName', '', false);
  assert.equal(cached.pictureMetadataSession({ ...owner }), first);
  assert.equal(cached.pictureMetadataSession({ ...owner }).view().draft.cells.PicName.raw, '');
  assert.notEqual(cached.pictureMetadataSession({ ...owner, contextId: 'foreign' }), first);
  assert.notEqual(cached.pictureMetadataSession({ ...owner, project: 'foreign' }), first);
  assert.notEqual(cached.pictureMetadataSession({ ...owner, plotNumber: 'foreign' }), first);
});
