const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { compile } = require('svelte/compiler');
const ts = require('typescript');
const { componentFunctions, loadTypeScript } = require('./svelteTestHelpers.cjs');

const owner = { contextId: 'owned-context', project: 'OwnedProject', plot: 'P1' };
const forms = ['FS882-8x6XL', 'FS882-8x6XL-CHARS'];
const cell = (storage, text = null) => ({ storage, text, integer: null, real: null, blobHex: null });
const clean = () => ({ unsaved: false, busy: false, blocked: false, error: null, canSave: false });
let compiledScript;

async function mountEntry(flags = {}, options = {}) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  if (!compiledScript) {
    let script = readFileSync(path.join(__dirname, 'TwoPageEntryEditor.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
    script = script.replace('projectMetadataClient, metadataRestorationAvailable', 'inspect, projectMetadataClient, metadataRestorationAvailable');
    script += `\n$effect(() => inspect({
      canOpenMetadataEditor, openMetadataEditor, getCloseState, getDraftState, undo, cancel,
      stage, selectProject, acknowledge, operation, chooseForm, closeReview, refreshMetadataPeers,
      openProjectMetadata, returnFromProjectMetadata, mode: projectMetadataOpen, error, form, session,
      closeState: getCloseState(), draftState: getDraftState(), client: metadataClient,
      attachMetadata: value => metadataEditor = value
    }));`;
    const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
    assert.equal(compiled.warnings.length, 0);
    const ast = ts.createSourceFile('two-page-metadata-client.js', compiled.js.code, ts.ScriptTarget.Latest, true);
    compiledScript = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast))
      .join('\n').replace('export default function', 'function').replaceAll('import.meta.env', '__testEnvironment');
  }
  let props, actions;
  const calls = [];
  class Session {
    constructor(form) {
      this.form = form;
      this.state = { original: { projectId: cell('text', 'Project') }, historyId: `${form}-history`,
        error: null, referenceError: null, drafts: {}, busy: false, referencesBusy: false };
      this.close = clean();
      this.fail = false;
    }
    view() { return this.state; }
    closeState() { return this.close; }
    async load() {
      calls.push(this.form);
      this.state.original = this.fail ? null : { projectId: cell('text', 'Project') };
      this.state.error = this.fail ? 'held peer read failure' : null;
      props.revision++;
      return !this.fail;
    }
    cancel() { throw new Error('No parent cancellation is authorized by metadata ownership'); }
  }
  const sessions = new Map(forms.map(form => [form, new Session(form)]));
  const cache = new Map([[JSON.stringify(owner), sessions]]);
  const client = { owned: true };
  props = runtime.proxy({ ...owner, cache, revision: 0, projectMetadataClient: client,
    sourceBlocked: false, childBlocked: false, initialForm: forms[0],
    onchange: () => props.revision++, onbusy: value => calls.push(['busy', value]),
    onclosed: () => calls.push('close'), oncommitted: async () => calls.push('root'),
    embedded() {}, inspect: value => actions = value, ...options });
  const mount = new Function('$', 'onDestroy', 'untrack', 'twoPageEntryCell', '__testEnvironment',
    compiledScript + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack,
    (original, column) => { assert.equal(column, 'ProjectID'); return original.projectId; }, {
      VITE_TWO_PAGE_PARENT_REVIEW: 'true', VITE_TWO_PAGE_PARENT_ENTRY_REVIEW: 'true',
      VITE_PROJECT_METADATA_EDITING: 'true', ...flags,
    });
  const dispose = runtime.effect_root(() => mount(null, props));
  runtime.flush();
  return { actions: () => actions, props, sessions, cache, calls, dispose, flush: runtime.flush };
}

test('actual complete-entry client requires all literal metadata prerequisites, independent of parent editing', async () => {
  const names = ['VITE_TWO_PAGE_PARENT_REVIEW', 'VITE_TWO_PAGE_PARENT_ENTRY_REVIEW', 'VITE_PROJECT_METADATA_EDITING'];
  for (const name of names) for (const value of [undefined, false, 'false', 'TRUE', '1', 'true']) {
    const panel = await mountEntry({ [name]: value, VITE_TWO_PAGE_PARENT_ENTRY_EDITING: 'false' });
    try {
      assert.equal(panel.actions().canOpenMetadataEditor(), value === 'true', `${name}=${value}`);
      if (value !== 'true') {
        panel.actions().openMetadataEditor(); panel.flush();
        assert.equal(panel.actions().mode, null); assert.match(panel.actions().error, /Resolve/);
      }
    } finally { panel.dispose(); }
  }
});

test('actual client retains literal saved identity and all peer/root/child barriers', async () => {
  for (const source of [cell('null'), cell('integer'), cell('blob'), cell('text', ''), cell('text', 'Project'), cell('text', ' Project ')]) {
    const panel = await mountEntry();
    try {
      panel.sessions.get(forms[0]).state.original.projectId = source;
      panel.props.revision++; panel.flush();
      assert.equal(panel.actions().canOpenMetadataEditor(), source.storage === 'text' && source.text !== '');
      const peer = panel.sessions.get(forms[1]);
      for (const key of ['unsaved', 'busy', 'blocked']) {
        peer.close[key] = true; panel.props.revision++; panel.flush();
        assert.equal(panel.actions().canOpenMetadataEditor(), false);
        peer.close[key] = false;
      }
      for (const key of ['error', 'referenceError']) {
        peer.state[key] = 'retained peer error'; panel.props.revision++; panel.flush();
        assert.equal(panel.actions().canOpenMetadataEditor(), false);
        peer.state[key] = null;
      }
      for (const key of ['sourceBlocked', 'childBlocked']) {
        panel.props[key] = true; panel.flush(); assert.equal(panel.actions().canOpenMetadataEditor(), false);
        panel.props[key] = false; panel.flush();
      }
    } finally { panel.dispose(); }
  }
});

test('actual metadata mode delegates validation/busy/Undo and bars competing source mutations', async () => {
  const panel = await mountEntry();
  try {
    const initialClient = panel.actions().client;
    panel.props.projectMetadataClient = { foreign: true };
    panel.flush(); assert.equal(panel.actions().client, initialClient);
    panel.actions().openMetadataEditor(); panel.flush();
    assert.equal(panel.actions().mode, 'edit');
    assert.equal(panel.actions().canOpenMetadataEditor(), false);
    panel.actions().returnFromProjectMetadata(); panel.flush();
    assert.equal(panel.actions().mode, 'edit');
    assert.match(panel.actions().error, /finish editing in its owned editor/);
    let undos = 0;
    const close = panel.props.metadataClose = { unsaved: true, busy: true, blocked: true, error: 'raw metadata validation', canSave: false };
    panel.actions().attachMetadata({ getCloseState: () => close, undo: () => undos++ });
    panel.flush();
    assert.equal(panel.actions().closeState.error, 'raw metadata validation');
    assert.equal(panel.actions().draftState.busy, true);
    assert.equal(panel.actions().draftState.blocked, true);
    panel.actions().undo(); assert.equal(undos, 1);
    panel.actions().stage('Latitude', { kind: 'text', raw: '54' }); assert.match(panel.actions().getDraftState().error, /no field/);
    panel.actions().selectProject('1'); assert.match(panel.actions().getDraftState().error, /no ProjectID/);
    panel.actions().acknowledge('Latitude'); assert.match(panel.actions().getDraftState().error, /no literal/);
    for (const action of ['load', 'save', 'undo', 'retain', 'prune', 'references']) {
      await panel.actions().operation(action);
      assert.match(panel.actions().getDraftState().error, /project metadata review/);
    }
    panel.actions().cancel(); assert.match(panel.actions().getDraftState().error, /cancel its own read/);
    await panel.actions().chooseForm(forms[1]); panel.flush(); assert.equal(panel.actions().form, forms[0]);
    panel.actions().closeReview(); assert.ok(!panel.calls.includes('close'));
    assert.equal(panel.cache.get(JSON.stringify(owner)), panel.sessions);
  } finally { panel.dispose(); }
});

test('read-only metadata refresh preserves both session histories and recovers failed originals without replay', async () => {
  const panel = await mountEntry();
  try {
    const normal = panel.sessions.get(forms[0]), chars = panel.sessions.get(forms[1]);
    const histories = [...panel.sessions.values()].map(session => session.state.historyId);
    chars.fail = true;
    await assert.rejects(panel.actions().refreshMetadataPeers(), /held peer read failure/);
    assert.equal(chars.state.original, null);
    chars.fail = false;
    await panel.actions().refreshMetadataPeers();
    panel.flush();
    assert.equal(panel.actions().session, normal);
    assert.equal(panel.sessions.get(forms[1]), chars);
    assert.deepEqual([...panel.sessions.values()].map(session => session.state.historyId), histories);
    assert.deepEqual(panel.calls.filter(call => typeof call === 'string'), ['root', ...forms, 'root', ...forms]);
    chars.close.unsaved = true;
    await assert.rejects(panel.actions().refreshMetadataPeers(), /could not refresh/);
    assert.equal(chars.state.historyId, histories[1]);
  } finally { panel.dispose(); }
});

test('metadata recovery retries failed outer cohorts through the actual root callback and production reload', async () => {
  const { writeSession, original, cell: sourceCell, setCell } = require('./siviParentTestHelpers.cjs');
  for (const failedScope of ['extra', 'common']) {
    let writes = 0, fail = false, rootReads = 0;
    const reads = { extra: 0, common: 0 };
    const key = JSON.stringify(owner);
    const caches = {};
    const retained = {};
    for (const scope of ['extra', 'common']) {
      const current = original();
      current.ContextID = owner.contextId; current.Project = owner.project; current.Plot = owner.plot;
      current.EnvTable = `${owner.project}_Env`; current.AdminTable = `${owner.project}_Admin`;
      for (const binding of current.Bindings) {
        binding.Table = binding.Table.endsWith('_Admin') ? current.AdminTable : current.EnvTable;
      }
      setCell(current, 'PlotNumber', sourceCell('text', owner.plot));
      for (const column of current.AdminColumns.keys()) {
        if (current.AdminColumns[column].name === 'Plot') current.Rows[0].Admin.cells[column] = sourceCell('text', owner.plot);
      }
      const peer = new writeSession.SIVIParentWriteSession(owner, {
        read: async () => {
          reads[scope]++;
          if (scope === failedScope && fail) throw new Error(`held ${scope} cohort read failure`);
          return structuredClone(current);
        },
        save: async () => {
          writes++; setCell(current, 'SV_PolygonNumber', sourceCell('text', 'retained'));
          return { ChangedCells: 1, HistoryID: scope === 'extra' ? '801' : '802' };
        },
        restore: async () => assert.fail('Metadata recovery must not restore a cohort'),
        refreshParent: async () => {}, cancelRead() {},
      }, () => {});
      assert.equal(await peer.load(), true, peer.view().error);
      peer.stage('SV_PolygonNumber', { kind: 'text', raw: 'retained' });
      assert.equal(await peer.save(), true, peer.view().error);
      retained[scope] = peer;
      caches[scope] = new Map([[key, new Map([[forms[0], peer]])]]);
    }
    const host = componentFunctions('FS882Form.svelte', ['refreshTwoPagePeers'], {
      contextId: owner.contextId, twoPageProject: owner.project, draft: { plotNumber: owner.plot },
      twoPageSessions: caches.extra, twoPageCommonSessions: caches.common, twoPageEntrySessions: new Map(),
      refreshSIVIParent: async () => rootReads++,
      siviParentWriteSession: null, siviParentActionSession: null,
      siviProjectAssignmentSession: null, siviParentSharedSession: null,
    });
    const panel = await mountEntry({}, { oncommitted: () => host.actions.refreshTwoPagePeers('entry') });
    try {
      fail = true;
      await assert.rejects(panel.actions().refreshMetadataPeers(), new RegExp(`held ${failedScope} cohort read failure`));
      assert.equal(retained[failedScope].view().original, null);
      const firstFailedReads = reads[failedScope];
      await assert.rejects(panel.actions().refreshMetadataPeers(), new RegExp(`held ${failedScope} cohort read failure`));
      assert.equal(reads[failedScope], firstFailedReads + 1);
      assert.equal(panel.calls.filter(call => typeof call === 'string').length, 0);
      fail = false;
      await panel.actions().refreshMetadataPeers();
      assert.equal(rootReads, 3);
      assert.equal(reads[failedScope], firstFailedReads + 2);
      assert.deepEqual(panel.calls.filter(call => typeof call === 'string'), forms);
      for (const scope of ['extra', 'common']) {
        assert.equal(caches[scope].get(key).get(forms[0]), retained[scope]);
        assert.notEqual(retained[scope].view().original, null);
        assert.equal(retained[scope].view().error, null);
        assert.equal(retained[scope].view().historyId, scope === 'extra' ? '801' : '802');
      }
      assert.equal(writes, 2, 'Only fixture history preparation may write; postcommit retries are reads');
    } finally { panel.dispose(); }
  }
});

test('original action and Forms menu use one owned opener without enabling another two-page scope', () => {
  const source = readFileSync(path.join(__dirname, 'TwoPageEntryEditor.svelte'), 'utf8');
  const root = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(root, /sourceBlocked=\{twoPageLinkedParentBlocked \|\| Object\.keys\(headerValidation\)\.length > 0/);
  assert.match(source, /disabled=\{!canOpenMetadataEditor\(\)\} onclick=\{openMetadataEditor\}/);
  assert.match(source, /<ProjectMetadataEditor[\s\S]*?oncommitted=\{refreshMetadataPeers\}/);
  const child = { canOpenMetadataEditor: () => true, openMetadataEditor: () => opened++ };
  let opened = 0;
  const host = componentFunctions('FS882Form.svelte', ['canOpenProjectMetadata', 'openProjectMetadata'], {
    metadataEnabled: true, twoPageOpen: true, twoPageScope: 'entry', twoPageEditor: child,
    headerValidation: {}, metadataOpen: false, error: null, successMsg: 'old',
  });
  assert.equal(host.actions.canOpenProjectMetadata(), true);
  host.actions.openProjectMetadata(); assert.equal(opened, 1); assert.equal(host.metadataOpen, false);
  for (const scope of ['extra', 'common']) {
    host.twoPageScope = scope; assert.equal(host.actions.canOpenProjectMetadata(), false);
  }
  host.twoPageScope = 'entry'; host.headerValidation = { raw: 'invalid' };
  assert.equal(host.actions.canOpenProjectMetadata(), false);
  host.headerValidation = {}; host.metadataEnabled = false;
  assert.equal(host.actions.canOpenProjectMetadata(), false);
  host.metadataEnabled = true; child.openMetadataEditor = undefined;
  assert.equal(host.actions.canOpenProjectMetadata(), false);
});

test('in-place metadata preserves its owner while ordinary navigation still requires explicit review closure', async () => {
  const { closeDisposition } = loadTypeScript('closeLifecycle.ts');
  const make = overrides => componentFunctions('App.svelte', ['requestTransition'], {
    document: { activeElement: null }, HTMLElement: class {}, tick: async () => {}, closeDisposition,
    view: 'fs882', editor: { getCloseState: () => ({ unsaved: true, busy: true, canSave: false }) },
    busy: false, editorBusy: true, transitionWorking: false, archivePublicationBusy: false,
    vegetationPublicationBusy: false, lifeformPublicationBusy: false, summaryPublicationBusy: false,
    pendingTransition: null, closeRequest: null, $projectState: { contextId: owner.contextId },
    error: '', transitionError: '', ...overrides,
  });
  for (const key of ['busy', 'transitionWorking', 'archivePublicationBusy', 'vegetationPublicationBusy',
    'lifeformPublicationBusy', 'summaryPublicationBusy', 'pendingTransition', 'closeRequest']) {
    const host = make({ [key]: true });
    await host.actions.requestTransition(async () => assert.fail('Global ownership must remain held'), () => true);
    assert.match(host.error, /Wait for the current operation/);
  }
  const calls = [];
  const owned = make();
  await owned.actions.requestTransition(async context => calls.push(context), () => true);
  assert.deepEqual(calls, [owner.contextId]); assert.equal(owned.pendingTransition, null);
  const stale = make();
  await stale.actions.requestTransition(async () => assert.fail('Stale action'), () => false);
  assert.match(stale.error, /availability changed/); assert.equal(stale.pendingTransition, null);
  const navigation = make({ editorBusy: false,
    editor: { getCloseState: () => ({ unsaved: true, busy: false, canSave: false }) } });
  await navigation.actions.requestTransition(async () => assert.fail('Review must remain retained'));
  assert.equal(navigation.pendingTransition.contextId, owner.contextId);
  let available = true;
  const blurred = make({ tick: async () => { available = false; } });
  await blurred.actions.requestTransition(async () => assert.fail('Recheck after blur'), () => available);
  assert.match(blurred.error, /availability changed/);
});

test('actual Forms dispatch requires the explicit freshly retained metadata capability', async () => {
  const { closeDisposition } = loadTypeScript('closeLifecycle.ts');
  for (const retained of [false, true]) for (const late of [false, true]) {
    let opened = 0, current = true;
    const editor = {
      getCloseState: () => ({ unsaved: true, busy: false, canSave: false }),
      canOpenProjectMetadata: () => current, canOpenMetadataInPlace: () => retained && current,
      openProjectMetadata: () => opened++,
    };
    const host = componentFunctions('App.svelte', ['requestTransition', 'openProjectMetadata'], {
      document: { activeElement: null }, HTMLElement: class {}, tick: async () => { if (late) current = false; },
      closeDisposition, view: 'fs882', editor, busy: false, editorBusy: retained,
      transitionWorking: false, archivePublicationBusy: false, vegetationPublicationBusy: false,
      lifeformPublicationBusy: false, summaryPublicationBusy: false, pendingTransition: null,
      closeRequest: null, $projectState: { contextId: owner.contextId }, error: '', transitionError: '',
    });
    await host.actions.openProjectMetadata();
    assert.equal(opened, retained && !late ? 1 : 0);
    if (!retained) assert.equal(typeof host.pendingTransition.run, 'function');
    else assert.equal(host.pendingTransition, null);
    if (retained && late) assert.match(host.error, /availability changed/);
  }
});
