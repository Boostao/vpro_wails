const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const fields = loadTypeScript('summaryVegetationFieldList.ts');
const client = loadTypeScript('summaryVegetationEnvironmentalNumeric.ts', { './summaryVegetationFieldList': fields });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalNumericWire.json'), 'utf8'));
const plain = value => JSON.parse(JSON.stringify(value));
const cell = (storage, text = null) => ({ storage, text, integer: null, real: null, blobHex: null });

test('144 strict-Go successful source cases preserve Empty and String bytes', () => {
  assert.equal(wire.length, 144);
  const functions = new Set();
  for (const item of wire) {
    functions.add(item.function);
    assert.deepEqual(plain(client.numericEnvironmentalValue(item.function, item.argument, item.value)), item.value);
  }
  assert.equal(functions.size, 7);
  assert.deepEqual(wire.find(item => item.function === 'SVSeepage' && item.unit === 'allnull').value,
    { kind: 'empty', cell: null });
  assert.equal(wire.find(item => item.function === 'SVStandAge' && item.unit === 'allnull').value.cell.text, '(Null 2)');
  assert.equal(wire.find(item => item.function === 'SVElevation' && item.unit === 'allnull').value.cell.text, '  (Null 2)');
  assert.equal(wire.find(item => item.function === 'SVStandAge' && item.unit === 'missing').value.cell.text, '');
});

test('actual Svelte proxy is detached at typed transport boundary', () => {
  const item = wire.find(item => item.function === 'SVSubstrate' && item.unit === 'mixed');
  const source = proxy(structuredClone(item));
  assert.throws(() => structuredClone(source));
  const result = client.numericEnvironmentalValue(source.function, source.argument, source.value);
  source.value.cell.text = 'caller';
  source.argument.text = 'caller';
  assert.deepEqual(plain(result), item.value);
});

test('malformed output cannot disguise metadata NULL or String empty as Empty', () => {
  const argument = cell('null');
  for (const value of [
    null, [], {}, { kind: 'empty' }, { kind: 'empty', cell: cell('null') },
    { kind: 'empty', cell: cell('text', '') }, { kind: 'null', cell: null },
    { kind: 'cell', cell: null }, { kind: 'cell', cell: cell('null') },
    { kind: 'cell', cell: cell('integer') }, { kind: 'empty', cell: null, extra: true },
    { kind: 'cell', cell: { ...cell('text', 'valid'), real: 1 } },
    { kind: 'cell', cell: cell('text', '\ud800') },
    Object.assign({ kind: 'empty', cell: null }, { [Symbol('foreign')]: true }),
  ]) assert.throws(() => client.numericEnvironmentalValue('SVSeepage', argument, value));
  for (const name of ['SVStandAge', 'SVElevation'])
    assert.throws(() => client.numericEnvironmentalValue(name, argument, { kind: 'empty', cell: null }));
});

test('function and argument guards retain exact NULL and TEXT suffix policy', () => {
  for (const name of ['SVSeepage', 'SVRootingDepth', 'SVStandAge', 'SVElevation', 'SVSlope']) {
    assert.equal(client.numericEnvironmentalArgument(name, cell('null')), '');
    for (const argument of [cell('text', ''), cell('text', '1'), { ...cell('null'), text: '1' }])
      assert.throws(() => client.numericEnvironmentalArgument(name, argument));
  }
  for (const [name, suffixes] of [['SVSubstrate', ['DecWood', 'BedRock', 'Rocks', 'MineralSoil', 'OrganicMatter', 'Water']],
    ['SVStrataCover', ['Tree', 'Shrub', 'Herb', 'Moss']]]) {
    for (const suffix of suffixes) {
      assert.equal(client.numericEnvironmentalArgument(name, cell('text', suffix)), suffix);
      for (const changed of [suffix.toLowerCase(), `${suffix} `, ` ${suffix}`])
        assert.throws(() => client.numericEnvironmentalArgument(name, cell('text', changed)));
    }
    for (const argument of [cell('null'), cell('text', ''), cell('text', '1'),
      { ...cell('integer'), integer: '1' }, cell('text', '\ud800')])
      assert.throws(() => client.numericEnvironmentalArgument(name, argument));
  }
  for (const name of ['SVHumusThickness', 'SVLifeformCover', 'SVSlope ', 'svSlope', 'SVnPlots'])
    assert.throws(() => client.numericEnvironmentalArgument(name, cell('null')));
});
