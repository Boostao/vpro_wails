const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const { render } = require('svelte/server');
const { loadTypeScript, serverComponent, presentationHelpers } = require('./svelteTestHelpers.cjs');
const editor = loadTypeScript('vegetationSpeciesEditor.ts', {
  './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
});
const json = value => JSON.parse(JSON.stringify(value));
const options = [{ code: 'A', scientificName: null, englishName: '', lifeform: 3, codeType: 'U' },
  { code: 'A', scientificName: 'duplicate', englishName: null, lifeform: 3, codeType: 'U' },
  { code: null, scientificName: 'NULL code', englishName: null, lifeform: null, codeType: null }];
const lists = Object.fromEntries(editor.speciesForms.map(form => [form, options]));
const personal = loadTypeScript('personalSpeciesEditor.ts', {
  './vegetationSpeciesEditor': editor,
  './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
});
const creation = loadTypeScript('vegetationCreationEditor.ts', {
  './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  './vegetationSpeciesEditor': editor,
  './qualityEditor': loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') }),
});

test('New-row personal metadata captures a distinct editor identity without inventing a physical row ID', () => {
  let proposal = creation.stageVegetationCreationSpecies(creation.beginVegetationCreation('SubVegAXL_BC', ['Cover1']), 'zznew02');
  const choices = { form: proposal.form, entered: proposal.species, aliases: [{ code: null }], users: [] };
  const metadata = personal.beginCreationPersonalSpecies(proposal, choices);
  assert.deepEqual(json(metadata.source), { kind: 'creation', editorKey: proposal.editorKey });
  assert.equal(Object.hasOwn(metadata, 'id'), false);
  assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, json(proposal)), true);
  assert.equal(personal.matchesPersonalSpeciesSource(metadata, { form: proposal.form, raw: proposal.species, expected: '', error: 'unknown' }), false);
  const other = creation.stageVegetationCreationSpecies(creation.beginVegetationCreation(proposal.form, ['Cover1']), proposal.species);
  assert.notEqual(other.editorKey, proposal.editorKey);
  assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, other), false);
  assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, null), false);
  for (const changed of [{ form: 'SubVegDXL' }, { species: 'changed' }, { decision: { kind: 'keep', entered: proposal.species } }]) {
    assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, { ...proposal, ...changed }), false);
  }
  assert.throws(() => personal.beginCreationPersonalSpecies(proposal, { ...choices, aliases: [{ code: 'MASTER' }] }));
  assert.throws(() => personal.beginCreationPersonalSpecies(proposal, { ...choices, users: [{ code: 'USER' }] }));
  assert.throws(() => personal.beginCreationPersonalSpecies(proposal, { ...choices, entered: 'changed' }));
  const existing = personal.beginPersonalSpecies(0, { form: proposal.form, raw: proposal.species, expected: 'RAW', error: 'unknown' }, choices);
  assert.equal(personal.matchesCreationPersonalSpeciesSource(existing, proposal), false);
});

test('New-row metadata save stages a separate personal decision and transport never sends editor identity', () => {
  let proposal = creation.stageVegetationCreationSpecies(creation.beginVegetationCreation('SubVegCXL', ['Cover6']), 'zznew02');
  const choices = { form: proposal.form, entered: proposal.species, aliases: [], users: [] };
  let metadata = personal.beginCreationPersonalSpecies(proposal, choices);
  metadata = personal.stagePersonalText(metadata, 'englishName', '', false);
  metadata = personal.stagePersonalText(metadata, 'scientificName', '  Raw name é  ', false);
  const request = personal.personalSpeciesRequest(json(metadata));
  assert.deepEqual(json(request), { entered: 'zznew02', scientificName: '  Raw name é  ', englishName: '', lifeform: null });
  proposal = creation.stageVegetationCreation(proposal, 'Cover6', '0');
  assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, proposal), true);
  const saved = { code: 'ZZNEW02', scientificName: request.scientificName, englishName: '', lifeform: null, codeType: null };
  assert.equal(personal.personalSpeciesMatches(saved, request), true);
  proposal = creation.chooseVegetationCreationSpecies(proposal, { ...choices, users: [saved] }, 'user', saved.code);
  assert.equal(personal.matchesCreationPersonalSpeciesSource(metadata, proposal), false);
  assert.deepEqual(json(creation.vegetationCreationRequest(proposal, [])), { form: proposal.form, species: 'ZZNEW02',
    decision: 'user', entered: 'zznew02', selected: 'ZZNEW02', values: { cover6: 0 } });
  assert.equal(Object.hasOwn(creation.vegetationCreationRequest(proposal, []), 'editorKey'), false);
});

test('New-row metadata owns separate Save/Undo gates and retains deliberately saved definitions on proposal cancellation', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /personalSourceMatches\(proposed\)/);
  assert.match(form, /matchesCreationPersonalSpeciesSource\(proposed, creationDraft\)/);
  assert.match(form, /data-personal-create-source=\{creationDraft\.form\}/);
  assert.match(form, /if \(personalDraft !== null\) \{ error = 'Cancel personal definition entry before cancelling the new-row proposal/);
  assert.match(form, /creationDraft = chooseVegetationCreationSpecies\(creationDraft, reloaded, 'user'/);
  assert.match(form, /No project data or history changed; explicitly saved personal definitions remain available/);
  assert.match(form, /Save new vegetation record[\s\S]*?Cancel vegetation creation/);
  assert.doesNotMatch(form, /proposed\.id|personalDraft\.id/);
});

test('Personal metadata starts only from the reviewed unknown source and retains original row identity', () => {
  const drafts = editor.stageSpecies({}, 'SubVegAXL_BC', -9, 'zznew01', 'RAW', lists);
  assert.doesNotMatch(drafts['-9'].error, /creation is unavailable/);
  const choices = { form: 'SubVegAXL_BC', entered: 'zznew01', aliases: [], users: [] };
  const draft = personal.beginPersonalSpecies(-9, drafts['-9'], choices);
  assert.equal(personal.matchesPersonalSpeciesSource(draft, drafts['-9']), true);
  assert.equal(personal.matchesPersonalSpeciesSource(draft, { ...drafts['-9'], expected: 'changed' }), false);
  assert.equal(personal.matchesPersonalSpeciesSource(draft, { ...drafts['-9'], raw: 'changed' }), false);
  assert.throws(() => personal.beginPersonalSpecies(-9, drafts['-9'], { ...choices, aliases: [{ code: 'MASTER' }] }));
  assert.throws(() => personal.beginPersonalSpecies(-9, drafts['-9'], { ...choices, users: [{ code: 'USER' }] }));
  assert.doesNotThrow(() => personal.beginPersonalSpecies(-9, drafts['-9'], { ...choices, aliases: [{ code: null }] }));
  assert.deepEqual(json(personal.personalSpeciesRequest(draft)), { entered: 'zznew01', scientificName: null, lifeform: null, englishName: null });
});

test('Personal metadata retains raw errors, UTF-16 bounds and distinct NULL/empty values through remount', () => {
  const cell = editor.stageSpecies({}, 'SubVegCXL', 0, 'zznew01', 'RAW', lists)['0'];
  let draft = personal.beginPersonalSpecies(0, cell, { form: cell.form, entered: cell.raw, aliases: [], users: [] });
  for (const raw of ['\ud800', '😀'.repeat(128)]) {
    draft = personal.stagePersonalText(draft, 'scientificName', raw, false);
    const remounted = json(draft);
    assert.equal(remounted.scientificName.raw, raw);
    assert.equal(personal.personalSpeciesErrors(remounted).length, 1);
    assert.throws(() => personal.personalSpeciesRequest(remounted));
  }
  draft = personal.stagePersonalText(draft, 'scientificName', '😀'.repeat(127) + 'x', false);
  draft = personal.stagePersonalText(draft, 'englishName', '', false);
  assert.equal(personal.personalSpeciesErrors(draft).length, 0);
  assert.equal(personal.personalSpeciesRequest(draft).englishName, '');
  draft = personal.stagePersonalText(draft, 'scientificName', '\ud800', true);
  assert.equal(personal.personalSpeciesErrors(draft).length, 0);
  assert.equal(personal.personalSpeciesRequest(draft).scientificName, null);
  assert.equal(draft.scientificName.raw, '\ud800');
  assert.equal(personal.personalLifeforms.length, 12);
  assert.throws(() => personal.personalSpeciesRequest({ ...draft, lifeform: 0 }));
});

test('Personal creation checks independent metadata and only recognizes explicit committed-cleanup errors', () => {
  const request = { entered: 'zznew01', scientificName: null, lifeform: null, englishName: '' };
  const option = { code: 'ZZNEW01', scientificName: null, lifeform: null, englishName: '', codeType: null };
  assert.equal(personal.personalSpeciesMatches(option, request), true);
  for (const changed of [{ scientificName: '' }, { code: 'zznew01' }, { lifeform: 0 }, { codeType: 'u' }]) {
    assert.equal(personal.personalSpeciesMatches({ ...option, ...changed }, request), false);
  }
  assert.equal(personal.personalSpeciesCommittedError({ message: 'personal species definition committed, but writer cleanup failed; details' }), true);
  assert.equal(personal.personalSpeciesCommittedError({ message: 'personal species audit failed: personal species definition committed, but writer cleanup failed;' }), false);
  assert.equal(personal.personalSpeciesCommittedError('network error'), false);
});

test('Personal definition has separate explicit save, persistent metadata and native close/Undo gates', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /VITE_PERSONAL_SPECIES_EDITING === 'true'/);
  assert.match(form, /personalDraft !== null \? 'Save the personal definition explicitly/);
  assert.match(form, /if \(personalDraft !== null\) \{ cancelPersonalSpecies\(\); return; \}/);
  assert.match(form, /ordinary plot Save never writes the user database/);
  assert.match(form, /await PlotService\.CreatePersonalSpeciesDefinition\(request\)/);
  assert.match(form, /users\.filter\(option => personalSpeciesMatches\(option, request\)\)\.length !== 1/);
  assert.match(form, /committed = committed \|\| personalSpeciesCommittedError\(cause\)/);
  assert.match(form, /if \(committed\) \{ personalDraft = null; capabilitiesReady = false; \}/);
  assert.match(form, /personal-species-draft[\s\S]*?Save personal definition only/);
  assert.doesNotMatch(form, /personal-[\s\S]{0,100}maxlength=/);
});

test('Species drafts preserve original identity, raw errors and exact list membership across all grids', () => {
  for (const form of editor.speciesForms) {
    for (const raw of ['', 'a', ' A', 'A ', "raw'", '123456789', '\ud800', '😀😀😀😀X']) {
      const drafts = editor.stageSpecies({}, form, 0, raw, 'RAW', lists);
      assert.equal(drafts['0'].raw, raw);
      assert.equal(editor.speciesDirty(drafts), true);
      assert.equal(editor.speciesErrors(drafts).length, 1);
      assert.throws(() => editor.speciesUpdates(drafts));
      const corrected = editor.stageSpecies(json(drafts), form, 0, 'A', 'refreshed', lists);
      assert.equal(editor.speciesErrors(corrected).length, 0);
      assert.deepEqual(json(editor.speciesUpdates(corrected)), [{ id: 0, form, expected: 'RAW', value: 'A' }]);
      const undone = editor.stageSpecies(corrected, form, 0, 'RAW', 'refreshed', {});
      assert.equal(editor.speciesDirty(undone), false);
      assert.deepEqual(json(editor.speciesUpdates(undone)), []);
    }
    const unchanged = editor.stageSpecies({}, form, -9, 'historical invalid', 'historical invalid', {});
    assert.equal(editor.speciesDirty(unchanged), false);
    assert.deepEqual(json(editor.speciesUpdates(unchanged)), []);
    const unavailable = editor.stageSpecies({}, form, 0, 'A', 'RAW', {});
    assert.match(unavailable['0'].error, /references are unavailable/);
  }
  for (const id of [0.5, 2147483648, -2147483649, NaN]) assert.throws(() => editor.stageSpecies({}, 'SubVegCXL', id, 'A', 'RAW', lists));
  assert.throws(() => editor.stageSpecies({}, 'SubVegAXL', 0, 'A', 'RAW', lists));
  let drafts = editor.stageSpecies({}, 'SubVegAXL_BC', 0, 'A', 'RAW', lists);
  drafts = editor.stageSpecies(drafts, 'SubVegAhtXL', 0, 'A', 'changed elsewhere', lists);
  assert.equal(drafts['0'].expected, 'RAW');
  assert.equal(drafts['0'].form, 'SubVegAhtXL');
});

test('Species source renderers keep one live labelled control and nullable duplicate metadata', () => {
  const { paper, presentation } = presentationHelpers();
  const SourceChild = serverComponent(readFileSync(path.join(__dirname, 'SourceChild.svelte'), 'utf8'), 'SourceChild.svelte', {
    './paperLayout': paper, './formPresentation': presentation,
    './heightEditor': loadTypeScript('heightEditor.ts', { './numericEditor': loadTypeScript('numericEditor.ts') }),
  });
  for (const name of editor.speciesForms) {
    const readonly = render(SourceChild, { props: {
      name, rows: [{ id: 0, values: { species: 'RAW' } }], disabled: false, onedit() {},
    } }).body;
    assert.match(readonly, /<input\b[^>]*value="RAW"[^>]* disabled[^>]*data-column="Species"/);
    for (const disabled of [false, true]) {
      const html = render(SourceChild, { props: {
        name, rows: [{ id: 0, values: { species: 'RAW' } }, { id: -9, values: { species: 'RAW' } }],
        disabled: true, speciesDisabled: disabled, onspeciesstage() {}, speciesLists: lists,
        speciesDrafts: { '0': { form: name, raw: 'bad', expected: 'RAW', error: 'Invalid species' } },
      } }).body;
      assert.equal((html.match(/<input\b[^>]*data-column="Species"/g) || []).length, 2);
      assert.match(html, /value="bad"[^>]*aria-invalid="true"/);
      assert.match(html, /aria-label="Species, row 0"/);
      assert.equal((html.match(/<datalist\b/g) || []).length, 1);
      assert.equal((html.match(/<option value="A"/g) || []).length, 2);
      assert.doesNotMatch(html, /<option value="null"/);
      assert.match(html, /NULL \|  \| Lifeform 3/);
      assert.equal(/<input\b[^>]*value="bad"[^>]* disabled/.test(html), disabled);
    }
  }
});

test('Explicit replacement, keep and personal choices preserve ambiguity, original entry and source-event case changes', () => {
  const form = 'SubVegAXL_BC';
  const choices = { form, entered: 'olddup', aliases: [
    { ...options[0], code: 'new_a', oldCode: 'olddup' },
    { ...options[1], code: 'NEW_B', oldCode: 'olddup' },
    { ...options[2], oldCode: 'olddup' },
  ], users: [{ ...options[0], code: 'olddup' }] };
  const drafts = editor.stageSpecies({}, form, 0, 'olddup', 'ORIGINAL', lists);
  for (const [kind, selected, value] of [['replace', 'new_a', 'NEW_A'], ['replace', 'NEW_B', 'NEW_B'], ['keep', undefined, 'OLDDUP']]) {
    const chosen = editor.chooseSpecies(json(drafts), 0, choices, kind, selected);
    assert.equal(chosen['0'].raw, value);
    assert.equal(chosen['0'].expected, 'ORIGINAL');
    assert.equal(editor.speciesErrors(chosen).length, 0);
    assert.deepEqual(json(editor.speciesUpdates(chosen)), [{
      id: 0, form, expected: 'ORIGINAL', value, decision: kind, entered: 'olddup',
      ...(selected === undefined ? {} : { selected }),
    }]);
    const edited = editor.stageSpecies(chosen, form, 0, 'bad again', 'different server value', lists);
    assert.equal(edited['0'].expected, 'ORIGINAL');
    assert.equal(edited['0'].decision, undefined);
    assert.equal(editor.speciesErrors(edited).length, 1);
  }
  assert.throws(() => editor.chooseSpecies(drafts, 0, choices, 'replace', 'NEW_A'));
  assert.throws(() => editor.chooseSpecies(drafts, 0, choices, 'replace', 'guessed'));
  assert.throws(() => editor.chooseSpecies(drafts, 0, choices, 'user', 'olddup'));
  assert.throws(() => editor.chooseSpecies(drafts, 0, { ...choices, entered: 'changed' }, 'keep'));
  assert.throws(() => editor.chooseSpecies(drafts, 0, { ...choices, form: 'SubVegCXL' }, 'keep'));
  assert.throws(() => editor.chooseSpecies(drafts, 0, { ...choices, aliases: [choices.aliases[2]] }, 'keep'));
  const userChoices = { form, entered: 'personal', aliases: [], users: [{ ...options[0], code: 'Personal', lifeform: 99, codeType: 'S' }] };
  const personal = editor.stageSpecies({}, form, -9, 'personal', 'RAW', lists);
  const chosen = editor.chooseSpecies(personal, -9, userChoices, 'user', 'Personal');
  assert.deepEqual(json(editor.speciesUpdates(chosen)), [{
    id: -9, form, expected: 'RAW', value: 'PERSONAL', decision: 'user', entered: 'personal', selected: 'Personal',
  }]);
  for (const code of [null, '', '\ud800', '123456789', 'ÉLI', '😀']) assert.notEqual(editor.speciesEventError(code), null);
  assert.equal(editor.speciesEventError("  raw' "), null);
});

test('Species drafts gate every session and survive tab remount, Save, Undo, Lock and native close', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /VITE_VEGETATION_SPECIES_EDITING !== 'false'/);
  assert.match(form, /let speciesDrafts = \$state<SpeciesDrafts>/);
  assert.match(form, /childUnsaved = \$derived\([^;]*speciesUnsaved \|\| deletionReview !== null \|\| creationDraft !== null \|\| codeCheckOpen\)/);
  for (const name of ['height', 'other', 'soil', 'attribute', 'collected']) {
    assert.match(form, new RegExp(`const ${name}EditingDisabled = \\$derived\\([^;]*speciesUnsaved\\)`));
  }
  assert.match(form, /if \(speciesUnsaved\) \{ await saveSpeciesDrafts\(\); return; \}/);
  assert.match(form, /if \(speciesUnsaved\) \{ void cancelSpeciesDrafts\(\); return; \}/);
  assert.match(form, /Save or Cancel species drafts before changing the plot lock/);
  assert.match(form, /speciesInvalid\.length > 0 \? 'Correct invalid species drafts/);
  assert.match(form, /speciesUnsaved && \(!speciesReferenceReady \|\| speciesReferenceBusy\)/);
  assert.match(form, /await PlotService\.UpdateVegetationSpecies[\s\S]*?committed = true;[\s\S]*?speciesDrafts = \{\}/);
  assert.match(form, /Species save failed; drafts retained/);
  assert.match(form, /Species changes committed, but refresh failed/);
  assert.match(form, /onspeciesstage=\{speciesEditingEnabled \? stageSpeciesCell : undefined\}/);
  assert.match(form, /Species requires the source-list draft workflow; unrestricted source-grid edits are unavailable/);
  assert.match(form, /speciesDecisionBusy = \$state\(false\)/);
  assert.match(form, /speciesChoiceRequest\+\+; speciesChoiceReads\.cancelAll\(\)/);
  assert.match(form, /if \(!cell\.decision\) drafts = stageSpecies/);
  assert.match(form, /const users = aliases\.some\(option => option\.code !== null\) \? \[\]/);
  assert.match(form, /Personal-list creation is unavailable; correct or Cancel the draft/);
});
