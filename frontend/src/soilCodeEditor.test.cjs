const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const { presentation } = presentationHelpers();
const read = name => readFileSync(path.join(__dirname, name), 'utf8');
function load(name, dependencies = {}) {
  const exports = {};
  vm.runInNewContext(ts.transpileModule(read(name), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
  }).outputText, { exports, require: name => {
    if (!(name in dependencies)) throw new Error(`Unexpected dependency ${name}`);
    return dependencies[name];
  } });
  return exports;
}
const bec = load('becEditor.ts');
const quality = load('qualityEditor.ts', { './becEditor': bec });
const reference = load('referenceCodeEditor.ts', { './becEditor': bec, './qualityEditor': quality });
const editor = load('soilCodeEditor.ts', { './referenceCodeEditor': reference });
const plain = value => JSON.parse(JSON.stringify(value));
const codes = changes => ({ soilClassGroup: null, soilClassSubGroup: null, ...changes });
const row = (rowId, listName, code, overrides = {}) => ({
  rowId, listName, code, description: 'Definition', listFilter: null, itemOrder: -0,
  fieldUsedIn: null, validateLoops: null, validate: false, note: '', flag: null,
  selectable: code !== null && code !== '', diagnostic: '', ...overrides
});
const ready = () => ({
  SoilClassGroup: { choices: [row('1', 'SoilClassGroup', 'BG'), row('2', 'SoilClassGroup', 'BG', { description: 'Duplicate' }),
    row('3', 'SoilClassGroup', '')], ready: true, busy: false, error: null },
  SoilClassSubgroup: { choices: [row('1', 'SoilClassSubgroup', 'BGL'), row('2', 'SoilClassSubgroup', '')],
    ready: true, busy: false, error: null }
});
test('Shared reference engine keeps per-workflow acknowledgement stores isolated on the same draft', () => {
  const region = load('regionCodeEditor.ts', { './referenceCodeEditor': reference });
  const site = load('siteCodeEditor.ts', { './referenceCodeEditor': reference });
  const draft = {
    ...codes({ soilClassGroup: 'raw', soilClassSubGroup: null }),
    fsRegionDistrict: 'raw', ecosection: null,
    siteDisturbance1: 'raw', siteDisturbance2: null, siteDisturbance3: null, exposure1: null, exposure2: null
  };
  editor.rememberSoilCodeAcknowledgement(draft, draft, true);
  assert.equal(editor.soilCodeAcknowledged(draft, draft), true);
  assert.equal(region.regionCodeAcknowledged(draft, draft), false);
  assert.equal(site.siteCodeAcknowledged(draft, draft), false);
  region.rememberRegionCodeAcknowledgement(draft, draft, true);
  site.rememberSiteCodeAcknowledgement(draft, draft, true);
  draft.soilClassGroup = 'RAW';
  assert.equal(editor.soilCodeAcknowledged(draft, draft), false);
  assert.equal(region.regionCodeAcknowledged(draft, draft), true);
  assert.equal(site.siteCodeAcknowledged(draft, draft), true);
  draft.exposure2 = 'AT';
  assert.equal(site.siteCodeAcknowledged(draft, draft), false);
  assert.equal(region.regionCodeAcknowledged(draft, draft), true);
  region.rememberRegionCodeAcknowledgement(draft, draft, false);
  assert.equal(region.regionCodeAcknowledged(draft, draft), false);
});
test('Consolidation retains exact error strings, workflow validation ordering and strict Exposure membership', () => {
  const region = load('regionCodeEditor.ts', { './referenceCodeEditor': reference });
  const site = load('siteCodeEditor.ts', { './referenceCodeEditor': reference });
  for (const [error, key, label, limit] of [
    [region.regionCodeValueError, 'fsRegionDistrict', 'Region/district', 7],
    [region.regionCodeValueError, 'ecosection', 'Ecosection', 3],
    [site.siteCodeValueError, 'siteDisturbance1', 'Site disturbance 1', 8],
    [site.siteCodeValueError, 'exposure2', 'Exposure 2', 2],
    [editor.soilCodeValueError, 'soilClassGroup', 'Soil great group', 4],
    [editor.soilCodeValueError, 'soilClassSubGroup', 'Soil subgroup', 4]
  ]) {
    assert.equal(error(key, '', null), `Clear ${label} to NULL instead of an empty string.`);
    assert.equal(error(key, '\ud800', null), `${label} contains an incomplete Unicode character; it has not been replaced.`);
    assert.equal(error(key, 'x'.repeat(limit + 1), null),
      `${label} must be at most ${limit} UTF-16 characters; the raw entry has not been truncated.`);
  }
  const pending = { choices: [], ready: false, busy: true, error: null };
  const regionCodes = { fsRegionDistrict: 'new', ecosection: 'long' };
  assert.equal(region.regionCodeValidation(regionCodes, null, { Region: pending, Ecosection: pending }, true),
    'Wait for Region/district choices to finish refreshing.');
  const soilCodes = codes({ soilClassGroup: 'new', soilClassSubGroup: 'longer' });
  assert.match(editor.soilCodeValidation(soilCodes, null, { SoilClassGroup: pending, SoilClassSubgroup: pending }, true),
    /^Soil subgroup must be at most 4/);
  const siteCodes = { siteDisturbance1: 'new', siteDisturbance2: null, siteDisturbance3: null, exposure1: 'long', exposure2: null };
  assert.match(site.siteCodeValidation(siteCodes, null, { SiteDisturbance: pending, Exposure: pending }, true),
    /^Exposure 1 must be at most 2/);
  siteCodes.siteDisturbance1 = null;
  siteCodes.exposure1 = 'at';
  const views = { SiteDisturbance: pending, Exposure: { choices: [row('1', 'Exposure', 'AT')], ready: true, busy: false, error: null } };
  assert.match(site.siteCodeValidation(siteCodes, null, views, true), /must use an exact Item/);
  siteCodes.exposure1 = 'AT';
  assert.equal(site.siteCodeValidation(siteCodes, null, views, false), null);
});
test('Shared reference grouping retains original typed row objects without normalizing codes or metadata', () => {
  const record = row('1', 'SoilClassGroup', "Q'x ", { note: '', itemOrder: -0 });
  const duplicate = row('2', 'SoilClassGroup', "Q'x ", { note: null, description: null });
  const rows = [record, duplicate, row('3', 'SoilClassSubgroup', "Q'x "), row('4', 'SoilClassGroup', "q'X ")];
  const before = JSON.stringify(rows);
  const groups = editor.soilCodeGroups(rows, 'SoilClassGroup');
  assert.equal(groups.length, 2);
  assert.equal(groups[0].code, "Q'x ");
  assert.equal(groups[0].records[0], record);
  assert.equal(groups[0].records[1], duplicate);
  assert.equal(editor.soilCodeDefinitions(rows, 'SoilClassGroup', "q'X ")[0], record);
  assert.equal(Object.is(groups[0].records[0].itemOrder, -0), true);
  assert.equal(groups[0].records[0].note, '');
  assert.equal(groups[0].records[1].note, null);
  assert.equal(JSON.stringify(rows), before);
});
test('Two independent parent codes retain nullable raw input and four UTF16 boundaries', () => {
  assert.deepEqual(plain(editor.soilCodeKeys), Object.keys(codes()));
  for (const [column, key] of [['SoilClassGroup', 'soilClassGroup'], ['SoilClassSubGroup', 'soilClassSubGroup']]) {
    assert.equal(editor.soilCodeKey(column), key);
    for (const value of [null, '1234', 'Z9', "q'X4", ' x ', '    ', '\u{1f600}\u{1f600}', 'e\u0301e\u0301']) {
      assert.equal(editor.soilCodeValueError(key, value, null), null);
      const draft = codes({ [key]: value });
      assert.equal(editor.soilCodeValidation(draft, codes(), ready(), true), null);
      assert.equal(draft[key], value);
      assert.equal(draft[key === 'soilClassGroup' ? 'soilClassSubGroup' : 'soilClassGroup'], null);
    }
    for (const value of ['12345', 'abc\u{1f600}', 'e\u0301e\u0301e']) {
      assert.match(editor.soilCodeValueError(key, value, null), /4 UTF-16.*not been truncated/);
    }
    for (const value of ['\ud800', '\udfff', '\ud800A', '\udfff\ud800']) {
      assert.match(editor.soilCodeValueError(key, value, null), /incomplete Unicode/);
    }
    assert.match(editor.soilCodeValueError(key, '', null), /NULL/);
  }
  assert.equal(editor.soilCodeKey('SoilClassSubgroup'), undefined);
  assert.equal(editor.soilCodeKey('SoilPlotQuality'), undefined);
});
test('NULL, unchanged historical malformed/empty/overlength codes and unrelated saves bypass catalogue gates', () => {
  const views = ready();
  for (const list of Object.keys(views)) views[list] = { choices: [], ready: false, busy: true, error: 'Offline' };
  for (const value of ['historical-long', '', '\ud800']) {
    const original = codes({ soilClassGroup: value, soilClassSubGroup: value });
    assert.equal(editor.soilCodeValidation({ ...original }, original, views, false), null);
    assert.equal(editor.soilCodeValidation(codes(), original, views, false), null);
    assert.equal(editor.soilCodeBusy(codes(), original, views), false);
  }
  const original = codes({ soilClassSubGroup: 'old-long' });
  const draft = codes({ soilClassGroup: 'BG', soilClassSubGroup: 'old-long' });
  assert.match(editor.soilCodeValidation(draft, original, views, true), /Wait/);
  views.SoilClassGroup = ready().SoilClassGroup;
  assert.equal(editor.soilCodeValidation(draft, original, views, false), null);
});
test('Selectable actual Items use exact ListName casing and full-code suggestions retain duplicate definitions', () => {
  const rows = [...ready().SoilClassGroup.choices, row('4', null, 'BG'), row('5', 'soilclassgroup', 'BG'),
    row('6', 'SoilClassSubgroup', 'BG'), row('7', 'SoilClassGroup', null),
    row('8', 'SoilClassGroup', 'abcde'), row('9', 'SoilClassGroup', '\ud800'),
    row('10', 'SoilClassGroup', 'BG', { selectable: false })];
  assert.deepEqual(plain(editor.soilCodeGroups(rows, 'SoilClassGroup').map(group => [group.code, group.records.length])), [['BG', 2]]);
  assert.deepEqual(plain(editor.soilCodeDefinitions(rows, 'SoilClassGroup', 'bg').map(row => row.rowId)), ['1', '2']);
  assert.deepEqual(plain(editor.soilCodeSuggestions(rows, 'soilClassGroup', 'b').map(group => group.code)), ['BG']);
  assert.equal(editor.soilCodeDefinitions(rows, 'SoilClassGroup', '').length, 0);
  assert.equal(editor.soilCodeDefinitions(rows, 'SoilClassGroup', null).length, 0);
  assert.equal(editor.soilCodeSuggestions(rows, 'soilClassSubGroup', 'b').length, 1);
  assert.equal(editor.soilCodeList('soilClassSubGroup'), 'SoilClassSubgroup');
});
test('Review requires exact two raw values and actual draft identity, survives remount but not new plot or text', () => {
  const draft = codes({ soilClassGroup: "q'X4", soilClassSubGroup: 'Z9' });
  assert.match(editor.soilCodeValidation(draft, codes(), ready(), false), /acknowledge/);
  editor.rememberSoilCodeAcknowledgement(draft, draft, true);
  assert.equal(editor.soilCodeAcknowledged(draft, draft), true);
  assert.equal(editor.soilCodeAcknowledged({ ...draft }, draft), false);
  for (const changes of [{ soilClassGroup: "q'x4" }, { soilClassSubGroup: 'z9' }, { soilClassGroup: 'qX4' }, { soilClassSubGroup: 'Z9 ' }]) {
    assert.equal(editor.soilCodeAcknowledged(draft, { ...draft, ...changes }), false);
  }
  editor.rememberSoilCodeAcknowledgement(draft, draft, false);
  assert.equal(editor.soilCodeAcknowledged(draft, draft), false);
});
test('Unavailable/failed catalogues permit explicit unchecked review, never bypass physical bounds or pending changed lookup', () => {
  const views = ready();
  views.SoilClassGroup = { choices: [], ready: false, busy: false, error: 'Group failed' };
  const draft = codes({ soilClassGroup: '1234', soilClassSubGroup: 'Z9' });
  assert.match(editor.soilCodeWarnings(draft, codes(), views).join(' '), /Group failed.*not a listed Item/);
  assert.match(editor.soilCodeValidation(draft, codes(), views, false), /acknowledge/);
  assert.equal(editor.soilCodeValidation(draft, codes(), views, true), null);
  views.SoilClassGroup.busy = true;
  assert.match(editor.soilCodeValidation(draft, codes(), views, true), /Wait/);
  assert.match(editor.soilCodeValidation(codes({ soilClassGroup: '12345' }), codes(), views, true), /4 UTF-16/);
});
test('Shared QualityLookup failed retry, stale success and disposal preserve safe hidden validation', async () => {
  let resolveOld, resolveDisposed, calls = 0;
  const publications = [];
  const lookup = new quality.QualityLookup(() => {
    calls++;
    if (calls === 1) return Promise.reject(new Error('Offline'));
    if (calls === 2) return new Promise(resolve => { resolveOld = resolve; });
    if (calls === 3) return Promise.resolve(ready().SoilClassGroup.choices);
    return new Promise(resolve => { resolveDisposed = resolve; });
  }, view => publications.push(view), 'Soil great group');
  await lookup.refresh();
  assert.match(lookup.snapshot().error, /Offline/);
  const stale = lookup.refresh();
  await lookup.refresh();
  resolveOld([row('old', 'SoilClassGroup', 'OLD')]);
  await stale;
  assert.equal(lookup.snapshot().choices[0].code, 'BG');
  const pending = lookup.refresh();
  assert.equal(lookup.snapshot().choices.length, 0);
  lookup.dispose();
  const count = publications.length;
  resolveDisposed([row('disposed', 'SoilClassGroup', 'BAD')]);
  await pending;
  assert.equal(publications.length, count);
  const hidden = ready();
  hidden.SoilClassGroup = { ...lookup.snapshot(), busy: false, ready: false };
  assert.match(editor.soilCodeValidation(codes({ soilClassGroup: 'BG' }), codes(), hidden, false), /acknowledge/);
  assert.equal(editor.soilCodeValidation(codes(), codes(), hidden, false), null);
});

const source = read('SoilCodeFields.svelte');
function component(enabled, views = ready()) {
  return serverComponent(source.replace("import.meta.env.VITE_SOIL_CODES_EDITING",
    enabled === undefined ? 'undefined' : JSON.stringify(enabled ? 'true' : 'false')),
    'SoilCodeFields.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { SoilCodeService: {
        ListGreatGroupChoices() { throw new Error('SSR must not call bindings'); },
        ListSubgroupChoices() { throw new Error('SSR must not call bindings'); }
      } },
      './qualityEditor': { QualityLookup: class {
        constructor(loader, callback, label) { this.list = label === 'Soil great group' ? 'SoilClassGroup' : 'SoilClassSubgroup'; }
        snapshot() { return views[this.list]; }
        dispose() {}
      } },
      './soilCodeEditor': editor
    });
}
function renderEditor(enabled, options = {}) {
  const control = (column, x, tabOrder, type = 'ComboBox') => ({
    column, x, y: 84, width: 805 / 15, height: 18, tabOrder, type, controlId: column,
    controlName: column, caption: column, fontName: 'Arial', fontSize: 10, bold: false, textAlign: 'center'
  });
  const page = { contentWidth: 600, height: 200, controls: [
    control('SoilClassSubGroup', 105, 16), control('SoilClassGroup', 231, 17),
    control('SoilPlotQuality', 350, 18, 'TextBox'),
    { ...control('SoilHumusXL', 9, 1, 'Subform'), width: 300, height: 70 },
    { ...control('SoilMineralXL', 9, 2, 'Subform'), width: 300, height: 70 }
  ] };
  const SourcePage = serverComponent(read('SourcePage.svelte'), 'SourcePage.svelte', {
    './paperLayout': { paperPage: () => page, accessCaption: value => value },
    './formPresentation': presentation
  });
  const Fixture = serverComponent(`
    <script>
      import SoilCodeFields from './SoilCodeFields.svelte';
      import SourcePage from './SourcePage.svelte';
      let { draft, original, capabilities, disabled } = $props();
      const values = $derived(new Map([['soilclassgroup', draft.soilClassGroup], ['soilclasssubgroup', draft.soilClassSubGroup],
        ['soilplotquality', 'Preserved']]));
    </script>
    <SoilCodeFields {draft} {original} {capabilities} {disabled} onchange={() => {}} onvalidation={() => {}}>
      {#snippet children(editor)}
        <SourcePage name="Soil/Terrain" {values} {editor}>
          {#snippet embedded(control)}<span data-child={control.controlName}>Child</span>{/snippet}
        </SourcePage>
      {/snippet}
    </SoilCodeFields>`, 'SoilEditorFixture.svelte', {
    './SoilCodeFields.svelte': { default: component(enabled, options.views) },
    './SourcePage.svelte': { default: SourcePage }
  });
  return render(Fixture, { props: { draft: codes({ soilClassGroup: 'BG', soilClassSubGroup: 'BGL' }),
    original: codes(), capabilities: { soilClassGroup: true, soilClassSubGroup: true }, disabled: false, ...options } }).body;
}
test('Actual SSR editor defaults on with readonly opt-out, responsive labels and preserved children', () => {
  assert.equal(compile(source, { filename: 'SoilCodeFields.svelte', generate: 'client' }).warnings.length, 0);
  const off = renderEditor(false);
  assert.doesNotMatch(off, /soil-code-toolbar|soil-code-control|data-reference-row/);
  assert.ok(off.match(/<input\b[^>]*>/g).every(input => /\bdisabled\b/.test(input)));
  assert.equal((renderEditor().match(/class="soil-code-control/g) ?? []).length, 2);
  const html = renderEditor(true);
  for (const [column, x] of [['SoilClassGroup', 231], ['SoilClassSubGroup', 105]]) {
    const input = html.match(/<input\b[^>]*>/g).find(input => input.includes(`data-column="${column}"`));
    assert.match(input, /min-height:40px;font:inherit/);
    assert.doesNotMatch(input, /position:absolute|left:|top:|height:18px/);
    assert.match(input, new RegExp(`data-source-control="${column}"`));
    assert.doesNotMatch(input, /\bdisabled\b/);
  }
  const neighbor = html.match(/<input\b[^>]*>/g).find(input => input.includes('SoilPlotQuality'));
  assert.match(neighbor, /disabled/);
  assert.match(neighbor, /value="Preserved"/);
  assert.match(html, /data-child="SoilHumusXL"/);
  assert.match(html, /data-child="SoilMineralXL"/);
  assert.equal((html.match(/class="soil-code-control/g) ?? []).length, 2);
});
test('Actual SSR exposes 13 typed metadata properties, empty codes, duplicate definitions and only full Item buttons', () => {
  const html = renderEditor(true);
  assert.match(html, /data-reference-list="SoilClassGroup"/);
  assert.match(html, /data-reference-list="SoilClassSubgroup"/);
  assert.match(html, /data-property="note" data-value="&quot;&quot;" data-kind="string" data-empty="true"/);
  assert.match(html, /data-property="flag" data-value="null" data-kind="null"/);
  assert.match(html, /data-property="itemOrder" data-value="0" data-kind="number" data-negative-zero="true"/);
  assert.equal(Object.keys(row('1', 'SoilClassGroup', 'BG')).length, 13);
  assert.match(html, /2 reference definitions/);
  assert.match(html, /Use BG for Soil great group/);
  assert.match(html, /Use BGL for Soil subgroup/);
  assert.doesNotMatch(html, />Use  for|>Use NULL for/);
});
test('Actual SSR errors and capability gates are explicit without disabling inputs for own lookup busy', () => {
  const views = ready();
  views.SoilClassGroup = { choices: [], ready: false, busy: false, error: 'Great group failed' };
  views.SoilClassSubgroup = { choices: [], ready: false, busy: true, error: null };
  const html = renderEditor(true, { views });
  assert.match(html, /Great group failed/);
  assert.match(html, /Retry great group choices/);
  assert.match(html, /Refreshing soil classification choices/);
  assert.match(html, /Wait for changed soil classification choices/);
  assert.ok(html.match(/<input\b[^>]*class="soil-code-control[^>]*>/g).every(input => !/\bdisabled\b/.test(input)));
  for (const options of [{ disabled: true }, { capabilities: {} }]) {
    const blocked = renderEditor(true, options);
    assert.ok(blocked.match(/<input\b[^>]*class="soil-code-control[^>]*>/g).every(input => /\bdisabled\b/.test(input)));
  }
});
test('Parent integrates hidden validation, lifecycle/reset gates and default-on/no-double-fetch adaptation', () => {
  const form = read('FS882Form.svelte');
  assert.equal(compile(form, { filename: 'FS882Form.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(form, /VITE_SOIL_CODES_EDITING !== 'false'/);
  assert.match(form, /regionCodeBusy \|\| soilCodeBusy/);
  assert.match(form, /if !soilCodeEditingEnabled.*SoilCodeReference/);
  assert.match(form, /onvalidation=\{validateHeader\} onbusy=\{\(pending\) => soilCodeBusy = pending\}/);
  assert.match(form, /<SourcePage name="Soil\/Terrain" values=\{sourceHeaderValues\} \{editor\}/);
  assert.match(form, /invalid header input on Soils or Undo/);
  assert.match(form, /Object\.keys\(headerValidation\)\.length > 0/);
  assert.match(form, /draft = newDraft\(\);\s+original = null;\s+dirty = false;\s+headerValidation = \{\}/);
  assert.match(source, /onDestroy[\s\S]*hiddenViews[\s\S]*onvalidation\('soilCodes', soilCodeValidation/);
  assert.match(source, /raw === '' \? null : raw/);
  assert.match(source, /soilCodeAcknowledged\(draft, draft\)/);
  assert.doesNotMatch(source, /maxlength|\.slice\(|\.substring\(|\.trim\(|\.toUpperCase\(|PlotService|UpdatePlot|CreatePlot|<datalist/);
  const disabled = form.match(/<SoilCodeFields[\s\S]*?onchange=/)[0];
  assert.doesNotMatch(disabled, /headerWorkflowBusy|soilCodeBusy/);
  assert.match(disabled, /headerInputsDisabled/);
  const header = form.match(/const headerInputsDisabled = \$derived\(([^;\n]+)\);/)[1];
  assert.match(header, /draft.locked.*busy.*!capabilitiesReady/);
  assert.doesNotMatch(header, /headerWorkflowBusy|soilCodeBusy/);
});
