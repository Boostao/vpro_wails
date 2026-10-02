const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const editor = loadTypeScript('otherEditor.ts', { './qualityEditor': quality });
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('heightEditor.ts', { './numericEditor': numeric });
const json = value => JSON.parse(JSON.stringify(value));

test('Eight Other bindings retain source labels, types and eventless editable controls', () => {
  const { paper } = presentationHelpers();
  const controls = paper.paperChild('SubOtherXL').controls;
  assert.equal(editor.otherFields.length, 8);
  for (const field of editor.otherFields) {
    const source = controls.find(control => control.column === field.column);
    assert.equal(source.type, field.kind === 'flag' ? 'CheckBox' : 'TextBox');
    assert.equal(source.locked, false);
    assert.equal(source.enabled, true);
    for (const event of ['BeforeUpdate', 'AfterUpdate', 'OnChange']) assert.equal(source.properties[event], undefined);
    assert.equal(editor.otherField(field.column.toUpperCase()).key, field.key);
  }
  assert.equal(editor.otherField('Flag'), undefined);
  assert.equal(editor.otherField('ID'), undefined);
});

test('Text staging retains whitespace, case, apostrophes and UTF-16 bounds without repair', () => {
  for (const field of editor.otherFields.filter(field => field.kind === 'text')) {
    let drafts = editor.stageOther({}, 0, field, ' Raw\' item ', null);
    assert.equal(drafts['0'][field.key].value, ' Raw\' item ');
    const exact = 'x'.repeat(field.maximum - 2) + '🌲';
    drafts = editor.stageOther({}, 0, field, exact, null);
    assert.equal(editor.otherErrors(drafts).length, 0);
    for (const raw of [exact + 'x', '\ud800', '\udfff']) {
      drafts = editor.stageOther({}, 0, field, raw, null);
      assert.equal(drafts['0'][field.key].raw, raw);
      assert.equal(editor.otherDirty(drafts), true);
      assert.throws(() => editor.otherUpdates(drafts), /UTF-16|Unicode/);
    }
  }
});

test('Repeated corrections preserve original NULL, partner drafts and identity across remount transforms', () => {
  const name = editor.otherField('DataName'), item = editor.otherField('DataItem');
  let drafts = editor.stageOther({}, -7, name, 'x'.repeat(51), null);
  drafts = editor.stageOther(drafts, -7, item, 'second', 'original');
  drafts = editor.stageOther(drafts, -7, name, 'fixed', 'not the original');
  assert.equal(editor.otherErrors(drafts).length, 0);
  assert.deepEqual(json(editor.otherUpdates(drafts)), [{
    id: -7, text: { dataName: { value: 'fixed', expected: null }, dataItem: { value: 'second', expected: 'original' } }, flags: {}
  }]);
  drafts = editor.stageOther(drafts, -7, name, '', 'not the original');
  assert.equal(drafts['-7'].dataName.expected, null);
  assert.deepEqual(Object.keys(editor.otherUpdates(drafts)[0].text), ['dataItem']);
});

test('Three flags distinguish true, false and NULL with immutable expected-value patches', () => {
  for (const field of editor.otherFields.filter(field => field.kind === 'flag')) {
    const original = editor.stageOther({}, 1, field, true, null);
    const cleared = editor.stageOther(original, 1, field, null, true);
    assert.equal(original['1'][field.key].value, true);
    assert.equal(editor.otherDirty(cleared), false);
    const changed = editor.stageOther({}, 1, field, false, true);
    assert.deepEqual(json(editor.otherUpdates(changed))[0].flags[field.key], { value: false, expected: true });
  }
});

test('Historical overlength no-ops remain untouched while fresh corrections and identities are guarded', () => {
  const field = editor.otherField('DataName'), historical = 'h'.repeat(51);
  assert.equal(editor.otherDirty(editor.stageOther({}, 1, field, historical, historical)), false);
  assert.equal(editor.otherErrors(editor.stageOther({}, 1, field, historical + 'x', historical)).length, 1);
  for (const id of [null, NaN, 1.5, 2147483648, -2147483649]) {
    assert.throws(() => editor.stageOther({}, id, field, 'x', null), /identity/);
  }
  assert.throws(() => editor.stageOther({}, 1, field, true, null), /text/);
  assert.throws(() => editor.stageOther({}, 1, editor.otherField('UserFlag1'), 'true', null), /flag/);
});

test('Actual source table owns eight controls and nullable flag clearing with raw rejected text', () => {
  const { paper, presentation } = presentationHelpers();
  const Child = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation, './heightEditor': height, './otherEditor': editor
  });
  const values = Object.fromEntries(editor.otherFields.map(field => [field.column.toLowerCase(), null]));
  let drafts = editor.stageOther({}, 0, editor.otherField('DataName'), 'x'.repeat(51), null);
  drafts = editor.stageOther(drafts, 0, editor.otherField('UserFlag1'), true, null);
  const html = render(Child, { props: { name: 'SubOtherXL', rows: [{ id: 0, values }], disabled: false,
    onotherstage: () => {}, otherDrafts: drafts, ondelete: () => {}, deleteDisabled: true } }).body;
  assert.equal((html.match(/data-column=/g) || []).length, 8);
  assert.equal((html.match(/type="checkbox"/g) || []).length, 3);
  assert.equal((html.match(/Clear to NULL/g) || []).length, 3);
  assert.match(html, /aria-invalid="true"/);
  assert.match(html, new RegExp('x'.repeat(51)));
  assert.match(html, /Delete SubOtherXL row 0/);
});

test('Other drafts gate hidden Save, Undo, Lock, close, context and unrelated mutations independently', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /VITE_OTHER_EDITING !== 'false'/);
  assert.match(form, /childUnsaved = \$derived\(heightUnsaved \|\| otherUnsaved \|\| soilUnsaved \|\| attributeUnsaved \|\| collectedUnsaved \|\| speciesUnsaved \|\| deletionReview !== null \|\| creationDraft !== null \|\| codeCheckOpen \|\| metadataOpen\)/);
  assert.match(form, /if \(otherUnsaved\) \{ await saveOtherDrafts\(\); return; \}/);
  assert.match(form, /if \(otherUnsaved\) \{ void cancelOtherDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel Other drafts before changing the plot lock/);
  assert.match(form, /otherInvalid.length > 0 \? 'Correct invalid Other drafts/);
  assert.match(form, /await PlotService.UpdateOtherRecords\(draft.plotNumber, updates\)/);
  assert.match(form, /Other changes committed, but refresh failed/);
  assert.match(form, /Child changes committed, but refresh failed/);
});
