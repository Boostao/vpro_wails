const assert = require('node:assert/strict');
const { test } = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const req = createRequire(path.join(__dirname, '..', 'package.json'));
const ts = req('typescript'), compiler = req('svelte/compiler'), runtime = req('svelte/internal/client');
const wire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldListWire.json'), 'utf8'));
const { physical, source, sourcePath, ...context } = wire.review;
const env = { VITE_SUMMARY_VEGETATION: 'true', VITE_SUMMARY_VEGETATION_FIELD_LIST: 'true' };
const plain = value => JSON.parse(JSON.stringify(value));
function execute(code, dependencies = {}, globals = {}) {
  const module = { exports: {} };
  const js = ts.transpileModule(code.replaceAll('import.meta.env', '__env'), { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS,
  } }).outputText;
  vm.runInNewContext(js, { ...globals, __env: env, module, exports: module.exports, setTimeout,
    crypto: { randomUUID: () => wire.request.requestId },
    require(id) { assert.ok(id in dependencies, `Uninjected import ${id}`); return dependencies[id]; } });
  return module.exports;
}
const client = execute(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldList.ts'), 'utf8'));
const registry = execute(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldListSessions.ts'), 'utf8'),
  { './summaryVegetationFieldList': client });
const bindings = { SummaryVegetationFieldListService: {
  GetSummaryVegetationFieldList: () => { throw new Error('No implicit network reads'); },
  SaveSummaryVegetationFieldList: () => { throw new Error('No implicit writes'); },
} };
const adapter = execute(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldListDesktop.ts'), 'utf8'),
  { './summaryVegetationFieldList': client, './summaryVegetationFieldListSessions': registry,
    '../bindings/github.com/boostao/vpro-wails': bindings });
function deferred() {
  let resolve, reject, cancellations = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancellations++; };
  return { promise, resolve, reject, get cancellations() { return cancellations; } };
}
function desktop(options = {}) {
  let gets = 0, saves = 0;
  const value = adapter.fieldListDesktop({
    read: id => { assert.equal(id, context.contextId); gets++; return options.read?.() ?? Promise.resolve(runtime.proxy(structuredClone(wire.review))); },
    save: (id, json) => { assert.equal(id, context.contextId); saves++; return options.save?.(json) ?? Promise.resolve(wire.outcome); },
    validationYield: options.validationYield,
  }, env, () => wire.request.requestId);
  return { value, gets: () => gets, saves: () => saves };
}
function scriptHandlers(filename, appended, dependencies, props) {
  let destroy;
  let script = fs.readFileSync(path.join(__dirname, filename), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('$props()', '__props') + appended;
  const stripped = ts.transpileModule(script.replaceAll('import.meta.env', '__env'), { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext,
  } }).outputText;
  const compiled = compiler.compileModule(stripped, { filename: filename + '.js', generate: 'client' });
  const exports = execute(compiled.js.code, { ...dependencies, 'svelte/internal/client': runtime,
    svelte: { untrack: runtime.untrack, onDestroy: callback => { destroy = callback; } } }, { __props: props });
  return { ...exports.handlers, destroy: () => destroy() };
}
function pane(state, enabled = true, disabled = false, validation = '') {
  return scriptHandlers('SummaryVegetationFieldList.svelte', `
    export const handlers = {load, save, undo, bulk, edit, correct, acknowledge, cancel, canStage, canLoad,
      view: () => view, update: (on: boolean, busy = false, error = '') => {
        enabledFromPresentationDraft = on; peerDisabled = busy; peerValidation = error;
      }};
  `, { './summaryVegetationFieldList': client }, { scope: state.scope, session: state.session,
    enabledFromPresentationDraft: enabled, peerDisabled: disabled, peerValidation: validation, onStateChange: () => {} });
}
function root(value) {
  const source = fs.readFileSync(path.join(__dirname, 'App.svelte'), 'utf8').replaceAll('\r\n', '\n');
  const effect = source.match(/  \$effect\(\(\) => \{\n    if \(!summaryVegetationFieldListDesktop.enabled\)[\s\S]*?\n  \}\);/)[0];
  const derived = source.match(/  const editorBusy = \$derived\([^\n]+/)[0];
  const compiled = compiler.compileModule(`
    let localEditorBusy = $state(false), pictureCreationBusy = false, pictureLifecycleBusy = false,
      twoPageSoilBusy = false, twoPageVegetationBusy = false, summaryPreferenceBusy = false, projectReportBusy = false,
      summaryEnvironmentalBusy = false, externalSUEnvironmentBusy = false, matrixPublicationBusy = false;
    let summaryFieldListBusy = $state(false), view = 'home';
    ${derived}
    ${effect.replace('$effect(', 'installEffect(')}
    export const handlers = { held: () => editorBusy, route: () => view,
      peer: value => { localEditorBusy = value; } };
  `, { filename: 'desktop-root-holds.svelte.js', generate: 'client' });
  return execute(compiled.js.code, { 'svelte/internal/client': runtime }, {
    summaryVegetationFieldListDesktop: value, installEffect: callback => callback(),
  }).handlers;
}
function report(value, { presentation = null, preferences = null, quality = null, owner = context } = {}) {
  const preview = { view: () => ({ busy: false, draftError: '', groupAndOrder: 11, average: 1, display: 1, showClass: 1 }),
    subscribe: callback => { callback(); return () => {}; }, cancel() {}, edit() {} };
  const changes = [];
  const dependencies = {
    '../bindings/github.com/boostao/vpro-wails': {},
    './readRequests': { ReadRequests: class { track(promise) { return promise; } cancelAll() {} } },
    './summaryVegetation': { summaryVegetationSession: () => preview },
    './summaryVegetationPreferenceSessions': { summaryVegetationPreferenceSession: preferences ? () => preferences : null },
    './summaryVegetationQualityPreferenceSessions': { summaryVegetationQualityPreferenceSession: quality ? () => quality : null },
    './summaryVegetationPresentationPreferenceSessions': { summaryVegetationPresentationPreferenceSession: presentation ? () => presentation : null },
    './summaryVegetationFieldListDesktop': { summaryVegetationFieldListDesktop: value },
    './summaryVegetationEnvironmentalDesktop': { summaryEnvironmentalEnabled: false, summaryEnvironmentalDesktop: {}, summaryEnvironmentalPort: {} },
  };
  return { ...scriptHandlers('SummaryVegetationReport.svelte', `
    export const handlers = { loadFieldList, dismissInitialFieldListReadError, loadPreferences, savePreferences, editPreference,
      held: () => fieldListHeld, peerHeld: () => fieldListPeerDisabled, peerError: () => fieldListPeerValidation,
      enabled: () => fieldListEnabledFromPresentation, state: () => fieldListState };
  `, dependencies, { ...owner, onBusyChange: busy => changes.push(busy) }), changes };
}
function peerView(initial) {
  let current = initial;
  const listeners = new Set();
  return { view: () => current, subscribe: callback => { listeners.add(callback); callback(); return () => listeners.delete(callback); },
    update: next => { current = next; for (const callback of listeners) callback(); } };
}

test('backend-off initial read remains a remount/root hold until explicit acknowledgement; fresh Load mandatory', async () => {
  let backendOff = true;
  const d = desktop({ read: () => backendOff
    ? Promise.reject(new Error('Summary environmental field-list is disabled in this session'))
    : Promise.resolve(runtime.proxy(structuredClone(wire.review))) });
  const r = root(d.value), first = report(d.value);
  await first.loadFieldList();
  assert.equal(r.held(), true);
  assert.equal(first.state().canDismissInitialReadError, true);
  first.destroy();
  const next = report(d.value);
  assert.match(next.state().readError, /disabled in this session/);
  assert.equal(r.held(), true);
  next.dismissInitialFieldListReadError();
  assert.equal(r.held(), false);
  assert.equal(next.state().readError, '');
  assert.equal(next.state().scope, null);
  assert.equal(next.state().session, null);
  assert.equal(d.gets(), 1); assert.equal(d.saves(), 0);
  next.destroy();
  const remounted = report(d.value);
  assert.equal(remounted.state().session, null);
  assert.equal(d.gets(), 1, 'acknowledgement and remount do not issue GET');
  backendOff = false;
  await remounted.loadFieldList();
  assert.equal(d.gets(), 2);
  assert.deepEqual(plain(remounted.state().session.view().review), wire.review);
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  remounted.destroy();
});

test('active and cancelled-but-unsettled bootstrap dispatch cannot acknowledge error or retry', async () => {
  const gate = deferred(), d = desktop({ read: () => gate.promise }), r = root(d.value), current = report(d.value);
  const loading = current.loadFieldList();
  assert.throws(() => d.value.dismissInitialReadError(context), /dispatch and validation/);
  d.value.cancel(context);
  assert.equal(current.state().readBusy, false);
  assert.equal(current.state().bootstrapPending, true);
  assert.equal(current.state().canDismissInitialReadError, false);
  assert.equal(r.held(), true);
  current.dismissInitialFieldListReadError();
  assert.match(current.state().readError, /cancelled/);
  assert.equal(await d.value.load(context), false);
  assert.equal(d.gets(), 1);
  assert.throws(() => d.value.dismissInitialReadError(context), /dispatch and validation/);
  gate.resolve(wire.review); await loading;
  assert.equal(current.state().scope, null);
  assert.equal(current.state().bootstrapPending, false);
  assert.equal(current.state().canDismissInitialReadError, true);
  current.dismissInitialFieldListReadError();
  assert.equal(r.held(), false);
  current.destroy();
});

test('cancelled validation yield blocks acknowledgement until stale validation settles without assigning authority', async () => {
  const gate = deferred(), d = desktop({ validationYield: () => gate.promise });
  const r = root(d.value), first = report(d.value), loading = first.loadFieldList();
  await Promise.resolve(); await Promise.resolve();
  first.destroy();
  const next = report(d.value);
  assert.equal(next.state().bootstrapPending, true);
  assert.throws(() => d.value.dismissInitialReadError(context), /dispatch and validation/);
  next.dismissInitialFieldListReadError();
  assert.equal(r.held(), true);
  gate.resolve(); await loading;
  assert.equal(next.state().session, null);
  assert.equal(next.state().canDismissInitialReadError, true);
  next.dismissInitialFieldListReadError();
  assert.equal(r.held(), false);
  next.destroy();
});

test('registry-created authority refuses bootstrap acknowledgement even if a subscriber retires owner assignment', async () => {
  const d = desktop();
  let busyNotifications = 0;
  const unsubscribe = d.value.subscribe(() => {
    if (d.value.activity().busy && !d.value.state(context).session && ++busyNotifications === 2)
      d.value.cancel(context);
  });
  assert.equal(await d.value.load(context), false);
  assert.equal(d.value.state(context).scope, null);
  assert.equal(d.value.state(context).bootstrapPending, false);
  assert.equal(d.value.state(context).canDismissInitialReadError, false);
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  unsubscribe();
  assert.equal(await d.value.load(context), true);
});

test('established authority/read errors/draft/Save receipt and dispatch can never use initial read acknowledgement', async () => {
  let fail = false;
  const saveGate = deferred(), d = desktop({ read: () => fail ? Promise.reject(new Error('established read failure')) : Promise.resolve(wire.review),
    save: () => saveGate.promise });
  const r = root(d.value), current = report(d.value);
  assert.throws(() => d.value.dismissInitialReadError(context), /No failed initial/);
  await current.loadFieldList();
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  fail = true;
  const p = pane(current.state());
  await p.load();
  assert.match(p.view().readError, /established read failure/);
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  current.dismissInitialFieldListReadError(); assert.equal(r.held(), true);
  fail = false; await p.load();
  await p.bulk(true);
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  const saving = p.save();
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  p.destroy(); current.destroy();
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  saveGate.resolve(wire.outcome); await saving;
  assert.equal(d.value.state(context).session.view().receipts.length, 1);
  assert.throws(() => d.value.dismissInitialReadError(context), /established/);
  assert.equal(r.held(), true);
});

test('visible acknowledgement control is explicit, settlement guarded and does not alter App/controller contracts', () => {
  const reportSource = fs.readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
  assert.match(reportSource, /data-summary-field-list-bootstrap-dismiss onclick=\{dismissInitialFieldListReadError\}/);
  assert.match(reportSource, /disabled=\{!fieldListState.canDismissInitialReadError\}/);
  assert.match(reportSource, /fresh successful Load is mandatory/);
  assert.match(reportSource, /Established drafts and Save receipts cannot be dismissed here/);
});

test('bootstrap validates actual Go DTO/proxy once, retains exact Unicode paths and remount does not GET', async () => {
  const d = desktop(), r = root(d.value), first = report(d.value);
  assert.equal(d.gets(), 0);
  await first.loadFieldList();
  assert.equal(d.gets(), 1);
  assert.equal(first.state().scope.sourcePath, wire.review.sourcePath);
  assert.deepEqual(plain(first.state().session.view().review), wire.review);
  assert.equal(r.held(), false);
  first.destroy();
  const next = report(d.value);
  assert.equal(next.state().session, first.state().session);
  assert.equal(d.gets(), 1);
  next.destroy();
});

test('failed bootstrap survives route remount and independently holds root until explicit correction', async () => {
  let fail = true;
  const d = desktop({ read: () => fail ? Promise.reject(new Error('owned read unavailable')) : Promise.resolve(wire.review) });
  const r = root(d.value), first = report(d.value);
  await first.loadFieldList(); first.destroy();
  assert.equal(r.held(), true);
  const next = report(d.value);
  assert.match(next.state().readError, /owned read unavailable/);
  assert.equal(next.peerHeld(), false, 'field-list errors are not its own peer errors');
  fail = false; await next.loadFieldList();
  assert.equal(r.held(), false); assert.equal(d.gets(), 2);
  next.destroy();
});

test('bootstrap cancellation during validation/remount never assigns old authority', async () => {
  const yieldGate = deferred();
  const d = desktop({ validationYield: () => yieldGate.promise }), r = root(d.value), first = report(d.value);
  const loading = first.loadFieldList();
  await Promise.resolve(); await Promise.resolve();
  assert.equal(r.held(), true);
  first.destroy();
  assert.equal(r.held(), true);
  yieldGate.resolve(); await loading;
  assert.equal(d.value.state(context).scope, null);
  assert.match(d.value.state(context).readError, /cancelled/);
});

test('cancel bootstrap network response then late result cannot populate remounted pane', async () => {
  const gate = deferred(), d = desktop({ read: () => gate.promise });
  const first = report(d.value), loading = first.loadFieldList();
  first.destroy(); const next = report(d.value);
  assert.equal(gate.cancellations, 1);
  gate.resolve(wire.review); await loading;
  assert.equal(next.state().session, null);
  next.destroy();
});

test('registry bootstrap notification cancellation cannot assign a retired owner; explicit retry remains safe', async () => {
  const d = desktop();
  let busyNotifications = 0;
  const unsubscribe = d.value.subscribe(() => {
    if (d.value.activity().busy && !d.value.state(context).session && ++busyNotifications === 2)
      d.value.cancel(context);
  });
  assert.equal(await d.value.load(context), false);
  assert.equal(d.value.state(context).scope, null);
  assert.match(d.value.state(context).readError, /cancelled/);
  unsubscribe();
  assert.equal(await d.value.load(context), true);
  assert.equal(d.gets(), 2);
  assert.equal(d.value.activity().blocked, false);
});

test('handoff controller Load validation cancellation remains a retained root hold with pane recovery', async () => {
  let yields = 0;
  const gate = deferred(), d = desktop({ validationYield: () => ++yields === 1 ? Promise.resolve() : gate.promise });
  const r = root(d.value), first = report(d.value);
  const loading = first.loadFieldList();
  for (let turn = 0; turn < 8; turn++) await Promise.resolve();
  assert.ok(first.state().session);
  assert.equal(first.state().session.view().review, null);
  first.destroy();
  gate.resolve(); await loading;
  assert.equal(r.held(), true);
  const next = report(d.value), recovery = pane(next.state(), false);
  assert.match(recovery.view().readError, /cancelled/);
  await recovery.load();
  assert.equal(r.held(), false);
  assert.equal(d.gets(), 2);
  recovery.destroy(); next.destroy();
});

test('complete five-field context mismatch/malformed Unicode and physical DTO never establishes session', async () => {
  for (const change of [v => { v.projectPath += 'x'; }, v => { v.su = 'different'; },
    v => { v.sourcePath = '\ud800'; }, v => { delete v.physical.columns[0].declaredType; },
    v => { v.extra = true; }]) {
    const bad = structuredClone(wire.review); change(bad);
    const d = desktop({ read: () => Promise.resolve(runtime.proxy(bad)) });
    assert.equal(await d.value.load(context), false);
    assert.equal(d.value.state(context).session, null);
    assert.equal(d.value.activity().blocked, true);
  }
  const d = desktop();
  assert.throws(() => d.value.state({ ...context, projectPath: '\ud800' }), /Unicode/);
  assert.throws(() => d.value.state({ ...context, su: 'normal', suPath: '' }), /five-field/);
  const unavailable = report(d.value, { owner: { ...context, contextId: '' } });
  await unavailable.loadFieldList(); assert.equal(unavailable.state(), null);
  unavailable.destroy();
  assert.equal(d.gets(), 0);
});

test('selected normal scope preserves Unicode URI-delimiter paths with exact source provenance', async () => {
  const owner = { ...context, su: '  Literal 𐐀 ', projectPath: 'C:\\owned\\Projet #é\\project.sqlite',
    suPath: 'C:\\owned\\Unité ?#%𐐀\\scope.sqlite' };
  const review = { ...wire.review, ...owner, sourcePath: 'C:\\owned\\VPro64 #é?%𐐀.sqlite' };
  const d = desktop({ read: () => Promise.resolve(runtime.proxy(review)) });
  const current = report(d.value, { owner });
  await current.loadFieldList();
  assert.deepEqual(plain(current.state().scope), { ...owner, sourcePath: review.sourcePath });
  assert.deepEqual(plain(current.state().session.view().review), review);
  assert.equal(d.gets(), 1);
  current.destroy();
});

test('None with empty path belongs only to field-list context, no fabricated source path', async () => {
  const owner = { ...context, su: 'None', suPath: '' };
  const d = desktop({ read: () => Promise.resolve({ ...wire.review, ...owner }) });
  const current = report(d.value, { owner });
  await current.loadFieldList();
  assert.equal(current.state().scope.suPath, '');
  assert.equal(current.state().scope.sourcePath, sourcePath);
  const app = fs.readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  assert.ok(app.includes('!state?.contextId || !state.projectPath || !state.suPath'));
  current.destroy();
});

test('includeEnv requires reviewed presentation authority; unavailable/off keeps recovery but not mutation live', async () => {
  const d = desktop(), current = report(d.value);
  await current.loadFieldList();
  assert.equal(current.enabled(), false);
  const p = pane(current.state(), current.enabled());
  await p.bulk(true); await p.save();
  assert.equal(p.view().actions.length, 0); assert.equal(d.saves(), 0);
  await p.load(); assert.equal(d.gets(), 2);
  p.destroy(); current.destroy();
  const presentation = peerView({ saved: { includeEnv: false }, draft: { includeEnv: false } });
  const next = report(d.value, { presentation });
  assert.equal(next.enabled(), false);
  presentation.update({ saved: { includeEnv: false }, draft: { includeEnv: true } });
  assert.equal(next.enabled(), true);
  assert.equal(d.saves(), 0); assert.equal(next.state().session.view().actions.length, 0);
  presentation.update({ saved: { includeEnv: true }, draft: { includeEnv: true }, readError: 'unavailable' });
  assert.equal(next.enabled(), false);
  next.destroy();
});

test('peer busy/errors block field actions; own field errors do not masquerade as peer validation', async () => {
  const quality = peerView({ busy: true });
  const d = desktop(), current = report(d.value, { quality });
  await current.loadFieldList(); assert.equal(d.gets(), 0);
  quality.update({ readError: 'invalid quality' });
  await current.loadFieldList(); assert.equal(d.gets(), 0);
  quality.update({});
  await current.loadFieldList(); assert.equal(d.gets(), 1);
  const p = pane(current.state(), true, false, 'invalid peer');
  await p.bulk(true); assert.equal(p.view().actions.length, 0);
  p.update(true); await p.bulk(true);
  assert.equal(p.view().actions.length, 1);
  assert.equal(current.held(), true); assert.equal(current.peerError(), '');
  await p.undo(); assert.equal(current.held(), false);
  p.destroy(); current.destroy();
});

test('field-list draft holds sibling option Save and root without self-blocking Undo/recovery', async () => {
  let saves = 0;
  const preferences = peerView({ saved: {}, draft: {}, initialized: false });
  preferences.save = async () => { saves++; };
  preferences.cancelLoad = () => {};
  const d = desktop(), r = root(d.value), current = report(d.value, { preferences });
  await current.loadFieldList();
  const p = pane(current.state());
  await p.bulk(true);
  await current.savePreferences();
  assert.equal(saves, 0); assert.equal(r.held(), true);
  assert.equal(current.peerHeld(), false);
  preferences.editOption = (_field, raw) => preferences.update({
    saved: {}, draft: {}, initialized: false, draftError: raw === 'bad' ? 'invalid peer draft' : '',
  });
  current.editPreference('average', 'bad');
  assert.match(current.peerError(), /options validation/);
  current.editPreference('average', '1');
  assert.equal(current.peerError(), '', 'peer correction remains live during a field-list draft hold');
  await p.undo();
  assert.equal(r.held(), false);
  p.destroy(); current.destroy();
});

test('root holds staged fields, read cancellation and late actual Save delivery independently of destroyed Report/pane', async () => {
  let committed = false;
  const gate = deferred(), d = desktop({ read: () => Promise.resolve(committed ? wire.after : wire.review), save: json => {
    assert.deepEqual(JSON.parse(json), wire.request); return gate.promise;
  } });
  const r = root(d.value), current = report(d.value);
  await current.loadFieldList();
  const p = pane(current.state());
  for (const action of wire.request.actions) {
    if (action.kind === 'all' || action.kind === 'none') await p.bulk(action.kind === 'all');
    else await p.edit(action.rowId, action.selected);
  }
  assert.equal(r.held(), true); assert.equal(current.held(), true);
  const saved = p.save();
  assert.equal(d.saves(), 1);
  p.destroy(); current.destroy();
  assert.equal(r.held(), true);
  assert.equal(d.value.state(context).session.view().dispatchSettled, false);
  committed = true; gate.resolve(wire.outcome); await saved;
  assert.equal(r.held(), true);
  const remounted = report(d.value), recovery = pane(remounted.state(), false);
  assert.equal(d.gets(), 1); assert.equal(d.saves(), 1);
  assert.equal(recovery.view().dispatchSettled, true);
  assert.equal(recovery.view().receipts.length, 1);
  assert.equal(recovery.view().receipts[0].kind, 'unknown');
  assert.equal(recovery.view().review.physical.rows[0].cells[4].integer,
    wire.review.physical.rows[0].cells[4].integer, 'late response cannot assign old-generation authority');
  await recovery.acknowledge(); assert.equal(r.held(), true);
  await recovery.load();
  await recovery.correct();
  assert.equal(recovery.view().review.physical.rows[0].cells[4].integer,
    wire.after.physical.rows[0].cells[4].integer);
  await recovery.acknowledge();
  assert.equal(r.held(), false);
  r.peer(true); assert.equal(r.held(), true);
  r.peer(false); assert.equal(r.held(), false);
  recovery.destroy(); remounted.destroy();
});

test('actual Save validation yield survives root subscription while remount prevents stale assignment', async () => {
  let hold = false;
  const validation = deferred(), d = desktop({ validationYield: () => hold ? validation.promise : Promise.resolve() });
  const r = root(d.value), current = report(d.value);
  await current.loadFieldList();
  const p = pane(current.state());
  for (const action of wire.request.actions) {
    if (action.kind === 'all' || action.kind === 'none') await p.bulk(action.kind === 'all');
    else await p.edit(action.rowId, action.selected);
  }
  hold = true;
  const saving = p.save();
  await Promise.resolve(); await Promise.resolve();
  assert.equal(r.held(), true);
  p.destroy(); current.destroy();
  assert.equal(d.value.state(context).session.view().dispatchSettled, false);
  const remounted = report(d.value);
  validation.resolve(); await saving;
  assert.equal(remounted.state().session.view().receipts[0].kind, 'unknown');
  assert.equal(remounted.state().session.view().review.physical.rows[0].cells[4].integer, '0');
  assert.equal(r.held(), true);
  assert.equal(d.saves(), 1); assert.equal(d.gets(), 1);
  remounted.destroy();
});
