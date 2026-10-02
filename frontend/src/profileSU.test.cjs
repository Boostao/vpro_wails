const assert=require('node:assert/strict');
const {test}=require('node:test');
const {readFileSync}=require('node:fs');
const path=require('node:path');
const {loadTypeScript}=require('./svelteTestHelpers.cjs');
const resource=name=>JSON.parse(readFileSync(path.join(__dirname,'..','..','resources',name),'utf8'));
const quality=loadTypeScript('qualityEditor.ts',{'./becEditor':loadTypeScript('becEditor.ts')});
const metadata=loadTypeScript('projectMetadataEditor.ts',{'./qualityEditor':quality,
  '../../resources/project-metadata-standard.json':resource('project-metadata-standard.json'),
  '../../resources/project-metadata-template.json':resource('project-metadata-template.json')});
const profile=loadTypeScript('projectPlotProfileReview.ts',{'./projectMetadataEditor':metadata});
const su=loadTypeScript('profileSU.ts',{'./projectPlotProfileReview':profile});
const projectSU=loadTypeScript('profileSUProject.ts',{'./profileSU':su,'./projectPlotProfileReview':profile});
const read=name=>readFileSync(path.join(__dirname,name),'utf8');
const nullCell=()=>({storage:'null',text:null,integer:null,real:null,blobHex:null});
const rules={columns:profile.plotProfileFields.map(name=>({name,declaredType:'TEXT'})),
  rows:[{rowId:'1',cells:profile.plotProfileFields.map(nullCell)}]};
const preview={project:'Sample',table:'External_Profile',su:'None',totalPlots:2,plotNumbers:['A','B'],
  steps:[{rowId:'1',order:1,operation:'Add plots',plotCount:2,remaining:2}]};
const filter={input:{originalRules:rules,projectLump:null,subvarieties:false},preview};
const review=()=>({filter:structuredClone(filter),template:{columns:['PlotNumber','SiteUnit'].map(name=>({name,declaredType:'VARCHAR'})),rows:[]},
  sourceSU:null,descriptions:null,plots:[{plotNumber:'A',siteUnit:null},{plotNumber:'B',siteUnit:null}]});

test('Project-file SU review preserves complete destination descriptions and rejects foreign paths or inferred metadata',()=>{
  const source={project:'Sample',path:"C:\\project O'Brien #.db",su:review(),
    metadata:{columns:[{name:'table_name',declaredType:'TEXT'},{name:'description',declaredType:'TEXT'}],rows:[]}};
  assert.equal(projectSU.validateProjectSUReview(source,filter,'Sample',source.path).path,source.path);
  for(const alter of [value=>value.project='Foreign',value=>value.path='C:\\other.db',
    value=>value.metadata=null,value=>value.su.descriptions={columns:source.metadata.columns,rows:[]},
    value=>value.su.plots.pop()]){
    const invalid=structuredClone(source);alter(invalid);
    assert.throws(()=>projectSU.validateProjectSUReview(invalid,filter,'Sample',source.path));
  }
  const created={name:'New',path:source.path,plotCount:2};
  assert.equal(projectSU.validateProjectSUCreated(created,'New',2,source.path),created);
  assert.throws(()=>projectSU.validateProjectSUCreated({...created,path:'C:\\other.db'},'New',2,source.path),/do not replay/);
});

test('Project-file SU insertion is separately gated, immutable-destination reviewed and shared-transition owned',()=>{
  const app=read('App.svelte');
  assert.match(app,/VITE_PROJECT_PLOT_PROFILE_SAVE_SU_PROJECT === 'true'/);
  assert.match(app,/aria-label="Reviewed project-file SU creation"/);
  assert.match(app,/<label for="profile-su-project-name"/);
  const start=app.indexOf('async function saveProfileSUProject');
  const action=app.slice(start,app.indexOf('\n  async function ',start+1));
  assert.match(action,/\$state\.snapshot\(\{ review: suProjectReview, name: suProjectName, confirmed: true \}\)/);
  assert.match(action,/await requestTransition/);
  assert.match(action,/contextId !== origin/);
  assert.match(action,/await tick\(\);\s*const response = await ContextService.SaveProjectPlotProfileSUInProject/);
  assert.match(action,/suProjectCommitted = true/);
  assert.match(action,/Profile SU table committed, but/);
  assert.match(action,/stateReads\.track\(ProjectService\.GetState\(\)\)/);
  assert.match(action,/state\.contextId !== contextId \|\| \$projectState\?\.contextId !== contextId/);
  assert.match(action,/projectState\.set\(state\)/);
  assert.doesNotMatch(action,/SwitchContext|SelectPlotProfile/);
});

test('SU review preserves exact external-rule membership, explicit absent metadata and NULL/empty/literal SiteUnits',()=>{
  assert.equal(su.validateProfileSUReview(review(),filter).plots.length,2);
  const selected=review();
  selected.filter.preview.su='Picked';
  selected.sourceSU={columns:selected.template.columns,rows:[]};
  selected.plots[0].siteUnit='';
  selected.plots[1].siteUnit=" U' # ";
  const result=su.validateProfileSUReview(selected,selected.filter);
  assert.equal(result.plots[0].siteUnit,'');
  assert.equal(result.plots[1].siteUnit," U' # ");
});

test('Incomplete, foreign, repaired and overlength SU responses fail rather than inventing assignments',()=>{
  for(const alter of [
    value=>delete value.sourceSU,value=>delete value.descriptions,
    value=>value.filter.preview.project='Other',value=>value.filter.input.subvarieties=true,
    value=>value.plots.reverse(),value=>delete value.plots[0].siteUnit,
    value=>value.plots[0].siteUnit=5,value=>value.plots[0].siteUnit='A'.repeat(256),
    value=>value.plots=[],value=>value.sourceSU={columns:value.template.columns,rows:[]},
    value=>value.template.columns.reverse()
  ]){
    const value=review();alter(value);
    assert.throws(()=>su.validateProfileSUReview(value,filter));
  }
  const created={name:'Saved',path:'C:\\new.db',plotCount:2};
  assert.deepEqual(su.validateProfileSUCreated(created,'Saved',2),created);
  for(const value of [{...created,name:'Other'},{...created,path:''},{...created,plotCount:1}])
    assert.throws(()=>su.validateProfileSUCreated(value,'Saved',2),/do not replay/);
});

test('SU publication is default-off, snapshot-captured and guarded independently by shared Save/Discard/Cancel',()=>{
  const app=read('App.svelte'),form=read('FS882Form.svelte'),panel=read('ProjectPlotProfileReview.svelte');
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_SAVE_SU === 'true'/);
  assert.match(panel,/allowSaveSU = false/);
  assert.match(panel,/onsureview\(structuredClone\(\$state\.snapshot/);
  assert.match(app,/onProfileSUReview=\{reviewProfileSU\}/);
  const start=app.indexOf('async function saveProfileSU');
  const action=app.slice(start,app.indexOf('\n  async function ',start+1));
  assert.match(action,/\$state\.snapshot\(\{ review: suReview, name: suName, path: suPath, confirmed: true \}\)/);
  assert.match(action,/await requestTransition/);
  assert.match(action,/await tick\(\);\s*const response = await ContextService.SaveProjectPlotProfileSU/);
  assert.match(action,/suCommitted = true/);
  assert.match(action,/Profile SU file published, but/);
  assert.match(action,/view = 'plots'; editorPlotNumber = undefined/);
  assert.doesNotMatch(action,/SwitchContext|SelectPlotProfile/);
  assert.match(app,/suReviewContext !== \$projectState\?\.contextId/);
  assert.match(app,/<label for="profile-su-name"/);
  assert.match(app,/<label for="profile-su-path"/);
});
