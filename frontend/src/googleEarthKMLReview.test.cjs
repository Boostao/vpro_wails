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
let parsedDocument;
const kml = loadTypeScript('googleEarthKMLReview.ts', { './qualityEditor': quality, './googleEarthReview': earth }, {
  TextEncoder, setTimeout, DOMParser: class {
    parseFromString(xml, type) { assert.equal(type, 'application/xml'); return parsedDocument; }
  },
});
const scope = { contextId: 'owned', project: 'Sample', projectPath: 'C:\\fixture\\Sample.db', su: 'None', suPath: '' };
const header = '<?xml version="1.0" encoding="UTF-8"?>\n';
function value() {
  const xml = header + '<kml xmlns="http://earth.google.com/kml/2.1"><Document><name> Title </name></Document></kml>';
  return { ...scope, descriptionField: 'Zone', title: ' Title ', placemarkCount: 0,
    byteCount: new TextEncoder().encode(xml).length, kml: xml };
}
function validate(input, inspect = () => true) {
  return kml.validateGoogleEarthKMLReview(input, scope, 'Zone', ' Title ', inspect);
}

test('KML preview validates detached exact owned metadata and UTF8 byte count without title repair', async () => {
  const input = value();
  let inspected = false;
  const result = await validate(input, (xml, title, count) => {
    inspected = true; input.kml = 'caller'; input.title = 'caller';
    assert.equal(title, ' Title '); assert.equal(count, 0); assert.ok(xml.startsWith(header)); return true;
  });
  assert.ok(inspected); assert.equal(result.title, ' Title '); assert.notEqual(result.kml, input.kml);
  for (const mutate of [
    v => v.contextId = 'foreign', v => v.projectPath = 'elsewhere', v => v.suPath = 'elsewhere',
    v => v.descriptionField = 'zone', v => v.title = 'Title', v => v.placemarkCount = -1,
    v => v.placemarkCount = 0.5, v => v.byteCount++, v => v.kml = '\ud800',
    v => v.kml += '<!DOCTYPE kml>', v => v.kml += '<?xml-stylesheet href="https://example.invalid"/>',
  ]) { const input = value(); mutate(input); await assert.rejects(validate(input)); }
  await assert.rejects(validate(value(), () => false));
  const current = value();
  let yields = 0;
  const detached = await kml.validateGoogleEarthKMLReview(current, scope, 'Zone', ' Title ', () => true, async () => {
    yields++; current.kml = 'mutated during yield'; current.title = 'changed';
  });
  assert.equal(yields, 1); assert.equal(detached.title, ' Title '); assert.ok(detached.kml.startsWith(header));
});

test('KML title Unicode XML rules preserve whitespace and reject invalid controls without defaults', () => {
  for (const valid of ['', ' \t\r\n<&\u{1f332} ', '\ufffd', '\u{10000}', '\u{10ffff}']) {
    assert.equal(kml.googleEarthXMLText(valid), true);
  }
  for (const invalid of ['\0', '\x01', '\x0b', '\x1f', '\ufffe', '\uffff', '\ud800', '\udfff']) {
    assert.equal(kml.googleEarthXMLText(invalid), false);
  }
});

function dom() {
  const ns = 'http://earth.google.com/kml/2.1';
  const element = (name, text = '', children = []) => ({ localName: name, textContent: text, children,
    namespaceURI: ns, attributes: [], getAttribute: () => null });
  const mark = element('Placemark', '', [element('name', 'name'), element('description', '&lt;img&gt;.'),
    element('Point', '', [element('coordinates', '-0,90,0')])]);
  const body = element('Document', '', [element('name', ' Title '), mark]);
  const root = element('kml', '', [body]); root.attributes = [{}]; root.getAttribute = () => ns;
  const all = node => [node, ...node.children.flatMap(all)];
  return { root, body, mark, documentElement: root, doctype: null,
    getElementsByTagName: name => name === '*' ? all(root) : [] };
}

test('KML browser DOM shape inspector rejects foreign namespaces/resources/hierarchy and invalid coordinates', async () => {
  try {
    for (const mutate of [() => {}, d => d.root.namespaceURI = 'foreign',
      d => d.doctype = {}, d => d.body.children[0].textContent = 'changed',
      d => d.mark.children[1].textContent = '<img>',
      d => d.mark.children.push({ ...d.mark.children[0], localName: 'NetworkLink' }),
      d => d.mark.children[2].children[0].textContent = '181,90,0',
      d => d.mark.children[2].children[0].textContent = '-0,91,0',
      d => d.mark.children[2].children[0].textContent = '-0,90,1']) {
      const document = dom(); mutate(document);
      parsedDocument = document;
      const input = value(); input.placemarkCount = 1;
      const action = () => kml.validateGoogleEarthKMLReview(input, scope, 'Zone', ' Title ');
      if (document.root.namespaceURI === 'http://earth.google.com/kml/2.1' && !document.doctype &&
          document.body.children[0].textContent === ' Title ' && document.mark.children.length === 3 &&
          document.mark.children[1].textContent === '&lt;img&gt;.' &&
          document.mark.children[2].children[0].textContent === '-0,90,0') await assert.doesNotReject(action);
      else await assert.rejects(action);
    }
  } finally { parsedDocument = undefined; }
});

test('KML panel remains independently gated and renders XML only in a labelled read-only textarea', () => {
  const source = readFileSync(path.join(__dirname, 'GoogleEarthReview.svelte'), 'utf8');
  const compiled = compile(source, { filename: 'GoogleEarthReview.svelte', generate: 'client' });
  assert.ok(compiled.js.code);
  assert.match(source, /VITE_GOOGLE_EARTH_KML_PREVIEW === 'true'/);
  assert.match(source, /GetGoogleEarthKMLReview/);
  assert.match(source, /<label for="google-earth-place-name">Place Name:/);
  assert.match(source, /<textarea id="google-earth-kml-text" readonly/);
  assert.doesNotMatch(source, /\{@html/);
  assert.match(source, /onDestroy\(cancel\)/);
  assert.match(source, /No file created, preference saved or viewer launched/);
});
