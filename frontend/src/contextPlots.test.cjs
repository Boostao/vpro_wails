const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const read = name => readFileSync(path.join(__dirname, name), 'utf8');
const output = {};
vm.runInNewContext(ts.transpileModule(read('contextPlots.ts'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText, { exports: output, require: () => ({ ContextService: {} }) });

test('every editor read and mutation carries the original context identity and unchanged payload', async () => {
  const calls = [];
  const port = new Proxy({}, { get: (_, method) => (...args) => {
    calls.push([method, ...args]);
    return Promise.resolve('result');
  } });
  const client = output.bindContextPlots('original-context', port, port);
  for (const [name, invoke] of Object.entries(client)) {
    const payload = { name, raw: "don't trim  " };
    assert.equal(await invoke(payload, ['row'], 'retain'), 'result');
    const call = calls.at(-1);
    assert.equal(call[0], name === 'PreviewProjectProfileSpeciesPresence' ? 'Preview' : name);
    assert.equal(call[1], 'original-context');
    if (name === 'PreviewProjectProfileSpeciesPresence') {
      assert.deepEqual(JSON.parse(call[2]), payload);
      continue;
    }
    if (!['GetHeaderCapabilities', 'CanEditMasterBEC', 'ListSoilSuggestions', 'ListVegetationAttributeSuggestions', 'ReviewSpeciesCodes', 'ListProjectMetadataFields', 'ReviewProjectPlotProfile', 'ReviewProjectPlotProfileLump', 'ListProjectPlotProfileChoices', 'ReviewEnvironmentSiteUnits', 'ReviewSiteUnitEnvironment'].includes(name)) assert.equal(call[2], payload);
  }
  assert.equal(calls.length, 66);
  assert.throws(() => output.bindContextPlots('', port), /loaded project context identity/);
});

test('context transitions reuse native close validation and preserve drafts until publication succeeds', () => {
  const root = read('App.svelte');
  const form = read('FS882Form.svelte');
  for (const file of ['App.svelte', 'FS882Form.svelte', 'CloseConfirm.svelte']) {
    assert.equal(compile(read(file), { filename: file, generate: 'client' }).warnings.length, 0);
  }
  assert.doesNotMatch(root, /window\.confirm/);
  assert.match(root, /document\.activeElement\.blur\(\)/);
  assert.match(root, /await tick\(\)/);
  assert.match(root, /pendingTransition = \{ contextId, run \}/);
  assert.match(root, /await editor\?\.saveForClose\(\)/);
  assert.match(root, /const state = await ContextService\.SwitchContext[\s\S]*?projectState\.set\(state\)/);
  assert.match(root, /\{#key \$projectState\?\.contextId\}/);
  assert.match(form, /bindContextPlots\(untrack\(\(\) => contextId\)\)/);
  assert.match(form, /onDestroy\(\(\) => \{[\s\S]*?loadRequest\+\+;[\s\S]*?reads\.cancelAll\(\)/);
});

test('verified external attachment defaults on and project choices visibly identify their files', () => {
  const root = read('App.svelte');
  assert.match(root, /VITE_EXTERNAL_PROJECTS !== 'false'/);
  assert.match(root, /class="project-location"/);
  assert.match(root, /item\.name === project\.name/);
  assert.match(root, /\? project\.file : project\.path/);
});

test('actual editor service merge scopes nullable Audit and rejects a delayed read after context switching', async () => {
  const calls = [];
  let active = 'original-context', release;
  const held = new Promise(resolve => release = resolve);
  const scoped = output.bindContextPlots('original-context', {
    ListAuditReadEntries(contextId, plot) {
      calls.push(['scoped', contextId, plot]);
      return held.then(() => {
        if (contextId !== active) throw new Error('stale context; no history returned');
        return [];
      });
    },
  });
  const merged = { ListAuditReadEntries(plot) { calls.push(['legacy', plot]); return Promise.resolve([]); }, ...scoped };
  const pending = merged.ListAuditReadEntries(' literal plot ');
  active = 'next-context'; release();
  await assert.rejects(pending, /stale context/);
  assert.deepEqual(calls, [['scoped', 'original-context', ' literal plot ']]);
  const root = read('FS882Form.svelte');
  assert.match(root, /const PlotService = \{ \.\.\.LegacyPlotService, \.\.\.bindContextPlots\(untrack\(\(\) => contextId\)\) \}/);
  assert.match(root, /reads\.track\(PlotService\.ListAuditReadEntries\(p\)\)/);
});
