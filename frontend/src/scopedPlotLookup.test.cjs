const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const find = loadTypeScript('scopedPlotLookup.ts', { './qualityEditor': quality });
const result = (plotNumber = " A' # ") => ({ contextId: 'owned', plot: {
  plotNumber, fieldNumber: null, plotRepresenting: null, zone: null, subZone: null, siteSeries: null
}});

test('Find preserves exact spaces, quotes, case, Unicode and historical long identifiers', () => {
  for (const number of [" A' # ", '108050x', 'É😀', 'historical long plot identifier']) {
    assert.equal(find.plotLookupInputError(number), null);
    assert.equal(find.validateScopedPlotLookup(result(number), number, 'owned').plot.plotNumber, number);
  }
  for (const number of ['', '\ud800', '\udc00', '\0']) assert.notEqual(find.plotLookupInputError(number), null);
});

test('Find rejects incomplete, normalized or foreign responses rather than opening another record', () => {
  for (const alter of [value => value.contextId = 'stale', value => value.plot.plotNumber = "A' #",
    value => value.plot = null, value => delete value.plot.fieldNumber, value => value.plot.zone = 5]) {
    const invalid = result(); alter(invalid);
    assert.throws(() => find.validateScopedPlotLookup(invalid, " A' # ", 'owned'));
  }
});

test('Read-only Find and explicit Open retain SU/profile scope, immutable identity and shared transition ownership', () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(app, /VITE_SOURCE_PLOT_FIND === 'true'/);
  assert.match(app, /aria-label="Find scoped plot"/);
  assert.match(app, /<label for="find-scoped-plot"/);
  assert.match(form, /\{#if onFindPlot\}/);
  assert.match(form, /\{onFindPlot\} findPlotDisabled=\{busy \|\| headerWorkflowBusy \|\| !capabilitiesReady\}/);
  for (const name of ['lookupPlot', 'findScopedPlot', 'openFoundPlot']) {
    const start = app.indexOf(`async function ${name}`);
    const action = app.slice(start, app.indexOf('\n  async function ', start + 1));
    assert.doesNotMatch(action, /SwitchContext|profileNavigation = null|CreatePlot|\.trim\(|\.toUpperCase\(/);
    if (name === 'lookupPlot') {
      assert.match(action, /await resolveNavigation\(source.proposal, contextId\)/);
      assert.match(action, /next.plots.some/);
      assert.match(action, /ContextService.LookupScopedPlot/);
    }
    if (name === 'findScopedPlot') assert.doesNotMatch(action, /requestTransition|editorPlotNumber =/);
    if (name === 'openFoundPlot') {
      assert.match(action, /\$state.snapshot\(plotFindResult\)/);
      assert.match(action, /proposal.plot.plotNumber === editorPlotNumber/);
      assert.match(action, /await requestTransition/);
      assert.match(action, /await lookupPlot\(proposal.plot.plotNumber, contextId\)/);
      assert.match(action, /JSON.stringify\(current\) !== JSON.stringify\(proposal\)/);
    }
  }
});
