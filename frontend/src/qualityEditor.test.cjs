const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
function load(name, dependencies = {}) {
  const exports = {};
  vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, name), 'utf8'),
    { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText,
  { exports, require: name => {
    if (!(name in dependencies)) throw new Error(`Unexpected test dependency ${name}`);
    return dependencies[name];
  } });
  return exports;
}
const { qualityKeys, qualityKey, qualityLengthError, wellFormedUTF16, qualityValidation,
  qualityGroups, qualityDefinitions, qualityWarnings, qualityAcknowledged,
  rememberQualityAcknowledgement, QualityLookup } = load('qualityEditor.ts', { './becEditor': load('becEditor.ts') });
const plain = value => JSON.parse(JSON.stringify(value));
const codes = value => ({ sitePlotQuality: value, vegPlotQuality: null, soilPlotQuality: null });
const row = (rowId, code = 'Good') => ({ rowId, code, listName: 'PlotQualitySite', listFilter: null,
  itemOrder: 3, description: 'Native description', fieldUsedIn: null, validateLoops: null,
  validate: false, note: 'G', flag: true, selectable: code !== null && code !== '', diagnostic: '' });
const ready = () => ({ choices: [row('1', 'NA'), row('2', 'Excellent'), row('3'), row('4', 'Fair'), row('5', 'Poor')],
  ready: true, busy: false, error: null });
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };

test('Native-verified quality source renderer compiles, preserves lexical input, and retains explicit opt-out', () => {
  const source = readFileSync(path.join(__dirname, 'QualityFields.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'QualityFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_QUALITY_EDITING !== 'false'/);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /data-column=\{control.column\}/);
  assert.match(source, /oninput=\{event => setCode/);
  assert.doesNotMatch(source, /maxlength|\.slice\(|\.substring\(|\.trim\(|PlotService|UpdatePlot|CreatePlot/);
  const parent = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  assert.match(parent, /qualityKey\(control.column\)/);
  assert.match(parent, /@render qualityInputs/);
  assert.match(parent, /onbusy=\{onQualityBusyChange\}/);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /coordinateBusy \|\| workingUnitBusy \|\| qualityBusy/);
  assert.match(form, /onclick=\{toggleLock\}\s+disabled=\{busy \|\| headerWorkflowBusy \|\| !capabilitiesReady \|\| siviStandalone &&/);
});
test('All three raw nullable Admin columns have15 UTF-16 bounds without silent truncation', () => {
  assert.deepEqual(plain(qualityKeys), ['sitePlotQuality', 'vegPlotQuality', 'soilPlotQuality']);
  for (const [column, key] of [['SitePlotQuality', 'sitePlotQuality'], ['VegPlotQuality', 'vegPlotQuality'], ['SoilPlotQuality', 'soilPlotQuality']]) {
    assert.equal(qualityKey(column), key);
    const original = codes(null);
    for (const value of [null, 'NA', 'good', "O'Q;01", ' x ', 'x'.repeat(15), 'x'.repeat(13) + '😀']) {
      assert.equal(qualityLengthError({ ...original, [key]: value }), null);
    }
    for (const value of ['x'.repeat(16), 'x'.repeat(14) + '😀']) {
      assert.match(qualityLengthError({ ...original, [key]: value }), /15 UTF-16.*not been truncated/);
    }
    assert.match(qualityLengthError({ ...original, [key]: '' }), /NULL/);
  }
  assert.equal(qualityKey('Flag'), undefined);
});
test('Malformed UTF-16 is explicitly rejected, never turned into a replacement glyph', () => {
  for (const value of ['\ud800', '\udfff', 'x'.repeat(14) + '\ud83d', '\ud800A', '\ud800\ud800', '\udfff\ud800']) {
    assert.equal(wellFormedUTF16(value), false);
    assert.match(qualityLengthError(codes(value)), /incomplete Unicode/);
    assert.match(qualityValidation(codes(value), codes(null), ready(), true), /incomplete Unicode/);
  }
  for (const value of ['', 'abc', '\\ud800', '\ufffd', '😀', 'x'.repeat(13) + '😀']) assert.equal(wellFormedUTF16(value), true);
});
test('NA and full raw Item codes bind; note letters and descriptions are only metadata', () => {
  assert.equal(qualityValidation(codes('NA'), codes(null), ready(), false), null);
  assert.equal(qualityValidation(codes('good'), codes('Good'), ready(), false), null);
  assert.match(qualityValidation(codes('G'), codes(null), ready(), false), /acknowledge/);
  assert.equal(qualityValidation(codes('G'), codes(null), ready(), true), null);
  assert.equal(qualityWarnings(codes('G'), codes(null), ready()).length, 1);
  assert.equal(qualityDefinitions(ready().choices, 'g').length, 0);
  assert.equal(qualityDefinitions(ready().choices, 'GOOD').length, 1);
  assert.equal(qualityDefinitions([row('a', 'ä')], 'Ä').length, 0);
});
test('Every duplicate raw-code definition remains visible without making NULL/empty/invalid rows choices', () => {
  const rows = [row('1'), row('2'), row('3', 'good'), row('4', null), row('5', ''), row('6', 'x'.repeat(16)),
    row('7', '\ud800')];
  const groups = qualityGroups(rows);
  assert.deepEqual(plain(groups.map(group => [group.code, group.records.length])), [['Good', 2], ['good', 1]]);
  assert.equal(qualityDefinitions(rows, 'GOOD').length, 3);
  assert.equal(rows.length, 7);
  assert.equal(groups[0].records[0].note, 'G');
  assert.equal(groups[0].records[0].flag, true);
  assert.equal(groups[0].records[0].validate, false);
});
test('Historical unknown/overlength values stay clean on unrelated edits; new manual entries require acknowledgement', () => {
  for (const value of ['historical_unknown', 'x'.repeat(101), '\ud800']) {
    const historical = codes(value);
    assert.equal(qualityLengthError(historical, historical), null);
    assert.equal(qualityValidation(historical, historical, { ...ready(), ready: false, error: 'offline' }, false), null);
    assert.equal(qualityValidation(codes(null), historical, ready(), false), null);
  }
  assert.match(qualityValidation(codes("O'Q"), codes(null), ready(), false), /acknowledge/);
  assert.equal(qualityValidation(codes("O'Q"), codes(null), ready(), true), null);
  assert.match(qualityValidation(codes('Good'), codes(null), { ...ready(), ready: false, error: 'offline' }, false), /acknowledge/);
  assert.match(qualityValidation(codes('Good'), codes(null), { ...ready(), busy: true }, true), /finish refreshing/);
  assert.equal(qualityValidation(codes(null), codes('Good'), { ...ready(), ready: false, error: 'offline' }, false), null);
});
test('Acknowledgement survives the same draft remount but neither raw-code change nor a different loaded plot', () => {
  const draft = codes('custom');
  rememberQualityAcknowledgement(draft, draft, true);
  assert.equal(qualityAcknowledged(draft, draft), true);
  assert.equal(qualityAcknowledged({ ...draft }, draft), false);
  assert.equal(qualityAcknowledged(draft, { ...draft, vegPlotQuality: 'custom' }), false);
  assert.equal(qualityAcknowledged(draft, codes('CUSTOM')), false);
  rememberQualityAcknowledgement(draft, draft, false);
  assert.equal(qualityAcknowledged(draft, draft), false);
});
test('Lookup publishes real failure/retry, ignores late stale success and releases disposed busy callbacks', async () => {
  const first = deferred(), second = deferred();
  const requests = [first, second];
  const states = [];
  const lookup = new QualityLookup(() => requests.shift().promise, view => states.push(view));
  const old = lookup.refresh(), current = lookup.refresh();
  second.resolve(ready().choices); await current;
  first.resolve([row('stale', 'old')]); await old;
  assert.deepEqual(plain(lookup.snapshot().choices.map(row => row.code)), ['NA', 'Excellent', 'Good', 'Fair', 'Poor']);
  const failure = new QualityLookup(async () => { throw new Error('read-only catalogue failure'); }, view => states.push(view));
  await failure.refresh();
  assert.equal(failure.snapshot().ready, false);
  assert.equal(failure.snapshot().busy, false);
  assert.match(failure.snapshot().error, /read-only catalogue failure/);
  const disposed = new QualityLookup(() => first.promise, view => states.push(view));
  disposed.dispose();
  const count = states.length;
  await disposed.refresh();
  assert.equal(states.length, count);
  const malformed = new QualityLookup(async () => null, () => {});
  await malformed.refresh();
  assert.match(malformed.snapshot().error, /did not return/);
});

test('explicit catalogue Retry forces verification while normal refresh remains warm', async () => {
  const calls = [];
  const lookup = new QualityLookup(async force => { calls.push(force); return [row('1')]; }, () => {});
  await lookup.refresh();
  await lookup.refresh(true);
  assert.deepEqual(calls, [false, true]);
  assert.equal(lookup.snapshot().ready, true);
  for (const [file, service] of [
    ['ParentCodeFields.svelte', 'ParentCodeService'],
    ['GeologyCodeFields.svelte', 'GeologyCodeService'],
    ['SoilCodeFields.svelte', 'SoilCodeService'],
    ['SiteCodeFields.svelte', 'SiteCodeService'],
    ['RegionCodeFields.svelte', 'RegionCodeService'],
    ['SoilCodeReference.svelte', 'SoilCodeService']
  ]) {
    const source = readFileSync(path.join(__dirname, file), 'utf8');
    assert.match(source, new RegExp(`if \\(force\\) await requests\\.track\\(${service}\\.ReloadCatalogue\\(\\)\\)`));
    assert.match(source, /refresh\(true\)/);
    assert.equal(compile(source, { filename: file, generate: 'client' }).warnings.length, 0);
  }
});

test('failed forced revalidation clears choices and stays unavailable until valid retry', async () => {
  let broken = false;
  const lookup = new QualityLookup(async force => {
    if (force && broken) throw new Error('checksum failure');
    return [row('1')];
  }, () => {});
  await lookup.refresh();
  broken = true;
  await lookup.refresh(true);
  assert.equal(lookup.snapshot().ready, false);
  assert.equal(lookup.snapshot().choices.length, 0);
  assert.match(lookup.snapshot().error, /checksum failure/);
  broken = false;
  await lookup.refresh(true);
  assert.equal(lookup.snapshot().ready, true);
  assert.equal(lookup.snapshot().choices.length, 1);
});
