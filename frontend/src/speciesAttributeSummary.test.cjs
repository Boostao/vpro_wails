const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const summary = loadTypeScript('speciesAttributeSummary.ts', { './lifeformSummary': lifeform });
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
const owner = { contextId: 'owned', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };

function fixture() {
  const definitions = structuredClone(summary.speciesAttributeDefinitions);
  const memberships = [
    { rowId: '1', plotNumber: cell('P'), siteUnit: cell('U') },
    { rowId: '2', plotNumber: cell('P'), siteUnit: cell('U') },
    { rowId: '3', plotNumber: cell('P'), siteUnit: cell(null) },
    { rowId: '4', plotNumber: cell(null), siteUnit: cell('') },
  ];
  const matches = [];
  for (const suRowId of ['1', '2']) {
    for (const vegRowId of ['1', '2']) {
      for (const attributeRowId of ['1', '2']) matches.push({
        suRowId, vegRowId, attributeRowId, plotNumber: 'P', species: 'A',
        values: definitions.map(d => cell(attributeRowId === '1' ? d.categories[0] : '')),
      });
    }
  }
  const rows = observed => definitions.map(d => ({
    field: d.field, count: observed ? 8 : null, plotOccurrences: observed ? 1 : 0,
    categories: d.categories.map((_, i) => observed && i === 0 ? 4 : null),
  }));
  return { contextId: owner.contextId, projectPath: owner.projectPath, suPath: owner.suPath,
    report: { project: owner.project, su: owner.su, querySource: 'normal-su-raw-veg-attribute-join',
      definitions, memberships, matches, units: [
        { code: cell('U'), nPlots: 2, suRowIds: ['1', '2'], rows: rows(true) },
        { code: cell(null), nPlots: 1, suRowIds: ['3'], rows: rows(false) },
        { code: cell(''), nPlots: 0, suRowIds: ['4'], rows: rows(false) },
      ] } };
}

test('attribute summary preserves raw physical weights, fixed pivots, non-pivot totals and detached NULLs', () => {
  const input = fixture(), result = summary.validateSpeciesAttributeSummary(input, owner);
  assert.equal(result.report.units[0].rows[0].count, 8);
  assert.equal(result.report.units[0].rows[0].plotOccurrences, 1);
  assert.equal(result.report.units[0].rows[0].categories[0], 4);
  assert.equal(result.report.units[1].rows[0].count, null);
  result.report.matches[0].values[0].text = 'detached';
  assert.equal(input.report.matches[0].values[0].text, 'S1');
  assert.equal(summary.speciesAttributeCountText(null), 'NULL');
  assert.equal(summary.speciesAttributeCountText(0), '0');
});

test('attribute transport rejects incoherent source identities and counts without repairing them', () => {
  for (const mutate of [
    v => v.contextId = 'stale', v => v.suPath = 'other', v => v.report.su = 'None',
    v => v.report.querySource = 'max-cover', v => v.report.extra = true,
    v => v.report.definitions[0].categories.reverse(), v => v.report.definitions[0].label = 'guessed',
    v => v.report.memberships[0].rowId = '01', v => v.report.memberships.push(v.report.memberships[0]),
    v => v.report.matches.push(v.report.matches[0]), v => v.report.matches[0].suRowId = '3',
    v => v.report.matches[0].plotNumber = 'other', v => v.report.matches[0].species = '\ud800',
    v => v.report.matches[0].values[0] = cell('\udfff'), v => v.report.matches[0].values.pop(),
    v => v.report.matches[0].values[0].storage = 'integer',
    v => v.report.matches[1].vegRowId = '2', v => v.report.matches[1].species = 'different',
    v => v.report.matches[4].values[0] = cell('different'),
    v => v.report.units[0].nPlots = 1, v => v.report.units[0].suRowIds = ['1'],
    v => v.report.units[0].rows[0].count = 4, v => v.report.units[0].rows[0].plotOccurrences = 2,
    v => v.report.units[0].rows[0].categories[0] = 8, v => v.report.units[1].rows[0].count = 0,
    v => v.report.units[1].rows[0].categories[0] = 0, v => v.report.units[0].rows[0].field = 'other',
    v => v.report.units.push(v.report.units[0]), v => v.report.units.pop(),
  ]) {
    const input = fixture(); mutate(input);
    assert.throws(() => summary.validateSpeciesAttributeSummary(input, owner), /no repairs applied/, String(mutate));
  }
});

test('attribute read uses the shared cancellable busy boundary and independent default-off gate', () => {
  const source = readFileSync(path.join(__dirname, 'LifeformSummary.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'LifeformSummary.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_SPECIES_ATTRIBUTE_SUMMARY === 'true'/);
  assert.match(source, /!attributesEnabled \|\| busy/);
  assert.match(source, /SpeciesAttributeSummaryService\.Preview\(contextId\)/);
  assert.match(source, /validateSpeciesAttributeSummary\(value, owner\)/);
  assert.match(source, /reads\.track<unknown>\(kind === 'attributes'/);
  assert.match(source, /onDestroy\(cancel\)/);
  assert.match(source, /request !== generation \|\| key !== ownerKey/);
  assert.doesNotMatch(source, /localStorage|\.Save|\.Apply|\.Export/);
});

test('attribute preview renders six source families, nullable counts and visible detail headers without editors', () => {
  const source = readFileSync(path.join(__dirname, 'LifeformSummary.svelte'), 'utf8')
    .replace("import.meta.env.VITE_LIFEFORM_SUMMARY === 'true'", 'true')
    .replace("import.meta.env.VITE_SPECIES_ATTRIBUTE_SUMMARY === 'true'", 'true')
    .replace("import.meta.env.VITE_LIFEFORM_WORKBOOK === 'true'", 'false')
    .replace('let attributePreview = $state<SpeciesAttributeSummaryPreview | null>(null);',
      `let attributePreview = $state<SpeciesAttributeSummaryPreview | null>(${JSON.stringify(fixture())});`)
    .replace("let previewKey = $state('');",
      `let previewKey = $state(${JSON.stringify(JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]))});`);
  const component = serverComponent(source, 'LifeformSummary.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { LifeformSummaryService: {}, SpeciesAttributeSummaryService: {} },
    './LifeformWorkbookPanel.svelte': { default() { assert.fail('Disabled workbook panel must not mount'); } },
    './lifeformSummary': lifeform, './speciesAttributeSummary': summary,
  });
  const html = render(component, { props: { ...owner, onBusyChange() {} } }).body.replace(/\sclass="[^"]*"/g, '');
  for (const definition of summary.speciesAttributeDefinitions) assert.match(html, new RegExp(`<th scope="row">${definition.label}</th>`));
  assert.match(html, /Number of plot occurrences/);
  assert.match(html, /<td>8<\/td><td>1<\/td>/);
  assert.match(html, /<td>NULL<\/td><td>0<\/td>/);
  assert.match(html, /scope="col">S2S3<\/th>/);
  assert.doesNotMatch(html, /<input|contenteditable|<textarea/);
});
