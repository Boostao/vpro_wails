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
const editor = load('regionCodeEditor.ts', { './referenceCodeEditor': reference });
const plain = value => JSON.parse(JSON.stringify(value));
const codes = changes => ({ fsRegionDistrict: null, ecosection: null, ...changes });
const row = (rowId, listName, code, description = 'Definition') => ({
  rowId, listName, code, description, listFilter: null, itemOrder: 1, fieldUsedIn: listName,
  validateLoops: null, validate: false, note: '', flag: true, selectable: code !== null && code !== '', diagnostic: ''
});
const ready = () => ({
  Region: { choices: [row('1', 'Region', 'RCB.DCC'), row('2', 'Region', 'RCB.DCC', 'Duplicate')],
    ready: true, busy: false, error: null },
  Ecosection: { choices: [row('1', 'Ecosection', 'ABC'), row('2', 'Ecosection', 'DEF')],
    ready: true, busy: false, error: null }
});
test('Native-verified source controls compile with explicit opt-out, shared gates and no direct writes', () => {
  const source = readFileSync(path.join(__dirname, 'RegionCodeFields.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'RegionCodeFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_REGION_CODES_EDITING !== 'false'/);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /data-source-control=\{control.controlName\}/);
  assert.match(source, /ListRegionChoices/);
  assert.match(source, /ListEcosectionChoices/);
  assert.match(source, /Retry \{index/);
  assert.match(source, /data-value=\{JSON.stringify\(value\)\}/);
  assert.doesNotMatch(source, /maxlength|\.slice\(|\.substring\(|\.trim\(|\.toUpperCase\(|PlotService|UpdatePlot|CreatePlot/);
  const parent = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  assert.equal(compile(parent, { filename: 'HeaderEditor.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(parent, /regionCodeKey\(control.column\)/);
  assert.match(parent, /@render regionCodeInputs/);
  assert.match(parent, /onbusy=\{onRegionCodeBusyChange\}/);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /siteCodeBusy \|\| regionCodeBusy/);
  assert.match(form, /onRegionCodeBusyChange=\{\(pending\) => regionCodeBusy = pending\}/);
});
test('Measured7/3 UTF16 bounds reject new empty/malformed/overlength text without repair', () => {
  assert.deepEqual(plain(editor.regionCodeKeys), Object.keys(codes()));
  for (const [column, key, limit] of [['FSRegionDistrict', 'fsRegionDistrict', 7], ['Ecosection', 'ecosection', 3]]) {
    assert.equal(editor.regionCodeKey(column), key);
    for (const value of [null, "'".repeat(limit), 'x'.repeat(limit - 2) + '\u{1f600}', ' ']) {
      assert.equal(editor.regionCodeValueError(key, value, null), null);
    }
    for (const value of ['x'.repeat(limit + 1), 'x'.repeat(limit - 1) + '\u{1f600}']) {
      assert.match(editor.regionCodeValueError(key, value, null), /UTF-16.*not been truncated/);
    }
    for (const value of ['\ud800', '\udfff', '\ud800A', '\udfff\ud800']) {
      assert.match(editor.regionCodeValueError(key, value, null), /incomplete Unicode/);
    }
    assert.match(editor.regionCodeValueError(key, '', null), /NULL/);
  }
  assert.equal(editor.regionCodeKey('NtsMapSheet'), undefined);
});
test('Both manual-code domains preserve literal case/apostrophes/whitespace with explicit unmatched review', () => {
  for (const key of editor.regionCodeKeys) {
    for (const value of ["O'Q", ' x ', 'zz', '\u{1f600}']) {
      const draft = codes({ [key]: value });
      assert.match(editor.regionCodeValidation(draft, codes(), ready(), false), /acknowledge/);
      assert.equal(editor.regionCodeValidation(draft, codes(), ready(), true), null);
      assert.equal(draft[key], value);
    }
  }
  const draft = codes({ fsRegionDistrict: 'rcb.dcc', ecosection: 'abc' });
  assert.equal(editor.regionCodeValidation(draft, codes({ fsRegionDistrict: 'RCB.DCC', ecosection: 'ABC' }), ready(), false), null);
  assert.deepEqual(plain(draft), { fsRegionDistrict: 'rcb.dcc', ecosection: 'abc' });
});
test('Suggestions are explicit and independent; definitions retain duplicates and exclude wrong-list/invalid rows', () => {
  const views = ready();
  const rows = [...views.Region.choices, row('3', 'Region', null), row('4', 'Region', ''),
    row('5', 'Region', 'too-long-code'), row('6', 'Region', '\ud800'), row('7', 'Ecosection', 'ABC')];
  assert.deepEqual(plain(editor.regionCodeGroups(rows, 'Region').map(group => [group.code, group.records.length])), [['RCB.DCC', 2]]);
  assert.deepEqual(plain(editor.regionCodeDefinitions(rows, 'Region', 'rcb.dcc').map(item => [item.rowId, item.description, item.note])),
    [['1', 'Definition', ''], ['2', 'Duplicate', '']]);
  assert.deepEqual(plain(editor.regionCodeSuggestions(rows, 'fsRegionDistrict', 'rcb').map(group => group.code)), ['RCB.DCC']);
  assert.deepEqual(plain(editor.regionCodeSuggestions(views.Ecosection.choices, 'ecosection', 'ab').map(group => group.code)), ['ABC']);
  assert.equal(editor.regionCodeSuggestions(rows, 'fsRegionDistrict', 'ABC').length, 0);
});
test('Unavailable catalogue is explicit but does not impose membership or block NULL and unchanged historical values', () => {
  const offline = ready();
  for (const name of ['Region', 'Ecosection']) offline[name] = { choices: [], ready: false, busy: false, error: `${name} offline` };
  const original = codes({ fsRegionDistrict: 'historical-long', ecosection: 'unknown-long' });
  assert.equal(editor.regionCodeValidation({ ...original }, original, offline, false), null);
  assert.equal(editor.regionCodeValidation(codes(), original, offline, false), null);
  for (const key of editor.regionCodeKeys) {
    const draft = codes({ [key]: 'zz' });
    assert.match(editor.regionCodeWarnings(draft, codes(), offline).join(' '), /offline/);
    assert.match(editor.regionCodeValidation(draft, codes(), offline, false), /acknowledge/);
    assert.equal(editor.regionCodeValidation(draft, codes(), offline, true), null);
  }
});
test('Review is bound to exact draft and both values across remounts; changed text invalidates it', () => {
  const draft = codes({ fsRegionDistrict: 'zz', ecosection: "O'Q" });
  editor.rememberRegionCodeAcknowledgement(draft, draft, true);
  assert.equal(editor.regionCodeAcknowledged(draft, draft), true);
  assert.equal(editor.regionCodeAcknowledged({ ...draft }, draft), false);
  assert.equal(editor.regionCodeAcknowledged(draft, codes({ ...draft, ecosection: "o'Q" })), false);
  assert.equal(editor.regionCodeAcknowledged(draft, codes({ ...draft, fsRegionDistrict: 'ZZ' })), false);
  editor.rememberRegionCodeAcknowledgement(draft, draft, false);
  assert.equal(editor.regionCodeAcknowledged(draft, draft), false);
});
test('Busy lookup and physical validation cannot be bypassed by acknowledgement', () => {
  const views = ready();
  views.Region = { ...views.Region, busy: true, ready: false };
  assert.match(editor.regionCodeValidation(codes({ fsRegionDistrict: 'zz' }), codes(), views, true), /Wait/);
  assert.match(editor.regionCodeValidation(codes({ ecosection: 'four' }), codes(), views, true), /3 UTF-16/);
  assert.equal(editor.regionCodeValidation(codes(), codes({ fsRegionDistrict: 'zz' }), views, false), null);
});
