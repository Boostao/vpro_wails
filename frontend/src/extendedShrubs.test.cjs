const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { render } = require('svelte/server');
const { compile } = require('svelte/compiler');
const { loadTypeScript, presentationHelpers, serverComponent, componentFunctions } = require('./svelteTestHelpers.cjs');
const numeric = loadTypeScript('numericEditor.ts');
const height = loadTypeScript('heightEditor.ts', { './numericEditor': numeric });
const { paper, presentation } = presentationHelpers();
const json = value => JSON.parse(JSON.stringify(value));
const read = file => readFileSync(path.join(__dirname, file), 'utf8');

test('extended shrubs retain source bindings, labels, numeric order and independent totals', () => {
  const controls = paper.paperChild('SubVegAXL').controls;
  const columns = controls.filter(control => control.column).sort((a, b) => a.tabOrder - b.tabOrder);
  const numeric = columns.filter(control => height.heightField(control.column));
  assert.deepEqual(json(numeric.map(control => control.column)),
    ['Cover1', 'Cover2', 'cover3', 'TotalA', 'Cover4', 'Cover5', 'Cover5a', 'Cover5b', 'Cover5c', 'TotalB']);
  for (const [field, label] of [['Cover5a', 'B3'], ['Cover5b', 'B4'], ['Cover5c', 'B5']]) {
    const control = columns.find(item => item.column === field);
    assert.equal(control.caption, label);
    assert.equal(control.properties.ValidationRule.value, '<100 Or Is Null');
    assert.equal(control.locked, false);
    assert.equal(control.enabled, true);
  }
  assert.equal(paper.paperChild('SubVegAXL_BC').controls.some(control => control.column === 'Cover5a'), false);
});

test('extended fields share strict numeric drafts and preserve original identity through hide/remount/correction', () => {
  for (const field of ['cover5a', 'cover5b', 'cover5c']) {
    let drafts = height.stageHeight({}, -9, field, '100', null, 'SubVegAXL');
    drafts = json(drafts);
    assert.equal(height.heightDirty(drafts), true);
    assert.equal(drafts['-9'][field].raw, '100');
    assert.throws(() => height.vegetationNumberUpdates(drafts), /less than 100/);
    drafts = height.stageHeight(drafts, -9, field, '-1.234567890123', 999, 'SubVegAXL_BC');
    assert.equal(height.heightErrors(drafts).length, 0);
    assert.deepEqual(json(height.vegetationNumberUpdates(drafts)), [{
      id: -9, values: { [field]: -1.234567890123 }, expected: { [field]: null }, forms: { [field]: 'SubVegAXL' }
    }]);
    assert.ok(height.heightValue(field, '3.5e38').error);
    assert.ok(height.heightValue(field, '12x').error);
    assert.equal(height.heightValue(field, '99.999').value, 99.999);
    assert.equal(height.heightValue(field, '').value, null);
    assert.equal(height.heightDirty(height.stageHeight({}, -9, field, '', null, 'SubVegAXL')), false);
  }
  assert.match(height.aCoverSourceNotice('SubVegAXL', -9, { cover5a: 0 }), /all A\/B cover values are NULL/);
});

test('extended renderer has one labelled live control per field and normal renderer does not lose hidden drafts', () => {
  const child = serverComponent(read('SourceChild.svelte'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation, './heightEditor': height
  });
  const drafts = height.stageHeight({}, -9, 'cover5a', '100', 0, 'SubVegAXL');
  const props = { rows: [{ id: -9, values: { cover5a: 0, cover5b: null, cover5c: null } }],
    disabled: false, drafts, onstage() {} };
  const extended = render(child, { props: { ...props, name: 'SubVegAXL' } }).body;
  for (const [field, label] of [['Cover5a', 'B3'], ['Cover5b', 'B4'], ['Cover5c', 'B5']]) {
    assert.equal((extended.match(new RegExp(`data-column="${field}"`, 'g')) || []).length, 1);
    assert.match(extended, new RegExp(`aria-label="${label}, row -9"`));
  }
  assert.match(extended, /value="100"[^>]*aria-invalid="true"/);
  const normal = render(child, { props: { ...props, name: 'SubVegAXL_BC' } }).body;
  assert.doesNotMatch(normal, /data-column="Cover5a"/);
  assert.equal(height.heightErrors(drafts).length, 1);
  assert.equal(drafts['-9'].cover5a.form, 'SubVegAXL');
});

test('shared option gates extended presentation and restricts heights without clearing drafts or replacing parent identity', () => {
  const form = read('FS882Form.svelte');
  assert.match(form, /data-extended-shrubs/);
  assert.match(form, /Show extended shrub layers/);
  assert.match(form, /function setExtendedShrubs\(next: boolean\)/);
  assert.match(form, /if \(next\) vegetationMode = 'cover'/);
  assert.match(form, /onchange=\{\(event\) => setExtendedShrubs\(event\.currentTarget\.checked\)\}/);
  assert.equal((form.match(/onExtendedShrubsChange=\{extendedShrubsEnabled \? setExtendedShrubs : undefined\}/g) || []).length, 4);
  assert.match(form, /heightToggleDisabled=\{extendedShrubs \|\| busy \|\| headerWorkflowBusy\}/);
  assert.match(form, /extendedShrubs && originalChild\.form === 'SubVegAXL_BC' \? 'SubVegAXL'/);
  assert.match(form, /name === 'SubVegAXL_BC' \|\| name === 'SubVegAXL'/);
  assert.match(form, /hidden values, drafts and errors are retained/);
  assert.doesNotMatch(form, /onchange=\{[^}]*heightDrafts\s*=/);
  const header = read('HeaderEditor.svelte');
  assert.match(header, /data-column="SnowCoverregime"/);
  assert.match(header, /data-column="ActiveLayerDepth"/);
  const twoPage = read('TwoPageEntryEditor.svelte');
  assert.match(twoPage, /data-extended-shrubs/);
  assert.match(twoPage, /Show extended shrub layers/);
  for (const twoPageFile of ['TwoPageEntryEditor.svelte', 'TwoPageParentCommonEditor.svelte', 'TwoPageParentEditor.svelte']) {
  const tpSrc = read(twoPageFile);
  assert.doesNotMatch(tpSrc, /Normal two-page source|CHARS two-page source/, `${twoPageFile} must not render redundant CHARS source buttons`);
  }
  const siviShared = read('SIVIParentSharedFields.svelte');
  assert.match(siviShared, /data-sivi-extended-shrubs/);
  assert.match(siviShared, /data-sivi-arctic-fields/);
  assert.match(siviShared, /Show extended shrub layers/);
  for (const file of ['FS882Form.svelte', 'HeaderEditor.svelte', 'TwoPageEntryEditor.svelte', 'SIVIParentSharedFields.svelte', 'SourcePage.svelte', 'SourceChild.svelte']) {
    assert.equal(compile(read(file), { filename: file, generate: 'client' }).warnings.length, 0);
  }
});

test('setExtendedShrubs synchronizes all six SIVI vegetation sessions and forces cover mode when enabled', () => {
  const calls = [];
  const makeSession = name => ({ presentation: next => calls.push([name, next]) });
  const ctx = componentFunctions('FS882Form.svelte', ['setExtendedShrubs'], {
    extendedShrubs: false,
    vegetationMode: 'height',
    siviSession: makeSession('height'),
    siviCoverSession: makeSession('cover'),
    siviCombinedSession: makeSession('combined'),
    siviCollectedSession: makeSession('collected'),
    siviSpeciesSession: makeSession('species'),
    siviIdentitySession: makeSession('identity'),
  });
  ctx.actions.setExtendedShrubs(true);
  assert.equal(ctx.extendedShrubs, true);
  assert.equal(ctx.vegetationMode, 'cover');
  assert.deepEqual(calls, [
    ['height', true], ['cover', true], ['combined', true],
    ['collected', true], ['species', true], ['identity', true],
  ]);
  calls.length = 0;
  ctx.actions.setExtendedShrubs(false);
  assert.equal(ctx.extendedShrubs, false);
  assert.equal(ctx.vegetationMode, 'cover');
  assert.deepEqual(calls, [
    ['height', false], ['cover', false], ['combined', false],
    ['collected', false], ['species', false], ['identity', false],
  ]);
});

test('SnowCoverregime and ActiveLayerDepth stage, validate, and project non-blank values on FS882 and SIVI', () => {
  const bec = loadTypeScript('becEditor.ts');
  const quality = loadTypeScript('qualityEditor.ts', { './becEditor': bec });
  const ordinary = loadTypeScript('ordinaryEditor.ts', { './qualityEditor': quality, './numericEditor': numeric });
  let dirtyCount = 0;
  const validations = {};
  const ctx = componentFunctions('FS882Form.svelte', ['stageSnowCoverRegime', 'stageActiveLayerDepth'], {
    headerInputsDisabled: false,
    capabilities: { snowCoverRegime: true, activeLayerDepth: true },
    draft: { snowCoverRegime: null, activeLayerDepth: null },
    original: { snowCoverRegime: 'S', activeLayerDepth: 42.5 },
    activeLayerDepthRaw: null,
    ordinaryTextError: ordinary.ordinaryTextError,
    ordinaryNumberValue: ordinary.ordinaryNumberValue,
    validateHeader: (key, msg) => { if (msg === null) delete validations[key]; else validations[key] = msg; },
    markDirty: () => { dirtyCount++; },
  });
  ctx.actions.stageSnowCoverRegime('Z');
  assert.equal(ctx.draft.snowCoverRegime, 'Z');
  assert.equal(validations.snowCoverRegime, undefined);
  ctx.actions.stageSnowCoverRegime('AB');
  assert.match(validations.snowCoverRegime, /at most 1 UTF-16/);
  ctx.actions.stageSnowCoverRegime('');
  assert.equal(ctx.draft.snowCoverRegime, null);
  assert.equal(validations.snowCoverRegime, undefined);

  ctx.actions.stageActiveLayerDepth('35.25');
  assert.equal(ctx.draft.activeLayerDepth, 35.25);
  assert.equal(validations.activeLayerDepth, undefined);
  ctx.actions.stageActiveLayerDepth('3.5e38');
  assert.match(validations.activeLayerDepth, /Single/);
  ctx.actions.stageActiveLayerDepth('');
  assert.equal(ctx.draft.activeLayerDepth, null);
  assert.equal(validations.activeLayerDepth, undefined);
  assert.equal(dirtyCount, 6);

  const { editor, writeSession, transport, source, metadata } = require('./siviParentTestHelpers.cjs');
  const shared = loadTypeScript('siviParentSharedSession.ts', {
    '../../resources/fs1333-sivi-layout.json': source, './ordinaryEditor': ordinary,
    './projectMetadataEditor': metadata, './siviParentEditor': editor,
    './siviParentWriteSession': writeSession, './siviParentTransport': transport,
    './coordinateEditor': loadTypeScript('coordinateEditor.ts'),
    './siviDateTimestamp': loadTypeScript('siviDateTimestamp.ts'),
    './referenceEditor': loadTypeScript('referenceEditor.ts', {
      './qualityEditor': quality, './projectMetadataEditor': metadata, './siviParentTransport': transport,
    }),
  });
  const SIVIParentSharedFields = serverComponent(read('SIVIParentSharedFields.svelte'), 'SIVIParentSharedFields.svelte', {
    './projectMetadataEditor': metadata, './siviParentSharedSession': shared,
  });
  const siviMarkup = render(SIVIParentSharedFields, {
    props: {
      view: { original: null, drafts: {}, busy: false, operation: null, blocked: false, error: null, historyId: null },
      disabled: false, canSave: false, onstage() {}, onoperation() {}, oncancel() {},
      extendedShrubs: true, onExtendedShrubsChange() {},
      snowCoverRegime: 'S', activeLayerDepth: '42.5',
      onSnowCoverRegimeChange() {}, onActiveLayerDepthChange() {},
    },
  }).body;
  assert.match(siviMarkup, /id="sivi-arctic-SnowCoverregime"[^>]*value="S"/);
  assert.match(siviMarkup, /id="sivi-arctic-ActiveLayerDepth"[^>]*value="42\.5"/);
  assert.doesNotMatch(siviMarkup, /id="sivi-arctic-SnowCoverregime"[^>]*disabled/);
  assert.doesNotMatch(siviMarkup, /id="sivi-arctic-ActiveLayerDepth"[^>]*disabled/);
});
