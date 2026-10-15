const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {render} = require('svelte/server');
const {proxy} = require('svelte/internal/client');
const {loadTypeScript,serverComponent} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json':JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':editor});
const names = loadTypeScript('reportUnitNames.ts', {'./projectMetadataRestore':restoration});
const environment = loadTypeScript('longEnvironmentReport.ts', {'./projectMetadataRestore':restoration,'./reportUnitNames':names});
const report = loadTypeScript('longVegetationReport.ts', {
  './projectMetadataRestore':restoration, './longEnvironmentReport':environment, './qualityEditor':quality,'./reportUnitNames':names
});
const workbook = loadTypeScript('vegetationWorkbook.ts', {
  './longVegetationReport':report, './projectMetadataRestore':restoration,
  './lifeformSummary':loadTypeScript('lifeformSummary.ts'), './publicationSession':loadTypeScript('publicationSession.ts')
});
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});
function settings() {
  return {title:'  Title \u00e9  ',grouping:'layer',average:'all-plots',constantSpeciesList:true,
    presenceGreaterThan:95,meanCoverGreaterThan:99,order:'species',showEnglishName:true,showSpeciesCode:false,quality:null};
}
function fixture() {
  return {contextId:'owned',projectPath:'C:\\project.db',suPath:'C:\\external.db',settings:settings(),report:{
    project:'Sample',su:'Selected',title:settings().title,quality:null,diagnostics:[
      {code:'constant_list_name_fanout',identity:'text:Tree/text:SP',count:2}
    ],units:[{code:cell(''),longName:null,nameStatus:'missing',nameCandidates:[],numPlots:2,membershipIds:['1','2'],rows:[
      {layer:cell('Tree'),species:cell('SP'),englishName:cell('List name'),matchedName:cell('A'),
        presence:0.5,meanCover:0,plots:[{plotNumber:'',cover:0},{plotNumber:'P1',cover:null}]},
      {layer:cell('Tree'),species:cell('SP'),englishName:cell('List name'),matchedName:cell('B'),
        presence:0.5,meanCover:-1,plots:[{plotNumber:'',cover:-2},{plotNumber:'P1',cover:null}]},
      {layer:cell(null),species:cell(''),englishName:cell(null),matchedName:cell(null),
        presence:null,meanCover:null,plots:[{plotNumber:'',cover:null},{plotNumber:'P1',cover:null}]}
    ]}]
  }};
}
function qualifiedFixture() {
  const value=fixture(), criterion={minimum:'  Poor  ',includeNull:true};
  value.settings.quality={site:{...criterion},veg:{...criterion},soil:{...criterion}};
  const rank=n=>n===null?cell(null):{storage:'integer',integer:String(n),text:null,real:null,blobHex:null};
  const reference=(id,item,order,domain='DataQuality')=>({rowId:id,item:cell(item),listName:cell(domain),itemOrder:rank(order)});
  value.report.quality={thresholdRowIds:[['90'],['90'],['90']],excludedMembershipIds:['9'],references:[
    reference('90','  Poor  ',1),reference('100','Site',3),reference('101','Veg',3),
    reference('102','Soil',3),reference('103','Soil',null,null),
    reference('201','OtherV',4),reference('202','OtherS',4),reference('400','NA',1,'PlotQualitySite')
  ],occurrences:[
    {membershipId:'1',plotNumber:cell(''),siteUnit:cell(''),envRowId:'11',adminRowId:'21',listRowIds:['100','101','102'],values:[cell('Site'),cell('Veg'),cell('Soil')]},
    {membershipId:'1',plotNumber:cell(''),siteUnit:cell(''),envRowId:'11',adminRowId:'21',listRowIds:['100','101','103'],values:[cell('Site'),cell('Veg'),cell('Soil')]},
    {membershipId:'2',plotNumber:cell('P1'),siteUnit:cell(''),envRowId:'12',adminRowId:'22',listRowIds:[null,'201','202'],values:[cell(null),cell('OtherV'),cell('OtherS')]}
  ]};
  value.report.units[0].numPlots=3;
  value.report.units[0].rows[0].presence=1/3;
  value.report.units[0].rows[1].presence=1/3;
  value.report.units[0].rows[1].meanCover=-2/3;
  value.report.diagnostics.push({code:'quality_reference_multiplicity',identity:'1',count:2});
  return value;
}
const validate = value => report.validateLongVegetationPreview(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db');
const validateOptions = value => report.validateLongVegetationOptions(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db');
const scalarWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationSelectedScalarWire.json'),'utf8'));
const selectedUnitWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationSelectedUnitsWire.json'),'utf8'));
const validateSelectedUnits = value => report.validateLongVegetationPreview(value,'owned','Sample','C:\\project.db','Selected','C:\\external.db',true);
test('ordinary selected-list validator refuses all distinct project wire on actual reactive proxies',()=>{
  const projectWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationProjectUnitsWire.json'),'utf8'));
  assert.equal(projectWire.length,3);
  for(const value of projectWire) {
    assert.throws(()=>report.validateLongVegetationPreview(value,'owned','Sample','C:\\project.db','None','',true),/Whole-project/);
    const reactive=proxy(structuredClone(value));
    assert.throws(()=>structuredClone(reactive));
    assert.throws(()=>report.validateLongVegetationPreview(reactive,'owned','Sample','C:\\project.db','None','',true),/Whole-project/);
  }
  for(const mutate of [
    value=>value.report.sourceKind='project-selected-fields',
    value=>value.report.projectUnits=[],
    value=>value.report.excludedProjectUnitIndexes=[],
    value=>value.report.units[0].projectUnitIndexes=[]
  ]) {
    const value=fixture();
    mutate(value);
    assert.throws(()=>validate(value),/Whole-project/);
  }
});
test('strict-Go selected units retain complete joins and unique original IDs across derived labels on actual proxies',()=>{
  const value=structuredClone(selectedUnitWire);
  assert.throws(()=>validate(value),/unavailable/);
  assert.equal(validateSelectedUnits(value),value);
  const reactive=proxy(structuredClone(value));
  assert.throws(()=>structuredClone(reactive));
  assert.equal(validateSelectedUnits(reactive),reactive);
  assert.deepEqual(value.report.units.map(unit=>unit.numPlots),[4,4]);
  assert.deepEqual(value.report.units.map(unit=>unit.membershipIds),[['1','2'],['1','2']]);
  assert.throws(()=>report.requireLongVegetationGrouping(value.settings,true,true,true,true,true),/unavailable/);
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(value.settings,false,false,false,false,false,true));
  const options={contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'Selected',suPath:'C:\\external.db',settings:value.settings};
  assert.equal(report.validateLongVegetationOptions(options,'owned','Sample','C:\\project.db','Selected','C:\\external.db',true),options);
  for(const mutate of [
    x=>delete x.settings.unitFields, x=>x.settings.unitFields=null, x=>x.settings.unitFields=[],
    x=>x.settings.unitFields=['Zone','zone'], x=>x.settings.unitFields=['Zone','bad-field'],
    x=>x.settings.unitFields=['\ud800'], x=>x.settings.unitGrouping='all-plots',
    x=>delete x.settings.unitGrouping, x=>delete x.report.selectedUnits, x=>x.report.selectedUnits=null,
    x=>x.report.selectedUnits=[], x=>x.report.noneUnits=[],
    x=>x.report.selectedUnits.pop(), x=>x.report.selectedUnits.push(structuredClone(x.report.selectedUnits[0])),
    x=>x.report.selectedUnits[0].envRowId=null, x=>x.report.selectedUnits[0].adminRowId=null,
    x=>x.report.selectedUnits[0].membershipId='01', x=>x.report.selectedUnits[0].envRowId='011',
    x=>delete x.report.selectedUnits[0].adminRowId, x=>x.report.selectedUnits[0].extra=true,
    x=>x.report.selectedUnits[0].plotNumber=cell('P2'), x=>x.report.selectedUnits[0].plotNumber=cell('p1'),
    x=>x.report.selectedUnits[0].siteUnit=cell('changed'),
    x=>x.report.selectedUnits[0].plotNumber=cell(''), x=>x.report.selectedUnits[0].siteUnit=cell(''),
    x=>x.report.selectedUnits[0].plotNumber=cell('12345678'), x=>x.report.selectedUnits[0].siteUnit=cell('a'.repeat(256)),
    x=>x.report.selectedUnits[0].siteUnit=cell('\ud800'), x=>x.report.selectedUnits[0].siteUnit=cell('a\0b'),
    x=>x.report.selectedUnits[0].plotNumber=cell('\u00e9'), x=>x.report.selectedUnits[0].plotNumber=cell(null),
    x=>x.report.units[0].membershipIds.push('1'), x=>x.report.units[0].membershipIds=['1'],
    x=>x.report.units[0].membershipIds=['1','9'], x=>x.report.units[0].numPlots=2,
    x=>x.report.units.pop(), x=>x.report.diagnostics=x.report.diagnostics.filter(d=>d.identity!=='1'),
    x=>x.report.diagnostics.find(d=>d.code==='selected_unit_join_multiplicity').count=3,
    x=>x.report.diagnostics.push(structuredClone(x.report.diagnostics.find(d=>d.code==='selected_unit_join_multiplicity'))),
    x=>{x.report.selectedUnits[0].envRowId=null;x.report.selectedUnits[0].adminRowId=null;x.report.selectedUnits[0].siteUnit=cell(null);}
  ]) {
    const invalid=structuredClone(value);mutate(invalid);
    assert.throws(()=>validateSelectedUnits(invalid),undefined,String(mutate));
    assert.throws(()=>validateSelectedUnits(proxy(structuredClone(invalid))),undefined,String(mutate));
  }
});
test('selected-unit nullable orphan witnesses preserve original physical identity and destination bounds',()=>{
  for(const plot of [null,'P2']) {
    const value=structuredClone(selectedUnitWire);
    value.report.selectedUnits.push({membershipId:'3',envRowId:null,adminRowId:null,plotNumber:cell(plot),siteUnit:cell(null)});
    if(plot===null) value.report.diagnostics.push({code:'excluded_unit_without_vegetation',identity:'null:',count:1});
    else value.report.units.push({code:cell(null),longName:'',nameStatus:'unassigned',nameCandidates:[],
      numPlots:1,membershipIds:['3'],rows:[]});
    assert.equal(validateSelectedUnits(value),value);
    for(const mutate of [
      x=>x.report.selectedUnits.at(-1).siteUnit=cell('invented'),
      x=>x.report.selectedUnits.at(-1).envRowId='11',
      x=>x.report.selectedUnits.push({...x.report.selectedUnits.at(-1)}),
    ]) { const invalid=structuredClone(value);mutate(invalid);assert.throws(()=>validateSelectedUnits(invalid)); }
  }
  const bounded=structuredClone(selectedUnitWire);
  for(const row of bounded.report.selectedUnits) if(row.siteUnit.text==='Alpha') row.siteUnit=cell('\ud83d\ude00'.repeat(127)+'a');
  bounded.report.units[0].code=cell('\ud83d\ude00'.repeat(127)+'a');
  assert.equal(validateSelectedUnits(bounded),bounded);
});
test('selected units use qualified plot existence without weighting original reference fanout',()=>{
  const value=structuredClone(selectedUnitWire), source=qualifiedFixture();
  value.settings.quality=source.settings.quality;
  value.report.quality=source.report.quality;
  value.report.quality.occurrences=value.report.quality.occurrences.slice(0,2).flatMap(row=>['1','2'].map(id=>({
    ...structuredClone(row),membershipId:id,plotNumber:cell('P1'),siteUnit:cell('Original')
  })));
  value.report.diagnostics.push(...['1','2'].map(id=>({code:'quality_reference_multiplicity',identity:id,count:2})));
  assert.equal(validateSelectedUnits(value),value);
  assert.deepEqual(value.report.units.map(unit=>unit.numPlots),[4,4]);
  for(const mutate of [
    x=>x.report.quality.occurrences[0].envRowId='99',
    x=>x.report.quality.occurrences[0].membershipId='9',
    x=>x.report.quality.occurrences=[],
    x=>x.report.diagnostics=x.report.diagnostics.filter(d=>d.code!=='quality_reference_multiplicity'),
    x=>x.report.units[0].numPlots=8,
  ]) {const invalid=structuredClone(value);mutate(invalid);assert.throws(()=>validateSelectedUnits(invalid));}
});
test('draft source2 DTOs remain unavailable and cannot masquerade as ordinary reports',()=>{
  for(const mutate of [
    value=>value.settings.unitFields=['PlotNumber'],
    value=>value.settings.unitFields=[],
    value=>value.settings.unitFields=null,
    value=>value.settings.unitGrouping='selected-fields',
    value=>value.report.selectedUnits=[],
    value=>value.report.selectedUnits=null,
    value=>value.report.selectedUnits=[{membershipId:'1',envRowId:null,adminRowId:null,plotNumber:cell('P'),siteUnit:cell(null)}]
  ]) {
    const value=fixture();mutate(value);
    assert.throws(()=>validate(value),/unavailable|unsupported/);
    assert.throws(()=>validate(proxy(structuredClone(value))),/unavailable|unsupported/);
  }
  const value=fixture();value.settings.unitFields=['PlotNumber'];
  assert.throws(()=>validateOptions({contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',settings:value.settings}),/unavailable/);
});
test('strict-Go selected scalar preview retains all four typed kinds on actual client proxies with independent gates',()=>{
  for(const [kind, scalar] of Object.entries(scalarWire.cells)) {
    const value=structuredClone(scalarWire.preview);
    value.report.selectedFieldType=kind;
    value.report.units[0].rows[0].englishName=structuredClone(scalar);
    value.report.units[0].rows[0].matchedName=structuredClone(scalar);
    assert.equal(validate(value),value);
    const reactive=proxy(structuredClone(value));
    assert.throws(()=>structuredClone(reactive));
    assert.equal(validate(reactive),reactive);
    assert.equal(reactive.report.units[0].rows[0].englishName.storage,scalar.storage);
    assert.throws(()=>report.requireLongVegetationGrouping(value.settings,true,true,true),/selected field display is disabled/);
    assert.doesNotThrow(()=>report.requireLongVegetationGrouping(value.settings,false,false,false,true));
    const nulls=structuredClone(value);
    nulls.report.units[0].rows[0].englishName=cell(null);
    nulls.report.units[0].rows[0].matchedName=cell(null);
    assert.equal(validate(nulls),nulls);
    for(const mutate of [
      x=>delete x.report.selectedFieldType,
      x=>x.report.selectedFieldType='unknown',
      x=>x.settings.selectedField='',
      x=>x.settings.selectedField='\ud800',
      x=>x.settings.selectedField='bad\0field',
      x=>x.settings.showEnglishName=true,
      x=>x.settings.showSpeciesCode=true,
      x=>x.report.units[0].rows[0].englishName={...scalar,storage:'blob',blobHex:'00'},
      x=>delete x.settings.selectedField,
    ]) { const invalid=structuredClone(value); mutate(invalid); assert.throws(()=>validate(invalid)); }
  }
  for(const [kind, scalar] of [
    ['text',cell('bad\0text')],['text',cell('\ud800')],['integer',{...scalarWire.cells.integer,integer:'2147483648'}],
    ['integer',{...scalarWire.cells.integer,integer:'01'}],['boolean',{...scalarWire.cells.boolean,integer:'1'}],
    ['boolean',{...scalarWire.cells.real,real:-1}],['real',{...scalarWire.cells.real,real:Infinity}],
    ['real',scalarWire.cells.integer],
  ]) {
    const value=structuredClone(scalarWire.preview);
    value.report.selectedFieldType=kind; value.report.units[0].rows[0].englishName=scalar;
    assert.throws(()=>validate(value));
  }
});
const noneWire = JSON.parse(readFileSync(path.join(__dirname,'longVegetationNoneUnitsWire.json'),'utf8'));
test('strict-Go None unit wire distinguishes derived joins from physical membership IDs on actual client proxies',()=>{
  assert.equal(validate(noneWire),noneWire);
  const reactive=proxy(structuredClone(noneWire));
  assert.throws(()=>structuredClone(reactive));
  assert.equal(validate(reactive),reactive);
  assert.throws(()=>report.requireLongVegetationGrouping(noneWire.settings,true,true,true,true),/None report units are disabled/);
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(noneWire.settings,false,false,false,false,true));
  for(const mutate of [
    x=>delete x.report.noneUnits,
    x=>x.report.noneUnits=[],
    x=>delete x.settings.unitGrouping,
    x=>x.settings.unitGrouping='site-unit-table',
    x=>x.report.noneUnits.push(structuredClone(x.report.noneUnits[0])),
    x=>x.report.noneUnits[0].envRowId='01',
    x=>x.report.noneUnits[0].membershipId='9223372036854775808',
    x=>x.report.noneUnits[0].siteUnit=cell('all plots'),
    x=>x.report.noneUnits[0].plotNumber=cell(null),
    x=>x.report.noneUnits[0].plotNumber=cell('non-ASCII é'),
    x=>x.report.noneUnits[0].plotNumber=cell('bad\0plot'),
    x=>x.report.noneUnits[0].qualityOccurrence=0,
    x=>delete x.report.noneUnits[0].qualityOccurrence,
    x=>x.report.units[0].code=cell('Original unit'),
    x=>x.report.units[0].numPlots=1,
    x=>x.report.units[0].membershipIds=['1','1'],
    x=>x.report.units[0].membershipIds=['11'],
    x=>x.report.diagnostics[0].count=3,
    x=>x.report.diagnostics.shift(),
  ]) {const invalid=structuredClone(noneWire);mutate(invalid);assert.throws(()=>validate(invalid),String(mutate));}
  const missing=structuredClone(noneWire);
  missing.report.noneUnits.push({...structuredClone(missing.report.noneUnits[0]),membershipId:'2'});
  missing.report.units[0].membershipIds.push('2');missing.report.units[0].numPlots=3;
  assert.throws(()=>validate(missing),/complete Env\/current-list/);
});
test('None unit client retains original quality identities and exact outer Env/indexed-reference completeness',()=>{
  const value=qualifiedFixture();
  value.settings.unitGrouping='all-plots';
  value.report.noneUnits=value.report.quality.occurrences.map((original,index)=>({
    envRowId:original.envRowId,membershipId:original.membershipId,qualityOccurrence:index,
    plotNumber:structuredClone(original.plotNumber),siteUnit:cell('All plots')
  }));
  value.report.noneUnits.push(...value.report.noneUnits.slice(0,2).map(row=>({...structuredClone(row),envRowId:'13'})));
  value.report.units[0].code=cell('All plots');
  value.report.units[0].numPlots=5;
  value.report.diagnostics.push({code:'none_unit_join_multiplicity',identity:'1',count:4});
  assert.equal(validate(value),value);
  const original=structuredClone(value.report.quality);
  assert.equal(validate(proxy(structuredClone(value))).report.quality.occurrences[0].siteUnit.text,'');
  assert.deepEqual(value.report.quality,original);
  for(const mutate of [
    x=>x.report.noneUnits[0].qualityOccurrence=null,
    x=>x.report.noneUnits[0].qualityOccurrence=-1,
    x=>x.report.noneUnits[0].qualityOccurrence=.5,
    x=>x.report.noneUnits[0].qualityOccurrence=5,
    x=>x.report.noneUnits[0].membershipId='2',
    x=>x.report.noneUnits[0].plotNumber=cell('P1'),
    x=>x.report.noneUnits[0].envRowId='99',
    x=>x.report.quality.occurrences[0].siteUnit=cell('All plots'),
    x=>x.report.noneUnits.pop(),
    x=>x.report.units[0].numPlots=3,
    x=>x.report.diagnostics.find(d=>d.code==='none_unit_join_multiplicity').count=2,
  ]) {const invalid=structuredClone(value);mutate(invalid);assert.throws(()=>validate(invalid));}
});
for(const [label,presentationWire,expected] of [
  ['None',noneWire,[
    'None — All plots from Env/current selected list joins','joined sum / derived Env/current-list denominator',
    'Derived Env/current-list denominator: 2','retained physical membership rows: 1',
    'None unit join provenance (2 occurrences)','Outer ENV row 11; original SU row 1',
    'Outer ENV row 12; original SU row 1','unfiltered current list','expand to all-project scope',
  ]],
  ['selected-field',selectedUnitWire,['Selected field unit join provenance (8 occurrences)','Original SU row 1;',
    'ENV row 11; Admin row 21;','derived unit Alpha','filters by qualified plot existence',
    'Derived selected-field denominator: 4','retained physical membership rows: 2']],
]) test(`${label} report presentation labels derived denominator and original witnesses without claiming all-project scope`,()=>{
  const original=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  const source=original.replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
    `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(presentationWire.settings)});`)
    .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(presentationWire)});`);
  assert.notEqual(source,original);
  const component=serverComponent(buildSource(source),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,
    './longVegetationPreferences':{},'./longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{},'./LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{},'./LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences':{},'./LongVegetationQualityNullPreferences.svelte':{default(){}},
    './longVegetationGroupPreferences':{},'./LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences':{},'./LongVegetationQualityMinimumPreferences.svelte':{default(){}},
    './longVegetationNamingPreferences':{},'./LongVegetationNamingPreferences.svelte':{default(){}},
    './longVegetationSelectedFieldPreferences':{},'./LongVegetationSelectedFieldPreferences.svelte':{default(){}},
    './longVegetationEnforcementPreferences':{},'./LongVegetationEnforcementPreferences.svelte':{default(){}},
    './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
    './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
  for(const text of expected) assert.ok(result.body.replace(/\s+/g,' ').includes(text),`missing ${label} presentation: ${text}`);
  assert.ok(!result.body.includes('joined sum / physical SU denominator'));
});
const buildSource = (source, enabled = false, strataEnabled = false, codeEnabled = false, selectedFieldEnabled = false) => source.replace(
  "import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true'", String(enabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true'", String(strataEnabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_CODE === 'true'", String(codeEnabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_SELECTED_FIELD === 'true'", String(selectedFieldEnabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_NONE_UNITS === 'true'", 'false').replace(
  "import.meta.env.VITE_LONG_VEGETATION_WORKBOOK === 'true'", 'false').replace(
  "import.meta.env.VITE_LONG_VEGETATION_PREFERENCES === 'true'", 'false');
function options() {
  return {contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'Selected',suPath:'C:\\external.db',settings:settings()};
}
test('None ordering remains explicit while preserving separate source layer statistics',()=>{
  const value=fixture();value.settings.grouping='none';
  assert.equal(validate(value),value);
  const configured=options();configured.settings.grouping='none';
  assert.equal(validateOptions(configured).settings.grouping,'none');
  for(const grouping of [undefined,null,'unsupported',4]) {
    const invalid=fixture();invalid.settings.grouping=grouping;assert.throws(()=>validate(invalid));
  }
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  assert.match(source,/None mode changes row ordering, not cover aggregation/);
  assert.match(source,/constant species list retains its source layer-first order/);
  assert.match(source,/settings.grouping === 'none'/);
});
test('lifeform transport preserves nullable exact INTEGER groups without widening layer or species identities',()=>{
  const value=fixture();value.settings.grouping='lifeform';
  const integer=n=>({storage:'integer',integer:n,text:null,real:null,blobHex:null});
  value.report.units[0].rows[0].layer=integer('1');
  value.report.units[0].rows[1].layer=integer('-2');
  assert.equal(validate(value),value);
  for(const invalid of [cell('1'),{...integer('1'),integer:'+1'},
    integer('32768'),integer('-32769'),{...integer('1'),integer:'9223372036854775808'},
    {storage:'real',real:1,text:null,integer:null,blobHex:null}]) {
    const rejected=structuredClone(value);rejected.report.units[0].rows[0].layer=invalid;
    assert.throws(()=>validate(rejected));
  }
  const layer=structuredClone(value);layer.settings.grouping='layer';assert.throws(()=>validate(layer));
  const species=structuredClone(value);species.report.units[0].rows[0].species=integer('1');assert.throws(()=>validate(species));
  const configured=options();configured.settings.grouping='lifeform';
  assert.equal(validateOptions(configured).settings.grouping,'lifeform');
  assert.throws(()=>report.requireLongVegetationGrouping(configured.settings,false),/disabled in this build/);
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(configured.settings,true));
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(settings(),false));
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  assert.match(source,/VITE_LONG_VEGETATION_LIFEFORM === 'true'/);
  assert.equal((source.match(/requireLongVegetationGrouping\(candidate\.?(?:settings)?, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled\)/g)||[]).length,2);
  assert.match(source,/<th scope="col">\{groupLabel\}<\/th>/);
});
test('vegetation transport preserves constant-list threshold bypass, fanout, NULL/empty and zero/negative covers',()=>{
  const value = fixture();
  assert.equal(validate(value),value);
  assert.equal(validateOptions(options()).settings.title,'  Title \u00e9  ');
  value.report.units.push({code:cell(null),longName:'',nameStatus:'unassigned',nameCandidates:[],numPlots:1,membershipIds:['3','4'],rows:[]});
  assert.doesNotThrow(()=>validate(value));
  value.report.units=[];
  value.report.diagnostics=[];
  assert.doesNotThrow(()=>validate(value));
});
test('Strata grouping preserves text NULL cells and independently refuses disabled builds',()=>{
  const value=fixture();value.settings.grouping='strata';
  assert.equal(validate(value),value);
  const configured=options();configured.settings.grouping='strata';
  assert.equal(validateOptions(configured).settings.grouping,'strata');
  assert.throws(()=>report.requireLongVegetationGrouping(configured.settings,true,false),/disabled in this build/);
  assert.throws(()=>report.requireLongVegetationGrouping(configured.settings,false),/disabled in this build/);
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(configured.settings,false,true));
  const malformed=structuredClone(value);
  malformed.report.units[0].rows[0].layer={storage:'integer',integer:'1',text:null,real:null,blobHex:null};
  assert.throws(()=>validate(malformed));
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  assert.match(source,/VITE_LONG_VEGETATION_STRATA === 'true'/);
  assert.equal((source.match(/requireLongVegetationGrouping\(candidate\.?(?:settings)?, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled\)/g)||[]).length,2);
});
test('quality transport preserves physical IDs and recomputes denominators from complete distinct reference occurrences',()=>{
  const value=qualifiedFixture();
  assert.equal(validate(value),value);
  assert.equal(value.report.units[0].membershipIds.length,2);
  assert.equal(value.report.units[0].numPlots,3);
  value.report.units=[];
  assert.doesNotThrow(()=>validate(value));
});
test('quality minima independently reject omitted duplicate/conflicting definitions instead of trusting an ID subset',()=>{
  const value=qualifiedFixture(), original=value.report.quality.references[0];
  const duplicate={...structuredClone(original),rowId:'91'};
  value.report.quality.references.push(duplicate);
  assert.throws(()=>validate(value),/omitted matching/);
  value.report.quality.thresholdRowIds=[['90','91'],['90','91'],['90','91']];
  assert.doesNotThrow(()=>validate(value));
  duplicate.itemOrder.integer='4';
  assert.throws(()=>validate(value),/Conflicting quality thresholds/);
  value.report.quality.thresholdRowIds=[['90'],['90'],['90']];
  assert.throws(()=>validate(value),/omitted matching/);
});
test('quality transport rejects forged scope, repeated tuples, missing thresholds and contradictory NULL/denominator metadata',()=>{
  for(const mutate of [
    x=>delete x.settings.quality,x=>x.settings.quality.site.minimum='\ud800',
    x=>x.settings.quality.site.minimum='A\0B',x=>x.settings.quality.veg.includeNull='True',
    x=>x.report.quality=null,x=>x.report.quality.thresholdRowIds.pop(),
    x=>x.report.quality.thresholdRowIds[0]=null,x=>x.report.quality.thresholdRowIds[0]=[],
    x=>x.report.quality.thresholdRowIds[0]=['01'],x=>x.report.quality.thresholdRowIds[0]=['90','90'],
    x=>x.report.quality.occurrences=null,x=>x.report.quality.occurrences=[],
    x=>x.report.quality.occurrences.push(x.report.quality.occurrences[0]),
    x=>x.report.quality.occurrences[0].membershipId='2',
    x=>x.report.quality.occurrences[0].plotNumber=cell(null),
    x=>x.report.quality.occurrences[0].siteUnit=cell('Other'),
    x=>x.report.quality.occurrences[0].envRowId='01',
    x=>x.report.quality.occurrences[0].adminRowId='99',
    x=>x.report.quality.occurrences[0].listRowIds.pop(),
    x=>x.report.quality.occurrences[0].listRowIds[1]='01',
    x=>x.report.quality.occurrences[0].values[0]=cell('NA'),
    x=>x.report.quality.occurrences[0].values[0]=cell('Changed'),
    x=>x.report.quality.occurrences[0].values=null,
    x=>x.report.quality.references=null,x=>x.report.quality.references.splice(4,1),
    x=>x.report.quality.references[0].itemOrder={...cell(null),storage:'real',real:1.5},
    x=>x.report.quality.references[0].item=cell('Invented'),
    x=>x.report.quality.references[0].listName=cell('PlotQualitySite'),
    x=>x.report.quality.references.push(x.report.quality.references[0]),
    x=>x.report.quality.occurrences.splice(1,1),
    x=>x.settings.quality.site.includeNull=false,
    x=>x.settings.quality.soil.includeNull=false,
    x=>x.report.quality.excludedMembershipIds=null,
    x=>x.report.quality.excludedMembershipIds=['1'],
    x=>x.report.quality.excludedMembershipIds=['9','9'],
    x=>x.report.quality.excludedMembershipIds=['01'],
    x=>x.report.units[0].numPlots=2,x=>x.report.units[0].numPlots=4,
    x=>x.report.units[0].membershipIds=['1','3'],
    x=>x.report.diagnostics.pop(),x=>x.report.diagnostics[1].count=3,
    x=>x.report.diagnostics.push(x.report.diagnostics[1])
  ]) { const value=qualifiedFixture(); mutate(value); assert.throws(()=>validate(value)); }
  const value=fixture();
  value.report.quality=qualifiedFixture().report.quality;
  assert.throws(()=>validate(value));
});
test('vegetation options reject stale identities and unsupported/incomplete settings without guessed bounds',()=>{
  for (const mutate of [
    x=>x.contextId='stale',x=>x.project='Other',x=>x.su='None',x=>x.suPath='C:\\other.db',
    x=>x.settings=null,x=>x.settings.title='\ud800',x=>x.settings.title='A\0B',
    x=>x.settings.average='plots',x=>x.settings.order='unknown',x=>x.settings.constantSpeciesList=-1,
    x=>delete x.settings.showEnglishName,x=>x.settings.presenceGreaterThan='5',x=>x.settings.meanCoverGreaterThan=Infinity
  ]) { const value=options(); mutate(value); assert.throws(()=>validateOptions(value)); }
  const value=options(); value.settings.presenceGreaterThan=-200; value.settings.meanCoverGreaterThan=-3;
  assert.doesNotThrow(()=>validateOptions(value));
});
test('vegetation preview rejects incomplete typed values, ownership, denominators and pivots',()=>{
  for (const mutate of [
    x=>x.contextId='stale',x=>x.projectPath='C:\\other.db',x=>x.suPath='C:\\other.db',
    x=>x.report.su='Other',x=>x.report.title='Title',x=>x.settings.average='unsupported',
    x=>x.report.units=null,x=>x.report.diagnostics=null,
    x=>x.report.units[0].code=cell('\ud800'),x=>x.report.units.push(x.report.units[0]),
    x=>x.report.units[0].numPlots=1,x=>x.report.units[0].numPlots=0,
    x=>x.report.units[0].membershipIds=['01','2'],x=>x.report.units[0].membershipIds=['1','1'],
    x=>x.report.units[0].rows[0].species={...cell(null),storage:'integer',integer:'1'},
    x=>x.report.units[0].rows[0].layer.text=null,
    x=>x.report.units[0].rows[0].presence=NaN,x=>delete x.report.units[0].rows[0].meanCover,
    x=>x.report.units[0].rows[0].meanCover=null,
    x=>x.report.units[0].rows[0].plots[0].plotNumber='\ud800',
    x=>x.report.units[0].rows[0].plots[0].cover=Infinity,
    x=>x.report.units[0].rows[0].plots[1].plotNumber='',
    x=>x.report.units[0].rows[1].plots.reverse(),
    x=>x.report.units[0].rows[2].plots[0].cover=0,
    x=>x.settings.showEnglishName=false,
    x=>x.report.diagnostics[0].count=-1,x=>x.report.diagnostics[0].code='unknown',
    x=>x.report.diagnostics[0].identity='\ud800'
  ]) { const value=fixture(); mutate(value); assert.throws(()=>validate(value)); }
});
test('shared unit names preserve empty/NULL/duplicate candidates and reject invented or ambiguous names',()=>{
  const value=fixture(), unit=value.report.units[0];
  unit.nameStatus='unique'; unit.longName='';
  unit.nameCandidates=[{rowId:'1',value:cell('')},{rowId:'2',value:cell(null)},{rowId:'3',value:cell('')}];
  assert.doesNotThrow(()=>validate(value));
  unit.nameCandidates.push({rowId:'4',value:cell('Other')});
  assert.throws(()=>validate(value));
  unit.nameStatus='conflicting'; unit.longName=null;
  assert.doesNotThrow(()=>validate(value));
  unit.nameCandidates=[{rowId:'1',value:cell(null)}]; unit.nameStatus='missing';
  value.report.diagnostics=[{code:'unit_name_missing',identity:'text:',count:1}];
  assert.doesNotThrow(()=>validate(value));
  for(const mutate of [
    x=>x.report.units[0].longName='Invented',x=>x.report.units[0].nameStatus='unique',
    x=>x.report.units[0].nameCandidates=null,
    x=>x.report.units[0].nameCandidates[0].rowId='01',
    x=>x.report.units[0].nameCandidates.push(x.report.units[0].nameCandidates[0]),
    x=>x.report.units[0].nameCandidates[0].value=cell('\ud800')
  ]) { const copy=structuredClone(value); mutate(copy); assert.throws(()=>validate(copy)); }
  unit.nameCandidates=[]; value.report.diagnostics[0].count=0;
  assert.doesNotThrow(()=>validate(value));
  unit.nameCandidates=[{rowId:'1',value:{...cell(null),storage:'integer',integer:'9'}}];
  unit.nameStatus='unsupported_storage';
  assert.doesNotThrow(()=>validate(value));
  const unassigned={longName:'',nameStatus:'unassigned',nameCandidates:[]};
  assert.doesNotThrow(()=>names.validateReportUnitNames(unassigned,true));
  assert.throws(()=>names.validateReportUnitNames({...unassigned,longName:null},true));
  assert.throws(()=>names.validateReportUnitNames({...unassigned,nameCandidates:[{rowId:'1',value:cell(null)}]},true));
});
test('vegetation bindings are cancellable and report UI is independently default-off and draft-safe',()=>{
  const bindings=readFileSync(path.join(__dirname,'..','bindings','github.com','boostao','vpro-wails','contextservice.ts'),'utf8');
  for (const name of ['GetLongVegetationOptions','PreviewLongVegetation']) {
    assert.match(bindings,new RegExp(`export function ${name}\\(`));
    assert.match(bindings,new RegExp(`${name}\\([^)]*\\): \\$CancellablePromise<`));
  }
  const source = file => readFileSync(path.join(__dirname,file),'utf8');
  for (const file of ['App.svelte','Navigation.svelte','LongVegetationReport.svelte']) {
    assert.equal(compile(source(file),{filename:file,generate:'client'}).warnings.length,0);
  }
  assert.match(source('Navigation.svelte'),/VITE_LONG_VEGETATION_REPORT === 'true' \? 'long-vegetation' : undefined/);
  assert.match(source('App.svelte'),/view === 'long-vegetation' && import\.meta\.env\.VITE_LONG_VEGETATION_REPORT === 'true'/);
  assert.match(source('App.svelte'),/view === 'long-vegetation'[^\n]+\n\s*\{#if error\}<p class="error" role="alert">\{error\}/);
  assert.match(source('App.svelte'),/\{#key \$projectState\.contextId\}[\s\S]*<LongVegetationReport/);
  assert.match(source('App.svelte'),/closeDisposition\(state, busy \|\| editorBusy/);
  const panel=source('LongVegetationReport.svelte');
  const teardown=panel.slice(panel.indexOf('  onDestroy(() => {'),panel.indexOf('</script>',panel.indexOf('  onDestroy(() => {')));
  for(const subscription of ['Workbook','Preferences','Statistics','Switches','Thresholds','Groups','QualityNulls',
    'QualityMinima','Naming','SelectedField','Enforcement','Units','Project']) {
    assert.match(teardown,new RegExp(`unsubscribe${subscription}\\??\\.?(?:\\(\\))`));
  }
  assert.match(teardown,/cancel\(\)/);
  assert.match(panel,/request !== generation/);
  assert.match(panel,/reads\.track\(ContextService\.PreviewLongVegetation\(contextId\)\)/);
  assert.match(panel,/const candidate = validateLongVegetationPreview/);
  assert.match(panel,/requireLongVegetationGrouping\(candidate\.settings, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled\)/);
  assert.match(panel,/preview = candidate/);
  assert.match(panel,/settings = candidate\.settings/);
  assert.match(panel,/data-source-control="btnViewReport"/);
  assert.doesNotMatch(panel,/SavePlot|SavePreferences/);
  assert.equal((panel.match(/oninput=/g)||[]).length,1);
  assert.equal((panel.match(/bind:value=/g)||[]).length,1);
});
test('vegetation report uses responsive labelled read-only settings, separate names and literal nullable statistics',()=>{
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  const component=serverComponent(buildSource(source),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,'./longVegetationPreferences':{},
    './longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
    './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
    './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{
    contextId:'owned',project:'Sample',projectPath:'C:\\quoted <project>.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}
  }});
  assert.match(result.body,/C:\\quoted &lt;project>\.db/);
  assert.match(result.body,/Whole selected-SU scope/);
  assert.match(result.body,/disabled[^>]*>View Report/);
  assert.match(source,/grid-template-columns: repeat\(auto-fit, minmax\(min\(100%, 19rem\), 1fr\)\)/);
  assert.match(source,/thresholds are bypassed, as in the source report/);
  assert.match(source,/<th scope="col">List English name<\/th><th scope="col">Matched report English name/);
  assert.match(source,/value === null \? 'NULL' : String\(value\)/);
  assert.match(source,/Unassigned \(NULL\)/);
  assert.match(source,/role="region" aria-label=/);
  assert.match(source,/Options are loaded from YAML; only title persistence has a separate reviewed default-off gate/);
  assert.doesNotMatch(result.body,/id="long-vegetation-title"/);
});
test('quality presentation retains literal criteria, NULL ranks versus missing joins and physical versus qualified counts',()=>{
  const value=qualifiedFixture();
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
      `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,'./longVegetationPreferences':{},
    './longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
    './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
    './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{
    contextId:'owned',project:'Sample',projectPath:'C:\\project.db',su:'Selected',
    suPath:'C:\\external.db',onBusyChange:()=>{}
  }});
  for(const text of ['Site quality minimum','Vegetation quality minimum','Soil quality minimum',
    '  Poor  ','joined sum / qualified SU denominator','Qualified denominator: 3','retained physical membership rows: 2',
    'Quality qualification provenance (3 occurrences)','rank NULL','NULL join',
    'DataQuality definitions, not the editor dropdown ordering']) {
    assert.ok(result.body.includes(text),`missing quality presentation: ${text}`);
  }
  assert.ok(!result.body.includes('joined sum / physical SU denominator'));
});
test('selected scalar presentation shows literal list/matched headings and native Boolean/Long/DOUBLE values',()=>{
  const original=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  for(const [kind, scalar, expected] of [
    ['text',scalarWire.cells.text,'=literal &amp; native'],
    ['integer',scalarWire.cells.integer,'2147483647'],
    ['boolean',scalarWire.cells.boolean,'TRUE'],
    ['boolean',{...scalarWire.cells.boolean,integer:'0'},'FALSE'],
    ['real',scalarWire.cells.real,'1.005'],
    ['text',cell(null),'NULL'],
  ]) {
    const value=structuredClone(scalarWire.preview);
    value.report.selectedFieldType=kind;
    value.report.units[0].rows[0].englishName=structuredClone(scalar);
    value.report.units[0].rows[0].matchedName=structuredClone(scalar);
    validate(value);
    const source=original.replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
      `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
      .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
        `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
    assert.notEqual(source,original);
    const component=serverComponent(buildSource(source,false,false,false,true),'LongVegetationReport.svelte',{
      '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
      './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,
      './longVegetationPreferences':{},'./longVegetationStatisticsPreferences':{},
      './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
      './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
      './longVegetationQualityNullPreferences':{}, './LongVegetationQualityNullPreferences.svelte':{default(){}},
      './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
      './longVegetationQualityMinimumPreferences':{}, './LongVegetationQualityMinimumPreferences.svelte':{default(){}},
      './longVegetationNamingPreferences':{}, './LongVegetationNamingPreferences.svelte':{default(){}},
      './longVegetationSelectedFieldPreferences':{}, './LongVegetationSelectedFieldPreferences.svelte':{default(){}},
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
    });
    const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
      su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
    for(const heading of ['List  Literal &amp; "field" ','Matched report  Literal &amp; "field" ']) {
      assert.ok(result.body.includes(heading),`missing literal scalar heading: ${heading}`);
    }
    assert.ok(result.body.includes(expected),`missing ${kind} output: ${expected}`);
    assert.ok(!result.body.includes('List English name'));
  }
});
test('lifeform presentation labels typed group and source-specific conversion instead of claiming standalone summary',()=>{
  const value=fixture();value.settings.grouping='lifeform';
  for(const row of value.report.units[0].rows) {
    if(row.layer.storage==='text') row.layer={storage:'integer',integer:'1',text:null,real:null,blobHex:null};
  }
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
      `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,'./longVegetationPreferences':{},
    './longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
    './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
    './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
  assert.match(result.body.replace(/<!--[\s\S]*?-->/g,''),/<th\b[^>]*scope="col"[^>]*>Lifeform<\/th>/);
  for(const text of ['Lifeform, then species','99.9 cap',
    'Cover10 is not part of this conversion','not the standalone Lifeform Summary']) {
    assert.ok(result.body.includes(text),`missing lifeform presentation: ${text}`);
  }
});
test('Strata presentation labels original totals fallback cap and independent child criteria',()=>{
  const value=fixture();value.settings.grouping='strata';
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
      `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,false,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,'./longVegetationPreferences':{},
    './longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
    './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
    './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
  assert.match(result.body.replace(/<!--[\s\S]*?-->/g,''),/<th\b[^>]*scope="col"[^>]*>Strata<\/th>/);
  for(const text of ['Strata, then species','TotalA/TotalB','capped at 99','Extended B covers',
    'Only positive A/B','zero or negative values','Cover8/9/10']) {
    assert.ok(result.body.includes(text),`missing Strata presentation: ${text}`);
  }
});
test('supplementary Code metadata is explicit nullable text and independently gated',()=>{
  const value=fixture();value.settings.showEnglishName=false;value.settings.showSpeciesCode=true;
  value.report.units[0].rows[0].englishName=cell('  LongerThanEight  ');
  assert.equal(validate(value),value);
  const configured=options();configured.settings.showEnglishName=false;configured.settings.showSpeciesCode=true;
  assert.equal(validateOptions(configured).settings.showSpeciesCode,true);
  assert.throws(()=>report.requireLongVegetationGrouping(configured.settings,true,true),/disabled in this build/);
  assert.doesNotThrow(()=>report.requireLongVegetationGrouping(configured.settings,false,false,true));
  for(const mutate of [
    x=>delete x.settings.showSpeciesCode,x=>x.settings.showSpeciesCode=2,
    x=>x.settings.showEnglishName=true,
    x=>x.report.units[0].rows[0].englishName={storage:'integer',integer:'1',text:null,real:null,blobHex:null}
  ]) {
    const invalid=structuredClone(value);mutate(invalid);assert.throws(()=>validate(invalid));
  }
  const bindings=readFileSync(path.join(__dirname,'..','bindings','github.com','boostao','vpro-wails','models.ts'),'utf8');
  assert.match(bindings,/"showSpeciesCode": boolean/);
});
test('Code presentation preserves source caption but labels literal list/matched master codes',()=>{
  const value=fixture();value.settings.showEnglishName=false;value.settings.showSpeciesCode=true;
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state.raw<LongVegetationSettings | null>(null);',
      `let settings = $state.raw<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state.raw<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,false,false,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook,'./longVegetationPreferences':{},
    './longVegetationStatisticsPreferences':{},
    './longVegetationSwitchPreferences':{}, './LongVegetationSwitchPreferences.svelte':{default(){}},
    './longVegetationThresholdPreferences':{}, './LongVegetationThresholdPreferences.svelte':{default(){}},
    './longVegetationQualityNullPreferences': {}, './LongVegetationQualityNullPreferences.svelte': { default() {} }, './longVegetationGroupPreferences':{}, './LongVegetationGroupPreferences.svelte':{default(){}},
    './longVegetationQualityMinimumPreferences': {}, './LongVegetationQualityMinimumPreferences.svelte': { default() {} },
    './longVegetationNamingPreferences': {}, './LongVegetationNamingPreferences.svelte': { default() {} },
    './longVegetationSelectedFieldPreferences': {}, './LongVegetationSelectedFieldPreferences.svelte': { default() {} },
      './longVegetationEnforcementPreferences': {}, './LongVegetationEnforcementPreferences.svelte': { default() {} },
      './longVegetationUnitPreferences': {}, './LongVegetationUnitPreferences.svelte': { default() {} },
      './longVegetationSelectedUnitSelection': {}, './LongVegetationSelectedUnitSelection.svelte': { default() {} },
  });
  const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
  for(const text of ['Show 8 char. code','List Code','Matched report Code','literal values are not truncated']) {
    assert.ok(result.body.includes(text),`missing Code presentation: ${text}`);
  }
  assert.ok(!result.body.includes('List English name'));
});
