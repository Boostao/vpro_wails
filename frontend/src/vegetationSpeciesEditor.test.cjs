const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const editor = loadTypeScript('vegetationSpeciesEditor.ts', {
  './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
});
const json = value => JSON.parse(JSON.stringify(value));
const options = [{ code: 'A', scientificName: null, englishName: '', lifeform: 3, codeType: 'U' },
  { code: 'A', scientificName: 'duplicate', englishName: null, lifeform: 3, codeType: 'U' },
  { code: null, scientificName: 'NULL code', englishName: null, lifeform: null, codeType: null }];
const lists = Object.fromEntries(editor.speciesForms.map(form => [form, options]));

test('Species drafts preserve original identity, raw errors and exact list membership across all grids', () => {
  for (const form of editor.speciesForms) {
    for (const raw of ['', 'a', ' A', 'A ', "raw'", '123456789', '\ud800', '😀😀😀😀X']) {
      const drafts = editor.stageSpecies({}, form, 0, raw, 'RAW', lists);
      assert.equal(drafts['0'].raw, raw);
      assert.equal(editor.speciesDirty(drafts), true);
      assert.equal(editor.speciesErrors(drafts).length, 1);
      assert.throws(() => editor.speciesUpdates(drafts));
      const corrected = editor.stageSpecies(json(drafts), form, 0, 'A', 'refreshed', lists);
      assert.equal(editor.speciesErrors(corrected).length, 0);
      assert.deepEqual(json(editor.speciesUpdates(corrected)), [{ id: 0, form, expected: 'RAW', value: 'A' }]);
      const undone = editor.stageSpecies(corrected, form, 0, 'RAW', 'refreshed', {});
      assert.equal(editor.speciesDirty(undone), false);
      assert.deepEqual(json(editor.speciesUpdates(undone)), []);
    }
    const unchanged = editor.stageSpecies({}, form, -9, 'historical invalid', 'historical invalid', {});
    assert.equal(editor.speciesDirty(unchanged), false);
    assert.deepEqual(json(editor.speciesUpdates(unchanged)), []);
    const unavailable = editor.stageSpecies({}, form, 0, 'A', 'RAW', {});
    assert.match(unavailable['0'].error, /references are unavailable/);
  }
  for (const id of [0.5, 2147483648, -2147483649, NaN]) assert.throws(() => editor.stageSpecies({}, 'SubVegCXL', id, 'A', 'RAW', lists));
  assert.throws(() => editor.stageSpecies({}, 'SubVegAXL', 0, 'A', 'RAW', lists));
  let drafts = editor.stageSpecies({}, 'SubVegAXL_BC', 0, 'A', 'RAW', lists);
  drafts = editor.stageSpecies(drafts, 'SubVegAhtXL', 0, 'A', 'changed elsewhere', lists);
  assert.equal(drafts['0'].expected, 'RAW');
  assert.equal(drafts['0'].form, 'SubVegAhtXL');
});

test('Species source renderers keep one live labelled control and nullable duplicate metadata', () => {
  const { paper, presentation } = presentationHelpers();
  const SourceChild = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation,
    './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  });
  for (const name of editor.speciesForms) {
    const readonly = render(SourceChild, { props: {
      name, rows: [{ id: 0, values: { species: 'RAW' } }], disabled: false, onedit() {},
    } }).body;
    assert.match(readonly, /<input\b[^>]*value="RAW"[^>]* disabled[^>]*data-column="Species"/);
    for (const disabled of [false, true]) {
      const html = render(SourceChild, { props: {
        name, rows: [{ id: 0, values: { species: 'RAW' } }, { id: -9, values: { species: 'RAW' } }],
        disabled: true, speciesDisabled: disabled, onspeciesstage() {}, speciesLists: lists,
        speciesDrafts: { '0': { form: name, raw: 'bad', expected: 'RAW', error: 'Invalid species' } },
      } }).body;
      assert.equal((html.match(/<input\b[^>]*data-column="Species"/g) || []).length, 2);
      assert.match(html, /value="bad"[^>]*aria-invalid="true"/);
      assert.match(html, /aria-label="Species, row 0"/);
      assert.equal((html.match(/<datalist\b/g) || []).length, 1);
      assert.equal((html.match(/<option value="A"/g) || []).length, 2);
      assert.doesNotMatch(html, /<option value="null"/);
      assert.match(html, /NULL \|  \| Lifeform 3/);
      assert.equal(/<input\b[^>]*value="bad"[^>]* disabled/.test(html), disabled);
    }
  }
});

test('Opt-in species drafts gate every session and survive tab remount, Save, Undo, Lock and native close', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /VITE_VEGETATION_SPECIES_EDITING === 'true'/);
  assert.match(form, /let speciesDrafts = \$state<SpeciesDrafts>/);
  assert.match(form, /childUnsaved = \$derived\([^;]*speciesUnsaved\)/);
  for (const name of ['height', 'other', 'soil', 'attribute', 'collected']) {
    assert.match(form, new RegExp(`const ${name}EditingDisabled = \\$derived\\([^;]*speciesUnsaved\\)`));
  }
  assert.match(form, /if \(speciesUnsaved\) \{ await saveSpeciesDrafts\(\); return; \}/);
  assert.match(form, /if \(speciesUnsaved\) \{ void cancelSpeciesDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel species drafts before changing the plot lock/);
  assert.match(form, /speciesInvalid\.length > 0 \? 'Correct invalid species drafts/);
  assert.match(form, /speciesUnsaved && \(!speciesReferenceReady \|\| speciesReferenceBusy\)/);
  assert.match(form, /await PlotService\.UpdateVegetationSpecies[\s\S]*?committed = true;[\s\S]*?speciesDrafts = \{\}/);
  assert.match(form, /Species save failed; drafts retained/);
  assert.match(form, /Species changes committed, but refresh failed/);
  assert.match(form, /onspeciesstage=\{speciesEditingEnabled \? stageSpeciesCell : undefined\}/);
  assert.match(form, /Species requires the source-list draft workflow; unrestricted source-grid edits are unavailable/);
});
