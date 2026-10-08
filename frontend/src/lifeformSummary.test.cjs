const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { render } = require('svelte/server');
const summary = loadTypeScript('lifeformSummary.ts');
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
const owner = { contextId: 'owned', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
function fixture() {
  const memberships = [
    { rowId: '1', plotNumber: cell('P'), siteUnit: cell('U') },
    { rowId: '2', plotNumber: cell('P'), siteUnit: cell('U') },
    { rowId: '3', plotNumber: cell('P'), siteUnit: cell('V') },
    { rowId: '4', plotNumber: cell('orphan'), siteUnit: cell('') },
    { rowId: '5', plotNumber: cell(null), siteUnit: cell('Zero') },
    { rowId: '6', plotNumber: cell('P'), siteUnit: cell(null) }
  ];
  const catalogue = [
    { rowId: '1', lifeform: 1, label: cell('  Tree  '), definition: cell(null), shortName: cell('') },
    { rowId: '2', lifeform: 14, label: cell(null), definition: cell(''), shortName: cell(null) }
  ];
  const entries = [];
  for (const m of [memberships[0], memberships[1], memberships[2], memberships[5]]) {
    for (const speciesRowId of ['1', '2']) entries.push({ plotNumber: 'P', species: 'A', lifeform: 1,
      cover: 20, insertSuRowId: m.rowId, speciesRowId, projectId: structuredClone(m.siteUnit) });
  }
  const rows = (n, covers, plots, mean) => [
    { catalogueRowId: '1', lifeform: 1, plotGroups: plots, coverCount: covers, presence: n === 0 ? null : plots / n, meanCover: mean },
    { catalogueRowId: '2', lifeform: 14, plotGroups: 0, coverCount: 0, presence: n === 0 ? null : 0, meanCover: null }
  ];
  return { contextId: owner.contextId, projectPath: owner.projectPath, suPath: owner.suPath,
    report: { project: owner.project, su: owner.su, querySource: 'normal-su-global-entrydat-plot-rejoin',
      ordering: 'physical-catalogue-rowid-and-first-membership', memberships, entries, catalogue, units: [
        { code: cell('U'), nPlots: 2, suRowIds: ['1', '2'], uniqueSpecies: 1, occurrences: 16, rows: rows(2, 16, 1, 1.6) },
        { code: cell('V'), nPlots: 1, suRowIds: ['3'], uniqueSpecies: 1, occurrences: 8, rows: rows(1, 8, 1, 1.6) },
        { code: cell(''), nPlots: 1, suRowIds: ['4'], uniqueSpecies: 0, occurrences: 0, rows: rows(1, 0, 0, null) },
        { code: cell('Zero'), nPlots: 0, suRowIds: ['5'], uniqueSpecies: 0, occurrences: 0, rows: rows(0, 0, 0, null) },
        { code: cell(null), nPlots: 1, suRowIds: ['6'], uniqueSpecies: 0, occurrences: 0, rows: rows(1, 0, 0, null) }
      ] } };
}
test('lifeform snapshot retains global SU/species fanout, physical denominators and detached NULL/empty metadata', () => {
  const input = fixture(), output = summary.validateLifeformSummary(input, owner);
  assert.equal(output.report.units[0].occurrences, 16);
  assert.equal(output.report.units[0].rows[0].meanCover, 1.6);
  assert.equal(output.report.catalogue[0].label.text, '  Tree  ');
  output.report.memberships[0].siteUnit.text = 'changed';
  assert.equal(input.report.memberships[0].siteUnit.text, 'U');
  assert.equal(summary.lifeformCellText(cell(null)), '(NULL)');
  assert.equal(summary.lifeformCellText(cell('')), '(empty)');
  assert.equal(summary.lifeformRatioText(null), '(NULL)');
  assert.equal(summary.lifeformRatioText(0), '0.0%');
});
test('lifeform rejects stale, unknown, incomplete, non-SINGLE and invented physical counts without repair', () => {
  for (const mutate of [
    v => v.contextId = 'stale', v => v.projectPath = 'elsewhere', v => v.suPath = 'elsewhere',
    v => v.report.project = 'other', v => v.report.su = 'None', v => v.report.querySource = 'per-unit-insert',
    v => v.report.ordering = 'Access', v => v.report.extra = 1, v => v.report.units = null,
    v => v.report.entries = null, v => v.report.catalogue = null, v => v.report.memberships = null,
    v => v.report.entries[0].cover = 0.1, v => v.report.entries[0].cover = Infinity,
    v => v.report.entries[0].species = '\ud800', v => v.report.entries[0].species = '123456789',
    v => v.report.entries[0].plotNumber = 'other', v => v.report.entries[0].projectId = cell('V'),
    v => v.report.entries[0].lifeform = 13, v => v.report.entries.push(v.report.entries[0]),
    v => v.report.memberships[0].rowId = '01', v => v.report.memberships.push(v.report.memberships[0]),
    v => v.report.catalogue[0].label.text = '\udfff', v => v.report.catalogue[0].label.extra = true,
    v => v.report.catalogue[0].definition.text = '', v => v.report.catalogue[0].lifeform = 13,
    v => v.report.catalogue.push(v.report.catalogue[0]), v => v.report.units[0].suRowIds = ['1'],
    v => v.report.units[0].nPlots = 1, v => v.report.units[0].occurrences = 8,
    v => v.report.units[0].uniqueSpecies = 2, v => v.report.units[0].rows[0].presence = 1,
    v => v.report.units[0].rows[0].coverCount = 8, v => v.report.units[0].rows[0].meanCover = 0.8,
    v => v.report.units[2].rows[0].meanCover = 0, v => v.report.units[3].rows[0].presence = 0,
    v => v.report.units[0].rows.reverse(), v => v.report.units[0].rows = null,
    v => v.report.units[0].code.integer = '1', v => v.report.units[0].code.text = '\0'
  ]) {
    const input = fixture(); mutate(input);
    assert.throws(() => summary.validateLifeformSummary(input, owner));
  }
});
test('lifeform empty shape is valid only with explicit complete empty arrays', () => {
  const input = fixture();
  Object.assign(input.report, { memberships: [], entries: [], catalogue: [], units: [] });
  assert.equal(summary.validateLifeformSummary(input, owner).report.units.length, 0);
  input.report.entries = undefined;
  assert.throws(() => summary.validateLifeformSummary(input, owner));
});
test('lifeform exact SINGLE sums preserve cancellation of large historical values', () => {
  const input = fixture();
  input.report.entries = [
    { plotNumber: 'P', species: 'A', lifeform: 1, cover: Math.fround(1e30), insertSuRowId: '1', speciesRowId: '1', projectId: cell('U') },
    { plotNumber: 'P', species: 'B', lifeform: 1, cover: 1, insertSuRowId: '1', speciesRowId: '2', projectId: cell('U') },
    { plotNumber: 'P', species: 'C', lifeform: 1, cover: -Math.fround(1e30), insertSuRowId: '1', speciesRowId: '3', projectId: cell('U') }
  ];
  input.report.units[0].occurrences = 6; input.report.units[0].uniqueSpecies = 3;
  input.report.units[0].rows[0].coverCount = 6; input.report.units[0].rows[0].meanCover = 0.01;
  input.report.units[1].occurrences = 3; input.report.units[1].uniqueSpecies = 3;
  input.report.units[1].rows[0].coverCount = 3; input.report.units[1].rows[0].meanCover = 0.01;
  assert.equal(summary.validateLifeformSummary(input, owner).report.units[0].rows[0].meanCover, 0.01);
});
test('standalone lifeform component is gated, read-only, responsive and cancels on destroy', () => {
  const source = readFileSync(path.join(__dirname, 'LifeformSummary.svelte'), 'utf8');
  assert.equal(compile(source, { filename: 'LifeformSummary.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(source, /VITE_LIFEFORM_SUMMARY === 'true'/);
  assert.match(source, /LifeformSummaryService\.Preview\(contextId\)/);
  assert.match(source, /onDestroy\(cancel\)/);
  assert.match(source, /request !== generation \|\| key !== ownerKey/);
  assert.match(source, /previewKey === ownerKey/);
  assert.match(source, /onBusyChange\(true\)/);
  assert.match(source, /reads\.cancelAll\(\)/);
  assert.match(source, /grid-template-columns: repeat\(auto-fit/);
  assert.match(source, /scope="col"/);
  assert.match(source, /<th scope="col">Definition<\/th>/);
  assert.match(source, /<th scope="col">Short name<\/th>/);
  assert.match(source, /lifeformCellText\(currentPreview\.report\.catalogue\[i\]\.definition\)/);
  assert.match(source, /lifeformCellText\(currentPreview\.report\.catalogue\[i\]\.shortName\)/);
  assert.doesNotMatch(source, /localStorage|\.Save|<input|Export|ExcelService/);
});
test('lifeform read-only catalogue descriptions preserve numeric labels and NULL versus empty text', () => {
  const input = fixture();
  input.report.catalogue[0].label = cell('01');
  input.report.catalogue[0].definition = cell('  coniferous tree  ');
  input.report.catalogue[0].shortName = cell('1Conifer');
  const source = readFileSync(path.join(__dirname, 'LifeformSummary.svelte'), 'utf8')
    .replace("import.meta.env.VITE_LIFEFORM_SUMMARY === 'true'", 'true')
    .replace('let preview = $state<LifeformSummaryPreview | null>(null);',
      `let preview = $state<LifeformSummaryPreview | null>(${JSON.stringify(input)});`)
    .replace("let previewKey = $state('');",
      `let previewKey = $state(${JSON.stringify(JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]))});`);
  const component = serverComponent(source, 'LifeformSummary.svelte', {
    '../bindings/github.com/boostao/vpro-wails': { LifeformSummaryService: {} },
    './lifeformSummary': summary
  });
  const html = render(component, { props: { ...owner, onBusyChange() {} } }).body
    .replace(/\sclass="[^"]*"/g, '');
  assert.match(html, /scope="col">Definition<\/th>/);
  assert.match(html, /scope="col">Short name<\/th>/);
  assert.match(html, /scope="row">1<\/th><td>01<\/td>\s*<td>  coniferous tree  <\/td><td>1Conifer<\/td>/);
  assert.match(html, /scope="row">14<\/th><td>\(NULL\)<\/td>\s*<td>\(empty\)<\/td><td>\(NULL\)<\/td>/);
  assert.doesNotMatch(html, /<input|contenteditable|<textarea/);
});
