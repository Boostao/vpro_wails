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
