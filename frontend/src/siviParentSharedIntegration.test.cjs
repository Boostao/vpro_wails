const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
test('shared SIVI scope is independently gated and uses its owned facade rather than whole-header saves', () => {
  assert.equal(compile(parent, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(parent, /siviParentReviewEnabled && import\.meta\.env\.VITE_SIVI_PARENT_SHARED_EDITING === 'true'/);
  for (const method of ['GetOriginal', 'Save', 'Restore']) assert.ok(parent.includes(`SIVIParentSharedService.${method}(`));
  assert.match(parent, /read: \(\) => siviParentSharedReads\.track/);
  assert.match(parent, /cancelRead: \(\) => siviParentSharedReads\.cancelAll/);
});
test('shared SIVI state joins Save Undo Lock close ownership and sibling refresh without duplicate source fields', () => {
  assert.match(parent, /if \(siviParentSharedUnsaved\) \{ await siviParentSharedOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviParentSharedUnsaved\) \{ void siviParentSharedOperation\('undo'\); return; \}/);
  assert.match(parent, /Undo shared SIVI drafts\/recovery before changing the plot lock/);
  assert.match(parent, /siviParentSharedSession\.closeState\(\)\.unsaved/);
  assert.match(parent, /\|\| \(siviParentSharedClose\?\.blocked \?\? false\)/);
  assert.match(parent, /liveColumns: siviParentSharedView\.original \? siviParentSharedColumns : \[\]/);
  assert.match(parent, /unsaved: siviParentSharedUnsaved \|\| \(siviParentSharedClose\?\.blocked \?\? false\)/);
  assert.match(parent, /refreshSIVIParent\(plot, \[siviParentWriteSession, siviParentActionSession, siviProjectAssignmentSession\]\)/);
  assert.match(parent, /siviParentSharedSession\?\.dispose\(\)/);
});
test('reference editing is independently gated and its owned reads share cancellation and persistent session lifetime', () => {
  assert.match(parent, /siviParentSharedEnabled && import\.meta\.env\.VITE_SIVI_PARENT_REFERENCE_EDITING === 'true'/);
  assert.match(parent, /readReferences: zone => siviParentSharedReads\.track\(SIVIParentSharedService\.GetReferences\(siviContextId, plot, zone\)\)/);
  assert.match(parent, /\(\) => siviParentSharedRevision\+\+, siviParentReferenceEnabled\)/);
  assert.match(parent, /cancelRead: \(\) => siviParentSharedReads\.cancelAll\(\)/);
  assert.match(parent, /siviParentSharedFieldsFor\(siviParentReferenceEnabled\)\.map\(field => field\.column\)/);
  assert.match(parent, /onreferences=\{\(\) => \{ void siviParentSharedSession\?\.refreshReferences\(\); \}\}/);
});
