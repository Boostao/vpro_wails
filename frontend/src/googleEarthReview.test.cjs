const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { compile } = require('svelte/compiler');
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const cells = loadTypeScript('projectMetadataRestore.ts', { './qualityEditor': quality, './projectMetadataEditor': {} });
const locations = loadTypeScript('plotLocationReview.ts', { './projectMetadataRestore': cells });
const earth = loadTypeScript('googleEarthReview.ts', {
  './projectMetadataRestore': cells, './qualityEditor': quality, './plotLocationReview': locations,
});
const scope = { contextId: 'owner', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', su: 'Unit', suPath: 'C:\\fixture\\Other.db' };
const cell = (storage, value) => ({ storage, text: null, integer: null, real: null, blobHex: null,
  ...(storage === 'null' ? {} : { [storage === 'blob' ? 'blobHex' : storage]: value }) });
function choices() {
  return { ...scope, envTable: 'Sample_Env', fields: ['PlotNumber', 'Longitude', 'Latitude', 'Custom " field ']
    .map(name => ({ name, declaredType: name === 'Longitude' ? 'REAL' : '' })) };
}
function document() {
  const row = { envRowId: '-9007199254740993', membershipRowId: '9223372036854775807',
    plotNumber: cell('text', '001'), storedLongitude: cell('real', 123.25), longitude: cell('real', -123.25),
    latitude: cell('real', 190), description: cell('null') };
  return { ...scope, descriptionField: 'Custom " field ', offset: 0, limit: 50, totalRows: 2,
    fields: ['PlotNumber', 'Longitude', 'Latitude', 'Custom " field '].map((key, i) => ({ source: 'Env', key, label: i === 0 ? 'Plot Number' : key, heading: false })),
    rows: [row, { ...structuredClone(row), membershipRowId: '-2' }] };
}
const validate = value => earth.validateGoogleEarthReview(value, scope, 'Custom " field ', 0, 50, async () => {});

test('Earth description choices are exact dynamic physical schema, not Sample hardcoding or case repair', () => {
  const value = choices(), result = earth.validateGoogleEarthFields(value, scope);
  assert.equal(JSON.stringify(result), JSON.stringify(value));
  result.fields[3].name = 'caller';
  assert.equal(value.fields[3].name, 'Custom " field ');
  for (const mutate of [v => v.contextId = 'stale', v => v.envTable = 'Other_Env', v => v.suPath = 'elsewhere',
    v => v.fields[0].name = 'plotnumber', v => v.fields.push(v.fields[0]), v => v.fields = null,
    v => v.fields[3].name = '\ud800', v => v.fields[3].declaredType = '\udfff']) {
    const bad = choices(); mutate(bad); assert.throws(() => earth.validateGoogleEarthFields(bad, scope));
  }
});

test('Earth review preserves duplicate SU fanout, nullable/non-text descriptions, exact IDs and historical ranges', async () => {
  for (const description of [cell('null'), cell('text', ''), cell('text', '  \u{1f332}\n '),
    cell('integer', '-9007199254740993'), cell('blob', '')]) {
    const value = document(); value.rows.forEach(row => row.description = structuredClone(description));
    const result = await validate(value);
    assert.equal(JSON.stringify(result), JSON.stringify(value));
    result.rows[0].description.text = 'caller';
    assert.notEqual(value.rows[0].description.text, 'caller');
  }
  const none = document(); none.su = 'None'; none.suPath = ''; none.totalRows = 1;
  none.rows = [none.rows[0]]; none.rows[0].membershipRowId = ''; none.rows[0].plotNumber = cell('null');
  assert.equal((await earth.validateGoogleEarthReview(none, { ...scope, su: 'None', suPath: '' }, none.descriptionField, 0, 50)).rows.length, 1);
});

test('Earth transport rejects foreign scopes, malformed pages/cells, repeated pairs and changed duplicate Env values', async () => {
  for (const mutate of [
    v => v.contextId = 'stale', v => v.project = 'Other', v => v.projectPath = 'elsewhere', v => v.suPath = 'elsewhere',
    v => v.descriptionField = 'Custom " FIELD ', v => v.offset = 1, v => v.limit = 49,
    v => v.totalRows = 3, v => v.totalRows = -1, v => v.fields.reverse(), v => v.fields[0].source = 'Admin',
    v => v.fields[0].heading = true, v => v.fields[3].label = 'Changed', v => v.rows = null, v => v.rows[0].envRowId = '+1',
    v => v.rows[0].membershipRowId = '', v => v.rows[1].membershipRowId = v.rows[0].membershipRowId,
    v => v.rows[0].description = cell('text', '\ud800'), v => v.rows[0].latitude = cell('null'),
    v => v.rows[0].latitude = cell('text', 'historical raw latitude'),
    v => v.rows[0].longitude = cell('real', 123.25), v => v.rows[1].description = cell('text', ''),
    v => v.rows[0].storedLongitude = cell('integer', '-9223372036854775808'),
  ]) {
    const value = document(); mutate(value); await assert.rejects(validate(value));
  }
  const zeros = document();
  zeros.rows[0].storedLongitude = cell('real', 0); zeros.rows[0].longitude = cell('real', -0);
  zeros.rows[1].storedLongitude = cell('real', -0); zeros.rows[1].longitude = cell('real', 0);
  await assert.rejects(validate(zeros));
});

test('Earth selected coordinate/name aliases keep the exact raw description value, not negated longitude', async () => {
  for (const [field, key] of [['PlotNumber', 'plotNumber'], ['Longitude', 'storedLongitude'], ['Latitude', 'latitude']]) {
    const value = document(); value.descriptionField = field; value.fields[3].key = field; value.fields[3].label = field;
    value.rows.forEach(row => row.description = structuredClone(row[key]));
    await earth.validateGoogleEarthReview(value, scope, field, 0, 50);
    value.rows[0].description = cell('text', 'changed');
    await assert.rejects(earth.validateGoogleEarthReview(value, scope, field, 0, 50));
  }
});

test('Earth page validation detaches before yielding and accepts exact empty beyond-end pages', async () => {
  const value = document(), row = value.rows[0];
  value.rows = Array.from({ length: 101 }, (_, i) => ({ ...structuredClone(row), membershipRowId: String(i) }));
  value.limit = 101; value.totalRows = 101;
  const expected = structuredClone(value); let yields = 0;
  const result = await earth.validateGoogleEarthReview(value, scope, value.descriptionField, 0, 101, async () => {
    yields++; value.contextId = 'caller'; value.rows[100].description = cell('text', 'caller');
  });
  assert.equal(yields, 1); assert.equal(JSON.stringify(result), JSON.stringify(expected));
  const empty = document(); empty.offset = 200; empty.rows = [];
  assert.equal((await earth.validateGoogleEarthReview(empty, scope, empty.descriptionField, 200, 50)).rows.length, 0);
});

test('Earth UI independently gates only preparation, shares context/close barriers and cancels remounted reads', () => {
  const component = readFileSync(path.join(__dirname, 'GoogleEarthReview.svelte'), 'utf8');
  const app = readFileSync(path.join(__dirname, 'App.svelte'), 'utf8');
  const navigation = readFileSync(path.join(__dirname, 'Navigation.svelte'), 'utf8');
  assert.equal(compile(component, { filename: 'GoogleEarthReview.svelte', generate: 'client' }).warnings.length, 0);
  assert.match(navigation, /VITE_GOOGLE_EARTH_REVIEW === 'true' \? 'google-earth-review' : undefined/);
  assert.match(navigation, /name: 'Show Plot Locations in Google Earth', icon: Globe \}/);
  assert.match(app, /view === 'google-earth-review' && import\.meta\.env\.VITE_GOOGLE_EARTH_REVIEW === 'true'/);
  assert.match(app, /function navigate[\s\S]*?requestTransition/);
  assert.match(component, /onDestroy\(cancel\)/); assert.match(component, /reads\.cancelAll\(\)/);
  assert.match(component, /label for="google-earth-description"/);
  assert.match(component, /const validated = await validateGoogleEarthReview[\s\S]*?if \(request !== generation\) return;[\s\S]*?review = validated/);
  assert.match(component, /No data, audits, preferences or files written/);
  assert.doesNotMatch(component, /download=|SaveProject|CreateBlank|BuildFile/);
});
