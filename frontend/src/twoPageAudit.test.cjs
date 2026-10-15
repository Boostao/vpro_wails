const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { render } = require('svelte/server');
const { loadTypeScript, componentFunctions, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const audit = loadTypeScript('twoPageAudit.ts', {
  './siviParentTransport': transport, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const wire = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageAuditWire.json'), 'utf8'));
const owner = { contextId: 'wire-owned', project: 'Sample', plot: '108050', form: 'FS882-8x6XL' };

test('actual Go round-trip wire preserves NULL, empty text, BOOLEAN storage and physical identity', () => {
  const original = wire(), before = JSON.stringify(original);
  const review = audit.twoPageAuditFromWire(original, owner);
  assert.equal(review.records.rows[0].rowId, '1');
  const row = review.records.rows[0];
  for (const [column, expected] of [['ID', '(NULL)'], ['BeforeEdit', '(NULL)'],
    ['AfterEdit', '(empty text)'], ['Restore', 'Yes (-1)'], ['Flag', 'No (0)'], ['User', 'source user']]) {
    assert.equal(audit.twoPageAuditLabel(review, row, column), expected);
  }
  row.cells[1].text = 'caller copy';
  assert.equal(JSON.stringify(original), before);
  for (const [column, value, expected] of [['ID', cell('integer', '2147483647'), '2147483647'],
    ['Restore', cell('integer', '2'), '2'], ['Flag', cell('real', '2.5'), '2.5'],
    ['BeforeEdit', cell('blob', 'FF00'), 'hex:FF00']]) {
    row.cells[review.records.columns.findIndex(item => item.name === column)] = value;
    assert.equal(audit.twoPageAuditLabel(review, row, column), expected);
  }
});

const invalid = [
  ['foreign context', value => value.contextId = 'other'],
  ['foreign project', value => value.project = 'other'],
  ['foreign literal plot', value => value.plot = '0108050'],
  ['foreign form', value => value.form = 'FS882-6x4XL'],
  ['foreign source', value => value.recordSource = 'Audit'],
  ['foreign physical table', value => value.table = 'Other_Audit'],
  ['missing source binding', value => value.records.columns[0].name = 'project'],
  ['duplicate binding', value => value.records.columns[1].name = 'Project'],
  ['wrong production column wire', value => { value.records.columns[0].type = 'TEXT'; delete value.records.columns[0].declaredType; }],
  ['foreign physical project', value => value.records.rows[0].cells[0].text = 'Other'],
  ['numeric plot ownership', value => value.records.rows[0].cells[2] = cell('integer', '108050')],
  ['empty plot ownership', value => value.records.rows[0].cells[2] = cell('text', '')],
  ['malformed source text', value => value.records.rows[0].cells[1].text = '\ud800'],
  ['incomplete row', value => value.records.rows[0].cells.pop()],
  ['duplicate physical identity', value => value.records.rows.push(structuredClone(value.records.rows[0]))],
  ['repaired physical identity', value => value.records.rows[0].rowId = '01'],
];
for (const [name, change] of invalid) test(`source audit refuses ${name} without repairing the source`, () => {
  const value = wire(); change(value);
  assert.throws(() => audit.twoPageAuditFromWire(value, owner));
});
test('both exact source forms accept independently owned history, never invalid owner forms', () => {
  const value = wire(); value.form = 'FS882-8x6XL-CHARS';
  assert.equal(audit.twoPageAuditFromWire(value, { ...owner, form: value.form }).form, value.form);
  for (const form of ['unknown', '', 'FS882-6x4XL']) {
    value.form = form;
    assert.throws(() => audit.twoPageAuditFromWire(value, { ...owner, form }));
  }
  assert.throws(() => audit.twoPageAuditFromWire(wire(), { ...owner, plot: '\ud800' }));
});

async function mountPanel(service) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageAuditPanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ load, cancel, busy, error, review, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('audit-reader.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageSourceAuditService',
    'sourceAuditFields', 'twoPageAuditFromWire', 'twoPageAuditLabel', code + '\nreturn _unknown_;')(
    runtime, onDestroy, runtime.untrack, reads.ReadRequests, service,
    audit.sourceAuditFields, audit.twoPageAuditFromWire, audit.twoPageAuditLabel);
  const activity = [];
  let actions;
  const dispose = runtime.effect_root(() => mount(null, {
    ...owner, onBusyChange: value => activity.push(value), inspect: value => actions = value,
  }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}

test('actual Svelte client loads only explicitly, surfaces errors and permits correction/retry', async () => {
  const calls = []; let fail = true;
  const panel = await mountPanel({ GetOriginal(...args) {
    calls.push(args);
    return fail ? Promise.reject(new Error('physical schema unavailable')) : Promise.resolve(wire());
  } });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().load(); panel.flush();
    assert.match(panel.actions().error, /physical schema unavailable/);
    assert.equal(panel.actions().busy, false);
    assert.equal(panel.actions().review, null);
    fail = false;
    await panel.actions().load(); panel.flush();
    assert.deepEqual(calls, [[owner.contextId, owner.plot, owner.form], [owner.contextId, owner.plot, owner.form]]);
    assert.equal(panel.actions().error, '');
    assert.equal(panel.actions().review.records.rows[0].cells[8].integer, '-1');
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
  } finally { panel.dispose(); }
});

for (const retire of ['cancel', 'destroy']) test(`actual Svelte client ${retire} retires held successful history before remount`, async () => {
  let release, cancellations = 0, calls = 0;
  const held = new Promise(resolve => release = resolve);
  held.cancel = () => cancellations++;
  const panel = await mountPanel({ GetOriginal() { calls++; return held; } });
  const pending = panel.actions().load(); panel.flush();
  const original = panel.actions();
  assert.equal(original.busy, true);
  await original.load();
  assert.equal(calls, 1);
  if (retire === 'cancel') original.cancel(); else panel.dispose();
  panel.flush();
  assert.equal(cancellations, 1);
  const fresh = await mountPanel({ GetOriginal: () => Promise.resolve(wire()) });
  try {
    release(wire()); await pending; panel.flush(); fresh.flush();
    assert.equal(original.current(), null);
    assert.equal(fresh.actions().review, null);
    await fresh.actions().load(); fresh.flush();
    assert.equal(fresh.actions().review.records.rows.length, 1);
  } finally { fresh.dispose(); if (retire === 'cancel') panel.dispose(); }
});

test('source Audit opener honours capability, parent/peer validation and pending child ownership', () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  function host(overrides = {}, peer = {}) {
    return componentFunctions('TwoPageEntryEditor.svelte', ['getDraftState', 'openAudit', 'returnFromAudit'], {
      auditReaderAvailable: true, auditOpen: false, auditBusy: false, externalOpen: false,
      googleEarthOpen: false, googleEarthBusy: false,
      sourceRefreshing: false, sourceBlocked: false, childBlocked: false, close: clean,
      view: { original: {}, referenceError: null }, error: null, onchange() {},
      sessions: new Map([
        ['normal', { closeState: () => overrides.close || clean, view: () => ({}) }],
        ['chars', { closeState: () => ({ ...clean, ...peer }), view: () => peer }],
      ]),
      ...overrides,
    });
  }
  for (const blocked of [{ auditReaderAvailable: false }, { sourceBlocked: true }, { childBlocked: true },
    { auditBusy: true }, { externalOpen: true }, { sourceRefreshing: true }, { googleEarthOpen: true }, { googleEarthBusy: true },
    { close: { ...clean, unsaved: true } }, { close: { ...clean, busy: true } },
    { close: { ...clean, blocked: true } }, { view: { original: {}, referenceError: 'reference unavailable' } }]) {
    const context = host(blocked); context.actions.openAudit();
    assert.equal(context.auditOpen, false, JSON.stringify(blocked));
    assert.ok(context.error);
  }
  for (const peer of [{ error: 'invalid historical draft' }, { referenceError: 'unavailable' },
    { unsaved: true }, { blocked: true }, { busy: true }]) {
    const context = host({}, peer); context.actions.openAudit();
    assert.equal(context.auditOpen, false);
  }
  const context = host(); context.actions.openAudit();
  assert.equal(context.auditOpen, true);
  assert.equal(context.actions.getDraftState().blocked, true);
  context.auditBusy = true; context.actions.returnFromAudit();
  assert.equal(context.auditOpen, true);
  context.auditBusy = false; context.actions.returnFromAudit();
  assert.equal(context.auditOpen, false);
  assert.equal(context.error, null);
});

test('Audit ownership prevents direct parent writes, drafts, source switches and close callbacks', async () => {
  const calls = [];
  const context = componentFunctions('TwoPageEntryEditor.svelte',
    ['operation', 'stage', 'selectProject', 'acknowledge', 'chooseForm', 'closeReview'], {
      auditOpen: true, auditBusy: false, externalOpen: false, error: null,
      close: { busy: false, unsaved: false, blocked: false }, childBlocked: false,
      reviewEnabled: true, editingEnabled: true, form: owner.form, view: { drafts: {} },
      onchange: () => calls.push('change'), onclosed: () => calls.push('close'),
      session: { save: () => calls.push('save'), undo: () => calls.push('undo'),
        stage: () => calls.push('stage'), selectProject: () => calls.push('project'),
        acknowledge: () => calls.push('acknowledge') },
    });
  for (const action of ['load', 'save', 'undo', 'retain', 'prune', 'references']) await context.actions.operation(action);
  context.actions.stage('Comments', { kind: 'text', raw: 'new' });
  context.actions.selectProject('1'); context.actions.acknowledge('Comments');
  await context.actions.chooseForm('FS882-8x6XL-CHARS'); context.actions.closeReview();
  assert.deepEqual(calls, []);
  assert.ok(context.error);
});

test('actual source Audit markup preserves labelled readonly cards without restoration or preference controls', () => {
  const source = readFileSync(path.join(__dirname, 'TwoPageAuditPanel.svelte'), 'utf8');
  const scriptEnd = source.indexOf('</script>') + '</script>'.length;
  const component = serverComponent(`<script>
    import { sourceAuditFields, twoPageAuditLabel } from './twoPageAudit';
    let { owner, review, busy = false, error = '' } = $props();
    const load = () => {}; const cancel = () => {};
    </script>${source.slice(scriptEnd)}`, 'SourceAuditCards.svelte', { './twoPageAudit': audit });
  const html = render(component, { props: { owner, review: audit.twoPageAuditFromWire(wire(), owner) } }).body;
  assert.match(html, /VPro Data History/);
  assert.match(html, /data-source-form="USysAuditXL"/);
  assert.match(html, /Yes \(-1\)/); assert.match(html, /\(empty text\)/);
  for (const [name, label] of audit.sourceAuditFields) {
    assert.ok(html.includes(`data-source-audit-column="${name}"`));
    assert.ok(html.includes(`<dt class="font-medium">${label}</dt>`));
  }
  assert.doesNotMatch(html, /<(?:input|select|textarea)\b/);
  assert.equal((html.match(/<button /g) || []).length, 1);
});
