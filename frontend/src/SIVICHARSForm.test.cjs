const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync, existsSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compileModule, compile } = require('svelte/compiler');
const runtime = require('svelte/internal/client');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { review, cell } = require('./siviSourceFixture.cjs');
const wire = require('../../testdata/chars-sivi-entry-client-wire.json');
const source = name => readFileSync(path.join(__dirname, name), 'utf8');
const clone = value => structuredClone(value);
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

function fixture(options = {}) {
  const calls = { parentRead: 0, childRead: 0, parentSave: 0, childSave: 0, restore: 0, refresh: 0, cancelled: 0 };
  const originalParent = clone(wire);
  let currentParent = clone(wire);
  const originalChild = review({ extended: true });
  for (const group of originalChild) for (const row of group.Rows) {
    row.cells[group.Columns.indexOf('PlotNumber')] = cell('text', wire.Original.Plot);
  }
  let currentChild = clone(originalChild);
  const cleanups = [];
  const parentProps = { contextId: wire.Original.ContextID, project: wire.Original.Project, plotNumber: wire.Original.Plot };
  const environment = { VITE_SIVI_CHARS_PARENT_ENTRY: 'true', VITE_SIVI_COMBINED_EDITING: 'true', ...options.environment };
  const api = {
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
    SIVICHARSParentEntryService: {
      Review: async () => {
        calls.parentRead++;
        return options.parentRead ? options.parentRead(calls.parentRead, clone(currentParent)) : clone(currentParent);
      },
      Save: async (_context, _plot, request) => {
        calls.parentSave++;
        for (const edit of request.edits) {
          const binding = currentParent.Original.Bindings.find(row => row.Binding === edit.column);
          const target = currentParent.Original.Rows[0][binding.Table.endsWith('_Env') ? 'Env' : 'Admin'];
          target.cells[binding.Column] = clone(edit.value);
        }
        return { ChangedCells: request.edits.length + (request.actions?.length ?? 0), HistoryID: '101' };
      },
      Restore: async () => { currentParent = clone(originalParent); return { restoredRows: 1, prunedAuditRows: 1,
        cleanedVegRows: 0, cancelled: false }; },
    },
    SIVICombinedService: {
      GetOriginal: async (context, plot, extended) => {
        calls.childRead++;
        assert.equal(context, wire.Original.ContextID);
        assert.equal(plot, wire.Original.Plot);
        assert.equal(extended, true);
        if (options.read) return options.read(calls.childRead, clone(currentChild));
        return clone(currentChild);
      },
      SaveReviewed: async (_context, _plot, extended, raw) => {
        calls.childSave++;
        assert.equal(extended, true);
        const request = JSON.parse(raw);
        calls.request = request;
        for (const edit of request.edits) {
          for (const group of currentChild) for (const row of group.Rows) {
            const index = group.Columns.indexOf(edit.column);
            if (row.rowId === edit.rowId && index >= 0) row.cells[index] = clone(edit.value);
          }
        }
        if (options.save) return options.save(request);
        return { ChangedCells: request.edits.length, HistoryID: '201' };
      },
      RestoreReviewed: async (_context, _plot, history, action) => {
        calls.restore++;
        assert.equal(history, '201');
        currentChild = clone(originalChild);
        return { restoredRows: calls.request.edits.length, prunedAuditRows: action === 'prune' ? calls.request.edits.length : 0,
          cleanedVegRows: 0, cancelled: false };
      },
    },
  };
  const cache = new Map();
  function load(filename) {
    if (cache.has(filename)) return cache.get(filename);
    const module = { exports: {} };
    cache.set(filename, module.exports);
    const code = ts.transpileModule(readFileSync(filename, 'utf8'), {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022, esModuleInterop: true },
    }).outputText;
    new Function('module', 'exports', 'require', code)(module, module.exports, request => dependency(request, filename));
    cache.set(filename, module.exports);
    return module.exports;
  }
  function dependency(request, filename) {
    if (request === '../bindings/github.com/boostao/vpro-wails') return api;
    if (request === './readRequests') return { ReadRequests: class {
      track(promise) { return promise; }
      cancelAll() { calls.cancelled++; options.cancel?.(); }
    } };
    if (request === 'svelte') return { untrack: runtime.untrack, onMount() {}, onDestroy: callback => cleanups.push(callback) };
    if (request === 'svelte/internal/client') return runtime;
    if (request.endsWith('.svelte')) return { default() {} };
    const target = path.resolve(path.dirname(filename), request);
    if (target.endsWith('.json')) return JSON.parse(readFileSync(target, 'utf8'));
    if (existsSync(target + '.ts')) return load(target + '.ts');
    throw new Error('Unexpected client test dependency: ' + request);
  }
  function component(name, props, extra) {
    const filename = path.join(__dirname, name);
    let script = source(name).match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
    const ast = ts.createSourceFile(name + '.ts', script, ts.ScriptTarget.Latest, true);
    const declaration = ast.statements.find(node => ts.isVariableStatement(node) && node.getText(ast).includes('$props()'));
    assert.ok(declaration);
    const properties = name === 'SIVICHARSForm.svelte' ? 'contextId, project, plotNumber, onSaved, onBusyChange'
      : 'contextId, project, plotNumber, onSaved, onBusyChange, linkedVegetation';
    script = script.slice(0, declaration.getStart(ast)) + `const { ${properties} } = __props;` +
      script.slice(declaration.end);
    if (name === 'SIVICHARSParentForm.svelte') script = script.replaceAll('peerBlockedReason', '__props.peerBlockedReason');
    const javascript = ts.transpileModule(script.replaceAll('import.meta.env', '__environment'), {
      compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
    }).outputText;
    const compiled = compileModule(`import { props as __props, environment as __environment } from 'test-owner';
      ${javascript}\n${extra}`, { filename: name + '.svelte.js', generate: 'client' });
    assert.equal(compiled.warnings.length, 0);
    const module = { exports: {} };
    const code = ts.transpileModule(compiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText;
    new Function('module', 'exports', 'require', code)(module, module.exports, request =>
      request === 'test-owner' ? { props, environment } : dependency(request, filename));
    return module.exports;
  }
  let parent, form;
  const disposeEffects = runtime.effect_root(() => {
    form = component('SIVICHARSForm.svelte', {
      ...parentProps, onSaved: async () => { calls.refresh++; await options.refresh?.(); },
    }, `export function bindParent(value) { parent = value; }
      export function perform(kind) { return operate(kind); }
      export function draft(row, column, value, nil = false) { stage(row, column, value, nil); }
      export function snapshot() { return childView; }
      export function peerReason() { return peerBlockedReason; }
      export function parentSaved() { return refreshAfterParentSave(); }
      export function cancelSource() { cancelRead(); }`);
    parentProps.onSaved = form.parentSaved;
    Object.defineProperty(parentProps, 'peerBlockedReason', { get: form.peerReason });
    parent = component('SIVICHARSParentForm.svelte', parentProps, `
      export function loadSource() { return operation('load'); }
      export function unlock() { locked = false; }
      export function draft(column, input) { stage(column, input); }
      export function snapshot() { return view; }
      export function undoSource() { return operation('undo'); }
      export function restoreSource() { return operation('prune'); }`);
    form.bindParent(parent);
  });
  return { calls, parent, form, rowId: originalChild[0].Rows[0].rowId,
    dispose() { for (const cleanup of cleanups.reverse()) cleanup(); disposeEffects(); } };
}

test('compiled linked snippet requires both exact literal flags and executes only inside the parent', () => {
  for (const parent of [undefined, 'false', 'TRUE', 'true']) for (const combined of [undefined, 'false', 'TRUE', 'true']) {
    let rendered = 0;
    const parentStub = serverComponent(`<script lang="ts">
      import { record } from 'proof'; let { linkedVegetation } = $props();
      </script><section>{record(Boolean(linkedVegetation))}{#if linkedVegetation}{@render linkedVegetation()}{/if}</section>`, 'linked-parent.svelte',
      { proof: { record: value => { if (value) rendered++; return ''; } } });
    const component = serverComponent(source('SIVICHARSForm.svelte'), 'SIVICHARSForm.svelte', {
      './SIVICHARSParentForm.svelte': { default: parentStub }, './SIVICombinedPanel.svelte': { default() {} },
      '../bindings/github.com/boostao/vpro-wails': { SIVICombinedService: {}, AuditRestoreAction: {} },
      './siviCombinedSession': { SIVICombinedSession: class {
        view() { return { busy: false }; }
        closeState() { return { busy: false, unsaved: false, blocked: false }; }
        dispose() {}
      } },
    }, { VITE_SIVI_CHARS_PARENT_ENTRY: parent, VITE_SIVI_COMBINED_EDITING: combined });
    const html = render(component, { props: { contextId: 'owned', project: 'Sample', plotNumber: 'P' } }).body;
    const enabled = parent === 'true' && combined === 'true';
    assert.equal(rendered, enabled ? 1 : 0);
    assert.equal(html.includes('data-chars-linked-vegetation'), enabled);
    assert.equal((html.match(/data-chars-linked-open/g) ?? []).length, enabled ? 1 : 0);
    assert.doesNotMatch(html, /data-sivi-combined-column/);
  }
  assert.equal(compile(source('SIVICHARSForm.svelte'), { filename: 'SIVICHARSForm.svelte', generate: 'client' }).warnings.length, 0);
  const parent = source('SIVICHARSParentForm.svelte');
  assert.match(parent, /group\.name === 'Vegetation'\}\{@render linkedVegetation\?\.\(\)\}/);
  assert.ok(parent.indexOf("name: 'Vegetation'") < parent.indexOf("name: 'Notes'"));
  const app = source('App.svelte');
  assert.match(app, /const siviCHARSLinkedVegetationEnabled = siviCHARSParentEntryEnabled && import\.meta\.env\.VITE_SIVI_COMBINED_EDITING === 'true'/);
  assert.match(app, /charsLinkedVegetation=\{siviCHARSLinkedVegetationEnabled\}/);
});

test('actual client owners react across peer drafts, lock, invalid correction, close Save and independent restoration', async () => {
  const f = fixture();
  try {
    assert.equal(await f.parent.loadSource(), true);
    assert.equal(await f.form.perform('load'), false);
    f.parent.unlock();
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.perform('load'), false);
    assert.equal(f.calls.childRead, 0);
    assert.equal(await f.parent.undoSource(), true);
    assert.equal(await f.form.perform('load'), true, f.form.snapshot().error);
    assert.equal(f.form.snapshot().extended, true);
    assert.equal(f.form.snapshot().review[0].Form, 'SubVegA-SIVI');
    f.form.draft(f.rowId, 'Cover5a', '100');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(f.form.getCloseState().canSave, false);
    const refreshes = f.calls.refresh;
    await assert.rejects(f.form.parentSaved(), /Linked vegetation changed/);
    assert.equal(f.calls.refresh, refreshes, 'dirty peer preflight must precede header refresh');
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'BLOCK' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    assert.match(f.parent.getCloseState().error, /linked vegetation/);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 0);
    f.form.draft(f.rowId, 'Cover5a', '3.5');
    f.form.draft(f.rowId, 'HeightB', 'literal height');
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.form.getCloseState().canSave, true);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.request.edits.length, 2);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().unsaved, false);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.perform('prune'), false);
    assert.equal(f.calls.restore, 0);
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.calls.parentSave, 1);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(f.form.snapshot().historyId, '201', 'parent refresh must not erase the child receipt');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(f.calls.restore, 1);
    assert.equal(f.parent.snapshot().historyId, '101', 'child refresh must not erase the parent receipt');
    assert.equal(await f.parent.restoreSource(), true);
    assert.equal(f.parent.snapshot().historyId, null);
  } finally { f.dispose(); }
});

test('postcommit refresh failure blocks replay and retains the receipt until explicit recovery', async () => {
  let fail = false;
  const f = fixture({ refresh: async () => { if (fail) throw new Error('held header failure'); } });
  try {
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    assert.equal(await f.form.perform('load'), true, f.form.snapshot().error);
    f.form.draft(f.rowId, 'Cover5b', '2');
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.calls.childSave, 1);
    fail = false;
    assert.equal(await f.form.perform('undo'), true);
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(await f.form.perform('prune'), true);
  } finally { f.dispose(); }
});

test('failed parent source refresh recovers in place without losing either history or replaying child Save', async () => {
  let fail = false;
  const f = fixture({ parentRead: (_count, value) => {
    if (fail) throw new Error('transient owned parent read failure');
    return value;
  } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'FINE' });
    assert.equal(await f.form.saveForClose(), true);
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.perform('load'), true);
    f.form.draft(f.rowId, 'Cover5b', '2');
    fail = true;
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(f.parent.snapshot().original, null);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().blocked, true);
    assert.equal(await f.parent.undoSource(), false, 'ordinary parent Undo must not bypass the blocked peer');
    assert.equal(await f.form.saveForClose(), false);
    assert.equal(await f.form.perform('undo'), false, 'failed recovery remains explicit and retryable');
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.parent.snapshot().historyId, '101');
    fail = false;
    assert.equal(await f.form.perform('undo'), true);
    assert.equal(f.form.getCloseState().blocked, false);
    assert.equal(f.calls.childSave, 1);
    assert.equal(f.calls.parentSave, 1);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.parent.snapshot().historyId, '101');
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(await f.parent.restoreSource(), true);
  } finally { f.dispose(); }
});

test('held source read blocks the root and cancellation/retirement never populates a replacement owner', async () => {
  const hold = deferred();
  const f = fixture({ read: (count, value) => count === 1 ? hold.promise : Promise.resolve(value),
    cancel: () => hold.reject(new Error('owned read cancelled')) });
  try {
    assert.equal(await f.parent.loadSource(), true);
    f.parent.unlock();
    const pending = f.form.perform('load');
    assert.equal(f.form.getCloseState().busy, true);
    assert.equal(f.form.getCloseState().canSave, false);
    f.form.cancelSource();
    assert.equal(await pending, false);
    assert.match(f.form.snapshot().error, /cancelled/);
    assert.equal(f.form.getCloseState().busy, false);
    assert.equal(f.calls.childSave, 0);
    assert.equal(await f.form.perform('load'), true);
    assert.equal(f.form.snapshot().error, null);
  } finally { f.dispose(); }
  const late = deferred();
  const old = fixture({ read: () => late.promise });
  assert.equal(await old.parent.loadSource(), true);
  old.parent.unlock();
  const pending = old.form.perform('load');
  old.dispose();
  const replacement = fixture();
  try {
    assert.equal(await replacement.parent.loadSource(), true);
    replacement.parent.unlock();
    late.resolve(review({ extended: true }));
    assert.equal(await pending, false);
    assert.equal(replacement.form.snapshot().review, null);
    assert.equal(replacement.calls.childRead, 0);
  } finally { replacement.dispose(); }
});

test('backend read refusal remains visible and correction/retry uses the same owner without mutation', async () => {
  const f = fixture({ read: (count, value) => {
    if (count === 1) throw new Error('SIVI combined cover/height editing is disabled in this session');
    return value;
  } });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), false);
    assert.equal(f.form.snapshot().review, null);
    assert.match(f.form.snapshot().error, /disabled in this session/);
    assert.equal(f.calls.childSave, 0);
    assert.equal(await f.form.perform('load'), true);
    assert.equal(f.form.snapshot().error, null);
    assert.equal(f.calls.childRead, 2);
  } finally { f.dispose(); }
});

test('held mutation cannot be cancelled or replayed and guards parent edits until its actual acknowledgement', async () => {
  const hold = deferred();
  const f = fixture({ save: () => hold.promise });
  try {
    assert.equal(await f.parent.loadSource(), true); f.parent.unlock();
    assert.equal(await f.form.perform('load'), true);
    f.form.draft(f.rowId, 'Cover5c', '1');
    const saving = f.form.saveForClose();
    assert.equal(f.form.getCloseState().busy, true);
    const cancellations = f.calls.cancelled;
    f.form.cancelSource();
    assert.equal(f.calls.cancelled, cancellations);
    assert.match(f.form.getCloseState().error, /Only a linked source read/);
    assert.equal(await f.form.saveForClose(), false);
    f.parent.draft('RootZoneParticleSize', { kind: 'text', raw: 'BLOCK' });
    assert.equal(f.parent.getCloseState().unsaved, false);
    assert.equal(f.calls.childSave, 1);
    hold.resolve({ ChangedCells: 1, HistoryID: '201' });
    assert.equal(await saving, true);
    assert.equal(f.form.snapshot().historyId, '201');
    assert.equal(f.form.getCloseState().busy, false);
    assert.equal(f.form.getCloseState().error, null);
    assert.equal(await f.form.perform('prune'), true);
    assert.equal(f.calls.childSave, 1);
  } finally { f.dispose(); }
});
