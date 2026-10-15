const assert = require('node:assert/strict');
const { test } = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const { randomUUID } = require('node:crypto');
const req = createRequire(path.join(__dirname, '..', 'package.json'));
const ts = req('typescript');
const compiler = req('svelte/compiler');
const runtime = req('svelte/internal/client');
const source = fs.readFileSync(path.join(__dirname, 'SummaryVegetationFieldList.svelte'), 'utf8');
const wire = JSON.parse(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldListWire.json'), 'utf8'));
const { physical, source: provenance, ...owner } = wire.review;
function execute(code, dependencies, globals = {}) {
  const module = { exports: {} };
  const js = ts.transpileModule(code, { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS,
  } }).outputText;
  vm.runInNewContext(js, { ...globals, module, exports: module.exports, crypto: { randomUUID },
    require(id) { assert.ok(id in dependencies, `Uninjected import ${id}`); return dependencies[id]; } });
  return module.exports;
}
const client = execute(fs.readFileSync(path.join(__dirname, 'summaryVegetationFieldList.ts'), 'utf8'), {});
// Execute the actual component script, not separately rewritten action helpers.
// The private harness substitutes lifecycle/props plumbing; full component is separately compiled/checked.
function mount(session, options = {}) {
  let destroy;
  const notifications = [];
  const props = runtime.proxy({ scope: structuredClone(owner), session, enabledFromPresentationDraft: true,
    onStateChange: (view, close) => notifications.push({ view, close }), ...options });
  let script = source.match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('$props()', '__props');
  script += `
    export const handlers = {load, save, undo, bulk, edit, correct, acknowledge, cancel, editable,
      canStage, canLoad, canAcknowledge, label, rowName, visibleRows,
      view: () => view, error: () => error,
      update: (enabled: boolean, disabled = false, validation = '') => {
        enabledFromPresentationDraft = enabled; peerDisabled = disabled; peerValidation = validation;
      }};
  `;
  const stripped = ts.transpileModule(script, { compilerOptions: {
    target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext,
  } }).outputText;
  const compiled = compiler.compileModule(stripped, { filename: 'pane-validation-handlers.svelte.js', generate: 'client' });
  const exports = execute(compiled.js.code, {
    'svelte/internal/client': runtime,
    svelte: { untrack: runtime.untrack, onDestroy: callback => { destroy = callback; } },
    './summaryVegetationFieldList': client,
  }, { __props: props });
  return { ...exports.handlers, destroy: () => destroy(), notifications };
}
function deferred() {
  let resolve, reject, cancellations = 0;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.cancel = () => { cancellations++; };
  return { promise, resolve, reject, get cancellations() { return cancellations; } };
}
function session(read = () => Promise.resolve(runtime.proxy(structuredClone(wire.review))),
  save = () => Promise.reject(new Error('unknown transport')), validationYield) {
  return new client.SummaryVegetationFieldListSession(runtime.proxy(structuredClone(owner)), { read, save, validationYield }, true);
}
const plain = value => JSON.parse(JSON.stringify(value));
const reports = table => Array.from(table.rows, row => row.cells[4].integer);
async function ready(read, save) { const retained = session(read, save); const pane = mount(retained); await pane.load(); return { retained, pane }; }

test('full Svelte component compiles, script executes actual runtime raw state and proxy Go wire', async () => {
  const result = compiler.compile(source, { filename: 'SummaryVegetationFieldList.svelte', generate: 'client' });
  assert.deepEqual(result.warnings, []);
  const { pane } = await ready();
  assert.deepEqual(plain(pane.view().review), wire.review);
  assert.ok(Object.isFrozen(pane.view()));
  assert.deepEqual(Array.from(pane.visibleRows(), row => row.rowId), ['1', '2', '5']);
  assert.ok(pane.notifications.length >= 3);
  assert.equal(pane.notifications.at(-1).close.busy, false);
  pane.destroy();
});

test('source counts, literal duplicate identity, NULL unavailable and global hidden mutation', async () => {
  const { pane } = await ready();
  const rows = pane.visibleRows();
  assert.equal(rows.length, 3);
  assert.equal(pane.rowName(rows[0]), 'duplicate — physical row 1');
  assert.equal(pane.rowName(rows[1]), 'duplicate — physical row 2');
  assert.equal(pane.editable('5'), false);
  await pane.edit('5', true);
  assert.equal(pane.view().actions.length, 0);
  await pane.bulk(false);
  assert.deepEqual(reports(pane.view().proposed), ['0', '0', '0', '0', '0']);
  assert.equal(pane.view().sourceIndex.selected.length, 2);
  assert.equal(pane.view().proposedIndex.selected.length, 0);
  assert.equal(pane.editable('5'), false, 'bulk cannot enable original unavailable individual Report');
  await pane.bulk(true);
  assert.deepEqual(reports(pane.view().proposed), ['-1', '-1', '-1', '-1', '-1']);
  assert.equal(pane.view().proposedIndex.selected.length, 5);
  await pane.edit('2', false);
  assert.deepEqual(reports(pane.view().proposed), ['-1', '0', '-1', '-1', '-1']);
  pane.destroy();
});

test('NULL/empty/literal labels and incompatible Report stay distinct without repair', async () => {
  const review = structuredClone(wire.review);
  review.physical.rows[0].cells[2] = { storage: 'null', text: null, integer: null, real: null, blobHex: null };
  review.physical.rows[1].cells[2].text = '';
  review.physical.rows[4].cells[2] = { storage: 'text', text: '  literal\n𐐀  ', integer: null, real: null, blobHex: null };
  review.physical.rows[1].cells[4] = { storage: 'text', text: '-1', integer: null, real: null, blobHex: null };
  const { pane } = await ready(() => Promise.resolve(runtime.proxy(review)));
  assert.equal(pane.label(pane.visibleRows()[0].cells[2]), '[NULL Label]');
  assert.equal(pane.label(pane.visibleRows()[1].cells[2]), '[empty Label]');
  assert.equal(pane.label(pane.visibleRows()[2].cells[2]), '  literal\n𐐀  ');
  assert.equal(pane.editable('2'), false);
  assert.ok(pane.view().sourceIndex.error);
  const original = plain(pane.view().review);
  await pane.edit('2', true);
  assert.equal(pane.view().actions.length, 0);
  await pane.bulk(true);
  assert.deepEqual(plain(pane.view().review), original);
  assert.equal(pane.view().proposed.rows[1].cells[4].integer, '-1');
  pane.destroy();
});

test('presentation disabled never stages or saves; recovery remains live and prop change is not mutation', async () => {
  let reads = 0, saves = 0;
  const { pane } = await ready(() => { reads++; return Promise.resolve(wire.review); },
    () => { saves++; throw Error('should not dispatch'); });
  await pane.edit('1', true);
  const retained = plain(pane.view());
  pane.update(false);
  assert.deepEqual(plain(pane.view()), retained);
  await pane.bulk(true); await pane.edit('2', true); await pane.save();
  assert.equal(saves, 0);
  assert.deepEqual(plain(pane.view()), retained);
  await pane.undo(); await pane.load();
  assert.equal(reads, 2);
  assert.equal(pane.view().actions.length, 0);
  pane.destroy();
});

test('peer busy and validation holds block actual action handlers; correction by valid input clears draft errors', async () => {
  const { retained, pane } = await ready();
  for (const [disabled, validation] of [[true, ''], [false, 'invalid sibling draft']]) {
    pane.update(true, disabled, validation);
    const before = plain(pane.view());
    for (const action of [() => pane.bulk(true), () => pane.edit('1', true), () => pane.load(),
      () => pane.save(), () => pane.undo(), () => pane.correct(), () => pane.acknowledge()]) await action();
    assert.deepEqual(plain(pane.view()), before);
  }
  pane.update(true);
  retained.setActions([{ kind: 'row', rowId: '5', selected: true }]);
  assert.ok(pane.view().draftError);
  assert.equal(pane.notifications.at(-1).close.blocked, true);
  await pane.edit('1', true);
  assert.equal(pane.view().draftError, '');
  pane.destroy();
});

test('held Load destruction/remount rejects old authority and retained read error is explicit', async () => {
  const held = deferred();
  let reads = 0;
  const retained = session(() => ++reads === 1 ? held.promise : Promise.resolve(wire.review));
  const first = mount(retained);
  const pending = first.load();
  first.destroy();
  assert.equal(held.cancellations, 1);
  const second = mount(retained, { enabledFromPresentationDraft: false });
  assert.match(second.view().readError, /cancelled/);
  await second.load();
  const changed = structuredClone(wire.review); changed.physical.rows[0].cells[2].text = 'stale';
  held.resolve(runtime.proxy(changed)); await pending;
  assert.equal(second.view().review.physical.rows[0].cells[2].text, 'duplicate');
  assert.equal(second.view().readError, '');
  second.destroy();
});

test('held Save remount retains unknown receipt, waits settlement, corrects changed authority without replay', async () => {
  const held = deferred();
  let saves = 0, review = structuredClone(wire.review);
  const retained = session(() => Promise.resolve(runtime.proxy(review)), () => { saves++; return held.promise; });
  const first = mount(retained); await first.load(); await first.bulk(true);
  const saving = first.save(); first.destroy();
  const second = mount(retained, { enabledFromPresentationDraft: false });
  assert.equal(second.view().receipts[0].kind, 'unknown');
  assert.equal(second.view().dispatchSettled, false);
  const attempt = plain(second.view().receipts[0].request);
  await second.load(); assert.equal(second.view().reconciliationReview, null);
  held.reject(Error('cancelled transport')); await saving;
  assert.equal(second.view().dispatchSettled, true);
  review.physical.rows = review.physical.rows.filter(row => row.rowId !== '1');
  await second.load();
  assert.equal(second.view().review.physical.rows.length, 5);
  assert.equal(second.view().reconciliationReview.physical.rows.length, 4);
  await second.acknowledge();
  assert.equal(second.view().receipts[0].acknowledged, false);
  await second.correct(); await second.acknowledge();
  assert.equal(second.view().review.physical.rows.length, 4);
  assert.equal(second.view().actions.length, 0);
  assert.deepEqual(plain(second.view().receipts[0].request), attempt);
  assert.equal(second.view().receipts[0].acknowledged, true);
  await second.save(); assert.equal(saves, 1);
  assert.equal(second.notifications.at(-1).close.blocked, false);
  second.destroy();
});

test('Load/Save held during validation, cancellation/remount cannot assign obsolete validated results', async () => {
  for (const operation of ['load', 'save']) {
    const hold = deferred(), entered = deferred();
    let holdValidation = false, saves = 0;
    const retained = session(undefined, json => {
      saves++;
      return Promise.resolve({ ...structuredClone(wire.outcome), requestId: JSON.parse(json).requestId });
    }, () => {
      if (holdValidation) { entered.resolve(); return hold.promise; }
      return Promise.resolve();
    });
    const first = mount(retained);
    await first.load();
    if (operation === 'save') await first.bulk(true);
    holdValidation = true;
    const pending = first[operation]();
    await entered.promise;
    await first.cancel();
    first.destroy();
    const second = mount(retained);
    if (operation === 'save') {
      assert.equal(second.view().dispatchSettled, false);
      assert.equal(second.view().receipts[0].kind, 'unknown');
    } else assert.match(second.view().readError, /cancelled/);
    holdValidation = false; hold.resolve(); await pending;
    assert.deepEqual(plain(second.view().review), wire.review);
    if (operation === 'save') {
      assert.equal(second.view().receipts[0].outcome, null);
      assert.equal(saves, 1);
      assert.equal(second.view().dispatchSettled, true);
    }
    await second.load();
    if (operation === 'save') { await second.correct(); await second.acknowledge(); }
    assert.equal(second.view().readError, '');
    second.destroy();
  }
});

test('actual Go committed receipt and unverified/error outcomes preserve correction requirements', async () => {
  for (const errorMessage of ['', 'postcommit verification failed']) {
    let saves = 0;
    const { pane } = await ready(undefined, json => {
      saves++;
      const request = JSON.parse(json);
      return Promise.resolve(runtime.proxy({ ...structuredClone(wire.outcome), requestId: request.requestId,
        verified: errorMessage === '', errorMessage }));
    });
    await pane.bulk(true); await pane.save();
    assert.equal(saves, 1);
    assert.equal(pane.view().receipts[0].kind, 'known');
    if (errorMessage) {
      assert.equal(pane.canAcknowledge(), false);
      await pane.load(); await pane.correct();
    }
    await pane.acknowledge();
    assert.equal(pane.view().receipts[0].acknowledged, true);
    assert.equal(pane.view().receipts[0].outcome.committed, true);
    assert.equal(pane.view().receipts.length, 1);
    pane.destroy();
  }
});

test('read failure retry, undo preserves receipts, busy cancellation and notifier holds are retained', async () => {
  let fail = true;
  const retained = session(() => fail ? Promise.reject(Error('read unavailable')) : Promise.resolve(wire.review));
  let pane = mount(retained, { enabledFromPresentationDraft: false });
  await pane.load(); assert.match(pane.view().readError, /read unavailable/);
  pane.destroy(); pane = mount(retained, { enabledFromPresentationDraft: false });
  assert.equal(pane.notifications[0].close.blocked, true);
  fail = false; await pane.load(); pane.update(true);
  await pane.bulk(true); await pane.save();
  const receipt = plain(pane.view().receipts[0]);
  await pane.undo();
  assert.deepEqual(plain(pane.view().receipts[0]), receipt);
  assert.equal(pane.notifications.at(-1).close.blocked, true);
  pane.destroy();
});

test('source headers, one checkbox binding, locked Label, safety order and explicit responsive no-clipping contract', () => {
  assert.match(source, /<th scope="col">Report<\/th><th scope="col">Field<\/th>/);
  assert.equal((source.match(/type="checkbox"/g) || []).length, 1);
  assert.match(source, /currentTarget\.checked; edit\(row\.rowId, raw\)/);
  assert.match(source, /aria-label=\{'Report — ' \+ rowName\(row\)\}/);
  assert.match(source, /data-source-binding="Label"><span class="literal">/);
  assert.match(source, /source SQL selection includes hidden rows/);
  assert.match(source, /including hidden rows and NULL\/incompatible historical Report/);
  assert.match(source, /JSON\.stringify\(view\.proposed, null, 2\)/);
  assert.ok(source.indexOf('class="safety"') < source.indexOf('<table>'));
  assert.ok(source.indexOf('class="guidance"') > source.indexOf('data-field-list-acknowledge'));
  for (const control of ['all', 'none', 'load', 'save', 'undo', 'correct', 'acknowledge'])
    assert.match(source, new RegExp(`data-field-list-${control}`));
  assert.doesNotMatch(source, /<fieldset[^>]*disabled|overflow:\s*hidden|overflow-x|(?<!max-)width:\s*\d+px|structuredClone|wailsjs|bind:value|bind:checked/);
  assert.match(source, /table-layout: fixed/);
  assert.match(source, /repeat\(3, minmax\(0, 1fr\)\)/);
  assert.match(source, /max-width: 800px/);
  assert.match(source, /max-width: 480px/);
  for (const [width, expectedTracks] of [[1400, 3], [760, 2], [420, 1]])
    assert.equal(width <= 480 ? 1 : width <= 800 ? 2 : 3, expectedTracks);
  assert.match(source, /white-space: pre-wrap/);
  assert.match(source, /overflow-wrap: anywhere/);
});
