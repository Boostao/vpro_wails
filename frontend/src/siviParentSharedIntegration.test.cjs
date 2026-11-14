const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
test('shared SIVI scope is independently gated and uses its owned facade rather than whole-header saves', () => {
  assert.equal(compile(parent, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  for (const method of ['GetSIVIParentSharedOriginal', 'SaveSIVIParentShared', 'RestoreSIVIParentShared']) assert.ok(parent.includes(`PlotEntryService.${method}(`));
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
test('reference editing owned reads share cancellation and persistent session lifetime', () => {
  assert.match(parent, /readReferences: zone => siviParentSharedReads\.track\(PlotEntryService\.GetSIVIParentSharedReferences\(siviContextId, plot, zone\)\)/);
  assert.match(parent, /\(\) => siviParentSharedRevision\+\+, siviParentReferenceEnabled\)/);
  assert.match(parent, /cancelRead: \(\) => siviParentSharedReads\.cancelAll\(\)/);
  assert.match(parent, /siviParentSharedFieldsFor\(siviParentReferenceEnabled\)\.map\(field => field\.column\)/);
  assert.match(parent, /onreferences=\{\(\) => \{ void siviParentSharedSession\?\.refreshReferences\(\); \}\}/);
});
