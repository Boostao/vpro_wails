const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const editor = loadTypeScript('collectedEditor.ts');
const json = value => JSON.parse(JSON.stringify(value));
const forms = ['SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL', 'SubVegAhtXL', 'SubVegChtXL'];

test('Collected click cycles preserve NULL, original expectations, lowercase events and unknown history', () => {
  for (const initial of [null, 'C', 'V', 'c', 'v', '\uFF23', '\uFF43', '\uFF36', '\uFF56', '', 'X', 'historical']) {
    let drafts = {}, value = initial;
    for (let clicks = 1; clicks <= 9; clicks++) {
      value = value === null ? 'C' : ['C', 'c', '\uFF23', '\uFF43'].includes(value) ? 'V'
        : ['V', 'v', '\uFF36', '\uFF56'].includes(value) ? null : value;
      drafts = editor.stageCollected(json(drafts), 0, initial);
      assert.equal(drafts['0'].value, value);
      assert.equal(drafts['0'].expected, initial);
      assert.equal(editor.collectedDirty(drafts), value !== initial);
      assert.deepEqual(json(editor.collectedUpdates(drafts)), value === initial ? []
        : [{ id: 0, expected: initial, clicks: (clicks - 1) % 3 + 1 }]);
    }
  }
  let drafts = editor.stageCollected({}, 0, null);
  drafts = editor.stageCollected(drafts, -9, 'C');
  drafts = editor.stageCollected(drafts, 0, 'V');
  assert.equal(drafts['0'].expected, null);
  assert.equal(editor.collectedUpdates(drafts).length, 2);
  for (const id of [0.5, 2147483648, -2147483649, NaN]) assert.throws(() => editor.stageCollected({}, id, null));
});

test('All five source renderers expose one labelled cycle button, not a free text Collected field', () => {
  const { paper, presentation } = presentationHelpers();
  const SourceChild = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation,
    './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  });

  for (const name of forms) {
    const control = paper.paperChild(name).controls.find(control => control.column === 'Collected');
    assert.equal(control.locked, false);
    assert.equal(control.enabled, true);
    for (const disabled of [false, true]) {
      const html = render(SourceChild, { props: {
        name, rows: [{ id: 0, values: { species: 'RAW', collected: null } }], disabled: true,
        collectedDisabled: disabled, oncollectedstage() {}, collectedDrafts: { '0': { expected: null, value: 'V', clicks: 2 } },
      } }).body;
      assert.equal((html.match(/<button\b[^>]*data-column="Collected"/g) || []).length, 1);
      assert.doesNotMatch(html, /<input\b[^>]*data-column="Collected"/);
      assert.match(html, /aria-label="Cycle Collected, row 0"/);
      assert.match(html, />V<\/button>/);
      assert.equal(/data-column="Collected"[^>]* disabled/.test(html), disabled);
    }
  }
});

test('Collected retains the original warning source through remount and alternate-view clicks without changing transport', () => {
  let drafts=editor.stageCollected({},7,null,'SubVegAhtXL');
  drafts=editor.stageCollected(json(drafts),7,null,'SubVegAXL_BC');
  assert.equal(drafts['7'].form,'SubVegAhtXL');
  assert.deepEqual(json(editor.collectedUpdates(drafts)),[{id:7,expected:null,clicks:2}]);
  assert.equal('form' in editor.collectedUpdates(drafts)[0],false);
  const source=readFileSync(path.join(__dirname,'SourceChild.svelte'),'utf8');
  assert.match(source,/oncollectedstage\?\.\(row\.id, name\)/);
});

test('Persistent Collected drafts gate every other session, Save, Undo, Lock and native/context lifecycle', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /let collectedDrafts = \$state<CollectedDrafts>/);
  assert.match(form, /const stored = rows\[0\]\.collected \?\? null/);
  assert.match(form, /childUnsaved = \$derived\([^;]*collectedUnsaved[^;]*\)/);
  for (const name of ['height', 'other', 'soil', 'attribute']) {
    assert.match(form, new RegExp(`const ${name}EditingDisabled = \\$derived\\([^;]*collectedUnsaved[^;]*\\)`));
  }
  assert.match(form, /if \(collectedUnsaved\) \{ await saveCollectedDrafts\(\); return; \}/);
  assert.match(form, /if \(collectedUnsaved\) \{ void cancelCollectedDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel Collected drafts before changing the plot lock/);
  assert.match(form, /await PlotService\.UpdateCollectedRecords[\s\S]*?committed = true;[\s\S]*?collectedDrafts = \{\}/);
  assert.match(form, /Collected save failed; drafts retained/);
  assert.match(form, /Collected changes committed, but refresh failed/);
  assert.match(form, /oncollectedstage=\{collectedEditingEnabled \? stageCollectedCell : undefined\}/);
});
