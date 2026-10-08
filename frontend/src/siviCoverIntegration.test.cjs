const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');

const parent = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
const script = parent.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
const ast = ts.createSourceFile('host.ts', script, ts.ScriptTarget.Latest, true);

function host(overrides = {}) {
  const names = ['siviCoverOperation', 'stageSIVICoverCell', 'showSIVICoverPanel', 'refreshSIVIChildEditors'];
  const functions = ast.statements.filter(node => ts.isFunctionDeclaration(node) && names.includes(node.name.text));
  assert.equal(functions.length, names.length);
  const code = functions.map(node => node.getText(ast)).join('\n') +
    `\nglobalThis.api = {${names.join(',')}};`;
  const context = {
    error: null, successMsg: null, siviCoverEnabled: true, busy: false, headerWorkflowBusy: false,
    siviUnsaved: false, heightUnsaved: false, otherUnsaved: false, soilUnsaved: false,
    attributeUnsaved: false, collectedUnsaved: false, speciesUnsaved: false,
    siviParentWriteUnsaved: false, siviParentActionUnsaved: false,
    siviProjectAssignmentUnsaved: false, siviParentSharedUnsaved: false,
    siviCoverEditingDisabled: false, siviCoverReading: false, siviCoverUnsaved: false,
    siviCoverPanelOpen: false, siviPanelOpen: true, siviCoverView: { review: [{}] },
    siviCoverSession: null, siviSession: null,
    AuditRestoreAction: { AuditRestoreRetain: 'retain', AuditRestorePrune: 'prune' },
    loadChildData: async () => {},
    ...overrides,
  };
  vm.createContext(context);
  vm.runInContext(ts.transpileModule(code, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.None },
  }).outputText, context);
  return context;
}

test('cover facade and source panel are independently gated without widening height saves', () => {
  assert.equal(compile(parent, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(parent, /VITE_SIVI_COVER_EDITING === 'true'/);
  for (const method of ['GetOriginal', 'SaveReviewed', 'RestoreReviewed']) {
    assert.ok(parent.includes(`SIVICoverService.${method}(`));
  }
  assert.match(parent, /read: extended => siviCoverReads\.track\(SIVICoverService\.GetOriginal/);
  assert.match(parent, /siviCoverSession\?\.dispose\(\);[\s\S]{0,70}siviCoverReads\.cancelAll\(\)/);
  assert.match(parent, /data-sivi-cover-cancel-read[\s\S]{0,110}siviCoverReads\.cancelAll/);
  assert.match(parent, /siviCoverSession = new SIVICoverSession/);
});

test('cover owner contributes dirty busy blocked Save Undo Lock and peer-editor barriers', () => {
  assert.match(parent, /headerWorkflowBusy = \$derived\([^\n]*\|\| siviCoverBusy\)/);
  assert.match(parent, /nonParentChildUnsaved = \$derived\([^\n]*\|\| siviCoverUnsaved/);
  assert.match(parent, /if \(siviCoverUnsaved\) \{ await siviCoverOperation\('save'\); return; \}/);
  assert.match(parent, /if \(siviCoverUnsaved\) \{ void siviCoverOperation\('undo'\); return; \}/);
  assert.match(parent, /\|\| \(siviCoverClose\?\.blocked \?\? false\)/);
  assert.match(parent, /siviCoverSession\.closeState\(\)\.unsaved \|\| siviCoverSession\.closeState\(\)\.busy/);
  assert.match(parent, /SIVI covers or unresolved acknowledgements must be saved or explicitly undone before changing the plot lock/);
  for (const gate of ['height', 'other', 'soil', 'attribute', 'collected', 'species', 'sivi']) {
    assert.match(parent, new RegExp(`${gate}EditingDisabled = \\$derived\\([^\\n]*\\|\\| siviCoverUnsaved`));
  }
});

test('cover operations reject every peer draft and never invoke mutation during another operation', async () => {
  for (const guard of ['busy', 'headerWorkflowBusy', 'siviUnsaved', 'heightUnsaved', 'otherUnsaved',
    'soilUnsaved', 'attributeUnsaved', 'collectedUnsaved', 'speciesUnsaved', 'siviParentWriteUnsaved',
    'siviParentActionUnsaved', 'siviProjectAssignmentUnsaved', 'siviParentSharedUnsaved']) {
    let calls = 0;
    const context = host({ [guard]: true, siviCoverSession: { save: async () => { calls++; return true; } } });
    assert.equal(await context.api.siviCoverOperation('save'), false, guard);
    assert.equal(calls, 0, guard);
    assert.match(context.error, /no conflicting drafts or operation/);
  }
});

test('cover read cancellation state exists only for loading, never for mutation or restoration', async () => {
  for (const operation of ['load', 'save', 'undo', 'retain', 'prune']) {
    let reading;
    let action;
    const session = {
      load: async () => { reading = context.siviCoverReading; return true; },
      save: async () => { reading = context.siviCoverReading; return true; },
      undo: async () => { reading = context.siviCoverReading; return true; },
      restore: async selected => { reading = context.siviCoverReading; action = selected; return true; },
    };
    const context = host({ siviCoverSession: session });
    assert.equal(await context.api.siviCoverOperation(operation), true);
    assert.equal(reading, operation === 'load');
    assert.equal(context.siviCoverReading, false);
    if (operation === 'retain' || operation === 'prune') assert.equal(action, operation);
  }
});

test('invalid cover drafts remain correctable while blocked authority uses explicit Undo recovery', async () => {
  let staged = 0;
  let undone = 0;
  const context = host({
    siviCoverSession: {
      stage: () => { staged++; },
      view: () => ({ error: 'less than 100' }),
      undo: async () => { undone++; return true; },
    },
  });
  context.api.stageSIVICoverCell('1', 'Cover1', '100', false);
  assert.equal(staged, 1);
  assert.match(context.error, /SIVI cover less than 100/);
  context.siviCoverEditingDisabled = true;
  context.api.stageSIVICoverCell('1', 'Cover1', '99', false);
  assert.equal(staged, 1);
  assert.equal(await context.api.siviCoverOperation('undo'), true);
  assert.equal(undone, 1);
});

test('panel switches do not replace or reload the retained cover draft owner', async () => {
  let loads = 0;
  const context = host({
    siviCoverUnsaved: true, siviCoverView: { review: null }, siviCoverEditingDisabled: true,
    siviCoverSession: { load: async () => { loads++; return true; } },
  });
  const owner = context.siviCoverSession;
  await context.api.showSIVICoverPanel();
  assert.equal(context.siviCoverPanelOpen, true);
  assert.equal(context.siviPanelOpen, false);
  await context.api.showSIVICoverPanel();
  assert.equal(context.siviCoverPanelOpen, false);
  assert.equal(context.siviCoverSession, owner);
  assert.equal(loads, 0);
});

test('acknowledged child changes refresh only an already-loaded peer and preserve its session', async () => {
  for (const owner of ['height', 'cover']) {
    const calls = [];
    const peer = { view: () => ({ review: [{}] }), load: async () => { calls.push('peer'); return true; } };
    const context = host({
      loadChildData: async plot => { calls.push(plot); },
      siviCoverSession: owner === 'height' ? peer : null,
      siviSession: owner === 'cover' ? peer : null,
    });
    await context.api.refreshSIVIChildEditors('P', owner);
    assert.deepEqual(calls, ['P', 'peer']);
    assert.equal(owner === 'height' ? context.siviCoverSession : context.siviSession, peer);
    peer.view = () => ({ review: null });
    calls.length = 0;
    await context.api.refreshSIVIChildEditors('P', owner);
    assert.deepEqual(calls, ['P']);
    peer.view = () => ({ review: [{}] });
    peer.load = async () => false;
    await assert.rejects(context.api.refreshSIVIChildEditors('P', owner), /failed to refresh/);
  }
});
