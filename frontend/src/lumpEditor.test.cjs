const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');

const calls = [];
const mockRuntime = {
  Call: {
    ByName: (name, ...args) => {
      calls.push([name, ...args]);
      return Promise.resolve(null);
    },
  },
};

const lump = loadTypeScript('lumpEditor.ts', { '@wailsio/runtime': mockRuntime });

const fromVM = (v) => JSON.parse(JSON.stringify(v));

test('validateLumpBaseName enforces identifier pattern, strips _Lump suffix, and rejects reserved names', () => {
  assert.deepEqual(fromVM(lump.validateLumpBaseName('Regional2026_Lump')), {
    valid: true,
    normalized: 'Regional2026',
    error: '',
  });
  assert.equal(lump.validateLumpBaseName('None').valid, false);
  assert.equal(lump.validateLumpBaseName('CombinedSpp_Lump').valid, false);
  assert.equal(lump.validateLumpBaseName('USysLumpMaster').valid, false);
  assert.equal(lump.validateLumpBaseName('123Bad').valid, false);
  assert.equal(lump.validateLumpBaseName('Bad\uD800Name').valid, false);
});

test('validateLumpRowDraft enforces UTF-16 code unit limits (LumpCode <= 50, SppCode <= 8) and non-self-lump', () => {
  const ok = fromVM(lump.validateLumpRowDraft(' carex ', ' caresit '));
  assert.deepEqual(ok, { valid: true, lumpCode: 'CAREX', sppCode: 'CARESIT', error: '' });

  assert.equal(lump.validateLumpRowDraft('CAREX', 'CAREX').valid, false);
  assert.equal(lump.validateLumpRowDraft('CAREX', 'ABCDEFGHI').valid, false);
  assert.equal(lump.validateLumpRowDraft('X'.repeat(51), 'CARESIT').valid, false);
  assert.equal(lump.validateLumpRowDraft('CAREX', 'BAD\0').valid, false);
});

test('parseGenusRollupTokens and groupLumpRowsByCode deduplicate and sort deterministically', () => {
  assert.deepEqual(Array.from(lump.parseGenusRollupTokens(' care* , sali*;\nSPHA*, CARE* ')), [
    'CARE*',
    'SALI*',
    'SPHA*',
  ]);

  const grouped = lump.groupLumpRowsByCode([
    { lumpCode: 'SALIX', sppCode: 'SALISCO', use: true, scientificName: 'Salix scouleriana', englishName: '', sourceCatalog: 'VLists', lumpKnown: true },
    { lumpCode: 'CAREX', sppCode: 'CARESIT', use: true, scientificName: 'Carex sitchensis', englishName: '', sourceCatalog: 'VLists', lumpKnown: true },
    { lumpCode: 'SALIX', sppCode: 'SALIBEB', use: false, scientificName: 'Salix bebbiana', englishName: '', sourceCatalog: 'VLists', lumpKnown: true },
  ]);
  assert.equal(grouped.length, 2);
  assert.equal(grouped[0].lumpCode, 'CAREX');
  assert.equal(grouped[1].lumpCode, 'SALIX');
  assert.equal(grouped[1].activeCount, 1);
  assert.deepEqual(Array.from(grouped[1].rows, (r) => r.sppCode), ['SALIBEB', 'SALISCO']);
});

test('defaultLumpEditorPort routes Wails Call.ByName to main.ImportExportService methods', async () => {
  calls.length = 0;
  await lump.defaultLumpEditorPort.discoverLumpTables('ctx-1', ['/ext/a.db']);
  await lump.defaultLumpEditorPort.toggleLumpRow('ctx-1', 'Test_Lump', 'CARESIT', false);
  await lump.defaultLumpEditorPort.previewLumpSpec('ctx-1', {
    preset: 'BEC_Provincial_Correlation_2026',
    collapseInfraspecific: true,
    collapseSeedlingAndDeadSuffixes: true,
    genusRollups: ['CARE*'],
    scopeToProjectSpecies: true,
  });
  assert.equal(calls[0][0], 'main.ImportExportService.DiscoverLumpTables');
  assert.equal(calls[1][0], 'main.ImportExportService.ToggleLumpRow');
  assert.equal(calls[2][0], 'main.ImportExportService.PreviewLumpSpec');
});

test('LumpEditor.svelte compiles with zero warnings, renders BC Gov header, and contains zero emojis', () => {
  const source = readFileSync(path.join(__dirname, 'LumpEditor.svelte'), 'utf8');
  assert.equal(/\p{Extended_Pictographic}/u.test(source), false);
  assert.match(source, /#003366/);
  assert.match(source, /#fcba19/i);

  const Component = serverComponent(source, 'LumpEditor.svelte', { './lumpEditor': lump });
  const rendered = render(Component, { props: { contextId: 'ctx-lump-test' } });
  assert.match(rendered.body, /Combine Species \(\*_Lump\)/);
  assert.match(rendered.body, /Declarative LumpSpec \(Auto-5\)/);
});
