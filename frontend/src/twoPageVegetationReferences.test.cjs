const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const client = require('svelte/internal/client');
const { render } = require('svelte/server');
const { compile } = require('svelte/compiler');
const ts = require('typescript');
const { serverComponent } = require('./svelteTestHelpers.cjs');
const { references, TwoPageVegetationSession, cell } = require('./twoPageVegetationTestHelpers.cjs');
const fixture = require('./twoPageVegetationReferenceWire.json');
const receipts = require('./twoPageVegetationReceipts.json');
const fresh = () => structuredClone(fixture);
const ownerOf = wire => ({ contextId: wire.contextId, project: wire.project, plot: wire.plot, form: wire.form });
const owner = ownerOf(fixture);
const ready = () => Promise.resolve();
const operation = value => Object.assign(Promise.resolve(value), { cancel() {} });
function held() {
  let resolve, reject, cancelled = 0;
  const request = Object.assign(new Promise((yes, no) => { resolve = yes; reject = no; }), { cancel() { cancelled++; } });
  return { request, resolve, reject, cancelled: () => cancelled };
}
function session(ports = {}, writing = true) {
  const original = structuredClone(receipts[3].history.Request.original);
  Object.assign(original, owner);
  return new TwoPageVegetationSession(owner, {
    read: () => operation(client.proxy(original)),
    references: () => operation(client.proxy(fresh())),
    save: () => { throw new Error('Unexpected Species mutation'); },
    outcome: () => { throw new Error('Unexpected Species recovery'); },
    validationYield: ready, ...ports,
  }, writing);
}

test('actual Go wire detaches real proxies without losing physical duplicate codes NULL/empty or source widths', async () => {
  for (const form of ['SubVegAXL_BC', 'SubVegAXL', 'SubVegCXL', 'SubVegDXL']) {
    const wire = fresh(); wire.form = form;
    const proxy = client.proxy(wire);
    assert.throws(() => structuredClone(proxy));
    const result = await references.vegetationReferencesFromWire(proxy, ownerOf(wire), ready);
    assert.deepEqual(JSON.parse(JSON.stringify(result)), wire);
    assert.deepEqual(Array.from(result.master.rows, row => row.rowId), ['-9', '7', '8', '9']);
    assert.equal(result.master.rows[0].cells[1].storage, 'null');
    assert.equal(result.master.rows[1].cells[1].text, '');
    result.master.rows[0].cells[0].text = 'changed';
    assert.equal(proxy.master.rows[0].cells[0].text, 'DUP');
  }
});

test('master signed32 and personal signed16 bounds are explicit regardless of SQLite declared type', async () => {
  for (const [role, accepted, rejected] of [
    ['master', ['-2147483648', '-32769', '32768', '2147483647'], ['-2147483649', '2147483648']],
    ['personal', ['-32768', '32767'], ['-32769', '32768']],
  ]) {
    for (const value of accepted) {
      const wire = fresh();
      wire[role].rows[0].cells[2] = cell('integer', value);
      wire[role].columns[2].declaredType = 'NUMERIC';
      assert.equal((await references.vegetationReferencesFromWire(wire, owner, ready))[role].rows[0].cells[2].integer, value);
    }
    for (const value of rejected) {
      const wire = fresh(); wire[role].rows[0].cells[2] = cell('integer', value);
      await assert.rejects(references.vegetationReferencesFromWire(wire, owner, ready), /source signed/);
    }
  }
});

test('strict reference shape rejects wrong owners schema coercion malformed tagged values and duplicate physical IDs', async () => {
  for (const mutate of [
    wire => { wire.contextId += ' '; }, wire => { wire.project = 'sample'; },
    wire => { wire.plot = '0108050'; }, wire => { wire.form = 'SubVegDXL'; },
    wire => { wire.source = 'source UNION'; }, wire => { wire.extra = true; },
    wire => { wire.master.rows = null; }, wire => { wire.personal.columns.pop(); },
    wire => { wire.master.rows[0].rowId = '01'; },
    wire => { wire.master.rows.push(structuredClone(wire.master.rows[0])); },
    wire => { wire.master.rows[0].cells.pop(); },
    wire => { wire.master.rows[0].cells[0].text = '\ud800'; },
    wire => { wire.master.rows[0].cells[0].integer = '1'; },
    wire => { wire.master.rows[0].cells[0].extra = true; },
    wire => { wire.master.rows[0].cells[1] = cell('integer', '2'); },
    wire => { wire.master.rows[0].cells[2] = cell('text', '1'); },
    wire => { wire.master.rows[0].cells[2] = cell('real', 1); },
    wire => { wire.master.columns[0].name = '\u017fode'; },
    wire => { wire.master.columns[0].type = 'TEXT'; delete wire.master.columns[0].declaredType; },
    wire => { wire.master.columns[0].declaredType = '\ud800'; },
  ]) {
    const wire = fresh(); mutate(wire);
    await assert.rejects(references.vegetationReferencesFromWire(wire, owner, ready));
  }
  for (const value of ['', '\ud800', 'P\0']) {
    const wire = fresh(); wire.plot = value;
    await assert.rejects(references.vegetationReferencesFromWire(wire, ownerOf(wire), ready));
  }
});

test('empty physical sources valid historical long text and ASCII-recased names remain lossless', async () => {
  const wire = fresh();
  wire.master.rows[0].cells[1] = cell('text', 'x'.repeat(500) + '\r\n  ');
  wire.master.columns[1].name = 'scientificname';
  wire.master.columns[1].declaredType = '';
  wire.personal.rows = [];
  const result = await references.vegetationReferencesFromWire(client.proxy(wire), owner, ready);
  assert.deepEqual(JSON.parse(JSON.stringify(result)), wire);
});

test('owned readonly review preserves invalid numeric drafts and errors through read failure correction and cancellation', async () => {
  let wire = fresh();
  const retained = session({ references: () => operation(client.proxy(wire)) });
  await retained.load();
  retained.stage('-7', 'Cover1', '100', false);
  const before = retained.view();
  assert.equal(await retained.loadReferences(), true);
  assert.deepEqual(retained.view().drafts, before.drafts);
  assert.equal(retained.view().error, before.error);
  assert.equal(retained.closeState().blocked, true);
  wire = fresh(); wire.personal.rows[0].cells[2] = cell('integer', '32768');
  assert.equal(await retained.loadReferences(), false);
  assert.equal(retained.view().references, null);
  assert.match(retained.view().referenceError, /signed16/);
  assert.deepEqual(retained.view().drafts, before.drafts);
  wire = fresh();
  assert.equal(await retained.loadReferences(), true);
  assert.equal(retained.view().referenceError, null);
  const snapshot = retained.view();
  snapshot.references.master.rows[0].cells[0].text = 'outside';
  assert.equal(retained.view().references.master.rows[0].cells[0].text, 'DUP');
  retained.undo();
  assert.equal(retained.closeState().blocked, false);
});

test('cancelled reads and validation yields cannot publish stale data or clear newer failures', async () => {
  for (const boundary of ['transport', 'validation']) {
    const wait = held();
    const entered = held();
    let first = true;
    const retained = session({
      references: () => boundary === 'transport' && first ? wait.request : operation(fresh()),
      validationYield: () => {
        if (boundary === 'validation' && first) { entered.resolve(); return wait.request; }
        return ready();
      },
    });
    const old = retained.loadReferences();
    if (boundary === 'validation') await entered.request;
    assert.equal(retained.closeState().busy, true);
    assert.throws(() => retained.dispose(), /cancel/);
    const notifications = [];
    let stop = retained.subscribe(() => notifications.push(retained.view()));
    stop();
    stop = retained.subscribe(() => notifications.push(retained.view()));
    retained.cancel();
    assert.equal(retained.closeState().busy, false);
    assert.match(retained.view().referenceError, /cancelled/);
    assert.equal(retained.view().references, null);
    first = false;
    assert.equal(await retained.loadReferences(), true);
    const count = notifications.length;
    wait.resolve(fresh());
    assert.equal(await old, false);
    assert.equal(notifications.length, count);
    assert.equal(retained.view().referenceError, null);
    assert.equal(retained.view().references.master.rows[0].cells[0].text, 'DUP');
    assert.equal(wait.cancelled(), boundary === 'transport' ? 1 : 0);
    stop(); retained.dispose();
  }
});

test('large definition validation yields and same numeric row IDs in separate sources are not collapsed', async () => {
  const wire = fresh();
  wire.master.rows = Array.from({ length: 260 }, (_, index) => ({ ...structuredClone(wire.master.rows[0]), rowId: String(index + 1) }));
  let yields = 0;
  const result = await references.vegetationReferencesFromWire(client.proxy(wire), owner, async () => { yields++; });
  assert.equal(yields, 3);
  assert.equal(result.master.rows.length, 260);
  assert.equal(result.personal.rows.length, 3);
});

test('readonly reference view exposes original typed labels identities and no editing or selectable UNION options', async () => {
  const data = await references.vegetationReferencesFromWire(fresh(), owner, ready);
  const component = serverComponent(readFileSync(path.join(__dirname, 'VegetationSpeciesReferences.svelte'), 'utf8'), 'VegetationSpeciesReferences.svelte', {});
  const html = render(component, { props: { references: data } }).body;
  for (const expected of [/signed32 LONG/, /signed16 INTEGER/, /not source UNION choices/, /Physical row -9/]) assert.match(html, expected);
  assert.match(html, /ScientificName/);
  assert.match(html, /INTEGER/);
  assert.match(html, /NULL/);
  assert.match(html, /<dd[^>]*>""<\/dd>/);
  assert.match(html, /-2147483648/);
  assert.match(html, /2147483647/);
  assert.doesNotMatch(html, /<(input|select|textarea)\b/);
  assert.doesNotMatch(html, /Save Species|Choose Species/);
});

test('readonly sessions can read definitions without a numeric original or a write permission', async () => {
  const retained = session({}, false);
  assert.equal(await retained.loadReferences(), true);
  assert.equal(retained.view().original, null);
  assert.equal(retained.closeState().canSave, false);
  assert.equal(retained.closeState().unsaved, false);
  retained.dispose();
  await assert.rejects(() => retained.loadReferences(), /disposed/);
});

test('actual client paging keeps source pages independent and clamps replacement metadata without resetting on numeric drafts', async () => {
  const wire = fresh();
  for (const [role, count] of [['master', 45], ['personal', 23]]) {
    wire[role].rows = Array.from({ length: count }, (_, index) => ({
      ...structuredClone(wire[role].rows[0]), rowId: String(index + 1),
    }));
  }
  const data = await references.vegetationReferencesFromWire(wire, owner, ready);
  let script = readFileSync(path.join(__dirname, 'VegetationSpeciesReferences.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('let { references }:', 'let { references, inspect }:');
  script += '\n$effect(() => inspect({ tables, page }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  assert.equal(compiled.warnings.length, 0);
  const ast = ts.createSourceFile('references.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n').replace('export default function', 'function');
  const mount = new Function('$', code + '\nreturn _unknown_;')(client);
  const state = client.state(client.proxy(data));
  let actions;
  const dispose = client.effect_root(() => mount(null, {
    get references() { return client.get(state); }, inspect: value => { actions = value; },
  }));
  client.flush();
  try {
    assert.deepEqual(Array.from(actions.tables, table => table.page), [0, 0]);
    actions.page('master', 1); client.flush();
    assert.deepEqual(Array.from(actions.tables, table => table.page), [1, 0]);
    assert.equal(actions.tables[0].table.rows[actions.tables[0].page * 20].rowId, '21');
    actions.page('personal', 1); actions.page('master', 2); client.flush();
    assert.deepEqual(Array.from(actions.tables, table => table.page), [2, 1]);
    assert.equal(actions.tables[0].table.rows.slice(40, 60).length, 5);
    client.set(state, client.proxy(structuredClone(data))); client.flush();
    assert.deepEqual(Array.from(actions.tables, table => table.page), [2, 1]);
    const replacement = await references.vegetationReferencesFromWire(fresh(), owner, ready);
    client.set(state, client.proxy(replacement)); client.flush();
    assert.deepEqual(Array.from(actions.tables, table => table.page), [0, 0]);
    actions.page('personal', 0); actions.page('master', 0); client.flush();
    assert.deepEqual(Array.from(actions.tables, table => table.page), [0, 0]);
  } finally { dispose(); }
});
