const assert = require('node:assert/strict');
const { test } = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const compiler = require('svelte/compiler');
const runtime = require('svelte/internal/client');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const fields = loadTypeScript('summaryVegetationFieldList.ts');
const vegetation = loadTypeScript('summaryVegetation.ts', { './lifeformSummary': lifeform }, { setTimeout });
const client = loadTypeScript('summaryVegetationEnvironmental.ts', {
  './lifeformSummary': lifeform, './summaryVegetationFieldList': fields, './summaryVegetation': vegetation,
  './summaryVegetationEnvironmentalNumeric': loadTypeScript('summaryVegetationEnvironmentalNumeric.ts', { './summaryVegetationFieldList': fields }),
}, { setTimeout });
const source = fs.readFileSync(path.join(__dirname, 'SummaryVegetationEnvironmental.svelte'), 'utf8');
const wire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalWire.json'), 'utf8'));
const numericWire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalNumericPreviewWire.json'), 'utf8'));
const lifeformWire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalLifeformPreviewWire.json'), 'utf8'));
const owner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, wire.vegetation[key]]));
const deferred = () => {
  let resolve;
  const promise = new Promise(yes => { resolve = yes; });
  return { promise, resolve };
};
function execute(code, dependencies, globals = {}) {
  const module = { exports: {} };
  const js = ts.transpileModule(code.replaceAll('import.meta.env', '__environment'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS },
  }).outputText;
  vm.runInNewContext(js, { ...globals, module, exports: module.exports,
    require(id) { assert.ok(id in dependencies, `Uninjected import ${id}`); return dependencies[id]; } });
  return module.exports;
}
function mount(session, port, held = false, numeric = false, lifeform = false) {
  let destroy;
  const notifications = [];
  let script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1].replace('$props()', '__props');
  script += '\nexport const handlers = {preview, cancel, environmentalText, view: () => view, held: (value: boolean) => { operationHeld = value; }};';
  const stripped = ts.transpileModule(script, { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext,
  } }).outputText;
  const compiled = compiler.compileModule(stripped, { filename: 'environment-validation.svelte.js', generate: 'client' });
  const component = execute(compiled.js.code, {
    'svelte/internal/client': runtime,
    svelte: { onDestroy: callback => { destroy = callback; }, untrack: runtime.untrack },
  }, { __props: { session, port, numeric, lifeform, operationHeld: held, onBusyChange: busy => notifications.push(busy) } });
  return { ...component.handlers, notifications, destroy: () => destroy() };
}

test('full component compiles and actual client script accepts immutable Go wire proxy', async () => {
  assert.deepEqual(compiler.compile(source, { filename: 'SummaryVegetationEnvironmental.svelte', generate: 'client' }).warnings, []);
  const session = new client.SummaryEnvironmentalSession(owner);
  const pane = mount(session, { preview: () => Promise.resolve(runtime.proxy(structuredClone(wire))), validationYield: () => Promise.resolve() });
  await pane.preview();
  assert.equal(pane.view().result.rows.length, 25);
  assert.equal(Object.isFrozen(pane.view().result), true);
  assert.equal(pane.notifications.includes(true), true);
  assert.equal(pane.notifications.at(-1), false);
  pane.destroy();
});

test('shared numeric pane preserves explicit Empty, literal strings/counts and proxy results across remount', async () => {
  const numericOwner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, numericWire.vegetation[key]]));
  const registry = client.summaryEnvironmentalNumericRegistry(true), session = registry.session(numericOwner);
  let reads = 0;
  const port = { preview: () => { reads++; return Promise.resolve(runtime.proxy(structuredClone(numericWire))); },
    validationYield: () => Promise.resolve() };
  const pane = mount(session, port, false, true);
  await pane.preview();
  assert.equal(pane.view().result.rows.length, 17);
  const slope = pane.view().result.rows.find(row => row.field.cells[5].text === 'SVSlope');
  assert.equal(pane.environmentalText(slope.values[0]), '1.25 - 3 - 5.75');
  assert.equal(pane.environmentalText(slope.values[1]), 'Empty (no numeric value)');
  assert.equal(pane.environmentalText({ kind: 'cell', cell: { storage: 'text', text: '' } }), '""');
  const count = pane.view().result.rows.find(row => row.field.cells[5].text === 'SVnPlots');
  assert.equal(pane.environmentalText(count.values[0]), '4');
  pane.destroy();
  const remounted = mount(registry.session(numericOwner), port, false, true);
  assert.equal(remounted.view().result.rows.length, 17);
  assert.equal(reads, 1);
  remounted.destroy();
});

test('numeric pane teardown keeps global validation hold and error until settlement/retry', async () => {
  const numericOwner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, numericWire.vegetation[key]]));
  const registry = client.summaryEnvironmentalNumericRegistry(true), session = registry.session(numericOwner);
  const reached = deferred(), held = deferred();
  const pane = mount(session, { preview: () => Promise.resolve(numericWire),
    validationYield: () => { reached.resolve(); return held.promise; } }, false, true);
  const operation = pane.preview();
  await reached.promise;
  pane.destroy();
  assert.equal(registry.activity().busy, true);
  const next = mount(session, { preview: () => Promise.resolve(numericWire), validationYield: () => Promise.resolve() }, false, true);
  assert.match(next.view().error, /cancelled/);
  held.resolve(); await operation;
  assert.equal(next.view().result, null);
  assert.equal(registry.activity().busy, false);
  await next.preview();
  assert.equal(next.view().result.rows.length, 17);
  next.destroy();
});

test('lifeform pane retains master provenance and String empty across actual proxy/remount', async () => {
  const owned = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, lifeformWire.vegetation[key]]));
  const registry = client.summaryEnvironmentalLifeformRegistry(true), session = registry.session(owned);
  let reads = 0;
  const port = { preview: () => { reads++; return Promise.resolve(runtime.proxy(structuredClone(lifeformWire))); },
    validationYield: () => Promise.resolve() };
  const pane = mount(session, port, false, true, true);
  await pane.preview();
  assert.equal(pane.view().result.rows.length, 30);
  assert.equal(pane.view().result.masterSourcePath, 'C:\\fixture\\VLists.db');
  const row = pane.view().result.rows.find(row => row.field.cells[5].text === 'SVLifeformCover' && row.field.cells[6].text === '3');
  assert.equal(pane.environmentalText(row.values[0]), '3.8 - 4.4 - 5.0');
  assert.equal(pane.environmentalText(row.values[1]), '""');
  const slope = pane.view().result.rows.find(row => row.field.cells[5].text === 'SVSlope');
  assert.equal(pane.environmentalText(slope.values[1]), 'Empty (no numeric value)');
  pane.destroy();
  const remounted = mount(registry.session(owned), port, false, true, true);
  assert.equal(remounted.view().result.rows.length, 30);
  assert.equal(reads, 1);
  remounted.destroy();
});

test('lifeform pane cancellation on teardown keeps registry hold until actual validation settles', async () => {
  const owned = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, lifeformWire.vegetation[key]]));
  const registry = client.summaryEnvironmentalLifeformRegistry(true), session = registry.session(owned);
  const entered = deferred(), held = deferred();
  const pane = mount(session, { preview: () => Promise.resolve(lifeformWire),
    validationYield: () => { entered.resolve(); return held.promise; } }, false, true, true);
  const operation = pane.preview();
  await entered.promise;
  pane.destroy();
  const next = mount(registry.session(owned), { preview: () => Promise.resolve(lifeformWire),
    validationYield: () => Promise.resolve() }, false, true, true);
  assert.equal(registry.activity().busy, true);
  assert.match(next.view().error, /cancelled/);
  held.resolve(); await operation;
  assert.equal(next.view().result, null);
  assert.equal(registry.activity().busy, false);
  await next.preview();
  assert.equal(next.view().result.rows.length, 30);
  next.destroy();
});

test('peer holds prevent dispatch but leave cancellation live', async () => {
  const session = new client.SummaryEnvironmentalSession(owner), held = deferred();
  let dispatches = 0, cancelled = 0;
  held.promise.cancel = () => { cancelled++; };
  const pane = mount(session, { preview: () => { dispatches++; return held.promise; } }, true);
  await pane.preview();
  assert.equal(dispatches, 0);
  pane.held(false);
  const operation = pane.preview();
  pane.held(true);
  pane.cancel();
  assert.equal(cancelled, 1);
  held.resolve(wire);
  await operation;
  assert.equal(pane.view().result, null);
  assert.match(pane.view().error, /cancelled/);
  pane.destroy();
});

test('real script remount cancels held validation and global dispatch hold outlives component', async () => {
  const registry = client.summaryEnvironmentalRegistry(true), session = registry.session(owner);
  const held = deferred(), reached = deferred(), notifications = [];
  const unsubscribe = registry.subscribe(() => notifications.push(registry.activity().busy));
  const port = { preview: () => Promise.resolve(wire), validationYield: () => { reached.resolve(); return held.promise; } };
  const pane = mount(session, port), operation = pane.preview();
  await reached.promise;
  pane.destroy();
  assert.equal(registry.activity().busy, true);
  const remounted = mount(registry.session(owner), { preview: () => Promise.resolve(wire), validationYield: () => Promise.resolve() });
  assert.match(remounted.view().error, /cancelled/);
  held.resolve();
  await operation;
  assert.equal(remounted.view().result, null);
  assert.equal(registry.activity().busy, false);
  await remounted.preview();
  assert.equal(remounted.view().result.rows.length, 25);
  assert.equal(notifications.includes(true), true);
  remounted.destroy(); unsubscribe();
});

test('retained completed result survives remount without another dispatch or lost errors', async () => {
  const session = new client.SummaryEnvironmentalSession(owner);
  let reads = 0;
  const port = { preview: () => { reads++; return Promise.resolve(wire); }, validationYield: () => Promise.resolve() };
  const pane = mount(session, port);
  await pane.preview(); pane.destroy();
  const next = mount(session, port);
  assert.equal(next.view().result.rows.length, 25);
  assert.equal(reads, 1);
  next.destroy();
});

test('both literal client feature flags required and fixed wire API calls only one combined service', async () => {
  const desktop = fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalDesktop.ts'), 'utf8');
  for (const [base, environment, expected] of [['true', 'true', true], ['true', 'false', false],
    ['false', 'true', false], ['true', 'TRUE', false], ['', 'true', false]]) {
    const calls = [];
    const module = execute(desktop, {
      '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationEnvironmentalService: {
        PreviewEnvironmental: (...args) => { calls.push(args); return Promise.resolve(wire); },
      } },
      './summaryVegetationEnvironmental': client,
    }, { __environment: { VITE_SUMMARY_VEGETATION: base, VITE_SUMMARY_VEGETATION_ENVIRONMENT: environment }, setTimeout });
    assert.equal(module.summaryEnvironmentalEnabled, expected);
    if (expected) {
      await module.summaryEnvironmentalDesktop.session(owner).show(module.summaryEnvironmentalPort);
      assert.deepEqual(calls, [[owner.contextId, 1, 6, 0]]);
    } else assert.throws(() => module.summaryEnvironmentalDesktop.session(owner), /disabled/);
  }
});

test('independent numeric client flag selects one correct service and preserves base/environment gates', async () => {
  const desktop = fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalDesktop.ts'), 'utf8');
  const numericOwner = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, numericWire.vegetation[key]]));
  for (const base of ['true', 'false']) for (const environment of ['true', 'false'])
    for (const numeric of ['true', 'false', 'TRUE', '']) {
      const calls = [];
      const module = execute(desktop, {
        '../bindings/github.com/boostao/vpro-wails': { SummaryVegetationEnvironmentalService: {
          PreviewEnvironmental: (...args) => { calls.push(['legacy', ...args]); return Promise.resolve(wire); },
          PreviewEnvironmentalNumeric: (...args) => { calls.push(['numeric', ...args]); return Promise.resolve(numericWire); },
        } },
        './summaryVegetationEnvironmental': client,
      }, { __environment: { VITE_SUMMARY_VEGETATION: base, VITE_SUMMARY_VEGETATION_ENVIRONMENT: environment,
        VITE_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC: numeric }, setTimeout });
      const enabled = base === 'true' && environment === 'true', typed = enabled && numeric === 'true';
      assert.equal(module.summaryEnvironmentalEnabled, enabled);
      assert.equal(module.summaryEnvironmentalNumericEnabled, typed);
      if (enabled) {
        const selectedOwner = typed ? numericOwner : owner;
        const session = module.summaryEnvironmentalDesktop.session(selectedOwner);
        assert.equal(await session.show(module.summaryEnvironmentalPort), true);
        assert.deepEqual(calls, [[typed ? 'numeric' : 'legacy', selectedOwner.contextId, 1, 6, 0]]);
        assert.equal(session instanceof (typed ? client.SummaryEnvironmentalNumericSession : client.SummaryEnvironmentalSession), true);
      } else {
        assert.throws(() => module.summaryEnvironmentalDesktop.session(numericOwner), /disabled/);
        assert.deepEqual(calls, []);
      }
    }
});

test('literal lifeform client gate requires all three predecessors and selects exactly one matching registry/service', async () => {
  for (const base of ['false', 'true']) for (const environment of ['false', 'true']) for (const numeric of ['false', 'true'])
    for (const lifeform of ['', 'false', 'true', 'TRUE', '1', ' true']) {
      const calls = [];
      const module = execute(fs.readFileSync(path.join(__dirname, 'summaryVegetationEnvironmentalDesktop.ts'), 'utf8'), {
        '../bindings/github.com/boostao/vpro-wails': {
          SummaryVegetationEnvironmentalService: {
            PreviewEnvironmental: (...args) => { calls.push(['legacy', ...args]); return Promise.resolve(wire); },
            PreviewEnvironmentalNumeric: (...args) => { calls.push(['numeric', ...args]); return Promise.resolve(numericWire); },
          },
          SummaryVegetationEnvironmentalLifeformService: {
            PreviewEnvironmentalLifeform: (...args) => { calls.push(['lifeform', ...args]); return Promise.resolve(lifeformWire); },
          },
        },
        './summaryVegetationEnvironmental': client,
      }, { __environment: { VITE_SUMMARY_VEGETATION: base, VITE_SUMMARY_VEGETATION_ENVIRONMENT: environment,
        VITE_SUMMARY_VEGETATION_ENVIRONMENT_NUMERIC: numeric, VITE_SUMMARY_VEGETATION_ENVIRONMENT_LIFEFORM: lifeform }, setTimeout });
      const enabled = base === 'true' && environment === 'true', typed = enabled && numeric === 'true';
      const full = typed && lifeform === 'true';
      assert.equal(module.summaryEnvironmentalLifeformEnabled, full);
      if (!enabled) {
        assert.throws(() => module.summaryEnvironmentalDesktop.session(owner), /disabled/);
        assert.deepEqual(calls, []);
        continue;
      }
      const selected = full ? lifeformWire : typed ? numericWire : wire;
      const owned = Object.fromEntries(['contextId', 'project', 'projectPath', 'su', 'suPath'].map(key => [key, selected.vegetation[key]]));
      const session = module.summaryEnvironmentalDesktop.session(owned);
      assert.equal(session instanceof (full ? client.SummaryEnvironmentalLifeformSession :
        typed ? client.SummaryEnvironmentalNumericSession : client.SummaryEnvironmentalSession), true);
      assert.equal(await session.show(module.summaryEnvironmentalPort), true);
      assert.deepEqual(calls, [[full ? 'lifeform' : typed ? 'numeric' : 'legacy', owned.contextId, 1, 6, 0]]);
    }
});

test('actual root derived hold survives pane teardown until cancelled dispatch settles', async () => {
  const app = fs.readFileSync(path.join(__dirname, 'App.svelte'), 'utf8').replaceAll('\r\n', '\n');
  const effect = app.match(/  \$effect\(\(\) => \{\n    if \(!summaryEnvironmentalEnabled\)[\s\S]*?\n  \}\);/)[0];
  const derived = app.match(/  const editorBusy = \$derived\([^\n]+/)[0];
  const registry = client.summaryEnvironmentalRegistry(true);
  const compiled = compiler.compileModule(`
    let localEditorBusy = $state(false), pictureCreationBusy = false, pictureLifecycleBusy = false,
      twoPageSoilBusy = false, twoPageVegetationBusy = false, summaryPreferenceBusy = false,
      summaryFieldListBusy = false, projectReportBusy = false, externalSUEnvironmentBusy = false, matrixPublicationBusy = false;
    let summaryEnvironmentalBusy = $state(false), view = 'home';
    ${derived}
    ${effect.replace('$effect(', 'installEffect(')}
    export const handlers = { held: () => editorBusy, route: () => view,
      releaseLocal: () => { localEditorBusy = false; } };
  `, { filename: 'environment-root-holds.svelte.js', generate: 'client' });
  const root = execute(compiled.js.code, { 'svelte/internal/client': runtime }, {
    summaryEnvironmentalEnabled: true, summaryEnvironmentalDesktop: registry, installEffect: callback => callback(),
  }).handlers;
  const session = registry.session(owner), held = deferred();
  const pane = mount(session, { preview: () => held.promise });
  const operation = pane.preview();
  assert.equal(root.held(), true);
  assert.equal(root.route(), 'summary-vegetation');
  pane.destroy(); root.releaseLocal();
  assert.equal(root.held(), true);
  held.resolve(wire); await operation;
  assert.equal(root.held(), false);
});
