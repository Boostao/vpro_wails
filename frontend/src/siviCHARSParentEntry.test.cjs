const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { metadata, editor, cell, setCell } = require('./siviParentTestHelpers.cjs');
const wire = require('../../testdata/chars-sivi-entry-client-wire.json');
const source = require('../../resources/fs1333-chars-sivi-layout.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const shared = loadTypeScript('sourceParentEntryDraft.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './ordinaryEditor': ordinary,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
});
const loaded = loadTypeScript('siviCHARSParentEntry.ts', {
  '../../resources/fs1333-chars-sivi-layout.json': source, './siviParentEditor': editor,
  './projectMetadataEditor': metadata, './qualityEditor': quality, './sourceParentEntryDraft': shared,
});
const original = () => structuredClone(wire.Original);
const policies = () => loaded.siviCHARSParentPoliciesFromWire(structuredClone(wire.Policies), original());
const policy = column => policies().find(field => field.column === column);
const parse = (column, raw, expected = cell()) =>
  structuredClone(loaded.parseSIVICHARSParentValue(policy(column), expected, { kind: 'text', raw }));

test('strict Go wire retains71 bindings,68 domains,67 writable fields and physical/source distinctions', () => {
  const originalValue = original();
  assert.deepEqual(structuredClone(loaded.validateSIVICHARSParentOriginal(originalValue)), originalValue);
  const fields = policies();
  assert.equal(fields.length, 68);
  assert.equal(fields.filter(field => field.writable).length, 67);
  assert.equal(fields.find(field => field.column === 'RootZoneParticleSize').maximum, 6);
  assert.equal(fields.find(field => field.column === 'SV_WaterTableCM').kind, 'single');
  assert.equal(fields.find(field => field.column === 'SiteNotes').binding, 'siteNotes');
  for (const excluded of ['PlotNumber', 'ProjectID', 'PlotType', 'SpeciesListComplete', 'SV_RootZoneTexture', 'SeepageDepth']) {
    assert.equal(fields.some(field => field.column === excluded), false, excluded);
  }
  originalValue.Rows[0].Env.cells[0].text = 'caller mutation';
  assert.deepEqual(original(), wire.Original);
});

test('client source-owned request matches the actual strict Go round-trip and prepared-plan fixture', () => {
  const value = original(), fields = policies();
  let drafts = {};
  for (const [column, input] of [
    ['RootZoneParticleSize', { kind: 'text', raw: ' a  ' }],
    ['SV_WaterTableCM', { kind: 'text', raw: '.1' }],
    ['StartDate', { kind: 'text', raw: '1900' }],
    ['SV_FloodPlain', { kind: 'boolean', value: true }],
    ['SpeciesListComplete', { kind: 'option', option: 1 }],
  ]) drafts = loaded.stageSIVICHARSParentEntry(value, fields, drafts, column, input);
  assert.deepEqual(JSON.parse(JSON.stringify(loaded.siviCHARSParentEntryRequest(value, fields, drafts))), wire.Request);
});

test('CHARS Comp./Part. preserves source identity, historical omission and independent request preparation', () => {
  const fields = policies();
  for (const stored of [cell(), cell('integer', '-1'), cell('integer', '0'), cell('integer', '7')]) {
    const value = original(); setCell(value, 'SpeciesListComplete', stored);
    const retained = loaded.stageSIVICHARSParentEntry(value, fields, {}, 'SpeciesListComplete', { kind: 'original' });
    const omitted = loaded.siviCHARSParentEntryRequest(value, fields, retained);
    assert.deepEqual(Array.from(omitted.edits), []);
    assert.equal(omitted.actions, undefined);
    for (const option of [1, 2]) {
      const drafts = loaded.stageSIVICHARSParentEntry(value, fields, {}, 'SpeciesListComplete', { kind: 'option', option });
      const request = loaded.siviCHARSParentEntryRequest(value, fields, drafts);
      const next = option === 1 ? '-1' : '0';
      assert.equal(request.actions?.length ?? 0, stored.storage === 'integer' && stored.integer === next ? 0 : 1);
      if (request.actions) {
        assert.equal(request.actions[0].controlId, 'form:frmSIVIsite-CHARS/optSpeciesListComplete');
        assert.equal(request.actions[0].table, 'Sample_Env');
        assert.equal(request.actions[0].option, option);
      }
    }
    for (const input of [{ kind: 'clear' }, { kind: 'option', option: null }, { kind: 'option', option: 0 },
      { kind: 'option', option: 3 }, { kind: 'boolean', value: true }, { kind: 'text', raw: '\ud800' }]) {
      const drafts = loaded.stageSIVICHARSParentEntry(value, fields, {}, 'SpeciesListComplete', input);
      assert.match(drafts.SpeciesListComplete.error, /Comp.\/Part./);
      drafts.SpeciesListComplete.error = null;
      assert.throws(() => loaded.siviCHARSParentEntryRequest(value, fields, drafts), /Comp.\/Part./);
    }
  }
  for (const mutate of [
    row => { row.controlId = 'form:frmSIVIsite/optSpeciesListComplete'; },
    row => { row.table = 'Sample_Admin'; },
    row => { row.rowId = 'foreign'; },
    row => { row.contextId = 'foreign'; },
    row => { row.column = 'PlotType'; },
    row => { row.expected = row.value; },
    row => { row.value = cell(); },
    row => { row.input = { kind: 'original' }; },
  ]) {
    const drafts = loaded.stageSIVICHARSParentEntry(original(), fields, {}, 'SpeciesListComplete', { kind: 'option', option: 1 });
    mutate(drafts.SpeciesListComplete);
    assert.throws(() => loaded.siviCHARSParentEntryRequest(original(), fields, drafts));
  }
});

test('client refuses wrong forms, aliases, physical rows and incomplete or substituted policy cohorts', () => {
  for (const mutate of [
    value => { value.Form = 'frmSIVIsite'; },
    value => { value.Project += ' '; },
    value => { value.Rows[0].Env.rowId = '1.0'; },
    value => { value.EnvColumns[0].type = value.EnvColumns[0].declaredType; delete value.EnvColumns[0].declaredType; },
    value => { value.Bindings[0].ControlID += 'foreign'; },
    value => { value.Rows.push(structuredClone(value.Rows[0])); },
  ]) {
    const value = original(); mutate(value);
    assert.throws(() => loaded.validateSIVICHARSParentOriginal(value));
  }
  for (const mutate of [
    fields => fields.pop(),
    fields => { fields[0] = fields[1]; },
    fields => { fields[0].owner = fields[0].owner === 'Env' ? 'Admin' : 'Env'; },
    fields => { fields[0].kind = 'latitude'; },
    fields => { fields[0].maximum = -1; },
    fields => { fields.find(row => row.column === 'SubZone').writable = true; },
    fields => { fields.find(row => row.column === 'SV_AhorizonType').allowEmpty = true; },
    fields => { fields.find(row => row.column === 'StartDate').lower = 100; },
    fields => { fields.find(row => row.column === 'RootZoneParticleSize').maximum = 100; },
    fields => { fields.find(row => row.column === 'SV_WaterTableCM').kind = 'integer'; },
    fields => { fields.find(row => row.column === 'SV_FloodPlain').diagnostic = ''; },
  ]) {
    const fields = structuredClone(wire.Policies); mutate(fields);
    assert.throws(() => loaded.siviCHARSParentPoliciesFromWire(fields, original()));
  }
});

test('CHARS numeric drafts use source narrowing, finite DOUBLEs and actual INTEGER rules, not vegetation/geographic limits', () => {
  for (const [column, raw, expected] of [
    ['SV_WaterTableCM', '.1', cell('real', 0.10000000149011612)],
    ['StrataCoverTotal', '99.999999', cell('real', 100)],
    ['SV_WaterTableCM', '3.402823466385289e38', cell('real', 3.4028234663852886e38)],
    ['SV_WaterTableCM', '1e-999', cell('real', 0)],
    ['Latitude', '91', cell('real', 91)], ['Longitude', '-181', cell('real', -181)],
    ['Latitude', '1e300', cell('real', 1e300)], ['Aspect', '-32768', cell('integer', '-32768')],
    ['Aspect', '32767', cell('integer', '32767')], ['Aspect', '-0', cell('integer', '0')],
    ['StartDate', '1900', cell('integer', '1900')], ['StartDate', '2500', cell('integer', '2500')],
  ]) assert.deepEqual(parse(column, raw), { value: expected, error: null }, `${column}:${raw}`);
  for (const [column, raw] of [
    ['SV_WaterTableCM', '1e39'], ['SV_WaterTableCM', '-1e-999'], ['Latitude', '-0'],
    ['Latitude', '1e309'], ['Aspect', '32768'], ['Aspect', '-32769'], ['Aspect', '1.5'],
    ['StartDate', '1899'], ['StartDate', '2501'], ['Aspect', ' 1'], ['Latitude', '1 '],
    ['SV_WaterTableCM', 'NaN'], ['Latitude', '0x1'], ['Latitude', '1x'],
  ]) assert.ok(parse(column, raw).error, `${column}:${raw}`);
  assert.deepEqual(parse('Latitude', ''), { value: cell(), error: null });
});

test('literal UTF16/text/date/NULL distinctions survive, including source blank gaps', () => {
  assert.deepEqual(parse('RootZoneParticleSize', ' a  '), { value: cell('text', ' a  '), error: null });
  assert.equal(parse('RootZoneParticleSize', '😀😀😀').error, null);
  assert.ok(parse('RootZoneParticleSize', '😀😀😀x').error);
  for (const raw of ['\ud800', '\udfff', 'a\0b']) assert.ok(parse('RootZoneParticleSize', raw).error);
  assert.equal(parse('SnowCoverregime', '').error, null);
  assert.ok(parse('SV_AhorizonType', '').error);
  assert.match(parse('SubZone', 'new').error, /frmSIVIsite.cmbZone/);
  assert.equal(parse('Date', '2020-02-29 12:13:14.125').error, null);
  for (const raw of ['2021-02-29 12:13:14', '2020-02-29', '2020-02-29 12:13:14Z']) {
    assert.ok(parse('Date', raw).error);
  }
  for (const field of policies()) {
    const result = loaded.parseSIVICHARSParentValue(field, cell(), { kind: 'clear' });
    assert.equal(result.error, null, field.column);
    assert.deepEqual(structuredClone(result.value), cell());
  }
});

test('Boolean NULL is not a new-write value; historical NULL omission and explicit Yes/No stay available', () => {
  const field = policy('SV_FloodPlain');
  for (const expected of [cell('integer', '0'), cell('integer', '-1')]) {
    const parsed = loaded.parseSIVICHARSParentValue(field, expected, { kind: 'clear' });
    assert.deepEqual(structuredClone(parsed.value), cell());
    assert.match(parsed.error, /NULL to No/);
    const value = original(); setCell(value, 'SV_FloodPlain', expected);
    const drafts = loaded.stageSIVICHARSParentEntry(value, policies(), {}, 'SV_FloodPlain', { kind: 'clear' });
    drafts.SV_FloodPlain.error = null;
    assert.throws(() => loaded.siviCHARSParentEntryRequest(value, policies(), drafts), /NULL to No/);
  }
  const value = original(); setCell(value, 'SV_FloodPlain', cell());
  const unchanged = loaded.stageSIVICHARSParentEntry(value, policies(), {}, 'SV_FloodPlain', { kind: 'clear' });
  assert.equal(unchanged.SV_FloodPlain.error, null);
  assert.equal(loaded.siviCHARSParentEntryRequest(value, policies(), unchanged).edits.length, 0);
  for (const selected of [true, false]) {
    const drafts = loaded.stageSIVICHARSParentEntry(value, policies(), {}, 'SV_FloodPlain',
      { kind: 'boolean', value: selected });
    assert.equal(drafts.SV_FloodPlain.error, null);
    assert.equal(loaded.siviCHARSParentEntryRequest(value, policies(), drafts).edits[0].value.integer, selected ? '-1' : '0');
  }
});

test('unchanged historical invalid values omit before new-write rules, including disabled SubZone and blank Ahorizon', () => {
  for (const [column, expected] of [
    ['RootZoneParticleSize', cell('text', 'historically overlength')],
    ['StartDate', cell('integer', '1800')], ['Latitude', cell('real', -0)],
    ['SV_WaterTableCM', cell('real', 1e40)], ['SV_AhorizonType', cell('text', '')],
    ['SubZone', cell('text', 'historical')], ['Date', cell('text', 'historical timestamp')],
  ]) {
    const value = original(); setCell(value, policy(column).binding, expected);
    const drafts = loaded.stageSIVICHARSParentEntry(value, policies(), {}, column,
      { kind: 'text', raw: metadata.metadataCellText(expected) });
    assert.equal(drafts[column].error, null, column);
    assert.deepEqual(structuredClone(drafts[column].value), expected);
    assert.equal(loaded.siviCHARSParentEntryRequest(value, policies(), drafts).edits.length, 0);
  }
  assert.match(parse('RootZoneParticleSize', 'new', cell('blob', 'FF')).error, /BLOB replacement/);
});

test('draft errors stay explicit until valid correction/retain, while exact source-owned proposals stay detached', () => {
  const value = original(), fields = policies();
  let drafts = loaded.stageSIVICHARSParentEntry(value, fields, {}, 'RootZoneParticleSize', { kind: 'text', raw: 'too long' });
  assert.throws(() => loaded.siviCHARSParentEntryRequest(value, fields, drafts), /UTF-16/);
  drafts = loaded.stageSIVICHARSParentEntry(value, fields, drafts, 'RootZoneParticleSize', { kind: 'text', raw: 'new' });
  drafts = loaded.stageSIVICHARSParentEntry(value, fields, drafts, 'SV_WaterTableCM', { kind: 'text', raw: '.1' });
  const request = loaded.siviCHARSParentEntryRequest(value, fields, drafts);
  assert.equal(request.edits.length, 2);
  for (const edit of request.edits) {
    assert.equal(edit.contextId, value.ContextID);
    assert.equal(edit.table, value.EnvTable);
    assert.equal(edit.rowId, value.Rows[0].Env.rowId);
    assert.equal('input' in edit || 'error' in edit, false);
  }
  request.edits[0].value.text = 'mutated';
  assert.equal(drafts.RootZoneParticleSize.value.text, 'new');
  drafts = loaded.stageSIVICHARSParentEntry(value, fields, drafts, 'RootZoneParticleSize', { kind: 'original' });
  assert.equal(loaded.siviCHARSParentEntryRequest(value, fields, drafts).edits.length, 1);
  for (const column of ['ProjectID', 'PlotNumber', 'PlotType', 'SV_RootZoneTexture']) {
    assert.throws(() => loaded.stageSIVICHARSParentEntry(value, fields, drafts, column, { kind: 'clear' }), /readonly/);
  }
  for (const mutate of [
    row => { row.table += 'foreign'; }, row => { row.contextId += 'foreign'; }, row => { row.rowId = '3'; },
    row => { row.column = 'ProjectID'; }, row => { row.expected = cell('real', 0); },
    row => { row.value = cell('real', 42); }, row => { row.input.raw = 'bad number'; row.error = null; },
  ]) {
    const forged = structuredClone(drafts); mutate(forged.SV_WaterTableCM);
    assert.throws(() => loaded.siviCHARSParentEntryRequest(value, fields, forged));
  }
});

test('Boolean and source option intent is explicit and normalizes only the accepted source storage', () => {
  for (const [column, input, expected] of [
    ['SV_FloodPlain', { kind: 'boolean', value: true }, cell('integer', '-1')],
    ['SV_FloodPlain', { kind: 'boolean', value: false }, cell('integer', '0')],
    ['SV_StandAgeEstMeas', { kind: 'option', option: 1 }, cell('text', '1')],
    ['SV_StandHeightEstMeas', { kind: 'option', option: 2 }, cell('text', '2')],
  ]) {
    const result = loaded.parseSIVICHARSParentValue(policy(column), cell(), input);
    assert.deepEqual(structuredClone(result), { value: expected, error: null });
  }
  assert.ok(parse('SV_FloodPlain', '-1').error);
  assert.ok(parse('SV_StandAgeEstMeas', '1').error);
});
