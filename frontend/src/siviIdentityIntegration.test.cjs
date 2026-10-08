const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { componentFunctions } = require('./svelteTestHelpers.cjs');
const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const main = readFileSync(path.join(__dirname, '..', '..', 'main.go'), 'utf8');
const peers = ['siviSession', 'siviCoverSession', 'siviCombinedSession', 'siviCollectedSession', 'siviSpeciesSession', 'siviIdentitySession'];
const guards = ['busy', 'headerWorkflowBusy', 'siviUnsaved', 'siviCoverUnsaved', 'siviCombinedUnsaved', 'siviCollectedUnsaved',
  'siviSpeciesUnsaved', 'heightUnsaved', 'otherUnsaved', 'soilUnsaved', 'attributeUnsaved', 'collectedUnsaved',
  'speciesUnsaved', 'siviParentWriteUnsaved', 'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved'];
function host(overrides = {}) {
  return componentFunctions('FS882Form.svelte', [
    'siviIdentityOperation', 'stageSIVIIdentity', 'showSIVIIdentityPanel', 'refreshSIVIChildEditors', 'load',
  ], {
    ...Object.fromEntries(guards.map(name => [name, false])), ...Object.fromEntries(peers.map(name => [name, null])), pictureMetadataPending: false, siviCreationPending: false,
    siviDeletionPending: false, siviRestorationPending: false, siviCreationUndoPending: false,
    siviIdentityEnabled: true, siviIdentityEditingDisabled: false, siviIdentityUnsaved: false,
    siviIdentityReading: false, siviIdentityPanelOpen: false, siviIdentityView: { review: [{}] },
    siviPanelOpen: true, siviCoverPanelOpen: true, siviCombinedPanelOpen: true,
    siviCollectedPanelOpen: true, siviSpeciesPanelOpen: true,
    siviParentSharedSession: null, siviProjectAssignmentSession: null, siviParentActionSession: null,
    siviParentWriteSession: null, siviParentSession: null, siviParentSourceSession: null, siviParentSourceView: null,
    error: null, successMsg: null, loadChildData: async () => {},
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' }, ...overrides,
  });
}
test('identity is independently default-off in actual main and uses tracked owned reads without requiring audited peers', () => {
  assert.equal(compile(parent, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(parent, /VITE_SIVI_IDENTITY_EDITING === 'true'/);
  assert.match(main, /siviFeature\(siviIdentityFeatureEnvironment, os.LookupEnv\)/);
  assert.match(main, /NewSIVIIdentityService\(contextService, os.LookupEnv\)/);
  assert.match(main, /application.NewService\(siviIdentity\)/);
  for (const method of ['GetOriginal', 'SaveReviewed', 'RestoreReviewed']) assert.ok(parent.includes(`SIVIIdentityService.${method}(`));
  assert.match(parent, /siviIdentityReads.track\(SIVIIdentityService.GetOriginal/);
  assert.match(parent, /siviIdentitySession\?\.dispose\(\);[\s\S]{0,70}siviIdentityReads\.cancelAll\(\)/);
  assert.match(parent, /data-sivi-identity-cancel-read[\s\S]{0,110}siviIdentityReads\.cancelAll/);
  assert.match(parent, /siviIdentitySession\?\.presentation\(extendedShrubs\)/);
  assert.match(parent, /siviIdentityEnabled\s*\? reads.track\(ContextService.GetVegetationReadAvailability/);
  assert.match(parent, /vegetationReadUnavailable[\s\S]*Ordinary vegetation rows, totals and actions/);
});
test('identity participates in Save Undo Lock close and every ordinary/source/parent draft barrier', () => {
  assert.match(parent, /otherHeaderWorkflowBusy = \$derived\([^\n]*\|\| siviIdentityBusy/);
  assert.match(parent, /headerWorkflowBusy = \$derived\(otherHeaderWorkflowBusy \|\| pictureBusy \|\| pictureMetadataPending\)/);
  assert.match(parent, /siviCreationPeerUnsaved = \$derived\([^\n]*\|\| siviIdentityUnsaved/);
  assert.match(parent, /nonParentChildUnsaved = \$derived\(siviCreationPeerUnsaved \|\| siviCreationPending \|\| siviDeletionPending \|\| siviRestorationPending \|\| siviCreationUndoPending\)/);
  assert.match(parent, /if \(siviIdentityUnsaved\) \{ await siviIdentityOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviIdentityUnsaved\) \{ void siviIdentityOperation\('undo'\); return; \}/);
  assert.match(parent, /\|\| \(siviIdentityClose\?\.blocked \?\? false\)/);
  assert.match(parent, /siviIdentityClose\?\.saveReason \?\? 'Correct or Undo unresolved SIVI ID drafts.'/);
  assert.match(parent, /SIVI IDs or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock/);
  for (const gate of ['height', 'other', 'soil', 'attribute', 'collected', 'species', 'sivi', 'siviCover', 'siviCombined', 'siviCollected', 'siviSpecies']) {
    assert.match(parent, new RegExp(`${gate}EditingDisabled = \\$derived\\([^\\n]*\\|\\| siviIdentityUnsaved`));
  }
});
test('actual identity operations refuse conflicts and dispatch precisely one command with accurate audit-free feedback', async () => {
  for (const guard of [...guards, 'siviIdentityEditingDisabled']) {
    let calls = 0;
    const current = host({ [guard]: true, siviIdentitySession: { save: async () => { calls++; return true; } } });
    assert.equal(await current.actions.siviIdentityOperation('save'), false, guard);
    assert.equal(calls, 0, guard);
    assert.match(current.error, /no conflicting drafts or operation/);
  }
  for (const operation of ['load', 'save', 'undo', 'retain', 'prune']) {
    let calls = 0;
    const action = async selected => {
      calls++;
      assert.equal(current.siviIdentityReading, operation === 'load');
      if (operation === 'retain' || operation === 'prune') assert.equal(selected, operation);
      return true;
    };
    const current = host({ siviIdentitySession: { load: action, save: action, undo: action, restore: action } });
    assert.equal(await current.actions.siviIdentityOperation(operation), true);
    assert.equal(calls, 1);
    assert.equal(current.siviIdentityReading, false);
    if (operation === 'save') assert.match(current.successMsg, /no source field audits/);
    if (operation === 'prune') assert.match(current.successMsg, /without source audit pruning/);
  }
});
test('identity remount retains exact raw/error/NULL state and excludes all other source panels', async () => {
  const calls = [];
  const owner = { stage: (...args) => calls.push(args), view: () => ({ error: 'retained invalid entry' }) };
  const current = host({ siviIdentitySession: owner, siviIdentityUnsaved: true, siviIdentityView: { review: null } });
  current.actions.stageSIVIIdentity('9007199254740993', 'raw ', false);
  assert.equal(calls.length, 1);
  assert.deepEqual(Array.from(calls[0]), ['9007199254740993', 'ID', 'raw ', false]);
  assert.match(current.error, /retained invalid/);
  await current.actions.showSIVIIdentityPanel();
  assert.equal(current.siviIdentityPanelOpen, true);
  for (const panel of ['siviPanelOpen', 'siviCoverPanelOpen', 'siviCombinedPanelOpen', 'siviCollectedPanelOpen', 'siviSpeciesPanelOpen']) {
    assert.equal(current[panel], false);
  }
  await current.actions.showSIVIIdentityPanel();
  assert.equal(current.siviIdentitySession, owner);
  current.siviIdentityEditingDisabled = true;
  current.actions.stageSIVIIdentity('9007199254740993', '2', true);
  assert.equal(calls.length, 1);
  assert.match(current.error, /draft failed.*Unlock/);
});
test('busy/unresolved identity owners cannot be replaced and late acknowledgements cannot overwrite a replacement', async () => {
  for (const state of [{ unsaved: true }, { busy: true }, { unsaved: true, blocked: true }]) {
    let disposed = 0;
    const owner = { closeState: () => state };
    const current = host({ siviIdentitySession: owner, siviParentSession: { dispose: () => { disposed++; } } });
    await current.actions.load('another');
    assert.equal(current.siviIdentitySession, owner);
    assert.equal(disposed, 0);
    assert.match(current.error, /before replacing their plot owner/);
  }
  let deliver;
  const current = host({ siviIdentitySession: { load: () => new Promise(resolve => { deliver = resolve; }) } });
  const pending = current.actions.siviIdentityOperation('load');
  current.siviIdentitySession = {};
  current.error = 'replacement error';
  deliver(true);
  assert.equal(await pending, false);
  assert.equal(current.error, 'replacement error');
});
test('each source child refreshes all five other physical editors and refuses unresolved peer replacement', async () => {
  for (const owner of ['height', 'cover', 'combined', 'collected', 'species', 'identity']) {
    const changed = [];
    const current = host(Object.fromEntries(peers.map((name, index) => [name, {
      closeState: () => ({ unsaved: false, busy: false }), view: () => ({ review: [{}] }),
      refreshSource: async () => { changed.push(index); return true; },
    }])));
    await current.actions.refreshSIVIChildEditors('P', owner);
    const index = ['height', 'cover', 'combined', 'collected', 'species', 'identity'].indexOf(owner);
    assert.deepEqual(changed, [0, 1, 2, 3, 4, 5].filter(value => value !== index));
  }
  const current = host({ siviIdentitySession: { closeState: () => ({ unsaved: true }) } });
  await assert.rejects(current.actions.refreshSIVIChildEditors('P', 'height'), /unresolved drafts/);
});
