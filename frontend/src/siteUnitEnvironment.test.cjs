const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor': loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor': quality, './projectMetadataEditor': editor});
const forward = loadTypeScript('environmentSUTransfer.ts', {'./projectMetadataRestore': restoration, './projectMetadataEditor': editor});
const reverse = loadTypeScript('siteUnitEnvironment.ts', {
  './projectMetadataRestore': restoration, './projectMetadataEditor': editor, './environmentSUTransfer': forward
});
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});
const table = (names,cells) => ({columns:names.map(name=>({name,declaredType:'VARCHAR'})),rows:cells.map((cells,i)=>({rowId:String(i+1),cells}))});
function review() {
  return {contextId:'owned',project:'Project',su:'Selected',path:'C:\\owned.db',historyHash:'',
    env:table(['PlotNumber','Flag'],[[cell('P1'),cell(null)]]),
    admin:table(['Plot','UserSiteUnit','SiteUnitShortName','SiteUnitLongName'],[[cell('P1'),cell('Old'),cell('Short'),cell('Long')]]),
    source:table(['PlotNumber','SiteUnit'],[[cell('P1'),cell('  Unit  ')]]),
    master:table(['SiteSeries','SiteSeriesLongName'],[[cell('  Unit  '),cell('Master long')]]),
    personal:table(['SiteSeries','SiteSeriesLongName'],[[cell('  Unit  '),cell(null)]]),
    changes:[
      {plotNumber:'P1',envRowId:'1',adminRowId:'1',suRowId:'1',field:'UserSiteUnit',origin:'selected SU',before:cell('Old'),after:cell('  Unit  ')},
      {plotNumber:'P1',envRowId:'1',adminRowId:'1',suRowId:'1',field:'SiteUnitShortName',origin:'personal override',before:cell('Short'),after:cell('  Unit  ')},
      {plotNumber:'P1',envRowId:'1',adminRowId:'1',suRowId:'1',field:'SiteUnitLongName',origin:'personal override',before:cell('Long'),after:cell(null)}
    ]};
}
const validate = r => reverse.validateSiteUnitEnvironmentReview(r,'owned','Project','Selected','C:\\owned.db');
const request = r => reverse.siteUnitEnvironmentRequest(r,'owned','Project','Selected','C:\\owned.db');
test('Reverse SU review preserves original identities, NULL names, personal precedence and immutable proposals',()=>{
  const original=review(), payload=request(original);
  original.personal.rows[0].cells[1].text='Independent';
  assert.equal(payload.review.personal.rows[0].cells[1].text,null);
  assert.equal(payload.review.changes[0].after.text,'  Unit  ');
  assert.equal(payload.review.changes[2].after.storage,'null');
  assert.throws(()=>request({...review(),changes:[]}),/nonempty/);
});
test('Reverse review rejects locked/duplicate/foreign/malformed and mismatched definitions',()=>{
  for(const mutate of [
    r=>r.contextId='stale',r=>r.path='C:\\foreign.db',r=>r.su='None',
    r=>r.env.rows[0].cells[1]={storage:'integer',integer:'-1',text:null,real:null,blobHex:null},
    r=>r.personal.rows.push({...r.personal.rows[0],rowId:'2'}),
    r=>r.changes[2].origin='master',r=>r.changes[1].field='EnteredBy',
    r=>r.changes.push({...r.changes[0]}),r=>r.changes[0].adminRowId='01',
    r=>r.changes[0].after.text='\ud800',r=>r.changes[0].after.text='x'.repeat(101),
    r=>r.changes[1].after.text='x'.repeat(51),r=>r.changes[2].after.integer='1'
  ]) { const r=review(); mutate(r); assert.throws(()=>validate(r)); }
});
test('Reverse committed results require exact cell and row counts with positive exact history ID',()=>{
  const payload=request(review());
  assert.doesNotThrow(()=>reverse.validateSiteUnitEnvironmentResult({changedRows:1,changedCells:3,historyId:'9007199254740993'},payload));
  for(const result of [null,{changedRows:3,changedCells:3,historyId:'1'},{changedRows:1,changedCells:2,historyId:'1'},
    {changedRows:1,changedCells:3,historyId:'0'}]) {
    assert.throws(()=>reverse.validateSiteUnitEnvironmentResult(result,payload),/without replaying/);
  }
});
test('Reverse uses shared persistent panel and independent source gate, retiring commits before reload',()=>{
  const root=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  const panel=readFileSync(path.join(__dirname,'EnvironmentSUTransfer.svelte'),'utf8');
  for(const file of ['FS882Form.svelte','EnvironmentSUTransfer.svelte']) {
    assert.equal(compile(readFileSync(path.join(__dirname,file),'utf8'),{filename:file,generate:'client'}).warnings.length,0);
  }
  assert.match(root,/VITE_SOURCE_SU_ENV_TRANSFER === 'true'/);
  assert.match(root,/data-source-control="btnSuIntoEnv"/);
  assert.match(root,/environmentSUDirection === 'reverse'\) void load\(original\?\.plotNumber\)/);
  assert.match(panel,/committed = true; review = null;[\s\S]*validateSiteUnitEnvironmentResult/);
  assert.match(panel,/Personal definitions override master names; missing definitions retain existing names/);
  assert.match(panel,/SU\/environment transfer committed, but/);
});
