const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const read = name => readFileSync(path.join(__dirname, name), 'utf8');
function load(name, dependencies = {}) {
  const exports = {};
  vm.runInNewContext(ts.transpileModule(read(name), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
  }).outputText, { exports, require: dependency => {
    if (!(dependency in dependencies)) throw new Error(`Unexpected dependency ${dependency}`);
    return dependencies[dependency];
  } });
  return exports;
}
const bec = load('becEditor.ts');
const quality = load('qualityEditor.ts', { './becEditor': bec });
const reference = load('referenceCodeEditor.ts', { './becEditor': bec, './qualityEditor': quality });
const moduleEditor = load('parentCodeEditor.ts', { './referenceCodeEditor': reference });
const { parentCodeFields: fields, createParentCodeEditor } = moduleEditor;
const codes = changes => ({ ...Object.fromEntries(fields.map(field => [field.key, null])), ...changes });
const row = (listName, code, rowId = '1') => ({
  rowId, listName, code, description: null, listFilter: '', itemOrder: -0,
  fieldUsedIn: null, validateLoops: null, validate: false, note: '', flag: null,
  selectable: code !== null && code !== '', diagnostic: ''
});
const views = () => Object.fromEntries(fields.map(field => [field.list, {
  busy: false, ready: true, error: null,
  choices: [row(field.list, 'A'), row(field.list, 'A', '2'), row(field.list, '', '3'), row(field.list, null, '4')]
}]));

test('Every parent descriptor matches its backend field, list and UTF-16 storage length', () => {
  const backend = readFileSync(path.join(__dirname, '..', '..', 'parentcodeheader.go'), 'utf8');
  const source = JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'fs882-xl-layout.json'), 'utf8'));
  assert.equal(fields.length, 21);
  assert.equal(new Set(fields.map(field => field.column)).size, 21);
  for (const field of fields) {
    assert.ok(backend.includes(`{"${field.column}", "${field.key}", "Env", ${field.maximum}, h.`));
    assert.ok(backend.includes(`}, "${field.list}"}`));
    assert.equal(moduleEditor.parentCodeField(field.column), field);
    assert.ok(JSON.stringify(source).includes(`"column":"${field.column}"`) ||
      JSON.stringify(source).includes(`"column": "${field.column}"`));
    const editor = createParentCodeEditor(field.key === 'realmClass' ? 'site' : 'soils');
    assert.equal(editor.valueError(field.key, 'x'.repeat(field.maximum), null), null);
    assert.match(editor.valueError(field.key, 'x'.repeat(field.maximum + 1), null), /at most/);
    assert.match(editor.valueError(field.key, '\ud800', null), /incomplete Unicode/);
    assert.match(editor.valueError(field.key, '', null), /NULL/);
    assert.equal(editor.valueError(field.key, null, null), null);
    assert.equal(editor.valueError(field.key, 'historical-invalid'.repeat(4), 'historical-invalid'.repeat(4)), null);
    assert.equal(editor.valueError(field.key, '😀', null), field.maximum < 2
      ? `${field.label} must be at most 1 UTF-16 characters; the raw entry has not been truncated.` : null);
  }
  assert.equal(moduleEditor.parentCodeField('SoilDrainage'), undefined);
  assert.equal(moduleEditor.parentCodeField('BedrockGeology1'), undefined);
});

test('Parent scopes use independent lists, full Items and scope-bound draft review across remounts', () => {
  const editor = createParentCodeEditor('soils');
  const site = createParentCodeEditor('site');
  const ready = views(), original = codes(), draft = codes({ humusForm: 'A', waterSource: "q'X " });
  assert.match(editor.validation(draft, original, ready, false), /acknowledge/);
  editor.rememberAcknowledgement(draft, draft, true);
  assert.equal(createParentCodeEditor('soils').acknowledged(draft, draft), true);
  assert.equal(editor.validation(draft, original, ready, true), null);
  assert.equal(editor.acknowledged({ ...draft }, draft), false);
  draft.realmClass = 'raw';
  assert.equal(editor.acknowledged(draft, draft), true);
  assert.equal(site.acknowledged(draft, draft), false);
  assert.match(site.validation(draft, original, ready, false), /acknowledge/);
  draft.waterSource = 'ca';
  assert.equal(editor.acknowledged(draft, draft), false);
  assert.equal(editor.groups(ready.HumusForm.choices, 'HumusForm').length, 1);
  assert.equal(editor.groups(ready.HumusForm.choices, 'HumusForm')[0].records.length, 2);
  assert.equal(editor.groups(ready.HumusForm.choices, 'WaterSource').length, 0);
  assert.equal(editor.selectable(row('HumusForm', ''), 'HumusForm'), false);
  assert.equal(editor.selectable(row('HumusForm', null), 'HumusForm'), false);
  assert.equal(draft.waterSource, 'ca');
  assert.equal(draft.hydroGeoSystem, null);
});

test('Reference loading and failures are per-list, unchanged historical data stays writable and invalid values survive hidden tabs', () => {
  const editor = createParentCodeEditor('soils'), ready = views(), original = codes();
  ready.HumusForm = { ...ready.HumusForm, busy: true, ready: false };
  const draft = codes({ humusForm: 'raw' });
  assert.equal(editor.busy(draft, original, ready), true);
  assert.match(editor.validation(draft, original, ready, true), /Wait/);
  assert.equal(editor.busy(codes({ waterSource: 'A' }), original, ready), false);
  assert.equal(editor.validation(codes({ waterSource: 'A' }), original, ready, false), null);
  ready.HumusForm = { ...ready.HumusForm, busy: false, error: 'Checksum failed' };
  assert.match(editor.warnings(draft, original, ready)[0], /Humus form: Checksum failed/);
  assert.match(editor.validation(draft, original, ready, false), /acknowledge/);
  assert.equal(editor.validation(draft, original, ready, true), null);
  draft.humusForm = '12345';
  assert.match(editor.validation(draft, original, ready, true), /at most 4/);
  assert.match(createParentCodeEditor('soils').validation(draft, original, ready, true), /at most 4/);
  draft.humusForm = null;
  assert.equal(editor.validation(draft, original, ready, false), null);
  const historical = codes({ humusForm: 'old-invalid-code' });
  assert.equal(editor.validation(historical, historical, {}, false), null);
  assert.match(editor.validation(codes({ waterSource: 'raw' }), original, {}, false), /acknowledge/);
});

test('Parent component renders exactly its source slots, metadata distinctions and verified default with opt-out', () => {
  const source = read('ParentCodeFields.svelte');
  assert.equal(compile(source, { filename: 'ParentCodeFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_PARENT_CODES_EDITING !== 'false'/);
  assert.match(source, /onDestroy[\s\S]*onvalidation\(validationKey/);
  assert.match(source, /Retry \{list\} choices/);
  assert.ok(source.indexOf('{@render children(') < source.indexOf('<FieldGuidance'));
  function renderScope(scope, enabled) {
    const controls = fields.filter(field => (field.key === 'realmClass') === (scope === 'site')).map(field => ({
      controlId: field.column, controlName: field.column, column: field.column, type: 'ComboBox', caption: ''
    }));
    controls.push({ controlId: 'SoilDrainage', controlName: 'SoilDrainage', column: 'SoilDrainage', type: 'ComboBox', caption: '' });
    const SourcePage = serverComponent(read('SourcePage.svelte'), 'SourcePage.svelte', {
      './paperLayout': { paperPage: () => ({ controls }), accessCaption: value => value },
      './formPresentation': presentationHelpers().presentation
    });
    const Component = serverComponent(source.replace('import.meta.env.VITE_PARENT_CODES_EDITING',
      enabled === undefined ? 'undefined' : enabled ? "'true'" : "'false'"), 'ParentCodeFields.svelte', {
      '../bindings/github.com/boostao/vpro-wails': { ParentCodeService: { ListChoices() { throw Error('No native calls in SSR'); } } },
      './qualityEditor': { QualityLookup: class {
        constructor(fetch, publish, list) { this.list = list; }
        snapshot() { return views()[this.list]; } dispose() {}
      } }, './parentCodeEditor': moduleEditor
    });
    const Fixture = serverComponent(`
      <script>
        import Component from './Component.svelte';
        import SourcePage from './SourcePage.svelte';
        let { draft, scope, capabilities } = $props();
      </script>
      <Component bind:draft original={null} {scope} {capabilities} disabled={false} onchange={() => {}} onvalidation={() => {}}>
        {#snippet children(editor)}
          <SourcePage name={scope === 'site' ? 'Site' : 'Soil/Terrain'} {editor}>
            {#snippet embedded(control)}<span>Child</span>{/snippet}
          </SourcePage>
        {/snippet}
      </Component>`, 'ParentFixture.svelte', {
      './Component.svelte': { default: Component }, './SourcePage.svelte': { default: SourcePage }
    });
    return render(Fixture, { props: { draft: codes(), scope,
      capabilities: Object.fromEntries(fields.map(field => [field.key, true])) } }).body;
  }
  for (const scope of ['site', 'soils']) {
    const off = renderScope(scope, false);
    assert.ok(off.match(/<input\b[^>]*>/g).every(input => /\bdisabled\b/.test(input)));
    const html = renderScope(scope, true);
    assert.equal((renderScope(scope).match(/class="parent-code-control/g) ?? []).length, scope === 'site' ? 1 : 20);
    assert.equal((html.match(/class="parent-code-control/g) ?? []).length, scope === 'site' ? 1 : 20);
    assert.match(html, /data-column="SoilDrainage"[^>]*disabled/);
    assert.match(html, /data-kind="null"/);
    assert.match(html, /data-empty="true"/);
    assert.match(html, /data-negative-zero="true"/);
    assert.doesNotMatch(html, /position:absolute/);
  }
  const form = read('FS882Form.svelte'), header = read('HeaderEditor.svelte');
  assert.match(form, /scope="site"/);
  assert.match(form, /scope="soils"/);
  assert.match(form, /geologyCodeBusy \|\| parentCodeBusy/);
  assert.match(form, /headerValidation\['parentCodes-soils'\]/);
  assert.match(header, /editor\.input\(control, position\(control\)\)/);
});
