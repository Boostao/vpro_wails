const assert = require('node:assert/strict');
const { test } = require('node:test');
const { webcrypto } = require('node:crypto');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy, state, get } = require('svelte/internal/client');
const { render } = require('svelte/server');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'project-metadata-template.json'))),
});
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': editor });
const forward = loadTypeScript('environmentSUTransfer.ts', { './projectMetadataRestore': cells, './projectMetadataEditor': editor });
const reverse = loadTypeScript('siteUnitEnvironment.ts', {
  './projectMetadataRestore': cells, './projectMetadataEditor': editor, './environmentSUTransfer': forward,
});
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
class Reads {
  pending = new Set();
  async track(value) { this.pending.add(value); try { return await value; } finally { this.pending.delete(value); } }
  cancelAll() { for (const value of this.pending) value.cancel(); this.pending.clear(); }
}
const external = loadTypeScript('externalSUEnvironment.ts', {
  './siteUnitEnvironment': reverse, './googleEarthReview': earth, './qualityEditor': quality, './readRequests': { ReadRequests: Reads },
}, { crypto: webcrypto });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'externalSUEnvironmentWire.json'), 'utf8'));
const scope = { contextId: wire.review.review.contextId, project: wire.review.review.project,
  projectPath: wire.review.projectPath, su: wire.review.review.su, suPath: wire.review.suPath };
const plain = value => JSON.parse(JSON.stringify(value));
const review = () => structuredClone(wire.review);
const promise = value => Object.assign(Promise.resolve(value), { cancel() {} });
const session = () => new external.ExternalSUEnvironmentSession(scope);
async function unknown(target) {
  await target.transfer(review(), () => Object.assign(Promise.reject(new Error('lost response')), { cancel() {} }), wire.request.requestId);
  assert.equal(target.view().blocked, true); assert.equal(target.view().outcome, null);
}

test('strict production wire binds complete separate paths, source, approval and immutable typed changes', () => {
  const original = review(), checked = external.validateExternalSUEnvironmentReview(original, scope);
  assert.deepEqual(plain(checked), wire.review);
  original.review.source.rows[0].cells[1].text = 'changed';
  assert.equal(checked.review.source.rows[0].cells[1].text, 'S');
  assert.deepEqual(plain(external.validateExternalSUEnvironmentOutcome(wire.outcome, checked)), wire.outcome);
});

test('external reviews and receipts refuse foreign scope, malformed source and incomplete acknowledgement', () => {
  for (const mutate of [
    value => { value.suPath += 'x'; }, value => { value.projectPath = value.suPath; },
    value => { value.review.contextId = 'foreign'; }, value => { value.review.su = 'None'; },
    value => { value.sourceHash = 'x'; }, value => { value.approvalHash = 'A'.repeat(64); },
    value => { value.sessionId = ''; }, value => { value.extra = true; },
    value => { value.review.source.rows[0].cells[1].text = '\ud800'; },
    value => { value.review.master.columns[0].type = 'TEXT'; delete value.review.master.columns[0].declaredType; },
  ]) { const value = review(); mutate(value); assert.throws(() => external.validateExternalSUEnvironmentReview(value, scope)); }
  for (const value of [
    null, { ...wire.outcome, receipt: null }, { ...wire.outcome, status: 'published' },
    { ...wire.outcome, receipt: { ...wire.outcome.receipt, changedCells: 2 } },
    { ...wire.outcome, receipt: { ...wire.outcome.receipt, changedRows: 3 } },
    { ...wire.outcome, receipt: { ...wire.outcome.receipt, historyId: '0' } },
    { status: 'notCommitted', receipt: wire.outcome.receipt, errorMessage: 'failure' },
    { status: 'unknown', receipt: null, errorMessage: '' },
  ]) assert.throws(() => external.validateExternalSUEnvironmentOutcome(value, review()));
});

test('actual client raw signal keeps approved transport cloneable; a deep proxy cannot masquerade as reviewed data', async () => {
  const raw = state(review()), target = session();
  await target.transfer(get(raw), () => promise(wire.outcome), wire.request.requestId);
  assert.equal(target.view().outcome.receipt.historyId, '1');
  assert.throws(() => external.validateExternalSUEnvironmentReview(proxy(review()), scope), /clone/i);
  const source = readFileSync(path.join(__dirname, 'ExternalSUEnvironmentTransfer.svelte'), 'utf8');
  assert.match(source, /review = \$state\.raw/);
});

test('one attempt retains UUID/service/approval and immutable receipt through explicit acknowledgement', async () => {
  const target = session(), calls = [];
  await target.transfer(review(), request => { calls.push(plain(request)); return promise(wire.outcome); }, wire.request.requestId);
  assert.deepEqual(calls, [wire.request]); assert.equal(target.closeState().blocked, true);
  await assert.rejects(() => target.transfer(review(), () => promise(wire.outcome)), /prior/);
  target.view().outcome.receipt.historyId = 'changed';
  assert.equal(target.view().outcome.receipt.historyId, '1');
  target.acknowledge(); assert.equal(target.closeState().blocked, false);
  assert.equal(target.view().outcome.receipt.historyId, '1');
});

test('actual source variant is retained with its exact attempt, not rewritten by a later review or acknowledgement', async () => {
  const target = session();
  assert.equal(target.requestSourceForm(), null);
  target.reviewSourceForm('FS882-8x6XL-CHARS');
  await unknown(target);
  assert.equal(target.requestSourceForm(), 'FS882-8x6XL-CHARS');
  assert.throws(() => target.reviewSourceForm('FS882-8x6XL'), /identity cannot change/);
  assert.throws(() => target.reviewSourceForm(null), /identity cannot change/);
  await target.recover(() => promise(wire.recovery));
  target.acknowledge();
  target.reviewSourceForm(null);
  assert.equal(target.requestSourceForm(), 'FS882-8x6XL-CHARS');
  await target.transfer(review(), () => promise(wire.outcome), '22222222-2222-4222-8222-222222222222');
  assert.equal(target.requestSourceForm(), null);
});

test('source identity accepts only exact normal/CHARS forms and changes no production wire', async () => {
  const target = session();
  for (const form of ['CHARS', '', undefined, 'frmSIVIsite', {}]) {
    assert.throws(() => target.reviewSourceForm(form), /Exact normal/);
  }
  target.reviewSourceForm('FS882-8x6XL');
  let sent;
  await target.transfer(review(), request => { sent = plain(request); return promise(wire.outcome); }, wire.request.requestId);
  assert.deepEqual(sent, wire.request);
  assert.equal(target.requestSourceForm(), 'FS882-8x6XL');
});

test('lost/unknown response blocks replay and acknowledgement until exact readonly recovery', async () => {
  const target = session(); await unknown(target);
  assert.throws(() => target.acknowledge(), /Recover/);
  await assert.rejects(() => target.transfer(review(), () => promise(wire.outcome)), /prior/);
  await target.recover(() => promise(wire.recovery));
  assert.deepEqual(plain(target.view().outcome), wire.outcome); assert.equal(target.view().blocked, true);
  target.acknowledge(); assert.equal(target.view().blocked, false);
});

test('running/foreign/malformed recovery and uncertain outcome never erase retained ownership or allow acknowledgement', async () => {
  const target = session(); await unknown(target);
  for (const value of [
    { ...wire.recovery, status: 'running', outcome: null },
    { ...wire.recovery, requestId: 'foreign' }, { ...wire.recovery, contextId: 'foreign' },
    { ...wire.recovery, outcome: { status: 'unknown', receipt: null, errorMessage: 'uncertain commit' } },
    { ...wire.recovery, outcome: null },
  ]) {
    await target.recover(() => promise(value));
    assert.equal(target.view().outcome, null); assert.equal(target.view().blocked, true);
    assert.deepEqual(plain(target.view().request), wire.request);
    assert.throws(() => target.acknowledge(), /Recover/);
  }
});

test('cancelled actual request wait rejects late response and still requires readonly recovery', async () => {
  const target = session(); let release, cancelled = 0;
  const held = Object.assign(new Promise(resolve => { release = resolve; }), { cancel() { cancelled++; } });
  const operation = target.transfer(review(), () => held, wire.request.requestId);
  assert.equal(target.view().busy, true); target.cancel();
  assert.equal(cancelled, 1); assert.equal(target.view().busy, false); assert.equal(target.view().blocked, true);
  release(wire.outcome); await operation;
  assert.equal(target.view().outcome, null);
  await target.recover(() => promise(wire.recovery));
  assert.equal(target.view().outcome.receipt.historyId, '1');
});

test('retained session lookup preserves exact context paths and aggregated root barriers', async () => {
  const target = external.externalSUEnvironmentSession(scope), events = [];
  const unsubscribe = external.subscribeExternalSUEnvironmentActivity(value => events.push(plain(value)));
  await unknown(target);
  assert.equal(external.externalSUEnvironmentSession({ ...scope }), target);
  assert.throws(() => external.externalSUEnvironmentSession({ ...scope, suPath: scope.suPath + 'x' }), /different/);
  assert.equal(events.at(-1).blocked, true);
  await target.recover(() => promise(wire.recovery)); target.acknowledge();
  assert.equal(events.at(-1).blocked, false); unsubscribe();
});

test('component review cancellation and remount generation reject late results without replay', async () => {
  let release; const held = new Promise(resolve => { release = resolve; });
  const host = componentFunctions('ExternalSUEnvironmentTransfer.svelte', ['reload', 'cancelReview', 'cancel', 'undo'], {
    scope, session: session(), reading: false, generation: 0, error: '', review: null,
    reads: { track: value => value, cancelAll() {} },
    validateExternalSUEnvironmentReview: external.validateExternalSUEnvironmentReview,
    ExternalSUEnvironmentService: { GetReview: () => held },
  });
  const operation = host.actions.reload();
  assert.equal(host.reading, true); host.actions.cancel();
  release(review()); await operation;
  assert.equal(host.review, null); assert.match(host.error, /cancelled/);
  host.actions.undo(); assert.equal(host.error, '');
});

test('card-independent retained receipt and readonly recovery controls survive server remount', async () => {
  const target = external.externalSUEnvironmentSession(scope);
  await unknown(target);
  const source = readFileSync(path.join(__dirname, 'ExternalSUEnvironmentTransfer.svelte'), 'utf8');
  const component = serverComponent(source, 'ExternalSUEnvironmentTransfer.svelte', {
    './externalSUEnvironment': external,
    '../bindings/github.com/boostao/vpro-wails': { ExternalSUEnvironmentService: {} },
  });
  let body = render(component, { props: { ...scope, onclosed() {} } }).body;
  assert.match(body, /Recover transfer outcome readonly/); assert.match(body, new RegExp(wire.request.requestId));
  await target.recover(() => promise(wire.recovery)); target.acknowledge();
  body = render(component, { props: { ...scope, onclosed() {} } }).body;
  assert.match(body, /atomic history 1/); assert.match(body, /3 cells across 1 plots/);
});

test('normal/CHARS integration keeps independent gates, shared parent lifecycle and one source action path', () => {
  for (const name of ['ExternalSUEnvironmentTransfer.svelte', 'FS882Form.svelte', 'App.svelte']) {
    assert.equal(compile(readFileSync(path.join(__dirname, name), 'utf8'), { filename: name, generate: 'client' }).warnings.length, 0);
  }
  const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(root, /VITE_EXTERNAL_SU_ENVIRONMENT_EDITING === 'true'/);
  assert.match(root, /environmentSUDirection = 'external-reverse'; environmentSUOpen = true/);
  assert.match(root, /ExternalSUEnvironmentTransfer bind:this=\{environmentSUEditor\}/);
  assert.match(root, /externalSUEnvironmentBusy \|\| externalSUEnvironmentBlocked/);
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  assert.match(app, /subscribeExternalSUEnvironmentActivity/);
  assert.match(app, /twoPageVegetationBusy \|\| externalSUEnvironmentBusy/);
});

test('actual root background reads stay held during a retained external transfer', async () => {
  const host = componentFunctions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy'], {
    projectReportBusy: false, twoPageVegetationBusy: false, twoPageSoilBusy: false,
    pictureCreationBusy: false, pictureLifecycleBusy: false, vegetationPublicationBusy: false,
    lifeformPublicationBusy: false, summaryPublicationBusy: false, summaryPreferenceBusy: false,
    externalSUEnvironmentBusy: true, view: 'fs882', editorBusy: true,
  });
  await host.actions.loadPlots(0); await host.actions.refresh(); await host.actions.loadHierarchy();
  assert.equal(host.busy, undefined);
});
