const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { cell, metadata, restoration } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric,
});
const cover = loadTypeScript('siviCoverEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': numeric, './siviHeightEditor': height,
});
const decisions = loadTypeScript('vegetationSpeciesEditor.ts', { './qualityEditor': quality });
const species = loadTypeScript('siviSpeciesEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './vegetationSpeciesEditor': decisions, './siviHeightEditor': height,
});
const editor = loadTypeScript('siviCreationEditor.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './siviCoverEditor': cover,
  './siviSpeciesEditor': species, './vegetationSpeciesEditor': decisions,
  './siviRequestId': loadTypeScript('siviRequestId.ts'),
});
const defaults = require('../../resources/sivi-new-row-defaults.json');
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
function references() {
  const refs = structuredClone(require('./siviSpeciesReferenceFixture.json'));
  const herb = structuredClone(refs.master.rows[0]);
  herb.rowId = '2'; herb.cells[0] = cell('text', 'HERB'); herb.cells[2] = cell('integer', '5'); herb.cells[5] = cell();
  refs.master.rows.push(herb);
  return refs;
}
function begin(form = 'SubVegA-SIVI') {
  return editor.stageSIVICreationSpecies(editor.beginSIVICreation(form, owner),
    form === 'SubVegC-SIVI' ? 'HERB' : form === 'SubVegD-SIVI' ? 'MOSS' : 'TREE');
}
const errors = (draft, refs = references()) => editor.siviCreationErrors(draft, refs, owner);
const plan = (draft, refs = references()) => editor.siviCreationPlan(draft, refs, owner);

test('private creation fields expose exactly21 source-labelled controls across four forms without identity or write authority', () => {
  const source = readFileSync(path.join(__dirname, 'SIVICreationFields.svelte'), 'utf8');
  const component = serverComponent(source, 'SIVICreationFields.svelte', {
    './siviCoverEditor': cover, './siviSpeciesEditor': species,
    './vegetationSpeciesEditor': decisions, './siviCreationEditor': editor,
  });
  let controls = 0;
  for (const [form, group, extended] of [['SubVegA-SIVI', 0, true], ['SubVegA-SIVI_BC', 0, false],
    ['SubVegC-SIVI', 1, false], ['SubVegD-SIVI', 2, false]]) {
    const draft = begin(form);
    const output = render(component, { props: { draft, owner, references: references(), onchange() {} } }).body;
    const columns = cover.siviCoverGroups(group, extended).flat();
    for (const column of columns) {
      const id = `sivi-creation-${form}-${column}`;
      assert.equal([...output.matchAll(new RegExp(`id="${id}"`, 'g'))].length, 1);
      assert.match(output, new RegExp(`for="${id}"`));
      assert.match(output, new RegExp(`<input[^>]*id="${id}"[^>]*disabled`));
      controls++;
    }
    assert.equal([...output.matchAll(/<input[^>]*id="sivi-creation-[^"]+"/g)].length, columns.length + 1);
    assert.ok(output.indexOf('explicit non-NULL') < output.indexOf(`sivi-creation-${form}-species"`));
    assert.match(output, /No application or physical identity has been assigned/);
    assert.doesNotMatch(output, /(?:Save|Create).*<\/button>|maxlength|HeightA|HeightB/);
  }
  assert.equal(controls, 21);
  assert.doesNotMatch(source, /bindings|GetImage|CreateSourceVegetation|SIVICreationService/);
});

test('actual private field handlers reuse raw validation, explicit NULL and source species decisions', () => {
  let component;
  component = componentFunctions('SIVICreationFields.svelte', ['cover', 'species', 'choose'], {
    ...editor, draft: begin(), owner, references: references(), error: '',
    onchange: next => { component.draft = next; },
  });
  component.actions.cover('Cover1', '100', false);
  component.actions.cover('TotalA', '0', false);
  assert.equal(component.draft.cells.Cover1.raw, '100');
  assert.match(errors(component.draft)[0], /100/);
  component.actions.cover('Cover1', '0', false);
  assert.equal(errors(component.draft).length, 0);
  component.actions.cover('Cover1', 'retained raw', true);
  assert.equal(component.draft.cells.Cover1.value.storage, 'null');
  assert.equal(component.draft.cells.Cover1.raw, 'retained raw');
  component.actions.cover('HeightA', '1', false);
  assert.match(component.error, /hidden|another source/);
  assert.equal(Object.hasOwn(component.draft.cells, 'HeightA'), false);
  component.actions.species(' TREE ');
  assert.equal(component.draft.species, ' TREE ');
  assert.ok(errors(component.draft).length > 0);
  component.actions.species('TREE');
  assert.equal(errors(component.draft).length, 0);
  component.actions.species('old');
  component.actions.choose('replace', 'TREE');
  assert.equal(component.draft.species, 'TREE');
  assert.equal(component.draft.decision.entered, 'old');
  component.actions.species('mine');
  assert.equal(component.draft.decision, undefined);
  component.actions.choose('user', 'MINE');
  assert.equal(component.draft.species, 'MINE');
  assert.equal(errors(component.draft).length, 0);
});

test('measured44-cell defaults have42 distinct NULLs, false Flag and non-inferred engine identity', () => {
  assert.equal(defaults.initialNullColumns.length, 42);
  assert.equal(new Set(defaults.initialNullColumns).size, 42);
  assert.deepEqual(defaults.initialBoolean, { column: 'Flag', integer: '0' });
  assert.equal(defaults.identity.defaultValue, 'GenUniqueID()');
  assert.equal(defaults.identity.allocationAlgorithmVerified, false);
  assert.equal(defaults.noRecordUpdateIssued, true);
  assert.equal(defaults.addBufferCancelled, true);
  assert.equal(defaults.transactionRolledBack, true);
  for (const query of defaults.membershipQueries) {
    assert.match(query.sourceSHA256, /^[0-9a-f]{64}$/);
    for (const column of query.anyNonNull) assert.ok(defaults.initialNullColumns.includes(column));
  }
});

test('all21 source-visible creation fields reuse literal SINGLE parsing and zero establishes query membership', () => {
  let checked = 0;
  for (const [form, group, extended] of [['SubVegA-SIVI', 0, true], ['SubVegA-SIVI_BC', 0, false],
    ['SubVegC-SIVI', 1, false], ['SubVegD-SIVI', 2, false]]) {
    for (const column of cover.siviCoverGroups(group, extended).flat()) {
      let draft = begin(form);
      assert.match(errors(draft)[0], /explicit non-NULL/);
      draft = editor.stageSIVICreationCover(draft, column, '0', false);
      assert.equal(errors(draft).length, 0);
      const result = plan(draft);
      assert.deepEqual(Array.from(result.covers, value => [value.column, value.value.storage, value.value.real]), [[column, 'real', 0]]);
      assert.equal(result.form, form);
      assert.equal(Object.hasOwn(result, 'ID'), false);
      assert.equal(Object.hasOwn(result, 'rowId'), false);
      checked++;
    }
  }
  assert.equal(checked, 21);
});

test('creation preserves raw invalid covers across other edits, revalidates tampering and clears on correction', () => {
  for (const raw of ['100', '100.1', ' 1', '1 ', 'NaN', 'Infinity', '\ud800', 'x\0y']) {
    let draft = editor.stageSIVICreationCover(begin(), 'Cover1', raw, false);
    assert.equal(draft.cells.Cover1.raw, raw);
    assert.ok(errors(draft).length);
    draft = editor.stageSIVICreationCover(draft, 'Cover2', '20', false);
    draft.cells.Cover1.error = null;
    assert.throws(() => plan(draft));
    draft = editor.stageSIVICreationCover(draft, 'Cover1', '99.99', false);
    assert.equal(errors(draft).length, 0);
    assert.equal(plan(draft).covers.length, 2);
  }
  const draft = editor.stageSIVICreationCover(begin(), 'Cover1', '1', false);
  draft.cells.Cover1.value = cell('real', 2);
  assert.throws(() => plan(draft), /differs from its literal/);
  draft.cells.Cover1.expected = cell('real', 1);
  assert.throws(() => plan(draft), /not a historical original/);
});

test('NULL and absent controls stay omitted, no totals or defaults are manufactured', () => {
  let draft = editor.stageSIVICreationCover(begin(), 'Cover1', '2', false);
  draft = editor.stageSIVICreationCover(draft, 'Cover2', '', true);
  const result = plan(draft);
  assert.deepEqual(Array.from(result.covers, value => value.column), ['Cover1']);
  result.covers[0].value.real = 99;
  assert.equal(draft.cells.Cover1.value.real, 2);
  draft = editor.stageSIVICreationCover(draft, 'Cover1', '', true);
  assert.throws(() => plan(draft), /explicit non-NULL/);
  for (const column of ['Cover5a', 'Cover5b', 'Cover5c', 'Cover6', 'ID', 'PlotNumber', 'Flag', 'Collected', 'HeightA']) {
    assert.throws(() => editor.stageSIVICreationCover(begin('SubVegA-SIVI_BC'), column, '1', false), /hidden or belongs/);
    const tampered = begin('SubVegA-SIVI_BC');
    tampered.cells[column] = { raw: '1', nullValue: false, expected: cell(), value: cell('real', 1), error: null };
    assert.throws(() => plan(tampered), /hidden or foreign/);
  }
});

test('creation binds an explicit owner and rechecks source reference ownership without forwarding identity properties', () => {
  assert.throws(() => editor.beginSIVICreation('unknown', owner), /explicit A, C or D/);
  for (const plot of ['', '12345678', '\ud800', 'P\0']) {
    assert.throws(() => editor.beginSIVICreation('SubVegA-SIVI', { ...owner, plot }));
  }
  const draft = editor.stageSIVICreationCover(begin(), 'Cover1', '1', false);
  for (const key of ['contextId', 'project', 'plot']) {
    assert.throws(() => editor.siviCreationPlan(draft, references(), { ...owner, [key]: 'foreign' }));
    const refs = references(); refs[key] = 'foreign';
    assert.throws(() => plan(draft, refs), /different context/);
  }
  assert.equal(Object.hasOwn(editor.siviCreationPlan(draft, references(), { ...owner, ID: 99, rowId: '9' }), 'ID'), false);
  assert.equal(Object.hasOwn(editor.siviCreationPlan(draft, references(), { ...owner, ID: 99, rowId: '9' }), 'rowId'), false);
});

test('creation species uses source lists and explicit old-code/personal decisions, never silent casing or repair', () => {
  let draft = editor.stageSIVICreationCover(begin(), 'Cover1', '1', false);
  for (const raw of ['', 'tree', '\ud800', 'x\0y', '123456789']) {
    draft = editor.stageSIVICreationSpecies(draft, raw);
    assert.ok(errors(draft).length);
    assert.throws(() => plan(draft));
    assert.equal(draft.species, raw);
  }
  draft = editor.stageSIVICreationSpecies(draft, 'old');
  draft = editor.chooseSIVICreationSpecies(draft, references(), owner, 'replace', 'TREE');
  assert.equal(plan(draft).species, 'TREE');
  assert.equal(plan(draft).decision.entered, 'old');
  draft = editor.stageSIVICreationSpecies(draft, 'mine');
  assert.equal(draft.decision, undefined);
  draft = editor.chooseSIVICreationSpecies(draft, references(), owner, 'user', 'MINE');
  assert.equal(plan(draft).species, 'MINE');
  draft.species = 'TREE';
  assert.throws(() => plan(draft), /differs from its explicit/);
});
