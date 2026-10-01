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
const bec = load('becEditor.ts');
const quality = load('qualityEditor.ts', { './becEditor': bec });
const reference = load('referenceCodeEditor.ts', { './becEditor': bec, './qualityEditor': quality });
const { siteCodeKeys, siteCodeKey, siteCodeValueError, siteCodeFieldError, siteCodeLengthError, siteCodeValidation,
  siteCodeGroups, siteCodeDefinitions, siteCodeSuggestions, siteCodeWarnings, siteCodeAcknowledged,
  rememberSiteCodeAcknowledgement } = load('siteCodeEditor.ts', { './referenceCodeEditor': reference });
const plain = value => JSON.parse(JSON.stringify(value));
const codes = changes => ({ siteDisturbance1: null, siteDisturbance2: null, siteDisturbance3: null,
  exposure1: null, exposure2: null, ...changes });
const row = (rowId, listName, code, description = 'Measured definition') => ({
  rowId, code, listName, listFilter: null, itemOrder: 1, description, fieldUsedIn: 'Exposure',
  validateLoops: null, validate: false, note: null, flag: true, selectable: code !== null && code !== '', diagnostic: ''
});
const ready = () => ({
  SiteDisturbance: { choices: [row('1', 'SiteDisturbance', 'A'), row('2', 'SiteDisturbance', 'M.f', 'forest'),
    row('3', 'SiteDisturbance', 'M.f', 'fire'), row('4', 'SiteDisturbance', 'M.s'), row('5', 'SiteDisturbance', 'M.s')],
  ready: true, busy: false, error: null },
  Exposure: { choices: [row('128', 'Exposure', 'AT'), row('129', 'Exposure', 'CA'),
    row('130', 'Exposure', 'X'), row('131', 'Exposure', 'NA')],
    ready: true, busy: false, error: null }
});
test('Native-verified five source slots compile with explicit opt-out and no implicit rewriting or direct writes', () => {
  const source = readFileSync(path.join(__dirname, 'SiteCodeFields.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'SiteCodeFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_SITE_CODES_EDITING !== 'false'/);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /data-source-control=\{control.controlName\}/);
  assert.match(source, /ListSiteDisturbanceChoices/);
  assert.match(source, /ListExposureChoices/);
  assert.match(source, /Use \{suggestion.code\}/);
  assert.doesNotMatch(source, /maxlength|\.slice\(|\.substring\(|\.trim\(|\.toUpperCase\(|PlotService|UpdatePlot|CreatePlot/);
  const parent = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  assert.match(parent, /siteCodeKey\(control.column\)/);
  assert.match(parent, /@render siteCodeInputs/);
  assert.match(parent, /onbusy=\{onSiteCodeBusyChange\}/);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /coordinateBusy \|\| workingUnitBusy \|\| qualityBusy \|\| siteCodeBusy/);
  assert.match(form, /onSiteCodeBusyChange=\{\(pending\) => siteCodeBusy = pending\}/);
});
test('All five nullable text fields use exact measured8/2 UTF-16 bounds and reject malformed new input', () => {
  assert.deepEqual(plain(siteCodeKeys), Object.keys(codes()));
  for (const [column, key, limit] of [['SiteDisturbance1', 'siteDisturbance1', 8], ['SiteDisturbance2', 'siteDisturbance2', 8],
    ['SiteDisturbance3', 'siteDisturbance3', 8], ['Exposure1', 'exposure1', 2], ['Exposure2', 'exposure2', 2]]) {
    assert.equal(siteCodeKey(column), key);
    for (const value of [null, 'x'.repeat(limit), 'x'.repeat(limit - 2) + '😀', "'"]) {
      assert.equal(siteCodeLengthError(codes({ [key]: value }), codes()), null);
    }
    for (const value of ['x'.repeat(limit + 1), 'x'.repeat(limit - 1) + '😀']) {
      assert.match(siteCodeLengthError(codes({ [key]: value }), codes()), /UTF-16.*not been truncated/);
    }
    for (const value of ['\ud800', '\udfff', '\ud800A', '\ud800\ud800', '\udfff\ud800']) {
      assert.match(siteCodeValueError(key, value, null), /incomplete Unicode/);
    }
    assert.match(siteCodeValueError(key, '', null), /NULL/);
  }
  assert.equal(siteCodeKey('SiteNotes'), undefined);
});
test('Only exact canonical Exposure Item codes can save; acknowledgement cannot bypass membership', () => {
  for (const key of ['exposure1', 'exposure2']) {
    for (const value of ['AT', 'CA', 'X', 'NA', null]) assert.equal(siteCodeValidation(codes({ [key]: value }), codes(), ready(), false), null);
    for (const value of ['at', 'At', 'A', 'zz', 'CO', '  ', '😀']) {
      assert.match(siteCodeValidation(codes({ [key]: value }), codes(), ready(), true), /exact Item/);
      assert.match(siteCodeFieldError(key, value, null, ready()), /exact Item/);
    }
    assert.match(siteCodeValidation(codes({ [key]: 'Measured definition' }), codes(), ready(), true), /2 UTF-16/);
  }
  assert.deepEqual(plain(siteCodeSuggestions(ready().Exposure.choices, 'exposure1', 'at').map(group => group.code)), ['AT']);
  assert.deepEqual(plain(siteCodeSuggestions(ready().Exposure.choices, 'exposure2', 'A').map(group => group.code)), ['AT']);
  assert.equal(siteCodeSuggestions(ready().Exposure.choices, 'exposure1', 'zz').length, 0);
});
test('Manual disturbance raw bytes are preserved; unknown values need explicit exact-draft acknowledgement', () => {
  for (const key of ['siteDisturbance1', 'siteDisturbance2', 'siteDisturbance3']) {
    for (const value of ["O'Q;01", ' x ', 'unknown', '😀😀😀😀']) {
      const draft = codes({ [key]: value });
      assert.match(siteCodeValidation(draft, codes(), ready(), false), /acknowledge/);
      assert.equal(siteCodeValidation(draft, codes(), ready(), true), null);
      assert.equal(draft[key], value);
    }
    assert.equal(siteCodeValidation(codes({ [key]: 'a' }), codes({ [key]: 'A' }), ready(), false), null);
  }
  const draft = codes({ siteDisturbance1: "O'Q" });
  rememberSiteCodeAcknowledgement(draft, draft, true);
  assert.equal(siteCodeAcknowledged(draft, draft), true);
  assert.equal(siteCodeAcknowledged({ ...draft }, draft), false);
  assert.equal(siteCodeAcknowledged(draft, codes({ siteDisturbance1: "O'q" })), false);
  rememberSiteCodeAcknowledgement(draft, draft, false);
  assert.equal(siteCodeAcknowledged(draft, draft), false);
});
test('Duplicate M.f/M.s definitions retain metadata/order without making NULL/blank/wrong-list rows selectable', () => {
  const views = ready(), rows = [...views.SiteDisturbance.choices, row('6', 'SiteDisturbance', null),
    row('7', 'SiteDisturbance', ''), row('8', 'Exposure', 'AT'), row('9', 'SiteDisturbance', 'toolongcode'),
    row('10', 'SiteDisturbance', '\ud800')];
  assert.deepEqual(plain(siteCodeGroups(rows, 'SiteDisturbance').map(group => [group.code, group.records.length])),
    [['A', 1], ['M.f', 2], ['M.s', 2]]);
  const definitions = siteCodeDefinitions(rows, 'SiteDisturbance', 'm.F');
  assert.deepEqual(plain(definitions.map(item => [item.rowId, item.description, item.note, item.validate, item.flag])),
    [['2', 'forest', null, false, true], ['3', 'fire', null, false, true]]);
  assert.equal(siteCodeDefinitions(rows, 'Exposure', 'AT').length, 1);
  assert.equal(siteCodeDefinitions(rows, 'SiteDisturbance', 'Measured definition').length, 0);
  assert.equal(rows.length, 10);
});
test('Historical unknown/overlength values survive unrelated writes; clears and repeated slots remain valid', () => {
  const original = codes({ exposure1: 'zz', exposure2: 'historical-long', siteDisturbance1: 'historical-disturbance' });
  const offline = ready();
  offline.Exposure = { ...offline.Exposure, choices: [], ready: false, error: 'offline exposure' };
  assert.equal(siteCodeValidation(original, original, offline, false), null);
  assert.equal(siteCodeValidation({ ...original, siteDisturbance2: 'A' }, original, offline, false), null);
  assert.equal(siteCodeValidation(codes(), original, offline, false), null);
  assert.equal(siteCodeValidation(codes({ exposure1: 'AT', exposure2: 'AT', siteDisturbance1: 'M.f',
    siteDisturbance2: 'M.f', siteDisturbance3: 'M.f' }), codes(), ready(), false), null);
  assert.match(siteCodeValidation({ ...original, exposure1: 'AT' }, original, offline, true), /offline exposure/);
});
test('Exposure failures are fail-closed; disturbance failure permits only explicit raw acknowledgement', () => {
  const views = ready();
  views.Exposure = { ...views.Exposure, choices: [], ready: false, error: 'broken exposure checksum' };
  assert.match(siteCodeValidation(codes({ exposure1: 'AT' }), codes(), views, true), /broken exposure checksum/);
  views.SiteDisturbance = { ...views.SiteDisturbance, choices: [], ready: false, error: 'broken disturbance checksum' };
  assert.equal(siteCodeWarnings(codes({ siteDisturbance1: 'A' }), codes(), views)[0], 'broken disturbance checksum');
  assert.match(siteCodeValidation(codes({ siteDisturbance1: 'A' }), codes(), views, false), /acknowledge/);
  assert.equal(siteCodeValidation(codes({ siteDisturbance1: 'A' }), codes(), views, true), null);
  views.SiteDisturbance.busy = true;
  assert.match(siteCodeValidation(codes({ siteDisturbance1: 'A' }), codes(), views, true), /finish refreshing/);
});
test('Shared list lookup reports its domain explicitly and retains successful retry semantics', async () => {
  let fail = true;
  const states = [];
  const lookup = new quality.QualityLookup(async () => {
    if (fail) throw new Error('closed catalogue');
    return ready().Exposure.choices;
  }, view => states.push(view), 'Exposure');
  await lookup.refresh();
  assert.match(lookup.snapshot().error, /^Exposure choices could not be loaded:.*closed catalogue/);
  assert.equal(lookup.snapshot().ready, false);
  fail = false;
  await lookup.refresh();
  assert.equal(lookup.snapshot().ready, true);
  assert.equal(lookup.snapshot().error, null);
  assert.equal(states.filter(state => state.busy).length, 2);
  lookup.dispose();
});
