const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const fieldList = loadTypeScript('summaryVegetationFieldList.ts', {}, { structuredClone });
const vegetation = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform }, { setTimeout });
const client = loadTypeScript('summaryVegetationEnvironmental.ts', {
  './lifeformSummary': lifeform, './summaryVegetationFieldList': fieldList, './summaryVegetation': vegetation,
  './summaryVegetationEnvironmentalNumeric': loadTypeScript('summaryVegetationEnvironmentalNumeric.ts', { './summaryVegetationFieldList': fieldList }),
}, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.vegetation[key]]));
const plain = value => JSON.parse(JSON.stringify(value));
const yieldNow = () => Promise.resolve();
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

test('strict-Go combined wire retains sixteen categories and adds three derived categories with independent counts', async () => {
  const result = await client.validateSummaryEnvironmental(wire, owner, yieldNow);
  assert.deepEqual(plain(result), wire);
  assert.equal(result.rows.length, 25);
  assert.equal(result.rows[0].field.cells[0].storage, 'null');
  const count = result.rows.find(row => row.field.cells[5].text === 'SVnPlots');
  assert.equal(count.labelColumn, 3);
  assert.deepEqual(plain(count.values.map(value => value.integer)), ['4', '1']);
  assert.equal(result.rows.find(row => row.field.cells[5].text === 'SVSnr').values[0].text, 'M(2)  N(1)  ');
  assert.equal(result.rows.find(row => row.field.cells[5].text === 'SVSmr').values[0].text, 'VX(2)  (Null 1)');
  assert.equal(result.rows.find(row => row.field.cells[5].text === 'SVBioGeoClimaticUnit').values[0].text, 'Q1(2)  (Null 1)');
  assert.equal(result.rows.find(row => row.field.cells[5].text === 'SVAspect').values[0].text, 'N(2)  (Null 1)');
  for (const [functionName, argument, text] of [
    ['SVBedrockType', '1', 'B1'], ['SVBedrockType', '2', 'B2'], ['SVBedrockType', '3', 'B3'],
    ['SVSiteDisturbance', '1', 'D1'], ['SVSiteDisturbance', '2', 'D2'],
  ]) {
    const row = result.rows.find(row => row.field.cells[5].text === functionName && row.field.cells[6].text === argument);
    assert.equal(row.values[0].text, `${text}(2)  (Null 1)`);
  }
  assert.equal(Object.isFrozen(result), true);
  assert.equal(Object.isFrozen(result.rows[0].values[0]), true);
});

test('dynamic fields require exact calibrated TEXT arguments and complete literal witnesses', async () => {
  for (const [functionName, argument] of [
    ['SVBedrockType', null], ['SVBedrockType', '0'], ['SVBedrockType', '4'],
    ['SVSiteDisturbance', '3'], ['SVBedrockType', '01'],
  ]) {
    const source = structuredClone(wire), row = source.rows.find(row => row.field.cells[5].text === functionName);
    const physical = source.fieldList.rows.find(field => field.rowId === row.field.rowId);
    const cell = { storage: argument === null ? 'null' : 'text', text: argument, integer: null, real: null, blobHex: null };
    row.field.cells[6] = physical.cells[6] = cell;
    await assert.rejects(client.validateSummaryEnvironmental(source, owner, yieldNow));
  }
});

test('derived category values remain literal TEXT; extra arguments and uncalibrated numeric functions refuse', async () => {
  for (const functionName of ['SVSmr', 'SVBioGeoClimaticUnit', 'SVAspect']) {
    const source = structuredClone(wire);
    const row = source.rows.find(item => item.field.cells[5].text === functionName);
    row.values[0] = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
    await assert.rejects(client.validateSummaryEnvironmental(source, owner, yieldNow));
  }
  for (const functionName of ['SVHumusDepth', 'SVSlope', 'SVSeepage']) {
    const source = structuredClone(wire), row = source.rows[0];
    const physical = source.fieldList.rows.find(field => field.rowId === row.field.rowId);
    physical.cells[5].text = row.field.cells[5].text = functionName;
    await assert.rejects(client.validateSummaryEnvironmental(source, owner, yieldNow));
  }
});

test('actual deep Svelte proxy is detached; field witnesses and future reads cannot alias', async () => {
  const source = proxy(structuredClone(wire));
  assert.throws(() => structuredClone(source));
  const result = await client.validateSummaryEnvironmental(source, owner, yieldNow);
  source.fieldList.rows[0].cells[2].text = 'caller';
  source.rows[0].values[0].text = 'caller';
  source.vegetation.units[0].code = 'caller';
  assert.deepEqual(plain(result), wire);
});

test('foreign owner, selections, source labels and malformed typed output refuse entire matrix', async () => {
  const changes = [
    value => { value.source = 'independently merged'; },
    value => { value.extra = true; },
    value => { value.sourcePath = ''; },
    value => { value.envSourcePath = 'foreign.db'; },
    value => { value.vegetation.contextId = 'foreign'; },
    value => { value.vegetation.average = 2; },
    value => { value.rows.pop(); },
    value => { value.rows.reverse(); },
    value => { value.rows[0].field.cells[2].text = 'changed witness'; },
    value => { value.fieldList.columns[0].type = 'INTEGER'; },
    value => { value.fieldList.rows[0].cells[4].integer = '1'; },
    value => { value.rows[0].values.pop(); },
    value => { value.rows[0].values[0].integer = '1'; },
    value => { value.rows[0].values[0].text = '\ud800'; },
    value => { value.rows[0].labelColumn = 3; },
    value => {
      const count = value.rows.find(row => row.field.cells[5].text === 'SVnPlots');
      count.values[0].integer = '3';
    },
    value => {
      const row = value.rows[0], physical = value.fieldList.rows.find(field => field.rowId === row.field.rowId);
      row.field.cells[5].text = physical.cells[5].text = 'SVUnknown';
    },
    value => {
      const row = value.rows[0], physical = value.fieldList.rows.find(field => field.rowId === row.field.rowId);
      row.field.cells[6] = physical.cells[6] = { storage: 'integer', text: null, integer: '1', real: null, blobHex: null };
    },
  ];
  for (const change of changes) {
    const value = structuredClone(wire); change(value);
    await assert.rejects(client.validateSummaryEnvironmental(value, owner, yieldNow));
  }
});

test('cancel held transport retains diagnostic, blocks replay until settlement and rejects stale data', async () => {
  const session = new client.SummaryEnvironmentalSession(owner), held = deferred();
  let cancelled = 0;
  held.promise.cancel = () => { cancelled++; };
  const operation = session.show({ preview: () => held.promise, validationYield: yieldNow });
  assert.equal(session.view().busy, true);
  session.cancel();
  assert.equal(cancelled, 1);
  assert.equal(session.view().settling, true);
  await assert.rejects(session.show({ preview: () => Promise.resolve(wire) }), /settle/);
  held.resolve(wire);
  assert.equal(await operation, false);
  assert.equal(session.view().result, null);
  assert.match(session.view().error, /cancelled/);
  assert.equal(session.view().settling, false);
  assert.equal(await session.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
});

test('every physical field witness cell is dense and independently required', async () => {
  for (let cell = 0; cell < 7; cell++) {
    const value = structuredClone(wire);
    delete value.rows[0].field.cells[cell];
    await assert.rejects(client.validateSummaryEnvironmental(value, owner, yieldNow));
  }
  const extra = structuredClone(wire);
  extra.rows[0].field.cells.unexpected = 'not a source cell';
  await assert.rejects(client.validateSummaryEnvironmental(extra, owner, yieldNow));
});

test('cancel after awaited validation never assigns old result on retained remount', async () => {
  const registry = client.summaryEnvironmentalRegistry(true), session = registry.session(owner);
  const held = deferred(), reached = deferred();
  let yields = 0;
  const operation = session.show({ preview: () => Promise.resolve(wire), validationYield: () => {
    yields++; reached.resolve(); return held.promise;
  } });
  await reached.promise;
  session.cancel();
  const remounted = registry.session(owner);
  assert.equal(remounted, session);
  assert.equal(registry.activity().busy, true);
  held.resolve();
  assert.equal(await operation, false);
  assert.equal(yields, 1);
  assert.equal(remounted.view().result, null);
  assert.equal(registry.activity().busy, false);
  assert.equal(await remounted.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
});

test('read and cancellation transport failures remain explicit across owner remount and recover', async () => {
  const registry = client.summaryEnvironmentalRegistry(true), session = registry.session(owner);
  assert.equal(await session.show({ preview: () => Promise.reject(new Error('owned SQL failed')) }), false);
  assert.match(registry.session(owner).view().error, /owned SQL failed/);
  const held = deferred(); held.promise.cancel = () => { throw new Error('cancel failed'); };
  const operation = session.show({ preview: () => held.promise });
  session.cancel();
  assert.match(session.view().error, /cancel failed/);
  held.reject(new Error('old failure'));
  await operation;
  assert.match(session.view().error, /cancel failed/);
  assert.equal(await session.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
  assert.equal(session.view().error, '');
});

test('literal owner key, disabled entry, capacity and no selected rows preserve availability distinctions', async () => {
  assert.throws(() => client.summaryEnvironmentalRegistry(false).session(owner), /disabled/);
  const registry = client.summaryEnvironmentalRegistry(true);
  const first = registry.session(owner);
  assert.equal(first instanceof client.SummaryEnvironmentalSession, true);
  for (let i = 1; i < 16; i++) registry.session({ ...owner, contextId: String(i) });
  assert.throws(() => registry.session({ ...owner, contextId: '17' }), /capacity/);
  assert.equal(registry.session(owner), first);
  for (const wrong of [{ ...owner, su: 'None' }, { ...owner, suPath: '' }, { ...owner, extra: '' }])
    assert.throws(() => new client.SummaryEnvironmentalSession(wrong));
  const empty = structuredClone(wire);
  for (const row of empty.fieldList.rows) row.cells[4] = { storage: 'integer', text: null, integer: '0', real: null, blobHex: null };
  empty.rows = [];
  assert.deepEqual(plain(await client.validateSummaryEnvironmental(empty, owner, yieldNow)), empty);
});
