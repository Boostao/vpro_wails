const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { transport, cell } = require('./siviParentTestHelpers.cjs');
const defaults = require('../../resources/sivi-new-row-defaults.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ids = loadTypeScript('siviRequestId.ts');
const editor = loadTypeScript('siviDeletionEditor.ts', {
  '../../resources/sivi-new-row-defaults.json': defaults, './qualityEditor': quality,
  './siviParentTransport': transport, './siviRequestId': ids,
});
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const token = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
function original(form = 'SubVegA-SIVI_BC', column = 'Cover1') {
  const names = [...defaults.initialNullColumns, 'Flag', 'ID'];
  return { ...owner, form, columns: names.map(name => ({ name, declaredType: 'TEXT' })),
    original: { rowId: '9223372036854775807', cells: names.map(name =>
      name === 'PlotNumber' ? cell('text', 'P') : name === 'Species' ? cell('text', '  historical overlength  ') :
      name === 'ID' ? cell('integer', '-2147483648') : name === 'Flag' ? cell('integer', '1') :
      name === column ? cell('real', 0) : cell()) } };
}
function read(wire) {
  return editor.siviDeletionOriginalFromWire(wire, owner, wire.form, wire.original.rowId);
}
function set(wire, name, value) {
  wire.original.cells[wire.columns.findIndex(column => column.name === name)] = value;
}

test('private deletion preserves every44 historical cell and separates physical from nullable logical identity', () => {
  const wire = original(), approved = read(wire);
  assert.equal(approved.original.cells.length, 44);
  assert.equal(approved.original.rowId, '9223372036854775807');
  assert.equal(approved.original.cells[43].integer, '-2147483648');
  assert.equal(editor.siviDeletionUnavailable(approved), null);
  const request = editor.siviDeletionRequest(approved, owner, token, true);
  assert.equal(request.requestId, token);
  assert.equal(request.original.cells[1].text, '  historical overlength  ');
  assert.equal(request.original.cells[42].integer, '1');
  assert.equal(Object.hasOwn(request, 'id'), false);
  request.original.cells[1].text = 'changed';
  assert.equal(approved.original.cells[1].text, '  historical overlength  ');
  wire.original.cells[1].text = 'foreign';
  assert.equal(approved.original.cells[1].text, '  historical overlength  ');
});

test('deletion membership follows original queries including hidden extended covers and literal zero', () => {
  for (const [forms, query] of [
    [['SubVegA-SIVI', 'SubVegA-SIVI_BC'], defaults.membershipQueries[0]],
    [['SubVegC-SIVI'], defaults.membershipQueries[1]],
    [['SubVegD-SIVI'], defaults.membershipQueries[2]],
  ]) {
    for (const form of forms) {
      for (const column of query.anyNonNull) {
        const wire = original(form, column);
        assert.equal(read(wire).form, form);
        set(wire, column, cell());
        assert.throws(() => read(wire), /source view/);
      }
    }
  }
  for (const form of ['SubVegA-SIVI_BC', 'SubVegC-SIVI', 'SubVegD-SIVI']) {
    assert.throws(() => read(original(form, 'Height1')), /source view/);
  }
});

test('physical schema order is retained without recasing, duplicate definitions or inferred cells', () => {
  const wire = original();
  wire.columns.reverse(); wire.original.cells.reverse();
  assert.equal(read(wire).columns[0].name, 'ID');
  for (const mutate of [
    wire => wire.columns[0].name = 'plotnumber',
    wire => wire.columns[0].name = 'ID',
    wire => wire.columns[0].declaredType = '\ud800',
    wire => wire.columns[0].extra = true,
    wire => wire.columns.pop(),
    wire => wire.original.cells.pop(),
    wire => wire.original.cells[0].foreign = 'value',
    wire => wire.original.cells[0].integer = '1',
    wire => set(wire, 'Species', cell('text', '\ud800')),
    wire => wire.original.rowId = '9223372036854775808',
  ]) {
    const bad = original(); mutate(bad);
    assert.throws(() => read(bad));
  }
});

test('source, project, context and planned physical target never cross owners', () => {
  const wire = original();
  for (const changed of [{ ...owner, contextId: 'foreign' }, { ...owner, project: 'foreign' },
    { ...owner, plot: 'foreign' }, { ...owner, plot: '\ud800' }]) {
    assert.throws(() => editor.siviDeletionOriginalFromWire(wire, changed, wire.form, wire.original.rowId));
  }
  assert.throws(() => editor.siviDeletionOriginalFromWire(wire, owner, 'SubVegC-SIVI', wire.original.rowId));
  assert.throws(() => editor.siviDeletionOriginalFromWire(wire, owner, wire.form, '1'));
  const foreign = original(); set(foreign, 'PlotNumber', cell('text', 'foreign'));
  assert.throws(() => read(foreign));
});

test('NULL/non-signed32 identity and BLOB storage stay explicitly unavailable without original repair', () => {
  for (const value of [cell(), cell('text', '1'), cell('integer', '2147483648'), cell('integer', '-2147483649')]) {
    const wire = original(); set(wire, 'ID', value);
    const approved = read(wire);
    assert.match(editor.siviDeletionUnavailable(approved), /unavailable/);
    assert.throws(() => editor.siviDeletionRequest(approved, owner, token, true), /unavailable/);
    assert.deepEqual(approved.original.cells[43], value);
  }
  const blob = original(); set(blob, 'HeightB', cell('blob', '00ff'));
  assert.match(editor.siviDeletionUnavailable(read(blob)), /BLOB/);
  assert.throws(() => editor.siviDeletionRequest(read(blob), owner, token, true), /BLOB/);
});

test('source request UUID and exact deletion acknowledgement are explicit, not implicit retries', () => {
  assert.equal(ids.validSIVIRequestId(token), true);
  for (const value of [null, '', token.toUpperCase(), token + '\n', token.replace('4aaa', '5aaa')]) {
    assert.equal(ids.validSIVIRequestId(value), false);
    assert.throws(() => editor.siviDeletionRequest(read(original()), owner, value, true), /stable request/);
  }
  for (const confirmed of [false, undefined, 'true']) {
    assert.throws(() => editor.siviDeletionRequest(read(original()), owner, token, confirmed), /Confirm deletion/);
  }
});
