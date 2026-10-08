const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { runInNewContext } = require('node:vm');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { editor, writeSession, transport, source, cell, metadata } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const shared = loadTypeScript('siviParentSharedSession.ts', {
  '../../resources/fs1333-sivi-layout.json': source, './ordinaryEditor': ordinary,
  './projectMetadataEditor': metadata, './siviParentEditor': editor,
  './siviParentWriteSession': writeSession, './siviParentTransport': transport,
  './coordinateEditor': loadTypeScript('coordinateEditor.ts'),
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
  './referenceEditor': loadTypeScript('referenceEditor.ts', {
    './qualityEditor': quality, './projectMetadataEditor': metadata, './siviParentTransport': transport,
  }),
});
const normal = JSON.parse(readFileSync(require.resolve('../../resources/fs882-two-page-layout.json'), 'utf8'));
const chars = JSON.parse(readFileSync(require.resolve('../../resources/fs882-two-page-chars-layout.json'), 'utf8'));
const api = loadTypeScript('twoPageParentSession.ts', {
  '../../resources/fs882-two-page-layout.json': normal,
  '../../resources/fs882-two-page-chars-layout.json': chars,
  './siviParentEditor': editor, './siviParentTransport': transport, './siviParentWriteSession': writeSession,
  './siviParentSharedSession': shared, './ordinaryEditor': ordinary, './projectMetadataEditor': metadata,
});
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
function original(form = 'FS882-8x6XL') {
  const r = { ContextID: owner.contextId, Project: owner.project, Plot: owner.plot, Form: form,
    Query: 'USysEnv', Membership: 'literal-binary-inner-pairs', EnvTable: 'Project_Env', AdminTable: 'Project_Admin',
    EnvColumns: [{ name: 'PlotNumber', declaredType: 'TEXT' }], AdminColumns: [{ name: 'Plot', declaredType: 'TEXT' }],
    Rows: [{ Env: { rowId: '1', cells: [cell('text', 'P')] }, Admin: { rowId: '901', cells: [cell('text', 'P')] } }],
    Bindings: [] };
  const fields = api.twoPageParentFields(form);
  const source = form === 'FS882-8x6XL' ? normal : chars;
  for (const control of source.forms[0].fields.filter(f => f.binding)) {
    const field = fields.find(f => f.column === control.binding);
    const role = field?.policy.owner ?? 'Env', schema = r[`${role}Columns`], row = r.Rows[0][role];
    let index = schema.findIndex(column => column.name.toLowerCase() === control.binding.toLowerCase());
    if (index === -1) {
      index = schema.length;
      schema.push({ name: control.binding, declaredType: 'TEXT' });
      row.cells.push(cell());
    }
    r.Bindings.push({ ControlID: control.controlId, Binding: control.binding,
      Table: r[`${role}Table`], Column: index, Implicit: false });
  }
  return r;
}
function set(r, column, value) {
  const binding = r.Bindings.find(b => b.Binding === column);
  r.Rows[0][binding.Table === r.EnvTable ? 'Env' : 'Admin'].cells[binding.Column] = structuredClone(value);
}
function fixture(form = 'FS882-8x6XL', overrides = {}, initial = original(form)) {
  let current = structuredClone(initial), last;
  const calls = { save: 0, restore: 0, refresh: 0, read: 0, cancel: 0 };
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    save: async request => {
      calls.save++; last = structuredClone(request);
      for (const edit of request.edits) set(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '9007199254740993' };
    },
    restore: async (_, action) => {
      calls.restore++; current = structuredClone(initial);
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => calls.refresh++,
    ...overrides,
  };
  return { session: new api.TwoPageParentSession(owner, form, port, () => {}), calls, last: () => last };
}
test('exact two-page variants preserve duplicate instances and separate extra-field scopes', async () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const r = original(form), f = fixture(form);
    assert.deepEqual(api.twoPageParentOriginalFromWire(r, owner, form), r);
    assert.equal(api.twoPageParentFields(form).length, form.endsWith('-CHARS') ? 8 : 9);
    assert.equal(r.Bindings.filter(b => b.Binding === 'PlotNumber').length, 3);
    assert.equal(r.Bindings.filter(b => b.Binding === 'SV_StandAgeEstMeas').length, form.endsWith('-CHARS') ? 1 : 2);
    assert.equal(await f.session.load(), true);
    assert.throws(() => editor.validateSIVIParentOriginal(r), /requires/);
    if (form.endsWith('-CHARS')) assert.throws(() => f.session.stage('BEC_Use', { kind: 'text', raw: 'new' }), /require/);
    assert.throws(() => f.session.stage('SV_StandHeight', { kind: 'text', raw: '3' }), /require/);
  }
  assert.throws(() => api.twoPageParentFields('FS882-8x6XL '), /Exact/);
  assert.throws(() => api.twoPageParentOriginalFromWire(original(), { ...owner, contextId: 'stale' }, 'FS882-8x6XL'), /another/);
});
test('two-page raw originals reject tampered schemas, bindings, row identities and source variants', () => {
  for (const kind of ['form', 'row', 'join', 'binding', 'implicit', 'duplicate-schema', 'shadow', 'owner']) {
    const r = original();
    switch (kind) {
      case 'form': r.Form = 'FS882-8x6XL-CHARS'; break;
      case 'row': r.Rows[0].Admin.rowId = '0901'; break;
      case 'join': r.Rows[0].Env.cells[0] = cell('text', 'P '); break;
      case 'binding': r.Bindings[0].ControlID += ':stale'; break;
      case 'implicit': r.Bindings[0].Implicit = true; break;
      case 'duplicate-schema': r.EnvColumns[1].name = 'plotnumber'; break;
      case 'shadow': r.EnvColumns[1].name = 'rowid'; break;
      case 'owner': {
        const b = r.Bindings.find(b => b.Binding === 'GIS_BGC');
        r.AdminColumns[b.Column].name = 'unbound';
        b.Table = r.EnvTable; b.Column = r.EnvColumns.length;
        r.EnvColumns.push({ name: 'GIS_BGC', declaredType: 'TEXT' }); r.Rows[0].Env.cells.push(cell());
        break;
      }
    }
    assert.throws(() => api.twoPageParentOriginalFromWire(r, owner, 'FS882-8x6XL'), undefined, kind);
  }
});
test('two-page draft boundaries cover every extra field, UTF-16 limits and signed Integer storage', async () => {
  for (const field of api.twoPageParentFields('FS882-8x6XL')) {
    const f = fixture(); await f.session.load();
    const valid = field.policy.kind === 'text' ? 'x'.repeat(field.policy.maximum)
      : field.policy.kind === 'integer' ? '-32768' : '-3.25';
    f.session.stage(field.column, { kind: 'text', raw: valid });
    assert.equal(f.session.closeState().canSave, true, field.column);
    const draft = f.session.view().drafts[field.column];
    assert.equal(draft.table, `Project_${field.policy.owner}`);
    assert.equal(draft.rowId, field.policy.owner === 'Admin' ? '901' : '1');
    const invalid = field.policy.kind === 'text' ? 'x'.repeat(field.policy.maximum + 1)
      : field.policy.kind === 'integer' ? '32768' : '3.402823466385289e38';
    f.session.stage(field.column, { kind: 'text', raw: invalid });
    assert.equal(f.session.closeState().blocked, true, field.column);
    await assert.rejects(f.session.save());
    assert.equal(f.calls.save, 0);
    assert.equal(f.session.view().drafts[field.column].input.raw, invalid);
    f.session.stage(field.column, { kind: 'text', raw: valid });
    assert.equal(f.session.view().error, null);
    assert.equal(await f.session.save(), true);
    const edit = f.last().edits[0];
    assert.equal(edit.value.storage, field.policy.kind === 'text' ? 'text' : field.policy.kind === 'integer' ? 'integer' : 'real');
    assert.equal(Object.keys(f.session.view().drafts).length, 0);
  }
  const f = fixture(); await f.session.load();
  for (const raw of ['\ud800', '\udfff', '\ud800x', '\udfff\ud800', '\u{1f600}'.repeat(26)]) {
    f.session.stage('SV_FullCruiseCard', { kind: 'text', raw });
    assert.equal(f.session.closeState().blocked, true);
  }
  f.session.stage('SV_FullCruiseCard', { kind: 'text', raw: '\u{1f600}'.repeat(25) });
  assert.equal(f.session.closeState().canSave, true);
});
test('two-page historical storage is omitted unchanged, never normalized into a new assignment', async () => {
  for (const [column, before] of [['GIS_BGC_VER', cell('integer', '40000')],
    ['PlotSize', cell('real', 1e100)], ['GIS_BGC', cell('text', '')],
    ['SiteUnitLongName', cell('text', 'x'.repeat(101))], ['ActiveLayerDepth', cell('blob', '00ff')]]) {
    const r = original(); set(r, column, before);
    const f = fixture('FS882-8x6XL', {}, r); await f.session.load();
    f.session.stage(column, { kind: 'original' });
    assert.equal(await f.session.save(), true);
    assert.equal(f.calls.save, 0);
    f.session.stage(column, { kind: 'clear' });
    if (before.storage === 'blob') {
      assert.equal(f.session.closeState().blocked, true);
      f.session.stage(column, { kind: 'original' });
    } else assert.equal(f.session.closeState().canSave, true);
  }
});
test('two-page invalid drafts survive view remounts and Undo clears the correct original identity', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('GIS_BGC_VER', { kind: 'text', raw: '1.5' });
  const mounted = f.session.view();
  mounted.drafts.GIS_BGC_VER.input.raw = 'mutated presentation';
  assert.equal(f.session.view().drafts.GIS_BGC_VER.input.raw, '1.5');
  assert.equal(f.session.closeState().unsaved, true);
  assert.equal(f.session.closeState().blocked, true);
  await assert.rejects(f.session.load(), /Save or Undo/);
  assert.equal(await f.session.undo(), true);
  assert.deepEqual(f.session.view().drafts, {});
  assert.equal(f.session.view().error, null);
});
test('two-page requests recheck draft ownership instead of trusting presentation state', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('GIS_BGC', { kind: 'text', raw: '  MiXeD  ' });
  for (const key of ['contextId', 'table', 'rowId', 'column']) {
    const view = f.session.view(); view.drafts.GIS_BGC[key] = 'stale';
    assert.throws(() => api.twoPageParentRequest(view.original, view.drafts, 'FS882-8x6XL'));
  }
  assert.equal(await f.session.save(), true);
  assert.equal(f.last().edits[0].value.text, '  MiXeD  ');
  assert.equal(await f.session.restore('prune'), true);
  assert.equal(f.session.view().historyId, null);
});
test('two-page ambiguous saves latch unknown authority and recovery never replays the write', async () => {
  let writes = 0;
  const f = fixture('FS882-8x6XL', { save: async () => { writes++; throw new Error('lost response'); } });
  await f.session.load();
  f.session.stage('PlotSize', { kind: 'text', raw: '3' });
  assert.equal(await f.session.save(), false);
  assert.equal(f.session.closeState().blocked, true);
  assert.equal(f.session.closeState().canSave, false);
  await assert.rejects(f.session.save());
  assert.equal(writes, 1);
  assert.equal(await f.session.undo(), true);
  assert.equal(writes, 1);
  assert.equal(f.session.closeState().blocked, false);
});

function panel(form, view, props = {}) {
  const component = serverComponent(readFileSync(require.resolve('./TwoPageParentFields.svelte'), 'utf8'), 'TwoPageParentFields.svelte', {
    './twoPageParentSession': api, './projectMetadataEditor': metadata,
  });
  return render(component, { props: { form, view, onstage() {}, onoperation() {}, oncancel() {}, ...props } }).body;
}
test('two-page panel preserves exact labels, source pages and one live control per extra field', async () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const f = fixture(form); await f.session.load();
    const html = panel(form, f.session.view(), { disabled: false, canSave: true });
    const fields = api.twoPageParentFields(form);
    assert.equal((html.match(/data-two-page-field=/g) || []).length, fields.length);
    assert.equal((html.match(/<input /g) || []).length, fields.length);
    assert.match(html, /aria-label="Site\/Veg"/);
    assert.match(html, /aria-label="Soil\/Terrain"/);
    assert.match(html, /grid-cols-1[^"]*sm:grid-cols-2[^"]*lg:grid-cols-3/);
    for (const field of fields) {
      assert.equal((html.match(new RegExp(`id="two-page-${field.column}"`, 'g')) || []).length, 1);
      assert.ok(html.includes(`for="two-page-${field.column}"`));
      assert.ok(html.includes(field.label));
      assert.ok(html.includes(`data-source-control="${field.controlId}"`));
    }
    assert.equal(html.includes('data-two-page-field="BEC_Use"'), !form.endsWith('-CHARS'));
  }
});
test('two-page presentation remount preserves raw invalid input, associated alert and disabled Save', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('GIS_BGC_VER', { kind: 'text', raw: '1.5' });
  for (let mount = 0; mount < 2; mount++) {
    const html = panel('FS882-8x6XL', f.session.view(), {
      disabled: false, canSave: f.session.closeState().canSave,
    });
    assert.match(html, /id="two-page-GIS_BGC_VER"[^>]*value="1\.5"[^>]*aria-invalid="true"/);
    assert.match(html, /aria-describedby="two-page-GIS_BGC_VER-error"/);
    assert.match(html, /id="two-page-GIS_BGC_VER-error" role="alert"/);
    assert.match(html, /<button[^>]*disabled[^>]*>Save additional fields<\/button>/);
  }
});
test('two-page panel defaults to disabled and keeps safety feedback above routine guidance', async () => {
  const f = fixture(); await f.session.load();
  const view = f.session.view();
  view.blocked = true; view.error = 'Independent write outcome unknown';
  const html = panel('FS882-8x6XL', view);
  assert.equal((html.match(/<input[^>]*disabled/g) || []).length, 9);
  assert.ok(html.indexOf('Independent write outcome unknown') < html.indexOf('desktop safety adaptations'));
  assert.match(html, /never replay Save/);
  assert.match(html, /<button[^>]*>Undo \/ reload<\/button>/);
});
test('two-page panel offers cancellation only for an active original read', () => {
  const view = { original: null, drafts: {}, busy: true, operation: 'read',
    blocked: false, error: null, historyId: null };
  assert.match(panel('FS882-8x6XL', view), /Cancel additional-field read/);
  assert.doesNotMatch(panel('FS882-8x6XL', { ...view, operation: 'save' }), /Cancel additional-field read/);
});
test('independent source review can load without enabling additional-field edits', async () => {
  const f = fixture(); await f.session.load();
  const html = panel('FS882-8x6XL', f.session.view(), { disabled: true, readDisabled: false });
  assert.match(html, /<button(?![^>]*disabled)[^>]*>Load additional fields<\/button>/);
  assert.match(html, /<button[^>]*disabled[^>]*>Save additional fields<\/button>/);
  assert.equal((html.match(/<input[^>]*disabled/g) || []).length, 9);
});
test('actual controller factories retain variant histories across remounts and refresh clean peer snapshots', async () => {
  const current = { 'FS882-8x6XL': original(), 'FS882-8x6XL-CHARS': original('FS882-8x6XL-CHARS') };
  const sessions = new Map(), calls = { read: [], save: 0, refresh: 0 };
  const values = {
    sessions, owner, TwoPageParentSession: api.TwoPageParentSession,
    reads: { track: value => value, cancelAll() {} }, onchange() {},
    oncommitted: async () => calls.refresh++,
    TwoPageParentExtraService: {
      GetOriginal: async (context, plot, form) => {
        assert.equal(context, owner.contextId); assert.equal(plot, owner.plot);
        calls.read.push(form); return structuredClone(current[form]);
      },
      Save: async (context, plot, form, request) => {
        assert.equal(context, owner.contextId); assert.equal(plot, owner.plot);
        calls.save++;
        for (const edit of request.edits) for (const [variant, r] of Object.entries(current)) {
          if (edit.column !== 'BEC_Use' || variant === 'FS882-8x6XL') set(r, edit.column, edit.value);
        }
        return { ChangedCells: request.edits.length, HistoryID: '77' };
      },
    },
  };
  const first = componentFunctions('TwoPageParentEditor.svelte', ['makeSession'], values);
  const normalSession = first.actions.makeSession('FS882-8x6XL');
  const charsSession = first.actions.makeSession('FS882-8x6XL-CHARS');
  await normalSession.load(); await charsSession.load();
  normalSession.stage('GIS_BGC_VER', { kind: 'text', raw: '7' });
  assert.equal(await normalSession.save(), true);
  assert.equal(normalSession.view().historyId, '77');
  assert.equal(api.twoPageParentCell(charsSession.view().original, 'GIS_BGC_VER').integer, '7');
  const remounted = componentFunctions('TwoPageParentEditor.svelte', ['makeSession'], values);
  assert.equal(remounted.actions.makeSession('FS882-8x6XL'), normalSession);
  assert.equal(remounted.actions.makeSession('FS882-8x6XL-CHARS'), charsSession);
  assert.equal(remounted.actions.makeSession('FS882-8x6XL').view().historyId, '77');
  assert.equal(calls.save, 1);
  assert.equal(calls.refresh, 1);
});
test('actual root guards delegate additional-field errors and refuse ordinary Save/Lock while review owns the plot', async () => {
  const invalid = { unsaved: true, busy: false, blocked: true, canSave: false, error: 'Integer invalid', saveReason: 'Correct Integer' };
  const root = componentFunctions('FS882Form.svelte', ['getCloseState', 'save', 'toggleLock', 'undo'], {
    siviParentSourceView: null, siviSourceAuthorityUnknown: false, twoPageOpen: true, pictureMetadataPending: false,
    twoPageEditor: { getCloseState: () => invalid, undo() { root.undone = true; } },
    busy: false, headerWorkflowBusy: true, error: null,
  });
  assert.equal(root.actions.getCloseState(), invalid);
  await root.actions.save();
  assert.match(root.error, /Wait for the current operation/);
  await root.actions.toggleLock();
  assert.match(root.error, /Wait for the current operation/);
  root.actions.undo();
  assert.equal(root.undone, true);
  root.twoPageEditor = undefined;
  assert.equal(root.actions.getCloseState().blocked, true);
  assert.equal(root.actions.getCloseState().busy, true);
});
test('additional-field review excludes ordinary parent drafts until its owner is explicitly closed', () => {
  const source = readFileSync(require.resolve('./FS882Form.svelte'), 'utf8');
  const expression = source.match(/const childUnsaved = \$derived\(([^;]+)\);/)[1];
  const clean = { nonParentChildUnsaved: false, siviParentWriteUnsaved: false,
    siviParentActionUnsaved: false, siviProjectAssignmentUnsaved: false,
    siviParentSharedUnsaved: false, twoPageOpen: false };
  assert.equal(runInNewContext(expression, clean), false);
  for (const owner of Object.keys(clean)) {
    assert.equal(runInNewContext(expression, { ...clean, [owner]: true }), true, owner);
  }
  assert.match(source, /const headerInputsDisabled = \$derived\([^;]*childUnsaved/);
  for (const component of ['HeaderEditor', 'ParentCodeFields', 'OrdinaryFields']) {
    assert.match(source, new RegExp(`<${component}[^>]*disabled=\\{headerInputsDisabled\\}`));
  }
});
