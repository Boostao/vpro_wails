const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { editor, transport, writeSession, metadata, cell } = require('./siviParentTestHelpers.cjs');
const modules = { siviParentEditor: editor, siviParentTransport: transport,
  siviParentWriteSession: writeSession, projectMetadataEditor: metadata };
function load(name) {
  if (modules[name]) return modules[name];
  const source = readFileSync(path.join(__dirname, name + '.ts'), 'utf8');
  const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS,
    target: ts.ScriptTarget.ES2022 } }).outputText;
  const dependencies = {};
  for (const [, specifier] of js.matchAll(/require\("([^"]+)"\)/g)) {
    dependencies[specifier] = specifier.endsWith('.json') ? require(specifier)
      : load(specifier.replace('./', ''));
  }
  return modules[name] = loadTypeScript(name + '.ts', dependencies);
}
const api = load('twoPageEntrySession');
const sourceAPI = load('twoPageParentSession');
const commonAPI = load('twoPageParentCommonSession');
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
const forms = ['FS882-8x6XL', 'FS882-8x6XL-CHARS'];
const text = raw => ({ kind: 'text', raw });
function fixtureSnapshot(form = forms[0]) {
  const controls = sourceAPI.twoPageParentSource(form).fields.filter(field => field.binding);
  const known = [...sourceAPI.twoPageParentFields(form), ...commonAPI.twoPageParentCommonFields(form)];
  const original = { ContextID: owner.contextId, Project: owner.project, Plot: owner.plot, Form: form,
    Query: 'USysEnv', Membership: 'literal-binary-inner-pairs', EnvTable: 'Project_Env', AdminTable: 'Project_Admin',
    EnvColumns: [{ name: 'PlotNumber', declaredType: 'TEXT' }], AdminColumns: [{ name: 'Plot', declaredType: 'TEXT' }],
    Rows: [{ Env: { rowId: '1', cells: [cell('text', 'P')] }, Admin: { rowId: '901', cells: [cell('text', 'P')] } }],
    Bindings: [] };
  const policies = [];
  for (const control of controls) {
    const p = known.find(field => field.column === control.binding)?.policy;
    const role = p?.owner ?? 'Env', columns = original[role + 'Columns'], row = original.Rows[0][role];
    let index = columns.findIndex(column => column.name === control.binding);
    if (index === -1) {
      index = columns.length; columns.push({ name: control.binding, declaredType: 'TEXT' }); row.cells.push(cell());
    }
    original.Bindings.push({ ControlID: control.controlId, Binding: control.binding,
      Table: original[role + 'Table'], Column: index, Implicit: false });
    if (control.binding !== 'PlotNumber' && !policies.some(policy => policy.column === control.binding)) {
      const special = { LocationAccuracy: 'integer', Elevation: 'integer', Latitude: 'latitude', Longitude: 'longitude',
        SpeciesListComplete: 'boolean', Zone: 'categorical', SubZone: 'categorical', FieldNumber: 'text' };
      policies.push({ column: control.binding, owner: role, kind: special[control.binding] ?? p?.kind ?? 'text',
        maximum: control.binding === 'Zone' ? 4 : control.binding === 'SubZone' ? 8 : p?.maximum ?? 50 });
    }
  }
  const Fields = controls.filter(control => control.type === 'ComboBox').map(control => ({
    column: control.binding, listName: control.binding, required: Boolean(control.properties.LimitToList),
    source: 'fixture physical source', available: true, diagnostic: '',
    definitions: { columns: [{ name: 'Item', declaredType: 'TEXT' }, { name: 'Description', declaredType: 'TEXT' }],
      rows: [{ rowId: '1', cells: [cell('text', 'A'), cell()] },
        { rowId: '2', cells: [cell('text', 'A'), cell('text', '')] },
        { rowId: '3', cells: [cell(), cell()] }, { rowId: '4', cells: [cell('text', ''), cell('text', '')] }] },
    choices: [{ rowId: '1', code: 'A', description: null, selectable: true, diagnostic: '' },
      { rowId: '2', code: 'A', description: '', selectable: true, diagnostic: '' },
      { rowId: '3', code: null, description: null, selectable: false, diagnostic: 'NULL' },
      { rowId: '4', code: '', description: '', selectable: false, diagnostic: 'empty' }],
  }));
  return { Original: original, Policies: policies, Fields, Zone: cell(), SubZone: cell(),
    ProjectSource: 1, WorkingSource: 3, MasterEditingAvailable: true,
    ProjectAssignmentAvailable: true, ProjectAssignmentDiagnostic: 'verified fixture',
    ProjectChoices: { ContextID: owner.contextId, Project: owner.project, SourceOption: 1, Source: 'Env',
      Alias: 'project', Table: 'Project_Metadata', Choices: {
        columns: [{ name: 'ProjectID', declaredType: 'TEXT' }, { name: 'ProjectTitle', declaredType: 'TEXT' }],
        rows: [{ rowId: '10', cells: [cell('text', 'A'), cell()] },
          { rowId: '11', cells: [cell('text', 'A'), cell('text', '')] },
          { rowId: '12', cells: [cell(), cell()] }] } } };
}
function set(snapshot, column, value) {
  const original = snapshot.Original, binding = original.Bindings.find(binding => binding.Binding === column);
  original.Rows[0][binding.Table === original.EnvTable ? 'Env' : 'Admin'].cells[binding.Column] = structuredClone(value);
  if (column === 'Zone' || column === 'SubZone') snapshot[column] = structuredClone(value);
}
function fixture({ form = forms[0], snapshot = fixtureSnapshot(form), overrides = {} } = {}) {
  let current = structuredClone(snapshot), last;
  const calls = { save: 0, read: 0, refs: 0, restore: 0, refresh: 0, cancel: 0 };
  const port = { read: async () => { calls.read++; return structuredClone(current); },
    readReferences: async filters => { calls.refs++; return { ...structuredClone(current),
      Zone: structuredClone(filters.zone), SubZone: structuredClone(filters.subZone) }; },
    cancelRead: () => calls.cancel++, refreshParent: async () => calls.refresh++,
    save: async request => {
      calls.save++; last = structuredClone(request);
      assertProjectAssignmentProof(request);
      for (const edit of request.edits) set(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '9007199254740993' };
    },
    restore: async (_, action) => {
      calls.restore++; current = structuredClone(snapshot);
      return { cancelled: false, restoredRows: last.edits.length,
        prunedAuditRows: action === 'prune' ? last.edits.length : 0, cleanedVegRows: 0 };
    }, ...overrides };
  return { session: new api.TwoPageEntrySession(owner, form, port, () => {}), calls, last: () => last,
    current: () => current, port };
}
function assertProjectAssignmentProof(request) {
  const edits = request.edits.filter(edit => edit.column === 'ProjectID'), selection = request.projectSelection;
  assert.equal(edits.length, selection ? 1 : 0, 'backend requires exactly one scalar ProjectID assignment per physical proof');
  if (!selection) return;
  assert.deepEqual(structuredClone(edits[0]), {
    contextId: selection.contextId, table: selection.table, rowId: selection.rowId, column: 'ProjectID',
    expected: structuredClone(selection.expected), value: structuredClone(selection.metadataOriginal.cells[0]),
  });
}
const tick = () => new Promise(resolve => setImmediate(resolve));
const deferred = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject }; };

test('both complete form cohorts load one owned pair, 117/119 policies and 56/55 typed references', async () => {
  for (const form of forms) {
    const f = fixture({ form }); assert.equal(await f.session.load(), true);
    const view = f.session.view();
    assert.equal(view.snapshot.Policies.length, form.endsWith('-CHARS') ? 119 : 117);
    assert.equal(view.references.length, form.endsWith('-CHARS') ? 55 : 56);
    assert.equal(f.calls.refs, 0);
    assert.equal(view.references[0].choices[0].description, null);
    assert.equal(view.references[0].choices[1].description, '');
    view.snapshot.Original.ContextID = 'tampered';
    assert.equal(f.session.view().original.ContextID, owner.contextId);
    assert.throws(() => f.session.stage('PlotNumber', text('new')), /owned/);
  }
});
test('panel-facing view and cell reads retain stable names; physical selection drafts use public input intents', async () => {
  const f = fixture(); await f.session.load();
  const view = f.session.view();
  assert.equal(view.referencesBusy, false); assert.equal(view.referenceError, null);
  assert.equal(view.projectRowId, null);
  assert.deepEqual(structuredClone(view.advisories), {});
  assert.ok(view.snapshot.Fields); assert.ok(view.snapshot.Policies); assert.ok(view.snapshot.ProjectChoices);
  const value = api.twoPageEntryCell(view.original, 'PlotNumber');
  assert.deepEqual(value, cell('text', 'P')); value.text = 'mutated';
  assert.deepEqual(api.twoPageEntryCell(f.session.view().original, 'PlotNumber'), cell('text', 'P'));
  assert.throws(() => api.twoPageEntryCell(view.original, 'not-bound'), /source-binding/);
  f.session.selectProject('11');
  assert.deepEqual(f.session.view().drafts.ProjectID.input, text('A'));
  assert.equal(f.session.view().drafts.ProjectID.selection.metadataOriginal.rowId, '11');
  assert.equal(f.session.view().projectRowId, '11');
  f.session.selectProject('10');
  assert.equal(f.session.view().projectRowId, '10');
  assert.deepEqual(f.session.view().drafts.ProjectID.value, cell('text', 'A'));
  f.session.selectProject(null);
  assert.equal(f.session.view().projectRowId, null);
  assert.deepEqual(f.session.view().drafts.ProjectID.input, { kind: 'original' });
  assert.equal(f.session.closeState().unsaved, false);
});
function nativeZoneValues(snapshot) {
  const zone = snapshot.Fields.find(field => field.column === 'Zone');
  Object.assign(zone, {
    source: 'verified BEC catalogue; Zone value identities and SubZone source-ordinal identities',
    definitions: { columns: [], rows: [] },
    choices: [
      { rowId: '', code: 'AT', description: 'Alpine Tundra', selectable: true, diagnostic: '' },
      { rowId: '', code: 'BAFA', description: 'Boreal Altai Fescue Alpine', selectable: true, diagnostic: '' },
      { rowId: '', code: 'AT', description: null, selectable: true, diagnostic: '' },
      { rowId: '', code: 'AT', description: '', selectable: true, diagnostic: '' },
      { rowId: '', code: null, description: null, selectable: false, diagnostic: 'Item is NULL' },
      { rowId: '', code: '', description: null, selectable: false, diagnostic: 'empty literal' },
    ],
  });
  return zone;
}
test('native-shaped Zone DISTINCT values have no physical row IDs; empty available SiteSeries stays distinct from unavailable', async () => {
  const snapshot = fixtureSnapshot(), zone = nativeZoneValues(snapshot);
  const series = snapshot.Fields.find(field => field.column === 'SiteSeries');
  Object.assign(series, {
    source: 'verified BEC catalogue; current Zone/SubZone pair; source-ordinal identities; no stale RowSource reuse',
    definitions: { columns: [{ name: 'SiteSeries', declaredType: 'TEXT' }, { name: 'Description', declaredType: 'TEXT' }],
      rows: [] }, choices: [],
  });
  const f = fixture({ snapshot });
  assert.equal(await f.session.load(), true);
  assert.deepEqual(f.session.view().snapshot.Fields.find(field => field.column === 'Zone'), zone);
  assert.deepEqual(f.session.view().snapshot.Fields.find(field => field.column === 'SiteSeries'), series);
  f.session.stage('Zone', text('AT')); await tick();
  assert.equal(f.session.closeState().canSave, true);
  assert.equal(f.session.view().drafts.Zone.value.text, 'AT');
  // An optional local receipt verifies the untouched real Wails transport without invoking Wails.
  if (process.env.TWO_PAGE_ENTRY_WIRE_RECEIPT) {
    const captured = JSON.parse(readFileSync(process.env.TWO_PAGE_ENTRY_WIRE_RECEIPT, 'utf8'));
    const owned = { contextId: captured.Original.ContextID, project: captured.Original.Project, plot: captured.Original.Plot };
    const session = new api.TwoPageEntrySession(owned, captured.Original.Form,
      { read: async () => captured, cancelRead() {}, save() { throw new Error('read-only receipt'); },
        restore() { throw new Error('read-only receipt'); }, refreshParent: async () => {},
        readReferences() { throw new Error('read-only receipt'); } }, () => {});
    assert.equal(await session.load(), true, session.view().error);
    assert.deepEqual(session.view().snapshot.Fields, captured.Fields);
    assert.equal(session.view().snapshot.Fields.length, 56);
  }
});
test('Zone value identity exception rejects duplicate tuples, omitted nulls and wrong/mixed provenance; physical references remain strict', async () => {
  for (const mutate of [
    (s, zone) => zone.choices.push(structuredClone(zone.choices[0])),
    (s, zone) => delete zone.choices[0].description,
    (s, zone) => delete zone.choices[0].code,
    (s, zone) => zone.source = 'unreviewed DISTINCT source',
    (s, zone) => zone.listName = 'other',
    (s, zone) => zone.choices[0].rowId = '1',
    (s, zone) => zone.definitions.columns.push({ name: 'Zone', declaredType: 'TEXT' }),
    s => { const field = s.Fields.find(field => field.column === 'SubZone'); field.choices[0].rowId = ''; },
  ]) {
    const snapshot = fixtureSnapshot(), zone = nativeZoneValues(snapshot); mutate(snapshot, zone);
    const f = fixture({ snapshot }); assert.equal(await f.session.load(), false, mutate.toString());
    assert.equal(f.session.view().original, null);
  }
});
test('snapshot rejects ownership, form, physical cohort, duplicate policies/references and malformed typed definitions', async () => {
  const mutations = [
    s => s.Original.ContextID = 'other', s => s.Original.Form = forms[1],
    s => s.Original.Rows.push(structuredClone(s.Original.Rows[0])),
    s => s.Policies.pop(), s => s.Policies[1] = structuredClone(s.Policies[0]),
    s => s.Policies[0].kind = 'unsupported', s => s.Policies[0].maximum = -1,
    s => s.Policies[0].owner = s.Policies[0].owner === 'Env' ? 'Admin' : 'Env',
    s => s.Fields.pop(), s => s.Fields[1] = structuredClone(s.Fields[0]),
    s => s.Fields[0].required = !s.Fields[0].required,
    s => s.Fields[0].choices[0].description = 1,
    s => s.Fields[0].definitions.rows[0].cells[0] = { storage: 'text', text: 'partial' },
    s => s.Fields[0].definitions.rows[1].rowId = '1',
    s => s.Fields[0].definitions.columns[1].name = 'item',
    s => s.Fields[0].choices[0].rowId = '999',
    s => s.ProjectSource = 0, s => s.WorkingSource = 4,
    s => s.ProjectChoices.SourceOption = 2, s => s.Zone = cell('text', 'stale'),
    s => s.MasterEditingAvailable = null, s => s.ProjectAssignmentDiagnostic = null,
  ];
  for (const mutate of mutations) {
    const snapshot = fixtureSnapshot(); mutate(snapshot);
    const f = fixture({ snapshot });
    assert.equal(await f.session.load(), false, mutate.toString());
    assert.equal(f.session.view().original, null, mutate.toString());
  }
});
test('one atomic Save spans XL/common/extra; preserves unchanged historical invalid storage', async () => {
  const snapshot = fixtureSnapshot(); set(snapshot, 'FieldNumber', cell('text', 'x'.repeat(200)));
  const f = fixture({ snapshot }); await f.session.load();
  f.session.stage('FieldNumber', text('x'.repeat(200)));
  f.session.stage('Location', text('literal'));
  f.session.stage('SV_StandHeight', text('3.25'));
  f.session.stage('ActiveLayerDepth', text('-2.5'));
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 1);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['Location', 'SV_StandHeight', 'ActiveLayerDepth']);
  assert.equal(f.last().workingSource, 3);
  assert.equal(f.last().original.Rows.length, 1);
  assert.equal(f.session.closeState().unsaved, false);
  assert.equal(await f.session.restore('prune'), true);
  assert.equal(f.calls.restore, 1);
});
test('actual shared parser covers integer, Single, Boolean, text, UTF-16 and option domains; errors persist in cache/remount', async () => {
  const f = fixture(); await f.session.load();
  for (const [column, input, storage, value] of [
    ['Elevation', text('-32768'), 'integer', '-32768'],
    ['SV_StandHeight', text('-3.25'), 'real', -3.25],
    ['SV_FloodPlain', { kind: 'boolean', value: true }, 'integer', '-1'],
    ['SV_StandAgeEstMeas', { kind: 'option', option: 2 }, 'text', '2'],
    ['SiteUnitLongName', text(' a  '), 'text', ' a  '],
  ]) {
    f.session.stage(column, input);
    assert.deepEqual(f.session.view().drafts[column].value, cell(storage, value));
  }
  for (const [column, input] of [['Elevation', text('32768')], ['SV_StandHeight', text('NaN')],
    ['SV_FloodPlain', text('true')], ['SV_StandAgeEstMeas', { kind: 'option', option: 3 }],
    ['SiteUnitLongName', text('\ud800')]]) {
    f.session.stage(column, input);
    assert.equal(f.session.closeState().blocked, true);
    await assert.rejects(f.session.save());
    f.session.stage(column, { kind: 'original' });
  }
  f.session.stage('SiteUnitLongName', text('😀'.repeat(51)));
  const cache = new Map([[owner.contextId, new Map([[forms[0], f.session]])]]);
  const remounted = cache.get(owner.contextId).get(forms[0]);
  assert.match(remounted.view().drafts.SiteUnitLongName.error, /UTF-16/);
  assert.equal(remounted.closeState().canSave, false);
  assert.equal(remounted.closeState().unsaved, true);
  await assert.rejects(remounted.load());
  assert.equal(await remounted.undo(), true);
  assert.deepEqual(remounted.view().drafts, {});
});
test('required references enforce exact selectable nonempty membership; NULL clearing needs no acknowledgement', async () => {
  const f = fixture(); await f.session.load();
  const column = f.session.view().references.find(field => field.required && !['ProjectID', 'Zone', 'SubZone'].includes(field.column)).column;
  for (const raw of ['a', 'A ', '', 'missing']) {
    f.session.stage(column, text(raw)); assert.equal(f.session.closeState().canSave, false, raw);
    assert.throws(() => f.session.acknowledge(column), /optional/);
  }
  f.session.stage(column, text('A')); assert.equal(f.session.closeState().canSave, true);
  f.session.stage(column, { kind: 'clear' }); assert.equal(f.session.closeState().canSave, true);
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 0);
});
test('optional literal requires a fresh full original/reference acknowledgement, invalidated by correction or refresh', async () => {
  const f = fixture(); await f.session.load();
  const column = f.session.view().references.find(field => !field.required && !['ProjectID', 'Zone', 'SubZone'].includes(field.column)).column;
  f.session.stage(column, text('unlisted'));
  assert.equal(f.session.closeState().canSave, false);
  assert.ok(f.session.view().advisories[column]);
  f.session.acknowledge(column); assert.equal(f.session.closeState().canSave, true);
  f.session.stage(column, text('other')); assert.equal(f.session.closeState().canSave, false);
  f.session.acknowledge(column);
  assert.equal(await f.session.loadReferences(), true);
  assert.equal(f.session.closeState().canSave, false);
  f.session.acknowledge(column);
  assert.equal(await f.session.save(), true);
  const ack = f.last().acknowledgements[0], edit = f.last().edits[0];
  for (const key of ['contextId', 'table', 'rowId', 'column', 'expected', 'value']) assert.deepEqual(ack[key], edit[key]);
  assert.equal(ack.form, forms[0]); assert.equal(ack.project, owner.project); assert.equal(ack.plot, owner.plot);
  assert.deepEqual(ack.reference, fixtureSnapshot().Fields.find(field => field.column === column));
});
test('historically invalid BEC filters and unavailable dependent refs do not prevent unrelated complete-entry Save', async () => {
  const snapshot = fixtureSnapshot(); set(snapshot, 'Zone', cell('blob', 'ff'));
  for (const field of snapshot.Fields.filter(field => ['SubZone', 'SiteSeries'].includes(field.column))) {
    field.available = false; field.diagnostic = 'historical Zone cannot filter dependent source';
    field.definitions = { columns: [], rows: [] }; field.choices = [];
  }
  const f = fixture({ snapshot }); assert.equal(await f.session.load(), true);
  f.session.stage('Location', text('unrelated'));
  assert.equal(await f.session.save(), true);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['Location']);
});
test('newly listed PlotType omits obsolete acknowledgement and saves its companion atomically', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('PlotType', text('Custom'));
  f.session.acknowledge('PlotType');
  const acknowledged = f.session.view().drafts.PlotType.acknowledgement;
  const field = f.current().Fields.find(field => field.column === 'PlotType');
  assert.equal(field.required, false);
  field.definitions.rows.push({ rowId: '5', cells: [cell('text', 'Custom'), cell('text', 'new definition')] });
  field.choices.push({ rowId: '5', code: 'Custom', description: 'new definition', selectable: true, diagnostic: '' });
  assert.equal(await f.session.loadReferences(), true);
  assert.deepEqual(f.session.view().drafts.PlotType.acknowledgement, acknowledged);
  f.session.stage('Location', text('companion'));
  assert.equal(f.session.closeState().canSave, true);
  const save = f.port.save;
  f.port.save = request => {
    assert.deepEqual(structuredClone(request.acknowledgements), []);
    return save(request);
  };
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 1);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['PlotType', 'Location']);
  assert.deepEqual(f.last().acknowledgements, []);
  assert.equal(f.session.closeState().blocked, false);
  assert.equal(f.session.closeState().unsaved, false);
});
test('still-unlisted PlotType retains companion drafts and blocks stale reference proofs until reacknowledged', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('PlotType', text('Custom')); f.session.acknowledge('PlotType');
  f.session.stage('Location', text('companion'));
  const field = f.current().Fields.find(field => field.column === 'PlotType');
  field.definitions.rows[0].cells[1] = cell('text', 'updated description');
  field.choices[0].description = 'updated description';
  assert.equal(await f.session.loadReferences(), true);
  assert.equal(f.session.closeState().canSave, false);
  await assert.rejects(f.session.save(), /fresh reference acknowledgement/);
  assert.equal(f.calls.save, 0);
  assert.equal(f.session.view().drafts.PlotType.input.raw, 'Custom');
  assert.equal(f.session.view().drafts.Location.input.raw, 'companion');
  f.session.acknowledge('PlotType');
  assert.equal(await f.session.save(), true);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['PlotType', 'Location']);
  assert.equal(f.last().acknowledgements.length, 1);
  assert.deepEqual(f.last().acknowledgements[0].reference, field);
});
test('effective BEC pair refresh preserves drafts/original, blocks Save, excludes late generations and never clears dependent fields', async () => {
  const reads = [], snapshot = fixtureSnapshot(), f = fixture({ snapshot, overrides: {
    readReferences: filters => { const d = deferred(); reads.push({ ...d, filters: structuredClone(filters) }); return d.promise; },
  } });
  await f.session.load();
  const before = f.session.view().original;
  f.session.stage('Location', text('draft'));
  f.session.stage('Zone', text('A'));
  assert.equal(reads.length, 1); assert.equal(f.session.closeState().busy, true);
  await assert.rejects(f.session.save());
  f.session.stage('SubZone', text('A')); assert.equal(reads.length, 2);
  reads[1].resolve({ ...structuredClone(snapshot), Zone: cell('text', 'A'), SubZone: cell('text', 'A') }); await tick();
  assert.equal(f.session.view().referencesBusy, false);
  reads[0].resolve({ ...structuredClone(snapshot), Zone: cell('text', 'A'), SubZone: cell() }); await tick();
  assert.deepEqual(f.session.view().snapshot.SubZone, cell('text', 'A'));
  assert.deepEqual(f.session.view().original, before);
  assert.equal(f.session.view().drafts.Location.input.raw, 'draft');
  assert.equal(f.session.view().drafts.SiteSeries, undefined);
  assert.equal(f.session.closeState().canSave, true);
  assert.equal(f.session.view().error, null);
  assert.equal(f.session.closeState().error, null);
});
test('reference receipts deny stale original/filter/source/preference/project proof without changing drafts', async () => {
  for (const mutate of [
    s => set(s, 'Location', cell('text', 'external')),
    s => s.Zone = cell('text', 'wrong'), s => s.SubZone = cell('text', 'wrong'),
    s => s.WorkingSource = 2,
    s => { s.ProjectSource = 2; Object.assign(s.ProjectChoices, { SourceOption: 2, Source: 'Master', Alias: 'VMetaData', Table: 'ProjectMetadata' }); },
    s => s.ProjectChoices.Choices.rows[0].cells[1] = cell('text', 'changed'),
  ]) {
    const snapshot = fixtureSnapshot(), f = fixture({ snapshot, overrides: {
      readReferences: async filters => { const receipt = { ...structuredClone(snapshot), Zone: filters.zone, SubZone: filters.subZone };
        mutate(receipt); return receipt; },
    } });
    await f.session.load(); f.session.stage('Location', text('draft'));
    assert.equal(await f.session.loadReferences(), false, mutate.toString());
    assert.equal(f.session.closeState().canSave, false);
    assert.equal(f.session.view().drafts.Location.input.raw, 'draft');
    assert.equal(f.calls.save, 0);
  }
});
test('PHYSICAL Project selection preserves duplicate code rows and NULL/empty titles; rejects free text, creation and NULL assignment', async () => {
  for (const rowId of ['10', '11']) {
    const f = fixture(); await f.session.load(); f.session.selectProject(rowId);
    f.session.stage('Location', text('atomic companion'));
    assert.equal(await f.session.save(), true);
    assert.equal(f.calls.save, 1);
    assert.equal(f.last().projectSelection.metadataOriginal.rowId, rowId);
    assert.deepEqual(f.last().projectSelection.metadataOriginal.cells[1], rowId === '10' ? cell() : cell('text', ''));
    assert.equal(f.last().projectSelection.controlId, `form:${forms[0]}/ProjectID`);
    assert.deepEqual(f.last().edits.map(edit => edit.column), ['ProjectID', 'Location']);
  }
  const snapshot = fixtureSnapshot(); set(snapshot, 'ProjectID', cell('text', 'old'));
  const f = fixture({ snapshot }); await f.session.load();
  for (const input of [text('A'), { kind: 'clear' }]) {
    f.session.stage('ProjectID', input); assert.equal(f.session.closeState().blocked, true);
    await assert.rejects(f.session.save());
  }
  for (const rowId of ['999', '12']) {
    f.session.selectProject(rowId); assert.equal(f.session.closeState().blocked, true);
    await assert.rejects(f.session.save());
  }
  f.session.selectProject(null); assert.equal(f.session.closeState().unsaved, false);
  snapshot.ProjectAssignmentAvailable = false;
  const disabled = fixture({ snapshot }); await disabled.session.load(); disabled.session.selectProject('10');
  assert.equal(disabled.session.closeState().canSave, false);
});
test('strict Master ProjectID request carries scalar mutation and independent physical proof exactly once', async () => {
  const synthetic = fixtureSnapshot();
  set(synthetic, 'ProjectID', cell('text', 'hju'));
  synthetic.ProjectSource = 2;
  Object.assign(synthetic.ProjectChoices, {
    SourceOption: 2, Source: 'Master', Alias: 'VMetaData', Table: 'ProjectMetadata',
    Choices: { columns: synthetic.ProjectChoices.Choices.columns,
      rows: [{ rowId: '1', cells: [cell('text', 'BEC'), cell()] }] },
  });
  const snapshots = [synthetic];
  if (process.env.TWO_PAGE_ENTRY_PROJECT_WIRE_RECEIPT) {
    snapshots.push(JSON.parse(readFileSync(process.env.TWO_PAGE_ENTRY_PROJECT_WIRE_RECEIPT, 'utf8')));
  }
  for (const snapshot of snapshots) {
    const owned = { contextId: snapshot.Original.ContextID, project: snapshot.Original.Project, plot: snapshot.Original.Plot };
    let current = structuredClone(snapshot), last = null, saves = 0;
    const choice = snapshot.ProjectChoices.Choices.rows.find(row => row.rowId === '1');
    assert.deepEqual(choice.cells[0], cell('text', 'BEC'));
    const binding = snapshot.Original.Bindings.find(binding => binding.Binding === 'ProjectID');
    assert.equal(binding.Table, snapshot.Original.EnvTable);
    assert.equal(snapshot.Original.Rows[0].Env.rowId, '1');
    const expected = api.twoPageEntryCell(snapshot.Original, 'ProjectID');
    assert.deepEqual(expected, cell('text', 'hju'));
    const realm = snapshot.Fields.find(field => field.column === 'RealmClass')
      .choices.find(choice => choice.selectable && choice.code);
    const session = new api.TwoPageEntrySession(owned, snapshot.Original.Form, {
      read: async () => structuredClone(current), cancelRead() {}, refreshParent: async () => {},
      readReferences() { throw new Error('No BEC filter changes in this request'); },
      restore() { throw new Error('No native restoration'); },
      save: async request => {
        assertProjectAssignmentProof(request);
        assert.equal(request.projectSource, 2);
        assert.equal(request.projectSelection.sourceOption, 2);
        assert.equal(request.projectSelection.metadataAlias, 'VMetaData');
        assert.equal(request.projectSelection.metadataTable, 'ProjectMetadata');
        assert.equal(request.projectSelection.controlId, `form:${snapshot.Original.Form}/ProjectID`);
        assert.deepEqual(structuredClone(request.projectSelection.metadataOriginal), choice);
        assert.deepEqual(structuredClone(request.projectSelection.metadataColumns), snapshot.ProjectChoices.Choices.columns);
        assert.deepEqual(structuredClone(request.edits.map(edit => edit.column)),
          ['ProjectID', 'FieldNumber', 'SV_CanopyComposition', 'ProvinceStateTerritory', 'RealmClass']);
        const project = request.edits.find(edit => edit.column === 'ProjectID');
        assert.deepEqual(structuredClone(project.expected), expected);
        assert.deepEqual(structuredClone(project.value), choice.cells[0]);
        assert.equal(project.table, snapshot.Original.EnvTable);
        assert.equal(project.rowId, '1');
        assert.equal(request.acknowledgements.length, 0);
        saves++; last = structuredClone(request);
        for (const edit of request.edits) set(current, edit.column, edit.value);
        return { ChangedCells: 5, HistoryID: '77' };
      },
    }, () => {});
    assert.equal(await session.load(), true, session.view().error);
    session.selectProject('1');
    session.stage('FieldNumber', text('strict proof field'));
    session.stage('SV_CanopyComposition', text('strict proof canopy'));
    session.stage('ProvinceStateTerritory', text('strict proof province'));
    session.stage('RealmClass', text(realm.code));
    assert.equal(await session.save(), true, session.view().error);
    assert.equal(saves, 1); assert.equal(last.edits.length, 5);
    assert.equal(session.view().historyId, '77');
    assert.equal(session.closeState().blocked, false);
  }
});
test('same-code Project physical row selection omits both no-op scalar and unused proof, preserving companion writes', async () => {
  const snapshot = fixtureSnapshot(); set(snapshot, 'ProjectID', cell('text', 'A'));
  const f = fixture({ snapshot }); await f.session.load();
  f.session.selectProject('11');
  assert.equal(f.session.view().projectRowId, '11');
  assert.equal(f.session.closeState().unsaved, false);
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 0);
  f.session.selectProject('11');
  f.session.stage('Location', text('companion'));
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 1);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['Location']);
  assert.equal(f.last().projectSelection, null);
  assert.equal(f.last().acknowledgements.length, 0);
});
test('MasterEditingAvailable guards BECSiteUnit stage and refreshed requests; unrelated writes stay available', async () => {
  const snapshot = fixtureSnapshot(); snapshot.MasterEditingAvailable = false;
  const f = fixture({ snapshot }); await f.session.load();
  assert.throws(() => f.session.stage('BECSiteUnit', text('A')), /MasterEditingAvailable/);
  f.session.stage('Location', text('unrelated')); assert.equal(await f.session.save(), true);
  const enabled = fixture({ overrides: { readReferences: async filters => ({ ...fixtureSnapshot(),
    Zone: filters.zone, SubZone: filters.subZone, MasterEditingAvailable: false }) } });
  await enabled.session.load(); enabled.session.stage('BECSiteUnit', text('A'));
  await enabled.session.loadReferences(); assert.equal(enabled.session.closeState().canSave, false);
});
test('cancelled/disposed initial reads cannot initialize originals or snapshot; cancelled reference receipts cannot replace live state', async () => {
  for (const action of ['cancel', 'dispose']) {
    const d = deferred(), f = fixture({ overrides: { read: () => d.promise } });
    const result = f.session.load(); f.session[action](); d.resolve(fixtureSnapshot());
    assert.equal(await result, false); assert.equal(f.session.view().original, null); assert.equal(f.session.view().snapshot, null);
  }
  const d = deferred(), f = fixture({ overrides: { readReferences: () => d.promise } });
  await f.session.load(); f.session.stage('Location', text('draft'));
  const result = f.session.loadReferences(); f.session.cancel();
  d.resolve(fixtureSnapshot()); assert.equal(await result, false);
  assert.equal(f.session.closeState().canSave, false);
  assert.match(f.session.view().referenceError, /cancelled/);
  assert.equal(f.session.view().drafts.Location.input.raw, 'draft');
});
test('uncertain Save receipts and rejected transport retire drafts, block close/replay and recover only through Undo/reload', async () => {
  for (const save of [async () => { throw new Error('lost receipt'); },
    async () => ({ ChangedCells: 999, HistoryID: '1' }), async () => ({ ChangedCells: 1, HistoryID: '01' })]) {
    const f = fixture({ overrides: { save } }); await f.session.load(); f.session.stage('Location', text('new'));
    assert.equal(await f.session.save(), false);
    assert.equal(f.session.closeState().blocked, true); assert.equal(f.session.closeState().unsaved, true);
    assert.deepEqual(f.session.view().drafts, {});
    await assert.rejects(f.session.save()); assert.match(f.session.view().error, /do not replay/);
    assert.equal(await f.session.undo(), true); assert.equal(f.session.closeState().blocked, false);
  }
});
test('verified Save/Restore share conservative lifecycle and reject cancellation/disposal during commit', async () => {
  const d = deferred(), f = fixture({ overrides: { save: () => d.promise } });
  await f.session.load(); f.session.stage('Location', text('new'));
  const saving = f.session.save();
  assert.throws(() => f.session.cancel(), /cannot/); assert.throws(() => f.session.dispose(), /cannot/);
  d.resolve({ ChangedCells: 1, HistoryID: '5' }); assert.equal(await saving, true);
  const restore = deferred(); f.port.restore = () => restore.promise;
  const restoring = f.session.restore('retain'); assert.throws(() => f.session.cancel(), /cannot/);
  restore.reject(new Error('lost restore receipt')); assert.equal(await restoring, false);
  assert.equal(f.session.closeState().blocked, true); assert.equal(f.session.view().historyId, null);
  await assert.rejects(f.session.restore('retain'));
  assert.equal(await f.session.undo(), true);
});
