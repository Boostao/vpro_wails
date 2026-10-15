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
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalNumericPreviewWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.vegetation[key]]));
const plain = value => JSON.parse(JSON.stringify(value));
const yieldNow = () => Promise.resolve();
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};

test('strict-Go seventeen selected rows preserve direct numeric strings, all suffixes and explicit Empty', async () => {
  const result = await client.validateSummaryEnvironmentalNumeric(wire, owner, yieldNow);
  assert.deepEqual(plain(result), wire);
  assert.equal(result.rows.length, 17);
  const expected = { SVSeepage: '1 - 2.3 - 5', SVRootingDepth: '2 - 3.3 - 6',
    SVStandAge: '3 - 04 - 7', SVElevation: '4 - 05 - 8', SVSlope: '1.25 - 3 - 5.75' };
  for (const [fn, text] of Object.entries(expected)) {
    const row = result.rows.find(row => row.field.cells[5].text === fn);
    assert.ok(row, fn);
    assert.equal(row.values[0].kind, 'cell');
    assert.equal(row.values[0].cell.text, text);
    if (fn === 'SVStandAge' || fn === 'SVElevation')
      assert.equal(row.values[1].cell.text, fn === 'SVStandAge' ? '(Null 1)' : '  (Null 1)');
    else assert.deepEqual(plain(row.values[1]), { kind: 'empty', cell: null });
  }
  assert.equal(result.rows.filter(row => row.field.cells[5].text === 'SVSubstrate').length, 6);
  assert.equal(result.rows.filter(row => row.field.cells[5].text === 'SVStrataCover').length, 4);
  const count = result.rows.find(row => row.field.cells[5].text === 'SVnPlots');
  assert.equal(count.labelColumn, 3);
  assert.deepEqual(plain(count.values.map(value => value.cell.integer)), ['4', '1']);
  assert.equal(Object.isFrozen(result.rows[0].values[0]), true);
  await assert.rejects(client.validateSummaryEnvironmental(wire, owner, yieldNow));
});

test('real Svelte proxy numeric preview is detached and deeply frozen without clone repair', async () => {
  const source = proxy(structuredClone(wire));
  assert.throws(() => structuredClone(source));
  const result = await client.validateSummaryEnvironmentalNumeric(source, owner, yieldNow);
  source.rows[0].values[0].cell.text = 'caller';
  source.fieldList.rows[0].cells[2].text = 'caller';
  source.vegetation.units[0].code = 'caller';
  assert.deepEqual(plain(result), wire);
  assert.equal(Object.isFrozen(result.fieldList.rows[0].cells), true);
});

test('numeric validator rejects owner, argument, dense witness and tagged value inconsistencies', async () => {
  const changes = [
    value => { value.source = client.summaryEnvironmentalSource; },
    value => { value.envSourcePath = 'other.db'; },
    value => { value.vegetation.contextId = 'other'; },
    value => { value.rows.reverse(); },
    value => { value.rows.pop(); },
    value => { value.rows[0].field.cells[2].text = 'changed'; },
    value => { delete value.rows[0].values[0]; },
    value => { value.rows[0].values.extra = true; },
    value => { value.rows[0].values[0].extra = true; },
    value => { value.rows[0].values[0] = { kind: 'empty' }; },
    value => { value.rows[0].values[0] = { kind: 'empty', cell: value.rows[0].values[0].cell }; },
    value => { value.rows[0].values[0].cell.text = '\ud800'; },
    value => { value.rows.find(row => row.field.cells[5].text === 'SVStandAge').values[0] = { kind: 'empty', cell: null }; },
    value => { value.rows.find(row => row.field.cells[5].text === 'SVnPlots').values[0].cell.integer = '3'; },
    value => {
      const row = value.rows.find(row => row.field.cells[5].text === 'SVSubstrate');
      const physical = value.fieldList.rows.find(field => field.rowId === row.field.rowId);
      physical.cells[6].text = row.field.cells[6].text = 'decwood';
    },
    value => {
      const row = value.rows.find(row => row.field.cells[5].text === 'SVSlope');
      const physical = value.fieldList.rows.find(field => field.rowId === row.field.rowId);
      row.field.cells[6] = physical.cells[6] = { storage: 'text', text: '', integer: null, real: null, blobHex: null };
    },
  ];
  for (const [index, change] of changes.entries()) {
    const source = structuredClone(wire); change(source);
    await assert.rejects(client.validateSummaryEnvironmentalNumeric(source, owner, yieldNow), `case ${index}`);
  }
});

test('numeric retained session cancels held validation and preserves owner/remount/error/retry', async () => {
  const registry = client.summaryEnvironmentalNumericRegistry(true);
  const session = registry.session(owner), reached = deferred(), held = deferred();
  const operation = session.show({ preview: () => Promise.resolve(wire), validationYield: () => {
    reached.resolve(); return held.promise;
  } });
  await reached.promise;
  session.cancel();
  assert.equal(registry.session(owner), session);
  assert.equal(registry.activity().busy, true);
  await assert.rejects(session.show({ preview: () => Promise.resolve(wire) }), /settle/);
  held.resolve();
  assert.equal(await operation, false);
  assert.equal(registry.activity().busy, false);
  assert.equal(session.view().result, null);
  assert.match(session.view().error, /cancelled/);
  assert.equal(await session.show({ preview: () => Promise.reject(new Error('numeric read failed')) }), false);
  assert.match(registry.session(owner).view().error, /numeric read failed/);
  assert.equal(await session.show({ preview: () => Promise.resolve(wire), validationYield: yieldNow }), true);
  assert.equal(session.view().error, '');
  assert.deepEqual(plain(session.view().result), wire);
});

test('numeric and legacy registries remain separate; disabled and capacity refuse without eviction', () => {
  assert.throws(() => client.summaryEnvironmentalNumericRegistry(false).session(owner), /disabled/);
  const numericRegistry = client.summaryEnvironmentalNumericRegistry(true);
  const legacy = client.summaryEnvironmentalRegistry(true).session(owner);
  const first = numericRegistry.session(owner);
  assert.equal(first instanceof client.SummaryEnvironmentalNumericSession, true);
  assert.equal(legacy instanceof client.SummaryEnvironmentalSession, true);
  assert.notEqual(first, legacy);
  for (let i = 1; i < 16; i++) numericRegistry.session({ ...owner, contextId: String(i) });
  assert.throws(() => numericRegistry.session({ ...owner, contextId: '17' }), /capacity/);
  assert.equal(numericRegistry.session(owner), first);
  for (const invalid of [{ ...owner, su: 'None' }, { ...owner, suPath: '' }, { ...owner, extra: true }])
    assert.throws(() => new client.SummaryEnvironmentalNumericSession(invalid));
});
