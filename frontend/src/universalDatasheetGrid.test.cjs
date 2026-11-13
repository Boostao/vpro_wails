const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const wailsStub = {
  Call: {
    ByName: () => Promise.resolve(null),
  },
};

const grid = loadTypeScript('universalDatasheetGrid.ts', {
  '@wailsio/runtime': wailsStub,
});
const plain = value => JSON.parse(JSON.stringify(value));

test('canonical 9 datasheet keys cover all required VPro project/SU/lump/hierarchy tables', () => {
  assert.deepEqual(plain(grid.CANONICAL_DATASHEET_KEYS), [
    'Env',
    'Admin',
    'Veg',
    'Humus',
    'Mineral',
    'Other',
    'SU',
    'Lump',
    'Hierarchy',
  ]);
});

test('computeVirtualWindow computes 60fps virtualized slice and top/bottom spacers accurately', () => {
  const empty = grid.computeVirtualWindow(0, 500, 34, 0, 6);
  assert.deepEqual(plain(empty), { startIndex: 0, endIndex: 0, topSpacerPx: 0, bottomSpacerPx: 0 });

  const mid = grid.computeVirtualWindow(3400, 340, 34, 1000, 5);
  assert.equal(mid.startIndex, 95);
  assert.equal(mid.endIndex, 115);
  assert.equal(mid.topSpacerPx, 95 * 34);
  assert.equal(mid.bottomSpacerPx, (1000 - 115) * 34);
});

test('parseAndValidateCellInput enforces float32 <100 narrowing, UTF-16 widths, Access BOOLEAN -1/0, and BECSiteUnit protection', () => {
  const coverCol = {
    name: 'Cover1',
    declaredType: 'SINGLE',
    kind: 'single',
    pinned: false,
    editable: true,
    protected: false,
    maxUtf16: 0,
    isCoverSingle: true,
    isBoolean: false,
  };

  // 99.999996 narrows to float32 99.99999237060547 (< 100) -> passes!
  const okCover = grid.parseAndValidateCellInput('Veg', coverCol, '99.999996', false);
  assert.equal(okCover.error, '');
  assert.equal(okCover.cell.storage, 'real');
  assert.equal(okCover.cell.real, Math.fround(99.999996));

  // 99.999999 narrows to float32 100 -> rejects!
  for (const bad of ['99.999999', '100', '105', '-1']) {
    const rejected = grid.parseAndValidateCellInput('Veg', coverCol, bad, false);
    assert.notEqual(rejected.error, '', `expected ${bad} to be rejected`);
    assert.equal(rejected.cell, null);
  }

  // UTF-16 width checks: Texture <= 4 (astral char \u{1D306} = 2 UTF-16 units)
  const textureCol = {
    name: 'Texture',
    declaredType: 'VARCHAR',
    kind: 'text',
    pinned: false,
    editable: true,
    protected: false,
    maxUtf16: 4,
    isCoverSingle: false,
    isBoolean: false,
  };
  assert.equal(grid.parseAndValidateCellInput('Mineral', textureCol, 'SiCL', false).error, '');
  assert.notEqual(
    grid.parseAndValidateCellInput('Mineral', textureCol, 'AB\u{1D306}D', false).error,
    ''
  );

  // Access BOOLEAN true = -1, false = 0
  const boolCol = {
    name: 'SpeciesListComplete',
    declaredType: 'BOOLEAN',
    kind: 'boolean',
    pinned: false,
    editable: true,
    protected: false,
    maxUtf16: 0,
    isCoverSingle: false,
    isBoolean: true,
  };
  assert.deepEqual(plain(grid.parseAndValidateCellInput('Env', boolCol, 'true', false).cell), {
    storage: 'integer',
    integer: '-1',
  });
  assert.deepEqual(plain(grid.parseAndValidateCellInput('Env', boolCol, 'false', false).cell), {
    storage: 'integer',
    integer: '0',
  });

  // BECSiteUnit custodian lock
  const becCol = {
    name: 'BECSiteUnit',
    declaredType: 'VARCHAR',
    kind: 'text',
    pinned: false,
    editable: true,
    protected: true,
    maxUtf16: 100,
    isCoverSingle: false,
    isBoolean: false,
  };
  assert.notEqual(
    grid.parseAndValidateCellInput('Admin', becCol, 'CWHvm1/01', false).error,
    ''
  );
  assert.equal(grid.parseAndValidateCellInput('Admin', becCol, 'CWHvm1/01', true).error, '');
});

test('TSV clipboard round-trip handles Excel CRLF, tabs, quotes, and leading-zero PlotNumber', () => {
  const cols = [
    { name: 'PlotNumber', declaredType: 'VARCHAR', kind: 'text', pinned: true, editable: true, protected: false, maxUtf16: 7, isCoverSingle: false, isBoolean: false },
    { name: 'Species', declaredType: 'VARCHAR', kind: 'text', pinned: false, editable: true, protected: false, maxUtf16: 8, isCoverSingle: false, isBoolean: false },
    { name: 'Cover1', declaredType: 'SINGLE', kind: 'single', pinned: false, editable: true, protected: false, maxUtf16: 0, isCoverSingle: true, isBoolean: false },
  ];
  const rows = [
    {
      rowId: '1',
      cells: [
        { storage: 'text', text: '00337' },
        { storage: 'text', text: 'TSUGHET' },
        { storage: 'real', real: 25.5 },
      ],
    },
    {
      rowId: '2',
      cells: [
        { storage: 'text', text: '108050' },
        { storage: 'text', text: 'THUJPIC' },
        { storage: 'null' },
      ],
    },
  ];
  const tsv = grid.formatTSVRange(rows, cols, { startRow: 0, endRow: 1, startCol: 0, endCol: 2 });
  assert.equal(tsv, '00337\tTSUGHET\t25.5\n108050\tTHUJPIC\t');

  const parsed = grid.parseTSVClipboard('00337\tTSUGHET\t25.5\r\n"Note\tWithTab"\tABIEAMA\t12\r\n');
  assert.deepEqual(plain(parsed), [
    ['00337', 'TSUGHET', '25.5'],
    ['Note\tWithTab', 'ABIEAMA', '12'],
  ]);
});

test('Enter vertical navigation, Tab horizontal navigation, and multi-column sort toggle', () => {
  assert.deepEqual(plain(grid.computeNextGridCell({ row: 2, col: 3 }, 'Enter', false, 10, 5)), {
    row: 3,
    col: 3,
  });
  assert.deepEqual(plain(grid.computeNextGridCell({ row: 2, col: 3 }, 'Enter', true, 10, 5)), {
    row: 1,
    col: 3,
  });
  assert.deepEqual(plain(grid.computeNextGridCell({ row: 0, col: 4 }, 'Tab', false, 10, 5)), {
    row: 1,
    col: 0,
  });
  assert.deepEqual(plain(grid.computeNextGridCell({ row: 1, col: 0 }, 'Tab', true, 10, 5)), {
    row: 0,
    col: 4,
  });

  let s = grid.toggleMultiColumnSort([], 'Zone', false);
  assert.deepEqual(plain(s), [{ column: 'Zone', direction: 'asc' }]);
  s = grid.toggleMultiColumnSort(s, 'Elevation', true);
  assert.deepEqual(plain(s), [
    { column: 'Zone', direction: 'asc' },
    { column: 'Elevation', direction: 'asc' },
  ]);
  s = grid.toggleMultiColumnSort(s, 'Elevation', true);
  assert.deepEqual(plain(s), [
    { column: 'Zone', direction: 'asc' },
    { column: 'Elevation', direction: 'desc' },
  ]);
});

test('UniversalDatasheetGrid.svelte compiles with zero warnings, renders all 9 tabs, and contains zero emojis', () => {
  const svelteSource = readFileSync(path.join(__dirname, 'UniversalDatasheetGrid.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(svelteSource), false, 'must contain zero emojis');

  const Component = serverComponent(svelteSource, 'UniversalDatasheetGrid.svelte', {
    './universalDatasheetGrid': grid,
  });
  const { body } = render(Component, {
    props: {
      contextId: 'ctx-123',
      project: 'Sample',
      su: 'Sample',
      hierarchy: 'Sample',
    },
  });
  assert.match(body, /Universal Project Datasheet/);
  for (const key of grid.CANONICAL_DATASHEET_KEYS) {
    assert.match(body, new RegExp(`_${key}`));
  }
});

test('UniversalDatasheetGrid preserves statusBanner across post-commit refreshTablesAndLoad after Save and Find/Replace', async () => {
  const vm = require('node:vm');
  const ts = require('typescript');
  const { compile } = require('svelte/compiler');
  const client = await import('svelte/internal/client');

  let dsScript = readFileSync(path.join(__dirname, 'UniversalDatasheetGrid.svelte'), 'utf8').match(
    /<script lang="ts">([\s\S]*?)<\/script>/
  )[1];
  dsScript = dsScript.replace(
    'onOpenPlot?: (plotNumber: string) => void;',
    'onOpenPlot?: (plotNumber: string) => void; inspect?: (v: any) => void;'
  );
  dsScript = dsScript.replace('onOpenPlot,', 'onOpenPlot, inspect,');
  dsScript += `\n$effect(() => inspect?.({
    setCellDraft,
    saveDrafts,
    setFindArgs(col, f, r) { findColumn = col; findText = f; replaceText = r; },
    runFindPreview,
    applyFindReplaceValid,
    getBanner: () => statusBanner,
  }));`;

  const compiled = compile(`<script lang="ts">${dsScript}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const dsMod = { exports: {} };
  vm.runInNewContext(
    ts.transpileModule(compiled.js.code, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
    }).outputText,
    {
      module: dsMod,
      exports: dsMod.exports,
      structuredClone,
      queueMicrotask,
      require(name) {
        if (name === 'svelte') {
          return { onMount: fn => fn(), onDestroy: () => {}, untrack: client.untrack };
        }
        if (name === 'svelte/internal/client') return client;
        if (name.startsWith('svelte/')) return require(name);
        if (name === './readRequests') return { ReadRequests: class { track(p) { return p; } cancelAll() {} } };
        if (name === './universalDatasheetGrid') return grid;
        throw new Error(`Unexpected module ${name}`);
      },
    }
  );

  let api;
  const dispose = client.effect_root(() => {
    dsMod.exports.default(null, {
      contextId: 'ctx-banner',
      initialTableKey: 'Veg',
      port: {
        listTables: async () => [{ key: 'Veg', tableName: 'Sample_Veg', available: true, rowCount: 1 }],
        getPage: async () => ({
          tableKey: 'Veg',
          tableName: 'Sample_Veg',
          totalRows: 1,
          offset: 0,
          limit: 250,
          canEditMaster: false,
          columns: [
            { name: 'PlotNumber', declaredType: 'TEXT', kind: 'text', pinned: true, editable: false, protected: false, maxUtf16: 7, isCoverSingle: false, isBoolean: false },
            { name: 'Species', declaredType: 'TEXT', kind: 'text', pinned: false, editable: true, protected: false, maxUtf16: 8, isCoverSingle: false, isBoolean: false },
          ],
          rows: [{ rowId: '1', cells: [{ storage: 'text', text: '00337' }, { storage: 'text', text: 'PSEUMEN' }] }],
        }),
        applyEdits: async () => ({ updatedCells: 1, updatedRows: 1, auditRows: 1 }),
        previewFindReplace: async () => ({
          tableKey: 'Veg',
          tableName: 'Sample_Veg',
          column: 'Species',
          matchedRows: 1,
          validCount: 1,
          invalidCount: 0,
          truncated: false,
          items: [
            {
              rowId: '1',
              plotNumber: '00337',
              validationError: '',
              edit: {
                rowId: '1',
                column: 'Species',
                before: { storage: 'text', text: 'PSEUMEN' },
                after: { storage: 'text', text: 'TSUGHET' },
              },
            },
          ],
        }),
      },
      inspect: v => {
        api = v;
      },
    });
  });

  try {
    client.flush();
    await new Promise(r => setTimeout(r, 10));
    client.flush();

    api.setCellDraft(0, 1, 'TSUGHET');
    client.flush();
    await api.saveDrafts();
    client.flush();
    assert.match(api.getBanner(), /^Saved 1 cell\(s\) across 1 row\(s\)/);

    api.setFindArgs('Species', 'PSEUMEN', 'TSUGHET');
    client.flush();
    await api.runFindPreview();
    client.flush();
    await api.applyFindReplaceValid();
    client.flush();
    assert.match(api.getBanner(), /^Applied 1 replacement\(s\) on Sample_Veg/);
  } finally {
    dispose();
  }
});
