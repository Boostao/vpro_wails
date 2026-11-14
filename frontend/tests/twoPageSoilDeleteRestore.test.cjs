const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, srcPath } = require('./svelteTestHelpers.cjs');
const { editor, TwoPageSoilSession, fixture, original, cell } = require('./twoPageSoilTestHelpers.cjs');
const { metadata, transport, restoration } = require('./siviParentTestHelpers.cjs');

const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const soilReferences = loadTypeScript('twoPageSoilReferences.ts', {
  './siviParentTransport': transport,
  './projectMetadataRestore': restoration,
  './qualityEditor': quality,
  './readRequests': loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} }),
});
const soil = loadTypeScript('soilChildEditor.ts', {
  './qualityEditor': quality,
  './numericEditor': loadTypeScript('numericEditor.ts'),
});
const presentation = loadTypeScript('twoPageSoilPresentation.ts', {
  './projectMetadataEditor': metadata,
  './twoPageSoilEditor': editor,
  './soilChildEditor': soil,
});
const navigation = loadTypeScript('enterNavigation.ts');
const read = name => readFileSync(srcPath(name), 'utf8');

const bindings = {
  Humus: [
    'Horizon', 'UpperDepth', 'LowerDepth', 'HumusStructureDegree', 'HumusStructureKind',
    'MycelAbundance', 'FecalAbundance', 'RootsAbundance', 'RootsSize', 'vonPost', 'HumusFormpH', 'Comment',
  ],
  Mineral: [
    'Horizon', 'UpperDepth', 'LowerDepth', 'PitDepthLimit', 'Texture', 'Colour', 'ASP',
    'PercentCoarseFragsGravel', 'PercentCoarseFragsCobbles', 'PercentCoarseFragsStones', 'PercentCoarseFragsTotal',
    'PercentCoarseFragsShape', 'RootsAbundance', 'RootsSize', 'MineralStructureClass', 'MineralStructureKind',
    'MineralFormpH', 'Comments',
  ],
};

function completeSource(kind = 'Humus') {
  const source = original();
  source.form = `Soil${kind}XL`;
  source.table = `Sample_${kind}`;
  source.recordSource = `Usys${kind}`;
  for (const name of bindings[kind]) {
    if (source.records.columns.some(c => c.name === name)) continue;
    source.records.columns.push({ name, declaredType: '' });
    for (const row of source.records.rows) row.cells.push(cell());
  }
  return source;
}

const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });

test('TwoPageSoilPanel SSR renders guarded Delete and Deleted horizons actions when enabled', async () => {
  for (const kind of ['Humus', 'Mineral']) {
    const source = completeSource(kind);
    const owner = { ...fixture.owner, form: source.form };
    const session = new TwoPageSoilSession(
      owner,
      {
        read: () => operation(source),
        save: () => { throw new Error('unexpected save'); },
        outcome: () => { throw new Error('unexpected outcome'); },
      },
      true,
      true
    );
    await session.load();

    const sourceChoices = serverComponent(read('SoilReferenceChoices.svelte'), 'SoilReferenceChoices.svelte', {
      './projectMetadataEditor': metadata,
    });
    const Component = serverComponent(read('TwoPageSoilPanel.svelte'), 'TwoPageSoilPanel.svelte', {
      '../bindings/github.com/boostao/vpro-wails': {
        PlotEntryService: { GetTwoPageSoilReferences() { throw new Error('unexpected'); } },
      },
      './twoPageSoilReferences': soilReferences,
      './SoilReferenceChoices.svelte': { default: sourceChoices },
      './twoPageSoilSessions': {
        twoPageSoilSession: () => session,
        twoPageSoilWritingEnabled: true,
        twoPageSoilCreationEnabled: true,
      },
      './twoPageSoilEditor': editor,
      './projectMetadataEditor': metadata,
      './soilChildEditor': soil,
      './twoPageSoilPresentation': presentation,
      './enterNavigation': navigation,
    });

    const html = render(Component, { props: { ...owner, parentBlocked: false } }).body;
    assert.match(html, /data-soil-delete-trigger/);
    assert.match(html, /data-soil-deleted-history-trigger/);
    assert.match(html, /LMH25 pedon horizon editor/);
  }
});

test('TwoPageSoilPanel client guarded delete and restore enforce clean draft state, detach proxies, and reload session', async () => {
  const client = await import('svelte/internal/client');
  let currentSource = completeSource('Humus');
  const owner = { ...fixture.owner, form: 'SoilHumusXL' };
  let readCount = 0;
  const session = new TwoPageSoilSession(
    owner,
    {
      read: () => {
        readCount++;
        return operation(structuredClone(currentSource));
      },
      save: () => { throw new Error('unexpected save'); },
      outcome: () => { throw new Error('unexpected outcome'); },
    },
    true,
    true
  );
  await session.load();

  let script = read('TwoPageSoilPanel.svelte').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, parentBlocked }', 'form, parentBlocked, inspect }');
  script += `\n$effect(() => inspect({
    row,
    error,
    stage,
    operation,
    beginDelete() { deleteConfirmRowId = row?.rowId ?? null; },
    confirmDeleteHorizon,
    refreshDeletedHorizons,
    restoreDeletedHorizon,
    getDeletedState: () => ({ deleteConfirmRowId, deletedHistoryOpen, deletedHorizons }),
  }));`;

  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('soilDeleteRestore.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements
    .filter(n => !ts.isImportDeclaration(n))
    .map(n => n.getText(ast))
    .join('\n')
    .replace('export default function', 'function');

  let uuidSeq = 0;
  const mountFn = new Function(
    '$',
    'tick',
    'untrack',
    'twoPageSoilSession',
    'twoPageSoilWritingEnabled',
    'twoPageSoilKind',
    'metadataCellText',
    'equalCell',
    'soilFields',
    'twoPageSoilGroups',
    'twoPageSoilCell',
    'twoPageSoilStoredText',
    'twoPageSoilRowDiagnostic',
    'twoPageSoilDepthOrder',
    'enterNavigationMode',
    'document',
    'HTMLInputElement',
    'HTMLTextAreaElement',
    'crypto',
    'SoilReferenceLookup',
    'PlotEntryService',
    code + '\nreturn _unknown_;'
  )(
    client,
    async () => client.flush(),
    client.untrack,
    () => session,
    true,
    editor.twoPageSoilKind,
    metadata.metadataCellText,
    metadata.equalCell,
    soil.soilFields,
    presentation.twoPageSoilGroups,
    presentation.twoPageSoilCell,
    presentation.twoPageSoilStoredText,
    presentation.twoPageSoilRowDiagnostic,
    presentation.twoPageSoilDepthOrder,
    navigation.enterNavigationMode,
    undefined,
    undefined,
    undefined,
    { randomUUID: () => `33333333-4444-4555-8666-77777777770${++uuidSeq}` },
    soilReferences.SoilReferenceLookup,
    { GetTwoPageSoilReferences() { throw new Error('unexpected'); } }
  );

  const deletedLedger = [];
  const deleteCalls = [];
  const restoreCalls = [];
  const deletedRowSnapshot = structuredClone(currentSource.records.rows[0]);

  const mockDeleteRestorePort = {
    async deleteHorizon(cid, req) {
      assert.equal(cid, owner.contextId);
      assert.doesNotThrow(() => structuredClone(req));
      deleteCalls.push(req);
      currentSource.records.rows = currentSource.records.rows.filter(r => r.rowId !== req.rowId);
      deletedLedger.push({
        deleteRequestId: req.requestId,
        plotNumber: req.plotNumber,
        form: req.form,
        kind: 'Humus',
        deletedId: req.id,
        horizon: 'Lv',
        deletedBy: 'Analyst',
        deletedAt: '2026-11-12 10:00:00',
        restored: false,
      });
      return { requestId: req.requestId, deletedId: req.id, horizon: 'Lv' };
    },
    async restoreHorizon(cid, req) {
      assert.equal(cid, owner.contextId);
      assert.doesNotThrow(() => structuredClone(req));
      restoreCalls.push(req);
      const item = deletedLedger.find(d => d.deleteRequestId === req.deleteRequestId);
      assert.ok(item);
      item.restored = true;
      currentSource.records.rows = [structuredClone(deletedRowSnapshot), ...currentSource.records.rows];
      return {
        requestId: req.requestId,
        restoredId: item.deletedId,
        restoredRowId: deletedRowSnapshot.rowId,
        horizon: item.horizon,
      };
    },
    async listDeletedHorizons() {
      return structuredClone(deletedLedger);
    },
  };

  let api;
  const dispose = client.effect_root(() =>
    mountFn(null, {
      ...owner,
      parentBlocked: false,
      soilDeleteRestorePort: mockDeleteRestorePort,
      inspect: v => {
        api = v;
      },
    })
  );
  client.flush();

  try {
    // 1. Cannot delete without explicit confirmation
    await api.confirmDeleteHorizon();
    client.flush();
    assert.equal(deleteCalls.length, 0);
    assert.match(api.error, /confirm the selected physical row/);

    // 2. Unsaved draft blocks deletion
    api.beginDelete();
    api.stage('Horizon', 'Fm', false);
    client.flush();
    await api.confirmDeleteHorizon();
    client.flush();
    assert.equal(deleteCalls.length, 0);
    await api.operation('undo');
    client.flush();

    // 3. Confirmed delete dispatches detached request and reloads soil original
    api.beginDelete();
    client.flush();
    const readsBefore = readCount;
    await api.confirmDeleteHorizon();
    client.flush();
    assert.equal(deleteCalls.length, 1);
    assert.equal(readCount, readsBefore + 1);

    // 4. Refresh deleted horizons & restore horizon
    await api.refreshDeletedHorizons();
    client.flush();
    assert.equal(api.getDeletedState().deletedHorizons.length, 1);
    assert.equal(api.getDeletedState().deletedHorizons[0].restored, false);

    const delId = api.getDeletedState().deletedHorizons[0].deleteRequestId;
    await api.restoreDeletedHorizon(delId);
    client.flush();
    assert.equal(restoreCalls.length, 1);
    assert.equal(api.getDeletedState().deletedHorizons[0].restored, true);
  } finally {
    dispose();
  }
});
