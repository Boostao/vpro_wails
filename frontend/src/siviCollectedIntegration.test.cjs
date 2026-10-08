const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { componentFunctions } = require('./svelteTestHelpers.cjs');

const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const main = readFileSync(path.join(__dirname, '..', '..', 'main.go'), 'utf8');
const peers = ['siviSession', 'siviCoverSession', 'siviCombinedSession', 'siviCollectedSession'];
const guards = ['busy', 'headerWorkflowBusy', 'siviUnsaved', 'siviCoverUnsaved', 'siviCombinedUnsaved',
  'heightUnsaved', 'otherUnsaved', 'soilUnsaved', 'attributeUnsaved', 'collectedUnsaved', 'speciesUnsaved',
  'siviParentWriteUnsaved', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved'];

function host(overrides = {}) {
  return componentFunctions('FS882Form.svelte', [
    'siviCollectedOperation', 'cycleSIVICollected', 'showSIVICollectedPanel', 'refreshSIVIChildEditors', 'load',
  ], {
    ...Object.fromEntries(guards.map(name => [name, false])),
    ...Object.fromEntries(peers.map(name => [name, null])),
    error: null, successMsg: null, siviCollectedEnabled: true, siviCollectedEditingDisabled: false,
    siviCollectedUnsaved: false, siviCollectedReading: false, siviCollectedPanelOpen: false,
    siviCollectedView: { review: [{}] }, siviPanelOpen: true, siviCoverPanelOpen: true, siviCombinedPanelOpen: true,
    siviParentSharedSession: null, siviProjectAssignmentSession: null, siviParentActionSession: null,
    siviParentWriteSession: null, siviParentSourceView: null, siviParentSession: null,
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
    loadChildData: async () => {},
    ...overrides,
  });
}

test('Collected has independent literal frontend/backend gates and actual native registration without granting other columns', () => {
  assert.equal(compile(parent, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(parent, /VITE_SIVI_COLLECTED_EDITING === 'true'/);
  assert.match(main, /siviFeature\(siviCollectedFeatureEnvironment, os.LookupEnv\)/);
  assert.match(main, /NewSIVICollectedService\(contextService, os.LookupEnv\)/);
  assert.match(main, /application.NewService\(siviCollected\)/);
  for (const method of ['GetOriginal', 'SaveReviewed', 'RestoreReviewed']) assert.ok(parent.includes(`SIVICollectedService.${method}(`));
  assert.match(parent, /read: extended => siviCollectedReads.track\(SIVICollectedService.GetOriginal/);
  assert.match(parent, /siviCollectedSession\?\.dispose\(\);[\s\S]{0,70}siviCollectedReads\.cancelAll\(\)/);
  assert.match(parent, /data-sivi-collected-cancel-read[\s\S]{0,110}siviCollectedReads\.cancelAll/);
  assert.match(parent, /siviCollectedSession\?\.presentation\(extendedShrubs\)/);
});

test('Collected owns root Save Undo Lock close busy and every mutually exclusive editor guard', () => {
  assert.match(parent, /headerWorkflowBusy = \$derived\([^\n]*\|\| siviCollectedBusy/);
  assert.match(parent, /nonParentChildUnsaved = \$derived\([^\n]*\|\| siviCollectedUnsaved/);
  assert.match(parent, /if \(siviCollectedUnsaved\) \{ await siviCollectedOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviCollectedUnsaved\) \{ void siviCollectedOperation\('undo'\); return; \}/);
  assert.match(parent, /\|\| \(siviCollectedClose\?\.blocked \?\? false\)/);
  assert.match(parent, /siviCollectedClose\?\.saveReason \?\? 'Correct or Undo unresolved SIVI Collected cycles.'/);
  assert.match(parent, /SIVI Collected or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock/);
  for (const gate of ['height', 'other', 'soil', 'attribute', 'collected', 'species', 'sivi', 'siviCover', 'siviCombined']) {
    assert.match(parent, new RegExp(`${gate}EditingDisabled = \\$derived\\([^\\n]*\\|\\| siviCollectedUnsaved`));
  }
});

test('actual root Collected operations refuse every conflict without invoking backend mutation', async () => {
  for (const guard of [...guards, 'siviCollectedEditingDisabled']) {
    let calls = 0;
    const context = host({ [guard]: true, siviCollectedSession: { save: async () => { calls++; return true; } } });
    assert.equal(await context.actions.siviCollectedOperation('save'), false, guard);
    assert.equal(calls, 0, guard);
    assert.match(context.error, /no conflicting drafts or operation/);
  }
  for (const operation of ['load', 'save', 'undo', 'retain', 'prune']) {
    let calls = 0;
    const action = async selected => {
      calls++;
      assert.equal(context.siviCollectedReading, operation === 'load');
      if (operation === 'retain' || operation === 'prune') assert.equal(selected, operation);
      return true;
    };
    const context = host({ siviCollectedSession: { load: action, save: action, undo: action, restore: action } });
    assert.equal(await context.actions.siviCollectedOperation(operation), true);
    assert.equal(calls, 1);
    assert.equal(context.siviCollectedReading, false);
  }
});

test('root cycles expose errors and explicit recovery while panel remounts retain their original owner', async () => {
  let cycles = 0;
  let loads = 0;
  let undos = 0;
  const owner = { cycle: rowId => { assert.equal(rowId, '9007199254740993'); cycles++; },
    view: () => ({ error: 'known cycle failure' }), load: async () => { loads++; return true; },
    undo: async () => { undos++; return true; } };
  const context = host({ siviCollectedSession: owner });
  context.actions.cycleSIVICollected('9007199254740993');
  assert.equal(cycles, 1);
  assert.match(context.error, /SIVI Collected known cycle failure/);
  context.siviCollectedEditingDisabled = true;
  context.actions.cycleSIVICollected('9007199254740993');
  assert.equal(cycles, 1);
  assert.match(context.error, /cycle failed.*Unlock/);
  context.siviCollectedUnsaved = true;
  context.siviCollectedView = { review: null };
  await context.actions.showSIVICollectedPanel();
  assert.equal(context.siviCollectedPanelOpen, true);
  assert.equal(context.siviPanelOpen || context.siviCoverPanelOpen || context.siviCombinedPanelOpen, false);
  await context.actions.showSIVICollectedPanel();
  assert.equal(context.siviCollectedPanelOpen, false);
  assert.equal(context.siviCollectedSession, owner);
  assert.equal(loads, 0);
  assert.equal(await context.actions.siviCollectedOperation('undo'), true);
  assert.equal(undos, 1);
});

test('all four actual source owners preflight every peer before any original or parent reset', async () => {
  const ownerNames = ['height', 'cover', 'combined', 'collected'];
  for (const [index, owner] of ownerNames.entries()) {
    const calls = [];
    const contexts = Object.fromEntries(peers.map((name, peerIndex) => [name, {
      closeState: () => ({ unsaved: false, busy: false }), view: () => ({ review: [{}] }),
      refreshSource: async () => { calls.push(name); return true; },
    }]));
    const context = host({ ...contexts, loadChildData: async plot => { calls.push(plot); } });
    await context.actions.refreshSIVIChildEditors('P', owner);
    assert.deepEqual(calls, ['P', ...peers.filter((_, peerIndex) => peerIndex !== index)]);
    for (const peer of peers.filter((_, peerIndex) => peerIndex !== index)) {
      for (const field of ['unsaved', 'busy']) {
        calls.length = 0;
        context[peer].closeState = () => ({ [field]: true });
        await assert.rejects(context.actions.refreshSIVIChildEditors('P', owner), /no peer originals were reset/);
        assert.equal(calls.length, 0);
        context[peer].closeState = () => ({ unsaved: false, busy: false });
      }
    }
  }
});

test('unknown or busy Collected owners cannot be replaced, and stale completed operations cannot overwrite the new owner', async () => {
  for (const state of [{ unsaved: true, busy: false }, { unsaved: false, busy: true },
    { unsaved: true, busy: true, blocked: true }]) {
    let disposed = 0;
    const owner = { closeState: () => state, dispose: () => { disposed++; } };
    const context = host({ siviCollectedSession: owner, siviParentSession: { dispose: () => { disposed++; } } });
    await context.actions.load('replacement');
    assert.equal(context.siviCollectedSession, owner);
    assert.equal(disposed, 0);
    assert.match(context.error, /Collected drafts or operations.*before replacing their plot owner/);
  }
  let resolve;
  const context = host({ siviCollectedSession: { load: () => new Promise(done => { resolve = done; }) } });
  const loading = context.actions.siviCollectedOperation('load');
  context.siviCollectedSession = {};
  context.error = 'new owner error';
  context.successMsg = 'new owner status';
  context.siviCollectedReading = true;
  resolve(true);
  assert.equal(await loading, false);
  assert.equal(context.error, 'new owner error');
  assert.equal(context.successMsg, 'new owner status');
  assert.equal(context.siviCollectedReading, true);
});
