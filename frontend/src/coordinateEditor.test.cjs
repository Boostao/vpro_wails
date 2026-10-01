const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const source = readFileSync(path.join(__dirname, 'coordinateEditor.ts'), 'utf8');
const moduleExports = {};
vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText,
  { exports: moduleExports, structuredClone });
const { CoordinateEditor, coordinateParts, isCoordinateControl, coordinateSession, rememberCoordinateSession, coordinateBusy } = moduleExports;
const buffers = (degrees = '', minutes = '', seconds = '', negative = false) => ({ degrees, minutes, seconds, negative });
const tick = () => new Promise(resolve => setImmediate(resolve));
function deferred() { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
function setup(overrides = {}, session) {
  const events = { states: [], busy: [], dirty: 0, values: [], validation: [], errors: [], conversions: [], preferences: [] };
  const api = {
    getMode: async () => 'dd',
    setMode: async mode => { events.preferences.push(mode); },
    decompose: async (value, mode) => mode === 'dd'
      ? { degrees: value, minutes: null, seconds: null, negative: false }
      : value === null ? { degrees: null, minutes: null, seconds: null, negative: false }
      : { degrees: Math.trunc(Math.abs(value)), minutes: mode === 'dms' ? 30 : 30.25, seconds: mode === 'dms' ? 1.25 : null, negative: value < 0 },
    convert: async (axis, mode, parts) => {
      events.conversions.push({ axis, mode, parts });
      if (parts.degrees === null) return null;
      return mode === 'dd' ? parts.degrees : (parts.degrees + parts.minutes / 60 + (parts.seconds ?? 0) / 3600) * (parts.negative ? -1 : 1);
    }, ...overrides
  };
  const editor = new CoordinateEditor(api, {
    state: state => events.states.push(state), busy: busy => events.busy.push(busy),
    dirty: () => events.dirty++, value: (axis, value) => events.values.push({ axis, value }),
    validation: (axis, message) => events.validation.push({ axis, message }),
    error: message => events.errors.push(message)
  }, session);
  return { editor, events, api };
}

test('typed parts preserve signed DD, NULL and whole-magnitude sign', () => {
  const dd = coordinateParts('latitude', 'dd', buffers('-49.123456789'));
  assert.equal(dd.degrees, -49.123456789);
  assert.equal(dd.negative, false);
  assert.equal(dd.minutes, null);
  const dm = coordinateParts('longitude', 'dm', buffers('123', '30.25', '', true));
  assert.equal(dm.negative, true);
  assert.equal(dm.minutes, 30.25);
  assert.equal(coordinateParts('latitude', 'dms', buffers()).degrees, null);
  for (const input of [buffers('49', ''), buffers('49', '60'), buffers('49', '-1'), buffers('-49', '30'), buffers('-0', '30'), buffers('49.5', '1'), buffers('90', '1')]) {
    assert.throws(() => coordinateParts('latitude', 'dm', input));
  }
  for (const input of [buffers('49', '1.5', '0'), buffers('49', '1', '60'), buffers('49', '1', '-1'), buffers('49', '1', ''), buffers('49', '1', 'NaN')]) {
    assert.throws(() => coordinateParts('latitude', 'dms', input));
  }
  for (const text of [' ', '1e', 'Infinity', '91', 'not-number']) assert.throws(() => coordinateParts('latitude', 'dd', buffers(text)));
  assert.equal(coordinateParts('longitude', 'dd', buffers('-180')).degrees, -180);
});
test('preference and decomposition never change draft values or mark dirty', async () => {
  const { editor, events } = setup();
  editor.syncDraft(-49.123456789, null);
  await editor.initialize();
  await editor.changeMode('dm');
  assert.equal(events.dirty, 0);
  assert.equal(events.values.length, 0);
  assert.equal(events.preferences[0], 'dm');
  assert.equal(editor.snapshot().axes.latitude.buffers.negative, true);
  assert.equal(editor.snapshot().axes.longitude.buffers.degrees, '');
  assert.equal(events.busy[0], true);
  assert.equal(events.busy.at(-1), false);
});
test('conversions affect only edited axis and preserve lexical buffer on draft echo', async () => {
  const { editor, events } = setup();
  editor.syncDraft(-49.123456789, -123.987654321);
  await editor.initialize();
  editor.edit('latitude', 'degrees', '-50.1200');
  await tick();
  editor.syncDraft(-50.12, -123.987654321);
  assert.equal(events.values.length, 1);
  assert.equal(events.values[0].axis, 'latitude');
  assert.equal(editor.snapshot().axes.latitude.buffers.degrees, '-50.1200');
  assert.equal(editor.snapshot().axes.longitude.value, -123.987654321);
  assert.equal(events.validation.at(-1).message, null);
});
test('partial/invalid parts mark dirty, preserve coordinate and prevent mode switch', async () => {
  const { editor, events } = setup({ getMode: async () => 'dm' });
  editor.syncDraft(-49.5, null);
  await editor.initialize();
  editor.edit('latitude', 'minutes', '');
  await editor.changeMode('dms');
  assert.equal(events.dirty, 1);
  assert.equal(events.conversions.length, 0);
  assert.equal(events.preferences.length, 0);
  assert.equal(editor.snapshot().axes.latitude.value, -49.5);
  assert.equal(editor.snapshot().axes.latitude.buffers.minutes, '');
  assert.match(editor.snapshot().axes.latitude.error, /Complete all/);
  editor.edit('latitude', 'degrees', '');
  await tick();
  assert.equal(events.values.at(-1).value, null);
  assert.equal(editor.snapshot().axes.longitude.value, null);
});
test('out-of-order converter replies and replies after disposal are ignored', async () => {
  const requests = [];
  const { editor, events } = setup({ convert: () => { const request = deferred(); requests.push(request); return request.promise; } });
  editor.syncDraft(49, -123.25);
  await editor.initialize();
  editor.edit('latitude', 'degrees', '50');
  editor.edit('latitude', 'degrees', '51');
  requests[1].resolve(51); await tick();
  requests[0].resolve(50); await tick();
  assert.equal(events.values.length, 1);
  assert.equal(events.values[0].value, 51);
  assert.equal(events.busy.at(-1), false);
  editor.edit('latitude', 'degrees', '52');
  editor.dispose();
  assert.equal(events.busy.at(-1), true);
  requests[2].resolve(52); await tick();
  assert.equal(events.values.length, 1);
  assert.equal(events.busy.at(-1), false);
});
test('invalid newer text invalidates an older successful converter reply', async () => {
  const request = deferred();
  const { editor, events } = setup({ convert: () => request.promise });
  editor.syncDraft(49, null); await editor.initialize();
  editor.edit('latitude', 'degrees', '50');
  editor.edit('latitude', 'degrees', '1e');
  request.resolve(50); await tick();
  assert.equal(events.values.length, 0);
  assert.equal(editor.snapshot().axes.latitude.value, 49);
  assert.equal(editor.snapshot().axes.latitude.buffers.degrees, '1e');
});
test('external reload invalidates pending conversion and decomposes new raw values', async () => {
  const request = deferred();
  const { editor, events } = setup({ convert: () => request.promise });
  editor.syncDraft(49, null); await editor.initialize();
  editor.edit('latitude', 'degrees', '50');
  editor.syncDraft(-48.123456789, null);
  await tick();
  request.resolve(50); await tick();
  assert.equal(events.values.length, 0);
  assert.equal(editor.snapshot().axes.latitude.buffers.degrees, '-48.123456789');
});
test('cached invalid input survives tab remount but a new draft identity has no cache', async () => {
  const identity = {};
  const first = setup({ getMode: async () => 'dm' });
  first.editor.syncDraft(49.5, null); await first.editor.initialize();
  first.editor.edit('latitude', 'minutes', '1e');
  rememberCoordinateSession(identity, first.editor.snapshot());
  first.editor.dispose();
  const second = setup({ getMode: async () => 'dm' }, coordinateSession(identity));
  second.editor.syncDraft(49.5, null); await second.editor.initialize();
  assert.equal(second.editor.snapshot().axes.latitude.buffers.minutes, '1e');
  assert.match(second.editor.snapshot().axes.latitude.error, /finite/);
  await second.editor.changeMode('dd');
  assert.equal(second.events.preferences.length, 0);
  assert.equal(coordinateSession({}), undefined);
});
test('binding failures and invalid service results expose validation without draft writes', async () => {
  const { editor, events } = setup({ convert: async () => { throw new Error('conversion rejected'); } });
  editor.syncDraft(49, null); await editor.initialize();
  editor.edit('latitude', 'degrees', '50'); await tick();
  assert.equal(events.values.length, 0);
  assert.match(events.validation.at(-1).message, /conversion rejected/);
  assert.equal(events.busy.at(-1), false);
  const invalid = setup({ getMode: async () => '' });
  await invalid.editor.initialize();
  assert.equal(invalid.editor.snapshot().ready, false);
  assert.match(invalid.events.errors.at(-1), /unsupported display mode/);
});
test('pending busy ownership remains accurate across same-draft component remounts', () => {
  const identity = {}, oldOwner = {}, newOwner = {};
  assert.equal(coordinateBusy(identity, oldOwner, true), true);
  assert.equal(coordinateBusy(identity, newOwner, true), true);
  assert.equal(coordinateBusy(identity, oldOwner, false), true);
  assert.equal(coordinateBusy(identity, newOwner, false), false);
});
test('decomposition failures are retryable without draft writes and preserve negative zero', async () => {
  let fail = true;
  const { editor, events } = setup({
    decompose: async value => {
      if (fail) throw new Error('display failure');
      return { degrees: value, minutes: null, seconds: null, negative: false };
    }
  });
  editor.syncDraft(-0, null); await editor.initialize();
  assert.equal(editor.snapshot().axes.latitude.ready, false);
  assert.match(editor.snapshot().axes.latitude.error, /display failure/);
  fail = false; await editor.initialize();
  assert.equal(editor.snapshot().axes.latitude.ready, true);
  assert.equal(editor.snapshot().axes.latitude.buffers.degrees, '-0');
  assert.equal(events.validation.at(-1).message, null);
  assert.equal(events.dirty, 0);
  assert.equal(events.values.length, 0);
});
test('preference failure preserves display mode/buffers and invalid server numeric results never write', async () => {
  const { editor, events } = setup({ setMode: async () => { throw new Error('preference failure'); }, convert: async () => undefined });
  editor.syncDraft(-49.123456789, null); await editor.initialize();
  await editor.changeMode('dm');
  assert.equal(editor.snapshot().mode, 'dd');
  assert.equal(editor.snapshot().axes.latitude.buffers.degrees, '-49.123456789');
  assert.match(events.errors.at(-1), /preference failure/);
  editor.edit('latitude', 'degrees', '-50.1200'); await tick();
  assert.equal(events.values.length, 0);
  assert.match(events.validation.at(-1).message, /invalid coordinate/);
});
test('coordinate inputs retain labelled relationships and configuration/schema/lock gates', () => {
  const component = readFileSync(path.join(__dirname, 'CoordinateFields.svelte'), 'utf8');
  const header = readFileSync(path.join(__dirname, 'HeaderEditor.svelte'), 'utf8');
  for (const [filename, text] of [['CoordinateFields.svelte', component], ['HeaderEditor.svelte', header]]) {
    const compiled = compile(text, { filename, generate: 'client' });
    assert.equal(compiled.warnings.filter(w => w.code.startsWith('a11y')).length, 0);
  }
  assert.match(component, /VITE_COORDINATE_EDITING !== 'false'/);
  assert.match(component, /placeholder=\{capabilities\[mapping\.axis\] !== true \? 'Pending' : ''\}/);
  assert.match(component, /capabilities\[axis\] !== true/);
  assert.match(component, /style=\{position\(control\)\}/);
  assert.match(component, /Safety addition: explicit numeric signs/);
  assert.match(component, /role=\{only \? undefined : 'radiogroup'\}/);
  assert.match(component, /identity = draft;[\s\S]*editor\.syncDraft/);
  assert.match(component, /coordinateBusy\(busyIdentity, busyOwner, value\)/);
  assert.match(header, /paperPage\('Site', 'initial', coordinateMode\)/);
  assert.match(header, /onCoordinateBusyChange\?:/);
  assert.match(header, /isCoordinateControl\(control.controlName\)/);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /error === headerValidationError\(previous\)/);
  assert.match(form, /error = headerValidationError\(validation\)/);
  for (const name of ['Latitude', 'Longitude', 'LatD', 'LatM', 'LatS', 'LatD2', 'LatMD', 'LonD', 'LonM', 'LonS', 'LonD2', 'LonMD', 'Check369', 'Check371', 'Check373', 'optCoordMethod2']) assert.equal(isCoordinateControl(name), true);
  for (const name of ['lblLatitude', 'lblLatD', 'lblLonM', 'OfficeNotes']) assert.equal(isCoordinateControl(name), false);
});
