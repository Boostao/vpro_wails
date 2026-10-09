const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const lifeform = loadTypeScript('lifeformSummary.ts');
const attributes = loadTypeScript('speciesAttributeSummary.ts', { './lifeformSummary': lifeform });
const publication = loadTypeScript('publicationSession.ts');
const environment = loadTypeScript('environmentWorkbook.ts', {
  './lifeformSummary': lifeform, './publicationSession': publication,
  './longEnvironmentReport': {},
});
const workbook = loadTypeScript('lifeformWorkbook.ts', {
  './lifeformSummary': lifeform, './speciesAttributeSummary': attributes,
  './environmentWorkbook': environment, './publicationSession': publication,
});
const owner = { contextId: 'lifeform-workbook', project: 'Sample', projectPath: 'C:\\project.db', su: 'Selected', suPath: 'C:\\external.db' };
const destination = 'C:\\output\\ exact name .xlsx';
const cell = text => ({ storage: text === null ? 'null' : 'text', text, integer: null, real: null, blobHex: null });
const defaultDetails = () => [false, false, false, false, false, false];
function fixture(details = defaultDetails()) {
  const memberships = [
    { rowId: '1', plotNumber: cell('P'), siteUnit: cell('U:1') },
    { rowId: '2', plotNumber: cell('P'), siteUnit: cell('U:1') },
    { rowId: '3', plotNumber: cell('Q'), siteUnit: cell(null) },
    { rowId: '4', plotNumber: cell(null), siteUnit: cell('') },
    { rowId: '5', plotNumber: cell('Z'), siteUnit: cell('Zero') },
  ];
  const definitions = structuredClone(attributes.speciesAttributeDefinitions);
  const catalogue = [
    { rowId: '1', lifeform: 1, label: cell(' Tree '), definition: cell(null), shortName: cell('') },
    { rowId: '2', lifeform: 14, label: cell(null), definition: cell(''), shortName: cell(null) },
  ];
  const units = [
    { code: cell('U:1'), nPlots: 2, suRowIds: ['1', '2'], uniqueSpecies: 1, occurrences: 4,
      rows: [
        { catalogueRowId: '1', lifeform: 1, plotGroups: 1, coverCount: 4, presence: .5, meanCover: .4 },
        { catalogueRowId: '2', lifeform: 14, plotGroups: 0, coverCount: 0, presence: 0, meanCover: null },
      ] },
    ...memberships.slice(2).map(m => ({
      code: structuredClone(m.siteUnit), nPlots: m.plotNumber.text === null ? 0 : 1, suRowIds: [m.rowId],
      uniqueSpecies: 0, occurrences: 0, rows: catalogue.map(c => ({
        catalogueRowId: c.rowId, lifeform: c.lifeform, plotGroups: 0, coverCount: 0,
        presence: m.plotNumber.text === null ? null : 0, meanCover: null,
      })),
    })),
  ];
  const entries = ['1', '2'].map(insertSuRowId => ({
    plotNumber: 'P', species: 'A', lifeform: 1, cover: 20, insertSuRowId, speciesRowId: '1', projectId: cell('U:1'),
  }));
  const matches = [];
  for (const suRowId of ['1', '2']) for (const vegRowId of ['1', '2']) for (const attributeRowId of ['1', '2']) {
    matches.push({ suRowId, vegRowId, attributeRowId, plotNumber: 'P', species: 'A',
      values: definitions.map(d => cell(attributeRowId === '1' ? d.categories[0] : '')) });
  }
  const identity = { contextId: owner.contextId, projectPath: owner.projectPath, suPath: owner.suPath };
  return {
    lifeform: { ...identity, report: { project: owner.project, su: owner.su,
      querySource: 'normal-su-global-entrydat-plot-rejoin', ordering: 'physical-catalogue-rowid-and-first-membership',
      memberships: structuredClone(memberships), entries, catalogue, units } },
    attributes: { ...identity, report: { project: owner.project, su: owner.su,
      querySource: 'normal-su-raw-veg-attribute-join', definitions, memberships: structuredClone(memberships), matches,
      units: units.map((u, i) => ({ code: structuredClone(u.code), nPlots: u.nPlots, suRowIds: [...u.suRowIds],
        rows: definitions.map(d => ({ field: d.field, count: i === 0 ? 8 : null, plotOccurrences: i === 0 ? 1 : 0,
          categories: d.categories.map((_, j) => i === 0 && j === 0 ? 4 : null) })) })) } },
    details: [...details],
    sheets: units.map((u, i) => ({ unit: structuredClone(u.code), name: i === 0 ? 'U-1' : u.code.text || `NoName${i}` })),
    approvalHash: 'a'.repeat(64), workbookSHA256: 'b'.repeat(64), bytes: 12345,
  };
}
function renameUnit(value, i, code, name) {
  for (const preview of [value.lifeform, value.attributes]) {
    const unit = preview.report.units[i], prior = JSON.stringify(unit.code);
    unit.code = cell(code);
    for (const m of preview.report.memberships) if (unit.suRowIds.includes(m.rowId)) m.siteUnit = cell(code);
    for (const e of preview.report.entries || []) if (JSON.stringify(e.projectId) === prior) e.projectId = cell(code);
  }
  value.sheets[i] = { unit: cell(code), name };
}
const receipt = (status = 'published') => ({
  status, requestedDestination: destination, path: status === 'not-published' ? '' : destination,
  sha256: status === 'not-published' ? '' : 'b'.repeat(64), errorMessage: status === 'published' ? '' : 'Explicit source/receipt error',
});

test('complete projection is detached and immutable, preserving all six totals, physical counts and NULL/empty codes', () => {
  const input = fixture(), output = workbook.validateLifeformWorkbookReview(input, owner, defaultDetails());
  input.lifeform.report.catalogue[0].label.text = 'changed'; input.details[0] = true; input.sheets[0].name = 'changed';
  assert.equal(output.lifeform.report.catalogue[0].label.text, ' Tree ');
  assert.equal(output.details[0], false); assert.equal(output.sheets[0].name, 'U-1');
  assert.ok(Object.isFrozen(output)); assert.ok(Object.isFrozen(output.lifeform.report.units[0].rows));
  assert.ok(Object.isFrozen(output.details));
  assert.equal(output.attributes.report.units[0].rows.length, 6);
  assert.equal(output.attributes.report.units[0].rows[0].count, 8);
  assert.equal(output.lifeform.report.units[0].occurrences, 4);
  assert.equal(output.lifeform.report.units[0].nPlots, 2);
  assert.equal(output.sheets[1].unit.text, null); assert.equal(output.sheets[1].name, 'NoName1');
  assert.equal(output.sheets[2].unit.text, ''); assert.equal(output.sheets[2].name, 'NoName2');
  for (let mask = 0; mask < 64; mask++) {
    const details = Array.from({ length: 6 }, (_, i) => !!(mask & (1 << i)));
    const value = workbook.validateLifeformWorkbookReview(fixture(details), owner, details);
    assert.equal(value.attributes.report.units[0].rows.length, 6);
    assert.equal(value.details.join(','), details.join(','));
  }
});

test('rejects malformed source details, ownership, metadata, incomplete joins, counts and worksheet mappings', () => {
  const mutations = [
    v => v.details = null, v => v.details = [], v => v.details[0] = 0, v => delete v.details[0],
    v => v.details[0] = true, v => v.approvalHash = 'A'.repeat(64), v => v.workbookSHA256 = '',
    v => v.bytes = 0, v => v.bytes = 1.1, v => v.bytes = Infinity, v => v.bytes = Number.MAX_SAFE_INTEGER + 1,
    v => v.extra = 1, v => v.lifeform.contextId = 'other', v => v.attributes.projectPath = 'other',
    v => v.lifeform.suPath = 'other', v => v.attributes.report.project = 'other',
    v => v.lifeform.report.su = 'None', v => v.lifeform.report.catalogue = [],
    v => v.attributes.report.memberships.pop(), v => v.lifeform.report.units = null,
    v => v.attributes.report.units.pop(), v => v.attributes.report.units[0].rows.pop(),
    v => v.attributes.report.units[0].rows[0].count = 4,
    v => v.attributes.report.units[0].rows[0].categories[0] = 8,
    v => v.lifeform.report.units[0].nPlots = 1, v => v.lifeform.report.units[0].occurrences = 2,
    v => v.lifeform.report.entries[0].cover = .1, v => v.lifeform.report.units[0].rows[0].meanCover = .2,
    v => v.sheets = null, v => v.sheets.pop(), v => v.sheets[0].unit = cell('U-1'),
    v => v.sheets[1].unit = cell(''), v => v.sheets[0].name = 'U:1', v => v.sheets[0].name = '\ud800',
    v => v.sheets[0].extra = true, v => v.lifeform.report.catalogue[0].definition = cell('\u0001'),
    v => v.lifeform.report.memberships[0].siteUnit = cell('\ud800'),
  ];
  for (const [i, mutate] of mutations.entries()) {
    const value = fixture(); mutate(value);
    assert.throws(() => workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()), `mutation ${i}`);
  }
  for (const details of [null, [], [false], [false, false, false, false, false, null]]) {
    assert.throws(() => workbook.validateLifeformWorkbookReview(fixture(), owner, details));
  }
  for (const key of Object.keys(owner)) assert.throws(() =>
    workbook.validateLifeformWorkbookReview(fixture(), { ...owner, [key]: 'other' }, defaultDetails()));
});

test('requires original first-membership order and complete matching ordered memberships', () => {
  for (const mutate of [
    v => { v.attributes.report.memberships.reverse(); },
    v => { for (const p of [v.lifeform, v.attributes]) p.report.units[0].suRowIds.reverse(); },
    v => { for (const p of [v.lifeform, v.attributes]) [p.report.units[1], p.report.units[2]] = [p.report.units[2], p.report.units[1]];
      [v.sheets[1], v.sheets[2]] = [v.sheets[2], v.sheets[1]]; },
  ]) {
    const value = fixture(); mutate(value);
    assert.throws(() => workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()));
  }
  const value = fixture();
  value.sheets[0].unit = { text: 'U:1', storage: 'text', blobHex: null, real: null, integer: null };
  assert.equal(workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()).sheets[0].name, 'U-1');
});

test('worksheet sanitization is exact at 31 UTF16 units, refuses split surrogates and collisions without suffixes', () => {
  for (const [code, name] of [
    ['A:/\\[]*?B', 'A-------B'], ['x'.repeat(29) + '😀' + 'tail', 'x'.repeat(29) + '😀'],
    ['x'.repeat(32), 'x'.repeat(31)], ['  exact  ', '  exact  '],
  ]) {
    const value = fixture(); renameUnit(value, 3, code, name);
    assert.equal(workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()).sheets[3].name, name);
  }
  for (const code of ["'bad", "bad'", '_VPRO_Source', '\ud800', 'x'.repeat(30) + '😀', 'x'.repeat(32768)]) {
    const value = fixture(); renameUnit(value, 3, code, code.slice(0, 31));
    assert.throws(() => workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()));
  }
  for (const [a, b, first, second] of [
    ['A:B', 'a?b', 'A-B', 'a-b'], ['NoName1', null, 'NoName1', 'NoName1'],
    ['x'.repeat(31) + 'a', 'x'.repeat(31) + 'b', 'x'.repeat(31), 'x'.repeat(31)],
    ['S', 'ſ', 'S', 'ſ'], ['Μ', 'µ', 'Μ', 'µ'], ['Σ', 'ς', 'Σ', 'ς'],
  ]) {
    const value = fixture(); renameUnit(value, 2, a, first); renameUnit(value, 1, b, second);
    assert.throws(() => workbook.validateLifeformWorkbookReview(value, owner, defaultDetails()));
  }
});

test('actual shared publication session freezes request, cannot cancel/replay, persists remount and refuses cross-owner reuse', async () => {
  const owned = { ...owner, contextId: 'persistent-lifeform' };
  const value = fixture(); value.lifeform.contextId = value.attributes.contextId = owned.contextId;
  const session = workbook.lifeformWorkbookPublicationSession(owned);
  let resolve, request, calls = 0;
  const pending = session.publish(value, destination, captured => {
    request = captured; calls++; return new Promise(done => { resolve = done; });
  });
  value.approvalHash = 'changed'; value.details[0] = true; owned.projectPath = 'changed';
  assert.equal(session.view().busy, true); assert.equal(session.view().blocked, true);
  assert.throws(() => session.acknowledge(), /cannot be cancelled/);
  const clean = fixture(); clean.lifeform.contextId = clean.attributes.contextId = 'persistent-lifeform';
  await assert.rejects(session.publish(clean, destination, async () => receipt()), /Acknowledge/);
  const remountOwner = { ...owner, contextId: 'persistent-lifeform' };
  assert.equal(workbook.lifeformWorkbookPublicationSession(remountOwner), session);
  for (const key of ['project', 'projectPath', 'su', 'suPath']) assert.throws(() =>
    workbook.lifeformWorkbookPublicationSession({ ...remountOwner, [key]: 'other' }), /owned paths/);
  resolve(receipt('published-with-errors')); await pending;
  assert.equal(calls, 1); assert.equal(request.details.join(','), defaultDetails().join(','));
  assert.equal(request.destination, destination); assert.equal(request.approvalHash, 'a'.repeat(64));
  assert.equal(session.view().blocked, true); assert.equal(session.view().busy, false);
  await assert.rejects(session.publish(clean, destination, async () => receipt()), /Acknowledge/);
  session.acknowledge();
  assert.equal(session.view().blocked, false); assert.equal(session.view().outcome.status, 'published-with-errors');
  assert.match(session.view().error, /Explicit/);
});

test('unknown or contradictory outcomes retain barriers and requested destination until explicit acknowledgement', async () => {
  for (const result of [null, new Error('lost acknowledgement'), { status: 'success' },
    { ...receipt(), sha256: 'c'.repeat(64) }, { ...receipt(), requestedDestination: 'other' },
    { ...receipt(), path: '' }, { ...receipt(), errorMessage: 'hidden warning' }]) {
    const session = new workbook.LifeformWorkbookPublicationSession(owner);
    let calls = 0;
    await session.publish(fixture(), destination, async () => {
      calls++; if (result instanceof Error) throw result; return result;
    });
    assert.equal(session.view().outcome, null); assert.equal(session.view().busy, false);
    assert.equal(session.view().blocked, true); assert.equal(session.view().requestedDestination, destination);
    assert.match(session.view().error, /outcome unknown/);
    await assert.rejects(session.publish(fixture(), destination, async () => { calls++; return receipt(); }), /Acknowledge/);
    assert.equal(calls, 1); session.acknowledge();
    assert.equal(session.view().outcome, null); assert.match(session.view().error, /inspect/);
  }
  const session = new workbook.LifeformWorkbookPublicationSession(owner);
  await session.publish(fixture(), destination, async () => receipt('not-published'));
  assert.equal(session.view().blocked, true); assert.equal(session.view().outcome.status, 'not-published');
  session.acknowledge(); assert.equal(session.view().outcome.status, 'not-published');
});

test('unknown acknowledgement persists registry remounts and review cancellation cannot clear its barrier', async () => {
  const owned = { ...owner, contextId: 'unknown-remount' };
  const value = fixture(); value.lifeform.contextId = value.attributes.contextId = owned.contextId;
  const session = workbook.lifeformWorkbookPublicationSession(owned);
  let held = false;
  const unsubscribe = session.subscribe(() => { const state = session.view(); held = state.busy || state.blocked; });
  await session.publish(value, destination, async () => { throw new Error('transport lost'); });
  assert.equal(held, true); unsubscribe();
  const remounted = workbook.lifeformWorkbookPublicationSession({ ...owned });
  assert.equal(remounted, session); assert.equal(remounted.view().blocked, true);
  assert.equal(remounted.view().requestedDestination, destination);
  await assert.rejects(remounted.publish(value, destination, async () => receipt()), /Acknowledge/);
  remounted.acknowledge(); assert.equal(remounted.view().blocked, false);
  assert.match(remounted.view().error, /transport lost/);
  const source = readFileSync(path.join(__dirname, 'LifeformWorkbookPanel.svelte'), 'utf8');
  const cancel = source.slice(source.indexOf('function cancel()'), source.indexOf('function changeDetail'));
  assert.doesNotMatch(cancel, /acknowledge|publication\s*=|session\./);
});

test('publication rejects malformed literal paths and foreign review before dispatch, preserving exact valid destination', async () => {
  const session = new workbook.LifeformWorkbookPublicationSession(owner);
  let calls = 0;
  for (const bad of ['', null, 'x.csv', 'x.xlsx ', 'x.xlsx\0', '\ud800.xlsx']) {
    await assert.rejects(session.publish(fixture(), bad, async () => { calls++; return receipt(); }), /literal .xlsx/);
  }
  const foreign = fixture(); foreign.lifeform.projectPath = 'other';
  await assert.rejects(session.publish(foreign, destination, async () => { calls++; return receipt(); }));
  assert.equal(calls, 0); assert.equal(session.view().blocked, false);
  await session.publish(fixture(), destination, async request => {
    calls++; assert.equal(request.destination, destination); return receipt();
  });
  assert.equal(calls, 1);
  const snapshot = session.view(); snapshot.outcome.path = 'changed';
  assert.equal(session.view().outcome.path, destination);
});

test('panel compiles and exposes source options, independent gate, cancellable reads and persistent publication barriers', () => {
  const source = readFileSync(path.join(__dirname, 'LifeformWorkbookPanel.svelte'), 'utf8');
  const compiled = compile(source, { filename: 'LifeformWorkbookPanel.svelte', generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  assert.match(source, /VITE_LIFEFORM_WORKBOOK === 'true'/);
  assert.match(source, /details = \$state<boolean\[\]>\(\[false, false, false, false, false, false\]\)/);
  assert.match(source, /onBusyChange\(busy \|\| publication.busy \|\| publication.blocked\)/);
  assert.match(source, /reads\.track\(LifeformWorkbookService\.GetReview/);
  assert.doesNotMatch(source, /reads\.track\(LifeformWorkbookService\.ExportReviewed/);
  assert.match(source, /disposed \|\| requestGeneration !== generation/);
  assert.match(source, /session\.publish\(review, destination/);
  assert.match(source, /session\.acknowledge\(\); review = null/);
  assert.match(source, /role="alert"/);
  assert.match(source, /before acknowledging; never repeat/);
  for (const selector of ['review', 'cancel', 'destination', 'export', 'acknowledge', 'option']) {
    assert.ok(source.includes(`data-lifeform-workbook-${selector}`));
  }
  assert.ok(source.indexOf('publication.error') < source.indexOf('<fieldset'));
  assert.ok(source.indexOf('data-lifeform-workbook-receipt') < source.indexOf('class="guidance"'));
  assert.match(source, /attributeUnit.rows as row/);
});
