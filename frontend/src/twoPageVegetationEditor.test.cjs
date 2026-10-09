const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { wire, editor, fixture, original, cell } = require('./twoPageVegetationTestHelpers.cjs');
function draftSet(source) {
  return {
    cover: editor.stageTwoPageVegetation(source, '-7', 'Cover1', '5', false),
    collected: editor.stageTwoPageVegetation(source, '-7', 'Collected', '', true),
    extended: editor.stageTwoPageVegetation(source, '-7', 'Cover5c', '99.5', false),
  };
}
function fieldCell(source, column) {
  return source.records.rows[0].cells[wire.vegetationColumnIndex(source, column)];
}
function variant(form) {
  const source = original();
  source.form = form;
  if (form === 'SubVegCXL' || form === 'SubVegDXL') {
    source.recordSource = form === 'SubVegCXL' ? 'USysVegC' : 'USysVegD';
    Object.assign(fieldCell(source, form === 'SubVegCXL' ? 'Cover6' : 'Cover7'), cell('real', 0));
    source.sourceRowIds = ['-7'];
  }
  return source;
}

test('vegetation strict-Go wire detaches actual Svelte proxies and preserves negative-zero physical witnesses', () => {
  const source = client.proxy(original());
  const drafts = client.proxy(draftSet(source));
  const request = editor.twoPageVegetationRequest(source, fixture.owner, drafts, fixture.request.requestId);
  assert.deepEqual(structuredClone(request), fixture.request);
  assert.doesNotThrow(() => structuredClone(request));
  const encoded = wire.serializeTwoPageVegetationRequest(request);
  assert.deepEqual(JSON.parse(encoded), fixture.request);
  assert.equal(Object.is(JSON.parse(encoded).original.records.rows[1].cells[4].real, -0), true);
  assert.equal(Object.is(JSON.parse(JSON.stringify(request)).original.records.rows[1].cells[4].real, -0), false);
  source.records.rows[0].cells[1].text = 'changed source';
  drafts.cover.value.real = 8;
  assert.equal(request.original.records.rows[0].cells[1].text, '108050');
  assert.equal(request.edits[0].value.real, 5);
});

test('complete originals preserve physical membership separately and reject incomplete or unrelated wire', () => {
  assert.deepEqual(structuredClone(wire.twoPageVegetationOriginalFromWire(client.proxy(original()), fixture.owner)), fixture.request.original);
  for (const mutate of [
    source => source.contextId += '-stale',
    source => source.recordSource = 'USysVegD',
    source => source.table = 'Other_Veg',
    source => source.records.columns[0] = { name: 'ID', type: 'INTEGER' },
    source => source.records.columns[19].name = 'COVER1',
    source => source.records.columns[19].name = '\u017fpecies',
    source => source.records.rows[0].rowId = '-07',
    source => source.records.rows[1].rowId = '-7',
    source => source.records.rows[0].cells.pop(),
    source => source.records.rows[0].cells[1] = cell('integer', '108050'),
    source => source.records.rows[0].cells[2].text = '\ud800',
    source => source.records.rows[0].cells[19].blobHex = 'FF00',
    source => source.records.rows[0].cells[3].real = Infinity,
    source => source.records.rows[0].cells[3].unexpected = true,
    source => source.sourceRowIds.reverse(),
    source => source.sourceRowIds = ['-7'],
    source => source.records.extra = true,
  ]) {
    const source = original(); mutate(source);
    assert.throws(() => wire.twoPageVegetationOriginalFromWire(source, fixture.owner));
  }
  const source = original();
  source.records.rows[1].cells[4] = cell();
  source.sourceRowIds = ['-7'];
  assert.equal(wire.twoPageVegetationOriginalFromWire(source, fixture.owner).records.rows.length, 2);
});

test('all25 source bindings are independently mapped and all21 cover controls enforce strict below100 without lower bound', () => {
  for (const [form, fields] of [
    ['SubVegAXL_BC', ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB', 'Collected']],
    ['SubVegAXL', ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB', 'Cover5a', 'Cover5b', 'Cover5c', 'Collected']],
    ['SubVegCXL', ['Cover6', 'Collected']],
    ['SubVegDXL', ['Cover7', 'Cover8', 'Cover9', 'Collected']],
  ]) {
    const source = variant(form);
    assert.deepEqual(Array.from(editor.twoPageVegetationFields(source)), fields);
    for (const column of fields) {
      assert.equal(editor.stageTwoPageVegetation(source, '-7', column, column === 'Collected' ? 'x' : '5', false).error, null);
      assert.equal(editor.stageTwoPageVegetation(source, '-7', column, '', true).error, null);
      if (column === 'Collected') continue;
      for (const invalid of ['99.99999999999999', '99.999999', '100', '100.00000000000001', '101', '-0', '-1e-46', '1e39', ' 5', '5 ', 'NaN', '5x', '']) {
        assert.notEqual(editor.stageTwoPageVegetation(source, '-7', column, invalid, false).error, null, `${form}.${column} ${invalid}`);
      }
      for (const valid of ['99.99999', '1e-46', '-1', '-3.4028234663852886e38']) {
        assert.equal(editor.stageTwoPageVegetation(source, '-7', column, valid, false).error, null, `${form}.${column} ${valid}`);
      }
      for (const historical of [99.999999, 0.1, 1e-46, 100, 101, 1e40, -0]) {
        Object.assign(fieldCell(source, column), cell('real', historical));
        const raw = Object.is(historical, -0) ? '-0' : historical === 100 ? '100.0' : String(historical);
        const staged = editor.stageTwoPageVegetation(source, '-7', column, raw, false);
        assert.equal(staged.error, null);
        assert.equal(wire.equalVegetationCell(staged.expected, staged.value), true);
      }
      Object.assign(fieldCell(source, column), cell());
    }
  }
  assert.equal(Math.fround(99.99999999999999), 100, 'source validation applies after SINGLE narrowing');
});

test('all21 numeric bindings match independently observed Access SINGLE storage and preserve raw draft text', () => {
  const calibration = require('./twoPageVegetationSingle.json');
  for (const form of ['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL']) {
    const source = variant(form);
    for (const column of editor.twoPageVegetationFields(source).filter(column => column !== 'Collected')) {
      for (const observed of calibration.cases) {
        Object.assign(fieldCell(source, column), cell('real', 1));
        const staged = editor.stageTwoPageVegetation(source, '-7', column, observed.raw, false);
        assert.equal(staged.error === null, observed.clientAccepted, `${form}.${column} ${observed.raw}`);
        assert.equal(staged.raw, observed.raw);
        if (observed.clientAccepted) {
          assert.equal(staged.value.storage, 'real');
          assert.equal(Object.is(staged.value.real, observed.nativeValue), true);
          const request = editor.twoPageVegetationRequest(source, {
            contextId: source.contextId, project: source.project, plot: source.plot, form,
          }, { staged }, fixture.request.requestId);
          assert.equal(Object.is(request.edits[0].value.real, observed.nativeValue), true);
        }
      }
    }
  }
});

test('SINGLE midpoint neighbours validate after narrowing rather than against the raw binary64 bound', () => {
  const source = original();
  Object.assign(fieldCell(source, 'Cover1'), cell('real', 1));
  for (const [raw, expected] of [
    ['99.99999618530272', 99.99999237060547],
    ['99.99999618530273', null],
    ['99.99999618530275', null],
    ['0.1', 0.10000000149011612],
  ]) {
    const staged = editor.stageTwoPageVegetation(source, '-7', 'Cover1', raw, false);
    assert.equal(staged.error === null, expected !== null, raw);
    if (expected !== null) assert.equal(staged.value.real, expected);
  }
});

test('nonunique signed32 IDs are allowed peers while NULL/noninteger identities and unavailable source actions remain read-only', () => {
  const source = original();
  source.records.rows[1].cells[0] = cell('integer', '42');
  assert.equal(editor.twoPageVegetationEditableID(source, '-7'), 42);
  for (const id of ['-2147483648', '0', '2147483647']) {
    source.records.rows[0].cells[0] = cell('integer', id);
    assert.equal(editor.twoPageVegetationEditableID(source, '-7'), Number(id));
  }
  for (const id of [cell(), cell('text', '42'), cell('real', 42), cell('integer', '2147483648'), cell('integer', '00')]) {
    source.records.rows[0].cells[0] = id;
    assert.throws(() => editor.twoPageVegetationEditableID(source, '-7'));
  }
  assert.throws(() => editor.stageTwoPageVegetation(original(), '9223372036854775807', 'Cover1', '5', false));
  for (const column of ['Species', 'ID', 'Unmapped', 'HeightB']) {
    assert.throws(() => editor.stageTwoPageVegetation(original(), '-7', column, 'x', false));
  }
  assert.throws(() => editor.stageTwoPageVegetation(variant('SubVegAXL_BC'), '-7', 'Cover5a', '5', false));
});

test('Collected preserves NULL/empty/manual TEXT1 and unchanged historical invalid values without invented enum or repair', () => {
  const source = original();
  for (const value of ['C', 'V', 'x', '', 'é']) {
    assert.equal(editor.stageTwoPageVegetation(source, '-7', 'Collected', value, false).error, null);
  }
  for (const value of ['😀', 'CV', '\ud800']) {
    assert.notEqual(editor.stageTwoPageVegetation(source, '-7', 'Collected', value, false).error, null);
  }
  Object.assign(fieldCell(source, 'Collected'), cell('text', 'historical invalid'));
  const staged = editor.stageTwoPageVegetation(source, '-7', 'Collected', 'historical invalid', false);
  assert.equal(staged.error, null);
  assert.equal(wire.equalVegetationCell(staged.expected, staged.value), true);
  assert.equal(editor.stageTwoPageVegetation(source, '-7', 'Collected', '', false).value.storage, 'text');
  assert.equal(editor.stageTwoPageVegetation(source, '-7', 'Collected', '', true).value.storage, 'null');
});

test('request rebuilding rejects stale ownership, raw/error/value tampering, repeated assignments and no-op plans', () => {
  for (const mutate of [
    drafts => drafts.cover.contextId += '-stale',
    drafts => drafts.cover.expected = cell('real', 1),
    drafts => drafts.cover.value.real = 6,
    drafts => drafts.cover.error = 'retained draft error',
    drafts => drafts.cover.raw = '100',
    drafts => drafts.duplicate = structuredClone(drafts.cover),
  ]) {
    const source = original(), drafts = draftSet(source); mutate(drafts);
    assert.throws(() => editor.twoPageVegetationRequest(source, fixture.owner, drafts, fixture.request.requestId));
  }
  assert.throws(() => editor.twoPageVegetationRequest(original(), fixture.owner, {}, fixture.request.requestId));
  assert.throws(() => editor.twoPageVegetationRequest(original(), fixture.owner, draftSet(original()), 'INVALID'));
});

test('column identifier witnesses match strict Go folding without folding data values or expanding literals', () => {
  for (const witness of fixture.columnAliases) {
    assert.equal(wire.vegetationIdentifierKey(witness.left) === wire.vegetationIdentifierKey(witness.right), witness.equal);
    const source = original();
    source.records.columns.push({ name: witness.left + '_collision', declaredType: 'TEXT' },
      { name: witness.right + '_collision', declaredType: 'TEXT' });
    for (const row of source.records.rows) row.cells.push(cell(), cell());
    if (witness.equal) assert.throws(() => wire.twoPageVegetationOriginalFromWire(source, fixture.owner));
    else assert.doesNotThrow(() => wire.twoPageVegetationOriginalFromWire(source, fixture.owner));
  }
  const table = require('./twoPageVegetationIdentifierFold.json');
  const keys = new Set();
  for (const group of table.groups) {
    const canonical = Array.from(group)[0];
    assert.equal(keys.has(canonical), false);
    keys.add(canonical);
    for (const rune of Array.from(group)) assert.equal(wire.vegetationIdentifierKey(rune), canonical);
  }
});
