const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { render } = require('svelte/server');
const { componentFunctions, serverComponent, loadTypeScript } = require('./svelteTestHelpers.cjs');
const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const token = '00000000-0000-4000-8000-000000000001';
const flags = ['busy', 'headerWorkflowBusy', 'siviSourceAuthorityUnknown', 'codeCheckOpen', 'metadataOpen',
  'profileReviewBlocked', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentWriteUnsaved',
  'siviParentSharedUnsaved', 'siviUnsaved', 'siviCoverUnsaved', 'siviCombinedUnsaved', 'siviCollectedUnsaved',
  'siviSpeciesUnsaved', 'siviIdentityUnsaved', 'heightUnsaved', 'otherUnsaved', 'soilUnsaved',
  'attributeUnsaved', 'collectedUnsaved', 'speciesUnsaved', 'pictureMetadataPending', 'twoPageOpen', 'environmentSUOpen',
  'siviDeletionPending', 'siviRestorationPending', 'siviCreationUndoPending'];
function host(overrides = {}) {
  return componentFunctions('FS882Form.svelte', ['siviCreationOperation', 'stageSIVICreation',
    'refreshSIVIChildEditors', 'refreshSIVILifecycleLists', 'save', 'undo', 'getCloseState', 'load', 'toggleLock'], {
    ...Object.fromEntries(flags.map(name => [name, false])),
    siviCreationEnabled: true, siviCreationRecoveryDisabled: false, siviCreationEditingDisabled: false,
    siviCreationPending: true, siviCreationHostBusy: false, siviCreationSession: null, siviCreationUndoSession: null,
    siviDeletionSession: null, siviRestorationSession: null,
    siviCreationClose: { unsaved: true, busy: false, blocked: true, canSave: false, saveReason: 'Correct invalid cover.', error: 'raw 100' },
    siviParentSourceView: null, personalDraft: null, deletionReview: null, creationDraft: null,
    siviSession: null, siviCoverSession: null, siviCombinedSession: null, siviCollectedSession: null,
    siviSpeciesSession: null, siviIdentitySession: null, draft: { locked: false, plotNumber: 'P' },
    error: null, successMsg: null, capabilitiesReady: true, crypto: { randomUUID: () => token }, loadChildData: async () => {},
    ...overrides,
  });
}
function derived(name, state) {
  const expression = root.match(new RegExp(`const ${name} = \\$derived\\(([\\s\\S]*?)\\);`));
  assert.ok(expression, name);
  return vm.runInNewContext(expression[1], state);
}

test('actual client subscription settles, notifies once and detaches when its owner changes', async () => {
  const client = await import('svelte/internal/client');
  const { compile } = require('svelte/compiler');
  for (const prefix of ['siviCreation', 'pictureMetadata', 'siviDeletion', 'siviRestoration', 'siviCreationUndo']) {
    const subscription = root.split('\n').find(line => line.includes(`${prefix}Session?.subscribe`));
    assert.ok(subscription, prefix);
    const compiled = compile(`<script>
      import { untrack } from 'svelte';
      let { ${prefix}Session } = $props();
      let ${prefix}Revision = $state(0);
      ${subscription}
      $effect(() => { ${prefix}Revision; ${prefix}Session?.view(); });
    </script>`, { generate: 'client' }).js.code;
    const harness = new Function('$', 'untrack', compiled.replace(/^import .*$/gm, '')
      .replace('export default function', 'function') + '\nreturn _unknown_;')(client, client.untrack);
    function session() {
      const listeners = new Set();
      return {
        subscriptions: 0, cleanups: 0, views: 0,
        subscribe(notify) {
          assert.ok(++this.subscriptions <= 5, 'Subscription recursively invalidated itself');
          listeners.add(notify);
          notify();
          return () => { this.cleanups++; listeners.delete(notify); };
        },
        view() { this.views++; },
        notify() { for (const notify of listeners) notify(); },
      };
    }
    const first = session(), second = session(), owner = client.state(first);
    const dispose = client.effect_root(() => harness(null, {
      get [prefix + 'Session']() { return client.get(owner); },
    }));
    try {
      client.flush();
      assert.equal(first.subscriptions, 1);
      const views = first.views;
      first.notify();
      client.flush();
      assert.equal(first.subscriptions, 1);
      assert.equal(first.views, views + 1);
      client.set(owner, second);
      client.flush();
      assert.equal(first.cleanups, 1);
      assert.equal(second.subscriptions, 1);
      first.notify();
      client.flush();
      assert.equal(first.views, views + 1);
      assert.equal(second.subscriptions, 1);
    } finally { dispose(); }
    assert.equal(second.cleanups, 1);
  }
});

test('creation independently gates actual root/main registration and binds only owned refs/Create/readonly lookup', async () => {
  const main = readFileSync(path.join(__dirname, '..', '..', 'main.go'), 'utf8');
  assert.match(root, /VITE_SIVI_CREATION_WRITING === 'true'/);
  assert.match(main, /NewSIVICreationService\(contextService, os.LookupEnv\)/);
  assert.match(main, /application.NewService\(siviCreation\)/);
  const calls = [];
  class Session { constructor(owner, port) { this.owner = owner; this.port = port; } }
  const cache = loadTypeScript('siviCreationSessions.ts', {
    '../bindings/github.com/boostao/vpro-wails': { SIVICreationService: {
      GetReferences: (...args) => calls.push(['references', ...args]),
      Create: (...args) => calls.push(['create', ...args]),
      LookupReceipt: (...args) => calls.push(['lookup', ...args]),
    } }, './siviCreationSession': { SIVICreationSession: Session },
  });
  const first = cache.siviCreationSession(owner);
  assert.equal(cache.siviCreationSession({ ...owner }), first);
  for (const foreign of [{ ...owner, contextId: 'Second' }, { ...owner, project: 'Second' }, { ...owner, plot: 'Second' }]) {
    assert.notEqual(cache.siviCreationSession(foreign), first);
  }
  first.port.references(); first.port.create('literal'); first.port.receipt('stable');
  assert.deepEqual(calls, [['references', 'C', 'P'], ['create', 'C', 'literal'], ['lookup', 'C', 'stable']]);
});

test('shared pending guards preserve ordinary/source/parent barriers while draft-only layout remount stays available', () => {
  const state = { siviCreationPeerUnsaved: false, siviCreationPending: true, siviDeletionPending: false, siviRestorationPending: false, siviCreationUndoPending: false,
    capabilitiesReady: true, draft: { locked: false }, busy: false, headerWorkflowBusy: false,
    siviSourceBarrier: false, dirty: false, original: {}, deletionReview: null, creationDraft: null,
    personalDraft: null, codeCheckOpen: false, metadataOpen: false, profileReviewBlocked: false,
    siviParentWriteUnsaved: false, siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false,
    siviParentSharedUnsaved: false, childUnsaved: true, pictureBusy: false, pictureMetadataPending: false,
    otherHeaderWorkflowBusy: false, pictureMetadataClose: null };
  assert.equal(derived('nonParentChildUnsaved', state), true);
  assert.equal(derived('headerInputsDisabled', state), true);
  assert.equal(derived('childParentDisabled', state), true);
  assert.equal(derived('tabWorkflowBusy', state), false);
  assert.equal(derived('nonParentChildUnsaved', { ...state, siviCreationPending: false }), false);
  assert.equal(derived('nonParentChildUnsaved', { ...state, siviCreationPending: false, siviCreationPeerUnsaved: true }), true);
});

test('unknown request recovery permission is separate from ordinary editing and retains every peer-operation guard', () => {
  const state = { siviCreationEnabled: true, busy: false, siviCreationHostBusy: false, siviDeletionPending: false, siviRestorationPending: false, siviCreationUndoPending: false,
    otherHeaderWorkflowBusy: false, siviCreationClose: { busy: false }, pictureBusy: false,
    pictureMetadataPending: false, siviCreationPeerUnsaved: false, siviParentWriteUnsaved: false,
    siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false, siviParentSharedUnsaved: false };
  assert.equal(derived('siviCreationRecoveryDisabled', state), false);
  for (const name of ['busy', 'siviCreationHostBusy', 'otherHeaderWorkflowBusy', 'pictureBusy', 'pictureMetadataPending',
    'siviCreationPeerUnsaved', 'siviParentWriteUnsaved', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved']) {
    assert.equal(derived('siviCreationRecoveryDisabled', { ...state, [name]: true }), true, name);
  }
  assert.equal(derived('siviCreationRecoveryDisabled', { ...state, siviCreationEnabled: false }), true);
});

test('retained creation invalid/unknown state blocks parent close, owner replacement and Lock without erasing its error', async () => {
  const current = host();
  const close = current.actions.getCloseState();
  assert.equal(close.unsaved, true);
  assert.equal(close.blocked, true);
  assert.equal(close.canSave, false);
  assert.equal(close.error, 'raw 100');
  await current.actions.load('replacement');
  assert.match(current.error, /before replacing the plot owner/);
  await current.actions.toggleLock();
  assert.match(current.error, /unknown acknowledgements.*before changing the plot lock/);
  assert.equal(current.siviCreationPending, true);
  assert.equal(current.siviCreationClose.error, 'raw 100');
});

test('actual root creation dispatches one Save or readonly resolution, refreshes peers and never repeats a verified commit', async () => {
  for (const operation of ['save', 'resolve']) {
    const calls = [], view = { revision: 0, receipt: null, error: null };
    const session = { view: () => view,
      save: async id => { calls.push(['save', id]); view.revision++; view.receipt = { id: 42, plot: 'P', replayed: false }; return true; },
      resolve: async () => { calls.push(['lookup']); view.revision++; view.receipt = { id: 42, plot: 'P', replayed: true }; return true; },
    };
    const current = host({ siviCreationSession: session, loadChildData: async plot => calls.push(['refresh', plot]) });
    assert.equal(await current.actions.siviCreationOperation(operation), true);
    assert.deepEqual(calls, [operation === 'save' ? ['save', token] : ['lookup'], ['refresh', 'P']]);
    assert.match(current.successMsg, operation === 'save' ? /one row committed/ : /no mutation repeated/);
    assert.equal(current.siviCreationHostBusy, false);
  }
  const view = { revision: 0, receipt: null, error: null }; let writes = 0;
  const session = { view: () => view, save: async () => { writes++; view.revision++; view.receipt = { id: 42, plot: 'P' }; return true; } };
  const current = host({ siviCreationSession: session, loadChildData: async () => { throw new Error('peer unavailable'); } });
  assert.equal(await current.actions.siviCreationOperation('save'), false);
  assert.equal(writes, 1);
  assert.match(current.error, /already verified.*without repeating Create/);
  assert.equal(view.receipt.id, 42);
  assert.equal(current.siviCreationHostBusy, false);
});

test('generic parent Save and Undo route to exactly the retained creator, with explicit unknown refusal', async () => {
  let saved = 0, undone = 0;
  const view = { revision: 0, receipt: null, error: null };
  const session = { view: () => view, save: async () => { saved++; view.revision++; view.receipt = { id: 42, plot: 'P' }; return true; },
    discard: () => { undone++; }, updateDraft: () => { throw new Error('unknown request'); } };
  const current = host({ siviCreationSession: session });
  await current.actions.save();
  assert.equal(saved, 1);
  current.actions.undo();
  assert.equal(undone, 1);
  current.actions.stageSIVICreation({});
  assert.match(current.error, /retained draft unchanged.*unknown request/);
});

test('actual creation commands fail closed when unavailable/conflicted, and cancellation stays reachable without replacing owner', async () => {
  let calls = 0;
  const session = { view: () => ({ revision: 0 }), save: async () => { calls++; return true; }, cancel: () => { calls++; } };
  for (const barrier of ['siviCreationRecoveryDisabled', 'siviCreationEditingDisabled']) {
    const current = host({ siviCreationSession: session, [barrier]: true });
    assert.equal(await current.actions.siviCreationOperation('save'), false);
    assert.equal(calls, 0);
  }
  const unavailable = host({ siviCreationSession: session, siviCreationEnabled: false });
  assert.equal(await unavailable.actions.siviCreationOperation('save'), false);
  const cancel = host({ siviCreationSession: session, siviCreationRecoveryDisabled: true });
  assert.equal(await cancel.actions.siviCreationOperation('cancel'), true);
  assert.equal(calls, 1);
  assert.equal(cancel.siviCreationSession, session);
});

test('panel remount keeps safety ahead of guidance and unknown recovery reachable outside transient review', () => {
  const fields = serverComponent('<script lang="ts">let {draft} = $props();</script><div data-sivi-creation-fields>{draft.species}</div>', 'CreationFieldsMock.svelte', {});
  const component = serverComponent(readFileSync(path.join(__dirname, 'SIVICreationPanel.svelte'), 'utf8'),
    'SIVICreationPanel.svelte', { './SIVICreationFields.svelte': { default: fields } });
  const view = { draft: { ...owner, form: 'SubVegA-SIVI', species: 'TREE' }, references: { ...owner },
    busy: false, blocked: true, error: 'retained unknown', requestId: token, receipt: null };
  const output = render(component, { props: { view, close: { canSave: false }, owner, disabled: true,
    recoveryDisabled: false, onoperation() {}, onchange() {} } }).body;
  assert.ok(output.indexOf('retained unknown') < output.indexOf('Creation assigns only'));
  assert.match(output, /data-sivi-creation-fields/);
  assert.match(output, /data-sivi-creation-resolve(?:(?!<\/button>)[\s\S])*read-only/);
  assert.doesNotMatch(output, /data-sivi-creation-resolve[^>]*\sdisabled(?:=|\s|>)/);
  for (const button of ['save', 'undo']) assert.match(output, new RegExp(`data-sivi-creation-${button}[^>]*\\sdisabled(?:=|\\s|>)`));
  assert.match(root, /siviCreationSession\?\.subscribe/);
  assert.match(root, /<SIVICreationPanel view=\{siviCreationView\}/);
});
