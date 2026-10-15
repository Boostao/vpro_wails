const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { render } = require('svelte/server');
const { componentFunctions: extractFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const componentFunctions = (name, functions, values) => extractFunctions(name, functions, {
  auditOpen: false, auditBusy: false, googleEarthOpen: false, googleEarthBusy: false, ...values,
});
const owner = { contextId: 'context:owned', project: 'OwnedProject', plot: 'P' };

test('actual mapped reverse action holds all variants, child ownership, validation and foreign scope before opening', () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const active = { ...clean }, peer = { ...clean }, calls = [];
  const host = componentFunctions('TwoPageEntryEditor.svelte', ['getDraftState', 'openExternalTransfer'], {
    owner, form: 'FS882-8x6XL-CHARS', close: active, error: null,
    externalOpen: false, sourceRefreshing: false, sourceBlocked: false, childBlocked: false,
    sourceScope: { ...owner, projectPath: 'P.db', su: 'Separate', suPath: 'S.db' },
    externalSession: { reviewSourceForm: form => calls.push(form) },
    view: { original: {}, referenceError: null },
    sessions: new Map([['normal', { closeState: () => peer, view: () => ({ error: peer.error, referenceError: peer.referenceError }) }],
      ['chars', { closeState: () => active, view: () => ({ error: active.error, referenceError: null }) }]]),
    onchange() {},
  });
  for (const blocked of ['unsaved', 'busy', 'blocked']) {
    peer[blocked] = true; host.actions.openExternalTransfer();
    assert.equal(host.externalOpen, false); assert.deepEqual(calls, []);
    peer[blocked] = false; host.error = null;
  }
  for (const blocked of ['sourceBlocked', 'childBlocked']) {
    host[blocked] = true; host.actions.openExternalTransfer(); assert.equal(host.externalOpen, false);
    host[blocked] = false; host.error = null;
  }
  peer.referenceError = 'Inactive peer references failed'; host.actions.openExternalTransfer();
  assert.equal(host.externalOpen, false); peer.referenceError = null; host.error = null;
  host.error = 'Retained source validation'; host.actions.openExternalTransfer();
  assert.equal(host.externalOpen, false);
  host.error = null; host.sourceScope.contextId = 'foreign'; host.actions.openExternalTransfer();
  assert.equal(host.externalOpen, false);
  host.error = null; host.sourceScope.contextId = owner.contextId; host.actions.openExternalTransfer();
  assert.equal(host.externalOpen, true); assert.deepEqual(calls, ['FS882-8x6XL-CHARS']);
  assert.equal(host.actions.getDraftState().unsaved, true);
  assert.equal(host.actions.getDraftState().blocked, true);
});

test('source Return refreshes original and loaded peer variants; failure retains pane and receipt without replay', async () => {
  const calls = [];
  const peer = name => ({ closeState: () => ({ unsaved: false, busy: false, blocked: false }),
    view: () => ({ original: {}, error: null, referenceError: null }),
    load: async () => { calls.push(name); return true; } });
  const current = peer('chars'), normal = peer('normal');
  const host = componentFunctions('TwoPageEntryEditor.svelte', ['returnFromExternalTransfer'], {
    externalOpen: true, sourceRefreshing: false, error: null, session: current,
    externalEditor: { getCloseState: () => ({ unsaved: false, busy: false }) },
    sessions: new Map([['chars', current], ['normal', normal]]), onchange() {},
    oncommitted: async () => calls.push('parent'),
  });
  normal.load = async () => { calls.push('failed normal'); return false; };
  await host.actions.returnFromExternalTransfer();
  assert.equal(host.externalOpen, true); assert.equal(host.sourceRefreshing, false);
  assert.match(host.error, /refresh failed.*never replay/i);
  assert.deepEqual(calls, ['parent', 'chars', 'failed normal']);
  normal.load = async () => { calls.push('normal'); return true; };
  await host.actions.returnFromExternalTransfer();
  assert.equal(host.externalOpen, false); assert.equal(host.error, null);
  assert.deepEqual(calls.slice(3), ['parent', 'chars', 'normal']);
  host.externalEditor.getCloseState = () => ({ unsaved: true, busy: false });
  host.externalOpen = true; calls.length = 0;
  await host.actions.returnFromExternalTransfer();
  assert.deepEqual(calls, []); assert.equal(host.externalOpen, true);
});

test('source-held callbacks cannot stage, save, switch or close a retained source transfer', async () => {
  const host = componentFunctions('TwoPageEntryEditor.svelte',
    ['stage', 'selectProject', 'acknowledge', 'chooseForm', 'closeReview', 'operation'], {
      externalOpen: true, error: null, childBlocked: false, close: { unsaved: false, busy: false, blocked: false },
      onclosed: () => { throw new Error('No close permitted'); },
    });
  host.actions.stage('SiteUnitLongName', { kind: 'text', raw: 'new' });
  assert.match(host.error, /no field was staged/);
  host.actions.selectProject('1'); assert.match(host.error, /no ProjectID was staged/);
  host.actions.acknowledge('SiteUnitLongName'); assert.match(host.error, /no literal draft was acknowledged/);
  await host.actions.operation('save'); assert.match(host.error, /source transfer/);
  await host.actions.chooseForm('FS882-8x6XL'); assert.match(host.error, /before changing source variant/);
  host.actions.closeReview(); assert.match(host.error, /before closing/);
});

test('actual mapped picture activity holds parent callbacks and source action until owner-local Cancel completes', async () => {
  const source = readFileSync(`${__dirname}\\FS882Form.svelte`, 'utf8');
  const expression = source.match(/childBlocked=\{([^}]+)\}/)?.[1];
  assert.ok(expression);
  const keys = ['twoPageSoilBusy', 'twoPageSoilBlocked', 'twoPageVegetationBusy', 'twoPageVegetationBlocked', 'pictureBusy', 'pictureMetadataPending'];
  const blocked = new Function(...keys, `return ${expression};`);
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const calls = [];
  const host = componentFunctions('TwoPageEntryEditor.svelte',
    ['getDraftState', 'openExternalTransfer', 'stage', 'selectProject', 'operation', 'chooseForm', 'closeReview'], {
      owner, form: 'FS882-8x6XL', close: clean, error: null, editingEnabled: true, reviewEnabled: true,
      externalOpen: false, sourceRefreshing: false, sourceBlocked: false, childBlocked: false,
      view: { original: {}, drafts: {}, referenceError: null },
      sourceScope: { ...owner, projectPath: 'P.db', su: 'Separate', suPath: 'S.db' },
      externalSession: { reviewSourceForm: form => calls.push(['transfer', form]) },
      sessions: new Map(), session: {
        stage: () => calls.push('stage'), selectProject: () => calls.push('project'),
        load: () => calls.push('load'), save: () => calls.push('save'),
      }, onclosed: () => calls.push('close'), onchange() {},
    });
  for (const activity of keys) {
    host.childBlocked = blocked(...keys.map(key => key === activity));
    assert.equal(host.childBlocked, true, activity);
    host.actions.stage('SiteUnitLongName', { kind: 'text', raw: 'new' });
    assert.match(host.error, /linked-child ownership/);
    host.actions.selectProject('1'); assert.match(host.error, /linked-child ownership/);
    await host.actions.operation('save'); assert.match(host.error, /linked-child drafts, reads and receipts/);
    await host.actions.chooseForm('FS882-8x6XL-CHARS');
    host.actions.closeReview();
    host.actions.openExternalTransfer();
    assert.deepEqual(calls, [], activity);
    assert.equal(host.externalOpen, false);
    host.error = null;
  }
  host.childBlocked = blocked(...keys.map(() => false));
  host.actions.openExternalTransfer();
  assert.deepEqual(calls, [['transfer', 'FS882-8x6XL']]);
});

test('actual reference option keys preserve Zone DISTINCT tuples and duplicate physical code identities', () => {
  const source = readFileSync(`${__dirname}\\TwoPageEntryEditor.svelte`, 'utf8');
  const expression = source.match(/as choice \((.*)\)\}/)?.[1];
  assert.ok(expression);
  const key = new Function('choice', `return ${expression};`);
  const choices = [
    { rowId: '', code: 'BWBS', description: null },
    { rowId: '', code: 'BWBS', description: '' },
    { rowId: '', code: 'CWH', description: 'Coastal Western Hemlock' },
    { rowId: '1', code: 'DUP', description: 'Same code and title' },
    { rowId: '2', code: 'DUP', description: 'Same code and title' },
  ];
  assert.equal(new Set(choices.map(key)).size, choices.length);
  assert.equal(key({ ...choices[0] }), key(choices[0]));
  const Component = serverComponent(`<script>let { choices } = $props();</script>
    <datalist>{#each choices as choice (${expression})}<option value={choice.code}>{choice.description ?? ''}</option>{/each}</datalist>`,
  'TwoPageEntryReferenceOptions.svelte', {});
  const html = render(Component, { props: { choices } }).body;
  assert.equal((html.match(/<option /g) ?? []).length, choices.length);
});

test('actual root visibility expression hides the ordinary host for entry independently of the older source-layout gate', () => {
  const source = readFileSync(`${__dirname}\\FS882Form.svelte`, 'utf8');
  const expression = source.match(/<div hidden=\{([^}]*)\} data-two-page-ordinary-host>/)?.[1];
  assert.ok(expression);
  const Component = serverComponent(`<script>
    let { twoPageOpen, twoPageScope, twoPageSourceLayoutEnabled } = $props();
    </script><div hidden={${expression}} data-two-page-ordinary-host><input aria-label="Retained ordinary field" /></div>`,
  'TwoPageEntryHostVisibility.svelte', {});
  for (const twoPageOpen of [false, true]) for (const twoPageSourceLayoutEnabled of [false, true]) {
    for (const twoPageScope of ['entry', 'common', 'extra']) {
      const html = render(Component, { props: { twoPageOpen, twoPageSourceLayoutEnabled, twoPageScope } }).body;
      assert.match(html, /Retained ordinary field/);
      assert.equal(/<div hidden(?:[ =>])/.test(html), twoPageOpen && (twoPageScope === 'entry' || twoPageSourceLayoutEnabled), html);
    }
  }
});

test('complete-entry root activation preserves exclusive scope ownership and independent availability', () => {
  const root = componentFunctions('FS882Form.svelte', ['openTwoPageEntryReview'], {
    twoPageOpen: true, twoPageScope: 'common', twoPageEntryReviewEnabled: true, error: null,
    openTwoPageReview() { throw new Error('must not replace existing original/draft'); },
  });
  root.actions.openTwoPageEntryReview();
  assert.equal(root.twoPageScope, 'common');
  assert.match(root.error, /another two-page owner/);
  root.twoPageOpen = false; root.twoPageEntryReviewEnabled = false;
  root.actions.openTwoPageEntryReview();
  assert.equal(root.twoPageScope, 'common');
  assert.match(root.error, /disabled/);
  root.twoPageEntryReviewEnabled = true;
  root.openTwoPageReview = () => { root.twoPageOpen = true; };
  root.actions.openTwoPageEntryReview();
  assert.equal(root.twoPageScope, 'entry');
});

test('root refreshes every other parent scope without resetting dirty or unknown peers', async () => {
  const calls = [];
  const peer = name => ({
    view: () => ({ original: {}, error: null }),
    closeState: () => ({ unsaved: false, busy: false }),
    load: async () => { calls.push(name); return true; },
  });
  const extra = peer('extra'), common = peer('common'), entry = peer('entry');
  const key = JSON.stringify(owner);
  const cache = session => new Map([[key, new Map([['FS882-8x6XL', session]])]]);
  const root = componentFunctions('FS882Form.svelte', ['refreshTwoPagePeers'], {
    contextId: owner.contextId, twoPageProject: owner.project, draft: { plotNumber: owner.plot },
    twoPageSessions: cache(extra), twoPageCommonSessions: cache(common), twoPageEntrySessions: cache(entry),
    refreshSIVIParent: async () => calls.push('parent'),
    siviParentWriteSession: null, siviParentActionSession: null, siviProjectAssignmentSession: null, siviParentSharedSession: null,
  });
  await root.actions.refreshTwoPagePeers('entry');
  assert.deepEqual(calls, ['parent', 'extra', 'common']);
  calls.length = 0;
  await root.actions.refreshTwoPagePeers('extra');
  assert.deepEqual(calls, ['parent', 'common', 'entry']);
  calls.length = 0;
  entry.closeState = () => ({ unsaved: true, busy: false });
  await assert.rejects(root.actions.refreshTwoPagePeers('common'), /could not be refreshed/);
  assert.deepEqual(calls, ['parent', 'extra']);
  entry.closeState = () => ({ unsaved: false, busy: true });
  await assert.rejects(root.actions.refreshTwoPagePeers('common'), /could not be refreshed/);
});

test('actual complete-entry panel caches read ports per session and refuses replay across variant refreshes', async () => {
  const sessions = new Map(), calls = [];
  class Requests {
    track(request) { return request; }
    cancelAll() { calls.push('cancel'); }
  }
  class Session {
    constructor(owner, form, port) { this.owner = owner; this.form = form; this.port = port; }
    view() { return { original: {}, error: null }; }
    closeState() { return { unsaved: false, busy: false }; }
    async load() { calls.push(`load:${this.form}`); return true; }
  }
  const values = { owner, sessions, onchange() {}, ReadRequests: Requests, TwoPageEntrySession: Session,
    TwoPageParentEntryService: {
      GetOriginal: (context, plot, form) => { calls.push(['read', context, plot, form]); return Promise.resolve({}); },
      GetReferences: (context, plot, form, filters) => { calls.push(['references', context, plot, form, filters]); return Promise.resolve({}); },
      Save: (context, plot, form, request) => { calls.push(['save', context, plot, form, request]); return Promise.resolve({}); },
      Restore: (context, plot, form, history, action) => { calls.push(['restore', context, plot, form, history, action]); return Promise.resolve({}); },
    },
    oncommitted: async () => calls.push('parent'),
  };
  const mounted = componentFunctions('TwoPageEntryEditor.svelte', ['makeSession'], values);
  const normal = mounted.actions.makeSession('FS882-8x6XL'), chars = mounted.actions.makeSession('FS882-8x6XL-CHARS');
  const remounted = componentFunctions('TwoPageEntryEditor.svelte', ['makeSession'], values);
  assert.equal(remounted.actions.makeSession('FS882-8x6XL'), normal);
  const request = { edits: [] };
  await normal.port.save(request);
  assert.deepEqual(calls[0], ['save', owner.contextId, owner.plot, 'FS882-8x6XL', request]);
  await normal.port.refreshParent();
  assert.deepEqual(calls.slice(1), ['parent', 'load:FS882-8x6XL-CHARS']);
  chars.closeState = () => ({ unsaved: true, busy: false });
  await assert.rejects(normal.port.refreshParent(), /could not refresh/);
  assert.equal(calls.filter(call => Array.isArray(call) && call[0] === 'save').length, 1);
});

test('actual panel close-state barrier covers the open review even after clean atomic save', () => {
  const close = { unsaved: false, busy: false, blocked: false, canSave: true, saveReason: '', error: null };
  const panel = componentFunctions('TwoPageEntryEditor.svelte', ['getCloseState', 'closeReview'], {
    close, error: null, externalOpen: false, childBlocked: false, onclosed() {}, view: { error: null },
  });
  const barrier = panel.actions.getCloseState();
  assert.equal(barrier.unsaved, true);
  assert.equal(barrier.canSave, false);
  assert.match(barrier.saveReason, /close their review/);
  panel.close = { ...close, unsaved: true };
  panel.actions.closeReview();
  assert.match(panel.error, /Save or Undo/);
});
