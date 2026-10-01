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
const values = () => Object.fromEntries(ordinaryFields.map(field => [field.key, null]));

test('Fourteen fields retain source scopes and keep seven distinct workflows unavailable', () => {
  assert.equal(ordinaryFields.length, 14);
  assert.equal(ordinaryFields.filter(field => field.scope === 'site').length, 2);
  assert.equal(ordinaryFields.filter(field => field.scope === 'veg').length, 6);
  assert.equal(ordinaryFields.filter(field => field.scope === 'soils').length, 6);
  for (const column of ['BECSiteUnit', 'Photo', 'XCoord', 'YCoord', 'SpeciesListComplete', 'UpdatedFromCards', 'SoilDrainage']) {
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
      enabled ? 'undefined' : "'false'").replace('import.meta.env.VITE_PARENT_FLAGS_EDITING', "'false'"),
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
    .replace('import.meta.env.VITE_PARENT_FLAGS_EDITING', "'true'"), 'OrdinaryFields.svelte', { './ordinaryEditor': editor });
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
