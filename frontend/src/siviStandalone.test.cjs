const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { componentFunctions } = require('./svelteTestHelpers.cjs');
const source = name => readFileSync(path.join(__dirname, name), 'utf8');

test('standalone source remains reduced normal SIVI, with 77 unique bound controls and three exact plot-linked children', () => {
  const exported = require('../../resources/fs1333-sivi-layout.json').forms[0];
  assert.equal(exported.name, 'frmSIVIsite'); assert.equal(exported.recordSource, 'USysEnv');
  const fields = exported.fields.filter(field => field.binding);
  assert.equal(fields.length, 77); assert.equal(new Set(fields.map(field => field.binding)).size, 77);
  assert.equal(fields.some(field => field.binding === 'SpeciesListComplete'), false, 'completeness is an implicit source action, not another bound control');
  const children = exported.controls.filter(control => control.type === 'Subform');
  assert.deepEqual(children.map(control => [control.controlName, control.properties.SourceObject.value,
    control.properties.LinkMasterFields.value, control.properties.LinkChildFields.value]), [
    ['VegA', 'Form.SubVegA-SIVI_BC', 'PlotNumber', 'PlotNumber'],
    ['VegC', 'Form.SubVegC-SIVI', 'PlotNumber', 'PlotNumber'],
    ['VegD', 'Form.SubVegD-SIVI', 'PlotNumber', 'PlotNumber'],
  ]);
});

test('actual standalone entry refuses unavailable gate/selection and routes only inside existing owned transition', async () => {
  let calls = 0;
  const context = componentFunctions('App.svelte', ['openSIVI'], {
    siviStandaloneEnabled: false, selected: { plotNumber: ' P ' }, $projectState: { contextId: 'owned' },
    error: '', editorPlotNumber: 'original', editorEntryForm: 'fs882', view: 'plots',
    requestTransition: async run => { calls++; await run('owned'); },
  });
  await context.actions.openSIVI();
  assert.equal(calls, 0); assert.match(context.error, /independent entry gate/);
  context.siviStandaloneEnabled = true; context.selected = null;
  await context.actions.openSIVI(); assert.equal(calls, 0);
  context.selected = { plotNumber: ' P ' };
  await context.actions.openSIVI();
  assert.equal(calls, 1); assert.equal(context.editorPlotNumber, ' P ');
  assert.equal(context.editorEntryForm, 'sivi'); assert.equal(context.view, 'fs882');
  assert.equal(context.error, '');
});

test('actual entry does not bypass unsaved/close transition decisions or replace a changed selection', async () => {
  let pending;
  const context = componentFunctions('App.svelte', ['openSIVI'], {
    siviStandaloneEnabled: true, selected: { plotNumber: 'first' }, $projectState: { contextId: 'owned' },
    error: '', editorPlotNumber: 'original', editorEntryForm: 'fs882', view: 'plots',
    requestTransition: async run => { pending = run; },
  });
  await context.actions.openSIVI();
  assert.equal(context.editorPlotNumber, 'original'); assert.equal(context.editorEntryForm, 'fs882');
  context.selected = { plotNumber: 'second' };
  await assert.rejects(pending('owned'), /Selected plot changed/);
  assert.equal(context.editorPlotNumber, 'original'); assert.equal(context.view, 'plots');
});

test('shared parent opener preserves busy/draft barriers and loads originals and preference choices through one path', async () => {
  let parentCalls = 0, choiceCalls = 0;
  const parent = { view: () => ({ original: null }), load: async () => { parentCalls++; return true; } };
  const context = componentFunctions('FS882Form.svelte', ['showSIVIParentPanel', 'loadSIVIParentOriginals'], {
    siviParentReviewEnabled: true, busy: true, headerWorkflowBusy: false, dirty: false,
    nonParentChildUnsaved: false, siviParentSession: parent, error: null, activeTab: 'site', siviParentOpen: false,
    siviParentWriteUnsaved: false, siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false,
    siviParentSharedUnsaved: false, siviParentSourceView: { choices: null, error: null, authorityUnknown: false },
    siviParentSourceSession: { load: async () => { choiceCalls++; } },
  });
  await context.actions.showSIVIParentPanel();
  assert.equal(parentCalls, 0); assert.equal(context.siviParentOpen, false);
  context.busy = false;
  await context.actions.showSIVIParentPanel();
  assert.equal(parentCalls, 1); assert.equal(choiceCalls, 1);
  assert.equal(context.activeTab, 'siviParent'); assert.equal(context.siviParentOpen, true);
  context.siviParentSourceView.authorityUnknown = true;
  await context.actions.showSIVIParentPanel();
  assert.equal(choiceCalls, 1, 'an unknown choice preference is not silently reloaded or reset');
  context.siviParentSourceView.authorityUnknown = false; context.siviParentSharedUnsaved = true;
  await context.actions.showSIVIParentPanel(); assert.equal(choiceCalls, 1);
});

test('shared original loading does not read new-owner choices after late delivery and stops on failed original capture', async () => {
  let resolve, choices = 0;
  const parent = { view: () => ({ original: null }), load: () => new Promise(done => resolve = done) };
  const context = componentFunctions('FS882Form.svelte', ['loadSIVIParentOriginals'], {
    siviParentSession: parent, siviParentSourceSession: { load: async () => { choices++; } },
    siviParentSourceView: { choices: null, error: null, authorityUnknown: false },
    siviParentWriteUnsaved: false, siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false,
    siviParentSharedUnsaved: false,
  });
  const loading = context.actions.loadSIVIParentOriginals();
  context.siviParentSession = { view: () => ({ original: {} }) };
  resolve(true); await loading; assert.equal(choices, 0);
  context.siviParentSession = { view: () => ({ original: null }), load: async () => false };
  await context.actions.loadSIVIParentOriginals(); assert.equal(choices, 0);
  context.siviParentSession = null;
  await assert.rejects(context.actions.loadSIVIParentOriginals(), /no current owned session/);
});

test('standalone host uses the existing editor owner/close guards, not a duplicated controller or FS882 field fallback', () => {
  const app = source('App.svelte'), host = source('FS882Form.svelte');
  for (const [filename, text] of [['App.svelte', app], ['FS882Form.svelte', host]]) {
    assert.equal(compile(text, { filename, generate: 'client' }).warnings.length, 0);
  }
  assert.match(app, /VITE_SIVI_STANDALONE === 'true' && import\.meta\.env\.VITE_SIVI_PARENT_REVIEW === 'true'/);
  assert.match(app, /entryForm=\{editorEntryForm\}/);
  assert.match(app, /editorLabel=\{editorEntryForm === 'sivi' \? 'SIVI \/ FS1333' : 'FS882'\}/);
  const close = source('CloseConfirm.svelte');
  assert.match(close, /editorLabel = 'FS882'/);
  assert.match(close, /\$\{editorLabel\} has an unsaved draft/g);
  assert.match(app, /\{#key editorEntryForm\}/);
  assert.match(app, /view === 'fs882' \? editor\?\.getCloseState/);
  assert.match(host, /activeTab === 'site' && !siviStandalone/);
  assert.match(host, /siviStandalone && !siviStandaloneEnabled/);
  assert.match(host, /No FS882 fallback|no FS882 fallback/);
  assert.match(host, /async function loadSIVIParentOriginals/);
  assert.match(host, /siviStandalone && siviStandaloneEnabled && siviParentSession/);
  assert.match(host, /combined cover\/height editing remain unavailable/);
  assert.match(host, /Close SIVI form/);
  assert.match(host, /siviStandalone && !getCloseState\(\)\.canSave/);
  assert.match(host, /!siviStandaloneEnabled \|\| !siviParentView\?\.original \|\| Boolean\(siviParentView\?\.error\) \|\| childUnsaved \|\| siviSourceBarrier/);
});
