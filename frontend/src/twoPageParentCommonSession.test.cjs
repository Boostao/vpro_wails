const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const { editor, writeSession, transport, source, cell, metadata } = require('./siviParentTestHelpers.cjs');
const normal = require('../../resources/fs882-two-page-layout.json');
const chars = require('../../resources/fs882-two-page-chars-layout.json');
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
const extra = loadTypeScript('twoPageParentSession.ts', {
  '../../resources/fs882-two-page-layout.json': normal,
  '../../resources/fs882-two-page-chars-layout.json': chars,
  './siviParentEditor': editor, './siviParentTransport': transport, './siviParentWriteSession': writeSession,
  './siviParentSharedSession': shared, './ordinaryEditor': ordinary, './projectMetadataEditor': metadata,
});
const api = loadTypeScript('twoPageParentCommonSession.ts', {
  './siviParentEditor': editor, './twoPageParentSession': extra, './siviParentTransport': transport,
  './siviParentWriteSession': writeSession, './siviParentSharedSession': shared, './ordinaryEditor': ordinary,
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const xl = require('../../resources/fs882-xl-layout.json');
const paper = loadTypeScript('paperLayout.ts', {
  '../../resources/fs882-xl-layout.json': xl,
  '../../resources/fs882-extended-shrub-layout.json': require('../../resources/fs882-extended-shrub-layout.json'),
});
const projectionDependencies = {
  '../../resources/fs882-xl-layout.json': xl, './twoPageParentSession': extra,
  './twoPageParentCommonSession': api, './paperLayout': paper, './projectMetadataEditor': metadata,
};
const projection = loadTypeScript('twoPageEntryProjection.ts', projectionDependencies);
const presentation = loadTypeScript('formPresentation.ts', { './paperLayout': paper });
const coordinates = loadTypeScript('coordinateEditor.ts');
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
function original(form = 'FS882-8x6XL') {
  const r = { ContextID: owner.contextId, Project: owner.project, Plot: owner.plot, Form: form,
    Query: 'USysEnv', Membership: 'literal-binary-inner-pairs', EnvTable: 'Project_Env', AdminTable: 'Project_Admin',
    EnvColumns: [{ name: 'PlotNumber', declaredType: 'TEXT' }], AdminColumns: [{ name: 'Plot', declaredType: 'TEXT' }],
    Rows: [{ Env: { rowId: '1', cells: [cell('text', 'P')] }, Admin: { rowId: '901', cells: [cell('text', 'P')] } }],
    Bindings: [] };
  const fields = [...api.twoPageParentCommonFields(form), ...extra.twoPageParentFields(form)];
  for (const control of extra.twoPageParentSource(form).fields.filter(field => field.binding)) {
    const policy = fields.find(field => field.column === control.binding)?.policy;
    const role = policy?.owner ?? 'Env', schema = r[`${role}Columns`], row = r.Rows[0][role];
    let index = schema.findIndex(column => column.name.toLowerCase() === control.binding.toLowerCase());
    if (index === -1) {
      index = schema.length;
      schema.push({ name: control.binding, declaredType: 'TEXT' });
      row.cells.push(cell());
    }
    r.Bindings.push({ ControlID: control.controlId, Binding: control.binding, Table: r[`${role}Table`],
      Column: index, Implicit: false });
  }
  return r;
}
function set(r, column, value) {
  const b = r.Bindings.find(binding => binding.Binding === column);
  r.Rows[0][b.Table === r.EnvTable ? 'Env' : 'Admin'].cells[b.Column] = structuredClone(value);
}
function fixture(form = 'FS882-8x6XL', overrides = {}, initial = original(form)) {
  let current = structuredClone(initial), last;
  const calls = { save: 0, refresh: 0, cancel: 0 };
  const port = {
    read: async () => structuredClone(current),
    cancelRead: () => calls.cancel++,
    save: async request => {
      calls.save++; last = structuredClone(request);
      for (const edit of request.edits) set(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '9007199254740993' };
    },
    restore: async () => {
      current = structuredClone(initial);
      return { cancelled: false, restoredRows: 1, prunedAuditRows: 1, cleanedVegRows: 0 };
    },
    refreshParent: async () => calls.refresh++,
    ...overrides,
  };
  return { session: new api.TwoPageParentCommonSession(owner, form, port, () => {}), calls, last: () => last };
}
test('common source scope preserves every instance with one live owner per logical field', async () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const fields = api.twoPageParentCommonFields(form), f = fixture(form);
    assert.equal(fields.length, form.endsWith('-CHARS') ? 14 : 13);
    assert.equal(new Set(fields.map(field => field.column)).size, fields.length);
    const age = fields.find(field => field.column === 'SV_StandAgeEstMeas');
    assert.equal(age.sourceInstances.length, form.endsWith('-CHARS') ? 1 : 2);
    assert.deepEqual(Array.from(age.options, option => option.value), [1, 2]);
    assert.deepEqual(Array.from(age.options, option => option.label), ['Est.', form.endsWith('-CHARS') ? 'Meas.' : 'Measure']);
    assert.equal(fields.find(field => field.column === 'PlotType').label,
      extra.twoPageParentSource(form).fields.find(field => field.controlName === 'Label261').caption);
    assert.equal(await f.session.load(), true);
    assert.throws(() => f.session.stage('SV_FullCruiseCard', { kind: 'text', raw: 'foreign' }), /require/);
    assert.throws(() => f.session.stage('SnowCoverregime', { kind: 'text', raw: 'S' }), /require/);
    if (!form.endsWith('-CHARS')) assert.throws(() => f.session.stage('SV_StandHeightEstMeas', { kind: 'option', option: 1 }), /require/);
  }
});

function headerComponents() {
  const read = name => readFileSync(`${__dirname}\\${name}.svelte`, 'utf8')
    .replace(/import\.meta\.env\.VITE_[A-Z_]+/g, "'true'");
  const bec = loadTypeScript('becEditor.ts');
  const reference = loadTypeScript('referenceCodeEditor.ts', { './becEditor': bec, './qualityEditor': quality });
  const workingUnit = loadTypeScript('workingUnitEditor.ts', { './becEditor': bec });
  const dependencies = {
    './paperLayout': paper, './formPresentation': presentation, './qualityEditor': quality,
    './becEditor': bec, './ordinaryEditor': ordinary,
    './workingUnitEditor': workingUnit,
    './masterBECEditor': loadTypeScript('masterBECEditor.ts', { './referenceCodeEditor': reference, './workingUnitEditor': workingUnit }),
    './coordinateEditor': coordinates,
    './substrateEditor': loadTypeScript('substrateEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
    './siteCodeEditor': loadTypeScript('siteCodeEditor.ts', { './referenceCodeEditor': reference }),
    './regionCodeEditor': loadTypeScript('regionCodeEditor.ts', { './referenceCodeEditor': reference }),
    '../bindings/github.com/boostao/vpro-wails': {
      CoordinateMode: { CoordinateModeDD: 'dd', CoordinateModeDM: 'dm', CoordinateModeDMS: 'dms' },
    },
  };
  for (const name of ['WorkingUnitFields', 'QualityFields', 'SiteCodeFields', 'RegionCodeFields',
    'SubstrateFields', 'BECFields', 'CoordinateFields']) {
    dependencies[`./${name}.svelte`] = { default: serverComponent(read(name), `${name}.svelte`, dependencies) };
  }
  const Header = serverComponent(read('HeaderEditor'), 'HeaderEditor.svelte', dependencies);
  const SourcePage = serverComponent(read('SourcePage'), 'SourcePage.svelte', dependencies);
  const Layout = serverComponent(read('TwoPageEntryLayout'), 'TwoPageEntryLayout.svelte', {
    './SourcePage.svelte': { default: SourcePage }, './twoPageEntryProjection': projection,
    './coordinateEditor': coordinates,
  });
  const model = readFileSync(`${__dirname}\\..\\bindings\\github.com\\boostao\\vpro-wails\\models.ts`, 'utf8')
    .match(/export interface FS882Header \{([\s\S]*?)\n\}/)[1];
  const draft = Object.fromEntries([...model.matchAll(/"([^"]+)":/g)].map(match => [match[1], null]));
  draft.plotNumber = 'P'; draft.locked = false; draft.fieldNo = '  literal field  ';
  return { Header, Layout, draft };
}

test('actual header families reuse exact two-page source inputs without another label or identity owner', () => {
  const { Header, Layout, draft } = headerComponents();
  const fixture = `<script>
    import Header from './Header.svelte'; import Layout from './Layout.svelte';
    export let form; export let page; export let draft; export let values; export let mode;
    const capabilities = Object.fromEntries(Object.keys(draft).map(key => [key, true]));
  </script>
  <Header {draft} original={draft} {capabilities} disabled={false} existing={true} lists={{}}
    workingUnitSession={{mode:null}} onchange={()=>{}} onerror={()=>{}} onvalidation={()=>{}} sourcePage={page} coordinateMode={mode}>
    {#snippet children(editor,coordinateMode)}
      <Layout {form} {values} {coordinateMode} editors={[{...editor,scope:'xl'}]}>
        {#snippet identity(field)}<input readonly data-header={field.column} value="P"/>{/snippet}
        {#snippet embedded(child,control)}<span data-linked-child={child.form}></span>{/snippet}
      </Layout>
    {/snippet}
  </Header>`;
  const Component = serverComponent(fixture, 'ComposedTwoPageHeaderFixture.svelte', {
    './Header.svelte': { default: Header }, './Layout.svelte': { default: Layout },
  });
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) for (const mode of ['dd', 'dm', 'dms']) {
    coordinates.rememberCoordinateSession(draft, { mode, ready: true, busy: false, axes: Object.fromEntries(
      ['latitude', 'longitude'].map(axis => [axis, { value: null, buffers: { degrees: '12', minutes: '3', seconds: '4', negative: false },
        error: null, ready: true, touched: false, pending: false }])) });
    const projected = projection.twoPageEntryProjection(form, mode);
    const html = render(Component, { props: { form, page: projected.pages[0], draft,
      mode, values: projection.twoPageEntryValues(original(form), form) } }).body;
    for (const key of ['fieldNo', 'zone', 'userSiteUnit', 'sitePlotQuality', 'substrateWater', 'siteDisturbance1', 'ecosection']) {
      assert.equal((html.match(new RegExp(`id="header-${key}"`, 'g')) || []).length, 1, key);
      assert.equal((html.match(new RegExp(`for="header-${key}"`, 'g')) || []).length, 1, key);
    }
    assert.match(html, /value="  literal field  "/);
    assert.equal((html.match(/data-header="PlotNumber"/g) || []).length, 1);
    assert.doesNotMatch(html, /id="header-plotNumber"|<label[^>]*>(?:(?!<\/label>)[\s\S])*<label/);
    assert.equal((html.match(/data-linked-child=/g) || []).length, 5);
    assert.match(html, /Pictures unavailable/);
    assert.match(html, /data-source-control="btnCopyToUserSU"[^>]*disabled|disabled[^>]*data-source-control="btnCopyToUserSU"/);
    const visible = mode === 'dd' ? ['Latitude', 'Longitude'] : mode === 'dm' ?
      ['LatD2', 'LatMD', 'LonD2', 'LonMD'] : ['LatD', 'LatM', 'LatS', 'LonD', 'LonM', 'LonS'];
    for (const name of [...visible, 'Check369', 'Check371', 'Check373']) {
      assert.equal((html.match(new RegExp(`id="header-coordinate-${name}"`, 'g')) || []).length, 1, `${mode}/${name}`);
      assert.equal((html.match(new RegExp(`for="header-coordinate-${name}"`, 'g')) || []).length, 1, `${mode}/${name}`);
    }
    for (const name of Object.keys(coordinates.coordinateControls).filter(name => !visible.includes(name))) {
      assert.doesNotMatch(html, new RegExp(`id="header-coordinate-${name}"`));
    }
  }
});

test('header composition refuses foreign controls and resolves flags from its independent source owner', () => {
  const { Header, draft } = headerComponents();
  const page = projection.twoPageEntryProjection('FS882-8x6XL').pages[0];
  const field = page.controls.find(control => control.column === 'FieldNumber');
  const fixture = `<script>import Header from './Header.svelte'; export let page; export let draft; export let control;</script>
    <Header {draft} original={draft} capabilities={{fieldNo:true}} disabled={false} existing={true} lists={{}}
      workingUnitSession={{mode:null}} onchange={()=>{}} onerror={()=>{}} onvalidation={()=>{}} sourcePage={page}>
      {#snippet children(editor,mode)}{@render editor.input(control,'')}{/snippet}
    </Header>`;
  const Component = serverComponent(fixture, 'HeaderOwnerFixture.svelte', { './Header.svelte': { default: Header } });
  for (const control of [{ ...field, controlId: 'foreign' }, { ...field, column: 'PlotNumber' },
    { ...field, column: 'SV_CanopyComposition' }]) {
    assert.throws(() => render(Component, { props: { page, draft, control } }).body, /no exact source field owner/);
  }
  const locked = { ...page, controls: page.controls.map(control => control === field ? { ...control, locked: true } : control) };
  const html = render(Component, { props: { page: locked, draft, control: { ...field, locked: false } } }).body;
  assert.match(html, /id="header-fieldNo"[^>]*disabled/);
});

test('standalone header still owns the XL identity and existing family controls by default', () => {
  const { Header, draft } = headerComponents();
  const html = render(Header, { props: { draft, original: draft, capabilities: { fieldNo: true, plotNumber: true },
    disabled: false, existing: true, lists: {}, workingUnitSession: { mode: null },
    onchange() {}, onerror() {}, onvalidation() {} } }).body;
  assert.match(html, /data-source-page="Site"/);
  assert.equal((html.match(/id="header-plotNumber"/g) || []).length, 1);
  assert.match(html, /id="header-plotNumber"[^>]*readonly/);
  assert.equal((html.match(/id="header-fieldNo"/g) || []).length, 1);
  assert.match(html, /value="  literal field  "/);
  assert.match(html, /Coordinate safety controls/);
});
test('every common domain blocks invalid assignments and clears errors only on valid correction', async () => {
  for (const field of api.twoPageParentCommonFields('FS882-8x6XL-CHARS')) {
    const f = fixture('FS882-8x6XL-CHARS'); await f.session.load();
    const valid = field.policy.kind === 'boolean' ? { kind: 'boolean', value: true }
      : field.policy.kind === 'option' ? { kind: 'option', option: 2 }
      : { kind: 'text', raw: field.policy.kind === 'single' ? '-3.25' : 'x'.repeat(field.policy.maximum) };
    const invalid = field.policy.kind === 'boolean' ? { kind: 'text', raw: '-1' }
      : field.policy.kind === 'option' ? { kind: 'option', option: 3 }
      : { kind: 'text', raw: field.policy.kind === 'single' ? '3.402823466385289e38' : 'x'.repeat(field.policy.maximum + 1) };
    f.session.stage(field.column, invalid);
    assert.equal(f.session.closeState().blocked, true, field.column);
    await assert.rejects(f.session.save()); assert.equal(f.calls.save, 0);
    const retained = f.session.view(); retained.drafts[field.column].error = null;
    assert.equal(f.session.closeState().blocked, true, 'views must remain detached');
    f.session.stage(field.column, valid);
    assert.equal(f.session.view().error, null);
    const draft = f.session.view().drafts[field.column];
    assert.equal(draft.table, `Project_${field.policy.owner}`);
    assert.equal(draft.rowId, field.policy.owner === 'Admin' ? '901' : '1');
    assert.equal(await f.session.save(), true);
    assert.equal(f.calls.refresh, 1);
  }
});
test('bound PlotType preserves literal text and rejects the SIVI action mapping', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('PlotType', { kind: 'option', option: 1 });
  assert.equal(f.session.closeState().blocked, true);
  f.session.stage('PlotType', { kind: 'text', raw: '  Custom  ' });
  assert.equal(await f.session.save(), true);
  assert.equal(f.last().edits[0].value.text, '  Custom  ');
  for (const raw of ['', '\ud800', '\udfff', 'a\0b', '😀'.repeat(6)]) {
    f.session.stage('PlotType', { kind: 'text', raw });
    assert.equal(f.session.closeState().blocked, true);
  }
  f.session.stage('PlotType', { kind: 'clear' });
  assert.equal(f.session.view().drafts.PlotType.value.storage, 'null');
  assert.equal(await f.session.save(), true);
});
test('categorical NULL and empty text remain explicit and distinct', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('SV_RootZoneTexture', { kind: 'empty' });
  assert.equal(await f.session.save(), true);
  assert.equal(f.last().edits[0].value.text, '');
  f.session.stage('SV_RootZoneTexture', { kind: 'clear' });
  assert.equal(await f.session.save(), true);
  assert.equal(f.last().edits[0].value.storage, 'null');
});
test('common BOOLEAN uses Access true=-1 and never phantom option audits', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('SV_FloodPlain', { kind: 'boolean', value: true });
  f.session.stage('SV_StandAgeEstMeas', { kind: 'option', option: 1 });
  assert.equal(await f.session.save(), true);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['SV_FloodPlain', 'SV_StandAgeEstMeas']);
  assert.equal(f.last().edits[0].value.integer, '-1');
  f.session.stage('SV_StandAgeEstMeas', { kind: 'option', option: 1 });
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 1, 'unchanged source option must not write');
});
test('unchanged historical invalid common storage is omitted rather than repaired', async () => {
  const r = original();
  set(r, 'PlotType', cell('text', 'historical invalid plot type'));
  set(r, 'SV_FloodPlain', cell('integer', '2'));
  set(r, 'SV_PolygonNumber', cell('text', ''));
  set(r, 'SV_RootZoneTexture', cell('text', ''));
  const f = fixture('FS882-8x6XL', {}, r); await f.session.load();
  f.session.stage('PlotType', { kind: 'text', raw: 'historical invalid plot type' });
  f.session.stage('SV_FloodPlain', { kind: 'original' });
  f.session.stage('SV_PolygonNumber', { kind: 'text', raw: '' });
  f.session.stage('SV_RootZoneTexture', { kind: 'text', raw: '' });
  f.session.stage('SV_StandHeight', { kind: 'text', raw: '2.5' });
  assert.equal(await f.session.save(), true);
  assert.deepEqual(f.last().edits.map(edit => edit.column), ['SV_StandHeight']);
});
test('common Save transport is revalidated against physical identity and original values', async () => {
  const f = fixture(); await f.session.load();
  f.session.stage('SV_StandHeight', { kind: 'text', raw: '2' });
  const view = f.session.view();
  for (const key of ['contextId', 'table', 'rowId', 'column']) {
    const bad = structuredClone(view.drafts); bad.SV_StandHeight[key] = 'foreign';
    assert.throws(() => api.twoPageParentCommonRequest(view.original, bad, 'FS882-8x6XL'), /another|exclude/);
  }
  const bad = structuredClone(view.original);
  const b = bad.Bindings.find(binding => binding.Binding === 'PlotType');
  b.Table = bad.EnvTable;
  assert.throws(() => api.validateTwoPageParentCommonOriginal(bad, 'FS882-8x6XL'), /ownership/);
});
test('common cancelled reads reject late originals and permit explicit retry', async () => {
  let deliver;
  const f = fixture('FS882-8x6XL', { read: () => new Promise(resolve => { deliver = resolve; }) });
  const pending = f.session.load(); f.session.cancel(); deliver(original());
  assert.equal(await pending, false);
  assert.equal(f.session.view().original, null);
  assert.equal(f.calls.cancel, 1);
  const retry = f.session.load(); deliver(original()); assert.equal(await retry, true);
});
test('common lost Save response requires Undo and never replays the write', async () => {
  const f = fixture('FS882-8x6XL', { save: async () => { f.calls.save++; throw new Error('lost response'); } });
  await f.session.load(); f.session.stage('PlotType', { kind: 'text', raw: 'Custom' });
  assert.equal(await f.session.save(), false);
  assert.equal(f.session.closeState().blocked, true);
  await assert.rejects(f.session.load(), /Save or Undo/);
  assert.equal(f.calls.save, 1);
  assert.equal(await f.session.undo(), true);
  assert.equal(f.session.closeState().blocked, false);
  assert.equal(f.calls.save, 1);
});
test('actual common component renders thirteen/fourteen labelled controls with source option captions', async () => {
  const component = serverComponent(readFileSync(require.resolve('./TwoPageParentCommonFields.svelte'), 'utf8'),
    'TwoPageParentCommonFields.svelte', {
      './projectMetadataEditor': metadata, './twoPageParentCommonSession': api,
    });
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const f = fixture(form); await f.session.load();
    const html = render(component, { props: { form, view: f.session.view(), choices: [
      { listName: 'PlotType', item: 'Custom', itemDescription: 'Literal choice' },
    ], choicesBusy: false, choicesError: null, disabled: false, readDisabled: false, canSave: true,
    onstage() {}, onoperation() {}, oncancel() {}, onchoices() {} } }).body;
    for (const field of api.twoPageParentCommonFields(form)) {
      const id = `two-page-common-${field.column}`;
      assert.equal((html.match(new RegExp(`id="${id}"`, 'g')) ?? []).length, 1, field.column);
      assert.ok(html.includes(`for="${id}"`), field.column);
      assert.ok(html.includes(field.label), field.column);
    }
    assert.ok(html.includes('grid-cols-1'));
    assert.ok(html.includes('sm:grid-cols-2'));
    assert.ok(html.includes('lg:grid-cols-3'));
    assert.ok(html.includes('two-page-common-grid'));
    assert.ok(!html.includes('class="grid grid-cols-1'), 'generic host grid must not override responsive columns');
    assert.ok(html.includes('value="Custom"'));
    assert.ok(html.includes('value="true"'));
    assert.ok(html.includes('value="false"'));
    assert.ok(html.includes('Est.'));
    assert.ok(html.includes(form.endsWith('-CHARS') ? 'Meas.' : 'Measure'));
  }
});
test('common field grids use scoped display and explicit related-field row starts', () => {
  const source = readFileSync(require.resolve('./TwoPageParentCommonFields.svelte'), 'utf8');
  assert.match(source, /\.two-page-common-grid\s*\{\s*display:\s*grid;/);
  assert.match(source, /class:group-start=\{field.column === 'SV_StandAgeEstMeas' \|\| field.column === 'StrataCoverTotal'\}/);
  assert.match(source, /@media \(min-width: 640px\)[\s\S]*\.group-start \{ grid-column-start: 1; \}/);
  assert.match(source, /@media \(min-width: 1024px\)[\s\S]*\.height-estimation \{ grid-column-start: 3; \}/);
});
test('actual common controller caches variant owners and refreshes clean peers without replay', async () => {
  const sessions = new Map(), current = {
    'FS882-8x6XL': original(), 'FS882-8x6XL-CHARS': original('FS882-8x6XL-CHARS'),
  };
  let writes = 0, refreshes = 0;
  const values = { owner, sessions, onchange() {}, TwoPageParentCommonSession: api.TwoPageParentCommonSession,
    reads: { track: value => value, cancelAll() {} }, oncommitted: async () => refreshes++,
    TwoPageParentCommonService: {
      GetOriginal: async (context, plot, form) => {
        assert.equal(context, owner.contextId); assert.equal(plot, owner.plot);
        return structuredClone(current[form]);
      },
      Save: async (_, __, form, request) => {
        writes++;
        for (const edit of request.edits) for (const r of Object.values(current)) set(r, edit.column, edit.value);
        return { ChangedCells: request.edits.length, HistoryID: '51' };
      },
    },
  };
  const mounted = componentFunctions('TwoPageParentCommonEditor.svelte', ['makeSession'], values);
  const session = mounted.actions.makeSession('FS882-8x6XL');
  const peer = mounted.actions.makeSession('FS882-8x6XL-CHARS');
  await session.load(); await peer.load();
  session.stage('SV_StandAgeEstMeas', { kind: 'option', option: 2 });
  assert.equal(await session.save(), true);
  assert.equal(api.twoPageParentCommonCell(peer.view().original, 'SV_StandAgeEstMeas').text, '2');
  const remounted = componentFunctions('TwoPageParentCommonEditor.svelte', ['makeSession'], values);
  assert.equal(remounted.actions.makeSession('FS882-8x6XL'), session);
  assert.equal(session.view().historyId, '51');
  assert.equal(writes, 1); assert.equal(refreshes, 1);
});
test('actual root refuses common review scope changes over an existing additional-field owner', () => {
  const root = componentFunctions('FS882Form.svelte', ['openTwoPageCommonReview'], {
    twoPageOpen: true, twoPageScope: 'extra', twoPageCommonReviewEnabled: true,
    openTwoPageReview() { throw new Error('must not replace active review'); }, error: null,
  });
  root.actions.openTwoPageCommonReview();
  assert.equal(root.twoPageScope, 'extra');
  assert.match(root.error, /close the current/);
  root.twoPageOpen = false; root.twoPageCommonReviewEnabled = false;
  root.actions.openTwoPageCommonReview();
  assert.match(root.error, /independently disabled/);
});
test('actual root refreshes opposite-scope caches and retains drafts when a peer is not clean', async () => {
  let loads = 0, parentRefresh = 0;
  const peer = {
    view: () => ({ original: original(), error: null }),
    closeState: () => ({ unsaved: false, busy: false }),
    load: async () => { loads++; return true; },
  };
  const key = JSON.stringify(owner);
  const root = componentFunctions('FS882Form.svelte', ['refreshTwoPagePeers'], {
    contextId: owner.contextId, twoPageProject: owner.project, draft: { plotNumber: owner.plot },
    twoPageSessions: new Map(), twoPageCommonSessions: new Map([[key, new Map([['FS882-8x6XL', peer]])]]),
    twoPageEntrySessions: new Map(),
    refreshSIVIParent: async () => parentRefresh++,
    siviParentWriteSession: null, siviParentActionSession: null, siviProjectAssignmentSession: null, siviParentSharedSession: null,
  });
  await root.actions.refreshTwoPagePeers('extra');
  assert.equal(loads, 1); assert.equal(parentRefresh, 1);
  peer.closeState = () => ({ unsaved: true, busy: false });
  await assert.rejects(root.actions.refreshTwoPagePeers('extra'), /could not be refreshed/);
  assert.equal(loads, 1, 'opposite-scope draft must not be overwritten');
});
test('actual PlotType loader rejects unavailable, foreign and late reference responses explicitly', async () => {
  let deliver;
  const controller = componentFunctions('TwoPageParentCommonEditor.svelte', ['loadChoices'], {
    reviewEnabled: true, choicesRequest: 0, choices: null, choicesBusy: false, choicesError: null,
    reads: { track: value => value }, ReferenceService: { GetListItems: () => new Promise(resolve => { deliver = resolve; }) },
  });
  for (const response of [null, [{ listName: 'Other', item: 'Other', itemDescription: '' }]]) {
    const pending = controller.actions.loadChoices(); deliver(response); await pending;
    assert.equal(controller.choices, null);
    assert.match(controller.choicesError, /retry explicitly/);
  }
  const stale = controller.actions.loadChoices(), deliverStale = deliver;
  const fresh = controller.actions.loadChoices();
  const current = [{ listName: 'PlotType', item: '  Custom  ', itemDescription: '' }];
  deliver(current); await fresh;
  deliverStale(null); await stale;
  assert.equal(controller.choices, current);
  assert.equal(controller.choicesError, null);
  assert.equal(controller.choicesBusy, false);
});
test('complete entry projection accounts for every source binding and exact scope without enabling pictures', () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const view = projection.twoPageEntryProjection(form);
    assert.equal(view.fields.length, form.endsWith('-CHARS') ? 120 : 118);
    assert.deepEqual(['xl', 'common', 'extra'].map(scope => view.fields.filter(field => field.scope === scope).length),
      form.endsWith('-CHARS') ? [98, 14, 8] : [96, 13, 9]);
    assert.deepEqual(Array.from(view.header, field => field.column), ['PlotNumber']);
    assert.equal(view.header[0].sourceInstances.length, 3);
    assert.deepEqual(Array.from(view.pages, page => page.name), ['Site/Veg', 'Soil/Terrain']);
    assert.equal(view.children.length, 6);
    assert.equal(view.children[0].form, form.endsWith('-CHARS') ? 'SubVegAXL' : 'SubVegAXL_BC');
    assert.equal(view.children.find(child => child.form === 'frmVPicsXL').availability, 'unavailable');
    for (const page of view.pages) {
      const bound = page.controls.filter(control => control.column).map(control => control.column);
      assert.equal(new Set(bound).size, bound.length, 'one logical field owner per page');
      assert.ok(!bound.includes('PlotNumber'), 'header retains the sole PlotNumber owner');
    }
    const site = view.pages[0];
    assert.equal(site.controls.filter(control => control.column === 'SV_StandAgeEstMeas').length, 1);
    assert.equal(site.controls.find(control => control.column === 'PlotType').caption,
      api.twoPageParentCommonFields(form).find(field => field.column === 'PlotType').label);
    assert.equal(view.fields.some(field => field.column === 'EnteredBy'), form.endsWith('-CHARS'));
    assert.equal(view.fields.some(field => field.column === 'UpdatedFromCards'), form.endsWith('-CHARS'));
  }
});
test('entry projection rejects altered binding, header, child and page closures rather than inferring owners', () => {
  for (const kind of ['binding', 'column', 'page', 'header', 'child-form', 'child-link', 'child-page']) {
    const source = structuredClone(normal.forms[0]);
    if (kind === 'binding') source.fields.find(field => field.binding === 'PlotType').binding = 'unknown';
    if (kind === 'column') source.fields.find(field => field.binding === 'PlotType').column = 'StandAge';
    if (kind === 'page') source.pages[0].controlId = 'form:FS882-8x6XL/foreign';
    if (kind === 'header') for (const field of source.fields) if (field.binding === 'PlotNumber') field.pageId = 'form:FS882-8x6XL/Site/Veg';
    if (kind === 'child-form') source.embedded[0].form = 'SubVegAXL';
    if (kind === 'child-link') source.embedded[0].childFields = ['PlotNumber '];
    if (kind === 'child-page') source.controls.find(control => control.controlId === source.embedded[0].controlId).pageId = 'unknown';
    const altered = loadTypeScript('twoPageEntryProjection.ts', {
      ...projectionDependencies, './twoPageParentSession': { ...extra, twoPageParentSource: () => source },
    });
    assert.throws(() => altered.twoPageEntryProjection('FS882-8x6XL'), undefined, kind);
  }
});
test('entry projection detaches source evidence and never replaces the accepted XL geometry', () => {
  const before = JSON.stringify(normal), xlBefore = JSON.stringify(paper.paperPage('Site'));
  const view = projection.twoPageEntryProjection('FS882-8x6XL');
  view.children[0].masterFields[0] = 'tampered';
  view.pages[0].controls[0].properties = {};
  view.fields[0].sourceInstances.push('foreign');
  assert.equal(JSON.stringify(normal), before);
  assert.equal(JSON.stringify(paper.paperPage('Site')), xlBefore);
  assert.ok(!projection.twoPageEntryProjection('FS882-8x6XL').fields[0].sourceInstances.includes('foreign'));
});
test('complete source pages group every visible control once and bound options exclude their duplicate source buttons', () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    for (const mode of ['dd', 'dm', 'dms']) {
      const view = projection.twoPageEntryProjection(form, mode);
      const source = extra.twoPageParentSource(form);
      for (const page of view.pages) {
        const groups = presentation.presentationGroups(page.name, page.controls);
        const members = groups.flatMap(group => Array.from(group.controls));
        const expected = page.controls.filter(control => !['Label', 'Rectangle', 'Line'].includes(control.type) &&
          (control.type !== 'OptionGroup' || control.column));
        assert.deepEqual(Array.from(members, control => control.controlId).sort(),
          Array.from(expected, control => control.controlId).sort());
        assert.equal(new Set(members.map(control => control.controlId)).size, members.length);
        assert.equal(groups.some(group => group.title === 'Additional source fields and actions'), false,
          `${form}/${page.name}/${mode} requires explicit semantic grouping: ${
            groups.filter(group => group.title === 'Additional source fields and actions')
              .flatMap(group => group.controls.map(control => control.controlName)).join(', ')}`);
      }
      for (const field of source.fields.filter(field => field.type === 'OptionGroup' && field.binding)) {
        const descendants = source.controls.filter(control => {
          let parent = source.controls.find(candidate => candidate.controlId === control.parentId);
          while (parent) {
            if (parent.controlId === field.controlId) return true;
            parent = source.controls.find(candidate => candidate.controlId === parent.parentId);
          }
          return false;
        });
        assert.ok(descendants.some(control => ['CheckBox', 'OptionButton'].includes(control.type)));
        assert.ok(descendants.every(control => !view.pages.some(page =>
          page.controls.some(candidate => candidate.controlId === control.controlId))));
      }
    }
  }
});
test('complete entry layout composes exact source pages, disjoint scoped editors and linked children with no picture dispatch', () => {
  const SourcePage = serverComponent(readFileSync(`${__dirname}\\SourcePage.svelte`, 'utf8'), 'SourcePage.svelte', {
    './paperLayout': { ...paper, paperPage: () => { throw new Error('Complete entry must not select XL pages'); } },
    './formPresentation': presentation,
  });
  const Layout = serverComponent(readFileSync(`${__dirname}\\TwoPageEntryLayout.svelte`, 'utf8'), 'TwoPageEntryLayout.svelte', {
    './SourcePage.svelte': { default: SourcePage }, './twoPageEntryProjection': projection,
    './coordinateEditor': coordinates,
  });
  const fixture = `<script>
    import Layout from './Layout.svelte';
    export let form; export let columns; export let duplicate = false; export let wrongScope = false;
    export let controls = []; export let controlScope = 'xl'; export let duplicateControls = false;
  </script>
  {#snippet input(control, position)}<input data-live-column={control.column} data-live-control={control.controlId} style={position}/>{/snippet}
  {#snippet identity(field)}<input data-header-column={field.column} readonly value="P"/>{/snippet}
  {#snippet embedded(child, control)}
    <span data-linked-child={child.form} data-master={child.masterFields[0]} data-child={child.childFields[0]}
      data-child-control={control.controlName}></span>
  {/snippet}
  <Layout {form} {identity} {embedded} editors={[
    {scope:wrongScope ? 'xl' : 'common',columns:columns.common,input},
    {scope:'extra',columns:columns.extra,input},
    ...(duplicate ? [{scope:'common',columns:columns.common,input}] : []),
    {scope:controlScope,columns:[],controls,input},
    ...(duplicateControls ? [{scope:'xl',columns:[],controls,input}] : [])
  ]}/>`;
  const Component = serverComponent(fixture, 'TwoPageEntryLayoutFixture.svelte', {
    './Layout.svelte': { default: Layout },
  });
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const columns = {
      common: Array.from(api.twoPageParentCommonFields(form), field => field.column),
      extra: Array.from(extra.twoPageParentFields(form), field => field.column),
    };
    const html = render(Component, { props: { form, columns } }).body;
    const live = Array.from(html.matchAll(/data-live-column="([^"]+)"/g), match => match[1]);
    assert.equal(live.length, columns.common.length + columns.extra.length);
    assert.equal(new Set(live).size, live.length);
    assert.equal((html.match(/data-header-column="PlotNumber"/g) || []).length, 1);
    assert.doesNotMatch(html, /data-column="PlotNumber"|data-linked-child="frmVPicsXL"/);
    assert.equal((html.match(/data-linked-child=/g) || []).length, 5);
    assert.equal((html.match(/data-master="PlotNumber" data-child="PlotNumber"/g) || []).length, 5);
    assert.match(html, /aria-label="Pictures unavailable"/);
    assert.match(html, /data-source-page="Site\/Veg"/);
    assert.match(html, /data-source-page="Soil\/Terrain"/);
    assert.throws(() => render(Component, { props: { form, columns, duplicate: true } }).body, /Multiple two-page editors/);
    assert.throws(() => render(Component, { props: { form, columns, wrongScope: true } }).body, /no matching xl page editor owner/);
    const page = projection.twoPageEntryProjection(form).pages[0];
    const coordinate = page.controls.find(control => control.controlName === 'Check369').controlId;
    const props = { form, columns, controls: [coordinate] };
    assert.match(render(Component, { props }).body, new RegExp(`data-live-control="${coordinate}"`));
    assert.throws(() => render(Component, { props: { ...props, duplicateControls: true } }).body, /Multiple two-page editors own control/);
    assert.throws(() => render(Component, { props: { ...props, controlScope: 'common' } }).body, /no matching unbound coordinate/);
    for (const id of ['foreign', page.controls.find(control => control.column === 'FieldNumber').controlId,
      page.controls.find(control => control.controlName === 'btnCopyToUserSU').controlId]) {
      assert.throws(() => render(Component, { props: { ...props, controls: [id] } }).body, /no matching unbound coordinate/);
    }
  }
});
test('complete layout values preserve exact physical storage and reject foreign variants or changed ownership', () => {
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const r = original(form);
    set(r, 'PlotType', cell('text', '  Custom  '));
    set(r, 'GIS_BGC_VER', cell('integer', '9007199254740993'));
    set(r, 'SV_FloodPlain', cell('integer', '-1'));
    set(r, 'SV_AhorizonType', cell('text', ''));
    const before = JSON.stringify(r), values = projection.twoPageEntryValues(r, form);
    assert.equal(values.size, form.endsWith('-CHARS') ? 120 : 118);
    assert.equal(values.get('plotnumber'), 'P');
    assert.equal(values.get('plottype'), '  Custom  ');
    assert.equal(values.get('gis_bgc_ver'), '9007199254740993');
    assert.equal(values.get('sv_floodplain'), '-1');
    assert.equal(values.get('sv_ahorizontype'), '');
    assert.equal(values.get('sv_soildepth'), null);
    values.set('plottype', 'not written');
    assert.equal(JSON.stringify(r), before);
    assert.throws(() => projection.twoPageEntryValues(r,
      form.endsWith('-CHARS') ? 'FS882-8x6XL' : 'FS882-8x6XL-CHARS'), /exact source variant/);
    const changed = structuredClone(r);
    changed.Bindings.find(binding => binding.Binding === 'PlotType').Table = changed.EnvTable;
    assert.throws(() => projection.twoPageEntryValues(changed, form), /ownership changed/);
  }
});
test('real Common and Extra inputs compose into the complete source layout with one associated label and unchanged safety feedback', async () => {
  const read = file => readFileSync(`${__dirname}\\${file}.svelte`, 'utf8');
  const SourcePage = serverComponent(read('SourcePage'), 'SourcePage.svelte', {
    './paperLayout': paper, './formPresentation': presentation,
  });
  const Layout = serverComponent(read('TwoPageEntryLayout'), 'TwoPageEntryLayout.svelte', {
    './SourcePage.svelte': { default: SourcePage }, './twoPageEntryProjection': projection,
    './coordinateEditor': coordinates,
  });
  const Common = serverComponent(read('TwoPageParentCommonFields'), 'TwoPageParentCommonFields.svelte', {
    './twoPageParentCommonSession': api, './projectMetadataEditor': metadata,
  });
  const Extra = serverComponent(read('TwoPageParentFields'), 'TwoPageParentFields.svelte', {
    './twoPageParentSession': extra, './projectMetadataEditor': metadata,
  });
  const template = `<script>
    import Fields from './Fields.svelte'; import Layout from './Layout.svelte';
    export let form; export let view; export let values; export let scope;
    const choices = [{item:'  Custom  ',itemDescription:'Literal source suggestion'}];
  </script>
  <Fields {form} {view} {choices} choicesBusy={false} choicesError="Reference retry required"
    disabled={false} readDisabled={false} canSave={true} onstage={()=>{}} onoperation={()=>{}} oncancel={()=>{}} onchoices={()=>{}}>
    {#snippet children(editor)}
      <Layout {form} {values} editors={[{...editor,scope}]}>
        {#snippet identity(field)}<input readonly data-header={field.column} value="P"/>{/snippet}
        {#snippet embedded(child,control)}<span data-linked-child={child.form}></span>{/snippet}
      </Layout>
    {/snippet}
  </Fields>`;
  for (const form of ['FS882-8x6XL', 'FS882-8x6XL-CHARS']) {
    const r = original(form);
    set(r, 'SV_StandAgeEstMeas', cell('text', '0'));
    set(r, 'SV_FloodPlain', cell('integer', '1'));
    for (const scope of ['common', 'extra']) {
      const session = scope === 'common' ? fixture(form, {}, r).session :
        new extra.TwoPageParentSession(owner, form, { read: async () => structuredClone(r) }, () => {});
      assert.equal(await session.load(), true);
      const view = session.view();
      view.error = 'Owned draft safety feedback';
      const Component = serverComponent(template, 'LiveTwoPageFieldsFixture.svelte', {
        './Fields.svelte': { default: scope === 'common' ? Common : Extra }, './Layout.svelte': { default: Layout },
      });
      const html = render(Component, { props: { form, scope, view,
        values: projection.twoPageEntryValues(r, form) } }).body;
      const fields = scope === 'common' ? api.twoPageParentCommonFields(form) : extra.twoPageParentFields(form);
      for (const field of fields) {
        const id = `${scope === 'common' ? 'two-page-common' : 'two-page'}-${field.column}`;
        assert.equal((html.match(new RegExp(`id="${id}"`, 'g')) || []).length, 1, id);
        assert.equal((html.match(new RegExp(`for="${id}"`, 'g')) || []).length, 1, id);
      }
      assert.doesNotMatch(html, /<label[^>]*>(?:(?!<\/label>)[\s\S])*<label/);
      assert.match(html, /Owned draft safety feedback/);
      assert.equal((html.match(/data-linked-child=/g) || []).length, 5);
      assert.match(html, /Pictures unavailable/);
      if (scope === 'common') {
        assert.match(html, /Historical 0/);
        assert.match(html, /Historical 1/);
        assert.match(html, /Reference retry required/);
        assert.match(html, /Literal source suggestion/);
      }
    }
  }
});
