const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { assignmentSession, assignmentField, original, cell, setCell, writeSession, actionWriteSession, editor,
  source, metadata, transport } = require('./siviParentTestHelpers.cjs');
const { SIVIProjectAssignmentSession } = assignmentSession;
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
const first = '-9223372036854775808', second = '9007199254740993';
function snapshot(before = cell('text', 'old')) {
  const parent = original();
  setCell(parent, 'ProjectID', before);
  return { Original: parent, AssignmentAvailable: true,
    AssignmentDiagnostic: 'Observed availability; Save independently rechecks.', Choices: {
    ContextID: owner.contextId, Project: owner.project, Source: 'Env', Alias: 'project', Table: 'Project_Metadata', SourceOption: 1,
    Choices: { columns: [{ name: 'ProjectID', declaredType: 'TEXT' }, { name: 'ProjectTitle', declaredType: 'TEXT' }],
      rows: [{ rowId: first, cells: [cell('text', '  Literal 🌱  '), cell('text', '')] },
        { rowId: second, cells: [cell('text', '  Literal 🌱  '), cell()] }] },
  } };
}
function fixture(overrides = {}, before) {
  let current = snapshot(before), saved;
  const calls = { save: 0, read: 0, restore: 0, cancel: 0, refresh: 0 };
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    refreshParent: async () => calls.refresh++,
    save: async request => {
      calls.save++; saved = structuredClone(request);
      setCell(current.Original, 'ProjectID', structuredClone(request.selection.metadataOriginal.cells[0]));
      return { ChangedCells: 1, HistoryID: second };
    },
    restore: async (_, action) => {
      calls.restore++; current = snapshot(before);
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    ...overrides,
  };
  return { session: new SIVIProjectAssignmentSession(owner, port, () => {}), port, calls,
    current: () => current, saved: () => saved };
}
const choose = (session, rowId = second) => session.stage('ProjectID', { kind: 'selection', rowId });

test('assignment initialization is read-only and preserves physical duplicates, NULL/empty titles and exact schema', async () => {
  const { session, calls } = fixture();
  assert.equal(await session.load(), true);
  assert.equal(session.view().choices.Choices.rows.length, 2);
  assert.equal(session.view().choices.Choices.rows[0].cells[1].storage, 'text');
  assert.equal(session.view().choices.Choices.rows[1].cells[1].storage, 'null');
  assert.equal(session.view().drafts.selection, null);
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  choose(session);
  const view = session.view();
  assert.equal(view.drafts.rowId, second);
  assert.equal(view.drafts.selection.metadataOriginal.rowId, second);
  assert.equal(view.drafts.selection.rowId, second);
  view.drafts.selection.metadataOriginal.cells[0].text = 'foreign';
  view.choices.Choices.columns[0].declaredType = 'BLOB';
  assert.equal(session.view().drafts.selection.metadataOriginal.cells[0].text, '  Literal 🌱  ');
  assert.equal(session.view().choices.Choices.columns[0].declaredType, 'TEXT');
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 1);
  assert.equal(session.view().historyId, second);
  assert.equal(session.closeState().unsaved, false);
});

test('selection transport retains owned control/parent/source/schema and selected literal physical title', async () => {
  for (const source of [1, 2]) {
    const f = fixture();
    if (source === 2) Object.assign(f.current().Choices, { Source: 'Master', SourceOption: 2, Alias: 'VMetaData', Table: 'pRoJeCtMeTaDaTa' });
    await f.session.load(); choose(f.session, first);
    assert.equal(await f.session.save(), true);
    const request = f.saved();
    assert.deepEqual(Object.keys(request).sort(), ['original', 'selection']);
    assert.equal(request.selection.contextId, owner.contextId);
    assert.equal(request.selection.controlId, 'form:frmSIVIsite/ProjectID');
    assert.equal(request.selection.table, 'Project_Env');
    assert.equal(request.selection.rowId, second);
    assert.equal(request.selection.metadataOriginal.rowId, first);
    assert.equal(request.selection.metadataOriginal.cells[0].text, '  Literal 🌱  ');
    assert.equal(request.selection.metadataOriginal.cells[1].text, '');
    assert.equal(request.selection.sourceOption, source);
    assert.equal(request.selection.metadataTable, source === 1 ? 'Project_Metadata' : 'pRoJeCtMeTaDaTa');
    assert.deepEqual(request.selection.metadataColumns, snapshot().Choices.Choices.columns);
    assert.equal(request.selection.expected.text, 'old');
  }
});

test('assignment errors survive remount/close state, block Save and clear on correction or Undo', async () => {
  const { session, calls } = fixture();
  await session.load(); choose(session, 'not a physical identity');
  assert.equal(session.closeState().unsaved, true);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.closeState().canSave, false);
  const remount = session.view();
  remount.drafts.error = null;
  assert.match(session.view().error, /observed physical/);
  await assert.rejects(session.save());
  await assert.rejects(session.load(), /Save or Undo/);
  assert.equal(calls.save, 0);
  choose(session);
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().blocked, false);
  choose(session, 'bad');
  assert.equal(await session.undo(), true);
  assert.equal(session.view().drafts.selection, null);
  assert.equal(session.view().drafts.rowId, null);
  assert.equal(session.view().error, null);
});

test('new values enforce exactly30 UTF-16 units, nonempty TEXT and lossless parent auditing without repairing historical noops', async () => {
  for (const [value, allowed] of [
    [cell('text', 'x'.repeat(30)), true], [cell('text', 'x'.repeat(31)), false],
    [cell('text', '🌱'.repeat(15)), true], [cell('text', '🌱'.repeat(16)), false],
    [cell('text', ''), false], [cell(), false], [cell('integer', '-1'), false],
    [cell('blob', '00'), false],
  ]) {
    const f = fixture();
    f.current().Choices.Choices.rows[1].cells[0] = value;
    await f.session.load(); choose(f.session);
    assert.equal(f.session.closeState().canSave, allowed);
    if (allowed) assert.equal(await f.session.save(), true);
    else await assert.rejects(f.session.save());
    assert.equal(f.calls.save, Number(allowed));
    const historical = fixture({}, value);
    historical.current().Choices.Choices.rows[1].cells[0] = value;
    await historical.session.load(); choose(historical.session);
    assert.equal(historical.session.closeState().unsaved, false);
    assert.equal(await historical.session.save(), true);
    assert.equal(historical.calls.save, 0);
  }
  const f = fixture({}, cell('blob', '00'));
  await f.session.load(); choose(f.session);
  assert.match(f.session.view().error, /lossless source audit/);
  await assert.rejects(f.session.save());
});

test('selection scope cannot stage a direct/action field or leak into accepted direct/action transports', async () => {
  const f = fixture();
  await f.session.load();
  assert.throws(() => f.session.stage('PlotType', { kind: 'selection', rowId: second }), /assignment is unavailable/);
  assert.throws(() => f.session.stage('SV_StandHeight', { kind: 'selection', rowId: second }), /assignment is unavailable/);
  assert.throws(() => editor.stageSIVIParent(original(), {}, 'ProjectID', { kind: 'text', raw: 'new' }), /sixteen-field/);
  const direct = new writeSession.SIVIParentWriteSession(owner, f.port, () => {});
  const action = new actionWriteSession.SIVIParentActionWriteSession(owner, f.port, () => {});
  f.port.read = async () => original();
  await direct.load(); await action.load();
  assert.throws(() => direct.stage('ProjectID', { kind: 'text', raw: 'new' }), /directly bound source/);
  assert.throws(() => action.stage('ProjectID', { kind: 'text', raw: 'new' }), /action editing/);
});

test('read cancellation and disposal discard late paired metadata responses and allow explicit retry', async () => {
  let release;
  const f = fixture({ read: () => new Promise(resolve => { release = resolve; }) });
  const loading = f.session.load();
  f.session.cancel();
  release(snapshot());
  assert.equal(await loading, false);
  assert.equal(f.session.view().original, null);
  assert.equal(f.session.view().choices, null);
  assert.equal(f.calls.cancel, 1);
  f.port.read = async () => snapshot();
  assert.equal(await f.session.load(), true);
  f.port.read = () => new Promise(resolve => { release = resolve; });
  const pending = f.session.load();
  f.session.dispose(); release(snapshot());
  assert.equal(await pending, false);
  assert.equal(f.session.view().choices, null);
  assert.throws(() => choose(f.session), /ownership ended/);
});

test('foreign/incomplete paired originals and metadata fail explicitly without live choices', async () => {
  for (const mutate of [
    wire => { wire.Original.ContextID = 'foreign'; },
    wire => { wire.Choices.ContextID = 'foreign'; },
    wire => { wire.Choices.SourceOption = 3; },
    wire => { wire.Choices.Alias = 'main'; },
    wire => { wire.Choices.Table = 'foreign_Metadata'; },
    wire => { wire.Choices.Choices.columns.pop(); },
    wire => { wire.Choices.Choices.rows[1].rowId = first; },
    wire => { wire.Choices.Choices.rows[0].cells[0] = cell('text', '\ud800'); },
    wire => { delete wire.Choices; },
    wire => { delete wire.Original; },
    wire => { delete wire.AssignmentAvailable; },
    wire => { wire.AssignmentAvailable = 1; },
    wire => { delete wire.AssignmentDiagnostic; },
    wire => { wire.AssignmentDiagnostic = ''; },
  ]) {
    const wire = snapshot(); mutate(wire);
    const { session } = fixture({ read: async () => wire });
    assert.equal(await session.load(), false);
    assert.equal(session.view().original, null);
    assert.equal(session.view().choices, null);
    assert.match(session.view().error, /reload failed/);
    assert.throws(() => choose(session), /assignment is unavailable/);
  }
});

test('observed unavailable metadata remains visible but disabled, without becoming an unsaved/close safety failure', async () => {
  const f = fixture();
  f.current().AssignmentAvailable = false;
  f.current().AssignmentDiagnostic = 'Master WAL assignment unavailable; rollback-journal locking required.';
  assert.equal(await f.session.load(), true);
  assert.equal(f.session.view().available, false);
  assert.equal(f.session.view().choices.Choices.rows.length, 2);
  assert.equal(f.session.closeState().canSave, false);
  assert.equal(f.session.closeState().unsaved, false);
  assert.equal(f.session.closeState().blocked, false);
  assert.throws(() => choose(f.session), /WAL assignment unavailable/);
  await assert.rejects(f.session.save(), /WAL assignment unavailable/);
  assert.equal(f.calls.save, 0);
  f.current().AssignmentAvailable = true;
  await f.session.load(); choose(f.session);
  assert.equal(await f.session.save(), true);
  f.current().AssignmentAvailable = false;
  await f.session.load();
  assert.equal(f.session.view().historyId, second);
  assert.equal(await f.session.restore('prune'), true);
  assert.equal(f.calls.restore, 1);
});

test('rejected Save keeps physical selection; acknowledged or cleanup-failed Save retires it before no-replay recovery', async () => {
  for (const [failure, committed] of [['metadata/source drift', false],
    ['SIVI parent edit committed but cleanup failed; observe storage', true]]) {
    const f = fixture({ save: async () => { f.calls.save++; throw new Error(failure); } });
    await f.session.load(); choose(f.session);
    assert.equal(await f.session.save(), false);
    assert.equal(f.session.view().drafts.selection === null, committed);
    assert.equal(f.session.closeState().blocked, committed);
    if (committed) await assert.rejects(f.session.save(), /do not replay/);
    else {
      f.port.save = async () => ({ ChangedCells: 1, HistoryID: second });
      assert.equal(await f.session.save(), true);
    }
  }
  for (const result of [{ ChangedCells: 1 }, { ChangedCells: 0, HistoryID: '1' }, { ChangedCells: 1, HistoryID: 1 }]) {
    const f = fixture({ save: async () => result });
    await f.session.load(); choose(f.session);
    assert.equal(await f.session.save(), false);
    assert.equal(f.session.view().drafts.selection, null);
    assert.equal(f.session.view().historyId, null);
    assert.equal(f.session.closeState().blocked, true);
    await assert.rejects(f.session.save(), /do not replay/);
    assert.equal(await f.session.undo(), true);
  }
});

test('owned history survives clean peer reload/remount and Undo, then restores exactly once', async () => {
  for (const action of ['retain', 'prune']) {
    const f = fixture();
    await f.session.load(); choose(f.session); await f.session.save();
    await f.session.load();
    assert.equal(f.session.view().historyId, second);
    choose(f.session, first);
    assert.equal(f.session.closeState().unsaved, false);
    choose(f.session, 'invalid');
    await assert.rejects(f.session.restore(action));
    await f.session.undo();
    assert.equal(f.session.view().historyId, second);
    assert.equal(await f.session.restore(action), true);
    assert.equal(f.session.view().historyId, null);
    assert.equal(f.calls.restore, 1);
    await assert.rejects(f.session.restore(action), /verified parent history/);
  }
});

test('commit/restore operations cannot cancel, dispose or stage over in-flight ownership', async () => {
  let release;
  const f = fixture({ save: () => new Promise(resolve => { release = resolve; }) });
  await f.session.load(); choose(f.session);
  const saving = f.session.save();
  assert.throws(() => f.session.cancel(), /cannot be cancelled/);
  assert.throws(() => f.session.dispose(), /cannot be cancelled/);
  assert.throws(() => choose(f.session), /Wait/);
  release({ ChangedCells: 1, HistoryID: second });
  assert.equal(await saving, true);
});

test('assignment adapts prototype-backed ports without losing methods or their receiver', async () => {
  class Port {
    value = snapshot();
    reads = 0;
    refreshes = 0;
    read() { this.reads++; return Promise.resolve(structuredClone(this.value)); }
    cancelRead() {}
    refreshParent() { this.refreshes++; return Promise.resolve(); }
    save(request) {
      setCell(this.value.Original, 'ProjectID', structuredClone(request.selection.metadataOriginal.cells[0]));
      return Promise.resolve({ ChangedCells: 1, HistoryID: second });
    }
    restore(_, action) {
      this.value = snapshot();
      return Promise.resolve({ cancelled: false, restoredRows: 1,
        prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 });
    }
  }
  const port = new Port();
  const session = new SIVIProjectAssignmentSession(owner, port, () => {});
  await session.load(); choose(session);
  assert.equal(await session.save(), true);
  assert.equal(await session.restore('prune'), true);
  assert.equal(port.reads, 3);
  assert.equal(port.refreshes, 2);
});

const DirectField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'),
  'SIVIParentDirectField.svelte', { './projectMetadataEditor': metadata });
const ActionField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'),
  'SIVIParentActionField.svelte', { './projectMetadataEditor': metadata });
const Panel = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8'), 'SIVIParentReadPanel.svelte', {
  '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
  './siviParentTransport': transport, './siviParentWriteSession': writeSession, './siviParentEditor': editor,
  './siviProjectAssignmentSession': assignmentSession, './SIVIProjectAssignmentField.svelte': { default: assignmentField },
  './SIVIParentDirectField.svelte': { default: DirectField }, './SIVIParentActionField.svelte': { default: ActionField },
});
function markup(session) {
  return render(Panel, { props: { view: { original: snapshot().Original, busy: false, error: null },
    assignmentView: session.view(), assignmentDisabled: false, assignmentCanSave: session.closeState().canSave,
    reloadDisabled: session.closeState().unsaved, onreload() {}, oncancel() {} } }).body;
}

test('mounted candidate keeps77 fields, one associated ProjectID selector and literal physical duplicates without requiring action UI', async () => {
  const f = fixture(); await f.session.load();
  const html = markup(f.session);
  assert.equal((html.match(/data-sivi-parent-field=/g) || []).length, 77);
  assert.equal((html.match(/data-sivi-project-assignment-field/g) || []).length, 1);
  assert.equal((html.match(/id="sivi-project-assignment"/g) || []).length, 1);
  assert.match(html, /<label for="sivi-project-assignment">/);
  assert.ok(html.includes(`value="${first}"`));
  assert.ok(html.includes(`value="${second}"`));
  assert.ok(html.includes('(empty text)'));
  assert.ok(html.includes('NULL'));
  assert.equal(html.includes('data-sivi-action-toolbar'), false);
  assert.equal(html.includes('data-sivi-direct-toolbar'), false);
  assert.ok(html.includes('data-sivi-assignment-toolbar'));
  const routine = html.lastIndexOf('ProjectID selection uses');
  assert.ok(routine > html.lastIndexOf('data-sivi-parent-field='));
  choose(f.session, 'invalid');
  const remount = markup(f.session);
  assert.match(remount, /aria-invalid="true"/);
  assert.ok(remount.indexOf('role="alert"') < remount.indexOf('data-sivi-parent-owner'));
});

test('unsupported new metadata remains distinguishable but disabled, and unavailable source displays safety feedback before fields', async () => {
  const f = fixture();
  f.current().Choices.Choices.rows.push({ rowId: '3', cells: [cell(), cell('text', '')] },
    { rowId: '4', cells: [cell('text', ''), cell()] },
    { rowId: '5', cells: [cell('text', '🌱'.repeat(16)), cell('text', 'wide metadata')] });
  await f.session.load();
  const html = markup(f.session);
  for (const id of ['3', '4', '5']) assert.match(html, new RegExp(`<option value="${id}" disabled`));
  f.current().AssignmentAvailable = false;
  f.current().AssignmentDiagnostic = 'Master WAL assignment is unavailable.';
  await f.session.load();
  const unavailable = markup(f.session);
  assert.match(unavailable, /id="sivi-project-assignment"[^>]*disabled/);
  assert.ok(unavailable.indexOf('data-sivi-assignment-unavailable') < unavailable.indexOf('data-sivi-parent-owner'));
  const defaultHtml = render(Panel, { props: { view: { original: snapshot().Original, busy: false, error: null },
    reloadDisabled: false, onreload() {}, oncancel() {} } }).body;
  assert.equal(defaultHtml.includes('data-sivi-assignment-toolbar'), false);
  assert.equal(defaultHtml.includes('data-sivi-project-assignment-field'), false);
});

test('FS882 candidate owns assignment across Save/Undo/Lock/close/remount/source and refreshes both clean peers', () => {
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /siviProjectAssignmentEnabled = siviParentEditingEnabled && import\.meta\.env\.VITE_SIVI_PROJECT_ASSIGNMENT === 'true'/);
  for (const api of ['GetSIVIProjectAssignmentOriginal', 'SaveSIVIProjectAssignment', 'RestoreSIVIProjectAssignment']) {
    assert.ok(parent.includes(`ContextService.${api}`));
  }
  assert.match(parent, /if \(siviProjectAssignmentUnsaved\) \{ await siviProjectAssignmentOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviProjectAssignmentUnsaved\) \{ void siviProjectAssignmentOperation\('undo'\); return; \}/);
  assert.match(parent, /Undo ProjectID selection\/recovery before changing the plot lock/);
  assert.match(parent, /siviProjectAssignmentSession\.closeState\(\)\.unsaved/);
  assert.match(parent, /\|\| \(siviProjectAssignmentClose\?\.blocked \?\? false\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentWriteSession, siviParentActionSession, siviParentSharedSession\]\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession\]\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentWriteSession, siviProjectAssignmentSession, siviParentSharedSession\]\)/);
  assert.match(parent, /onSourceChange=\{source => \{ void changeSIVIProjectSource\(source\); \}\}/);
  assert.match(parent, /onSourceReload=\{\(\) => \{ void reloadSIVIProjectSource\(\); \}\}/);
  assert.match(parent, /Boolean\(siviParentSourceView\?\.error\)/);
});

test('SIVI metadata entry is independently absent by default and guarded by parent/peer drafts and operations', async () => {
  const f = fixture(); await f.session.load();
  const props = { view: { original: snapshot().Original, busy: false, error: null },
    assignmentView: f.session.view(), reloadDisabled: false, onreload() {}, oncancel() {} };
  assert.equal(render(Panel, { props }).body.includes('data-sivi-metadata-open'), false);
  const enabled = render(Panel, { props: { ...props, onMetadata() {}, metadataDisabled: false } }).body;
  assert.match(enabled, /data-sivi-metadata-open[^>]*>Edit metadata<\/button>/);
  assert.doesNotMatch(enabled, /data-sivi-metadata-open[^>]*disabled/);
  choose(f.session);
  const dirty = render(Panel, { props: { ...props, assignmentView: f.session.view(),
    onMetadata() {}, metadataDisabled: false } }).body;
  assert.match(dirty, /data-sivi-metadata-open[^>]*disabled/);
  const busy = render(Panel, { props: { ...props, view: { ...props.view, busy: true },
    onMetadata() {}, metadataDisabled: false } }).body;
  assert.match(busy, /data-sivi-metadata-open[^>]*disabled/);
  const locked = render(Panel, { props: { ...props, onMetadata() {}, metadataDisabled: true } }).body;
  assert.match(locked, /data-sivi-metadata-open[^>]*disabled/);
});

test('SIVI metadata reuses the existing guarded editor and commit recovery instead of assigning or creating an identity implicitly', () => {
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /siviParentView && !metadataOpen/);
  assert.match(parent, /onMetadata=\{metadataEnabled \? openProjectMetadata : undefined\}/);
  assert.match(parent, /metadataDisabled=\{childParentDisabled \|\| childUnsaved \|\| Object\.keys\(headerValidation\)\.length > 0\}/);
  assert.match(parent, /oncommitted=\{refreshProjectMetadata\}/);
  const refresh = parent.slice(parent.indexOf('async function refreshProjectMetadata()'), parent.indexOf('async function refreshProjectMetadata()') + 700);
  assert.match(refresh, /refreshSIVIParent\(draft\.plotNumber, \[siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession, siviParentSharedSession\]\)/);
  assert.match(refresh, /siviParentSourceSession\.load\(\)/);
  assert.match(refresh, /siviParentSession\?\.view\(\)\.error/);
  assert.match(refresh, /siviParentSourceSession\.view\(\)\.error/);
  assert.match(parent, /peer\.view\(\)\.original \|\| peer\.view\(\)\.error/);
  assert.match(refresh, /throw new Error/);
  assert.doesNotMatch(refresh, /\.save\(|\.restore\(|CreateBlankProjectMetadata|SaveSIVIProjectAssignment/);
});

test('metadata commit refresh recovery blocks manual/native close and Undo until refresh-only retry succeeds', () => {
  const component = readFileSync(path.join(__dirname, 'ProjectMetadataEditor.svelte'), 'utf8');
  const restore = readFileSync(path.join(__dirname, 'ProjectMetadataRestore.svelte'), 'utf8');
  assert.match(component, /blocked: committedFailure \|\| errors\.length > 0/);
  assert.match(restore, /blocked: committedFailure/);
  assert.match(component, /if \(committedFailure\) \{ error = 'Reload the committed metadata refresh before Undo or close/);
  assert.match(component, /if \(busy \|\| dirty \|\| errors\.length \|\| committedFailure\)/);
  assert.match(component, /onclick=\{close\} disabled=\{busy \|\| dirty \|\| errors\.length > 0 \|\| committedFailure\}/);
  const reload = component.slice(component.indexOf('async function reload()'), component.indexOf('function select('));
  assert.match(reload, /if \(committedFailure\) await oncommitted\(\)/);
  assert.ok(reload.indexOf('await oncommitted()') < reload.indexOf('client.ReviewProjectMetadata'));
  assert.ok(reload.indexOf('client.ReviewProjectMetadata') < reload.indexOf('committedFailure = false'));
  assert.doesNotMatch(reload, /SaveProjectMetadata|CreateBlankProjectMetadata|CreateProjectMetadataFromTemplate|RestoreProjectMetadata/);
});
