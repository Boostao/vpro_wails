const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createHash, webcrypto } = require('node:crypto');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const scope = { contextId: 'owned-export', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', su: 'None', suPath: '' };
const xml = '<?xml version="1.0" encoding="UTF-8"?>\n<kml xmlns="http://earth.google.com/kml/2.1"><Document><name> Title </name></Document></kml>';
const sha = createHash('sha256').update(xml).digest('hex');
const destination = 'C:\\fixture\\ Literal.kml';
const exportsModule = loadTypeScript('googleEarthKMLExport.ts', {
  './qualityEditor': quality, './googleEarthReview': earth,
  './googleEarthKMLReview': { async validateGoogleEarthKMLReview(value, owner, field, title) {
    assert.equal(value.contextId, owner.contextId); assert.equal(value.descriptionField, field); assert.equal(value.title, title);
    return structuredClone(value);
  } },
}, { crypto: webcrypto, TextEncoder, Uint8Array });
function review(owner = scope) {
  return { review: { ...owner, descriptionField: 'Zone', title: ' Title ', placemarkCount: 0,
    byteCount: Buffer.byteLength(xml), kml: xml }, approvalHash: 'a'.repeat(64), kmlSHA256: sha };
}
function outcome(status = 'published') {
  return { status, requestedDestination: destination, path: destination, sha256: sha,
    errorMessage: status === 'published' ? '' : 'explicit publication error' };
}

test('KML export review verifies exact displayed UTF8 SHA256 without losing source approval', async () => {
  const input = review();
  const result = await exportsModule.validateGoogleEarthKMLExportReview(input, scope, 'Zone', ' Title ');
  input.approvalHash = 'changed'; input.review.kml = 'changed';
  assert.equal(result.approvalHash, 'a'.repeat(64)); assert.equal(result.review.kml, xml);
  for (const mutate of [v => v.approvalHash = '', v => v.approvalHash = 'A'.repeat(64),
    v => v.kmlSHA256 = '0'.repeat(64), v => v.kmlSHA256 = null]) {
    const value = review(); mutate(value);
    await assert.rejects(exportsModule.validateGoogleEarthKMLExportReview(value, scope, 'Zone', ' Title '));
  }
});

test('KML outcomes reject contradictory committed status bytes destination and hidden errors', () => {
  for (const status of ['published', 'published-with-errors', 'not-published']) {
    assert.equal(exportsModule.validateGoogleEarthKMLExportOutcome(outcome(status), destination, sha).status, status);
  }
  for (const mutate of [v => v.status = 'success', v => v.requestedDestination = destination.replace('\\ Literal', '\\Literal'),
    v => v.requestedDestination += 'other', v => v.path = '\ud800', v => v.path = '',
    v => v.sha256 = '0'.repeat(64), v => v.sha256 = 7, v => v.errorMessage = 'hidden warning']) {
    const value = outcome(); mutate(value);
    assert.throws(() => exportsModule.validateGoogleEarthKMLExportOutcome(value, destination, sha));
  }
  const failed = outcome('not-published'); failed.errorMessage = '';
  assert.throws(() => exportsModule.validateGoogleEarthKMLExportOutcome(failed, destination, sha));
});

test('KML publication is noncancellable and holds acknowledgement barrier across a held RPC', async () => {
  const session = new exportsModule.GoogleEarthKMLPublicationSession(scope);
  const input = review();
  let resolve, calls = 0, cancels = 0, captured;
  const pending = session.publish(input, destination, request => {
    calls++; captured = request;
    return { then(onFulfilled, onRejected) { return new Promise(done => { resolve = done; }).then(onFulfilled, onRejected); },
      cancel() { cancels++; } };
  });
  input.review.title = 'caller mutation'; input.approvalHash = 'caller';
  await Promise.resolve();
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.equal(captured.title, ' Title '); assert.equal(captured.approvalHash, 'a'.repeat(64));
  assert.throws(() => session.acknowledge());
  await assert.rejects(session.publish(review(), destination, () => { calls++; }));
  resolve(outcome()); await pending;
  assert.equal(calls, 1); assert.equal(cancels, 0);
  assert.equal(session.view().busy, false); assert.equal(session.view().blocked, true);
  assert.equal(session.view().outcome.status, 'published');
  session.acknowledge();
  assert.equal(session.view().blocked, false); assert.equal(session.view().outcome.sha256, sha);
});

test('KML committed errors and explicit rejection remain visible until deliberate acknowledgement', async () => {
  for (const status of ['published-with-errors', 'not-published']) {
    const session = new exportsModule.GoogleEarthKMLPublicationSession(scope);
    await session.publish(review(), destination, async () => outcome(status));
    const view = session.view();
    assert.equal(view.outcome.status, status); assert.equal(view.error, 'explicit publication error');
    assert.equal(view.blocked, true); session.acknowledge();
    assert.equal(session.view().error, view.error);
  }
});

test('KML transport and malformed acknowledgement failures are unknown never no-file fallbacks', async () => {
  for (const port of [async () => { throw new Error('IPC disconnected'); }, async () => null,
    async () => ({ ...outcome(), sha256: '0'.repeat(64) })]) {
    const session = new exportsModule.GoogleEarthKMLPublicationSession(scope);
    await session.publish(review(), destination, port);
    assert.equal(session.view().outcome, null); assert.equal(session.view().blocked, true);
    assert.match(session.view().error, /outcome unknown/); assert.match(session.view().error, /Do not repeat/);
    await assert.rejects(session.publish(review(), destination, port));
  }
});

test('KML publication receipt and busy barrier survive remount and reject ownership aliases', async () => {
  const owner = { ...scope, contextId: 'remount-export' };
  const first = exportsModule.googleEarthKMLPublicationSession(owner);
  let resolve, firstUpdates = 0, remountedUpdates = 0;
  const unsubscribe = first.subscribe(() => firstUpdates++);
  const pending = first.publish(review(owner), destination, () => new Promise(done => { resolve = done; }));
  unsubscribe();
  const remount = exportsModule.googleEarthKMLPublicationSession(owner);
  const stop = remount.subscribe(() => remountedUpdates++);
  assert.equal(remount.view().busy, true);
  resolve(outcome('published-with-errors')); await pending;
  assert.equal(firstUpdates, 2); assert.equal(remountedUpdates, 2);
  assert.equal(remount.view().blocked, true); assert.equal(remount.view().outcome.status, 'published-with-errors');
  assert.throws(() => exportsModule.googleEarthKMLPublicationSession({ ...owner, projectPath: 'foreign' }));
  remount.acknowledge(); stop();
});

test('KML export panel shares single labels uses independent gate and keeps receipt above routine guidance', () => {
  const source = readFileSync(path.join(__dirname, 'GoogleEarthReview.svelte'), 'utf8');
  assert.ok(compile(source, { filename: 'GoogleEarthReview.svelte', generate: 'client' }).js.code);
  assert.match(source, /VITE_GOOGLE_EARTH_KML_EXPORT === 'true'/);
  assert.equal((source.match(/id="google-earth-place-name"/g) || []).length, 1);
  assert.match(source, /<label for="google-earth-kml-destination">New output file:/);
  assert.match(source, /data-google-earth-kml-acknowledge/);
  assert.match(source, /onDestroy\(unsubscribe\)/);
  assert.doesNotMatch(source, /reads\.track\(GoogleEarthKMLExportService\.ExportReviewed/);
  assert.ok(source.indexOf('data-google-earth-kml-outcome') < source.indexOf('class="notice"'));
  assert.doesNotMatch(source, /\{@html/);
});
