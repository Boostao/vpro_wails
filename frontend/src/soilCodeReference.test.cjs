const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');

const source = readFileSync(path.join(__dirname, 'SoilCodeReference.svelte'), 'utf8');
const row = (listName, note) => ({
  rowId: '1', listName, code: '', listFilter: null, itemOrder: 0, description: 'Empty reference',
  fieldUsedIn: null, validateLoops: null, validate: false, note, flag: true,
  selectable: false, diagnostic: 'Empty Item is not selectable'
});
function component(enabled) {
  return serverComponent(source.replace("import.meta.env.VITE_SOIL_CODES_REFERENCE === 'true'", String(enabled)),
    'SoilCodeReference.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {
        SoilCodeService: {
          ListGreatGroupChoices: () => { throw new Error('SSR must not invoke native methods'); },
          ListSubgroupChoices: () => { throw new Error('SSR must not invoke native methods'); }
        }
      },
      './qualityEditor': {
        QualityLookup: class {
          constructor(loader, onchange, label) { this.label = label; }
          snapshot() {
            const group = this.label === 'Soil great group';
            return { ready: true, busy: false, error: null,
              choices: [row(group ? 'SoilClassGroup' : 'SoilClassSubgroup', group ? null : '')] };
          }
          dispose() {}
        }
      }
    });
}

test('Soil reference compiles, remains opt-in and cannot mutate a draft or source record', () => {
  assert.equal(compile(source, { filename: 'SoilCodeReference.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_SOIL_CODES_REFERENCE === 'true'/);
  assert.match(source, /ListGreatGroupChoices/);
  assert.match(source, /ListSubgroupChoices/);
  assert.match(source, /Retry \{label\} reference/);
  assert.doesNotMatch(source, /<input|bind:|PlotService|UpdatePlot|CreatePlot|onchange=|oninput=/);
  assert.doesNotMatch(render(component(false)).body, /soil-code-reference|data-reference-row=/);
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /<SoilCodeReference disabled=\{busy \|\| headerWorkflowBusy\}/);
});

test('Readonly reference retains empty Items, distinct NULL/empty metadata and exact list casing', () => {
  const html = render(component(true)).body;
  assert.match(html, /data-reference-list="SoilClassGroup"/);
  assert.match(html, /data-reference-list="SoilClassSubgroup"/);
  assert.equal((html.match(/data-property="note"/g) ?? []).length, 2);
  assert.match(html, /data-property="note" data-value="null" data-kind="null"/);
  assert.match(html, /data-property="note" data-value="&quot;&quot;" data-kind="string" data-empty="true"/);
  assert.equal((html.match(/data-property="code" data-value="&quot;&quot;"/g) ?? []).length, 2);
  assert.match(html, /Empty Item is not selectable/);
  assert.doesNotMatch(html, /<input/);
});
