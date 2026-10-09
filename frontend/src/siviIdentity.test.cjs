const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { cell, review } = require('./siviSourceFixture.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality, '../../resources/project-metadata-standard.json': [], '../../resources/project-metadata-template.json': [],
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const heightSession = loadTypeScript('siviHeightSession.ts', {
  './siviHeightEditor': height, './projectMetadataRestore': restoration,
});
const child = loadTypeScript('siviChildWriteSession.ts', {
  './projectMetadataRestore': restoration, './siviHeightEditor': height, './siviHeightSession': heightSession,
});
const editor = loadTypeScript('siviIdentityEditor.ts', {
  './qualityEditor': quality, './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './siviHeightEditor': height,
});
const session = loadTypeScript('siviIdentitySession.ts', {
  './siviChildWriteSession': child, './siviIdentityEditor': editor,
});
const availability = loadTypeScript('vegetationReadAvailability.ts');
const id = '9007199254740993';
const clone = value => structuredClone(value);
function source(value = cell('integer', '1')) {
  const result = review();
  for (const group of result) group.Rows[0].cells[0] = clone(value);
  return result;
}
function fixture(overrides = {}) {
  let current = source();
  const calls = { save: 0, read: 0, refresh: 0, restore: 0 };
  const port = {
    read: async extended => { calls.read++; current[0].Form = extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC'; return clone(current); },
    save: async (extended, json) => {
      calls.save++;
      const request = JSON.parse(json);
      assert.equal(request.edits.length, 1);
      assert.equal(request.edits[0].column, 'ID');
      assert.equal(request.edits[0].rowId, id);
      for (const group of current) group.Rows[0].cells[0] = clone(request.edits[0].value);
      return { ChangedCells: 1, HistoryID: '1' };
    },
    restore: async (history, action) => {
      calls.restore++;
      assert.equal(history, '1');
      assert.ok(['retain', 'prune'].includes(action));
      current = source();
      return { cancelled: false, restoredRows: 1, prunedAuditRows: 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  return { owner: new session.SIVIIdentitySession('P', port, () => {}), calls };
}
test('nullable exact signed32 IDs preserve physical signed64 rows and all shared source contexts', () => {
  for (const raw of ['-2147483648', '2147483647', '-1', '0', '2']) {
    const original = source();
    const draft = editor.stageSIVIIdentity(original, {}, id, raw, false);
    assert.equal(editor.siviIdentityErrors(draft).length, 0);
    const request = JSON.parse(editor.siviIdentityRequestJSON(original, draft));
    assert.equal(request.edits.length, 1);
    assert.equal(request.edits[0].rowId, id);
    assert.deepEqual(request.edits[0].value, cell('integer', raw));
    assert.equal(editor.siviIdentityRows(original).length, 1);
    assert.equal(editor.siviIdentityRows(original)[0].contexts.length, 3);
    assert.equal(original[0].Rows[0].cells[0].integer, '1');
  }
  const draft = editor.stageSIVIIdentity(source(), {}, id, '', true);
  assert.deepEqual(clone(draft[id].value), cell('null'));
  assert.equal(editor.siviIdentityDirty(draft), true);
});
test('malformed and noncanonical ID inputs retain raw errors until valid correction or Undo', async () => {
  for (const raw of ['', ' 2', '2 ', '+2', '-0', '02', '1e0', '1.0', '2147483648', '-2147483649', '\ud800']) {
    const { owner, calls } = fixture();
    assert.equal(await owner.load(), true);
    owner.stage(id, 'ID', raw, false);
    assert.equal(owner.view().drafts[id].raw, raw);
    assert.equal(owner.closeState().blocked, true);
    assert.equal(owner.closeState().canSave, false);
    owner.presentation(true);
    assert.equal(owner.view().drafts[id].raw, raw);
    await assert.rejects(owner.save(), /Correct invalid/);
    assert.equal(calls.save, 0);
    owner.stage(id, 'ID', '2', false);
    assert.equal(owner.closeState().blocked, false);
    assert.equal(await owner.undo(), true);
    assert.deepEqual(owner.view().drafts, {});
  }
});
test('historical invalid ID omissions remain unchanged and unsupported correction is read-only', () => {
  for (const value of [cell('text', 'historical'), cell('integer', '2147483648'), cell('real', 1)]) {
    const original = source(value);
    const unchanged = editor.stageSIVIIdentity(original, {}, id, metadata.metadataCellText(value), false);
    assert.equal(editor.siviIdentityEdits(original, unchanged).length, 0);
    assert.equal(editor.siviIdentityDirty(unchanged), false);
    assert.equal(editor.siviIdentityEditable(value), false);
    const changed = editor.stageSIVIIdentity(original, {}, id, '2', false);
    assert.match(editor.siviIdentityErrors(changed)[0], /read-only/);
  }
});
test('duplicate draft IDs, visible collisions, swaps and stale originals block Save across remounts', async () => {
  const original = source();
  for (const group of original) {
    const peer = clone(group.Rows[0]);
    peer.rowId = '9007199254740994';
    peer.cells[0] = cell('integer', '2');
    group.Rows.push(peer);
  }
  const { owner, calls } = fixture({ read: async extended => clone(height.siviHeightPresentation(original, 'P', extended)) });
  await owner.load();
  owner.stage(id, 'ID', '2', false);
  assert.equal(owner.closeState().blocked, true);
  assert.match(owner.closeState().saveReason, /swaps/);
  owner.presentation(true);
  assert.equal(owner.view().drafts[id].raw, '2');
  await assert.rejects(owner.save(), /swaps/);
  owner.stage(id, 'ID', '3', false);
  owner.stage('9007199254740994', 'ID', '3', false);
  assert.match(owner.closeState().saveReason, /repeated/);
  assert.equal(owner.closeState().blocked, true);
  owner.stage('9007199254740994', 'ID', '4', false);
  assert.equal(owner.closeState().blocked, false);
  assert.equal(calls.save, 0);
  const drafts = editor.stageSIVIIdentity(original, {}, id, '3', false);
  const drift = clone(original);
  for (const group of drift) group.Rows[0].cells[0] = cell('integer', '5');
  assert.match(editor.siviIdentityErrors(drafts, drift)[0], /original ID changed/);
  assert.throws(() => editor.stageSIVIIdentity(drift, drafts, id, '6', false), /original ID changed/);
  const damaged = clone(drafts);
  damaged[id].value.integer = '9';
  assert.match(editor.siviIdentityErrors(damaged)[0], /retained raw intent/);
});
test('ordinary availability distinguishes empty data from NULL/duplicate/unsafe identity and refuses malformed acknowledgements', () => {
  assert.deepEqual(clone(availability.vegetationReadAvailability({ available: true, reason: '', records: null })),
    { records: [], reason: null });
  assert.deepEqual(clone(availability.vegetationReadAvailability({ available: false, reason: 'NULL ID', records: null })),
    { records: [], reason: 'NULL ID' });
  const records = [{ id: 0 }, { id: -1 }];
  assert.equal(availability.vegetationReadAvailability({ available: true, reason: '', records }).records, records);
  for (const result of [
    null, { available: false, reason: '', records: null }, { available: true, reason: 'failure', records: [] },
    { available: true, reason: '' }, { available: false, reason: 'NULL', records: [] },
    { available: true, reason: '', records: [{ id: 1 }, { id: 1 }] },
    { available: true, reason: '', records: [{ id: 9007199254740992 }] },
  ]) assert.throws(() => availability.vegetationReadAvailability(result), /incomplete|unsupported/);
});
test('identity Save and both restoration actions require technical history but no source audit pruning', async () => {
  for (const action of ['retain', 'prune']) {
    const { owner, calls } = fixture();
    assert.equal(await owner.load(), true);
    owner.stage(id, 'ID', '2', false);
    assert.equal(await owner.save(), true);
    assert.equal(owner.view().historyId, '1');
    assert.equal(owner.closeState().unsaved, false);
    assert.equal(await owner.restore(action), true);
    assert.equal(owner.view().historyId, null);
    assert.equal(calls.save, 1);
    assert.equal(calls.restore, 1);
    assert.equal(calls.refresh, 2);
  }
  const { owner } = fixture({ save: async () => ({ ChangedCells: 1, HistoryID: '' }) });
  await owner.load();
  owner.stage(id, 'ID', '2', false);
  assert.equal(await owner.save(), false);
  assert.equal(owner.closeState().blocked, true);
  assert.match(owner.view().error, /incomplete/);
});
test('identity restoration rejects source audit pruning while legacy audited scopes still require exact prune counts', async () => {
  for (const auditFreeHistory of [true, false]) {
    for (const prunedAuditRows of [0, 1, 2]) {
      const owner = new child.SIVIChildWriteSession('P', {
        read: async () => source(), save: async () => ({ ChangedCells: 1, HistoryID: '1' }),
        restore: async () => ({ cancelled: false, restoredRows: 1, prunedAuditRows, cleanedVegRows: 0 }),
        refreshParent: async () => {},
      }, () => {}, {
        label: 'contract', pluralLabel: 'drafts', empty: () => ({}),
        stage: () => ({ changed: true }), errors: () => [], dirty: drafts => !!drafts.changed,
        hidden: () => false, count: () => 1, requestJSON: () => '{}', auditFreeHistory,
      });
      await owner.load();
      owner.stage(id, 'ID', '2', false);
      assert.equal(await owner.save(), true);
      const expected = prunedAuditRows === (auditFreeHistory ? 0 : 1);
      assert.equal(await owner.restore('prune'), expected);
      assert.equal(owner.closeState().blocked, !expected);
      if (!expected) assert.match(owner.view().error, /incomplete.*never replay/);
    }
  }
});
test('lost acknowledgements, refresh failures and disposed late reads remain explicit no-replay barriers', async () => {
  for (const override of [
    { save: async () => { throw new Error('lost receipt'); } },
    { refreshParent: async () => { throw new Error('peer refresh failed'); } },
  ]) {
    const { owner } = fixture(override);
    await owner.load();
    owner.stage(id, 'ID', '2', false);
    assert.equal(await owner.save(), false);
    assert.equal(owner.closeState().unsaved, true);
    assert.equal(owner.closeState().blocked, true);
    assert.match(owner.view().error, /do not replay/);
  }
  let deliver;
  const { owner } = fixture({ read: () => new Promise(resolve => { deliver = resolve; }) });
  const pending = owner.load();
  owner.dispose();
  deliver(source());
  assert.equal(await pending, false);
  assert.equal(owner.view().review, null);
});
test('identity panel has one labelled physical field, NULL choice and honest audit-free restoration guidance', () => {
  const component = serverComponent(readFileSync(path.join(__dirname, 'SIVIIdentityPanel.svelte'), 'utf8'), 'SIVIIdentityPanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' } },
    './projectMetadataEditor': metadata, './siviIdentityEditor': editor,
  });
  const { render } = require('svelte/server');
  const html = render(component, { props: {
    view: { review: source(), drafts: {}, busy: false, blocked: false, error: null, historyId: '1', sourceNotices: [] },
    disabled: false, canSave: true, onstage() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {},
  } }).body;
  assert.equal((html.match(/data-sivi-identity-field=/g) || []).length, 1);
  assert.match(html, new RegExp(`for="sivi-identity-${id}"`));
  assert.match(html, /Tree\/Shrubs, Herb, Moss\/Lichen/);
  assert.match(html, /not ordinary field audits/);
  assert.match(html, /does not prune source audits or delete vegetation/);
  assert.match(html, /data-sivi-identity-restore="prune"/);
});
