const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { render } = require('svelte/server');
const { serverComponent, loadTypeScript } = require('./svelteTestHelpers.cjs');
const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': loadTypeScript('lifeformSummary.ts') }, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationGroupedWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire[0][key]]));
const plain = value => JSON.parse(JSON.stringify(value));

test('six actual Go grouped wires preserve DAO metadata, numeric Order and detached provenance', () => {
  assert.equal(wire.length, 6);
  for (const value of wire) {
    assert.deepEqual(plain(report.validateSummaryVegetation(value, owner, value.average, 6, 0, value.groupAndOrder)), value);
    assert.deepEqual(value.orderField, { name: 'Order', daoType: 7, size: 8 });
    assert.equal(value.rows[0].order.storage, 'real');
    const alpha = value.rows.find(row => row.species.text === 'Alpha');
    assert.ok(alpha);
    assert.equal(alpha.order.real, value.average === 1 ? 10 : 13.33);
    assert.equal(alpha.covers[0].text, value.average === 1 ? '10.00' : '13.33');
    for (const row of value.rows) for (const witness of row.witnesses) {
      assert.equal(witness.conversionLayerId, '');
      assert.ok(witness.rawVegRowIds.length);
    }
    if (value.groupAndOrder === 14) assert.ok(value.rows.every(row => row.layer.storage === 'null'));
  }
});

test('strict grouped validation refuses wrong source, metadata, numeric Order, grouping and conversion provenance', () => {
  for (const change of [
    v => { delete v.groupField; }, v => { delete v.groupAndOrder; }, v => { v.groupAndOrder = 13; },
    v => { v.groupField.daoType = 9; }, v => { v.groupField.size = 3; }, v => { v.groupField.extra = true; },
    v => { v.orderField.daoType = 10; }, v => { v.source = 'literal-normal-su-layer'; },
    v => { v.ordering = 'literal-null-first-layer-species'; }, v => { v.showClass = 20; },
    v => { v.rows[0].order.real = 5; }, v => { v.rows[0].order.real = Infinity; },
    v => { v.rows[0].order = { storage: 'text', text: '5.00', real: null, integer: null, blobHex: null }; },
    v => { v.rows[0].witnesses[0].conversionLayerId = '1'; },
    v => { v.rows[0].layer.text = 'AA'; },
  ]) {
    const value = structuredClone(wire[0]); change(value);
    assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 6, 0, 12));
  }
  const none = structuredClone(wire[4]);
  none.rows[0].layer = { storage: 'text', text: '', real: null, integer: null, blobHex: null };
  assert.throws(() => report.validateSummaryVegetation(none, owner, 1, 6, 0, 14));
  for (const group of [0, 10, 15, '12', null]) assert.notEqual(report.summaryVegetationOptionError(1, 6, 1, group), '');
  for (const group of [12, 13, 14]) {
    assert.equal(report.summaryVegetationOptionError(1, 6, 20, group), '');
    for (const display of [1, 3, 4, 5]) assert.equal(report.summaryVegetationOptionError(1, display, 20, group), '');
    assert.equal(report.summaryVegetationOptionError(1, 2, 20, group) === '', group === 14);
  }
});

test('group draft remount, actual reactive proxy detachment, explicit RPC and live correction', async () => {
  const session = report.summaryVegetationSession(owner), calls = [];
  session.edit(1, 6, 20, 12);
  const source = proxy(structuredClone(wire[0]));
  assert.throws(() => structuredClone(source));
  await session.show({ preview() { throw new Error('Must use grouped RPC'); },
    previewGrouped: async (...args) => { calls.push(args); return source; }, cancel() {} });
  assert.deepEqual(calls, [[1, 12]]);
  source.rows[0].order.real = 999;
  assert.equal(session.view().preview.rows[0].order.real, 10);
  assert.equal(report.summaryVegetationSession(owner).view().groupAndOrder, 12);
  assert.equal(session.view().showClass, 20);
  session.edit(1, 6, 20, 15);
  assert.notEqual(session.view().draftError, '');
  await session.show({ preview() {}, cancel() {} });
  assert.match(session.view().error, /Group by/);
  session.edit(2, 6, 20, 14);
  await session.show({ preview() {}, previewGrouped: async () => structuredClone(wire[5]), cancel() {} });
  assert.equal(session.view().preview.groupAndOrder, 14);
  assert.equal(session.view().draftError, '');
  assert.equal(session.view().error, '');
});

test('grouped yielded validation Cancel/remount/retry rejects stale response and foreign ownership', async () => {
  const session = new report.SummaryVegetationSession(owner);
  session.edit(1, 6, 1, 12);
  let release, reached;
  const hold = new Promise(resolve => { release = resolve; }), entered = new Promise(resolve => { reached = resolve; });
  const pending = session.show({ preview() {}, previewGrouped: async () => structuredClone(wire[0]), cancel() {},
    validationYield: () => { reached(); return hold; } });
  await entered;
  session.cancel({ cancel() {} });
  session.edit(1, 6, 1, 13);
  await session.show({ preview() {}, previewGrouped: async () => structuredClone(wire[2]), cancel() {} });
  release(); await pending;
  assert.equal(session.view().preview.groupAndOrder, 13);
  await session.show({ preview() {}, previewGrouped: async () => ({ ...wire[2], contextId: 'foreign' }), cancel() {} });
  assert.equal(session.view().preview, null);
  assert.match(session.view().error, /another context/);
  await session.show({ preview() {}, cancel() {} });
  assert.match(session.view().error, /grouped transport is unavailable/);
});

test('compiled grouped panel uses source captions, numeric Order and explicitly unavailable source failure branch', async () => {
  const component = readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
  const panel = serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': report, '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: {} },
  }, { VITE_SUMMARY_VEGETATION: 'true' });
  const session = report.summaryVegetationSession(owner);
  for (const value of wire.filter(value => value.average === 1)) {
    session.edit(1, 6, 1, value.groupAndOrder);
    await session.show({ preview() {}, previewGrouped: async () => structuredClone(value), cancel() {} });
    const body = render(panel, { props: { ...owner, onBusyChange() {} } }).body;
    assert.match(body, /<td[^>]*>10<\/td>/);
    assert.ok(body.includes(`Current Site Unit Table — ${value.groupAndOrder === 12 ? 'Strata' : value.groupAndOrder === 13 ? 'Lifeform' : 'None'}`));
    assert.ok(body.includes(value.groupField.name));
    assert.doesNotMatch(body.match(/<select[^>]*data-summary-group[^>]*>/)[0], /\bdisabled\b/);
    assert.doesNotMatch(body.match(/<option value="4"[^>]*>/)[0], /\bdisabled\b/);
    assert.equal(/\bdisabled\b/.test(body.match(/<option value="2"[^>]*>/)[0]), value.groupAndOrder !== 14);
  }
  const disabled = serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': report, '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: {} },
  }, { VITE_SUMMARY_VEGETATION: 'false' });
  const body = render(disabled, { props: { ...owner, onBusyChange() {} } }).body;
  assert.match(body.match(/<select[^>]*data-summary-group[^>]*>/)[0], /\bdisabled\b/);
});
