const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const { loadTypeScript, componentFunctions } = require('./svelteTestHelpers.cjs');
const { transport, metadata, cell } = require('./siviParentTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const fixture = () => JSON.parse(readFileSync(path.join(__dirname, 'twoPageGoogleEarthWire.json'), 'utf8'));
const owner = { contextId: 'wire-owned', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', plot: '108050', form: 'FS882-8x6XL' };
const decode = text => text.replace(/&(amp|lt|gt|quot|apos|#x[\da-f]+|#\d+);/gi,
  (_, key) => key.startsWith('#') ? String.fromCodePoint(key.startsWith('#x') ? parseInt(key.slice(2), 16) : Number(key.slice(1))) :
    ({ amp: '&', lt: '<', gt: '>', quot: '"', apos: "'" })[key]);
class FixtureDOMParser {
  parseFromString(xml, type) {
    if (type === 'text/html') return { getElementsByTagName: () => [{ textContent: decode(xml.slice(10, -11)).replace(/\r\n?/g, '\n').replace(/^\n/, '') }] };
    assert.equal(type, 'application/xml');
    const namespace = xml.match(/<kml xmlns="([^"]+)"/)[1];
    const element = (localName, textContent = '', children = []) =>
      ({ localName, textContent, children, namespaceURI: namespace, attributes: [], getAttribute: () => null });
    const marks = [...xml.matchAll(/<Placemark><name>([\s\S]*?)<\/name><description>([\s\S]*?)<\/description><Point><coordinates>([\s\S]*?)<\/coordinates><\/Point><\/Placemark>/g)]
      .map(match => element('Placemark', '', [element('name', decode(match[1])), element('description', decode(match[2])),
        element('Point', '', [element('coordinates', match[3])])]));
    const body = element('Document', '', [element('name', decode(xml.match(/<Document><name>([\s\S]*?)<\/name>/)[1])), ...marks]);
    const root = element('kml', '', [body]); root.attributes = [{}]; root.getAttribute = () => namespace;
    const all = node => [node, ...node.children.flatMap(all)];
    return { documentElement: root, doctype: null, getElementsByTagName: name => name === '*' ? all(root) : [] };
  }
}
const kml = loadTypeScript('googleEarthKMLReview.ts', {
  './qualityEditor': quality, './googleEarthReview': { ownedScope: (value, scope) => Object.entries(scope).every(([key, expected]) => value[key] === expected) },
}, { DOMParser: FixtureDOMParser, TextEncoder, setTimeout });
const earth = loadTypeScript('twoPageGoogleEarth.ts', {
  './siviParentTransport': transport, './qualityEditor': quality, './googleEarthKMLReview': kml,
}, { DOMParser: FixtureDOMParser });
const validate = value => earth.twoPageGoogleEarthReviewFromWire(value, owner, 'PlotRepresenting', 'Fixture & café');

test('actual Go source options/review wire keeps complete physical fields, typed original and exact placemark', async () => {
  const wire = fixture(), before = JSON.stringify(wire);
  const options = earth.twoPageGoogleEarthOptionsFromWire(wire.options, owner);
  assert.equal(options.fields[0].declaredType, 'TEXT');
  const review = await validate(wire.review);
  assert.equal(earth.twoPageGoogleEarthCell(review, 'Longitude').real, 123.25);
  assert.equal(earth.twoPageGoogleEarthCell(review, 'Unmapped').blobHex, '00ff');
  assert.match(review.kml, /<coordinates>-123.25,53.5,0<\/coordinates>/);
  review.original.rows[0].cells[0].text = 'caller';
  options.fields[0].name = 'caller';
  assert.equal(JSON.stringify(wire), before);
});
test('actual Go description wires preserve leading LF, CRLF, CR and HTML entities without HTML normalization', async () => {
  const expected = ['\nSaved text', 'Saved\r\ntext', 'Saved\rtext', '\r\nSaved\r\ntext', `"quoted" & 'literal' <b>café</b>`];
  const cases = fixture().descriptionCases;
  assert.equal(cases.length, expected.length);
  for (const [index, value] of cases.entries()) {
    const result = await validate(value);
    assert.equal(earth.twoPageGoogleEarthCell(result, 'PlotRepresenting').text, expected[index]);
    assert.equal(result.kml, value.kml);
  }
});
for (const [name, mutate] of [
  ['foreign context', value => value.contextId = 'stale'], ['foreign path', value => value.projectPath += '.other'],
  ['foreign plot', value => value.plot = '0108050'], ['foreign form', value => value.form = 'unknown'],
  ['foreign title', value => value.title = 'default'], ['wrong byte count', value => value.byteCount++],
  ['wrong production column wire', value => { value.original.columns[0].type = 'TEXT'; delete value.original.columns[0].declaredType; }],
  ['duplicate schema', value => value.original.columns[1].name = 'PlotNumber'],
  ['malformed schema', value => value.original.columns[1].name = '\ud800'],
  ['empty physical rows', value => value.original.rows = []],
  ['extra physical row', value => value.original.rows.push(structuredClone(value.original.rows[0]))],
  ['numeric ownership', value => value.original.rows[0].cells[0] = cell('integer', '108050')],
  ['negative original west', value => value.original.rows[0].cells[1] = cell('real', -123.25)],
  ['NULL latitude', value => value.original.rows[0].cells[2] = cell()],
  ['malformed original text', value => value.original.rows[0].cells[3] = cell('text', '\ud800')],
  ['different coordinates', value => value.original.rows[0].cells[2] = cell('real', 54)],
  ['different description', value => value.original.rows[0].cells[3] = cell('text', 'changed')],
]) test(`source KML refuses ${name} without partial output`, async () => {
  const value = fixture().review; mutate(value); await assert.rejects(validate(value));
});
test('saved invalid options are retained, not repaired or silently defaulted', () => {
  const value = fixture().options;
  value.values = { title: 'old\x01title', descriptionField: 'missing\x00field' };
  const options = earth.twoPageGoogleEarthOptionsFromWire(value, owner);
  assert.equal(options.title, value.values.title);
  assert.equal(options.descriptionField, value.values.descriptionField);
  value.values.title = '\ud800';
  assert.throws(() => earth.twoPageGoogleEarthOptionsFromWire(value, owner));
});
test('real source panel compiles without presentation warnings', () => {
  const result = compile(readFileSync(path.join(__dirname, 'TwoPageGoogleEarthPanel.svelte'), 'utf8'), { generate: 'client' });
  assert.equal(result.warnings.length, 0);
});
async function mountPanel(service, validateReview = earth.twoPageGoogleEarthReviewFromWire) {
  const runtime = await import('svelte/internal/client');
  const { onDestroy } = await import(pathToFileURL(path.join(path.dirname(require.resolve('svelte')), 'index-client.js')).href);
  let script = readFileSync(path.join(__dirname, 'TwoPageGoogleEarthPanel.svelte'), 'utf8').match(/<script lang="ts">([\s\S]*?)<\/script>/)[1];
  script = script.replace('form, onBusyChange }:', 'form, onBusyChange, inspect }:');
  script += '\n$effect(() => inspect({ loadOptions, prepare, cancel, editTitle, editField, busy, error, review, options, title, descriptionField, current: () => review }));';
  const compiled = compile(`<script lang="ts">${script}</script>`, { generate: 'client' });
  const ast = ts.createSourceFile('saved-plot.js', compiled.js.code, ts.ScriptTarget.Latest, true);
  const code = ast.statements.filter(node => !ts.isImportDeclaration(node)).map(node => node.getText(ast)).join('\n')
    .replace('export default function', 'function');
  const reads = loadTypeScript('readRequests.ts', {
    '@wailsio/runtime': { Call: { RuntimeError: class extends Error {} }, CancelledRejectionError: class extends Error {} },
  });
  const mount = new Function('$', 'onDestroy', 'untrack', 'ReadRequests', 'TwoPageGoogleEarthService', 'googleEarthXMLText',
    'twoPageGoogleEarthOptionsFromWire', 'twoPageGoogleEarthReviewFromWire', 'twoPageGoogleEarthCell', 'metadataCellText',
    code + '\nreturn _unknown_;')(runtime, onDestroy, runtime.untrack, reads.ReadRequests, service, kml.googleEarthXMLText,
    earth.twoPageGoogleEarthOptionsFromWire, validateReview, earth.twoPageGoogleEarthCell, metadata.metadataCellText);
  let actions; const activity = [];
  const dispose = runtime.effect_root(() => mount(null, { ...owner, onBusyChange: value => activity.push(value), inspect: value => actions = value }));
  runtime.flush();
  return { actions: () => actions, activity, dispose, flush: runtime.flush };
}
test('actual client requires explicit options/preview and captures DOM correction before validation', async () => {
  const calls = [], wire = fixture();
  wire.options.values.title = 'invalid\x01saved';
  const panel = await mountPanel({
    GetOptions(...args) { calls.push(['options', ...args]); return Promise.resolve(wire.options); },
    GetReview(...args) { calls.push(['review', ...args]); return Promise.resolve(wire.review); },
  });
  try {
    assert.deepEqual(calls, []);
    await panel.actions().loadOptions(); panel.flush();
    assert.match(panel.actions().error, /retained literally/);
    await panel.actions().prepare(); panel.flush();
    assert.equal(calls.length, 1);
    panel.actions().editField({ currentTarget: { value: 'PlotRepresenting' } }); panel.flush();
    assert.match(panel.actions().error, /Place Name/);
    panel.actions().editTitle({ currentTarget: { value: 'Fixture & café' } }); panel.flush();
    panel.actions().editField({ currentTarget: { value: 'PlotRepresenting' } }); panel.flush();
    await panel.actions().prepare(); panel.flush();
    assert.equal(panel.actions().error, '');
    assert.equal(panel.actions().review.original.rows.length, 1);
    assert.doesNotThrow(() => structuredClone(panel.actions().review));
    assert.deepEqual(JSON.parse(calls[1][4]), { descriptionField: 'PlotRepresenting', title: 'Fixture & café' });
  } finally { panel.dispose(); }
});
test('actual client surfaces options and KML failures, then retries without automatic fallback', async () => {
  const wire = fixture(); wire.options.values = { title: wire.review.title, descriptionField: wire.review.descriptionField };
  let optionsFail = true, reviewFail = true;
  const panel = await mountPanel({
    GetOptions: () => optionsFail ? Promise.reject(new Error('options physical failure')) : Promise.resolve(wire.options),
    GetReview: () => reviewFail ? Promise.reject(new Error('coordinate unavailable')) : Promise.resolve(wire.review),
  });
  try {
    await panel.actions().loadOptions(); panel.flush();
    assert.match(panel.actions().error, /options physical failure/); assert.equal(panel.actions().options, null);
    assert.equal(panel.actions().busy, false);
    optionsFail = false; await panel.actions().loadOptions(); panel.flush();
    await panel.actions().prepare(); panel.flush();
    assert.match(panel.actions().error, /coordinate unavailable/); assert.equal(panel.actions().review, null);
    reviewFail = false; await panel.actions().prepare(); panel.flush();
    assert.equal(panel.actions().error, ''); assert.equal(panel.actions().busy, false);
    assert.equal(panel.actions().review.plot, owner.plot);
  } finally { panel.dispose(); }
});
test('actual client Cancel/remount rejects held saved options without populating defaults', async () => {
  let release, cancellations = 0;
  const held = new Promise(resolve => release = resolve); held.cancel = () => cancellations++;
  const panel = await mountPanel({ GetOptions: () => held });
  const pending = panel.actions().loadOptions(); panel.flush();
  const original = panel.actions(); original.cancel();
  const fresh = await mountPanel({ GetOptions: () => Promise.resolve(fixture().options) });
  try {
    release(fixture().options); await pending; panel.flush(); fresh.flush();
    assert.equal(cancellations, 1); assert.equal(original.options, null);
    assert.equal(fresh.actions().options, null); assert.equal(fresh.actions().title, '');
  } finally { panel.dispose(); fresh.dispose(); }
});
for (const stage of ['response', 'validation']) for (const retire of ['cancel', 'destroy']) {
  test(`actual client ${retire}/remount rejects held ${stage} without replay`, async () => {
    const wire = fixture(); wire.options.values = { title: wire.review.title, descriptionField: wire.review.descriptionField };
    let release, entered, calls = 0;
    const held = new Promise(resolve => release = resolve);
    const validationEntered = new Promise(resolve => entered = resolve);
    held.cancel = () => {};
    const service = { GetOptions: () => Promise.resolve(wire.options), GetReview: () => { calls++; return stage === 'response' ? held : Promise.resolve(wire.review); } };
    const panel = await mountPanel(service, stage === 'validation' ? async (...args) => {
      entered(); await held; return earth.twoPageGoogleEarthReviewFromWire(...args);
    } : earth.twoPageGoogleEarthReviewFromWire);
    await panel.actions().loadOptions(); panel.flush();
    const pending = panel.actions().prepare(); panel.flush();
    if (stage === 'validation') await validationEntered;
    const original = panel.actions();
    if (retire === 'cancel') original.cancel(); else panel.dispose();
    const fresh = await mountPanel(service);
    try {
      release(wire.review); await pending; panel.flush(); fresh.flush();
      assert.equal(original.current(), null);
      assert.equal(fresh.actions().review, null);
      assert.equal(calls, 1);
    } finally { fresh.dispose(); if (retire === 'cancel') panel.dispose(); }
  });
}
test('mapped source opener blocks all parent/peer/child errors and ownership, Return holds active reads', () => {
  const create = input => componentFunctions('TwoPageEntryEditor.svelte', ['openGoogleEarth', 'returnFromGoogleEarth'], {
    googleEarthReaderAvailable: true, googleEarthPath: 'owned', sourceBlocked: false, childBlocked: false,
    googleEarthOpen: false, googleEarthBusy: false, error: null, onchange() {},
    getDraftState: () => ({ unsaved: false, busy: false, blocked: false, error: null, ...input.draft }),
    sessions: new Map([[0, { view: () => input.peer ?? {} }]]), ...input,
  });
  for (const input of [{googleEarthReaderAvailable:false},{googleEarthPath:''},{sourceBlocked:true},{childBlocked:true},
    {draft:{unsaved:true}},{draft:{busy:true}},{draft:{blocked:true}},{draft:{error:'invalid'}},
    {peer:{error:'invalid'}},{peer:{referenceError:'invalid'}}]) {
    const panel=create(input); panel.actions.openGoogleEarth(); assert.equal(panel.googleEarthOpen,false); assert.match(panel.error,/Resolve/);
  }
  const panel=create({googleEarthBusy:true}); panel.actions.openGoogleEarth(); assert.equal(panel.googleEarthOpen,true);
  panel.actions.returnFromGoogleEarth(); assert.equal(panel.googleEarthOpen,true); assert.match(panel.error,/cancel/);
});
test('open saved-plot review blocks actual parent callbacks, variant/close and peer source actions', async () => {
  const clean = { unsaved: false, busy: false, blocked: false, error: null };
  const host = componentFunctions('TwoPageEntryEditor.svelte',
    ['getDraftState', 'stage', 'selectProject', 'acknowledge', 'operation', 'chooseForm', 'closeReview', 'openAudit'], {
      googleEarthOpen: true, googleEarthBusy: false, externalOpen: false, auditOpen: false, auditBusy: false,
      sourceRefreshing: false, childBlocked: false, sourceBlocked: false, error: null, close: clean,
      view: { original: {}, referenceError: null }, auditReaderAvailable: true,
      sessions: new Map([[0, { closeState: () => clean, view: () => ({}) }]]), onchange() {},
      onclosed: () => { throw new Error('No source close permitted'); },
    });
  assert.equal(host.actions.getDraftState().unsaved, true);
  assert.equal(host.actions.getDraftState().blocked, true);
  host.actions.stage('Latitude', { kind: 'text', raw: '54' }); assert.match(host.error, /no field was staged/);
  host.actions.selectProject('1'); assert.match(host.error, /no ProjectID was staged/);
  host.actions.acknowledge('Latitude'); assert.match(host.error, /no literal draft was acknowledged/);
  await host.actions.operation('save'); assert.match(host.error, /saved-plot KML review/);
  await host.actions.chooseForm('FS882-8x6XL-CHARS'); assert.match(host.error, /before changing source variant/);
  host.actions.closeReview(); assert.match(host.error, /before closing/);
  host.error = null; host.actions.openAudit(); assert.equal(host.auditOpen, false); assert.match(host.error, /before Audit/);
});
