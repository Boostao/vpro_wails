const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { render } = require('svelte/server');
const { serverComponent, loadTypeScript } = require('./svelteTestHelpers.cjs');
const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': loadTypeScript('lifeformSummary.ts') }, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationClassWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire[0][key]]));
const plain = value => JSON.parse(JSON.stringify(value));

test('all twelve class production wires retain source classes/numeric text/ignored Calculate/NULL', () => {
  assert.equal(wire.length, 12);
  for (const value of wire) assert.deepEqual(plain(report.validateSummaryVegetation(value, owner, value.average, value.display, value.showClass)), value);
  const expected = { 4: { 1: ['3', '2', '1'], 10: ['3', '2', '1'], 20: ['86.60254', '50', '2.5'] },
    5: { 1: ['4', '4', '1'], 10: ['4', '4', '1'], 20: ['237.1708', '223.6068', '17.67767'] } };
  for (const value of wire) {
    assert.deepEqual([value.rows[0].covers[0].text, value.rows[0].covers[1].text, value.rows[1].covers[0].text], expected[value.display][value.showClass]);
    assert.equal(value.rows[1].covers[1].text, null);
  }
});

test('strict class wire rejects Show drift, invalid class/text and lexical Order without arithmetic repair', () => {
  for (const change of [
    v => { delete v.showClass; }, v => { v.showClass = 10; }, v => { v.extra = true; },
    v => { v.rows[0].covers[0].text = 'NaN'; }, v => { v.rows[0].covers[0].text = '01'; },
    v => { v.rows[0].covers[0].text = '1.0'; }, v => { v.rows[0].covers[0].text = '1e+07'; },
    v => { v.rows[0].covers[0].text = '1E+00'; }, v => { v.rows[0].covers[0].text = '12345678'; },
    v => { v.rows[0].covers[0].text = '1E+100'; }, v => { v.rows[0].order.text = '50'; },
    v => { v.rows[0].order.storage = 'null'; v.rows[0].order.text = null; },
  ]) {
    const value = structuredClone(wire[4]); change(value);
    assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 4, 20));
  }
  const value = structuredClone(wire[0]); value.rows[0].covers[0].text = '0';
  assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 4, 1));
  for (const show of [0, 2, '10', null]) assert.notEqual(report.summaryVegetationOptionError(1, 4, show), '');
  for (const display of [4, 5, 6]) for (const show of [1, 10, 20]) assert.equal(report.summaryVegetationOptionError(1, display, show), '');
});

test('all281 original SINGLE strings pass class wire presentation grammar without recomputing arithmetic', () => {
  const source = JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'testdata', 'summaryVegetationClassSource.json'), 'utf8'));
  assert.equal(source.singleText.length, 281);
  for (const expected of source.singleText) {
    const value = structuredClone(wire[4]);
    value.rows[0].covers[0].text = expected.text;
    value.rows[0].order.text = [expected.text, value.rows[0].covers[1].text].sort().at(-1);
    assert.doesNotThrow(() => report.validateSummaryVegetation(value, owner, 1, 4, 20), `${expected.source}/${expected.id}`);
  }
});

test('class proxy detachment, explicit transport and historical Show survive remount and correction', async () => {
  const session = report.summaryVegetationSession(owner), calls = [];
  session.edit(1, 4, 1);
  const source = proxy(structuredClone(wire[0]));
  assert.throws(() => structuredClone(source));
  await session.show({
    preview() { throw new Error('Class must not call old transport'); },
    previewClass: async (...args) => { calls.push(args); return source; }, cancel() {},
  });
  assert.deepEqual(calls, [[1, 4, 1]]);
  source.rows[0].covers[0].text = 'changed';
  assert.equal(session.view().preview.rows[0].covers[0].text, '3');
  assert.equal(report.summaryVegetationSession(owner).view().showClass, 1);
  session.edit(2, 5, 2);
  assert.notEqual(session.view().draftError, '');
  session.edit(2, 5, 20);
  await session.show({ previewClass: async () => structuredClone(wire[11]), preview() {}, cancel() {} });
  assert.equal(session.view().preview.showClass, 20);
  assert.equal(session.view().preview.average, 2);
  assert.equal(session.view().draftError, '');
});

test('class validation Cancel then newer Show request cannot accept stale numeric DTO', async () => {
  const session = new report.SummaryVegetationSession(owner);
  session.edit(1, 4, 20);
  let release, reached;
  const hold = new Promise(resolve => { release = resolve; });
  const entered = new Promise(resolve => { reached = resolve; });
  const port = { preview() {}, previewClass: async () => structuredClone(wire[4]), cancel() {},
    validationYield: () => { reached(); return hold; } };
  const pending = session.show(port);
  await entered; session.cancel(port); session.edit(1, 5, 10);
  await session.show({ preview() {}, previewClass: async () => structuredClone(wire[8]), cancel() {} });
  release(); await pending;
  assert.equal(session.view().preview.display, 5);
  assert.equal(session.view().preview.showClass, 10);
  assert.equal(session.view().error, '');
  await session.show({ preview() {}, cancel() {} });
  assert.match(session.view().error, /class transport is unavailable/);
});

test('compiled class panel retains Show labels and source-enabled but ignored Show on cover mode', async () => {
  const component = readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
  const panel = serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': report, '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: {} },
  }, { VITE_SUMMARY_VEGETATION: 'true' });
  const session = report.summaryVegetationSession(owner);
  session.edit(1, 4, 20);
  await session.show({ preview() {}, previewClass: async () => structuredClone(wire[4]), cancel() {} });
  let body = render(panel, { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(body, /86\.60254/); assert.match(body, /Show: Value/);
  assert.match(body, /Calculate is retained but ignored/);
  assert.match(body, /Class \(historical default 1\)/);
  assert.doesNotMatch(body.match(/<select[^>]*data-summary-show[^>]*>/)[0], /\bdisabled\b/);
  session.edit(1, 6, 20);
  body = render(panel, { props: { ...owner, onBusyChange() {} } }).body;
  assert.doesNotMatch(body.match(/<select[^>]*data-summary-show[^>]*>/)[0], /\bdisabled\b/);
  session.edit(1, 3, 20);
  body = render(panel, { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(body.match(/<select[^>]*data-summary-show[^>]*>/)[0], /\bdisabled\b/);
});
