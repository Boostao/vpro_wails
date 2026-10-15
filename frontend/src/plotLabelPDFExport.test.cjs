const assert = require('node:assert/strict');
const { test } = require('node:test');
const { webcrypto } = require('node:crypto');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy, state, get } = require('svelte/internal/client');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const { render } = require('svelte/server');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const lookup = loadTypeScript('scopedPlotLookup.ts', { './qualityEditor': quality });
const labels = loadTypeScript('plotLabelPreview.ts', { './qualityEditor': quality, './scopedPlotLookup': lookup });
const regional = loadTypeScript('plotLabelRegional.ts', { './qualityEditor': quality, './plotLabelPreview': labels });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const pdf = loadTypeScript('plotLabelPDFExport.ts', {
  './qualityEditor': quality, './googleEarthReview': earth, './plotLabelRegional': regional,
  './publicationSession': loadTypeScript('publicationSession.ts'),
}, { crypto: webcrypto });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'plotLabelPDFWire.json'), 'utf8'));
const scope = { contextId: wire.review.preview.contextId, project: wire.review.project,
  projectPath: wire.review.projectPath, su: wire.review.su, suPath: wire.review.suPath };
const plain = value => JSON.parse(JSON.stringify(value));
function review() { return structuredClone(wire.review); }
function session() { return new pdf.PlotLabelPDFPublicationSession(scope); }
async function unknown(target) {
  await target.publish(review(), wire.request.destination, () => Promise.reject(new Error('lost response')), wire.request.requestId);
  assert.equal(target.view().blocked, true);
  assert.equal(target.view().outcome, null);
}
function host(operation = () => Promise.resolve(review())) {
  const changes = [], calls = [], target = session();
  const context = componentFunctions('PlotLabelPreview.svelte',
    ['cancel', 'load', 'updateBusy', 'publishPDF', 'recoverPDF', 'acknowledgePDF'], {
      ...scope, plot: 'P01', ownerKey: 'owned-PDF', pdfEnabled: true, pdfSession: target,
      pdfReview: null, preview: null, regionalEvidence: null, labelDate: '', busy: false, generation: 0, error: '',
      destination: wire.request.destination, publication: target.view(),
      validatePlotLabelPDFReview: pdf.validatePlotLabelPDFReview,
      validatePlotLabelRegionalPreview: regional.validatePlotLabelRegionalPreview,
      plotLookupInputError: lookup.plotLookupInputError,
      onBusyChange(value) { changes.push(value); },
      reads: { track(value) { return value; }, cancelAll() { calls.push('cancel'); } },
      PlotLabelPDFExportService: {
        GetReview(...args) { calls.push(['review', ...args]); return operation(...args); },
        ExportReviewed(...args) { calls.push(['export', ...args]); return Promise.resolve(wire.outcome); },
        RecoverOutcome(...args) { calls.push(['recover', ...args]); return Promise.resolve(wire.recovery); },
      },
      ContextService: { GetPlot() { throw new Error('legacy PDF fallback forbidden'); } },
      PlotLabelRegionalService: { GetPreview() { throw new Error('regional PDF fallback forbidden'); } },
    });
  target.subscribe(() => { context.publication = target.view(); context.actions.updateBusy(); });
  return { context, changes, calls, target };
}

test('strict-Go PDF wire retains all literal owned paths, session and approvals without caller aliases', () => {
  const original = review(), result = pdf.validatePlotLabelPDFReview(original, scope, 'P01');
  assert.deepEqual(plain(result), wire.review);
  original.preview.header.projectId = 'changed';
  original.sessionId = 'changed';
  assert.equal(result.preview.header.projectId, 'ID-1');
  assert.equal(result.sessionId, 'TEST_SERVICE_OWNER');
  assert.deepEqual(plain(pdf.validatePlotLabelPDFOutcome(wire.outcome, wire.request.destination, wire.review.pdfSHA256)), wire.outcome);
});

test('PDF reviews refuse foreign owners, sparse fields, wrong layout, malformed approval and date provenance', () => {
  for (const change of [
    v => { v.projectPath += 'x'; }, v => { v.su = 'None'; }, v => { v.suPath = ''; },
    v => { v.preview.contextId = 'foreign'; }, v => { v.preview.header.plotNumber = 'foreign'; },
    v => { v.layout = 'access-printer'; }, v => { v.approvalHash = 'A'.repeat(64); },
    v => { v.pdfSHA256 = ''; }, v => { v.sessionId = ''; }, v => { v.sessionId = '\ud800'; },
    v => { v.sessionId = 'x'.repeat(129); }, v => { v.preview.locale = 'fr-CA'; },
    v => { v.preview.pattern = 'yyyy-MM-dd'; }, v => { v.preview.source = 'browser'; },
    v => { v.extra = true; }, v => { delete v.suPath; }, v => { delete v.preview.header.projectId; },
  ]) {
    const value = review(); change(value);
    assert.throws(() => pdf.validatePlotLabelPDFReview(value, scope, 'P01'));
  }
  assert.throws(() => pdf.validatePlotLabelPDFReview(null, scope, 'P01'));
  assert.throws(() => pdf.validatePlotLabelPDFReview(review(), scope, 'another'));
});

test('PDF acknowledgements reject inconsistent destinations, committed bytes, statuses and hidden errors', () => {
  for (const change of [
    v => { v.requestedDestination += 'x'; }, v => { v.sha256 = '0'.repeat(64); },
    v => { v.path = ''; }, v => { v.path = '\ud800'; }, v => { v.errorMessage = 'hidden error'; },
    v => { v.status = 'success'; }, v => { v.extra = true; }, v => { delete v.sha256; },
    v => { v.status = 'published-with-errors'; }, v => { v.status = 'not-published'; },
  ]) {
    const value = structuredClone(wire.outcome); change(value);
    assert.throws(() => pdf.validatePlotLabelPDFOutcome(value, wire.request.destination, wire.review.pdfSHA256));
  }
  for (const status of ['published-with-errors', 'not-published']) {
    const value = { ...wire.outcome, status, errorMessage: 'explicit failure' };
    assert.equal(pdf.validatePlotLabelPDFOutcome(value, wire.request.destination, wire.review.pdfSHA256).status, status);
  }
});

test('actual Svelte raw signal keeps reviewed DTO cloneable; deep proxy refuses before transport', async () => {
  const signal = state(pdf.validatePlotLabelPDFReview(review(), scope, 'P01'));
  let calls = 0;
  await session().publish(get(signal), wire.request.destination, () => { calls++; return Promise.resolve(wire.outcome); }, wire.request.requestId);
  assert.equal(calls, 1);
  const reactive = proxy(review());
  assert.throws(() => structuredClone(reactive));
  await assert.rejects(session().publish(reactive, wire.request.destination, () => { calls++; }, wire.request.requestId));
  assert.equal(calls, 1);
});

test('held PDF attempt owns exact UUID payload, has no cancellation/replay and retains detached receipt after acknowledgement', async () => {
  const target = session();
  let release, received, calls = 0;
  const pending = target.publish(review(), wire.request.destination, request => {
    received = request; calls++; return new Promise(resolve => { release = resolve; });
  }, wire.request.requestId);
  assert.deepEqual(plain(received), wire.request);
  assert.equal(target.view().busy, true);
  assert.equal(target.view().blocked, true);
  assert.throws(() => target.acknowledge());
  await assert.rejects(target.publish(review(), wire.request.destination, () => { calls++; }));
  await assert.rejects(target.recover(() => Promise.resolve(wire.recovery)));
  release(wire.outcome);
  await pending;
  const returned = target.view(); returned.outcome.sha256 = 'caller';
  assert.equal(target.view().outcome.sha256, wire.review.pdfSHA256);
  target.acknowledge();
  assert.equal(target.view().blocked, false);
  assert.deepEqual(plain(target.view().outcome), wire.outcome);
  assert.equal(calls, 1);
});

test('unknown PDF response requires readonly exact running/completed recovery and retains immutable result', async () => {
  const target = session(); await unknown(target);
  assert.throws(() => target.acknowledge(), /Recover.*unknown/);
  let release, payload, reads = 0;
  const pending = target.recover(request => {
    reads++; payload = request; return new Promise(resolve => { release = resolve; });
  });
  assert.deepEqual(plain(payload), wire.request);
  assert.equal(target.view().busy, true);
  await assert.rejects(target.recover(() => { reads++; }));
  assert.throws(() => target.acknowledge());
  release({ ...wire.recovery, status: 'running', reason: 'Original attempt is still running.', outcome: null });
  await pending;
  assert.equal(target.view().outcome, null);
  assert.match(target.view().error, /still running/);
  await target.recover(() => { reads++; return Promise.resolve(wire.recovery); });
  assert.deepEqual(plain(target.view().outcome), wire.outcome);
  target.acknowledge();
  assert.equal(target.view().blocked, false);
  assert.deepEqual(plain(target.view().outcome), wire.outcome);
  assert.equal(reads, 2);
});

test('readonly recovery never certifies foreign/malformed results or retired service ownership', async () => {
  for (const change of [
    v => { v.requestId = '00000000-0000-4000-8000-000000000002'; },
    v => { v.contextId = 'foreign'; }, v => { v.reason = ''; },
    v => { v.outcome.sha256 = '0'.repeat(64); }, v => { v.status = 'running'; },
    v => { v.extra = true; }, v => { v.outcome = null; },
  ]) {
    const target = session(); await unknown(target);
    const value = structuredClone(wire.recovery); change(value);
    await target.recover(() => Promise.resolve(value));
    assert.equal(target.view().outcome, null);
    assert.equal(target.view().blocked, true);
    assert.match(target.view().error, /unknown.*never replay/);
  }
  const target = session(); await unknown(target);
  await target.recover(() => Promise.reject(new Error('retired service owner; outcome unknown')));
  assert.match(target.view().error, /retired service owner/);
  assert.throws(() => target.acknowledge());
});

test('PDF context registry retains exact receipt across remount but rejects changed paths', async () => {
  const owner = { ...scope, contextId: 'retained-PDF' }, value = review();
  value.preview.contextId = owner.contextId;
  const target = pdf.plotLabelPDFPublicationSession(owner);
  await target.publish(value, wire.request.destination, () => Promise.resolve(wire.outcome));
  assert.equal(pdf.plotLabelPDFPublicationSession({ ...owner }), target);
  assert.throws(() => pdf.plotLabelPDFPublicationSession({ ...owner, projectPath: 'other' }));
  assert.deepEqual(plain(target.view().outcome), wire.outcome);
});

test('PDF component routes owned reviews/export through production-shaped bindings and blocks root until acknowledgement', async () => {
  const { context, changes, calls } = host();
  await context.actions.load();
  assert.deepEqual(calls.filter(Array.isArray), [['review', scope.contextId, '{"plotNumber":"P01"}']]);
  assert.equal(context.preview.plotNumber1, 'P01');
  assert.equal(context.labelDate, 'Monday, October 12, 2026');
  assert.equal(changes.at(-1), false);
  await context.actions.publishPDF();
  assert.equal(changes.at(-1), true);
  const exported = calls.find(call => call[0] === 'export');
  const request = JSON.parse(exported[2]);
  assert.deepEqual({ ...request, requestId: wire.request.requestId }, wire.request);
  context.actions.cancel();
  assert.equal(changes.at(-1), true);
  await context.actions.load();
  assert.equal(calls.filter(call => call[0] === 'review').length, 1);
  context.actions.acknowledgePDF();
  assert.equal(changes.at(-1), false);
  assert.deepEqual(plain(context.publication.outcome), wire.outcome);
});

test('cancel/owner invalidation discards late PDF review, malformed backend review assigns no partial data', async () => {
  for (const action of ['cancel', 'owner']) {
    let release;
    const { context } = host(() => new Promise(resolve => { release = resolve; }));
    const pending = context.actions.load();
    if (action === 'cancel') context.actions.cancel(); else context.ownerKey = 'foreign';
    release(review()); await pending;
    assert.equal(context.preview, null);
    assert.equal(context.pdfReview, null);
    assert.equal(context.regionalEvidence, null);
  }
  const { context } = host(() => Promise.resolve({ ...review(), layout: 'wrong' }));
  await context.actions.load();
  assert.match(context.error, /layout/);
  assert.equal(context.preview, null);
  assert.equal(context.pdfReview, null);
});

test('PDF-only flag renders distinct portable workflow and retains disabled physical print claims', () => {
  const component = serverComponent(readFileSync(path.join(__dirname, 'PlotLabelPreview.svelte'), 'utf8'),
    'PlotLabelPreview.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {},
      './scopedPlotLookup': lookup, './plotLabelPreview': labels,
      './plotLabelRegional': regional, './plotLabelPDFExport': pdf,
    }, { VITE_PLOT_LABEL_PDF_EXPORT: 'true' });
  const body = render(component, { props: { ...scope, onBusyChange() {} } }).body;
  assert.match(body, /Review saved plot label PDF/);
  assert.match(body, /Portable saved label PDF/);
  assert.match(body, /Print All and physical printing remain unavailable/);
});
