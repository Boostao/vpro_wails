const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { cell, metadata, owner, historyId, requestId, review, history, result } = require('./siviCreationUndoTestHelpers.cjs');
const historical = serverComponent(readFileSync(path.join(__dirname, 'SIVIHistoricalRow.svelte'), 'utf8'),
  'SIVIHistoricalRow.svelte', { './projectMetadataEditor': metadata });
const source = readFileSync(path.join(__dirname, 'SIVICreationUndoPanel.svelte'), 'utf8');
const panel = serverComponent(source, 'SIVICreationUndoPanel.svelte', { './SIVIHistoricalRow.svelte': { default: historical } });
function props() {
  return { view: { history: history(), review: review(), action: null, confirmed: false, busy: false,
    blocked: false, error: null, requestId: null, receipt: null, revision: 0 },
    close: { unsaved: true, busy: false, blocked: false, canSave: false, saveReason: 'Confirm', error: null },
    onreview() {}, onchoose() {}, onconfirm() {}, onoperation() {} };
}
const html = props => render(panel, { props }).body;
function disabled(markup, name) {
  const tag = markup.match(new RegExp(`<[^>]*data-sivi-creation-undo-${name}(?:[\\s=>])[^>]*>`));
  assert.ok(tag, name); return /\sdisabled(?:\s|=|>)/.test(tag[0]);
}
test('all44 exact historical cells, NULL/empty/storage and responsive layout; confirmation visibly row-specific', () => {
  const value = props();
  value.view.review.committed.cells[value.view.review.columns.findIndex(c => c.name === 'Other1')] = cell('text', '');
  const markup = html(value);
  assert.equal((markup.match(/data-sivi-historical-cell=/g) ?? []).length, 44);
  assert.match(markup, /\(empty text\)/); assert.match(markup, />NULL</);
  assert.match(markup, /9223372036854775806/);
  assert.match(markup, /grid-cols-1[^"]*sm:grid-cols-2[^"]*lg:grid-cols-3/);
  assert.match(markup, /<label[^>]*>[\s\S]*data-sivi-creation-undo-confirm[\s\S]*Confirm removing physical row 9223372036854775806/);
  assert.match(markup, /application ID 1/); assert.match(markup, /all44 committed fields/);
  assert.equal(disabled(markup, 'submit'), true); assert.equal(disabled(markup, 'confirm'), true);
  assert.equal(disabled(markup, 'review'), true);
  value.view.action = 'retain'; value.view.confirmed = true; value.close.canSave = true;
  assert.equal(disabled(html(value), 'submit'), false);
  assert.match(html(value), /selected retain audit action/);
});
test('unloaded, missing, existing empty and consumed history remain distinct without UUID input', () => {
  const value = props(); value.view.review = null; value.view.history = null;
  assert.match(html(value), /has not been loaded/);
  value.view.history = { ...owner, historyPresent: false, events: [] };
  assert.match(html(value), /No durable creation history exists/);
  value.view.history.historyPresent = true;
  assert.match(html(value), /Durable creation history exists, with no events/);
  value.view.history = history(true);
  assert.match(html(value), /Already consumed:/);
  assert.equal(disabled(html(value), 'review'), true);
  value.view.history = history(false);
  assert.equal(disabled(html(value), 'review'), false);
  assert.doesNotMatch(source, /type="text"|bind:value|targets:/);
});
test('legacy history stays visible with its diagnostic and disabled review without disabling available UUID choices', () => {
  const value = props(); value.view.review = null;
  value.view.history.events.push({ ...value.view.history.events[0], historyId: 'legacy creation identity',
    reviewAvailable: false, unavailableReason: 'Legacy identity requires an unavailable protocol.' });
  const markup = html(value);
  assert.match(markup, /Unavailable creation:/);
  assert.match(markup, /legacy creation identity/);
  assert.match(markup, /Legacy identity requires an unavailable protocol/);
  assert.match(markup, /not an inference of current row eligibility/);
  const tags = [...markup.matchAll(/<button[^>]*data-sivi-creation-undo-review="([^"]+)"[^>]*>/g)];
  assert.equal(tags.length, 2);
  assert.equal(tags[0][1], historyId);
  assert.equal(/\sdisabled(?:\s|=|>)/.test(tags[0][0]), false);
  assert.equal(/\sdisabled(?:\s|=|>)/.test(tags[1][0]), true);
});
test('unknown outcome safety feedback precedes fields and blocks write/discard while permitting read-only recovery', () => {
  const value = props(); value.view.blocked = true; value.view.error = 'Exact retained request unknown';
  value.view.requestId = requestId; value.close.canSave = true;
  const markup = html(value);
  for (const control of ['history', 'review', 'submit', 'discard', 'confirm', 'action']) assert.equal(disabled(markup, control), true);
  assert.equal(disabled(markup, 'resolve'), false);
  assert.ok(markup.indexOf(value.view.error) < markup.indexOf('data-sivi-creation-undo-committed'));
  assert.ok(markup.indexOf('independent private safety adaptation') > markup.indexOf('data-sivi-creation-undo-confirm'));
  value.view.busy = true; assert.equal(disabled(html(value), 'resolve'), true);
  assert.match(html(value), /Cancel operation acknowledgement/);
});
test('editing and per-peer recovery disables are independent', () => {
  const value = props(); value.view.action = 'retain'; value.view.confirmed = true; value.close.canSave = true;
  value.disabled = true;
  assert.equal(disabled(html(value), 'submit'), true);
  assert.equal(disabled(html(value), 'discard'), false);
  value.view.blocked = true;
  assert.equal(disabled(html(value), 'resolve'), false);
  value.recoveryDisabled = true;
  assert.equal(disabled(html(value), 'resolve'), true);
});
test('verified lookup receipt describes observation, not replay; distinct explicit callbacks and associated labels', () => {
  const value = props();
  value.view.receipt = result(review(), { ...owner, requestId, historyId, expected: review().expected, action: 'prune' }, true);
  value.view.review = null;
  assert.match(html(value), /Observed an earlier commit; no Undo was repeated/);
  for (const operation of ['history', 'undo', 'discard', 'resolve', 'cancel']) {
    assert.ok(source.includes(`onoperation('${operation}')`), operation);
  }
  assert.match(source, /onreview\(target.historyId\)/);
  for (const action of ['retain', 'prune']) {
    assert.match(source, new RegExp(`<label[^>]*>[\\s\\S]*?data-sivi-creation-undo-action="${action}"[\\s\\S]*?</label>`));
    assert.ok(source.includes(`onchoose('${action}')`));
  }
  assert.match(source, /onconfirm\(event.currentTarget.checked\)/);
  assert.doesNotMatch(source, /onoperation\('save'\)/);
});
