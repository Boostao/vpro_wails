const assert = require('node:assert/strict');
const { test } = require('node:test');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const { editor, SIVIParentSession, transport, original, cell, setCell } = require('./siviParentTestHelpers.cjs');
const { closeDisposition } = loadTypeScript('closeLifecycle.ts');
const inputs = {
  SV_StandHeight: { kind: 'text', raw: '-1.5' },
  SV_AhorizonDepth: { kind: 'text', raw: '1e2' },
  SV_GleyingMottlingCM: { kind: 'text', raw: '.25' },
  SV_PercentCoarseFrags: { kind: 'text', raw: '110' },
  SV_SoilDepth: { kind: 'text', raw: '3.4028234663852886e38' },
  StrataCoverTotal: { kind: 'text', raw: '120' },
  SV_FloodPlain: { kind: 'boolean', value: true },
  SV_StandAgeEstMeas: { kind: 'option', option: 1 },
  SV_StandHeightEstMeas: { kind: 'option', option: 2 },
  SV_PolygonNumber: { kind: 'text', raw: '  literal  ' },
  SV_CanopyComposition: { kind: 'text', raw: 'Mixed Case' },
  SnowCoverregime: { kind: 'text', raw: 'Z' },
  SV_RootZoneTexture: { kind: 'text', raw: '  Unlisted  ' },
  SV_AhorizonType: { kind: 'empty' },
  PlotType: { kind: 'option', option: 4 },
  SpeciesListComplete: { kind: 'option', option: 1 },
};
function stageAll(review, entries = Object.entries(inputs)) {
  return entries.reduce((drafts, [column, input]) => editor.stageSIVIParent(review, drafts, column, input), {});
}
test('parent model preserves complete originals and all sixteen literal five-domain payloads', () => {
  const r = original(), drafts = stageAll(r), proposal = editor.siviParentProposal(r, drafts);
  const values = {
    SV_StandHeight: cell('real', -1.5), SV_AhorizonDepth: cell('real', 100), SV_GleyingMottlingCM: cell('real', 0.25),
    SV_PercentCoarseFrags: cell('real', 110), SV_SoilDepth: cell('real', 3.4028234663852886e38),
    StrataCoverTotal: cell('real', 120), SV_FloodPlain: cell('integer', '-1'),
    SV_StandAgeEstMeas: cell('text', '1'), SV_StandHeightEstMeas: cell('text', '2'),
    SV_PolygonNumber: cell('text', '  literal  '), SV_CanopyComposition: cell('text', 'Mixed Case'),
    SnowCoverregime: cell('text', 'Z'), SV_RootZoneTexture: cell('text', '  Unlisted  '), SV_AhorizonType: cell('text', ''),
  };
  assert.equal(proposal.scalars.length, 7);
  assert.equal(proposal.options.length, 2);
  assert.equal(proposal.text.length, 2);
  assert.equal(proposal.categorical.length, 3);
  assert.equal(proposal.actions.length, 2);
  for (const domain of ['scalars', 'options', 'text', 'categorical']) {
    for (const edit of proposal[domain]) {
      const binding = r.Bindings.find(binding => binding.Binding === edit.column);
      assert.equal(edit.contextId, r.ContextID);
      assert.equal(edit.table, binding.Table);
      assert.equal(edit.rowId, binding.Table.endsWith('_Admin') ? '-9223372036854775808' : '9007199254740993');
      assert.equal(edit.controlId, binding.ControlID);
      assert.deepEqual(edit.expected, cell());
      assert.deepEqual(edit.value, drafts[edit.column].value);
      assert.deepEqual(structuredClone(edit.value), values[edit.column]);
    }
  }
  assert.equal(proposal.scalars.find(edit => edit.column === 'SV_FloodPlain').value.integer, '-1');
  assert.equal(proposal.options[0].value.text, '1');
  assert.equal(proposal.options[1].value.text, '2');
  assert.equal(proposal.text[0].value.text, '  literal  ');
  assert.equal(proposal.categorical[2].value.text, '');
  assert.deepEqual(Array.from(proposal.actions, edit => ({ ...edit })), [
    { contextId: r.ContextID, table: r.AdminTable, rowId: r.Rows[0].Admin.rowId,
      controlId: 'form:frmSIVIsite/optPlotType', expected: cell(), option: 4 },
    { contextId: r.ContextID, table: r.EnvTable, rowId: r.Rows[0].Env.rowId,
      controlId: 'form:frmSIVIsite/optSpeciesListComplete', expected: cell(), option: 1 },
  ]);
  proposal.original.Rows[0].Env.cells[0].text = 'foreign';
  proposal.scalars[0].value.real = 999;
  assert.equal(r.Rows[0].Env.cells[0].text, 'P');
  assert.equal(drafts.SV_StandHeight.value.real, -1.5);
});
test('all31 nonempty domain combinations remain independent with no invented assignments', () => {
  const domains = [
    ['SV_StandHeight', 'SV_FloodPlain'], ['SV_StandAgeEstMeas'], ['SV_PolygonNumber'],
    ['SV_AhorizonType'], ['SpeciesListComplete'],
  ];
  const names = ['scalars', 'options', 'text', 'categorical', 'actions'];
  for (let mask = 1; mask < 32; mask++) {
    const r = original();
    const entries = domains.flatMap((fields, index) => mask & (1 << index) ? fields.map(field => [field, inputs[field]]) : []);
    const proposal = editor.siviParentProposal(r, stageAll(r, entries));
    names.forEach((name, index) => assert.equal(proposal[name].length, mask & (1 << index) ? domains[index].length : 0));
  }
});
test('parent source closure rejects foreign, ambiguous, normalized and incomplete physical identities', () => {
  for (const mutate of [
    r => r.ContextID = '', r => r.Project = 'Other', r => r.Plot = 'p',
    r => r.Form = 'frmSIVIsite-CHARS', r => r.Query = 'USysEnvAlias', r => r.Membership = 'access-parity',
    r => r.Rows.push(structuredClone(r.Rows[0])), r => r.Rows = [],
    r => r.Rows[0].Env.rowId = '09007199254740993', r => r.Rows[0].Admin.rowId = '9223372036854775808',
    r => r.EnvColumns[0].name = 'rowid', r => r.EnvColumns[0].name = '_ROWID_', r => r.EnvColumns[0].name = 'oid',
    r => r.EnvColumns.push({ ...r.EnvColumns[0] }), r => r.EnvColumns[1].name = r.EnvColumns[0].name.toLowerCase(),
    r => r.Rows[0].Env.cells.pop(), r => r.Rows[0].Env.cells[0] = { ...cell('text', 'P'), real: 1 },
    r => r.Bindings.pop(), r => r.Bindings[0].ControlID = 'alias', r => r.Bindings[0].Column += 1,
    r => r.Bindings[0].Table = 'Project_Admin', r => r.Bindings[77].Implicit = false,
    r => r.Bindings[1].Binding = r.Bindings[0].Binding,
    r => r.Rows[0].Env.cells[0] = cell('text', '\ud800'),
  ]) {
    const r = original(); mutate(r);
    assert.throws(() => editor.validateSIVIParentOriginal(r));
  }
  const r = original();
  const copy = editor.validateSIVIParentOriginal(r);
  copy.Bindings[0].Binding = 'foreign';
  assert.equal(r.Bindings[0].Binding, 'PlotNumber');
});
test('TEXT bounds use UTF16 units without recasing trimming repair or catalogue membership', () => {
  const limits = { SV_PolygonNumber: 25, SV_CanopyComposition: 50, SnowCoverregime: 1,
    SV_RootZoneTexture: 100, SV_AhorizonType: 5 };
  for (const [column, maximum] of Object.entries(limits)) {
    for (const raw of ['X'.repeat(maximum), '🌲'.repeat(Math.floor(maximum / 2)) + (maximum % 2 ? 'x' : ''), ' ']) {
      const r = original(), d = editor.stageSIVIParent(r, {}, column, { kind: 'text', raw });
      assert.equal(editor.siviParentErrors(d).length, 0);
      assert.equal(d[column].value.text, raw);
      assert.doesNotThrow(() => editor.siviParentProposal(r, d));
    }
    for (const raw of ['x'.repeat(maximum + 1), '🌲'.repeat(Math.floor(maximum / 2) + 1), '\ud800', '\udc00', 'a\0b']) {
      const r = original(), d = editor.stageSIVIParent(r, {}, column, { kind: 'text', raw });
      assert.equal(d[column].input.raw, raw);
      assert.equal(editor.siviParentDirty(d), true);
      assert.throws(() => editor.siviParentProposal(r, JSON.parse(JSON.stringify(d))));
    }
  }
});
test('edited categorical clear and selected empty remain distinct from unchanged empty originals', () => {
  for (const column of ['SnowCoverregime', 'SV_RootZoneTexture', 'SV_AhorizonType']) {
    const r = original(); setCell(r, column, cell('text', ''));
    const keep = editor.stageSIVIParent(r, {}, column, { kind: 'original' });
    assert.equal(editor.siviParentDirty(keep), false);
    const cleared = editor.stageSIVIParent(r, {}, column, { kind: 'text', raw: '' });
    assert.equal(editor.siviParentProposal(r, cleared).categorical[0].value.storage, 'null');
    const explicitClear = editor.stageSIVIParent(r, {}, column, { kind: 'clear' });
    assert.equal(editor.siviParentProposal(r, explicitClear).categorical[0].value.storage, 'null');
    setCell(r, column, cell('text', 'X'));
    const typedClear = editor.stageSIVIParent(r, {}, column, { kind: 'text', raw: '' });
    assert.equal(editor.siviParentProposal(r, typedClear).categorical[0].value.storage, 'null');
    const empty = editor.stageSIVIParent(r, {}, column, { kind: 'empty' });
    assert.equal(editor.siviParentProposal(r, empty).categorical[0].value.text, '');
  }
});
test('numeric BOOLEAN and action domains preserve exact accepted storage and reject wrong intent', () => {
  const r = original();
  for (const [option, literal] of [[1, 'Ground'], [2, 'Visual'], [3, 'Note'], [4, 'FS882'], [5, 'Other']]) {
    const d = editor.stageSIVIParent(r, {}, 'PlotType', { kind: 'option', option });
    assert.equal(d.PlotType.value.text, literal);
    assert.equal(editor.siviParentProposal(r, d).actions[0].option, option);
  }
  for (const [option, integer] of [[1, '-1'], [2, '0'], [null, null]]) {
    const d = editor.stageSIVIParent(r, {}, 'SpeciesListComplete', { kind: 'option', option });
    assert.equal(d.SpeciesListComplete.value.integer, integer);
    assert.equal(d.SpeciesListComplete.value.storage, option === null ? 'null' : 'integer');
  }
  assert.equal(editor.stageSIVIParent(r, {}, 'SV_FloodPlain', { kind: 'boolean', value: false }).SV_FloodPlain.value.integer, '0');
  for (const [column, domain] of [['SV_FloodPlain', 'scalars'], ['SV_StandAgeEstMeas', 'options'],
    ['SV_PolygonNumber', 'text'], ['SpeciesListComplete', 'actions']]) {
    const nonnull = original(); setCell(nonnull, column, cell('integer', '-1'));
    const d = editor.stageSIVIParent(nonnull, {}, column, { kind: 'clear' });
    const proposal = editor.siviParentProposal(nonnull, d);
    assert.equal(proposal[domain].length, 1);
    if (domain === 'actions') assert.equal(proposal.actions[0].option, null);
    else assert.equal(proposal[domain][0].value.storage, 'null');
  }
  for (const [column, input] of [
    ['PlotType', { kind: 'clear' }], ['PlotType', { kind: 'option', option: null }],
    ['PlotType', { kind: 'text', raw: 'Ground' }], ['PlotType', { kind: 'option', option: 6 }],
    ['SpeciesListComplete', { kind: 'option', option: 3 }], ['SV_StandAgeEstMeas', { kind: 'option', option: 0 }],
    ['SV_FloodPlain', { kind: 'text', raw: '-1' }], ['SV_StandHeight', { kind: 'boolean', value: true }],
    ['SV_PolygonNumber', { kind: 'empty' }], ['SV_StandHeight', { kind: 'option', option: 1 }],
    ...['NaN', '1bad', '1e999', '3.5e38', '\ud800'].map(raw => ['SV_StandHeight', { kind: 'text', raw }]),
  ]) assert.throws(() => editor.siviParentProposal(r, editor.stageSIVIParent(r, {}, column, input)));
});
test('unchanged invalid history is omitted without imposing height-specific BLOB restrictions', () => {
  for (const column of Object.keys(inputs)) {
    for (const historical of [cell('text', 'x'.repeat(300)), cell('integer', '7'), cell('real', 3.5e38), cell('blob', 'ff')]) {
      const r = original(); setCell(r, column, historical);
      const kept = editor.stageSIVIParent(r, {}, column, { kind: 'original' });
      assert.equal(editor.siviParentDirty(kept), false);
      assert.equal(Object.values(editor.siviParentProposal(r, kept)).filter(Array.isArray).flat().length, 0);
      const corrected = editor.stageSIVIParent(r, kept, column, inputs[column]);
      assert.doesNotThrow(() => editor.siviParentProposal(r, corrected));
    }
  }
});
test('stale originals and altered serialized intent cannot authorize a proposal', () => {
  const r = original(), d = editor.stageSIVIParent(r, {}, 'SV_StandHeight', inputs.SV_StandHeight);
  for (const mutate of [
    c => c.ContextID = 'context:new', c => c.Rows[0].Env.rowId = '1',
    c => setCell(c, 'SV_StandHeight', cell('real', 10)),
  ]) {
    const changed = structuredClone(r); mutate(changed);
    assert.throws(() => editor.siviParentProposal(changed, d));
    assert.throws(() => editor.stageSIVIParent(changed, d, 'SV_StandHeight', { kind: 'clear' }));
  }
  for (const mutate of [
    d => d.SV_StandHeight.value.real = 9, d => d.SV_StandHeight.input.raw = '7',
    d => d.SV_StandHeight.contextId = 'foreign', d => d.SV_StandHeight.controlId = 'foreign',
    d => d.SV_StandHeight.error = 'invented',
    d => d.SV_StandHeight.expected = cell('real', 99), d => d.Unavailable = d.SV_StandHeight,
  ]) {
    const bad = structuredClone(d); mutate(bad);
    assert.throws(() => editor.siviParentProposal(r, bad));
  }
});
test('owned session keeps remount errors and drafts blocks Save/Lock/close and Undo uses exact editor identity', () => {
  for (const editorId of ['', null, {}, 1, '\ud800', 'editor\0id']) {
    assert.throws(() => new SIVIParentSession(editorId, original()), /valid editor identity/);
  }
  const session = new SIVIParentSession('editor:1', original());
  assert.equal(session.closeState().canSave, false);
  assert.equal(closeDisposition(session.closeState(), false), 'clean');
  session.stage('editor:1', 'SV_PolygonNumber', { kind: 'text', raw: 'x'.repeat(26) });
  session.stage('editor:1', 'SV_AhorizonType', { kind: 'text', raw: '\ud800' });
  const remount = session.view();
  assert.equal(JSON.parse(JSON.stringify(remount)).drafts.SV_AhorizonType.input.raw, '\ud800');
  assert.equal(closeDisposition(session.closeState(), false), 'busy');
  assert.equal(session.closeState().blocked, true);
  assert.equal(session.closeState().canSave, false);
  assert.throws(() => session.proposal('editor:1'));
  assert.throws(() => session.undo('editor:other'));
  assert.throws(() => session.replaceOriginal('editor:1', original()));
  assert.throws(() => session.dispose('editor:1'));
  session.stage('editor:1', 'SV_PolygonNumber', { kind: 'text', raw: 'valid' });
  assert.equal(session.closeState().blocked, true);
  session.stage('editor:1', 'SV_AhorizonType', { kind: 'text', raw: 'Ae' });
  assert.equal(session.closeState().blocked, false);
  assert.equal(session.closeState().error, null);
  assert.equal(closeDisposition(session.closeState(), false), 'dirty');
  const view = session.view(); view.drafts.SV_AhorizonType.value.text = 'foreign';
  assert.equal(session.proposal('editor:1').categorical[0].value.text, 'Ae');
  session.undo('editor:1');
  assert.equal(closeDisposition(session.closeState(), false), 'clean');
  assert.equal(session.closeState().error, null);
  const next = original(); next.ContextID = 'context:next';
  session.replaceOriginal('editor:1', next);
  assert.equal(session.view().original.ContextID, 'context:next');
  session.dispose('editor:1');
  assert.throws(() => session.stage('editor:1', 'SV_StandHeight', inputs.SV_StandHeight));
});
test('parent wire boundary requires independent active identity and complete raw payload before drafts exist', () => {
  const r = original(), owner = { contextId: r.ContextID, project: r.Project, plot: r.Plot };
  const wire = JSON.parse(JSON.stringify(r));
  const session = transport.siviParentSessionFromWire('editor:transport', wire, owner);
  assert.equal(session.closeState().unsaved, false);
  assert.equal(session.closeState().canSave, false);
  wire.ContextID = 'caller';
  assert.equal(session.view().original.ContextID, r.ContextID);
  for (const key of ['contextId', 'project', 'plot']) {
    assert.throws(() => transport.siviParentOriginalFromWire(r, { ...owner, [key]: 'foreign' }));
  }
  for (const bad of [null, undefined, [], 'original', 0, {}]) {
    assert.throws(() => transport.siviParentSessionFromWire('editor:transport', bad, owner));
  }
  for (const mutate of [
    r => r.EnvColumns = null, r => r.AdminColumns[0] = null, r => delete r.AdminColumns[0].declaredType,
    r => r.Rows = null, r => r.Rows[0] = null, r => r.Rows[0].Env = null,
    r => r.Rows[0].Env.cells = null, r => r.Rows[0].Env.cells[0] = null,
    r => delete r.Rows[0].Env.cells[0].blobHex, r => r.Rows[0].Env.rowId = 1,
    r => r.Bindings = null, r => r.Bindings[0] = null, r => delete r.Bindings[0].Implicit,
    r => r.Bindings[0].Column = '0', r => r.Bindings[0].Column = 0.5,
    r => r.Rows[0].Env.cells[0].text = '\ud800',
    r => delete r.Bindings[0], r => delete r.Rows[0], r => delete r.EnvColumns[0],
    r => {
      r.EnvColumns.push({ name: 'UnboundPhysicalColumn', declaredType: 'TEXT' });
      r.Rows[0].Env.cells.length += 1;
    },
  ]) {
    const malformed = original(); mutate(malformed);
    assert.throws(() => transport.siviParentSessionFromWire('editor:transport', malformed, owner));
    assert.throws(() => editor.validateSIVIParentOriginal(malformed));
  }
});
test('source action display is read-only literal mapping with explicit unsupported-original diagnostics', () => {
  for (const [literal, option] of [['Ground', 1], ['Visual', 2], ['Note', 3], ['FS882', 4], ['Other', 5]]) {
    for (const [integer, speciesOption] of [['-1', 1], ['0', 2]]) {
      const r = original(); setCell(r, 'PlotType', cell('text', literal));
      setCell(r, 'SpeciesListComplete', cell('integer', integer));
      const before = JSON.stringify(r), display = transport.siviParentActionDisplay(r);
      assert.equal(display[0].controlId, 'form:frmSIVIsite/optPlotType');
      assert.equal(display[0].option, option);
      assert.equal(display[1].controlId, 'form:frmSIVIsite/optSpeciesListComplete');
      assert.equal(display[1].option, speciesOption);
      assert.equal(display[0].diagnostic, null);
      assert.equal(display[1].diagnostic, null);
      const session = new SIVIParentSession('editor:display', r);
      assert.equal(session.closeState().unsaved, false);
      assert.equal(session.proposal('editor:display').actions.length, 0);
      display[0].expected.text = 'caller';
      assert.equal(JSON.stringify(r), before);
    }
  }
  assert.equal(transport.siviParentActionDisplay(original())[0].diagnostic, null);
  for (const [column, index, values] of [
    ['PlotType', 0, [cell('text', ''), cell('text', 'ground'), cell('text', 'Ground '), cell('text', 'Grnd'),
      cell('text', 'Full'), cell('integer', '1'), cell('blob', 'ff')]],
    ['SpeciesListComplete', 1, [cell('integer', '1'), cell('integer', '2'), cell('real', -1), cell('text', '-1'), cell('blob', 'ff')]],
  ]) {
    for (const historical of values) {
      const r = original(); setCell(r, column, historical);
      const display = transport.siviParentActionDisplay(r)[index];
      assert.equal(display.option, null);
      assert.match(display.diagnostic, /no verified literal display mapping/);
      assert.deepEqual(display.expected, historical);
      assert.equal(editor.siviParentProposal(r, {}).actions.length, 0);
    }
  }
});
