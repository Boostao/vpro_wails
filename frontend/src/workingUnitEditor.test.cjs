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
const helpers = load('workingUnitEditor.ts', { './becEditor': load('becEditor.ts') });
const masterHelpers = load('masterBECEditor.ts', {
  './workingUnitEditor': helpers,
  './referenceCodeEditor': load('referenceCodeEditor.ts', {
    './becEditor': load('becEditor.ts'),
    './qualityEditor': load('qualityEditor.ts', { './becEditor': load('becEditor.ts') })
  })
});
const { WorkingUnitLookup, workingUnitMode, workingUnitLengthError, workingUnitGroups, workingUnitDefinitions,
  workingUnitValidation, workingUnitCopy, workingUnitAcknowledged, rememberWorkingUnitAcknowledgement } = helpers;
const serialize = value => JSON.parse(JSON.stringify(value));
const codes = (userSiteUnit = '01', becSiteUnit = '02') => ({ userSiteUnit, becSiteUnit });
const row = (rowId, code = '01', description = null) => ({
  rowId, sourceId: '-2147483648', origin: 'master', code, description, scientificName: null, level: 11,
  selectable: code !== null && code !== '', diagnostic: code === null ? 'NULL code' : ''
});
const ready = () => ({ mode: 'master', warning: null, choices: [row('1')], master: [row('1')],
  ready: true, masterReady: true, busy: false, error: null, masterError: null });
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
const api = overrides => ({
  getMode: async () => ({ mode: 'master', warning: null }),
  setMode: async mode => ({ mode, warning: null }),
  choices: async () => [row('1')],
  master: async () => [row('1')],
  ...overrides
});

test('Source-positioned component gates ordinary copy and separate authorized Master editing without reverse writes', () => {
  const source = readFileSync(path.join(__dirname, 'WorkingUnitFields.svelte'), 'utf8');
  const result = compile(source, { filename: 'WorkingUnitFields.svelte', generate: 'client' });
  assert.equal(result.warnings.length, 0);
  assert.match(source, /VITE_WORKING_UNIT_EDITING !== 'false'/);
  assert.match(source, /data-column="BECSiteUnit"[\s\S]*?readonly/);
  assert.match(source, /style=\{position\(control\)\}/);
  assert.match(source, /Confirm draft replacement/);
  assert.match(source, /proposal\.identity !== draft/);
  assert.match(source, /rememberWorkingUnitAcknowledgement\(draft, draft\.userSiteUnit, accepted\)/);
  assert.match(source, /VITE_MASTER_BEC_EDITING !== 'false'/);
  assert.match(source, /!masterEditingEnabled \|\| !masterAllowed \|\| disabled/);
  assert.match(source, /readonly=\{!masterEditingEnabled \|\| !masterAllowed\}/);
  assert.match(source, /disabled=\{disabled \|\| capabilities\.becSiteUnit !== true \|\| !control\.enabled \|\| \(masterEditingEnabled && masterAllowed && view\.busy\)\}/);
  assert.doesNotMatch(source, /PlotService|UpdatePlot|CreatePlot|onmousedown|oncontextmenu/);
  for (const file of ['HeaderEditor.svelte', 'FS882Form.svelte']) {
    assert.equal(compile(readFileSync(path.join(__dirname, file), 'utf8'), { filename: file, generate: 'client' }).warnings.length, 0);
  }
});
test('Working Unit uses exact nullable raw strings and100 UTF-16 bounds without guessed normalization', () => {
  assert.equal(workingUnitLengthError(null), null);
  assert.equal(workingUnitLengthError("O'Q;01"), null);
  assert.equal(workingUnitLengthError('0'.repeat(100)), null);
  assert.match(workingUnitLengthError('0'.repeat(101)), /100 UTF-16/);
  assert.match(workingUnitLengthError(''), /NULL/);
  assert.equal(workingUnitLengthError('😀'.repeat(50)), null);
  assert.match(workingUnitLengthError('😀'.repeat(51)), /100 UTF-16/);
  assert.equal(workingUnitLengthError('old'.repeat(100), 'old'.repeat(100)), null);
});
test('Master policy, physical errors and acknowledgements are independent of Working Unit drafts', () => {
  const view = ready();
  const { masterBECValidation, masterBECAcknowledged, rememberMasterBECAcknowledgement } = masterHelpers;
  assert.equal(masterBECValidation('historical'.repeat(20), 'historical'.repeat(20), false, view, false), null);
  assert.match(masterBECValidation(null, '01', false, view, true), /restricted/);
  assert.equal(masterBECValidation('01', null, true, view, false), null);
  for (const raw of ['', '\ud800', 'x'.repeat(101), '😀'.repeat(51)]) {
    assert.ok(masterBECValidation(raw, null, true, view, true));
  }
  assert.match(masterBECValidation('unknown', null, true, view, false), /acknowledge/);
  assert.equal(masterBECValidation('unknown', null, true, view, true), null);
  assert.match(masterBECValidation('01', null, true, { ...view, masterReady: false }, false), /acknowledge/);
  assert.match(masterBECValidation('01', null, true, { ...view, busy: true }, true), /Wait/);
  const draft = codes('unknown', 'unknown');
  rememberWorkingUnitAcknowledgement(draft, draft.userSiteUnit, true);
  assert.equal(masterBECAcknowledged(draft, draft), false);
  rememberMasterBECAcknowledgement(draft, draft, true);
  draft.userSiteUnit = 'different';
  assert.equal(masterBECAcknowledged(draft, draft), true);
  draft.becSiteUnit = 'different';
  assert.equal(masterBECAcknowledged(draft, draft), false);
  assert.equal(masterBECAcknowledged({ ...draft }, draft), false);
});
test('Actual source Master input overrides source Locked only under its separate gate and loaded policy', () => {
  const { serverComponent } = require('./svelteTestHelpers.cjs');
  const { render } = require('svelte/server');
  const source = readFileSync(path.join(__dirname, 'WorkingUnitFields.svelte'), 'utf8');
  for (const [enabled, allowed] of [[true, true], [true, false], [false, true]]) {
    const Component = serverComponent(source
      .replace('import.meta.env.VITE_MASTER_BEC_EDITING', enabled ? "'true'" : "'false'")
      .replace('import.meta.env.VITE_WORKING_UNIT_EDITING', "'false'"), 'WorkingUnitFields.svelte', {
      './workingUnitEditor': helpers, './masterBECEditor': masterHelpers,
      '../bindings/github.com/boostao/vpro-wails': { WorkingUnitService: {} }
    });
    const fixture = `<script>import Fields from './Fields.svelte';let draft={userSiteUnit:null,becSiteUnit:'old'};</script>
      <Fields bind:draft original={{userSiteUnit:null,becSiteUnit:'old'}} controls={[{controlId:'master',controlName:'BECSiteUnit',enabled:true,locked:true}]}
        position={()=>''} capabilities={{becSiteUnit:true}} disabled={false} masterAllowed={${allowed}}
        onchange={()=>{}} onerror={()=>{}} onvalidation={()=>{}} session={{mode:null}}>
        {#snippet children(inputs)}{@render inputs()}{/snippet}
      </Fields>`;
    const Wrapper = serverComponent(fixture, 'MasterFixture.svelte', { './Fields.svelte': { default: Component } });
    const html = render(Wrapper).body;
    assert.equal((html.match(/data-column="BECSiteUnit"/g) || []).length, 1);
    assert.equal(/\sreadonly(?:\s|=|>)/.test(html), !(enabled && allowed));
    assert.match(html, /value="old"/);
  }
});
test('Duplicate definitions retain every nullable metadata row and signed-string identity', () => {
  const rows = [row('9007199254740993', '01', 'first'), row('2', '02'), row('3', '01', 'conflicting'),
    row('4', null), row('5', ''), row('6', 'X'.repeat(101))];
  const groups = workingUnitGroups(rows);
  assert.equal(groups.length, 2);
  assert.deepEqual(serialize(groups[0].records.map(item => item.rowId)), ['9007199254740993', '3']);
  assert.equal(groups[0].records[0].sourceId, '-2147483648');
  assert.equal(groups[0].records[0].scientificName, null);
  assert.equal(rows.length, 6);
  assert.deepEqual(serialize(workingUnitDefinitions([row('a', 'aB'), row('b', 'Ab')], 'ab').map(item => item.rowId)), ['a', 'b']);
  assert.equal(workingUnitDefinitions([row('a', 'ä')], 'Ä').length, 0);
});
test('Historical unknown/invalid Working Unit never obstructs unrelated saves; changed unknown needs acknowledgement', () => {
  const legacy = codes('X'.repeat(101));
  assert.equal(workingUnitValidation(legacy, legacy, { ...ready(), ready: false, error: 'offline' }, false), null);
  assert.match(workingUnitValidation(codes('custom'), codes(), ready(), false), /acknowledge/);
  assert.equal(workingUnitValidation(codes('custom'), codes(), ready(), true), null);
  assert.equal(workingUnitValidation(codes(null), codes(), ready(), false), null);
  assert.match(workingUnitValidation(codes('X'.repeat(101)), codes(), ready(), true), /100/);
  assert.match(workingUnitValidation(codes('02'), codes(), { ...ready(), busy: true }, true), /finish updating/);
  assert.match(workingUnitValidation(codes('01'), codes(null), { ...ready(), ready: false, error: 'offline' }, false), /acknowledge/);
});
test('Code acknowledgements survive current-draft remount but not edits or another loaded plot', () => {
  const draft = codes('custom');
  rememberWorkingUnitAcknowledgement(draft, draft.userSiteUnit, true);
  assert.equal(workingUnitAcknowledged(draft, draft.userSiteUnit), true);
  assert.equal(workingUnitAcknowledged({ ...draft }, draft.userSiteUnit), false);
  assert.equal(workingUnitAcknowledged(draft, 'CUSTOM'), false);
  rememberWorkingUnitAcknowledgement(draft, draft.userSiteUnit, false);
  assert.equal(workingUnitAcknowledged(draft, draft.userSiteUnit), false);
});
test('Ordinary copy stages one value, confirms overwrite/NULL clear, refuses invalid source and keeps semantic no-op clean', () => {
  assert.equal(workingUnitCopy(codes('old', 'new')).kind, 'confirm');
  const clear = workingUnitCopy(codes('old', null));
  assert.equal(clear.kind, 'confirm');
  assert.equal(clear.value, null);
  assert.match(clear.message, /clearing/);
  assert.equal(workingUnitCopy(codes(null, '01')).kind, 'stage');
  assert.equal(workingUnitCopy(codes('01', '01')).kind, 'unchanged');
  assert.equal(workingUnitCopy(codes(null, null)).kind, 'unchanged');
  assert.equal(workingUnitCopy(codes('old', '')).kind, 'invalid');
  assert.equal(workingUnitCopy(codes('old', 'X'.repeat(101))).kind, 'invalid');
  const unchanged = codes('old', "O'Q;01");
  workingUnitCopy(unchanged);
  assert.deepEqual(unchanged, codes('old', "O'Q;01"));
});
test('Mode selection uses service fallback without mutating codes and exposes no-SU warning', async () => {
  const calls = [], original = codes();
  const lookup = new WorkingUnitLookup(api({
    setMode: async mode => { calls.push(mode); return { mode: 'master', warning: 'Select an SU table first.' }; },
    choices: async mode => { calls.push(mode); return []; }
  }), () => {});
  await lookup.refresh('su');
  assert.deepEqual(calls, ['su', 'master']);
  assert.equal(lookup.snapshot().mode, 'master');
  assert.match(lookup.snapshot().warning, /SU table/);
  assert.equal(lookup.snapshot().ready, true);
  assert.deepEqual(original, codes());
  assert.throws(() => workingUnitMode('DANGER'), /Unsupported/);
});
test('Refresh clears old lists, ignores stale successes/failures and allows true empty choices', async () => {
  const first = deferred();
  let calls = 0;
  const lookup = new WorkingUnitLookup(api({
    getMode: () => ++calls === 1 ? first.promise : Promise.resolve({ mode: 'env', warning: null }),
    choices: async () => []
  }), () => {});
  const old = lookup.refresh();
  assert.equal(lookup.snapshot().busy, true);
  assert.equal(lookup.snapshot().choices.length, 0);
  await lookup.refresh();
  first.reject(new Error('old request failed'));
  await old;
  assert.equal(lookup.snapshot().mode, 'env');
  assert.equal(lookup.snapshot().error, null);
  assert.equal(lookup.snapshot().ready, true);
  assert.equal(lookup.snapshot().choices.length, 0);
});
test('Preference failures/invalid saved modes are explicit, not success-shaped defaults', async () => {
  const lookup = new WorkingUnitLookup(api({ getMode: async () => ({ mode: 'bad', warning: null }) }), () => {});
  await lookup.refresh();
  assert.equal(lookup.snapshot().ready, false);
  assert.equal(lookup.snapshot().mode, null);
  assert.match(lookup.snapshot().error, /Unsupported/);
});
test('Partial catalogue failure retains actual Env choices but advertises unchecked BEC metadata; retry recovers', async () => {
  let fail = true;
  const lookup = new WorkingUnitLookup(api({
    getMode: async () => ({ mode: 'env', warning: null }),
    master: async () => { if (fail) throw new Error('catalogue failed'); return [row('1')]; }
  }), () => {});
  await lookup.refresh();
  assert.equal(lookup.snapshot().ready, true);
  assert.equal(lookup.snapshot().masterReady, false);
  assert.match(lookup.snapshot().masterError, /catalogue failed/);
  fail = false;
  await lookup.refresh();
  assert.equal(lookup.snapshot().masterReady, true);
  assert.equal(lookup.snapshot().masterError, null);
});
test('Concurrent preference writes refused explicitly and disposal never publishes late results', async () => {
  const pending = deferred(), events = [];
  const lookup = new WorkingUnitLookup(api({ setMode: () => pending.promise }), state => events.push(state));
  const first = lookup.refresh('env');
  await assert.rejects(lookup.refresh('master'), /Wait for the current/);
  lookup.dispose();
  const count = events.length;
  pending.resolve({ mode: 'env', warning: null });
  await first;
  assert.equal(events.length, count);
});
test('Header lifecycle and all mutation gates include Working Unit preference/lookup ownership', () => {
  const source = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(source, /headerWorkflowBusy = \$derived\(coordinateBusy \|\| workingUnitBusy \|\| qualityBusy \|\| siteCodeBusy \|\| regionCodeBusy \|\| soilCodeBusy \|\| geologyCodeBusy \|\| parentCodeBusy \|\| drainageBusy \|\| speciesDecisionBusy \|\| deletionBusy \|\| codeCheckBusy \|\| metadataBusy \|\| profileReviewBusy \|\| environmentSUOpen \|\| siviParentBusy \|\| siviBusy\)/);
  assert.match(source, /onWorkingUnitBusyChange=\{\(pending\) => workingUnitBusy = pending\}/);
  assert.match(source, /busy: busy \|\| headerWorkflowBusy/);
  assert.doesNotMatch(source, /busy \|\| coordinateBusy/);
});
test('Same editor mode survives Site remount and Undo without rerunning source Form_Load fallback', async () => {
  let loads = 0;
  const session = { mode: null };
  const source = api({ getMode: async () => { loads++; return { mode: 'master', warning: 'No selected SU' }; } });
  const first = new WorkingUnitLookup(source, () => {}, session);
  await first.refresh();
  await first.refresh('env');
  first.dispose();
  const remounted = new WorkingUnitLookup(source, () => {}, session);
  await remounted.refresh();
  assert.equal(remounted.snapshot().mode, 'env');
  assert.equal(remounted.snapshot().warning, null);
  assert.equal(loads, 1);
  const freshEditor = new WorkingUnitLookup(source, () => {});
  await freshEditor.refresh();
  assert.equal(freshEditor.snapshot().mode, 'master');
  assert.equal(loads, 2);
});
