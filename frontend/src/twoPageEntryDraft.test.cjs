const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { metadata } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const loaded = loadTypeScript('twoPageEntryDraft.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './ordinaryEditor': ordinary,
  './coordinateEditor': loadTypeScript('coordinateEditor.ts'),
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
});
const api = { parseTwoPageEntryValue: (...args) => structuredClone(loaded.parseTwoPageEntryValue(...args)) };
const policy = (kind, maximum = 0) => ({ column: 'OwnedField', owner: 'Env', kind, maximum });
const text = raw => ({ kind: 'text', raw });
const cell = (storage = 'null', value = null) => ({ storage, text: storage === 'text' ? value : null,
  integer: storage === 'integer' ? value : null, real: storage === 'real' ? value : null,
  blobHex: storage === 'blob' ? value : null });

test('entry parser retains literal text and exact UTF-16 thresholds without repair', () => {
  for (const kind of ['text', 'categorical', 'plot-type']) {
    const p = policy(kind, 4);
    assert.deepEqual(api.parseTwoPageEntryValue(p, cell(), text(' a  ')), { value: cell('text', ' a  '), error: null });
    assert.equal(api.parseTwoPageEntryValue(p, cell(), text('😀😀')).error, null);
    assert.match(api.parseTwoPageEntryValue(p, cell(), text('😀😀x')).error, /at most 4 UTF-16/);
    for (const bad of ['\ud800', '\udfff', 'a\0b']) {
      assert.match(api.parseTwoPageEntryValue(p, cell(), text(bad)).error, /Unicode or NUL/);
    }
    const empty = api.parseTwoPageEntryValue(p, cell(), text(''));
    assert.equal(empty.error === null, kind === 'categorical');
    assert.equal(empty.value.storage, 'text');
    assert.deepEqual(api.parseTwoPageEntryValue(p, cell('text', 'old'), { kind: 'clear' }),
      { value: cell(), error: null });
  }
  assert.equal(api.parseTwoPageEntryValue(policy('text'), cell(), text('x'.repeat(2000))).error, null);
  assert.match(api.parseTwoPageEntryValue(policy('text'), cell(), text('')).error, /NULL/);
});

test('entry parser preserves unchanged historical invalid storage and does not alias it', () => {
  for (const [p, original] of [[policy('text', 2), cell('text', 'historical long')],
    [policy('integer'), cell('integer', '9007199254740993')], [policy('single'), cell('real', 1e40)],
    [policy('date'), cell('text', 'invalid historical date')], [policy('categorical', 3), cell('text', '')]]) {
    const raw = metadata.metadataCellText(original);
    const result = api.parseTwoPageEntryValue(p, original, text(raw));
    assert.equal(result.error, null);
    assert.deepEqual(result.value, original);
    assert.notEqual(loaded.parseTwoPageEntryValue(p, original, text(raw)).value, original);
    assert.deepEqual(api.parseTwoPageEntryValue(p, original, { kind: 'original' }), { value: original, error: null });
  }
  const blob = cell('blob', 'FF');
  assert.deepEqual(api.parseTwoPageEntryValue(policy('text', 2), blob, { kind: 'original' }), { value: blob, error: null });
  assert.match(api.parseTwoPageEntryValue(policy('text', 2), blob, text('x')).error, /BLOB replacement/);
  assert.match(api.parseTwoPageEntryValue(policy('text', 2), blob, { kind: 'clear' }).error, /BLOB replacement/);
});

test('entry numeric parser obeys Single, Access INTEGER and geographic domains', () => {
  const expectations = [
    ['single', '0', cell('real', 0)], ['single', '-3.25', cell('real', -3.25)],
    ['integer', '-32768', cell('integer', '-32768')], ['integer', '32767', cell('integer', '32767')],
    ['latitude', '-90', cell('real', -90)], ['longitude', '180', cell('real', 180)],
  ];
  for (const [kind, raw, value] of expectations) {
    assert.deepEqual(api.parseTwoPageEntryValue(policy(kind), cell(), text(raw)), { value, error: null });
  }
  for (const [kind, raw] of [['single', '1e39'], ['single', 'NaN'], ['single', '1x'],
    ['integer', '32768'], ['integer', '-32769'], ['integer', '0.5'],
    ['latitude', '90.0001'], ['longitude', '-180.0001']]) {
    assert.ok(api.parseTwoPageEntryValue(policy(kind), cell(), text(raw)).error, `${kind}:${raw}`);
  }
  for (const kind of ['single', 'integer', 'latitude', 'longitude']) {
    assert.deepEqual(api.parseTwoPageEntryValue(policy(kind), cell('real', 2), { kind: 'clear' }), { value: cell(), error: null });
  }
});

test('entry Boolean and Est/Meas inputs preserve exact Access storage and source option text', () => {
  for (const value of [false, true]) {
    assert.deepEqual(api.parseTwoPageEntryValue(policy('boolean'), cell(), { kind: 'boolean', value }),
      { value: cell('integer', value ? '-1' : '0'), error: null });
  }
  for (const option of [null, 1, 2]) {
    assert.deepEqual(api.parseTwoPageEntryValue(policy('option'), cell(), { kind: 'option', option }),
      { value: option === null ? cell() : cell('text', String(option)), error: null });
  }
  for (const [kind, input] of [['option', { kind: 'option', option: 0 }],
    ['option', { kind: 'boolean', value: true }], ['boolean', { kind: 'option', option: 1 }],
    ['boolean', { kind: 'boolean', value: 'false' }], ['boolean', { kind: 'boolean', value: null }],
    ['boolean', text('-1')], ['option', text('1')]]) {
    assert.ok(api.parseTwoPageEntryValue(policy(kind), cell(), input).error, kind);
  }
});

test('entry timestamp parser rejects repaired dates, timezones and unsupported policies explicitly', () => {
  for (const raw of ['2020-02-29 12:13:14', '2020-02-29 12:13:14.125']) {
    assert.deepEqual(api.parseTwoPageEntryValue(policy('date'), cell(), text(raw)), { value: cell('text', raw), error: null });
  }
  for (const raw of ['2021-02-29 12:13:14', '2020-02-29', '2020-02-29 12:13:14Z', '']) {
    assert.ok(api.parseTwoPageEntryValue(policy('date'), cell(), text(raw)).error, raw);
  }
  assert.throws(() => api.parseTwoPageEntryValue(policy('unknown'), cell(), text('x')), /unavailable parser policy/);
  assert.throws(() => api.parseTwoPageEntryValue(policy('text', -1), cell(), text('x')), /unavailable parser policy/);
});
