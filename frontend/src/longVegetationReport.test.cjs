const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {render} = require('svelte/server');
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
const buildSource = (source, enabled = false, strataEnabled = false, codeEnabled = false) => source.replace(
  "import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true'", String(enabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true'", String(strataEnabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_CODE === 'true'", String(codeEnabled)).replace(
  "import.meta.env.VITE_LONG_VEGETATION_WORKBOOK === 'true'", 'false');
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
  assert.equal((source.match(/requireLongVegetationGrouping\(candidate\.?(?:settings)?, lifeformEnabled, strataEnabled, codeEnabled\)/g)||[]).length,2);
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
  assert.equal((source.match(/requireLongVegetationGrouping\(candidate\.?(?:settings)?, lifeformEnabled, strataEnabled, codeEnabled\)/g)||[]).length,2);
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
  assert.match(panel,/onDestroy\(\(\) => \{ unsubscribeWorkbook\(\); cancel\(\); \}\)/);
  assert.match(panel,/request !== generation/);
  assert.match(panel,/reads\.track\(ContextService\.PreviewLongVegetation\(contextId\)\)/);
  assert.match(panel,/const candidate = validateLongVegetationPreview/);
  assert.match(panel,/requireLongVegetationGrouping\(candidate\.settings, lifeformEnabled, strataEnabled, codeEnabled\)/);
  assert.match(panel,/preview = candidate/);
  assert.match(panel,/settings = candidate\.settings/);
  assert.match(panel,/data-source-control="btnViewReport"/);
  assert.doesNotMatch(panel,/SavePlot|SavePreferences|oninput=/);
  assert.equal((panel.match(/bind:value=/g)||[]).length,1);
});
test('vegetation report uses responsive labelled read-only settings, separate names and literal nullable statistics',()=>{
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8');
  const component=serverComponent(buildSource(source),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook
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
  assert.match(source,/Options are loaded from YAML and cannot be edited here/);
});
test('quality presentation retains literal criteria, NULL ranks versus missing joins and physical versus qualified counts',()=>{
  const value=qualifiedFixture();
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state<LongVegetationSettings | null>(null);',
      `let settings = $state<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook
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
test('lifeform presentation labels typed group and source-specific conversion instead of claiming standalone summary',()=>{
  const value=fixture();value.settings.grouping='lifeform';
  for(const row of value.report.units[0].rows) {
    if(row.layer.storage==='text') row.layer={storage:'integer',integer:'1',text:null,real:null,blobHex:null};
  }
  validate(value);
  const source=readFileSync(path.join(__dirname,'LongVegetationReport.svelte'),'utf8')
    .replace('let settings = $state<LongVegetationSettings | null>(null);',
      `let settings = $state<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook
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
    .replace('let settings = $state<LongVegetationSettings | null>(null);',
      `let settings = $state<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,false,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook
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
    .replace('let settings = $state<LongVegetationSettings | null>(null);',
      `let settings = $state<LongVegetationSettings | null>(${JSON.stringify(value.settings)});`)
    .replace('let preview = $state<ValidatedLongVegetationPreview | null>(null);',
      `let preview = $state<ValidatedLongVegetationPreview | null>(${JSON.stringify(value)});`);
  const component=serverComponent(buildSource(source,false,false,true),'LongVegetationReport.svelte',{
    '../bindings/github.com/boostao/vpro-wails':{ContextService:{}},
    './longEnvironmentReport':environment,'./longVegetationReport':report,'./vegetationWorkbook':workbook
  });
  const result=render(component,{props:{contextId:'owned',project:'Sample',projectPath:'C:\\project.db',
    su:'Selected',suPath:'C:\\external.db',onBusyChange:()=>{}}});
  for(const text of ['Show 8 char. code','List Code','Matched report Code','literal values are not truncated']) {
    assert.ok(result.body.includes(text),`missing Code presentation: ${text}`);
  }
  assert.ok(!result.body.includes('List English name'));
});
