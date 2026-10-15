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
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalCanonicalWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.vegetation[key]]));
const plain = value => JSON.parse(JSON.stringify(value));

test('actual client proxy accepts all53 supported canonical definitions and retains unselected Humus diagnostic storage', async () => {
  const source = proxy(structuredClone(wire));
  assert.throws(() => structuredClone(source));
  const result = await client.validateSummaryEnvironmentalLifeform(source, owner, () => Promise.resolve());
  assert.deepEqual(plain(result), wire);
  assert.equal(result.fieldList.rows.length, 54);
  assert.equal(result.rows.length, 53);
  assert.equal(result.rows.filter(row => row.field.cells[5].text === 'SVLifeformCover').length, 13);
  assert.equal(result.rows.some(row => row.field.cells[5].text === 'SVHumusDepth'), false);
  const unresolved = result.fieldList.rows.find(row => row.cells[5].text === 'SVHumusDepth');
  assert.equal(unresolved.cells[4].integer, '0');
  source.masterSourcePath = 'caller';
  source.rows[0].values[0].cell.integer = 'caller';
  assert.deepEqual(plain(result), wire);
  assert.equal(Object.isFrozen(result.rows), true);
});

test('canonical no-Env categories and ages remain ordinary empty strings, numeric Empty stays explicit', async () => {
  const result = await client.validateSummaryEnvironmentalLifeform(wire, owner, () => Promise.resolve());
  const emptyNumeric = new Set(['SVSeepage', 'SVRootingDepth', 'SVSlope', 'SVSubstrate', 'SVStrataCover']);
  for (const row of result.rows) {
    const fn = row.field.cells[5].text;
    if (emptyNumeric.has(fn)) {
      assert.deepEqual(plain(row.values), [{ kind: 'empty', cell: null }, { kind: 'empty', cell: null }]);
    } else if (fn !== 'SVnPlots' && fn !== 'SVLifeformCover') {
      assert.deepEqual(plain(row.values.map(value => value.cell.text)), ['', '']);
    }
  }
});

test('selected unresolved canonical Humus refuses complete client result even with a success-shaped row', async () => {
  const source = structuredClone(wire);
  const physical = source.fieldList.rows.find(row => row.cells[5].text === 'SVHumusDepth');
  physical.cells[4].integer = '-1';
  const position = source.rows.findIndex(row => Number(row.field.cells[0].integer) > Number(physical.cells[0].integer));
  assert.ok(position >= 0);
  source.rows.splice(position, 0, {
    field: structuredClone(physical), labelColumn: 2,
    values: [{ kind: 'empty', cell: null }, { kind: 'empty', cell: null }],
  });
  await assert.rejects(client.validateSummaryEnvironmentalLifeform(source, owner, () => Promise.resolve()),
    /Combined Summary environment is incomplete, inconsistent or belongs to another owner; no repairs applied/);
});
