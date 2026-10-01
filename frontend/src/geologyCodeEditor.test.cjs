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
const editor = load('geologyCodeEditor.ts', { './referenceCodeEditor': reference });
const codes = changes => ({ bedrockGeology1: null, bedrockGeology2: null, bedrockGeology3: null, ...changes });
const row = (rowId, code, changes = {}) => ({
  rowId, listName: 'BedrockType', code, description: null, listFilter: '', itemOrder: -0,
  fieldUsedIn: null, validateLoops: null, validate: false, note: '', flag: null,
  selectable: code !== null && code !== '', diagnostic: '', ...changes
});
const ready = { choices: [row('1', 'AA'), row('2', 'AA'), row('3', ''), row('4', null)],
  ready: true, busy: false, error: null };

test('Bedrock uses independent nullable four-unit codes without normalization or cascading', () => {
  for (const key of editor.geologyCodeKeys) {
    for (const value of [null, 'AA', "q'X ", 'A😀B', 'ca']) {
      assert.equal(editor.geologyCodeValueError(key, value, null), null);
    }
    assert.match(editor.geologyCodeValueError(key, '12345', null), /at most 4/);
    assert.match(editor.geologyCodeValueError(key, '\ud800', null), /incomplete Unicode/);
    assert.match(editor.geologyCodeValueError(key, '', null), /NULL/);
    assert.equal(editor.geologyCodeValueError(key, 'historical-long', 'historical-long'), null);
  }
  assert.equal(editor.geologyCodeKey('BedrockGeology3'), 'bedrockGeology3');
  assert.equal(editor.geologyCodeKey('CoarseFragLith1'), undefined);
  assert.equal(editor.geologyCodeValidation(codes({ bedrockGeology2: 'AA' }), codes(), ready, false), null);
  const groups = editor.geologyCodeGroups(ready.choices, 'BedrockType');
  assert.equal(groups.length, 1);
  assert.equal(groups[0].records.length, 2);
  assert.equal(groups[0].records[0], ready.choices[0]);
});

test('Bedrock review is draft-bound, exact-value-bound and retained across remounts', () => {
  const draft = codes({ bedrockGeology1: 'raw' });
  assert.match(editor.geologyCodeValidation(draft, codes(), ready, false), /acknowledge/);
  editor.rememberGeologyCodeAcknowledgement(draft, draft, true);
  assert.equal(editor.geologyCodeAcknowledged(draft, draft), true);
  assert.equal(editor.geologyCodeValidation(draft, codes(), ready, true), null);
  draft.bedrockGeology3 = "q'X ";
  assert.equal(editor.geologyCodeAcknowledged(draft, draft), false);
  assert.equal(editor.geologyCodeAcknowledged({ ...draft }, draft), false);
  const pending = { ...ready, busy: true };
  assert.equal(editor.geologyCodeBusy(draft, codes(), pending), true);
  assert.match(editor.geologyCodeValidation(draft, codes(), pending, true), /Wait/);
  assert.equal(editor.geologyCodeBusy(codes(), codes(), pending), false);
  const failed = { choices: [], busy: false, ready: false, error: 'Checksum failed' };
  assert.deepEqual(Array.from(editor.geologyCodeWarnings(draft, codes(), failed)), ['Checksum failed', 'Checksum failed']);
  assert.match(editor.geologyCodeValidation(draft, codes(), failed, false), /acknowledge/);
  assert.equal(editor.geologyCodeValidation(draft, codes(), failed, true), null);
});

test('Bedrock defaults on with opt-out and wires source slots, hidden validation and explicit Retry', () => {
  const source = read('GeologyCodeFields.svelte'), form = read('FS882Form.svelte');
  assert.equal(compile(source, { filename: 'GeologyCodeFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_GEOLOGY_CODES_EDITING !== 'false'/);
  assert.match(source, /onDestroy[\s\S]*onvalidation\('geologyCodes'/);
  assert.match(source, /Retry bedrock choices/);
  assert.match(form, /soilCodeBusy \|\| geologyCodeBusy/);
  assert.match(form, /headerValidation\.soilCodes \|\| message === headerValidation\.geologyCodes/);
  assert.match(form, /editors=\{\[\.\.\.\(geologyEditor \? \[geologyEditor\] : \[\]\), \.\.\.\(parentEditor \? \[parentEditor\] : \[\]\), \.\.\.\(ordinaryEditor \? \[ordinaryEditor\] : \[\]\), \.\.\.\(drainageEditor \? \[drainageEditor\] : \[\]\)\]\}/);
});

test('Actual Bedrock component renders only its three source slots with default enablement and opt-out', () => {
  const controls = editor.geologyCodeColumns.map((column, index) => ({
    controlId: column, controlName: column, column, type: 'ComboBox', caption: '',
    x: 183 + index * 60, y: 30, width: 895 / 15, height: 18,
    fontName: 'Arial', fontSize: 10, bold: false, textAlign: 'center'
  }));
  const SourcePage = serverComponent(read('SourcePage.svelte'), 'SourcePage.svelte', {
    './paperLayout': { paperPage: () => ({ contentWidth: 600, height: 200, controls }), accessCaption: value => value },
    './formPresentation': presentation
  });
  function renderEditor(enabled) {
    const Component = serverComponent(read('GeologyCodeFields.svelte')
      .replace("import.meta.env.VITE_GEOLOGY_CODES_EDITING",
        enabled === undefined ? 'undefined' : JSON.stringify(enabled ? 'true' : 'false')),
      'GeologyCodeFields.svelte', {
        '../bindings/github.com/boostao/vpro-wails': { GeologyCodeService: {
          ListBedrockChoices() { throw new Error('SSR must not call native bindings'); }
        } },
        './qualityEditor': { QualityLookup: class { snapshot() { return ready; } dispose() {} } },
        './geologyCodeEditor': editor
      });
    const fixture = `
      <script>
        import Component from './Component.svelte';
        import SourcePage from './SourcePage.svelte';
        let { draft, capabilities } = $props();
      </script>
      <Component bind:draft original={null} {capabilities} disabled={false}
        onchange={() => {}} onvalidation={() => {}}>
        {#snippet children(editor)}
          <SourcePage name="Soil/Terrain" {editor}>
            {#snippet embedded(control)}<span>Child</span>{/snippet}
          </SourcePage>
        {/snippet}
      </Component>`;
    const Fixture = serverComponent(fixture, 'GeologyFixture.svelte', {
      './Component.svelte': { default: Component }, './SourcePage.svelte': { default: SourcePage }
    });
    return render(Fixture, { props: { draft: codes({ bedrockGeology2: "q'X " }),
      capabilities: { bedrockGeology1: true, bedrockGeology2: true, bedrockGeology3: true } } }).body;
  }
  assert.ok(renderEditor(false).match(/<input\b[^>]*>/g).every(input => /\bdisabled\b/.test(input)));
  assert.equal((renderEditor().match(/class="geology-code-control/g) ?? []).length, 3);
  const html = renderEditor(true);
  for (let index = 0; index < 3; index++) {
    assert.match(html, new RegExp(`id="header-bedrockGeology${index + 1}"`));
    assert.match(html, /min-height:40px;font:inherit/);
    assert.doesNotMatch(html, /position:absolute|left:|top:|height:18px/);
  }
  assert.equal((html.match(/class="geology-code-control/g) ?? []).length, 3);
  assert.match(html, /All frozen bedrock reference rows \(4\)/);
  assert.match(html, /data-kind="null"/);
  assert.match(html, /data-empty="true"/);
  assert.match(html, /data-negative-zero="true"/);
});
