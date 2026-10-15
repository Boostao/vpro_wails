const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const report = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform }, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire[0][key]]));
const plain = value => JSON.parse(JSON.stringify(value));
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

test('all eight strict-Go production fixture modes validate exact wire and NULL blanks', () => {
  assert.equal(wire.length, 8);
  for (const value of wire) {
    assert.deepEqual(plain(report.validateSummaryVegetation(value, owner, value.average, value.display)), value);
  }
  assert.equal(wire[0].rows[0].covers[0].text, '0.75 - 10.00');
  assert.equal(wire[1].rows[0].covers[0].text, '0.75 - 13.33');
  assert.equal(wire[2].rows[0].covers[0].text, 'IV - 4');
  assert.equal(wire[3].rows[0].covers[0].text, 'IV - 5');
  assert.equal(wire[6].rows[0].order.text, '5.00');
  assert.equal(wire[6].rows[1].covers[1].storage, 'null');
});

test('reject mismatched owner/options, malformed wire and source provenance without repair', () => {
  const changes = [
    value => { value.contextId = 'another'; },
    value => { value.projectPath += '#changed'; },
    value => { value.suPath = ''; },
    value => { value.average = 2; },
    value => { value.source = 'all-project'; },
    value => { value.extra = true; },
    value => { value.units = null; },
    value => { value.columns.push('Orphan'); },
    value => { value.columns.reverse(); },
    value => { value.units[0].numPlots = 3; },
    value => { value.units[0].membershipIds.pop(); },
    value => { value.units[0].rawVegRowIds = []; },
    value => { value.memberships.push(value.memberships[0]); },
    value => { value.memberships[0].plotNumber.real = 1; },
    value => { value.rows[0].covers[0].text = '10'; },
    value => { value.rows[0].order.text = '10.00'; },
    value => { value.rows.reverse(); },
    value => { value.rows.push(value.rows[0]); },
    value => { value.rows[0].witnesses[0].cover.real = 0.1; },
    value => { value.rows[0].witnesses[0].rawVegRowIds = []; },
    value => { value.rows[0].witnesses[0].rawVegRowIds = ['999']; },
    value => { value.rows[0].witnesses[0].conversionLayerId = ''; },
    value => { value.rows[0].witnesses[0].membershipId = '999'; },
    value => { value.rows[0].witnesses[0].unit = 'B'; },
    value => { value.rows[0].witnesses.push(value.rows[0].witnesses[0]); },
    value => { value.rows[0].witnesses[0].referenceOrigins[0].table = 'VUserWrong'; },
    value => { value.rows[0].witnesses[0].referenceOrigins.push(value.rows[0].witnesses[0].referenceOrigins[0]); },
    value => { value.rows[0].species.text = '\ud800'; },
  ];
  for (const change of changes) {
    const value = structuredClone(wire[6]); change(value);
    assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 6), /incomplete|inconsistent|context/);
  }
  for (const wrong of [{ ...owner, contextId: '' }, { ...owner, su: 'None' }, { ...owner, su: 'USysSuTableDynamic' }]) {
    assert.throws(() => report.validateSummaryVegetation(wire[6], wrong, 1, 6));
  }
  for (const [average, display] of [[0, 6], [1, 4], [1, 5], [1, '6']]) {
    assert.notEqual(report.summaryVegetationOptionError(average, display), '');
  }
});

test('actual Svelte deep proxy wire is detached without structuredClone proxy failure', () => {
  const source = proxy(structuredClone(wire[6]));
  assert.throws(() => structuredClone(source));
  const result = report.validateSummaryVegetation(source, owner, 1, 6);
  source.rows[0].covers[0].text = 'changed';
  source.rows[0].witnesses[0].cover.real = 99;
  source.rows[0].witnesses[0].referenceOrigins[0].table = 'changed';
  source.units[0].rawVegRowIds[0] = 'changed';
  assert.equal(result.rows[0].covers[0].text, '10.00');
  assert.equal(result.rows[0].witnesses[0].cover.real, 10);
  assert.equal(result.rows[0].witnesses[0].referenceOrigins[0].table, 'USysAllSpecs');
  assert.notEqual(result.units[0].rawVegRowIds[0], 'changed');
});

test('literal Unicode ordering follows code points rather than JavaScript UTF16 order', () => {
  const value = structuredClone(wire[6]);
  const names = ['\uE000', '\u{10000}'];
  const prior = value.units.map(unit => unit.code);
  value.units.forEach((unit, i) => { unit.code = names[i]; });
  value.columns = names;
  value.memberships.forEach(member => { const i = prior.indexOf(member.siteUnit.text); if (i >= 0) member.siteUnit.text = names[i]; });
  value.rows.forEach(row => row.witnesses.forEach(witness => { witness.unit = names[prior.indexOf(witness.unit)]; }));
  assert.doesNotThrow(() => report.validateSummaryVegetation(value, owner, 1, 6));
  value.columns.reverse();
  assert.throws(() => report.validateSummaryVegetation(value, owner, 1, 6));
});

test('retained session preserves per-context drafts, read errors and corrections through remount', async () => {
  const session = report.summaryVegetationSession(owner), calls = [];
  session.edit(2, 6);
  const port = { preview: async (average, display) => { calls.push([average, display]); return structuredClone(wire[7]); }, cancel() {} };
  await session.show(port);
  assert.deepEqual(calls, [[2, 6]]);
  assert.equal(session.view().preview.rows[0].covers[0].text, '13.33');
  assert.equal(report.summaryVegetationSession({ ...owner }), session);
  const other = report.summaryVegetationSession({ ...owner, contextId: 'another' });
  assert.equal(other.view().average, 1);
  assert.equal(other.view().preview, null);
  await session.show({ ...port, preview: async () => { throw new Error('physical read failed'); } });
  session.cancel(port);
  assert.match(report.summaryVegetationSession(owner).view().error, /physical read failed/);
  session.edit(1, 4);
  assert.notEqual(session.view().draftError, '');
  await session.show(port);
  assert.equal(calls.length, 1);
  session.edit(1, 6);
  assert.equal(session.view().draftError, '');
  assert.equal(session.view().error, '');
});

test('held validation yield and stale read completion cannot populate a remounted panel', async () => {
  const session = new report.SummaryVegetationSession(owner);
  session.edit(1, 6);
  const read = deferred(), hold = deferred(), reached = deferred();
  let cancelled = 0;
  const port = { preview: () => read.promise, cancel: () => { cancelled++; }, validationYield: () => { reached.resolve(); return hold.promise; } };
  const pending = session.show(port);
  read.resolve(proxy(structuredClone(wire[6])));
  await reached.promise;
  session.cancel(port);
  assert.equal(cancelled, 1);
  assert.equal(session.view().busy, false);
  session.edit(2, 6);
  await session.show({ preview: async () => structuredClone(wire[7]), cancel() {} });
  hold.resolve();
  await pending;
  assert.equal(session.view().preview.average, 2);
  assert.equal(session.view().preview.rows[0].covers[0].text, '13.33');
  assert.equal(session.view().error, '');
});

test('cancelled stale rejection never clears a newer error or draft', async () => {
  const session = new report.SummaryVegetationSession(owner), read = deferred();
  const port = { preview: () => read.promise, cancel() {} };
  const pending = session.show(port);
  session.cancel(port);
  session.edit(1, 6);
  await session.show({ ...port, preview: async () => { throw new Error('new read failure'); } });
  read.reject(new Error('stale failure'));
  await pending;
  assert.match(session.view().error, /new read failure/);
  assert.doesNotMatch(session.view().error, /stale failure/);
});

test('reviewed session result cannot be modified by a consumer sharing its retained DTO', async () => {
  const session = new report.SummaryVegetationSession(owner);
  session.edit(1, 6);
  await session.show({ preview: async () => structuredClone(wire[6]), cancel() {} });
  const result = session.view().preview;
  assert.ok(Object.isFrozen(result));
  assert.ok(Object.isFrozen(result.rows[0].covers[0]));
  assert.ok(Object.isFrozen(result.rows[0].witnesses[0].referenceOrigins));
  assert.equal(Reflect.set(result.rows[0].covers[0], 'text', 'changed'), false);
  assert.throws(() => result.columns.push('changed'));
  assert.equal(session.view().preview.rows[0].covers[0].text, '10.00');
});

test('large validation yields after actual partial work and cancellation stops the next batch', async () => {
  const session = new report.SummaryVegetationSession(owner);
  session.edit(1, 6);
  const value = structuredClone(wire[6]);
  for (let i = 0; i < 900; i++) {
    value.memberships.push({ ...value.memberships[0], rowId: String(1000 + i), siteUnit: { ...value.memberships[0].siteUnit, text: 'not admitted' } });
  }
  const hold = deferred(), reached = deferred();
  let yields = 0;
  const port = { preview: async () => value, cancel() {},
    validationYield: () => { if (++yields === 2) { reached.resolve(); return hold.promise; } return Promise.resolve(); } };
  const pending = session.show(port);
  await reached.promise;
  assert.equal(session.view().busy, true);
  session.cancel(port);
  hold.resolve();
  await pending;
  assert.equal(yields, 2);
  assert.equal(session.view().preview, null);
  assert.equal(session.view().error, '');
});
