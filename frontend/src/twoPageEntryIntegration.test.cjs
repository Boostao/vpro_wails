const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { render } = require('svelte/server');
const { componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const owner = { contextId: 'context:owned', project: 'OwnedProject', plot: 'P' };

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
    close, error: null, onclosed() {}, view: { error: null },
  });
  const barrier = panel.actions.getCloseState();
  assert.equal(barrier.unsaved, true);
  assert.equal(barrier.canSave, false);
  assert.match(barrier.saveReason, /close their review/);
  panel.close = { ...close, unsaved: true };
  panel.actions.closeReview();
  assert.match(panel.error, /Save or Undo/);
});
