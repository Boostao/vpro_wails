const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { componentFunctions } = require('./svelteTestHelpers.cjs');
const ts = require('typescript');
const vm = require('node:vm');

test('actual top-level Summary authority effect subscribes and reopens a retained receipt without the panel', () => {
  const source = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('App.svelte.ts', script, ts.ScriptTarget.Latest, true);
  const effects = ast.statements.filter(node => ts.isExpressionStatement(node) &&
    ts.isCallExpression(node.expression) && node.expression.expression.getText(ast) === '$effect');
  const effect = effects.filter(node => node.getText(ast).includes('siteUnitSummaryWorkbookPublicationSession'));
  assert.equal(effect.length, 1, 'Summary subscription must be an independently reachable top-level effect');
  for (const enabled of [true, false]) for (const blocked of [true, false]) {
    let subscribed = 0, unsubscribed = 0, owner;
    const context = vm.createContext({
      summaryPublicationBusy: false, view: 'home',
      $projectState: { contextId: 'owned', activeProject: 'Sample', projectPath: 'C:\\Sample.db',
        activeSU: 'Summary', suPath: 'C:\\external.db' },
      $effect(run) { this.cleanup = run(); },
      siteUnitSummaryWorkbookPublicationSession(value) {
        owner = value;
        return { view: () => ({ busy: !blocked, blocked }), subscribe(listener) {
          subscribed++; listener(); return () => { unsubscribed++; };
        } };
      },
    });
    context.$effect = run => { context.cleanup = run(); };
    const code = effect[0].getText(ast).replaceAll("import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK",
      JSON.stringify(enabled ? 'true' : 'false')).replaceAll("import.meta.env.VITE_SITE_UNIT_SUMMARY", "'true'");
    vm.runInContext(ts.transpileModule(code, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
    assert.equal(subscribed, enabled ? 1 : 0);
    assert.equal(context.summaryPublicationBusy, enabled);
    assert.equal(context.view, enabled && blocked ? 'summary-environment' : 'home');
    if (enabled) {
      assert.equal(owner.contextId, 'owned'); assert.equal(owner.suPath, 'C:\\external.db');
      context.cleanup(); assert.equal(unsubscribed, 1);
    }
  }
});

test('actual Summary root guards refuse native close and context changes with an unmounted retained publication', async () => {
  let cancelled = 0, confirmed = 0, navigated = 0;
  const context = componentFunctions('App.svelte', ['handleCloseRequest', 'respondToClose', 'requestTransition'], {
    closeWorking: false, closeRequest: '', closeError: '', error: '', view: 'home', editor: undefined,
    busy: false, editorBusy: false, transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: true,
    pendingTransition: null, transitionError: '', $projectState: { contextId: 'owned' },
    document: { activeElement: null }, HTMLElement: class {}, tick: async () => {},
    closeDisposition: (state, busy) => busy ? 'busy' : 'clean',
    CloseService: { GetPendingCloseRequest: async () => 'request',
      CancelClose: async () => { cancelled++; }, ConfirmClose: async () => { confirmed++; } },
  });
  await context.actions.handleCloseRequest('request');
  assert.equal(cancelled, 1); assert.equal(confirmed, 0);
  context.closeRequest = 'request';
  await context.actions.respondToClose('discard');
  assert.equal(confirmed, 0); assert.match(context.closeError, /Wait/);
  context.closeRequest = '';
  await context.actions.requestTransition(async () => { navigated++; });
  assert.equal(navigated, 0); assert.match(context.error, /Wait/);
  context.summaryPublicationBusy = false;
  await context.actions.requestTransition(async () => { navigated++; });
  assert.equal(navigated, 1);
});

test('actual Summary publication blocks background plot, hierarchy, state and profile reads', async () => {
  const context = componentFunctions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy', 'inspectProfileSources',
    'reviewProfileSU', 'findScopedPlot', 'reviewProfileWriteOwnership', 'reviewBlankProfileFile', 'reviewBlankProfileTable'], {
    summaryPublicationBusy: true, lifeformPublicationBusy: false, vegetationPublicationBusy: false,
    view: 'home', editorBusy: false, busy: false, transitionWorking: false, pendingTransition: null,
    closeRequest: '', request: 0, stateRequest: 0, hierarchyRequest: 0, plotFindEnabled: true,
    $projectState: { contextId: 'owned' }, profileSourceError: '', profileFileError: '', profileTableError: '',
  });
  for (const name of ['loadPlots', 'refresh', 'loadHierarchy', 'inspectProfileSources', 'findScopedPlot',
    'reviewProfileWriteOwnership', 'reviewBlankProfileFile', 'reviewBlankProfileTable']) {
    await context.actions[name](0);
  }
  await assert.rejects(context.actions.reviewProfileSU({}, 'owned'), /Wait/);
  assert.equal(context.request, 0); assert.equal(context.stateRequest, 0); assert.equal(context.hierarchyRequest, 0);
  assert.equal(context.busy, false);
  for (const name of ['profileSourceError', 'profileFileError', 'profileTableError']) assert.match(context[name], /Wait/);
});

test('actual Summary parent prevents method, preview and preference changes while publication is held', async () => {
  const held = [];
  const context = componentFunctions('SiteUnitSummary.svelte',
    ['reportBusy', 'cancel', 'options', 'show', 'loadPreferences', 'savePreferences', 'editMethod'], {
      workbookBusy: true, busy: false, preferenceBarrier: false, method: 1,
      generation: 0, onBusyChange: value => held.push(value),
      reads: { cancelAll() {} }, preferences: { cancelLoad() {} },
    });
  await context.actions.options(); await context.actions.show();
  await context.actions.loadPreferences(); await context.actions.savePreferences(); context.actions.editMethod(2);
  assert.equal(context.method, 1); assert.equal(context.generation, 0);
  context.actions.cancel();
  assert.equal(context.generation, 1); assert.deepEqual(held, [true]);
  context.workbookBusy = false; context.preferenceBarrier = true;
  context.actions.reportBusy();
  assert.deepEqual(held, [true, true]);
  context.preferenceBarrier = false;
  context.actions.reportBusy();
  assert.deepEqual(held, [true, true, false]);
});

test('Summary root and parent wire independent default-off gates and persistent publication authority', () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const parent = readFileSync(path.join(__dirname, 'SiteUnitSummary.svelte'), 'utf8');
  assert.match(app, /VITE_SITE_UNIT_SUMMARY_WORKBOOK !== 'true' \|\| import\.meta\.env\.VITE_SITE_UNIT_SUMMARY !== 'true'/);
  assert.match(app, /summaryPublicationBusy = publication.busy \|\| publication.blocked/);
  assert.match(app, /publication.blocked && view === 'home'\) view = 'summary-environment'/);
  assert.match(parent, /VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true'/);
  assert.match(parent, /<SiteUnitSummaryWorkbookPanel owner=\{\{ contextId, project, projectPath, su, suPath \}\} \{method\}/);
  assert.match(parent, /onBusyChange=\{\(held\) => \{ workbookBusy = held; reportBusy\(\); \}\}/);
  for (const selector of ['undo', 'normal', 'acknowledge']) {
    assert.match(parent, new RegExp(`data-summary-preferences-${selector} disabled=\\{workbookBusy`));
  }
});
