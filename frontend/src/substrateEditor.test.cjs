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
    if (!(name in dependencies)) throw new Error(`Unexpected dependency ${name}`);
    return dependencies[name];
  } });
  return exports;
}
const numeric = load('numericEditor.ts');
const { finiteSingleValue, singleMaximum } = numeric;
const { substrateKeys, substrateKey, stageSubstrate, substrateErrors, substrateDirty,
  substrateValidation, substrateSession, rememberSubstrateSession } = load('substrateEditor.ts', { './numericEditor': numeric });
const plain = value => JSON.parse(JSON.stringify(value));
const values = (value = null) => Object.fromEntries(substrateKeys.map(key => [key, value]));
test('Native-verified substrate component compiles, stages lexical input and retains explicit opt-out', () => {
  const source = readFileSync(path.join(__dirname, 'SubstrateFields.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'SubstrateFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_SUBSTRATE_EDITING !== 'false'/);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /type="text" inputmode="decimal"/);
  assert.match(source, /staged\[key\]\?\.raw \?\?/);
  assert.match(source, /rememberSubstrateSession\(draft, staged, original\)/);
  assert.match(source, /untrack\(\(\) => onvalidation\('substrate', message\)\)/);
  assert.doesNotMatch(source, /PlotService|UpdatePlot|CreatePlot|Math\.fround|max="100"|min="0"/);
  const parent = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  assert.match(parent, /substrateKey\(control.column\)/);
  assert.match(parent, /@render substrateInputs/);
  assert.equal(compile(parent, { filename: 'HeaderEditor.svelte', generate: 'client' }).warnings.length, 0);
});
test('All six nullable substrate values accept negatives, fractions, 100, 101 and exact Single boundaries', () => {
  assert.equal(substrateKeys.length, 6);
  for (const key of substrateKeys) {
    assert.equal(substrateKey(key[0].toUpperCase() + key.slice(1)), key);
    for (const raw of ['', '0', '-3.75', '.125', '99', '100', '101', '1.234567890123', String(singleMaximum), String(-singleMaximum)]) {
      const staged = stageSubstrate({}, key, raw, null);
      assert.equal(staged[key].error, null, `${key}: ${raw}`);
      assert.equal(staged[key].value, raw === '' ? null : Number(raw));
      assert.equal(staged[key].raw, raw);
    }
  }
  assert.equal(substrateKey('Height1'), undefined);
  assert.equal(substrateKey(undefined), undefined);
});
test('Nonfinite, partial, overflow and nondecimal inputs stay explicit raw errors without model writes', () => {
  for (const key of substrateKeys) {
    for (const raw of ['NaN', 'Infinity', '-Infinity', '3.5e38', '-3.5e38', '1e309', '1e', '-', '.', '0x10', '12x']) {
      const staged = stageSubstrate({}, key, raw, 1.25);
      assert.equal(staged[key].raw, raw);
      assert.equal(staged[key].expected, 1.25);
      assert.equal(staged[key].value, null);
      assert.ok(staged[key].error);
      assert.equal(substrateDirty(staged), true);
      assert.ok(substrateValidation(staged, values(1.25), values(1.25)));
    }
  }
});
test('Precision is deliberately float64; combined 60/100/120 values are never balanced or restricted', () => {
  const precision = stageSubstrate({}, 'substrateOrganicMatter', '1.234567890123', 1.25);
  assert.equal(precision.substrateOrganicMatter.value, 1.234567890123);
  assert.notEqual(precision.substrateOrganicMatter.value, Math.fround(1.234567890123));
  for (const samples of [[10,10,10,10,10,10], [10,20,20,20,20,10], [20,20,20,20,20,20]]) {
    const input = Object.fromEntries(substrateKeys.map((key, index) => [key, samples[index]]));
    assert.equal(substrateValidation({}, input, values(0)), null);
    assert.deepEqual(plain(Object.values(input)), samples);
  }
});
test('Historical finite overflow is preserved only unchanged or cleared, never silently reintroduced', () => {
  const historical = values(3.5e38);
  assert.equal(substrateValidation({}, historical, historical), null);
  assert.equal(substrateValidation({}, values(null), historical), null);
  assert.ok(substrateValidation({}, historical, values(null)));
  const same = stageSubstrate({}, 'substrateWater', '3.5e38', 3.5e38);
  assert.equal(same.substrateWater.error, null);
  assert.equal(substrateDirty(same), false);
  assert.ok(finiteSingleValue('historical', 'Infinity', Infinity).error);
});
test('Raw sessions retain invalid partial input across same-draft remount, not Undo/new plot identities', () => {
  const identity = values(1.25);
  let staged = stageSubstrate({}, 'substrateWater', '1e', 1.25);
  rememberSubstrateSession(identity, staged);
  assert.equal(substrateSession(identity).substrateWater.raw, '1e');
  assert.equal(substrateErrors(substrateSession(identity)).length, 1);
  assert.deepEqual(plain(substrateSession({ ...identity })), {});
  staged = stageSubstrate(staged, 'substrateWater', '-.125', 999);
  assert.equal(staged.substrateWater.expected, 1.25);
  assert.equal(staged.substrateWater.error, null);
  rememberSubstrateSession(identity, staged);
  assert.equal(substrateSession(identity).substrateWater.value, -.125);
  const originallyNull = stageSubstrate(stageSubstrate({}, 'substrateWater', '1', null), 'substrateWater', '2', 99);
  assert.equal(originallyNull.substrateWater.expected, null);
});
test('Lexical-only no-op stays semantically clean and NULL remains distinct from numeric zero', () => {
  const same = stageSubstrate({}, 'substrateWater', '1.2500', 1.25);
  assert.equal(substrateDirty(same), false);
  assert.equal(same.substrateWater.raw, '1.2500');
  const zero = stageSubstrate({}, 'substrateWater', '0', null);
  assert.equal(substrateDirty(zero), true);
  assert.equal(zero.substrateWater.value, 0);
  const cleared = stageSubstrate(zero, 'substrateWater', '', null);
  assert.equal(substrateDirty(cleared), false);
  assert.equal(cleared.substrateWater.value, null);
});
test('Successful Save rebases lexical sessions even when the draft identity remains unchanged', () => {
  const draft = values(1.25), before = values(1.25), saved = values(.125);
  const staged = stageSubstrate({}, 'substrateWater', '.125', 1.25);
  rememberSubstrateSession(draft, staged, before);
  assert.equal(substrateSession(draft, before).substrateWater.raw, '.125');
  assert.deepEqual(plain(substrateSession(draft, saved)), {});
  assert.deepEqual(plain(substrateSession(draft, before)), {});
});
