const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const numeric = loadTypeScript('numericEditor.ts');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('soilChildEditor.ts', { './numericEditor': numeric, './qualityEditor': quality });
const height = loadTypeScript('heightEditor.ts', { './numericEditor': numeric });
const json = value => JSON.parse(JSON.stringify(value));

test('Soil descriptors cover all30 source controls and preserve intentional suggestion differences', () => {
  const { paper } = presentationHelpers();
  for (const [kind, count] of [['Humus', 12], ['Mineral', 18]]) {
    const controls = paper.paperChild(`Soil${kind}XL`).controls;
    assert.equal(editor.soilFields[kind].length, count);
    for (const field of editor.soilFields[kind]) {
      const source = controls.find(control => control.column === field.column);
      assert.ok(source, `${kind}.${field.column}`);
      assert.equal(source.locked, false);
      assert.equal(source.enabled, true);
      assert.equal(editor.soilField(kind, field.column.toUpperCase()).key, field.key);
      if (field.group) assert.equal(source.type, 'ComboBox');
      if (field.key.startsWith('roots')) {
        assert.equal(source.type, 'TextBox');
        assert.equal(field.group, undefined);
      }
    }
  }
  assert.equal(editor.soilField('Humus', 'FecalAbundance').group, 'MycelAbundance');
  assert.equal(editor.soilField('Humus', 'ID'), undefined);
  assert.equal(editor.soilField('Mineral', 'Flag'), undefined);
});

test('All physical text and numeric bounds retain malformed raw input and omit unchanged historical values', () => {
  for (const kind of ['Humus', 'Mineral']) {
    for (const field of editor.soilFields[kind]) {
      const stored = field.kind === 'text' || field.kind === 'memo' ? null : null;
      const valid = field.kind === 'text' ? 'x'.repeat(field.maximum) : field.kind === 'memo' ? " raw'\n🌲 " : '-1';
      let drafts = editor.stageSoil({}, kind, 0, field, valid, stored);
      assert.equal(editor.soilErrors(drafts).length, 0);
      for (const raw of field.kind === 'text' || field.kind === 'memo'
        ? [...(field.maximum ? ['x'.repeat(field.maximum + 1), 'x'.repeat(field.maximum - 1) + '🌲'] : []), '\ud800', '\udfff']
        : ['1bad', '1e999', ...(field.kind === 'integer' ? ['32768', '-32769', '1.5'] : ['4e38', '-4e38'])]) {
        drafts = editor.stageSoil({}, kind, 0, field, raw, stored);
        assert.equal(editor.soilCell(drafts, kind, 0, field.key).raw, raw);
        assert.equal(editor.soilDirty(drafts), true);
        assert.throws(() => editor.soilUpdates(drafts));
      }
      const historical = field.kind === 'text' ? 'h'.repeat(field.maximum + 1)
        : field.kind === 'memo' ? "historical\nmemo" : field.kind === 'integer' ? 32768 : 4e38;
      drafts = editor.stageSoil({}, kind, 0, field, String(historical), historical);
      assert.equal(editor.soilDirty(drafts), false);
      assert.deepEqual(json(editor.soilUpdates(drafts)), []);
    }
  }
});

test('Cross-table identical IDs, original NULL expectations, independent errors and raw corrections survive remount', () => {
  const h = editor.soilField('Humus', 'Horizon'), m = editor.soilField('Mineral', 'ASP');
  let drafts = editor.stageSoil({}, 'Humus', -9, h, 'toolongvalue', null);
  drafts = editor.stageSoil(drafts, 'Mineral', -9, m, '1.5', null);
  drafts = editor.stageSoil(json(drafts), 'Humus', -9, h, " Raw' ", 'not original');
  assert.equal(editor.soilErrors(drafts).length, 1);
  assert.throws(() => editor.soilUpdates(drafts));
  drafts = editor.stageSoil(drafts, 'Mineral', -9, m, '-32768', 1);
  assert.deepEqual(json(editor.soilUpdates(drafts)), [
    { kind: 'Humus', id: -9, text: { horizon: { expected: null, value: " Raw' " } }, numbers: {} },
    { kind: 'Mineral', id: -9, text: {}, numbers: { asp: { expected: null, value: -32768 } } },
  ]);
  drafts = editor.stageSoil(drafts, 'Mineral', -9, m, '', 1);
  assert.equal(editor.soilCell(drafts, 'Mineral', -9, m.key).expected, null);
  assert.equal(editor.soilUpdates(drafts).length, 1);
  for (const id of [1.5, 2147483648, -2147483649]) assert.throws(() => editor.stageSoil({}, 'Humus', id, h, '', null), /signed32/);
});

test('Actual source renderer keeps one labelled draft control per soil binding, multiline memo and duplicated suggestions', () => {
  const { paper, presentation } = presentationHelpers();
  const SourceChild = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation, './heightEditor': height, './soilChildEditor': editor
  });
  for (const kind of ['Humus', 'Mineral']) {
    const fields = editor.soilFields[kind];
    const values = Object.fromEntries(fields.map(field => [field.column.toLowerCase(), null]));
    const html = render(SourceChild, { props: {
      name: `Soil${kind}XL`, rows: [{ id: 0, values }], disabled: false, onsoilstage() {},
      soilSuggestions: [
        { listName: 'MycelAbundance', item: 'fixture', itemDescription: null },
        { listName: 'MycelAbundance', item: 'fixture', itemDescription: '' }
      ]
    } }).body;
    assert.equal((html.match(/data-column=/g) || []).length, fields.length);
    assert.equal((html.match(/<textarea/g) || []).length, 1);
    for (const field of fields) {
      assert.match(html, new RegExp(`aria-label="[^"]+, row 0"[^>]*data-column="${field.column}"`));
    }
    if (kind === 'Humus') assert.equal((html.match(/value="fixture"/g) || []).length, 4);
  }
});

test('Root soil sessions participate in every shared lifecycle and disable instant-save/hidden alternate actions', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /let soilDrafts = \$state<SoilDrafts>/);
  assert.match(form, /childUnsaved = \$derived\(heightUnsaved \|\| otherUnsaved \|\| soilUnsaved \|\| attributeUnsaved \|\| collectedUnsaved \|\| speciesUnsaved \|\| deletionReview !== null \|\| creationDraft !== null \|\| codeCheckOpen \|\| metadataOpen \|\| profileReviewBlocked\)/);
  assert.match(form, /soilInvalid\.length > 0/);
  assert.match(form, /if \(soilUnsaved\) \{ await saveSoilDrafts\(\); return; \}/);
  assert.match(form, /if \(soilUnsaved\) \{ void cancelSoilDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel soil drafts before changing the plot lock/);
  assert.match(form, /await PlotService\.UpdateSoilRecords[\s\S]*?committed = true;[\s\S]*?soilDrafts = \{\}/);
  assert.match(form, /Soil save failed; drafts retained/);
  assert.match(form, /Soil changes committed, but refresh failed/);
  assert.match(form, /untrack\(\(\) => void loadSoilSuggestions\(\)\)/);
  assert.match(form, /onsoilstage=\{soilEditingEnabled \? stageSoilCell : undefined\}/);
  assert.match(form, /deleteDisabled=\{soilUnsaved\}/);
});
