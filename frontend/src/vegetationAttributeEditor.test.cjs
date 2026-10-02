const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const editor = loadTypeScript('vegetationAttributeEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') });
const json = value => JSON.parse(JSON.stringify(value));

test('All12 attribute bindings preserve source suggestion reuse, free AF and physical Long/Integer differences', () => {
  const { paper } = presentationHelpers();
  const controls = paper.paperChild('USysVegOtherXL').controls;
  assert.equal(editor.vegetationAttributeFields.length, 12);
  for (const field of editor.vegetationAttributeFields) {
    const source = controls.find(control => control.column === field.column);
    assert.ok(source, field.column);
    assert.equal(source.locked, false);
    assert.equal(source.enabled, true);
    assert.equal(source.type, field.group ? 'ComboBox' : 'TextBox');
    assert.equal(editor.vegetationAttributeField(field.column.toLowerCase()).key, field.key);
  }
  assert.equal(editor.vegetationAttributeField('AF').group, undefined);
  assert.equal(editor.vegetationAttributeField('Cultural2').group, 'Cultural1');
  assert.equal(editor.vegetationAttributeField('Species'), undefined);
  assert.equal(editor.vegetationAttributeField('Cover1'), undefined);
});

test('Raw integer errors, physical endpoints, original NULL and unchanged historical invalid attributes persist independently', () => {
  for (const field of editor.vegetationAttributeFields) {
    const minimum = field.long ? -2147483648 : -32768, maximum = field.long ? 2147483647 : 32767;
    for (const value of [minimum, maximum, -1, 0]) {
      const drafts = editor.stageVegetationAttribute({}, 0, field, String(value), null);
      assert.deepEqual(json(editor.vegetationAttributeUpdates(drafts)), [{ id: 0, values: { [field.key]: value }, expected: { [field.key]: null } }]);
    }
    for (const raw of [String(minimum - 1), String(maximum + 1), '1.5', '1bad', '1e999']) {
      const drafts = editor.stageVegetationAttribute({}, -9, field, raw, null);
      assert.equal(drafts['-9'][field.key].raw, raw);
      assert.equal(editor.vegetationAttributeDirty(drafts), true);
      assert.throws(() => editor.vegetationAttributeUpdates(drafts));
    }
    const historical = maximum + 1;
    const drafts = editor.stageVegetationAttribute({}, 0, field, String(historical), historical);
    assert.equal(editor.vegetationAttributeDirty(drafts), false);
    assert.deepEqual(json(editor.vegetationAttributeUpdates(drafts)), []);
  }
  const af = editor.vegetationAttributeField('AF'), ll = editor.vegetationAttributeField('LL');
  let drafts = editor.stageVegetationAttribute({}, 0, af, 'bad', null);
  drafts = editor.stageVegetationAttribute(drafts, -9, ll, '1.5', null);
  drafts = editor.stageVegetationAttribute(json(drafts), 0, af, '-1', 2);
  assert.equal(editor.vegetationAttributeErrors(drafts).length, 1);
  assert.throws(() => editor.vegetationAttributeUpdates(drafts));
  drafts = editor.stageVegetationAttribute(drafts, -9, ll, '2147483647', 3);
  assert.equal(drafts['0'].af.expected, null);
  assert.equal(drafts['-9'].ll.expected, null);
  assert.equal(editor.vegetationAttributeUpdates(drafts).length, 2);
  drafts = editor.stageVegetationAttribute(drafts, 0, af, '', 2);
  assert.equal(editor.vegetationAttributeUpdates(drafts).length, 1);
});

test('Actual renderer gives12 labelled raw controls, duplicate cultural suggestions and no species editor', () => {
  const { paper, presentation } = presentationHelpers();
  const SourceChild = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation,
    './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  });
  const values = { species: 'RAW', ...Object.fromEntries(editor.vegetationAttributeFields.map(field => [field.column.toLowerCase(), null])) };
  const html = render(SourceChild, { props: {
    name: 'USysVegOtherXL', rows: [{ id: 0, values }], disabled: false, onattributestage() {},
    attributeSuggestions: [{ listName: 'Cultural1', item: '1', itemDescription: null },
      { listName: 'Cultural1', item: '1', itemDescription: '' }],
  } }).body;
  assert.equal((html.match(/<input\b[^>]*data-column=/g) || []).length, 12);
  assert.equal((html.match(/value="1"/g) || []).length, 4);
  assert.doesNotMatch(html, /<input\b[^>]*data-column="Species"/);
  assert.match(html, /<span\b[^>]*data-column="Species"/);
  for (const field of editor.vegetationAttributeFields) {
    assert.match(html, new RegExp(`aria-label="[^"]+, row 0"[^>]*data-column="${field.column}"`));
  }
});

test('Persistent attribute sessions gate shared lifecycle and clear only committed or explicitly cancelled drafts', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /let attributeDrafts = \$state<VegetationAttributeDrafts>/);
  assert.match(form, /childUnsaved = \$derived\(heightUnsaved \|\| otherUnsaved \|\| soilUnsaved \|\| attributeUnsaved \|\| collectedUnsaved \|\| speciesUnsaved \|\| deletionReview !== null \|\| creationDraft !== null \|\| codeCheckOpen\)/);
  assert.match(form, /attributeInvalid\.length > 0/);
  assert.match(form, /if \(attributeUnsaved\) \{ await saveAttributeDrafts\(\); return; \}/);
  assert.match(form, /if \(attributeUnsaved\) \{ void cancelAttributeDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel vegetation attribute drafts before changing the plot lock/);
  assert.match(form, /await PlotService\.UpdateVegetationAttributes[\s\S]*?committed = true;[\s\S]*?attributeDrafts = \{\}/);
  assert.match(form, /Vegetation attribute save failed; drafts retained/);
  assert.match(form, /Vegetation attributes committed, but refresh failed/);
  assert.match(form, /untrack\(\(\) => void loadAttributeSuggestions\(\)\)/);
  assert.match(form, /onattributestage=\{attributeEditingEnabled \? stageAttributeCell : undefined\}/);
});
