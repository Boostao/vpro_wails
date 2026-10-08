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
const restoration = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': metadata });
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const species = loadTypeScript('vegetationSpeciesEditor.ts', { './qualityEditor': quality });
const editor = loadTypeScript('siviSpeciesEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './vegetationSpeciesEditor': species, './siviHeightEditor': height,
});
const heightSession = loadTypeScript('siviHeightSession.ts', {
  './siviHeightEditor': height, './projectMetadataRestore': restoration,
});
const child = loadTypeScript('siviChildWriteSession.ts', {
  './projectMetadataRestore': restoration, './siviHeightEditor': height, './siviHeightSession': heightSession,
});
const session = loadTypeScript('siviSpeciesSession.ts', {
  './siviChildWriteSession': child, './siviHeightSession': heightSession, './siviSpeciesEditor': editor,
});
const wireReferences = require('./siviSpeciesReferenceFixture.json');
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const id = '9007199254740993';
const clone = value => structuredClone(value);
const text = value => value === null ? cell('null') : cell('text', value);
function definitions(rows, aliases) {
  const names = ['Code', 'ScientificName', 'Lifeform', 'EnglishName', 'Codetype', ...(aliases ? ['OldCode'] : [])];
  return { columns: names.map(name => ({ name, declaredType: name === 'Lifeform' ? 'INTEGER' : 'TEXT' })),
    rows: rows.map((values, index) => ({ rowId: String(index + 1), cells: values.map((value, column) =>
      column === 2 && value !== null ? cell('integer', String(value)) : text(value)) })) };
}
function refs(master = [], personal = []) {
  return editor.validateSIVISpeciesReferences({ ...owner, master: definitions(master, true), personal: definitions(personal, false) }, owner);
}
function source(value = 'RAW', options = {}) {
  const projection = review(options);
  for (const group of projection) group.Rows[0].cells[2] = typeof value === 'string' || value === null ? text(value) : clone(value);
  return projection;
}

function sessionFixture(overrides = {}) {
  let current = source('RAW', { extendedCoverOnly: true });
  const calls = { save: 0, refresh: 0, read: 0 };
  const port = {
    read: async extended => {
      calls.read++;
      current = height.siviHeightPresentation(current, 'P', extended);
      return { original: clone(current), references: clone(wireReferences) };
    },
    save: async (extended, json) => {
      calls.save++;
      const request = JSON.parse(json);
      assert.deepEqual(request.references, wireReferences);
      for (const edit of request.edits) {
        for (const group of current) group.Rows.find(row => row.rowId === edit.rowId).cells[2] = text(edit.value);
      }
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
    restore: async () => { throw new Error('restoration was not expected'); },
    refreshParent: async () => { calls.refresh++; },
    ...overrides,
  };
  return { owner: new session.SIVISpeciesSession(owner, port, () => {}, false, true), calls, port };
}

test('production Go reference wire fixture preserves declaredType and is included unchanged in Save', async () => {
  assert.deepEqual(clone(editor.validateSIVISpeciesReferences(wireReferences, owner)), wireReferences);
  const invalid = clone(wireReferences);
  invalid.master.columns[0].type = invalid.master.columns[0].declaredType;
  delete invalid.master.columns[0].declaredType;
  assert.throws(() => editor.validateSIVISpeciesReferences(invalid, owner), /complete ordered typed/);
  const { owner: current, calls } = sessionFixture();
  assert.equal(await current.load(), true);
  current.stage(id, 'Species', 'TREE', false);
  assert.equal(current.closeState().canSave, true);
  assert.equal(await current.save(), true);
  assert.equal(calls.save, 1);
  assert.equal(calls.refresh, 1);
  assert.equal(current.view().review[0].Rows[0].cells[2].text, 'TREE');
  assert.equal(current.view().historyId, '1');
  assert.equal(current.closeState().unsaved, false);
});

test('persistent Species errors, source context and decisions survive view/remount and presentation without resetting entry', async () => {
  const { owner: current, calls } = sessionFixture();
  await current.load();
  current.stage(id, 'Species', 'MOSS', false);
  assert.equal(current.closeState().blocked, true);
  assert.equal(current.view().drafts[id].raw, 'MOSS');
  assert.equal(current.view().drafts[id].raw, 'MOSS');
  await assert.rejects(current.save(), /Correct invalid/);
  await assert.rejects(current.refreshSource(), /Finish or Undo/);
  current.context(id, 2);
  current.presentation(true);
  assert.equal(current.closeState().blocked, false);
  assert.equal(current.view().drafts[id].group, 2);
  assert.equal(await current.save(), true);
  current.stage(id, 'Species', 'old', false);
  current.choose(id, 'keep');
  assert.equal(current.view().drafts[id].raw, 'OLD');
  assert.equal(await current.save(), true);
  current.stage(id, 'Species', 'mine', false);
  current.choose(id, 'user', 'MINE');
  assert.equal(await current.save(), true);
  current.stage(id, 'Species', '\uD800', false);
  assert.equal(current.view().drafts[id].raw, '\uD800');
  assert.equal(current.closeState().blocked, true);
  assert.equal(await current.undo(), true);
  assert.deepEqual(clone(current.view().drafts), {});
  assert.equal(current.closeState().blocked, false);
  assert.equal(calls.save, 3);
});

test('Species source/reference load publishes no partial review and disposed completed reads cannot publish definitions', async () => {
  for (const bundle of [
    null, { original: source(), references: { ...wireReferences, plot: 'alien' } },
    { original: null, references: wireReferences },
  ]) {
    const { owner: current } = sessionFixture({ read: async () => bundle });
    assert.equal(await current.load(), false);
    assert.equal(current.view().review, null);
    assert.equal(current.view().references, null);
    assert.equal(current.closeState().canSave, false);
    assert.match(current.view().error, /reload failed/);
  }
  let deliver;
  const { owner: current } = sessionFixture({ read: () => new Promise(resolve => { deliver = resolve; }) });
  const pending = current.load();
  assert.equal(current.closeState().busy, true);
  current.dispose();
  deliver({ original: source(), references: wireReferences });
  assert.equal(await pending, false);
  assert.equal(current.view().references, null);
  assert.equal(current.view().review, null);
});

test('Species acknowledgement failures retain no-replay barriers and source notices use the effective Species value', async () => {
  const { owner: current, calls } = sessionFixture({ save: async () => null });
  await current.load();
  current.stage(id, 'Species', 'TREE', false);
  assert.ok(current.view().sourceNotices.length > 0);
  assert.ok(current.view().sourceNotices.every(notice => notice.species === 'TREE'));
  assert.equal(await current.save(), false);
  assert.equal(current.closeState().blocked, true);
  await assert.rejects(current.save(), /acknowledgement or refresh failed/);
  assert.equal(await current.undo(), true);
  assert.equal(current.closeState().blocked, false);
  assert.equal(calls.save, 0);
});

test('Species panel remount renders one associated field per physical row with explicit contexts and retained source decisions', async () => {
  const { render } = require('svelte/server');
  const Panel = serverComponent(readFileSync(path.join(__dirname, 'SIVISpeciesPanel.svelte'), 'utf8'),
    'SIVISpeciesPanel.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {
        AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
      },
      './projectMetadataEditor': metadata, './siviSpeciesEditor': editor, './vegetationSpeciesEditor': species,
    });
  const { owner: current } = sessionFixture();
  await current.load();
  const html = () => render(Panel, { props: {
    view: current.view(), disabled: false, canSave: current.closeState().canSave,
    onstage() {}, oncontext() {}, onchoose() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {},
  } }).body;
  assert.equal((html().match(/data-sivi-species-identity=/g) ?? []).length, 1);
  assert.ok(html().includes(`for="sivi-species-${id}"`));
  for (const label of ['Tree/Shrubs', 'Herb', 'Moss/Lichen']) assert.ok(html().includes(label));
  current.stage(id, 'Species', 'MOSS', false);
  assert.match(html(), /aria-invalid="true"/);
  assert.match(html(), /value="MOSS"/);
  assert.match(html(), /data-sivi-species-save[^>]*disabled/);
  current.context(id, 2);
  assert.doesNotMatch(html(), /aria-invalid="true"/);
  current.stage(id, 'Species', 'old', false);
  assert.match(html(), /data-sivi-species-keep=/);
  assert.match(html(), /data-sivi-species-replace=/);
  current.choose(id, 'replace', 'TREE');
  assert.match(html(), /Reviewed replace decision for old/);
  assert.match(html(), /Cover7 \(read-only in this editor\)/);
  assert.match(html(), /value="TREE"/);
  assert.equal((html().match(/data-sivi-species-identity=/g) ?? []).length, 1);
});

test('source lists batch all A/C/D lifeform/codetype combinations without converting physical identities or reference metadata', () => {
  const allowed = [[1, 2, 3, 4], [5, 6, 7, 8, 12], [1, 2, 9, 10, 11]];
  for (const type of ['u', 'U', 'x', 'X', 's', null, '']) for (let lifeform = 0; lifeform <= 13; lifeform++) {
    const references = refs([['CODE', null, lifeform, '', type, null]]);
    for (const group of [0, 1, 2]) {
      const listed = clone(editor.siviSpeciesListed(references, group));
      assert.equal(listed.length, /^[ux]$/i.test(type ?? '') && allowed[group].includes(lifeform) ? 1 : 0);
      if (listed.length) {
        assert.equal(listed[0].scientificName, null);
        assert.equal(listed[0].englishName, '');
      }
    }
  }
  const original = source();
  const references = refs([['LISTED', null, 1, '', 'u', null]]);
  const drafts = editor.stageSIVISpecies(original, references, {}, id, 'LISTED');
  const edits = clone(editor.siviSpeciesEdits(original, references, drafts));
  assert.equal(edits[0].rowId, id);
  assert.equal(original[0].Rows[0].cells[0].integer, '1');
  assert.deepEqual(Object.keys(edits[0]).sort(), ['expected', 'form', 'rowId', 'value']);
});

test('literal NULL/empty metadata and duplicate definitions survive while identical five-field source UNION tuples collapse', () => {
  const references = refs([
    ['CODE', null, 1, '', 'u', null], ['CODE', '', 1, null, 'u', 'OLD'],
    ['CODE', null, 1, '', 'u', 'ANOTHER'],
    [null, 'NULL code', 1, null, 'u', 'NO'],
    ['', null, 1, null, 'u', null],
  ], [['CODE', null, 1, '', 'u']]);
  assert.equal(references.master.rows.length, 5);
  assert.equal(references.personal.rows.length, 1);
  const options = clone(editor.siviSpeciesListed(references, 0));
  assert.equal(options.length, 2);
  assert.equal(options[0].scientificName, null);
  assert.equal(options[1].scientificName, '');
  assert.equal(options[0].englishName, '');
  assert.equal(options[1].englishName, null);
  const copy = editor.validateSIVISpeciesReferences(references, owner);
  copy.master.rows[0].cells[0].text = 'CHANGED';
  assert.equal(references.master.rows[0].cells[0].text, 'CODE');
});

test('references reject alien ownership and incomplete/duplicate/Unicode-repaired physical schemas instead of falling back', () => {
  const valid = refs([['CODE', null, 1, '', 'u', null]]);
  for (const property of ['contextId', 'project', 'plot']) {
    const alien = clone(valid); alien[property] = 'alien';
    assert.throws(() => editor.validateSIVISpeciesReferences(alien, owner), /different context/);
  }
  const mutations = [
    value => { value.master = null; }, value => { value.personal.rows = null; },
    value => { value.master.columns.pop(); }, value => { value.master.rows[0].cells.pop(); },
    value => { value.master.rows.push(clone(value.master.rows[0])); },
    value => { value.master.rows[0].rowId = '9223372036854775808'; },
    value => { value.master.rows[0].cells[0].text = '\uD800'; },
    value => { value.master.columns[0].declaredType = '\uD800'; },
    value => { value.master.rows[0].cells[0] = cell('integer', '1'); },
    value => { value.master.rows[0].cells[2] = cell('real', 1); },
    value => { value.master.rows[0].cells[2] = cell('integer', '32768'); },
    value => { value.master.rows[0].cells[2] = cell('integer', '-32769'); },
  ];
  for (const mutate of mutations) {
    const invalid = clone(valid); mutate(invalid);
    assert.throws(() => editor.validateSIVISpeciesReferences(invalid, owner), /complete ordered typed/);
  }
});

test('shared physical fields preserve an explicit eligible source group and current A presentation rather than dropping D-only choices', () => {
  const original = source();
  const references = refs([['TREE', null, 1, null, 'u', null], ['MOSS', null, 9, null, 'u', null]]);
  const physical = editor.siviSpeciesRows(original);
  assert.equal(physical.length, 1);
  assert.deepEqual(clone(physical[0].contexts.map(context => context.index)), [0, 1, 2]);
  let drafts = editor.stageSIVISpecies(original, references, {}, id, 'MOSS');
  assert.equal(editor.siviSpeciesErrors(drafts).length, 1);
  drafts = editor.contextSIVISpecies(original, references, drafts, id, 2);
  assert.equal(drafts[id].raw, 'MOSS');
  assert.equal(drafts[id].expected.text, 'RAW');
  assert.equal(editor.siviSpeciesErrors(drafts).length, 0);
  assert.equal(editor.siviSpeciesEdits(original, references, drafts)[0].form, 'SubVegD-SIVI');
  const invalidContext = editor.contextSIVISpecies(original, references, drafts, id, 0);
  assert.equal(invalidContext[id].raw, 'MOSS');
  assert.equal(editor.siviSpeciesErrors(invalidContext).length, 1);
  assert.throws(() => editor.siviSpeciesEdits(original, references, invalidContext), /exact source-list/);
  drafts = editor.stageSIVISpecies(original, references, {}, id, 'TREE', 0);
  const extended = height.siviHeightPresentation(original, 'P', true);
  assert.equal(editor.siviSpeciesEdits(extended, references, drafts)[0].form, 'SubVegA-SIVI');
  assert.throws(() => editor.contextSIVISpecies(original, references, drafts, id, 9), /source context/);
  const stale = source('CHANGED');
  assert.throws(() => editor.siviSpeciesEdits(stale, references, drafts), /original changed/);
});

test('old-code choices take precedence and explicit personal decisions bypass lists without granting creation or Unicode event conversion', () => {
  const original = source();
  const references = refs([['REPLACED', null, 9, null, 's', 'old'], [null, null, 1, null, 'u', 'mine']],
    [['OLD', null, 1, null, 'u'], ['MINE', '', 99, null, 's']]);
  for (const [kind, selected, expected] of [['replace', 'REPLACED', 'REPLACED'], ['keep', undefined, 'OLD']]) {
    const entered = editor.stageSIVISpecies(original, references, {}, id, 'old');
    const drafts = editor.chooseSIVISpecies(original, references, entered, id, kind, selected);
    assert.equal(drafts[id].raw, expected);
    const edit = editor.siviSpeciesEdits(original, references, drafts)[0];
    assert.equal(edit.decision, kind);
    assert.equal(edit.entered, 'old');
  }
  const old = editor.stageSIVISpecies(original, references, {}, id, 'old');
  assert.throws(() => editor.chooseSIVISpecies(original, references, old, id, 'user', 'OLD'), /not available/);
  const mine = editor.stageSIVISpecies(original, references, {}, id, 'mine');
  const selected = editor.chooseSIVISpecies(original, references, mine, id, 'user', 'MINE');
  assert.equal(editor.siviSpeciesEdits(original, references, selected)[0].value, 'MINE');
  assert.throws(() => editor.chooseSIVISpecies(original, references, selected, id, 'user', 'MINE'), /retained Species entry/);
  const unicode = refs([['\u00c6', null, 1, null, 'u', 'old']]);
  assert.throws(() => editor.chooseSIVISpecies(original, unicode, old, id, 'replace', '\u00c6'), /non-ASCII/);
});

test('new bounds are UTF-16 units and exact source literals; unchanged historical invalid text is omitted before validation', () => {
  const code = '\uD83D\uDE00'.repeat(4);
  const references = refs([[code, null, 1, null, 'u', null], [' A ', null, 1, null, 'u', null]]);
  const original = source();
  for (const raw of [code, ' A ']) {
    const draft = editor.stageSIVISpecies(original, references, {}, id, raw);
    assert.equal(editor.siviSpeciesErrors(draft).length, 0);
    assert.equal(editor.siviSpeciesEdits(original, references, draft)[0].value, raw);
  }
  for (const raw of ['', '\uD800', '\uD83D\uDE00'.repeat(5), 'A', ' a ']) {
    const draft = editor.stageSIVISpecies(original, references, {}, id, raw);
    assert.equal(draft[id].raw, raw);
    assert.equal(editor.siviSpeciesDirty(draft), true);
    assert.equal(editor.siviSpeciesErrors(draft).length, 1);
    assert.throws(() => editor.siviSpeciesEdits(original, references, draft), /Species|source-list/);
  }
  for (const raw of ['', 'unknown', 'historical overlength species']) {
    const historical = source(raw);
    const draft = editor.stageSIVISpecies(historical, null, {}, id, raw);
    assert.equal(editor.siviSpeciesDirty(draft), false);
    assert.deepEqual(clone(editor.siviSpeciesEdits(historical, references, draft)), []);
  }
  const nonText = editor.stageSIVISpecies(source(cell('integer', '1')), references, {}, id, 'CODE');
  assert.match(nonText[id].error, /non-text.*read-only/);
  assert.throws(() => editor.siviSpeciesRequestJSON([], references, owner, {}), /source plot/);
});
