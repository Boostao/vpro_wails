const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { actionWriteSession, writeSession, editor, original, cell, setCell, transport, source, metadata,
  assignmentSession, assignmentField } = require('./siviParentTestHelpers.cjs');
const { SIVIParentActionWriteSession, siviParentActionRequest } = actionWriteSession;
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
const plots = ['Ground', 'Visual', 'Note', 'FS882', 'Other'];
function base() {
  const value = original();
  setCell(value, 'PlotType', cell('text', 'legacy'));
  setCell(value, 'SpeciesListComplete', cell('integer', '2'));
  return value;
}
function fixture(overrides = {}) {
  let current = base(), maximum = 0;
  const calls = { save: 0, read: 0, restore: 0, cancel: 0, refresh: 0 };
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    refreshParent: async () => calls.refresh++,
    save: async request => {
      calls.save++;
      maximum = request.actions.length;
      for (const action of request.actions) {
        const plot = action.controlId.endsWith('/optPlotType');
        const value = action.option === null ? cell() : plot ? cell('text', plots[action.option - 1])
          : cell('integer', action.option === 1 ? '-1' : '0');
        setCell(current, plot ? 'PlotType' : 'SpeciesListComplete', value);
      }
      return { ChangedCells: maximum, HistoryID: '9007199254740993',
        SourceRefreshRequired: request.actions.some(action => action.controlId.endsWith('/optPlotType')) };
    },
    restore: async (_, action) => {
      calls.restore++; current = base();
      return { cancelled: false, restoredRows: maximum, prunedAuditRows: action === 'prune' ? maximum : 0, cleanedVegRows: 0 };
    },
    ...overrides,
  };
  return { session: new SIVIParentActionWriteSession(owner, port, () => {}), calls, port };
}
test('all15 action requests preserve explicit source options, implicit row identity and exact wire shape', () => {
  for (let plot = 1; plot <= 5; plot++) for (const species of [null, 1, 2]) {
    const value = base();
    let drafts = editor.stageSIVIParent(value, {}, 'PlotType', { kind: 'option', option: plot });
    drafts = editor.stageSIVIParent(value, drafts, 'SpeciesListComplete', { kind: 'option', option: species });
    const request = siviParentActionRequest(value, drafts);
    assert.deepEqual(Object.keys(request).sort(), ['actions', 'original']);
    assert.equal(request.actions.length, 2);
    for (const action of request.actions) {
      assert.deepEqual(Object.keys(action).sort(), ['contextId', 'controlId', 'expected', 'option', 'rowId', 'table']);
      assert.equal(action.contextId, owner.contextId);
      const isPlot = action.controlId.endsWith('/optPlotType');
      assert.equal(action.rowId, isPlot ? '-9223372036854775808' : '9007199254740993');
      assert.equal(action.table, isPlot ? 'Project_Admin' : 'Project_Env');
      assert.equal(action.option, isPlot ? plot : species);
    }
    request.original.ContextID = 'foreign';
    request.actions[0].expected.storage = 'blob';
    assert.equal(value.ContextID, owner.contextId);
    assert.equal(drafts.PlotType.expected.storage, 'text');
  }
});
test('action and direct scopes reject each other and never manufacture initialization audits', async () => {
  const { session, calls } = fixture();
  await session.load();
  assert.equal(Object.keys(session.view().drafts).length, 0);
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  assert.throws(() => session.stage('SV_FloodPlain', { kind: 'boolean', value: true }), /action editing/);
  let drafts = editor.stageSIVIParent(base(), {}, 'SV_StandHeight', { kind: 'text', raw: '3' });
  assert.throws(() => siviParentActionRequest(base(), drafts), /exclude direct/);
  drafts = editor.stageSIVIParent(base(), {}, 'PlotType', { kind: 'option', option: 4 });
  assert.throws(() => writeSession.siviParentDirectRequest(base(), drafts), /exclude callback/);
});
test('normal PlotType NULL/error survives cloning/remount and clears on correction or Undo', async () => {
  const { session, calls } = fixture();
  await session.load();
  session.stage('PlotType', { kind: 'option', option: null });
  assert.equal(session.closeState().blocked, true);
  const remount = session.view();
  remount.drafts.PlotType.input.option = 1;
  assert.equal(session.view().drafts.PlotType.input.option, null);
  await assert.rejects(session.save());
  assert.equal(calls.save, 0);
  session.stage('PlotType', { kind: 'option', option: 4 });
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().error, null);
  assert.equal(await session.save(), true);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.view().historyId, '9007199254740993');
  assert.equal(await session.restore('retain'), true);
  assert.equal(session.view().historyId, null);
  session.stage('PlotType', { kind: 'option', option: null });
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.view().error, null);
});
test('source Refresh acknowledgement is checked against actual action scope after retiring replayable drafts', async () => {
  for (const [column, result] of [
    ['PlotType', { ChangedCells: 1, HistoryID: '1', SourceRefreshRequired: false }],
    ['SpeciesListComplete', { ChangedCells: 1, HistoryID: '1', SourceRefreshRequired: true }],
    ['PlotType', { ChangedCells: 1, HistoryID: '1' }],
    ['PlotType', { ChangedCells: 1, HistoryID: '1', SourceRefreshRequired: 1 }],
  ]) {
    let saves = 0;
    const { session } = fixture({ save: async () => { saves++; return result; } });
    await session.load(); session.stage(column, { kind: 'option', option: 1 });
    assert.equal(await session.save(), false);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    assert.equal(session.view().historyId, null);
    assert.equal(session.closeState().blocked, true);
    assert.match(session.view().error, /Refresh acknowledgement/);
    await assert.rejects(session.save(), /do not replay/);
    assert.equal(saves, 1);
    assert.equal(await session.undo(), true);
  }
});
test('species-only action acknowledgement permits explicit NULL without claiming source Refresh', async () => {
  const { session, calls } = fixture();
  await session.load();
  session.stage('SpeciesListComplete', { kind: 'option', option: null });
  assert.equal(await session.save(), true);
  const binding = session.view().original.Bindings.find(binding => binding.Binding === 'SpeciesListComplete');
  assert.equal(session.view().original.Rows[0].Env.cells[binding.Column].storage, 'null');
  assert.equal(calls.refresh, 1);
  assert.equal(await session.restore('prune'), true);
  assert.equal(calls.restore, 1);
});
test('action BLOB correction is blocked while retention is unchanged and known cleanup commits cannot replay', async () => {
  const value = base(); setCell(value, 'SpeciesListComplete', cell('blob', 'dead'));
  const { session, calls } = fixture({ read: async () => value });
  await session.load(); session.stage('SpeciesListComplete', { kind: 'option', option: 1 });
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(), /BLOB/);
  assert.equal(calls.save, 0);
  session.stage('SpeciesListComplete', { kind: 'original' });
  assert.equal(session.closeState().blocked, false);
  assert.equal(await session.save(), true);
  let saves = 0;
  const committed = fixture({ save: async () => { saves++; throw new Error('SIVI parent edit committed but cleanup failed; reload before retrying'); } });
  await committed.session.load(); committed.session.stage('PlotType', { kind: 'option', option: 1 });
  assert.equal(await committed.session.save(), false);
  assert.equal(Object.keys(committed.session.view().drafts).length, 0);
  assert.equal(committed.session.closeState().blocked, true);
  await assert.rejects(committed.session.save(), /do not replay/);
  assert.equal(saves, 1);
});
const ActionField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'), 'SIVIParentActionField.svelte', {
  './projectMetadataEditor': metadata,
});
const DirectField = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'), 'SIVIParentDirectField.svelte', {
  './projectMetadataEditor': metadata,
});
const Panel = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8'), 'SIVIParentReadPanel.svelte', {
  '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
  './siviParentTransport': transport, './siviParentWriteSession': writeSession, './siviParentEditor': editor,
  './SIVIParentDirectField.svelte': { default: DirectField }, './SIVIParentActionField.svelte': { default: ActionField },
  './siviProjectAssignmentSession': assignmentSession, './SIVIProjectAssignmentField.svelte': { default: assignmentField },
});
test('normal source action controls have literal labels/options without initialization or NULL PlotType', async () => {
  const { session, calls } = fixture(); await session.load();
  const props = { view: { original: base(), busy: false, error: null }, actionView: session.view(),
    actionDisabled: false, actionCanSave: true, reloadDisabled: false, onreload() {}, oncancel() {} };
  const markup = render(Panel, { props }).body;
  assert.equal((markup.match(/data-sivi-parent-field=/g) || []).length, 77);
  assert.equal((markup.match(/data-sivi-action-field=/g) || []).length, 2);
  for (const column of ['PlotType', 'SpeciesListComplete']) {
    assert.equal((markup.match(new RegExp(`id="sivi-action-${column}"`, 'g')) || []).length, 1);
    assert.match(markup, new RegExp(`<label for="sivi-action-${column}">`));
  }
  const plot = markup.match(/<select id="sivi-action-PlotType"[\s\S]*?<\/select>/)[0];
  assert.doesNotMatch(plot, /value="null"/);
  for (const caption of ['Grnd', 'Visual', 'Note', 'Full', 'Other']) assert.match(plot, new RegExp(caption));
  const species = markup.match(/<select id="sivi-action-SpeciesListComplete"[\s\S]*?<\/select>/)[0];
  for (const caption of ['Comp.', 'Part.', 'NULL']) assert.ok(species.includes(caption));
  assert.equal(calls.save, 0);
  session.stage('SpeciesListComplete', { kind: 'option', option: null });
  const remount = render(Panel, { props: { ...props, actionView: session.view() } }).body;
  assert.match(remount, /value="null" selected/);
  assert.equal(calls.save, 0);
  const disabled = render(Panel, { props: { ...props, actionView: null } }).body;
  assert.doesNotMatch(disabled, /data-sivi-action-(?:field|toolbar)/);
});
test('action errors precede fields and both scoped editors preserve distinct history during clean reloads', async () => {
  const { session } = fixture(); await session.load();
  session.stage('PlotType', { kind: 'option', option: null });
  const markup = render(Panel, { props: { view: { original: base(), busy: false, error: null }, actionView: session.view(),
    actionDisabled: false, actionCanSave: false, reloadDisabled: true, onreload() {}, oncancel() {} } }).body;
  assert.ok(markup.indexOf('Normal PlotType NULL') < markup.indexOf('data-sivi-parent-field='));
  assert.match(markup, /data-sivi-action-save[^>]*disabled/);
  await session.undo(); session.stage('PlotType', { kind: 'option', option: 1 });
  await session.save();
  const history = session.view().historyId;
  await session.load();
  assert.equal(session.view().historyId, history);
  assert.equal(await session.restore('prune'), true);
});
test('mounted source actions join Save/Undo/Lock/close ownership and refresh only clean peer originals', () => {
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /siviParentActionEditingEnabled = siviParentEditingEnabled && import\.meta\.env\.VITE_SIVI_PARENT_ACTION_EDITING === 'true'/);
  for (const api of ['GetSIVIParentActionOriginal', 'SaveSIVIParentActions', 'RestoreSIVIParentActions']) assert.ok(parent.includes(`ContextService.${api}`));
  assert.match(parent, /if \(siviParentActionUnsaved\) \{ await siviParentActionOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviParentActionUnsaved\) \{ void siviParentActionOperation\('undo'\); return; \}/);
  assert.match(parent, /Undo SIVI source-action drafts\/recovery before changing the plot lock/);
  assert.match(parent, /siviParentActionSession\.closeState\(\)\.unsaved/);
  assert.match(parent, /\|\| \(siviParentActionClose\?\.blocked \?\? false\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentWriteSession, siviProjectAssignmentSession\]\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentActionSession, siviProjectAssignmentSession\]\)/);
  assert.match(parent, /peer\.closeState\(\)\.unsaved \|\| peer\.closeState\(\)\.busy/);
  assert.match(parent, /peer && \(peer\.view\(\)\.original \|\| peer\.view\(\)\.error\) && !await peer\.load\(\)/);
});
