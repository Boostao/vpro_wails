const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { componentFunctions } = require('./svelteTestHelpers.cjs');
const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const requestId = 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
function host(overrides = {}) {
  return componentFunctions('FS882Form.svelte', ['siviCreationUndoOperation', 'stageSIVICreationUndo',
    'siviCreationOperation', 'refreshSIVIChildEditors', 'refreshSIVILifecycleLists', 'save', 'undo', 'getCloseState', 'load', 'toggleLock'], {
    busy: false, headerWorkflowBusy: false, siviLifecycleHostBusy: false, siviCreationHostBusy: false,
    siviSourceAuthorityUnknown: false, siviCreationUndoEnabled: true, siviCreationUndoPending: true,
    siviCreationUndoRecoveryDisabled: false, siviCreationUndoEditingDisabled: false,
    siviCreationUndoClose: { unsaved: true, busy: false, blocked: false, canSave: true, saveReason: '', error: null },
    siviCreationUndoSession: null, siviCreationPending: false, siviDeletionPending: false, siviRestorationPending: false,
    siviCreationSession: null, siviDeletionSession: null, siviRestorationSession: null,
    siviSession: null, siviCoverSession: null, siviCombinedSession: null, siviCollectedSession: null,
    siviSpeciesSession: null, siviIdentitySession: null, twoPageOpen: false, environmentSUOpen: false,
    codeCheckOpen: false, metadataOpen: false, profileReviewBlocked: false, personalDraft: null,
    deletionReview: null, creationDraft: null, pictureMetadataPending: false, siviParentSourceView: null,
    draft: { locked: false }, error: null, successMsg: null, capabilitiesReady: true,
    crypto: { randomUUID: () => requestId }, loadChildData: async () => {}, ...overrides,
  });
}
function derived(name, state) {
  const expression = root.match(new RegExp(`const ${name} = \\$derived\\(([\\s\\S]*?)\\);`));
  assert.ok(expression, name);
  return vm.runInNewContext(expression[1], state);
}
test('actual main and root independently gate historical creation Undo without granting Create or Delete', () => {
  const main = readFileSync(path.join(__dirname, '..', '..', 'main.go'), 'utf8');
  assert.ok(main.indexOf('siviCreationUndoFeatureEnvironment') < main.indexOf('userDataDir()'));
  assert.match(main, /NewSIVICreationUndoService\(contextService, os.LookupEnv\)/);
  assert.match(main, /application.NewService\(siviCreationUndo\)/);
  assert.match(root, /VITE_SIVI_CREATION_UNDO === 'true'/);
  assert.match(root, /ownedSIVICreationUndoSession\(\{ contextId: siviContextId, project: \$projectState.activeProject, plot: original.plotNumber \}\)/);
  assert.match(root, /siviCreationUndoSession\?\.subscribe\(\(\) => untrack\(\(\) => siviCreationUndoRevision\+\+\)\)/);
  assert.match(root, /onoperation=\{operation => void siviCreationUndoOperation\(operation\)\}/);
});
test('parent Save, owner replacement, Lock and native close cannot implicitly perform historical Undo', async () => {
  let writes = 0;
  const current = host({ siviCreationUndoSession: { undo() { writes++; } } });
  assert.equal(current.actions.getCloseState().canSave, false);
  assert.equal(current.actions.getCloseState().unsaved, true);
  await current.actions.save();
  assert.match(current.error, /ordinary Save never performs/);
  await current.actions.load('another plot');
  assert.match(current.error, /before replacing the plot owner/);
  await current.actions.toggleLock();
  assert.match(current.error, /before changing the plot lock/);
  assert.equal(current.draft.locked, false);
  assert.equal(writes, 0);
});
test('parent Undo only discards a known unsubmitted review; unknown authority remains retained', async () => {
  for (const unknown of [false, true]) {
    let discards = 0;
    const current = host({ siviCreationUndoSession: {
      view: () => ({ revision: 0, error: null }), discard() {
        discards++;
        if (unknown) throw new Error('Unknown creation Undo receipt cannot be discarded.');
      },
    } });
    current.actions.undo();
    await Promise.resolve();
    assert.equal(discards, 1);
    assert.equal(current.error === null, !unknown);
    if (unknown) assert.match(current.error, /cannot be discarded/);
  }
});
test('creation Undo close authority preserves unknown parent priority and cannot mask an unknown Undo with a known peer', () => {
  const parent = host({ siviParentSourceView: { authorityUnknown: true, busy: false, error: 'Lost parent source receipt' } });
  assert.equal(parent.actions.getCloseState().error, 'Lost parent source receipt');
  assert.equal(parent.actions.getCloseState().blocked, true);
  const unknown = host({ siviDeletionPending: true,
    siviDeletionClose: { unsaved: true, busy: false, blocked: false, canSave: true, saveReason: '', error: null },
    siviCreationUndoClose: { unsaved: true, busy: false, blocked: true, canSave: false, saveReason: 'Resolve Undo read-only.', error: 'Unknown Undo' } });
  assert.equal(unknown.actions.getCloseState().blocked, true);
  assert.equal(unknown.actions.getCloseState().error, 'Unknown Undo');
  assert.equal(unknown.actions.getCloseState().canSave, false);
});
test('read-only recovery is independent of dirty locked invalid parent editing and retains all peer/host guards', () => {
  const state = { siviCreationUndoEnabled: true, busy: false, siviLifecycleHostBusy: false,
    otherHeaderWorkflowBusy: false, siviCreationUndoClose: { busy: false }, pictureBusy: false,
    pictureMetadataPending: false, siviCreationPeerUnsaved: false, siviCreationPending: false,
    siviDeletionPending: false, siviRestorationPending: false, siviParentWriteUnsaved: false,
    siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false, siviParentSharedUnsaved: false };
  assert.equal(derived('siviCreationUndoRecoveryDisabled', state), false);
  for (const flag of ['busy', 'siviLifecycleHostBusy', 'otherHeaderWorkflowBusy', 'pictureBusy',
    'pictureMetadataPending', 'siviCreationPeerUnsaved', 'siviCreationPending', 'siviDeletionPending',
    'siviRestorationPending', 'siviParentWriteUnsaved', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved']) {
    assert.equal(derived('siviCreationUndoRecoveryDisabled', { ...state, [flag]: true }), true, flag);
  }
  const editing = { siviCreationUndoRecoveryDisabled: false, capabilitiesReady: false, draft: { locked: true },
    siviSourceBarrier: true, dirty: true, original: {}, personalDraft: null, headerValidation: { bad: 'invalid' } };
  assert.equal(derived('siviCreationUndoEditingDisabled', editing), true);
});
test('known mutation and read-only resolution refresh peers/history under a host barrier without replay', async () => {
  for (const operation of ['undo', 'resolve']) {
    const calls = [];
    let revision = 0, current;
    const session = { view: () => ({ revision, receipt: revision ? { plot: 'P', id: 1, replayed: operation === 'resolve' } : null }),
      async undo(id) { calls.push(['undo', id]); revision++; return true; },
      async resolve() { calls.push(['resolve']); revision++; return true; },
      async loadHistory() { calls.push(['creation-history']); return true; } };
    current = host({ siviCreationUndoSession: session,
      siviDeletionSession: { view: () => ({ targets: [] }), async loadTargets() { calls.push(['deletion-targets']); return true; } },
      siviRestorationSession: { view: () => ({ history: { events: [] } }), async loadHistory() { calls.push(['deletion-history']); return true; } },
      loadChildData: async plot => { assert.equal(current.siviLifecycleHostBusy, true); calls.push(['peers', plot]); } });
    assert.equal(await current.actions.siviCreationUndoOperation(operation), true);
    assert.deepEqual(calls, [operation === 'undo' ? ['undo', requestId] : ['resolve'], ['peers', 'P'],
      ['deletion-targets'], ['deletion-history'], ['creation-history']]);
    assert.equal(current.siviLifecycleHostBusy, false);
    assert.match(current.successMsg, operation === 'resolve' ? /no mutation repeated/ : /one exact created row removed/);
  }
});
test('lifecycle refresh initializes only its own list and refreshes every already-loaded peer, including empty lists', async () => {
  for (const owner of ['creation', 'deletion', 'restoration', 'creationUndo']) for (const loaded of [false, true]) {
    const calls = [];
    const current = host({
      siviDeletionSession: { view: () => ({ targets: loaded ? [] : null }),
        async loadTargets() { calls.push('deletion'); return true; } },
      siviRestorationSession: { view: () => ({ history: loaded ? { events: [] } : null }),
        async loadHistory() { calls.push('restoration'); return true; } },
      siviCreationUndoSession: { view: () => ({ history: loaded ? { events: [] } : null }),
        async loadHistory() { calls.push('creationUndo'); return true; } },
    });
    await current.actions.refreshSIVILifecycleLists(owner);
    assert.deepEqual(calls, ['deletion', 'restoration', 'creationUndo'].filter(peer => loaded || peer === owner));
  }
});
test('verified Undo and read-only recovery never initialize independently unavailable optional peers', async () => {
  for (const operation of ['undo', 'resolve']) {
    let revision = 0, mutations = 0, historyReads = 0;
    const unavailable = async () => { throw new Error('Independent peer backend is disabled.'); };
    const current = host({
      siviDeletionSession: { view: () => ({ targets: null }), loadTargets: unavailable },
      siviRestorationSession: { view: () => ({ history: null }), loadHistory: unavailable },
      siviCreationUndoSession: {
        view: () => ({ revision, history: null, receipt: { plot: 'P', id: 1, replayed: operation === 'resolve' } }),
        async undo() { mutations++; revision++; return true; },
        async resolve() { revision++; return true; },
        async loadHistory() { historyReads++; return true; },
      },
    });
    assert.equal(await current.actions.siviCreationUndoOperation(operation), true);
    assert.equal(mutations, operation === 'undo' ? 1 : 0);
    assert.equal(historyReads, 1);
    assert.equal(current.capabilitiesReady, true);
    assert.equal(current.error, null);
    assert.equal(current.siviLifecycleHostBusy, false);
  }
});
test('a failed already-loaded peer refresh remains an explicit postcommit failure without replay', async () => {
  let revision = 0, mutations = 0;
  const current = host({
    siviDeletionSession: { view: () => ({ targets: [] }), async loadTargets() { return false; } },
    siviCreationUndoSession: {
      view: () => ({ revision, receipt: { plot: 'P', id: 1, replayed: false } }),
      async undo() { mutations++; revision++; return true; },
    },
  });
  assert.equal(await current.actions.siviCreationUndoOperation('undo'), false);
  assert.equal(mutations, 1);
  assert.equal(current.capabilitiesReady, false);
  assert.equal(current.siviLifecycleHostBusy, false);
  assert.match(current.error, /already verified.*without repeating Undo.*Reload deletion source targets/);
});
test('postcommit source refresh failure disables edits and explicitly forbids repeating Undo', async () => {
  let writes = 0, revision = 0;
  const current = host({ siviCreationUndoSession: {
    view: () => ({ revision, receipt: { plot: 'P', id: 1, replayed: false } }),
    async undo() { writes++; revision++; return true; },
  }, loadChildData: async () => { throw new Error('source unavailable'); } });
  assert.equal(await current.actions.siviCreationUndoOperation('undo'), false);
  assert.equal(writes, 1);
  assert.equal(current.capabilitiesReady, false);
  assert.equal(current.siviLifecycleHostBusy, false);
  assert.match(current.error, /already verified.*without repeating Undo/);
});
test('an unresolved creation Undo peer prevents any reset of existing child originals', async () => {
  let reads = 0;
  const current = host({ loadChildData: async () => reads++ });
  await assert.rejects(current.actions.refreshSIVIChildEditors('P', 'height'), /no peer originals were reset/);
  assert.equal(reads, 0);
});
test('audit choice and explicit confirmation stage retained authority without writing', () => {
  const calls = [];
  const current = host({ siviCreationUndoSession: { choose: value => calls.push(['action', value]),
    confirm: value => calls.push(['confirm', value]), view: () => ({ error: null }) } });
  current.actions.stageSIVICreationUndo(false, 'prune');
  current.actions.stageSIVICreationUndo(true);
  assert.deepEqual(calls, [['action', 'prune'], ['confirm', true]]);
  current.siviCreationUndoEditingDisabled = true;
  current.actions.stageSIVICreationUndo(false);
  assert.match(current.error, /Unlock the clean owned parent/);
  assert.equal(calls.length, 2);
});
