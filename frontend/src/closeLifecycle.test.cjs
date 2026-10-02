const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');

const read = name => readFileSync(path.join(__dirname, name), 'utf8');
const exportsObject = {};
vm.runInNewContext(ts.transpileModule(read('closeLifecycle.ts'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText, { exports: exportsObject });
const { closeDisposition } = exportsObject;
const state = (overrides = {}) => ({ unsaved: false, busy: false, canSave: true, saveReason: '', error: null, ...overrides });

test('clean native close is allowed only after context and editor operations finish', () => {
  assert.equal(closeDisposition(null, false), 'clean');
  assert.equal(closeDisposition(state(), false), 'clean');
  assert.equal(closeDisposition(null, true), 'busy');
  assert.equal(closeDisposition(state({ busy: true }), false), 'busy');
});
test('busy takes precedence over an unsaved draft and dirty requires a decision', () => {
  assert.equal(closeDisposition(state({ unsaved: true }), false), 'dirty');
  assert.equal(closeDisposition(state({ unsaved: true }), true), 'busy');
  assert.equal(closeDisposition(state({ unsaved: true, busy: true }), false), 'busy');
  assert.equal(closeDisposition(state({ unsaved: true, canSave: false }), false), 'dirty');
});
test('native close dialog compiles and exposes explicit accessible decisions', () => {
  const source = read('CloseConfirm.svelte');
  const result = compile(source, { filename: 'CloseConfirm.svelte', generate: 'client' });
  assert.equal(result.warnings.length, 0);
  assert.match(source, /dialog\.showModal\(\)/);
  assert.match(source, /aria-labelledby="close-title"/);
  assert.match(source, /event\.preventDefault\(\)/);
  for (const decision of ['cancel', 'save', 'discard']) assert.ok(source.includes(`onrespond('${decision}')`));
  assert.match(source, /disabled=\{working \|\| !canSave\}/);
});
test('form refuses save-and-close on invalid, locked, loading or incomplete child entry', () => {
  const source = read('FS882Form.svelte');
  assert.match(source, /export function getCloseState\(\): EditorCloseState/);
  assert.match(source, /childUnsaved = \$derived\(heightUnsaved \|\| otherUnsaved \|\| soilUnsaved \|\| attributeUnsaved\)/);
  assert.match(source, /unsaved: dirty \|\| childUnsaved \|\| invalid \|\| newChild !== null \|\| invalidChild/);
  assert.match(source, /const invalid = Object\.keys\(headerValidation\)\.length > 0 \|\| heightInvalid\.length > 0 \|\| otherInvalid\.length > 0/);
  assert.match(source, /if \(!state\.canSave\)/);
  assert.match(source, /return !dirty && !childUnsaved && Object\.keys\(headerValidation\)\.length === 0 && error === null/);
  assert.match(source, /headerRevision\+\+;[\s\S]*?successMsg = 'Changes undone.'/);
  assert.match(source, /\{#key headerRevision\}/);
});
test('root retrieves missed events, flushes focused edits, cancels busy close and awaits save before approval', () => {
  const source = read('App.svelte');
  assert.match(source, /Events\.On\('vpro:close-request'/);
  assert.match(source, /CloseService\.GetPendingCloseRequest\(\)/);
  assert.match(source, /document\.activeElement\.blur\(\)/);
  assert.match(source, /await tick\(\)/);
  assert.match(source, /disposition === 'busy'[\s\S]*?await CloseService\.CancelClose\(requestId\)/);
  assert.match(source, /await editor\.saveForClose\(\)/);
  assert.match(source, /stopCloseEvents\(\)/);
});
