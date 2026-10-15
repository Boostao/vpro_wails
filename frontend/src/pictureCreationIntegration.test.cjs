const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { editor, picture, owner, selection, receipt, requestId } = require('./pictureCreationTestHelpers.cjs');
const { metadata } = require('./siviParentTestHelpers.cjs');
const reads = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const sessionModule = loadTypeScript('pictureCreationSession.ts', {
  './readRequests': reads, './pictureRead': picture, './pictureCreationEditor': editor,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
const source = name => readFileSync(path.join(__dirname, name), 'utf8');

test('actual creation and coupled metadata subscription effects settle, track owner changes and detach on disposal', async () => {
  const client = await import('svelte/internal/client');
  const { compile } = require('svelte/compiler');
  for (const [filename, name, revision] of [
    ['PictureCreationPanel.svelte', 'session', 'revision'],
    ['PictureManager.svelte', 'creationSession', 'creationRevision'],
    ['PicturePanel.svelte', 'creationSession', 'creationRevision'],
    ['PictureManager.svelte', 'metadataSession', 'metadataRevision'],
    ['PicturePanel.svelte', 'metadataSession', 'metadataRevision'],
  ]) {
    const subscription = source(filename).split('\n').find(line => line.includes(`${name}${name === 'session' ? '.' : '?.'}subscribe`));
    assert.ok(subscription, `${filename} ${name}`);
    const compiled = compile(`<script>
      import { untrack } from 'svelte';
      let { ${name} } = $props();
      let ${revision} = $state(0);
      ${subscription}
      $effect(() => { ${revision}; ${name}?.view(); });
    </script>`, { generate: 'client' }).js.code;
    const harness = new Function('$', 'untrack', compiled.replace(/^import .*$/gm, '')
      .replace('export default function', 'function') + '\nreturn _unknown_;')(client, client.untrack);
    function trackedSession() {
      const instance = new sessionModule.PictureCreationSession(owner, { choose: () => operation(null) });
      const subscribe = instance.subscribe.bind(instance), view = instance.view.bind(instance);
      instance.subscriptions = 0; instance.cleanups = 0; instance.views = 0;
      instance.subscribe = notify => {
        assert.ok(++instance.subscriptions <= 5, 'Subscription invalidated itself');
        const stop = subscribe(notify);
        return () => { instance.cleanups++; stop(); };
      };
      instance.view = () => { instance.views++; return view(); };
      return instance;
    }
    const first = trackedSession(), second = trackedSession(), current = client.state(first);
    const dispose = client.effect_root(() => harness(null, { get [name]() { return client.get(current); } }));
    try {
      client.flush();
      assert.equal(first.subscriptions, 1);
      const views = first.views;
      await first.add(requestId);
      client.flush();
      assert.equal(first.subscriptions, 1);
      assert.ok(first.views > views);
      client.set(current, second);
      client.flush();
      assert.equal(first.cleanups, 1);
      assert.equal(second.subscriptions, 1);
      const detachedViews = first.views;
      await first.add(requestId);
      client.flush();
      assert.equal(first.views, detachedViews);
    } finally { dispose(); }
    assert.equal(second.cleanups, 1);
  }
});

test('actual manager derived session creation does not notify reactive root observers before an operation', async () => {
  const client = await import('svelte/internal/client');
  const { compile } = require('svelte/compiler');
  const ts = require('typescript');
  const rootBusy = client.state(false);
  const registry = loadTypeScript('pictureCreationSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { PictureCreationService: {} },
    './pictureCreationSession': sessionModule,
  });
  let notifications = 0;
  const stop = registry.subscribePictureCreationActivity(activity => {
    notifications++;
    client.set(rootBusy, activity.busy || activity.blocked);
  });
  const script = source('PictureManager.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' }).js.code
    .replaceAll('import.meta.env', '__testEnvironment');
  const ast = ts.createSourceFile('manager.js', compiled, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const manager = new Function('$', 'untrack', 'plotLookupInputError', 'pictureMetadataSession',
    'pictureCreationSession', '__testEnvironment', code + '\nreturn _unknown_;')(
      client, client.untrack, () => null, () => { throw new Error('Metadata remains disabled'); },
      registry.pictureCreationSession, { VITE_PICTURE_CREATION_WRITING: 'true' });
  const activity = [];
  const dispose = client.effect_root(() => manager(null, {
    ...owner, initialPlot: owner.plotNumber, onBusyChange: value => activity.push(value),
  }));
  try {
    client.flush();
    assert.deepEqual(activity, [false]);
    assert.equal(notifications, 1, 'Idle construction must not emit an unchanged aggregate inside a derived evaluation');
    assert.equal(client.get(rootBusy), false);
    assert.equal(registry.pictureCreationSession(owner).view().phase, 'idle');
  } finally { dispose(); stop(); }
});

test('actual binding adapter retains per-parent sessions and root barriers through subscriber remount and acknowledgement', async () => {
  let finishChoose;
  const calls = [], pending = new Promise(resolve => finishChoose = resolve);
  const request = editor.pictureCreationRequest(selection(), owner, requestId);
  const registry = loadTypeScript('pictureCreationSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { PictureCreationService: {
      Choose(...args) { calls.push(['choose', ...args]); return pending; },
      Create(token, json) { calls.push(['create', token, JSON.parse(json)]); return operation(receipt(request)); },
      AcknowledgeReceipt(token) { calls.push(['ack', token]); return operation(); },
    } },
    './pictureCreationSession': sessionModule,
  });
  const states = [];
  const unsubscribe = registry.subscribePictureCreationActivity(state => states.push(state));
  const session = registry.pictureCreationSession(owner), adding = session.add(requestId);
  assert.equal(states.at(-1).busy, true);
  assert.equal(states.at(-1).blocked, true);
  unsubscribe();
  assert.equal(registry.pictureCreationSession({ ...owner }), session);
  const remounted = [];
  registry.subscribePictureCreationActivity(state => remounted.push(state));
  assert.equal(remounted.at(-1).busy, true);
  finishChoose(selection());
  assert.equal(await adding, true);
  assert.equal(remounted.at(-1).blocked, true);
  assert.deepEqual(calls[0], ['choose', owner.contextId, owner.project, owner.plotNumber]);
  assert.deepEqual(calls[1], ['create', selection().token, JSON.parse(JSON.stringify(request))]);
  assert.equal(await session.acknowledge(), true);
  assert.equal(remounted.at(-1).blocked, false);
  assert.equal(registry.pictureCreationSession(owner).view().receipts.length, 1);
  assert.notEqual(registry.pictureCreationSession({ ...owner, plotNumber: 'other' }), session);
});

test('manager blocks literal parent replacement and cancelled read callbacks cannot clear retained creation ownership', () => {
  const states = [];
  const manager = componentFunctions('PictureManager.svelte', ['selectPlot', 'reading'], {
    busy: false, metadataBlocked: false, creationBlocked: true, selected: 'original', plot: 'other', error: '',
    onBusyChange: value => states.push(value),
  });
  manager.actions.reading(false);
  manager.actions.selectPlot();
  assert.deepEqual(states, [true]);
  assert.equal(manager.selected, 'original');
  assert.match(manager.error, /retained picture selection or creation receipt/);
});

test('committed list refresh reads owned metadata and rejects complete-row drift without erasing creation receipts', async () => {
  const request = editor.pictureCreationRequest(selection(), owner, requestId), committed = receipt(request);
  const context = componentFunctions('PicturePanel.svelte', ['cancel', 'refreshCreated'], {
    ...owner, generation: 0, ownerKey: 'owned', reviewKey: '', busy: false, error: '', image: null,
    viewerOpen: false, review: null, selected: '', onBusyChange() {},
    reads: { cancelAll() {}, track: value => value }, pictureMetadataFromWire: picture.pictureMetadataFromWire,
    equalCell: metadata.equalCell,
    PictureService: { GetMetadata: () => Promise.resolve({ ...owner, records: { columns: committed.columns, rows: [committed.committed] } }) },
  });
  await context.actions.refreshCreated(committed);
  assert.equal(context.selected, committed.rowId);
  assert.equal(context.review.records.rows[0].cells[4].storage, 'null');
  const previous = JSON.stringify(context.review);
  context.PictureService.GetMetadata = () => Promise.resolve({ ...owner, records: { columns: committed.columns,
    rows: [{ rowId: committed.rowId, cells: committed.committed.cells.map((cell, index) =>
      index === 2 ? { ...cell, text: 'replacement.jpg' } : cell) }] } });
  await context.actions.refreshCreated(committed);
  assert.match(context.error, /creation committed.*refresh failed/);
  assert.equal(JSON.stringify(context.review), previous);
});

test('creation panel retains receipts and safety feedback while unknown writes expose no replay action', () => {
  const component = serverComponent(source('PictureCreationPanel.svelte'), 'PictureCreationPanel.svelte', {});
  const wire = receipt(), view = { selection: selection(), request: editor.pictureCreationRequest(selection(), owner, requestId),
    receipt: null, receipts: [wire], image: null, phase: 'idle', busy: false, blocked: true, error: 'unknown write',
    acknowledged: false, attempted: true, noCommitVerified: false };
  const html = render(component, { props: { session: { subscribe: () => () => {}, view: () => view } } }).body;
  assert.ok(html.indexOf('unknown write') < html.indexOf('data-picture-add'));
  assert.match(html, /Resolve creation history \(read-only\)/);
  assert.doesNotMatch(html, /data-picture-create-recovered/);
  assert.match(html, /data-picture-release[^>]*disabled/);
  assert.match(html, /physical row 9223372036854775807/);
  assert.match(html, /PicComment remains NULL/);
});

test('root aggregates persistent creation ownership rather than letting a disposed read clear native-close and transition guards', () => {
  const app = source('App.svelte');
  assert.match(app, /editorBusy = \$derived\(localEditorBusy \|\| matrixPublicationBusy \|\| pictureCreationBusy \|\| pictureLifecycleBusy \|\| twoPageSoilBusy \|\| twoPageVegetationBusy \|\| externalSUEnvironmentBusy \|\| summaryPreferenceBusy \|\| summaryFieldListBusy \|\| summaryEnvironmentalBusy \|\| projectReportBusy\)/);
  assert.match(app, /pictureCreationBusy = activity\.busy \|\| activity\.blocked/);
  assert.match(app, /subscribePictureCreationActivity/);
  assert.match(app, /pictureManagerPlot = activity\.owner\.plotNumber/);
  assert.match(app, /closeDisposition\(state, busy \|\| editorBusy \|\| transitionWorking/);
  assert.match(app, /onBusyChange=\{\(value\) => \{ localEditorBusy = value; \}\}/);
  assert.doesNotMatch(source('pictureCreationSessions.ts'), /GetImage\(|SwitchContext|CreatePlot|Delete|Restore/);
});

test('retained creation ownership independently blocks background state, plot and hierarchy reloads', async () => {
  const context = componentFunctions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy'], {
    pictureCreationBusy: true, pictureLifecycleBusy: false, vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    editorBusy: false, view: 'home', request: 0, stateRequest: 0, hierarchyRequest: 0, busy: false,
  });
  await context.actions.loadPlots(0);
  await context.actions.refresh();
  await context.actions.loadHierarchy();
  assert.equal(context.request, 0);
  assert.equal(context.stateRequest, 0);
  assert.equal(context.hierarchyRequest, 0);
  assert.equal(context.busy, false);
});

test('actual Add panel dispatches one Create and preserves a preview error, while chooser Cancel dispatches none', async () => {
  for (const cancelled of [false, true]) {
    const calls = [];
    const session = new sessionModule.PictureCreationSession(owner, {
      choose: () => { calls.push('choose'); return operation(cancelled ? null : selection()); },
      create: (token, json) => { calls.push('create'); return operation(receipt(JSON.parse(json))); },
      image: () => { calls.push('image'); return operation(Promise.reject(new Error('Selected preview unavailable'))); },
    });
    const panel = componentFunctions('PictureCreationPanel.svelte', ['operate'], {
      session, disabled: false, error: '', crypto: { randomUUID: () => requestId },
    });
    await panel.actions.operate('add');
    assert.deepEqual(calls, cancelled ? ['choose'] : ['choose', 'create', 'image']);
    assert.equal(session.view().receipts.length, cancelled ? 0 : 1);
    assert.equal(session.closeState().blocked, !cancelled);
    if (!cancelled) assert.match(session.view().error, /Selected preview unavailable/);
  }
});

test('retained chooser and unknown writes block actual root close/context/navigation after reader disposal and remount', async () => {
  let finishChoose;
  const calls = [];
  const registry = loadTypeScript('pictureCreationSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { PictureCreationService: {
      Choose: () => new Promise(resolve => finishChoose = resolve),
      Create: () => { calls.push('create'); return operation(Promise.reject(new Error('Lost write delivery'))); },
      LookupReceipt: json => { calls.push('lookup'); return operation(receipt(JSON.parse(json))); },
      AcknowledgeReceipt: () => { calls.push('ack'); return operation(); },
    } }, './pictureCreationSession': sessionModule,
  });
  const closed = [];
  const context = componentFunctions('App.svelte', ['handleCloseRequest', 'respondToClose', 'requestTransition',
    'respondToTransition', 'navigate'], {
    view: 'picture-manager', localEditorBusy: true, pictureCreationBusy: false, pictureLifecycleBusy: false, summaryEnvironmentalBusy: false, projectReportBusy: false, busy: false, editor: null,
    transitionWorking: false, archivePublicationBusy: false, vegetationPublicationBusy: false,
    lifeformPublicationBusy: false, summaryPublicationBusy: false, pendingTransition: null,
    closeWorking: false, closeRequest: '', closeError: '', error: '', transitionError: '',
    $projectState: { contextId: owner.contextId }, document: { activeElement: null }, HTMLElement: class {},
    closeDisposition: loadTypeScript('closeLifecycle.ts').closeDisposition,
    tick: async () => {}, CloseService: {
      GetPendingCloseRequest: () => Promise.resolve('owned-close'),
      CancelClose: id => { closed.push(['cancel', id]); },
      ConfirmClose: id => { closed.push(['confirm', id]); },
    },
  });
  const expression = source('App.svelte').match(/const editorBusy = \$derived\(([^;]+)\);/)[1];
  const aggregate = new Function('localEditorBusy', 'pictureCreationBusy', 'pictureLifecycleBusy', 'twoPageSoilBusy', 'twoPageVegetationBusy', 'summaryPreferenceBusy', 'summaryFieldListBusy', 'summaryEnvironmentalBusy', 'projectReportBusy', 'externalSUEnvironmentBusy', 'matrixPublicationBusy', `return ${expression}`);
  Object.defineProperty(context, 'editorBusy', {
    get: () => aggregate(context.localEditorBusy, context.pictureCreationBusy, context.pictureLifecycleBusy, context.twoPageSoilBusy, context.twoPageVegetationBusy, context.summaryPreferenceBusy, context.summaryFieldListBusy, context.summaryEnvironmentalBusy, context.projectReportBusy, context.externalSUEnvironmentBusy, false),
  });
  let observe = registry.subscribePictureCreationActivity(activity => {
    context.pictureCreationBusy = activity.busy || activity.blocked;
  });
  const session = registry.pictureCreationSession(owner), adding = session.add(requestId);
  const disposedReader = componentFunctions('PicturePanel.svelte', ['cancel'], {
    generation: 0, reads: { cancelAll() {} }, busy: true,
    onBusyChange: value => { context.localEditorBusy = value; },
  });
  disposedReader.actions.cancel();
  observe();
  observe = registry.subscribePictureCreationActivity(activity => {
    context.pictureCreationBusy = activity.busy || activity.blocked;
  });
  let transitions = 0;
  async function blocked() {
    assert.equal(context.localEditorBusy, false);
    assert.equal(context.editorBusy, true);
    await context.actions.handleCloseRequest('owned-close');
    assert.deepEqual(closed.at(-1), ['cancel', 'owned-close']);
    await context.actions.requestTransition(async () => { transitions++; });
    context.actions.navigate('plots');
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
    assert.equal(transitions, 0);
    assert.match(context.transitionError, /Wait/);
    context.pendingTransition = null;
    context.editor = null;
  }
  await blocked();
  finishChoose(selection());
  assert.equal(await adding, false);
  assert.equal(registry.pictureCreationSession({ ...owner }), session);
  assert.equal(session.view().attempted, true);
  await blocked();
  assert.equal(await session.recoverReceipt(), true);
  await blocked();
  assert.deepEqual(calls, ['create', 'lookup']);
  assert.equal(await session.acknowledge(), true);
  assert.equal(context.editorBusy, false);
  await context.actions.requestTransition(async () => { transitions++; });
  assert.equal(transitions, 1);
  await context.actions.handleCloseRequest('owned-close');
  assert.deepEqual(closed.at(-1), ['confirm', 'owned-close']);
  assert.equal(session.view().receipts.length, 1);
  observe();
});

test('verified release unlocks a recovered unattempted selection without dispatching Create', async () => {
  const calls = [];
  const session = new sessionModule.PictureCreationSession(owner, {
    choose: () => operation(Promise.reject(new Error('Lost chooser delivery'))),
    recoverSelection: () => operation(selection()),
    release: token => { calls.push(token); return operation(); },
    create: () => { throw new Error('Recovery/release must not Create'); },
  });
  assert.equal(await session.add(requestId), false);
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.recoverSelection(), true);
  assert.equal(await session.release(), true);
  assert.equal(session.closeState().blocked, false);
  assert.deepEqual(calls, [selection().token]);
});

test('enabled Add with backend creation disabled retains the refusal until owned readonly recovery proves no selection', async () => {
  const calls = [];
  const registry = loadTypeScript('pictureCreationSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { PictureCreationService: {
      Choose: () => { calls.push('choose'); return operation(Promise.reject(new Error('picture creation writing is disabled in this session'))); },
      RecoverSelection: () => { calls.push('recover'); return operation(null); },
      Create: () => { throw new Error('Disabled selection recovery cannot Create'); },
    } }, './pictureCreationSession': sessionModule,
  });
  const activity = [];
  const stop = registry.subscribePictureCreationActivity(value => activity.push(value));
  const session = registry.pictureCreationSession(owner);
  const panel = componentFunctions('PictureCreationPanel.svelte', ['operate'], {
    session, disabled: false, error: '', crypto: { randomUUID: () => requestId },
  });
  await panel.actions.operate('add');
  assert.match(session.view().error, /creation writing is disabled/);
  assert.equal(activity.at(-1).blocked, true);
  assert.equal(session.view().attempted, false);
  await panel.actions.operate('selection');
  assert.equal(activity.at(-1).blocked, false);
  assert.equal(session.view().selection, null);
  assert.equal(session.view().request, null);
  assert.equal(session.view().receipts.length, 0);
  assert.deepEqual(calls, ['choose', 'recover']);
  stop();
});

test('actual peer receipt effects refresh loaded owners once and discard late delivery after cancellation, disposal or owner change', async () => {
  const client = await import('svelte/internal/client');
  const { pathToFileURL } = require('node:url');
  const lifecycle = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte/package.json')), 'src', 'index-client.js')).href);
  const { compile } = require('svelte/compiler');
  const ts = require('typescript');
  const script = source('PicturePanel.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1]
    .replace('onBusyChange, metadataSession,', 'onBusyChange, metadataSession, inspect,')
    .replace('metadataSession?: PictureMetadataSession;', 'metadataSession?: PictureMetadataSession; inspect: (value: unknown) => void;');
  const compiled = compile(`<script lang="ts">${script}
    $effect(() => inspect({ review, selected, error, busy, cancel }));
  </script>`, { generate: 'client' }).js.code.replaceAll('import.meta.env', '__testEnvironment');
  const ast = ts.createSourceFile('peer.js', compiled, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const harness = new Function('$', 'untrack', 'onDestroy', 'ReadRequests', 'PictureService',
    'pictureMetadataFromWire', 'pictureImageFromWire', 'pictureCellLabel', 'pictureCreationSession',
    'equalCell', '__testEnvironment', code + '\nreturn _unknown_;');
  for (const mode of ['normal', 'drift', 'cancel', 'dispose', 'owner-change']) {
    const pending = [], snapshots = [], properties = [], disposers = [];
    const session = new sessionModule.PictureCreationSession(owner, {
      choose: () => operation(selection()), create: (token, json) => operation(receipt(JSON.parse(json))),
    });
    const service = { GetMetadata(contextId, plotNumber) {
      let finish;
      const promise = new Promise(resolve => finish = resolve);
      const tracked = Object.assign(promise, { cancel() { tracked.cancelled = true; } });
      pending.push({ contextId, plotNumber, finish, tracked });
      return tracked;
    } };
    const peer = harness(client, client.untrack, lifecycle.onDestroy, reads.ReadRequests, service,
      picture.pictureMetadataFromWire, picture.pictureImageFromWire, picture.pictureCellLabel,
      () => session, metadata.equalCell, { VITE_PICTURE_CREATION_WRITING: 'true' });
    for (let index = 0; index < 2; index++) {
      const props = client.proxy({ ...owner, view: index ? 'child' : 'manager',
        onBusyChange() {}, inspect(value) { snapshots[index] = value; } });
      properties.push(props);
      disposers.push(client.effect_root(() => peer(null, props)));
    }
    try {
      client.flush();
      assert.equal(pending.length, 0);
      assert.equal(await session.add(requestId), true);
      client.flush();
      assert.equal(pending.length, 2, mode);
      const committed = receipt(), originalRows = [committed.committed];
      if (mode === 'cancel') {
        snapshots[0].cancel();
      } else if (mode === 'dispose') {
        disposers[1]();
        disposers[1] = () => {};
      } else if (mode === 'owner-change') {
        properties[0].plotNumber = 'OTHER';
        client.flush();
      }
      const before = snapshots.map(value => ({ ...value }));
      for (const [index, request] of pending.entries()) {
        const rows = mode === 'drift' && index === 0 ? [{ ...committed.committed,
          cells: committed.committed.cells.map((cell, field) => field === 2 ? { ...cell, text: 'reused-row.jpg' } : cell) }] : originalRows;
        request.finish({ ...owner, records: { columns: committed.columns, rows } });
      }
      await new Promise(resolve => setImmediate(resolve));
      client.flush();
      assert.equal(pending.length, 2, 'Reactive updates must not repeat a completed refresh');
      const discarded = mode === 'cancel' || mode === 'owner-change' ? 0 : mode === 'dispose' ? 1 : -1;
      for (let index = 0; index < 2; index++) {
        if (index === discarded) {
          assert.equal(snapshots[index].review, before[index].review);
          assert.equal(snapshots[index].selected, before[index].selected);
        } else if (mode === 'drift' && index === 0) {
          assert.equal(snapshots[index].review, null);
          assert.match(snapshots[index].error, /complete committed physical row differs/);
        } else {
          assert.equal(snapshots[index].selected, committed.rowId);
          assert.equal(snapshots[index].review.records.rows[0].cells[4].storage, 'null');
        }
      }
      assert.equal(session.view().receipts.length, 1);
    } finally { for (const dispose of disposers) dispose(); client.flush(); }
  }
});
