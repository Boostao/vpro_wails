const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { cell, review } = require('./siviSourceFixture.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality, '../../resources/project-metadata-standard.json': [],
  '../../resources/project-metadata-template.json': [],
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
const cover = loadTypeScript('siviCoverEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'), './siviHeightEditor': height,
});
const child = loadTypeScript('siviChildWriteSession.ts', {
  './projectMetadataRestore': restoration, './siviHeightEditor': height, './siviHeightSession': heightSession,
});
const editor = loadTypeScript('siviCombinedEditor.ts', { './siviCoverEditor': cover, './siviHeightEditor': height });
const session = loadTypeScript('siviCombinedSession.ts', {
  './siviChildWriteSession': child, './siviCombinedEditor': editor,
});
const sourceNotices = loadTypeScript('siviChildSourceNotices.ts', {
  './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
});
const coverSession = loadTypeScript('siviCoverSession.ts', {
  './siviChildWriteSession': child, './siviCoverEditor': cover, './siviChildSourceNotices': sourceNotices,
});
const id = '9007199254740993';
const clone = value => structuredClone(value);
const empty = () => ({ covers: {}, heights: {} });
const at = (source, index, column) => source[index].Rows[0].cells[source[index].Columns.indexOf(column)];
const set = (source, index, column, value) => { source[index].Rows[0].cells[source[index].Columns.indexOf(column)] = value; };
function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
function fixture(overrides = {}, extended = false, sourceNotices = false) {
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, request: null };
  let source = review({ extended });
  const port = {
    read: async presentation => {
      calls.read++;
      source[0].Form = presentation ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC';
      return clone(source).map(group => ({ ...group, Rows: group.Rows.filter(row => group.Columns.some((column, index) =>
        (column.startsWith('Cover') || column.startsWith('Total')) && row.cells[index].storage !== 'null')) }));
    },
    save: async (presentation, json) => {
      calls.save++;
      const request = JSON.parse(json);
      calls.request = request;
      assert.equal(request.original[0].Form, presentation ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC');
      for (const edit of request.edits) {
        for (const group of source) for (const row of group.Rows) {
          const index = group.Columns.indexOf(edit.column);
          if (row.rowId === edit.rowId && index >= 0) row.cells[index] = clone(edit.value);
        }
      }
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
    restore: async (history, action) => {
      calls.restore++; assert.equal(history, '1');
      source = review({ extended });
      return { cancelled: false, restoredRows: calls.request.edits.length,
        prunedAuditRows: action === 'prune' ? calls.request.edits.length : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  return { owner: new session.SIVICombinedSession('P', port, () => {}, extended, sourceNotices), port, calls };
}

test('combined source groups pair heights with their own covers without invented columns or membership', () => {
  for (const extended of [false, true]) {
    const columns = [0, 1, 2].flatMap(index => editor.siviCombinedGroups(index, extended).flat());
    assert.equal(columns.length, extended ? 17 : 14);
    assert.equal(new Set(columns).size, columns.length);
    assert.deepEqual(clone(editor.siviCombinedGroups(1, extended)), [['Cover6', 'Height6']]);
    assert.deepEqual(clone(editor.siviCombinedGroups(2, extended)), [['Cover7', 'Cover8', 'Cover9']]);
    assert.throws(() => editor.stageSIVICombined(review({ extended }), empty(), id, 'Species', 'X', false), /source-bound/);
    assert.throws(() => editor.stageSIVICombined(review({ extended }), empty(), '404', 'HeightA', '1', false), /unavailable/);
  }
});

test('all combined controls reuse exact cover/height numeric and literal domains', () => {
  const source = review({ extended: true });
  for (const column of cover.siviCoverColumns) {
    const rowId = source[column === 'Cover6' ? 1 : ['Cover7', 'Cover8', 'Cover9'].includes(column) ? 2 : 0].Rows[0].rowId;
    const drafts = editor.stageSIVICombined(source, empty(), rowId, column, '100', false);
    assert.match(editor.siviCombinedErrors(drafts)[0], /less than 100/);
  }
  for (const column of ['HeightA', 'Height6']) {
    const rowId = source[column === 'Height6' ? 1 : 0].Rows[0].rowId;
    const drafts = editor.stageSIVICombined(source, empty(), rowId, column, '100', false);
    assert.deepEqual(clone(editor.siviCombinedErrors(drafts)), []);
    assert.equal(drafts.heights[rowId][column].value.real, 100);
  }
  for (const [raw, invalid] of [['', false], [' '.repeat(255), false], ['😀'.repeat(127) + 'x', false],
    ['😀'.repeat(128), true], ['\ud800', true]]) {
    const drafts = editor.stageSIVICombined(source, empty(), id, 'HeightB', raw, false);
    assert.equal(editor.siviCombinedErrors(drafts).length > 0, invalid);
    if (!invalid) assert.equal(drafts.heights[id].HeightB.value.text, raw);
  }
  const cleared = editor.stageSIVICombined(source, empty(), id, 'HeightB', '', true);
  assert.equal(cleared.heights[id].HeightB.value.storage, 'null');
});

test('mixed request preserves one complete original and omits unchanged historical invalid assignments', () => {
  const source = review({ extended: true });
  set(source, 0, 'Cover2', cell('text', 'historical'));
  set(source, 0, 'HeightB', cell('text', 'x'.repeat(256)));
  let drafts = editor.stageSIVICombined(source, empty(), id, 'Cover2', 'historical', false);
  drafts = editor.stageSIVICombined(source, drafts, id, 'HeightB', 'x'.repeat(256), false);
  drafts = editor.stageSIVICombined(source, drafts, id, 'Cover1', '7', false);
  drafts = editor.stageSIVICombined(source, drafts, id, 'HeightA', '100', false);
  const request = JSON.parse(editor.siviCombinedRequestJSON(source, drafts));
  assert.deepEqual(request.original, source);
  assert.deepEqual(request.edits.map(edit => edit.column), ['Cover1', 'HeightA']);
  assert.equal(request.edits[0].rowId, id);
  assert.equal(request.edits[0].form, 'SubVegA-SIVI');
});

test('one combined Save spans both domains and owns only its verified combined restoration history', async () => {
  for (const action of ['retain', 'prune']) {
    const { owner, calls } = fixture();
    assert.equal(await owner.load(), true);
    owner.stage(id, 'Cover1', '7', false);
    owner.stage(id, 'HeightA', '100', false);
    assert.equal(await owner.save(), true);
    assert.equal(calls.save, 1);
    assert.equal(calls.request.edits.length, 2);
    assert.equal(owner.view().historyId, '1');
    assert.equal(await owner.restore(action), true);
    assert.equal(calls.restore, 1);
    assert.equal(owner.view().historyId, null);
    assert.equal(owner.closeState().unsaved, false);
  }
});

test('invalid cross-domain and hidden extended drafts persist through presentation and block all writes', async () => {
  const { owner, calls } = fixture({}, true);
  await owner.load();
  owner.stage(id, 'Cover5a', '3', false);
  owner.stage(id, 'HeightB', 'x'.repeat(256), false);
  owner.presentation(false);
  assert.equal(owner.closeState().blocked, true);
  assert.equal(owner.closeState().unsaved, true);
  assert.match(owner.view().drafts.heights[id].HeightB.error, /255/);
  await assert.rejects(owner.save(), /extended/);
  owner.presentation(true);
  await assert.rejects(owner.save(), /invalid/);
  assert.equal(calls.save, 0);
  owner.stage(id, 'HeightB', ' corrected ', false);
  assert.equal(await owner.save(), true);
  assert.equal(calls.request.edits.length, 2);
});

test('rejected/lost/malformed combined Save receipts block replay and require explicit recovery', async () => {
  for (const save of [
    async () => { throw new Error('transport rejected'); },
    async () => null,
    async () => ({ ChangedCells: 1, HistoryID: '1' }),
  ]) {
    const { owner } = fixture({ save });
    await owner.load();
    owner.stage(id, 'Cover1', '7', false);
    owner.stage(id, 'HeightA', '100', false);
    assert.equal(await owner.save(), false);
    assert.equal(owner.closeState().blocked, true);
    assert.equal(owner.view().review, null);
    assert.match(owner.view().error, /do not replay/);
    await assert.rejects(owner.save(), /do not replay/);
    assert.equal(await owner.undo(), true);
    assert.equal(owner.closeState().blocked, false);
  }
});

test('verified mixed commit retains history after failed peer refresh and never replays its write', async () => {
  const { owner, port, calls } = fixture({ refreshParent: async () => { throw new Error('peer failed'); } });
  await owner.load();
  owner.stage(id, 'HeightA', '100', false);
  assert.equal(await owner.save(), false);
  assert.equal(owner.view().historyId, '1');
  assert.equal(owner.closeState().blocked, true);
  port.refreshParent = async () => {};
  assert.equal(await owner.undo(), true);
  assert.equal(owner.view().historyId, '1');
  assert.equal(calls.save, 1);
});

test('disposed late combined reads cannot populate originals or notify another owner', async () => {
  const pending = deferred();
  const { owner } = fixture({ read: () => pending.promise });
  const loading = owner.load();
  owner.dispose();
  pending.resolve(review());
  assert.equal(await loading, false);
  assert.equal(owner.view().review, null);
  assert.throws(() => owner.stage(id, 'HeightA', '1', false), /ownership ended/);
});

test('actual combined Svelte output pairs all 14/17 live controls with visible labels and distinct NULL text semantics', async () => {
  const { render } = await import('svelte/server');
  const component = serverComponent(readFileSync(path.join(__dirname, 'SIVICombinedPanel.svelte'), 'utf8'),
    'SIVICombinedPanel.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' } },
      './projectMetadataEditor': metadata, './siviCoverEditor': cover, './siviCombinedEditor': editor,
    });
  for (const extended of [false, true]) {
    const { owner } = fixture({}, extended);
    await owner.load();
    const html = render(component, { props: { view: owner.view(), disabled: false, canSave: false,
      onstage() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {} } }).body;
    const ids = Array.from(html.matchAll(/<(?:input|textarea)\b[^>]*\bid="([^"]+)"[^>]*data-sivi-combined-column="([^"]+)"/g));
    assert.equal(ids.length, extended ? 17 : 14);
    assert.equal(new Set(ids.map(match => match[1])).size, ids.length);
    for (const match of ids) assert.ok(html.includes(`<label for="${match[1]}"`), match[2]);
    for (const column of ['HeightA', 'HeightB', 'Height6']) assert.equal(ids.filter(match => match[2] === column).length, 1);
    assert.match(html, /NULL \(distinct from empty text\)/);
    assert.equal((html.match(/data-sivi-combined-group=/g) || []).length, 3);
    assert.match(html, /grid-cols-1[^"]*sm:grid-cols-2[^"]*lg:grid-cols-5/);
    assert.match(html, /Species, Collected, IDs, creation\/deletion/);
  }
});

test('disposed late combined mutation receipts retire authority and history without refreshing or notifying', async () => {
  for (const operation of ['save', 'restore']) {
    const pending = deferred();
    const { owner, port, calls } = fixture();
    await owner.load();
    owner.stage(id, 'Cover1', '7', false);
    if (operation === 'save') port.save = () => pending.promise;
    else {
      assert.equal(await owner.save(), true);
      port.restore = () => pending.promise;
    }
    const before = clone(calls);
    const writing = operation === 'save' ? owner.save() : owner.restore('prune');
    owner.dispose();
    pending.resolve(operation === 'save' ? { ChangedCells: 1, HistoryID: '1' }
      : { cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 });
    assert.equal(await writing, false);
    assert.equal(owner.view().blocked, true);
    assert.equal(owner.view().historyId, null);
    assert.equal(calls.read, before.read);
    assert.equal(calls.refresh, before.refresh);
  }
});

test('source A warning reuses all seven normal-cover NULL combinations, never extended membership or height presence', () => {
  const columns = ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB'];
  for (const extended of [false, true]) for (let mask = 0; mask < 128; mask++) {
    const source = review({ extended });
    columns.forEach((column, index) => set(source, 0, column, mask & (1 << index) ? cell('real', 0) : cell('null')));
    set(source, 0, 'Cover5a', cell('real', -1));
    set(source, 0, 'HeightA', cell('real', 100));
    const edit = { rowId: id, form: source[0].Form, column: 'HeightB',
      expected: at(source, 0, 'HeightB'), value: cell('text', 'source notice') };
    const notices = sourceNotices.siviChildSourceNotices(source, [edit]);
    assert.equal(notices.length, mask === 0 ? 1 : 0);
    if (notices.length) {
      assert.equal(notices[0].rowId, id);
      assert.equal(notices[0].form, extended ? 'SubVegA-SIVI' : 'SubVegA-SIVI_BC');
      assert.equal(notices[0].event, 'AfterUpdate');
      assert.equal(notices[0].focus, 'Species');
      assert.match(notices[0].message, /No cover value is inferred; the vegetation record is not deleted/);
    }
    for (const column of columns) assert.equal(at(source, 0, column).storage, mask & (1 << columns.indexOf(column)) ? 'real' : 'null');
  }
});

test('source notices compose only effective owned changes and never warn on no-op saves', () => {
  const source = review();
  const columns = ['Cover1', 'Cover2', 'Cover3', 'TotalA', 'Cover4', 'Cover5', 'TotalB'];
  columns.forEach(column => set(source, 0, column, cell('real', -1)));
  const edits = columns.map(column => ({ rowId: id, form: source[0].Form, column,
    expected: at(source, 0, column), value: cell('null') }));
  assert.equal(sourceNotices.siviChildSourceNotices(source, edits.slice(1)).length, 0);
  assert.equal(sourceNotices.siviChildSourceNotices(source, edits).length, 1);
  assert.equal(sourceNotices.siviChildSourceNotices(source, []).length, 0);
  const unchanged = { ...edits[0], value: edits[0].expected };
  assert.equal(sourceNotices.siviChildSourceNotices(source, [unchanged]).length, 0);
  assert.equal(at(source, 0, 'Cover1').real, -1);
});

test('source C NULL warning precedes update while D has no invented warning or record deletion', () => {
  const source = review();
  const edit = { rowId: id, form: source[1].Form, column: 'Cover6',
    expected: at(source, 1, 'Cover6'), value: cell('null') };
  const notices = sourceNotices.siviChildSourceNotices(source, [edit]);
  assert.equal(notices.length, 1);
  assert.equal(notices[0].event, 'BeforeUpdate');
  assert.equal(notices[0].focus, 'Species');
  assert.match(notices[0].message, /removes this row from the C source view; it does not delete/);
  assert.equal(sourceNotices.siviChildSourceNotices(source, [{
    rowId: id, form: source[1].Form, column: 'Height6', expected: at(source, 1, 'Height6'), value: cell('real', 100),
  }]).length, 0);
  assert.equal(sourceNotices.siviChildSourceNotices(source, [{
    rowId: id, form: source[2].Form, column: 'Cover7', expected: at(source, 2, 'Cover7'), value: cell('null'),
  }]).length, 0);
});

test('source notice identity rejects foreign, repeated, incomplete or stale originals explicitly', () => {
  const source = review();
  const edit = { rowId: id, form: source[0].Form, column: 'Cover1',
    expected: at(source, 0, 'Cover1'), value: cell('null') };
  for (const invalid of [
    [{ ...edit, rowId: '1' }], [{ ...edit, form: 'SubVegAXL' }],
    [edit, edit], [{ ...edit, expected: cell('real', 42) }], [{ ...edit, column: 'unknown' }],
  ]) assert.throws(() => sourceNotices.siviChildSourceNotices(source, invalid), /notice source/);
  const incomplete = clone(source);
  incomplete[0].Columns.splice(incomplete[0].Columns.indexOf('TotalB'), 1);
  incomplete[0].Rows[0].cells.splice(source[0].Columns.indexOf('TotalB'), 1);
  assert.throws(() => sourceNotices.siviChildSourceNotices(incomplete, [{
    ...edit, column: 'HeightB', expected: at(source, 0, 'HeightB'), value: cell('text', 'changed'),
  }]), /projection is incomplete/);
});

test('all three writer surfaces independently gate planned source warnings without changing Save or close guards', async () => {
  for (const Writer of [session.SIVICombinedSession, coverSession.SIVICoverSession, heightSession.SIVIHeightSession]) {
    for (const enabled of [false, true]) {
      const source = review({ extended: true, extendedCoverOnly: true });
      const owner = new Writer('P', { read: async () => source }, () => {}, true, enabled);
      assert.equal(await owner.load(), true);
      const column = Writer === coverSession.SIVICoverSession ? 'Cover5a' : 'HeightB';
      owner.stage(id, column, column === 'Cover5a' ? '-1' : 'literal warning', false);
      assert.equal(owner.view().sourceNotices.length, enabled ? 1 : 0);
      assert.equal(owner.view().sourceNoticesSaved, false);
      assert.equal(owner.closeState().canSave, true);
      assert.equal(owner.closeState().blocked, false);
      owner.stage(id, column, column === 'Cover5a' ? 'bad' : 'x'.repeat(256), false);
      assert.equal(owner.view().sourceNotices.length, 0);
      assert.equal(owner.closeState().canSave, false);
      assert.equal(owner.closeState().unsaved, true);
      owner.stage(id, column, column === 'Cover5a' ? '0' : '', false);
      assert.equal(owner.view().sourceNotices.length, 0);
      assert.equal(owner.closeState().unsaved, false);
    }
  }
});

test('acknowledged source warnings retain exact Species and identity after query removal, remount and refresh failure', async () => {
  for (const failure of [false, true]) {
    const { owner, port, calls } = fixture({}, false, true);
    assert.equal(await owner.load(), true);
    owner.stage(id, 'Cover6', '', false);
    assert.equal(owner.view().sourceNotices.length, 1);
    assert.equal(owner.view().sourceNotices[0].species, 'RAW');
    assert.equal(owner.view().sourceNoticesSaved, false);
    port.read = async extended => {
      const source = review({ extended });
      source[1].Rows = [];
      return source;
    };
    if (failure) port.refreshParent = async () => { throw new Error('peer refresh failed'); };
    assert.equal(await owner.save(), !failure);
    assert.equal(calls.save, 1);
    assert.equal(owner.view().sourceNoticesSaved, true);
    assert.equal(owner.view().sourceNotices.length, 1);
    assert.equal(owner.view().sourceNotices[0].rowId, id);
    assert.equal(owner.view().sourceNotices[0].species, 'RAW');
    if (!failure) {
      assert.equal(owner.view().review[1].Rows.length, 0);
      assert.equal(owner.closeState().unsaved, false);
      owner.presentation(true);
      assert.equal(owner.view().sourceNotices[0].form, 'SubVegC-SIVI');
    } else {
      assert.equal(owner.closeState().blocked, true);
      assert.equal(owner.view().historyId, '1');
    }
    const snapshot = owner.view();
    snapshot.sourceNotices[0].species = 'foreign';
    assert.equal(owner.view().sourceNotices[0].species, 'RAW');
    port.refreshParent = async () => {};
    assert.equal(await owner.undo(), true);
    assert.equal(owner.view().sourceNotices.length, 0);
    assert.equal(owner.view().sourceNoticesSaved, false);
  }
});

test('unknown or rejected saves never present warning feedback as an acknowledged commit', async () => {
  for (const save of [async () => null, async () => { throw new Error('lost response'); }]) {
    const { owner } = fixture({ save }, false, true);
    await owner.load();
    owner.stage(id, 'Cover6', '', false);
    assert.equal(owner.view().sourceNotices.length, 1);
    assert.equal(await owner.save(), false);
    assert.equal(owner.view().sourceNoticesSaved, false);
    assert.equal(owner.view().sourceNotices.length, 0);
    assert.equal(owner.closeState().blocked, true);
  }
});

test('source warnings clear after verified restoration or explicit reload and never outlive disposed ownership', async () => {
  const { owner } = fixture({}, false, true);
  await owner.load();
  owner.stage(id, 'Cover6', '', false);
  assert.equal(await owner.save(), true);
  assert.equal(owner.view().sourceNoticesSaved, true);
  assert.equal(await owner.restore('prune'), true);
  assert.equal(owner.view().sourceNotices.length, 0);
  owner.stage(id, 'Cover6', '', false);
  assert.equal(await owner.save(), true);
  assert.equal(await owner.load(), true);
  assert.equal(owner.view().sourceNotices.length, 0);
  owner.stage(id, 'Cover1', '', false);
  assert.equal(owner.view().sourceNotices.length, 1);
  owner.dispose();
  assert.equal(owner.view().sourceNotices.length, 0);
});

test('real warning panel exposes an explicit read-only Species focus target and never invents an editor or a modal', async () => {
  const { render } = await import('svelte/server');
  const component = serverComponent(readFileSync(path.join(__dirname, 'SIVIChildSourceNotices.svelte'), 'utf8'),
    'SIVIChildSourceNotices.svelte', {});
  const source = review();
  const notices = sourceNotices.siviChildSourceNotices(source, [{
    rowId: id, form: source[1].Form, column: 'Cover6', expected: at(source, 1, 'Cover6'), value: cell('null'),
  }]);
  for (const saved of [false, true]) {
    const html = render(component, { props: { notices, saved } }).body;
    assert.match(html, saved ? /acknowledged Save/ : /planned Save/);
    assert.match(html, /role="status"/);
    assert.match(html, /tabindex="-1"/);
    assert.match(html, new RegExp(`aria-controls="sivi-source-species-SubVegC-SIVI-${id}"`));
    assert.match(html, /Species: RAW/);
    assert.match(html, /Species remains read-only/);
    assert.doesNotMatch(html, /<(?:input|textarea|dialog)\b/);
  }
  assert.doesNotMatch(render(component, { props: { notices: [], saved: false } }).body, /data-sivi-source-notices/);
});

test('every opted-in writer retains acknowledged warning feedback and clears it on verified restoration', async () => {
  for (const Writer of [session.SIVICombinedSession, coverSession.SIVICoverSession, heightSession.SIVIHeightSession]) {
    let source = review({ extended: true, extendedCoverOnly: true });
    let writes = 0;
    const owner = new Writer('P', {
      read: async () => clone(source),
      save: async (extended, originalOrRequest, heightEdits) => {
        assert.equal(extended, true);
        const edits = typeof originalOrRequest === 'string' ? JSON.parse(originalOrRequest).edits : heightEdits;
        writes++;
        for (const edit of edits) for (const group of source) for (const row of group.Rows) {
          const index = group.Columns.indexOf(edit.column);
          if (row.rowId === edit.rowId && index >= 0) row.cells[index] = clone(edit.value);
        }
        return { ChangedCells: edits.length, HistoryID: '1' };
      },
      restore: async () => {
        source = review({ extended: true, extendedCoverOnly: true });
        return { cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 };
      },
      refreshParent: async () => {},
    }, () => {}, true, true);
    await owner.load();
    const column = Writer === coverSession.SIVICoverSession ? 'Cover5a' : 'HeightB';
    owner.stage(id, column, column === 'Cover5a' ? '-1' : 'retained source warning', false);
    assert.equal(await owner.save(), true);
    assert.equal(writes, 1);
    assert.equal(owner.view().sourceNoticesSaved, true);
    assert.equal(owner.view().sourceNotices.length, 1);
    assert.equal(owner.view().sourceNotices[0].form, 'SubVegA-SIVI');
    assert.equal(owner.view().sourceNotices[0].species, 'RAW');
    assert.equal(owner.closeState().unsaved, false);
    const context = componentFunctions('FS882Form.svelte', ['refreshSIVIChildEditors'], {
      siviSession: Writer === heightSession.SIVIHeightSession ? owner : null,
      siviCoverSession: Writer === coverSession.SIVICoverSession ? owner : null,
      siviCombinedSession: Writer === session.SIVICombinedSession ? owner : null,
      loadChildData: async () => {},
    });
    await context.actions.refreshSIVIChildEditors('P', Writer === session.SIVICombinedSession ? 'height' : 'combined');
    assert.equal(owner.view().sourceNoticesSaved, true);
    assert.equal(owner.view().sourceNotices.length, 1);
    assert.equal(owner.view().sourceNotices[0].species, 'RAW');
    assert.equal(await owner.restore('prune'), true);
    assert.equal(owner.view().sourceNotices.length, 0);
    assert.equal(owner.view().sourceNoticesSaved, false);
  }
});

test('each real child panel renders retained feedback once even when originals are unavailable', async () => {
  const { render } = await import('svelte/server');
  const source = review();
  const notices = sourceNotices.siviChildSourceNotices(source, [{
    rowId: id, form: source[1].Form, column: 'Cover6', expected: at(source, 1, 'Cover6'), value: cell('null'),
  }]);
  for (const name of ['SIVICoverPanel', 'SIVIHeightPanel', 'SIVICombinedPanel']) {
    const component = serverComponent(readFileSync(path.join(__dirname, name + '.svelte'), 'utf8'), name + '.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' } },
      './projectMetadataEditor': metadata, './siviCoverEditor': cover, './siviHeightEditor': height, './siviCombinedEditor': editor,
    });
    for (const review of [source, null]) {
      const view = { review, drafts: name === 'SIVICombinedPanel' ? empty() : {}, extended: false,
        busy: false, blocked: review === null, error: review === null ? 'refresh failed' : null,
        historyId: '1', sourceNotices: notices, sourceNoticesSaved: true };
      const html = render(component, { props: { view, disabled: false, canSave: false,
        onstage() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {} } }).body;
      assert.equal((html.match(/\bdata-sivi-source-notices\b/g) || []).length, 1, name);
      assert.equal((html.match(new RegExp(`id="sivi-source-species-SubVegC-SIVI-${id}"`, 'g')) || []).length, 1, name);
      assert.match(html, /acknowledged Save/);
      assert.match(html, /Species: RAW/);
      if (review === null) assert.match(html, /refresh failed/);
    }
  }
});

test('mixed physical-row assignments produce one A and one C event notice without D or metadata side effects', () => {
  const source = review();
  const edits = [[0, 'Cover1'], [1, 'Cover6'], [2, 'Cover9']].map(([index, column]) => ({
    rowId: id, form: source[index].Form, column, expected: at(source, index, column), value: cell('null'),
  }));
  const notices = sourceNotices.siviChildSourceNotices(source, edits);
  assert.equal(notices.length, 2);
  assert.deepEqual(clone(notices.map(({ form, event, rowId }) => ({ form, event, rowId }))), [
    { form: 'SubVegA-SIVI_BC', event: 'AfterUpdate', rowId: id },
    { form: 'SubVegC-SIVI', event: 'BeforeUpdate', rowId: id },
  ]);
  assert.equal(at(source, 0, 'Cover1').real, 0);
  assert.equal(at(source, 1, 'Cover6').real, 0);
  assert.equal(at(source, 2, 'Cover9').real, 0);
});
