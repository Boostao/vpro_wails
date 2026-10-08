const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

function functions(filename, names, values) {
  const source = readFileSync(path.join(__dirname, filename), 'utf8');
  const script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile(filename + '.ts', script, ts.ScriptTarget.Latest, true);
  const selected = ast.statements.filter(node => ts.isFunctionDeclaration(node) && names.includes(node.name.text));
  assert.equal(selected.length, names.length);
  const context = vm.createContext(values);
  const code = selected.map(node => node.getText(ast)).join('\n') + `\nthis.actions = {${names.join(',')}};`;
  vm.runInContext(ts.transpileModule(code, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context);
  return context;
}

test('actual root native close and context transition reject retained summary publication even without a mounted panel', async () => {
  let cancelled = 0, confirmed = 0, navigated = 0;
  const context = functions('App.svelte', ['handleCloseRequest', 'respondToClose', 'requestTransition'], {
    closeWorking: false, closeRequest: '', closeError: '', error: '', view: 'home', editor: undefined,
    busy: false, editorBusy: false, transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: true, pendingTransition: null,
    transitionError: '', $projectState: { contextId: 'owned' }, document: { activeElement: null },
    HTMLElement: class {}, tick: async () => {},
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
  context.lifeformPublicationBusy = false;
  await context.actions.requestTransition(async () => { navigated++; });
  assert.equal(navigated, 1);
});

test('actual background plot state hierarchy and profile reads cannot overlap retained summary publication', async () => {
  const context = functions('App.svelte', ['loadPlots', 'refresh', 'loadHierarchy', 'inspectProfileSources',
    'reviewProfileSU', 'findScopedPlot', 'reviewProfileWriteOwnership', 'reviewBlankProfileFile', 'reviewBlankProfileTable'], {
    lifeformPublicationBusy: true, vegetationPublicationBusy: false, view: 'home', editorBusy: false,
    busy: false, transitionWorking: false, pendingTransition: null, closeRequest: '',
    request: 0, stateRequest: 0, hierarchyRequest: 0, plotFindEnabled: true,
    $projectState: { contextId: 'owned' }, profileSourceError: '', profileFileError: '', profileTableError: '',
  });
  await context.actions.loadPlots(0);
  await context.actions.refresh();
  await context.actions.loadHierarchy();
  await context.actions.inspectProfileSources();
  await context.actions.findScopedPlot();
  await context.actions.reviewProfileWriteOwnership();
  await context.actions.reviewBlankProfileFile();
  await context.actions.reviewBlankProfileTable();
  await assert.rejects(context.actions.reviewProfileSU({}, 'owned'), /Wait/);
  assert.equal(context.request, 0); assert.equal(context.stateRequest, 0); assert.equal(context.hierarchyRequest, 0);
  assert.equal(context.busy, false);
  assert.match(context.profileSourceError, /Wait/);
  assert.match(context.profileFileError, /Wait/);
  assert.match(context.profileTableError, /Wait/);
});

test('actual parent preview and cancellation preserve the persistent workbook acknowledgement barrier', async () => {
  const held = [];
  const context = functions('LifeformSummary.svelte', ['reportBusy', 'cancel', 'show'], {
    enabled: true, attributesEnabled: true, busy: false, workbookBusy: true, su: 'Selected',
    generation: 0, reads: { cancelAll() {} }, onBusyChange: value => held.push(value),
  });
  await context.actions.show('lifeform');
  await context.actions.show('attributes');
  assert.equal(context.generation, 0);
  context.actions.cancel();
  assert.equal(context.generation, 1);
  assert.deepEqual(held, [true]);
});

test('root and parent wire the default-off workbook without authorizing hierarchy or incomplete summary vegetation', () => {
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const parent = readFileSync(path.join(__dirname, 'LifeformSummary.svelte'), 'utf8');
  assert.match(app, /VITE_LIFEFORM_WORKBOOK !== 'true' \|\| import\.meta\.env\.VITE_LIFEFORM_SUMMARY !== 'true'/);
  assert.match(app, /lifeformPublicationBusy = publication.busy \|\| publication.blocked/);
  assert.match(app, /publication.blocked && view === 'home'\) view = 'lifeform-summary'/);
  assert.match(parent, /VITE_LIFEFORM_WORKBOOK === 'true'/);
  assert.match(parent, /<LifeformWorkbookPanel owner=\{\{ contextId, project, projectPath, su, suPath \}\} disabled=\{busy\}/);
  assert.match(parent, /onBusyChange=\{\(held\) => \{ workbookBusy = held; reportBusy\(\); \}\}/);
  assert.match(parent, /disabled=\{!enabled \|\| busy \|\| workbookBusy \|\| su === 'None' \|\| su === 'USysSuTableDynamic'\}/);
});
