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
const editor = loadTypeScript('siviCollectedEditor.ts', {
  './collectedEditor': loadTypeScript('collectedEditor.ts'), './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './siviHeightEditor': height,
});
const session = loadTypeScript('siviCollectedSession.ts', {
  './siviChildWriteSession': child, './siviCollectedEditor': editor,
});
const id = '9007199254740993';
const clone = value => structuredClone(value);
function source(value = cell('null'), options = {}) {
  const result = review(options);
  for (const group of result) group.Rows[0].cells[group.Columns.indexOf('Collected')] = clone(value);
  return result;
}
function fixture(overrides = {}, notices = true) {
  let current = source(cell('null'), { extendedCoverOnly: true });
  const calls = { save: 0, read: 0, refresh: 0, restore: 0 };
  const port = {
    read: async extended => { calls.read++; current[0].Form = extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC'; return clone(current); },
    save: async (extended, json) => {
      calls.save++;
      const request = JSON.parse(json);
      for (const edit of request.edits) {
        assert.equal('value' in edit, false);
        assert.equal('column' in edit, false);
        assert.equal(edit.rowId, id);
        let next = edit.expected.storage === 'null' ? null : edit.expected.text;
        for (let count = 0; count < edit.clicks; count++) next = next === null ? 'C' : next === 'C' ? 'V' : null;
        for (const group of current) for (const row of group.Rows) if (row.rowId === edit.rowId) {
          row.cells[group.Columns.indexOf('Collected')] = next === null ? cell('null') : cell('text', next);
        }
      }
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
    restore: async (history, action) => {
      calls.restore++;
      assert.equal(history, '1');
      current = source(cell('null'), { extendedCoverOnly: true });
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  return { owner: new session.SIVICollectedSession('P', port, () => {}, false, notices), port, calls };
}

test('all source groups and variants reuse exact nullable Collected cycles without assigning raw text or application IDs', () => {
  const cases = [
    [null, ['C', 'V', null]], ['C', ['V', null, 'C']], ['V', [null, 'C', 'V']],
    ['c', ['V', null, 'C']], ['v', [null, 'C', 'V']],
    ['\uFF23', ['V', null, 'C']], ['\uFF43', ['V', null, 'C']],
    ['\uFF36', [null, 'C', 'V']], ['\uFF56', [null, 'C', 'V']],
    ['', ['', '', '']], ['X', ['X', 'X', 'X']], ['historical', ['historical', 'historical', 'historical']],
    ['x'.repeat(256), ['x'.repeat(256), 'x'.repeat(256), 'x'.repeat(256)]],
  ];
  for (const extended of [false, true]) for (const index of [0, 1, 2]) for (const [initial, expected] of cases) {
    const original = initial === null ? cell('null') : cell('text', initial);
    const projection = source(original, { extended });
    projection.forEach((group, groupIndex) => { if (groupIndex !== index) group.Rows = []; });
    let drafts = {};
    for (let clicks = 1; clicks <= 9; clicks++) {
      drafts = editor.stageSIVICollected(projection, clone(drafts), id);
      const actual = drafts[id].value.storage === 'null' ? null : drafts[id].value.text;
      assert.equal(actual, expected[(clicks - 1) % 3]);
      assert.deepEqual(clone(drafts[id].expected), original);
      assert.equal(editor.siviCollectedDirty(drafts), actual !== initial);
      const edits = clone(editor.siviCollectedEdits(projection, drafts));
      assert.deepEqual(edits, actual === initial ? [] : [{
        rowId: id, form: projection[index].Form, expected: original, clicks: (clicks - 1) % 3 + 1,
      }]);
      assert.equal(projection[index].Rows[0].rowId, id);
      assert.equal(projection[index].Rows[0].cells[0].integer, '1');
    }
  }
});

test('shared physical rows have one source owner and one cycle intent, including across shrub presentation changes', () => {
  const original = source();
  assert.equal(editor.siviCollectedRows(original).length, 1);
  assert.equal(editor.siviCollectedRows(original)[0].group.Form, 'SubVegA-SIVI_BC');
  const drafts = editor.stageSIVICollected(original, {}, id);
  const extended = height.siviHeightPresentation(original, 'P', true);
  assert.equal(editor.siviCollectedEdits(extended, drafts)[0].form, 'SubVegA-SIVI');
  const request = JSON.parse(editor.siviCollectedRequestJSON(extended, drafts));
  assert.equal(request.original.length, 3);
  assert.equal(request.edits.length, 1);
  assert.deepEqual(Object.keys(request.edits[0]).sort(), ['clicks', 'expected', 'form', 'rowId']);
  request.edits[0].expected.storage = 'foreign';
  assert.equal(drafts[id].expected.storage, 'null');
  assert.equal(original[0].Form, 'SubVegA-SIVI_BC');
});

test('malformed, non-text, foreign, stale and invented source/cycle intent fail explicitly and remain non-saveable', () => {
  for (const value of [cell('integer', '1'), cell('real', 1), cell('blob', 'ff')]) {
    assert.throws(() => editor.stageSIVICollected(source(value), {}, id), /non-text.*read-only/);
  }
  const original = source();
  assert.throws(() => editor.stageSIVICollected(original, {}, '1'), /unavailable/);
  assert.throws(() => editor.siviCollectedRequestJSON([], {}), /unavailable/);
  const missing = clone(original);
  missing[0].Rows[0].cells.pop();
  assert.throws(() => editor.stageSIVICollected(missing, {}, id), /source|incomplete/);
  const drafts = editor.stageSIVICollected(original, {}, id);
  for (const clicks of [0, -1, 4, 1.5, NaN]) {
    const invalid = clone(drafts); invalid[id].clicks = clicks;
    assert.ok(editor.siviCollectedErrors(invalid).length);
    assert.equal(editor.siviCollectedDirty(invalid), true);
    assert.throws(() => editor.siviCollectedEdits(original, invalid), /cycle intent/);
  }
  const forged = clone(drafts); forged[id].value = cell('text', 'X');
  assert.ok(editor.siviCollectedErrors(forged).length);
  assert.throws(() => editor.siviCollectedEdits(original, forged), /cycle intent/);
  assert.throws(() => editor.stageSIVICollected(source(cell('text', 'V')), drafts, id), /original changed/);
  const disagree = clone(original);
  disagree[1].Rows[0].cells[disagree[1].Columns.indexOf('Collected')] = cell('text', 'C');
  assert.throws(() => editor.stageSIVICollected(disagree, {}, id), /disagree/);
  assert.throws(() => editor.stageSIVICollected(source(cell('text', '\uD800')), {}, id), /incomplete/);
});

test('shared Collected lifecycle retains click intent/history/source notices through presentation and automatic refresh', async () => {
  const { owner, calls } = fixture();
  assert.equal(await owner.load(), true);
  owner.cycle(id);
  assert.equal(owner.view().sourceNotices.length, 1);
  assert.equal(owner.closeState().unsaved, true);
  assert.equal(owner.closeState().canSave, true);
  owner.presentation(true);
  assert.equal(owner.view().drafts[id].clicks, 1);
  assert.equal(await owner.save(), true);
  assert.equal(calls.save, 1);
  assert.equal(owner.view().historyId, '1');
  assert.equal(owner.view().sourceNoticesSaved, true);
  assert.equal(owner.view().sourceNotices[0].form, 'SubVegA-SIVI');
  assert.equal(await owner.refreshSource(), true);
  assert.equal(owner.view().sourceNoticesSaved, true);
  assert.equal(owner.closeState().unsaved, false);
  assert.equal(await owner.restore('prune'), true);
  assert.equal(owner.view().sourceNotices.length, 0);
  assert.equal(owner.view().historyId, null);
  assert.equal(calls.restore, 1);
});

test('no-op full cycles never call the writer; unknown receipts block replay until explicit Undo', async () => {
  const { owner, calls } = fixture();
  await owner.load();
  for (let count = 0; count < 3; count++) owner.cycle(id);
  assert.equal(owner.closeState().unsaved, false);
  assert.equal(await owner.save(), true);
  assert.equal(calls.save, 0);
  for (const save of [async () => null, async () => { throw new Error('lost receipt'); }]) {
    const { owner } = fixture({ save });
    await owner.load(); owner.cycle(id);
    assert.equal(await owner.save(), false);
    assert.equal(owner.closeState().blocked, true);
    assert.equal(owner.view().historyId, null);
    assert.equal(owner.view().sourceNoticesSaved, false);
    await assert.rejects(owner.save(), /do not replay/);
    assert.equal(await owner.undo(), true);
    assert.equal(owner.closeState().blocked, false);
  }
});

test('actual Collected panel preserves source groups with one labelled live cycle per shared physical field', async () => {
  const { render } = await import('svelte/server');
  const component = serverComponent(readFileSync(path.join(__dirname, 'SIVICollectedPanel.svelte'), 'utf8'), 'SIVICollectedPanel.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' } },
    './projectMetadataEditor': metadata, './siviCollectedEditor': editor,
  });
  const { owner } = fixture();
  await owner.load();
  owner.cycle(id);
  const html = render(component, { props: { view: owner.view(), disabled: false, canSave: true,
    oncycle() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {} } }).body;
  assert.equal((html.match(/data-sivi-collected-group=/g) || []).length, 3);
  assert.equal((html.match(/data-sivi-collected-identity=/g) || []).length, 1);
  assert.match(html, new RegExp(`<label for="sivi-collected-${id}"`));
  assert.match(html, /Cycle Collected \(\?\) for RAW/);
  assert.match(html, /same physical field is edited/);
  assert.match(html, /planned Save/);
  assert.doesNotMatch(html, /<(?:input|textarea)\b/);
});
