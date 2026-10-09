const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { transport, restoration, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const requests = loadTypeScript('readRequests.ts', { '@wailsio/runtime': {} });
const references = loadTypeScript('twoPageSoilReferences.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
  './qualityEditor': quality, './readRequests': requests,
});
const fixtures = require('./twoPageSoilReferenceWire.json');
const fresh = (index = 0) => structuredClone(fixtures[index]);
const normalized = () => references.soilReferencesFromWire(fresh().wire, fresh().owner);
function held() {
  let resolve, reject, cancelled = 0;
  const request = Object.assign(new Promise((yes, no) => { resolve = yes; reject = no; }), {
    cancel() { cancelled++; },
  });
  return { request, resolve, reject, cancelled: () => cancelled };
}

test('actual Go reference wire preserves both source forms and ten-column storage', () => {
  for (const [index, counts] of [[0, [20, 9, 4, 11, 5, 5]], [1, [15, 30, 6, 10]]]) {
    const fixture = fresh(index);
    const result = references.soilReferencesFromWire(fixture.wire, fixture.owner);
    assert.deepEqual(JSON.parse(JSON.stringify(result)), fixture.wire);
    assert.deepEqual(Array.from(result.fields, field => field.definitions.rows.length), counts);
    result.fields[0].definitions.rows[0].cells[0].text = 'changed';
    assert.notEqual(fixture.wire.fields[0].definitions.rows[0].cells[0].text, 'changed');
  }
});

test('reference transport refuses wrong owner/source/mappings/schema and malformed physical rows', () => {
  const mutations = [
    wire => { wire.contextId = 'other'; }, wire => { wire.project = 'other'; },
    wire => { wire.plot = '0108050'; }, wire => { wire.form = 'SoilHumus'; },
    wire => { wire.source = 'frozen catalogue'; }, wire => { wire.extra = true; },
    wire => { wire.fields = null; }, wire => { wire.fields.pop(); },
    wire => { wire.fields[5].listName = 'FecalAbundance'; },
    wire => { wire.fields[0].definitions.columns[0].type = 'TEXT'; delete wire.fields[0].definitions.columns[0].declaredType; },
    wire => { wire.fields[0].definitions.rows[0].rowId = '01'; },
    wire => { wire.fields[0].definitions.rows.push(structuredClone(wire.fields[0].definitions.rows[0])); },
    wire => { wire.fields[0].definitions.rows[0].cells.pop(); },
    wire => { wire.fields[0].definitions.rows[0].cells[0].text = 'another list'; },
    wire => { wire.fields[0].definitions.rows[0].cells[3] = cell('text', '\ud800'); },
    wire => { wire.fields[0].definitions.rows[0].cells[3].extra = 'discarded'; },
  ];
  for (const mutation of mutations) {
    const fixture = fresh();
    mutation(fixture.wire);
    assert.throws(() => references.soilReferencesFromWire(fixture.wire, fixture.owner));
  }
  for (const value of ['', '\ud800']) {
    const fixture = fresh();
    fixture.owner.contextId = value; fixture.wire.contextId = value;
    assert.throws(() => references.soilReferencesFromWire(fixture.wire, fixture.owner));
  }
});

test('cancelled/disposed and superseded source reads cannot populate a later panel', async () => {
  for (const action of ['cancel', 'dispose', 'refresh']) {
    const first = held(), second = held(), states = [];
    let calls = 0;
    const lookup = new references.SoilReferenceLookup(fresh().owner,
      () => ++calls === 1 ? first.request : second.request, view => states.push(view));
    const old = lookup.refresh();
    const next = action === 'refresh' ? lookup.refresh() : (lookup[action](), null);
    assert.equal(first.cancelled(), 1);
    const count = states.length;
    first.resolve(fresh().wire);
    assert.equal(await old, false);
    assert.equal(states.length, count);
    if (next) {
      second.resolve(fresh().wire);
      assert.equal(await next, true);
      assert.equal(lookup.snapshot().data.form, 'SoilHumusXL');
    } else if (action === 'cancel') {
      assert.equal(lookup.snapshot().data, null);
      assert.match(lookup.snapshot().error, /cancelled/);
    }
    if (action === 'dispose') await assert.rejects(() => lookup.refresh(), /Disposed/);
    lookup.dispose();
  }
});

test('read/validation errors are explicit and corrected source retry remains usable', async () => {
  const failure = held(), invalid = held(), success = held();
  const queue = [failure, invalid, success];
  const lookup = new references.SoilReferenceLookup(fresh().owner, () => queue.shift().request, () => {});
  let pending = lookup.refresh();
  failure.reject(new Error('physical family unavailable'));
  assert.equal(await pending, false);
  assert.equal(lookup.snapshot().data, null);
  assert.match(lookup.snapshot().error, /physical family unavailable/);
  pending = lookup.refresh();
  invalid.resolve(null);
  assert.equal(await pending, false);
  assert.match(lookup.snapshot().error, /complete configured source bindings/);
  pending = lookup.refresh();
  success.resolve(fresh().wire);
  assert.equal(await pending, true);
  assert.equal(lookup.snapshot().error, null);
  lookup.dispose();
});

test('compiled source choices retain distinct duplicate physical definitions and NULL/empty metadata', () => {
  const field = normalized().fields[4];
  const template = structuredClone(field.definitions.rows[0]);
  template.cells[3] = cell('text', 'A');
  template.cells[4] = cell();
  const duplicate = structuredClone(template);
  duplicate.rowId = '9223372036854775807';
  duplicate.cells[4] = cell('text', '');
  duplicate.cells[7] = cell('integer', '-1');
  field.definitions.rows = [template, duplicate];
  const component = serverComponent(readFileSync(path.join(__dirname, 'SoilReferenceChoices.svelte'), 'utf8'),
    'SoilReferenceChoices.svelte', { './projectMetadataEditor': metadata });
  const html = render(component, { props: { field, label: 'Mycelium abundance', disabled: false, onchoose() {} } }).body;
  assert.equal((html.match(/data-soil-reference-row=/g) ?? []).length, 2);
  assert.match(html, /9223372036854775807/);
  assert.match(html, /NULL/);
  assert.match(html, /empty text/);
  assert.match(html, /-1 \(integer\)/);
  assert.match(html, /does not Save or restrict manual entry/);
});

module.exports = { references, fresh };
