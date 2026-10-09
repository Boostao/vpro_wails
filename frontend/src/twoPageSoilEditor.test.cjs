const assert = require('node:assert/strict');
const { test } = require('node:test');
const client = require('svelte/internal/client');
const { editor, fixture, original, cell } = require('./twoPageSoilTestHelpers.cjs');
const plain = value => JSON.parse(JSON.stringify(value));
const owner = fixture.owner;
function drafts(source = original()) {
  return {
    horizon: editor.stageTwoPageSoil(source, '-7', 'Horizon', 'LF', false),
    depth: editor.stageTwoPageSoil(source, '-7', 'UpperDepth', '5', false),
    comment: editor.stageTwoPageSoil(source, '-7', 'Comment', '', true),
  };
}
function oneField(form, column) {
  const source = original();
  source.form = form;
  const kind = form === 'SoilHumusXL' ? 'Humus' : 'Mineral';
  source.recordSource = `Usys${kind}`; source.table = `Sample_${kind}`;
  source.records.columns = [{ name: 'ID', declaredType: '' }, { name: 'PlotNumber', declaredType: 'TEXT' },
    { name: column, declaredType: '' }, { name: 'Unmapped', declaredType: 'BLOB' }];
  source.records.rows = [{ rowId: '-7', cells: [cell('integer', '0'), cell('text', owner.plot), cell(), cell('blob', 'ff00')] }];
  return source;
}

test('soil client constructs the shared strict-Go wire and detaches real Svelte proxy originals/drafts', () => {
  const source = client.proxy(original());
  const staged = client.proxy(drafts(source));
  const request = editor.twoPageSoilRequest(source, owner, staged, fixture.request.requestId);
  assert.deepEqual(plain(request), fixture.request);
  assert.doesNotThrow(() => structuredClone(request));
  source.records.rows[0].cells[1].text = 'changed original';
  staged.horizon.value.text = 'changed draft';
  assert.equal(request.original.records.rows[0].cells[1].text, owner.plot);
  assert.equal(request.edits[0].value.text, 'LF');
  assert.equal(request.original.records.rows[1].rowId, '9223372036854775807');
  assert.equal(request.original.records.rows[1].cells[0].storage, 'null');
});

test('soil raw originals require exact owner/schema/parent/canonical tags without repairing historical values', () => {
  assert.deepEqual(plain(editor.twoPageSoilOriginalFromWire(client.proxy(original()), owner)), fixture.request.original);
  for (const mutate of [
    value => value.contextId += '-stale',
    value => value.plot += ' ',
    value => value.form = 'SoilMineralXL',
    value => value.recordSource = 'UsysMineral',
    value => value.table = 'Other_Humus',
    value => value.records.columns[0] = { name: 'ID', type: '' },
    value => value.records.columns[2].name = 'ID',
    value => value.records.rows[0].rowId = '-07',
    value => value.records.rows[0].cells.pop(),
    value => value.records.rows[1].rowId = '-7',
    value => value.records.rows[0].cells[1] = cell('integer', '108050'),
    value => value.records.rows[0].cells[1].text = '0108050 ',
    value => value.records.rows[0].cells[2] = { storage: 'null' },
    value => value.records.rows[0].cells[5].blobHex = 'FF00',
    value => value.records.rows[0].cells[4].text = '\ud800',
    value => value.records.extra = true,
  ]) {
    const source = original(); mutate(source);
    assert.throws(() => editor.twoPageSoilOriginalFromWire(source, owner));
  }
});

test('signed32 edit eligibility preserves signed64 physical rowids and never invents NULL ID0', () => {
  for (const id of ['-2147483648', '-1', '0', '2147483647']) {
    const source = original(); source.records.rows[0].cells[0] = cell('integer', id);
    assert.equal(editor.twoPageSoilEditableID(source, '-7'), Number(id));
  }
  for (const identity of [cell(), cell('text', '0'), cell('real', 0), cell('integer', '2147483648'), cell('integer', '00')]) {
    const source = original(); source.records.rows[0].cells[0] = identity;
    assert.throws(() => editor.twoPageSoilEditableID(source, '-7'));
  }
  const source = original(); source.records.rows[1].cells[0] = source.records.rows[0].cells[0];
  assert.throws(() => editor.twoPageSoilEditableID(source, '-7'));
  assert.throws(() => editor.stageTwoPageSoil(original(), '9223372036854775807', 'Horizon', 'LF', false));
  assert.throws(() => editor.stageTwoPageSoil(original(), '-7', 'ID', '1', false));
  assert.throws(() => editor.stageTwoPageSoil(original(), '-7', 'Unmapped', 'ff', false));
});

test('shared Go peer witnesses refuse numeric aliases without JavaScript-only coercions', () => {
  for (const peer of fixture.peerIDs) {
    const source = original();
    source.records.rows[0].cells[0] = cell('integer', peer.id);
    const selectedDrafts = drafts(source);
    source.records.rows[1].cells[0] = cell(peer.storage, peer.value);
    const label = `${peer.id}/${peer.storage}/${JSON.stringify(peer.value)}`;
    if (peer.collides) {
      assert.throws(() => editor.twoPageSoilEditableID(source, '-7'), /multiple/, label);
      assert.throws(() => editor.twoPageSoilRequest(source, owner, selectedDrafts, fixture.request.requestId), /multiple/, label);
    } else {
      assert.equal(editor.twoPageSoilEditableID(source, '-7'), Number(peer.id), label);
      assert.equal(editor.twoPageSoilRequest(source, owner, selectedDrafts, fixture.request.requestId).edits.length, 3, label);
    }
  }
});

const sourceFields = {
  SoilHumusXL: {
    Horizon: ['text', 8], UpperDepth: ['real'], LowerDepth: ['real'], HumusFormpH: ['real'],
    Comment: ['text', 0], HumusStructureDegree: ['text', 1], HumusStructureKind: ['text', 5],
    MycelAbundance: ['text', 1], FecalAbundance: ['text', 1], RootsAbundance: ['text', 6],
    RootsSize: ['text', 6], vonPost: ['integer'],
  },
  SoilMineralXL: {
    Horizon: ['text', 8], UpperDepth: ['real'], LowerDepth: ['real'], Texture: ['text', 4],
    Colour: ['text', 14], Comments: ['text', 0], PitDepthLimit: ['text', 1], ASP: ['integer'],
    PercentCoarseFragsGravel: ['integer'], PercentCoarseFragsCobbles: ['integer'],
    PercentCoarseFragsStones: ['integer'], PercentCoarseFragsTotal: ['integer'],
    PercentCoarseFragsShape: ['text', 1], RootsAbundance: ['text', 6], RootsSize: ['text', 6],
    MineralStructureClass: ['text', 3], MineralStructureKind: ['text', 7], MineralFormpH: ['real'],
  },
};
for (const [form, fields] of Object.entries(sourceFields)) {
  test(`${form} all exact source fields preserve nullable domains, UTF16 bounds and unchanged historical omissions`, () => {
    for (const [column, [storage, maximum]] of Object.entries(fields)) {
      const source = oneField(form, column);
      const raw = storage === 'text' ? 'X' : storage === 'integer' ? '-32768' : '-1.25';
      const draft = editor.stageTwoPageSoil(source, '-7', column, raw, false);
      assert.equal(draft.error, null, column);
      assert.deepEqual(plain(draft.value), cell(storage, storage === 'real' ? -1.25 : raw));
      for (const invalid of storage === 'text'
        ? ['\ud800', ...(maximum ? ['x'.repeat(maximum + 1), '😀'.repeat(Math.floor(maximum / 2) + 1)] : [])]
        : storage === 'integer' ? ['32768', '-32769', '01', '-0', '1.5', ' 1 ', '9'.repeat(1000)]
        : ['NaN', 'Infinity', '3.402823466385289e38', '-1e100', ' 1 ', '']) {
        const bad = editor.stageTwoPageSoil(source, '-7', column, invalid, false);
        assert.notEqual(bad.error, null, `${column}=${invalid}`);
        assert.equal(bad.raw, invalid);
        assert.throws(() => editor.twoPageSoilRequest(source, { ...owner, form }, { bad }, fixture.request.requestId));
        assert.equal(editor.stageTwoPageSoil(source, '-7', column, raw, false).error, null);
      }
      const nil = editor.stageTwoPageSoil(source, '-7', column, '', true);
      assert.deepEqual(plain(nil.value), cell());
      if (storage === 'text') {
        const empty = editor.stageTwoPageSoil(source, '-7', column, '', false);
        assert.deepEqual(plain(empty.value), cell('text', ''));
        assert.equal(empty.error, null);
      }
      source.records.rows[0].cells[2] = cell('text', 'historical invalid type and overlength');
      const historical = editor.stageTwoPageSoil(source, '-7', column, 'historical invalid type and overlength', false);
      assert.equal(historical.error, null);
      assert.throws(() => editor.twoPageSoilRequest(source, { ...owner, form }, { historical }, fixture.request.requestId), /changed/);
      const correction = editor.stageTwoPageSoil(source, '-7', column, raw, false);
      assert.equal(correction.error, null);
      const request = editor.twoPageSoilRequest(source, { ...owner, form }, { correction }, fixture.request.requestId);
      assert.equal(request.edits.length, 1);
      assert.equal(request.edits[0].expected.text, 'historical invalid type and overlength');
    }
  });
}

test('draft ownership, duplicate assignments and raw/value inconsistencies cannot produce a Save request', () => {
  for (const mutate of [
    value => value.horizon.contextId += '-other',
    value => value.horizon.form = 'SoilMineralXL',
    value => value.horizon.expected = cell('text', 'invented'),
    value => value.horizon.value.text = 'inconsistent',
    value => value.horizon.raw = 'inconsistent',
    value => value.horizon.error = 'retained invalid draft',
    value => value.duplicate = { ...value.horizon },
  ]) {
    const staged = drafts(); mutate(staged);
    assert.throws(() => editor.twoPageSoilRequest(original(), owner, staged, fixture.request.requestId));
  }
  assert.throws(() => editor.twoPageSoilRequest(original(), owner, drafts(), 'not-a-uuid'));
  assert.throws(() => editor.twoPageSoilRequest(original(), owner, {}, fixture.request.requestId));
});
