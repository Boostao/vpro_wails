const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {proxy} = require('svelte/internal/client');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names = loadTypeScript('reportUnitNames.ts', {'./projectMetadataRestore':restoration});
const environment = loadTypeScript('longEnvironmentReport.ts', {'./projectMetadataRestore':restoration,'./reportUnitNames':names});
const ordinary = loadTypeScript('longVegetationReport.ts', {
  './projectMetadataRestore':restoration, './longEnvironmentReport':environment, './qualityEditor':quality,'./reportUnitNames':names
});
const project = loadTypeScript('longVegetationProjectReport.ts', {
  './longVegetationReport':ordinary, './projectMetadataRestore':restoration, './qualityEditor':quality
});
const wire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectUnitsWire.json'),'utf8'));
const owner = {contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'None',suPath:''};
const fields = value => value.report.sourceKind === 'project-selected-fields' ? ['Zone'] : [];
const validate = (value, enabled=true, context={...owner,su:value.report.su,suPath:value.suPath}) =>
  project.validateLongVegetationProjectPreview(value,context,value.report.sourceKind,fields(value),enabled);
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});

test('strict production-Go project wire validates original tuples and exact statistics on real client proxies',()=>{
  assert.equal(wire.length,3);
  for(const value of structuredClone(wire.slice(0,2))) {
    assert.throws(()=>validate(value,false),/disabled/);
    assert.equal(validate(value),value);
    assert.throws(()=>ordinary.validateLongVegetationPreview(value,'owned','Sample','C:\\project.db','None','',true),/Whole-project/);
    const reactive=proxy(structuredClone(value));
    assert.throws(()=>structuredClone(reactive));
    assert.equal(validate(reactive),reactive);
    assert.deepEqual(value.report.units.map(unit=>unit.numPlots),value.report.sourceKind==='project-selected-fields'?[2,2]:[2]);
    for(const unit of value.report.units) {
      assert.deepEqual(unit.membershipIds,[]);
      assert.equal(unit.rows[0].presence,.5);
      assert.equal(unit.rows[0].meanCover,2);
      assert.deepEqual(unit.rows[0].plots,[{plotNumber:'P1',cover:4}]);
    }
  }
});

test('original selected-SU QCLV fanout remains separate from project tuple ownership and denominator',()=>{
  const value=structuredClone(wire[2]);
  assert.equal(validate(value),value);
  assert.equal(validate(proxy(structuredClone(value))).report.units[0].numPlots,1);
  assert.equal(value.report.quality.occurrences.length,8);
  assert.equal(value.report.projectUnits.length,1);
  assert.equal(value.report.units[0].rows[0].presence,1);
  assert.deepEqual(value.report.units[0].rows[0].plots,[{plotNumber:'P1',cover:2}]);
  for(const mutate of [
    x=>x.report.quality.occurrences.pop(), x=>x.report.quality.occurrences[0].listRowIds[0]='90',
    x=>x.report.quality.occurrences[0].envRowId='99', x=>x.report.quality.occurrences[0].plotNumber=cell('P2'),
    x=>x.report.projectUnits[0].adminRowId='99', x=>x.report.units[0].numPlots=8,
    x=>x.report.units[0].membershipIds=['1'], x=>x.report.diagnostics[0].count=1,
    x=>x.settings.quality=null
  ]) {
    const changed=structuredClone(value); mutate(changed); assert.throws(()=>validate(changed));
  }
});

test('project client rejects incomplete forged scope, original fanout, ownership and shared row metadata',()=>{
  for(const mutate of [
    x=>x.contextId='stale', x=>x.projectPath='different', x=>x.suPath='invented',
    x=>x.report.su='Selected', x=>x.report.sourceKind='selected-list',
    x=>x.report.noneUnits=[], x=>x.report.selectedUnits=[], x=>x.report.projectUnits=null,
    x=>x.report.projectUnits=[], x=>x.report.projectUnits.pop(),
    x=>x.report.projectUnits.push(structuredClone(x.report.projectUnits[0])),
    x=>x.report.projectUnits[0].envRowId='01', x=>delete x.report.projectUnits[0].adminRowId,
    x=>x.report.projectUnits[0].adminRowId=null, x=>x.report.projectUnits[0].extra=true,
    x=>x.report.projectUnits[0].plotNumber=cell('p1'), x=>x.report.projectUnits[0].plotNumber=cell('\u00e9'),
    x=>x.report.projectUnits[0].siteUnit=cell('a'.repeat(256)), x=>x.report.projectUnits[0].siteUnit=cell('\ud800'),
    x=>x.report.projectUnits[0].plotNumber=cell(null),
    x=>x.report.units[0].membershipIds=['1'], x=>x.report.units[0].projectUnitIndexes=[0],
    x=>x.report.units[0].projectUnitIndexes=[0,0], x=>x.report.units[0].projectUnitIndexes=[0,9],
    x=>x.report.units[0].projectUnitIndexes=[0,2], x=>x.report.units[0].numPlots=4,
    x=>x.report.units[0].rows[0].presence=Infinity, x=>x.report.units[0].rows[0].meanCover=null,
    x=>x.report.units[0].rows[0].plots.push({plotNumber:'P1',cover:0}),
    x=>x.report.units[0].rows[0].englishName=cell('phantom'),
    x=>x.report.units[0].nameStatus='unique', x=>x.report.units[0].nameCandidates=null,
    x=>x.report.units.pop(), x=>x.report.units.push(structuredClone(x.report.units[0])),
    x=>x.settings.title='changed', x=>x.settings.unitFields=['Other'], x=>x.settings.unitGrouping='all-plots',
    x=>x.report.excludedProjectUnitIndexes=[0],
    x=>x.report.diagnostics[0].count=3, x=>x.report.diagnostics[0].code='physical_membership_multiplicity',
    x=>x.report.diagnostics.shift(),
    x=>x.report.diagnostics.push(structuredClone(x.report.diagnostics[0]))
  ]) {
    const value=structuredClone(wire[0]);
    mutate(value);
    assert.throws(()=>validate(value),undefined,JSON.stringify(value));
  }
});

test('project Env-only tuples permit source NULL and empty labels without invented Admin or SU path',()=>{
  const value=structuredClone(wire[1]);
  value.report.projectUnits[1].plotNumber=cell(null);
  value.report.diagnostics=value.report.diagnostics.filter(row=>row.code!=='project_unit_join_multiplicity');
  assert.equal(validate(value),value);
  for(const mutate of [
    x=>x.report.projectUnits[0].adminRowId='21', x=>x.report.projectUnits[0].siteUnit=cell('not All plots'),
    x=>x.settings.unitFields=[], x=>x.settings.quality={site:{minimum:'Poor',includeNull:false},veg:{minimum:'Poor',includeNull:false},soil:{minimum:'Poor',includeNull:false}}
  ]) {
    const changed=structuredClone(value); mutate(changed); assert.throws(()=>validate(changed));
  }
  assert.throws(()=>validate(value,true,{...owner,su:'Selected',suPath:'C:\\external.db'}),/context/);
  const tuples=structuredClone(wire[0].report.projectUnits);
  tuples[0].siteUnit=cell('');
  tuples[1].siteUnit=cell('');
  assert.doesNotThrow(()=>project.validateProjectUnitTuples(tuples,'project-selected-fields'));
});

test('excluded project tuple ownership needs exact diagnosed original indexes and cannot overlap emitted units',()=>{
  const value=structuredClone(wire[0]);
  value.report.projectUnits.push({envRowId:'13',adminRowId:'23',plotNumber:cell('P2'),siteUnit:cell('Orphan')});
  value.report.excludedProjectUnitIndexes=[4];
  value.report.diagnostics.push({code:'excluded_unit_without_vegetation',identity:'text:Orphan',count:1});
  assert.equal(validate(value),value);
  for(const mutate of [
    x=>delete x.report.excludedProjectUnitIndexes, x=>x.report.excludedProjectUnitIndexes=null,
    x=>x.report.excludedProjectUnitIndexes=[4,4], x=>x.report.excludedProjectUnitIndexes=[0],
    x=>x.report.diagnostics.pop(), x=>x.report.diagnostics.at(-1).count=2
  ]) {
    const changed=structuredClone(value); mutate(changed); assert.throws(()=>validate(changed));
  }
});

test('emitted project NULL and empty unit labels remain distinct on plain and reactive previews',()=>{
  const value=structuredClone(wire[0]);
  for(const [unitIndex,label] of [[0,null],[1,'']]) {
    const unit=value.report.units[unitIndex];
    unit.code=cell(label);
    for(const index of unit.projectUnitIndexes) value.report.projectUnits[index].siteUnit=cell(label);
    value.report.diagnostics[unitIndex+1].identity=label===null?'null:':'text:';
    if(label===null) {
      unit.nameStatus='unassigned';
      unit.longName='';
    }
  }
  value.report.diagnostics=value.report.diagnostics.filter(row=>row.identity!=='null:');
  for(const preview of [value,proxy(structuredClone(value))]) {
    assert.equal(validate(preview),preview);
    assert.equal(preview.report.units[0].code.storage,'null');
    assert.equal(preview.report.units[1].code.text,'');
    assert.deepEqual(preview.report.units.map(unit=>unit.numPlots),[2,2]);
  }
  const merged=structuredClone(value);
  merged.report.units[1].code=cell(null);
  assert.throws(()=>validate(merged),/denominator/);
});

test('shared project row validator retains layer/None/Strata and exact short lifeform scalar checks',()=>{
  for(const grouping of ['layer','none','strata','lifeform']) {
    const value=structuredClone(wire[1]);
    value.settings.grouping=grouping;
    if(grouping==='lifeform') {
      value.report.units[0].rows[0].layer={storage:'integer',text:null,integer:'-32768',real:null,blobHex:null};
    }
    assert.equal(validate(value),value);
    if(grouping==='lifeform') {
      value.report.units[0].rows[0].layer.integer='-32769';
      assert.throws(()=>validate(value),/grouping/);
    }
  }
});
