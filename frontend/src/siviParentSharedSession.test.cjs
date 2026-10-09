const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { editor, writeSession, transport, source, original: inheritedOriginal, cell, setCell, metadata, restoration, assignmentField } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const ordinary = loadTypeScript('ordinaryEditor.ts', {
  './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const coordinates = loadTypeScript('coordinateEditor.ts');
const references = loadTypeScript('referenceEditor.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata, './siviParentTransport': transport,
});
const shared = loadTypeScript('siviParentSharedSession.ts', {
  '../../resources/fs1333-sivi-layout.json': source, './ordinaryEditor': ordinary,
  './projectMetadataEditor': metadata, './siviParentEditor': editor,
  './siviParentWriteSession': writeSession, './siviParentTransport': transport,
  './coordinateEditor': coordinates,
  './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
  './referenceEditor': references,
});
const owner = { contextId: 'context:owned', project: 'Project', plot: 'P' };
function original() {
  const r = inheritedOriginal();
  for (const column of ['HumusThickness', 'StartDate']) {
    const binding = r.Bindings.find(binding => binding.Binding === column);
    r.EnvColumns[binding.Column].name = `UnboundHistoricalExtra${column}`;
    binding.Table = r.AdminTable;
    binding.Column = r.AdminColumns.length;
    r.AdminColumns.push({ name: column, declaredType: column === 'StartDate' ? 'INTEGER' : 'REAL' });
    r.Rows[0].Admin.cells.push(cell());
  }
  const notes = r.Bindings.find(binding => binding.Binding === 'siteNotes');
  r.EnvColumns[notes.Column].name = 'SiteNotes';
  return r;
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((a, b) => { resolve = a; reject = b; });
  return { promise, resolve, reject };
}
function fixture(overrides = {}, referenceEditingEnabled = false) {
  let current = original();
  const calls = { read: 0, save: 0, restore: 0, refresh: 0, cancel: 0 };
  let last;
  const port = {
    read: async () => { calls.read++; return structuredClone(current); },
    cancelRead: () => calls.cancel++,
    save: async request => {
      calls.save++; last = structuredClone(request);
      for (const edit of request.edits) setCell(current, edit.column === 'SiteNotes' ? 'siteNotes' : edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '9007199254740993' };
    },
    restore: async (_, action) => {
      calls.restore++; current = original();
      return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
    },
    refreshParent: async () => calls.refresh++,
    ...overrides,
  };
  return { session: new shared.SIVIParentSharedSession(owner, port, () => {}, referenceEditingEnabled), calls, port, last: () => last };
}
function referenceWire(zone, overrides = {}) {
  return { ...owner, zone: structuredClone(zone), fields: references.siviReferencePolicies.map(policy => ({
    column: policy.column, listName: policy.list, required: policy.required, source: 'owned test catalogue',
    available: true, diagnostic: '',
    definitions: { columns: [{ name: 'Item', declaredType: 'TEXT' }, { name: 'Note', declaredType: 'TEXT' },
      { name: 'Flag', declaredType: 'BOOLEAN' }],
      rows: [{ rowId: '1', cells: [cell('text', 'Q'), cell('null'), cell('integer', '1')] },
        { rowId: '2', cells: [cell('text', 'Q'), cell('text', ''), cell('integer', '-1')] }] },
    choices: policy.column === 'SubZone' && zone.storage === 'null' ? [] : [
      { rowId: '1', code: 'Q', description: null, selectable: true, diagnostic: '' },
      { rowId: '2', code: 'Q', description: '', selectable: true, diagnostic: '' },
      { rowId: '3', code: null, description: 'Null', selectable: false, diagnostic: 'NULL' },
      { rowId: '4', code: '', description: 'Empty', selectable: false, diagnostic: 'Empty' }],
    ...overrides[policy.column],
  })) };
}
function referenceFixture(overrides = {}, portOverrides = {}, referenceOverrides = {}) {
  const referencePort = {
    buildFlag: 'true', cancel() {},
    read: async zone => referenceWire(zone, referenceOverrides), ...overrides,
  };
  return fixture({ ...portOverrides, readReferences: zone => referencePort.read(zone),
    cancelRead: () => referencePort.cancel() }, referencePort.buildFlag === 'true');
}
test('reference gates default to exactly 32; opting in adds 28 without changing existing APIs', async () => {
  for (const buildFlag of [undefined, false, 'false', '1', true]) {
    let reads = 0;
    const { session } = referenceFixture({ buildFlag, read: async () => { reads++; } });
    await session.load();
    assert.equal(reads, 0);
    assert.equal(shared.siviParentSharedFieldsFor(session.view().referencesEnabled).length, 32);
    assert.throws(() => session.stage('Zone', { kind: 'clear' }), /unavailable/);
    await assert.rejects(session.refreshReferences(), /disabled/);
  }
  const { session } = referenceFixture();
  assert.equal(await session.load(), true);
  assert.equal(shared.siviParentSharedFieldsFor(session.view().referencesEnabled).length, 60);
  assert.equal(session.view().references.fields.length, 28);
  const originalCopy = session.view().original;
  assert.throws(() => shared.siviParentSharedRequest(originalCopy, {
    Zone: { column: 'Zone', input: { kind: 'clear' } },
  }), /exclude/);
});
test('three-argument construction ignores an available reference port through the accepted 32-field lifecycle', async () => {
  let referenceReads = 0;
  const { port, calls } = fixture({ readReferences: async () => { referenceReads++; throw new Error('must remain unused'); } });
  const session = new shared.SIVIParentSharedSession(owner, port, () => {});
  assert.equal(await session.load(), true);
  assert.equal(session.view().referencesEnabled, false);
  assert.equal(shared.siviParentSharedFieldsFor(session.view().referencesEnabled).length, 32);
  session.stage('AirPhotoNum', { kind: 'text', raw: 'literal' });
  assert.equal(await session.save(), true);
  assert.equal(await session.restore('prune'), true);
  assert.equal(await session.undo(), true);
  assert.equal(referenceReads, 0);
  assert.equal(calls.cancel, 0);
  assert.throws(() => session.stage('Exposure1', { kind: 'clear' }), /unavailable/);
});
test('the optional fourth argument enables references only for boolean true and requires a read port', async () => {
  const { port } = fixture();
  assert.throws(() => new shared.SIVIParentSharedSession(owner, port, () => {}, true), /requires a reference read port/);
  for (const flag of [undefined, false, 'false', 'true', 1, true]) {
    let reads = 0;
    const session = new shared.SIVIParentSharedSession(owner, {
      ...port, readReferences: async zone => { reads++; return referenceWire(zone); },
    }, () => {}, flag);
    assert.equal(await session.load(), true);
    assert.equal(session.view().referencesEnabled, flag === true);
    assert.equal(reads, flag === true ? 1 : 0);
  }
});
test('reference build off accepts all server reference cells while exposing only 32 editable fields', async () => {
  const current = original(), saved = [];
  for (const policy of references.siviReferencePolicies) setCell(current, policy.column, cell('text', 'historical value'));
  let referenceReads = 0;
  const { session } = fixture({
    read: async () => structuredClone(current),
    readReferences: async zone => { referenceReads++; return referenceWire(zone); },
    save: async request => {
      saved.push(structuredClone(request));
      for (const edit of request.edits) setCell(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: '1' };
    },
  }, false);
  assert.equal(await session.load(), true);
  assert.equal(session.view().error, null);
  assert.equal(session.view().referenceError, null);
  assert.equal(shared.siviParentSharedFieldsFor(session.view().referencesEnabled).length, 32);
  for (const policy of references.siviReferencePolicies) {
    assert.equal(shared.siviParentSharedCell(session.view().original, policy.column).text, 'historical value');
    assert.throws(() => session.stage(policy.column, { kind: 'clear' }), /unavailable/);
  }
  session.stage('AirPhotoNum', { kind: 'text', raw: 'accepted32' });
  assert.equal(await session.save(), true);
  assert.equal(saved.length, 1);
  assert.equal(saved[0].edits.length, 1);
  assert.equal(saved[0].edits[0].column, 'AirPhotoNum');
  assert.equal(shared.siviParentSharedCell(session.view().original, 'AirPhotoNum').text, 'accepted32');
  assert.equal(referenceReads, 0);
  for (const policy of references.siviReferencePolicies) {
    assert.equal(shared.siviParentSharedCell(session.view().original, policy.column).text, 'historical value');
  }
});
test('reference build on with runtime denial keeps accepted 32 Saves usable and never falls back for reference writes', async () => {
  const current = original();
  let reads = 0, acceptedSaves = 0, rejectedSaves = 0;
  const { session } = fixture({
    read: async () => structuredClone(current),
    readReferences: async () => { reads++; throw new Error('SIVI reference editing is independently disabled'); },
    save: async request => {
      if (request.edits.some(edit => references.siviReferencePolicies.some(policy => policy.column === edit.column))) {
        rejectedSaves++;
        throw new Error('SIVI shared field RealmClass is outside the canonical editor scope');
      }
      acceptedSaves++;
      for (const edit of request.edits) setCell(current, edit.column, edit.value);
      return { ChangedCells: request.edits.length, HistoryID: String(acceptedSaves) };
    },
  }, true);
  assert.equal(await session.load(), true);
  assert.match(session.view().referenceError, /independently disabled/);
  const denied = render(enabledReferenceComponent, { props: {
    view: session.view(), disabled: false, canSave: true,
    onstage() {}, onoperation() {}, oncancel() {}, onreferences() {},
  } }).body;
  assert.ok(denied.includes('data-sivi-reference-error'));
  assert.ok(denied.includes('independently disabled'));
  assert.equal(session.view().original !== null, true);
  assert.equal(session.closeState().canSave, true);
  session.stage('AirPhotoNum', { kind: 'text', raw: 'accepted32' });
  assert.equal(await session.save(), true);
  assert.equal(acceptedSaves, 1);
  assert.equal(reads, 2);
  assert.match(session.view().referenceError, /independently disabled/);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'AirPhotoNum').text, 'accepted32');
  session.stage('Exposure1', { kind: 'text', raw: 'Q' });
  assert.equal(session.closeState().canSave, false);
  session.stage('Exposure1', { kind: 'original' });
  session.stage('RealmClass', { kind: 'text', raw: 'free' });
  assert.equal(await session.save(), false);
  assert.equal(acceptedSaves, 1);
  assert.equal(rejectedSaves, 1);
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(), /replay|reload/);
  assert.equal(rejectedSaves, 1);
  assert.equal(shared.siviParentSharedCell(current, 'RealmClass').storage, 'null');
});
for (const policy of references.siviReferencePolicies) {
  test(`${policy.column} uses nullable bounded text; only source-required membership blocks`, async () => {
    const { session, calls } = referenceFixture();
    await session.load();
    const bound = '😀'.repeat(Math.floor(policy.maximum / 2)) + 'x'.repeat(policy.maximum % 2);
    session.stage(policy.column, { kind: 'text', raw: bound });
    if (policy.column === 'Zone') await session.refreshReferences();
    assert.equal(session.view().drafts[policy.column].value.text, bound);
    assert.equal(Boolean(session.view().drafts[policy.column].error), policy.required);
    if (!policy.required) {
      const request = shared.siviParentSharedRequest(session.view().original, session.view().drafts, true, session.view().references);
      assert.equal(request.edits[0].value.text, bound);
      assert.ok(session.view().advisories[policy.column]);
    }
    for (const invalid of ['', bound + 'x', '\ud800', '\udfff']) {
      session.stage(policy.column, { kind: 'text', raw: invalid });
      if (policy.column === 'Zone') await session.refreshReferences();
      assert.ok(session.view().drafts[policy.column].error);
      assert.equal(session.closeState().blocked, true);
      await assert.rejects(session.save());
    }
    assert.equal(calls.save, 0);
    session.stage(policy.column, { kind: 'clear' });
    if (policy.column === 'Zone') await session.refreshReferences();
    assert.equal(session.view().drafts[policy.column].error, null);
    session.stage(policy.column, { kind: 'text', raw: 'Q' });
    if (policy.column === 'Zone') await session.refreshReferences();
    assert.equal(session.view().drafts[policy.column].error, null);
    assert.equal(await session.save(), true);
  });
}
test('advisory unknown codes need no acknowledgement; required errors survive remount and clear per identity', async () => {
  const { session } = referenceFixture();
  await session.load();
  session.stage('SiteDisturbance1', { kind: 'text', raw: 'unlisted' });
  assert.equal(session.closeState().canSave, true);
  assert.match(session.view().advisories.SiteDisturbance1, /literally/);
  session.stage('Exposure1', { kind: 'text', raw: '?' });
  session.stage('SoilDrainage', { kind: 'text', raw: '?' });
  assert.ok(session.view().drafts.Exposure1.error);
  assert.ok(session.view().drafts.SoilDrainage.error);
  session.stage('Exposure1', { kind: 'text', raw: 'Q' });
  assert.equal(session.view().drafts.Exposure1.error, null);
  assert.ok(session.view().drafts.SoilDrainage.error);
  session.stage('SoilDrainage', { kind: 'original' });
  assert.equal(await session.save(), true);
});
test('reference receipts preserve duplicate definitions and NULL/empty/BOOLEAN; reject foreign owner/policy', async () => {
  const { session } = referenceFixture();
  await session.load();
  const snapshot = session.view().references;
  const first = snapshot.fields[0];
  assert.equal(first.choices[0].description, null);
  assert.equal(first.choices[1].description, '');
  assert.equal(first.definitions.rows[0].cells[2].integer, '1');
  first.choices[0].code = 'caller';
  assert.equal(session.view().references.fields[0].choices[0].code, 'Q');
  const zone = cell('null');
  for (const mutate of [wire => wire.contextId = 'foreign', wire => wire.fields[0].required = true,
    wire => wire.fields.pop(), wire => wire.zone = cell('text', 'OTHER')]) {
    const wire = referenceWire(zone); mutate(wire);
    assert.throws(() => references.siviReferencesFromWire(wire, owner, zone), /reference/i);
  }
});
test('Zone refresh never clears SubZone/SiteSeries; obsolete reads cannot publish and cancellation retries', async () => {
  const pending = [];
  let hold = false;
  const { session } = referenceFixture({ read: async zone => {
    if (!hold) return referenceWire(zone);
    const request = deferred(); pending.push({ request, zone }); return request.promise;
  } });
  await session.load();
  session.stage('SubZone', { kind: 'text', raw: 'oldSub' });
  session.stage('SiteSeries', { kind: 'text', raw: 'oldSS' });
  hold = true;
  session.stage('Zone', { kind: 'text', raw: 'A' });
  assert.equal(session.closeState().busy, true);
  session.stage('Zone', { kind: 'text', raw: 'B' });
  pending[0].request.resolve(referenceWire(pending[0].zone));
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(session.view().references, null);
  pending[1].request.resolve(referenceWire(pending[1].zone));
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(session.view().references.zone.text, 'B');
  assert.equal(session.view().drafts.SubZone.value.text, 'oldSub');
  assert.equal(session.view().drafts.SiteSeries.value.text, 'oldSS');
  assert.equal(session.view().drafts.SubZone.error, null);
  assert.match(session.view().advisories.SubZone, /effective Zone/);
  const read = session.refreshReferences();
  session.cancel();
  pending[2].request.resolve(referenceWire(pending[2].zone));
  assert.equal(await read, false);
  assert.match(session.view().referenceError, /cancelled/);
  hold = false;
  assert.equal(await session.refreshReferences(), true);
  session.stage('Zone', { kind: 'clear' });
  await session.refreshReferences();
  assert.equal(session.view().references.fields.find(field => field.column === 'SubZone').choices.length, 0);
  assert.equal(session.view().drafts.SubZone.value.text, 'oldSub');
});
test('unavailable required catalogues block only changed non-NULL codes; historical storage is omitted', async () => {
  const current = original();
  setCell(current, 'Exposure1', cell('text', 'historic-invalid'));
  setCell(current, 'MoistureRegime', cell('integer', '123'));
  const { session } = referenceFixture({}, { read: async () => structuredClone(current) },
    { Exposure1: { available: false, diagnostic: 'closed catalogue', choices: [] } });
  await session.load();
  session.stage('Exposure1', { kind: 'text', raw: 'historic-invalid' });
  session.stage('MoistureRegime', { kind: 'text', raw: '123' });
  assert.equal(session.closeState().unsaved, false);
  session.stage('Exposure1', { kind: 'text', raw: 'Q' });
  assert.match(session.view().drafts.Exposure1.error, /closed catalogue/);
  session.stage('Exposure1', { kind: 'clear' });
  assert.equal(session.closeState().canSave, true);
});
test('required valid drafts recover after delayed choices without stale errors or implicit writes', async () => {
  const pending = deferred();
  const { session, calls } = referenceFixture({ read: async zone => {
    await pending.promise; return referenceWire(zone);
  } });
  const loading = session.load();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(session.view().referencesBusy, true);
  session.stage('Exposure1', { kind: 'text', raw: 'Q' });
  assert.ok(session.view().drafts.Exposure1.error);
  await assert.rejects(session.load(), /reference choices/);
  pending.resolve();
  assert.equal(await loading, true);
  assert.equal(session.view().drafts.Exposure1.error, null);
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().canSave, true);
  session.cancel();
  assert.ok(session.view().references);
  assert.equal(calls.save, 0);
});
test('changed Zone advises against a retained incompatible SubZone without staging or clearing it', async () => {
  const current = original();
  setCell(current, 'Zone', cell('text', 'Q'));
  setCell(current, 'SubZone', cell('text', 'Q'));
  const { session } = referenceFixture({ read: async zone =>
    referenceWire(zone, zone.text === 'Q' ? {} : { SubZone: { choices: [] } }),
  }, { read: async () => structuredClone(current) });
  await session.load();
  session.stage('Zone', { kind: 'text', raw: 'B' });
  await session.refreshReferences();
  assert.equal(session.view().drafts.SubZone, undefined);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'SubZone').text, 'Q');
  assert.match(session.view().advisories.SubZone, /effective Zone/);
  assert.equal(session.closeState().canSave, true);
  const request = shared.siviParentSharedRequest(session.view().original, session.view().drafts, true, session.view().references);
  assert.equal(request.edits.length, 1);
  assert.equal(request.edits[0].column, 'Zone');
});
test('reference work cannot dispose/cancel/replay an unknown mixed commit', async () => {
  const pending = deferred();
  const { session, calls } = referenceFixture({}, { save: async () => {
    calls.save++; return pending.promise;
  } });
  await session.load();
  session.stage('StartDate', { kind: 'text', raw: '-32768' });
  session.stage('Exposure1', { kind: 'text', raw: 'Q' });
  const saving = session.save();
  assert.throws(() => session.cancel(), /cannot be cancelled/);
  assert.throws(() => session.dispose(), /cannot be cancelled/);
  assert.ok(session.view().references);
  await assert.rejects(session.refreshReferences(), /idle owned/);
  pending.resolve({ ChangedCells: 2 });
  assert.equal(await saving, false);
  assert.equal(session.closeState().blocked, true);
  assert.deepEqual(Object.keys(session.view().drafts), []);
  await assert.rejects(session.save(), /replay|reload/);
  assert.equal(calls.save, 1);
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
});
test('borrowed owner arguments are cloned for reference receipt verification', async () => {
  const localOwner = { ...owner };
  const { port } = fixture();
  const session = new shared.SIVIParentSharedSession(localOwner,
    { ...port, readReferences: async zone => referenceWire(zone) }, () => {}, true);
  localOwner.contextId = 'replaced';
  assert.equal(await session.load(), true);
  assert.equal(session.view().references.contextId, owner.contextId);
});

const textBounds = { SiteSurveyor: 30, FieldNumber: 50, Location: 255, NtsMapSheet: 8,
  UTMZone: 2, PlotRepresenting: 255, SiteSeries: 5, MapUnit: 15, SiteNotes: 0 };
test('Date uses the exact source Env binding and lossless wallclock TEXT with explicit NULL', async () => {
  const { session, calls, last } = fixture();
  await session.load();
  const field = shared.siviParentSharedFields.find(field => field.column === 'Date');
  assert.equal(field.binding, 'Date');
  assert.equal(field.label, 'Date');
  assert.equal(field.owner, 'Env');
  assert.equal(field.policy.scope, 'plot');
  const valid = ['0100-01-01 00:00:00', '9999-12-31 23:59:59.999999999',
    '2000-02-29 12:34:56', '2026-03-08 02:30:00.100000000', '2026-11-01 01:30:00.123456789'];
  for (let size = 1; size <= 9; size++) valid.push('2024-02-29 00:00:00.' + '0'.repeat(size));
  for (const raw of valid) {
    session.stage('Date', { kind: 'text', raw });
    const edit = shared.siviParentSharedRequest(session.view().original, session.view().drafts).edits[0];
    assert.equal(edit.table, session.view().original.EnvTable);
    assert.equal(edit.rowId, session.view().original.Rows[0].Env.rowId);
    assert.equal(edit.value.storage, 'text');
    assert.equal(edit.value.text, raw);
  }
  for (const raw of ['', '0099-12-31 23:59:59', '1900-02-29 12:34:56', '2026-01-01',
    '2026-01-01T00:00:00', '2026-01-01 00:00:00Z', '2026-01-01 00:00:00.1234567890',
    ' 2026-01-01 00:00:00', '2026-01-01 00:00:00\n', '\ud800', '\udfff']) {
    session.stage('Date', { kind: 'text', raw });
    const remounted = session.view();
    assert.equal(remounted.drafts.Date.input.raw, raw);
    assert.ok(remounted.drafts.Date.error);
    assert.equal(session.closeState().blocked, true);
    assert.equal(session.closeState().canSave, false);
    await assert.rejects(session.save());
    await assert.rejects(session.load(), /Save or Undo/);
  }
  assert.equal(calls.save, 0);
  session.stage('SiteSurveyor', { kind: 'text', raw: 'x'.repeat(31) });
  session.stage('Date', { kind: 'text', raw: valid[3] });
  assert.equal(session.view().drafts.Date.error, null);
  assert.ok(session.view().drafts.SiteSurveyor.error);
  assert.equal(session.closeState().blocked, true);
  session.stage('SiteSurveyor', { kind: 'text', raw: 'Literal Surveyor' });
  assert.equal(await session.save(), true);
  assert.equal(last().edits.find(edit => edit.column === 'Date').value.text, valid[3]);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'Date').text, valid[3]);
  session.stage('Date', { kind: 'clear' });
  assert.equal(await session.save(), true);
  assert.equal(last().edits[0].value.storage, 'null');
  session.stage('Date', { kind: 'text', raw: '' });
  assert.ok(session.view().drafts.Date.error);
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
  for (const wrong of ['date', 'DATE']) {
    assert.throws(() => session.stage(wrong, { kind: 'clear' }), /unavailable/);
  }
});
test('historical Date display noops omit invalid/empty/nontext cells; correction restores raw legacy identity', async () => {
  for (const historic of [cell('text', 'old localized date'), cell('text', ''), cell('text', '2024-02-29'),
    cell('integer', '45292'), cell('real', 45292.125), cell('null'), cell('blob', '00ff')]) {
    for (const action of ['retain', 'prune']) {
      const current = original();
      setCell(current, 'Date', historic);
      const baseline = structuredClone(current);
      let writes = 0;
      const { session } = fixture({
        read: async () => structuredClone(current),
        save: async request => {
          writes++;
          for (const edit of request.edits) setCell(current, edit.column, edit.value);
          return { ChangedCells: request.edits.length, HistoryID: '42' };
        },
        restore: async (_, received) => {
          assert.equal(received, action);
          Object.assign(current, structuredClone(baseline));
          return { cancelled: false, restoredRows: 1, prunedAuditRows: action === 'prune' ? 1 : 0, cleanedVegRows: 0 };
        },
      });
      await session.load();
      if (historic.storage !== 'null') {
        session.stage('Date', { kind: 'text', raw: metadata.metadataCellText(historic) });
      }
      assert.equal(await session.save(), true);
      assert.equal(writes, 0);
      if (historic.storage === 'blob') {
        session.stage('Date', { kind: 'clear' });
        assert.match(session.view().error, /BLOB/);
        session.stage('Date', { kind: 'original' });
        assert.equal(session.closeState().unsaved, false);
        continue;
      }
      session.stage('Date', { kind: 'text', raw: '9999-12-31 23:59:59.999999999' });
      assert.equal(await session.save(), true);
      assert.equal(writes, 1);
      assert.equal(await session.restore(action), true);
      assert.deepEqual(shared.siviParentSharedCell(session.view().original, 'Date'), historic);
      assert.equal(session.view().historyId, null);
    }
  }
});
test('Date unknown receipt blocks replay and invalid drafts survive owner-stable views', async () => {
  let saves = 0;
  const { session } = fixture({ save: async () => { saves++; throw new Error('Date commit outcome unknown'); } });
  await session.load();
  session.stage('Date', { kind: 'text', raw: '2026-11-01 01:30:00.123456789' });
  assert.equal(await session.save(), false);
  assert.equal(session.closeState().blocked, true);
  await assert.rejects(session.save(), /do not replay/);
  assert.equal(saves, 1);
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
  session.stage('Date', { kind: 'text', raw: 'invalid' });
  assert.equal(session.view().drafts.Date.input.raw, 'invalid');
  session.stage('Date', { kind: 'original' });
  assert.equal(session.closeState().unsaved, false);
});
for (const [column, maximum] of Object.entries(textBounds)) {
  test(`${column} preserves raw entries, exact UTF16 bounds, explicit NULL and durable remount errors`, async () => {
    const { session, calls } = fixture();
    await session.load();
    const field = shared.siviParentSharedFields.find(field => field.column === column);
    assert.equal(field.policy.kind, maximum ? 'text' : 'memo');
    if (maximum) assert.equal(field.policy.maximum, maximum);
    const bound = maximum ? '😀'.repeat(Math.floor(maximum / 2)) + 'x'.repeat(maximum % 2)
      : 'line\r\n' + '😀'.repeat(100000);
    session.stage(column, { kind: 'text', raw: bound });
    let request = shared.siviParentSharedRequest(session.view().original, session.view().drafts);
    assert.equal(request.edits[0].value.text, bound);
    assert.equal(request.edits[0].column, column);
    for (const invalid of ['', '\ud800', '\udfff', ...(maximum ? [bound + 'x'] : [])]) {
      session.stage(column, { kind: 'text', raw: invalid });
      const remounted = session.view();
      assert.equal(remounted.drafts[column].input.raw, invalid);
      assert.ok(remounted.drafts[column].error);
      assert.equal(session.closeState().blocked, true);
      assert.equal(session.closeState().canSave, false);
      await assert.rejects(session.save());
      await assert.rejects(session.load(), /Save or Undo/);
    }
    assert.equal(calls.save, 0);
    session.stage(column, { kind: 'clear' });
    assert.equal(session.view().drafts[column].value.storage, 'null');
    assert.equal(session.view().error, null);
    const literal = column === 'UTMZone' ? '01' : ' X ';
    session.stage(column, { kind: 'text', raw: literal });
    assert.equal(await session.save(), true);
    assert.equal(shared.siviParentSharedCell(session.view().original, column).text, literal);
    assert.equal(session.closeState().unsaved, false);
    session.stage(column, { kind: 'text', raw: '\ud800' });
    assert.equal(await session.undo(), true);
    assert.equal(session.closeState().blocked, false);
  });
}
test('only Notes has an explicit literal binding adaptation; physical and draft identities remain canonical', async () => {
  const { session } = fixture();
  await session.load();
  session.stage('SiteNotes', { kind: 'text', raw: 'line\r\nsecond' });
  const view = session.view();
  assert.equal(shared.siviParentSharedFields.find(field => field.column === 'SiteNotes').binding, 'siteNotes');
  assert.equal(view.drafts.SiteNotes.column, 'SiteNotes');
  assert.equal(view.drafts.SiteNotes.rowId, view.original.Rows[0].Env.rowId);
  for (const column of ['siteNotes', 'SITENOTES', 'sitesurveyor']) {
    assert.throws(() => session.stage(column, { kind: 'clear' }), /unavailable/);
  }
  for (const property of ['table', 'rowId', 'column']) {
    const drafts = structuredClone(view.drafts);
    drafts.SiteNotes[property] = property === 'table' ? view.original.AdminTable
      : property === 'rowId' ? view.original.Rows[0].Admin.rowId : 'siteNotes';
    assert.throws(() => shared.siviParentSharedRequest(view.original, drafts), /original|exclude/);
  }
  const wrong = structuredClone(view.original);
  const binding = wrong.Bindings.find(binding => binding.Binding === 'siteNotes');
  binding.Binding = 'SiteNotes';
  assert.throws(() => shared.siviParentSharedCell(wrong, 'SiteNotes'), /ownership/);
  binding.Binding = 'siteNotes';
  wrong.EnvColumns[binding.Column].name = 'siteNotes';
  assert.throws(() => shared.siviParentSharedCell(wrong, 'SiteNotes'), /physical/);
});
test('mixed text historical empty/invalid/nontext storage is omitted and restoration retires history', async () => {
  for (const action of ['retain', 'prune']) {
    const current = original();
    const historic = { SiteSurveyor: cell('text', 'x'.repeat(40)), FieldNumber: cell('text', ''),
      Location: cell('integer', '9007199254740993'), MapUnit: cell('real', 777),
      PlotRepresenting: cell('blob', '00ff'), SiteNotes: cell('text', '') };
    for (const [column, value] of Object.entries(historic)) {
      setCell(current, column === 'SiteNotes' ? 'siteNotes' : column, value);
    }
    const baseline = structuredClone(current);
    let writes = 0;
    const { session } = fixture({
      read: async () => structuredClone(current),
      save: async request => {
        writes++;
        assert.deepEqual(Array.from(request.edits, edit => edit.column), ['UTMZone', 'HumusThickness']);
        for (const edit of request.edits) setCell(current, edit.column, edit.value);
        return { ChangedCells: 2, HistoryID: '42' };
      },
      restore: async (_, received) => {
        assert.equal(received, action);
        Object.assign(current, structuredClone(baseline));
        return { cancelled: false, restoredRows: 2, prunedAuditRows: action === 'prune' ? 2 : 0, cleanedVegRows: 0 };
      },
    });
    await session.load();
    for (const [column, value] of Object.entries(historic)) {
      session.stage(column, { kind: 'text', raw: metadata.metadataCellText(value) });
    }
    assert.equal(session.closeState().blocked, false);
    session.stage('UTMZone', { kind: 'text', raw: '01' });
    session.stage('HumusThickness', { kind: 'text', raw: '12.5' });
    assert.equal(await session.save(), true, session.view().error);
    assert.equal(writes, 1);
    for (const [column, value] of Object.entries(historic)) {
      assert.deepEqual(shared.siviParentSharedCell(session.view().original, column), value);
    }
    assert.equal(await session.restore(action), true);
    assert.deepEqual(session.view().original, baseline);
    await assert.rejects(session.restore(action), /verified parent history/);
  }
});
test('text Save lost acknowledgement blocks replay until explicit fresh recovery', async () => {
  let saves = 0;
  const { session } = fixture({ save: async () => { saves++; throw new Error('lost text acknowledgement'); } });
  await session.load();
  session.stage('SiteSeries', { kind: 'text', raw: 'aB  ' });
  session.stage('SiteNotes', { kind: 'text', raw: 'line\r\nsecond' });
  assert.equal(await session.save(), false);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.view().historyId, null);
  await assert.rejects(session.save(), /Undo|reload/);
  assert.equal(saves, 1);
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
});
for (const column of ['SiteNotes', 'VegNotes']) {
  test(`${column} browser textarea no-op retains historical CRLF while changed multiline drafts keep literal LF`, async () => {
    const current = original();
    const binding = column === 'SiteNotes' ? 'siteNotes' : column;
    const saved = ' First line\r\nSecond line\r\n ';
    setCell(current, binding, cell('text', saved));
    let captured;
    const { session, calls } = fixture({
      read: async () => structuredClone(current),
      save: async request => {
        calls.save++;
        captured = structuredClone(request);
        for (const edit of request.edits) setCell(current, edit.column === 'SiteNotes' ? 'siteNotes' : edit.column, edit.value);
        return { ChangedCells: request.edits.length, HistoryID: '42' };
      },
    });
    await session.load();
    assert.equal(shared.siviParentSharedCell(session.view().original, column).text, saved);
    session.stage(column, { kind: 'text', raw: saved.replace(/\r\n/g, '\n') });
    const noOpRemount = session.view();
    assert.equal(noOpRemount.drafts[column].input.raw, ' First line\nSecond line\n ');
    assert.equal(noOpRemount.drafts[column].value.text, saved);
    assert.equal(session.closeState().unsaved, false);
    assert.equal(shared.siviParentSharedRequest(noOpRemount.original, noOpRemount.drafts).edits.length, 0);
    assert.equal(await session.save(), true);
    assert.equal(calls.save, 0);
    const edited = ' First line\nChanged second line\n ';
    session.stage(column, { kind: 'text', raw: edited });
    const remount = session.view();
    assert.equal(remount.drafts[column].input.raw, edited);
    assert.equal(remount.drafts[column].expected.text, saved);
    assert.equal(remount.drafts[column].value.text, edited);
    assert.equal(await session.save(), true);
    assert.equal(captured.edits.length, 1);
    assert.equal(captured.edits[0].column, column);
    assert.equal(captured.edits[0].expected.text, saved);
    assert.equal(captured.edits[0].value.text, edited);
    assert.equal(shared.siviParentSharedCell(session.view().original, column).text, edited);
  });
}
test('textarea comparison does not normalize ordinary single-line text assignments', async () => {
  const current = original();
  setCell(current, 'Location', cell('text', 'first\r\nsecond'));
  const { session } = fixture({ read: async () => current });
  await session.load();
  session.stage('Location', { kind: 'text', raw: 'first\nsecond' });
  const request = shared.siviParentSharedRequest(session.view().original, session.view().drafts);
  assert.equal(request.edits.length, 1);
  assert.equal(request.edits[0].value.text, 'first\nsecond');
  assert.equal(request.edits[0].expected.text, 'first\r\nsecond');
});
const numericKinds = { UTMEasting: 'single', UTMNorthing: 'single', SlopeGradient: 'single',
  LocationAccuracy: 'integer', Elevation: 'integer', Aspect: 'integer', StandAge: 'integer', StartDate: 'integer',
  Latitude: 'double', Longitude: 'double' };
for (const [column, kind] of Object.entries(numericKinds)) {
  test(`${column} stages physical numeric domains, unrounded precision, explicit NULL and durable raw errors`, async () => {
    const { session, calls } = fixture();
    await session.load();
    assert.equal(shared.siviParentSharedFields.find(field => field.column === column).policy.kind, kind);
    const limit = column === 'Latitude' ? 90 : 180;
    const valid = kind === 'integer' ? ['-32768', '32767', '-1', '0', '1e2']
      : kind === 'single' ? ['-3.4028234663852886e38', '3.4028234663852886e38', '-101.125', '1.1234567890123', ' 2e1 ']
      : [String(limit), String(-limit), '0', '-49.1234567890123', '1e1'];
    for (const raw of valid) {
      session.stage(column, { kind: 'text', raw });
      const edit = shared.siviParentSharedRequest(session.view().original, session.view().drafts).edits[0];
      assert.equal(edit.value.storage, kind === 'integer' ? 'integer' : 'real');
      assert.equal(kind === 'integer' ? edit.value.integer : edit.value.real,
        kind === 'integer' ? String(Number(raw)) : Number(raw));
      assert.equal(session.view().drafts[column].input.raw, raw);
    }
    for (const raw of ['1e', '-', 'NaN', 'Infinity', '1e309', '0x10',
      ...(kind === 'integer' ? ['32768', '-32769', '3.25'] : kind === 'single' ? ['3.5e38'] : [String(limit + .000001), ' 1 '])]) {
      session.stage(column, { kind: 'text', raw });
      const remounted = session.view();
      assert.equal(remounted.drafts[column].input.raw, raw);
      assert.ok(remounted.drafts[column].error);
      assert.equal(session.closeState().blocked, true);
      assert.equal(session.closeState().canSave, false);
      await assert.rejects(session.save());
    }
    assert.equal(calls.save, 0);
    session.stage(column, { kind: 'clear' });
    assert.equal(session.view().drafts[column].value.storage, 'null');
    assert.equal(session.view().error, null);
    session.stage(column, { kind: 'text', raw: '' });
    assert.equal(session.view().drafts[column].value.storage, 'null');
    session.stage(column, { kind: 'text', raw: valid.at(-2) });
    assert.equal(await session.save(), true);
    assert.equal(session.closeState().unsaved, false);
  });
}
test('multiple numeric errors survive remount independently and Admin year verifies its physical owner', async () => {
  const { session, calls } = fixture();
  await session.load();
  for (const [column, raw] of [['StartDate', '32768'], ['Latitude', '91'], ['Longitude', '-181'], ['SlopeGradient', '1e']]) {
    session.stage(column, { kind: 'text', raw });
  }
  const remounted = session.view();
  assert.equal(Object.values(remounted.drafts).filter(draft => draft.error).length, 4);
  session.stage('Latitude', { kind: 'text', raw: '-49.1234567890123' });
  assert.equal(session.closeState().blocked, true);
  assert.equal(Object.values(session.view().drafts).filter(draft => draft.error).length, 3);
  await assert.rejects(session.save());
  assert.equal(calls.save, 0);
  session.stage('StartDate', { kind: 'text', raw: '-32768' });
  const view = session.view();
  assert.equal(view.drafts.StartDate.table, view.original.AdminTable);
  assert.equal(view.drafts.StartDate.rowId, view.original.Rows[0].Admin.rowId);
  for (const property of ['rowId', 'table']) {
    const drafts = structuredClone(view.drafts);
    drafts.StartDate[property] = property === 'rowId' ? view.original.Rows[0].Env.rowId : view.original.EnvTable;
    assert.throws(() => shared.siviParentSharedRequest(view.original, drafts), /another original/);
  }
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
});
test('numeric no-ops preserve historical malformed cells/classes and out-of-range geographic aliases', async () => {
  const current = original();
  const historical = { UTMEasting: cell('integer', '9007199254740993'), UTMNorthing: cell('real', 1e39),
    SlopeGradient: cell('text', 'historic'), LocationAccuracy: cell('integer', '40000'),
    Elevation: cell('real', 3.25), Aspect: cell('text', ''), StandAge: cell('blob', '00ff'),
    StartDate: cell('text', 'old-year'), Latitude: cell('real', 400), Longitude: cell('integer', '-400') };
  for (const [column, value] of Object.entries(historical)) setCell(current, column, value);
  const { session, calls } = fixture({ read: async () => structuredClone(current) });
  await session.load();
  for (const [column, value] of Object.entries(historical)) {
    session.stage(column, { kind: 'text', raw: metadata.metadataCellText(value) });
    assert.deepEqual(session.view().drafts[column].value, value);
  }
  for (const [column, raw] of [['Latitude', '4e2'], ['Longitude', '-4e2'], ['UTMNorthing', '1.0e39'], ['LocationAccuracy', '4e4']]) {
    session.stage(column, { kind: 'text', raw });
    assert.deepEqual(session.view().drafts[column].value, historical[column]);
  }
  assert.equal(session.closeState().blocked, false);
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  session.stage('Latitude', { kind: 'text', raw: '401' });
  assert.equal(session.closeState().blocked, true);
  session.stage('Latitude', { kind: 'original' });
  assert.equal(session.closeState().blocked, false);
});
test('numeric unknown acknowledgements retire requests and cannot replay the mixed owner write', async () => {
  let saves = 0;
  const { session } = fixture({ save: async () => { saves++; return { ChangedCells: 2 }; } });
  await session.load();
  session.stage('StartDate', { kind: 'text', raw: '-32768' });
  session.stage('Longitude', { kind: 'text', raw: '123.9876543210987' });
  assert.equal(await session.save(), false);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.view().historyId, null);
  await assert.rejects(session.save(), /Undo|reload/);
  assert.equal(saves, 1);
  assert.equal(await session.undo(), true);
  assert.equal(session.closeState().blocked, false);
});
test('thirty-two exact source bindings reuse typed policies without callbacks or whole-header writes', async () => {
  assert.equal(shared.siviParentSharedFields.length, 32);
  for (const field of shared.siviParentSharedFields) {
    const exported = source.forms[0].fields.find(node => node.binding === field.binding);
    assert.equal(field.controlId, exported.controlId);
    assert.equal(field.label, field.column === 'RootRestrictingDepth' ? 'Root Restricting Depth' : exported.caption || exported.controlName);
    if (ordinary.ordinaryField(field.column)) assert.equal(field.policy, ordinary.ordinaryField(field.column));
  }
  const { session, calls, last } = fixture();
  await session.load();
  for (const column of shared.siviParentSharedColumns) {
    const policy = shared.siviParentSharedFields.find(field => field.column === column).policy;
    session.stage(column, { kind: 'text', raw: ['AirPhotoNum', 'VegNotes'].includes(column) ? '  Literal  '
      : column === 'Date' ? '0100-01-01 00:00:00.100000000'
      : column === 'UTMZone' ? '01' : ['text', 'memo'].includes(policy.kind) ? '  X  '
      : policy.kind === 'integer' ? '3' : '3.25' });
  }
  assert.equal(await session.save(), true);
  assert.equal(last().edits.length, 32);
  assert.equal(Object.keys(last()).join(','), 'original,edits');
  for (const edit of last().edits) {
    assert.equal(edit.contextId, owner.contextId);
    const admin = ['HumusThickness', 'StartDate'].includes(edit.column);
    assert.equal(edit.table, admin ? 'Project_Admin' : 'Project_Env');
    assert.equal(edit.rowId, admin ? '-9223372036854775808' : '9007199254740993');
    assert.equal(edit.expected.storage, 'null');
  }
  assert.equal(last().edits.find(edit => edit.column === 'AirPhotoNum').value.text, '  Literal  ');
  for (const column of ['PlotType', 'SpeciesListComplete', 'ProjectID', 'VegSurveyor', 'SV_StandHeight']) {
    assert.throws(() => session.stage(column, { kind: 'clear' }), /unavailable/);
  }
  assert.equal(calls.save, 1);
});
test('raw errors survive snapshots/remount and block Save/Lock/close until correction or Undo', async () => {
  const { session, calls } = fixture();
  await session.load();
  session.stage('AirPhotoNum', { kind: 'text', raw: '😀'.repeat(11) });
  const remounted = session.view();
  assert.equal(remounted.drafts.AirPhotoNum.input.raw.length, 22);
  remounted.drafts.AirPhotoNum.input.raw = 'changed elsewhere';
  assert.equal(session.view().drafts.AirPhotoNum.input.raw.length, 22);
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.closeState().canSave, false);
  assert.equal(session.closeState().unsaved, true);
  await assert.rejects(session.save(), /UTF-16/);
  await assert.rejects(session.load(), /Save or Undo/);
  assert.equal(calls.save, 0);
  session.stage('AirPhotoNum', { kind: 'text', raw: '\ud800' });
  assert.match(session.view().error, /Unicode/);
  session.stage('AirPhotoNum', { kind: 'text', raw: 'literal' });
  assert.equal(session.view().error, null);
  assert.equal(session.closeState().blocked, false);
  session.stage('XCoord', { kind: 'text', raw: '1e39' });
  assert.match(session.view().error, /Single/);
  assert.equal(await session.undo(), true);
  assert.equal(session.view().error, null);
  assert.equal(shared.siviParentSharedDirty(session.view().drafts), false);
});
test('unchanged historical storage stays raw and omitted; NULL and empty are distinguishable', async () => {
  const r = original();
  setCell(r, 'AirPhotoNum', cell('text', 'x'.repeat(45)));
  setCell(r, 'XCoord', cell('text', 'historic-invalid'));
  setCell(r, 'YCoord', cell('integer', '9007199254740993'));
  setCell(r, 'VegNotes', cell('text', ''));
  const { session, calls } = fixture({ read: async () => r });
  await session.load();
  for (const column of ['AirPhotoNum', 'XCoord', 'YCoord', 'VegNotes']) {
    session.stage(column, { kind: 'text', raw: metadata.metadataCellText(shared.siviParentSharedCell(r, column)) });
  }
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'YCoord').integer, '9007199254740993');
  session.stage('VegNotes', { kind: 'clear' });
  assert.equal(shared.siviParentSharedRequest(r, session.view().drafts).edits[0].value.storage, 'null');
  session.stage('VegNotes', { kind: 'original' });
  assert.equal(session.closeState().unsaved, false);
  setCell(r, 'AirPhotoNum', cell('blob', '00ff'));
  await session.load();
  session.stage('AirPhotoNum', { kind: 'clear' });
  assert.match(session.view().error, /BLOB/);
  session.stage('AirPhotoNum', { kind: 'original' });
  assert.equal(session.view().error, null);
});
test('foreign drafts/original ownership cannot manufacture a shared-field request', async () => {
  const { session } = fixture();
  await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
  const view = session.view();
  view.drafts.XCoord.rowId = '123';
  assert.throws(() => shared.siviParentSharedRequest(view.original, view.drafts), /another original/);
  view.drafts = { ProjectID: { column: 'ProjectID' } };
  assert.throws(() => shared.siviParentSharedRequest(view.original, view.drafts), /exclude callbacks/);
  const other = fixture({ read: async () => ({ ...original(), ContextID: 'stale' }) });
  assert.equal(await other.session.load(), false);
  assert.match(other.session.view().error, /owner|ownership|context/i);
});
test('rejected or malformed acknowledgements conservatively block replay and recover explicitly', async () => {
  for (const result of [null, {}, { ChangedCells: 2, HistoryID: '1' }, { ChangedCells: 1, HistoryID: '01' }, 'reject']) {
    let saves = 0;
    const { session } = fixture({ save: async () => { saves++; if (result === 'reject') throw new Error('connection lost'); return result; } });
    await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
    assert.equal(await session.save(), false);
    assert.equal(session.closeState().blocked, true);
    assert.equal(Object.keys(session.view().drafts).length, 0);
    await assert.rejects(session.save(), /do not replay/);
    assert.equal(saves, 1);
    assert.equal(await session.undo(), true);
    assert.equal(session.closeState().blocked, false);
    assert.equal(saves, 1);
  }
});
test('failed refresh never replays committed writes; retain/prune restoration use isolated history', async () => {
  let fail = true;
  const { session, calls } = fixture({ refreshParent: async () => { if (fail) throw new Error('refresh failed'); } });
  await session.load(); session.stage('XCoord', { kind: 'text', raw: '2' });
  assert.equal(await session.save(), false);
  assert.equal(session.view().historyId, '9007199254740993');
  assert.equal(session.closeState().blocked, true);
  assert.equal(await session.undo(), false);
  fail = false;
  assert.equal(await session.undo(), true);
  assert.equal(calls.save, 1);
  assert.equal(await session.restore('retain'), true);
  assert.equal(calls.restore, 1);
  for (const action of ['retain', 'prune']) {
    const f = fixture();
    await f.session.load(); f.session.stage('XCoord', { kind: 'text', raw: '2' }); await f.session.save();
    assert.equal(await f.session.restore(action), true);
    await assert.rejects(f.session.restore(action), /verified parent history/);
  }
  const unknown = fixture({ restore: async () => { throw new Error('lost acknowledgement'); } });
  await unknown.session.load(); unknown.session.stage('XCoord', { kind: 'text', raw: '2' }); await unknown.session.save();
  assert.equal(await unknown.session.restore('retain'), false);
  assert.equal(unknown.session.closeState().blocked, true);
  assert.equal(unknown.session.view().historyId, null);
});
test('read cancellation rejects late originals; active commits cannot be cancelled or disposed', async () => {
  const read = deferred();
  const f = fixture({ read: () => read.promise });
  const load = f.session.load();
  f.session.cancel(); read.resolve(original());
  assert.equal(await load, false);
  assert.equal(f.session.view().original, null);
  assert.equal(f.calls.cancel, 1);
  const save = deferred();
  const active = fixture({ save: () => save.promise });
  await active.session.load(); active.session.stage('XCoord', { kind: 'text', raw: '2' });
  const pending = active.session.save();
  assert.throws(() => active.session.cancel(), /cannot be cancelled/);
  assert.throws(() => active.session.dispose(), /cannot be cancelled/);
  save.resolve({ ChangedCells: 1, HistoryID: '' });
  assert.equal(await pending, true);
});
const component = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentSharedFields.svelte'), 'utf8')
  .replace('import.meta.env.VITE_SIVI_PARENT_REFERENCE_EDITING', 'undefined'),
  'SIVIParentSharedFields.svelte', { './siviParentSharedSession': shared, './projectMetadataEditor': metadata });
  const enabledReferenceComponent = serverComponent(
    readFileSync(path.join(__dirname, 'SIVIParentSharedFields.svelte'), 'utf8')
      .replace('import.meta.env.VITE_SIVI_PARENT_REFERENCE_EDITING', "'true'"),
    'SIVIParentSharedFields.svelte',
    { './projectMetadataEditor': metadata, './siviParentSharedSession': shared },
  );
  test('default build renders 32; enabled suggestions render exactly 60 live controls and duplicate options', async () => {
    const { session } = referenceFixture();
    await session.load();
    const props = { view: session.view(), disabled: false, canSave: true,
      onstage() {}, onoperation() {}, oncancel() {}, onreferences() {} };
    const hidden = render(component, { props }).body;
    assert.equal((hidden.match(/data-sivi-shared-field="/g) ?? []).length, 32);
    assert.equal(hidden.includes('sivi-shared-Exposure1'), false);
    const html = render(enabledReferenceComponent, { props }).body;
    assert.equal((html.match(/data-sivi-shared-field="/g) ?? []).length, 60);
    assert.equal((html.match(/<input\b|<textarea\b/g) ?? []).length, 60);
    assert.equal((html.match(/<datalist\b/g) ?? []).length, 28);
    assert.equal((html.match(/data-reference-row="1"/g) ?? []).length, 27);
    assert.equal((html.match(/data-reference-row="2"/g) ?? []).length, 27);
    for (const policy of references.siviReferencePolicies) {
      const field = shared.siviParentSharedFieldsFor(true).find(field => field.column === policy.column);
      assert.equal(field.binding, policy.column);
      assert.equal(field.owner, 'Env');
      assert.ok(html.includes(`id="sivi-shared-${policy.column}"`));
      assert.ok(html.includes(`for="sivi-shared-${policy.column}"`));
    }
    assert.ok(html.includes('(NULL description)'));
    assert.ok(html.includes('Reload reference choices'));
  });
test('real Svelte controls carry original labels, one input per field, nullable actions and safety feedback', async () => {
  const { session } = fixture();
  await session.load(); session.stage('XCoord', { kind: 'text', raw: 'broken' });
  const html = render(component, { props: { view: session.view(), disabled: false, canSave: false,
    onstage() {}, onoperation() {}, oncancel() {} } }).body;
  for (const field of shared.siviParentSharedFields) {
    assert.equal(html.split(`id="sivi-shared-${field.column}"`).length - 1, 1);
    assert.equal(html.split(`for="sivi-shared-${field.column}"`).length - 1, 1);
    assert.ok(html.includes(field.label));
    assert.ok(html.includes(field.controlId));
  }
  assert.equal((html.match(/data-sivi-shared-field=/g) || []).length, 32);
  assert.equal((html.match(/Clear to NULL/g) || []).length, 32);
  assert.match(html, /<textarea[^>]*id="sivi-shared-SiteNotes"/);
  assert.match(html, /aria-label="Plot &amp; survey"/);
  assert.match(html, /aria-label="SOIL"/);
  assert.match(html, /aria-label="Topography &amp; stand"/);
  assert.match(html, /Fixed\/six-decimal formatting/);
  assert.ok(html.indexOf('Fixed/six-decimal formatting') > html.indexOf('sivi-shared-SiteNotes'));
  assert.match(html, /<input[^>]*id="sivi-shared-Date"[^>]*type="text"/);
  assert.ok(html.indexOf('source Medium Date') > html.indexOf('sivi-shared-SiteNotes'));
  const soil = ['HumusThickness', 'SeepageDepth', 'RootingDepth', 'RootRestrictingDepth'];
  assert.ok(soil.every((column, index) => index === 0 ||
    html.indexOf(`data-sivi-shared-field="${soil[index - 1]}"`) < html.indexOf(`data-sivi-shared-field="${column}"`)));
  assert.match(html, /aria-invalid="true"/);
  assert.match(html, /role="alert"/);
  assert.ok(html.indexOf('role="alert"') < html.indexOf('desktop safety adaptations'));
  assert.match(html, /sm:grid-cols-2/);
});
test('read panel optional shared slot suppresses only supplied live fields and blocks competing operations', () => {
  const direct = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentDirectField.svelte'), 'utf8'),
    'SIVIParentDirectField.svelte', { './siviParentEditor': editor, './projectMetadataEditor': metadata });
  const actions = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentActionField.svelte'), 'utf8'),
    'SIVIParentActionField.svelte', { './siviParentEditor': editor, './projectMetadataEditor': metadata });
  const panel = serverComponent(readFileSync(path.join(__dirname, 'SIVIParentReadPanel.svelte'), 'utf8'),
    'SIVIParentReadPanel.svelte', {
      '../../resources/fs1333-sivi-layout.json': { default: source }, './projectMetadataEditor': metadata,
      './siviParentTransport': transport, './siviParentEditor': editor, './siviParentWriteSession': writeSession,
      './SIVIParentDirectField.svelte': { default: direct }, './SIVIParentActionField.svelte': { default: actions },
      './SIVIProjectAssignmentField.svelte': { default: assignmentField },
      './siviProjectAssignmentSession': require('./siviParentTestHelpers.cjs').assignmentSession,
    });
  const props = { view: { original: original(), busy: false, error: null }, onreload() {}, oncancel() {}, reloadDisabled: false };
  const baseline = render(panel, { props }).body;
  assert.equal((baseline.match(/data-sivi-parent-field=/g) || []).length, 77);
  const content = () => {};
  const mounted = render(panel, { props: { ...props,
    sharedEditor: { liveColumns: shared.siviParentSharedColumns, content, busy: false, unsaved: true } } }).body;
  assert.equal((mounted.match(/data-sivi-parent-field=/g) || []).length, 45);
  for (const column of shared.siviParentSharedColumns) assert.ok(!mounted.includes(`data-sivi-parent-field="${column}"`));
  for (const policy of references.siviReferencePolicies) {
    assert.ok(mounted.includes(`data-sivi-parent-field="${policy.column}"`));
  }
  assert.ok(!mounted.includes('data-sivi-parent-field="siteNotes"'));
  assert.match(mounted, /<fieldset[^>]*disabled/);
  const expanded = render(panel, { props: { ...props,
    sharedEditor: { liveColumns: shared.siviParentSharedFieldsFor(true).map(field => field.column),
      content, busy: false, unsaved: false } } }).body;
  assert.equal((expanded.match(/data-sivi-parent-field=/g) || []).length, 17);
  for (const policy of references.siviReferencePolicies) {
    assert.ok(!expanded.includes(`data-sivi-parent-field="${policy.column}"`));
  }
});

test('soil integer boundaries stage canonical exact INTEGER and explicit NULL with durable errors', async () => {
  for (const column of ['SeepageDepth', 'RootingDepth', 'RootRestrictingDepth']) {
    const { session, calls } = fixture();
    await session.load();
    for (const raw of ['-32768', '32767', '0', '-0', '1e2']) {
      session.stage(column, { kind: 'text', raw });
      const edit = shared.siviParentSharedRequest(session.view().original, session.view().drafts).edits[0];
      assert.equal(edit.value.storage, 'integer');
      assert.equal(edit.value.integer, String(Number(raw)));
      assert.equal(edit.value.real, null);
    }
    for (const raw of ['-32769', '32768', '3.25', '9007199254740993', 'bad']) {
      session.stage(column, { kind: 'text', raw });
      assert.equal(session.view().drafts[column].input.raw, raw);
      assert.equal(session.closeState().blocked, true);
      await assert.rejects(session.save());
    }
    assert.equal(calls.save, 0);
    session.stage(column, { kind: 'clear' });
    assert.equal(session.view().drafts[column].value.storage, 'null');
    assert.equal(session.view().error, null);
    session.stage(column, { kind: 'text', raw: '32767' });
    assert.equal(await session.save(), true);
    assert.equal(shared.siviParentSharedCell(session.view().original, column).integer, '32767');
    session.stage(column, { kind: 'text', raw: 'invalid again' });
    assert.equal(await session.undo(), true);
    assert.equal(session.closeState().blocked, false);
  }
});
test('Admin drafts verify their distinct row/table and reject swapped physical ownership', async () => {
  const { session } = fixture();
  await session.load(); session.stage('HumusThickness', { kind: 'text', raw: '2.5' });
  const view = session.view();
  assert.equal(view.drafts.HumusThickness.table, 'Project_Admin');
  assert.equal(view.drafts.HumusThickness.rowId, view.original.Rows[0].Admin.rowId);
  for (const property of ['rowId', 'table', 'contextId']) {
    const drafts = structuredClone(view.drafts);
    drafts.HumusThickness[property] = property === 'rowId' ? view.original.Rows[0].Env.rowId
      : property === 'table' ? view.original.EnvTable : 'foreign';
    assert.throws(() => shared.siviParentSharedRequest(view.original, drafts), /another original/);
  }
  const foreign = structuredClone(view.original);
  foreign.Bindings.find(binding => binding.Binding === 'HumusThickness').Table = foreign.EnvTable;
  assert.throws(() => shared.siviParentSharedCell(foreign, 'HumusThickness'), /ownership/);
});
test('soil historical raw integer/real/text values survive unchanged mixed Save and restoration', async () => {
  const current = original();
  setCell(current, 'SeepageDepth', cell('integer', '9007199254740993'));
  setCell(current, 'RootingDepth', cell('real', 2.5));
  setCell(current, 'RootRestrictingDepth', cell('text', 'historical invalid'));
  setCell(current, 'HumusThickness', cell('real', 1e39));
  const f = fixture({ read: async () => current });
  await f.session.load();
  for (const column of ['SeepageDepth', 'RootingDepth', 'RootRestrictingDepth', 'HumusThickness']) {
    f.session.stage(column, { kind: 'text', raw: metadata.metadataCellText(shared.siviParentSharedCell(current, column)) });
  }
  assert.equal(await f.session.save(), true);
  assert.equal(f.calls.save, 0);
  f.session.stage('AirPhotoNum', { kind: 'text', raw: 'changed' });
  assert.equal(await f.session.save(), true);
  assert.equal(f.last().edits.length, 1);
  assert.equal(shared.siviParentSharedCell(f.session.view().original, 'SeepageDepth').integer, '9007199254740993');
  const mixed = fixture();
  await mixed.session.load();
  mixed.session.stage('HumusThickness', { kind: 'text', raw: '2.5' });
  mixed.session.stage('RootingDepth', { kind: 'text', raw: '-32768' });
  assert.equal(await mixed.session.save(), true);
  assert.equal(mixed.last().edits.length, 2);
  assert.equal(await mixed.session.restore('prune'), true);
});

test('numeric aliases of unchanged historical values omit assignments instead of normalizing storage', async () => {
  const current = original();
  setCell(current, 'RootingDepth', cell('integer', '40000'));
  setCell(current, 'SeepageDepth', cell('real', 40000));
  setCell(current, 'HumusThickness', cell('integer', '5'));
  const { session, calls } = fixture({ read: async () => current });
  await session.load();
  session.stage('RootingDepth', { kind: 'text', raw: '4e4' });
  session.stage('SeepageDepth', { kind: 'text', raw: '40000.0' });
  session.stage('HumusThickness', { kind: 'text', raw: '5.0' });
  assert.equal(session.view().drafts.SeepageDepth.value.storage, 'real');
  assert.equal(session.view().drafts.HumusThickness.value.storage, 'integer');
  assert.equal(await session.save(), true);
  assert.equal(calls.save, 0);
  session.stage('RootingDepth', { kind: 'text', raw: '40001' });
  assert.equal(session.closeState().blocked, true);
});

test('mixed soil commit uncertainty and failed recovery cannot replay across tables', async () => {
  let failRefresh = true;
  const { session, calls } = fixture({ refreshParent: async () => {
    if (failRefresh) throw new Error('mixed refresh failure');
  } });
  await session.load();
  session.stage('HumusThickness', { kind: 'text', raw: '3.5' });
  session.stage('RootRestrictingDepth', { kind: 'text', raw: '32767' });
  assert.equal(await session.save(), false);
  assert.equal(session.closeState().blocked, true);
  assert.equal(Object.keys(session.view().drafts).length, 0);
  assert.equal(await session.undo(), false);
  await assert.rejects(session.save(), /do not replay/);
  failRefresh = false;
  assert.equal(await session.undo(), true);
  assert.equal(calls.save, 1);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'HumusThickness').real, 3.5);
  assert.equal(shared.siviParentSharedCell(session.view().original, 'RootRestrictingDepth').integer, '32767');
});
