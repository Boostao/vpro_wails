const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, loadTypeScript } = require('./svelteTestHelpers.cjs');
const component = readFileSync(path.join(__dirname, 'SummaryVegetationReport.svelte'), 'utf8');
const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
const nav = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');

test('summary report native route, backend and build gates are independent default-off', () => {
  assert.equal(compile(component, { filename: 'SummaryVegetationReport.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(nav, /name: 'Summary Vegetation'.*VITE_SUMMARY_VEGETATION === 'true' \? 'summary-vegetation' : undefined/);
  assert.match(app, /view === 'summary-vegetation' && import\.meta\.env\.VITE_SUMMARY_VEGETATION === 'true' && \$projectState/);
  assert.match(component, /const enabled = import\.meta\.env\.VITE_SUMMARY_VEGETATION === 'true'/);
  assert.match(component, /\$state\.raw\(session\.view\(\)\)/);
  assert.doesNotMatch(component, /structuredClone|SummaryVegetationService\.Save|ExportReviewed|\.update/);
  assert.match(component, /SummaryVegetationPreferencesService\.SaveSummaryVegetationPreferences/);
});

test('owned tab remount and cancellable root-close/navigation/selection holds are wired', () => {
  assert.match(app, /\{#key \$projectState\.contextId\}\s*<SummaryVegetationReport/);
  assert.match(app, /<SummaryVegetationReport[\s\S]*?operationHeld=\{busy \|\| transitionWorking \|\| pendingTransition !== null \|\| !!closeRequest\}/);
  assert.match(component, /session\.subscribe\([\s\S]*onBusyChange/);
  assert.match(component, /onBusyChange\(viewState\.busy \|\| \(!preferences && !!viewState\.draftError\) \|\| preferenceReadBusy/);
  assert.match(component, /onDestroy\(\(\) => \{\s*session\.cancel\(port\); unsubscribe\(\); unsubscribePreferences\?\.\(\); unsubscribeQuality\?\.\(\); unsubscribePresentation\?\.\(\);\s*unsubscribeFieldList\?\.\(\); unsubscribeEnvironment\?\.\(\); if \(fieldListScopeAvailable\) fieldList\.detach\(ownedContext\(\)\);\s*cancelPreferenceRead\(\); onBusyChange\(false\);\s*\}\)/);
  assert.match(app, /closeDisposition\(state, busy \|\| editorBusy \|\| transitionWorking/);
  for (const name of ['loadPlots', 'refresh', 'loadHierarchy']) {
    assert.match(app, new RegExp(`async function ${name}\\([^)]*\\) \\{\\s*if \\([^\\n]*view === 'summary-vegetation'[^\\n]*editorBusy\\) return`));
  }
});

test('source labels, unsupported options and DOM-value-before-validation remain explicit', () => {
  for (const label of ['Summary Report Units', 'Group by', 'Display Values', 'Calculate', 'By n Plots', 'Characteristic']) assert.ok(component.includes(label));
  assert.match(component, /<option value=\{4\}>Prominence Class/);
  assert.match(component, /<option value=\{5\}>Goldstream Class/);
  assert.match(component, /viewState\.display <= 3/);
  assert.match(component, /data-summary-show/);
  assert.match(component, /const raw = \(event\.currentTarget as HTMLSelectElement\)\.value;\s*if \(preferences\)/);
  assert.match(component, /const current = Number\(raw\);\s*session\.edit/);
  assert.match(component, /grid-template-columns: minmax\(0, 1fr\)/);
  assert.match(component, /overflow-x: auto/);
});

test('compiled panel renders retained production result with labels and immutable NULL blanks', async () => {
  const lifeform = loadTypeScript('lifeformSummary.ts');
  const summary = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform }, { setTimeout });
  const wire = JSON.parse(readFileSync(path.join(__dirname, 'summaryVegetationWire.json'), 'utf8'));
  const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire[6][key]]));
  const session = summary.summaryVegetationSession(owner);
  session.edit(1, 6);
  await session.show({ preview: async () => structuredClone(wire[6]), cancel() {} });
  const panel = serverComponent(component, 'SummaryVegetationReport.svelte', {
    './summaryVegetation': summary,
    '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationService: { Preview() { throw new Error('Rendering must not replay a read'); } } },
  }, { VITE_SUMMARY_VEGETATION: 'true' });
  const held = [];
  let body = render(panel, { props: { ...owner, onBusyChange: value => held.push(value) } }).body;
  assert.match(body, /10\.00/);
  assert.match(body, /13\.33|5\.00/);
  assert.match(body, /Physical admission and calculation provenance/);
  assert.match(body, /aria-label="S — B"[^>]*><\/td>/);
  assert.match(body, /<label[^>]*>Calculate/);
  assert.match(body, /<option value="4"/);
  assert.doesNotMatch(body.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
  session.edit(1, 4);
  body = render(panel, { props: { ...owner, onBusyChange: value => held.push(value) } }).body;
  assert.ok(held.includes(true));
  assert.match(body, /Choose a supported Calculate and Display Values option/);
  assert.match(body.match(/<button[^>]*data-summary-preview[^>]*>/)[0], /\bdisabled\b/);
  assert.doesNotMatch(body.match(/<select[^>]*data-summary-display[^>]*>/)[0], /\bdisabled\b/);
});
