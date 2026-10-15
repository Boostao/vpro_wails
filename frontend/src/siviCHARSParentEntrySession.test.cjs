const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const helpers = require('./siviParentTestHelpers.cjs');
const { metadata, editor, cell, setCell, transport, writeSession } = helpers;
const wire = require('../../testdata/chars-sivi-entry-client-wire.json');
const source = require('../../resources/fs1333-chars-sivi-layout.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const timestamp = loadTypeScript('siviDateTimestamp.ts');
const references = loadTypeScript('referenceEditor.ts', {
  './projectMetadataEditor': metadata, './siviParentTransport': transport, './qualityEditor': quality,
});
const shared = loadTypeScript('sourceParentEntryDraft.ts', {
  './projectMetadataEditor': metadata, './qualityEditor': quality, './ordinaryEditor': ordinary,
  './siviDateTimestamp': timestamp,
});
const entry = loadTypeScript('siviCHARSParentEntry.ts', {
  '../../resources/fs1333-chars-sivi-layout.json': source, './siviParentEditor': editor,
  './projectMetadataEditor': metadata, './qualityEditor': quality, './sourceParentEntryDraft': shared,
});
const parentShared = loadTypeScript('siviParentSharedSession.ts', {
  '../../resources/fs1333-sivi-layout.json': helpers.source, './ordinaryEditor': ordinary,
  './coordinateEditor': loadTypeScript('coordinateEditor.ts'), './siviDateTimestamp': timestamp,
  './projectMetadataEditor': metadata, './siviParentEditor': editor, './siviParentWriteSession': writeSession,
  './referenceEditor': references,
});
const api = loadTypeScript('siviCHARSParentEntrySession.ts', {
  './siviParentTransport': transport, './siviParentWriteSession': writeSession,
  './siviParentSharedSession': parentShared, './referenceEditor': references,
  './qualityEditor': quality, './siviCHARSParentEntry': entry,
});
const owner = { contextId: 'owned', project: 'Sample', plot: '108050' };
const input = raw => ({ kind: 'text', raw });
const ahorizon = () => wire.Fields.find(field => field.column === 'SV_AhorizonType');
function review() {
  return { ...structuredClone(wire), Fields: source.forms[0].fields
    .filter(field => field.type === 'ComboBox' && field.binding !== 'ProjectID').map(field => {
      if (['SV_AhorizonType', 'Zone'].includes(field.binding)) {
        return structuredClone(wire.Fields.find(row => row.column === field.binding));
      }
      const policy = references.siviReferencePolicies.find(row => row.column === field.binding);
      const listName = field.binding === 'RootZoneParticleSize' ? 'RootZoneParticleSize' :
        field.binding === 'SnowCoverregime' ? 'SnowCoverRegime' :
          field.binding === 'SV_AhorizonType' ? 'SV_AhorizonType' : policy.list;
      const codes = field.binding === 'SubZone' ? [] : ['X', 'X', '', null];
      return { column: field.binding, listName, required: Boolean(field.properties.LimitToList),
        source: 'owned test reference', available: field.binding !== 'SubZone',
        diagnostic: field.binding === 'SubZone' ? 'CHARS source targets frmSIVIsite.cmbZone' : '',
        definitions: { columns: [{ name: 'Item', declaredType: 'TEXT' }, { name: 'Description', declaredType: 'TEXT' }],
          rows: codes.map((code, i) => ({ rowId: String(i + 1), cells: [code === null ? cell() : cell('text', code),
            i % 2 ? cell('text', '') : cell()] })) },
        choices: codes.map((code, i) => ({ rowId: String(i + 1), code, description: i % 2 ? '' : null,
          selectable: code !== null && code !== '', diagnostic: code === null || code === '' ? 'not selectable' : '' })) };
    }) };
}
function fixture(overrides = {}) {
  const state = { review: review(), writes: [], restores: [], cancelled: 0, refreshed: 0, notifications: 0 };
  const port = {
    read: async () => structuredClone(state.review),
    cancelRead: () => state.cancelled++,
    save: async request => {
      state.writes.push(structuredClone(request));
      for (const edit of request.edits) {
        const binding = state.review.Original.Bindings.find(row =>
          row.Binding === (edit.column === 'SiteNotes' ? 'siteNotes' : edit.column));
        const row = state.review.Original.Rows[0][binding.Table.endsWith('_Admin') ? 'Admin' : 'Env'];
        row.cells[binding.Column] = structuredClone(edit.value);
      }
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
    restore: async (id, action) => {
      state.restores.push({ id, action }); state.review = review();
      return { restoredRows: state.writes[0].edits.length, cleanedVegRows: 0, cancelled: false,
        prunedAuditRows: action === 'prune' ? state.writes[0].edits.length : 0 };
    },
    refreshParent: async () => { state.refreshed++; },
    ...overrides,
  };
  return { state, session: new api.SIVICHARSParentEntrySession(owner, port, () => state.notifications++) };
}

test('owned CHARS session reads complete source/reference cohorts without staging options or writes', async () => {
  const { session, state } = fixture();
  assert.equal(await session.load(), true);
  assert.equal(session.view().snapshot.fields.length, 31);
  assert.equal(session.view().snapshot.policies.length, 68);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(state.writes.length, 0);
  const fields = session.view().snapshot.fields;
  const snow = fields.find(row => row.column === 'SnowCoverregime');
  assert.deepEqual(Array.from(snow.choices, row => row.code), ['X', 'X', '', null]);
  assert.deepEqual(Array.from(snow.definitions.rows, row => row.cells[1].storage), ['null', 'text', 'null', 'text']);
  fields[0].choices[0].code = 'caller mutation';
  assert.equal(session.view().snapshot.fields[0].choices[0].code, 'X');
});

test('actual Go Ahorizon wire retains literal ordinals and blank TEXT without relaxing physical row identity', async () => {
  const { session } = fixture();
  assert.equal(await session.load(), true);
  const field = session.view().snapshot.fields.find(row => row.column === 'SV_AhorizonType');
  assert.deepEqual(JSON.parse(JSON.stringify(field)), ahorizon());
  assert.deepEqual(Array.from(field.definitions.rows, row => row.rowId),
    ['source-value-list:0', 'source-value-list:1', 'source-value-list:2']);
  assert.equal(field.definitions.rows[2].cells[0].storage, 'text');
  assert.equal(field.definitions.rows[2].cells[0].text, '');
  assert.equal(field.choices[2].selectable, false);
  assert.throws(() => transport.siviMetadataTableFromWire(ahorizon().definitions), /row identities/);
  session.stage('SV_AhorizonType', input('Ae'));
  assert.equal(session.closeState().canSave, true);
  session.stage('SV_AhorizonType', input(''));
  assert.equal(session.closeState().blocked, true);
  session.stage('SV_AhorizonType', { kind: 'clear' });
  assert.equal(session.closeState().blocked, false);
});

test('Ahorizon decoding rejects altered source ordinals, order, storage and schema explicitly', async () => {
  for (const mutate of [
    value => value.rows.pop(),
    value => { value.rows[0].rowId = '1'; },
    value => { value.rows[1].rowId = value.rows[0].rowId; },
    value => value.rows.reverse(),
    value => { value.rows[2].cells[0] = cell(); },
    value => { value.rows[0].cells[0] = cell('text', 'AH'); },
    value => { value.rows[0].cells[0].integer = '1'; },
    value => { value.columns[0] = { name: 'Item', type: 'TEXT' }; },
    value => { value.columns[0].name = 'Other'; },
  ]) {
    const { session, state } = fixture();
    mutate(state.review.Fields.find(row => row.column === 'SV_AhorizonType').definitions);
    assert.equal(await session.load(), false);
    assert.equal(session.view().original, null);
    assert.equal(session.view().snapshot, null);
    assert.match(session.view().error, /Ahorizon/);
    assert.equal(state.writes.length, 0);
  }
  const { session, state } = fixture();
  state.review.Fields[0].definitions = structuredClone(ahorizon().definitions);
  assert.equal(await session.load(), false);
  assert.equal(session.view().snapshot, null);
  assert.match(session.view().error, /row identities/);
});

test('complete actual Go reference cohort preserves Zone value identities without inventing physical rowids', () => {
  const decoded = api.siviCHARSParentEntryFromWire(structuredClone(wire), owner);
  assert.deepEqual(JSON.parse(JSON.stringify(decoded.fields)), wire.Fields);
  const zone = wire.Fields.find(field => field.column === 'Zone');
  assert.equal(zone.choices[0].rowId, '');
  const repeated = structuredClone(wire);
  const choices = repeated.Fields.find(field => field.column === 'Zone').choices;
  choices.splice(0, choices.length, { ...structuredClone(choices[0]), description: null },
    structuredClone(choices[0]), { ...structuredClone(choices[0]), description: '' });
  const projected = api.siviCHARSParentEntryFromWire(repeated, owner).fields.find(field => field.column === 'Zone');
  assert.equal(projected.choices.length, choices.length);
  assert.equal(projected.choices[0].description, null);
  assert.equal(projected.choices[2].description, '');
  for (const mutate of [
    value => { value.choices[0].rowId = '1'; },
    value => { value.definitions.columns.push({ name: 'Item', declaredType: 'TEXT' }); },
    value => { value.definitions.rows.push({ rowId: '1', cells: [] }); },
  ]) {
    const malformed = structuredClone(wire);
    mutate(malformed.Fields.find(field => field.column === 'Zone'));
    assert.throws(() => api.siviCHARSParentEntryFromWire(malformed, owner), /Zone/);
  }
});

test('only original four mandatory lists block changed nonmembers; optional input needs no invented acknowledgement', async () => {
  for (const column of ['Exposure1', 'Exposure2', 'SuccessionalStatus', 'SoilDrainage']) {
    const { session, state } = fixture(); await session.load();
    session.stage(column, input('Y'));
    assert.equal(session.closeState().blocked, true, column);
    assert.equal(session.closeState().canSave, false, column);
    assert.ok(session.view().drafts[column].error, column);
    await assert.rejects(() => session.save());
    assert.equal(state.writes.length, 0);
    session.stage(column, input('X'));
    assert.equal(session.closeState().blocked, false, column);
    assert.equal(await session.save(), true, column);
    assert.equal(state.writes.length, 1);
  }
  const { session, state } = fixture(); await session.load();
  session.stage('MoistureRegime', input('Y'));
  assert.equal(session.closeState().canSave, true);
  assert.ok(session.view().advisories.MoistureRegime);
  assert.equal(await session.save(), true);
  assert.equal(state.writes[0].edits[0].value.text, 'Y');
});

test('validation survives independent view reads and corrects without discarding original/history identities', async () => {
  const { session, state } = fixture(); await session.load();
  session.stage('RootZoneParticleSize', input('1234567'));
  for (let render = 0; render < 3; render++) {
    assert.match(session.view().drafts.RootZoneParticleSize.error, /UTF-16/);
    assert.equal(session.closeState().unsaved, true);
    assert.equal(session.closeState().blocked, true);
  }
  session.stage('RootZoneParticleSize', input('123456'));
  session.stage('SV_WaterTableCM', input('.1'));
  assert.equal(session.closeState().canSave, true);
  assert.equal(await session.save(), true);
  assert.equal(session.view().historyId, '1');
  assert.equal(session.closeState().unsaved, false);
  assert.equal(state.writes[0].edits[1].value.real, 0.10000000149011612);
  assert.equal(await session.restore('prune'), true);
  assert.deepEqual(state.restores, [{ id: '1', action: 'prune' }]);
  assert.equal(session.view().historyId, null);
});

test('NULL and historical nonmembers omit before required membership; source gaps never authorize changed assignments', async () => {
  const { session, state } = fixture();
  setCell(state.review.Original, 'Exposure1', cell('text', 'old nonmember'));
  setCell(state.review.Original, 'SubZone', cell('text', 'old source'));
  setCell(state.review.Original, 'SV_AhorizonType', cell('text', ''));
  await session.load();
  session.stage('Exposure1', input('old nonmember'));
  session.stage('SubZone', input('old source'));
  session.stage('SV_AhorizonType', input(''));
  assert.equal(session.closeState().unsaved, false);
  assert.equal(await session.save(), true);
  assert.equal(state.writes.length, 0);
  session.stage('Exposure1', { kind: 'clear' });
  assert.equal(session.closeState().canSave, true);
  session.stage('SubZone', { kind: 'clear' });
  assert.equal(session.closeState().canSave, false);
  session.stage('SubZone', { kind: 'original' });
  assert.equal(await session.save(), true);
  assert.equal(state.writes[0].edits.length, 1);
});

test('malformed reference/ownership responses fail explicitly without partial originals or success-shaped defaults', async () => {
  for (const mutate of [
    value => { value.Original.ContextID += 'foreign'; }, value => value.Fields.pop(),
    value => { value.Fields[0] = value.Fields[1]; },
    value => { value.Fields.find(row => row.column === 'Exposure1').required = false; },
    value => { value.Fields[0].choices[0].code = '\ud800'; },
    value => { value.Fields[0].choices[0].rowId = value.Fields[0].choices[1].rowId; },
    value => { value.Fields[0].definitions.columns[0] = { name: 'Item', type: 'TEXT' }; },
    value => { value.Fields.find(row => row.column === 'SubZone').available = true; },
  ]) {
    const { session, state } = fixture(); mutate(state.review);
    assert.equal(await session.load(), false);
    assert.equal(session.view().original, null);
    assert.equal(session.view().snapshot, null);
    assert.ok(session.view().error);
    assert.equal(state.writes.length, 0);
  }
});

test('cancelled or disposed held reads cannot repopulate originals or reference state', async () => {
  for (const dispose of [false, true]) {
    let release;
    const { session, state } = fixture({ read: () => new Promise(resolve => { release = resolve; }) });
    const pending = session.load();
    assert.equal(session.closeState().busy, true);
    if (dispose) session.dispose(); else session.cancel();
    release(review());
    assert.equal(await pending, false);
    assert.equal(session.view().original, null);
    assert.equal(session.view().snapshot, null);
    assert.ok(state.cancelled);
  }
});

test('ambiguous Save blocks replay until explicit observation/reload; incomplete acknowledgements never claim success', async () => {
  for (const save of [async () => { throw new Error('transport ended'); },
    async () => ({ ChangedCells: 99, HistoryID: '1' })]) {
    const { session, state } = fixture({ save }); await session.load();
    session.stage('RootZoneParticleSize', input('new'));
    assert.equal(await session.save(), false);
    assert.equal(session.closeState().blocked, true);
    assert.match(session.view().error, /do not replay/);
    await assert.rejects(() => session.save());
    assert.equal(await session.undo(), true);
    assert.equal(state.refreshed, 1);
    assert.equal(session.closeState().blocked, false);
  }
});
