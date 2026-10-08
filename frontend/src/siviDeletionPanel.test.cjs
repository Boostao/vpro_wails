const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { cell, metadata } = require('./siviParentTestHelpers.cjs');
const historical = serverComponent(readFileSync(path.join(__dirname, 'SIVIHistoricalRow.svelte'), 'utf8'),
  'SIVIHistoricalRow.svelte', { './projectMetadataEditor': metadata });
const panels = {};
for (const name of ['SIVIDeletionPanel.svelte', 'SIVIDeletionRestorationPanel.svelte']) {
  panels[name] = serverComponent(readFileSync(path.join(__dirname, name), 'utf8'), name,
    { './SIVIHistoricalRow.svelte': { default: historical } });
}
const defaults = require('../../resources/sivi-new-row-defaults.json');
function props() {
  const columns = [...defaults.initialNullColumns, 'Flag', 'ID'].map(name => ({ name, declaredType: 'TEXT' }));
  return {
    view: { original: { contextId: 'C', project: 'Sample', plot: 'P', form: 'SubVegA-SIVI_BC',
      columns, original: { rowId: '1', cells: columns.map(column => column.name === 'Species'
        ? cell('text', '  historical overlength  ') : column.name === 'Flag' ? cell('integer', '1') : cell()) } },
      confirmed: false, busy: false, blocked: false, error: null, requestId: null, receipt: null, revision: 0 },
    close: { unsaved: true, busy: false, blocked: false, canSave: false, saveReason: 'Confirm deletion', error: null },
    targets: [{ form: 'SubVegA-SIVI_BC', rowId: '1', label: 'Tree/Shrubs: TREE; physical row 1', unavailable: null }],
    onreview() {}, onconfirm() {}, onoperation() {},
  };
}
function html(value, name = 'SIVIDeletionPanel.svelte') { return render(panels[name], { props: value }).body; }
function disabled(markup, attribute) {
  const tag = markup.match(new RegExp(`<[^>]*${attribute}[^>]*>`));
  assert.ok(tag, attribute);
  return /\sdisabled(?:\s|=|>)/.test(tag[0]);
}

test('private deletion presentation exposes all44 raw cells and a visible row-specific confirmation label', () => {
  const markup = html(props());
  assert.equal((markup.match(/data-sivi-historical-cell=/g) ?? []).length, 44);
  assert.match(markup, /  historical overlength  /);
  assert.match(markup, /data-sivi-historical-cell="Flag">1</);
  assert.match(markup, /<label[^>]*>[\s\S]*data-sivi-deletion-confirm[\s\S]*Confirm deletion of physical row 1/);
  assert.equal(disabled(markup, 'data-sivi-deletion-save'), true);
  assert.equal(disabled(markup, 'data-sivi-deletion-review'), true);
  const value = props(); value.view.confirmed = true; value.close.canSave = true;
  assert.equal(disabled(html(value), 'data-sivi-deletion-save'), false);
});

test('unknown deletion disables replay/Undo but preserves explicitly read-only recovery and safety feedback above fields', () => {
  const value = props();
  value.view.blocked = true; value.view.error = 'Unresolved retained UUID'; value.view.requestId = 'stable';
  const markup = html(value);
  assert.equal(disabled(markup, 'data-sivi-deletion-save'), true);
  assert.equal(disabled(markup, 'data-sivi-deletion-undo'), true);
  assert.equal(disabled(markup, 'data-sivi-deletion-confirm'), true);
  assert.equal(disabled(markup, 'data-sivi-deletion-resolve'), false);
  assert.ok(markup.indexOf('Unresolved retained UUID') < markup.indexOf('data-sivi-deletion-original'));
  assert.ok(markup.indexOf('Deletion preserves complete') > markup.indexOf('data-sivi-deletion-confirm'));
  value.view.busy = true;
  const busy = html(value);
  assert.equal(disabled(busy, 'data-sivi-deletion-resolve'), true);
  assert.match(busy, /data-sivi-deletion-cancel/);
});

test('disabled private panel never offers a write despite a previously valid review', () => {
  const value = props();
  value.disabled = true; value.view.confirmed = true; value.close.canSave = true;
  const markup = html(value);
  assert.equal(disabled(markup, 'data-sivi-deletion-save'), true);
  assert.equal(disabled(markup, 'data-sivi-deletion-confirm'), true);
});

test('historical NULL/empty/space text remains distinguishable in shared raw-row presentation', () => {
  const value = props();
  value.view.original.original.cells[2] = cell('text', '');
  const markup = html(value);
  assert.match(markup, /data-sivi-historical-cell="Layer">\(empty text\)</);
  assert.match(markup, /data-sivi-historical-cell="Cover1">NULL</);
  assert.match(markup, /whitespace-pre-wrap/);
});

test('private restoration requires an explicit audit action and confirmation, keeping consumed histories unavailable', () => {
  const source = props(), value = { ...source, view: { review: { ...source.view.original,
    historyId: 'deleted', rowId: '1', id: -1, auditsBefore: [] }, action: null, confirmed: false,
    busy: false, blocked: false, error: null, receipt: null, requestId: null },
    targets: [{ historyId: 'old', label: 'Species TREE; physical row 1', restored: true }],
    onchoose() {},
  };
  const name = 'SIVIDeletionRestorationPanel.svelte', markup = html(value, name);
  assert.equal((markup.match(/data-sivi-historical-cell=/g) ?? []).length, 44);
  assert.equal(disabled(markup, 'data-sivi-restoration-save'), true);
  assert.equal(disabled(markup, 'data-sivi-restoration-confirm'), true);
  assert.equal(disabled(markup, 'data-sivi-restoration-review'), true);
  assert.match(markup, /Already restored:/);
  value.view.action = 'retain'; value.view.confirmed = true; value.close.canSave = true;
  assert.equal(disabled(html(value, name), 'data-sivi-restoration-save'), false);
  value.view.blocked = true;
  assert.equal(disabled(html(value, name), 'data-sivi-restoration-save'), true);
  assert.equal(disabled(html(value, name), 'data-sivi-restoration-undo'), true);
  assert.equal(disabled(html(value, name), 'data-sivi-restoration-resolve'), false);
});

test('restoration presents missing history separately from an existing empty owned history', () => {
  for (const present of [false, true]) {
    const value = { ...props(), view: { review: null, history: { historyPresent: present, events: [] },
      busy: false, blocked: false, receipt: null, error: null }, targets: [], onchoose() {} };
    const markup = html(value, 'SIVIDeletionRestorationPanel.svelte');
    assert.match(markup, present ? /Durable deletion history exists, with no events for this owned plot/
      : /No durable deletion history exists for this project.*This does not resolve an unknown request/);
  }
});
