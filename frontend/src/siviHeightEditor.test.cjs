const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': [],
  '../../resources/project-metadata-template.json': [],
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const editor = loadTypeScript('siviHeightEditor.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': metadata,
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const { cell, review: sourceReview } = require('./siviSourceFixture.cjs');
function review() {
  return sourceReview({ extendedCoverOnly: true, applicationId: '0' });
}
test('SIVI validates exact source closures, raw physical identities, shared cells and cover-only membership', () => {
  const original = review();
  const copy = editor.validateSIVIProjection(original, 'P', false);
  copy[0].Rows[0].cells[2].text = 'mutated';
  assert.equal(original[0].Rows[0].cells[2].text, 'RAW');
  for (const mutate of [
    r => r.pop(), r => r[0].Columns.splice(10, 1), r => r[0].Form = 'SubVegAXL',
    r => r[0].Rows.push(structuredClone(r[0].Rows[0])), r => r[0].Rows[0].rowId = '01',
    r => r[1].Rows[0].cells[2].text = 'different',
    r => r[0].Rows[0].cells[1].text = 'p',
    r => r[0].Rows[0].cells[10] = cell('null'),
    r => r[0].Rows[0].cells[14] = { ...cell('text', ''), integer: '1' },
  ]) {
    const bad = review(); mutate(bad);
    assert.throws(() => editor.validateSIVIProjection(bad, 'P', false));
  }
});
test('SIVI raw draft errors survive serialized remounts and preserve NULL versus literal empty text', () => {
  const r = review(), id = r[0].Rows[0].rowId;
  let drafts = editor.stageSIVIHeight(r, {}, id, 'HeightB', 'x'.repeat(256), false);
  assert.equal(editor.siviHeightDirty(drafts), true);
  assert.equal(editor.siviHeightErrors(drafts).length, 1);
  assert.throws(() => editor.siviHeightEdits(r, JSON.parse(JSON.stringify(drafts))));
  drafts = editor.stageSIVIHeight(r, drafts, id, 'HeightB', '', true);
  const payload = editor.siviHeightEdits(r, drafts);
  assert.equal(payload[0].expected.text, '');
  assert.equal(payload[0].value.storage, 'null');
  drafts = editor.stageSIVIHeight(r, drafts, id, 'HeightB', '', false);
  assert.equal(editor.siviHeightDirty(drafts), false);
  assert.equal(editor.siviHeightEdits(r, drafts).length, 0);
  for (const text of ['  literal  ', 'x'.repeat(255), '🌲'.repeat(127) + 'x']) {
    drafts = editor.stageSIVIHeight(r, {}, id, 'HeightB', text, false);
    assert.equal(editor.siviHeightEdits(r, drafts)[0].value.text, text);
  }
  for (const text of ['🌲'.repeat(128), '\ud800']) {
    assert.throws(() => editor.siviHeightEdits(r, editor.stageSIVIHeight(r, {}, id, 'HeightB', text, false)));
  }
});
test('SIVI numeric guards and historical invalid omissions remain typed and presentation changes never reset drafts', () => {
  const r = review(), id = r[0].Rows[0].rowId;
  let drafts = editor.stageSIVIHeight(r, {}, id, 'HeightA', '3', false);
  const extended = editor.siviHeightPresentation(r, 'P', true);
  assert.equal(editor.siviHeightEdits(extended, drafts)[0].form, 'SubVegA-SIVI');
  assert.equal(editor.siviHeightEdits(r, drafts)[0].form, 'SubVegA-SIVI_BC');
  for (const raw of ['1bad', 'NaN', '1e999', '3.5e38']) {
    assert.throws(() => editor.siviHeightEdits(r, editor.stageSIVIHeight(r, {}, id, 'HeightA', raw, false)));
  }
  for (const raw of ['-1', '0', '3.4028234663852886e38']) {
    assert.equal(editor.siviHeightErrors(editor.stageSIVIHeight(r, {}, id, 'HeightA', raw, false)).length, 0);
  }
  for (const original of [cell('text', 'x'.repeat(256)), cell('integer', '1'), cell('blob', 'ff')]) {
    r[0].Rows[0].cells[14] = original;
    drafts = editor.stageSIVIHeight(r, {}, id, 'HeightB', metadata.metadataCellText(original), false);
    assert.equal(editor.siviHeightDirty(drafts), false);
    assert.equal(editor.siviHeightEdits(r, drafts).length, 0);
  }
  assert.throws(() => editor.siviHeightEdits(r, editor.stageSIVIHeight(r, {}, id, 'HeightB', '', true)), /BLOB/);
});
test('SIVI transport is cloned, keeps the original typed identity and rejects changed original heights', () => {
  const r = review(), id = r[0].Rows[0].rowId;
  const drafts = editor.stageSIVIHeight(r, {}, id, 'HeightA', '3', false);
  const payload = editor.siviHeightEdits(r, drafts);
  payload[0].expected.real = 99; payload[0].value.real = 100;
  assert.equal(drafts[id].HeightA.expected.real, 2);
  assert.equal(drafts[id].HeightA.value.real, 3);
  r[0].Rows[0].cells[7] = cell('real', 4);
  assert.throws(() => editor.siviHeightEdits(r, drafts), /original height changed/);
});
