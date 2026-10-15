const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { proxy } = require('svelte/internal/client');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const lookup = loadTypeScript('scopedPlotLookup.ts', { './qualityEditor': quality });
const labels = loadTypeScript('plotLabelPreview.ts', { './qualityEditor': quality, './scopedPlotLookup': lookup });
const regional = loadTypeScript('plotLabelRegional.ts', { './qualityEditor': quality, './plotLabelPreview': labels });
const wire = JSON.parse(readFileSync(path.join(__dirname, 'plotLabelRegionalWire.json'), 'utf8'));
const plain = value => JSON.parse(JSON.stringify(value));
function host(operation = () => Promise.resolve(wire)) {
  const calls = [], changes = [];
  const context = componentFunctions('PlotLabelPreview.svelte', ['cancel', 'load', 'updateBusy'], {
    contextId: 'label-wire', project: 'Sample', ownerKey: 'label-wire-Sample', plot: 'LB001',
    preview: null, labelDate: '', regionalEvidence: null, regionalDateEnabled: true,
    pdfEnabled: false, pdfReview: null, publication: { busy: false, blocked: false },
    error: '', busy: false, generation: 0,
    plotLookupInputError: lookup.plotLookupInputError, plotLabelFromHeader: labels.plotLabelFromHeader,
    validatePlotLabelRegionalPreview: regional.validatePlotLabelRegionalPreview,
    Date: class { toLocaleDateString() { throw new Error('browser fallback forbidden'); } },
    onBusyChange(value) { changes.push(value); },
    reads: { track(value) { return value; }, cancelAll() { calls.push('cancel'); } },
    ContextService: { GetPlot() { throw new Error('legacy fallback forbidden'); } },
    PlotLabelRegionalService: { GetPreview(...args) { calls.push(['regional', ...args]); return operation(...args); } },
  });
  return { context, calls, changes };
}

test('strict-Go regional label wire retains raw assignments and independently expected date metadata', () => {
  const result = regional.validatePlotLabelRegionalPreview(wire, 'label-wire', 'LB001');
  assert.deepEqual(plain(result.label), {
    plotNumber1: 'LB001', zone1: 'CWH dm/01', representing1: '  source representing  ',
    projectId1: ' literal ID ', displayPlotNumber: 'LB001', displayRepresenting: 'source representing',
  });
  assert.equal(result.labelDate, 'Sunday, October 11, 2026');
  assert.deepEqual(plain(result.evidence), {
    isoDate: '2026-10-11', locale: 'en-US', pattern: 'dddd, MMMM d, yyyy', source: 'windows-user-long-date',
  });
  assert.equal(Object.isFrozen(result), true);
  assert.equal(Object.isFrozen(result.label), true);
  assert.equal(Object.isFrozen(result.evidence), true);
});

test('actual Svelte regional DTO proxy is detached without cloning or caller references', () => {
  const source = proxy(structuredClone(wire));
  assert.throws(() => structuredClone(source));
  const result = regional.validatePlotLabelRegionalPreview(source, 'label-wire', 'LB001');
  source.header.projectId = 'caller';
  source.labelDate = 'caller';
  source.pattern = 'caller';
  assert.equal(result.label.projectId1, ' literal ID ');
  assert.equal(result.labelDate, 'Sunday, October 11, 2026');
  assert.equal(result.evidence.pattern, 'dddd, MMMM d, yyyy');
});

test('regional labels refuse wrong owner, sparse or malformed wire and invalid calendar dates without repair', () => {
  const changes = [
    value => { value.contextId = 'other'; }, value => { value.plotNumber = 'other'; },
    value => { value.header.plotNumber = 'other'; }, value => { value.source = 'browser'; },
    value => { value.extra = true; }, value => { delete value.pattern; },
    value => { value.header.extra = true; }, value => { delete value.header.projectId; },
    value => { value.header.zone = null; }, value => { value.header.projectId = '\ud800'; },
    value => { value.labelDate = ''; }, value => { value.labelDate = '\udc00'; },
    value => { value.pattern = '\0'; }, value => { value.locale = 'x'.repeat(85); },
    value => { value.isoDate = '2026-02-29'; }, value => { value.isoDate = '1900-02-29'; },
    value => { value.isoDate = '2026-13-01'; }, value => { value.isoDate = '2026-01-00'; },
    value => { value.isoDate = '1600-01-01'; }, value => { value.isoDate = '2026-1-01'; },
  ];
  for (const change of changes) {
    const value = structuredClone(wire); change(value);
    assert.throws(() => regional.validatePlotLabelRegionalPreview(value, 'label-wire', 'LB001'));
  }
  for (const isoDate of ['2000-02-29', '2024-02-29', '1601-01-01', '9999-12-31']) {
    const value = structuredClone(wire); value.isoDate = isoDate;
    assert.equal(regional.validatePlotLabelRegionalPreview(value, 'label-wire', 'LB001').evidence.isoDate, isoDate);
  }
});

test('native label loader uses exactly its reviewed RPC and preserves explicit backend/date failures', async () => {
  const { context, calls, changes } = host();
  await context.actions.load();
  assert.deepEqual(calls.filter(Array.isArray), [['regional', 'label-wire', 'LB001']]);
  assert.equal(context.preview.plotNumber1, 'LB001');
  assert.equal(context.labelDate, 'Sunday, October 11, 2026');
  assert.equal(context.regionalEvidence.locale, 'en-US');
  assert.equal(context.busy, false);
  assert.equal(changes.at(-1), false);
  for (const message of ['backend gate disabled', 'native calendar unavailable', 'source UTF-8 malformed']) {
    const failed = host(() => Promise.reject(new Error(message))).context;
    await failed.actions.load();
    assert.match(failed.error, new RegExp(message));
    assert.equal(failed.preview, null);
    assert.equal(failed.labelDate, '');
    assert.equal(failed.regionalEvidence, null);
    assert.equal(failed.busy, false);
  }
});

test('held native label completion is discarded by cancel, owner change and teardown with correction retry', async () => {
  for (const change of ['cancel', 'owner']) {
    let release;
    const { context } = host(() => new Promise(resolve => { release = resolve; }));
    const pending = context.actions.load();
    assert.equal(context.busy, true);
    if (change === 'cancel') context.actions.cancel(); else context.ownerKey = 'new-owner';
    release(wire);
    await pending;
    assert.equal(context.preview, null);
    assert.equal(context.labelDate, '');
    assert.equal(context.regionalEvidence, null);
  }
  let release;
  const { context } = host(() => new Promise(resolve => { release = resolve; }));
  const pending = context.actions.load();
  context.actions.cancel();
  release(wire);
  await pending;
  context.PlotLabelRegionalService.GetPreview = () => Promise.resolve(wire);
  await context.actions.load();
  assert.equal(context.labelDate, 'Sunday, October 11, 2026');
  const source = readFileSync(path.join(__dirname, 'PlotLabelPreview.svelte'), 'utf8');
  assert.match(source, /onDestroy\(\(\) => \{ unsubscribe\?\.\(\); cancel\(\); \}\)/);
});

test('native label validation cannot assign after its generation is invalidated', async () => {
  const { context } = host();
  context.validatePlotLabelRegionalPreview = (...args) => {
    context.actions.cancel();
    return regional.validatePlotLabelRegionalPreview(...args);
  };
  await context.actions.load();
  assert.equal(context.preview, null);
  assert.equal(context.labelDate, '');
  assert.equal(context.regionalEvidence, null);
});
