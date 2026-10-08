const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { test } = require('node:test');
const path = require('node:path');
const { compile } = require('svelte/compiler');

const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
const navigation = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');

test('standalone lifeform preview is independently gated without enabling incomplete summary vegetation', () => {
  assert.equal(compile(navigation, { filename: 'Navigation.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(navigation, /name: 'Lifeform Summary'.*VITE_LIFEFORM_SUMMARY === 'true' \? 'lifeform-summary' : undefined/);
  assert.match(navigation, /name: 'Summary Vegetation', icon: ChartColumn \}/);
  assert.match(app, /view === 'lifeform-summary' && import\.meta\.env\.VITE_LIFEFORM_SUMMARY === 'true' && \$projectState/);
});

test('lifeform preview borrows the owned context lifetime and reports busy state to navigation and native close', () => {
  assert.match(app, /\{#key \$projectState\.contextId\}\s*<LifeformSummary contextId=\{\$projectState\.contextId \?\? ''\} project=\{\$projectState\.activeProject\}/);
  assert.match(app, /<LifeformSummary[\s\S]*?su=\{\$projectState\.activeSU\} suPath=\{\$projectState\.suPath \?\? ''\}[\s\S]*?onBusyChange=\{\(value\) => \{ editorBusy = value; \}\}/);
  assert.match(app, /closeDisposition\(state, busy \|\| editorBusy \|\| transitionWorking/);
  assert.match(app, /void requestTransition\(async \(\) =>/);
});
