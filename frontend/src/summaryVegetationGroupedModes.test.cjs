const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { render } = require('svelte/server');
const { serverComponent, loadTypeScript } = require('./svelteTestHelpers.cjs');
const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': loadTypeScript('lifeformSummary.ts') }, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationGroupedModesWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire[0][key]]));
const plain = value => JSON.parse(JSON.stringify(value));

test('fifty actual Go additional grouped wires retain source text Order and exact DAO metadata', () => {
  assert.equal(wire.length, 50);
  for (const value of wire) {
    assert.deepEqual(plain(report.validateSummaryVegetation(value, owner, value.average, value.display, value.showClass ?? 0, value.groupAndOrder)), value);
    assert.deepEqual(value.orderField, { name: 'Order', daoType: 10, size: 0 });
    for (const row of value.rows) assert.ok(['text', 'null'].includes(row.order.storage));
    if (value.display === 4 || value.display === 5) {
      const peer = wire.find(v => v.groupAndOrder === value.groupAndOrder && v.display === value.display &&
        v.showClass === value.showClass && v.average !== value.average);
      assert.ok(peer);
      assert.deepEqual(value.rows, peer.rows);
    }
    if (value.display === 2) assert.equal(value.groupAndOrder, 14);
  }
});

test('new grouped modes reject numeric Order, foreign group, invalid class Show and source-failing mode', () => {
  const chosen = wire.find(v => v.groupAndOrder === 13 && v.display === 4 && v.showClass === 20 && v.average === 1);
  for (const change of [
    v => { v.orderField.daoType = 7; v.orderField.size = 8; },
    v => { v.groupAndOrder = 14; }, v => { delete v.showClass; }, v => { v.showClass = 2; },
    v => { v.rows[0].order = { storage: 'real', real: 2.5, text: null, integer: null, blobHex: null }; },
    v => { v.rows.at(-1).covers[0].text = 'NaN'; },
  ]) {
    const value = structuredClone(chosen); change(value);
    assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 4, 20, 13));
  }
  for (const group of [12, 13]) assert.match(report.summaryVegetationOptionError(1, 2, 1, group), /original criteria type mismatch/);
});

test('retained grouped display controller routes explicit RPC, snapshots proxies and preserves live correction', async () => {
  const session = report.summaryVegetationSession(owner), calls = [];
  const chosen = wire.find(v => v.groupAndOrder === 13 && v.display === 4 && v.showClass === 20 && v.average === 1);
  session.edit(1, 4, 20, 13);
  const source = proxy(structuredClone(chosen));
  assert.throws(() => structuredClone(source));
  await session.show({ preview() { throw new Error('Wrong legacy API'); },
    previewGroupedDisplay: async (...args) => { calls.push(args); return source; }, cancel() {} });
  assert.deepEqual(calls, [[1, 13, 4, 20]]);
  source.rows.at(-1).covers[0].text = 'changed';
  assert.equal(session.view().preview.rows.at(-1).covers[0].text, '86.60254');
  assert.equal(report.summaryVegetationSession(owner).view().groupAndOrder, 13);
  session.edit(1, 2, 20, 13);
  assert.match(session.view().draftError, /criteria type mismatch/);
  await session.show({ preview() {}, cancel() {} });
  assert.equal(session.view().preview, null);
  session.edit(1, 2, 20, 14);
  const none = wire.find(v => v.groupAndOrder === 14 && v.display === 2 && v.average === 1);
  await session.show({ preview() {}, previewGroupedDisplay: async (...args) => { calls.push(args); return structuredClone(none); }, cancel() {} });
  assert.deepEqual(calls.at(-1), [1, 14, 2, 0]);
  assert.equal(session.view().preview.groupAndOrder, 14);
  assert.equal(session.view().error, '');
});

test('held grouped class validation cannot assign after Cancel and newer None display2 result', async () => {
  const session = new report.SummaryVegetationSession(owner);
  const chosen = wire.find(v => v.groupAndOrder === 12 && v.display === 5 && v.showClass === 20 && v.average === 1);
  session.edit(1, 5, 20, 12);
  let release, reached;
  const hold = new Promise(resolve => { release = resolve; }), entered = new Promise(resolve => { reached = resolve; });
  const pending = session.show({ preview() {}, previewGroupedDisplay: async () => structuredClone(chosen), cancel() {},
    validationYield: () => { reached(); return hold; } });
  await entered; session.cancel({ cancel() {} }); session.edit(2, 2, 20, 14);
  const none = wire.find(v => v.groupAndOrder === 14 && v.display === 2 && v.average === 2);
  await session.show({ preview() {}, previewGroupedDisplay: async () => structuredClone(none), cancel() {} });
  release(); await pending;
  assert.equal(session.view().preview.groupAndOrder, 14);
  assert.equal(session.view().preview.display, 2);
  await session.show({ preview() {}, cancel() {} });
  assert.match(session.view().error, /grouped transport is unavailable/);
});

test('compiled class and None grouped panels retain source controls without enabling CDbl-composite branches', async () => {
  const component = readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
  const panel = serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': report, '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: {} },
  }, { VITE_SUMMARY_VEGETATION: 'true' });
  const session = report.summaryVegetationSession(owner);
  for (const group of [12, 13, 14]) {
    const chosen = wire.find(v => v.groupAndOrder === group && v.display === 4 && v.showClass === 20 && v.average === 1);
    session.edit(1, 4, 20, group);
    await session.show({ preview() {}, previewGroupedDisplay: async () => structuredClone(chosen), cancel() {} });
    const body = render(panel, { props: { ...owner, onBusyChange() {} } }).body;
    assert.match(body, /86\.60254/);
    assert.match(body, /Show: Value/);
    assert.equal(/\bdisabled\b/.test(body.match(/<option value="2"[^>]*>/)[0]), group !== 14);
    assert.doesNotMatch(body.match(/<option value="4"[^>]*>/)[0], /\bdisabled\b/);
    assert.match(body, /original grouped query has a criteria type mismatch/);
  }
});
