const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { review } = require('./siviSourceFixture.cjs');
const source = readFileSync(path.join(__dirname, 'SIVIHeightPanel.svelte'), 'utf8');
const Panel = serverComponent(source, 'SIVIHeightPanel.svelte', {
  '../bindings/github.com/boostao/vpro-wails': {
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
  },
  './projectMetadataEditor': { metadataCellText: cell =>
    cell.storage === 'text' ? cell.text : cell.storage === 'real' ? String(cell.real) :
      cell.storage === 'integer' ? cell.integer : cell.storage === 'blob' ? `hex:${cell.blobHex}` : '' },
  './siviHeightEditor': { siviHeightDirty: drafts => Object.keys(drafts).length > 0 },
});
function html(view = {}) {
  return render(Panel, { props: {
    view: { review: review(), drafts: {}, extended: false, busy: false, blocked: false, error: null, historyId: null, ...view },
    disabled: false, canSave: true, onstage() {}, onsave() {}, onundo() {}, onreload() {}, onrestore() {},
  } }).body;
}
test('SIVI responsive source panel has one visibly labelled control per height and read-only A/C/D context', () => {
  const markup = html();
  for (const [column, label] of [['HeightA', 'Ht A'], ['HeightB', 'Ht B'], ['Height6', 'Ht']]) {
    assert.equal((markup.match(new RegExp(`data-sivi-column="${column}"`, 'g')) || []).length, 1);
    assert.match(markup, new RegExp(`<label[^>]*for="sivi-9007199254740993-${column}"[^>]*>${label}`));
  }
  assert.match(markup, /Tree\/Shrubs/);
  assert.match(markup, /Moss\/Lichen/);
  assert.match(markup, /Dr\/Dw/);
  assert.match(markup, /NULL \(distinct from empty text\)/);
  assert.doesNotMatch(markup, /<dt[^>]*>B3</);
  assert.doesNotMatch(markup, /data-sivi-extended-guidance/);
  assert.doesNotMatch(markup, /data-sivi-column="(?:Species|Cover\d|Collected)"/);
  assert.match(source, /grid-cols-1 gap-3 sm:grid-cols-2/);
  assert.match(source, /w-full min-w-0/);
});
test('SIVI extended presentation exposes source B3/B4/B5 without resetting or hiding height errors', () => {
  const markup = html({ review: review({ extended: true }), extended: true,
    drafts: { '9007199254740993': { HeightB: { raw: 'bad literal', nullValue: false, error: 'overlength retained' } } } });
  for (const label of ['B3', 'B4', 'B5']) assert.match(markup, new RegExp(`<dt[^>]*>${label}</`));
  assert.match(markup, /data-sivi-group="SubVegA-SIVI"/);
  assert.match(markup, />bad literal<\/textarea>/);
  assert.match(markup, /aria-invalid="true"/);
  assert.match(markup, /overlength retained/);
  assert.match(markup, /aria-describedby="sivi-9007199254740993-HeightB-error"/);
  assert.match(markup, /aggregate height editing remains available/);
  assert.ok(markup.indexOf('data-sivi-extended-guidance') > markup.lastIndexOf('data-sivi-column='));
});
test('SIVI safety feedback stays above fields and ordinary guidance below all fields', () => {
  const markup = html({ blocked: true, error: 'Do not replay the committed operation.' });
  assert.ok(markup.indexOf('Do not replay the committed operation.') < markup.indexOf('data-sivi-row='));
  assert.ok(markup.indexOf('Numeric heights may be cleared') > markup.lastIndexOf('data-sivi-column='));
  assert.match(markup, /data-sivi-save[^>]*disabled/);
  assert.match(markup, /data-sivi-column="HeightA"[^>]*disabled/);
});
test('SIVI parent keeps the owner outside tabs and wires busy/dirty/safety gates before native acceptance', () => {
  const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(parent, /VITE_SIVI_HEIGHT_EDITING === 'true'/);
  assert.match(parent, /const siviContextId = untrack\(\(\) => contextId\)/);
  assert.match(parent, /otherHeaderWorkflowBusy = \$derived\([^\n]*\|\| siviBusy \|\| siviCombinedBusy \|\| siviCoverBusy \|\| siviCollectedBusy \|\| siviSpeciesBusy \|\| siviIdentityBusy \|\| siviCreationBusy\)/);
  assert.match(parent, /headerWorkflowBusy = \$derived\(otherHeaderWorkflowBusy \|\| pictureBusy \|\| pictureMetadataPending\)/);
  assert.match(parent, /siviCreationPeerUnsaved = \$derived\(heightUnsaved \|\| siviUnsaved/);
  for (const gate of ['height', 'other', 'soil', 'attribute', 'collected', 'species']) {
    assert.match(parent, new RegExp(`${gate}EditingDisabled = \\$derived\\([^\\n]*\\|\\| siviUnsaved`));
  }
  assert.match(parent, /if \(siviUnsaved\) \{ await siviOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviUnsaved\) \{ void siviOperation\('undo'\); return; \}/);
  assert.match(parent, /blocked: \(siviClose\?\.blocked \?\? false\) \|\| \(siviCoverClose\?\.blocked \?\? false\) \|\| \(siviCombinedClose\?\.blocked \?\? false\) \|\| \(siviCollectedClose\?\.blocked \?\? false\) \|\| \(siviSpeciesClose\?\.blocked \?\? false\) \|\| \(siviIdentityClose\?\.blocked \?\? false\) \|\| \(siviParentWriteClose\?\.blocked \?\? false\)/);
  assert.match(parent, /siviSession\?\.dispose\(\);[\s\S]{0,60}siviReads\.cancelAll\(\)/);
  assert.match(parent, /\{:else if siviPanelOpen && siviView\}[\s\S]*?<SIVIHeightPanel[\s\S]*?\{:else\}[\s\S]*?<SourcePage name="Vegetation"/);
  assert.match(parent, /\{#if extendedShrubs && !siviPanelOpen && !siviCoverPanelOpen && !siviCombinedPanelOpen && !siviCollectedPanelOpen && !siviSpeciesPanelOpen && !siviIdentityPanelOpen\}[\s\S]*?Extended shrubs use cover mode/);
});
