const assert = require('node:assert/strict');
const { test } = require('node:test');
const { readFileSync } = require('node:fs');
const path = require('node:path');

function layout(file, root, hash) {
  const value = JSON.parse(readFileSync(path.join(__dirname, '../../resources', file), 'utf8'));
  assert.equal(value.root, root);
  assert.equal(value.geometryUnit, 'twips');
  assert.match(value.layoutPolicy, /no physical print, pagination or built-in default inference/);
  assert.equal(value.forms.length, 1);
  const report = value.forms[0];
  assert.equal(report.sha256, hash);
  assert.equal(report.name, root);
  assert.equal(report.recordSource, 'BecLabels');
  assert.equal(report.controls[0].type, 'Report');
  assert.equal(report.pages.length, 0);
  assert.equal(report.embedded.length, 0);
  for (const control of report.controls) {
    assert.equal(control.events?.length ?? 0, 0);
    for (const opaque of ['PrtMip', 'PrtDevMode', 'PrtDevNames', 'GUID', 'NameMap']) {
      assert.equal(Object.hasOwn(control.properties, opaque), false);
    }
  }
  return report;
}

function property(control, name) {
  const fact = control.properties[name];
  assert.ok(fact, `${control.controlName}: missing explicit/inherited ${name}`);
  assert.ok(Number.isInteger(fact.line) && fact.line > 0);
  return fact.value;
}

function bound(report, expression) {
  const controls = report.fields.filter(field => field.binding === expression);
  assert.equal(controls.length, 1, `exactly one source binding: ${expression}`);
  assert.equal(controls[0].implementation, 'unmapped');
  assert.equal(controls[0].readOnly, true);
  return controls[0];
}

function slot(report, number, top) {
  const plot = bound(report, `=Trim([PlotNumber${number}])`);
  const representing = bound(report, `=Trim([Representing${number}])`);
  const zone = bound(report, `Zone${number}`);
  const project = bound(report, `ProjectID${number}`);
  for (const [control, left, offset] of [
    [plot, 72, 0], [representing, 72, 240], [zone, 2448, 0], [project, 1296, 0],
  ]) {
    assert.equal(property(control, 'Left'), String(left));
    assert.equal(property(control, 'Top'), String(top + offset));
    assert.equal(property(control, 'FontName'), 'Arial');
    assert.equal(control.properties.FontName.inherited, true);
  }
  assert.equal(property(plot, 'FontSize'), '9');
  assert.equal(property(plot, 'FontWeight'), '700');
  assert.equal(property(representing, 'Height'), '360');
  assert.equal(property(representing, 'FontSize'), '6');
  assert.equal(property(zone, 'TextAlign'), '3');
  assert.equal(property(project, 'FontWeight'), '700');
  assert.equal(Object.hasOwn(plot.properties, 'Height'), false);
  assert.equal(Object.hasOwn(project.properties, 'Height'), false);
  assert.equal(Object.hasOwn(project.properties, 'FontSize'), false);
}

test('native omitted-default overlay resolves every control without overwriting exported provenance', () => {
    const measured = JSON.parse(readFileSync(path.join(__dirname, '../../resources/plot-label-report-measured-defaults.json'), 'utf8'));
    assert.equal(measured.geometryUnit, 'twips');
    assert.equal(measured.exportedFactsPreservedSeparately, true);
    assert.equal(measured.noPrintPreviewOrDataWrites, true);
    assert.equal(measured.matchedExportedProperties, 546);
    assert.equal(measured.measuredOmittedProperties, 78);
    assert.equal(measured.reports.length, 2);
    let resolved = 0, controls = 0;
    for (const [file, name] of [['plot-label-report-layout.json', 'USysBecLabels'], ['plot-label-all-report-layout.json', 'USysBecLabelsAll']]) {
      const source = JSON.parse(readFileSync(path.join(__dirname, '../../resources', file), 'utf8')).forms[0];
      const overlay = measured.reports.filter(report => report.name === name);
      assert.equal(overlay.length, 1);
      assert.equal(overlay[0].sourceSHA256, source.sha256);
      const fields = source.fields.filter(field => ['TextBox', 'Label'].includes(field.type));
      assert.equal(overlay[0].controls.length, fields.length);
      for (const field of fields) {
        const matches = overlay[0].controls.filter(control => control.name === field.controlName);
        assert.equal(matches.length, 1);
        const defaults = matches[0].properties;
        for (const key of Object.keys(defaults)) {
          assert.equal(Object.hasOwn(field.properties, key), false, `${name}/${field.controlName}.${key} must retain exported provenance`);
          assert.ok(['Height', 'FontSize', 'FontWeight', 'TextAlign'].includes(key));
          resolved++;
        }
        const effective = key => field.properties[key]?.value ?? String(defaults[key]);
        assert.equal(effective('FontName'), 'Arial');
        if (/^ProjectID\d*$/.test(field.binding ?? '')) {
          assert.equal(effective('FontSize'), '8');
          assert.equal(effective('Height'), '240');
          assert.equal(effective('TextAlign'), '0');
        } else if (/^=Trim\(\[PlotNumber\d+\]\)$/.test(field.binding ?? '') || /^Zone\d+$/.test(field.binding ?? '')) {
          assert.equal(effective('Height'), '240');
        }
        controls++;
      }
    }
    assert.equal(controls, 78);
    assert.equal(resolved, 78);
});

test('single report retains six source controls, bindings, dimensions and omitted defaults', () => {
  const report = layout('plot-label-report-layout.json', 'USysBecLabels',
    'b5779398241ed783a9a3c5011861cb695605c05d5201e21ec2af009c7113d94f');
  assert.equal(property(report.controls[0], 'Width'), '4954');
  assert.equal(report.fields.filter(field => ['TextBox', 'Label'].includes(field.type)).length, 6);
  assert.equal(property(report.fields.find(field => field.controlName === 'Detail'), 'Height'), '7260');
  slot(report, 1, 60);
  const date = bound(report, '="Label Date: " & Format(Now(),"Long Date")');
  assert.equal(property(date, 'Left'), '507');
  assert.equal(property(date, 'Top'), '660');
  assert.equal(property(date, 'Height'), '180');
  const marker = report.fields.find(field => field.controlName === 'RK');
  assert.equal(property(marker, 'Caption'), '(VPro)');
  assert.equal(property(marker, 'Left'), '75');
  assert.equal(property(marker, 'Top'), '660');
});

test('all report retains every slot and the nonuniform gap instead of a guessed uniform grid', () => {
  const report = layout('plot-label-all-report-layout.json', 'USysBecLabelsAll',
    'ecbc1ffbe01e0cd2d66836c4301ed1e173de7e468cd4743d27ef9b4ac038f1c2');
  assert.equal(property(report.controls[0], 'Width'), '4833');
  assert.equal(property(report.fields.find(field => field.controlName === 'Detail'), 'Height'), '14760');
  assert.equal(report.fields.filter(field => ['TextBox', 'Label'].includes(field.type)).length, 72);
  const tops = [60, 1200, 2340, 3480, 4620, 5760, 8280, 9420, 10560, 11700, 12840, 13980];
  tops.forEach((top, index) => slot(report, index + 1, top));
  const dates = report.fields.filter(field => field.binding === '="Label Date: " & Format(Now(),"Long Date")');
  const markers = report.fields.filter(field => field.type === 'Label');
  assert.equal(dates.length, 12);
  assert.equal(markers.length, 12);
  tops.forEach((top, index) => {
    assert.equal(property(dates[index], 'Top'), String(top + 600));
    assert.equal(property(dates[index], 'Left'), '507');
    assert.equal(property(markers[index], 'Top'), String(top + 600));
    assert.equal(property(markers[index], 'Caption'), '(VPro)');
  });
  assert.equal(tops[6] - tops[5], 2520);
});
