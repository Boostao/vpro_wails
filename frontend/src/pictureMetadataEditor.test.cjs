const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const client = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const sourcePolicy = JSON.parse(readFileSync(path.join(__dirname, '../../resources/picture-metadata-policy.json'), 'utf8'));
const picture = loadTypeScript('pictureRead.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata,
});
const editor = loadTypeScript('pictureMetadataEditor.ts', {
  '../../resources/picture-metadata-policy.json': sourcePolicy,
  './projectMetadataEditor': metadata, './pictureRead': picture,
});
const owner = { contextId: 'owned', project: 'Sample', plotNumber: '00337' };
function review() {
  return { ...owner, records: {
    columns: ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment'].map(name => ({ name, declaredType: 'TEXT' })),
    rows: [{ rowId: '9223372036854775806', cells: [
      cell('integer', '-2147483648'), cell('text', 'Default'), cell('text', 'Pic01.jpg'), cell('text', owner.plotNumber), cell(),
    ] }],
  } };
}
function begin(source = review()) {
  return editor.beginPictureMetadataDraft(source, source.records.rows[0].rowId, owner);
}

test('picture policy preserves explicit NULL/empty rules, primary index and differing logical/storage order', () => {
  assert.equal(sourcePolicy.sourceSHA256, 'b84312d7f19f5a84b54b98be80f6b46fd86223a24eb168d11a0a90b570debcea');
  assert.deepEqual(sourcePolicy.logicalFieldOrder, ['ID', 'PlotNumber', 'PicDir', 'PicName', 'PicComment']);
  assert.deepEqual(sourcePolicy.nativeStorageOrder, ['ID', 'PicDir', 'PicName', 'PlotNumber', 'PicComment']);
  assert.equal(sourcePolicy.fields.length, 5);
  for (const [name, size, allowEmpty] of [['PlotNumber', 7, false], ['PicDir', 255, true], ['PicName', 255, false], ['PicComment', 255, false]]) {
    const field = sourcePolicy.fields.find(field => field.name === name);
    assert.equal(field.daoType, 10); assert.equal(field.size, size);
    assert.equal(field.required, false); assert.equal(field.allowZeroLength, allowEmpty);
    assert.equal(field.defaultValue, ''); assert.equal(field.validationRule, ''); assert.equal(field.validationText, '');
  }
  const id = sourcePolicy.fields.find(field => field.name === 'ID');
  assert.equal(id.daoType, 4); assert.equal(id.attributes, 17); assert.equal(id.defaultValue, 'GenUniqueID()');
  assert.deepEqual(sourcePolicy.indexes.find(index => index.primary).fields, ['ID']);
  assert.equal(sourcePolicy.indexes.find(index => index.primary).required, true);
});

test('picture draft selects exact physical identity and owns a detached original even from a Svelte proxy', () => {
  const source = client.proxy(review()), draft = begin(source);
  assert.equal(draft.id, -2147483648);
  assert.equal(draft.original.rowId, '9223372036854775806');
  source.records.rows[0].cells[1].text = 'changed externally';
  assert.equal(draft.original.cells[1].text, 'Default');
  assert.equal(editor.pictureMetadataChanges(draft).length, 0);
  assert.equal(editor.pictureMetadataFieldValue(draft, 'PicDir').raw, 'Default');
  const maximum = review(); maximum.records.rows[0].cells[0] = cell('integer', '2147483647');
  assert.equal(begin(maximum).id, 2147483647);
});

test('picture drafts reject foreign owners, missing/duplicate physical rows and ambiguous or non-Long IDs', () => {
  for (const key of ['contextId', 'project', 'plotNumber']) {
    assert.throws(() => editor.beginPictureMetadataDraft(review(), review().records.rows[0].rowId, { ...owner, [key]: 'foreign' }));
    assert.throws(() => editor.beginPictureMetadataDraft(review(), review().records.rows[0].rowId, { ...owner, [key]: '' }));
  }
  assert.throws(() => editor.beginPictureMetadataDraft(review(), 'missing', owner), /Select one reviewed/);
  for (const identity of [cell(), cell('text', '1'), cell('integer', '2147483648'), cell('integer', '-2147483649')]) {
    const source = review(); source.records.rows[0].cells[0] = identity;
    assert.throws(() => begin(source));
  }
  for (const rowId of [review().records.rows[0].rowId, '-9']) {
    const source = review(); source.records.rows.push({ ...source.records.rows[0], rowId });
    assert.throws(() => begin(source));
  }
});

test('picture text is literal with independent NULL/empty choices and no partial request on error', () => {
  let draft = editor.stagePictureMetadata(begin(), 'PicDir', '', false);
  draft = editor.stagePictureMetadata(draft, 'PicName', '', false);
  assert.match(editor.pictureMetadataDraftErrors(draft)[0], /permits NULL but not new empty text/);
  assert.throws(() => editor.pictureMetadataChanges(draft), /permits NULL/);
  draft = editor.stagePictureMetadata(draft, 'PicName', '', true);
  let changes = editor.pictureMetadataChanges(draft);
  assert.deepEqual(Array.from(changes, change => [change.column, change.value.storage, change.value.text]),
    [['PicDir', 'text', ''], ['PicName', 'null', null]]);
  draft = editor.stagePictureMetadata(draft, 'PicDir', ' literal Directory  ', false);
  draft = editor.stagePictureMetadata(draft, 'PicName', ' Pic01.JPEG  ', false);
  changes = editor.pictureMetadataChanges(draft);
  assert.equal(changes[0].value.text, ' literal Directory  ');
  assert.equal(changes[1].value.text, ' Pic01.JPEG  ');
  assert.equal(editor.pictureMetadataDraftErrors(draft).length, 0);
});

test('picture source bounds count UTF-16 units and preserve errors until a valid correction', () => {
  for (const column of ['PicDir', 'PicName']) {
    const exact = '😀'.repeat(127) + 'x';
    let draft = editor.stagePictureMetadata(begin(), column, exact, false);
    assert.equal(editor.pictureMetadataChanges(draft)[0].value.text.length, 255);
    for (const raw of ['😀'.repeat(128), '\ud800', '\udc00', 'x\0y']) {
      draft = editor.stagePictureMetadata(draft, column, raw, false);
      assert.equal(editor.pictureMetadataFieldValue(draft, column).raw, raw);
      assert.equal(editor.pictureMetadataDraftErrors(draft).length, 1);
      assert.throws(() => editor.pictureMetadataChanges(draft));
      const other = column === 'PicDir' ? 'PicName' : 'PicDir';
      draft = editor.stagePictureMetadata(draft, other, 'valid', false);
      assert.equal(editor.pictureMetadataDraftErrors(draft).length, 1);
      draft = editor.stagePictureMetadata(draft, column, exact, false);
      assert.equal(editor.pictureMetadataDraftErrors(draft).length, 0);
    }
  }
});

test('picture historical invalid text/storage stays omitted when unchanged instead of being repaired', () => {
  for (const original of [cell('text', ''), cell('text', 'x'.repeat(256)), cell('integer', '8'), cell('blob', 'aabb')]) {
    const source = review(); source.records.rows[0].cells[2] = original;
    let draft = begin(source);
    const displayed = editor.pictureMetadataFieldValue(draft, 'PicName');
    draft = editor.stagePictureMetadata(draft, 'PicName', displayed.raw, displayed.nullValue);
    draft = editor.stagePictureMetadata(draft, 'PicDir', 'changed', false);
    assert.deepEqual(Array.from(editor.pictureMetadataChanges(draft), change => change.column), ['PicDir']);
    assert.equal(JSON.stringify(draft.original.cells[2]), JSON.stringify(original));
    draft = editor.stagePictureMetadata(draft, 'PicName', 'literal replacement', false);
    assert.equal(editor.pictureMetadataChanges(draft)[1].value.storage, 'text');
  }
});

test('picture draft never implicitly edits identity/comments and requests revalidate rather than trusting staged cells', () => {
  const draft = begin();
  for (const column of ['ID', 'PlotNumber', 'PicComment', 'unknown']) {
    assert.throws(() => editor.stagePictureMetadata(draft, column, 'value', false), /Only existing picture/);
  }
  const staged = editor.stagePictureMetadata(draft, 'PicName', 'x'.repeat(256), false);
  staged.cells.PicName.error = null; staged.cells.PicName.value = cell('text', 'forged valid');
  assert.throws(() => editor.pictureMetadataChanges(staged), /255 UTF-16/);
  assert.equal(editor.pictureMetadataDraftErrors(staged).length, 1);
  for (const [raw, nullValue] of [[1, false], ['valid', 1]]) {
    assert.throws(() => editor.stagePictureMetadata(draft, 'PicName', raw, nullValue), /exact text/);
  }
});
