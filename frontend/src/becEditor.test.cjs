const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const exportsForTest = {};
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname, 'becEditor.ts'), 'utf8'),
  { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText,
  { exports: exportsForTest });
const { BECLookup, becGroups, becWarnings, becValidation, becLengthError, becAcknowledged, rememberBECAcknowledgement } = exportsForTest;
const codes = (zone = 'BG', subZone = 'xh1', siteSeries = '01') => ({ zone, subZone, siteSeries });
const row = (rowId, siteSeries = '01', description = null) => ({
  rowId, sourceId: null, zone: 'BG', subZone: 'xh1', siteSeries, description, region: null, variant: null, phase: null
});
const ready = () => ({ zones: [{ zone: 'BG', description: 'Bunchgrass' }],
  subZones: [{ zone: 'BG', subZone: 'xh1', description: null }],
  siteSeries: [row('1')], ready: true, busy: false, error: null });
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
const serialize = value => JSON.parse(JSON.stringify(value));

test('Source-positioned BEC component compiles without warnings and retains prior SiteSeries editing when gated', () => {
  const source = readFileSync(path.join(__dirname, 'BECFields.svelte'), 'utf8');
  const result = compile(source, { filename: 'BECFields.svelte', generate: 'client' });
  assert.equal(result.warnings.length, 0);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /!editingEnabled && key !== 'siteSeries'/);
  assert.match(source, /value === '' \? null : value/);
  assert.match(source, /definition\.rowId/);
  assert.match(source, /rememberBECAcknowledgement\(draft, draft, accepted\)/);
  assert.match(source, /VITE_BEC_EDITING !== 'false'/);
  assert.doesNotMatch(source, /draft\.(subZone|siteSeries)\s*=\s*null/);
});
test('Source code bounds preserve leading zeros, raw case and Access UTF-16 lengths', () => {
  assert.equal(becLengthError(codes('bg', 'Xh1', '001')), null);
  assert.equal(becLengthError(codes('ABCD', '12345678', '12345')), null);
  assert.match(becLengthError(codes('ABCDE')), /at most 4/);
  assert.match(becLengthError(codes('BG', '123456789')), /at most 8/);
  assert.match(becLengthError(codes('BG', 'xh1', '123456')), /at most 5/);
  assert.match(becLengthError(codes('')), /NULL/);
  assert.equal(becLengthError(codes(null, null, null)), null);
  assert.equal(becLengthError(codes('😀😀', null, null)), null);
  assert.match(becLengthError(codes('😀😀😀', null, null)), /at most 4/);
});
test('Unchanged imported invalid codes do not obstruct unrelated header edits', () => {
  const legacy = codes('TOOLONG', '', '12345678');
  assert.equal(becValidation(legacy, legacy, { ...ready(), ready: false, error: 'offline' }, false), null);
  assert.equal(becLengthError({ ...legacy, siteSeries: '01' }, legacy), null);
  assert.match(becLengthError({ ...legacy, zone: 'NEWLONG' }, legacy), /at most 4/);
});
test('Duplicate code groups retain every definition, including large string identities', () => {
  const rows = [row('9007199254740993', '01', 'first'), row('2', '02', 'middle'), row('3', '01', 'last'),
    row('4', null), row('5', ''), row('6', '01', 'first')];
  const groups = becGroups(rows, item => item.siteSeries);
  assert.equal(groups.length, 2);
  assert.deepEqual(serialize(groups[0].records.map(item => item.rowId)), ['9007199254740993', '3', '6']);
  assert.equal(rows.length, 6);
  assert.equal(groups[0].code, '01');
});
test('Lookup case matching never mutates stored codes; NULL hierarchy and literal All are explicit', () => {
  const raw = codes('bg', 'XH1', '01');
  assert.equal(becWarnings(raw, ready()).length, 0);
  assert.deepEqual(raw, codes('bg', 'XH1', '01'));
  assert.match(becWarnings(codes('All'), ready())[0], /not in the catalogue/);
  assert.ok(becWarnings(codes(null), ready()).some(message => /without a zone/.test(message)));
  assert.equal(becWarnings(codes(null, null, null), ready()).length, 0);
});
test('Changed parents preserve dependent values and require acknowledgement for mismatches', () => {
  const draft = codes('AT');
  const view = { ...ready(), subZones: [], siteSeries: [] };
  assert.match(becValidation(draft, codes(), view, false), /acknowledge/);
  assert.equal(becValidation(draft, codes(), view, true), null);
  assert.deepEqual(draft, codes('AT', 'xh1', '01'));
  assert.match(becValidation(draft, codes(), { ...view, busy: true }, true), /refreshing/);
});
test('Acknowledgement survives remount of current draft, but never applies to a changed fingerprint or another plot', () => {
  const draft = codes('ZZ');
  rememberBECAcknowledgement(draft, draft, true);
  assert.equal(becAcknowledged(draft, draft), true);
  assert.equal(becAcknowledged({ ...draft }, draft), false);
  draft.subZone = 'custom';
  assert.equal(becAcknowledged(draft, draft), false);
  rememberBECAcknowledgement(draft, draft, true);
  rememberBECAcknowledgement(draft, draft, false);
  assert.equal(becAcknowledged(draft, draft), false);
});
test('Unavailable catalogue is not success-shaped validation; manual codes need explicit acknowledgement', () => {
  const unavailable = { ...ready(), ready: false, error: 'database failed' };
  assert.deepEqual(serialize(becWarnings(codes('ZZ'), unavailable)), ['database failed']);
  assert.match(becValidation(codes('ZZ'), codes(), unavailable, false), /acknowledge/);
  assert.equal(becValidation(codes('ZZ'), codes(), unavailable, true), null);
  assert.match(becValidation(codes('12345'), codes(), unavailable, true), /at most 4/);
});
test('Unselectable catalogue codes never falsely validate an entered classification', () => {
  const view = { ...ready(), siteSeries: [{ ...row('1'), selectable: false, diagnostic: 'unselectable' }] };
  assert.ok(becWarnings(codes(), view).some(message => /no matching definition/.test(message)));
  assert.match(becValidation(codes(), codes(null, null, null), view, false), /acknowledge/);
});
test('Refreshing clears old lists immediately and ignores stale lookup success and failure', async () => {
  const firstSub = deferred(), firstSeries = deferred();
  const events = [];
  const lookup = new BECLookup({ zones: async () => ready().zones,
    subZones: zone => zone === 'BG' ? firstSub.promise : Promise.resolve([]),
    siteSeries: zone => zone === 'BG' ? firstSeries.promise : Promise.resolve([]) }, state => events.push(state));
  const first = lookup.refresh('BG', 'xh1');
  assert.equal(events.at(-1).busy, true);
  assert.equal(events.at(-1).siteSeries.length, 0);
  await lookup.refresh('AT', 'un');
  firstSub.resolve(ready().subZones);
  firstSeries.reject(new Error('old request failed'));
  await first;
  assert.equal(lookup.snapshot().ready, true);
  assert.equal(lookup.snapshot().error, null);
  assert.equal(lookup.snapshot().siteSeries.length, 0);
});
test('NULL and no-match parents get empty lists, including first/last matches without a cursor bug', async () => {
  const requests = [];
  const lookup = new BECLookup({ zones: async () => ready().zones,
    subZones: async zone => { requests.push(zone); return zone === 'BG' ? ready().subZones : []; },
    siteSeries: async (zone, subZone) => zone === 'BG' && subZone === 'xh1' ? [row('first'), row('last', '02')] : [] }, () => {});
  await lookup.refresh('BG', 'xh1');
  assert.deepEqual(serialize(lookup.snapshot().siteSeries.map(item => item.rowId)), ['first', 'last']);
  await lookup.refresh(null, null);
  assert.equal(lookup.snapshot().siteSeries.length, 0);
  assert.equal(requests.at(-1), null);
  await lookup.refresh('ZZ', 'custom');
  assert.equal(lookup.snapshot().siteSeries.length, 0);
});
test('Explicit service failure, invalid arrays, retry and disposal do not leave phantom ready state', async () => {
  let calls = 0;
  const events = [];
  const lookup = new BECLookup({ zones: async () => { if (++calls === 1) throw new Error('locked catalogue'); return []; },
    subZones: async () => [], siteSeries: async () => [] }, state => events.push(state));
  await lookup.refresh(null, null);
  assert.match(lookup.snapshot().error, /locked catalogue/);
  assert.equal(lookup.snapshot().ready, false);
  await lookup.refresh(null, null);
  assert.equal(lookup.snapshot().ready, true);
  lookup.dispose();
  const count = events.length;
  await lookup.refresh('BG', 'xh1');
  assert.equal(events.length, count);
  const broken = new BECLookup({ zones: async () => null, subZones: async () => [], siteSeries: async () => [] }, () => {});
  await broken.refresh(null, null);
  assert.match(broken.snapshot().error, /catalogue arrays/);
});
