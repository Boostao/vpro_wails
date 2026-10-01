const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { compile } = require('svelte/compiler');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const bec = loadTypeScript('becEditor.ts');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': bec });
const numeric = loadTypeScript('numericEditor.ts');
const editor = loadTypeScript('ordinaryEditor.ts', { './qualityEditor': quality, './numericEditor': numeric });
const { ordinaryFields, ordinaryField, ordinaryTextError, ordinaryNumberValue,
  ordinaryValidation, ordinarySession, rememberOrdinarySession } = editor;
const values = () => Object.fromEntries([...ordinaryFields, ...editor.additionalParentFields].map(field => [field.key, null]));

test('Fourteen original fields retain scopes independently from later scalar and constrained workflows', () => {
  assert.equal(ordinaryFields.length, 14);
  assert.equal(ordinaryFields.filter(field => field.scope === 'site').length, 2);
  assert.equal(ordinaryFields.filter(field => field.scope === 'veg').length, 6);
  assert.equal(ordinaryFields.filter(field => field.scope === 'soils').length, 6);
  for (const column of ['BECSiteUnit', 'SpeciesListComplete', 'UpdatedFromCards', 'SoilDrainage']) {
    assert.equal(ordinaryField(column), undefined);
  }
});

test('Text bounds use UTF16 without trimming, case changes or arbitrary memo limits', () => {
  for (const field of ordinaryFields.filter(field => ['text', 'memo'].includes(field.kind))) {
    for (const value of [null, '  q\'X4  ', '\r\nline\n', 'e\u0301', '\u{1f600}']) {
      assert.equal(ordinaryTextError(field, value, null), null);
    }
    assert.ok(ordinaryTextError(field, '', null));
    assert.ok(ordinaryTextError(field, '\ud800', null));
    if (field.kind === 'text') {
      const bound = 'x'.repeat(field.maximum);
      assert.equal(ordinaryTextError(field, bound, null), null);
      assert.ok(ordinaryTextError(field, bound + 'x', null));
      assert.ok(ordinaryTextError(field, 'x'.repeat(field.maximum - 1) + '\u{1f600}', null));
      assert.equal(ordinaryTextError(field, bound + 'x', bound + 'x'), null);
    } else {
      assert.equal(ordinaryTextError(field, 'x'.repeat(100000), null), null);
    }
  }
});

test('Integer depths and Single summaries retain raw errors and physical domains, not guessed clinical limits', () => {
  for (const field of ordinaryFields.filter(field => ['integer', 'single'].includes(field.kind))) {
    for (const raw of ['', '-1', '0', '100', '101', '2e1']) {
      assert.equal(ordinaryNumberValue(field, raw, null).error, null);
    }
    for (const raw of ['1e', '-', 'NaN', 'Infinity', '0x10', '1e309']) {
      const cell = ordinaryNumberValue(field, raw, null);
      assert.equal(cell.raw, raw);
      assert.ok(cell.error);
      assert.equal(cell.value, null);
    }
    if (field.kind === 'integer') {
      for (const raw of ['32768', '-32769', '.5']) assert.ok(ordinaryNumberValue(field, raw, null).error);
      assert.equal(ordinaryNumberValue(field, '65535', 65535).error, null);
    } else {
      assert.equal(ordinaryNumberValue(field, '1.1234567890123', null).value, 1.1234567890123);
      assert.ok(ordinaryNumberValue(field, '3.5e38', null).error);
      assert.equal(ordinaryNumberValue(field, '3.5e38', 3.5e38).error, null);
    }
  }
});

test('Invalid numeric sessions survive remount and block their scope, then reset on Save or Undo identities', () => {
  const draft = values(), original = values();
  const field = ordinaryField('RootingDepth');
  const staged = { rootingDepth: ordinaryNumberValue(field, '1e', null) };
  rememberOrdinarySession(draft, original, staged);
  assert.equal(ordinarySession(draft, original).rootingDepth.raw, '1e');
  assert.ok(ordinaryValidation('soils', draft, original, staged));
  assert.equal(ordinaryValidation('site', draft, original, staged), null);
  assert.equal(Object.keys(ordinarySession(draft, values())).length, 0);
  assert.equal(Object.keys(ordinarySession({ ...draft }, original)).length, 0);
});

test('Actual scoped source controls render once with labels and explicit opt-out', () => {
  const source = readFileSync(path.join(__dirname, 'OrdinaryFields.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'OrdinaryFields.svelte', generate: 'client' }).warnings.length, 0);
  for (const enabled of [true, false]) {
    const Ordinary = serverComponent(source.replace('import.meta.env.VITE_ORDINARY_PARENT_EDITING',
      enabled ? 'undefined' : "'false'").replace('import.meta.env.VITE_PARENT_FLAGS_EDITING', "'false'")
      .replace('import.meta.env.VITE_ADDITIONAL_PARENT_EDITING', "'false'"),
      'OrdinaryFields.svelte', { './ordinaryEditor': editor });
    for (const scope of ['site', 'veg', 'soils']) {
      const fields = ordinaryFields.filter(field => field.scope === scope);
      const controls = fields.map(field => ({ column: field.column, type: 'TextBox', enabled: true,
        locked: false, controlName: field.column, controlId: field.column }));
      const fixture = `<script>import Ordinary from './OrdinaryFields.svelte'; let { draft, controls, scope, capabilities }=$props();</script>
        <Ordinary bind:draft original={null} {scope} {capabilities} disabled={false} onchange={()=>{}} onvalidation={()=>{}}>
          {#snippet children(slot)}
            {#each controls as control}
              {#if slot && slot.columns.includes(control.column)}{@render slot.input(control,'width:100%')}
              {:else}<input disabled data-column={control.column}/>{/if}
            {/each}
          {/snippet}
        </Ordinary>`;
      const Component = serverComponent(fixture, 'OrdinaryFixture.svelte', { './OrdinaryFields.svelte': { default: Ordinary } });
      const html = render(Component, { props: { draft: values(), controls, scope,
        capabilities: Object.fromEntries(fields.map(field => [field.key, true])) } }).body;
      for (const field of fields) {
        assert.equal((html.match(new RegExp(`data-column="${field.column}"`, 'g')) || []).length, 1);
        if (enabled) assert.match(html, new RegExp(`aria-label="${field.label}"`));
      }
      assert.equal((html.match(/ disabled/g) || []).length, enabled ? 0 : fields.length);
    }
  }
});

test('Shared lifecycle owns validation and VegNotes Tab targets source Terrain without writes', () => {
  const source = readFileSync(path.join(__dirname, 'OrdinaryFields.svelte'), 'utf8');
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(source, /rememberOrdinarySession\(draft, original, staged\)/);
  assert.match(source, /onvalidation\(`ordinary-\$\{scope\}`, message\)/);
  assert.match(source, /field\.key === 'vegNotes' && event\.key === 'Tab'/);
  assert.match(form, /activeTab = 'soils';\s*await tick\(\)/);
  assert.doesNotMatch(source, /PlotService|UpdatePlot|CreatePlot|Math\.fround|max="100"|min="0"|maxlength/);
});

test('Source X/Y Singles and Photo TEXT50 reuse raw-domain validation without geographic/path transformations', () => {
  const { presentationHelpers } = require('./svelteTestHelpers.cjs');
  const source = presentationHelpers().paper.paperPage('Site').controls;
  assert.equal(editor.additionalParentFields.length, 3);
  for (const field of editor.additionalParentFields) {
    const control = source.find(control => control.column === field.column);
    assert.equal(control.type, 'TextBox');
    assert.equal(control.locked, false);
    assert.equal(control.enabled, true);
    for (const event of ['BeforeUpdate', 'AfterUpdate', 'OnChange', 'OnKeyDown']) {
      assert.equal(control.properties[event], undefined);
    }
    assert.equal(ordinaryField(field.column).key, field.key);
    if (field.kind === 'single') {
      assert.equal(ordinaryNumberValue(field, '-123456.123456789', null).value, -123456.123456789);
      assert.ok(ordinaryNumberValue(field, '3.5e38', null).error);
      assert.equal(ordinaryNumberValue(field, '3.5e38', 3.5e38).error, null);
    } else {
      assert.equal(ordinaryTextError(field, "  Raw'O " + 'x'.repeat(42), null), null);
      assert.ok(ordinaryTextError(field, 'x'.repeat(51), null));
      assert.ok(ordinaryTextError(field, 'x'.repeat(49) + '😀', null));
      assert.ok(ordinaryTextError(field, '\ud800', null));
    }
  }
  const draft = { ...values(), xCoord: 3.5e38 };
  assert.ok(ordinaryValidation('site', draft, null, {}));
  assert.equal(ordinaryValidation('site', draft, null, {}, ordinaryFields), null);
});

test('Actual final scalar slot is independently gated and never enables the picture manager', () => {
  const source = readFileSync(path.join(__dirname, 'OrdinaryFields.svelte'), 'utf8');
  for (const enabled of [true, false]) {
    const Scalar = serverComponent(source
      .replace('import.meta.env.VITE_ORDINARY_PARENT_EDITING', "'false'")
      .replace('import.meta.env.VITE_PARENT_FLAGS_EDITING', "'false'")
      .replace('import.meta.env.VITE_ADDITIONAL_PARENT_EDITING', enabled ? "'true'" : "'false'"),
      'OrdinaryFields.svelte', { './ordinaryEditor': editor });
    const fixture = `<script>import Scalar from './Scalar.svelte';let {draft, controls, capabilities}=$props();</script>
      <Scalar bind:draft original={null} scope="site" {capabilities} disabled={false} onchange={()=>{}} onvalidation={()=>{}}>
        {#snippet children(slot)}
          {#each controls as control}
            {#if slot && slot.columns.includes(control.column)}{@render slot.input(control,'width:100%')}
            {:else}<input disabled data-column={control.column}/>{/if}
          {/each}
        {/snippet}
      </Scalar>`;
    const Wrapper = serverComponent(fixture, 'ScalarFixture.svelte', { './Scalar.svelte': { default: Scalar } });
    const fields = editor.additionalParentFields;
    const html = render(Wrapper, { props: { draft: values(),
      controls: fields.map(field => ({ column: field.column, type: 'TextBox', enabled: true, locked: false, controlName: field.column })),
      capabilities: Object.fromEntries(fields.map(field => [field.key, true])) } }).body;
    assert.equal((html.match(/data-column=/g) || []).length, 3);
    assert.equal((html.match(/ disabled/g) || []).length, enabled ? 0 : 3);
    assert.doesNotMatch(html, /btnManagePictures|btnPlotPicture/);
  }
});

test('Two separately gated source flags retain nullable identity and labelled CheckBox ownership', () => {
  assert.equal(editor.nullableParentFlags.length, 2);
  for (const field of editor.nullableParentFlags) {
    assert.equal(editor.ordinaryField(field.column), undefined);
    assert.equal(editor.nullableParentField(field.column).key, field.key);
  }
  const source = readFileSync(path.join(__dirname, 'OrdinaryFields.svelte'), 'utf8');
  assert.match(source, /VITE_PARENT_FLAGS_EDITING !== 'false'/);
  assert.match(source, /indeterminate=\{draft\[field.key\] === null\}/);
  assert.match(source, /draft\[field.key\] = event\.currentTarget\.checked/);
  assert.match(source, /draft\[field.key\] = null; onchange\(\)/);
  const Flags = serverComponent(source.replace('import.meta.env.VITE_ORDINARY_PARENT_EDITING', "'false'")
    .replace('import.meta.env.VITE_PARENT_FLAGS_EDITING', "'true'")
    .replace('import.meta.env.VITE_ADDITIONAL_PARENT_EDITING', "'false'"), 'OrdinaryFields.svelte', { './ordinaryEditor': editor });
  const fixture = `<script>import Flags from './OrdinaryFields.svelte';let {draft, field}=$props();</script>
    <Flags bind:draft original={null} scope={field.scope} capabilities={{[field.key]:true}} disabled={false} onchange={()=>{}} onvalidation={()=>{}}>
      {#snippet children(slot)}
        {#if slot}{@render slot.input({column:field.column,type:'CheckBox',enabled:true,locked:false,controlName:field.column},'width:100%')}{/if}
      {/snippet}
    </Flags>`;
  const Component = serverComponent(fixture, 'FlagFixture.svelte', { './OrdinaryFields.svelte': { default: Flags } });
  for (const field of editor.nullableParentFlags) {
    for (const value of [null, false, true]) {
      const html = render(Component, { props: { draft: { ...values(), [field.key]: value }, field } }).body;
      assert.equal((html.match(/type="checkbox"/g) || []).length, 1);
      assert.match(html, new RegExp(`id="header-${field.key}"`));
      assert.match(html, new RegExp(`aria-label="${field.label}"`));
      assert.match(html, new RegExp(value === null ? 'Not recorded \\(NULL\\)' : value ? 'Yes' : 'No'));
      assert.match(html, new RegExp(`aria-label="Clear ${field.label} to NULL"`));
    }
  }
});

const drainage = loadTypeScript('drainageEditor.ts', {
  './referenceCodeEditor': loadTypeScript('referenceCodeEditor.ts', { './becEditor': bec, './qualityEditor': quality })
});
test('Soil drainage requires exact membership and availability, never acknowledgement or case rewriting', () => {
  const rows = [
    { rowId: '1', listName: 'SoilDrainage', code: 'w', selectable: true },
    { rowId: '2', listName: 'SoilDrainage', code: 'w', selectable: true },
    { rowId: '3', listName: 'SoilDrainage', code: '', selectable: false },
    { rowId: '4', listName: 'SoilDrainage', code: 'p', selectable: false },
    { rowId: '5', listName: 'Other', code: 'r', selectable: true }
  ];
  const view = { choices: rows, ready: true, busy: false, error: null };
  assert.equal(drainage.drainageError('w', null, view), null);
  for (const raw of ['W', ' w', 'w ', 'p', 'r', '?????', 'abcdef', '', '\ud800']) {
    assert.ok(drainage.drainageError(raw, null, view));
  }
  assert.equal(drainage.drainageGroups(rows, 'SoilDrainage').length, 1);
  assert.equal(drainage.drainageGroups(rows, 'SoilDrainage')[0].records.length, 2);
  const unavailable = { ...view, ready: false, error: 'reference failure' };
  assert.equal(drainage.drainageError(null, 'legacy-long', unavailable), null);
  assert.equal(drainage.drainageError('legacy-long', 'legacy-long', unavailable), null);
  assert.equal(drainage.drainageError('w', null, unavailable), 'reference failure');
  assert.match(drainage.drainageError('w', null, { ...view, busy: true }), /Wait/);
});

test('Actual drainage component owns only its strict source slot and retains explicit opt-out', () => {
  const source = readFileSync(path.join(__dirname, 'DrainageFields.svelte'), 'utf8');
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'DrainageFields.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /requests\.track\(ParentCodeService\.ListChoices\('SoilDrainage'\)\)/);
  assert.match(source, /onDestroy[\s\S]*lookup\.dispose\(\)[\s\S]*onvalidation\('soilDrainage'/);
  assert.match(form, /parentCodeBusy \|\| drainageBusy/);
  assert.doesNotMatch(source, /type="checkbox"|rememberAcknowledgement|maxlength=/);
  for (const enabled of [true, false]) {
    const Component = serverComponent(source.replace('import.meta.env.VITE_SOIL_DRAINAGE_EDITING',
      enabled ? "'true'" : "'false'"), 'DrainageFields.svelte', {
      './drainageEditor': drainage, './qualityEditor': quality,
      '../bindings/github.com/boostao/vpro-wails': { ParentCodeService: {} }
    });
    const fixture = `<script>import Drainage from './Drainage.svelte';let draft={soilDrainage:'legacy-long'};</script>
      <label>Drainage class
        <Drainage bind:draft original={{soilDrainage:'legacy-long'}} capabilities={{soilDrainage:true}} disabled={false}
          onchange={()=>{}} onvalidation={()=>{}} onbusy={()=>{}}>
          {#snippet children(slot)}
            {#if slot}{@render slot.input({column:'SoilDrainage',type:'ComboBox',enabled:true,locked:false,controlName:'SoilDrainage'},'width:100%')}
            {:else}<input disabled data-column="SoilDrainage"/>{/if}
          {/snippet}
        </Drainage>
      </label>`;
    const Wrapper = serverComponent(fixture, 'DrainageFixture.svelte', { './Drainage.svelte': { default: Component } });
    const html = render(Wrapper).body;
    assert.equal((html.match(/data-column="SoilDrainage"/g) || []).length, 1);
    if (enabled) {
      assert.match(html, /id="header-soilDrainage"/);
      assert.match(html, /value="legacy-long"/);
      assert.doesNotMatch(html, /aria-invalid="true"/);
    } else assert.match(html, /disabled(?:="")? data-column="SoilDrainage"/);
  }
});
