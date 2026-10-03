const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { compile } = require('svelte/compiler');
const { loadTypeScript, presentationHelpers, serverComponent } = require('./svelteTestHelpers.cjs');
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('heightEditor.ts', { './numericEditor': numeric });
const { paper, presentation } = presentationHelpers();
const json = value => JSON.parse(JSON.stringify(value));
const read = file => readFileSync(path.join(__dirname, file), 'utf8');

test('extended shrubs retain source bindings, labels, numeric order and independent totals', () => {
  const controls = paper.paperChild('SubVegAXL').controls;
  const columns = controls.filter(control => control.column).sort((a, b) => a.tabOrder - b.tabOrder);
  const numeric = columns.filter(control => height.heightField(control.column));
  assert.deepEqual(json(numeric.map(control => control.column)),
    ['Cover1', 'Cover2', 'cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB']);
  for (const [field, label] of [['Cover5a', 'B3'], ['Cover5b', 'B4'], ['Cover5c', 'B5']]) {
    const control = columns.find(item => item.column === field);
    assert.equal(control.caption, label);
    assert.equal(control.properties.ValidationRule.value, '<100 Or Is Null');
    assert.equal(control.locked, false);
    assert.equal(control.enabled, true);
  }
  assert.equal(paper.paperChild('SubVegAXL_BC').controls.some(control => control.column === 'Cover5a'), false);
});

test('extended fields share strict numeric drafts and preserve original identity through hide/remount/correction', () => {
  for (const field of ['cover5a', 'cover5b', 'cover5c']) {
    let drafts = height.stageHeight({}, -9, field, '100', null, 'SubVegAXL');
    drafts = json(drafts);
    assert.equal(height.heightDirty(drafts), true);
    assert.equal(drafts['-9'][field].raw, '100');
    assert.throws(() => height.vegetationNumberUpdates(drafts), /less than 100/);
    drafts = height.stageHeight(drafts, -9, field, '-1.234567890123', 999, 'SubVegAXL_BC');
    assert.equal(height.heightErrors(drafts).length, 0);
    assert.deepEqual(json(height.vegetationNumberUpdates(drafts)), [{
      id: -9, values: { [field]: -1.234567890123 }, expected: { [field]: null }, forms: { [field]: 'SubVegAXL' }
    }]);
    assert.ok(height.heightValue(field, '3.5e38').error);
    assert.ok(height.heightValue(field, '12x').error);
    assert.equal(height.heightValue(field, '99.999').value, 99.999);
    assert.equal(height.heightValue(field, '').value, null);
    assert.equal(height.heightDirty(height.stageHeight({}, -9, field, '', null, 'SubVegAXL')), false);
  }
  assert.match(height.aCoverSourceNotice('SubVegAXL', -9, { cover5a: 0 }), /all A\/B cover values are NULL/);
});

test('extended renderer has one labelled live control per field and normal renderer does not lose hidden drafts', () => {
  const child = serverComponent(read('SourceChild.svelte'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation, './heightEditor': height
  });
  const drafts = height.stageHeight({}, -9, 'cover5a', '100', 0, 'SubVegAXL');
  const props = { rows: [{ id: -9, values: { cover5a: 0, cover5b: null, cover5c: null } }],
    disabled: false, drafts, onstage() {} };
  const extended = render(child, { props: { ...props, name: 'SubVegAXL' } }).body;
  for (const [field, label] of [['Cover5a', 'B3'], ['Cover5b', 'B4'], ['Cover5c', 'B5']]) {
    assert.equal((extended.match(new RegExp(`data-column="${field}"`, 'g')) || []).length, 1);
    assert.match(extended, new RegExp(`aria-label="${label}, row -9"`));
  }
  assert.match(extended, /value="100"[^>]*aria-invalid="true"/);
  const normal = render(child, { props: { ...props, name: 'SubVegAXL_BC' } }).body;
  assert.doesNotMatch(normal, /data-column="Cover5a"/);
  assert.equal(height.heightErrors(drafts).length, 1);
  assert.equal(drafts['-9'].cover5a.form, 'SubVegAXL');
});

test('shared option gates extended presentation and restricts heights without clearing drafts or replacing parent identity', () => {
  const form = read('FS882Form.svelte');
  assert.match(form, /VITE_EXTENDED_SHRUBS === 'true'/);
  assert.match(form, /data-extended-shrubs/);
  assert.match(form, /if \(extendedShrubs\) vegetationMode = 'cover'/);
  assert.match(form, /heightToggleDisabled=\{extendedShrubs \|\| busy \|\| headerWorkflowBusy\}/);
  assert.match(form, /extendedShrubs && originalChild\.form === 'SubVegAXL_BC' \? 'SubVegAXL'/);
  assert.match(form, /name === 'SubVegAXL_BC' \|\| name === 'SubVegAXL'/);
  assert.match(form, /hidden values, drafts and errors are retained/);
  assert.doesNotMatch(form, /onchange=\{[^}]*heightDrafts\s*=/);
  for (const file of ['FS882Form.svelte', 'SourcePage.svelte', 'SourceChild.svelte']) {
    assert.equal(compile(read(file), { filename: file, generate: 'client' }).warnings.length, 0);
  }
});
