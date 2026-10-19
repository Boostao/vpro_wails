const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { editor, fixture, owner, requestId, deletion, review, restorationReview, restoration, cell, loadTypeScript } =
  require('./pictureLifecycleTestHelpers.cjs');
const { picture } = require('./pictureCreationTestHelpers.cjs');
const reads = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const sessionModule = loadTypeScript('pictureLifecycleSession.ts', {
  './pictureLifecycleEditor': editor, './pictureRead': picture, './readRequests': reads,
});
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const plain = value => JSON.parse(JSON.stringify(value));
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
function session(port = {}) {
  return new sessionModule.PictureLifecycleSession(owner, {
    read: () => operation(review()),
    history: () => operation({ ...owner, events: [{ deletion: deletion(), restoration: null }] }),
    review: () => operation(restorationReview()),
    delete: json => operation({ ...deletion(), request: JSON.parse(json) }),
    deletionReceipt: json => operation({ ...deletion(), request: JSON.parse(json), replayed: true }),
    restore: json => operation(restoration(JSON.parse(json))),
    restorationReceipt: json => operation({ ...restoration(JSON.parse(json)), replayed: true }),
    ...port,
  });
}
function registry(services) {
  return loadTypeScript('pictureLifecycleSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': services,
    './pictureLifecycleSession': sessionModule,
  });
}

test('actual lifecycle subscriptions settle, retain raw DTOs and detach on owner change and disposal', async () => {
  const client = await import('svelte/internal/client');
  const { compile } = require('svelte/compiler');
  for (const [filename, name, revision] of [
    ['PictureManager.svelte', 'lifecycleSession', 'lifecycleRevision'],
    ['PicturePanel.svelte', 'lifecycleSession', 'lifecycleRevision'],
    ['PictureLifecyclePanel.svelte', 'session', null],
  ]) {
    const subscription = source(filename).split('\n').find(line =>
      line.includes(`${name}${name === 'session' ? '.' : '?.'}subscribe`));
    assert.ok(subscription, filename);
    const declaration = revision ? `let ${revision} = $state(0);`
      : 'let view = $state.raw(untrack(() => session.view()));';
    const observation = revision ? `${revision}; ${name}?.view();` : 'observe(view);';
    const compiled = compile(`<script>
      import { untrack } from 'svelte';
      let { ${name}, observe } = $props();
      ${declaration}
      ${subscription}
      $effect(() => { ${observation} });
    </script>`, { generate: 'client' }).js.code;
    const harness = new Function('$', 'untrack', compiled.replace(/^import .*$/gm, '')
      .replace('export default function', 'function') + '\nreturn _unknown_;')(client, client.untrack);
    function tracked() {
      const value = session(), subscribe = value.subscribe.bind(value), view = value.view.bind(value);
      value.subscriptions = 0; value.cleanups = 0; value.views = 0;
      value.subscribe = notify => {
        assert.ok(++value.subscriptions <= 5, 'Lifecycle subscription invalidated itself');
        const stop = subscribe(notify);
        return () => { value.cleanups++; stop(); };
      };
      value.view = () => { value.views++; return view(); };
      return value;
    }
    const first = tracked(), second = tracked(), current = client.state(first), snapshots = [];
    const dispose = client.effect_root(() => harness(null, {
      get [name]() { return client.get(current); },
      observe: value => snapshots.push(structuredClone(value)),
    }));
    try {
      client.flush();
      assert.equal(first.subscriptions, 1);
      const before = first.views;
      assert.equal(await first.reviewDeletion('-7'), true);
      client.flush();
      assert.equal(first.subscriptions, 1);
      assert.ok(first.views > before);
      if (!revision) assert.equal(snapshots.at(-1).original.records.rows[0].rowId, '-7');
      client.set(current, second); client.flush();
      assert.equal(first.cleanups, 1);
      assert.equal(second.subscriptions, 1);
      const detachedViews = first.views;
      first.discard(); client.flush();
      assert.equal(first.views, detachedViews);
    } finally { dispose(); }
    assert.equal(second.cleanups, 1);
  }
});

test('either independent lifecycle flag constructs an idle manager without notifying reactive root observers', async () => {
  const client = await import('svelte/internal/client');
  const { compile } = require('svelte/compiler');
  const ts = require('typescript');
  for (const flags of [
    { VITE_PICTURE_DELETION_WRITING: 'true' },
    { VITE_PICTURE_DELETION_RESTORING: 'true' },
    { VITE_PICTURE_DELETION_WRITING: 'true', VITE_PICTURE_DELETION_RESTORING: 'true' },
  ]) {
    const retained = registry({ PictureService: {}, PictureDeletionService: {} });
    const rootBusy = client.state(false);
    let notifications = 0;
    const stop = retained.subscribePictureLifecycleActivity(activity => {
      notifications++; client.set(rootBusy, activity.busy || activity.blocked);
    });
    const script = source('PictureManager.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
    const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' }).js.code
      .replaceAll('import.meta.env', '__testEnvironment');
    const ast = ts.createSourceFile('manager.js', compiled, ts.ScriptTarget.Latest, true);
    const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
      .replace('export default function', 'function');
    const manager = new Function('$', 'untrack', 'plotLookupInputError', 'pictureMetadataSession',
      'pictureCreationSession', 'pictureLifecycleSession', '__testEnvironment', code + '\nreturn _unknown_;')(
      client, client.untrack, () => null,
      () => { throw new Error('Metadata remains disabled'); },
      () => { throw new Error('Add remains disabled'); }, retained.pictureLifecycleSession, flags);
    const activity = [];
    const dispose = client.effect_root(() => manager(null, {
      ...owner, initialPlot: owner.plotNumber, onBusyChange: value => activity.push(value),
    }));
    try {
      client.flush();
      assert.deepEqual(activity, [false]);
      assert.equal(notifications, 1, 'Idle construction must not notify inside a derived evaluation');
      assert.equal(client.get(rootBusy), false);
      assert.equal(retained.pictureLifecycleSession(owner).closeState().unsaved, false);
    } finally { dispose(); stop(); }
  }
});

test('binding registry retains unknown requests across remount, recovers without replay and preserves both committed receipts', async () => {
  const calls = [];
  const retained = registry({
    PictureService: { GetMetadata: (...args) => { calls.push(['read', ...args]); return operation(review()); } },
    PictureDeletionService: {
      Delete: (...args) => { calls.push(['delete', ...args]); return operation(Promise.reject(new Error('Lost delivery'))); },
      LookupReceipt: (contextId, json) => {
        calls.push(['lookup', contextId, json]);
        return operation({ ...deletion(), request: JSON.parse(json), replayed: true });
      },
      GetHistory: (...args) => {
        calls.push(['history', ...args]);
        return operation({ ...owner, events: [{ deletion: deletion(), restoration: null }] });
      },
      ReviewRestoration: (...args) => { calls.push(['review', ...args]); return operation(restorationReview()); },
      Restore: (contextId, json) => {
        calls.push(['restore', contextId, json]); return operation(restoration(JSON.parse(json)));
      },
    },
  });
  const states = [], stop = retained.subscribePictureLifecycleActivity(state => states.push(state));
  const current = retained.pictureLifecycleSession(owner);
  assert.equal(states.length, 1);
  assert.equal(await current.reviewDeletion('-7'), true);
  current.confirm(true);
  assert.equal(await current.save(requestId), false);
  assert.equal(states.at(-1).blocked, true);
  stop();
  assert.equal(retained.pictureLifecycleSession({ ...owner }), current);
  assert.notEqual(retained.pictureLifecycleSession({ ...owner, plotNumber: '00337 ' }), current);
  assert.notEqual(retained.pictureLifecycleSession({ ...owner, contextId: 'later' }), current);
  const remounted = [];
  const unsubscribe = retained.subscribePictureLifecycleActivity(state => remounted.push(state));
  try {
    assert.deepEqual(plain(remounted.at(-1).owner), owner);
    remounted.at(-1).owner.project = 'changed observer copy';
    assert.equal(await current.resolve(), true);
    assert.equal(remounted.at(-1).owner.project, owner.project);
    assert.equal(remounted.at(-1).blocked, true);
    assert.equal(current.view().receipts.length, 1);
    current.acknowledge();
    assert.equal(remounted.at(-1).blocked, false);
    assert.equal(current.view().receipts.length, 1);
    assert.equal(await current.loadHistory(), true);
    assert.equal(await current.reviewRestoration('1'), true);
    current.confirm(true);
    assert.equal(await current.save(fixture.restorationRequestId), true);
    assert.equal(remounted.at(-1).blocked, true);
    current.acknowledge();
    assert.equal(remounted.at(-1).blocked, false);
    assert.deepEqual(plain(current.view().receipts.map(commit => commit.kind)), ['delete', 'restore']);
    assert.deepEqual(calls[0], ['read', owner.contextId, owner.plotNumber]);
    const dispatched = calls.find(call => call[0] === 'delete');
    assert.deepEqual(calls.find(call => call[0] === 'lookup'), ['lookup', ...dispatched.slice(1)]);
    assert.equal(calls.filter(call => call[0] === 'delete').length, 1);
    assert.deepEqual(calls.find(call => call[0] === 'history'), ['history', owner.contextId, owner.plotNumber]);
    assert.deepEqual(calls.find(call => call[0] === 'review'), ['review', owner.contextId, owner.plotNumber, '1']);
    assert.equal(calls.find(call => call[0] === 'restore')[1], owner.contextId);
  } finally { unsubscribe(); }
});

test('lifecycle panel renders associated source fields and retained safety feedback before ordinary guidance', async () => {
  const value = session({ delete: () => operation(Promise.reject(new Error('Lost delivery'))) });
  await value.reviewDeletion('-7'); value.confirm(true);
  const component = serverComponent(source('PictureLifecyclePanel.svelte'), 'PictureLifecyclePanel.svelte', {
    './pictureRead': picture,
  });
  let html = render(component, { props: { session: value } }).body;
  assert.equal((html.match(/<output(?:\s[^>]*)?>/g) || []).length, 5);
  for (const column of review().records.columns) assert.match(html, new RegExp(`<label[^>]*>${column.name}<output(?:\\s[^>]*)?>`));
  assert.match(html, /NULL/);
  assert.match(html, /Delete exactly this reviewed physical reference/);
  assert.doesNotMatch(html, /Historical picture deletions/);
  await value.save(requestId);
  html = render(component, { props: { session: value } }).body;
  assert.match(html, /Mutation outcome is unknown/);
  assert.match(html, /Recover picture receipt read-only/);
  assert.ok(html.indexOf('Mutation outcome is unknown') < html.indexOf('Reviewed physical rowid'));
  assert.ok(html.indexOf('role="alert"') < html.indexOf('Reviewed physical rowid'));
  assert.match(html, /data-picture-lifecycle-confirm[^>]*disabled/);
  await value.resolve();
  html = render(component, { props: { session: value } }).body;
  assert.match(html, /The physical operation committed/);
  assert.match(html, /Retained irreversible receipts \(1\)/);
  value.acknowledge();
  html = render(component, { props: { session: value } }).body;
  assert.match(html, /Retained irreversible receipts \(1\)/);
  assert.doesNotMatch(html, /Acknowledge committed picture receipt/);
});

test('restoration history is independently gated and consumed records remain non-actionable', async () => {
  const value = session({
    history: () => operation({ ...owner, events: [{ deletion: deletion(), restoration: restoration() }] }),
  });
  await value.loadHistory();
  const component = serverComponent(source('PictureLifecyclePanel.svelte')
    .replace("import.meta.env.VITE_PICTURE_DELETION_RESTORING === 'true'", 'true'),
  'PictureLifecyclePanel.svelte', { './pictureRead': picture });
  const html = render(component, { props: { session: value } }).body;
  assert.match(html, /Historical picture deletions/);
  assert.match(html, /Already restored; acknowledgement retained/);
  assert.match(html, /<button disabled="" aria-label="Review picture restoration history 1"/);
  assert.doesNotMatch(html, /data-picture-lifecycle-save/);
});

function refreshHost(wire) {
  const callbacks = [];
  const context = componentFunctions('PicturePanel.svelte', ['cancel', 'refreshLifecycle'], {
    ...owner, generation: 0, ownerKey: 'owned-key', reviewKey: 'owned-key',
    selected: '-7', image: { old: true }, viewerOpen: true, review: review(), error: '', busy: false,
    reads: new reads.ReadRequests(),
    PictureService: { GetMetadata: () => operation(wire) },
    pictureMetadataFromWire: picture.pictureMetadataFromWire,
    pictureLifecycleOriginalMatches: editor.pictureLifecycleOriginalMatches,
    onBusyChange: value => callbacks.push(value),
  });
  return { context, callbacks };
}

test('committed deletion clears stale image/selection and refuses a reappearing physical row or identity', async () => {
  const commit = { kind: 'delete', receipt: deletion() };
  for (const rows of [[], [deletion().request.original], [{ ...deletion().request.original, rowId: '3' }]]) {
    const { context } = refreshHost({ ...review(), records: { ...review().records, rows } });
    await context.actions.refreshLifecycle(commit);
    assert.equal(context.selected, '');
    assert.equal(context.image, null);
    assert.equal(context.viewerOpen, false);
    if (rows.length) {
      assert.equal(context.review, null);
      assert.match(context.error, /deleted physical row\/identity reappeared/);
      assert.match(context.error, /irreversible receipt is retained/);
    } else {
      assert.equal(context.review.records.rows.length, 0);
      assert.equal(context.error, '');
    }
    assert.equal(commit.receipt.historyId, '1');
  }
});

test('restoration refresh requires every complete physical peer and historical cell, not just IDs', async () => {
  const original = deletion().request.original;
  const peer = structuredClone(original); peer.rowId = '4'; peer.cells[0] = cell('integer', '91');
  const request = editor.pictureRestorationRequest({ ...restorationReview(), peers: [peer] }, owner, fixture.restorationRequestId);
  const commit = { kind: 'restore', receipt: restoration(request) };
  const expected = [original, peer];
  for (const changed of [-1, 0, 1, 2, 3, 4, 5]) {
    const rows = structuredClone(expected);
    if (changed >= 0 && changed < 5) rows[1].cells[changed] = cell('text', 'replacement');
    if (changed === 5) rows.pop();
    const { context } = refreshHost({ ...review(), records: { ...review().records, rows } });
    await context.actions.refreshLifecycle(commit);
    assert.equal(context.image, null);
    if (changed < 0) {
      assert.equal(context.selected, original.rowId);
      assert.equal(context.review.records.rows.length, 2);
      assert.equal(context.error, '');
    } else {
      assert.equal(context.selected, '');
      assert.equal(context.review, null);
      assert.match(context.error, changed === 3
        ? /literal physical plot membership were not returned/
        : /complete restored physical row\/peers differ/);
    }
    assert.deepEqual(plain(commit.receipt.request.peers), [peer]);
  }
});

test('late lifecycle refresh cannot populate a cancelled or replaced picture owner', async () => {
  for (const replaced of [false, true]) {
    let finish;
    const pending = new Promise(resolve => finish = resolve);
    const { context } = refreshHost(pending);
    const refreshing = context.actions.refreshLifecycle({ kind: 'restore', receipt: restoration() });
    assert.equal(context.busy, true);
    if (replaced) context.ownerKey = 'new-owner';
    context.actions.cancel();
    finish(review());
    await refreshing;
    assert.equal(context.review, null);
    assert.equal(context.selected, '');
    assert.equal(context.image, null);
    assert.equal(context.busy, false);
    assert.equal(context.error, '');
  }
});

test('lifecycle review and manager ownership guards preserve default-off and unrelated draft barriers', async () => {
  for (const barrier of ['disabled', 'busy', 'metadataPending', 'lifecyclePending', 'child', 'gate']) {
    const calls = [];
    const context = componentFunctions('PicturePanel.svelte', ['reviewDeletion', 'editMetadata'], {
      row: deletion().request.original, currentReview: review(), deletionEnabled: true, view: 'manager',
      disabled: false, busy: false, metadataPending: false, lifecyclePending: false, error: '',
      lifecycleSession: { reviewDeletion: (...args) => { calls.push(['delete', ...args]); } },
      metadataSession: { select: (...args) => { calls.push(['metadata', ...args]); } },
    });
    if (barrier === 'child') context.view = 'child';
    else if (barrier === 'gate') context.deletionEnabled = false;
    else context[barrier] = true;
    await context.actions.reviewDeletion();
    assert.deepEqual(calls, [], barrier);
    if (!['child', 'gate'].includes(barrier)) {
      context.actions.editMetadata();
      assert.deepEqual(calls, [], barrier);
    }
  }
  const activity = [];
  const manager = componentFunctions('PictureManager.svelte', ['selectPlot', 'reading'], {
    busy: false, metadataBlocked: false, creationBlocked: false, lifecycleBlocked: true,
    selected: owner.plotNumber, plot: 'other', error: '', plotLookupInputError: () => null,
    onBusyChange: value => activity.push(value),
  });
  manager.actions.selectPlot();
  assert.equal(manager.selected, owner.plotNumber);
  assert.match(manager.error, /retained picture lifecycle/);
  manager.actions.reading(false);
  assert.deepEqual(activity, [true]);
});

test('persistent lifecycle ownership blocks root close, transitions and background reloads until receipt acknowledgement', async () => {
  const value = session({ delete: () => operation(Promise.reject(new Error('Lost delivery'))) });
  const closed = [];
  const context = componentFunctions('App.svelte', ['handleCloseRequest', 'respondToClose', 'requestTransition',
    'respondToTransition', 'navigate', 'loadPlots', 'refresh', 'loadHierarchy'], {
    view: 'picture-manager', localEditorBusy: false, pictureCreationBusy: false, pictureLifecycleBusy: false, summaryEnvironmentalBusy: false, projectReportBusy: false,
    busy: false, editor: null, editorInitializing: false, transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    pendingTransition: null, closeWorking: false, closeRequest: '', closeError: '', error: '', transitionError: '',
    request: 0, stateRequest: 0, hierarchyRequest: 0, $projectState: { contextId: owner.contextId },
    document: { activeElement: null }, HTMLElement: class {},
    closeDisposition: loadTypeScript('closeLifecycle.ts').closeDisposition,
    tick: async () => {}, CloseService: {
      GetPendingCloseRequest: () => operation('owned-close'),
      CancelClose: id => { closed.push(['cancel', id]); },
      ConfirmClose: id => { closed.push(['confirm', id]); },
    },
  });
  const expression = source('App.svelte').match(/const editorBusy = \$derived\(([^;]+)\);/)[1];
  const aggregate = new Function('localEditorBusy', 'pictureCreationBusy', 'pictureLifecycleBusy', 'twoPageSoilBusy', 'twoPageVegetationBusy', 'summaryPreferenceBusy', 'summaryFieldListBusy', 'summaryEnvironmentalBusy', 'projectReportBusy', 'externalSUEnvironmentBusy', 'matrixPublicationBusy', `return ${expression}`);
  Object.defineProperty(context, 'editorBusy', {
    get: () => aggregate(context.localEditorBusy, context.pictureCreationBusy, context.pictureLifecycleBusy, context.twoPageSoilBusy, context.twoPageVegetationBusy, context.summaryPreferenceBusy, context.summaryFieldListBusy, context.summaryEnvironmentalBusy, context.projectReportBusy, context.externalSUEnvironmentBusy, false),
  });
  const stop = value.subscribe(() => {
    const close = value.closeState();
    context.pictureLifecycleBusy = close.busy || close.unsaved || close.blocked;
  });
  let transitions = 0;
  async function blocked() {
    assert.equal(context.editorBusy, true);
    await context.actions.handleCloseRequest('owned-close');
    assert.deepEqual(closed.at(-1), ['cancel', 'owned-close']);
    await context.actions.requestTransition(async () => { transitions++; });
    context.actions.navigate('home');
    await Promise.resolve();
    assert.equal(transitions, 0);
    assert.equal(context.view, 'picture-manager');
    context.closeRequest = 'owned-close';
    await context.actions.respondToClose('discard');
    assert.match(context.closeError, /Wait/);
    context.closeRequest = '';
    context.pendingTransition = { contextId: owner.contextId, run: async () => { transitions++; } };
    context.editor = { getCloseState: () => ({ busy: false, blocked: false, unsaved: true }) };
    await context.actions.respondToTransition('discard');
    assert.match(context.transitionError, /Wait/);
    assert.equal(transitions, 0);
    context.pendingTransition = null; context.editor = null;
    await context.actions.loadPlots(0); await context.actions.refresh(); await context.actions.loadHierarchy();
    assert.deepEqual([context.request, context.stateRequest, context.hierarchyRequest], [0, 0, 0]);
  }
  try {
    await value.reviewDeletion('-7'); await blocked();
    value.confirm(true);
    assert.equal(await value.save(requestId), false); await blocked();
    assert.equal(await value.resolve(), true); await blocked();
    value.acknowledge();
    assert.equal(context.editorBusy, false);
    await context.actions.requestTransition(async () => { transitions++; });
    assert.equal(transitions, 1);
    await context.actions.handleCloseRequest('owned-close');
    assert.deepEqual(closed.at(-1), ['confirm', 'owned-close']);
    assert.equal(value.view().receipts.length, 1);
  } finally { stop(); }
});
