const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { test } = require('node:test');
const { editor, transport } = require('./siviParentTestHelpers.cjs');

const path = process.env.VPRO_SIVI_PARENT_WIRE_RECEIPT;
if (!path) throw new Error('Set VPRO_SIVI_PARENT_WIRE_RECEIPT to a successful verbose TestSIVIParentOriginalWireRoundTrip receipt.');
const raw = readFileSync(path);
const receipt = raw.toString(raw[0] === 0xff && raw[1] === 0xfe ? 'utf16le' : 'utf8');
assert.match(receipt, /^ok\s+github.com\/boostao\/vpro-wails\s+[0-9.]+s\s*$/m);
assert.doesNotMatch(receipt, /^FAIL/m);
function emitted(name) {
  const matches = [...receipt.matchAll(new RegExp(`SIVI_PARENT_WIRE_${name}:([A-Za-z0-9+/=]+)`, 'g'))];
  assert.equal(matches.length, 1, `Exactly one successful owned ${name} fixture is required.`);
  return JSON.parse(Buffer.from(matches[0][1], 'base64').toString('utf8'));
}
const owner = emitted('OWNER');
const wire = emitted('ORIGINAL');

test('actual owned Go parent JSON initializes the frontend without normalizing schema rows or raw cells', () => {
  const original = transport.siviParentOriginalFromWire(wire, owner);
  assert.deepEqual(original, wire);
  const session = transport.siviParentSessionFromWire('editor:go-wire', wire, owner);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.closeState().canSave, false);
  assert.equal(session.proposal('editor:go-wire').actions.length, 0);
  assert.equal(original.Bindings.length, 78);
  const before = JSON.stringify(wire);
  transport.siviParentActionDisplay(original);
  original.Rows[0].Env.cells[0].text = 'caller';
  assert.equal(JSON.stringify(wire), before);
  assert.deepEqual(session.view().original, wire);
});
test('all sixteen frontend targets retain actual owned Go expected cells in five independent domains', () => {
  const r = transport.siviParentOriginalFromWire(wire, owner);
  function expected(column) {
    const binding = r.Bindings.find(binding => binding.Binding === column);
    return r.Rows[0][binding.Table === r.EnvTable ? 'Env' : 'Admin'].cells[binding.Column];
  }
  const numbers = ['SV_StandHeight', 'SV_AhorizonDepth', 'SV_GleyingMottlingCM', 'SV_PercentCoarseFrags',
    'SV_SoilDepth', 'StrataCoverTotal'];
  const entries = numbers.map(column => [column, { kind: 'text',
    raw: expected(column).real === 13.5 || expected(column).text === '13.5' ? '14.5' : '13.5' }]);
  entries.push(['SV_FloodPlain', { kind: 'boolean', value: expected('SV_FloodPlain').integer !== '-1' }]);
  for (const column of ['SV_StandAgeEstMeas', 'SV_StandHeightEstMeas']) {
    entries.push([column, { kind: 'option', option: expected(column).text === '1' ? 2 : 1 }]);
  }
  for (const column of ['SV_PolygonNumber', 'SV_CanopyComposition']) {
    entries.push([column, { kind: 'text', raw: expected(column).text === 'Wire literal' ? 'Other literal' : 'Wire literal' }]);
  }
  entries.push(['SnowCoverregime', { kind: 'text', raw: expected('SnowCoverregime').text === 'Z' ? 'Q' : 'Z' }],
    ['SV_RootZoneTexture', { kind: 'text', raw: expected('SV_RootZoneTexture').text === 'Wire Unlisted' ? 'Other unlisted' : 'Wire Unlisted' }],
    ['SV_AhorizonType', expected('SV_AhorizonType').text === '' ? { kind: 'text', raw: 'Ae' } : { kind: 'empty' }],
    ['PlotType', { kind: 'option', option: expected('PlotType').text === 'Ground' ? 2 : 1 }],
    ['SpeciesListComplete', { kind: 'option', option: expected('SpeciesListComplete').integer === '-1' ? 2 : 1 }]);
  const drafts = entries.reduce((drafts, [column, input]) => editor.stageSIVIParent(r, drafts, column, input), {});
  const result = editor.siviParentProposal(r, drafts);
  assert.equal(result.scalars.length, 7);
  assert.equal(result.options.length, 2);
  assert.equal(result.text.length, 2);
  assert.equal(result.categorical.length, 3);
  assert.equal(result.actions.length, 2);
  for (const domain of ['scalars', 'options', 'text', 'categorical']) {
    for (const edit of result[domain]) {
      assert.equal(edit.contextId, owner.contextId);
      assert.deepEqual(edit.expected, expected(edit.column));
    }
  }
  assert.deepEqual(result.actions[0].expected, expected('PlotType'));
  assert.deepEqual(result.actions[1].expected, expected('SpeciesListComplete'));
  assert.deepEqual(result.original, wire);
});
