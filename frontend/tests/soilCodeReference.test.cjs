const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { serverComponent, srcPath } = require('./svelteTestHelpers.cjs');

const source = readFileSync(srcPath('SoilCodeReference.svelte'), 'utf8');
const row = (listName, note) => ({
  rowId: '1', listName, code: '', listFilter: null, itemOrder: 0, description: 'Empty reference',
  fieldUsedIn: null, validateLoops: null, validate: false, note, flag: true,
  selectable: false, diagnostic: 'Empty Item is not selectable'
});
function component() {
  return serverComponent(source,
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

test('Soil reference compiles and cannot mutate a draft or source record', () => {
  assert.equal(compile(source, { filename: 'SoilCodeReference.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /ListGreatGroupChoices/);
  assert.match(source, /ListSubgroupChoices/);
  assert.match(source, /Retry \{label\} reference/);
  assert.doesNotMatch(source, /<input|bind:|PlotService|UpdatePlot|CreatePlot|onchange=|oninput=/);
  const form = readFileSync(srcPath('FS882Form.svelte'), 'utf8');
  const soilSection = readFileSync(srcPath('FS882SoilHorizonSection.svelte'), 'utf8');
  assert.match(form, /<FS882SoilHorizonSection/);
  assert.match(soilSection, /<SoilCodeReference disabled=\{busy \|\| headerWorkflowBusy\}/);
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
