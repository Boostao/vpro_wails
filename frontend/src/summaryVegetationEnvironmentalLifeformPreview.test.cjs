const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const fields = loadTypeScript('summaryVegetationFieldList.ts');
const vegetation = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform }, { setTimeout });
const numeric = loadTypeScript('summaryVegetationEnvironmentalNumeric.ts', { './summaryVegetationFieldList': fields });
const client = loadTypeScript('summaryVegetationEnvironmental.ts', {
  './lifeformSummary': lifeform, './summaryVegetationFieldList': fields, './summaryVegetation': vegetation,
  './summaryVegetationEnvironmentalNumeric': numeric,
}, { setTimeout });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalLifeformPreviewWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.vegetation[key]]));
const plain = value => JSON.parse(JSON.stringify(value));
const yieldNow = () => Promise.resolve();
const deferred = () => {
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  return { promise, resolve };
};

test('strict-Go thirty-row lifeform snapshot retains all thirteen exact arguments and master provenance', async () => {
  const result = await client.validateSummaryEnvironmentalLifeform(wire, owner, yieldNow);
  assert.deepEqual(plain(result), wire);
  assert.equal(result.rows.length, 30);
  assert.equal(result.masterSourcePath, 'C:\\fixture\\VLists.db');
  const rows = result.rows.filter(row => row.field.cells[5].text === 'SVLifeformCover');
  assert.equal(rows.length, 13);
  assert.deepEqual(plain(rows.map(row => Number(row.field.cells[6].text)).sort((a, b) => a - b)),
    [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13]);
  for (const row of rows) {
    assert.equal(row.values[0].cell.text, '3.8 - 4.4 - 5.0');
    assert.equal(row.values[1].kind, 'cell');
    assert.equal(row.values[1].cell.text, row.field.cells[6].text === '1' ? '0.0 - 0.0 - 0.0' : '');
  }
  const seepage = result.rows.find(row => row.field.cells[5].text === 'SVSeepage');
  assert.deepEqual(plain(seepage.values[1]), { kind: 'empty', cell: null });
  assert.equal(Object.isFrozen(result), true);
  assert.equal(Object.isFrozen(result.rows[0].values), true);
  await assert.rejects(client.validateSummaryEnvironmentalNumeric(wire, owner, yieldNow));
  await assert.rejects(client.validateSummaryEnvironmental(wire, owner, yieldNow));
});

test('actual client Svelte proxy is detached while lifeform String empty remains distinct from numeric Empty', async () => {
  const source = proxy(structuredClone(wire));
  assert.throws(() => structuredClone(source));
  const result = await client.validateSummaryEnvironmentalLifeform(source, owner, yieldNow);
  source.masterSourcePath = 'caller';
  source.fieldList.rows[0].cells[2].text = 'caller';
  source.rows[0].values[0].cell.text = 'caller';
  assert.deepEqual(plain(result), wire);
});

test('lifeform wire rejects incomplete master, owner, selected argument, dense witness and tagged values', async () => {
  const changes = [
    value => { delete value.masterSourcePath; },
    value => { value.masterSourcePath = ''; },
    value => { value.masterSourcePath = '\ud800'; },
    value => { value.masterSourcePath = null; },
    value => { value.extra = true; },
    value => { value.source = client.summaryEnvironmentalNumericSource; },
    value => { value.vegetation.contextId = 'foreign'; },
    value => { value.envSourcePath = 'foreign'; },
    value => { value.rows.reverse(); },
    value => { delete value.rows[0].values[0]; },
    value => { value.rows[0].values.extra = true; },
    value => { value.rows[0].field.cells.pop(); },
    value => {
      const row = value.rows.find(row => row.field.cells[5].text === 'SVLifeformCover');
      row.values[1] = { kind: 'empty', cell: null };
    },
    value => {
      const row = value.rows.find(row => row.field.cells[5].text === 'SVLifeformCover');
      row.values[0].cell.text = '\ud800';
    },
    value => {
      const row = value.rows.find(row => row.field.cells[5].text === 'SVnPlots');
      row.values[0].cell.integer = '3';
    },
  ];
  for (const [index, change] of changes.entries()) {
    const source = structuredClone(wire);
    change(source);
    await assert.rejects(client.validateSummaryEnvironmentalLifeform(source, owner, yieldNow), `case ${index}`);
  }
  for (const argument of ['', '03', '3 ', '0', '14', '-1', '3.0']) {
    const source = structuredClone(wire);
    const row = source.rows.find(row => row.field.cells[5].text === 'SVLifeformCover');
    const physical = source.fieldList.rows.find(field => field.rowId === row.field.rowId);
    row.field.cells[6].text = physical.cells[6].text = argument;
    await assert.rejects(client.validateSummaryEnvironmentalLifeform(source, owner, yieldNow), argument);
  }
  const source = structuredClone(wire);
  const row = source.rows.find(row => row.field.cells[5].text === 'SVLifeformCover');
  const physical = source.fieldList.rows.find(field => field.rowId === row.field.rowId);
  row.field.cells[6] = physical.cells[6] = { storage: 'integer', integer: '3', text: null, real: null, blobHex: null };
  await assert.rejects(client.validateSummaryEnvironmentalLifeform(source, owner, yieldNow));
});

test('lifeform retained session keeps held validation cancelled across remount and supports explicit retry', async () => {
  const registry = client.summaryEnvironmentalLifeformRegistry(true);
  const session = registry.session(owner);
  assert.ok(session instanceof client.SummaryEnvironmentalLifeformSession);
  const entered = deferred(), release = deferred();
  let cancelled = 0;
  const read = Promise.resolve(wire);
  read.cancel = () => { cancelled++; };
  let held = false;
  const showing = session.show({
    preview: () => read,
    validationYield: async () => {
      if (!held) { held = true; entered.resolve(); await release.promise; }
    },
  });
  await entered.promise;
  session.cancel();
  assert.equal(registry.session(owner), session);
  assert.equal(registry.activity().busy, true);
  assert.equal(session.view().result, null);
  assert.match(session.view().error, /cancelled/);
  await assert.rejects(session.show({ preview: () => Promise.resolve(wire) }), /settle/);
  release.resolve();
  assert.equal(await showing, false);
  assert.equal(cancelled, 1);
  assert.equal(session.view().result, null);
  assert.equal(registry.activity().busy, false);
  assert.equal(await session.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
  assert.equal(session.view().result.masterSourcePath, 'C:\\fixture\\VLists.db');
  assert.equal(registry.session(owner).view().result, session.view().result);
});

test('lifeform final wrapper checks generation, failures survive remount, and registries stay separate', async () => {
  let current = true;
  const result = await client.validateSummaryEnvironmentalLifeform(wire, owner, async () => { current = false; }, () => current);
  assert.equal(result, null);
  const registry = client.summaryEnvironmentalLifeformRegistry(true);
  const session = registry.session(owner);
  assert.equal(await session.show({ preview: () => Promise.reject(new Error('read failed')) }), false);
  assert.match(registry.session(owner).view().error, /read failed/);
  assert.equal(await session.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
  assert.equal(session.view().error, '');
  assert.notEqual(client.summaryEnvironmentalNumericRegistry(true).session(owner), session);
  assert.throws(() => client.summaryEnvironmentalLifeformRegistry(false).session(owner), /disabled/);
  for (let i = 1; i < 16; i++) registry.session({ ...owner, contextId: `owner-${i}` });
  assert.throws(() => registry.session({ ...owner, contextId: 'overflow' }), /capacity/);
  assert.equal(registry.session(owner), session);
});
