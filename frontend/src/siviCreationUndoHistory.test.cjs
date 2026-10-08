const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { restoration } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const deletion = loadTypeScript('siviDeletionHistory.ts', {
  './projectMetadataRestore': restoration, './qualityEditor': quality,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const history = loadTypeScript('siviCreationUndoHistory.ts', { './siviDeletionHistory': deletion,
  './siviRequestId': loadTypeScript('siviRequestId.ts'), './qualityEditor': quality });
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
function list() {
  return { ...owner, historyPresent: true, events: [{
    historyId: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', form: 'SubVegA-SIVI',
    rowId: '9223372036854775806', id: 1, species: 'TREE',
    actor: ' literal actor ', editWhen: '2026-10-08 10:11:12', undone: false, consumed: false,
    reviewAvailable: true, unavailableReason: null,
  }] };
}
test('creation history reuses exact owned identity/time validation without deletion facts or implicit selection', () => {
  const wire = list();
  const parsed = history.siviCreationUndoHistoryFromWire(wire, owner);
  assert.equal(Object.keys(parsed).length, 5);
  assert.equal(Object.keys(parsed.events[0]).length, 11);
  assert.equal(parsed.events[0].undone, false);
  assert.equal(parsed.events[0].restored, undefined);
  wire.events[0].species = 'changed';
  assert.equal(parsed.events[0].species, 'TREE');
  for (const present of [false, true]) {
    const empty = history.siviCreationUndoHistoryFromWire({ ...owner, historyPresent: present, events: [] }, owner);
    assert.equal(empty.historyPresent, present);
    assert.deepEqual(Array.from(empty.events), []);
  }
  const consumed = list();
  consumed.events[0].undone = consumed.events[0].consumed = true;
  consumed.events[0].reviewAvailable = false;
  consumed.events[0].unavailableReason = 'Already consumed.';
  assert.equal(history.siviCreationUndoHistoryFromWire(consumed, owner).events[0].consumed, true);
});
test('creation history rejects incomplete, foreign, malformed, mixed and unsorted source authority', () => {
  for (const mutate of [
    value => value.contextId = 'foreign', value => value.project = 'sample',
    value => value.plot = 'P ', value => value.historyPresent = false,
    value => value.extra = true, value => delete value.events[0].undone,
    value => delete value.events[0].reviewAvailable, value => delete value.events[0].unavailableReason,
    value => value.events[0].reviewAvailable = false, value => value.events[0].unavailableReason = 'unexpected',
    value => value.events[0].restored = false, value => value.events[0].consumed = true,
    value => value.events[0].id = 0, value => value.events[0].id = -1,
    value => value.events[0].id = 2147483648, value => value.events[0].species = null,
    value => value.events[0].species = '', value => value.events[0].species = '123456789',
    value => value.events[0].species = 'A\0',
    value => value.events[0].species = '\ud800', value => value.events[0].rowId = '01',
    value => value.events[0].actor = '', value => value.events[0].editWhen = '2026-02-30 10:11:12',
    value => value.events[0].historyId = value.events[0].historyId.toUpperCase(),
    value => value.events.push(structuredClone(value.events[0])),
    value => value.events.push({ ...value.events[0], historyId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      editWhen: '2026-10-09 10:11:12' }),
  ]) {
    const wire = list();
    mutate(wire);
    assert.throws(() => history.siviCreationUndoHistoryFromWire(wire, owner));
  }
});
test('coherent legacy identities stay visible and unavailable without blocking fresh UUID choices', () => {
  const wire = list();
  for (const identity of ['legacy-creation', '\ue000', '\ud800\udc00']) {
    wire.events.push({ ...wire.events[0], historyId: identity, reviewAvailable: false,
      unavailableReason: 'Legacy non-UUID creation identity.' });
  }
  const parsed = history.siviCreationUndoHistoryFromWire(wire, owner);
  assert.equal(parsed.events.length, 4);
  assert.equal(parsed.events[0].reviewAvailable, true);
  assert.equal(parsed.events[0].unavailableReason, null);
  assert.equal(parsed.events[1].historyId, 'legacy-creation');
  assert.equal(parsed.events[1].reviewAvailable, false);
  assert.match(parsed.events[1].unavailableReason, /Legacy/);
  assert.equal(parsed.events[2].historyId, '\ue000');
  assert.equal(parsed.events[3].historyId, '\ud800\udc00');
  for (const mutate of [
    value => value.events[1].reviewAvailable = true,
    value => value.events[1].unavailableReason = null,
    value => value.events[1].unavailableReason = '',
    value => value.events[1].historyId = '\ud800',
    value => value.events[1].historyId = 'A'.repeat(101),
    value => { [value.events[2], value.events[3]] = [value.events[3], value.events[2]]; },
  ]) {
    const changed = structuredClone(wire);
    mutate(changed);
    assert.throws(() => history.siviCreationUndoHistoryFromWire(changed, owner));
  }
});
