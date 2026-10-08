const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, original, cell, setCell, source, metadata, restoration, writeSession, editor } = require('./siviParentTestHelpers.cjs');
const { SIVIParentReadSession } = loadTypeScript('siviParentReadSession.ts', { './siviParentTransport': transport });
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
const { SIVIParentSourceSession } = loadTypeScript('siviParentSourceSession.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
});
const join = () => ({ ContextID: owner.contextId, Project: owner.project, Plot: owner.plot,
  Scope: 'DAO-General1033-ASCII-alphanumeric-TEXT7', Diagnostic: 'Not write authorization', Verified: true,
  EnvRowIDs: ['1'], AdminRowIDs: ['2'] });
function choices(option = 2) {
  return { ContextID: owner.contextId, Project: owner.project, SourceOption: option,
    Source: option === 1 ? 'Env' : 'Master', Alias: option === 1 ? 'project' : 'VMetaData',
    Table: option === 1 ? 'Project_Metadata' : 'ProjectMetadata', Choices: {
      columns: [{ name: 'ProjectID', declaredType: 'TEXT' }, { name: 'ProjectTitle', declaredType: 'TEXT' }],
      rows: [{ rowId: '1', cells: [cell('text', 'DUP'), cell('null')] },
        { rowId: '2', cells: [cell('text', 'DUP'), cell('text', '')] }],
    } };
}
function deferred() { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; }
test('read-only owner validates response and keeps clones across panel remounts', async () => {
  const r = original(), captured = { ...owner };
  const session = new SIVIParentReadSession(captured, () => Promise.resolve(r), () => {}, () => {});
  captured.plot = 'foreign';
  assert.equal(await session.load(), true);
  const remount = session.view(); remount.original.Plot = 'caller';
  assert.equal(session.view().original.Plot, 'P');
  assert.equal(session.view().busy, false);
  for (const mutate of [r => r.ContextID = 'old', r => r.Plot = 'old', r => r.Project = 'old']) {
    const foreign = original(); mutate(foreign);
    const failed = new SIVIParentReadSession(owner, () => Promise.resolve(foreign), () => {}, () => {});
    assert.equal(await failed.load(), false);
    assert.equal(failed.view().original, null);
    assert.match(failed.view().error, /another active context/);
  }
});
test('cancellation disposal late completion and retry never replace the current reader', async () => {
  const first = deferred(), second = deferred(); let count = 0, cancelled = 0, notified = 0;
  const session = new SIVIParentReadSession(owner, () => ++count === 1 ? first.promise : second.promise,
    () => cancelled++, () => notified++);
  const pending = session.load();
  assert.equal(session.view().busy, true);
  await assert.rejects(() => session.load(), /cancel the current/);
  session.cancel();
  assert.equal(cancelled, 1);
  assert.equal(session.view().busy, false);
  assert.match(session.view().error, /cancelled/);
  const retry = session.load(); second.resolve(original());
  assert.equal(await retry, true);
  first.resolve(null);
  assert.equal(await pending, false);
  assert.equal(session.view().original.Plot, 'P');
  assert.equal(session.view().error, null);
  const late = deferred();
  const disposed = new SIVIParentReadSession(owner, () => late.promise, () => cancelled++, () => notified++);
  const old = disposed.load(); disposed.dispose();
  const notices = notified; late.reject(new Error('late backend failure'));
  assert.equal(await old, false);
  assert.equal(notified, notices);
  await assert.rejects(() => disposed.load(), /ownership ended/);
});
test('failed reads clear old originals show explicit errors and allow clean retry', async () => {
  let fail = false;
  const session = new SIVIParentReadSession(owner, () => fail ? Promise.reject(new Error('locked source')) : Promise.resolve(original()),
    () => {}, () => {});
  assert.equal(await session.load(), true); fail = true;
  assert.equal(await session.load(), false);
  assert.equal(session.view().original, null);
  assert.match(session.view().error, /locked source/);
  fail = false; assert.equal(await session.load(), true);
  assert.equal(session.view().error, null);
});
const panelSource = readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8');
const DirectField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'), 'SIVIParentDirectField.svelte', {
  './projectMetadataEditor': metadata,
});
const ActionField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'), 'SIVIParentActionField.svelte', {
  './projectMetadataEditor': metadata,
});
const Panel = serverComponent(panelSource, 'SIVIParentReadPanel.svelte', {
  '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
  './siviParentTransport': transport,
  './siviParentWriteSession': writeSession, './siviParentEditor': editor,
  './SIVIParentDirectField.svelte': { default: DirectField },
  './SIVIParentActionField.svelte': { default: ActionField },
});
test('read-only source panel has all77 labels/groups and distinct raw storage without editing controls', () => {
  const r = original(); setCell(r, 'SV_PolygonNumber', cell('text', ''));
  setCell(r, 'SV_CanopyComposition', cell('text', '  literal  '));
  setCell(r, 'SpeciesListComplete', cell('integer', '-1'));
  const markup = render(Panel, { props: { view: { original: r, busy: false, error: null },
    reloadDisabled: false, onreload() {}, oncancel() {} } }).body;
  assert.equal((markup.match(/data-sivi-parent-field=/g) || []).length, 77);
  for (const field of source.forms[0].fields.filter(field => field.binding)) {
    assert.match(markup, new RegExp(`data-sivi-parent-field="${field.binding}"`));
    const label = (field.caption || field.controlName).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
    assert.ok(markup.includes(label), field.controlId);
  }
  assert.match(markup, /Plot &amp; location/);
  assert.match(markup, /Site description/);
  assert.match(markup, /Vegetation/);
  assert.match(markup, /\(empty text\)/);
  assert.match(markup, /data-storage="null">NULL/);
  assert.match(markup, /  literal  /);
  assert.match(markup, /Species list complete:[\s\S]*?original -1/);
  assert.doesNotMatch(markup, /<(?:input|textarea|select)\b|Save parent/);
});
test('panel safety feedback precedes fields and parent owner sits outside tab lifetime', () => {
  const markup = render(Panel, { props: { view: { original: null, busy: true, error: 'Foreign context rejected' },
    reloadDisabled: false, onreload() {}, oncancel() {} } }).body;
  assert.match(markup, /Foreign context rejected/);
  assert.match(markup, /data-sivi-parent-cancel/);
  assert.match(markup, /data-sivi-parent-reload[^>]*disabled/);
  assert.doesNotMatch(markup, /data-sivi-parent-field=/);
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /VITE_SIVI_PARENT_REVIEW === 'true'/);
  assert.match(parent, /siviParentSession\?\.dispose\(\)/);
  assert.match(parent, /GetSIVIParentOriginal\(siviContextId, plot\)/);
  assert.match(parent, /headerWorkflowBusy = \$derived\([^\n]*siviParentBusy/);
  assert.match(parent, /dirty \|\| nonParentChildUnsaved \|\| !siviParentSession/);
});
test('source preference updates only its owned choice snapshot and survives remount without reinitialization', async () => {
  const calls = [];
  const session = new SIVIParentSourceSession(owner, {
    join: async () => join(), choices: async () => choices(),
    setSource: async (expected, source) => { calls.push([expected, source]); return choices(source); }, cancel() {},
  }, () => {});
  assert.equal(await session.load(), true);
  const remount = session.view(); remount.choices.Choices.rows[0].cells[0].text = 'caller';
  assert.equal(session.view().choices.Choices.rows[0].cells[0].text, 'DUP');
  assert.equal(await session.setSource(1), true);
  assert.equal(session.view().choices.SourceOption, 1);
  assert.deepEqual(calls, [[2, 1]]);
  assert.equal(session.view().join.Verified, true);
  await assert.rejects(() => session.setSource(3), /Env or Master/);
});
test('source commits block cancel disposal and concurrent operations; failed responses require observation before retry', async () => {
  const pending = deferred();
  const session = new SIVIParentSourceSession(owner, {
    join: async () => join(), choices: async () => choices(), setSource: () => pending.promise, cancel() {},
  }, () => {});
  await session.load();
  const saving = session.setSource(1);
  assert.equal(session.view().saving, true);
  assert.throws(() => session.cancel(), /cannot be cancelled/);
  assert.throws(() => session.dispose(), /before ending ownership/);
  await assert.rejects(() => session.load(), /current SIVI source operation/);
  pending.reject(new Error('source changed'));
  assert.equal(await saving, false);
  assert.equal(session.view().choices, null);
  assert.match(session.view().error, /Reload to observe/);
  await assert.rejects(() => session.setSource(1), /Reload owned/);
  assert.equal(await session.load(), true);
  assert.equal(session.view().error, null);
});
test('source wire rejects foreign identities partial sparse rows bad typed cells and false certification', async () => {
  for (const change of [
    value => value.ContextID = 'foreign',
    value => value.Source = 'Env',
    value => delete value.Choices.rows[0],
    value => value.Choices.rows[1].rowId = '1',
    value => value.Choices.rows[0].cells[1] = cell('integer', '01'),
    value => value.Choices.columns.pop(),
  ]) {
    const wire = choices(); change(wire);
    const session = new SIVIParentSourceSession(owner, {
      join: async () => join(), choices: async () => wire, setSource: async () => wire, cancel() {},
    }, () => {});
    assert.equal(await session.load(), false);
    assert.match(session.view().error, /incomplete|another|identities|not returned/);
  }
  const bad = join(); bad.AdminRowIDs = [];
  const session = new SIVIParentSourceSession(owner, {
    join: async () => bad, choices: async () => choices(), setSource: async () => choices(), cancel() {},
  }, () => {});
  assert.equal(await session.load(), false);
  assert.equal(session.view().choices, null);
});
test('source cancelled/disposed reads cannot repopulate another owner or erase errors', async () => {
  const late = deferred(); let cancellations = 0;
  const session = new SIVIParentSourceSession(owner, {
    join: () => late.promise, choices: async () => choices(), setSource: async () => choices(),
    cancel: () => cancellations++,
  }, () => {});
  const loading = session.load(); session.cancel(); late.resolve(join());
  assert.equal(await loading, false);
  assert.equal(session.view().choices, null);
  assert.match(session.view().error, /cancelled/);
  assert.equal(cancellations, 1);
  session.dispose();
  await assert.rejects(() => session.load(), /ownership ended/);
});
test('a failed parallel source read cancels its outstanding peer before releasing busy state', async () => {
  const pending = deferred(); let cancelled = 0;
  const session = new SIVIParentSourceSession(owner, {
    join: () => pending.promise, choices: async () => { throw new Error('invalid retained source'); },
    setSource: async () => choices(), cancel: () => cancelled++,
  }, () => {});
  assert.equal(await session.load(), false);
  assert.equal(cancelled, 1);
  assert.equal(session.view().busy, false);
  assert.match(session.view().error, /invalid retained source/);
  pending.resolve(join());
  await Promise.resolve();
  assert.equal(session.view().join, null);
});
test('native source presentation distinguishes duplicate NULL/empty metadata and exposes only preference actions', () => {
  const markup = render(Panel, { props: { view: { original: original(), busy: false, error: null },
    sourceView: { join: join(), choices: choices(), busy: false, saving: false, error: null },
    reloadDisabled: false, onreload() {}, oncancel() {} } }).body;
  assert.match(markup, /data-sivi-source-env/);
  assert.match(markup, /data-sivi-source-master/);
  assert.equal((markup.match(/data-sivi-project-choice-row=/g) || []).length, 2);
  assert.match(markup, /Not write authorization/);
  assert.match(markup, /\(empty text\)/);
  assert.match(markup, /data-storage="null"/);
  assert.doesNotMatch(markup, /<(?:input|textarea|select)\b/);
  assert.match(markup, /ProjectID assignment remain unavailable/);
});
