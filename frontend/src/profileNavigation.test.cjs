const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const resource = name => JSON.parse(readFileSync(path.join(__dirname,'..','..','resources',name),'utf8'));
const quality = loadTypeScript('qualityEditor.ts',{'./becEditor':loadTypeScript('becEditor.ts')});
const metadata = loadTypeScript('projectMetadataEditor.ts',{'./qualityEditor':quality,
  '../../resources/project-metadata-standard.json':resource('project-metadata-standard.json'),
  '../../resources/project-metadata-template.json':resource('project-metadata-template.json')});
const profile = loadTypeScript('projectPlotProfileReview.ts',{'./projectMetadataEditor':metadata});
const read = name => readFileSync(path.join(__dirname,name),'utf8');
const nullCell = () => ({storage:'null',text:null,integer:null,real:null,blobHex:null});
const rules = {columns:Array.from(profile.plotProfileFields,name=>({name,declaredType:'TEXT'})),
  rows:['1','2'].map(rowId=>({rowId,cells:Array.from({length:9},nullCell)}))};
const preview = {project:'Sample',table:'Sample_Profile',su:'None',totalPlots:4,plotNumbers:[" A' # ",'Z'],
  steps:[{rowId:'1',order:1,operation:'Add plots',plotCount:4,remaining:4},
    {rowId:'2',order:2,operation:'Subtract plots',plotCount:2,remaining:2}]};
const proposal = {input:{originalRules:rules,projectLump:null,subvarieties:false},preview};
const summary = plotNumber => ({plotNumber,fieldNumber:null,plotRepresenting:null,zone:null,subZone:null,siteSeries:null});
const navigation = () => ({contextId:'owned',result:structuredClone(preview),plots:preview.plotNumbers.map(summary)});

test('Scoped navigation preserves exact literal membership and paginates one immutable approved recordset',()=>{
  const source=profile.validateProfileNavigation(navigation(),proposal,'owned');
  const first=profile.profileNavigationPage(source,'owned',0,1);
  const second=profile.profileNavigationPage(source,'owned',1,1);
  assert.equal(first.total,2); assert.equal(first.plots[0].plotNumber," A' # ");
  assert.equal(second.total,2); assert.equal(second.plots[0].plotNumber,'Z');
  assert.equal(profile.profileNavigationPage(source,'owned',2,1).plots.length,0);
  for(const [id,offset,limit] of [['stale',0,1],['owned',-1,1],['owned',0,201],['owned',0,0],['owned',.5,1]]) {
    assert.throws(()=>profile.profileNavigationPage(source,id,offset,limit),/stale or out of range/);
  }
});

test('Navigation rejects foreign scopes, inconsistent counts, normalization, duplicate summaries and missing fields',()=>{
  for(const alter of [
    value=>value.contextId='stale',value=>value.result.su='Other',value=>value.result.totalPlots=5,
    value=>value.result.steps[0].plotCount=3,value=>value.result.plotNumbers[0]="A' #",
    value=>value.plots[0].plotNumber="A' #",value=>value.plots.reverse(),
    value=>value.plots[1]=value.plots[0],value=>value.plots=null,
    value=>delete value.plots[0].fieldNumber,value=>value.plots[0].zone=5
  ]) {
    const value=navigation(); alter(value);
    assert.throws(()=>profile.validateProfileNavigation(value,proposal,'owned'));
  }
});

test('Zero and over200 previews stay unavailable without clearing an existing filter; exact200 remains supported',()=>{
  for(const [count,unavailable] of [[0,true],[1,false],[200,false],[201,true]]) {
    const result={...preview,plotNumbers:Array.from({length:count},(_,index)=>String(index))};
    assert.equal(profile.profileFilterUnavailable(result)!==null,unavailable);
  }
});

test('Apply/Clear/Previous/Next reuse transition ownership and revalidation before publishing, with context retirement and a fresh plot mount',()=>{
  const app=read('App.svelte'),panel=read('ProjectPlotProfileReview.svelte'),form=read('FS882Form.svelte');
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_FILTERING === 'true'/);
  assert.match(panel,/allowFiltering = false/);
  assert.match(panel,/disabled=\{busy \|\| blocked \|\| profileFilterUnavailable\(result\) !== null\}/);
  assert.match(panel,/runInput = \$state\.snapshot\(input\)/);
  assert.match(panel,/structuredClone\(\$state\.snapshot\(\{ input: runInput, preview: result \}\)\)/);
  assert.match(app,/onProfileNavigation=\{applyProfileNavigation\}/);
  for(const name of ['applyProfileNavigation','clearProfileNavigation','navigateProfilePlot']) {
    const start=app.indexOf(`async function ${name}`);
    assert.ok(start>=0);
    assert.match(app.slice(start,app.indexOf('\n  async function ',start+1)),/await requestTransition/);
  }
  assert.match(app,/const next = await resolveNavigation\(proposal, contextId\);[\s\S]*profileNavigation = next; page = nextPage/);
  assert.match(app,/const next = await resolveNavigation\(source\.proposal, contextId\);[\s\S]*editorPlotNumber = target/);
  assert.match(app,/const next = source \? await resolveNavigation\(source\.proposal, source\.contextId\)/);
  assert.match(app,/if \(profileNavigation && profileNavigation\.contextId !== state\.contextId\) profileNavigation = null/);
  assert.match(app,/projectState\.set\(state\);\s*profileNavigation = null/);
  assert.match(app,/\{#key \$projectState\?\.contextId\}\s*\{#key editorPlotNumber\}/);
  assert.match(app,/<fieldset disabled=\{busy\} class="contents">/);
  assert.match(app,/await tick\(\);\s*const next = await resolveNavigation/);
  assert.doesNotMatch(app,/SetProfileFilter|CreateSU|UpdatePlotCount/);
});
