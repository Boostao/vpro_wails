const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');

const definitionPath = path.resolve(__dirname, '..', '..', 'resources', 'fs882-xl-layout.json');
const source = readFileSync(path.join(__dirname, 'paperLayout.ts'), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, esModuleInterop: true }
}).outputText;

function load(mutate = () => {}) {
  const definition = JSON.parse(readFileSync(definitionPath, 'utf8'));
  const form = definition.forms.find(item => item.name === definition.root);
  mutate(form);
  const exports = {};
  vm.runInNewContext(compiled, {
    exports,
    require: name => {
      if (name === '../../resources/fs882-extended-shrub-layout.json') {
        return JSON.parse(readFileSync(path.join(__dirname, '..', '..', 'resources', 'fs882-extended-shrub-layout.json'), 'utf8'));
      }
      assert.equal(name, '../../resources/fs882-xl-layout.json');
      return definition;
    }
  });
  return exports;
}

test('Site retains source bounds, attached labels and omitted built-in extents', () => {
  const page = load().paperPage('Site');
  assert.equal(page.width, 951);
  assert.equal(page.height, 564);
  const find = name => page.controls.find(control => control.controlName === name);
  const plot = find('PlotNumber');
  assert.equal(plot.x, 825);
  assert.equal(plot.y, 18);
  assert.equal(plot.width, 125.8);
  assert.equal(find('Text35').x, 825);
  assert.equal(find('Text35').y, 0);
  assert.equal(find('UTMEasting').width, 96);
  assert.equal(find('UTMEasting').fontName, 'Arial');
  assert.equal(find('UTMEasting').textAlign, 'center');
  assert.equal(find('UTMNorthing').width, 96);
  assert.equal(find('SiteNotes').x, 9);
  assert.equal(find('SiteNotes').y, 468);
  assert.equal(find('SiteNotes').width, 654);
  assert.equal(find('OfficeNotes').y, 522);
});

test('Region and ecosection retain their exported source geometry and independent stored bindings', () => {
  const page = load().paperPage('Site');
  for (const [name, x, y, width] of [['FSRegionDistrict', (270 - 135) / 15, (2610 - 450) / 15, 138],
    ['Ecosection', (8730 - 135) / 15, (3150 - 450) / 15, 1360 / 15]]) {
    const control = page.controls.find(item => item.controlName === name);
    assert.ok(control);
    assert.equal(control.column, name);
    assert.equal(control.type, 'ComboBox');
    assert.equal(control.x, x);
    assert.equal(control.y, y);
    assert.equal(control.width, width);
    assert.equal(control.height, 18);
    assert.equal(control.textAlign, 'center');
    assert.equal(control.fontName, 'Arial');
  }
});

test('Soil classification retains independent source bindings, geometry and tab indices', () => {
  const page = load().paperPage('Soil/Terrain');
  for (const [name, left, tabOrder] of [['SoilClassGroup', 3600, 17], ['SoilClassSubGroup', 1710, 16]]) {
    const control = page.controls.find(item => item.controlName === name);
    assert.ok(control);
    assert.equal(control.column, name);
    assert.equal(control.type, 'ComboBox');
    assert.equal(control.x, (left - 135) / 15);
    assert.equal(control.y, 1260 / 15);
    assert.equal(control.width, 805 / 15);
    assert.equal(control.height, 18);
    assert.equal(control.tabOrder, tabOrder);
    assert.equal(control.textAlign, 'center');
    assert.equal(control.fontName, 'Arial');
    assert.ok(control.pageId && control.parentId);
  }
});

test('Site hides source-hidden decomposed coordinates and retains containment IDs', () => {
  const page = load().paperPage('Site');
  assert.equal(page.controls.some(control => control.controlName === 'LatD'), false);
  assert.equal(new Set(page.controls.map(control => control.controlId)).size, page.controls.length);
  assert.ok(page.controls.every(control => control.pageId && control.parentId));
});

test('coordinate modes apply the exact source visibility matrix without changing metadata or geometry', () => {
  let packaged;
  const model = load(form => { packaged = form; });
  const before = JSON.stringify(packaged);
  const groups = [
    [['Latitude', 'Longitude', 'lblLatitude', 'lblLongitude'], ['dd']],
    [['lblLat', 'lblLon', 'lblLatD', 'lblLonD', 'lblLonM'], ['dm', 'dms']],
    [['LatD', 'LatM', 'LatS', 'LonD', 'LonM', 'LonS', 'lblLatM', 'lblLatS', 'lblLonS'], ['dms']],
    [['LatD2', 'LatMD', 'LonD2', 'LonMD', 'lblLatD2', 'lblLatMD', 'lblLonD2', 'lblLonMD'], ['dm']]
  ];
  for (const mode of ['dd', 'dm', 'dms']) {
    const page = model.paperPage('Site', 'initial', mode);
    for (const [names, modes] of groups) for (const name of names) {
      assert.equal(page.controls.some(control => control.controlName === name), modes.includes(mode), `${mode} ${name}`);
    }
    assert.equal(page.width, 951);
    assert.equal(page.height, 564);
    const input = page.controls.find(control => control.controlName === (mode === 'dd' ? 'Latitude' : mode === 'dm' ? 'LatD2' : 'LatD'));
    assert.equal(input.y, (3150 - 450) / 15);
    assert.equal(input.x, ((mode === 'dd' ? 4770 : 4140) - 135) / 15);
    assert.equal(page.controls.find(control => control.controlName === 'Check369').properties.OptionValue.value, '1');
  }
  assert.equal(JSON.stringify(packaged), before);
  assert.throws(() => model.paperPage('Site', 'initial', 'unknown'), /Invalid coordinate display mode/);
  const hiddenPage = load(form => {
    form.controls.find(control => control.controlName === '&Site').properties.Visible = { value: '0', line: 1 };
  });
  assert.equal(hiddenPage.paperPage('Site', 'initial', 'dm').controls.length, 0);
});

test('empty and malformed geometry fail explicitly', () => {
  for (const value of [undefined, '', 'invalid', 'Infinity']) {
    const model = load(form => {
      form.controls.find(control => control.controlName === 'PlotNumber').properties.Left.value = value;
    });
    assert.throws(() => model.paperPage('Site'), /source Left/);
  }
});

test('missing or negative cached extents fail explicitly', () => {
  const missing = load(form => {
    delete form.controls.find(control => control.controlName === 'UTMEasting').properties.LayoutCachedWidth;
  });
  assert.throws(() => missing.paperPage('Site'), /Missing source Width for UTMEasting/);
  const negative = load(form => {
    form.controls.find(control => control.controlName === 'UTMEasting').properties.LayoutCachedWidth.value = '1';
  });
  assert.throws(() => negative.paperPage('Site'), /Invalid cached Width/);
  const explicit = load(form => {
    form.controls.find(control => control.controlName === 'PlotNumber').properties.Width.value = '-1';
  });
  assert.throws(() => explicit.paperPage('Site'), /Invalid source Width/);
});

test('unknown flags and containment cycles fail explicitly', () => {
  const unknown = load(form => {
    form.controls.find(control => control.controlName === 'PlotNumber').properties.Visible = { value: 'unknown', line: 1 };
  });

  assert.throws(() => unknown.paperPage('Site'), /Unknown source Visible/);
  const cyclic = load(form => {
    const plot = form.controls.find(control => control.controlName === 'PlotNumber');
    plot.parentId = plot.controlId;
  });
  assert.throws(() => cyclic.paperPage('Site'), /Cycle in source containment/);
  const orphan = load(form => {
    form.controls.find(control => control.controlName === 'PlotNumber').parentId = 'missing';
  });
  assert.throws(() => orphan.paperPage('Site'), /Missing source parent/);
});

test('parent embedded links and repeated child rows retain source geometry', () => {
  const model = load();
  const vegetation = model.paperPage('Vegetation');
  const embedded = vegetation.controls.find(control => control.controlName === 'SubVegA');
  assert.equal(embedded.x, 9);
  assert.equal(embedded.y, 42);
  const link = model.embeddedForm(embedded.controlId);
  assert.equal(link.form, 'SubVegAXL_BC');
  assert.equal(link.masterFields.join(','), 'PlotNumber');
  assert.equal(link.childFields.join(','), 'PlotNumber');
  const child = model.paperChild(link.form);
  assert.equal(child.width, 312);
  assert.equal(child.rowHeight, 14.4);
  assert.equal(child.headerHeight, 18);
  assert.ok(child.headerControls.some(control => control.caption === 'Tree/Shrubs'));
  assert.equal(child.controls.find(control => control.column === 'Species').x, 0);
  assert.equal(child.controls.find(control => control.column === 'Cover1').x, 90);
  assert.equal(child.controls.find(control => control.column === 'Cover1').width, 30);
  assert.equal(child.controls.some(control => control.column === 'ID' || control.column === 'PlotNumber'), false);
  for (const name of ['SoilHumusXL', 'SoilMineralXL', 'SubOtherXL', 'USysVegOtherXL']) {
    assert.ok(model.paperChild(name).controls.length > 0);
  }
});

test('Veg Other resolves its stored vegetation grid with source parent-child links', () => {
  const model = load();
  const page = model.paperPage('Veg Other');
  const embedded = page.controls.filter(control => control.type === 'Subform');
  assert.equal(embedded.length, 1);
  const link = model.embeddedForm(embedded[0].controlId);
  assert.equal(link.form, 'USysVegOtherXL');
  assert.equal(link.masterFields.join(',').toLowerCase(), 'plotnumber');
  assert.equal(link.childFields.join(',').toLowerCase(), 'plotnumber');
  const columns = model.paperChild(link.form).controls.map(control => control.column).filter(Boolean);
  for (const column of ['Species', 'LL', 'AF', 'DC', 'UT', 'VI', 'PV', 'PG', 'FFA', 'Cultural1', 'Cultural2', 'Other1', 'Other2']) {
    assert.ok(columns.includes(column), column);
  }
  const header = model.paperChild(link.form);
  assert.equal(header.headerHeight, 18);
  assert.ok(header.headerControls.some(control => control.caption === 'Species'));
  assert.ok(header.headerControls.some(control => control.caption === 'LL'));
});

test('cover-height transition retains observed VBA visibility and exact geometry', () => {
  const model = load();
  const initial = model.paperPage('Vegetation');
  const height = model.paperPage('Vegetation', 'height');
  const returned = model.paperPage('Vegetation', 'cover');
  const find = (page, name) => page.controls.find(control => control.controlName === name);
  assert.equal(find(initial, 'SubVegD').x, (8370 - 135) / 15);
  assert.equal(find(height, 'SubVegA'), undefined);
  assert.equal(find(height, 'SubVegC'), undefined);
  assert.equal(model.embeddedForm(find(height, 'SubVegAht').controlId).form, 'SubVegAhtXL');
  assert.equal(model.embeddedForm(find(height, 'SubVegCht').controlId).form, 'SubVegChtXL');
  assert.equal(find(height, 'SubVegD').x, (10900 - 135) / 15);
  assert.equal(find(height, 'VegNotes').width, 950);
  assert.equal(find(height, 'lblNotes').width, 950);
  assert.equal(height.contentWidth, 959);
  assert.equal(find(height, 'btnCoverAndHeight').caption, 'Cover Only');
  assert.equal(find(returned, 'SubVegAht'), undefined);
  assert.equal(find(returned, 'SubVegCht'), undefined);
  assert.equal(find(returned, 'SubVegD').x, (8320 - 135) / 15);
  assert.equal(find(returned, 'VegNotes').width, 780);
  assert.equal(find(returned, 'btnCoverAndHeight').caption, 'Cover && Height');
  assert.equal(find(model.paperPage('Vegetation'), 'SubVegD').x, find(initial, 'SubVegD').x);
});

test('Access caption accelerators preserve escaped literal ampersands', () => {
  const model = load();
  assert.equal(model.accessCaption('&Vegetation'), 'Vegetation');
  assert.equal(model.accessCaption('Cover && Height'), 'Cover & Height');
});
