const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });

const wailsCalls = [];
const siviCreate = loadTypeScript('siviParentCreate.ts', {
  '@wailsio/runtime': {
    Call: {
      ByName(methodName, ...args) {
        wailsCalls.push({ methodName, args });
        return Promise.resolve([]);
      },
    },
  },
  './qualityEditor': quality,
});

const read = name => readFileSync(path.join(__dirname, name), 'utf8');

test('validateCreateSIVIPlotDraft enforces PlotNumber <= 7 UTF-16 code units, SIVI PlotType, and ISO Date', () => {
  assert.deepEqual(
    structuredClone(
      siviCreate.validateCreateSIVIPlotDraft({
        plotNumber: '00771',
        plotType: 'Ground',
        date: '2026-07-19',
        svPolygonNumber: 'POLY-14',
      })
    ),
    {}
  );

  // Empty, overlength ASCII, and 4 astral characters (= 8 UTF-16 code units > 7) must fail
  assert.match(siviCreate.validateCreateSIVIPlotDraft({ plotNumber: '', plotType: 'Ground' }).plotNumber, /required/);
  assert.match(siviCreate.validateCreateSIVIPlotDraft({ plotNumber: '12345678', plotType: 'Ground' }).plotNumber, /7 UTF-16/);
  assert.match(
    siviCreate.validateCreateSIVIPlotDraft({ plotNumber: '\u{1D538}\u{1D538}\u{1D538}\u{1D538}', plotType: 'Ground' }).plotNumber,
    /7 UTF-16/
  );
  // 3 astral characters + 1 ASCII (= 7 UTF-16 code units) passes
  assert.equal(
    siviCreate.validateCreateSIVIPlotDraft({ plotNumber: '\u{1D538}\u{1D538}\u{1D538}A', plotType: 'Visual' }).plotNumber,
    undefined
  );

  // Invalid PlotType & malformed surrogate & non-ISO date
  assert.match(
    siviCreate.validateCreateSIVIPlotDraft({ plotNumber: 'SV01', plotType: 'FS882' }).plotType,
    /Ground, Visual, Note, or Other/
  );
  assert.match(
    siviCreate.validateCreateSIVIPlotDraft({ plotNumber: 'SV\uD800', plotType: 'Note' }).plotNumber,
    /malformed Unicode/
  );
  assert.match(
    siviCreate.validateCreateSIVIPlotDraft({ plotNumber: 'SV01', plotType: 'Other', date: '07/19/2026' }).date,
    /YYYY-MM-DD/
  );
});

test('defaultSIVIParentCreatePort wires to ContextService Wails IPC methods', async () => {
  wailsCalls.length = 0;
  await siviCreate.defaultSIVIParentCreatePort.createPlot('ctx-sivi', { plotNumber: '00771', plotType: 'Ground' });
  await siviCreate.defaultSIVIParentCreatePort.undoCreatePlot('ctx-sivi', { creationRequestId: 'u1', plotNumber: '00771' });
  await siviCreate.defaultSIVIParentCreatePort.listCreatedPlots('ctx-sivi');
  assert.deepEqual(
    wailsCalls.map(c => c.methodName),
    [
      'main.ContextService.CreateSIVIPlot',
      'main.ContextService.UndoCreateSIVIPlot',
      'main.ContextService.ListCreatedSIVIPlots',
    ]
  );
});

test('SIVIParentCreateModal SSR and client mount create FS1333 SIVI plots, detach proxies, and execute guarded Undo', async () => {
  const Component = serverComponent(read('SIVIParentCreateModal.svelte'), 'SIVIParentCreateModal.svelte', {
    './siviParentCreate': siviCreate,
  });
  const ssrHtml = render(Component, {
    props: { contextId: 'ctx-sivi', activeProject: 'Sample', activeSu: 'Sample', open: true },
  }).body;
  assert.match(ssrHtml, /Create SIVI Parent Plot/);
  assert.match(ssrHtml, /Ground/);
  assert.match(ssrHtml, /Visual/);
  assert.match(ssrHtml, /Note/);
  assert.match(ssrHtml, /Other/);

  const client = await import('svelte/internal/client');
  let script = read('SIVIParentCreateModal.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('onclose,', 'onclose, inspect,');
  script += `\n$effect(() => inspect({
    setForm(p, t, poly, su) { plotNumber = p; plotType = t; svPolygonNumber = poly; addToActiveSu = su; },
    submitCreate,
    executeUndo,
    getState: () => ({ createdPlots, status, error, hasValidationErrors }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('siviModal.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements
    .filter(n => !ts.isImportDeclaration(n))
    .map(n => n.getText(ast))
    .join('\n')
    .replace('export default function', 'function');

  let seq = 0;
  const mountFn = new Function(
    '$',
    'onDestroy',
    'onMount',
    'defaultSIVIParentCreatePort',
    'SIVI_PLOT_TYPES',
    'validateCreateSIVIPlotDraft',
    'crypto',
    code + '\nreturn _unknown_;'
  );

  const history = [];
  let createdCallback = null;
  let undoneCallback = null;

  const mockPort = {
    async listCreatedPlots() {
      return structuredClone(history);
    },
    async createPlot(cid, req) {
      assert.equal(cid, 'ctx-sivi');
      assert.doesNotThrow(() => structuredClone(req));
      const entry = {
        requestId: req.requestId,
        plotNumber: req.plotNumber,
        plotType: req.plotType,
        svPolygonNumber: req.svPolygonNumber,
        createdAt: '2026-11-12 11:00:00',
        actor: 'Surveyor',
        undone: false,
        canUndo: true,
      };
      history.unshift(entry);
      return {
        requestId: req.requestId,
        contextId: cid,
        project: 'Sample',
        plotNumber: req.plotNumber,
        plotType: req.plotType,
        addedToSu: !!req.addToActiveSu,
        auditRowCount: 5,
        createdAt: entry.createdAt,
        actor: entry.actor,
        replayed: false,
      };
    },
    async undoCreatePlot(cid, req) {
      assert.equal(cid, 'ctx-sivi');
      assert.doesNotThrow(() => structuredClone(req));
      const found = history.find(h => h.requestId === req.creationRequestId);
      assert.ok(found);
      found.undone = true;
      found.undoneAt = '2026-11-12 11:05:00';
      found.canUndo = false;
      return {
        requestId: req.requestId,
        creationRequestId: req.creationRequestId,
        contextId: cid,
        project: 'Sample',
        plotNumber: req.plotNumber,
        removedFromSu: true,
        undoneAt: found.undoneAt,
        actor: 'Surveyor',
        replayed: false,
      };
    },
  };

  let api;
  const dispose = client.effect_root(() =>
    mountFn(
      client,
      () => {},
      fn => fn(),
      siviCreate.defaultSIVIParentCreatePort,
      siviCreate.SIVI_PLOT_TYPES,
      siviCreate.validateCreateSIVIPlotDraft,
      { randomUUID: () => `44444444-5555-4666-8777-88888888880${++seq}` }
    )(null, {
      contextId: 'ctx-sivi',
      activeProject: 'Sample',
      activeSu: 'Sample',
      open: true,
      port: mockPort,
      oncreated: r => {
        createdCallback = r;
      },
      onundone: p => {
        undoneCallback = p;
      },
      inspect: v => {
        api = v;
      },
    })
  );
  client.flush();

  try {
    assert.equal(api.getState().hasValidationErrors, true);
    api.setForm('00771', 'Ground', 'POLY-14', true);
    client.flush();
    assert.equal(api.getState().hasValidationErrors, false);

    await api.submitCreate();
    client.flush();
    assert.equal(createdCallback.plotNumber, '00771');
    assert.equal(createdCallback.addedToSu, true);
    assert.equal(api.getState().createdPlots.length, 1);

    await api.executeUndo(api.getState().createdPlots[0]);
    client.flush();
    assert.equal(undoneCallback, '00771');
    assert.equal(api.getState().createdPlots[0].undone, true);
  } finally {
    dispose();
  }
});
