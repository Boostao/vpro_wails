const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const requestId = '00000000-0000-4000-8000-000000000001';
function host(kind, overrides = {}) {
  return componentFunctions('FS882Form.svelte', ['siviLifecycleOperation', 'stageSIVILifecycle',
    'refreshSIVIChildEditors', 'refreshSIVILifecycleLists', 'save', 'undo', 'getCloseState', 'load', 'toggleLock'], {
    busy: false, headerWorkflowBusy: false, siviLifecycleHostBusy: false, siviSourceAuthorityUnknown: false,
    siviDeletionEnabled: true, siviRestorationEnabled: true,
    siviDeletionRecoveryDisabled: false, siviRestorationRecoveryDisabled: false,
    siviDeletionEditingDisabled: false, siviRestorationEditingDisabled: false,
    siviDeletionPending: kind === 'deletion', siviRestorationPending: kind === 'restoration', siviCreationUndoPending: false,
    siviDeletionClose: { unsaved: true, busy: false, blocked: false, canSave: true, saveReason: '', error: null },
    siviRestorationClose: { unsaved: true, busy: false, blocked: false, canSave: true, saveReason: '', error: null },
    siviDeletionSession: null, siviRestorationSession: null, siviCreationUndoSession: null, siviCreationPending: false,
    siviSession: null, siviCoverSession: null, siviCombinedSession: null, siviCollectedSession: null,
    siviSpeciesSession: null, siviIdentitySession: null,
    twoPageOpen: false, environmentSUOpen: false, codeCheckOpen: false, metadataOpen: false,
    profileReviewBlocked: false, personalDraft: null, deletionReview: null, creationDraft: null,
    pictureMetadataPending: false, siviParentSourceView: null,
    draft: { locked: false }, error: null, successMsg: null, capabilitiesReady: true,
    crypto: { randomUUID: () => requestId }, loadChildData: async () => {},
    ...overrides,
  });
}
function derived(name, state) {
  const expression = root.match(new RegExp(`const ${name} = \\$derived\\(([\\s\\S]*?)\\);`));
  assert.ok(expression, name);
  return vm.runInNewContext(expression[1], state);
}

test('actual main preflights both independent deletion flags before data work and registers the scoped service', () => {
  const main = readFileSync(path.join(__dirname, '..', '..', 'main.go'), 'utf8');
  assert.match(main, /siviDeletionWritingFeatureEnvironment, siviDeletionRestoringFeatureEnvironment/);
  assert.ok(main.indexOf('siviDeletionWritingFeatureEnvironment') < main.indexOf('userDataDir()'));
  assert.match(main, /NewSIVIDeletionService\(contextService, os.LookupEnv\)/);
  assert.match(main, /application.NewService\(siviDeletion\)/);
  assert.match(root, /VITE_SIVI_DELETION_WRITING === 'true'/);
  assert.match(root, /VITE_SIVI_DELETION_RESTORING === 'true'/);
  assert.match(root, /ownedSIVIDeletionSession\(\{ contextId: siviContextId, project: \$projectState.activeProject, plot: original.plotNumber \}\)/);
  assert.match(root, /ownedSIVIRestorationSession\(\{ contextId: siviContextId, project: \$projectState.activeProject, plot: original.plotNumber \}\)/);
});

test('cached deletion/restoration ownership binds all eight actual methods without replacing pending sessions on remount', () => {
  const calls = [];
  const service = Object.fromEntries(['GetTargets', 'GetOriginal', 'Delete', 'LookupReceipt', 'GetHistory',
    'ReviewRestoration', 'Restore', 'LookupRestorationReceipt'].map(method => [method, (...args) => calls.push([method, ...args])]));
  class Session { constructor(owner, port) { this.owner = owner; this.port = port; } }
  const cache = loadTypeScript('siviDeletionSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { SIVIDeletionService: service },
    './siviDeletionSession': { SIVIDeletionSession: Session },
    './siviDeletionRestorationSession': { SIVIDeletionRestorationSession: Session },
  });
  const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
  const deletion = cache.siviDeletionSession(owner), restoration = cache.siviDeletionRestorationSession(owner);
  assert.equal(cache.siviDeletionSession({ ...owner }), deletion);
  assert.equal(cache.siviDeletionRestorationSession({ ...owner }), restoration);
  for (const foreign of [{ ...owner, contextId: 'Second' }, { ...owner, project: 'Second' }, { ...owner, plot: 'Second' }]) {
    assert.notEqual(cache.siviDeletionSession(foreign), deletion);
    assert.notEqual(cache.siviDeletionRestorationSession(foreign), restoration);
  }
  deletion.port.targets(); deletion.port.original('SubVegC-SIVI', '9223372036854775807');
  deletion.port.delete('raw-delete'); deletion.port.receipt('retained-delete');
  restoration.port.history(); restoration.port.review('exact-deletion-uuid');
  restoration.port.restore('raw-restore'); restoration.port.receipt('retained-restore');
  assert.deepEqual(calls, [
    ['GetTargets', 'C', 'P'], ['GetOriginal', 'C', 'P', 'SubVegC-SIVI', '9223372036854775807'],
    ['Delete', 'C', 'raw-delete'], ['LookupReceipt', 'C', 'retained-delete'],
    ['GetHistory', 'C', 'P'], ['ReviewRestoration', 'C', 'P', 'exact-deletion-uuid'],
    ['Restore', 'C', 'raw-restore'], ['LookupRestorationReceipt', 'C', 'retained-restore'],
  ]);
});

test('parent Save, owner replacement and Lock cannot implicitly delete or restore; close cannot offer Save', async () => {
  for (const kind of ['deletion', 'restoration']) {
    let mutations = 0;
    const session = { save: () => mutations++ };
    const current = host(kind, { [kind === 'deletion' ? 'siviDeletionSession' : 'siviRestorationSession']: session });
    assert.equal(current.actions.getCloseState().canSave, false);
    assert.equal(current.actions.getCloseState().unsaved, true);
    await current.actions.save();
    assert.match(current.error, /ordinary Save never performs/);
    await current.actions.load('another plot');
    assert.match(current.error, /before replacing the plot owner/);
    await current.actions.toggleLock();
    assert.match(current.error, /before changing the plot lock/);
    assert.equal(mutations, 0);
    assert.equal(current.draft.locked, false);
  }
});

test('parent Undo delegates only to the retained known review; unknown authority survives failed discard', async () => {
  for (const kind of ['deletion', 'restoration']) {
    let discards = 0;
    const session = { view: () => ({ revision: 0 }), discard() {
      discards++;
      throw new Error('Resolve retained unknown request read-only.');
    } };
    const current = host(kind, { [kind === 'deletion' ? 'siviDeletionSession' : 'siviRestorationSession']: session });
    current.actions.undo();
    await Promise.resolve();
    assert.equal(discards, 1);
    assert.match(current.error, /Resolve retained unknown request read-only/);
    assert.equal(current.actions.getCloseState().unsaved, true);
  }
});

test('known lifecycle review cannot mask pre-existing unknown parent source authority on native close', () => {
  for (const kind of ['deletion', 'restoration']) {
    const current = host(kind, { siviParentSourceView: { authorityUnknown: true, busy: false, error: 'Lost parent source receipt' } });
    const close = current.actions.getCloseState();
    assert.equal(close.blocked, true);
    assert.equal(close.unsaved, true);
    assert.equal(close.canSave, false);
    assert.equal(close.error, 'Lost parent source receipt');
    assert.match(close.saveReason, /observe the source preference outcome/);
  }
});

test('recovery remains possible on a locked invalid parent while peers and host refresh prevent competition', () => {
  const state = { siviDeletionEnabled: true, siviRestorationEnabled: true,
    busy: false, siviLifecycleHostBusy: false, otherHeaderWorkflowBusy: false,
    siviDeletionClose: { busy: false }, siviRestorationClose: { busy: false },
    pictureBusy: false, pictureMetadataPending: false, siviCreationPeerUnsaved: false,
    siviCreationPending: false, siviDeletionPending: false, siviRestorationPending: false, siviCreationUndoPending: false,
    siviParentWriteUnsaved: false, siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false,
    siviParentSharedUnsaved: false, capabilitiesReady: false, draft: { locked: true },
    siviSourceBarrier: true, dirty: true, original: {}, personalDraft: null, headerValidation: { invalid: 'raw error' } };
  for (const kind of ['Deletion', 'Restoration']) {
    const recovery = `sivi${kind}RecoveryDisabled`, editing = `sivi${kind}EditingDisabled`;
    assert.equal(derived(recovery, state), false);
    assert.equal(derived(editing, { ...state, [recovery]: false }), true);
    for (const flag of ['busy', 'siviLifecycleHostBusy', 'otherHeaderWorkflowBusy', 'pictureBusy', 'pictureMetadataPending',
      'siviCreationPeerUnsaved', 'siviCreationPending', kind === 'Deletion' ? 'siviRestorationPending' : 'siviDeletionPending',
      'siviParentWriteUnsaved', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved']) {
      assert.equal(derived(recovery, { ...state, [flag]: true }), true, `${recovery}: ${flag}`);
    }
  }
});

test('known mutation and read-only recovery refresh source peers once; refresh failures cannot repeat mutation', async () => {
  for (const kind of ['deletion', 'restoration']) for (const operation of ['save', 'resolve']) {
    const calls = [];
    let revision = 0, current;
    const session = {
      view: () => ({ revision, receipt: revision ? { plot: 'P', id: -7, replayed: operation === 'resolve' } : null }),
      async save(id) { calls.push(['save', id]); revision++; return true; },
      async resolve() { calls.push(['resolve']); revision++; return true; },
      async loadTargets() { calls.push(['targets']); return true; },
      async loadHistory() { calls.push(['history']); return true; },
    };
    current = host(kind, {
      [kind === 'deletion' ? 'siviDeletionSession' : 'siviRestorationSession']: session,
      loadChildData: async plot => { assert.equal(current.siviLifecycleHostBusy, true); calls.push(['peers', plot]); },
    });
    assert.equal(await current.actions.siviLifecycleOperation(kind, operation), true);
    assert.deepEqual(calls, [operation === 'save' ? ['save', requestId] : ['resolve'], ['peers', 'P'],
      [kind === 'deletion' ? 'targets' : 'history']]);
    assert.equal(current.siviLifecycleHostBusy, false);
    assert.match(current.successMsg, operation === 'resolve' ? /no mutation repeated/ : /one exact physical row committed/);
    const failed = host(kind, { [kind === 'deletion' ? 'siviDeletionSession' : 'siviRestorationSession']: session,
      loadChildData: async () => { throw new Error('source unavailable'); } });
    assert.equal(await failed.actions.siviLifecycleOperation(kind, operation), false);
    assert.match(failed.error, /already verified.*without repeating the mutation/);
    assert.equal(failed.capabilitiesReady, false);
    assert.equal(failed.siviLifecycleHostBusy, false);
    assert.equal(calls.filter(([call]) => call === operation).length, 2);
  }
});

test('an unresolved lifecycle peer prevents refresh from resetting its originals', async () => {
  for (const flag of ['siviCreationPending', 'siviDeletionPending', 'siviRestorationPending']) {
    let reads = 0;
    const current = host('none', { [flag]: true, loadChildData: async () => reads++ });
    await assert.rejects(current.actions.refreshSIVIChildEditors('P', 'height'), /no peer originals were reset/);
    assert.equal(reads, 0);
  }
});

test('explicit confirmation and action callbacks never write or silently confirm a changed audit action', () => {
  const calls = [];
  const deletion = { confirm: value => calls.push(['delete-confirm', value]), view: () => ({ error: null }) };
  const restoration = { choose: action => calls.push(['action', action]), confirm: value => calls.push(['restore-confirm', value]),
    view: () => ({ error: null }) };
  const current = host('none', { siviDeletionSession: deletion, siviRestorationSession: restoration });
  current.actions.stageSIVILifecycle('deletion', true);
  current.actions.stageSIVILifecycle('restoration', false, 'prune');
  current.actions.stageSIVILifecycle('restoration', true);
  assert.deepEqual(calls, [['delete-confirm', true], ['action', 'prune'], ['restore-confirm', true]]);
  current.siviDeletionEditingDisabled = true;
  current.actions.stageSIVILifecycle('deletion', false);
  assert.match(current.error, /Unlock the clean owned parent/);
  assert.equal(calls.length, 3);
});
