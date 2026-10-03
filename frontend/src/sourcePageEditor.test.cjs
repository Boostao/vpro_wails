const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const { presentation } = presentationHelpers();

const source = readFileSync(path.join(__dirname, 'SourcePage.svelte'), 'utf8');
const control = (column, type, x) => ({
  controlId: column, controlName: column, column, type, caption: column,
  x, y: 84, width: 805 / 15, height: 18, fontName: 'Arial', fontSize: 10,
  bold: false, textAlign: 'center'
});
const page = {
  contentWidth: 600, height: 200,
  controls: [
    control('SoilClassSubGroup', 'ComboBox', 105),
    control('SoilClassGroup', 'ComboBox', 231),
    control('Unmigrated', 'TextBox', 350),
    { ...control('Child', 'Subform', 9), width: 300, height: 70 }
  ]
};

test('Source checkbox editors take ownership while unavailable checkbox partners stay disabled', () => {
  const checkboxPage = { ...page, controls: [control('SpeciesListComplete', 'CheckBox', 105),
    control('UnmigratedFlag', 'CheckBox', 231)] };
  const FlagsPage = serverComponent(source, 'SourcePage.svelte', {
    './paperLayout': { paperPage: () => checkboxPage, accessCaption: value => value },
    './formPresentation': presentation
  });
  const fixture = `<script>import FlagsPage from './FlagsPage.svelte';</script>
    {#snippet flag(control, position)}<input type="checkbox" data-live-flag={control.column} style={position}/>{/snippet}
    <FlagsPage name="Vegetation" editor={{columns:['SpeciesListComplete'],input:flag}}>
      {#snippet embedded(control)}{/snippet}
    </FlagsPage>`;
  const Component = serverComponent(fixture, 'FlagSourceFixture.svelte', { './FlagsPage.svelte': { default: FlagsPage } });
  const html = render(Component).body;
  assert.equal((html.match(/data-live-flag="SpeciesListComplete"/g) || []).length, 1);
  assert.match(html, /data-column="UnmigratedFlag"[^>]*disabled/);
  assert.doesNotMatch(html, /data-column="SpeciesListComplete"/);
});

test('Source Find Plot shares the explicit callback while unavailable and busy actions remain disabled', () => {
  const findPage = { ...page, controls: [{ ...control('btnFindPlot', 'CommandButton', 0), column: '', caption: 'Find Plot' }] };
  const FindPage = serverComponent(source, 'SourcePage.svelte', {
    './paperLayout': { paperPage: () => findPage, accessCaption: value => value },
    './formPresentation': presentation
  });
  for (const [props, disabled] of [[{}, true], [{ onFindPlot: () => {}, findPlotDisabled: true }, true],
    [{ onFindPlot: () => {}, findPlotDisabled: false }, false]]) {
    const html = render(FindPage, { props: { name: 'Vegetation', embedded: () => {}, ...props } }).body;
    const button = html.match(/<button\b[^>]*>Find Plot<\/button>/)?.[0];
    assert.ok(button);
    assert.equal(/\bdisabled\b/.test(button), disabled);
  }
});

const SourcePage = serverComponent(source, 'SourcePage.svelte', {
  './paperLayout': { paperPage: () => page, accessCaption: value => value },
  './formPresentation': presentation
});

function renderPage(withEditor, extra = '') {
  const fixture = `
    <script>
      import SourcePage from './SourcePage.svelte';
      const values = new Map([['soilclassgroup', 'BG'], ['unmigrated', 'Preserved']]);
    </script>
    {#snippet input(control, position)}
      <input data-editor={control.column} data-source-control={control.controlName} style={position} value="EDIT" />
    {/snippet}
    <SourcePage name="Soil/Terrain" {values}
      ${withEditor ? "editor={{ columns: ['SoilClassGroup', 'SoilClassSubGroup'], input }}" : ''} ${extra}>
      {#snippet embedded(control)}<span data-child={control.controlName}>Child</span>{/snippet}
    </SourcePage>`;
  const Component = serverComponent(fixture, 'SourcePageEditorFixture.svelte', {
    './SourcePage.svelte': { default: SourcePage }
  });
  return render(Component).body;
}

test('Independent editor slots compose without changing readonly partners or grouping', () => {
  const html = renderPage(true, "editors={[{ columns: ['Unmigrated'], input }]}");
  assert.equal((html.match(/data-editor=/g) ?? []).length, 3);
  assert.match(html, /data-child="Child"/);
  assert.doesNotMatch(html, /position:absolute|left:|top:|transform:scale/);
  assert.equal((renderPage(false, "editors={[{ columns: ['SoilClassGroup'], input }]}").match(/data-editor=/g) ?? []).length, 1);
});

test('Overlapping source-slot ownership fails explicitly instead of choosing an arbitrary editor', () => {
  assert.throws(() => renderPage(true, "editors={[{ columns: ['SoilClassGroup'], input }]}"),
    /Multiple source editors own SoilClassGroup/);
});

test('Default source rendering remains readonly with stored values and embedded children', () => {
  assert.equal(compile(source, { filename: 'SourcePage.svelte', generate: 'client' }).warnings.length, 0);
  const html = renderPage(false);
  const inputs = html.match(/<input\b[^>]*>/g);
  assert.equal(inputs.length, 3);
  assert.ok(inputs.every(input => /\bdisabled\b/.test(input)));
  assert.match(inputs.find(input => input.includes('data-column="SoilClassGroup"')), /value="BG"/);
  assert.match(inputs.find(input => input.includes('data-column="Unmigrated"')), /value="Preserved"/);
  assert.doesNotMatch(html, /data-editor=/);
  assert.match(html, /data-source-control="Child"/);
  assert.match(html, /data-child="Child"/);
});

test('Only supplied text/combo columns render readable labelled editor snippets within groups', () => {
  const html = renderPage(true);
  const inputs = html.match(/<input\b[^>]*>/g);
  assert.equal(inputs.length, 3);
  for (const [column, x] of [['SoilClassSubGroup', 105], ['SoilClassGroup', 231]]) {
    const input = inputs.find(input => input.includes(`data-editor="${column}"`));
    assert.ok(input);
    assert.match(input, /min-height:40px;font:inherit/);
    assert.doesNotMatch(input, /position:absolute|left:|top:|height:18px/);
    assert.match(input, new RegExp(`data-source-control="${column}"`));
    assert.doesNotMatch(input, /\bdisabled\b/);
  }
  const unmigrated = inputs.find(input => input.includes('data-column="Unmigrated"'));
  assert.match(unmigrated, /\bdisabled\b/);
  assert.match(unmigrated, /value="Preserved"/);
  assert.match(html, /data-child="Child"/);
  assert.match(html, /<legend>Soil classification<\/legend>/);
  assert.match(html, /<label class="form-field[^"]*"><span class="field-label">Soil great group<\/span>/);
});
